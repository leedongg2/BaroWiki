package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// register_baro_home_routes : 바로위키 [home] 모듈 라우트
//
//	/baro           바로위키 허브 홈
//	/baro/founding  1-1 창립 시즌 랜딩 (POST 는 출시 알림 신청)
//	/baro/about     바로위키 소개 + 0장 고정 원칙 전문
//	/baro/help      도움말
//	/baro/map       19장 화면 지도
func register_baro_home_routes(r *gin.Engine) {
	r.GET("/baro", func(c *gin.Context) {
		route_data := route.View_baro_home_index(make_route_config(c), c.Request.URL.Query(), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/founding", func(c *gin.Context) {
		route_data := route.View_baro_home_founding(make_route_config(c), c.Request.URL.Query(), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/baro/founding", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		route_data := route.View_baro_home_founding(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/about", func(c *gin.Context) {
		route_data := route.View_baro_home_about(make_route_config(c), c.Request.URL.Query(), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/help", func(c *gin.Context) {
		route_data := route.View_baro_home_help(make_route_config(c), c.Request.URL.Query(), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/map", func(c *gin.Context) {
		route_data := route.View_baro_home_map(make_route_config(c), c.Request.URL.Query(), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})
}
