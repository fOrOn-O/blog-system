package chunking

import "unicode"

// splitBlock 仅在单个块超过软上限和溢出额度之和时拆分。
// 切片保留分隔符及所有 rune；按顺序拼接后严格等于归一化的原块，不添加文本重叠。
func splitBlock(text string, code bool, options Options) []string {
	softLimit := options.MaxSize + options.MaxOverflow
	if textSize(text) <= softLimit {
		return []string{text}
	}
	runes := []rune(text)
	boundaries := blockBoundaries(runes, code)
	pieces := make([]string, 0)
	start, next := 0, 0
	for len(runes)-start > softLimit {
		cut := start
		for next < len(boundaries) && boundaries[next]-start <= options.MaxSize {
			cut = boundaries[next]
			next++
		}
		if cut == start {
			if next < len(boundaries) && boundaries[next]-start <= softLimit {
				cut = boundaries[next]
				next++
			} else {
				cut = start + options.MaxSize
			}
		}
		pieces = append(pieces, string(runes[start:cut]))
		start = cut
	}
	if start < len(runes) {
		pieces = append(pieces, string(runes[start:]))
	}
	return pieces
}

func blockBoundaries(text []rune, code bool) []int {
	boundaries := make([]int, 0)
	for i := 0; i < len(text); i++ {
		if code {
			if text[i] == '\n' {
				boundaries = append(boundaries, i+1)
			}
			continue
		}
		switch text[i] {
		case '。', '！', '？', '.', '!', '?':
			end := i + 1
			// 将连续句末标点、闭引号和其后空白归入前一句，避免标点或空白单独成片。
			for end < len(text) && (unicode.IsSpace(text[end]) || sentenceClosingRune(text[end])) {
				end++
			}
			boundaries = append(boundaries, end)
			i = end - 1
		}
	}
	return boundaries
}

func sentenceClosingRune(r rune) bool {
	switch r {
	case '。', '！', '？', '.', '!', '?', '"', '\'', '”', '’', '」', '』', ')', '）':
		return true
	default:
		return false
	}
}
