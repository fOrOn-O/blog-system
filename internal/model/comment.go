package model

import (
	"time"

	"gorm.io/gorm"
)

// Comment 评论模型
type Comment struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	User      User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ArticleID uint           `gorm:"index;not null" json:"article_id"`
	Article   Article        `gorm:"foreignKey:ArticleID" json:"article,omitempty"`
	ParentID  *uint          `gorm:"index" json:"parent_id"` // 父评论ID，nil表示顶级评论
	Parent    *Comment       `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Replies   []Comment      `gorm:"foreignKey:ParentID" json:"replies,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
