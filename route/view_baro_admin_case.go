package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  16-4 신고 · 사건 큐
//
//  상세 심리 화면은 mod 모듈(/baro/report, /baro/case)에 있다.
//  여기서는 큐 관리(우선순위 · 유형 · 긴급도 · 배정 · 중복 병합 · 기한 · 메모 · 공개 답변)를 맡는다.
//  신고의 내부 상태는 baro_report.note 에 JSON 으로 보관한다. (새 컬럼을 만들지 않기 위해)
// ============================================================

// baro_admin_case_kind_list : 신고 유형
//
//	신고 접수 화면이 baro_report.kind 에 넣는 값과 같은 열쇠말을 쓴다.
//	여기서 유형을 바꿔도 신고 상세 화면에서 같은 이름으로 보여야 하기 때문이다.
func baro_admin_case_kind_list() [][]string {
	return [][]string{
		{"", tool.Baro_t("(고르지 않음)")},
		{"doc_error", tool.Baro_t("문서 오류")},
		{"source", tool.Baro_t("출처 문제")},
		{"copyright", tool.Baro_t("저작권")},
		{"privacy", tool.Baro_t("개인정보")},
		{"defamation", tool.Baro_t("명예훼손 우려")},
		{"spam", tool.Baro_t("광고 · 도배")},
		{"discussion", tool.Baro_t("토론 방해")},
		{"account", tool.Baro_t("계정 악용")},
		{"admin_action", tool.Baro_t("관리자 조치 이의")},
	}
}

// baro_admin_case_kind_name : 신고 유형 이름
func baro_admin_case_kind_name(kind string) string {
	for _, row := range baro_admin_case_kind_list() {
		if row[0] == kind && kind != "" {
			return row[1]
		}
	}

	if kind == "" {
		return tool.Baro_t("미분류")
	}

	// 목록에 없는 값은 지우지 않고 그대로 보여 준다
	return tool.Baro_t("기타 (") + kind + ")"
}

// baro_admin_case_urgency_list : 긴급도
func baro_admin_case_urgency_list() [][]string {
	return [][]string{
		{"normal", tool.Baro_t("일반")},
		{"fast", tool.Baro_t("빠른 처리")},
		{"urgent", tool.Baro_t("긴급 (법적 · 개인정보)")},
	}
}

// baro_admin_case_urgency_badge : 긴급도 배지
func baro_admin_case_urgency_badge(urgency string) string {
	switch urgency {
	case "urgent":
		return tool.Baro_badge(tool.Baro_t("긴급"), "danger")
	case "fast":
		return tool.Baro_badge(tool.Baro_t("빠름"), "warn")
	}

	return tool.Baro_badge(tool.Baro_t("일반"), "muted")
}

// baro_admin_case_note_get : 신고의 내부 상태(JSON) 읽기
func baro_admin_case_note_get(note string) map[string]string {
	out := map[string]string{"assignee": "", "memo": "", "deadline": "", "urgency": "normal"}

	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return out
	}

	if !strings.HasPrefix(trimmed, "{") {
		// 예전 형식(그냥 메모)도 그대로 살린다
		out["memo"] = trimmed

		return out
	}

	for key, value := range tool.Baro_json_dec_str(trimmed) {
		out[key] = value
	}

	if out["urgency"] == "" {
		out["urgency"] = "normal"
	}

	return out
}

// baro_admin_case_note_put : 신고의 내부 상태(JSON) 저장
func baro_admin_case_note_put(db *sql.DB, id string, note map[string]string) {
	tool.Exec_DB(db, "update baro_report set note = ? where id = ?", tool.Baro_json_enc(note), id)
}

// baro_admin_case_report_deadline : 신고 처리 기한 (직접 지정 우선, 없으면 접수일 + 설정값)
func baro_admin_case_report_deadline(db *sql.DB, date string, note map[string]string) string {
	if note["deadline"] != "" {
		return note["deadline"]
	}

	return tool.Baro_time_add(date, tool.Baro_get_int(db, "report_deadline_days", 7), 0, 0)
}

// baro_admin_case_target_link : 신고 대상 링크
func baro_admin_case_target_link(db *sql.DB, config tool.Config, target_kind string, target string) string {
	if target == "" {
		return `<span class="bw_muted">-</span>`
	}

	switch target_kind {
	case "user":
		return tool.Baro_user_link(db, target, config.IP) + ` <a href="/baro/admin/user/view/` + tool.Url_parser(target) + tool.Baro_t(`">(관리)</a>`)
	case "doc", "document":
		return tool.Baro_doc_link(target)
	}

	return baro_admin_esc(target)
}

// baro_admin_case_post : 신고 · 사건 큐 쓰기 동작
func baro_admin_case_post(db *sql.DB, config tool.Config, values url.Values) (string, string) {
	do := baro_admin_get(values, "do")
	id := baro_admin_get(values, "id")

	switch do {
	case "report_assign":
		row := baro_admin_rows(db, 3, "select id, note, target from baro_report where id = ?", id)
		if len(row) == 0 {
			return "", tool.Baro_alert("warn", tool.Baro_t("그 번호의 신고가 없습니다."))
		}

		note := baro_admin_case_note_get(row[0][1])
		assignee := baro_admin_get(values, "assignee")
		note["assignee"] = assignee

		switch urgency := baro_admin_get(values, "urgency"); urgency {
		case "normal", "fast", "urgent":
			note["urgency"] = urgency
		default:
			note["urgency"] = "normal"
		}

		if deadline := baro_admin_get(values, "deadline"); deadline != "" {
			note["deadline"] = deadline + " 23:59:59"
		}

		baro_admin_case_note_put(db, id, note)

		priority := baro_admin_priority_norm(baro_admin_get(values, "priority"))

		// 유형을 고르지 않았으면 원래 값을 지우지 않는다
		kind := baro_admin_get(values, "kind")
		if kind == "" {
			kind = baro_admin_one(db, "select kind from baro_report where id = ?", id)
		}

		status := "pending"
		if assignee == "" {
			// 담당자를 비웠으면 접수 상태를 그대로 둔다
			status = baro_admin_one(db, "select status from baro_report where id = ?", id)
		}

		tool.Exec_DB(db, "update baro_report set priority = ?, kind = ?, status = ? where id = ?", priority, kind, status, id)

		baro_admin_log(db, config, "report_assign", "report:"+id, tool.Baro_t("담당자 배정"), "", assignee)

		if assignee != "" && tool.Baro_is_user(assignee) {
			tool.Baro_notify(db, assignee, "moderation", "신고 #"+id+" 처리를 맡았습니다.", "/baro/admin/case", "기한 "+tool.Baro_date_only(note["deadline"]))
		}

		return tool.Get_redirect("/baro/admin/case?done=assign"), ""

	case "report_memo":
		row := baro_admin_rows(db, 1, "select note from baro_report where id = ?", id)
		if len(row) == 0 {
			return "", tool.Baro_alert("warn", tool.Baro_t("그 번호의 신고가 없습니다."))
		}

		note := baro_admin_case_note_get(row[0][0])
		note["memo"] = baro_admin_get(values, "memo")
		baro_admin_case_note_put(db, id, note)

		tool.Baro_audit(db, config, "report_memo", "report:"+id, "", tool.Baro_trunc(note["memo"], 300), "내부 메모")

		return tool.Get_redirect("/baro/admin/case?done=memo"), ""

	case "report_reply":
		// 공개 답변 : 신고자가 신고 상세 화면에서 보는 답변이다
		row := baro_admin_rows(db, 2, "select note, reporter from baro_report where id = ?", id)
		if len(row) == 0 {
			return "", tool.Baro_alert("warn", tool.Baro_t("그 번호의 신고가 없습니다."))
		}

		note := baro_admin_case_note_get(row[0][0])
		note["reply"] = baro_admin_get(values, "reply")
		baro_admin_case_note_put(db, id, note)

		tool.Baro_public_log(db, "report_reply", config.IP, "report:"+id, "신고 공개 답변", tool.Baro_trunc(note["reply"], 300))
		tool.Baro_audit(db, config, "report_reply", "report:"+id, "", tool.Baro_trunc(note["reply"], 300), "공개 답변")

		if reporter := row[0][1]; reporter != "" && tool.Baro_is_user(reporter) {
			tool.Baro_notify(db, reporter, "moderation", "신고 #"+id+" 에 담당자 답변이 달렸습니다.", "/baro/report/view/"+tool.Url_parser(id), "")
		}

		return tool.Get_redirect("/baro/admin/case?done=reply"), ""

	case "report_status":
		status := baro_admin_get(values, "status")

		switch status {
		case "new", "open", "pending", "closed", "rejected":
		default:
			return "", tool.Baro_alert("warn", tool.Baro_t("상태 값이 올바르지 않습니다."))
		}

		if len(baro_admin_rows(db, 1, "select id from baro_report where id = ?", id)) == 0 {
			return "", tool.Baro_alert("warn", tool.Baro_t("그 번호의 신고가 없습니다."))
		}

		tool.Exec_DB(db, "update baro_report set status = ? where id = ?", status, id)
		baro_admin_log(db, config, "report_status", "report:"+id, baro_admin_get(values, "reason"), "", status)

		return tool.Get_redirect("/baro/admin/case?done=status"), ""

	case "report_merge":
		// 중복 신고 병합 : 대표 신고 하나로 묶는다
		main_id := baro_admin_get(values, "main_id")
		if main_id == "" {
			return "", tool.Baro_alert("warn", tool.Baro_t("대표 신고 번호를 넣어 주세요."))
		}

		if len(baro_admin_rows(db, 1, "select id from baro_report where id = ?", main_id)) == 0 {
			return "", tool.Baro_alert("warn", tool.Baro_t("대표 신고 번호를 찾지 못했습니다."))
		}

		merged := 0
		for _, one := range tool.Baro_split_lines(baro_admin_get(values, "sub_ids")) {
			sub_id := strings.TrimSpace(one)
			if sub_id == "" || sub_id == main_id {
				continue
			}

			row := baro_admin_rows(db, 1, "select note from baro_report where id = ?", sub_id)
			if len(row) == 0 {
				continue
			}

			note := baro_admin_case_note_get(row[0][0])
			note["memo"] = strings.TrimSpace(note["memo"] + tool.Baro_t("\n[중복 병합] 대표 신고 #") + main_id)
			baro_admin_case_note_put(db, sub_id, note)

			tool.Exec_DB(db, "update baro_report set status = 'closed' where id = ?", sub_id)
			merged++
		}

		baro_admin_log(db, config, "report_merge", "report:"+main_id, "중복 신고 병합", "", tool.Baro_itoa(merged)+tool.Baro_t("건"))

		return tool.Get_redirect("/baro/admin/case?done=merge&num=" + tool.Baro_itoa(merged)), ""

	case "case_assign":
		if len(baro_admin_rows(db, 1, "select id from baro_case where id = ?", id)) == 0 {
			return "", tool.Baro_alert("warn", tool.Baro_t("그 번호의 사건이 없습니다."))
		}

		actor := baro_admin_get(values, "actor")
		deadline := baro_admin_get(values, "deadline")
		if deadline != "" {
			deadline += " 23:59:59"
		}

		status := baro_admin_get(values, "status")

		switch status {
		case "open", "pending", "closed", "rejected":
		default:
			// 값이 없거나 모르는 값이면 지금 상태를 그대로 둔다
			status = baro_admin_one(db, "select status from baro_case where id = ?", id)
		}

		tool.Exec_DB(
			db,
			"update baro_case set actor = ?, deadline = ?, priority = ?, status = ? where id = ?",
			actor, deadline, baro_admin_priority_norm(baro_admin_get(values, "priority")), status, id,
		)

		tool.Exec_DB(
			db,
			"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'assign', ?, ?, ?)",
			tool.Baro_next_id(db, "baro_case_action"), id, config.IP, "담당자 "+actor+" / 기한 "+tool.Baro_date_only(deadline), tool.Baro_now(), actor,
		)

		baro_admin_log(db, config, "case_assign", "case:"+id, tool.Baro_t("담당자 배정"), "", actor)

		if actor != "" && tool.Baro_is_user(actor) {
			tool.Baro_notify(db, actor, "moderation", "사건 #"+id+" 담당자로 배정되었습니다.", "/baro/case/view/"+tool.Url_parser(id), "기한 "+tool.Baro_date_only(deadline))
		}

		return tool.Get_redirect("/baro/admin/case?tab=case&done=assign"), ""

	case "case_note":
		note := baro_admin_get(values, "private_note")
		tool.Exec_DB(db, "update baro_case set private_note = ? where id = ?", note, id)
		tool.Baro_audit(db, config, "case_note", "case:"+id, "", tool.Baro_trunc(note, 300), "내부 메모")

		return tool.Get_redirect("/baro/admin/case?tab=case&done=memo"), ""

	case "case_public":
		summary := baro_admin_get(values, "public_summary")
		tool.Exec_DB(db, "update baro_case set public_summary = ? where id = ?", summary, id)
		tool.Baro_public_log(db, "case_public", config.IP, "case:"+id, "공개 답변 게시", tool.Baro_trunc(summary, 300))
		tool.Baro_audit(db, config, "case_public", "case:"+id, "", tool.Baro_trunc(summary, 300), "공개 답변")

		target := baro_admin_one(db, "select target from baro_case where id = ?", id)
		if target != "" && tool.Baro_is_user(target) {
			tool.Baro_notify(db, target, "moderation", "사건 #"+id+" 에 공개 답변이 올라왔습니다.", "/baro/case/view/"+tool.Url_parser(id), "")
		}

		return tool.Get_redirect("/baro/admin/case?tab=case&done=public"), ""

	case "case_close":
		if len(baro_admin_rows(db, 1, "select id from baro_case where id = ?", id)) == 0 {
			return "", tool.Baro_alert("warn", tool.Baro_t("그 번호의 사건이 없습니다."))
		}

		tool.Exec_DB(db, "update baro_case set status = 'closed', close_date = ? where id = ?", tool.Baro_now(), id)
		tool.Exec_DB(
			db,
			"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'close', ?, ?, '')",
			tool.Baro_next_id(db, "baro_case_action"), id, config.IP, baro_admin_get(values, "reason"), tool.Baro_now(),
		)
		baro_admin_log(db, config, "case_close", "case:"+id, baro_admin_get(values, "reason"), "", tool.Baro_t("종결"))

		return tool.Get_redirect("/baro/admin/case?tab=case&done=close"), ""
	}

	return "", tool.Baro_alert("warn", tool.Baro_t("알 수 없는 동작입니다."))
}

// View_baro_admin_case : 16-4 신고 · 사건 큐
func View_baro_admin_case(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if html, stop := baro_admin_gate(db, config, "case"); stop {
		return html
	}

	notice := ""
	if values != nil {
		redirect, message := baro_admin_case_post(db, config, values)
		if redirect != "" {
			return redirect
		}

		notice = message
	}

	tab := baro_admin_get(query, "tab")
	if tab == "" {
		tab = "report"
	}

	body := tool.Baro_hero(
		tool.Baro_t("신고 · 사건 큐"),
		tool.Baro_t("접수된 신고를 분류하고 담당자를 배정합니다. 심리와 조치는 사건 상세 화면에서 합니다."),
		tool.Baro_button("/baro/report", tool.Baro_t("신고 접수 화면"), "")+" "+tool.Baro_button("/baro/case", tool.Baro_t("사건 목록"), ""),
	)
	body += baro_admin_tabs("/baro/admin/case")
	body += tool.Baro_tabs([][]string{
		{"/baro/admin/case?tab=report", tool.Baro_t("신고 큐")},
		{"/baro/admin/case?tab=case", tool.Baro_t("사건 큐")},
		{"/baro/admin/case?tab=dup", "중복 신고 병합"},
	}, "/baro/admin/case?tab="+tab)

	if notice != "" {
		body += notice
	}

	if done := baro_admin_get(query, "done"); done != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("처리했습니다. (")+baro_admin_esc(done)+")")
	}

	switch tab {
	case "case":
		body += baro_admin_case_case_tab(db, config, query)
	case "dup":
		body += baro_admin_case_dup_tab(db, config, query)
	default:
		body += baro_admin_case_report_tab(db, config, query)
	}

	return baro_admin_page(db, config, tool.Baro_t("신고 · 사건 큐"), body)
}

// baro_admin_case_report_tab : 신고 큐
func baro_admin_case_report_tab(db *sql.DB, config tool.Config, query url.Values) string {
	page := baro_admin_page_num(query)
	size := tool.Baro_page_size
	status := baro_admin_get(query, "status")
	if status == "" {
		status = "new"
	}

	kind := baro_admin_get(query, "kind")

	sql_text := "select id, kind, target_kind, target, reporter, detail, status, date, priority, note, case_id from baro_report where status = ?"
	args := []any{status}

	// 신고 접수 화면은 새 신고를 open 으로 넣고, 옛 기록은 new 로 남아 있다. 둘 다 '접수' 로 본다.
	if status == "new" {
		sql_text = "select id, kind, target_kind, target, reporter, detail, status, date, priority, note, case_id from baro_report where status in ('new', 'open')"
		args = []any{}
	}

	if kind != "" {
		sql_text += " and kind = ?"
		args = append(args, kind)
	}

	sql_text += " order by " + baro_admin_priority_order("priority") + ", id + 0 desc limit ?, ?"
	args = append(args, (page-1)*size, size)

	list := baro_admin_rows(db, 11, sql_text, args...)

	rows := [][]string{}
	overdue := 0

	for _, one := range list {
		note := baro_admin_case_note_get(one[9])
		deadline := baro_admin_case_report_deadline(db, one[7], note)

		if tool.Baro_is_past(deadline) && (one[6] == "new" || one[6] == "open" || one[6] == "pending") {
			overdue++
		}

		assignee := tool.Baro_t(`<span class="bw_muted">미배정</span>`)
		if note["assignee"] != "" {
			assignee = tool.Baro_user_link(db, note["assignee"], config.IP)
		}

		case_html := `<span class="bw_muted">-</span>`
		if one[10] != "" {
			case_html = `<a href="/baro/case/view/` + tool.Url_parser(one[10]) + `">#` + baro_admin_esc(one[10]) + `</a>`
		}

		memo := `<span class="bw_muted">-</span>`
		if note["memo"] != "" {
			memo = baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(note["memo"]), 30))
		}

		rows = append(rows, []string{
			`<a href="/baro/report/view/` + tool.Url_parser(one[0]) + `">#` + baro_admin_esc(one[0]) + `</a>`,
			baro_admin_priority_badge(one[8]) + " " + baro_admin_case_urgency_badge(note["urgency"]),
			baro_admin_esc(baro_admin_case_kind_name(one[1])),
			baro_admin_case_target_link(db, config, one[2], one[3]),
			baro_admin_actor(db, config, one[4]),
			tool.Baro_time_cell(one[7]),
			baro_admin_deadline_cell(deadline),
			assignee,
			memo,
			case_html,
			tool.Baro_button("/baro/admin/case?tab=report&status="+tool.Url_parser(status)+"&edit="+tool.Url_parser(one[0]), "배정", "primary"),
		})
	}

	table := tool.Baro_table(
		[]string{"번호", tool.Baro_t("우선순위 · 긴급도"), tool.Baro_t("유형"), tool.Baro_t("대상"), "신고자", "접수", "기한", "담당자", "내부 메모", "사건", ""},
		rows,
		tool.Baro_t("이 조건에 맞는 신고가 없습니다."),
	)

	if len(rows) == 0 {
		table += tool.Baro_empty(tool.Baro_t("처리할 신고가 없습니다. 상태 · 유형 조건을 바꿔 보세요."))
	}

	paging_url := "/baro/admin/case?tab=report&status=" + tool.Url_parser(status)
	if kind != "" {
		paging_url += "&kind=" + tool.Url_parser(kind)
	}
	table += tool.Baro_paging(page, len(rows), size, paging_url+"&page={}")

	body := ""
	if overdue > 0 {
		body += tool.Baro_alert("danger", tool.Baro_t("처리 기한이 지난 신고가 이 쪽에만 <b>")+baro_admin_esc(tool.Baro_itoa(overdue))+tool.Baro_t("</b>건 있습니다."))
	}

	body += tool.Baro_tabs([][]string{
		{"/baro/admin/case?tab=report&status=new", tool.Baro_t("접수 (new · open)")},
		{"/baro/admin/case?tab=report&status=pending", tool.Baro_t("처리 중")},
		{"/baro/admin/case?tab=report&status=closed", tool.Baro_t("종결")},
		{"/baro/admin/case?tab=report&status=rejected", tool.Baro_t("반려")},
	}, "/baro/admin/case?tab=report&status="+status)

	kind_options := [][]string{}
	kind_options = append(kind_options, []string{"", tool.Baro_t("전체 유형")})
	for _, row := range baro_admin_case_kind_list() {
		if row[0] == "" {
			continue
		}

		kind_options = append(kind_options, row)
	}

	filter_form := tool.Baro_hidden("tab", "report")
	filter_form += tool.Baro_hidden("status", status)
	filter_form += tool.Baro_select("kind", tool.Baro_t("유형으로 거르기"), kind_options, kind, "")
	filter_form += tool.Baro_submit(tool.Baro_t("적용"), "")

	body += `<form class="bw_form" method="get" action="/baro/admin/case">` + filter_form + `</form>`
	body += table

	card := tool.Baro_card_ex(tool.Baro_t("신고 큐"), tool.Baro_t("우선순위가 높은 순서로 보여 줍니다."), body, "")

	// 배정 / 메모 / 상태 폼
	edit_id := baro_admin_get(query, "edit")
	if edit_id != "" {
		card += baro_admin_case_report_form(db, config, edit_id)
	} else {
		card += tool.Baro_note(tool.Baro_t("표의 '배정' 단추를 누르면 담당자 · 기한 · 유형 · 내부 메모를 지정하는 칸이 열립니다."))
	}

	return card
}

// baro_admin_case_report_form : 신고 한 건의 배정 · 메모 · 상태 폼
func baro_admin_case_report_form(db *sql.DB, config tool.Config, id string) string {
	row := baro_admin_rows(
		db, 9,
		"select id, kind, target_kind, target, reporter, detail, status, note, priority from baro_report where id = ?",
		id,
	)

	if len(row) == 0 {
		return tool.Baro_alert("warn", tool.Baro_t("그 번호의 신고가 없습니다."))
	}

	one := row[0]
	note := baro_admin_case_note_get(one[7])
	action := "/baro/admin/case"

	info := tool.Baro_kv_grid([][]string{
		{tool.Baro_t("신고 번호"), "#" + baro_admin_esc(one[0])},
		{tool.Baro_t("유형"), baro_admin_esc(baro_admin_case_kind_name(one[1]))},
		{tool.Baro_t("대상"), baro_admin_case_target_link(db, config, one[2], one[3])},
		{"신고자", baro_admin_actor(db, config, one[4])},
		{"상태", baro_admin_status_badge(one[6])},
	})

	info += tool.Baro_section(tool.Baro_t("신고 내용 (내부 열람)"), tool.Baro_code(tool.Baro_trunc(one[5], 800)))
	info += tool.Baro_note(tool.Baro_t("신고 원문은 공개 화면에 그대로 쓰지 않습니다. (0장 원칙 5)"))

	assign_form := tool.Baro_hidden("do", "report_assign")
	assign_form += tool.Baro_hidden("id", id)
	assign_form += tool.Baro_input("assignee", "담당자", note["assignee"], tool.Baro_t("계정 이름"), tool.Baro_t("배정되면 알림이 갑니다."))
	assign_form += tool.Baro_select("priority", tool.Baro_t("우선순위"), baro_admin_priority_list(), baro_admin_priority_norm(one[8]), tool.Baro_t("지금 값이 그대로 골라져 있습니다."))
	assign_form += tool.Baro_select("urgency", tool.Baro_t("긴급도"), baro_admin_case_urgency_list(), note["urgency"], tool.Baro_t("긴급은 법적 · 개인정보 사안에만 씁니다."))
	assign_form += tool.Baro_select("kind", tool.Baro_t("유형"), baro_admin_case_kind_list(), one[1], "")
	assign_form += tool.Baro_input_type("deadline", tool.Baro_t("처리 기한"), tool.Baro_date_only(note["deadline"]), "date", tool.Baro_t("비우면 접수일 기준 기본 기한을 씁니다."))
	assign_form += tool.Baro_submit(tool.Baro_t("배정 저장"), "primary")

	memo_form := tool.Baro_hidden("do", "report_memo")
	memo_form += tool.Baro_hidden("id", id)
	memo_form += tool.Baro_textarea("memo", "내부 메모", note["memo"], 4, tool.Baro_t("판단 근거 · 확인한 사항"), tool.Baro_t("내부 전용입니다. 공개 화면에 나오지 않습니다."))
	memo_form += tool.Baro_submit(tool.Baro_t("메모 저장"), "")

	reply_form := tool.Baro_hidden("do", "report_reply")
	reply_form += tool.Baro_hidden("id", id)
	reply_form += tool.Baro_textarea("reply", "공개 답변", note["reply"], 4, tool.Baro_t("신고자에게 보여 줄 답변"), tool.Baro_t("신고자가 신고 상세 화면에서 봅니다. 개인정보와 신고 원문을 넣지 마세요."))
	reply_form += tool.Baro_submit(tool.Baro_t("공개 답변 저장"), "primary")

	status_options := [][]string{
		{"open", "접수"},
		{"pending", tool.Baro_t("처리 중")},
		{"closed", tool.Baro_t("종결")},
		{"rejected", tool.Baro_t("반려")},
	}

	if one[6] == "new" {
		status_options = append([][]string{{"new", tool.Baro_t("접수 (옛 기록)")}}, status_options...)
	}

	status_form := tool.Baro_hidden("do", "report_status")
	status_form += tool.Baro_hidden("id", id)
	status_form += tool.Baro_select("status", "상태", status_options, one[6], "")
	status_form += tool.Baro_input("reason", "사유", "", tool.Baro_t("상태를 바꾸는 이유"), tool.Baro_t("공개 운영 로그에 남습니다."))
	status_form += tool.Baro_submit(tool.Baro_t("상태 저장"), "")

	body := info
	body += tool.Baro_grid([]string{
		tool.Baro_section(tool.Baro_t("담당자 배정 · 기한"), tool.Baro_form(action, assign_form)),
		tool.Baro_section("내부 메모", tool.Baro_form(action, memo_form)),
		tool.Baro_section("공개 답변", tool.Baro_form(action, reply_form)),
		tool.Baro_section("상태 바꾸기", tool.Baro_form(action, status_form)),
	}, 2)

	return tool.Baro_card_ex(
		"신고 #"+id+" 처리",
		tool.Baro_t("배정 · 기한 · 메모는 이 화면에서 바로 저장됩니다."),
		body,
		tool.Baro_button("/baro/report/view/"+tool.Url_parser(id), "신고 상세 화면", ""),
	)
}

// baro_admin_case_case_tab : 사건 큐
func baro_admin_case_case_tab(db *sql.DB, config tool.Config, query url.Values) string {
	page := baro_admin_page_num(query)
	size := tool.Baro_page_size
	status := baro_admin_get(query, "status")
	if status == "" {
		status = "open"
	}

	list := baro_admin_rows(
		db, 10,
		"select id, number, kind, target_kind, target, actor, status, deadline, priority, public_summary from baro_case where status = ? order by "+baro_admin_priority_order("priority")+", id + 0 desc limit ?, ?",
		status, (page-1)*size, size,
	)

	rows := [][]string{}
	overdue := 0

	for _, one := range list {
		if one[7] != "" && tool.Baro_is_past(one[7]) && one[6] != "closed" {
			overdue++
		}

		number := one[1]
		if number == "" {
			number = one[0]
		}

		public := tool.Baro_badge("없음", "muted")
		if strings.TrimSpace(one[9]) != "" {
			public = tool.Baro_badge(tool.Baro_t("게시됨"), "ok")
		}

		actor := tool.Baro_t(`<span class="bw_muted">미배정</span>`)
		if one[5] != "" {
			actor = baro_admin_actor(db, config, one[5])
		}

		rows = append(rows, []string{
			`<a href="/baro/case/view/` + tool.Url_parser(one[0]) + `">#` + baro_admin_esc(number) + `</a>`,
			baro_admin_priority_badge(one[8]),
			baro_admin_esc(baro_admin_case_kind_name(one[2])),
			baro_admin_case_target_link(db, config, one[3], one[4]),
			actor,
			baro_admin_deadline_cell(one[7]),
			public,
			baro_admin_status_badge(one[6]),
			tool.Baro_button("/baro/admin/case?tab=case&status="+tool.Url_parser(status)+"&edit="+tool.Url_parser(one[0]), "관리", "primary"),
		})
	}

	table := tool.Baro_table(
		[]string{"사건", tool.Baro_t("우선순위"), tool.Baro_t("유형"), tool.Baro_t("대상"), "담당자", "기한", "공개 답변", "상태", ""},
		rows,
		tool.Baro_t("이 상태의 사건이 없습니다."),
	)

	if len(rows) == 0 {
		table += tool.Baro_empty(tool.Baro_t("사건이 없습니다. 신고 큐에서 사건을 열어 보세요."))
	}

	table += tool.Baro_paging(page, len(rows), size, "/baro/admin/case?tab=case&status="+tool.Url_parser(status)+"&page={}")

	body := ""
	if overdue > 0 {
		body += tool.Baro_alert("danger", tool.Baro_t("기한이 지난 사건이 이 쪽에만 <b>")+baro_admin_esc(tool.Baro_itoa(overdue))+tool.Baro_t("</b>건 있습니다."))
	}

	body += tool.Baro_tabs([][]string{
		{"/baro/admin/case?tab=case&status=open", tool.Baro_t("진행")},
		{"/baro/admin/case?tab=case&status=pending", tool.Baro_t("대기")},
		{"/baro/admin/case?tab=case&status=closed", tool.Baro_t("종결")},
	}, "/baro/admin/case?tab=case&status="+status)

	body += table

	card := tool.Baro_card_ex(tool.Baro_t("사건 큐"), tool.Baro_t("배정 · 기한 · 내부 메모 · 공개 답변을 관리합니다."), body, "")

	edit_id := baro_admin_get(query, "edit")
	if edit_id != "" {
		card += baro_admin_case_case_form(db, config, edit_id)
	} else {
		card += tool.Baro_note(tool.Baro_t("표의 '관리' 단추를 누르면 담당자 · 기한 · 메모 · 공개 답변 칸이 열립니다."))
	}

	return card
}

// baro_admin_case_case_form : 사건 한 건의 관리 폼
func baro_admin_case_case_form(db *sql.DB, config tool.Config, id string) string {
	row := baro_admin_rows(
		db, 11,
		"select id, number, kind, target_kind, target, actor, status, deadline, private_note, public_summary, priority from baro_case where id = ?",
		id,
	)

	if len(row) == 0 {
		return tool.Baro_alert("warn", tool.Baro_t("그 번호의 사건이 없습니다."))
	}

	one := row[0]
	action := "/baro/admin/case"

	// 이해충돌 확인 : 담당자가 대상 본인이면 경고
	conflict := ""
	if one[5] != "" && one[5] == one[4] {
		conflict = tool.Baro_alert("danger", tool.Baro_t("담당자와 대상이 같습니다. 이해충돌이므로 다른 담당자를 배정하세요."))
	}
	if one[5] != "" && one[5] == config.IP {
		conflict += tool.Baro_alert("warn", tool.Baro_t("본인이 담당 중인 사건입니다. 이의제기 심사는 다른 담당자가 맡아야 합니다."))
	}

	assign_form := tool.Baro_hidden("do", "case_assign")
	assign_form += tool.Baro_hidden("id", id)
	assign_form += tool.Baro_input("actor", "담당자", one[5], tool.Baro_t("계정 이름"), tool.Baro_t("배정되면 알림이 갑니다."))
	assign_form += tool.Baro_select("priority", tool.Baro_t("우선순위"), baro_admin_priority_list(), baro_admin_priority_norm(one[10]), tool.Baro_t("지금 값이 그대로 골라져 있습니다."))
	assign_form += tool.Baro_select("status", "상태", [][]string{
		{"open", tool.Baro_t("진행")},
		{"pending", tool.Baro_t("대기")},
		{"closed", tool.Baro_t("종결")},
	}, one[6], "")
	assign_form += tool.Baro_input_type("deadline", tool.Baro_t("처리 기한"), tool.Baro_date_only(one[7]), "date", tool.Baro_t("지나면 대시보드에서 빨갛게 표시됩니다."))
	assign_form += tool.Baro_submit(tool.Baro_t("배정 저장"), "primary")

	note_form := tool.Baro_hidden("do", "case_note")
	note_form += tool.Baro_hidden("id", id)
	note_form += tool.Baro_textarea("private_note", "내부 메모", one[8], 5, tool.Baro_t("판단 근거 · 확인 사항"), tool.Baro_t("공개되지 않습니다."))
	note_form += tool.Baro_submit(tool.Baro_t("내부 메모 저장"), "")

	public_form := tool.Baro_hidden("do", "case_public")
	public_form += tool.Baro_hidden("id", id)
	public_form += tool.Baro_textarea("public_summary", "공개 답변", one[9], 5, tool.Baro_t("적용 규정과 조치 요약"), tool.Baro_t("개인정보 · 신고 원문을 넣지 마세요. 공개 로그에 그대로 나갑니다."))
	public_form += tool.Baro_submit("공개 답변 게시", "primary")

	close_form := tool.Baro_hidden("do", "case_close")
	close_form += tool.Baro_hidden("id", id)
	close_form += tool.Baro_input("reason", tool.Baro_t("종결 사유"), "", tool.Baro_t("왜 종결하는지"), "")
	close_form += tool.Baro_submit(tool.Baro_t("사건 종결"), "danger")

	action_rows := [][]string{}
	for _, log := range baro_admin_rows(
		db, 4,
		"select actor, action, detail, date from baro_case_action where case_id = ? order by id + 0 desc limit 20",
		id,
	) {
		action_rows = append(action_rows, []string{
			baro_admin_actor(db, config, log[0]),
			baro_admin_esc(log[1]),
			baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(log[2]), 50)),
			tool.Baro_time_cell(log[3]),
		})
	}

	body := conflict
	body += tool.Baro_grid([]string{
		tool.Baro_section(tool.Baro_t("담당자 · 기한"), tool.Baro_form(action, assign_form)),
		tool.Baro_section("내부 메모", tool.Baro_form(action, note_form)),
		tool.Baro_section("공개 답변", tool.Baro_form(action, public_form)),
	}, 3)

	body += tool.Baro_section("조치 기록", tool.Baro_table([]string{tool.Baro_t("조치자"), "종류", tool.Baro_t("내용"), "시각"}, action_rows, tool.Baro_t("조치 기록이 없습니다.")))
	body += tool.Baro_danger_zone(tool.Baro_t("사건 종결"), tool.Baro_form(action, close_form))

	return tool.Baro_card_ex(
		"사건 #"+id+" 관리",
		tool.Baro_t("자세한 심리와 제재 집행은 사건 상세 화면에서 합니다."),
		body,
		tool.Baro_button("/baro/case/view/"+tool.Url_parser(id), "사건 상세 화면", ""),
	)
}

// baro_admin_case_dup_tab : 중복 신고 병합
func baro_admin_case_dup_tab(db *sql.DB, config tool.Config, query url.Values) string {
	// 같은 대상에 대해 접수 상태인 신고를 모은다
	list := baro_admin_rows(
		db, 4,
		"select target_kind, target, count(*), min(id) from baro_report where status in ('new', 'open') and target != '' group by target_kind, target having count(*) > 1 order by count(*) desc limit 30",
	)

	rows := [][]string{}
	for _, one := range list {
		ids := []string{}
		for _, sub := range baro_admin_rows(
			db, 1,
			"select id from baro_report where status in ('new', 'open') and target_kind = ? and target = ? order by id + 0 asc limit 50",
			one[0], one[1],
		) {
			ids = append(ids, sub[0])
		}

		rows = append(rows, []string{
			baro_admin_case_target_link(db, config, one[0], one[1]),
			baro_admin_esc(one[2]) + tool.Baro_t("건"),
			baro_admin_esc(strings.Join(ids, ", ")),
			tool.Baro_button("/baro/admin/case?tab=dup&main="+tool.Url_parser(one[3])+"&subs="+tool.Url_parser(strings.Join(ids, "\n")), "병합 준비", "primary"),
		})
	}

	body := tool.Baro_note(tool.Baro_t("같은 대상에 대해 접수 상태인 신고가 여러 건이면 중복일 수 있습니다. 대표 신고 하나만 남기고 나머지는 종결로 묶습니다."))
	body += tool.Baro_table([]string{tool.Baro_t("대상"), tool.Baro_t("신고 수"), tool.Baro_t("신고 번호"), ""}, rows, tool.Baro_t("중복으로 보이는 신고가 없습니다."))

	if len(rows) == 0 {
		body += tool.Baro_empty(tool.Baro_t("현재 중복 후보가 없습니다."))
	}

	main_id := baro_admin_get(query, "main")
	subs := baro_admin_get(query, "subs")

	form := tool.Baro_hidden("do", "report_merge")
	form += tool.Baro_input("main_id", tool.Baro_t("대표 신고 번호"), main_id, tool.Baro_t("예: 12"), tool.Baro_t("이 신고만 남기고 나머지를 종결합니다."))
	form += tool.Baro_textarea("sub_ids", tool.Baro_t("묶을 신고 번호"), subs, 5, tool.Baro_t("한 줄에 하나씩"), tool.Baro_t("대표 번호는 넣어도 무시합니다."))
	form += tool.Baro_submit("중복 병합", "danger")

	body += tool.Baro_section(tool.Baro_t("병합 실행"), tool.Baro_form("/baro/admin/case", form))

	return tool.Baro_card_ex("중복 신고 병합", tool.Baro_t("같은 대상에 몰린 신고를 하나로 묶습니다."), body, "")
}
