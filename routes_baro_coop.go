package main

import (
	"net/http"
	"net/url"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// ============================================================
//  편집방 길 등록
//
//  방 주소가 /baro/cooperation/<이름표> 한 가지이고 그 아래 탭도 있어야 해서,
//  테마와 같은 까닭으로 한 자리에서 받아 Go 쪽에서 갈라 보낸다.
// ============================================================

// baro_coop_html : 화면 함수 결과 내려보내기
func baro_coop_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// baro_coop_values : 보낸 값 (물음표 뒤와 본문을 합친다)
func baro_coop_values(c *gin.Context) url.Values {
	out := url.Values{}

	for key, list := range c.Request.URL.Query() {
		out[key] = append(out[key], list...)
	}

	if c.Request.Method == http.MethodPost {
		_ = c.Request.ParseForm()

		for key, list := range c.Request.PostForm {
			out[key] = append(out[key], list...)
		}
	}

	return out
}

// register_baro_coop_routes : 편집방
func register_baro_coop_routes(r *gin.Engine) {
	r.GET("/baro/cooperation", baro_coop_route)
	r.POST("/baro/cooperation", baro_coop_route)
	r.GET("/baro/cooperation/*rest", baro_coop_route)
	r.POST("/baro/cooperation/*rest", baro_coop_route)
}

// baro_coop_route : /baro/cooperation 아래를 갈라 보낸다
func baro_coop_route(c *gin.Context) {
	config := make_route_config(c)
	values := baro_coop_values(c)

	rest := strings.Trim(strings.TrimPrefix(c.Param("rest"), "/"), "/")
	if rest == "" {
		baro_coop_html(c, route.View_baro_coop_list(config, "public", values))
		return
	}

	head, tail, _ := strings.Cut(rest, "/")

	switch head {
	case "new":
		baro_coop_html(c, route.View_baro_coop_new(config, values))
		return

	case "mine":
		baro_coop_html(c, route.View_baro_coop_list(config, "mine", values))
		return
	}

	// 나머지는 모두 방 이름표로 본다. 그 뒤가 탭이다.
	switch tail {
	case "setting":
		baro_coop_html(c, route.View_baro_coop_setting(config, head, values))
	case "work":
		baro_coop_html(c, route.View_baro_coop_work(config, head, values))
	case "talk", "detail":
		baro_coop_html(c, route.View_baro_coop_room(config, head, tail, values))
	default:
		baro_coop_html(c, route.View_baro_coop_room(config, head, "room", values))
	}
}
