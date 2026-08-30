package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// register_baro_metric_routes : 18장 분석 지표 라우트
//
//	모든 주소는 /baro/metrics 아래에만 만든다.
func register_baro_metric_routes(r *gin.Engine) {
	// 요약 대시보드 (★ 주간 유효 기여자 수가 맨 위에 가장 크게 나온다)
	r.GET("/baro/metrics", func(c *gin.Context) {
		data := route.View_baro_metric_dash(make_route_config(c), c.Query("span"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	// 유입 지표
	r.GET("/baro/metrics/acquisition", func(c *gin.Context) {
		data := route.View_baro_metric_acquisition(make_route_config(c), c.Query("span"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	// 활성화 지표
	r.GET("/baro/metrics/activation", func(c *gin.Context) {
		data := route.View_baro_metric_activation(make_route_config(c), c.Query("span"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	// 유지 지표
	r.GET("/baro/metrics/retention", func(c *gin.Context) {
		data := route.View_baro_metric_retention(make_route_config(c), c.Query("span"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	// 품질 지표
	r.GET("/baro/metrics/quality", func(c *gin.Context) {
		data := route.View_baro_metric_quality(make_route_config(c), c.Query("span"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	// 운영 신뢰 지표
	r.GET("/baro/metrics/trust", func(c *gin.Context) {
		data := route.View_baro_metric_trust(make_route_config(c), c.Query("span"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	// 저장된 추이 (목록 + 쪽 넘기기)
	r.GET("/baro/metrics/history", func(c *gin.Context) {
		data := route.View_baro_metric_history(make_route_config(c), "1", c.Query("span"), c.Query("key"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/metrics/history/:num", func(c *gin.Context) {
		data := route.View_baro_metric_history(make_route_config(c), c.Param("num"), c.Query("span"), c.Query("key"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	// 장기 미처리 사건 목록 (목록 + 쪽 넘기기)
	r.GET("/baro/metrics/backlog", func(c *gin.Context) {
		data := route.View_baro_metric_backlog(make_route_config(c), "1", c.Query("span"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/baro/metrics/backlog/:num", func(c *gin.Context) {
		data := route.View_baro_metric_backlog(make_route_config(c), c.Param("num"), c.Query("span"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	// 오늘 값 저장 (권한 : system)
	r.GET("/baro/metrics/snapshot", func(c *gin.Context) {
		data := route.View_baro_metric_snapshot(make_route_config(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/metrics/snapshot", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_metric_snapshot(make_route_config(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
}
