package repository

import (
	"blog-system/internal/database"
	"blog-system/internal/model"
)

// UserRepository 用户数据访问层
type UserRepository struct{}

// NewUserRepository 创建用户Repository
func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

// Create 创建用户
func (r *UserRepository) Create(user *model.User) error {
	return database.DB.Create(user).Error
}

// FindByID 根据ID查找用户
func (r *UserRepository) FindByID(id uint) (*model.User, error) {
	var user model.User
	err := database.DB.First(&user, id).Error
	return &user, err
}

// FindByUsername 根据用户名查找用户
func (r *UserRepository) FindByUsername(username string) (*model.User, error) {
	var user model.User
	err := database.DB.Where("username = ?", username).First(&user).Error
	return &user, err
}

// FindByEmail 根据邮箱查找用户
func (r *UserRepository) FindByEmail(email string) (*model.User, error) {
	var user model.User
	err := database.DB.Where("email = ?", email).First(&user).Error
	return &user, err
}

// Update 更新用户
func (r *UserRepository) Update(user *model.User) error {
	return database.DB.Save(user).Error
}

// Delete 删除用户（软删除）
func (r *UserRepository) Delete(id uint) error {
	return database.DB.Delete(&model.User{}, id).Error
}

// List 获取用户列表
func (r *UserRepository) List(page, limit int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	database.DB.Model(&model.User{}).Count(&total)

	offset := (page - 1) * limit
	err := database.DB.Offset(offset).Limit(limit).Order("id DESC").Find(&users).Error

	return users, total, err
}
