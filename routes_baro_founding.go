package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// baro_founding_base_url : 요청이 들어온 주소 (https://example.com)
//
//	공유 카드의 Open Graph 주소를 절대 주소로 적기 위해 쓴다.
func baro_founding_base_url(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}

	if forwarded := c.GetHeader("X-Forwarded-Proto"); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}

	host := c.Request.Host
	if host == "" {
		return ""
	}

	return scheme + "://" + host
}

// baro_founding_html : 창립 시즌 모듈의 공통 응답 처리
func baro_founding_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// register_baro_founding_routes : 창립 시즌 모듈 라우트
//
//	1-2 100인 창립단 · 1-3 관심 분야 · 1-4 분야별 모집 ·
//	1-5 추천/초대 · 1-6 공유 카드
func register_baro_founding_routes(r *gin.Engine) {
	// --- 1-2 100인 창립단 ---
	r.GET("/baro/founding/apply", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_apply(make_route_config(c), "1", c.Request.URL.Query(), nil))
	})

	r.POST("/baro/founding/apply", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_founding_html(c, route.View_baro_founding_apply(make_route_config(c), "1", c.Request.URL.Query(), c.Request.PostForm))
	})

	r.GET("/baro/founding/check", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_check(make_route_config(c), "1", c.Request.URL.Query(), nil))
	})

	r.GET("/baro/founding/list", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_list(make_route_config(c), "1", c.Request.URL.Query(), nil))
	})

	r.GET("/baro/founding/list/:num", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_list(make_route_config(c), c.Param("num"), c.Request.URL.Query(), nil))
	})

	r.GET("/baro/founding/hall", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_hall(make_route_config(c), "1", c.Request.URL.Query(), nil))
	})

	r.GET("/baro/founding/hall/:num", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_hall(make_route_config(c), c.Param("num"), c.Request.URL.Query(), nil))
	})

	r.GET("/baro/founding/manage", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_manage(make_route_config(c), "1", c.Request.URL.Query(), nil))
	})

	r.POST("/baro/founding/manage", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_founding_html(c, route.View_baro_founding_manage(make_route_config(c), "1", c.Request.URL.Query(), c.Request.PostForm))
	})

	r.GET("/baro/founding/manage/:num", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_manage(make_route_config(c), c.Param("num"), c.Request.URL.Query(), nil))
	})

	// --- 1-4 분야별 창립 모집 ---
	r.GET("/baro/founding/field", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_field(make_route_config(c), "1", c.Request.URL.Query(), nil))
	})

	r.POST("/baro/founding/field", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_founding_html(c, route.View_baro_founding_field(make_route_config(c), "1", c.Request.URL.Query(), c.Request.PostForm))
	})

	// --- 1-3 관심 분야 ---
	r.GET("/baro/interest", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_interest(make_route_config(c), "1", c.Request.URL.Query(), nil))
	})

	r.POST("/baro/interest", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_founding_html(c, route.View_baro_founding_interest(make_route_config(c), "1", c.Request.URL.Query(), c.Request.PostForm))
	})

	// --- 1-5 추천 · 초대 ---
	r.GET("/baro/invite", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_invite(make_route_config(c), c.Query("page"), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/invite", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_founding_html(c, route.View_baro_founding_invite(make_route_config(c), c.Query("page"), c.Request.URL.Query(), c.Request.PostForm))
	})

	r.GET("/baro/invite/use/:code", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_invite_use(make_route_config(c), c.Param("code"), c.Request.URL.Query(), nil))
	})

	// --- 1-6 공유 카드 ---
	r.GET("/baro/card", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_card_list(make_route_config(c), c.Query("page"), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/card", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_founding_html(c, route.View_baro_founding_card_list(make_route_config(c), c.Query("page"), c.Request.URL.Query(), c.Request.PostForm))
	})

	r.GET("/baro/card/make", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_card_make(make_route_config(c), "1", c.Request.URL.Query(), nil))
	})

	r.POST("/baro/card/make", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_founding_html(c, route.View_baro_founding_card_make(make_route_config(c), "1", c.Request.URL.Query(), c.Request.PostForm))
	})

	r.GET("/baro/card/:id", func(c *gin.Context) {
		baro_founding_html(c, route.View_baro_founding_card_view(make_route_config(c), c.Param("id"), baro_founding_base_url(c), c.Request.URL.Query(), nil))
	})
}
