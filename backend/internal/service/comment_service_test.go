package service

import (
	"encoding/json"
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
