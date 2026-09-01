package service

import (
	"errors"

	"blog-system/internal/repository"
	"blog-system/pkg/auth"

	"gorm.io/gorm"
)

var (
	ErrUserNotFound      = errors.New("用户不存在")
	ErrCannotDisableSelf = errors.New("管理员不能封禁自己")
	ErrCannotManageAdmin = errors.New("不能通过用户管理接口操作管理员账号")
)

// UserService 用户服务
type UserService struct {
	userRepo *repository.UserRepository
}

// NewUserService 创建用户服务
func NewUserService() *UserService {
	return &UserService{
		userRepo: repository.NewUserRepository(),
	}
}

// UpdateProfileRequest 更新个人信息请求
type UpdateProfileRequest struct {
	Email  string `json:"email" binding:"omitempty,email"`
	Avatar string `json:"avatar"`
	Bio    string `json:"bio"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=72"`
}

// UpdateUserStatusRequest 管理员更新用户状态请求
type UpdateUserStatusRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

// ResetUserPasswordRequest 管理员重置用户密码请求
type ResetUserPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6,max=72"`
}

// GetProfile 获取用户个人信息
func (s *UserService) GetProfile(userID uint) (*UserResponse, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	return toUserResponse(user), nil
}

// UpdateProfile 更新用户个人信息
func (s *UserService) UpdateProfile(userID uint, req UpdateProfileRequest) (*UserResponse, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 检查邮箱是否已被其他用户使用
	if req.Email != "" && req.Email != user.Email {
		if _, err := s.userRepo.FindByEmail(req.Email); err == nil {
			return nil, errors.New("邮箱已被使用")
		}
		user.Email = req.Email
	}

	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}
	if req.Bio != "" {
		user.Bio = req.Bio
	}

	if err := s.userRepo.Update(user); err != nil {
		return nil, errors.New("更新失败")
	}

	return toUserResponse(user), nil
}

// ChangePassword 修改密码
func (s *UserService) ChangePassword(userID uint, req ChangePasswordRequest) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	// 验证旧密码
	if !auth.CheckPassword(req.OldPassword, user.Password) {
		return errors.New("旧密码错误")
	}

	// 加密新密码
	hashedPassword, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return errors.New("密码加密失败")
	}

	user.Password = hashedPassword
	return s.userRepo.Update(user)
}

// ListUsers 获取用户列表（管理员）
func (s *UserService) ListUsers(page, limit int) ([]UserResponse, int64, error) {
	users, total, err := s.userRepo.List(page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]UserResponse, 0, len(users))
	for _, user := range users {
		responses = append(responses, *toUserResponse(&user))
	}

	return responses, total, nil
}

// UpdateUserStatus 管理员封禁或解封普通用户
func (s *UserService) UpdateUserStatus(adminUserID, userID uint, isActive bool) (*UserResponse, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, errors.New("获取用户失败")
	}

	if user.ID == adminUserID && !isActive {
		return nil, ErrCannotDisableSelf
	}
	if user.Role == "admin" {
		return nil, ErrCannotManageAdmin
	}

	user.IsActive = isActive
	if err := s.userRepo.Update(user); err != nil {
		return nil, errors.New("更新用户状态失败")
	}

	return toUserResponse(user), nil
}

// ResetUserPassword 管理员重置普通用户密码
func (s *UserService) ResetUserPassword(userID uint, newPassword string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return errors.New("获取用户失败")
	}
	if user.Role == "admin" {
		return ErrCannotManageAdmin
	}

	hashedPassword, err := auth.HashPassword(newPassword)
	if err != nil {
		return errors.New("密码加密失败")
	}

	user.Password = hashedPassword
	if err := s.userRepo.Update(user); err != nil {
		return errors.New("密码重置失败")
	}

	return nil
}
