package main

import (
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"opennamu/route"
	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

// ============================================================
//  밐 · 바로샵 · 프로필 사진 길 등록
//
//  URL 접두사 : /baro/mik · /baro/shop · /baro/avatar
// ============================================================

// baro_mik_html : 화면 함수 결과 내려보내기
func baro_mik_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// baro_mik_form : POST 본문
func baro_mik_form(c *gin.Context) url.Values {
	_ = c.Request.ParseForm()

	if c.Request.PostForm == nil {
		return url.Values{}
	}

	return c.Request.PostForm
}

// register_baro_mik_routes : 밐 · 바로샵 · 프로필 사진
func register_baro_mik_routes(r *gin.Engine) {
	// --- 밐 지갑 ---
	r.GET("/baro/mik", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik(make_route_config(c), nil))
	})

	r.POST("/baro/mik", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik(make_route_config(c), baro_mik_form(c)))
	})

	r.GET("/baro/mik/log", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_log(make_route_config(c), c.Query("page")))
	})

	r.GET("/baro/mik/rank", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_rank(make_route_config(c), c.Request.URL.Query()))
	})

	r.GET("/baro/mik/send", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_send(make_route_config(c), nil))
	})

	r.POST("/baro/mik/send", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_send(make_route_config(c), baro_mik_form(c)))
	})

	// --- 관리자 지급 ---
	r.GET("/baro/mik/admin", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_admin(make_route_config(c), nil))
	})

	r.POST("/baro/mik/admin", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_admin(make_route_config(c), baro_mik_form(c)))
	})

	r.GET("/baro/mik/setting", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_setting(make_route_config(c), nil))
	})

	r.POST("/baro/mik/setting", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_setting(make_route_config(c), baro_mik_form(c)))
	})

	// --- 바로샵 ---
	r.GET("/baro/shop", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_shop(make_route_config(c), nil))
	})

	r.POST("/baro/shop", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_shop(make_route_config(c), baro_mik_form(c)))
	})

	r.GET("/baro/shop/admin", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_shop_admin(make_route_config(c), nil))
	})

	r.POST("/baro/shop/admin", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_shop_admin(make_route_config(c), baro_mik_form(c)))
	})

	r.GET("/baro/shop/admin/edit/:id", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_shop_edit(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/shop/admin/edit/:id", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_mik_shop_edit(make_route_config(c), c.Param("id"), baro_mik_form(c)))
	})

	// --- 프로필 사진 ---
	r.GET("/baro/avatar", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_account_avatar(make_route_config(c), "", nil, ""))
	})

	r.POST("/baro/avatar", baro_avatar_upload)

	r.POST("/baro/avatar/delete", func(c *gin.Context) {
		baro_mik_html(c, route.View_baro_account_avatar(make_route_config(c), "delete", nil, ""))
	})

	// 올린 사진 내려주기. 파일 이름이 아니라 계정 이름으로 찾는다.
	r.GET("/baro/avatar/file/*user", route.View_baro_account_avatar_file)
}

// baro_avatar_upload : 프로필 사진 올리기 (multipart)
func baro_avatar_upload(c *gin.Context) {
	max_bytes := int64(tool.Baro_avatar_max_bytes)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max_bytes+1000*1000)

	form, err := c.MultipartForm()
	if err != nil || form == nil {
		baro_mik_html(c, route.View_baro_account_avatar(make_route_config(c), "upload", nil, ""))
		return
	}

	files := form.File["avatar"]
	if len(files) == 0 {
		baro_mik_html(c, route.View_baro_account_avatar(make_route_config(c), "upload", nil, ""))
		return
	}

	fh := files[0]

	f, err := fh.Open()
	if err != nil {
		baro_mik_html(c, route.View_baro_account_avatar(make_route_config(c), "upload", nil, ""))
		return
	}

	raw, err := io.ReadAll(io.LimitReader(f, max_bytes+1))
	_ = f.Close()

	if err != nil {
		raw = nil
	}

	name := filepath.Base(strings.TrimSpace(fh.Filename))

	baro_mik_html(c, route.View_baro_account_avatar(make_route_config(c), "upload", raw, name))
}
