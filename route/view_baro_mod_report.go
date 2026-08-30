package route

// ============================================================
//  10-1 신고 (/baro/report)
//
//  신고자는 반드시 자기 신고의 진행 상태를 볼 수 있어야 한다.
//  담당자는 우선순위·유형·긴급도로 정렬된 큐에서 처리하고,
//  같은 대상에 대한 중복 신고는 하나의 사건으로 묶는다.
// ============================================================

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ------------------------------------------------------------
//  신고 안내 홈
// ------------------------------------------------------------

// View_baro_mod_report_home : 신고 안내 (/baro/report)
func View_baro_mod_report_home(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = arg
	_ = query
	_ = values

	body := tool.Baro_hero(
		"신고",
		tool.Baro_t("문제를 발견하면 알려 주세요. 접수한 신고의 진행 상태는 신고자가 직접 확인할 수 있습니다."),
		tool.Baro_button("/baro/report/new", tool.Baro_t("신고 접수하기"), "primary")+" "+
			tool.Baro_button("/baro/report/mine", tool.Baro_t("내 신고 보기"), ""),
	)

	// 신고 종류 안내
	kind_rows := [][]string{}
	for _, row := range baro_mod_report_kind_list() {
		kind_rows = append(kind_rows, []string{
			`<a href="/baro/report/new?kind=` + tool.Url_parser(row[0]) + `">` + baro_mod_esc(row[1]) + `</a>`,
			baro_mod_esc(row[2]),
			baro_mod_priority_badge(baro_mod_priority_of_kind(row[0])),
		})
	}

	body += tool.Baro_card(tool.Baro_t("신고할 수 있는 것"),
		tool.Baro_table([]string{"종류", tool.Baro_t("이런 때 신고합니다"), tool.Baro_t("기본 우선순위")}, kind_rows, tool.Baro_t("종류를 불러오지 못했습니다.")),
	)

	// 공개 통계 (개인정보 없이 건수만)
	open_count, pending_count, closed_count := 0, 0, 0
	tool.QueryRow_DB(db, "select count(*) from baro_report where status"+baro_mod_status_new_sql, []any{&open_count})
	tool.QueryRow_DB(db, "select count(*) from baro_report where status = 'pending'", []any{&pending_count})
	tool.QueryRow_DB(db, "select count(*) from baro_report where status = 'closed'", []any{&closed_count})

	body += tool.Baro_card(tool.Baro_t("지금 처리 현황"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("접수됨"), tool.Baro_comma(open_count), tool.Baro_t("아직 담당자가 배정되지 않았습니다")},
		{tool.Baro_t("처리 중"), tool.Baro_comma(pending_count), tool.Baro_t("담당자가 확인하고 있습니다")},
		{tool.Baro_t("종결"), tool.Baro_comma(closed_count), tool.Baro_t("처리가 끝났습니다")},
	}))

	body += tool.Baro_card(tool.Baro_t("신고가 처리되는 방법"),
		tool.Baro_list([]string{
			tool.Baro_t("접수하면 곧바로 접수 알림을 보내 드립니다. 접수 번호로 진행 상태를 볼 수 있습니다."),
			tool.Baro_t("담당자가 배정되고, 필요하면 <b>운영 사건</b>으로 전환되어 사건 번호가 붙습니다."),
			tool.Baro_t("문제가 있는 문서는 지우는 대신 <b>임시 격리</b>합니다. 원본은 그대로 보존됩니다."),
			tool.Baro_t("조치 결과와 적용 규정은 공개하지만, 신고자 정보와 신고 원문은 공개하지 않습니다."),
			tool.Baro_t("조치에 동의할 수 없으면 <b>이의제기</b>를 신청할 수 있습니다. 원조치 담당자는 심사에서 빠집니다."),
		}),
	)

	if tool.Baro_can(db, config.IP, "report") {
		body += tool.Baro_card(tool.Baro_t("담당자 기능"),
			tool.Baro_button("/baro/report/queue", tool.Baro_t("신고 큐 열기"), "primary")+" "+
				tool.Baro_button("/baro/case", tool.Baro_t("운영 사건"), "")+" "+
				tool.Baro_button("/baro/quarantine", tool.Baro_t("문서 격리"), ""),
		)
	}

	body += baro_mod_principle_note()

	return baro_mod_shell(db, config, "신고", "/baro/report", body)
}

// ------------------------------------------------------------
//  신고 접수
// ------------------------------------------------------------

// View_baro_mod_report_new : 신고 접수 (/baro/report/new)
func View_baro_mod_report_new(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = arg

	if page, stop := baro_mod_read_only_block(db, config, "/baro/report"); stop {
		return page
	}

	if values != nil {
		return baro_mod_report_new_post(db, config, values)
	}

	kind := baro_mod_val(query, "kind")
	if kind == "" {
		kind = "doc_error"
	}

	target_kind := baro_mod_val(query, "target_kind")
	if target_kind == "" {
		target_kind = "doc"
	}

	target := baro_mod_val(query, "target")

	body := tool.Baro_crumb([][]string{{"/baro/report", "신고"}, {"", tool.Baro_t("신고 접수")}})

	if !tool.Baro_is_user(config.IP) {
		body += tool.Baro_alert("info",
			tool.Baro_t("로그인하지 않아도 신고할 수 있습니다. 다만 진행 상태는 이 접속 주소로만 확인할 수 있으므로, ")+
				tool.Baro_t("결과를 확실히 받아 보려면 로그인 후 접수하시길 권합니다."))
	}

	form := tool.Baro_select("kind", tool.Baro_t("신고 종류"), baro_mod_report_kind_options(), kind, tool.Baro_t("가장 가까운 종류를 고르세요."))
	form += tool.Baro_select("target_kind", tool.Baro_t("대상 종류"), baro_mod_target_kind_list(), target_kind, tool.Baro_t("문서 / 사용자 / 토론 / 파일 / 사건 중에서 고릅니다."))
	form += tool.Baro_input("target", tool.Baro_t("대상"), target, tool.Baro_t("문서 이름 · 사용자 이름 · 토론 번호 · 사건 번호"), tool.Baro_t("무엇에 대한 신고인지 정확히 적어 주세요."))
	form += tool.Baro_textarea("detail", tool.Baro_t("상세 설명"), "", 8, tool.Baro_t("어떤 점이 문제인지 구체적으로 적어 주세요."), tool.Baro_t("필수 항목입니다. 어느 문단·어느 판인지 적어 주면 처리가 빨라집니다."))
	form += tool.Baro_textarea("evidence", tool.Baro_t("증거"), "", 5, tool.Baro_t("출처 URL, 원문 주소, 판 번호 등을 한 줄에 하나씩"), tool.Baro_t("증거는 담당자와 신고자만 볼 수 있습니다."))
	form += tool.Baro_submit(tool.Baro_t("신고 접수"), "primary")

	body += tool.Baro_card(tool.Baro_t("신고 접수"), tool.Baro_form("/baro/report/new", form))

	body += tool.Baro_card(tool.Baro_t("접수 전에 확인해 주세요"),
		tool.Baro_list([]string{
			tool.Baro_t("신고 내용과 신고자 정보는 공개되지 않습니다. 공개되는 것은 조치 결과와 적용 규정뿐입니다."),
			tool.Baro_t("단순한 의견 차이는 먼저 <a href=\"/baro/policy\">정책</a>과 토론으로 풀어 주세요."),
			tool.Baro_t("개인정보 노출·저작권 침해는 긴급으로 분류되어 먼저 처리됩니다."),
			tool.Baro_t("같은 대상에 대한 중복 신고는 하나의 사건으로 묶여 함께 처리됩니다."),
		}),
	)

	body += baro_mod_principle_note()

	return baro_mod_shell(db, config, tool.Baro_t("신고 접수"), "/baro/report", body)
}

// baro_mod_report_new_post : 신고 접수 처리
func baro_mod_report_new_post(db *sql.DB, config tool.Config, values url.Values) string {
	kind := baro_mod_val(values, "kind")
	target_kind := baro_mod_val(values, "target_kind")
	target := baro_mod_val(values, "target")
	detail := baro_mod_val(values, "detail")
	evidence := baro_mod_val(values, "evidence")

	fail := func(message string) string {
		body := tool.Baro_alert("danger", baro_mod_esc(message))
		body += tool.Baro_card("", tool.Baro_button("/baro/report/new", tool.Baro_t("신고 화면으로 돌아가기"), "primary"))

		return baro_mod_shell(db, config, tool.Baro_t("신고 접수"), "/baro/report", body)
	}

	if baro_mod_report_kind_name(kind) == "기타" && kind == "" {
		return fail(tool.Baro_t("신고 종류를 골라 주세요."))
	}

	if detail == "" {
		return fail(tool.Baro_t("상세 설명은 반드시 적어 주세요. 무엇이 문제인지 알 수 없으면 처리할 수 없습니다."))
	}

	if target == "" {
		return fail(tool.Baro_t("신고 대상을 적어 주세요."))
	}

	// 남용 방지 : 한 사람이 1시간에 5건까지
	if !tool.Baro_rate_ok(db, "baro_report_new", config.IP, 5, 3600) {
		return fail(tool.Baro_t("짧은 시간에 너무 많이 신고했습니다. 잠시 뒤에 다시 시도해 주세요."))
	}

	id := tool.Baro_next_id(db, "baro_report")
	priority := baro_mod_priority_of_kind(kind)
	now := tool.Baro_now()

	// 처리 기한 : 긴급은 3일, 나머지는 14일
	deadline_days := 14
	if priority == "0" {
		deadline_days = 3
	}

	note := tool.Baro_json_enc(map[string]string{
		"assignee": "",
		"deadline": tool.Baro_time_add(now, deadline_days, 0, 0),
		"memo":     "",
		"reply":    "",
	})

	tool.Exec_DB(
		db,
		"insert into baro_report (id, kind, target_kind, target, reporter, detail, evidence, status, case_id, date, priority, note) values (?, ?, ?, ?, ?, ?, ?, 'open', '', ?, ?, ?)",
		id, kind, target_kind, target, config.IP,
		tool.Baro_trunc(detail, 8000), tool.Baro_trunc(evidence, 4000),
		now, priority, note,
	)

	// 접수 알림 (신고자에게)
	baro_mod_notify_user(db, config.IP, "case",
		tool.Baro_t("신고가 접수되었습니다 (접수번호 ")+id+")",
		"/baro/report/view/"+tool.Url_parser(id),
		baro_mod_report_kind_name(kind),
	)

	// 공개 로그에는 개인정보 없이 종류만 남긴다 (5번 원칙)
	tool.Baro_public_log(db, "report_new", config.IP, baro_mod_target_kind_name(target_kind), baro_mod_report_kind_name(kind), "접수번호 "+id)
	tool.Baro_audit(db, config, "report_new", target, "", tool.Baro_trunc(detail, 500), baro_mod_report_kind_name(kind))
	tool.Baro_metric_bump(db, "report_new", tool.Baro_today(), 1)

	return tool.Get_redirect("/baro/report/view/" + tool.Url_parser(id))
}

// ------------------------------------------------------------
//  내 신고
// ------------------------------------------------------------

// View_baro_mod_report_mine : 내 신고 목록 (/baro/report/mine)
func View_baro_mod_report_mine(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query
	_ = values

	page := baro_mod_page_num(arg)
	size := tool.Baro_page_size

	body := tool.Baro_crumb([][]string{{"/baro/report", "신고"}, {"", tool.Baro_t("내 신고")}})

	if config.IP == "" {
		body += tool.Baro_alert("warn", tool.Baro_t("접속 정보를 확인할 수 없습니다."))

		return baro_mod_shell(db, config, tool.Baro_t("내 신고"), "/baro/report", body)
	}

	if !tool.Baro_is_user(config.IP) {
		body += tool.Baro_alert("info",
			tool.Baro_t("로그인하지 않은 상태에서는 같은 접속 주소로 접수한 신고만 보입니다. ")+
				tool.Baro_t("접속 주소가 바뀌면 목록이 비어 보일 수 있습니다."))
	}

	rows := tool.Query_DB(
		db,
		"select id, kind, target_kind, target, status, case_id, date, priority, note from baro_report where reporter = ? order by id + 0 desc limit ?, ?",
		config.IP, (page-1)*size, size,
	)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0
	for rows.Next() {
		id, kind, target_kind, target, status, case_id, date, priority, note := "", "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &kind, &target_kind, &target, &status, &case_id, &date, &priority, &note); err != nil {
			continue
		}

		count++

		meta := tool.Baro_json_dec_str(note)
		progress := baro_mod_status_badge(status)
		if meta["reply"] != "" {
			progress += " " + tool.Baro_badge(tool.Baro_t("답변 있음"), "ok")
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/report/view/` + tool.Url_parser(id) + `">` + baro_mod_esc(id) + `</a>`,
			baro_mod_esc(baro_mod_report_kind_name(kind)),
			baro_mod_target_link(db, config, target_kind, target, false),
			progress,
			baro_mod_case_link(db, case_id),
			baro_mod_priority_badge(priority),
			tool.Baro_time_cell(date),
		})
	}

	list_body := tool.Baro_table(
		[]string{"접수번호", "종류", tool.Baro_t("대상"), tool.Baro_t("진행 상태"), "사건", tool.Baro_t("우선순위"), "접수"},
		table_rows,
		"",
	)

	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("접수한 신고가 없습니다."))
	}

	list_body += tool.Baro_paging(page, count, size, "/baro/report/mine/{}")

	body += tool.Baro_card(tool.Baro_t("내가 접수한 신고"), list_body)
	body += tool.Baro_card("", tool.Baro_button("/baro/report/new", tool.Baro_t("새 신고 접수"), "primary")+" "+tool.Baro_button("/baro/appeal/mine", tool.Baro_t("내 이의제기"), ""))
	body += baro_mod_principle_note()

	return baro_mod_shell(db, config, tool.Baro_t("내 신고"), "/baro/report", body)
}

// ------------------------------------------------------------
//  신고 상세
// ------------------------------------------------------------

// View_baro_mod_report_view : 신고 상세 (/baro/report/view/<id>)
func View_baro_mod_report_view(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query

	row, ok := baro_mod_report_get(db, arg)
	if !ok {
		return baro_mod_missing(db, config, "/baro/report", "신고", "/baro/report", tool.Baro_t("신고 화면으로"))
	}

	can_handle := tool.Baro_can(db, config.IP, "report")
	is_reporter := config.IP != "" && row.Reporter == config.IP

	// 신고자 본인과 담당자만 상세를 볼 수 있다
	if !can_handle && !is_reporter {
		return tool.Baro_deny(db, config, tool.Baro_t("이 신고의 상세 내용은 신고자 본인과 담당자만 볼 수 있습니다."))
	}

	if values != nil {
		if !can_handle {
			return tool.Baro_deny(db, config, tool.Baro_t("신고를 처리할 권한이 없습니다."))
		}

		return baro_mod_report_view_post(db, config, row, values)
	}

	meta := baro_mod_report_note(row)

	body := tool.Baro_crumb([][]string{{"/baro/report", "신고"}, {"", "접수번호 " + row.ID}})

	deadline_html := baro_mod_muted(meta["deadline"])
	if meta["deadline"] != "" {
		if tool.Baro_is_past(meta["deadline"]) && row.Status != "closed" && row.Status != "rejected" {
			deadline_html = tool.Baro_badge(tool.Baro_t("기한 초과"), "danger") + " " + baro_mod_esc(meta["deadline"])
		} else {
			deadline_html = baro_mod_esc(meta["deadline"]) + " (" + baro_mod_esc(tool.Baro_duration_left(meta["deadline"])) + ")"
		}
	}

	assignee_html := tool.Baro_t(`<span class="bw_muted">아직 배정되지 않음</span>`)
	if meta["assignee"] != "" {
		assignee_html = tool.Baro_user_link(db, meta["assignee"], config.IP)
	}

	reporter_html := tool.Baro_t(`<span class="bw_muted">비공개</span>`)
	if is_reporter {
		reporter_html = tool.Baro_t("본인")
	} else if can_handle {
		reporter_html = tool.Baro_user_link(db, row.Reporter, config.IP)
	}

	body += tool.Baro_card(tool.Baro_t("신고 정보"), tool.Baro_kv_grid([][]string{
		{"접수번호", baro_mod_esc(row.ID)},
		{"종류", baro_mod_esc(baro_mod_report_kind_name(row.Kind))},
		{tool.Baro_t("대상"), baro_mod_target_kind_name(row.TargetKind) + " · " + baro_mod_target_link(db, config, row.TargetKind, row.Target, false)},
		{tool.Baro_t("진행 상태"), baro_mod_status_badge(row.Status)},
		{tool.Baro_t("우선순위"), baro_mod_priority_badge(row.Priority)},
		{"담당자", assignee_html},
		{tool.Baro_t("처리 기한"), deadline_html},
		{tool.Baro_t("접수 시각"), baro_mod_esc(row.Date) + " (" + baro_mod_esc(tool.Baro_time_ago(row.Date)) + ")"},
		{"신고자", reporter_html},
		{tool.Baro_t("연결된 사건"), baro_mod_case_link(db, row.CaseID)},
	}))

	body += tool.Baro_card(tool.Baro_t("신고 내용"), tool.Baro_code(row.Detail))

	if strings.TrimSpace(row.Evidence) != "" {
		body += tool.Baro_card(tool.Baro_t("증거"), tool.Baro_code(row.Evidence))
	}

	if meta["reply"] != "" {
		body += tool.Baro_card(tool.Baro_t("담당자 답변"), `<div class="bw_card_b">`+baro_mod_esc(meta["reply"])+`</div>`)
	} else if row.Status == "closed" || row.Status == "rejected" {
		body += tool.Baro_alert("info", tool.Baro_t("처리는 끝났지만 별도의 공개 답변은 남아 있지 않습니다."))
	}

	if is_reporter && !can_handle {
		body += tool.Baro_card(tool.Baro_t("신고자 안내"),
			tool.Baro_list([]string{
				tool.Baro_t("진행 상태가 바뀌거나 답변이 달리면 알림을 보내 드립니다."),
				tool.Baro_t("조치에 동의할 수 없으면 사건이 만들어진 뒤 <b>이의제기</b>를 신청할 수 있습니다."),
				tool.Baro_t("신고 내용은 담당자 외에는 볼 수 없습니다."),
			}),
		)
	}

	if can_handle {
		body += baro_mod_report_handle_form(db, config, row, meta)
	}

	body += baro_mod_principle_note()

	return baro_mod_shell(db, config, "신고 "+row.ID, "/baro/report", body)
}

// baro_mod_report_handle_form : 담당자 처리 상자
func baro_mod_report_handle_form(db *sql.DB, config tool.Config, row baro_mod_report_row, meta map[string]string) string {
	body := ""

	// 내부 메모 (공개되지 않는다)
	if meta["memo"] != "" {
		body += tool.Baro_alert("warn", tool.Baro_t("<b>내부 메모</b><br>")+baro_mod_esc(meta["memo"]))
	}

	form := tool.Baro_hidden("act", "update")
	form += tool.Baro_select("status", tool.Baro_t("진행 상태"), baro_mod_status_list(), baro_mod_or(row.Status, "open"), tool.Baro_t("상태를 바꾸면 신고자에게 알림이 갑니다."))
	form += tool.Baro_select("priority", tool.Baro_t("우선순위"), baro_mod_priority_list(), baro_mod_or(row.Priority, "2"), tool.Baro_t("긴급은 큐 맨 위로 올라갑니다."))
	form += tool.Baro_input("assignee", "담당자", meta["assignee"], tool.Baro_t("사용자 이름"), tool.Baro_t("비워 두면 미배정입니다."))
	form += tool.Baro_input_type("deadline", tool.Baro_t("처리 기한"), tool.Baro_date_only(meta["deadline"]), "date", tool.Baro_t("이 날짜를 넘기면 큐에 기한 초과로 표시됩니다."))
	form += tool.Baro_textarea("memo", "내부 메모", meta["memo"], 4, tool.Baro_t("담당자끼리만 보는 기록"), tool.Baro_t("공개되지 않습니다."))
	form += tool.Baro_textarea("reply", "공개 답변", meta["reply"], 4, tool.Baro_t("신고자에게 보여 줄 답변"), tool.Baro_t("신고자가 상세 화면에서 볼 수 있습니다."))
	form += tool.Baro_submit("저장", "primary")

	body += tool.Baro_form("/baro/report/view/"+tool.Url_parser(row.ID), form)

	assign_form := tool.Baro_hidden("act", "assign") + tool.Baro_submit(tool.Baro_t("내가 맡기"), "")
	body += tool.Baro_form("/baro/report/view/"+tool.Url_parser(row.ID), assign_form)

	links := tool.Baro_button("/baro/case/new?report="+tool.Url_parser(row.ID), "이 신고로 사건 만들기", "primary")
	links += " " + tool.Baro_button("/baro/report/merge/"+tool.Url_parser(row.ID), "중복 신고 병합", "")
	if row.TargetKind == "doc" || row.TargetKind == "file" {
		links += " " + tool.Baro_button("/baro/quarantine/new?title="+tool.Url_parser(row.Target)+"&report="+tool.Url_parser(row.ID), "문서 임시 격리", "")
	}
	if row.TargetKind == "user" {
		links += " " + tool.Baro_button("/baro/quarantine/user/"+tool.Url_parser(row.Target), "이 사용자의 문서 다루기", "")
	}

	return tool.Baro_card(tool.Baro_t("담당자 처리"), body) + tool.Baro_card(tool.Baro_t("다음 단계"), links+baro_mod_principle_note())
}

// baro_mod_report_view_post : 신고 상세 처리
func baro_mod_report_view_post(db *sql.DB, config tool.Config, row baro_mod_report_row, values url.Values) string {
	act := baro_mod_val(values, "act")
	meta := baro_mod_report_note(row)
	target_url := "/baro/report/view/" + tool.Url_parser(row.ID)

	switch act {
	case "assign":
		meta["assignee"] = config.IP
		tool.Exec_DB(db, "update baro_report set note = ?, status = ? where id = ?", tool.Baro_json_enc(meta), "pending", row.ID)

		baro_mod_log_both(db, config, "report_assign", "접수번호 "+row.ID, tool.Baro_t("담당자 배정"), "", row.Status, "pending")
		baro_mod_notify_user(db, row.Reporter, "case", tool.Baro_t("신고에 담당자가 배정되었습니다 (접수번호 ")+row.ID+")", target_url, "")

		return tool.Get_redirect(target_url)

	case "update":
		old_status := row.Status
		old_reply := meta["reply"]

		status := baro_mod_val(values, "status")
		if baro_mod_status_name(status) == "-" || status == "" {
			status = old_status
		}

		priority := baro_mod_val(values, "priority")
		if priority == "" {
			priority = baro_mod_or(row.Priority, "2")
		}

		meta["assignee"] = baro_mod_val(values, "assignee")
		meta["memo"] = tool.Baro_trunc(baro_mod_raw(values, "memo"), 4000)
		meta["reply"] = tool.Baro_trunc(baro_mod_raw(values, "reply"), 4000)

		deadline := baro_mod_val(values, "deadline")
		if deadline != "" {
			meta["deadline"] = deadline
		}

		tool.Exec_DB(
			db,
			"update baro_report set status = ?, priority = ?, note = ? where id = ?",
			status, priority, tool.Baro_json_enc(meta), row.ID,
		)

		baro_mod_log_both(db, config, "report_update", "접수번호 "+row.ID, tool.Baro_t("신고 처리"), baro_mod_report_kind_name(row.Kind), old_status, status)

		if status != old_status {
			baro_mod_notify_user(db, row.Reporter, "case",
				tool.Baro_t("신고 진행 상태가 '")+baro_mod_status_name(status)+tool.Baro_t("'(으)로 바뀌었습니다 (접수번호 ")+row.ID+")",
				target_url, "")
		}

		if meta["reply"] != "" && meta["reply"] != old_reply {
			baro_mod_notify_user(db, row.Reporter, "case",
				tool.Baro_t("신고에 담당자 답변이 달렸습니다 (접수번호 ")+row.ID+")", target_url, "")
		}

		return tool.Get_redirect(target_url)
	}

	return tool.Get_redirect(target_url)
}

// ------------------------------------------------------------
//  신고 큐
// ------------------------------------------------------------

// View_baro_mod_report_queue : 신고 큐 (/baro/report/queue)
func View_baro_mod_report_queue(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "report") {
		return tool.Baro_deny(db, config, tool.Baro_t("신고 큐는 신고 처리 권한(순찰자 · 중재자 · 제재 담당)이 있어야 볼 수 있습니다."))
	}

	if values != nil {
		return baro_mod_report_queue_post(db, config, values)
	}

	page := baro_mod_page_num(arg)
	size := tool.Baro_page_size

	status := baro_mod_val(query, "status")
	kind := baro_mod_val(query, "kind")
	sort_key := baro_mod_val(query, "sort")
	if sort_key == "" {
		sort_key = "priority"
	}

	where := " where 1 = 1"
	params := []any{}

	switch status {
	case "":
		where += " and status" + baro_mod_status_open_sql
	case "all":
	case "open":
		// 예전 자료에는 접수 상태가 new 로 적혀 있다
		where += " and status" + baro_mod_status_new_sql
	default:
		where += " and status = ?"
		params = append(params, status)
	}

	// 옛 이름으로 저장된 종류까지 함께 찾는다
	if kind != "" {
		alias := baro_mod_report_kind_alias_all(kind)
		where += " and kind in (" + baro_mod_place_holders(len(alias)) + ")"

		for _, item := range alias {
			params = append(params, item)
		}
	}

	order := " order by priority + 0 asc, id + 0 asc"
	switch sort_key {
	case "new":
		order = " order by id + 0 desc"
	case "old":
		order = " order by id + 0 asc"
	case "kind":
		order = " order by kind asc, priority + 0 asc"
	}

	query_sql := "select id, kind, target_kind, target, status, case_id, date, priority, note from baro_report" + where + order + " limit ?, ?"
	params = append(params, (page-1)*size, size)

	rows := tool.Query_DB(db, query_sql, params...)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0
	unassigned := 0
	overdue := 0

	for rows.Next() {
		id, r_kind, target_kind, target, r_status, case_id, date, priority, note := "", "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &r_kind, &target_kind, &target, &r_status, &case_id, &date, &priority, &note); err != nil {
			continue
		}

		count++
		meta := tool.Baro_json_dec_str(note)

		assignee_html := tool.Baro_t(`<span class="bw_muted">미배정</span>`)
		if meta["assignee"] != "" {
			assignee_html = tool.Baro_user_link(db, meta["assignee"], config.IP)
		} else {
			unassigned++
		}

		deadline_html := `<span class="bw_muted">-</span>`
		if meta["deadline"] != "" {
			if tool.Baro_is_past(meta["deadline"]) && r_status != "closed" && r_status != "rejected" {
				deadline_html = tool.Baro_badge(tool.Baro_t("기한 초과"), "danger")
				overdue++
			} else {
				deadline_html = baro_mod_esc(tool.Baro_duration_left(meta["deadline"]))
			}
		}

		// 같은 대상에 열려 있는 다른 신고 수 (중복 신고 표시)
		dup := 0
		tool.QueryRow_DB(
			db,
			"select count(*) from baro_report where target_kind = ? and target = ? and status"+baro_mod_status_open_sql,
			[]any{&dup}, target_kind, target,
		)

		dup_html := `<span class="bw_muted">-</span>`
		if dup > 1 {
			dup_html = `<a href="/baro/report/merge/` + tool.Url_parser(id) + `">` + tool.Baro_badge(tool.Baro_itoa(dup)+"건 중복", "warn") + `</a>`
		}

		table_rows = append(table_rows, []string{
			`<input type="checkbox" name="target" value="` + baro_mod_esc(id) + `" form="bw_mod_queue_bulk">`,
			baro_mod_priority_badge(priority),
			`<a href="/baro/report/view/` + tool.Url_parser(id) + `">` + baro_mod_esc(id) + `</a>`,
			baro_mod_esc(baro_mod_report_kind_name(r_kind)),
			baro_mod_target_link(db, config, target_kind, target, false),
			dup_html,
			assignee_html,
			deadline_html,
			baro_mod_status_badge(r_status),
			tool.Baro_time_cell(date),
		})
	}

	body := tool.Baro_crumb([][]string{{"/baro/report", "신고"}, {"", tool.Baro_t("신고 큐")}})

	// 전체 현황
	total_open, total_pending := 0, 0
	tool.QueryRow_DB(db, "select count(*) from baro_report where status"+baro_mod_status_new_sql, []any{&total_open})
	tool.QueryRow_DB(db, "select count(*) from baro_report where status = 'pending'", []any{&total_pending})

	body += tool.Baro_card(tool.Baro_t("큐 현황"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("미처리"), tool.Baro_comma(total_open), tool.Baro_t("담당자 배정 전")},
		{tool.Baro_t("처리 중"), tool.Baro_comma(total_pending), tool.Baro_t("담당자 배정됨")},
		{tool.Baro_t("이 쪽 미배정"), tool.Baro_comma(unassigned), tool.Baro_t("지금 화면 기준")},
		{tool.Baro_t("이 쪽 기한 초과"), tool.Baro_comma(overdue), tool.Baro_t("지금 화면 기준")},
	}))

	// 정렬 · 걸러 보기
	filter_form := `<form class="bw_form" method="get" action="/baro/report/queue">`
	filter_form += tool.Baro_select("status", "상태", append([][]string{{"", tool.Baro_t("미처리 + 처리 중")}, {"all", tool.Baro_t("전체")}}, baro_mod_status_list()...), status, "")
	filter_form += tool.Baro_select("kind", "종류", append([][]string{{"", tool.Baro_t("전체")}}, baro_mod_report_kind_options()...), kind, "")
	filter_form += tool.Baro_select("sort", tool.Baro_t("정렬"), [][]string{
		{"priority", tool.Baro_t("긴급도 순 (기본)")},
		{"new", tool.Baro_t("최근 접수 순")},
		{"old", tool.Baro_t("오래된 순")},
		{"kind", tool.Baro_t("유형 순")},
	}, sort_key, "")
	filter_form += tool.Baro_submit(tool.Baro_t("적용"), "")
	filter_form += `</form>`

	body += tool.Baro_card("걸러 보기", filter_form)

	list_body := tool.Baro_table(
		[]string{tool.Baro_t("선택"), tool.Baro_t("우선순위"), "번호", "종류", tool.Baro_t("대상"), tool.Baro_t("중복"), "담당자", "기한", "상태", "접수"},
		table_rows,
		"",
	)
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("조건에 맞는 신고가 없습니다."))
	}

	// 큐에서 바로 담당자를 배정하거나 긴급도를 조정한다 (10-1)
	if count > 0 {
		bulk := `<form class="bw_form" id="bw_mod_queue_bulk" method="post" action="/baro/report/queue">`
		bulk += tool.Baro_note(tool.Baro_t("표에서 신고를 고른 뒤 아래 단추를 누르면 한꺼번에 처리됩니다. 고른 것이 없으면 아무 일도 일어나지 않습니다."))
		bulk += tool.Baro_select("priority", tool.Baro_t("우선순위 일괄 조정"), baro_mod_priority_list(), "1", tool.Baro_t("'긴급도 조정' 을 누를 때만 쓰입니다."))
		bulk += tool.Baro_t(`<button class="bw_btn" type="submit" name="act" value="bulk_assign">고른 신고 내가 맡기</button> `)
		bulk += tool.Baro_t(`<button class="bw_btn bw_btn_primary" type="submit" name="act" value="bulk_priority">고른 신고 긴급도 조정</button>`)
		bulk += `</form>`

		list_body += bulk
	}

	list_url := "/baro/report/queue/{}?status=" + tool.Url_parser(status) + "&kind=" + tool.Url_parser(kind) + "&sort=" + tool.Url_parser(sort_key)
	list_body += tool.Baro_paging(page, count, size, list_url)

	body += tool.Baro_card(tool.Baro_t("신고 큐"), list_body)

	body += tool.Baro_card(tool.Baro_t("처리 원칙"),
		tool.Baro_list([]string{
			tool.Baro_t("같은 대상에 대한 신고가 여러 건이면 하나의 사건으로 묶어 함께 판단합니다."),
			tool.Baro_t("문서 문제는 삭제가 아니라 임시 격리부터 검토합니다."),
			tool.Baro_t("사용자 제재와 문서 처리는 별개의 판단입니다. 제재했다고 문서를 지우지 않습니다."),
			tool.Baro_t("조치 사유와 적용 규정은 공개 로그에 남기고, 신고자 정보와 신고 원문은 남기지 않습니다."),
		}),
	)

	return baro_mod_shell(db, config, tool.Baro_t("신고 큐"), "/baro/report", body)
}

// baro_mod_report_queue_post : 큐에서의 일괄 처리
func baro_mod_report_queue_post(db *sql.DB, config tool.Config, values url.Values) string {
	act := baro_mod_val(values, "act")

	switch act {
	case "bulk_priority":
		priority := baro_mod_priority_alias(baro_mod_val(values, "priority"))
		changed := 0

		for _, id := range values["target"] {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}

			if _, ok := baro_mod_report_get(db, id); !ok {
				continue
			}

			tool.Exec_DB(db, "update baro_report set priority = ? where id = ?", priority, id)
			changed++
		}

		tool.Baro_audit(db, config, "report_bulk_priority", tool.Baro_itoa(changed)+"건", "", priority, "큐 일괄 우선순위 조정")

	case "bulk_assign":
		// 큐에서 바로 담당자를 배정한다 (10-1). 신고 자체는 그대로 남는다.
		changed := 0

		for _, id := range values["target"] {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}

			row, ok := baro_mod_report_get(db, id)
			if !ok {
				continue
			}

			meta := baro_mod_report_note(row)
			meta["assignee"] = config.IP

			status := row.Status
			if baro_mod_status_is_open(status) {
				status = "pending"
			}

			tool.Exec_DB(db, "update baro_report set note = ?, status = ? where id = ?", tool.Baro_json_enc(meta), status, row.ID)
			changed++

			baro_mod_notify_user(db, row.Reporter, "case",
				tool.Baro_t("신고에 담당자가 배정되었습니다 (접수번호 ")+row.ID+")",
				"/baro/report/view/"+tool.Url_parser(row.ID), "")
		}

		tool.Baro_audit(db, config, "report_bulk_assign", tool.Baro_itoa(changed)+"건", "", config.IP, "큐에서 담당자 일괄 배정")
	}

	return tool.Get_redirect("/baro/report/queue")
}

// ------------------------------------------------------------
//  중복 신고 병합
// ------------------------------------------------------------

// View_baro_mod_report_merge : 중복 신고 병합 (/baro/report/merge/<id>)
func View_baro_mod_report_merge(config tool.Config, arg string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = query

	if !tool.Baro_can(db, config.IP, "report") {
		return tool.Baro_deny(db, config, tool.Baro_t("신고 병합은 신고 처리 권한이 있어야 합니다."))
	}

	row, ok := baro_mod_report_get(db, arg)
	if !ok {
		return baro_mod_missing(db, config, "/baro/report", "신고", "/baro/report/queue", tool.Baro_t("신고 큐로"))
	}

	if values != nil {
		return baro_mod_report_merge_post(db, config, row, values)
	}

	// 같은 대상에 대한 다른 신고
	rows := tool.Query_DB(
		db,
		"select id, kind, reporter, status, date, case_id from baro_report where target_kind = ? and target = ? and id <> ? order by id + 0 asc",
		row.TargetKind, row.Target, row.ID,
	)
	defer rows.Close()

	items := ""
	found := 0
	for rows.Next() {
		id, kind, reporter, status, date, case_id := "", "", "", "", "", ""
		if err := rows.Scan(&id, &kind, &reporter, &status, &date, &case_id); err != nil {
			continue
		}

		found++

		label := "접수번호 " + id + " · " + baro_mod_report_kind_name(kind) + " · " + baro_mod_status_name(status) + " · " + tool.Baro_time_ago(date)
		if case_id != "" {
			label += tool.Baro_t(" (이미 사건 연결됨)")
		}

		items += tool.Baro_checkbox("merge", label, id, case_id == "", "신고자 "+tool.Baro_mask_ip(reporter))
	}

	body := tool.Baro_crumb([][]string{{"/baro/report", "신고"}, {"/baro/report/view/" + tool.Url_parser(row.ID), "접수번호 " + row.ID}, {"", "중복 병합"}})

	body += tool.Baro_alert("info",
		tool.Baro_t("같은 대상에 대한 신고를 하나의 사건으로 묶습니다. 신고 자체는 지워지지 않고 사건에 연결됩니다. ")+
			tool.Baro_t("각 신고자는 계속 자기 신고 화면에서 진행 상태를 볼 수 있습니다."))

	body += tool.Baro_card(tool.Baro_t("기준 신고"), tool.Baro_kv_grid([][]string{
		{"접수번호", baro_mod_esc(row.ID)},
		{"종류", baro_mod_esc(baro_mod_report_kind_name(row.Kind))},
		{tool.Baro_t("대상"), baro_mod_target_kind_name(row.TargetKind) + " · " + baro_mod_target_link(db, config, row.TargetKind, row.Target, false)},
		{tool.Baro_t("연결된 사건"), baro_mod_case_link(db, row.CaseID)},
	}))

	if found == 0 {
		body += tool.Baro_card(tool.Baro_t("병합할 신고"), tool.Baro_empty(tool.Baro_t("같은 대상에 대한 다른 신고가 없습니다.")))
		body += tool.Baro_card("", tool.Baro_button("/baro/case/new?report="+tool.Url_parser(row.ID), "이 신고로 사건 만들기", "primary"))

		return baro_mod_shell(db, config, "중복 신고 병합", "/baro/report", body)
	}

	form := items
	form += tool.Baro_input("case_id", tool.Baro_t("묶을 사건"), row.CaseID, tool.Baro_t("사건 번호(BW-...) 또는 내부 id"), tool.Baro_t("비워 두면 기준 신고로 새 사건을 만들어 함께 묶습니다."))
	form += tool.Baro_textarea("reason", tool.Baro_t("병합 사유"), "", 3, tool.Baro_t("같은 대상에 대한 중복 신고"), "")
	form += tool.Baro_submit(tool.Baro_t("선택한 신고 병합"), "primary")

	body += tool.Baro_card("병합할 신고 고르기 ("+tool.Baro_itoa(found)+"건)", tool.Baro_form("/baro/report/merge/"+tool.Url_parser(row.ID), form))
	body += baro_mod_principle_note()

	return baro_mod_shell(db, config, "중복 신고 병합", "/baro/report", body)
}

// baro_mod_report_merge_post : 병합 처리
func baro_mod_report_merge_post(db *sql.DB, config tool.Config, row baro_mod_report_row, values url.Values) string {
	reason := baro_mod_or(baro_mod_val(values, "reason"), tool.Baro_t("같은 대상에 대한 중복 신고"))
	case_key := baro_mod_val(values, "case_id")

	case_row, ok := baro_mod_case_get(db, case_key)
	case_id := ""

	if ok {
		case_id = case_row.ID
	} else {
		// 사건이 지정되지 않았으면 기준 신고로 새 사건을 만든다
		case_id = baro_mod_case_make(db, config, row, "중복 신고 병합", reason)
	}

	merged := []string{row.ID}
	tool.Exec_DB(db, "update baro_report set case_id = ?, status = 'pending' where id = ?", case_id, row.ID)

	for _, id := range values["merge"] {
		id = strings.TrimSpace(id)
		if id == "" || id == row.ID {
			continue
		}

		target, ok := baro_mod_report_get(db, id)
		if !ok {
			continue
		}

		tool.Exec_DB(db, "update baro_report set case_id = ?, status = 'pending' where id = ?", case_id, id)
		merged = append(merged, id)

		baro_mod_notify_user(db, target.Reporter, "case",
			tool.Baro_t("신고가 다른 신고와 함께 하나의 사건으로 묶였습니다 (접수번호 ")+id+")",
			"/baro/report/view/"+tool.Url_parser(id), "")
	}

	tool.Exec_DB(
		db,
		"insert into baro_case_action (id, case_id, actor, action, detail, date, target) values (?, ?, ?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_case_action"), case_id, config.IP, "note",
		"중복 신고 "+tool.Baro_itoa(len(merged))+"건을 이 사건으로 병합했습니다. 사유: "+reason,
		tool.Baro_now(), row.Target,
	)

	baro_mod_log_both(db, config, "report_merge", "사건 "+case_id, reason, tool.Baro_itoa(len(merged))+"건 병합", "", strings.Join(merged, ","))

	return tool.Get_redirect("/baro/case/view/" + tool.Url_parser(case_id))
}
