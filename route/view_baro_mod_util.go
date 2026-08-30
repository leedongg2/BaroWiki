package route

// ============================================================
//  바로위키 [mod] 모듈 — 공통 도구
//
//  10장 : 신고 · 운영 사건 · 제재 · 문서 보존 · 이의제기 · 대량 조치 안전장치
//
//  0장 고정 원칙 중 1·2·3·5 가 이 모듈에 전부 걸려 있다.
//   1. 이용자 제재와 콘텐츠 판단을 분리한다  → 제재 화면에 문서 삭제 버튼을 두지 않는다
//   2. 삭제보다 보존과 복구를 우선한다        → 삭제의 기본 경로는 임시 격리(baro_quarantine)
//   3. 대량 파괴 작업에는 안전장치를 둔다     → baro_mass_op 절차 (수 표시 → 목록 공개 →
//                                              2인 승인 → 스냅샷 → 유예 → 실행 → 공지 → 복구)
//   5. 근거는 공개하되 개인정보는 보호한다    → Baro_public_log / Baro_audit 분리
// ============================================================

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ------------------------------------------------------------
//  입력 다루기
// ------------------------------------------------------------

// baro_mod_val : 폼 값 하나를 공백을 다듬어 읽는다
func baro_mod_val(values url.Values, name string) string {
	if values == nil {
		return ""
	}

	return strings.TrimSpace(values.Get(name))
}

// baro_mod_raw : 폼 값을 다듬지 않고 그대로 읽는다 (본문 등)
func baro_mod_raw(values url.Values, name string) string {
	if values == nil {
		return ""
	}

	return values.Get(name)
}

// baro_mod_on : 체크 상자가 켜져 있는지
func baro_mod_on(values url.Values, name string) bool {
	return baro_mod_val(values, name) != ""
}

// baro_mod_page_num : 쪽 번호 (1 미만은 1)
func baro_mod_page_num(page string) int {
	num := tool.Str_to_int(page)
	if num < 1 {
		num = 1
	}

	return num
}

// baro_mod_esc : 화면에 넣을 사용자 입력을 안전하게
func baro_mod_esc(data string) string {
	return tool.HTML_escape(data)
}

// baro_mod_or : 값이 비었으면 대체 문자열
func baro_mod_or(data string, def string) string {
	if strings.TrimSpace(data) == "" {
		return def
	}

	return data
}

// baro_mod_muted : 값이 비었을 때 회색 대시
func baro_mod_muted(data string) string {
	if strings.TrimSpace(data) == "" {
		return `<span class="bw_muted">-</span>`
	}

	return baro_mod_esc(data)
}

// baro_mod_place_holders : ?, ?, ? 처럼 자리표시자를 필요한 만큼 만든다 (값은 절대 이어 붙이지 않는다)
func baro_mod_place_holders(count int) string {
	if count < 1 {
		return "?"
	}

	out := []string{}
	for index := 0; index < count; index++ {
		out = append(out, "?")
	}

	return strings.Join(out, ", ")
}

// baro_mod_lines : 여러 줄 입력을 정리한다 (공백 줄·중복 제거, 순서 유지)
func baro_mod_lines(data string) []string {
	out := []string{}
	seen := map[string]bool{}

	for _, line := range tool.Baro_split_lines(data) {
		line = strings.TrimSpace(line)
		if line == "" || seen[line] {
			continue
		}

		seen[line] = true
		out = append(out, line)
	}

	return out
}

// ------------------------------------------------------------
//  공통 화면 뼈대
// ------------------------------------------------------------

// baro_mod_menu : 바로위키 표준 메뉴
func baro_mod_menu() [][]any {
	return [][]any{{"baro", "바로위키"}, {"baro/report", "신고"}, {"baro/case", tool.Baro_t("운영 사건")}}
}

// baro_mod_tabs : 10장 전체 이동 줄
func baro_mod_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/report", "신고"},
		{"/baro/case", tool.Baro_t("운영 사건")},
		{"/baro/case/sanction/list", "제재"},
		{"/baro/quarantine", tool.Baro_t("문서 격리")},
		{"/baro/appeal", tool.Baro_t("이의제기")},
		{"/baro/mass", tool.Baro_t("대량 작업")},
	}, active)
}

// baro_mod_shell : 10장 화면 공통 껍데기
func baro_mod_shell(db *sql.DB, config tool.Config, title string, active string, body string) string {
	return tool.Baro_page(db, config, title, baro_mod_tabs(active)+body, baro_mod_menu())
}

// baro_mod_missing : 찾을 수 없는 항목 안내 (500 대신 200 안내 화면)
func baro_mod_missing(db *sql.DB, config tool.Config, active string, what string, back_url string, back_text string) string {
	body := tool.Baro_alert("warn", baro_mod_esc(what)+tool.Baro_t("을(를) 찾을 수 없습니다. 주소가 잘못되었거나 이미 삭제된 항목일 수 있습니다."))
	body += tool.Baro_card("", tool.Baro_button(back_url, back_text, "primary"))

	return baro_mod_shell(db, config, tool.Baro_t("찾을 수 없음"), active, body)
}

// baro_mod_read_only_block : 읽기 전용 모드면 안내 화면을 돌려준다
func baro_mod_read_only_block(db *sql.DB, config tool.Config, active string) (string, bool) {
	if !tool.Baro_read_only(db) {
		return "", false
	}

	body := tool.Baro_alert("warn", tool.Baro_t("지금은 읽기 전용 모드입니다. ")+baro_mod_esc(tool.Baro_read_only_reason(db)))

	return baro_mod_shell(db, config, tool.Baro_t("읽기 전용"), active, body), true
}

// ------------------------------------------------------------
//  분류 값
// ------------------------------------------------------------

// baro_mod_report_kind_list : 신고 종류 (10-1)
func baro_mod_report_kind_list() [][]string {
	return [][]string{
		{"doc_error", tool.Baro_t("문서 오류"), tool.Baro_t("사실과 다르거나 잘못된 서술이 있습니다.")},
		{"source", tool.Baro_t("출처 문제"), tool.Baro_t("출처가 없거나 출처가 내용을 뒷받침하지 못합니다.")},
		{"copyright", tool.Baro_t("저작권"), tool.Baro_t("다른 곳의 글·사진을 허락 없이 옮겨 왔습니다.")},
		{"privacy", tool.Baro_t("개인정보"), tool.Baro_t("실명·연락처·주소 등 개인정보가 노출되어 있습니다.")},
		{"defamation", tool.Baro_t("명예훼손 우려"), tool.Baro_t("특정인의 명예를 훼손할 수 있는 서술입니다.")},
		{"spam", tool.Baro_t("광고·도배"), tool.Baro_t("광고이거나 같은 내용을 반복해 올립니다.")},
		{"discussion", tool.Baro_t("토론 방해"), tool.Baro_t("토론을 방해하거나 합의를 무시합니다.")},
		{"account", tool.Baro_t("계정 악용"), tool.Baro_t("다중 계정·사칭 등 계정을 악용합니다.")},
		{"admin_action", tool.Baro_t("관리자 조치 이의"), tool.Baro_t("운영자의 조치가 부당하다고 생각합니다.")},
	}
}

// baro_mod_report_kind_options : 선택 상자용 (값, 이름)
func baro_mod_report_kind_options() [][]string {
	out := [][]string{}
	for _, row := range baro_mod_report_kind_list() {
		out = append(out, []string{row[0], row[1]})
	}

	return out
}

// baro_mod_report_kind_alias : 예전 자료·다른 화면에서 들어온 신고 종류 값을 이 모듈의 값으로 맞춘다
//
//	baro_report / baro_case 에는 이 모듈이 만들어지기 전에 쌓인 줄이 있고,
//	거기에는 content · sourcing 같은 옛 이름이 들어 있다.
//	새 표를 만들 수 없으므로 읽을 때 이름만 맞춰 준다.
func baro_mod_report_kind_alias(kind string) string {
	switch kind {
	case "content", "error", "fact", "wrong":
		return "doc_error"
	case "sourcing", "citation", "source_problem":
		return "source"
	case "personal", "privacy_leak":
		return "privacy"
	case "libel", "insult":
		return "defamation"
	case "ad", "advertise", "vandalism", "flood":
		return "spam"
	case "thread", "talk":
		return "discussion"
	case "sock", "multi", "impersonation":
		return "account"
	case "admin", "action":
		return "admin_action"
	}

	return kind
}

// baro_mod_report_kind_alias_all : 한 종류로 걸러 볼 때 함께 찾아야 하는 옛 이름까지
func baro_mod_report_kind_alias_all(kind string) []string {
	out := []string{kind}
	if kind == "" {
		return out
	}

	for _, legacy := range []string{
		"content", "error", "fact", "wrong",
		"sourcing", "citation", "source_problem",
		"personal", "privacy_leak",
		"libel", "insult",
		"ad", "advertise", "vandalism", "flood",
		"thread", "talk",
		"sock", "multi", "impersonation",
		"admin", "action",
	} {
		if baro_mod_report_kind_alias(legacy) == kind {
			out = append(out, legacy)
		}
	}

	return out
}

// baro_mod_report_kind_name : 신고 종류 이름
func baro_mod_report_kind_name(kind string) string {
	normalized := baro_mod_report_kind_alias(kind)

	for _, row := range baro_mod_report_kind_list() {
		if row[0] == normalized {
			return row[1]
		}
	}

	return baro_mod_or(kind, "기타")
}

// baro_mod_target_kind_list : 신고·사건 대상 종류
func baro_mod_target_kind_list() [][]string {
	return [][]string{
		{"doc", "문서"},
		{"user", tool.Baro_t("사용자")},
		{"thread", tool.Baro_t("토론")},
		{"file", tool.Baro_t("파일")},
		{"case", "사건"},
	}
}

// baro_mod_target_kind_alias : 옛 이름으로 저장된 대상 종류를 이 모듈의 값으로 맞춘다
func baro_mod_target_kind_alias(kind string) string {
	switch kind {
	case "document", "doc_name", "page":
		return "doc"
	case "member", "account", "contributor":
		return "user"
	case "discussion", "talk", "topic":
		return "thread"
	case "image", "upload":
		return "file"
	case "incident":
		return "case"
	}

	return kind
}

// baro_mod_target_kind_name : 대상 종류 이름
func baro_mod_target_kind_name(kind string) string {
	normalized := baro_mod_target_kind_alias(kind)

	for _, row := range baro_mod_target_kind_list() {
		if row[0] == normalized {
			return row[1]
		}
	}

	return baro_mod_or(kind, "기타")
}

// baro_mod_target_link : 대상 종류에 맞는 링크 (사용자 IP 는 공개 화면에서 가림)
func baro_mod_target_link(db *sql.DB, config tool.Config, target_kind string, target string, public bool) string {
	if strings.TrimSpace(target) == "" {
		return `<span class="bw_muted">-</span>`
	}

	switch baro_mod_target_kind_alias(target_kind) {
	case "doc", "file":
		return tool.Baro_doc_link(target)
	case "user":
		if public && !tool.Baro_is_user(target) {
			return baro_mod_esc(tool.Baro_mask_ip(target))
		}

		return tool.Baro_user_link(db, target, config.IP)
	case "thread":
		return `<a href="/thread/` + tool.Url_parser(target) + `">` + baro_mod_esc(target) + `</a>`
	case "case":
		return `<a href="/baro/case/view/` + tool.Url_parser(target) + `">` + baro_mod_esc(target) + `</a>`
	}

	return baro_mod_esc(target)
}

// baro_mod_priority_list : 우선순위 (숫자가 작을수록 급하다 — 정렬을 위해 숫자로 저장한다)
func baro_mod_priority_list() [][]string {
	return [][]string{
		{"0", tool.Baro_t("긴급")},
		{"1", tool.Baro_t("높음")},
		{"2", tool.Baro_t("보통")},
		{"3", tool.Baro_t("낮음")},
	}
}

// baro_mod_priority_alias : 글자로 저장된 옛 우선순위 값을 숫자로 맞춘다
//
//	정렬이 priority + 0 이므로 숫자가 아닌 값은 전부 0 으로 취급되어 순서가 뒤섞인다.
//	보여 줄 때만이라도 제 자리에 놓는다.
func baro_mod_priority_alias(priority string) string {
	switch priority {
	case "urgent", "critical":
		return "0"
	case "high":
		return "1"
	case "normal", "middle", "":
		return "2"
	case "low":
		return "3"
	}

	return priority
}

// baro_mod_priority_name : 우선순위 이름
func baro_mod_priority_name(priority string) string {
	normalized := baro_mod_priority_alias(priority)

	for _, row := range baro_mod_priority_list() {
		if row[0] == normalized {
			return row[1]
		}
	}

	return tool.Baro_t("보통")
}

// baro_mod_priority_badge : 우선순위 배지
func baro_mod_priority_badge(priority string) string {
	kind := "muted"
	switch baro_mod_priority_alias(priority) {
	case "0":
		kind = "danger"
	case "1":
		kind = "warn"
	case "2":
		kind = "info"
	}

	return tool.Baro_badge(baro_mod_priority_name(priority), kind)
}

// baro_mod_priority_of_kind : 신고 종류에 따른 기본 우선순위
//
//	법적 위험이 있는 신고는 사람이 손대기 전에 이미 위로 올라와 있어야 한다.
func baro_mod_priority_of_kind(kind string) string {
	switch kind {
	case "privacy", "copyright":
		return "0"
	case "defamation", "account":
		return "1"
	case "spam", "discussion", "admin_action":
		return "2"
	}

	return "2"
}

// baro_mod_status_list : 신고·사건 처리 상태
func baro_mod_status_list() [][]string {
	return [][]string{
		{"open", tool.Baro_t("접수됨")},
		{"pending", tool.Baro_t("처리 중")},
		{"closed", tool.Baro_t("종결")},
		{"rejected", tool.Baro_t("반려")},
	}
}

// baro_mod_status_open_sql : "아직 처리가 끝나지 않은" 상태를 고르는 SQL 조각
//
//	이 모듈은 접수 상태를 open 으로 쓰지만, 예전에 쌓인 줄에는 new 로 적혀 있다.
//	둘 다 잡지 않으면 큐가 비어 보인다.
const baro_mod_status_open_sql = " in ('open', 'new', 'pending') "

// baro_mod_status_new_sql : 아직 담당자가 없는 상태 (접수됨)
const baro_mod_status_new_sql = " in ('open', 'new') "

// baro_mod_status_is_open : 아직 처리 중인 상태인가
func baro_mod_status_is_open(status string) bool {
	switch status {
	case "open", "new", "pending", "":
		return true
	}

	return false
}

// baro_mod_status_name : 처리 상태 이름
func baro_mod_status_name(status string) string {
	for _, row := range baro_mod_status_list() {
		if row[0] == status {
			return row[1]
		}
	}

	switch status {
	case "new":
		return tool.Baro_t("접수됨")
	case "done":
		return tool.Baro_t("완료")
	case "active":
		return tool.Baro_t("적용 중")
	case "expired":
		return tool.Baro_t("만료")
	case "draft":
		return tool.Baro_t("작성 중")
	case "approved":
		return tool.Baro_t("승인됨")
	case "released":
		return tool.Baro_t("복구됨")
	case "deleted":
		return tool.Baro_t("삭제됨")
	case "undone":
		return tool.Baro_t("일괄 복구됨")
	}

	return baro_mod_or(status, "-")
}

// baro_mod_status_badge : 처리 상태 배지
func baro_mod_status_badge(status string) string {
	kind := "muted"
	switch status {
	case "open", "new":
		kind = "info"
	case "pending", "draft", "approved":
		kind = "warn"
	case "closed", "done", "released":
		kind = "ok"
	case "rejected", "deleted", "active":
		kind = "danger"
	}

	return tool.Baro_badge(baro_mod_status_name(status), kind)
}

// baro_mod_quarantine_kind_list : 격리 종류 (10-4)
func baro_mod_quarantine_kind_list() [][]string {
	return [][]string{
		{"normal", tool.Baro_t("일반")},
		{"copyright", tool.Baro_t("저작권 검토")},
		{"privacy", tool.Baro_t("개인정보 긴급 비공개")},
		{"false_info", tool.Baro_t("허위정보 검토")},
	}
}

// baro_mod_quarantine_kind_name : 격리 종류 이름
func baro_mod_quarantine_kind_name(kind string) string {
	for _, row := range baro_mod_quarantine_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return baro_mod_or(kind, tool.Baro_t("일반"))
}

// baro_mod_case_action_kind_list : 사건 조치 종류
func baro_mod_case_action_kind_list() [][]string {
	return [][]string{
		{"note", tool.Baro_t("경과 기록")},
		{"contact", tool.Baro_t("당사자 안내")},
		{"evidence", tool.Baro_t("증거 확보")},
		{"warn", tool.Baro_t("구두 경고")},
		{"sanction", tool.Baro_t("제재 집행")},
		{"quarantine", tool.Baro_t("문서 격리")},
		{"restore", tool.Baro_t("문서 복구")},
		{"reject", tool.Baro_t("반려")},
		{"close", tool.Baro_t("종결")},
	}
}

// baro_mod_case_action_kind_name : 조치 종류 이름
func baro_mod_case_action_kind_name(kind string) string {
	for _, row := range baro_mod_case_action_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return baro_mod_or(kind, tool.Baro_t("기록"))
}

// ------------------------------------------------------------
//  자료 구조
// ------------------------------------------------------------

// baro_mod_report_row : 신고 한 건
type baro_mod_report_row struct {
	ID         string
	Kind       string
	TargetKind string
	Target     string
	Reporter   string
	Detail     string
	Evidence   string
	Status     string
	CaseID     string
	Date       string
	Priority   string
	Note       string
}

// baro_mod_report_note : 신고의 내부 부가 정보 (note 컬럼에 JSON 으로 담는다)
//
//	baro_report 에 담당자·기한 컬럼이 따로 없으므로 이름-값 저장소로 쓴다.
func baro_mod_report_note(row baro_mod_report_row) map[string]string {
	out := tool.Baro_json_dec_str(row.Note)
	if out == nil {
		out = map[string]string{}
	}

	return out
}

// baro_mod_report_get : 신고 한 건 읽기
func baro_mod_report_get(db *sql.DB, id string) (baro_mod_report_row, bool) {
	row := baro_mod_report_row{}
	found := tool.QueryRow_DB(
		db,
		"select id, kind, target_kind, target, reporter, detail, evidence, status, case_id, date, priority, note from baro_report where id = ?",
		[]any{&row.ID, &row.Kind, &row.TargetKind, &row.Target, &row.Reporter, &row.Detail, &row.Evidence, &row.Status, &row.CaseID, &row.Date, &row.Priority, &row.Note},
		id,
	)

	return row, found && row.ID != ""
}

// baro_mod_case_row : 운영 사건 한 건
type baro_mod_case_row struct {
	ID            string
	Number        string
	Kind          string
	TargetKind    string
	Target        string
	Rule          string
	Actor         string
	Status        string
	Action        string
	PublicSummary string
	PrivateNote   string
	Date          string
	CloseDate     string
	Deadline      string
	AppealStatus  string
	Priority      string
}

// baro_mod_case_get : 사건 한 건 읽기 (id 또는 사건 번호로 찾는다)
func baro_mod_case_get(db *sql.DB, id string) (baro_mod_case_row, bool) {
	row := baro_mod_case_row{}
	target_list := []any{
		&row.ID, &row.Number, &row.Kind, &row.TargetKind, &row.Target, &row.Rule, &row.Actor,
		&row.Status, &row.Action, &row.PublicSummary, &row.PrivateNote, &row.Date,
		&row.CloseDate, &row.Deadline, &row.AppealStatus, &row.Priority,
	}

	const columns = "id, number, kind, target_kind, target, rule, actor, status, action, public_summary, private_note, date, close_date, deadline, appeal_status, priority"

	if tool.QueryRow_DB(db, "select "+columns+" from baro_case where id = ?", target_list, id) && row.ID != "" {
		return row, true
	}

	row = baro_mod_case_row{}
	if tool.QueryRow_DB(db, "select "+columns+" from baro_case where number = ?", target_list, id) && row.ID != "" {
		return row, true
	}

	return row, false
}

// baro_mod_case_number_make : 새 사건 번호 (BW-연도-순번)
func baro_mod_case_number_make(db *sql.DB) string {
	year := tool.Baro_date_only(tool.Baro_now())
	if len(year) >= 4 {
		year = year[:4]
	}

	count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_case where number like ?", []any{&count}, "BW-"+year+"-%")

	for index := count + 1; index < count+10000; index++ {
		number := "BW-" + year + "-" + baro_mod_pad4(index)

		found := ""
		if !tool.QueryRow_DB(db, "select number from baro_case where number = ?", []any{&found}, number) || found == "" {
			return number
		}
	}

	return "BW-" + year + "-" + tool.Get_random_key(4)
}

// baro_mod_pad4 : 4자리 0 채움
func baro_mod_pad4(num int) string {
	data := tool.Baro_itoa(num)
	for len(data) < 4 {
		data = "0" + data
	}

	return data
}

// baro_mod_case_link : 사건 링크
func baro_mod_case_link(db *sql.DB, case_id string) string {
	if strings.TrimSpace(case_id) == "" {
		return `<span class="bw_muted">-</span>`
	}

	row, ok := baro_mod_case_get(db, case_id)
	if !ok {
		return baro_mod_esc(case_id)
	}

	return `<a href="/baro/case/view/` + tool.Url_parser(row.ID) + `">` + baro_mod_esc(row.Number) + `</a>`
}

// baro_mod_appeal_row : 이의제기 한 건
type baro_mod_appeal_row struct {
	ID           string
	CaseID       string
	User         string
	Reason       string
	Extra        string
	Status       string
	Reviewer     string
	Result       string
	ResultReason string
	Date         string
	Deadline     string
	CloseDate    string
}

// baro_mod_appeal_get : 이의제기 한 건 읽기
func baro_mod_appeal_get(db *sql.DB, id string) (baro_mod_appeal_row, bool) {
	row := baro_mod_appeal_row{}
	found := tool.QueryRow_DB(
		db,
		"select id, case_id, user, reason, extra, status, reviewer, result, result_reason, date, deadline, close_date from baro_appeal where id = ?",
		[]any{&row.ID, &row.CaseID, &row.User, &row.Reason, &row.Extra, &row.Status, &row.Reviewer, &row.Result, &row.ResultReason, &row.Date, &row.Deadline, &row.CloseDate},
		id,
	)

	return row, found && row.ID != ""
}

// baro_mod_quarantine_row : 격리 한 건
type baro_mod_quarantine_row struct {
	ID          string
	Title       string
	Rev         string
	Kind        string
	Reason      string
	Actor       string
	Date        string
	Status      string
	Backup      string
	ReleaseDate string
	CaseID      string
}

// baro_mod_quarantine_get : 격리 한 건 읽기
func baro_mod_quarantine_get(db *sql.DB, id string) (baro_mod_quarantine_row, bool) {
	row := baro_mod_quarantine_row{}
	found := tool.QueryRow_DB(
		db,
		"select id, title, rev, kind, reason, actor, date, status, backup, release_date, case_id from baro_quarantine where id = ?",
		[]any{&row.ID, &row.Title, &row.Rev, &row.Kind, &row.Reason, &row.Actor, &row.Date, &row.Status, &row.Backup, &row.ReleaseDate, &row.CaseID},
		id,
	)

	return row, found && row.ID != ""
}

// baro_mod_mass_row : 대량 작업 한 건
type baro_mod_mass_row struct {
	ID           string
	Kind         string
	Actor        string
	TargetList   string
	Num          string
	Reason       string
	Status       string
	Approver     string
	Created      string
	ApprovedDate string
	ExecuteDate  string
	GraceUntil   string
	Urgent       string
	UndoStatus   string
	Detail       string
}

// baro_mod_mass_get : 대량 작업 한 건 읽기
func baro_mod_mass_get(db *sql.DB, id string) (baro_mod_mass_row, bool) {
	row := baro_mod_mass_row{}
	found := tool.QueryRow_DB(
		db,
		"select id, kind, actor, target_list, num, reason, status, approver, created, approved_date, execute_date, grace_until, urgent, undo_status, detail from baro_mass_op where id = ?",
		[]any{&row.ID, &row.Kind, &row.Actor, &row.TargetList, &row.Num, &row.Reason, &row.Status, &row.Approver, &row.Created, &row.ApprovedDate, &row.ExecuteDate, &row.GraceUntil, &row.Urgent, &row.UndoStatus, &row.Detail},
		id,
	)

	return row, found && row.ID != ""
}

// baro_mod_mass_targets : 대량 작업 대상 목록
func baro_mod_mass_targets(row baro_mod_mass_row) []string {
	return baro_mod_lines(row.TargetList)
}

// baro_mod_mass_approvers : 승인자 목록
func baro_mod_mass_approvers(row baro_mod_mass_row) []string {
	out := []string{}
	for _, name := range strings.Split(row.Approver, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		out = append(out, name)
	}

	return out
}

// ------------------------------------------------------------
//  문서 다루기 (2번 원칙 : 지우기 전에 반드시 보존한다)
// ------------------------------------------------------------

// baro_mod_doc_data : 문서 본문 읽기 (없으면 두 번째 값이 false)
func baro_mod_doc_data(db *sql.DB, title string) (string, bool) {
	data := ""
	found := tool.QueryRow_DB(db, "select data from data where title = ?", []any{&data}, title)

	return data, found
}

// baro_mod_doc_exists : 문서 존재 여부
func baro_mod_doc_exists(db *sql.DB, title string) bool {
	_, ok := baro_mod_doc_data(db, title)

	return ok
}

// baro_mod_doc_write : 문서 본문 저장 (역사도 함께 남긴다)
func baro_mod_doc_write(db *sql.DB, config tool.Config, title string, data string, send string) {
	tool.Exec_DB(db, "delete from data where title = ?", title)
	tool.Exec_DB(db, "insert into data (title, data, type) values (?, ?, '')", title, data)
	tool.Do_add_history(db, title, data, tool.Get_time(), config.IP, send, tool.Baro_itoa(tool.Get_len(data)), "", "")
}

// baro_mod_doc_last_editor : 문서의 마지막 기여자
func baro_mod_doc_last_editor(db *sql.DB, title string) string {
	ip := ""
	tool.QueryRow_DB(db, "select ip from history where title = ? order by id + 0 desc limit 1", []any{&ip}, title)

	return ip
}

// baro_mod_doc_editors : 문서에 손댄 사람 목록 (중복 없이)
func baro_mod_doc_editors(db *sql.DB, title string) []string {
	out := []string{}
	seen := map[string]bool{}

	rows := tool.Query_DB(db, "select ip from history where title = ? order by id + 0 desc limit 200", title)
	defer rows.Close()

	for rows.Next() {
		ip := ""
		if err := rows.Scan(&ip); err != nil {
			continue
		}

		if ip == "" || seen[ip] {
			continue
		}

		seen[ip] = true
		out = append(out, ip)
	}

	sort.Strings(out)

	return out
}

// baro_mod_doc_prev_rev : 직전 판 (되돌리기용) — 판 번호와 본문
func baro_mod_doc_prev_rev(db *sql.DB, title string) (string, string, bool) {
	rows := tool.Query_DB(db, "select id, data from history where title = ? order by id + 0 desc limit 2", title)
	defer rows.Close()

	list := [][]string{}
	for rows.Next() {
		id, data := "", ""
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}

		list = append(list, []string{id, data})
	}

	if len(list) < 2 {
		return "", "", false
	}

	return list[1][0], list[1][1], true
}

// baro_mod_quarantine_notice : 격리로 본문을 가릴 때 대신 넣는 안내문
func baro_mod_quarantine_notice(kind string, reason string) string {
	return tool.Baro_t("이 문서는 ") + baro_mod_quarantine_kind_name(kind) + tool.Baro_t(" 사유로 임시 격리되어 있습니다.\n\n") +
		tool.Baro_t("사유: ") + reason + "\n\n" +
		tool.Baro_t("원본 내용은 삭제되지 않고 그대로 보존되어 있으며, 검토가 끝나면 복구됩니다.\n") +
		tool.Baro_t("바로위키는 삭제보다 보존과 복구를 우선합니다. (고정 원칙 2)")
}

// ------------------------------------------------------------
//  기록 도우미
// ------------------------------------------------------------

// baro_mod_notify_user : 대상이 로그인 사용자일 때만 알림을 보낸다
func baro_mod_notify_user(db *sql.DB, user string, kind string, title string, target_url string, data string) {
	if !tool.Baro_is_user(user) {
		return
	}

	tool.Baro_notify(db, user, kind, title, target_url, data)
}

// baro_mod_log_both : 공개 로그와 내부 감사 기록을 한 번에 남긴다 (5번 원칙)
func baro_mod_log_both(db *sql.DB, config tool.Config, kind string, target string, reason string, detail string, before string, after string) {
	tool.Baro_public_log(db, kind, config.IP, target, reason, detail)
	tool.Baro_audit(db, config, kind, target, before, after, reason)
}

// ------------------------------------------------------------
//  정책 (사건에 적용 규정을 붙이기 위해 baro_policy 를 직접 읽는다)
// ------------------------------------------------------------

// baro_mod_policy_options : 정책 선택 상자 항목
func baro_mod_policy_options(db *sql.DB) [][]string {
	out := [][]string{{"", tool.Baro_t("(선택 안 함)")}}

	rows := tool.Query_DB(db, "select key_name, title from baro_policy where status = 'active' order by id + 0 asc")
	defer rows.Close()

	seen := map[string]bool{}
	for rows.Next() {
		key_name, title := "", ""
		if err := rows.Scan(&key_name, &title); err != nil {
			continue
		}

		if key_name == "" || seen[key_name] {
			continue
		}

		seen[key_name] = true
		out = append(out, []string{key_name, title})
	}

	return out
}

// baro_mod_policy_link : 적용 규정 링크 (근거 공개 — 5번 원칙)
func baro_mod_policy_link(db *sql.DB, key_name string) string {
	if strings.TrimSpace(key_name) == "" {
		return tool.Baro_t(`<span class="bw_muted">지정되지 않음</span>`)
	}

	title := ""
	tool.QueryRow_DB(
		db,
		"select title from baro_policy where key_name = ? and status = 'active' order by version + 0 desc limit 1",
		[]any{&title}, key_name,
	)

	if title == "" {
		title = key_name
	}

	return `<a href="/baro/policy/view/` + tool.Url_parser(key_name) + `">` + baro_mod_esc(title) + `</a>`
}

// ------------------------------------------------------------
//  이해충돌 (11-4 / 10-5)
// ------------------------------------------------------------

// baro_mod_coi_mark : 이해충돌을 baro_coi 에 기록한다 (같은 사건·같은 사람은 한 번만)
func baro_mod_coi_mark(db *sql.DB, case_id string, actor string, reason string) {
	if case_id == "" || actor == "" {
		return
	}

	found := ""
	tool.QueryRow_DB(db, "select id from baro_coi where case_id = ? and actor = ?", []any{&found}, case_id, actor)
	if found != "" {
		return
	}

	tool.Exec_DB(
		db,
		"insert into baro_coi (id, case_id, actor, reason, date) values (?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_coi"), case_id, actor, reason, tool.Baro_now(),
	)
}

// baro_mod_coi_list : 사건에 기록된 이해충돌
func baro_mod_coi_list(db *sql.DB, case_id string) [][]string {
	out := [][]string{}
	if case_id == "" {
		return out
	}

	rows := tool.Query_DB(db, "select actor, reason, date from baro_coi where case_id = ? order by id + 0 desc", case_id)
	defer rows.Close()

	for rows.Next() {
		actor, reason, date := "", "", ""
		if err := rows.Scan(&actor, &reason, &date); err != nil {
			continue
		}

		out = append(out, []string{actor, reason, date})
	}

	return out
}

// ------------------------------------------------------------
//  제재 도우미
// ------------------------------------------------------------

// baro_mod_sanction_is_block : 차단 계열 제재인지 (openNAMU 차단과 연동해야 하는 종류)
func baro_mod_sanction_is_block(kind string) bool {
	switch kind {
	case "short_block", "long_block", "perm_block":
		return true
	}

	return false
}

// baro_mod_sanction_options : 제재 종류 선택 상자
func baro_mod_sanction_options() [][]string {
	out := [][]string{}
	for _, row := range tool.Baro_sanction_kind_list() {
		out = append(out, []string{row[0], row[1]})
	}

	return out
}

// baro_mod_sanction_history : 사용자의 제재 이력 (단계적 제재 판단용)
func baro_mod_sanction_history(db *sql.DB, user string, limit int) [][]string {
	out := [][]string{}
	if strings.TrimSpace(user) == "" {
		return out
	}

	if limit < 1 {
		limit = 20
	}

	rows := tool.Query_DB(
		db,
		"select id, kind, scope, reason, status, start_date, end_date, case_id from baro_sanction where user = ? order by id + 0 desc limit ?",
		user, limit,
	)
	defer rows.Close()

	for rows.Next() {
		id, kind, scope, reason, status, start_date, end_date, case_id := "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &kind, &scope, &reason, &status, &start_date, &end_date, &case_id); err != nil {
			continue
		}

		out = append(out, []string{id, kind, scope, reason, status, start_date, end_date, case_id})
	}

	return out
}

// baro_mod_principle_note : 화면 아래에 늘 붙이는 고정 원칙 안내
func baro_mod_principle_note() string {
	return tool.Baro_note(
		tool.Baro_t("바로위키 고정 원칙 — 이용자 제재와 콘텐츠 판단은 분리합니다. ") +
			tool.Baro_t("삭제보다 보존과 복구를 우선합니다. 운영 근거는 공개하되 개인정보는 보호합니다."),
	)
}
