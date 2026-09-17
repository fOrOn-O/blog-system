package service

import "blog-system/internal/htmlcontent"

// Diff 使用独立模型，继续忽略标题级别，保持 Task 08 的归一化语义。
func normalizeArticleHTML(content string) ([]ContentBlock, error) {
	parsed, err := htmlcontent.ParseFragment(content)
	if err != nil {
		return nil, err
	}
	blocks := make([]ContentBlock, 0, len(parsed))
	for _, block := range parsed {
		blocks = append(blocks, ContentBlock{Type: block.Type, Text: block.Text})
	}
	return blocks, nil
}
