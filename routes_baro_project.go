package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// register_baro_project_routes : 바로위키 6장 (분야 개척 시스템) 라우트
//
//	URL 접두사 : /baro/field · /baro/project · /baro/task
func register_baro_project_routes(r *gin.Engine) {
	baro_project_html := func(c *gin.Context, data string) {
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	}

	// ---------- 6-1 / 6-2 분야 허브 ----------

	r.GET("/baro/field", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_field_list(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/field/view/:key", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_field_view(make_route_config(c), c.Param("key"), c.Request.URL.Query()))
	})

	// ---------- 6-3 과제 ----------

	r.GET("/baro/task", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_task_list(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/task/new", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_task_new(make_route_config(c), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/task/new", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_task_new(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm))
	})

	r.GET("/baro/task/mine", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_task_mine(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/task/edit/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_task_edit(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/task/edit/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_task_edit(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	r.GET("/baro/task/view/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_task_view(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/task/view/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_task_view(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	// ---------- 6-4 프로젝트 ----------

	r.GET("/baro/project", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_list(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/project/new", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_new(make_route_config(c), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/project/new", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_new(make_route_config(c), c.Request.URL.Query(), c.Request.PostForm))
	})

	r.GET("/baro/project/view/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_view(make_route_config(c), c.Param("id"), c.Request.URL.Query()))
	})

	r.GET("/baro/project/edit/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_edit(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/project/edit/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_edit(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	r.GET("/baro/project/join/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_join(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/project/join/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_join(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	r.GET("/baro/project/leave/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_leave(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/project/leave/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_leave(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	r.GET("/baro/project/target/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_target(make_route_config(c), c.Param("id"), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/project/target/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_target(make_route_config(c), c.Param("id"), c.Request.URL.Query(), c.Request.PostForm))
	})

	r.GET("/baro/project/task/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_task_of(make_route_config(c), c.Param("id"), c.Request.URL.Query()))
	})

	r.GET("/baro/project/source/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_source(make_route_config(c), c.Param("id"), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/project/source/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_source(make_route_config(c), c.Param("id"), c.Request.URL.Query(), c.Request.PostForm))
	})

	r.GET("/baro/project/progress/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_progress(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/project/progress/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_progress(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	r.GET("/baro/project/report/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_report(make_route_config(c), c.Param("id"), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/project/report/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_report(make_route_config(c), c.Param("id"), c.Request.URL.Query(), c.Request.PostForm))
	})

	// ---------- 6-5 역할 ----------

	r.GET("/baro/project/member/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_member(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/project/member/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_member(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	// ---------- 6-6 공동 작업실 ----------

	r.GET("/baro/project/room/:id", func(c *gin.Context) {
		baro_project_html(c, route.View_baro_project_room(make_route_config(c), c.Param("id"), c.Request.URL.Query(), nil))
	})

	r.POST("/baro/project/room/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_project_html(c, route.View_baro_project_room(make_route_config(c), c.Param("id"), c.Request.URL.Query(), c.Request.PostForm))
	})
}
