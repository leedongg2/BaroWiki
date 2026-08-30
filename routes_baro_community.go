package main

// 바로위키 8장 커뮤니티 · 협업 모듈의 라우트 등록
//
// 이 모듈이 만드는 주소는 /baro/talk, /baro/watch, /baro/mentor 아래에만 있다.

import (
	"net/http"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// baro_community_param : *param 앞의 / 를 떼어 낸다
func baro_community_param(c *gin.Context, name string) string {
	return strings.TrimPrefix(c.Param(name), "/")
}

// baro_community_page : ?page= 값 (없으면 1)
func baro_community_page(c *gin.Context) string {
	page := c.Query("page")
	if page == "" {
		return "1"
	}

	return page
}

// baro_community_html : 화면 결과를 그대로 내보낸다
func baro_community_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

func register_baro_community_routes(r *gin.Engine) {
	// ---------- 8장 토론 허브 ----------
	r.GET("/baro/talk", func(c *gin.Context) {
		baro_community_html(c, route.View_baro_community_talk_home(make_route_config(c), c.Request.URL.Query()))
	})

	r.POST("/baro/talk", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_community_html(c, route.View_baro_community_talk_home(make_route_config(c), c.Request.PostForm))
	})

	// ---------- 8-1 문서 토론 ----------
	r.GET("/baro/talk/doc/*doc_name", func(c *gin.Context) {
		baro_community_html(c, route.View_baro_community_talk_doc(
			make_route_config(c), baro_community_param(c, "doc_name"),
			baro_community_page(c), c.Request.URL.Query(), nil,
		))
	})

	r.POST("/baro/talk/doc/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_community_html(c, route.View_baro_community_talk_doc(
			make_route_config(c), baro_community_param(c, "doc_name"),
			baro_community_page(c), c.Request.URL.Query(), c.Request.PostForm,
		))
	})

	// ---------- 8-1 사용자 언급 처리 ----------
	r.GET("/baro/talk/mention", func(c *gin.Context) {
		baro_community_html(c, route.View_baro_community_talk_mention(
			make_route_config(c), c.Request.URL.Query(), nil,
		))
	})

	r.POST("/baro/talk/mention", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_community_html(c, route.View_baro_community_talk_mention(
			make_route_config(c), c.Request.URL.Query(), c.Request.PostForm,
		))
	})

	// ---------- 8-2 사용자 토론 ----------
	r.GET("/baro/talk/user/*user_name", func(c *gin.Context) {
		baro_community_html(c, route.View_baro_community_talk_user(
			make_route_config(c), baro_community_param(c, "user_name"),
			baro_community_page(c), c.Request.URL.Query(), nil,
		))
	})

	r.POST("/baro/talk/user/*user_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_community_html(c, route.View_baro_community_talk_user(
			make_route_config(c), baro_community_param(c, "user_name"),
			baro_community_page(c), c.Request.URL.Query(), c.Request.PostForm,
		))
	})

	// ---------- 8-3 프로젝트 토론 ----------
	r.GET("/baro/talk/project/*project_id", func(c *gin.Context) {
		baro_community_html(c, route.View_baro_community_talk_project(
			make_route_config(c), baro_community_param(c, "project_id"),
			baro_community_page(c), c.Request.URL.Query(), nil,
		))
	})

	r.POST("/baro/talk/project/*project_id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_community_html(c, route.View_baro_community_talk_project(
			make_route_config(c), baro_community_param(c, "project_id"),
			baro_community_page(c), c.Request.URL.Query(), c.Request.PostForm,
		))
	})

	// ---------- 8-4 주시 목록 ----------
	r.GET("/baro/watch", func(c *gin.Context) {
		baro_community_html(c, route.View_baro_community_watch(
			make_route_config(c), baro_community_page(c), c.Request.URL.Query(), nil,
		))
	})

	r.POST("/baro/watch", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_community_html(c, route.View_baro_community_watch(
			make_route_config(c), baro_community_page(c), c.Request.URL.Query(), c.Request.PostForm,
		))
	})

	r.GET("/baro/watch/add", func(c *gin.Context) {
		baro_community_html(c, route.View_baro_community_watch_add(
			make_route_config(c), c.Request.URL.Query(), nil,
		))
	})

	r.POST("/baro/watch/add", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_community_html(c, route.View_baro_community_watch_add(
			make_route_config(c), c.Request.URL.Query(), c.Request.PostForm,
		))
	})

	r.GET("/baro/watch/feed", func(c *gin.Context) {
		baro_community_html(c, route.View_baro_community_watch_feed(
			make_route_config(c), c.Request.URL.Query(),
		))
	})

	// ---------- 8-5 멘토링 ----------
	r.GET("/baro/mentor", func(c *gin.Context) {
		baro_community_html(c, route.View_baro_community_mentor(
			make_route_config(c), baro_community_page(c), c.Request.URL.Query(), nil,
		))
	})

	r.POST("/baro/mentor", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_community_html(c, route.View_baro_community_mentor(
			make_route_config(c), baro_community_page(c), c.Request.URL.Query(), c.Request.PostForm,
		))
	})

	r.GET("/baro/mentor/mine", func(c *gin.Context) {
		baro_community_html(c, route.View_baro_community_mentor_mine(
			make_route_config(c), c.Request.URL.Query(), nil,
		))
	})

	r.POST("/baro/mentor/mine", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_community_html(c, route.View_baro_community_mentor_mine(
			make_route_config(c), c.Request.URL.Query(), c.Request.PostForm,
		))
	})
}
