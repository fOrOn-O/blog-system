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

func (h *ArticleHandler) GetOwnedVersion(c *gin.Context) {
	id, idErr := strconv.ParseUint(c.Param("id"), 10, 32)
	version, versionErr := strconv.ParseUint(c.Param("version"), 10, 32)
	if idErr != nil || versionErr != nil || id == 0 || version == 0 {
		response.BadRequest(c, service.ErrInvalidArticleVersion.Error())
		return
	}
	claims := c.MustGet("claims").(*auth.Claims)
	result, err := h.articleService.GetOwnedVersion(claims.UserID, uint(id), uint(version))
	switch {
	case err == nil:
		response.Success(c, result)
	case errors.Is(err, model.ErrArticleNotFound), errors.Is(err, model.ErrArticleVersionNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, model.ErrArticleForbidden):
		response.Forbidden(c, err.Error())
	default:
		response.InternalError(c, "读取文章历史版本失败")
	}
}

func (h *ArticleHandler) ListOwnedVersions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.BadRequest(c, "文章 ID 必须为正整数")
		return
	}
	claims := c.MustGet("claims").(*auth.Claims)
	page, limit := getPagination(c)
	versions, total, err := h.articleService.ListOwnedVersions(claims.UserID, uint(id), page, limit)
	switch {
	case err == nil:
		response.Paginated(c, versions, response.Meta{Page: page, Limit: limit, Total: total, Pages: (total + int64(limit) - 1) / int64(limit)})
	case errors.Is(err, model.ErrArticleNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, model.ErrArticleForbidden):
		response.Forbidden(c, err.Error())
	default:
		response.InternalError(c, "读取版本列表失败")
	}
}
