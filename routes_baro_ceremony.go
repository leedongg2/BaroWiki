package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// baro_ceremony_html : 위키 수상식 화면 응답
func baro_ceremony_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// register_baro_ceremony_routes : 위키 수상식 라우트
func register_baro_ceremony_routes(r *gin.Engine) {
	r.GET("/baro/ceremony", func(c *gin.Context) {
		baro_ceremony_html(c, route.View_baro_ceremony_list(make_route_config(c), "1"))
	})

	r.GET("/baro/ceremony/page/:num", func(c *gin.Context) {
		baro_ceremony_html(c, route.View_baro_ceremony_list(make_route_config(c), c.Param("num")))
	})

	r.GET("/baro/ceremony/open", func(c *gin.Context) {
		baro_ceremony_html(c, route.View_baro_ceremony_open(make_route_config(c), nil))
	})

	r.POST("/baro/ceremony/open", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_ceremony_html(c, route.View_baro_ceremony_open(make_route_config(c), c.Request.PostForm))
	})

	r.GET("/baro/ceremony/view/:id", func(c *gin.Context) {
		baro_ceremony_html(c, route.View_baro_ceremony_view(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/ceremony/view/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_ceremony_html(c, route.View_baro_ceremony_view(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	r.GET("/baro/ceremony/run/:id", func(c *gin.Context) {
		baro_ceremony_html(c, route.View_baro_ceremony_run(make_route_config(c), c.Param("id"), c.Query("cat"), nil))
	})

	r.POST("/baro/ceremony/run/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_ceremony_html(c, route.View_baro_ceremony_run(make_route_config(c), c.Param("id"), c.Query("cat"), c.Request.PostForm))
	})

	r.GET("/baro/ceremony/step/:id", func(c *gin.Context) {
		baro_ceremony_html(c, route.View_baro_ceremony_step(make_route_config(c), c.Param("id")))
	})

	r.GET("/baro/ceremony/close/:id", func(c *gin.Context) {
		baro_ceremony_html(c, route.View_baro_ceremony_close(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/ceremony/close/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_ceremony_html(c, route.View_baro_ceremony_close(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})
}
