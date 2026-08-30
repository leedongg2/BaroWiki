package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// register_baro_legal_routes : 바로위키 15장 (개인정보 · 저작권 · 법적 요청) 라우트
//
//	URL 접두사 : /baro/privacy , /baro/copyright , /baro/legal
func register_baro_legal_routes(r *gin.Engine) {
	baro_legal_html := "text/html; charset=utf-8"

	// ----------------------------------------------------
	//  15-1 / 15-2 개인정보
	// ----------------------------------------------------

	r.GET("/baro/privacy", func(c *gin.Context) {
		data := route.View_baro_legal_privacy_home(make_route_config(c), "1", nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/privacy/policy", func(c *gin.Context) {
		data := route.View_baro_legal_privacy_policy(make_route_config(c))
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/privacy/retention", func(c *gin.Context) {
		data := route.View_baro_legal_privacy_retention(make_route_config(c), nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.POST("/baro/privacy/retention", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_legal_privacy_retention(make_route_config(c), c.Request.PostForm)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/privacy/new", func(c *gin.Context) {
		data := route.View_baro_legal_privacy_new(make_route_config(c), c.Query("kind"), nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.POST("/baro/privacy/new", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_legal_privacy_new(make_route_config(c), "", c.Request.PostForm)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/privacy/mine", func(c *gin.Context) {
		data := route.View_baro_legal_privacy_mine(make_route_config(c), "1")
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/privacy/mine/:page", func(c *gin.Context) {
		data := route.View_baro_legal_privacy_mine(make_route_config(c), c.Param("page"))
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/privacy/queue", func(c *gin.Context) {
		data := route.View_baro_legal_privacy_queue(make_route_config(c), "open", "1")
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/privacy/queue/:mode/:page", func(c *gin.Context) {
		data := route.View_baro_legal_privacy_queue(make_route_config(c), c.Param("mode"), c.Param("page"))
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/privacy/req/:id", func(c *gin.Context) {
		data := route.View_baro_legal_privacy_req(make_route_config(c), c.Param("id"), nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.POST("/baro/privacy/req/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_legal_privacy_req(make_route_config(c), c.Param("id"), c.Request.PostForm)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/privacy/view/:id", func(c *gin.Context) {
		data := route.View_baro_legal_privacy_view(make_route_config(c), c.Param("id"))
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/privacy/download/:id", func(c *gin.Context) {
		content_type, file_name, data := route.View_baro_legal_privacy_download(make_route_config(c), c.Param("id"))
		if file_name != "" {
			c.Header("Content-Disposition", `attachment; filename="`+file_name+`"`)
		}

		write_data(c, http.StatusOK, content_type, []byte(data))
	})

	r.GET("/baro/privacy/leave", func(c *gin.Context) {
		data := route.View_baro_legal_privacy_leave(make_route_config(c), nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.POST("/baro/privacy/leave", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_legal_privacy_leave(make_route_config(c), c.Request.PostForm)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/privacy/del/:id", func(c *gin.Context) {
		data := route.View_baro_legal_privacy_del(make_route_config(c), c.Param("id"), nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.POST("/baro/privacy/del/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_legal_privacy_del(make_route_config(c), c.Param("id"), c.Request.PostForm)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	// ----------------------------------------------------
	//  15-3 저작권
	// ----------------------------------------------------

	r.GET("/baro/copyright", func(c *gin.Context) {
		data := route.View_baro_legal_copyright_home(make_route_config(c))
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/copyright/new", func(c *gin.Context) {
		data := route.View_baro_legal_copyright_new(make_route_config(c), nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.POST("/baro/copyright/new", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_legal_copyright_new(make_route_config(c), c.Request.PostForm)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/copyright/mine", func(c *gin.Context) {
		data := route.View_baro_legal_copyright_mine(make_route_config(c), "1")
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/copyright/mine/:page", func(c *gin.Context) {
		data := route.View_baro_legal_copyright_mine(make_route_config(c), c.Param("page"))
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/copyright/queue", func(c *gin.Context) {
		data := route.View_baro_legal_copyright_queue(make_route_config(c), "open", "1", nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/copyright/queue/:mode/:page", func(c *gin.Context) {
		data := route.View_baro_legal_copyright_queue(make_route_config(c), c.Param("mode"), c.Param("page"), nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.POST("/baro/copyright/queue/:mode/:page", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_legal_copyright_queue(make_route_config(c), c.Param("mode"), c.Param("page"), c.Request.PostForm)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/copyright/req/:id", func(c *gin.Context) {
		data := route.View_baro_legal_copyright_req(make_route_config(c), c.Param("id"), nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.POST("/baro/copyright/req/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_legal_copyright_req(make_route_config(c), c.Param("id"), c.Request.PostForm)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/copyright/counter/:id", func(c *gin.Context) {
		data := route.View_baro_legal_copyright_counter(make_route_config(c), c.Param("id"), nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.POST("/baro/copyright/counter/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_legal_copyright_counter(make_route_config(c), c.Param("id"), c.Request.PostForm)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	// ----------------------------------------------------
	//  15-4 / 15-5 라이선스 동의와 통계
	// ----------------------------------------------------

	r.GET("/baro/legal", func(c *gin.Context) {
		data := route.View_baro_legal_log(make_route_config(c), c.Query("period"), "1")
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/legal/page/:page", func(c *gin.Context) {
		data := route.View_baro_legal_log(make_route_config(c), c.Query("period"), c.Param("page"))
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/legal/license", func(c *gin.Context) {
		data := route.View_baro_legal_license(make_route_config(c), nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.POST("/baro/legal/license", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_legal_license(make_route_config(c), c.Request.PostForm)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/legal/consent", func(c *gin.Context) {
		data := route.View_baro_legal_consent(make_route_config(c), "1")
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/legal/consent/:page", func(c *gin.Context) {
		data := route.View_baro_legal_consent(make_route_config(c), c.Param("page"))
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.GET("/baro/legal/make", func(c *gin.Context) {
		data := route.View_baro_legal_make(make_route_config(c), c.Query("period"), nil)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})

	r.POST("/baro/legal/make", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_legal_make(make_route_config(c), "", c.Request.PostForm)
		write_data(c, http.StatusOK, baro_legal_html, []byte(data))
	})
}
