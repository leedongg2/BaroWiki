package route

import (
	"database/sql"

	"opennamu/route/tool"
)

// ============================================================
//  18장 — 활성화 지표 (/baro/metrics/activation)
//
//  가입만 하고 사라지는 사람과, 실제로 한 글자라도 남긴 사람을 갈라 본다.
//  가입 → 첫 연습 편집 → 첫 실제 기여 → 첫 7일 유효 기여 로 이어지는 깔때기.
// ============================================================

// baro_metric_first_map : "사람 → 가장 이른 시각" 표 만들기
func baro_metric_first_map(db *sql.DB, query string, values ...any) map[string]string {
	out := map[string]string{}

	for _, row := range baro_metric_col2(db, query, values...) {
		if row[0] == "" || row[1] == "" {
			continue
		}

		old, ok := out[row[0]]
		if !ok || row[1] < old {
			out[row[0]] = row[1]
		}
	}

	return out
}

// baro_metric_gap_bucket : 첫 기여까지 걸린 날짜를 구간으로 나눈다
func baro_metric_gap_bucket(days float64) int {
	switch {
	case days < 1:
		return 0
	case days < 2:
		return 1
	case days < 3:
		return 2
	case days < 8:
		return 3
	case days < 15:
		return 4
	case days < 31:
		return 5
	}

	return 6
}

// baro_metric_gap_bucket_names : 첫 기여까지 걸린 시간 구간 이름
func baro_metric_gap_bucket_names() []string {
	return []string{tool.Baro_t("당일"), tool.Baro_t("1일"), tool.Baro_t("2일"), tool.Baro_t("3~7일"), tool.Baro_t("8~14일"), tool.Baro_t("15~30일"), tool.Baro_t("31일 이상")}
}

// baro_metric_review_ok : 검토 결과가 승인으로 볼 만한 상태인지
func baro_metric_review_ok(status string) bool {
	switch status {
	case "done", "confirmed", "approved", "accept", "accepted", "ok", "pass":
		return true
	}

	return false
}

// baro_metric_review_closed : 검토가 끝난 상태인지
func baro_metric_review_closed(status string) bool {
	if baro_metric_review_ok(status) {
		return true
	}

	switch status {
	case "rejected", "reverted", "denied", "closed", "expired":
		return true
	}

	return false
}

// View_baro_metric_activation : 활성화 지표
func View_baro_metric_activation(config tool.Config, span_key string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, stop := baro_metric_guard(db, config); stop {
		return deny
	}

	span := baro_metric_span_get(span_key)
	body := ""

	body += tool.Baro_alert("info",
		tool.Baro_t(`활성화 지표는 "가입한 사람이 실제로 한 글자라도 남겼는가" 를 봅니다. `)+
			tool.Baro_t(`가입자 수가 아무리 늘어도 이 깔때기의 아래쪽이 얇으면 위키는 자라지 않습니다.`))

	// --- 대상 집단 : 이 기간에 가입한 사람 ---
	cohort := baro_metric_signup_users(db, span)
	reg_date := map[string]string{}
	for _, row := range cohort {
		reg_date[row[0]] = row[1]
	}

	first_draft := baro_metric_first_map(db, "select user, date from baro_workspace where user != ''")
	first_event := baro_metric_first_map(db, "select user, date from baro_contrib_event where user != ''"+baro_metric_bad_status)

	draft_done := 0
	event_done := 0
	gap_days := []float64{}
	gap_hist := make([]int, len(baro_metric_gap_bucket_names()))

	for user := range reg_date {
		if _, ok := first_draft[user]; ok {
			draft_done++
		}

		first, ok := first_event[user]
		if !ok {
			continue
		}

		event_done++

		hours := baro_metric_gap_hours(reg_date[user], first)
		if hours < 0 {
			continue
		}

		days := hours / 24
		gap_days = append(gap_days, days)
		gap_hist[baro_metric_gap_bucket(days)]++
	}

	// --- 첫 7일 유효 기여 수 ---
	week_events := map[string]int{}
	if len(cohort) > 0 {
		for _, row := range baro_metric_col2(
			db,
			"select user, date from baro_contrib_event where user != '' and date >= ?"+baro_metric_bad_status,
			baro_metric_span_start(span),
		) {
			reg, ok := reg_date[row[0]]
			if !ok {
				continue
			}

			hours := baro_metric_gap_hours(reg, row[1])
			if hours < 0 || hours > 24*7 {
				continue
			}

			week_events[row[0]]++
		}
	}

	week_total := 0
	week_list := []float64{}
	for user := range reg_date {
		week_total += week_events[user]
		week_list = append(week_list, float64(week_events[user]))
	}

	// --- 첫 기여 승인률 ---
	first_review := map[string][]string{}
	for _, row := range baro_metric_col3(db, "select user, status, date from baro_review where user != '' order by date") {
		if _, ok := reg_date[row[0]]; !ok {
			continue
		}

		if _, ok := first_review[row[0]]; ok {
			continue
		}

		first_review[row[0]] = []string{row[1], row[2]}
	}

	review_closed := 0
	review_ok := 0
	for _, row := range first_review {
		if !baro_metric_review_closed(row[0]) {
			continue
		}

		review_closed++
		if baro_metric_review_ok(row[0]) {
			review_ok++
		}
	}

	// --- 깔때기 ---
	funnel := ""
	funnel += tool.Baro_progress(len(cohort), len(cohort), tool.Baro_t("① 가입 (")+span.Name+")")
	funnel += tool.Baro_progress(draft_done, len(cohort), tool.Baro_t("② 첫 연습 편집 (baro_workspace)"))
	funnel += tool.Baro_progress(event_done, len(cohort), tool.Baro_t("③ 첫 실제 기여 (baro_contrib_event)"))

	week_active := 0
	for user := range reg_date {
		if week_events[user] > 0 {
			week_active++
		}
	}
	funnel += tool.Baro_progress(week_active, len(cohort), tool.Baro_t("④ 가입 7일 안에 유효 기여"))

	if len(cohort) == 0 {
		funnel += tool.Baro_empty(tool.Baro_t("이 기간에 가입한 사람이 없습니다. 아직 데이터가 없습니다."))
	}

	funnel += tool.Baro_note(tool.Baro_t("각 칸은 이 기간에 가입한 사람만 대상으로 셉니다. 위 칸에 든 사람이 아래 칸으로 얼마나 내려가는지가 활성화의 전부입니다."))

	body += tool.Baro_card_ex(tool.Baro_t("활성화 깔때기"), tool.Baro_t("가입 → 연습 → 실제 기여 → 첫 주 유효 기여"), funnel, "")

	// --- 지표 카드 ---
	items := []baro_metric_item{
		baro_metric_num(tool.Baro_t("대상 신규 가입"), len(cohort), span.Name, tool.Baro_t("이 기간에 가입한 사람 수입니다. 아래 비율의 분모가 됩니다.")),
		baro_metric_rate(tool.Baro_t("가입 → 첫 연습 편집"), draft_done, len(cohort), "명", tool.Baro_t("가입한 사람 중 연습장(baro_workspace)에 초안을 한 번이라도 만든 비율입니다.")),
		baro_metric_rate(tool.Baro_t("가입 → 첫 실제 기여"), event_done, len(cohort), "명", tool.Baro_t("가입한 사람 중 실제 기여(baro_contrib_event)를 한 번이라도 남긴 비율입니다. 활성화의 핵심 칸입니다.")),
		baro_metric_rate(tool.Baro_t("첫 기여 승인률"), review_ok, review_closed, "명", tool.Baro_t("첫 기여가 검토를 거쳐 승인된 비율입니다. 낮으면 새 사람이 첫 시도에서 좌절한다는 뜻입니다.")),
	}

	median_gap := baro_metric_median(gap_days)
	if len(gap_days) > 0 {
		items = append(items, baro_metric_text_item(
			tool.Baro_t("첫 기여까지 걸린 시간(중앙값)"), baro_metric_days_text(median_gap),
			tool.Baro_comma(len(gap_days))+tool.Baro_t("명 기준"),
			tool.Baro_t("가입일과 첫 기여 날짜 차이의 중앙값입니다. 평균 대신 중앙값을 쓰는 이유는 아주 늦게 시작한 몇 사람이 평균을 끌어올리기 때문입니다."), false))
	} else {
		items = append(items, baro_metric_text_item(
			tool.Baro_t("첫 기여까지 걸린 시간(중앙값)"), tool.Baro_t("0일"), tool.Baro_t("대상 0명"),
			tool.Baro_t("가입일과 첫 기여 날짜 차이의 중앙값입니다. 아직 첫 기여를 한 신규 가입자가 없습니다."), true))
	}

	items = append(items, baro_metric_num(tool.Baro_t("첫 7일 유효 기여 수"), week_total, "합계", tool.Baro_t("이 기간 신규 가입자가 가입 후 7일 안에 남긴 유효 기여의 총 건수입니다.")))
	items = append(items, baro_metric_text_item(
		tool.Baro_t("1인당 첫 7일 기여(평균)"), baro_metric_round1(baro_metric_mean(week_list))+tool.Baro_t("건"),
		tool.Baro_comma(len(cohort))+tool.Baro_t("명 기준"),
		"신규 가입자 한 사람이 첫 주에 남긴 평균 유효 기여 건수입니다.", len(cohort) == 0))
	items = append(items, baro_metric_text_item(
		tool.Baro_t("1인당 첫 7일 기여(중앙값)"), baro_metric_round1(baro_metric_median(week_list))+tool.Baro_t("건"),
		tool.Baro_comma(len(cohort))+tool.Baro_t("명 기준"),
		"절반의 사람이 이 값보다 적게 기여했다는 뜻입니다. 평균이 높아도 중앙값이 0이면 소수가 다 한 것입니다.", len(cohort) == 0))

	body += tool.Baro_card_ex(tool.Baro_t("활성화 지표"), tool.Baro_t("모든 값은 baro_workspace · baro_contrib_event · baro_review 를 직접 센 것입니다"), baro_metric_cards(items, 4), "")

	// --- 첫 기여까지 걸린 시간 분포 ---
	dist_body := baro_metric_chart_int("bar", baro_metric_gap_bucket_names(), gap_hist, "명", tool.Baro_t("가입 후 첫 기여까지 걸린 시간 분포"))
	dist_body += tool.Baro_note(tool.Baro_t("당일 칸이 두꺼울수록 온보딩이 잘 이어진 것입니다. 오른쪽으로 길게 늘어지면 가입 직후에 할 일을 못 찾았다는 뜻입니다."))

	dist_rows := [][]string{}
	for i, name := range baro_metric_gap_bucket_names() {
		dist_rows = append(dist_rows, []string{
			tool.HTML_escape(name),
			tool.Baro_comma(gap_hist[i]),
			tool.Baro_pct_text(gap_hist[i], len(gap_days)),
		})
	}
	dist_body += tool.Baro_table([]string{tool.Baro_t("걸린 시간"), tool.Baro_t("사람 수"), tool.Baro_t("비중")}, dist_rows, tool.Baro_t("아직 첫 기여를 한 신규 가입자가 없습니다."))

	body += tool.Baro_card_ex(tool.Baro_t("첫 기여까지 걸린 시간"), tool.Baro_t("가입일과 첫 기여 날짜의 차이"), dist_body, "")

	// --- 첫 기여 검토 결과 ---
	review_rows := [][]string{}
	review_stat := map[string]int{}
	for _, row := range first_review {
		review_stat[row[0]]++
	}

	for status, count := range review_stat {
		name := status
		if name == "" {
			name = tool.Baro_t("(상태 없음)")
		}

		kind := "muted"
		switch {
		case baro_metric_review_ok(status):
			kind = "ok"
		case status == "rejected" || status == "reverted" || status == "denied":
			kind = "danger"
		case !baro_metric_review_closed(status):
			kind = "warn"
		}

		review_rows = append(review_rows, []string{
			tool.Baro_badge(name, kind),
			tool.Baro_comma(count),
			tool.Baro_pct_text(count, len(first_review)),
		})
	}

	review_body := tool.Baro_table([]string{tool.Baro_t("첫 검토 결과"), tool.Baro_t("사람 수"), tool.Baro_t("비중")}, review_rows, tool.Baro_t("이 기간 신규 가입자의 검토 기록이 아직 없습니다."))
	review_body += tool.Baro_note(tool.Baro_t("한 사람의 가장 이른 baro_review 기록 하나만 셉니다. 아직 결론이 나지 않은 상태는 승인률의 분모에서 뺍니다."))

	body += tool.Baro_card_ex(tool.Baro_t("첫 기여 승인률"), tool.Baro_t("baro_review 결과 기준"), review_body, "")

	// --- 활성화 추이 ---
	trend_body := baro_metric_chart_int(
		"line", baro_metric_periods(span),
		baro_metric_bucket_uniq(span, baro_metric_col2(
			db,
			"select date, user from baro_contrib_event where date >= ? and date <= ? and user != ''"+baro_metric_bad_status,
			baro_metric_span_start(span), baro_metric_span_end(span),
		)),
		"명", tool.Baro_t("유효 기여를 한 사람 수 추이 (")+span.Name+")")

	trend_body += tool.Baro_note(tool.Baro_t("이 그래프가 오른쪽으로 올라가야 활성화가 실제로 늘어난 것입니다. 가입 곡선만 올라가는 것은 성과가 아닙니다."))

	body += tool.Baro_card_ex(tool.Baro_t("활성화 추이"), tool.Baro_t("구간별 유효 기여자 수"), trend_body, "")

	return baro_metric_frame(db, config, tool.Baro_t("활성화 지표"), "/baro/metrics/activation", span, body)
}
