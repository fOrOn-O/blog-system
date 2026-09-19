package router_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"blog-system/internal/database"
	"blog-system/internal/model"
	"blog-system/internal/service"
)

func TestArticleProposalApplicationRoutes(t *testing.T) {
	h, owner, other := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/articles", owner, `{"title":"标题","content":"<p>公开稿</p>"}`, 201))
	path := fmt.Sprintf("/articles/%d/edit-proposal", created.ID)
	body := `{"base_version_no":1,"proposed_content":"<p>批准稿</p>"}`
	for _, action := range []string{"preview", "apply"} {
		workflowRequest(t, h, "POST", path+"/"+action, "", body, 401)
		workflowRequest(t, h, "POST", path+"/"+action, "invalid-test-token", body, 401)
		workflowRequest(t, h, "POST", path+"/"+action, other, body, 403)
		workflowRequest(t, h, "POST", "/articles/999999/edit-proposal/"+action, owner, body, 404)
	}
	var before, after model.Article
	if err := database.DB.First(&before, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	var preview service.ArticleEditPreview
	if err := json.Unmarshal(workflowRequest(t, h, "POST", path+"/preview", owner, body, 200), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.BaseVersionNo != 1 || len(preview.Content.Changes) != 1 || preview.Content.Changes[0].Before.Text != "公开稿" ||
		preview.Content.Changes[0].After.Text != "批准稿" {
		t.Fatalf("preview=%+v", preview)
	}
	if err := database.DB.First(&after, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("preview mutated article")
	}
	var count int64
	if err := database.DB.Model(&model.ArticleVersion{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("snapshots=%d err=%v", count, err)
	}
	var applied service.ArticleEditApplied
	if err := json.Unmarshal(workflowRequest(t, h, "POST", path+"/apply", owner, body, 200), &applied); err != nil {
		t.Fatal(err)
	}
	if applied.ArticleID != created.ID || applied.PreviousVersionNo != 1 || applied.NewVersionNo != 2 || applied.Status != "draft" || applied.PublishedVersion != 1 {
		t.Fatalf("applied=%+v", applied)
	}
	// 同一请求重放必须冲突，不能再次创建版本或自动重放批准。
	workflowRequest(t, h, "POST", path+"/apply", owner, body, 409)
	workflowRequest(t, h, "POST", path+"/preview", owner, `{"base_version_no":99,"proposed_content":"x"}`, 404)
	public := httpArticle(t, workflowRequest(t, h, "GET", fmt.Sprintf("/articles/%d", created.ID), "", "", 200))
	if public.Content != created.Content || public.Version != 1 {
		t.Fatal("apply published proposal")
	}
	if err := database.DB.Model(&model.ArticleVersion{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("snapshots=%d err=%v", count, err)
	}
	workflowRequest(t, h, "POST", fmt.Sprintf("/articles/%d/archive", created.ID), owner, "", 200)
	workflowRequest(t, h, "POST", path+"/apply", owner, `{"base_version_no":2,"proposed_content":"x"}`, 409)
}

func TestArticleProposalRejectsUntrustedPayloadFields(t *testing.T) {
	h, owner, _ := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/articles/drafts", owner, `{"title":"标题","content":"原稿"}`, 201))
	for _, action := range []string{"preview", "apply"} {
		path := fmt.Sprintf("/articles/%d/edit-proposal/%s", created.ID, action)
		for _, body := range []string{
			`{}`, `null`, `{"base_version_no":0,"proposed_content":"x"}`,
			`{"base_version_no":-1,"proposed_content":"x"}`, `{"base_version_no":1.5,"proposed_content":"x"}`,
			`{"base_version_no":1}`, `{"base_version_no":1,"proposed_content":null}`,
			`{"base_version_no":1,"proposed_content":""}`, `{"base_version_no":1,"proposed_content":123}`,
			`{"base_version_no":1,"proposed_content":"x"} {}`,
		} {
			workflowRequest(t, h, "POST", path, owner, body, 400)
		}
		for _, field := range []string{"user_id", "owner_id", "article_id", "new_version_no", "published_version", "version", "status", "tag_ids", "title", "change_summary", "approved"} {
			workflowRequest(t, h, "POST", path, owner, `{"base_version_no":1,"proposed_content":"x","`+field+`":1}`, 400)
		}
		for _, id := range []string{"0", "-1", "abc", "4294967296"} {
			workflowRequest(t, h, "POST", "/articles/"+id+"/edit-proposal/"+action, owner, `{"base_version_no":1,"proposed_content":"x"}`, 400)
		}
	}
	var stored model.Article
	if err := database.DB.First(&stored, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Version != 1 || stored.Content != "原稿" {
		t.Fatal("invalid payload changed draft")
	}
}

func TestArticleProposalErrorResults(t *testing.T) {
	h, owner, _ := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/articles/drafts", owner, `{"title":"标题","content":"原稿"}`, 201))
	path := fmt.Sprintf("/articles/%d/edit-proposal/apply", created.ID)
	req := httptest.NewRequest("POST", "/api/v1"+path, strings.NewReader(`{"base_version_no":1,"proposed_content":"原稿"}`))
	req.Header.Set("Authorization", "Bearer "+owner)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	var envelope struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != 400 || envelope.Message != model.ErrArticleContentUnchanged.Error() {
		t.Fatalf("no-op not distinguishable: %s", recorder.Body)
	}
	if err := database.DB.Exec(`CREATE TRIGGER reject_apply AFTER INSERT ON article_versions BEGIN SELECT RAISE(ABORT, 'internal database detail'); END`).Error; err != nil {
		t.Fatal(err)
	}
	workflowRequest(t, h, "POST", path, owner, `{"base_version_no":1,"proposed_content":"新稿"}`, 500)
	if err := database.DB.Exec("DROP TRIGGER reject_apply").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Where("article_id = ?", created.ID).Delete(&model.ArticleVersion{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"apply", "preview"} {
		workflowRequest(t, h, "POST", fmt.Sprintf("/articles/%d/edit-proposal/%s", created.ID, action), owner,
			`{"base_version_no":1,"proposed_content":"新稿"}`, 404)
	}
}

type rejectProposalOutbound struct{ calls int }

func TestArticleProposalApplyReauthorizesAfterPreview(t *testing.T) {
	h, owner, other := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/articles/drafts", owner, `{"title":"标题","content":"原稿"}`, 201))
	path := fmt.Sprintf("/articles/%d/edit-proposal", created.ID)
	body := `{"base_version_no":1,"proposed_content":"批准稿"}`
	workflowRequest(t, h, "POST", path+"/preview", owner, body, 200)
	var newOwner model.User
	if err := database.DB.Where("username = ?", "user1").First(&newOwner).Error; err != nil {
		t.Fatal(err)
	}
	// 模拟预览后权限发生变化，Apply 不能依赖先前预览/提案的授权结果。
	if err := database.DB.Model(&model.Article{}).Where("id = ?", created.ID).Update("user_id", newOwner.ID).Error; err != nil {
		t.Fatal(err)
	}
	workflowRequest(t, h, "POST", path+"/apply", owner, body, 403)
	current := httpArticle(t, workflowRequest(t, h, "GET", fmt.Sprintf("/user/articles/%d", created.ID), other, "", 200))
	if current.Version != 1 || current.Content != "原稿" {
		t.Fatal("previous owner's approved proposal was applied")
	}
	var count int64
	if err := database.DB.Model(&model.ArticleVersion{}).Where("article_id = ?", created.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("unauthorized apply changed history: %d %v", count, err)
	}
}

func (r *rejectProposalOutbound) RoundTrip(*http.Request) (*http.Response, error) {
	r.calls++
	return nil, fmt.Errorf("unexpected outbound request during apply")
}

func TestArticleProposalApplyDoesNotCallExternalIndexing(t *testing.T) {
	h, owner, _ := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/articles/drafts", owner, `{"title":"标题","content":"原稿"}`, 201))
	previous := http.DefaultTransport
	outbound := &rejectProposalOutbound{}
	http.DefaultTransport = outbound
	t.Cleanup(func() { http.DefaultTransport = previous })
	workflowRequest(t, h, "POST", fmt.Sprintf("/articles/%d/edit-proposal/apply", created.ID), owner,
		`{"base_version_no":1,"proposed_content":"新稿"}`, 200)
	if outbound.calls != 0 {
		t.Fatal("apply attempted outbound indexing or embedding")
	}
}
