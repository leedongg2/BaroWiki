package main

import (
	"net/http"
	"net/url"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// ============================================================
//  바로위키 14장 : 데이터 개방 · 백업 · 내보내기 · 공개 읽기 API
//
//  /baro/export · /baro/backup · /baro/api
// ============================================================

// baro_data_query : 질의 문자열
func baro_data_query(c *gin.Context) url.Values {
	return c.Request.URL.Query()
}

// baro_data_api_key_of : 봇 API 키 (헤더 또는 질의 문자열)
func baro_data_api_key_of(c *gin.Context) string {
	key := strings.TrimSpace(c.GetHeader("X-Baro-Api-Key"))
	if key != "" {
		return key
	}

	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}

	return strings.TrimSpace(c.Query("key"))
}

// baro_data_send : 화면(HTML) 또는 내려받기 파일을 보낸다
//
//	file_name 이 비어 있으면 화면으로 그대로 보여 준다.
func baro_data_send(c *gin.Context, body string, content_type string, file_name string) {
	if file_name != "" {
		c.Header("Content-Disposition",
			`attachment; filename="`+baro_data_ascii_name(file_name)+`"; filename*=UTF-8''`+url.PathEscape(file_name))
		c.Header("X-Content-Type-Options", "nosniff")
	}

	if content_type == "" {
		content_type = "application/octet-stream"
	}

	write_data(c, http.StatusOK, content_type, []byte(body))
}

// baro_data_ascii_ext : ASCII 로만 이루어진 확장자인가 (".json" 처럼)
func baro_data_ascii_ext(ext string) bool {
	if len(ext) < 2 || len(ext) > 6 {
		return false
	}

	for _, ch := range ext[1:] {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9':
		default:
			return false
		}
	}

	return true
}

// baro_data_ascii_name : 옛 브라우저용 ASCII 파일 이름
//
//	한글만으로 된 이름이 통째로 사라져 ".txt" 같은 이름이 되는 것을 막는다.
//	확장자를 먼저 떼어 두고, 남는 이름이 없으면 기본 이름을 쓴다.
func baro_data_ascii_name(name string) string {
	ext := ""
	if dot := strings.LastIndex(name, "."); dot > 0 && baro_data_ascii_ext(name[dot:]) {
		ext = strings.ToLower(name[dot:])
		name = name[:dot]
	}

	out := []rune{}
	for _, ch := range name {
		if ch < 32 || ch == '"' || ch == '\\' || ch == '/' || ch == ';' {
			continue
		}

		if ch > 126 {
			out = append(out, '_')

			continue
		}

		out = append(out, ch)
	}

	text := strings.Trim(string(out), "_. ")
	if text == "" {
		text = "baro_export"
	}

	return text + ext
}

// baro_data_json_out : API JSON 응답 (c.JSON 대신 write_data 로 보낸다)
func baro_data_json_out(c *gin.Context, body string, status int) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")

	write_data(c, status, "application/json; charset=utf-8", []byte(body))
}

func register_baro_data_routes(r *gin.Engine) {
	// ------------------------------------------------------------
	//  14-1 문서 내보내기
	// ------------------------------------------------------------
	r.GET("/baro/export", func(c *gin.Context) {
		route_data := route.View_baro_data_export(make_route_config(c), "1", nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/baro/export", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		route_data := route.View_baro_data_export(make_route_config(c), "1", c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/export/list/:num", func(c *gin.Context) {
		route_data := route.View_baro_data_export(make_route_config(c), c.Param("num"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	for _, kind := range []string{"raw", "history", "pdf", "json", "project", "me"} {
		export_kind := kind

		r.GET("/baro/export/"+export_kind, func(c *gin.Context) {
			body, content_type, file_name := route.View_baro_data_export_file(make_route_config(c), export_kind, baro_data_query(c))
			baro_data_send(c, body, content_type, file_name)
		})
	}

	// ------------------------------------------------------------
	//  14-2 정기 데이터 덤프
	// ------------------------------------------------------------
	r.GET("/baro/export/dump", func(c *gin.Context) {
		route_data := route.View_baro_data_dump(make_route_config(c), "1", nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/baro/export/dump", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		route_data := route.View_baro_data_dump(make_route_config(c), "1", c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/export/dump/list/:num", func(c *gin.Context) {
		route_data := route.View_baro_data_dump(make_route_config(c), c.Param("num"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/export/dump/file", func(c *gin.Context) {
		body, content_type, file_name := route.View_baro_data_dump_file(make_route_config(c), baro_data_query(c))
		baro_data_send(c, body, content_type, file_name)
	})

	// ------------------------------------------------------------
	//  14-3 백업
	// ------------------------------------------------------------
	r.GET("/baro/backup", func(c *gin.Context) {
		route_data := route.View_baro_data_backup(make_route_config(c), "1", nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/baro/backup", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		route_data := route.View_baro_data_backup(make_route_config(c), "1", c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/backup/list/:num", func(c *gin.Context) {
		route_data := route.View_baro_data_backup(make_route_config(c), c.Param("num"), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/backup/verify", func(c *gin.Context) {
		route_data := route.View_baro_data_backup_verify(make_route_config(c), baro_data_query(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/baro/backup/verify", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		route_data := route.View_baro_data_backup_verify(make_route_config(c), baro_data_query(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/backup/set", func(c *gin.Context) {
		route_data := route.View_baro_data_backup_set(make_route_config(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/baro/backup/set", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		route_data := route.View_baro_data_backup_set(make_route_config(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/backup/snapshot", func(c *gin.Context) {
		route_data := route.View_baro_data_backup_snapshot(make_route_config(c), "1")
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/backup/snapshot/:num", func(c *gin.Context) {
		route_data := route.View_baro_data_backup_snapshot(make_route_config(c), c.Param("num"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/backup/file", func(c *gin.Context) {
		body, content_type, file_name := route.View_baro_data_backup_file(make_route_config(c), baro_data_query(c))
		baro_data_send(c, body, content_type, file_name)
	})

	// ------------------------------------------------------------
	//  14-4 공개 읽기 API
	// ------------------------------------------------------------
	r.GET("/baro/api", func(c *gin.Context) {
		route_data := route.View_baro_data_api(make_route_config(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/baro/api/key", func(c *gin.Context) {
		route_data := route.View_baro_data_api_key(make_route_config(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/baro/api/key", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		route_data := route.View_baro_data_api_key(make_route_config(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	for _, kind := range []string{"doc", "search", "recent", "category", "project", "user"} {
		api_kind := kind

		r.GET("/baro/api/"+api_kind, func(c *gin.Context) {
			body, status := route.View_baro_data_api_call(
				make_route_config(c), api_kind, baro_data_query(c), baro_data_api_key_of(c),
			)
			baro_data_json_out(c, body, status)
		})
	}
}
