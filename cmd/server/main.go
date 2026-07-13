package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"blog-system/internal/config"
	"blog-system/internal/database"
	"blog-system/internal/model"
	"blog-system/internal/router"
	"blog-system/pkg/auth"
)

func main() {
	// 1. 初始化配置
	config.InitConfig()

	// 2. 初始化数据库
	database.InitDatabase()

	// 3. 自动迁移数据库表
	if err := database.AutoMigrate(
		&model.User{},
		&model.Article{},
		&model.Tag{},
		&model.Comment{},
		&model.Like{},
		&model.Favorite{},
	); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	log.Println("数据库迁移完成")

	// 4. 初始化Redis（可选）
	database.InitRedis()

	// 5. 创建默认管理员账号
	createDefaultAdmin()

	// 6. 配置路由
	r := router.SetupRouter()

	// 7. 启动服务器
	port := config.AppConfig.App.Port
	if port == "" {
		port = ":8080"
	}

	srv := &http.Server{
		Addr:    port,
		Handler: r,
	}

	// 在goroutine中启动服务器
	go func() {
		fmt.Printf("===================================\n")
		fmt.Printf("  Blog System API Server\n")
		fmt.Printf("  Port: %s\n", port)
		fmt.Printf("  Mode: %s\n", config.AppConfig.App.Mode)
		fmt.Printf("===================================\n")
		fmt.Printf("  Health: http://localhost%s/health\n", port)
		fmt.Printf("  API:    http://localhost%s/api/v1\n", port)
		fmt.Printf("===================================\n")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 8. 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("服务器关闭错误: %v", err)
	}

	// 关闭数据库连接
	database.CloseDatabase()
	database.CloseRedis()

	log.Println("服务器已安全关闭")
}

// createDefaultAdmin 创建默认管理员账号
func createDefaultAdmin() {
	db := database.DB

	var count int64
	db.Model(&model.User{}).Where("role = ?", "admin").Count(&count)
	if count > 0 {
		return
	}

	hashedPassword, err := auth.HashPassword("admin123456")
	if err != nil {
		log.Printf("创建默认管理员失败: %v", err)
		return
	}

	admin := &model.User{
		Username: "admin",
		Email:    "admin@blog.com",
		Password: hashedPassword,
		Role:     "admin",
		IsActive: true,
	}

	if err := db.Create(admin).Error; err != nil {
		log.Printf("创建默认管理员失败: %v", err)
		return
	}

	log.Println("默认管理员账号已创建: admin / admin123456")
}
