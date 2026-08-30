package route

// ============================================================
//  10-6 대량 조치 안전장치 (/baro/mass)  ★ 바로위키의 시그니처 기능
//
//  관리자 한 명이 버튼을 잘못 눌러 문서 4,000개를 증발시키는 구조를 없애는 것이 목적이다.
//
//  절차 :
//    1) 영향받는 대상 수를 미리 크게 표시
//    2) 처리 대상 목록 공개 + 목록 내려받기
//    3) 예상 영향 미리보기 (각 대상이 실제로 존재하는지, 지금 상태가 무엇인지)
//    4) 서로 다른 운영자 2인 승인 (만든 사람은 자기 작업을 승인할 수 없다)
//    5) 유예 시간 (기본 24시간)
//    6) 실행 전 복구 스냅샷 생성
//    7) 실행 후 자동 공지
//    8) 원클릭 일괄 복구
//
//  긴급 법적 사안만 유예를 건너뛸 수 있고, 그 사실은 공개 로그에 남는다.
// ============================================================

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ------------------------------------------------------------
//  공통
// ------------------------------------------------------------

// baro_mod_mass_detail : 대량 작업의 부가 설정 (detail 컬럼에 JSON 으로 담는다)
func baro_mod_mass_detail(row baro_mod_mass_row) map[string]string {
	out := tool.Baro_json_dec_str(row.Detail)
	if out == nil {
		out = map[string]string{}
	}

	return out
}

// baro_mod_mass_is_user_kind : 대상이 문서가 아니라 사용자인 작업인지
func baro_mod_mass_is_user_kind(kind string) bool {
	return kind == "block"
}

// baro_mod_mass_step : 지금 어느 단계인지 (화면 위에 크게 보여 준다)
func baro_mod_mass_step(row baro_mod_mass_row) string {
	steps := [][]string{
		{"draft", tool.Baro_t("1. 작성")},
		{"approved", tool.Baro_t("2. 승인 · 유예")},
		{"done", tool.Baro_t("3. 실행")},
		{"undone", tool.Baro_t("4. 일괄 복구")},
	}

	current := row.Status
	if current == "rejected" {
		return tool.Baro_badge(tool.Baro_t("취소됨"), "muted")
	}

	html := `<div class="bw_pills">`
	for _, step := range steps {
		kind := "muted"
		if step[0] == current {
			kind = "ok"
		}

		html += tool.Baro_badge(step[1], kind) + " "
	}

	return html + `</div>`
}

// baro_mod_mass_grace_left : 유예가 끝났는지와 남은 시간
func baro_mod_mass_grace_left(row baro_mod_mass_row) (bool, string) {
	if row.Urgent == "on" {
		return true, tool.Baro_t("긴급 지정 — 유예 없음")
	}

	if strings.TrimSpace(row.GraceUntil) == "" {
		return false, tool.Baro_t("승인 전")
	}

	if tool.Baro_is_past(row.GraceUntil) {
		return true, tool.Baro_t("유예 종료 (") + row.GraceUntil + ")"
	}

	return false, row.GraceUntil + tool.Baro_t(" 까지 (") + tool.Baro_duration_left(row.GraceUntil) + ")"
}

// baro_mod_mass_can_execute : 실행 가능 상태인지와 그 이유
func baro_mod_mass_can_execute(db *sql.DB, row baro_mod_mass_row) (bool, string) {
	if row.Status == "done" {
		return false, tool.Baro_t("이미 실행된 작업입니다.")
	}

	if row.Status == "undone" {
		return false, tool.Baro_t("이미 일괄 복구된 작업입니다.")
	}

	if row.Status == "rejected" {
		return false, tool.Baro_t("취소된 작업입니다.")
	}

	threshold := tool.Baro_mass_threshold(db)
	num := tool.Str_to_int(row.Num)

	if num >= threshold {
		if row.Status != "approved" {
			return false, tool.Baro_t("운영자 2인의 승인이 필요합니다. (현재 ") + tool.Baro_itoa(len(baro_mod_mass_approvers(row))) + tool.Baro_t("명)")
		}

		if ok, left := baro_mod_mass_grace_left(row); !ok {
			return false, tool.Baro_t("유예 시간이 남아 있습니다. ") + left
		}
	}

	return true, ""
}

// ------------------------------------------------------------
//  대량 작업 목록
// ------------------------------------------------------------

// View_baro_mod_mass_list : 대량 작업 목록 (/baro/mass)
func View_baro_mod_mass_list(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	page := baro_mod_page_num(arg)
	size := tool.Baro_page_size

	status := baro_mod_val(query, "status")

	where := " where 1 = 1"
	params := []any{}

	if status != "" && status != "all" {
		where += " and status = ?"
		params = append(params, status)
	}

	params = append(params, (page-1)*size, size)

	rows := tool.Query_DB(
		db,
		"select id, kind, actor, num, reason, status, approver, created, execute_date, grace_until, urgent, undo_status from baro_mass_op"+where+" order by id + 0 desc limit ?, ?",
		params...,
	)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0

	for rows.Next() {
		id, kind, actor, num, reason, m_status, approver, created, execute_date, grace_until, urgent, undo_status := "", "", "", "", "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &kind, &actor, &num, &reason, &m_status, &approver, &created, &execute_date, &grace_until, &urgent, &undo_status); err != nil {
			continue
		}

		count++

		urgent_html := ""
		if urgent == "on" {
			urgent_html = tool.Baro_badge(tool.Baro_t("긴급"), "danger") + " "
		}

		approver_count := 0
		for _, name := range strings.Split(approver, ",") {
			if strings.TrimSpace(name) != "" {
				approver_count++
			}
		}

		grace_html := `<span class="bw_muted">-</span>`
		if grace_until != "" {
			if tool.Baro_is_past(grace_until) {
				grace_html = tool.Baro_badge(tool.Baro_t("유예 종료"), "ok")
			} else {
				grace_html = tool.Baro_badge(tool.Baro_duration_left(grace_until), "warn")
			}
		}

		undo_html := `<span class="bw_muted">-</span>`
		if undo_status != "" {
			undo_html = tool.Baro_badge(tool.Baro_t("복구됨"), "ok")
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/mass/view/` + tool.Url_parser(id) + `">` + baro_mod_esc(id) + `</a>`,
			urgent_html + baro_mod_esc(tool.Baro_mass_kind_name(kind)),
			`<b>` + baro_mod_esc(tool.Baro_comma(tool.Str_to_int(num))) + `</b>`,
			baro_mod_esc(tool.Baro_trunc(tool.Baro_one_line(reason), 100)),
			baro_mod_status_badge(m_status),
			baro_mod_esc(tool.Baro_itoa(approver_count) + " / 2"),
			grace_html,
			undo_html,
			tool.Baro_user_link(db, actor, config.IP),
			tool.Baro_time_cell(baro_mod_or(execute_date, created)),
		})
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("대량 작업")}})

	body += tool.Baro_hero(
		tool.Baro_t("대량 작업 안전장치"),
		tool.Baro_t("관리자 한 명의 실수로 문서가 한꺼번에 사라지는 일을 막습니다."),
		tool.Baro_note(tool.Baro_t("대상 수 표시 → 목록 공개 → 예상 영향 미리보기 → 2인 승인 → 복구 스냅샷 → 유예 → 실행 → 자동 공지 → 원클릭 일괄 복구")),
	)

	threshold := tool.Baro_mass_threshold(db)
	grace := tool.Baro_mass_grace_hours(db)

	draft_count, approved_count, done_count, undone_count := 0, 0, 0, 0
	tool.QueryRow_DB(db, "select count(*) from baro_mass_op where status = 'draft'", []any{&draft_count})
	tool.QueryRow_DB(db, "select count(*) from baro_mass_op where status = 'approved'", []any{&approved_count})
	tool.QueryRow_DB(db, "select count(*) from baro_mass_op where status = 'done'", []any{&done_count})
	tool.QueryRow_DB(db, "select count(*) from baro_mass_op where status = 'undone'", []any{&undone_count})

	body += tool.Baro_card(tool.Baro_t("현황"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("승인 대기"), tool.Baro_comma(draft_count), tool.Baro_t("2인 승인이 필요합니다")},
		{tool.Baro_t("유예 중"), tool.Baro_comma(approved_count), tool.Baro_t("승인 완료, 실행 대기")},
		{tool.Baro_t("실행됨"), tool.Baro_comma(done_count), ""},
		{tool.Baro_t("일괄 복구됨"), tool.Baro_comma(undone_count), ""},
	}))

	body += tool.Baro_card(tool.Baro_t("지금 설정"), tool.Baro_kv_grid([][]string{
		{tool.Baro_t("승인 절차가 강제되는 기준"), `<b>` + baro_mod_esc(tool.Baro_itoa(threshold)) + tool.Baro_t(`개 이상</b>`)},
		{tool.Baro_t("기본 유예 시간"), `<b>` + baro_mod_esc(tool.Baro_itoa(grace)) + tool.Baro_t(`시간</b>`)},
		{tool.Baro_t("필요 승인자 수"), tool.Baro_t(`<b>서로 다른 운영자 2명</b>`)},
	}))

	filter_form := `<form class="bw_form" method="get" action="/baro/mass">`
	filter_form += tool.Baro_select("status", "상태", [][]string{
		{"", tool.Baro_t("전체")},
		{"draft", tool.Baro_t("승인 대기")},
		{"approved", tool.Baro_t("유예 중")},
		{"done", tool.Baro_t("실행됨")},
		{"undone", tool.Baro_t("일괄 복구됨")},
		{"rejected", tool.Baro_t("취소됨")},
	}, status, "")
	filter_form += tool.Baro_submit(tool.Baro_t("적용"), "")
	filter_form += `</form>`

	body += tool.Baro_card("걸러 보기", filter_form)

	list_body := tool.Baro_table(
		[]string{"번호", "종류", tool.Baro_t("대상 수"), "사유", "상태", "승인", tool.Baro_t("유예"), "복구", tool.Baro_t("신청자"), tool.Baro_t("최근")},
		table_rows,
		"",
	)
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("조건에 맞는 대량 작업이 없습니다."))
	}

	list_body += tool.Baro_paging(page, count, size, "/baro/mass/list/{}?status="+tool.Url_parser(status))

	body += tool.Baro_card(tool.Baro_t("대량 작업 목록"), list_body)

	if tool.Baro_can(db, config.IP, "mass") {
		body += tool.Baro_card("", tool.Baro_button("/baro/mass/new", tool.Baro_t("대량 작업 만들기"), "primary"))
	}

	body += tool.Baro_card(tool.Baro_t("왜 이렇게 번거롭게 만들었나요"),
		tool.Baro_list([]string{
			tool.Baro_t("위키에서 가장 크게 잘못되는 일은 악의가 아니라 <b>실수</b>로 일어납니다."),
			tool.Baro_t("한 사람의 판단으로 수천 개 문서가 사라지면, 그 뒤에 아무리 사과해도 원래대로 돌아오지 않습니다."),
			tool.Baro_t("그래서 대량 작업은 <b>느리게</b> 만들었습니다. 목록을 공개하고, 두 사람이 확인하고, 하루를 기다립니다."),
			tool.Baro_t("실행 전에는 반드시 스냅샷을 남깁니다. 잘못되면 버튼 하나로 전부 되돌립니다."),
			tool.Baro_t("긴급 법적 사안만 유예를 건너뛸 수 있고, 그 사실은 공개 로그에 그대로 남습니다."),
		}),
	)

	return baro_mod_shell(db, config, tool.Baro_t("대량 작업"), "/baro/mass", body)
}

// ------------------------------------------------------------
//  대량 작업 만들기
// ------------------------------------------------------------

// View_baro_mod_mass_new : 대량 작업 만들기 (/baro/mass/new)
func View_baro_mod_mass_new(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = arg
	_ = query

	if !tool.Baro_can(db, config.IP, "mass") {
		return tool.Baro_deny(db, config, tool.Baro_t("대량 작업은 제재 담당 또는 시스템 관리자 역할이 있어야 만들 수 있습니다."))
	}

	if page, stop := baro_mod_read_only_block(db, config, "/baro/mass"); stop {
		return page
	}

	if values != nil {
		return baro_mod_mass_new_post(db, config, values)
	}

	threshold := tool.Baro_mass_threshold(db)
	grace := tool.Baro_mass_grace_hours(db)

	body := tool.Baro_crumb([][]string{{"/baro/mass", tool.Baro_t("대량 작업")}, {"", tool.Baro_t("새 작업")}})

	body += tool.Baro_alert("warn",
		tool.Baro_t("<b>대량 작업은 되돌리기 어려운 일입니다.</b> 대상이 ")+baro_mod_esc(tool.Baro_itoa(threshold))+
			tool.Baro_t("개 이상이면 목록 공개 · 2인 승인 · 스냅샷 · ")+baro_mod_esc(tool.Baro_itoa(grace))+
			tool.Baro_t("시간 유예를 반드시 거칩니다. 이 절차는 건너뛸 수 없습니다."))

	form := tool.Baro_select("kind", tool.Baro_t("작업 종류"), tool.Baro_mass_kind_list(), "quarantine", tool.Baro_t("삭제를 골라도 실제로는 임시 격리로 처리됩니다. (고정 원칙 2)"))
	form += tool.Baro_textarea("targets", tool.Baro_t("대상 목록"), "", 12, tool.Baro_t("한 줄에 하나씩 적습니다."), tool.Baro_t("문서 이름 또는 사용자 이름. 빈 줄과 중복은 자동으로 걸러집니다."))
	form += tool.Baro_textarea("reason", "사유", "", 5, tool.Baro_t("왜 이 작업이 필요한지 적습니다."), tool.Baro_t("필수 항목입니다. 승인자와 모든 이용자가 이 사유를 봅니다."))
	form += tool.Baro_input("case_id", tool.Baro_t("연결 사건"), "", "BW-2026-0001", tool.Baro_t("사건과 연결하면 이의제기가 가능해집니다."))

	form += tool.Baro_section(tool.Baro_t("종류별 추가 설정"), ""+
		tool.Baro_select("sanction_kind", tool.Baro_t("대량 차단: 제재 종류"), baro_mod_sanction_options(), "short_block", tool.Baro_t("'대량 차단' 을 고른 경우에만 씁니다."))+
		tool.Baro_input_type("sanction_days", tool.Baro_t("대량 차단: 기간(일)"), "3", "number", tool.Baro_t("0 이면 무기한입니다."))+
		tool.Baro_input("acl", tool.Baro_t("대량 보호: 편집 권한"), "admin", "admin / user / 50_edit ...", tool.Baro_t("'대량 보호' 를 고른 경우에만 씁니다."))+
		tool.Baro_input("from", tool.Baro_t("대량 분류 변경: 바꿀 문자열"), "", tool.Baro_t("[[분류:옛이름]]"), tool.Baro_t("'대량 분류 변경' 을 고른 경우에만 씁니다."))+
		tool.Baro_input("to", tool.Baro_t("대량 분류 변경: 새 문자열"), "", tool.Baro_t("[[분류:새이름]]"), tool.Baro_t("비우면 해당 분류를 지웁니다.")),
	)

	form += tool.Baro_checkbox("urgent", tool.Baro_t("긴급 (유예 없이 실행)"), "on", false, tool.Baro_t("긴급 법적 사안만 해당합니다. 남용하면 감사 기록에 그대로 남습니다."))
	form += tool.Baro_textarea("urgent_reason", tool.Baro_t("긴급 사유"), "", 3, tool.Baro_t("어떤 법적 사안인지 구체적으로"), tool.Baro_t("긴급을 켰다면 반드시 적어야 합니다."))
	form += tool.Baro_submit(tool.Baro_t("작업 만들기 (아직 실행되지 않습니다)"), "primary")

	body += tool.Baro_card(tool.Baro_t("새 대량 작업"), tool.Baro_form("/baro/mass/new", form))

	body += tool.Baro_alert("danger",
		tool.Baro_t("<b>긴급 지정은 긴급 법적 사안만 해당합니다.</b> 법원 명령, 개인정보 유출, 명백한 저작권 침해처럼 ")+
			tool.Baro_t("기다리는 것 자체가 피해를 키우는 경우에만 사용하세요. 그 밖의 사유로 긴급을 쓰면 감사 대상이 됩니다."))

	body += tool.Baro_card(tool.Baro_t("작업 종류별로 실제로 일어나는 일"),
		tool.Baro_table(
			[]string{"종류", tool.Baro_t("실제 동작"), tool.Baro_t("되돌릴 수 있는가")},
			[][]string{
				{tool.Baro_t("대량 삭제"), tool.Baro_t("문서를 지우지 않고 <b>임시 격리</b>합니다. 원본은 통째로 보존됩니다."), tool.Baro_badge(tool.Baro_t("가능"), "ok")},
				{tool.Baro_t("대량 격리"), tool.Baro_t("문서 상태를 격리로 바꾸고 원본을 보존합니다. 본문은 그대로 둡니다."), tool.Baro_badge(tool.Baro_t("가능"), "ok")},
				{tool.Baro_t("대량 되돌리기"), tool.Baro_t("직전 판으로 되돌립니다. 되돌리기 전 내용은 스냅샷에 남습니다."), tool.Baro_badge(tool.Baro_t("가능"), "ok")},
				{tool.Baro_t("대량 차단"), tool.Baro_t("사람에게만 제재를 겁니다. 문서는 건드리지 않습니다."), tool.Baro_badge(tool.Baro_t("가능"), "ok")},
				{tool.Baro_t("대량 보호"), tool.Baro_t("문서 편집 권한(ACL)을 바꿉니다. 기존 값은 스냅샷에 남습니다."), tool.Baro_badge(tool.Baro_t("가능"), "ok")},
				{tool.Baro_t("대량 분류 변경"), tool.Baro_t("본문의 특정 문자열을 바꿉니다. 원본은 스냅샷에 남습니다."), tool.Baro_badge(tool.Baro_t("가능"), "ok")},
			},
			"",
		),
	)

	return baro_mod_shell(db, config, tool.Baro_t("대량 작업 만들기"), "/baro/mass", body)
}

// baro_mod_mass_new_post : 대량 작업 생성 처리
func baro_mod_mass_new_post(db *sql.DB, config tool.Config, values url.Values) string {
	kind := baro_mod_val(values, "kind")
	targets := baro_mod_lines(baro_mod_raw(values, "targets"))
	reason := tool.Baro_trunc(baro_mod_raw(values, "reason"), 4000)
	urgent := baro_mod_on(values, "urgent")
	urgent_reason := tool.Baro_trunc(baro_mod_raw(values, "urgent_reason"), 2000)

	fail := func(message string) string {
		body := tool.Baro_alert("danger", baro_mod_esc(message))
		body += tool.Baro_card("", tool.Baro_button("/baro/mass/new", "돌아가기", "primary"))

		return baro_mod_shell(db, config, tool.Baro_t("대량 작업 만들기"), "/baro/mass", body)
	}

	if tool.Baro_mass_kind_name(kind) == kind {
		return fail(tool.Baro_t("작업 종류를 올바르게 골라 주세요."))
	}

	if len(targets) == 0 {
		return fail(tool.Baro_t("대상 목록이 비어 있습니다. 한 줄에 하나씩 적어 주세요."))
	}

	if strings.TrimSpace(reason) == "" {
		return fail(tool.Baro_t("사유를 반드시 적어 주세요. 사유 없는 대량 작업은 승인되지 않습니다."))
	}

	if urgent && strings.TrimSpace(urgent_reason) == "" {
		return fail(tool.Baro_t("긴급을 선택했다면 긴급 사유를 반드시 적어야 합니다. 긴급은 법적 사안에만 씁니다."))
	}

	if len(targets) > 5000 {
		return fail(tool.Baro_t("한 번에 처리할 수 있는 대상은 5,000개까지입니다. 나누어 신청해 주세요."))
	}

	case_id := ""
	if case_row, ok := baro_mod_case_get(db, baro_mod_val(values, "case_id")); ok {
		case_id = case_row.ID
	}

	detail := map[string]string{
		"urgent_reason": urgent_reason,
		"sanction_kind": baro_mod_or(baro_mod_val(values, "sanction_kind"), "short_block"),
		"sanction_days": baro_mod_or(baro_mod_val(values, "sanction_days"), "3"),
		"acl":           baro_mod_or(baro_mod_val(values, "acl"), "admin"),
		"from":          baro_mod_raw(values, "from"),
		"to":            baro_mod_raw(values, "to"),
		"case_id":       case_id,
	}

	id := tool.Baro_next_id(db, "baro_mass_op")
	now := tool.Baro_now()
	num := len(targets)
	threshold := tool.Baro_mass_threshold(db)

	// 만들면 언제나 draft 로 저장한다. 만드는 것과 실행하는 것은 다른 일이다.
	status := "draft"
	grace_until := ""

	// 기준 미만은 2인 승인과 유예를 면제하되, 스냅샷과 일괄 복구는 그대로 적용된다
	if num < threshold {
		detail["small"] = "on"
	}

	urgent_value := ""
	if urgent {
		urgent_value = "on"
	}

	tool.Exec_DB(
		db,
		"insert into baro_mass_op (id, kind, actor, target_list, num, reason, status, approver, created, approved_date, execute_date, grace_until, urgent, undo_status, detail) values (?, ?, ?, ?, ?, ?, ?, '', ?, '', '', ?, ?, '', ?)",
		id, kind, config.IP, strings.Join(targets, "\n"), tool.Baro_itoa(num), reason, status, now, grace_until, urgent_value, tool.Baro_json_enc(detail),
	)

	tool.Baro_public_log(db, "mass_create", config.IP, tool.Baro_mass_kind_name(kind),
		tool.Baro_trunc(tool.Baro_one_line(reason), 200),
		"대상 "+tool.Baro_comma(num)+"개"+baro_mod_mass_urgent_mark(urgent))

	tool.Baro_audit(db, config, "mass_create", tool.Baro_mass_kind_name(kind), "",
		"대상 "+tool.Baro_itoa(num)+"개 / 긴급 "+urgent_value, reason)

	return tool.Get_redirect("/baro/mass/view/" + tool.Url_parser(id))
}

// baro_mod_mass_urgent_mark : 공개 로그에 붙일 긴급 표시
func baro_mod_mass_urgent_mark(urgent bool) string {
	if urgent {
		return tool.Baro_t(" [긴급]")
	}

	return ""
}

// ------------------------------------------------------------
//  대량 작업 상세
// ------------------------------------------------------------

// View_baro_mod_mass_view : 대량 작업 상세 (/baro/mass/view/<id>)
func View_baro_mod_mass_view(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query

	row, ok := baro_mod_mass_get(db, arg)
	if !ok {
		return baro_mod_missing(db, config, "/baro/mass", tool.Baro_t("대량 작업"), "/baro/mass", tool.Baro_t("대량 작업 목록으로"))
	}

	if values != nil {
		if !tool.Baro_can(db, config.IP, "mass") {
			return tool.Baro_deny(db, config, tool.Baro_t("대량 작업을 수정할 권한이 없습니다."))
		}

		return baro_mod_mass_view_post(db, config, row, values)
	}

	targets := baro_mod_mass_targets(row)
	detail := baro_mod_mass_detail(row)
	approvers := baro_mod_mass_approvers(row)
	num := tool.Str_to_int(row.Num)
	threshold := tool.Baro_mass_threshold(db)

	body := tool.Baro_crumb([][]string{{"/baro/mass", tool.Baro_t("대량 작업")}, {"", "작업 " + row.ID}})

	body += baro_mod_mass_step(row)

	if row.Urgent == "on" {
		body += tool.Baro_alert("danger",
			tool.Baro_t("<b>긴급 작업입니다.</b> 유예 없이 실행할 수 있습니다. 긴급 사유: ")+
				baro_mod_esc(baro_mod_or(detail["urgent_reason"], tool.Baro_t("(기록되지 않음)")))+
				tool.Baro_t("<br>긴급 지정은 긴급 법적 사안만 해당합니다. 이 사실은 공개 로그에 그대로 남습니다."))
	}

	// 1) 영향받는 대상 수를 크게 표시
	body += tool.Baro_card(tool.Baro_t("이 작업이 영향을 주는 대상"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("영향받는 대상"), tool.Baro_comma(num), tool.Baro_t("이만큼이 한꺼번에 바뀝니다")},
		{tool.Baro_t("작업 종류"), tool.Baro_mass_kind_name(row.Kind), ""},
		{"승인", tool.Baro_itoa(len(approvers)) + " / 2", tool.Baro_t("서로 다른 운영자 2명")},
		{"상태", baro_mod_status_name(row.Status), ""},
	}))

	if num >= threshold {
		body += tool.Baro_alert("warn",
			tool.Baro_t("대상이 ")+baro_mod_esc(tool.Baro_itoa(threshold))+tool.Baro_t("개 이상이라 승인 절차가 강제됩니다. ")+
				tool.Baro_t("목록 공개 · 2인 승인 · 스냅샷 · 유예를 모두 거쳐야 실행할 수 있습니다."))
	} else {
		body += tool.Baro_alert("info",
			tool.Baro_t("대상이 ")+baro_mod_esc(tool.Baro_itoa(threshold))+tool.Baro_t("개 미만이라 2인 승인은 면제됩니다. ")+
				tool.Baro_t("그래도 실행 전 스냅샷과 일괄 복구는 그대로 적용됩니다."))
	}

	grace_ok, grace_text := baro_mod_mass_grace_left(row)
	grace_html := baro_mod_esc(grace_text)
	if !grace_ok && row.Status == "approved" {
		grace_html = tool.Baro_badge(tool.Baro_t("유예 중"), "warn") + " " + baro_mod_esc(grace_text)
	} else if grace_ok && row.Status == "approved" {
		grace_html = tool.Baro_badge(tool.Baro_t("실행 가능"), "ok") + " " + baro_mod_esc(grace_text)
	}

	approver_html := tool.Baro_t(`<span class="bw_muted">아직 없음</span>`)
	if len(approvers) > 0 {
		parts := []string{}
		for _, name := range approvers {
			parts = append(parts, tool.Baro_user_link(db, name, config.IP))
		}

		approver_html = strings.Join(parts, ", ")
	}

	body += tool.Baro_card(tool.Baro_t("작업 정보"), tool.Baro_kv_grid([][]string{
		{tool.Baro_t("작업 번호"), baro_mod_esc(row.ID)},
		{"종류", baro_mod_esc(tool.Baro_mass_kind_name(row.Kind))},
		{tool.Baro_t("신청자"), tool.Baro_user_link(db, row.Actor, config.IP)},
		{tool.Baro_t("신청 시각"), baro_mod_esc(row.Created)},
		{tool.Baro_t("승인자"), approver_html},
		{tool.Baro_t("승인 시각"), baro_mod_muted(row.ApprovedDate)},
		{tool.Baro_t("유예"), grace_html},
		{"실행 시각", baro_mod_muted(row.ExecuteDate)},
		{"일괄 복구", baro_mod_muted(baro_mod_or(row.UndoStatus, ""))},
		{tool.Baro_t("연결 사건"), baro_mod_case_link(db, detail["case_id"])},
	}))

	body += tool.Baro_card("사유", tool.Baro_code(row.Reason))

	// 종류별 설정
	setting_pairs := [][]string{}
	switch row.Kind {
	case "block":
		setting_pairs = append(setting_pairs,
			[]string{tool.Baro_t("제재 종류"), baro_mod_esc(tool.Baro_sanction_name(detail["sanction_kind"]))},
			[]string{"기간", baro_mod_esc(baro_mod_or(detail["sanction_days"], "0") + tool.Baro_t("일 (0 은 무기한)"))},
		)
	case "protect":
		setting_pairs = append(setting_pairs, []string{tool.Baro_t("편집 권한"), baro_mod_esc(baro_mod_or(detail["acl"], "admin"))})
	case "category":
		setting_pairs = append(setting_pairs,
			[]string{tool.Baro_t("바꿀 문자열"), tool.Baro_code(baro_mod_or(detail["from"], tool.Baro_t("(비어 있음)")))},
			[]string{tool.Baro_t("새 문자열"), tool.Baro_code(baro_mod_or(detail["to"], tool.Baro_t("(지움)")))},
		)
	}

	if len(setting_pairs) > 0 {
		body += tool.Baro_card(tool.Baro_t("종류별 설정"), tool.Baro_kv_grid(setting_pairs))
	}

	// 2) 처리 대상 목록 공개 + 내려받기
	list_html := tool.Baro_note(tool.Baro_t("이 목록은 누구나 볼 수 있고 내려받을 수 있습니다. 대량 작업의 대상은 숨기지 않습니다."))
	list_html += tool.Baro_button("/baro/mass/download/"+tool.Url_parser(row.ID), "대상 목록 내려받기 (텍스트)", "primary")
	list_html += tool.Baro_code(strings.Join(baro_mod_mass_preview_list(targets, 500), "\n"))

	body += tool.Baro_card(tool.Baro_t("처리 대상 목록 (")+tool.Baro_comma(num)+tool.Baro_t("개)"), list_html)

	// 3) 예상 영향 미리보기
	body += tool.Baro_card(tool.Baro_t("예상 영향 미리보기"), baro_mod_mass_preview_table(db, config, row, targets))

	// 승인 / 실행 / 복구 버튼
	body += baro_mod_mass_action_block(db, config, row)

	// 스냅샷
	if row.Status == "done" || row.Status == "undone" {
		body += tool.Baro_card(tool.Baro_t("복구 스냅샷"), baro_mod_mass_snapshot_table(db, row))
	}

	body += tool.Baro_card(tool.Baro_t("이 절차가 있는 이유"),
		tool.Baro_note(tool.Baro_t("관리자 한 명이 버튼을 잘못 눌러 문서 수천 개가 사라지는 구조를 없애기 위한 장치입니다. ")+
			tool.Baro_t("느린 것이 목적입니다. 되돌릴 수 없는 일을 빠르게 하지 않습니다.")),
	)

	return baro_mod_shell(db, config, tool.Baro_t("대량 작업 ")+row.ID, "/baro/mass", body)
}

// baro_mod_mass_preview_list : 목록을 화면에 보여 줄 만큼만 자른다
func baro_mod_mass_preview_list(targets []string, max int) []string {
	if len(targets) <= max {
		return targets
	}

	out := append([]string{}, targets[:max]...)
	out = append(out, "... 그리고 "+tool.Baro_itoa(len(targets)-max)+tool.Baro_t("개 더 (내려받기로 전체를 확인하세요)"))

	return out
}

// baro_mod_mass_preview_table : 각 대상이 실제로 존재하는지, 지금 상태가 무엇인지
func baro_mod_mass_preview_table(db *sql.DB, config tool.Config, row baro_mod_mass_row, targets []string) string {
	limit := 100
	table_rows := [][]string{}

	missing := 0
	quarantined := 0

	for index, target := range targets {
		if index >= limit {
			break
		}

		if baro_mod_mass_is_user_kind(row.Kind) {
			// 사용자 대상 : 지금 제재 상태를 보여 준다
			active := tool.Baro_sanction_active(db, target)
			state := tool.Baro_badge(tool.Baro_t("제재 없음"), "ok")
			if len(active) > 0 {
				state = tool.Baro_badge("제재 "+tool.Baro_itoa(len(active))+tool.Baro_t("건"), "danger")
			}

			edit_count := 0
			tool.QueryRow_DB(db, "select count(*) from history where ip = ?", []any{&edit_count}, target)

			exists := tool.Baro_badge(tool.Baro_t("로그인 사용자"), "info")
			if !tool.Baro_is_user(target) {
				exists = tool.Baro_badge("IP", "muted")
			}

			table_rows = append(table_rows, []string{
				tool.Baro_user_link(db, target, config.IP),
				exists,
				state,
				baro_mod_esc(tool.Baro_comma(edit_count) + tool.Baro_t("판")),
				tool.Baro_t(`<span class="bw_muted">문서는 건드리지 않습니다</span>`),
			})

			continue
		}

		data, exists := baro_mod_doc_data(db, target)

		exists_html := tool.Baro_badge(tool.Baro_t("있음"), "ok")
		if !exists {
			exists_html = tool.Baro_badge("없음", "danger")
			missing++
		}

		status := tool.Baro_doc_status_get(db, target)
		status_html := tool.Baro_status_badge(status)
		if tool.Baro_doc_quarantined(db, target) {
			status_html = tool.Baro_badge(tool.Baro_t("이미 격리 중"), "warn")
			quarantined++
		}

		editors := baro_mod_doc_editors(db, target)
		joint := tool.Baro_badge(tool.Baro_t("단독"), "info")
		if len(editors) > 1 {
			joint = tool.Baro_badge(tool.Baro_itoa(len(editors))+tool.Baro_t("명 공동"), "warn")
		}

		table_rows = append(table_rows, []string{
			tool.Baro_doc_link(target),
			exists_html,
			status_html,
			baro_mod_esc(tool.Baro_comma(tool.Get_len(data)) + tool.Baro_t("자")),
			joint,
		})
	}

	headers := []string{"문서", tool.Baro_t("존재"), tool.Baro_t("현재 상태"), "길이", tool.Baro_t("기여 형태")}
	if baro_mod_mass_is_user_kind(row.Kind) {
		headers = []string{tool.Baro_t("사용자"), "종류", tool.Baro_t("현재 제재"), tool.Baro_t("기여"), tool.Baro_t("문서 영향")}
	}

	html := ""

	if missing > 0 {
		html += tool.Baro_alert("warn",
			tool.Baro_t("목록에 <b>존재하지 않는 대상 ")+baro_mod_esc(tool.Baro_itoa(missing))+tool.Baro_t("개</b>가 있습니다 (표시된 ")+
				baro_mod_esc(tool.Baro_itoa(len(table_rows)))+tool.Baro_t("개 기준). 이름이 잘못되었을 수 있으니 실행 전에 확인하세요."))
	}

	if quarantined > 0 {
		html += tool.Baro_alert("info",
			tool.Baro_t("이미 격리 중인 문서가 ")+baro_mod_esc(tool.Baro_itoa(quarantined))+tool.Baro_t("개 있습니다. 중복 격리는 기록만 하나 더 남습니다."))
	}

	html += tool.Baro_table(headers, table_rows, tool.Baro_t("미리 볼 대상이 없습니다."))

	if len(targets) > limit {
		html += tool.Baro_note(tool.Baro_t("앞의 ") + tool.Baro_itoa(limit) + tool.Baro_t("개만 미리 봅니다. 전체 목록은 내려받기로 확인하세요."))
	}

	return html
}

// baro_mod_mass_action_block : 승인 · 실행 · 복구 버튼
func baro_mod_mass_action_block(db *sql.DB, config tool.Config, row baro_mod_mass_row) string {
	can_mass := tool.Baro_can(db, config.IP, "mass")
	can_approve := tool.Baro_can(db, config.IP, "mass_approve")

	body := ""

	switch row.Status {
	case "draft":
		small := tool.Str_to_int(row.Num) < tool.Baro_mass_threshold(db)

		if small {
			body += tool.Baro_alert("info",
				tool.Baro_t("대상이 기준(")+baro_mod_esc(tool.Baro_itoa(tool.Baro_mass_threshold(db)))+tool.Baro_t("개) 미만이라 2인 승인과 유예가 면제됩니다. ")+
					tool.Baro_t("그래도 실행 전 스냅샷과 일괄 복구는 그대로 적용됩니다."))

			if can_mass {
				body += tool.Baro_button("/baro/mass/execute/"+tool.Url_parser(row.ID), "실행 화면으로", "danger") + " "
			}
		} else {
			body += tool.Baro_alert("info", tool.Baro_t("아직 실행되지 않았습니다. 서로 다른 운영자 2명이 승인해야 다음 단계로 넘어갑니다."))
		}

		if can_approve {
			if row.Actor == config.IP {
				body += tool.Baro_alert("warn", tool.Baro_t("자기가 만든 작업은 스스로 승인할 수 없습니다. 다른 운영자의 승인이 필요합니다."))
			} else {
				body += tool.Baro_button("/baro/mass/approve/"+tool.Url_parser(row.ID), "이 작업 승인하기", "primary")
			}
		} else {
			body += tool.Baro_note(tool.Baro_t("승인 권한이 없습니다. 승인은 제재 담당 · 시스템 관리자 · 이의제기 담당이 할 수 있습니다."))
		}

	case "approved":
		ok, why := baro_mod_mass_can_execute(db, row)
		if ok {
			body += tool.Baro_alert("warn", tool.Baro_t("승인과 유예가 모두 끝났습니다. 이제 실행할 수 있습니다. 실행 전에 스냅샷이 자동으로 만들어집니다."))
			if can_mass {
				body += tool.Baro_button("/baro/mass/execute/"+tool.Url_parser(row.ID), "실행 화면으로", "danger")
			}
		} else {
			body += tool.Baro_alert("info", baro_mod_esc(why))
		}

	case "done":
		body += tool.Baro_alert("ok", tool.Baro_t("실행이 끝났습니다. 문제가 있으면 아래 버튼으로 한 번에 되돌릴 수 있습니다."))
		if can_mass && row.UndoStatus == "" {
			body += tool.Baro_button("/baro/mass/undo/"+tool.Url_parser(row.ID), "원클릭 일괄 복구", "danger")
		}

	case "undone":
		body += tool.Baro_alert("ok", tool.Baro_t("일괄 복구가 끝났습니다. 스냅샷을 되돌려 실행 전 상태로 돌아갔습니다."))

	case "rejected":
		body += tool.Baro_alert("info", tool.Baro_t("취소된 작업입니다."))
	}

	// 취소
	if can_mass && (row.Status == "draft" || row.Status == "approved") {
		cancel_form := tool.Baro_hidden("act", "cancel")
		cancel_form += tool.Baro_textarea("reason", tool.Baro_t("취소 사유"), "", 2, "", "")
		cancel_form += tool.Baro_submit(tool.Baro_t("이 작업 취소"), "")

		body += tool.Baro_section("작업 취소", tool.Baro_form("/baro/mass/view/"+tool.Url_parser(row.ID), cancel_form))
	}

	return tool.Baro_card(tool.Baro_t("진행"), body)
}

// baro_mod_mass_snapshot_table : 스냅샷 목록
func baro_mod_mass_snapshot_table(db *sql.DB, row baro_mod_mass_row) string {
	rows := tool.Query_DB(
		db,
		"select id, kind, title, data, meta, date from baro_snapshot where mass_id = ? order by id + 0 asc limit 200",
		row.ID,
	)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		id, kind, title, data, meta, date := "", "", "", "", "", ""
		if err := rows.Scan(&id, &kind, &title, &data, &meta, &date); err != nil {
			continue
		}

		table_rows = append(table_rows, []string{
			baro_mod_esc(id),
			baro_mod_esc(kind),
			baro_mod_esc(title),
			baro_mod_esc(tool.Baro_comma(tool.Get_len(data)) + tool.Baro_t("자")),
			baro_mod_esc(tool.Baro_trunc(meta, 120)),
			baro_mod_esc(date),
		})
	}

	total := 0
	tool.QueryRow_DB(db, "select count(*) from baro_snapshot where mass_id = ?", []any{&total}, row.ID)

	return tool.Baro_note(tool.Baro_t("스냅샷 ")+tool.Baro_comma(total)+tool.Baro_t("건이 보존되어 있습니다. 이것이 일괄 복구의 근거입니다.")) +
		tool.Baro_table([]string{"번호", "종류", tool.Baro_t("대상"), tool.Baro_t("보존 길이"), tool.Baro_t("메타"), "시각"}, table_rows, tool.Baro_t("스냅샷이 없습니다."))
}

// baro_mod_mass_view_post : 상세 화면에서의 처리 (취소)
func baro_mod_mass_view_post(db *sql.DB, config tool.Config, row baro_mod_mass_row, values url.Values) string {
	target_url := "/baro/mass/view/" + tool.Url_parser(row.ID)

	if baro_mod_val(values, "act") == "cancel" {
		if row.Status != "draft" && row.Status != "approved" {
			return tool.Get_redirect(target_url)
		}

		reason := baro_mod_or(tool.Baro_trunc(baro_mod_raw(values, "reason"), 1000), "신청자 취소")

		tool.Exec_DB(db, "update baro_mass_op set status = 'rejected' where id = ?", row.ID)

		tool.Baro_public_log(db, "mass_cancel", config.IP, tool.Baro_mass_kind_name(row.Kind), reason, "작업 "+row.ID)
		tool.Baro_audit(db, config, "mass_cancel", "작업 "+row.ID, row.Status, "rejected", reason)
	}

	return tool.Get_redirect(target_url)
}

// ------------------------------------------------------------
//  대상 목록 내려받기
// ------------------------------------------------------------

// View_baro_mod_mass_download : 대상 목록을 text/plain 으로 (/baro/mass/download/<id>)
//
//	대량 작업의 대상은 공개가 원칙이므로 누구나 내려받을 수 있다.
func View_baro_mod_mass_download(config tool.Config, arg string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = config

	row, ok := baro_mod_mass_get(db, arg)
	if !ok {
		return tool.Baro_t("# 대량 작업을 찾을 수 없습니다. (작업 번호를 확인해 주세요)\n")
	}

	detail := baro_mod_mass_detail(row)

	head := tool.Baro_t("# 바로위키 대량 작업 대상 목록\n")
	head += tool.Baro_t("# 작업 번호 : ") + row.ID + "\n"
	head += tool.Baro_t("# 종류      : ") + tool.Baro_mass_kind_name(row.Kind) + "\n"
	head += tool.Baro_t("# 대상 수   : ") + row.Num + "\n"
	head += tool.Baro_t("# 신청자    : ") + tool.Baro_mask_ip(row.Actor) + "\n"
	head += tool.Baro_t("# 신청 시각 : ") + row.Created + "\n"
	head += tool.Baro_t("# 상태      : ") + baro_mod_status_name(row.Status) + "\n"
	head += tool.Baro_t("# 승인자    : ") + baro_mod_or(row.Approver, tool.Baro_t("(없음)")) + "\n"
	head += tool.Baro_t("# 유예 종료 : ") + baro_mod_or(row.GraceUntil, tool.Baro_t("(미정)")) + "\n"

	if row.Urgent == "on" {
		head += tool.Baro_t("# 긴급      : 예 / 사유 : ") + tool.Baro_one_line(detail["urgent_reason"]) + "\n"
	}

	head += tool.Baro_t("# 사유      : ") + tool.Baro_one_line(row.Reason) + "\n"
	head += "#\n"
	head += tool.Baro_t("# 아래는 이 작업이 영향을 주는 대상 전체입니다.\n")
	head += "# ------------------------------------------------------------\n"

	return head + strings.Join(baro_mod_mass_targets(row), "\n") + "\n"
}

// ------------------------------------------------------------
//  승인 (2인 승인)
// ------------------------------------------------------------

// View_baro_mod_mass_approve : 대량 작업 승인 (/baro/mass/approve/<id>)
func View_baro_mod_mass_approve(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query

	if !tool.Baro_can(db, config.IP, "mass_approve") {
		return tool.Baro_deny(db, config, tool.Baro_t("대량 작업 승인은 제재 담당 · 시스템 관리자 · 이의제기 담당 역할이 있어야 합니다."))
	}

	row, ok := baro_mod_mass_get(db, arg)
	if !ok {
		return baro_mod_missing(db, config, "/baro/mass", tool.Baro_t("대량 작업"), "/baro/mass", tool.Baro_t("대량 작업 목록으로"))
	}

	approvers := baro_mod_mass_approvers(row)
	already := false
	for _, name := range approvers {
		if name == config.IP {
			already = true
		}
	}

	self := row.Actor == config.IP

	if values != nil {
		if self {
			return tool.Baro_deny(db, config, tool.Baro_t("자기가 만든 대량 작업은 스스로 승인할 수 없습니다."))
		}

		if already {
			return tool.Get_redirect("/baro/mass/view/" + tool.Url_parser(row.ID))
		}

		return baro_mod_mass_approve_post(db, config, row, approvers, values)
	}

	targets := baro_mod_mass_targets(row)

	body := tool.Baro_crumb([][]string{
		{"/baro/mass", tool.Baro_t("대량 작업")},
		{"/baro/mass/view/" + tool.Url_parser(row.ID), "작업 " + row.ID},
		{"", "승인"},
	})

	body += tool.Baro_alert("warn",
		tool.Baro_t("<b>승인은 서명입니다.</b> 승인하면 회원님의 이름이 이 작업의 승인자로 공개 기록에 남습니다. ")+
			tool.Baro_t("대상 목록과 예상 영향을 직접 확인한 뒤에만 승인하세요."))

	body += tool.Baro_card(tool.Baro_t("승인 대상"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("영향받는 대상"), tool.Baro_comma(tool.Str_to_int(row.Num)), ""},
		{tool.Baro_t("작업 종류"), tool.Baro_mass_kind_name(row.Kind), ""},
		{tool.Baro_t("현재 승인"), tool.Baro_itoa(len(approvers)) + " / 2", ""},
		{tool.Baro_t("긴급"), baro_mod_mass_urgent_text(row), ""},
	}))

	body += tool.Baro_card("사유", tool.Baro_code(row.Reason))
	body += tool.Baro_card(tool.Baro_t("대상 목록 (앞 200개)"),
		tool.Baro_button("/baro/mass/download/"+tool.Url_parser(row.ID), "전체 목록 내려받기", "")+
			tool.Baro_code(strings.Join(baro_mod_mass_preview_list(targets, 200), "\n")))

	body += tool.Baro_card(tool.Baro_t("예상 영향 미리보기"), baro_mod_mass_preview_table(db, config, row, targets))

	if self {
		body += tool.Baro_alert("danger",
			tool.Baro_t("<b>자기가 만든 작업은 스스로 승인할 수 없습니다.</b> 이것이 2인 승인의 핵심입니다. ")+
				tool.Baro_t("다른 운영자에게 검토를 요청하세요."))

		return baro_mod_shell(db, config, tool.Baro_t("대량 작업 승인"), "/baro/mass", body)
	}

	if already {
		body += tool.Baro_alert("info", tool.Baro_t("이미 이 작업을 승인했습니다. 같은 사람이 두 번 승인할 수 없습니다."))

		return baro_mod_shell(db, config, tool.Baro_t("대량 작업 승인"), "/baro/mass", body)
	}

	if row.Status != "draft" {
		body += tool.Baro_alert("info", tool.Baro_t("지금은 승인 단계가 아닙니다. (현재 상태: ")+baro_mod_esc(baro_mod_status_name(row.Status))+")")

		return baro_mod_shell(db, config, tool.Baro_t("대량 작업 승인"), "/baro/mass", body)
	}

	form := tool.Baro_textarea("memo", tool.Baro_t("승인 의견"), "", 3, tool.Baro_t("확인한 내용이나 조건을 적습니다."), tool.Baro_t("감사 기록에 남습니다."))
	form += tool.Baro_checkbox("checked_list", tool.Baro_t("대상 목록을 직접 확인했습니다"), "on", false, "")
	form += tool.Baro_checkbox("checked_reason", tool.Baro_t("사유가 규정에 맞는지 확인했습니다"), "on", false, "")
	form += tool.Baro_submit("승인", "danger")

	body += tool.Baro_card("승인", tool.Baro_form("/baro/mass/approve/"+tool.Url_parser(row.ID), form))

	body += tool.Baro_card("", tool.Baro_button("/baro/mass/view/"+tool.Url_parser(row.ID), "작업 상세로 돌아가기", ""))

	return baro_mod_shell(db, config, tool.Baro_t("대량 작업 승인"), "/baro/mass", body)
}

// baro_mod_mass_urgent_text : 긴급 여부 표시용
func baro_mod_mass_urgent_text(row baro_mod_mass_row) string {
	if row.Urgent == "on" {
		return tool.Baro_t("긴급")
	}

	return tool.Baro_t("일반")
}

// baro_mod_mass_approve_post : 승인 처리
func baro_mod_mass_approve_post(db *sql.DB, config tool.Config, row baro_mod_mass_row, approvers []string, values url.Values) string {
	target_url := "/baro/mass/view/" + tool.Url_parser(row.ID)

	if !baro_mod_on(values, "checked_list") || !baro_mod_on(values, "checked_reason") {
		body := tool.Baro_alert("danger", tool.Baro_t("확인란 두 개를 모두 체크해야 승인할 수 있습니다. 목록과 사유를 직접 확인해 주세요."))
		body += tool.Baro_card("", tool.Baro_button("/baro/mass/approve/"+tool.Url_parser(row.ID), "돌아가기", "primary"))

		return baro_mod_shell(db, config, tool.Baro_t("대량 작업 승인"), "/baro/mass", body)
	}

	memo := tool.Baro_trunc(baro_mod_raw(values, "memo"), 1000)
	now := tool.Baro_now()

	new_approvers := append(append([]string{}, approvers...), config.IP)
	approver_text := strings.Join(new_approvers, ",")

	status := row.Status
	grace_until := row.GraceUntil
	approved_date := row.ApprovedDate

	// 서로 다른 운영자 2인이 되면 승인 완료 → 유예 시작
	if len(new_approvers) >= 2 {
		status = "approved"
		approved_date = now

		if row.Urgent == "on" {
			grace_until = now
		} else {
			grace_until = tool.Baro_time_add(now, 0, tool.Baro_mass_grace_hours(db), 0)
		}
	}

	tool.Exec_DB(
		db,
		"update baro_mass_op set approver = ?, status = ?, approved_date = ?, grace_until = ? where id = ?",
		approver_text, status, approved_date, grace_until, row.ID,
	)

	tool.Baro_public_log(db, "mass_approve", config.IP, tool.Baro_mass_kind_name(row.Kind),
		memo, "작업 "+row.ID+" 승인 "+tool.Baro_itoa(len(new_approvers))+"/2")

	tool.Baro_audit(db, config, "mass_approve", "작업 "+row.ID, row.Approver, approver_text, memo)

	baro_mod_notify_user(db, row.Actor, "system",
		tool.Baro_t("대량 작업 ")+row.ID+tool.Baro_t(" 이(가) 승인되었습니다 (")+tool.Baro_itoa(len(new_approvers))+"/2)",
		target_url, memo)

	return tool.Get_redirect(target_url)
}

// ------------------------------------------------------------
//  실행
// ------------------------------------------------------------

// View_baro_mod_mass_execute : 대량 작업 실행 (/baro/mass/execute/<id>)
func View_baro_mod_mass_execute(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query

	if !tool.Baro_can(db, config.IP, "mass") {
		return tool.Baro_deny(db, config, tool.Baro_t("대량 작업 실행 권한이 없습니다."))
	}

	row, ok := baro_mod_mass_get(db, arg)
	if !ok {
		return baro_mod_missing(db, config, "/baro/mass", tool.Baro_t("대량 작업"), "/baro/mass", tool.Baro_t("대량 작업 목록으로"))
	}

	if page, stop := baro_mod_read_only_block(db, config, "/baro/mass"); stop {
		return page
	}

	can_run, why := baro_mod_mass_can_execute(db, row)

	if values != nil {
		if !can_run {
			body := tool.Baro_alert("danger", baro_mod_esc(why))
			body += tool.Baro_card("", tool.Baro_button("/baro/mass/view/"+tool.Url_parser(row.ID), "작업 상세로", "primary"))

			return baro_mod_shell(db, config, tool.Baro_t("대량 작업 실행"), "/baro/mass", body)
		}

		return baro_mod_mass_execute_post(db, config, row, values)
	}

	targets := baro_mod_mass_targets(row)

	body := tool.Baro_crumb([][]string{
		{"/baro/mass", tool.Baro_t("대량 작업")},
		{"/baro/mass/view/" + tool.Url_parser(row.ID), "작업 " + row.ID},
		{"", "실행"},
	})

	body += tool.Baro_alert("danger",
		tool.Baro_t("<b>지금 ")+baro_mod_esc(tool.Baro_comma(tool.Str_to_int(row.Num)))+tool.Baro_t("개 대상이 한꺼번에 바뀝니다.</b> ")+
			tool.Baro_t("실행 직전에 복구 스냅샷을 자동으로 만들지만, 그래도 다시 한 번 목록을 확인하세요."))

	body += tool.Baro_card(tool.Baro_t("실행 요약"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("영향받는 대상"), tool.Baro_comma(tool.Str_to_int(row.Num)), ""},
		{tool.Baro_t("작업 종류"), tool.Baro_mass_kind_name(row.Kind), ""},
		{tool.Baro_t("승인자"), tool.Baro_itoa(len(baro_mod_mass_approvers(row))) + tool.Baro_t("명"), ""},
		{tool.Baro_t("긴급"), baro_mod_mass_urgent_text(row), ""},
	}))

	if !can_run {
		body += tool.Baro_alert("warn", baro_mod_esc(why))
		body += tool.Baro_card("", tool.Baro_button("/baro/mass/view/"+tool.Url_parser(row.ID), "작업 상세로 돌아가기", "primary"))

		return baro_mod_shell(db, config, tool.Baro_t("대량 작업 실행"), "/baro/mass", body)
	}

	body += tool.Baro_card("사유", tool.Baro_code(row.Reason))
	body += tool.Baro_card(tool.Baro_t("대상 목록 (앞 200개)"),
		tool.Baro_button("/baro/mass/download/"+tool.Url_parser(row.ID), "전체 목록 내려받기", "")+
			tool.Baro_code(strings.Join(baro_mod_mass_preview_list(targets, 200), "\n")))

	body += tool.Baro_card(tool.Baro_t("예상 영향 미리보기"), baro_mod_mass_preview_table(db, config, row, targets))

	form := tool.Baro_input("confirm", tool.Baro_t("확인 문구"), "", tool.Baro_itoa(len(targets)), tool.Baro_t("영향받는 대상 수를 정확히 입력해야 실행됩니다."))
	form += tool.Baro_checkbox("agree", tool.Baro_t("복구 스냅샷이 만들어진다는 점과, 잘못되면 일괄 복구로 되돌릴 수 있다는 점을 확인했습니다"), "on", false, "")
	form += tool.Baro_submit(tool.Baro_t("지금 실행"), "danger")

	body += tool.Baro_danger_zone("실행", tool.Baro_form("/baro/mass/execute/"+tool.Url_parser(row.ID), form))

	body += tool.Baro_card("", tool.Baro_button("/baro/mass/view/"+tool.Url_parser(row.ID), "돌아가기", ""))

	return baro_mod_shell(db, config, tool.Baro_t("대량 작업 실행"), "/baro/mass", body)
}

// baro_mod_mass_execute_post : 실제 실행
func baro_mod_mass_execute_post(db *sql.DB, config tool.Config, row baro_mod_mass_row, values url.Values) string {
	targets := baro_mod_mass_targets(row)
	detail := baro_mod_mass_detail(row)

	fail := func(message string) string {
		body := tool.Baro_alert("danger", baro_mod_esc(message))
		body += tool.Baro_card("", tool.Baro_button("/baro/mass/execute/"+tool.Url_parser(row.ID), "돌아가기", "primary"))

		return baro_mod_shell(db, config, tool.Baro_t("대량 작업 실행"), "/baro/mass", body)
	}

	if baro_mod_val(values, "confirm") != tool.Baro_itoa(len(targets)) {
		return fail(tool.Baro_t("확인 문구에 영향받는 대상 수(") + tool.Baro_itoa(len(targets)) + tool.Baro_t(")를 정확히 입력해 주세요."))
	}

	if !baro_mod_on(values, "agree") {
		return fail(tool.Baro_t("확인란을 체크해 주세요."))
	}

	now := tool.Baro_now()
	done := 0
	skipped := 0

	// ---- 1. 복구 스냅샷 먼저 만든다 ----
	for _, target := range targets {
		if baro_mod_mass_is_user_kind(row.Kind) {
			active := tool.Baro_sanction_active(db, target)
			meta := tool.Baro_json_enc(map[string]string{
				"kind":     row.Kind,
				"sanction": tool.Baro_itoa(len(active)),
			})

			tool.Exec_DB(
				db,
				"insert into baro_snapshot (id, mass_id, kind, title, data, meta, date) values (?, ?, 'user', ?, '', ?, ?)",
				tool.Baro_next_id(db, "baro_snapshot"), row.ID, target, meta, now,
			)

			continue
		}

		data, exists := baro_mod_doc_data(db, target)
		acl_data := ""
		tool.QueryRow_DB(db, "select data from acl where title = ? and type = ?", []any{&acl_data}, target, "document_edit_acl")

		meta := tool.Baro_json_enc(map[string]string{
			"kind":       row.Kind,
			"exists":     baro_mod_bool_text(exists),
			"status":     tool.Baro_doc_status_get(db, target),
			"acl":        acl_data,
			"rev":        tool.Get_document_revision(db, target),
			"snap_taken": now,
		})

		tool.Exec_DB(
			db,
			"insert into baro_snapshot (id, mass_id, kind, title, data, meta, date) values (?, ?, 'doc', ?, ?, ?, ?)",
			tool.Baro_next_id(db, "baro_snapshot"), row.ID, target, data, meta, now,
		)
	}

	// ---- 2. 실제 작업 ----
	notified := map[string]bool{}

	for _, target := range targets {
		switch row.Kind {
		case "delete":
			// 삭제는 격리로 처리한다 (고정 원칙 2 — 지우지 않는다)
			if !baro_mod_doc_exists(db, target) {
				skipped++
				continue
			}

			baro_mod_quarantine_add(db, config, target, "", "normal",
				tool.Baro_t("[대량 작업 ")+row.ID+"] "+row.Reason, detail["case_id"], true)
			done++

		case "quarantine":
			if !baro_mod_doc_exists(db, target) {
				skipped++
				continue
			}

			baro_mod_quarantine_add(db, config, target, "", "normal",
				tool.Baro_t("[대량 작업 ")+row.ID+"] "+row.Reason, detail["case_id"], false)
			done++

		case "revert":
			_, prev_data, ok := baro_mod_doc_prev_rev(db, target)
			if !ok {
				skipped++
				continue
			}

			baro_mod_doc_write(db, config, target, prev_data, tool.Baro_t("[대량 되돌리기 ")+row.ID+"] "+tool.Baro_one_line(row.Reason))
			done++

		case "block":
			if !tool.Baro_is_user(target) {
				skipped++
				continue
			}

			days := tool.Str_to_int(baro_mod_or(detail["sanction_days"], "3"))
			kind := baro_mod_or(detail["sanction_kind"], "short_block")

			end_date := ""
			if days > 0 {
				end_date = tool.Baro_time_add(now, days, 0, 0)
			}

			tool.Exec_DB(
				db,
				"insert into baro_sanction (id, user, kind, scope, reason, case_id, actor, start_date, end_date, status, date) values (?, ?, ?, '', ?, ?, ?, ?, ?, 'active', ?)",
				tool.Baro_next_id(db, "baro_sanction"), target, kind,
				"[대량 작업 "+row.ID+"] "+row.Reason, detail["case_id"], config.IP, now, end_date, now,
			)

			if baro_mod_sanction_is_block(kind) {
				ban_end := end_date
				if ban_end == "" {
					ban_end = "0"
				}

				tool.Do_ban_insert(db, target, ban_end, tool.Baro_t("[대량 작업 ")+row.ID+"] "+row.Reason, "", config.IP, "", false)
			}

			done++

		case "protect":
			acl_value := baro_mod_or(detail["acl"], "admin")

			tool.Exec_DB(db, "delete from acl where title = ? and type = ?", target, "document_edit_acl")
			tool.Exec_DB(db, "insert into acl (title, data, type) values (?, ?, ?)", target, acl_value, "document_edit_acl")
			done++

		case "category":
			from := detail["from"]
			if strings.TrimSpace(from) == "" {
				skipped++
				continue
			}

			data, exists := baro_mod_doc_data(db, target)
			if !exists || !strings.Contains(data, from) {
				skipped++
				continue
			}

			baro_mod_doc_write(db, config, target, strings.ReplaceAll(data, from, detail["to"]),
				tool.Baro_t("[대량 분류 변경 ")+row.ID+"] "+tool.Baro_one_line(row.Reason))
			done++

		default:
			skipped++
		}

		// ---- 3. 대상 사용자에게 알림 ----
		notify_target := target
		if !baro_mod_mass_is_user_kind(row.Kind) {
			notify_target = baro_mod_doc_last_editor(db, target)
		}

		if notify_target != "" && !notified[notify_target] {
			notified[notify_target] = true

			baro_mod_notify_user(db, notify_target, "system",
				tool.Baro_t("대량 작업 ")+row.ID+" ("+tool.Baro_mass_kind_name(row.Kind)+tool.Baro_t(") 이(가) 실행되었습니다. 대상 목록과 사유를 확인할 수 있습니다."),
				"/baro/mass/view/"+tool.Url_parser(row.ID), row.Reason)
		}
	}

	detail["done"] = tool.Baro_itoa(done)
	detail["skipped"] = tool.Baro_itoa(skipped)
	detail["executor"] = config.IP

	tool.Exec_DB(
		db,
		"update baro_mass_op set status = 'done', execute_date = ?, detail = ? where id = ?",
		now, tool.Baro_json_enc(detail), row.ID,
	)

	// ---- 4. 자동 공지 ----
	urgent_mark := ""
	if row.Urgent == "on" {
		urgent_mark = tool.Baro_t(" [긴급 실행 — 유예 없음]")
	}

	tool.Baro_public_log(db, "mass_execute", config.IP, tool.Baro_mass_kind_name(row.Kind),
		tool.Baro_trunc(tool.Baro_one_line(row.Reason), 200),
		"작업 "+row.ID+" / 대상 "+tool.Baro_comma(len(targets))+"개 / 처리 "+tool.Baro_itoa(done)+
			"개 / 건너뜀 "+tool.Baro_itoa(skipped)+"개 / 승인자 "+row.Approver+urgent_mark)

	tool.Baro_audit(db, config, "mass_execute", "작업 "+row.ID, row.Status, "done",
		row.Reason+" / 처리 "+tool.Baro_itoa(done)+"건")

	tool.Baro_metric_bump(db, "mass_execute", tool.Baro_today(), 1)
	tool.Baro_metric_bump(db, "mass_target", tool.Baro_today(), len(targets))

	if detail["case_id"] != "" {
		tool.Exec_DB(
			db,
			"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, 'note', ?, ?, ?)",
			tool.Baro_next_id(db, "baro_case_action"), detail["case_id"], config.IP,
			"대량 작업 "+row.ID+" 을(를) 실행했습니다. 대상 "+tool.Baro_itoa(len(targets))+"개 / 처리 "+tool.Baro_itoa(done)+"개",
			now, tool.Baro_mass_kind_name(row.Kind),
		)
	}

	return tool.Get_redirect("/baro/mass/view/" + tool.Url_parser(row.ID))
}

// baro_mod_bool_text : 불리언을 저장용 문자열로
func baro_mod_bool_text(value bool) string {
	if value {
		return "on"
	}

	return ""
}

// ------------------------------------------------------------
//  원클릭 일괄 복구
// ------------------------------------------------------------

// View_baro_mod_mass_undo : 일괄 복구 (/baro/mass/undo/<id>)
func View_baro_mod_mass_undo(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query

	if !tool.Baro_can(db, config.IP, "mass") {
		return tool.Baro_deny(db, config, tool.Baro_t("일괄 복구 권한이 없습니다."))
	}

	row, ok := baro_mod_mass_get(db, arg)
	if !ok {
		return baro_mod_missing(db, config, "/baro/mass", tool.Baro_t("대량 작업"), "/baro/mass", tool.Baro_t("대량 작업 목록으로"))
	}

	if values != nil {
		return baro_mod_mass_undo_post(db, config, row, values)
	}

	snapshot_count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_snapshot where mass_id = ?", []any{&snapshot_count}, row.ID)

	body := tool.Baro_crumb([][]string{
		{"/baro/mass", tool.Baro_t("대량 작업")},
		{"/baro/mass/view/" + tool.Url_parser(row.ID), "작업 " + row.ID},
		{"", "일괄 복구"},
	})

	if row.Status != "done" {
		body += tool.Baro_alert("info", tool.Baro_t("실행된 작업만 복구할 수 있습니다. (현재 상태: ")+baro_mod_esc(baro_mod_status_name(row.Status))+")")
		body += tool.Baro_card("", tool.Baro_button("/baro/mass/view/"+tool.Url_parser(row.ID), "작업 상세로", "primary"))

		return baro_mod_shell(db, config, "일괄 복구", "/baro/mass", body)
	}

	body += tool.Baro_alert("ok",
		tool.Baro_t("<b>버튼 하나로 실행 전 상태로 돌아갑니다.</b> 실행 직전에 만들어 둔 스냅샷 ")+
			baro_mod_esc(tool.Baro_comma(snapshot_count))+tool.Baro_t("건을 그대로 되돌립니다."))

	body += tool.Baro_card(tool.Baro_t("복구 요약"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("스냅샷"), tool.Baro_comma(snapshot_count), tool.Baro_t("이만큼을 되돌립니다")},
		{tool.Baro_t("작업 종류"), tool.Baro_mass_kind_name(row.Kind), ""},
		{"실행 시각", baro_mod_or(row.ExecuteDate, "-"), ""},
	}))

	body += tool.Baro_card(tool.Baro_t("복구할 스냅샷"), baro_mod_mass_snapshot_table(db, row))

	form := tool.Baro_textarea("reason", tool.Baro_t("복구 사유"), "", 4, tool.Baro_t("왜 되돌리는지 적습니다."), tool.Baro_t("필수 항목입니다. 공개 로그에 남습니다."))
	form += tool.Baro_checkbox("agree", tool.Baro_t("실행 후에 다른 사람이 편집한 내용이 있다면 덮어쓸 수 있음을 확인했습니다"), "on", false, "")
	form += tool.Baro_submit("원클릭 일괄 복구", "danger")

	body += tool.Baro_danger_zone("일괄 복구", tool.Baro_form("/baro/mass/undo/"+tool.Url_parser(row.ID), form))

	return baro_mod_shell(db, config, "일괄 복구", "/baro/mass", body)
}

// baro_mod_mass_undo_post : 일괄 복구 처리
func baro_mod_mass_undo_post(db *sql.DB, config tool.Config, row baro_mod_mass_row, values url.Values) string {
	target_url := "/baro/mass/view/" + tool.Url_parser(row.ID)

	if row.Status != "done" {
		return tool.Get_redirect(target_url)
	}

	reason := tool.Baro_trunc(baro_mod_raw(values, "reason"), 2000)
	if strings.TrimSpace(reason) == "" {
		body := tool.Baro_alert("danger", tool.Baro_t("복구 사유를 적어 주세요."))
		body += tool.Baro_card("", tool.Baro_button("/baro/mass/undo/"+tool.Url_parser(row.ID), "돌아가기", "primary"))

		return baro_mod_shell(db, config, "일괄 복구", "/baro/mass", body)
	}

	if !baro_mod_on(values, "agree") {
		body := tool.Baro_alert("danger", tool.Baro_t("확인란을 체크해 주세요."))
		body += tool.Baro_card("", tool.Baro_button("/baro/mass/undo/"+tool.Url_parser(row.ID), "돌아가기", "primary"))

		return baro_mod_shell(db, config, "일괄 복구", "/baro/mass", body)
	}

	// 스냅샷을 먼저 전부 읽어 둔다 (되돌리는 도중에 표를 바꾸므로)
	rows := tool.Query_DB(db, "select kind, title, data, meta from baro_snapshot where mass_id = ? order by id + 0 asc", row.ID)

	list := [][]string{}
	for rows.Next() {
		kind, title, data, meta := "", "", "", ""
		if err := rows.Scan(&kind, &title, &data, &meta); err != nil {
			continue
		}

		list = append(list, []string{kind, title, data, meta})
	}
	rows.Close()

	now := tool.Baro_now()
	restored := 0

	for _, item := range list {
		snap_kind, title, data, meta_raw := item[0], item[1], item[2], item[3]
		meta := tool.Baro_json_dec_str(meta_raw)

		if snap_kind == "user" {
			// 이 대량 작업으로 걸린 제재를 해제한다
			sanction_rows := tool.Query_DB(
				db,
				"select id, kind from baro_sanction where user = ? and status = 'active' and reason like ?",
				title, tool.Baro_t("[대량 작업 ")+row.ID+"]%",
			)

			sanctions := [][]string{}
			for sanction_rows.Next() {
				id, kind := "", ""
				if err := sanction_rows.Scan(&id, &kind); err != nil {
					continue
				}

				sanctions = append(sanctions, []string{id, kind})
			}
			sanction_rows.Close()

			for _, sanction := range sanctions {
				tool.Exec_DB(db, "update baro_sanction set status = 'closed', end_date = ? where id = ?", now, sanction[0])

				if baro_mod_sanction_is_block(sanction[1]) {
					tool.Do_ban_insert(db, title, "", tool.Baro_t("대량 작업 ")+row.ID+tool.Baro_t(" 일괄 복구: ")+reason, "", config.IP, "", true)
				}

				restored++
			}

			baro_mod_notify_user(db, title, "system",
				tool.Baro_t("대량 작업 ")+row.ID+tool.Baro_t(" 이(가) 일괄 복구되어 제재가 해제되었습니다"),
				target_url, reason)

			continue
		}

		// 문서 : 실행 직전 본문과 상태·ACL 을 되돌린다
		if meta["exists"] == "on" {
			baro_mod_doc_write(db, config, title, data, tool.Baro_t("[대량 작업 ")+row.ID+tool.Baro_t(" 일괄 복구] ")+tool.Baro_one_line(reason))
		}

		tool.Baro_doc_status_set(db, title, meta["status"], "대량 작업 일괄 복구", config.IP)

		tool.Exec_DB(db, "delete from acl where title = ? and type = ?", title, "document_edit_acl")
		if meta["acl"] != "" {
			tool.Exec_DB(db, "insert into acl (title, data, type) values (?, ?, ?)", title, meta["acl"], "document_edit_acl")
		}

		// 이 작업으로 만들어진 격리 기록을 닫는다
		tool.Exec_DB(
			db,
			"update baro_quarantine set status = 'released', release_date = ? where title = ? and status = 'open' and reason like ?",
			now, title, "[대량 작업 "+row.ID+"]%",
		)

		restored++
	}

	detail := baro_mod_mass_detail(row)
	detail["undo_reason"] = reason
	detail["undo_by"] = config.IP
	detail["undo_count"] = tool.Baro_itoa(restored)

	tool.Exec_DB(
		db,
		"update baro_mass_op set status = 'undone', undo_status = ?, detail = ? where id = ?",
		"done "+now, tool.Baro_json_enc(detail), row.ID,
	)

	tool.Baro_public_log(db, "mass_undo", config.IP, tool.Baro_mass_kind_name(row.Kind),
		tool.Baro_trunc(tool.Baro_one_line(reason), 200),
		"작업 "+row.ID+" 일괄 복구 / 되돌린 대상 "+tool.Baro_comma(restored)+"개")

	tool.Baro_audit(db, config, "mass_undo", "작업 "+row.ID, "done", "undone", reason)
	tool.Baro_metric_bump(db, "mass_undo", tool.Baro_today(), 1)

	baro_mod_notify_user(db, row.Actor, "system",
		tool.Baro_t("대량 작업 ")+row.ID+tool.Baro_t(" 이(가) 일괄 복구되었습니다"),
		target_url, reason)

	return tool.Get_redirect(target_url)
}
