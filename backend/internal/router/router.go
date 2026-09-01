package router

import (
	"blog-system/internal/handler"
	"blog-system/internal/middleware"
	"blog-system/pkg/response"

	"github.com/gin-gonic/gin"
)

// SetupRouter 配置路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// 全局中间件
	r.Use(middleware.CORS())
	r.Use(middleware.Logger())

	// 静态文件服务（上传的文件）
	r.Static("/uploads", "./uploads")

	// 初始化处理器
	authHandler := handler.NewAuthHandler()
	userHandler := handler.NewUserHandler()
	articleHandler := handler.NewArticleHandler()
	commentHandler := handler.NewCommentHandler()
	likeHandler := handler.NewLikeHandler()
	favoriteHandler := handler.NewFavoriteHandler()
	tagHandler := handler.NewTagHandler()
	uploadHandler := handler.NewUploadHandler()

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		response.Success(c, gin.H{
			"status":  "ok",
			"message": "Blog System API is running",
		})
	})

	// API v1
	api := r.Group("/api/v1")
	{
		// 认证路由（无需登录）
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}

		// 标签公开路由
		api.GET("/tags", tagHandler.GetAll)
		api.GET("/tags/:id", tagHandler.GetByID)

		// 文章公开路由
		articles := api.Group("/articles")
		{
			articles.GET("", articleHandler.List)
			articles.GET("/search", articleHandler.Search)
			articles.GET("/:id", articleHandler.GetByID)
			articles.GET("/:id/comments", commentHandler.GetByArticleID)

			// 可选认证路由（登录用户可以看到自己的点赞状态）
			articles.GET("/:id/likes", middleware.OptionalAuth(), likeHandler.GetLikeInfo)
		}

		// 需要认证的路由
		protected := api.Group("")
		protected.Use(middleware.AuthRequired())
		{
			// 用户路由
			user := protected.Group("/user")
			{
				user.GET("/profile", userHandler.GetProfile)
				user.PUT("/profile", userHandler.UpdateProfile)
				user.PUT("/password", userHandler.ChangePassword)
				user.GET("/favorites", favoriteHandler.GetUserFavorites)
			}

			// 文章操作路由
			protected.POST("/articles", articleHandler.Create)
			protected.PUT("/articles/:id", articleHandler.Update)
			protected.DELETE("/articles/:id", articleHandler.Delete)

			// 点赞路由
			protected.POST("/articles/:id/like", likeHandler.Like)
			protected.DELETE("/articles/:id/like", likeHandler.Unlike)

			// 评论路由
			protected.POST("/articles/:id/comments", commentHandler.Create)
			protected.DELETE("/comments/:id", commentHandler.Delete)

			// 收藏路由
			protected.POST("/articles/:id/favorite", favoriteHandler.Favorite)
			protected.DELETE("/articles/:id/favorite", favoriteHandler.Unfavorite)
			protected.GET("/articles/:id/favorite", favoriteHandler.IsFavorited)

			// 图片上传路由
			protected.POST("/upload/image", uploadHandler.UploadImage)
		}

		// 管理员路由
		admin := api.Group("/admin")
		admin.Use(middleware.AuthRequired(), middleware.AdminRequired())
		{
			admin.GET("/users", userHandler.ListUsers)
			admin.PUT("/users/:id/status", userHandler.UpdateUserStatus)
			admin.PUT("/users/:id/password", userHandler.ResetUserPassword)

			// 标签管理路由（管理员）
			admin.POST("/tags", tagHandler.Create)
			admin.PUT("/tags/:id", tagHandler.Update)
			admin.DELETE("/tags/:id", tagHandler.Delete)
		}
	}

	return r
}
