package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

// ============================================================
//  연결된 계정 — /change/social
//
//  유튜브 · 인스타그램 · 네이버 블로그 · 티스토리처럼
//  위키 밖에서 활동하는 자리를 사용자 문서와 프로필에 걸어 둔다.
//
//  적어 둔 계정은
//    - 사용자 문서 (`/w/user:이름`) 위쪽
//    - 바로위키 프로필 (`/baro/profile/이름`)
//  두 곳에 같은 모양으로 나온다.
//
//  본문에서는 `[계정(유튜브, @아이디)]` 문법으로 아무 데나 넣을 수 있다.
//  여기서 저장한 계정도 그 문법과 같은 모양으로 그려진다.
// ============================================================

// user_social_field : 화면에 보여 줄 계정 종류 (열쇠말, 이름, 도움말)
func user_social_field() [][3]string {
	return [][3]string{
		{"youtube", tool.Baro_t("유튜브"), tool.Baro_t("@손잡이 또는 채널 주소")},
		{"instagram", tool.Baro_t("인스타그램"), tool.Baro_t("아이디 또는 프로필 주소")},
		{"naverblog", tool.Baro_t("네이버 블로그"), tool.Baro_t("블로그 아이디 또는 주소")},
		{"tistory", tool.Baro_t("티스토리"), tool.Baro_t("블로그 이름 또는 주소")},
		{"twitter", "X", tool.Baro_t("아이디 또는 프로필 주소")},
		{"github", "GitHub", tool.Baro_t("아이디 또는 프로필 주소")},
		{"chzzk", tool.Baro_t("치지직"), tool.Baro_t("채널 아이디 또는 주소")},
	}
}

// user_social_key : baro_user_set 에 넣을 이름
func user_social_key(kind string) string {
	return "social_" + kind
}

// User_social_get : 사용자가 적어 둔 계정 목록 (열쇠말 → 아이디)
func user_social_get(db *sql.DB, user string) map[string]string {
	out := map[string]string{}
	for _, one := range user_social_field() {
		if value := baro_account_uset_get(db, user, user_social_key(one[0])); value != "" {
			out[one[0]] = value
		}
	}

	return out
}

// user_social_html : 계정 상자 줄 (없으면 빈 글)
//
//	본문에서 쓰는 `[계정(…)]` 문법과 같은 모양으로 그린다.
func user_social_html(db *sql.DB, user string) string {
	accounts := user_social_get(db, user)
	if len(accounts) == 0 {
		return ""
	}

	html := ""
	for _, one := range user_social_field() {
		value := accounts[one[0]]
		if value == "" {
			continue
		}

		html += markup.Markup_account_html(one[0], value)
	}

	if html == "" {
		return ""
	}

	return `<div class="namu_social_row">` + html + `</div>`
}

// View_user_social : 연결된 계정 설정
func View_user_social(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if tool.IP_or_user(config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	message := ""
	if values != nil {
		saved := 0
		dropped := []string{}

		for _, one := range user_social_field() {
			raw := strings.TrimSpace(values.Get(one[0]))
			if raw == "" {
				baro_account_uset_put(db, config.IP, user_social_key(one[0]), "")
				tool.Exec_DB(db, "delete from baro_user_set where user = ? and name = ?", config.IP, user_social_key(one[0]))

				continue
			}

			clean := markup.Markup_account_id(one[0], raw)
			if clean == "" {
				dropped = append(dropped, one[1])

				continue
			}

			baro_account_uset_put(db, config.IP, user_social_key(one[0]), clean)
			saved++
		}

		if len(dropped) > 0 {
			message = tool.Baro_alert("warn",
				tool.HTML_escape(strings.Join(dropped, ", "))+tool.Baro_t(" 은 아이디 모양이 아니어서 저장하지 않았습니다. 아이디나 프로필 주소를 그대로 넣어 주세요."))
		} else {
			message = tool.Baro_alert("ok", tool.Baro_t("연결된 계정을 저장했습니다."))
		}
	}

	accounts := user_social_get(db, config.IP)

	form := ""
	for _, one := range user_social_field() {
		form += tool.Baro_input(one[0], one[1], accounts[one[0]], one[2], "")
	}
	form += tool.Baro_submit(tool.Get_language(db, "save", true), "primary")

	body := tool.Baro_hero(
		tool.Baro_t("연결된 계정"),
		tool.Baro_t("여기에 적어 두면 사용자 문서와 프로필에 함께 보입니다. 비우고 저장하면 지워집니다."),
		"",
	)

	inner := message + tool.Baro_form("/change/social", form)
	body += tool.Baro_card("", inner)

	if preview := user_social_html(db, config.IP); preview != "" {
		body += tool.Baro_card(tool.Baro_t("이렇게 보입니다"), preview)
	}

	body += tool.Baro_card("", tool.Baro_note("본문에서는 [계정(유튜브, @아이디)] 처럼 적어 아무 데나 넣을 수 있습니다.")+
		tool.Baro_button("/w/"+tool.Url_parser("user:"+config.IP), tool.Baro_t("내 사용자 문서"), "")+" "+
		tool.Baro_button("/change", tool.Get_language(db, "user_setting", true), ""))

	return tool.Baro_page(db, config, tool.Baro_t("연결된 계정"), body, [][]any{
		{"change", tool.Get_language(db, "user_setting", true)},
	})
}
