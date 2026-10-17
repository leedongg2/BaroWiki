package route

import (
	"database/sql"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 9장 품질 관리 — 공통 도구 + 9-1 문서 상태
//
//  이 모듈의 모든 비공개 식별자는 baro_quality_ 로 시작한다.
// ============================================================

// ------------------------------------------------------------
//  공통 도구
// ------------------------------------------------------------

// baro_quality_menu : 스킨 상단 메뉴
func baro_quality_menu() [][]any {
	return [][]any{
		{"baro/quality", tool.Baro_t("문서 상태")},
		{"baro/review", tool.Baro_t("검토 큐")},
		{"baro/source", tool.Baro_t("출처 품질")},
		{"baro", "바로위키"},
	}
}

// baro_quality_nav : 품질 관리 화면 공통 탭
func baro_quality_nav(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/quality", tool.Baro_t("문서 상태")},
		{"/baro/review", tool.Baro_t("검토 큐")},
		{"/baro/quality/stable", tool.Baro_t("안정판")},
		{"/baro/source", tool.Baro_t("출처 품질")},
		{"/baro/quality/dup", tool.Baro_t("중복 감지")},
		{"/baro/quality/filter", tool.Baro_t("편집 필터")},
	}, active)
}

// baro_quality_crumb : 이동 경로
func baro_quality_crumb(now string) string {
	return tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/quality", tool.Baro_t("품질 관리")},
		{"", now},
	})
}

// baro_quality_page_num : 쪽 번호 정규화
func baro_quality_page_num(page string) int {
	num := tool.Str_to_int(page)
	if num < 1 {
		num = 1
	}
	if num > 100000 {
		num = 100000
	}

	return num
}

// baro_quality_val : 폼 값 읽기 (앞뒤 공백 제거)
func baro_quality_val(values url.Values, name string) string {
	if values == nil {
		return ""
	}

	return strings.TrimSpace(values.Get(name))
}

// baro_quality_count : 개수 한 개를 읽는다
func baro_quality_count(db *sql.DB, query string, values ...any) int {
	count := 0
	tool.QueryRow_DB(db, query, []any{&count}, values...)

	return count
}

// baro_quality_doc_exists : 문서 본문이 존재하는가
func baro_quality_doc_exists(db *sql.DB, title string) bool {
	found := ""

	return tool.QueryRow_DB(db, "select title from data where title = ?", []any{&found}, title)
}

// baro_quality_doc_data : 문서 본문 (없으면 "")
func baro_quality_doc_data(db *sql.DB, title string) string {
	data := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&data}, title)

	return data
}

// baro_quality_rev : 문서의 최신 판 번호
func baro_quality_rev(db *sql.DB, title string) string {
	return tool.Get_document_revision(db, title)
}

// baro_quality_status_options : 상태 선택 상자 항목 (빈 값 = 지정 해제)
func baro_quality_status_options() [][]string {
	out := [][]string{{"", tool.Baro_t("(지정 없음 / 해제)")}}
	for _, row := range tool.Baro_doc_status_list() {
		out = append(out, []string{row[0], row[1]})
	}

	return out
}

// baro_quality_status_valid : 알려진 상태인가
func baro_quality_status_valid(status string) bool {
	if status == "" {
		return true
	}

	for _, row := range tool.Baro_doc_status_list() {
		if row[0] == status {
			return true
		}
	}

	return false
}

// baro_quality_doc_cell : 표에 넣을 문서 칸 (품질 정보 링크 포함)
func baro_quality_doc_cell(title string) string {
	if title == "" {
		return `<span class="bw_muted">-</span>`
	}

	return tool.Baro_doc_link(title) +
		` <a class="bw_muted" href="/baro/quality/doc/` + tool.Url_parser(title) + tool.Baro_t(`">[품질]</a>`)
}

// baro_quality_yes_no : 참/거짓 배지
func baro_quality_yes_no(check bool, yes string, no string) string {
	if check {
		return tool.Baro_badge(yes, "ok")
	}

	return tool.Baro_badge(no, "muted")
}

// baro_quality_host : URL 에서 도메인만 뽑는다
func baro_quality_host(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ""
	}

	host := strings.ToLower(parsed.Host)
	if index := strings.Index(host, ":"); index >= 0 {
		host = host[:index]
	}

	return strings.TrimPrefix(host, "www.")
}

// baro_quality_url_re : 본문에서 URL 을 뽑는 정규식
var baro_quality_url_re = regexp.MustCompile(`https?://[^\s\[\]|<>"'{}()]+`)

// baro_quality_url_list : 본문에 들어 있는 URL 목록 (중복 제거)
func baro_quality_url_list(data string) []string {
	seen := map[string]bool{}
	out := []string{}

	for _, raw := range baro_quality_url_re.FindAllString(data, -1) {
		raw = strings.TrimRight(raw, ".,;:")
		if raw == "" || seen[raw] {
			continue
		}

		seen[raw] = true
		out = append(out, raw)
	}

	return out
}

// baro_quality_footnotes : 나무마크 각주 [* ... ] 내용을 뽑는다 (중첩 대괄호 고려)
func baro_quality_footnotes(data string) []string {
	out := []string{}
	runes := []rune(data)
	length := len(runes)

	for index := 0; index+1 < length; index++ {
		if runes[index] != '[' || runes[index+1] != '*' {
			continue
		}

		depth := 1
		cursor := index + 2
		for cursor < length {
			if runes[cursor] == '[' {
				depth++
			} else if runes[cursor] == ']' {
				depth--
				if depth == 0 {
					break
				}
			}

			cursor++
		}

		if cursor >= length {
			break
		}

		body := strings.TrimSpace(string(runes[index+2 : cursor]))
		if body != "" {
			out = append(out, body)
		}

		index = cursor
	}

	return out
}

// baro_quality_has_source : 본문에 출처로 볼 만한 표시가 있는가
func baro_quality_has_source(data string) bool {
	if strings.Contains(data, "[*") {
		return true
	}

	return len(baro_quality_url_list(data)) > 0
}

// baro_quality_news_domains : 신문·방송사 도메인
func baro_quality_news_domains() []string {
	return []string{
		"news.naver.com", "news.daum.net", "chosun.com", "donga.com", "joongang.co.kr",
		"hani.co.kr", "khan.co.kr", "yna.co.kr", "yonhapnews.co.kr", "mk.co.kr",
		"hankyung.com", "seoul.co.kr", "kbs.co.kr", "imbc.com", "sbs.co.kr",
		"ytn.co.kr", "jtbc.co.kr", "ohmynews.com", "pressian.com", "segye.com",
		"kmib.co.kr", "munhwa.com", "newsis.com", "nocutnews.co.kr", "edaily.co.kr",
		"mt.co.kr", "asiae.co.kr", "etnews.com", "zdnet.co.kr", "hankookilbo.com",
		"bbc.com", "bbc.co.uk", "cnn.com", "nytimes.com", "reuters.com",
		"apnews.com", "theguardian.com", "nikkei.com", "asahi.com", "washingtonpost.com",
	}
}

// baro_quality_news_words : 본문 표기에서 찾을 신문사 이름
func baro_quality_news_words() []string {
	return []string{
		"신문", "일보", "뉴스", "방송", "통신사", "기자", "연합뉴스", "조선일보", "동아일보",
		"중앙일보", "한겨레", "경향신문", "매일경제", "한국경제", "서울신문", "국민일보",
		"문화일보", "세계일보", "머니투데이", "이데일리", "뉴시스", "노컷뉴스", "오마이뉴스",
	}
}

// baro_quality_book_words : 본문 표기에서 찾을 책 표시
func baro_quality_book_words() []string {
	return []string{
		"ISBN", "isbn", "출판사", "출판", "펴냄", "지음", "옮김", "단행본", "초판",
		"개정판", "쪽.", "쪽,", "p.", "pp.", "저서", "역서", "편저",
	}
}

// baro_quality_paper_words : 논문 표시
func baro_quality_paper_words() []string {
	return []string{"doi.org", "arxiv.org", "riss.kr", "kiss.kstudy.com", "dbpia.co.kr", "scholar.google", "학위논문", "학술지", "논문집"}
}

// baro_quality_cite_kind : 출처 종류 추정 (web / book / news / paper / etc)
func baro_quality_cite_kind(text string, raw string) string {
	lower := strings.ToLower(text + " " + raw)

	for _, word := range baro_quality_paper_words() {
		if strings.Contains(lower, strings.ToLower(word)) {
			return "paper"
		}
	}

	if raw != "" {
		host := baro_quality_host(raw)
		for _, domain := range baro_quality_news_domains() {
			if host == domain || strings.HasSuffix(host, "."+domain) {
				return "news"
			}
		}

		if strings.Contains(host, "news") || strings.Contains(host, "press") || strings.Contains(host, "media") {
			return "news"
		}

		return "web"
	}

	for _, word := range baro_quality_news_words() {
		if strings.Contains(text, word) {
			return "news"
		}
	}

	for _, word := range baro_quality_book_words() {
		if strings.Contains(text, word) {
			return "book"
		}
	}

	return "etc"
}

// baro_quality_cite_kind_name : 출처 종류 이름
//
//	수집기(tool.Baro_sync_citation)가 붙이는 종류는
//	web / news / paper / book / official / archive 여섯 가지다. 모두 한국어 이름을 갖는다.
func baro_quality_cite_kind_name(kind string) string {
	for _, option := range baro_quality_cite_kind_options() {
		if option[0] == kind {
			return option[1]
		}
	}

	return kind
}

// baro_quality_cite_kind_options : 출처 종류 선택 항목
func baro_quality_cite_kind_options() [][]string {
	return [][]string{
		{"web", tool.Baro_t("웹사이트")},
		{"book", tool.Baro_t("책")},
		{"news", tool.Baro_t("신문·방송")},
		{"paper", tool.Baro_t("논문")},
		{"official", tool.Baro_t("공공기관")},
		{"archive", tool.Baro_t("보존본")},
		{"etc", "기타"},
	}
}

// baro_quality_low_trust_domains : 신뢰도가 낮다고 보는 도메인 (도메인, 사유)
func baro_quality_low_trust_domains() [][]string {
	return [][]string{
		{"blog.naver.com", tool.Baro_t("개인 블로그")},
		{"blog.me", tool.Baro_t("개인 블로그")},
		{"tistory.com", tool.Baro_t("개인 블로그")},
		{"brunch.co.kr", tool.Baro_t("개인 블로그")},
		{"blogspot.com", tool.Baro_t("개인 블로그")},
		{"wordpress.com", tool.Baro_t("개인 블로그")},
		{"medium.com", tool.Baro_t("개인 블로그")},
		{"velog.io", tool.Baro_t("개인 블로그")},
		{"namu.wiki", tool.Baro_t("위키 (2차 저작물)")},
		{"wikipedia.org", tool.Baro_t("위키 (2차 저작물)")},
		{"fandom.com", tool.Baro_t("위키 (2차 저작물)")},
		{"wikiwand.com", tool.Baro_t("위키 (2차 저작물)")},
		{"dcinside.com", tool.Baro_t("커뮤니티")},
		{"fmkorea.com", tool.Baro_t("커뮤니티")},
		{"theqoo.net", tool.Baro_t("커뮤니티")},
		{"ruliweb.com", tool.Baro_t("커뮤니티")},
		{"clien.net", tool.Baro_t("커뮤니티")},
		{"instiz.net", tool.Baro_t("커뮤니티")},
		{"pann.nate.com", tool.Baro_t("커뮤니티")},
		{"cafe.naver.com", tool.Baro_t("커뮤니티 카페")},
		{"cafe.daum.net", tool.Baro_t("커뮤니티 카페")},
		{"kin.naver.com", tool.Baro_t("질의응답")},
		{"quora.com", tool.Baro_t("질의응답")},
		{"reddit.com", tool.Baro_t("커뮤니티")},
		{"twitter.com", tool.Baro_t("소셜 미디어")},
		{"x.com", tool.Baro_t("소셜 미디어")},
		{"facebook.com", tool.Baro_t("소셜 미디어")},
		{"instagram.com", tool.Baro_t("소셜 미디어")},
		{"tiktok.com", tool.Baro_t("소셜 미디어")},
		{"youtube.com", tool.Baro_t("동영상 공유")},
		{"youtu.be", tool.Baro_t("동영상 공유")},
	}
}

// baro_quality_low_trust : 신뢰도 낮은 도메인이면 사유를 돌려준다
func baro_quality_low_trust(raw string) string {
	host := baro_quality_host(raw)
	if host == "" {
		return ""
	}

	for _, row := range baro_quality_low_trust_domains() {
		if host == row[0] || strings.HasSuffix(host, "."+row[0]) {
			return row[1]
		}
	}

	return ""
}

// baro_quality_quality_options : 자료 등급 선택 항목
func baro_quality_quality_options() [][]string {
	return [][]string{
		{"", tool.Baro_t("(미지정)")},
		{"primary", tool.Baro_t("1차 자료 (원자료·원문)")},
		{"secondary", tool.Baro_t("2차 자료 (해설·분석)")},
		{"low", tool.Baro_t("신뢰도 낮음")},
	}
}

// baro_quality_quality_name : 자료 등급 이름
func baro_quality_quality_name(quality string) string {
	switch quality {
	case "primary":
		return tool.Baro_t("1차 자료")
	case "secondary":
		return tool.Baro_t("2차 자료")
	case "low":
		return tool.Baro_t("신뢰도 낮음")
	case "":
		return tool.Baro_t("미지정")
	}

	return quality
}

// baro_quality_quality_badge : 자료 등급 배지
func baro_quality_quality_badge(quality string) string {
	switch quality {
	case "primary":
		return tool.Baro_badge(tool.Baro_t("1차 자료"), "ok")
	case "secondary":
		return tool.Baro_badge(tool.Baro_t("2차 자료"), "info")
	case "low":
		return tool.Baro_badge(tool.Baro_t("신뢰도 낮음"), "danger")
	}

	return tool.Baro_badge(tool.Baro_t("미지정"), "muted")
}

// baro_quality_link_badge : 링크 검사 상태 배지
func baro_quality_link_badge(status string) string {
	switch status {
	case "ok":
		return tool.Baro_badge(tool.Baro_t("정상"), "ok")
	case "broken":
		return tool.Baro_badge(tool.Baro_t("깨짐"), "danger")
	case "timeout":
		return tool.Baro_badge(tool.Baro_t("응답 없음"), "warn")
	}

	return tool.Baro_badge(tool.Baro_t("미검사"), "muted")
}

// baro_quality_ext_link : 표에 넣을 바깥 링크 (길면 줄인다)
func baro_quality_ext_link(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return `<span class="bw_muted">-</span>`
	}

	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return tool.HTML_escape(tool.Baro_trunc(raw, 70))
	}

	return `<a href="` + tool.HTML_escape(raw) + `" rel="nofollow noopener" target="_blank">` +
		tool.HTML_escape(tool.Baro_trunc(raw, 70)) + `</a>`
}

// baro_quality_diff_link : 판 사이 차이 보기 링크
func baro_quality_diff_link(db *sql.DB, title string, rev string) string {
	rev_num := tool.Str_to_int(rev)
	if title == "" || rev_num <= 0 {
		return `<span class="bw_muted">-</span>`
	}

	// 판을 지우면 번호에 구멍이 나므로 (예: 1 · 2 · 5) "r - 1" 로 어림잡으면
	// 없는 판을 가리켜 역사 목록으로 튕긴다. 실제로 남아 있는 앞 판을 찾는다.
	previous := history_tool_previous_revision(db, title, rev_num)
	if previous == "" {
		return `<a href="/render/` + strconv.Itoa(rev_num) + `/` + tool.Url_parser(title) + `">r` +
			strconv.Itoa(rev_num) + tool.Baro_t(` (첫 판)</a>`)
	}

	return `<a href="/diff/` + tool.Url_parser(previous) + `/` + strconv.Itoa(rev_num) + `/` + tool.Url_parser(title) + `">r` +
		tool.HTML_escape(previous) + ` → r` + strconv.Itoa(rev_num) + `</a>`
}

// baro_quality_leng_cell : 편집 길이 변화 칸
func baro_quality_leng_cell(leng string) string {
	value := tool.Str_to_int(leng)
	switch {
	case value > 0:
		return `<span style="color:#1a7f4f">+` + tool.Baro_comma(value) + `</span>`
	case value < 0:
		return `<span style="color:#b12b2b">-` + tool.Baro_comma(-value) + `</span>`
	}

	return `<span class="bw_muted">0</span>`
}

// baro_quality_ask_doc : 문서 이름을 받는 안내 화면 본문
func baro_quality_ask_doc(action string, label string) string {
	return tool.Baro_card(label,
		`<form class="bw_form" method="get" action="`+tool.HTML_escape(action)+`">`+
			tool.Baro_input("title", tool.Baro_t("문서 이름"), "", tool.Baro_t("예: 바로위키"), tool.Baro_t("정확한 문서 이름을 입력하세요."))+
			tool.Baro_submit("이동", "primary")+
			`</form>`,
	)
}

// ------------------------------------------------------------
//  9-1 문서 상태 — /baro/quality
// ------------------------------------------------------------

// View_baro_quality_home : 문서 상태 목록 (상태별 탭 + 쪽 넘기기)
func View_baro_quality_home(config tool.Config, page string, status string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_num := baro_quality_page_num(page)
	offset := (page_num - 1) * tool.Baro_page_size

	if !baro_quality_status_valid(status) {
		status = ""
	}

	// 상태별 개수
	count_map := map[string]int{}
	total_marked := 0

	count_rows := tool.Query_DB(db, "select status, count(*) from baro_doc_status group by status")
	defer count_rows.Close()

	for count_rows.Next() {
		key, num := "", 0
		if err := count_rows.Scan(&key, &num); err != nil {
			continue
		}

		count_map[key] = num
		total_marked += num
	}

	doc_total := baro_quality_count(db,
		"select count(*) from data where "+tool.Get_except_document_name_SQL("title"))

	// 지표
	body := baro_quality_crumb(tool.Baro_t("문서 상태"))
	body += tool.Baro_hero(
		tool.Baro_t("문서 상태"),
		tool.Baro_t("문서마다 지금 어떤 손질이 필요한지 한 줄로 알려 주는 표시입니다."),
		tool.Baro_note(tool.Baro_t("상태는 벌점이 아니라 할 일 안내입니다. 상태를 바꾸면 공개 운영 로그에 사유와 함께 남습니다.")),
	)
	body += baro_quality_nav("/baro/quality")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("일반 문서"), tool.Baro_comma(doc_total), tool.Baro_t("이름공간 없는 문서")},
		{tool.Baro_t("상태 지정됨"), tool.Baro_comma(total_marked), tool.Baro_pct_text(total_marked, doc_total)},
		{tool.Baro_t("검토 필요"), tool.Baro_comma(count_map["need_review"]), ""},
		{tool.Baro_t("출처 필요"), tool.Baro_comma(count_map["need_source"]), ""},
		{tool.Baro_t("논쟁 중"), tool.Baro_comma(count_map["disputed"]), ""},
		{tool.Baro_t("우수 문서"), tool.Baro_comma(count_map["featured"]), ""},
	})

	// 상태 탭
	tab_items := [][]string{{"/baro/quality", tool.Baro_t("전체 (") + tool.Baro_itoa(total_marked) + ")"}}
	for _, row := range tool.Baro_doc_status_list() {
		tab_items = append(tab_items, []string{
			"/baro/quality?status=" + tool.Url_parser(row[0]),
			row[1] + " (" + tool.Baro_itoa(count_map[row[0]]) + ")",
		})
	}

	active := "/baro/quality"
	if status != "" {
		active = "/baro/quality?status=" + tool.Url_parser(status)
	}

	body += tool.Baro_tabs(tab_items, active)

	// 목록
	query := "select title, status, reason, actor, date from baro_doc_status"
	args := []any{}
	if status != "" {
		query += " where status = ?"
		args = append(args, status)
	}
	query += " order by date desc limit ?, ?"
	args = append(args, offset, tool.Baro_page_size)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		title, row_status, reason, actor, date := "", "", "", "", ""
		if err := rows.Scan(&title, &row_status, &reason, &actor, &date); err != nil {
			continue
		}

		table_rows = append(table_rows, []string{
			baro_quality_doc_cell(title),
			tool.Baro_status_badge(row_status),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(reason), 60)),
			tool.Baro_user_link(db, actor, config.IP),
			tool.Baro_time_cell(date),
			tool.Baro_button("/baro/quality/set?title="+tool.Url_parser(title), "상태 바꾸기", ""),
		})
	}

	list_html := tool.Baro_table(
		[]string{"문서", "상태", "사유", tool.Baro_t("지정자"), tool.Baro_t("지정 시각"), "관리"},
		table_rows,
		"",
	)

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("이 상태로 지정된 문서가 아직 없습니다. 상태를 지정하면 이 목록에 나타납니다."))
	}

	page_url := "/baro/quality?page={}"
	if status != "" {
		page_url = "/baro/quality?status=" + tool.Url_parser(status) + "&page={}"
	}

	list_html += tool.Baro_paging(page_num, len(table_rows), tool.Baro_page_size, page_url)

	body += tool.Baro_card_ex(
		tool.Baro_t("상태가 지정된 문서"),
		tool.Baro_t("최근에 지정된 순서입니다."),
		list_html,
		tool.Baro_button("/baro/quality/set", tool.Baro_t("문서 상태 지정"), "primary")+" "+
			tool.Baro_button("/baro/review", tool.Baro_t("검토 큐 보기"), "")+" "+
			tool.Baro_button("/baro/source/nosource", tool.Baro_t("출처 없는 문서"), ""),
	)

	// 상태 설명
	legend := []string{}
	for _, row := range tool.Baro_doc_status_list() {
		legend = append(legend, tool.Baro_badge(row[1], row[2])+" "+
			tool.HTML_escape(baro_quality_status_help(row[0])))
	}

	body += tool.Baro_card(tool.Baro_t("상태 안내"), tool.Baro_list(legend))

	return tool.Baro_page(db, config, tool.Baro_t("문서 상태"), body, baro_quality_menu())
}

// baro_quality_status_help : 상태별 설명
func baro_quality_status_help(status string) string {
	switch status {
	case "draft":
		return tool.Baro_t("뼈대만 있는 문서입니다. 이어서 써 주세요.")
	case "need_review":
		return tool.Baro_t("내용이 사실인지 확인이 필요합니다.")
	case "need_source":
		return tool.Baro_t("주장에 붙일 출처가 부족합니다.")
	case "need_expand":
		return tool.Baro_t("분량이나 항목이 모자랍니다.")
	case "disputed":
		return tool.Baro_t("서술을 두고 논쟁이 진행 중입니다. 토론을 먼저 확인하세요.")
	case "reviewed":
		return tool.Baro_t("검토를 마쳤습니다.")
	case "featured":
		return tool.Baro_t("출처와 서술이 모범적인 문서입니다.")
	case "quarantined":
		return tool.Baro_t("문제 제기로 임시 격리 중입니다. 삭제가 아니라 보존 상태입니다.")
	case "archived":
		return tool.Baro_t("더 이상 갱신하지 않고 보존만 하는 문서입니다.")
	}

	return ""
}

// View_baro_quality_status_set : 문서 상태 지정 (권한 review)
func View_baro_quality_status_set(config tool.Config, title string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/quality/set"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "review") {
		return tool.Baro_deny(db, config, "문서 상태는 검토 권한이 있는 사용자만 지정할 수 있습니다.")
	}

	title = strings.TrimSpace(title)
	message := ""

	if values != nil {
		if tool.Baro_read_only(db) {
			message = tool.Baro_alert("danger", "지금은 읽기 전용 상태입니다. "+tool.HTML_escape(tool.Baro_read_only_reason(db)))
		} else {
			post_title := tool.Namespace_normalize(baro_quality_val(values, "title"))
			post_status := baro_quality_val(values, "status")
			post_reason := baro_quality_val(values, "reason")

			switch {
			case post_title == "":
				message = tool.Baro_alert("danger", "문서 이름을 입력하세요.")
			case !baro_quality_status_valid(post_status):
				message = tool.Baro_alert("danger", "알 수 없는 상태입니다.")
			case post_reason == "":
				message = tool.Baro_alert("danger", "사유는 반드시 남겨야 합니다. 상태 표시는 다른 기여자에게 보내는 안내이기 때문입니다.")
			default:
				before := tool.Baro_doc_status_get(db, post_title)

				tool.Baro_doc_status_set(db, post_title, post_status, tool.Baro_trunc(post_reason, 500), config.IP)

				kind := "doc_status"
				after_name := tool.Baro_doc_status_name(post_status)
				if post_status == "" {
					kind = "doc_status_clear"
					after_name = "지정 해제"
				}

				tool.Baro_public_log(db, kind, config.IP, post_title, tool.Baro_trunc(post_reason, 300), after_name)
				tool.Baro_audit(db, config, kind, post_title, before, post_status, post_reason)

				return tool.Get_redirect("/baro/quality/doc/" + tool.Url_parser(post_title))
			}
		}

		title = tool.Namespace_normalize(baro_quality_val(values, "title"))
	}

	current := ""
	current_reason := ""
	if title != "" {
		current = tool.Baro_doc_status_get(db, title)
		tool.QueryRow_DB(db, "select reason from baro_doc_status where title = ?", []any{&current_reason}, title)
	}

	form := tool.Baro_input("title", "문서 이름", title, "예: 바로위키", "이름공간 접두사를 포함해 정확히 입력하세요.")
	form += tool.Baro_select("status", "상태", baro_quality_status_options(), current, "지정 없음을 고르면 기존 상태를 해제합니다.")
	form += tool.Baro_textarea("reason", "사유", current_reason, 4, "무엇이 부족한지 구체적으로 적어 주세요.", "이 사유는 공개 운영 로그에 남습니다. 개인정보를 적지 마세요.")
	form += tool.Baro_submit("상태 저장", "primary")

	body := baro_quality_crumb("문서 상태 지정")
	body += tool.Baro_hero("문서 상태 지정", "문서에 지금 필요한 손질을 표시합니다.", "")
	body += baro_quality_nav("/baro/quality")
	body += message

	if title != "" && !baro_quality_doc_exists(db, title) {
		body += tool.Baro_alert("warn", "본문이 아직 없는 문서입니다. 상태는 지정할 수 있지만, 문서가 만들어질 때까지는 목록에서만 보입니다.")
	}

	body += tool.Baro_card_ex(
		"상태 지정",
		"검토 권한이 있는 사용자만 사용할 수 있습니다.",
		tool.Baro_form("/baro/quality/set", form),
		tool.Baro_button("/baro/quality", "문서 상태 목록", ""),
	)

	help := []string{}
	for _, row := range tool.Baro_doc_status_list() {
		help = append(help, tool.Baro_badge(row[1], row[2])+" "+tool.HTML_escape(baro_quality_status_help(row[0])))
	}
	body += tool.Baro_card("어떤 상태를 골라야 하나요?", tool.Baro_list(help))

	return tool.Baro_page(db, config, "문서 상태 지정", body, baro_quality_menu())
}

// View_baro_quality_doc : 문서 한 개의 품질 정보 모아 보기
func View_baro_quality_doc(config tool.Config, title string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	title = strings.TrimSpace(title)

	if title == "" {
		body := baro_quality_crumb("문서 품질 정보")
		body += tool.Baro_hero("문서 품질 정보", tool.Baro_t("문서 하나의 상태·출처·검토·중복을 한 화면에서 봅니다."), "")
		body += baro_quality_nav("/baro/quality")
		body += tool.Baro_alert("info", tool.Baro_t("문서 이름을 입력하면 그 문서의 품질 정보를 보여 줍니다."))
		body += baro_quality_ask_doc("/baro/quality/doc_go", tool.Baro_t("문서 고르기"))

		return tool.Baro_page(db, config, "문서 품질 정보", body, baro_quality_menu())
	}

	exists := baro_quality_doc_exists(db, title)
	status := tool.Baro_doc_status_get(db, title)
	rev := baro_quality_rev(db, title)

	status_reason, status_actor, status_date := "", "", ""
	tool.QueryRow_DB(db,
		"select reason, actor, date from baro_doc_status where title = ?",
		[]any{&status_reason, &status_actor, &status_date}, title)

	// 출처
	cite_total := baro_quality_count(db, "select count(*) from baro_citation where title = ?", title)
	cite_broken := baro_quality_count(db, "select count(*) from baro_citation where title = ? and status = 'broken'", title)
	cite_timeout := baro_quality_count(db, "select count(*) from baro_citation where title = ? and status = 'timeout'", title)
	cite_low := baro_quality_count(db, "select count(*) from baro_citation where title = ? and quality = 'low'", title)
	cite_archive := baro_quality_count(db, "select count(*) from baro_citation where title = ? and archive_url != ''", title)

	// 안정판
	stable_rev, stable_actor, stable_date := "", "", ""
	has_stable := tool.QueryRow_DB(db,
		"select rev, actor, date from baro_stable where title = ?",
		[]any{&stable_rev, &stable_actor, &stable_date}, title)

	stable_cell := tool.Baro_badge(tool.Baro_t("지정 안 됨"), "muted")
	if has_stable {
		if stable_rev == rev {
			stable_cell = tool.Baro_badge("r"+tool.HTML_escape(stable_rev)+tool.Baro_t(" (최신과 같음)"), "ok")
		} else {
			stable_cell = tool.Baro_badge("r"+tool.HTML_escape(stable_rev)+tool.Baro_t(" (최신 r")+tool.HTML_escape(rev)+tool.Baro_t("과 다름)"), "warn")
		}
	}

	quarantined := tool.Baro_doc_quarantined(db, title)
	field := tool.Baro_doc_field(db, title)
	field_name := tool.Baro_t("미지정")
	if field != "" {
		field_name = tool.Baro_field_name(db, field)
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/quality", tool.Baro_t("품질 관리")},
		{"", tool.Namespace_display(title)},
	})

	body += tool.Baro_hero(
		tool.Namespace_display(title)+tool.Baro_t(" 품질 정보"),
		tool.Baro_t("상태 · 출처 · 검토 이력 · 중복 후보 · 안정판 · 격리 여부"),
		tool.Baro_button("/w/" + tool.Doc_url(title), "문서 보기", "primary")+" "+
			tool.Baro_button("/history/"+tool.Url_parser(title), "역사", "")+" "+
			tool.Baro_button("/baro/quality/set?title="+tool.Url_parser(title), "상태 바꾸기", "")+" "+
			tool.Baro_button("/baro/quality/stable/set?title="+tool.Url_parser(title), "안정판 지정", ""),
	)

	body += baro_quality_nav("/baro/quality")

	if !exists {
		body += tool.Baro_alert("warn", tool.Baro_t("이 이름의 문서 본문이 없습니다. 삭제되었거나 아직 만들어지지 않았습니다."))
	}
	if quarantined {
		body += tool.Baro_alert("danger", tool.Baro_t("이 문서는 임시 격리 중입니다. 삭제된 것이 아니라 검토를 기다리는 상태입니다."))
	}

	body += tool.Baro_kv_grid([][]string{
		{tool.Baro_t("현재 상태"), tool.Baro_status_badge(status)},
		{tool.Baro_t("상태 사유"), tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(status_reason), 120))},
		{tool.Baro_t("지정자"), tool.Baro_user_link(db, status_actor, config.IP)},
		{tool.Baro_t("지정 시각"), tool.Baro_time_cell(status_date)},
		{tool.Baro_t("최신 판"), "r" + tool.HTML_escape(rev)},
		{tool.Baro_t("안정판"), stable_cell},
		{tool.Baro_t("격리 여부"), baro_quality_yes_no(quarantined, tool.Baro_t("격리 중"), tool.Baro_t("정상"))},
		{"분야", tool.HTML_escape(field_name)},
	})

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("등록된 출처"), tool.Baro_comma(cite_total), ""},
		{tool.Baro_t("깨진 링크"), tool.Baro_comma(cite_broken), ""},
		{tool.Baro_t("응답 없음"), tool.Baro_comma(cite_timeout), ""},
		{tool.Baro_t("신뢰도 낮음"), tool.Baro_comma(cite_low), ""},
		{tool.Baro_t("보존 링크"), tool.Baro_comma(cite_archive), ""},
	})

	// 출처 목록
	cite_rows := [][]string{}
	cites := tool.Query_DB(db,
		"select id, kind, url, publisher, quality, status, archive_url from baro_citation where title = ? order by id + 0 asc limit 50",
		title)
	defer cites.Close()

	for cites.Next() {
		id, kind, raw, publisher, quality, link_status, archive := "", "", "", "", "", "", ""
		if err := cites.Scan(&id, &kind, &raw, &publisher, &quality, &link_status, &archive); err != nil {
			continue
		}

		archive_cell := `<span class="bw_muted">-</span>`
		if archive != "" {
			archive_cell = baro_quality_ext_link(archive)
		}

		cite_rows = append(cite_rows, []string{
			tool.HTML_escape(baro_quality_cite_kind_name(kind)),
			baro_quality_ext_link(raw),
			tool.HTML_escape(tool.Baro_trunc(publisher, 30)),
			baro_quality_quality_badge(quality),
			baro_quality_link_badge(link_status),
			archive_cell,
			tool.Baro_button("/baro/source/edit/"+tool.Url_parser(id), "손질", ""),
		})
	}

	cite_html := tool.Baro_table(
		[]string{"종류", tool.Baro_t("주소"), tool.Baro_t("표기·발행처"), tool.Baro_t("등급"), tool.Baro_t("링크"), tool.Baro_t("보존본"), "관리"},
		cite_rows, "")
	if len(cite_rows) == 0 {
		cite_html = tool.Baro_empty(tool.Baro_t("등록된 출처가 없습니다. 출처 수집을 실행하면 본문에서 각주와 URL 을 뽑아 등록합니다."))
	}

	body += tool.Baro_card_ex("출처", tool.Baro_t("본문에서 뽑아 등록된 출처입니다."), cite_html,
		tool.Baro_button("/baro/source/scan?title="+tool.Url_parser(title), "이 문서 출처 다시 수집", "")+" "+
			tool.Baro_button("/baro/source?title="+tool.Url_parser(title), "출처 전체 보기", ""))

	// 검토 이력
	review_rows := [][]string{}
	reviews := tool.Query_DB(db,
		"select id, rev, user, kind, status, reviewer, review_date, risk, date from baro_review where title = ? order by id + 0 desc limit 20",
		title)
	defer reviews.Close()

	for reviews.Next() {
		id, row_rev, user, kind, row_status, reviewer, review_date, risk, date := "", "", "", "", "", "", "", "", ""
		if err := reviews.Scan(&id, &row_rev, &user, &kind, &row_status, &reviewer, &review_date, &risk, &date); err != nil {
			continue
		}

		rev_cell := "r" + tool.HTML_escape(row_rev)
		if kind == "dup" {
			rev_cell = `<a href="/baro/quality/dup/view/` + tool.Url_parser(row_rev) + `">중복 #` + tool.HTML_escape(row_rev) + `</a>`
		}

		review_rows = append(review_rows, []string{
			`<a href="/baro/review/view/` + tool.Url_parser(id) + `">#` + tool.HTML_escape(id) + `</a>`,
			tool.HTML_escape(baro_quality_review_kind_name(kind)),
			rev_cell,
			tool.Baro_user_link(db, user, config.IP),
			baro_quality_review_status_badge(row_status),
			tool.HTML_escape(risk),
			tool.Baro_user_link(db, reviewer, config.IP),
			tool.Baro_time_cell(date),
		})
	}

	review_html := tool.Baro_table(
		[]string{"번호", "종류", "판", tool.Baro_t("편집자"), "상태", tool.Baro_t("위험도"), tool.Baro_t("검토자"), "접수"},
		review_rows, "")
	if len(review_rows) == 0 {
		review_html = tool.Baro_empty(tool.Baro_t("이 문서에 대한 검토 기록이 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("검토 이력"), review_html)

	// 중복 후보
	dup_rows := [][]string{}
	dups := tool.Query_DB(db,
		"select id, title, target, kind, ratio, status, date from baro_dup where title = ? or target = ? order by id + 0 desc limit 20",
		title, title)
	defer dups.Close()

	for dups.Next() {
		id, dup_title, target, kind, ratio, dup_status, date := "", "", "", "", "", "", ""
		if err := dups.Scan(&id, &dup_title, &target, &kind, &ratio, &dup_status, &date); err != nil {
			continue
		}

		dup_rows = append(dup_rows, []string{
			`<a href="/baro/quality/dup/view/` + tool.Url_parser(id) + `">#` + tool.HTML_escape(id) + `</a>`,
			tool.HTML_escape(baro_quality_dup_kind_name(kind)),
			baro_quality_doc_cell(dup_title),
			baro_quality_doc_cell(target),
			tool.HTML_escape(ratio) + "%",
			baro_quality_dup_status_badge(dup_status),
			tool.Baro_time_cell(date),
		})
	}

	dup_html := tool.Baro_table(
		[]string{"번호", "종류", "문서", tool.Baro_t("상대 문서"), tool.Baro_t("겹침"), "상태", tool.Baro_t("감지")},
		dup_rows, "")
	if len(dup_rows) == 0 {
		dup_html = tool.Baro_empty(tool.Baro_t("중복 후보가 없습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("중복 후보"), tool.Baro_t("겹치는 문단이 발견된 기록입니다. 자동으로 지우지 않고 검토 큐로만 보냅니다."), dup_html,
		tool.Baro_button("/baro/quality/dup", tool.Baro_t("중복 감지 화면"), ""))

	// 편집 필터 기록
	filter_rows := [][]string{}
	logs := tool.Query_DB(db,
		"select id, filter_id, user, action, detail, date from baro_filter_log where title = ? order by id + 0 desc limit 20",
		title)
	defer logs.Close()

	for logs.Next() {
		id, filter_id, user, action, detail, date := "", "", "", "", "", ""
		if err := logs.Scan(&id, &filter_id, &user, &action, &detail, &date); err != nil {
			continue
		}

		filter_rows = append(filter_rows, []string{
			"#" + tool.HTML_escape(id),
			tool.HTML_escape(baro_quality_filter_name(db, filter_id)),
			tool.Baro_user_link(db, user, config.IP),
			tool.HTML_escape(baro_quality_filter_action_name(action)),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(detail), 60)),
			tool.Baro_time_cell(date),
		})
	}

	filter_html := tool.Baro_table(
		[]string{"번호", tool.Baro_t("규칙"), tool.Baro_t("사용자"), tool.Baro_t("조치"), tool.Baro_t("내용"), "시각"},
		filter_rows, "")
	if len(filter_rows) == 0 {
		filter_html = tool.Baro_empty(tool.Baro_t("편집 필터에 걸린 기록이 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("편집 필터 기록"), filter_html)

	// 최근 편집
	history_rows := [][]string{}
	histories := tool.Query_DB(db,
		"select id, date, ip, send, leng from history where title = ? order by id + 0 desc limit 10",
		title)
	defer histories.Close()

	for histories.Next() {
		id, date, ip, send, leng := "", "", "", "", ""
		if err := histories.Scan(&id, &date, &ip, &send, &leng); err != nil {
			continue
		}

		history_rows = append(history_rows, []string{
			baro_quality_diff_link(db, title, id),
			tool.Baro_user_link(db, ip, config.IP),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(send), 50)),
			baro_quality_leng_cell(leng),
			tool.Baro_time_cell(date),
		})
	}

	history_html := tool.Baro_table(
		[]string{"판", tool.Baro_t("편집자"), tool.Baro_t("요약"), "길이 변화", "시각"},
		history_rows, "")
	if len(history_rows) == 0 {
		history_html = tool.Baro_empty(tool.Baro_t("편집 기록이 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("최근 편집"), history_html)

	return tool.Baro_page(db, config, tool.Namespace_display(title)+tool.Baro_t(" 품질 정보"), body, baro_quality_menu())
}
