package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
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
	case "reopen":
		return tool.Baro_t("다시 열기")
	case "lock":
		return tool.Baro_t("잠금")
	case "unlock":
		return tool.Baro_t("잠금 해제")
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
	kind, target, locked := "", "", ""

	found := tool.QueryRow_DB(
		db,
		"select title, base_rev, user, data, summary, status, handler, handle_date, reason, date, new_rev, kind, target, locked from baro_edit_req where id = ?",
		[]any{&req_title, &base_rev, &user_name, &data, &summary, &status, &handler, &handle_date, &reason, &date, &new_rev, &kind, &target, &locked},
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
	row["kind"] = baro_req_kind_normalize(kind)
	row["target"] = target
	row["locked"] = locked

	return row
}

// baro_req_is_locked : 잠긴 요청인가
//
//	잠긴 요청은 승인 · 거절 · 삭제 · 다시 열기가 모두 막힌다.
//	같은 부탁이 되풀이해 올라오는 것을 멈추는 마지막 수단이라 관리자만 잠그고 푼다.
func baro_req_is_locked(row map[string]string) bool {
	return row["locked"] == "1"
}

// baro_req_status_now : 지금 DB 에 적힌 요청 상태 (경쟁 상태 확인용)
func baro_req_status_now(db *sql.DB, id string) string {
	status := ""
	tool.QueryRow_DB(db, "select status from baro_edit_req where id = ?", []any{&status}, id)

	return status
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
//
//	윗줄은 「무엇을 부탁하는가」(종류), 아랫줄은 「어디까지 처리됐는가」(상태) 다.
func baro_req_tabs(db *sql.DB, active string) string {
	count := baro_req_kind_count(db)

	kind_items := [][]string{
		{"/baro/req", tool.Get_language(db, "contribution_all", true)},
	}
	for _, one := range baro_req_kind_list() {
		label := one.Name
		if number := count[one.Key]; number > 0 {
			label += " " + tool.Baro_comma(number)
		}

		kind_items = append(kind_items, []string{"/baro/req/kind/" + one.Key, label})
	}

	status_items := [][]string{}
	for _, row := range baro_req_status_list() {
		status_items = append(status_items, []string{"/baro/req/list/" + row[0], tool.Get_language(db, row[1], true)})
	}
	status_items = append(status_items, []string{"/baro/req/log", tool.Get_language(db, "edit_request_handle_record", true)})

	return tool.Baro_tabs(kind_items, active) + tool.Baro_tabs(status_items, active)
}

// baro_req_new_bar : 종류마다 「만들기」 단추를 한 줄로
func baro_req_new_bar(doc_name string) string {
	tail := ""
	if doc_name != "" {
		tail = "&doc=" + tool.Url_parser(doc_name)
	}

	html := ""
	for index, one := range baro_req_kind_list() {
		kind_style := ""
		if index == 0 {
			kind_style = "primary"
		}

		html += tool.Baro_button("/baro/req/new?kind="+one.Key+tail, one.Name+tool.Baro_t(" 만들기"), kind_style) + " "
	}

	return strings.TrimSpace(html)
}

// View_baro_req_list : 요청 목록 (편집 · 이동 · 삭제 · 되돌림을 한자리에서)
func View_baro_req_list(config tool.Config, status string, page string) string {
	return view_baro_req_list_kind(config, status, "", page)
}

// View_baro_req_kind_list : 종류로 걸러 본 요청 목록
func View_baro_req_kind_list(config tool.Config, kind string, page string) string {
	return view_baro_req_list_kind(config, "", baro_req_kind_normalize(kind), page)
}

func view_baro_req_list_kind(config tool.Config, status string, kind string, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_num := baro_req_page_no(page)
	offset := (page_num - 1) * baro_req_page_size

	query := "select id, title, user, summary, status, date, handler, kind, target from baro_edit_req"
	conditions := []string{}
	values := []any{}

	if status != "" {
		conditions = append(conditions, " status = ?")
		values = append(values, status)
	}
	if kind_where, kind_values := baro_req_kind_where(kind); kind_where != "" {
		conditions = append(conditions, kind_where)
		values = append(values, kind_values...)
	}
	if len(conditions) > 0 {
		query += " where" + strings.Join(conditions, " and")
	}
	query += " order by date desc limit ?, ?"
	values = append(values, offset, baro_req_page_size)

	rows := tool.Query_DB(db, query, values...)
	defer rows.Close()

	req_filter := tool.New_acl_render_filter(db, config.IP)
	list := [][]string{}
	count := 0
	hidden := 0
	for rows.Next() {
		id, title, user_name, summary, one_status, date, handler, one_kind, target := "", "", "", "", "", "", "", "", ""
		if rows.Scan(&id, &title, &user_name, &summary, &one_status, &date, &handler, &one_kind, &target) != nil {
			continue
		}

		// 쪽 넘기기는 걸러 내기 전 줄 수로 센다 (걸러진 만큼 「다음」 이 사라지면 안 된다)
		count++

		// 읽을 수 없는 문서의 요청은 목록에서도 뺀다 (본인 요청은 보인다)
		if user_name != config.IP && !req_filter.Allow(title) {
			hidden++

			continue
		}

		detail := baro_req_esc(tool.Baro_trunc(summary, 60))
		if target != "" {
			detail = `<span class="bw_muted">` + baro_req_esc(baro_req_target_label(one_kind)) + ` : </span>` +
				baro_req_esc(tool.Baro_trunc(target, 60)) + `<br>` + detail
		}

		list = append(list, []string{
			`<a href="/baro/req/view/` + tool.Url_parser(id) + `">#` + baro_req_esc(id) + `</a>`,
			baro_req_kind_badge(one_kind),
			tool.Baro_doc_link(title),
			tool.Baro_user_link(db, user_name, config.IP),
			detail,
			baro_req_status_badge(db, one_status),
			tool.Baro_time_cell(date),
		})
	}
	rows.Close()

	hero_title := tool.Get_language(db, "request_center", true)
	hero_note := tool.Baro_t("문서를 직접 고치기 어려울 때 편집 · 이동 · 삭제 · 되돌림을 부탁하는 곳입니다.")
	if kind != "" {
		one := baro_req_kind_find(kind)
		hero_title = one.Name
		hero_note = one.Note
	}

	body := tool.Baro_hero(hero_title, hero_note, baro_req_new_bar(""))

	active := "/baro/req"
	if status != "" {
		active = "/baro/req/list/" + status
	}
	if kind != "" {
		active = "/baro/req/kind/" + kind
	}

	inner := baro_req_tabs(db, active)
	inner += tool.Baro_table(
		[]string{"번호", tool.Get_language(db, "type", true), tool.Get_language(db, "document", true),
			tool.Get_language(db, "writer", true), tool.Get_language(db, "why", true),
			tool.Get_language(db, "state", true), tool.Get_language(db, "date", true)},
		list,
		tool.Baro_t("요청이 없습니다."),
	)
	if hidden > 0 {
		inner += tool.Baro_note("읽을 권한이 없는 문서의 요청 " + tool.Baro_comma(hidden) + "건은 목록에서 뺐습니다.")
	}

	url_tail := "/baro/req/page/{}"
	if status != "" {
		url_tail = "/baro/req/list/" + tool.Url_parser(status) + "/{}"
	}
	if kind != "" {
		url_tail = "/baro/req/kind/" + tool.Url_parser(kind) + "/{}"
	}
	inner += tool.Baro_paging(page_num, count, baro_req_page_size, url_tail)

	body += tool.Baro_card("", inner)

	return tool.Baro_page(db, config, hero_title, body, baro_req_menu())
}

// View_baro_req_new : 요청 만들기 (편집 · 이동 · 삭제 · 되돌림)
func View_baro_req_new(config tool.Config, doc_name string, values url.Values) string {
	return View_baro_req_new_kind(config, "edit", doc_name, values)
}

// View_baro_req_new_kind : 종류를 정해 요청 만들기
func View_baro_req_new_kind(config tool.Config, kind string, doc_name string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	kind = baro_req_kind_normalize(kind)
	kind_row := baro_req_kind_find(kind)

	if !tool.Baro_is_user(config.IP) {
		return tool.Baro_deny(db, config, kind_row.Name+tool.Baro_t("은 가입한 사용자만 만들 수 있습니다."))
	}

	if baro_reason, baro_ok := tool.Baro_hook_can_act(db, config, "edit"); !baro_ok {
		return tool.Baro_deny(db, config, baro_reason)
	}

	title := tool.Namespace_normalize(strings.TrimSpace(doc_name))

	if values != nil {
		// 폼 안에 든 종류를 우선한다 (주소만 바꿔치기하는 것을 막는다)
		if form_kind := strings.TrimSpace(values.Get("kind")); form_kind != "" {
			kind = baro_req_kind_normalize(form_kind)
			kind_row = baro_req_kind_find(kind)
		}

		title = tool.Namespace_normalize(strings.TrimSpace(values.Get("title")))
		data := strings.ReplaceAll(values.Get("data"), "\r", "")
		target := tool.Baro_one_line(values.Get("target"))
		summary := tool.Baro_one_line(values.Get("summary"))

		// 밝혔으면 사유 뒤에 표시를 붙인다. 승인되어 문서에 반영될 때 그대로 편집 요약이 된다.
		summary = tool.Baro_ai_send(summary, tool.Baro_ai_on(values.Get(tool.Baro_ai_field)))

		if title == "" {
			return tool.Baro_deny(db, config, tool.Baro_t("문서 이름을 적어 주세요."))
		}
		if !baro_req_can_make(db, config, title) {
			return tool.Baro_deny(db, config, tool.Baro_t("이 문서에는 요청을 만들 수 없습니다."))
		}

		switch kind {
		case "edit":
			if strings.TrimSpace(data) == "" {
				return tool.Baro_deny(db, config, tool.Baro_t("바꿀 본문을 적어 주세요."))
			}
			if !tool.Do_edit_max_length_check(db, config, data) {
				return tool.Baro_deny(db, config, tool.Baro_t("본문이 너무 깁니다."))
			}
			target = ""

		case "move":
			data = ""
			target = tool.Namespace_normalize(strings.TrimSpace(target))
			if target == "" {
				return tool.Baro_deny(db, config, tool.Baro_t("옮길 새 이름을 적어 주세요."))
			}
			if target == title {
				return tool.Baro_deny(db, config, tool.Baro_t("지금 이름과 같습니다."))
			}
			if !tool.Do_title_length_check(db, target, "document") {
				return tool.Baro_deny(db, config, tool.Baro_t("새 이름이 너무 깁니다."))
			}

		case "delete":
			data = ""
			target = ""
			if summary == "" {
				return tool.Baro_deny(db, config, tool.Baro_t("삭제 요청은 사유를 반드시 적어야 합니다."))
			}

		case "revert":
			data = ""
			target = strings.TrimPrefix(strings.TrimSpace(target), "r")
			if target == "" {
				return tool.Baro_deny(db, config, tool.Baro_t("되돌릴 판 번호를 적어 주세요."))
			}

			check := ""
			if !tool.QueryRow_DB(db, "select id from history where title = ? and id = ?", []any{&check}, title, target) {
				return tool.Baro_deny(db, config, tool.Baro_t("그 판을 찾지 못했습니다."))
			}
			if summary == "" {
				return tool.Baro_deny(db, config, tool.Baro_t("되돌림 요청은 사유를 반드시 적어야 합니다."))
			}
		}

		id := tool.Baro_next_id(db, "baro_edit_req")
		base_rev := tool.Get_document_revision(db, title)

		tool.Exec_DB(
			db,
			"insert into baro_edit_req (id, title, base_rev, user, data, summary, status, handler, handle_date, reason, date, new_rev, kind, target)"+
				" values (?, ?, ?, ?, ?, ?, 'open', '', '', '', ?, '', ?, ?)",
			id, title, base_rev, config.IP, data, summary, tool.Baro_now(), kind, target,
		)

		baro_req_log_add(db, id, title, config.IP, "create", kind_row.Name+" — "+summary)

		// 근거 공개 : 요청이 생겼다는 사실은 공개 기록으로 남긴다
		tool.Baro_public_log(db, "edit_request_create", config.IP, title, summary, kind_row.Name+" "+id)

		// 처리할 수 있는 사람이 볼 수 있도록 문서 주시자에게 알린다
		baro_req_notify_watcher(db, config.IP, title, id)

		return tool.Get_redirect("/baro/req/view/" + tool.Url_parser(id))
	}

	form := tool.Baro_hidden("kind", kind)
	form += tool.Baro_input("title", tool.Get_language(db, "document_name", true), tool.Namespace_display(title), tool.Baro_t("예) 바로위키"), tool.Baro_t("부탁하려는 문서 이름"))

	switch kind {
	case "edit":
		now_data := ""
		if title != "" {
			tool.QueryRow_DB(db, "select data from data where title = ?", []any{&now_data}, title)
		}
		form += tool.Baro_textarea("data", tool.Get_language(db, "content", true), now_data, 18, "", tool.Baro_t("이 내용이 그대로 저장됩니다. 지금 본문을 미리 넣어 두었습니다."))

	case "move":
		form += tool.Baro_input("target", baro_req_target_label(kind), "", tool.Baro_t("예) 바로위키/역사"), tool.Baro_t("옮겨 놓고 싶은 새 문서 이름입니다."))

	case "revert":
		now_rev := ""
		if title != "" {
			now_rev = tool.Get_document_revision(db, title)
		}
		form += tool.Baro_input("target", baro_req_target_label(kind), "", tool.Baro_t("예) 12"), tool.Baro_t("지금은 r")+now_rev+tool.Baro_t(" 입니다. 역사 화면에서 판 번호를 확인하세요."))
	}

	why_help := tool.Baro_t("처리하는 사람이 가장 먼저 읽는 줄입니다.")
	if kind == "delete" || kind == "revert" {
		why_help = tool.Baro_t("사유를 반드시 적어야 합니다. 그대로 공개됩니다.")
	}
	form += tool.Baro_input("summary", tool.Get_language(db, "why", true), "", tool.Baro_t("무엇을 왜 부탁하는지"), why_help)

	// AI 를 참고했는지 스스로 밝히는 칸
	form += tool.Baro_checkbox(tool.Baro_ai_field, tool.Baro_ai_text(), "yes", false, tool.Baro_ai_help())

	form += tool.Baro_submit(kind_row.Name+tool.Baro_t(" 만들기"), "primary")

	body := tool.Baro_hero(kind_row.Name+tool.Baro_t(" 만들기"), kind_row.Note, baro_req_new_bar(title))
	body += tool.Baro_card("", tool.Baro_form("/baro/req/new?kind="+kind, form))

	return tool.Baro_page(db, config, kind_row.Name, body, baro_req_menu())
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
		return tool.Baro_deny(db, config, tool.Baro_t("없는 요청입니다."))
	}

	title := row["title"]
	kind := row["kind"]
	kind_row := baro_req_kind_find(kind)

	// 읽을 수 없는 문서의 요청은 본문까지 실려 있으므로 보여 주지 않는다.
	// (요청을 만든 본인은 자기가 쓴 글이니 볼 수 있다)
	is_mine := config.IP == row["user"] && tool.Baro_is_user(config.IP)
	if !is_mine && !tool.Check_acl(db, title, "", "render", config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("이 문서를 읽을 권한이 없습니다."))
	}

	can_handle := baro_req_can_handle_kind(db, config, kind, title)

	if values != nil {
		action := values.Get("action")
		reason := tool.Baro_one_line(values.Get("reason"))
		is_admin := tool.Baro_is_admin(db, config.IP)

		// 잠금과 다시 열기는 이미 처리가 끝난 요청에도 쓰므로 상태 확인보다 앞에 둔다
		switch action {
		case "lock", "unlock":
			if !is_admin {
				return tool.Baro_deny(db, config, tool.Baro_t("관리자만 요청을 잠그고 풀 수 있습니다."))
			}

			new_lock := "1"
			if action == "unlock" {
				new_lock = "0"
			}
			if row["locked"] == new_lock {
				return tool.Get_redirect("/baro/req/view/" + tool.Url_parser(id))
			}

			tool.Exec_DB(db, "update baro_edit_req set locked = ? where id = ?", new_lock, id)
			baro_req_log_add(db, id, title, config.IP, action, reason)
			tool.Baro_public_log(db, "edit_request_"+action, config.IP, title, reason, kind_row.Name+" "+id)

			return tool.Get_redirect("/baro/req/view/" + tool.Url_parser(id))

		case "reopen":
			// 2번 원칙 : 거절되거나 지워진 요청도 본문은 남아 있으므로 다시 열 수 있다.
			// 다만 되풀이를 막으려 잠근 요청은 열지 않는다.
			if !is_mine {
				return tool.Baro_deny(db, config, tool.Baro_t("요청한 본인만 다시 열 수 있습니다."))
			}
			if baro_req_is_locked(row) {
				return tool.Baro_deny(db, config, tool.Baro_t("잠긴 요청은 다시 열 수 없습니다."))
			}
			if row["status"] != "rejected" && row["status"] != "deleted" {
				return tool.Baro_deny(db, config, tool.Baro_t("거절되거나 지워진 요청만 다시 열 수 있습니다."))
			}

			// 기준 판을 지금 판으로 새로 잡는다. 옛 기준 판으로 열어 두면
			// 승인하는 순간 그 사이의 편집이 소리 없이 덮여 버린다.
			tool.Exec_DB(
				db,
				"update baro_edit_req set status = 'open', handler = '', handle_date = '', reason = '', new_rev = '', base_rev = ?, date = ? where id = ?",
				tool.Get_document_revision(db, title), tool.Baro_now(), id,
			)
			baro_req_log_add(db, id, title, config.IP, "reopen", reason)
			tool.Baro_public_log(db, "edit_request_reopen", config.IP, title, reason, kind_row.Name+" "+id)
			baro_req_notify_watcher(db, config.IP, title, id)

			return tool.Get_redirect("/baro/req/view/" + tool.Url_parser(id))
		}

		if row["status"] != "open" {
			return tool.Baro_deny(db, config, tool.Baro_t("이미 처리된 요청입니다."))
		}
		if baro_req_is_locked(row) {
			return tool.Baro_deny(db, config, tool.Baro_t("잠긴 요청입니다. 관리자가 잠금을 풀어야 처리할 수 있습니다."))
		}

		switch action {
		case "accept":
			if !can_handle {
				return tool.Baro_deny(db, config, tool.Baro_t("이 문서에 대해 그 권한이 있는 사람만 승인할 수 있습니다."))
			}

			// 같은 요청을 두 사람이 동시에 승인해도 한 번만 반영되게 한다.
			// 문서 편집 잠금을 잡은 뒤 상태를 다시 읽어 확인한다.
			unlock := tool.Lock_document_edit(title)

			if now_status := baro_req_status_now(db, id); now_status != "open" {
				unlock()

				return tool.Baro_deny(db, config, tool.Baro_t("이미 처리된 요청입니다."))
			}

			new_rev, apply_error := baro_req_apply_kind(db, config, row, reason)
			if apply_error != "" {
				unlock()

				return tool.Baro_deny(db, config, apply_error)
			}

			tool.Exec_DB(
				db,
				"update baro_edit_req set status = 'accepted', handler = ?, handle_date = ?, reason = ?, new_rev = ? where id = ?",
				config.IP, tool.Baro_now(), reason, new_rev, id,
			)
			unlock()
			baro_req_log_add(db, id, title, config.IP, "accept", kind_row.Name+" — "+reason)
			tool.Baro_public_log(db, "edit_request_accept", config.IP, title, reason, kind_row.Name+" "+id)
			tool.Baro_notify(
				db, row["user"], "review",
				tool.Namespace_display(title)+" "+kind_row.Name+tool.Baro_t("이 승인됐습니다."),
				"/baro/req/view/"+tool.Url_parser(id), "",
			)

		case "reject":
			if !can_handle {
				return tool.Baro_deny(db, config, tool.Baro_t("이 문서에 대해 그 권한이 있는 사람만 거절할 수 있습니다."))
			}
			if reason == "" {
				return tool.Baro_deny(db, config, tool.Baro_t("거절 사유를 적어 주세요."))
			}

			tool.Exec_DB(
				db,
				"update baro_edit_req set status = 'rejected', handler = ?, handle_date = ?, reason = ? where id = ?",
				config.IP, tool.Baro_now(), reason, id,
			)
			baro_req_log_add(db, id, title, config.IP, "reject", kind_row.Name+" — "+reason)
			tool.Baro_public_log(db, "edit_request_reject", config.IP, title, reason, kind_row.Name+" "+id)
			tool.Baro_notify(
				db, row["user"], "review",
				tool.Namespace_display(title)+" "+kind_row.Name+tool.Baro_t("이 거절됐습니다."),
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

	state_html := baro_req_status_badge(db, row["status"])
	if baro_req_is_locked(row) {
		state_html += " " + tool.Baro_badge(tool.Baro_t("잠김"), "danger")
	}

	info := [][]string{
		{tool.Get_language(db, "type", true), baro_req_kind_badge(kind)},
		{tool.Get_language(db, "document", true), tool.Baro_doc_link(title)},
		{tool.Get_language(db, "writer", true), tool.Baro_user_link(db, row["user"], config.IP)},
		{tool.Get_language(db, "state", true), state_html},
		{tool.Get_language(db, "why", true), baro_req_esc(row["summary"])},
		{tool.Get_language(db, "date", true), tool.Baro_time_cell(row["date"])},
		{tool.Baro_t("기준 판"), "r" + baro_req_esc(row["base_rev"]) + tool.Baro_t(" (지금 r") + baro_req_esc(now_rev) + ")"},
	}

	if row["target"] != "" {
		target_html := baro_req_esc(row["target"])
		switch kind {
		case "move":
			target_html = tool.Baro_doc_link(row["target"])
		case "revert":
			target_html = `<a href="/render/` + tool.Url_parser(row["target"]) + `/` + tool.Url_parser(title) + `">r` + baro_req_esc(row["target"]) + `</a>` +
				` <a href="/diff/` + tool.Url_parser(row["target"]) + `/` + tool.Url_parser(now_rev) + `/` + tool.Url_parser(title) + `">(` + tool.Get_language(db, "compare", true) + `)</a>`
		}

		info = append(info, []string{baro_req_target_label(kind), target_html})
	}

	if row["handler"] != "" {
		info = append(info, []string{tool.Baro_t("처리한 사람"), tool.Baro_user_link(db, row["handler"], config.IP)})
		info = append(info, []string{tool.Baro_t("처리 시각"), tool.Baro_time_cell(row["handle_date"])})
	}
	if row["reason"] != "" {
		info = append(info, []string{tool.Baro_t("처리 사유"), baro_req_esc(row["reason"])})
	}
	if row["new_rev"] != "" {
		info = append(info, []string{"반영된 판", `<a href="/render/` + tool.Url_parser(row["new_rev"]) + `/` + tool.Url_parser(title) + `">r` + baro_req_esc(row["new_rev"]) + `</a>`})
	}

	body := tool.Baro_hero(kind_row.Name+" #"+baro_req_esc(id), tool.Namespace_display(title), "")
	body += tool.Baro_card("", tool.Baro_dl(info))

	if row["status"] == "open" && kind == "edit" && row["base_rev"] != now_rev {
		body += tool.Baro_alert("warn",
			tool.Baro_t("이 요청을 만든 뒤에 문서가 다시 편집됐습니다. (r")+baro_req_esc(row["base_rev"])+
				" → r"+baro_req_esc(now_rev)+tool.Baro_t(") 승인하면 그 사이의 편집은 이 본문으로 덮어써집니다."))
	}

	if kind == "edit" {
		body += tool.Baro_card(tool.Get_language(db, "content", true), `<pre class="bw_pre">`+baro_req_esc(row["data"])+`</pre>`)
	}

	view_is_admin := tool.Baro_is_admin(db, config.IP)
	view_is_locked := baro_req_is_locked(row)

	if view_is_locked {
		body += tool.Baro_alert("danger", tool.Baro_t("잠긴 요청입니다. 관리자가 잠금을 풀기 전에는 처리할 수도, 다시 열 수도 없습니다."))
	}

	if row["status"] == "open" && !view_is_locked {
		form := ""
		if can_handle {
			form += tool.Baro_input("reason", tool.Get_language(db, "why", true), "", "", tool.Baro_t("거절할 때는 사유가 반드시 필요합니다."))
			form += tool.Baro_hidden("action", "accept")
			form += tool.Baro_submit(kind_row.Verb, "primary")

			body += tool.Baro_card(tool.Get_language(db, "approve_or_decline", true), tool.Baro_form("/baro/req/view/"+tool.Url_parser(id), form))

			reject := tool.Baro_input("reason", tool.Get_language(db, "why", true), "", "", "")
			reject += tool.Baro_hidden("action", "reject")
			reject += tool.Baro_submit(tool.Get_language(db, "edit_request_reject", true), "danger")

			body += tool.Baro_card("", tool.Baro_form("/baro/req/view/"+tool.Url_parser(id), reject))
		}

		if is_mine || can_handle || view_is_admin {
			del := tool.Baro_input("reason", tool.Get_language(db, "why", true), "", "", "")
			del += tool.Baro_hidden("action", "delete")
			del += tool.Baro_submit(tool.Get_language(db, "edit_request_delete", true), "danger")

			body += tool.Baro_danger_zone(tool.Get_language(db, "edit_request_delete", true),
				tool.Baro_form("/baro/req/view/"+tool.Url_parser(id), del))
		}
	}

	// 다시 열기 : 거절되거나 지워진 내 요청을 되살린다 (2번 원칙 — 본문은 지워지지 않았다)
	if is_mine && !view_is_locked && (row["status"] == "rejected" || row["status"] == "deleted") {
		again := tool.Baro_input("reason", tool.Get_language(db, "why", true), "", "",
			tool.Baro_t("기준 판은 지금 판으로 새로 잡힙니다."))
		again += tool.Baro_hidden("action", "reopen")
		again += tool.Baro_submit(tool.Baro_t("다시 열기"), "primary")

		body += tool.Baro_card(tool.Baro_t("다시 열기"), tool.Baro_form("/baro/req/view/"+tool.Url_parser(id), again))
	}

	// 잠금 : 같은 부탁이 되풀이해 올라올 때 관리자가 멈춘다
	if view_is_admin {
		lock_action := "lock"
		lock_name := tool.Baro_t("잠금")
		lock_help := tool.Baro_t("잠그면 요청한 본인도 다시 열 수 없습니다.")
		if view_is_locked {
			lock_action = "unlock"
			lock_name = tool.Baro_t("잠금 해제")
			lock_help = tool.Baro_t("잠금을 풀면 다시 처리할 수 있습니다.")
		}

		lock_form := tool.Baro_input("reason", tool.Get_language(db, "why", true), "", "", lock_help)
		lock_form += tool.Baro_hidden("action", lock_action)
		lock_form += tool.Baro_submit(lock_name, "danger")

		body += tool.Baro_danger_zone(lock_name, tool.Baro_form("/baro/req/view/"+tool.Url_parser(id), lock_form))
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
		"select id, user, summary, status, date, kind, target from baro_edit_req where title = ? order by date desc limit ?, ?",
		title, offset, baro_req_page_size,
	)
	defer rows.Close()

	list := [][]string{}
	count := 0
	for rows.Next() {
		id, user_name, summary, status, date, kind, target := "", "", "", "", "", "", ""
		if rows.Scan(&id, &user_name, &summary, &status, &date, &kind, &target) != nil {
			continue
		}

		detail := baro_req_esc(tool.Baro_trunc(summary, 60))
		if target != "" {
			detail = `<span class="bw_muted">` + baro_req_esc(baro_req_target_label(kind)) + ` : </span>` +
				baro_req_esc(tool.Baro_trunc(target, 60)) + `<br>` + detail
		}

		count++
		list = append(list, []string{
			`<a href="/baro/req/view/` + tool.Url_parser(id) + `">#` + baro_req_esc(id) + `</a>`,
			baro_req_kind_badge(kind),
			tool.Baro_user_link(db, user_name, config.IP),
			detail,
			baro_req_status_badge(db, status),
			tool.Baro_time_cell(date),
		})
	}
	rows.Close()

	body := tool.Baro_hero(tool.Get_language(db, "request_center", true), tool.Namespace_display(title), baro_req_new_bar(title))

	inner := tool.Baro_table(
		[]string{"번호", tool.Get_language(db, "type", true), tool.Get_language(db, "writer", true),
			tool.Get_language(db, "why", true), tool.Get_language(db, "state", true), tool.Get_language(db, "date", true)},
		list, tool.Baro_t("요청이 없습니다."),
	)
	inner += tool.Baro_paging(page_num, count, baro_req_page_size, "/baro/req/doc/"+tool.Url_parser(title)+"/{}")

	body += tool.Baro_card("", inner)

	return tool.Baro_page(db, config, tool.Get_language(db, "edit_request_list", true), body, baro_req_menu())
}
