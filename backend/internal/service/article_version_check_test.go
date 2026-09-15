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

func createPublishedV5(t *testing.T, svc *ArticleService, userID uint) *ArticleResponse {
	t.Helper()
	article := createWorkflowArticle(t, svc, userID, false)
	for version := uint(2); version <= 5; version++ {
		content := fmt.Sprintf("<p>V%d</p>", version)
		var err error
		article, err = svc.Update(userID, article.ID, UpdateArticleRequest{ExpectedVersion: version - 1, Content: &content})
		if err != nil {
			t.Fatal(err)
		}
	}
	return article
}

func TestArticleVersionCheckRejectsLostUpdates(t *testing.T) {
	for _, human := range []bool{false, true} {
		t.Run(fmt.Sprintf("human=%t", human), func(t *testing.T) {
			svc, user, tags := setupArticleServiceTest(t)
			base := createPublishedV5(t, svc, user.ID)
			// B's model was read before A saved. Repository must reject it independently of Service.
			staleModel, err := svc.articleRepo.FindByID(base.ID)
			if err != nil {
				t.Fatal(err)
			}
			update := svc.UpdateDraft
			if human {
				update = func(uid, id uint, req UpdateDraftRequest) (*ArticleResponse, error) {
					return svc.Update(uid, id, UpdateArticleRequest(req))
				}
			}
			contentA := "<p>A saved V6</p>"
			tagsA := []uint{tags[0].ID}
			if _, err := update(user.ID, base.ID, UpdateDraftRequest{ExpectedVersion: 5, Content: &contentA, TagIDs: &tagsA}); err != nil {
				t.Fatal(err)
			}
			publishedVersion := uint(5)
			if human {
				publishedVersion = 6
			}
			afterA := assertWorkflowState(t, svc, base.ID, "published", 6, publishedVersion)
			versions := readArticleVersions(t, base.ID)
			keys := []string{fmt.Sprintf("article:public:v2:%d", base.ID), "articles:list:public:v2:1:10"}
			for _, key := range keys {
				database.CacheSet(key, "unchanged after conflict", time.Hour)
				defer database.CacheDelete(key)
			}
			assertUnchanged := func() {
				t.Helper()
				current, err := svc.articleRepo.FindByID(base.ID)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(current, afterA) {
					t.Fatal("conflict changed article fields, tags, version, published version or timestamps")
				}
				if !reflect.DeepEqual(readArticleVersions(t, base.ID), versions) {
					t.Fatal("conflict changed snapshots")
				}
				for _, key := range keys {
					if value, err := database.CacheGet(key); err != nil || value != "unchanged after conflict" {
						t.Fatalf("conflict changed cache: %s", key)
					}
				}
			}
			contentB := "<p>B must not overwrite A</p>"
			tagsB := []uint{tags[1].ID}
			cases := []struct {
				name    string
				request UpdateDraftRequest
			}{
				{"content and tags", UpdateDraftRequest{ExpectedVersion: 5, Content: &contentB, TagIDs: &tagsB}},
				{"tags only", UpdateDraftRequest{ExpectedVersion: 5, TagIDs: &tagsB}},
				{"same value", UpdateDraftRequest{ExpectedVersion: 5, Content: &contentA}},
				{"empty update", UpdateDraftRequest{ExpectedVersion: 5}},
				{"future version", UpdateDraftRequest{ExpectedVersion: 7, Content: &contentB}},
			}
			for _, tc := range cases {
				if _, err := update(user.ID, base.ID, tc.request); !errors.Is(err, model.ErrVersionConflict) {
					t.Fatalf("%s: want version conflict, got %v", tc.name, err)
				}
				assertUnchanged()
			}
			staleModel.Content = contentB
			if human {
				err = svc.articleRepo.UpdateAndPublish(staleModel, []model.Tag{tags[1]}, true, user.ID, 5)
			} else {
				err = svc.articleRepo.UpdateDraft(staleModel, []model.Tag{tags[1]}, true, user.ID, 5)
			}
			if !errors.Is(err, model.ErrVersionConflict) {
				t.Fatalf("repository allowed stale model: %v", err)
			}
			assertUnchanged()
			// A matching working version succeeds even if PublishedVersion is still 5.
			updated, err := update(user.ID, base.ID, UpdateDraftRequest{ExpectedVersion: 6, Content: &contentB, TagIDs: &tagsB})
			if err != nil {
				t.Fatal(err)
			}
			if human {
				publishedVersion = 7
			}
			current := assertWorkflowState(t, svc, base.ID, "published", 7, publishedVersion)
			if updated.Version != 7 || current.Content != contentB || len(current.Tags) != 1 || current.Tags[0].ID != tags[1].ID {
				t.Fatal("matching version did not persist B's changes")
			}
		})
	}
}

func TestPublishChecksTheApprovedWorkingVersion(t *testing.T) {
	for _, advanceBeforeApproval := range []bool{false, true} {
		t.Run(fmt.Sprintf("newer_version=%t", advanceBeforeApproval), func(t *testing.T) {
			svc, user, _ := setupArticleServiceTest(t)
			base := createPublishedV5(t, svc, user.ID)
			content := "<p>Reviewed V6</p>"
			if _, err := svc.UpdateDraft(user.ID, base.ID, UpdateDraftRequest{ExpectedVersion: 5, Content: &content}); err != nil {
				t.Fatal(err)
			}
			version := uint(6)
			if advanceBeforeApproval {
				content = "<p>Unreviewed V7</p>"
				if _, err := svc.UpdateDraft(user.ID, base.ID, UpdateDraftRequest{ExpectedVersion: 6, Content: &content}); err != nil {
					t.Fatal(err)
				}
				version = 7
			}
			before := assertWorkflowState(t, svc, base.ID, "published", version, 5)
			versions := readArticleVersions(t, base.ID)
			if advanceBeforeApproval {
				key := fmt.Sprintf("article:public:v2:%d", base.ID)
				database.CacheSet(key, "published V5", time.Hour)
				defer database.CacheDelete(key)
				if _, err := svc.PublishArticle(user.ID, base.ID, PublishArticleRequest{ExpectedVersion: 6}); !errors.Is(err, model.ErrVersionConflict) {
					t.Fatalf("published an unreviewed version: %v", err)
				}
				if err := svc.articleRepo.PublishArticle(user.ID, base.ID, 6); !errors.Is(err, model.ErrVersionConflict) {
					t.Fatalf("repository skipped approval version: %v", err)
				}
				after, err := svc.articleRepo.FindByID(base.ID)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(versions, readArticleVersions(t, base.ID)) {
					t.Fatal("failed publish changed article or history")
				}
				if value, err := database.CacheGet(key); err != nil || value != "published V5" {
					t.Fatal("failed publish changed cache")
				}
			}
			published, err := svc.PublishArticle(user.ID, base.ID, PublishArticleRequest{ExpectedVersion: version})
			if err != nil {
				t.Fatal(err)
			}
			assertWorkflowState(t, svc, base.ID, "published", version, version)
			if published.Version != version || published.PublishedVersion != version || !reflect.DeepEqual(versions, readArticleVersions(t, base.ID)) {
				t.Fatal("publish must only move the pointer to the approved version")
			}
		})
	}
}

func TestVersionRequiredForDirectBusinessCalls(t *testing.T) {
	svc, user, _ := setupArticleServiceTest(t)
	created := createWorkflowArticle(t, svc, user.ID, true)
	before := assertWorkflowState(t, svc, created.ID, "draft", 1, 0)
	content := "must not be written"
	if _, err := svc.UpdateDraft(user.ID, created.ID, UpdateDraftRequest{Content: &content}); !errors.Is(err, model.ErrExpectedVersionRequired) {
		t.Fatalf("draft update defaulted version: %v", err)
	}
	if _, err := svc.Update(user.ID, created.ID, UpdateArticleRequest{Content: &content}); !errors.Is(err, model.ErrExpectedVersionRequired) {
		t.Fatalf("human update defaulted version: %v", err)
	}
	if _, err := svc.PublishArticle(user.ID, created.ID, PublishArticleRequest{}); !errors.Is(err, model.ErrExpectedVersionRequired) {
		t.Fatalf("publish defaulted version: %v", err)
	}
	after := assertWorkflowState(t, svc, created.ID, "draft", 1, 0)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("missing version changed article")
	}
}
