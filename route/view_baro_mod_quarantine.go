package route

// ============================================================
//  10-4 문서 보존 원칙 (/baro/quarantine)
//
//  계정이 제재되더라도 정상 기여는 유지된다. 문제가 있는 리비전만 검토한다.
//  삭제의 기본 경로는 "임시 격리" 다. 격리할 때 원본 내용을 통째로 보존한다.
//  완전 삭제는 별도 확인 화면에서 사유를 다시 받고, 왜 복구가 아니라 삭제인지 적게 한다.
// ============================================================

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ------------------------------------------------------------
//  격리 등록 (다른 화면에서도 쓴다)
// ------------------------------------------------------------

// baro_mod_quarantine_add : 문서를 임시 격리하고 원본을 보존한다
//
//	hide 가 true 면 본문을 안내문으로 갈음하고 원본은 backup 에 남긴다.
//	어떤 경우에도 원본 내용은 지워지지 않는다.
func baro_mod_quarantine_add(db *sql.DB, config tool.Config, title string, rev string, kind string, reason string, case_id string, hide bool) string {
	data, exists := baro_mod_doc_data(db, title)
	if rev == "" {
		rev = tool.Get_document_revision(db, title)
	}

	id := tool.Baro_next_id(db, "baro_quarantine")
	now := tool.Baro_now()

	tool.Exec_DB(
		db,
		"insert into baro_quarantine (id, title, rev, kind, reason, actor, date, status, backup, release_date, case_id) values (?, ?, ?, ?, ?, ?, ?, 'open', ?, '', ?)",
		id, title, rev, kind, reason, config.IP, now, data, case_id,
	)

	// 문서 상태도 격리로 바꾼다
	tool.Baro_doc_status_set(db, title, "quarantined", reason, config.IP)

	if hide && exists {
		baro_mod_doc_write(db, config, title, baro_mod_quarantine_notice(kind, reason), tool.Baro_t("임시 격리 (격리번호 ")+id+")")
	}

	if case_id != "" {
		tool.Exec_DB(
			db,
			"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'quarantine', ?, ?, ?)",
			tool.Baro_next_id(db, "baro_case_action"), case_id, config.IP,
			baro_mod_quarantine_kind_name(kind)+" 사유로 문서를 임시 격리했습니다. (격리번호 "+id+")\n사유: "+reason,
			now, title,
		)
	}

	tool.Baro_public_log(db, "quarantine", config.IP, title, tool.Baro_trunc(tool.Baro_one_line(reason), 200), baro_mod_quarantine_kind_name(kind))
	tool.Baro_audit(db, config, "quarantine", title, tool.Baro_trunc(data, 500), baro_mod_quarantine_kind_name(kind), reason)
	tool.Baro_metric_bump(db, "quarantine", tool.Baro_today(), 1)

	// 마지막 기여자에게 알린다 (기여가 지워진 것이 아니라는 사실을 알려야 한다)
	if editor := baro_mod_doc_last_editor(db, title); editor != "" {
		baro_mod_notify_user(db, editor, "case",
			tool.Baro_t("문서 '")+tool.Namespace_display(title)+tool.Baro_t("' 이(가) 임시 격리되었습니다. 원본은 보존되어 있습니다."),
			"/baro/quarantine/view/"+tool.Url_parser(id), reason)
	}

	return id
}

// ------------------------------------------------------------
//  격리 목록
// ------------------------------------------------------------

// View_baro_mod_quarantine_list : 격리 목록 (/baro/quarantine)
func View_baro_mod_quarantine_list(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	page := baro_mod_page_num(arg)
	size := tool.Baro_page_size

	status := baro_mod_or(baro_mod_val(query, "status"), "open")
	kind := baro_mod_val(query, "kind")

	can_handle := tool.Baro_can(db, config.IP, "quarantine")

	where := " where 1 = 1"
	params := []any{}

	if status != "all" {
		where += " and status = ?"
		params = append(params, status)
	}

	if kind != "" {
		where += " and kind = ?"
		params = append(params, kind)
	}

	params = append(params, (page-1)*size, size)

	rows := tool.Query_DB(
		db,
		"select id, title, rev, kind, reason, actor, date, status, release_date, case_id from baro_quarantine"+where+" order by id + 0 desc limit ?, ?",
		params...,
	)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0

	for rows.Next() {
		id, title, rev, q_kind, reason, actor, date, q_status, release_date, case_id := "", "", "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &title, &rev, &q_kind, &reason, &actor, &date, &q_status, &release_date, &case_id); err != nil {
			continue
		}

		count++

		actor_html := baro_mod_esc(tool.Baro_mask_ip(actor))
		if can_handle {
			actor_html = tool.Baro_user_link(db, actor, config.IP)
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/quarantine/view/` + tool.Url_parser(id) + `">` + baro_mod_esc(id) + `</a>`,
			tool.Baro_doc_link(title),
			baro_mod_quarantine_edit_badge(db, config, title),
			baro_mod_muted(rev),
			baro_mod_esc(baro_mod_quarantine_kind_name(q_kind)),
			baro_mod_esc(tool.Baro_trunc(tool.Baro_one_line(reason), 120)),
			baro_mod_status_badge(q_status),
			baro_mod_case_link(db, case_id),
			actor_html,
			tool.Baro_time_cell(baro_mod_or(release_date, date)),
		})
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("문서 격리")}})

	body += tool.Baro_hero(
		tool.Baro_t("문서 격리"),
		tool.Baro_t("문제가 제기된 문서는 지우지 않고 임시로 격리합니다. 원본 내용은 그대로 보존되며, 검토가 끝나면 복구합니다."),
		"",
	)

	open_count, released_count, deleted_count := 0, 0, 0
	tool.QueryRow_DB(db, "select count(*) from baro_quarantine where status = 'open'", []any{&open_count})
	tool.QueryRow_DB(db, "select count(*) from baro_quarantine where status = 'released'", []any{&released_count})
	tool.QueryRow_DB(db, "select count(*) from baro_quarantine where status = 'deleted'", []any{&deleted_count})

	body += tool.Baro_card(tool.Baro_t("격리 현황"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("격리 중"), tool.Baro_comma(open_count), tool.Baro_t("검토를 기다리고 있습니다")},
		{tool.Baro_t("복구됨"), tool.Baro_comma(released_count), tool.Baro_t("문제가 없어 되돌렸습니다")},
		{tool.Baro_t("삭제됨"), tool.Baro_comma(deleted_count), tool.Baro_t("복구할 수 없다고 판단했습니다")},
	}))

	filter_form := `<form class="bw_form" method="get" action="/baro/quarantine">`
	filter_form += tool.Baro_select("status", "상태", [][]string{
		{"open", tool.Baro_t("격리 중")},
		{"released", tool.Baro_t("복구됨")},
		{"deleted", tool.Baro_t("삭제됨")},
		{"all", tool.Baro_t("전체")},
	}, status, "")
	filter_form += tool.Baro_select("kind", "종류", append([][]string{{"", tool.Baro_t("전체")}}, baro_mod_quarantine_kind_list()...), kind, "")
	filter_form += tool.Baro_submit(tool.Baro_t("적용"), "")
	filter_form += `</form>`

	body += tool.Baro_card("걸러 보기", filter_form)

	list_body := baro_mod_quarantine_edit_notice(can_handle)
	list_body += tool.Baro_table(
		[]string{"번호", "문서", "편집", "판", "종류", "사유", "상태", "사건", tool.Baro_t("집행자"), tool.Baro_t("최근")},
		table_rows,
		"",
	)
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("조건에 맞는 격리 기록이 없습니다."))
	}

	list_body += tool.Baro_paging(page, count, size,
		"/baro/quarantine/list/{}?status="+tool.Url_parser(status)+"&kind="+tool.Url_parser(kind))

	body += tool.Baro_card(tool.Baro_t("격리 목록"), list_body)

	if can_handle {
		body += tool.Baro_card(tool.Baro_t("담당자 기능"),
			tool.Baro_button("/baro/quarantine/new", "문서 임시 격리", "primary")+" "+
				tool.Baro_button("/baro/quarantine/user", tool.Baro_t("사용자별 문서 다루기"), "")+" "+
				tool.Baro_button("/baro/mass/new", tool.Baro_t("대량 작업 만들기"), ""),
		)
	}

	body += tool.Baro_card(tool.Baro_t("격리 규정"),
		tool.Baro_list([]string{
			tool.Baro_t("격리는 삭제가 아닙니다. 원본 내용은 <b>baro_quarantine</b> 에 통째로 보존됩니다."),
			tool.Baro_t("저작권 침해와 개인정보 노출은 긴급 비공개 대상이며, 즉시 격리한 뒤 검토합니다."),
			tool.Baro_t("검토 결과는 복구 · 수정 후 복구 · 완전 삭제 중 하나입니다. 완전 삭제는 별도 확인 화면을 거칩니다."),
			tool.Baro_t("계정이 제재되었다는 이유만으로 그 사람의 문서를 격리하지 않습니다. 문서 문제는 따로 판단합니다."),
		}),
	)

	return baro_mod_shell(db, config, tool.Baro_t("문서 격리"), "/baro/quarantine", body)
}

// ------------------------------------------------------------
//  격리 등록 화면
// ------------------------------------------------------------

// View_baro_mod_quarantine_new : 문서 임시 격리 (/baro/quarantine/new)
func View_baro_mod_quarantine_new(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = arg

	if !tool.Baro_can(db, config.IP, "quarantine") {
		return tool.Baro_deny(db, config, tool.Baro_t("문서 격리는 순찰자 · 제재 담당 · 개인정보 담당 역할이 있어야 합니다."))
	}

	if page, stop := baro_mod_read_only_block(db, config, "/baro/quarantine"); stop {
		return page
	}

	if values != nil {
		return baro_mod_quarantine_new_post(db, config, values)
	}

	title := baro_mod_val(query, "title")
	kind := baro_mod_or(baro_mod_val(query, "kind"), "normal")
	case_key := baro_mod_val(query, "case")
	report_id := baro_mod_val(query, "report")

	case_row, has_case := baro_mod_case_get(db, case_key)
	if has_case {
		case_key = case_row.ID
	}

	body := tool.Baro_crumb([][]string{{"/baro/quarantine", tool.Baro_t("문서 격리")}, {"", tool.Baro_t("임시 격리")}})

	body += tool.Baro_alert("info",
		tool.Baro_t("격리하면 원본 내용이 통째로 보존됩니다. 언제든 복구할 수 있습니다. ")+
			tool.Baro_t("이 화면은 문서를 지우지 않습니다. (고정 원칙 2)"))

	if report, ok := baro_mod_report_get(db, report_id); ok {
		body += tool.Baro_card(tool.Baro_t("연결된 신고"), tool.Baro_kv_grid([][]string{
			{"접수번호", `<a href="/baro/report/view/` + tool.Url_parser(report.ID) + `">` + baro_mod_esc(report.ID) + `</a>`},
			{"종류", baro_mod_esc(baro_mod_report_kind_name(report.Kind))},
		}))
	}

	// 현재 문서 상태 미리 보기
	if title != "" {
		data, exists := baro_mod_doc_data(db, title)
		status_html := tool.Baro_badge(tool.Baro_t("문서 없음"), "danger")
		length_html := "-"
		if exists {
			status_html = tool.Baro_badge(tool.Baro_t("존재함"), "ok")
			length_html = tool.Baro_comma(tool.Get_len(data)) + tool.Baro_t("자")
		}

		doc_status := tool.Baro_doc_status_get(db, title)
		already := ""
		if tool.Baro_doc_quarantined(db, title) {
			already = tool.Baro_badge(tool.Baro_t("이미 격리 중"), "warn")
		}

		body += tool.Baro_card(tool.Baro_t("대상 문서"), tool.Baro_kv_grid([][]string{
			{"문서", tool.Baro_doc_link(title)},
			{tool.Baro_t("존재 여부"), status_html + " " + already},
			{"길이", baro_mod_esc(length_html)},
			{tool.Baro_t("현재 판"), baro_mod_muted(tool.Get_document_revision(db, title))},
			{tool.Baro_t("문서 상태"), tool.Baro_status_badge(doc_status)},
			{tool.Baro_t("마지막 기여자"), tool.Baro_user_link(db, baro_mod_doc_last_editor(db, title), config.IP)},
		}))
	}

	form := tool.Baro_input("title", tool.Baro_t("문서 이름"), title, tool.Baro_t("격리할 문서"), tool.Baro_t("정확한 문서 이름을 적습니다."))
	form += tool.Baro_input("rev", tool.Baro_t("판 번호"), "", tool.Baro_t("비우면 현재 판"), tool.Baro_t("특정 판만 문제라면 그 판 번호를 적습니다."))
	form += tool.Baro_select("kind", tool.Baro_t("격리 종류"), baro_mod_quarantine_kind_list(), kind, tool.Baro_t("저작권 · 개인정보는 긴급 비공개 대상입니다."))
	form += tool.Baro_textarea("reason", "사유", "", 5, tool.Baro_t("왜 격리하는지 적습니다."), tool.Baro_t("필수 항목입니다. 사유 없는 격리는 남길 수 없습니다."))
	form += tool.Baro_input("case_id", tool.Baro_t("연결 사건"), case_key, tool.Baro_t("사건 번호 또는 내부 id"), tool.Baro_t("비워 둘 수 있습니다."))
	form += tool.Baro_checkbox("hide", "본문을 비공개 처리한다 (원본은 그대로 보존)", "on", kind == "privacy" || kind == "copyright", "개인정보·저작권 침해는 반드시 켜야 합니다.")
	form += tool.Baro_submit(tool.Baro_t("임시 격리"), "danger")

	body += tool.Baro_card(tool.Baro_t("격리 정보"), tool.Baro_form("/baro/quarantine/new", form))

	body += tool.Baro_card(tool.Baro_t("격리하면 이렇게 됩니다"),
		tool.Baro_list([]string{
			tool.Baro_t("원본 내용이 <b>격리 기록</b>에 통째로 저장됩니다."),
			tool.Baro_t("문서 상태가 '격리 중'으로 바뀝니다."),
			tool.Baro_t("본문 비공개를 켜면 본문 자리에 안내문이 들어가고, 원본은 격리 기록에 남습니다."),
			tool.Baro_t("마지막 기여자에게 알림이 갑니다. 기여가 삭제된 것이 아니라는 사실을 알려야 합니다."),
			tool.Baro_t("공개 로그에 문서 이름과 사유가 남습니다. 신고자 정보는 남지 않습니다."),
		}),
	)

	return baro_mod_shell(db, config, "문서 임시 격리", "/baro/quarantine", body)
}

// baro_mod_quarantine_new_post : 격리 등록 처리
func baro_mod_quarantine_new_post(db *sql.DB, config tool.Config, values url.Values) string {
	title := baro_mod_val(values, "title")
	rev := baro_mod_val(values, "rev")
	kind := baro_mod_or(baro_mod_val(values, "kind"), "normal")
	reason := tool.Baro_trunc(baro_mod_raw(values, "reason"), 4000)
	case_key := baro_mod_val(values, "case_id")
	hide := baro_mod_on(values, "hide")

	fail := func(message string) string {
		body := tool.Baro_alert("danger", baro_mod_esc(message))
		body += tool.Baro_card("", tool.Baro_button("/baro/quarantine/new", "돌아가기", "primary"))

		return baro_mod_shell(db, config, "문서 임시 격리", "/baro/quarantine", body)
	}

	if title == "" {
		return fail(tool.Baro_t("문서 이름을 적어 주세요."))
	}

	if strings.TrimSpace(reason) == "" {
		return fail(tool.Baro_t("격리 사유는 반드시 적어야 합니다."))
	}

	if !baro_mod_doc_exists(db, title) {
		return fail("'" + title + tool.Baro_t("' 문서를 찾을 수 없습니다. 문서 이름을 확인해 주세요."))
	}

	case_id := ""
	if case_row, ok := baro_mod_case_get(db, case_key); ok {
		case_id = case_row.ID
	}

	id := baro_mod_quarantine_add(db, config, title, rev, kind, reason, case_id, hide)

	return tool.Get_redirect("/baro/quarantine/view/" + tool.Url_parser(id))
}

// ------------------------------------------------------------
//  격리 상세
// ------------------------------------------------------------

// View_baro_mod_quarantine_view : 격리 상세 (/baro/quarantine/view/<id>)
func View_baro_mod_quarantine_view(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query

	row, ok := baro_mod_quarantine_get(db, arg)
	if !ok {
		return baro_mod_missing(db, config, "/baro/quarantine", tool.Baro_t("격리 기록"), "/baro/quarantine", tool.Baro_t("격리 목록으로"))
	}

	can_handle := tool.Baro_can(db, config.IP, "quarantine")

	if values != nil {
		if !can_handle {
			return tool.Baro_deny(db, config, tool.Baro_t("격리를 처리할 권한이 없습니다."))
		}

		return baro_mod_quarantine_view_post(db, config, row, values)
	}

	body := tool.Baro_crumb([][]string{{"/baro/quarantine", tool.Baro_t("문서 격리")}, {"", "격리번호 " + row.ID}})

	now_status := tool.Baro_doc_status_get(db, row.Title)
	_, exists := baro_mod_doc_data(db, row.Title)

	exists_html := tool.Baro_badge(tool.Baro_t("문서 없음"), "danger")
	if exists {
		exists_html = tool.Baro_badge(tool.Baro_t("문서 있음"), "ok")
	}

	body += tool.Baro_card(tool.Baro_t("격리 정보"), tool.Baro_kv_grid([][]string{
		{"격리번호", baro_mod_esc(row.ID)},
		{"문서", tool.Baro_doc_link(row.Title)},
		{"판", baro_mod_muted(row.Rev)},
		{"종류", baro_mod_esc(baro_mod_quarantine_kind_name(row.Kind))},
		{"상태", baro_mod_status_badge(row.Status)},
		{tool.Baro_t("현재 문서"), exists_html + " " + tool.Baro_status_badge(now_status)},
		{tool.Baro_t("격리 시각"), baro_mod_esc(row.Date)},
		{tool.Baro_t("해제 시각"), baro_mod_muted(row.ReleaseDate)},
		{tool.Baro_t("연결 사건"), baro_mod_case_link(db, row.CaseID)},
		{tool.Baro_t("집행자"), baro_mod_esc(tool.Baro_mask_ip(row.Actor))},
	}))

	body += tool.Baro_card(tool.Baro_t("격리 사유"), tool.Baro_code(row.Reason))

	// 편집이 막혀 있다는 것을 저장 버튼을 누르기 전에 알려 준다.
	body += baro_mod_quarantine_edit_alert(db, config, row.Title, row.Status)

	if !can_handle {
		body += tool.Baro_alert("info",
			tool.Baro_t("원본 내용은 담당자만 볼 수 있습니다. 원본은 삭제되지 않고 그대로 보존되어 있으며, ")+
				tool.Baro_t("검토가 끝나면 복구되거나 사유를 밝힌 뒤 삭제됩니다."))
		body += tool.Baro_note(tool.Baro_t("보존된 원본 길이: ") + tool.Baro_comma(tool.Get_len(row.Backup)) + tool.Baro_t("자"))

		return baro_mod_shell(db, config, "격리 "+row.ID, "/baro/quarantine", body)
	}

	// 담당자 전용 : 원본 내용
	body += tool.Baro_card(tool.Baro_t("보존된 원본 (담당자 전용)"),
		tool.Baro_alert("warn", tool.Baro_t("이 내용은 담당자만 볼 수 있습니다. 개인정보가 들어 있을 수 있으니 밖으로 옮기지 마세요."))+
			tool.Baro_note("길이 "+tool.Baro_comma(tool.Get_len(row.Backup))+tool.Baro_t("자"))+
			tool.Baro_code(row.Backup),
	)

	if row.Status == "open" {
		// 복구
		restore_form := tool.Baro_hidden("act", "restore")
		restore_form += tool.Baro_textarea("reason", tool.Baro_t("복구 사유"), "", 3, tool.Baro_t("예: 검토 결과 문제가 없었습니다."), tool.Baro_t("공개 로그에 남습니다."))
		restore_form += tool.Baro_submit(tool.Baro_t("원본 그대로 복구"), "primary")

		body += tool.Baro_card("복구", tool.Baro_form("/baro/quarantine/view/"+tool.Url_parser(row.ID), restore_form))

		// 수정 후 복구
		edit_form := tool.Baro_hidden("act", "restore_edit")
		edit_form += tool.Baro_textarea("data", tool.Baro_t("수정한 내용"), row.Backup, 16, "", tool.Baro_t("문제가 되는 부분만 고쳐서 되돌립니다. 원본 기록은 그대로 남습니다."))
		edit_form += tool.Baro_textarea("reason", tool.Baro_t("수정 사유"), "", 3, tool.Baro_t("예: 저작권 침해 문단만 덜어 냈습니다."), tool.Baro_t("공개 로그에 남습니다."))
		edit_form += tool.Baro_submit("수정 후 복구", "")

		body += tool.Baro_card("수정 후 복구", tool.Baro_form("/baro/quarantine/view/"+tool.Url_parser(row.ID), edit_form))

		body += tool.Baro_danger_zone("완전 삭제",
			tool.Baro_note(tool.Baro_t("복구가 아니라 삭제를 선택하려면 별도 확인 화면에서 사유를 다시 적어야 합니다."))+
				tool.Baro_button("/baro/quarantine/delete/"+tool.Url_parser(row.ID), "완전 삭제 화면으로", "danger"),
		)
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("이미 처리가 끝난 격리 기록입니다. 보존된 원본은 계속 남아 있습니다."))

		if row.Status == "deleted" {
			restore_form := tool.Baro_hidden("act", "restore")
			restore_form += tool.Baro_textarea("reason", tool.Baro_t("되살리는 사유"), "", 3, tool.Baro_t("삭제 판단을 뒤집는 이유"), "")
			restore_form += tool.Baro_submit(tool.Baro_t("삭제된 문서를 원본으로 되살리기"), "primary")

			body += tool.Baro_card("되살리기", tool.Baro_form("/baro/quarantine/view/"+tool.Url_parser(row.ID), restore_form))
		}
	}

	body += tool.Baro_card(tool.Baro_t("이 문서의 다른 격리 기록"), baro_mod_quarantine_same_doc(db, row))

	body += baro_mod_principle_note()

	return baro_mod_shell(db, config, "격리 "+row.ID, "/baro/quarantine", body)
}

// baro_mod_quarantine_edit_badge : 이 문서를 지금 편집할 수 있는지 한 칸으로 보여 준다
//
//	tool.Baro_doc_hidden 이 true 면 보는 사람에게는 편집도 막혀 있다.
func baro_mod_quarantine_edit_badge(db *sql.DB, config tool.Config, title string) string {
	if !tool.Baro_doc_quarantined(db, title) {
		return tool.Baro_badge(tool.Baro_t("편집 가능"), "muted")
	}

	if tool.Baro_doc_hidden(db, config, title) {
		return tool.Baro_badge(tool.Baro_t("편집 잠김"), "danger")
	}

	return tool.Baro_badge(tool.Baro_t("담당자만 편집"), "warn")
}

// baro_mod_quarantine_edit_notice : 목록 맨 위에 붙는 편집 잠김 안내
func baro_mod_quarantine_edit_notice(can_handle bool) string {
	text := tool.Baro_t("<b>격리 중인 문서는 편집이 막혀 있습니다.</b> 아래 목록에서 '편집 잠김'으로 표시된 문서는 ") +
		tool.Baro_t("편집 화면을 열 수는 있어도 저장이 되지 않습니다. 헛수고를 하지 않도록 먼저 여기서 확인해 주세요. ") +
		tool.Baro_t("내용은 지워지지 않고 그대로 보존되며, 검토가 끝나 복구되면 다시 편집할 수 있습니다.")
	if can_handle {
		text = tool.Baro_t("<b>격리 중인 문서는 담당자만 편집할 수 있습니다.</b> 아래 목록의 '편집' 칸에서 ") +
			tool.Baro_t("일반 사용자에게 편집이 막혀 있는 문서를 확인할 수 있습니다.")
	}

	return tool.Baro_alert("warn", text)
}

// baro_mod_quarantine_edit_alert : 격리 상세에서 편집이 막혀 있음을 알린다
func baro_mod_quarantine_edit_alert(db *sql.DB, config tool.Config, title string, status string) string {
	if !tool.Baro_doc_quarantined(db, title) {
		if status == "open" {
			return ""
		}

		return tool.Baro_alert("ok",
			tool.Baro_t("이 문서는 지금 격리 상태가 아닙니다. <b>편집할 수 있습니다.</b> ")+
				tool.Baro_button("/edit/"+tool.Url_parser(title), "편집 화면 열기", ""))
	}

	if tool.Baro_doc_hidden(db, config, title) {
		return tool.Baro_alert("danger",
			tool.Baro_t(`<b>이 문서는 편집이 막혀 있습니다.</b> 격리 검토가 끝날 때까지 `)+
				tool.Baro_doc_link(title)+tool.Baro_t(` 문서는 저장할 수 없습니다. `)+
				tool.Baro_t("편집 화면(/edit/)이 열리더라도 저장 단계에서 되돌아오니 지금은 고쳐 쓰지 마세요. ")+
				tool.Baro_t("원본은 그대로 보존되어 있으며, 복구되면 곧바로 다시 편집할 수 있습니다. ")+
				tool.Baro_t("고칠 부분이 있으면 아래 격리 사유를 확인하고 담당자에게 알려 주세요."))
	}

	return tool.Baro_alert("warn",
		tool.Baro_t("<b>이 문서는 일반 사용자에게 편집이 막혀 있습니다.</b> 격리 담당자인 당신은 편집할 수 있지만, ")+
			tool.Baro_t("보통은 편집 대신 아래 '수정 후 복구'를 써서 기록을 남기는 것이 좋습니다."))
}

// baro_mod_quarantine_same_doc : 같은 문서의 다른 격리 기록
func baro_mod_quarantine_same_doc(db *sql.DB, row baro_mod_quarantine_row) string {
	rows := tool.Query_DB(
		db,
		"select id, kind, status, date, reason from baro_quarantine where title = ? and id <> ? order by id + 0 desc limit 20",
		row.Title, row.ID,
	)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		id, kind, status, date, reason := "", "", "", "", ""
		if err := rows.Scan(&id, &kind, &status, &date, &reason); err != nil {
			continue
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/quarantine/view/` + tool.Url_parser(id) + `">` + baro_mod_esc(id) + `</a>`,
			baro_mod_esc(baro_mod_quarantine_kind_name(kind)),
			baro_mod_status_badge(status),
			baro_mod_esc(tool.Baro_trunc(tool.Baro_one_line(reason), 120)),
			tool.Baro_time_cell(date),
		})
	}

	return tool.Baro_table([]string{"번호", "종류", "상태", "사유", "시각"}, table_rows, tool.Baro_t("다른 격리 기록이 없습니다."))
}

// baro_mod_quarantine_view_post : 격리 상세 처리 (복구 / 수정 후 복구)
func baro_mod_quarantine_view_post(db *sql.DB, config tool.Config, row baro_mod_quarantine_row, values url.Values) string {
	target_url := "/baro/quarantine/view/" + tool.Url_parser(row.ID)
	act := baro_mod_val(values, "act")
	reason := tool.Baro_trunc(baro_mod_raw(values, "reason"), 2000)

	switch act {
	case "restore", "restore_edit":
		data := row.Backup
		send := tool.Baro_t("격리 복구 (격리번호 ") + row.ID + ")"

		if act == "restore_edit" {
			data = baro_mod_raw(values, "data")
			send = tool.Baro_t("격리 수정 후 복구 (격리번호 ") + row.ID + ")"
		}

		// 원본(또는 수정본)을 data 표로 되돌리고 역사를 남긴다
		baro_mod_doc_write(db, config, row.Title, data, send)

		tool.Baro_doc_status_set(db, row.Title, "", "", config.IP)

		now := tool.Baro_now()
		tool.Exec_DB(db, "update baro_quarantine set status = 'released', release_date = ? where id = ?", now, row.ID)

		if row.CaseID != "" {
			tool.Exec_DB(
				db,
				"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'restore', ?, ?, ?)",
				tool.Baro_next_id(db, "baro_case_action"), row.CaseID, config.IP,
				"격리를 해제하고 문서를 복구했습니다. (격리번호 "+row.ID+")\n사유: "+reason, now, row.Title,
			)
		}

		baro_mod_log_both(db, config, "quarantine_release", row.Title, reason,
			baro_mod_quarantine_kind_name(row.Kind), row.Status, "released")

		if editor := baro_mod_doc_last_editor(db, row.Title); editor != "" {
			baro_mod_notify_user(db, editor, "case",
				tool.Baro_t("문서 '")+tool.Namespace_display(row.Title)+tool.Baro_t("' 격리가 해제되었습니다."),
				"/w/"+tool.Url_parser(row.Title), reason)
		}

		tool.Baro_metric_bump(db, "quarantine_release", tool.Baro_today(), 1)

		return tool.Get_redirect(target_url)
	}

	return tool.Get_redirect(target_url)
}

// ------------------------------------------------------------
//  완전 삭제 (별도 확인 화면)
// ------------------------------------------------------------

// View_baro_mod_quarantine_delete : 완전 삭제 확인 (/baro/quarantine/delete/<id>)
func View_baro_mod_quarantine_delete(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query

	if !tool.Baro_can(db, config.IP, "quarantine") {
		return tool.Baro_deny(db, config, tool.Baro_t("문서 삭제는 격리 처리 권한이 있어야 합니다."))
	}

	row, ok := baro_mod_quarantine_get(db, arg)
	if !ok {
		return baro_mod_missing(db, config, "/baro/quarantine", tool.Baro_t("격리 기록"), "/baro/quarantine", tool.Baro_t("격리 목록으로"))
	}

	if values != nil {
		return baro_mod_quarantine_delete_post(db, config, row, values)
	}

	body := tool.Baro_crumb([][]string{
		{"/baro/quarantine", tool.Baro_t("문서 격리")},
		{"/baro/quarantine/view/" + tool.Url_parser(row.ID), "격리번호 " + row.ID},
		{"", "완전 삭제"},
	})

	body += tool.Baro_alert("danger",
		tool.Baro_t("<b>잠깐 멈추세요.</b> 바로위키의 기본 처리는 복구입니다. 삭제는 복구가 불가능하다고 판단했을 때만 선택합니다. ")+
			tool.Baro_t("삭제하더라도 원본은 격리 기록에 계속 보존되며, 문서는 휴지통으로 옮겨집니다."))

	body += tool.Baro_card(tool.Baro_t("삭제 대상"), tool.Baro_kv_grid([][]string{
		{"격리번호", baro_mod_esc(row.ID)},
		{"문서", tool.Baro_doc_link(row.Title)},
		{"종류", baro_mod_esc(baro_mod_quarantine_kind_name(row.Kind))},
		{tool.Baro_t("격리 사유"), baro_mod_esc(tool.Baro_trunc(row.Reason, 400))},
		{tool.Baro_t("보존된 원본 길이"), baro_mod_esc(tool.Baro_comma(tool.Get_len(row.Backup)) + tool.Baro_t("자"))},
		{tool.Baro_t("연결 사건"), baro_mod_case_link(db, row.CaseID)},
	}))

	form := tool.Baro_textarea("reason", tool.Baro_t("삭제 사유"), "", 4, tool.Baro_t("왜 삭제해야 하는지 다시 적어 주세요."), tool.Baro_t("필수 항목입니다. 격리 사유를 그대로 복사하지 마세요."))
	form += tool.Baro_textarea("why_not_restore", tool.Baro_t("왜 복구가 아니라 삭제인가"), "", 4, tool.Baro_t("복구·수정으로 해결할 수 없는 이유"), tool.Baro_t("필수 항목입니다. 이 답이 없으면 삭제할 수 없습니다."))
	form += tool.Baro_input("confirm", tool.Baro_t("확인 문구"), "", "삭제", tool.Baro_t("삭제 라고 정확히 입력해야 진행됩니다."))
	form += tool.Baro_checkbox("agree", tool.Baro_t("복구·수정으로는 해결할 수 없음을 확인했습니다"), "on", false, "")
	form += tool.Baro_submit("완전 삭제", "danger")

	body += tool.Baro_danger_zone("완전 삭제", tool.Baro_form("/baro/quarantine/delete/"+tool.Url_parser(row.ID), form))

	body += tool.Baro_card("", tool.Baro_button("/baro/quarantine/view/"+tool.Url_parser(row.ID), "복구 화면으로 돌아가기", "primary"))

	return baro_mod_shell(db, config, tool.Baro_t("완전 삭제 확인"), "/baro/quarantine", body)
}

// baro_mod_quarantine_delete_post : 완전 삭제 처리
func baro_mod_quarantine_delete_post(db *sql.DB, config tool.Config, row baro_mod_quarantine_row, values url.Values) string {
	reason := tool.Baro_trunc(baro_mod_raw(values, "reason"), 2000)
	why_not := tool.Baro_trunc(baro_mod_raw(values, "why_not_restore"), 2000)
	confirm := baro_mod_val(values, "confirm")

	fail := func(message string) string {
		body := tool.Baro_alert("danger", baro_mod_esc(message))
		body += tool.Baro_card("", tool.Baro_button("/baro/quarantine/delete/"+tool.Url_parser(row.ID), "돌아가기", "primary"))

		return baro_mod_shell(db, config, tool.Baro_t("완전 삭제 확인"), "/baro/quarantine", body)
	}

	if strings.TrimSpace(reason) == "" {
		return fail(tool.Baro_t("삭제 사유를 적어 주세요."))
	}

	if strings.TrimSpace(why_not) == "" {
		return fail(tool.Baro_t("왜 복구가 아니라 삭제인지 적어 주세요. 이 답이 없으면 삭제할 수 없습니다."))
	}

	if confirm != "삭제" {
		return fail(tool.Baro_t("확인 문구에 '삭제' 라고 정확히 입력해 주세요."))
	}

	if !baro_mod_on(values, "agree") {
		return fail(tool.Baro_t("확인란을 체크해 주세요."))
	}

	now := tool.Baro_now()
	data, exists := baro_mod_doc_data(db, row.Title)
	if !exists {
		data = row.Backup
	}

	// 원본은 격리 기록에 남기고, 문서는 휴지통으로 옮긴 뒤 본문에서 제거한다
	tool.Do_move_to_trash(db, row.Title, data, config.IP, "["+row.ID+"] "+reason)
	tool.Exec_DB(db, "delete from data where title = ?", row.Title)
	tool.Do_add_history(db, row.Title, "", now, config.IP, reason, "0", "delete", "")

	tool.Baro_doc_status_set(db, row.Title, "archived", reason, config.IP)

	tool.Exec_DB(
		db,
		"update baro_quarantine set status = 'deleted', release_date = ?, reason = ? where id = ?",
		now, row.Reason+"\n\n[완전 삭제] "+reason+"\n[복구하지 않는 이유] "+why_not, row.ID,
	)

	if row.CaseID != "" {
		tool.Exec_DB(
			db,
			"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'note', ?, ?, ?)",
			tool.Baro_next_id(db, "baro_case_action"), row.CaseID, config.IP,
			"문서를 완전 삭제했습니다. (격리번호 "+row.ID+")\n사유: "+reason+"\n복구하지 않는 이유: "+why_not,
			now, row.Title,
		)
	}

	tool.Baro_public_log(db, "document_delete", config.IP, row.Title, tool.Baro_trunc(tool.Baro_one_line(reason), 200), "복구하지 않는 이유: "+tool.Baro_trunc(tool.Baro_one_line(why_not), 200))
	tool.Baro_audit(db, config, "document_delete", row.Title, tool.Baro_trunc(data, 1000), "deleted", reason+" / "+why_not)
	tool.Baro_metric_bump(db, "document_delete", tool.Baro_today(), 1)

	if editor := baro_mod_doc_last_editor(db, row.Title); editor != "" {
		baro_mod_notify_user(db, editor, "case",
			tool.Baro_t("문서 '")+tool.Namespace_display(row.Title)+tool.Baro_t("' 이(가) 삭제되었습니다. 사유와 근거를 확인할 수 있습니다."),
			"/baro/quarantine/view/"+tool.Url_parser(row.ID), reason)
	}

	return tool.Get_redirect("/baro/quarantine/view/" + tool.Url_parser(row.ID))
}

// ------------------------------------------------------------
//  사용자별 문서 다루기 (10-4 의 핵심 화면)
// ------------------------------------------------------------

// View_baro_mod_quarantine_user : 제재된 사용자의 문서를 어떻게 다루는지 보여 준다
func View_baro_mod_quarantine_user(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	user := strings.TrimSpace(arg)
	if user == "" {
		user = baro_mod_val(query, "user")
	}

	page := baro_mod_page_num(baro_mod_or(baro_mod_val(query, "page"), "1"))
	size := tool.Baro_page_size

	body := tool.Baro_crumb([][]string{{"/baro/quarantine", tool.Baro_t("문서 격리")}, {"", tool.Baro_t("사용자별 문서")}})

	body += tool.Baro_alert("warn",
		tool.Baro_t("<b>계정이 제재되어도 정상 기여는 유지됩니다.</b> 이 화면에는 '전부 삭제' 같은 일괄 버튼이 없습니다. ")+
			tool.Baro_t("공동 기여 문서는 문서 단위가 아니라 <b>리비전 단위</b>로 판단해야 합니다. ")+
			tool.Baro_t("여러 문서를 한꺼번에 처리해야 한다면 <a href=\"/baro/mass/new\">대량 작업</a>의 안전장치를 거치세요."))

	search_form := `<form class="bw_form" method="get" action="/baro/quarantine/user">`
	search_form += tool.Baro_input("user", tool.Baro_t("사용자"), user, tool.Baro_t("사용자 이름 또는 IP"), "")
	search_form += tool.Baro_submit(tool.Baro_t("조회"), "")
	search_form += `</form>`

	body += tool.Baro_card(tool.Baro_t("사용자 찾기"), search_form)

	if user == "" {
		body += tool.Baro_empty(tool.Baro_t("조회할 사용자를 입력해 주세요."))

		return baro_mod_shell(db, config, tool.Baro_t("사용자별 문서"), "/baro/quarantine", body)
	}

	// 작성자(사람) 상태 — 문서 상태와 반드시 분리해서 보여 준다
	sanction_list := tool.Baro_sanction_active(db, user)
	sanction_html := tool.Baro_badge(tool.Baro_t("제재 없음"), "ok")
	if len(sanction_list) > 0 {
		parts := []string{}
		for _, item := range sanction_list {
			parts = append(parts, tool.Baro_sanction_name(item[1])+" ("+baro_mod_or(item[4], "무기한")+")")
		}

		sanction_html = tool.Baro_badge("제재 "+tool.Baro_itoa(len(sanction_list))+tool.Baro_t("건"), "danger") + " " + baro_mod_esc(strings.Join(parts, ", "))
	}

	edit_count := 0
	tool.QueryRow_DB(db, "select count(*) from history where ip = ?", []any{&edit_count}, user)

	body += tool.Baro_card(tool.Baro_t("작성자 상태"), tool.Baro_kv_grid([][]string{
		{tool.Baro_t("사용자"), tool.Baro_user_link(db, user, config.IP)},
		{tool.Baro_t("제재 상태"), sanction_html},
		{tool.Baro_t("기여 판 수"), baro_mod_esc(tool.Baro_comma(edit_count))},
		{"제재 이력", `<a href="/baro/case/sanction/list?user=` + tool.Url_parser(user) + `">제재 목록 보기</a>`},
	}))

	// 이 사용자가 손댄 문서 목록
	rows := tool.Query_DB(
		db,
		"select distinct title from history where ip = ? order by title asc limit ?, ?",
		user, (page-1)*size, size,
	)
	defer rows.Close()

	titles := []string{}
	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		titles = append(titles, title)
	}

	table_rows := [][]string{}
	for _, title := range titles {
		editors := baro_mod_doc_editors(db, title)

		// 공동 기여 여부 — 리비전 단위 판단이 필요한 문서
		joint_html := tool.Baro_badge(tool.Baro_t("단독 기여"), "info")
		if len(editors) > 1 {
			joint_html = tool.Baro_badge(tool.Baro_t("공동 기여 ")+tool.Baro_itoa(len(editors))+tool.Baro_t("명"), "warn")
		}

		my_rev := 0
		tool.QueryRow_DB(db, "select count(*) from history where title = ? and ip = ?", []any{&my_rev}, title, user)

		all_rev := 0
		tool.QueryRow_DB(db, "select count(*) from history where title = ?", []any{&all_rev}, title)

		doc_status := tool.Baro_doc_status_get(db, title)
		doc_status_html := tool.Baro_status_badge(doc_status)
		if tool.Baro_doc_quarantined(db, title) {
			doc_status_html = tool.Baro_badge(tool.Baro_t("격리 중"), "danger")
		}

		exists_html := tool.Baro_badge(tool.Baro_t("있음"), "ok")
		if !baro_mod_doc_exists(db, title) {
			exists_html = tool.Baro_badge("없음", "muted")
		}

		manage := `<a href="/history/` + tool.Url_parser(title) + tool.Baro_t(`">역사</a>`)
		if tool.Baro_can(db, config.IP, "quarantine") {
			manage += ` · <a href="/baro/quarantine/new?title=` + tool.Url_parser(title) + tool.Baro_t(`">개별 격리</a>`)
		}

		table_rows = append(table_rows, []string{
			tool.Baro_doc_link(title),
			exists_html,
			// 작성자 상태 (사람) 와 문서 상태 (콘텐츠) 를 별도 열로 나눈다
			baro_mod_sanction_badge_of(sanction_list),
			doc_status_html,
			joint_html,
			baro_mod_esc(tool.Baro_itoa(my_rev) + " / " + tool.Baro_itoa(all_rev)),
			manage,
		})
	}

	list_body := tool.Baro_table(
		[]string{"문서", tool.Baro_t("존재"), tool.Baro_t("작성자 상태"), tool.Baro_t("문서 상태"), tool.Baro_t("기여 형태"), tool.Baro_t("이 사람 판 / 전체 판"), "관리"},
		table_rows,
		"",
	)

	if len(table_rows) == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("이 사용자가 손댄 문서를 찾지 못했습니다."))
	}

	list_body += tool.Baro_paging(page, len(table_rows), size, "/baro/quarantine/user/"+tool.Url_parser(user)+"?page={}")

	body += tool.Baro_card(tool.Baro_t("이 사용자가 기여한 문서"), list_body)

	body += tool.Baro_card(tool.Baro_t("이 표를 읽는 법"),
		tool.Baro_list([]string{
			tool.Baro_t("<b>작성자 상태</b>는 사람에 대한 판단입니다. 제재 여부가 여기에 나옵니다."),
			tool.Baro_t("<b>문서 상태</b>는 콘텐츠에 대한 판단입니다. 두 값은 서로 독립입니다."),
			tool.Baro_t("작성자가 차단되었다는 사실만으로 문서 상태를 바꾸지 않습니다. (고정 원칙 1)"),
			tool.Baro_t("<b>공동 기여</b> 문서는 그 사람의 판만 문제일 수 있습니다. 문서 전체가 아니라 <b>리비전 단위</b>로 판단하세요."),
			tool.Baro_t("'이 사람 판 / 전체 판' 비율이 낮을수록 문서 전체를 건드리면 안 됩니다."),
		}),
	)

	body += tool.Baro_alert("info",
		tool.Baro_t("여러 문서를 한꺼번에 되돌리거나 격리해야 한다면 <a href=\"/baro/mass/new\">대량 작업</a>을 이용하세요. ")+
			tool.Baro_t("대상 수 표시 · 목록 공개 · 2인 승인 · 스냅샷 · 유예 · 일괄 복구를 거칩니다."))

	body += baro_mod_principle_note()

	return baro_mod_shell(db, config, tool.Baro_t("사용자별 문서 · ")+user, "/baro/quarantine", body)
}

// baro_mod_sanction_badge_of : 제재 목록을 한 칸짜리 배지로
func baro_mod_sanction_badge_of(list [][]string) string {
	if len(list) == 0 {
		return tool.Baro_badge(tool.Baro_t("제재 없음"), "ok")
	}

	return tool.Baro_badge(tool.Baro_sanction_name(list[0][1]), "danger")
}
