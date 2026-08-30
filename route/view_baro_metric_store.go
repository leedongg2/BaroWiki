package route

import (
	"database/sql"

	"opennamu/route/tool"
)

// ============================================================
//  18장 — baro_metric 표 읽기 / 쓰기
//
//  방문 수처럼 "그때그때 세어 둬야만 알 수 있는" 값은
//  tool.Baro_metric_bump 으로 쌓인 baro_metric 표에서 읽는다.
//  쌓인 값이 없으면 지어내지 않고 "아직 집계 없음" 으로 표시한다.
// ============================================================

// baro_metric_counter_list : 이 모듈이 읽는 집계 키 (키, 이름, 설명)
func baro_metric_counter_list() [][]string {
	return [][]string{
		{"landing_view", tool.Baro_t("랜딩 방문"), tool.Baro_t("바로위키 첫 화면이 열린 횟수")},
		{"signup_view", tool.Baro_t("가입 화면 방문"), tool.Baro_t("가입 화면이 열린 횟수")},
		{"signup_done", tool.Baro_t("가입 완료"), tool.Baro_t("가입이 실제로 끝난 횟수")},
		{"search_in", tool.Baro_t("검색 유입"), tool.Baro_t("외부 검색 결과를 거쳐 들어온 횟수")},
		{"invite_view", tool.Baro_t("초대 링크 열람"), tool.Baro_t("초대 링크가 열린 횟수")},
		{"card_view", tool.Baro_t("공유 카드 열람"), tool.Baro_t("공유 카드가 열린 횟수")},
	}
}

// baro_metric_counter_name : 집계 키 이름
func baro_metric_counter_name(key string) string {
	for _, row := range baro_metric_counter_list() {
		if row[0] == key {
			return row[1]
		}
	}

	return key
}

// baro_metric_series_row : 집계 키 한 개의 기간별 값
type baro_metric_series_row struct {
	Key    string
	Name   string
	Desc   string
	Labels []string
	Values []int
	Total  int
	Ever   bool // 이 키로 한 번이라도 집계된 적이 있는지
}

// baro_metric_series_get : 집계 키의 기간별 값 (없는 구간은 0)
func baro_metric_series_get(db *sql.DB, key string, span baro_metric_span) baro_metric_series_row {
	periods := baro_metric_periods(span)

	bucket := map[string]int{}
	for _, period := range periods {
		bucket[period] = 0
	}

	rows := baro_metric_col2(
		db,
		"select period, value from baro_metric where key_name = ? and period >= ? and period <= ?",
		key, baro_metric_range_low(span), baro_metric_range_high(span),
	)

	for _, row := range rows {
		slot := baro_metric_period_of(span, row[0])
		if slot == "" {
			continue
		}

		if _, ok := bucket[slot]; !ok {
			continue
		}

		bucket[slot] += tool.Str_to_int(row[1])
	}

	out := baro_metric_series_row{Key: key, Name: baro_metric_counter_name(key), Labels: periods}

	for _, row := range baro_metric_counter_list() {
		if row[0] == key {
			out.Desc = row[2]
		}
	}

	for _, period := range periods {
		out.Values = append(out.Values, bucket[period])
		out.Total += bucket[period]
	}

	out.Ever = baro_metric_int(db, "select count(*) from baro_metric where key_name = ?", key) > 0

	return out
}

// baro_metric_put : baro_metric 에 값을 확정 저장 (같은 키·구간이면 덮어쓴다)
func baro_metric_put(db *sql.DB, key string, period string, value string, detail string) {
	old := ""
	found := tool.QueryRow_DB(
		db,
		"select value from baro_metric where key_name = ? and period = ?",
		[]any{&old}, key, period,
	)

	if found {
		tool.Exec_DB(
			db,
			"update baro_metric set value = ?, detail = ?, date = ? where key_name = ? and period = ?",
			value, detail, tool.Baro_now(), key, period,
		)

		return
	}

	tool.Exec_DB(
		db,
		"insert into baro_metric (id, key_name, period, value, detail, date) values (?, ?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_metric"), key, period, value, detail, tool.Baro_now(),
	)
}

// baro_metric_snap_list : 스냅샷으로 저장하는 지표 (키, 이름, 설명)
//
//	접두사 snap_ 을 붙여 방문 집계용 키와 섞이지 않게 한다.
func baro_metric_snap_list() [][]string {
	return [][]string{
		{"snap_weekly_effective", tool.Baro_t("주간 유효 기여자 수"), tool.Baro_t("★ 절대 핵심 지표. 최근 7일 안에 되돌려지지 않은 기여를 한 사람 수")},
		{"snap_weekly_event", tool.Baro_t("주간 유효 기여 건수"), tool.Baro_t("최근 7일 안의 되돌려지지 않은 기여 건수")},
		{"snap_month_core", tool.Baro_t("이번 달 핵심 기여자 수"), tool.Baro_t("이번 달 유효 기여가 5건 이상인 사람 수")},
		{"snap_new_user", tool.Baro_t("오늘 신규 가입"), tool.Baro_t("오늘 가입한 사용자 수 (보조 지표)")},
		{"snap_total_user", tool.Baro_t("누적 가입자 수"), tool.Baro_t("가입자 총수 (보조 지표 — 이 수만 보고 판단하지 않는다)")},
		{"snap_first_contrib", tool.Baro_t("첫 기여 도달 인원"), tool.Baro_t("가입 뒤 첫 실제 기여까지 간 사람의 누적 수")},
		{"snap_revert_rate", tool.Baro_t("편집 되돌림률(‱)"), tool.Baro_t("검토 결과가 되돌림인 비율 (만분율로 저장)")},
		{"snap_citation_rate", tool.Baro_t("출처 포함률(‱)"), tool.Baro_t("출처가 하나라도 달린 문서 비율 (만분율로 저장)")},
		{"snap_broken_link", tool.Baro_t("깨진 링크 수"), tool.Baro_t("baro_citation 에서 status 가 broken 인 출처 수")},
		{"snap_review_wait", tool.Baro_t("검토 대기 건수"), tool.Baro_t("아직 검토가 끝나지 않은 baro_review 건수")},
		{"snap_open_case", tool.Baro_t("미종결 사건 수"), tool.Baro_t("아직 닫히지 않은 baro_case 건수")},
		{"snap_open_report", tool.Baro_t("미처리 신고 수"), tool.Baro_t("아직 처리되지 않은 baro_report 건수")},
		{"snap_open_appeal", tool.Baro_t("미처리 이의제기 수"), tool.Baro_t("아직 결론이 나지 않은 baro_appeal 건수")},
		{"snap_quarantine_open", tool.Baro_t("격리 중 문서 수"), tool.Baro_t("아직 풀리지 않은 baro_quarantine 건수")},
	}
}

// baro_metric_snap_name : 스냅샷 키 이름
func baro_metric_snap_name(key string) string {
	for _, row := range baro_metric_snap_list() {
		if row[0] == key {
			return row[1]
		}
	}

	return baro_metric_counter_name(key)
}

// baro_metric_snap_desc : 스냅샷 키 설명
func baro_metric_snap_desc(key string) string {
	for _, row := range baro_metric_snap_list() {
		if row[0] == key {
			return row[2]
		}
	}

	for _, row := range baro_metric_counter_list() {
		if row[0] == key {
			return row[2]
		}
	}

	return tool.Baro_t("이 위키에서 직접 정한 집계 키입니다.")
}

// baro_metric_snap_value : 오늘 저장할 스냅샷 값 계산 (키 → 값)
func baro_metric_snap_value(db *sql.DB) [][]string {
	week_from, week_to, _ := baro_metric_week_range(0)
	month := tool.Get_month()
	today := tool.Baro_today()

	out := [][]string{}

	out = append(out, []string{"snap_weekly_effective", tool.Baro_itoa(baro_metric_effective_users(db, week_from, week_to))})
	out = append(out, []string{"snap_weekly_event", tool.Baro_itoa(baro_metric_effective_events(db, week_from, week_to))})

	// 이번 달 핵심 기여자 (월 5건 이상)
	core := 0
	for _, row := range baro_metric_col2(
		db,
		"select user, count(*) from baro_contrib_event where date >= ? and date <= ? and user != ''"+baro_metric_bad_status+" group by user",
		month+"-01 00:00:00", month+"-31 23:59:59",
	) {
		if tool.Str_to_int(row[1]) >= 5 {
			core++
		}
	}
	out = append(out, []string{"snap_month_core", tool.Baro_itoa(core)})

	out = append(out, []string{"snap_new_user", tool.Baro_itoa(baro_metric_int(
		db, "select count(*) from user_set where name = 'date' and data >= ? and data <= ?", today+" 00:00:00", today+" 23:59:59",
	))})
	out = append(out, []string{"snap_total_user", tool.Baro_itoa(baro_metric_int(
		db, "select count(*) from user_set where name = 'date'",
	))})
	out = append(out, []string{"snap_first_contrib", tool.Baro_itoa(baro_metric_int(
		db, "select count(distinct user) from baro_contrib_event where user != ''"+baro_metric_bad_status,
	))})

	// 되돌림률 / 출처 포함률은 만분율(0~10000)로 저장해 소수점을 잃지 않게 한다
	review_total := baro_metric_int(db, "select count(*) from baro_review")
	review_revert := baro_metric_int(db, "select count(*) from baro_review where status = 'reverted'")
	out = append(out, []string{"snap_revert_rate", tool.Baro_itoa(baro_metric_permil(review_revert, review_total))})

	doc_total := baro_metric_doc_total(db)
	doc_cited := baro_metric_int(db, "select count(distinct title) from baro_citation where title != ''")
	out = append(out, []string{"snap_citation_rate", tool.Baro_itoa(baro_metric_permil(doc_cited, doc_total))})

	out = append(out, []string{"snap_broken_link", tool.Baro_itoa(baro_metric_int(
		db, "select count(*) from baro_citation where status = 'broken'",
	))})
	out = append(out, []string{"snap_review_wait", tool.Baro_itoa(baro_metric_int(
		db, "select count(*) from baro_review where status in ('open', 'pending', 'new', '')",
	))})
	out = append(out, []string{"snap_open_case", tool.Baro_itoa(baro_metric_int(
		db, "select count(*) from baro_case where status not in ('closed', 'done', 'rejected', 'expired')",
	))})
	out = append(out, []string{"snap_open_report", tool.Baro_itoa(baro_metric_int(
		db, "select count(*) from baro_report where status in ('open', 'pending', 'new', '')",
	))})
	out = append(out, []string{"snap_open_appeal", tool.Baro_itoa(baro_metric_int(
		db, "select count(*) from baro_appeal where status in ('open', 'pending', 'new', '')",
	))})
	out = append(out, []string{"snap_quarantine_open", tool.Baro_itoa(baro_metric_int(
		db, "select count(*) from baro_quarantine where status not in ('released', 'deleted', 'closed')",
	))})

	return out
}

// baro_metric_permil : 만분율 (0 ~ 10000) — 소수 두 자리를 정수로 보관하기 위한 것
func baro_metric_permil(done int, total int) int {
	if total <= 0 {
		return 0
	}

	value := float64(done) * 10000 / float64(total)

	return int(value + 0.5)
}

// baro_metric_permil_text : 만분율 값을 백분율 문자열로
func baro_metric_permil_text(value int) string {
	return baro_metric_round1(float64(value)/100) + "%"
}

// baro_metric_doc_total : 문서 총수 (사용자·파일·분류 이름공간 제외)
func baro_metric_doc_total(db *sql.DB) int {
	return baro_metric_int(
		db,
		"select count(*) from data where title not like 'user:%' and title not like 'file:%' and title not like 'category:%' and title not like 'trash:%' and title not like 'filetrash:%'",
	)
}

// baro_metric_snap_series : 저장된 스냅샷의 기간별 값
func baro_metric_snap_series(db *sql.DB, key string, span baro_metric_span) ([]string, []float64, bool) {
	periods := baro_metric_periods(span)

	bucket := map[string]float64{}
	seen := map[string]bool{}

	rows := baro_metric_col2(
		db,
		"select period, value from baro_metric where key_name = ? and period >= ? and period <= ? order by period",
		key, baro_metric_range_low(span), baro_metric_range_high(span),
	)

	for _, row := range rows {
		slot := baro_metric_period_of(span, row[0])
		if slot == "" {
			continue
		}

		// 스냅샷은 그날의 상태 값이므로 더하지 않고 마지막 값을 쓴다
		bucket[slot] = float64(tool.Str_to_int(row[1]))
		seen[slot] = true
	}

	values := []float64{}
	has := false
	for _, period := range periods {
		values = append(values, bucket[period])
		if seen[period] {
			has = true
		}
	}

	return periods, values, has
}
