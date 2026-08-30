package main

import (
	"net/http"
	"net/url"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// baro_admin_query : 주소줄의 질의 문자열
func baro_admin_query(c *gin.Context) url.Values {
	return c.Request.URL.Query()
}

// baro_admin_form : POST 로 들어온 값 (없으면 빈 값이라도 nil 이 아니게)
func baro_admin_form(c *gin.Context) url.Values {
	_ = c.Request.ParseForm()

	if c.Request.PostForm == nil {
		return url.Values{}
	}

	return c.Request.PostForm
}

// baro_admin_user_param : /baro/admin/user/view/<이름> 의 이름 부분
func baro_admin_user_param(c *gin.Context) string {
	return strings.TrimPrefix(c.Param("user_name"), "/")
}

// baro_admin_write : 화면 결과 내보내기
func baro_admin_write(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// register_baro_admin_routes : 16장 관리자 백오피스 라우트
func register_baro_admin_routes(r *gin.Engine) {
	// 16-1 운영 대시보드
	r.GET("/baro/admin", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_dash(make_route_config(c)))
	})

	// 16-9 관리자 화면 지도
	r.GET("/baro/admin/map", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_map(make_route_config(c)))
	})

	// 16-2 사용자 관리
	r.GET("/baro/admin/user", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_user(make_route_config(c), baro_admin_query(c)))
	})

	r.GET("/baro/admin/user/view/*user_name", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_user_view(make_route_config(c), baro_admin_user_param(c), baro_admin_query(c), nil))
	})

	r.POST("/baro/admin/user/view/*user_name", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_user_view(make_route_config(c), baro_admin_user_param(c), baro_admin_query(c), baro_admin_form(c)))
	})

	// 16-3 문서 관리
	r.GET("/baro/admin/doc", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_doc(make_route_config(c), baro_admin_query(c), nil))
	})

	r.POST("/baro/admin/doc", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_doc(make_route_config(c), baro_admin_query(c), baro_admin_form(c)))
	})

	// 16-4 신고 · 사건 큐
	r.GET("/baro/admin/case", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_case(make_route_config(c), baro_admin_query(c), nil))
	})

	r.POST("/baro/admin/case", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_case(make_route_config(c), baro_admin_query(c), baro_admin_form(c)))
	})

	// 16-5 프로젝트 관리
	r.GET("/baro/admin/project", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_project(make_route_config(c), baro_admin_query(c), nil))
	})

	r.POST("/baro/admin/project", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_project(make_route_config(c), baro_admin_query(c), baro_admin_form(c)))
	})

	// 16-6 창립 시즌 관리
	r.GET("/baro/admin/founding", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_founding(make_route_config(c), baro_admin_query(c), nil))
	})

	r.POST("/baro/admin/founding", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_founding(make_route_config(c), baro_admin_query(c), baro_admin_form(c)))
	})

	// 16-7 어워드 관리
	r.GET("/baro/admin/award", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_award(make_route_config(c), baro_admin_query(c), nil))
	})

	r.POST("/baro/admin/award", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_award(make_route_config(c), baro_admin_query(c), baro_admin_form(c)))
	})

	// 16-8 기능 플래그
	r.GET("/baro/admin/flag", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_flag(make_route_config(c), baro_admin_query(c), nil))
	})

	r.POST("/baro/admin/flag", func(c *gin.Context) {
		baro_admin_write(c, route.View_baro_admin_flag(make_route_config(c), baro_admin_query(c), baro_admin_form(c)))
	})
}
