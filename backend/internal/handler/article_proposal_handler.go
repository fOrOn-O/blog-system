package handler

import (
	"errors"
	"strconv"

	"blog-system/internal/model"
	"blog-system/internal/service"
	"blog-system/pkg/auth"
	"blog-system/pkg/response"

	"github.com/gin-gonic/gin"
)

func (h *ArticleHandler) PreviewArticleEdit(c *gin.Context) {
	h.articleEditAction(c, false)
}

func (h *ArticleHandler) ApplyArticleEdit(c *gin.Context) {
	h.articleEditAction(c, true)
}

// 仅由认证应用操作调用，未注册为 Agent 工具。
func (h *ArticleHandler) articleEditAction(c *gin.Context, apply bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.BadRequest(c, "文章 ID 必须为正整数")
		return
	}
	var req service.ArticleEditRequest
	if err := bindArticleRequest(c, &req); err != nil {
		response.BadRequest(c, "无效的请求数据: "+err.Error())
		return
	}
	claims := c.MustGet("claims").(*auth.Claims)
	var result interface{}
	if apply {
		result, err = h.articleService.ApplyArticleEdit(claims.UserID, uint(id), req)
	} else {
		result, err = h.articleService.PreviewArticleEdit(claims.UserID, uint(id), req)
	}
	switch {
	case err == nil:
		response.Success(c, result)
	case errors.Is(err, model.ErrArticleNotFound), errors.Is(err, model.ErrArticleVersionNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, model.ErrArticleForbidden):
		response.Forbidden(c, err.Error())
	case errors.Is(err, model.ErrVersionConflict), errors.Is(err, model.ErrArticleArchivedEdit):
		response.Conflict(c, err.Error())
	case errors.Is(err, model.ErrInvalidArticleContent), errors.Is(err, model.ErrArticleContentUnchanged), errors.Is(err, service.ErrInvalidArticleVersion):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, "处理文章编辑提案失败")
	}
}
