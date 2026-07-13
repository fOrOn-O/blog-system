package repository

import (
	"blog-system/internal/database"
	"blog-system/internal/model"

	"gorm.io/gorm"
)

// CommentRepository 评论数据访问层
type CommentRepository struct{}

// NewCommentRepository 创建评论Repository
func NewCommentRepository() *CommentRepository {
	return &CommentRepository{}
}

// Create 创建评论
func (r *CommentRepository) Create(comment *model.Comment) error {
	return database.DB.Create(comment).Error
}

// FindByID 根据ID查找评论
func (r *CommentRepository) FindByID(id uint) (*model.Comment, error) {
	var comment model.Comment
	err := database.DB.Preload("User").First(&comment, id).Error
	return &comment, err
}

// Delete 删除评论（软删除）
func (r *CommentRepository) Delete(id uint) error {
	return database.DB.Delete(&model.Comment{}, id).Error
}

// DeleteByArticleID 删除文章的所有评论
func (r *CommentRepository) DeleteByArticleID(articleID uint) error {
	return database.DB.Where("article_id = ?", articleID).Delete(&model.Comment{}).Error
}

// GetByArticleID 获取文章的评论列表（分页，包含嵌套回复）
func (r *CommentRepository) GetByArticleID(articleID uint, page, limit int) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64

	// 只查询顶级评论
	query := database.DB.Model(&model.Comment{}).
		Where("article_id = ? AND parent_id IS NULL", articleID)

	query.Count(&total)

	offset := (page - 1) * limit
	err := query.Preload("User").
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Preload("User").Order("created_at ASC")
		}).
		Offset(offset).Limit(limit).
		Order("created_at DESC").
		Find(&comments).Error

	return comments, total, err
}

// GetReplyCount 获取评论的回复数
func (r *CommentRepository) GetReplyCount(commentID uint) int64 {
	var count int64
	database.DB.Model(&model.Comment{}).Where("parent_id = ?", commentID).Count(&count)
	return count
}

// CountByArticleID 统计文章的评论数
func (r *CommentRepository) CountByArticleID(articleID uint) int64 {
	var count int64
	database.DB.Model(&model.Comment{}).Where("article_id = ?", articleID).Count(&count)
	return count
}
