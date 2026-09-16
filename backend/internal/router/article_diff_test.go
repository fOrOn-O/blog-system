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

func TestAgentVersionDiff(t *testing.T) {
	h, owner, other := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner,
		`{"title":"Title 1","content":"<p>Redis很快</p>","summary":"Summary 1","cover_image":"/1.png"}`, 201))
	path := fmt.Sprintf("/agent/articles/%d", created.ID)
	for v := 2; v <= 6; v++ {
		body := fmt.Sprintf(`{"expected_version":%d,"title":"Title %d","summary":"Summary %d","cover_image":"/%d.png","content":"<p>Redis性能 %d</p>"}`, v-1, v, v, v, v)
		workflowRequest(t, h, "PUT", path+"/draft", owner, body, 200)
	}
	before, snapshots := readVersionHTTPState(t, created.ID)
	cacheKey := fmt.Sprintf("article:%d", created.ID)
	database.CacheSet(cacheKey, "keep-cache", time.Hour)
	for _, pair := range [][2]uint{{1, 2}, {2, 6}, {5, 6}} {
		data := workflowRequest(t, h, "GET", fmt.Sprintf("%s/diff?from_version=%d&to_version=%d", path, pair[0], pair[1]), owner, "", 200)
		var diff service.ArticleVersionDiff
		if err := json.Unmarshal(data, &diff); err != nil {
			t.Fatal(err)
		}
		if diff.ArticleID != created.ID || diff.FromVersion != pair[0] || diff.ToVersion != pair[1] {
			t.Fatalf("wrong versions: %+v", diff)
		}
		fields := diff.FieldChanges
		if fields.Title.Before != fmt.Sprintf("Title %d", pair[0]) || fields.Title.After != fmt.Sprintf("Title %d", pair[1]) || !fields.Title.Changed {
			t.Fatal("wrong title change")
		}
		if fields.Summary.Before != fmt.Sprintf("Summary %d", pair[0]) || fields.Summary.After != fmt.Sprintf("Summary %d", pair[1]) || !fields.Summary.Changed {
			t.Fatal("wrong summary change")
		}
		if fields.CoverImage.Before != fmt.Sprintf("/%d.png", pair[0]) || fields.CoverImage.After != fmt.Sprintf("/%d.png", pair[1]) || !fields.CoverImage.Changed {
			t.Fatal("wrong cover change")
		}
		if !diff.Content.Changed || len(diff.Content.Changes) != 1 || diff.Content.Changes[0].Operation != "modify" || diff.Content.Changes[0].After.Text != fmt.Sprintf("Redis性能 %d", pair[1]) {
			t.Fatalf("wrong content diff: %+v", diff.Content)
		}
		for _, excluded := range []string{`"status"`, `"tags"`, `"published_version"`, `"user_id"`, `"created_by"`} {
			if strings.Contains(string(data), excluded) {
				t.Fatalf("non-versioned field leaked: %s", excluded)
			}
		}
	}
	for _, query := range []string{"", "?from_version=1", "?from_version=0&to_version=2", "?from_version=2&to_version=2", "?from_version=3&to_version=2", "?from_version=-1&to_version=2", "?from_version=x&to_version=2", "?from_version=1&to_version=4294967296"} {
		workflowRequest(t, h, "GET", path+"/diff"+query, owner, "", 400)
	}
	for _, token := range []string{"", "invalid-jwt"} {
		workflowRequest(t, h, "GET", path+"/diff?from_version=1&to_version=2", token, "", 401)
	}
	for _, query := range []string{"?from_version=1&to_version=2", "?from_version=90&to_version=91&user_id=1"} {
		// 其他管理员也不能查看快照，或根据响应推断哪些历史版本存在。
		req := httptest.NewRequest("GET", "/api/v1"+path+"/diff"+query, nil)
		req.Header.Set("Authorization", "Bearer "+other)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 403 || strings.Contains(rec.Body.String(), "Title") || strings.Contains(rec.Body.String(), "Redis") {
			t.Fatal("ownership isolation failed")
		}
	}
	workflowRequest(t, h, "GET", "/agent/articles/999999/diff?from_version=1&to_version=2", owner, "", 404)
	workflowRequest(t, h, "GET", path+"/diff?from_version=7&to_version=8", owner, "", 404)
	workflowRequest(t, h, "GET", path+"/diff?from_version=1&to_version=7", owner, "", 404)
	after, history := readVersionHTTPState(t, created.ID)
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(snapshots, history) {
		t.Fatal("diff changed article, versions, counters or tags")
	}
	if cached, err := database.CacheGet(cacheKey); err != nil || cached != "keep-cache" {
		t.Fatal("diff invalidated cache")
	}
}

func TestDiffUsesOnlyRequestedArticleSnapshots(t *testing.T) {
	h, owner, _ := setupWorkflowHTTP(t)
	a := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, `{"title":"A","content":"<p>A</p>"}`, 201))
	b := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, `{"title":"B","content":"<p>B</p>"}`, 201))
	path := fmt.Sprintf("/agent/articles/%d", b.ID)
	workflowRequest(t, h, "PUT", path+"/draft", owner, `{"expected_version":1,"content":"<p>B2</p>"}`, 200)
	workflowRequest(t, h, "GET", fmt.Sprintf("/agent/articles/%d/diff?from_version=1&to_version=2", a.ID), owner, "", 404)
	// 单独覆盖起始版本缺失、目标版本存在的情况，而非仅测试两个版本均缺失。
	if err := database.DB.Exec("DELETE FROM article_versions WHERE article_id = ? AND version_no = 1", b.ID).Error; err != nil {
		t.Fatal(err)
	}
	workflowRequest(t, h, "GET", path+"/diff?from_version=1&to_version=2", owner, "", 404)
}

func TestDiffIdenticalContentAndMetadataOnlyChanges(t *testing.T) {
	h, owner, _ := setupWorkflowHTTP(t)
	a := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, `{"title":"A","content":"<p>Redis</p>"}`, 201))
	path := fmt.Sprintf("/agent/articles/%d", a.ID)
	for i, body := range []string{
		`{"expected_version":1,"title":"B"}`,
		`{"expected_version":2,"summary":"new summary"}`,
		`{"expected_version":3,"content":"<p><strong>Redis</strong></p>"}`,
	} {
		workflowRequest(t, h, "PUT", path+"/draft", owner, body, 200)
		data := workflowRequest(t, h, "GET", fmt.Sprintf("%s/diff?from_version=%d&to_version=%d", path, i+1, i+2), owner, "", 200)
		var diff service.ArticleVersionDiff
		if err := json.Unmarshal(data, &diff); err != nil {
			t.Fatal(err)
		}
		if diff.Content.Changed || diff.Content.Changes == nil || len(diff.Content.Changes) != 0 {
			t.Fatal("equivalent content changed")
		}
		if diff.FieldChanges.Title.Changed != (i == 0) || diff.FieldChanges.Summary.Changed != (i == 1) || diff.FieldChanges.CoverImage.Changed {
			t.Fatal("incorrect field changes")
		}
	}
	// 数据库故障应返回不泄露内部信息的 500，不能误报为历史版本缺失或客户端错误。
	if err := database.DB.Exec("DROP TABLE article_versions").Error; err != nil {
		t.Fatal(err)
	}
	workflowRequest(t, h, "GET", path+"/diff?from_version=1&to_version=2", owner, "", 500)
}
