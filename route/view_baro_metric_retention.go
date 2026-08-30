package route

import (
	"database/sql"
	"sort"
	"time"

	"opennamu/route/tool"
)

// ============================================================
//  18장 — 유지 지표 (/baro/metrics/retention)
//
//  한 번 기여한 사람이 다시 오는가. 이 장에서 가장 무거운 질문이다.
//  7일 후 재기여 · 30일 후 재기여 · 프로젝트 재참여 ·
//  월간 핵심 기여자 수 · 분야별 활성 기여자 수
// ============================================================

// baro_metric_user_events : 사람별 유효 기여 시각 목록 (오래된 순)
func baro_metric_user_events(db *sql.DB) map[string][]string {
	out := map[string][]string{}

	for _, row := range baro_metric_col2(
		db,
		"select user, date from baro_contrib_event where user != '' and date != ''"+baro_metric_bad_status+" order by date",
	) {
		out[row[0]] = append(out[row[0]], row[1])
	}

	return out
}

// baro_metric_retention_calc : 첫 기여 후 n 일 안에 다시 기여한 비율
//
//	관찰 기간이 아직 n 일이 지나지 않은 사람은 분모에서 뺀다. (아직 판정할 수 없으므로)
func baro_metric_retention_calc(events map[string][]string, span baro_metric_span, days int) (int, int) {
	limit := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")

	from := baro_metric_span_start(span)
	to := baro_metric_span_end(span)

	total := 0
	back := 0

	for _, list := range events {
		if len(list) == 0 {
			continue
		}

		first := list[0]
		if first < from || first > to {
			continue
		}

		// 아직 n 일이 지나지 않은 사람은 판정 대상이 아니다
		if first > limit {
			continue
		}

		total++

		deadline := tool.Baro_time_add(first, days, 0, 0)
		for _, date := range list[1:] {
			if date > first && date <= deadline {
				back++

				break
			}
		}
	}

	return back, total
}

// baro_metric_month_list : 최근 n 개월 키 목록
func baro_metric_month_list(count int) []string {
	now := time.Now()
	base := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	out := []string{}
	for i := count - 1; i >= 0; i-- {
		out = append(out, base.AddDate(0, -i, 0).Format("2006-01"))
	}

	return out
}

// baro_metric_core_by_month : 월별 핵심 기여자 수 (월 5건 이상)
func baro_metric_core_by_month(db *sql.DB, months []string) ([]int, []int) {
	count := map[string]map[string]int{}
	for _, month := range months {
		count[month] = map[string]int{}
	}

	for _, row := range baro_metric_col2(
		db,
		"select user, date from baro_contrib_event where user != '' and date >= ?"+baro_metric_bad_status,
		months[0]+"-01 00:00:00",
	) {
		if len(row[1]) < 7 {
			continue
		}

		month := row[1][:7]
		if count[month] == nil {
			continue
		}

		count[month][row[0]]++
	}

	core := []int{}
	all := []int{}
	for _, month := range months {
		core_num := 0
		for _, num := range count[month] {
			if num >= 5 {
				core_num++
			}
		}

		core = append(core, core_num)
		all = append(all, len(count[month]))
	}

	return core, all
}

// View_baro_metric_retention : 유지 지표
func View_baro_metric_retention(config tool.Config, span_key string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, stop := baro_metric_guard(db, config); stop {
		return deny
	}

	span := baro_metric_span_get(span_key)
	body := ""

	body += tool.Baro_alert("info",
		tool.Baro_t(`유지 지표는 "한 번 온 사람이 다시 오는가" 를 봅니다. 유입과 활성화가 아무리 좋아도 `)+
			tool.Baro_t(`유지가 0 이면 위키는 매달 처음부터 다시 시작하는 셈입니다.`))

	events := baro_metric_user_events(db)

	// --- 7일 / 30일 재기여 ---
	back7, total7 := baro_metric_retention_calc(events, span, 7)
	back30, total30 := baro_metric_retention_calc(events, span, 30)

	items := []baro_metric_item{
		baro_metric_rate(tool.Baro_t("7일 후 재기여"), back7, total7, "명", tool.Baro_t("첫 기여 후 7일 안에 한 번 더 기여한 사람의 비율입니다. 첫 기여가 아직 7일이 지나지 않은 사람은 분모에서 뺍니다.")),
		baro_metric_rate(tool.Baro_t("30일 후 재기여"), back30, total30, "명", tool.Baro_t("첫 기여 후 30일 안에 한 번 더 기여한 사람의 비율입니다. 한 달 안에 습관이 붙었는지를 봅니다.")),
	}

	// --- 프로젝트 재참여 ---
	project_of := map[string]map[string]bool{}
	for _, row := range baro_metric_col2(db, "select user, project from baro_project_member where user != '' and project != ''") {
		if project_of[row[0]] == nil {
			project_of[row[0]] = map[string]bool{}
		}

		project_of[row[0]][row[1]] = true
	}

	project_one := 0
	project_many := 0
	for _, list := range project_of {
		if len(list) >= 1 {
			project_one++
		}
		if len(list) >= 2 {
			project_many++
		}
	}

	items = append(items, baro_metric_rate(tool.Baro_t("프로젝트 재참여"), project_many, project_one, "명", tool.Baro_t("프로젝트에 참여한 사람 중 서로 다른 프로젝트에 두 개 이상 참여한 비율입니다. (baro_project_member 기준)")))

	// --- 이번 달 핵심 기여자 ---
	months := baro_metric_month_list(12)
	core_series, all_series := baro_metric_core_by_month(db, months)

	this_core := 0
	this_all := 0
	if len(core_series) > 0 {
		this_core = core_series[len(core_series)-1]
		this_all = all_series[len(all_series)-1]
	}

	items = append(items, baro_metric_num(tool.Baro_t("이번 달 핵심 기여자"), this_core, tool.Baro_t("월 5건 이상"), tool.Baro_t("이번 달에 유효 기여가 5건 이상인 사람 수입니다. 위키를 실제로 굴리는 사람의 수에 가장 가깝습니다.")))
	items = append(items, baro_metric_rate(tool.Baro_t("핵심 기여자 비중"), this_core, this_all, "명", tool.Baro_t("이번 달 기여자 중 핵심 기여자의 비율입니다. 너무 높으면 소수에게 쏠려 있다는 뜻이기도 합니다.")))

	body += tool.Baro_card_ex(tool.Baro_t("유지 지표"), tool.Baro_t("모든 값은 baro_contrib_event · baro_project_member 를 직접 센 것입니다"), baro_metric_cards(items, 4), "")

	// --- 재기여 상세 ---
	detail_rows := [][]string{
		{tool.Baro_t("7일 후 재기여"), tool.Baro_comma(back7), tool.Baro_comma(total7), tool.Baro_pct_text(back7, total7), tool.Baro_progress(back7, total7, "")},
		{tool.Baro_t("30일 후 재기여"), tool.Baro_comma(back30), tool.Baro_comma(total30), tool.Baro_pct_text(back30, total30), tool.Baro_progress(back30, total30, "")},
	}

	detail_body := tool.Baro_table([]string{tool.Baro_t("구간"), tool.Baro_t("다시 온 사람"), tool.Baro_t("판정 대상"), tool.Baro_t("비율"), tool.Baro_t("분포")}, detail_rows, "")
	if total7 == 0 && total30 == 0 {
		detail_body += tool.Baro_empty(tool.Baro_t("아직 데이터가 없습니다. 이 기간에 첫 기여를 한 사람이 없거나, 판정할 만큼 시간이 지나지 않았습니다."))
	}
	detail_body += tool.Baro_note(tool.Baro_t("판정 대상은 '첫 기여가 이 기간 안에 있고, 그로부터 해당 일수가 이미 지난 사람' 입니다. 아직 시간이 지나지 않은 사람을 분모에 넣으면 비율이 실제보다 낮게 보이기 때문입니다."))

	body += tool.Baro_card_ex(tool.Baro_t("재기여 상세"), tool.Baro_t("분모를 숨기지 않습니다"), detail_body, "")

	// --- 월간 핵심 기여자 추이 ---
	core_body := baro_metric_chart_int("bar", months, core_series, "명", tool.Baro_t("월간 핵심 기여자 수 (월 5건 이상, 최근 12개월)"))
	core_body += baro_metric_chart_int("line", months, all_series, "명", tool.Baro_t("월간 전체 기여자 수 (최근 12개월)"))

	core_rows := [][]string{}
	for i := len(months) - 1; i >= 0; i-- {
		core_rows = append(core_rows, []string{
			tool.HTML_escape(months[i]),
			tool.Baro_comma(core_series[i]),
			tool.Baro_comma(all_series[i]),
			tool.Baro_pct_text(core_series[i], all_series[i]),
		})
	}

	core_body += tool.Baro_table([]string{"월", tool.Baro_t("핵심 기여자(5건 이상)"), tool.Baro_t("전체 기여자"), tool.Baro_t("핵심 비중")}, core_rows, tool.Baro_t("아직 기여 기록이 없습니다."))
	core_body += tool.Baro_note(tool.Baro_t("핵심 기여자 기준인 '월 5건' 은 0장 4번 원칙(횟수보다 가치)에 따라 점수가 아니라 행동 기록 수로만 셉니다."))

	body += tool.Baro_card_ex(tool.Baro_t("월간 핵심 기여자 수"), tool.Baro_t("최근 12개월"), core_body, "")

	// --- 분야별 활성 기여자 ---
	field_users := map[string]map[string]bool{}
	for _, row := range baro_metric_col2(
		db,
		"select field, user from baro_contrib_event where user != '' and date >= ? and date <= ?"+baro_metric_bad_status,
		baro_metric_span_start(span), baro_metric_span_end(span),
	) {
		if field_users[row[0]] == nil {
			field_users[row[0]] = map[string]bool{}
		}

		field_users[row[0]][row[1]] = true
	}

	field_rows := [][]string{}
	for _, field := range tool.Baro_field_list(db) {
		field_rows = append(field_rows, []string{baro_metric_field_name(db, field.Key), tool.Baro_itoa(len(field_users[field.Key]))})
		delete(field_users, field.Key)
	}

	rest := []string{}
	for key := range field_users {
		rest = append(rest, key)
	}
	sort.Strings(rest)
	for _, key := range rest {
		field_rows = append(field_rows, []string{baro_metric_field_name(db, key), tool.Baro_itoa(len(field_users[key]))})
	}

	field_body := baro_metric_rank_table("분야", tool.Baro_t("활성 기여자"), field_rows, tool.Baro_t("이 기간에 기여한 사람이 없습니다."))
	field_body += tool.Baro_note(tool.Baro_t("분야는 baro_contrib_event 에 기록된 field 값을 씁니다. 문서에 분야가 붙어 있지 않으면 '분야 미지정' 으로 모입니다."))

	body += tool.Baro_card_ex(tool.Baro_t("분야별 활성 기여자 수"), span.Name, field_body, "")

	// --- 기여 유지 곡선 ---
	curve_labels := []string{}
	curve_values := []float64{}
	for _, days := range []int{1, 3, 7, 14, 30, 60, 90} {
		back, total := baro_metric_retention_calc(events, span, days)
		curve_labels = append(curve_labels, tool.Baro_itoa(days)+tool.Baro_t("일"))
		curve_values = append(curve_values, tool.Baro_pct(back, total))
	}

	curve_body := baro_metric_chart("line", curve_labels, curve_values, "%", tool.Baro_t("첫 기여 후 며칠 안에 다시 왔는가 (누적 비율)"))
	curve_body += tool.Baro_note(tool.Baro_t("오른쪽으로 갈수록 값이 올라가는 것은 당연합니다. 중요한 것은 7일 지점의 높이입니다. 여기가 낮으면 첫 주 안에 대부분 떠난 것입니다."))

	body += tool.Baro_card_ex(tool.Baro_t("재기여 곡선"), tool.Baro_t("첫 기여 후 경과일별 재기여 비율"), curve_body, "")

	return baro_metric_frame(db, config, tool.Baro_t("유지 지표"), "/baro/metrics/retention", span, body)
}
