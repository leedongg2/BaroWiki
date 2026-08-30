package main

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// baro_sec_write : sec 모듈 화면의 응답을 그대로 내보낸다
func baro_sec_write(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// baro_sec_client_ip : 이 요청의 실제 접속 IP
//
//	로그인한 사람은 tool.Config.IP 가 "사용자 이름" 이라서 접속 지점을 알 수 없다.
//	접속 기록과 의심 접속 알림은 실제 IP 대역이 있어야 뜻이 있으므로 여기서 따로 구한다.
func baro_sec_client_ip(c *gin.Context) string {
	for _, name := range []string{"X-Real-IP", "CF-Connecting-IP", "X-Forwarded-For"} {
		for _, part := range strings.Split(c.GetHeader(name), ",") {
			if ip := net.ParseIP(strings.TrimSpace(part)); ip != nil {
				return ip.String()
			}
		}
	}

	if ip := net.ParseIP(strings.TrimSpace(c.ClientIP())); ip != nil {
		return ip.String()
	}

	return ""
}

// baro_sec_query : 질의 문자열에 실제 접속 IP 를 실어 화면 함수로 넘긴다
//
//	같은 이름으로 보낸 값은 반드시 덮어쓰므로 이용자가 IP 를 속일 수 없다.
func baro_sec_query(c *gin.Context) url.Values {
	query := c.Request.URL.Query()
	query.Set("baro_sec_ip", baro_sec_client_ip(c))

	return query
}

// baro_sec_post_form : POST 로 들어온 값 (없으면 빈 값이라도 nil 이 아니어야 한다)
func baro_sec_post_form(c *gin.Context) map[string][]string {
	_ = c.Request.ParseForm()

	return c.Request.PostForm
}

// register_baro_sec_routes : 17장 보안 · 안정성 · 성능 · 접근성 라우트
func register_baro_sec_routes(r *gin.Engine) {
	// --- 17-1 계정 보안 ---
	r.GET("/baro/security", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_home(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/security", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_home(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	r.GET("/baro/security/2fa", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_2fa(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/security/2fa", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_2fa(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	r.GET("/baro/security/login_log", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_login_log(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/security/login_log", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_login_log(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	r.GET("/baro/security/session", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_session(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/security/session", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_session(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	// --- 17-2 악용 방지 ---
	r.GET("/baro/security/abuse", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_abuse(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/security/abuse", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_abuse(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	r.GET("/baro/security/abuse/rate", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_abuse_rate(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/security/abuse/rate", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_abuse_rate(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	r.GET("/baro/security/abuse/signup", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_abuse_signup(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/security/abuse/signup", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_abuse_signup(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	r.GET("/baro/security/abuse/spam", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_abuse_spam(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/security/abuse/spam", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_abuse_spam(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	r.GET("/baro/security/abuse/link", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_abuse_link(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/security/abuse/link", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_abuse_link(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	r.GET("/baro/security/abuse/file", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_abuse_file(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/security/abuse/file", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_abuse_file(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	// --- 17-3 내부 감사 로그 (쓰기 기능 없음) ---
	r.GET("/baro/security/audit", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_audit(make_route_config(c), baro_sec_query(c), nil))
	})

	r.GET("/baro/security/audit/verify", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_audit_verify(make_route_config(c), baro_sec_query(c), nil))
	})

	r.GET("/baro/security/audit/detail", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_audit_detail(make_route_config(c), baro_sec_query(c), nil))
	})

	// --- 17-5 성능 ---
	r.GET("/baro/security/perf", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_perf(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/security/perf", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_perf(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	// --- 17-6 접근성 ---
	r.GET("/baro/security/a11y", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_a11y(make_route_config(c), baro_sec_query(c), nil))
	})

	// --- 17-4 장애 대응 ---
	r.GET("/baro/status", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_status(make_route_config(c), baro_sec_query(c), nil))
	})

	r.GET("/baro/status/incident", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_incident_list(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/status/incident", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_incident_list(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	r.GET("/baro/status/incident/:incident_id", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_incident(make_route_config(c), c.Param("incident_id"), nil))
	})

	r.POST("/baro/status/incident/:incident_id", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_incident(make_route_config(c), c.Param("incident_id"), baro_sec_post_form(c)))
	})

	r.GET("/baro/status/readonly", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_readonly(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/status/readonly", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_readonly(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	r.GET("/baro/status/emergency", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_emergency(make_route_config(c), baro_sec_query(c), nil))
	})

	r.POST("/baro/status/emergency", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_emergency(make_route_config(c), baro_sec_query(c), baro_sec_post_form(c)))
	})

	r.GET("/baro/status/recovery", func(c *gin.Context) {
		baro_sec_write(c, route.View_baro_sec_recovery(make_route_config(c), baro_sec_query(c), nil))
	})
}
