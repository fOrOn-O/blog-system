package handler

import (
	"strconv"

	"blog-system/internal/service"
	"blog-system/pkg/auth"
	"blog-system/pkg/response"

	"github.com/gin-gonic/gin"
)

// ArticleHandler 文章处理器
type ArticleHandler struct {
	articleService *service.ArticleService
}

// NewArticleHandler 创建文章处理器
func NewArticleHandler() *ArticleHandler {
	return &ArticleHandler{
		articleService: service.NewArticleService(),
	}
}

// Create 创建文章
// POST /api/v1/articles
func (h *ArticleHandler) Create(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	var req service.CreateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "无效的请求数据: "+err.Error())
		return
	}

	article, err := h.articleService.Create(claims.UserID, req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Created(c, article)
}

// GetByID 获取文章详情
// GET /api/v1/articles/:id
func (h *ArticleHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}

	article, err := h.articleService.GetByID(uint(id))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, article)
}

// List 获取文章列表
// GET /api/v1/articles
func (h *ArticleHandler) List(c *gin.Context) {
	page, limit := getPagination(c)
	status := c.DefaultQuery("status", "published")

	articles, total, err := h.articleService.List(page, limit, status)
	if err != nil {
		response.InternalError(c, "获取文章列表失败")
		return
	}

	response.Paginated(c, articles, response.Meta{
		Page:  page,
		Limit: limit,
		Total: total,
		Pages: (total + int64(limit) - 1) / int64(limit),
	})
}

// Search 搜索文章
// GET /api/v1/articles/search
func (h *ArticleHandler) Search(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		response.BadRequest(c, "请输入搜索关键词")
		return
	}

	page, limit := getPagination(c)

	articles, total, err := h.articleService.Search(keyword, page, limit)
	if err != nil {
		response.InternalError(c, "搜索失败")
		return
	}

	response.Paginated(c, articles, response.Meta{
		Page:  page,
		Limit: limit,
		Total: total,
		Pages: (total + int64(limit) - 1) / int64(limit),
	})
}

// Update 更新文章
// PUT /api/v1/articles/:id
func (h *ArticleHandler) Update(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}

	var req service.UpdateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "无效的请求数据: "+err.Error())
		return
	}

	article, err := h.articleService.Update(claims.UserID, uint(id), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, article)
}

// Delete 删除文章
// DELETE /api/v1/articles/:id
func (h *ArticleHandler) Delete(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}

	if err := h.articleService.Delete(claims.UserID, uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "文章删除成功"})
}
