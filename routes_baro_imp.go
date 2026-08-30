package main

import (
	"io"
	"net/http"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// baro_imp_max_upload : 한 번에 받는 업로드 크기 한도 (32MB)
const baro_imp_max_upload = 32 << 20

// baro_imp_max_file : 파일 하나에서 읽어 들이는 최대 글자 수
const baro_imp_max_file = 8 << 20

// baro_imp_read_files : 올라온 파일을 이름과 내용 문자열로 읽는다
func baro_imp_read_files(c *gin.Context) ([]string, []string) {
	names := []string{}
	bodies := []string{}

	if c.Request == nil {
		return names, bodies
	}

	if err := c.Request.ParseMultipartForm(baro_imp_max_upload); err != nil {
		return names, bodies
	}

	if c.Request.MultipartForm == nil {
		return names, bodies
	}

	for _, key := range []string{"files", "file"} {
		for _, header := range c.Request.MultipartForm.File[key] {
			if header == nil {
				continue
			}

			opened, err := header.Open()
			if err != nil {
				continue
			}

			raw, err := io.ReadAll(io.LimitReader(opened, baro_imp_max_file))
			_ = opened.Close()

			if err != nil {
				continue
			}

			name := header.Filename
			if name == "" {
				name = "이름 없는 파일"
			}

			names = append(names, name)
			bodies = append(bodies, string(raw))
		}
	}

	return names, bodies
}

// baro_imp_html : 화면 결과 내보내기
func baro_imp_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

func register_baro_imp_routes(r *gin.Engine) {
	// 13-1 가져오기 작업 목록
	r.GET("/baro/import", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_list(make_route_config(c), "1"))
	})

	r.GET("/baro/import/list/:num", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_list(make_route_config(c), c.Param("num")))
	})

	// 13-1 새 가져오기 (파일 업로드)
	r.GET("/baro/import/new", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_new(make_route_config(c), nil, nil, nil))
	})

	r.POST("/baro/import/new", func(c *gin.Context) {
		names, bodies := baro_imp_read_files(c)
		_ = c.Request.ParseForm()

		baro_imp_html(c, route.View_baro_imp_new(make_route_config(c), c.Request.PostForm, names, bodies))
	})

	// 13-1 가져오기 작업 상세 · 미리보기
	r.GET("/baro/import/view/:id", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_view(make_route_config(c), c.Param("id"), "1"))
	})

	r.GET("/baro/import/view/:id/:num", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_view(make_route_config(c), c.Param("id"), c.Param("num")))
	})

	// 13-1 선택적 가져오기 실행
	r.GET("/baro/import/run/:id", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_run(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/import/run/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_imp_html(c, route.View_baro_imp_run(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	// 13-1 오류 보고서
	r.GET("/baro/import/report/:id", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_report(make_route_config(c), c.Param("id"), "1"))
	})

	r.GET("/baro/import/report/:id/:num", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_report(make_route_config(c), c.Param("id"), c.Param("num")))
	})

	// 13-2 라이선스 · 출처 기록
	r.GET("/baro/import/license/:id", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_license(make_route_config(c), c.Param("id"), "1"))
	})

	r.GET("/baro/import/license/:id/:num", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_license(make_route_config(c), c.Param("id"), c.Param("num")))
	})

	// 13-3 옛 주소 연결
	r.GET("/baro/import/url", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_url(make_route_config(c), "1", nil))
	})

	r.POST("/baro/import/url", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_imp_html(c, route.View_baro_imp_url(make_route_config(c), "1", c.Request.PostForm))
	})

	r.GET("/baro/import/url/list/:num", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_url(make_route_config(c), c.Param("num"), nil))
	})

	r.GET("/baro/import/url/broken", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_url_broken(make_route_config(c), "1"))
	})

	r.GET("/baro/import/url/broken/:num", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_url_broken(make_route_config(c), c.Param("num")))
	})

	// 13-3 실제 리다이렉트 : /baro/import/go?u=<옛주소>
	r.GET("/baro/import/go", func(c *gin.Context) {
		old_url := c.Query("u")
		if strings.TrimSpace(old_url) == "" {
			old_url = c.Query("url")
		}

		baro_imp_html(c, route.View_baro_imp_go(make_route_config(c), old_url))
	})

	// 13-4 대량 가져오기 봇
	r.GET("/baro/import/bot", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_bot(make_route_config(c), nil))
	})

	r.POST("/baro/import/bot", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_imp_html(c, route.View_baro_imp_bot(make_route_config(c), c.Request.PostForm))
	})

	// 13-4 사전 검토 (테스트 가져오기 10건)
	r.GET("/baro/import/test/:id", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_test(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/import/test/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_imp_html(c, route.View_baro_imp_test(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})

	// 13-4 전체 취소
	r.GET("/baro/import/undo/:id", func(c *gin.Context) {
		baro_imp_html(c, route.View_baro_imp_undo(make_route_config(c), c.Param("id"), nil))
	})

	r.POST("/baro/import/undo/:id", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		baro_imp_html(c, route.View_baro_imp_undo(make_route_config(c), c.Param("id"), c.Request.PostForm))
	})
}
