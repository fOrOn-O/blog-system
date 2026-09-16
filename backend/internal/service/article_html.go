package service

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// normalizeArticleHTML 提取纯文本，不返回可执行的 HTML；本阶段忽略行内格式和属性。
// 列表项和引用中的嵌套段落继承其容器类型。
func normalizeArticleHTML(content string) ([]ContentBlock, error) {
	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(content), root)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		root.AppendChild(node)
	}
	blocks := make([]ContentBlock, 0)
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
				blocks = append(blocks, ContentBlock{kind, value})
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
			blocks = append(blocks, ContentBlock{kind, ""})
		}
	}
	extract(root, "paragraph")
	return blocks, nil
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
