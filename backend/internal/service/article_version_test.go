package service

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"blog-system/internal/database"
	"blog-system/internal/model"
)

func readArticleVersions(t *testing.T, articleID uint) []model.ArticleVersion {
	t.Helper()
	var versions []model.ArticleVersion
	if err := database.DB.Where("article_id = ?", articleID).Order("version_no").Find(&versions).Error; err != nil {
		t.Fatalf("read article versions: %v", err)
	}
	return versions
}

func assertArticleSnapshot(t *testing.T, snapshot model.ArticleVersion, article *model.Article, userID uint) {
	t.Helper()
	if snapshot.ID == 0 || snapshot.CreatedAt.IsZero() {
		t.Fatal("snapshot ID and creation time must be populated")
	}
	if snapshot.ArticleID != article.ID || snapshot.VersionNo != article.Version {
		t.Fatalf("snapshot article/version = %d/%d, want %d/%d", snapshot.ArticleID, snapshot.VersionNo, article.ID, article.Version)
	}
	if snapshot.Title != article.Title || snapshot.Content != article.Content ||
		snapshot.Summary != article.Summary || snapshot.CoverImage != article.CoverImage {
		t.Fatalf("snapshot does not contain the complete persisted content: %#v", snapshot)
	}
	if snapshot.CreatedBy != userID || snapshot.Source != model.ArticleVersionSourceUser {
		t.Fatalf("unexpected snapshot attribution: user=%d source=%q", snapshot.CreatedBy, snapshot.Source)
	}
}

func TestArticleServiceCreatesVersionOne(t *testing.T) {
	for _, withTags := range []bool{false, true} {
		t.Run(fmt.Sprintf("with_tags=%t", withTags), func(t *testing.T) {
			svc, user, tags := setupArticleServiceTest(t)
			req := CreateArticleRequest{
				Title: "Original title", Content: "<h2>Heading</h2><p>Original content</p>",
				Summary: "Original summary", CoverImage: "/uploads/original.png",
			}
			if withTags {
				req.TagIDs = []uint{tags[0].ID}
			}
			created, err := svc.Create(user.ID, req)
			if err != nil {
				t.Fatalf("create article: %v", err)
			}
			article, err := svc.articleRepo.FindByID(created.ID)
			if err != nil {
				t.Fatalf("read created article: %v", err)
			}
			if article.Version != 1 {
				t.Fatalf("article version = %d, want 1", article.Version)
			}
			versions := readArticleVersions(t, article.ID)
			if len(versions) != 1 {
				t.Fatalf("version count = %d, want 1", len(versions))
			}
			assertArticleSnapshot(t, versions[0], article, user.ID)
		})
	}
}

func TestArticleServiceVersionsAreSequentialAndPreserveHistory(t *testing.T) {
	svc, user, _ := setupArticleServiceTest(t)
	created, err := svc.Create(user.ID, CreateArticleRequest{
		Title: "Original title", Content: "<p>V1</p>",
		Summary: "Original summary", CoverImage: "/uploads/original.png",
	})
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	previous := readArticleVersions(t, created.ID)
	for version := uint(2); version <= 3; version++ {
		content := fmt.Sprintf("<p>V%d</p>", version)
		if _, err := svc.Update(user.ID, created.ID, UpdateArticleRequest{ExpectedVersion: version - 1, Content: &content}); err != nil {
			t.Fatalf("update article to V%d: %v", version, err)
		}
		article, err := svc.articleRepo.FindByID(created.ID)
		if err != nil {
			t.Fatalf("read updated article: %v", err)
		}
		versions := readArticleVersions(t, created.ID)
		if article.Version != version || len(versions) != int(version) {
			t.Fatalf("article version=%d snapshot count=%d, want %d", article.Version, len(versions), version)
		}
		if !reflect.DeepEqual(versions[:len(previous)], previous) {
			t.Fatal("existing snapshots were modified")
		}
		for i, snapshot := range versions {
			if snapshot.VersionNo != uint(i+1) || snapshot.Content != fmt.Sprintf("<p>V%d</p>", i+1) {
				t.Fatalf("unexpected historical snapshot: %#v", snapshot)
			}
		}
		assertArticleSnapshot(t, versions[len(versions)-1], article, user.ID)
		previous = versions
	}
}

func TestArticleServiceVersionsOnlyActualContentChanges(t *testing.T) {
	ptr := func(s string) *string { return &s }
	cases := []struct {
		name        string
		req         UpdateArticleRequest
		wantVersion uint
	}{
		{"title", UpdateArticleRequest{Title: ptr("New title")}, 2},
		{"content", UpdateArticleRequest{Content: ptr("<p>New content</p>")}, 2},
		{"summary", UpdateArticleRequest{Summary: ptr("New summary")}, 2},
		{"cover", UpdateArticleRequest{CoverImage: ptr("/uploads/new.png")}, 2},
		{"clear summary", UpdateArticleRequest{Summary: ptr("")}, 2},
		{"clear cover", UpdateArticleRequest{CoverImage: ptr("")}, 2},
		{"same values", UpdateArticleRequest{
			Title: ptr("Original title"), Content: ptr("<p>Original content</p>"),
			Summary: ptr("Original summary"), CoverImage: ptr("/uploads/original.png"),
		}, 1},
		{"empty update", UpdateArticleRequest{}, 1},
		{"tags only", UpdateArticleRequest{}, 1},
		{"clear tags", UpdateArticleRequest{}, 1},
		{"content and tags", UpdateArticleRequest{Content: ptr("<p>New content</p>")}, 2},
		{"all content fields", UpdateArticleRequest{
			Title: ptr("New title"), Content: ptr("<p>New content</p>"),
			Summary: ptr("New summary"), CoverImage: ptr("/uploads/new.png"),
		}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, user, tags := setupArticleServiceTest(t)
			created, err := svc.Create(user.ID, CreateArticleRequest{
				Title: "Original title", Content: "<p>Original content</p>",
				Summary: "Original summary", CoverImage: "/uploads/original.png",
				TagIDs: []uint{tags[0].ID},
			})
			if err != nil {
				t.Fatalf("create article: %v", err)
			}
			original := readArticleVersions(t, created.ID)
			req := tc.req
			req.ExpectedVersion = created.Version
			if tc.name == "tags only" || tc.name == "content and tags" {
				replacement := []uint{tags[1].ID}
				req.TagIDs = &replacement
			} else if tc.name == "clear tags" {
				empty := []uint{}
				req.TagIDs = &empty
			}
			if _, err := svc.Update(user.ID, created.ID, req); err != nil {
				t.Fatalf("update article: %v", err)
			}
			article, err := svc.articleRepo.FindByID(created.ID)
			if err != nil {
				t.Fatalf("read article: %v", err)
			}
			versions := readArticleVersions(t, article.ID)
			if article.Version != tc.wantVersion || len(versions) != int(tc.wantVersion) {
				t.Fatalf("article version=%d snapshot count=%d, want %d", article.Version, len(versions), tc.wantVersion)
			}
			if !reflect.DeepEqual(versions[0], original[0]) {
				t.Fatal("V1 was modified")
			}
			assertArticleSnapshot(t, versions[len(versions)-1], article, user.ID)
			if req.TagIDs != nil {
				if len(article.Tags) != len(*req.TagIDs) {
					t.Fatalf("unexpected persisted tags: %#v", article.Tags)
				}
				for i, tagID := range *req.TagIDs {
					if article.Tags[i].ID != tagID {
						t.Fatalf("persisted tag = %d, want %d", article.Tags[i].ID, tagID)
					}
				}
			}
		})
	}
}

// A database trigger fails the actual INSERT after article and tag writes.
func rejectArticleVersionInserts(t *testing.T) {
	t.Helper()
	if err := database.DB.Exec(`CREATE TRIGGER reject_article_version
		BEFORE INSERT ON article_versions
		BEGIN SELECT RAISE(ABORT, 'test: snapshot insert rejected'); END`).Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
}

func TestArticleServiceCreateRollsBackWhenVersionInsertFails(t *testing.T) {
	svc, user, tags := setupArticleServiceTest(t)
	rejectArticleVersionInserts(t)
	if _, err := svc.Create(user.ID, CreateArticleRequest{
		Title: "Must roll back", Content: "<p>Content</p>", TagIDs: []uint{tags[0].ID},
	}); err == nil {
		t.Fatal("expected snapshot insert failure")
	}
	for _, table := range []string{"articles", "article_tags", "article_versions"} {
		var count int64
		if err := database.DB.Table(table).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s has %d rows after rollback", table, count)
		}
	}
}

func TestArticleServiceUpdateRollsBackWhenVersionInsertFails(t *testing.T) {
	svc, user, tags := setupArticleServiceTest(t)
	created, err := svc.Create(user.ID, CreateArticleRequest{
		Title: "Original title", Content: "<p>Original content</p>", Summary: "Original summary",
		CoverImage: "/uploads/original.png", TagIDs: []uint{tags[0].ID},
	})
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	original, err := svc.articleRepo.FindByID(created.ID)
	if err != nil {
		t.Fatalf("read original article: %v", err)
	}
	originalVersions := readArticleVersions(t, created.ID)
	cacheKeys := []string{fmt.Sprintf("article:%d", created.ID), "articles:list:1:10:published"}
	for _, key := range cacheKeys {
		database.CacheSet(key, "original cached value", time.Hour)
		defer database.CacheDelete(key)
	}
	rejectArticleVersionInserts(t)
	title, content, summary, cover := "New title", "<p>New content</p>", "", ""
	replacement := []uint{tags[1].ID}
	req := UpdateArticleRequest{
		ExpectedVersion: original.Version,
		Title:           &title, Content: &content, Summary: &summary, CoverImage: &cover,
		TagIDs: &replacement,
	}
	if _, err := svc.Update(user.ID, created.ID, req); err == nil {
		t.Fatal("expected snapshot insert failure")
	}
	after, err := svc.articleRepo.FindByID(created.ID)
	if err != nil {
		t.Fatalf("read article after rollback: %v", err)
	}
	if !reflect.DeepEqual(after, original) {
		t.Fatal("article fields, version, timestamps or tag associations changed despite rollback")
	}
	if !reflect.DeepEqual(readArticleVersions(t, created.ID), originalVersions) {
		t.Fatal("snapshot history changed despite rollback")
	}
	for _, key := range cacheKeys {
		if value, err := database.CacheGet(key); err != nil || value != "original cached value" {
			t.Fatalf("cache changed after rollback: key=%q value=%q err=%v", key, value, err)
		}
	}

	// Removing the failure allows a normal V2 write without a skipped version.
	if err := database.DB.Exec("DROP TRIGGER reject_article_version").Error; err != nil {
		t.Fatalf("remove failure trigger: %v", err)
	}
	if _, err := svc.Update(user.ID, created.ID, req); err != nil {
		t.Fatalf("retry update: %v", err)
	}
	versions := readArticleVersions(t, created.ID)
	if len(versions) != 2 || versions[1].VersionNo != 2 || versions[1].Content != content {
		t.Fatalf("unexpected history after retry: %#v", versions)
	}
	for _, key := range cacheKeys {
		if _, err := database.CacheGet(key); err == nil {
			t.Fatalf("cache was not invalidated after successful update: %s", key)
		}
	}
}

func TestArticleVersionUniquePerArticle(t *testing.T) {
	svc, user, _ := setupArticleServiceTest(t)
	first, err := svc.Create(user.ID, CreateArticleRequest{Title: "First", Content: "Content"})
	if err != nil {
		t.Fatalf("create first article: %v", err)
	}
	second, err := svc.Create(user.ID, CreateArticleRequest{Title: "Second", Content: "Content"})
	if err != nil {
		t.Fatalf("create second article with its own V1: %v", err)
	}
	duplicate := readArticleVersions(t, first.ID)[0]
	duplicate.ID = 0
	if err := database.DB.Create(&duplicate).Error; err == nil {
		t.Fatal("database accepted duplicate article/version pair")
	}
	for _, id := range []uint{first.ID, second.ID} {
		if versions := readArticleVersions(t, id); len(versions) != 1 || versions[0].VersionNo != 1 {
			t.Fatalf("unexpected history for article %d: %#v", id, versions)
		}
	}
}

func TestArticleVersionColumnDefaultsAndRejectsNull(t *testing.T) {
	svc, user, _ := setupArticleServiceTest(t)
	// Bypass repository initialization to exercise the database default itself.
	row := map[string]interface{}{"title": "Default version", "content": "Content", "user_id": user.ID}
	if err := database.DB.Model(&model.Article{}).Create(row).Error; err != nil {
		t.Fatalf("insert article without version: %v", err)
	}
	var article model.Article
	if err := database.DB.First(&article).Error; err != nil {
		t.Fatalf("read article: %v", err)
	}
	if article.Version != 1 {
		t.Fatalf("database default version = %d, want 1", article.Version)
	}
	if err := database.DB.Model(&article).UpdateColumn("version", nil).Error; err == nil {
		t.Fatal("database accepted a NULL article version")
	}
	stored, err := svc.articleRepo.FindByID(article.ID)
	if err != nil || stored.Version != 1 {
		t.Fatalf("version changed after rejected NULL update: err=%v", err)
	}
}
