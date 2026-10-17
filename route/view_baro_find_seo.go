package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  5-5 외부 검색 최적화
//
//  문서별 제목·설명 / 공유 이미지 / 사이트맵 / 대표 URL 고정 /
//  중복 URL 정리 / 서버 렌더링 확인 / 노출·제외 구분
// ============================================================

// baro_find_site_url : 사이트맵과 대표 URL 에 쓸 기준 주소
func baro_find_site_url(db *sql.DB) string {
	return strings.TrimRight(tool.Baro_get_def(db, "find_site_url", ""), "/")
}

// baro_find_xml_escape : XML 안전 문자열
func baro_find_xml_escape(data string) string {
	data = strings.ReplaceAll(data, "&", "&amp;")
	data = strings.ReplaceAll(data, "<", "&lt;")
	data = strings.ReplaceAll(data, ">", "&gt;")
	data = strings.ReplaceAll(data, `"`, "&quot;")
	data = strings.ReplaceAll(data, "'", "&apos;")

	return data
}

// baro_find_seo_noindex : 검색 엔진에서 빼야 하는 문서인지
func baro_find_seo_noindex(db *sql.DB, title string, hidden map[string]bool) (bool, string) {
	if baro_find_is_trash(title) {
		return true, tool.Baro_t("휴지통 문서")
	}
	if baro_find_is_sandbox(title) {
		return true, tool.Baro_t("사용자 연습장")
	}
	if hidden[title] {
		return true, tool.Baro_t("격리 문서")
	}
	if tool.Baro_doc_meta_get(db, title, "noindex") == "on" {
		return true, tool.Baro_t("문서별 제외 설정")
	}

	return false, ""
}

// baro_find_robots_rules : robots.txt 제외 규칙
func baro_find_robots_rules() [][]string {
	return [][]string{
		{"/edit/", tool.Baro_t("편집 화면 — 같은 내용이 두 번 색인되지 않게 막습니다.")},
		{"/history/", tool.Baro_t("역사 화면 — 옛 판이 본문보다 위에 뜨는 것을 막습니다.")},
		{"/delete/", tool.Baro_t("삭제 화면")},
		{"/move/", tool.Baro_t("이동 화면")},
		{"/acl/", tool.Baro_t("권한 화면")},
		{"/trash", tool.Baro_t("휴지통 목록")},
		{"/file_trash", tool.Baro_t("파일 휴지통 목록")},
		{"/w/trash:", tool.Baro_t("휴지통 이름공간 문서")},
		{"/w/filetrash:", tool.Baro_t("파일 휴지통 이름공간 문서")},
		{"/login", tool.Baro_t("로그인")},
		{"/register", "가입"},
		{"/search", tool.Baro_t("검색 결과 — 검색 결과가 색인되면 중복 문서가 늘어납니다.")},
		{"/baro/search", tool.Baro_t("바로위키 검색 결과")},
		{"/baro/admin", tool.Baro_t("백오피스")},
		{"/vote", tool.Baro_t("투표 화면")},
	}
}

// baro_find_robots_text : robots.txt 본문 생성
func baro_find_robots_text(db *sql.DB) string {
	base := baro_find_site_url(db)
	hidden := baro_find_hidden_set(db)

	out := "User-agent: *\n"
	for _, rule := range baro_find_robots_rules() {
		out += "Disallow: " + rule[0] + "\n"
	}

	// 격리 문서는 개별로 막는다. (2번 원칙 : 지우지 않고 감춘다)
	count := 0
	for title := range hidden {
		if count >= 100 {
			out += tool.Baro_t("# 격리 문서가 100건을 넘어 나머지는 사이트맵에서만 제외합니다.\n")
			break
		}

		out += "Disallow: /w/" + tool.Doc_url(title) + "\n"
		count++
	}

	out += "Allow: /w/\n"
	if base != "" {
		out += "\nSitemap: " + base + "/baro/doc/sitemap\n"
	} else {
		out += tool.Baro_t("\n# 기준 주소가 설정되지 않아 Sitemap 줄을 넣지 못했습니다.\n")
	}

	return out
}

// View_baro_find_seo : 외부 검색 최적화 개요 (/baro/doc/seo)
func View_baro_find_seo(config tool.Config, page string, query url.Values, post url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if post != nil {
		if !tool.Baro_can(db, config.IP, "system") {
			return tool.Baro_deny(db, config, tool.Baro_t("기준 주소를 바꾸려면 시스템 관리 권한이 필요합니다."))
		}

		before := baro_find_site_url(db)
		after := strings.TrimRight(baro_find_val(post, "site_url"), "/")

		tool.Baro_put(db, "find_site_url", after)
		tool.Baro_audit(db, config, "seo_site_url", "find_site_url", before, after, "사이트맵 기준 주소 변경")
		tool.Baro_public_log(db, "seo_site_url", config.IP, "", "사이트맵 기준 주소 변경", after)

		return tool.Get_redirect("/baro/doc/seo")
	}

	keyword := baro_find_val(query, "q")

	page_int := baro_find_page_int(page)
	if raw_page := baro_find_val(query, "page"); raw_page != "" {
		page_int = baro_find_page_int(raw_page)
	}

	offset := (page_int - 1) * tool.Baro_page_size
	hidden := baro_find_hidden_set(db)

	// 문서 목록 (설명이 채워졌는지 한눈에)
	sql_text := "select title from data where 1 = 1"
	args := []any{}
	if keyword != "" {
		sql_text += " and lower(title) like ?"
		args = append(args, "%"+strings.ToLower(keyword)+"%")
	}
	sql_text += " order by title limit ?, ?"
	args = append(args, offset, tool.Baro_page_size)

	rows := tool.Query_DB(db, sql_text, args...)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		meta := baro_find_meta_all(db, title)
		no_index, reason := baro_find_seo_noindex(db, title, hidden)

		state := tool.Baro_badge(tool.Baro_t("색인 허용"), "ok")
		if no_index {
			state = tool.Baro_badge(tool.Baro_t("색인 제외"), "danger") + ` <span class="bw_note">` + tool.HTML_escape(reason) + `</span>`
		}

		desc := tool.HTML_escape(tool.Baro_trunc(meta["description"], 60))
		if desc == "" {
			desc = tool.Baro_badge(tool.Baro_t("설명 없음"), "warn")
		}

		canonical := tool.HTML_escape(meta["canonical"])
		if canonical == "" {
			canonical = tool.Baro_t(`<span class="bw_muted">기본 (/w/문서)</span>`)
		}

		table_rows = append(table_rows, []string{
			tool.Baro_doc_link(title),
			desc,
			canonical,
			state,
			tool.Baro_button("/baro/doc/seo/"+tool.Url_parser(title), "설정", ""),
		})
	}

	// 통계
	doc_count, desc_count, noindex_count, canonical_count, map_count, back_count := 0, 0, 0, 0, 0, 0
	tool.QueryRow_DB(db, "select count(*) from data", []any{&doc_count})
	tool.QueryRow_DB(db, "select count(*) from baro_doc_meta where name = 'description' and not data = ''", []any{&desc_count})
	tool.QueryRow_DB(db, "select count(*) from baro_doc_meta where name = 'noindex' and data = 'on'", []any{&noindex_count})
	tool.QueryRow_DB(db, "select count(*) from baro_doc_meta where name = 'canonical' and not data = ''", []any{&canonical_count})
	tool.QueryRow_DB(db, "select count(*) from baro_url_map", []any{&map_count})
	tool.QueryRow_DB(db, "select count(*) from back", []any{&back_count})

	body := baro_find_tabs("/baro/doc/seo")
	body += tool.Baro_hero(tool.Baro_t("외부 검색 최적화"),
		tool.Baro_t("검색 엔진에 무엇을 보이고 무엇을 감출지 정합니다."),
		tool.Baro_button("/baro/doc/robots", tool.Baro_t("robots 규칙 보기"), "primary")+" "+
			tool.Baro_button("/baro/doc/sitemap", tool.Baro_t("sitemap.xml 내려받기"), "")+" "+
			tool.Baro_button("/baro/doc/meta?name=description", tool.Baro_t("메타 설명 모아 보기"), ""))

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 문서"), tool.Baro_comma(doc_count), ""},
		{tool.Baro_t("메타 설명 있음"), tool.Baro_comma(desc_count), tool.Baro_pct_text(desc_count, doc_count)},
		{tool.Baro_t("색인 제외 지정"), tool.Baro_comma(noindex_count), tool.Baro_t("문서별 설정")},
		{tool.Baro_t("대표 URL 고정"), tool.Baro_comma(canonical_count), ""},
		{tool.Baro_t("격리 문서"), tool.Baro_comma(len(hidden)), tool.Baro_t("자동 제외")},
		{tool.Baro_t("URL 매핑"), tool.Baro_comma(map_count), tool.Baro_t("이전 주소 → 새 주소")},
	})

	// 서버 렌더링 확인
	render_body := tool.Baro_alert("ok",
		tool.Baro_t(`이 위키는 <b>서버에서 본문을 렌더링</b>합니다. 문서 본문은 <code>data</code> 표에 원본으로 저장되고, `)+
			tool.Baro_t(`요청이 오면 서버(Go + pongo2)가 HTML 로 만들어 내려 줍니다. 자바스크립트를 끈 검색 엔진 크롤러도 본문을 그대로 읽습니다.`))
	render_body += tool.Baro_dl([][]string{
		{tool.Baro_t("본문 저장 위치"), tool.Baro_t(`<code>data.data</code> (원본 마크업)`)},
		{tool.Baro_t("렌더링 시점"), tool.Baro_t("요청 시 서버에서 (클라이언트 렌더링 아님)")},
		{tool.Baro_t("역링크 색인 수"), tool.Baro_comma(back_count) + tool.Baro_t("건 — 서버가 실제로 본문을 해석해 남긴 기록입니다.")},
		{tool.Baro_t("확인 방법"), tool.Baro_t(`문서 화면에서 "페이지 소스 보기" 를 하면 본문 글자가 그대로 들어 있습니다.`)},
	})

	body += tool.Baro_card(tool.Baro_t("문서 본문 서버 렌더링"), render_body)

	// 기준 주소
	site_url := baro_find_site_url(db)
	url_body := ""
	if site_url == "" {
		url_body += tool.Baro_alert("warn", tool.Baro_t("기준 주소가 비어 있어 사이트맵의 주소가 상대 경로로 나갑니다. 검색 엔진에 제출하려면 기준 주소를 채워 주세요."))
	} else {
		url_body += tool.Baro_alert("ok", tool.Baro_t("기준 주소 : <b>")+tool.HTML_escape(site_url)+"</b>")
	}

	if tool.Baro_can(db, config.IP, "system") {
		form := tool.Baro_input("site_url", tool.Baro_t("기준 주소"), site_url, "https://example.com", tool.Baro_t("끝에 / 는 빼고 적습니다. 사이트맵과 대표 URL 에 쓰입니다."))
		form += `<div class="bw_f">` + tool.Baro_submit("저장", "primary") + `</div>`
		url_body += tool.Baro_form("/baro/doc/seo", form)
	} else {
		url_body += tool.Baro_note(tool.Baro_t("기준 주소는 시스템 관리 권한이 있는 운영자만 바꿀 수 있습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("사이트 기준 주소"), url_body)

	// 중복 URL 정리
	dup_rows := [][]string{}
	map_rows := tool.Query_DB(db, "select old_url, new_url, kind, date from baro_url_map order by id + 0 desc limit 30")
	defer map_rows.Close()

	for map_rows.Next() {
		old_url, new_url, kind, date := "", "", "", ""
		if err := map_rows.Scan(&old_url, &new_url, &kind, &date); err != nil {
			continue
		}

		dup_rows = append(dup_rows, []string{
			tool.HTML_escape(old_url),
			tool.HTML_escape(new_url),
			tool.HTML_escape(kind),
			tool.Baro_time_cell(date),
		})
	}

	canon_rows := [][]string{}
	canon_query := tool.Query_DB(db, "select title, data from baro_doc_meta where name = 'canonical' and not data = '' limit 30")
	defer canon_query.Close()

	for canon_query.Next() {
		title, data := "", ""
		if err := canon_query.Scan(&title, &data); err != nil {
			continue
		}

		canon_rows = append(canon_rows, []string{
			tool.Baro_doc_link(title),
			tool.HTML_escape(data),
			tool.Baro_button("/baro/doc/seo/"+tool.Url_parser(title), "수정", ""),
		})
	}

	body += tool.Baro_card_ex(tool.Baro_t("중복 URL 정리"), tool.Baro_t("같은 내용이 여러 주소로 색인되지 않게 합니다"),
		tool.Baro_section(tool.Baro_t("대표 URL 을 고정한 문서"),
			tool.Baro_table([]string{"문서", tool.Baro_t("대표 URL"), "관리"}, canon_rows, tool.Baro_t("대표 URL 을 따로 고정한 문서가 없습니다. (기본값은 /w/문서 입니다)")))+
			tool.Baro_section(tool.Baro_t("등록된 이전 주소 매핑 (읽기 전용)"),
				tool.Baro_table([]string{tool.Baro_t("이전 주소"), tool.Baro_t("새 주소"), "종류", tool.Baro_t("등록")}, dup_rows, tool.Baro_t("등록된 URL 매핑이 없습니다."))+
					tool.Baro_note(tool.Baro_t("URL 매핑은 문서 이전(13장) 화면에서 등록합니다. 여기서는 색인에 미치는 영향만 확인합니다."))),
		"")

	// 문서 목록
	filter := tool.Baro_input("q", tool.Baro_t("문서 검색"), keyword, tool.Baro_t("문서 이름 일부"), "")
	filter += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("찾기"), "primary") + `</div>`

	page_url := url.Values{}
	page_url.Set("q", keyword)

	list_html := tool.Baro_table([]string{"문서", tool.Baro_t("메타 설명"), tool.Baro_t("대표 URL"), tool.Baro_t("색인"), "관리"}, table_rows, "")
	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("조건에 맞는 문서가 없습니다."))
	}
	list_html += tool.Baro_paging(page_int, len(table_rows), tool.Baro_page_size, "/baro/doc/seo?"+page_url.Encode()+"&page={}")

	body += tool.Baro_card(tool.Baro_t("문서별 설정"), `<form class="bw_form" method="get" action="/baro/doc/seo">`+filter+`</form>`+list_html)

	return tool.Baro_page(db, config, tool.Baro_t("외부 검색 최적화"), body, baro_find_menu())
}

// View_baro_find_seo_doc : 문서별 외부 검색 설정 (/baro/doc/seo/<문서>)
func View_baro_find_seo_doc(config tool.Config, doc_name string, post url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	doc_name = tool.Namespace_normalize(strings.TrimSpace(doc_name))
	if doc_name == "" {
		return tool.Get_redirect("/baro/doc/seo")
	}

	if post != nil {
		if wall, need := tool.Baro_login_wall(db, config, "/baro/doc/seo"); need {
			return wall
		}
		if tool.Baro_read_only(db) {
			return tool.Baro_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db))
		}
		if !baro_find_doc_exists(db, doc_name) {
			return tool.Get_redirect("/baro/doc/seo")
		}

		before := tool.Baro_json_enc(baro_find_meta_all(db, doc_name))

		tool.Baro_doc_meta_set(db, doc_name, "description", tool.Baro_trunc(tool.Baro_one_line(baro_find_val(post, "description")), 160))
		tool.Baro_doc_meta_set(db, doc_name, "og_image", tool.Baro_trunc(baro_find_val(post, "og_image"), 300))
		tool.Baro_doc_meta_set(db, doc_name, "canonical", tool.Baro_trunc(baro_find_val(post, "canonical"), 300))

		noindex := ""
		if baro_find_val(post, "noindex") == "on" {
			noindex = "on"
		}
		tool.Baro_doc_meta_set(db, doc_name, "noindex", noindex)

		after := tool.Baro_json_enc(baro_find_meta_all(db, doc_name))
		tool.Baro_audit(db, config, "seo_doc_edit", doc_name, before, after, "외부 검색 설정 변경")
		tool.Baro_public_log(db, "seo_doc_edit", config.IP, doc_name, "외부 검색 설정 변경", "색인 제외 : "+noindex)

		return tool.Get_redirect("/baro/doc/seo/" + tool.Url_parser(doc_name))
	}

	body := baro_find_tabs("/baro/doc/seo")
	body += tool.Baro_crumb([][]string{
		{"/baro/doc/seo", tool.Baro_t("외부 검색 최적화")},
		{"", tool.Namespace_display(doc_name)},
	})

	if !baro_find_doc_exists(db, doc_name) {
		body += tool.Baro_alert("warn", `<b>`+tool.HTML_escape(tool.Namespace_display(doc_name))+tool.Baro_t(`</b> 문서가 없습니다.`))
		body += tool.Baro_card(tool.Baro_t("무엇을 할 수 있나요"),
			tool.Baro_button("/edit/"+tool.Url_parser(doc_name), "이 문서 만들기", "primary")+" "+
				tool.Baro_button("/baro/doc/seo", tool.Baro_t("외부 검색 목록으로"), ""))

		return tool.Baro_page(db, config, tool.Baro_t("외부 검색 : ")+tool.Namespace_display(doc_name), body, baro_find_menu())
	}

	meta := baro_find_meta_all(db, doc_name)
	hidden := baro_find_hidden_set(db)
	no_index, reason := baro_find_seo_noindex(db, doc_name, hidden)

	raw := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&raw}, doc_name)

	auto_desc := tool.Baro_trunc(tool.Baro_one_line(raw), 160)
	site_url := baro_find_site_url(db)
	default_url := "/w/" + tool.Doc_url(doc_name)
	if site_url != "" {
		default_url = site_url + default_url
	}

	preview_title := tool.Namespace_display(doc_name) + " - " + tool.Baro_get_def(db, "wiki_name", "바로위키")
	preview_desc := meta["description"]
	if preview_desc == "" {
		preview_desc = auto_desc
	}

	preview := `<div class="bw_card_b"><div style="font-size:1.1em;color:#1a0dab">` + tool.HTML_escape(preview_title) + `</div>`
	preview += `<div style="color:#006621">` + tool.HTML_escape(default_url) + `</div>`
	preview += `<div>` + tool.HTML_escape(preview_desc) + `</div></div>`

	body += tool.Baro_hero(tool.Namespace_display(doc_name), tool.Baro_t("이 문서가 검색 결과에 어떻게 보일지 정합니다."),
		tool.Baro_button("/w/" + tool.Doc_url(doc_name), "문서 보기", "primary")+" "+
			tool.Baro_button("/baro/doc/alias/"+tool.Url_parser(doc_name), "별칭·인물 정보", ""))

	if no_index {
		body += tool.Baro_alert("warn", tool.Baro_t("이 문서는 지금 검색 엔진에서 제외됩니다. 사유 : <b>")+tool.HTML_escape(reason)+"</b>")
	} else {
		body += tool.Baro_alert("ok", tool.Baro_t("이 문서는 검색 엔진에 노출됩니다."))
	}

	body += tool.Baro_card(tool.Baro_t("검색 결과 미리 보기"), preview)

	if tool.Baro_is_user(config.IP) {
		form := tool.Baro_textarea("description", tool.Baro_t("메타 설명"), meta["description"], 3, auto_desc,
			tool.Baro_t("비워 두면 본문 첫 부분이 자동으로 쓰입니다. 160자 안쪽이 좋습니다."))
		form += tool.Baro_input("og_image", tool.Baro_t("공유 이미지 주소"), meta["og_image"], "/image/example.jpg",
			tool.Baro_t("카카오톡·트위터 등에 링크를 붙였을 때 보이는 그림입니다. 파일 문서를 올린 뒤 그 주소를 적습니다."))
		form += tool.Baro_input("canonical", tool.Baro_t("대표 URL"), meta["canonical"], default_url,
			tool.Baro_t("같은 내용이 여러 주소로 있을 때 어느 쪽을 원본으로 볼지 고정합니다. 비우면 기본값을 씁니다."))
		form += tool.Baro_checkbox("noindex", "검색 엔진에서 제외", "on", meta["noindex"] == "on",
			tool.Baro_t("개인정보 우려나 초안 상태일 때 켭니다. 문서를 지우지 않고 노출만 막습니다."))
		form += `<div class="bw_f">` + tool.Baro_submit("저장", "primary") + `</div>`

		body += tool.Baro_card("설정", tool.Baro_form("/baro/doc/seo/"+tool.Url_parser(doc_name), form))
	} else {
		body += tool.Baro_card("설정", tool.Baro_alert("info",
			tool.Baro_t("외부 검색 설정은 로그인한 기여자만 바꿀 수 있습니다. ")+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary")))
	}

	body += tool.Baro_card(tool.Baro_t("지금 값"), tool.Baro_dl([][]string{
		{tool.Baro_t("메타 설명"), tool.HTML_escape(meta["description"])},
		{tool.Baro_t("공유 이미지"), tool.HTML_escape(meta["og_image"])},
		{tool.Baro_t("대표 URL"), tool.HTML_escape(meta["canonical"])},
		{tool.Baro_t("색인 제외"), tool.HTML_escape(meta["noindex"])},
		{tool.Baro_t("문서 상태"), tool.Baro_status_badge(tool.Baro_doc_status_get(db, doc_name))},
	}))

	return tool.Baro_page(db, config, tool.Baro_t("외부 검색 : ")+tool.Namespace_display(doc_name), body, baro_find_menu())
}

// View_baro_find_robots : 검색 엔진 제외 규칙 미리 보기 (/baro/doc/robots)
func View_baro_find_robots(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	hidden := baro_find_hidden_set(db)

	rule_rows := [][]string{}
	for _, rule := range baro_find_robots_rules() {
		rule_rows = append(rule_rows, []string{
			tool.Baro_code(rule[0]),
			tool.HTML_escape(rule[1]),
		})
	}

	// 실제로 제외되는 문서 몇 개를 보여 준다.
	sample_rows := [][]string{}
	count := 0
	for title := range hidden {
		if count >= 20 {
			break
		}

		sample_rows = append(sample_rows, []string{
			tool.HTML_escape(tool.Namespace_display(title)),
			tool.Baro_badge(tool.Baro_t("격리 문서"), "danger"),
		})
		count++
	}

	sandbox_rows := [][]string{}
	rows := tool.Query_DB(db, "select title from data where lower(title) like 'user:%' or lower(title) like 'ipuser:%' limit 500")
	defer rows.Close()

	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}
		if !baro_find_is_sandbox(title) || len(sandbox_rows) >= 20 {
			continue
		}

		sandbox_rows = append(sandbox_rows, []string{
			tool.HTML_escape(tool.Namespace_display(title)),
			tool.Baro_badge(tool.Baro_t("사용자 연습장"), "warn"),
		})
	}

	body := baro_find_tabs("/baro/doc/seo")
	body += tool.Baro_crumb([][]string{
		{"/baro/doc/seo", tool.Baro_t("외부 검색 최적화")},
		{"", tool.Baro_t("robots 규칙")},
	})
	body += tool.Baro_hero(tool.Baro_t("검색 엔진 제외 규칙"),
		tool.Baro_t("어떤 주소를 검색 엔진에서 막을지 미리 확인합니다."),
		tool.Baro_button("/baro/doc/sitemap", tool.Baro_t("sitemap.xml 보기"), "primary"))

	body += tool.Baro_card(tool.Baro_t("규칙 목록"),
		tool.Baro_table([]string{tool.Baro_t("막는 주소"), tool.Baro_t("이유")}, rule_rows, ""))

	body += tool.Baro_card_ex(tool.Baro_t("격리 문서 (개별 제외)"), tool.Baro_t("삭제하지 않고 노출만 막습니다"),
		tool.Baro_table([]string{"문서", tool.Baro_t("구분")}, sample_rows, tool.Baro_t("격리 중인 문서가 없습니다.")),
		tool.Baro_note(tool.Baro_t("격리는 되돌릴 수 있는 조치입니다. 격리가 풀리면 자동으로 다시 색인됩니다.")))

	body += tool.Baro_card_ex(tool.Baro_t("사용자 연습장 (자동 제외)"), tool.Baro_t("연습 중인 글이 검색 결과에 뜨지 않게 합니다"),
		tool.Baro_table([]string{"문서", tool.Baro_t("구분")}, sandbox_rows, tool.Baro_t("연습장 문서가 없습니다.")), "")

	body += tool.Baro_card(tool.Baro_t("robots.txt 로 내보낼 내용"), tool.Baro_code(baro_find_robots_text(db))+
		tool.Baro_note(tool.Baro_t("이 내용을 웹 서버의 /robots.txt 로 올리면 그대로 적용됩니다.")))

	return tool.Baro_page(db, config, tool.Baro_t("검색 엔진 제외 규칙"), body, baro_find_menu())
}

// View_baro_find_sitemap_xml : sitemap.xml 생성 (/baro/doc/sitemap)
func View_baro_find_sitemap_xml(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = config

	base := baro_find_site_url(db)
	hidden := baro_find_hidden_set(db)
	edit_map := baro_find_last_edit_map(db)

	out := `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
	out += `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n"

	rows := tool.Query_DB(db, "select title from data order by title limit 5000")
	defer rows.Close()

	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		no_index, _ := baro_find_seo_noindex(db, title, hidden)
		if no_index {
			continue
		}

		loc := base + "/w/" + tool.Doc_url(title)
		if canonical := tool.Baro_doc_meta_get(db, title, "canonical"); canonical != "" {
			loc = canonical
		}

		out += "  <url>\n"
		out += "    <loc>" + baro_find_xml_escape(loc) + "</loc>\n"

		if date := edit_map[title]; len(date) >= 10 {
			out += "    <lastmod>" + baro_find_xml_escape(date[:10]) + "</lastmod>\n"
		}

		out += "    <changefreq>weekly</changefreq>\n"
		out += "  </url>\n"
	}

	out += "</urlset>\n"

	return out
}
