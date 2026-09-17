package htmlcontent

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type Block struct {
	Type         string
	Text         string
	HeadingLevel int
}

// Parse 按 HTML 文档解析，保留 head 容器，随后统一忽略其中的非正文内容。
func Parse(content string) ([]Block, error) {
	root, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, err
	}
	return extractBlocks(root), nil
}

// ParseFragment 保留 Diff 原有的片段解析行为，与文档解析共享文本提取逻辑。
func ParseFragment(content string) ([]Block, error) {
	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(content), root)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		root.AppendChild(node)
	}
	return extractBlocks(root), nil
}

// extractBlocks 提取纯文本并忽略行内格式和属性；列表项和引用中的段落继承容器类型。
func extractBlocks(root *html.Node) []Block {
	blocks := make([]Block, 0)
	var extract func(*html.Node, string)
	extract = func(node *html.Node, kind string) {
		var text strings.Builder
		start := len(blocks)
		flush := func() {
			value := text.String()
			text.Reset()
			if kind != "code_block" {
				value = strings.Join(strings.Fields(value), " ")
			} else {
				value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
			}
			if value != "" {
				blocks = append(blocks, Block{Type: kind, Text: value, HeadingLevel: headingLevel(node.Data)})
			}
		}
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.TextNode {
				text.WriteString(n.Data)
				return
			}
			if n.Type != html.ElementNode {
				return
			}
			switch n.Data {
			case "script", "style", "template", "head":
				return
			case "br":
				text.WriteByte('\n')
				return
			}
			next := semanticBlockType(n.Data)
			if next != "" && kind != "code_block" {
				flush()
				if next == "paragraph" && (kind == "list_item" || kind == "blockquote") {
					next = kind
				}
				extract(n, next)
				return
			}
			// 未知块容器按文档顺序保留文本，避免相邻容器的文本粘连。
			boundary := n.Data == "div" || n.Data == "ul" || n.Data == "ol" || n.Data == "section"
			if boundary {
				flush()
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
			if boundary {
				flush()
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		flush()
		if len(blocks) == start && semanticBlockType(node.Data) != "" {
			blocks = append(blocks, Block{Type: kind, Text: "", HeadingLevel: headingLevel(node.Data)})
		}
	}
	extract(root, "paragraph")
	return blocks
}

func semanticBlockType(tag string) string {
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return "heading"
	case "p":
		return "paragraph"
	case "li":
		return "list_item"
	case "blockquote":
		return "blockquote"
	case "pre":
		return "code_block"
	default:
		return ""
	}
}

func headingLevel(tag string) int {
	if len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6' {
		return int(tag[1] - '0')
	}
	return 0
}
