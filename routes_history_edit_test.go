package main

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"opennamu/route"
	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

func TestHistoryDiffUuidQueryResolution(t *testing.T) {
	t.Setenv("NAMU_DB", filepath.Join(t.TempDir(), "history-uuid"))
	t.Setenv("NAMU_DB_TYPE", "sqlite")
	tool.DB_shutdown()
	tool.DB_boot()
	t.Cleanup(func() { tool.DB_shutdown() })

	db := tool.DB_connect()
	if err := tool.DB_make(db, tool.Get_DB_set()); err != nil {
		t.Fatal(err)
	}
	tool.Exec_DB(db, "insert into alist (name, acl) values ('ip', 'ip')")
	tool.Exec_DB(db, "insert into alist (name, acl) values ('user', 'user')")
	tool.Exec_DB(db, "insert into alist (name, acl) values ('owner', 'owner')")

	title := "UUIDdiffquery"
	config := tool.Config{IP: "편집자"}
	route.Api_edit_post(config, title, "첫 판입니다.", "첫 판", "yes", "")
	route.Api_edit_post(config, title, "둘째 판입니다.", "둘째 판", "yes", "")
	first_uuid := tool.Baro_rev_uuid(db, title, "1")
	second_uuid := tool.Baro_rev_uuid(db, title, "2")

	jump_context, _ := gin.CreateTestContext(httptest.NewRecorder())
	jump_context.Request = httptest.NewRequest("GET", "/history/"+title+"?rev=r2", nil)
	if got, want := history_revision_jump(jump_context, title), "/history/"+title+"?commit="+second_uuid; got != want {
		t.Fatalf("판 입력은 해당 UUID commit 주소로 이동해야 한다: got %q want %q", got, want)
	}

	missing_context, _ := gin.CreateTestContext(httptest.NewRecorder())
	missing_context.Request = httptest.NewRequest("GET", "/history/"+title+"?rev=99", nil)
	if got := history_revision_jump(missing_context, title); got != "" {
		t.Fatalf("없는 판으로는 리다이렉트하면 안 된다: %q", got)
	}

	make_context := func(query string) *gin.Context {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest("GET", "/diff/"+title+query, nil)
		context.Params = gin.Params{{Key: "path", Value: "/" + title}}
		return context
	}

	doc, before, after := history_diff_revisions(make_context("?uuid="+second_uuid), "/"+title)
	if doc != title || before != "1" || after != "2" {
		t.Fatalf("UUID 하나는 바로 이전 판과 비교해야 한다 : %q, %q, %q", doc, before, after)
	}

	doc, before, after = history_diff_revisions(make_context("?uuid="+second_uuid+"&olduuid="+first_uuid), "/"+title)
	if doc != title || before != "1" || after != "2" {
		t.Fatalf("UUID 두 개를 각각 지정 판으로 풀어야 한다 : %q, %q, %q", doc, before, after)
	}

	doc, before, after = history_diff_revisions(make_context(""), "/1/2/"+title)
	if doc != title || before != "1" || after != "2" {
		t.Fatalf("기존 판 번호 diff 주소도 계속 동작해야 한다 : %q, %q, %q", doc, before, after)
	}
	doc, revision := history_doc_revision(make_context(""), "/2/"+title)
	if doc != title || revision != "2" {
		t.Fatalf("기존 판 번호 단일 주소도 계속 동작해야 한다 : %q, %q", doc, revision)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	register_history_edit_routes(router)
	routes := map[string]bool{}
	for _, one := range router.Routes() {
		if one.Method == "GET" {
			routes[one.Path] = true
		}
	}
	for _, path := range []string{
		"/render/*path",
		"/blame/*path",
		"/revert/*path",
		"/diff/*path",
	} {
		if !routes[path] {
			t.Errorf("UUID 쿼리 문서 경로 라우트가 등록되지 않았다: %s", path)
		}
	}
}