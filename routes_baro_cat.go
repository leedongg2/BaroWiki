package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// register_baro_cat_routes : 바로위키 [cat] 모듈 라우트
//
//	3-11 분류 도구 (/baro/category) · 3-14 역링크·연결 문서 도구 (/baro/link)
func register_baro_cat_routes(r *gin.Engine) {
	// ---- 3-11 분류 도구 ----

	r.GET("/baro/category", func(c *gin.Context) {
		data := route.View_baro_cat_list(make_route_config(c), c.Query("page"), c.Query("q"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/category/tree", func(c *gin.Context) {
		data := route.View_baro_cat_tree(make_route_config(c), c.Query("depth"), c.Query("root"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/category/stats", func(c *gin.Context) {
		data := route.View_baro_cat_stats(make_route_config(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/category/stats", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_cat_stats(make_route_config(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/category/check", func(c *gin.Context) {
		data := route.View_baro_cat_check(make_route_config(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/category/check", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_cat_check(make_route_config(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/category/nocategory", func(c *gin.Context) {
		data := route.View_baro_cat_nocategory(make_route_config(c), c.Query("page"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/category/view/*doc_name", func(c *gin.Context) {
		data := route.View_baro_cat_view(make_route_config(c), baro_cat_route_name(c), c.Query("page"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	// ---- 3-14 역링크 · 연결 문서 도구 ----

	r.GET("/baro/link", func(c *gin.Context) {
		data := route.View_baro_cat_link_home(make_route_config(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/link", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_cat_link_home(make_route_config(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/link/orphan", func(c *gin.Context) {
		data := route.View_baro_cat_link_orphan(make_route_config(c), c.Query("page"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/link/broken", func(c *gin.Context) {
		data := route.View_baro_cat_link_broken(make_route_config(c), c.Query("page"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/link/deadend", func(c *gin.Context) {
		data := route.View_baro_cat_link_deadend(make_route_config(c), c.Query("page"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/link/doc/*doc_name", func(c *gin.Context) {
		data := route.View_baro_cat_link_doc(
			make_route_config(c), baro_cat_route_name(c), c.Query("in_page"), c.Query("out_page"), nil,
		)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
}

// baro_cat_route_name : 주소 뒤에 붙은 문서 이름 (없으면 name 질의값)
func baro_cat_route_name(c *gin.Context) string {
	name := doc_param(c)
	if name == "" {
		name = c.Query("name")
	}

	return name
}
