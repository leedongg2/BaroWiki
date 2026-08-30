package main

import (
	"net/http"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

func namespace_param(c *gin.Context) string {
	return strings.TrimPrefix(c.Param("namespace_name"), "/")
}

func register_namespace_routes(r *gin.Engine) {
	r.GET("/namespace", func(c *gin.Context) {
		route_data := route.View_namespace_list(make_route_config(c))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/namespace/:namespace_name", func(c *gin.Context) {
		route_data := route.View_namespace_document_list(make_route_config(c), namespace_param(c), "1")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/namespace/:namespace_name/:num", func(c *gin.Context) {
		route_data := route.View_namespace_document_list(make_route_config(c), namespace_param(c), c.Param("num"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/namespace_acl/:namespace_name", func(c *gin.Context) {
		route_data := route.View_namespace_acl(make_route_config(c), namespace_param(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/namespace_acl/:namespace_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		route_data := route.View_namespace_acl(make_route_config(c), namespace_param(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})
}
