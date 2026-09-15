package router_test

import (
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"blog-system/internal/database"
	"blog-system/internal/model"
)

func readVersionHTTPState(t *testing.T, id uint) (model.Article, []model.ArticleVersion) {
	t.Helper()
	var article model.Article
	if err := database.DB.Preload("Tags").First(&article, id).Error; err != nil {
		t.Fatal(err)
	}
	var snapshots []model.ArticleVersion
	if err := database.DB.Where("article_id = ?", id).Order("version_no").Find(&snapshots).Error; err != nil {
		t.Fatal(err)
	}
	return article, snapshots
}

func TestArticleHTTPVersionConflictsHaveNoSideEffects(t *testing.T) {
	h, owner, _ := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/articles", owner, `{"title":"V1","content":"V1"}`, 201))
	path := fmt.Sprintf("/articles/%d", created.ID)
	working := httpArticle(t, workflowRequest(t, h, "PUT", path+"/draft", owner, `{"expected_version":1,"content":"V2 by A"}`, 200))
	if working.Version != 2 || working.PublishedVersion != 1 {
		t.Fatal("draft update unexpectedly published")
	}
	before, snapshots := readVersionHTTPState(t, created.ID)
	for _, target := range []struct{ method, path, body string }{
		{"PUT", path, `{"expected_version":1,"content":"V2 by B","tag_ids":[]}`},
		{"PUT", path + "/draft", `{"expected_version":1,"content":"V2 by B","tag_ids":[]}`},
		{"PUT", path + "/draft", `{"expected_version":1,"content":"V2 by A"}`},
		{"PUT", path + "/draft", `{"expected_version":1,"tag_ids":[]}`},
		{"POST", path + "/publish", `{"expected_version":1}`},
	} {
		req := httptest.NewRequest(target.method, "/api/v1"+target.path, strings.NewReader(target.body))
		req.Header.Set("Authorization", "Bearer "+owner)
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, req)
		if recorder.Code != 409 || !strings.Contains(recorder.Body.String(), model.ErrVersionConflict.Error()) {
			t.Fatalf("%s %s: want explicit 409 conflict, got %d %s", target.method, target.path, recorder.Code, recorder.Body.String())
		}
		after, currentSnapshots := readVersionHTTPState(t, created.ID)
		if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(snapshots, currentSnapshots) {
			t.Fatal("HTTP version conflict partially wrote article or snapshots")
		}
	}
	published := httpArticle(t, workflowRequest(t, h, "POST", path+"/publish", owner, `{"expected_version":2}`, 200))
	if published.Version != 2 || published.PublishedVersion != 2 {
		t.Fatal("matching publish failed")
	}
	_, afterPublish := readVersionHTTPState(t, created.ID)
	if !reflect.DeepEqual(snapshots, afterPublish) {
		t.Fatal("publish changed content history")
	}
	updated := httpArticle(t, workflowRequest(t, h, "PUT", path, owner, `{"expected_version":2,"content":"V3 human save"}`, 200))
	if updated.Version != 3 || updated.PublishedVersion != 3 {
		t.Fatal("matching legacy PUT did not atomically publish V3")
	}
}

func TestArticleHTTPRequiresPositiveExpectedVersion(t *testing.T) {
	h, owner, _ := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/articles/drafts", owner, `{"title":"Draft","content":"Original"}`, 201))
	path := fmt.Sprintf("/articles/%d", created.ID)
	before, snapshots := readVersionHTTPState(t, created.ID)
	for _, target := range []struct{ method, path string }{
		{"PUT", path}, {"PUT", path + "/draft"}, {"POST", path + "/publish"},
	} {
		for _, body := range []string{
			"", `{}`, `null`, `{"expected_version":null}`, `{"expected_version":0}`,
			`{"expected_version":-1}`, `{"expected_version":1.5}`, `{"expected_version":"1"}`,
		} {
			workflowRequest(t, h, target.method, target.path, owner, body, 400)
		}
	}
	after, currentSnapshots := readVersionHTTPState(t, created.ID)
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(snapshots, currentSnapshots) {
		t.Fatal("invalid expected_version modified the article")
	}
	// Archive intentionally remains independent of expected_version.
	workflowRequest(t, h, "POST", path+"/archive", owner, "", 200)
}
