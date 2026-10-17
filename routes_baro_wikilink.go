package main

import (
	"net/http"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// register_baro_wikilink_routes : 위키 데이터 연동 라우트
func register_baro_wikilink_routes(r *gin.Engine) {
	html := func(c *gin.Context, data string) {
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	}

	r.GET("/baro/wikilink", func(c *gin.Context) {
		html(c, route.View_baro_link(make_route_config(c)))
	})

	r.GET("/baro/wikilink/wiki", func(c *gin.Context) {
		html(c, route.View_baro_link_wiki(make_route_config(c)))
	})

	r.GET("/baro/wikilink/new", func(c *gin.Context) {
		html(c, route.View_baro_link_wiki_edit(make_route_config(c), "", nil))
	})

	r.POST("/baro/wikilink/new", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		html(c, route.View_baro_link_wiki_edit(make_route_config(c), "", c.Request.PostForm))
	})

	r.GET("/baro/wikilink/edit/:id", func(c *gin.Context) {
		html(c, route.View_baro_link_wiki_edit(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/wikilink/edit/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		html(c, route.View_baro_link_wiki_edit(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	r.GET("/baro/wikilink/set", func(c *gin.Context) {
		html(c, route.View_baro_link_set(make_route_config(c), nil))
	})

	r.POST("/baro/wikilink/set", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		html(c, route.View_baro_link_set(make_route_config(c), c.Request.PostForm))
	})

	r.GET("/baro/wikilink/compare/:id", func(c *gin.Context) {
		html(c, route.View_baro_link_compare(make_route_config(c), c.Param("id")))
	})

	r.GET("/baro/wikilink/doc/*doc_name", func(c *gin.Context) {
		html(c, route.View_baro_link_doc(make_route_config(c), strings.TrimPrefix(c.Param("doc_name"), "/"), nil))
	})

	r.POST("/baro/wikilink/doc/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		html(c, route.View_baro_link_doc(make_route_config(c), strings.TrimPrefix(c.Param("doc_name"), "/"), c.Request.PostForm))
	})
}
