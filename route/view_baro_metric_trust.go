package route

import (
	"database/sql"
	"sort"

	"opennamu/route/tool"
)

// ============================================================
//  18장 — 운영 신뢰 지표 (/baro/metrics/trust)
//
//  운영이 빠르고 되돌릴 수 있는가를 본다.
//  신고 처리 시간 · 이의제기 처리 시간 · 이의제기 인용률 ·
//  대량 조치 횟수 · 복구 횟수 · 관리자별 장기 미처리 사건
//
//  5번 원칙에 따라 이 화면에는 신고 원문 · IP · 이메일을 쓰지 않는다.
// ============================================================

// baro_metric_appeal_ok : 이의제기 결과가 인용(받아들여짐)인지
func baro_metric_appeal_ok(result string) bool {
	switch result {
	case "accept", "accepted", "granted", "ok", "partial", "reduce", "revoke":
		return true
	}

	return false
}

// baro_metric_case_open : 사건이 아직 열려 있는 상태인지
func baro_metric_case_open(status string) bool {
	switch status {
	case "closed", "done", "rejected", "expired":
		return false
	}

	return true
}

// baro_metric_backlog_days : 장기 미처리로 보는 기준 일수
func baro_metric_backlog_days(db *sql.DB) int {
	days := tool.Baro_get_int(db, "metric_backlog_days", 7)
	if days < 1 {
		days = 7
	}

	return days
}

// baro_metric_backlog_rows : 장기 미처리 사건 목록 (오래된 순)
func baro_metric_backlog_rows(db *sql.DB) [][]string {
	limit := tool.Baro_time_add(tool.Baro_now(), -baro_metric_backlog_days(db), 0, 0)

	rows := baro_metric_col5(
		db,
		"select id, number, actor, status, date from baro_case where date != '' and date <= ? order by date",
		limit,
	)

	out := [][]string{}
	for _, row := range rows {
		if !baro_metric_case_open(row[3]) {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_metric_actor_backlog : 관리자별 장기 미처리 사건 수
func baro_metric_actor_backlog(rows [][]string) [][]string {
	count := map[string]int{}
	for _, row := range rows {
		actor := row[2]
		if actor == "" {
			actor = "(담당자 없음)"
		}

		count[actor]++
	}

	keys := []string{}
	for key := range count {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := [][]string{}
	for _, key := range keys {
		out = append(out, []string{key, tool.Baro_itoa(count[key])})
	}

	return out
}

// View_baro_metric_trust : 운영 신뢰 지표
func View_baro_metric_trust(config tool.Config, span_key string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, stop := baro_metric_guard(db, config); stop {
		return deny
	}

	span := baro_metric_span_get(span_key)
	from := baro_metric_span_start(span)
	to := baro_metric_span_end(span)
	now := tool.Baro_now()

	body := ""
	body += tool.Baro_alert("info",
		tool.Baro_t(`운영 신뢰 지표는 "운영이 빠른가" 와 "틀렸을 때 되돌릴 수 있는가" 를 함께 봅니다. `)+
			tool.Baro_t(`빠르기만 하고 복구가 없으면 신뢰가 아니라 공포입니다.`))

	// --- 1. 신고 처리 시간 ---
	report_total := baro_metric_int(db, "select count(*) from baro_report where date >= ? and date <= ?", from, to)
	report_open_rows := baro_metric_col1(db, "select date from baro_report where status in ('open', 'pending', 'new', '')")

	handled := baro_metric_col2(
		db,
		"select baro_report.date, baro_case.close_date from baro_report join baro_case on baro_report.case_id = baro_case.id where baro_report.case_id != '' and baro_case.close_date != '' and baro_report.date >= ? and baro_report.date <= ?",
		from, to,
	)

	report_hours := []float64{}
	for _, row := range handled {
		gap := baro_metric_gap_hours(row[0], row[1])
		if gap < 0 {
			continue
		}

		report_hours = append(report_hours, gap)
	}

	report_oldest := 0.0
	for _, date := range report_open_rows {
		gap := baro_metric_gap_hours(date, now)
		if gap > report_oldest {
			report_oldest = gap
		}
	}

	report_items := []baro_metric_item{
		baro_metric_text_item(tool.Baro_t("신고 처리 시간(중앙값)"), baro_metric_hours_text(baro_metric_median(report_hours)), tool.Baro_comma(len(report_hours))+tool.Baro_t("건 기준"),
			"신고가 들어온 뒤 그 신고가 연결된 사건이 닫힐 때까지 걸린 시간의 중앙값입니다.", len(report_hours) == 0),
		baro_metric_text_item(tool.Baro_t("신고 처리 시간(평균)"), baro_metric_hours_text(baro_metric_mean(report_hours)), tool.Baro_comma(len(report_hours))+tool.Baro_t("건 기준"),
			"같은 값의 평균입니다. 오래 묵은 몇 건이 평균을 크게 올릴 수 있어 중앙값과 함께 봅니다.", len(report_hours) == 0),
		baro_metric_num(tool.Baro_t("이 기간의 신고"), report_total, span.Name, tool.Baro_t("이 기간에 접수된 신고 건수입니다.")),
		baro_metric_text_item(tool.Baro_t("가장 오래 기다린 신고"), baro_metric_hours_text(report_oldest), tool.Baro_comma(len(report_open_rows))+tool.Baro_t("건 대기 중"),
			"아직 처리되지 않은 신고 중 가장 오래된 것이 기다린 시간입니다.", len(report_open_rows) == 0),
	}

	report_body := baro_metric_cards(report_items, 4)
	report_body += baro_metric_chart_int("bar", baro_metric_periods(span),
		baro_metric_bucket(span, baro_metric_col1(db, "select date from baro_report where date >= ? and date <= ?", from, to)),
		"건", tool.Baro_t("신고 접수 추이 (")+span.Name+")")

	report_status := [][]string{}
	for _, row := range baro_metric_col2(db, "select status, count(*) from baro_report group by status") {
		name := row[0]
		if name == "" {
			name = tool.Baro_t("(상태 없음)")
		}

		report_status = append(report_status, []string{name, row[1]})
	}
	report_body += baro_metric_rank_table(tool.Baro_t("신고 상태"), tool.Baro_t("건수"), report_status, tool.Baro_t("아직 접수된 신고가 없습니다."))
	report_body += tool.Baro_note(tool.Baro_t("이 화면에는 신고 원문과 신고자 정보를 표시하지 않습니다. (0장 5번 원칙 — 근거는 공개하되 개인정보는 보호)"))

	body += tool.Baro_card_ex(tool.Baro_t("신고 처리 시간"), tool.Baro_t("baro_report 접수 ~ 연결된 baro_case 종결"), report_body, "")

	// --- 2. 이의제기 처리 시간 · 인용률 ---
	appeal_rows := baro_metric_col4(
		db,
		"select date, close_date, status, result from baro_appeal where date >= ? and date <= ?",
		from, to,
	)

	appeal_hours := []float64{}
	appeal_closed := 0
	appeal_ok := 0

	for _, row := range appeal_rows {
		if row[1] == "" {
			continue
		}

		gap := baro_metric_gap_hours(row[0], row[1])
		if gap >= 0 {
			appeal_hours = append(appeal_hours, gap)
		}

		appeal_closed++
		if baro_metric_appeal_ok(row[3]) {
			appeal_ok++
		}
	}

	// 대기 중인 이의제기는 기간과 무관하게 전부 본다 (묵은 건이 기간 밖에 숨지 않도록)
	appeal_open_rows := baro_metric_col1(db, "select date from baro_appeal where close_date = '' or close_date is null")
	appeal_open := len(appeal_open_rows)
	appeal_oldest := 0.0

	for _, date := range appeal_open_rows {
		gap := baro_metric_gap_hours(date, now)
		if gap > appeal_oldest {
			appeal_oldest = gap
		}
	}

	appeal_all := baro_metric_int(db, "select count(*) from baro_appeal")
	appeal_all_closed := baro_metric_int(db, "select count(*) from baro_appeal where close_date != ''")
	appeal_all_ok := 0
	for _, row := range baro_metric_col1(db, "select result from baro_appeal where close_date != ''") {
		if baro_metric_appeal_ok(row) {
			appeal_all_ok++
		}
	}

	appeal_items := []baro_metric_item{
		baro_metric_text_item(tool.Baro_t("이의제기 처리 시간(중앙값)"), baro_metric_hours_text(baro_metric_median(appeal_hours)), tool.Baro_comma(len(appeal_hours))+tool.Baro_t("건 기준"),
			"이의제기가 들어온 뒤 결론이 날 때까지 걸린 시간의 중앙값입니다. (baro_appeal 의 date ~ close_date)", len(appeal_hours) == 0),
		baro_metric_rate(tool.Baro_t("이의제기 인용률"), appeal_ok, appeal_closed, "건", tool.Baro_t("결론이 난 이의제기 중 받아들여진 비율입니다. 0% 가 좋은 것이 아닙니다 — 한 건도 안 받아들여지면 이의제기 절차가 형식뿐이라는 신호입니다.")),
		baro_metric_rate(tool.Baro_t("누적 이의제기 인용률"), appeal_all_ok, appeal_all_closed, "건", tool.Baro_t("기간과 무관하게 지금까지 결론이 난 모든 이의제기의 인용률입니다.")),
		baro_metric_text_item(tool.Baro_t("가장 오래 기다린 이의제기"), baro_metric_hours_text(appeal_oldest), tool.Baro_comma(appeal_open)+tool.Baro_t("건 대기 중"),
			"아직 결론이 나지 않은 이의제기 중 가장 오래된 것이 기다린 시간입니다.", appeal_open == 0),
	}

	appeal_body := baro_metric_cards(appeal_items, 4)

	appeal_result := [][]string{}
	for _, row := range baro_metric_col2(db, "select result, count(*) from baro_appeal where close_date != '' group by result") {
		name := row[0]
		if name == "" {
			name = tool.Baro_t("(결과 표시 없음)")
		}

		appeal_result = append(appeal_result, []string{name, row[1]})
	}
	appeal_body += baro_metric_rank_table(tool.Baro_t("이의제기 결과"), tool.Baro_t("건수"), appeal_result, tool.Baro_t("아직 결론이 난 이의제기가 없습니다."))

	if appeal_all == 0 {
		appeal_body += tool.Baro_empty(tool.Baro_t("아직 데이터가 없습니다. baro_appeal 표에 기록이 하나도 없습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("이의제기"), tool.Baro_t("처리 시간과 인용률"), appeal_body, "")

	// --- 3. 대량 조치 · 복구 ---
	mass_total := baro_metric_int(db, "select count(*) from baro_mass_op where created >= ? and created <= ?", from, to)
	mass_done := baro_metric_int(db, "select count(*) from baro_mass_op where status in ('done', 'executed') and created >= ? and created <= ?", from, to)
	mass_undo := baro_metric_int(db, "select count(*) from baro_mass_op where undo_status in ('done', 'undone') and created >= ? and created <= ?", from, to)
	mass_urgent := baro_metric_int(db, "select count(*) from baro_mass_op where urgent = 'on' and created >= ? and created <= ?", from, to)
	q_release := baro_metric_int(db, "select count(*) from baro_quarantine where status = 'released' and release_date >= ? and release_date <= ?", from, to)

	mass_target := 0
	for _, row := range baro_metric_col1(db, "select num from baro_mass_op where created >= ? and created <= ?", from, to) {
		mass_target += tool.Str_to_int(row)
	}

	mass_items := []baro_metric_item{
		baro_metric_num(tool.Baro_t("대량 조치 횟수"), mass_total, span.Name, tool.Baro_t("이 기간에 만들어진 대량 작업 건수입니다. (baro_mass_op)")),
		baro_metric_num(tool.Baro_t("실행된 대량 조치"), mass_done, span.Name, tool.Baro_t("그 중 실제로 실행까지 간 건수입니다.")),
		baro_metric_num(tool.Baro_t("대량 조치 대상 수 합계"), mass_target, span.Name, tool.Baro_t("대량 작업이 건드린 대상 수의 합입니다. 횟수보다 이 값이 실제 충격의 크기입니다.")),
		baro_metric_num(tool.Baro_t("긴급 처리"), mass_urgent, span.Name, tool.Baro_t("유예 없이 긴급으로 처리된 대량 작업 수입니다. 많아지면 안전장치가 형식화된 것입니다.")),
	}

	undo_items := []baro_metric_item{
		baro_metric_num(tool.Baro_t("복구 횟수(대량 조치 되돌리기)"), mass_undo, span.Name, tool.Baro_t("실행된 대량 작업을 다시 되돌린 횟수입니다.")),
		baro_metric_num(tool.Baro_t("복구 횟수(격리 해제)"), q_release, span.Name, tool.Baro_t("임시 격리에서 풀려 되살아난 문서 수입니다.")),
		baro_metric_num(tool.Baro_t("복구 합계"), mass_undo+q_release, span.Name, tool.Baro_t("되돌린 조치의 총 횟수입니다. 0 이 좋은 것이 아니라, 되돌릴 수 있다는 사실이 신뢰의 근거입니다.")),
		baro_metric_rate(tool.Baro_t("대량 조치 되돌림 비율"), mass_undo, mass_done, "건", tool.Baro_t("실행된 대량 작업 중 되돌려진 비율입니다. 높으면 실행 전 검토가 부족하다는 뜻입니다.")),
	}

	mass_body := baro_metric_cards(mass_items, 4)
	mass_body += baro_metric_cards(undo_items, 4)

	mass_kind := [][]string{}
	for _, row := range baro_metric_col2(db, "select kind, count(*) from baro_mass_op where created >= ? and created <= ? group by kind", from, to) {
		mass_kind = append(mass_kind, []string{tool.Baro_mass_kind_name(row[0]), row[1]})
	}
	mass_body += baro_metric_rank_table(tool.Baro_t("대량 작업 종류"), tool.Baro_t("횟수"), mass_kind, tool.Baro_t("이 기간에 대량 작업이 없었습니다."))

	mass_body += baro_metric_chart_int("bar", baro_metric_periods(span),
		baro_metric_bucket(span, baro_metric_col1(db, "select created from baro_mass_op where created >= ? and created <= ?", from, to)),
		"건", tool.Baro_t("대량 조치 추이 (")+span.Name+")")

	mass_body += tool.Baro_note(tool.Baro_t("대량 작업 문턱은 현재 ") + tool.Baro_comma(tool.Baro_mass_threshold(db)) + tool.Baro_t("건, 유예 시간은 ") + tool.Baro_comma(tool.Baro_mass_grace_hours(db)) + tool.Baro_t("시간으로 설정되어 있습니다. (0장 3번 원칙)"))

	body += tool.Baro_card_ex(tool.Baro_t("대량 조치와 복구"), tool.Baro_t("얼마나 크게 건드렸고, 얼마나 되돌렸는가"), mass_body, "")

	// --- 4. 관리자별 장기 미처리 사건 ---
	backlog := baro_metric_backlog_rows(db)
	actor_rows := baro_metric_actor_backlog(backlog)

	case_open := baro_metric_int(db, "select count(*) from baro_case where status not in ('closed', 'done', 'rejected', 'expired')")

	backlog_items := []baro_metric_item{
		baro_metric_num(tool.Baro_t("장기 미처리 사건"), len(backlog), tool.Baro_itoa(baro_metric_backlog_days(db))+tool.Baro_t("일 이상 열려 있음"), tool.Baro_t("정해진 기준보다 오래 열려 있는 사건 수입니다. 사람이 아니라 절차의 병목을 찾는 지표입니다.")),
		baro_metric_num(tool.Baro_t("열려 있는 사건 전체"), case_open, tool.Baro_t("전체"), tool.Baro_t("지금 닫히지 않은 사건 수입니다.")),
		baro_metric_rate(tool.Baro_t("장기 미처리 비중"), len(backlog), case_open, "건", tool.Baro_t("열려 있는 사건 중 기준을 넘긴 것의 비율입니다.")),
		baro_metric_num(tool.Baro_t("담당자 수"), len(actor_rows), tool.Baro_t("장기 미처리 보유"), tool.Baro_t("장기 미처리 사건을 하나라도 들고 있는 담당자 수입니다.")),
	}

	backlog_body := baro_metric_cards(backlog_items, 4)

	actor_table := [][]string{}
	for _, row := range actor_rows {
		name := row[0]
		cell := tool.HTML_escape(name)
		if name != "(담당자 없음)" {
			cell = tool.Baro_user_link(db, name, config.IP)
		}

		actor_table = append(actor_table, []string{
			cell,
			tool.Baro_comma(tool.Str_to_int(row[1])),
			tool.Baro_progress(tool.Str_to_int(row[1]), len(backlog), ""),
		})
	}

	backlog_body += tool.Baro_table([]string{"담당자", tool.Baro_t("장기 미처리 사건"), tool.Baro_t("분포")}, actor_table, tool.Baro_t("장기 미처리 사건이 없습니다. 좋은 상태입니다."))
	backlog_body += tool.Baro_button("/baro/metrics/backlog?span="+span.Key, tool.Baro_t("장기 미처리 사건 전체 목록 보기"), "primary")
	backlog_body += tool.Baro_note(tool.Baro_t("이 표는 담당자를 평가하기 위한 것이 아니라, 사건이 어디에서 멈춰 있는지 찾기 위한 것입니다. 기준 일수는 baro_set 의 metric_backlog_days 로 바꿀 수 있습니다."))

	body += tool.Baro_card_ex(tool.Baro_t("관리자별 장기 미처리 사건"), tool.Baro_itoa(baro_metric_backlog_days(db))+tool.Baro_t("일 넘게 닫히지 않은 사건"), backlog_body, "")

	return baro_metric_frame(db, config, tool.Baro_t("운영 신뢰 지표"), "/baro/metrics/trust", span, body)
}
