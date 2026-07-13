package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blog-system/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UploadHandler 上传处理器
type UploadHandler struct {
	uploadDir string
	baseURL   string
}

// NewUploadHandler 创建上传处理器实例
func NewUploadHandler() *UploadHandler {
	// 创建上传目录
	uploadDir := "./uploads"
	os.MkdirAll(uploadDir, 0755)

	return &UploadHandler{
		uploadDir: uploadDir,
		baseURL:   "/uploads",
	}
}

// UploadImage 上传图片
// POST /api/v1/upload/image
func (h *UploadHandler) UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, 400, "请选择要上传的文件")
		return
	}

	// 检查文件大小（最大10MB）
	if file.Size > 10*1024*1024 {
		response.Error(c, 400, "文件大小不能超过10MB")
		return
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}

	if !allowedExts[ext] {
		response.Error(c, 400, "只支持 jpg、jpeg、png、gif、webp 格式的图片")
		return
	}

	// 生成唯一文件名
	dateDir := time.Now().Format("2006/01/02")
	saveDir := filepath.Join(h.uploadDir, dateDir)
	os.MkdirAll(saveDir, 0755)

	newFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	savePath := filepath.Join(saveDir, newFilename)

	// 保存文件
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		response.Error(c, 500, "文件保存失败")
		return
	}

	// 返回文件URL
	fileURL := fmt.Sprintf("%s/%s/%s", h.baseURL, dateDir, newFilename)

	response.SuccessWithMessage(c, "上传成功", gin.H{
		"url":      fileURL,
		"filename": file.Filename,
	})
}
