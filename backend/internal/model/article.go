package model

import (
	"time"

	"gorm.io/gorm"
)

// Article 文章模型
type Article struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Title        string         `gorm:"size:200;not null" json:"title"`
	Content      string         `gorm:"type:text;not null" json:"content"`
	Summary      string         `gorm:"size:500" json:"summary"`
	CoverImage   string         `gorm:"size:255" json:"cover_image"`
	UserID       uint           `gorm:"index;not null" json:"user_id"`
	User         User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ViewCount    int            `gorm:"default:0" json:"view_count"`
	LikeCount    int            `gorm:"default:0" json:"like_count"`
	CommentCount int            `gorm:"default:0" json:"comment_count"`
	Status       string         `gorm:"size:20;default:published" json:"status"` // draft, published, archived
	Tags         []Tag          `gorm:"many2many:article_tags;" json:"tags,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Tag 标签模型
type Tag struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	Name     string    `gorm:"size:50;uniqueIndex;not null" json:"name"`
	Articles []Article `gorm:"many2many:article_tags;" json:"articles,omitempty"`
}
