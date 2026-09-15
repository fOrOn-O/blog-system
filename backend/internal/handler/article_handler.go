package handler

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"

	"blog-system/internal/model"
	"blog-system/internal/service"
	"blog-system/pkg/auth"
	"blog-system/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
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
	if err := bindArticleRequest(c, &req); err != nil {
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
	articles, total, err := h.articleService.List(page, limit, model.ArticleStatusPublished)
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
	if err := bindArticleRequest(c, &req); err != nil {
		response.BadRequest(c, "无效的请求数据: "+err.Error())
		return
	}

	article, err := h.articleService.Update(claims.UserID, uint(id), req)
	if err != nil {
		articleBusinessError(c, err)
		return
	}

	response.Success(c, article)
}

// 仅对文章内容接口严格解析，防止状态或发布指针作为隐藏参数传入。
func bindArticleRequest(c *gin.Context, req interface{}) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(req); err != nil {
		return err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("请求必须只包含一个 JSON 对象")
	}
	return binding.Validator.ValidateStruct(req)
}

func articleBusinessError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrArticleNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, model.ErrArticleForbidden):
		response.Forbidden(c, err.Error())
	case errors.Is(err, model.ErrVersionConflict):
		response.Conflict(c, err.Error())
	case errors.Is(err, model.ErrArticleArchivedEdit), errors.Is(err, model.ErrArticleArchivedPublish), errors.Is(err, model.ErrArticleSnapshotMissing):
		response.Conflict(c, err.Error())
	default:
		response.BadRequest(c, err.Error())
	}
}

func (h *ArticleHandler) CreateDraft(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	var req service.CreateDraftRequest
	if err := bindArticleRequest(c, &req); err != nil {
		response.BadRequest(c, "无效的请求数据: "+err.Error())
		return
	}
	article, err := h.articleService.CreateDraft(claims.UserID, req)
	if err != nil {
		articleBusinessError(c, err)
		return
	}
	response.Created(c, article)
}

func (h *ArticleHandler) UpdateDraft(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}
	var req service.UpdateDraftRequest
	if err := bindArticleRequest(c, &req); err != nil {
		response.BadRequest(c, "无效的请求数据: "+err.Error())
		return
	}
	article, err := h.articleService.UpdateDraft(claims.UserID, uint(id), req)
	if err != nil {
		articleBusinessError(c, err)
		return
	}
	response.Success(c, article)
}

func (h *ArticleHandler) PublishArticle(c *gin.Context) {
	var req service.PublishArticleRequest
	if err := bindArticleRequest(c, &req); err != nil {
		response.BadRequest(c, "无效的请求数据: "+err.Error())
		return
	}
	h.ownedArticleAction(c, func(userID, articleID uint) (*service.ArticleResponse, error) {
		return h.articleService.PublishArticle(userID, articleID, req)
	})
}

func (h *ArticleHandler) ArchiveArticle(c *gin.Context) {
	h.ownedArticleAction(c, h.articleService.ArchiveArticle)
}

func (h *ArticleHandler) GetOwnedArticle(c *gin.Context) {
	h.ownedArticleAction(c, h.articleService.GetOwnedArticle)
}

func (h *ArticleHandler) ownedArticleAction(c *gin.Context, action func(uint, uint) (*service.ArticleResponse, error)) {
	claims := c.MustGet("claims").(*auth.Claims)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}
	article, err := action(claims.UserID, uint(id))
	if err != nil {
		articleBusinessError(c, err)
		return
	}
	response.Success(c, article)
}

func (h *ArticleHandler) ListMyArticles(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	page, limit := getPagination(c)
	articles, total, err := h.articleService.ListMyArticles(claims.UserID, page, limit, c.Query("status"))
	if err != nil {
		articleBusinessError(c, err)
		return
	}
	response.Paginated(c, articles, response.Meta{
		Page: page, Limit: limit, Total: total,
		Pages: (total + int64(limit) - 1) / int64(limit),
	})
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

	if err := h.articleService.Delete(claims.UserID, claims.Role, uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "文章删除成功"})
}
