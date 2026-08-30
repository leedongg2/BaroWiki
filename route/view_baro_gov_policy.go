package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  11-2 정책 문서 버전 관리  (/baro/policy)
//
//  정책도 일반 문서처럼 역사를 남긴다.
//  기존 판은 status = old 로 두고, 새 줄을 version + 1 로 넣는다.
//  변경 사유와 시행일은 반드시 받는다.
// ============================================================

// baro_gov_policy_row : 정책 한 판
type baro_gov_policy_row struct {
	ID       string
	Key      string
	Title    string
	Version  string
	Data     string
	Eff      string
	Proposer string
	Reason   string
	Talk     string
	Date     string
	Status   string
}

// baro_gov_policy_get : 정책 한 판 읽기 (version 이 "" 면 현재 판)
func baro_gov_policy_get(db *sql.DB, key string, version string) (baro_gov_policy_row, bool) {
	row := baro_gov_policy_row{Key: key}

	query := "select id, key_name, title, version, data, effective_date, proposer, reason, discussion, date, status from baro_policy where key_name = ? and status = 'active' order by version + 0 desc limit 1"
	args := []any{key}

	if version != "" {
		query = "select id, key_name, title, version, data, effective_date, proposer, reason, discussion, date, status from baro_policy where key_name = ? and version = ? limit 1"
		args = []any{key, version}
	}

	found := tool.QueryRow_DB(
		db, query,
		[]any{&row.ID, &row.Key, &row.Title, &row.Version, &row.Data, &row.Eff, &row.Proposer, &row.Reason, &row.Talk, &row.Date, &row.Status},
		args...,
	)

	if !found && version == "" {
		// active 표시가 없는 정책이라면 가장 높은 판을 현재 판으로 본다
		found = tool.QueryRow_DB(
			db,
			"select id, key_name, title, version, data, effective_date, proposer, reason, discussion, date, status from baro_policy where key_name = ? order by version + 0 desc limit 1",
			[]any{&row.ID, &row.Key, &row.Title, &row.Version, &row.Data, &row.Eff, &row.Proposer, &row.Reason, &row.Talk, &row.Date, &row.Status},
			key,
		)
	}

	return row, found
}

// baro_gov_policy_versions : 정책의 판 목록 (최신 판이 먼저)
func baro_gov_policy_versions(db *sql.DB, key string) []baro_gov_policy_row {
	out := []baro_gov_policy_row{}

	rows := tool.Query_DB(
		db,
		"select id, key_name, title, version, effective_date, proposer, reason, discussion, date, status from baro_policy where key_name = ? order by version + 0 desc",
		key,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_gov_policy_row{}
		if err := rows.Scan(&row.ID, &row.Key, &row.Title, &row.Version, &row.Eff, &row.Proposer, &row.Reason, &row.Talk, &row.Date, &row.Status); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_gov_policy_max_version : 정책의 가장 큰 판 번호
func baro_gov_policy_max_version(db *sql.DB, key string) int {
	last := "0"
	tool.QueryRow_DB(db, "select version from baro_policy where key_name = ? order by version + 0 desc limit 1", []any{&last}, key)

	return tool.Str_to_int(last)
}

// baro_gov_policy_talk_html : 관련 토론 링크 (주소가 아니면 그대로 글자로 보여 준다)
func baro_gov_policy_talk_html(talk string) string {
	talk = strings.TrimSpace(talk)
	if talk == "" {
		return tool.Baro_t(`<span class="bw_muted">연결된 토론이 없습니다.</span>`)
	}

	if strings.HasPrefix(talk, "/") || strings.HasPrefix(talk, "http://") || strings.HasPrefix(talk, "https://") {
		return `<a href="` + tool.HTML_escape(talk) + `" rel="nofollow">` + baro_gov_safe(tool.Baro_trunc(talk, 90)) + `</a>`
	}

	return tool.Baro_doc_link(talk)
}

// baro_gov_policy_status_badge : 판 상태 배지
func baro_gov_policy_status_badge(status string) string {
	switch status {
	case "active":
		return tool.Baro_badge(tool.Baro_t("현재 판"), "ok")
	case "old":
		return tool.Baro_badge(tool.Baro_t("지난 판"), "muted")
	case "pending":
		return tool.Baro_badge(tool.Baro_t("시행 대기"), "warn")
	}

	return tool.Baro_badge(status, "muted")
}

// baro_gov_policy_notify_staff : 정책이 바뀌었음을 운영 역할자에게 알린다
func baro_gov_policy_notify_staff(db *sql.DB, key string, title string, version string, actor string) {
	rows := tool.Query_DB(db, "select distinct user from baro_role")
	defer rows.Close()

	users := []string{}
	for rows.Next() {
		user := ""
		if err := rows.Scan(&user); err != nil {
			continue
		}

		if user == "" || user == actor {
			continue
		}

		users = append(users, user)
	}

	for _, user := range users {
		tool.Baro_notify(
			db, user, "system",
			"정책 "+tool.Baro_trunc(title, 40)+tool.Baro_t(" 이(가) ")+version+tool.Baro_t("판으로 바뀌었습니다."),
			"/baro/policy/view/"+tool.Url_parser(key),
			"",
		)
	}
}

// View_baro_gov_policy_list : 정책 목록
func View_baro_gov_policy_list(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	word := baro_gov_q(query, "q")

	where := " where status = 'active'"
	args := []any{}

	if word != "" {
		where += " and (title like ? or key_name like ? or data like ?)"
		like := "%" + word + "%"
		args = append(args, like, like, like)
	}

	page := baro_gov_page_num(baro_gov_q(query, "page"))
	size := tool.Baro_page_size

	total := 0
	tool.QueryRow_DB(db, "select count(*) from baro_policy"+where, []any{&total}, args...)

	list_args := append(append([]any{}, args...), (page-1)*size, size)

	rows := tool.Query_DB(
		db,
		"select key_name, title, version, effective_date, proposer, reason, date from baro_policy"+where+" order by key_name asc limit ?, ?",
		list_args...,
	)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0

	for rows.Next() {
		key, title, version, eff, proposer, reason, date := "", "", "", "", "", "", ""
		if err := rows.Scan(&key, &title, &version, &eff, &proposer, &reason, &date); err != nil {
			continue
		}

		count++

		total_version := baro_gov_policy_max_version(db, key)

		name_html := `<a href="/baro/policy/view/` + tool.Url_parser(key) + `"><b>` + baro_gov_safe(title) + `</b></a>`
		name_html += `<div class="bw_note">` + tool.HTML_escape(key) + `</div>`

		table_rows = append(table_rows, []string{
			name_html,
			tool.Baro_badge(version+tool.Baro_t("판"), "info"),
			tool.HTML_escape(eff),
			baro_gov_actor_html(db, proposer, config.IP),
			baro_gov_safe(tool.Baro_trunc(tool.Baro_one_line(reason), 40)),
			tool.Baro_itoa(total_version) + tool.Baro_t("개"),
		})
	}

	can_edit := tool.Baro_can(db, config.IP, "policy")

	body := baro_gov_css()
	body += tool.Baro_hero(tool.Baro_t("정책 문서"), tool.Baro_t("바로위키의 정책은 일반 문서처럼 판 번호와 역사를 가집니다."), "")
	body += baro_gov_tabs("/baro/policy")

	body += tool.Baro_alert("info",
		tool.Baro_t(`정책이 바뀌면 <b>이전 판이 사라지지 않고 남습니다</b>. 각 판에는 시행일 · 제안자 · 변경 사유 · 관련 토론이 함께 기록되며, 판끼리 줄 단위로 비교할 수 있습니다.`),
	)

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("현행 정책"), tool.Baro_comma(baro_gov_count(db, "select count(*) from baro_policy where status = 'active'")), tool.Baro_t("지금 적용되는 판")},
		{tool.Baro_t("전체 판"), tool.Baro_comma(baro_gov_count(db, "select count(*) from baro_policy")), tool.Baro_t("지난 판 포함")},
		{tool.Baro_t("최근 변경"), baro_gov_policy_last_change(db), tool.Baro_t("가장 최근 개정")},
	})

	filter := `<div class="bw_gov_filter">`
	filter += tool.Baro_input("q", tool.Baro_t("정책 찾기"), word, tool.Baro_t("제목 · 키 · 본문"), "")
	filter += `<div class="bw_gov_filter_btn">` + tool.Baro_submit(tool.Baro_t("찾기"), "primary") + ` ` + tool.Baro_button("/baro/policy", tool.Baro_t("조건 지우기"), "") + `</div>`
	filter += `</div>`

	body += tool.Baro_card(tool.Baro_t("거르기"), baro_gov_get_form("/baro/policy", filter))

	list_body := ""
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("조건에 맞는 정책이 없습니다."))
	} else {
		list_body = tool.Baro_table(
			[]string{"정책", tool.Baro_t("현재 판"), "시행일", tool.Baro_t("제안자"), tool.Baro_t("최근 변경 사유"), tool.Baro_t("판 수")},
			table_rows,
			tool.Baro_t("등록된 정책이 없습니다."),
		)
	}

	list_body += tool.Baro_paging(page, count, size, baro_gov_url("/baro/policy", [][]string{
		{"q", word}, {"page", "{}"},
	}))

	foot := ""
	if can_edit {
		foot = tool.Baro_button("/baro/policy/new", tool.Baro_t("새 정책 제정"), "primary")
	} else {
		foot = tool.Baro_note(tool.Baro_t("정책 제정과 개정은 정책 권한(시스템 관리자 · 중재자)을 가진 사람만 할 수 있습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("현행 정책"), tool.Baro_t("조건에 맞는 정책 ")+tool.Baro_comma(total)+tool.Baro_t("건"), list_body, foot)

	return tool.Baro_page(db, config, tool.Baro_t("정책 문서"), body, baro_gov_menu())
}

// baro_gov_policy_last_change : 가장 최근 정책 변경 시각 표시
func baro_gov_policy_last_change(db *sql.DB) string {
	date := ""
	tool.QueryRow_DB(db, "select date from baro_policy order by date desc limit 1", []any{&date})

	if date == "" {
		return "-"
	}

	return tool.Baro_time_ago(date)
}

// View_baro_gov_policy_view : 정책 본문 + 판 목록
func View_baro_gov_policy_view(config tool.Config, key string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	key = strings.TrimSpace(key)

	row, found := baro_gov_policy_get(db, key, "")
	if !found {
		body := baro_gov_css()
		body += tool.Baro_crumb([][]string{{"/baro/policy", tool.Baro_t("정책 문서")}, {"", tool.Baro_t("찾을 수 없음")}})
		body += tool.Baro_alert("warn", tool.Baro_t("그런 이름의 정책이 없습니다. 주소를 다시 확인해 주세요."))
		body += tool.Baro_card(tool.Baro_t("정책 찾기"),
			tool.Baro_empty(tool.Baro_t("키 ")+tool.Baro_trunc(key, 40)+tool.Baro_t(" 에 해당하는 정책이 없습니다."))+
				tool.Baro_button("/baro/policy", tool.Baro_t("정책 목록으로"), "primary"),
		)

		return tool.Baro_page(db, config, "정책", body, baro_gov_menu())
	}

	versions := baro_gov_policy_versions(db, key)
	can_edit := tool.Baro_can(db, config.IP, "policy")

	body := baro_gov_css()
	body += tool.Baro_crumb([][]string{{"/baro/policy", tool.Baro_t("정책 문서")}, {"", row.Title}})
	body += tool.Baro_hero(row.Title, tool.Baro_t("정책 키 ")+row.Key+tool.Baro_t(" · 현재 ")+row.Version+tool.Baro_t("판"), "")
	body += baro_gov_tabs("/baro/policy")

	if strings.TrimSpace(row.Eff) == "" {
		body += tool.Baro_alert("warn", tool.Baro_t("이 판에는 시행일이 적혀 있지 않습니다. 지금은 현재 판으로 적용되고 있지만, 다음 개정 때 시행일을 반드시 적어 주세요."))
	} else if tool.Baro_is_past(row.Eff) {
		body += tool.Baro_alert("ok", tool.Baro_t("이 판은 <b>")+tool.HTML_escape(row.Eff)+tool.Baro_t("</b> 부터 시행 중입니다."))
	} else {
		body += tool.Baro_alert("warn", tool.Baro_t("이 판은 <b>")+tool.HTML_escape(row.Eff)+tool.Baro_t("</b> 부터 시행됩니다. 아직 시행 전입니다."))
	}

	body += tool.Baro_card(tool.Baro_t("이 판의 정보"), tool.Baro_kv_grid([][]string{
		{tool.Baro_t("판 번호"), tool.Baro_badge(row.Version+tool.Baro_t("판"), "info")},
		{"상태", baro_gov_policy_status_badge(row.Status)},
		{"시행일", tool.HTML_escape(row.Eff)},
		{tool.Baro_t("제안자"), baro_gov_actor_html(db, row.Proposer, config.IP)},
		{tool.Baro_t("기록 시각"), tool.HTML_escape(row.Date)},
		{tool.Baro_t("관련 토론"), baro_gov_policy_talk_html(row.Talk)},
	}))

	reason_html := tool.Baro_t(`<span class="bw_muted">변경 사유가 기록되지 않았습니다.</span>`)
	if strings.TrimSpace(row.Reason) != "" {
		reason_html = baro_gov_pre(row.Reason)
	}

	body += tool.Baro_card(tool.Baro_t("변경 사유"), reason_html)
	body += tool.Baro_card(tool.Baro_t("정책 본문"), baro_gov_pre(row.Data))

	// 판 목록
	table_rows := [][]string{}
	for i, one := range versions {
		diff_html := `<span class="bw_muted">-</span>`
		if i+1 < len(versions) {
			prev := versions[i+1]
			diff_html = tool.Baro_button(
				"/baro/policy/diff/"+tool.Url_parser(key)+"/"+tool.Url_parser(prev.Version)+"/"+tool.Url_parser(one.Version),
				prev.Version+tool.Baro_t("판과 비교"), "",
			)
		}

		table_rows = append(table_rows, []string{
			tool.Baro_badge(one.Version+tool.Baro_t("판"), "info"),
			baro_gov_policy_status_badge(one.Status),
			tool.HTML_escape(one.Eff),
			baro_gov_actor_html(db, one.Proposer, config.IP),
			baro_gov_safe(tool.Baro_trunc(tool.Baro_one_line(one.Reason), 50)),
			tool.Baro_time_cell(one.Date),
			diff_html,
		})
	}

	list_body := tool.Baro_table(
		[]string{"판", "상태", "시행일", tool.Baro_t("제안자"), tool.Baro_t("변경 사유"), tool.Baro_t("기록"), tool.Baro_t("비교")},
		table_rows,
		tool.Baro_t("판 기록이 없습니다."),
	)

	// 임의의 두 판 비교 폼
	options := [][]string{}
	for _, one := range versions {
		options = append(options, []string{one.Version, one.Version + tool.Baro_t("판 (") + one.Eff + ")"})
	}

	compare := ""
	if len(options) >= 2 {
		compare = `<div class="bw_gov_filter">`
		compare += tool.Baro_select("v1", tool.Baro_t("이전 판"), options, versions[len(versions)-1].Version, "")
		compare += tool.Baro_select("v2", tool.Baro_t("새 판"), options, versions[0].Version, "")
		compare += `<div class="bw_gov_filter_btn">` + tool.Baro_submit(tool.Baro_t("두 판 비교"), "") + `</div>`
		compare += `</div>`
		compare = baro_gov_get_form("/baro/policy/diff/"+tool.Url_parser(key), compare)
	}

	body += tool.Baro_card_ex(tool.Baro_t("판 목록"), "판 "+tool.Baro_itoa(len(versions))+tool.Baro_t("개"), list_body+compare, "")

	foot := ""
	if can_edit {
		foot = tool.Baro_button("/baro/policy/edit/"+tool.Url_parser(key), "새 판 작성", "primary")
	} else {
		foot = tool.Baro_note(tool.Baro_t("정책 개정은 정책 권한을 가진 사람만 할 수 있습니다. 의견이 있으면 관련 토론에 남겨 주세요."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("정책 고치기"), tool.Baro_t("새 판을 만들면 지금 판은 지난 판으로 남습니다."), tool.Baro_note(tool.Baro_t("이전 판은 삭제되지 않습니다. 언제든 되짚어 볼 수 있습니다.")), foot)

	return tool.Baro_page(db, config, row.Title, body, baro_gov_menu())
}

// View_baro_gov_policy_diff : 두 판 비교
func View_baro_gov_policy_diff(config tool.Config, key string, v1 string, v2 string, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	key = strings.TrimSpace(key)

	if v1 == "" {
		v1 = baro_gov_q(query, "v1")
	}
	if v2 == "" {
		v2 = baro_gov_q(query, "v2")
	}

	old_row, old_found := baro_gov_policy_get(db, key, v1)
	new_row, new_found := baro_gov_policy_get(db, key, v2)

	body := baro_gov_css()
	body += tool.Baro_crumb([][]string{
		{"/baro/policy", tool.Baro_t("정책 문서")},
		{"/baro/policy/view/" + tool.Url_parser(key), key},
		{"", tool.Baro_t("판 비교")},
	})

	if !old_found || !new_found {
		body += tool.Baro_alert("warn", tool.Baro_t("비교할 판을 찾지 못했습니다. 판 번호를 다시 확인해 주세요."))
		body += tool.Baro_card(tool.Baro_t("판 비교"),
			tool.Baro_empty("정책 "+tool.Baro_trunc(key, 40)+tool.Baro_t(" 의 ")+tool.Baro_trunc(v1, 10)+tool.Baro_t("판 또는 ")+tool.Baro_trunc(v2, 10)+tool.Baro_t("판이 없습니다."))+
				tool.Baro_button("/baro/policy/view/"+tool.Url_parser(key), "정책으로 돌아가기", "primary"),
		)

		return tool.Baro_page(db, config, tool.Baro_t("판 비교"), body, baro_gov_menu())
	}

	diff := baro_gov_diff_lines(old_row.Data, new_row.Data)
	add, del := baro_gov_diff_stat(diff)

	body += tool.Baro_hero(new_row.Title+tool.Baro_t(" 판 비교"), old_row.Version+tool.Baro_t("판 → ")+new_row.Version+tool.Baro_t("판"), "")
	body += baro_gov_tabs("/baro/policy")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("추가된 줄"), tool.Baro_comma(add), tool.Baro_t("새 판에만 있는 줄")},
		{tool.Baro_t("삭제된 줄"), tool.Baro_comma(del), tool.Baro_t("이전 판에만 있던 줄")},
		{tool.Baro_t("이전 판 시행일"), old_row.Eff, old_row.Version + tool.Baro_t("판")},
		{tool.Baro_t("새 판 시행일"), new_row.Eff, new_row.Version + tool.Baro_t("판")},
	})

	body += `<div class="bw_gov_split">`
	body += tool.Baro_card(old_row.Version+tool.Baro_t("판 정보"), tool.Baro_dl([][]string{
		{"시행일", tool.HTML_escape(old_row.Eff)},
		{tool.Baro_t("제안자"), baro_gov_actor_html(db, old_row.Proposer, config.IP)},
		{tool.Baro_t("변경 사유"), baro_gov_safe(tool.Baro_trunc(old_row.Reason, 300))},
		{tool.Baro_t("관련 토론"), baro_gov_policy_talk_html(old_row.Talk)},
	}))
	body += tool.Baro_card(new_row.Version+tool.Baro_t("판 정보"), tool.Baro_dl([][]string{
		{"시행일", tool.HTML_escape(new_row.Eff)},
		{tool.Baro_t("제안자"), baro_gov_actor_html(db, new_row.Proposer, config.IP)},
		{tool.Baro_t("변경 사유"), baro_gov_safe(tool.Baro_trunc(new_row.Reason, 300))},
		{tool.Baro_t("관련 토론"), baro_gov_policy_talk_html(new_row.Talk)},
	}))
	body += `</div>`

	legend := tool.Baro_badge(tool.Baro_t("+ 추가된 줄"), "ok") + " " + tool.Baro_badge(tool.Baro_t("- 삭제된 줄"), "danger") + " " + tool.Baro_badge(tool.Baro_t("변경 없음"), "muted")

	body += tool.Baro_card_ex("본문 비교", "줄 단위로 견줍니다.", legend+baro_gov_diff_html(diff), tool.Baro_button("/baro/policy/view/"+tool.Url_parser(key), "정책으로 돌아가기", ""))

	return tool.Baro_page(db, config, tool.Baro_t("판 비교"), body, baro_gov_menu())
}

// baro_gov_policy_form : 정책 입력 폼 본문
func baro_gov_policy_form(action string, key string, title string, data string, reason string, eff string, talk string, is_new bool) string {
	form := ""

	if is_new {
		form += tool.Baro_input("key_name", tool.Baro_t("정책 키"), key, tool.Baro_t("sanction, quarantine 처럼 영소문자·숫자·밑줄"), tool.Baro_t("주소에 쓰입니다. 나중에 바꿀 수 없습니다."))
	} else {
		form += tool.Baro_hidden("key_name", key)
		form += tool.Baro_field_row_html(tool.Baro_t("정책 키"), `<div class="bw_code">`+tool.HTML_escape(key)+`</div>`, tool.Baro_t("정책 키는 바꿀 수 없습니다."))
	}

	form += tool.Baro_input("title", tool.Baro_t("정책 제목"), title, tool.Baro_t("예) 제재 규정"), "")
	form += tool.Baro_textarea("data", tool.Baro_t("정책 본문"), data, 18, tool.Baro_t("정책 내용을 적습니다."), tool.Baro_t("여러 줄로 적어도 됩니다. 판을 비교할 때 줄 단위로 견줍니다."))
	form += tool.Baro_textarea("reason", tool.Baro_t("변경 사유 (필수)"), reason, 4, tool.Baro_t("무엇을 왜 바꾸는지 적습니다."), tool.Baro_t("사유 없이 정책을 바꿀 수 없습니다."))
	form += tool.Baro_input_type("effective_date", tool.Baro_t("시행일 (필수)"), eff, "date", tool.Baro_t("이 판이 언제부터 적용되는지 밝힙니다."))
	form += tool.Baro_input("discussion", tool.Baro_t("관련 토론"), talk, "/thread/12 또는 문서 이름", tool.Baro_t("이 개정을 논의한 곳을 적어 두면 나중에 근거를 찾기 쉽습니다."))

	form += tool.Baro_submit("저장", "primary")

	return tool.Baro_form(action, form)
}

// View_baro_gov_policy_edit : 정책 새 판 작성
func View_baro_gov_policy_edit(config tool.Config, key string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	key = strings.TrimSpace(key)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/policy/edit/"+key); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "policy") {
		return tool.Baro_deny(db, config, tool.Baro_t("정책을 고치려면 정책 권한(시스템 관리자 · 중재자)이 필요합니다."))
	}

	row, found := baro_gov_policy_get(db, key, "")
	if !found {
		body := baro_gov_css()
		body += tool.Baro_alert("warn", tool.Baro_t("고칠 정책을 찾지 못했습니다."))
		body += tool.Baro_card(tool.Baro_t("정책 개정"),
			tool.Baro_empty(tool.Baro_t("키 ")+tool.Baro_trunc(key, 40)+tool.Baro_t(" 에 해당하는 정책이 없습니다."))+
				tool.Baro_button("/baro/policy/new", tool.Baro_t("새 정책 제정하기"), "primary")+" "+
				tool.Baro_button("/baro/policy", tool.Baro_t("정책 목록으로"), ""),
		)

		return tool.Baro_page(db, config, tool.Baro_t("정책 개정"), body, baro_gov_menu())
	}

	title := row.Title
	data := row.Data
	reason := ""
	eff := tool.Baro_today()
	talk := row.Talk
	alert := ""

	if values != nil {
		title = strings.TrimSpace(values.Get("title"))
		data = values.Get("data")
		reason = strings.TrimSpace(values.Get("reason"))
		eff = strings.TrimSpace(values.Get("effective_date"))
		talk = strings.TrimSpace(values.Get("discussion"))

		switch {
		case tool.Baro_read_only(db):
			alert = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.HTML_escape(tool.Baro_read_only_reason(db)))
		case title == "":
			alert = tool.Baro_alert("danger", tool.Baro_t("정책 제목을 적어 주세요."))
		case strings.TrimSpace(data) == "":
			alert = tool.Baro_alert("danger", tool.Baro_t("정책 본문이 비어 있습니다."))
		case reason == "":
			alert = tool.Baro_alert("danger", tool.Baro_t("변경 사유는 반드시 적어야 합니다. 왜 바꾸는지 남기지 않은 개정은 받지 않습니다."))
		case len(eff) != 10:
			alert = tool.Baro_alert("danger", tool.Baro_t("시행일을 날짜로 골라 주세요."))
		case data == row.Data && title == row.Title:
			alert = tool.Baro_alert("warn", tool.Baro_t("본문과 제목이 이전 판과 같습니다. 바뀐 것이 없으면 새 판을 만들지 않습니다."))
		default:
			next := tool.Baro_itoa(baro_gov_policy_max_version(db, key) + 1)

			tool.Exec_DB(db, "update baro_policy set status = 'old' where key_name = ? and status = 'active'", key)
			tool.Exec_DB(
				db,
				"insert into baro_policy (id, key_name, title, version, data, effective_date, proposer, reason, discussion, date, status) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active')",
				tool.Baro_next_id(db, "baro_policy"), key, title, next, data, eff, config.IP, reason, talk, tool.Baro_now(),
			)

			tool.Baro_public_log(db, "policy", config.IP, key, tool.Baro_trunc(reason, 300), tool.Baro_json_enc(map[string]string{
				"정책":  title,
				"판":   next + "판",
				"시행일": eff,
			}))
			tool.Baro_audit(db, config, "policy_edit", key, tool.Baro_trunc(row.Data, 1500), tool.Baro_trunc(data, 1500), reason)
			baro_gov_policy_notify_staff(db, key, title, next, config.IP)

			return tool.Get_redirect("/baro/policy/view/" + tool.Url_parser(key))
		}
	}

	body := baro_gov_css()
	body += tool.Baro_crumb([][]string{
		{"/baro/policy", tool.Baro_t("정책 문서")},
		{"/baro/policy/view/" + tool.Url_parser(key), row.Title},
		{"", "새 판 작성"},
	})
	body += tool.Baro_hero(row.Title+tool.Baro_t(" 개정"), tool.Baro_t("지금 판은 ")+row.Version+tool.Baro_t("판입니다. 저장하면 ")+tool.Baro_itoa(baro_gov_policy_max_version(db, key)+1)+tool.Baro_t("판이 됩니다."), "")
	body += alert

	body += tool.Baro_alert("info",
		tool.Baro_t(`저장하면 <b>이전 판은 지워지지 않고 지난 판으로 남습니다</b>. 변경 사유와 시행일은 반드시 적어야 하며, 두 값은 공개 운영 로그에도 함께 남습니다.`),
	)

	body += tool.Baro_card("새 판 작성", baro_gov_policy_form(
		"/baro/policy/edit/"+tool.Url_parser(key),
		key, title, data, reason, eff, talk, false,
	))

	body += tool.Baro_card_ex(tool.Baro_t("지금 판 본문"), row.Version+tool.Baro_t("판 (시행일 ")+row.Eff+")", baro_gov_pre(row.Data), "")

	return tool.Baro_page(db, config, row.Title+tool.Baro_t(" 개정"), body, baro_gov_menu())
}

// View_baro_gov_policy_new : 새 정책 제정
func View_baro_gov_policy_new(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/policy/new"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "policy") {
		return tool.Baro_deny(db, config, "정책을 제정하려면 정책 권한(시스템 관리자 · 중재자)이 필요합니다.")
	}

	key := ""
	title := ""
	data := ""
	reason := "최초 제정"
	eff := tool.Baro_today()
	talk := ""
	alert := ""

	if values != nil {
		key = strings.ToLower(strings.TrimSpace(values.Get("key_name")))
		title = strings.TrimSpace(values.Get("title"))
		data = values.Get("data")
		reason = strings.TrimSpace(values.Get("reason"))
		eff = strings.TrimSpace(values.Get("effective_date"))
		talk = strings.TrimSpace(values.Get("discussion"))

		exist := ""
		duplicated := key != "" && tool.QueryRow_DB(db, "select key_name from baro_policy where key_name = ? limit 1", []any{&exist}, key)

		switch {
		case tool.Baro_read_only(db):
			alert = tool.Baro_alert("danger", "지금은 읽기 전용 상태입니다. "+tool.HTML_escape(tool.Baro_read_only_reason(db)))
		case !baro_gov_key_re.MatchString(key):
			alert = tool.Baro_alert("danger", "정책 키는 영소문자 · 숫자 · 밑줄 2~40자로 적어 주세요.")
		case duplicated:
			alert = tool.Baro_alert("danger", "이미 있는 정책 키입니다. 기존 정책을 고치려면 그 정책에서 새 판을 작성하세요.")
		case title == "":
			alert = tool.Baro_alert("danger", "정책 제목을 적어 주세요.")
		case strings.TrimSpace(data) == "":
			alert = tool.Baro_alert("danger", "정책 본문이 비어 있습니다.")
		case reason == "":
			alert = tool.Baro_alert("danger", "제정 사유를 적어 주세요.")
		case len(eff) != 10:
			alert = tool.Baro_alert("danger", "시행일을 날짜로 골라 주세요.")
		default:
			tool.Exec_DB(
				db,
				"insert into baro_policy (id, key_name, title, version, data, effective_date, proposer, reason, discussion, date, status) values (?, ?, ?, '1', ?, ?, ?, ?, ?, ?, 'active')",
				tool.Baro_next_id(db, "baro_policy"), key, title, data, eff, config.IP, reason, talk, tool.Baro_now(),
			)

			tool.Baro_public_log(db, "policy_new", config.IP, key, tool.Baro_trunc(reason, 300), tool.Baro_json_enc(map[string]string{
				"정책":  title,
				"판":   "1판",
				"시행일": eff,
			}))
			tool.Baro_audit(db, config, "policy_new", key, "", tool.Baro_trunc(data, 1500), reason)
			baro_gov_policy_notify_staff(db, key, title, "1", config.IP)

			return tool.Get_redirect("/baro/policy/view/" + tool.Url_parser(key))
		}
	}

	body := baro_gov_css()
	body += tool.Baro_crumb([][]string{{"/baro/policy", "정책 문서"}, {"", "새 정책 제정"}})
	body += tool.Baro_hero("새 정책 제정", "정책은 1판부터 시작해 개정할 때마다 판 번호가 올라갑니다.", "")
	body += alert

	body += tool.Baro_alert("info",
		tool.Baro_t(`정책 키는 주소에 쓰이는 이름입니다. 예를 들어 키를 <b>sanction</b> 으로 하면 주소는 /baro/policy/view/sanction 이 됩니다.`),
	)

	body += tool.Baro_card("정책 내용", baro_gov_policy_form("/baro/policy/new", key, title, data, reason, eff, talk, true))

	body += tool.Baro_card("이미 있는 정책 키", baro_gov_policy_key_hint(db))

	return tool.Baro_page(db, config, "새 정책 제정", body, baro_gov_menu())
}

// baro_gov_policy_key_hint : 이미 쓰이고 있는 정책 키 목록
func baro_gov_policy_key_hint(db *sql.DB) string {
	rows := tool.Query_DB(db, "select distinct key_name from baro_policy order by key_name asc limit 100")
	defer rows.Close()

	keys := []string{}
	for rows.Next() {
		key := ""
		if err := rows.Scan(&key); err != nil {
			continue
		}

		keys = append(keys, key)
	}

	if len(keys) == 0 {
		return tool.Baro_empty(tool.Baro_t("아직 등록된 정책이 없습니다."))
	}

	return tool.Baro_pills(keys)
}
