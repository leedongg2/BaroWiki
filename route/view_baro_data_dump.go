package route

import (
	"bufio"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  14-2 정기 데이터 덤프  (/baro/export/dump)
//
//  민감 정보를 제외한 공개 문서 데이터를 JSON Lines 로 정기 배포한다.
//  포함 : 문서 · 리비전 요약 · 분류 · 링크 관계 · 라이선스
//  제외 : IP · 이메일 · 비공개 신고 내용 · 격리 문서
// ============================================================

// baro_data_dump_exclude : 덤프에서 빠지는 것 (화면에 그대로 보여 준다)
func baro_data_dump_exclude() []string {
	return []string{
		tool.Baro_t("IP 주소 — 비로그인 편집자는 이름 없이 anonymous 로만 기록합니다."),
		tool.Baro_t("이메일 주소 — 어떤 형태로도 담지 않습니다."),
		tool.Baro_t("비공개 신고 내용 (baro_report · baro_case 의 비공개 항목)과 내부 감사 기록."),
		tool.Baro_t("격리된 문서 (baro_quarantine) 와 격리 상태 문서 — 본문·역사 모두 제외합니다."),
		tool.Baro_t("이용자 개인 문서 이름공간 (user:) — 개인 공간이라 공개 배포에서 뺍니다."),
		tool.Baro_t("숨김 처리된 판의 편집 요약 — 요약 대신 (숨겨진 요약) 으로만 남깁니다."),
	}
}

// baro_data_dump_max : 한 번에 처리할 최대 문서 수
func baro_data_dump_max(db *sql.DB) int {
	max_count := tool.Baro_get_int(db, "dump_max_docs", 300)
	if max_count < 10 {
		max_count = 10
	}
	if max_count > 5000 {
		max_count = 5000
	}

	return max_count
}

// baro_data_dump_total : 덤프 대상 문서 수
func baro_data_dump_total(db *sql.DB) int {
	count := "0"
	tool.QueryRow_DB(db, "select count(*) from data where title not like 'user:%'", []any{&count})

	return tool.Str_to_int(count)
}

// baro_data_dump_offset : 이어서 만들 위치
func baro_data_dump_offset(db *sql.DB) int {
	offset := tool.Baro_get_int(db, "dump_offset", 0)
	if offset < 0 {
		offset = 0
	}

	return offset
}

// View_baro_data_dump : 정기 덤프 화면
func View_baro_data_dump(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if values != nil {
		return baro_data_dump_post(db, config, values)
	}

	page_num := baro_data_page_num(page)
	offset := (page_num - 1) * baro_data_page_size

	total := baro_data_dump_total(db)
	cursor := baro_data_dump_offset(db)
	max_count := baro_data_dump_max(db)
	can_make := tool.Baro_can(db, config.IP, "system")

	body := baro_data_tabs("/baro/export/dump")
	body += tool.Baro_hero(
		tool.Baro_t("정기 데이터 덤프"),
		tool.Baro_t("민감 정보를 뺀 공개 문서 데이터를 누구나 통째로 내려받을 수 있게 배포합니다."),
		tool.Baro_note(tool.Baro_t("형식 : JSON Lines (.jsonl) — 한 줄에 JSON 하나. 큰 파일도 한 줄씩 읽을 수 있습니다.")),
	)

	// --- 제외 항목 명시 (만들기 전에 반드시 보여 준다) ---
	exclude_items := []string{}
	for _, text := range baro_data_dump_exclude() {
		exclude_items = append(exclude_items, tool.HTML_escape(text))
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("덤프에서 제외하는 것"),
		tool.Baro_t("덤프를 만들기 전에 반드시 확인하세요."),
		tool.Baro_alert("warn", tool.Baro_t("아래 항목은 덤프 파일에 <b>절대 들어가지 않습니다</b>."))+tool.Baro_list(exclude_items),
		tool.Baro_note(tool.Baro_t("포함하는 것 : 문서 본문 · 리비전 요약(판 번호·시각·기여자 이름·요약·길이) · 분류 · 링크 관계 · 라이선스 정보")),
	)

	// --- 진행 상황 ---
	progress := tool.Baro_progress(cursor, total, tool.Baro_t("덤프 진행"))
	if cursor >= total && total > 0 {
		progress += tool.Baro_alert("ok", tool.Baro_t("모든 문서를 한 바퀴 처리했습니다. <b>처음부터 다시</b> 를 눌러 새 회차를 시작할 수 있습니다."))
	}

	progress += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 문서"), tool.Baro_comma(total), tool.Baro_t("user: 이름공간 제외")},
		{tool.Baro_t("처리한 문서"), tool.Baro_comma(cursor), tool.Baro_t("이어서 만들기 시작 위치")},
		{tool.Baro_t("한 번에"), tool.Baro_comma(max_count), tool.Baro_t("회차당 최대 문서 수")},
		{tool.Baro_t("남은 문서"), tool.Baro_comma(baro_data_dump_left(total, cursor)), ""},
	})

	body += tool.Baro_card_ex(tool.Baro_t("진행 상황"), tool.Baro_t("문서 수가 많을 수 있어 회차를 나눠 처리합니다."), progress, "")

	// --- 만들기 ---
	if can_make {
		make_form := tool.Baro_input_type("max_count", tool.Baro_t("이번 회차 문서 수"), tool.Baro_itoa(max_count), "number", tool.Baro_t("10 ~ 5000 사이. 기본값은 설정에 저장됩니다."))
		make_form += tool.Baro_checkbox("restart", tool.Baro_t("처음부터 다시 (진행 위치를 0 으로 되돌리고 시작)"), "on", false, "")
		make_form += tool.Baro_checkbox("confirm", tool.Baro_t("위 제외 항목을 확인했습니다."), "on", false, tool.Baro_t("확인하지 않으면 만들지 않습니다."))
		make_form += tool.Baro_hidden("action", "make")
		make_form += tool.Baro_submit(tool.Baro_t("이번 회차 덤프 만들기"), "primary")

		reset_form := tool.Baro_hidden("action", "reset") + tool.Baro_submit(tool.Baro_t("진행 위치만 0 으로 되돌리기"), "")

		body += tool.Baro_card_ex(
			tool.Baro_t("덤프 만들기"),
			tool.Baro_t("data/dump/ 아래에 .jsonl 파일로 저장하고 백업 목록(baro_backup)에 기록합니다."),
			tool.Baro_form("/baro/export/dump", make_form)+tool.Baro_form("/baro/export/dump", reset_form),
			"",
		)
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("덤프를 새로 만드는 것은 시스템 관리 권한이 있는 사람만 할 수 있습니다. 아래 목록에서 이미 만들어진 파일은 누구나 내려받을 수 있습니다."))
	}

	// --- 덤프 목록 ---
	list := baro_data_backup_list(db, "dump", offset, baro_data_page_size)
	rows := [][]string{}

	for _, row := range list {
		exists := tool.File_exist_check(row.Path)

		file_html := tool.Baro_code(row.Path)
		if !exists {
			file_html += tool.Baro_badge(tool.Baro_t("파일 없음"), "danger")
		}

		down := `<span class="bw_muted">-</span>`
		if exists {
			down = tool.Baro_button("/baro/export/dump/file?id="+tool.Url_parser(row.ID), "내려받기", "primary")
		}

		range_text := row.Note["range"]
		if range_text == "" {
			range_text = "-"
		}

		rows = append(rows, []string{
			tool.HTML_escape(row.ID),
			file_html,
			tool.HTML_escape(range_text),
			tool.HTML_escape(row.Note["docs"]) + " 건",
			tool.HTML_escape(baro_data_size_text(baro_data_atoi64(row.Size))),
			tool.Baro_time_cell(row.Date),
			down,
		})
	}

	table := tool.Baro_table(
		[]string{"번호", tool.Baro_t("파일"), tool.Baro_t("문서 범위"), tool.Baro_t("문서 수"), tool.Baro_t("크기"), tool.Baro_t("만든 시각"), "내려받기"},
		rows,
		tool.Baro_t("아직 만들어진 덤프가 없습니다."),
	)

	if len(rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("아직 만들어진 덤프가 없습니다."))
	}

	table += tool.Baro_paging(page_num, len(rows), baro_data_page_size, "/baro/export/dump/list/{}")

	body += tool.Baro_card_ex(tool.Baro_t("덤프 파일 목록"), tool.Baro_t("회차마다 파일 하나가 만들어집니다."), table, "")

	// --- 읽는 방법 안내 ---
	body += tool.Baro_card(
		tool.Baro_t("덤프 파일 읽는 방법"),
		tool.Baro_t(`<p>파일의 각 줄은 독립된 JSON 하나입니다. <code>type</code> 값으로 줄 종류를 구분합니다.</p>`)+
			tool.Baro_code(`{"type":"meta","wiki":"바로위키","license":"CC BY-SA 4.0","docs":300,...}
{"type":"doc","title":"`+tool.Baro_t(`문서 이름`)+`","text":"`+tool.Baro_t(`본문`)+`","length":123,"categories":[...],"links":[...]}
{"type":"rev","title":"`+tool.Baro_t(`문서 이름`)+`","rev":"3","date":"2026-01-01 00:00:00","user":"`+tool.Baro_t(`이름`)+`","anonymous":false,"summary":"`+tool.Baro_t(`요약`)+`","length":123}`)+
			tool.Baro_note(tool.Baro_t("파이썬 예시 : for line in open('dump.jsonl', encoding='utf-8'): row = json.loads(line)")),
	)

	return tool.Baro_page(db, config, tool.Baro_t("정기 데이터 덤프"), body, baro_data_menu())
}

// baro_data_dump_left : 남은 문서 수
func baro_data_dump_left(total int, cursor int) int {
	left := total - cursor
	if left < 0 {
		left = 0
	}

	return left
}

// baro_data_atoi64 : 문자열을 int64 로
func baro_data_atoi64(data string) int64 {
	return int64(tool.Str_to_int(data))
}

// baro_data_dump_post : 덤프 만들기 / 되돌리기 처리
func baro_data_dump_post(db *sql.DB, config tool.Config, values url.Values) string {
	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("덤프를 만들 권한이 없습니다. (시스템 관리 권한 필요)"))
	}

	action := baro_data_q(values, "action")

	if action == "reset" {
		tool.Baro_put(db, "dump_offset", "0")
		tool.Baro_audit(db, config, "dump_reset", "dump", "", "0", "덤프 진행 위치 초기화")

		return tool.Get_redirect("/baro/export/dump")
	}

	if action != "make" {
		return tool.Get_redirect("/baro/export/dump")
	}

	if baro_data_q(values, "confirm") != "on" {
		return baro_data_error_page(db, config, tool.Baro_t("정기 데이터 덤프"), tool.Baro_t("제외 항목 확인란에 표시해야 덤프를 만들 수 있습니다."))
	}

	max_count := tool.Str_to_int(baro_data_q(values, "max_count"))
	if max_count < 10 {
		max_count = 10
	}
	if max_count > 5000 {
		max_count = 5000
	}

	tool.Baro_put(db, "dump_max_docs", tool.Baro_itoa(max_count))

	start := baro_data_dump_offset(db)
	if baro_data_q(values, "restart") == "on" {
		start = 0
	}

	total := baro_data_dump_total(db)
	if start >= total {
		start = 0
	}

	path, doc_count, rev_count, err := baro_data_dump_write(db, start, max_count)
	if err != nil {
		return baro_data_error_page(db, config, tool.Baro_t("정기 데이터 덤프"), tool.Baro_t("덤프 파일을 만들지 못했습니다 : ")+err.Error())
	}

	checksum, size, hash_err := baro_data_file_hash(path)
	if hash_err != nil {
		checksum = ""
		size = 0
	}

	note := baro_data_note_put(map[string]string{
		"docs":     tool.Baro_itoa(doc_count),
		"revs":     tool.Baro_itoa(rev_count),
		"range":    tool.Baro_itoa(start+1) + " ~ " + tool.Baro_itoa(start+doc_count),
		"total":    tool.Baro_itoa(total),
		"license":  baro_data_license(db),
		"excluded": "ip,email,private_report,quarantine,user_namespace",
		"memo":     tool.Baro_t("정기 공개 덤프"),
	})

	tool.Exec_DB(
		db,
		"insert into baro_backup (id, kind, path, size, status, date, note, checksum) values (?, 'dump', ?, ?, 'done', ?, ?, ?)",
		tool.Baro_next_id(db, "baro_backup"), path, tool.Baro_itoa(int(size)), tool.Baro_now(), note, checksum,
	)

	tool.Baro_put(db, "dump_offset", tool.Baro_itoa(start+doc_count))
	tool.Baro_put(db, "dump_last_date", tool.Baro_now())

	baro_data_export_add(db, config.IP, "dump", path, path)
	tool.Baro_public_log(db, "data_dump", config.IP, "공개 덤프", "정기 데이터 개방", tool.Baro_itoa(doc_count)+"개 문서")
	tool.Baro_audit(db, config, "dump_make", path, "", tool.Baro_itoa(doc_count), "정기 덤프 생성")
	tool.Baro_metric_bump(db, "dump_docs", tool.Baro_today(), doc_count)

	return tool.Get_redirect("/baro/export/dump")
}

// baro_data_dump_write : 실제 JSON Lines 파일을 만든다
func baro_data_dump_write(db *sql.DB, start int, max_count int) (string, int, int, error) {
	if err := baro_data_ensure_dir(baro_data_dump_dir); err != nil {
		return "", 0, 0, err
	}

	path := baro_data_path(baro_data_dump_dir, "baro_dump_"+baro_data_stamp()+".jsonl")

	file, err := os.Create(path)
	if err != nil {
		return "", 0, 0, err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	license := baro_data_license(db)
	wiki := baro_data_wiki_name(db)

	// 첫 줄 : 메타 정보
	writer.WriteString(baro_data_json_line(map[string]any{
		"type":     "meta",
		"wiki":     wiki,
		"license":  license,
		"format":   "jsonl",
		"version":  tool.Baro_ver,
		"date":     tool.Baro_now(),
		"start":    start,
		"max":      max_count,
		"excluded": baro_data_dump_exclude(),
	}))

	// 문서 목록 (제목 순서 고정 — 회차를 나눠도 겹치지 않는다)
	titles := []string{}
	rows := tool.Query_DB(
		db,
		"select title from data where title not like 'user:%' order by title limit ?, ?",
		start, max_count,
	)

	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		titles = append(titles, title)
	}
	rows.Close()

	doc_count := 0
	rev_count := 0

	for _, title := range titles {
		doc_count++

		// 격리 문서는 본문·역사 모두 빼고 자리만 남긴다.
		if tool.Baro_doc_quarantined(db, title) {
			writer.WriteString(baro_data_json_line(map[string]any{
				"type":     "doc",
				"title":    tool.Namespace_display(title),
				"excluded": true,
				"reason":   "quarantine",
			}))

			continue
		}

		status := tool.Baro_doc_status_get(db, title)
		if status == "quarantined" {
			writer.WriteString(baro_data_json_line(map[string]any{
				"type":     "doc",
				"title":    tool.Namespace_display(title),
				"excluded": true,
				"reason":   "status:" + status,
			}))

			continue
		}

		text, _ := baro_data_doc_text(db, title)

		writer.WriteString(baro_data_json_line(map[string]any{
			"type":       "doc",
			"title":      tool.Namespace_display(title),
			"raw_title":  title,
			"text":       text,
			"length":     tool.Get_len(text),
			"revision":   tool.Get_document_revision(db, title),
			"status":     status,
			"field":      tool.Baro_doc_field(db, title),
			"categories": baro_data_doc_categories(db, title),
			"links":      baro_data_doc_links(db, title, 300),
			"license":    license,
			"url":        "/w/" + title,
		}))

		for _, rev := range baro_data_doc_history(db, title, 100) {
			rev_count++

			writer.WriteString(baro_data_json_line(map[string]any{
				"type":      "rev",
				"title":     tool.Namespace_display(title),
				"rev":       rev.Rev,
				"date":      rev.Date,
				"user":      rev.User,
				"anonymous": rev.Anonymous,
				"summary":   rev.Summary,
				"length":    rev.Length,
				"kind":      rev.Kind,
			}))
		}
	}

	// 마지막 줄 : 회차 요약
	writer.WriteString(baro_data_json_line(map[string]any{
		"type":  "end",
		"docs":  doc_count,
		"revs":  rev_count,
		"date":  tool.Baro_now(),
		"range": []int{start + 1, start + doc_count},
	}))

	if err := writer.Flush(); err != nil {
		return path, doc_count, rev_count, err
	}

	return path, doc_count, rev_count, nil
}

// View_baro_data_dump_file : 덤프 파일 내려받기
func View_baro_data_dump_file(config tool.Config, query url.Values) (string, string, string) {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	id := baro_data_q(query, "id")
	if id == "" {
		return baro_data_error_page(db, config, tool.Baro_t("정기 데이터 덤프"), tool.Baro_t("덤프 번호(id)가 없습니다.")), "text/html; charset=utf-8", ""
	}

	row, found := baro_data_backup_get(db, id)
	if !found || row.Kind != "dump" {
		return baro_data_error_page(db, config, tool.Baro_t("정기 데이터 덤프"), tool.Baro_t("그런 덤프가 없습니다 : ")+id), "text/html; charset=utf-8", ""
	}

	// 경로 탈출 방지 : data/dump 안의 파일만 내보낸다.
	clean := filepath.ToSlash(filepath.Clean(row.Path))
	if !strings.HasPrefix(clean, baro_data_dump_dir+"/") {
		return baro_data_error_page(db, config, tool.Baro_t("정기 데이터 덤프"), tool.Baro_t("덤프 폴더 밖의 파일은 내려받을 수 없습니다.")), "text/html; charset=utf-8", ""
	}

	raw, err := os.ReadFile(row.Path)
	if err != nil {
		return baro_data_error_page(db, config, tool.Baro_t("정기 데이터 덤프"), tool.Baro_t("덤프 파일을 열 수 없습니다. 이미 지워졌을 수 있습니다.")), "text/html; charset=utf-8", ""
	}

	baro_data_export_add(db, config.IP, "dump", row.Path, row.Path)

	return string(raw), "application/x-ndjson; charset=utf-8", filepath.Base(row.Path)
}
