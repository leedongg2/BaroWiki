package route

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 [find] 모듈 공통 도구
//
//  5장 검색·탐색·SEO / 3-15 동명이인·별칭 / 4장 파일 을 담당한다.
//  이 파일의 비공개 식별자는 모두 baro_find_ 로 시작한다.
// ============================================================

// baro_find_scan_limit : 제목 전수 조사(오타 허용 / 초성 검색)에서 훑어볼 최대 문서 수
const baro_find_scan_limit = 20000

// baro_find_menu : 바로위키 화면 공통 메뉴
func baro_find_menu() [][]any {
	return [][]any{{"baro", "바로위키"}}
}

// baro_find_val : 폼/질의 값 한 개를 다듬어 읽기
func baro_find_val(values url.Values, key string) string {
	if values == nil {
		return ""
	}

	return strings.TrimSpace(values.Get(key))
}

// baro_find_page_int : 쪽 번호 정규화 (1 미만이면 1)
func baro_find_page_int(page string) int {
	num := tool.Str_to_int(page)
	if num < 1 {
		num = 1
	}

	return num
}

// baro_find_tabs : [find] 모듈 상단 탭
func baro_find_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/search", tool.Baro_t("검색")},
		{"/baro/discover", tool.Baro_t("발견")},
		{"/baro/discover/recent", tool.Baro_t("최근 변경")},
		{"/baro/doc/alias", tool.Baro_t("별칭·동명이인")},
		{"/baro/doc/seo", tool.Baro_t("외부 검색")},
		{"/baro/doc/graph", "지식 그래프"},
		{"/baro/file", tool.Baro_t("파일")},
	}, active)
}

// ------------------------------------------------------------
//  문자열 / 거리 계산
// ------------------------------------------------------------

// baro_find_lev : 편집 거리(Levenshtein) — 표준 라이브러리만으로 직접 구현
//
//	limit 을 넘어서면 limit + 1 을 돌려주고 조기 종료한다.
func baro_find_lev(a string, b string, limit int) int {
	first := []rune(a)
	second := []rune(b)

	if len(first) == 0 {
		if len(second) > limit {
			return limit + 1
		}

		return len(second)
	}
	if len(second) == 0 {
		if len(first) > limit {
			return limit + 1
		}

		return len(first)
	}

	// 길이 차이만으로 이미 한계를 넘으면 계산할 필요가 없다.
	gap := len(first) - len(second)
	if gap < 0 {
		gap = -gap
	}
	if gap > limit {
		return limit + 1
	}

	prev := make([]int, len(second)+1)
	curr := make([]int, len(second)+1)

	for j := 0; j <= len(second); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(first); i++ {
		curr[0] = i
		row_min := curr[0]

		for j := 1; j <= len(second); j++ {
			cost := 1
			if first[i-1] == second[j-1] {
				cost = 0
			}

			value := prev[j] + 1
			if curr[j-1]+1 < value {
				value = curr[j-1] + 1
			}
			if prev[j-1]+cost < value {
				value = prev[j-1] + cost
			}

			curr[j] = value
			if value < row_min {
				row_min = value
			}
		}

		if row_min > limit {
			return limit + 1
		}

		prev, curr = curr, prev
	}

	return prev[len(second)]
}

// baro_find_norm : 검색 비교용 정규화 (공백 제거 + 소문자)
func baro_find_norm(data string) string {
	return tool.Baro_norm(data)
}

// baro_find_snippet : 본문에서 검색어 주변을 잘라낸 발췌 (이미 escape 된 HTML 을 돌려준다)
func baro_find_snippet(data string, keyword string) string {
	data = tool.Baro_one_line(data)
	if data == "" {
		return tool.Baro_t(`<span class="bw_muted">본문이 비어 있습니다.</span>`)
	}

	lower_data := strings.ToLower(data)
	lower_key := strings.ToLower(strings.TrimSpace(keyword))

	index := -1
	if lower_key != "" {
		index = strings.Index(lower_data, lower_key)
	}

	if index < 0 {
		return tool.HTML_escape(tool.Baro_trunc(data, 140))
	}

	// 바이트 위치를 글자 위치로 바꾼다.
	start_rune := tool.Get_len(data[:index])
	from := start_rune - 40
	if from < 0 {
		from = 0
	}
	to := start_rune + tool.Get_len(keyword) + 80

	head := ""
	if from > 0 {
		head = "… "
	}

	cut := tool.Get_slice(data, from, to)
	tail := ""
	if tool.Get_len(data) > to {
		tail = " …"
	}

	// 검색어 부분을 강조한다. (escape 이후에 바꿔 넣으므로 안전)
	escaped := tool.HTML_escape(cut)
	key_escaped := tool.HTML_escape(tool.Get_slice(data, start_rune, start_rune+tool.Get_len(keyword)))
	if key_escaped != "" {
		escaped = strings.Replace(escaped, key_escaped, `<strong>`+key_escaped+`</strong>`, 1)
	}

	return tool.HTML_escape(head) + escaped + tool.HTML_escape(tail)
}

// ------------------------------------------------------------
//  문서 조회 도우미
// ------------------------------------------------------------

// baro_find_is_sandbox : 사용자 연습장 문서인지 (검색·사이트맵에서 제외)
func baro_find_is_sandbox(title string) bool {
	lower := strings.ToLower(title)
	if !strings.HasPrefix(lower, "user:") && !strings.HasPrefix(lower, "ipuser:") {
		return false
	}

	for _, mark := range []string{"/연습장", "/sandbox", "/draft", "/초안", "/작업실"} {
		if strings.Contains(lower, strings.ToLower(mark)) {
			return true
		}
	}

	return false
}

// baro_find_is_trash : 휴지통 계열 문서인지
func baro_find_is_trash(title string) bool {
	lower := strings.ToLower(title)

	return strings.HasPrefix(lower, "trash:") || strings.HasPrefix(lower, "filetrash:")
}

// baro_find_hidden_set : 검색에서 빼야 할 문서 집합 (격리 문서)
func baro_find_hidden_set(db *sql.DB) map[string]bool {
	out := map[string]bool{}

	rows := tool.Query_DB(db, "select title from baro_quarantine where status = 'open'")
	defer rows.Close()

	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		out[title] = true
	}

	rows2 := tool.Query_DB(db, "select title from baro_doc_status where status = 'quarantined'")
	defer rows2.Close()

	for rows2.Next() {
		title := ""
		if err := rows2.Scan(&title); err != nil {
			continue
		}

		out[title] = true
	}

	return out
}

// baro_find_excluded : 검색 결과에서 빼야 하는 문서인지
func baro_find_excluded(title string, hidden map[string]bool) bool {
	if title == "" {
		return true
	}
	if baro_find_is_trash(title) || baro_find_is_sandbox(title) {
		return true
	}

	return hidden[title]
}

// baro_find_scope_list : 검색 대상 구분
func baro_find_scope_list() [][]string {
	return [][]string{
		{"", tool.Baro_t("전체")},
		{"doc", tool.Baro_t("일반 문서")},
		{"file", tool.Baro_t("파일")},
		{"user", tool.Baro_t("사용자")},
		{"project", tool.Baro_t("프로젝트(바로위키)")},
		{"category", "분류"},
	}
}

// baro_find_scope_ok : 문서가 선택한 구분에 들어가는지
func baro_find_scope_ok(title string, scope string) bool {
	if scope == "" {
		return true
	}

	lower := strings.ToLower(title)
	switch scope {
	case "doc":
		return tool.Namespace_of(title).Prefix == ""
	case "file":
		return strings.HasPrefix(lower, "file:")
	case "user":
		return strings.HasPrefix(lower, "user:") || strings.HasPrefix(lower, "ipuser:")
	case "project":
		return strings.HasPrefix(lower, "baro:") || strings.HasPrefix(lower, "manage:")
	case "category":
		return strings.HasPrefix(lower, "category:")
	}

	return true
}

// baro_find_title_list : 제목 전수 목록 (오타 허용 / 초성 검색용)
func baro_find_title_list(db *sql.DB) []string {
	out := []string{}

	rows := tool.Query_DB(db, "select title from data limit ?", baro_find_scan_limit)
	defer rows.Close()

	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		out = append(out, title)
	}

	return out
}

// baro_find_doc_exists : 문서 존재 여부
func baro_find_doc_exists(db *sql.DB, title string) bool {
	if title == "" {
		return false
	}

	found := ""

	return tool.QueryRow_DB(db, "select title from data where title = ?", []any{&found}, title)
}

// baro_find_last_edit : 문서의 마지막 편집 시각
func baro_find_last_edit(db *sql.DB, title string) string {
	date := ""
	if tool.QueryRow_DB(
		db,
		"select set_data from data_set where doc_name = ? and set_name = 'last_edit' and doc_rev = ''",
		[]any{&date}, title,
	) && date != "" {
		return date
	}

	tool.QueryRow_DB(db, "select date from history where title = ? order by id + 0 desc limit 1", []any{&date}, title)

	return date
}

// baro_find_view_count : 문서 전체 조회수
func baro_find_view_count(db *sql.DB, title string) int {
	count := "0"
	tool.QueryRow_DB(
		db,
		"select set_data from data_set where doc_name = ? and set_name = 'view_count' and doc_rev = ''",
		[]any{&count}, title,
	)

	return tool.Str_to_int(count)
}

// ------------------------------------------------------------
//  별칭 / 문서 메타
// ------------------------------------------------------------

// baro_find_alias_kind_list : 별칭 종류 (3-15)
func baro_find_alias_kind_list() [][]string {
	return [][]string{
		{"hanja", "한자명"},
		{"former", tool.Baro_t("개명 전 이름")},
		{"ho", tool.Baro_t("호·아명·이명")},
		{"foreign", tool.Baro_t("외국어 표기")},
		{"wrong", tool.Baro_t("잘못 알려진 이름")},
		{"search", tool.Baro_t("검색 별칭")},
	}
}

// baro_find_alias_kind_name : 별칭 종류 이름
func baro_find_alias_kind_name(kind string) string {
	for _, row := range baro_find_alias_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// baro_find_alias_kind_ok : 알려진 별칭 종류인지
func baro_find_alias_kind_ok(kind string) bool {
	for _, row := range baro_find_alias_kind_list() {
		if row[0] == kind {
			return true
		}
	}

	return false
}

// baro_find_alias_of : 한 문서의 별칭 목록 ({id, alias, kind, note, date})
func baro_find_alias_of(db *sql.DB, title string) [][]string {
	out := [][]string{}
	if title == "" {
		return out
	}

	rows := tool.Query_DB(
		db,
		"select id, alias, kind, note, date from baro_doc_alias where title = ? order by id + 0 desc",
		title,
	)
	defer rows.Close()

	for rows.Next() {
		id, alias, kind, note, date := "", "", "", "", ""
		if err := rows.Scan(&id, &alias, &kind, &note, &date); err != nil {
			continue
		}

		out = append(out, []string{id, alias, kind, note, date})
	}

	return out
}

// baro_find_alias_text : 별칭을 한 줄로 (표에 넣을 HTML)
func baro_find_alias_text(list [][]string) string {
	if len(list) == 0 {
		return `<span class="bw_muted">-</span>`
	}

	parts := []string{}
	for index, row := range list {
		if index >= 6 {
			parts = append(parts, `<span class="bw_muted">…</span>`)
			break
		}

		parts = append(parts, tool.HTML_escape(row[1])+` <span class="bw_muted">(`+tool.HTML_escape(baro_find_alias_kind_name(row[2]))+`)</span>`)
	}

	return strings.Join(parts, ", ")
}

// baro_find_meta_name_list : [find] 모듈이 다루는 baro_doc_meta 항목
func baro_find_meta_name_list() [][]string {
	return [][]string{
		{"birth_year", tool.Baro_t("출생연도")},
		{"death_year", tool.Baro_t("사망연도")},
		{"job", tool.Baro_t("직업")},
		{"hanja", "한자명"},
		{"field", "분야"},
		{"description", tool.Baro_t("메타 설명 (외부 검색 결과에 보이는 문구)")},
		{"og_image", tool.Baro_t("공유 이미지 주소")},
		{"canonical", tool.Baro_t("대표 URL")},
		{"noindex", tool.Baro_t("검색 엔진 제외 (on)")},
		{"relation", tool.Baro_t("관계 JSON (지식 그래프)")},
	}
}

// baro_find_meta_all : 문서의 [find] 관련 메타를 한 번에 읽기
func baro_find_meta_all(db *sql.DB, title string) map[string]string {
	out := map[string]string{}
	if title == "" {
		return out
	}

	rows := tool.Query_DB(db, "select name, data from baro_doc_meta where title = ?", title)
	defer rows.Close()

	for rows.Next() {
		name, data := "", ""
		if err := rows.Scan(&name, &data); err != nil {
			continue
		}

		out[name] = data
	}

	return out
}

// baro_find_person_line : 출생연도·직업·한자명을 한 줄로
func baro_find_person_line(meta map[string]string) string {
	parts := []string{}
	if meta["birth_year"] != "" {
		line := meta["birth_year"] + tool.Baro_t("년생")
		if meta["death_year"] != "" {
			line = meta["birth_year"] + "~" + meta["death_year"]
		}
		parts = append(parts, line)
	}
	if meta["job"] != "" {
		parts = append(parts, meta["job"])
	}
	if meta["hanja"] != "" {
		parts = append(parts, meta["hanja"])
	}

	return strings.Join(parts, " · ")
}

// baro_find_meta_value_list : 특정 메타 이름에 실제로 쓰이고 있는 값 목록 (필터 상자용)
func baro_find_meta_value_list(db *sql.DB, name string) []string {
	seen := map[string]bool{}
	out := []string{}

	rows := tool.Query_DB(
		db,
		"select data from baro_doc_meta where name = ? and not data = '' limit 2000",
		name,
	)
	defer rows.Close()

	for rows.Next() {
		data := ""
		if err := rows.Scan(&data); err != nil {
			continue
		}

		data = strings.TrimSpace(data)
		if data == "" || seen[data] {
			continue
		}

		seen[data] = true
		out = append(out, data)
	}

	sort.Strings(out)

	return out
}

// baro_find_field_options : 분야 선택 상자 항목
func baro_find_field_options(db *sql.DB) [][]string {
	out := [][]string{{"", tool.Baro_t("분야 전체")}}
	for _, row := range tool.Baro_field_list(db) {
		out = append(out, []string{row.Key, row.Name})
	}

	return out
}

// baro_find_value_options : 값 목록을 선택 상자 항목으로
func baro_find_value_options(head string, list []string) [][]string {
	out := [][]string{{"", head}}
	for _, value := range list {
		out = append(out, []string{value, value})
	}

	return out
}

// baro_find_meta_title_list : 특정 메타 값을 가진 문서 목록
func baro_find_meta_title_list(db *sql.DB, name string, data string, limit int) []string {
	out := []string{}
	if limit <= 0 {
		limit = 200
	}

	rows := tool.Query_DB(
		db,
		"select title from baro_doc_meta where name = ? and data = ? limit ?",
		name, data, limit,
	)
	defer rows.Close()

	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		out = append(out, title)
	}

	return out
}

// ------------------------------------------------------------
//  파일
// ------------------------------------------------------------

// baro_find_file_split : "file:이름.확장자" 를 (이름, 확장자) 로
func baro_find_file_split(title string) (string, string) {
	if !strings.HasPrefix(strings.ToLower(title), "file:") {
		return "", ""
	}

	rest := title[len("file:"):]
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(rest)), ".")
	name := strings.TrimSuffix(rest, filepath.Ext(rest))

	return name, ext
}

// baro_find_file_path : 파일 문서에 대응하는 실제 파일 경로 ("" 면 알 수 없음)
func baro_find_file_path(db *sql.DB, title string) string {
	name, ext := baro_find_file_split(title)
	if name == "" || ext == "" {
		return ""
	}

	return filepath.Join(tool.Get_file_main_dir(db), tool.File_name_to_dir(name, ext))
}

// baro_find_file_meta : 파일 메타 한 줄
func baro_find_file_meta(db *sql.DB, title string) map[string]string {
	out := map[string]string{}

	source, author, photographer, license, basis := "", "", "", "", ""
	shot_date, description, related, hash, exif := "", "", "", "", ""
	date, uploader := "", ""

	found := tool.QueryRow_DB(
		db,
		"select source, author, photographer, license, basis, shot_date, description, related, hash, exif_removed, date, uploader from baro_file_meta where title = ?",
		[]any{&source, &author, &photographer, &license, &basis, &shot_date, &description, &related, &hash, &exif, &date, &uploader},
		title,
	)
	if !found {
		return out
	}

	out["source"] = source
	out["author"] = author
	out["photographer"] = photographer
	out["license"] = license
	out["basis"] = basis
	out["shot_date"] = shot_date
	out["description"] = description
	out["related"] = related
	out["hash"] = hash
	out["exif_removed"] = exif
	out["date"] = date
	out["uploader"] = uploader
	out["exist"] = "on"

	return out
}

// baro_find_file_meta_save : 파일 메타 저장 (지우고 다시 넣는다)
func baro_find_file_meta_save(db *sql.DB, title string, meta map[string]string) {
	tool.Exec_DB(db, "delete from baro_file_meta where title = ?", title)
	tool.Exec_DB(
		db,
		"insert into baro_file_meta (title, source, author, photographer, license, basis, shot_date, description, related, hash, exif_removed, date, uploader) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		title, meta["source"], meta["author"], meta["photographer"], meta["license"], meta["basis"],
		meta["shot_date"], meta["description"], meta["related"], meta["hash"], meta["exif_removed"],
		meta["date"], meta["uploader"],
	)
}

// baro_find_license_options : 파일 라이선스 선택 항목
func baro_find_license_options() [][]string {
	return [][]string{
		{"", tool.Baro_t("선택하세요")},
		{"public_domain", tool.Baro_t("퍼블릭 도메인 (저작권 만료·포기)")},
		{"kogl_1", tool.Baro_t("공공누리 제1유형")},
		{"kogl_2", tool.Baro_t("공공누리 제2유형 (상업적 이용 금지)")},
		{"kogl_3", tool.Baro_t("공공누리 제3유형 (변경 금지)")},
		{"cc_by", "CC BY"},
		{"cc_by_sa", "CC BY-SA"},
		{"cc_by_nc", "CC BY-NC"},
		{"cc0", "CC0"},
		{"self", tool.Baro_t("직접 제작 (기여자 본인)")},
		{"quotation", tool.Baro_t("인용 (저작권법 제28조)")},
		{"fair_use", tool.Baro_t("공정 이용 주장")},
		{"unknown", tool.Baro_t("저작권 불명")},
	}
}

// baro_find_license_name : 라이선스 표시 이름
func baro_find_license_name(key string) string {
	if key == "" {
		return tool.Baro_t("미기재")
	}

	for _, row := range baro_find_license_options() {
		if row[0] == key {
			return row[1]
		}
	}

	return key
}

// baro_find_file_title_list : 파일 문서 제목 목록
func baro_find_file_title_list(db *sql.DB, limit int) []string {
	out := []string{}
	if limit <= 0 {
		limit = 2000
	}

	rows := tool.Query_DB(db, "select title from data where title like 'file:%' order by title limit ?", limit)
	defer rows.Close()

	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		out = append(out, title)
	}

	return out
}

// baro_find_bot_check : 봇 편집인지 (사용자 설정 is_bot 또는 이름 규칙)
func baro_find_bot_check(db *sql.DB, user string) bool {
	if user == "" {
		return false
	}

	flag := ""
	if tool.QueryRow_DB(
		db,
		"select data from baro_user_set where user = ? and name = 'is_bot'",
		[]any{&flag}, user,
	) && flag == "on" {
		return true
	}

	lower := strings.ToLower(user)

	return strings.HasSuffix(lower, "bot") || strings.HasSuffix(user, "봇")
}
