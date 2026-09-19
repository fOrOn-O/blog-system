package router_test

import (
	"blog-system/internal/service"
	"encoding/json"
	"fmt"
	"testing"
)

func TestOwnedVersionHistoryIsReadOnlyAndOwnerScoped(t *testing.T) {
	h, owner, other := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/articles/drafts", owner, `{"title":"版本测试","content":"<p>V1</p>"}`, 201))
	id := created.ID
	workflowRequest(t, h, "PUT", fmt.Sprintf("/articles/%d/draft", id), owner, `{"expected_version":1,"content":"<p>V2</p>"}`, 200)
	path := fmt.Sprintf("/user/articles/%d/versions", id)
	workflowRequest(t, h, "GET", path, "", "", 401)
	workflowRequest(t, h, "GET", path, other, "", 403)
	workflowRequest(t, h, "GET", "/user/articles/99999/versions", owner, "", 404)
	workflowRequest(t, h, "GET", "/user/articles/0/versions", owner, "", 400)
	var versions []service.ArticleVersionSummary
	if err := json.Unmarshal(workflowRequest(t, h, "GET", path, owner, "", 200), &versions); err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0].VersionNo != 2 || versions[1].VersionNo != 1 || versions[0].ArticleID != id {
		t.Fatalf("history=%+v", versions)
	}
	var page []service.ArticleVersionSummary
	if err := json.Unmarshal(workflowRequest(t, h, "GET", path+"?page=2&limit=1", owner, "", 200), &page); err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].VersionNo != 1 {
		t.Fatal("pagination")
	}
	after := httpArticle(t, workflowRequest(t, h, "GET", fmt.Sprintf("/user/articles/%d", id), owner, "", 200))
	if after.Version != 2 || after.PublishedVersion != 0 || after.Content != "<p>V2</p>" || after.ViewCount != 0 {
		t.Fatal("history mutated article")
	}
}
