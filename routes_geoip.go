package main

import (
	"database/sql"
	"strings"

	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

// ============================================================
//  앞단이 알려 주는 나라 받기
//
//  Cloudflare 같은 앞단은 요청에 `CF-IPCountry: KR` 처럼 나라를 붙여 준다.
//  위키가 앞단 뒤에 있을 때는 이게 가장 정확하다.
//
//  다만 위키가 앞단 없이 바로 열려 있으면 이용자가 이 머리글을 마음대로 지어 보낼 수 있다.
//  그래서 **설정에 머리글 이름을 적어 둘 때만** 읽는다. (기본은 꺼져 있다)
//  설정 : 위키 설정의 `geoip_header` 에 `CF-IPCountry` 처럼 적는다.
// ============================================================

// geoip_header_read : 요청에 붙은 나라를 받아 적어 둔다
func geoip_header_read(c *gin.Context, db *sql.DB) {
	header_name := strings.TrimSpace(tool.Wiki_setting(db, tool.Acl_geoip_header_setting))
	if header_name == "" {
		return
	}

	country := strings.TrimSpace(c.GetHeader(header_name))
	if len(country) != 2 {
		// Cloudflare 는 알 수 없을 때 `XX` 를 보낸다. 그런 값은 버린다.
		return
	}

	country = strings.ToUpper(country)
	if country == "XX" || country == "T1" {
		return
	}

	tool.Acl_geoip_remember(tool.Get_IP(c), country)
}
