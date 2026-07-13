package handler

import (
	"strconv"

	"blog-system/internal/service"
	"blog-system/pkg/response"

	"github.com/gin-gonic/gin"
)

// TagHandler 标签处理器
type TagHandler struct {
	tagService *service.TagService
}

// NewTagHandler 创建标签处理器实例
func NewTagHandler() *TagHandler {
	return &TagHandler{
		tagService: service.NewTagService(),
	}
}

// GetAll 获取所有标签
// GET /api/v1/tags
func (h *TagHandler) GetAll(c *gin.Context) {
	tags, err := h.tagService.GetAll()
	if err != nil {
		response.Error(c, 500, "获取标签列表失败")
		return
	}

	response.Success(c, tags)
}

// GetByID 根据ID获取标签
// GET /api/v1/tags/:id
func (h *TagHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的标签ID")
		return
	}

	tag, err := h.tagService.GetByID(uint(id))
	if err != nil {
		response.Error(c, 404, "标签不存在")
		return
	}

	response.Success(c, tag)
}

// Create 创建标签（管理员）
// POST /api/v1/tags
func (h *TagHandler) Create(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required,min=1,max=50"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "请输入标签名称")
		return
	}

	tag, err := h.tagService.Create(req.Name)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.SuccessWithMessage(c, "创建成功", tag)
}

// Update 更新标签（管理员）
// PUT /api/v1/tags/:id
func (h *TagHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的标签ID")
		return
	}

	var req struct {
		Name string `json:"name" binding:"required,min=1,max=50"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "请输入标签名称")
		return
	}

	tag, err := h.tagService.Update(uint(id), req.Name)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.SuccessWithMessage(c, "更新成功", tag)
}

// Delete 删除标签（管理员）
// DELETE /api/v1/tags/:id
func (h *TagHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, 400, "无效的标签ID")
		return
	}

	if err := h.tagService.Delete(uint(id)); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}
