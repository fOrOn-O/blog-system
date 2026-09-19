package main

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blog-system/internal/config"
	"blog-system/internal/database"
	"blog-system/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCommandRequiresExplicitConfigurationAndDoesNotCreateDatabase(t *testing.T) {
	t.Setenv("BLOG_DATABASE_DRIVER", "")
	t.Setenv("BLOG_DATABASE_DSN", "")
	var out, errors bytes.Buffer
	if code := run(context.Background(), nil, &out, &errors); code != 2 {
		t.Fatalf("code=%d", code)
	}
	t.Setenv("BLOG_DATABASE_DRIVER", "sqlite")
	path := filepath.Join(t.TempDir(), "missing.db")
	t.Setenv("BLOG_DATABASE_DSN", path)
	if code := run(context.Background(), nil, &out, &errors); code != 1 {
		t.Fatalf("code=%d", code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("错误路径创建了新数据库")
	}
	if code := run(context.Background(), []string{"--apply", "--dry-run"}, &out, &errors); code != 2 {
		t.Fatal("互斥参数未拒绝")
	}
}

func TestCommandDefaultsToReadOnlyAndApplyIsExplicit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy with spaces.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Article{}, &model.ArticleVersion{}); err != nil {
		t.Fatal(err)
	}
	secret := "正文绝不能出现在命令输出中"
	a := model.Article{Title: secret, Content: secret, UserID: 1, Version: 5, Status: model.ArticleStatusPublished}
	db.Create(&a)
	sqlDB, _ := db.DB()
	sqlDB.Close()
	t.Setenv("BLOG_DATABASE_DRIVER", "sqlite")
	t.Setenv("BLOG_DATABASE_DSN", path)
	for i, args := range [][]string{nil, {"--apply"}, {"--dry-run"}, {"--apply"}} {
		var out, errors bytes.Buffer
		if code := run(context.Background(), args, &out, &errors); code != 0 {
			t.Fatalf("step=%d code=%d error=%s", i, code, errors.String())
		}
		if strings.Contains(out.String()+errors.String(), secret) {
			t.Fatal("正文泄漏")
		}
		want := []string{`"would_migrate":1`, `"migrated":1`, `"already_versioned":1`, `"migrated":0`}[i]
		if !strings.Contains(out.String(), want) {
			t.Fatalf("缺少 %s: %s", want, out.String())
		}
	}
	readOnly, err := database.OpenMigrationDatabase(config.Database{Driver: "sqlite", DSN: path}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { sqlDB, _ := readOnly.DB(); sqlDB.Close() }()
	if err := readOnly.Model(&model.Article{}).Where("id = ?", a.ID).UpdateColumn("title", "unexpected").Error; err == nil {
		t.Fatal("dry-run 连接不是只读")
	}
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := url.URL{Scheme: "file", Path: uriPath, RawQuery: "cache=private"}
	t.Setenv("BLOG_DATABASE_DSN", uri.String())
	var out, errors bytes.Buffer
	if code := run(context.Background(), []string{"--dry-run"}, &out, &errors); code != 0 {
		t.Fatalf("文件 URI 连接失败: %s", errors.String())
	}
	readWrite, err := database.OpenMigrationDatabase(config.Database{Driver: "sqlite", DSN: path}, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := readWrite.Model(&model.Article{}).Where("id = ?", a.ID).UpdateColumn("published_version", 99).Error; err != nil {
		t.Fatal(err)
	}
	writeSQL, _ := readWrite.DB()
	writeSQL.Close()
	out.Reset()
	errors.Reset()
	if code := run(context.Background(), []string{"--apply"}, &out, &errors); code != 1 || !strings.Contains(out.String(), `"inconsistent":1`) {
		t.Fatal("异常数据没有返回非零退出码")
	}
}

func TestCommandRejectsMissingSchemaWithoutCreatingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.db")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BLOG_DATABASE_DRIVER", "sqlite")
	t.Setenv("BLOG_DATABASE_DSN", path)
	var out, errors bytes.Buffer
	if code := run(context.Background(), []string{"--dry-run"}, &out, &errors); code != 1 {
		t.Fatal("缺失表未失败")
	}
	info, _ := os.Stat(path)
	if info.Size() != 0 {
		t.Fatal("dry-run 执行了结构迁移")
	}
}
