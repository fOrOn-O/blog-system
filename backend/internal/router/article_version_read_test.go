package router_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"blog-system/internal/database"
	"blog-system/internal/service"
)

func TestAgentReadCanonicalArticleVersionIsOwnerOnlyAndReadOnly(t *testing.T) {
	h, owner, other := setupWorkflowHTTP(t)
	article := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner,
		`{"title":"旧标题","content":"<h1>Redis</h1><p><strong>旧版</strong>正文</p>","summary":"旧摘要","cover_image":"/old.png"}`, 201))
	base := fmt.Sprintf("/agent/articles/%d", article.ID)
	workflowRequest(t, h, "POST", base+"/publish", owner, `{"expected_version":1}`, 200)
	workflowRequest(t, h, "PUT", base+"/draft", owner,
		`{"expected_version":1,"title":"新标题","content":"<p>新工作正文</p>","summary":"新摘要","cover_image":"/new.png"}`, 200)
	before, history := readVersionHTTPState(t, article.ID)
	key := fmt.Sprintf("article:%d", article.ID)
	database.CacheSet(key, "untouched", time.Hour)
	for _, version := range history {
		path := fmt.Sprintf("%s/versions/%d", base, version.VersionNo)
		// 第一次供 Agent 阅读，第二次模拟提交提案时的重新校验。
		for read := 0; read < 2; read++ {
			raw := workflowRequest(t, h, "GET", path, owner, "", 200)
			var response service.ArticleVersionResponse
			if err := json.Unmarshal(raw, &response); err != nil {
				t.Fatal(err)
			}
			if response.ArticleID != article.ID || response.VersionNo != version.VersionNo ||
				response.Content != version.Content || response.Title != version.Title ||
				response.Summary != version.Summary || response.CoverImage != version.CoverImage {
				t.Fatalf("did not read the exact snapshot: %+v", response)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			if len(fields) != 6 {
				t.Fatalf("unexpected snapshot fields: %s", raw)
			}
		}
	}
	for _, token := range []string{"", "invalid-jwt"} {
		workflowRequest(t, h, "GET", base+"/versions/1", token, "", 401)
	}
	for _, version := range []string{"1", "999"} {
		req := httptest.NewRequest("GET", "/api/v1"+base+"/versions/"+version+"?user_id=1", nil)
		req.Header.Set("Authorization", "Bearer "+other)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 403 || strings.Contains(rec.Body.String(), "Redis") || strings.Contains(rec.Body.String(), "正文") {
			t.Fatal("another owner/admin can access snapshot data")
		}
	}
	for _, invalid := range []string{"0", "-1", "abc", "4294967296"} {
		workflowRequest(t, h, "GET", base+"/versions/"+invalid, owner, "", 400)
		workflowRequest(t, h, "GET", "/agent/articles/"+invalid+"/versions/1", owner, "", 400)
	}
	workflowRequest(t, h, "GET", base+"/versions/999", owner, "", 404)
	workflowRequest(t, h, "GET", "/agent/articles/999999/versions/1", owner, "", 404)
	after, afterHistory := readVersionHTTPState(t, article.ID)
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(history, afterHistory) {
		t.Fatal("reading changed content, article versions, published version, tags or counters")
	}
	if value, err := database.CacheGet(key); err != nil || value != "untouched" {
		t.Fatal("read invalidated cache")
	}
	workflowRequest(t, h, "DELETE", fmt.Sprintf("/articles/%d", article.ID), owner, "", 200)
	workflowRequest(t, h, "GET", base+"/versions/1", owner, "", 404)
}

func TestAgentReadVersionNeverFallsBackToAnotherSnapshot(t *testing.T) {
	h, owner, _ := setupWorkflowHTTP(t)
	a := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, `{"title":"A","content":"<p>A</p>"}`, 201))
	b := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, `{"title":"B","content":"<p>B</p>"}`, 201))
	workflowRequest(t, h, "PUT", fmt.Sprintf("/agent/articles/%d/draft", b.ID), owner, `{"expected_version":1,"content":"<p>B2</p>"}`, 200)
	workflowRequest(t, h, "GET", fmt.Sprintf("/agent/articles/%d/versions/2", a.ID), owner, "", 404)
}
