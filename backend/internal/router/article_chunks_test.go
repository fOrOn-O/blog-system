package router_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"blog-system/internal/chunking"
	"blog-system/internal/service"
)

func TestAgentVersionChunks(t *testing.T) {
	h, owner, other := setupWorkflowHTTP(t)
	html := `<h1>Redis</h1><h2>持久化</h2><h3>RDB</h3><p>快照</p><pre><code>save 60 1</code></pre><h2>AOF</h2><p>日志</p>`
	body, _ := json.Marshal(map[string]string{"title": "Chunks", "content": html})
	article := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, string(body), 201))
	base := fmt.Sprintf("/agent/articles/%d", article.ID)
	workflowRequest(t, h, "PUT", base+"/draft", owner, `{"expected_version":1,"content":"<p>新版</p>"}`, 200)
	before, snapshots := readVersionHTTPState(t, article.ID)
	path := base + "/versions/1/chunks"
	raw := workflowRequest(t, h, "GET", path, owner, "", 200)
	if string(raw) != string(workflowRequest(t, h, "GET", path, owner, "", 200)) {
		t.Fatal("chunk response is not deterministic")
	}
	var result service.ArticleVersionChunks
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.ArticleID != article.ID || result.VersionNo != 1 || result.UserID != before.UserID {
		t.Fatalf("wrong identity: %+v", result)
	}
	expected, err := chunking.ChunkHTML(html, chunking.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Chunks) != len(expected) || len(expected) < 2 {
		t.Fatal("wrong chunk count")
	}
	for i, chunk := range result.Chunks {
		if chunk.Index != i || !reflect.DeepEqual(chunk.Chunk, expected[i]) || chunk.Text != chunking.RenderText(expected[i]) {
			t.Fatalf("chunk differs from Task 09: %+v", chunk)
		}
	}
	if len(result.Chunks[0].HeadingPath) != 3 {
		t.Fatal("heading hierarchy lost")
	}
	latest := workflowRequest(t, h, "GET", base+"/versions/2/chunks", owner, "", 200)
	if !strings.Contains(string(latest), "新版") || strings.Contains(string(raw), "新版") {
		t.Fatal("wrong snapshot content")
	}
	for _, token := range []string{"", "invalid-jwt"} {
		workflowRequest(t, h, "GET", path, token, "", 401)
	}
	for _, version := range []string{"1", "99"} {
		req := httptest.NewRequest("GET", "/api/v1"+base+"/versions/"+version+"/chunks?user_id=1", nil)
		req.Header.Set("Authorization", "Bearer "+other)
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, req)
		if recorder.Code != 403 || strings.Contains(recorder.Body.String(), "Redis") {
			t.Fatal("other owner read snapshot")
		}
	}
	for _, suffix := range []string{"0", "-1", "x", "4294967296"} {
		workflowRequest(t, h, "GET", base+"/versions/"+suffix+"/chunks", owner, "", 400)
		workflowRequest(t, h, "GET", "/agent/articles/"+suffix+"/versions/1/chunks", owner, "", 400)
	}
	workflowRequest(t, h, "GET", base+"/versions/99/chunks", owner, "", 404)
	workflowRequest(t, h, "GET", "/agent/articles/999999/versions/1/chunks", owner, "", 404)
	after, history := readVersionHTTPState(t, article.ID)
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(snapshots, history) {
		t.Fatal("read changed article or versions")
	}
	workflowRequest(t, h, "DELETE", fmt.Sprintf("/articles/%d", article.ID), owner, "", 200)
	workflowRequest(t, h, "GET", path, owner, "", 404)
}

func TestChunksCannotUseAnotherArticleVersion(t *testing.T) {
	h, owner, _ := setupWorkflowHTTP(t)
	a := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, `{"title":"A","content":"<p>A</p>"}`, 201))
	b := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, `{"title":"B","content":"<p>B</p>"}`, 201))
	workflowRequest(t, h, "PUT", fmt.Sprintf("/agent/articles/%d/draft", b.ID), owner, `{"expected_version":1,"content":"<p>B2</p>"}`, 200)
	workflowRequest(t, h, "GET", fmt.Sprintf("/agent/articles/%d/versions/2/chunks", a.ID), owner, "", 404)
	// 空正文仍是合法历史版本，响应必须是 []，而不是 null。
	empty := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, `{"title":"Empty","content":"<p></p>"}`, 201))
	raw := workflowRequest(t, h, "GET", fmt.Sprintf("/agent/articles/%d/versions/1/chunks", empty.ID), owner, "", 200)
	if !strings.Contains(string(raw), `"chunks":[]`) {
		t.Fatal("empty chunks must be an array")
	}
}
