package router_test

import (
	"blog-system/internal/database"
	"blog-system/internal/model"
	"blog-system/internal/service"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestPublishedKnowledgeAuthority(t *testing.T) {
	h, owner, other := setupWorkflowHTTP(t)
	create := func(token, title string) service.ArticleResponse {
		return httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", token,
			fmt.Sprintf(`{"title":%q,"content":"<h2>公开知识</h2><p>V1</p>"}`, title), 201))
	}
	a, b, draft, archived := create(owner, "A"), create(other, "B"), create(owner, "私有草稿"), create(owner, "归档内容")
	path := func(id uint) string { return fmt.Sprintf("/agent/articles/%d", id) }
	for _, row := range []struct {
		article service.ArticleResponse
		token   string
	}{{a, owner}, {b, other}, {archived, owner}} {
		workflowRequest(t, h, "POST", path(row.article.ID)+"/publish", row.token, `{"expected_version":1}`, 200)
	}
	workflowRequest(t, h, "POST", path(archived.ID)+"/archive", owner, "", 200)
	for version := 2; version <= 7; version++ {
		workflowRequest(t, h, "PUT", path(a.ID)+"/draft", owner,
			fmt.Sprintf(`{"expected_version":%d,"title":"A V%d","content":"<p>V%d</p>"}`, version-1, version, version), 200)
		if version == 5 {
			workflowRequest(t, h, "POST", path(a.ID)+"/publish", owner, `{"expected_version":5}`, 200)
		}
	}
	before, versions := readVersionHTTPState(t, a.ID)
	read := func() []service.PublishedKnowledgeRecord {
		raw := workflowRequest(t, h, "GET", "/agent/published-knowledge", other, "", 200)
		var records []service.PublishedKnowledgeRecord
		if err := json.Unmarshal(raw, &records); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "V7") || strings.Contains(string(raw), "私有草稿") || strings.Contains(string(raw), "归档内容") {
			t.Fatal("private/history leaked")
		}
		return records
	}
	records := read()
	if len(records) != 2 || records[0].ArticleID != a.ID || records[0].PublishedVersion != 5 || records[0].Title != "A V5" || records[1].ArticleID != b.ID {
		t.Fatalf("incorrect published view: %+v", records)
	}
	if len(records[0].Chunks) != 1 || !strings.Contains(records[0].Chunks[0].Text, "V5") {
		t.Fatal("wrong canonical chunks")
	}
	after, history := readVersionHTTPState(t, a.ID)
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(versions, history) {
		t.Fatal("source read mutated state")
	}
	for _, id := range []uint{draft.ID, archived.ID, 99999} {
		raw := workflowRequest(t, h, "GET", fmt.Sprintf("/agent/published-knowledge/%d", id), other, "", 200)
		if string(raw) != "[]" {
			t.Fatal("inactive source must be empty")
		}
	}
	for _, token := range []string{"", "invalid"} {
		workflowRequest(t, h, "GET", "/agent/published-knowledge", token, "", 401)
		workflowRequest(t, h, "GET", fmt.Sprintf("/agent/published-knowledge/%d", a.ID), token, "", 401)
	}
	for _, id := range []string{"0", "-1", "bad", "4294967296"} {
		workflowRequest(t, h, "GET", "/agent/published-knowledge/"+id, other, "", 400)
	}
	// 发布事务不涉及 Python/Qdrant；测试无需这些服务也能成功推进公开指针。
	workflowRequest(t, h, "POST", path(a.ID)+"/publish", owner, `{"expected_version":7}`, 200)
	raw := workflowRequest(t, h, "GET", fmt.Sprintf("/agent/published-knowledge/%d", a.ID), other, "", 200)
	if !strings.Contains(string(raw), `"published_version":7`) || strings.Contains(string(raw), "V5") {
		t.Fatal("republish did not replace active source")
	}
	// 缺失公开快照和软删除均不可成为知识源。
	if err := database.DB.Where("article_id = ? AND version_no = ?", a.ID, 7).Delete(&model.ArticleVersion{}).Error; err != nil {
		t.Fatal(err)
	}
	workflowRequest(t, h, "DELETE", fmt.Sprintf("/articles/%d", b.ID), other, "", 200)
	if raw := workflowRequest(t, h, "GET", "/agent/published-knowledge", owner, "", 200); string(raw) != "[]" {
		t.Fatal("missing/deleted snapshots leaked")
	}
}
