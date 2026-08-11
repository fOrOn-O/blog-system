package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"blog-system/internal/database"
	"blog-system/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupArticleServiceTest(t *testing.T) (*ArticleService, model.User, []model.Tag) {
	t.Helper()

	databaseName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", databaseName)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	database.DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Article{}, &model.Tag{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	user := model.User{
		Username: "writer",
		Email:    "writer@example.com",
		Password: "hashed-password",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}

	tags := []model.Tag{{Name: "Go"}, {Name: "Vue"}}
	if err := db.Create(&tags).Error; err != nil {
		t.Fatalf("create test tags: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return NewArticleService(), user, tags
}

func TestArticleServiceCreateAndUpdateCoverAndTags(t *testing.T) {
	articleService, user, tags := setupArticleServiceTest(t)
	listCacheKey := "articles:list:1:10:published"
	database.CacheSet(listCacheKey, "stale", time.Hour)

	created, err := articleService.Create(user.ID, CreateArticleRequest{
		Title:      "Monorepo deployment",
		Content:    "Article content",
		Summary:    "Initial summary",
		CoverImage: "/uploads/cover.png",
		TagIDs:     []uint{tags[0].ID, tags[1].ID},
	})
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	if created.CoverImage != "/uploads/cover.png" {
		t.Fatalf("unexpected cover image: %q", created.CoverImage)
	}
	if len(created.Tags) != 2 || created.Tags[0].ID != tags[0].ID || created.Tags[1].ID != tags[1].ID {
		t.Fatalf("unexpected created tags: %#v", created.Tags)
	}
	if _, err := database.CacheGet(listCacheKey); err == nil {
		t.Fatal("article list cache was not invalidated after create")
	}

	legacyCache := fmt.Sprintf(`{"articles":[{"id":%d,"tags":["Go"]}],"total":1}`, created.ID)
	database.CacheSet(listCacheKey, legacyCache, time.Hour)
	listed, total, err := articleService.List(1, 10, "published")
	if err != nil {
		t.Fatalf("list articles with legacy cache: %v", err)
	}
	if total != 1 || len(listed) != 1 || len(listed[0].Tags) != 2 {
		t.Fatalf("legacy cache did not fall back to database: total=%d articles=%#v", total, listed)
	}

	emptyCover := ""
	emptySummary := ""
	replacementTagIDs := []uint{tags[1].ID}
	updated, err := articleService.Update(user.ID, created.ID, UpdateArticleRequest{
		Summary:    &emptySummary,
		CoverImage: &emptyCover,
		TagIDs:     &replacementTagIDs,
	})
	if err != nil {
		t.Fatalf("update article: %v", err)
	}
	if updated.CoverImage != "" {
		t.Fatalf("cover image was not cleared: %q", updated.CoverImage)
	}
	if updated.Summary != "" {
		t.Fatalf("summary was not cleared: %q", updated.Summary)
	}
	if len(updated.Tags) != 1 || updated.Tags[0].ID != tags[1].ID || updated.Tags[0].Name != tags[1].Name {
		t.Fatalf("unexpected updated tags: %#v", updated.Tags)
	}

	var associationCount int64
	if err := database.DB.Table("article_tags").Where("article_id = ?", created.ID).Count(&associationCount).Error; err != nil {
		t.Fatalf("count article tags: %v", err)
	}
	if associationCount != 1 {
		t.Fatalf("expected one article tag association, got %d", associationCount)
	}
}

func TestArticleServiceRejectsUnknownTagIDs(t *testing.T) {
	articleService, user, _ := setupArticleServiceTest(t)

	var before int64
	if err := database.DB.Model(&model.Article{}).Count(&before).Error; err != nil {
		t.Fatalf("count articles before create: %v", err)
	}

	_, err := articleService.Create(user.ID, CreateArticleRequest{
		Title:   "Invalid tag",
		Content: "Article content",
		TagIDs:  []uint{999999},
	})
	if err == nil {
		t.Fatal("expected unknown tag ID to be rejected")
	}

	var after int64
	if err := database.DB.Model(&model.Article{}).Count(&after).Error; err != nil {
		t.Fatalf("count articles after create: %v", err)
	}
	if after != before {
		t.Fatalf("invalid article was persisted: before=%d after=%d", before, after)
	}
}
