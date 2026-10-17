package main

import (
	"net/http"
	"net/url"
	"strings"

	"opennamu/route"
	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

// ============================================================
//  편집 요청 짧은 주소
//
//  편집 요청 자체는 `/baro/req` 가 다 맡는다.
//  여기 있는 것은 문서 화면에서 바로 걸기 좋은 짧은 주소일 뿐이고,
//  하는 일은 `/baro/req/...` 로 넘겨 주는 것뿐이다. 처리 규칙은 한 군데에만 둔다.
//
//    /request/<종류>/<문서>       → /baro/req/new?kind=<종류>&doc=<문서>
//    /request_view/<요청 번호>    → /baro/req/view/<요청 번호>
//    /request_action/<번호>/<처리> → /baro/req/view/<번호> 에 그 처리를 넘긴다
// ============================================================

// edit_request_action_map : 짧은 주소에서 쓰던 처리 이름을 맞춰 준다
//
//	「닫기(close)」 는 사유를 받아 거절하는 것과 같은 뜻으로 다룬다.
func edit_request_action_map(action string) string {
	if action == "close" {
		return "reject"
	}

	return action
}

// register_edit_request_compat_routes : 편집 요청 짧은 주소
func register_edit_request_compat_routes(r *gin.Engine) {
	request_new := func(c *gin.Context) {
		kind := strings.ToLower(strings.TrimSpace(c.Param("kind")))
		doc_name := doc_param(c)

		target := "/baro/req/new?kind=" + tool.Url_parser(kind)
		if doc_name != "" {
			target += "&doc=" + tool.Url_parser(doc_name)
		}

		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(tool.Get_redirect(target)))
	}

	r.GET("/request/:kind/*doc_name", request_new)
	r.POST("/request/:kind/*doc_name", request_new)

	r.GET("/request_view/*id", func(c *gin.Context) {
		req_id := strings.TrimPrefix(c.Param("id"), "/")

		write_data(
			c, http.StatusOK, "text/html; charset=utf-8",
			[]byte(tool.Get_redirect("/baro/req/view/"+tool.Url_parser(req_id))),
		)
	})

	// 처리는 넘겨줄 수가 없다. (다시 보내면 두 번 처리된다)
	// 그래서 여기서 바로 같은 화면 함수를 부르고, 결과 화면도 그대로 내려보낸다.
	r.POST("/request_action/:id/:action", func(c *gin.Context) {
		_ = c.Request.ParseForm()

		req_id := strings.TrimPrefix(c.Param("id"), "/")
		values := url.Values{}
		if c.Request.PostForm != nil {
			values = c.Request.PostForm
		}
		values.Set("action", edit_request_action_map(strings.TrimPrefix(c.Param("action"), "/")))

		data := route.View_baro_req_view(make_route_config(c), req_id, values)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
}
