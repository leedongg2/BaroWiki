package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 15장 : 개인정보 · 저작권 · 법적 요청 (모듈 키 legal)
//
//  이 파일은 legal 모듈이 공용으로 쓰는 작은 도구를 모아 둔 곳이다.
//  화면 자체는 view_baro_legal_privacy.go / _copyright.go / _license.go / _log.go 에 있다.
//
//  0장 고정 원칙 가운데 이 모듈이 직접 지켜야 하는 것
//   - 원칙 1 : 신고자·대상자에 대한 판단과 콘텐츠 판단을 섞지 않는다.
//   - 원칙 2 : 저작권 신고는 즉시 삭제가 아니라 임시 격리(baro_quarantine)로 처리한다.
//   - 원칙 5 : 통계와 공개 로그에는 이름·문서명·IP·연락처를 남기지 않는다.
// ============================================================

// baro_legal_menu : 이 모듈 화면의 공통 메뉴
func baro_legal_menu() [][]any {
	return [][]any{{"baro", "바로위키"}}
}

// baro_legal_style : 이 모듈에서만 쓰는 아주 작은 보조 스타일
func baro_legal_style() string {
	return `<style id="bw_legal_css">` +
		`.bw_legal_text{white-space:pre-wrap;line-height:1.75;word-break:break-word;}` +
		`.bw_legal_quote{border-left:3px solid rgba(127,127,127,.35);padding:.2rem 0 .2rem .8rem;margin:.5rem 0;}` +
		`</style>`
}

// baro_legal_page : legal 모듈 표준 페이지 (초안 안내 + 탭을 항상 위에 붙인다)
func baro_legal_page(db *sql.DB, config tool.Config, title string, active string, body string) string {
	full := baro_legal_style() + baro_legal_draft_alert() + baro_legal_tabs(active) + body

	return tool.Baro_page(db, config, title, full, baro_legal_menu())
}

// baro_legal_draft_alert : 법률 검토 전 초안 안내 (15장 전체 공통)
func baro_legal_draft_alert() string {
	return tool.Baro_alert(
		"warn",
		tool.Baro_t(`<strong>법률 검토 전 초안입니다.</strong> `)+
			tool.Baro_t(`이 화면의 법률 문구(개인정보 처리 방침 · 기여 라이선스 · 저작권 처리 절차)는 아직 법률 검토를 받지 않은 초안이며, `)+
			tool.Baro_t(`정식 공개 전에 실제 검토를 거쳐 바뀔 수 있습니다. 접수 · 처리 · 기록 기능 자체는 지금도 그대로 동작합니다.`),
	)
}

// baro_legal_tabs : 15장 공통 탭
func baro_legal_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/privacy", tool.Baro_t("개인정보 요청")},
		{"/baro/privacy/policy", tool.Baro_t("처리 방침")},
		{"/baro/copyright", tool.Baro_t("저작권 요청")},
		{"/baro/legal/license", tool.Baro_t("기여 라이선스")},
		{"/baro/legal", tool.Baro_t("법적 요청 통계")},
	}, active)
}

// ------------------------------------------------------------
//  폼 / 목록 보조
// ------------------------------------------------------------

// baro_legal_form_get : 폼 값 읽기 (앞뒤 공백 제거)
func baro_legal_form_get(values url.Values, name string) string {
	if values == nil {
		return ""
	}

	return strings.TrimSpace(values.Get(name))
}

// baro_legal_form_raw : 폼 값 읽기 (줄바꿈을 살려야 하는 본문용)
func baro_legal_form_raw(values url.Values, name string) string {
	if values == nil {
		return ""
	}

	return values.Get(name)
}

// baro_legal_form_on : 체크 상자가 켜져 있는지
func baro_legal_form_on(values url.Values, name string) bool {
	if values == nil {
		return false
	}

	return strings.TrimSpace(values.Get(name)) != ""
}

// baro_legal_page_num : 쪽 번호 정규화
func baro_legal_page_num(page string) int {
	num := tool.Str_to_int(page)
	if num < 1 {
		num = 1
	}

	return num
}

// baro_legal_count : count(*) 한 줄 읽기
func baro_legal_count(db *sql.DB, query string, values ...any) int {
	count := 0
	tool.QueryRow_DB(db, query, []any{&count}, values...)

	return count
}

// baro_legal_text_html : 여러 줄 본문을 안전하게 출력
func baro_legal_text_html(data string) string {
	if strings.TrimSpace(data) == "" {
		return tool.Baro_empty(tool.Baro_t("내용이 비어 있습니다."))
	}

	return `<div class="bw_legal_text">` + tool.HTML_escape(data) + `</div>`
}

// baro_legal_quote_html : 사용자 입력 인용 상자
func baro_legal_quote_html(data string) string {
	if strings.TrimSpace(data) == "" {
		return `<span class="bw_muted">-</span>`
	}

	return `<div class="bw_legal_quote bw_legal_text">` + tool.HTML_escape(data) + `</div>`
}

// baro_legal_period_norm : 기간(YYYY-MM) 정규화
func baro_legal_period_norm(period string) string {
	period = strings.TrimSpace(period)
	if len(period) >= 7 {
		return period[:7]
	}

	return tool.Get_month()
}

// baro_legal_month_add : YYYY-MM 에서 몇 달 전/후
func baro_legal_month_add(period string, delta int) string {
	base := tool.Baro_parse_time(baro_legal_period_norm(period) + "-01")
	if base.IsZero() {
		return tool.Get_month()
	}

	return base.AddDate(0, delta, 0).Format("2006-01")
}

// ------------------------------------------------------------
//  상태 / 처리 결과
// ------------------------------------------------------------

// baro_legal_status_list : 요청 상태 목록
func baro_legal_status_list() [][]string {
	return [][]string{
		{"new", tool.Baro_t("새 접수")},
		{"open", tool.Baro_t("접수됨")},
		{"pending", tool.Baro_t("처리 중")},
		{"done", "처리 완료"},
		{"rejected", "기각"},
		{"closed", tool.Baro_t("종료")},
	}
}

// baro_legal_status_name : 상태 이름
func baro_legal_status_name(status string) string {
	for _, row := range baro_legal_status_list() {
		if row[0] == status {
			return row[1]
		}
	}

	if status == "" {
		return tool.Baro_t("새 접수")
	}

	return status
}

// baro_legal_status_kind : 상태 색상
func baro_legal_status_kind(status string) string {
	switch status {
	case "new", "open", "":
		return "info"
	case "pending":
		return "warn"
	case "done":
		return "ok"
	case "rejected":
		return "danger"
	case "closed":
		return "muted"
	}

	return "muted"
}

// baro_legal_status_badge : 상태 배지
func baro_legal_status_badge(status string) string {
	return tool.Baro_badge(baro_legal_status_name(status), baro_legal_status_kind(status))
}

// baro_legal_status_open : 아직 처리가 끝나지 않았는지
func baro_legal_status_open(status string) bool {
	return status == "" || status == "new" || status == "open" || status == "pending"
}

// baro_legal_status_ok : 우리가 아는 상태 값인지
func baro_legal_status_ok(status string) bool {
	for _, row := range baro_legal_status_list() {
		if row[0] == status {
			return true
		}
	}

	return false
}

// baro_legal_open_where : 아직 처리가 끝나지 않은 요청을 고르는 조건
//
//	시드 데이터와 다른 모듈이 status 를 'new' 로 넣는 경우가 있어 함께 센다.
const baro_legal_open_where = "(status = 'new' or status = 'open' or status = 'pending' or status = '')"

// baro_legal_closed_where : 처리가 끝난 요청을 고르는 조건
const baro_legal_closed_where = "(status = 'done' or status = 'rejected' or status = 'closed')"

// baro_legal_result_list : 처리 결과 코드
func baro_legal_result_list() [][]string {
	return [][]string{
		{"", tool.Baro_t("결정 없음")},
		{"approved", "승인"},
		{"partial", tool.Baro_t("일부 승인")},
		{"rejected", tool.Baro_t("거부")},
	}
}

// baro_legal_result_name : 처리 결과 이름
func baro_legal_result_name(code string) string {
	for _, row := range baro_legal_result_list() {
		if row[0] == code {
			return row[1]
		}
	}

	return code
}

// baro_legal_result_kind : 처리 결과 색상
func baro_legal_result_kind(code string) string {
	switch code {
	case "approved":
		return "ok"
	case "partial":
		return "warn"
	case "rejected":
		return "danger"
	}

	return "muted"
}

// baro_legal_result_enc : 처리 결과를 JSON 한 칸에 담는다 (새 컬럼을 만들지 않기 위해)
func baro_legal_result_enc(code string, note string) string {
	if code == "" && strings.TrimSpace(note) == "" {
		return ""
	}

	return tool.Baro_json_enc(map[string]string{"code": code, "note": note})
}

// baro_legal_result_code : 저장된 처리 결과에서 코드만
func baro_legal_result_code(result string) string {
	result = strings.TrimSpace(result)
	if result == "" {
		return ""
	}

	if strings.HasPrefix(result, "{") {
		return tool.Baro_json_dec_str(result)["code"]
	}

	return ""
}

// baro_legal_result_note : 저장된 처리 결과에서 설명만
func baro_legal_result_note(result string) string {
	result = strings.TrimSpace(result)
	if result == "" {
		return ""
	}

	if strings.HasPrefix(result, "{") {
		return tool.Baro_json_dec_str(result)["note"]
	}

	return result
}

// baro_legal_result_cell : 표에 넣을 처리 결과 칸
func baro_legal_result_cell(result string) string {
	code := baro_legal_result_code(result)
	note := baro_legal_result_note(result)

	if code == "" && note == "" {
		return `<span class="bw_muted">-</span>`
	}

	html := ""
	if code != "" {
		html += tool.Baro_badge(baro_legal_result_name(code), baro_legal_result_kind(code))
	}

	if note != "" {
		if html != "" {
			html += " "
		}

		html += `<span class="bw_note">` + tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(note), 80)) + `</span>`
	}

	return html
}

// baro_legal_result_approved : 승인(또는 일부 승인)되었는지
func baro_legal_result_approved(status string, result string) bool {
	if status != "done" {
		return false
	}

	code := baro_legal_result_code(result)

	return code == "approved" || code == "partial"
}

// ------------------------------------------------------------
//  개인정보 요청 종류
// ------------------------------------------------------------

// baro_legal_privacy_kind_list : 개인정보 요청 종류 (키, 이름, 설명)
func baro_legal_privacy_kind_list() [][]string {
	return [][]string{
		{"view", tool.Baro_t("열람"), tool.Baro_t("우리가 보관 중인 내 정보를 항목까지 확인합니다.")},
		{"edit", "수정", tool.Baro_t("잘못 저장된 내 정보를 고쳐 달라고 요청합니다.")},
		{"download", tool.Baro_t("다운로드"), tool.Baro_t("내 정보를 기계가 읽을 수 있는 JSON 파일로 받습니다.")},
		{"leave", tool.Baro_t("계정 탈퇴"), tool.Baro_t("계정을 닫습니다. 이미 공개된 정상 기여는 자동으로 지워지지 않습니다.")},
		{"delete", tool.Baro_t("삭제 요청"), tool.Baro_t("특정 개인정보 항목의 삭제를 요청합니다.")},
	}
}

// baro_legal_privacy_kind_name : 요청 종류 이름
func baro_legal_privacy_kind_name(kind string) string {
	for _, row := range baro_legal_privacy_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// baro_legal_privacy_kind_ok : 알고 있는 요청 종류인지
func baro_legal_privacy_kind_ok(kind string) bool {
	for _, row := range baro_legal_privacy_kind_list() {
		if row[0] == kind {
			return true
		}
	}

	return false
}

// baro_legal_privacy_kind_options : 선택 상자용 목록
func baro_legal_privacy_kind_options() [][]string {
	out := [][]string{}
	for _, row := range baro_legal_privacy_kind_list() {
		out = append(out, []string{row[0], row[1]})
	}

	return out
}

// ------------------------------------------------------------
//  저작권 신고 대상 종류
// ------------------------------------------------------------

// baro_legal_target_kind_list : 신고 대상 종류
func baro_legal_target_kind_list() [][]string {
	return [][]string{
		{"doc", "문서"},
		{"rev", tool.Baro_t("특정 판(리비전)")},
		{"file", tool.Baro_t("파일")},
	}
}

// baro_legal_target_kind_norm : 대상 종류를 우리가 쓰는 값으로 맞춘다
//
//	다른 화면이나 시드 데이터가 "document" · "revision" 처럼 긴 이름을 넣어 두는 경우가 있다.
func baro_legal_target_kind_norm(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "rev", "revision", "history":
		return "rev"
	case "file", "upload":
		return "file"
	}

	return "doc"
}

// baro_legal_target_kind_name : 대상 종류 이름
func baro_legal_target_kind_name(kind string) string {
	kind = baro_legal_target_kind_norm(kind)

	for _, row := range baro_legal_target_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// baro_legal_target_kind_ok : 알고 있는 대상 종류인지
func baro_legal_target_kind_ok(kind string) bool {
	for _, row := range baro_legal_target_kind_list() {
		if row[0] == kind {
			return true
		}
	}

	return false
}

// ------------------------------------------------------------
//  정책 본문 (baro_policy)
// ------------------------------------------------------------

// baro_legal_policy_row : 시행 중인 정책 한 판
type baro_legal_policy_row struct {
	ID            string
	KeyName       string
	Title         string
	Version       string
	Data          string
	EffectiveDate string
	Status        string
	Date          string
}

// baro_legal_policy_get : 시행 중인 최신 판을 읽는다 (없으면 가장 최근 판)
func baro_legal_policy_get(db *sql.DB, key_name string) baro_legal_policy_row {
	row := baro_legal_policy_row{KeyName: key_name}

	found := tool.QueryRow_DB(
		db,
		"select id, title, version, data, effective_date, status, date from baro_policy where key_name = ? and status = 'active' order by version + 0 desc limit 1",
		[]any{&row.ID, &row.Title, &row.Version, &row.Data, &row.EffectiveDate, &row.Status, &row.Date},
		key_name,
	)

	if !found {
		tool.QueryRow_DB(
			db,
			"select id, title, version, data, effective_date, status, date from baro_policy where key_name = ? order by version + 0 desc limit 1",
			[]any{&row.ID, &row.Title, &row.Version, &row.Data, &row.EffectiveDate, &row.Status, &row.Date},
			key_name,
		)
	}

	return row
}

// baro_legal_policy_version : 현재 정책 판 번호 (없으면 "0")
func baro_legal_policy_version(db *sql.DB, key_name string) string {
	row := baro_legal_policy_get(db, key_name)
	if row.Version == "" {
		return "0"
	}

	return row.Version
}

// baro_legal_policy_head : 정책 판 정보 줄
func baro_legal_policy_head(row baro_legal_policy_row) string {
	if row.ID == "" {
		return tool.Baro_alert("danger", tool.Baro_t("아직 이 정책의 본문이 등록되어 있지 않습니다. 운영자가 정책 화면에서 먼저 제정해야 합니다."))
	}

	version := row.Version
	if version == "" {
		version = "1"
	}

	effective := row.EffectiveDate
	if effective == "" {
		effective = tool.Baro_date_only(row.Date)
	}

	return tool.Baro_kv_grid([][]string{
		{tool.Baro_t("정책 제목"), tool.HTML_escape(row.Title)},
		{tool.Baro_t("현재 판"), tool.HTML_escape(version) + tool.Baro_t("판")},
		{"시행일", tool.HTML_escape(effective)},
		{"상태", baro_legal_status_badge(row.Status)},
	})
}

// ------------------------------------------------------------
//  설정 값
// ------------------------------------------------------------

// baro_legal_retention_days : 운영 데이터 보존 기간 (일)
func baro_legal_retention_days(db *sql.DB) int {
	days := tool.Baro_get_int(db, "log_retention_days", 90)
	if days < 1 {
		days = 90
	}

	return days
}

// baro_legal_reject_limit : 기각된 저작권 신고가 몇 건을 넘으면 새 접수를 막는가
func baro_legal_reject_limit(db *sql.DB) int {
	limit := tool.Baro_get_int(db, "copyright_reject_limit", 3)
	if limit < 1 {
		limit = 3
	}

	return limit
}

// baro_legal_rate_limit : 한 시간 안에 낼 수 있는 저작권 신고 수
func baro_legal_rate_limit(db *sql.DB) int {
	limit := tool.Baro_get_int(db, "copyright_rate_limit", 3)
	if limit < 1 {
		limit = 3
	}

	return limit
}

// ------------------------------------------------------------
//  개인정보 보호용 표시 도우미
// ------------------------------------------------------------

// baro_legal_who_cell : 요청자 표시 (로그인 사용자는 이름, 비로그인은 가린 IP)
func baro_legal_who_cell(db *sql.DB, who string, my_ip string) string {
	if strings.TrimSpace(who) == "" {
		return tool.Baro_t(`<span class="bw_muted">익명</span>`)
	}

	if tool.Baro_is_user(who) {
		return tool.Baro_user_link(db, who, my_ip)
	}

	return tool.Baro_t(`<span class="bw_muted" title="비로그인 접수">`) + tool.HTML_escape(tool.Baro_mask_ip(who)) + `</span>`
}

// baro_legal_contact_mask : 연락처 부분 가림 (목록에서는 원문을 보여주지 않는다)
func baro_legal_contact_mask(contact string) string {
	contact = strings.TrimSpace(contact)
	if contact == "" {
		return `<span class="bw_muted">-</span>`
	}

	at := strings.Index(contact, "@")
	if at > 1 {
		return tool.HTML_escape(tool.Get_slice(contact, 0, 2) + "***" + contact[at:])
	}

	if tool.Get_len(contact) > 4 {
		return tool.HTML_escape(tool.Get_slice(contact, 0, 3) + "****")
	}

	return tool.Baro_t(`<span class="bw_muted">비공개</span>`)
}

// baro_legal_target_cell : 신고 대상 표시 (문서면 링크)
func baro_legal_target_cell(target_kind string, target string) string {
	if strings.TrimSpace(target) == "" {
		return `<span class="bw_muted">-</span>`
	}

	if kind := baro_legal_target_kind_norm(target_kind); kind == "doc" || kind == "file" {
		return tool.Baro_doc_link(target)
	}

	return tool.HTML_escape(tool.Baro_trunc(target, 60))
}

// baro_legal_not_found : 대상이 없을 때 안내 (500 대신 안내 화면)
func baro_legal_not_found(db *sql.DB, config tool.Config, active string, text string, back_url string, back_text string) string {
	if text == "" {
		text = tool.Baro_t("요청하신 항목을 찾을 수 없습니다. 주소가 바뀌었거나 이미 정리된 기록일 수 있습니다.")
	}

	body := tool.Baro_alert("info", tool.HTML_escape(text))
	if back_url != "" {
		body += tool.Baro_card("", tool.Baro_button(back_url, back_text, "primary"))
	}

	return baro_legal_page(db, config, tool.Baro_t("찾을 수 없음"), active, body)
}
