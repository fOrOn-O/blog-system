package middleware

import (
	"errors"
	"strings"

	"blog-system/internal/model"
	"blog-system/internal/repository"
	"blog-system/pkg/auth"
	"blog-system/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const currentUserKey = "current_user"

// AuthRequired JWT认证中间件
func AuthRequired() gin.HandlerFunc {
	userRepo := repository.NewUserRepository()

	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			response.Unauthorized(c, "未提供认证token")
			c.Abort()
			return
		}

		// 移除Bearer前缀
		if strings.HasPrefix(token, "Bearer ") {
			token = token[7:]
		}

		claims, err := auth.ValidateToken(token)
		if err != nil {
			response.Unauthorized(c, "无效的token")
			c.Abort()
			return
		}

		user, err := userRepo.FindByID(claims.UserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				response.Unauthorized(c, "用户不存在或登录已失效")
			} else {
				response.InternalError(c, "认证状态检查失败")
			}
			c.Abort()
			return
		}

		if !user.IsActive {
			response.Unauthorized(c, "账号已被禁用")
			c.Abort()
			return
		}

		// 使用数据库中的最新身份覆盖JWT快照，避免角色变更后继续沿用旧权限。
		claims.Username = user.Username
		claims.Role = user.Role
		c.Set("claims", claims)
		c.Set(currentUserKey, user)
		c.Next()
	}
}

// AdminRequired 管理员权限中间件
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUser, exists := c.Get(currentUserKey)
		if !exists {
			response.Unauthorized(c, "未认证")
			c.Abort()
			return
		}

		user, ok := currentUser.(*model.User)
		if !ok || user.Role != "admin" {
			response.Forbidden(c, "需要管理员权限")
			c.Abort()
			return
		}

		c.Next()
	}
}

// OptionalAuth 可选认证中间件（不强制要求token）
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.Next()
			return
		}

		if strings.HasPrefix(token, "Bearer ") {
			token = token[7:]
		}

		claims, err := auth.ValidateToken(token)
		if err == nil {
			c.Set("claims", claims)
		}

		c.Next()
	}
}
