package route

import (
	"opennamu/route/tool"
)

// ============================================================
//  18장 — 지표 대시보드 (/baro/metrics)
//
//  ★ 이 화면의 규칙 : 가장 크게, 맨 위에 오는 수는 언제나
//    "주간 유효 기여자 수" 이다. 가입자 수와 MAU 는 그 아래에
//    "보조 지표" 로만 둔다. (MAU 하나만 보고 좋아하면 안 된다)
// ============================================================

// View_baro_metric_dash : 지표 대시보드 요약
func View_baro_metric_dash(config tool.Config, span_key string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, stop := baro_metric_guard(db, config); stop {
		return deny
	}

	span := baro_metric_span_get(span_key)
	from := baro_metric_span_start(span)
	to := baro_metric_span_end(span)

	body := ""

	// --- ★ 절대 핵심 지표 ---
	body += baro_metric_key_block(db)

	week_labels, week_values := baro_metric_weekly_series(db, 12)
	body += tool.Baro_card_ex(
		tool.Baro_t("주간 유효 기여자 수 추이"),
		tool.Baro_t("최근 12주"),
		baro_metric_chart_int("line", week_labels, week_values, "명", tool.Baro_t("주간 유효 기여자 수 (최근 12주)"))+
			tool.Baro_note(tool.Baro_t("이 선이 옆으로 눕거나 내려가면, 방문자와 가입자가 아무리 늘어도 위키는 자라지 않고 있는 것입니다.")),
		"",
	)

	// --- 보조 지표 (MAU 함정 경고) ---
	total_user := baro_metric_int(db, "select count(*) from user_set where name = 'date'")
	new_user := baro_metric_int(db, "select count(*) from user_set where name = 'date' and data >= ? and data <= ?", from, to)

	month_from := tool.Baro_time_add(tool.Baro_now(), -30, 0, 0)
	mau := baro_metric_effective_users(db, month_from, tool.Baro_now())
	mau_events := baro_metric_effective_events(db, month_from, tool.Baro_now())

	span_users := baro_metric_effective_users(db, from, to)
	span_events := baro_metric_effective_events(db, from, to)

	side := []baro_metric_item{
		baro_metric_num(tool.Baro_t("누적 가입자 수"), total_user, tool.Baro_t("보조 지표"), tool.Baro_t("지금까지 가입한 사람의 총수입니다. 한 번 올라가면 내려가지 않으므로 상태를 알려주지 못합니다.")),
		baro_metric_num(tool.Baro_t("신규 가입"), new_user, span.Name, tool.Baro_t("이 기간에 가입한 사람 수입니다. 가입은 시작일 뿐 기여가 아닙니다.")),
		baro_metric_num(tool.Baro_t("30일 유효 기여자(MAU)"), mau, tool.Baro_t("최근 30일"), tool.Baro_t("최근 30일 안에 유효 기여를 한 사람 수입니다. 흔히 MAU 라 부르지만, 이 값 하나로 판단하면 안 됩니다.")),
		baro_metric_num(tool.Baro_t("30일 유효 기여 건수"), mau_events, tool.Baro_t("최근 30일"), tool.Baro_t("최근 30일 안의 유효 기여 건수입니다. 사람 수와 함께 봐야 쏠림을 알 수 있습니다.")),
	}

	side_body := baro_metric_cards(side, 4)
	side_body += tool.Baro_alert("warn",
		tool.Baro_t(`<b>MAU 하나만 보고 좋아하면 안 됩니다.</b><br>`)+
			tool.Baro_t(`가입자 수는 줄어들지 않는 수라서 늘 좋아 보이고, 월간 활동자 수는 한 달에 한 번 들른 사람까지 세어 줍니다. `)+
			tool.Baro_t(`위 네 수는 전부 <b>주간 유효 기여자 수</b>를 해석하기 위한 보조 자료로만 쓰세요.`))

	body += tool.Baro_card_ex(tool.Baro_t("보조 지표"), tool.Baro_t("핵심 지표를 해석하기 위한 참고 값"), side_body, "")

	// --- 이 기간 요약 ---
	span_items := []baro_metric_item{
		baro_metric_num(tool.Baro_t("유효 기여자"), span_users, span.Name, tool.Baro_t("이 기간에 되돌려지지 않은 기여를 한 서로 다른 사람 수입니다.")),
		baro_metric_num(tool.Baro_t("유효 기여 건수"), span_events, span.Name, tool.Baro_t("이 기간의 유효 기여 건수입니다.")),
		baro_metric_text_item(tool.Baro_t("1인당 기여 건수"),
			baro_metric_round1(baro_metric_ratio(span_events, span_users))+tool.Baro_t("건"),
			span.Name, "기여자 한 사람이 평균 몇 건을 남겼는지입니다. 사람 수가 그대로인데 이 값만 오르면 소수에게 쏠린 것입니다.", span_users == 0),
		baro_metric_num(tool.Baro_t("하루 평균 유효 기여"), baro_metric_div(span_events, baro_metric_span_days(span)), span.Name, tool.Baro_t("이 기간의 유효 기여를 날짜 수로 나눈 값입니다.")),
	}

	span_body := baro_metric_cards(span_items, 4)
	span_body += baro_metric_chart_int(
		"line", baro_metric_periods(span),
		baro_metric_bucket_uniq(span, baro_metric_col2(
			db,
			"select date, user from baro_contrib_event where date >= ? and date <= ? and user != ''"+baro_metric_bad_status,
			from, to,
		)),
		"명", tool.Baro_t("구간별 유효 기여자 수 (")+span.Name+")")

	span_body += baro_metric_chart_int(
		"bar", baro_metric_periods(span),
		baro_metric_bucket(span, baro_metric_col1(
			db,
			"select date from baro_contrib_event where date >= ? and date <= ?"+baro_metric_bad_status,
			from, to,
		)),
		"건", tool.Baro_t("구간별 유효 기여 건수 (")+span.Name+")")

	body += tool.Baro_card_ex(tool.Baro_t("선택한 기간 요약"), span.From+" ~ "+span.To, span_body, "")

	// --- 네 묶음 + 운영 신뢰 요약 ---
	body += baro_metric_group_summary(config, span)

	// --- 스냅샷 상태 ---
	last_snap := ""
	tool.QueryRow_DB(db, "select period from baro_metric where key_name = 'snap_weekly_effective' order by period desc", []any{&last_snap})

	snap_body := ""
	if last_snap == "" {
		snap_body += tool.Baro_alert("warn", tool.Baro_t("아직 저장된 지표 스냅샷이 없습니다. 추이를 쌓으려면 하루에 한 번 스냅샷을 저장하세요."))
	} else {
		snap_body += tool.Baro_kv_grid([][]string{
			{tool.Baro_t("마지막 저장 구간"), tool.HTML_escape(last_snap)},
			{tool.Baro_t("저장되는 지표 수"), tool.Baro_comma(len(baro_metric_snap_list())) + tool.Baro_t("개")},
		})
	}

	snap_body += tool.Baro_button("/baro/metrics/snapshot", tool.Baro_t("오늘 값 저장하기"), "primary") + " "
	snap_body += tool.Baro_button("/baro/metrics/history?span="+span.Key, tool.Baro_t("저장된 추이 보기"), "")
	snap_body += tool.Baro_note(tool.Baro_t("스냅샷은 baro_metric 표에 오늘 날짜로 저장됩니다. 저장 권한은 system 권한입니다."))

	body += tool.Baro_card_ex(tool.Baro_t("지표 스냅샷"), tool.Baro_t("오늘 값을 저장해 추이를 쌓습니다"), snap_body, "")

	// --- 지표 읽는 법 ---
	body += tool.Baro_card(tool.Baro_t("이 대시보드를 읽는 순서"),
		tool.Baro_list([]string{
			tool.Baro_t(`맨 위 <b>주간 유효 기여자 수</b>를 먼저 봅니다. 이 수가 늘지 않으면 나머지가 다 좋아도 성과가 아닙니다.`),
			`늘지 않았다면 <a href="/baro/metrics/activation?span=` + span.Key + tool.Baro_t(`">활성화</a> 탭에서 깔때기의 어느 칸이 막혔는지 찾습니다.`),
			`활성화가 정상이면 <a href="/baro/metrics/retention?span=` + span.Key + tool.Baro_t(`">유지</a> 탭에서 사람들이 언제 떠나는지 봅니다.`),
			`유지도 정상인데 수가 안 늘면 그때 <a href="/baro/metrics/acquisition?span=` + span.Key + tool.Baro_t(`">유입</a> 탭을 봅니다. 유입은 마지막에 봅니다.`),
			`<a href="/baro/metrics/quality?span=` + span.Key + tool.Baro_t(`">품질</a>과 <a href="/baro/metrics/trust?span=`) + span.Key + tool.Baro_t(`">운영 신뢰</a>는 브레이크입니다. 여기가 나빠지면 위 숫자가 좋아도 멈춰야 합니다.`),
		}))

	return baro_metric_frame(db, config, tool.Baro_t("요약"), "/baro/metrics", span, body)
}

// baro_metric_ratio : 나눗셈 (분모가 0 이면 0)
func baro_metric_ratio(a int, b int) float64 {
	if b <= 0 {
		return 0
	}

	return float64(a) / float64(b)
}

// baro_metric_div : 정수 나눗셈 (반올림)
func baro_metric_div(a int, b int) int {
	if b <= 0 {
		return 0
	}

	return int(float64(a)/float64(b) + 0.5)
}

// baro_metric_group_summary : 다섯 묶음 요약 카드
func baro_metric_group_summary(config tool.Config, span baro_metric_span) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	from := baro_metric_span_start(span)
	to := baro_metric_span_end(span)

	cards := []string{}

	// 유입
	landing := baro_metric_series_get(db, "landing_view", span)
	new_user := baro_metric_int(db, "select count(*) from user_set where name = 'date' and data >= ? and data <= ?", from, to)
	invite_total := baro_metric_int(db, "select count(*) from baro_invite")
	invite_done := baro_metric_int(db, "select count(*) from baro_invite where status = 'done'")

	landing_value := tool.Baro_t("아직 집계 없음")
	if landing.Ever {
		landing_value = tool.Baro_comma(landing.Total) + tool.Baro_t("회")
	}

	cards = append(cards, tool.Baro_card_ex(tool.Baro_t("유입"), tool.Baro_t("사람이 얼마나 들어왔는가"),
		tool.Baro_kv_grid([][]string{
			{tool.Baro_t("랜딩 방문"), tool.HTML_escape(landing_value)},
			{tool.Baro_t("신규 가입"), tool.Baro_comma(new_user) + tool.Baro_t("명")},
			{tool.Baro_t("초대 전환율"), tool.Baro_pct_text(invite_done, invite_total)},
		}),
		tool.Baro_button("/baro/metrics/acquisition?span="+span.Key, tool.Baro_t("유입 지표 자세히"), "")))

	// 활성화
	cohort := baro_metric_signup_users(db, span)
	reg := map[string]bool{}
	for _, row := range cohort {
		reg[row[0]] = true
	}

	first_event := baro_metric_first_map(db, "select user, date from baro_contrib_event where user != ''"+baro_metric_bad_status)
	first_draft := baro_metric_first_map(db, "select user, date from baro_workspace where user != ''")

	act_event := 0
	act_draft := 0
	for user := range reg {
		if _, ok := first_event[user]; ok {
			act_event++
		}
		if _, ok := first_draft[user]; ok {
			act_draft++
		}
	}

	cards = append(cards, tool.Baro_card_ex(tool.Baro_t("활성화"), tool.Baro_t("가입한 사람이 실제로 남겼는가"),
		tool.Baro_kv_grid([][]string{
			{tool.Baro_t("가입 → 첫 연습 편집"), tool.Baro_pct_text(act_draft, len(cohort))},
			{tool.Baro_t("가입 → 첫 실제 기여"), tool.Baro_pct_text(act_event, len(cohort))},
			{tool.Baro_t("대상 신규 가입"), tool.Baro_comma(len(cohort)) + tool.Baro_t("명")},
		}),
		tool.Baro_button("/baro/metrics/activation?span="+span.Key, tool.Baro_t("활성화 지표 자세히"), "")))

	// 유지
	events := baro_metric_user_events(db)
	back7, total7 := baro_metric_retention_calc(events, span, 7)
	back30, total30 := baro_metric_retention_calc(events, span, 30)

	months := baro_metric_month_list(1)
	core_series, _ := baro_metric_core_by_month(db, months)
	core_now := 0
	if len(core_series) > 0 {
		core_now = core_series[0]
	}

	cards = append(cards, tool.Baro_card_ex("유지", tool.Baro_t("한 번 온 사람이 다시 오는가"),
		tool.Baro_kv_grid([][]string{
			{tool.Baro_t("7일 후 재기여"), tool.Baro_pct_text(back7, total7)},
			{tool.Baro_t("30일 후 재기여"), tool.Baro_pct_text(back30, total30)},
			{tool.Baro_t("이번 달 핵심 기여자"), tool.Baro_comma(core_now) + tool.Baro_t("명")},
		}),
		tool.Baro_button("/baro/metrics/retention?span="+span.Key, tool.Baro_t("유지 지표 자세히"), "")))

	// 품질
	review_total := baro_metric_int(db, "select count(*) from baro_review where date >= ? and date <= ?", from, to)
	review_revert := baro_metric_int(db, "select count(*) from baro_review where status = 'reverted' and date >= ? and date <= ?", from, to)
	doc_total := baro_metric_doc_total(db)
	doc_cited := baro_metric_int(db, "select count(distinct title) from baro_citation where title != '' and title in (select title from data)")
	broken := baro_metric_int(db, "select count(*) from baro_citation where status = 'broken'")

	cards = append(cards, tool.Baro_card_ex(tool.Baro_t("품질"), tool.Baro_t("쌓인 글이 남을 만한가"),
		tool.Baro_kv_grid([][]string{
			{tool.Baro_t("편집 되돌림률"), tool.Baro_pct_text(review_revert, review_total)},
			{tool.Baro_t("출처 포함률"), tool.Baro_pct_text(doc_cited, doc_total)},
			{tool.Baro_t("깨진 링크"), tool.Baro_comma(broken) + tool.Baro_t("건")},
		}),
		tool.Baro_button("/baro/metrics/quality?span="+span.Key, tool.Baro_t("품질 지표 자세히"), "")))

	// 운영 신뢰
	backlog := baro_metric_backlog_rows(db)
	mass_total := baro_metric_int(db, "select count(*) from baro_mass_op where created >= ? and created <= ?", from, to)

	appeal_closed := baro_metric_int(db, "select count(*) from baro_appeal where close_date != ''")
	appeal_ok := 0
	for _, result := range baro_metric_col1(db, "select result from baro_appeal where close_date != ''") {
		if baro_metric_appeal_ok(result) {
			appeal_ok++
		}
	}

	cards = append(cards, tool.Baro_card_ex(tool.Baro_t("운영 신뢰"), tool.Baro_t("빠른가, 그리고 되돌릴 수 있는가"),
		tool.Baro_kv_grid([][]string{
			{tool.Baro_t("장기 미처리 사건"), tool.Baro_comma(len(backlog)) + tool.Baro_t("건")},
			{tool.Baro_t("대량 조치"), tool.Baro_comma(mass_total) + tool.Baro_t("회")},
			{tool.Baro_t("이의제기 인용률"), tool.Baro_pct_text(appeal_ok, appeal_closed)},
		}),
		tool.Baro_button("/baro/metrics/trust?span="+span.Key, tool.Baro_t("운영 신뢰 지표 자세히"), "")))

	_ = config

	return tool.Baro_section(tool.Baro_t("묶음별 요약"), tool.Baro_grid(cards, 2))
}
