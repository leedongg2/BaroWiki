package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  16-6 창립 시즌 관리
//
//  지원자 · 분야 · 초대 코드 · 첫 유효 기여 · 번호 발급/회수 · 배지 ·
//  중복/봇 계정 검토를 한 화면에서 처리한다.
// ============================================================

// baro_admin_founding_status_badge : 창립 지원 상태 배지
func baro_admin_founding_status_badge(status string) string {
	switch status {
	case "confirmed":
		return tool.Baro_badge(tool.Baro_t("확정"), "ok")
	case "rejected":
		return tool.Baro_badge(tool.Baro_t("반려 · 회수"), "danger")
	case "pending":
		return tool.Baro_badge(tool.Baro_t("보류"), "warn")
	case "", "apply", "applied":
		return tool.Baro_badge(tool.Baro_t("심사 대기"), "info")
	}

	return baro_admin_status_badge(status)
}

// baro_admin_founding_next_number : 다음 창립 기여자 번호
func baro_admin_founding_next_number(db *sql.DB) int {
	max_number := 0

	for _, one := range baro_admin_rows(db, 1, "select number from baro_founder where number != '' limit 2000") {
		if value := tool.Str_to_int(one[0]); value > max_number {
			max_number = value
		}
	}

	return max_number + 1
}

// baro_admin_founding_first_event : 첫 유효 기여 (없으면 "")
//
//	baro_founder.first_event 에 기록이 있으면 그것을 쓰고,
//	없으면 기여 이벤트 표에서 가장 오래된 것을 찾는다.
func baro_admin_founding_first_event(db *sql.DB, user string, saved string) (string, string) {
	if strings.TrimSpace(saved) != "" {
		row := baro_admin_rows(db, 3, "select id, kind, title from baro_contrib_event where id = ?", saved)
		if len(row) > 0 {
			return row[0][0], tool.Baro_contrib_kind_name(row[0][1]) + " · " + row[0][2]
		}

		return saved, tool.Baro_t("기록 ") + saved
	}

	row := baro_admin_rows(
		db, 3,
		"select id, kind, title from baro_contrib_event where user = ? order by id + 0 asc limit 1",
		user,
	)

	if len(row) == 0 {
		return "", ""
	}

	return row[0][0], tool.Baro_contrib_kind_name(row[0][1]) + " · " + row[0][2]
}

// baro_admin_founding_bot_risk : 봇·중복 계정 의심 신호
//
//	돌려주는 값 : (경고 HTML 묶음, 위험 여부)
func baro_admin_founding_bot_risk(db *sql.DB, user string) (string, bool) {
	warn := []string{}

	_, same_ip, ip_source := baro_admin_user_same_ip(db, user)
	if len(same_ip) >= 3 {
		warn = append(warn, tool.Baro_badge(tool.Baro_t("같은 IP 계정 ")+tool.Baro_itoa(len(same_ip))+tool.Baro_t("개 (")+ip_source+")", "danger"))
	} else if len(same_ip) > 0 {
		warn = append(warn, tool.Baro_badge(tool.Baro_t("같은 IP 계정 ")+tool.Baro_itoa(len(same_ip))+tool.Baro_t("개 (")+ip_source+")", "warn"))
	}

	// 가입 직후 기계적 편집 : 가입 뒤 1시간 안의 편집 수와 편집 간격
	join_date := baro_admin_one(db, "select data from user_set where id = ? and name = 'date'", user)
	if join_date != "" {
		hour_after := tool.Baro_time_add(join_date, 0, 1, 0)

		quick := baro_admin_num(
			db,
			"select count(*) from history where ip = ? and date >= ? and date <= ?",
			user, join_date, hour_after,
		)

		if quick >= 30 {
			warn = append(warn, tool.Baro_badge(tool.Baro_t("가입 1시간 내 편집 ")+tool.Baro_itoa(quick)+tool.Baro_t("회"), "danger"))
		} else if quick >= 10 {
			warn = append(warn, tool.Baro_badge(tool.Baro_t("가입 1시간 내 편집 ")+tool.Baro_itoa(quick)+tool.Baro_t("회"), "warn"))
		}
	}

	_, burst, _ := baro_admin_user_burst(db, user)
	if burst >= 20 {
		warn = append(warn, tool.Baro_badge(tool.Baro_t("10분 최대 ")+tool.Baro_itoa(burst)+tool.Baro_t("회"), "danger"))
	}

	if len(warn) == 0 {
		return tool.Baro_badge(tool.Baro_t("특이 신호 없음"), "ok"), false
	}

	return strings.Join(warn, " "), true
}

// baro_admin_founding_post : 창립 시즌 관리 쓰기 동작
func baro_admin_founding_post(db *sql.DB, config tool.Config, values url.Values) (string, string) {
	do := baro_admin_get(values, "do")
	user := baro_admin_get(values, "user")
	reason := baro_admin_get(values, "reason")

	if user == "" {
		return "", tool.Baro_alert("warn", tool.Baro_t("대상 계정 이름이 없습니다."))
	}

	row := baro_admin_rows(db, 4, "select user, number, status, first_event from baro_founder where user = ?", user)
	if len(row) == 0 {
		return "", tool.Baro_alert("warn", tool.Baro_t("그 계정의 창립 시즌 지원 기록이 없습니다."))
	}

	switch do {
	case "confirm":
		event_id, _ := baro_admin_founding_first_event(db, user, row[0][3])
		if event_id == "" && baro_admin_get(values, "force") == "" {
			return "", tool.Baro_alert("warn", tool.Baro_t("첫 유효 기여가 확인되지 않았습니다. 그래도 발급하려면 '기여 확인 없이 발급'을 체크하세요."))
		}

		number := row[0][1]
		if number == "" {
			number = tool.Baro_itoa(baro_admin_founding_next_number(db))
		}

		tool.Exec_DB(
			db,
			"update baro_founder set number = ?, status = 'confirmed', date = ?, first_event = ? where user = ?",
			number, tool.Baro_now(), event_id, user,
		)

		// 창립 기여자 배지
		found := baro_admin_one(db, "select id from baro_user_badge where user = ? and badge = 'founder'", user)
		if found == "" {
			tool.Exec_DB(
				db,
				"insert into baro_user_badge (id, user, badge, date, detail) values (?, ?, 'founder', ?, ?)",
				tool.Baro_next_id(db, "baro_user_badge"), user, tool.Baro_now(), "창립 기여자 #"+number,
			)
		}

		baro_admin_log(db, config, "founder_confirm", user, reason, row[0][2], "confirmed #"+number)
		tool.Baro_notify(db, user, "founding", "창립 기여자 번호 #"+number+" 가 발급되었습니다.", "/baro/founding", reason)

		return tool.Get_redirect("/baro/admin/founding?done=confirm"), ""

	case "revoke":
		if reason == "" {
			return "", tool.Baro_alert("warn", tool.Baro_t("회수 사유는 반드시 적어야 합니다."))
		}

		tool.Exec_DB(db, "update baro_founder set status = 'rejected', number = '' where user = ?", user)
		tool.Exec_DB(db, "delete from baro_user_badge where user = ? and badge = 'founder'", user)

		baro_admin_log(db, config, "founder_revoke", user, reason, "confirmed #"+row[0][1], "rejected")
		tool.Baro_notify(db, user, "founding", "창립 기여자 번호가 회수되었습니다.", "/baro/founding", reason)

		return tool.Get_redirect("/baro/admin/founding?done=revoke"), ""

	case "hold":
		tool.Exec_DB(db, "update baro_founder set status = 'pending' where user = ?", user)
		baro_admin_log(db, config, "founder_hold", user, reason, row[0][2], "pending")

		return tool.Get_redirect("/baro/admin/founding?done=hold"), ""

	case "note":
		note := baro_admin_get(values, "note")
		tool.Exec_DB(db, "update baro_founder set note = ? where user = ?", note, user)
		tool.Baro_audit(db, config, "founder_note", "user:"+user, row[0][2], tool.Baro_trunc(note, 300), "심사 메모")

		return tool.Get_redirect("/baro/admin/founding?done=note"), ""
	}

	return "", tool.Baro_alert("warn", tool.Baro_t("알 수 없는 동작입니다."))
}

// View_baro_admin_founding : 16-6 창립 시즌 관리
func View_baro_admin_founding(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if html, stop := baro_admin_gate(db, config, "founding"); stop {
		return html
	}

	notice := ""
	if values != nil {
		redirect, message := baro_admin_founding_post(db, config, values)
		if redirect != "" {
			return redirect
		}

		notice = message
	}

	tab := baro_admin_get(query, "tab")
	if tab == "" {
		tab = "apply"
	}

	goal := tool.Baro_get_int(db, "founding_goal", 100)
	confirmed := baro_admin_num(db, "select count(*) from baro_founder where status = 'confirmed'")
	waiting := baro_admin_num(db, "select count(*) from baro_founder where status not in ('confirmed', 'rejected')")
	rejected := baro_admin_num(db, "select count(*) from baro_founder where status = 'rejected'")

	body := tool.Baro_hero(
		tool.Baro_t("창립 시즌 관리"),
		tool.Baro_t("지원자 심사 · 번호 발급 · 초대 코드 · 중복 계정 검토"),
		tool.Baro_button("/baro/founding", tool.Baro_t("창립 시즌 화면"), "")+" "+tool.Baro_button("/baro/invite", tool.Baro_t("초대 코드"), ""),
	)
	body += baro_admin_tabs("/baro/admin/founding")

	if notice != "" {
		body += notice
	}
	if done := baro_admin_get(query, "done"); done != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("처리했습니다. (")+baro_admin_esc(done)+")")
	}

	body += tool.Baro_card(tool.Baro_t("창립 시즌 현황"), tool.Baro_progress(confirmed, goal, tool.Baro_t("창립 기여자 확정"))+
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("확정"), tool.Baro_comma(confirmed), tool.Baro_t("번호 발급 완료")},
			{tool.Baro_t("심사 대기"), tool.Baro_comma(waiting), tool.Baro_t("지원 · 보류")},
			{tool.Baro_t("반려 · 회수"), tool.Baro_comma(rejected), ""},
			{tool.Baro_t("목표"), tool.Baro_comma(goal), "baro_set.founding_goal"},
			{tool.Baro_t("다음 번호"), "#" + tool.Baro_itoa(baro_admin_founding_next_number(db)), tool.Baro_t("자동 계산")},
		}))

	body += tool.Baro_tabs([][]string{
		{"/baro/admin/founding?tab=apply", tool.Baro_t("지원자")},
		{"/baro/admin/founding?tab=field", tool.Baro_t("분야별 현황")},
		{"/baro/admin/founding?tab=invite", tool.Baro_t("초대 코드")},
		{"/baro/admin/founding?tab=risk", tool.Baro_t("중복 · 봇 계정 검토")},
	}, "/baro/admin/founding?tab="+tab)

	switch tab {
	case "field":
		body += baro_admin_founding_field_tab(db)
	case "invite":
		body += baro_admin_founding_invite_tab(db, config, query)
	case "risk":
		body += baro_admin_founding_risk_tab(db, config, query)
	default:
		body += baro_admin_founding_apply_tab(db, config, query)
	}

	return baro_admin_page(db, config, tool.Baro_t("창립 시즌 관리"), body)
}

// baro_admin_founding_apply_tab : 지원자 목록
func baro_admin_founding_apply_tab(db *sql.DB, config tool.Config, query url.Values) string {
	page := baro_admin_page_num(query)
	size := tool.Baro_page_size
	status := baro_admin_get(query, "status")

	sql_text := "select user, number, status, first_event, field, apply_date, note from baro_founder"
	args := []any{}

	if status != "" {
		sql_text += " where status = ?"
		args = append(args, status)
	}

	sql_text += " order by apply_date asc limit ?, ?"
	args = append(args, (page-1)*size, size)

	list := baro_admin_rows(db, 7, sql_text, args...)

	rows := [][]string{}
	for _, one := range list {
		event_id, event_name := baro_admin_founding_first_event(db, one[0], one[3])

		event_html := tool.Baro_badge(tool.Baro_t("첫 기여 없음"), "danger")
		if event_id != "" {
			event_html = tool.Baro_badge(tool.Baro_t("확인됨"), "ok") + " " + baro_admin_esc(tool.Baro_trunc(event_name, 30))
		}

		number_html := `<span class="bw_muted">-</span>`
		if one[1] != "" {
			number_html = tool.Baro_badge("#"+one[1], "ok")
		}

		status_html := baro_admin_founding_status_badge(one[2])

		risk_html, _ := baro_admin_founding_bot_risk(db, one[0])

		rows = append(rows, []string{
			tool.Baro_user_link(db, one[0], config.IP),
			baro_admin_esc(tool.Baro_field_name(db, one[4])),
			status_html,
			number_html,
			event_html,
			risk_html,
			tool.Baro_time_cell(one[5]),
			tool.Baro_button("/baro/admin/founding?tab=apply&user="+tool.Url_parser(one[0]), "심사", "primary"),
		})
	}

	table := tool.Baro_table(
		[]string{tool.Baro_t("지원자"), "분야", "상태", "번호", tool.Baro_t("첫 유효 기여"), tool.Baro_t("의심 신호"), tool.Baro_t("지원 시각"), ""},
		rows,
		tool.Baro_t("지원자가 없습니다."),
	)

	if len(rows) == 0 {
		table += tool.Baro_empty(tool.Baro_t("이 조건의 지원자가 없습니다."))
	}

	paging_url := "/baro/admin/founding?tab=apply"
	if status != "" {
		paging_url += "&status=" + tool.Url_parser(status)
	}
	table += tool.Baro_paging(page, len(rows), size, paging_url+"&page={}")

	filter := tool.Baro_tabs([][]string{
		{"/baro/admin/founding?tab=apply", tool.Baro_t("전체")},
		{"/baro/admin/founding?tab=apply&status=pending", tool.Baro_t("심사 대기")},
		{"/baro/admin/founding?tab=apply&status=confirmed", tool.Baro_t("확정")},
		{"/baro/admin/founding?tab=apply&status=rejected", tool.Baro_t("반려 · 회수")},
	}, "/baro/admin/founding?tab=apply"+func() string {
		if status == "" {
			return ""
		}

		return "&status=" + status
	}())

	card := tool.Baro_card_ex(tool.Baro_t("지원자 목록"), tool.Baro_t("지원 순서대로 보여 줍니다."), filter+table, "")

	if user := baro_admin_get(query, "user"); user != "" {
		card += baro_admin_founding_apply_form(db, config, user)
	} else {
		card += tool.Baro_note(tool.Baro_t("표의 '심사' 단추를 누르면 번호 발급 · 회수 · 메모 칸이 열립니다."))
	}

	return card
}

// baro_admin_founding_apply_form : 지원자 한 명 심사 폼
func baro_admin_founding_apply_form(db *sql.DB, config tool.Config, user string) string {
	row := baro_admin_rows(
		db, 7,
		"select user, number, status, first_event, field, apply_date, note from baro_founder where user = ?",
		user,
	)

	if len(row) == 0 {
		return tool.Baro_alert("warn", tool.Baro_t("그 계정의 창립 시즌 지원 기록이 없습니다."))
	}

	one := row[0]
	action := "/baro/admin/founding"

	event_id, event_name := baro_admin_founding_first_event(db, user, one[3])
	risk_html, risky := baro_admin_founding_bot_risk(db, user)

	contrib_num := baro_admin_num(db, "select count(*) from baro_contrib_event where user = ?", user)
	edit_num := baro_admin_num(db, "select count(*) from history where ip = ?", user)
	invite_used := baro_admin_num(db, "select count(*) from baro_invite where target = ? and status = 'used'", user)

	info := tool.Baro_kv_grid([][]string{
		{"계정", tool.Baro_user_link(db, user, config.IP) + ` <a href="/baro/admin/user/view/` + tool.Url_parser(user) + tool.Baro_t(`">(사용자 관리)</a>`)},
		{tool.Baro_t("관심 분야"), baro_admin_esc(tool.Baro_field_name(db, one[4]))},
		{"상태", baro_admin_founding_status_badge(one[2])},
		{tool.Baro_t("현재 번호"), baro_admin_esc(one[1])},
		{tool.Baro_t("지원 시각"), baro_admin_esc(one[5])},
	})

	info += tool.Baro_stat_row([][]string{
		{"기여 이벤트", tool.Baro_comma(contrib_num), "baro_contrib_event"},
		{tool.Baro_t("편집 수"), tool.Baro_comma(edit_num), "history"},
		{tool.Baro_t("초대 코드 사용"), tool.Baro_comma(invite_used), "baro_invite"},
	})

	if event_id == "" {
		info += tool.Baro_alert("warn", tool.Baro_t("첫 유효 기여가 확인되지 않았습니다. 창립 번호는 첫 유효 기여를 마친 사람에게 줍니다."))
	} else {
		info += tool.Baro_alert("ok", tool.Baro_t("첫 유효 기여 확인 : ")+baro_admin_esc(event_name)+tool.Baro_t(" (기록 #")+baro_admin_esc(event_id)+")")
	}

	info += tool.Baro_section(tool.Baro_t("의심 신호"), "<p>"+risk_html+"</p>")
	if risky {
		info += tool.Baro_alert("danger", tool.Baro_t("중복 · 봇 계정 신호가 있습니다. 발급 전에 사용자 관리 화면에서 확인하세요."))
	}

	confirm_form := tool.Baro_hidden("do", "confirm")
	confirm_form += tool.Baro_hidden("user", user)
	confirm_form += tool.Baro_input("reason", tool.Baro_t("발급 사유"), "", tool.Baro_t("확인한 내용"), "")
	confirm_form += tool.Baro_checkbox("force", tool.Baro_t("기여 확인 없이 발급"), "on", false, tool.Baro_t("예외 발급은 사유를 반드시 남기세요."))
	confirm_form += tool.Baro_submit(tool.Baro_t("번호 발급 · 배지 부여"), "primary")

	revoke_form := tool.Baro_hidden("do", "revoke")
	revoke_form += tool.Baro_hidden("user", user)
	revoke_form += tool.Baro_input("reason", tool.Baro_t("회수 사유 (필수)"), "", tool.Baro_t("왜 회수하는지"), "")
	revoke_form += tool.Baro_submit(tool.Baro_t("번호 회수 · 배지 삭제"), "danger")

	hold_form := tool.Baro_hidden("do", "hold")
	hold_form += tool.Baro_hidden("user", user)
	hold_form += tool.Baro_input("reason", tool.Baro_t("보류 사유"), "", "", "")
	hold_form += tool.Baro_submit(tool.Baro_t("심사 보류"), "")

	note_form := tool.Baro_hidden("do", "note")
	note_form += tool.Baro_hidden("user", user)
	note_form += tool.Baro_textarea("note", "심사 메모", one[6], 4, tool.Baro_t("확인한 사항"), tool.Baro_t("내부 전용입니다."))
	note_form += tool.Baro_submit(tool.Baro_t("메모 저장"), "")

	body := info
	body += tool.Baro_grid([]string{
		tool.Baro_section(tool.Baro_t("번호 발급"), tool.Baro_form(action, confirm_form)),
		tool.Baro_section(tool.Baro_t("번호 회수"), tool.Baro_form(action, revoke_form)),
		tool.Baro_section(tool.Baro_t("보류"), tool.Baro_form(action, hold_form)),
		tool.Baro_section("심사 메모", tool.Baro_form(action, note_form)),
	}, 2)

	return tool.Baro_card_ex(tool.Baro_t("지원자 심사 : ")+user, tool.Baro_t("번호는 첫 유효 기여를 마친 사람에게 순서대로 발급합니다."), body, "")
}

// baro_admin_founding_field_tab : 분야별 현황
func baro_admin_founding_field_tab(db *sql.DB) string {
	rows := [][]string{}

	for _, field := range tool.Baro_field_list(db) {
		apply := baro_admin_num(db, "select count(*) from baro_founder where field = ?", field.Key)
		confirm := baro_admin_num(db, "select count(*) from baro_founder where field = ? and status = 'confirmed'", field.Key)
		interest := baro_admin_num(db, "select count(*) from baro_interest where field = ?", field.Key)

		recruit := tool.Baro_badge(tool.Baro_t("모집 마감"), "muted")
		if field.RecruitOpen {
			recruit = tool.Baro_badge(tool.Baro_t("모집 중"), "ok")
		}

		rows = append(rows, []string{
			baro_admin_esc(field.Name),
			recruit,
			baro_admin_esc(tool.Baro_comma(field.Recruit)),
			baro_admin_esc(tool.Baro_comma(apply)),
			baro_admin_esc(tool.Baro_comma(confirm)),
			baro_admin_esc(tool.Baro_comma(interest)),
			tool.Baro_progress(confirm, field.Recruit, ""),
		})
	}

	return tool.Baro_card_ex(
		tool.Baro_t("분야별 창립단 현황"),
		tool.Baro_t("baro_field · baro_founder · baro_interest 표를 직접 읽습니다."),
		tool.Baro_table(
			[]string{"분야", tool.Baro_t("모집"), tool.Baro_t("모집 정원"), tool.Baro_t("지원"), tool.Baro_t("확정"), tool.Baro_t("관심 등록"), tool.Baro_t("충원률")},
			rows,
			tool.Baro_t("분야 정의가 없습니다."),
		),
		tool.Baro_button("/baro/field", tool.Baro_t("분야 허브 보기"), ""),
	)
}

// baro_admin_founding_invite_tab : 초대 코드 현황
func baro_admin_founding_invite_tab(db *sql.DB, config tool.Config, query url.Values) string {
	page := baro_admin_page_num(query)
	size := tool.Baro_page_size
	status := baro_admin_get(query, "status")

	sql_text := "select id, code, owner, target, date, used_date, status from baro_invite"
	args := []any{}

	if status != "" {
		sql_text += " where status = ?"
		args = append(args, status)
	}

	sql_text += " order by id + 0 desc limit ?, ?"
	args = append(args, (page-1)*size, size)

	rows := [][]string{}
	for _, one := range baro_admin_rows(db, 7, sql_text, args...) {
		// 초대 코드 자체는 개인 자산이므로 앞부분만 보여 준다
		code := one[1]
		if tool.Get_len(code) > 4 {
			code = tool.Get_slice(code, 0, 4) + "****"
		}

		target := tool.Baro_t(`<span class="bw_muted">미사용</span>`)
		if one[3] != "" {
			target = tool.Baro_user_link(db, one[3], config.IP)
		}

		rows = append(rows, []string{
			`#` + baro_admin_esc(one[0]),
			baro_admin_esc(code),
			tool.Baro_user_link(db, one[2], config.IP),
			target,
			tool.Baro_time_cell(one[4]),
			tool.Baro_time_cell(one[5]),
			baro_admin_status_badge(one[6]),
		})
	}

	total := baro_admin_num(db, "select count(*) from baro_invite")
	used := baro_admin_num(db, "select count(*) from baro_invite where status = 'used'")
	open_num := baro_admin_num(db, "select count(*) from baro_invite where status = 'open'")

	body := tool.Baro_stat_row([][]string{
		{tool.Baro_t("발급된 코드"), tool.Baro_comma(total), "baro_invite"},
		{tool.Baro_t("사용됨"), tool.Baro_comma(used), tool.Baro_pct_text(used, total)},
		{tool.Baro_t("미사용"), tool.Baro_comma(open_num), ""},
		{"1인 발급 한도", tool.Baro_comma(tool.Baro_get_int(db, "invite_limit", 10)), "baro_set.invite_limit"},
	})

	body += tool.Baro_tabs([][]string{
		{"/baro/admin/founding?tab=invite", tool.Baro_t("전체")},
		{"/baro/admin/founding?tab=invite&status=open", tool.Baro_t("미사용")},
		{"/baro/admin/founding?tab=invite&status=used", tool.Baro_t("사용됨")},
	}, "/baro/admin/founding?tab=invite"+func() string {
		if status == "" {
			return ""
		}

		return "&status=" + status
	}())

	table := tool.Baro_table(
		[]string{"번호", tool.Baro_t("코드"), tool.Baro_t("발급자"), tool.Baro_t("사용자"), tool.Baro_t("발급 시각"), tool.Baro_t("사용 시각"), "상태"},
		rows,
		tool.Baro_t("초대 코드가 없습니다."),
	)

	if len(rows) == 0 {
		table += tool.Baro_empty(tool.Baro_t("이 조건의 초대 코드가 없습니다."))
	}

	paging_url := "/baro/admin/founding?tab=invite"
	if status != "" {
		paging_url += "&status=" + tool.Url_parser(status)
	}

	table += tool.Baro_paging(page, len(rows), size, paging_url+"&page={}")

	body += table
	body += tool.Baro_note(tool.Baro_t("코드 전체 값은 발급자 본인의 것이므로 앞 네 글자만 보여 줍니다. (0장 원칙 5)"))

	return tool.Baro_card_ex(tool.Baro_t("초대 코드 현황"), tool.Baro_t("누가 누구를 데려왔는지 확인합니다."), body, "")
}

// baro_admin_founding_risk_tab : 중복 · 봇 계정 검토
func baro_admin_founding_risk_tab(db *sql.DB, config tool.Config, query url.Values) string {
	_ = query

	// 같은 IP 를 쓰는 무리 : 가입 IP 기록이 없으면 접속 기록(ua_d)으로 본다
	ip_group := baro_admin_rows(
		db, 2,
		"select data, count(distinct id) from user_set where name = 'register_ip' and data != '' group by data having count(distinct id) > 1 order by count(distinct id) desc limit 30",
	)

	group_source := "가입 IP"
	if len(ip_group) == 0 {
		ip_group = baro_admin_rows(
			db, 2,
			"select ip, count(distinct name) from ua_d where ip != '' group by ip having count(distinct name) > 1 order by count(distinct name) desc limit 30",
		)
		group_source = tool.Baro_t("접속 기록(ua_d)")
	}

	group_rows := [][]string{}
	for _, one := range ip_group {
		member := baro_admin_rows(
			db, 1,
			"select distinct id from user_set where name = 'register_ip' and data = ? order by id limit 20",
			one[0],
		)
		member = append(member, baro_admin_rows(
			db, 1,
			"select distinct name from ua_d where ip = ? limit 20",
			one[0],
		)...)

		seen := map[string]bool{}
		users := []string{}
		founder_num := 0

		for _, sub := range member {
			if sub[0] == "" || seen[sub[0]] {
				continue
			}

			seen[sub[0]] = true
			users = append(users, tool.Baro_user_link(db, sub[0], config.IP))

			if baro_admin_one(db, "select user from baro_founder where user = ?", sub[0]) != "" {
				founder_num++
			}
		}

		kind := "warn"
		if tool.Str_to_int(one[1]) >= 4 {
			kind = "danger"
		}

		group_rows = append(group_rows, []string{
			tool.Baro_badge(tool.Baro_mask_ip(one[0]), kind),
			baro_admin_esc(one[1]) + tool.Baro_t("개"),
			baro_admin_esc(tool.Baro_itoa(founder_num)) + tool.Baro_t("명"),
			strings.Join(users, ", "),
		})
	}

	body := tool.Baro_alert("warn", tool.Baro_t("IP 는 가려서 표시합니다. 정확한 값은 사용자 관리 화면에서 개인정보 권한으로만 볼 수 있습니다. (판단 근거 : ")+baro_admin_esc(group_source)+")")
	body += tool.Baro_table(
		[]string{tool.Baro_t("IP (가림)"), tool.Baro_t("계정 수"), tool.Baro_t("창립 지원"), "계정"},
		group_rows,
		tool.Baro_t("같은 IP 로 가입한 계정 무리가 없습니다."),
	)

	// 가입 직후 기계적 편집 패턴
	bot_rows := [][]string{}
	for _, one := range baro_admin_rows(
		db, 3,
		"select user, status, apply_date from baro_founder order by apply_date desc limit 100",
	) {
		risk_html, risky := baro_admin_founding_bot_risk(db, one[0])
		if !risky {
			continue
		}

		bot_rows = append(bot_rows, []string{
			tool.Baro_user_link(db, one[0], config.IP),
			baro_admin_founding_status_badge(one[1]),
			risk_html,
			tool.Baro_button("/baro/admin/founding?tab=apply&user="+tool.Url_parser(one[0]), "심사", "primary"),
		})
	}

	body += tool.Baro_section(tool.Baro_t("가입 직후 기계적 편집 패턴"), tool.Baro_table(
		[]string{tool.Baro_t("지원자"), "상태", tool.Baro_t("의심 신호"), ""},
		bot_rows,
		tool.Baro_t("의심 신호가 잡힌 지원자가 없습니다."),
	))

	if len(group_rows) == 0 && len(bot_rows) == 0 {
		body += tool.Baro_empty(tool.Baro_t("현재 검토할 중복 · 봇 계정 후보가 없습니다."))
	}

	body += tool.Baro_note(tool.Baro_t("신호가 있다고 곧바로 반려하지 않습니다. 가족 · 학교 · 회사 공용 회선일 수 있으므로 사람이 확인합니다."))

	return tool.Baro_card_ex(tool.Baro_t("중복 · 봇 계정 검토"), tool.Baro_t("같은 가입 IP 무리와 가입 직후 편집 패턴을 봅니다."), body, "")
}
