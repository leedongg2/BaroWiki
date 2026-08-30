package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

// ============================================================
//  14-1 문서 내보내기  (/baro/export)
//
//  "기여를 보존한다" 고 말하려면 운영자 서버 안에만 가둬두면 안 된다.
//  누구나 자기 기여와 문서를 실제 파일로 가져갈 수 있어야 한다.
// ============================================================

// baro_data_export_kind_options : 내보내기 형식 고르기
func baro_data_export_kind_options() [][]string {
	return [][]string{
		{"raw", tool.Baro_t("원문 (.txt) — 위키 문법 그대로")},
		{"json", tool.Baro_t("문서 JSON (.json) — 본문 + 메타 + 출처")},
		{"history", tool.Baro_t("문서 역사 (.json) — 판 번호·시각·기여자·요약·길이")},
		{"pdf", tool.Baro_t("인쇄용 화면 — 브라우저에서 PDF 로 저장")},
	}
}

// View_baro_data_export : 내보내기 고르기 화면 + 내보내기 이력
func View_baro_data_export(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	// 쓰기 동작 : 어떤 파일을 받을지 고른 뒤 해당 주소로 보낸다.
	if values != nil {
		return baro_data_export_post(db, config, values)
	}

	page_num := baro_data_page_num(page)
	offset := (page_num - 1) * baro_data_page_size

	body := baro_data_tabs("/baro/export")
	body += tool.Baro_hero(
		tool.Baro_t("데이터 내보내기"),
		tool.Baro_t("바로위키의 문서와 내 기여 기록은 언제든 파일로 가져갈 수 있습니다."),
		tool.Baro_note(tool.Baro_t("라이선스 : ")+baro_data_license(db)+tool.Baro_t(" · 내려받은 자료를 다시 쓸 때는 출처를 표기해 주세요.")),
	)

	body += tool.Baro_alert("info",
		tool.Baro_t(`받은 파일은 <b>실제 내려받기 파일</b>입니다. 원문은 <code>.txt</code>, 역사·문서·프로젝트는 `)+
			tool.Baro_t(`<code>.json</code>, 인쇄용은 브라우저의 "PDF 로 저장" 을 쓰는 화면으로 열립니다.`),
	)

	// --- 문서 하나 내보내기 ---
	doc_form := tool.Baro_input("doc", tool.Baro_t("문서 이름"), "", tool.Baro_t("예) 바로위키"), tool.Baro_t("정확한 문서 이름을 적어 주세요."))
	doc_form += tool.Baro_select("kind", tool.Baro_t("내보낼 형식"), baro_data_export_kind_options(), "raw", "")
	doc_form += tool.Baro_hidden("action", "doc")
	doc_form += tool.Baro_submit("내려받기", "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("문서 하나 내보내기"),
		tool.Baro_t("원문 · 역사 · JSON · 인쇄용 중에서 고릅니다."),
		tool.Baro_form("/baro/export", doc_form),
		tool.Baro_note(tool.Baro_t("격리된 문서와 읽기 권한이 없는 문서는 내보낼 수 없습니다.")),
	)

	// --- 프로젝트 전체 내보내기 ---
	project_options := baro_data_project_options(db)
	if len(project_options) == 0 {
		body += tool.Baro_card(tool.Baro_t("프로젝트 전체 내보내기"), tool.Baro_empty(tool.Baro_t("아직 등록된 프로젝트가 없습니다.")))
	} else {
		project_form := tool.Baro_select("id", "프로젝트", project_options, "", tool.Baro_t("목표 · 과제 · 문서 목록을 JSON 으로 묶어 내려받습니다."))
		project_form += tool.Baro_hidden("action", "project")
		project_form += tool.Baro_submit(tool.Baro_t("프로젝트 JSON 내려받기"), "")

		body += tool.Baro_card_ex(
			tool.Baro_t("프로젝트 전체 내보내기"),
			tool.Baro_t("프로젝트 목표 · 과제 · 문서 목록"),
			tool.Baro_form("/baro/export", project_form),
			"",
		)
	}

	// --- 내 기여 내려받기 ---
	me_body := ""
	if tool.Baro_is_user(config.IP) {
		me_body = tool.Baro_t(`<p>내 기여 이벤트 · 받은 감사 · 배지를 JSON 하나로 묶어 내려받습니다. 본인만 받을 수 있습니다.</p>`)
		me_body += tool.Baro_button("/baro/export/me", tool.Baro_t("내 기여 기록 내려받기"), "primary")
	} else {
		me_body = tool.Baro_alert("warn", tool.Baro_t("내 기여 기록은 로그인한 본인만 내려받을 수 있습니다.")) +
			tool.Baro_button("/login", tool.Baro_t("로그인"), "")
	}

	body += tool.Baro_card_ex(tool.Baro_t("내 기여 기록 내보내기"), "baro_contrib_event · baro_thanks · baro_badge", me_body, "")

	// --- 공개 덤프 / API 로 이어지는 안내 ---
	body += tool.Baro_card(
		tool.Baro_t("더 큰 단위로 가져가기"),
		tool.Baro_list([]string{
			tool.Baro_t(`<a href="/baro/export/dump">정기 데이터 덤프</a> — 민감 정보를 뺀 공개 문서 전체를 JSON Lines 로 배포합니다.`),
			tool.Baro_t(`<a href="/baro/api">공개 읽기 API</a> — 프로그램에서 바로 읽을 수 있는 JSON 응답을 제공합니다.`),
			tool.Baro_t(`<a href="/baro/backup">백업</a> — 운영자용 전체 백업과 무결성 검사입니다.`),
		}),
	)

	// --- 내보내기 이력 ---
	rows := [][]string{}
	list := tool.Query_DB(
		db,
		"select id, user, kind, scope, status, path, date from baro_export order by id + 0 desc limit ?, ?",
		offset, baro_data_page_size,
	)
	defer list.Close()

	count := 0
	for list.Next() {
		id, user, kind, scope, status, path, date := "", "", "", "", "", "", ""
		if err := list.Scan(&id, &user, &kind, &scope, &status, &path, &date); err != nil {
			continue
		}

		count++

		scope_html := tool.HTML_escape(tool.Baro_trunc(scope, 40))
		if scope == "" {
			scope_html = `<span class="bw_muted">-</span>`
		}

		path_html := tool.Baro_t(`<span class="bw_muted">즉시 내려받기</span>`)
		if path != "" {
			path_html = tool.Baro_code(path)
		}

		status_kind := "ok"
		if status != "done" {
			status_kind = "warn"
		}

		rows = append(rows, []string{
			tool.HTML_escape(id),
			tool.Baro_user_link(db, user, config.IP),
			tool.HTML_escape(baro_data_export_kind_name(kind)),
			scope_html,
			path_html,
			tool.Baro_badge(status, status_kind),
			tool.Baro_time_cell(date),
		})
	}

	table := tool.Baro_table(
		[]string{"번호", tool.Baro_t("받은 사람"), "종류", tool.Baro_t("대상"), tool.Baro_t("저장 경로"), "상태", "시각"},
		rows,
		tool.Baro_t("아직 내보내기 기록이 없습니다."),
	)

	if count == 0 {
		table = tool.Baro_empty(tool.Baro_t("아직 내보내기 기록이 없습니다. 위에서 문서를 하나 골라 내려받아 보세요."))
	}

	table += tool.Baro_paging(page_num, count, baro_data_page_size, "/baro/export/list/{}")

	body += tool.Baro_card_ex(tool.Baro_t("내보내기 이력"), tool.Baro_t("누가 무엇을 가져갔는지 기록합니다. (IP 는 가려서 기록합니다)"), table, "")

	return tool.Baro_page(db, config, tool.Baro_t("데이터 내보내기"), body, baro_data_menu())
}

// baro_data_export_post : 내보내기 고르기 폼 처리
func baro_data_export_post(db *sql.DB, config tool.Config, values url.Values) string {
	action := baro_data_q(values, "action")

	switch action {
	case "doc":
		doc := tool.Namespace_normalize(baro_data_q(values, "doc"))
		kind := baro_data_q(values, "kind")

		if doc == "" {
			return baro_data_error_page(db, config, tool.Baro_t("데이터 내보내기"), tool.Baro_t("문서 이름을 적어 주세요."))
		}

		allow := false
		for _, row := range baro_data_export_kind_options() {
			if row[0] == kind {
				allow = true
			}
		}

		if !allow {
			kind = "raw"
		}

		return tool.Get_redirect("/baro/export/" + kind + "?doc=" + tool.Url_parser(doc))
	case "project":
		id := baro_data_q(values, "id")
		if id == "" {
			return baro_data_error_page(db, config, tool.Baro_t("데이터 내보내기"), tool.Baro_t("프로젝트를 골라 주세요."))
		}

		return tool.Get_redirect("/baro/export/project?id=" + tool.Url_parser(id))
	}

	return tool.Get_redirect("/baro/export")
}

// baro_data_project_options : 프로젝트 고르기 목록 (다른 모듈 표를 SQL 로 직접 읽는다)
func baro_data_project_options(db *sql.DB) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(db, "select id, name, status from baro_project order by id + 0 desc limit 200")
	defer rows.Close()

	for rows.Next() {
		id, name, status := "", "", ""
		if err := rows.Scan(&id, &name, &status); err != nil {
			continue
		}

		label := name
		if label == "" {
			label = tool.Baro_t("이름 없는 프로젝트 ") + id
		}

		if status != "" {
			label += " (" + status + ")"
		}

		out = append(out, []string{id, label})
	}

	return out
}

// View_baro_data_export_file : 실제로 내려받아지는 파일을 만든다
//
//	돌려주는 값 : 본문, Content-Type, 내려받기 파일 이름
//	파일 이름이 "" 이면 화면(HTML)으로 그대로 보여 준다.
func View_baro_data_export_file(config tool.Config, kind string, query url.Values) (string, string, string) {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	switch kind {
	case "raw":
		return baro_data_export_raw(db, config, query)
	case "history":
		return baro_data_export_history(db, config, query)
	case "json":
		return baro_data_export_json(db, config, query)
	case "pdf":
		return baro_data_export_pdf(db, config, query)
	case "project":
		return baro_data_export_project(db, config, query)
	case "me":
		return baro_data_export_me(db, config)
	}

	return baro_data_error_page(db, config, tool.Baro_t("데이터 내보내기"), tool.Baro_t("알 수 없는 내보내기 형식입니다.")), "text/html; charset=utf-8", ""
}

// baro_data_export_doc_guard : 문서 내보내기 공통 확인
func baro_data_export_doc_guard(db *sql.DB, config tool.Config, query url.Values) (string, string, bool) {
	doc := tool.Namespace_normalize(baro_data_q(query, "doc"))
	if doc == "" {
		return "", baro_data_error_page(db, config, tool.Baro_t("데이터 내보내기"), tool.Baro_t("내보낼 문서 이름(doc)이 없습니다.")), false
	}

	if tool.Baro_doc_quarantined(db, doc) || tool.Baro_doc_status_get(db, doc) == "quarantined" {
		return doc, baro_data_error_page(db, config, tool.Baro_t("데이터 내보내기"), tool.Baro_t("격리된 문서는 내보낼 수 없습니다.")), false
	}

	if !tool.Check_acl(db, doc, "", "render", config.IP) {
		return doc, baro_data_error_page(db, config, tool.Baro_t("데이터 내보내기"), tool.Baro_t("이 문서를 읽을 권한이 없어 내보낼 수 없습니다.")), false
	}

	if _, found := baro_data_doc_text(db, doc); !found {
		return doc, baro_data_error_page(db, config, tool.Baro_t("데이터 내보내기"), tool.Baro_t("그런 문서가 없습니다 : ")+doc), false
	}

	return doc, "", true
}

// baro_data_export_raw : 단일 문서 원문 (text/plain)
func baro_data_export_raw(db *sql.DB, config tool.Config, query url.Values) (string, string, string) {
	doc, deny, ok := baro_data_export_doc_guard(db, config, query)
	if !ok {
		return deny, "text/html; charset=utf-8", ""
	}

	text, _ := baro_data_doc_text(db, doc)

	head := "# " + tool.Namespace_display(doc) + "\n"
	head += tool.Baro_t("# 출처 : ") + baro_data_wiki_name(db) + " /w/" + doc + "\n"
	head += tool.Baro_t("# 판 : r") + tool.Get_document_revision(db, doc) + "\n"
	head += tool.Baro_t("# 라이선스 : ") + baro_data_license(db) + "\n"
	head += tool.Baro_t("# 받은 시각 : ") + tool.Baro_now() + "\n"
	head += strings.Repeat("-", 40) + "\n\n"

	baro_data_export_add(db, config.IP, "raw", doc, "")

	return head + text, "text/plain; charset=utf-8", baro_data_safe_name(tool.Namespace_display(doc)) + ".txt"
}

// baro_data_export_history : 문서 역사 (JSON)
func baro_data_export_history(db *sql.DB, config tool.Config, query url.Values) (string, string, string) {
	doc, deny, ok := baro_data_export_doc_guard(db, config, query)
	if !ok {
		return deny, "text/html; charset=utf-8", ""
	}

	limit := tool.Str_to_int(baro_data_q(query, "limit"))
	if limit < 1 || limit > 2000 {
		limit = 1000
	}

	history := baro_data_doc_history(db, doc, limit)

	out := map[string]any{
		"wiki":       baro_data_wiki_name(db),
		"title":      tool.Namespace_display(doc),
		"raw_title":  doc,
		"license":    baro_data_license(db),
		"exported":   tool.Baro_now(),
		"rev_count":  baro_data_doc_rev_count(db, doc),
		"rev_limit":  limit,
		"revisions":  history,
		"note":       tool.Baro_t("IP 편집자는 개인정보 보호를 위해 anonymous 로만 표시됩니다."),
		"source_url": "/w/" + doc,
	}

	baro_data_export_add(db, config.IP, "history", doc, "")

	return baro_data_json(out), "application/json; charset=utf-8", baro_data_safe_name(tool.Namespace_display(doc)) + "_history.json"
}

// baro_data_export_json : 문서 + 메타 + 출처 (JSON)
func baro_data_export_json(db *sql.DB, config tool.Config, query url.Values) (string, string, string) {
	doc, deny, ok := baro_data_export_doc_guard(db, config, query)
	if !ok {
		return deny, "text/html; charset=utf-8", ""
	}

	text, _ := baro_data_doc_text(db, doc)

	out := map[string]any{
		"wiki":       baro_data_wiki_name(db),
		"title":      tool.Namespace_display(doc),
		"raw_title":  doc,
		"text":       text,
		"length":     tool.Get_len(text),
		"revision":   tool.Get_document_revision(db, doc),
		"rev_count":  baro_data_doc_rev_count(db, doc),
		"status":     tool.Baro_doc_status_get(db, doc),
		"field":      tool.Baro_doc_field(db, doc),
		"categories": baro_data_doc_categories(db, doc),
		"links":      baro_data_doc_links(db, doc, 300),
		"license":    baro_data_license(db),
		"exported":   tool.Baro_now(),
		"source_url": "/w/" + doc,
		"citations":  baro_data_citation_list(db, doc),
		"stable":     baro_data_stable_info(db, doc),
		"alias":      baro_data_alias_list(db, doc),
		"history":    baro_data_doc_history(db, doc, 50),
	}

	baro_data_export_add(db, config.IP, "json", doc, "")

	return baro_data_json(out), "application/json; charset=utf-8", baro_data_safe_name(tool.Namespace_display(doc)) + ".json"
}

// baro_data_citation_list : 문서 출처 목록 (baro_citation 표를 직접 읽는다)
func baro_data_citation_list(db *sql.DB, title string) []map[string]any {
	out := []map[string]any{}

	rows := tool.Query_DB(
		db,
		"select id, kind, url, author, publisher, pub_date, access_date, archive_url, quality, status from baro_citation where title = ? order by id + 0 asc limit 300",
		title,
	)
	defer rows.Close()

	for rows.Next() {
		id, kind, link, author, publisher, pub_date, access_date, archive, quality, status := "", "", "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &kind, &link, &author, &publisher, &pub_date, &access_date, &archive, &quality, &status); err != nil {
			continue
		}

		out = append(out, map[string]any{
			"id":          id,
			"kind":        kind,
			"url":         link,
			"author":      author,
			"publisher":   publisher,
			"pub_date":    pub_date,
			"access_date": access_date,
			"archive_url": archive,
			"quality":     quality,
			"status":      status,
		})
	}

	return out
}

// baro_data_stable_info : 안정판 정보
func baro_data_stable_info(db *sql.DB, title string) map[string]any {
	rev, date, note := "", "", ""
	if !tool.QueryRow_DB(db, "select rev, date, note from baro_stable where title = ?", []any{&rev, &date, &note}, title) {
		return map[string]any{}
	}

	return map[string]any{"rev": rev, "date": date, "note": note}
}

// baro_data_alias_list : 문서 별칭 목록
func baro_data_alias_list(db *sql.DB, title string) []map[string]any {
	out := []map[string]any{}

	rows := tool.Query_DB(db, "select alias, kind, note from baro_doc_alias where title = ? order by id + 0 asc limit 100", title)
	defer rows.Close()

	for rows.Next() {
		alias, kind, note := "", "", ""
		if err := rows.Scan(&alias, &kind, &note); err != nil {
			continue
		}

		out = append(out, map[string]any{"alias": alias, "kind": kind, "note": note})
	}

	return out
}

// baro_data_render_safe : 위키 문법 렌더 (실패해도 화면이 죽지 않게 감싼다)
func baro_data_render_safe(db *sql.DB, title string, text string) (html string) {
	defer func() {
		if err := recover(); err != nil {
			html = `<pre class="bw_print_raw">` + tool.HTML_escape(text) + `</pre>`
		}
	}()

	rendered := markup.Get_render(db, title, text, "normal", map[string]any{})["data"]
	if strings.TrimSpace(rendered) == "" {
		return `<pre class="bw_print_raw">` + tool.HTML_escape(text) + `</pre>`
	}

	return rendered
}

// baro_data_export_pdf : 인쇄용 화면 (브라우저의 "PDF 로 저장" 사용)
//
//	외부 라이브러리를 쓰지 않고, 실제로 인쇄에 적합한 화면을 준다.
func baro_data_export_pdf(db *sql.DB, config tool.Config, query url.Values) (string, string, string) {
	doc, deny, ok := baro_data_export_doc_guard(db, config, query)
	if !ok {
		return deny, "text/html; charset=utf-8", ""
	}

	text, _ := baro_data_doc_text(db, doc)
	title := tool.Namespace_display(doc)
	rev := tool.Get_document_revision(db, doc)
	license := baro_data_license(db)
	wiki := baro_data_wiki_name(db)
	now := tool.Baro_now()

	rendered := baro_data_render_safe(db, doc, text)

	html := `<!doctype html>
<html lang="ko">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + tool.HTML_escape(title) + ` — 인쇄용</title>
<style>
:root{color-scheme:light;}
body{margin:0;background:#f4f5f7;color:#111;font-family:"Malgun Gothic","Apple SD Gothic Neo",sans-serif;line-height:1.75;}
.bw_print_bar{position:sticky;top:0;background:#fff;border-bottom:1px solid #d9dde2;padding:.8rem 1rem;display:flex;flex-wrap:wrap;gap:.5rem;align-items:center;}
.bw_print_bar b{font-size:1rem;}
.bw_print_bar .bw_print_help{color:#6b7280;font-size:.88rem;flex:1 1 16rem;}
.bw_print_btn{display:inline-block;padding:.4rem .9rem;border:1px solid #d9dde2;border-radius:6px;background:linear-gradient(90deg,#010080,#0014de);color:#fff;font-weight:700;cursor:pointer;text-decoration:none;font-size:.92rem;}
.bw_print_btn2{display:inline-block;padding:.4rem .9rem;border:1px solid #d9dde2;border-radius:6px;background:#fff;color:#111;font-weight:600;text-decoration:none;font-size:.92rem;}
.bw_print_sheet{max-width:820px;margin:1.4rem auto;background:#fff;padding:2.6rem 2.8rem;border:1px solid #e2e5e9;border-radius:4px;}
.bw_print_h1{font-size:1.9rem;font-weight:800;margin:0 0 .3rem 0;}
.bw_print_meta{color:#555;font-size:.86rem;border-bottom:1px solid #ddd;padding-bottom:.9rem;margin-bottom:1.4rem;}
.bw_print_meta div{margin:.15rem 0;}
.bw_print_body{font-size:1rem;word-break:break-word;}
.bw_print_body img{max-width:100%;height:auto;}
.bw_print_body table{border-collapse:collapse;}
.bw_print_body table td,.bw_print_body table th{border:1px solid #bbb;padding:.3rem .5rem;}
.bw_print_raw{white-space:pre-wrap;word-break:break-word;font-size:.94rem;}
.bw_print_foot{margin-top:2rem;padding-top:.9rem;border-top:1px solid #ddd;color:#555;font-size:.82rem;}
@media print{
  @page{size:A4;margin:18mm 16mm 20mm 16mm;}
  body{background:#fff;}
  .bw_print_bar{display:none !important;}
  .bw_print_sheet{max-width:none;margin:0;padding:0;border:0;border-radius:0;}
  .bw_print_body{font-size:10.5pt;}
  .bw_print_h1{font-size:20pt;}
  .bw_print_body h1,.bw_print_body h2,.bw_print_body h3{page-break-after:avoid;break-after:avoid;}
  .bw_print_body img,.bw_print_body table,.bw_print_body pre{page-break-inside:avoid;break-inside:avoid;}
  a{color:#000;text-decoration:none;}
  .bw_print_foot{page-break-inside:avoid;}
}
</style>
</head>
<body>
<div class="bw_print_bar">
<b>인쇄용 화면</b>
<span class="bw_print_help">아래 버튼을 누른 뒤 인쇄 대화상자에서 <b>대상</b>을 "PDF로 저장"(Save as PDF)으로 고르면 PDF 파일이 만들어집니다.</span>
<a class="bw_print_btn" href="javascript:window.print()">인쇄 / PDF 로 저장</a>
<a class="bw_print_btn2" href="/w/` + tool.HTML_escape(tool.Url_parser(doc)) + `">문서로 돌아가기</a>
<a class="bw_print_btn2" href="/baro/export">내보내기 화면</a>
</div>
<div class="bw_print_sheet">
<h1 class="bw_print_h1">` + tool.HTML_escape(title) + `</h1>
<div class="bw_print_meta">
<div>출처 : ` + tool.HTML_escape(wiki) + ` — /w/` + tool.HTML_escape(doc) + `</div>
<div>판 : r` + tool.HTML_escape(rev) + ` · 글자 수 ` + tool.Baro_comma(tool.Get_len(text)) + `</div>
<div>내려받은 시각 : ` + tool.HTML_escape(now) + `</div>
<div>라이선스 : ` + tool.HTML_escape(license) + `</div>
</div>
<div class="bw_print_body">` + rendered + `</div>
<div class="bw_print_foot">
이 문서는 ` + tool.HTML_escape(wiki) + ` 에서 ` + tool.HTML_escape(now) + ` 에 내려받았습니다.
` + tool.HTML_escape(license) + ` 조건에 따라 자유롭게 쓸 수 있으며, 재사용 시 출처를 표기해 주세요.
</div>
</div>
</body>
</html>`

	baro_data_export_add(db, config.IP, "pdf", doc, "")

	return html, "text/html; charset=utf-8", ""
}

// baro_data_export_project : 프로젝트 전체 (목표 · 과제 · 문서 목록)
func baro_data_export_project(db *sql.DB, config tool.Config, query url.Values) (string, string, string) {
	id := baro_data_q(query, "id")
	if id == "" {
		return baro_data_error_page(db, config, tool.Baro_t("데이터 내보내기"), tool.Baro_t("프로젝트 번호(id)가 없습니다.")), "text/html; charset=utf-8", ""
	}

	name, field, goal, scope, start_date, end_date, owner, status := "", "", "", "", "", "", "", ""
	target_count, done_count, rule, finish_cond, date, summary := "", "", "", "", "", ""

	found := tool.QueryRow_DB(
		db,
		"select name, field, goal, scope, start_date, end_date, owner, status, target_count, done_count, rule, finish_cond, date, summary from baro_project where id = ?",
		[]any{&name, &field, &goal, &scope, &start_date, &end_date, &owner, &status, &target_count, &done_count, &rule, &finish_cond, &date, &summary},
		id,
	)

	if !found {
		return baro_data_error_page(db, config, tool.Baro_t("데이터 내보내기"), tool.Baro_t("그런 프로젝트가 없습니다 : ")+id), "text/html; charset=utf-8", ""
	}

	// 목표 (대상 문서)
	targets := []map[string]any{}
	target_rows := tool.Query_DB(
		db,
		"select id, name, note, status, doc, assignee, date from baro_project_target where project = ? order by id + 0 asc limit 2000",
		id,
	)
	doc_set := map[string]bool{}

	for target_rows.Next() {
		tid, tname, note, tstatus, doc, assignee, tdate := "", "", "", "", "", "", ""
		if err := target_rows.Scan(&tid, &tname, &note, &tstatus, &doc, &assignee, &tdate); err != nil {
			continue
		}

		if doc != "" && !tool.Baro_doc_quarantined(db, doc) {
			doc_set[doc] = true
		}

		targets = append(targets, map[string]any{
			"id": tid, "name": tname, "note": note, "status": tstatus,
			"doc": doc, "assignee": assignee, "date": tdate,
		})
	}
	target_rows.Close()

	// 과제
	tasks := []map[string]any{}
	task_rows := tool.Query_DB(
		db,
		"select id, kind, title, detail, difficulty, est_time, assignee, status, doc, date, done_date, field from baro_project_task where project = ? order by id + 0 asc limit 2000",
		id,
	)

	for task_rows.Next() {
		tid, kind, title, detail, difficulty, est, assignee, tstatus, doc, tdate, done_date, tfield := "", "", "", "", "", "", "", "", "", "", "", ""
		if err := task_rows.Scan(&tid, &kind, &title, &detail, &difficulty, &est, &assignee, &tstatus, &doc, &tdate, &done_date, &tfield); err != nil {
			continue
		}

		if doc != "" && !tool.Baro_doc_quarantined(db, doc) {
			doc_set[doc] = true
		}

		tasks = append(tasks, map[string]any{
			"id": tid, "kind": kind, "title": title, "detail": detail,
			"difficulty": difficulty, "est_time": est, "assignee": assignee,
			"status": tstatus, "doc": doc, "date": tdate, "done_date": done_date, "field": tfield,
		})
	}
	task_rows.Close()

	// 문서 목록
	docs := []map[string]any{}
	for doc := range doc_set {
		text, exists := baro_data_doc_text(db, doc)
		docs = append(docs, map[string]any{
			"title":    tool.Namespace_display(doc),
			"exists":   exists,
			"length":   tool.Get_len(text),
			"status":   tool.Baro_doc_status_get(db, doc),
			"revision": tool.Get_document_revision(db, doc),
			"url":      "/w/" + doc,
		})
	}

	// 진행 기록
	progress := []map[string]any{}
	progress_rows := tool.Query_DB(
		db,
		"select date, done, total, note from baro_project_progress where project = ? order by id + 0 asc limit 500",
		id,
	)

	for progress_rows.Next() {
		pdate, done, total, note := "", "", "", ""
		if err := progress_rows.Scan(&pdate, &done, &total, &note); err != nil {
			continue
		}

		progress = append(progress, map[string]any{"date": pdate, "done": done, "total": total, "note": note})
	}
	progress_rows.Close()

	// 참고 자료
	sources := []map[string]any{}
	source_rows := tool.Query_DB(
		db,
		"select title, url, note, date from baro_project_source where project = ? order by id + 0 asc limit 500",
		id,
	)

	for source_rows.Next() {
		stitle, surl, snote, sdate := "", "", "", ""
		if err := source_rows.Scan(&stitle, &surl, &snote, &sdate); err != nil {
			continue
		}

		sources = append(sources, map[string]any{"title": stitle, "url": surl, "note": snote, "date": sdate})
	}
	source_rows.Close()

	out := map[string]any{
		"wiki": baro_data_wiki_name(db),
		"project": map[string]any{
			"id": id, "name": name, "field": field, "field_name": tool.Baro_field_name(db, field),
			"goal": goal, "scope": scope, "start_date": start_date, "end_date": end_date,
			"owner": owner, "status": status, "target_count": tool.Str_to_int(target_count),
			"done_count": tool.Str_to_int(done_count), "rule": rule, "finish_cond": finish_cond,
			"date": date, "summary": summary,
		},
		"targets":  targets,
		"tasks":    tasks,
		"docs":     docs,
		"progress": progress,
		"sources":  sources,
		"license":  baro_data_license(db),
		"exported": tool.Baro_now(),
		"note":     tool.Baro_t("격리된 문서는 목록에서 빠집니다. 개인정보는 담기지 않습니다."),
	}

	baro_data_export_add(db, config.IP, "project", "project:"+id, "")

	return baro_data_json(out), "application/json; charset=utf-8", "project_" + baro_data_safe_name(id) + ".json"
}

// baro_data_export_me : 내 기여 기록 (본인만)
func baro_data_export_me(db *sql.DB, config tool.Config) (string, string, string) {
	if !tool.Baro_is_user(config.IP) {
		page, _ := tool.Baro_login_wall(db, config, "/baro/export/me")

		return page, "text/html; charset=utf-8", ""
	}

	user := config.IP

	// 기여 이벤트
	events := []map[string]any{}
	event_rows := tool.Query_DB(
		db,
		"select id, kind, title, rev, project, weight, detail, date, status, field from baro_contrib_event where user = ? order by id + 0 desc limit 5000",
		user,
	)

	for event_rows.Next() {
		id, kind, title, rev, project, weight, detail, date, status, field := "", "", "", "", "", "", "", "", "", ""
		if err := event_rows.Scan(&id, &kind, &title, &rev, &project, &weight, &detail, &date, &status, &field); err != nil {
			continue
		}

		events = append(events, map[string]any{
			"id": id, "kind": kind, "kind_name": tool.Baro_contrib_kind_name(kind),
			"title": title, "rev": rev, "project": project,
			"weight": tool.Str_to_int(weight), "detail": detail,
			"date": date, "status": status, "field": field,
		})
	}
	event_rows.Close()

	// 받은 감사 / 보낸 감사
	thanks_in := []map[string]any{}
	thanks_out := []map[string]any{}

	thanks_rows := tool.Query_DB(
		db,
		"select id, from_user, to_user, title, rev, reason, is_public, date from baro_thanks where to_user = ? or from_user = ? order by id + 0 desc limit 3000",
		user, user,
	)

	for thanks_rows.Next() {
		id, from_user, to_user, title, rev, reason, is_public, date := "", "", "", "", "", "", "", ""
		if err := thanks_rows.Scan(&id, &from_user, &to_user, &title, &rev, &reason, &is_public, &date); err != nil {
			continue
		}

		row := map[string]any{
			"id": id, "from": from_user, "to": to_user, "title": title,
			"rev": rev, "reason": reason, "public": is_public == "on", "date": date,
		}

		if to_user == user {
			thanks_in = append(thanks_in, row)
		} else {
			thanks_out = append(thanks_out, row)
		}
	}
	thanks_rows.Close()

	// 배지
	badges := []map[string]any{}
	badge_rows := tool.Query_DB(
		db,
		"select id, badge, date, detail from baro_user_badge where user = ? order by id + 0 asc limit 500",
		user,
	)

	for badge_rows.Next() {
		id, badge, date, detail := "", "", "", ""
		if err := badge_rows.Scan(&id, &badge, &date, &detail); err != nil {
			continue
		}

		name, description, kind := "", "", ""
		tool.QueryRow_DB(
			db,
			"select name, description, kind from baro_badge where key_name = ?",
			[]any{&name, &description, &kind}, badge,
		)

		badges = append(badges, map[string]any{
			"key": badge, "name": name, "description": description,
			"kind": kind, "date": date, "detail": detail,
		})
	}
	badge_rows.Close()

	out := map[string]any{
		"wiki":           baro_data_wiki_name(db),
		"user":           user,
		"founder_number": tool.Baro_founder_number(db, user),
		"roles":          tool.Baro_user_roles(db, user),
		"interests":      tool.Baro_user_interest(db, user),
		"contrib_events": events,
		"thanks_in":      thanks_in,
		"thanks_out":     thanks_out,
		"badges":         badges,
		"license":        baro_data_license(db),
		"exported":       tool.Baro_now(),
		"note":           tool.Baro_t("본인만 내려받을 수 있는 파일입니다. IP 와 이메일은 담기지 않습니다."),
	}

	baro_data_export_add(db, user, "me", user, "")
	tool.Baro_audit(db, config, "export_me", user, "", "", "본인 기여 기록 내려받기")

	return baro_data_json(out), "application/json; charset=utf-8", "my_contributions_" + baro_data_safe_name(user) + ".json"
}
