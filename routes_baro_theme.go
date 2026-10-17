package main

import (
	"net/http"
	"net/url"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// ============================================================
//  테마 길 등록
//
//  테마 링크는 /baro/theme/<uuid> 한 가지다.
//  그 밑에 /edit/<uuid> 같은 길도 함께 두어야 하는데, gin 에서
//  `/baro/theme/:uuid` 와 `/baro/theme/edit/:uuid` 는 서로 부딪힌다.
//  그래서 한 자리에서 모두 받아 Go 쪽에서 갈라 보낸다.
// ============================================================

// baro_theme_html : 화면 함수 결과 내려보내기
func baro_theme_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// baro_theme_values : 보낸 값 (GET 이면 물음표 뒤, POST 면 본문)
func baro_theme_values(c *gin.Context) url.Values {
	if c.Request.Method != http.MethodPost {
		return c.Request.URL.Query()
	}

	_ = c.Request.ParseForm()

	if c.Request.PostForm == nil {
		return url.Values{}
	}

	// 물음표 뒤에 붙은 값도 함께 볼 수 있게 합친다.
	out := url.Values{}
	for key, list := range c.Request.URL.Query() {
		out[key] = append(out[key], list...)
	}
	for key, list := range c.Request.PostForm {
		out[key] = append(out[key], list...)
	}

	return out
}

// register_baro_theme_routes : 사용자 지정 테마
func register_baro_theme_routes(r *gin.Engine) {
	r.GET("/baro/theme", baro_theme_route)
	r.POST("/baro/theme", baro_theme_route)
	r.GET("/baro/theme/*rest", baro_theme_route)
	r.POST("/baro/theme/*rest", baro_theme_route)
}

// baro_theme_route : /baro/theme 아래를 갈라 보낸다
func baro_theme_route(c *gin.Context) {
	config := make_route_config(c)
	values := baro_theme_values(c)

	rest := strings.Trim(strings.TrimPrefix(c.Param("rest"), "/"), "/")
	if rest == "" {
		baro_theme_html(c, route.View_baro_theme_list(config, values))
		return
	}

	head, tail, _ := strings.Cut(rest, "/")

	switch head {
	case "new":
		baro_theme_html(c, route.View_baro_theme_new(config, values))

	case "use":
		baro_theme_html(c, route.View_baro_theme_use(config, values))

	case "edit":
		baro_theme_html(c, route.View_baro_theme_edit(config, tail, values))

	case "delete":
		baro_theme_html(c, route.View_baro_theme_delete(config, tail, values))

	case "official":
		baro_theme_html(c, route.View_baro_theme_official(config, tail, values))

	default:
		// 나머지는 모두 테마 고유 번호로 본다. (/baro/theme/<uuid>)
		baro_theme_html(c, route.View_baro_theme_view(config, head, values))
	}
}
