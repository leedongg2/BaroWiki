package route

import (
	"database/sql"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 [cat] 모듈 공통 도우미
//
//  3-11 분류 도구 · 3-14 역링크/연결 문서 도구 가 함께 쓰는 부분이다.
//  연결 색인을 만드는 곳은 view_baro_cat_index.go 에 따로 두었다.
// ============================================================

// baro_cat_page_size : 한 쪽에 보여 주는 항목 수
const baro_cat_page_size = 30

// baro_cat_scan_max : 한 번에 훑는 최대 건수 (문서가 많아도 화면이 멈추지 않게 한다)
const baro_cat_scan_max = 5000

// baro_cat_text_scan_max : 본문까지 읽어야 하는 무거운 검사의 최대 건수
const baro_cat_text_scan_max = 2000

// baro_cat_metric_period : baro_metric 에 캐시를 저장할 때 쓰는 기간 값
const baro_cat_metric_period = "baro_cat_cache"

// baro_cat_cache_ver : 집계 방식이 바뀌면 올린다 (예전 방식으로 저장된 값은 버린다)
const baro_cat_cache_ver = "2"

// baro_cat_ns : 분류 이름공간 접두사
const baro_cat_ns = "category:"

// baro_cat_page_no : 쪽 번호 문자열을 1 이상의 정수로
func baro_cat_page_no(page string) int {
	num := tool.Str_to_int(page)
	if num < 1 {
		num = 1
	}

	// 지나치게 깊은 쪽은 막는다 (무거운 건너뛰기 방지)
	if num > 2000 {
		num = 2000
	}

	return num
}

// baro_cat_offset : 쪽 번호에 해당하는 건너뛸 개수
func baro_cat_offset(page int) int {
	return (page - 1) * baro_cat_page_size
}

// baro_cat_is_cat : 분류 문서 이름인지
func baro_cat_is_cat(title string) bool {
	return strings.HasPrefix(strings.ToLower(title), baro_cat_ns)
}

// baro_cat_norm_cat : 사용자가 넘긴 분류 이름을 내부 이름으로 (분류:X → category:X)
func baro_cat_norm_cat(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	name = tool.Namespace_normalize(name)
	if !baro_cat_is_cat(name) {
		name = baro_cat_ns + name
	}

	return name
}

// baro_cat_disp : 표시용 문서 이름
func baro_cat_disp(title string) string {
	return tool.Namespace_display(title)
}

// baro_cat_cat_link : 분류 도구 화면으로 가는 링크
func baro_cat_cat_link(name string) string {
	if name == "" {
		return `<span class="bw_muted">-</span>`
	}

	return `<a href="/baro/category/view/` + tool.Url_parser(name) + `">` +
		tool.HTML_escape(baro_cat_disp(name)) + `</a>`
}

// baro_cat_doc_tools : 문서 이름 옆에 붙는 도구 링크
func baro_cat_doc_tools(title string) string {
	return `<a class="bw_btn" href="/baro/link/doc/` + tool.Url_parser(title) + tool.Baro_t(`">연결</a> `) +
		`<a class="bw_btn" href="/w/` + tool.Url_parser(title) + tool.Baro_t(`">문서</a>`)
}

// baro_cat_doc_exists : data 표에 문서가 있는지
func baro_cat_doc_exists(db *sql.DB, title string) bool {
	if title == "" {
		return false
	}

	found := ""
	tool.QueryRow_DB(db, "select title from data where title = ? limit 1", []any{&found}, title)

	return found != ""
}

// baro_cat_count_one : 값 하나를 세는 질의
func baro_cat_count_one(db *sql.DB, query string, values ...any) int {
	count := 0
	tool.QueryRow_DB(db, query, []any{&count}, values...)

	return count
}

// ------------------------------------------------------------
//  무거운 집계 캐시 (baro_metric 을 그대로 쓴다)
// ------------------------------------------------------------

// baro_cat_cache_put : 집계 결과를 baro_metric 에 저장
func baro_cat_cache_put(db *sql.DB, key_name string, value int, detail map[string]string) {
	// 어떤 방식으로 센 값인지 함께 남긴다
	detail["ver"] = baro_cat_cache_ver

	found := ""
	exist := tool.QueryRow_DB(
		db,
		"select id from baro_metric where key_name = ? and period = ?",
		[]any{&found}, key_name, baro_cat_metric_period,
	)

	if exist {
		tool.Exec_DB(
			db,
			"update baro_metric set value = ?, detail = ?, date = ? where key_name = ? and period = ?",
			tool.Baro_itoa(value), tool.Baro_json_enc(detail), tool.Baro_now(), key_name, baro_cat_metric_period,
		)

		return
	}

	tool.Exec_DB(
		db,
		"insert into baro_metric (id, key_name, period, value, detail, date) values (?, ?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_metric"), key_name, baro_cat_metric_period,
		tool.Baro_itoa(value), tool.Baro_json_enc(detail), tool.Baro_now(),
	)
}

// baro_cat_cache_get : 저장된 집계 결과 (없으면 두 번째 값이 "")
func baro_cat_cache_get(db *sql.DB, key_name string) (map[string]string, string) {
	detail, date := "", ""
	tool.QueryRow_DB(
		db,
		"select detail, date from baro_metric where key_name = ? and period = ?",
		[]any{&detail, &date}, key_name, baro_cat_metric_period,
	)

	if date == "" {
		return map[string]string{}, ""
	}

	data := tool.Baro_json_dec_str(detail)

	// 집계 방식이 바뀌기 전에 저장된 값은 믿지 않고 버린다
	if data["ver"] != baro_cat_cache_ver {
		return map[string]string{}, ""
	}

	return data, date
}

// baro_cat_cache_num : 캐시 안의 숫자 값
func baro_cat_cache_num(data map[string]string, key_name string) int {
	return tool.Str_to_int(data[key_name])
}

// baro_cat_recalc_box : 집계 상태 안내 + 다시 계산 버튼
//
//	date 가 "" 면 저장된 집계가 없어 지금 즉석에서 센 값이라는 뜻이다.
func baro_cat_recalc_box(db *sql.DB, config tool.Config, action string, date string) string {
	body := ""
	if date == "" {
		body += tool.Baro_alert("info", tool.Baro_t("아직 저장된 집계가 없어 지금 즉석에서 센 값을 보여 주고 있습니다."))
	} else {
		body += tool.Baro_t(`<p>마지막 계산 : `) + tool.HTML_escape(date) + ` (` + tool.HTML_escape(tool.Baro_time_ago(date)) + `)</p>`
	}

	body += tool.Baro_note(tool.Baro_t("문서가 많으면 집계에 시간이 걸리므로 결과를 저장해 두고 보여 줍니다. 최신 값이 필요하면 다시 계산하세요."))

	if tool.Baro_can(db, config.IP, "review") {
		body += tool.Baro_form(action,
			tool.Baro_hidden("do", "recalc")+tool.Baro_submit(tool.Baro_t("다시 계산"), "primary"),
		)
	} else {
		body += tool.Baro_note(tool.Baro_t("다시 계산해서 저장하는 것은 검토 권한이 있는 사용자만 할 수 있습니다. 보는 것은 누구나 할 수 있습니다."))
	}

	return tool.Baro_card(tool.Baro_t("집계 상태"), body)
}

// baro_cat_recalc_guard : 재계산 요청을 받아들여도 되는지 확인한다
func baro_cat_recalc_guard(db *sql.DB, config tool.Config, back_url string) (string, bool) {
	if wall, need := tool.Baro_login_wall(db, config, back_url); need {
		return wall, false
	}

	if !tool.Baro_can(db, config.IP, "review") {
		return tool.Baro_deny(db, config, tool.Baro_t("집계를 다시 계산하려면 검토 권한이 필요합니다.")), false
	}

	if tool.Baro_read_only(db) {
		return tool.Baro_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db)), false
	}

	if !tool.Baro_rate_ok(db, "baro_cat_recalc", config.IP, 5, 600) {
		return tool.Baro_deny(db, config, tool.Baro_t("재계산 요청이 너무 잦습니다. 잠시 뒤에 다시 시도하세요.")), false
	}

	return "", true
}

// baro_cat_recalc_log : 재계산을 기록으로 남긴다
func baro_cat_recalc_log(db *sql.DB, config tool.Config, target string, summary string) {
	tool.Baro_public_log(db, "cat_recalc", config.IP, target, "연결·분류 집계 재계산", summary)
	tool.Baro_audit(db, config, "baro_cat_recalc", target, "", summary, "연결·분류 집계 재계산")

	if tool.Baro_is_user(config.IP) {
		tool.Baro_notify(db, config.IP, "system", "집계를 다시 계산했습니다 : "+target, "/baro/category", summary)
	}

	tool.Baro_metric_bump(db, "baro_cat_recalc", tool.Baro_today(), 1)
}

// baro_cat_who : 기여자 표시 (비로그인 편집자는 IP 를 가려서 보여 준다)
func baro_cat_who(db *sql.DB, who string, my_ip string) string {
	if who == "" {
		return `<span class="bw_muted">-</span>`
	}

	if tool.Baro_is_user(who) {
		return tool.Baro_user_link(db, who, my_ip)
	}

	return `<span class="bw_muted">` + tool.HTML_escape(tool.Baro_mask_ip(who)) + `</span>`
}

// baro_cat_menu : 화면 아래 이동 메뉴
func baro_cat_menu() [][]any {
	return [][]any{
		{"baro/category", tool.Baro_t("분류 도구")},
		{"baro/link", tool.Baro_t("연결 도구")},
		{"baro", "바로위키"},
	}
}

// baro_cat_tabs : 분류 도구 탭
func baro_cat_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/category", tool.Baro_t("분류 목록")},
		{"/baro/category/tree", tool.Baro_t("분류 트리")},
		{"/baro/category/stats", tool.Baro_t("분류 통계")},
		{"/baro/category/check", tool.Baro_t("분류 점검")},
		{"/baro/category/nocategory", tool.Baro_t("분류 없는 문서")},
	}, active)
}

// baro_cat_link_tabs : 연결 문서 도구 탭
func baro_cat_link_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/link", tool.Baro_t("연결 요약")},
		{"/baro/link/orphan", tool.Baro_t("고립 문서")},
		{"/baro/link/broken", tool.Baro_t("끊어진 링크")},
		{"/baro/link/deadend", tool.Baro_t("막다른 문서")},
	}, active)
}

// baro_cat_go_form : 문서(또는 분류) 이름을 받아 이동하는 작은 폼
func baro_cat_go_form(action string, field string, label string, value string, help string) string {
	return `<form class="bw_form" method="get" action="` + tool.HTML_escape(action) + `">` +
		tool.Baro_input(field, label, value, tool.Baro_t("이름을 입력하세요"), help) +
		tool.Baro_submit("보기", "primary") +
		`</form>`
}
