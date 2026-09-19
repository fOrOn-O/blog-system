package router_test

import (
	"blog-system/internal/database"
	"blog-system/internal/model"
	"blog-system/pkg/auth"
	"net/http"
	"os"
	"testing"
)

// 仅显式 E2E 命令运行；使用 setupWorkflowHTTP 的独立内存库，不加载本地或生产配置。
func TestTask13E2EServer(t *testing.T) {
	if os.Getenv("TASK13_E2E") != "1" {
		t.Skip("explicit browser E2E fixture only")
	}
	h, _, _ := setupWorkflowHTTP(t)
	password, err := auth.HashPassword("e2e-test-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(&model.User{}).Where("username = ?", "user0").Update("password", password).Error; err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Addr: "127.0.0.1:18080", Handler: h}
	t.Cleanup(func() { _ = server.Close() })
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		t.Fatal(err)
	}
}
