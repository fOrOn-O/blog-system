package service

import (
	"errors"

	"blog-system/internal/model"
	"blog-system/internal/repository"
)

// TagService 标签服务
type TagService struct {
	tagRepo *repository.TagRepository
}

// NewTagService 创建标签服务实例
func NewTagService() *TagService {
	return &TagService{
		tagRepo: repository.NewTagRepository(),
	}
}

// GetAll 获取所有标签
func (s *TagService) GetAll() ([]model.Tag, error) {
	return s.tagRepo.GetAll()
}

// GetByID 根据ID获取标签
func (s *TagService) GetByID(id uint) (*model.Tag, error) {
	return s.tagRepo.GetByID(id)
}

// Create 创建标签
func (s *TagService) Create(name string) (*model.Tag, error) {
	// 检查标签名是否已存在
	existing, _ := s.tagRepo.GetByName(name)
	if existing != nil && existing.ID > 0 {
		return nil, errors.New("标签名已存在")
	}

	tag := &model.Tag{
		Name: name,
	}

	if err := s.tagRepo.Create(tag); err != nil {
		return nil, errors.New("创建标签失败")
	}

	return tag, nil
}

// Update 更新标签
func (s *TagService) Update(id uint, name string) (*model.Tag, error) {
	tag, err := s.tagRepo.GetByID(id)
	if err != nil {
		return nil, errors.New("标签不存在")
	}

	// 检查新名称是否与其他标签重复
	if name != tag.Name {
		existing, _ := s.tagRepo.GetByName(name)
		if existing != nil && existing.ID > 0 && existing.ID != id {
			return nil, errors.New("标签名已存在")
		}
	}

	tag.Name = name

	if err := s.tagRepo.Update(tag); err != nil {
		return nil, errors.New("更新标签失败")
	}

	return tag, nil
}

// Delete 删除标签
func (s *TagService) Delete(id uint) error {
	_, err := s.tagRepo.GetByID(id)
	if err != nil {
		return errors.New("标签不存在")
	}

	return s.tagRepo.Delete(id)
}
