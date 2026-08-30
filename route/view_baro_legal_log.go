package route

import (
	"database/sql"
	"net/url"

	"opennamu/route/tool"
)

// ============================================================
//  15-5 법적 · 권리 요청 로그
//
//  URL : /baro/legal , /baro/legal/make
//
//  개인정보를 뺀 건수만 공개한다.
//  신고자 · 대상자의 이름과 문서명은 이 화면 어디에도 나오지 않는다. (원칙 5)
// ============================================================

// baro_legal_log_kind_list : 집계 항목 (키, 이름, 설명)
func baro_legal_log_kind_list() [][]string {
	return [][]string{
		{"copyright_req", tool.Baro_t("저작권 요청 접수"), tool.Baro_t("기간 안에 새로 들어온 저작권 신고 수")},
		{"copyright_done", tool.Baro_t("저작권 침해 인정"), tool.Baro_t("기간 안에 침해로 결정된 건수")},
		{"copyright_rejected", tool.Baro_t("저작권 요청 기각"), tool.Baro_t("기간 안에 기각된 건수")},
		{"copyright_counter", tool.Baro_t("작성자 소명"), tool.Baro_t("기간 안에 접수된 반론 수")},
		{"privacy_req", "개인정보 요청 접수", tool.Baro_t("열람 · 수정 · 다운로드 · 삭제 요청 수")},
		{"privacy_done", tool.Baro_t("개인정보 요청 처리 완료"), tool.Baro_t("기간 안에 처리를 끝낸 건수")},
		{"privacy_rejected", tool.Baro_t("개인정보 요청 기각"), tool.Baro_t("기간 안에 기각된 건수")},
		{"deletion_req", "계정 탈퇴 요청 접수", tool.Baro_t("기간 안에 들어온 탈퇴 요청 수")},
		{"deletion_done", tool.Baro_t("계정 탈퇴 요청 처리 완료"), tool.Baro_t("기간 안에 처리를 끝낸 건수")},
		{"quarantine_open", tool.Baro_t("저작권 사유 임시 격리"), tool.Baro_t("기간 안에 새로 격리된 대상 수")},
		{"quarantine_release", tool.Baro_t("격리 해제"), tool.Baro_t("기간 안에 격리가 풀린 대상 수")},
	}
}

// baro_legal_log_kind_name : 집계 항목 이름
func baro_legal_log_kind_name(kind string) string {
	for _, row := range baro_legal_log_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// baro_legal_metric_month : 지표 표에서 한 달 치를 더한다
func baro_legal_metric_month(db *sql.DB, key_name string, period string) int {
	total := 0

	rows := tool.Query_DB(db, "select value from baro_metric where key_name = ? and period like ?", key_name, period+"%")
	defer rows.Close()

	for rows.Next() {
		value := ""
		if err := rows.Scan(&value); err != nil {
			continue
		}

		total += tool.Str_to_int(value)
	}

	return total
}

// baro_legal_log_measure : 기간의 집계 값을 실제 표에서 센다
func baro_legal_log_measure(db *sql.DB, period string) map[string]int {
	like := period + "%"

	out := map[string]int{}

	out["copyright_req"] = baro_legal_count(db, "select count(*) from baro_copyright_req where date like ?", like)
	out["copyright_done"] = baro_legal_count(db, "select count(*) from baro_copyright_req where status = 'done' and close_date like ?", like)
	out["copyright_rejected"] = baro_legal_count(db, "select count(*) from baro_copyright_req where status = 'rejected' and close_date like ?", like)
	out["copyright_counter"] = baro_legal_metric_month(db, "copyright_counter", period)

	out["privacy_req"] = baro_legal_count(db, "select count(*) from baro_privacy_req where date like ?", like)
	out["privacy_done"] = baro_legal_count(db, "select count(*) from baro_privacy_req where status = 'done' and close_date like ?", like)
	out["privacy_rejected"] = baro_legal_count(db, "select count(*) from baro_privacy_req where status = 'rejected' and close_date like ?", like)

	out["deletion_req"] = baro_legal_count(db, "select count(*) from baro_deletion_req where date like ?", like)
	out["deletion_done"] = baro_legal_count(db, "select count(*) from baro_deletion_req where status = 'done' and close_date like ?", like)

	out["quarantine_open"] = baro_legal_count(db, "select count(*) from baro_quarantine where kind = 'copyright' and date like ?", like)
	out["quarantine_release"] = baro_legal_count(db, "select count(*) from baro_quarantine where kind = 'copyright' and release_date like ?", like)

	return out
}

// baro_legal_log_status_dist : 상태별 분포 (표 이름 / 날짜 컬럼 기준)
func baro_legal_log_status_dist(db *sql.DB, table_name string, period string) [][]string {
	out := [][]string{}

	for _, row := range baro_legal_status_list() {
		count := baro_legal_count(
			db,
			"select count(*) from "+table_name+" where status = ? and date like ?",
			row[0], period+"%",
		)

		out = append(out, []string{row[0], row[1], tool.Baro_itoa(count)})
	}

	// 상태 값이 비어 있는 옛 기록도 접수됨으로 센다
	empty := baro_legal_count(db, "select count(*) from "+table_name+" where status = '' and date like ?", period+"%")
	if empty > 0 && len(out) > 0 {
		out[0][2] = tool.Baro_itoa(tool.Str_to_int(out[0][2]) + empty)
	}

	return out
}

// baro_legal_log_dist_html : 분포를 막대와 표로 그린다
func baro_legal_log_dist_html(dist [][]string) string {
	total := 0
	for _, row := range dist {
		total += tool.Str_to_int(row[2])
	}

	if total < 1 {
		return tool.Baro_empty(tool.Baro_t("이 기간에 접수된 요청이 없습니다."))
	}

	html := ""
	table_rows := [][]string{}

	for _, row := range dist {
		count := tool.Str_to_int(row[2])
		if count > 0 {
			html += tool.Baro_progress(count, total, row[1])
		}

		table_rows = append(table_rows, []string{
			baro_legal_status_badge(row[0]),
			tool.Baro_comma(count) + tool.Baro_t("건"),
			tool.Baro_pct_text(count, total),
		})
	}

	return html + tool.Baro_table([]string{"상태", tool.Baro_t("건수"), tool.Baro_t("비율")}, table_rows, "")
}

// ------------------------------------------------------------
//  15-5 통계 화면 : /baro/legal
// ------------------------------------------------------------

// View_baro_legal_log : 법적 · 권리 요청 통계
func View_baro_legal_log(config tool.Config, period string, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	period = baro_legal_period_norm(period)
	page_num := baro_legal_page_num(page)
	size := tool.Baro_page_size

	measure := baro_legal_log_measure(db, period)

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"", tool.Baro_t("법적 · 권리 요청 통계")},
	})

	body += tool.Baro_hero(
		tool.Baro_t("법적 · 권리 요청 통계"),
		period+tool.Baro_t(" 기간의 처리 현황입니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("저작권 요청"), tool.Baro_comma(measure["copyright_req"]), tool.Baro_t("기간 중 접수")},
			{tool.Baro_t("개인정보 요청"), tool.Baro_comma(measure["privacy_req"]), tool.Baro_t("기간 중 접수")},
			{tool.Baro_t("삭제(탈퇴) 요청"), tool.Baro_comma(measure["deletion_req"]), tool.Baro_t("기간 중 접수")},
			{tool.Baro_t("임시 격리"), tool.Baro_comma(measure["quarantine_open"]), tool.Baro_t("기간 중 격리")},
			{tool.Baro_t("격리 해제"), tool.Baro_comma(measure["quarantine_release"]), tool.Baro_t("기간 중 복구")},
		}),
	)

	body += tool.Baro_alert(
		"info",
		tool.Baro_t("이 통계에는 신고자 · 대상자의 이름이나 문서 제목이 들어가지 않습니다. 건수만 공개합니다. (원칙 5)"),
	)

	// 기간 이동
	prev := baro_legal_month_add(period, -1)
	next := baro_legal_month_add(period, 1)

	body += tool.Baro_card(
		tool.Baro_t("기간 고르기"),
		tool.Baro_button("/baro/legal?period="+tool.Url_parser(prev), "이전 달 ("+prev+")", "")+" "+
			tool.Baro_button("/baro/legal?period="+tool.Url_parser(tool.Get_month()), "이번 달", "primary")+" "+
			tool.Baro_button("/baro/legal?period="+tool.Url_parser(next), "다음 달 ("+next+")", ""),
	)

	// 항목별 집계
	measure_rows := [][]string{}
	for _, row := range baro_legal_log_kind_list() {
		count := measure[row[0]]

		badge := tool.Baro_badge(tool.Baro_comma(count)+tool.Baro_t("건"), "muted")
		if count > 0 {
			badge = tool.Baro_badge(tool.Baro_comma(count)+tool.Baro_t("건"), "info")
		}

		measure_rows = append(measure_rows, []string{
			tool.HTML_escape(row[1]),
			badge,
			tool.HTML_escape(row[2]),
		})
	}

	body += tool.Baro_card_ex(
		period+tool.Baro_t(" 항목별 집계"),
		tool.Baro_t("지금 데이터베이스에서 바로 센 값입니다."),
		tool.Baro_table([]string{tool.Baro_t("항목"), tool.Baro_t("건수"), tool.Baro_t("설명")}, measure_rows, ""),
		"",
	)

	// 처리 결과 분포
	body += tool.Baro_grid([]string{
		tool.Baro_card_ex(
			tool.Baro_t("저작권 요청 처리 결과"),
			period+tool.Baro_t(" 접수분 기준"),
			baro_legal_log_dist_html(baro_legal_log_status_dist(db, "baro_copyright_req", period)),
			"",
		),
		tool.Baro_card_ex(
			tool.Baro_t("개인정보 요청 처리 결과"),
			period+tool.Baro_t(" 접수분 기준"),
			baro_legal_log_dist_html(baro_legal_log_status_dist(db, "baro_privacy_req", period)),
			"",
		),
		tool.Baro_card_ex(
			tool.Baro_t("계정 탈퇴 요청 처리 결과"),
			period+tool.Baro_t(" 접수분 기준"),
			baro_legal_log_dist_html(baro_legal_log_status_dist(db, "baro_deletion_req", period)),
			"",
		),
	}, 3)

	// 저장된 집계 기록
	list_rows := [][]string{}
	count := 0

	rows := tool.Query_DB(
		db,
		"select id, kind, num, period, date, note from baro_legal_log order by id + 0 desc limit "+tool.Baro_itoa(size)+" offset "+tool.Baro_itoa((page_num-1)*size),
	)
	defer rows.Close()

	for rows.Next() {
		id, kind, num, log_period, date, note := "", "", "", "", "", ""
		if err := rows.Scan(&id, &kind, &num, &log_period, &date, &note); err != nil {
			continue
		}

		count++

		list_rows = append(list_rows, []string{
			tool.HTML_escape(id),
			tool.HTML_escape(log_period),
			tool.HTML_escape(baro_legal_log_kind_name(kind)),
			tool.Baro_comma(tool.Str_to_int(num)) + tool.Baro_t("건"),
			tool.HTML_escape(note),
			tool.Baro_time_cell(date),
		})
	}

	list_html := tool.Baro_table(
		[]string{"번호", "기간", tool.Baro_t("항목"), tool.Baro_t("건수"), tool.Baro_t("메모"), tool.Baro_t("집계 시각")},
		list_rows,
		tool.Baro_t("저장된 집계가 없습니다."),
	)

	if count == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("아직 저장된 기간별 집계가 없습니다. 담당자가 집계를 만들면 여기에 남습니다."))
	}

	list_html += tool.Baro_paging(page_num, count, size, "/baro/legal/page/{}?period="+tool.Url_parser(period))

	foot := tool.Baro_note(tool.Baro_t("집계는 baro_legal_log 에 저장됩니다. 저장된 값은 나중에 표가 정리되어도 그대로 남습니다."))
	if tool.Baro_can(db, config.IP, "privacy") {
		foot += tool.Baro_button("/baro/legal/make?period="+tool.Url_parser(period), "이 기간 집계 만들기", "primary")
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("저장된 기간별 집계"),
		tool.Baro_t("과거 기록을 그대로 보존하기 위해 기간이 끝나면 집계를 따로 저장합니다."),
		list_html,
		foot,
	)

	body += tool.Baro_card(
		tool.Baro_t("이 통계를 읽는 방법"),
		tool.Baro_list([]string{
			tool.Baro_t("<strong>접수 수</strong>가 늘었다고 위키가 나빠진 것은 아닙니다. 신고 창구를 찾기 쉬워졌다는 뜻일 수도 있습니다."),
			tool.Baro_t("<strong>기각 비율</strong>이 아주 높으면 신고 양식이 헷갈리게 되어 있을 수 있습니다."),
			tool.Baro_t("<strong>격리 해제 수</strong>는 잘못된 격리를 되돌린 횟수입니다. 이 값이 0 이라고 좋은 것은 아닙니다."),
			tool.Baro_t("<strong>처리 완료까지 걸린 기간</strong>은 각 요청 상세의 접수 · 처리 시각으로 확인할 수 있습니다."),
		}),
	)

	return baro_legal_page(db, config, tool.Baro_t("법적 · 권리 요청 통계"), "/baro/legal", body)
}

// ------------------------------------------------------------
//  15-5 집계 만들기 : /baro/legal/make
// ------------------------------------------------------------

// View_baro_legal_make : 기간별 집계를 baro_legal_log 에 저장 (권한 privacy)
func View_baro_legal_make(config tool.Config, period string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "privacy") {
		return tool.Baro_deny(db, config, tool.Baro_t("기간별 집계 저장은 개인정보·법적 요청 담당(privacy) 역할만 할 수 있습니다."))
	}

	message := ""

	if values != nil {
		period = baro_legal_form_get(values, "period")
		period = baro_legal_period_norm(period)

		switch {
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 집계를 저장할 수 없습니다."))
		case !baro_legal_form_on(values, "agree"):
			message = tool.Baro_alert("danger", tool.Baro_t("같은 기간의 기존 집계를 다시 만든다는 확인이 필요합니다."))
		default:
			measure := baro_legal_log_measure(db, period)
			note := baro_legal_form_get(values, "note")

			// 같은 기간의 집계는 지우고 다시 만든다
			tool.Exec_DB(db, "delete from baro_legal_log where period = ?", period)

			total := 0
			for _, row := range baro_legal_log_kind_list() {
				num := measure[row[0]]
				total += num

				tool.Exec_DB(
					db,
					"insert into baro_legal_log (id, kind, num, period, date, note) values (?, ?, ?, ?, ?, ?)",
					tool.Baro_next_id(db, "baro_legal_log"), row[0], tool.Baro_itoa(num), period, tool.Baro_now(),
					tool.Baro_trunc(note, 300),
				)
			}

			tool.Baro_public_log(db, "legal_stat", config.IP, period, "법적 요청 기간 집계 저장", "합계 "+tool.Baro_comma(total)+"건")
			tool.Baro_audit(db, config, "legal_stat_make", "baro_legal_log/"+period, "", "합계 "+tool.Baro_itoa(total)+"건", tool.Baro_trunc(note, 300))

			return tool.Get_redirect("/baro/legal?period=" + tool.Url_parser(period))
		}
	}

	period = baro_legal_period_norm(period)
	measure := baro_legal_log_measure(db, period)
	saved := baro_legal_count(db, "select count(*) from baro_legal_log where period = ?", period)

	body := tool.Baro_crumb([][]string{
		{"/baro/legal", tool.Baro_t("법적 · 권리 요청 통계")},
		{"", tool.Baro_t("집계 만들기")},
	})

	body += message

	body += tool.Baro_hero(
		tool.Baro_t("기간별 집계 만들기"),
		tool.Baro_t("지금 값을 세어 baro_legal_log 에 저장합니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("대상 기간"), period, ""},
			{tool.Baro_t("이미 저장된 항목"), tool.Baro_comma(saved), tool.Baro_t("다시 만들면 덮어씁니다")},
		}),
	)

	preview_rows := [][]string{}
	total := 0
	for _, row := range baro_legal_log_kind_list() {
		num := measure[row[0]]
		total += num

		preview_rows = append(preview_rows, []string{
			tool.HTML_escape(row[1]),
			tool.Baro_code(row[0]),
			tool.Baro_comma(num) + tool.Baro_t("건"),
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("저장될 값 미리 보기"),
		tool.Baro_t("이름 · 문서명이 들어가지 않는 건수만 저장됩니다."),
		tool.Baro_table([]string{tool.Baro_t("항목"), tool.Baro_t("키"), tool.Baro_t("건수")}, preview_rows, ""),
		tool.Baro_note("합계 "+tool.Baro_comma(total)+tool.Baro_t("건")),
	)

	form := tool.Baro_input_type("period", tool.Baro_t("집계 기간 (YYYY-MM)"), period, "month", tool.Baro_t("기간을 바꾸면 그 기간으로 다시 셉니다."))
	form += tool.Baro_input("note", tool.Baro_t("메모"), "", tool.Baro_t("예) 분기 보고용"), tool.Baro_t("집계 기록에 함께 남습니다. 개인정보를 적지 마세요."))
	form += tool.Baro_checkbox("agree", tool.Baro_t("같은 기간의 기존 집계를 지우고 다시 만드는 데 동의합니다."), "on", false, "")
	form += tool.Baro_submit(tool.Baro_t("집계 저장"), "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("집계 저장"),
		tool.Baro_t("저장한 사실은 공개 로그에 남습니다."),
		tool.Baro_form("/baro/legal/make", form),
		"",
	)

	return baro_legal_page(db, config, tool.Baro_t("기간별 집계 만들기"), "/baro/legal", body)
}
