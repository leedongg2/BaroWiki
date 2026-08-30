package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// register_baro_find_routes : 바로위키 [find] 모듈 라우트
//
//	5장 검색·탐색·SEO / 3-15 동명이인·별칭 / 4장 파일
//	URL 접두사 : /baro/search · /baro/discover · /baro/doc · /baro/file
func register_baro_find_routes(r *gin.Engine) {
	// ---------- 5-1 · 5-2 검색 ----------
	r.GET("/baro/search", func(c *gin.Context) {
		data := route.View_baro_find_search(make_route_config(c), "1", c.Request.URL.Query())
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/search/suggest", func(c *gin.Context) {
		data := route.View_baro_find_suggest_json(make_route_config(c), c.Query("q"))
		write_data(c, http.StatusOK, "application/json; charset=utf-8", []byte(data))
	})

	// ---------- 5-3 · 5-4 발견 ----------
	r.GET("/baro/discover", func(c *gin.Context) {
		data := route.View_baro_find_discover(make_route_config(c), "1", c.Request.URL.Query())
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/discover/random", func(c *gin.Context) {
		data := route.View_baro_find_random(make_route_config(c))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/discover/recent", func(c *gin.Context) {
		data := route.View_baro_find_recent(make_route_config(c), "1", c.Request.URL.Query())
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	// ---------- 3-15 별칭 · 동명이인 ----------
	r.GET("/baro/doc/alias/*doc_name", func(c *gin.Context) {
		data := route.View_baro_find_alias_doc(make_route_config(c), doc_param(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/doc/alias/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_find_alias_doc(make_route_config(c), doc_param(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/doc/alias", func(c *gin.Context) {
		data := route.View_baro_find_alias(make_route_config(c), "1", c.Request.URL.Query(), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/doc/alias", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_find_alias(make_route_config(c), "1", c.Request.URL.Query(), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/doc/dab", func(c *gin.Context) {
		data := route.View_baro_find_dab(make_route_config(c), c.Request.URL.Query())
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/doc/meta", func(c *gin.Context) {
		data := route.View_baro_find_meta(make_route_config(c), "1", c.Request.URL.Query())
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	// ---------- 5-5 외부 검색 최적화 ----------
	r.GET("/baro/doc/seo/*doc_name", func(c *gin.Context) {
		data := route.View_baro_find_seo_doc(make_route_config(c), doc_param(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/doc/seo/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_find_seo_doc(make_route_config(c), doc_param(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/doc/seo", func(c *gin.Context) {
		data := route.View_baro_find_seo(make_route_config(c), "1", c.Request.URL.Query(), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/doc/seo", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_find_seo(make_route_config(c), "1", c.Request.URL.Query(), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/doc/robots", func(c *gin.Context) {
		data := route.View_baro_find_robots(make_route_config(c))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/doc/sitemap", func(c *gin.Context) {
		data := route.View_baro_find_sitemap_xml(make_route_config(c))
		write_data(c, http.StatusOK, "application/xml; charset=utf-8", []byte(data))
	})

	// ---------- 5-6 지식 그래프 ----------
	r.GET("/baro/doc/graph/*doc_name", func(c *gin.Context) {
		data := route.View_baro_find_graph_doc(make_route_config(c), doc_param(c), c.Request.URL.Query(), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/doc/graph/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_find_graph_doc(make_route_config(c), doc_param(c), c.Request.URL.Query(), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/doc/graph", func(c *gin.Context) {
		data := route.View_baro_find_graph(make_route_config(c), "1", c.Request.URL.Query())
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	// ---------- 4장 파일 ----------
	r.GET("/baro/file/meta/*doc_name", func(c *gin.Context) {
		data := route.View_baro_find_file_meta(make_route_config(c), doc_param(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/file/meta/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_find_file_meta(make_route_config(c), doc_param(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/file/nosource", func(c *gin.Context) {
		data := route.View_baro_find_file_nosource(make_route_config(c), "1", c.Request.URL.Query())
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/file/dup", func(c *gin.Context) {
		data := route.View_baro_find_file_dup(make_route_config(c), c.Request.URL.Query(), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/file/dup", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_find_file_dup(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/file/privacy", func(c *gin.Context) {
		data := route.View_baro_find_file_privacy(make_route_config(c), "1", c.Request.URL.Query(), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/file/privacy", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_find_file_privacy(make_route_config(c), "1", c.Request.URL.Query(), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/file", func(c *gin.Context) {
		data := route.View_baro_find_file(make_route_config(c), "1", c.Request.URL.Query())
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
}
