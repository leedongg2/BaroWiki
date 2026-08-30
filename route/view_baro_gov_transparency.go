package route

import (
	"database/sql"
	"net/url"
	"strconv"
	"strings"
	"time"

	"opennamu/route/tool"
)

// ============================================================
//  11-6 정기 투명성 보고서  (/baro/transparency)
//
//  월별 또는 분기별로 신고 · 삭제 · 복구 · 차단 · 이의제기 결과 ·
//  저작권 요청 · 개인정보 요청 · 보안 사고 · 정책 변경을 집계해 발행한다.
// ============================================================

// baro_gov_report_fields : 보고서 항목 (키, 이름, 묶음, 설명)
func baro_gov_report_fields() [][]string {
	return [][]string{
		{"report_total", tool.Baro_t("접수된 신고"), tool.Baro_t("신고와 사건"), tool.Baro_t("이 기간에 새로 들어온 신고 수")},
		{"report_open", tool.Baro_t("아직 처리 중인 신고"), tool.Baro_t("신고와 사건"), tool.Baro_t("기간 안에 접수돼 아직 닫히지 않은 신고")},
		{"case_total", tool.Baro_t("열린 사건"), tool.Baro_t("신고와 사건"), tool.Baro_t("신고가 사건으로 넘어간 수")},
		{"case_closed", tool.Baro_t("종결된 사건"), tool.Baro_t("신고와 사건"), tool.Baro_t("이 기간에 닫힌 사건")},

		{"quarantine", tool.Baro_t("문서 격리"), tool.Baro_t("문서 조치"), tool.Baro_t("삭제 대신 임시로 감춘 문서")},
		{"release", tool.Baro_t("격리 해제"), tool.Baro_t("문서 조치"), tool.Baro_t("검토 뒤 다시 공개한 문서")},
		{"delete", "삭제", tool.Baro_t("문서 조치"), tool.Baro_t("공개 로그에 남은 삭제 조치")},
		{"restore", tool.Baro_t("복원"), tool.Baro_t("문서 조치"), tool.Baro_t("지워졌다 되살아난 문서")},
		{"protect", tool.Baro_t("문서 보호"), tool.Baro_t("문서 조치"), tool.Baro_t("편집을 제한한 문서")},

		{"block", tool.Baro_t("차단"), tool.Baro_t("제재와 이의제기"), tool.Baro_t("이용자 차단 제재")},
		{"sanction_total", tool.Baro_t("전체 제재"), tool.Baro_t("제재와 이의제기"), tool.Baro_t("경고부터 차단까지 모든 제재")},
		{"appeal_total", "이의제기 접수", tool.Baro_t("제재와 이의제기"), tool.Baro_t("제재에 대한 재심 신청")},
		{"appeal_accept", tool.Baro_t("이의제기 인용"), tool.Baro_t("제재와 이의제기"), tool.Baro_t("재심에서 뒤집힌 건")},
		{"appeal_reject", tool.Baro_t("이의제기 기각"), tool.Baro_t("제재와 이의제기"), tool.Baro_t("재심에서 유지된 건")},

		{"copyright_total", tool.Baro_t("저작권 요청"), tool.Baro_t("법적 요청"), tool.Baro_t("저작권 침해 신고와 반박")},
		{"copyright_done", tool.Baro_t("저작권 요청 처리 완료"), tool.Baro_t("법적 요청"), tool.Baro_t("닫힌 저작권 요청")},
		{"privacy_total", tool.Baro_t("개인정보 요청"), tool.Baro_t("법적 요청"), tool.Baro_t("열람 · 수정 · 삭제 · 내려받기 요청")},
		{"privacy_done", tool.Baro_t("개인정보 요청 처리 완료"), tool.Baro_t("법적 요청"), tool.Baro_t("닫힌 개인정보 요청")},
		{"deletion_total", tool.Baro_t("계정 삭제 요청"), tool.Baro_t("법적 요청"), tool.Baro_t("탈퇴 · 삭제 요청")},

		{"incident_total", tool.Baro_t("보안 사고"), tool.Baro_t("운영과 보안"), tool.Baro_t("기록된 장애 · 보안 사고")},
		{"incident_open", tool.Baro_t("아직 닫히지 않은 사고"), tool.Baro_t("운영과 보안"), tool.Baro_t("해결 기록이 없는 사고")},
		{"policy_change", tool.Baro_t("정책 변경"), tool.Baro_t("운영과 보안"), tool.Baro_t("새로 만들어진 정책 판")},
		{"mass_total", tool.Baro_t("대량 작업"), tool.Baro_t("운영과 보안"), tool.Baro_t("승인 절차를 거친 대량 작업")},
		{"role_change", tool.Baro_t("권한 변경"), tool.Baro_t("운영과 보안"), tool.Baro_t("관리자 역할 부여 · 회수")},
		{"coi_total", tool.Baro_t("이해충돌 회피"), tool.Baro_t("운영과 보안"), tool.Baro_t("운영자가 스스로 빠진 사건")},
		{"log_total", tool.Baro_t("공개 로그 기록"), tool.Baro_t("운영과 보안"), tool.Baro_t("이 기간 공개된 운영 기록 전체")},
	}
}

// baro_gov_report_group_order : 묶음 표시 순서
func baro_gov_report_group_order() []string {
	return []string{tool.Baro_t("신고와 사건"), tool.Baro_t("문서 조치"), tool.Baro_t("제재와 이의제기"), tool.Baro_t("법적 요청"), tool.Baro_t("운영과 보안")}
}

// baro_gov_report_calc : 기간을 집계한다
func baro_gov_report_calc(db *sql.DB, period string) map[string]string {
	start_date, end_date := baro_gov_period_range(period)
	start, end := baro_gov_date_between(start_date, end_date)

	out := map[string]string{}

	set := func(key string, num int) {
		out[key] = tool.Baro_itoa(num)
	}

	set("report_total", baro_gov_count(db, "select count(*) from baro_report where date >= ? and date <= ?", start, end))
	set("report_open", baro_gov_count(db, "select count(*) from baro_report where date >= ? and date <= ? and status != 'closed' and status != 'done' and status != 'rejected'", start, end))
	set("case_total", baro_gov_count(db, "select count(*) from baro_case where date >= ? and date <= ?", start, end))
	set("case_closed", baro_gov_count(db, "select count(*) from baro_case where close_date >= ? and close_date <= ?", start, end))

	set("quarantine", baro_gov_count(db, "select count(*) from baro_quarantine where date >= ? and date <= ?", start, end))
	set("release", baro_gov_count(db, "select count(*) from baro_public_log where kind = 'release' and date >= ? and date <= ?", start, end))
	set("delete", baro_gov_count(db, "select count(*) from baro_public_log where kind = 'delete' and date >= ? and date <= ?", start, end))
	set("restore", baro_gov_count(db, "select count(*) from baro_public_log where kind = 'restore' and date >= ? and date <= ?", start, end))
	set("protect", baro_gov_count(db, "select count(*) from baro_public_log where kind = 'protect' and date >= ? and date <= ?", start, end))

	// 차단 계열 제재 종류 (tool.Baro_sanction_kind_list 의 short_block / long_block / perm_block)
	set("block", baro_gov_count(
		db,
		"select count(*) from baro_sanction where (kind = 'short_block' or kind = 'long_block' or kind = 'perm_block') and date >= ? and date <= ?",
		start, end,
	))
	set("sanction_total", baro_gov_count(db, "select count(*) from baro_sanction where date >= ? and date <= ?", start, end))
	set("appeal_total", baro_gov_count(db, "select count(*) from baro_appeal where date >= ? and date <= ?", start, end))

	accept, reject := 0, 0
	rows := tool.Query_DB(db, "select status, result from baro_appeal where date >= ? and date <= ?", start, end)
	for rows.Next() {
		status, result := "", ""
		if err := rows.Scan(&status, &result); err != nil {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(result))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(status))
		}

		switch {
		case baro_gov_activity_accept_words[key]:
			accept++
		case key == "reject" || key == "rejected" || key == "기각" || key == "keep" || key == "유지":
			reject++
		}
	}
	rows.Close()

	set("appeal_accept", accept)
	set("appeal_reject", reject)

	set("copyright_total", baro_gov_count(db, "select count(*) from baro_copyright_req where date >= ? and date <= ?", start, end))
	set("copyright_done", baro_gov_count(db, "select count(*) from baro_copyright_req where close_date >= ? and close_date <= ?", start, end))
	set("privacy_total", baro_gov_count(db, "select count(*) from baro_privacy_req where date >= ? and date <= ?", start, end))
	set("privacy_done", baro_gov_count(db, "select count(*) from baro_privacy_req where close_date >= ? and close_date <= ?", start, end))
	set("deletion_total", baro_gov_count(db, "select count(*) from baro_deletion_req where date >= ? and date <= ?", start, end))

	set("incident_total", baro_gov_count(db, "select count(*) from baro_incident where date >= ? and date <= ?", start, end))
	set("incident_open", baro_gov_count(db, "select count(*) from baro_incident where date >= ? and date <= ? and (resolve_date = '' or resolve_date is null)", start, end))
	set("policy_change", baro_gov_count(db, "select count(*) from baro_policy where date >= ? and date <= ?", start, end))
	set("mass_total", baro_gov_count(db, "select count(*) from baro_mass_op where created >= ? and created <= ?", start, end))
	set("role_change", baro_gov_count(db, "select count(*) from baro_public_log where (kind = 'role_grant' or kind = 'role_revoke') and date >= ? and date <= ?", start, end))
	set("coi_total", baro_gov_count(db, "select count(*) from baro_coi where date >= ? and date <= ?", start, end))
	set("log_total", baro_gov_count(db, "select count(*) from baro_public_log where date >= ? and date <= ?", start, end))

	out["period"] = period
	out["start"] = start_date
	out["end"] = end_date

	return out
}

// baro_gov_report_html : 집계 결과를 표와 그래프로 그린다
func baro_gov_report_html(report map[string]string) string {
	html := ""

	for _, group := range baro_gov_report_group_order() {
		table_rows := [][]string{}
		chart := [][]string{}

		for _, field := range baro_gov_report_fields() {
			if field[2] != group {
				continue
			}

			value := report[field[0]]
			if value == "" {
				value = "0"
			}

			table_rows = append(table_rows, []string{
				`<b>` + tool.HTML_escape(field[1]) + `</b>`,
				tool.Baro_comma(tool.Str_to_int(value)),
				tool.HTML_escape(field[3]),
			})

			chart = append(chart, []string{field[1], value})
		}

		if len(table_rows) == 0 {
			continue
		}

		body := tool.Baro_table([]string{tool.Baro_t("항목"), tool.Baro_t("건수"), tool.Baro_t("설명")}, table_rows, "")
		body += baro_gov_bar_svg(chart, group+tool.Baro_t(" 집계"))

		html += tool.Baro_card(group, body)
	}

	return html
}

// baro_gov_report_summary : 보고서 맨 위 요약 지표
func baro_gov_report_summary(report map[string]string) string {
	get := func(key string) string {
		return tool.Baro_comma(tool.Str_to_int(report[key]))
	}

	appeal_total := tool.Str_to_int(report["appeal_total"])
	accept := tool.Str_to_int(report["appeal_accept"])

	rate := tool.Baro_t("이의제기 없음")
	if appeal_total > 0 {
		rate = tool.Baro_pct_text(accept, appeal_total) + " 인용"
	}

	return tool.Baro_stat_row([][]string{
		{"신고", get("report_total"), "접수"},
		{"삭제", get("delete"), tool.Baro_t("복원 ") + get("restore") + tool.Baro_t("건")},
		{"격리", get("quarantine"), "해제 " + get("release") + tool.Baro_t("건")},
		{tool.Baro_t("차단"), get("block"), tool.Baro_t("전체 제재 ") + get("sanction_total") + tool.Baro_t("건")},
		{tool.Baro_t("이의제기"), get("appeal_total"), rate},
		{tool.Baro_t("법적 요청"), tool.Baro_comma(tool.Str_to_int(report["copyright_total"]) + tool.Str_to_int(report["privacy_total"])), tool.Baro_t("저작권 + 개인정보")},
		{tool.Baro_t("보안 사고"), get("incident_total"), tool.Baro_t("미해결 ") + get("incident_open") + tool.Baro_t("건")},
		{tool.Baro_t("정책 변경"), get("policy_change"), tool.Baro_t("새 판")},
	})
}

// baro_gov_transparency_notice : 투명성 보고서 안내
func baro_gov_transparency_notice() string {
	return tool.Baro_alert("info",
		tool.Baro_t(`투명성 보고서는 정해진 기간 동안 <b>운영이 얼마나, 어떤 방향으로 이루어졌는지</b>를 숫자로 공개하는 문서입니다.`)+
			tool.Baro_t(` 개별 사건의 당사자나 신고 내용은 담지 않고, 집계된 건수만 담습니다.`),
	)
}

// View_baro_gov_transparency_list : 발행된 보고서 목록 + 최신 보고서
func View_baro_gov_transparency_list(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page := baro_gov_page_num(baro_gov_q(query, "page"))

	total := baro_gov_count(db, "select count(*) from baro_transparency")

	size := tool.Baro_page_size
	rows := tool.Query_DB(db, "select id, period, date, publisher from baro_transparency order by id + 0 desc limit ?, ?", (page-1)*size, size)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0
	latest_id := ""

	for rows.Next() {
		id, period, date, publisher := "", "", "", ""
		if err := rows.Scan(&id, &period, &date, &publisher); err != nil {
			continue
		}

		count++
		if latest_id == "" {
			latest_id = id
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/transparency/view/` + tool.Url_parser(id) + `"><b>` + tool.HTML_escape(baro_gov_period_name(period)) + `</b></a>`,
			tool.HTML_escape(period),
			tool.Baro_time_cell(date),
			baro_gov_actor_html(db, publisher, config.IP),
			tool.Baro_button("/baro/transparency/view/"+tool.Url_parser(id), "보고서 보기", ""),
		})
	}

	can_make := tool.Baro_can(db, config.IP, "policy")

	body := baro_gov_css()
	body += tool.Baro_hero(tool.Baro_t("투명성 보고서"), tool.Baro_t("운영이 어떻게 이루어졌는지 기간별로 모아 공개합니다."), "")
	body += baro_gov_tabs("/baro/transparency")
	body += baro_gov_transparency_notice()

	list_body := ""
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("아직 발행된 투명성 보고서가 없습니다. 기간을 골라 첫 보고서를 발행해 보세요."))
	} else {
		list_body = tool.Baro_table([]string{"기간", tool.Baro_t("기간 키"), tool.Baro_t("발행 시각"), tool.Baro_t("발행자"), ""}, table_rows, "")
	}

	list_body += tool.Baro_paging(page, count, size, baro_gov_url("/baro/transparency", [][]string{{"page", "{}"}}))

	foot := ""
	if can_make {
		foot = tool.Baro_button("/baro/transparency/make", tool.Baro_t("새 보고서 발행"), "primary")
	} else {
		foot = tool.Baro_note(tool.Baro_t("보고서 발행은 정책 권한(시스템 관리자 · 중재자)을 가진 사람만 할 수 있습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("발행된 보고서"), tool.Baro_t("전체 ")+tool.Baro_comma(total)+tool.Baro_t("건"), list_body, foot)

	// 최신 보고서 본문
	if latest_id != "" && page == 1 {
		period, data, date, publisher := "", "", "", ""
		if tool.QueryRow_DB(
			db,
			"select period, data, date, publisher from baro_transparency where id = ?",
			[]any{&period, &data, &date, &publisher},
			latest_id,
		) {
			report := tool.Baro_json_dec_str(data)

			body += tool.Baro_card_ex(
				tool.Baro_t("최신 보고서 — ")+baro_gov_period_name(period),
				tool.Baro_t("발행 ")+date+tool.Baro_t(" · 발행자 ")+publisher,
				baro_gov_report_summary(report)+baro_gov_report_html(report),
				tool.Baro_button("/baro/transparency/view/"+tool.Url_parser(latest_id), "이 보고서만 크게 보기", ""),
			)
		}
	}

	// 아직 보고서가 없는 최근 기간 안내
	body += tool.Baro_card(tool.Baro_t("이번 달 잠정 집계"), baro_gov_transparency_draft(db, tool.Baro_today()[:7]))

	return tool.Baro_page(db, config, tool.Baro_t("투명성 보고서"), body, baro_gov_menu())
}

// baro_gov_transparency_draft : 아직 발행하지 않은 기간의 잠정 집계
func baro_gov_transparency_draft(db *sql.DB, period string) string {
	report := baro_gov_report_calc(db, period)

	html := tool.Baro_note(tool.Baro_t("아직 발행되지 않은 ") + baro_gov_period_name(period) + tool.Baro_t(" 의 잠정 집계입니다. 기간이 끝나면 정식으로 발행할 수 있습니다."))
	html += baro_gov_report_summary(report)

	return html
}

// View_baro_gov_transparency_view : 보고서 하나
func View_baro_gov_transparency_view(config tool.Config, id string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	id = strings.TrimSpace(id)

	period, data, date, publisher := "", "", "", ""
	found := false

	if id != "" {
		found = tool.QueryRow_DB(
			db,
			"select period, data, date, publisher from baro_transparency where id = ?",
			[]any{&period, &data, &date, &publisher},
			id,
		)
	}

	if !found {
		body := baro_gov_css()
		body += tool.Baro_crumb([][]string{{"/baro/transparency", tool.Baro_t("투명성 보고서")}, {"", tool.Baro_t("찾을 수 없음")}})
		body += tool.Baro_alert("warn", tool.Baro_t("그런 번호의 투명성 보고서가 없습니다."))
		body += tool.Baro_card(tool.Baro_t("보고서 찾기"),
			tool.Baro_empty("번호 "+tool.Baro_trunc(id, 20)+tool.Baro_t(" 에 해당하는 보고서가 없습니다."))+
				tool.Baro_button("/baro/transparency", tool.Baro_t("보고서 목록으로"), "primary"),
		)

		return tool.Baro_page(db, config, tool.Baro_t("투명성 보고서"), body, baro_gov_menu())
	}

	report := tool.Baro_json_dec_str(data)

	body := baro_gov_css()
	body += tool.Baro_crumb([][]string{{"/baro/transparency", tool.Baro_t("투명성 보고서")}, {"", baro_gov_period_name(period)}})
	body += tool.Baro_hero(baro_gov_period_name(period)+tool.Baro_t(" 투명성 보고서"), tool.Baro_t("발행 ")+date, "")
	body += baro_gov_tabs("/baro/transparency")
	body += baro_gov_transparency_notice()

	body += tool.Baro_card(tool.Baro_t("보고서 정보"), tool.Baro_kv_grid([][]string{
		{"기간", tool.HTML_escape(baro_gov_period_name(period))},
		{tool.Baro_t("집계 범위"), tool.HTML_escape(report["start"]) + " ~ " + tool.HTML_escape(report["end"])},
		{tool.Baro_t("발행 시각"), tool.HTML_escape(date)},
		{tool.Baro_t("발행자"), baro_gov_actor_html(db, publisher, config.IP)},
		{tool.Baro_t("보고서 번호"), tool.HTML_escape(id)},
	}))

	body += tool.Baro_card(tool.Baro_t("한눈에 보기"), baro_gov_report_summary(report))
	body += baro_gov_report_html(report)

	body += tool.Baro_card(tool.Baro_t("이 보고서를 어떻게 읽나요"), tool.Baro_list([]string{
		tool.Baro_t("숫자가 늘었다고 곧바로 운영이 나빠진 것은 아닙니다. 위키가 커지면 신고도 늘어납니다."),
		tool.Baro_t("<b>삭제 대비 복원 비율</b>과 <b>격리 대비 해제 비율</b>은 삭제보다 보존을 우선한다는 원칙이 지켜지는지를 보여 줍니다."),
		tool.Baro_t("<b>이의제기 인용률</b>이 높으면 제재 기준을 다시 살펴야 합니다."),
		tool.Baro_t("개별 사건의 당사자와 신고 원문은 이 보고서에 담지 않습니다."),
	}))

	body += tool.Baro_button("/baro/transparency", tool.Baro_t("보고서 목록으로"), "") + " " +
		tool.Baro_button(baro_gov_url("/baro/log", [][]string{{"from", report["start"]}, {"to", report["end"]}}), tool.Baro_t("이 기간 공개 로그 보기"), "")

	return tool.Baro_page(db, config, baro_gov_period_name(period)+tool.Baro_t(" 투명성 보고서"), body, baro_gov_menu())
}

// View_baro_gov_transparency_make : 기간을 골라 집계해 발행
func View_baro_gov_transparency_make(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/transparency/make"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "policy") {
		return tool.Baro_deny(db, config, tool.Baro_t("투명성 보고서 발행은 정책 권한(시스템 관리자 · 중재자)을 가진 사람만 할 수 있습니다."))
	}

	now := time.Now()
	year := strconv.Itoa(now.Year())
	kind := "month"
	month := baro_gov_two(int(now.Month()))
	quarter := strconv.Itoa((int(now.Month())-1)/3 + 1)
	alert := ""

	if q := baro_gov_q(query, "year"); q != "" {
		year = q
	}
	if q := baro_gov_q(query, "kind"); q != "" {
		kind = q
	}
	if q := baro_gov_q(query, "month"); q != "" {
		month = q
	}
	if q := baro_gov_q(query, "quarter"); q != "" {
		quarter = q
	}

	if values != nil {
		if v := strings.TrimSpace(values.Get("year")); v != "" {
			year = v
		}
		if v := strings.TrimSpace(values.Get("kind")); v != "" {
			kind = v
		}
		if v := strings.TrimSpace(values.Get("month")); v != "" {
			month = v
		}
		if v := strings.TrimSpace(values.Get("quarter")); v != "" {
			quarter = v
		}
	}

	// 주소나 폼으로 들어온 값을 그대로 믿지 않는다.
	// 형식이 어긋난 기간은 집계 범위가 통째로 넓어져 엉뚱한 숫자가 발행되기 때문이다.
	if kind != "quarter" {
		kind = "month"
	}

	year_num := tool.Str_to_int(year)
	if year_num < 2000 || year_num > 2200 {
		year_num = now.Year()
	}
	year = strconv.Itoa(year_num)

	month_num := tool.Str_to_int(month)
	if month_num < 1 || month_num > 12 {
		month_num = int(now.Month())
	}
	month = baro_gov_two(month_num)

	quarter_num := tool.Str_to_int(quarter)
	if quarter_num < 1 || quarter_num > 4 {
		quarter_num = (int(now.Month())-1)/3 + 1
	}
	quarter = strconv.Itoa(quarter_num)

	period := year + "-" + month
	if kind == "quarter" {
		period = year + "-Q" + quarter
	}

	if values != nil && strings.TrimSpace(values.Get("action")) == "publish" {
		exist_id := ""
		duplicated := tool.QueryRow_DB(db, "select id from baro_transparency where period = ? limit 1", []any{&exist_id}, period)
		replace := strings.TrimSpace(values.Get("replace")) != ""

		switch {
		case tool.Baro_read_only(db):
			alert = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.HTML_escape(tool.Baro_read_only_reason(db)))
		case duplicated && !replace:
			alert = tool.Baro_alert("warn",
				tool.HTML_escape(baro_gov_period_name(period))+tool.Baro_t(" 보고서가 이미 발행돼 있습니다. 다시 집계해 갈아 끼우려면 아래 <b>이미 있는 보고서 갈아 끼우기</b>를 체크하세요. ")+
					tool.Baro_button("/baro/transparency/view/"+tool.Url_parser(exist_id), "이미 있는 보고서 보기", ""),
			)
		default:
			report := baro_gov_report_calc(db, period)
			data := tool.Baro_json_enc(report)
			now_text := tool.Baro_now()

			if duplicated {
				tool.Exec_DB(db, "update baro_transparency set data = ?, date = ?, publisher = ? where id = ?", data, now_text, config.IP, exist_id)
				tool.Baro_audit(db, config, "transparency_replace", period, exist_id, tool.Baro_trunc(data, 1500), "보고서 재발행")
			} else {
				exist_id = tool.Baro_next_id(db, "baro_transparency")
				tool.Exec_DB(
					db,
					"insert into baro_transparency (id, period, data, date, publisher) values (?, ?, ?, ?, ?)",
					exist_id, period, data, now_text, config.IP,
				)
				tool.Baro_audit(db, config, "transparency_publish", period, "", tool.Baro_trunc(data, 1500), "보고서 발행")
			}

			tool.Baro_public_log(
				db, "transparency", config.IP, baro_gov_period_name(period),
				"정기 투명성 보고서 발행",
				tool.Baro_json_enc(map[string]string{
					"기간":   baro_gov_period_name(period),
					"보고서":  exist_id,
					"공개로그": report["log_total"],
				}),
			)

			baro_gov_transparency_notify(db, period, exist_id, config.IP)

			return tool.Get_redirect("/baro/transparency/view/" + tool.Url_parser(exist_id))
		}
	}

	report := baro_gov_report_calc(db, period)

	body := baro_gov_css()
	body += tool.Baro_crumb([][]string{{"/baro/transparency", tool.Baro_t("투명성 보고서")}, {"", tool.Baro_t("새 보고서 발행")}})
	body += tool.Baro_hero(tool.Baro_t("투명성 보고서 발행"), tool.Baro_t("기간을 고르면 그 기간을 집계해 미리 보여 줍니다."), "")
	body += baro_gov_tabs("/baro/transparency")
	body += alert
	body += baro_gov_transparency_notice()

	month_options := [][]string{}
	for i := 1; i <= 12; i++ {
		month_options = append(month_options, []string{baro_gov_two(i), strconv.Itoa(i) + tool.Baro_t("월")})
	}

	quarter_options := [][]string{
		{"1", tool.Baro_t("1분기 (1~3월)")},
		{"2", tool.Baro_t("2분기 (4~6월)")},
		{"3", tool.Baro_t("3분기 (7~9월)")},
		{"4", tool.Baro_t("4분기 (10~12월)")},
	}

	pick := `<div class="bw_gov_filter">`
	pick += tool.Baro_select("kind", tool.Baro_t("주기"), [][]string{{"month", tool.Baro_t("월별")}, {"quarter", tool.Baro_t("분기별")}}, kind, "")
	pick += tool.Baro_input("year", tool.Baro_t("연도"), year, "2026", "")
	pick += tool.Baro_select("month", tool.Baro_t("월 (월별일 때)"), month_options, month, "")
	pick += tool.Baro_select("quarter", tool.Baro_t("분기 (분기별일 때)"), quarter_options, quarter, "")
	pick += `<div class="bw_gov_filter_btn">` + tool.Baro_submit(tool.Baro_t("이 기간 미리 보기"), "") + `</div>`
	pick += `</div>`

	body += tool.Baro_card_ex(tool.Baro_t("기간 고르기"), tool.Baro_t("지금 고른 기간 : ")+baro_gov_period_name(period)+" ("+report["start"]+" ~ "+report["end"]+")", baro_gov_get_form("/baro/transparency/make", pick), "")

	body += tool.Baro_card(tool.Baro_t("미리 보기 — 한눈에 보기"), baro_gov_report_summary(report))
	body += baro_gov_report_html(report)

	publish := tool.Baro_hidden("action", "publish")
	publish += tool.Baro_hidden("kind", kind)
	publish += tool.Baro_hidden("year", year)
	publish += tool.Baro_hidden("month", month)
	publish += tool.Baro_hidden("quarter", quarter)
	publish += tool.Baro_checkbox("replace", tool.Baro_t("이미 있는 보고서 갈아 끼우기"), "on", false, tool.Baro_t("같은 기간 보고서가 이미 있으면 다시 집계해 덮어씁니다."))
	publish += tool.Baro_submit(baro_gov_period_name(period)+" 보고서 발행", "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("발행"),
		tool.Baro_t("발행하면 baro_transparency 에 저장되고 공개 운영 로그에 남습니다."),
		tool.Baro_form("/baro/transparency/make", publish),
		tool.Baro_note(tool.Baro_t("발행한 보고서는 누구나 볼 수 있습니다. 집계된 건수만 담기므로 개별 사건의 당사자는 드러나지 않습니다.")),
	)

	return tool.Baro_page(db, config, tool.Baro_t("투명성 보고서 발행"), body, baro_gov_menu())
}

// baro_gov_transparency_notify : 보고서 발행을 운영 역할자에게 알린다
func baro_gov_transparency_notify(db *sql.DB, period string, id string, actor string) {
	rows := tool.Query_DB(db, "select distinct user from baro_role")
	defer rows.Close()

	users := []string{}
	for rows.Next() {
		user := ""
		if err := rows.Scan(&user); err != nil {
			continue
		}

		if user == "" || user == actor {
			continue
		}

		users = append(users, user)
	}

	for _, user := range users {
		tool.Baro_notify(
			db, user, "system",
			baro_gov_period_name(period)+tool.Baro_t(" 투명성 보고서가 발행됐습니다."),
			"/baro/transparency/view/"+tool.Url_parser(id),
			"",
		)
	}
}
