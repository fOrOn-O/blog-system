package handler

import (
	"blog-system/pkg/response"
	"github.com/gin-gonic/gin"
	"strconv"
)

// 登录用户可读全站公开知识；未公开、已删除和不存在的文章统一不返回记录。
func (h *ArticleHandler) PublishedKnowledge(c *gin.Context) {
	var id uint64
	if raw := c.Param("id"); raw != "" {
		var err error
		id, err = strconv.ParseUint(raw, 10, 32)
		if err != nil || id == 0 {
			response.BadRequest(c, "文章 ID 必须为正整数")
			return
		}
	}
	records, err := h.articleService.PublishedKnowledge(uint(id))
	if err != nil {
		response.InternalError(c, "读取公开知识源失败")
		return
	}
	response.Success(c, records)
}
