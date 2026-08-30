package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

func register_trash_routes(r *gin.Engine) {
	r.GET("/trash", func(c *gin.Context) {
		route_data := route.View_trash_list(make_route_config(c), "1", "")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/trash/:num", func(c *gin.Context) {
		route_data := route.View_trash_list(make_route_config(c), c.Param("num"), "")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/file_trash", func(c *gin.Context) {
		route_data := route.View_trash_list(make_route_config(c), "1", "file")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/file_trash/:num", func(c *gin.Context) {
		route_data := route.View_trash_list(make_route_config(c), c.Param("num"), "file")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/trash_restore/*doc_name", func(c *gin.Context) {
		route_data := route.View_trash_restore(make_route_config(c), doc_param(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/trash_restore/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		route_data := route.View_trash_restore(make_route_config(c), doc_param(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/trash_delete/*doc_name", func(c *gin.Context) {
		route_data := route.View_trash_delete(make_route_config(c), doc_param(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/trash_delete/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		route_data := route.View_trash_delete(make_route_config(c), doc_param(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})
}
