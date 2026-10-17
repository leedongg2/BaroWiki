package main

import (
	"net/http"
	"strings"

	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

// ============================================================
//  바로 캡차 길 등록
//
//  /captcha/new        새 문제 하나를 만들고 이름표만 돌려준다
//  /captcha/<이름표>.png  그 문제의 그림
//
//  답은 절대 내려보내지 않는다. 그림에만 들어 있다.
// ============================================================

// register_baro_captcha_routes : 캡차 그림 길
func register_baro_captcha_routes(r *gin.Engine) {
	r.GET("/captcha/new", func(c *gin.Context) {
		db := tool.DB_connect()
		defer tool.DB_close(db)

		code := tool.Baro_captcha_new(db, tool.Get_IP(c))

		c.Header("Cache-Control", "no-store")
		c.String(http.StatusOK, code)
	})

	r.GET("/captcha/:code", func(c *gin.Context) {
		db := tool.DB_connect()
		defer tool.DB_close(db)

		code := strings.TrimSuffix(c.Param("code"), ".png")

		answer := tool.Baro_captcha_answer(db, code)
		if answer == "" {
			c.String(http.StatusNotFound, "")
			return
		}

		raw := tool.Baro_captcha_png(answer)
		if len(raw) == 0 {
			c.String(http.StatusInternalServerError, "")
			return
		}

		// 그림을 어디에도 남기지 않는다. 한 번 보고 마는 것이다.
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, "image/png", raw)
	})
}
