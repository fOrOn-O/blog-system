package repository

import (
	"blog-system/internal/database"
	"blog-system/internal/model"
)

// TagRepository 标签仓库
type TagRepository struct{}

// NewTagRepository 创建标签仓库实例
func NewTagRepository() *TagRepository {
	return &TagRepository{}
}

// GetAll 获取所有标签
func (r *TagRepository) GetAll() ([]model.Tag, error) {
	var tags []model.Tag
	err := database.DB.Find(&tags).Error
	return tags, err
}

// GetByID 根据ID获取标签
func (r *TagRepository) GetByID(id uint) (*model.Tag, error) {
	var tag model.Tag
	err := database.DB.First(&tag, id).Error
	return &tag, err
}

// GetByName 根据名称获取标签
func (r *TagRepository) GetByName(name string) (*model.Tag, error) {
	var tag model.Tag
	err := database.DB.Where("name = ?", name).First(&tag).Error
	return &tag, err
}

// Create 创建标签
func (r *TagRepository) Create(tag *model.Tag) error {
	return database.DB.Create(tag).Error
}

// Update 更新标签
func (r *TagRepository) Update(tag *model.Tag) error {
	return database.DB.Save(tag).Error
}

// Delete 删除标签
func (r *TagRepository) Delete(id uint) error {
	return database.DB.Delete(&model.Tag{}, id).Error
}

// GetByArticleID 获取文章的标签
func (r *TagRepository) GetByArticleID(articleID uint) ([]model.Tag, error) {
	var tags []model.Tag
	err := database.DB.Joins("JOIN article_tags ON article_tags.tag_id = tags.id").
		Where("article_tags.article_id = ?", articleID).
		Find(&tags).Error
	return tags, err
}
