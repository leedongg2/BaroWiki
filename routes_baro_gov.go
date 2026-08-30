package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// register_baro_gov_routes : 11장 운영 투명성 · 거버넌스 라우트
//
//	/baro/log          공개 운영 로그
//	/baro/policy       정책 문서 버전 관리
//	/baro/role         운영자 역할 분리 · 이해충돌 회피 · 관리자 활동
//	/baro/transparency 정기 투명성 보고서
func register_baro_gov_routes(r *gin.Engine) {
	baro_gov_html := func(c *gin.Context, data string) {
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	}

	// ---------- 11-1 공개 운영 로그 ----------

	r.GET("/baro/log", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_log_list(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/log/legacy", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_log_legacy(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/log/view/:id", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_log_view(make_route_config(c), c.Param("id")))
	})

	// ---------- 11-2 정책 문서 버전 관리 ----------

	r.GET("/baro/policy", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_policy_list(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/policy/new", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_policy_new(make_route_config(c), nil))
	})

	r.POST("/baro/policy/new", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_gov_html(c, route.View_baro_gov_policy_new(make_route_config(c), c.Request.PostForm))
	})

	r.GET("/baro/policy/view/:key", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_policy_view(make_route_config(c), c.Param("key")))
	})

	r.GET("/baro/policy/edit/:key", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_policy_edit(make_route_config(c), c.Param("key"), nil))
	})

	r.POST("/baro/policy/edit/:key", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_gov_html(c, route.View_baro_gov_policy_edit(make_route_config(c), c.Param("key"), c.Request.PostForm))
	})

	r.GET("/baro/policy/diff/:key", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_policy_diff(make_route_config(c), c.Param("key"), "", "", c.Request.URL.Query()))
	})

	r.GET("/baro/policy/diff/:key/:v1/:v2", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_policy_diff(make_route_config(c), c.Param("key"), c.Param("v1"), c.Param("v2"), c.Request.URL.Query()))
	})

	// ---------- 11-3 운영자 역할 분리 ----------

	r.GET("/baro/role", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_role_list(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/role/user/:user", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_role_user(make_route_config(c), c.Param("user"), nil))
	})

	r.POST("/baro/role/user/:user", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_gov_html(c, route.View_baro_gov_role_user(make_route_config(c), c.Param("user"), c.Request.PostForm))
	})

	// ---------- 11-4 이해충돌 회피 ----------

	r.GET("/baro/role/coi", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_coi_list(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/role/coi/new", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_coi_new(make_route_config(c), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/role/coi/new", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_gov_html(c, route.View_baro_gov_coi_new(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm))
	})

	// ---------- 11-5 관리자 활동 기록 ----------

	r.GET("/baro/role/activity", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_activity(make_route_config(c), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/role/activity", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_gov_html(c, route.View_baro_gov_activity(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm))
	})

	// ---------- 11-6 정기 투명성 보고서 ----------

	r.GET("/baro/transparency", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_transparency_list(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/transparency/make", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_transparency_make(make_route_config(c), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/transparency/make", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_gov_html(c, route.View_baro_gov_transparency_make(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm))
	})

	r.GET("/baro/transparency/view/:id", func(c *gin.Context) {
		baro_gov_html(c, route.View_baro_gov_transparency_view(make_route_config(c), c.Param("id")))
	})
}
