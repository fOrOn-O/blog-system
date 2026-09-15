package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"blog-system/internal/database"
	"blog-system/internal/model"
)

func createWorkflowArticle(t *testing.T, svc *ArticleService, userID uint, draft bool) *ArticleResponse {
	t.Helper()
	req := CreateDraftRequest{Title: "Public title", Content: "<p>PublicBody</p>", Summary: "Public summary", CoverImage: "/public.png"}
	var article *ArticleResponse
	var err error
	if draft {
		article, err = svc.CreateDraft(userID, req)
	} else {
		article, err = svc.Create(userID, CreateArticleRequest(req))
	}
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	return article
}

func assertWorkflowState(t *testing.T, svc *ArticleService, id uint, status string, version, published uint) *model.Article {
	t.Helper()
	article, err := svc.articleRepo.FindByID(id)
	if err != nil {
		t.Fatalf("read article: %v", err)
	}
	if article.Status != status || article.Version != version || article.PublishedVersion != published {
		t.Fatalf("state=%s/%d/%d, want %s/%d/%d", article.Status, article.Version, article.PublishedVersion, status, version, published)
	}
	versions := readArticleVersions(t, id)
	if len(versions) != int(version) {
		t.Fatalf("snapshot count=%d, want %d", len(versions), version)
	}
	assertArticleSnapshot(t, versions[len(versions)-1], article, article.UserID)
	return article
}

func assertSameContent(t *testing.T, got, want *ArticleResponse) {
	t.Helper()
	if got.Title != want.Title || got.Content != want.Content || got.Summary != want.Summary || got.CoverImage != want.CoverImage {
		t.Fatalf("unexpected visible content: got=%+v want=%+v", got, want)
	}
}

func TestArticleDraftPublishLifecycle(t *testing.T) {
	svc, user, tags := setupArticleServiceTest(t)
	created := createWorkflowArticle(t, svc, user.ID, true)
	assertWorkflowState(t, svc, created.ID, "draft", 1, 0)
	if created.Status != "draft" || created.Version != 1 || created.PublishedVersion != 0 {
		t.Fatalf("wrong create draft response: %+v", created)
	}
	ids := []uint{tags[0].ID}
	if _, err := svc.UpdateDraft(user.ID, created.ID, UpdateDraftRequest{ExpectedVersion: 1, TagIDs: &ids}); err != nil {
		t.Fatal(err)
	}
	article := assertWorkflowState(t, svc, created.ID, "draft", 1, 0)
	if len(article.Tags) != 1 || article.Tags[0].ID != ids[0] {
		t.Fatal("draft tags not persisted")
	}
	for version := 2; version <= 5; version++ {
		content := fmt.Sprintf("<p>VisibleV%d</p>", version)
		if _, err := svc.UpdateDraft(user.ID, created.ID, UpdateDraftRequest{ExpectedVersion: uint(version - 1), Content: &content}); err != nil {
			t.Fatal(err)
		}
	}
	assertWorkflowState(t, svc, created.ID, "draft", 5, 0)
	beforePublish := readArticleVersions(t, created.ID)
	visible, err := svc.PublishArticle(user.ID, created.ID, PublishArticleRequest{ExpectedVersion: 5})
	if err != nil {
		t.Fatal(err)
	}
	assertWorkflowState(t, svc, created.ID, "published", 5, 5)
	if !reflect.DeepEqual(beforePublish, readArticleVersions(t, created.ID)) {
		t.Fatal("publish changed snapshots")
	}
	// Populate public caches before editing to exercise invalidation.
	if _, err := svc.GetByID(created.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.List(1, 10, ""); err != nil {
		t.Fatal(err)
	}
	title, content, summary, cover := "PrivateTitle", "<p>PrivateBodyV6</p>", "PrivateSummary", "/private.png"
	working, err := svc.UpdateDraft(user.ID, created.ID, UpdateDraftRequest{ExpectedVersion: visible.Version, Title: &title, Content: &content, Summary: &summary, CoverImage: &cover})
	if err != nil {
		t.Fatal(err)
	}
	assertWorkflowState(t, svc, created.ID, "published", 6, 5)
	public, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertSameContent(t, public, visible)
	if public.Version != 5 || public.PublishedVersion != 5 {
		t.Fatalf("public response must describe visible V5: %+v", public)
	}
	beforeOwned := assertWorkflowState(t, svc, created.ID, "published", 6, 5).ViewCount
	for i := 0; i < 3; i++ {
		owned, err := svc.GetOwnedArticle(user.ID, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		assertSameContent(t, owned, working)
		if owned.Version != 6 || owned.PublishedVersion != 5 {
			t.Fatal("owner did not receive working version metadata")
		}
	}
	if after := assertWorkflowState(t, svc, created.ID, "published", 6, 5).ViewCount; after != beforeOwned || after == 0 {
		t.Fatalf("view count: before owned=%d after=%d", beforeOwned, after)
	}
	for _, keyword := range []string{title, "PrivateBodyV6"} {
		found, total, err := svc.Search(keyword, 1, 10)
		if err != nil || total != 0 || len(found) != 0 {
			t.Fatalf("unpublished search leak for %q: total=%d err=%v", keyword, total, err)
		}
	}
	found, total, err := svc.Search("VisibleV5", 1, 10)
	if err != nil || total != 1 || len(found) != 1 {
		t.Fatalf("published content not searchable: total=%d err=%v", total, err)
	}
	assertSameContent(t, &found[0], visible)
	listed, total, err := svc.List(1, 10, "draft")
	if err != nil || total != 1 || len(listed) != 1 {
		t.Fatalf("public list: total=%d err=%v", total, err)
	}
	assertSameContent(t, &listed[0], visible)
	beforePublish = readArticleVersions(t, created.ID)
	for i := 0; i < 2; i++ {
		if _, err := svc.PublishArticle(user.ID, created.ID, PublishArticleRequest{ExpectedVersion: working.Version}); err != nil {
			t.Fatal(err)
		}
	}
	assertWorkflowState(t, svc, created.ID, "published", 6, 6)
	if !reflect.DeepEqual(beforePublish, readArticleVersions(t, created.ID)) {
		t.Fatal("publishing pending changes created or modified snapshots")
	}
	public, err = svc.GetByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertSameContent(t, public, working)
	listed, _, err = svc.List(1, 10, "published")
	if err != nil || len(listed) != 1 {
		t.Fatalf("list after publish: %v", err)
	}
	assertSameContent(t, &listed[0], working)
}

func TestArticleArchiveIsTerminalAndIdempotent(t *testing.T) {
	for _, draft := range []bool{true, false} {
		t.Run(fmt.Sprintf("draft=%t", draft), func(t *testing.T) {
			svc, user, _ := setupArticleServiceTest(t)
			created := createWorkflowArticle(t, svc, user.ID, draft)
			before := readArticleVersions(t, created.ID)
			if !draft {
				if _, err := svc.GetByID(created.ID); err != nil {
					t.Fatal(err)
				}
				if _, _, err := svc.List(1, 10, ""); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				if _, err := svc.ArchiveArticle(user.ID, created.ID); err != nil {
					t.Fatal(err)
				}
			}
			assertWorkflowState(t, svc, created.ID, "archived", 1, created.PublishedVersion)
			if !reflect.DeepEqual(before, readArticleVersions(t, created.ID)) {
				t.Fatal("archive changed snapshots")
			}
			content := "forbidden"
			if _, err := svc.UpdateDraft(user.ID, created.ID, UpdateDraftRequest{ExpectedVersion: 1, Content: &content}); !errors.Is(err, model.ErrArticleArchivedEdit) {
				t.Fatalf("draft edit: %v", err)
			}
			if _, err := svc.Update(user.ID, created.ID, UpdateArticleRequest{ExpectedVersion: 1, Content: &content}); !errors.Is(err, model.ErrArticleArchivedEdit) {
				t.Fatalf("legacy edit: %v", err)
			}
			if _, err := svc.PublishArticle(user.ID, created.ID, PublishArticleRequest{ExpectedVersion: 1}); !errors.Is(err, model.ErrArticleArchivedPublish) {
				t.Fatalf("publish: %v", err)
			}
			if _, err := svc.GetByID(created.ID); !errors.Is(err, model.ErrArticleNotFound) {
				t.Fatalf("public archive: %v", err)
			}
			listed, total, err := svc.List(1, 10, "archived")
			if err != nil || total != 0 || len(listed) != 0 {
				t.Fatalf("archived list leak: total=%d err=%v", total, err)
			}
			found, total, err := svc.Search("Public", 1, 10)
			if err != nil || total != 0 || len(found) != 0 {
				t.Fatalf("archived search leak: total=%d err=%v", total, err)
			}
		})
	}
}

func TestArticleWorkflowAuthorizationAndOwnerPagination(t *testing.T) {
	svc, owner, _ := setupArticleServiceTest(t)
	other := model.User{Username: "other", Email: "other@example.com", Password: "hashed", IsActive: true}
	if err := database.DB.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	draft := createWorkflowArticle(t, svc, owner.ID, true)
	createWorkflowArticle(t, svc, owner.ID, false)
	archived := createWorkflowArticle(t, svc, owner.ID, true)
	if _, err := svc.ArchiveArticle(owner.ID, archived.ID); err != nil {
		t.Fatal(err)
	}
	createWorkflowArticle(t, svc, other.ID, true)
	content := "unauthorized"
	actions := []func(uint, uint) (*ArticleResponse, error){
		svc.GetOwnedArticle, svc.ArchiveArticle,
		func(uid, id uint) (*ArticleResponse, error) {
			return svc.PublishArticle(uid, id, PublishArticleRequest{ExpectedVersion: draft.Version})
		},
		func(uid, id uint) (*ArticleResponse, error) {
			return svc.UpdateDraft(uid, id, UpdateDraftRequest{ExpectedVersion: 1, Content: &content})
		},
		func(uid, id uint) (*ArticleResponse, error) {
			return svc.Update(uid, id, UpdateArticleRequest{ExpectedVersion: 1, Content: &content})
		},
	}
	for _, action := range actions {
		if _, err := action(other.ID, draft.ID); !errors.Is(err, model.ErrArticleForbidden) {
			t.Fatalf("expected forbidden, got %v", err)
		}
		if _, err := action(owner.ID, 999999); !errors.Is(err, model.ErrArticleNotFound) {
			t.Fatalf("expected missing article, got %v", err)
		}
	}
	assertWorkflowState(t, svc, draft.ID, "draft", 1, 0)
	seen := map[uint]bool{}
	for page := 1; page <= 3; page++ {
		articles, total, err := svc.ListMyArticles(owner.ID, page, 1, "")
		if err != nil || total != 3 || len(articles) != 1 {
			t.Fatalf("owner pagination: total=%d err=%v", total, err)
		}
		if articles[0].User.ID != owner.ID || seen[articles[0].ID] {
			t.Fatal("owner pagination leaks another owner or duplicates")
		}
		seen[articles[0].ID] = true
	}
	for _, status := range []string{"draft", "published", "archived"} {
		articles, total, err := svc.ListMyArticles(owner.ID, 1, 10, status)
		if err != nil || total != 1 || len(articles) != 1 || articles[0].Status != status {
			t.Fatalf("owner status %s: total=%d err=%v", status, total, err)
		}
	}
	if _, err := svc.GetByID(draft.ID); !errors.Is(err, model.ErrArticleNotFound) {
		t.Fatalf("public draft: %v", err)
	}
	found, total, err := svc.List(1, 10, "draft")
	if err != nil || total != 1 || len(found) != 1 || found[0].Status != "published" {
		t.Fatalf("public draft list leak: total=%d err=%v", total, err)
	}
}

func TestLegacyHumanWritesPublishAtomically(t *testing.T) {
	svc, user, tags := setupArticleServiceTest(t)
	created := createWorkflowArticle(t, svc, user.ID, false)
	assertWorkflowState(t, svc, created.ID, "published", 1, 1)
	if _, err := svc.GetByID(created.ID); err != nil {
		t.Fatal(err)
	}
	content := "<p>HumanV2</p>"
	ids := []uint{tags[1].ID}
	updated, err := svc.Update(user.ID, created.ID, UpdateArticleRequest{ExpectedVersion: 1, Content: &content, TagIDs: &ids})
	if err != nil {
		t.Fatal(err)
	}
	assertWorkflowState(t, svc, created.ID, "published", 2, 2)
	public, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertSameContent(t, public, updated)
	if len(public.Tags) != 1 || public.Tags[0].ID != tags[1].ID {
		t.Fatal("human tags missing")
	}
}

func TestArticleWorkflowSnapshotFailureRollsBackAllFields(t *testing.T) {
	for _, mode := range []string{"draft", "published draft edit", "human publish"} {
		t.Run(mode, func(t *testing.T) {
			svc, user, tags := setupArticleServiceTest(t)
			created := createWorkflowArticle(t, svc, user.ID, mode == "draft")
			oldIDs := []uint{tags[0].ID}
			if _, err := svc.UpdateDraft(user.ID, created.ID, UpdateDraftRequest{ExpectedVersion: 1, TagIDs: &oldIDs}); err != nil {
				t.Fatal(err)
			}
			original, err := svc.articleRepo.FindByID(created.ID)
			if err != nil {
				t.Fatal(err)
			}
			versions := readArticleVersions(t, created.ID)
			keys := []string{fmt.Sprintf("article:public:v2:%d", created.ID), "articles:list:public:v2:1:10"}
			for _, key := range keys {
				database.CacheSet(key, "unchanged", time.Hour)
				defer database.CacheDelete(key)
			}
			rejectArticleVersionInserts(t)
			title, content, summary, cover := "Updated", "<p>Updated</p>", "", ""
			ids := []uint{tags[1].ID}
			req := UpdateDraftRequest{ExpectedVersion: 1, Title: &title, Content: &content, Summary: &summary, CoverImage: &cover, TagIDs: &ids}
			if mode == "human publish" {
				_, err = svc.Update(user.ID, created.ID, UpdateArticleRequest(req))
			} else {
				_, err = svc.UpdateDraft(user.ID, created.ID, req)
			}
			if err == nil {
				t.Fatal("expected snapshot insertion failure")
			}
			after, err := svc.articleRepo.FindByID(created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(original, after) {
				t.Fatal("article, version, published version or tags did not roll back")
			}
			if !reflect.DeepEqual(versions, readArticleVersions(t, created.ID)) {
				t.Fatal("partial V2 or damaged history")
			}
			for _, key := range keys {
				if got, err := database.CacheGet(key); err != nil || got != "unchanged" {
					t.Fatalf("cache changed on rollback: %s", key)
				}
			}
		})
	}
}

func TestCreateDraftSnapshotFailureRollsBack(t *testing.T) {
	svc, user, tags := setupArticleServiceTest(t)
	rejectArticleVersionInserts(t)
	if _, err := svc.CreateDraft(user.ID, CreateDraftRequest{Title: "Draft", Content: "Body", TagIDs: []uint{tags[0].ID}}); err == nil {
		t.Fatal("expected insert failure")
	}
	for _, table := range []string{"articles", "article_versions", "article_tags"} {
		var count int64
		if err := database.DB.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("partial create in %s", table)
		}
	}
}

func TestArticlePublicReadFailsClosedForLegacyDataAndCaches(t *testing.T) {
	for _, published := range []uint{0, 1} {
		t.Run(fmt.Sprintf("pointer=%d", published), func(t *testing.T) {
			svc, user, _ := setupArticleServiceTest(t)
			legacy := model.Article{Title: "LegacySecret", Content: "LegacySecret", UserID: user.ID, Status: "published", PublishedVersion: published}
			if err := database.DB.Create(&legacy).Error; err != nil {
				t.Fatal(err)
			}
			// Old caches may contain unversioned working content. They must never be consumed.
			payload, err := json.Marshal(toArticleResponse(&legacy))
			if err != nil {
				t.Fatal(err)
			}
			key := fmt.Sprintf("article:%d", legacy.ID)
			database.CacheSet(key, string(payload), time.Hour)
			defer database.CacheDelete(key)
			database.CacheDeletePrefix("article:public:v2:")
			database.CacheDeletePrefix("articles:list:")
			if _, err := svc.GetByID(legacy.ID); !errors.Is(err, model.ErrArticleNotFound) {
				t.Fatalf("legacy content leaked: %v", err)
			}
			if _, err := svc.PublishArticle(user.ID, legacy.ID, PublishArticleRequest{ExpectedVersion: legacy.Version}); !errors.Is(err, model.ErrArticleSnapshotMissing) {
				t.Fatalf("missing snapshot published: %v", err)
			}
			for _, search := range []bool{false, true} {
				var items []ArticleResponse
				var total int64
				if search {
					items, total, err = svc.Search("LegacySecret", 1, 10)
				} else {
					items, total, err = svc.List(1, 10, "")
				}
				if err != nil || total != 0 || len(items) != 0 {
					t.Fatalf("legacy list/search leak: total=%d err=%v", total, err)
				}
			}
			owned, err := svc.GetOwnedArticle(user.ID, legacy.ID)
			if err != nil || owned.Content != legacy.Content {
				t.Fatalf("owner cannot inspect legacy article: %v", err)
			}
		})
	}
}

func TestFavoritesOnlyExposePublishedSnapshots(t *testing.T) {
	svc, owner, _ := setupArticleServiceTest(t)
	reader := model.User{Username: "reader", Email: "reader@example.com", Password: "hashed"}
	if err := database.DB.Create(&reader).Error; err != nil {
		t.Fatal(err)
	}
	created := createWorkflowArticle(t, svc, owner.ID, false)
	favorites := NewFavoriteService()
	if err := favorites.Favorite(reader.ID, created.ID); err != nil {
		t.Fatal(err)
	}
	content := "UnpublishedSecret"
	if _, err := svc.UpdateDraft(owner.ID, created.ID, UpdateDraftRequest{ExpectedVersion: 1, Content: &content}); err != nil {
		t.Fatal(err)
	}
	items, total, err := favorites.GetUserFavorites(reader.ID, 1, 10)
	if err != nil || total != 1 || len(items) != 1 || items[0].Article == nil {
		t.Fatalf("favorites: total=%d err=%v", total, err)
	}
	assertSameContent(t, items[0].Article, created)
	if _, err := svc.ArchiveArticle(owner.ID, created.ID); err != nil {
		t.Fatal(err)
	}
	items, total, err = favorites.GetUserFavorites(reader.ID, 1, 10)
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("archived favorites leaked: total=%d err=%v", total, err)
	}
	draft := createWorkflowArticle(t, svc, owner.ID, true)
	if err := favorites.Favorite(reader.ID, draft.ID); err == nil {
		t.Fatal("reader bookmarked a private draft")
	}
}

func TestPublicReadsRejectLateStaleCacheRefill(t *testing.T) {
	svc, user, _ := setupArticleServiceTest(t)
	created := createWorkflowArticle(t, svc, user.ID, false)
	public, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	list, err := json.Marshal(map[string]interface{}{"articles": []ArticleResponse{*public}, "total": 1})
	if err != nil {
		t.Fatal(err)
	}
	refill := func() {
		// Simulate a reader that captured V1 before a write and filled the cache after invalidation.
		database.CacheSet(fmt.Sprintf("article:public:v2:%d", created.ID), string(detail), time.Hour)
		database.CacheSet("articles:list:public:v2:1:10", string(list), time.Hour)
	}
	content := "NewPublishedContent"
	if _, err := svc.Update(user.ID, created.ID, UpdateArticleRequest{ExpectedVersion: 1, Content: &content}); err != nil {
		t.Fatal(err)
	}
	refill()
	public, err = svc.GetByID(created.ID)
	if err != nil || public.Content != content || public.Version != 2 {
		t.Fatalf("stale detail used after publish: %v", err)
	}
	listed, total, err := svc.List(1, 10, "")
	if err != nil || total != 1 || len(listed) != 1 || listed[0].Content != content {
		t.Fatalf("stale list used after publish: %v", err)
	}
	if _, err := svc.ArchiveArticle(user.ID, created.ID); err != nil {
		t.Fatal(err)
	}
	refill()
	if _, err := svc.GetByID(created.ID); !errors.Is(err, model.ErrArticleNotFound) {
		t.Fatalf("late cache refill exposed archived detail: %v", err)
	}
	listed, total, err = svc.List(1, 10, "")
	if err != nil || total != 0 || len(listed) != 0 {
		t.Fatalf("late cache refill exposed archived list: total=%d err=%v", total, err)
	}
}
