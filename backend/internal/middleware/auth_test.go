package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"blog-system/internal/config"
	"blog-system/internal/database"
	"blog-system/internal/model"
	"blog-system/pkg/auth"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthMiddlewareTest(t *testing.T) *gorm.DB {
	t.Helper()

	databaseName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", databaseName)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	database.DB = db
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	previousConfig := config.AppConfig
	config.AppConfig = &config.Config{JWT: config.JWT{Secret: "middleware-test-secret", Expiration: 1}}
	t.Cleanup(func() { config.AppConfig = previousConfig })

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gin.SetMode(gin.TestMode)
	return db
}

func authenticatedRequest(t *testing.T, router http.Handler, path, token string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	responseRecorder := httptest.NewRecorder()
	router.ServeHTTP(responseRecorder, request)
	return responseRecorder
}

func TestAuthRequiredRejectsDisabledUserWithPreviouslyIssuedToken(t *testing.T) {
	db := setupAuthMiddlewareTest(t)
	user := model.User{Username: "member", Email: "member@example.com", Password: "hashed", Role: "user", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := auth.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	router := gin.New()
	router.GET("/protected", AuthRequired(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	if responseRecorder := authenticatedRequest(t, router, "/protected", token); responseRecorder.Code != http.StatusNoContent {
		t.Fatalf("active user was rejected: status=%d body=%s", responseRecorder.Code, responseRecorder.Body.String())
	}

	if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}

	responseRecorder := authenticatedRequest(t, router, "/protected", token)
	if responseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("disabled user kept protected access: status=%d body=%s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !strings.Contains(responseRecorder.Body.String(), "账号已被禁用") {
		t.Fatalf("unexpected disabled-user response: %s", responseRecorder.Body.String())
	}
}

func TestAdminRequiredUsesCurrentDatabaseRole(t *testing.T) {
	db := setupAuthMiddlewareTest(t)
	user := model.User{Username: "ordinary", Email: "ordinary@example.com", Password: "hashed", Role: "user", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 模拟角色变更前签发的管理员JWT；中间件必须以数据库当前角色为准。
	token, err := auth.GenerateToken(user.ID, user.Username, "admin")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	router := gin.New()
	router.GET("/admin", AuthRequired(), AdminRequired(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	responseRecorder := authenticatedRequest(t, router, "/admin", token)
	if responseRecorder.Code != http.StatusForbidden {
		t.Fatalf("ordinary database user gained admin access: status=%d body=%s", responseRecorder.Code, responseRecorder.Body.String())
	}
}

func TestAuthRequiredRejectsDeletedUser(t *testing.T) {
	db := setupAuthMiddlewareTest(t)
	user := model.User{Username: "deleted", Email: "deleted@example.com", Password: "hashed", Role: "user", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := auth.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if err := db.Delete(&user).Error; err != nil {
		t.Fatalf("delete user: %v", err)
	}

	router := gin.New()
	router.GET("/protected", AuthRequired(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	responseRecorder := authenticatedRequest(t, router, "/protected", token)
	if responseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("deleted user kept protected access: status=%d body=%s", responseRecorder.Code, responseRecorder.Body.String())
	}
}
