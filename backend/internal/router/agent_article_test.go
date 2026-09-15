package router_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"testing"

	"blog-system/internal/database"
	"blog-system/internal/model"
	"blog-system/internal/service"
	"blog-system/pkg/auth"
	"blog-system/pkg/response"
)

func TestAgentArticleWorkingVersionAndWorkflow(t *testing.T) {
	h, owner, _ := setupWorkflowHTTP(t)
	claims, err := auth.ValidateToken(owner)
	if err != nil {
		t.Fatal(err)
	}
	tags := []model.Tag{{Name: "original"}, {Name: "replacement"}}
	if err := database.DB.Create(&tags).Error; err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"title":"Draft","content":"<p>V1</p>","summary":"Summary","cover_image":"/uploads/cover.png","tag_ids":[%d]}`, tags[0].ID)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, body, 201))
	path := fmt.Sprintf("/agent/articles/%d", created.ID)
	stored, snapshots := readVersionHTTPState(t, created.ID)
	if created.Status != "draft" || created.Version != 1 || created.PublishedVersion != 0 || stored.UserID != claims.UserID {
		t.Fatalf("agent create must create the JWT owner's draft: %+v", stored)
	}
	if len(snapshots) != 1 || snapshots[0].VersionNo != 1 || snapshots[0].CreatedBy != claims.UserID || snapshots[0].Content != created.Content {
		t.Fatal("agent create did not retain V1 snapshot semantics")
	}
	draft := httpArticle(t, workflowRequest(t, h, "GET", path, owner, "", 200))
	if draft.Status != "draft" || draft.Content != created.Content || draft.ViewCount != 0 {
		t.Fatal("owner cannot read the draft without recording a view")
	}
	for version := 2; version <= 5; version++ {
		body := fmt.Sprintf(`{"expected_version":%d,"content":"<p>V%d</p>"}`, version-1, version)
		working := httpArticle(t, workflowRequest(t, h, "PUT", path+"/draft", owner, body, 200))
		if working.Version != uint(version) || working.PublishedVersion != 0 || working.Status != "draft" {
			t.Fatalf("draft update published content: %+v", working)
		}
	}
	workflowRequest(t, h, "POST", path+"/publish", owner, `{"expected_version":5}`, 200)
	working := httpArticle(t, workflowRequest(t, h, "PUT", path+"/draft", owner, `{"expected_version":5,"content":"<p>Private V6</p>"}`, 200))
	if working.Version != 6 || working.PublishedVersion != 5 || working.Status != "published" {
		t.Fatalf("agent draft edit advanced the public version: %+v", working)
	}
	beforeRead, beforeSnapshots := readVersionHTTPState(t, created.ID)
	owned := httpArticle(t, workflowRequest(t, h, "GET", path, owner, "", 200))
	if owned.Version != 6 || owned.PublishedVersion != 5 || owned.Content != working.Content {
		t.Fatal("agent get did not return the latest working version")
	}
	afterRead, afterSnapshots := readVersionHTTPState(t, created.ID)
	if !reflect.DeepEqual(beforeRead, afterRead) || !reflect.DeepEqual(beforeSnapshots, afterSnapshots) {
		t.Fatal("agent get changed the article, view count or history")
	}
	public := httpArticle(t, workflowRequest(t, h, "GET", fmt.Sprintf("/articles/%d", created.ID), "", "", 200))
	if public.Version != 5 || public.PublishedVersion != 5 || public.Content != "<p>V5</p>" {
		t.Fatal("public get leaked the working version")
	}
	beforeConflict, versions := readVersionHTTPState(t, created.ID)
	for _, target := range []struct{ method, suffix, body string }{
		{"PUT", "/draft", fmt.Sprintf(`{"expected_version":5,"content":"Rejected","tag_ids":[%d]}`, tags[1].ID)},
		{"POST", "/publish", `{"expected_version":5}`},
	} {
		workflowRequest(t, h, target.method, path+target.suffix, owner, target.body, 409)
		after, history := readVersionHTTPState(t, created.ID)
		if !reflect.DeepEqual(beforeConflict, after) || !reflect.DeepEqual(versions, history) {
			t.Fatal("agent version conflict changed article, tags or snapshots")
		}
	}
	published := httpArticle(t, workflowRequest(t, h, "POST", path+"/publish", owner, `{"expected_version":6}`, 200))
	_, history := readVersionHTTPState(t, created.ID)
	if published.Version != 6 || published.PublishedVersion != 6 || !reflect.DeepEqual(versions, history) {
		t.Fatal("agent publish must only publish the approved version without a new snapshot")
	}
	workflowRequest(t, h, "POST", path+"/archive", owner, "", 200)
	archived, archivedVersions := readVersionHTTPState(t, created.ID)
	workflowRequest(t, h, "POST", path+"/archive", owner, "", 200)
	owned = httpArticle(t, workflowRequest(t, h, "GET", path, owner, "", 200))
	if owned.Status != "archived" || owned.Content != working.Content || owned.Version != 6 || owned.PublishedVersion != 6 {
		t.Fatal("owner cannot read the archived working version")
	}
	workflowRequest(t, h, "PUT", path+"/draft", owner, `{"expected_version":6,"content":"Rejected"}`, 409)
	workflowRequest(t, h, "POST", path+"/publish", owner, `{"expected_version":6}`, 409)
	after, history := readVersionHTTPState(t, created.ID)
	if !reflect.DeepEqual(archived, after) || !reflect.DeepEqual(archivedVersions, history) {
		t.Fatal("archive was not idempotent or archived read/edit/publish changed state")
	}
}

func TestAgentArticlesRequireJWTAndOwnership(t *testing.T) {
	h, owner, otherAdmin := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, `{"title":"Private","content":"Private"}`, 201))
	path := fmt.Sprintf("/agent/articles/%d", created.ID)
	before, versions := readVersionHTTPState(t, created.ID)
	for _, target := range []struct{ method, path, body string }{
		{"GET", "/agent/articles", ""},
		{"GET", path, ""},
		{"POST", "/agent/articles", `{"title":"Rejected","content":"Rejected"}`},
		{"PUT", path + "/draft", `{"expected_version":1,"content":"Rejected"}`},
		{"POST", path + "/publish", `{"expected_version":1}`},
		{"POST", path + "/archive", ""},
	} {
		for _, token := range []string{"", "invalid-jwt"} {
			workflowRequest(t, h, target.method, target.path, token, target.body, 401)
		}
	}
	// Even another administrator has no owner access through Agent routes.
	for _, target := range []struct{ method, suffix, body string }{
		{"GET", "", ""},
		{"PUT", "/draft", `{"expected_version":1,"content":"Rejected"}`},
		{"POST", "/publish", `{"expected_version":1}`},
		{"POST", "/archive", ""},
	} {
		workflowRequest(t, h, target.method, path+target.suffix, otherAdmin, target.body, 403)
		workflowRequest(t, h, target.method, path+target.suffix+fmt.Sprintf("?user_id=%d", before.UserID), otherAdmin, target.body, 403)
		workflowRequest(t, h, target.method, "/agent/articles/999999"+target.suffix, owner, target.body, 404)
		workflowRequest(t, h, target.method, "/agent/articles/not-an-id"+target.suffix, owner, target.body, 400)
	}
	spoof := fmt.Sprintf(`{"user_id":%d}`, before.UserID)
	workflowRequest(t, h, "GET", path, otherAdmin, spoof, 403)
	workflowRequest(t, h, "POST", path+"/archive", otherAdmin, spoof, 403)
	spoof = fmt.Sprintf(`{"expected_version":1,"user_id":%d}`, before.UserID)
	workflowRequest(t, h, "PUT", path+"/draft", otherAdmin, spoof, 400)
	workflowRequest(t, h, "POST", path+"/publish", otherAdmin, spoof, 400)
	after, history := readVersionHTTPState(t, created.ID)
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(versions, history) {
		t.Fatal("unauthorized requests changed the owner's data")
	}
}

func TestAgentArticlesRejectDangerousFieldsAndMissingVersions(t *testing.T) {
	h, owner, otherAdmin := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, `{"title":"Private","content":"Private"}`, 201))
	path := fmt.Sprintf("/agent/articles/%d", created.ID)
	before, versions := readVersionHTTPState(t, created.ID)
	for _, field := range []string{
		`"user_id":999`, `"status":"published"`, `"status":"archived"`, `"version":99`,
		`"published_version":99`, `"view_count":99`, `"like_count":99`, `"comment_count":99`,
	} {
		workflowRequest(t, h, "POST", "/agent/articles", owner, `{"title":"Injected","content":"Injected",`+field+`}`, 400)
		workflowRequest(t, h, "PUT", path+"/draft", owner, `{"expected_version":1,"content":"Injected",`+field+`}`, 400)
	}
	// A caller cannot create an article for someone else by supplying their ID.
	workflowRequest(t, h, "POST", "/agent/articles", otherAdmin, fmt.Sprintf(`{"title":"Forged","content":"Forged","user_id":%d}`, before.UserID), 400)
	for _, target := range []struct{ method, suffix string }{{"PUT", "/draft"}, {"POST", "/publish"}} {
		for _, body := range []string{"", `{}`, `{"expected_version":0}`, `{"expected_version":-1}`} {
			workflowRequest(t, h, target.method, path+target.suffix, owner, body, 400)
		}
	}
	after, history := readVersionHTTPState(t, created.ID)
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(versions, history) {
		t.Fatal("invalid Agent requests changed content, status, ownership, counters or snapshots")
	}
	var count int64
	if err := database.DB.Model(&model.Article{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("rejected creates left articles behind: count=%d err=%v", count, err)
	}
}

func TestAgentArticleListIsOwnerScopedAndPaginated(t *testing.T) {
	h, owner, otherAdmin := setupWorkflowHTTP(t)
	ids := make(map[string]uint)
	for _, status := range []string{"draft", "published", "archived"} {
		created := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", owner, `{"title":"Own","content":"Private"}`, 201))
		path := fmt.Sprintf("/agent/articles/%d", created.ID)
		if status == "published" {
			workflowRequest(t, h, "POST", path+"/publish", owner, `{"expected_version":1}`, 200)
			workflowRequest(t, h, "PUT", path+"/draft", owner, `{"expected_version":1,"content":"Unpublished working content"}`, 200)
		} else if status == "archived" {
			workflowRequest(t, h, "POST", path+"/archive", owner, "", 200)
		}
		ids[status] = created.ID
	}
	other := httpArticle(t, workflowRequest(t, h, "POST", "/agent/articles", otherAdmin, `{"title":"Other","content":"Other"}`, 201))
	list := func(token, query string) ([]service.ArticleResponse, response.Meta) {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/agent/articles"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, req)
		if recorder.Code != 200 {
			t.Fatalf("agent list failed: %d %s", recorder.Code, recorder.Body.String())
		}
		var result struct {
			Data []service.ArticleResponse `json:"data"`
			Meta response.Meta             `json:"meta"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Data, result.Meta
	}
	seen := make(map[uint]bool)
	for page := 1; page <= 3; page++ {
		items, meta := list(owner, fmt.Sprintf("?page=%d&limit=1&user_id=%d", page, other.User.ID))
		if len(items) != 1 || meta.Page != page || meta.Limit != 1 || meta.Total != 3 || meta.Pages != 3 {
			t.Fatalf("incorrect owner pagination: items=%+v meta=%+v", items, meta)
		}
		item := items[0]
		if ids[item.Status] != item.ID || seen[item.ID] {
			t.Fatal("pagination returned another owner's article or repeated an article")
		}
		if item.Status == "published" && (item.Version != 2 || item.PublishedVersion != 1 || item.Content != "Unpublished working content") {
			t.Fatal("agent list returned public content instead of working content")
		}
		seen[item.ID] = true
	}
	for status, id := range ids {
		items, meta := list(owner, "?status="+status)
		if len(items) != 1 || items[0].ID != id || meta.Total != 1 {
			t.Fatalf("owner status filter failed: %s", status)
		}
		stored, _ := readVersionHTTPState(t, id)
		if stored.ViewCount != 0 {
			t.Fatal("agent list incremented view count")
		}
	}
	items, meta := list(otherAdmin, "?user_id=1")
	if len(items) != 1 || items[0].ID != other.ID || meta.Total != 1 {
		t.Fatal("query user_id overrode the authenticated owner's scope")
	}
	workflowRequest(t, h, "GET", "/agent/articles?status=invalid", owner, "", 400)
}
