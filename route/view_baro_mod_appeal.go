package route

// ============================================================
//  10-5 이의제기 (/baro/appeal)
//
//  사건 페이지에서 바로 신청할 수 있어야 한다.
//  원조치 담당자와 다른 검토자가 배정된다.
//  원조치 담당자가 이의제기를 열면 "이해충돌로 처리할 수 없습니다" 를 보여 주고
//  처리 버튼을 감춘다. 이 사실은 baro_coi 에 남는다.
// ============================================================

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// baro_mod_appeal_deadline_days : 이의제기 처리 기한 (기본 14일)
func baro_mod_appeal_deadline_days(db *sql.DB) int {
	days := tool.Baro_get_int(db, "appeal_deadline_days", 14)
	if days < 1 {
		days = 14
	}

	return days
}

// baro_mod_appeal_result_list : 이의제기 결과
func baro_mod_appeal_result_list() [][]string {
	return [][]string{
		{"accept", tool.Baro_t("인용 (조치를 취소한다)")},
		{"partial", tool.Baro_t("일부 인용 (조치를 완화한다)")},
		{"reject", tool.Baro_t("기각 (원조치를 유지한다)")},
	}
}

// baro_mod_appeal_result_name : 결과 이름
func baro_mod_appeal_result_name(result string) string {
	switch result {
	case "accept":
		return "인용"
	case "partial":
		return tool.Baro_t("일부 인용")
	case "reject":
		return "기각"
	}

	return "미결"
}

// baro_mod_appeal_result_badge : 결과 배지
func baro_mod_appeal_result_badge(result string) string {
	switch result {
	case "accept":
		return tool.Baro_badge("인용", "ok")
	case "partial":
		return tool.Baro_badge(tool.Baro_t("일부 인용"), "warn")
	case "reject":
		return tool.Baro_badge("기각", "danger")
	}

	return tool.Baro_badge(tool.Baro_t("심사 중"), "info")
}

// baro_mod_appeal_pick_reviewer : 원조치 담당자가 아닌 검토자를 고른다
//
//	'appeal' 역할 보유자 중에서 이의제기 신청자·원조치 담당자를 뺀 사람을 고른다.
//	담당 건수가 가장 적은 사람에게 배정한다. 마땅한 사람이 없으면 "" (미배정).
func baro_mod_appeal_pick_reviewer(db *sql.DB, case_actor string, appellant string) string {
	rows := tool.Query_DB(db, "select user from baro_role where role = 'appeal'")
	defer rows.Close()

	candidates := []string{}
	seen := map[string]bool{}

	for rows.Next() {
		user := ""
		if err := rows.Scan(&user); err != nil {
			continue
		}

		if user == "" || seen[user] || user == case_actor || user == appellant {
			continue
		}

		seen[user] = true
		candidates = append(candidates, user)
	}

	best := ""
	best_load := -1

	for _, user := range candidates {
		load := 0
		tool.QueryRow_DB(
			db,
			"select count(*) from baro_appeal where reviewer = ? and status"+baro_mod_status_open_sql,
			[]any{&load}, user,
		)

		if best_load < 0 || load < best_load {
			best = user
			best_load = load
		}
	}

	return best
}

// ------------------------------------------------------------
//  이의제기 신청
// ------------------------------------------------------------

// View_baro_mod_appeal_new : 이의제기 신청 (/baro/appeal/new/<사건id>)
func View_baro_mod_appeal_new(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	case_key := strings.TrimSpace(arg)
	if case_key == "" {
		case_key = baro_mod_val(query, "case")
	}

	// 이의제기는 사람 단위 기록이므로 로그인이 필요하다
	if page, stop := tool.Baro_login_wall(db, config, "/baro/appeal/new/"+tool.Url_parser(case_key)); stop {
		return page
	}

	if page, stop := baro_mod_read_only_block(db, config, "/baro/appeal"); stop {
		return page
	}

	case_row, ok := baro_mod_case_get(db, case_key)
	if !ok {
		body := tool.Baro_alert("warn", tool.Baro_t("이의제기를 신청할 사건을 찾을 수 없습니다. 사건 번호를 확인해 주세요."))

		form := `<form class="bw_form" method="get" action="/baro/appeal/new">`
		form += tool.Baro_input("case", "사건 번호", case_key, "BW-2026-0001", tool.Baro_t("사건 목록에서 번호를 확인할 수 있습니다."))
		form += tool.Baro_submit(tool.Baro_t("사건 찾기"), "primary")
		form += `</form>`

		body += tool.Baro_card(tool.Baro_t("사건 찾기"), form)
		body += tool.Baro_card("", tool.Baro_button("/baro/case", tool.Baro_t("사건 목록으로"), ""))

		return baro_mod_shell(db, config, "이의제기 신청", "/baro/appeal", body)
	}

	if values != nil {
		return baro_mod_appeal_new_post(db, config, case_row, values)
	}

	days := baro_mod_appeal_deadline_days(db)

	body := tool.Baro_crumb([][]string{
		{"/baro/case", tool.Baro_t("운영 사건")},
		{"/baro/case/view/" + tool.Url_parser(case_row.ID), case_row.Number},
		{"", tool.Baro_t("이의제기")},
	})

	body += tool.Baro_alert("info",
		tool.Baro_t("이의제기는 <b>원조치 담당자가 아닌 다른 검토자</b>가 심사합니다. ")+
			tool.Baro_t("처리 기한은 신청일로부터 ")+tool.Baro_itoa(days)+tool.Baro_t("일입니다."))

	body += tool.Baro_card(tool.Baro_t("대상 사건"), tool.Baro_kv_grid([][]string{
		{"사건 번호", `<a href="/baro/case/view/` + tool.Url_parser(case_row.ID) + `">` + baro_mod_esc(case_row.Number) + `</a>`},
		{tool.Baro_t("유형"), baro_mod_esc(baro_mod_report_kind_name(case_row.Kind))},
		{tool.Baro_t("적용 규정"), baro_mod_policy_link(db, case_row.Rule)},
		{tool.Baro_t("조치"), baro_mod_muted(case_row.Action)},
		{"상태", baro_mod_status_badge(case_row.Status)},
		{tool.Baro_t("처리 시각"), baro_mod_muted(case_row.CloseDate)},
	}))

	if strings.TrimSpace(case_row.PublicSummary) != "" {
		body += tool.Baro_card(tool.Baro_t("사건 공개 요약"), tool.Baro_code(case_row.PublicSummary))
	}

	// 이미 신청한 이의제기가 있으면 보여 준다
	old_id, old_status := "", ""
	tool.QueryRow_DB(
		db,
		"select id, status from baro_appeal where case_id = ? and user = ? order by id + 0 desc limit 1",
		[]any{&old_id, &old_status}, case_row.ID, config.IP,
	)

	if old_id != "" {
		body += tool.Baro_alert("warn",
			tool.Baro_t("이 사건에 이미 이의제기를 신청했습니다. (번호 ")+baro_mod_esc(old_id)+" · "+baro_mod_status_name(old_status)+") "+
				`<a href="/baro/appeal/view/`+tool.Url_parser(old_id)+tool.Baro_t(`">신청 내용 보기</a>`))

		if old_status == "open" || old_status == "pending" {
			body += tool.Baro_card("", tool.Baro_button("/baro/appeal/view/"+tool.Url_parser(old_id), "심사 중인 이의제기 보기", "primary"))

			return baro_mod_shell(db, config, "이의제기 신청", "/baro/appeal", body)
		}

		body += tool.Baro_note(tool.Baro_t("이전 이의제기가 끝났다면 새로운 근거를 들어 재심을 신청할 수 있습니다."))
	}

	form := tool.Baro_textarea("reason", tool.Baro_t("이의 사유"), "", 8, tool.Baro_t("왜 조치가 부당하다고 생각하는지 적어 주세요."), tool.Baro_t("필수 항목입니다. 어떤 규정 적용이 잘못됐는지 짚어 주면 좋습니다."))
	form += tool.Baro_textarea("extra", tool.Baro_t("추가 자료"), "", 5, tool.Baro_t("근거가 되는 링크·판 번호·기록"), tool.Baro_t("심사에 참고할 자료를 적습니다."))
	form += tool.Baro_submit("이의제기 신청", "primary")

	body += tool.Baro_card("이의제기 신청", tool.Baro_form("/baro/appeal/new/"+tool.Url_parser(case_row.ID), form))

	body += tool.Baro_card(tool.Baro_t("이의제기는 이렇게 처리됩니다"),
		tool.Baro_list([]string{
			tool.Baro_t("신청하면 <b>원조치 담당자가 아닌</b> 검토자가 자동으로 배정됩니다."),
			tool.Baro_t("원조치 담당자가 이 이의제기를 열면 이해충돌로 처리할 수 없다는 안내가 뜨고 처리 버튼이 사라집니다."),
			tool.Baro_t("처리 기한은 ") + tool.Baro_itoa(days) + tool.Baro_t("일입니다. 기한이 지나면 화면에 초과로 표시됩니다."),
			tool.Baro_t("결과와 근거는 공개됩니다. 기각되어도 새로운 근거가 있으면 재심을 신청할 수 있습니다."),
		}),
	)

	return baro_mod_shell(db, config, "이의제기 신청", "/baro/appeal", body)
}

// baro_mod_appeal_new_post : 이의제기 신청 처리
func baro_mod_appeal_new_post(db *sql.DB, config tool.Config, case_row baro_mod_case_row, values url.Values) string {
	reason := tool.Baro_trunc(baro_mod_raw(values, "reason"), 8000)
	extra := tool.Baro_trunc(baro_mod_raw(values, "extra"), 8000)

	if strings.TrimSpace(reason) == "" {
		body := tool.Baro_alert("danger", tool.Baro_t("이의 사유를 적어 주세요."))
		body += tool.Baro_card("", tool.Baro_button("/baro/appeal/new/"+tool.Url_parser(case_row.ID), "돌아가기", "primary"))

		return baro_mod_shell(db, config, "이의제기 신청", "/baro/appeal", body)
	}

	if !tool.Baro_rate_ok(db, "baro_appeal_new", config.IP, 3, 3600) {
		body := tool.Baro_alert("danger", tool.Baro_t("짧은 시간에 이의제기를 너무 많이 신청했습니다. 잠시 뒤에 다시 시도해 주세요."))
		body += tool.Baro_card("", tool.Baro_button("/baro/appeal/mine", tool.Baro_t("내 이의제기 보기"), "primary"))

		return baro_mod_shell(db, config, "이의제기 신청", "/baro/appeal", body)
	}

	id := tool.Baro_next_id(db, "baro_appeal")
	now := tool.Baro_now()
	deadline := tool.Baro_time_add(now, baro_mod_appeal_deadline_days(db), 0, 0)
	reviewer := baro_mod_appeal_pick_reviewer(db, case_row.Actor, config.IP)

	tool.Exec_DB(
		db,
		"insert into baro_appeal (id, case_id, user, reason, extra, status, reviewer, result, result_reason, date, deadline, close_date) values (?, ?, ?, ?, ?, 'open', ?, '', '', ?, ?, '')",
		id, case_row.ID, config.IP, reason, extra, reviewer, now, deadline,
	)

	tool.Exec_DB(db, "update baro_case set appeal_status = 'open' where id = ?", case_row.ID)

	tool.Exec_DB(
		db,
		"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'note', ?, ?, ?)",
		tool.Baro_next_id(db, "baro_case_action"), case_row.ID, config.IP,
		"이의제기가 접수되었습니다. (이의번호 "+id+")", now, config.IP,
	)

	// 원조치 담당자는 이 건을 처리할 수 없다 — 미리 이해충돌로 기록해 둔다
	if case_row.Actor != "" {
		baro_mod_coi_mark(db, case_row.ID, case_row.Actor, tool.Baro_t("원조치 담당자는 이의제기 심사에서 배제됩니다."))
	}

	baro_mod_notify_user(db, config.IP, "case",
		tool.Baro_t("이의제기가 접수되었습니다 (이의번호 ")+id+")",
		"/baro/appeal/view/"+tool.Url_parser(id), case_row.Number)

	if reviewer != "" {
		baro_mod_notify_user(db, reviewer, "case",
			tool.Baro_t("이의제기 심사가 배정되었습니다 (사건 ")+case_row.Number+")",
			"/baro/appeal/view/"+tool.Url_parser(id), "")
	}

	tool.Baro_public_log(db, "appeal_new", config.IP, case_row.Number, "이의제기 접수", "이의번호 "+id)
	tool.Baro_audit(db, config, "appeal_new", case_row.Number, "", tool.Baro_trunc(reason, 500), "이의제기 접수")
	tool.Baro_metric_bump(db, "appeal_new", tool.Baro_today(), 1)

	return tool.Get_redirect("/baro/appeal/view/" + tool.Url_parser(id))
}

// ------------------------------------------------------------
//  이의제기 목록
// ------------------------------------------------------------

// View_baro_mod_appeal_list : 이의제기 목록 (/baro/appeal)
func View_baro_mod_appeal_list(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	if !tool.Baro_can(db, config.IP, "appeal") {
		return tool.Baro_deny(db, config,
			tool.Baro_t("이의제기 목록은 '이의제기 담당' 역할이 있어야 볼 수 있습니다. ")+
				tool.Baro_t("본인이 신청한 이의제기는 /baro/appeal/mine 에서 볼 수 있습니다."))
	}

	page := baro_mod_page_num(arg)
	size := tool.Baro_page_size

	status := baro_mod_val(query, "status")
	case_key := baro_mod_val(query, "case")

	where := " where 1 = 1"
	params := []any{}

	switch status {
	case "":
		where += " and status" + baro_mod_status_open_sql
	case "all":
	default:
		where += " and status = ?"
		params = append(params, status)
	}

	if case_key != "" {
		if case_row, ok := baro_mod_case_get(db, case_key); ok {
			where += " and case_id = ?"
			params = append(params, case_row.ID)
		}
	}

	params = append(params, (page-1)*size, size)

	rows := tool.Query_DB(
		db,
		"select id, case_id, user, status, reviewer, result, date, deadline from baro_appeal"+where+" order by id + 0 desc limit ?, ?",
		params...,
	)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0
	overdue := 0

	for rows.Next() {
		id, case_id, user, a_status, reviewer, result, date, deadline := "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &case_id, &user, &a_status, &reviewer, &result, &date, &deadline); err != nil {
			continue
		}

		count++

		deadline_html := baro_mod_muted(deadline)
		if deadline != "" && a_status != "closed" {
			if tool.Baro_is_past(deadline) {
				deadline_html = tool.Baro_badge(tool.Baro_t("기한 초과"), "danger")
				overdue++
			} else {
				deadline_html = baro_mod_esc(tool.Baro_duration_left(deadline))
			}
		}

		reviewer_html := tool.Baro_t(`<span class="bw_muted">미배정</span>`)
		if reviewer != "" {
			reviewer_html = tool.Baro_user_link(db, reviewer, config.IP)
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/appeal/view/` + tool.Url_parser(id) + `">` + baro_mod_esc(id) + `</a>`,
			baro_mod_case_link(db, case_id),
			tool.Baro_user_link(db, user, config.IP),
			reviewer_html,
			baro_mod_status_badge(a_status),
			baro_mod_appeal_result_badge(result),
			deadline_html,
			tool.Baro_time_cell(date),
		})
	}

	body := tool.Baro_crumb([][]string{{"/baro/case", tool.Baro_t("운영 사건")}, {"", tool.Baro_t("이의제기")}})

	open_count, closed_count := 0, 0
	tool.QueryRow_DB(db, "select count(*) from baro_appeal where status"+baro_mod_status_open_sql, []any{&open_count})
	tool.QueryRow_DB(db, "select count(*) from baro_appeal where status = 'closed'", []any{&closed_count})

	accept_count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_appeal where result in ('accept', 'partial')", []any{&accept_count})

	body += tool.Baro_card(tool.Baro_t("이의제기 현황"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("심사 중"), tool.Baro_comma(open_count), ""},
		{"처리 완료", tool.Baro_comma(closed_count), ""},
		{tool.Baro_t("인용 · 일부 인용"), tool.Baro_comma(accept_count), tool.Baro_t("조치가 뒤집힌 건수")},
		{tool.Baro_t("이 쪽 기한 초과"), tool.Baro_comma(overdue), tool.Baro_t("지금 화면 기준")},
	}))

	filter_form := `<form class="bw_form" method="get" action="/baro/appeal">`
	filter_form += tool.Baro_select("status", "상태", [][]string{
		{"", tool.Baro_t("심사 중")},
		{"closed", "처리 완료"},
		{"rejected", tool.Baro_t("반려")},
		{"all", tool.Baro_t("전체")},
	}, status, "")
	filter_form += tool.Baro_input("case", "사건", case_key, "BW-2026-0001", "")
	filter_form += tool.Baro_submit(tool.Baro_t("적용"), "")
	filter_form += `</form>`

	body += tool.Baro_card("걸러 보기", filter_form)

	list_body := tool.Baro_table(
		[]string{"이의번호", "사건", tool.Baro_t("신청자"), tool.Baro_t("검토자"), "상태", tool.Baro_t("결과"), "기한", tool.Baro_t("신청")},
		table_rows,
		"",
	)
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("조건에 맞는 이의제기가 없습니다."))
	}

	list_body += tool.Baro_paging(page, count, size,
		"/baro/appeal/list/{}?status="+tool.Url_parser(status)+"&case="+tool.Url_parser(case_key))

	body += tool.Baro_card(tool.Baro_t("이의제기 목록"), list_body)

	body += tool.Baro_alert("info",
		tool.Baro_t("원조치를 내린 담당자는 그 사건의 이의제기를 처리할 수 없습니다. 이해충돌 사실은 사건 기록에 남습니다."))

	return baro_mod_shell(db, config, tool.Baro_t("이의제기"), "/baro/appeal", body)
}

// View_baro_mod_appeal_mine : 내 이의제기 (/baro/appeal/mine)
func View_baro_mod_appeal_mine(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query
	_ = values

	if page, stop := tool.Baro_login_wall(db, config, "/baro/appeal/mine"); stop {
		return page
	}

	page := baro_mod_page_num(arg)
	size := tool.Baro_page_size

	rows := tool.Query_DB(
		db,
		"select id, case_id, status, reviewer, result, result_reason, date, deadline, close_date from baro_appeal where user = ? order by id + 0 desc limit ?, ?",
		config.IP, (page-1)*size, size,
	)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0

	for rows.Next() {
		id, case_id, status, reviewer, result, result_reason, date, deadline, close_date := "", "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &case_id, &status, &reviewer, &result, &result_reason, &date, &deadline, &close_date); err != nil {
			continue
		}

		count++

		deadline_html := baro_mod_muted(deadline)
		if deadline != "" && status != "closed" {
			deadline_html = baro_mod_esc(tool.Baro_duration_left(deadline))
			if tool.Baro_is_past(deadline) {
				deadline_html = tool.Baro_badge(tool.Baro_t("기한 초과"), "danger")
			}
		}

		reviewer_html := tool.Baro_t(`<span class="bw_muted">배정 중</span>`)
		if reviewer != "" {
			reviewer_html = tool.Baro_user_link(db, reviewer, config.IP)
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/appeal/view/` + tool.Url_parser(id) + `">` + baro_mod_esc(id) + `</a>`,
			baro_mod_case_link(db, case_id),
			baro_mod_status_badge(status),
			baro_mod_appeal_result_badge(result),
			baro_mod_esc(tool.Baro_trunc(tool.Baro_one_line(result_reason), 120)),
			reviewer_html,
			deadline_html,
			tool.Baro_time_cell(baro_mod_or(close_date, date)),
		})
	}

	body := tool.Baro_crumb([][]string{{"/baro/appeal", tool.Baro_t("이의제기")}, {"", tool.Baro_t("내 이의제기")}})

	list_body := tool.Baro_table(
		[]string{"이의번호", "사건", "상태", tool.Baro_t("결과"), tool.Baro_t("결과 근거"), tool.Baro_t("검토자"), "기한", tool.Baro_t("최근")},
		table_rows,
		"",
	)
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("신청한 이의제기가 없습니다."))
	}

	list_body += tool.Baro_paging(page, count, size, "/baro/appeal/mine/{}")

	body += tool.Baro_card(tool.Baro_t("내가 신청한 이의제기"), list_body)
	body += tool.Baro_card("", tool.Baro_button("/baro/case", tool.Baro_t("사건 목록에서 신청하기"), "primary")+" "+tool.Baro_button("/baro/report/mine", tool.Baro_t("내 신고"), ""))
	body += baro_mod_principle_note()

	return baro_mod_shell(db, config, tool.Baro_t("내 이의제기"), "/baro/appeal", body)
}

// ------------------------------------------------------------
//  이의제기 상세
// ------------------------------------------------------------

// View_baro_mod_appeal_view : 이의제기 상세 (/baro/appeal/view/<id>)
func View_baro_mod_appeal_view(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query

	row, ok := baro_mod_appeal_get(db, arg)
	if !ok {
		return baro_mod_missing(db, config, "/baro/appeal", tool.Baro_t("이의제기"), "/baro/appeal/mine", tool.Baro_t("내 이의제기로"))
	}

	case_row, has_case := baro_mod_case_get(db, row.CaseID)

	is_owner := config.IP != "" && row.User == config.IP
	is_reviewer := config.IP != "" && row.Reviewer == config.IP
	can_appeal := tool.Baro_can(db, config.IP, "appeal")

	if !is_owner && !is_reviewer && !can_appeal {
		return tool.Baro_deny(db, config, tool.Baro_t("이의제기 내용은 신청자와 심사 담당자만 볼 수 있습니다."))
	}

	// 이해충돌 : 원조치 담당자는 이 건을 처리할 수 없다
	conflict := has_case && case_row.Actor != "" && case_row.Actor == config.IP
	if conflict {
		baro_mod_coi_mark(db, row.CaseID, config.IP, tool.Baro_t("원조치 담당자가 이의제기 화면에 접근했습니다. 처리 권한을 차단했습니다."))
	}

	can_handle := (is_reviewer || can_appeal) && !conflict

	if values != nil {
		if !can_handle {
			if conflict {
				return tool.Baro_deny(db, config, tool.Baro_t("이해충돌로 처리할 수 없습니다. 원조치를 내린 담당자는 같은 사건의 이의제기를 심사할 수 없습니다."))
			}

			return tool.Baro_deny(db, config, tool.Baro_t("이의제기를 처리할 권한이 없습니다."))
		}

		return baro_mod_appeal_view_post(db, config, row, case_row, has_case, values)
	}

	body := tool.Baro_crumb([][]string{{"/baro/appeal", tool.Baro_t("이의제기")}, {"", "이의번호 " + row.ID}})

	if conflict {
		body += tool.Baro_alert("danger",
			tool.Baro_t("<b>이해충돌로 처리할 수 없습니다.</b> 회원님은 이 사건의 원조치 담당자입니다. ")+
				tool.Baro_t("같은 사람이 조치와 재심을 모두 맡으면 이의제기 제도가 의미를 잃습니다. ")+
				tool.Baro_t("이 접근 사실은 사건 기록(이해충돌)에 남았습니다."))
	}

	deadline_html := baro_mod_muted(row.Deadline)
	if row.Deadline != "" {
		if row.Status == "closed" {
			deadline_html = baro_mod_esc(row.Deadline)
		} else if tool.Baro_is_past(row.Deadline) {
			deadline_html = tool.Baro_badge(tool.Baro_t("기한 초과"), "danger") + " " + baro_mod_esc(row.Deadline)
		} else {
			deadline_html = baro_mod_esc(row.Deadline) + " (" + baro_mod_esc(tool.Baro_duration_left(row.Deadline)) + ")"
		}
	}

	reviewer_html := tool.Baro_t(`<span class="bw_muted">배정 중</span>`)
	if row.Reviewer != "" {
		reviewer_html = tool.Baro_user_link(db, row.Reviewer, config.IP)
	}

	case_html := `<span class="bw_muted">-</span>`
	original_actor := `<span class="bw_muted">-</span>`
	if has_case {
		case_html = `<a href="/baro/case/view/` + tool.Url_parser(case_row.ID) + `">` + baro_mod_esc(case_row.Number) + `</a>`
		if can_handle || is_reviewer || can_appeal {
			original_actor = tool.Baro_user_link(db, case_row.Actor, config.IP)
		} else {
			original_actor = baro_mod_esc(tool.Baro_mask_ip(case_row.Actor))
		}
	}

	body += tool.Baro_card(tool.Baro_t("이의제기 정보"), tool.Baro_kv_grid([][]string{
		{"이의번호", baro_mod_esc(row.ID)},
		{tool.Baro_t("대상 사건"), case_html},
		{tool.Baro_t("신청자"), tool.Baro_user_link(db, row.User, config.IP)},
		{tool.Baro_t("원조치 담당자"), original_actor},
		{tool.Baro_t("심사 담당자"), reviewer_html},
		{"상태", baro_mod_status_badge(row.Status)},
		{tool.Baro_t("결과"), baro_mod_appeal_result_badge(row.Result)},
		{tool.Baro_t("신청 시각"), baro_mod_esc(row.Date)},
		{tool.Baro_t("처리 기한"), deadline_html},
		{tool.Baro_t("처리 시각"), baro_mod_muted(row.CloseDate)},
	}))

	body += tool.Baro_card(tool.Baro_t("이의 사유"), tool.Baro_code(row.Reason))

	if strings.TrimSpace(row.Extra) != "" {
		body += tool.Baro_card(tool.Baro_t("추가 자료"), tool.Baro_code(row.Extra))
	}

	if row.Status == "closed" {
		result_body := tool.Baro_alert(baro_mod_appeal_result_alert_kind(row.Result),
			tool.Baro_t("<b>결과: ")+baro_mod_esc(baro_mod_appeal_result_name(row.Result))+"</b>")
		result_body += tool.Baro_code(baro_mod_or(row.ResultReason, tool.Baro_t("결과 근거가 기록되지 않았습니다.")))

		if row.Result == "reject" {
			result_body += tool.Baro_note(tool.Baro_t("기각되었더라도 새로운 근거가 있으면 재심을 신청할 수 있습니다."))
			if has_case {
				result_body += tool.Baro_button("/baro/appeal/new/"+tool.Url_parser(case_row.ID), "재심 신청", "")
			}
		} else {
			result_body += tool.Baro_note(tool.Baro_t("인용된 이의제기입니다. 원조치가 취소되거나 완화되었습니다."))
		}

		body += tool.Baro_card(tool.Baro_t("심사 결과"), result_body)
	}

	// 사건에 연결된 제재 (인용 시 해제 대상)
	if has_case {
		body += tool.Baro_card(tool.Baro_t("이 사건으로 내려진 제재"), baro_mod_case_sanction_table(db, config, case_row, can_handle))
	}

	if can_handle && row.Status != "closed" {
		form := tool.Baro_select("result", tool.Baro_t("심사 결과"), baro_mod_appeal_result_list(), baro_mod_or(row.Result, "reject"), tool.Baro_t("인용하면 원조치를 취소합니다."))
		form += tool.Baro_textarea("result_reason", tool.Baro_t("결과 근거"), row.ResultReason, 6, tool.Baro_t("왜 그렇게 판단했는지 적습니다."), tool.Baro_t("필수 항목입니다. 신청자에게 그대로 전달되고 공개됩니다."))
		form += tool.Baro_checkbox("release_sanction", tool.Baro_t("인용·일부 인용이면 이 사건의 제재를 함께 해제한다"), "on", true, tool.Baro_t("차단 계열 제재는 openNAMU 차단도 함께 풀립니다."))
		form += tool.Baro_checkbox("reopen_case", tool.Baro_t("사건을 다시 열어 재검토한다"), "on", false, "")
		form += tool.Baro_submit(tool.Baro_t("심사 결과 저장"), "primary")

		body += tool.Baro_card("심사 처리", tool.Baro_form("/baro/appeal/view/"+tool.Url_parser(row.ID), form))

		assign_form := tool.Baro_hidden("act", "assign")
		assign_form += tool.Baro_submit(tool.Baro_t("내가 심사 맡기"), "")
		if !is_reviewer {
			body += tool.Baro_card("심사자 배정", tool.Baro_form("/baro/appeal/view/"+tool.Url_parser(row.ID), assign_form))
		}
	} else if conflict {
		body += tool.Baro_card("심사 처리", tool.Baro_alert("danger", tool.Baro_t("이해충돌로 처리 버튼이 표시되지 않습니다.")))
	} else if is_owner && row.Status != "closed" {
		body += tool.Baro_card(tool.Baro_t("신청자 안내"),
			tool.Baro_list([]string{
				tool.Baro_t("심사 담당자가 배정되면 알림을 보내 드립니다."),
				tool.Baro_t("처리 기한 안에 결과가 나오지 않으면 담당자에게 자동으로 알립니다."),
				tool.Baro_t("결과와 근거는 이 화면에서 그대로 볼 수 있습니다."),
			}),
		)
	}

	// 이해충돌 기록 (담당자에게만)
	if can_appeal || is_reviewer {
		coi := baro_mod_coi_list(db, row.CaseID)
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
	}

	body += baro_mod_principle_note()

	return baro_mod_shell(db, config, tool.Baro_t("이의제기 ")+row.ID, "/baro/appeal", body)
}

// baro_mod_appeal_result_alert_kind : 결과에 맞는 안내 상자 색
func baro_mod_appeal_result_alert_kind(result string) string {
	switch result {
	case "accept":
		return "ok"
	case "partial":
		return "warn"
	case "reject":
		return "danger"
	}

	return "info"
}

// baro_mod_appeal_view_post : 이의제기 심사 처리
func baro_mod_appeal_view_post(db *sql.DB, config tool.Config, row baro_mod_appeal_row, case_row baro_mod_case_row, has_case bool, values url.Values) string {
	target_url := "/baro/appeal/view/" + tool.Url_parser(row.ID)

	if baro_mod_val(values, "act") == "assign" {
		tool.Exec_DB(db, "update baro_appeal set reviewer = ?, status = 'pending' where id = ?", config.IP, row.ID)
		tool.Exec_DB(db, "update baro_case set appeal_status = 'pending' where id = ?", row.CaseID)

		baro_mod_notify_user(db, row.User, "case", tool.Baro_t("이의제기에 심사 담당자가 배정되었습니다 (이의번호 ")+row.ID+")", target_url, "")
		tool.Baro_audit(db, config, "appeal_assign", row.ID, row.Reviewer, config.IP, "이의제기 심사자 배정")

		return tool.Get_redirect(target_url)
	}

	result := baro_mod_val(values, "result")
	result_reason := tool.Baro_trunc(baro_mod_raw(values, "result_reason"), 8000)

	if strings.TrimSpace(result_reason) == "" {
		body := tool.Baro_alert("danger", tool.Baro_t("결과 근거를 반드시 적어 주세요. 근거 없는 심사 결과는 남길 수 없습니다."))
		body += tool.Baro_card("", tool.Baro_button(target_url, "돌아가기", "primary"))

		return baro_mod_shell(db, config, tool.Baro_t("이의제기 심사"), "/baro/appeal", body)
	}

	if baro_mod_appeal_result_name(result) == "미결" {
		body := tool.Baro_alert("danger", tool.Baro_t("심사 결과를 골라 주세요."))
		body += tool.Baro_card("", tool.Baro_button(target_url, "돌아가기", "primary"))

		return baro_mod_shell(db, config, tool.Baro_t("이의제기 심사"), "/baro/appeal", body)
	}

	now := tool.Baro_now()
	reviewer := baro_mod_or(row.Reviewer, config.IP)

	tool.Exec_DB(
		db,
		"update baro_appeal set status = 'closed', result = ?, result_reason = ?, reviewer = ?, close_date = ? where id = ?",
		result, result_reason, reviewer, now, row.ID,
	)

	tool.Exec_DB(db, "update baro_case set appeal_status = ? where id = ?", result, row.CaseID)

	released := 0

	// 인용·일부 인용이면 이 사건의 제재를 해제한다
	if (result == "accept" || result == "partial") && baro_mod_on(values, "release_sanction") && row.CaseID != "" {
		sanction_rows := tool.Query_DB(
			db,
			"select id, user, kind from baro_sanction where case_id = ? and status = 'active'",
			row.CaseID,
		)

		list := [][]string{}
		for sanction_rows.Next() {
			id, user, kind := "", "", ""
			if err := sanction_rows.Scan(&id, &user, &kind); err != nil {
				continue
			}

			list = append(list, []string{id, user, kind})
		}
		sanction_rows.Close()

		for _, item := range list {
			tool.Exec_DB(db, "update baro_sanction set status = 'closed', end_date = ? where id = ?", now, item[0])
			released++

			if baro_mod_sanction_is_block(item[2]) {
				tool.Do_ban_insert(db, item[1], "", tool.Baro_t("이의제기 인용: ")+result_reason, "", config.IP, "", true)
			}

			baro_mod_notify_user(db, item[1], "case",
				tool.Baro_t("이의제기가 인용되어 ")+tool.Baro_sanction_name(item[2])+tool.Baro_t(" 제재가 해제되었습니다"),
				target_url, result_reason)
		}
	}

	// 사건 재검토
	if baro_mod_on(values, "reopen_case") && row.CaseID != "" {
		tool.Exec_DB(db, "update baro_case set status = 'pending', close_date = '' where id = ?", row.CaseID)
	}

	if row.CaseID != "" {
		tool.Exec_DB(
			db,
			"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'note', ?, ?, ?)",
			tool.Baro_next_id(db, "baro_case_action"), row.CaseID, config.IP,
			"이의제기 "+row.ID+" 심사 결과: "+baro_mod_appeal_result_name(result)+
				"\n근거: "+result_reason+
				"\n해제된 제재: "+tool.Baro_itoa(released)+"건",
			now, row.User,
		)
	}

	baro_mod_notify_user(db, row.User, "case",
		tool.Baro_t("이의제기 심사가 끝났습니다: ")+baro_mod_appeal_result_name(result)+tool.Baro_t(" (이의번호 ")+row.ID+")",
		target_url, result_reason)

	number := row.CaseID
	if has_case {
		number = case_row.Number
	}

	tool.Baro_public_log(db, "appeal_result", config.IP, number,
		tool.Baro_trunc(tool.Baro_one_line(result_reason), 200),
		baro_mod_appeal_result_name(result)+" / 해제된 제재 "+tool.Baro_itoa(released)+"건")

	tool.Baro_audit(db, config, "appeal_result", number, row.Status, result, result_reason)
	tool.Baro_metric_bump(db, "appeal_result", tool.Baro_today(), 1)

	return tool.Get_redirect(target_url)
}
