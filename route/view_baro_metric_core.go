package route

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"time"

	"opennamu/route/tool"
)

// ============================================================
//  18장 분석 지표 — 공통 부품
//
//  이 장의 취지는 "MAU 하나만 보고 좋아하면 안 된다" 이다.
//  절대 핵심 지표는 가입자 수가 아니라 "주간 유효 기여자 수" 이며,
//  나머지 지표는 그 수를 해석하기 위한 보조 지표로만 쓴다.
//
//  모든 수치는 실제 baro_ 표를 SQL 로 집계해서 만든다. 가짜 숫자를 넣지 않는다.
//  집계할 자료가 없으면 0 과 함께 "아직 데이터가 없습니다" 를 보여준다.
// ============================================================

// ------------------------------------------------------------
//  기간 선택
// ------------------------------------------------------------

// baro_metric_span : 지표를 볼 기간
type baro_metric_span struct {
	Key    string // "7" / "30" / "12m"
	Name   string // 화면에 보여줄 이름
	Days   int    // 일 단위 기간 (12개월이면 0)
	Months int    // 월 단위 기간 (일 단위면 0)
	From   string // 시작 날짜 (2006-01-02)
	To     string // 끝 날짜 (2006-01-02)
}

// baro_metric_span_list : 고를 수 있는 기간 목록
func baro_metric_span_list() [][]string {
	return [][]string{
		{"7", tool.Baro_t("최근 7일")},
		{"30", tool.Baro_t("최근 30일")},
		{"12m", tool.Baro_t("최근 12개월")},
	}
}

// baro_metric_span_get : 기간 키를 기간 정보로 (모르는 값은 30일로 떨어진다)
func baro_metric_span_get(key string) baro_metric_span {
	now := time.Now()
	today := now.Format("2006-01-02")

	switch key {
	case "7":
		return baro_metric_span{
			Key: "7", Name: tool.Baro_t("최근 7일"), Days: 7,
			From: now.AddDate(0, 0, -6).Format("2006-01-02"), To: today,
		}
	case "12m":
		month_start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

		return baro_metric_span{
			Key: "12m", Name: tool.Baro_t("최근 12개월"), Months: 12,
			From: month_start.AddDate(0, -11, 0).Format("2006-01-02"), To: today,
		}
	}

	return baro_metric_span{
		Key: "30", Name: tool.Baro_t("최근 30일"), Days: 30,
		From: now.AddDate(0, 0, -29).Format("2006-01-02"), To: today,
	}
}

// baro_metric_span_start : 기간 시작 시각 문자열
func baro_metric_span_start(span baro_metric_span) string {
	return span.From + " 00:00:00"
}

// baro_metric_span_end : 기간 끝 시각 문자열
func baro_metric_span_end(span baro_metric_span) string {
	return span.To + " 23:59:59"
}

// baro_metric_span_days : 기간의 대략적인 일 수 (평균 계산용)
func baro_metric_span_days(span baro_metric_span) int {
	if span.Days > 0 {
		return span.Days
	}

	days := tool.Baro_days_between(span.From, span.To) + 1
	if days < 1 {
		days = 1
	}

	return days
}

// baro_metric_periods : 기간을 이루는 구간 목록 (일 단위면 날짜, 월 단위면 월)
func baro_metric_periods(span baro_metric_span) []string {
	out := []string{}
	now := time.Now()

	if span.Months > 0 {
		base := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		for i := span.Months - 1; i >= 0; i-- {
			out = append(out, base.AddDate(0, -i, 0).Format("2006-01"))
		}

		return out
	}

	for i := span.Days - 1; i >= 0; i-- {
		out = append(out, now.AddDate(0, 0, -i).Format("2006-01-02"))
	}

	return out
}

// baro_metric_range_low : baro_metric.period 비교용 아래쪽 경계
//
//	월 단위 기간에서는 period 가 "2026-01" 처럼 저장될 수 있으므로 월까지만 자른다.
func baro_metric_range_low(span baro_metric_span) string {
	if span.Months > 0 {
		return span.From[:7]
	}

	return span.From
}

// baro_metric_range_high : baro_metric.period 비교용 위쪽 경계
func baro_metric_range_high(span baro_metric_span) string {
	return span.To + "~"
}

// baro_metric_period_of : 시각 문자열이 속한 구간 키
func baro_metric_period_of(span baro_metric_span, date string) string {
	if len(date) < 7 {
		return ""
	}

	if span.Months > 0 {
		return date[:7]
	}

	if len(date) < 10 {
		return ""
	}

	return date[:10]
}

// baro_metric_period_label : 그래프 가로축에 쓸 짧은 이름
func baro_metric_period_label(period string) string {
	if len(period) == 7 {
		return period[2:4] + "." + period[5:7]
	}

	if len(period) == 10 {
		return period[5:7] + "." + period[8:10]
	}

	return period
}

// ------------------------------------------------------------
//  작은 SQL 도우미 (전부 ? 자리표시자만 쓴다)
// ------------------------------------------------------------

// baro_metric_int : 값 한 개를 정수로 읽는다
func baro_metric_int(db *sql.DB, query string, values ...any) int {
	data := "0"
	tool.QueryRow_DB(db, query, []any{&data}, values...)

	return tool.Str_to_int(data)
}

// baro_metric_col1 : 한 열짜리 결과를 문자열 목록으로
func baro_metric_col1(db *sql.DB, query string, values ...any) []string {
	out := []string{}

	rows := tool.Query_DB(db, query, values...)
	defer rows.Close()

	for rows.Next() {
		data := ""
		if err := rows.Scan(&data); err != nil {
			continue
		}

		out = append(out, data)
	}

	return out
}

// baro_metric_col2 : 두 열짜리 결과
func baro_metric_col2(db *sql.DB, query string, values ...any) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(db, query, values...)
	defer rows.Close()

	for rows.Next() {
		a, b := "", ""
		if err := rows.Scan(&a, &b); err != nil {
			continue
		}

		out = append(out, []string{a, b})
	}

	return out
}

// baro_metric_col3 : 세 열짜리 결과
func baro_metric_col3(db *sql.DB, query string, values ...any) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(db, query, values...)
	defer rows.Close()

	for rows.Next() {
		a, b, c := "", "", ""
		if err := rows.Scan(&a, &b, &c); err != nil {
			continue
		}

		out = append(out, []string{a, b, c})
	}

	return out
}

// baro_metric_col4 : 네 열짜리 결과
func baro_metric_col4(db *sql.DB, query string, values ...any) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(db, query, values...)
	defer rows.Close()

	for rows.Next() {
		a, b, c, d := "", "", "", ""
		if err := rows.Scan(&a, &b, &c, &d); err != nil {
			continue
		}

		out = append(out, []string{a, b, c, d})
	}

	return out
}

// baro_metric_col5 : 다섯 열짜리 결과
func baro_metric_col5(db *sql.DB, query string, values ...any) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(db, query, values...)
	defer rows.Close()

	for rows.Next() {
		a, b, c, d, e := "", "", "", "", ""
		if err := rows.Scan(&a, &b, &c, &d, &e); err != nil {
			continue
		}

		out = append(out, []string{a, b, c, d, e})
	}

	return out
}

// ------------------------------------------------------------
//  계산 도우미
// ------------------------------------------------------------

// baro_metric_median : 중앙값 (자료가 없으면 0)
func baro_metric_median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64{}, values...)
	sort.Float64s(sorted)

	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}

	return (sorted[mid-1] + sorted[mid]) / 2
}

// baro_metric_mean : 평균 (자료가 없으면 0)
func baro_metric_mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, value := range values {
		sum += value
	}

	return sum / float64(len(values))
}

// baro_metric_round1 : 소수 한 자리 문자열
func baro_metric_round1(value float64) string {
	return strconv.FormatFloat(float64(int(value*10+0.5))/10, 'f', 1, 64)
}

// baro_metric_hours_text : 시간 수를 사람이 읽는 문자열로
func baro_metric_hours_text(hours float64) string {
	if hours <= 0 {
		return tool.Baro_t("0시간")
	}

	if hours < 1 {
		return baro_metric_round1(hours*60) + tool.Baro_t("분")
	}

	if hours < 48 {
		return baro_metric_round1(hours) + tool.Baro_t("시간")
	}

	return baro_metric_round1(hours/24) + tool.Baro_t("일")
}

// baro_metric_days_text : 일 수를 사람이 읽는 문자열로
func baro_metric_days_text(days float64) string {
	if days <= 0 {
		return tool.Baro_t("0일")
	}

	if days < 1 {
		return baro_metric_hours_text(days * 24)
	}

	return baro_metric_round1(days) + tool.Baro_t("일")
}

// baro_metric_gap_hours : 두 시각 사이의 시간 (b - a, 음수면 0)
func baro_metric_gap_hours(a string, b string) float64 {
	ta := tool.Baro_parse_time(a)
	tb := tool.Baro_parse_time(b)
	if ta.IsZero() || tb.IsZero() {
		return -1
	}

	gap := tb.Sub(ta).Hours()
	if gap < 0 {
		return 0
	}

	return gap
}

// baro_metric_uniq : 문자열 목록 중복 제거
func baro_metric_uniq(values []string) []string {
	seen := map[string]bool{}
	out := []string{}

	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}

		seen[value] = true
		out = append(out, value)
	}

	return out
}

// baro_metric_bad_status : 유효 기여로 세지 않는 상태 (되돌려졌거나 반려된 것)
//
//	SQL 안에 그대로 넣는 상수 문자열이며 사용자 입력이 아니다.
const baro_metric_bad_status = " and status not in ('rejected', 'reverted', 'invalid', 'denied', 'spam') "

// ------------------------------------------------------------
//  지표 카드 (모든 지표에는 반드시 한 줄 설명을 단다)
// ------------------------------------------------------------

// baro_metric_item : 지표 한 개
type baro_metric_item struct {
	Label string // 지표 이름
	Value string // 값
	Sub   string // 값 아래 보조 설명 (분모 등)
	Desc  string // 이 지표가 무엇을 뜻하는지 (필수)
	Empty bool   // 계산할 자료가 아직 없는지
}

// baro_metric_cards : 지표 카드 묶음
func baro_metric_cards(items []baro_metric_item, cols int) string {
	parts := []string{}

	for _, item := range items {
		html := tool.Baro_stat(item.Label, item.Value, item.Sub)
		if item.Empty {
			html += tool.Baro_t(`<div class="bw_metric_none">아직 데이터가 없습니다.</div>`)
		}
		if item.Desc != "" {
			html += tool.Baro_note(item.Desc)
		}

		parts = append(parts, html)
	}

	return tool.Baro_grid(parts, cols)
}

// baro_metric_num : 정수 지표 한 개 만들기
func baro_metric_num(label string, value int, sub string, desc string) baro_metric_item {
	return baro_metric_item{
		Label: label,
		Value: tool.Baro_comma(value),
		Sub:   sub,
		Desc:  desc,
		Empty: value == 0,
	}
}

// baro_metric_rate : 비율 지표 한 개 만들기 (분모가 0 이면 자료 없음)
func baro_metric_rate(label string, done int, total int, unit string, desc string) baro_metric_item {
	if total <= 0 {
		return baro_metric_item{
			Label: label,
			Value: "0.0%",
			Sub:   tool.Baro_t("분모 0") + tool.Baro_t(unit),
			Desc:  desc,
			Empty: true,
		}
	}

	return baro_metric_item{
		Label: label,
		Value: tool.Baro_pct_text(done, total),
		Sub:   tool.Baro_comma(done) + " / " + tool.Baro_comma(total) + tool.Baro_t(unit),
		Desc:  desc,
	}
}

// baro_metric_text_item : 값이 문자열인 지표 한 개
func baro_metric_text_item(label string, value string, sub string, desc string, empty bool) baro_metric_item {
	return baro_metric_item{Label: label, Value: value, Sub: sub, Desc: desc, Empty: empty}
}

// ------------------------------------------------------------
//  인라인 SVG 그래프 (외부 라이브러리를 쓰지 않는다)
// ------------------------------------------------------------

// baro_metric_nice_max : 눈금이 예뻐지도록 최댓값을 올림
func baro_metric_nice_max(value float64) float64 {
	if value <= 0 {
		return 1
	}

	step := 1.0
	for value/step >= 10 {
		step *= 10
	}

	out := float64(int(value/step)+1) * step
	if out <= 0 {
		out = 1
	}

	return out
}

// baro_metric_fmt : 그래프 눈금 숫자 표시
func baro_metric_fmt(value float64, unit string) string {
	if unit == "%" {
		return baro_metric_round1(value) + "%"
	}

	if value == float64(int(value)) {
		return tool.Baro_comma(int(value)) + tool.Baro_t(unit)
	}

	return baro_metric_round1(value) + tool.Baro_t(unit)
}

// baro_metric_chart : 인라인 SVG 그래프 (kind : line / bar)
//
//	labels 와 values 의 길이는 같아야 한다. 값이 전부 0 이면 안내 문구를 돌려준다.
func baro_metric_chart(kind string, labels []string, values []float64, unit string, caption string) string {
	if len(values) == 0 || len(values) != len(labels) {
		return tool.Baro_empty(tool.Baro_t("그래프를 그릴 자료가 없습니다."))
	}

	max_value := 0.0
	has_value := false
	for _, value := range values {
		if value > max_value {
			max_value = value
		}
		if value > 0 {
			has_value = true
		}
	}

	if !has_value {
		return tool.Baro_empty(tool.Baro_t("이 기간에는 모든 값이 0 입니다. 아직 데이터가 없습니다."))
	}

	top_value := baro_metric_nice_max(max_value)
	if unit == "%" && top_value > 100 {
		top_value = 100
	}

	const width = 720.0
	const height = 210.0
	const pad_left = 52.0
	const pad_right = 12.0
	const pad_top = 14.0
	const pad_bottom = 30.0

	plot_w := width - pad_left - pad_right
	plot_h := height - pad_top - pad_bottom

	pos_x := func(index int) float64 {
		if len(values) == 1 {
			return pad_left + plot_w/2
		}

		if kind == "bar" {
			slot := plot_w / float64(len(values))

			return pad_left + slot*float64(index) + slot/2
		}

		return pad_left + plot_w*float64(index)/float64(len(values)-1)
	}

	pos_y := func(value float64) float64 {
		if top_value <= 0 {
			return pad_top + plot_h
		}

		ratio := value / top_value
		if ratio > 1 {
			ratio = 1
		}
		if ratio < 0 {
			ratio = 0
		}

		return pad_top + plot_h - plot_h*ratio
	}

	num := func(value float64) string {
		return strconv.FormatFloat(value, 'f', 1, 64)
	}

	svg := `<svg class="bw_metric_svg" viewBox="0 0 720 210" role="img" preserveAspectRatio="none" xmlns="http://www.w3.org/2000/svg">`
	svg += `<title>` + tool.HTML_escape(caption) + `</title>`

	// 가로 눈금선 다섯 줄
	for i := 0; i <= 4; i++ {
		value := top_value * float64(4-i) / 4
		y := pos_y(value)

		svg += `<line x1="` + num(pad_left) + `" y1="` + num(y) + `" x2="` + num(width-pad_right) + `" y2="` + num(y) + `" stroke="currentColor" stroke-opacity="0.14" stroke-width="1"/>`
		svg += `<text x="` + num(pad_left-6) + `" y="` + num(y+3.5) + `" text-anchor="end" font-size="10" fill="currentColor" fill-opacity="0.55">` + tool.HTML_escape(baro_metric_fmt(value, unit)) + `</text>`
	}

	if kind == "bar" {
		slot := plot_w / float64(len(values))
		bar_w := slot * 0.62
		if bar_w < 2 {
			bar_w = 2
		}

		for i, value := range values {
			y := pos_y(value)
			h := pad_top + plot_h - y
			if h < 1 && value > 0 {
				h = 1
				y = pad_top + plot_h - 1
			}

			svg += `<rect x="` + num(pos_x(i)-bar_w/2) + `" y="` + num(y) + `" width="` + num(bar_w) + `" height="` + num(h) + `" rx="2" fill="#010080" fill-opacity="0.85">`
			svg += `<title>` + tool.HTML_escape(labels[i]+" : "+baro_metric_fmt(value, unit)) + `</title></rect>`
		}
	} else {
		points := []string{}
		for i, value := range values {
			points = append(points, num(pos_x(i))+","+num(pos_y(value)))
		}

		area := "M " + num(pos_x(0)) + "," + num(pad_top+plot_h) + " L " + strings.Join(points, " L ") +
			" L " + num(pos_x(len(values)-1)) + "," + num(pad_top+plot_h) + " Z"

		svg += `<path d="` + area + `" fill="#010080" fill-opacity="0.12"/>`
		svg += `<polyline points="` + strings.Join(points, " ") + `" fill="none" stroke="#010080" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>`

		step := 1
		if len(values) > 60 {
			step = len(values) / 60
		}

		for i, value := range values {
			if i%step != 0 && i != len(values)-1 {
				continue
			}

			svg += `<circle cx="` + num(pos_x(i)) + `" cy="` + num(pos_y(value)) + `" r="2.6" fill="#010080">`
			svg += `<title>` + tool.HTML_escape(labels[i]+" : "+baro_metric_fmt(value, unit)) + `</title></circle>`
		}
	}

	// 세로축 밑선
	svg += `<line x1="` + num(pad_left) + `" y1="` + num(pad_top+plot_h) + `" x2="` + num(width-pad_right) + `" y2="` + num(pad_top+plot_h) + `" stroke="currentColor" stroke-opacity="0.35" stroke-width="1"/>`

	// 가로축 이름 (너무 빽빽하지 않게 최대 8개)
	label_step := 1
	if len(labels) > 8 {
		label_step = (len(labels) + 7) / 8
	}

	for i, label := range labels {
		if i%label_step != 0 && i != len(labels)-1 {
			continue
		}

		svg += `<text x="` + num(pos_x(i)) + `" y="` + num(height-10) + `" text-anchor="middle" font-size="10" fill="currentColor" fill-opacity="0.55">` + tool.HTML_escape(baro_metric_period_label(label)) + `</text>`
	}

	svg += `</svg>`

	html := `<figure class="bw_metric_fig">` + svg
	if caption != "" {
		html += `<figcaption class="bw_note">` + tool.HTML_escape(caption) + `</figcaption>`
	}

	return html + `</figure>`
}

// baro_metric_chart_int : 정수 값 그래프
func baro_metric_chart_int(kind string, labels []string, values []int, unit string, caption string) string {
	floats := []float64{}
	for _, value := range values {
		floats = append(floats, float64(value))
	}

	return baro_metric_chart(kind, labels, floats, unit, caption)
}

// ------------------------------------------------------------
//  화면 껍데기 (탭 · 기간 선택 · 스타일)
// ------------------------------------------------------------

// baro_metric_tab_list : 지표 대시보드 탭 목록
func baro_metric_tab_list() [][]string {
	return [][]string{
		{"/baro/metrics", tool.Baro_t("요약")},
		{"/baro/metrics/acquisition", tool.Baro_t("유입")},
		{"/baro/metrics/activation", tool.Baro_t("활성화")},
		{"/baro/metrics/retention", "유지"},
		{"/baro/metrics/quality", tool.Baro_t("품질")},
		{"/baro/metrics/trust", tool.Baro_t("운영 신뢰")},
		{"/baro/metrics/history", tool.Baro_t("저장된 추이")},
	}
}

// baro_metric_tabs : 탭 줄 (기간 선택을 유지한 채 이동한다)
func baro_metric_tabs(active string, span baro_metric_span) string {
	items := [][]string{}
	for _, tab := range baro_metric_tab_list() {
		items = append(items, []string{tab[0] + "?span=" + span.Key, tab[1]})
	}

	return tool.Baro_tabs(items, active+"?span="+span.Key)
}

// baro_metric_span_picker : 기간 고르기 줄
func baro_metric_span_picker(path string, span baro_metric_span) string {
	html := tool.Baro_t(`<div class="bw_metric_span"><span class="bw_metric_span_l">기간</span>`)

	for _, row := range baro_metric_span_list() {
		kind := ""
		if row[0] == span.Key {
			kind = "primary"
		}

		html += tool.Baro_button(path+"?span="+row[0], row[1], kind) + " "
	}

	html += `</div>`
	html += tool.Baro_note(tool.Baro_t("보고 있는 기간 : ") + span.From + " ~ " + span.To)

	return html
}

// baro_metric_css : 이 장에서만 쓰는 덧붙임 스타일
func baro_metric_css() string {
	return `<style id="bw_metric_css">
.bw_metric_key{border:2px solid rgba(0,166,156,.5);border-radius:12px;padding:1.4rem 1.5rem;margin:0 0 1.2rem 0;background:linear-gradient(135deg,rgba(0,166,156,.10),rgba(40,180,114,.04));}
.bw_metric_key_l{font-size:.95rem;font-weight:700;letter-spacing:.02em;color:#1a7f4f;margin:0 0 .2rem 0;}
.bw_metric_big{font-size:4rem;font-weight:900;line-height:1.05;letter-spacing:-.02em;margin:.1rem 0;}
.bw_metric_unit{font-size:1.3rem;font-weight:700;margin-left:.25rem;}
.bw_metric_delta{font-size:.95rem;font-weight:700;margin-left:.6rem;}
.bw_metric_up{color:#1a7f4f;}
.bw_metric_down{color:#b12b2b;}
.bw_metric_flat{color:#6b7280;}
.bw_metric_none{font-size:.8rem;font-weight:700;color:#8a6100;margin-top:.3rem;text-align:center;}
.bw_metric_svg{width:100%;height:auto;max-height:230px;display:block;}
.bw_metric_fig{margin:.7rem 0;padding:0;}
.bw_metric_span{display:flex;flex-wrap:wrap;align-items:center;gap:.35rem;margin:.6rem 0 .2rem 0;}
.bw_metric_span_l{font-size:.86rem;font-weight:700;color:#6b7280;margin-right:.3rem;}
.bw_metric_def{font-size:.86rem;color:#6b7280;margin:.4rem 0 0 0;padding-left:.8rem;border-left:3px solid rgba(0,166,156,.4);}
@media (max-width:640px){.bw_metric_big{font-size:2.6rem;}}
</style>`
}

// baro_metric_frame : 지표 화면 공통 틀
func baro_metric_frame(db *sql.DB, config tool.Config, title string, path string, span baro_metric_span, body string) string {
	full := baro_metric_css()
	full += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/metrics", tool.Baro_t("분석 지표")}, {"", title}})
	full += baro_metric_tabs(path, span)
	full += baro_metric_span_picker(path, span)
	full += body

	return tool.Baro_page(db, config, tool.Baro_t("분석 지표 · ")+title, full, [][]any{{"baro", "바로위키"}})
}

// baro_metric_guard : 지표 대시보드 접근 권한 확인
//
//	지표는 운영 판단에 쓰이므로 관리자만 볼 수 있다.
//	권한이 없어도 500 이 아니라 안내 화면(200)을 돌려준다.
func baro_metric_guard(db *sql.DB, config tool.Config) (string, bool) {
	if wall, need := tool.Baro_login_wall(db, config, "/baro/metrics"); need {
		return wall, true
	}

	if !tool.Baro_is_admin(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("분석 지표 대시보드는 위키 관리자만 볼 수 있습니다. 지표는 운영 판단에 쓰이므로 개인 활동이 드러나지 않도록 제한합니다.")), true
	}

	return "", false
}

// ------------------------------------------------------------
//  ★ 절대 핵심 지표 : 주간 유효 기여자 수
// ------------------------------------------------------------

// baro_metric_effective_users : 주어진 기간 안에 유효 기여를 한 서로 다른 사용자 수
//
//	유효 기여 = baro_contrib_event 중 되돌려지거나 반려되지 않은 기록.
//	가입만 한 사람, 읽기만 한 사람은 세지 않는다.
func baro_metric_effective_users(db *sql.DB, from string, to string) int {
	return baro_metric_int(
		db,
		"select count(distinct user) from baro_contrib_event where date >= ? and date <= ? and user != ''"+baro_metric_bad_status,
		from, to,
	)
}

// baro_metric_effective_events : 주어진 기간 안의 유효 기여 건수
func baro_metric_effective_events(db *sql.DB, from string, to string) int {
	return baro_metric_int(
		db,
		"select count(*) from baro_contrib_event where date >= ? and date <= ?"+baro_metric_bad_status,
		from, to,
	)
}

// baro_metric_week_range : n 주 전의 (시작, 끝, 이름)
func baro_metric_week_range(back int) (string, string, string) {
	now := time.Now()
	end := now.AddDate(0, 0, -7*back)
	start := end.AddDate(0, 0, -6)

	return start.Format("2006-01-02") + " 00:00:00",
		end.Format("2006-01-02") + " 23:59:59",
		start.Format("2006-01-02") + "~" + end.Format("2006-01-02")
}

// baro_metric_weekly_series : 최근 n 주의 주간 유효 기여자 수
func baro_metric_weekly_series(db *sql.DB, weeks int) ([]string, []int) {
	labels := []string{}
	values := []int{}

	for i := weeks - 1; i >= 0; i-- {
		from, to, name := baro_metric_week_range(i)
		labels = append(labels, name)
		values = append(values, baro_metric_effective_users(db, from, to))
	}

	return labels, values
}

// baro_metric_delta_html : 지난 기간 대비 변화 표시
func baro_metric_delta_html(now int, before int) string {
	if before == 0 && now == 0 {
		return tool.Baro_t(`<span class="bw_metric_delta bw_metric_flat">지난주에도 0명</span>`)
	}

	if before == 0 {
		return tool.Baro_t(`<span class="bw_metric_delta bw_metric_up">지난주 0명 → +`) + tool.Baro_comma(now) + tool.Baro_t(`명</span>`)
	}

	diff := now - before
	if diff == 0 {
		return tool.Baro_t(`<span class="bw_metric_delta bw_metric_flat">지난주와 같음</span>`)
	}

	class := "bw_metric_up"
	sign := "+"
	if diff < 0 {
		class = "bw_metric_down"
		sign = "-"
		diff = -diff
	}

	return `<span class="bw_metric_delta ` + class + `">지난주 대비 ` + sign + tool.Baro_comma(diff) +
		tool.Baro_t(`명 (`) + sign + tool.Baro_pct_text(diff, before) + `)</span>`
}

// baro_metric_key_block : ★ 핵심 지표를 가장 크게 보여주는 상자
func baro_metric_key_block(db *sql.DB) string {
	this_from, this_to, this_name := baro_metric_week_range(0)
	last_from, last_to, _ := baro_metric_week_range(1)

	now_value := baro_metric_effective_users(db, this_from, this_to)
	last_value := baro_metric_effective_users(db, last_from, last_to)

	html := `<div class="bw_metric_key">`
	html += tool.Baro_t(`<p class="bw_metric_key_l">★ 절대 핵심 지표 — 주간 유효 기여자 수</p>`)
	html += `<div><span class="bw_metric_big">` + tool.Baro_comma(now_value) + tool.Baro_t(`</span><span class="bw_metric_unit">명</span>`)
	html += baro_metric_delta_html(now_value, last_value) + `</div>`
	html += tool.Baro_t(`<p class="bw_metric_def">최근 7일(`) + tool.HTML_escape(this_name) + tool.Baro_t(`) 동안 되돌려지거나 반려되지 않은 기여를 한 번 이상 남긴 서로 다른 사람 수입니다. `) +
		tool.Baro_t(`가입자 수 · 방문자 수 · MAU 는 이 수를 설명하기 위한 보조 지표일 뿐입니다.</p>`)

	if now_value == 0 {
		html += tool.Baro_alert("warn", tool.Baro_t("아직 데이터가 없습니다. baro_contrib_event 에 이번 주 기여 기록이 하나도 없습니다. 실제 기여가 기록되기 시작하면 이 수가 올라갑니다."))
	}

	html += `</div>`

	return html
}

// ------------------------------------------------------------
//  구간별 묶기
// ------------------------------------------------------------

// baro_metric_bucket : 시각 문자열 목록을 기간 구간별 개수로 센다
func baro_metric_bucket(span baro_metric_span, dates []string) []int {
	periods := baro_metric_periods(span)

	count := map[string]int{}
	for _, date := range dates {
		slot := baro_metric_period_of(span, date)
		if slot == "" {
			continue
		}

		count[slot]++
	}

	out := []int{}
	for _, period := range periods {
		out = append(out, count[period])
	}

	return out
}

// baro_metric_bucket_uniq : {시각, 사람} 쌍을 구간별 서로 다른 사람 수로 센다
func baro_metric_bucket_uniq(span baro_metric_span, rows [][]string) []int {
	periods := baro_metric_periods(span)

	seen := map[string]map[string]bool{}
	for _, row := range rows {
		if len(row) < 2 || row[1] == "" {
			continue
		}

		slot := baro_metric_period_of(span, row[0])
		if slot == "" {
			continue
		}

		if seen[slot] == nil {
			seen[slot] = map[string]bool{}
		}

		seen[slot][row[1]] = true
	}

	out := []int{}
	for _, period := range periods {
		out = append(out, len(seen[period]))
	}

	return out
}

// baro_metric_field_name : 분야 키를 이름으로 (빈 값은 "분야 미지정")
func baro_metric_field_name(db *sql.DB, key string) string {
	if key == "" {
		return tool.Baro_t("분야 미지정")
	}

	name := tool.Baro_field_name(db, key)
	if name == "" {
		return key
	}

	return name
}

// baro_metric_rank_table : {이름, 값} 목록을 값 큰 순서로 정렬한 표
func baro_metric_rank_table(head_name string, head_value string, rows [][]string, empty string) string {
	sort.SliceStable(rows, func(i int, j int) bool {
		return tool.Str_to_int(rows[i][1]) > tool.Str_to_int(rows[j][1])
	})

	total := 0
	for _, row := range rows {
		total += tool.Str_to_int(row[1])
	}

	body := [][]string{}
	for _, row := range rows {
		value := tool.Str_to_int(row[1])
		body = append(body, []string{
			tool.HTML_escape(row[0]),
			tool.Baro_comma(value),
			tool.Baro_pct_text(value, total),
			tool.Baro_progress(value, total, ""),
		})
	}

	return tool.Baro_table([]string{head_name, head_value, tool.Baro_t("비중"), tool.Baro_t("분포")}, body, empty)
}
