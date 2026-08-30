package main

import (
	"net/http"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// baro_contrib_user_of : 경로 변수에서 사용자 이름을 꺼낸다
func baro_contrib_user_of(c *gin.Context) string {
	return strings.TrimPrefix(c.Param("user"), "/")
}

// baro_contrib_html : 바로위키 화면 응답
func baro_contrib_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// register_baro_contrib_routes : 7장 기여 인정 · 게임화 라우트
func register_baro_contrib_routes(r *gin.Engine) {
	// --- 7-1 기여 이벤트 ---
	r.GET("/baro/contrib", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_list(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/contrib/user/*user", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_user(make_route_config(c), baro_contrib_user_of(c), c.Request.URL.Query()))
	})

	r.GET("/baro/contrib/scan", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_scan(make_route_config(c), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/contrib/scan", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_contrib_html(c, route.View_baro_contrib_scan(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm))
	})

	// --- 7-2 기여 가치 ---
	r.GET("/baro/contrib/score", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_score(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/contrib/score/run", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_score_run(make_route_config(c), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/contrib/score/run", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_contrib_html(c, route.View_baro_contrib_score_run(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm))
	})

	// --- 7-3 기여 영향력 ---
	r.GET("/baro/contrib/impact/*user", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_impact(make_route_config(c), baro_contrib_user_of(c)))
	})

	// --- 7-4 전문 분야 ---
	r.GET("/baro/contrib/expertise", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_expertise(make_route_config(c), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/contrib/expertise", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_contrib_html(c, route.View_baro_contrib_expertise(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm))
	})

	// --- 7-8 활동 캘린더 ---
	r.GET("/baro/contrib/calendar/*user", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_calendar(make_route_config(c), baro_contrib_user_of(c)))
	})

	// --- 7-5 분야별 순위 ---
	r.GET("/baro/rank", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_rank(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/rank/run", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_rank_run(make_route_config(c), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/rank/run", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_contrib_html(c, route.View_baro_contrib_rank_run(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm))
	})

	// --- 7-6 배지 ---
	r.GET("/baro/badge", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_badge(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/badge/user/*user", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_badge_user(make_route_config(c), baro_contrib_user_of(c)))
	})

	r.GET("/baro/badge/check", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_badge_check(make_route_config(c), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/badge/check", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_contrib_html(c, route.View_baro_contrib_badge_check(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm))
	})

	// --- 7-7 감사 ---
	r.GET("/baro/thanks", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_thanks(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/thanks/send", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_thanks_send(make_route_config(c), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/thanks/send", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_contrib_html(c, route.View_baro_contrib_thanks_send(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm))
	})

	r.GET("/baro/thanks/cancel", func(c *gin.Context) {
		baro_contrib_html(c, route.View_baro_contrib_thanks_cancel(make_route_config(c), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/thanks/cancel", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_contrib_html(c, route.View_baro_contrib_thanks_cancel(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm))
	})
}
