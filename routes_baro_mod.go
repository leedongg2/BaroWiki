package main

// ============================================================
//  바로위키 [mod] 모듈 라우트 등록
//
//  10장 : 신고 · 운영 사건 · 제재 · 문서 보존 · 이의제기 · 대량 조치 안전장치
//  URL 접두사 : /baro/report · /baro/case · /baro/appeal · /baro/mass · /baro/quarantine
// ============================================================

import (
	"net/http"
	"net/url"

	"opennamu/route"
	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

// baro_mod_view_func : 이 모듈 화면 함수의 공통 모양
type baro_mod_view_func func(config tool.Config, arg string, query url.Values, values url.Values) string

// baro_mod_html : HTML 화면을 돌려준다
func baro_mod_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// baro_mod_text : 텍스트 파일을 돌려준다 (대상 목록 내려받기)
func baro_mod_text(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/plain; charset=utf-8", []byte(data))
}

// baro_mod_query : GET 질의 문자열
func baro_mod_query(c *gin.Context) url.Values {
	return c.Request.URL.Query()
}

// baro_mod_post : POST 폼 값
func baro_mod_post(c *gin.Context) url.Values {
	_ = c.Request.ParseForm()

	return c.Request.PostForm
}

// baro_mod_bind : GET / POST 를 한 쌍으로 묶어 등록한다
func baro_mod_bind(r *gin.Engine, path string, arg_name string, view baro_mod_view_func) {
	r.GET(path, func(c *gin.Context) {
		baro_mod_html(c, view(make_route_config(c), baro_mod_arg(c, arg_name), baro_mod_query(c), nil))
	})

	r.POST(path, func(c *gin.Context) {
		baro_mod_html(c, view(make_route_config(c), baro_mod_arg(c, arg_name), baro_mod_query(c), baro_mod_post(c)))
	})
}

// baro_mod_bind_get : GET 만 등록한다
func baro_mod_bind_get(r *gin.Engine, path string, arg_name string, view baro_mod_view_func) {
	r.GET(path, func(c *gin.Context) {
		baro_mod_html(c, view(make_route_config(c), baro_mod_arg(c, arg_name), baro_mod_query(c), nil))
	})
}

// baro_mod_arg : 경로 변수 (없으면 기본값)
func baro_mod_arg(c *gin.Context, arg_name string) string {
	if arg_name == "" {
		return "1"
	}

	value := c.Param(arg_name)
	if value == "" {
		if arg_name == "page" {
			return "1"
		}

		return ""
	}

	return value
}

// register_baro_mod_routes : [mod] 모듈 라우트 등록
func register_baro_mod_routes(r *gin.Engine) {
	// ---------- 10-1 신고 ----------
	baro_mod_bind_get(r, "/baro/report", "", route.View_baro_mod_report_home)
	baro_mod_bind(r, "/baro/report/new", "", route.View_baro_mod_report_new)
	baro_mod_bind_get(r, "/baro/report/mine", "", route.View_baro_mod_report_mine)
	baro_mod_bind_get(r, "/baro/report/mine/:page", "page", route.View_baro_mod_report_mine)
	baro_mod_bind(r, "/baro/report/view/:id", "id", route.View_baro_mod_report_view)
	baro_mod_bind(r, "/baro/report/queue", "", route.View_baro_mod_report_queue)
	baro_mod_bind_get(r, "/baro/report/queue/:page", "page", route.View_baro_mod_report_queue)
	baro_mod_bind(r, "/baro/report/merge/:id", "id", route.View_baro_mod_report_merge)

	// ---------- 10-2 운영 사건 ----------
	baro_mod_bind_get(r, "/baro/case", "", route.View_baro_mod_case_list)
	baro_mod_bind_get(r, "/baro/case/list/:page", "page", route.View_baro_mod_case_list)
	baro_mod_bind(r, "/baro/case/new", "", route.View_baro_mod_case_new)
	baro_mod_bind(r, "/baro/case/view/:id", "id", route.View_baro_mod_case_view)
	baro_mod_bind(r, "/baro/case/act/:id", "id", route.View_baro_mod_case_act)

	// ---------- 10-3 제재 ----------
	baro_mod_bind(r, "/baro/case/sanction", "", route.View_baro_mod_sanction_new)
	baro_mod_bind_get(r, "/baro/case/sanction/list", "", route.View_baro_mod_sanction_list)
	baro_mod_bind_get(r, "/baro/case/sanction/list/:page", "page", route.View_baro_mod_sanction_list)
	baro_mod_bind(r, "/baro/case/sanction/release/:id", "id", route.View_baro_mod_sanction_release)

	// ---------- 10-4 문서 보존 (격리) ----------
	baro_mod_bind_get(r, "/baro/quarantine", "", route.View_baro_mod_quarantine_list)
	baro_mod_bind_get(r, "/baro/quarantine/list/:page", "page", route.View_baro_mod_quarantine_list)
	baro_mod_bind(r, "/baro/quarantine/new", "", route.View_baro_mod_quarantine_new)
	baro_mod_bind(r, "/baro/quarantine/view/:id", "id", route.View_baro_mod_quarantine_view)
	baro_mod_bind(r, "/baro/quarantine/delete/:id", "id", route.View_baro_mod_quarantine_delete)
	baro_mod_bind_get(r, "/baro/quarantine/user", "user", route.View_baro_mod_quarantine_user)
	baro_mod_bind_get(r, "/baro/quarantine/user/:user", "user", route.View_baro_mod_quarantine_user)

	// ---------- 10-5 이의제기 ----------
	baro_mod_bind_get(r, "/baro/appeal", "", route.View_baro_mod_appeal_list)
	baro_mod_bind_get(r, "/baro/appeal/list/:page", "page", route.View_baro_mod_appeal_list)
	baro_mod_bind(r, "/baro/appeal/new", "case", route.View_baro_mod_appeal_new)
	baro_mod_bind(r, "/baro/appeal/new/:case", "case", route.View_baro_mod_appeal_new)
	baro_mod_bind_get(r, "/baro/appeal/mine", "", route.View_baro_mod_appeal_mine)
	baro_mod_bind_get(r, "/baro/appeal/mine/:page", "page", route.View_baro_mod_appeal_mine)
	baro_mod_bind(r, "/baro/appeal/view/:id", "id", route.View_baro_mod_appeal_view)

	// ---------- 10-6 대량 조치 안전장치 ----------
	baro_mod_bind_get(r, "/baro/mass", "", route.View_baro_mod_mass_list)
	baro_mod_bind_get(r, "/baro/mass/list/:page", "page", route.View_baro_mod_mass_list)
	baro_mod_bind(r, "/baro/mass/new", "", route.View_baro_mod_mass_new)
	baro_mod_bind(r, "/baro/mass/view/:id", "id", route.View_baro_mod_mass_view)
	baro_mod_bind(r, "/baro/mass/approve/:id", "id", route.View_baro_mod_mass_approve)
	baro_mod_bind(r, "/baro/mass/execute/:id", "id", route.View_baro_mod_mass_execute)
	baro_mod_bind(r, "/baro/mass/undo/:id", "id", route.View_baro_mod_mass_undo)

	r.GET("/baro/mass/download/:id", func(c *gin.Context) {
		baro_mod_text(c, route.View_baro_mod_mass_download(make_route_config(c), c.Param("id")))
	})
}
