package route

import (
	"database/sql"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

// ============================================================
//  바로위키 2장 : 계정 · 온보딩 · 첫 기여 도우미 · 프로필 · 연습장 · 알림
//
//  이 파일은 account 모듈의 공통 도우미만 모아 둔다.
//  화면 함수는 view_baro_account_*.go 에 나뉘어 있다.
// ============================================================

// baro_account_menu : 계정 영역 화면의 공통 메뉴
func baro_account_menu() [][]any {
	return [][]any{
		{"baro", "바로위키"},
		{"baro/account", tool.Baro_t("내 계정")},
		{"baro/draft", tool.Baro_t("내 초안")},
		{"baro/notify", tool.Baro_t("알림")},
	}
}

// baro_account_tabs : 계정 영역 탭 줄
func baro_account_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/account", tool.Baro_t("내 계정")},
		{"/baro/onboarding", tool.Baro_t("첫걸음")},
		{"/baro/draft", tool.Baro_t("내 초안")},
		{"/baro/draft/sandbox", tool.Baro_t("연습장")},
		{"/baro/notify", tool.Baro_t("알림")},
		{"/baro/profile/edit", tool.Baro_t("프로필 설정")},
		{"/baro/avatar", tool.Baro_t("프로필 사진")},
		{"/baro/mik", tool.Baro_t("밐 지갑")},
		{"/change/social", tool.Baro_t("연결된 계정")},
	}, active)
}

// baro_account_page_int : 쪽 번호 정규화
func baro_account_page_int(page string) int {
	num := tool.Str_to_int(page)
	if num < 1 {
		num = 1
	}

	return num
}

// ------------------------------------------------------------
//  바로위키 사용자 설정 (baro_user_set)
// ------------------------------------------------------------

// baro_account_uset_get : 바로위키 사용자 설정 읽기
func baro_account_uset_get(db *sql.DB, user string, name string) string {
	data := ""
	tool.QueryRow_DB(db, "select data from baro_user_set where user = ? and name = ?", []any{&data}, user, name)

	return data
}

// baro_account_uset_put : 바로위키 사용자 설정 저장 (한 줄만 유지)
func baro_account_uset_put(db *sql.DB, user string, name string, data string) {
	tool.Exec_DB(db, "delete from baro_user_set where user = ? and name = ?", user, name)
	tool.Exec_DB(db, "insert into baro_user_set (user, name, data) values (?, ?, ?)", user, name, data)
}

// ------------------------------------------------------------
//  정책 판 · 동의 기록 (2-1 / 15-4)
// ------------------------------------------------------------

// baro_account_policy_kind_list : 동의가 필요한 정책 종류
func baro_account_policy_kind_list() [][]string {
	return [][]string{
		{"license", tool.Baro_t("기여 라이선스")},
		{"privacy", tool.Baro_t("개인정보 처리 방침")},
	}
}

// baro_account_policy_name : 정책 종류 표시 이름
func baro_account_policy_name(kind string) string {
	for _, row := range baro_account_policy_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// baro_account_policy : 현재 시행 중인 정책 판 {version, title, data}
func baro_account_policy(db *sql.DB, key_name string) []string {
	version, title, data := "", "", ""
	tool.QueryRow_DB(
		db,
		"select version, title, data from baro_policy where key_name = ? and status = 'active' order by version + 0 desc limit 1",
		[]any{&version, &title, &data}, key_name,
	)

	if version == "" {
		tool.QueryRow_DB(
			db,
			"select version, title, data from baro_policy where key_name = ? order by version + 0 desc limit 1",
			[]any{&version, &title, &data}, key_name,
		)
	}

	return []string{version, title, data}
}

// baro_account_consent_version : 사용자가 마지막으로 동의한 판 번호
func baro_account_consent_version(db *sql.DB, user string, kind string) string {
	version := ""
	tool.QueryRow_DB(
		db,
		"select version from baro_consent where user = ? and kind = ? order by version + 0 desc limit 1",
		[]any{&version}, user, kind,
	)

	return version
}

// baro_account_consent_add : 동의 기록 남기기 (IP 는 가려서 저장한다 — 5번 원칙)
func baro_account_consent_add(db *sql.DB, config tool.Config, kind string, version string) {
	if strings.TrimSpace(version) == "" {
		version = "1"
	}

	if baro_account_consent_version(db, config.IP, kind) == version {
		return
	}

	tool.Exec_DB(
		db,
		"insert into baro_consent (id, user, kind, version, date, ip) values (?, ?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_consent"), config.IP, kind, version,
		tool.Baro_now(), tool.Baro_mask_ip(config.IP),
	)
}

// ------------------------------------------------------------
//  온보딩 진행 상태 (2-2)
// ------------------------------------------------------------

// baro_account_onboard_last : 온보딩 마지막 단계 번호
const baro_account_onboard_last = 4

// baro_account_onboard_get : 온보딩 진행 단계와 상태
func baro_account_onboard_get(db *sql.DB, user string) (int, string) {
	step, status := "", ""
	if !tool.QueryRow_DB(db, "select step, status from baro_onboard where user = ?", []any{&step, &status}, user) {
		return 1, "new"
	}

	num := tool.Str_to_int(step)
	if num < 1 {
		num = 1
	}
	if num > baro_account_onboard_last+1 {
		num = baro_account_onboard_last + 1
	}

	if status == "" {
		status = "open"
	}

	return num, status
}

// baro_account_onboard_put : 온보딩 진행 상태 저장
func baro_account_onboard_put(db *sql.DB, user string, step int, status string) {
	tool.Exec_DB(db, "delete from baro_onboard where user = ?", user)
	tool.Exec_DB(
		db,
		"insert into baro_onboard (user, step, status, date) values (?, ?, ?, ?)",
		user, tool.Baro_itoa(step), status, tool.Baro_now(),
	)
}

// baro_account_onboard_marks : 단계별 완료/건너뛰기 기록
func baro_account_onboard_marks(db *sql.DB, user string) map[string]string {
	return tool.Baro_json_dec_str(baro_account_uset_get(db, user, "onboard_steps"))
}

// baro_account_onboard_mark : 단계 하나의 상태를 기록 (done / skip)
func baro_account_onboard_mark(db *sql.DB, user string, step int, mark string) {
	marks := baro_account_onboard_marks(db, user)
	marks[tool.Baro_itoa(step)] = mark
	baro_account_uset_put(db, user, "onboard_steps", tool.Baro_json_enc(marks))
}

// baro_account_onboard_advance : 단계를 처리하고 다음 단계로 넘긴다
func baro_account_onboard_advance(db *sql.DB, user string, step int, mark string) {
	baro_account_onboard_mark(db, user, step, mark)

	now_step, _ := baro_account_onboard_get(db, user)
	next := step + 1
	if next < now_step {
		next = now_step
	}

	if next > baro_account_onboard_last {
		baro_account_onboard_put(db, user, baro_account_onboard_last+1, "done")

		return
	}

	baro_account_onboard_put(db, user, next, "open")
}

// ------------------------------------------------------------
//  분야 / 관심 분야
// ------------------------------------------------------------

// baro_account_field_options : 선택 상자용 분야 목록
func baro_account_field_options(db *sql.DB) [][]string {
	out := [][]string{}
	for _, row := range tool.Baro_field_list(db) {
		out = append(out, []string{row.Key, row.Name})
	}

	if len(out) == 0 {
		for _, row := range tool.Baro_field_default() {
			out = append(out, []string{row.Key, row.Name})
		}
	}

	return out
}

// baro_account_interest_put : 관심 분야 저장 (기존 목록을 대체한다)
func baro_account_interest_put(db *sql.DB, user string, fields []string) {
	tool.Exec_DB(db, "delete from baro_interest where user = ?", user)

	saved := map[string]bool{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || saved[field] {
			continue
		}
		saved[field] = true

		tool.Exec_DB(
			db,
			"insert into baro_interest (user, field, kind, date) values (?, ?, 'interest', ?)",
			user, field, tool.Baro_now(),
		)
	}
}

// baro_account_expertise_main : 대표(전문) 분야 목록
func baro_account_expertise_main(db *sql.DB, user string) []string {
	out := []string{}

	rows := tool.Query_DB(db, "select field from baro_expertise where user = ? and is_main = 'on'", user)
	defer rows.Close()

	for rows.Next() {
		field := ""
		if err := rows.Scan(&field); err != nil {
			continue
		}

		out = append(out, field)
	}

	return out
}

// baro_account_field_event_count : 사용자의 분야별 기여 이벤트 수
func baro_account_field_event_count(db *sql.DB, user string, field string) int {
	count := 0
	tool.QueryRow_DB(
		db,
		"select count(*) from baro_contrib_event where user = ? and field = ? and status != 'rejected'",
		[]any{&count}, user, field,
	)

	return count
}

// baro_account_expertise_put : 대표 분야 저장 (최대 3개)
func baro_account_expertise_put(db *sql.DB, user string, fields []string) {
	tool.Exec_DB(db, "update baro_expertise set is_main = '' where user = ?", user)

	saved := map[string]bool{}
	count := 0
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || saved[field] {
			continue
		}
		saved[field] = true

		count++
		if count > 3 {
			break
		}

		score := tool.Baro_itoa(baro_account_field_event_count(db, user, field))

		tool.Exec_DB(db, "delete from baro_expertise where user = ? and field = ?", user, field)
		tool.Exec_DB(
			db,
			"insert into baro_expertise (user, field, score, is_main, date) values (?, ?, ?, 'on', ?)",
			user, field, score, tool.Baro_now(),
		)
	}
}

// baro_account_field_suggest : 기여 기록으로 전문 분야를 추정한다 (상위 3개)
func baro_account_field_suggest(db *sql.DB, user string) [][]string {
	counts := map[string]int{}

	rows := tool.Query_DB(
		db,
		"select field from baro_contrib_event where user = ? and status != 'rejected'",
		user,
	)
	defer rows.Close()

	for rows.Next() {
		field := ""
		if err := rows.Scan(&field); err != nil {
			continue
		}
		if strings.TrimSpace(field) == "" {
			continue
		}

		counts[field]++
	}

	out := [][]string{}
	for field, count := range counts {
		out = append(out, []string{field, tool.Baro_itoa(count)})
	}

	sort.SliceStable(out, func(i int, j int) bool {
		if tool.Str_to_int(out[i][1]) != tool.Str_to_int(out[j][1]) {
			return tool.Str_to_int(out[i][1]) > tool.Str_to_int(out[j][1])
		}

		return out[i][0] < out[j][0]
	})

	if len(out) > 3 {
		out = out[:3]
	}

	return out
}

// baro_account_difficulty_list : 과제 난이도 (baro_project_task.difficulty)
func baro_account_difficulty_list() [][]string {
	return [][]string{
		{"easy", tool.Baro_t("쉬움"), "ok"},
		{"normal", tool.Baro_t("보통"), "info"},
		{"hard", tool.Baro_t("어려움"), "warn"},
	}
}

// baro_account_difficulty_name : 난이도 표시 이름 (모르는 값은 그대로 보여준다)
func baro_account_difficulty_name(difficulty string) string {
	for _, row := range baro_account_difficulty_list() {
		if row[0] == difficulty {
			return row[1]
		}
	}

	if strings.TrimSpace(difficulty) == "" {
		return tool.Baro_t("미정")
	}

	return difficulty
}

// baro_account_difficulty_kind : 난이도 배지 색
func baro_account_difficulty_kind(difficulty string) string {
	for _, row := range baro_account_difficulty_list() {
		if row[0] == difficulty {
			return row[2]
		}
	}

	return "muted"
}

// baro_account_difficulty_order : 난이도를 쉬운 순서로 정렬하는 SQL 조각
//
//	difficulty 는 easy / normal / hard 문자열이라 숫자로 정렬하면 전부 같은 값이 된다.
func baro_account_difficulty_order() string {
	return "case difficulty when 'easy' then 1 when 'normal' then 2 when 'hard' then 3 else 4 end asc"
}

// ------------------------------------------------------------
//  계정 정보
// ------------------------------------------------------------

// baro_account_join_date : 가입일 (없으면 "")
func baro_account_join_date(db *sql.DB, user string) string {
	info := tool.Get_user_register_info(db, user)
	if info["date"] != "" {
		return info["date"]
	}

	return tool.Get_user_set_data(db, user, "date")
}

// baro_account_email_verified : 이메일 인증 여부와 가려진 이메일
func baro_account_email_verified(db *sql.DB, user string) (bool, string) {
	email := tool.Get_user_set_data(db, user, "email")
	if strings.TrimSpace(email) == "" {
		return false, ""
	}

	masked := email
	if at := strings.Index(email, "@"); at > 1 {
		masked = tool.Get_slice(email, 0, 1) + strings.Repeat("*", at-1) + email[at:]
	}

	return true, masked
}

// baro_account_last_active : 마지막 활동 시각
func baro_account_last_active(db *sql.DB, user string) string {
	date := ""
	tool.QueryRow_DB(
		db,
		"select date from baro_contrib_event where user = ? order by id + 0 desc limit 1",
		[]any{&date}, user,
	)

	if date == "" {
		tool.QueryRow_DB(db, "select date from baro_calendar where user = ? order by date desc limit 1", []any{&date}, user)
	}

	return date
}

// baro_account_contrib_count : 유효 기여 수 (거부된 것 제외)
func baro_account_contrib_count(db *sql.DB, user string) int {
	count := 0
	tool.QueryRow_DB(
		db,
		"select count(*) from baro_contrib_event where user = ? and status != 'rejected'",
		[]any{&count}, user,
	)

	return count
}

// baro_account_recent_count : 최근 N 일 기여 수
func baro_account_recent_count(db *sql.DB, user string, days int) int {
	count := 0
	tool.QueryRow_DB(
		db,
		"select count(*) from baro_contrib_event where user = ? and date >= ?",
		[]any{&count}, user, tool.Baro_time_add(tool.Baro_now(), -days, 0, 0),
	)

	return count
}

// baro_account_sanction_html : 제재 상태 안내 (2-1)
func baro_account_sanction_html(db *sql.DB, user string) string {
	list := tool.Baro_sanction_active(db, user)
	if len(list) == 0 {
		return tool.Baro_alert("ok", tool.Baro_t("적용 중인 제재가 없습니다."))
	}

	rows := [][]string{}
	for _, row := range list {
		scope := row[2]
		if strings.TrimSpace(scope) == "" {
			scope = tool.Baro_t("전체")
		}

		case_html := `<span class="bw_muted">-</span>`
		if row[5] != "" {
			case_html = `<a href="/baro/case/view/` + tool.Url_parser(row[5]) + `">` + tool.HTML_escape(row[5]) + `번 사건</a>`
		}

		rows = append(rows, []string{
			tool.Baro_badge(tool.Baro_sanction_name(row[1]), "danger"),
			tool.HTML_escape(scope),
			tool.HTML_escape(row[3]),
			tool.HTML_escape(tool.Baro_duration_left(row[4])),
			case_html,
		})
	}

	html := tool.Baro_alert("danger", tool.Baro_t("현재 적용 중인 제재가 있습니다. 사유와 만료 시점을 확인하세요."))
	html += tool.Baro_table([]string{"종류", tool.Baro_t("범위"), "사유", tool.Baro_t("남은 기간"), "사건"}, rows, tool.Baro_t("적용 중인 제재가 없습니다."))
	html += tool.Baro_note(tool.Baro_t("제재는 사람에게만 적용되며, 제재를 이유로 문서가 삭제되지 않습니다. 결과에 동의할 수 없으면 이의제기를 신청할 수 있습니다."))
	html += tool.Baro_button("/baro/appeal", tool.Baro_t("이의제기 안내"), "")

	return html
}

// ------------------------------------------------------------
//  활동 캘린더 (잔디밭)
// ------------------------------------------------------------

// baro_account_calendar_html : 최근 1년 활동 캘린더
func baro_account_calendar_html(db *sql.DB, user string) string {
	counts := map[string]int{}

	rows := tool.Query_DB(db, "select date, num from baro_calendar where user = ?", user)
	defer rows.Close()

	for rows.Next() {
		date, num := "", "0"
		if err := rows.Scan(&date, &num); err != nil {
			continue
		}

		counts[tool.Baro_date_only(date)] = tool.Str_to_int(num)
	}

	today := time.Now()
	total := 0
	active_days := 0

	html := `<div class="bw_cal">`
	for offset := 364; offset >= 0; offset-- {
		day := today.AddDate(0, 0, -offset).Format("2006-01-02")
		num := counts[day]
		total += num
		if num > 0 {
			active_days++
		}

		level := ""
		switch {
		case num >= 10:
			level = " bw_cal_4"
		case num >= 5:
			level = " bw_cal_3"
		case num >= 2:
			level = " bw_cal_2"
		case num >= 1:
			level = " bw_cal_1"
		}

		html += `<span class="bw_cal_d` + level + `" title="` + tool.HTML_escape(day+" · "+tool.Baro_itoa(num)+tool.Baro_t("건")) + `"></span>`
	}
	html += `</div>`

	html += tool.Baro_stat_row([][]string{
		{tool.Baro_t("최근 1년 기록"), tool.Baro_comma(total), "건"},
		{tool.Baro_t("활동한 날"), tool.Baro_comma(active_days), tool.Baro_t("일")},
	})
	html += tool.Baro_note(tool.Baro_t("연속 기록을 강요하지 않습니다. 쉬어도 괜찮습니다."))

	return html
}

// ------------------------------------------------------------
//  초안 / 문법 검사 (2-3)
// ------------------------------------------------------------

// baro_account_check : 초안 본문 점검 결과
type baro_account_check struct {
	Length    int
	Links     []string
	Missing   []string
	Footnote  bool
	Category  bool
	Heading   bool
	Warns     []string
	Citations int
}

var baro_account_link_re = regexp.MustCompile(`\[\[([^\[\]|]+)(?:\|[^\[\]]*)?\]\]`)
var baro_account_foot_re = regexp.MustCompile(`\[\*[^\]]*`)
var baro_account_head_re = regexp.MustCompile(`(?m)^\s*=+\s.*\s=+\s*$`)

// baro_account_doc_exists : 문서가 이미 있는지
func baro_account_doc_exists(db *sql.DB, title string) bool {
	if strings.TrimSpace(title) == "" {
		return false
	}

	found := ""

	return tool.QueryRow_DB(db, "select title from data where title = ?", []any{&found}, title)
}

// baro_account_scan : 본문을 훑어 문법 오류 후보와 누락 항목을 찾는다
func baro_account_scan(db *sql.DB, body string) baro_account_check {
	out := baro_account_check{}
	out.Length = tool.Get_len(body)

	seen := map[string]bool{}
	for _, match := range baro_account_link_re.FindAllStringSubmatch(body, -1) {
		target := strings.TrimSpace(match[1])
		if target == "" {
			continue
		}

		if strings.HasPrefix(target, "분류:") {
			out.Category = true

			continue
		}

		if strings.HasPrefix(target, "파일:") || strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
			continue
		}

		if strings.HasPrefix(target, "#") {
			continue
		}

		if seen[target] {
			continue
		}
		seen[target] = true

		out.Links = append(out.Links, target)
		if !baro_account_doc_exists(db, tool.Namespace_normalize(target)) {
			out.Missing = append(out.Missing, target)
		}
	}

	out.Citations = len(baro_account_foot_re.FindAllString(body, -1))
	out.Footnote = out.Citations > 0
	out.Heading = baro_account_head_re.MatchString(body)

	if strings.Count(body, "[[") != strings.Count(body, "]]") {
		out.Warns = append(out.Warns, tool.Baro_t("대괄호 링크 [[ 와 ]] 의 개수가 맞지 않습니다."))
	}
	if strings.Count(body, "{{{") != strings.Count(body, "}}}") {
		out.Warns = append(out.Warns, tool.Baro_t("{{{ 와 }}} 의 개수가 맞지 않습니다."))
	}
	if strings.Count(body, "'''")%2 != 0 {
		out.Warns = append(out.Warns, tool.Baro_t("굵게 표시 ''' 가 짝을 이루지 않습니다."))
	}
	if strings.Contains(body, "[[[") {
		out.Warns = append(out.Warns, tool.Baro_t("대괄호가 세 개 이상 연달아 있습니다. 링크 문법은 [[문서명]] 입니다."))
	}
	if !out.Footnote {
		out.Warns = append(out.Warns, tool.Baro_t("출처(각주)가 하나도 없습니다. [* 출처 내용] 형식으로 근거를 남겨 주세요."))
	}
	if !out.Category {
		out.Warns = append(out.Warns, tool.Baro_t("분류가 없습니다. 맨 아래에 [[분류:이름]] 을 넣어 주세요."))
	}
	if out.Length > 0 && out.Length < 200 {
		out.Warns = append(out.Warns, tool.Baro_t("본문이 200자 미만입니다. 문서로 올리기에는 짧을 수 있습니다."))
	}

	return out
}

// baro_account_scan_html : 점검 결과를 화면용 HTML 로
func baro_account_scan_html(check baro_account_check) string {
	html := tool.Baro_stat_row([][]string{
		{tool.Baro_t("글자 수"), tool.Baro_comma(check.Length), ""},
		{tool.Baro_t("링크"), tool.Baro_comma(len(check.Links)), "개"},
		{"각주", tool.Baro_comma(check.Citations), "개"},
		{tool.Baro_t("문제 후보"), tool.Baro_comma(len(check.Warns) + len(check.Missing)), "건"},
	})

	items := []string{}
	for _, warn := range check.Warns {
		items = append(items, tool.Baro_badge("확인", "warn")+" "+tool.HTML_escape(warn))
	}

	if len(check.Missing) > 0 {
		links := []string{}
		for _, target := range check.Missing {
			links = append(links, `<a href="/w/`+tool.Doc_url(target)+`">`+tool.HTML_escape(target)+`</a>`)
		}

		items = append(items, tool.Baro_badge("빨간 링크", "danger")+" 아직 없는 문서로 연결됩니다 : "+strings.Join(links, ", "))
	}

	if len(items) == 0 {
		return html + tool.Baro_alert("ok", tool.Baro_t("눈에 띄는 문법 문제가 없습니다."))
	}

	html += tool.Baro_list(items)
	html += tool.Baro_note(tool.Baro_t("자동 점검이라 틀릴 수 있습니다. 의도한 표현이면 그대로 두어도 됩니다."))

	return html
}

// baro_account_checklist_html : 제출 전 체크리스트
func baro_account_checklist_html(title string, check baro_account_check) string {
	mark := func(ok bool, text string, help string) string {
		badge := tool.Baro_badge(tool.Baro_t("아직"), "warn")
		if ok {
			badge = tool.Baro_badge("확인", "ok")
		}

		return badge + " " + tool.HTML_escape(text) + ` <span class="bw_muted">` + tool.HTML_escape(help) + `</span>`
	}

	trimmed := strings.TrimSpace(title)
	title_ok := tool.Get_len(trimmed) >= 2 &&
		!strings.Contains(trimmed, "제목 없음") &&
		!strings.HasPrefix(trimmed, "연습")

	return tool.Baro_list([]string{
		mark(check.Footnote, tool.Baro_t("출처가 있나요?"), "각주 "+tool.Baro_itoa(check.Citations)+tool.Baro_t("개")),
		mark(check.Category, tool.Baro_t("분류를 넣었나요?"), tool.Baro_t("[[분류:이름]]")),
		mark(title_ok, tool.Baro_t("제목이 적절한가요?"), tool.Baro_t("두 글자 이상, 실제 주제를 가리키는 이름")),
		mark(check.Length >= 200, tool.Baro_t("내용이 충분한가요?"), tool.Baro_comma(check.Length)+tool.Baro_t("자")),
		mark(check.Heading, "문단 제목으로 나눴나요?", "== 문단 제목 =="),
	})
}

// ------------------------------------------------------------
//  출처 입력 도우미 (2-3)
// ------------------------------------------------------------

// baro_account_cite_kind_list : 출처 종류
func baro_account_cite_kind_list() [][]string {
	return [][]string{
		{"book", tool.Baro_t("책")},
		{"paper", tool.Baro_t("논문")},
		{"news", tool.Baro_t("신문 기사")},
		{"web", tool.Baro_t("웹사이트")},
	}
}

// baro_account_cite_kind_name : 출처 종류 이름
func baro_account_cite_kind_name(kind string) string {
	for _, row := range baro_account_cite_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return tool.Baro_t("책")
}

// baro_account_cite_build : 입력한 항목으로 각주 문법 문자열을 만든다
func baro_account_cite_build(kind string, values url.Values) string {
	get := func(name string) string {
		return strings.TrimSpace(values.Get(name))
	}

	parts := []string{}
	add := func(text string) {
		if strings.TrimSpace(text) != "" {
			parts = append(parts, strings.TrimSpace(text))
		}
	}

	switch kind {
	case "paper":
		add(get("author"))
		if get("title") != "" {
			add("〈" + get("title") + "〉")
		}
		if get("publisher") != "" {
			add("《" + get("publisher") + "》")
		}
		add(get("volume"))
		add(get("pub_date"))
		if get("page") != "" {
			add(get("page") + tool.Baro_t("쪽"))
		}
		add(get("url"))
	case "news":
		add(get("author"))
		if get("title") != "" {
			add("〈" + get("title") + "〉")
		}
		add(get("publisher"))
		add(get("pub_date"))
		add(get("url"))
	case "web":
		add(get("author"))
		if get("title") != "" {
			add("〈" + get("title") + "〉")
		}
		add(get("publisher"))
		add(get("url"))
		if get("access_date") != "" {
			add(get("access_date") + " 확인")
		}
		if get("archive_url") != "" {
			add(tool.Baro_t("보존본 ") + get("archive_url"))
		}
	default:
		add(get("author"))
		if get("title") != "" {
			add("《" + get("title") + "》")
		}
		add(get("publisher"))
		add(get("pub_date"))
		if get("page") != "" {
			add(get("page") + tool.Baro_t("쪽"))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return "[* " + strings.Join(parts, ", ") + ".]"
}

// ------------------------------------------------------------
//  미리보기 / 편집 도우미
// ------------------------------------------------------------

// baro_account_render : 연습장 미리보기 (실패해도 화면이 죽지 않게 감싼다)
func baro_account_render(db *sql.DB, body string) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = tool.Baro_alert("warn", tool.Baro_t("미리보기를 만들지 못했습니다. 원문을 그대로 보여줍니다.")) + tool.Baro_code(body)
		}
	}()

	if strings.TrimSpace(body) == "" {
		return tool.Baro_empty(tool.Baro_t("미리볼 내용이 없습니다."))
	}

	rendered := markup.Get_render(db, "", body, "thread", map[string]any{})["data"]
	if strings.TrimSpace(rendered) == "" {
		return tool.Baro_empty(tool.Baro_t("미리볼 내용이 없습니다."))
	}

	return `<div class="bw_card_b">` + rendered + `</div>`
}

// baro_account_syntax_guide : openNAMU 문법 30초 안내
func baro_account_syntax_guide() string {
	rows := [][]string{
		{tool.Baro_code("== 문단 제목 =="), "문단을 나눕니다. = 개수가 많을수록 하위 문단입니다."},
		{tool.Baro_code(tool.Baro_t("'''굵게''' / ''기울임''")), tool.Baro_t("글자를 강조합니다.")},
		{tool.Baro_code(tool.Baro_t("[[문서 이름]]")), tool.Baro_t("다른 문서로 연결합니다.")},
		{tool.Baro_code(tool.Baro_t("[[문서 이름|보일 글자]]")), tool.Baro_t("다르게 보이는 링크를 만듭니다.")},
		{tool.Baro_code(tool.Baro_t("[* 홍길동, 《책이름》, 출판사, 2020, 12쪽.]")), tool.Baro_t("각주(출처)를 답니다. 바로위키에서 가장 중요한 문법입니다.")},
		{tool.Baro_code(tool.Baro_t("[[분류:역사]]")), tool.Baro_t("문서를 분류에 넣습니다. 맨 아래에 씁니다.")},
		{tool.Baro_code(tool.Baro_t(" * 목록 항목")), tool.Baro_t("앞에 공백 한 칸을 두고 * 를 쓰면 목록이 됩니다.")},
		{tool.Baro_code(tool.Baro_t("||칸1||칸2||")), tool.Baro_t("표를 만듭니다.")},
	}

	return tool.Baro_table([]string{tool.Baro_t("문법"), tool.Baro_t("설명")}, rows, "")
}

// baro_account_editor_js : 초보자용 간단 편집 도구 (커서 위치에 문법 삽입 + 자동 저장)
func baro_account_editor_js(area_id string, storage_key string) string {
	id := tool.JS_escape(area_id)
	key := tool.JS_escape(storage_key)

	html := `<div class="bw_pills" id="bw_acc_bar_` + tool.HTML_escape(area_id) + `">`
	buttons := [][]string{
		{"'''", "'''", tool.Baro_t("굵게")},
		{"''", "''", tool.Baro_t("기울임")},
		{"[[", "]]", tool.Baro_t("링크")},
		{"[* ", "]", "각주"},
		{"\n== ", " ==\n", "문단 제목"},
		{tool.Baro_t("\n[[분류:"), "]]", "분류"},
		{"\n * ", "", tool.Baro_t("목록")},
		{"||", "||", tool.Baro_t("표 칸")},
	}

	for _, row := range buttons {
		html += `<button type="button" class="bw_btn" data-bw-open="` + tool.HTML_escape(row[0]) + `" data-bw-close="` + tool.HTML_escape(row[1]) + `">` + tool.HTML_escape(row[2]) + `</button>`
	}
	html += `</div>`
	html += `<p class="bw_note" id="bw_acc_save_` + tool.HTML_escape(area_id) + tool.Baro_t(`">쓰는 동안 이 브라우저에 자동으로 임시 저장됩니다.</p>`)

	html += `<script>
(function(){
  var area = document.getElementById("` + id + `");
  if (!area) { return; }
  var bar = document.getElementById("bw_acc_bar_` + id + `");
  var info = document.getElementById("bw_acc_save_` + id + `");
  var storage_key = "bw_acc_draft_` + key + `";

  function save() {
    try {
      window.localStorage.setItem(storage_key, area.value);
      if (info) { info.textContent = ` + strconv.Quote(tool.Baro_t("자동 저장됨 (")) + ` + new Date().toLocaleTimeString() + ` + strconv.Quote(tool.Baro_t(") — 이 브라우저에만 보관됩니다.")) + `; }
    } catch (e) { }
  }

  function insert(open, close) {
    var start = area.selectionStart || 0;
    var end = area.selectionEnd || 0;
    var value = area.value;
    var picked = value.substring(start, end);
    area.value = value.substring(0, start) + open + picked + close + value.substring(end);
    area.focus();
    var pos = start + open.length + picked.length;
    area.setSelectionRange(pos, pos);
    save();
  }

  if (bar) {
    var list = bar.querySelectorAll("button");
    for (var i = 0; i < list.length; i++) {
      (function(node){
        node.addEventListener("click", function(){
          insert(node.getAttribute("data-bw-open") || "", node.getAttribute("data-bw-close") || "");
        });
      })(list[i]);
    }
  }

  try {
    var kept = window.localStorage.getItem(storage_key);
    if (kept && area.value.replace(/\s/g, "") === "") {
      area.value = kept;
      if (info) { info.textContent = ` + strconv.Quote(tool.Baro_t("이전에 쓰던 내용을 이 브라우저에서 되살렸습니다.")) + `; }
    }
  } catch (e) { }

  var timer = null;
  area.addEventListener("input", function(){
    if (timer) { window.clearTimeout(timer); }
    timer = window.setTimeout(save, 1200);
  });

  var form = area.form;
  if (form) {
    form.addEventListener("submit", function(){
      try { window.localStorage.removeItem(storage_key); } catch (e) { }
    });
  }
})();
</script>`

	return html
}

// ------------------------------------------------------------
//  초안 (baro_workspace)
// ------------------------------------------------------------

// baro_account_draft_status_list : 초안 공개 설정
func baro_account_draft_status_list() [][]string {
	return [][]string{
		{"closed", tool.Baro_t("비공개 (나만 보기)")},
		{"open", tool.Baro_t("공개 (다른 기여자도 볼 수 있음)")},
	}
}

// baro_account_draft_status_name : 초안 상태 이름
func baro_account_draft_status_name(status string) string {
	switch status {
	case "done":
		return tool.Baro_t("게시 완료")
	case "open":
		return "공개"
	}

	return tool.Baro_t("비공개")
}

// baro_account_draft_status_kind : 초안 상태 배지 색
func baro_account_draft_status_kind(status string) string {
	switch status {
	case "done":
		return "ok"
	case "open":
		return "info"
	}

	return "muted"
}

// baro_account_draft_get : 초안 한 건 (없으면 nil)
func baro_account_draft_get(db *sql.DB, id string) map[string]string {
	project, title, data, user, date, status, note := "", "", "", "", "", "", ""
	if !tool.QueryRow_DB(
		db,
		"select project, title, data, user, date, status, note from baro_workspace where id = ?",
		[]any{&project, &title, &data, &user, &date, &status, &note}, id,
	) {
		return nil
	}

	return map[string]string{
		"id": id, "project": project, "title": title, "data": data,
		"user": user, "date": date, "status": status, "note": note,
	}
}

// baro_account_sandbox_get : 연습장 초안 (없으면 nil)
func baro_account_sandbox_get(db *sql.DB, user string) map[string]string {
	id := ""
	tool.QueryRow_DB(
		db,
		"select id from baro_workspace where user = ? and project = 'sandbox' order by id + 0 desc limit 1",
		[]any{&id}, user,
	)

	if id == "" {
		return nil
	}

	return baro_account_draft_get(db, id)
}

// baro_account_user_doc_link : 사용자 소개 문서 링크 (openNAMU 사용자 이름공간)
func baro_account_user_doc_link(user string) string {
	title := tool.Namespace_normalize("사용자:" + user)

	return `<a href="/w/` + tool.Doc_url(title) + `">사용자:` + tool.HTML_escape(user) + `</a>`
}

// ------------------------------------------------------------
//  알림 (2-6)
// ------------------------------------------------------------

// baro_account_notify_mode_list : 알림 수신 방식
func baro_account_notify_mode_list() [][]string {
	return [][]string{
		{"on", tool.Baro_t("즉시 받기")},
		{"daily", tool.Baro_t("일일 요약으로 받기")},
		{"off", tool.Baro_t("받지 않기")},
	}
}

// baro_account_notify_mode : 종류별 수신 방식 (기본 on)
func baro_account_notify_mode(db *sql.DB, user string, kind string) string {
	mode := ""
	tool.QueryRow_DB(db, "select mode from baro_notify_set where user = ? and kind = ?", []any{&mode}, user, kind)

	if mode != "on" && mode != "daily" && mode != "off" {
		return "on"
	}

	return mode
}

// baro_account_notify_mode_put : 종류별 수신 방식 저장
func baro_account_notify_mode_put(db *sql.DB, user string, kind string, mode string) {
	if mode != "on" && mode != "daily" && mode != "off" {
		mode = "on"
	}

	tool.Exec_DB(db, "delete from baro_notify_set where user = ? and kind = ?", user, kind)
	tool.Exec_DB(db, "insert into baro_notify_set (user, kind, mode) values (?, ?, ?)", user, kind, mode)
}

// baro_account_notify_kind_name : 알림 종류 이름
func baro_account_notify_kind_name(kind string) string {
	for _, row := range tool.Baro_notify_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}
