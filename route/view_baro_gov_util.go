package route

import (
	"database/sql"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  11장 운영 투명성 · 거버넌스 — 공통 도구
//
//  이 파일의 비공개 식별자는 전부 baro_gov_ 로 시작한다.
// ============================================================

// baro_gov_email_re : 화면에 그대로 드러나면 안 되는 이메일 모양 문자열
var baro_gov_email_re = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)

// baro_gov_key_re : 정책 키 형식 (영소문자·숫자·밑줄)
var baro_gov_key_re = regexp.MustCompile(`^[a-z0-9_]{2,40}$`)

// baro_gov_mask_email : 이메일 모양 문자열을 가린다
//
//	저장 단계에서 이미 tool.Baro_mask_ip 로 걸러지지만,
//	사유·상세 본문에 사람이 손으로 적어 넣은 이메일이 남을 수 있어 화면에서 한 번 더 가린다.
func baro_gov_mask_email(data string) string {
	if data == "" {
		return ""
	}

	return baro_gov_email_re.ReplaceAllStringFunc(data, func(one string) string {
		at := strings.Index(one, "@")
		if at <= 0 {
			return tool.Baro_t("(이메일 가림)")
		}

		head := one[:at]
		tail := one[at+1:]

		keep := 1
		if len(head) < 2 {
			keep = len(head)
		}

		dot := strings.LastIndex(tail, ".")
		domain := "***"
		if dot > 0 {
			domain = "***" + tail[dot:]
		}

		return head[:keep] + "***@" + domain
	})
}

// baro_gov_safe : 사용자 입력을 화면에 넣기 전 이메일 가림 + HTML 이스케이프
func baro_gov_safe(data string) string {
	return tool.HTML_escape(baro_gov_mask_email(data))
}

// baro_gov_q : 질의 문자열에서 값 하나 읽기 (앞뒤 공백 제거)
func baro_gov_q(query url.Values, name string) string {
	if query == nil {
		return ""
	}

	return strings.TrimSpace(query.Get(name))
}

// baro_gov_page_num : 쪽 번호 (1 미만이면 1)
func baro_gov_page_num(data string) int {
	num := tool.Str_to_int(data)
	if num < 1 {
		num = 1
	}
	if num > 100000 {
		num = 100000
	}

	return num
}

// baro_gov_url : 질의 문자열을 붙인 주소 만들기 (값이 빈 항목은 생략)
func baro_gov_url(base string, pairs [][]string) string {
	parts := []string{}
	for _, pair := range pairs {
		if len(pair) < 2 || pair[1] == "" {
			continue
		}

		if pair[1] == "{}" {
			parts = append(parts, url.QueryEscape(pair[0])+"={}")
			continue
		}

		parts = append(parts, url.QueryEscape(pair[0])+"="+url.QueryEscape(pair[1]))
	}

	if len(parts) == 0 {
		return base
	}

	return base + "?" + strings.Join(parts, "&")
}

// baro_gov_get_form : 찾기·거르기용 GET 폼 (쓰기가 아니므로 POST 를 쓰지 않는다)
func baro_gov_get_form(action string, body string) string {
	return `<form class="bw_form" method="get" action="` + tool.HTML_escape(action) + `">` + body + `</form>`
}

// baro_gov_log_kind_list : 공개 운영 로그에 실리는 항목 종류
//
//	11-1 에서 공개하기로 한 것 : 차단, 보호, 문서 격리, 삭제, 복원,
//	관리자 권한 부여·회수, 정책 변경, 대량 작업.
func baro_gov_log_kind_list() [][]string {
	return [][]string{
		{"block", tool.Baro_t("차단"), "danger"},
		{"unblock", tool.Baro_t("차단 해제"), "ok"},
		{"sanction", "제재", "danger"},
		{"protect", tool.Baro_t("문서 보호"), "warn"},
		{"unprotect", tool.Baro_t("보호 해제"), "ok"},
		{"quarantine", tool.Baro_t("문서 격리"), "warn"},
		{"release", tool.Baro_t("격리 해제"), "ok"},
		{"delete", "삭제", "danger"},
		{"restore", tool.Baro_t("복원"), "ok"},
		{"role_grant", tool.Baro_t("관리자 권한 부여"), "info"},
		{"role_revoke", tool.Baro_t("관리자 권한 회수"), "info"},
		{"policy", tool.Baro_t("정책 변경"), "info"},
		{"policy_new", tool.Baro_t("정책 제정"), "info"},
		{"mass", tool.Baro_t("대량 작업"), "danger"},
		{"case", tool.Baro_t("사건 처리"), "muted"},
		{"appeal", tool.Baro_t("이의제기 처리"), "muted"},
		{"coi", tool.Baro_t("이해충돌 회피"), "muted"},
		{"transparency", tool.Baro_t("투명성 보고서 발행"), "muted"},

		// 어워드(12장)도 공개 로그에 기록을 남긴다.
		// baro_public_log 에 실제로 쌓이는 award_ 로 시작하는 종류를 빠짐없이 적는다.
		{"award_season", tool.Baro_t("어워드 시즌 개설"), "info"},
		{"award_stage", tool.Baro_t("어워드 단계 변경"), "info"},
		{"award_auto", tool.Baro_t("어워드 자동 자격 계산"), "info"},
		{"award_nominate", tool.Baro_t("어워드 후보 추천"), "info"},
		{"award_judge", tool.Baro_t("어워드 심사 평가"), "info"},
		{"award_vote", tool.Baro_t("어워드 커뮤니티 투표"), "info"},
		{"award_close", tool.Baro_t("어워드 선정 확정"), "ok"},
		{"award_mvp", tool.Baro_t("월간 MVP 선정"), "ok"},
		{"award_setting", tool.Baro_t("어워드 설정 변경"), "muted"},
	}
}

// baro_gov_log_kind_name : 로그 종류 표시 이름
func baro_gov_log_kind_name(kind string) string {
	for _, row := range baro_gov_log_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	if kind == "" {
		return "기타"
	}

	return kind
}

// baro_gov_log_kind_badge : 로그 종류 배지
func baro_gov_log_kind_badge(kind string) string {
	for _, row := range baro_gov_log_kind_list() {
		if row[0] == kind {
			return tool.Baro_badge(row[1], row[2])
		}
	}

	return tool.Baro_badge(baro_gov_log_kind_name(kind), "muted")
}

// baro_gov_log_kind_options : 필터용 선택 상자 항목 (알려진 종류 + DB 에 실제로 있는 종류)
func baro_gov_log_kind_options(db *sql.DB) [][]string {
	options := [][]string{{"", tool.Baro_t("전체 종류")}}
	seen := map[string]bool{"": true}

	for _, row := range baro_gov_log_kind_list() {
		options = append(options, []string{row[0], row[1]})
		seen[row[0]] = true
	}

	extra := []string{}
	rows := tool.Query_DB(db, "select distinct kind from baro_public_log")
	defer rows.Close()

	for rows.Next() {
		kind := ""
		if err := rows.Scan(&kind); err != nil {
			continue
		}

		if seen[kind] {
			continue
		}

		seen[kind] = true
		extra = append(extra, kind)
	}

	sort.Strings(extra)
	for _, kind := range extra {
		options = append(options, []string{kind, kind})
	}

	return options
}

// baro_gov_tabs : 11장 화면 사이의 이동 줄
func baro_gov_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/log", tool.Baro_t("공개 운영 로그")},
		{"/baro/policy", tool.Baro_t("정책 문서")},
		{"/baro/role", tool.Baro_t("운영자 역할")},
		{"/baro/role/coi", tool.Baro_t("이해충돌 회피")},
		{"/baro/role/activity", tool.Baro_t("관리자 활동")},
		{"/baro/transparency", tool.Baro_t("투명성 보고서")},
	}, active)
}

// baro_gov_menu : 화면 아래쪽 메뉴
func baro_gov_menu() [][]any {
	return [][]any{
		{"baro", "바로위키"},
		{"baro/log", tool.Baro_t("운영 로그")},
		{"baro/policy", "정책"},
		{"baro/role", "역할"},
		{"baro/transparency", tool.Baro_t("투명성")},
	}
}

// baro_gov_actor_html : 담당자 표시 (IP 는 이미 가려져 저장되지만 한 번 더 확인)
func baro_gov_actor_html(db *sql.DB, actor string, my_ip string) string {
	if actor == "" {
		return `<span class="bw_muted">-</span>`
	}

	if actor == "system" {
		return tool.Baro_badge(tool.Baro_t("시스템"), "muted")
	}

	if tool.Baro_is_user(actor) {
		return tool.Baro_user_link(db, actor, my_ip)
	}

	return `<span class="bw_muted">` + tool.HTML_escape(tool.Baro_mask_ip(actor)) + `</span>`
}

// baro_gov_is_staff : 운영 역할을 하나라도 가진 사람인가 (또는 관리자·소유자)
func baro_gov_is_staff(db *sql.DB, ip string) bool {
	if !tool.Baro_is_user(ip) {
		return false
	}

	if tool.Baro_is_owner(db, ip) || tool.Baro_is_admin(db, ip) {
		return true
	}

	return len(tool.Baro_user_roles(db, ip)) > 0
}

// baro_gov_count : 단일 집계 값 읽기
func baro_gov_count(db *sql.DB, query string, values ...any) int {
	num := 0
	tool.QueryRow_DB(db, query, []any{&num}, values...)

	return num
}

// baro_gov_period_range : 기간 문자열을 시작·끝 날짜로 바꾼다
//
//	"2026-08" → 2026-08-01 ~ 2026-08-31
//	"2026-Q3" → 2026-07-01 ~ 2026-09-30
//	"2026"    → 2026-01-01 ~ 2026-12-31
func baro_gov_period_range(period string) (string, string) {
	period = strings.TrimSpace(period)

	if len(period) == 7 && strings.Contains(period, "-Q") {
		year := tool.Str_to_int(period[:4])
		quarter := tool.Str_to_int(period[6:])
		if quarter < 1 || quarter > 4 {
			quarter = 1
		}

		start_month := (quarter-1)*3 + 1
		end_month := start_month + 2

		return strconv.Itoa(year) + "-" + baro_gov_two(start_month) + "-01", baro_gov_month_end(year, end_month)
	}

	if len(period) == 7 {
		year := tool.Str_to_int(period[:4])
		month := tool.Str_to_int(period[5:])
		if month < 1 || month > 12 {
			month = 1
		}

		return strconv.Itoa(year) + "-" + baro_gov_two(month) + "-01", baro_gov_month_end(year, month)
	}

	if len(period) == 4 {
		return period + "-01-01", period + "-12-31"
	}

	// 알 수 없는 형식이면 아주 넓은 범위를 준다 (집계가 0 이 되는 것을 막는다)
	return "0000-01-01", "9999-12-31"
}

// baro_gov_two : 두 자리 숫자 문자열
func baro_gov_two(num int) string {
	if num < 10 {
		return "0" + strconv.Itoa(num)
	}

	return strconv.Itoa(num)
}

// baro_gov_month_end : 그 달의 마지막 날 (YYYY-MM-DD)
func baro_gov_month_end(year int, month int) string {
	if month < 1 || month > 12 {
		month = 12
	}

	days := []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}[month-1]
	if month == 2 && ((year%4 == 0 && year%100 != 0) || year%400 == 0) {
		days = 29
	}

	return strconv.Itoa(year) + "-" + baro_gov_two(month) + "-" + baro_gov_two(days)
}

// baro_gov_period_name : 기간 표시 이름
func baro_gov_period_name(period string) string {
	if len(period) == 7 && strings.Contains(period, "-Q") {
		return period[:4] + "년 " + period[6:] + tool.Baro_t("분기")
	}

	if len(period) == 7 {
		return period[:4] + "년 " + strings.TrimPrefix(period[5:], "0") + "월"
	}

	if len(period) == 4 {
		return period + tool.Baro_t("년")
	}

	if period == "" {
		return tool.Baro_t("기간 없음")
	}

	return period
}

// baro_gov_date_between : 기간 조건 (끝 날짜는 그 날 23:59:59 까지 포함)
func baro_gov_date_between(start string, end string) (string, string) {
	return start + " 00:00:00", end + " 23:59:59"
}

// baro_gov_hours_between : 두 시각 사이의 시간(시) — 계산 불가면 -1
func baro_gov_hours_between(start string, end string) int {
	if start == "" || end == "" {
		return -1
	}

	a := tool.Baro_parse_time(start)
	b := tool.Baro_parse_time(end)
	if a.IsZero() || b.IsZero() {
		return -1
	}

	diff := int(b.Sub(a).Hours())
	if diff < 0 {
		return -1
	}

	return diff
}

// baro_gov_hour_text : 시간(시) 을 사람이 읽는 문자열로
func baro_gov_hour_text(hours int) string {
	if hours < 0 {
		return "-"
	}

	if hours < 24 {
		return strconv.Itoa(hours) + tool.Baro_t("시간")
	}

	days := hours / 24
	rest := hours % 24
	if rest == 0 {
		return strconv.Itoa(days) + tool.Baro_t("일")
	}

	return strconv.Itoa(days) + tool.Baro_t("일 ") + strconv.Itoa(rest) + tool.Baro_t("시간")
}

// baro_gov_pre : 여러 줄 본문을 그대로 보여주는 상자
func baro_gov_pre(data string) string {
	return `<pre class="bw_code bw_gov_body">` + baro_gov_safe(data) + `</pre>`
}

// baro_gov_css : 11장 화면에서만 쓰는 보조 스타일
func baro_gov_css() string {
	return `<style id="bw_gov_css">
.bw_gov_body{line-height:1.75;}
.bw_gov_filter{display:flex;flex-wrap:wrap;gap:.6rem;align-items:flex-end;}
.bw_gov_filter .bw_f{flex:1 1 160px;margin:.2rem 0;}
.bw_gov_filter_btn{flex:0 0 auto;margin:.2rem 0;}
.bw_gov_diff{border:1px solid var(--bw-line,#d9dde2);border-radius:6px;overflow-x:auto;font-size:.86rem;font-family:ui-monospace,Consolas,monospace;}
.bw_gov_dl{display:flex;gap:0;white-space:pre-wrap;word-break:break-all;padding:.1rem .5rem;}
.bw_gov_dn{flex:0 0 3.4rem;color:var(--bw-mut,#6b7280);text-align:right;padding-right:.7rem;user-select:none;}
.bw_gov_dt{flex:1 1 auto;}
.bw_gov_add{background:rgba(40,180,114,.14);}
.bw_gov_del{background:rgba(220,67,67,.13);}
.bw_gov_same{background:transparent;}
.bw_gov_sign{font-weight:700;flex:0 0 1rem;}
.bw_gov_chart{max-width:100%;height:auto;}
.bw_gov_split{display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:1rem;}
</style>`
}

// ------------------------------------------------------------
//  줄 단위 비교 (11-2 정책 판 비교)
// ------------------------------------------------------------

// baro_gov_diff_row : 비교 결과 한 줄
type baro_gov_diff_row struct {
	Kind string // same / add / del
	Old  int    // 이전 판 줄 번호 (0 이면 없음)
	New  int    // 새 판 줄 번호 (0 이면 없음)
	Text string
}

// baro_gov_diff_max : 비교할 최대 줄 수 (아주 큰 본문에서 메모리를 아끼기 위한 상한)
const baro_gov_diff_max = 1500

// baro_gov_lines : 본문을 줄 단위로 나눈다 (빈 줄도 그대로 남긴다 — 판 비교에서 줄 번호가 맞아야 한다)
func baro_gov_lines(data string) []string {
	if data == "" {
		return []string{}
	}

	return strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
}

// baro_gov_diff_lines : 두 본문을 줄 단위로 비교한다 (최장 공통 부분열)
func baro_gov_diff_lines(old_data string, new_data string) []baro_gov_diff_row {
	old_lines := baro_gov_lines(old_data)
	new_lines := baro_gov_lines(new_data)

	cut := false
	if len(old_lines) > baro_gov_diff_max {
		old_lines = old_lines[:baro_gov_diff_max]
		cut = true
	}
	if len(new_lines) > baro_gov_diff_max {
		new_lines = new_lines[:baro_gov_diff_max]
		cut = true
	}

	n := len(old_lines)
	m := len(new_lines)

	// 최장 공통 부분열 표
	table := make([][]int, n+1)
	for i := range table {
		table[i] = make([]int, m+1)
	}

	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if old_lines[i] == new_lines[j] {
				table[i][j] = table[i+1][j+1] + 1
				continue
			}

			if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}

	out := []baro_gov_diff_row{}
	i, j := 0, 0

	for i < n && j < m {
		if old_lines[i] == new_lines[j] {
			out = append(out, baro_gov_diff_row{"same", i + 1, j + 1, old_lines[i]})
			i++
			j++
			continue
		}

		if table[i+1][j] >= table[i][j+1] {
			out = append(out, baro_gov_diff_row{"del", i + 1, 0, old_lines[i]})
			i++
			continue
		}

		out = append(out, baro_gov_diff_row{"add", 0, j + 1, new_lines[j]})
		j++
	}

	for ; i < n; i++ {
		out = append(out, baro_gov_diff_row{"del", i + 1, 0, old_lines[i]})
	}

	for ; j < m; j++ {
		out = append(out, baro_gov_diff_row{"add", 0, j + 1, new_lines[j]})
	}

	if cut {
		out = append(out, baro_gov_diff_row{"same", 0, 0, tool.Baro_t("… (본문이 너무 길어 앞부분만 비교했습니다)")})
	}

	return out
}

// baro_gov_diff_html : 비교 결과를 색으로 구분해 그린다
func baro_gov_diff_html(rows []baro_gov_diff_row) string {
	if len(rows) == 0 {
		return tool.Baro_empty(tool.Baro_t("비교할 내용이 없습니다."))
	}

	html := `<div class="bw_gov_diff">`
	for _, row := range rows {
		class := "bw_gov_same"
		sign := " "
		num := ""

		switch row.Kind {
		case "add":
			class = "bw_gov_add"
			sign = "+"
			num = strconv.Itoa(row.New)
		case "del":
			class = "bw_gov_del"
			sign = "-"
			num = strconv.Itoa(row.Old)
		default:
			if row.New > 0 {
				num = strconv.Itoa(row.New)
			}
		}

		text := row.Text
		if strings.TrimSpace(text) == "" {
			text = " "
		}

		html += `<div class="bw_gov_dl ` + class + `">`
		html += `<span class="bw_gov_dn">` + tool.HTML_escape(num) + `</span>`
		html += `<span class="bw_gov_sign">` + tool.HTML_escape(sign) + `</span>`
		html += `<span class="bw_gov_dt">` + baro_gov_safe(text) + `</span>`
		html += `</div>`
	}

	return html + `</div>`
}

// baro_gov_diff_stat : 추가·삭제 줄 수
func baro_gov_diff_stat(rows []baro_gov_diff_row) (int, int) {
	add, del := 0, 0
	for _, row := range rows {
		switch row.Kind {
		case "add":
			add++
		case "del":
			del++
		}
	}

	return add, del
}

// ------------------------------------------------------------
//  인라인 막대 그래프 (11-6 투명성 보고서)
// ------------------------------------------------------------

// baro_gov_bar_svg : 값 목록을 가로 막대 그래프 SVG 로 그린다
func baro_gov_bar_svg(items [][]string, title string) string {
	if len(items) == 0 {
		return tool.Baro_empty(tool.Baro_t("그래프로 그릴 값이 없습니다."))
	}

	max := 0
	for _, item := range items {
		if len(item) < 2 {
			continue
		}

		value := tool.Str_to_int(item[1])
		if value > max {
			max = value
		}
	}

	if max <= 0 {
		max = 1
	}

	row_h := 26
	pad_top := 10
	width := 620
	label_w := 170
	bar_w := width - label_w - 70
	height := pad_top*2 + row_h*len(items)

	svg := `<svg class="bw_gov_chart" viewBox="0 0 ` + strconv.Itoa(width) + ` ` + strconv.Itoa(height) + `" `
	svg += `width="100%" height="` + strconv.Itoa(height) + `" role="img" aria-label="` + tool.HTML_escape(title) + `">`

	y := pad_top
	for _, item := range items {
		if len(item) < 2 {
			continue
		}

		value := tool.Str_to_int(item[1])
		if value < 0 {
			value = 0
		}

		w := value * bar_w / max
		if value > 0 && w < 2 {
			w = 2
		}

		svg += `<text x="0" y="` + strconv.Itoa(y+16) + `" font-size="12" fill="currentColor">` + tool.HTML_escape(tool.Baro_trunc(item[0], 16)) + `</text>`
		svg += `<rect x="` + strconv.Itoa(label_w) + `" y="` + strconv.Itoa(y+4) + `" width="` + strconv.Itoa(bar_w) + `" height="16" rx="4" fill="currentColor" opacity="0.08"></rect>`
		svg += `<rect x="` + strconv.Itoa(label_w) + `" y="` + strconv.Itoa(y+4) + `" width="` + strconv.Itoa(w) + `" height="16" rx="4" fill="#010080"></rect>`
		svg += `<text x="` + strconv.Itoa(label_w+bar_w+8) + `" y="` + strconv.Itoa(y+16) + `" font-size="12" fill="currentColor">` + tool.HTML_escape(tool.Baro_comma(value)) + `</text>`

		y += row_h
	}

	return svg + `</svg>`
}
