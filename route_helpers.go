package main

import (
	"net/http"
	"strings"

	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

// doc_param : URL에서 문서 이름을 읽고 이름공간 접두사를 내부 형식으로 변환
func doc_param(c *gin.Context) string {
	return tool.Namespace_normalize(strings.TrimPrefix(c.Param("doc_name"), "/"))
}

func make_route_config(c *gin.Context) tool.Config {
	return tool.Config{
		IP:        tool.Get_IP(c),
		Cookies:   tool.Get_Cookies(c),
		Session:   tool.Get_session(c),
		UserAgent: c.GetHeader("User-Agent"),
	}
}

func captcha_response(c *gin.Context) string {
	out := tool.Captcha_response(c.PostForm("g-recaptcha"), c.PostForm("g-recaptcha-response"), c.PostForm("h-captcha-response"), c.PostForm("cf-turnstile-response"))
	if out != "" {
		return out
	}

	// 바로 캡차는 「문제 이름표」 와 「적어 낸 답」 두 칸을 쓴다.
	// 본체 검사는 값을 하나만 받으므로 한 줄로 묶어 넘긴다.
	return tool.Baro_captcha_pack(c.PostForm("bw_captcha_code"), c.PostForm("bw_captcha"))
}

func parse_bbs_code(value string) (string, string) {
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func write_data(c *gin.Context, status int, content_type string, data []byte) {
	target, ok := tool.Get_redirect_target(string(data))
	if ok {
		c.Redirect(http.StatusFound, target)
		return
	}

	c.Data(status, content_type, data)
}
