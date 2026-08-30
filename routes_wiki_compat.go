package main

import (
	"opennamu/route"

	"github.com/gin-gonic/gin"
)

func register_wiki_compat_routes(r *gin.Engine) {
	r.GET("/w_from/*doc_name", func(c *gin.Context) {
		data, status := route.View_w(c, make_route_config(c), doc_param(c), "from")
		write_data(c, status, "text/html; charset=utf-8", []byte(data))
	})
}
