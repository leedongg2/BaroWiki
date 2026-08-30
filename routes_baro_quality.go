package main

import (
	"net/http"
	"net/url"

	"opennamu/route"
	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

// ============================================================
//  바로위키 9장 품질 관리 라우트
//
//  /baro/quality · /baro/review · /baro/source 아래에만 만든다.
// ============================================================

// baro_quality_html : 화면 문자열을 그대로 내보낸다
func baro_quality_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// baro_quality_post_form : POST 폼 값
func baro_quality_post_form(c *gin.Context) url.Values {
	_ = c.Request.ParseForm()

	return c.Request.PostForm
}

func register_baro_quality_routes(r *gin.Engine) {
	// --- 9-1 문서 상태 ---
	r.GET("/baro/quality", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_home(
			make_route_config(c), c.DefaultQuery("page", "1"), c.Query("status")))
	})

	r.GET("/baro/quality/set", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_status_set(
			make_route_config(c), c.Query("title"), nil))
	})

	r.POST("/baro/quality/set", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_status_set(
			make_route_config(c), c.Query("title"), baro_quality_post_form(c)))
	})

	// 문서 이름을 물어보는 폼에서 넘어오는 자리
	r.GET("/baro/quality/doc_go", func(c *gin.Context) {
		title := tool.Namespace_normalize(c.Query("title"))
		if title == "" {
			baro_quality_html(c, route.View_baro_quality_doc(make_route_config(c), ""))

			return
		}

		baro_quality_html(c, tool.Get_redirect("/baro/quality/doc/"+tool.Url_parser(title)))
	})

	r.GET("/baro/quality/doc/*doc_name", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_doc(make_route_config(c), doc_param(c)))
	})

	// --- 9-3 안정판 ---
	r.GET("/baro/quality/stable", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_stable_list(
			make_route_config(c), c.DefaultQuery("page", "1")))
	})

	r.GET("/baro/quality/stable/set", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_stable_set(
			make_route_config(c), c.Query("title"), nil))
	})

	r.POST("/baro/quality/stable/set", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_stable_set(
			make_route_config(c), c.Query("title"), baro_quality_post_form(c)))
	})

	// --- 9-5 중복 감지 ---
	r.GET("/baro/quality/dup", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_dup_list(
			make_route_config(c), c.DefaultQuery("page", "1"), c.Query("status"), c.Query("kind")))
	})

	r.GET("/baro/quality/dup/scan", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_dup_scan(make_route_config(c), nil))
	})

	r.POST("/baro/quality/dup/scan", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_dup_scan(make_route_config(c), baro_quality_post_form(c)))
	})

	r.GET("/baro/quality/dup/view/:id", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_dup_view(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/quality/dup/view/:id", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_dup_view(
			make_route_config(c), c.Param("id"), baro_quality_post_form(c)))
	})

	r.GET("/baro/quality/dup/paste", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_dup_paste(
			make_route_config(c), c.DefaultQuery("page", "1"), nil))
	})

	r.POST("/baro/quality/dup/paste", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_dup_paste(
			make_route_config(c), c.DefaultQuery("page", "1"), baro_quality_post_form(c)))
	})

	// --- 9-6 편집 필터 ---
	r.GET("/baro/quality/filter", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_filter_list(make_route_config(c)))
	})

	r.GET("/baro/quality/filter/add", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_filter_add(make_route_config(c), nil))
	})

	r.POST("/baro/quality/filter/add", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_filter_add(make_route_config(c), baro_quality_post_form(c)))
	})

	r.GET("/baro/quality/filter/edit/:id", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_filter_edit(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/quality/filter/edit/:id", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_filter_edit(
			make_route_config(c), c.Param("id"), baro_quality_post_form(c)))
	})

	r.POST("/baro/quality/filter/toggle/:id", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_filter_toggle(
			make_route_config(c), c.Param("id"), baro_quality_post_form(c)))
	})

	r.GET("/baro/quality/filter/delete/:id", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_filter_delete(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/quality/filter/delete/:id", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_filter_delete(
			make_route_config(c), c.Param("id"), baro_quality_post_form(c)))
	})

	r.GET("/baro/quality/filter/test", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_filter_test(make_route_config(c), c.Query("id"), nil))
	})

	r.POST("/baro/quality/filter/test", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_filter_test(
			make_route_config(c), c.Query("id"), baro_quality_post_form(c)))
	})

	r.GET("/baro/quality/filter/log", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_filter_log(
			make_route_config(c), c.DefaultQuery("page", "1"), nil))
	})

	r.POST("/baro/quality/filter/log", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_filter_log(
			make_route_config(c), c.DefaultQuery("page", "1"), baro_quality_post_form(c)))
	})

	// --- 9-2 편집 검토 큐 ---
	r.GET("/baro/review", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_review_list(
			make_route_config(c), c.DefaultQuery("page", "1"), c.Query("status")))
	})

	r.GET("/baro/review/scan", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_review_scan(make_route_config(c), nil))
	})

	r.POST("/baro/review/scan", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_review_scan(make_route_config(c), baro_quality_post_form(c)))
	})

	r.GET("/baro/review/view/:id", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_review_view(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/review/view/:id", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_review_view(
			make_route_config(c), c.Param("id"), baro_quality_post_form(c)))
	})

	r.GET("/baro/review/mine", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_review_mine(
			make_route_config(c), c.DefaultQuery("page", "1")))
	})

	// --- 9-4 출처 품질 ---
	r.GET("/baro/source", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_source_list(
			make_route_config(c), c.DefaultQuery("page", "1"),
			c.Query("title"), c.Query("kind"), c.Query("status"), c.Query("quality")))
	})

	r.GET("/baro/source/scan", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_source_scan(make_route_config(c), c.Query("title"), nil))
	})

	r.POST("/baro/source/scan", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_source_scan(
			make_route_config(c), c.Query("title"), baro_quality_post_form(c)))
	})

	r.GET("/baro/source/nosource", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_source_nosource(
			make_route_config(c), c.DefaultQuery("page", "1")))
	})

	r.GET("/baro/source/check", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_source_check(make_route_config(c), nil))
	})

	r.POST("/baro/source/check", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_source_check(make_route_config(c), baro_quality_post_form(c)))
	})

	r.GET("/baro/source/overused", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_source_overused(
			make_route_config(c), c.DefaultQuery("page", "1"), c.Query("limit")))
	})

	r.GET("/baro/source/lowtrust", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_source_lowtrust(
			make_route_config(c), c.DefaultQuery("page", "1"), nil))
	})

	r.POST("/baro/source/lowtrust", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_source_lowtrust(
			make_route_config(c), c.DefaultQuery("page", "1"), baro_quality_post_form(c)))
	})

	r.GET("/baro/source/format", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_source_format(
			make_route_config(c), c.DefaultQuery("page", "1")))
	})

	r.GET("/baro/source/edit/:id", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_source_edit(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/source/edit/:id", func(c *gin.Context) {
		baro_quality_html(c, route.View_baro_quality_source_edit(
			make_route_config(c), c.Param("id"), baro_quality_post_form(c)))
	})
}
