// Package chunking 将文章 HTML 确定性地转换为带标题上下文的结构化文本块。
package chunking

import (
	"errors"
	"strings"
	"unicode/utf8"

	"blog-system/internal/htmlcontent"
)

type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

type ChunkBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Chunk struct {
	Index       int          `json:"index"`
	HeadingPath []Heading    `json:"heading_path"`
	Blocks      []ChunkBlock `json:"blocks"`
}

type Options struct {
	TargetSize  int // 优先在达到目标后沿块边界结束，但可继续合并较小的尾部。
	MaxSize     int // 合并普通块时的上限；单个完整块可使用 MaxOverflow 额度。
	MaxOverflow int // 为保留单个块或句/行边界而允许的小幅溢出量；0 表示不允许溢出。
}

func DefaultOptions() Options {
	return Options{TargetSize: 800, MaxSize: 1200, MaxOverflow: 120}
}

// textSize 是唯一的文本尺寸计量入口；当前使用 rune，不按 UTF-8 字节计数。
func textSize(text string) int { return utf8.RuneCountInString(text) }

const blockSeparator = "\n\n"

// ChunkHTML 只产生内存中的结构化数据，不访问数据库、模型或外部服务。
// 尺寸包含正文和块间分隔符，不包含 HeadingPath。空正文块不生成 Chunk。
func ChunkHTML(content string, options Options) ([]Chunk, error) {
	maxInt := int(^uint(0) >> 1)
	if options.TargetSize <= 0 || options.MaxSize < options.TargetSize ||
		options.MaxOverflow < 0 || options.MaxOverflow > maxInt-options.MaxSize {
		return nil, errors.New("分块配置要求 0 < TargetSize <= MaxSize，且溢出额度非负、总上限不溢出")
	}
	parsed, err := htmlcontent.Parse(content)
	if err != nil {
		return nil, err
	}
	chunks := make([]Chunk, 0)
	path := make([]Heading, 0)
	group := make([]ChunkBlock, 0)
	flush := func() {
		for _, blocks := range packBlocks(group, options) {
			// 每个 Chunk 持有独立切片，后续标题栈变化和调用方修改不会影响其他 Chunk。
			headings := append(make([]Heading, 0, len(path)), path...)
			chunks = append(chunks, Chunk{Index: len(chunks), HeadingPath: headings, Blocks: blocks})
		}
		group = group[:0]
	}
	for _, block := range parsed {
		if block.Type == "heading" {
			flush()
			for len(path) > 0 && path[len(path)-1].Level >= block.HeadingLevel {
				path = path[:len(path)-1]
			}
			path = append(path, Heading{Level: block.HeadingLevel, Text: block.Text})
			continue
		}
		if block.Text == "" {
			continue
		}
		group = append(group, ChunkBlock{Type: block.Type, Text: block.Text})
	}
	flush()
	return chunks, nil
}

func packBlocks(group []ChunkBlock, options Options) [][]ChunkBlock {
	pieces := make([]ChunkBlock, 0, len(group))
	for _, block := range group {
		for _, text := range splitBlock(block.Text, block.Type == "code_block", options) {
			pieces = append(pieces, ChunkBlock{Type: block.Type, Text: text})
		}
	}
	sizes := make([]int, len(pieces))
	remaining := 0
	separatorSize := textSize(blockSeparator)
	for i := range pieces {
		sizes[i] = textSize(pieces[i].Text)
		remaining += sizes[i] + separatorSize
	}
	result := make([][]ChunkBlock, 0)
	current := make([]ChunkBlock, 0)
	size := 0
	flush := func() {
		if len(current) > 0 {
			result = append(result, current)
			current = make([]ChunkBlock, 0)
			size = 0
		}
	}
	for i, block := range pieces {
		remaining -= sizes[i] + separatorSize
		if len(current) > 0 && sizes[i]+separatorSize > options.MaxSize-size {
			flush()
		}
		if len(current) > 0 {
			size += separatorSize
		}
		current = append(current, block)
		size += sizes[i]
		// 达到目标后优先结束；若剩余尾部能全部放进 MaxSize，则继续合并，避免小尾块。
		if size >= options.TargetSize && remaining > options.MaxSize-size {
			flush()
		}
	}
	flush()
	return result
}

// RenderText 按需渲染纯文本；Chunk 只存结构化数据，不维护第二份 Content 字段。
func RenderText(chunk Chunk) string {
	headings := make([]string, 0, len(chunk.HeadingPath))
	for _, heading := range chunk.HeadingPath {
		headings = append(headings, heading.Text)
	}
	blocks := make([]string, 0, len(chunk.Blocks))
	for _, block := range chunk.Blocks {
		blocks = append(blocks, block.Text)
	}
	body := strings.Join(blocks, blockSeparator)
	if len(headings) == 0 {
		return body
	}
	if body == "" {
		return strings.Join(headings, " > ")
	}
	return strings.Join(headings, " > ") + blockSeparator + body
}
