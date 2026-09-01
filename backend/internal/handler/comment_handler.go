package handler

import (
	"strconv"

	"blog-system/internal/service"
	"blog-system/pkg/auth"
	"blog-system/pkg/response"

	"github.com/gin-gonic/gin"
)

// CommentHandler 评论处理器
type CommentHandler struct {
	commentService *service.CommentService
}

// NewCommentHandler 创建评论处理器
func NewCommentHandler() *CommentHandler {
	return &CommentHandler{
		commentService: service.NewCommentService(),
	}
}

// Create 创建评论
// POST /api/v1/articles/:id/comments
func (h *CommentHandler) Create(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	articleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}

	var req service.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "无效的请求数据: "+err.Error())
		return
	}

	comment, err := h.commentService.Create(claims.UserID, uint(articleID), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, comment)
}

// GetByArticleID 获取文章的评论列表
// GET /api/v1/articles/:id/comments
func (h *CommentHandler) GetByArticleID(c *gin.Context) {
	articleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的文章ID")
		return
	}

	page, limit := getPagination(c)

	comments, total, err := h.commentService.GetByArticleID(uint(articleID), page, limit)
	if err != nil {
		response.InternalError(c, "获取评论失败")
		return
	}

	response.Paginated(c, comments, response.Meta{
		Page:  page,
		Limit: limit,
		Total: total,
		Pages: (total + int64(limit) - 1) / int64(limit),
	})
}

// Delete 删除评论
// DELETE /api/v1/comments/:id
func (h *CommentHandler) Delete(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)

	commentID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的评论ID")
		return
	}

	if err := h.commentService.Delete(claims.UserID, claims.Role, uint(commentID)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "评论删除成功"})
}
