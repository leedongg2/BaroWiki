package route

// 바로위키 8-3 프로젝트 토론
//
// 실시간 채팅이 아니라 검색되고 기록되는 스레드형 토론이다.
// 저장은 baro_workspace 를 쓴다.
//
//	project = "projecttalk:<프로젝트id>"
//	title   = 스레드 제목
//	data    = 본문
//	user    = 글쓴이
//	note    = "kind:<종류>" (답글이면 "re:<부모 id>")
//	status  = open (삭제하면 closed)

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// baro_community_kind_list : 프로젝트 토론의 종류
func baro_community_kind_list() [][]string {
	return [][]string{
		{"notice", tool.Baro_t("공지")},
		{"idea", tool.Baro_t("제안")},
		{"source", tool.Baro_t("자료 공유")},
		{"assign", tool.Baro_t("작업 분담")},
		{"policy", tool.Baro_t("정책 합의")},
		{"minutes", tool.Baro_t("회의록")},
	}
}

// baro_community_kind_name : 종류 표시 이름
func baro_community_kind_name(kind string) string {
	for _, row := range baro_community_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	if kind == "" {
		return "기타"
	}

	return kind
}

// baro_community_kind_ok : 아는 종류인지
func baro_community_kind_ok(kind string) bool {
	for _, row := range baro_community_kind_list() {
		if row[0] == kind {
			return true
		}
	}

	return false
}

// baro_community_project_key : baro_workspace 에 쓰는 프로젝트 토론 키
func baro_community_project_key(project_id string) string {
	return "projecttalk:" + project_id
}

// baro_community_project_name : 프로젝트 이름 (없으면 id 그대로)
func baro_community_project_name(db *sql.DB, project_id string) string {
	name := ""
	tool.QueryRow_DB(db, "select name from baro_project where id = ?", []any{&name}, project_id)
	if name == "" {
		return project_id
	}

	return name
}

// baro_community_project_can_write : 프로젝트 참여자만 글을 쓸 수 있다
func baro_community_project_can_write(db *sql.DB, project_id string, ip string) bool {
	if !tool.Baro_is_user(ip) {
		return false
	}

	owner := ""
	tool.QueryRow_DB(db, "select owner from baro_project where id = ?", []any{&owner}, project_id)
	if owner != "" && owner == ip {
		return true
	}

	status := ""
	found := tool.QueryRow_DB(
		db,
		"select status from baro_project_member where project = ? and user = ? order by id + 0 desc limit 1",
		[]any{&status}, project_id, ip,
	)
	if found && status != "closed" && status != "rejected" {
		return true
	}

	return tool.Baro_can(db, ip, "project")
}

// View_baro_community_talk_project : 8-3 프로젝트 토론
func View_baro_community_talk_project(config tool.Config, project_id string, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	project := baro_community_trim_path(project_id)
	if project == "" {
		return baro_community_project_index(db, config, page)
	}

	name, field, status, owner := "", "", "", ""
	exist := tool.QueryRow_DB(
		db,
		"select name, field, status, owner from baro_project where id = ?",
		[]any{&name, &field, &status, &owner}, project,
	)

	if !exist {
		return tool.Baro_deny(db, config, tool.Baro_t("그런 프로젝트가 없습니다."))
	}

	key := baro_community_project_key(project)
	action := "/baro/talk/project/" + tool.Url_parser(project)

	// ---------- 쓰기 처리 ----------
	if values != nil {
		return baro_community_talk_project_post(db, config, project, values)
	}

	kind := baro_community_get(query, "kind")
	find := baro_community_get(query, "q")
	page_int := baro_community_page_int(page)
	offset := (page_int - 1) * baro_community_page_size

	// ---------- 목록 ----------
	posts := []baro_community_post{}

	sql_text := "select id, title, data, user, date, status, note from baro_workspace where project = ? and note not like 're:%'"
	args := []any{key}

	if baro_community_kind_ok(kind) {
		sql_text += " and note = ?"
		args = append(args, "kind:"+kind)
	}

	if find != "" {
		sql_text += " and (title like ? or data like ?)"
		args = append(args, "%"+find+"%", "%"+find+"%")
	}

	sql_text += " order by id + 0 desc limit ?, ?"
	args = append(args, offset, baro_community_page_size)

	rows := tool.Query_DB(db, sql_text, args...)
	defer rows.Close()

	for rows.Next() {
		row := baro_community_post{}
		if err := rows.Scan(&row.Id, &row.Title, &row.Data, &row.User, &row.Date, &row.Status, &row.Note); err != nil {
			continue
		}

		posts = append(posts, row)
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/talk", tool.Baro_t("토론")},
		{"", name + " 프로젝트 토론"},
	})

	body += tool.Baro_hero(
		name+" 프로젝트 토론",
		tool.Baro_t("공지 · 제안 · 자료 공유 · 작업 분담 · 정책 합의 · 회의록을 스레드로 남깁니다."),
		tool.Baro_button("/baro/project/view/"+tool.Url_parser(project), "프로젝트 화면", "primary")+" "+
			tool.Baro_button("/baro/watch/add?kind=project&target="+tool.Url_parser(project), "이 프로젝트 주시", ""),
	)

	body += tool.Baro_kv_grid([][]string{
		{"분야", tool.HTML_escape(tool.Baro_field_name(db, field))},
		{"상태", tool.Baro_badge(status, "info")},
		{tool.Baro_t("운영자"), tool.Baro_user_link(db, owner, config.IP)},
	})

	// 종류별 탭
	tabs := [][]string{{action, tool.Baro_t("전체")}}
	for _, row := range baro_community_kind_list() {
		tabs = append(tabs, []string{action + "?kind=" + row[0], row[1]})
	}

	active := action
	if baro_community_kind_ok(kind) {
		active = action + "?kind=" + kind
	}
	body += tool.Baro_tabs(tabs, active)

	// 검색
	find_form := tool.Baro_input("q", tool.Baro_t("토론 검색"), find, tool.Baro_t("제목이나 본문에 들어간 말"), tool.Baro_t("프로젝트 토론은 검색되고 기록으로 남습니다."))
	if baro_community_kind_ok(kind) {
		find_form += tool.Baro_hidden("kind", kind)
	}
	find_form += tool.Baro_submit(tool.Baro_t("검색"), "")
	body += tool.Baro_card(tool.Baro_t("검색"), `<form class="bw_form" method="get" action="`+tool.HTML_escape(action)+`">`+find_form+`</form>`)

	// 종류별 개수
	stats := [][]string{}
	for _, row := range baro_community_kind_list() {
		count := 0
		tool.QueryRow_DB(
			db,
			"select count(*) from baro_workspace where project = ? and note = ? and status = 'open'",
			[]any{&count}, key, "kind:"+row[0],
		)
		stats = append(stats, []string{row[1], tool.Baro_itoa(count), ""})
	}
	body += tool.Baro_stat_row(stats)

	list_html := ""
	for _, row := range posts {
		subject := row.Title
		if subject == "" {
			subject = "제목 없음"
		}
		subject = "[" + baro_community_kind_name(strings.TrimPrefix(row.Note, "kind:")) + "] " + subject

		list_html += baro_community_post_block(db, config, key, action, row, subject, 0)
		for _, reply := range baro_community_replies(db, key, row.Id) {
			list_html += baro_community_post_block(db, config, key, action, reply, "", 1)
		}
	}

	if list_html == "" {
		list_html = tool.Baro_empty(tool.Baro_t("조건에 맞는 토론이 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("토론 목록"), list_html)

	paging_url := action + "?page={}"
	if baro_community_kind_ok(kind) {
		paging_url += "&kind=" + tool.Url_parser(kind)
	}
	if find != "" {
		paging_url += "&q=" + tool.Url_parser(find)
	}
	body += tool.Baro_paging(page_int, len(posts), baro_community_page_size, paging_url)

	// 글쓰기
	if block := baro_community_write_block(db); block != "" {
		body += tool.Baro_alert("warn", block)
	} else if baro_community_project_can_write(db, project, config.IP) {
		options := baro_community_kind_list()

		form := tool.Baro_hidden("act", "write")
		form += tool.Baro_select("kind", "종류", options, kind, tool.Baro_t("공지·제안·자료·분담·정책·회의록 중에서 고릅니다."))
		form += tool.Baro_input("subject", tool.Baro_t("제목"), "", tool.Baro_t("무슨 이야기인지"), "")
		form += tool.Baro_textarea("data", tool.Baro_t("본문"), "", 7, tool.Baro_t("@이름 을 쓰면 그 사용자에게 알림이 갑니다."), tool.Baro_t("실시간 대화가 아니라 나중에 찾아볼 기록입니다."))
		form += tool.Baro_submit(tool.Baro_t("스레드 열기"), "primary")

		body += tool.Baro_card(tool.Baro_t("새 스레드"), tool.Baro_form(action, form))
	} else if tool.Baro_is_user(config.IP) {
		body += tool.Baro_alert("info", tool.Baro_t("읽기는 누구나 할 수 있지만 글쓰기는 이 프로젝트 참여자만 할 수 있습니다."))
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("글을 쓰려면 로그인하고 프로젝트에 참여해야 합니다."))
	}

	body += tool.Baro_note(tool.Baro_t("합의된 내용은 회의록 종류로 정리해 남겨 주세요. 나중에 검색으로 찾을 수 있어야 합니다."))

	return tool.Baro_page(db, config, name+" 프로젝트 토론", body, baro_community_menu())
}

// baro_community_project_index : 토론이 열려 있는 프로젝트 목록
func baro_community_project_index(db *sql.DB, config tool.Config, page string) string {
	rows_html := [][]string{}

	page_int := baro_community_page_int(page)
	offset := (page_int - 1) * baro_community_page_size

	rows := tool.Query_DB(
		db,
		"select id, name, field, status, owner from baro_project order by id + 0 desc limit ?, ?",
		offset, baro_community_page_size,
	)
	defer rows.Close()

	for rows.Next() {
		id, name, field, status, owner := "", "", "", "", ""
		if err := rows.Scan(&id, &name, &field, &status, &owner); err != nil {
			continue
		}

		count := 0
		tool.QueryRow_DB(
			db,
			"select count(*) from baro_workspace where project = ? and status = 'open' and note not like 're:%'",
			[]any{&count}, baro_community_project_key(id),
		)

		rows_html = append(rows_html, []string{
			`<a href="/baro/talk/project/` + tool.Url_parser(id) + `">` + tool.HTML_escape(name) + `</a>`,
			tool.HTML_escape(tool.Baro_field_name(db, field)),
			tool.Baro_badge(status, "info"),
			tool.Baro_user_link(db, owner, config.IP),
			tool.Baro_itoa(count),
		})
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/talk", tool.Baro_t("토론")},
		{"", "프로젝트 토론"},
	})
	body += tool.Baro_hero(
		"프로젝트 토론",
		tool.Baro_t("프로젝트를 골라 그 안의 스레드형 토론으로 들어갑니다."),
		tool.Baro_button("/baro/project", tool.Baro_t("프로젝트 목록 보기"), ""),
	)
	body += baro_community_talk_tabs("/baro/talk/project/")
	body += tool.Baro_card(
		tool.Baro_t("프로젝트 목록"),
		tool.Baro_table([]string{"프로젝트", "분야", "상태", tool.Baro_t("운영자"), tool.Baro_t("토론 글 수")}, rows_html, tool.Baro_t("아직 프로젝트가 없습니다.")),
	)
	body += tool.Baro_paging(page_int, len(rows_html), baro_community_page_size, "/baro/talk/project/?page={}")
	body += tool.Baro_note(tool.Baro_t("프로젝트 토론은 실시간 대화가 아니라 검색되고 남는 스레드형 기록입니다."))

	return tool.Baro_page(db, config, "프로젝트 토론", body, baro_community_menu())
}

// baro_community_talk_project_post : 프로젝트 토론 쓰기 처리
func baro_community_talk_project_post(db *sql.DB, config tool.Config, project string, values url.Values) string {
	key := baro_community_project_key(project)
	back := "/baro/talk/project/" + tool.Url_parser(project)

	if wall, need := tool.Baro_login_wall(db, config, back); need {
		return wall
	}

	if block := baro_community_write_block(db); block != "" {
		return tool.Baro_deny(db, config, block)
	}

	act := baro_community_get(values, "act")

	if act == "delete" {
		id := baro_community_get(values, "id")
		writer, row_project := "", ""
		if !tool.QueryRow_DB(
			db,
			"select user, project from baro_workspace where id = ?",
			[]any{&writer, &row_project}, id,
		) || row_project != key {
			return tool.Baro_deny(db, config, tool.Baro_t("그런 글이 없습니다."))
		}

		if !baro_community_post_can_delete(db, config, writer) {
			return tool.Baro_deny(db, config, tool.Baro_t("본인 글과 관리자만 지울 수 있습니다."))
		}

		tool.Exec_DB(db, "update baro_workspace set status = 'closed' where id = ?", id)
		tool.Baro_public_log(db, "projecttalk_delete", config.IP, project, "", "프로젝트 토론 글 삭제 #"+id)
		tool.Baro_audit(db, config, "projecttalk_delete", key+"#"+id, writer, "closed", "프로젝트 토론 글 삭제")

		return tool.Get_redirect(back)
	}

	if !baro_community_project_can_write(db, project, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("프로젝트 참여자만 글을 쓸 수 있습니다. 읽기는 누구나 할 수 있습니다."))
	}

	if tool.Baro_sanction_has(db, config.IP, "talk_block", "") {
		return tool.Baro_deny(db, config, tool.Baro_t("토론 참여가 제한된 상태입니다."))
	}

	if !tool.Baro_rate_ok(db, "community_projecttalk", config.IP, 30, 600) {
		return tool.Baro_deny(db, config, tool.Baro_t("짧은 시간에 너무 많은 글을 남겼습니다. 잠시 뒤에 다시 시도해 주세요."))
	}

	data := tool.Baro_trunc(baro_community_get(values, "data"), 8000)
	if data == "" {
		return tool.Baro_deny(db, config, tool.Baro_t("본문을 적어 주세요."))
	}

	id := tool.Baro_next_id(db, "baro_workspace")
	title := ""
	note := ""

	switch act {
	case "write":
		kind := baro_community_get(values, "kind")
		if !baro_community_kind_ok(kind) {
			kind = "idea"
		}

		title = tool.Baro_trunc(baro_community_subject_norm(baro_community_get(values, "subject")), 200)
		if title == "" {
			title = tool.Baro_trunc(tool.Baro_one_line(data), 60)
		}

		note = "kind:" + kind

	case "reply":
		parent := baro_community_get(values, "parent")
		parent_project, parent_title := "", ""
		if !tool.QueryRow_DB(
			db,
			"select project, title from baro_workspace where id = ?",
			[]any{&parent_project, &parent_title}, parent,
		) || parent_project != key {
			return tool.Baro_deny(db, config, tool.Baro_t("답글을 달 글이 없습니다."))
		}

		title = parent_title
		note = baro_community_reply_note(parent)

	default:
		return tool.Get_redirect(back)
	}

	tool.Exec_DB(
		db,
		"insert into baro_workspace (id, project, title, data, user, date, status, note) values (?, ?, ?, ?, ?, ?, 'open', ?)",
		id, key, title, data, config.IP, tool.Baro_now(), note,
	)

	// 본문 속 @언급 알림
	baro_community_notify_mentions(db, data, config.IP, tool.Baro_t("프로젝트 토론에서 언급되었습니다"), back)

	// 프로젝트를 주시하는 사람에게 알림
	baro_community_notify_watchers(db, "project", project, config.IP,
		baro_community_project_name(db, project)+tool.Baro_t(" 프로젝트 토론에 새 글이 있습니다"), back)

	tool.Baro_metric_bump(db, "projecttalk_post", tool.Baro_today(), 1)

	return tool.Get_redirect(back)
}
