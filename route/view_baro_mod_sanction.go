package route

// ============================================================
//  10-3 제재 종류 (/baro/case/sanction)
//
//  무조건 영구 차단부터 가지 않는다.
//  경고 → 편집 속도 제한 → 특정 문서 제한 → 특정 분야 제한 → 토론 제한 →
//  파일 업로드 제한 → 단기 차단 → 장기 차단 → 영구 차단 순으로 올라간다.
//
//  제재는 사람에게만 적용된다. 이 화면에는 문서를 삭제하는 버튼이 없다. (고정 원칙 1)
//  권한은 관리자 권한이 아니라 반드시 "sanction" 역할로 확인한다. (역할 분리)
// ============================================================

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// baro_mod_sanction_default_days : 제재 종류별 권장 기간 (0 은 기간 없음)
func baro_mod_sanction_default_days(kind string) int {
	switch kind {
	case "slow", "doc_limit", "field_limit", "discussion_limit", "upload_limit":
		return 14
	case "short_block":
		return 3
	case "long_block":
		return 30
	case "perm_block":
		return 0
	}

	return 0
}

// baro_mod_sanction_step_guide : 단계적 제재 안내
func baro_mod_sanction_step_guide(db *sql.DB, user string) string {
	if strings.TrimSpace(user) == "" {
		return tool.Baro_note(tool.Baro_t("대상 사용자를 입력하면 이전 제재 이력을 보고 적절한 단계를 판단할 수 있습니다."))
	}

	history := baro_mod_sanction_history(db, user, 30)

	steps := []string{}
	for _, row := range tool.Baro_sanction_kind_list() {
		count := 0
		for _, item := range history {
			if item[1] == row[0] {
				count++
			}
		}

		label := row[1]
		if count > 0 {
			label += " ×" + tool.Baro_itoa(count)
		}

		steps = append(steps, label)
	}

	body := tool.Baro_pills(steps)

	if len(history) == 0 {
		body += tool.Baro_alert("warn",
			tool.Baro_t("이 사용자에게는 제재 이력이 없습니다. 첫 제재라면 <b>경고</b>나 <b>범위 제한</b>부터 검토하세요. ")+
				tool.Baro_t("첫 조치로 영구 차단을 내리는 것은 규정 위반입니다."))

		return body
	}

	table_rows := [][]string{}
	for _, item := range history {
		table_rows = append(table_rows, []string{
			baro_mod_esc(tool.Baro_sanction_name(item[1])),
			baro_mod_muted(item[2]),
			baro_mod_esc(tool.Baro_trunc(item[3], 160)),
			baro_mod_status_badge(item[4]),
			baro_mod_esc(item[5]),
			baro_mod_esc(baro_mod_or(item[6], "무기한")),
		})
	}

	body += tool.Baro_table(
		[]string{"종류", tool.Baro_t("범위"), "사유", "상태", tool.Baro_t("시작"), tool.Baro_t("종료")},
		table_rows,
		tool.Baro_t("제재 이력이 없습니다."),
	)

	return body
}

// ------------------------------------------------------------
//  제재 부과
// ------------------------------------------------------------

// View_baro_mod_sanction_new : 제재 부과 (/baro/case/sanction)
func View_baro_mod_sanction_new(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = arg

	// 역할 분리 : 관리자 권한만으로는 제재할 수 없다
	if !tool.Baro_can(db, config.IP, "sanction") {
		return tool.Baro_deny(db, config,
			tool.Baro_t("제재는 '제재 담당' 역할을 가진 운영자만 집행할 수 있습니다. ")+
				tool.Baro_t("위키 관리자 권한만으로는 제재를 내릴 수 없습니다. (역할 분리 원칙)"))
	}

	if page, stop := baro_mod_read_only_block(db, config, "/baro/case"); stop {
		return page
	}

	if values != nil {
		return baro_mod_sanction_new_post(db, config, values)
	}

	case_key := baro_mod_val(query, "case")
	user := baro_mod_val(query, "user")
	kind := baro_mod_or(baro_mod_val(query, "kind"), "warn")

	case_row, has_case := baro_mod_case_get(db, case_key)
	if has_case && user == "" && case_row.TargetKind == "user" {
		user = case_row.Target
	}

	body := tool.Baro_crumb([][]string{{"/baro/case", tool.Baro_t("운영 사건")}, {"", "제재 부과"}})

	body += tool.Baro_alert("warn",
		tool.Baro_t("제재는 <b>사람</b>에게만 적용됩니다. 제재를 이유로 문서를 지우지 않습니다. ")+
			tool.Baro_t("문서에 문제가 있다면 <a href=\"/baro/quarantine/new\">문서 격리</a>로 따로 판단하세요. (고정 원칙 1)"))

	if has_case {
		body += tool.Baro_card(tool.Baro_t("연결된 사건"), tool.Baro_kv_grid([][]string{
			{"사건 번호", `<a href="/baro/case/view/` + tool.Url_parser(case_row.ID) + `">` + baro_mod_esc(case_row.Number) + `</a>`},
			{tool.Baro_t("유형"), baro_mod_esc(baro_mod_report_kind_name(case_row.Kind))},
			{tool.Baro_t("적용 규정"), baro_mod_policy_link(db, case_row.Rule)},
		}))
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("사건 번호 없이도 제재를 기록할 수 있지만, 이의제기와 근거 공개를 위해 사건을 먼저 만드는 것을 권합니다."))
	}

	body += tool.Baro_card(tool.Baro_t("이 사용자의 제재 이력"), baro_mod_sanction_step_guide(db, user))

	kind_options := [][]string{}
	for _, row := range tool.Baro_sanction_kind_list() {
		days := baro_mod_sanction_default_days(row[0])
		label := row[1]
		if days > 0 {
			label += tool.Baro_t(" (권장 ") + tool.Baro_itoa(days) + tool.Baro_t("일)")
		} else if baro_mod_sanction_is_block(row[0]) {
			label += tool.Baro_t(" (무기한)")
		}

		kind_options = append(kind_options, []string{row[0], label})
	}

	form := tool.Baro_input("user", tool.Baro_t("대상 사용자"), user, tool.Baro_t("사용자 이름"), tool.Baro_t("제재는 사람에게만 적용됩니다."))
	form += tool.Baro_select("kind", tool.Baro_t("제재 종류"), kind_options, kind, tool.Baro_t("약한 단계부터 차례로 적용합니다."))
	form += tool.Baro_input("scope", tool.Baro_t("범위"), "", tool.Baro_t("문서 이름 또는 분야 키"), tool.Baro_t("'특정 문서 제한' 이면 문서 이름, '특정 분야 제한' 이면 분야 키를 적습니다. 비우면 전체입니다."))
	form += tool.Baro_input_type("days", tool.Baro_t("기간(일)"), "", "number", tool.Baro_t("0 이나 빈 칸은 무기한입니다. 영구 차단이 아니면 반드시 기간을 정하세요."))
	form += tool.Baro_textarea("reason", "사유", "", 5, tool.Baro_t("왜 이 제재가 필요한지 적습니다."), tool.Baro_t("필수 항목입니다. 이 사유는 대상에게 그대로 전달되고 공개 로그에도 남습니다."))
	form += tool.Baro_input("case_id", tool.Baro_t("연결 사건"), baro_mod_case_id_or_empty(case_row, has_case), tool.Baro_t("사건 번호 또는 내부 id"), tool.Baro_t("사건과 연결하면 이의제기가 가능해집니다."))
	form += tool.Baro_checkbox("confirm", tool.Baro_t("단계적 제재 원칙을 확인했고, 이 단계가 적절하다고 판단했습니다"), "on", false, tool.Baro_t("확인하지 않으면 저장되지 않습니다."))
	form += tool.Baro_submit("제재 부과", "danger")

	body += tool.Baro_card("제재 부과", tool.Baro_form("/baro/case/sanction", form))

	body += tool.Baro_card(tool.Baro_t("제재 종류"),
		tool.Baro_table(
			[]string{tool.Baro_t("단계"), "종류", tool.Baro_t("openNAMU 차단 연동"), tool.Baro_t("기본 기간")},
			baro_mod_sanction_kind_rows(),
			"",
		),
	)

	body += tool.Baro_card("", tool.Baro_button("/baro/case/sanction/list", tool.Baro_t("적용 중인 제재 목록"), "")+" "+tool.Baro_button("/baro/policy/view/sanction", tool.Baro_t("제재 규정 보기"), ""))
	body += baro_mod_principle_note()

	return baro_mod_shell(db, config, "제재 부과", "/baro/case/sanction/list", body)
}

// baro_mod_case_id_or_empty : 사건이 있으면 내부 id, 없으면 빈 문자열
func baro_mod_case_id_or_empty(row baro_mod_case_row, ok bool) string {
	if !ok {
		return ""
	}

	return row.ID
}

// baro_mod_sanction_kind_rows : 제재 종류 설명 표
func baro_mod_sanction_kind_rows() [][]string {
	out := [][]string{}
	for index, row := range tool.Baro_sanction_kind_list() {
		block := tool.Baro_t(`<span class="bw_muted">연동 안 함</span>`)
		if baro_mod_sanction_is_block(row[0]) {
			block = tool.Baro_badge(tool.Baro_t("차단 연동"), "danger")
		}

		days := baro_mod_sanction_default_days(row[0])
		days_text := tool.Baro_t("무기한 / 지정 없음")
		if days > 0 {
			days_text = tool.Baro_itoa(days) + tool.Baro_t("일")
		}

		out = append(out, []string{
			tool.Baro_itoa(index + 1),
			baro_mod_esc(row[1]),
			block,
			baro_mod_esc(days_text),
		})
	}

	return out
}

// baro_mod_sanction_new_post : 제재 부과 처리
func baro_mod_sanction_new_post(db *sql.DB, config tool.Config, values url.Values) string {
	user := baro_mod_val(values, "user")
	kind := baro_mod_val(values, "kind")
	scope := baro_mod_val(values, "scope")
	reason := tool.Baro_trunc(baro_mod_raw(values, "reason"), 4000)
	case_key := baro_mod_val(values, "case_id")
	days := tool.Str_to_int(baro_mod_val(values, "days"))

	fail := func(message string) string {
		body := tool.Baro_alert("danger", baro_mod_esc(message))
		body += tool.Baro_card("", tool.Baro_button("/baro/case/sanction", tool.Baro_t("제재 화면으로 돌아가기"), "primary"))

		return baro_mod_shell(db, config, "제재 부과", "/baro/case/sanction/list", body)
	}

	if user == "" {
		return fail(tool.Baro_t("대상 사용자를 적어 주세요."))
	}

	if strings.TrimSpace(reason) == "" {
		return fail(tool.Baro_t("제재 사유를 반드시 적어 주세요. 사유 없는 제재는 이의제기에서 그대로 뒤집힙니다."))
	}

	if !baro_mod_on(values, "confirm") {
		return fail(tool.Baro_t("단계적 제재 원칙 확인란을 체크해 주세요."))
	}

	if tool.Baro_sanction_name(kind) == kind {
		return fail(tool.Baro_t("제재 종류를 올바르게 골라 주세요."))
	}

	// 자기 자신에게는 제재를 내릴 수 없다
	if user == config.IP {
		return fail(tool.Baro_t("자기 자신에게는 제재를 내릴 수 없습니다."))
	}

	case_row, has_case := baro_mod_case_get(db, case_key)
	case_id := ""
	if has_case {
		case_id = case_row.ID
	}

	now := tool.Baro_now()
	end_date := ""
	if days > 0 {
		end_date = tool.Baro_time_add(now, days, 0, 0)
	}

	id := tool.Baro_next_id(db, "baro_sanction")

	tool.Exec_DB(
		db,
		"insert into baro_sanction (id, user, kind, scope, reason, case_id, actor, start_date, end_date, status, date) values (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?)",
		id, user, kind, scope, reason, case_id, config.IP, now, end_date, now,
	)

	// 차단 계열은 openNAMU 차단과 연동한다
	if baro_mod_sanction_is_block(kind) {
		ban_end := end_date
		if kind == "perm_block" || ban_end == "" {
			ban_end = "0"
		}

		tool.Do_ban_insert(db, user, ban_end, "["+baro_mod_or(case_row.Number, tool.Baro_t("사건 없음"))+"] "+reason, "", config.IP, "", false)
		tool.Send_alarm(db, config.IP, user, "제재로 차단되었습니다. 종료 : "+baro_mod_or(end_date, "무기한")+" / 사유 : "+reason)
	}

	// 사건 조치 이력에도 남긴다
	if case_id != "" {
		tool.Exec_DB(
			db,
			"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'sanction', ?, ?, ?)",
			tool.Baro_next_id(db, "baro_case_action"), case_id, config.IP,
			tool.Baro_sanction_name(kind)+" 제재를 집행했습니다. 범위: "+baro_mod_or(scope, "전체")+" / 종료: "+baro_mod_or(end_date, "무기한")+"\n사유: "+reason,
			now, user,
		)
	}

	// 대상에게 알림 + 공개 로그 + 내부 감사 로그
	baro_mod_notify_user(db, user, "case",
		tool.Baro_sanction_name(kind)+tool.Baro_t(" 제재가 적용되었습니다")+baro_mod_case_suffix(case_row, has_case),
		baro_mod_sanction_notify_url(case_row, has_case),
		reason,
	)

	tool.Baro_public_log(db, "sanction", config.IP, tool.Baro_mask_ip(user),
		tool.Baro_trunc(tool.Baro_one_line(reason), 200),
		tool.Baro_sanction_name(kind)+" / 범위 "+baro_mod_or(scope, "전체")+" / 종료 "+baro_mod_or(end_date, "무기한")+baro_mod_case_suffix(case_row, has_case))

	tool.Baro_audit(db, config, "sanction", user, "", tool.Baro_sanction_name(kind)+" scope="+scope+" end="+end_date, reason)
	tool.Baro_metric_bump(db, "sanction", tool.Baro_today(), 1)

	if case_id != "" {
		return tool.Get_redirect("/baro/case/view/" + tool.Url_parser(case_id))
	}

	return tool.Get_redirect("/baro/case/sanction/list")
}

// baro_mod_case_suffix : 알림·로그에 붙일 사건 번호 꼬리말
func baro_mod_case_suffix(row baro_mod_case_row, ok bool) string {
	if !ok {
		return ""
	}

	return tool.Baro_t(" (사건 ") + row.Number + ")"
}

// baro_mod_sanction_notify_url : 제재 알림이 가리킬 주소
func baro_mod_sanction_notify_url(row baro_mod_case_row, ok bool) string {
	if !ok {
		return "/baro/case/sanction/list"
	}

	return "/baro/case/view/" + tool.Url_parser(row.ID)
}

// ------------------------------------------------------------
//  적용 중인 제재 목록
// ------------------------------------------------------------

// View_baro_mod_sanction_list : 적용 중인 제재 목록 (/baro/case/sanction/list)
func View_baro_mod_sanction_list(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	page := baro_mod_page_num(arg)
	size := tool.Baro_page_size

	status := baro_mod_or(baro_mod_val(query, "status"), "active")
	kind := baro_mod_val(query, "kind")
	user := baro_mod_val(query, "user")

	can_handle := tool.Baro_can(db, config.IP, "sanction")

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

	if user != "" {
		where += " and user = ?"
		params = append(params, user)
	}

	params = append(params, (page-1)*size, size)

	rows := tool.Query_DB(
		db,
		"select id, user, kind, scope, reason, case_id, actor, start_date, end_date, status from baro_sanction"+where+" order by id + 0 desc limit ?, ?",
		params...,
	)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0
	expired := []string{}

	for rows.Next() {
		id, s_user, s_kind, scope, reason, case_id, actor, start_date, end_date, s_status := "", "", "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &s_user, &s_kind, &scope, &reason, &case_id, &actor, &start_date, &end_date, &s_status); err != nil {
			continue
		}

		count++

		// 만료된 것은 화면에서 만료로 보여 주고 뒤에서 상태를 정리한다
		if s_status == "active" && tool.Baro_is_past(end_date) {
			s_status = "expired"
			expired = append(expired, id)
		}

		user_html := tool.Baro_user_link(db, s_user, config.IP)
		if !can_handle {
			user_html = baro_mod_esc(tool.Baro_mask_ip(s_user))
		}

		manage := `<span class="bw_muted">-</span>`
		if can_handle && s_status == "active" {
			manage = tool.Baro_button("/baro/case/sanction/release/"+tool.Url_parser(id), "해제", "danger")
		}

		actor_html := baro_mod_esc(tool.Baro_mask_ip(actor))
		if can_handle {
			actor_html = tool.Baro_user_link(db, actor, config.IP)
		}

		table_rows = append(table_rows, []string{
			baro_mod_esc(id),
			user_html,
			baro_mod_esc(tool.Baro_sanction_name(s_kind)),
			baro_mod_muted(scope),
			baro_mod_esc(tool.Baro_trunc(tool.Baro_one_line(reason), 120)),
			baro_mod_case_link(db, case_id),
			baro_mod_status_badge(s_status),
			baro_mod_esc(baro_mod_or(end_date, "무기한")) + `<br><span class="bw_muted">` + baro_mod_esc(tool.Baro_duration_left(end_date)) + `</span>`,
			actor_html,
			manage,
		})
	}

	for _, id := range expired {
		tool.Exec_DB(db, "update baro_sanction set status = 'expired' where id = ?", id)
	}

	body := tool.Baro_crumb([][]string{{"/baro/case", tool.Baro_t("운영 사건")}, {"", "제재"}})

	active_count, block_count := 0, 0
	tool.QueryRow_DB(db, "select count(*) from baro_sanction where status = 'active'", []any{&active_count})
	tool.QueryRow_DB(db, "select count(*) from baro_sanction where status = 'active' and kind in ('short_block', 'long_block', 'perm_block')", []any{&block_count})

	body += tool.Baro_card(tool.Baro_t("제재 현황"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("적용 중"), tool.Baro_comma(active_count), tool.Baro_t("만료되지 않은 제재")},
		{tool.Baro_t("차단 계열"), tool.Baro_comma(block_count), tool.Baro_t("단기 · 장기 · 영구 차단")},
		{tool.Baro_t("제재 단계 수"), tool.Baro_itoa(len(tool.Baro_sanction_kind_list())), tool.Baro_t("경고부터 영구 차단까지")},
	}))

	filter_form := `<form class="bw_form" method="get" action="/baro/case/sanction/list">`
	filter_form += tool.Baro_select("status", "상태", [][]string{
		{"active", tool.Baro_t("적용 중")},
		{"expired", tool.Baro_t("만료")},
		{"closed", tool.Baro_t("해제됨")},
		{"all", tool.Baro_t("전체")},
	}, status, "")
	filter_form += tool.Baro_select("kind", "종류", append([][]string{{"", tool.Baro_t("전체")}}, baro_mod_sanction_options()...), kind, "")
	filter_form += tool.Baro_input("user", tool.Baro_t("사용자"), user, tool.Baro_t("사용자 이름"), "")
	filter_form += tool.Baro_submit(tool.Baro_t("적용"), "")
	filter_form += `</form>`

	body += tool.Baro_card("걸러 보기", filter_form)

	list_body := tool.Baro_table(
		[]string{"번호", tool.Baro_t("대상"), "종류", tool.Baro_t("범위"), "사유", "사건", "상태", tool.Baro_t("종료"), tool.Baro_t("집행자"), "관리"},
		table_rows,
		"",
	)
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("조건에 맞는 제재가 없습니다."))
	}

	list_body += tool.Baro_paging(page, count, size,
		"/baro/case/sanction/list/{}?status="+tool.Url_parser(status)+"&kind="+tool.Url_parser(kind)+"&user="+tool.Url_parser(user))

	body += tool.Baro_card(tool.Baro_t("제재 목록"), list_body)

	if can_handle {
		body += tool.Baro_card("", tool.Baro_button("/baro/case/sanction", "제재 부과", "primary"))
	}

	body += tool.Baro_alert("info",
		tool.Baro_t("이 화면에는 문서를 삭제하는 기능이 없습니다. 제재된 사용자의 정상 기여는 그대로 유지됩니다. ")+
			tool.Baro_t("문서 처리가 필요하면 <a href=\"/baro/quarantine\">문서 격리</a>에서 리비전 단위로 판단하세요."))

	body += baro_mod_principle_note()

	return baro_mod_shell(db, config, tool.Baro_t("제재 목록"), "/baro/case/sanction/list", body)
}

// ------------------------------------------------------------
//  제재 해제
// ------------------------------------------------------------

// View_baro_mod_sanction_release : 제재 해제 (/baro/case/sanction/release/<id>)
func View_baro_mod_sanction_release(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query

	if !tool.Baro_can(db, config.IP, "sanction") {
		return tool.Baro_deny(db, config, tool.Baro_t("제재 해제는 '제재 담당' 역할이 있어야 합니다."))
	}

	id, user, kind, scope, reason, case_id, start_date, end_date, status := "", "", "", "", "", "", "", "", ""
	found := tool.QueryRow_DB(
		db,
		"select id, user, kind, scope, reason, case_id, start_date, end_date, status from baro_sanction where id = ?",
		[]any{&id, &user, &kind, &scope, &reason, &case_id, &start_date, &end_date, &status},
		arg,
	)

	if !found || id == "" {
		return baro_mod_missing(db, config, "/baro/case/sanction/list", "제재", "/baro/case/sanction/list", tool.Baro_t("제재 목록으로"))
	}

	if values != nil {
		release_reason := tool.Baro_trunc(baro_mod_raw(values, "reason"), 2000)
		if strings.TrimSpace(release_reason) == "" {
			body := tool.Baro_alert("danger", tool.Baro_t("해제 사유를 적어 주세요."))
			body += tool.Baro_card("", tool.Baro_button("/baro/case/sanction/release/"+tool.Url_parser(id), "돌아가기", "primary"))

			return baro_mod_shell(db, config, "제재 해제", "/baro/case/sanction/list", body)
		}

		now := tool.Baro_now()
		tool.Exec_DB(db, "update baro_sanction set status = 'closed', end_date = ? where id = ?", now, id)

		if baro_mod_sanction_is_block(kind) {
			tool.Do_ban_insert(db, user, "", release_reason, "", config.IP, "", true)
			tool.Send_alarm(db, config.IP, user, "차단이 해제되었습니다. 사유 : "+release_reason)
		}

		if case_id != "" {
			tool.Exec_DB(
				db,
				"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'note', ?, ?, ?)",
				tool.Baro_next_id(db, "baro_case_action"), case_id, config.IP,
				tool.Baro_sanction_name(kind)+" 제재를 해제했습니다. 사유: "+release_reason, now, user,
			)
		}

		baro_mod_notify_user(db, user, "case", tool.Baro_sanction_name(kind)+tool.Baro_t(" 제재가 해제되었습니다"), "/baro/case/sanction/list", release_reason)

		tool.Baro_public_log(db, "sanction_release", config.IP, tool.Baro_mask_ip(user), release_reason, tool.Baro_sanction_name(kind))
		tool.Baro_audit(db, config, "sanction_release", user, "active", "closed", release_reason)

		return tool.Get_redirect("/baro/case/sanction/list")
	}

	body := tool.Baro_crumb([][]string{
		{"/baro/case/sanction/list", "제재"},
		{"", "해제"},
	})

	body += tool.Baro_card(tool.Baro_t("해제할 제재"), tool.Baro_kv_grid([][]string{
		{"번호", baro_mod_esc(id)},
		{tool.Baro_t("대상"), tool.Baro_user_link(db, user, config.IP)},
		{"종류", baro_mod_esc(tool.Baro_sanction_name(kind))},
		{tool.Baro_t("범위"), baro_mod_muted(scope)},
		{"사유", baro_mod_esc(tool.Baro_trunc(reason, 500))},
		{"사건", baro_mod_case_link(db, case_id)},
		{tool.Baro_t("시작"), baro_mod_esc(start_date)},
		{tool.Baro_t("종료 예정"), baro_mod_esc(baro_mod_or(end_date, "무기한"))},
		{tool.Baro_t("현재 상태"), baro_mod_status_badge(status)},
	}))

	if status != "active" {
		body += tool.Baro_alert("info", tool.Baro_t("이미 적용 중이 아닌 제재입니다. 해제 기록만 남습니다."))
	}

	form := tool.Baro_textarea("reason", tool.Baro_t("해제 사유"), "", 4, tool.Baro_t("왜 해제하는지 적습니다."), tool.Baro_t("필수 항목입니다. 공개 로그에 남습니다."))
	form += tool.Baro_submit("제재 해제", "danger")

	body += tool.Baro_danger_zone("제재 해제", tool.Baro_form("/baro/case/sanction/release/"+tool.Url_parser(id), form))

	if baro_mod_sanction_is_block(kind) {
		body += tool.Baro_alert("warn", tool.Baro_t("차단 계열 제재입니다. 해제하면 openNAMU 차단도 함께 풀립니다."))
	}

	return baro_mod_shell(db, config, "제재 해제", "/baro/case/sanction/list", body)
}
