package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

// ============================================================
//  바로위키 [req] 모듈 — 편집 요청
//
//  편집 권한이 없거나 스스로 확신이 서지 않을 때 본문을 만들어 두고
//  편집할 수 있는 사람에게 반영을 부탁하는 기능이다.
//
//  두 갈래로 기록이 남는다.
//    - 생성 기록 : 누가 어떤 문서에 편집 요청을 만들었는가 (baro_edit_req)
//    - 처리 기록 : 누가 그 요청을 승인·거절·삭제했는가 (baro_edit_req_log)
//
//  두 기록 모두 통합 기여 기록(/baro/record)에서 한 줄로 같이 보인다.
// ============================================================

// baro_req_page_size : 한 쪽에 보여 주는 항목 수
const baro_req_page_size = 30

// baro_req_status_list : 요청 상태와 표시 이름
func baro_req_status_list() [][]string {
	return [][]string{
		{"open", "edit_request_open", "warn"},
		{"accepted", "edit_request_accepted", "ok"},
		{"rejected", "edit_request_rejected", "muted"},
		{"deleted", "edit_request_deleted", "danger"},
	}
}

// baro_req_status_name : 상태 표시 이름
func baro_req_status_name(db *sql.DB, status string) string {
	for _, row := range baro_req_status_list() {
		if row[0] == status {
			return tool.Get_language(db, row[1], true)
		}
	}

	return status
}

// baro_req_status_badge : 상태 뱃지
func baro_req_status_badge(db *sql.DB, status string) string {
	for _, row := range baro_req_status_list() {
		if row[0] == status {
			return tool.Baro_badge(tool.Get_language(db, row[1], true), row[2])
		}
	}

	return tool.Baro_badge(status, "muted")
}

// baro_req_action_name : 처리 기록의 행동 표시 이름
func baro_req_action_name(db *sql.DB, action string) string {
	switch action {
	case "create":
		return tool.Get_language(db, "edit_request_make", true)
	case "accept":
		return tool.Get_language(db, "edit_request_accept", true)
	case "reject":
		return tool.Get_language(db, "edit_request_reject", true)
	case "delete":
		return tool.Get_language(db, "edit_request_delete", true)
	}

	return action
}

// baro_req_page_no : 쪽 번호를 1 이상의 정수로
func baro_req_page_no(page string) int {
	num := tool.Str_to_int(page)
	if num < 1 {
		num = 1
	}
	if num > 2000 {
		num = 2000
	}

	return num
}

// baro_req_esc : 짧은 HTML escape
func baro_req_esc(data string) string {
	return tool.HTML_escape(data)
}

// baro_req_log_add : 처리 기록 한 줄 남기기
func baro_req_log_add(db *sql.DB, req_id string, title string, actor string, action string, detail string) {
	tool.Exec_DB(
		db,
		"insert into baro_edit_req_log (id, req_id, title, actor, action, detail, date) values (?, ?, ?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_edit_req_log"),
		req_id, title, actor, action, tool.Baro_trunc(detail, 500), tool.Baro_now(),
	)
}

// baro_req_one : 편집 요청 한 건 읽기 (없으면 빈 map)
func baro_req_one(db *sql.DB, id string) map[string]string {
	row := map[string]string{}

	req_title, base_rev, user_name, data, summary := "", "", "", "", ""
	status, handler, handle_date, reason, date, new_rev := "", "", "", "", "", ""

	found := tool.QueryRow_DB(
		db,
		"select title, base_rev, user, data, summary, status, handler, handle_date, reason, date, new_rev from baro_edit_req where id = ?",
		[]any{&req_title, &base_rev, &user_name, &data, &summary, &status, &handler, &handle_date, &reason, &date, &new_rev},
		id,
	)
	if !found {
		return row
	}

	row["id"] = id
	row["title"] = req_title
	row["base_rev"] = base_rev
	row["user"] = user_name
	row["data"] = data
	row["summary"] = summary
	row["status"] = status
	row["handler"] = handler
	row["handle_date"] = handle_date
	row["reason"] = reason
	row["date"] = date
	row["new_rev"] = new_rev

	return row
}

// baro_req_can_handle : 이 문서의 편집 요청을 처리할 수 있는가
//
//	문서를 실제로 편집할 수 있는 사람이 처리한다.
func baro_req_can_handle(db *sql.DB, config tool.Config, title string) bool {
	if !tool.Baro_is_user(config.IP) {
		return false
	}

	return tool.Check_acl(db, title, "", "document_edit", config.IP)
}

// baro_req_can_make : 편집 요청을 만들 수 있는가 (가입한 사용자만)
func baro_req_can_make(db *sql.DB, config tool.Config, title string) bool {
	if !tool.Baro_is_user(config.IP) {
		return false
	}

	// 읽을 수 없는 문서에는 편집 요청도 만들 수 없다.
	return tool.Check_acl(db, title, "", "view", config.IP)
}

// baro_req_menu : 공통 메뉴
func baro_req_menu() [][]any {
	return [][]any{
		{"baro/req", tool.Baro_t("편집 요청")},
		{"baro", "바로위키"},
	}
}

// baro_req_tabs : 목록 탭
func baro_req_tabs(db *sql.DB, active string) string {
	items := [][]string{
		{"/baro/req", tool.Get_language(db, "contribution_all", true)},
	}
	for _, row := range baro_req_status_list() {
		items = append(items, []string{"/baro/req/list/" + row[0], tool.Get_language(db, row[1], true)})
	}

	items = append(items, []string{"/baro/req/log", tool.Get_language(db, "edit_request_handle_record", true)})

	return tool.Baro_tabs(items, active)
}

// View_baro_req_list : 편집 요청 목록
func View_baro_req_list(config tool.Config, status string, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_num := baro_req_page_no(page)
	offset := (page_num - 1) * baro_req_page_size

	var rows *sql.Rows
	if status == "" {
		rows = tool.Query_DB(
			db,
			"select id, title, user, summary, status, date, handler from baro_edit_req order by date desc limit ?, ?",
			offset, baro_req_page_size,
		)
	} else {
		rows = tool.Query_DB(
			db,
			"select id, title, user, summary, status, date, handler from baro_edit_req where status = ? order by date desc limit ?, ?",
			status, offset, baro_req_page_size,
		)
	}
	defer rows.Close()

	list := [][]string{}
	count := 0
	for rows.Next() {
		id, title, user_name, summary, one_status, date, handler := "", "", "", "", "", "", ""
		if rows.Scan(&id, &title, &user_name, &summary, &one_status, &date, &handler) != nil {
			continue
		}

		count++
		list = append(list, []string{
			`<a href="/baro/req/view/` + tool.Url_parser(id) + `">#` + baro_req_esc(id) + `</a>`,
			tool.Baro_doc_link(title),
			tool.Baro_user_link(db, user_name, config.IP),
			baro_req_esc(tool.Baro_trunc(summary, 60)),
			baro_req_status_badge(db, one_status),
			tool.Baro_time_cell(date),
		})
	}
	rows.Close()

	body := tool.Baro_hero(
		tool.Get_language(db, "edit_request", true),
		tool.Baro_t("문서를 직접 고치기 어려울 때 본문을 만들어 두고 반영을 부탁하는 곳입니다."),
		tool.Baro_button("/baro/req/new", tool.Get_language(db, "edit_request_make", true), "primary"),
	)

	active := "/baro/req"
	if status != "" {
		active = "/baro/req/list/" + status
	}

	inner := baro_req_tabs(db, active)
	inner += tool.Baro_table(
		[]string{"번호", tool.Get_language(db, "document", true), tool.Get_language(db, "writer", true),
			tool.Get_language(db, "why", true), tool.Get_language(db, "state", true), tool.Get_language(db, "date", true)},
		list,
		tool.Baro_t("편집 요청이 없습니다."),
	)

	url_tail := "/baro/req/page/{}"
	if status != "" {
		url_tail = "/baro/req/list/" + tool.Url_parser(status) + "/{}"
	}
	inner += tool.Baro_paging(page_num, count, baro_req_page_size, url_tail)

	body += tool.Baro_card("", inner)

	return tool.Baro_page(db, config, tool.Get_language(db, "edit_request_list", true), body, baro_req_menu())
}

// View_baro_req_new : 편집 요청 만들기
func View_baro_req_new(config tool.Config, doc_name string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_is_user(config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("편집 요청은 가입한 사용자만 만들 수 있습니다."))
	}

	if baro_reason, baro_ok := tool.Baro_hook_can_act(db, config, "edit"); !baro_ok {
		return tool.Baro_deny(db, config, baro_reason)
	}

	title := tool.Namespace_normalize(strings.TrimSpace(doc_name))

	if values != nil {
		title = tool.Namespace_normalize(strings.TrimSpace(values.Get("title")))
		data := strings.ReplaceAll(values.Get("data"), "\r", "")
		summary := tool.Baro_one_line(values.Get("summary"))

		if title == "" {
			return tool.Baro_deny(db, config, tool.Baro_t("문서 이름을 적어 주세요."))
		}
		if !baro_req_can_make(db, config, title) {
			return tool.Baro_deny(db, config, tool.Baro_t("이 문서에는 편집 요청을 만들 수 없습니다."))
		}
		if strings.TrimSpace(data) == "" {
			return tool.Baro_deny(db, config, tool.Baro_t("바꿀 본문을 적어 주세요."))
		}
		if !tool.Do_edit_max_length_check(db, config, data) {
			return tool.Baro_deny(db, config, tool.Baro_t("본문이 너무 깁니다."))
		}

		id := tool.Baro_next_id(db, "baro_edit_req")
		base_rev := tool.Get_document_revision(db, title)

		tool.Exec_DB(
			db,
			"insert into baro_edit_req (id, title, base_rev, user, data, summary, status, handler, handle_date, reason, date, new_rev)"+
				" values (?, ?, ?, ?, ?, ?, 'open', '', '', '', ?, '')",
			id, title, base_rev, config.IP, data, summary, tool.Baro_now(),
		)

		baro_req_log_add(db, id, title, config.IP, "create", summary)

		// 근거 공개 : 편집 요청이 생겼다는 사실은 공개 기록으로 남긴다
		tool.Baro_public_log(db, "edit_request_create", config.IP, title, summary, "요청 번호 "+id)

		// 문서를 편집할 수 있는 사람들이 볼 수 있도록 문서 주시자에게 알린다
		baro_req_notify_watcher(db, config.IP, title, id)

		return tool.Get_redirect("/baro/req/view/" + tool.Url_parser(id))
	}

	now_data := ""
	if title != "" {
		tool.QueryRow_DB(db, "select data from data where title = ?", []any{&now_data}, title)
	}

	form := tool.Baro_input("title", tool.Get_language(db, "document_name", true), tool.Namespace_display(title), tool.Baro_t("예) 바로위키"), tool.Baro_t("고치고 싶은 문서 이름"))
	form += tool.Baro_textarea("data", tool.Get_language(db, "content", true), now_data, 18, "", tool.Baro_t("이 내용이 그대로 저장됩니다. 지금 본문을 미리 넣어 두었습니다."))
	form += tool.Baro_input("summary", tool.Get_language(db, "why", true), "", tool.Baro_t("무엇을 왜 고치는지"), tool.Baro_t("처리하는 사람이 가장 먼저 읽는 줄입니다."))
	form += tool.Baro_submit(tool.Get_language(db, "edit_request_make", true), "primary")

	body := tool.Baro_hero(tool.Get_language(db, "edit_request_make", true), "", "")
	body += tool.Baro_card("", tool.Baro_form("/baro/req/new", form))

	return tool.Baro_page(db, config, tool.Get_language(db, "edit_request_make", true), body, baro_req_menu())
}

// baro_req_notify_watcher : 문서를 주시하는 사람에게 편집 요청이 생겼다고 알린다
func baro_req_notify_watcher(db *sql.DB, actor string, title string, req_id string) {
	rows := tool.Query_DB(
		db,
		"select user from baro_watch where kind = 'document' and target = ?",
		title,
	)
	defer rows.Close()

	targets := []string{}
	for rows.Next() {
		user_name := ""
		if rows.Scan(&user_name) != nil {
			continue
		}
		if user_name == actor {
			continue
		}

		targets = append(targets, user_name)
	}
	rows.Close()

	for _, user_name := range targets {
		tool.Baro_notify(
			db, user_name, "watch",
			tool.Namespace_display(title)+tool.Baro_t(" 문서에 편집 요청이 들어왔습니다."),
			"/baro/req/view/"+tool.Url_parser(req_id),
			"",
		)
	}
}

// View_baro_req_view : 편집 요청 한 건 보기 · 처리
func View_baro_req_view(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row := baro_req_one(db, id)
	if len(row) == 0 {
		return tool.Baro_deny(db, config, tool.Baro_t("없는 편집 요청입니다."))
	}

	title := row["title"]
	can_handle := baro_req_can_handle(db, config, title)
	is_mine := config.IP == row["user"] && tool.Baro_is_user(config.IP)

	if values != nil {
		action := values.Get("action")
		reason := tool.Baro_one_line(values.Get("reason"))

		if row["status"] != "open" {
			return tool.Baro_deny(db, config, tool.Baro_t("이미 처리된 편집 요청입니다."))
		}

		switch action {
		case "accept":
			if !can_handle {
				return tool.Baro_deny(db, config, tool.Baro_t("이 문서를 편집할 수 있는 사람만 승인할 수 있습니다."))
			}

			new_rev := baro_req_apply(db, config, row, reason)

			tool.Exec_DB(
				db,
				"update baro_edit_req set status = 'accepted', handler = ?, handle_date = ?, reason = ?, new_rev = ? where id = ?",
				config.IP, tool.Baro_now(), reason, new_rev, id,
			)
			baro_req_log_add(db, id, title, config.IP, "accept", reason)
			tool.Baro_public_log(db, "edit_request_accept", config.IP, title, reason, "요청 번호 "+id)
			tool.Baro_notify(
				db, row["user"], "review",
				tool.Namespace_display(title)+tool.Baro_t(" 편집 요청이 승인됐습니다."),
				"/baro/req/view/"+tool.Url_parser(id), "",
			)

		case "reject":
			if !can_handle {
				return tool.Baro_deny(db, config, tool.Baro_t("이 문서를 편집할 수 있는 사람만 거절할 수 있습니다."))
			}
			if reason == "" {
				return tool.Baro_deny(db, config, tool.Baro_t("거절 사유를 적어 주세요."))
			}

			tool.Exec_DB(
				db,
				"update baro_edit_req set status = 'rejected', handler = ?, handle_date = ?, reason = ? where id = ?",
				config.IP, tool.Baro_now(), reason, id,
			)
			baro_req_log_add(db, id, title, config.IP, "reject", reason)
			tool.Baro_public_log(db, "edit_request_reject", config.IP, title, reason, "요청 번호 "+id)
			tool.Baro_notify(
				db, row["user"], "review",
				tool.Namespace_display(title)+tool.Baro_t(" 편집 요청이 거절됐습니다."),
				"/baro/req/view/"+tool.Url_parser(id), "",
			)

		case "delete":
			if !is_mine && !can_handle && !tool.Baro_is_admin(db, config.IP) {
				return tool.Baro_deny(db, config, tool.Baro_t("요청한 본인이나 처리 권한이 있는 사람만 지울 수 있습니다."))
			}

			// 2번 원칙 : 지워도 요청 본문은 남기고 상태만 바꾼다
			tool.Exec_DB(
				db,
				"update baro_edit_req set status = 'deleted', handler = ?, handle_date = ?, reason = ? where id = ?",
				config.IP, tool.Baro_now(), reason, id,
			)
			baro_req_log_add(db, id, title, config.IP, "delete", reason)
			tool.Baro_public_log(db, "edit_request_delete", config.IP, title, reason, "요청 번호 "+id)

		default:
			return tool.Baro_deny(db, config, tool.Baro_t("알 수 없는 처리입니다."))
		}

		return tool.Get_redirect("/baro/req/view/" + tool.Url_parser(id))
	}

	now_rev := tool.Get_document_revision(db, title)

	info := [][]string{
		{tool.Get_language(db, "document", true), tool.Baro_doc_link(title)},
		{tool.Get_language(db, "writer", true), tool.Baro_user_link(db, row["user"], config.IP)},
		{tool.Get_language(db, "state", true), baro_req_status_badge(db, row["status"])},
		{tool.Get_language(db, "why", true), baro_req_esc(row["summary"])},
		{tool.Get_language(db, "date", true), tool.Baro_time_cell(row["date"])},
		{tool.Baro_t("기준 판"), "r" + baro_req_esc(row["base_rev"]) + tool.Baro_t(" (지금 r") + baro_req_esc(now_rev) + ")"},
	}

	if row["handler"] != "" {
		info = append(info, []string{tool.Baro_t("처리한 사람"), tool.Baro_user_link(db, row["handler"], config.IP)})
		info = append(info, []string{tool.Baro_t("처리 시각"), tool.Baro_time_cell(row["handle_date"])})
	}
	if row["reason"] != "" {
		info = append(info, []string{tool.Baro_t("처리 사유"), baro_req_esc(row["reason"])})
	}
	if row["new_rev"] != "" {
		info = append(info, []string{"반영된 판", `<a href="/w_rev/` + tool.Url_parser(row["new_rev"]) + `/` + tool.Url_parser(title) + `">r` + baro_req_esc(row["new_rev"]) + `</a>`})
	}

	body := tool.Baro_hero(tool.Get_language(db, "edit_request_view", true)+" #"+baro_req_esc(id), tool.Namespace_display(title), "")
	body += tool.Baro_card("", tool.Baro_dl(info))

	if row["status"] == "open" && row["base_rev"] != now_rev {
		body += tool.Baro_alert("warn",
			tool.Baro_t("이 요청을 만든 뒤에 문서가 다시 편집됐습니다. (r")+baro_req_esc(row["base_rev"])+
				" → r"+baro_req_esc(now_rev)+tool.Baro_t(") 승인하면 그 사이의 편집은 이 본문으로 덮어써집니다."))
	}

	body += tool.Baro_card(tool.Get_language(db, "content", true), `<pre class="bw_pre">`+baro_req_esc(row["data"])+`</pre>`)

	if row["status"] == "open" {
		form := ""
		if can_handle {
			form += tool.Baro_input("reason", tool.Get_language(db, "why", true), "", "", tool.Baro_t("거절할 때는 사유가 반드시 필요합니다."))
			form += tool.Baro_hidden("action", "accept")
			form += tool.Baro_submit(tool.Get_language(db, "edit_request_accept", true), "primary")

			body += tool.Baro_card(tool.Get_language(db, "approve_or_decline", true), tool.Baro_form("/baro/req/view/"+tool.Url_parser(id), form))

			reject := tool.Baro_input("reason", tool.Get_language(db, "why", true), "", "", "")
			reject += tool.Baro_hidden("action", "reject")
			reject += tool.Baro_submit(tool.Get_language(db, "edit_request_reject", true), "danger")

			body += tool.Baro_card("", tool.Baro_form("/baro/req/view/"+tool.Url_parser(id), reject))
		}

		if is_mine || can_handle || tool.Baro_is_admin(db, config.IP) {
			del := tool.Baro_input("reason", tool.Get_language(db, "why", true), "", "", "")
			del += tool.Baro_hidden("action", "delete")
			del += tool.Baro_submit(tool.Get_language(db, "edit_request_delete", true), "danger")

			body += tool.Baro_danger_zone(tool.Get_language(db, "edit_request_delete", true),
				tool.Baro_form("/baro/req/view/"+tool.Url_parser(id), del))
		}
	}

	body += baro_req_log_html(db, config, id)

	return tool.Baro_page(db, config, tool.Get_language(db, "edit_request_view", true), body, baro_req_menu())
}

// baro_req_log_html : 이 요청에 남은 처리 기록
func baro_req_log_html(db *sql.DB, config tool.Config, req_id string) string {
	rows := tool.Query_DB(
		db,
		"select actor, action, detail, date from baro_edit_req_log where req_id = ? order by date asc, id + 0 asc",
		req_id,
	)
	defer rows.Close()

	list := [][]string{}
	for rows.Next() {
		actor, action, detail, date := "", "", "", ""
		if rows.Scan(&actor, &action, &detail, &date) != nil {
			continue
		}

		list = append(list, []string{
			tool.Baro_time_cell(date),
			baro_req_esc(baro_req_action_name(db, action)),
			tool.Baro_user_link(db, actor, config.IP),
			baro_req_esc(detail),
		})
	}
	rows.Close()

	return tool.Baro_card(
		tool.Get_language(db, "record", true),
		tool.Baro_table(
			[]string{tool.Get_language(db, "date", true), tool.Get_language(db, "type", true),
				tool.Get_language(db, "user", true), tool.Get_language(db, "explanation", true)},
			list, tool.Baro_t("기록이 없습니다."),
		),
	)
}

// baro_req_apply : 승인된 편집 요청을 문서에 반영한다
//
//	4번 원칙 : 반영된 편집의 주인은 요청한 사람이다. (처리한 사람이 아니다)
//	되돌릴 수 있도록 openNAMU 역사에 그대로 한 판으로 남긴다.
func baro_req_apply(db *sql.DB, config tool.Config, row map[string]string, reason string) string {
	title := row["title"]
	data := row["data"]

	old_data := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&old_data}, title)

	length := tool.Get_edit_length_diff(old_data, data)

	send := "편집 요청 #" + row["id"] + " 승인 (처리 : " + config.IP + ")"
	if reason != "" {
		send += " — " + reason
	}
	send = tool.Baro_trunc(send, 400)

	tool.Exec_DB(db, "delete from data where title = ?", title)
	tool.Exec_DB(db, "insert into data (title, data) values (?, ?)", title, data)

	tool.Do_watchlist_alarm_send(db, config, title)

	// 역사에 남는 편집자는 요청한 사람이다
	tool.Do_add_history(db, title, data, tool.Get_time(), row["user"], send, length, "", "")

	markup.Get_render(db, title, data, "backlink")

	// 기여 이벤트 · 출처 · 검토 큐 · 지표도 요청한 사람 이름으로 기록한다
	tool.Baro_hook_after_edit(db, tool.Config{IP: row["user"]}, title, old_data, data, send)

	return tool.Get_document_revision(db, title)
}

// View_baro_req_log : 편집 요청 처리 기록 (승인 · 거절 · 삭제 · 생성 전부)
func View_baro_req_log(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_num := baro_req_page_no(page)
	offset := (page_num - 1) * baro_req_page_size

	rows := tool.Query_DB(
		db,
		"select req_id, title, actor, action, detail, date from baro_edit_req_log order by date desc, id + 0 desc limit ?, ?",
		offset, baro_req_page_size,
	)
	defer rows.Close()

	list := [][]string{}
	count := 0
	for rows.Next() {
		req_id, title, actor, action, detail, date := "", "", "", "", "", ""
		if rows.Scan(&req_id, &title, &actor, &action, &detail, &date) != nil {
			continue
		}

		count++
		list = append(list, []string{
			tool.Baro_time_cell(date),
			`<a href="/baro/req/view/` + tool.Url_parser(req_id) + `">#` + baro_req_esc(req_id) + `</a>`,
			tool.Baro_doc_link(title),
			baro_req_esc(baro_req_action_name(db, action)),
			tool.Baro_user_link(db, actor, config.IP),
			baro_req_esc(tool.Baro_trunc(detail, 60)),
		})
	}
	rows.Close()

	inner := baro_req_tabs(db, "/baro/req/log")
	inner += tool.Baro_table(
		[]string{tool.Get_language(db, "date", true), "번호", tool.Get_language(db, "document", true),
			tool.Get_language(db, "type", true), tool.Get_language(db, "user", true), tool.Get_language(db, "explanation", true)},
		list, tool.Baro_t("기록이 없습니다."),
	)
	inner += tool.Baro_paging(page_num, count, baro_req_page_size, "/baro/req/log/{}")

	body := tool.Baro_hero(tool.Get_language(db, "edit_request_handle_record", true), tool.Baro_t("누가 어떤 편집 요청을 만들고 처리했는지 모두 남습니다."), "")
	body += tool.Baro_card("", inner)

	return tool.Baro_page(db, config, tool.Get_language(db, "edit_request_handle_record", true), body, baro_req_menu())
}

// View_baro_req_doc : 문서 하나에 걸린 편집 요청 목록
func View_baro_req_doc(config tool.Config, doc_name string, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	title := tool.Namespace_normalize(strings.TrimSpace(doc_name))
	if title == "" {
		return tool.Get_redirect("/baro/req")
	}

	page_num := baro_req_page_no(page)
	offset := (page_num - 1) * baro_req_page_size

	rows := tool.Query_DB(
		db,
		"select id, user, summary, status, date from baro_edit_req where title = ? order by date desc limit ?, ?",
		title, offset, baro_req_page_size,
	)
	defer rows.Close()

	list := [][]string{}
	count := 0
	for rows.Next() {
		id, user_name, summary, status, date := "", "", "", "", ""
		if rows.Scan(&id, &user_name, &summary, &status, &date) != nil {
			continue
		}

		count++
		list = append(list, []string{
			`<a href="/baro/req/view/` + tool.Url_parser(id) + `">#` + baro_req_esc(id) + `</a>`,
			tool.Baro_user_link(db, user_name, config.IP),
			baro_req_esc(tool.Baro_trunc(summary, 60)),
			baro_req_status_badge(db, status),
			tool.Baro_time_cell(date),
		})
	}
	rows.Close()

	body := tool.Baro_hero(tool.Get_language(db, "edit_request_list", true), tool.Namespace_display(title),
		tool.Baro_button("/baro/req/new?doc="+tool.Url_parser(title), tool.Get_language(db, "edit_request_make", true), "primary"))

	inner := tool.Baro_table(
		[]string{"번호", tool.Get_language(db, "writer", true), tool.Get_language(db, "why", true),
			tool.Get_language(db, "state", true), tool.Get_language(db, "date", true)},
		list, tool.Baro_t("편집 요청이 없습니다."),
	)
	inner += tool.Baro_paging(page_num, count, baro_req_page_size, "/baro/req/doc/"+tool.Url_parser(title)+"/{}")

	body += tool.Baro_card("", inner)

	return tool.Baro_page(db, config, tool.Get_language(db, "edit_request_list", true), body, baro_req_menu())
}
