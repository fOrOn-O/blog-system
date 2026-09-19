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

func (h *ArticleHandler) GetVersionChunks(c *gin.Context) {
	id, idErr := strconv.ParseUint(c.Param("id"), 10, 32)
	version, versionErr := strconv.ParseUint(c.Param("version"), 10, 32)
	if idErr != nil || versionErr != nil || id == 0 || version == 0 {
		response.BadRequest(c, service.ErrInvalidChunkVersion.Error())
		return
	}
	claims := c.MustGet("claims").(*auth.Claims)
	chunks, err := h.articleService.GetVersionChunks(claims.UserID, uint(id), uint(version))
	switch {
	case err == nil:
		response.Success(c, chunks)
	case errors.Is(err, model.ErrArticleNotFound), errors.Is(err, model.ErrArticleVersionNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, model.ErrArticleForbidden):
		response.Forbidden(c, err.Error())
	default:
		response.InternalError(c, "读取文章版本分块失败")
	}
}
