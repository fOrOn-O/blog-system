package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"blog-system/internal/database"
	"blog-system/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCommentServiceReturnsEmptyArrayWhenArticleHasNoComments(t *testing.T) {
	databaseName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", databaseName)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	database.DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Article{}, &model.Comment{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	comments, total, err := NewCommentService().GetByArticleID(1, 1, 10)
	if err != nil {
		t.Fatalf("get comments: %v", err)
	}
	if comments == nil {
		t.Fatal("expected an empty comment slice, got nil")
	}
	if len(comments) != 0 || total != 0 {
		t.Fatalf("expected no comments, got total=%d comments=%#v", total, comments)
	}

	encoded, err := json.Marshal(comments)
	if err != nil {
		t.Fatalf("marshal comments: %v", err)
	}
	if string(encoded) != "[]" {
		t.Fatalf("expected JSON empty array, got %s", encoded)
	}
}

func TestCommentServiceDeleteAuthorization(t *testing.T) {
	databaseName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", databaseName)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	database.DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Article{}, &model.Comment{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	owner := model.User{Username: "commenter", Email: "commenter@example.com", Password: "hashed-password", Role: "user", IsActive: true}
	otherUser := model.User{Username: "other", Email: "other@example.com", Password: "hashed-password", Role: "user", IsActive: true}
	admin := model.User{Username: "comment-admin", Email: "comment-admin@example.com", Password: "hashed-password", Role: "admin", IsActive: true}
	for _, user := range []*model.User{&owner, &otherUser, &admin} {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create user %q: %v", user.Username, err)
		}
	}

	article := model.Article{Title: "comments", Content: "content", UserID: owner.ID, Status: "published"}
	if err := db.Create(&article).Error; err != nil {
		t.Fatalf("create article: %v", err)
	}

	commentService := NewCommentService()
	createComment := func(content string) *CommentResponse {
		t.Helper()
		comment, err := commentService.Create(owner.ID, article.ID, CreateCommentRequest{Content: content})
		if err != nil {
			t.Fatalf("create comment %q: %v", content, err)
		}
		if comment.UserID != owner.ID {
			t.Fatalf("comment response omitted owner ID: %#v", comment)
		}
		return comment
	}

	t.Run("author can delete own comment", func(t *testing.T) {
		comment := createComment("author delete")
		if err := commentService.Delete(owner.ID, "user", comment.ID); err != nil {
			t.Fatalf("author delete own comment: %v", err)
		}
		if _, err := commentService.commentRepo.FindByID(comment.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("expected comment to be deleted, got %v", err)
		}
	})

	t.Run("ordinary user cannot delete another user's comment", func(t *testing.T) {
		comment := createComment("unauthorized delete")
		if err := commentService.Delete(otherUser.ID, "user", comment.ID); err == nil {
			t.Fatal("expected deleting another user's comment to be rejected")
		}
		if _, err := commentService.commentRepo.FindByID(comment.ID); err != nil {
			t.Fatalf("comment was unexpectedly deleted: %v", err)
		}
	})

	t.Run("admin can delete another user's comment", func(t *testing.T) {
		comment := createComment("admin delete")
		if err := commentService.Delete(admin.ID, "admin", comment.ID); err != nil {
			t.Fatalf("admin delete comment: %v", err)
		}
		if _, err := commentService.commentRepo.FindByID(comment.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("expected comment to be deleted, got %v", err)
		}
	})
}
