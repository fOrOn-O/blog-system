package handler

import (
	"blog-system/internal/service"
	"blog-system/pkg/auth"
	"blog-system/pkg/response"

	"github.com/gin-gonic/gin"
)

// UserHandler 用户处理器
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler 创建用户处理器
func NewUserHandler() *UserHandler {
	return &UserHandler{
		userService: service.NewUserService(),
	}
}

// GetProfile 获取个人信息
// GET /api/v1/user/profile
func (h *UserHandler) GetProfile(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	user, err := h.userService.GetProfile(claims.UserID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, user)
}

// UpdateProfile 更新个人信息
// PUT /api/v1/user/profile
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	var req service.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "无效的请求数据: "+err.Error())
		return
	}

	user, err := h.userService.UpdateProfile(claims.UserID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, user)
}

// ChangePassword 修改密码
// PUT /api/v1/user/password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	var req service.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "无效的请求数据: "+err.Error())
		return
	}

	if err := h.userService.ChangePassword(claims.UserID, req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "密码修改成功"})
}

// ListUsers 获取用户列表（管理员）
// GET /api/v1/admin/users
func (h *UserHandler) ListUsers(c *gin.Context) {
	page, limit := getPagination(c)

	users, total, err := h.userService.ListUsers(page, limit)
	if err != nil {
		response.InternalError(c, "获取用户列表失败")
		return
	}

	response.Paginated(c, users, response.Meta{
		Page:  page,
		Limit: limit,
		Total: total,
		Pages: (total + int64(limit) - 1) / int64(limit),
	})
}
