package router_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"blog-system/internal/config"
	"blog-system/internal/database"
	"blog-system/internal/model"
	"blog-system/internal/router"
	"blog-system/internal/service"
	"blog-system/pkg/auth"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupWorkflowHTTP(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previousDB, previousConfig := database.DB, config.AppConfig
	database.DB = db
	config.AppConfig = &config.Config{JWT: config.JWT{Secret: "workflow-test-secret", Expiration: 1}}
	if err := db.AutoMigrate(&model.User{}, &model.Article{}, &model.ArticleVersion{}, &model.Tag{}, &model.Comment{}, &model.Like{}, &model.Favorite{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
		database.DB, config.AppConfig = previousDB, previousConfig
		database.CacheDeletePrefix("article:")
		database.CacheDeletePrefix("articles:list:")
	})
	tokens := make([]string, 2)
	for i := range tokens {
		user := model.User{Username: fmt.Sprintf("user%d", i), Email: fmt.Sprintf("user%d@example.com", i), Password: "hashed", IsActive: true, Role: "user"}
		if i == 1 {
			user.Role = "admin"
		}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		tokens[i], err = auth.GenerateToken(user.ID, user.Username, user.Role)
		if err != nil {
			t.Fatal(err)
		}
	}
	gin.SetMode(gin.TestMode)
	return router.SetupRouter(), tokens[0], tokens[1]
}

func workflowRequest(t *testing.T, h http.Handler, method, path, token, body string, wantStatus int) json.RawMessage {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	if recorder.Code != wantStatus {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, recorder.Code, wantStatus, recorder.Body.String())
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func httpArticle(t *testing.T, data json.RawMessage) service.ArticleResponse {
	t.Helper()
	var article service.ArticleResponse
	if err := json.Unmarshal(data, &article); err != nil {
		t.Fatal(err)
	}
	return article
}

func TestArticleWorkflowRoutes(t *testing.T) {
	h, owner, _ := setupWorkflowHTTP(t)
	draft := httpArticle(t, workflowRequest(t, h, "POST", "/articles/drafts", owner, `{"title":"Initial","content":"<p>Initial</p>"}`, 201))
	path := fmt.Sprintf("/articles/%d", draft.ID)
	ownedPath := fmt.Sprintf("/user/articles/%d", draft.ID)
	if draft.Status != "draft" || draft.Version != 1 || draft.PublishedVersion != 0 {
		t.Fatalf("draft=%+v", draft)
	}
	workflowRequest(t, h, "GET", path, "", "", 404)
	for _, publicPath := range []string{"/articles?status=draft", "/articles/search?keyword=Initial"} {
		if data := workflowRequest(t, h, "GET", publicPath, "", "", 200); string(data) != "[]" {
			t.Fatalf("private draft leaked: %s", data)
		}
	}
	owned := httpArticle(t, workflowRequest(t, h, "GET", ownedPath, owner, "", 200))
	if owned.ViewCount != 0 {
		t.Fatal("owner get counted a view")
	}
	published := httpArticle(t, workflowRequest(t, h, "POST", path+"/publish", owner, "", 200))
	if published.Status != "published" || published.PublishedVersion != 1 || published.Version != 1 {
		t.Fatalf("publish=%+v", published)
	}
	working := httpArticle(t, workflowRequest(t, h, "PUT", path+"/draft", owner, `{"content":"<p>PrivateSecret</p>"}`, 200))
	if working.Version != 2 || working.PublishedVersion != 1 || working.Status != "published" {
		t.Fatalf("working=%+v", working)
	}
	public := httpArticle(t, workflowRequest(t, h, "GET", path, "", "", 200))
	if public.Content != draft.Content {
		t.Fatal("public read returned private working content")
	}
	if data := workflowRequest(t, h, "GET", "/articles/search?keyword=PrivateSecret", "", "", 200); string(data) != "[]" {
		t.Fatalf("search leaked: %s", data)
	}
	workflowRequest(t, h, "POST", path+"/publish", owner, "", 200)
	public = httpArticle(t, workflowRequest(t, h, "GET", path, "", "", 200))
	if public.Content != working.Content || public.Version != 2 {
		t.Fatal("pending publish failed")
	}
	workflowRequest(t, h, "POST", path+"/archive", owner, "", 200)
	workflowRequest(t, h, "POST", path+"/archive", owner, "", 200)
	workflowRequest(t, h, "GET", path, "", "", 404)
	workflowRequest(t, h, "PUT", path+"/draft", owner, `{"content":"forbidden"}`, 409)
	workflowRequest(t, h, "PUT", path, owner, `{"content":"forbidden"}`, 409)
	workflowRequest(t, h, "POST", path+"/publish", owner, "", 409)
	var ownedList []service.ArticleResponse
	if err := json.Unmarshal(workflowRequest(t, h, "GET", "/user/articles?status=archived", owner, "", 200), &ownedList); err != nil {
		t.Fatal(err)
	}
	if len(ownedList) != 1 || ownedList[0].ID != draft.ID {
		t.Fatal("owner list did not include own archive")
	}
	legacy := httpArticle(t, workflowRequest(t, h, "POST", "/articles", owner, `{"title":"Human","content":"<p>HumanV1</p>"}`, 201))
	if legacy.Status != "published" || legacy.Version != 1 || legacy.PublishedVersion != 1 {
		t.Fatalf("legacy create=%+v", legacy)
	}
	legacyPath := fmt.Sprintf("/articles/%d", legacy.ID)
	updated := httpArticle(t, workflowRequest(t, h, "PUT", legacyPath, owner, `{"content":"<p>HumanV2</p>"}`, 200))
	public = httpArticle(t, workflowRequest(t, h, "GET", legacyPath, "", "", 200))
	if updated.Version != 2 || updated.PublishedVersion != 2 || public.Content != updated.Content {
		t.Fatal("legacy update did not immediately publish V2")
	}
}

func TestArticleContentRoutesRejectLifecycleFields(t *testing.T) {
	h, owner, _ := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/articles/drafts", owner, `{"title":"Initial","content":"Initial"}`, 201))
	for _, target := range []struct{ method, path string }{
		{"POST", "/articles"}, {"POST", "/articles/drafts"},
		{"PUT", fmt.Sprintf("/articles/%d", created.ID)}, {"PUT", fmt.Sprintf("/articles/%d/draft", created.ID)},
	} {
		for _, field := range []string{`"status":"published"`, `"status":"archived"`, `"published_version":99`, `"version":99`} {
			body := `{"title":"Injected","content":"Injected",` + field + `}`
			workflowRequest(t, h, target.method, target.path, owner, body, 400)
		}
	}
	var articles []model.Article
	if err := database.DB.Find(&articles).Error; err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 || articles[0].Status != "draft" || articles[0].Version != 1 || articles[0].PublishedVersion != 0 || articles[0].Content != "Initial" {
		t.Fatal("rejected request still changed article state")
	}
	for _, typ := range []reflect.Type{reflect.TypeOf(service.CreateDraftRequest{}), reflect.TypeOf(service.UpdateDraftRequest{}), reflect.TypeOf(service.CreateArticleRequest{}), reflect.TypeOf(service.UpdateArticleRequest{})} {
		for _, field := range []string{"Status", "PublishedVersion", "Version"} {
			if _, exists := typ.FieldByName(field); exists {
				t.Fatalf("%s exposes %s", typ.Name(), field)
			}
		}
	}
}

func TestArticleRoutesRequireAuthenticationAndOwnership(t *testing.T) {
	h, owner, otherAdmin := setupWorkflowHTTP(t)
	created := httpArticle(t, workflowRequest(t, h, "POST", "/articles/drafts", owner, `{"title":"Initial","content":"Initial"}`, 201))
	path := fmt.Sprintf("/articles/%d", created.ID)
	for _, target := range []struct{ method, path, body string }{
		{"GET", fmt.Sprintf("/user/articles/%d", created.ID), ""},
		{"PUT", path + "/draft", `{"content":"forbidden"}`},
		{"PUT", path, `{"content":"forbidden"}`},
		{"POST", path + "/publish", ""}, {"POST", path + "/archive", ""},
	} {
		workflowRequest(t, h, target.method, target.path, "", target.body, 401)
		workflowRequest(t, h, target.method, target.path, otherAdmin, target.body, 403)
	}
	workflowRequest(t, h, "POST", "/articles/drafts", "", `{"title":"x","content":"x"}`, 401)
	workflowRequest(t, h, "GET", "/user/articles", "", "", 401)
	if data := workflowRequest(t, h, "GET", "/user/articles?user_id=1", otherAdmin, "", 200); string(data) != "[]" {
		t.Fatalf("owner scope overridden: %s", data)
	}
	workflowRequest(t, h, "GET", "/user/articles/999999", owner, "", 404)
}
