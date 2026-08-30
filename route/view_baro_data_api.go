package route

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  14-4 공개 읽기 API  (/baro/api)
//
//  전부 JSON 으로 응답한다.
//  격리 문서와 비공개 정보는 API 로 절대 나가지 않는다.
// ============================================================

// baro_data_api_window : 요청 제한 시간 창 (1시간)
const baro_data_api_window = 3600

// baro_data_api_endpoints : 엔드포인트 안내 목록 {주소, 설명, 예시}
func baro_data_api_endpoints() [][]string {
	return [][]string{
		{"/baro/api/doc?name=<문서>", tool.Baro_t("문서 하나의 본문·메타·분류·링크를 돌려줍니다."), "/baro/api/doc?name=바로위키"},
		{"/baro/api/search?q=<검색어>", tool.Baro_t("제목과 본문에서 검색합니다. (page 로 쪽 넘기기)"), "/baro/api/search?q=위키&page=1"},
		{"/baro/api/recent", tool.Baro_t("최근 변경 목록입니다. (limit 최대 100)"), "/baro/api/recent?limit=20"},
		{"/baro/api/category?name=<분류>", tool.Baro_t("분류에 속한 문서 목록입니다."), "/baro/api/category?name=분류:역사"},
		{"/baro/api/project", tool.Baro_t("프로젝트 목록과 진행률입니다."), "/baro/api/project?page=1"},
		{"/baro/api/user?name=<사용자>", tool.Baro_t("사용자의 공개 기여 요약입니다."), "/baro/api/user?name=example"},
	}
}

// baro_data_api_key_row : 봇 API 키 한 줄
type baro_data_api_key_row struct {
	ID       string
	User     string
	Name     string
	Scope    string
	Rate     int
	Status   string
	Date     string
	LastUsed string
}

// baro_data_api_key_find : 키 원본으로 등록된 키 찾기 (해시로만 비교한다)
func baro_data_api_key_find(db *sql.DB, raw_key string) (baro_data_api_key_row, bool) {
	row := baro_data_api_key_row{}
	if strings.TrimSpace(raw_key) == "" {
		return row, false
	}

	hash := tool.Sha224(raw_key)
	rate := ""

	found := tool.QueryRow_DB(
		db,
		"select id, user, name, scope, rate, status, date, last_used from baro_api_key where key_hash = ?",
		[]any{&row.ID, &row.User, &row.Name, &row.Scope, &rate, &row.Status, &row.Date, &row.LastUsed},
		hash,
	)

	if !found || row.Status != "active" {
		return row, false
	}

	row.Rate = tool.Str_to_int(rate)

	return row, true
}

// baro_data_api_limit : 이 요청에 적용할 시간당 호출 수와 제한 대상
func baro_data_api_limit(db *sql.DB, config tool.Config, raw_key string) (string, int, string) {
	base := tool.Baro_get_int(db, "api_rate_per_hour", 600)
	if base < 10 {
		base = 10
	}

	key_row, ok := baro_data_api_key_find(db, raw_key)
	if !ok {
		return "ip:" + tool.Sha224(config.IP), base, ""
	}

	limit := key_row.Rate
	if limit < base {
		limit = base * 5
	}

	tool.Exec_DB(db, "update baro_api_key set last_used = ? where id = ?", tool.Baro_now(), key_row.ID)

	return "key:" + key_row.ID, limit, key_row.ID
}

// baro_data_api_error : JSON 오류 응답
//
//	MediaWiki 처럼 오류도 HTTP 200 으로 돌려주고 ok 필드로 구분한다.
//	(중간 프록시가 4xx 본문을 잘라 버리는 일을 피하기 위해서다)
//	예외는 호출 제한 초과뿐이며, 그때만 429 를 돌려준다.
func baro_data_api_error(code string, message string) (string, int) {
	return baro_data_json(map[string]any{
		"ok":      false,
		"error":   code,
		"message": message,
		"date":    tool.Baro_now(),
	}), http.StatusOK
}

// View_baro_data_api_call : 공개 읽기 API 응답 (본문, HTTP 상태)
func View_baro_data_api_call(config tool.Config, kind string, query url.Values, raw_key string) (string, int) {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	// 기능 플래그로 끌 수 있게 한다.
	if !tool.Baro_flag_on(db, config.IP, "public_api") {
		return baro_data_api_error("api_disabled", tool.Baro_t("공개 읽기 API 가 꺼져 있습니다."))
	}

	who, limit, key_id := baro_data_api_limit(db, config, raw_key)

	if !tool.Baro_rate_ok(db, "baro_api", who, limit, baro_data_api_window) {
		return baro_data_json(map[string]any{
			"ok":       false,
			"error":    "rate_limited",
			"message":  tool.Baro_t("시간당 호출 수를 넘었습니다. 잠시 뒤 다시 시도하세요."),
			"limit":    limit,
			"window":   tool.Baro_t("1시간"),
			"key_used": key_id != "",
			"hint":     tool.Baro_t("봇 API 키를 발급받으면 제한이 올라갑니다. /baro/api/key"),
			"date":     tool.Baro_now(),
		}), http.StatusTooManyRequests
	}

	tool.Baro_metric_bump(db, "api_call", tool.Baro_today(), 1)
	tool.Baro_metric_bump(db, "api_"+kind, tool.Baro_today(), 1)

	switch kind {
	case "doc":
		return baro_data_api_doc(db, config, query)
	case "search":
		return baro_data_api_search(db, config, query)
	case "recent":
		return baro_data_api_recent(db, config, query)
	case "category":
		return baro_data_api_category(db, config, query)
	case "project":
		return baro_data_api_project(db, query)
	case "user":
		return baro_data_api_user(db, query)
	}

	return baro_data_api_error("not_found", tool.Baro_t("그런 엔드포인트가 없습니다. /baro/api 에서 목록을 확인하세요."))
}

// baro_data_like_word : LIKE 검색어 (% 와 _ 를 글자 그대로 찾게 한다)
//
//	검색어에 % 를 넣으면 문서 전체가 걸려 나오는 것을 막는다.
func baro_data_like_word(word string) string {
	replacer := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")

	return "%" + replacer.Replace(word) + "%"
}

// baro_data_api_limit_int : limit 값 정리
func baro_data_api_limit_int(data string, def int, max_value int) int {
	value := tool.Str_to_int(data)
	if value < 1 {
		value = def
	}
	if value > max_value {
		value = max_value
	}

	return value
}

// baro_data_api_doc : 문서 조회
func baro_data_api_doc(db *sql.DB, config tool.Config, query url.Values) (string, int) {
	name := tool.Namespace_normalize(baro_data_q(query, "name"))
	if name == "" {
		return baro_data_api_error("bad_request", tool.Baro_t("문서 이름(name)이 없습니다."))
	}

	// 격리 문서와 읽기 권한이 없는 문서는 존재 여부조차 알려주지 않는다.
	if !baro_data_doc_readable(db, config, name) {
		return baro_data_api_error("not_found", tool.Baro_t("그런 문서가 없습니다."))
	}

	text, found := baro_data_doc_text(db, name)
	if !found {
		return baro_data_api_error("not_found", tool.Baro_t("그런 문서가 없습니다."))
	}

	return baro_data_json(map[string]any{
		"ok": true,
		"doc": map[string]any{
			"title":       tool.Namespace_display(name),
			"raw_title":   name,
			"text":        text,
			"length":      tool.Get_len(text),
			"revision":    tool.Get_document_revision(db, name),
			"rev_count":   baro_data_doc_rev_count(db, name),
			"status":      tool.Baro_doc_status_get(db, name),
			"status_name": tool.Baro_doc_status_name(tool.Baro_doc_status_get(db, name)),
			"field":       tool.Baro_doc_field(db, name),
			"categories":  baro_data_doc_categories(db, name),
			"links":       baro_data_doc_links(db, name, 200),
			"stable":      baro_data_stable_info(db, name),
			"url":         "/w/" + name,
		},
		"license": baro_data_license(db),
		"date":    tool.Baro_now(),
	}), http.StatusOK
}

// baro_data_api_search : 검색
func baro_data_api_search(db *sql.DB, config tool.Config, query url.Values) (string, int) {
	word := baro_data_q(query, "q")
	if word == "" {
		return baro_data_api_error("bad_request", tool.Baro_t("검색어(q)가 없습니다."))
	}

	page_num := baro_data_page_num(baro_data_q(query, "page"))
	limit := baro_data_api_limit_int(baro_data_q(query, "limit"), 30, 100)
	offset := (page_num - 1) * limit

	like := baro_data_like_word(word)

	items := []map[string]any{}
	rows := tool.Query_DB(
		db,
		"select title, data from data where title like ? escape '!' or data like ? escape '!' order by title limit ?, ?",
		like, like, offset, limit,
	)
	defer rows.Close()

	for rows.Next() {
		title, text := "", ""
		if err := rows.Scan(&title, &text); err != nil {
			continue
		}

		if !baro_data_doc_readable(db, config, title) {
			continue
		}

		items = append(items, map[string]any{
			"title":       tool.Namespace_display(title),
			"raw_title":   title,
			"length":      tool.Get_len(text),
			"title_match": strings.Contains(title, word),
			"preview":     tool.Baro_trunc(tool.Baro_one_line(text), 120),
			"url":         "/w/" + title,
		})
	}

	return baro_data_json(map[string]any{
		"ok":      true,
		"query":   word,
		"page":    page_num,
		"limit":   limit,
		"count":   len(items),
		"results": items,
		"note":    tool.Baro_t("격리 문서와 읽기 권한이 없는 문서는 결과에서 빠집니다."),
		"date":    tool.Baro_now(),
	}), http.StatusOK
}

// baro_data_api_recent : 최근 변경
func baro_data_api_recent(db *sql.DB, config tool.Config, query url.Values) (string, int) {
	limit := baro_data_api_limit_int(baro_data_q(query, "limit"), 30, 100)
	page_num := baro_data_page_num(baro_data_q(query, "page"))
	offset := (page_num - 1) * limit

	// 읽을 수 없는 문서를 걸러 낸 뒤에 쪽을 나눈다.
	// (SQL 쪽에서 미리 잘라 내면 걸러진 만큼 앞뒤 쪽이 겹치거나 빠진다)
	need := offset + limit
	scan_max := need * 4
	if scan_max > 4000 {
		scan_max = 4000
	}

	items := []map[string]any{}
	seen := 0

	rows := tool.Query_DB(
		db,
		"select id, title, date, ip, send, leng, hide, type from history order by date desc, id + 0 desc limit ?",
		scan_max,
	)
	defer rows.Close()

	for rows.Next() {
		id, title, date, ip, send, leng, hide, kind := "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &title, &date, &ip, &send, &leng, &hide, &kind); err != nil {
			continue
		}

		if len(items) >= limit {
			break
		}

		if !baro_data_doc_readable(db, config, title) {
			continue
		}

		seen++
		if seen <= offset {
			continue
		}

		if hide == "on" {
			send = tool.Baro_t("(숨겨진 요약)")
		}

		user := ""
		anonymous := true
		if tool.Baro_is_user(ip) {
			user = ip
			anonymous = false
		}

		items = append(items, map[string]any{
			"title":     tool.Namespace_display(title),
			"raw_title": title,
			"rev":       id,
			"date":      date,
			"user":      user,
			"anonymous": anonymous,
			"summary":   send,
			"length":    tool.Str_to_int(leng),
			"kind":      kind,
			"url":       "/w/" + title,
		})
	}

	return baro_data_json(map[string]any{
		"ok":      true,
		"page":    page_num,
		"limit":   limit,
		"count":   len(items),
		"changes": items,
		"note":    tool.Baro_t("IP 편집자는 anonymous 로만 표시됩니다."),
		"date":    tool.Baro_now(),
	}), http.StatusOK
}

// baro_data_api_category : 분류
func baro_data_api_category(db *sql.DB, config tool.Config, query url.Values) (string, int) {
	name := tool.Namespace_normalize(baro_data_q(query, "name"))
	if name == "" {
		return baro_data_api_error("bad_request", tool.Baro_t("분류 이름(name)이 없습니다."))
	}

	page_num := baro_data_page_num(baro_data_q(query, "page"))
	limit := baro_data_api_limit_int(baro_data_q(query, "limit"), 50, 200)
	offset := (page_num - 1) * limit

	items := []map[string]any{}
	rows := tool.Query_DB(
		db,
		// 분류에 속한 문서 : back 표의 title 쪽에 분류를 맞춘다
		"select link from back where title = ? and type = 'cat' and exists (select 1 from data where data.title = back.link) order by link limit ?, ?",
		name, offset, limit,
	)
	defer rows.Close()

	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		if !baro_data_doc_readable(db, config, title) {
			continue
		}

		items = append(items, map[string]any{
			"title":     tool.Namespace_display(title),
			"raw_title": title,
			"status":    tool.Baro_doc_status_get(db, title),
			"url":       "/w/" + title,
		})
	}

	return baro_data_json(map[string]any{
		"ok":       true,
		"category": tool.Namespace_display(name),
		"page":     page_num,
		"limit":    limit,
		"count":    len(items),
		"docs":     items,
		"date":     tool.Baro_now(),
	}), http.StatusOK
}

// baro_data_api_project : 프로젝트 목록
func baro_data_api_project(db *sql.DB, query url.Values) (string, int) {
	page_num := baro_data_page_num(baro_data_q(query, "page"))
	limit := baro_data_api_limit_int(baro_data_q(query, "limit"), 30, 100)
	offset := (page_num - 1) * limit

	items := []map[string]any{}
	rows := tool.Query_DB(
		db,
		"select id, name, field, goal, scope, start_date, end_date, status, target_count, done_count, summary from baro_project order by id + 0 desc limit ?, ?",
		offset, limit,
	)
	defer rows.Close()

	for rows.Next() {
		id, name, field, goal, scope, start_date, end_date, status, target_count, done_count, summary := "", "", "", "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &name, &field, &goal, &scope, &start_date, &end_date, &status, &target_count, &done_count, &summary); err != nil {
			continue
		}

		done := tool.Str_to_int(done_count)
		total := tool.Str_to_int(target_count)

		items = append(items, map[string]any{
			"id":           id,
			"name":         name,
			"field":        field,
			"field_name":   tool.Baro_field_name(db, field),
			"goal":         goal,
			"scope":        scope,
			"start_date":   start_date,
			"end_date":     end_date,
			"status":       status,
			"target_count": total,
			"done_count":   done,
			"percent":      tool.Baro_pct(done, total),
			"summary":      summary,
			"url":          "/baro/project",
		})
	}

	return baro_data_json(map[string]any{
		"ok":       true,
		"page":     page_num,
		"limit":    limit,
		"count":    len(items),
		"projects": items,
		"date":     tool.Baro_now(),
	}), http.StatusOK
}

// baro_data_api_user : 사용자 공개 기여 (비공개 설정한 사용자는 제외)
func baro_data_api_user(db *sql.DB, query url.Values) (string, int) {
	name := baro_data_q(query, "name")
	if name == "" {
		return baro_data_api_error("bad_request", tool.Baro_t("사용자 이름(name)이 없습니다."))
	}

	if !tool.Baro_is_user(name) {
		return baro_data_api_error("not_found", tool.Baro_t("가입 사용자만 조회할 수 있습니다."))
	}

	if baro_data_user_private(db, name) {
		return baro_data_api_error("private", tool.Baro_t("이 사용자는 공개 조회를 원하지 않습니다."))
	}

	// 기여 종류별 수 (횟수 그대로가 아니라 행동별로 나눈다 — 0장 4번 원칙)
	kind_count := map[string]int{}
	total := 0

	rows := tool.Query_DB(db, "select kind from baro_contrib_event where user = ? limit 20000", name)
	for rows.Next() {
		kind := ""
		if err := rows.Scan(&kind); err != nil {
			continue
		}

		kind_count[kind]++
		total++
	}
	rows.Close()

	kinds := []map[string]any{}
	for _, row := range tool.Baro_contrib_kind_list() {
		if kind_count[row[0]] == 0 {
			continue
		}

		kinds = append(kinds, map[string]any{
			"kind": row[0], "name": row[1], "count": kind_count[row[0]],
		})
	}

	// 배지
	badges := []map[string]any{}
	badge_rows := tool.Query_DB(db, "select badge, date from baro_user_badge where user = ? order by id + 0 asc limit 100", name)
	for badge_rows.Next() {
		badge, date := "", ""
		if err := badge_rows.Scan(&badge, &date); err != nil {
			continue
		}

		badge_name := ""
		tool.QueryRow_DB(db, "select name from baro_badge where key_name = ?", []any{&badge_name}, badge)

		badges = append(badges, map[string]any{"key": badge, "name": badge_name, "date": date})
	}
	badge_rows.Close()

	// 공개된 감사만
	thanks := 0
	thanks_text := "0"
	tool.QueryRow_DB(db, "select count(*) from baro_thanks where to_user = ? and is_public = 'on'", []any{&thanks_text}, name)
	thanks = tool.Str_to_int(thanks_text)

	return baro_data_json(map[string]any{
		"ok": true,
		"user": map[string]any{
			"name":           name,
			"founder_number": tool.Baro_founder_number(db, name),
			"roles":          tool.Baro_user_roles(db, name),
			"interests":      tool.Baro_user_interest(db, name),
			"contrib_total":  total,
			"contrib_kinds":  kinds,
			"badges":         badges,
			"thanks_public":  thanks,
			"url":            "/baro/profile/" + name,
		},
		"note": tool.Baro_t("이메일 · IP · 비공개 정보는 담기지 않습니다."),
		"date": tool.Baro_now(),
	}), http.StatusOK
}

// View_baro_data_api : API 사용 안내 화면
func View_baro_data_api(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	base := tool.Baro_get_int(db, "api_rate_per_hour", 600)

	body := baro_data_tabs("/baro/api")
	body += tool.Baro_hero(
		tool.Baro_t("공개 읽기 API"),
		tool.Baro_t("바로위키의 공개 문서를 프로그램에서 바로 읽을 수 있습니다. 응답은 모두 JSON 입니다."),
		tool.Baro_note(tool.Baro_t("라이선스 : ")+baro_data_license(db)+tool.Baro_t(" · 재사용할 때는 출처를 표기해 주세요.")),
	)

	if !tool.Baro_flag_on(db, config.IP, "public_api") {
		body += tool.Baro_alert("danger", tool.Baro_t("지금은 공개 읽기 API 가 꺼져 있습니다. 아래 안내는 참고용입니다."))
	}

	// --- 엔드포인트 목록 ---
	rows := [][]string{}
	for _, row := range baro_data_api_endpoints() {
		rows = append(rows, []string{
			tool.Baro_code(row[0]),
			tool.HTML_escape(row[1]),
			`<a href="` + tool.HTML_escape(row[2]) + `">` + tool.HTML_escape(row[2]) + `</a>`,
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("엔드포인트"),
		tool.Baro_t("모두 GET 이고, 응답은 application/json 입니다."),
		tool.Baro_table([]string{tool.Baro_t("주소"), tool.Baro_t("설명"), tool.Baro_t("눌러 보기")}, rows, tool.Baro_t("엔드포인트가 없습니다.")),
		"",
	)

	// --- 예시 응답 ---
	body += tool.Baro_card_ex(
		tool.Baro_t("예시 응답"),
		"/baro/api/doc?name=바로위키",
		tool.Baro_code(`{
  "ok": true,
  "doc": {
    "title": "바로위키",
    "text": "`+tool.Baro_t(`= 개요 =\n...`)+`",
    "length": 1234,
    "revision": "12",
    "status": "normal",
    "categories": ["`+tool.Baro_t(`분류:위키`)+`"],
    "links": ["`+tool.Baro_t(`문서 A`)+`", "`+tool.Baro_t(`문서 B`)+`"],
    "url": "/w/바로위키"
  },
  "license": "`+baro_data_license(db)+`",
  "date": "`+tool.Baro_now()+`"
}`),
		tool.Baro_note("오류일 때는 {\"ok\": false, \"error\": \"...\", \"message\": \"...\"} 형태로 돌려줍니다."),
	)

	// --- 제한 정책 ---
	body += tool.Baro_card_ex(
		"호출 제한 정책",
		"같은 사람이 서버를 독차지하지 못하게 시간당 호출 수를 제한합니다.",
		tool.Baro_stat_row([][]string{
			{"기본 제한", tool.Baro_comma(base) + tool.Baro_t("회"), "1시간 기준"},
			{"키 사용 시", tool.Baro_comma(base*5) + tool.Baro_t("회"), "봇 API 키 발급 시"},
			{"초과 응답", "429", "Too Many Requests"},
		})+
			tool.Baro_list([]string{
				tool.Baro_t("제한을 넘으면 <code>429</code> 와 함께 <code>{\"ok\":false,\"error\":\"rate_limited\"}</code> 를 돌려줍니다."),
				tool.Baro_t("그 밖의 오류(없는 문서 · 빠진 인자 등)는 <code>200</code> 으로 오고 <code>ok</code> 가 <code>false</code> 입니다."),
				tool.Baro_t("키는 <code>?key=발급받은키</code> 또는 <code>X-Baro-Api-Key</code> 헤더로 보냅니다."),
				"제한 창은 1시간이며, 창이 지나면 자동으로 초기화됩니다.",
			}),
		"",
	)

	// --- 나가지 않는 정보 ---
	body += tool.Baro_card_ex(
		"API 로 나가지 않는 것",
		"0장 5번 원칙 : 근거는 공개하되 개인정보는 보호합니다.",
		tool.Baro_alert("warn", tool.Baro_t("아래 정보는 어떤 엔드포인트에서도 <b>나가지 않습니다</b>."))+
			tool.Baro_list([]string{
				"격리된 문서(baro_quarantine)의 본문과 존재 여부",
				"읽기 권한이 없는 문서",
				"IP 주소와 이메일 주소",
				"비공개 신고 내용과 사건의 내부 기록",
				"공개 조회를 거부한 사용자의 기여 요약",
			}),
		"",
	)

	// --- 키 안내 ---
	body += tool.Baro_card(
		"봇 API 키",
		tool.Baro_t(`<p>정기적으로 많이 호출해야 한다면 봇 API 키를 발급받으세요. 키가 있으면 시간당 제한이 올라갑니다.</p>`)+
			tool.Baro_button("/baro/api/key", "봇 API 키 발급 / 폐기", "primary"),
	)

	return tool.Baro_page(db, config, "공개 읽기 API", body, baro_data_menu())
}

// View_baro_data_api_key : 봇 API 키 발급 / 폐기
func View_baro_data_api_key(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if page, need := tool.Baro_login_wall(db, config, "/baro/api/key"); need {
		return page
	}

	issued := ""

	if values != nil {
		action := baro_data_q(values, "action")

		switch action {
		case "make":
			name := baro_data_q(values, "name")
			if name == "" {
				name = "이름 없는 키"
			}

			count := "0"
			tool.QueryRow_DB(db, "select count(*) from baro_api_key where user = ? and status = 'active'", []any{&count}, config.IP)

			if tool.Str_to_int(count) >= 5 {
				return baro_data_error_page(db, config, "봇 API 키", "쓰지 않는 키를 먼저 폐기해 주세요. 계정당 활성 키는 5개까지입니다.")
			}

			raw := "bw_" + tool.Get_random_key(40)
			base := tool.Baro_get_int(db, "api_rate_per_hour", 600)

			tool.Exec_DB(
				db,
				"insert into baro_api_key (id, user, key_hash, name, scope, rate, status, date, last_used) values (?, ?, ?, ?, 'read', ?, 'active', ?, '')",
				tool.Baro_next_id(db, "baro_api_key"), config.IP, tool.Sha224(raw), name, tool.Baro_itoa(base*5), tool.Baro_now(),
			)

			tool.Baro_audit(db, config, "api_key_make", name, "", "", "봇 API 키 발급")
			tool.Baro_notify(db, config.IP, "system", "봇 API 키를 발급했습니다", "/baro/api/key", name)

			issued = raw
		case "revoke":
			id := baro_data_q(values, "id")

			owner := ""
			tool.QueryRow_DB(db, "select user from baro_api_key where id = ?", []any{&owner}, id)

			if owner != config.IP && !tool.Baro_is_owner(db, config.IP) {
				return tool.Baro_deny(db, config, "내가 발급한 키만 폐기할 수 있습니다.")
			}

			tool.Exec_DB(db, "update baro_api_key set status = 'revoked' where id = ?", id)
			tool.Baro_audit(db, config, "api_key_revoke", id, "active", "revoked", "봇 API 키 폐기")

			return tool.Get_redirect("/baro/api/key")
		default:
			return tool.Get_redirect("/baro/api/key")
		}
	}

	body := baro_data_tabs("/baro/api")
	body += tool.Baro_crumb([][]string{{"/baro/api", "공개 읽기 API"}, {"", "봇 API 키"}})

	// 방금 발급한 키는 여기서 한 번만 보여 준다.
	if issued != "" {
		body += tool.Baro_alert("ok",
			tool.Baro_t(`<b>키를 발급했습니다. 이 화면을 벗어나면 다시 볼 수 없습니다.</b> 지금 안전한 곳에 옮겨 두세요.`))
		body += tool.Baro_card("발급된 키 (한 번만 표시)",
			tool.Baro_code(issued)+
				tool.Baro_note("서버에는 이 값의 SHA224 해시만 저장됩니다. 잃어버리면 폐기하고 새로 발급해야 합니다.")+
				tool.Baro_code("curl -H \"X-Baro-Api-Key: "+issued+"\" http://<서버주소>/baro/api/recent"))
	}

	body += tool.Baro_alert("info",
		tool.Baro_t("키는 원본을 저장하지 않고 <code>SHA224</code> 해시로만 보관합니다. 발급 순간에만 원본을 보여 줍니다."))

	// 발급 폼
	form := tool.Baro_input("name", "키 이름", "", "예) 통계 수집 봇", "어디에 쓰는 키인지 적어 두세요.")
	form += tool.Baro_hidden("action", "make")
	form += tool.Baro_submit("새 키 발급", "primary")

	body += tool.Baro_card_ex("새 키 발급", "계정당 활성 키 5개까지", tool.Baro_form("/baro/api/key", form), "")

	// 내 키 목록
	rows := [][]string{}
	list := tool.Query_DB(
		db,
		"select id, name, scope, rate, status, date, last_used from baro_api_key where user = ? order by id + 0 desc limit 100",
		config.IP,
	)
	defer list.Close()

	for list.Next() {
		id, name, scope, rate, status, date, last_used := "", "", "", "", "", "", ""
		if err := list.Scan(&id, &name, &scope, &rate, &status, &date, &last_used); err != nil {
			continue
		}

		status_kind := "muted"
		if status == "active" {
			status_kind = "ok"
		}

		action_html := `<span class="bw_muted">-</span>`
		if status == "active" {
			revoke := tool.Baro_hidden("action", "revoke") + tool.Baro_hidden("id", id) + tool.Baro_submit("폐기", "danger")
			action_html = tool.Baro_form("/baro/api/key", revoke)
		}

		rows = append(rows, []string{
			tool.HTML_escape(id),
			tool.HTML_escape(name),
			tool.HTML_escape(scope),
			tool.Baro_comma(tool.Str_to_int(rate)) + " 회/시간",
			tool.Baro_badge(status, status_kind),
			tool.Baro_time_cell(date),
			tool.Baro_time_cell(last_used),
			action_html,
		})
	}

	table := tool.Baro_table(
		[]string{"번호", "이름", "범위", "제한", "상태", "발급", "마지막 사용", "관리"},
		rows,
		"발급받은 키가 없습니다.",
	)

	if len(rows) == 0 {
		table = tool.Baro_empty("아직 발급받은 키가 없습니다. 위에서 첫 키를 만들어 보세요.")
	}

	body += tool.Baro_card_ex("내 API 키", "폐기한 키는 즉시 막힙니다.", table, "")

	body += tool.Baro_button("/baro/api", "API 안내로 돌아가기", "")

	return tool.Baro_page(db, config, "봇 API 키", body, baro_data_menu())
}
