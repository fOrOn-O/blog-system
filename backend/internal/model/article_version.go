package model

import (
	"errors"
	"time"
)

var ErrArticleVersionNotFound = errors.New("文章历史版本不存在")

const (
	ArticleVersionSourceUser  = "user"
	ArticleVersionSourceAgent = "agent"
)

// ArticleVersion 记录文章内容的不可变快照。
type ArticleVersion struct {
	ID         uint   `gorm:"primaryKey"`
	ArticleID  uint   `gorm:"not null;uniqueIndex:idx_article_versions_article_version"`
	VersionNo  uint   `gorm:"not null;uniqueIndex:idx_article_versions_article_version"`
	Title      string `gorm:"size:200;not null"`
	Content    string `gorm:"type:text;not null"`
	Summary    string `gorm:"size:500"`
	CoverImage string `gorm:"size:255"`
	CreatedBy  uint   `gorm:"not null"`
	Source     string `gorm:"size:20;not null"`
	CreatedAt  time.Time
}
