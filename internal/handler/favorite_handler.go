package handler

import (
	"strconv"

	"blog-system/internal/service"
	"blog-system/pkg/auth"
	"blog-system/pkg/response"

	"github.com/gin-gonic/gin"
)

// FavoriteHandler 收藏处理器
type FavoriteHandler struct {
	favoriteService *service.FavoriteService
}

// NewFavoriteHandler 创建收藏处理器
func NewFavoriteHandler() *FavoriteHandler {
	return &FavoriteHandler{
		favoriteService: service.NewFavoriteService(),
	}
}

// Favorite 收藏文章
// POST /api/v1/articles/:id/favorite
func (h *FavoriteHandler) Favorite(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	articleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}

	if err := h.favoriteService.Favorite(claims.UserID, uint(articleID)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "收藏成功"})
}

// Unfavorite 取消收藏
// DELETE /api/v1/articles/:id/favorite
func (h *FavoriteHandler) Unfavorite(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	articleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}

	if err := h.favoriteService.Unfavorite(claims.UserID, uint(articleID)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "取消收藏成功"})
}

// IsFavorited 检查是否已收藏
// GET /api/v1/articles/:id/favorite
func (h *FavoriteHandler) IsFavorited(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	articleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}

	result := h.favoriteService.IsFavorited(claims.UserID, uint(articleID))
	response.Success(c, result)
}

// GetUserFavorites 获取用户收藏列表
// GET /api/v1/user/favorites
func (h *FavoriteHandler) GetUserFavorites(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	page, limit := getPagination(c)

	favorites, total, err := h.favoriteService.GetUserFavorites(claims.UserID, page, limit)
	if err != nil {
		response.InternalError(c, "获取收藏列表失败")
		return
	}

	response.Paginated(c, favorites, response.Meta{
		Page:  page,
		Limit: limit,
		Total: total,
		Pages: (total + int64(limit) - 1) / int64(limit),
	})
}
