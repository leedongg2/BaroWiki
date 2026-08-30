package main

import (
	"net/http"
	"strings"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// baro_record_html : 통합 기여 기록 화면 응답
func baro_record_html(c *gin.Context, data string) {
	write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

// baro_record_kind_ok : 아는 탭 이름인지 확인
func baro_record_kind_ok(kind string) bool {
	switch kind {
	case "all", "edit", "topic", "req", "req_handle":
		return true
	}

	return false
}

// baro_record_split : "<사용자>/<탭>/<쪽>" 을 나눈다
//
//	사용자 이름에 / 가 들어갈 수 있으므로 뒤에서부터 떼어 낸다.
func baro_record_split(path string) (string, string, string) {
	user_name := strings.TrimPrefix(path, "/")
	kind := "all"
	page := "1"

	if index := strings.LastIndex(user_name, "/"); index > 0 {
		tail := user_name[index+1:]
		if tail != "" && strings.Trim(tail, "0123456789") == "" {
			page = tail
			user_name = user_name[:index]
		}
	}

	if index := strings.LastIndex(user_name, "/"); index > 0 {
		tail := user_name[index+1:]
		if baro_record_kind_ok(tail) {
			kind = tail
			user_name = user_name[:index]
		}
	}

	return user_name, kind, page
}

// register_baro_record_routes : 통합 기여 기록 라우트
func register_baro_record_routes(r *gin.Engine) {
	r.GET("/baro/record", func(c *gin.Context) {
		baro_record_html(c, route.View_baro_record(make_route_config(c), "", "all", "1"))
	})

	r.GET("/baro/record/user/*path", func(c *gin.Context) {
		user_name, kind, page := baro_record_split(c.Param("path"))
		baro_record_html(c, route.View_baro_record(make_route_config(c), user_name, kind, page))
	})
}
