package route

import (
	"database/sql"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 6장 : 분야 개척 시스템 (공통 도우미)
//
//  이 파일에는 화면 여러 곳에서 함께 쓰는 자료 읽기 / 계산 / 조각 HTML 이 들어 있다.
//  package route 안의 비공개 이름은 전부 baro_project_ 로 시작한다.
// ============================================================

// baro_project_page_size : 목록 한 쪽에 보여줄 줄 수
const baro_project_page_size = 20

// ------------------------------------------------------------
//  자료 구조
// ------------------------------------------------------------

// baro_project_row : 프로젝트 한 건 (baro_project)
type baro_project_row struct {
	ID          string
	Name        string
	Field       string
	Goal        string
	Scope       string
	Dataset     string
	StartDate   string
	EndDate     string
	Owner       string
	Status      string
	TargetCount int
	DoneCount   int
	Rule        string
	FinishCond  string
	Date        string
	Summary     string
	Report      string
}

// baro_project_task_row : 과제 한 건 (baro_project_task)
type baro_project_task_row struct {
	ID         string
	Project    string
	Kind       string
	Title      string
	Detail     string
	Difficulty string
	EstTime    string
	Assignee   string
	Status     string
	Doc        string
	Date       string
	DoneDate   string
	Requester  string
	Field      string
}

// baro_project_target_row : 목표 한 건 (baro_project_target)
type baro_project_target_row struct {
	ID       string
	Project  string
	Name     string
	Note     string
	Status   string
	Doc      string
	Date     string
	Assignee string
}

// baro_project_member_row : 참여자 한 건 (baro_project_member)
type baro_project_member_row struct {
	ID      string
	Project string
	User    string
	Role    string
	Date    string
	Status  string
}

// baro_project_draft_row : 공동 작업실 초안 한 건 (baro_workspace)
type baro_project_draft_row struct {
	ID      string
	Project string
	Title   string
	Data    string
	User    string
	Date    string
	Status  string
	Note    string
}

// baro_project_field_stat : 분야 하나의 개척 현황
type baro_project_field_stat struct {
	Field        tool.Baro_field_row
	ProjectCount int
	ProjectGoal  int
	TargetTotal  int
	TargetDone   int
	DocCount     int
	NeedExpand   int
	NeedSource   int
	Reviewed     int
	WeekEvents   int
	TaskOpen     int
	PeopleCount  int
	Recorded     int
}

// ------------------------------------------------------------
//  분류 표
// ------------------------------------------------------------

// baro_project_status_list : 프로젝트 진행 상태
func baro_project_status_list() [][]string {
	return [][]string{
		{"open", tool.Baro_t("모집 중")},
		{"active", tool.Baro_t("진행 중")},
		{"pending", tool.Baro_t("잠시 멈춤")},
		{"done", tool.Baro_t("목표 달성")},
		{"closed", tool.Baro_t("종료")},
	}
}

// baro_project_status_name : 진행 상태 이름
func baro_project_status_name(status string) string {
	for _, row := range baro_project_status_list() {
		if row[0] == status {
			return row[1]
		}
	}

	return status
}

// baro_project_status_kind : 진행 상태 색상
func baro_project_status_kind(status string) string {
	switch status {
	case "open":
		return "info"
	case "active":
		return "ok"
	case "pending":
		return "warn"
	case "done":
		return "ok"
	case "closed":
		return "muted"
	}

	return "muted"
}

// baro_project_status_badge : 진행 상태 배지
func baro_project_status_badge(status string) string {
	if status == "" {
		return tool.Baro_badge(tool.Baro_t("진행 중"), "ok")
	}

	return tool.Baro_badge(baro_project_status_name(status), baro_project_status_kind(status))
}

// baro_project_role_list : 프로젝트 안의 역할 (6-5)
//
//	모두 콘텐츠 역할이다. 어떤 역할도 사이트 전체 이용자 제재 권한을 갖지 않는다.
func baro_project_role_list() [][]string {
	return [][]string{
		{"member", tool.Baro_t("참가자"), tool.Baro_t("목표 문서를 직접 찾아 쓰고 다듬는다")},
		{"reviewer", tool.Baro_t("검토자"), tool.Baro_t("작성된 문서의 사실과 출처를 검토한다")},
		{"source", tool.Baro_t("자료 담당"), tool.Baro_t("자료·데이터셋·출처를 모으고 정리한다")},
		{"file", tool.Baro_t("파일 담당"), tool.Baro_t("사진·지도·문서 이미지를 확보하고 올린다")},
		{"manager", tool.Baro_t("프로젝트 운영자"), tool.Baro_t("일정과 분담을 관리한다 (이용자 제재 권한 없음)")},
	}
}

// baro_project_role_norm : 다른 화면이나 예전 자료에서 들어온 역할 값을 표준 키로 맞춘다.
//
//	예 : owner / lead 로 저장된 값도 "프로젝트 운영자"(manager)로 읽는다.
func baro_project_role_norm(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "", "member", "participant", "writer":
		return "member"
	case "reviewer", "review", "checker":
		return "reviewer"
	case "source", "data", "dataset", "citation":
		return "source"
	case "file", "photo", "image", "picture":
		return "file"
	case "manager", "owner", "lead", "leader", "operator":
		return "manager"
	}

	return role
}

// baro_project_role_known : 우리가 아는 역할인가
func baro_project_role_known(role string) bool {
	for _, row := range baro_project_role_list() {
		if row[0] == baro_project_role_norm(role) {
			return true
		}
	}

	return false
}

// baro_project_role_name : 역할 이름
func baro_project_role_name(role string) string {
	key := baro_project_role_norm(role)
	for _, row := range baro_project_role_list() {
		if row[0] == key {
			return row[1]
		}
	}

	if role == "" {
		return tool.Baro_t("참가자")
	}

	return role
}

// baro_project_role_options : 역할 선택 상자용 목록
func baro_project_role_options() [][]string {
	out := [][]string{}
	for _, row := range baro_project_role_list() {
		out = append(out, []string{row[0], row[1]})
	}

	return out
}

// baro_project_task_kind_list : 과제 종류 (6-3)
func baro_project_task_kind_list() [][]string {
	return [][]string{
		{"need_doc", tool.Baro_t("필요한 문서 요청"), tool.Baro_t("아직 없는 문서를 새로 만들어 달라는 요청")},
		{"need_source", tool.Baro_t("출처 보강 요청"), tool.Baro_t("본문은 있으나 근거가 부족한 문서")},
		{"need_photo", tool.Baro_t("사진 요청"), tool.Baro_t("직접 찍었거나 자유 이용이 가능한 사진이 필요한 문서")},
		{"disambig", tool.Baro_t("동명이인 정리 요청"), tool.Baro_t("같은 이름이 섞여 있어 나누어야 하는 문서")},
		{"translate", tool.Baro_t("문서 번역 요청"), tool.Baro_t("다른 언어 자료를 옮겨 적어야 하는 문서")},
		{"beginner", tool.Baro_t("초보자용 과제"), tool.Baro_t("위키가 처음인 사람도 30분 안에 끝낼 수 있는 일")},
	}
}

// baro_project_task_kind_norm : 다른 화면이나 예전 자료에서 들어온 과제 종류 값을 표준 키로 맞춘다.
func baro_project_task_kind_norm(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "need_doc", "need_document", "document", "doc", "new_document", "request":
		return "need_doc"
	case "need_source", "source", "citation":
		return "need_source"
	case "need_photo", "photo", "image", "picture":
		return "need_photo"
	case "disambig", "disambiguation", "samename":
		return "disambig"
	case "translate", "translation":
		return "translate"
	case "beginner", "newbie", "starter":
		return "beginner"
	}

	return kind
}

// baro_project_task_kind_known : 우리가 아는 과제 종류인가
func baro_project_task_kind_known(kind string) bool {
	for _, row := range baro_project_task_kind_list() {
		if row[0] == baro_project_task_kind_norm(kind) {
			return true
		}
	}

	return false
}

// baro_project_task_kind_keys : 한 종류와 같은 뜻으로 쓰이는 값 전부
//
//	같은 뜻인데 다른 낱말로 저장된 과제가 걸러지지 않는 일을 막는다.
func baro_project_task_kind_keys(kind string) []string {
	switch baro_project_task_kind_norm(kind) {
	case "need_doc":
		return []string{"need_doc", "need_document", "document", "doc", "new_document", "request"}
	case "need_source":
		return []string{"need_source", "source", "citation"}
	case "need_photo":
		return []string{"need_photo", "photo", "image", "picture"}
	case "disambig":
		return []string{"disambig", "disambiguation", "samename"}
	case "translate":
		return []string{"translate", "translation"}
	case "beginner":
		return []string{"beginner", "newbie", "starter"}
	}

	return []string{kind}
}

// baro_project_task_kind_cond : 종류 조건 SQL 조각과 값 (별칭까지 함께 찾는다)
func baro_project_task_kind_cond(kind string) (string, []any) {
	keys := baro_project_task_kind_keys(kind)

	marks := []string{}
	args := []any{}
	for _, key := range keys {
		marks = append(marks, "?")
		args = append(args, key)
	}

	return "kind in (" + strings.Join(marks, ", ") + ")", args
}

// baro_project_task_kind_name : 과제 종류 이름
func baro_project_task_kind_name(kind string) string {
	key := baro_project_task_kind_norm(kind)
	for _, row := range baro_project_task_kind_list() {
		if row[0] == key {
			return row[1]
		}
	}

	if kind == "" {
		return tool.Baro_t("일반 요청")
	}

	return kind
}

// baro_project_task_kind_options : 과제 종류 선택 상자용 목록
func baro_project_task_kind_options() [][]string {
	out := [][]string{}
	for _, row := range baro_project_task_kind_list() {
		out = append(out, []string{row[0], row[1]})
	}

	return out
}

// baro_project_difficulty_list : 예상 난이도
func baro_project_difficulty_list() [][]string {
	return [][]string{
		{"easy", tool.Baro_t("쉬움")},
		{"normal", tool.Baro_t("보통")},
		{"hard", tool.Baro_t("어려움")},
	}
}

// baro_project_difficulty_name : 난이도 이름
func baro_project_difficulty_name(difficulty string) string {
	for _, row := range baro_project_difficulty_list() {
		if row[0] == difficulty {
			return row[1]
		}
	}

	return tool.Baro_t("보통")
}

// baro_project_difficulty_badge : 난이도 배지
func baro_project_difficulty_badge(difficulty string) string {
	kind := "info"
	switch difficulty {
	case "easy":
		kind = "ok"
	case "hard":
		kind = "danger"
	case "normal", "":
		kind = "info"
	}

	return tool.Baro_badge(baro_project_difficulty_name(difficulty), kind)
}

// baro_project_task_status_name : 과제 상태 이름
func baro_project_task_status_name(status string) string {
	switch status {
	case "", "open":
		return tool.Baro_t("모집 중")
	case "active":
		return tool.Baro_t("진행 중")
	case "done":
		return tool.Baro_t("완료")
	case "closed":
		return tool.Baro_t("닫힘")
	case "rejected":
		return tool.Baro_t("반려")
	}

	return status
}

// baro_project_task_status_badge : 과제 상태 배지
func baro_project_task_status_badge(status string) string {
	kind := "info"
	switch status {
	case "active":
		kind = "warn"
	case "done":
		kind = "ok"
	case "closed", "rejected":
		kind = "muted"
	}

	return tool.Baro_badge(baro_project_task_status_name(status), kind)
}

// baro_project_draft_status_name : 작업실 초안 상태 이름
func baro_project_draft_status_name(status string) string {
	switch status {
	case "", "draft":
		return tool.Baro_t("작성 중")
	case "review":
		return "검토 요청"
	case "done":
		return tool.Baro_t("게시 완료")
	}

	return status
}

// baro_project_draft_status_badge : 작업실 초안 상태 배지
func baro_project_draft_status_badge(status string) string {
	kind := "info"
	switch status {
	case "review":
		kind = "warn"
	case "done":
		kind = "ok"
	}

	return tool.Baro_badge(baro_project_draft_status_name(status), kind)
}

// ------------------------------------------------------------
//  화면 공통 조각
// ------------------------------------------------------------

// baro_project_menu : 스킨 메뉴
func baro_project_menu() [][]any {
	return [][]any{
		{"baro", "바로위키"},
		{"baro/field", "분야"},
		{"baro/project", "프로젝트"},
		{"baro/task", tool.Baro_t("과제")},
	}
}

// baro_project_tabs : 6장 안의 이동 탭
func baro_project_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/field", tool.Baro_t("분야 허브")},
		{"/baro/project", "프로젝트"},
		{"/baro/task", tool.Baro_t("과제")},
		{"/baro/task/mine", tool.Baro_t("내 과제")},
	}, active)
}

// baro_project_missing : 대상이 없을 때의 안내 화면 (500 을 내지 않는다)
func baro_project_missing(db *sql.DB, config tool.Config, title string, message string, back_url string, back_text string) string {
	body := baro_project_tabs("")
	body += tool.Baro_alert("warn", tool.HTML_escape(message))
	body += tool.Baro_card("", tool.Baro_button(back_url, back_text, "primary"))

	return tool.Baro_page(db, config, title, body, baro_project_menu())
}

// baro_project_readonly_block : 읽기 전용 모드면 안내 화면을 돌려준다
func baro_project_readonly_block(db *sql.DB, config tool.Config) (string, bool) {
	if !tool.Baro_read_only(db) {
		return "", false
	}

	body := tool.Baro_alert("danger", tool.HTML_escape(tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db)))
	body += tool.Baro_card("", tool.Baro_button("/baro/project", tool.Baro_t("프로젝트 목록으로"), ""))

	return tool.Baro_page(db, config, tool.Baro_t("읽기 전용"), body, baro_project_menu()), true
}

// baro_project_page_num : 쪽 번호 읽기
func baro_project_page_num(values url.Values) int {
	page := tool.Str_to_int(values.Get("page"))
	if page < 1 {
		page = 1
	}

	return page
}

// baro_project_field_options : 분야 선택 상자용 목록
func baro_project_field_options(db *sql.DB, all_label string) [][]string {
	out := [][]string{}
	if all_label != "" {
		out = append(out, []string{"", all_label})
	}

	for _, row := range tool.Baro_field_list(db) {
		out = append(out, []string{row.Key, row.Name})
	}

	return out
}

// baro_project_field_link : 분야 링크
func baro_project_field_link(db *sql.DB, key string) string {
	if key == "" {
		return tool.Baro_t(`<span class="bw_muted">미지정</span>`)
	}

	return `<a href="/baro/field/view/` + tool.Url_parser(key) + `">` + tool.HTML_escape(tool.Baro_field_name(db, key)) + `</a>`
}

// baro_project_link : 프로젝트 링크
func baro_project_link(db *sql.DB, id string) string {
	if id == "" {
		return `<span class="bw_muted">-</span>`
	}

	name := ""
	tool.QueryRow_DB(db, "select name from baro_project where id = ?", []any{&name}, id)
	if name == "" {
		name = "프로젝트 " + id
	}

	return `<a href="/baro/project/view/` + tool.Url_parser(id) + `">` + tool.HTML_escape(name) + `</a>`
}

// baro_project_query_string : 필터 값을 주소 질의 문자열로 (쪽 넘기기용 {} 유지)
func baro_project_query_string(base string, pairs [][]string) string {
	parts := []string{"page={}"}
	for _, pair := range pairs {
		if len(pair) < 2 || pair[1] == "" {
			continue
		}

		parts = append(parts, pair[0]+"="+url.QueryEscape(pair[1]))
	}

	return base + "?" + strings.Join(parts, "&")
}

// ------------------------------------------------------------
//  프로젝트 읽기
// ------------------------------------------------------------

// baro_project_get : 프로젝트 한 건 읽기
func baro_project_get(db *sql.DB, id string) (baro_project_row, bool) {
	row := baro_project_row{}
	if strings.TrimSpace(id) == "" {
		return row, false
	}

	target_count, done_count := "", ""
	found := tool.QueryRow_DB(
		db,
		`select coalesce(id, ''), coalesce(name, ''), coalesce(field, ''), coalesce(goal, ''),
			coalesce(scope, ''), coalesce(dataset, ''), coalesce(start_date, ''), coalesce(end_date, ''),
			coalesce(owner, ''), coalesce(status, ''), coalesce(target_count, '0'), coalesce(done_count, '0'),
			coalesce(rule, ''), coalesce(finish_cond, ''), coalesce(date, ''), coalesce(summary, ''), coalesce(report, '')
			from baro_project where id = ?`,
		[]any{
			&row.ID, &row.Name, &row.Field, &row.Goal,
			&row.Scope, &row.Dataset, &row.StartDate, &row.EndDate,
			&row.Owner, &row.Status, &target_count, &done_count,
			&row.Rule, &row.FinishCond, &row.Date, &row.Summary, &row.Report,
		},
		id,
	)

	row.TargetCount = tool.Str_to_int(target_count)
	row.DoneCount = tool.Str_to_int(done_count)

	return row, found && row.ID != ""
}

// baro_project_list_load : 조건에 맞는 프로젝트 목록
func baro_project_list_load(db *sql.DB, field string, status string, keyword string, offset int, limit int) []baro_project_row {
	out := []baro_project_row{}

	where := []string{}
	args := []any{}

	if field != "" {
		where = append(where, "field = ?")
		args = append(args, field)
	}
	if status != "" {
		where = append(where, "status = ?")
		args = append(args, status)
	}
	if keyword != "" {
		where = append(where, "name like ?")
		args = append(args, "%"+keyword+"%")
	}

	query := `select coalesce(id, ''), coalesce(name, ''), coalesce(field, ''), coalesce(goal, ''),
		coalesce(owner, ''), coalesce(status, ''), coalesce(target_count, '0'), coalesce(done_count, '0'),
		coalesce(start_date, ''), coalesce(end_date, ''), coalesce(date, '') from baro_project`
	if len(where) > 0 {
		query += " where " + strings.Join(where, " and ")
	}
	query += fmt.Sprintf(" order by id + 0 desc limit %d offset %d", limit, offset)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	for rows.Next() {
		row := baro_project_row{}
		target_count, done_count := "0", "0"

		if err := rows.Scan(
			&row.ID, &row.Name, &row.Field, &row.Goal,
			&row.Owner, &row.Status, &target_count, &done_count,
			&row.StartDate, &row.EndDate, &row.Date,
		); err != nil {
			continue
		}

		row.TargetCount = tool.Str_to_int(target_count)
		row.DoneCount = tool.Str_to_int(done_count)
		out = append(out, row)
	}

	return out
}

// ------------------------------------------------------------
//  참여자
// ------------------------------------------------------------

// baro_project_member_list : 참여자 목록
func baro_project_member_list(db *sql.DB, project string) []baro_project_member_row {
	out := []baro_project_member_row{}
	if project == "" {
		return out
	}

	rows := tool.Query_DB(
		db,
		`select coalesce(id, ''), coalesce(project, ''), coalesce(user, ''), coalesce(role, ''),
			coalesce(date, ''), coalesce(status, '') from baro_project_member where project = ? order by id + 0 asc`,
		project,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_project_member_row{}
		if err := rows.Scan(&row.ID, &row.Project, &row.User, &row.Role, &row.Date, &row.Status); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_project_member_role : 참여 중인 사용자의 역할 ("" 이면 참여하지 않음)
func baro_project_member_role(db *sql.DB, project string, user string) string {
	if project == "" || !tool.Baro_is_user(user) {
		return ""
	}

	role := ""
	found := tool.QueryRow_DB(
		db,
		"select coalesce(role, '') from baro_project_member where project = ? and user = ? and status = 'active'",
		[]any{&role}, project, user,
	)
	if !found {
		return ""
	}

	if role == "" {
		return "member"
	}

	return baro_project_role_norm(role)
}

// baro_project_is_member : 참여 중인지
func baro_project_is_member(db *sql.DB, project string, user string) bool {
	return baro_project_member_role(db, project, user) != ""
}

// baro_project_can_manage : 프로젝트를 관리할 수 있는지 (콘텐츠 권한만이다 — 제재 권한과 무관)
func baro_project_can_manage(db *sql.DB, config tool.Config, row baro_project_row) bool {
	if !tool.Baro_is_user(config.IP) {
		return false
	}

	if row.Owner != "" && row.Owner == config.IP {
		return true
	}

	if baro_project_member_role(db, row.ID, config.IP) == "manager" {
		return true
	}

	return tool.Baro_can(db, config.IP, "project")
}

// baro_project_member_add : 참여자 추가 (이미 있으면 다시 활성화)
func baro_project_member_add(db *sql.DB, project string, user string, role string) {
	if project == "" || !tool.Baro_is_user(user) {
		return
	}

	role = baro_project_role_norm(role)
	if role == "" {
		role = "member"
	}

	found := ""
	if tool.QueryRow_DB(db, "select id from baro_project_member where project = ? and user = ?", []any{&found}, project, user) {
		tool.Exec_DB(
			db,
			"update baro_project_member set status = 'active', role = ?, date = ? where project = ? and user = ?",
			role, tool.Baro_now(), project, user,
		)

		return
	}

	tool.Exec_DB(
		db,
		"insert into baro_project_member (id, project, user, role, date, status) values (?, ?, ?, ?, ?, 'active')",
		tool.Baro_next_id(db, "baro_project_member"), project, user, role, tool.Baro_now(),
	)
}

// ------------------------------------------------------------
//  목표 / 진행률
// ------------------------------------------------------------

// baro_project_target_list : 목표 목록
func baro_project_target_list(db *sql.DB, project string, status string, offset int, limit int) []baro_project_target_row {
	out := []baro_project_target_row{}
	if project == "" {
		return out
	}

	query := `select coalesce(id, ''), coalesce(project, ''), coalesce(name, ''), coalesce(note, ''),
		coalesce(status, ''), coalesce(doc, ''), coalesce(date, ''), coalesce(assignee, '')
		from baro_project_target where project = ?`
	args := []any{project}

	if status != "" {
		query += " and status = ?"
		args = append(args, status)
	}

	query += fmt.Sprintf(" order by id + 0 asc limit %d offset %d", limit, offset)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	for rows.Next() {
		row := baro_project_target_row{}
		if err := rows.Scan(&row.ID, &row.Project, &row.Name, &row.Note, &row.Status, &row.Doc, &row.Date, &row.Assignee); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_project_count : 조건 하나짜리 개수 세기
func baro_project_count(db *sql.DB, query string, args ...any) int {
	count := 0
	tool.QueryRow_DB(db, query, []any{&count}, args...)

	return count
}

// baro_project_target_sync : 목표가 실제 문서로 만들어졌는지 data 표와 대조해 자동으로 done 표시
//
//	목표 이름(또는 지정된 문서 이름)이 data 표에 있으면 완료로 본다.
func baro_project_target_sync(db *sql.DB, project string) (int, int) {
	if project == "" {
		return 0, 0
	}

	type pending struct {
		id    string
		title string
	}

	list := []pending{}
	done := 0
	total := 0

	rows := tool.Query_DB(
		db,
		`select coalesce(id, ''), coalesce(name, ''), coalesce(status, ''), coalesce(doc, '')
			from baro_project_target where project = ?`,
		project,
	)

	for rows.Next() {
		id, name, status, doc := "", "", "", ""
		if err := rows.Scan(&id, &name, &status, &doc); err != nil {
			continue
		}

		total++
		if status == "done" {
			done++
			continue
		}

		title := doc
		if strings.TrimSpace(title) == "" {
			title = name
		}

		if strings.TrimSpace(title) == "" {
			continue
		}

		list = append(list, pending{id: id, title: tool.Namespace_normalize(title)})
	}
	rows.Close()

	if tool.Baro_read_only(db) {
		return done, total
	}

	for _, item := range list {
		exist := ""
		if !tool.QueryRow_DB(db, "select title from data where title = ?", []any{&exist}, item.title) {
			continue
		}

		tool.Exec_DB(
			db,
			"update baro_project_target set status = 'done', doc = ?, date = ? where id = ?",
			item.title, tool.Baro_now(), item.id,
		)

		done++
	}

	return done, total
}

// baro_project_progress_auto_note : 자동 집계가 남기는 메모
const baro_project_progress_auto_note = "자동 집계"

// baro_project_progress_save : 오늘 자 진행 기록을 남긴다 (하루 한 줄)
//
//	사람이 적은 메모(체크포인트)를 자동 집계가 덮어쓰지 않는다.
func baro_project_progress_save(db *sql.DB, project string, done int, total int, note string) {
	if project == "" || tool.Baro_read_only(db) {
		return
	}

	today := tool.Baro_today()

	found, old_note := "", ""
	if tool.QueryRow_DB(
		db,
		"select coalesce(id, ''), coalesce(note, '') from baro_project_progress where project = ? and date = ?",
		[]any{&found, &old_note}, project, today,
	) {
		// 자동 집계나 빈 메모는 이미 적힌 사람 메모를 지우지 않는다.
		if strings.TrimSpace(note) == "" || note == baro_project_progress_auto_note {
			if strings.TrimSpace(old_note) != "" {
				note = old_note
			}
		}

		tool.Exec_DB(
			db,
			"update baro_project_progress set done = ?, total = ?, note = ? where id = ?",
			tool.Baro_itoa(done), tool.Baro_itoa(total), note, found,
		)

		return
	}

	tool.Exec_DB(
		db,
		"insert into baro_project_progress (id, project, date, done, total, note) values (?, ?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_project_progress"), project, today,
		tool.Baro_itoa(done), tool.Baro_itoa(total), note,
	)
}

// baro_project_recount : 목표 대조 + 진행률 갱신 + 진행 기록 남기기
func baro_project_recount(db *sql.DB, row baro_project_row) baro_project_row {
	done, total := baro_project_target_sync(db, row.ID)

	target := row.TargetCount
	if total > target {
		target = total
	}

	if tool.Baro_read_only(db) {
		row.DoneCount = done
		row.TargetCount = target

		return row
	}

	if done != row.DoneCount || target != row.TargetCount {
		tool.Exec_DB(
			db,
			"update baro_project set done_count = ?, target_count = ? where id = ?",
			tool.Baro_itoa(done), tool.Baro_itoa(target), row.ID,
		)
	}

	row.DoneCount = done
	row.TargetCount = target

	if total > 0 {
		baro_project_progress_save(db, row.ID, done, target, baro_project_progress_auto_note)
	}

	// 목표를 모두 채웠으면 참여자에게 한 번만 알린다.
	if target > 0 && done >= target {
		flag := "project_done_notify_" + row.ID
		if tool.Baro_get(db, flag) != "on" {
			tool.Baro_put(db, flag, "on")

			for _, member := range baro_project_member_list(db, row.ID) {
				if member.Status != "active" {
					continue
				}

				tool.Baro_notify(
					db, member.User, "project",
					tool.Baro_t("프로젝트 목표를 모두 채웠습니다 : ")+row.Name,
					"/baro/project/view/"+tool.Url_parser(row.ID), "",
				)
			}

			tool.Baro_public_log(db, "project_goal_done", "system", "project:"+row.ID, "목표 수 도달", row.Name)
		}
	}

	return row
}

// baro_project_progress_bar : 프로젝트 진행 막대
//
//	6-2 규칙 : 분모가 확정된 분야(kind=closed)의 프로젝트만 그냥 진행률로 보여 주고,
//	그 밖에는 "프로젝트가 스스로 정한 목표 수"가 분모임을 함께 밝힌다.
//	목표 수가 없으면 기록 수만 보여 준다.
func baro_project_progress_bar(db *sql.DB, row baro_project_row, label string) string {
	if row.TargetCount <= 0 {
		return tool.Baro_progress_open(row.DoneCount, 0, label)
	}

	if row.Field != "" && tool.Baro_field_get(db, row.Field).Kind == "closed" {
		return tool.Baro_progress(row.DoneCount, row.TargetCount, label)
	}

	return tool.Baro_progress_open(row.DoneCount, row.TargetCount, label)
}

// ------------------------------------------------------------
//  과제
// ------------------------------------------------------------

// baro_project_task_get : 과제 한 건 읽기
func baro_project_task_get(db *sql.DB, id string) (baro_project_task_row, bool) {
	row := baro_project_task_row{}
	if strings.TrimSpace(id) == "" {
		return row, false
	}

	found := tool.QueryRow_DB(
		db,
		`select coalesce(id, ''), coalesce(project, ''), coalesce(kind, ''), coalesce(title, ''),
			coalesce(detail, ''), coalesce(difficulty, ''), coalesce(est_time, ''), coalesce(assignee, ''),
			coalesce(status, ''), coalesce(doc, ''), coalesce(date, ''), coalesce(done_date, ''),
			coalesce(requester, ''), coalesce(field, '') from baro_project_task where id = ?`,
		[]any{
			&row.ID, &row.Project, &row.Kind, &row.Title,
			&row.Detail, &row.Difficulty, &row.EstTime, &row.Assignee,
			&row.Status, &row.Doc, &row.Date, &row.DoneDate,
			&row.Requester, &row.Field,
		},
		id,
	)

	return row, found && row.ID != ""
}

// baro_project_task_load : 조건에 맞는 과제 목록
func baro_project_task_load(db *sql.DB, where []string, args []any, offset int, limit int) []baro_project_task_row {
	out := []baro_project_task_row{}

	query := `select coalesce(id, ''), coalesce(project, ''), coalesce(kind, ''), coalesce(title, ''),
		coalesce(detail, ''), coalesce(difficulty, ''), coalesce(est_time, ''), coalesce(assignee, ''),
		coalesce(status, ''), coalesce(doc, ''), coalesce(date, ''), coalesce(done_date, ''),
		coalesce(requester, ''), coalesce(field, '') from baro_project_task`

	if len(where) > 0 {
		query += " where " + strings.Join(where, " and ")
	}
	query += fmt.Sprintf(" order by id + 0 desc limit %d offset %d", limit, offset)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	for rows.Next() {
		row := baro_project_task_row{}
		if err := rows.Scan(
			&row.ID, &row.Project, &row.Kind, &row.Title,
			&row.Detail, &row.Difficulty, &row.EstTime, &row.Assignee,
			&row.Status, &row.Doc, &row.Date, &row.DoneDate,
			&row.Requester, &row.Field,
		); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_project_task_row_cells : 과제 한 줄을 표 칸으로
func baro_project_task_row_cells(db *sql.DB, config tool.Config, row baro_project_task_row) []string {
	title := `<a href="/baro/task/view/` + tool.Url_parser(row.ID) + `">` + tool.HTML_escape(tool.Baro_trunc(row.Title, 60)) + `</a>`

	assignee := tool.Baro_t(`<span class="bw_muted">담당자 없음</span>`)
	if row.Assignee != "" {
		assignee = tool.Baro_user_link(db, row.Assignee, config.IP)
	}

	est := `<span class="bw_muted">-</span>`
	if row.EstTime != "" {
		est = tool.HTML_escape(row.EstTime)
	}

	return []string{
		title,
		tool.HTML_escape(baro_project_task_kind_name(row.Kind)),
		baro_project_field_link(db, row.Field),
		baro_project_difficulty_badge(row.Difficulty),
		est,
		assignee,
		baro_project_task_status_badge(row.Status),
		tool.Baro_time_cell(row.Date),
	}
}

// baro_project_task_headers : 과제 표 머리글
func baro_project_task_headers() []string {
	return []string{tool.Baro_t("과제"), "종류", "분야", tool.Baro_t("난이도"), tool.Baro_t("예상 시간"), "담당자", "상태", tool.Baro_t("등록")}
}

// ------------------------------------------------------------
//  분야 통계 (6-1 / 6-2)
// ------------------------------------------------------------

// baro_project_field_stat_get : 분야 하나의 개척 현황 집계
func baro_project_field_stat_get(db *sql.DB, field tool.Baro_field_row) baro_project_field_stat {
	stat := baro_project_field_stat{Field: field}

	stat.ProjectCount = baro_project_count(db, "select count(*) from baro_project where field = ?", field.Key)

	// 소속 프로젝트의 목표 수 합계 (모두 문자열 컬럼이라 Go 에서 더한다)
	rows := tool.Query_DB(db, "select coalesce(target_count, '0') from baro_project where field = ?", field.Key)
	for rows.Next() {
		value := "0"
		if err := rows.Scan(&value); err != nil {
			continue
		}

		stat.ProjectGoal += tool.Str_to_int(value)
	}
	rows.Close()

	stat.TargetTotal = baro_project_count(
		db,
		"select count(*) from baro_project_target where project in (select id from baro_project where field = ?)",
		field.Key,
	)
	stat.TargetDone = baro_project_count(
		db,
		"select count(*) from baro_project_target where status = 'done' and project in (select id from baro_project where field = ?)",
		field.Key,
	)

	// 이 분야에 속한 실제 문서 수 (여러 경로를 합쳐 중복 없이 센다)
	doc_titles := baro_project_field_doc_titles(db, field)
	stat.DocCount = len(doc_titles)

	// 상태별 수도 위와 같은 문서 묶음을 기준으로 센다. (기준이 다르면 숫자가 어긋난다)
	status_rows := tool.Query_DB(
		db,
		"select coalesce(title, ''), coalesce(status, '') from baro_doc_status where status in ('need_expand', 'need_source', 'reviewed')",
	)
	for status_rows.Next() {
		title, status := "", ""
		if err := status_rows.Scan(&title, &status); err != nil {
			continue
		}

		if !doc_titles[title] {
			continue
		}

		switch status {
		case "need_expand":
			stat.NeedExpand++
		case "need_source":
			stat.NeedSource++
		case "reviewed":
			stat.Reviewed++
		}
	}
	status_rows.Close()

	stat.WeekEvents = baro_project_count(
		db,
		"select count(*) from baro_contrib_event where field = ? and date >= ?",
		field.Key, tool.Baro_time_add(tool.Baro_now(), -7, 0, 0),
	)

	stat.TaskOpen = baro_project_count(
		db,
		"select count(*) from baro_project_task where field = ? and (status = '' or status = 'open' or status = 'active')",
		field.Key,
	)

	stat.PeopleCount = len(baro_project_field_people(db, field.Key))

	// "작성 완료"와 "기록된 문서"가 서로 다른 숫자로 보이지 않도록 같은 집계를 쓴다.
	stat.Recorded = stat.DocCount

	return stat
}

// baro_project_field_doc_titles : 이 분야에 속한다고 볼 수 있는 문서 이름 모음
//
//	baro_doc_meta 의 field 태그(작업실 게시본)만 보면 /edit/ 로 직접 만든 문서가 빠진다.
//	그래서 아래 네 가지를 합쳐 중복 없이 센다.
//	  1) baro_doc_meta 의 field 태그
//	  2) 이 분야 프로젝트의 목표(완료된 목표는 doc, 아니면 목표 이름)
//	  3) 이 분야 과제가 가리키는 문서
//	  4) 분야 이름과 같은 분류가 달린 문서
//	실제로 data 표에 있는 문서만 센다. (지워진 문서는 빼야 숫자가 정직해진다)
func baro_project_field_doc_titles(db *sql.DB, field tool.Baro_field_row) map[string]bool {
	out := map[string]bool{}
	if field.Key == "" {
		return out
	}

	collect := func(query string, values ...any) {
		rows := tool.Query_DB(db, query, values...)
		defer rows.Close()

		for rows.Next() {
			title := ""
			if err := rows.Scan(&title); err != nil {
				continue
			}

			title = strings.TrimSpace(title)
			if title == "" {
				continue
			}

			out[title] = true
		}
	}

	// 1) 분야 태그가 붙은 문서
	collect(
		`select m.title from baro_doc_meta m
			where m.name = 'field' and m.data = ?
				and exists (select 1 from data d where d.title = m.title)`,
		field.Key,
	)

	// 2) 이 분야 프로젝트의 목표 · 완료 문서
	collect(
		`select case when coalesce(t.doc, '') <> '' then t.doc else coalesce(t.name, '') end
			from baro_project_target t
			where t.project in (select id from baro_project where field = ?)
				and exists (
					select 1 from data d
					where d.title = case when coalesce(t.doc, '') <> '' then t.doc else coalesce(t.name, '') end
				)`,
		field.Key,
	)

	// 3) 이 분야 과제가 가리키는 문서
	collect(
		`select coalesce(t.doc, '') from baro_project_task t
			where t.field = ? and coalesce(t.doc, '') <> ''
				and exists (select 1 from data d where d.title = t.doc)`,
		field.Key,
	)

	// 4) 분야 이름과 같은 분류가 달린 문서
	//    back 표는 title 칸에 분류 문서 이름, link 칸에 그 분류에 속한 문서 이름을 담는다.
	collect(
		`select b.link from back b
			where b.type = 'cat' and b.title in (?, ?, ?)
				and exists (select 1 from data d where d.title = b.link)`,
		"category:"+field.Name, "분류:"+field.Name, "category:"+field.Key,
	)

	return out
}

// baro_project_field_people : 이 분야의 참여자 (관심 분야 + 프로젝트 참여자)
func baro_project_field_people(db *sql.DB, key string) []string {
	seen := map[string]bool{}
	out := []string{}

	add := func(user string) {
		if user == "" || seen[user] {
			return
		}

		seen[user] = true
		out = append(out, user)
	}

	rows := tool.Query_DB(db, "select coalesce(user, '') from baro_interest where field = ?", key)
	for rows.Next() {
		user := ""
		if err := rows.Scan(&user); err == nil {
			add(user)
		}
	}
	rows.Close()

	rows2 := tool.Query_DB(
		db,
		`select coalesce(user, '') from baro_project_member
			where status = 'active' and project in (select id from baro_project where field = ?)`,
		key,
	)
	for rows2.Next() {
		user := ""
		if err := rows2.Scan(&user); err == nil {
			add(user)
		}
	}
	rows2.Close()

	sort.Strings(out)

	return out
}

// baro_project_field_top : 이 분야의 주요 기여자 (횟수가 아니라 기여 가치 합으로 정렬 — 4번 원칙)
func baro_project_field_top(db *sql.DB, key string, limit int) [][]string {
	type score struct {
		user   string
		count  int
		weight int
	}

	table := map[string]*score{}

	rows := tool.Query_DB(
		db,
		`select coalesce(user, ''), coalesce(weight, '0') from baro_contrib_event
			where field = ? and status <> 'rejected' order by id + 0 desc limit 2000`,
		key,
	)
	defer rows.Close()

	for rows.Next() {
		user, weight := "", "0"
		if err := rows.Scan(&user, &weight); err != nil {
			continue
		}

		if user == "" {
			continue
		}

		if _, ok := table[user]; !ok {
			table[user] = &score{user: user}
		}

		table[user].count++
		table[user].weight += tool.Str_to_int(weight)
	}

	list := []*score{}
	for _, value := range table {
		list = append(list, value)
	}

	sort.SliceStable(list, func(i int, j int) bool {
		if list[i].weight == list[j].weight {
			return list[i].user < list[j].user
		}

		return list[i].weight > list[j].weight
	})

	out := [][]string{}
	for index, value := range list {
		if index >= limit {
			break
		}

		out = append(out, []string{value.user, tool.Baro_itoa(value.count), tool.Baro_itoa(value.weight)})
	}

	return out
}

// baro_project_field_recent : 이 분야의 최근 작업
func baro_project_field_recent(db *sql.DB, key string, limit int) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(
		db,
		fmt.Sprintf(
			`select coalesce(user, ''), coalesce(kind, ''), coalesce(title, ''), coalesce(date, ''), coalesce(project, '')
				from baro_contrib_event where field = ? order by id + 0 desc limit %d`,
			limit,
		),
		key,
	)
	defer rows.Close()

	for rows.Next() {
		user, kind, title, date, project := "", "", "", "", ""
		if err := rows.Scan(&user, &kind, &title, &date, &project); err != nil {
			continue
		}

		out = append(out, []string{user, kind, title, date, project})
	}

	return out
}

// baro_project_field_todo : 이 분야의 미작성 목록 (아직 문서가 없는 목표)
func baro_project_field_todo(db *sql.DB, key string, limit int) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(
		db,
		fmt.Sprintf(
			`select coalesce(id, ''), coalesce(name, ''), coalesce(project, ''), coalesce(note, '')
				from baro_project_target
				where status = 'todo' and project in (select id from baro_project where field = ?)
				order by id + 0 desc limit %d`,
			limit,
		),
		key,
	)
	defer rows.Close()

	for rows.Next() {
		id, name, project, note := "", "", "", ""
		if err := rows.Scan(&id, &name, &project, &note); err != nil {
			continue
		}

		out = append(out, []string{id, name, project, note})
	}

	return out
}

// baro_project_field_count_note : 문서 수를 어떤 기준으로 세는지 안내
//
//	위쪽 "작성 완료"와 아래쪽 "기록된 문서"가 같은 기준을 쓴다는 것을 밝혀 둔다.
func baro_project_field_count_note() string {
	return tool.Baro_note(
		tool.Baro_t("문서 수는 분야 태그가 붙은 문서 · 이 분야 프로젝트의 목표와 완료 문서 · 이 분야 과제가 가리키는 문서 · ") +
			tool.Baro_t("분야 이름과 같은 분류가 달린 문서를 중복 없이 합쳐서 셉니다. ") +
			tool.Baro_t("위쪽 지표의 '작성 완료'도 똑같은 기준으로 셉니다."),
	)
}

// baro_project_open_note : 개척률을 쓰지 않는 이유 안내 (6-2)
func baro_project_open_note() string {
	return tool.Baro_note(
		tool.Baro_t("전체 목록이 확정된 분야에만 개척률(%)을 표시합니다. ") +
			tool.Baro_t("게임·애니처럼 전체 문서 수를 아무도 모르는 분야에 '개척률 32.4%' 같은 숫자를 붙이면 ") +
			tool.Baro_t("근거 없는 정밀도가 되기 때문에, 그런 분야에는 기록 수와 프로젝트 목표 수만 보여 줍니다."),
	)
}

// baro_project_field_progress_html : 분야 개척 현황 막대 (6-2 규칙)
func baro_project_field_progress_html(field tool.Baro_field_row, stat baro_project_field_stat) string {
	if field.Kind == "closed" && field.TargetCount > 0 {
		html := tool.Baro_progress(stat.Recorded, field.TargetCount, tool.Baro_t("개척률"))
		html += tool.Baro_note(
			tool.Baro_t("전체 목록이 확정된 분야입니다. 공인 목록 ") + tool.Baro_comma(field.TargetCount) + tool.Baro_t("건 중 ") +
				tool.Baro_comma(stat.Recorded) + tool.Baro_t("건 기록 완료."),
		)

		return html
	}

	html := tool.Baro_progress_open(stat.Recorded, stat.ProjectGoal, field.Name+tool.Baro_t(" 기록 현황"))
	if stat.ProjectGoal > 0 {
		html += tool.Baro_note(tool.Baro_t("전체 목록이 확정되지 않아 개척률 대신 프로젝트가 스스로 정한 목표 수를 분모로 씁니다."))
	}

	return html
}

// ------------------------------------------------------------
//  자료 모음 / 작업실
// ------------------------------------------------------------

// baro_project_source_list : 프로젝트 자료 모음
func baro_project_source_list(db *sql.DB, project string, offset int, limit int) [][]string {
	out := [][]string{}
	if project == "" {
		return out
	}

	rows := tool.Query_DB(
		db,
		fmt.Sprintf(
			`select coalesce(id, ''), coalesce(title, ''), coalesce(url, ''), coalesce(note, ''),
				coalesce(user, ''), coalesce(date, '') from baro_project_source
				where project = ? order by id + 0 desc limit %d offset %d`,
			limit, offset,
		),
		project,
	)
	defer rows.Close()

	for rows.Next() {
		id, title, link, note, user, date := "", "", "", "", "", ""
		if err := rows.Scan(&id, &title, &link, &note, &user, &date); err != nil {
			continue
		}

		out = append(out, []string{id, title, link, note, user, date})
	}

	return out
}

// baro_project_link_html : 외부 주소를 안전하게 링크로 (http/https 만 허용)
func baro_project_link_html(link string) string {
	link = strings.TrimSpace(link)
	if link == "" {
		return `<span class="bw_muted">-</span>`
	}

	lower := strings.ToLower(link)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return tool.HTML_escape(link)
	}

	return `<a href="` + tool.HTML_escape(link) + `" rel="nofollow noopener">` + tool.HTML_escape(tool.Baro_trunc(link, 60)) + `</a>`
}

// baro_project_draft_list : 작업실 초안 목록
func baro_project_draft_list(db *sql.DB, project string) []baro_project_draft_row {
	out := []baro_project_draft_row{}
	if project == "" {
		return out
	}

	rows := tool.Query_DB(
		db,
		`select coalesce(id, ''), coalesce(project, ''), coalesce(title, ''), coalesce(data, ''),
			coalesce(user, ''), coalesce(date, ''), coalesce(status, ''), coalesce(note, '')
			from baro_workspace where project = ? order by id + 0 desc limit 200`,
		project,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_project_draft_row{}
		if err := rows.Scan(&row.ID, &row.Project, &row.Title, &row.Data, &row.User, &row.Date, &row.Status, &row.Note); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_project_draft_get : 초안 한 건
func baro_project_draft_get(db *sql.DB, id string) (baro_project_draft_row, bool) {
	row := baro_project_draft_row{}
	if strings.TrimSpace(id) == "" {
		return row, false
	}

	found := tool.QueryRow_DB(
		db,
		`select coalesce(id, ''), coalesce(project, ''), coalesce(title, ''), coalesce(data, ''),
			coalesce(user, ''), coalesce(date, ''), coalesce(status, ''), coalesce(note, '')
			from baro_workspace where id = ?`,
		[]any{&row.ID, &row.Project, &row.Title, &row.Data, &row.User, &row.Date, &row.Status, &row.Note},
		id,
	)

	return row, found && row.ID != ""
}

// baro_project_room_log : 작업실 변경 내역 (공개 로그에 남긴다 — 개인정보 제외)
func baro_project_room_log(db *sql.DB, project string, actor string, reason string, detail string) {
	tool.Baro_public_log(db, "project_room", actor, "project:"+project, reason, detail)
}

// baro_project_room_history : 작업실 변경 내역 읽기
func baro_project_room_history(db *sql.DB, project string, limit int) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(
		db,
		fmt.Sprintf(
			`select coalesce(actor, ''), coalesce(reason, ''), coalesce(detail, ''), coalesce(date, '')
				from baro_public_log where kind = 'project_room' and target = ?
				order by id + 0 desc limit %d`,
			limit,
		),
		"project:"+project,
	)
	defer rows.Close()

	for rows.Next() {
		actor, reason, detail, date := "", "", "", ""
		if err := rows.Scan(&actor, &reason, &detail, &date); err != nil {
			continue
		}

		out = append(out, []string{actor, reason, detail, date})
	}

	return out
}

// ------------------------------------------------------------
//  진행 기록 그래프
// ------------------------------------------------------------

// baro_project_progress_list : 진행 기록
func baro_project_progress_list(db *sql.DB, project string, limit int) [][]string {
	out := [][]string{}
	if project == "" {
		return out
	}

	rows := tool.Query_DB(
		db,
		fmt.Sprintf(
			`select coalesce(id, ''), coalesce(date, ''), coalesce(done, '0'), coalesce(total, '0'), coalesce(note, '')
				from baro_project_progress where project = ? order by date asc limit %d`,
			limit,
		),
		project,
	)
	defer rows.Close()

	for rows.Next() {
		id, date, done, total, note := "", "", "0", "0", ""
		if err := rows.Scan(&id, &date, &done, &total, &note); err != nil {
			continue
		}

		out = append(out, []string{id, date, done, total, note})
	}

	return out
}

// baro_project_chart : 날짜별 done 추이를 인라인 SVG 꺾은선 그래프로
func baro_project_chart(list [][]string) string {
	if len(list) < 2 {
		return tool.Baro_note(tool.Baro_t("진행 기록이 두 개 이상 쌓이면 꺾은선 그래프가 나타납니다."))
	}

	width := 640
	height := 200
	pad := 30

	max_value := 1
	for _, row := range list {
		done := tool.Str_to_int(row[2])
		total := tool.Str_to_int(row[3])

		if done > max_value {
			max_value = done
		}
		if total > max_value {
			max_value = total
		}
	}

	point_x := func(index int) int {
		if len(list) == 1 {
			return pad
		}

		return pad + index*(width-pad*2)/(len(list)-1)
	}
	point_y := func(value int) int {
		return height - pad - value*(height-pad*2)/max_value
	}

	done_points := []string{}
	total_points := []string{}
	for index, row := range list {
		x := point_x(index)
		done_points = append(done_points, fmt.Sprintf("%d,%d", x, point_y(tool.Str_to_int(row[2]))))
		total_points = append(total_points, fmt.Sprintf("%d,%d", x, point_y(tool.Str_to_int(row[3]))))
	}

	html := fmt.Sprintf(
		tool.Baro_t(`<div class="bw_table_wrap"><svg viewBox="0 0 %d %d" width="100%%" height="200" role="img" aria-label="날짜별 진행 추이">`),
		width, height,
	)

	// 바탕 격자
	for i := 0; i <= 4; i++ {
		y := pad + i*(height-pad*2)/4
		html += fmt.Sprintf(
			`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="rgba(127,127,127,.25)" stroke-width="1" />`,
			pad, y, width-pad, y,
		)
	}

	html += `<polyline fill="none" stroke="rgba(127,127,127,.55)" stroke-width="2" stroke-dasharray="4 4" points="` + strings.Join(total_points, " ") + `" />`
	html += `<polyline fill="none" stroke="#010080" stroke-width="2.5" points="` + strings.Join(done_points, " ") + `" />`

	for index, row := range list {
		x := point_x(index)
		y := point_y(tool.Str_to_int(row[2]))
		html += fmt.Sprintf(`<circle cx="%d" cy="%d" r="3" fill="#0014de"><title>%s : %s</title></circle>`,
			x, y, tool.HTML_escape(row[1]), tool.HTML_escape(row[2]))
	}

	html += fmt.Sprintf(
		`<text x="%d" y="%d" font-size="11" fill="currentColor">%s</text>`,
		pad, pad-10, tool.HTML_escape(tool.Baro_t("최대 ")+tool.Baro_comma(max_value)),
	)
	html += fmt.Sprintf(
		`<text x="%d" y="%d" font-size="11" fill="currentColor">%s</text>`,
		pad, height-8, tool.HTML_escape(list[0][1]),
	)
	html += fmt.Sprintf(
		`<text x="%d" y="%d" font-size="11" text-anchor="end" fill="currentColor">%s</text>`,
		width-pad, height-8, tool.HTML_escape(list[len(list)-1][1]),
	)

	html += `</svg></div>`
	html += tool.Baro_note(tool.Baro_t("진한 선은 완료 수, 점선은 목표 수입니다."))

	return html
}
