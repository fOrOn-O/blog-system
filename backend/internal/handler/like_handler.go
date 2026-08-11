package handler

import (
	"strconv"

	"blog-system/internal/service"
	"blog-system/pkg/auth"
	"blog-system/pkg/response"

	"github.com/gin-gonic/gin"
)

// LikeHandler 点赞处理器
type LikeHandler struct {
	likeService *service.LikeService
}

// NewLikeHandler 创建点赞处理器
func NewLikeHandler() *LikeHandler {
	return &LikeHandler{
		likeService: service.NewLikeService(),
	}
}

// Like 点赞文章
// POST /api/v1/articles/:id/like
func (h *LikeHandler) Like(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	articleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}

	if err := h.likeService.Like(claims.UserID, uint(articleID)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "点赞成功"})
}

// Unlike 取消点赞
// DELETE /api/v1/articles/:id/like
func (h *LikeHandler) Unlike(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	articleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}

	if err := h.likeService.Unlike(claims.UserID, uint(articleID)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "取消点赞成功"})
}

// GetLikeInfo 获取点赞信息
// GET /api/v1/articles/:id/likes
func (h *LikeHandler) GetLikeInfo(c *gin.Context) {
	articleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}

	// 获取当前用户ID（未登录为0）
	var userID uint
	if claims, exists := c.Get("claims"); exists {
		userID = claims.(*auth.Claims).UserID
	}

	likeInfo := h.likeService.GetLikeInfo(userID, uint(articleID))
	response.Success(c, likeInfo)
}
