package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  16-8 기능 플래그
//
//  baro_flag 표를 관리한다.
//  scope : all / admin / founder / percent:NN / project:<id> / off
// ============================================================

// baro_admin_flag_scope_help : scope 값 설명
func baro_admin_flag_scope_help() [][]string {
	return [][]string{
		{"all", tool.Baro_t("모두에게 켜짐"), tool.Baro_t("로그인 여부와 상관없이 모든 사람에게 기능이 보입니다.")},
		{"admin", tool.Baro_t("관리자만"), tool.Baro_t("위키 관리자 권한이 있는 사람에게만 켜집니다. 새 기능을 먼저 시험할 때 씁니다.")},
		{"founder", tool.Baro_t("창립 기여자만"), tool.Baro_t("창립 번호를 받은 사람에게만 켜집니다.")},
		{"percent:NN", tool.Baro_t("일부 이용자에게만"), tool.Baro_t("예: percent:10 이면 계정 이름을 해시로 나눠 약 10% 에게만 켜집니다. 같은 사람에게는 항상 같은 결과가 나옵니다.")},
		{"project:<번호>", tool.Baro_t("프로젝트 참여자만"), tool.Baro_t("예: project:3 이면 3번 프로젝트의 활동 중인 참여자에게만 켜집니다.")},
		{"off", tool.Baro_t("모두에게 꺼짐"), tool.Baro_t("누구에게도 보이지 않습니다. 문제가 생겼을 때 즉시 끄는 값입니다.")},
	}
}

// baro_admin_flag_scope_badge : scope 배지
func baro_admin_flag_scope_badge(scope string) string {
	switch {
	case scope == "off":
		return tool.Baro_badge(tool.Baro_t("꺼짐"), "danger")
	case scope == "all" || scope == "":
		return tool.Baro_badge(tool.Baro_t("모두"), "ok")
	case scope == "admin":
		return tool.Baro_badge(tool.Baro_t("관리자만"), "warn")
	case scope == "founder":
		return tool.Baro_badge(tool.Baro_t("창립 기여자만"), "info")
	case strings.HasPrefix(scope, "percent:"):
		return tool.Baro_badge(strings.TrimPrefix(scope, "percent:")+"% 이용자", "info")
	case strings.HasPrefix(scope, "project:"):
		return tool.Baro_badge(tool.Baro_t("프로젝트 ")+strings.TrimPrefix(scope, "project:")+tool.Baro_t(" 참여자"), "info")
	}

	return tool.Baro_badge(scope, "muted")
}

// baro_admin_flag_scope_valid : scope 값이 규칙에 맞는지
func baro_admin_flag_scope_valid(scope string) bool {
	switch scope {
	case "all", "admin", "founder", "off", "":
		return true
	}

	if strings.HasPrefix(scope, "percent:") {
		value := strings.TrimPrefix(scope, "percent:")

		return value != "" && tool.Baro_itoa(tool.Str_to_int(value)) == value
	}

	if strings.HasPrefix(scope, "project:") {
		return strings.TrimPrefix(scope, "project:") != ""
	}

	return false
}

// baro_admin_flag_post : 기능 플래그 쓰기 동작
func baro_admin_flag_post(db *sql.DB, config tool.Config, values url.Values) (string, string) {
	do := baro_admin_get(values, "do")
	key_name := baro_admin_get(values, "key_name")

	if key_name == "" {
		return "", tool.Baro_alert("warn", tool.Baro_t("플래그 열쇠말(key_name)이 없습니다."))
	}

	before := baro_admin_one(db, "select scope from baro_flag where key_name = ?", key_name)

	switch do {
	case "add", "save":
		scope := baro_admin_get(values, "scope")
		if !baro_admin_flag_scope_valid(scope) {
			return "", tool.Baro_alert("danger", tool.Baro_t("scope 값이 규칙에 맞지 않습니다. all / admin / founder / percent:NN / project:번호 / off 중에서 고르세요."))
		}

		name := baro_admin_get(values, "name")
		note := baro_admin_get(values, "note")
		value := baro_admin_get(values, "value")

		if do == "add" && before != "" {
			return "", tool.Baro_alert("warn", tool.Baro_t("같은 열쇠말의 플래그가 이미 있습니다. 수정 칸에서 바꾸세요."))
		}

		if baro_admin_one(db, "select key_name from baro_flag where key_name = ?", key_name) == "" {
			tool.Exec_DB(
				db,
				"insert into baro_flag (key_name, name, scope, value, note, date) values (?, ?, ?, ?, ?, ?)",
				key_name, name, scope, value, note, tool.Baro_now(),
			)
		} else {
			tool.Exec_DB(
				db,
				"update baro_flag set name = ?, scope = ?, value = ?, note = ?, date = ? where key_name = ?",
				name, scope, value, note, tool.Baro_now(), key_name,
			)
		}

		baro_admin_log(db, config, "flag_set", "flag:"+key_name, note, before, scope)

		return tool.Get_redirect("/baro/admin/flag?done=save"), ""

	case "off":
		tool.Exec_DB(db, "update baro_flag set scope = 'off', date = ? where key_name = ?", tool.Baro_now(), key_name)
		baro_admin_log(db, config, "flag_off", "flag:"+key_name, tool.Baro_t("즉시 끄기"), before, "off")

		return tool.Get_redirect("/baro/admin/flag?done=off"), ""

	case "delete":
		if baro_admin_get(values, "confirm") != key_name {
			return "", tool.Baro_alert("danger", tool.Baro_t("확인 칸에 지울 플래그의 열쇠말을 정확히 입력해야 합니다."))
		}

		tool.Exec_DB(db, "delete from baro_flag where key_name = ?", key_name)
		baro_admin_log(db, config, "flag_delete", "flag:"+key_name, baro_admin_get(values, "note"), before, "삭제")

		return tool.Get_redirect("/baro/admin/flag?done=delete"), ""
	}

	return "", tool.Baro_alert("warn", tool.Baro_t("알 수 없는 동작입니다."))
}

// View_baro_admin_flag : 16-8 기능 플래그
func View_baro_admin_flag(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if html, stop := baro_admin_gate(db, config, "system"); stop {
		return html
	}

	notice := ""
	if values != nil {
		redirect, message := baro_admin_flag_post(db, config, values)
		if redirect != "" {
			return redirect
		}

		notice = message
	}

	action := "/baro/admin/flag"

	body := tool.Baro_hero(
		tool.Baro_t("기능 플래그"),
		tool.Baro_t("기능을 누구에게 보여 줄지 정합니다. 문제가 생기면 scope 를 off 로 바꿔 즉시 끕니다."),
		"",
	)
	body += baro_admin_tabs("/baro/admin/flag")

	if notice != "" {
		body += notice
	}
	if done := baro_admin_get(query, "done"); done != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("처리했습니다. (")+baro_admin_esc(done)+")")
	}

	// --- scope 설명 ---
	help_rows := [][]string{}
	for _, row := range baro_admin_flag_scope_help() {
		help_rows = append(help_rows, []string{
			tool.Baro_code(row[0]),
			baro_admin_esc(row[1]),
			baro_admin_esc(row[2]),
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("scope 값 설명"),
		tool.Baro_t("플래그의 적용 범위를 정하는 값입니다."),
		tool.Baro_table([]string{tool.Baro_t("값"), tool.Baro_t("뜻"), tool.Baro_t("설명")}, help_rows, ""),
		tool.Baro_note(tool.Baro_t("percent 는 계정 이름을 해시로 나누므로 같은 사람에게는 언제나 같은 결과가 나옵니다.")),
	)

	// --- 목록 ---
	page := baro_admin_page_num(query)
	size := tool.Baro_page_size

	list := baro_admin_rows(
		db, 6,
		"select key_name, name, scope, value, note, date from baro_flag order by key_name limit ?, ?",
		(page-1)*size, size,
	)

	rows := [][]string{}
	for _, one := range list {
		mine := tool.Baro_badge(tool.Baro_t("나에게 꺼짐"), "muted")
		if tool.Baro_flag_on(db, config.IP, one[0]) {
			mine = tool.Baro_badge(tool.Baro_t("나에게 켜짐"), "ok")
		}

		off_form := tool.Baro_hidden("do", "off")
		off_form += tool.Baro_hidden("key_name", one[0])
		off_form += tool.Baro_submit(tool.Baro_t("즉시 끄기"), "danger")

		rows = append(rows, []string{
			tool.Baro_code(one[0]),
			baro_admin_esc(one[1]),
			baro_admin_flag_scope_badge(one[2]),
			mine,
			baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(one[4]), 40)),
			tool.Baro_time_cell(one[5]),
			tool.Baro_button("/baro/admin/flag?edit="+tool.Url_parser(one[0]), "수정", "primary") + " " + tool.Baro_form(action, off_form),
		})
	}

	table := tool.Baro_table(
		[]string{tool.Baro_t("열쇠말"), tool.Baro_t("이름"), tool.Baro_t("적용 범위"), tool.Baro_t("나에게"), tool.Baro_t("메모"), tool.Baro_t("바뀐 시각"), tool.Baro_t("조치")},
		rows,
		tool.Baro_t("등록된 기능 플래그가 없습니다."),
	)

	if len(rows) == 0 {
		table += tool.Baro_empty(tool.Baro_t("플래그가 없습니다. 아래에서 새로 추가하세요."))
	}

	table += tool.Baro_paging(page, len(rows), size, "/baro/admin/flag?page={}")

	body += tool.Baro_card_ex(tool.Baro_t("기능 플래그 목록"), tool.Baro_t("'나에게' 칸은 tool.Baro_flag_on 으로 지금 계정 기준을 계산한 값입니다."), table, "")

	// --- 수정 ---
	edit_key := baro_admin_get(query, "edit")
	if edit_key != "" {
		row := baro_admin_rows(db, 5, "select key_name, name, scope, value, note from baro_flag where key_name = ?", edit_key)
		if len(row) == 0 {
			body += tool.Baro_alert("warn", tool.Baro_t("그 열쇠말의 플래그가 없습니다."))
		} else {
			one := row[0]

			edit_form := tool.Baro_hidden("do", "save")
			edit_form += tool.Baro_hidden("key_name", one[0])
			edit_form += tool.Baro_input("name", tool.Baro_t("이름"), one[1], tool.Baro_t("사람이 읽는 이름"), "")
			edit_form += tool.Baro_input("scope", tool.Baro_t("적용 범위 (scope)"), one[2], "all / admin / founder / percent:10 / project:3 / off", tool.Baro_t("위 표의 값 중 하나여야 합니다."))
			edit_form += tool.Baro_input("value", tool.Baro_t("부가 값"), one[3], tool.Baro_t("필요할 때만"), tool.Baro_t("기능이 따로 읽는 값입니다."))
			edit_form += tool.Baro_textarea("note", tool.Baro_t("메모"), one[4], 3, tool.Baro_t("왜 이렇게 두는지"), "")
			edit_form += tool.Baro_submit("저장", "primary")

			delete_form := tool.Baro_hidden("do", "delete")
			delete_form += tool.Baro_hidden("key_name", one[0])
			delete_form += tool.Baro_input("confirm", "확인", "", one[0], tool.Baro_t("지울 플래그의 열쇠말을 정확히 입력하세요."))
			delete_form += tool.Baro_input("note", tool.Baro_t("삭제 사유"), "", "", "")
			delete_form += tool.Baro_submit(tool.Baro_t("플래그 삭제"), "danger")

			body += tool.Baro_card_ex(
				tool.Baro_t("플래그 수정 : ")+one[0],
				tool.Baro_t("열쇠말은 코드가 참조하므로 바꿀 수 없습니다."),
				tool.Baro_form(action, edit_form),
				"",
			)
			body += tool.Baro_danger_zone(tool.Baro_t("플래그 삭제"), tool.Baro_alert("warn", tool.Baro_t("삭제하면 코드가 이 플래그를 찾지 못해 <b>기본값(켜짐)</b> 으로 동작합니다. 끄고 싶은 것이라면 삭제 대신 scope 를 off 로 두세요."))+tool.Baro_form(action, delete_form))
		}
	}

	// --- 추가 ---
	add_form := tool.Baro_hidden("do", "add")
	add_form += tool.Baro_input("key_name", tool.Baro_t("열쇠말 (필수)"), "", tool.Baro_t("예: new_editor"), tool.Baro_t("코드에서 tool.Baro_flag_on 으로 부르는 이름입니다."))
	add_form += tool.Baro_input("name", tool.Baro_t("이름"), "", tool.Baro_t("예: 새 편집기"), "")
	add_form += tool.Baro_select("scope", tool.Baro_t("적용 범위"), [][]string{
		{"off", tool.Baro_t("off — 모두에게 꺼짐")},
		{"admin", tool.Baro_t("admin — 관리자만")},
		{"founder", tool.Baro_t("founder — 창립 기여자만")},
		{"percent:10", tool.Baro_t("percent:10 — 약 10% 이용자")},
		{"percent:50", tool.Baro_t("percent:50 — 약 50% 이용자")},
		{"all", tool.Baro_t("all — 모두에게 켜짐")},
	}, "off", tool.Baro_t("새 기능은 off 나 admin 으로 시작하는 것을 권합니다."))
	add_form += tool.Baro_input("value", tool.Baro_t("부가 값"), "", "", "")
	add_form += tool.Baro_textarea("note", tool.Baro_t("메모"), "", 3, tool.Baro_t("무엇을 켜고 끄는 스위치인지"), "")
	add_form += tool.Baro_submit(tool.Baro_t("플래그 추가"), "primary")

	body += tool.Baro_card_ex(tool.Baro_t("새 플래그 추가"), tool.Baro_t("열쇠말은 코드에서 그대로 쓰입니다."), tool.Baro_form(action, add_form), "")

	return baro_admin_page(db, config, tool.Baro_t("기능 플래그"), body)
}
