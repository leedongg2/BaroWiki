package route

import (
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  18장 — 지표 스냅샷 저장 / 저장된 추이 / 장기 미처리 목록
//
//  오늘 값을 baro_metric 에 저장해 두어야 나중에 추이를 볼 수 있다.
//  저장은 system 권한이 있는 사람만 할 수 있다.
// ============================================================

// View_baro_metric_snapshot : 오늘 값을 baro_metric 에 저장
func View_baro_metric_snapshot(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/metrics/snapshot"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("지표 스냅샷 저장은 system 권한(sysadmin 역할 또는 위키 관리자)이 있어야 합니다."))
	}

	span := baro_metric_span_get("30")
	today := tool.Baro_today()

	// --- POST : 실제로 저장 ---
	if values != nil {
		if tool.Baro_read_only(db) {
			return tool.Baro_page(db, config, tool.Baro_t("저장할 수 없음"),
				tool.Baro_alert("danger", tool.Baro_t("지금 위키는 읽기 전용입니다. 사유 : ")+tool.HTML_escape(tool.Baro_read_only_reason(db))),
				[][]any{{"baro", "바로위키"}})
		}

		period := strings.TrimSpace(values.Get("period"))
		if period == "" {
			period = today
		}

		if len(period) > 10 {
			period = period[:10]
		}

		saved := baro_metric_snap_value(db)
		names := []string{}

		for _, row := range saved {
			baro_metric_put(db, row[0], period, row[1], "snapshot")
			names = append(names, row[0])
		}

		detail := tool.Baro_json_enc(map[string]any{"period": period, "keys": names})

		tool.Baro_public_log(db, "metric_snapshot", config.IP, "baro_metric/"+period,
			"지표 스냅샷 저장", tool.Baro_itoa(len(saved))+"개 지표")
		tool.Baro_audit(db, config, "metric_snapshot", "baro_metric/"+period, "", detail, "지표 스냅샷 저장")

		return tool.Get_redirect("/baro/metrics/history?span=30")
	}

	// --- GET : 저장될 값을 미리 보여준다 ---
	preview := baro_metric_snap_value(db)

	rows := [][]string{}
	empty_count := 0

	for _, row := range preview {
		old := ""
		found := tool.QueryRow_DB(db, "select value from baro_metric where key_name = ? and period = ?", []any{&old}, row[0], today)

		old_cell := tool.Baro_t(`<span class="bw_muted">아직 없음</span>`)
		if found {
			old_cell = tool.HTML_escape(old)
		}

		value_cell := tool.HTML_escape(row[1])
		if strings.HasSuffix(row[0], "_rate") {
			value_cell = tool.HTML_escape(row[1]) + ` <span class="bw_muted">(` + tool.HTML_escape(baro_metric_permil_text(tool.Str_to_int(row[1]))) + `)</span>`
		}

		if tool.Str_to_int(row[1]) == 0 {
			empty_count++
		}

		rows = append(rows, []string{
			tool.Baro_code(row[0]),
			tool.HTML_escape(baro_metric_snap_name(row[0])),
			value_cell,
			old_cell,
			tool.HTML_escape(baro_metric_snap_desc(row[0])),
		})
	}

	body := ""
	body += tool.Baro_alert("info",
		tool.Baro_t("스냅샷은 지금 이 순간의 값을 <code>baro_metric</code> 표에 날짜와 함께 저장하는 기능입니다. ")+
			tool.Baro_t("매일 한 번 저장해 두면 <a href=\"/baro/metrics/history\">저장된 추이</a> 화면에서 변화를 볼 수 있습니다."))

	body += tool.Baro_card_ex(tool.Baro_t("오늘 저장될 값"), today+tool.Baro_t(" 기준"),
		tool.Baro_table([]string{tool.Baro_t("집계 키"), tool.Baro_t("지표"), tool.Baro_t("오늘 값"), tool.Baro_t("이미 저장된 값"), tool.Baro_t("뜻")}, rows, tool.Baro_t("저장할 지표가 없습니다.")),
		"")

	if empty_count == len(preview) && len(preview) > 0 {
		body += tool.Baro_alert("warn", tool.Baro_t("저장될 값이 전부 0 입니다. 아직 데이터가 없습니다. 그래도 저장해 두면 '이 날 0이었다' 는 사실이 기록으로 남습니다."))
	}

	form := tool.Baro_hidden("do", "save")
	form += tool.Baro_input_type("period", tool.Baro_t("저장할 구간"), today, "date", tool.Baro_t("보통은 오늘 날짜 그대로 둡니다. 같은 구간에 다시 저장하면 값이 덮어써집니다."))
	form += tool.Baro_submit(tool.Baro_t("이 값으로 저장"), "primary")

	body += tool.Baro_card_ex(tool.Baro_t("저장하기"), tool.Baro_t("같은 구간에 저장하면 덮어씁니다"), tool.Baro_form("/baro/metrics/snapshot", form), "")
	body += tool.Baro_note(tool.Baro_t("저장 기록은 공개 운영 로그와 내부 감사 기록에 모두 남습니다."))

	return baro_metric_frame(db, config, tool.Baro_t("지표 스냅샷"), "/baro/metrics", span, body)
}

// View_baro_metric_history : 저장된 지표 추이 (목록 + 쪽 넘기기)
func View_baro_metric_history(config tool.Config, page string, span_key string, key string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, stop := baro_metric_guard(db, config); stop {
		return deny
	}

	span := baro_metric_span_get(span_key)

	page_num := tool.Str_to_int(page)
	if page_num < 1 {
		page_num = 1
	}

	max_count := tool.Baro_page_size
	offset := (page_num - 1) * max_count

	body := ""
	body += tool.Baro_alert("info",
		tool.Baro_t("이 화면은 <code>baro_metric</code> 표에 실제로 저장된 값만 보여 줍니다. ")+
			tool.Baro_t("저장된 적이 없는 날은 줄이 없습니다. 빈 곳을 만들어 채우지 않습니다."))

	// --- 지표 고르기 ---
	options := [][]string{{"", tool.Baro_t("전체 보기")}}
	for _, row := range baro_metric_snap_list() {
		options = append(options, []string{row[0], row[1]})
	}
	for _, row := range baro_metric_counter_list() {
		options = append(options, []string{row[0], row[1]})
	}

	pick := tool.Baro_select("key", tool.Baro_t("지표 고르기"), options, key, tool.Baro_t("고른 지표의 저장된 추이를 그래프로 봅니다."))
	pick += tool.Baro_hidden("span", span.Key)
	pick += tool.Baro_submit("보기", "primary")

	body += tool.Baro_card_ex(tool.Baro_t("지표 고르기"), tool.Baro_t("저장된 값 중에서 고릅니다"),
		`<form class="bw_form" method="get" action="/baro/metrics/history">`+pick+`</form>`, "")

	// --- 고른 지표의 추이 ---
	if key != "" {
		labels, values, has := baro_metric_snap_series(db, key, span)

		chart_body := ""
		if has {
			unit := ""
			draw := values
			if strings.HasSuffix(key, "_rate") {
				unit = "%"
				draw = []float64{}
				for _, value := range values {
					draw = append(draw, value/100)
				}
			}

			chart_body += baro_metric_chart("line", labels, draw, unit, baro_metric_snap_name(key)+tool.Baro_t(" 추이 (")+span.Name+")")
		} else {
			chart_body += tool.Baro_empty(tool.Baro_t("이 기간에 저장된 값이 없습니다. 아직 데이터가 없습니다."))
		}

		chart_body += tool.Baro_note(baro_metric_snap_desc(key))

		body += tool.Baro_card_ex(baro_metric_snap_name(key), tool.HTML_escape(key), chart_body, "")
	}

	// --- 저장된 값 목록 ---
	var rows [][]string
	var total int

	if key == "" {
		total = baro_metric_int(db, "select count(*) from baro_metric")
		rows = baro_metric_col5(
			db,
			"select key_name, period, value, detail, date from baro_metric order by period desc, key_name limit ?, ?",
			offset, max_count,
		)
	} else {
		total = baro_metric_int(db, "select count(*) from baro_metric where key_name = ?", key)
		rows = baro_metric_col5(
			db,
			"select key_name, period, value, detail, date from baro_metric where key_name = ? order by period desc limit ?, ?",
			key, offset, max_count,
		)
	}

	table_rows := [][]string{}
	for _, row := range rows {
		value_cell := tool.HTML_escape(row[2])
		if strings.HasSuffix(row[0], "_rate") {
			value_cell += ` <span class="bw_muted">(` + tool.HTML_escape(baro_metric_permil_text(tool.Str_to_int(row[2]))) + `)</span>`
		}

		detail := row[3]
		if detail == "" {
			detail = "-"
		}

		table_rows = append(table_rows, []string{
			tool.Baro_code(row[0]),
			tool.HTML_escape(baro_metric_snap_name(row[0])),
			tool.HTML_escape(row[1]),
			value_cell,
			tool.HTML_escape(detail),
			tool.Baro_time_cell(row[4]),
		})
	}

	list_body := ""
	if total == 0 {
		list_body += tool.Baro_empty(tool.Baro_t("저장된 지표 값이 아직 하나도 없습니다. 스냅샷을 한 번 저장하면 여기에 쌓이기 시작합니다."))
	} else {
		list_body += tool.Baro_table([]string{tool.Baro_t("집계 키"), tool.Baro_t("지표"), tool.Baro_t("구간"), tool.Baro_t("값"), tool.Baro_t("비고"), tool.Baro_t("저장 시각")}, table_rows,
			tool.Baro_t("이 쪽에는 표시할 값이 없습니다. 앞쪽으로 돌아가세요."))
	}

	list_url := "/baro/metrics/history/{}?span=" + span.Key
	if key != "" {
		list_url += "&key=" + tool.Url_parser(key)
	}

	list_body += tool.Baro_paging(page_num, len(rows), max_count, list_url)
	list_body += tool.Baro_note(tool.Baro_t("전체 ") + tool.Baro_comma(total) + tool.Baro_t("개 기록 중 ") + tool.Baro_itoa(page_num) + tool.Baro_t("쪽입니다. 한 쪽에 ") + tool.Baro_itoa(max_count) + tool.Baro_t("개씩 보여 줍니다."))

	body += tool.Baro_card_ex(tool.Baro_t("저장된 값 목록"), tool.Baro_t("baro_metric 표의 실제 내용"), list_body, "")
	body += tool.Baro_button("/baro/metrics/snapshot", tool.Baro_t("오늘 값 저장하기"), "primary")

	return baro_metric_frame(db, config, tool.Baro_t("저장된 추이"), "/baro/metrics/history", span, body)
}

// View_baro_metric_backlog : 장기 미처리 사건 전체 목록 (목록 + 쪽 넘기기)
func View_baro_metric_backlog(config tool.Config, page string, span_key string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, stop := baro_metric_guard(db, config); stop {
		return deny
	}

	span := baro_metric_span_get(span_key)

	page_num := tool.Str_to_int(page)
	if page_num < 1 {
		page_num = 1
	}

	max_count := tool.Baro_page_size
	offset := (page_num - 1) * max_count

	all := baro_metric_backlog_rows(db)
	days := baro_metric_backlog_days(db)
	now := tool.Baro_now()

	body := ""
	body += tool.Baro_alert("warn",
		tool.Baro_t("이 목록은 담당자를 평가하기 위한 것이 아니라, 사건이 어디에서 멈춰 있는지 찾기 위한 것입니다. ")+
			tool.Baro_t("기준은 접수 후 <b>")+tool.Baro_itoa(days)+tool.Baro_t("일</b> 이상 닫히지 않은 사건입니다."))

	body += baro_metric_cards([]baro_metric_item{
		baro_metric_num(tool.Baro_t("장기 미처리 사건"), len(all), tool.Baro_itoa(days)+tool.Baro_t("일 이상"), tool.Baro_t("기준보다 오래 열려 있는 사건 수입니다.")),
		baro_metric_num(tool.Baro_t("담당자 수"), len(baro_metric_actor_backlog(all)), tool.Baro_t("장기 미처리 보유"), tool.Baro_t("장기 미처리 사건을 들고 있는 담당자 수입니다.")),
	}, 2)

	rows := [][]string{}
	for i := offset; i < offset+max_count && i < len(all); i++ {
		row := all[i]

		number := row[1]
		if number == "" {
			number = "#" + row[0]
		}

		actor := tool.Baro_t(`<span class="bw_muted">담당자 없음</span>`)
		if row[2] != "" {
			actor = tool.Baro_user_link(db, row[2], config.IP)
		}

		wait := baro_metric_gap_hours(row[4], now)

		rows = append(rows, []string{
			tool.HTML_escape(number),
			actor,
			tool.Baro_badge(row[3], "warn"),
			tool.Baro_time_cell(row[4]),
			tool.HTML_escape(baro_metric_hours_text(wait)),
		})
	}

	list_body := ""
	if len(all) == 0 {
		list_body += tool.Baro_empty(tool.Baro_t("장기 미처리 사건이 없습니다. 좋은 상태입니다."))
	} else {
		list_body += tool.Baro_table([]string{"사건 번호", "담당자", "상태", "접수", tool.Baro_t("기다린 시간")}, rows,
			tool.Baro_t("이 쪽에는 표시할 사건이 없습니다. 앞쪽으로 돌아가세요."))
	}

	list_body += tool.Baro_paging(page_num, len(rows), max_count, "/baro/metrics/backlog/{}?span="+span.Key)
	list_body += tool.Baro_note(tool.Baro_t("사건 상세 내용과 신고 원문은 이 화면에 표시하지 않습니다. 지표 화면은 개인정보를 담지 않습니다."))

	body += tool.Baro_card_ex(tool.Baro_t("장기 미처리 사건"), tool.Baro_t("오래 기다린 순서"), list_body, "")

	// 담당자별 묶음
	actor_rows := [][]string{}
	for _, row := range baro_metric_actor_backlog(all) {
		actor_rows = append(actor_rows, []string{row[0], row[1]})
	}

	body += tool.Baro_card_ex(tool.Baro_t("담당자별 묶음"), tool.Baro_t("어디에서 멈춰 있는가"),
		baro_metric_rank_table("담당자", tool.Baro_t("장기 미처리 사건"), actor_rows, tool.Baro_t("장기 미처리 사건이 없습니다.")), "")

	return baro_metric_frame(db, config, tool.Baro_t("장기 미처리 사건"), "/baro/metrics/trust", span, body)
}
