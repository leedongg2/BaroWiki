package route

import (
	"bytes"
	"database/sql"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"opennamu/route/tool"
)

// ============================================================
//  17-2 (2) 검사 도구
//
//   - 스팸 필터 검사   /baro/security/abuse/spam
//   - 위험 링크 검사   /baro/security/abuse/link
//   - 파일 검사        /baro/security/abuse/file
//
//  세 화면 모두 실제로 DB · 디스크를 읽어 결과를 만든다.
// ============================================================

// baro_sec_scan_size : 한 쪽에서 검사하는 문서 수
const baro_sec_scan_size = 200

// baro_sec_doc_page : 문서 본문을 쪽 단위로 읽는다 (제목, 본문)
func baro_sec_doc_page(db *sql.DB, offset int, limit int) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(db, "select title, data from data order by title limit ? offset ?", limit, offset)
	defer rows.Close()

	for rows.Next() {
		title, data := "", ""
		if err := rows.Scan(&title, &data); err != nil {
			continue
		}

		out = append(out, []string{title, data})
	}

	return out
}

// baro_sec_doc_count : 문서 수
func baro_sec_doc_count(db *sql.DB) int {
	count := 0
	tool.QueryRow_DB(db, "select count(*) from data", []any{&count})

	return count
}

// ------------------------------------------------------------
//  /baro/security/abuse/spam : 스팸 필터
// ------------------------------------------------------------

// View_baro_sec_abuse_spam : 스팸 필터 검사 화면
func View_baro_sec_abuse_spam(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/security/abuse/spam"); need {
		return wall
	}

	if !baro_sec_is_system(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("스팸 검사는 system 권한만 쓸 수 있습니다."))
	}

	baro_sec_watch(db, config, baro_sec_real_ip(config, query))

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"/baro/security/abuse", tool.Baro_t("악용 방지")}, {"", tool.Baro_t("스팸 검사")}})
	body += tool.Baro_hero(tool.Baro_t("스팸 필터"), tool.Baro_t("설정한 낱말 목록으로 글과 문서를 검사합니다."), "")
	body += baro_sec_tabs("/baro/security/abuse")
	body += baro_sec_abuse_sub_tabs("/baro/security/abuse/spam")

	if !baro_sec_flag(db, "abuse_spam_on", true) {
		body += tool.Baro_alert("warn", tool.Baro_t("스팸 필터가 꺼져 있습니다. 아래 검사는 항상 '걸리지 않음'으로 나옵니다."))
	}

	// --- 시험 도구 (쓰기 없음) ---
	test_text := baro_sec_v(values, "text")
	test_body := tool.Baro_form("/baro/security/abuse/spam",
		tool.Baro_textarea("text", tool.Baro_t("검사할 글"), test_text, 6, tool.Baro_t("여기에 글을 붙여 넣으세요."), tool.Baro_t("이 검사는 아무것도 저장하지 않습니다."))+
			tool.Baro_submit(tool.Baro_t("검사"), "primary"),
	)

	if test_text != "" {
		spam := baro_sec_spam_hits(db, test_text)
		links := baro_sec_link_hits(db, test_text)

		if len(spam) == 0 && len(links) == 0 {
			test_body += tool.Baro_alert("ok", tool.Baro_t("스팸 낱말도 위험 주소도 찾지 못했습니다."))
		} else {
			result := ""
			if len(spam) > 0 {
				result += "스팸 낱말 : " + tool.HTML_escape(strings.Join(spam, ", ")) + "<br>"
			}
			if len(links) > 0 {
				result += "위험 주소 : " + tool.HTML_escape(strings.Join(links, ", "))
			}

			test_body += tool.Baro_alert("danger", result)
		}
	}

	body += tool.Baro_card(tool.Baro_t("글 검사"), test_body)

	// --- 문서 검사 ---
	page := baro_sec_page_num(query)

	if !baro_sec_abuse_rate_ok(db, config.IP) {
		body += tool.Baro_alert("warn", tool.Baro_t("요청 속도 제한에 걸려 문서 검사를 건너뜁니다. 잠시 뒤에 다시 시도하세요."))

		return baro_sec_page(db, config, tool.Baro_t("스팸 필터"), body)
	}

	docs := baro_sec_doc_page(db, (page-1)*baro_sec_scan_size, baro_sec_scan_size)
	table_rows := [][]string{}

	for _, doc := range docs {
		hits := baro_sec_spam_hits(db, doc[1])
		if len(hits) == 0 {
			continue
		}

		table_rows = append(table_rows, []string{
			tool.Baro_doc_link(doc[0]),
			tool.HTML_escape(strings.Join(hits, ", ")),
			tool.Baro_itoa(tool.Get_len(doc[1])),
			tool.Baro_button("/w/"+tool.Url_parser(doc[0]), "문서 보기", ""),
		})
	}

	scan_body := tool.Baro_note(
		"문서 " + tool.Baro_comma(baro_sec_doc_count(db)) + tool.Baro_t("개 가운데 이 쪽에서 ") +
			tool.Baro_itoa(len(docs)) + tool.Baro_t("개를 검사했습니다. (제목 순서, 쪽마다 ") + tool.Baro_itoa(baro_sec_scan_size) + tool.Baro_t("개씩)"),
	)

	if len(table_rows) == 0 {
		scan_body += tool.Baro_empty(tool.Baro_t("이 쪽에서 검사한 문서에는 스팸 낱말이 없습니다."))
	} else {
		scan_body += tool.Baro_table([]string{"문서", tool.Baro_t("걸린 낱말"), tool.Baro_t("본문 길이"), "이동"}, table_rows, "")
	}

	scan_body += tool.Baro_paging(page, len(docs), baro_sec_scan_size, "/baro/security/abuse/spam?page={}")
	body += tool.Baro_card(tool.Baro_t("문서 본문 검사"), scan_body)

	body += tool.Baro_card(tool.Baro_t("걸린 문서를 어떻게 처리하나요"),
		tool.Baro_list([]string{
			tool.Baro_t("낱말 일치일 뿐입니다. 정상 문서(예 : 도박 중독을 다루는 문서)도 걸립니다. 반드시 본문을 확인하세요."),
			tool.Baro_t("명백한 광고라면 삭제가 아니라 <b>임시 격리</b>로 처리합니다. (0장 2번 원칙)"),
			tool.Baro_t("같은 편집자가 여러 문서에 넣었다면 사건으로 묶고, 대량 조치는 승인 절차를 거칩니다."),
			tool.Baro_t("자주 걸리는 표현은 이 화면의 낱말 목록에 추가하세요."),
		}),
	)

	return baro_sec_page(db, config, tool.Baro_t("스팸 필터"), body)
}

// ------------------------------------------------------------
//  /baro/security/abuse/link : 위험 링크 검사
// ------------------------------------------------------------

// View_baro_sec_abuse_link : 위험 링크 검사 화면
func View_baro_sec_abuse_link(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/security/abuse/link"); need {
		return wall
	}

	if !baro_sec_is_system(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("링크 검사는 system 권한만 쓸 수 있습니다."))
	}

	if values != nil {
		return tool.Get_redirect("/baro/security/abuse/link")
	}

	baro_sec_watch(db, config, baro_sec_real_ip(config, query))

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"/baro/security/abuse", tool.Baro_t("악용 방지")}, {"", tool.Baro_t("위험 링크")}})
	body += tool.Baro_hero(tool.Baro_t("위험 링크 검사"), tool.Baro_t("알려진 단축 주소와 의심 도메인이 문서에 들어 있는지 봅니다."), "")
	body += baro_sec_tabs("/baro/security/abuse")
	body += baro_sec_abuse_sub_tabs("/baro/security/abuse/link")

	domains := baro_sec_lines_set(db, "abuse_link_domains", baro_sec_abuse_link_default)

	if !baro_sec_flag(db, "abuse_link_on", true) {
		body += tool.Baro_alert("warn", tool.Baro_t("위험 링크 검사가 꺼져 있습니다. 결과가 비어 있게 나옵니다."))
	}

	body += tool.Baro_card(tool.Baro_t("검사 목록"), tool.Baro_pills(domains))

	page := baro_sec_page_num(query)

	if !baro_sec_abuse_rate_ok(db, config.IP) {
		body += tool.Baro_alert("warn", tool.Baro_t("요청 속도 제한에 걸려 검사를 건너뜁니다. 잠시 뒤에 다시 시도하세요."))

		return baro_sec_page(db, config, tool.Baro_t("위험 링크 검사"), body)
	}

	docs := baro_sec_doc_page(db, (page-1)*baro_sec_scan_size, baro_sec_scan_size)
	table_rows := [][]string{}

	for _, doc := range docs {
		hits := baro_sec_link_hits(db, doc[1])
		if len(hits) == 0 {
			continue
		}

		table_rows = append(table_rows, []string{
			tool.Baro_doc_link(doc[0]),
			tool.HTML_escape(strings.Join(hits, ", ")),
			tool.HTML_escape(baro_sec_link_sample(doc[1], hits[0])),
			tool.Baro_button("/w/"+tool.Url_parser(doc[0]), "문서 보기", ""),
		})
	}

	body += tool.Baro_note(
		tool.Baro_t("이 쪽에서 문서 ") + tool.Baro_itoa(len(docs)) + tool.Baro_t("개를 검사했습니다. (전체 ") + tool.Baro_comma(baro_sec_doc_count(db)) + tool.Baro_t("개)"),
	)

	if len(table_rows) == 0 {
		body += tool.Baro_empty(tool.Baro_t("이 쪽에서 검사한 문서에는 위험 링크가 없습니다."))
	} else {
		body += tool.Baro_table([]string{"문서", tool.Baro_t("걸린 도메인"), tool.Baro_t("본문 일부"), "이동"}, table_rows, "")
	}

	body += tool.Baro_paging(page, len(docs), baro_sec_scan_size, "/baro/security/abuse/link?page={}")

	body += tool.Baro_card(tool.Baro_t("무엇을 하면 되나요"),
		tool.Baro_list([]string{
			tool.Baro_t("단축 주소는 도착지를 감춥니다. 원래 주소로 바꾸거나 출처를 다시 달아 주세요."),
			tool.Baro_t("광고성 링크라면 문서 전체를 지우지 말고 링크만 걷어 내는 편집을 하세요."),
			tool.Baro_t("같은 도메인이 여러 문서에 반복되면 편집 필터 규칙으로 등록해 재발을 막습니다."),
			tool.Baro_t("새로 발견한 도메인은 <a href=\"/baro/security/abuse\">악용 방지 설정</a> 의 목록에 추가하세요."),
		}),
	)

	return baro_sec_page(db, config, tool.Baro_t("위험 링크 검사"), body)
}

// baro_sec_link_sample : 걸린 도메인 주변 본문 조각
//
//	한글이 섞인 본문에서 글자가 잘리지 않도록 글자 경계까지 맞춘 뒤 자른다.
func baro_sec_link_sample(text string, domain string) string {
	lower := strings.ToLower(text)
	index := strings.Index(lower, strings.ToLower(domain))
	if index < 0 {
		return ""
	}

	start := index - 30
	if start < 0 {
		start = 0
	}

	for start > 0 && !utf8.RuneStart(text[start]) {
		start--
	}

	end := index + 50
	if end > len(text) {
		end = len(text)
	}

	for end < len(text) && !utf8.RuneStart(text[end]) {
		end++
	}

	return tool.Baro_one_line(text[start:end])
}

// ------------------------------------------------------------
//  /baro/security/abuse/file : 파일 검사 (확장자 · 매직바이트)
// ------------------------------------------------------------

// baro_sec_magic_kind : 파일 앞부분을 보고 실제 종류를 판정한다
func baro_sec_magic_kind(head []byte) string {
	if len(head) < 4 {
		return "unknown"
	}

	switch {
	case bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF}):
		return "jpg"
	case bytes.HasPrefix(head, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}):
		return "png"
	case bytes.HasPrefix(head, []byte("GIF87a")) || bytes.HasPrefix(head, []byte("GIF89a")):
		return "gif"
	case bytes.HasPrefix(head, []byte("RIFF")) && len(head) >= 12 && bytes.Equal(head[8:12], []byte("WEBP")):
		return "webp"
	case bytes.HasPrefix(head, []byte("RIFF")) && len(head) >= 12 && bytes.Equal(head[8:12], []byte("WAVE")):
		return "wav"
	case bytes.HasPrefix(head, []byte("BM")):
		return "bmp"
	case bytes.HasPrefix(head, []byte("%PDF-")):
		return "pdf"
	case bytes.HasPrefix(head, []byte("ID3")) || bytes.HasPrefix(head, []byte{0xFF, 0xFB}) || bytes.HasPrefix(head, []byte{0xFF, 0xF3}):
		return "mp3"
	case bytes.HasPrefix(head, []byte("OggS")):
		return "ogg"
	case len(head) >= 12 && bytes.Equal(head[4:8], []byte("ftyp")):
		return "mp4"
	case bytes.HasPrefix(head, []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return "webm"
	case bytes.HasPrefix(head, []byte("PK")):
		return "zip"
	case bytes.HasPrefix(head, []byte{0x00, 0x00, 0x01, 0x00}):
		return "ico"
	case bytes.HasPrefix(head, []byte("II*")) || bytes.HasPrefix(head, []byte("MM")):
		return "tiff"
	case bytes.HasPrefix(head, []byte{0x7F, 0x45, 0x4C, 0x46}):
		return "elf"
	case bytes.HasPrefix(head, []byte{0x4D, 0x5A}):
		return "exe"
	}

	lower := strings.ToLower(string(head))
	switch {
	case strings.Contains(lower, "<?php"), strings.Contains(lower, "<script"):
		return "script"
	case strings.Contains(lower, "<svg"):
		return "svg"
	case strings.HasPrefix(strings.TrimSpace(lower), "<?xml"):
		return "xml"
	case strings.Contains(lower, "<!doctype html"), strings.Contains(lower, "<html"):
		return "html"
	}

	return "unknown"
}

// baro_sec_ext_family : 확장자가 기대하는 실제 종류
func baro_sec_ext_family(ext string) string {
	switch strings.ToLower(ext) {
	case "jpg", "jpeg", "jpe":
		return "jpg"
	case "png":
		return "png"
	case "gif":
		return "gif"
	case "webp":
		return "webp"
	case "bmp":
		return "bmp"
	case "pdf":
		return "pdf"
	case "mp3":
		return "mp3"
	case "wav":
		return "wav"
	case "ogg", "oga", "opus":
		return "ogg"
	case "mp4", "m4a", "m4v", "mov", "3gp":
		return "mp4"
	case "webm", "mkv":
		return "webm"
	case "ico":
		return "ico"
	case "tif", "tiff":
		return "tiff"
	case "svg":
		return "svg"
	case "zip", "docx", "xlsx", "pptx":
		return "zip"
	}

	return ""
}

// baro_sec_file_row : 검사한 파일 한 줄
type baro_sec_file_row struct {
	Name   string
	Ext    string
	Size   int64
	Kind   string
	State  string
	Detail string
}

// baro_sec_file_list : 저장된 파일 목록 (이름 순, 최대 max 개)
func baro_sec_file_list(dir string, max int) ([]string, bool) {
	names := []string{}
	cut := false

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if entry.IsDir() {
			return nil
		}

		if len(names) >= max {
			cut = true

			return fs.SkipAll
		}

		names = append(names, path)

		return nil
	})

	if err != nil {
		return names, cut
	}

	sort.Strings(names)

	return names, cut
}

// baro_sec_file_check : 파일 하나를 검사한다
func baro_sec_file_check(db *sql.DB, path string) baro_sec_file_row {
	row := baro_sec_file_row{Name: filepath.Base(path)}

	row.Ext = strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))

	if info, err := os.Stat(path); err == nil {
		row.Size = info.Size()
	}

	file, err := os.Open(path)
	if err != nil {
		row.Kind = "unknown"
		row.State = "warn"
		row.Detail = tool.Baro_t("파일을 열 수 없습니다.")

		return row
	}
	defer file.Close()

	head := make([]byte, 32)
	read, _ := file.Read(head)
	row.Kind = baro_sec_magic_kind(head[:read])

	family := baro_sec_ext_family(row.Ext)

	switch {
	case row.Kind == "script" || row.Kind == "html" || row.Kind == "exe" || row.Kind == "elf":
		row.State = "danger"
		row.Detail = tool.Baro_t("실행·스크립트로 보이는 내용입니다. 즉시 확인하세요.")
	case !baro_sec_ext_allowed(db, row.Ext):
		row.State = "danger"
		row.Detail = tool.Baro_t("허용 목록에 없는 확장자입니다.")
	case family == "":
		row.State = "warn"
		row.Detail = tool.Baro_t("매직바이트 규칙이 없는 확장자입니다. 내용은 확인하지 못했습니다.")
	case row.Kind == "unknown":
		row.State = "warn"
		row.Detail = tool.Baro_t("앞부분만으로는 종류를 알 수 없습니다.")
	case family == "svg" && row.Kind == "xml":
		row.State = "ok"
		row.Detail = tool.Baro_t("XML 선언으로 시작하는 SVG 로 보입니다.")
	case row.Kind != family:
		row.State = "danger"
		row.Detail = tool.Baro_t("확장자는 ") + row.Ext + tool.Baro_t(" 인데 내용은 ") + row.Kind + tool.Baro_t(" 로 보입니다.")
	default:
		row.State = "ok"
		row.Detail = tool.Baro_t("확장자와 내용이 맞습니다.")
	}

	return row
}

// View_baro_sec_abuse_file : 파일 검사 화면
func View_baro_sec_abuse_file(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/security/abuse/file"); need {
		return wall
	}

	if !baro_sec_is_system(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("파일 검사는 system 권한만 쓸 수 있습니다."))
	}

	if values != nil {
		return tool.Get_redirect("/baro/security/abuse/file")
	}

	baro_sec_watch(db, config, baro_sec_real_ip(config, query))

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"/baro/security/abuse", tool.Baro_t("악용 방지")}, {"", tool.Baro_t("파일 검사")}})
	body += tool.Baro_hero(tool.Baro_t("파일 검사"), tool.Baro_t("올라온 파일의 확장자와 실제 내용(매직바이트)이 맞는지 봅니다."), "")
	body += baro_sec_tabs("/baro/security/abuse")
	body += baro_sec_abuse_sub_tabs("/baro/security/abuse/file")

	body += tool.Baro_alert("warn",
		tool.Baro_t("<b>이 도구는 바이러스 백신이 아닙니다.</b> 파일 앞 32바이트와 확장자만 비교합니다. ")+
			tool.Baro_t("내용 속에 숨긴 악성 코드, 압축 파일 안의 파일, 문서 안의 매크로는 찾지 못합니다. ")+
			tool.Baro_t("의심되면 파일을 내려받지 말고 격리한 뒤 전문 도구로 확인하세요."),
	)

	if !baro_sec_flag(db, "abuse_file_on", true) {
		body += tool.Baro_alert("info", tool.Baro_t("파일 검사 설정이 꺼져 있습니다. 결과는 참고용으로만 보세요."))
	}

	dir := tool.Get_file_main_dir(db)

	if _, err := os.Stat(dir); err != nil {
		body += tool.Baro_empty(tool.Baro_t("파일 보관 폴더(") + dir + tool.Baro_t(")가 아직 없습니다. 올라온 파일이 없다는 뜻입니다."))

		return baro_sec_page(db, config, tool.Baro_t("파일 검사"), body)
	}

	if !baro_sec_abuse_rate_ok(db, config.IP) {
		body += tool.Baro_alert("warn", tool.Baro_t("요청 속도 제한에 걸려 검사를 건너뜁니다. 잠시 뒤에 다시 시도하세요."))

		return baro_sec_page(db, config, tool.Baro_t("파일 검사"), body)
	}

	names, cut := baro_sec_file_list(dir, 3000)

	page := baro_sec_page_num(query)
	start := (page - 1) * tool.Baro_page_size
	end := start + tool.Baro_page_size
	if start > len(names) {
		start = len(names)
	}
	if end > len(names) {
		end = len(names)
	}

	table_rows := [][]string{}
	danger := 0
	warn := 0

	for _, path := range names[start:end] {
		row := baro_sec_file_check(db, path)

		badge := tool.Baro_badge(tool.Baro_t("정상"), "ok")
		if row.State == "danger" {
			badge = tool.Baro_badge(tool.Baro_t("위험"), "danger")
			danger++
		} else if row.State == "warn" {
			badge = tool.Baro_badge(tool.Baro_t("확인 필요"), "warn")
			warn++
		}

		table_rows = append(table_rows, []string{
			tool.HTML_escape(tool.Baro_trunc(row.Name, 50)),
			tool.HTML_escape(row.Ext),
			tool.Baro_comma(int(row.Size)) + " B",
			tool.HTML_escape(row.Kind),
			badge,
			tool.HTML_escape(row.Detail),
		})
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("보관된 파일"), tool.Baro_comma(len(names)), func() string {
			if cut {
				return tool.Baro_t("3000개까지만 셉니다")
			}

			return tool.Baro_t("전체")
		}()},
		{tool.Baro_t("이 쪽 위험"), tool.Baro_itoa(danger), tool.Baro_t("확장자와 내용 불일치 등")},
		{tool.Baro_t("이 쪽 확인 필요"), tool.Baro_itoa(warn), tool.Baro_t("판정하지 못한 파일")},
		{tool.Baro_t("보관 폴더"), tool.Baro_trunc(dir, 24), tool.Baro_t("openNAMU 설정 image_where")},
	})

	if len(table_rows) == 0 {
		body += tool.Baro_empty(tool.Baro_t("검사할 파일이 없습니다."))
	} else {
		body += tool.Baro_table([]string{tool.Baro_t("파일"), tool.Baro_t("확장자"), tool.Baro_t("크기"), tool.Baro_t("실제 종류"), tool.Baro_t("판정"), tool.Baro_t("설명")}, table_rows, "")
	}

	body += tool.Baro_paging(page, len(table_rows), tool.Baro_page_size, "/baro/security/abuse/file?page={}")

	body += tool.Baro_card(tool.Baro_t("위험으로 나온 파일은"),
		tool.Baro_list([]string{
			tool.Baro_t("파일 문서를 임시 격리해 일반 이용자에게 보이지 않게 합니다. (삭제가 먼저가 아닙니다)"),
			tool.Baro_t("올린 사람에게 사유를 알리고, 정상 파일이면 다시 올릴 수 있게 안내합니다."),
			tool.Baro_t("허용 확장자 목록을 좁히면 같은 문제가 줄어듭니다."),
			tool.Baro_t("확장자와 내용이 다른 파일이 반복해서 올라오면 사건으로 다루세요."),
		}),
	)

	return baro_sec_page(db, config, tool.Baro_t("파일 검사"), body)
}
