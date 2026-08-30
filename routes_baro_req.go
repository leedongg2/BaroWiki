package main

import (
	"net/http"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// baro_req_html : 편집 요청 화면 응답
func baro_req_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// register_baro_req_routes : 편집 요청 라우트
func register_baro_req_routes(r *gin.Engine) {
	r.GET("/baro/req", func(c *gin.Context) {
		baro_req_html(c, route.View_baro_req_list(make_route_config(c), "", "1"))
	})

	r.GET("/baro/req/page/:num", func(c *gin.Context) {
		baro_req_html(c, route.View_baro_req_list(make_route_config(c), "", c.Param("num")))
	})

	r.GET("/baro/req/list/:status", func(c *gin.Context) {
		baro_req_html(c, route.View_baro_req_list(make_route_config(c), c.Param("status"), "1"))
	})

	r.GET("/baro/req/list/:status/:num", func(c *gin.Context) {
		baro_req_html(c, route.View_baro_req_list(make_route_config(c), c.Param("status"), c.Param("num")))
	})

	r.GET("/baro/req/new", func(c *gin.Context) {
		baro_req_html(c, route.View_baro_req_new(make_route_config(c), c.Query("doc"), nil))
	})

	r.POST("/baro/req/new", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_req_html(c, route.View_baro_req_new(make_route_config(c), c.Query("doc"), c.Request.PostForm))
	})

	r.GET("/baro/req/view/:id", func(c *gin.Context) {
		baro_req_html(c, route.View_baro_req_view(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/req/view/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_req_html(c, route.View_baro_req_view(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	r.GET("/baro/req/log", func(c *gin.Context) {
		baro_req_html(c, route.View_baro_req_log(make_route_config(c), "1"))
	})

	r.GET("/baro/req/log/:num", func(c *gin.Context) {
		baro_req_html(c, route.View_baro_req_log(make_route_config(c), c.Param("num")))
	})

	r.GET("/baro/req/doc/*doc_name", func(c *gin.Context) {
		doc_name := strings.TrimPrefix(c.Param("doc_name"), "/")

		// 마지막 칸이 쪽 번호이면 떼어 낸다
		page := "1"
		if index := strings.LastIndex(doc_name, "/"); index > 0 {
			tail := doc_name[index+1:]
			if tail != "" && strings.Trim(tail, "0123456789") == "" {
				page = tail
				doc_name = doc_name[:index]
			}
		}

		baro_req_html(c, route.View_baro_req_doc(make_route_config(c), doc_name, page))
	})
}
