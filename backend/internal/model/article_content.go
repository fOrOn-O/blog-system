package model

import "errors"

var (
	ErrInvalidArticleContent   = errors.New("文章正文不能为空")
	ErrArticleContentUnchanged = errors.New("文章正文没有实际变化")
)

// ValidateArticleContent 与现有人工写入的 required/min=1 规则一致，不是 HTML sanitizer。
func ValidateArticleContent(content string) error {
	if content == "" {
		return ErrInvalidArticleContent
	}
	return nil
}
