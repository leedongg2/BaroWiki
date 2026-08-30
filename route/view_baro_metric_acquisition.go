package route

import (
	"database/sql"

	"opennamu/route/tool"
)

// ============================================================
//  18장 — 유입 지표 (/baro/metrics/acquisition)
//
//  랜딩 방문 → 가입 전환 · 초대 링크 전환 · 검색 유입 ·
//  공유 카드 유입 · 분야별 신규 가입
// ============================================================

// baro_metric_signup_dates : 기간 안에 가입한 사람들의 가입 시각
func baro_metric_signup_dates(db *sql.DB, span baro_metric_span) []string {
	return baro_metric_col1(
		db,
		"select data from user_set where name = 'date' and data >= ? and data <= ?",
		baro_metric_span_start(span), baro_metric_span_end(span),
	)
}

// baro_metric_signup_users : 기간 안에 가입한 사람들의 (아이디, 가입 시각)
func baro_metric_signup_users(db *sql.DB, span baro_metric_span) [][]string {
	return baro_metric_col2(
		db,
		"select id, data from user_set where name = 'date' and data >= ? and data <= ? order by data",
		baro_metric_span_start(span), baro_metric_span_end(span),
	)
}

// baro_metric_counter_block : 집계 키 한 개를 카드 + 그래프로
func baro_metric_counter_block(db *sql.DB, key string, span baro_metric_span) (baro_metric_series_row, string) {
	series := baro_metric_series_get(db, key, span)

	html := ""
	if !series.Ever {
		html += tool.Baro_alert("warn",
			`<b>`+tool.HTML_escape(series.Name)+tool.Baro_t(` : 아직 집계 없음</b><br>`)+
				tool.Baro_t(`이 값은 방문이 일어날 때마다 세어 두어야만 알 수 있습니다. `)+
				tool.Baro_t(`baro_metric 표에 <code>`)+tool.HTML_escape(key)+tool.Baro_t(`</code> 키로 쌓인 기록이 아직 하나도 없습니다.`))

		return series, html
	}

	html += baro_metric_chart_int("bar", series.Labels, series.Values, tool.Baro_t("회"), series.Name+tool.Baro_t(" 추이 (")+span.Name+")")

	return series, html
}

// baro_metric_collect_guide : 집계를 시작하는 방법 안내
func baro_metric_collect_guide() string {
	body := tool.Baro_t(`<p>방문 수는 스스로 쌓이지 않습니다. 화면을 그리는 자리에서 아래 한 줄을 부르면 그날 값이 1씩 올라갑니다.</p>`)
	body += tool.Baro_code("tool.Baro_metric_bump(db, \"landing_view\", tool.Baro_today(), 1)")
	body += tool.Baro_t(`<p>이 화면이 읽는 집계 키는 다음과 같습니다. 값이 쌓이기 전까지는 "아직 집계 없음" 으로만 표시하며, 숫자를 지어내지 않습니다.</p>`)

	rows := [][]string{}
	for _, row := range baro_metric_counter_list() {
		rows = append(rows, []string{
			tool.Baro_code(row[0]),
			tool.HTML_escape(row[1]),
			tool.HTML_escape(row[2]),
		})
	}

	body += tool.Baro_table([]string{tool.Baro_t("집계 키"), tool.Baro_t("이름"), tool.Baro_t("뜻")}, rows, tool.Baro_t("집계 키가 없습니다."))
	body += tool.Baro_note(tool.Baro_t("period 자리에는 tool.Baro_today() (날짜) 를 넣습니다. 월 단위로 보고 싶으면 tool.Get_month() 도 함께 쌓아 두면 됩니다."))

	return tool.Baro_card(tool.Baro_t("집계를 시작하는 방법"), body)
}

// View_baro_metric_acquisition : 유입 지표
func View_baro_metric_acquisition(config tool.Config, span_key string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, stop := baro_metric_guard(db, config); stop {
		return deny
	}

	span := baro_metric_span_get(span_key)
	body := ""

	body += tool.Baro_alert("info",
		tool.Baro_t(`유입 지표는 "사람이 얼마나 들어왔는가" 를 봅니다. 다만 유입이 늘어도 `)+
			tool.Baro_t(`<b>주간 유효 기여자 수</b>가 늘지 않으면 성과가 아닙니다. 항상 <a href="/baro/metrics?span=`)+span.Key+tool.Baro_t(`">요약 탭</a>의 핵심 지표와 함께 보세요.`))

	// --- 1. 랜딩 방문 → 가입 전환 ---
	landing := baro_metric_series_get(db, "landing_view", span)
	signup_view := baro_metric_series_get(db, "signup_view", span)

	signup_dates := baro_metric_signup_dates(db, span)
	signup_count := len(signup_dates)

	items := []baro_metric_item{}

	if landing.Ever {
		items = append(items, baro_metric_num(tool.Baro_t("랜딩 방문"), landing.Total, span.Name, tool.Baro_t("바로위키 첫 화면이 열린 횟수입니다.")))
	} else {
		items = append(items, baro_metric_text_item(tool.Baro_t("랜딩 방문"), tool.Baro_t("아직 집계 없음"), "landing_view", tool.Baro_t("바로위키 첫 화면이 열린 횟수입니다. 아직 세어 둔 값이 없습니다."), true))
	}

	if signup_view.Ever {
		items = append(items, baro_metric_num(tool.Baro_t("가입 화면 방문"), signup_view.Total, span.Name, tool.Baro_t("가입 화면까지 간 횟수입니다.")))
	} else {
		items = append(items, baro_metric_text_item(tool.Baro_t("가입 화면 방문"), tool.Baro_t("아직 집계 없음"), "signup_view", tool.Baro_t("가입 화면까지 간 횟수입니다. 아직 세어 둔 값이 없습니다."), true))
	}

	items = append(items, baro_metric_num(tool.Baro_t("신규 가입"), signup_count, span.Name, tool.Baro_t("이 기간에 실제로 가입이 끝난 사람 수입니다. (user_set 의 가입일 기준)")))

	if landing.Ever && landing.Total > 0 {
		items = append(items, baro_metric_rate(tool.Baro_t("랜딩 → 가입 전환율"), signup_count, landing.Total, tool.Baro_t("회"), tool.Baro_t("랜딩을 본 횟수 대비 가입이 일어난 비율입니다.")))
	} else {
		items = append(items, baro_metric_text_item(tool.Baro_t("랜딩 → 가입 전환율"), tool.Baro_t("계산 불가"), tool.Baro_t("분모 없음"), tool.Baro_t("랜딩 방문 집계가 쌓이기 전에는 전환율을 계산할 수 없습니다. 숫자를 지어내지 않습니다."), true))
	}

	acq_body := baro_metric_cards(items, 4)
	acq_body += baro_metric_chart_int("bar", baro_metric_periods(span), baro_metric_bucket(span, signup_dates), "명", tool.Baro_t("신규 가입 추이 (")+span.Name+")")

	if _, chart := baro_metric_counter_block(db, "landing_view", span); chart != "" {
		acq_body += chart
	}

	body += tool.Baro_card_ex(tool.Baro_t("랜딩 방문 → 가입 전환"), tool.Baro_t("첫 화면을 본 사람 중 몇 명이 가입까지 갔는가"), acq_body, "")

	// --- 2. 초대 링크 전환 ---
	invite_total := baro_metric_int(db, "select count(*) from baro_invite where date >= ? and date <= ?", baro_metric_span_start(span), baro_metric_span_end(span))
	invite_done := baro_metric_int(db, "select count(*) from baro_invite where status = 'done' and date >= ? and date <= ?", baro_metric_span_start(span), baro_metric_span_end(span))
	invite_used := baro_metric_int(db, "select count(*) from baro_invite where used_date != '' and date >= ? and date <= ?", baro_metric_span_start(span), baro_metric_span_end(span))
	invite_all := baro_metric_int(db, "select count(*) from baro_invite")
	invite_all_done := baro_metric_int(db, "select count(*) from baro_invite where status = 'done'")

	invite_items := []baro_metric_item{
		baro_metric_num(tool.Baro_t("발급된 초대 링크"), invite_total, span.Name, tool.Baro_t("이 기간에 만들어진 초대 링크 수입니다.")),
		baro_metric_rate(tool.Baro_t("초대 전환율"), invite_done, invite_total, "건", tool.Baro_t("발급된 초대 중 status 가 done 이 된(가입까지 이어진) 비율입니다.")),
		baro_metric_num(tool.Baro_t("사용된 초대"), invite_used, tool.Baro_t("used_date 기록 기준"), tool.Baro_t("사용 시각이 남은 초대 수입니다. done 과 다르면 중간에 끊긴 초대가 있다는 뜻입니다.")),
		baro_metric_rate(tool.Baro_t("누적 초대 전환율"), invite_all_done, invite_all, "건", tool.Baro_t("기간과 무관하게 지금까지 발급된 모든 초대의 전환율입니다.")),
	}

	invite_body := baro_metric_cards(invite_items, 4)

	invite_dates := baro_metric_col1(
		db,
		"select used_date from baro_invite where status = 'done' and used_date >= ? and used_date <= ?",
		baro_metric_span_start(span), baro_metric_span_end(span),
	)
	invite_body += baro_metric_chart_int("bar", baro_metric_periods(span), baro_metric_bucket(span, invite_dates), "건", tool.Baro_t("초대로 가입이 끝난 건수 추이"))

	if invite_all == 0 {
		invite_body += tool.Baro_empty(tool.Baro_t("아직 발급된 초대 링크가 없습니다. baro_invite 표가 비어 있습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("초대 링크 전환"), tool.Baro_t("baro_invite 의 done 비율"), invite_body, "")

	// --- 3. 검색 유입 ---
	search := baro_metric_series_get(db, "search_in", span)

	search_body := ""
	if search.Ever {
		search_body += baro_metric_cards([]baro_metric_item{
			baro_metric_num(tool.Baro_t("검색 유입"), search.Total, span.Name, tool.Baro_t("외부 검색 결과를 거쳐 들어온 횟수입니다.")),
			baro_metric_rate(tool.Baro_t("검색 유입 → 가입"), signup_count, search.Total, tool.Baro_t("회"), tool.Baro_t("검색으로 들어온 횟수 대비 가입 수입니다. (같은 사람이 여러 번 들어올 수 있어 상한 추정치입니다)")),
		}, 2)
		search_body += baro_metric_chart_int("line", search.Labels, search.Values, tool.Baro_t("회"), tool.Baro_t("검색 유입 추이 (")+span.Name+")")
	} else {
		search_body += baro_metric_cards([]baro_metric_item{
			baro_metric_text_item(tool.Baro_t("검색 유입"), tool.Baro_t("아직 집계 없음"), "search_in", tool.Baro_t("외부 검색 결과를 거쳐 들어온 횟수입니다. 아직 세어 둔 값이 없습니다."), true),
		}, 2)
		search_body += tool.Baro_alert("warn", tool.Baro_t("검색 유입은 <code>search_in</code> 키로 쌓아야 알 수 있습니다. 아래 안내를 참고하세요."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("검색 유입"), tool.Baro_t("외부 검색에서 들어온 사람"), search_body, "")

	// --- 4. 공유 카드 유입 ---
	card_rows := baro_metric_col3(db, "select id, views, date from baro_share_card")
	card_views := 0
	for _, row := range card_rows {
		card_views += tool.Str_to_int(row[1])
	}

	card_new := baro_metric_int(db, "select count(*) from baro_share_card where date >= ? and date <= ?", baro_metric_span_start(span), baro_metric_span_end(span))
	card_view_counter := baro_metric_series_get(db, "card_view", span)

	card_items := []baro_metric_item{
		baro_metric_num(tool.Baro_t("공유 카드 총 열람"), card_views, tool.Baro_t("baro_share_card.views 합계"), tool.Baro_t("지금까지 만들어진 공유 카드가 열린 횟수의 합입니다.")),
		baro_metric_num(tool.Baro_t("만들어진 공유 카드"), len(card_rows), "누적", tool.Baro_t("지금까지 만들어진 공유 카드 수입니다.")),
		baro_metric_num(tool.Baro_t("이 기간에 만든 카드"), card_new, span.Name, tool.Baro_t("이 기간에 새로 만들어진 공유 카드 수입니다.")),
	}

	if card_view_counter.Ever {
		card_items = append(card_items, baro_metric_num(tool.Baro_t("공유 카드 유입"), card_view_counter.Total, span.Name, tool.Baro_t("card_view 키로 쌓인, 공유 카드를 거쳐 들어온 횟수입니다.")))
	} else {
		card_items = append(card_items, baro_metric_text_item(tool.Baro_t("공유 카드 유입"), tool.Baro_t("아직 집계 없음"), "card_view", tool.Baro_t("공유 카드를 거쳐 들어온 횟수입니다. views 합계와 달리 날짜별로 쌓아야 합니다."), true))
	}

	card_body := baro_metric_cards(card_items, 4)

	card_top := baro_metric_col3(db, "select title, views, user from baro_share_card order by views + 0 desc limit 10")
	top_rows := [][]string{}
	for _, row := range card_top {
		top_rows = append(top_rows, []string{
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row[0]), 60)),
			tool.Baro_user_link(db, row[2], config.IP),
			tool.Baro_comma(tool.Str_to_int(row[1])),
		})
	}
	card_body += tool.Baro_table([]string{tool.Baro_t("카드 제목"), tool.Baro_t("만든 사람"), tool.Baro_t("열람 수")}, top_rows, tool.Baro_t("아직 만들어진 공유 카드가 없습니다."))

	if card_view_counter.Ever {
		card_body += baro_metric_chart_int("bar", card_view_counter.Labels, card_view_counter.Values, tool.Baro_t("회"), tool.Baro_t("공유 카드 유입 추이"))
	}

	body += tool.Baro_card_ex(tool.Baro_t("공유 카드 유입"), tool.Baro_t("baro_share_card 의 열람 수"), card_body, "")

	// --- 5. 분야별 신규 가입 ---
	new_users := baro_metric_signup_users(db, span)
	user_set := map[string]bool{}
	for _, row := range new_users {
		user_set[row[0]] = true
	}

	field_count := map[string]int{}
	picked := map[string]bool{}

	for _, row := range baro_metric_col2(db, "select user, field from baro_interest") {
		if !user_set[row[0]] {
			continue
		}

		field_count[row[1]]++
		picked[row[0]] = true
	}

	field_rows := [][]string{}
	for _, field := range tool.Baro_field_list(db) {
		field_rows = append(field_rows, []string{baro_metric_field_name(db, field.Key), tool.Baro_itoa(field_count[field.Key])})
		delete(field_count, field.Key)
	}
	for key, value := range field_count {
		field_rows = append(field_rows, []string{baro_metric_field_name(db, key), tool.Baro_itoa(value)})
	}

	field_body := baro_metric_cards([]baro_metric_item{
		baro_metric_num(tool.Baro_t("신규 가입"), len(new_users), span.Name, tool.Baro_t("이 기간에 가입한 사람 수입니다.")),
		baro_metric_rate(tool.Baro_t("관심분야를 고른 비율"), len(picked), len(new_users), "명", tool.Baro_t("새로 가입한 사람 중 baro_interest 에 관심분야를 남긴 비율입니다. 낮으면 온보딩이 끊긴다는 뜻입니다.")),
	}, 2)

	field_body += baro_metric_rank_table("분야", tool.Baro_t("신규 가입"), field_rows, tool.Baro_t("이 기간에 관심분야를 고른 신규 가입자가 없습니다."))
	field_body += tool.Baro_note(tool.Baro_t("분야별 신규 가입은 baro_interest 에 남은 관심분야 기록을 기준으로 셉니다. 한 사람이 여러 분야를 고르면 각 분야에 모두 셉니다."))

	body += tool.Baro_card_ex(tool.Baro_t("분야별 신규 가입"), tool.Baro_t("baro_interest 기준"), field_body, "")

	// --- 6. 집계 안내 ---
	body += baro_metric_collect_guide()

	return baro_metric_frame(db, config, tool.Baro_t("유입 지표"), "/baro/metrics/acquisition", span, body)
}
