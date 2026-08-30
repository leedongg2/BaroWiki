package route

import (
	"database/sql"
	"net/url"
	"os"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  4장 파일
//
//  업로드 자체는 openNAMU 본체 /upload 가 처리한다.
//  여기서는 그 위에 필요한 것을 만든다.
//    4-2 라이선스 정보 (P0) / 4-3 중복 파일 감지 (P1) / 4-4 개인정보 보호 처리 (P1)
// ============================================================

// baro_find_file_state : 파일 한 개의 상태 요약
type baro_find_file_state struct {
	Title    string
	Meta     map[string]string
	Path     string
	Size     int64
	Exists   bool
	Hash     string
	NoSource bool
	Exif     bool
	Jpeg     bool
}

// baro_find_file_state_get : 파일 상태 읽기 (해시는 저장된 값만 쓴다)
func baro_find_file_state_get(db *sql.DB, title string) baro_find_file_state {
	out := baro_find_file_state{Title: title}
	out.Meta = baro_find_file_meta(db, title)
	out.Path = baro_find_file_path(db, title)
	out.Hash = out.Meta["hash"]
	out.NoSource = strings.TrimSpace(out.Meta["source"]) == "" || strings.TrimSpace(out.Meta["license"]) == ""
	out.Exif = out.Meta["exif_removed"] == "on"

	_, ext := baro_find_file_split(title)
	out.Jpeg = ext == "jpg" || ext == "jpeg"

	if out.Path != "" {
		if info, err := os.Stat(out.Path); err == nil {
			out.Exists = true
			out.Size = info.Size()
		}
	}

	return out
}

// baro_find_size_text : 파일 크기 표시
func baro_find_size_text(size int64) string {
	if size <= 0 {
		return "-"
	}
	if size < 1024 {
		return tool.Baro_comma(int(size)) + " B"
	}
	if size < 1024*1024 {
		return tool.Baro_comma(int(size/1024)) + " KB"
	}

	return tool.Baro_comma(int(size/1024/1024)) + " MB"
}

// baro_find_file_hash : 파일 내용의 SHA224 를 계산해 저장한다
func baro_find_file_hash(db *sql.DB, title string) (string, string) {
	path := baro_find_file_path(db, title)
	if path == "" {
		return "", tool.Baro_t("파일 경로를 알 수 없습니다.")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", tool.Baro_t("파일을 읽지 못했습니다.")
	}

	hash := tool.Sha224(string(raw))

	meta := baro_find_file_meta(db, title)
	if meta["exist"] != "on" {
		meta = map[string]string{"date": tool.Baro_now()}
	}
	meta["hash"] = hash
	baro_find_file_meta_save(db, title, meta)

	return hash, ""
}

// ------------------------------------------------------------
//  EXIF 제거 (표준 라이브러리만으로 직접 구현)
// ------------------------------------------------------------

// baro_find_exif_has : JPEG 안에 EXIF(APP1) 구간이 있는지
func baro_find_exif_has(raw []byte) bool {
	_, found := baro_find_jpeg_strip(raw)

	return found
}

// baro_find_jpeg_strip : JPEG 에서 APP1(EXIF·XMP) 구간을 잘라 낸다
//
//	두 번째 값이 true 면 실제로 지운 구간이 있었다는 뜻이다.
func baro_find_jpeg_strip(raw []byte) ([]byte, bool) {
	if len(raw) < 4 || raw[0] != 0xFF || raw[1] != 0xD8 {
		return raw, false
	}

	out := make([]byte, 0, len(raw))
	out = append(out, 0xFF, 0xD8)

	removed := false
	index := 2

	for index < len(raw) {
		// 표식은 0xFF 로 시작한다. 채움 바이트(0xFF)는 건너뛴다.
		if raw[index] != 0xFF {
			out = append(out, raw[index:]...)
			break
		}

		next := index + 1
		for next < len(raw) && raw[next] == 0xFF {
			next++
		}
		if next >= len(raw) {
			out = append(out, raw[index:]...)
			break
		}

		marker := raw[next]

		// 길이가 없는 표식
		if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			out = append(out, raw[index:next+1]...)
			index = next + 1

			continue
		}

		// 압축 자료 시작 / 그림 끝 — 나머지는 그대로 둔다
		if marker == 0xDA || marker == 0xD9 {
			out = append(out, raw[index:]...)
			break
		}

		if next+2 >= len(raw) {
			out = append(out, raw[index:]...)
			break
		}

		length := int(raw[next+1])<<8 | int(raw[next+2])
		if length < 2 {
			out = append(out, raw[index:]...)
			break
		}

		end := next + 1 + length
		if end > len(raw) {
			out = append(out, raw[index:]...)
			break
		}

		payload := raw[next+3 : end]

		if marker == 0xE1 && baro_find_app1_private(payload) {
			// EXIF(위치정보 포함) 와 XMP 는 통째로 뺀다.
			removed = true
			index = end

			continue
		}

		out = append(out, raw[index:end]...)
		index = end
	}

	return out, removed
}

// baro_find_app1_private : APP1 구간이 EXIF 또는 XMP 인지
func baro_find_app1_private(payload []byte) bool {
	if len(payload) >= 6 && string(payload[:6]) == "Exif\x00\x00" {
		return true
	}
	if len(payload) >= 28 && strings.HasPrefix(string(payload[:28]), "http://ns.adobe.com/xap/1.0/") {
		return true
	}

	return false
}

// baro_find_exif_run : 파일 하나에서 EXIF 를 지운다 (원본은 백업해 둔다)
func baro_find_exif_run(db *sql.DB, config tool.Config, title string) string {
	path := baro_find_file_path(db, title)
	if path == "" {
		return tool.Baro_t("파일 경로를 알 수 없습니다.")
	}

	_, ext := baro_find_file_split(title)
	if ext != "jpg" && ext != "jpeg" {
		return tool.Baro_t("EXIF 는 JPEG 파일에만 들어 있습니다.")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return tool.Baro_t("파일을 읽지 못했습니다.")
	}

	stripped, removed := baro_find_jpeg_strip(raw)
	if !removed {
		// 지울 것이 없어도 처리 완료로 기록해 목록에서 빠지게 한다.
		baro_find_exif_mark(db, title, config.IP)
		tool.Baro_audit(db, config, "file_exif_none", title, "", "", "EXIF 구간 없음")

		return tool.Baro_t("이 파일에는 지울 EXIF 정보가 없었습니다. 처리 완료로 표시했습니다.")
	}

	// 2번 원칙 : 지우기 전에 원본을 남긴다.
	backup := path + ".exifbak"
	if _, err := os.Stat(backup); err != nil {
		if err := os.WriteFile(backup, raw, 0o644); err != nil {
			return tool.Baro_t("원본 백업에 실패해 중단했습니다.")
		}
	}

	if err := os.WriteFile(path, stripped, 0o644); err != nil {
		return tool.Baro_t("파일을 다시 쓰지 못했습니다.")
	}

	baro_find_exif_mark(db, title, config.IP)

	// 파일 내용이 바뀌었으므로 중복 판정에 쓰는 해시를 다시 계산한다.
	// (다시 계산하지 않으면 처리 전 해시가 남아 서로 다른 파일이 중복으로 잘못 묶인다)
	baro_find_file_hash(db, title)

	tool.Baro_public_log(db, "file_exif_remove", config.IP, title, "위치정보 등 EXIF 제거", "")
	tool.Baro_audit(db, config, "file_exif_remove", title,
		"원본 "+tool.Baro_comma(len(raw))+"바이트", "처리 후 "+tool.Baro_comma(len(stripped))+"바이트", "개인정보 보호 처리")

	return ""
}

// baro_find_exif_mark : EXIF 처리 완료 기록
func baro_find_exif_mark(db *sql.DB, title string, actor string) {
	meta := baro_find_file_meta(db, title)
	if meta["exist"] != "on" {
		meta = map[string]string{"date": tool.Baro_now(), "uploader": ""}
	}

	meta["exif_removed"] = "on"
	if meta["date"] == "" {
		meta["date"] = tool.Baro_now()
	}

	baro_find_file_meta_save(db, title, meta)

	_ = actor
}

// ------------------------------------------------------------
//  파일 목록
// ------------------------------------------------------------

// baro_find_file_filter_list : 파일 목록 거르기
func baro_find_file_filter_list() [][]string {
	return [][]string{
		{"", tool.Baro_t("전체")},
		{"nosource", tool.Baro_t("출처·라이선스 없음 (검토 대기)")},
		{"nohash", tool.Baro_t("해시 미계산")},
		{"noexif", tool.Baro_t("EXIF 미처리 (JPEG)")},
		{"missing", tool.Baro_t("실제 파일 없음")},
	}
}

// View_baro_find_file : 파일 목록 (/baro/file)
func View_baro_find_file(config tool.Config, page string, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	keyword := baro_find_val(query, "q")
	filter := baro_find_val(query, "filter")
	message := baro_find_val(query, "msg")

	page_int := baro_find_page_int(page)
	if raw_page := baro_find_val(query, "page"); raw_page != "" {
		page_int = baro_find_page_int(raw_page)
	}

	all := baro_find_file_title_list(db, 3000)
	hidden := baro_find_hidden_set(db)

	// 같은 해시를 가진 파일을 미리 세어 둔다.
	hash_count := map[string]int{}
	hash_rows := tool.Query_DB(db, "select hash, count(*) from baro_file_meta where not hash = '' group by hash")
	defer hash_rows.Close()

	for hash_rows.Next() {
		hash := ""
		count := 0
		if err := hash_rows.Scan(&hash, &count); err != nil {
			continue
		}

		hash_count[hash] = count
	}

	kept := []baro_find_file_state{}
	stat_nosource, stat_dup, stat_exif, stat_missing := 0, 0, 0, 0

	for _, title := range all {
		if hidden[title] {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(keyword)) {
			continue
		}

		state := baro_find_file_state_get(db, title)

		if state.NoSource {
			stat_nosource++
		}
		if state.Hash != "" && hash_count[state.Hash] > 1 {
			stat_dup++
		}
		if state.Jpeg && !state.Exif {
			stat_exif++
		}
		if !state.Exists {
			stat_missing++
		}

		switch filter {
		case "nosource":
			if !state.NoSource {
				continue
			}
		case "nohash":
			if state.Hash != "" {
				continue
			}
		case "noexif":
			if !state.Jpeg || state.Exif {
				continue
			}
		case "missing":
			if state.Exists {
				continue
			}
		}

		kept = append(kept, state)
	}

	start := (page_int - 1) * tool.Baro_page_size
	if start > len(kept) {
		start = len(kept)
	}
	end := start + tool.Baro_page_size
	if end > len(kept) {
		end = len(kept)
	}

	rows := [][]string{}
	for _, state := range kept[start:end] {
		license_cell := tool.Baro_badge(baro_find_license_name(state.Meta["license"]), "ok")
		if state.NoSource {
			license_cell = tool.Baro_badge(tool.Baro_t("검토 대기"), "warn")
		}

		dup_cell := `<span class="bw_muted">-</span>`
		if state.Hash == "" {
			dup_cell = tool.Baro_badge(tool.Baro_t("해시 없음"), "muted")
		} else if hash_count[state.Hash] > 1 {
			dup_cell = tool.Baro_badge(tool.Baro_t("중복 ")+tool.Baro_itoa(hash_count[state.Hash])+tool.Baro_t("건"), "danger")
		} else {
			dup_cell = tool.Baro_badge(tool.Baro_t("단일"), "ok")
		}

		exif_cell := tool.Baro_t(`<span class="bw_muted">JPEG 아님</span>`)
		if state.Jpeg {
			if state.Exif {
				exif_cell = tool.Baro_badge("처리 완료", "ok")
			} else {
				exif_cell = tool.Baro_badge(tool.Baro_t("미처리"), "warn")
			}
		}

		file_cell := baro_find_size_text(state.Size)
		if !state.Exists {
			file_cell = tool.Baro_badge(tool.Baro_t("파일 없음"), "danger")
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(state.Title),
			license_cell,
			dup_cell,
			exif_cell,
			file_cell,
			tool.Baro_button("/baro/file/meta/"+tool.Url_parser(state.Title), "정보", ""),
		})
	}

	filter_form := tool.Baro_input("q", tool.Baro_t("파일 이름"), keyword, tool.Baro_t("이름 일부"), "")
	filter_form += tool.Baro_select("filter", tool.Baro_t("거르기"), baro_find_file_filter_list(), filter, "")
	filter_form += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("적용"), "primary") + `</div>`

	page_url := url.Values{}
	page_url.Set("q", keyword)
	page_url.Set("filter", filter)

	list_html := tool.Baro_table(
		[]string{tool.Baro_t("파일 문서"), "라이선스", tool.Baro_t("중복"), "EXIF", tool.Baro_t("크기"), "관리"},
		rows, "",
	)
	if len(kept) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("조건에 맞는 파일이 없습니다. 파일을 올리려면 위키의 파일 올리기 화면을 쓰세요."))
	}
	list_html += tool.Baro_paging(page_int, end-start, tool.Baro_page_size, "/baro/file?"+page_url.Encode()+"&page={}")

	body := baro_find_tabs("/baro/file")
	body += tool.Baro_hero(tool.Baro_t("파일"), tool.Baro_t("라이선스 · 중복 · 개인정보를 한 화면에서 확인합니다."),
		tool.Baro_button("/baro/file/nosource", tool.Baro_t("출처 없는 파일"), "primary")+" "+
			tool.Baro_button("/baro/file/dup", tool.Baro_t("중복 파일"), "")+" "+
			tool.Baro_button("/baro/file/privacy", tool.Baro_t("개인정보 처리"), "")+" "+
			tool.Baro_button("/upload", tool.Baro_t("파일 올리기"), ""))

	if message != "" {
		body += tool.Baro_alert("info", tool.HTML_escape(message))
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("파일 문서"), tool.Baro_comma(len(all)), ""},
		{tool.Baro_t("검토 대기"), tool.Baro_comma(stat_nosource), tool.Baro_t("출처 또는 라이선스 없음")},
		{tool.Baro_t("중복 의심"), tool.Baro_comma(stat_dup), tool.Baro_t("같은 해시")},
		{tool.Baro_t("EXIF 미처리"), tool.Baro_comma(stat_exif), tool.Baro_t("JPEG 기준")},
		{tool.Baro_t("파일 없음"), tool.Baro_comma(stat_missing), tool.Baro_t("문서만 남은 것")},
	})

	body += tool.Baro_card(tool.Baro_t("찾기"), `<form class="bw_form" method="get" action="/baro/file">`+filter_form+`</form>`)
	body += tool.Baro_card(tool.Baro_t("파일 목록"), list_html)

	body += tool.Baro_card(tool.Baro_t("이 화면에서 하는 일"), tool.Baro_dl([][]string{
		{"라이선스", tool.Baro_t("출처·제작자·촬영자·사용 근거를 파일마다 남깁니다. 없으면 <b>검토 대기</b> 로 표시됩니다.")},
		{tool.Baro_t("중복"), tool.Baro_t("파일 내용의 SHA224 해시를 계산해 같은 파일을 묶습니다.")},
		{"EXIF", tool.Baro_t("JPEG 안의 촬영 위치·기기 정보를 지웁니다. 원본은 따로 백업합니다.")},
	}))

	return tool.Baro_page(db, config, tool.Baro_t("파일"), body, baro_find_menu())
}

// View_baro_find_file_meta : 4-2 파일 라이선스 정보 (/baro/file/meta/<파일문서>)
func View_baro_find_file_meta(config tool.Config, doc_name string, post url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	doc_name = tool.Namespace_normalize(strings.TrimSpace(doc_name))
	if doc_name == "" {
		return tool.Get_redirect("/baro/file")
	}

	if post != nil {
		if wall, need := tool.Baro_login_wall(db, config, "/baro/file"); need {
			return wall
		}
		if tool.Baro_read_only(db) {
			return tool.Baro_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db))
		}
		if !baro_find_doc_exists(db, doc_name) {
			return tool.Get_redirect("/baro/file")
		}

		before := tool.Baro_json_enc(baro_find_file_meta(db, doc_name))
		old := baro_find_file_meta(db, doc_name)

		meta := map[string]string{
			"source":       tool.Baro_trunc(baro_find_val(post, "source"), 500),
			"author":       tool.Baro_trunc(baro_find_val(post, "author"), 200),
			"photographer": tool.Baro_trunc(baro_find_val(post, "photographer"), 200),
			"license":      baro_find_val(post, "license"),
			"basis":        tool.Baro_trunc(baro_find_val(post, "basis"), 500),
			"shot_date":    tool.Baro_trunc(baro_find_val(post, "shot_date"), 20),
			"description":  tool.Baro_trunc(baro_find_val(post, "description"), 1000),
			"related":      tool.Baro_trunc(baro_find_val(post, "related"), 500),
			"hash":         old["hash"],
			"exif_removed": old["exif_removed"],
			"date":         old["date"],
			"uploader":     old["uploader"],
		}

		if meta["date"] == "" {
			meta["date"] = tool.Baro_now()
		}
		if meta["uploader"] == "" {
			meta["uploader"] = config.IP
		}

		baro_find_file_meta_save(db, doc_name, meta)

		// 저장할 때 해시가 없으면 함께 계산해 둔다.
		if meta["hash"] == "" {
			baro_find_file_hash(db, doc_name)
		}

		// 원본 접근 권한 분리 (문서 메타에 남긴다)
		restricted := ""
		if baro_find_val(post, "original_restricted") == "on" {
			restricted = "on"
		}
		tool.Baro_doc_meta_set(db, doc_name, "original_restricted", restricted)

		after := tool.Baro_json_enc(baro_find_file_meta(db, doc_name))
		tool.Baro_audit(db, config, "file_meta_edit", doc_name, before, after, "파일 라이선스 정보 편집")
		tool.Baro_public_log(db, "file_meta_edit", config.IP, doc_name, "파일 라이선스 정보 편집",
			baro_find_license_name(meta["license"]))

		if meta["source"] != "" {
			tool.Baro_contrib_add(db, config.IP, "source", doc_name, "", "", "파일 출처 기록")
		}

		return tool.Get_redirect("/baro/file/meta/" + tool.Url_parser(doc_name))
	}

	body := baro_find_tabs("/baro/file")
	body += tool.Baro_crumb([][]string{
		{"/baro/file", tool.Baro_t("파일")},
		{"", tool.Namespace_display(doc_name)},
	})

	if !baro_find_doc_exists(db, doc_name) {
		body += tool.Baro_alert("warn", `<b>`+tool.HTML_escape(tool.Namespace_display(doc_name))+tool.Baro_t(`</b> 파일 문서가 없습니다.`))
		body += tool.Baro_card(tool.Baro_t("무엇을 할 수 있나요"),
			tool.Baro_button("/upload", tool.Baro_t("파일 올리기"), "primary")+" "+
				tool.Baro_button("/baro/file", tool.Baro_t("파일 목록으로"), ""))

		return tool.Baro_page(db, config, tool.Baro_t("파일 정보 : ")+tool.Namespace_display(doc_name), body, baro_find_menu())
	}

	state := baro_find_file_state_get(db, doc_name)
	meta := state.Meta
	restricted := tool.Baro_doc_meta_get(db, doc_name, "original_restricted")

	preview := ""
	if state.Exists {
		name, ext := baro_find_file_split(doc_name)
		if ext == "jpg" || ext == "jpeg" || ext == "png" || ext == "gif" || ext == "webp" {
			preview = `<img src="/image/` + tool.Url_parser(tool.File_name_to_dir(name, ext)) +
				`" alt="` + tool.HTML_escape(tool.Namespace_display(doc_name)) + `" style="max-width:100%;height:auto;border-radius:8px">`
		}
	}

	body += tool.Baro_hero(tool.Namespace_display(doc_name), baro_find_license_name(meta["license"]),
		tool.Baro_button("/w/"+tool.Url_parser(doc_name), "파일 문서 보기", "primary")+" "+
			tool.Baro_button("/baro/file/dup?q="+tool.Url_parser(state.Hash), "같은 파일 찾기", "")+" "+
			tool.Baro_button("/baro/file/privacy", tool.Baro_t("개인정보 처리"), ""))

	if state.NoSource {
		body += tool.Baro_alert("warn", tool.Baro_t("출처 또는 라이선스가 비어 있어 <b>검토 대기</b> 로 표시됩니다. 아래에서 채워 주세요."))
	}
	if !state.Exists {
		body += tool.Baro_alert("danger", tool.Baro_t("문서는 있지만 실제 파일을 찾지 못했습니다. 업로드가 끊겼거나 파일이 옮겨졌을 수 있습니다."))
	}

	if preview != "" {
		body += tool.Baro_card(tool.Baro_t("미리 보기"), preview)
	}

	body += tool.Baro_card(tool.Baro_t("파일 정보"), tool.Baro_dl([][]string{
		{"출처", tool.HTML_escape(meta["source"])},
		{tool.Baro_t("제작자"), tool.HTML_escape(meta["author"])},
		{tool.Baro_t("촬영자"), tool.HTML_escape(meta["photographer"])},
		{"라이선스", tool.HTML_escape(baro_find_license_name(meta["license"]))},
		{tool.Baro_t("사용 근거"), tool.HTML_escape(meta["basis"])},
		{tool.Baro_t("촬영일"), tool.HTML_escape(meta["shot_date"])},
		{tool.Baro_t("설명"), tool.HTML_escape(meta["description"])},
		{tool.Baro_t("관련 문서"), tool.HTML_escape(meta["related"])},
		{tool.Baro_t("파일 해시"), tool.Baro_code(meta["hash"])},
		{tool.Baro_t("EXIF 처리"), tool.HTML_escape(meta["exif_removed"])},
		{tool.Baro_t("파일 크기"), tool.HTML_escape(baro_find_size_text(state.Size))},
		{tool.Baro_t("원본 접근 제한"), tool.HTML_escape(restricted)},
		{tool.Baro_t("기록자"), tool.Baro_user_link(db, meta["uploader"], config.IP)},
		{tool.Baro_t("기록 시각"), tool.Baro_time_cell(meta["date"])},
	}))

	if tool.Baro_is_user(config.IP) {
		form := tool.Baro_textarea("source", "출처", meta["source"], 3, tool.Baro_t("예: 국사편찬위원회 한국사데이터베이스, 접속일 2026-01-01"),
			tool.Baro_t("어디서 가져왔는지 사람이 확인할 수 있게 적습니다. 주소만 있으면 링크가 끊겼을 때 확인할 수 없습니다."))
		form += tool.Baro_input("author", tool.Baro_t("제작자"), meta["author"], tool.Baro_t("그림·문서를 만든 사람"), "")
		form += tool.Baro_input("photographer", tool.Baro_t("촬영자"), meta["photographer"], tool.Baro_t("사진을 찍은 사람"), tool.Baro_t("제작자와 다를 수 있습니다."))
		form += tool.Baro_select("license", "라이선스", baro_find_license_options(), meta["license"], "")
		form += tool.Baro_textarea("basis", tool.Baro_t("사용 근거"), meta["basis"], 3, tool.Baro_t("예: 저작권 만료 (1945년 이전 발행, 저작자 사후 70년 경과)"),
			tool.Baro_t("왜 이 파일을 올려도 되는지 적습니다. 라이선스만으로 부족할 때 특히 중요합니다."))
		form += tool.Baro_input_type("shot_date", tool.Baro_t("촬영일"), meta["shot_date"], "date", "")
		form += tool.Baro_textarea("description", tool.Baro_t("설명"), meta["description"], 3, tool.Baro_t("무엇을 담은 파일인지"), "")
		form += tool.Baro_input("related", tool.Baro_t("관련 문서"), meta["related"], tool.Baro_t("쉼표로 구분"), tool.Baro_t("이 파일이 쓰이는 문서를 적어 둡니다."))
		form += tool.Baro_checkbox("original_restricted", "원본 접근 권한 분리", "on", restricted == "on",
			tool.Baro_t("켜면 원본(고해상도·EXIF 포함본)은 담당 역할만 볼 수 있는 파일로 다룹니다."))
		form += `<div class="bw_f">` + tool.Baro_submit("저장", "primary") + `</div>`

		body += tool.Baro_card("정보 편집", tool.Baro_form("/baro/file/meta/"+tool.Url_parser(doc_name), form))
	} else {
		body += tool.Baro_card("정보 편집", tool.Baro_alert("info",
			tool.Baro_t("파일 정보 편집은 로그인한 기여자만 할 수 있습니다. ")+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary")))
	}

	return tool.Baro_page(db, config, tool.Baro_t("파일 정보 : ")+tool.Namespace_display(doc_name), body, baro_find_menu())
}

// View_baro_find_file_nosource : 출처 없는 파일 모아 보기 (/baro/file/nosource)
func View_baro_find_file_nosource(config tool.Config, page string, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_int := baro_find_page_int(page)
	if raw_page := baro_find_val(query, "page"); raw_page != "" {
		page_int = baro_find_page_int(raw_page)
	}

	hidden := baro_find_hidden_set(db)
	kept := []baro_find_file_state{}

	for _, title := range baro_find_file_title_list(db, 3000) {
		if hidden[title] {
			continue
		}

		state := baro_find_file_state_get(db, title)
		if !state.NoSource {
			continue
		}

		kept = append(kept, state)
	}

	start := (page_int - 1) * tool.Baro_page_size
	if start > len(kept) {
		start = len(kept)
	}
	end := start + tool.Baro_page_size
	if end > len(kept) {
		end = len(kept)
	}

	rows := [][]string{}
	for _, state := range kept[start:end] {
		missing := []string{}
		if strings.TrimSpace(state.Meta["source"]) == "" {
			missing = append(missing, "출처")
		}
		if strings.TrimSpace(state.Meta["license"]) == "" {
			missing = append(missing, "라이선스")
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(state.Title),
			tool.Baro_badge(tool.Baro_t("검토 대기"), "warn"),
			tool.HTML_escape(strings.Join(missing, " · ") + " 없음"),
			tool.HTML_escape(baro_find_size_text(state.Size)),
			tool.Baro_button("/baro/file/meta/"+tool.Url_parser(state.Title), "정보 채우기", "primary"),
		})
	}

	list_html := tool.Baro_table([]string{tool.Baro_t("파일 문서"), "상태", tool.Baro_t("빠진 항목"), tool.Baro_t("크기"), "관리"}, rows, "")
	if len(kept) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("출처와 라이선스가 모두 채워져 있습니다. 좋습니다."))
	}
	list_html += tool.Baro_paging(page_int, end-start, tool.Baro_page_size, "/baro/file/nosource?page={}")

	body := baro_find_tabs("/baro/file")
	body += tool.Baro_crumb([][]string{
		{"/baro/file", tool.Baro_t("파일")},
		{"", tool.Baro_t("출처 없는 파일")},
	})
	body += tool.Baro_hero(tool.Baro_t("출처 없는 파일"), tool.Baro_t("검토 대기 ")+tool.Baro_comma(len(kept))+tool.Baro_t("건"),
		tool.Baro_button("/baro/file", tool.Baro_t("전체 파일 목록"), "primary"))

	body += tool.Baro_alert("info",
		tool.Baro_t("출처가 없는 파일은 <b>지우지 않고</b> 여기에 모읍니다. 먼저 출처를 찾아 채우고, 끝내 찾지 못할 때만 다음 단계를 밟습니다."))

	body += tool.Baro_card(tool.Baro_t("검토 대기 목록"), list_html)

	body += tool.Baro_card(tool.Baro_t("출처를 찾는 방법"), tool.Baro_list([]string{
		tool.Baro_t(`파일 문서의 편집 역사를 보고 올린 사람에게 물어봅니다.`),
		tool.Baro_t(`같은 그림이 실린 책·신문·기관 자료를 찾아 <b>출처</b> 칸에 적습니다.`),
		tool.Baro_t(`공공기관 자료라면 공공누리 유형을 확인해 라이선스로 고릅니다.`),
		tool.Baro_t(`끝내 확인되지 않으면 <b>저작권 불명</b> 으로 표시하고 저작권 담당자에게 넘깁니다.`),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("출처 없는 파일"), body, baro_find_menu())
}

// View_baro_find_file_dup : 4-3 중복 파일 감지 (/baro/file/dup)
func View_baro_find_file_dup(config tool.Config, query url.Values, post url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if post != nil {
		if wall, need := tool.Baro_login_wall(db, config, "/baro/file/dup"); need {
			return wall
		}
		if !tool.Baro_can(db, config.IP, "review") {
			return tool.Baro_deny(db, config, tool.Baro_t("해시 검사를 돌리려면 문서 검토 권한이 필요합니다."))
		}
		if tool.Baro_read_only(db) {
			return tool.Baro_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db))
		}

		done, fail := 0, 0
		for _, title := range baro_find_file_title_list(db, 3000) {
			state := baro_find_file_state_get(db, title)
			if state.Hash != "" && baro_find_val(post, "again") != "on" {
				continue
			}

			if _, err := baro_find_file_hash(db, title); err != "" {
				fail++
				continue
			}

			done++

			if done >= 300 {
				break
			}
		}

		tool.Baro_audit(db, config, "file_hash_scan", "", "", tool.Baro_itoa(done)+"건 계산 / "+tool.Baro_itoa(fail)+"건 실패", "중복 파일 검사")
		tool.Baro_public_log(db, "file_hash_scan", config.IP, "", "중복 파일 검사", tool.Baro_itoa(done)+"건")

		return tool.Get_redirect("/baro/file/dup?msg=" + tool.Url_parser(tool.Baro_itoa(done)+"건의 해시를 계산했습니다. (실패 "+tool.Baro_itoa(fail)+"건)"))
	}

	keyword := baro_find_val(query, "q")
	message := baro_find_val(query, "msg")

	// 해시별 묶음
	group := map[string][]string{}
	no_hash := 0

	rows := tool.Query_DB(db, "select title, hash from baro_file_meta order by title limit 3000")
	defer rows.Close()

	for rows.Next() {
		title, hash := "", ""
		if err := rows.Scan(&title, &hash); err != nil {
			continue
		}

		if strings.TrimSpace(hash) == "" {
			no_hash++
			continue
		}
		if keyword != "" && !strings.Contains(hash, keyword) && !strings.Contains(strings.ToLower(title), strings.ToLower(keyword)) {
			continue
		}

		group[hash] = append(group[hash], title)
	}

	hash_list := []string{}
	for hash, list := range group {
		if len(list) > 1 {
			hash_list = append(hash_list, hash)
		}
	}
	sort.Strings(hash_list)

	dup_body := ""
	for _, hash := range hash_list {
		list := group[hash]
		sort.Strings(list)

		rows_html := [][]string{}
		for index, title := range list {
			state := baro_find_file_state_get(db, title)
			role := tool.Baro_badge(tool.Baro_t("중복본"), "warn")
			if index == 0 {
				role = tool.Baro_badge(tool.Baro_t("대표본 (먼저 올라온 것)"), "ok")
			}

			rows_html = append(rows_html, []string{
				tool.Baro_doc_link(title),
				role,
				tool.HTML_escape(baro_find_size_text(state.Size)),
				tool.HTML_escape(baro_find_license_name(state.Meta["license"])),
				tool.Baro_button("/baro/file/meta/"+tool.Url_parser(title), "정보", ""),
			})
		}

		dup_body += tool.Baro_card_ex(
			tool.Baro_t("같은 내용의 파일 ")+tool.Baro_itoa(len(list))+tool.Baro_t("개"),
			"SHA224 "+tool.Baro_trunc(hash, 24),
			tool.Baro_table([]string{tool.Baro_t("파일 문서"), tool.Baro_t("구분"), tool.Baro_t("크기"), "라이선스", "관리"}, rows_html, ""),
			tool.Baro_note(tool.Baro_t("새로 올리는 대신 대표본을 그대로 쓰면 됩니다. 문서 본문에서 대표본 이름을 부르세요.")),
		)
	}

	if len(hash_list) == 0 {
		dup_body = tool.Baro_empty(tool.Baro_t("같은 내용의 파일을 찾지 못했습니다. 해시가 아직 계산되지 않았다면 아래에서 검사를 돌려 주세요."))
	}

	// 동일 파일명 경고 (확장자만 다른 같은 이름)
	name_group := map[string][]string{}
	for _, title := range baro_find_file_title_list(db, 3000) {
		name, _ := baro_find_file_split(title)
		if name == "" {
			continue
		}

		key := strings.ToLower(name)
		name_group[key] = append(name_group[key], title)
	}

	name_rows := [][]string{}
	name_keys := []string{}
	for key, list := range name_group {
		if len(list) > 1 {
			name_keys = append(name_keys, key)
		}
	}
	sort.Strings(name_keys)

	for _, key := range name_keys {
		list := name_group[key]
		sort.Strings(list)

		links := []string{}
		for _, title := range list {
			links = append(links, tool.Baro_doc_link(title))
		}

		name_rows = append(name_rows, []string{
			tool.HTML_escape(key),
			strings.Join(links, ", "),
			tool.Baro_badge(tool.Baro_itoa(len(list))+tool.Baro_t("개"), "warn"),
		})

		if len(name_rows) >= 50 {
			break
		}
	}

	total_meta := 0
	tool.QueryRow_DB(db, "select count(*) from baro_file_meta", []any{&total_meta})

	body := baro_find_tabs("/baro/file")
	body += tool.Baro_crumb([][]string{
		{"/baro/file", tool.Baro_t("파일")},
		{"", tool.Baro_t("중복 파일")},
	})
	body += tool.Baro_hero(tool.Baro_t("중복 파일"), tool.Baro_t("파일 내용의 SHA224 해시로 같은 파일을 묶습니다."),
		tool.Baro_button("/baro/file", tool.Baro_t("전체 파일 목록"), "primary"))

	if message != "" {
		body += tool.Baro_alert("ok", tool.HTML_escape(message))
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("해시 기록된 파일"), tool.Baro_comma(total_meta - no_hash), ""},
		{tool.Baro_t("해시 없는 파일"), tool.Baro_comma(no_hash), tool.Baro_t("검사가 필요합니다")},
		{tool.Baro_t("중복 묶음"), tool.Baro_comma(len(hash_list)), ""},
		{tool.Baro_t("이름 겹침"), tool.Baro_comma(len(name_keys)), tool.Baro_t("확장자만 다른 같은 이름")},
	})

	scan_form := tool.Baro_checkbox("again", tool.Baro_t("이미 계산된 파일도 다시 계산"), "on", false, tool.Baro_t("파일이 바뀌었을 때만 켜세요."))
	scan_form += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("해시 검사 실행 (최대 300건)"), "primary") + `</div>`

	if tool.Baro_can(db, config.IP, "review") {
		body += tool.Baro_card(tool.Baro_t("해시 검사"), tool.Baro_form("/baro/file/dup", scan_form)+
			tool.Baro_note(tool.Baro_t("파일을 읽어 SHA224 를 계산해 baro_file_meta.hash 에 저장합니다.")))
	} else {
		body += tool.Baro_card(tool.Baro_t("해시 검사"), tool.Baro_alert("info",
			tool.Baro_t("해시 검사는 문서 검토 권한이 있는 기여자만 돌릴 수 있습니다. 결과는 누구나 볼 수 있습니다.")))
	}

	find_form := tool.Baro_input("q", tool.Baro_t("해시 또는 파일 이름"), keyword, tool.Baro_t("앞자리만 넣어도 됩니다"), "")
	find_form += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("찾기"), "primary") + `</div>`
	body += tool.Baro_card(tool.Baro_t("찾기"), `<form class="bw_form" method="get" action="/baro/file/dup">`+find_form+`</form>`)

	body += tool.Baro_section(tool.Baro_t("같은 내용의 파일"), dup_body)

	body += tool.Baro_card_ex(tool.Baro_t("동일 파일명 경고"), tool.Baro_t("이름이 같고 확장자만 다른 파일"),
		tool.Baro_table([]string{tool.Baro_t("이름"), tool.Baro_t("파일 문서"), tool.Baro_t("개수")}, name_rows,
			tool.Baro_t("이름이 겹치는 파일이 없습니다.")),
		tool.Baro_note(tool.Baro_t("같은 그림을 형식만 바꿔 여러 번 올린 경우가 많습니다. 하나만 남기고 나머지는 문서에서 부르지 않는 것이 좋습니다.")))

	body += tool.Baro_card(tool.Baro_t("기존 파일 재사용 안내"), tool.Baro_list([]string{
		tool.Baro_t(`같은 그림이 이미 있다면 <b>새로 올리지 말고</b> 있는 파일 이름을 문서에서 부르면 됩니다.`),
		tool.Baro_t(`해상도가 더 좋은 파일을 올렸다면, 낮은 쪽을 지우지 말고 문서에서 부르는 이름만 바꿔 주세요.`),
		tool.Baro_t(`정말 지워야 한다면 파일 문서를 격리로 보내 되돌릴 수 있게 합니다. (2번 원칙)`),
		tool.Baro_t(`중복 판단은 사람이 합니다. 해시는 같은 파일을 <b>찾아 주는</b> 도구일 뿐입니다.`),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("중복 파일"), body, baro_find_menu())
}

// View_baro_find_file_privacy : 4-4 개인정보 보호 처리 (/baro/file/privacy)
func View_baro_find_file_privacy(config tool.Config, page string, query url.Values, post url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if post != nil {
		if wall, need := tool.Baro_login_wall(db, config, "/baro/file/privacy"); need {
			return wall
		}
		if tool.Baro_read_only(db) {
			return tool.Baro_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db))
		}

		action := baro_find_val(post, "action")
		title := tool.Namespace_normalize(baro_find_val(post, "title"))
		message := ""

		switch action {
		case "exif":
			if !tool.Baro_can(db, config.IP, "review") {
				return tool.Baro_deny(db, config, tool.Baro_t("EXIF 제거는 문서 검토 권한이 있는 기여자만 할 수 있습니다."))
			}

			message = baro_find_exif_run(db, config, title)
			if message == "" {
				message = tool.Baro_t("EXIF 정보를 지웠습니다. 원본은 .exifbak 으로 백업했습니다.")
			}
		case "exif_all":
			if !tool.Baro_can(db, config.IP, "privacy") {
				return tool.Baro_deny(db, config, tool.Baro_t("일괄 처리는 개인정보 담당 역할만 할 수 있습니다."))
			}

			done := 0
			for _, name := range baro_find_file_title_list(db, 3000) {
				state := baro_find_file_state_get(db, name)
				if !state.Jpeg || state.Exif || !state.Exists {
					continue
				}

				baro_find_exif_run(db, config, name)
				done++

				if done >= 100 {
					break
				}
			}

			message = tool.Baro_itoa(done) + tool.Baro_t("건을 처리했습니다.")
		case "quarantine":
			if !tool.Baro_can(db, config.IP, "quarantine") {
				return tool.Baro_deny(db, config, tool.Baro_t("긴급 비공개는 격리 권한이 있는 운영자만 할 수 있습니다."))
			}

			message = baro_find_file_quarantine(db, config, title, baro_find_val(post, "reason"))
			if message == "" {
				message = tool.Baro_t("파일 문서를 격리했습니다. 삭제가 아니므로 언제든 되돌릴 수 있습니다.")
			}
		case "restrict":
			if !tool.Baro_can(db, config.IP, "privacy") {
				return tool.Baro_deny(db, config, tool.Baro_t("원본 접근 권한 설정은 개인정보 담당 역할만 바꿀 수 있습니다."))
			}

			before := tool.Baro_get_def(db, "find_file_original_role", "privacy")
			after := baro_find_val(post, "role")
			if after == "" {
				after = "privacy"
			}

			tool.Baro_put(db, "find_file_original_role", after)
			tool.Baro_audit(db, config, "file_original_role", "find_file_original_role", before, after, "원본 접근 권한 분리 설정")
			tool.Baro_public_log(db, "file_original_role", config.IP, "", "원본 접근 권한 분리 설정", after)
			message = tool.Baro_t("원본 접근 권한을 바꿨습니다.")
		}

		next := "/baro/file/privacy"
		if message != "" {
			next += "?msg=" + tool.Url_parser(message)
		}

		return tool.Get_redirect(next)
	}

	message := baro_find_val(query, "msg")

	page_int := baro_find_page_int(page)
	if raw_page := baro_find_val(query, "page"); raw_page != "" {
		page_int = baro_find_page_int(raw_page)
	}

	hidden := baro_find_hidden_set(db)
	kept := []baro_find_file_state{}
	done_count := 0

	for _, title := range baro_find_file_title_list(db, 3000) {
		state := baro_find_file_state_get(db, title)
		if !state.Jpeg {
			continue
		}
		if state.Exif {
			done_count++

			continue
		}
		if hidden[title] {
			continue
		}

		kept = append(kept, state)
	}

	start := (page_int - 1) * tool.Baro_page_size
	if start > len(kept) {
		start = len(kept)
	}
	end := start + tool.Baro_page_size
	if end > len(kept) {
		end = len(kept)
	}

	can_exif := tool.Baro_can(db, config.IP, "review")
	can_quarantine := tool.Baro_can(db, config.IP, "quarantine")

	rows := [][]string{}
	for _, state := range kept[start:end] {
		exif_mark := tool.Baro_badge(tool.Baro_t("확인 필요"), "muted")
		if state.Exists {
			if raw, err := os.ReadFile(state.Path); err == nil {
				if baro_find_exif_has(raw) {
					exif_mark = tool.Baro_badge(tool.Baro_t("EXIF 있음"), "danger")
				} else {
					exif_mark = tool.Baro_badge(tool.Baro_t("EXIF 없음"), "ok")
				}
			}
		} else {
			exif_mark = tool.Baro_badge(tool.Baro_t("파일 없음"), "warn")
		}

		manage := ""
		if can_exif && state.Exists {
			manage += tool.Baro_form("/baro/file/privacy",
				tool.Baro_hidden("action", "exif")+tool.Baro_hidden("title", state.Title)+
					tool.Baro_submit(tool.Baro_t("EXIF 제거"), "primary"))
		}
		manage += " " + tool.Baro_button("/baro/report/new?target="+tool.Url_parser(state.Title)+"&kind=privacy", "민감 정보 신고", "")
		if can_quarantine {
			manage += " " + tool.Baro_form("/baro/file/privacy",
				tool.Baro_hidden("action", "quarantine")+tool.Baro_hidden("title", state.Title)+
					tool.Baro_hidden("reason", tool.Baro_t("긴급 비공개 (개인정보 우려)"))+
					tool.Baro_submit(tool.Baro_t("긴급 비공개"), "danger"))
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(state.Title),
			exif_mark,
			tool.HTML_escape(baro_find_size_text(state.Size)),
			tool.HTML_escape(state.Meta["shot_date"]),
			manage,
		})
	}

	list_html := tool.Baro_table([]string{tool.Baro_t("파일 문서"), "EXIF", tool.Baro_t("크기"), tool.Baro_t("촬영일"), tool.Baro_t("조치")}, rows, "")
	if len(kept) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("처리할 JPEG 파일이 없습니다."))
	}
	list_html += tool.Baro_paging(page_int, end-start, tool.Baro_page_size, "/baro/file/privacy?page={}")

	role_now := tool.Baro_get_def(db, "find_file_original_role", "privacy")
	role_options := [][]string{{"privacy", tool.Baro_t("개인정보·법적 요청 담당만")}}
	for _, row := range tool.Baro_role_list() {
		if row[0] == "privacy" {
			continue
		}

		role_options = append(role_options, []string{row[0], row[1] + tool.Baro_t(" 까지")})
	}
	role_options = append(role_options, []string{"admin", tool.Baro_t("위키 관리자까지")})

	body := baro_find_tabs("/baro/file")
	body += tool.Baro_crumb([][]string{
		{"/baro/file", tool.Baro_t("파일")},
		{"", tool.Baro_t("개인정보 처리")},
	})
	body += tool.Baro_hero(tool.Baro_t("파일 개인정보 보호 처리"),
		tool.Baro_t("사진에 남는 위치정보와 사람이 알아볼 수 있는 정보를 다룹니다."),
		tool.Baro_button("/baro/file", tool.Baro_t("전체 파일 목록"), "primary")+" "+
			tool.Baro_button("/baro/report/new?kind=privacy", tool.Baro_t("민감 정보 신고하기"), ""))

	if message != "" {
		body += tool.Baro_alert("ok", tool.HTML_escape(message))
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("처리 대기 JPEG"), tool.Baro_comma(len(kept)), ""},
		{"처리 완료", tool.Baro_comma(done_count), "exif_removed=on"},
		{tool.Baro_t("원본 접근 권한"), tool.HTML_escape(tool.Baro_role_name(role_now)), tool.Baro_t("분리 설정")},
	})

	body += tool.Baro_alert("info",
		tool.Baro_t("사진 파일에는 <b>촬영 위치·기기·촬영자 이름</b>이 그대로 들어 있는 경우가 많습니다. ")+
			tool.Baro_t("바로위키는 JPEG 의 EXIF(APP1) 구간을 통째로 지우고, 원본은 따로 백업해 되돌릴 수 있게 합니다."))

	body += tool.Baro_card(tool.Baro_t("EXIF 제거 대기 목록"), list_html)

	if tool.Baro_can(db, config.IP, "privacy") {
		all_form := tool.Baro_hidden("action", "exif_all")
		all_form += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("대기 목록 일괄 처리 (최대 100건)"), "danger") + `</div>`

		body += tool.Baro_danger_zone(tool.Baro_t("일괄 처리"),
			tool.Baro_note(tool.Baro_t("한 번에 여러 파일을 고칩니다. 원본은 모두 .exifbak 으로 백업되지만, 되돌릴 때는 서버 파일을 직접 손봐야 합니다."))+
				tool.Baro_form("/baro/file/privacy", all_form))

		role_form := tool.Baro_hidden("action", "restrict")
		role_form += tool.Baro_select("role", tool.Baro_t("원본을 볼 수 있는 역할"), role_options, role_now,
			tool.Baro_t("원본 접근 제한이 켜진 파일의 고해상도·EXIF 포함본을 누가 볼 수 있는지 정합니다."))
		role_form += `<div class="bw_f">` + tool.Baro_submit("저장", "primary") + `</div>`

		body += tool.Baro_card("원본 접근 권한 분리", tool.Baro_form("/baro/file/privacy", role_form))
	} else {
		body += tool.Baro_card("원본 접근 권한 분리", tool.Baro_dl([][]string{
			{tool.Baro_t("지금 설정"), tool.HTML_escape(tool.Baro_role_name(role_now))},
			{tool.Baro_t("뜻"), tool.Baro_t("원본 접근 제한이 켜진 파일은 이 역할만 원본을 볼 수 있습니다. 일반 열람자는 처리본만 봅니다.")},
			{tool.Baro_t("바꾸는 방법"), tool.Baro_t("개인정보 담당 역할이 이 화면에서 바꿉니다.")},
		}))
	}

	body += tool.Baro_card(tool.Baro_t("얼굴·차량번호 등 민감 정보"), tool.Baro_list([]string{
		tool.Baro_t(`사진 속 <b>얼굴·차량번호·명패·문패</b>는 기계로 자동 판별하지 않습니다. 사람이 보고 신고합니다.`),
		tool.Baro_t(`신고가 들어오면 담당자가 확인해 가림 처리하거나 파일을 교체합니다.`),
		tool.Baro_t(`급할 때는 <b>긴급 비공개</b>로 먼저 감춘 뒤 판단합니다. 삭제가 아니라 격리이므로 되돌릴 수 있습니다.`),
		tool.Baro_button("/baro/report/new?kind=privacy", tool.Baro_t("민감 정보 신고 접수"), "primary"),
	}))

	body += tool.Baro_card(tool.Baro_t("EXIF 제거가 하는 일"), tool.Baro_dl([][]string{
		{tool.Baro_t("지우는 것"), tool.Baro_t("JPEG 의 APP1 구간 — EXIF(GPS 좌표·촬영 기기·촬영자)와 XMP")},
		{tool.Baro_t("남기는 것"), tool.Baro_t("그림 자료 자체와 다른 구간 (JFIF·색 프로필·허프만 표 등)")},
		{tool.Baro_t("원본"), tool.Baro_t("같은 자리에 <code>.exifbak</code> 으로 남깁니다. 덮어쓰지 않습니다.")},
		{tool.Baro_t("기록"), tool.Baro_t("처리하면 baro_file_meta.exif_removed 가 on 이 되고 공개 로그에 남습니다.")},
	}))

	return tool.Baro_page(db, config, tool.Baro_t("파일 개인정보 처리"), body, baro_find_menu())
}

// baro_find_file_quarantine : 파일 문서를 긴급 비공개(격리)로 보낸다
func baro_find_file_quarantine(db *sql.DB, config tool.Config, title string, reason string) string {
	if title == "" {
		return tool.Baro_t("대상 파일을 고르지 못했습니다.")
	}
	if !baro_find_doc_exists(db, title) {
		return tool.Baro_t("그런 파일 문서가 없습니다.")
	}
	if tool.Baro_doc_quarantined(db, title) {
		return tool.Baro_t("이미 격리 중인 파일입니다.")
	}
	if strings.TrimSpace(reason) == "" {
		return tool.Baro_t("격리 사유를 적어야 합니다.")
	}

	backup := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&backup}, title)

	rev := tool.Get_document_revision(db, title)

	tool.Exec_DB(
		db,
		"insert into baro_quarantine (id, title, rev, kind, reason, actor, date, status, backup, release_date, case_id) values (?, ?, ?, 'file_privacy', ?, ?, ?, 'open', ?, '', '')",
		tool.Baro_next_id(db, "baro_quarantine"), title, rev, tool.Baro_trunc(reason, 300),
		config.IP, tool.Baro_now(), tool.Baro_trunc(backup, 2000),
	)

	tool.Baro_doc_status_set(db, title, "quarantined", tool.Baro_trunc(reason, 300), config.IP)

	tool.Baro_public_log(db, "file_quarantine", config.IP, title, "긴급 비공개 (개인정보)", "")
	tool.Baro_audit(db, config, "file_quarantine", title, "공개", "격리", reason)

	// 올린 사람에게 알린다.
	meta := baro_find_file_meta(db, title)
	if meta["uploader"] != "" && tool.Baro_is_user(meta["uploader"]) {
		tool.Baro_notify(db, meta["uploader"], "case",
			tool.Baro_t("올리신 파일이 임시로 비공개되었습니다"),
			"/baro/file/meta/"+tool.Url_parser(title),
			tool.Namespace_display(title)+" — "+tool.Baro_trunc(reason, 100))
	}

	return ""
}
