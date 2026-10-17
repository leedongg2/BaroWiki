package main

import (
	"net/http"
	"strings"

	"opennamu/route"
	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

func register_access_routes(r *gin.Engine) {
	r.GET("/wiki_access", func(c *gin.Context) {
		data := route.View_wiki_access(make_route_config(c))
		write_data(c, http.StatusForbidden, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/wiki_access", func(c *gin.Context) {
		password := c.PostForm("password")
		if !route.Check_wiki_access(password) {
			c.Redirect(http.StatusFound, "/wiki_access")
			return
		}

		http.SetCookie(c.Writer, &http.Cookie{
			Name:     "opennamu_wiki_access",
			Value:    tool.Url_parser(password),
			Path:     "/",
			HttpOnly: false,
			SameSite: http.SameSiteLaxMode,
		})
		next := c.PostForm("next")
		if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
			next = "/"
		}
		c.Redirect(http.StatusFound, next)
	})
}

func wiki_access_middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/wiki_access" || path == "/healthz" || path == "/readyz" || strings.HasPrefix(path, "/views/") || strings.HasPrefix(path, "/view/") {
			c.Next()
			return
		}

		db := tool.DB_connect()
		defer tool.DB_close(db)

		// 화면 문구 번역이 본문을 만들기 전에 정해지도록 요청마다 먼저 읽는다
		tool.Wiki_language(db)

		// 앞단(CDN·프록시)이 나라를 붙여 준다면 여기서 한 번 받아 적어 둔다.
		// ACL 의 geoip 규칙이 이 값을 쓴다.
		geoip_header_read(c, db)

		password_need := ""
		tool.QueryRow_DB(db, "select data from other where name = 'wiki_access_password_need'", []any{&password_need})
		if password_need == "" {
			c.Next()
			return
		}

		password := ""
		tool.QueryRow_DB(db, "select data from other where name = 'wiki_access_password'", []any{&password})
		if password == "" {
			c.Next()
			return
		}

		cookie, _ := c.Cookie("opennamu_wiki_access")
		if cookie == tool.Url_parser(password) {
			c.Next()
			return
		}

		next := tool.HTML_escape(c.Request.URL.RequestURI())
		body := "<h2>" + tool.Get_language(db, "error_password_require_for_wiki_access", true) + "</h2>"
		body += "<form method='post' action='/wiki_access'><input type='hidden' name='next' value='" + next + "'><input type='password' name='password'><button type='submit'>submit</button></form>"
		write_data(c, http.StatusForbidden, "text/html; charset=utf-8", []byte(body))
		c.Abort()
	}
}
