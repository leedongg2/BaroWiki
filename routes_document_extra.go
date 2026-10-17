package main

import (
	"net/http"

	"opennamu/route"
	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

func register_document_extra_routes(r *gin.Engine) {
	r.GET("/edit_from/*doc_name", func(c *gin.Context) {
		data := route.View_edit(make_route_config(c), doc_param(c), c.Query("load"), c.Query("section"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
	r.POST("/edit_from/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		send := tool.Baro_ai_send(c.PostForm("send"), tool.Baro_ai_on(c.PostForm(tool.Baro_ai_field)))
		data := route.View_edit_post(make_route_config(c), doc_param(c), c.PostForm("content"), send, c.PostForm("copyright_agreement"), captcha_response(c), c.PostForm("ver"), c.PostForm("section"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/history_tool/:rev/*doc_name", func(c *gin.Context) {
		data := route.View_history_tool(make_route_config(c), doc_param(c), c.Param("rev"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
	r.GET("/history_hidden/:rev/*doc_name", func(c *gin.Context) {
		data := route.View_history_hidden_safe(make_route_config(c), doc_param(c), c.Param("rev"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
	r.POST("/history_hidden/:rev/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_history_hidden_safe(make_route_config(c), doc_param(c), c.Param("rev"), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
	r.GET("/history_send/:rev/*doc_name", func(c *gin.Context) {
		data := route.View_history_send_safe(make_route_config(c), doc_param(c), c.Param("rev"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
	r.POST("/history_send/:rev/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_history_send_safe(make_route_config(c), doc_param(c), c.Param("rev"), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
	r.GET("/history_delete/:rev/*doc_name", func(c *gin.Context) {
		data := route.View_history_delete(make_route_config(c), doc_param(c), c.Param("rev"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
	r.POST("/history_delete/:rev/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_history_delete(make_route_config(c), doc_param(c), c.Param("rev"), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
	r.GET("/history_reset/*doc_name", func(c *gin.Context) {
		data := route.View_history_reset(make_route_config(c), doc_param(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
	r.POST("/history_reset/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_history_reset(make_route_config(c), doc_param(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
	r.GET("/history_add/*doc_name", func(c *gin.Context) {
		data := route.View_history_add_safe(make_route_config(c), doc_param(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
	r.POST("/history_add/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_history_add_safe(make_route_config(c), doc_param(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
}
