package tool

import (
	"database/sql"
	"strings"
)

// ============================================================
//  사용자 이름표 — 누르면 펼쳐지는 차림표
//
//  여태 위키에서 사용자 이름을 누르면 곧바로 사용자 문서로 갔다.
//  나무위키 토론처럼 이름을 누르면 작은 차림표가 펼쳐지고,
//  거기서 문서 · 기여 · 토론 · 차단 기록으로 갈라져 들어가게 한다.
//
//  바로위키에만 있는 것도 함께 보여 준다.
//    · 레벨
//    · #창립 번호
//    · 바로샵에서 산 칭호
//
//  자바스크립트가 막혀 있어도 이름이 사라지지 않게 <details> 로 만든다.
// ============================================================

// Baro_user_tag : 사용자 이름표 (누르면 차림표가 펼쳐진다)
func Baro_user_tag(db *sql.DB, user string, my_ip string) string {
	user = strings.TrimSpace(user)
	if user == "" {
		return ""
	}

	is_ip := IP_or_user(user)

	// 보여 줄 이름은 위키가 쓰던 규칙을 그대로 따른다. (숨김 · 차단 표시까지)
	shown := IP_parser(db, user, my_ip)
	if shown == "" {
		shown = HTML_escape(user)
	}

	// 링크가 겹치지 않게 겉의 <a> 만 벗긴다.
	shown = baro_user_tag_unlink(shown)

	mark := ""
	if !is_ip {
		if title := Baro_mik_title_user_html(db, user); title != "" {
			mark += " " + title
		}

		if number := Baro_founder_number(db, user); number != "" {
			mark += ` <span class="bw_utag_no">#` + HTML_escape(number) + `</span>`
		}

		level := Get_level(db, user)
		if len(level) > 0 && level[0] != "" {
			mark += ` <sup class="bw_utag_lv">` + HTML_escape(level[0]) + `</sup>`
		}
	}

	html := `<details class="bw_utag"><summary class="bw_utag_s">` + shown + mark + `</summary>`
	html += `<div class="bw_utag_menu">` + baro_user_tag_menu(db, user, my_ip, is_ip) + `</div>`
	html += `</details>`

	return html + baro_user_tag_css()
}

// baro_user_tag_menu : 펼쳤을 때 나오는 줄들
func baro_user_tag_menu(db *sql.DB, user string, my_ip string, is_ip bool) string {
	name := Url_parser(user)

	doc := "user:" + user
	if is_ip {
		doc = "ipuser:" + user
	}

	items := [][]string{
		{"/w/" + Doc_url(doc), Baro_t("사용자 문서")},
		{"/record/" + name, Baro_t("문서 기여")},
		{"/record/topic/" + name, Baro_t("토론 기여")},
		{"/record/bbs/" + name, Baro_t("게시판 기여")},
		{"/baro/record/user/" + name + "/all", Baro_t("통합 기여 기록")},
		{"/recent_block/user/" + name, Baro_t("차단 기록")},
	}

	if !is_ip {
		items = append(items,
			[]string{"/baro/profile/" + name, Baro_t("프로필")},
			[]string{"/baro/land/user/" + name, Baro_t("기여 땅따먹기")},
		)
	}

	// 운영 권한이 있으면 조치로 바로 갈 수 있게 한다.
	if Check_acl(db, "", "", "ban_auth", my_ip) {
		items = append(items, []string{"/auth/ban/" + name, Baro_t("차단")})

		if !is_ip {
			items = append(items, []string{"/auth/warn/" + name, Baro_t("경고")})
		}
	}

	if !is_ip && Check_acl(db, "", "", "give_auth", my_ip) {
		items = append(items, []string{"/auth/user/" + name, Baro_t("권한")})
	}

	html := `<ul>`
	for _, one := range items {
		html += `<li><a href="` + HTML_escape(one[0]) + `">` + Baro_t_esc(one[1]) + `</a></li>`
	}

	return html + `</ul>`
}

// baro_user_tag_unlink : 겉을 감싼 <a> 한 겹만 벗긴다
//
//	IP_parser 가 붙여 준 <sup> 이나 <s> 같은 꾸밈은 그대로 둔다.
func baro_user_tag_unlink(data string) string {
	for {
		start := strings.Index(data, "<a ")
		if start < 0 {
			break
		}

		close_tag := strings.Index(data[start:], ">")
		if close_tag < 0 {
			break
		}

		end := strings.LastIndex(data, "</a>")
		if end < 0 || end < start {
			break
		}

		data = data[:start] + data[start+close_tag+1:end] + data[end+len("</a>"):]
	}

	return data
}

// baro_user_tag_css : 이름표 모양 (한 쪽에 한 번만 나가면 된다)
func baro_user_tag_css() string {
	return `<style id="bw_utag_css">
.bw_utag { display: inline-block; position: relative; }
.bw_utag > summary { display: inline; cursor: pointer; list-style: none; }
.bw_utag > summary::-webkit-details-marker { display: none; }
.bw_utag_no { font-size: 11px; opacity: 0.8; }
.bw_utag_lv { font-size: 10px; opacity: 0.7; }
.bw_utag_menu { position: absolute; z-index: 30; min-width: 11rem; margin-top: 4px;
    border: 1px solid var(--namu-card-border-color, #ddd);
    background: var(--namu-card-background-color, #fff);
    border-radius: 6px; padding: 4px 0; box-shadow: 0 4px 14px rgba(0,0,0,0.14); }
.bw_utag_menu ul { list-style: none; margin: 0; padding: 0; }
.bw_utag_menu li { margin: 0; }
.bw_utag_menu a { display: block; padding: 5px 12px; font-size: 13px; text-decoration: none; }
.bw_utag_menu a:hover { background: var(--namu-button-background-hover-color, #efefef); }
</style>`
}
