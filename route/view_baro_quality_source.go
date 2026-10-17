package route

import (
	"database/sql"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"opennamu/route/tool"
)

// ============================================================
//  9-4 출처 품질 도구 — /baro/source
//
//  출처는 "있다 / 없다" 로만 보지 않는다.
//  종류 · 신뢰도 · 1차/2차 · 링크 생존 여부까지 따로 본다.
// ============================================================

// baro_quality_year_re : 본문 표기에서 발행 연도를 찾는다
var baro_quality_year_re = regexp.MustCompile(`(19|20)\d{2}`)

// baro_quality_source_unchecked_sql : 아직 검사하지 않은 링크를 고르는 조건
//
//	수집기(tool.Baro_sync_citation)는 새로 모은 출처의 status 를 'new' 로 넣고,
//	예전 기록은 '' 로 비어 있다. 둘 다 "아직 검사하지 않음"으로 본다.
const baro_quality_source_unchecked_sql = "(status = '' or status = 'new')"

// baro_quality_source_checked_sql : 이미 검사를 마친 링크를 고르는 조건
const baro_quality_source_checked_sql = "(status != '' and status != 'new')"

// baro_quality_source_nav : 출처 화면 안쪽 탭
func baro_quality_source_nav(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/source", tool.Baro_t("출처 목록")},
		{"/baro/source/nosource", tool.Baro_t("출처 없는 문서")},
		{"/baro/source/check", tool.Baro_t("링크 검사")},
		{"/baro/source/overused", tool.Baro_t("과다 사용")},
		{"/baro/source/lowtrust", tool.Baro_t("신뢰도 낮은 출처")},
		{"/baro/source/format", tool.Baro_t("인용 형식")},
		{"/baro/source/scan", "출처 수집"},
	}, active)
}

// baro_quality_source_stats : 출처 지표 줄
func baro_quality_source_stats(db *sql.DB) string {
	total := baro_quality_count(db, "select count(*) from baro_citation")
	docs := baro_quality_count(db, "select count(distinct title) from baro_citation")
	broken := baro_quality_count(db, "select count(*) from baro_citation where status = 'broken'")
	timeout := baro_quality_count(db, "select count(*) from baro_citation where status = 'timeout'")
	unchecked := baro_quality_count(db, "select count(*) from baro_citation where url != '' and "+baro_quality_source_unchecked_sql)
	low := baro_quality_count(db, "select count(*) from baro_citation where quality = 'low'")
	archive := baro_quality_count(db, "select count(*) from baro_citation where archive_url != ''")

	return tool.Baro_stat_row([][]string{
		{tool.Baro_t("등록된 출처"), tool.Baro_comma(total), ""},
		{tool.Baro_t("출처가 있는 문서"), tool.Baro_comma(docs), ""},
		{tool.Baro_t("깨진 링크"), tool.Baro_comma(broken), ""},
		{tool.Baro_t("응답 없음"), tool.Baro_comma(timeout), ""},
		{tool.Baro_t("미검사 링크"), tool.Baro_comma(unchecked), ""},
		{tool.Baro_t("신뢰도 낮음"), tool.Baro_comma(low), ""},
		{tool.Baro_t("보존 링크 있음"), tool.Baro_comma(archive), ""},
	})
}

// baro_quality_source_extract : 문서 본문에서 출처 후보를 뽑는다
//
//	돌려주는 값 : {종류, URL, 원문 표기, 발행 연도}
func baro_quality_source_extract(data string) [][]string {
	out := [][]string{}
	seen := map[string]bool{}

	add := func(kind string, raw string, text string) {
		key := raw
		if key == "" {
			key = "text:" + tool.Baro_norm(text)
		}

		if key == "" || seen[key] {
			return
		}

		seen[key] = true

		year := ""
		if match := baro_quality_year_re.FindString(text + " " + raw); match != "" {
			year = match
		}

		out = append(out, []string{kind, raw, tool.Baro_trunc(tool.Baro_one_line(text), 300), year})
	}

	// 1. 각주
	for _, note := range baro_quality_footnotes(data) {
		urls := baro_quality_url_list(note)
		raw := ""
		if len(urls) > 0 {
			raw = urls[0]
		}

		add(baro_quality_cite_kind(note, raw), raw, note)

		// 각주 안에 링크가 여러 개면 나머지도 등록한다
		for index := 1; index < len(urls); index++ {
			add(baro_quality_cite_kind(note, urls[index]), urls[index], note)
		}
	}

	// 2. 각주 밖의 맨 URL
	for _, raw := range baro_quality_url_list(data) {
		add(baro_quality_cite_kind("", raw), raw, raw)
	}

	if len(out) > 200 {
		out = out[:200]
	}

	return out
}

// baro_quality_source_key : 다시 수집할 때 같은 출처인지 알아보는 열쇠
//
//	주소가 있으면 주소로, 없으면 원문 표기를 정규화해 견준다.
func baro_quality_source_key(raw string, text string) string {
	if strings.TrimSpace(raw) != "" {
		return "url:" + strings.TrimSpace(raw)
	}

	return "text:" + tool.Baro_norm(text)
}

// baro_quality_source_kept : 다시 수집할 때 지키는 값
//
//	kind · author · publisher · pub_date · access_date · archive_url · quality · status
type baro_quality_source_kept struct {
	Kind, Author, Publisher, PubDate, AccessDate, Archive, Quality, Status string
}

// baro_quality_source_scan_doc : 문서 한 개의 출처를 다시 등록한다 (등록 수를 돌려준다)
//
//	0장 원칙 2(삭제보다 보존)에 따라, 사람이 손질해 둔 값은 지우지 않고 그대로 옮긴다.
//	같은 출처인지는 주소(없으면 원문 표기)로 알아본다.
func baro_quality_source_scan_doc(db *sql.DB, title string, data string) int {
	items := baro_quality_source_extract(data)
	rev := baro_quality_rev(db, title)
	now := tool.Baro_now()

	// 1. 지금 등록된 손질 값을 먼저 챙긴다
	old := map[string]baro_quality_source_kept{}

	old_rows := tool.Query_DB(db,
		"select kind, url, author, publisher, pub_date, access_date, archive_url, quality, status from baro_citation where title = ?",
		title)
	defer old_rows.Close()

	for old_rows.Next() {
		row := baro_quality_source_kept{}
		raw := ""

		if err := old_rows.Scan(&row.Kind, &raw, &row.Author, &row.Publisher,
			&row.PubDate, &row.AccessDate, &row.Archive, &row.Quality, &row.Status); err != nil {
			continue
		}

		key := baro_quality_source_key(raw, row.Publisher)
		if _, found := old[key]; !found {
			old[key] = row
		}
	}

	tool.Exec_DB(db, "delete from baro_citation where title = ?", title)

	// 2. 새로 뽑은 출처를 넣되, 예전에 있던 것이면 손질 값을 되살린다
	keep := func(before string, now_value string) string {
		if strings.TrimSpace(before) != "" {
			return before
		}

		return now_value
	}

	for _, item := range items {
		kind, raw, text, year := item[0], item[1], item[2], item[3]

		quality := ""
		if reason := baro_quality_low_trust(raw); reason != "" {
			quality = "low"
		}

		author, publisher, pub_date, access_date, archive, status := "", text, year, "", "", ""

		if before, found := old[baro_quality_source_key(raw, text)]; found {
			kind = keep(before.Kind, kind)
			author = before.Author
			publisher = keep(before.Publisher, publisher)
			pub_date = keep(before.PubDate, pub_date)
			access_date = before.AccessDate
			archive = before.Archive
			quality = keep(before.Quality, quality)
			status = before.Status
		}

		tool.Exec_DB(db,
			"insert into baro_citation (id, title, rev, kind, url, author, publisher, pub_date, access_date, archive_url, quality, status, date) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			tool.Baro_next_id(db, "baro_citation"), title, rev, kind, raw,
			author, publisher, pub_date, access_date, archive, quality, status, now)
	}

	tool.Baro_doc_meta_set(db, title, "citation_scan", now)

	return len(items)
}

// baro_quality_source_scan_run : 여러 문서의 출처를 한 번에 수집 (한 번에 최대 300문서)
func baro_quality_source_scan_run(db *sql.DB, limit int) (int, int) {
	if limit < 1 {
		limit = 100
	}
	if limit > 300 {
		limit = 300
	}

	offset := tool.Baro_get_int(db, "quality_source_offset", 0)
	if offset < 0 {
		offset = 0
	}

	type doc_row struct {
		Title, Data string
	}

	list := []doc_row{}

	rows := tool.Query_DB(db,
		"select title, data from data where "+tool.Get_except_document_name_SQL("title")+" order by title limit ?, ?",
		offset, limit)
	defer rows.Close()

	for rows.Next() {
		row := doc_row{}
		if err := rows.Scan(&row.Title, &row.Data); err != nil {
			continue
		}

		list = append(list, row)
	}

	cites := 0
	for _, row := range list {
		cites += baro_quality_source_scan_doc(db, row.Title, row.Data)
	}

	// 다음 실행은 그다음 묶음부터. 끝까지 갔으면 처음으로 되돌린다.
	if len(list) < limit {
		tool.Baro_put(db, "quality_source_offset", "0")
	} else {
		tool.Baro_put(db, "quality_source_offset", tool.Baro_itoa(offset+limit))
	}

	if cites > 0 {
		tool.Baro_metric_bump(db, "citation_scanned", tool.Baro_today(), cites)
	}

	return len(list), cites
}

// baro_quality_source_check_run : 링크를 실제로 HEAD 요청해 생사를 확인한다
//
//	한 번에 최대 30개 · 요청당 5초 제한
func baro_quality_source_check_run(db *sql.DB, mode string) (int, int, int, int) {
	type cite_row struct {
		ID, URL string
	}

	list := []cite_row{}

	query := "select id, url from baro_citation where url != '' and " + baro_quality_source_unchecked_sql + " order by id + 0 asc limit 30"
	if mode == "recheck" {
		query = "select id, url from baro_citation where url != '' order by access_date asc, id + 0 asc limit 30"
	}

	rows := tool.Query_DB(db, query)
	defer rows.Close()

	for rows.Next() {
		row := cite_row{}
		if err := rows.Scan(&row.ID, &row.URL); err != nil {
			continue
		}

		list = append(list, row)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	now := tool.Baro_now()

	ok_count, broken_count, timeout_count := 0, 0, 0

	for _, row := range list {
		status := baro_quality_source_head(client, row.URL)

		switch status {
		case "ok":
			ok_count++
		case "timeout":
			timeout_count++
		default:
			broken_count++
		}

		tool.Exec_DB(db,
			"update baro_citation set status = ?, access_date = ? where id = ?",
			status, now, row.ID)
	}

	return len(list), ok_count, broken_count, timeout_count
}

// baro_quality_source_head : 링크 한 개의 상태 (ok / broken / timeout)
func baro_quality_source_head(client *http.Client, raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "broken"
	}

	try := func(method string) (int, error) {
		request, err := http.NewRequest(method, raw, nil)
		if err != nil {
			return 0, err
		}

		request.Header.Set("User-Agent", "BaroWiki-LinkCheck/1.0")

		response, err := client.Do(request)
		if err != nil {
			return 0, err
		}
		defer response.Body.Close()

		return response.StatusCode, nil
	}

	code, err := try("HEAD")
	if err != nil {
		if baro_quality_is_timeout(err) {
			return "timeout"
		}

		return "broken"
	}

	// HEAD 를 막아 둔 서버가 많아 그런 경우에만 GET 으로 한 번 더 본다
	if code == http.StatusMethodNotAllowed || code == http.StatusNotImplemented || code == http.StatusForbidden {
		if get_code, get_err := try("GET"); get_err == nil {
			code = get_code
		} else if baro_quality_is_timeout(get_err) {
			return "timeout"
		}
	}

	if code >= 200 && code < 400 {
		return "ok"
	}

	return "broken"
}

// baro_quality_is_timeout : 시간 초과 오류인가
func baro_quality_is_timeout(err error) bool {
	if err == nil {
		return false
	}

	text := strings.ToLower(err.Error())

	return strings.Contains(text, "timeout") ||
		strings.Contains(text, "deadline exceeded") ||
		strings.Contains(text, "timed out")
}

// baro_quality_cite_format_problem : 인용 형식이 표준에 못 미치면 그 이유
func baro_quality_cite_format_problem(kind string, raw string, author string, publisher string, pub_date string, access_date string) string {
	missing := []string{}

	switch kind {
	case "book":
		if author == "" {
			missing = append(missing, tool.Baro_t("저자"))
		}
		if publisher == "" {
			missing = append(missing, tool.Baro_t("출판사·표기"))
		}
		if pub_date == "" {
			missing = append(missing, tool.Baro_t("발행 연도"))
		}
	case "paper":
		if author == "" {
			missing = append(missing, tool.Baro_t("저자"))
		}
		if publisher == "" {
			missing = append(missing, tool.Baro_t("학술지·표기"))
		}
		if pub_date == "" {
			missing = append(missing, tool.Baro_t("발행 연도"))
		}
	case "news":
		if publisher == "" {
			missing = append(missing, tool.Baro_t("신문사·표기"))
		}
		if pub_date == "" {
			missing = append(missing, tool.Baro_t("보도 날짜"))
		}
	case "web":
		if raw == "" {
			missing = append(missing, tool.Baro_t("주소"))
		}
		if publisher == "" {
			missing = append(missing, tool.Baro_t("사이트 이름·표기"))
		}
		if access_date == "" {
			missing = append(missing, tool.Baro_t("확인 날짜"))
		}
	default:
		if publisher == "" && raw == "" {
			missing = append(missing, tool.Baro_t("출처 표기"))
		}
	}

	if len(missing) == 0 {
		return ""
	}

	return strings.Join(missing, " · ") + " 없음"
}

// baro_quality_cite_format_guide : 종류별 표준 인용 형식
func baro_quality_cite_format_guide() [][]string {
	return [][]string{
		{tool.Baro_t("책"), tool.Baro_t("저자, 《책 제목》, 출판사, 발행연도, 000쪽."), tool.Baro_t("홍길동, 《바로위키 이야기》, 바로출판, 2024, 42쪽.")},
		{tool.Baro_t("논문"), tool.Baro_t("저자, 〈논문 제목〉, 《학술지》 권(호), 발행연도, 000쪽."), tool.Baro_t("김철수, 〈위키 품질 관리 연구〉, 《정보학보》 12(3), 2023, 15쪽.")},
		{tool.Baro_t("신문·방송"), tool.Baro_t("기자, 〈기사 제목〉, 《매체 이름》, 보도일자, 주소."), tool.Baro_t("이영희, 〈위키 이용 급증〉, 《바로일보》, 2024-05-01.")},
		{tool.Baro_t("웹사이트"), tool.Baro_t("사이트 이름, 〈문서 제목〉, 주소, 확인 날짜."), tool.Baro_t("바로위키, 〈편집 지침〉, https://example.org/guide, 2024-05-01 확인.")},
	}
}

// ------------------------------------------------------------
//  /baro/source : 출처 목록과 품질 점검 결과
// ------------------------------------------------------------

// View_baro_quality_source_list : 출처 목록
func View_baro_quality_source_list(config tool.Config, page string, title string, kind string, status string, quality string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_num := baro_quality_page_num(page)
	offset := (page_num - 1) * tool.Baro_page_size

	title = strings.TrimSpace(title)

	where := []string{}
	args := []any{}

	if title != "" {
		where = append(where, "title = ?")
		args = append(args, tool.Namespace_normalize(title))
	}
	if kind != "" {
		where = append(where, "kind = ?")
		args = append(args, kind)
	}
	if status != "" {
		where = append(where, "status = ?")
		args = append(args, status)
	}
	if quality != "" {
		where = append(where, "quality = ?")
		args = append(args, quality)
	}

	query := "select id, title, kind, url, author, publisher, pub_date, access_date, archive_url, quality, status from baro_citation"
	if len(where) > 0 {
		query += " where " + strings.Join(where, " and ")
	}
	query += " order by id + 0 desc limit ?, ?"
	args = append(args, offset, tool.Baro_page_size)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	table_rows := [][]string{}
	format_bad := 0

	for rows.Next() {
		id, row_title, row_kind, raw := "", "", "", ""
		author, publisher, pub_date, access_date, archive := "", "", "", "", ""
		row_quality, row_status := "", ""

		if err := rows.Scan(&id, &row_title, &row_kind, &raw, &author, &publisher, &pub_date, &access_date, &archive, &row_quality, &row_status); err != nil {
			continue
		}

		problem := baro_quality_cite_format_problem(row_kind, raw, author, publisher, pub_date, access_date)
		format_cell := tool.Baro_badge(tool.Baro_t("형식 충족"), "ok")
		if problem != "" {
			format_bad++
			format_cell = tool.Baro_badge(problem, "warn")
		}

		archive_cell := `<span class="bw_muted">-</span>`
		if archive != "" {
			archive_cell = baro_quality_ext_link(archive)
		}

		source_cell := baro_quality_ext_link(raw)
		if raw == "" {
			source_cell = tool.HTML_escape(tool.Baro_trunc(publisher, 70))
		}

		table_rows = append(table_rows, []string{
			baro_quality_doc_cell(row_title),
			tool.HTML_escape(baro_quality_cite_kind_name(row_kind)),
			source_cell,
			baro_quality_quality_badge(row_quality),
			baro_quality_link_badge(row_status),
			format_cell,
			archive_cell,
			tool.Baro_button("/baro/source/edit/"+tool.Url_parser(id), "손질", ""),
		})
	}

	list_html := tool.Baro_table(
		[]string{"문서", "종류", "출처", tool.Baro_t("등급"), tool.Baro_t("링크"), tool.Baro_t("형식"), tool.Baro_t("보존본"), "관리"},
		table_rows, "")

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("조건에 맞는 출처가 없습니다. [출처 수집] 을 실행하면 본문에서 각주와 URL 을 뽑아 등록합니다."))
	}

	page_url := "/baro/source?title=" + tool.Url_parser(title) +
		"&kind=" + tool.Url_parser(kind) +
		"&status=" + tool.Url_parser(status) +
		"&quality=" + tool.Url_parser(quality) + "&page={}"

	list_html += tool.Baro_paging(page_num, len(table_rows), tool.Baro_page_size, page_url)

	// 거르개
	filter_form := `<form class="bw_form" method="get" action="/baro/source">`
	filter_form += tool.Baro_input("title", tool.Baro_t("문서 이름"), title, tool.Baro_t("비우면 전체"), "")
	filter_form += tool.Baro_select("kind", "종류", append([][]string{{"", tool.Baro_t("전체")}}, baro_quality_cite_kind_options()...), kind, "")
	filter_form += tool.Baro_select("status", tool.Baro_t("링크 상태"), [][]string{
		{"", tool.Baro_t("전체")}, {"ok", tool.Baro_t("정상")}, {"broken", tool.Baro_t("깨짐")}, {"timeout", tool.Baro_t("응답 없음")},
	}, status, tool.Baro_t("미검사만 보려면 링크 검사 화면을 쓰세요."))
	filter_form += tool.Baro_select("quality", tool.Baro_t("자료 등급"), baro_quality_quality_options(), quality, "")
	filter_form += tool.Baro_submit(tool.Baro_t("거르기"), "primary")
	filter_form += `</form>`

	body := baro_quality_crumb(tool.Baro_t("출처 품질"))
	body += tool.Baro_hero(
		tool.Baro_t("출처 품질"),
		tool.Baro_t("출처가 있는지뿐 아니라, 어떤 종류인지·살아 있는지·믿을 만한지까지 봅니다."),
		tool.Baro_button("/baro/source/scan", "출처 수집", "primary")+" "+
			tool.Baro_button("/baro/source/check", tool.Baro_t("링크 검사"), "")+" "+
			tool.Baro_button("/baro/source/nosource", tool.Baro_t("출처 없는 문서"), ""),
	)
	body += baro_quality_nav("/baro/source")
	body += baro_quality_source_nav("/baro/source")
	body += baro_quality_source_stats(db)

	if format_bad > 0 {
		body += tool.Baro_alert("info",
			tool.Baro_t("이 쪽에서 표준 인용 형식을 채우지 못한 출처가 ")+tool.Baro_itoa(format_bad)+tool.Baro_t("건 있습니다. ")+
				tool.Baro_t(`<a href="/baro/source/format">인용 형식 안내</a> 를 확인해 주세요.`))
	}

	body += tool.Baro_card(tool.Baro_t("거르기"), filter_form)
	body += tool.Baro_card_ex(tool.Baro_t("출처 목록"), tool.Baro_t("최근에 등록된 순서입니다."), list_html, "")

	return tool.Baro_page(db, config, tool.Baro_t("출처 품질"), body, baro_quality_menu())
}

// View_baro_quality_source_scan : 출처 수집 (권한 review)
func View_baro_quality_source_scan(config tool.Config, title string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/source/scan"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "review") {
		return tool.Baro_deny(db, config, tool.Baro_t("출처 수집은 검토 권한이 있는 사용자만 실행할 수 있습니다."))
	}

	title = strings.TrimSpace(title)
	message := ""

	if values != nil {
		if tool.Baro_read_only(db) {
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
		} else if !tool.Baro_rate_ok(db, "quality_source_scan", config.IP, 6, 600) {
			message = tool.Baro_alert("warn", tool.Baro_t("출처 수집을 너무 자주 실행했습니다. 잠시 후 다시 시도하세요."))
		} else {
			post_title := tool.Namespace_normalize(baro_quality_val(values, "title"))

			if post_title != "" {
				if !baro_quality_doc_exists(db, post_title) {
					message = tool.Baro_alert("danger", tool.Baro_t("그 이름의 문서 본문이 없습니다."))
				} else {
					count := baro_quality_source_scan_doc(db, post_title, baro_quality_doc_data(db, post_title))

					tool.Baro_public_log(db, "citation_scan", config.IP, post_title, "출처 수집", tool.Baro_itoa(count)+"건 등록")
					tool.Baro_audit(db, config, "citation_scan", post_title, "", tool.Baro_itoa(count), "문서 한 개 출처 수집")

					return tool.Get_redirect("/baro/source?title=" + tool.Url_parser(post_title))
				}
			} else {
				limit := tool.Str_to_int(baro_quality_val(values, "limit"))
				docs, cites := baro_quality_source_scan_run(db, limit)

				tool.Baro_public_log(db, "citation_scan", config.IP, "",
					"문서 "+tool.Baro_itoa(docs)+"개 수집", "출처 "+tool.Baro_itoa(cites)+"건 등록")
				tool.Baro_audit(db, config, "citation_scan", "data",
					"", "docs="+tool.Baro_itoa(docs)+" cites="+tool.Baro_itoa(cites), "출처 일괄 수집")

				return tool.Get_redirect("/baro/source")
			}
		}

		title = tool.Namespace_normalize(baro_quality_val(values, "title"))
	}

	offset := tool.Baro_get_int(db, "quality_source_offset", 0)
	doc_total := baro_quality_count(db,
		"select count(*) from data where "+tool.Get_except_document_name_SQL("title"))

	form := tool.Baro_input("title", tool.Baro_t("문서 이름 (선택)"), title, tool.Baro_t("비우면 여러 문서를 한꺼번에"), tool.Baro_t("한 문서만 다시 수집하려면 이름을 적으세요."))
	form += tool.Baro_select("limit", tool.Baro_t("한 번에 처리할 문서 수"), [][]string{
		{"50", tool.Baro_t("50문서")},
		{"100", tool.Baro_t("100문서")},
		{"200", tool.Baro_t("200문서")},
		{"300", tool.Baro_t("300문서 (최대)")},
	}, "100", tool.Baro_t("이어서 실행하면 다음 묶음을 처리합니다. 끝까지 가면 처음으로 돌아갑니다."))
	form += tool.Baro_submit(tool.Baro_t("출처 수집 실행"), "primary")

	body := baro_quality_crumb("출처 수집")
	body += tool.Baro_hero("출처 수집", tool.Baro_t("문서 본문에서 각주와 URL 을 뽑아 출처 목록에 등록합니다."), "")
	body += baro_quality_nav("/baro/source")
	body += baro_quality_source_nav("/baro/source/scan")
	body += message

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("일반 문서"), tool.Baro_comma(doc_total), ""},
		{tool.Baro_t("다음 시작 위치"), tool.Baro_comma(offset) + tool.Baro_t("번째"), tool.Baro_t("이어서 처리")},
		{tool.Baro_t("등록된 출처"), tool.Baro_comma(baro_quality_count(db, "select count(*) from baro_citation")), ""},
	})

	body += tool.Baro_card_ex(tool.Baro_t("수집 실행"), tool.Baro_t("한 번에 최대 300문서까지 처리합니다."),
		tool.Baro_form("/baro/source/scan", form),
		tool.Baro_button("/baro/source", tool.Baro_t("출처 목록"), ""))

	body += tool.Baro_card(tool.Baro_t("종류 추정 규칙"), tool.Baro_table(
		[]string{tool.Baro_t("조건"), "종류"},
		[][]string{
			{tool.Baro_t("doi.org · arxiv · 학술지 표기"), tool.Baro_t("논문")},
			{tool.Baro_t("신문·방송사 도메인 또는 신문사 이름 표기"), tool.Baro_t("신문·방송")},
			{tool.Baro_t("그 밖의 URL"), tool.Baro_t("웹사이트")},
			{tool.Baro_t("ISBN · 출판사 · 쪽수 등 책 표기"), tool.Baro_t("책")},
			{tool.Baro_t("위 어디에도 맞지 않음"), "기타"},
		}, ""))

	body += tool.Baro_note(tool.Baro_t("수집은 본문을 다시 읽어 출처 목록을 새로 만듭니다. 다만 사람이 손질해 둔 값(종류·저자·발행처·날짜·보존 링크·자료 등급·링크 상태)은 ") +
		tool.Baro_t("같은 주소(주소가 없으면 같은 표기)로 알아보아 그대로 옮깁니다. 본문에서 사라진 출처만 목록에서 빠집니다."))

	return tool.Baro_page(db, config, "출처 수집", body, baro_quality_menu())
}

// View_baro_quality_source_nosource : 각주가 하나도 없는 문서
func View_baro_quality_source_nosource(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_num := baro_quality_page_num(page)
	offset := (page_num - 1) * tool.Baro_page_size

	except := tool.Get_except_document_name_SQL("d.title")

	// 등록된 출처가 하나도 없는 문서를 훑되, 본문에 각주·URL 이 정말 없는지 다시 확인한다.
	// (출처 수집을 아직 돌리지 않은 문서가 "출처 없음" 으로 잘못 보이지 않게 한다)
	const scan_max = 3000

	rows := tool.Query_DB(db,
		"select d.title, d.data from data d left join baro_citation c on c.title = d.title "+
			"where c.title is null and "+except+
			" and d.data not like '#redirect%' and d.data not like '#넘겨주기%' "+
			"order by d.title limit ?",
		scan_max)
	defer rows.Close()

	type nosource_row struct {
		Title  string
		Length int
	}

	list := []nosource_row{}
	for rows.Next() {
		title, data := "", ""
		if err := rows.Scan(&title, &data); err != nil {
			continue
		}

		if strings.TrimSpace(data) == "" {
			continue
		}

		// 본문에 각주나 링크가 하나라도 있으면 "출처 없음" 이 아니다 (수집만 안 돌린 문서)
		if baro_quality_has_source(data) {
			continue
		}

		list = append(list, nosource_row{Title: title, Length: tool.Get_len(data)})
	}

	start := offset
	end := start + tool.Baro_page_size
	if start > len(list) {
		start = len(list)
	}
	if end > len(list) {
		end = len(list)
	}

	table_rows := [][]string{}
	for _, row := range list[start:end] {
		table_rows = append(table_rows, []string{
			baro_quality_doc_cell(row.Title),
			tool.Baro_comma(row.Length) + tool.Baro_t("자"),
			tool.Baro_status_badge(tool.Baro_doc_status_get(db, row.Title)),
			tool.Baro_button("/baro/quality/set?title="+tool.Url_parser(row.Title), "출처 필요 표시", "") + " " +
				tool.Baro_button("/w/" + tool.Doc_url(row.Title)+"?edit=1", "고치기", ""),
		})
	}

	list_html := tool.Baro_table(
		[]string{"문서", tool.Baro_t("본문 길이"), tool.Baro_t("현재 상태"), "관리"},
		table_rows, "")

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("각주도 링크도 없는 문서를 찾지 못했습니다."))
	}

	list_html += tool.Baro_paging(page_num, len(table_rows), tool.Baro_page_size, "/baro/source/nosource?page={}")

	body := baro_quality_crumb(tool.Baro_t("출처 없는 문서"))
	body += tool.Baro_hero(
		tool.Baro_t("출처 없는 문서"),
		tool.Baro_t("각주도 링크도 하나 없는 문서입니다. 지우자는 목록이 아니라 채우자는 목록입니다."),
		tool.Baro_button("/baro/source/scan", tool.Baro_t("출처 수집 먼저 실행"), "primary"),
	)
	body += baro_quality_nav("/baro/source")
	body += baro_quality_source_nav("/baro/source/nosource")

	body += tool.Baro_alert("info",
		tool.Baro_t("등록된 출처가 없는 문서 가운데, 본문에도 각주(<code>[* ... ]</code>)와 링크가 하나도 없는 문서만 골랐습니다. ")+
			tool.Baro_t("수집을 아직 돌리지 않았을 뿐인 문서는 여기에 나오지 않습니다."))

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("각주도 링크도 없는 문서"), tool.Baro_comma(len(list)), tool.Baro_t("앞 ") + tool.Baro_comma(scan_max) + tool.Baro_t("개 문서 기준")},
		{tool.Baro_t("이 쪽에 보이는 문서"), tool.Baro_comma(len(table_rows)), ""},
	})

	body += tool.Baro_card_ex(tool.Baro_t("출처가 없는 문서"), tool.Baro_t("이름 순서입니다."), list_html,
		tool.Baro_button("/baro/source", tool.Baro_t("출처 목록"), ""))

	return tool.Baro_page(db, config, tool.Baro_t("출처 없는 문서"), body, baro_quality_menu())
}

// View_baro_quality_source_check : 오래된 링크 · 깨진 링크 검사
func View_baro_quality_source_check(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	message := ""

	if values != nil {
		switch {
		case !tool.Baro_is_user(config.IP):
			message = tool.Baro_alert("danger", tool.Baro_t("링크 검사는 로그인한 사용자만 실행할 수 있습니다."))
		case !tool.Baro_can(db, config.IP, "review"):
			message = tool.Baro_alert("danger", tool.Baro_t("링크 검사는 검토 권한이 있는 사용자만 실행할 수 있습니다."))
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
		case !tool.Baro_rate_ok(db, "quality_source_check", config.IP, 10, 600):
			message = tool.Baro_alert("warn", tool.Baro_t("링크 검사를 너무 자주 실행했습니다. 잠시 후 다시 시도하세요."))
		default:
			mode := baro_quality_val(values, "mode")
			checked, ok_count, broken, timeout := baro_quality_source_check_run(db, mode)

			tool.Baro_public_log(db, "citation_check", config.IP, "",
				"링크 "+tool.Baro_itoa(checked)+"개 검사",
				"정상 "+tool.Baro_itoa(ok_count)+" / 깨짐 "+tool.Baro_itoa(broken)+" / 응답 없음 "+tool.Baro_itoa(timeout))
			tool.Baro_audit(db, config, "citation_check", "baro_citation", "",
				"checked="+tool.Baro_itoa(checked), "링크 생존 검사")

			return tool.Get_redirect("/baro/source?status=broken")
		}
	}

	unchecked := baro_quality_count(db, "select count(*) from baro_citation where url != '' and "+baro_quality_source_unchecked_sql)
	broken := baro_quality_count(db, "select count(*) from baro_citation where status = 'broken'")
	timeout := baro_quality_count(db, "select count(*) from baro_citation where status = 'timeout'")

	rows := tool.Query_DB(db,
		"select id, title, url, status, access_date from baro_citation where "+baro_quality_source_checked_sql+
			" order by access_date desc, id + 0 desc limit 30")
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		id, title, raw, status, access_date := "", "", "", "", ""
		if err := rows.Scan(&id, &title, &raw, &status, &access_date); err != nil {
			continue
		}

		table_rows = append(table_rows, []string{
			baro_quality_doc_cell(title),
			baro_quality_ext_link(raw),
			baro_quality_link_badge(status),
			tool.Baro_time_cell(access_date),
			tool.Baro_button("/baro/source/edit/"+tool.Url_parser(id), "손질", ""),
		})
	}

	list_html := tool.Baro_table(
		[]string{"문서", tool.Baro_t("주소"), "상태", tool.Baro_t("검사 시각"), "관리"},
		table_rows, "")

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("아직 검사한 링크가 없습니다."))
	}

	form := tool.Baro_select("mode", tool.Baro_t("검사 방식"), [][]string{
		{"new", tool.Baro_t("아직 검사하지 않은 링크부터")},
		{"recheck", tool.Baro_t("가장 오래 전에 검사한 링크부터 다시")},
	}, "new", tool.Baro_t("한 번에 30개까지, 링크마다 5초까지 기다립니다."))
	form += tool.Baro_submit(tool.Baro_t("링크 검사 실행"), "primary")

	body := baro_quality_crumb(tool.Baro_t("링크 검사"))
	body += tool.Baro_hero(tool.Baro_t("오래된 링크 · 깨진 링크 검사"), tool.Baro_t("출처 주소에 HEAD 요청을 보내 살아 있는지 확인합니다."), "")
	body += baro_quality_nav("/baro/source")
	body += baro_quality_source_nav("/baro/source/check")
	body += message

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("미검사 링크"), tool.Baro_comma(unchecked), ""},
		{tool.Baro_t("깨진 링크"), tool.Baro_comma(broken), ""},
		{tool.Baro_t("응답 없음"), tool.Baro_comma(timeout), ""},
	})

	if tool.Baro_can(db, config.IP, "review") {
		body += tool.Baro_card_ex(tool.Baro_t("검사 실행"), tool.Baro_t("한 번에 최대 30개 · 요청당 5초 제한"),
			tool.Baro_form("/baro/source/check", form),
			tool.Baro_button("/baro/source?status=broken", tool.Baro_t("깨진 링크만 보기"), ""))
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("검사 결과는 누구나 볼 수 있지만, 검사를 실행하려면 검토 권한이 필요합니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("최근 검사 결과"), tool.Baro_t("가장 최근에 검사한 30건입니다."), list_html, "")

	body += tool.Baro_card(tool.Baro_t("링크가 깨졌을 때"), tool.Baro_list([]string{
		tool.Baro_t("먼저 원문 보존 링크(archive_url)를 채워 주세요. 출처를 지우는 것은 마지막 수단입니다."),
		tool.Baro_t("보존본이 없으면 web.archive.org 저장 링크로 새로 만들 수 있습니다. 출처 손질 화면에 안내가 있습니다."),
		tool.Baro_t("링크가 깨졌다고 서술까지 자동으로 지우지 않습니다. 출처 필요 상태로 표시해 두세요."),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("링크 검사"), body, baro_quality_menu())
}

// View_baro_quality_source_overused : 같은 출처 과다 사용 경고
func View_baro_quality_source_overused(config tool.Config, page string, limit string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_num := baro_quality_page_num(page)
	offset := (page_num - 1) * tool.Baro_page_size

	min_count := tool.Str_to_int(limit)
	if min_count < 2 {
		min_count = tool.Baro_get_int(db, "source_overused_limit", 10)
	}
	if min_count < 2 {
		min_count = 2
	}

	rows := tool.Query_DB(db,
		"select url, count(*) as use_count, count(distinct title) as doc_count from baro_citation "+
			"where url != '' group by url having use_count >= ? order by use_count desc limit ?, ?",
		min_count, offset, tool.Baro_page_size)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		raw := ""
		use_count, doc_count := 0, 0
		if err := rows.Scan(&raw, &use_count, &doc_count); err != nil {
			continue
		}

		reason := baro_quality_low_trust(raw)
		trust_cell := tool.Baro_badge(tool.Baro_t("일반"), "muted")
		if reason != "" {
			trust_cell = tool.Baro_badge(reason, "danger")
		}

		table_rows = append(table_rows, []string{
			baro_quality_ext_link(raw),
			tool.HTML_escape(baro_quality_host(raw)),
			tool.Baro_comma(use_count) + tool.Baro_t("회"),
			tool.Baro_comma(doc_count) + "개 문서",
			trust_cell,
			baro_quality_ext_link("https://web.archive.org/web/*/" + raw),
		})
	}

	list_html := tool.Baro_table(
		[]string{tool.Baro_t("주소"), tool.Baro_t("도메인"), tool.Baro_t("사용 횟수"), tool.Baro_t("쓰인 문서 수"), tool.Baro_t("신뢰도"), tool.Baro_t("보존본 찾기")},
		table_rows, "")

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_itoa(min_count) + tool.Baro_t("회 이상 쓰인 출처가 없습니다."))
	}

	list_html += tool.Baro_paging(page_num, len(table_rows), tool.Baro_page_size,
		"/baro/source/overused?limit="+tool.Baro_itoa(min_count)+"&page={}")

	// 도메인 단위 집계
	domain_rows := [][]string{}
	domains := tool.Query_DB(db, "select url from baro_citation where url != '' limit 5000")
	defer domains.Close()

	domain_count := map[string]int{}
	for domains.Next() {
		raw := ""
		if err := domains.Scan(&raw); err != nil {
			continue
		}

		host := baro_quality_host(raw)
		if host == "" {
			continue
		}

		domain_count[host]++
	}

	top := baro_quality_top_pairs(domain_count, 20)
	for _, pair := range top {
		reason := ""
		for _, row := range baro_quality_low_trust_domains() {
			if pair[0] == row[0] || strings.HasSuffix(pair[0], "."+row[0]) {
				reason = row[1]
				break
			}
		}

		trust_cell := tool.Baro_badge(tool.Baro_t("일반"), "muted")
		if reason != "" {
			trust_cell = tool.Baro_badge(reason, "danger")
		}

		domain_rows = append(domain_rows, []string{
			tool.HTML_escape(pair[0]),
			tool.Baro_comma(tool.Str_to_int(pair[1])) + tool.Baro_t("회"),
			trust_cell,
		})
	}

	body := baro_quality_crumb(tool.Baro_t("출처 과다 사용"))
	body += tool.Baro_hero(
		tool.Baro_t("같은 출처 과다 사용"),
		tool.Baro_t("한 곳에서만 퍼 온 문서는 균형이 무너지기 쉽습니다."),
		"",
	)
	body += baro_quality_nav("/baro/source")
	body += baro_quality_source_nav("/baro/source/overused")

	body += tool.Baro_alert("info",
		tool.Baro_t("같은 주소가 여러 번 쓰였다고 잘못된 것은 아닙니다. 다만 한 출처에 지나치게 기대고 있지 않은지 확인하는 데 씁니다."))

	body += tool.Baro_card(tool.Baro_t("기준 바꾸기"),
		`<form class="bw_form" method="get" action="/baro/source/overused">`+
			tool.Baro_input_type("limit", tool.Baro_t("몇 회 이상을 과다로 볼까요"), tool.Baro_itoa(min_count), "number", tool.Baro_t("기본값은 baro_set 의 source_overused_limit 입니다."))+
			tool.Baro_submit(tool.Baro_t("적용"), "primary")+
			`</form>`)

	body += tool.Baro_card_ex(tool.Baro_t("과다 사용된 주소"), tool.Baro_itoa(min_count)+tool.Baro_t("회 이상 쓰인 주소입니다."), list_html, "")
	body += tool.Baro_card_ex(tool.Baro_t("도메인별 사용 횟수"), tool.Baro_t("가장 많이 쓰인 도메인 20개입니다."), tool.Baro_table(
		[]string{tool.Baro_t("도메인"), tool.Baro_t("사용 횟수"), tool.Baro_t("신뢰도")},
		domain_rows, tool.Baro_t("집계할 출처가 없습니다.")), "")

	return tool.Baro_page(db, config, tool.Baro_t("출처 과다 사용"), body, baro_quality_menu())
}

// baro_quality_top_pairs : map 을 값이 큰 순서로 정렬해 상위 n 개를 돌려준다
func baro_quality_top_pairs(source map[string]int, limit int) [][]string {
	out := [][]string{}
	used := map[string]bool{}

	for len(out) < limit {
		best_key, best_value := "", 0
		for key, value := range source {
			if used[key] {
				continue
			}

			if value > best_value || (value == best_value && best_key != "" && key < best_key) {
				best_key, best_value = key, value
			}
		}

		if best_key == "" {
			break
		}

		used[best_key] = true
		out = append(out, []string{best_key, tool.Baro_itoa(best_value)})
	}

	return out
}

// View_baro_quality_source_lowtrust : 신뢰도 낮은 출처 안내
func View_baro_quality_source_lowtrust(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	message := ""

	if values != nil {
		switch {
		case !tool.Baro_can(db, config.IP, "review"):
			message = tool.Baro_alert("danger", tool.Baro_t("표시를 바꾸려면 검토 권한이 필요합니다."))
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
		default:
			marked := baro_quality_source_mark_low(db)

			tool.Baro_public_log(db, "citation_lowtrust", config.IP, "",
				"신뢰도 낮은 도메인 일괄 표시", tool.Baro_itoa(marked)+"건")
			tool.Baro_audit(db, config, "citation_lowtrust", "baro_citation", "", tool.Baro_itoa(marked), "신뢰도 낮은 출처 표시")

			return tool.Get_redirect("/baro/source/lowtrust")
		}
	}

	page_num := baro_quality_page_num(page)
	offset := (page_num - 1) * tool.Baro_page_size

	rows := tool.Query_DB(db,
		"select id, title, url, kind, quality from baro_citation where quality = 'low' order by id + 0 desc limit ?, ?",
		offset, tool.Baro_page_size)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		id, title, raw, kind, quality := "", "", "", "", ""
		if err := rows.Scan(&id, &title, &raw, &kind, &quality); err != nil {
			continue
		}

		table_rows = append(table_rows, []string{
			baro_quality_doc_cell(title),
			tool.HTML_escape(baro_quality_cite_kind_name(kind)),
			baro_quality_ext_link(raw),
			tool.HTML_escape(baro_quality_low_trust(raw)),
			baro_quality_quality_badge(quality),
			tool.Baro_button("/baro/source/edit/"+tool.Url_parser(id), "손질", ""),
		})
	}

	list_html := tool.Baro_table(
		[]string{"문서", "종류", tool.Baro_t("주소"), "사유", tool.Baro_t("등급"), "관리"},
		table_rows, "")

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("신뢰도 낮음으로 표시된 출처가 없습니다."))
	}

	list_html += tool.Baro_paging(page_num, len(table_rows), tool.Baro_page_size, "/baro/source/lowtrust?page={}")

	domain_items := []string{}
	for _, row := range baro_quality_low_trust_domains() {
		domain_items = append(domain_items, tool.Baro_code(row[0])+" "+tool.HTML_escape(row[1]))
	}

	candidate := baro_quality_source_low_candidate(db)

	body := baro_quality_crumb(tool.Baro_t("신뢰도 낮은 출처"))
	body += tool.Baro_hero(
		tool.Baro_t("신뢰도 낮은 출처 안내"),
		tool.Baro_t("개인 블로그 · 위키 · 커뮤니티 · 소셜 미디어는 그 자체로 근거가 되기 어렵습니다."),
		"",
	)
	body += baro_quality_nav("/baro/source")
	body += baro_quality_source_nav("/baro/source/lowtrust")
	body += message

	body += tool.Baro_alert("warn",
		tool.Baro_t("신뢰도 낮음 표시는 <b>금지</b>가 아니라 <b>주의</b>입니다. 그 매체 자체가 주제인 경우(예: 어떤 커뮤니티 글이 사건이 된 경우)에는 1차 자료로 인정할 수 있습니다."))

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("신뢰도 낮음 표시"), tool.Baro_comma(baro_quality_count(db, "select count(*) from baro_citation where quality = 'low'")), ""},
		{tool.Baro_t("아직 표시 안 된 후보"), tool.Baro_comma(candidate), tool.Baro_t("도메인 목록에 해당")},
		{tool.Baro_t("1차 자료 지정"), tool.Baro_comma(baro_quality_count(db, "select count(*) from baro_citation where quality = 'primary'")), ""},
		{tool.Baro_t("2차 자료 지정"), tool.Baro_comma(baro_quality_count(db, "select count(*) from baro_citation where quality = 'secondary'")), ""},
	})

	if tool.Baro_can(db, config.IP, "review") {
		body += tool.Baro_card_ex(tool.Baro_t("일괄 표시"), tool.Baro_t("도메인 목록에 해당하면서 아직 등급이 없는 출처를 신뢰도 낮음으로 표시합니다."),
			tool.Baro_form("/baro/source/lowtrust",
				tool.Baro_note(tool.Baro_t("이미 1차·2차 자료로 지정한 출처는 건드리지 않습니다."))+
					tool.Baro_submit(tool.Baro_t("후보 ")+tool.Baro_itoa(candidate)+tool.Baro_t("건 표시하기"), "primary")),
			"")
	}

	body += tool.Baro_card_ex(tool.Baro_t("신뢰도 낮음으로 표시된 출처"), tool.Baro_t("최근에 등록된 순서입니다."), list_html, "")
	body += tool.Baro_card(tool.Baro_t("주의 도메인 목록"), tool.Baro_list(domain_items))

	body += tool.Baro_card(tool.Baro_t("1차 자료와 2차 자료"), tool.Baro_dl([][]string{
		{tool.Baro_t("1차 자료"), tool.Baro_t("사건 당사자·기관이 직접 낸 원자료입니다. 공시, 판결문, 법령, 통계 원본, 인터뷰 원문 등.")},
		{tool.Baro_t("2차 자료"), tool.Baro_t("1차 자료를 해석하고 정리한 자료입니다. 연구 논문, 해설 기사, 단행본 등.")},
		{tool.Baro_t("쓰는 법"), tool.Baro_t("사실 확인은 1차 자료로, 평가와 맥락은 2차 자료로 붙이는 편이 안전합니다. 출처 손질 화면에서 지정할 수 있습니다.")},
	}))

	return tool.Baro_page(db, config, tool.Baro_t("신뢰도 낮은 출처"), body, baro_quality_menu())
}

// baro_quality_source_low_candidate : 신뢰도 낮음으로 표시할 후보 수
func baro_quality_source_low_candidate(db *sql.DB) int {
	rows := tool.Query_DB(db, "select url from baro_citation where url != '' and quality = '' limit 5000")
	defer rows.Close()

	count := 0
	for rows.Next() {
		raw := ""
		if err := rows.Scan(&raw); err != nil {
			continue
		}

		if baro_quality_low_trust(raw) != "" {
			count++
		}
	}

	return count
}

// baro_quality_source_mark_low : 도메인 목록에 해당하는 출처를 신뢰도 낮음으로 표시
func baro_quality_source_mark_low(db *sql.DB) int {
	type cite_row struct {
		ID, URL string
	}

	list := []cite_row{}

	rows := tool.Query_DB(db, "select id, url from baro_citation where url != '' and quality = '' limit 5000")
	defer rows.Close()

	for rows.Next() {
		row := cite_row{}
		if err := rows.Scan(&row.ID, &row.URL); err != nil {
			continue
		}

		if baro_quality_low_trust(row.URL) != "" {
			list = append(list, row)
		}
	}

	for _, row := range list {
		tool.Exec_DB(db, "update baro_citation set quality = 'low' where id = ?", row.ID)
	}

	return len(list)
}

// View_baro_quality_source_format : 인용 형식 통일
func View_baro_quality_source_format(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_num := baro_quality_page_num(page)
	offset := (page_num - 1) * tool.Baro_page_size

	guide_rows := [][]string{}
	for _, row := range baro_quality_cite_format_guide() {
		guide_rows = append(guide_rows, []string{
			tool.HTML_escape(row[0]),
			tool.Baro_code(row[1]),
			tool.HTML_escape(row[2]),
		})
	}

	rows := tool.Query_DB(db,
		"select id, title, kind, url, author, publisher, pub_date, access_date from baro_citation order by id + 0 desc limit ?, ?",
		offset, tool.Baro_page_size*4)
	defer rows.Close()

	type cite_row struct {
		ID, Title, Kind, URL, Author, Publisher, PubDate, AccessDate string
	}

	list := []cite_row{}
	for rows.Next() {
		row := cite_row{}
		if err := rows.Scan(&row.ID, &row.Title, &row.Kind, &row.URL, &row.Author, &row.Publisher, &row.PubDate, &row.AccessDate); err != nil {
			continue
		}

		list = append(list, row)
	}

	table_rows := [][]string{}
	for _, row := range list {
		problem := baro_quality_cite_format_problem(row.Kind, row.URL, row.Author, row.Publisher, row.PubDate, row.AccessDate)
		if problem == "" {
			continue
		}

		if len(table_rows) >= tool.Baro_page_size {
			break
		}

		source_cell := baro_quality_ext_link(row.URL)
		if row.URL == "" {
			source_cell = tool.HTML_escape(tool.Baro_trunc(row.Publisher, 60))
		}

		table_rows = append(table_rows, []string{
			baro_quality_doc_cell(row.Title),
			tool.HTML_escape(baro_quality_cite_kind_name(row.Kind)),
			source_cell,
			tool.Baro_badge(problem, "warn"),
			tool.Baro_button("/baro/source/edit/"+tool.Url_parser(row.ID), "채우기", "primary"),
		})
	}

	list_html := tool.Baro_table(
		[]string{"문서", "종류", "출처", tool.Baro_t("모자란 항목"), "관리"},
		table_rows, "")

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("이 구간에서 형식이 모자란 출처를 찾지 못했습니다."))
	}

	list_html += tool.Baro_paging(page_num, len(list), tool.Baro_page_size*4, "/baro/source/format?page={}")

	body := baro_quality_crumb(tool.Baro_t("인용 형식"))
	body += tool.Baro_hero(tool.Baro_t("인용 형식 통일"), tool.Baro_t("같은 규칙으로 적힌 출처는 확인하기도 쉽고 고치기도 쉽습니다."), "")
	body += baro_quality_nav("/baro/source")
	body += baro_quality_source_nav("/baro/source/format")

	body += tool.Baro_card_ex(tool.Baro_t("표준 형식"), tool.Baro_t("책 · 논문 · 신문 · 웹사이트 네 가지를 기준으로 씁니다."), tool.Baro_table(
		[]string{"종류", tool.Baro_t("형식"), "보기"},
		guide_rows, ""), "")

	body += tool.Baro_card_ex(tool.Baro_t("형식에 맞지 않는 출처"), tool.Baro_t("필수 항목이 비어 있는 출처입니다."), list_html,
		tool.Baro_note(tool.Baro_t("항목이 비었다고 잘못된 출처는 아닙니다. 손질 화면에서 저자·발행처·날짜를 채워 주세요.")))

	return tool.Baro_page(db, config, tool.Baro_t("인용 형식"), body, baro_quality_menu())
}

// View_baro_quality_source_edit : 출처 한 건 손질 (권한 review)
func View_baro_quality_source_edit(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	id = strings.TrimSpace(id)

	title, kind, raw, author := "", "", "", ""
	publisher, pub_date, access_date, archive := "", "", "", ""
	quality, status, rev, date := "", "", "", ""

	found := tool.QueryRow_DB(db,
		"select title, kind, url, author, publisher, pub_date, access_date, archive_url, quality, status, rev, date from baro_citation where id = ?",
		[]any{&title, &kind, &raw, &author, &publisher, &pub_date, &access_date, &archive, &quality, &status, &rev, &date},
		id)

	if !found {
		body := baro_quality_crumb(tool.Baro_t("출처 손질"))
		body += tool.Baro_hero(tool.Baro_t("출처를 찾을 수 없습니다"), tool.Baro_t("번호가 잘못되었거나 이미 지워진 기록입니다."), "")
		body += baro_quality_nav("/baro/source")
		body += tool.Baro_alert("warn", tool.Baro_t("출처 번호 ")+tool.HTML_escape(tool.Baro_trunc(id, 40))+tool.Baro_t(" 에 해당하는 기록이 없습니다."))
		body += tool.Baro_card(tool.Baro_t("다음 할 일"), tool.Baro_button("/baro/source", tool.Baro_t("출처 목록"), "primary")+" "+
			tool.Baro_button("/baro/source/scan", "출처 수집", ""))

		return tool.Baro_page(db, config, tool.Baro_t("출처 손질"), body, baro_quality_menu())
	}

	message := ""

	if values != nil {
		switch {
		case !tool.Baro_is_user(config.IP):
			message = tool.Baro_alert("danger", tool.Baro_t("출처를 손질하려면 로그인이 필요합니다."))
		case !tool.Baro_can(db, config.IP, "review"):
			message = tool.Baro_alert("danger", tool.Baro_t("출처 손질은 검토 권한이 있는 사용자만 할 수 있습니다."))
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
		default:
			new_kind := baro_quality_val(values, "kind")
			new_quality := baro_quality_val(values, "quality")
			new_author := tool.Baro_trunc(baro_quality_val(values, "author"), 200)
			new_publisher := tool.Baro_trunc(baro_quality_val(values, "publisher"), 300)
			new_pub_date := tool.Baro_trunc(baro_quality_val(values, "pub_date"), 30)
			new_access := tool.Baro_trunc(baro_quality_val(values, "access_date"), 30)
			new_archive := tool.Baro_trunc(baro_quality_val(values, "archive_url"), 500)

			valid_kind := false
			for _, option := range baro_quality_cite_kind_options() {
				if option[0] == new_kind {
					valid_kind = true
				}
			}

			valid_quality := false
			for _, option := range baro_quality_quality_options() {
				if option[0] == new_quality {
					valid_quality = true
				}
			}

			switch {
			case !valid_kind:
				message = tool.Baro_alert("danger", tool.Baro_t("알 수 없는 출처 종류입니다."))
			case !valid_quality:
				message = tool.Baro_alert("danger", tool.Baro_t("알 수 없는 자료 등급입니다."))
			case new_archive != "" && !strings.HasPrefix(new_archive, "http://") && !strings.HasPrefix(new_archive, "https://"):
				message = tool.Baro_alert("danger", tool.Baro_t("원문 보존 링크는 http:// 또는 https:// 로 시작해야 합니다."))
			default:
				before := kind + "/" + quality + "/" + archive

				tool.Exec_DB(db,
					"update baro_citation set kind = ?, author = ?, publisher = ?, pub_date = ?, access_date = ?, archive_url = ?, quality = ? where id = ?",
					new_kind, new_author, new_publisher, new_pub_date, new_access, new_archive, new_quality, id)

				tool.Baro_public_log(db, "citation_edit", config.IP, title, "출처 정보 손질",
					baro_quality_cite_kind_name(new_kind)+" / "+baro_quality_quality_name(new_quality))
				tool.Baro_audit(db, config, "citation_edit", "citation:"+id,
					before, new_kind+"/"+new_quality+"/"+new_archive, "출처 손질")

				return tool.Get_redirect("/baro/source?title=" + tool.Url_parser(title))
			}
		}
	}

	low_reason := baro_quality_low_trust(raw)
	problem := baro_quality_cite_format_problem(kind, raw, author, publisher, pub_date, access_date)

	form := tool.Baro_select("kind", tool.Baro_t("출처 종류"), baro_quality_cite_kind_options(), kind, tool.Baro_t("종류에 따라 필요한 항목이 달라집니다."))
	form += tool.Baro_select("quality", tool.Baro_t("자료 등급"), baro_quality_quality_options(), quality, tool.Baro_t("1차 자료 · 2차 자료 · 신뢰도 낮음을 지정합니다."))
	form += tool.Baro_input("author", tool.Baro_t("저자 · 기자"), author, tool.Baro_t("예: 홍길동"), "")
	form += tool.Baro_textarea("publisher", tool.Baro_t("발행처 · 원문 표기"), publisher, 3, tool.Baro_t("예: 바로출판 / 각주 원문"), tool.Baro_t("본문 각주 표기를 그대로 두어도 됩니다."))
	form += tool.Baro_input("pub_date", tool.Baro_t("발행 · 보도 날짜"), pub_date, tool.Baro_t("예: 2024 또는 2024-05-01"), "")
	form += tool.Baro_input("access_date", tool.Baro_t("확인 날짜"), access_date, tool.Baro_t("예: ")+tool.Baro_today(), tool.Baro_t("웹 출처는 확인 날짜가 필요합니다."))
	form += tool.Baro_input("archive_url", tool.Baro_t("원문 보존 링크"), archive, "https://web.archive.org/web/...", tool.Baro_t("링크가 사라져도 확인할 수 있게 보존본 주소를 남깁니다."))
	form += tool.Baro_submit("저장", "primary")

	body := baro_quality_crumb(tool.Baro_t("출처 손질"))
	body += tool.Baro_hero(
		tool.Baro_t("출처 손질 #")+id,
		tool.Namespace_display(title),
		tool.Baro_button("/baro/quality/doc/"+tool.Url_parser(title), "문서 품질 정보", "")+" "+
			tool.Baro_button("/baro/source?title="+tool.Url_parser(title), "이 문서 출처 목록", ""),
	)
	body += baro_quality_nav("/baro/source")
	body += baro_quality_source_nav("/baro/source")
	body += message

	body += tool.Baro_kv_grid([][]string{
		{"문서", baro_quality_doc_cell(title)},
		{tool.Baro_t("수집된 판"), "r" + tool.HTML_escape(rev)},
		{tool.Baro_t("주소"), baro_quality_ext_link(raw)},
		{tool.Baro_t("도메인"), tool.HTML_escape(baro_quality_host(raw))},
		{tool.Baro_t("링크 상태"), baro_quality_link_badge(status)},
		{tool.Baro_t("자료 등급"), baro_quality_quality_badge(quality)},
		{tool.Baro_t("등록 시각"), tool.Baro_time_cell(date)},
	})

	if low_reason != "" {
		body += tool.Baro_alert("warn",
			tool.Baro_t("이 도메인은 <b>")+tool.HTML_escape(low_reason)+tool.Baro_t("</b> 로 분류됩니다. 그 자체가 주제인 경우가 아니라면 다른 출처를 함께 붙여 주세요."))
	}

	if problem != "" {
		body += tool.Baro_alert("info", tool.Baro_t("표준 인용 형식에서 ")+tool.HTML_escape(problem)+tool.Baro_t(" 입니다. 아래에서 채워 주세요."))
	}

	// 원문 보존 안내
	archive_html := ""
	if archive != "" {
		archive_html = tool.Baro_alert("ok", tool.Baro_t("보존 링크가 등록되어 있습니다. ")+baro_quality_ext_link(archive))
	} else if strings.HasPrefix(raw, "http") {
		save_url := "https://web.archive.org/save/" + raw
		view_url := "https://web.archive.org/web/*/" + raw

		archive_html = tool.Baro_alert("info",
			tool.Baro_t("보존 링크가 없습니다. 아래 주소로 원문을 저장한 뒤, 만들어진 주소를 <b>원문 보존 링크</b> 칸에 넣어 주세요.<br>")+
				`<a href="`+tool.HTML_escape(save_url)+tool.Baro_t(`" rel="nofollow noopener" target="_blank">web.archive.org 에 저장하기</a> · `)+
				`<a href="`+tool.HTML_escape(view_url)+tool.Baro_t(`" rel="nofollow noopener" target="_blank">기존 보존본 찾기</a>`))
	} else {
		archive_html = tool.Baro_note(tool.Baro_t("주소가 없는 출처라 보존 링크를 만들 수 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("원문 보존 링크"), archive_html)

	if tool.Baro_can(db, config.IP, "review") {
		body += tool.Baro_card_ex(tool.Baro_t("출처 정보"), tool.Baro_t("검토 권한이 있는 사용자만 저장할 수 있습니다."),
			tool.Baro_form("/baro/source/edit/"+tool.Url_parser(id), form),
			tool.Baro_button("/baro/source", tool.Baro_t("출처 목록"), ""))
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("출처 정보는 누구나 볼 수 있지만, 손질하려면 검토 권한이 필요합니다."))
	}

	guide_rows := [][]string{}
	for _, row := range baro_quality_cite_format_guide() {
		guide_rows = append(guide_rows, []string{tool.HTML_escape(row[0]), tool.Baro_code(row[1])})
	}

	body += tool.Baro_card(tool.Baro_t("표준 인용 형식"), tool.Baro_table([]string{"종류", tool.Baro_t("형식")}, guide_rows, ""))

	return tool.Baro_page(db, config, tool.Baro_t("출처 손질"), body, baro_quality_menu())
}
