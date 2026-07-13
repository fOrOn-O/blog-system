package handler

import (
	"strconv"

	"blog-system/pkg/auth"

	"github.com/gin-gonic/gin"
)

// getPagination 从查询参数获取分页信息
func getPagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	return page, limit
}

// getCurrentUserID 从上下文获取当前用户ID
func getCurrentUserID(c *gin.Context) uint {
	if claims, exists := c.Get("claims"); exists {
		return claims.(*auth.Claims).UserID
	}
	return 0
}
