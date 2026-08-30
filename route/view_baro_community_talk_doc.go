package route

// 바로위키 8-1 문서 토론 보강 · 사용자 언급 처리
//
// 본체 토론(rd / topic 표)을 그대로 쓰고, 바로위키가 필요로 하는
// 해결 표시 · 요약 · 보관 · 리비전 연결만 baro_doc_meta 에 얹는다.

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// baro_community_thread : 문서 토론 스레드 한 줄
type baro_community_thread struct {
	Code    string
	Sub     string
	Date    string
	Stop    string
	Agree   string
	Count   int
	Last    string
	Solved  string
	Summary string
	Archive string
	Rev     string
}

// baro_community_thread_load : 스레드에 얹힌 바로위키 정보를 채운다
func baro_community_thread_load(db *sql.DB, title string, row *baro_community_thread) {
	row.Count = baro_community_thread_count(db, row.Code)
	row.Last = baro_community_thread_last(db, row.Code)
	if row.Last == "" {
		row.Last = row.Date
	}

	row.Solved = tool.Baro_doc_meta_get(db, title, baro_community_meta_name("solved", row.Code))
	row.Summary = tool.Baro_doc_meta_get(db, title, baro_community_meta_name("summary", row.Code))
	row.Archive = tool.Baro_doc_meta_get(db, title, baro_community_meta_name("archive", row.Code))
	row.Rev = tool.Baro_doc_meta_get(db, title, baro_community_meta_name("rev", row.Code))
}

// baro_community_archive_days : 보관으로 내릴 수 있는 최소 경과 일수
const baro_community_archive_days = 30

// baro_community_thread_archivable : 보관해도 되는 토론인지
//
//	8-1 명세대로 "오래되고 해결된" 토론만 보관한다.
//	해결 표시나 종료가 되어 있고, 마지막 활동이 30일 넘게 지난 토론만 대상이다.
func baro_community_thread_archivable(row baro_community_thread) bool {
	if row.Solved == "" && row.Stop != "O" {
		return false
	}

	last := row.Last
	if last == "" {
		last = row.Date
	}

	if last == "" || tool.Baro_parse_time(last).IsZero() {
		return true
	}

	return tool.Baro_days_between(last, tool.Baro_now()) >= baro_community_archive_days
}

// baro_community_thread_state : 스레드 상태 배지
func baro_community_thread_state(row baro_community_thread) string {
	if row.Archive != "" {
		return tool.Baro_badge(tool.Baro_t("보관됨"), "muted")
	}
	if row.Solved != "" {
		return tool.Baro_badge(tool.Baro_t("해결됨"), "ok")
	}
	if row.Stop == "O" {
		return tool.Baro_badge(tool.Baro_t("종료"), "muted")
	}
	if row.Agree == "O" {
		return tool.Baro_badge(tool.Baro_t("합의"), "info")
	}

	return tool.Baro_badge(tool.Baro_t("진행 중"), "warn")
}

// baro_community_solved_text : 해결 표시에 적힌 사람과 시각
func baro_community_solved_text(data string) string {
	if data == "" {
		return ""
	}

	info := tool.Baro_json_dec_str(data)
	if info["by"] == "" && info["date"] == "" {
		return tool.Baro_t("해결 처리됨")
	}

	return tool.Baro_t("해결 처리 : ") + info["by"] + " (" + info["date"] + ")"
}

// baro_community_thread_row : 표 한 줄 만들기
func baro_community_thread_row(db *sql.DB, config tool.Config, title string, row baro_community_thread) []string {
	subject := row.Sub
	if subject == "" {
		subject = tool.Baro_t("제목 없는 주제")
	}

	link := `<a href="/topic/` + tool.Url_parser(title) + `/` + tool.Url_parser(row.Code) + `">` +
		tool.HTML_escape(tool.Baro_trunc(subject, 70)) + `</a>`

	summary := `<span class="bw_muted">-</span>`
	if row.Summary != "" {
		summary = tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Summary), 90))
	}

	rev := `<span class="bw_muted">-</span>`
	if row.Rev != "" {
		rev = `<a href="/w/` + tool.Url_parser(title) + `?rev=` + tool.Url_parser(row.Rev) + `">r` + tool.HTML_escape(row.Rev) + `</a>`
	}

	manage := `<a class="bw_btn" href="/baro/talk/doc/` + tool.Url_parser(title) + `?code=` + tool.Url_parser(row.Code) + tool.Baro_t(`">정리</a>`)
	manage += " " + baro_community_report_link("topic", title+"#"+row.Code, "신고")

	return []string{
		link,
		tool.Baro_itoa(row.Count),
		tool.Baro_time_cell(row.Last),
		baro_community_thread_state(row),
		summary,
		rev,
		manage,
	}
}

// View_baro_community_talk_doc : 8-1 문서 토론 보강
func View_baro_community_talk_doc(config tool.Config, doc_name string, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	title := tool.Namespace_normalize(baro_community_trim_path(doc_name))
	if title == "" {
		return tool.Get_redirect("/baro/talk")
	}

	// 문서를 볼 수 있는 사람만 그 토론을 본다
	if !tool.Check_acl(db, title, "", "render", config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("이 문서의 토론을 볼 권한이 없습니다."))
	}

	// ---------- 쓰기 처리 ----------
	if values != nil {
		return baro_community_talk_doc_post(db, config, title, values)
	}

	page_int := baro_community_page_int(page)
	offset := (page_int - 1) * baro_community_page_size

	open_rows := [][]string{}
	solved_rows := [][]string{}
	archive_rows := [][]string{}
	all := []baro_community_thread{}

	rows := tool.Query_DB(
		db,
		"select code, sub, date, stop, agree from rd where title = ? order by date desc limit ?, ?",
		title, offset, baro_community_page_size,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_community_thread{}
		if err := rows.Scan(&row.Code, &row.Sub, &row.Date, &row.Stop, &row.Agree); err != nil {
			continue
		}

		baro_community_thread_load(db, title, &row)
		all = append(all, row)
	}

	for _, row := range all {
		cells := baro_community_thread_row(db, config, title, row)
		switch {
		case row.Archive != "":
			archive_rows = append(archive_rows, cells)
		case row.Solved != "" || row.Stop == "O":
			solved_rows = append(solved_rows, cells)
		default:
			open_rows = append(open_rows, cells)
		}
	}

	head := [][]string{}
	head = append(head, []string{"문서", tool.Baro_doc_link(title)})
	head = append(head, []string{tool.Baro_t("문서 상태"), tool.Baro_status_badge(tool.Baro_doc_status_get(db, title))})
	head = append(head, []string{tool.Baro_t("현재 판"), tool.HTML_escape("r" + tool.Get_document_revision(db, title))})
	head = append(head, []string{"분야", tool.HTML_escape(tool.Baro_field_name(db, tool.Baro_doc_field(db, title)))})

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/talk", tool.Baro_t("토론")},
		{"", tool.Namespace_display(title)},
	})

	body += tool.Baro_hero(
		tool.Namespace_display(title)+tool.Baro_t(" 토론"),
		tool.Baro_t("본체 토론을 바로위키 형식으로 정리해서 봅니다."),
		tool.Baro_button("/topic/"+tool.Url_parser(title), "본체 토론 화면", "primary")+" "+
			tool.Baro_button("/w/"+tool.Url_parser(title), "문서 보기", "")+" "+
			baro_community_watch_hint(title),
	)
	body += tool.Baro_kv_grid(head)

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("진행 중"), tool.Baro_itoa(len(open_rows)), tool.Baro_t("아직 결론이 없는 주제")},
		{tool.Baro_t("해결됨"), tool.Baro_itoa(len(solved_rows)), tool.Baro_t("결론이 난 주제")},
		{tool.Baro_t("보관됨"), tool.Baro_itoa(len(archive_rows)), tool.Baro_t("오래되어 내려둔 주제")},
	})

	headers := []string{tool.Baro_t("주제"), tool.Baro_t("글 수"), tool.Baro_t("마지막 활동"), "상태", tool.Baro_t("요약"), tool.Baro_t("관련 판"), "관리"}

	body += tool.Baro_card(
		tool.Baro_t("진행 중인 토론"),
		tool.Baro_table(headers, open_rows, tool.Baro_t("진행 중인 토론이 없습니다.")),
	)

	if len(solved_rows) > 0 {
		inner := tool.Baro_table(headers, solved_rows, "")
		body += tool.Baro_card(
			tool.Baro_t("해결된 토론"),
			tool.Baro_t(`<details><summary>해결된 토론 `)+tool.Baro_itoa(len(solved_rows))+tool.Baro_t(`건 펼치기</summary>`)+inner+`</details>`,
		)
	}

	if len(archive_rows) > 0 {
		inner := tool.Baro_table(headers, archive_rows, "")
		body += tool.Baro_card(
			tool.Baro_t("보관된 토론"),
			tool.Baro_t(`<details><summary>보관된 토론 `)+tool.Baro_itoa(len(archive_rows))+tool.Baro_t(`건 펼치기</summary>`)+inner+`</details>`,
		)
	}

	if len(all) == 0 {
		body += tool.Baro_empty(tool.Baro_t("이 문서에는 아직 토론이 없습니다. 본체 토론 화면에서 새 주제를 열 수 있습니다."))
	}

	body += tool.Baro_paging(page_int, len(all), baro_community_page_size, "/baro/talk/doc/"+tool.Url_parser(title)+"?page={}")

	// ---------- 스레드 정리 상자 ----------
	code := baro_community_get(query, "code")
	if code != "" {
		body += baro_community_talk_doc_edit(db, config, title, code)
	}

	body += tool.Baro_note(tool.Baro_t("결론이 난 토론은 요약을 남기고 해결 표시를 해 주세요. 오래된 해결 토론은 보관으로 내립니다."))

	return tool.Baro_page(db, config, tool.Namespace_display(title)+tool.Baro_t(" 토론"), body, baro_community_menu())
}

// baro_community_talk_doc_edit : 스레드 하나를 정리하는 상자
func baro_community_talk_doc_edit(db *sql.DB, config tool.Config, title string, code string) string {
	sub, date, stop := "", "", ""
	found := tool.QueryRow_DB(
		db,
		"select sub, date, stop from rd where title = ? and code = ?",
		[]any{&sub, &date, &stop}, title, code,
	)

	if !found {
		return tool.Baro_alert("danger", tool.Baro_t("그런 토론 주제가 없습니다."))
	}

	row := baro_community_thread{Code: code, Sub: sub, Date: date, Stop: stop}
	baro_community_thread_load(db, title, &row)

	if !tool.Baro_is_user(config.IP) {
		return tool.Baro_card(
			tool.Baro_t("토론 정리 : ")+tool.Baro_trunc(sub, 40),
			tool.Baro_alert("info", tool.Baro_t("토론 정리(해결 표시 · 요약 · 보관)는 로그인한 사용자만 할 수 있습니다.")),
		)
	}

	action := "/baro/talk/doc/" + tool.Url_parser(title)

	form := tool.Baro_hidden("code", code)
	form += tool.Baro_textarea("summary", tool.Baro_t("토론 요약"), row.Summary, 5, tool.Baro_t("무엇이 쟁점이었고 어떻게 결론이 났는지 적습니다."), tool.Baro_t("요약은 이 화면과 목록에 함께 보입니다."))
	form += tool.Baro_input("rev", tool.Baro_t("관련 문서 판"), row.Rev, tool.Baro_t("예: 12"), tool.Baro_t("이 토론이 어떤 판에 대한 것인지 적습니다. 숫자만 넣습니다."))
	form += tool.Baro_hidden("act", "save")
	form += tool.Baro_submit(tool.Baro_t("요약·판 저장"), "primary")

	body := tool.Baro_form(action, form)

	// 해결 표시 / 해제
	solve_form := tool.Baro_hidden("code", code)
	if row.Solved == "" {
		solve_form += tool.Baro_input("reason", tool.Baro_t("해결 사유"), "", tool.Baro_t("무엇으로 정리되었는지"), tool.Baro_t("공개 로그에 남습니다."))
		solve_form += tool.Baro_hidden("act", "solve")
		solve_form += tool.Baro_submit(tool.Baro_t("해결로 표시"), "primary")
	} else {
		solve_form += tool.Baro_hidden("act", "unsolve")
		solve_form += tool.Baro_submit(tool.Baro_t("해결 표시 되돌리기"), "")
	}

	body += tool.Baro_section(tool.Baro_t("해결 여부"), tool.Baro_form(action, solve_form)+
		func() string {
			if text := baro_community_solved_text(row.Solved); text != "" {
				return tool.Baro_note(text)
			}

			return ""
		}())

	// 보관 / 보관 해제 (검토 권한)
	if tool.Baro_can(db, config.IP, "review") {
		archive_form := tool.Baro_hidden("code", code)
		archive_body := tool.Baro_note(tool.Baro_t("오래되고 해결된 토론만 보관합니다. 보관해도 내용은 그대로 남습니다."))

		switch {
		case row.Archive != "":
			archive_form += tool.Baro_hidden("act", "unarchive")
			archive_form += tool.Baro_submit(tool.Baro_t("보관 되돌리기"), "")
			archive_body += tool.Baro_form(action, archive_form)

		case !baro_community_thread_archivable(row):
			archive_body += tool.Baro_alert(
				"info",
				tool.Baro_t("아직 보관할 수 없는 토론입니다. 먼저 해결로 표시하거나 본체 토론에서 종료한 뒤, ")+
					tool.HTML_escape(tool.Baro_itoa(baro_community_archive_days))+tool.Baro_t("일 넘게 활동이 없을 때 보관합니다."),
			)

		default:
			archive_form += tool.Baro_hidden("act", "archive")
			archive_form += tool.Baro_submit(tool.Baro_t("보관으로 내리기"), "")
			archive_body += tool.Baro_form(action, archive_form)
		}

		body += tool.Baro_section(tool.Baro_t("보관"), archive_body)
	} else {
		body += tool.Baro_section(tool.Baro_t("보관"), tool.Baro_note(tool.Baro_t("보관은 문서 검토 권한이 있는 사용자만 할 수 있습니다.")))
	}

	// 인용
	quote := "> [" + tool.Namespace_display(title) + tool.Baro_t(" 토론 #") + code + "] " + tool.Baro_one_line(sub)
	body += tool.Baro_section(tool.Baro_t("인용하기"), tool.Baro_code(quote)+tool.Baro_note(tool.Baro_t("다른 토론에 붙여 넣어 이 주제를 인용할 수 있습니다. @이름 을 쓰면 그 사용자에게 알림이 갑니다.")))

	body += tool.Baro_section("신고", baro_community_report_link("topic", title+"#"+code, tool.Baro_t("이 토론 신고하기")))

	return tool.Baro_card_ex(tool.Baro_t("토론 정리"), tool.Baro_trunc(sub, 60), body, "")
}

// baro_community_talk_doc_post : 문서 토론 정리 처리
func baro_community_talk_doc_post(db *sql.DB, config tool.Config, title string, values url.Values) string {
	back := "/baro/talk/doc/" + tool.Url_parser(title)

	if wall, need := tool.Baro_login_wall(db, config, back); need {
		return wall
	}

	if block := baro_community_write_block(db); block != "" {
		return tool.Baro_deny(db, config, block)
	}

	code := baro_community_get(values, "code")
	if code == "" {
		return tool.Get_redirect(back)
	}

	exist := ""
	if !tool.QueryRow_DB(db, "select code from rd where title = ? and code = ?", []any{&exist}, title, code) {
		return tool.Baro_deny(db, config, tool.Baro_t("그런 토론 주제가 없습니다."))
	}

	if !tool.Baro_rate_ok(db, "community_talk_doc", config.IP, 30, 300) {
		return tool.Baro_deny(db, config, tool.Baro_t("짧은 시간에 너무 많이 처리했습니다. 잠시 뒤에 다시 시도해 주세요."))
	}

	act := baro_community_get(values, "act")
	link := back + "?code=" + tool.Url_parser(code)

	switch act {
	case "save":
		summary := tool.Baro_trunc(baro_community_get(values, "summary"), 2000)
		rev := baro_community_get(values, "rev")
		if rev != "" {
			rev = tool.Baro_itoa(tool.Str_to_int(rev))
		}

		tool.Baro_doc_meta_set(db, title, baro_community_meta_name("summary", code), summary)
		tool.Baro_doc_meta_set(db, title, baro_community_meta_name("rev", code), rev)
		tool.Baro_audit(db, config, "talk_summary", title+"#"+code, "", tool.Baro_trunc(summary, 200), "토론 요약 저장")

		// 요약 안에 @이름 이 있으면 알림을 보낸다
		baro_community_notify_mentions(db, summary, config.IP, "["+tool.Namespace_display(title)+tool.Baro_t("] 토론 요약에서 언급되었습니다"), link)

	case "solve":
		reason := tool.Baro_trunc(baro_community_get(values, "reason"), 300)
		data := tool.Baro_json_enc(map[string]string{
			"by":     config.IP,
			"date":   tool.Baro_now(),
			"reason": reason,
		})

		tool.Baro_doc_meta_set(db, title, baro_community_meta_name("solved", code), data)
		tool.Baro_public_log(db, "talk_solved", config.IP, title+"#"+code, reason, "토론 해결 표시")
		tool.Baro_audit(db, config, "talk_solved", title+"#"+code, "", data, reason)

		// 이 문서를 주시하는 사람에게 알린다 (즉시 알림으로 해 둔 사람만)
		baro_community_notify_watchers(
			db, "document", title, config.IP,
			"["+tool.Namespace_display(title)+tool.Baro_t("] 토론이 해결로 정리되었습니다"), link,
		)

	case "unsolve":
		tool.Baro_doc_meta_set(db, title, baro_community_meta_name("solved", code), "")
		tool.Baro_public_log(db, "talk_unsolved", config.IP, title+"#"+code, "", "토론 해결 표시 해제")

	case "archive":
		if !tool.Baro_can(db, config.IP, "review") {
			return tool.Baro_deny(db, config, tool.Baro_t("토론 보관은 문서 검토 권한이 있어야 합니다."))
		}

		check := baro_community_thread{Code: code}
		tool.QueryRow_DB(
			db,
			"select sub, date, stop, agree from rd where title = ? and code = ?",
			[]any{&check.Sub, &check.Date, &check.Stop, &check.Agree}, title, code,
		)
		baro_community_thread_load(db, title, &check)

		if !baro_community_thread_archivable(check) {
			return tool.Baro_deny(
				db, config,
				tool.Baro_t("오래되고 해결된 토론만 보관할 수 있습니다. 먼저 해결로 표시하거나 토론을 종료해 주세요."),
			)
		}

		tool.Baro_doc_meta_set(db, title, baro_community_meta_name("archive", code), tool.Baro_now())
		tool.Baro_public_log(db, "talk_archive", config.IP, title+"#"+code, "", "토론 보관")

	case "unarchive":
		if !tool.Baro_can(db, config.IP, "review") {
			return tool.Baro_deny(db, config, tool.Baro_t("토론 보관 해제는 문서 검토 권한이 있어야 합니다."))
		}

		tool.Baro_doc_meta_set(db, title, baro_community_meta_name("archive", code), "")
		tool.Baro_public_log(db, "talk_unarchive", config.IP, title+"#"+code, "", "토론 보관 해제")

	default:
		return tool.Get_redirect(back)
	}

	return tool.Get_redirect(link)
}

// ============================================================
//  사용자 언급 처리 도구 (8-1)
// ============================================================

// baro_community_mention_mark : 어디까지 훑었는지 적어 두는 이름표
const baro_community_mention_mark = "community_talk_mention_date"

// baro_community_mention_scan : 아직 처리하지 않은 토론 글에서 언급을 모은다
//
//	send 가 참이면 실제로 알림을 보내고 표시를 옮긴다.
func baro_community_mention_scan(db *sql.DB, config tool.Config, send bool) ([][]string, int, string) {
	from := tool.Baro_get_def(db, baro_community_mention_mark, "")
	if from == "" {
		// 처음 실행이면 최근 7일치만 본다
		from = tool.Baro_time_add(tool.Baro_now(), -7, 0, 0)
	}

	found := [][]string{}
	last := from
	sent := 0

	rows := tool.Query_DB(
		db,
		"select code, id, data, date, ip from topic where date > ? order by date asc limit 300",
		from,
	)
	defer rows.Close()

	type mention_item struct {
		Code, Id, Data, Date, Ip string
		Names                    []string
	}

	items := []mention_item{}
	for rows.Next() {
		item := mention_item{}
		if err := rows.Scan(&item.Code, &item.Id, &item.Data, &item.Date, &item.Ip); err != nil {
			continue
		}

		if item.Date > last {
			last = item.Date
		}

		item.Names = baro_community_mention_list(item.Data)
		if len(item.Names) == 0 {
			continue
		}

		items = append(items, item)
	}

	for _, item := range items {
		doc_title := ""
		tool.QueryRow_DB(db, "select title from rd where code = ?", []any{&doc_title}, item.Code)

		targets := []string{}
		for _, name := range item.Names {
			if name == item.Ip || !baro_community_user_exists(db, name) {
				continue
			}

			targets = append(targets, name)
		}

		if len(targets) == 0 {
			continue
		}

		link := "/topic/" + tool.Url_parser(doc_title) + "/" + tool.Url_parser(item.Code)
		if send {
			for _, name := range targets {
				tool.Baro_notify(
					db, name, "mention",
					"["+tool.Namespace_display(doc_title)+tool.Baro_t("] 토론에서 언급되었습니다"),
					link, tool.Baro_trunc(tool.Baro_one_line(item.Data), 200),
				)
				sent++
			}
		}

		found = append(found, []string{
			tool.Baro_doc_link(doc_title),
			`<a href="` + tool.HTML_escape(link) + `">#` + tool.HTML_escape(item.Code) + `-` + tool.HTML_escape(item.Id) + `</a>`,
			tool.HTML_escape(strings.Join(targets, ", ")),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(item.Data), 80)),
			tool.Baro_time_cell(item.Date),
		})
	}

	if send {
		tool.Baro_put(db, baro_community_mention_mark, last)
		tool.Baro_audit(db, config, "talk_mention_send", "topic", from, last, "언급 알림 발송 "+tool.Baro_itoa(sent)+"건")
	}

	return found, sent, last
}

// View_baro_community_talk_mention : 8-1 사용자 언급 처리 화면
func View_baro_community_talk_mention(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/talk/mention"); need {
		return wall
	}

	// ---------- 처리 ----------
	if values != nil {
		if !tool.Baro_can(db, config.IP, "patrol") {
			return tool.Baro_deny(db, config, tool.Baro_t("언급 알림 발송은 순찰·검토 권한이 있어야 합니다."))
		}

		if block := baro_community_write_block(db); block != "" {
			return tool.Baro_deny(db, config, block)
		}

		if !tool.Baro_rate_ok(db, "community_mention_send", config.IP, 5, 600) {
			return tool.Baro_deny(db, config, tool.Baro_t("언급 발송은 10분에 5번까지만 실행할 수 있습니다."))
		}

		act := baro_community_get(values, "act")
		if act == "reset" {
			tool.Baro_put(db, baro_community_mention_mark, tool.Baro_time_add(tool.Baro_now(), -7, 0, 0))

			return tool.Get_redirect("/baro/talk/mention")
		}

		_, sent, _ := baro_community_mention_scan(db, config, true)
		tool.Baro_metric_bump(db, "talk_mention_sent", tool.Baro_today(), sent)

		return tool.Get_redirect("/baro/talk/mention?sent=" + tool.Baro_itoa(sent))
	}

	// ---------- 보여주기 ----------
	list, _, last := baro_community_mention_scan(db, config, false)
	mark := tool.Baro_get_def(db, baro_community_mention_mark, "(아직 없음)")

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/talk", tool.Baro_t("토론")},
		{"", tool.Baro_t("언급 처리")},
	})
	body += tool.Baro_hero(
		tool.Baro_t("사용자 언급 처리"),
		tool.Baro_t("토론 본문에 적힌 @사용자이름 을 훑어 아직 알림이 가지 않은 언급을 찾아냅니다."),
		"",
	)
	body += baro_community_talk_tabs("/baro/talk/mention")

	if sent := baro_community_get(query, "sent"); sent != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("언급 알림 ")+tool.Baro_itoa(tool.Str_to_int(sent))+tool.Baro_t("건을 보냈습니다."))
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("처리 대기"), tool.Baro_itoa(len(list)), tool.Baro_t("알림이 가지 않은 언급")},
		{tool.Baro_t("마지막 처리 지점"), tool.Baro_trunc(mark, 19), tool.Baro_t("이 시각 이후 글을 훑습니다")},
		{tool.Baro_t("오늘 보낸 수"), tool.Baro_itoa(tool.Baro_metric_get(db, "talk_mention_sent", tool.Baro_today())), tool.Baro_t("오늘 발송한 언급 알림")},
	})

	body += tool.Baro_card(
		tool.Baro_t("처리 대기 중인 언급"),
		tool.Baro_table(
			[]string{"문서", tool.Baro_t("글"), tool.Baro_t("언급 대상"), tool.Baro_t("본문"), "시각"},
			list,
			tool.Baro_t("처리할 언급이 없습니다."),
		),
	)

	if tool.Baro_can(db, config.IP, "patrol") {
		form := tool.Baro_hidden("act", "send")
		form += tool.Baro_note(tool.Baro_t("아래 단추를 누르면 위 목록의 대상에게 알림을 보내고, 처리 지점을 ") + tool.Baro_trunc(last, 19) + tool.Baro_t(" 로 옮깁니다."))
		form += tool.Baro_submit(tool.Baro_t("언급 알림 보내기"), "primary")

		reset := tool.Baro_hidden("act", "reset")
		reset += tool.Baro_submit(tool.Baro_t("처리 지점을 7일 전으로 되돌리기"), "")

		body += tool.Baro_card(tool.Baro_t("알림 보내기"), tool.Baro_form("/baro/talk/mention", form))
		body += tool.Baro_danger_zone(tool.Baro_t("처리 지점 되돌리기"), tool.Baro_note(tool.Baro_t("되돌리면 이미 보낸 언급이 다시 발송될 수 있습니다."))+tool.Baro_form("/baro/talk/mention", reset))
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("발송은 순찰·검토 권한이 있는 사용자만 할 수 있습니다. 목록은 누구나 볼 수 있습니다."))
	}

	return tool.Baro_page(db, config, tool.Baro_t("사용자 언급 처리"), body, baro_community_menu())
}
