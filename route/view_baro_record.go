package route

import (
	"database/sql"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 [record] 모듈 — 통합 기여 기록
//
//  흩어져 있던 네 가지 기록을 한 화면에 모은다.
//    - 문서 편집 기록        (history)
//    - 토론 참여 기록        (topic)
//    - 편집 요청 생성 기록   (baro_edit_req)
//    - 편집 요청 처리 기록   (baro_edit_req_log : 승인 · 거절 · 삭제)
//
//  탭으로 종류를 고를 수 있고, 「전체」 탭에서는 네 가지가 시간순으로 섞여 나온다.
// ============================================================

// baro_record_page_size : 한 쪽에 보여 주는 줄 수
const baro_record_page_size = 50

// baro_record_kind_list : 탭 (열쇠말, 표시 이름 열쇠말)
func baro_record_kind_list() [][]string {
	return [][]string{
		{"all", "contribution_all"},
		{"edit", "edit_record"},
		{"topic", "discussion_record"},
		{"req", "edit_request_record"},
		{"req_handle", "edit_request_handle_record"},
	}
}

// baro_record_row : 통합 기록 한 줄
type baro_record_row struct {
	date   string
	kind   string
	target string
	detail string
	extra  string
}

// baro_record_esc : 화면에 넣기 전 이스케이프
func baro_record_esc(data string) string {
	return tool.HTML_escape(data)
}

// baro_record_page_no : 쪽 번호
func baro_record_page_no(page string) int {
	num := tool.Str_to_int(strings.TrimSpace(page))
	if num < 1 {
		num = 1
	}
	if num > 2000 {
		num = 2000
	}

	return num
}

// baro_record_kind_badge : 종류 뱃지
func baro_record_kind_badge(db *sql.DB, kind string) string {
	switch kind {
	case "edit":
		return tool.Baro_badge(tool.Get_language(db, "edit", true), "info")
	case "topic":
		return tool.Baro_badge(tool.Get_language(db, "discussion", true), "ok")
	case "req":
		return tool.Baro_badge(tool.Get_language(db, "edit_request", true), "warn")
	case "req_accept":
		return tool.Baro_badge(tool.Get_language(db, "edit_request_accept", true), "ok")
	case "req_reject":
		return tool.Baro_badge(tool.Get_language(db, "edit_request_reject", true), "muted")
	case "req_delete":
		return tool.Baro_badge(tool.Get_language(db, "edit_request_delete", true), "danger")
	}

	return tool.Baro_badge(kind, "muted")
}

// baro_record_edit_rows : 문서 편집 기록
func baro_record_edit_rows(db *sql.DB, user_name string, limit int) []baro_record_row {
	rows := tool.Query_DB(
		db,
		"select id, title, date, send, leng, hide from history where ip = ? order by date desc limit ?",
		user_name, limit,
	)
	defer rows.Close()

	list := []baro_record_row{}
	for rows.Next() {
		id, title, date, send, leng, hide := "", "", "", "", "", ""
		if rows.Scan(&id, &title, &date, &send, &leng, &hide) != nil {
			continue
		}

		if hide != "" {
			continue
		}

		length_html := baro_record_esc(leng)
		switch {
		case leng == "0" || leng == "":
			length_html = `<span style="color: var(--namu-length-same-color, #808080);">` + baro_record_esc(leng) + `</span>`
		case strings.Contains(leng, "+"):
			length_html = `<span style="color: var(--namu-length-plus-color, #008000);">` + baro_record_esc(leng) + `</span>`
		default:
			length_html = `<span style="color: var(--namu-length-minus-color, #f00);">` + baro_record_esc(leng) + `</span>`
		}

		target := tool.Baro_doc_link(title)
		if tool.Str_to_int(id) > 1 {
			target += ` <a href="/diff/` + tool.Url_parser(tool.Baro_itoa(tool.Str_to_int(id)-1)) + `/` + tool.Url_parser(id) + `/` + tool.Url_parser(title) + `">r` + baro_record_esc(id) + `</a>`
		} else {
			target += ` <a href="/w_rev/` + tool.Url_parser(id) + `/` + tool.Url_parser(title) + `">r` + baro_record_esc(id) + `</a>`
		}

		list = append(list, baro_record_row{
			date:   date,
			kind:   "edit",
			target: target,
			detail: baro_record_esc(tool.Baro_trunc(send, 80)),
			extra:  length_html,
		})
	}

	return list
}

// baro_record_topic_rows : 토론 참여 기록
func baro_record_topic_rows(db *sql.DB, user_name string, limit int) []baro_record_row {
	rows := tool.Query_DB(
		db,
		"select code, id, date, data from topic where ip = ? order by date desc limit ?",
		user_name, limit,
	)
	defer rows.Close()

	type one_topic struct {
		code    string
		comment string
		date    string
		data    string
	}

	found := []one_topic{}
	for rows.Next() {
		code, comment_id, date, data := "", "", "", ""
		if rows.Scan(&code, &comment_id, &date, &data) != nil {
			continue
		}

		found = append(found, one_topic{code, comment_id, date, data})
	}
	rows.Close()

	list := []baro_record_row{}
	for _, one := range found {
		topic_title, topic_sub := "", ""
		tool.QueryRow_DB(db, "select title, sub from rd where code = ?", []any{&topic_title, &topic_sub}, one.code)

		target := `<a href="/thread/` + tool.Url_parser(one.code) + `#` + tool.Url_parser(one.comment) + `">` +
			baro_record_esc(topic_sub) + `</a>`
		if topic_title != "" {
			target += ` <span class="bw_muted">(` + baro_record_esc(tool.Namespace_display(topic_title)) + `)</span>`
		}

		list = append(list, baro_record_row{
			date:   one.date,
			kind:   "topic",
			target: target,
			detail: baro_record_esc(tool.Baro_trunc(tool.Baro_one_line(one.data), 80)),
			extra:  "#" + baro_record_esc(one.comment),
		})
	}

	return list
}

// baro_record_req_rows : 편집 요청 생성 기록
func baro_record_req_rows(db *sql.DB, user_name string, limit int) []baro_record_row {
	rows := tool.Query_DB(
		db,
		"select id, title, summary, status, date from baro_edit_req where user = ? order by date desc limit ?",
		user_name, limit,
	)
	defer rows.Close()

	list := []baro_record_row{}
	for rows.Next() {
		id, title, summary, status, date := "", "", "", "", ""
		if rows.Scan(&id, &title, &summary, &status, &date) != nil {
			continue
		}

		target := tool.Baro_doc_link(title) +
			` <a href="/baro/req/view/` + tool.Url_parser(id) + `">#` + baro_record_esc(id) + `</a>`

		list = append(list, baro_record_row{
			date:   date,
			kind:   "req",
			target: target,
			detail: baro_record_esc(tool.Baro_trunc(summary, 80)),
			extra:  baro_req_status_badge(db, status),
		})
	}

	return list
}

// baro_record_req_handle_rows : 편집 요청 처리 기록 (승인 · 거절 · 삭제)
func baro_record_req_handle_rows(db *sql.DB, user_name string, limit int) []baro_record_row {
	rows := tool.Query_DB(
		db,
		"select req_id, title, action, detail, date from baro_edit_req_log where actor = ? and action != 'create' order by date desc limit ?",
		user_name, limit,
	)
	defer rows.Close()

	list := []baro_record_row{}
	for rows.Next() {
		req_id, title, action, detail, date := "", "", "", "", ""
		if rows.Scan(&req_id, &title, &action, &detail, &date) != nil {
			continue
		}

		target := tool.Baro_doc_link(title) +
			` <a href="/baro/req/view/` + tool.Url_parser(req_id) + `">#` + baro_record_esc(req_id) + `</a>`

		list = append(list, baro_record_row{
			date:   date,
			kind:   "req_" + action,
			target: target,
			detail: baro_record_esc(tool.Baro_trunc(detail, 80)),
			extra:  baro_record_esc(baro_req_action_name(db, action)),
		})
	}

	return list
}

// baro_record_count : 종류별 건수
func baro_record_count(db *sql.DB, user_name string) map[string]int {
	count := map[string]int{}

	edit_count := 0
	tool.QueryRow_DB(db, "select count(*) from history where ip = ?", []any{&edit_count}, user_name)
	count["edit"] = edit_count

	topic_count := 0
	tool.QueryRow_DB(db, "select count(*) from topic where ip = ?", []any{&topic_count}, user_name)
	count["topic"] = topic_count

	req_count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_edit_req where user = ?", []any{&req_count}, user_name)
	count["req"] = req_count

	handle_count := 0
	tool.QueryRow_DB(
		db,
		"select count(*) from baro_edit_req_log where actor = ? and action != 'create'",
		[]any{&handle_count}, user_name,
	)
	count["req_handle"] = handle_count

	return count
}

// View_baro_record : 통합 기여 기록
func View_baro_record(config tool.Config, user_name string, kind string, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	user_name = strings.TrimSpace(user_name)
	if user_name == "" {
		user_name = config.IP
	}

	if kind == "" {
		kind = "all"
	}

	page_num := baro_record_page_no(page)
	offset := (page_num - 1) * baro_record_page_size
	need := offset + baro_record_page_size

	list := []baro_record_row{}
	switch kind {
	case "edit":
		list = baro_record_edit_rows(db, user_name, need)
	case "topic":
		list = baro_record_topic_rows(db, user_name, need)
	case "req":
		list = baro_record_req_rows(db, user_name, need)
	case "req_handle":
		list = baro_record_req_handle_rows(db, user_name, need)
	default:
		kind = "all"
		list = append(list, baro_record_edit_rows(db, user_name, need)...)
		list = append(list, baro_record_topic_rows(db, user_name, need)...)
		list = append(list, baro_record_req_rows(db, user_name, need)...)
		list = append(list, baro_record_req_handle_rows(db, user_name, need)...)

		sort.SliceStable(list, func(a int, b int) bool {
			return list[a].date > list[b].date
		})
	}

	if offset >= len(list) {
		list = []baro_record_row{}
	} else {
		list = list[offset:]
	}
	if len(list) > baro_record_page_size {
		list = list[:baro_record_page_size]
	}

	table_rows := [][]string{}
	for _, one := range list {
		table_rows = append(table_rows, []string{
			tool.Baro_time_cell(one.date),
			baro_record_kind_badge(db, one.kind),
			one.target,
			one.detail,
			one.extra,
		})
	}

	count := baro_record_count(db, user_name)

	tabs := [][]string{}
	for _, one := range baro_record_kind_list() {
		label := tool.Get_language(db, one[1], true)
		if number, ok := count[one[0]]; ok {
			label += " " + tool.Baro_comma(number)
		}

		tabs = append(tabs, []string{
			"/baro/record/user/" + tool.Url_parser(user_name) + "/" + one[0],
			label,
		})
	}

	body := tool.Baro_hero(
		tool.Get_language(db, "contribution", true),
		tool.IP_parser(db, user_name, config.IP),
		tool.Baro_stat_row([][]string{
			{tool.Get_language(db, "edit_record", true), tool.Baro_comma(count["edit"]), ""},
			{tool.Get_language(db, "discussion_record", true), tool.Baro_comma(count["topic"]), ""},
			{tool.Get_language(db, "edit_request_record", true), tool.Baro_comma(count["req"]), ""},
			{tool.Get_language(db, "edit_request_handle_record", true), tool.Baro_comma(count["req_handle"]), ""},
		}),
	)

	inner := tool.Baro_tabs(tabs, "/baro/record/user/"+tool.Url_parser(user_name)+"/"+kind)
	inner += tool.Baro_table(
		[]string{
			tool.Get_language(db, "date", true),
			tool.Get_language(db, "type", true),
			tool.Get_language(db, "content", true),
			tool.Get_language(db, "why", true),
			tool.Get_language(db, "etc", true),
		},
		table_rows,
		tool.Baro_t("기록이 없습니다."),
	)
	inner += tool.Baro_paging(
		page_num, len(table_rows), baro_record_page_size,
		"/baro/record/user/"+tool.Url_parser(user_name)+"/"+tool.Url_parser(kind)+"/{}",
	)

	body += tool.Baro_card("", inner)

	link := tool.Baro_button("/baro/contrib/user/"+tool.Url_parser(user_name), "기여 이벤트", "") + " " +
		tool.Baro_button("/user/"+tool.Url_parser(user_name), tool.Get_language(db, "user_tool", true), "") + " " +
		tool.Baro_button("/baro/req/new", tool.Get_language(db, "edit_request_make", true), "")

	body += tool.Baro_card("", link)

	return tool.Baro_page(
		db, config,
		tool.Get_language(db, "contribution", true),
		body,
		[][]any{
			{"baro/record", tool.Get_language(db, "contribution", true)},
			{"baro", "바로위키"},
		},
	)
}
