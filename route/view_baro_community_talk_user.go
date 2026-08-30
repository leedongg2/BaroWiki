package route

// 바로위키 8-2 사용자 토론
//
// 저장은 새 표를 만들지 않고 baro_workspace 를 약속대로 쓴다.
//
//	project = "usertalk"        (사용자 토론)
//	title   = 대상 사용자 이름
//	data    = 본문
//	user    = 글쓴이
//	note    = 제목 (답글이면 "re:<부모 id>")
//	status  = open (삭제하면 closed)

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// baro_community_reply_note : 답글에 적어 두는 부모 id 표시
func baro_community_reply_note(parent string) string {
	return "re:" + parent
}

// baro_community_is_reply : 답글 여부
func baro_community_is_reply(note string) bool {
	return strings.HasPrefix(note, "re:")
}

// baro_community_body_html : 본문을 안전하게 HTML 로 바꾼다 (@이름 은 프로필 링크)
//
//	먼저 HTML_escape 로 막고, 그 뒤에 @이름 만 정규식으로 한 번씩 바꾼다.
//	(어떤 이름이 다른 이름의 앞부분일 때 링크가 겹쳐 깨지던 문제를 막는다)
func baro_community_body_html(data string) string {
	safe := tool.HTML_escape(data)

	safe = baro_community_mention_re.ReplaceAllStringFunc(safe, func(match string) string {
		name := strings.Trim(strings.TrimPrefix(match, "@"), ".-")
		if name == "" {
			return match
		}

		return `<a class="bw_user" href="/baro/profile/` + tool.Url_parser(name) + `">@` + tool.HTML_escape(name) + `</a>`
	})

	return `<div class="bw_card_b">` + strings.ReplaceAll(safe, "\n", "<br>") + `</div>`
}

// baro_community_post : baro_workspace 에 저장된 글 한 개
type baro_community_post struct {
	Id     string
	Title  string
	Data   string
	User   string
	Date   string
	Status string
	Note   string
}

// baro_community_post_can_delete : 본인과 관리자만 지운다
//
//	8-2 명세대로 "본인과 관리자" 로만 제한한다.
//	위키 관리자와 제재·중재 역할만 남의 글을 지울 수 있고,
//	순찰 권한만 가진 사용자는 지우지 못한다. (지운 글도 기록은 남는다)
func baro_community_post_can_delete(db *sql.DB, config tool.Config, writer string) bool {
	if !tool.Baro_is_user(config.IP) {
		return false
	}

	if config.IP == writer {
		return true
	}

	if tool.Baro_is_admin(db, config.IP) || tool.Baro_is_owner(db, config.IP) {
		return true
	}

	return tool.Baro_has_role(db, config.IP, "sanction") || tool.Baro_has_role(db, config.IP, "mediator")
}

// baro_community_replies : 한 글에 달린 답글
func baro_community_replies(db *sql.DB, project string, parent string) []baro_community_post {
	out := []baro_community_post{}

	rows := tool.Query_DB(
		db,
		"select id, title, data, user, date, status, note from baro_workspace where project = ? and note = ? order by id + 0 asc",
		project, baro_community_reply_note(parent),
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_community_post{}
		if err := rows.Scan(&row.Id, &row.Title, &row.Data, &row.User, &row.Date, &row.Status, &row.Note); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_community_post_block : 글 하나를 카드로 그린다
func baro_community_post_block(db *sql.DB, config tool.Config, project string, action string, row baro_community_post, subject string, depth int) string {
	who := tool.Baro_user_link(db, row.User, config.IP)
	head := who + ` <span class="bw_muted">` + tool.Baro_time_cell(row.Date) + `</span>`

	body := ""
	if row.Status == "closed" {
		body = tool.Baro_alert("muted", tool.Baro_t("글쓴이 또는 관리자가 지운 글입니다. 기록은 남습니다."))
	} else {
		body = baro_community_body_html(row.Data)
	}

	foot := ""
	if row.Status != "closed" {
		if tool.Baro_is_user(config.IP) {
			reply := tool.Baro_hidden("act", "reply")
			reply += tool.Baro_hidden("parent", row.Id)
			reply += tool.Baro_textarea("data", tool.Baro_t("답글"), "", 3, tool.Baro_t("@이름 을 쓰면 그 사용자에게 알림이 갑니다."), "")
			reply += tool.Baro_submit(tool.Baro_t("답글 남기기"), "")
			foot += tool.Baro_t(`<details><summary>답글 쓰기</summary>`) + tool.Baro_form(action, reply) + `</details>`
		}

		if baro_community_post_can_delete(db, config, row.User) {
			del := tool.Baro_hidden("act", "delete")
			del += tool.Baro_hidden("id", row.Id)
			del += tool.Baro_submit(tool.Baro_t("이 글 지우기"), "danger")
			foot += tool.Baro_form(action, del)
		}

		foot += baro_community_report_link("post", project+"#"+row.Id, "신고")
	}

	class := "bw_card"
	if depth > 0 {
		class = "bw_card bw_reply"
	}

	html := `<section class="` + class + `">`
	if subject != "" {
		html += `<h2 class="bw_card_t">` + tool.HTML_escape(subject) + `</h2>`
	}
	html += `<p class="bw_card_s">` + head + `</p>`
	html += body
	if foot != "" {
		html += `<div class="bw_card_f">` + foot + `</div>`
	}

	return html + `</section>`
}

// View_baro_community_talk_user : 8-2 사용자 토론
func View_baro_community_talk_user(config tool.Config, user_name string, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	target := baro_community_trim_path(user_name)
	if target == "" {
		return tool.Get_redirect("/baro/talk")
	}

	if !tool.Baro_is_user(target) {
		return tool.Baro_deny(db, config, tool.Baro_t("사용자 토론은 로그인 계정에 대해서만 열립니다. (IP 는 대상이 되지 않습니다)"))
	}

	action := "/baro/talk/user/" + tool.Url_parser(target)

	// ---------- 쓰기 처리 ----------
	if values != nil {
		return baro_community_talk_user_post(db, config, target, values)
	}

	page_int := baro_community_page_int(page)
	offset := (page_int - 1) * baro_community_page_size

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/talk", tool.Baro_t("토론")},
		{"", target + tool.Baro_t(" 사용자 토론")},
	})

	body += tool.Baro_hero(
		target+tool.Baro_t(" 사용자 토론"),
		tool.Baro_t("이 사용자에게 남기는 공개 메시지입니다. 모두가 볼 수 있습니다."),
		tool.Baro_button("/baro/profile/"+tool.Url_parser(target), "기여 프로필", "primary")+" "+
			tool.Baro_button("/baro/talk", tool.Baro_t("토론 허브"), ""),
	)

	if !baro_community_user_exists(db, target) {
		body += tool.Baro_alert("warn", tool.Baro_t("가입 기록을 찾지 못한 이름입니다. 이름을 다시 확인해 주세요."))
	}

	// 신규 이용자 안내 (8-2)
	body += baro_community_welcome_box(db, target)
	body += baro_community_newbie_guard(db, target)

	// 경고 · 제재 기록 (공개 범위 안에서만)
	body += baro_community_sanction_box(db, target)

	// 글 목록
	posts := []baro_community_post{}
	rows := tool.Query_DB(
		db,
		"select id, title, data, user, date, status, note from baro_workspace where project = 'usertalk' and title = ? and note not like 're:%' order by id + 0 desc limit ?, ?",
		target, offset, baro_community_page_size,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_community_post{}
		if err := rows.Scan(&row.Id, &row.Title, &row.Data, &row.User, &row.Date, &row.Status, &row.Note); err != nil {
			continue
		}

		posts = append(posts, row)
	}

	list_html := ""
	for _, row := range posts {
		subject := row.Note
		if subject == "" {
			subject = "제목 없음"
		}

		list_html += baro_community_post_block(db, config, "usertalk", action, row, subject, 0)
		for _, reply := range baro_community_replies(db, "usertalk", row.Id) {
			list_html += baro_community_post_block(db, config, "usertalk", action, reply, "", 1)
		}
	}

	if list_html == "" {
		list_html = tool.Baro_empty(tool.Baro_t("아직 남겨진 메시지가 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("남겨진 메시지"), list_html)
	body += tool.Baro_paging(page_int, len(posts), baro_community_page_size, action+"?page={}")

	// 글쓰기
	if wall := baro_community_write_block(db); wall != "" {
		body += tool.Baro_alert("warn", wall)
	} else if tool.Baro_is_user(config.IP) {
		form := tool.Baro_hidden("act", "write")
		form += tool.Baro_input("subject", tool.Baro_t("제목"), baro_community_get(query, "subject"), tool.Baro_t("무슨 이야기인지"), "")
		form += tool.Baro_textarea("data", tool.Baro_t("본문"), "", 6, tool.Baro_t("@이름 을 쓰면 그 사용자에게 알림이 갑니다."), tool.Baro_t("공개 메시지입니다. 개인정보를 적지 마세요."))
		form += tool.Baro_submit(tool.Baro_t("메시지 남기기"), "primary")

		body += tool.Baro_card(tool.Baro_t("메시지 남기기"), tool.Baro_form(action, form))
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("메시지는 로그인한 사용자만 남길 수 있습니다."))
	}

	body += tool.Baro_note(tool.Baro_t("사용자 토론은 사람을 평가하는 자리가 아니라 문서 작업을 상의하는 자리입니다."))

	return tool.Baro_page(db, config, target+tool.Baro_t(" 사용자 토론"), body, baro_community_menu())
}

// baro_community_sanction_box : 공개 범위 안에서 보여주는 제재·경고 기록
//
//	사유는 보여주되 신고자 이름과 IP·이메일은 절대 보여주지 않는다. (0장 5번 원칙)
func baro_community_sanction_box(db *sql.DB, target string) string {
	rows := [][]string{}
	for _, row := range tool.Baro_sanction_active(db, target) {
		scope := row[2]
		if scope == "" {
			scope = tool.Baro_t("전체")
		}

		rows = append(rows, []string{
			tool.HTML_escape(tool.Baro_sanction_name(row[1])),
			tool.HTML_escape(scope),
			tool.HTML_escape(tool.Baro_trunc(row[3], 120)),
			tool.HTML_escape(tool.Baro_duration_left(row[4])),
		})
	}

	// 공개 로그에 남은 안내·경고
	log_rows := [][]string{}
	logs := tool.Query_DB(
		db,
		"select kind, reason, date from baro_public_log where target = ? and kind in ('warn', 'sanction', 'sanction_release', 'guide') order by id + 0 desc limit 10",
		target,
	)
	defer logs.Close()

	for logs.Next() {
		kind, reason, date := "", "", ""
		if err := logs.Scan(&kind, &reason, &date); err != nil {
			continue
		}

		log_rows = append(log_rows, []string{
			tool.HTML_escape(kind),
			tool.HTML_escape(tool.Baro_trunc(reason, 120)),
			tool.Baro_time_cell(date),
		})
	}

	if len(rows) == 0 && len(log_rows) == 0 {
		return ""
	}

	body := tool.Baro_table([]string{tool.Baro_t("제재 종류"), tool.Baro_t("범위"), "사유", tool.Baro_t("남은 기간")}, rows, tool.Baro_t("적용 중인 제재가 없습니다."))
	body += tool.Baro_section(tool.Baro_t("공개 기록"), tool.Baro_table([]string{"종류", "사유", "시각"}, log_rows, tool.Baro_t("공개 기록이 없습니다.")))
	body += tool.Baro_note(tool.Baro_t("신고자와 접속 정보는 공개하지 않습니다. 자세한 내용은 사건 화면에서 확인합니다."))

	return tool.Baro_card_ex(tool.Baro_t("경고 · 제재 기록"), tool.Baro_t("공개할 수 있는 범위만 보여 줍니다"), body, "")
}

// baro_community_talk_user_post : 사용자 토론 쓰기 처리
func baro_community_talk_user_post(db *sql.DB, config tool.Config, target string, values url.Values) string {
	back := "/baro/talk/user/" + tool.Url_parser(target)

	if wall, need := tool.Baro_login_wall(db, config, back); need {
		return wall
	}

	if block := baro_community_write_block(db); block != "" {
		return tool.Baro_deny(db, config, block)
	}

	act := baro_community_get(values, "act")

	if act == "delete" {
		id := baro_community_get(values, "id")
		writer, project := "", ""
		if !tool.QueryRow_DB(
			db,
			"select user, project from baro_workspace where id = ?",
			[]any{&writer, &project}, id,
		) || project != "usertalk" {
			return tool.Baro_deny(db, config, tool.Baro_t("그런 글이 없습니다."))
		}

		if !baro_community_post_can_delete(db, config, writer) {
			return tool.Baro_deny(db, config, tool.Baro_t("본인 글과 관리자만 지울 수 있습니다."))
		}

		tool.Exec_DB(db, "update baro_workspace set status = 'closed' where id = ?", id)
		tool.Baro_public_log(db, "usertalk_delete", config.IP, target, "", "사용자 토론 글 삭제 #"+id)
		tool.Baro_audit(db, config, "usertalk_delete", target+"#"+id, writer, "closed", "사용자 토론 글 삭제")

		return tool.Get_redirect(back)
	}

	if tool.Baro_sanction_has(db, config.IP, "talk_block", "") {
		return tool.Baro_deny(db, config, tool.Baro_t("토론 참여가 제한된 상태입니다."))
	}

	if !tool.Baro_rate_ok(db, "community_usertalk", config.IP, 20, 600) {
		return tool.Baro_deny(db, config, tool.Baro_t("짧은 시간에 너무 많은 글을 남겼습니다. 잠시 뒤에 다시 시도해 주세요."))
	}

	data := tool.Baro_trunc(baro_community_get(values, "data"), 5000)
	if data == "" {
		return tool.Baro_deny(db, config, tool.Baro_t("본문을 적어 주세요."))
	}

	id := tool.Baro_next_id(db, "baro_workspace")
	note := ""
	link := back

	switch act {
	case "write":
		note = tool.Baro_trunc(baro_community_subject_norm(baro_community_get(values, "subject")), 200)
		if note == "" {
			note = tool.Baro_trunc(tool.Baro_one_line(data), 60)
		}

	case "reply":
		parent := baro_community_get(values, "parent")
		parent_project, parent_title, parent_note := "", "", ""
		if !tool.QueryRow_DB(
			db,
			"select project, title, note from baro_workspace where id = ?",
			[]any{&parent_project, &parent_title, &parent_note}, parent,
		) || parent_project != "usertalk" || parent_title != target {
			return tool.Baro_deny(db, config, tool.Baro_t("이 화면의 글에만 답글을 달 수 있습니다."))
		}

		// 답글에 달린 답글은 원래 글에 붙여 실이 끊기지 않게 한다
		if baro_community_is_reply(parent_note) {
			parent = strings.TrimPrefix(parent_note, "re:")
		}

		note = baro_community_reply_note(parent)

	default:
		return tool.Get_redirect(back)
	}

	tool.Exec_DB(
		db,
		"insert into baro_workspace (id, project, title, data, user, date, status, note) values (?, 'usertalk', ?, ?, ?, ?, 'open', ?)",
		id, target, data, config.IP, tool.Baro_now(), note,
	)

	// 대상에게 알림 (8-2)
	if target != config.IP {
		tool.Baro_notify(
			db, target, "mention",
			config.IP+tool.Baro_t(" 님이 사용자 토론에 글을 남겼습니다"),
			link, tool.Baro_trunc(tool.Baro_one_line(data), 200),
		)
	}

	// 본문 속 @언급 알림
	baro_community_notify_mentions(db, data, config.IP, tool.Baro_t("사용자 토론에서 언급되었습니다"), link)
	tool.Baro_metric_bump(db, "usertalk_post", tool.Baro_today(), 1)

	return tool.Get_redirect(link)
}
