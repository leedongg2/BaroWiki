package route

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"opennamu/route/tool"
)

// ============================================================
//  17-4 장애 대응 — /baro/status
//
//   - 상태 페이지 (누구나)
//   - 장애 공지 등록·해제 (system)
//   - 읽기 전용 모드
//   - 긴급 편집 중단
//   - 데이터 복구 절차 / 사고 보고서 / 재발 방지
// ============================================================

// baro_sec_sev_list : 심각도 (키, 이름, 배지 색)
func baro_sec_sev_list() [][]string {
	return [][]string{
		{"critical", tool.Baro_t("심각 (서비스 중단)"), "danger"},
		{"major", tool.Baro_t("큼 (주요 기능 장애)"), "danger"},
		{"minor", tool.Baro_t("작음 (일부 불편)"), "warn"},
		{"notice", tool.Baro_t("안내 (예정 점검)"), "info"},
	}
}

// baro_sec_sev_name : 심각도 이름
func baro_sec_sev_name(key string) string {
	for _, row := range baro_sec_sev_list() {
		if row[0] == key {
			return row[1]
		}
	}

	return key
}

// baro_sec_sev_kind : 심각도 배지 색
func baro_sec_sev_kind(key string) string {
	for _, row := range baro_sec_sev_list() {
		if row[0] == key {
			return row[2]
		}
	}

	return "muted"
}

// baro_sec_sev_options : 선택 상자용 목록
func baro_sec_sev_options() [][]string {
	out := [][]string{}
	for _, row := range baro_sec_sev_list() {
		out = append(out, []string{row[0], row[1]})
	}

	return out
}

// baro_sec_incident : 장애 기록 한 줄
type baro_sec_incident struct {
	ID      string
	Title   string
	Status  string
	Sev     string
	Detail  string
	Date    string
	Resolve string
	Post    string
	Actor   string
}

// baro_sec_incident_rows : 장애 목록
func baro_sec_incident_rows(db *sql.DB, status string, offset int, limit int) []baro_sec_incident {
	out := []baro_sec_incident{}

	query := "select id, title, status, severity, detail, date, resolve_date, postmortem, actor from baro_incident"
	args := []any{}

	if status != "" {
		query += " where status = ?"
		args = append(args, status)
	}

	query += " order by id + 0 desc limit ? offset ?"
	args = append(args, limit, offset)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	for rows.Next() {
		row := baro_sec_incident{}
		if err := rows.Scan(
			&row.ID, &row.Title, &row.Status, &row.Sev,
			&row.Detail, &row.Date, &row.Resolve, &row.Post, &row.Actor,
		); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_sec_incident_one : 장애 하나
func baro_sec_incident_one(db *sql.DB, id string) (baro_sec_incident, bool) {
	row := baro_sec_incident{}

	found := tool.QueryRow_DB(
		db,
		"select id, title, status, severity, detail, date, resolve_date, postmortem, actor from baro_incident where id = ?",
		[]any{
			&row.ID, &row.Title, &row.Status, &row.Sev,
			&row.Detail, &row.Date, &row.Resolve, &row.Post, &row.Actor,
		},
		id,
	)

	return row, found
}

// baro_sec_uptime : 가동 시간 문자열
func baro_sec_uptime() string {
	gap := time.Since(baro_sec_boot)

	days := int(gap.Hours()) / 24
	hours := int(gap.Hours()) % 24
	minutes := int(gap.Minutes()) % 60

	if days > 0 {
		return tool.Baro_itoa(days) + tool.Baro_t("일 ") + tool.Baro_itoa(hours) + tool.Baro_t("시간 ") + tool.Baro_itoa(minutes) + tool.Baro_t("분")
	}

	if hours > 0 {
		return tool.Baro_itoa(hours) + tool.Baro_t("시간 ") + tool.Baro_itoa(minutes) + tool.Baro_t("분")
	}

	return tool.Baro_itoa(minutes) + tool.Baro_t("분")
}

// baro_sec_db_file_size : SQLite 파일 크기 (알 수 없으면 -1)
func baro_sec_db_file_size() int64 {
	if tool.Get_DB_type() != "sqlite" {
		return -1
	}

	name := tool.Get_DB_set()["db_name"]
	if name == "" {
		name = "data"
	}

	info, err := os.Stat(filepath.Join(name + ".db"))
	if err != nil {
		return -1
	}

	return info.Size()
}

// baro_sec_size_text : 사람이 읽는 크기
func baro_sec_size_text(size int64) string {
	if size < 0 {
		return tool.Baro_t("알 수 없음")
	}

	if size < 1024 {
		return tool.Baro_comma(int(size)) + " B"
	}

	if size < 1024*1024 {
		return tool.Baro_comma(int(size/1024)) + " KB"
	}

	return tool.Baro_comma(int(size/1024/1024)) + " MB"
}

// baro_sec_table_count : 표의 줄 수 (표 이름은 코드 안에서만 정한다)
func baro_sec_table_count(db *sql.DB, table_name string) int {
	if !baro_sec_table_name_ok(table_name) {
		return 0
	}

	count := 0
	tool.QueryRow_DB(db, "select count(*) from "+table_name, []any{&count})

	return count
}

// baro_sec_table_name_ok : 표 이름이 안전한 글자로만 되어 있는지
func baro_sec_table_name_ok(table_name string) bool {
	if table_name == "" {
		return false
	}

	for _, ch := range table_name {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			continue
		}

		return false
	}

	return true
}

// baro_sec_status_tabs : 상태 화면 하위 탭
func baro_sec_status_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/status", "상태"},
		{"/baro/status/incident", tool.Baro_t("장애 공지")},
		{"/baro/status/readonly", tool.Baro_t("읽기 전용")},
		{"/baro/status/emergency", tool.Baro_t("긴급 편집 중단")},
		{"/baro/status/recovery", tool.Baro_t("복구 절차")},
	}, active)
}

// baro_sec_confirm_ok : 관리자 작업 추가 확인 (설정이 켜져 있으면 문구를 정확히 입력해야 한다)
func baro_sec_confirm_ok(db *sql.DB, values url.Values, phrase string) bool {
	if !baro_sec_flag(db, "abuse_confirm_on", true) {
		return true
	}

	return baro_sec_v(values, "confirm") == phrase
}

// baro_sec_confirm_field : 확인 문구 입력칸
func baro_sec_confirm_field(db *sql.DB, phrase string) string {
	if !baro_sec_flag(db, "abuse_confirm_on", true) {
		return tool.Baro_note(tool.Baro_t("관리자 작업 추가 확인이 꺼져 있어 확인 문구 없이 실행됩니다."))
	}

	return tool.Baro_input("confirm", tool.Baro_t("확인 문구"), "", phrase,
		tool.Baro_t("실수를 막기 위해 «")+phrase+tool.Baro_t("» 를 그대로 입력해야 실행됩니다."))
}

// ------------------------------------------------------------
//  /baro/status : 상태 페이지 (누구나)
// ------------------------------------------------------------

// View_baro_sec_status : 상태 페이지
func View_baro_sec_status(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if values != nil {
		return tool.Get_redirect("/baro/status")
	}

	open_rows := baro_sec_incident_rows(db, "open", 0, 10)
	recent := baro_sec_incident_rows(db, "", 0, 20)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", "상태"}})

	// 진행 중인 장애를 맨 위에 크게
	for _, row := range open_rows {
		body += tool.Baro_alert(baro_sec_sev_kind(row.Sev),
			`<b>[`+tool.HTML_escape(baro_sec_sev_name(row.Sev))+`] `+tool.HTML_escape(row.Title)+`</b><br>`+
				tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Detail), 200))+`<br>`+
				tool.Baro_t(`시작 : `)+tool.HTML_escape(row.Date)+` · `+
				`<a href="/baro/status/incident/`+tool.Url_parser(row.ID)+tool.Baro_t(`">자세히 보기</a>`),
		)
	}

	if len(open_rows) == 0 && !tool.Baro_read_only(db) {
		body += tool.Baro_alert("ok", tool.Baro_t("<b>모든 기능이 정상입니다.</b> 알려진 장애가 없습니다."))
	}

	body += tool.Baro_hero(tool.Baro_t("바로위키 상태"), tool.Baro_t("서버와 위키의 지금 상태를 누구나 볼 수 있게 공개합니다."), "")
	body += baro_sec_status_tabs("/baro/status")

	doc_count := baro_sec_table_count(db, "data")
	history_count := baro_sec_table_count(db, "history")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("서버 시각"), tool.Baro_now(), tool.Baro_t("위키 서버 기준")},
		{tool.Baro_t("가동 시간"), baro_sec_uptime(), tool.Baro_t("마지막 시작 이후")},
		{tool.Baro_t("문서 수"), tool.Baro_comma(doc_count), tool.Baro_t("data 표 기준")},
		{tool.Baro_t("편집 기록"), tool.Baro_comma(history_count), tool.Baro_t("history 표 기준")},
	})

	body += tool.Baro_card(tool.Baro_t("서버"), tool.Baro_dl([][]string{
		{tool.Baro_t("읽기 전용"), func() string {
			if tool.Baro_read_only(db) {
				return tool.Baro_badge(tool.Baro_t("켜짐 (편집 중단)"), "danger") + tool.Baro_t(" 사유 : ") + tool.HTML_escape(tool.Baro_read_only_reason(db))
			}

			return tool.Baro_badge(tool.Baro_t("꺼짐 (정상 편집 가능)"), "ok")
		}()},
		{tool.Baro_t("긴급 편집 중단"), func() string {
			if baro_sec_flag(db, "sec_emergency_stop", false) {
				return tool.Baro_badge(tool.Baro_t("실행 중"), "danger") + tool.Baro_t(" 사유 : ") +
					tool.HTML_escape(tool.Baro_get_def(db, "sec_emergency_reason", "사유 없음"))
			}

			return tool.Baro_badge("없음", "ok")
		}()},
		{tool.Baro_t("DB 종류"), tool.HTML_escape(strings.ToUpper(tool.Get_DB_type()))},
		{tool.Baro_t("DB 파일 크기"), tool.HTML_escape(baro_sec_size_text(baro_sec_db_file_size()))},
		{tool.Baro_t("바로위키 확장"), "v" + tool.Baro_ver},
		{tool.Baro_t("규모"), tool.HTML_escape(baro_sec_perf_summary(db))},
		{tool.Baro_t("진행 중인 장애"), tool.Baro_itoa(len(open_rows)) + tool.Baro_t("건")},
	}))

	// 최근 오류 · 장애
	table_rows := [][]string{}
	for _, row := range recent {
		state := tool.Baro_badge(tool.Baro_t("진행 중"), "danger")
		if row.Status != "open" {
			state = tool.Baro_badge(tool.Baro_t("해결됨"), "ok")
		}

		table_rows = append(table_rows, []string{
			tool.Baro_badge(baro_sec_sev_name(row.Sev), baro_sec_sev_kind(row.Sev)),
			`<a href="/baro/status/incident/` + tool.Url_parser(row.ID) + `">` + tool.HTML_escape(tool.Baro_trunc(row.Title, 50)) + `</a>`,
			tool.Baro_time_cell(row.Date),
			func() string {
				if row.Resolve == "" {
					return tool.Baro_note("-")
				}

				return tool.Baro_time_cell(row.Resolve)
			}(),
			state,
		})
	}

	if len(table_rows) == 0 {
		body += tool.Baro_card(tool.Baro_t("최근 장애"), tool.Baro_empty(tool.Baro_t("기록된 장애가 없습니다.")))
	} else {
		body += tool.Baro_card(tool.Baro_t("최근 장애"), tool.Baro_table(
			[]string{tool.Baro_t("심각도"), tool.Baro_t("제목"), tool.Baro_t("시작"), tool.Baro_t("해결"), "상태"}, table_rows, "",
		))
	}

	body += tool.Baro_card(tool.Baro_t("도움이 필요하면"),
		tool.Baro_list([]string{
			tool.Baro_t(`장애가 계속되면 <a href="/baro/status/incident">장애 공지 목록</a> 을 확인하세요.`),
			tool.Baro_t(`복구 절차는 <a href="/baro/status/recovery">데이터 복구 절차</a> 에 공개돼 있습니다.`),
			tool.Baro_t("읽기 전용 중에도 문서 읽기와 내려받기는 그대로 됩니다."),
		}),
	)

	return baro_sec_page(db, config, "상태", body)
}

// ------------------------------------------------------------
//  /baro/status/incident : 장애 공지 목록 · 등록
// ------------------------------------------------------------

// View_baro_sec_incident_list : 장애 공지 목록
func View_baro_sec_incident_list(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if values != nil {
		return baro_sec_incident_add(db, config, values)
	}

	page := baro_sec_page_num(query)
	status := baro_sec_q(query, "status")
	rows := baro_sec_incident_rows(db, status, (page-1)*tool.Baro_page_size, tool.Baro_page_size)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/status", "상태"}, {"", tool.Baro_t("장애 공지")}})
	body += tool.Baro_hero(tool.Baro_t("장애 공지"), tool.Baro_t("무슨 일이 있었고 어떻게 처리했는지 남깁니다."), "")
	body += baro_sec_status_tabs("/baro/status/incident")

	if baro_sec_q(query, "done") != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("장애 공지를 등록했습니다."))
	}
	if baro_sec_q(query, "fail") != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(baro_sec_q(query, "fail")))
	}

	body += tool.Baro_tabs([][]string{
		{"/baro/status/incident", tool.Baro_t("전체")},
		{"/baro/status/incident?status=open", tool.Baro_t("진행 중")},
		{"/baro/status/incident?status=closed", tool.Baro_t("해결됨")},
	}, func() string {
		if status == "" {
			return "/baro/status/incident"
		}

		return "/baro/status/incident?status=" + status
	}())

	table_rows := [][]string{}
	for _, row := range rows {
		state := tool.Baro_badge(tool.Baro_t("진행 중"), "danger")
		if row.Status != "open" {
			state = tool.Baro_badge(tool.Baro_t("해결됨"), "ok")
		}

		post := tool.Baro_badge("없음", "muted")
		if strings.TrimSpace(row.Post) != "" {
			post = tool.Baro_badge(tool.Baro_t("있음"), "ok")
		}

		table_rows = append(table_rows, []string{
			tool.HTML_escape(row.ID),
			tool.Baro_badge(baro_sec_sev_name(row.Sev), baro_sec_sev_kind(row.Sev)),
			`<a href="/baro/status/incident/` + tool.Url_parser(row.ID) + `">` + tool.HTML_escape(tool.Baro_trunc(row.Title, 50)) + `</a>`,
			tool.Baro_time_cell(row.Date),
			state,
			post,
		})
	}

	if len(table_rows) == 0 {
		body += tool.Baro_empty(tool.Baro_t("조건에 맞는 장애 기록이 없습니다."))
	} else {
		body += tool.Baro_table([]string{"번호", tool.Baro_t("심각도"), tool.Baro_t("제목"), tool.Baro_t("시작"), "상태", tool.Baro_t("사후 보고서")}, table_rows, "")
	}

	page_url := "/baro/status/incident?page={}"
	if status != "" {
		page_url += "&status=" + url.QueryEscape(status)
	}
	body += tool.Baro_paging(page, len(rows), tool.Baro_page_size, page_url)

	if tool.Baro_can(db, config.IP, "system") {
		form := tool.Baro_hidden("op", "add")
		form += tool.Baro_input("title", tool.Baro_t("제목"), "", tool.Baro_t("예 : 문서 저장이 되지 않는 문제"), tool.Baro_t("이용자가 겪는 현상을 그대로 씁니다."))
		form += tool.Baro_select("severity", tool.Baro_t("심각도"), baro_sec_sev_options(), "major", "")
		form += tool.Baro_textarea("detail", "상세", "", 6, tool.Baro_t("언제부터 / 무엇이 안 되는지 / 지금 무엇을 하고 있는지"), tool.Baro_t("개인정보나 공격 방법은 적지 않습니다."))
		form += baro_sec_reauth_field(db, config)
		form += tool.Baro_submit(tool.Baro_t("장애 공지 등록"), "danger")

		body += tool.Baro_card(tool.Baro_t("새 장애 공지"), tool.Baro_form("/baro/status/incident", form))
	} else {
		body += tool.Baro_note(tool.Baro_t("장애 공지 등록은 system 권한만 할 수 있습니다."))
	}

	return baro_sec_page(db, config, tool.Baro_t("장애 공지"), body)
}

// baro_sec_incident_add : 장애 공지 등록
func baro_sec_incident_add(db *sql.DB, config tool.Config, values url.Values) string {
	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("장애 공지는 system 권한만 등록할 수 있습니다."))
	}

	if baro_sec_v(values, "op") != "add" {
		return tool.Get_redirect("/baro/status/incident")
	}

	if ok, message := baro_sec_reauth_ok(db, config, values); !ok {
		return tool.Get_redirect("/baro/status/incident?fail=" + url.QueryEscape(message))
	}

	title := baro_sec_v(values, "title")
	if title == "" {
		return tool.Get_redirect("/baro/status/incident?fail=" + url.QueryEscape(tool.Baro_t("제목을 적어 주세요.")))
	}

	severity := baro_sec_v(values, "severity")
	if baro_sec_sev_name(severity) == severity {
		severity = "major"
	}

	id := tool.Baro_next_id(db, "baro_incident")

	tool.Exec_DB(
		db,
		"insert into baro_incident (id, title, status, severity, detail, date, resolve_date, postmortem, actor) values (?, ?, 'open', ?, ?, ?, '', '', ?)",
		id, tool.Baro_trunc(title, 200), severity, tool.Baro_trunc(baro_sec_v(values, "detail"), 4000), tool.Baro_now(), config.IP,
	)

	baro_sec_log_op(db, config, "incident_open", tool.Baro_t("장애 ")+id, "", baro_sec_sev_name(severity)+" / "+title, tool.Baro_t("장애 공지 등록"))

	// 운영 역할을 가진 사람에게 알린다
	for _, user := range baro_sec_role_users(db) {
		tool.Baro_notify(db, user, "system", "장애 공지 : "+title, "/baro/status/incident/"+id, baro_sec_sev_name(severity))
	}

	return tool.Get_redirect("/baro/status/incident?done=1")
}

// baro_sec_role_users : 운영 역할을 가진 사용자 목록 (알림 대상)
func baro_sec_role_users(db *sql.DB) []string {
	out := []string{}
	seen := map[string]bool{}

	rows := tool.Query_DB(db, "select user from baro_role limit 300")
	defer rows.Close()

	for rows.Next() {
		user := ""
		if err := rows.Scan(&user); err != nil {
			continue
		}

		if user == "" || seen[user] {
			continue
		}

		seen[user] = true
		out = append(out, user)
	}

	return out
}

// ------------------------------------------------------------
//  /baro/status/incident/<id> : 사고 보고서 상세
// ------------------------------------------------------------

// View_baro_sec_incident : 장애 상세 화면
func View_baro_sec_incident(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	id = strings.TrimSpace(id)

	if values != nil {
		return baro_sec_incident_post(db, config, id, values)
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/status", "상태"}, {"/baro/status/incident", tool.Baro_t("장애 공지")}, {"", "상세"}})
	body += baro_sec_status_tabs("/baro/status/incident")

	row, found := baro_sec_incident_one(db, id)
	if !found {
		body += tool.Baro_empty(id + tool.Baro_t("번 장애 기록이 없습니다. 번호를 확인하거나 목록에서 고르세요."))
		body += tool.Baro_button("/baro/status/incident", tool.Baro_t("장애 공지 목록"), "primary")

		return baro_sec_page(db, config, tool.Baro_t("장애 기록"), body)
	}

	post := tool.Baro_json_dec_str(row.Post)

	body += tool.Baro_hero(row.Title, baro_sec_sev_name(row.Sev), "")

	if row.Status == "open" {
		body += tool.Baro_alert(baro_sec_sev_kind(row.Sev), tool.Baro_t("<b>아직 해결되지 않은 장애입니다.</b>"))
	} else {
		body += tool.Baro_alert("ok", tool.Baro_t("해결된 장애입니다. 해결 시각 : ")+tool.HTML_escape(row.Resolve))
	}

	body += tool.Baro_card(tool.Baro_t("기본 정보"), tool.Baro_dl([][]string{
		{"번호", tool.HTML_escape(row.ID)},
		{tool.Baro_t("심각도"), tool.Baro_badge(baro_sec_sev_name(row.Sev), baro_sec_sev_kind(row.Sev))},
		{"상태", func() string {
			if row.Status == "open" {
				return tool.Baro_badge(tool.Baro_t("진행 중"), "danger")
			}

			return tool.Baro_badge(tool.Baro_t("해결됨"), "ok")
		}()},
		{tool.Baro_t("시작"), tool.HTML_escape(row.Date)},
		{tool.Baro_t("해결"), func() string {
			if row.Resolve == "" {
				return tool.Baro_note(tool.Baro_t("아직"))
			}

			return tool.HTML_escape(row.Resolve)
		}()},
		{tool.Baro_t("등록한 사람"), tool.Baro_user_link(db, row.Actor, config.IP)},
		{tool.Baro_t("지속 시간"), func() string {
			if row.Resolve == "" {
				return tool.HTML_escape(tool.Baro_time_ago(row.Date)) + tool.Baro_t(" 시작해 아직 진행 중")
			}

			return tool.Baro_itoa(int(baro_sec_hours_between(row.Date, row.Resolve))) + tool.Baro_t("시간")
		}()},
	}))

	body += tool.Baro_card(tool.Baro_t("무슨 일이 있었나"), tool.Baro_code(row.Detail))

	report_body := ""
	if post["report"] == "" && post["cause"] == "" && post["prevent"] == "" {
		report_body = tool.Baro_empty(tool.Baro_t("아직 사후 보고서가 없습니다."))
	} else {
		report_body = tool.Baro_section(tool.Baro_t("사고 경위"), tool.Baro_code(post["report"]))
		report_body += tool.Baro_section(tool.Baro_t("원인"), tool.Baro_code(post["cause"]))
		report_body += tool.Baro_section(tool.Baro_t("재발 방지"), tool.Baro_code(post["prevent"]))
		report_body += tool.Baro_note(tool.Baro_t("영향 범위 : ") + post["impact"])
	}

	body += tool.Baro_card(tool.Baro_t("사후 보고서"), report_body)

	if tool.Baro_can(db, config.IP, "system") {
		if row.Status == "open" {
			close_form := tool.Baro_hidden("op", "close")
			close_form += tool.Baro_textarea("report", tool.Baro_t("사고 경위"), post["report"], 5, tool.Baro_t("언제 무엇이 어떻게 됐는지 시간 순으로"), "")
			close_form += tool.Baro_textarea("cause", tool.Baro_t("원인"), post["cause"], 4, tool.Baro_t("왜 일어났는지"), tool.Baro_t("사람 탓이 아니라 구조를 적습니다."))
			close_form += tool.Baro_textarea("prevent", tool.Baro_t("재발 방지"), post["prevent"], 4, tool.Baro_t("무엇을 바꾸면 다시 안 일어나는지"), "")
			close_form += tool.Baro_input("impact", tool.Baro_t("영향 범위"), post["impact"], tool.Baro_t("예 : 3시간 동안 편집 불가"), "")
			close_form += baro_sec_reauth_field(db, config)
			close_form += tool.Baro_submit(tool.Baro_t("해결로 표시하고 보고서 저장"), "primary")

			body += tool.Baro_card("장애 해제", tool.Baro_form("/baro/status/incident/"+tool.Url_parser(id), close_form))
		} else {
			update_form := tool.Baro_hidden("op", "report")
			update_form += tool.Baro_textarea("report", tool.Baro_t("사고 경위"), post["report"], 5, "", "")
			update_form += tool.Baro_textarea("cause", tool.Baro_t("원인"), post["cause"], 4, "", "")
			update_form += tool.Baro_textarea("prevent", tool.Baro_t("재발 방지"), post["prevent"], 4, "", "")
			update_form += tool.Baro_input("impact", tool.Baro_t("영향 범위"), post["impact"], "", "")
			update_form += baro_sec_reauth_field(db, config)
			update_form += tool.Baro_submit(tool.Baro_t("보고서 갱신"), "")

			body += tool.Baro_card("사후 보고서 갱신", tool.Baro_form("/baro/status/incident/"+tool.Url_parser(id), update_form))

			reopen_form := tool.Baro_hidden("op", "reopen")
			reopen_form += baro_sec_reauth_field(db, config)
			reopen_form += tool.Baro_submit(tool.Baro_t("다시 진행 중으로"), "danger")

			body += tool.Baro_danger_zone(tool.Baro_t("장애 다시 열기"),
				tool.Baro_note(tool.Baro_t("같은 문제가 다시 일어났다면 새 공지를 만드는 편이 낫습니다."))+
					tool.Baro_form("/baro/status/incident/"+tool.Url_parser(id), reopen_form),
			)
		}
	}

	body += tool.Baro_button("/baro/status/incident", tool.Baro_t("장애 공지 목록"), "")

	return baro_sec_page(db, config, tool.Baro_t("장애 ")+row.ID, body)
}

// baro_sec_incident_post : 장애 상세 화면의 쓰기 처리
func baro_sec_incident_post(db *sql.DB, config tool.Config, id string, values url.Values) string {
	target := "/baro/status/incident/" + tool.Url_parser(id)

	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("장애 공지 관리는 system 권한만 할 수 있습니다."))
	}

	row, found := baro_sec_incident_one(db, id)
	if !found {
		return tool.Get_redirect("/baro/status/incident")
	}

	if ok, message := baro_sec_reauth_ok(db, config, values); !ok {
		return tool.Get_redirect("/baro/status/incident?fail=" + url.QueryEscape(message))
	}

	post := tool.Baro_json_enc(map[string]string{
		"report":  tool.Baro_trunc(baro_sec_v(values, "report"), 4000),
		"cause":   tool.Baro_trunc(baro_sec_v(values, "cause"), 2000),
		"prevent": tool.Baro_trunc(baro_sec_v(values, "prevent"), 2000),
		"impact":  tool.Baro_trunc(baro_sec_v(values, "impact"), 300),
	})

	switch baro_sec_v(values, "op") {
	case "close":
		now := tool.Baro_now()
		tool.Exec_DB(
			db,
			"update baro_incident set status = 'closed', resolve_date = ?, postmortem = ? where id = ?",
			now, post, id,
		)

		baro_sec_log_op(db, config, "incident_close", tool.Baro_t("장애 ")+id, "open", "closed", tool.Baro_t("장애 해결 처리"))

		for _, user := range baro_sec_role_users(db) {
			tool.Baro_notify(db, user, "system", "장애 해결 : "+row.Title, target, "사후 보고서가 등록됐습니다.")
		}
	case "report":
		tool.Exec_DB(db, "update baro_incident set postmortem = ? where id = ?", post, id)
		baro_sec_log_op(db, config, "incident_report", tool.Baro_t("장애 ")+id, tool.Baro_trunc(row.Post, 200), tool.Baro_trunc(post, 200), "사후 보고서 갱신")
	case "reopen":
		tool.Exec_DB(db, "update baro_incident set status = 'open', resolve_date = '' where id = ?", id)
		baro_sec_log_op(db, config, "incident_reopen", tool.Baro_t("장애 ")+id, "closed", "open", tool.Baro_t("장애 재개"))
	}

	return tool.Get_redirect(target)
}

// ------------------------------------------------------------
//  /baro/status/readonly : 읽기 전용 모드
// ------------------------------------------------------------

// View_baro_sec_readonly : 읽기 전용 모드 화면
func View_baro_sec_readonly(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if values != nil {
		return baro_sec_readonly_post(db, config, values)
	}

	on := tool.Baro_read_only(db)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/status", "상태"}, {"", tool.Baro_t("읽기 전용")}})
	body += tool.Baro_hero(tool.Baro_t("읽기 전용 모드"), tool.Baro_t("장애나 점검 때 편집만 멈추고 읽기는 그대로 둡니다."), "")
	body += baro_sec_status_tabs("/baro/status/readonly")

	if baro_sec_q(query, "fail") != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(baro_sec_q(query, "fail")))
	}

	body += tool.Baro_card(tool.Baro_t("현재 상태"), tool.Baro_dl([][]string{
		{tool.Baro_t("읽기 전용"), func() string {
			if on {
				return tool.Baro_badge(tool.Baro_t("켜짐"), "danger")
			}

			return tool.Baro_badge(tool.Baro_t("꺼짐"), "ok")
		}()},
		{"사유", func() string {
			reason := strings.TrimSpace(tool.Baro_read_only_reason(db))
			if !on || reason == "" {
				return tool.Baro_note("-")
			}

			return tool.HTML_escape(reason)
		}()},
		{tool.Baro_t("설정 위치"), tool.Baro_t("baro_set 의 <code>read_only</code> · <code>read_only_reason</code>")},
	}))

	body += tool.Baro_alert("info",
		tool.Baro_t("읽기 전용은 <code>tool.Baro_read_only</code> 로 확인합니다. 바로위키 화면은 모두 이 값을 보고 맨 위에 안내를 띄웁니다. ")+
			tool.Baro_t("이 화면과 상태 페이지에서는 항상 크게 보입니다."),
	)

	if !tool.Baro_can(db, config.IP, "system") {
		body += tool.Baro_note(tool.Baro_t("읽기 전용 전환은 system 권한만 할 수 있습니다."))

		return baro_sec_page(db, config, tool.Baro_t("읽기 전용 모드"), body)
	}

	if on {
		form := tool.Baro_hidden("op", "off")
		form += baro_sec_confirm_field(db, tool.Baro_t("읽기 전용 해제"))
		form += baro_sec_reauth_field(db, config)
		form += tool.Baro_submit(tool.Baro_t("읽기 전용 끄기"), "primary")

		body += tool.Baro_card(tool.Baro_t("읽기 전용 끄기"), tool.Baro_form("/baro/status/readonly", form))
	} else {
		form := tool.Baro_hidden("op", "on")
		form += tool.Baro_input("reason", "사유", "", tool.Baro_t("예 : DB 점검 중입니다."), tool.Baro_t("상태 페이지와 모든 바로위키 화면에 그대로 보입니다."))
		form += baro_sec_confirm_field(db, tool.Baro_t("읽기 전용"))
		form += baro_sec_reauth_field(db, config)
		form += tool.Baro_submit(tool.Baro_t("읽기 전용 켜기"), "danger")

		body += tool.Baro_danger_zone(tool.Baro_t("읽기 전용 켜기"),
			tool.Baro_note(tool.Baro_t("켜면 편집을 멈춥니다. 이유를 분명히 적어 이용자가 기다릴 수 있게 하세요."))+
				tool.Baro_form("/baro/status/readonly", form),
		)
	}

	return baro_sec_page(db, config, tool.Baro_t("읽기 전용 모드"), body)
}

// baro_sec_readonly_post : 읽기 전용 전환 처리
func baro_sec_readonly_post(db *sql.DB, config tool.Config, values url.Values) string {
	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("읽기 전용 전환은 system 권한만 할 수 있습니다."))
	}

	op := baro_sec_v(values, "op")

	phrase := tool.Baro_t("읽기 전용")
	if op == "off" {
		phrase = tool.Baro_t("읽기 전용 해제")
	}

	if !baro_sec_confirm_ok(db, values, phrase) {
		return tool.Get_redirect("/baro/status/readonly?fail=" + url.QueryEscape(tool.Baro_t("확인 문구가 «")+phrase+tool.Baro_t("» 와 다릅니다.")))
	}

	if ok, message := baro_sec_reauth_ok(db, config, values); !ok {
		return tool.Get_redirect("/baro/status/readonly?fail=" + url.QueryEscape(message))
	}

	before := "read_only=" + tool.Baro_get(db, "read_only")

	switch op {
	case "on":
		reason := baro_sec_v(values, "reason")
		if reason == "" {
			reason = "점검 중입니다."
		}

		tool.Baro_put(db, "read_only", "on")
		tool.Baro_put(db, "read_only_reason", tool.Baro_trunc(reason, 300))

		baro_sec_log_op(db, config, "read_only_on", tool.Baro_t("위키 전체"), before, "read_only=on", reason)

		for _, user := range baro_sec_role_users(db) {
			tool.Baro_notify(db, user, "system", "위키가 읽기 전용으로 바뀌었습니다.", "/baro/status", reason)
		}
	case "off":
		tool.Baro_put(db, "read_only", "")
		tool.Baro_put(db, "read_only_reason", "")

		baro_sec_log_op(db, config, "read_only_off", tool.Baro_t("위키 전체"), before, "read_only=off", tool.Baro_t("읽기 전용 해제"))

		for _, user := range baro_sec_role_users(db) {
			tool.Baro_notify(db, user, "system", "읽기 전용이 해제됐습니다.", "/baro/status", "편집을 다시 할 수 있습니다.")
		}
	}

	return tool.Get_redirect("/baro/status/readonly")
}

// ------------------------------------------------------------
//  /baro/status/emergency : 긴급 편집 중단
// ------------------------------------------------------------

// View_baro_sec_emergency : 긴급 편집 중단 화면
func View_baro_sec_emergency(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if values != nil {
		return baro_sec_emergency_post(db, config, values)
	}

	on := baro_sec_flag(db, "sec_emergency_stop", false)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/status", "상태"}, {"", tool.Baro_t("긴급 편집 중단")}})
	body += tool.Baro_hero(tool.Baro_t("긴급 편집 중단"), tool.Baro_t("지금 당장 편집을 멈춰야 할 때 쓰는 하나의 단추입니다."), "")
	body += baro_sec_status_tabs("/baro/status/emergency")

	if baro_sec_q(query, "fail") != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(baro_sec_q(query, "fail")))
	}

	body += tool.Baro_alert("warn",
		tool.Baro_t("<b>이 단추는 세 가지를 한꺼번에 합니다.</b><br>")+
			tool.Baro_t("1. 위키를 읽기 전용으로 바꿉니다.<br>")+
			tool.Baro_t("2. 심각도 '심각' 장애 공지를 자동으로 만듭니다.<br>")+
			tool.Baro_t("3. 운영 역할을 가진 모두에게 알립니다.<br>")+
			tool.Baro_t("공개 로그와 내부 감사 로그에도 남습니다."),
	)

	body += tool.Baro_card(tool.Baro_t("현재 상태"), tool.Baro_dl([][]string{
		{tool.Baro_t("긴급 편집 중단"), func() string {
			if on {
				return tool.Baro_badge(tool.Baro_t("실행 중"), "danger")
			}

			return tool.Baro_badge("없음", "ok")
		}()},
		{"사유", tool.HTML_escape(tool.Baro_get_def(db, "sec_emergency_reason", "-"))},
		{"실행 시각", tool.HTML_escape(tool.Baro_get_def(db, "sec_emergency_date", "-"))},
		{tool.Baro_t("연결된 장애 번호"), func() string {
			id := tool.Baro_get(db, "sec_emergency_incident")
			if id == "" {
				return tool.Baro_note("-")
			}

			return `<a href="/baro/status/incident/` + tool.Url_parser(id) + `">` + tool.HTML_escape(id) + tool.Baro_t(`번</a>`)
		}()},
		{tool.Baro_t("읽기 전용"), func() string {
			if tool.Baro_read_only(db) {
				return tool.Baro_badge(tool.Baro_t("켜짐"), "danger")
			}

			return tool.Baro_badge(tool.Baro_t("꺼짐"), "ok")
		}()},
	}))

	if !tool.Baro_can(db, config.IP, "system") {
		body += tool.Baro_note(tool.Baro_t("긴급 편집 중단은 system 권한만 실행할 수 있습니다."))

		return baro_sec_page(db, config, tool.Baro_t("긴급 편집 중단"), body)
	}

	if on {
		form := tool.Baro_hidden("op", "release")
		form += tool.Baro_textarea("report", tool.Baro_t("무슨 일이 있었는지"), "", 4, tool.Baro_t("사후 보고서에 그대로 들어갑니다."), "")
		form += baro_sec_confirm_field(db, tool.Baro_t("편집 재개"))
		form += baro_sec_reauth_field(db, config)
		form += tool.Baro_submit(tool.Baro_t("편집 재개"), "primary")

		body += tool.Baro_card(tool.Baro_t("편집 재개"), tool.Baro_form("/baro/status/emergency", form))
	} else {
		form := tool.Baro_hidden("op", "stop")
		form += tool.Baro_input("reason", "사유", "", tool.Baro_t("예 : 대량 훼손이 진행 중입니다."), tool.Baro_t("이용자에게 그대로 보입니다."))
		form += baro_sec_confirm_field(db, tool.Baro_t("편집 중단"))
		form += baro_sec_reauth_field(db, config)
		form += tool.Baro_submit(tool.Baro_t("모든 편집 즉시 중단"), "danger")

		body += tool.Baro_danger_zone(tool.Baro_t("모든 편집 즉시 중단"),
			tool.Baro_note(tool.Baro_t("훼손이 번지고 있을 때만 쓰세요. 평상시 점검은 읽기 전용 모드로 충분합니다."))+
				tool.Baro_form("/baro/status/emergency", form),
		)
	}

	body += tool.Baro_card(tool.Baro_t("중단 뒤에 할 일"),
		tool.Baro_list([]string{
			tool.Baro_t("무엇이 일어났는지 장애 공지에 계속 적습니다. 이용자는 그 화면을 봅니다."),
			tool.Baro_t("되돌릴 편집이 많으면 대량 작업 절차(대상 공개 · 2인 승인 · 스냅샷 · 유예)를 따릅니다."),
			tool.Baro_t(`복구가 필요하면 <a href="/baro/status/recovery">데이터 복구 절차</a> 를 따르세요.`),
			tool.Baro_t("재개할 때는 사후 보고서를 함께 남깁니다."),
		}),
	)

	return baro_sec_page(db, config, tool.Baro_t("긴급 편집 중단"), body)
}

// baro_sec_emergency_post : 긴급 편집 중단 처리
func baro_sec_emergency_post(db *sql.DB, config tool.Config, values url.Values) string {
	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, "긴급 편집 중단은 system 권한만 실행할 수 있습니다.")
	}

	op := baro_sec_v(values, "op")

	phrase := "편집 중단"
	if op == "release" {
		phrase = "편집 재개"
	}

	if !baro_sec_confirm_ok(db, values, phrase) {
		return tool.Get_redirect("/baro/status/emergency?fail=" + url.QueryEscape("확인 문구가 «"+phrase+"» 와 다릅니다."))
	}

	if ok, message := baro_sec_reauth_ok(db, config, values); !ok {
		return tool.Get_redirect("/baro/status/emergency?fail=" + url.QueryEscape(message))
	}

	switch op {
	case "stop":
		reason := baro_sec_v(values, "reason")
		if reason == "" {
			reason = "긴급 상황으로 편집을 중단합니다."
		}

		id := tool.Baro_next_id(db, "baro_incident")
		tool.Exec_DB(
			db,
			"insert into baro_incident (id, title, status, severity, detail, date, resolve_date, postmortem, actor) values (?, ?, 'open', 'critical', ?, ?, '', '', ?)",
			id, "긴급 편집 중단", tool.Baro_trunc(reason, 2000), tool.Baro_now(), config.IP,
		)

		baro_sec_flag_put(db, "sec_emergency_stop", true)
		tool.Baro_put(db, "sec_emergency_reason", tool.Baro_trunc(reason, 300))
		tool.Baro_put(db, "sec_emergency_date", tool.Baro_now())
		tool.Baro_put(db, "sec_emergency_incident", id)

		tool.Baro_put(db, "read_only", "on")
		tool.Baro_put(db, "read_only_reason", "긴급 편집 중단 : "+tool.Baro_trunc(reason, 250))

		baro_sec_log_op(db, config, "emergency_stop", "위키 전체", "정상", "긴급 편집 중단", reason)

		for _, user := range baro_sec_role_users(db) {
			tool.Baro_notify(db, user, "system", "긴급 편집 중단이 실행됐습니다.", "/baro/status", reason)
		}
	case "release":
		id := tool.Baro_get(db, "sec_emergency_incident")

		if id != "" {
			post := tool.Baro_json_enc(map[string]string{
				"report":  tool.Baro_trunc(baro_sec_v(values, "report"), 4000),
				"cause":   "",
				"prevent": "",
				"impact":  "긴급 편집 중단이 걸려 있던 동안 편집 불가",
			})

			tool.Exec_DB(
				db,
				"update baro_incident set status = 'closed', resolve_date = ?, postmortem = ? where id = ?",
				tool.Baro_now(), post, id,
			)
		}

		baro_sec_flag_put(db, "sec_emergency_stop", false)
		tool.Baro_put(db, "sec_emergency_reason", "")
		tool.Baro_put(db, "read_only", "")
		tool.Baro_put(db, "read_only_reason", "")

		baro_sec_log_op(db, config, "emergency_release", "위키 전체", "긴급 편집 중단", "정상", "편집 재개")

		for _, user := range baro_sec_role_users(db) {
			tool.Baro_notify(db, user, "system", "편집이 다시 열렸습니다.", "/baro/status", "긴급 편집 중단이 풀렸습니다.")
		}
	}

	return tool.Get_redirect("/baro/status/emergency")
}

// ------------------------------------------------------------
//  /baro/status/recovery : 데이터 복구 절차
// ------------------------------------------------------------

// View_baro_sec_recovery : 데이터 복구 절차 화면
func View_baro_sec_recovery(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if values != nil {
		return tool.Get_redirect("/baro/status/recovery")
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/status", "상태"}, {"", tool.Baro_t("복구 절차")}})
	body += tool.Baro_hero(tool.Baro_t("데이터 복구 절차"), tool.Baro_t("무엇이 남아 있고, 어떤 순서로 되살리는지 공개합니다."), "")
	body += baro_sec_status_tabs("/baro/status/recovery")

	backup_count := baro_sec_table_count(db, "baro_backup")
	snapshot_count := baro_sec_table_count(db, "baro_snapshot")
	quarantine_count := baro_sec_table_count(db, "baro_quarantine")
	history_count := baro_sec_table_count(db, "history")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("백업 기록"), tool.Baro_comma(backup_count), "baro_backup"},
		{tool.Baro_t("복구 스냅샷"), tool.Baro_comma(snapshot_count), tool.Baro_t("대량 작업 되돌리기용")},
		{tool.Baro_t("격리 보관"), tool.Baro_comma(quarantine_count), tool.Baro_t("삭제 대신 격리한 문서")},
		{tool.Baro_t("편집 기록"), tool.Baro_comma(history_count), tool.Baro_t("문서 단위 되돌리기")},
	})

	// 최근 백업
	backup_rows := [][]string{}
	rows := tool.Query_DB(db, "select id, kind, path, size, status, date from baro_backup order by id + 0 desc limit 10")
	defer rows.Close()

	for rows.Next() {
		id, kind, path, size, status, date := "", "", "", "", "", ""
		if err := rows.Scan(&id, &kind, &path, &size, &status, &date); err != nil {
			continue
		}

		state := tool.Baro_badge(status, "muted")
		if status == "done" {
			state = tool.Baro_badge(tool.Baro_t("완료"), "ok")
		}

		backup_rows = append(backup_rows, []string{
			tool.HTML_escape(id),
			tool.HTML_escape(kind),
			tool.HTML_escape(tool.Baro_trunc(path, 40)),
			tool.HTML_escape(size),
			state,
			tool.Baro_time_cell(date),
		})
	}

	backup_body := tool.Baro_table([]string{"번호", "종류", tool.Baro_t("경로"), tool.Baro_t("크기"), "상태", "시각"}, backup_rows, tool.Baro_t("기록된 백업이 없습니다."))
	if len(backup_rows) == 0 {
		backup_body += tool.Baro_alert("warn",
			tool.Baro_t("<b>백업 기록이 하나도 없습니다.</b> 지금 위키는 사고가 나면 되살릴 수단이 편집 기록뿐입니다. ")+
				tool.Baro_t("정기 백업을 먼저 마련하세요."))
	}

	body += tool.Baro_card(tool.Baro_t("지금 남아 있는 것"), backup_body)

	body += tool.Baro_card(tool.Baro_t("복구 순서"),
		tool.Baro_list([]string{
			tool.Baro_t("<b>1. 멈춥니다.</b> 훼손이 진행 중이면 먼저 긴급 편집 중단으로 더 번지는 것을 막습니다."),
			tool.Baro_t("<b>2. 범위를 셉니다.</b> 언제부터 어떤 문서·계정이 영향을 받았는지 편집 기록으로 확인합니다."),
			tool.Baro_t("<b>3. 되돌립니다.</b> 문서 단위는 편집 기록에서 되돌리고, 격리된 문서는 격리 해제로 되살립니다."),
			tool.Baro_t("<b>4. 대량이면 절차를 밟습니다.</b> 대상 목록 공개 · 2인 승인 · 스냅샷 · 유예를 거칩니다."),
			tool.Baro_t("<b>5. DB 를 되돌리는 것은 마지막입니다.</b> 백업 시점 이후의 정상 편집까지 사라지므로, 손실 범위를 먼저 공지합니다."),
			tool.Baro_t("<b>6. 기록합니다.</b> 장애 공지에 사후 보고서와 재발 방지를 남깁니다."),
		}),
	)

	body += tool.Baro_card(tool.Baro_t("복구할 때 지키는 원칙"),
		tool.Baro_list([]string{
			tool.Baro_t("삭제보다 보존입니다. 애매하면 지우지 말고 격리합니다."),
			tool.Baro_t("제재와 콘텐츠 판단은 분리합니다. 문제 계정을 막는 것과 그 사람의 정상 기여를 지우는 것은 다른 일입니다."),
			tool.Baro_t("복구 작업 자체도 감사 로그에 남습니다. 조용히 고치지 않습니다."),
			tool.Baro_t("이용자에게 무엇을 잃었는지 정확히 알립니다."),
		}),
	)

	if tool.Baro_can(db, config.IP, "system") {
		body += tool.Baro_card(tool.Baro_t("운영자용 확인 목록"),
			tool.Baro_list([]string{
				tool.Baro_t(`<a href="/baro/status/emergency">긴급 편집 중단</a> 이 필요한 상황인지 판단했습니까?`),
				tool.Baro_t(`<a href="/baro/status/incident">장애 공지</a> 를 등록했습니까?`),
				tool.Baro_t(`<a href="/baro/security/audit">감사 로그</a> 로 무슨 조작이 있었는지 확인했습니까?`),
				tool.Baro_t(`<a href="/baro/security/abuse/rate">차단 현황</a> 에서 폭주한 편집자를 확인했습니까?`),
			}),
		)
	}

	return baro_sec_page(db, config, tool.Baro_t("데이터 복구 절차"), body)
}
