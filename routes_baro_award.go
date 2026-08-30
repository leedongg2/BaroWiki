package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// baro_award_html : 어워드 화면 응답 (write_data 가 Get_redirect 를 302 로 바꿔 준다)
func baro_award_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// register_baro_award_routes : 12장 어워드 · 연말 결산 라우트
func register_baro_award_routes(r *gin.Engine) {
	// --- 12-2 올해의 바로위키인 ---
	r.GET("/baro/award", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_list(make_route_config(c), "1"))
	})

	r.GET("/baro/award/page/:num", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_list(make_route_config(c), c.Param("num")))
	})

	r.GET("/baro/award/season/:season", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_season(make_route_config(c), c.Param("season")))
	})

	r.GET("/baro/award/view/:id", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_view(make_route_config(c), c.Param("id"), c.Query("page")))
	})

	// --- 12-1 월간 MVP ---
	r.GET("/baro/award/mvp", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_mvp(make_route_config(c), c.Query("month"), nil))
	})

	r.POST("/baro/award/mvp", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_award_html(c, route.View_baro_award_mvp(make_route_config(c), c.Query("month"), c.Request.PostForm))
	})

	// --- 12-3 선정 절차 1단계 : 자동 자격 후보 ---
	r.GET("/baro/award/auto/:id", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_auto(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/award/auto/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_award_html(c, route.View_baro_award_auto(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	// --- 12-3 선정 절차 2단계 : 이용자 추천 ---
	r.GET("/baro/award/nominate/:id", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_nominate(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/award/nominate/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_award_html(c, route.View_baro_award_nominate(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	// --- 12-3 선정 절차 4단계 : 심사위원 평가 ---
	r.GET("/baro/award/judge/:id", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_judge(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/award/judge/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_award_html(c, route.View_baro_award_judge(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	// --- 12-3 선정 절차 5단계 : 커뮤니티 투표 ---
	r.GET("/baro/award/vote/:id", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_vote(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/award/vote/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_award_html(c, route.View_baro_award_vote(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	// --- 결과 확정 (선정 근거 공개 · 영구 보존) ---
	r.GET("/baro/award/close/:id", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_close(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/award/close/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_award_html(c, route.View_baro_award_close(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	// --- 시즌 운영 ---
	r.GET("/baro/award/manage", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_manage(make_route_config(c), nil))
	})

	r.POST("/baro/award/manage", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_award_html(c, route.View_baro_award_manage(make_route_config(c), c.Request.PostForm))
	})

	// --- 12-4 연말 Wrapped ---
	r.GET("/baro/wrapped", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_wrapped(make_route_config(c), c.Query("year")))
	})

	r.GET("/baro/wrapped/site", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_wrapped_site(make_route_config(c), c.Query("year")))
	})

	r.GET("/baro/wrapped/list", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_wrapped_list(make_route_config(c), c.Query("page")))
	})

	r.GET("/baro/wrapped/:year", func(c *gin.Context) {
		baro_award_html(c, route.View_baro_award_wrapped(make_route_config(c), c.Param("year")))
	})
}
