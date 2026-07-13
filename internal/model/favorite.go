package model

import "time"

// Favorite 收藏模型
type Favorite struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ArticleID uint      `gorm:"index;not null" json:"article_id"`
	Article   Article   `gorm:"foreignKey:ArticleID" json:"article,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名
func (Favorite) TableName() string {
	return "favorites"
}
