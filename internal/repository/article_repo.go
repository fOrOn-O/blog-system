package repository

import (
	"blog-system/internal/database"
	"blog-system/internal/model"
)

// ArticleRepository 文章数据访问层
type ArticleRepository struct{}

// NewArticleRepository 创建文章Repository
func NewArticleRepository() *ArticleRepository {
	return &ArticleRepository{}
}

// Create 创建文章
func (r *ArticleRepository) Create(article *model.Article) error {
	return database.DB.Create(article).Error
}

// FindByID 根据ID查找文章
func (r *ArticleRepository) FindByID(id uint) (*model.Article, error) {
	var article model.Article
	err := database.DB.Preload("User").Preload("Tags").First(&article, id).Error
	return &article, err
}

// Update 更新文章
func (r *ArticleRepository) Update(article *model.Article) error {
	return database.DB.Save(article).Error
}

// Delete 删除文章（软删除）
func (r *ArticleRepository) Delete(id uint) error {
	return database.DB.Delete(&model.Article{}, id).Error
}

// List 获取文章列表（分页）
func (r *ArticleRepository) List(page, limit int, status string) ([]model.Article, int64, error) {
	var articles []model.Article
	var total int64

	query := database.DB.Model(&model.Article{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)

	offset := (page - 1) * limit
	err := query.Preload("User").Preload("Tags").
		Offset(offset).Limit(limit).
		Order("created_at DESC").
		Find(&articles).Error

	return articles, total, err
}

// Search 搜索文章
func (r *ArticleRepository) Search(keyword string, page, limit int) ([]model.Article, int64, error) {
	var articles []model.Article
	var total int64

	query := database.DB.Model(&model.Article{}).
		Where("title LIKE ? OR content LIKE ?", "%"+keyword+"%", "%"+keyword+"%")

	query.Count(&total)

	offset := (page - 1) * limit
	err := query.Preload("User").Preload("Tags").
		Offset(offset).Limit(limit).
		Order("created_at DESC").
		Find(&articles).Error

	return articles, total, err
}

// IncrementViewCount 增加浏览量
func (r *ArticleRepository) IncrementViewCount(id uint) error {
	return database.DB.Model(&model.Article{}).Where("id = ?", id).
		UpdateColumn("view_count", database.DB.Raw("view_count + 1")).Error
}

// UpdateLikeCount 更新点赞数
func (r *ArticleRepository) UpdateLikeCount(id uint, count int) error {
	return database.DB.Model(&model.Article{}).Where("id = ?", id).
		Update("like_count", count).Error
}

// UpdateCommentCount 更新评论数
func (r *ArticleRepository) UpdateCommentCount(id uint, count int) error {
	return database.DB.Model(&model.Article{}).Where("id = ?", id).
		Update("comment_count", count).Error
}

// Count 统计文章总数
func (r *ArticleRepository) Count() int64 {
	var count int64
	database.DB.Model(&model.Article{}).Count(&count)
	return count
}
