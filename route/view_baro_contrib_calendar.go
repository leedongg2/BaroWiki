package route

import (
	"database/sql"
	"sort"

	"opennamu/route/tool"
)

// ============================================================
//  7-8 활동 캘린더 — /baro/contrib/calendar/<사용자>
//
//  스트릭이 끊기면 죄책감을 주는 구조를 만들지 않는다.
//  "며칠 연속" 을 크게 띄우지 않고, 어떤 날 무엇을 기여했는지만 보여 준다.
//  쉬는 날은 잘못이 아니다.
// ============================================================

// baro_contrib_calendar_days : 잔디밭에 그릴 날 수 (약 1년)
const baro_contrib_calendar_days = 364

// baro_contrib_calendar_day : 하루치 활동
type baro_contrib_calendar_day struct {
	Date   string
	Num    int
	Detail map[string]string
}

// baro_contrib_calendar_load : 사용자의 활동 캘린더 읽기 (날짜 → 하루치)
func baro_contrib_calendar_load(db *sql.DB, user string, from string) map[string]baro_contrib_calendar_day {
	out := map[string]baro_contrib_calendar_day{}

	rows := tool.Query_DB(db, "select date, num, detail from baro_calendar where user = ? and date >= ?", user, from)
	defer rows.Close()

	for rows.Next() {
		date, num, detail := "", "", ""
		if err := rows.Scan(&date, &num, &detail); err != nil {
			continue
		}

		out[date] = baro_contrib_calendar_day{
			Date:   date,
			Num:    tool.Str_to_int(num),
			Detail: tool.Baro_json_dec_str(detail),
		}
	}

	return out
}

// baro_contrib_calendar_level : 하루 기여 수를 잔디 단계로
func baro_contrib_calendar_level(num int) string {
	switch {
	case num <= 0:
		return ""
	case num <= 2:
		return " bw_cal_1"
	case num <= 5:
		return " bw_cal_2"
	case num <= 9:
		return " bw_cal_3"
	}

	return " bw_cal_4"
}

// baro_contrib_calendar_detail_text : 하루치 종류별 집계를 문장으로
func baro_contrib_calendar_detail_text(detail map[string]string) string {
	if len(detail) == 0 {
		return ""
	}

	keys := []string{}
	for key := range detail {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	text := ""
	for _, key := range keys {
		if tool.Str_to_int(detail[key]) <= 0 {
			continue
		}

		if text != "" {
			text += ", "
		}

		text += tool.Baro_contrib_kind_name(key) + " " + detail[key]
	}

	return text
}

// baro_contrib_calendar_grid : 잔디밭 HTML (bw_cal / bw_cal_d / bw_cal_1~4)
func baro_contrib_calendar_grid(days map[string]baro_contrib_calendar_day, start string) string {
	html := `<div class="bw_cal">`

	for index := 0; index <= baro_contrib_calendar_days; index++ {
		date := tool.Baro_date_only(tool.Baro_time_add(start, index, 0, 0))
		day := days[date]

		title := date + tool.Baro_t(" · 기여 없음")
		if day.Num > 0 {
			title = date + " · " + tool.Baro_itoa(day.Num) + tool.Baro_t("건")
			if text := baro_contrib_calendar_detail_text(day.Detail); text != "" {
				title += " (" + text + ")"
			}
		}

		html += `<span class="bw_cal_d` + baro_contrib_calendar_level(day.Num) + `" title="` + tool.HTML_escape(title) + `"></span>`
	}

	return html + `</div>`
}

// View_baro_contrib_calendar : 활동 캘린더 화면
func View_baro_contrib_calendar(config tool.Config, user_raw string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	user := baro_contrib_user_param(user_raw)

	if user == "" {
		body := tool.Baro_crumb([][]string{{"/baro/contrib", "기여 기록"}, {"", "활동 캘린더"}})
		body += tool.Baro_alert("warn", tool.Baro_t("사용자 이름이 비어 있습니다. 주소 뒤에 사용자 이름을 붙여 주세요."))
		body += tool.Baro_button("/baro/contrib", tool.Baro_t("기여 기록으로"), "")

		return tool.Baro_page(db, config, "활동 캘린더", body, baro_contrib_menu())
	}

	start := tool.Baro_date_only(tool.Baro_time_add(tool.Baro_now(), -baro_contrib_calendar_days, 0, 0))
	days := baro_contrib_calendar_load(db, user, start)

	total_days := 0
	total_num := 0
	for _, day := range days {
		if day.Num > 0 {
			total_days++
			total_num += day.Num
		}
	}

	all_days := baro_contrib_count(db, "select count(*) from baro_calendar where user = ?", user)

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/contrib", "기여 기록"},
		{"/baro/contrib/user/" + tool.Url_parser(user), tool.Namespace_display(user)},
		{"", "활동 캘린더"},
	})

	body += tool.Baro_hero(
		user+tool.Baro_t(" 님의 활동 캘린더"),
		tool.Baro_t("어떤 날 무엇을 했는지를 기록합니다. 며칠 연속인지는 세지 않습니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("최근 1년 활동한 날"), tool.Baro_comma(total_days), tool.Baro_t("일")},
			{tool.Baro_t("최근 1년 기여"), tool.Baro_comma(total_num), "건"},
			{tool.Baro_t("전체 활동한 날"), tool.Baro_comma(all_days), tool.Baro_t("일")},
		}),
	)

	body += tool.Baro_tabs([][]string{
		{"/baro/contrib/user/" + tool.Url_parser(user), "기여 기록"},
		{"/baro/contrib/impact/" + tool.Url_parser(user), "기여 영향력"},
		{"/baro/contrib/calendar/" + tool.Url_parser(user), "활동 캘린더"},
		{"/baro/badge/user/" + tool.Url_parser(user), "배지"},
	}, "/baro/contrib/calendar/"+tool.Url_parser(user))

	body += tool.Baro_alert("info",
		tool.Baro_t("<b>연속 기록은 세지 않습니다.</b> ")+
			tool.Baro_t("하루 쉬었다고 무언가 무너지는 화면은 만들지 않습니다. ")+
			tool.Baro_t("바로위키는 매일 접속하게 만드는 것이 아니라, 좋은 기록을 남기는 것이 목적입니다. ")+
			tool.Baro_t("이 화면은 압박을 주기 위한 것이 아니라, 자기 활동을 돌아보기 위한 것입니다."),
	)

	// --- 잔디밭 ---
	body += tool.Baro_card_ex(
		tool.Baro_t("최근 1년"),
		start+tool.Baro_t(" 부터 ")+tool.Baro_today()+tool.Baro_t(" 까지"),
		baro_contrib_calendar_grid(days, start)+
			tool.Baro_note(tool.Baro_t("칸에 마우스를 올리면 그날 무엇을 했는지 볼 수 있습니다. 빈 칸은 쉰 날이며, 그것도 괜찮습니다.")),
		"",
	)

	// --- 월별 활동 ---
	month_num := map[string]int{}
	month_days := map[string]int{}
	for _, day := range days {
		if len(day.Date) < 7 {
			continue
		}

		month := day.Date[:7]
		month_num[month] += day.Num
		if day.Num > 0 {
			month_days[month]++
		}
	}

	months := []string{}
	for month := range month_num {
		months = append(months, month)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(months)))

	month_rows := [][]string{}
	max_num := 1
	for _, num := range month_num {
		if num > max_num {
			max_num = num
		}
	}

	for _, month := range months {
		month_rows = append(month_rows, []string{
			baro_contrib_esc(month),
			tool.Baro_comma(month_days[month]) + tool.Baro_t("일"),
			tool.Baro_comma(month_num[month]) + tool.Baro_t("건"),
			tool.Baro_progress(month_num[month], max_num, ""),
		})
	}

	month_html := tool.Baro_table([]string{tool.Baro_t("달"), tool.Baro_t("활동한 날"), tool.Baro_t("기여"), tool.Baro_t("상대 크기")}, month_rows, tool.Baro_t("월별 기록이 없습니다."))
	if len(month_rows) == 0 {
		month_html = tool.Baro_empty(tool.Baro_t("최근 1년 안에 기록된 활동이 없습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("월별 활동"), tool.Baro_t("달끼리 비교하기 위한 상대 크기입니다. 다른 사람과 비교하는 값이 아닙니다."), month_html, "")

	// --- 종류별 분포 ---
	kind_num := map[string]int{}
	for _, day := range days {
		for kind, count := range day.Detail {
			kind_num[kind] += tool.Str_to_int(count)
		}
	}

	kind_rows := [][]string{}
	for _, kind_row := range tool.Baro_contrib_kind_list() {
		if kind_num[kind_row[0]] <= 0 {
			continue
		}

		kind_rows = append(kind_rows, []string{
			tool.Baro_badge(kind_row[1], "info"),
			tool.Baro_comma(kind_num[kind_row[0]]) + tool.Baro_t("건"),
			tool.Baro_progress(kind_num[kind_row[0]], total_num, ""),
		})
	}

	kind_html := tool.Baro_table([]string{tool.Baro_t("행동"), tool.Baro_t("건수"), tool.Baro_t("비중")}, kind_rows, tool.Baro_t("종류별 기록이 없습니다."))
	if len(kind_rows) == 0 {
		kind_html = tool.Baro_empty(tool.Baro_t("아직 종류별로 나눌 기록이 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("무엇을 했는가"), kind_html)

	// --- 분야별 분포 ---
	field_rows := [][]string{}
	field_total := 0

	field_counts := [][]string{}

	rows := tool.Query_DB(
		db,
		"select "+baro_contrib_field_expr("baro_contrib_event")+" as f, count(*)"+
			" from baro_contrib_event where user = ? and status <> 'rejected' group by f",
		user,
	)
	defer rows.Close()

	for rows.Next() {
		field, count := "", 0
		if err := rows.Scan(&field, &count); err != nil {
			continue
		}

		field_total += count
		field_counts = append(field_counts, []string{field, tool.Baro_itoa(count)})
	}

	baro_contrib_sort_pairs(field_counts)

	for _, pair := range field_counts {
		count := tool.Str_to_int(pair[1])
		field_rows = append(field_rows, []string{
			baro_contrib_esc(tool.Baro_field_name(db, pair[0])),
			tool.Baro_comma(count) + tool.Baro_t("건"),
			tool.Baro_progress(count, field_total, ""),
		})
	}

	field_html := tool.Baro_table([]string{"분야", tool.Baro_t("기여"), tool.Baro_t("비중")}, field_rows, tool.Baro_t("분야별 기록이 없습니다."))
	if len(field_rows) == 0 {
		field_html = tool.Baro_empty(tool.Baro_t("아직 분야가 정해진 기여가 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("어느 분야에 기여했는가"), field_html)

	// --- 대표 성과 ---
	best_rows := [][]string{}

	best := tool.Query_DB(
		db,
		"select "+baro_contrib_event_cols+" from baro_contrib_event where user = ? and status <> 'rejected'"+
			" order by weight + 0 desc, id + 0 desc limit ?, ?",
		user, 0, 10,
	)
	defer best.Close()

	for _, row := range baro_contrib_event_scan(best) {
		thanks := baro_contrib_count(
			db,
			"select count(*) from baro_thanks where title = ? and rev = ? and from_user <> to_user",
			row.Title, row.Rev,
		)

		thanks_html := `<span class="bw_muted">-</span>`
		if thanks > 0 {
			thanks_html = tool.Baro_badge(tool.Baro_itoa(thanks)+tool.Baro_t("명이 감사"), "ok")
		}

		best_rows = append(best_rows, []string{
			tool.Baro_time_cell(row.Date),
			tool.Baro_badge(tool.Baro_contrib_kind_name(row.Kind), "info"),
			tool.Baro_doc_link(row.Title),
			baro_contrib_esc(tool.Baro_field_name(db, row.Field)),
			thanks_html,
		})
	}

	best_html := tool.Baro_table([]string{"시각", tool.Baro_t("행동"), "문서", "분야", tool.Baro_t("감사")}, best_rows, tool.Baro_t("대표 성과가 없습니다."))
	if len(best_rows) == 0 {
		best_html = tool.Baro_empty(tool.Baro_t("아직 대표로 보여 줄 기여가 없습니다."))
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("대표 성과"),
		tool.Baro_t("많이 한 것이 아니라 무게가 큰 기여를 골랐습니다."),
		best_html,
		tool.Baro_button("/baro/contrib/user/"+tool.Url_parser(user), "전체 기여 기록 보기", "")+" "+
			tool.Baro_button("/baro/contrib/impact/"+tool.Url_parser(user), "기여 영향력 보기", ""),
	)

	// --- 최근 활동한 날 ---
	recent_days := []baro_contrib_calendar_day{}
	for _, day := range days {
		if day.Num > 0 {
			recent_days = append(recent_days, day)
		}
	}

	sort.SliceStable(recent_days, func(i int, j int) bool {
		return recent_days[i].Date > recent_days[j].Date
	})

	day_rows := [][]string{}
	for index, day := range recent_days {
		if index >= 20 {
			break
		}

		text := baro_contrib_calendar_detail_text(day.Detail)
		if text == "" {
			text = "-"
		}

		day_rows = append(day_rows, []string{
			baro_contrib_esc(day.Date),
			tool.Baro_comma(day.Num) + tool.Baro_t("건"),
			baro_contrib_esc(text),
		})
	}

	day_html := tool.Baro_table([]string{tool.Baro_t("날짜"), tool.Baro_t("기여"), tool.Baro_t("무엇을 했는가")}, day_rows, tool.Baro_t("활동한 날이 없습니다."))
	if len(day_rows) == 0 {
		day_html = tool.Baro_empty(tool.Baro_t("최근 1년 안에 활동한 날이 없습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("최근 활동한 날 (최대 20일)"), tool.Baro_t("연속 일수 대신 '무엇을 했는지'를 적습니다."), day_html, "")

	return tool.Baro_page(db, config, user+tool.Baro_t(" 님의 활동 캘린더"), body, baro_contrib_menu())
}
