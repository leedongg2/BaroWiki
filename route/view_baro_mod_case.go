package route

// ============================================================
//  10-2 운영 사건 시스템 (/baro/case)
//
//  모든 운영 조치에는 사건 번호(BW-연도-순번)가 붙는다.
//  공개 요약과 적용 규정은 누구나 볼 수 있고,
//  비공개 정보와 증거 원문은 담당자만 볼 수 있다. (5번 원칙)
// ============================================================

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ------------------------------------------------------------
//  사건 목록
// ------------------------------------------------------------

// View_baro_mod_case_list : 사건 목록 (/baro/case)
func View_baro_mod_case_list(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	page := baro_mod_page_num(arg)
	size := tool.Baro_page_size

	status := baro_mod_val(query, "status")
	kind := baro_mod_val(query, "kind")

	where := " where 1 = 1"
	params := []any{}

	if status == "open" {
		// 예전 자료에는 접수 상태가 new 로 적혀 있다
		where += " and status" + baro_mod_status_new_sql
	} else if status != "" && status != "all" {
		where += " and status = ?"
		params = append(params, status)
	}

	// 옛 이름으로 저장된 유형까지 함께 찾는다
	if kind != "" {
		alias := baro_mod_report_kind_alias_all(kind)
		where += " and kind in (" + baro_mod_place_holders(len(alias)) + ")"

		for _, item := range alias {
			params = append(params, item)
		}
	}

	params = append(params, (page-1)*size, size)

	rows := tool.Query_DB(
		db,
		"select id, number, kind, target_kind, target, rule, status, public_summary, date, close_date, appeal_status, priority from baro_case"+where+" order by id + 0 desc limit ?, ?",
		params...,
	)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0

	for rows.Next() {
		id, number, c_kind, target_kind, target, rule, c_status, summary, date, close_date, appeal_status, priority := "", "", "", "", "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &number, &c_kind, &target_kind, &target, &rule, &c_status, &summary, &date, &close_date, &appeal_status, &priority); err != nil {
			continue
		}

		count++

		summary_html := tool.Baro_t(`<span class="bw_muted">공개 요약이 아직 작성되지 않았습니다.</span>`)
		if strings.TrimSpace(summary) != "" {
			summary_html = baro_mod_esc(tool.Baro_trunc(tool.Baro_one_line(summary), 120))
		}

		appeal_html := `<span class="bw_muted">-</span>`
		if appeal_status != "" {
			appeal_html = baro_mod_status_badge(appeal_status)
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/case/view/` + tool.Url_parser(id) + `">` + baro_mod_esc(baro_mod_or(number, id)) + `</a>`,
			baro_mod_esc(baro_mod_report_kind_name(c_kind)),
			baro_mod_target_link(db, config, target_kind, target, true),
			baro_mod_policy_link(db, rule),
			summary_html,
			baro_mod_status_badge(c_status),
			appeal_html,
			tool.Baro_time_cell(baro_mod_or(close_date, date)),
		})
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("운영 사건")}})

	body += tool.Baro_hero(
		tool.Baro_t("운영 사건"),
		tool.Baro_t("모든 운영 조치에는 사건 번호가 붙습니다. 조치 사유와 적용 규정은 공개하고, 개인정보는 공개하지 않습니다."),
		"",
	)

	total, open_count, closed_count, appeal_count := 0, 0, 0, 0
	tool.QueryRow_DB(db, "select count(*) from baro_case", []any{&total})
	tool.QueryRow_DB(db, "select count(*) from baro_case where status"+baro_mod_status_open_sql, []any{&open_count})
	tool.QueryRow_DB(db, "select count(*) from baro_case where status = 'closed'", []any{&closed_count})
	tool.QueryRow_DB(db, "select count(*) from baro_appeal where status"+baro_mod_status_open_sql, []any{&appeal_count})

	body += tool.Baro_card(tool.Baro_t("사건 현황"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 사건"), tool.Baro_comma(total), ""},
		{tool.Baro_t("진행 중"), tool.Baro_comma(open_count), ""},
		{tool.Baro_t("종결"), tool.Baro_comma(closed_count), ""},
		{tool.Baro_t("이의제기 심사 중"), tool.Baro_comma(appeal_count), ""},
	}))

	filter_form := `<form class="bw_form" method="get" action="/baro/case">`
	filter_form += tool.Baro_select("status", "상태", append([][]string{{"", tool.Baro_t("전체")}}, baro_mod_status_list()...), status, "")
	filter_form += tool.Baro_select("kind", tool.Baro_t("유형"), append([][]string{{"", tool.Baro_t("전체")}}, baro_mod_report_kind_options()...), kind, "")
	filter_form += tool.Baro_submit(tool.Baro_t("적용"), "")
	filter_form += `</form>`

	body += tool.Baro_card("걸러 보기", filter_form)

	list_body := tool.Baro_table(
		[]string{"사건 번호", tool.Baro_t("유형"), tool.Baro_t("대상"), tool.Baro_t("적용 규정"), tool.Baro_t("공개 요약"), "상태", tool.Baro_t("이의제기"), tool.Baro_t("최근")},
		table_rows,
		"",
	)
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("조건에 맞는 사건이 없습니다."))
	}

	list_body += tool.Baro_paging(page, count, size, "/baro/case/list/{}?status="+tool.Url_parser(status)+"&kind="+tool.Url_parser(kind))

	body += tool.Baro_card(tool.Baro_t("사건 목록"), list_body)

	if tool.Baro_can(db, config.IP, "case") {
		body += tool.Baro_card(tool.Baro_t("담당자 기능"),
			tool.Baro_button("/baro/case/new", tool.Baro_t("사건 만들기"), "primary")+" "+
				tool.Baro_button("/baro/report/queue", tool.Baro_t("신고 큐"), "")+" "+
				tool.Baro_button("/baro/case/sanction/list", tool.Baro_t("적용 중인 제재"), ""),
		)
	}

	body += tool.Baro_note(tool.Baro_t("이 목록에는 공개 요약만 표시됩니다. 비공개 정보와 증거 원문은 담당자만 볼 수 있습니다."))

	return baro_mod_shell(db, config, tool.Baro_t("운영 사건"), "/baro/case", body)
}

// ------------------------------------------------------------
//  사건 만들기
// ------------------------------------------------------------

// baro_mod_case_make : 신고 한 건에서 사건을 만든다 (병합·전환에서 함께 쓴다)
func baro_mod_case_make(db *sql.DB, config tool.Config, report baro_mod_report_row, action string, reason string) string {
	id := tool.Baro_next_id(db, "baro_case")
	number := baro_mod_case_number_make(db)
	now := tool.Baro_now()
	deadline := tool.Baro_time_add(now, 14, 0, 0)

	tool.Exec_DB(
		db,
		"insert into baro_case (id, number, kind, target_kind, target, rule, actor, status, action, public_summary, private_note, date, close_date, deadline, appeal_status, priority) values (?, ?, ?, ?, ?, '', ?, 'open', ?, '', '', ?, '', ?, '', ?)",
		id, number, report.Kind, report.TargetKind, report.Target, config.IP, action, now, deadline, baro_mod_or(report.Priority, "2"),
	)

	tool.Exec_DB(
		db,
		"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'note', ?, ?, ?)",
		tool.Baro_next_id(db, "baro_case_action"), id, config.IP,
		"신고 접수번호 "+report.ID+" 에서 사건을 개설했습니다. "+reason, now, report.Target,
	)

	// 신고 원문을 증거로 옮겨 담되 비공개로 둔다
	tool.Exec_DB(
		db,
		"insert into baro_case_evidence (id, case_id, kind, data, actor, date, is_private) values (?, ?, 'report', ?, ?, ?, 'on')",
		tool.Baro_next_id(db, "baro_case_evidence"), id,
		"접수번호 "+report.ID+"\n\n"+report.Detail+"\n\n[증거]\n"+report.Evidence,
		config.IP, now,
	)

	tool.Exec_DB(db, "update baro_report set case_id = ?, status = 'pending' where id = ?", id, report.ID)

	tool.Baro_public_log(db, "case_open", config.IP, number, reason, baro_mod_report_kind_name(report.Kind))
	tool.Baro_audit(db, config, "case_open", number, "", report.Target, reason)
	tool.Baro_metric_bump(db, "case_open", tool.Baro_today(), 1)

	baro_mod_notify_user(db, report.Reporter, "case",
		tool.Baro_t("신고가 운영 사건 ")+number+tool.Baro_t(" 으로 전환되었습니다"),
		"/baro/case/view/"+tool.Url_parser(id), "")

	return id
}

// View_baro_mod_case_new : 사건 만들기 (/baro/case/new)
func View_baro_mod_case_new(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = arg

	if !tool.Baro_can(db, config.IP, "case") {
		return tool.Baro_deny(db, config, tool.Baro_t("사건을 만들려면 사건 처리 권한(제재 담당 · 중재자 · 순찰자)이 필요합니다."))
	}

	if page, stop := baro_mod_read_only_block(db, config, "/baro/case"); stop {
		return page
	}

	if values != nil {
		return baro_mod_case_new_post(db, config, values)
	}

	report_id := baro_mod_val(query, "report")
	report, has_report := baro_mod_report_get(db, report_id)

	kind := baro_mod_val(query, "kind")
	target_kind := baro_mod_val(query, "target_kind")
	target := baro_mod_val(query, "target")
	priority := "2"

	if has_report {
		kind = report.Kind
		target_kind = report.TargetKind
		target = report.Target
		priority = baro_mod_or(report.Priority, "2")
	}

	if kind == "" {
		kind = "doc_error"
	}
	if target_kind == "" {
		target_kind = "doc"
	}

	body := tool.Baro_crumb([][]string{{"/baro/case", tool.Baro_t("운영 사건")}, {"", tool.Baro_t("사건 만들기")}})

	if has_report {
		body += tool.Baro_alert("info",
			"접수번호 "+baro_mod_esc(report.ID)+tool.Baro_t(" 신고에서 사건을 만듭니다. 신고 원문은 비공개 증거로 자동 보관되고, 신고자에게 알림이 갑니다."))
		body += tool.Baro_card(tool.Baro_t("원 신고 내용 (담당자 전용)"), tool.Baro_code(tool.Baro_trunc(report.Detail, 2000)))
	}

	form := tool.Baro_hidden("report", report_id)
	form += tool.Baro_select("kind", tool.Baro_t("사건 유형"), baro_mod_report_kind_options(), kind, tool.Baro_t("신고 종류와 같은 분류를 씁니다."))
	form += tool.Baro_select("target_kind", tool.Baro_t("대상 종류"), baro_mod_target_kind_list(), target_kind, "")
	form += tool.Baro_input("target", tool.Baro_t("대상"), target, tool.Baro_t("문서 이름 · 사용자 이름 · 토론 번호"), tool.Baro_t("관련 문서나 사용자를 적습니다."))
	form += tool.Baro_select("rule", tool.Baro_t("적용 규정"), baro_mod_policy_options(db), "", tool.Baro_t("어떤 규정을 근거로 판단하는지 고릅니다. 공개됩니다."))
	form += tool.Baro_select("priority", tool.Baro_t("우선순위"), baro_mod_priority_list(), priority, "")
	form += tool.Baro_input_type("deadline", tool.Baro_t("처리 기한"), tool.Baro_date_only(tool.Baro_time_add(tool.Baro_now(), 14, 0, 0)), "date", "")
	form += tool.Baro_textarea("public_summary", tool.Baro_t("공개 요약"), "", 5, tool.Baro_t("누구나 볼 수 있는 요약입니다. 개인정보는 쓰지 마세요."), tool.Baro_t("공개 화면에 그대로 보입니다."))
	form += tool.Baro_textarea("private_note", tool.Baro_t("비공개 정보"), "", 5, tool.Baro_t("담당자만 보는 기록"), tool.Baro_t("IP·연락처 등 민감한 정보는 여기에만 적습니다."))
	form += tool.Baro_textarea("action", tool.Baro_t("예정 조치"), "", 3, tool.Baro_t("예: 문서 임시 격리 후 출처 보강 요청"), "")
	form += tool.Baro_submit(tool.Baro_t("사건 만들기"), "primary")

	body += tool.Baro_card(tool.Baro_t("사건 정보"), tool.Baro_form("/baro/case/new", form))

	body += tool.Baro_card(tool.Baro_t("사건을 만들 때 지켜야 할 것"),
		tool.Baro_list([]string{
			tool.Baro_t("공개 요약에는 개인정보(IP·이메일·실명·연락처)를 절대 쓰지 않습니다."),
			tool.Baro_t("적용 규정을 반드시 지정합니다. 규정 없이 내려진 조치는 이의제기에서 뒤집힙니다."),
			tool.Baro_t("사용자 제재와 문서 처리는 각각 별도의 조치로 기록합니다."),
			tool.Baro_t("문서 문제는 삭제가 아니라 임시 격리로 시작합니다."),
		}),
	)

	return baro_mod_shell(db, config, tool.Baro_t("사건 만들기"), "/baro/case", body)
}

// baro_mod_case_new_post : 사건 만들기 처리
func baro_mod_case_new_post(db *sql.DB, config tool.Config, values url.Values) string {
	kind := baro_mod_val(values, "kind")
	target_kind := baro_mod_val(values, "target_kind")
	target := baro_mod_val(values, "target")
	rule := baro_mod_val(values, "rule")
	priority := baro_mod_or(baro_mod_val(values, "priority"), "2")
	deadline := baro_mod_val(values, "deadline")
	public_summary := tool.Baro_trunc(baro_mod_raw(values, "public_summary"), 4000)
	private_note := tool.Baro_trunc(baro_mod_raw(values, "private_note"), 8000)
	action := tool.Baro_trunc(baro_mod_raw(values, "action"), 2000)
	report_id := baro_mod_val(values, "report")

	if target == "" {
		body := tool.Baro_alert("danger", tool.Baro_t("사건 대상을 적어 주세요."))
		body += tool.Baro_card("", tool.Baro_button("/baro/case/new", "돌아가기", "primary"))

		return baro_mod_shell(db, config, tool.Baro_t("사건 만들기"), "/baro/case", body)
	}

	id := tool.Baro_next_id(db, "baro_case")
	number := baro_mod_case_number_make(db)
	now := tool.Baro_now()

	if deadline == "" {
		deadline = tool.Baro_time_add(now, 14, 0, 0)
	}

	tool.Exec_DB(
		db,
		"insert into baro_case (id, number, kind, target_kind, target, rule, actor, status, action, public_summary, private_note, date, close_date, deadline, appeal_status, priority) values (?, ?, ?, ?, ?, ?, ?, 'open', ?, ?, ?, ?, '', ?, '', ?)",
		id, number, kind, target_kind, target, rule, config.IP, action, public_summary, private_note, now, deadline, priority,
	)

	tool.Exec_DB(
		db,
		"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'note', ?, ?, ?)",
		tool.Baro_next_id(db, "baro_case_action"), id, config.IP, "사건을 개설했습니다.", now, target,
	)

	// 신고에서 넘어왔다면 신고와 연결하고 원문을 비공개 증거로 보관
	if report, ok := baro_mod_report_get(db, report_id); ok {
		tool.Exec_DB(db, "update baro_report set case_id = ?, status = 'pending' where id = ?", id, report.ID)

		tool.Exec_DB(
			db,
			"insert into baro_case_evidence (id, case_id, kind, data, actor, date, is_private) values (?, ?, 'report', ?, ?, ?, 'on')",
			tool.Baro_next_id(db, "baro_case_evidence"), id,
			"접수번호 "+report.ID+"\n\n"+report.Detail+"\n\n[증거]\n"+report.Evidence,
			config.IP, now,
		)

		baro_mod_notify_user(db, report.Reporter, "case",
			tool.Baro_t("신고가 운영 사건 ")+number+tool.Baro_t(" 으로 전환되었습니다"),
			"/baro/case/view/"+tool.Url_parser(id), "")
	}

	// 대상이 사용자면 당사자에게도 알린다 (자기 사건은 알 권리가 있다)
	if target_kind == "user" {
		baro_mod_notify_user(db, target, "case",
			tool.Baro_t("회원님과 관련된 운영 사건 ")+number+tool.Baro_t(" 이(가) 개설되었습니다"),
			"/baro/case/view/"+tool.Url_parser(id), "")
	}

	tool.Baro_public_log(db, "case_open", config.IP, number, baro_mod_report_kind_name(kind), tool.Baro_trunc(tool.Baro_one_line(public_summary), 200))
	tool.Baro_audit(db, config, "case_open", number, "", target, baro_mod_report_kind_name(kind))
	tool.Baro_metric_bump(db, "case_open", tool.Baro_today(), 1)

	return tool.Get_redirect("/baro/case/view/" + tool.Url_parser(id))
}

// ------------------------------------------------------------
//  사건 상세
// ------------------------------------------------------------

// View_baro_mod_case_view : 사건 상세 (/baro/case/view/<id>)
func View_baro_mod_case_view(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query

	row, ok := baro_mod_case_get(db, arg)
	if !ok {
		return baro_mod_missing(db, config, "/baro/case", "사건", "/baro/case", tool.Baro_t("사건 목록으로"))
	}

	can_handle := tool.Baro_can(db, config.IP, "case")

	if values != nil {
		if !can_handle {
			return tool.Baro_deny(db, config, tool.Baro_t("사건을 수정할 권한이 없습니다."))
		}

		return baro_mod_case_view_post(db, config, row, values)
	}

	body := tool.Baro_crumb([][]string{{"/baro/case", tool.Baro_t("운영 사건")}, {"", row.Number}})

	body += tool.Baro_hero(
		row.Number,
		baro_mod_report_kind_name(row.Kind)+" · "+baro_mod_status_name(row.Status),
		"",
	)

	// 누구나 볼 수 있는 정보
	deadline_html := baro_mod_muted(row.Deadline)
	if row.Deadline != "" && row.Status != "closed" {
		if tool.Baro_is_past(row.Deadline) {
			deadline_html = tool.Baro_badge(tool.Baro_t("기한 초과"), "danger") + " " + baro_mod_esc(row.Deadline)
		} else {
			deadline_html = baro_mod_esc(row.Deadline) + " (" + baro_mod_esc(tool.Baro_duration_left(row.Deadline)) + ")"
		}
	}

	appeal_html := tool.Baro_t(`<span class="bw_muted">신청 없음</span>`)
	if row.AppealStatus != "" {
		appeal_html = baro_mod_status_badge(row.AppealStatus)
	}

	public_pairs := [][]string{
		{"사건 번호", baro_mod_esc(row.Number)},
		{tool.Baro_t("유형"), baro_mod_esc(baro_mod_report_kind_name(row.Kind))},
		{tool.Baro_t("대상"), baro_mod_target_kind_name(row.TargetKind) + " · " + baro_mod_target_link(db, config, row.TargetKind, row.Target, !can_handle)},
		{tool.Baro_t("적용 규정"), baro_mod_policy_link(db, row.Rule)},
		{"상태", baro_mod_status_badge(row.Status)},
		{tool.Baro_t("우선순위"), baro_mod_priority_badge(row.Priority)},
		{tool.Baro_t("개설"), baro_mod_esc(row.Date)},
		{tool.Baro_t("처리 기한"), deadline_html},
		{tool.Baro_t("처리 시각"), baro_mod_muted(row.CloseDate)},
		{tool.Baro_t("이의제기"), appeal_html},
	}

	if can_handle {
		public_pairs = append(public_pairs, []string{"담당자", tool.Baro_user_link(db, row.Actor, config.IP)})
	}

	body += tool.Baro_card(tool.Baro_t("사건 정보"), tool.Baro_kv_grid(public_pairs))

	summary_html := tool.Baro_t(`<span class="bw_muted">공개 요약이 아직 작성되지 않았습니다.</span>`)
	if strings.TrimSpace(row.PublicSummary) != "" {
		summary_html = tool.Baro_code(row.PublicSummary)
	}
	body += tool.Baro_card(tool.Baro_t("공개 요약"), summary_html)

	if strings.TrimSpace(row.Action) != "" {
		body += tool.Baro_card(tool.Baro_t("조치"), tool.Baro_code(row.Action))
	}

	// 조치 이력 (시간순)
	body += tool.Baro_card(tool.Baro_t("조치 이력"), baro_mod_case_action_table(db, config, row, can_handle))

	// 연결된 제재 (사람에 대한 조치는 공개 로그에도 남으므로 여기서도 요약을 보여준다)
	body += tool.Baro_card(tool.Baro_t("이 사건으로 내려진 제재"), baro_mod_case_sanction_table(db, config, row, can_handle))

	// 담당자 전용 영역
	if can_handle {
		body += baro_mod_case_private_block(db, config, row)
	} else {
		body += tool.Baro_note(tool.Baro_t("비공개 정보와 증거 원문은 담당자만 볼 수 있습니다. 공개되는 것은 요약과 적용 규정, 조치 결과입니다."))
	}

	// 이의제기
	appeal_body := ""
	if row.Status == "closed" || row.AppealStatus != "" {
		appeal_body += tool.Baro_note(tool.Baro_t("이 사건의 조치에 동의할 수 없다면 이의제기를 신청할 수 있습니다. 원조치 담당자는 심사에서 빠집니다."))
	}
	appeal_body += tool.Baro_button("/baro/appeal/new/"+tool.Url_parser(row.ID), "이 사건에 이의제기", "")
	if tool.Baro_can(db, config.IP, "appeal") {
		appeal_body += " " + tool.Baro_button("/baro/appeal?case="+tool.Url_parser(row.ID), "이 사건의 이의제기 보기", "")
	}
	body += tool.Baro_card(tool.Baro_t("이의제기"), appeal_body)

	if can_handle {
		next_links := tool.Baro_button("/baro/case/act/"+tool.Url_parser(row.ID), "조치 기록 / 종결", "primary")
		if tool.Baro_can(db, config.IP, "sanction") {
			next_links += " " + tool.Baro_button("/baro/case/sanction?case="+tool.Url_parser(row.ID)+"&user="+tool.Url_parser(row.Target), "제재 부과", "")
		}
		if row.TargetKind == "doc" || row.TargetKind == "file" {
			next_links += " " + tool.Baro_button("/baro/quarantine/new?title="+tool.Url_parser(row.Target)+"&case="+tool.Url_parser(row.ID), "문서 임시 격리", "")
		}
		if row.TargetKind == "user" {
			next_links += " " + tool.Baro_button("/baro/quarantine/user/"+tool.Url_parser(row.Target), "이 사용자의 문서 다루기", "")
		}

		body += tool.Baro_card(tool.Baro_t("다음 단계"), next_links)
	}

	body += baro_mod_principle_note()

	return baro_mod_shell(db, config, "사건 "+row.Number, "/baro/case", body)
}

// baro_mod_case_action_table : 조치 이력 표 (시간순)
func baro_mod_case_action_table(db *sql.DB, config tool.Config, row baro_mod_case_row, can_handle bool) string {
	rows := tool.Query_DB(
		db,
		"select actor, action, detail, date, target from baro_case_action where case_id = ? order by id + 0 asc",
		row.ID,
	)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		actor, action, detail, date, target := "", "", "", "", ""
		if err := rows.Scan(&actor, &action, &detail, &date, &target); err != nil {
			continue
		}

		actor_html := baro_mod_esc(tool.Baro_mask_ip(actor))
		if can_handle {
			actor_html = tool.Baro_user_link(db, actor, config.IP)
		}

		table_rows = append(table_rows, []string{
			baro_mod_esc(date),
			baro_mod_esc(baro_mod_case_action_kind_name(action)),
			baro_mod_esc(tool.Baro_trunc(detail, 400)),
			baro_mod_muted(target),
			actor_html,
		})
	}

	return tool.Baro_table(
		[]string{"시각", tool.Baro_t("조치"), tool.Baro_t("내용"), tool.Baro_t("대상"), tool.Baro_t("집행자")},
		table_rows,
		tool.Baro_t("아직 기록된 조치가 없습니다."),
	)
}

// baro_mod_case_sanction_table : 사건에 연결된 제재
func baro_mod_case_sanction_table(db *sql.DB, config tool.Config, row baro_mod_case_row, can_handle bool) string {
	rows := tool.Query_DB(
		db,
		"select id, user, kind, scope, reason, status, start_date, end_date from baro_sanction where case_id = ? order by id + 0 desc",
		row.ID,
	)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		id, user, kind, scope, reason, status, start_date, end_date := "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &user, &kind, &scope, &reason, &status, &start_date, &end_date); err != nil {
			continue
		}

		user_html := baro_mod_esc(tool.Baro_mask_ip(user))
		if can_handle {
			user_html = tool.Baro_user_link(db, user, config.IP)
		}

		table_rows = append(table_rows, []string{
			user_html,
			baro_mod_esc(tool.Baro_sanction_name(kind)),
			baro_mod_muted(scope),
			baro_mod_esc(tool.Baro_trunc(reason, 200)),
			baro_mod_status_badge(status),
			baro_mod_esc(start_date),
			baro_mod_esc(baro_mod_or(end_date, "무기한")),
		})
	}

	table := tool.Baro_table(
		[]string{tool.Baro_t("대상"), "종류", tool.Baro_t("범위"), "사유", "상태", tool.Baro_t("시작"), tool.Baro_t("종료")},
		table_rows,
		tool.Baro_t("이 사건으로 내려진 제재가 없습니다."),
	)

	return table + tool.Baro_note(tool.Baro_t("제재는 사람에게만 적용됩니다. 제재를 이유로 문서를 지우지 않습니다. (고정 원칙 1)"))
}

// baro_mod_case_private_block : 담당자 전용 영역 (비공개 정보 · 증거 · 연결된 신고)
func baro_mod_case_private_block(db *sql.DB, config tool.Config, row baro_mod_case_row) string {
	body := ""

	private_html := tool.Baro_t(`<span class="bw_muted">기록된 비공개 정보가 없습니다.</span>`)
	if strings.TrimSpace(row.PrivateNote) != "" {
		private_html = tool.Baro_code(row.PrivateNote)
	}
	body += tool.Baro_card(tool.Baro_t("비공개 정보 (담당자 전용)"), tool.Baro_alert("warn", tool.Baro_t("이 내용은 공개 화면에 절대 노출되지 않습니다."))+private_html)

	// 증거
	evidence_rows := tool.Query_DB(
		db,
		"select id, kind, data, actor, date, is_private from baro_case_evidence where case_id = ? order by id + 0 asc",
		row.ID,
	)
	defer evidence_rows.Close()

	evidence_html := ""
	evidence_count := 0
	for evidence_rows.Next() {
		id, kind, data, actor, date, is_private := "", "", "", "", "", ""
		if err := evidence_rows.Scan(&id, &kind, &data, &actor, &date, &is_private); err != nil {
			continue
		}

		evidence_count++
		mark := tool.Baro_badge("공개", "ok")
		if is_private == "on" {
			mark = tool.Baro_badge(tool.Baro_t("비공개"), "danger")
		}

		evidence_html += tool.Baro_section(
			tool.Baro_t("증거 ")+id+" · "+kind,
			mark+" "+tool.Baro_time_cell(date)+" "+tool.Baro_user_link(db, actor, config.IP)+tool.Baro_code(tool.Baro_trunc(data, 4000)),
		)
	}

	if evidence_count == 0 {
		evidence_html = tool.Baro_empty(tool.Baro_t("등록된 증거가 없습니다."))
	}

	evidence_form := tool.Baro_hidden("act", "evidence")
	evidence_form += tool.Baro_input("evidence_kind", tool.Baro_t("증거 종류"), "link", "link / text / file / report", "")
	evidence_form += tool.Baro_textarea("evidence_data", tool.Baro_t("증거 내용"), "", 5, tool.Baro_t("URL 이나 원문"), "")
	evidence_form += tool.Baro_checkbox("evidence_private", tool.Baro_t("비공개 증거로 보관"), "on", true, tool.Baro_t("개인정보가 들어 있으면 반드시 켜 두세요."))
	evidence_form += tool.Baro_submit(tool.Baro_t("증거 추가"), "")

	body += tool.Baro_card("증거 (담당자 전용)", evidence_html+tool.Baro_form("/baro/case/view/"+tool.Url_parser(row.ID), evidence_form))

	// 연결된 신고
	report_rows := tool.Query_DB(
		db,
		"select id, kind, reporter, status, date from baro_report where case_id = ? order by id + 0 asc",
		row.ID,
	)
	defer report_rows.Close()

	report_table := [][]string{}
	for report_rows.Next() {
		id, kind, reporter, status, date := "", "", "", "", ""
		if err := report_rows.Scan(&id, &kind, &reporter, &status, &date); err != nil {
			continue
		}

		report_table = append(report_table, []string{
			`<a href="/baro/report/view/` + tool.Url_parser(id) + `">` + baro_mod_esc(id) + `</a>`,
			baro_mod_esc(baro_mod_report_kind_name(kind)),
			baro_mod_esc(tool.Baro_mask_ip(reporter)),
			baro_mod_status_badge(status),
			tool.Baro_time_cell(date),
		})
	}

	body += tool.Baro_card(tool.Baro_t("연결된 신고 (담당자 전용)"), tool.Baro_table(
		[]string{"접수번호", "종류", "신고자", "상태", "접수"},
		report_table,
		tool.Baro_t("연결된 신고가 없습니다."),
	))

	// 이해충돌 기록
	coi := baro_mod_coi_list(db, row.ID)
	if len(coi) > 0 {
		coi_table := [][]string{}
		for _, item := range coi {
			coi_table = append(coi_table, []string{
				tool.Baro_user_link(db, item[0], config.IP),
				baro_mod_esc(item[1]),
				baro_mod_esc(item[2]),
			})
		}

		body += tool.Baro_card(tool.Baro_t("이해충돌 기록"), tool.Baro_table([]string{tool.Baro_t("대상"), "사유", "시각"}, coi_table, ""))
	}

	// 사건 정보 수정
	edit_form := tool.Baro_hidden("act", "update")
	edit_form += tool.Baro_select("rule", tool.Baro_t("적용 규정"), baro_mod_policy_options(db), row.Rule, "")
	edit_form += tool.Baro_select("priority", tool.Baro_t("우선순위"), baro_mod_priority_list(), baro_mod_or(row.Priority, "2"), "")
	edit_form += tool.Baro_input_type("deadline", tool.Baro_t("처리 기한"), tool.Baro_date_only(row.Deadline), "date", "")
	edit_form += tool.Baro_textarea("public_summary", tool.Baro_t("공개 요약"), row.PublicSummary, 5, "", tool.Baro_t("개인정보를 쓰지 마세요."))
	edit_form += tool.Baro_textarea("private_note", tool.Baro_t("비공개 정보"), row.PrivateNote, 5, "", tool.Baro_t("공개되지 않습니다."))
	edit_form += tool.Baro_textarea("action", tool.Baro_t("조치"), row.Action, 3, "", "")
	edit_form += tool.Baro_submit(tool.Baro_t("사건 정보 저장"), "primary")

	body += tool.Baro_card("사건 정보 수정 (담당자 전용)", tool.Baro_form("/baro/case/view/"+tool.Url_parser(row.ID), edit_form))

	return body
}

// baro_mod_case_view_post : 사건 상세에서의 처리
func baro_mod_case_view_post(db *sql.DB, config tool.Config, row baro_mod_case_row, values url.Values) string {
	target_url := "/baro/case/view/" + tool.Url_parser(row.ID)
	act := baro_mod_val(values, "act")

	switch act {
	case "evidence":
		data := tool.Baro_trunc(baro_mod_raw(values, "evidence_data"), 8000)
		if strings.TrimSpace(data) == "" {
			return tool.Get_redirect(target_url)
		}

		is_private := ""
		if baro_mod_on(values, "evidence_private") {
			is_private = "on"
		}

		tool.Exec_DB(
			db,
			"insert into baro_case_evidence (id, case_id, kind, data, actor, date, is_private) values (?, ?, ?, ?, ?, ?, ?)",
			tool.Baro_next_id(db, "baro_case_evidence"), row.ID,
			baro_mod_or(baro_mod_val(values, "evidence_kind"), "text"), data, config.IP, tool.Baro_now(), is_private,
		)

		tool.Exec_DB(
			db,
			"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'evidence', ?, ?, ?)",
			tool.Baro_next_id(db, "baro_case_action"), row.ID, config.IP, "증거를 등록했습니다.", tool.Baro_now(), row.Target,
		)

		tool.Baro_audit(db, config, "case_evidence", row.Number, "", tool.Baro_trunc(data, 300), "증거 등록")

		return tool.Get_redirect(target_url)

	case "update":
		rule := baro_mod_val(values, "rule")
		priority := baro_mod_or(baro_mod_val(values, "priority"), "2")
		deadline := baro_mod_val(values, "deadline")
		public_summary := tool.Baro_trunc(baro_mod_raw(values, "public_summary"), 4000)
		private_note := tool.Baro_trunc(baro_mod_raw(values, "private_note"), 8000)
		action := tool.Baro_trunc(baro_mod_raw(values, "action"), 2000)

		tool.Exec_DB(
			db,
			"update baro_case set rule = ?, priority = ?, deadline = ?, public_summary = ?, private_note = ?, action = ? where id = ?",
			rule, priority, baro_mod_or(deadline, row.Deadline), public_summary, private_note, action, row.ID,
		)

		tool.Exec_DB(
			db,
			"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'note', ?, ?, ?)",
			tool.Baro_next_id(db, "baro_case_action"), row.ID, config.IP, "사건 정보를 수정했습니다.", tool.Baro_now(), row.Target,
		)

		baro_mod_log_both(db, config, "case_update", row.Number, tool.Baro_t("사건 정보 수정"), tool.Baro_trunc(tool.Baro_one_line(public_summary), 200), row.PublicSummary, public_summary)

		return tool.Get_redirect(target_url)
	}

	return tool.Get_redirect(target_url)
}

// ------------------------------------------------------------
//  조치 기록 / 종결
// ------------------------------------------------------------

// View_baro_mod_case_act : 조치 기록 추가 · 사건 종결 (/baro/case/act/<id>)
func View_baro_mod_case_act(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query

	if !tool.Baro_can(db, config.IP, "case") {
		return tool.Baro_deny(db, config, tool.Baro_t("조치를 기록하려면 사건 처리 권한이 필요합니다."))
	}

	row, ok := baro_mod_case_get(db, arg)
	if !ok {
		return baro_mod_missing(db, config, "/baro/case", "사건", "/baro/case", tool.Baro_t("사건 목록으로"))
	}

	if values != nil {
		return baro_mod_case_act_post(db, config, row, values)
	}

	body := tool.Baro_crumb([][]string{
		{"/baro/case", tool.Baro_t("운영 사건")},
		{"/baro/case/view/" + tool.Url_parser(row.ID), row.Number},
		{"", "조치 기록"},
	})

	body += tool.Baro_card(tool.Baro_t("사건 요약"), tool.Baro_kv_grid([][]string{
		{"사건 번호", baro_mod_esc(row.Number)},
		{tool.Baro_t("유형"), baro_mod_esc(baro_mod_report_kind_name(row.Kind))},
		{tool.Baro_t("대상"), baro_mod_target_link(db, config, row.TargetKind, row.Target, false)},
		{"상태", baro_mod_status_badge(row.Status)},
	}))

	form := tool.Baro_select("action", tool.Baro_t("조치 종류"), baro_mod_case_action_kind_list(), "note", tool.Baro_t("무엇을 했는지 고릅니다."))
	form += tool.Baro_input("target", tool.Baro_t("조치 대상"), row.Target, tool.Baro_t("문서 이름 · 사용자 이름"), tool.Baro_t("사건 대상과 다르면 바꿔 적습니다."))
	form += tool.Baro_textarea("detail", tool.Baro_t("조치 내용"), "", 6, tool.Baro_t("무엇을 왜 했는지 적습니다."), tool.Baro_t("필수 항목입니다. 이 기록은 이의제기 심사의 근거가 됩니다."))
	form += tool.Baro_textarea("public_summary", tool.Baro_t("공개 요약 갱신"), row.PublicSummary, 4, "", tool.Baro_t("비워 두면 기존 요약을 유지합니다. 개인정보를 쓰지 마세요."))
	form += tool.Baro_checkbox("close", tool.Baro_t("이 조치로 사건을 종결한다"), "on", false, tool.Baro_t("종결하면 처리 시각이 기록되고 이의제기 기간이 시작됩니다."))
	form += tool.Baro_checkbox("notify", "대상 사용자에게 알린다", "on", row.TargetKind == "user", "대상이 로그인 사용자일 때만 발송됩니다.")
	form += tool.Baro_submit("조치 기록", "primary")

	body += tool.Baro_card("조치 기록", tool.Baro_form("/baro/case/act/"+tool.Url_parser(row.ID), form))

	body += tool.Baro_card(tool.Baro_t("조치 이력"), baro_mod_case_action_table(db, config, row, true))

	body += tool.Baro_alert("info",
		"이 화면에서는 문서를 삭제할 수 없습니다. 문서 문제는 <a href=\"/baro/quarantine/new?title="+
			tool.Url_parser(row.Target)+"&case="+tool.Url_parser(row.ID)+"\">임시 격리</a>로 처리합니다. (고정 원칙 2)")

	return baro_mod_shell(db, config, "조치 기록", "/baro/case", body)
}

// baro_mod_case_act_post : 조치 기록 처리
func baro_mod_case_act_post(db *sql.DB, config tool.Config, row baro_mod_case_row, values url.Values) string {
	target_url := "/baro/case/view/" + tool.Url_parser(row.ID)

	action := baro_mod_or(baro_mod_val(values, "action"), "note")
	detail := tool.Baro_trunc(baro_mod_raw(values, "detail"), 8000)
	target := baro_mod_or(baro_mod_val(values, "target"), row.Target)
	public_summary := tool.Baro_trunc(baro_mod_raw(values, "public_summary"), 4000)
	close_it := baro_mod_on(values, "close")
	notify := baro_mod_on(values, "notify")

	if strings.TrimSpace(detail) == "" {
		body := tool.Baro_alert("danger", tool.Baro_t("조치 내용을 반드시 적어 주세요. 근거 없는 조치는 기록할 수 없습니다."))
		body += tool.Baro_card("", tool.Baro_button("/baro/case/act/"+tool.Url_parser(row.ID), "돌아가기", "primary"))

		return baro_mod_shell(db, config, "조치 기록", "/baro/case", body)
	}

	now := tool.Baro_now()

	tool.Exec_DB(
		db,
		"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_case_action"), row.ID, config.IP, action, detail, now, target,
	)

	new_status := row.Status
	close_date := row.CloseDate

	if close_it {
		new_status = "closed"
		close_date = now
	} else if row.Status == "open" {
		new_status = "pending"
	}

	if action == "reject" {
		new_status = "rejected"
		close_date = now
	}

	if strings.TrimSpace(public_summary) == "" {
		public_summary = row.PublicSummary
	}

	tool.Exec_DB(
		db,
		"update baro_case set status = ?, close_date = ?, public_summary = ?, action = ? where id = ?",
		new_status, close_date, public_summary, baro_mod_case_action_kind_name(action), row.ID,
	)

	// 사건이 종결되면 연결된 신고도 종결하고 신고자에게 알린다
	if new_status == "closed" || new_status == "rejected" {
		report_rows := tool.Query_DB(db, "select id, reporter from baro_report where case_id = ?", row.ID)

		reporter_list := [][]string{}
		for report_rows.Next() {
			id, reporter := "", ""
			if err := report_rows.Scan(&id, &reporter); err != nil {
				continue
			}

			reporter_list = append(reporter_list, []string{id, reporter})
		}
		report_rows.Close()

		for _, item := range reporter_list {
			tool.Exec_DB(db, "update baro_report set status = 'closed' where id = ?", item[0])
			baro_mod_notify_user(db, item[1], "case",
				tool.Baro_t("신고한 사건 ")+row.Number+tool.Baro_t(" 의 처리가 끝났습니다"),
				target_url, "")
		}
	}

	if notify && row.TargetKind == "user" {
		baro_mod_notify_user(db, target, "case",
			"사건 "+row.Number+tool.Baro_t(" 에 조치가 기록되었습니다: ")+baro_mod_case_action_kind_name(action),
			target_url, "")
	}

	baro_mod_log_both(db, config, "case_action", row.Number,
		baro_mod_case_action_kind_name(action),
		tool.Baro_trunc(tool.Baro_one_line(public_summary), 200),
		row.Status, new_status)

	tool.Baro_metric_bump(db, "case_action", tool.Baro_today(), 1)

	return tool.Get_redirect(target_url)
}
