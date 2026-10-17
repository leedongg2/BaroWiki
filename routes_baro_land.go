package main

import (
	"net/http"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// register_baro_land_routes : 기여 땅따먹기 라우트
func register_baro_land_routes(r *gin.Engine) {
	html := func(c *gin.Context, data string) {
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	}

	r.GET("/baro/land", func(c *gin.Context) {
		html(c, route.View_baro_land(make_route_config(c)))
	})

	r.GET("/baro/land/user/*user_name", func(c *gin.Context) {
		html(c, route.View_baro_land_user(make_route_config(c), strings.TrimPrefix(c.Param("user_name"), "/")))
	})

	r.GET("/baro/land/doc/*doc_name", func(c *gin.Context) {
		html(c, route.View_baro_land_doc(make_route_config(c), strings.TrimPrefix(c.Param("doc_name"), "/")))
	})
}
