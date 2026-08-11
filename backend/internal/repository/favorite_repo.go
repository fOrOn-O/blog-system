package repository

import (
	"blog-system/internal/database"
	"blog-system/internal/model"
)

// FavoriteRepository 收藏数据访问层
type FavoriteRepository struct{}

// NewFavoriteRepository 创建收藏Repository
func NewFavoriteRepository() *FavoriteRepository {
	return &FavoriteRepository{}
}

// Create 创建收藏
func (r *FavoriteRepository) Create(favorite *model.Favorite) error {
	return database.DB.Create(favorite).Error
}

// Delete 删除收藏
func (r *FavoriteRepository) Delete(userID, articleID uint) error {
	return database.DB.Where("user_id = ? AND article_id = ?", userID, articleID).
		Delete(&model.Favorite{}).Error
}

// Exists 检查用户是否已收藏
func (r *FavoriteRepository) Exists(userID, articleID uint) bool {
	var count int64
	database.DB.Model(&model.Favorite{}).
		Where("user_id = ? AND article_id = ?", userID, articleID).
		Count(&count)
	return count > 0
}

// GetByUserID 获取用户的收藏列表
func (r *FavoriteRepository) GetByUserID(userID uint, page, limit int) ([]model.Favorite, int64, error) {
	var favorites []model.Favorite
	var total int64

	query := database.DB.Model(&model.Favorite{}).Where("user_id = ?", userID)
	query.Count(&total)

	offset := (page - 1) * limit
	err := query.Preload("Article").Preload("Article.User").Preload("Article.Tags").
		Offset(offset).Limit(limit).
		Order("created_at DESC").
		Find(&favorites).Error

	return favorites, total, err
}
