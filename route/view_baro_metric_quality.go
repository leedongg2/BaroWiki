package route

import (
	"opennamu/route/tool"
)

// ============================================================
//  18장 — 품질 지표 (/baro/metrics/quality)
//
//  많이 쓰였는가가 아니라, 쓰인 것이 남을 만한가를 본다.
//  편집 되돌림률 · 출처 포함률 · 신규 문서 유지율 ·
//  검토 대기 시간 · 깨진 링크 · 격리 후 복구 비율
// ============================================================

// View_baro_metric_quality : 품질 지표
func View_baro_metric_quality(config tool.Config, span_key string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, stop := baro_metric_guard(db, config); stop {
		return deny
	}

	span := baro_metric_span_get(span_key)
	from := baro_metric_span_start(span)
	to := baro_metric_span_end(span)

	body := ""
	body += tool.Baro_alert("info",
		tool.Baro_t(`품질 지표는 "쌓인 글이 남을 만한가" 를 봅니다. 기여 수가 늘어도 되돌림률과 깨진 링크가 함께 늘면 `)+
			tool.Baro_t(`실제로는 뒷정리 부담만 늘어난 것입니다.`))

	// --- 1. 편집 되돌림률 ---
	review_total := baro_metric_int(db, "select count(*) from baro_review where date >= ? and date <= ?", from, to)
	review_revert := baro_metric_int(db, "select count(*) from baro_review where status = 'reverted' and date >= ? and date <= ?", from, to)
	review_reject := baro_metric_int(db, "select count(*) from baro_review where status = 'rejected' and date >= ? and date <= ?", from, to)
	review_all := baro_metric_int(db, "select count(*) from baro_review")

	revert_items := []baro_metric_item{
		baro_metric_rate(tool.Baro_t("편집 되돌림률"), review_revert, review_total, "건", tool.Baro_t("검토된 편집 중 되돌려진 비율입니다. 높으면 새 사람의 편집이 자주 무효가 된다는 뜻입니다.")),
		baro_metric_rate(tool.Baro_t("반려율"), review_reject, review_total, "건", tool.Baro_t("검토 결과가 반려인 비율입니다. 되돌림과 달리 본문에 반영되기 전에 막힌 경우입니다.")),
		baro_metric_num(tool.Baro_t("검토 건수"), review_total, span.Name, tool.Baro_t("이 기간에 만들어진 검토 기록 수입니다. 위 비율의 분모입니다.")),
		baro_metric_num(tool.Baro_t("누적 검토 건수"), review_all, tool.Baro_t("전체"), tool.Baro_t("지금까지 쌓인 검토 기록 전체 수입니다.")),
	}

	revert_body := baro_metric_cards(revert_items, 4)

	revert_labels := baro_metric_periods(span)
	revert_series := []float64{}

	all_bucket := baro_metric_bucket(span, baro_metric_col1(db, "select date from baro_review where date >= ? and date <= ?", from, to))
	rev_bucket := baro_metric_bucket(span, baro_metric_col1(db, "select date from baro_review where status = 'reverted' and date >= ? and date <= ?", from, to))

	for i := range revert_labels {
		revert_series = append(revert_series, tool.Baro_pct(rev_bucket[i], all_bucket[i]))
	}

	revert_body += baro_metric_chart("line", revert_labels, revert_series, "%", tool.Baro_t("편집 되돌림률 추이 (")+span.Name+")")
	revert_body += baro_metric_chart_int("bar", revert_labels, all_bucket, "건", tool.Baro_t("검토 건수 추이"))

	if review_all == 0 {
		revert_body += tool.Baro_empty(tool.Baro_t("아직 데이터가 없습니다. baro_review 표에 검토 기록이 하나도 없습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("편집 되돌림률"), tool.Baro_t("baro_review 의 reverted 비율"), revert_body, "")

	// --- 2. 출처 포함률 ---
	doc_total := baro_metric_doc_total(db)
	doc_cited := baro_metric_int(db, "select count(distinct title) from baro_citation where title != '' and title in (select title from data)")
	cite_total := baro_metric_int(db, "select count(*) from baro_citation")
	cite_new := baro_metric_int(db, "select count(*) from baro_citation where date >= ? and date <= ?", from, to)

	cite_items := []baro_metric_item{
		baro_metric_rate(tool.Baro_t("출처 포함률"), doc_cited, doc_total, "개 문서", tool.Baro_t("출처가 하나라도 달린 문서의 비율입니다. 사용자·파일·분류 이름공간은 분모에서 뺍니다.")),
		baro_metric_num(tool.Baro_t("출처가 달린 문서"), doc_cited, "누적", tool.Baro_t("baro_citation 에 출처가 한 개 이상 기록된 문서 수입니다.")),
		baro_metric_num(tool.Baro_t("전체 출처 수"), cite_total, "누적", tool.Baro_t("지금까지 기록된 출처 항목의 총수입니다.")),
		baro_metric_num(tool.Baro_t("이 기간에 추가된 출처"), cite_new, span.Name, tool.Baro_t("이 기간에 새로 기록된 출처 수입니다.")),
	}

	cite_body := baro_metric_cards(cite_items, 4)
	cite_body += tool.Baro_progress(doc_cited, doc_total, tool.Baro_t("출처가 달린 문서 비율"))
	cite_body += baro_metric_chart_int("bar", revert_labels,
		baro_metric_bucket(span, baro_metric_col1(db, "select date from baro_citation where date >= ? and date <= ?", from, to)),
		"건", tool.Baro_t("출처 추가 추이 (")+span.Name+")")

	// 출처 품질 분포
	quality_rows := [][]string{}
	for _, row := range baro_metric_col2(db, "select quality, count(*) from baro_citation group by quality") {
		name := row[0]
		if name == "" {
			name = tool.Baro_t("(품질 표시 없음)")
		}

		quality_rows = append(quality_rows, []string{tool.HTML_escape(name), tool.Baro_comma(tool.Str_to_int(row[1])), tool.Baro_pct_text(tool.Str_to_int(row[1]), cite_total)})
	}

	cite_body += tool.Baro_table([]string{tool.Baro_t("출처 품질"), tool.Baro_t("건수"), tool.Baro_t("비중")}, quality_rows, tool.Baro_t("아직 기록된 출처가 없습니다."))

	if doc_total == 0 {
		cite_body += tool.Baro_empty(tool.Baro_t("문서가 아직 하나도 없어 출처 포함률을 계산할 수 없습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("출처 포함률"), tool.Baro_t("baro_citation 이 있는 문서 비율"), cite_body, "")

	// --- 3. 신규 문서 유지율 ---
	new_total := baro_metric_int(
		db,
		"select count(distinct title) from baro_contrib_event where kind = 'new_document' and title != '' and date >= ? and date <= ?",
		from, to,
	)
	new_alive := baro_metric_int(
		db,
		"select count(distinct title) from baro_contrib_event where kind = 'new_document' and title != '' and date >= ? and date <= ? and title in (select title from data)",
		from, to,
	)
	new_quarantine := baro_metric_int(
		db,
		"select count(distinct title) from baro_contrib_event where kind = 'new_document' and title != '' and date >= ? and date <= ? and title in (select title from baro_quarantine where status not in ('released', 'deleted'))",
		from, to,
	)

	survive_items := []baro_metric_item{
		baro_metric_rate(tool.Baro_t("신규 문서 유지율"), new_alive, new_total, "개", tool.Baro_t("이 기간에 만들어진 문서 중 지금도 남아 있는 비율입니다. 낮으면 만들자마자 지워지는 문서가 많다는 뜻입니다.")),
		baro_metric_num(tool.Baro_t("이 기간에 만들어진 문서"), new_total, span.Name, tool.Baro_t("baro_contrib_event 의 new_document 기록 기준 문서 수입니다.")),
		baro_metric_num(tool.Baro_t("지금도 남아 있는 문서"), new_alive, tool.Baro_t("data 표 기준"), tool.Baro_t("위 문서 중 현재 문서 표에 그대로 남아 있는 수입니다.")),
		baro_metric_num(tool.Baro_t("격리 중인 신규 문서"), new_quarantine, "baro_quarantine", tool.Baro_t("위 문서 중 지금 임시 격리 상태인 수입니다. 삭제가 아니라 되돌릴 수 있는 상태입니다.")),
	}

	survive_body := baro_metric_cards(survive_items, 4)
	survive_body += tool.Baro_progress(new_alive, new_total, tool.Baro_t("남아 있는 신규 문서"))
	survive_body += baro_metric_chart_int("bar", revert_labels,
		baro_metric_bucket(span, baro_metric_col1(db, "select date from baro_contrib_event where kind = 'new_document' and date >= ? and date <= ?", from, to)),
		"개", tool.Baro_t("새 문서 작성 추이 (")+span.Name+")")

	if new_total == 0 {
		survive_body += tool.Baro_empty(tool.Baro_t("이 기간에 새로 만들어진 문서 기록이 없습니다. 아직 데이터가 없습니다."))
	}

	survive_body += tool.Baro_note(tool.Baro_t("0장 2번 원칙에 따라 삭제 기본 경로는 임시 격리입니다. 격리된 문서는 '사라진 것' 이 아니라 '되돌릴 수 있는 것' 으로 따로 셉니다."))

	body += tool.Baro_card_ex(tool.Baro_t("신규 문서 유지율"), tool.Baro_t("만들어진 뒤 아직 남아 있는 비율"), survive_body, "")

	// --- 4. 검토 대기 시간 ---
	wait_rows := baro_metric_col2(db, "select date, review_date from baro_review where review_date != '' and date >= ? and date <= ?", from, to)

	wait_hours := []float64{}
	for _, row := range wait_rows {
		gap := baro_metric_gap_hours(row[0], row[1])
		if gap < 0 {
			continue
		}

		wait_hours = append(wait_hours, gap)
	}

	open_review := baro_metric_col1(db, "select date from baro_review where review_date = '' or review_date is null")
	oldest_wait := 0.0
	now := tool.Baro_now()
	for _, date := range open_review {
		gap := baro_metric_gap_hours(date, now)
		if gap > oldest_wait {
			oldest_wait = gap
		}
	}

	wait_items := []baro_metric_item{
		baro_metric_text_item(tool.Baro_t("검토 대기 시간(평균)"), baro_metric_hours_text(baro_metric_mean(wait_hours)), tool.Baro_comma(len(wait_hours))+tool.Baro_t("건 기준"),
			"검토 요청이 들어온 뒤 결론이 날 때까지 걸린 평균 시간입니다. (baro_review 의 date ~ review_date)", len(wait_hours) == 0),
		baro_metric_text_item(tool.Baro_t("검토 대기 시간(중앙값)"), baro_metric_hours_text(baro_metric_median(wait_hours)), tool.Baro_comma(len(wait_hours))+tool.Baro_t("건 기준"),
			"절반의 검토는 이 시간 안에 끝났다는 뜻입니다. 평균보다 실제 체감에 가깝습니다.", len(wait_hours) == 0),
		baro_metric_num(tool.Baro_t("아직 검토 대기 중"), len(open_review), tool.Baro_t("전체"), tool.Baro_t("결론이 나지 않은 검토 건수입니다. 쌓이면 새 사람의 첫 기여가 계속 공중에 뜹니다.")),
		baro_metric_text_item(tool.Baro_t("가장 오래 기다린 건"), baro_metric_hours_text(oldest_wait), tool.Baro_t("대기 중 최장"),
			"아직 결론이 나지 않은 검토 중 가장 오래된 것이 기다린 시간입니다.", len(open_review) == 0),
	}

	wait_body := baro_metric_cards(wait_items, 4)

	// 대기 시간 분포
	wait_names := []string{tool.Baro_t("1시간 이내"), tool.Baro_t("1~6시간"), tool.Baro_t("6~24시간"), tool.Baro_t("1~3일"), tool.Baro_t("3~7일"), tool.Baro_t("7일 이상")}
	wait_hist := make([]int, len(wait_names))
	for _, hours := range wait_hours {
		switch {
		case hours < 1:
			wait_hist[0]++
		case hours < 6:
			wait_hist[1]++
		case hours < 24:
			wait_hist[2]++
		case hours < 72:
			wait_hist[3]++
		case hours < 168:
			wait_hist[4]++
		default:
			wait_hist[5]++
		}
	}

	wait_body += baro_metric_chart_int("bar", wait_names, wait_hist, "건", tool.Baro_t("검토 대기 시간 분포"))

	if len(wait_hours) == 0 {
		wait_body += tool.Baro_empty(tool.Baro_t("이 기간에 결론이 난 검토 기록이 없습니다. 아직 데이터가 없습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("검토 대기 시간"), tool.Baro_t("baro_review 의 date ~ review_date"), wait_body, "")

	// --- 5. 깨진 링크 ---
	broken := baro_metric_int(db, "select count(*) from baro_citation where status = 'broken'")
	broken_docs := baro_metric_int(db, "select count(distinct title) from baro_citation where status = 'broken' and title != ''")
	archived := baro_metric_int(db, "select count(*) from baro_citation where archive_url != ''")

	broken_items := []baro_metric_item{
		baro_metric_num(tool.Baro_t("깨진 링크"), broken, "status = broken", tool.Baro_t("더 이상 열리지 않는 출처 링크 수입니다. 그대로 두면 출처가 있다는 표시만 남고 확인은 불가능해집니다.")),
		baro_metric_rate(tool.Baro_t("깨진 링크 비율"), broken, cite_total, "건", tool.Baro_t("전체 출처 중 깨진 링크의 비율입니다.")),
		baro_metric_num(tool.Baro_t("깨진 링크가 있는 문서"), broken_docs, tool.Baro_t("문서 수"), tool.Baro_t("깨진 출처가 하나라도 있는 문서 수입니다.")),
		baro_metric_rate(tool.Baro_t("보존본이 있는 출처"), archived, cite_total, "건", tool.Baro_t("archive_url 이 채워진 출처의 비율입니다. 이 값이 높을수록 링크가 깨져도 확인할 수 있습니다.")),
	}

	broken_body := baro_metric_cards(broken_items, 4)

	broken_list := baro_metric_col3(
		db,
		"select title, count(*), max(date) from baro_citation where status = 'broken' and title != '' group by title order by count(*) desc limit 20",
	)

	broken_rows := [][]string{}
	for _, row := range broken_list {
		broken_rows = append(broken_rows, []string{
			tool.Baro_doc_link(row[0]),
			tool.Baro_comma(tool.Str_to_int(row[1])),
			tool.Baro_time_cell(row[2]),
		})
	}

	broken_body += tool.Baro_table([]string{"문서", tool.Baro_t("깨진 출처 수"), tool.Baro_t("마지막 기록")}, broken_rows, tool.Baro_t("깨진 링크로 표시된 출처가 없습니다."))

	body += tool.Baro_card_ex(tool.Baro_t("깨진 링크"), tool.Baro_t("baro_citation 의 status = broken"), broken_body, "")

	// --- 6. 격리 후 복구 비율 ---
	q_total := baro_metric_int(db, "select count(*) from baro_quarantine")
	q_released := baro_metric_int(db, "select count(*) from baro_quarantine where status = 'released'")
	q_open := baro_metric_int(db, "select count(*) from baro_quarantine where status not in ('released', 'deleted', 'closed')")
	q_new := baro_metric_int(db, "select count(*) from baro_quarantine where date >= ? and date <= ?", from, to)

	q_hours := []float64{}
	for _, row := range baro_metric_col2(db, "select date, release_date from baro_quarantine where release_date != '' and status = 'released'") {
		gap := baro_metric_gap_hours(row[0], row[1])
		if gap < 0 {
			continue
		}

		q_hours = append(q_hours, gap)
	}

	q_items := []baro_metric_item{
		baro_metric_rate(tool.Baro_t("격리 후 복구 비율"), q_released, q_total, "건", tool.Baro_t("임시 격리된 문서 중 다시 풀린 비율입니다. 너무 낮으면 격리가 사실상 삭제로 쓰이고 있다는 뜻입니다.")),
		baro_metric_num(tool.Baro_t("격리 중"), q_open, tool.Baro_t("아직 안 풀림"), tool.Baro_t("지금 격리 상태로 남아 있는 건수입니다.")),
		baro_metric_num(tool.Baro_t("이 기간의 격리"), q_new, span.Name, tool.Baro_t("이 기간에 새로 격리된 건수입니다.")),
		baro_metric_text_item(tool.Baro_t("복구까지 걸린 시간(중앙값)"), baro_metric_hours_text(baro_metric_median(q_hours)), tool.Baro_comma(len(q_hours))+tool.Baro_t("건 기준"),
			"격리된 뒤 다시 풀리기까지 걸린 시간의 중앙값입니다.", len(q_hours) == 0),
	}

	q_body := baro_metric_cards(q_items, 4)
	q_body += tool.Baro_progress(q_released, q_total, tool.Baro_t("격리 후 복구된 비율"))

	q_rows := [][]string{}
	for _, row := range baro_metric_col2(db, "select status, count(*) from baro_quarantine group by status") {
		name := row[0]
		if name == "" {
			name = tool.Baro_t("(상태 없음)")
		}

		q_rows = append(q_rows, []string{tool.HTML_escape(name), tool.Baro_comma(tool.Str_to_int(row[1])), tool.Baro_pct_text(tool.Str_to_int(row[1]), q_total)})
	}

	q_body += tool.Baro_table([]string{tool.Baro_t("격리 상태"), tool.Baro_t("건수"), tool.Baro_t("비중")}, q_rows, tool.Baro_t("아직 격리 기록이 없습니다."))
	q_body += tool.Baro_note(tool.Baro_t("0장 2번 원칙 : 삭제보다 보존과 복구가 먼저입니다. 격리는 되돌릴 수 있어야 하며, 복구 비율은 그 약속이 지켜지는지를 보는 지표입니다."))

	body += tool.Baro_card_ex(tool.Baro_t("격리 후 복구 비율"), tool.Baro_t("baro_quarantine 의 released / 전체"), q_body, "")

	return baro_metric_frame(db, config, tool.Baro_t("품질 지표"), "/baro/metrics/quality", span, body)
}
