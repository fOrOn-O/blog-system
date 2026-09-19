package service

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"blog-system/internal/database"
	"blog-system/internal/model"
)

func TestArticleEditPreviewUsesCanonicalBaseAndExistingDiff(t *testing.T) {
	svc, user, tags := setupArticleServiceTest(t)
	created, err := svc.CreateDraft(user.ID, CreateDraftRequest{Title: "标题", Content: "<p>Redis很快</p>",
		Summary: "摘要", CoverImage: "/cover.png", TagIDs: []uint{tags[0].ID}})
	if err != nil {
		t.Fatal(err)
	}
	content := "<p>Redis性能很高</p><p>新增段落</p>"
	if _, err := svc.UpdateDraft(user.ID, created.ID, UpdateDraftRequest{ExpectedVersion: 1, Content: &content}); err != nil {
		t.Fatal(err)
	}
	before, err := svc.articleRepo.FindByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	versions := readArticleVersions(t, created.ID)
	preview, err := svc.PreviewArticleEdit(user.ID, created.ID, ArticleEditRequest{1, content})
	if err != nil {
		t.Fatal(err)
	}
	diff, err := svc.GetVersionDiff(user.ID, created.ID, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if preview.BaseVersionNo != 1 || preview.ArticleID != created.ID ||
		!reflect.DeepEqual(preview.Content, diff.Content) || !reflect.DeepEqual(preview.FieldChanges, diff.FieldChanges) {
		t.Fatalf("preview did not reuse historical diff: %+v", preview)
	}
	if len(preview.Content.Changes) != 2 || preview.Content.Changes[0].Before.Text != "Redis很快" {
		t.Fatal("preview used current draft instead of requested base")
	}
	identical, err := svc.PreviewArticleEdit(user.ID, created.ID, ArticleEditRequest{1, "<p><strong>Redis很快</strong></p>"})
	if err != nil || identical.Content.Changed {
		t.Fatalf("format-only preview: %+v %v", identical, err)
	}
	after, err := svc.articleRepo.FindByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(versions, readArticleVersions(t, created.ID)) {
		t.Fatal("preview changed article, counters, tags, timestamps or snapshots")
	}
}

func TestApplyArticleEditCreatesOnlyDraftVersion(t *testing.T) {
	for _, published := range []bool{false, true} {
		t.Run(fmt.Sprint(published), func(t *testing.T) {
			svc, user, tags := setupArticleServiceTest(t)
			created, err := svc.CreateDraft(user.ID, CreateDraftRequest{Title: "标题", Content: "<p>V1</p>",
				Summary: "摘要", CoverImage: "/cover.png", TagIDs: []uint{tags[0].ID}})
			if err != nil {
				t.Fatal(err)
			}
			if published {
				if _, err := svc.PublishArticle(user.ID, created.ID, PublishArticleRequest{1}); err != nil {
					t.Fatal(err)
				}
			}
			v2 := "<p>V2</p>"
			before, err := svc.UpdateDraft(user.ID, created.ID, UpdateDraftRequest{ExpectedVersion: 1, Content: &v2})
			if err != nil {
				t.Fatal(err)
			}
			versions := readArticleVersions(t, created.ID)
			cacheKeys := []string{fmt.Sprintf("article:public:v2:%d", created.ID), "articles:list:proposal-test"}
			for _, key := range cacheKeys {
				database.CacheSet(key, "cached", time.Hour)
				defer database.CacheDelete(key)
			}
			result, err := svc.ApplyArticleEdit(user.ID, created.ID, ArticleEditRequest{2, "<p>V3批准稿</p>"})
			if err != nil {
				t.Fatal(err)
			}
			if result.ArticleID != created.ID || result.PreviousVersionNo != 2 || result.NewVersionNo != 3 ||
				result.Status != "draft" || result.PublishedVersion != before.PublishedVersion {
				t.Fatalf("result=%+v", result)
			}
			after, err := svc.articleRepo.FindByID(created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if after.Version != 3 || after.Content != "<p>V3批准稿</p>" || after.PublishedVersion != before.PublishedVersion ||
				after.Status != before.Status || after.Title != before.Title || after.Summary != before.Summary ||
				after.CoverImage != before.CoverImage || len(after.Tags) != 1 || after.Tags[0].ID != tags[0].ID {
				t.Fatalf("unexpected mutation: %+v", after)
			}
			stored := readArticleVersions(t, created.ID)
			if len(stored) != 3 || !reflect.DeepEqual(stored[:2], versions) {
				t.Fatal("history changed")
			}
			assertArticleSnapshot(t, stored[2], after, user.ID)
			for _, key := range cacheKeys {
				if _, err := database.CacheGet(key); err == nil {
					t.Fatal("cache not invalidated")
				}
			}
			if published {
				public, err := svc.GetByID(created.ID)
				if err != nil || public.Content != "<p>V1</p>" || public.Version != 1 {
					t.Fatalf("public content changed: %+v %v", public, err)
				}
			}
		})
	}
}

func TestApplyArticleEditFailuresNeverMutate(t *testing.T) {
	for _, name := range []string{"no_change", "empty", "stale", "stale_same_content", "foreign_owner", "missing_snapshot", "archived", "deleted"} {
		t.Run(name, func(t *testing.T) {
			svc, user, tags := setupArticleServiceTest(t)
			created, err := svc.CreateDraft(user.ID, CreateDraftRequest{Title: "原稿", Content: "<p>V1</p>", TagIDs: []uint{tags[0].ID}})
			if err != nil {
				t.Fatal(err)
			}
			req := ArticleEditRequest{1, "<p>新稿</p>"}
			actor := user.ID
			var want error
			switch name {
			case "no_change":
				req.ProposedContent = created.Content
				want = model.ErrArticleContentUnchanged
			case "empty":
				req.ProposedContent = ""
				want = model.ErrInvalidArticleContent
			case "stale", "stale_same_content":
				content := "<p>V2</p>"
				if _, err := svc.UpdateDraft(user.ID, created.ID, UpdateDraftRequest{ExpectedVersion: 1, Content: &content}); err != nil {
					t.Fatal(err)
				}
				if name == "stale_same_content" {
					req.ProposedContent = content
				}
				want = model.ErrVersionConflict
			case "foreign_owner":
				actor++
				want = model.ErrArticleForbidden
			case "missing_snapshot":
				if err := database.DB.Where("article_id = ?", created.ID).Delete(&model.ArticleVersion{}).Error; err != nil {
					t.Fatal(err)
				}
				want = model.ErrArticleVersionNotFound
			case "archived":
				if _, err := svc.ArchiveArticle(user.ID, created.ID); err != nil {
					t.Fatal(err)
				}
				want = model.ErrArticleArchivedEdit
			case "deleted":
				if err := svc.Delete(user.ID, "user", created.ID); err != nil {
					t.Fatal(err)
				}
				want = model.ErrArticleNotFound
			}
			var before, after model.Article
			if err := database.DB.Unscoped().Preload("Tags").First(&before, created.ID).Error; err != nil {
				t.Fatal(err)
			}
			versions := readArticleVersions(t, created.ID)
			result, err := svc.ApplyArticleEdit(actor, created.ID, req)
			if !errors.Is(err, want) || result != nil {
				t.Fatalf("got result=%+v err=%v want=%v", result, err, want)
			}
			if err := database.DB.Unscoped().Preload("Tags").First(&after, created.ID).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(versions, readArticleVersions(t, created.ID)) {
				t.Fatal("failed apply mutated data")
			}
		})
	}
}

func TestApplyArticleEditRollbackAtEachWrite(t *testing.T) {
	for _, failure := range []struct{ name, trigger string }{
		{"article_update", "BEFORE UPDATE ON articles"},
		{"snapshot_insert", "AFTER INSERT ON article_versions"},
	} {
		t.Run(failure.name, func(t *testing.T) {
			svc, user, tags := setupArticleServiceTest(t)
			created, err := svc.Create(user.ID, CreateArticleRequest{Title: "已发布", Content: "<p>V1</p>", TagIDs: []uint{tags[0].ID}})
			if err != nil {
				t.Fatal(err)
			}
			before, err := svc.articleRepo.FindByID(created.ID)
			if err != nil {
				t.Fatal(err)
			}
			versions := readArticleVersions(t, created.ID)
			key := fmt.Sprintf("article:public:v2:%d", created.ID)
			database.CacheSet(key, "original", time.Hour)
			defer database.CacheDelete(key)
			// SQLite 触发器在真实写入处失败，快照用 AFTER INSERT 覆盖已更新文章后的回滚。
			if err := database.DB.Exec("CREATE TRIGGER reject_proposal " + failure.trigger + " BEGIN SELECT RAISE(ABORT, 'proposal write failed'); END").Error; err != nil {
				t.Fatal(err)
			}
			req := ArticleEditRequest{1, "<p>批准稿</p>"}
			if result, err := svc.ApplyArticleEdit(user.ID, created.ID, req); err == nil || result != nil {
				t.Fatal("expected failure")
			}
			after, err := svc.articleRepo.FindByID(created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(versions, readArticleVersions(t, created.ID)) {
				t.Fatal("partial article/version/tag write escaped rollback")
			}
			if value, err := database.CacheGet(key); err != nil || value != "original" {
				t.Fatal("failed transaction invalidated cache")
			}
			if err := database.DB.Exec("DROP TRIGGER reject_proposal").Error; err != nil {
				t.Fatal(err)
			}
			result, err := svc.ApplyArticleEdit(user.ID, created.ID, req)
			if err != nil || result.NewVersionNo != 2 {
				t.Fatalf("retry skipped a version: %+v %v", result, err)
			}
		})
	}
}

func TestArticleContentValidationSharedByManualAndApprovedWrites(t *testing.T) {
	svc, user, _ := setupArticleServiceTest(t)
	if _, err := svc.CreateDraft(user.ID, CreateDraftRequest{Title: "empty"}); !errors.Is(err, model.ErrInvalidArticleContent) {
		t.Fatal(err)
	}
	created, err := svc.CreateDraft(user.ID, CreateDraftRequest{Title: "正文", Content: "<p>V1</p>"})
	if err != nil {
		t.Fatal(err)
	}
	empty := ""
	if _, err := svc.UpdateDraft(user.ID, created.ID, UpdateDraftRequest{ExpectedVersion: 1, Content: &empty}); !errors.Is(err, model.ErrInvalidArticleContent) {
		t.Fatal(err)
	}
	if _, err := svc.ApplyArticleEdit(user.ID, created.ID, ArticleEditRequest{1, empty}); !errors.Is(err, model.ErrInvalidArticleContent) {
		t.Fatal(err)
	}
	if len(readArticleVersions(t, created.ID)) != 1 {
		t.Fatal("invalid content persisted")
	}
}
