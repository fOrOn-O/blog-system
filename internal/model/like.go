package model

import "time"

// Like 点赞模型
type Like struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ArticleID uint      `gorm:"index;not null" json:"article_id"`
	Article   Article   `gorm:"foreignKey:ArticleID" json:"article,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名
func (Like) TableName() string {
	return "likes"
}
