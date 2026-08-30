package route

import (
	"database/sql"
	"net/url"
	"regexp"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 [home] 모듈 공통 도구
//
//  1-1 창립 시즌 랜딩 · 바로위키 홈 · 소개 · 도움말 · 화면 지도가
//  함께 쓰는 조회·가공 함수를 모아 둔다.
//  이 파일의 비공개 식별자는 전부 baro_home_ 으로 시작한다.
// ============================================================

// baro_home_doc_filter : 일반(문서) 이름공간만 남기는 조건과 값 목록
//
//	틀·도움말·분류·파일·사용자·토론처럼 접두사가 붙은 이름공간은
//	"문서" 가 아니라 부속 자료이므로 문서 수와 미리보기에서 모두 뺀다.
//	이름공간 목록은 기반 계층(tool.Namespace_list)에서 읽으므로
//	이름공간이 늘어나도 이 화면이 따라간다.
func baro_home_doc_filter() (string, []any) {
	conds := []string{}
	args := []any{}

	for _, namespace := range tool.Namespace_list() {
		if namespace.Prefix == "" {
			continue
		}

		conds = append(conds, "title not like ?")
		args = append(args, namespace.Prefix+"%")
	}

	if len(conds) == 0 {
		return "1 = 1", args
	}

	return strings.Join(conds, " and "), args
}

// baro_home_menu : 바로위키 홈 계열 화면의 상단 메뉴
func baro_home_menu() [][]any {
	return [][]any{{"baro", "바로위키"}}
}

// baro_home_count : count(*) 한 개를 읽는다
func baro_home_count(db *sql.DB, query string, args ...any) int {
	count := 0
	tool.QueryRow_DB(db, query, []any{&count}, args...)

	return count
}

// baro_home_page_num : 조회 문자열에서 쪽 번호 읽기
func baro_home_page_num(query url.Values, name string) int {
	if query == nil {
		return 1
	}

	page := tool.Str_to_int(query.Get(name))
	if page < 1 {
		page = 1
	}
	if page > 5000 {
		page = 5000
	}

	return page
}

// baro_home_week_ago : 7일 전 시각
func baro_home_week_ago() string {
	return tool.Baro_time_add(tool.Baro_now(), -7, 0, 0)
}

// baro_home_wiki_name : 위키 이름
func baro_home_wiki_name(db *sql.DB) string {
	return tool.Baro_get_def(db, "wiki_name", "바로위키")
}

// ------------------------------------------------------------
//  본문 가공
// ------------------------------------------------------------

var baro_home_drop_re = regexp.MustCompile(`(?i)\[\[\s*(?:분류|category|파일|file)\s*:[^\]\[]*\]\]`)
var baro_home_link_re = regexp.MustCompile(`\[\[(?:[^\]\[|]*\|)?([^\]\[|]*)\]\]`)
var baro_home_macro_re = regexp.MustCompile(`\[[^\[\]\n]{0,120}\]`)
var baro_home_block_re = regexp.MustCompile(`(?s)\{\{\{(?:#!\S*)?|\}\}\}`)
var baro_home_head_re = regexp.MustCompile(`(?m)^\s*=+\s*|\s*=+\s*$`)
var baro_home_space_re = regexp.MustCompile(`[ \t]+`)
var baro_home_mail_re = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)
var baro_home_num_re = regexp.MustCompile(`^\s*(\d+)\.\s*(.*)$`)

// baro_home_plain : 위키 문법을 대충 걷어내고 읽을 수 있는 줄글로 만든다
//
//	미리보기 발췌용이므로 완벽할 필요는 없다. 결과는 그대로 escape 해서 쓴다.
func baro_home_plain(data string) string {
	data = strings.ReplaceAll(data, "\r", "")
	data = baro_home_block_re.ReplaceAllString(data, " ")
	// 분류·파일 링크는 읽을 글이 아니므로 발췌에서 통째로 뺀다
	data = baro_home_drop_re.ReplaceAllString(data, " ")
	data = baro_home_link_re.ReplaceAllString(data, "$1")
	data = baro_home_macro_re.ReplaceAllString(data, " ")
	data = baro_home_head_re.ReplaceAllString(data, " ")

	for _, mark := range []string{"'''", "''", "~~", "--", "__", "^^", ",,", "||", "* ", "#redirect", "#넘겨주기"} {
		data = strings.ReplaceAll(data, mark, " ")
	}

	data = tool.Baro_one_line(data)
	data = baro_home_space_re.ReplaceAllString(data, " ")

	return strings.TrimSpace(data)
}

// baro_home_text_html : 여러 줄 글을 문단 HTML 로 (escape 포함)
func baro_home_text_html(data string) string {
	data = strings.ReplaceAll(data, "\r", "")
	blocks := strings.Split(data, "\n\n")

	html := ""
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		lines := []string{}
		for _, line := range strings.Split(block, "\n") {
			lines = append(lines, tool.HTML_escape(strings.TrimSpace(line)))
		}

		html += `<p>` + strings.Join(lines, "<br>") + `</p>`
	}

	if html == "" {
		return tool.Baro_empty(tool.Baro_t("아직 내용이 없습니다."))
	}

	return html
}

// ------------------------------------------------------------
//  정책 (baro_policy)
// ------------------------------------------------------------

// baro_home_policy_row : 정책 한 판
type baro_home_policy_row struct {
	Title    string
	Version  string
	Data     string
	Date     string
	Proposer string
	Found    bool
}

// baro_home_policy_get : key_name 의 최신 판을 읽는다 (판 번호가 큰 것 우선)
func baro_home_policy_get(db *sql.DB, key_name string) baro_home_policy_row {
	row := baro_home_policy_row{}

	found := tool.QueryRow_DB(
		db,
		"select title, version, data, effective_date, proposer from baro_policy where key_name = ? and status = 'active' order by version + 0 desc limit 1",
		[]any{&row.Title, &row.Version, &row.Data, &row.Date, &row.Proposer},
		key_name,
	)

	if !found {
		found = tool.QueryRow_DB(
			db,
			"select title, version, data, effective_date, proposer from baro_policy where key_name = ? order by version + 0 desc limit 1",
			[]any{&row.Title, &row.Version, &row.Data, &row.Date, &row.Proposer},
			key_name,
		)
	}

	row.Found = found

	return row
}

// baro_home_policy_items : 정책 본문을 "1. ..." 단위로 쪼갠다 ({제목, 본문})
func baro_home_policy_items(data string) [][]string {
	out := [][]string{}
	data = strings.ReplaceAll(data, "\r", "")

	title, body := "", []string{}
	flush := func() {
		if title == "" {
			return
		}

		out = append(out, []string{title, strings.TrimSpace(strings.Join(body, "\n"))})
		title, body = "", []string{}
	}

	for _, line := range strings.Split(data, "\n") {
		if match := baro_home_num_re.FindStringSubmatch(line); match != nil && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			flush()
			title = match[1] + ". " + strings.TrimSpace(match[2])

			continue
		}

		if title != "" {
			body = append(body, strings.TrimSpace(line))
		}
	}

	flush()

	return out
}

// baro_home_policy_cards : 정책 항목을 카드 격자로
func baro_home_policy_cards(items [][]string, cols int) string {
	cards := []string{}
	for _, item := range items {
		body := ""
		if item[1] != "" {
			body = baro_home_text_html(item[1])
		} else {
			body = tool.Baro_note(tool.Baro_t("자세한 내용은 정책 전문을 확인하세요."))
		}

		cards = append(cards, tool.Baro_card(item[0], body))
	}

	if len(cards) == 0 {
		return tool.Baro_empty(tool.Baro_t("등록된 원칙 본문이 없습니다."))
	}

	return tool.Baro_grid(cards, cols)
}

// ------------------------------------------------------------
//  창립 시즌 집계 (baro_founder / baro_set)
// ------------------------------------------------------------

// baro_home_founding_goal : 창립 기여자 목표 인원
func baro_home_founding_goal(db *sql.DB) int {
	goal := tool.Baro_get_int(db, "founding_goal", 100)
	if goal <= 0 {
		goal = 100
	}

	return goal
}

// baro_home_founder_confirmed : 확정된 창립 기여자 수
func baro_home_founder_confirmed(db *sql.DB) int {
	return baro_home_count(db, "select count(*) from baro_founder where status = ?", "confirmed")
}

// baro_home_founder_pending : 심사 중인 지원자 수
func baro_home_founder_pending(db *sql.DB) int {
	return baro_home_count(
		db,
		"select count(*) from baro_founder where status = ? or status = ? or status = ?",
		"pending", "new", "open",
	)
}

// baro_home_founder_by_field : 분야별 확정 창립 기여자 수
func baro_home_founder_by_field(db *sql.DB) map[string]int {
	out := map[string]int{}

	rows := tool.Query_DB(db, "select field, count(*) from baro_founder where status = ? group by field", "confirmed")
	defer rows.Close()

	for rows.Next() {
		field, count := "", 0
		if err := rows.Scan(&field, &count); err != nil {
			continue
		}

		out[field] = count
	}

	return out
}

// baro_home_founder_recent : 최근 확정된 창립 기여자 (번호가 큰 순)
func baro_home_founder_recent(db *sql.DB, limit int) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(
		db,
		"select user, number, field, date from baro_founder where status = ? order by number + 0 desc limit ?",
		"confirmed", limit,
	)
	defer rows.Close()

	for rows.Next() {
		user, number, field, date := "", "", "", ""
		if err := rows.Scan(&user, &number, &field, &date); err != nil {
			continue
		}

		out = append(out, []string{user, number, field, date})
	}

	return out
}

// ------------------------------------------------------------
//  문서 조회 (data / history)
// ------------------------------------------------------------

// baro_home_doc_total : 전체 일반 문서 수
func baro_home_doc_total(db *sql.DB) int {
	where, args := baro_home_doc_filter()

	return baro_home_count(db, "select count(*) from data where "+where, args...)
}

// baro_home_doc_excerpt : 문서 앞부분 발췌
func baro_home_doc_excerpt(db *sql.DB, title string, max int) string {
	data := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&data}, title)

	text := baro_home_plain(tool.Get_slice(data, 0, 600))
	if text == "" {
		return ""
	}

	return tool.Baro_trunc(text, max)
}

// baro_home_recent_docs : 최근 생성된 문서 ({제목, 만든 시각, 발췌})
//
//	history 의 첫 판(id = 1)을 새 문서로 본다.
func baro_home_recent_docs(db *sql.DB, offset int, limit int, with_excerpt bool) [][]string {
	out := [][]string{}

	where, args := baro_home_doc_filter()

	// 이미 삭제된 문서는 SQL 단계에서 걸러야 쪽 넘기기가 어긋나지 않는다
	query := "select title, date from history where id = '1' and " + where +
		" and exists (select 1 from data where data.title = history.title)" +
		" order by date desc limit ?, ?"

	args = append(args, offset, limit)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	titles := [][]string{}
	for rows.Next() {
		title, date := "", ""
		if err := rows.Scan(&title, &date); err != nil {
			continue
		}

		titles = append(titles, []string{title, date})
	}

	for _, row := range titles {
		excerpt := ""
		if with_excerpt {
			excerpt = baro_home_doc_excerpt(db, row[0], 110)
		}

		out = append(out, []string{row[0], row[1], excerpt})
	}

	return out
}

// baro_home_new_doc_count : 특정 시각 이후에 생긴 문서 수
func baro_home_new_doc_count(db *sql.DB, since string) int {
	where, args := baro_home_doc_filter()
	args = append([]any{since}, args...)

	return baro_home_count(
		db,
		"select count(*) from history where id = '1' and date >= ? and "+where,
		args...,
	)
}

// baro_home_pick_doc : 날짜를 씨앗으로 문서 하나를 고른다 (하루 동안 같은 문서)
//
//	우수 문서 → 검토 완료 문서 → 전체 문서 순으로 후보를 찾는다.
func baro_home_pick_doc(db *sql.DB, seed string) (string, string) {
	candidates := []string{}

	for _, status := range []string{"featured", "reviewed"} {
		candidates = baro_home_status_docs(db, status)
		if len(candidates) > 0 {
			break
		}
	}

	if len(candidates) == 0 {
		total := baro_home_doc_total(db)
		if total <= 0 {
			return "", ""
		}

		where, args := baro_home_doc_filter()
		args = append(args, baro_home_seed(seed)%total)

		title := ""
		tool.QueryRow_DB(
			db,
			"select title from data where "+where+" order by title limit ?, 1",
			[]any{&title}, args...,
		)

		if title == "" {
			return "", ""
		}

		return title, baro_home_doc_excerpt(db, title, 220)
	}

	title := candidates[baro_home_seed(seed)%len(candidates)]

	return title, baro_home_doc_excerpt(db, title, 220)
}

// baro_home_status_docs : 그 상태로 표시된 문서 중 지금 실제로 볼 수 있는 것만
//
//	격리 중이거나 이미 사라진 문서는 "오늘의 문서" 로 내세우지 않는다.
func baro_home_status_docs(db *sql.DB, status string) []string {
	titles := []string{}

	rows := tool.Query_DB(
		db,
		"select title from baro_doc_status where status = ? order by date desc limit 60",
		status,
	)
	defer rows.Close()

	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		titles = append(titles, title)
	}

	out := []string{}
	for _, title := range titles {
		exist := ""
		if !tool.QueryRow_DB(db, "select title from data where title = ?", []any{&exist}, title) {
			continue
		}

		if tool.Baro_doc_quarantined(db, title) {
			continue
		}

		out = append(out, title)
	}

	return out
}

// baro_home_seed : 문자열을 0 이상 정수로 (하루 단위 고정 선택용)
func baro_home_seed(data string) int {
	value := 7
	for _, ch := range data {
		value = (value*131 + int(ch)) % 1000003
	}

	if value < 0 {
		value = -value
	}

	return value
}

// ------------------------------------------------------------
//  프로젝트 / 과제
// ------------------------------------------------------------

// baro_home_project_row : 프로젝트 요약
type baro_home_project_row struct {
	ID     string
	Name   string
	Field  string
	Goal   string
	Status string
	Done   int
	Target int
	Owner  string
	End    string
}

// baro_home_projects : 준비 중이거나 진행 중인 프로젝트
func baro_home_projects(db *sql.DB, limit int) []baro_home_project_row {
	out := []baro_home_project_row{}

	rows := tool.Query_DB(
		db,
		"select id, name, field, goal, status, done_count, target_count, owner, end_date from baro_project where status = ? or status = ? order by id + 0 desc limit ?",
		"open", "active", limit,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_home_project_row{}
		done, target := "", ""

		if err := rows.Scan(&row.ID, &row.Name, &row.Field, &row.Goal, &row.Status, &done, &target, &row.Owner, &row.End); err != nil {
			continue
		}

		row.Done = tool.Str_to_int(done)
		row.Target = tool.Str_to_int(target)
		out = append(out, row)
	}

	return out
}

// baro_home_project_card : 프로젝트 카드 HTML
func baro_home_project_card(db *sql.DB, row baro_home_project_row) string {
	body := ""
	if row.Goal != "" {
		body += `<p>` + tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Goal), 120)) + `</p>`
	}

	if row.Target > 0 {
		body += tool.Baro_progress(row.Done, row.Target, tool.Baro_t("목표 달성"))
	} else {
		body += tool.Baro_progress_open(row.Done, 0, "진행 기록")
	}

	state := tool.Baro_t("모집 중")
	kind := "info"
	if row.Status == "active" {
		state, kind = tool.Baro_t("진행 중"), "ok"
	}

	body += tool.Baro_kv_grid([][]string{
		{"분야", tool.HTML_escape(tool.Baro_field_name(db, row.Field))},
		{"상태", tool.Baro_badge(state, kind)},
	})

	body += tool.Baro_button("/baro/project/view/"+tool.Url_parser(row.ID), "프로젝트 보기", "")

	return tool.Baro_card(tool.Baro_trunc(row.Name, 40), body)
}

// baro_home_tasks_for_fields : 관심 분야에 맞는 열린 과제
//
//	fields 가 비면 분야를 가리지 않고 최근 과제를 돌려준다.
func baro_home_tasks_for_fields(db *sql.DB, fields []string, limit int) [][]string {
	out := [][]string{}

	query := "select id, title, kind, difficulty, field, project, est_time from baro_project_task where status = 'open'"
	args := []any{}

	clean := []string{}
	for _, field := range fields {
		if strings.TrimSpace(field) == "" {
			continue
		}

		clean = append(clean, field)
	}

	if len(clean) > 0 {
		marks := []string{}
		for _, field := range clean {
			marks = append(marks, "?")
			args = append(args, field)
		}

		query += " and field in (" + strings.Join(marks, ", ") + ")"
	}

	query += " order by id + 0 desc limit ?"
	args = append(args, limit)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	for rows.Next() {
		id, title, kind, difficulty, field, project, est := "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &title, &kind, &difficulty, &field, &project, &est); err != nil {
			continue
		}

		out = append(out, []string{id, title, kind, difficulty, field, project, est})
	}

	return out
}

// baro_home_difficulty_badge : 과제 난이도 배지
func baro_home_difficulty_badge(difficulty string) string {
	switch difficulty {
	case "easy", "1":
		return tool.Baro_badge(tool.Baro_t("쉬움"), "ok")
	case "normal", "2":
		return tool.Baro_badge(tool.Baro_t("보통"), "info")
	case "hard", "3":
		return tool.Baro_badge(tool.Baro_t("어려움"), "warn")
	case "":
		return tool.Baro_badge(tool.Baro_t("난이도 미정"), "muted")
	}

	return tool.Baro_badge(difficulty, "muted")
}

// ------------------------------------------------------------
//  관심 문서 (baro_watch)
// ------------------------------------------------------------

// baro_home_watch_changes : 내가 지켜보는 문서의 최근 변경 ({제목, 판, 시각, 편집자, 요약})
func baro_home_watch_changes(db *sql.DB, user string, limit int) [][]string {
	targets := baro_home_watch_targets(db, user)

	out := [][]string{}
	for _, title := range targets {
		id, date, ip, send := "", "", "", ""
		found := tool.QueryRow_DB(
			db,
			"select id, date, ip, send from history where title = ? order by id + 0 desc limit 1",
			[]any{&id, &date, &ip, &send}, title,
		)

		if !found {
			continue
		}

		out = append(out, []string{title, id, date, ip, send})
	}

	// 시각이 최근인 것부터
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j][2] > out[i][2] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}

	if len(out) > limit {
		out = out[:limit]
	}

	return out
}

// ------------------------------------------------------------
//  출시 알림 신청 (baro_set 이름-값 저장소)
// ------------------------------------------------------------

// baro_home_mail_prefix : 출시 알림 이메일 저장 키 접두사
const baro_home_mail_prefix = "founding_mail_"

// baro_home_mail_valid : 이메일 형식 확인
func baro_home_mail_valid(mail string) bool {
	if tool.Get_len(mail) > 190 {
		return false
	}

	return baro_home_mail_re.MatchString(mail)
}

// baro_home_mail_key : 이메일 저장 키 (원문 대신 해시로 중복을 판별한다)
func baro_home_mail_key(mail string) string {
	return baro_home_mail_prefix + tool.Sha224(strings.ToLower(strings.TrimSpace(mail)))
}

// baro_home_mail_count : 출시 알림 신청 수
func baro_home_mail_count(db *sql.DB) int {
	return baro_home_count(db, "select count(*) from baro_set where name like ?", baro_home_mail_prefix+"%")
}

// baro_home_mail_exists : 이미 신청한 이메일인지
func baro_home_mail_exists(db *sql.DB, mail string) bool {
	found := ""

	return tool.QueryRow_DB(db, "select name from baro_set where name = ?", []any{&found}, baro_home_mail_key(mail))
}

// baro_home_mail_mask : 이메일 부분 가림 (화면·기록 공개용)
func baro_home_mail_mask(mail string) string {
	at := strings.Index(mail, "@")
	if at <= 0 {
		return "***"
	}

	head := mail[:at]
	if tool.Get_len(head) <= 2 {
		head = tool.Get_slice(head, 0, 1) + "*"
	} else {
		head = tool.Get_slice(head, 0, 2) + strings.Repeat("*", 3)
	}

	return head + mail[at:]
}

// baro_home_mail_save : 출시 알림 이메일 저장
func baro_home_mail_save(db *sql.DB, mail string, who string) {
	tool.Baro_put(db, baro_home_mail_key(mail), tool.Baro_json_enc(map[string]string{
		"mail": strings.ToLower(strings.TrimSpace(mail)),
		"date": tool.Baro_now(),
		"who":  tool.Baro_mask_ip(who),
	}))
}

// ------------------------------------------------------------
//  안내 문구
// ------------------------------------------------------------

// baro_home_notice : 조회 문자열의 ok / err 값을 안내 상자로
func baro_home_notice(query url.Values) string {
	if query == nil {
		return ""
	}

	switch query.Get("ok") {
	case "mail":
		return tool.Baro_alert("ok", tool.Baro_t("출시 알림 신청이 접수됐습니다. 바로위키가 문을 열면 알려 드리겠습니다."))
	case "mail_dup":
		return tool.Baro_alert("info", tool.Baro_t("이미 신청된 이메일입니다. 알림은 한 번만 보내 드립니다."))
	}

	switch query.Get("err") {
	case "mail_form":
		return tool.Baro_alert("danger", tool.Baro_t("이메일 형식을 다시 확인해 주세요."))
	case "mail_rate":
		return tool.Baro_alert("warn", tool.Baro_t("요청이 너무 잦습니다. 잠시 뒤에 다시 시도해 주세요."))
	case "read_only":
		return tool.Baro_alert("warn", tool.Baro_t("지금은 읽기 전용 상태라 신청을 받을 수 없습니다."))
	case "unknown":
		return tool.Baro_alert("danger", tool.Baro_t("알 수 없는 요청입니다."))
	}

	return ""
}

// baro_home_watch_targets : 내가 지켜보는 문서 이름 목록
func baro_home_watch_targets(db *sql.DB, user string) []string {
	out := []string{}

	rows := tool.Query_DB(
		db,
		"select target from baro_watch where user = ? and (kind = 'doc' or kind = '' or kind = 'document') order by id + 0 desc limit 60",
		user,
	)
	defer rows.Close()

	for rows.Next() {
		target := ""
		if err := rows.Scan(&target); err != nil {
			continue
		}

		if strings.TrimSpace(target) == "" {
			continue
		}

		out = append(out, target)
	}

	return out
}
