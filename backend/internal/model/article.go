package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	ArticleStatusDraft     = "draft"
	ArticleStatusPublished = "published"
	ArticleStatusArchived  = "archived"
)

var (
	ErrArticleNotFound         = errors.New("文章不存在")
	ErrArticleForbidden        = errors.New("无权访问或修改此文章")
	ErrArticleArchivedEdit     = errors.New("文章已归档，不能继续编辑")
	ErrArticleArchivedPublish  = errors.New("文章已归档，不能发布")
	ErrArticleSnapshotMissing  = errors.New("文章内容版本快照缺失，不能发布")
	ErrVersionConflict         = errors.New("文章已被其他操作更新，请刷新后重新编辑")
	ErrExpectedVersionRequired = errors.New("必须提供大于 0 的 expected_version")
)

// Article 文章模型
type Article struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	Version          uint           `gorm:"not null;default:1" json:"version"`
	PublishedVersion uint           `gorm:"not null;default:0" json:"published_version"`
	Title            string         `gorm:"size:200;not null" json:"title"`
	Content          string         `gorm:"type:text;not null" json:"content"`
	Summary          string         `gorm:"size:500" json:"summary"`
	CoverImage       string         `gorm:"size:255" json:"cover_image"`
	UserID           uint           `gorm:"index;not null" json:"user_id"`
	User             User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ViewCount        int            `gorm:"default:0" json:"view_count"`
	LikeCount        int            `gorm:"default:0" json:"like_count"`
	CommentCount     int            `gorm:"default:0" json:"comment_count"`
	Status           string         `gorm:"size:20;default:draft" json:"status"` // draft, published, archived
	Tags             []Tag          `gorm:"many2many:article_tags;" json:"tags,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// Tag 标签模型
type Tag struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	Name     string    `gorm:"size:50;uniqueIndex;not null" json:"name"`
	Articles []Article `gorm:"many2many:article_tags;" json:"articles,omitempty"`
}
