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

func (h *ArticleHandler) GetVersionDiff(c *gin.Context) {
	parse := func(value string) (uint, error) {
		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil || n == 0 {
			return 0, service.ErrInvalidDiffVersions
		}
		return uint(n), nil
	}
	id, idErr := parse(c.Param("id"))
	from, fromErr := parse(c.Query("from_version"))
	to, toErr := parse(c.Query("to_version"))
	if idErr != nil || fromErr != nil || toErr != nil {
		response.BadRequest(c, "文章 ID 和版本号必须为正整数")
		return
	}
	claims := c.MustGet("claims").(*auth.Claims)
	diff, err := h.articleService.GetVersionDiff(claims.UserID, id, from, to)
	switch {
	case err == nil:
		response.Success(c, diff)
	case errors.Is(err, service.ErrInvalidDiffVersions):
		response.BadRequest(c, err.Error())
	case errors.Is(err, model.ErrArticleNotFound), errors.Is(err, model.ErrArticleVersionNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, model.ErrArticleForbidden):
		response.Forbidden(c, err.Error())
	default:
		response.InternalError(c, "比较文章版本失败")
	}
}
