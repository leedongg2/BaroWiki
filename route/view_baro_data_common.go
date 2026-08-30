package route

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	std_json "encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  14장 데이터 개방 · 백업 · 내보내기 · 공개 읽기 API
//
//  이 파일은 data 모듈 안에서만 쓰는 공통 도우미를 모아 둔다.
//  (모든 비공개 이름은 baro_data_ 로 시작한다)
// ============================================================

// baro_data_page_size : 목록 한 쪽에 보여 줄 줄 수
const baro_data_page_size = 30

// baro_data_dump_dir : 정기 덤프 파일이 쌓이는 곳
const baro_data_dump_dir = "data/dump"

// baro_data_backup_dir : 전체 백업 파일이 쌓이는 곳
const baro_data_backup_dir = "data/backup"

// baro_data_menu : 바로위키 표준 메뉴
func baro_data_menu() [][]any {
	return [][]any{{"baro", "바로위키"}}
}

// baro_data_tabs : 14장 화면 사이 이동 줄
func baro_data_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/export", tool.Baro_t("문서 내보내기")},
		{"/baro/export/dump", tool.Baro_t("정기 덤프")},
		{"/baro/backup", "백업"},
		{"/baro/backup/snapshot", tool.Baro_t("스냅샷")},
		{"/baro/api", tool.Baro_t("공개 API")},
	}, active)
}

// baro_data_page_num : 쪽 번호 문자열을 1 이상 정수로
func baro_data_page_num(page string) int {
	num := tool.Str_to_int(page)
	if num < 1 {
		num = 1
	}

	return num
}

// baro_data_q : 질의 문자열에서 값 하나 꺼내기 (nil 안전)
func baro_data_q(values url.Values, name string) string {
	if values == nil {
		return ""
	}

	return strings.TrimSpace(values.Get(name))
}

// baro_data_license : 위키 기본 라이선스
func baro_data_license(db *sql.DB) string {
	return tool.Baro_get_def(db, "license", "CC BY-SA 4.0")
}

// baro_data_wiki_name : 위키 이름
func baro_data_wiki_name(db *sql.DB) string {
	return tool.Baro_get_def(db, "wiki_name", "바로위키")
}

// baro_data_doc_text : 문서 원문 (없으면 두 번째 값이 false)
func baro_data_doc_text(db *sql.DB, title string) (string, bool) {
	data := ""
	found := tool.QueryRow_DB(db, "select data from data where title = ?", []any{&data}, title)

	return data, found
}

// baro_data_doc_readable : 이 문서를 밖으로 내보내도 되는가
//
//	격리된 문서와 읽기 권한이 없는 문서는 절대 내보내지 않는다.
func baro_data_doc_readable(db *sql.DB, config tool.Config, title string) bool {
	if title == "" {
		return false
	}

	if tool.Baro_doc_quarantined(db, title) {
		return false
	}

	// 상태가 격리로 표시된 문서도 밖으로 내보내지 않는다.
	if tool.Baro_doc_status_get(db, title) == "quarantined" {
		return false
	}

	return tool.Check_acl(db, title, "", "render", config.IP)
}

// baro_data_doc_categories : 문서가 속한 분류 목록 (표시용 이름)
//
//	back 표는 link 에 그 문서를, title 에 가리키는 대상(분류)을 넣는다.
//	그러므로 "이 문서가 속한 분류" 는 link 쪽에 문서를 맞춰야 한다.
func baro_data_doc_categories(db *sql.DB, title string) []string {
	out := []string{}

	rows := tool.Query_DB(db, "select title from back where link = ? and type = 'cat' order by title", title)
	defer rows.Close()

	for rows.Next() {
		category := ""
		if err := rows.Scan(&category); err != nil {
			continue
		}

		out = append(out, tool.Namespace_display(category))
	}

	return out
}

// baro_data_doc_links : 이 문서와 이어진 문서 목록 (표시용 이름 / 분류·부재 링크 제외)
//
//	back(역링크) 표에는 문서가 지워진 뒤에도 행이 남아 있을 수 있다.
//	(문서를 지우는 쪽에서 정리하는 것이 원칙이지만, 한 군데라도 빠지면
//	 지워진 문서 제목이 공개 API 로 새어 나간다)
//	그래서 조회 시점에도 data 표에 실제로 있는 제목만 남기도록 한 번 더 거른다.
func baro_data_doc_links(db *sql.DB, title string, limit int) []string {
	out := []string{}
	if limit < 1 {
		limit = 200
	}

	rows := tool.Query_DB(
		db,
		"select distinct link from back where title = ? and not type = 'cat' and not type = 'no' and not type = 'nothing' "+
			"and exists (select 1 from data where data.title = back.link) order by link limit ?",
		title, limit,
	)
	defer rows.Close()

	for rows.Next() {
		link := ""
		if err := rows.Scan(&link); err != nil {
			continue
		}

		out = append(out, tool.Namespace_display(link))
	}

	return out
}

// baro_data_doc_rev_count : 문서 판 수
func baro_data_doc_rev_count(db *sql.DB, title string) int {
	count := "0"
	tool.QueryRow_DB(db, "select count(*) from history where title = ?", []any{&count}, title)

	return tool.Str_to_int(count)
}

// baro_data_rev_row : 문서 역사 한 줄 (개인정보 없는 형태)
type baro_data_rev_row struct {
	Rev       string `json:"rev"`
	Date      string `json:"date"`
	User      string `json:"user"`
	Anonymous bool   `json:"anonymous"`
	Summary   string `json:"summary"`
	Length    int    `json:"length"`
	Kind      string `json:"kind"`
}

// baro_data_doc_history : 문서 역사 (IP 편집자는 이름을 남기지 않는다)
func baro_data_doc_history(db *sql.DB, title string, limit int) []baro_data_rev_row {
	out := []baro_data_rev_row{}
	if limit < 1 {
		limit = 200
	}

	rows := tool.Query_DB(
		db,
		"select id, date, ip, send, leng, hide, type from history where title = ? order by id + 0 desc limit ?",
		title, limit,
	)
	defer rows.Close()

	for rows.Next() {
		id, date, ip, send, leng, hide, kind := "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &date, &ip, &send, &leng, &hide, &kind); err != nil {
			continue
		}

		if hide == "on" {
			send = tool.Baro_t("(숨겨진 요약)")
		}

		row := baro_data_rev_row{
			Rev:     id,
			Date:    date,
			Summary: send,
			Length:  tool.Str_to_int(leng),
			Kind:    kind,
		}

		// 5번 원칙 : 공개 자료에 IP 를 담지 않는다.
		if tool.Baro_is_user(ip) {
			row.User = ip
		} else {
			row.Anonymous = true
		}

		out = append(out, row)
	}

	return out
}

// baro_data_user_private : 공개 노출을 거부한 사용자인가
func baro_data_user_private(db *sql.DB, user string) bool {
	if user == "" {
		return true
	}

	data := ""
	tool.QueryRow_DB(
		db,
		"select data from baro_user_set where user = ? and name = ?",
		[]any{&data}, user, "profile_private",
	)

	if data == "on" {
		return true
	}

	api := ""
	tool.QueryRow_DB(
		db,
		"select data from baro_user_set where user = ? and name = ?",
		[]any{&api}, user, "api_public",
	)

	return api == "off"
}

// baro_data_json_enc : HTML 이스케이프 없이 JSON 으로 (< > & 가 유니코드로 바뀌지 않게)
func baro_data_json_enc(value any, indent bool) (string, error) {
	buffer := &bytes.Buffer{}

	encoder := std_json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)

	if indent {
		encoder.SetIndent("", "  ")
	}

	if err := encoder.Encode(value); err != nil {
		return "", err
	}

	return strings.TrimRight(buffer.String(), "\n"), nil
}

// baro_data_json : 보기 좋은 JSON 문자열
func baro_data_json(value any) string {
	raw, err := baro_data_json_enc(value, true)
	if err != nil {
		return `{"ok":false,"error":"json_encode_failed"}`
	}

	return raw
}

// baro_data_json_line : JSON Lines 한 줄 (줄바꿈으로 끝난다)
func baro_data_json_line(value any) string {
	raw, err := baro_data_json_enc(value, false)
	if err != nil {
		return ""
	}

	return raw + "\n"
}

// baro_data_safe_name : 파일 이름으로 쓸 수 있게 다듬기
func baro_data_safe_name(data string) string {
	out := []rune{}
	for _, ch := range data {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9':
			out = append(out, ch)
		case ch == '-', ch == '_', ch == '.':
			out = append(out, ch)
		case ch > 127:
			out = append(out, ch)
		default:
			out = append(out, '_')
		}
	}

	name := strings.Trim(string(out), "._")
	if name == "" {
		name = "export"
	}

	if tool.Get_len(name) > 60 {
		name = tool.Get_slice(name, 0, 60)
	}

	return name
}

// baro_data_stamp : 파일 이름에 쓰는 시각 (20060102_150405)
func baro_data_stamp() string {
	now := tool.Baro_now()
	now = strings.ReplaceAll(now, "-", "")
	now = strings.ReplaceAll(now, ":", "")
	now = strings.ReplaceAll(now, " ", "_")

	return now
}

// baro_data_file_hash : 파일 SHA224 체크섬과 크기 (tool.Sha224 와 같은 알고리즘)
func baro_data_file_hash(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hasher := sha256.New224()

	size, err := io.Copy(hasher, file)
	if err != nil {
		return "", 0, err
	}

	return hex.EncodeToString(hasher.Sum(nil)), size, nil
}

// baro_data_size_text : 바이트 수를 사람이 읽는 크기로
func baro_data_size_text(size int64) string {
	if size < 0 {
		return "-"
	}

	if size < 1024 {
		return strconv.FormatInt(size, 10) + " B"
	}

	unit_list := []string{"KB", "MB", "GB", "TB"}
	value := float64(size) / 1024
	index := 0

	for value >= 1024 && index < len(unit_list)-1 {
		value /= 1024
		index++
	}

	return strconv.FormatFloat(value, 'f', 1, 64) + " " + unit_list[index]
}

// baro_data_note_get : baro_backup.note 에 넣어 둔 이름-값 (새 컬럼을 만들지 않기 위한 방법)
func baro_data_note_get(note string) map[string]string {
	out := tool.Baro_json_dec_str(note)
	if len(out) == 0 && strings.TrimSpace(note) != "" {
		out = map[string]string{"memo": note}
	}

	return out
}

// baro_data_note_put : 이름-값을 note 문자열로
func baro_data_note_put(data map[string]string) string {
	return tool.Baro_json_enc(data)
}

// baro_data_backup_row : 백업 한 줄
type baro_data_backup_row struct {
	ID       string
	Kind     string
	Path     string
	Size     string
	Status   string
	Date     string
	Note     map[string]string
	Checksum string
}

// baro_data_backup_get : 백업 한 건 읽기
func baro_data_backup_get(db *sql.DB, id string) (baro_data_backup_row, bool) {
	row := baro_data_backup_row{}
	note := ""

	found := tool.QueryRow_DB(
		db,
		"select id, kind, path, size, status, date, note, checksum from baro_backup where id = ?",
		[]any{&row.ID, &row.Kind, &row.Path, &row.Size, &row.Status, &row.Date, &note, &row.Checksum},
		id,
	)

	row.Note = baro_data_note_get(note)

	return row, found
}

// baro_data_backup_list : 백업 목록 (kind 가 비면 전체)
func baro_data_backup_list(db *sql.DB, kind string, offset int, limit int) []baro_data_backup_row {
	out := []baro_data_backup_row{}

	var rows *sql.Rows
	if kind == "" {
		rows = tool.Query_DB(
			db,
			"select id, kind, path, size, status, date, note, checksum from baro_backup order by id + 0 desc limit ?, ?",
			offset, limit,
		)
	} else {
		rows = tool.Query_DB(
			db,
			"select id, kind, path, size, status, date, note, checksum from baro_backup where kind = ? order by id + 0 desc limit ?, ?",
			kind, offset, limit,
		)
	}
	defer rows.Close()

	for rows.Next() {
		row := baro_data_backup_row{}
		note := ""

		if err := rows.Scan(&row.ID, &row.Kind, &row.Path, &row.Size, &row.Status, &row.Date, &note, &row.Checksum); err != nil {
			continue
		}

		row.Note = baro_data_note_get(note)
		out = append(out, row)
	}

	return out
}

// baro_data_kind_name : 백업/덤프 종류 이름
func baro_data_kind_name(kind string) string {
	switch kind {
	case "daily":
		return tool.Baro_t("일일 백업")
	case "weekly":
		return tool.Baro_t("주간 백업")
	case "manual":
		return tool.Baro_t("수동 백업")
	case "auto":
		return tool.Baro_t("자동 백업")
	case "dump":
		return "공개 덤프"
	case "replica":
		return tool.Baro_t("복제본")
	}

	return kind
}

// baro_data_export_kind_name : 내보내기 종류 이름
func baro_data_export_kind_name(kind string) string {
	switch kind {
	case "raw":
		return tool.Baro_t("문서 원문")
	case "history":
		return tool.Baro_t("문서 역사")
	case "pdf":
		return tool.Baro_t("인쇄용 (PDF 저장)")
	case "json":
		return tool.Baro_t("문서 JSON")
	case "project":
		return tool.Baro_t("프로젝트 전체")
	case "me":
		return "내 기여 기록"
	case "dump":
		return "공개 덤프"
	}

	return kind
}

// baro_data_export_add : 내보내기 이력 남기기 (14-1)
func baro_data_export_add(db *sql.DB, user string, kind string, scope string, path string) {
	who := user
	if !tool.Baro_is_user(who) {
		// 5번 원칙 : 이력에 IP 를 그대로 남기지 않는다.
		who = tool.Baro_mask_ip(who)
	}

	tool.Exec_DB(
		db,
		"insert into baro_export (id, user, kind, scope, status, path, date, expire) values (?, ?, ?, ?, 'done', ?, ?, ?)",
		tool.Baro_next_id(db, "baro_export"), who, kind, scope, path, tool.Baro_now(),
		tool.Baro_time_add(tool.Baro_now(), 30, 0, 0),
	)

	tool.Baro_metric_bump(db, "export_"+kind, tool.Baro_today(), 1)
}

// baro_data_ensure_dir : 폴더 만들기
func baro_data_ensure_dir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// baro_data_path : 저장 경로를 항상 / 로 통일한다
//
//	윈도에서 만든 백업 경로가 data\backup\... 로 기록되면 화면에서 읽기 나쁘고
//	다른 운영체제로 옮겼을 때 폴더 확인이 어긋난다.
func baro_data_path(parts ...string) string {
	return filepath.ToSlash(filepath.Join(parts...))
}

// baro_data_db_file : SQLite 데이터 파일 경로 (sqlite 가 아니면 "")
//
//	데이터 파일 이름은 설정(db_set)의 db_name 을 따른다. "data" 로 못 박으면
//	이름을 바꿔 쓰는 위키에서 엉뚱한 파일을 백업하게 된다.
func baro_data_db_file() string {
	if tool.Get_DB_type() != "sqlite" {
		return ""
	}

	name := strings.TrimSpace(tool.Get_DB_set()["db_name"])
	if name == "" {
		name = "data"
	}

	for _, path := range []string{name + ".db", baro_data_path("data", name+".db")} {
		if tool.File_exist_check(path) {
			return path
		}
	}

	return name + ".db"
}

// baro_data_error_page : 안내 화면 (500 대신 이것을 돌려준다)
func baro_data_error_page(db *sql.DB, config tool.Config, title string, message string) string {
	body := baro_data_tabs("")
	body += tool.Baro_alert("warn", tool.HTML_escape(message))
	body += tool.Baro_card(tool.Baro_t("다른 화면으로 가기"),
		tool.Baro_button("/baro/export", tool.Baro_t("문서 내보내기"), "")+" "+
			tool.Baro_button("/baro/export/dump", tool.Baro_t("정기 덤프"), "")+" "+
			tool.Baro_button("/baro/backup", "백업", "")+" "+
			tool.Baro_button("/baro/api", tool.Baro_t("공개 API 안내"), ""),
	)

	return tool.Baro_page(db, config, title, body, baro_data_menu())
}
