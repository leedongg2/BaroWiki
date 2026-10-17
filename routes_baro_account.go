package main

import (
	"net/http"
	"net/url"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// baro_account_write : 화면 함수 결과를 그대로 내려보낸다
func baro_account_write(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// baro_account_form : POST 본문 읽기
func baro_account_form(c *gin.Context) url.Values {
	_ = c.Request.ParseForm()

	if c.Request.PostForm == nil {
		return url.Values{}
	}

	return c.Request.PostForm
}

// register_baro_account_routes : 2장 계정 · 온보딩 · 첫 기여 도우미 · 프로필 · 연습장 · 알림
func register_baro_account_routes(r *gin.Engine) {
	// --- 2-1 계정 ---
	r.GET("/baro/account", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_me(make_route_config(c), c.Query("page"), nil))
	})

	r.POST("/baro/account", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_me(make_route_config(c), c.Query("page"), baro_account_form(c)))
	})

	r.GET("/baro/account/download", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_download(make_route_config(c), c.Query("page"), nil))
	})

	// 계정 고유 번호 · 계정명 변경 이력
	// `?user=` 에는 계정 이름도, 계정 고유 번호도 넣을 수 있다. 비우면 본인이다.
	r.GET("/baro/account/name", func(c *gin.Context) {
		user := c.Query("user")
		if user == "" {
			user = c.Query("uuid")
		}

		baro_account_write(c, route.View_baro_account_name(make_route_config(c), user))
	})

	// --- 2-2 온보딩 ---
	r.GET("/baro/onboarding", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_onboarding(make_route_config(c), c.Query("step"), nil))
	})

	r.POST("/baro/onboarding", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_onboarding(make_route_config(c), c.Query("step"), baro_account_form(c)))
	})

	r.GET("/baro/onboarding/done", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_onboarding_done(make_route_config(c), c.Query("page"), nil))
	})

	// --- 2-3 / 2-5 초안 · 연습장 ---
	r.GET("/baro/draft", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_draft_list(make_route_config(c), c.Query("page"), nil))
	})

	r.POST("/baro/draft", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_draft_list(make_route_config(c), c.Query("page"), baro_account_form(c)))
	})

	r.GET("/baro/draft/new", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_draft_new(make_route_config(c), c.Query("title"), nil))
	})

	r.POST("/baro/draft/new", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_draft_new(make_route_config(c), c.Query("title"), baro_account_form(c)))
	})

	r.GET("/baro/draft/sandbox", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_sandbox(make_route_config(c), c.Query("from"), nil))
	})

	r.POST("/baro/draft/sandbox", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_sandbox(make_route_config(c), c.Query("from"), baro_account_form(c)))
	})

	r.GET("/baro/draft/cite", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_cite(make_route_config(c), c.Query("kind"), nil))
	})

	r.POST("/baro/draft/cite", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_cite(make_route_config(c), c.Query("kind"), baro_account_form(c)))
	})

	r.GET("/baro/draft/cite/:kind", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_cite(make_route_config(c), c.Param("kind"), nil))
	})

	r.POST("/baro/draft/cite/:kind", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_cite(make_route_config(c), c.Param("kind"), baro_account_form(c)))
	})

	r.GET("/baro/draft/edit/:id", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_draft_edit(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/draft/edit/:id", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_draft_edit(make_route_config(c), c.Param("id"), baro_account_form(c)))
	})

	r.GET("/baro/draft/delete/:id", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_draft_delete(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/draft/delete/:id", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_draft_delete(make_route_config(c), c.Param("id"), baro_account_form(c)))
	})

	r.GET("/baro/draft/publish/:id", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_draft_publish(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/draft/publish/:id", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_draft_publish(make_route_config(c), c.Param("id"), baro_account_form(c)))
	})

	// --- 2-4 프로필 ---
	r.GET("/baro/profile", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_profile(make_route_config(c), "", c.Query("page"), nil))
	})

	r.GET("/baro/profile/edit", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_profile_edit(make_route_config(c), c.Query("page"), nil))
	})

	r.POST("/baro/profile/edit", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_profile_edit(make_route_config(c), c.Query("page"), baro_account_form(c)))
	})

	r.GET("/baro/profile/:user", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_profile(make_route_config(c), c.Param("user"), c.Query("page"), nil))
	})

	// --- 2-6 알림 ---
	r.GET("/baro/notify", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_notify(make_route_config(c), c.Query("page"), c.Query("kind"), c.Query("read"), nil))
	})

	r.POST("/baro/notify", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_notify(make_route_config(c), c.Query("page"), c.Query("kind"), c.Query("read"), baro_account_form(c)))
	})

	r.GET("/baro/notify/set", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_notify_set(make_route_config(c), c.Query("page"), nil))
	})

	r.POST("/baro/notify/set", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_notify_set(make_route_config(c), c.Query("page"), baro_account_form(c)))
	})

	r.GET("/baro/notify/digest", func(c *gin.Context) {
		baro_account_write(c, route.View_baro_account_notify_digest(make_route_config(c), c.Query("page"), nil))
	})
}
