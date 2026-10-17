package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

func register_wiki_routes(r *gin.Engine) {
	r.GET("/xref/*doc_name", func(c *gin.Context) {
		route_data := route.View_w_xref(make_route_config(c), doc_param(c), "1", "1")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/xref_page/:num/*doc_name", func(c *gin.Context) {
		route_data := route.View_w_xref(make_route_config(c), doc_param(c), "1", c.Param("num"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/xref_this/*doc_name", func(c *gin.Context) {
		route_data := route.View_w_xref(make_route_config(c), doc_param(c), "2", "1")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/xref_this_page/:num/*doc_name", func(c *gin.Context) {
		route_data := route.View_w_xref(make_route_config(c), doc_param(c), "2", c.Param("num"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/xref_reset/*doc_name", func(c *gin.Context) {
		route_data := route.View_w_xref_reset(make_route_config(c), doc_param(c))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/xref_reset/*doc_name", func(c *gin.Context) {
		route_data := route.View_w_xref_reset_post(make_route_config(c), doc_param(c))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/upload", func(c *gin.Context) {
		route_data := route.View_edit_file_upload(make_route_config(c), c.Query("name"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/w/*doc_name", func(c *gin.Context) {
		doc_name := doc_param(c)

		// `/w/문서?uuid=...` 는 그 판을 보여 주는 주소로 넘긴다.
		// 문서 보기는 언제나 최신 판이라 판을 함께 그릴 자리가 없다.
		if rev := rev_param(c, doc_name, ""); rev != "" {
			route_data := route.View_render(make_route_config(c), doc_name, rev)
			write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))

			return
		}

		route_data, status_code := route.View_w(c, make_route_config(c), doc_name, "")
		write_data(c, status_code, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/down/*doc_name", func(c *gin.Context) {
		route_data := route.View_w_down(make_route_config(c), doc_param(c))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/raw/*doc_name", func(c *gin.Context) {
		doc_name := doc_param(c)
		route_data := route.View_w_raw(make_route_config(c), doc_name, rev_param(c, doc_name, ""), "")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/raw_rev/:rev/*doc_name", func(c *gin.Context) {
		doc_name := doc_param(c)
		route_data := route.View_w_raw(make_route_config(c), doc_name, rev_param(c, doc_name, c.Param("rev")), "")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/raw_acl/*doc_name", func(c *gin.Context) {
		route_data := route.View_w_raw(make_route_config(c), doc_param(c), "", "document_acl")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/topic/*doc_name", func(c *gin.Context) {
		route_data := route.View_topic_list(make_route_config(c), doc_param(c), c.Query("tool"), "1")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})
	r.POST("/topic/*doc_name", func(c *gin.Context) {
		route_data := route.View_topic_list(make_route_config(c), doc_param(c), c.Query("tool"), "1")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/topic_page/:num/*doc_name", func(c *gin.Context) {
		route_data := route.View_topic_list(make_route_config(c), doc_param(c), "", c.Param("num"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})
	r.POST("/topic_page/:num/*doc_name", func(c *gin.Context) {
		route_data := route.View_topic_list(make_route_config(c), doc_param(c), "", c.Param("num"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/topic_close/:num/*doc_name", func(c *gin.Context) {
		route_data := route.View_topic_list(make_route_config(c), doc_param(c), "close", c.Param("num"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/topic_agree/:num/*doc_name", func(c *gin.Context) {
		route_data := route.View_topic_list(make_route_config(c), doc_param(c), "agree", c.Param("num"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})
}
