package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 7장 : 기여 인정 · 게임화 (contrib 모듈)
//
//  이 파일은 모듈 공통 도구와 7-1 기여 이벤트 기록 화면을 담는다.
//
//  0장 4번 원칙 : 편집 횟수를 그대로 점수로 쓰지 않는다.
//  행동을 종류로 나누어 기록하고, 가치는 따로 계산한다.
// ============================================================

// baro_contrib_page_size : 목록 한 쪽에 보여줄 줄 수
const baro_contrib_page_size = tool.Baro_page_size

// baro_contrib_merge_minutes : 같은 사용자가 같은 문서를 이 시간 안에 여러 번 편집하면 하나로 묶는다
const baro_contrib_merge_minutes = 10

// baro_contrib_typo_free : 하루에 이만큼까지는 오타 수정 가중치를 그대로 준다
const baro_contrib_typo_free = 2

// baro_contrib_scan_limit : 한 번에 훑을 편집 수
const baro_contrib_scan_limit = 500

// ------------------------------------------------------------
//  공통 도구
// ------------------------------------------------------------

// baro_contrib_esc : 화면에 넣기 전 반드시 거치는 이스케이프
func baro_contrib_esc(data string) string {
	return tool.HTML_escape(data)
}

// baro_contrib_bot_user : 위키가 스스로 남기는 편집에 쓰는 내부 계정 이름
const baro_contrib_bot_user = "system"

// baro_contrib_is_person : 사람 단위 기여로 인정할 수 있는 계정인가
//
//	비로그인(IP) 편집과 위키 내부 계정(system)은 사람의 기여로 세지 않는다.
//	기본 문서·틀을 자동으로 깔아 둔 편집이 사람의 기여 순위에 섞이면
//	'횟수보다 가치' 라는 원칙이 곧바로 무너지기 때문이다.
func baro_contrib_is_person(user string) bool {
	if user == "" || !tool.Baro_is_user(user) {
		return false
	}

	return user != baro_contrib_bot_user
}

// baro_contrib_page_num : 쪽 번호 읽기 (1 미만이면 1)
func baro_contrib_page_num(data string) int {
	num := tool.Str_to_int(strings.TrimSpace(data))
	if num < 1 {
		num = 1
	}

	// 지나치게 큰 쪽 번호는 막는다 (offset 폭주 방지)
	if num > 100000 {
		num = 100000
	}

	return num
}

// baro_contrib_get : url.Values 에서 값 하나 꺼내기 (nil 안전)
func baro_contrib_get(values url.Values, name string) string {
	if values == nil {
		return ""
	}

	return strings.TrimSpace(values.Get(name))
}

// baro_contrib_user_param : 경로에서 받은 사용자 이름 정리
func baro_contrib_user_param(data string) string {
	data = strings.TrimPrefix(data, "/")

	if decoded, err := url.PathUnescape(data); err == nil {
		data = decoded
	}

	return strings.TrimSpace(data)
}

// baro_contrib_query : 필터를 유지한 주소 만들기 ({} 는 쪽 번호 자리)
func baro_contrib_query(base string, pairs [][]string) string {
	parts := []string{}
	for _, pair := range pairs {
		if len(pair) < 2 || pair[1] == "" {
			continue
		}

		// 쪽 번호 자리표시자 {} 는 tool.Baro_paging 이 글자 그대로 찾아 바꾸므로
		// URL 인코딩하지 않는다. (인코딩하면 %7B%7D 가 되어 쪽 넘기기가 깨진다)
		value := url.QueryEscape(pair[1])
		if pair[1] == "{}" {
			value = "{}"
		}

		parts = append(parts, url.QueryEscape(pair[0])+"="+value)
	}

	if len(parts) == 0 {
		return base
	}

	return base + "?" + strings.Join(parts, "&")
}

// baro_contrib_menu : 화면 아래 이동 메뉴
func baro_contrib_menu() [][]any {
	return [][]any{
		{"baro", "바로위키"},
		{"baro/contrib", "기여 기록"},
		{"baro/rank", tool.Baro_t("분야별 순위")},
		{"baro/badge", "배지"},
		{"baro/thanks", tool.Baro_t("감사")},
	}
}

// baro_contrib_tabs : 7장 공통 탭 줄
func baro_contrib_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/contrib", "기여 기록"},
		{"/baro/contrib/score", tool.Baro_t("기여 가치")},
		{"/baro/contrib/expertise", tool.Baro_t("전문 분야")},
		{"/baro/rank", tool.Baro_t("분야별 순위")},
		{"/baro/badge", "배지"},
		{"/baro/thanks", tool.Baro_t("감사")},
	}, active)
}

// baro_contrib_kind_options : 기여 종류 선택 상자 항목 (맨 앞은 전체)
func baro_contrib_kind_options(all_text string) [][]string {
	out := [][]string{{"", all_text}}
	for _, row := range tool.Baro_contrib_kind_list() {
		out = append(out, []string{row[0], row[1]})
	}

	return out
}

// baro_contrib_field_expr : 기여 한 건의 '지금 분야' 를 구하는 SQL 조각
//
//	이벤트의 field 는 저장 시점에 굳은 값이라, 문서 분야를 나중에 정하거나 고쳐도
//	예전 이벤트는 그대로 남는다. 그러면 분야별 집계에 영영 잡히지 않는다.
//	그래서 문서의 지금 분야(baro_doc_meta 의 field)를 먼저 보고,
//	그것이 비어 있을 때만 이벤트에 적힌 값을 쓴다.
//
//	alias 는 코드가 정하는 표 별칭이라 사용자 입력이 들어오지 않는다.
func baro_contrib_field_expr(alias string) string {
	return "coalesce(nullif((select m.data from baro_doc_meta m where m.title = " + alias +
		".title and m.name = 'field'), ''), " + alias + ".field)"
}

// baro_contrib_field_options : 분야 선택 상자 항목
func baro_contrib_field_options(db *sql.DB, all_text string) [][]string {
	out := [][]string{{"", all_text}}
	for _, row := range tool.Baro_field_list(db) {
		out = append(out, []string{row.Key, row.Name})
	}

	return out
}

// baro_contrib_kind_valid : 알 수 없는 종류를 걸러낸다
func baro_contrib_kind_valid(kind string) bool {
	if kind == "" {
		return false
	}

	for _, row := range tool.Baro_contrib_kind_list() {
		if row[0] == kind {
			return true
		}
	}

	return false
}

// baro_contrib_event_row : 기여 이벤트 한 줄
type baro_contrib_event_row struct {
	ID      string
	User    string
	Kind    string
	Title   string
	Rev     string
	Project string
	Weight  string
	Detail  string
	Date    string
	Status  string
	Field   string
}

// baro_contrib_event_cols : 이벤트 조회 컬럼
//
//	분야만은 이벤트에 굳어 있는 값 대신 문서의 지금 분야를 읽는다.
//	목록·통계 화면이 모두 같은 값을 보게 하기 위해서다.
var baro_contrib_event_cols = "id, user, kind, title, rev, project, weight, detail, date, status, " +
	baro_contrib_field_expr("baro_contrib_event")

// baro_contrib_event_scan : 조회 결과를 구조체 목록으로
func baro_contrib_event_scan(rows *sql.Rows) []baro_contrib_event_row {
	out := []baro_contrib_event_row{}

	for rows.Next() {
		row := baro_contrib_event_row{}
		if err := rows.Scan(
			&row.ID, &row.User, &row.Kind, &row.Title, &row.Rev,
			&row.Project, &row.Weight, &row.Detail, &row.Date, &row.Status, &row.Field,
		); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_contrib_event_one : 이벤트 하나 읽기
func baro_contrib_event_one(db *sql.DB, id string) (baro_contrib_event_row, bool) {
	row := baro_contrib_event_row{}

	found := tool.QueryRow_DB(
		db,
		"select "+baro_contrib_event_cols+" from baro_contrib_event where id = ?",
		[]any{
			&row.ID, &row.User, &row.Kind, &row.Title, &row.Rev,
			&row.Project, &row.Weight, &row.Detail, &row.Date, &row.Status, &row.Field,
		},
		id,
	)

	return row, found
}

// baro_contrib_status_badge : 이벤트 상태 배지
func baro_contrib_status_badge(status string) string {
	switch status {
	case "confirmed":
		return tool.Baro_badge(tool.Baro_t("확정"), "ok")
	case "rejected":
		return tool.Baro_badge(tool.Baro_t("제외됨"), "danger")
	case "merged":
		return tool.Baro_badge(tool.Baro_t("묶임"), "muted")
	case "pending", "":
		return tool.Baro_badge(tool.Baro_t("확정 대기"), "info")
	}

	return tool.Baro_badge(status, "muted")
}

// baro_contrib_count : 조건에 맞는 행 수 세기
func baro_contrib_count(db *sql.DB, query string, args ...any) int {
	count := 0
	tool.QueryRow_DB(db, query, []any{&count}, args...)

	return count
}

// baro_contrib_doc_exists : 문서가 지금도 살아 있는가
func baro_contrib_doc_exists(db *sql.DB, title string) bool {
	if title == "" {
		return false
	}

	found := ""
	return tool.QueryRow_DB(db, "select title from data where title = ?", []any{&found}, title)
}

// baro_contrib_rank_public : 순위 공개를 허용한 사용자인가 (기본값은 공개)
func baro_contrib_rank_public(db *sql.DB, user string) bool {
	data := ""
	tool.QueryRow_DB(db, "select data from baro_user_set where user = ? and name = 'rank_public'", []any{&data}, user)

	return data != "off"
}

// baro_contrib_event_link : 이벤트가 가리키는 문서와 판을 나타내는 HTML
func baro_contrib_event_link(row baro_contrib_event_row) string {
	html := tool.Baro_doc_link(row.Title)
	if row.Rev != "" {
		html += ` <span class="bw_muted">r` + baro_contrib_esc(row.Rev) + `</span>`
	}

	return html
}

// baro_contrib_kind_summary : 종류별 집계를 표로 (사용자 한 명 기준)
func baro_contrib_kind_summary(db *sql.DB, user string) ([][]string, int, int) {
	counts := map[string]int{}
	weights := map[string]int{}

	rows := tool.Query_DB(
		db,
		"select kind, count(*), coalesce(sum(weight + 0), 0) from baro_contrib_event where user = ? group by kind",
		user,
	)
	defer rows.Close()

	for rows.Next() {
		kind, count, weight := "", 0, 0
		if err := rows.Scan(&kind, &count, &weight); err != nil {
			continue
		}

		counts[kind] = count
		weights[kind] = weight
	}

	table := [][]string{}
	total_count, total_weight := 0, 0

	for _, kind_row := range tool.Baro_contrib_kind_list() {
		kind := kind_row[0]
		if counts[kind] == 0 {
			continue
		}

		total_count += counts[kind]
		total_weight += weights[kind]

		table = append(table, []string{
			baro_contrib_esc(kind_row[1]),
			tool.Baro_comma(counts[kind]) + tool.Baro_t("건"),
			tool.Baro_comma(weights[kind]),
		})
	}

	return table, total_count, total_weight
}

// ------------------------------------------------------------
//  7-1 기여 이벤트 목록 — /baro/contrib
// ------------------------------------------------------------

// View_baro_contrib_list : 최근 기여 이벤트 목록 (종류·사용자·분야 필터)
func View_baro_contrib_list(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page := baro_contrib_page_num(baro_contrib_get(query, "page"))
	kind := baro_contrib_get(query, "kind")
	user := baro_contrib_get(query, "user")
	field := baro_contrib_get(query, "field")

	if !baro_contrib_kind_valid(kind) {
		kind = ""
	}

	where := " where 1 = 1"
	args := []any{}

	if kind != "" {
		where += " and kind = ?"
		args = append(args, kind)
	}
	if user != "" {
		where += " and user = ?"
		args = append(args, user)
	}
	if field != "" {
		// 문서 분야가 나중에 정해진 기여도 함께 걸리도록 지금 분야로 비교한다
		where += " and " + baro_contrib_field_expr("baro_contrib_event") + " = ?"
		args = append(args, field)
	}

	list_args := append(append([]any{}, args...), (page-1)*baro_contrib_page_size, baro_contrib_page_size)

	rows := tool.Query_DB(
		db,
		"select "+baro_contrib_event_cols+" from baro_contrib_event"+where+" order by id + 0 desc limit ?, ?",
		list_args...,
	)
	defer rows.Close()

	list := baro_contrib_event_scan(rows)

	// --- 요약 지표 ---
	total := baro_contrib_count(db, "select count(*) from baro_contrib_event"+where, args...)
	today_count := baro_contrib_count(db, "select count(*) from baro_contrib_event where date like ?", tool.Baro_today()+"%")
	user_count := baro_contrib_count(db, "select count(distinct user) from baro_contrib_event")

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", "기여 기록"}})
	body += tool.Baro_hero(
		"기여 기록",
		tool.Baro_t("편집 횟수가 아니라 어떤 행동을 했는지를 기록합니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("조건에 맞는 기여"), tool.Baro_comma(total), tool.Baro_t("이벤트")},
			{tool.Baro_t("오늘 기록된 기여"), tool.Baro_comma(today_count), tool.Baro_t("이벤트")},
			{tool.Baro_t("기여한 사람"), tool.Baro_comma(user_count), "명"},
		}),
	)
	body += baro_contrib_tabs("/baro/contrib")

	body += tool.Baro_alert("info",
		tool.Baro_t("바로위키는 <b>오타 한 글자 수정 1,000번</b>과 <b>좋은 문서 100개 작성</b>을 같게 보지 않습니다. ")+
			tool.Baro_t("그래서 편집을 행동 종류로 나누어 기록하고, 가치는 ")+
			tool.Baro_t(`<a href="/baro/contrib/score">기여 가치 계산</a> 화면에서 따로 다룹니다.`),
	)

	// --- 필터 폼 ---
	filter := tool.Baro_select("kind", tool.Baro_t("행동 종류"), baro_contrib_kind_options(tool.Baro_t("전체 종류")), kind, "")
	filter += tool.Baro_input("user", tool.Baro_t("사용자"), user, tool.Baro_t("사용자 이름"), tool.Baro_t("정확히 일치하는 이름만 찾습니다."))
	filter += tool.Baro_select("field", "분야", baro_contrib_field_options(db, tool.Baro_t("전체 분야")), field, "")
	filter += tool.Baro_submit("걸러 보기", "primary") + " " + tool.Baro_button("/baro/contrib", tool.Baro_t("필터 지우기"), "")

	body += tool.Baro_card(tool.Baro_t("기록 걸러 보기"),
		`<form class="bw_form" method="get" action="/baro/contrib">`+filter+`</form>`,
	)

	// --- 목록 ---
	table_rows := [][]string{}
	for _, row := range list {
		table_rows = append(table_rows, []string{
			tool.Baro_time_cell(row.Date),
			tool.Baro_user_link(db, row.User, config.IP),
			tool.Baro_badge(tool.Baro_contrib_kind_name(row.Kind), "info"),
			baro_contrib_event_link(row),
			baro_contrib_esc(tool.Baro_field_name(db, row.Field)),
			baro_contrib_status_badge(row.Status),
		})
	}

	list_html := ""
	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("조건에 맞는 기여 기록이 없습니다. 필터를 지우거나 다른 조건으로 찾아 보세요."))
	} else {
		list_html = tool.Baro_table(
			[]string{"시각", tool.Baro_t("사용자"), tool.Baro_t("행동"), "문서", "분야", "상태"},
			table_rows,
			tool.Baro_t("조건에 맞는 기여 기록이 없습니다."),
		)
	}

	paging_url := baro_contrib_query("/baro/contrib", [][]string{
		{"kind", kind}, {"user", user}, {"field", field}, {"page", "{}"},
	})
	list_html += tool.Baro_paging(page, len(list), baro_contrib_page_size, paging_url)

	body += tool.Baro_card(tool.Baro_t("최근 기여 이벤트"), list_html)

	// --- 행동 종류 설명 ---
	kind_rows := [][]string{}
	for _, row := range tool.Baro_contrib_kind_list() {
		count := baro_contrib_count(db, "select count(*) from baro_contrib_event where kind = ?", row[0])
		kind_rows = append(kind_rows, []string{
			tool.Baro_badge(row[1], "info"),
			`<a href="` + baro_contrib_esc(baro_contrib_query("/baro/contrib", [][]string{{"kind", row[0]}})) + `">` + baro_contrib_esc(row[0]) + `</a>`,
			tool.Baro_comma(count) + tool.Baro_t("건"),
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("기록되는 행동 종류"),
		tool.Baro_t("같은 편집이라도 무엇을 했는지에 따라 다르게 기록됩니다."),
		tool.Baro_table([]string{tool.Baro_t("행동"), tool.Baro_t("키"), tool.Baro_t("쌓인 기록")}, kind_rows, tool.Baro_t("행동 종류가 없습니다.")),
		tool.Baro_button("/baro/contrib/scan", tool.Baro_t("기존 편집 기록 훑기"), "")+" "+
			tool.Baro_button("/baro/contrib/score", tool.Baro_t("기여 가치 계산 보기"), ""),
	)

	return tool.Baro_page(db, config, "기여 기록", body, baro_contrib_menu())
}

// ------------------------------------------------------------
//  7-1 사용자별 기여 — /baro/contrib/user/<사용자>
// ------------------------------------------------------------

// View_baro_contrib_user : 한 사람의 기여 이벤트 목록 (종류별 집계 포함)
func View_baro_contrib_user(config tool.Config, user_raw string, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	user := baro_contrib_user_param(user_raw)

	if user == "" {
		body := tool.Baro_crumb([][]string{{"/baro/contrib", "기여 기록"}, {"", tool.Baro_t("사용자")}})
		body += tool.Baro_alert("warn", tool.Baro_t("사용자 이름이 비어 있습니다. 주소를 확인해 주세요."))
		body += tool.Baro_card(tool.Baro_t("사용자 기여 보기"),
			`<form class="bw_form" method="get" action="/baro/contrib">`+
				tool.Baro_input("user", tool.Baro_t("사용자 이름"), "", tool.Baro_t("사용자 이름"), "")+
				tool.Baro_submit(tool.Baro_t("찾기"), "primary")+
				`</form>`,
		)

		return tool.Baro_page(db, config, tool.Baro_t("사용자 기여"), body, baro_contrib_menu())
	}

	page := baro_contrib_page_num(baro_contrib_get(query, "page"))
	kind := baro_contrib_get(query, "kind")
	if !baro_contrib_kind_valid(kind) {
		kind = ""
	}

	where := " where user = ?"
	args := []any{user}

	if kind != "" {
		where += " and kind = ?"
		args = append(args, kind)
	}

	list_args := append(append([]any{}, args...), (page-1)*baro_contrib_page_size, baro_contrib_page_size)

	rows := tool.Query_DB(
		db,
		"select "+baro_contrib_event_cols+" from baro_contrib_event"+where+" order by id + 0 desc limit ?, ?",
		list_args...,
	)
	defer rows.Close()

	list := baro_contrib_event_scan(rows)

	summary, total_count, total_weight := baro_contrib_kind_summary(db, user)

	// 확정된 가치 합계
	confirmed_sum := 0
	tool.QueryRow_DB(
		db,
		"select coalesce(sum(final + 0), 0) from baro_contrib_score where user = ? and confirmed = 'on'",
		[]any{&confirmed_sum},
		user,
	)

	first_date := ""
	tool.QueryRow_DB(db, "select date from baro_contrib_event where user = ? order by id + 0 asc limit 1", []any{&first_date}, user)

	active_days := baro_contrib_count(db, "select count(*) from baro_calendar where user = ?", user)

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/contrib", "기여 기록"},
		{"", tool.Namespace_display(user)},
	})

	body += tool.Baro_hero(
		user+tool.Baro_t(" 님의 기여"),
		tool.Baro_t("어떤 행동을 얼마나 했는지 보여 줍니다. 순위나 등수를 매기는 화면이 아닙니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("기록된 기여"), tool.Baro_comma(total_count), tool.Baro_t("이벤트")},
			{tool.Baro_t("행동 가중치 합"), tool.Baro_comma(total_weight), tool.Baro_t("확정 점수와 다릅니다")},
			{tool.Baro_t("확정된 기여 가치"), tool.Baro_comma(confirmed_sum), tool.Baro_t("확정된 것만")},
			{tool.Baro_t("활동한 날"), tool.Baro_comma(active_days), tool.Baro_t("일")},
		}),
	)

	body += tool.Baro_tabs([][]string{
		{"/baro/contrib/user/" + tool.Url_parser(user), "기여 기록"},
		{"/baro/contrib/impact/" + tool.Url_parser(user), "기여 영향력"},
		{"/baro/contrib/calendar/" + tool.Url_parser(user), "활동 캘린더"},
		{"/baro/badge/user/" + tool.Url_parser(user), "배지"},
	}, "/baro/contrib/user/"+tool.Url_parser(user))

	if total_count == 0 {
		body += tool.Baro_alert("info", tool.Baro_t("아직 이 사용자의 기여 기록이 없습니다. 기록이 없다고 해서 기여가 없었다는 뜻은 아닙니다 — ")+
			tool.Baro_t(`<a href="/baro/contrib/scan">기존 편집 기록 훑기</a> 를 아직 돌리지 않았을 수 있습니다.`))
	}

	if first_date != "" {
		body += tool.Baro_note(tool.Baro_t("첫 기여 기록 : ") + first_date + " (" + tool.Baro_time_ago(first_date) + ")")
	}

	// --- 종류별 집계 ---
	body += tool.Baro_card_ex(
		tool.Baro_t("행동 종류별 집계"),
		tool.Baro_t("횟수만이 아니라 무엇을 했는지를 봅니다."),
		tool.Baro_table([]string{tool.Baro_t("행동"), tool.Baro_t("건수"), tool.Baro_t("가중치 합")}, summary, tool.Baro_t("집계할 기여가 없습니다.")),
		tool.Baro_note(tool.Baro_t("가중치 합은 행동 종류의 기본값을 더한 것일 뿐이며, 최종 기여 가치와 같지 않습니다.")),
	)

	// --- 목록 ---
	table_rows := [][]string{}
	for _, row := range list {
		table_rows = append(table_rows, []string{
			tool.Baro_time_cell(row.Date),
			tool.Baro_badge(tool.Baro_contrib_kind_name(row.Kind), "info"),
			baro_contrib_event_link(row),
			baro_contrib_esc(tool.Baro_field_name(db, row.Field)),
			baro_contrib_esc(tool.Baro_trunc(tool.Baro_one_line(row.Detail), 60)),
			baro_contrib_status_badge(row.Status),
		})
	}

	list_html := ""
	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("표시할 기여 기록이 없습니다."))
	} else {
		list_html = tool.Baro_table(
			[]string{"시각", tool.Baro_t("행동"), "문서", "분야", tool.Baro_t("메모"), "상태"},
			table_rows,
			tool.Baro_t("표시할 기여 기록이 없습니다."),
		)
	}

	filter := tool.Baro_select("kind", tool.Baro_t("행동 종류"), baro_contrib_kind_options(tool.Baro_t("전체 종류")), kind, "")
	filter += tool.Baro_submit("걸러 보기", "")
	list_html = `<form class="bw_form" method="get" action="/baro/contrib/user/` + baro_contrib_esc(tool.Url_parser(user)) + `">` + filter + `</form>` + list_html

	paging_url := baro_contrib_query("/baro/contrib/user/"+tool.Url_parser(user), [][]string{
		{"kind", kind}, {"page", "{}"},
	})
	list_html += tool.Baro_paging(page, len(list), baro_contrib_page_size, paging_url)

	body += tool.Baro_card("기여 이벤트", list_html)

	return tool.Baro_page(db, config, user+tool.Baro_t(" 님의 기여"), body, baro_contrib_menu())
}

// ------------------------------------------------------------
//  7-1 기존 편집 훑기 — /baro/contrib/scan (권한 system)
// ------------------------------------------------------------

// baro_contrib_classify : 편집 한 건을 행동 종류로 분류한다
//
//	저장 시점 분류(tool.Baro_classify_edit)와 훑기 재분류가 어긋나면
//	같은 편집이 화면마다 다른 종류로 보이게 된다.
//	그래서 규칙을 여기서 다시 쓰지 않고 기반 계층 함수를 그대로 부른다.
func baro_contrib_classify(rev string, prev_data string, now_data string, send string) string {
	return tool.Baro_classify_edit(prev_data, now_data, send, rev)
}

// baro_contrib_prev_data : 바로 앞 판의 본문
func baro_contrib_prev_data(db *sql.DB, title string, rev string) string {
	data := ""
	tool.QueryRow_DB(
		db,
		"select data from history where title = ? and id + 0 < ? order by id + 0 desc limit 1",
		[]any{&data},
		title, tool.Str_to_int(rev),
	)

	return data
}

// baro_contrib_calendar_bump_at : 특정 날짜의 활동 캘린더를 갱신한다
//
//	기반 계층의 Baro_calendar_bump 는 오늘 날짜에만 더하므로,
//	과거 편집을 훑을 때는 편집 날짜에 맞춰 넣기 위해 이 함수를 쓴다.
func baro_contrib_calendar_bump_at(db *sql.DB, user string, kind string, date string) {
	day := tool.Baro_date_only(date)
	if day == "" {
		day = tool.Baro_today()
	}

	num, detail := "0", ""
	found := tool.QueryRow_DB(db, "select num, detail from baro_calendar where user = ? and date = ?", []any{&num, &detail}, user, day)

	kinds := tool.Baro_json_dec_str(detail)
	kinds[kind] = tool.Baro_itoa(tool.Str_to_int(kinds[kind]) + 1)

	if found {
		tool.Exec_DB(
			db,
			"update baro_calendar set num = ?, detail = ? where user = ? and date = ?",
			tool.Baro_itoa(tool.Str_to_int(num)+1), tool.Baro_json_enc(kinds), user, day,
		)

		return
	}

	tool.Exec_DB(
		db,
		"insert into baro_calendar (user, date, num, detail) values (?, ?, '1', ?)",
		user, day, tool.Baro_json_enc(kinds),
	)
}

// baro_contrib_event_insert : 편집 날짜를 그대로 살려 기여 이벤트를 넣는다
func baro_contrib_event_insert(db *sql.DB, user string, kind string, title string, rev string, detail string, date string) string {
	if !tool.Baro_is_user(user) {
		return ""
	}

	id := tool.Baro_next_id(db, "baro_contrib_event")
	field := tool.Baro_doc_field(db, title)

	if date == "" {
		date = tool.Baro_now()
	}

	tool.Exec_DB(
		db,
		"insert into baro_contrib_event (id, user, kind, title, rev, project, weight, detail, date, status, field) values (?, ?, ?, ?, ?, '', ?, ?, ?, 'pending', ?)",
		id, user, kind, title, rev,
		tool.Baro_itoa(tool.Baro_contrib_kind_weight(kind)), tool.Baro_trunc(detail, 500), date, field,
	)

	baro_contrib_calendar_bump_at(db, user, kind, date)

	return id
}

// baro_contrib_scan_cursor_name : 마지막으로 훑은 편집 시각을 적어 두는 설정 이름
//
//	예전에는 이 값을 "여기부터 다시 훑는다" 는 시작 위치로 썼다.
//	그런데 편집 시각은 초 단위라서 같은 초에 저장된 편집이 여러 건이면
//	한 건만 처리되고 나머지는 시작 위치 뒤로 밀려 영영 대상에서 빠졌다.
//	지금은 이 값을 고르는 조건으로 쓰지 않고, 마지막 실행이 어디까지 닿았는지
//	사람이 볼 수 있게 적어 두기만 한다. 실제로 무엇을 훑을지는
//	baro_contrib_scan_no_event_where 가 (문서, 판) 조합으로 정한다.
const baro_contrib_scan_cursor_name = "contrib_scan_cursor"

// baro_contrib_field_fill : 분야가 비어 있는 기여 이벤트를 문서의 지금 분야로 채운다
//
//	이벤트의 field 는 저장 시점 값이라, 문서에 분야를 나중에 지정하면 빈 채로 남는다.
//	집계 화면은 baro_contrib_field_expr 로 문서의 지금 분야를 함께 보지만,
//	이벤트 자체도 채워 두어야 저장된 순위(baro_rank)와 내려받은 자료까지 같아진다.
//	채운 이벤트 수를 돌려준다.
func baro_contrib_field_fill(db *sql.DB, limit int) int {
	titles := []string{}

	rows := tool.Query_DB(
		db,
		"select distinct title from baro_contrib_event where title <> '' and (field is null or field = '') limit ?, ?",
		0, limit,
	)
	defer rows.Close()

	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		titles = append(titles, title)
	}
	rows.Close()

	filled := 0
	for _, title := range titles {
		field := tool.Baro_doc_field(db, title)
		if field == "" {
			continue
		}

		filled += baro_contrib_count(
			db,
			"select count(*) from baro_contrib_event where title = ? and (field is null or field = '')",
			title,
		)
		tool.Exec_DB(
			db,
			"update baro_contrib_event set field = ? where title = ? and (field is null or field = '')",
			field, title,
		)
	}

	return filled
}

// baro_contrib_scan_row : 훑기 대상 한 줄
type baro_contrib_scan_row struct {
	Rev   string
	Title string
	Data  string
	Date  string
	IP    string
	Send  string
	Leng  string
}

// baro_contrib_scan_person_where : 사람이 한 편집만 고르는 조건
//
//	IP 주소에는 반드시 . 이나 : 가 들어가므로 그것으로 비로그인 편집을 걸러내고,
//	위키가 스스로 남긴 내부 계정 편집은 이름으로 걸러낸다. (자리표시자에 계정 이름이 들어간다)
const baro_contrib_scan_person_where = " and history.title <> '' and history.ip <> '' and history.ip not like '%.%' and history.ip not like '%:%' and history.ip <> ?"

// baro_contrib_scan_no_event_where : 아직 기여 이벤트가 없는 편집만 고르는 조건
const baro_contrib_scan_no_event_where = " and not exists (select 1 from baro_contrib_event where baro_contrib_event.title = history.title and baro_contrib_event.rev = history.id)"

// baro_contrib_scan_fetch : 아직 기여 이벤트가 없는 편집을 가져온다
//
//	시각으로 자르지 않는다. 이미 이벤트가 있는 (문서, 판) 조합만 빼고 가져오므로
//	같은 초에 저장된 편집이 여러 건이어도 빠지지 않는다.
//	그래서 화면에 보이는 '아직 분류되지 않음' 건수와 실제로 훑는 건수가 언제나 같다.
func baro_contrib_scan_fetch(db *sql.DB, limit int) []baro_contrib_scan_row {
	out := []baro_contrib_scan_row{}

	rows := tool.Query_DB(
		db,
		"select id, title, data, date, ip, send, leng from history"+
			" where 1 = 1"+
			baro_contrib_scan_person_where+
			baro_contrib_scan_no_event_where+
			" order by date asc, title asc, id + 0 asc limit ?, ?",
		baro_contrib_bot_user, 0, limit,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_contrib_scan_row{}
		if err := rows.Scan(&row.Rev, &row.Title, &row.Data, &row.Date, &row.IP, &row.Send, &row.Leng); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// View_baro_contrib_scan : 쌓인 편집 기록을 행동 종류로 분류해 한 번에 등록
func View_baro_contrib_scan(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/contrib/scan"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("기존 편집 훑기는 시스템 관리자만 실행할 수 있습니다."))
	}

	cursor := tool.Baro_get_def(db, baro_contrib_scan_cursor_name, "0000-00-00 00:00:00")

	// --- 실행 ---
	if values != nil {
		if tool.Baro_read_only(db) {
			body := tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 실행할 수 없습니다. ")+baro_contrib_esc(tool.Baro_read_only_reason(db)))

			return tool.Baro_page(db, config, tool.Baro_t("기존 편집 훑기"), body, baro_contrib_menu())
		}

		targets := baro_contrib_scan_fetch(db, baro_contrib_scan_limit)

		added := map[string]int{}
		total_added, skipped_ip, skipped_hidden := 0, 0, 0
		last_date := cursor

		for _, row := range targets {
			if row.Date > last_date {
				last_date = row.Date
			}

			if row.Title == "" {
				skipped_hidden++
				continue
			}

			// 로그인하지 않은 편집(IP 편집)과 위키 내부 계정(system)의 자동 편집은
			// 사람 단위 기여로 인정하지 않는다.
			if !baro_contrib_is_person(row.IP) {
				skipped_ip++
				continue
			}

			prev := ""
			if tool.Str_to_int(row.Rev) != 1 {
				prev = baro_contrib_prev_data(db, row.Title, row.Rev)
			}

			kind := baro_contrib_classify(row.Rev, prev, row.Data, row.Send)

			detail := tool.Baro_trunc(tool.Baro_one_line(row.Send), 200)
			if detail == "" {
				detail = tool.Baro_t("편집 기록 훑기로 분류됨")
			}

			if baro_contrib_event_insert(db, row.IP, kind, row.Title, row.Rev, detail, row.Date) != "" {
				added[kind]++
				total_added++
			}
		}

		// 분야가 비어 있는 이벤트를 문서의 지금 분야로 채운다 (7-1 분야별 집계 보정)
		filled := baro_contrib_field_fill(db, baro_contrib_scan_limit)

		tool.Baro_put(db, baro_contrib_scan_cursor_name, last_date)

		tool.Baro_public_log(db, "contrib_scan", config.IP, "history",
			"기존 편집 기록을 행동 종류로 분류", tool.Baro_itoa(total_added)+"건 등록")
		tool.Baro_audit(db, config, "contrib_scan", "history", cursor, last_date,
			"훑은 편집 "+tool.Baro_itoa(len(targets))+"건 / 등록 "+tool.Baro_itoa(total_added)+"건"+
				" / 분야 보정 "+tool.Baro_itoa(filled)+"건")
		tool.Baro_metric_bump(db, "contrib_scan", tool.Baro_today(), total_added)

		return tool.Get_redirect("/baro/contrib/scan?done=" + tool.Baro_itoa(total_added) +
			"&filled=" + tool.Baro_itoa(filled))
	}

	// --- 안내 화면 ---
	done := baro_contrib_get(query, "done")

	// 진행률은 '사람이 한 편집' 을 분모로 잡는다.
	// IP 편집과 내부 계정 편집은 아예 대상이 아니므로 분모에 넣으면 영원히 100% 가 되지 않는다.
	remain := baro_contrib_count(
		db,
		"select count(*) from history where 1 = 1"+baro_contrib_scan_person_where+baro_contrib_scan_no_event_where,
		baro_contrib_bot_user,
	)
	person_total := baro_contrib_count(
		db,
		"select count(*) from history where 1 = 1"+baro_contrib_scan_person_where,
		baro_contrib_bot_user,
	)
	history_total := baro_contrib_count(db, "select count(*) from history")
	event_total := baro_contrib_count(db, "select count(*) from baro_contrib_event")

	body := tool.Baro_crumb([][]string{{"/baro/contrib", "기여 기록"}, {"", tool.Baro_t("기존 편집 훑기")}})
	body += tool.Baro_hero(
		tool.Baro_t("기존 편집 기록 훑기"),
		tool.Baro_t("이미 쌓인 편집을 행동 종류로 분류해 기여 이벤트로 등록합니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("전체 편집 기록"), tool.Baro_comma(history_total), "history"},
			{tool.Baro_t("사람이 한 편집"), tool.Baro_comma(person_total), tool.Baro_t("훑기 대상")},
			{tool.Baro_t("등록된 기여 이벤트"), tool.Baro_comma(event_total), "baro_contrib_event"},
			{tool.Baro_t("아직 분류되지 않음"), tool.Baro_comma(remain), "건"},
			{tool.Baro_t("한 번에 처리"), tool.Baro_comma(baro_contrib_scan_limit), "건"},
		}),
	)
	body += baro_contrib_tabs("/baro/contrib")

	body += tool.Baro_progress(person_total-remain, person_total, tool.Baro_t("분류 진행 상황 (사람이 한 편집 기준)"))

	if history_total > person_total {
		body += tool.Baro_note(tool.Baro_t("전체 편집 ") + tool.Baro_comma(history_total) + tool.Baro_t("건 가운데 ") +
			tool.Baro_comma(history_total-person_total) + tool.Baro_t("건은 IP 편집이거나 위키가 스스로 남긴 편집이라 훑기 대상이 아닙니다."))
	}

	if done != "" {
		filled := baro_contrib_get(query, "filled")
		alert_text := tool.Baro_t("직전 실행에서 <b>") + baro_contrib_esc(tool.Baro_comma(tool.Str_to_int(done))) + tool.Baro_t("건</b>의 기여 이벤트를 등록했습니다.")
		if tool.Str_to_int(filled) > 0 {
			alert_text += tool.Baro_t(" 분야가 비어 있던 이벤트 <b>") + baro_contrib_esc(tool.Baro_comma(tool.Str_to_int(filled))) +
				tool.Baro_t("건</b>을 문서의 지금 분야로 채웠습니다.")
		}

		body += tool.Baro_alert("ok", alert_text)
	}

	body += tool.Baro_card(tool.Baro_t("분류 규칙"), tool.Baro_note(tool.Baro_t("편집을 저장할 때 쓰는 규칙과 똑같은 규칙으로 다시 분류합니다. ")+
		tool.Baro_t("그래서 같은 편집이 저장 시점과 훑기에서 다른 종류로 잡히는 일이 없습니다."))+
		tool.Baro_list([]string{
			tool.Baro_t("역사 판 번호가 <b>1</b> 이거나 이전 판 본문이 비어 있으면 → ") + baro_contrib_esc(tool.Baro_contrib_kind_name("new_document")),
			tool.Baro_t("본문의 각주가 늘었으면 → ") + baro_contrib_esc(tool.Baro_contrib_kind_name("source")),
			tool.Baro_t("분류 수가 바뀌고 길이 변화가 <b>120자 미만</b> 이면 → ") + baro_contrib_esc(tool.Baro_contrib_kind_name("category")),
			tool.Baro_t("길이 변화가 <b>3자 이하</b> 이면 → ") + baro_contrib_esc(tool.Baro_contrib_kind_name("typo")),
			tool.Baro_t("길이가 <b>300자 이상</b> 늘었으면 → ") + baro_contrib_esc(tool.Baro_contrib_kind_name("expand")),
			tool.Baro_t("그 밖에는 편집 요약에 적힌 말(오타 · 출처 · 각주 · 분류 · 보강 · 추가 · 가져오기)을 참고합니다."),
			tool.Baro_t("어디에도 걸리지 않으면 → ") + baro_contrib_esc(tool.Baro_contrib_kind_name("fix")),
		}))

	body += tool.Baro_card(tool.Baro_t("건너뛰는 편집"), tool.Baro_list([]string{
		tool.Baro_t("로그인하지 않은 편집(IP 편집)은 사람 단위 기여로 인정하지 않으므로 건너뜁니다."),
		tool.Baro_t("위키가 스스로 남긴 편집(<code>") + baro_contrib_bot_user + tool.Baro_t("</code> 계정의 기본 문서·틀 설치)도 사람의 기여가 아니므로 건너뜁니다."),
		tool.Baro_t("이미 같은 문서·같은 판으로 등록된 기여 이벤트가 있으면 건너뜁니다."),
		tool.Baro_t("등록된 이벤트는 곧바로 점수가 되지 않습니다. 확정은 ") +
			tool.Baro_t(`<a href="/baro/contrib/score/run">기여 가치 확정</a> 에서 따로 진행합니다.`),
	}))

	body += tool.Baro_card(tool.Baro_t("함께 하는 일"), tool.Baro_list([]string{
		tool.Baro_t("분야가 비어 있는 기여 이벤트를 <b>문서의 지금 분야</b> 로 채웁니다. ") +
			tool.Baro_t("문서에 분야를 나중에 지정해도 예전 기여가 ") +
			tool.Baro_t(`<a href="/baro/rank">분야별 순위</a> 에 잡히게 하기 위해서입니다.`),
		tool.Baro_t("분야별 집계 화면은 이벤트에 굳어 있는 값이 아니라 문서의 지금 분야를 함께 보므로, ") +
			tool.Baro_t("이 보정을 돌리지 않아도 순위에는 곧바로 반영됩니다."),
	}))

	body += tool.Baro_card_ex(
		tool.Baro_t("훑기 실행"),
		tool.Baro_t("마지막으로 훑은 편집 시각 : ")+baro_contrib_esc(cursor)+tool.Baro_t(" (고르는 조건이 아니라 기록입니다)"),
		tool.Baro_form("/baro/contrib/scan",
			tool.Baro_note(tool.Baro_t("한 번 누를 때마다 최대 ")+tool.Baro_comma(baro_contrib_scan_limit)+tool.Baro_t("건을 처리합니다. 남은 건이 있으면 다시 눌러 주세요."))+
				tool.Baro_submit(tool.Baro_t("최대 ")+tool.Baro_comma(baro_contrib_scan_limit)+tool.Baro_t("건 훑기"), "primary"),
		),
		tool.Baro_button("/baro/contrib", tool.Baro_t("기여 기록으로"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("기존 편집 훑기"), body, baro_contrib_menu())
}

// baro_contrib_sort_pairs : {이름, 수} 목록을 수가 큰 순으로 정렬
func baro_contrib_sort_pairs(pairs [][]string) {
	sort.SliceStable(pairs, func(i int, j int) bool {
		return tool.Str_to_int(pairs[i][1]) > tool.Str_to_int(pairs[j][1])
	})
}
