package repository

import (
	"blog-system/internal/database"
	"blog-system/internal/model"
)

// LikeRepository 点赞数据访问层
type LikeRepository struct{}

// NewLikeRepository 创建点赞Repository
func NewLikeRepository() *LikeRepository {
	return &LikeRepository{}
}

// Create 创建点赞
func (r *LikeRepository) Create(like *model.Like) error {
	return database.DB.Create(like).Error
}

// Delete 删除点赞
func (r *LikeRepository) Delete(userID, articleID uint) error {
	return database.DB.Where("user_id = ? AND article_id = ?", userID, articleID).
		Delete(&model.Like{}).Error
}

// Exists 检查用户是否已点赞
func (r *LikeRepository) Exists(userID, articleID uint) bool {
	var count int64
	database.DB.Model(&model.Like{}).
		Where("user_id = ? AND article_id = ?", userID, articleID).
		Count(&count)
	return count > 0
}

// CountByArticleID 统计文章的点赞数
func (r *LikeRepository) CountByArticleID(articleID uint) int64 {
	var count int64
	database.DB.Model(&model.Like{}).Where("article_id = ?", articleID).Count(&count)
	return count
}
