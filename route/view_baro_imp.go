package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  13장 기존 위키 파일 · 자료 이전
//
//  13-1 문서 가져오기 / 13-2 라이선스·출처 기록 / 13-3 옛 주소 연결 / 13-4 대량 가져오기 봇
//
//  이 파일에는 공통 자료 구조와 작업 목록 · 새 가져오기 화면이 들어 있다.
// ============================================================

// ------------------------------------------------------------
//  공통 값
// ------------------------------------------------------------

// baro_imp_menu : 화면 오른쪽 메뉴
func baro_imp_menu() [][]any {
	return [][]any{
		{"baro", "바로위키"},
		{"baro/import", "가져오기"},
	}
}

// baro_imp_tabs : 13장 화면 사이 이동 줄
func baro_imp_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/import", tool.Baro_t("가져오기 작업")},
		{"/baro/import/new", tool.Baro_t("새 가져오기")},
		{"/baro/import/url", tool.Baro_t("옛 주소 연결")},
		{"/baro/import/bot", "대량 가져오기 봇"},
	}, active)
}

// baro_imp_status_list : 가져오기 작업 상태
func baro_imp_status_list() [][]string {
	return [][]string{
		{"ready", tool.Baro_t("검토 대기"), "info"},
		{"done", tool.Baro_t("가져오기 완료"), "ok"},
		{"partial", tool.Baro_t("일부만 가져옴"), "warn"},
		{"undone", tool.Baro_t("전체 취소됨"), "muted"},
		{"rejected", tool.Baro_t("중단됨"), "danger"},
	}
}

// baro_imp_status_name : 작업 상태 이름과 배지 종류
func baro_imp_status_name(status string) (string, string) {
	for _, row := range baro_imp_status_list() {
		if row[0] == status {
			return row[1], row[2]
		}
	}

	return status, "muted"
}

// baro_imp_item_status_name : 항목 상태 이름과 배지 종류
func baro_imp_item_status_name(status string) (string, string) {
	switch status {
	case "ready":
		return tool.Baro_t("가져올 수 있음"), "info"
	case "duplicate":
		return tool.Baro_t("중복"), "warn"
	case "error":
		return tool.Baro_t("오류"), "danger"
	case "imported":
		return tool.Baro_t("가져옴"), "ok"
	case "skipped":
		return tool.Baro_t("건너뜀"), "muted"
	case "undone":
		return tool.Baro_t("취소됨"), "muted"
	}

	return status, "muted"
}

// baro_imp_license_list : 고를 수 있는 라이선스
func baro_imp_license_list() [][]string {
	return [][]string{
		{"", tool.Baro_t("선택하세요")},
		{"CC BY-SA 4.0", "CC BY-SA 4.0"},
		{"CC BY-SA 3.0", "CC BY-SA 3.0"},
		{"CC BY 4.0", "CC BY 4.0"},
		{"CC0 1.0", tool.Baro_t("CC0 1.0 / 퍼블릭 도메인")},
		{"CC BY-NC-SA 2.0 KR", tool.Baro_t("CC BY-NC-SA 2.0 KR (비영리)")},
		{"CC BY-ND 4.0", tool.Baro_t("CC BY-ND 4.0 (변경 금지)")},
		{tool.Baro_t("저작자 본인 허락"), tool.Baro_t("저작자 본인에게 직접 허락받음")},
		{"직접 입력", tool.Baro_t("직접 입력 (아래 칸에 적기)")},
	}
}

// baro_imp_wiki_license : 바로위키 본문 라이선스
func baro_imp_wiki_license(db *sql.DB) string {
	return tool.Baro_get_def(db, "license", "CC BY-SA 4.0")
}

// baro_imp_license_compat : 본문 라이선스와 호환되는지 (호환 여부, 설명)
//
//	★ 13-2 : 대량 복제는 이 판단을 사람이 확인하기 전에는 실행할 수 없다.
func baro_imp_license_compat(license string) (bool, string) {
	key := strings.ToUpper(strings.TrimSpace(license))

	if key == "" {
		return false, tool.Baro_t("라이선스가 지정되지 않았습니다. 가져오기를 실행할 수 없습니다.")
	}

	if strings.Contains(key, "NC") || strings.Contains(key, "비영리") {
		return false, tool.Baro_t("비영리(NC) 조건이 붙은 자료입니다. 바로위키 본문 라이선스와 호환되지 않습니다.")
	}

	if strings.Contains(key, "ND") || strings.Contains(key, "변경 금지") || strings.Contains(key, "NODERIV") {
		return false, tool.Baro_t("변경 금지(ND) 조건이 붙은 자료입니다. 위키 본문으로 쓸 수 없습니다.")
	}

	switch {
	case strings.HasPrefix(key, "CC BY-SA"),
		strings.HasPrefix(key, "CC BY 4"),
		strings.HasPrefix(key, "CC BY-SA 3"),
		strings.HasPrefix(key, "CC0"),
		strings.Contains(key, "퍼블릭"),
		strings.Contains(key, "PUBLIC DOMAIN"):
		return true, tool.Baro_t("바로위키 본문 라이선스와 호환되는 것으로 보입니다. 그래도 사람이 직접 확인해야 합니다.")
	}

	return false, tool.Baro_t("자동으로 호환 여부를 판단할 수 없는 라이선스입니다. 직접 확인한 내용을 근거로 진행하세요.")
}

// ------------------------------------------------------------
//  자료 구조
// ------------------------------------------------------------

// baro_imp_job : 가져오기 작업 한 건
type baro_imp_job struct {
	ID      string
	Kind    string
	Source  string
	File    string
	Status  string
	Actor   string
	Date    string
	Report  string
	Num     string
	License string
	Options string
}

// baro_imp_item : 가져오기 항목 한 건
type baro_imp_item struct {
	ID       string
	ImportID string
	Title    string
	Status   string
	Note     string
	Author   string
	Url      string
	License  string
	Date     string
}

// baro_imp_job_get : 작업 한 건 읽기
func baro_imp_job_get(db *sql.DB, id string) (baro_imp_job, bool) {
	job := baro_imp_job{}
	if strings.TrimSpace(id) == "" {
		return job, false
	}

	found := tool.QueryRow_DB(
		db,
		"select id, kind, source, file, status, actor, date, report, num, license, options from baro_import where id = ?",
		[]any{&job.ID, &job.Kind, &job.Source, &job.File, &job.Status, &job.Actor, &job.Date, &job.Report, &job.Num, &job.License, &job.Options},
		id,
	)

	return job, found
}

// baro_imp_opt : 작업 선택값 하나 읽기
func baro_imp_opt(job baro_imp_job, name string) string {
	return tool.Baro_json_dec_str(job.Options)[name]
}

// baro_imp_opt_on : 작업 선택값이 켜져 있는지
func baro_imp_opt_on(job baro_imp_job, name string) bool {
	return baro_imp_opt(job, name) == "on"
}

// baro_imp_opt_set : 작업 선택값 여러 개 저장
func baro_imp_opt_set(db *sql.DB, job baro_imp_job, pairs map[string]string) {
	options := tool.Baro_json_dec_str(job.Options)
	if options == nil {
		options = map[string]string{}
	}

	for name, data := range pairs {
		options[name] = data
	}

	tool.Exec_DB(db, "update baro_import set options = ? where id = ?", tool.Baro_json_enc(options), job.ID)
}

// baro_imp_item_get : 항목 한 건 읽기
func baro_imp_item_get(db *sql.DB, id string) (baro_imp_item, bool) {
	item := baro_imp_item{}
	if strings.TrimSpace(id) == "" {
		return item, false
	}

	found := tool.QueryRow_DB(
		db,
		"select id, import_id, title, status, note, orig_author, orig_url, license, date from baro_import_item where id = ?",
		[]any{&item.ID, &item.ImportID, &item.Title, &item.Status, &item.Note, &item.Author, &item.Url, &item.License, &item.Date},
		id,
	)

	return item, found
}

// baro_imp_item_list : 작업에 속한 항목 목록 (status 가 비면 전부)
func baro_imp_item_list(db *sql.DB, import_id string, status string, offset int, limit int) []baro_imp_item {
	out := []baro_imp_item{}

	query := "select id, import_id, title, status, note, orig_author, orig_url, license, date from baro_import_item where import_id = ?"
	args := []any{import_id}

	if status != "" {
		query += " and status = ?"
		args = append(args, status)
	}

	query += " order by id + 0 asc limit ? offset ?"
	args = append(args, limit, offset)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	for rows.Next() {
		item := baro_imp_item{}
		if err := rows.Scan(&item.ID, &item.ImportID, &item.Title, &item.Status, &item.Note, &item.Author, &item.Url, &item.License, &item.Date); err != nil {
			continue
		}

		out = append(out, item)
	}

	return out
}

// baro_imp_item_count : 작업 항목 수 (status 가 비면 전부)
func baro_imp_item_count(db *sql.DB, import_id string, status string) int {
	count := 0

	if status == "" {
		tool.QueryRow_DB(db, "select count(*) from baro_import_item where import_id = ?", []any{&count}, import_id)

		return count
	}

	tool.QueryRow_DB(db, "select count(*) from baro_import_item where import_id = ? and status = ?", []any{&count}, import_id, status)

	return count
}

// baro_imp_body_key : 항목 본문을 담아 두는 스냅샷 열쇠
func baro_imp_body_key(item_id string) string {
	return "impitem:" + item_id
}

// baro_imp_undo_key : 되돌리기용 스냅샷 열쇠
func baro_imp_undo_key(item_id string) string {
	return "impundo:" + item_id
}

// baro_imp_body_set : 변환이 끝난 본문을 보관한다 (baro_import_item 에는 본문 칸이 없다)
func baro_imp_body_set(db *sql.DB, snapshot_id string, item_id string, title string, body string, meta string) {
	tool.Exec_DB(db, "delete from baro_snapshot where mass_id = ? and kind = 'import_src'", baro_imp_body_key(item_id))
	tool.Exec_DB(
		db,
		"insert into baro_snapshot (id, mass_id, kind, title, data, meta, date) values (?, ?, 'import_src', ?, ?, ?, ?)",
		snapshot_id, baro_imp_body_key(item_id), title, body, meta, tool.Baro_now(),
	)
}

// baro_imp_body_get : 보관해 둔 본문 읽기
func baro_imp_body_get(db *sql.DB, item_id string) string {
	data := ""
	tool.QueryRow_DB(
		db,
		"select data from baro_snapshot where mass_id = ? and kind = 'import_src'",
		[]any{&data}, baro_imp_body_key(item_id),
	)

	return data
}

// baro_imp_body_meta : 보관해 둔 본문의 부가 정보
func baro_imp_body_meta(db *sql.DB, item_id string) map[string]string {
	data := ""
	tool.QueryRow_DB(
		db,
		"select meta from baro_snapshot where mass_id = ? and kind = 'import_src'",
		[]any{&data}, baro_imp_body_key(item_id),
	)

	out := tool.Baro_json_dec_str(data)
	if out == nil {
		return map[string]string{}
	}

	return out
}

// baro_imp_doc_exist : 같은 제목의 문서가 이미 있는지
func baro_imp_doc_exist(db *sql.DB, title string) bool {
	found := ""
	tool.QueryRow_DB(db, "select title from data where title = ?", []any{&found}, title)

	return found != ""
}

// baro_imp_doc_text : 현재 문서 본문
func baro_imp_doc_text(db *sql.DB, title string) string {
	data := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&data}, title)

	return data
}

// ------------------------------------------------------------
//  13-1 가져오기 작업 목록 : /baro/import
// ------------------------------------------------------------

// View_baro_imp_list : 가져오기 작업 목록
func View_baro_imp_list(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/import"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "import") {
		return tool.Baro_deny(db, config, tool.Baro_t("가져오기 기능은 시스템 관리자 역할이 있어야 씁니다."))
	}

	page_int := tool.Str_to_int(page)
	if page_int < 1 {
		page_int = 1
	}

	max_count := tool.Baro_page_size
	offset := (page_int - 1) * max_count

	list := []baro_imp_job{}

	func() {
		rows := tool.Query_DB(
			db,
			"select id, kind, source, file, status, actor, date, report, num, license, options from baro_import order by id + 0 desc limit ? offset ?",
			max_count, offset,
		)
		defer rows.Close()

		for rows.Next() {
			job := baro_imp_job{}
			if err := rows.Scan(&job.ID, &job.Kind, &job.Source, &job.File, &job.Status, &job.Actor, &job.Date, &job.Report, &job.Num, &job.License, &job.Options); err != nil {
				continue
			}

			list = append(list, job)
		}
	}()

	table_rows := [][]string{}
	for _, job := range list {
		name, kind := baro_imp_status_name(job.Status)

		done := baro_imp_item_count(db, job.ID, "imported")
		total := tool.Str_to_int(job.Num)

		links := tool.Baro_button("/baro/import/view/"+tool.Url_parser(job.ID), "미리보기", "")
		links += " " + tool.Baro_button("/baro/import/license/"+tool.Url_parser(job.ID), "라이선스", "")
		links += " " + tool.Baro_button("/baro/import/report/"+tool.Url_parser(job.ID), "오류 보고서", "")

		table_rows = append(table_rows, []string{
			`<a href="/baro/import/view/` + tool.Url_parser(job.ID) + `">#` + tool.HTML_escape(job.ID) + `</a>`,
			tool.HTML_escape(baro_imp_kind_name(job.Kind)),
			tool.HTML_escape(tool.Baro_trunc(job.Source, 40)),
			tool.HTML_escape(job.License),
			tool.Baro_badge(name, kind),
			tool.Baro_comma(done) + " / " + tool.Baro_comma(total),
			tool.Baro_user_link(db, job.Actor, config.IP),
			tool.Baro_time_cell(job.Date),
			links,
		})
	}

	total_job := 0
	tool.QueryRow_DB(db, "select count(*) from baro_import", []any{&total_job})

	total_item := 0
	tool.QueryRow_DB(db, "select count(*) from baro_import_item", []any{&total_item})

	total_done := 0
	tool.QueryRow_DB(db, "select count(*) from baro_import_item where status = 'imported'", []any{&total_done})

	total_map := 0
	tool.QueryRow_DB(db, "select count(*) from baro_url_map", []any{&total_map})

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("기존 위키 자료 이전")}})
	body += tool.Baro_hero(
		tool.Baro_t("기존 위키 자료 이전"),
		tool.Baro_t("이미 만들어 둔 위키 파일을 그대로 이어서 씁니다."),
		tool.Baro_button("/baro/import/new", tool.Baro_t("새 가져오기 만들기"), "primary")+" "+
			tool.Baro_button("/baro/import/url", tool.Baro_t("옛 주소 연결"), "")+" "+
			tool.Baro_button("/baro/import/bot", "대량 가져오기 봇", ""),
	)
	body += baro_imp_tabs("/baro/import")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("가져오기 작업"), tool.Baro_comma(total_job), tool.Baro_t("지금까지 만든 작업")},
		{tool.Baro_t("등록된 문서"), tool.Baro_comma(total_item), tool.Baro_t("미리보기에 올라온 항목")},
		{tool.Baro_t("실제 가져온 문서"), tool.Baro_comma(total_done), tool.Baro_t("data 표에 들어간 문서")},
		{tool.Baro_t("옛 주소 매핑"), tool.Baro_comma(total_map), tool.Baro_t("예전 주소 → 새 주소")},
	})

	if tool.Baro_read_only(db) {
		body += tool.Baro_alert("warn", tool.Baro_t("지금은 읽기 전용 상태입니다. (")+tool.HTML_escape(tool.Baro_read_only_reason(db))+tool.Baro_t(") 가져오기 실행은 막힙니다."))
	}

	table := tool.Baro_table(
		[]string{"번호", tool.Baro_t("형식"), tool.Baro_t("원본"), "라이선스", "상태", tool.Baro_t("가져옴 / 전체"), tool.Baro_t("만든 사람"), tool.Baro_t("만든 때"), "이동"},
		table_rows,
		tool.Baro_t("아직 가져오기 작업이 없습니다."),
	)

	if len(table_rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("아직 가져오기 작업이 없습니다. 위의 \"새 가져오기 만들기\" 로 시작하세요."))
	}

	body += tool.Baro_card(tool.Baro_t("가져오기 작업"), table+tool.Baro_paging(page_int, len(list), max_count, "/baro/import/list/{}"))

	body += tool.Baro_card(tool.Baro_t("13장에서 하는 일"), tool.Baro_list([]string{
		tool.Baro_t(`<b>13-1 문서 가져오기</b> — 업로드한 파일에서 제목과 본문을 뽑아 변환하고, 미리보기로 확인한 뒤 고른 문서만 넣습니다.`),
		tool.Baro_t(`<b>13-2 라이선스 · 출처 기록</b> — 원 작성자 · 원본 주소 · 라이선스 · 가져온 날짜를 문서마다 남깁니다.`),
		tool.Baro_t(`<b>13-3 옛 주소 연결</b> — 예전 주소로 들어와도 새 문서로 넘어가게 합니다.`),
		tool.Baro_t(`<b>13-4 대량 가져오기 봇</b> — 봇 계정 · 속도 제한 · 사전 검토 · 전체 취소를 갖춘 대량 처리입니다.`),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("기존 위키 자료 이전"), body, baro_imp_menu())
}

// ------------------------------------------------------------
//  13-1 새 가져오기 : /baro/import/new
// ------------------------------------------------------------

// View_baro_imp_new : 기존 위키 파일 업로드 → 가져오기 작업 만들기
//
//	file_names / file_bodies 는 routes 계층에서 읽어 넘겨 준 업로드 파일이다.
func View_baro_imp_new(config tool.Config, values url.Values, file_names []string, file_bodies []string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/import/new"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "import") {
		return tool.Baro_deny(db, config, tool.Baro_t("가져오기 기능은 시스템 관리자 역할이 있어야 씁니다."))
	}

	notice := ""

	kind := "json"
	source := ""
	license := ""
	license_free := ""
	orig_author := ""
	delim := ""
	content := ""
	history_keep := true
	contributors := ""

	if values != nil {
		kind = strings.TrimSpace(values.Get("kind"))
		source = strings.TrimSpace(values.Get("source"))
		license = strings.TrimSpace(values.Get("license"))
		license_free = strings.TrimSpace(values.Get("license_free"))
		orig_author = strings.TrimSpace(values.Get("orig_author"))
		delim = values.Get("delim")
		content = values.Get("content")
		history_keep = values.Get("history_keep") == "on"
		contributors = strings.TrimSpace(values.Get("contributors"))

		if license == "직접 입력" || license == "" {
			if license_free != "" {
				license = license_free
			}
		}

		notice = baro_imp_create(db, config, kind, source, license, orig_author, delim, content, contributors, history_keep, file_names, file_bodies)
		if strings.HasPrefix(notice, "/baro/") {
			return tool.Get_redirect(notice)
		}
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/import", "자료 이전"}, {"", tool.Baro_t("새 가져오기")}})
	body += baro_imp_tabs("/baro/import/new")

	if notice != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(notice))
	}

	if tool.Baro_read_only(db) {
		body += tool.Baro_alert("warn", tool.Baro_t("지금은 읽기 전용 상태입니다. 작업은 만들 수 있지만 실행은 막힙니다."))
	}

	body += tool.Baro_alert("info",
		tool.Baro_t("업로드한 파일은 곧바로 위키에 들어가지 않습니다. 먼저 <b>미리보기 목록</b>이 만들어지고, ")+
			tool.Baro_t("거기서 고른 문서만 실제로 들어갑니다."))

	form := tool.Baro_select("kind", tool.Baro_t("원본 형식"), baro_imp_kind_list(), kind,
		tool.Baro_t("JSON · 구분자 텍스트 · 개별 텍스트 파일 세 가지를 지원합니다. 백업 JSON 은 형태가 달라도 관대하게 읽습니다."))

	form += tool.Baro_field_row_html(
		tool.Baro_t("파일 올리기"),
		`<input class="bw_in" type="file" name="files" multiple>`,
		tool.Baro_t("여러 개를 한 번에 고를 수 있습니다. 개별 텍스트 파일 형식이면 파일 이름이 문서 제목이 됩니다."),
	)

	form += tool.Baro_textarea("content", tool.Baro_t("또는 내용 직접 붙여넣기"), content, 8, tool.Baro_t("여기에 파일 내용을 그대로 붙여 넣어도 됩니다."),
		tool.Baro_t("파일을 올리지 않고 내용만 붙여 넣어도 똑같이 처리합니다."))

	form += tool.Baro_input("delim", tool.Baro_t("사용자 지정 구분자"), delim, tool.Baro_t("예: ----"),
		"구분자 텍스트 형식에서만 씁니다. 비워 두면 \"== 제목 ==\" 줄을 문서 경계로 봅니다.")

	form += tool.Baro_section(tool.Baro_t("13-2 라이선스 · 출처 기록 (필수)"),
		tool.Baro_note(tool.Baro_t("가져오는 문서마다 원 작성자 · 원본 출처 · 라이선스 · 가져온 날짜를 반드시 남깁니다."))+
			tool.Baro_input("source", tool.Baro_t("원본 출처"), source, tool.Baro_t("예: 친구위키 https://example.com/wiki"),
				tool.Baro_t("어디서 가져온 자료인지 적습니다. 문서마다 다른 주소는 파일 안의 url 값이 우선합니다."))+
			tool.Baro_select("license", tool.Baro_t("적용 라이선스"), baro_imp_license_list(), license,
				tool.Baro_t("라이선스를 고르지 않으면 작업을 만들 수 없습니다."))+
			tool.Baro_input("license_free", tool.Baro_t("라이선스 직접 입력"), license_free, tool.Baro_t("예: 저작자 표시 4.0 국제"),
				tool.Baro_t("위 목록에서 \"직접 입력\" 을 골랐을 때 씁니다."))+
			tool.Baro_input("orig_author", tool.Baro_t("원 작성자"), orig_author, tool.Baro_t("예: 친구 계정 이름"),
				tool.Baro_t("문서마다 다른 작성자는 파일 안의 author 값이 우선합니다."))+
			tool.Baro_textarea("contributors", tool.Baro_t("기존 기여자 목록"), contributors, 3, tool.Baro_t("한 줄에 한 명씩"),
				tool.Baro_t("원본 위키의 기여자 목록입니다. 문서 부가 정보로 함께 남깁니다."))+
			tool.Baro_checkbox("history_keep", tool.Baro_t("수정 이력을 보존한다 (원본 이력 요약을 문서 부가 정보로 남김)"), "on", history_keep, ""),
	)

	form += tool.Baro_submit(tool.Baro_t("미리보기 만들기"), "primary")

	body += tool.Baro_card(tool.Baro_t("새 가져오기"), tool.Baro_form_file("/baro/import/new", form))

	body += tool.Baro_card(tool.Baro_t("지원 형식 예시"),
		tool.Baro_section("1) JSON",
			tool.Baro_code(tool.Baro_t("[\n  {\"title\": \"문서 제목\", \"text\": \"본문\"},\n  {\"title\": \"다른 문서\", \"text\": \"본문\", \"author\": \"작성자\", \"url\": \"https://...\"}\n]\n\n또는\n\n{\n  \"문서 제목\": \"본문\",\n  \"다른 문서\": \"본문\"\n}")))+
			tool.Baro_section(tool.Baro_t("2) 여러 문서를 담은 텍스트"),
				tool.Baro_code("== 문서 제목 ==\n본문 내용\n\n== 다른 문서 ==\n본문 내용"))+
			tool.Baro_section(tool.Baro_t("3) 개별 텍스트 파일 여러 개"),
				tool.Baro_note(tool.Baro_t("문서_제목.txt 처럼 파일 이름이 곧 문서 제목이 됩니다. 여러 개를 한 번에 올리세요.")))+
			tool.Baro_section(tool.Baro_t("4) openNAMU · 나무위키식 백업 JSON"),
				tool.Baro_note(tool.Baro_t("data / namespace / title / text 같은 이름이 섞여 있어도 title 과 본문 짝을 찾아냅니다."))),
	)

	return tool.Baro_page(db, config, tool.Baro_t("새 가져오기"), body, baro_imp_menu())
}

// baro_imp_create : 업로드 내용을 읽어 가져오기 작업과 항목을 만든다
//
//	성공하면 옮겨 갈 주소를, 실패하면 사람이 읽을 오류 문장을 돌려준다.
func baro_imp_create(
	db *sql.DB,
	config tool.Config,
	kind string,
	source string,
	license string,
	orig_author string,
	delim string,
	content string,
	contributors string,
	history_keep bool,
	file_names []string,
	file_bodies []string,
) string {
	if kind == "" {
		return tool.Baro_t("원본 형식을 고르세요.")
	}

	if strings.TrimSpace(license) == "" || license == "직접 입력" {
		return tool.Baro_t("라이선스를 반드시 입력해야 합니다. (13-2 라이선스 · 출처 기록)")
	}

	// 속도 제한 : 같은 사람이 짧은 시간에 너무 많은 작업을 만들지 못하게 한다.
	if !tool.Baro_rate_ok(db, "baro_imp_new", config.IP, 10, 600) {
		return tool.Baro_t("가져오기 작업을 너무 자주 만들고 있습니다. 잠시 뒤에 다시 시도하세요.")
	}

	docs := []baro_imp_doc{}
	errors := []string{}
	file_label := []string{}

	for index, name := range file_names {
		if index >= len(file_bodies) {
			break
		}

		body := file_bodies[index]
		if strings.TrimSpace(body) == "" {
			errors = append(errors, name+tool.Baro_t(" : 내용이 비어 있습니다."))

			continue
		}

		file_label = append(file_label, name)

		one, fail := baro_imp_parse_all(kind, delim, name, body)
		if fail != "" {
			errors = append(errors, name+" : "+fail)

			continue
		}

		docs = append(docs, one...)
	}

	if strings.TrimSpace(content) != "" {
		one, fail := baro_imp_parse_all(kind, delim, tool.Baro_t("붙여넣은 내용"), content)
		if fail != "" {
			errors = append(errors, tool.Baro_t("붙여넣은 내용 : ")+fail)
		} else {
			docs = append(docs, one...)
			file_label = append(file_label, tool.Baro_t("붙여넣은 내용"))
		}
	}

	if len(docs) == 0 {
		if len(errors) > 0 {
			return strings.Join(errors, " / ")
		}

		return tool.Baro_t("가져올 문서를 찾지 못했습니다. 파일을 올리거나 내용을 붙여 넣으세요.")
	}

	if len(docs) > 5000 {
		docs = docs[:5000]
		errors = append(errors, tool.Baro_t("한 번에 5,000개까지만 받습니다. 나머지는 잘렸습니다."))
	}

	// 작업 만들기
	job_id := tool.Baro_next_id(db, "baro_import")

	// 13-4 봇 설정에서 정한 기본값을 새 작업에 그대로 물려준다.
	bot_default := ""
	if tool.Baro_get_on(db, "imp_bot_mark") {
		bot_default = "on"
	}

	bot_hide_default := ""
	if tool.Baro_get_on(db, "imp_bot_hide") {
		bot_hide_default = "on"
	}

	options := map[string]string{
		"delim":        delim,
		"orig_author":  orig_author,
		"contributors": contributors,
		"overwrite":    "",
		"bot":          bot_default,
		"bot_hide":     bot_hide_default,
	}

	if history_keep {
		options["history_keep"] = "on"
	}

	tool.Exec_DB(
		db,
		"insert into baro_import (id, kind, source, file, status, actor, date, report, num, license, options) values (?, ?, ?, ?, 'ready', ?, ?, '', '0', ?, ?)",
		job_id, kind, source, strings.Join(file_label, ", "), config.IP, tool.Baro_now(), license, tool.Baro_json_enc(options),
	)

	job, _ := baro_imp_job_get(db, job_id)

	// 항목 만들기 (제목 추출 → 본문 변환 → 중복 탐지 → 상태 기록)
	next_item := tool.Str_to_int(tool.Baro_next_id(db, "baro_import_item"))
	next_snap := tool.Str_to_int(tool.Baro_next_id(db, "baro_snapshot"))

	seen := map[string]bool{}
	count := map[string]int{"ready": 0, "duplicate": 0, "error": 0}
	total_conv := baro_imp_conv{}

	for _, doc := range docs {
		item_id := tool.Baro_itoa(next_item)
		next_item++

		title := baro_imp_norm_title(doc.Title)
		status := "ready"
		note := ""

		text, conv := baro_imp_convert(doc.Text)

		total_conv.Br += conv.Br
		total_conv.Category += conv.Category
		total_conv.File += conv.File
		total_conv.Template += conv.Template
		total_conv.Ext += conv.Ext
		total_conv.Link += conv.Link

		switch {
		case doc.Err != "":
			status = "error"
			note = doc.Err
		case title == "":
			status = "error"
			note = tool.Baro_t("문서 제목을 읽지 못했습니다.")
		case strings.TrimSpace(text) == "":
			status = "error"
			note = tool.Baro_t("본문이 비어 있습니다.")
		case seen[title]:
			status = "error"
			note = tool.Baro_t("같은 파일 안에 제목이 겹치는 문서가 있습니다.")
		case baro_imp_doc_exist(db, title):
			status = "duplicate"
			note = tool.Baro_t("이 위키에 같은 제목의 문서가 이미 있습니다.")
		}

		if title != "" {
			seen[title] = true
		}

		if status == "ready" || status == "duplicate" {
			extra := baro_imp_conv_note(conv)

			missing := 0
			for _, file_name := range baro_imp_file_links(text) {
				if !baro_imp_doc_exist(db, tool.Namespace_normalize("파일:"+file_name)) {
					missing++
				}
			}

			if missing > 0 {
				extra += tool.Baro_t(" · 아직 없는 파일 ") + tool.Baro_itoa(missing) + tool.Baro_t("개")
			}

			if note == "" {
				note = extra
			} else {
				note = note + " · " + extra
			}
		}

		count[status]++

		item_author := doc.Author
		if strings.TrimSpace(item_author) == "" {
			item_author = orig_author
		}

		item_url := doc.Url
		if strings.TrimSpace(item_url) == "" {
			item_url = source
		}

		item_license := doc.License
		if strings.TrimSpace(item_license) == "" {
			item_license = license
		}

		tool.Exec_DB(
			db,
			"insert into baro_import_item (id, import_id, title, status, note, orig_author, orig_url, license, date) values (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			item_id, job_id, title, status, tool.Baro_trunc(note, 300), item_author, tool.Baro_trunc(item_url, 500), item_license, tool.Baro_now(),
		)

		meta := map[string]string{
			"length":   tool.Baro_itoa(tool.Get_len(text)),
			"category": tool.Baro_itoa(conv.Category),
			"link":     tool.Baro_itoa(conv.Link),
			"template": tool.Baro_itoa(conv.Template),
			"file":     tool.Baro_itoa(conv.File),
			"ext":      tool.Baro_itoa(conv.Ext),
		}

		baro_imp_body_set(db, tool.Baro_itoa(next_snap), item_id, title, text, tool.Baro_json_enc(meta))
		next_snap++
	}

	report := map[string]string{
		"ready":     tool.Baro_itoa(count["ready"]),
		"duplicate": tool.Baro_itoa(count["duplicate"]),
		"error":     tool.Baro_itoa(count["error"]),
		"category":  tool.Baro_itoa(total_conv.Category),
		"link":      tool.Baro_itoa(total_conv.Link + total_conv.Ext),
		"template":  tool.Baro_itoa(total_conv.Template),
		"file":      tool.Baro_itoa(total_conv.File),
		"br":        tool.Baro_itoa(total_conv.Br),
		"file_note": strings.Join(errors, "\n"),
		"date":      tool.Baro_now(),
	}

	tool.Exec_DB(
		db,
		"update baro_import set num = ?, report = ? where id = ?",
		tool.Baro_itoa(len(docs)), tool.Baro_json_enc(report), job_id,
	)

	tool.Baro_public_log(db, "import_prepare", config.IP, "가져오기 #"+job_id,
		"라이선스 "+license, "문서 "+tool.Baro_itoa(len(docs))+"건 미리보기 생성")
	tool.Baro_audit(db, config, "import_prepare", "baro_import#"+job_id, "",
		"kind="+kind+" num="+tool.Baro_itoa(len(docs))+" license="+license, source)
	tool.Baro_metric_bump(db, "import_prepare", tool.Baro_today(), len(docs))

	_ = job

	return "/baro/import/view/" + tool.Url_parser(job_id)
}
