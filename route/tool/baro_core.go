package tool

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ============================================================
//  바로위키 공통 계층
//
//  모든 바로위키 모듈은 이 파일의 함수만 사용해 DB / 권한 / 기록에 접근한다.
//  0장 고정 원칙이 여기에 코드로 박혀 있다.
//
//   1. 이용자 제재와 콘텐츠 판단을 분리 → Baro_sanction_* 은 문서를 건드리지 않는다
//   2. 삭제보다 보존과 복구 → Baro_quarantine_* (임시 격리) 가 삭제의 기본 경로
//   3. 대량 파괴 작업에는 안전장치 → Baro_mass_threshold / 2인 승인 / 스냅샷 / 유예
//   4. 기여 횟수보다 기여 가치 → Baro_contrib_add 는 행동 종류를 분류해 기록
//   5. 운영 근거는 공개하되 개인정보는 보호 → Baro_public_log / Baro_audit 분리
// ============================================================

const (
	// Baro_ver : 바로위키 확장 버전
	Baro_ver = "1.0"

	// Baro_page_size : 목록 기본 페이지 크기
	Baro_page_size = 30
)

// ------------------------------------------------------------
//  설정 (baro_set)
// ------------------------------------------------------------

// Baro_get : 바로위키 설정 값 읽기
func Baro_get(db *sql.DB, name string) string {
	data := ""
	QueryRow_DB(db, "select data from baro_set where name = ?", []any{&data}, name)

	return data
}

// Baro_get_def : 설정 값 읽기 (없으면 기본값)
func Baro_get_def(db *sql.DB, name string, def string) string {
	data := Baro_get(db, name)
	if strings.TrimSpace(data) == "" {
		return def
	}

	return data
}

// Baro_get_int : 설정 값을 정수로 읽기
func Baro_get_int(db *sql.DB, name string, def int) int {
	data := Baro_get(db, name)
	if strings.TrimSpace(data) == "" {
		return def
	}

	return Str_to_int(data)
}

// Baro_get_on : 설정이 켜져 있는지
func Baro_get_on(db *sql.DB, name string) bool {
	return Baro_get(db, name) == "on"
}

// Baro_put : 바로위키 설정 값 저장
func Baro_put(db *sql.DB, name string, data string) {
	Exec_DB(db, "delete from baro_set where name = ?", name)
	Exec_DB(db, "insert into baro_set (name, data) values (?, ?)", name, data)
}

// ------------------------------------------------------------
//  ID / 시간 / 문자열
// ------------------------------------------------------------

var baro_table_name_check = regexp.MustCompile(`^baro_[a-z_0-9]+$`)

// Baro_next_id : 테이블의 다음 순번 ID (문자열)
func Baro_next_id(db *sql.DB, table_name string) string {
	if !baro_table_name_check.MatchString(table_name) {
		panic("baro: invalid table name " + table_name)
	}

	last := "0"
	QueryRow_DB(
		db,
		fmt.Sprintf("select id from %s order by id + 0 desc limit 1", table_name),
		[]any{&last},
	)

	return strconv.Itoa(Str_to_int(last) + 1)
}

// Baro_now : 현재 시각 (2006-01-02 15:04:05)
func Baro_now() string {
	return Get_time()
}

// Baro_today : 오늘 날짜 (2006-01-02)
func Baro_today() string {
	return Get_date()
}

// Baro_date_only : 시각 문자열에서 날짜 부분만
func Baro_date_only(data string) string {
	if len(data) >= 10 {
		return data[:10]
	}

	return data
}

// Baro_parse_time : 시각 문자열 파싱 (실패 시 zero time)
func Baro_parse_time(data string) time.Time {
	data = strings.TrimSpace(data)
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02", "2006-01"} {
		if t, err := time.ParseInLocation(layout, data, time.Local); err == nil {
			return t
		}
	}

	return time.Time{}
}

// Baro_time_add : 시각에 일/시/분을 더한 문자열
func Baro_time_add(data string, days int, hours int, minutes int) string {
	base := Baro_parse_time(data)
	if base.IsZero() {
		base = time.Now()
	}

	return base.AddDate(0, 0, days).Add(time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute).Format("2006-01-02 15:04:05")
}

// Baro_days_between : 두 시각 사이의 일 수 (b - a)
func Baro_days_between(a string, b string) int {
	ta := Baro_parse_time(a)
	tb := Baro_parse_time(b)
	if ta.IsZero() || tb.IsZero() {
		return 0
	}

	return int(tb.Sub(ta).Hours() / 24)
}

// Baro_is_past : 지정한 시각이 이미 지났는지 (빈 문자열은 false = 무기한)
func Baro_is_past(data string) bool {
	if strings.TrimSpace(data) == "" {
		return false
	}

	t := Baro_parse_time(data)
	if t.IsZero() {
		return false
	}

	return time.Now().After(t)
}

// Baro_time_ago : "3일 전" 형태
func Baro_time_ago(data string) string {
	t := Baro_parse_time(data)
	if t.IsZero() {
		return data
	}

	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return Baro_t("방금 전")
	case diff < time.Hour:
		return strconv.Itoa(int(diff.Minutes())) + Baro_t("분 전")
	case diff < 24*time.Hour:
		return strconv.Itoa(int(diff.Hours())) + Baro_t("시간 전")
	case diff < 30*24*time.Hour:
		return strconv.Itoa(int(diff.Hours()/24)) + Baro_t("일 전")
	case diff < 365*24*time.Hour:
		return strconv.Itoa(int(diff.Hours()/24/30)) + Baro_t("개월 전")
	}

	return strconv.Itoa(int(diff.Hours()/24/365)) + Baro_t("년 전")
}

// Baro_duration_left : 남은 기간 문자열
func Baro_duration_left(end string) string {
	if strings.TrimSpace(end) == "" {
		return "무기한"
	}

	t := Baro_parse_time(end)
	if t.IsZero() {
		return end
	}

	diff := time.Until(t)
	if diff <= 0 {
		return Baro_t("만료됨")
	}

	switch {
	case diff < time.Hour:
		return strconv.Itoa(int(diff.Minutes())) + Baro_t("분 남음")
	case diff < 24*time.Hour:
		return strconv.Itoa(int(diff.Hours())) + Baro_t("시간 남음")
	}

	return strconv.Itoa(int(diff.Hours()/24)) + Baro_t("일 남음")
}

// Baro_itoa : 정수를 문자열로
func Baro_itoa(num int) string {
	return strconv.Itoa(num)
}

// Baro_comma : 천 단위 구분 기호
func Baro_comma(num int) string {
	minus := ""
	if num < 0 {
		minus = "-"
		num = -num
	}

	text := strconv.Itoa(num)
	out := ""
	for i, ch := range text {
		if i > 0 && (len(text)-i)%3 == 0 {
			out += ","
		}
		out += string(ch)
	}

	return minus + out
}

// Baro_pct : 백분율 (소수 첫째 자리)
func Baro_pct(done int, total int) float64 {
	if total <= 0 {
		return 0
	}

	value := float64(done) * 100 / float64(total)
	if value > 100 {
		value = 100
	}

	return float64(int(value*10+0.5)) / 10
}

// Baro_pct_text : 백분율 문자열
func Baro_pct_text(done int, total int) string {
	return strconv.FormatFloat(Baro_pct(done, total), 'f', 1, 64) + "%"
}

// Baro_trunc : 글자 수 제한 (넘으면 말줄임)
func Baro_trunc(data string, max int) string {
	if Get_len(data) <= max {
		return data
	}

	return Get_slice(data, 0, max) + "…"
}

// Baro_one_line : 줄바꿈 제거
func Baro_one_line(data string) string {
	data = strings.ReplaceAll(data, "\r", "")
	data = strings.ReplaceAll(data, "\n", " ")

	return strings.TrimSpace(data)
}

var baro_chosung_table = []string{
	"ㄱ", "ㄲ", "ㄴ", "ㄷ", "ㄸ", "ㄹ", "ㅁ", "ㅂ", "ㅃ", "ㅅ",
	"ㅆ", "ㅇ", "ㅈ", "ㅉ", "ㅊ", "ㅋ", "ㅌ", "ㅍ", "ㅎ",
}

// Baro_chosung : 한글 문자열의 초성 추출 (초성 검색용)
func Baro_chosung(data string) string {
	out := strings.Builder{}
	for _, ch := range data {
		if ch >= 0xAC00 && ch <= 0xD7A3 {
			index := (int(ch) - 0xAC00) / 588
			if index >= 0 && index < len(baro_chosung_table) {
				out.WriteString(baro_chosung_table[index])
				continue
			}
		}

		out.WriteRune(ch)
	}

	return out.String()
}

// Baro_is_chosung_only : 초성만으로 이루어진 검색어인지
func Baro_is_chosung_only(data string) bool {
	data = strings.TrimSpace(data)
	if data == "" {
		return false
	}

	for _, ch := range data {
		if ch == ' ' {
			continue
		}

		if !(ch >= 0x3131 && ch <= 0x314E) {
			return false
		}
	}

	return true
}

// Baro_norm : 검색 정규화 (공백 제거 + 소문자)
func Baro_norm(data string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(data), " ", ""))
}

// Baro_split_lines : 여러 줄 입력을 비어 있지 않은 줄 목록으로
func Baro_split_lines(data string) []string {
	out := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(data, "\r", ""), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}

	return out
}

// ------------------------------------------------------------
//  JSON
// ------------------------------------------------------------

// Baro_json_enc : 값을 JSON 문자열로
func Baro_json_enc(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}

	return string(raw)
}

// Baro_json_dec : JSON 문자열을 map 으로 (실패 시 빈 map)
func Baro_json_dec(data string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(data) == "" {
		return out
	}

	if err := json.Unmarshal([]byte(data), &out); err != nil {
		return map[string]any{}
	}

	return out
}

// Baro_json_dec_str : JSON 문자열을 map[string]string 으로
func Baro_json_dec_str(data string) map[string]string {
	out := map[string]string{}
	for key, value := range Baro_json_dec(data) {
		switch v := value.(type) {
		case string:
			out[key] = v
		case float64:
			out[key] = strconv.Itoa(int(v))
		case bool:
			if v {
				out[key] = "on"
			} else {
				out[key] = ""
			}
		}
	}

	return out
}

// Baro_json_dec_list : JSON 배열 문자열을 문자열 목록으로
func Baro_json_dec_list(data string) []string {
	out := []string{}
	if strings.TrimSpace(data) == "" {
		return out
	}

	raw := []any{}
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return out
	}

	for _, value := range raw {
		switch v := value.(type) {
		case string:
			out = append(out, v)
		case float64:
			out = append(out, strconv.Itoa(int(v)))
		}
	}

	return out
}

// ------------------------------------------------------------
//  사용자 / 권한
// ------------------------------------------------------------

// Baro_is_user : 로그인 사용자인지 (IP 가 아닌지)
func Baro_is_user(ip string) bool {
	return !IP_or_user(ip)
}

// Baro_is_admin : 관리자 권한 보유 여부
func Baro_is_admin(db *sql.DB, ip string) bool {
	return Check_acl(db, "", "", "all_admin_auth", ip)
}

// Baro_is_owner : 소유자 여부
func Baro_is_owner(db *sql.DB, ip string) bool {
	return Get_user_auth(db, ip) == "owner"
}

// Baro_role_list : 운영자 역할 분리 (11-3)
//
//	역할 하나가 신고 접수부터 판결·재심까지 전부 처리하지 못하도록 나눈다.
func Baro_role_list() [][]string {
	return [][]string{
		{"reviewer", Baro_t("문서 검토자"), Baro_t("편집 검토 큐를 처리한다")},
		{"patrol", Baro_t("순찰자"), Baro_t("최근 변경을 감시하고 신고를 접수한다")},
		{"mediator", Baro_t("중재자"), Baro_t("토론 분쟁을 중재한다")},
		{"sanction", Baro_t("제재 담당"), Baro_t("사건에 대한 제재를 집행한다")},
		{"appeal", Baro_t("이의제기 담당"), Baro_t("이의제기를 심사한다 (원조치 담당자는 배제)")},
		{"sysadmin", Baro_t("시스템 관리자"), Baro_t("시스템 설정과 기능 플래그를 관리한다")},
		{"privacy", Baro_t("개인정보·법적 요청 담당"), Baro_t("개인정보와 저작권 요청을 처리한다")},
		{"project", Baro_t("프로젝트 운영자"), Baro_t("프로젝트를 운영한다 (제재 권한 없음)")},
		{"founding", Baro_t("창립 시즌 운영자"), Baro_t("창립단 심사와 번호 발급을 담당한다")},
		{"award", Baro_t("어워드 운영자"), Baro_t("어워드 후보 검증과 발표를 담당한다")},
	}
}

// Baro_role_name : 역할 표시 이름
func Baro_role_name(role string) string {
	for _, row := range Baro_role_list() {
		if row[0] == role {
			return row[1]
		}
	}

	return role
}

// Baro_user_roles : 사용자가 가진 역할 목록
func Baro_user_roles(db *sql.DB, user string) []string {
	out := []string{}
	if !Baro_is_user(user) {
		return out
	}

	rows := Query_DB(db, "select role from baro_role where user = ?", user)
	defer rows.Close()

	for rows.Next() {
		role := ""
		if err := rows.Scan(&role); err != nil {
			continue
		}

		out = append(out, role)
	}

	return out
}

// Baro_has_role : 해당 역할 보유 여부 (소유자는 모든 역할을 가진다)
func Baro_has_role(db *sql.DB, user string, role string) bool {
	if !Baro_is_user(user) {
		return false
	}

	if Baro_is_owner(db, user) {
		return true
	}

	found := ""
	QueryRow_DB(db, "select role from baro_role where user = ? and role = ?", []any{&found}, user, role)

	return found != ""
}

// Baro_grant_role : 역할 부여
func Baro_grant_role(db *sql.DB, user string, role string, by string, memo string) {
	if Baro_has_role(db, user, role) {
		return
	}

	Exec_DB(
		db,
		"insert into baro_role (user, role, granted_by, date, memo) values (?, ?, ?, ?, ?)",
		user, role, by, Baro_now(), memo,
	)
	Baro_public_log(db, "role_grant", by, user, memo, Baro_role_name(role))
}

// Baro_revoke_role : 역할 회수
func Baro_revoke_role(db *sql.DB, user string, role string, by string, memo string) {
	Exec_DB(db, "delete from baro_role where user = ? and role = ?", user, role)
	Baro_public_log(db, "role_revoke", by, user, memo, Baro_role_name(role))
}

// Baro_can : 바로위키 기능 권한 확인
//
//	역할 기반 권한을 우선 보고, 없으면 openNAMU 관리자 권한으로 넘어간다.
func Baro_can(db *sql.DB, ip string, capability string) bool {
	if !Baro_is_user(ip) {
		return false
	}

	if Baro_is_owner(db, ip) {
		return true
	}

	role_map := map[string][]string{
		"review":       {"reviewer", "patrol"},
		"patrol":       {"patrol", "reviewer"},
		"report":       {"patrol", "mediator", "sanction"},
		"case":         {"sanction", "mediator", "patrol"},
		"sanction":     {"sanction"},
		"appeal":       {"appeal"},
		"mass":         {"sanction", "sysadmin"},
		"mass_approve": {"sanction", "sysadmin", "appeal"},
		"policy":       {"sysadmin", "mediator"},
		"system":       {"sysadmin"},
		"privacy":      {"privacy"},
		"copyright":    {"privacy"},
		"project":      {"project"},
		"founding":     {"founding"},
		"award":        {"award"},
		"import":       {"sysadmin"},
		"quarantine":   {"patrol", "sanction", "privacy"},
	}

	for _, role := range role_map[capability] {
		if Baro_has_role(db, ip, role) {
			return true
		}
	}

	// 프로젝트 / 창립 / 어워드 운영은 콘텐츠 역할이므로 제재 권한과 분리된다.
	// 나머지는 위키 관리자 권한으로 대체할 수 있다.
	switch capability {
	case "sanction", "appeal", "privacy", "copyright":
		return false
	}

	return Baro_is_admin(db, ip)
}

// Baro_deny : 권한 없음 안내 페이지
func Baro_deny(db *sql.DB, config Config, message string) string {
	if message == "" {
		message = Baro_t("이 기능을 사용할 권한이 없습니다.")
	}

	return Baro_page(db, config, "권한 없음", Baro_alert("danger", Baro_t_esc(message)), [][]any{{"baro", "바로위키"}})
}

// Baro_login_wall : 로그인 필요 안내 페이지 (필요하면 "", false)
func Baro_login_wall(db *sql.DB, config Config, next string) (string, bool) {
	if Baro_is_user(config.IP) {
		return "", false
	}

	body := Baro_alert("info", Baro_t("바로위키의 기여 기능은 로그인한 사용자만 사용할 수 있습니다."))
	body += Baro_card(Baro_t("로그인이 필요합니다"),
		Baro_t(`<p>바로위키는 기여 이력을 사람 단위로 남기기 위해 로그인 사용자만 편집·기여할 수 있습니다.</p>`)+
			Baro_button("/login", Baro_t("로그인"), "primary")+" "+
			Baro_button("/register", Baro_t("가입하기"), "")+" "+
			Baro_button("/baro/founding", Baro_t("창립 시즌 보기"), ""),
	)

	_ = next

	return Baro_page(db, config, Baro_t("로그인 필요"), body, [][]any{{"baro", "바로위키"}}), true
}

// ------------------------------------------------------------
//  기록 (5번 원칙 : 근거는 공개, 개인정보는 보호)
// ------------------------------------------------------------

// Baro_public_log : 공개 운영 로그 (누구나 볼 수 있음 / 민감 정보 금지)
func Baro_public_log(db *sql.DB, kind string, actor string, target string, reason string, detail string) {
	Exec_DB(
		db,
		"insert into baro_public_log (id, kind, actor, target, reason, detail, date) values (?, ?, ?, ?, ?, ?, ?)",
		Baro_next_id(db, "baro_public_log"), kind, Baro_mask_ip(actor), target, reason, detail, Baro_now(),
	)
}

// Baro_audit : 내부 감사 로그 (운영자도 수정할 수 없는 상세 기록)
func Baro_audit(db *sql.DB, config Config, action string, target string, before string, after string, reason string) {
	id := Baro_next_id(db, "baro_audit_log")
	auth := ""
	if config.IP != "" {
		auth = Get_user_auth(db, config.IP)
	}

	before_data := Baro_trunc(before, 2000)
	after_data := Baro_trunc(after, 2000)
	user_agent := Baro_trunc(config.UserAgent, 300)
	now := Baro_now()

	sign := Baro_audit_sign(id, config.IP, action, target, before_data, after_data,
		auth, reason, config.IP, user_agent, now)

	Exec_DB(
		db,
		"insert into baro_audit_log (id, actor, action, target, before_data, after_data, auth, reason, ip, ua, date, sign_data) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, config.IP, action, target, before_data, after_data,
		auth, reason, config.IP, user_agent, now, sign,
	)
}

// Baro_mask_ip : 공개 기록에 남길 때 IP 를 부분 가림 처리
func Baro_mask_ip(who string) string {
	if Baro_is_user(who) {
		return who
	}

	if strings.Contains(who, ":") {
		parts := strings.Split(who, ":")
		if len(parts) > 2 {
			return parts[0] + ":" + parts[1] + ":*"
		}

		return "*"
	}

	parts := strings.Split(who, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".*.*"
	}

	return who
}

// ------------------------------------------------------------
//  분야 (6장)
// ------------------------------------------------------------

// Baro_field_row : 분야 정보
type Baro_field_row struct {
	Key         string
	Name        string
	Description string
	Sort        int
	TargetCount int
	Recruit     int
	RecruitOpen bool
	Kind        string // "closed" : 분모가 확정된 분야 (개척률 표시 가능) / "open" : 목표 수만 표시
}

// Baro_field_default : 기본 분야 목록
func Baro_field_default() []Baro_field_row {
	return []Baro_field_row{
		{"history", "역사", "한국사 전반의 인물·사건·제도", 1, 0, 0, true, "open"},
		{"independence", "독립운동", "국가 공인 독립유공자 기록", 2, 18000, 5, true, "closed"},
		{"local", "지역사", "시·군·읍면동 단위의 지역 기록", 3, 0, 3, true, "open"},
		{"media", "언론·방송", "언론사·언론인·방송 프로그램", 4, 0, 2, true, "open"},
		{"baseball", "야구", "한국 야구의 선수·구단·기록", 5, 0, 3, true, "open"},
		{"rail", "철도", "노선·역·차량", 6, 0, 2, true, "open"},
		{"game", "게임", "게임·개발사·인물", 7, 0, 0, true, "open"},
		{"anime", "애니", "애니메이션·제작사·인물", 8, 0, 0, true, "open"},
		{"science", "과학", "과학 개념·연구·인물", 9, 0, 1, true, "open"},
		{"person", "인물", "분야로 나누기 어려운 인물 기록", 10, 0, 0, true, "open"},
		{"etc", "기타", "위 분류에 들어가지 않는 주제", 99, 0, 0, true, "open"},
	}
}

// Baro_field_list : 등록된 분야 목록
func Baro_field_list(db *sql.DB) []Baro_field_row {
	out := []Baro_field_row{}

	rows := Query_DB(db, "select key_name, name, description, sort, target_count, recruit_count, recruit_open, kind from baro_field")
	defer rows.Close()

	for rows.Next() {
		key_name, name, description, sort_data, target_count, recruit_count, recruit_open, kind := "", "", "", "", "", "", "", ""
		if err := rows.Scan(&key_name, &name, &description, &sort_data, &target_count, &recruit_count, &recruit_open, &kind); err != nil {
			continue
		}

		out = append(out, Baro_field_row{
			Key:         key_name,
			Name:        name,
			Description: description,
			Sort:        Str_to_int(sort_data),
			TargetCount: Str_to_int(target_count),
			Recruit:     Str_to_int(recruit_count),
			RecruitOpen: recruit_open == "on",
			Kind:        kind,
		})
	}

	sort.SliceStable(out, func(i int, j int) bool {
		return out[i].Sort < out[j].Sort
	})

	return out
}

// Baro_field_get : 분야 하나
func Baro_field_get(db *sql.DB, key_name string) Baro_field_row {
	for _, row := range Baro_field_list(db) {
		if row.Key == key_name {
			return row
		}
	}

	return Baro_field_row{}
}

// Baro_field_name : 분야 표시 이름 (없으면 키 그대로)
func Baro_field_name(db *sql.DB, key_name string) string {
	if key_name == "" {
		return Baro_t("미지정")
	}

	row := Baro_field_get(db, key_name)
	if row.Name == "" {
		return key_name
	}

	return row.Name
}

// Baro_field_save : 분야 저장 (없으면 추가)
func Baro_field_save(db *sql.DB, row Baro_field_row) {
	recruit_open := ""
	if row.RecruitOpen {
		recruit_open = "on"
	}

	Exec_DB(db, "delete from baro_field where key_name = ?", row.Key)
	Exec_DB(
		db,
		"insert into baro_field (key_name, name, description, sort, target_count, recruit_count, recruit_open, kind, date) values (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		row.Key, row.Name, row.Description, Baro_itoa(row.Sort),
		Baro_itoa(row.TargetCount), Baro_itoa(row.Recruit), recruit_open, row.Kind, Baro_now(),
	)
}

// Baro_user_interest : 사용자의 관심 분야
func Baro_user_interest(db *sql.DB, user string) []string {
	out := []string{}

	rows := Query_DB(db, "select field from baro_interest where user = ?", user)
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

// ------------------------------------------------------------
//  알림 (2-6)
// ------------------------------------------------------------

// Baro_notify_kind_list : 알림 종류
func Baro_notify_kind_list() [][]string {
	return [][]string{
		{"revert", Baro_t("내 편집이 되돌려짐")},
		{"improve", Baro_t("내 문서가 보강됨")},
		{"reply", Baro_t("토론 답글")},
		{"mention", Baro_t("사용자 언급")},
		{"thanks", Baro_t("감사 수신")},
		{"watch", Baro_t("관심 문서 변경")},
		{"user_doc", Baro_t("내 사용자 문서 변경")},
		{"project", Baro_t("프로젝트 목표 달성")},
		{"case", Baro_t("신고·이의제기 결과")},
		{"award", Baro_t("어워드 후보 등록")},
		{"badge", Baro_t("배지 획득")},
		{"review", Baro_t("검토 결과")},
		{"system", Baro_t("시스템 안내")},
	}
}

// Baro_notify : 알림 보내기 (사용자 설정에 따라 즉시 / 요약 / 끄기)
func Baro_notify(db *sql.DB, user string, kind string, title string, url string, data string) {
	if !Baro_is_user(user) {
		return
	}

	mode := "on"
	QueryRow_DB(db, "select mode from baro_notify_set where user = ? and kind = ?", []any{&mode}, user, kind)
	if mode == "off" {
		return
	}

	Exec_DB(
		db,
		"insert into baro_notify (id, user, kind, title, url, data, readme, date) values (?, ?, ?, ?, ?, ?, '', ?)",
		Baro_next_id(db, "baro_notify"), user, kind, Baro_trunc(title, 300), url, data, Baro_now(),
	)
}

// Baro_notify_count : 읽지 않은 알림 수
func Baro_notify_count(db *sql.DB, user string) int {
	if !Baro_is_user(user) {
		return 0
	}

	count := 0
	QueryRow_DB(db, "select count(*) from baro_notify where user = ? and readme = ''", []any{&count}, user)

	return count
}

// ------------------------------------------------------------
//  기여 이벤트 (7-1) — 횟수가 아니라 행동 종류를 기록한다
// ------------------------------------------------------------

// Baro_contrib_kind_list : 기여 행동 분류 (키, 이름, 기본 가중치)
func Baro_contrib_kind_list() [][]string {
	return [][]string{
		{"new_document", Baro_t("새 문서 작성"), "100"},
		{"expand", Baro_t("본문 보강"), "30"},
		{"source", Baro_t("출처 추가"), "40"},
		{"fix", Baro_t("오류 수정"), "15"},
		{"category", Baro_t("분류 정리"), "8"},
		{"file", Baro_t("파일 추가"), "25"},
		{"discussion", Baro_t("토론 해결"), "20"},
		{"review", Baro_t("문서 검토"), "25"},
		{"request_done", Baro_t("요청 문서 완성"), "60"},
		{"project", Baro_t("프로젝트 기여"), "35"},
		{"typo", Baro_t("오타 수정"), "3"},
		{"import", Baro_t("문서 이전"), "5"},
	}
}

// Baro_contrib_kind_name : 기여 종류 표시 이름
func Baro_contrib_kind_name(kind string) string {
	for _, row := range Baro_contrib_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// Baro_contrib_kind_weight : 기여 종류 기본 가중치
func Baro_contrib_kind_weight(kind string) int {
	for _, row := range Baro_contrib_kind_list() {
		if row[0] == kind {
			return Str_to_int(row[2])
		}
	}

	return 5
}

// Baro_contrib_add : 기여 이벤트 기록 후 이벤트 ID 반환
//
//	점수는 여기서 확정하지 않는다. 7~14일 뒤 Baro_score_confirm 이 확정한다. (7-2)
func Baro_contrib_add(db *sql.DB, user string, kind string, title string, rev string, project string, detail string) string {
	if !Baro_is_user(user) {
		return ""
	}

	id := Baro_next_id(db, "baro_contrib_event")
	field := Baro_doc_field(db, title)

	Exec_DB(
		db,
		"insert into baro_contrib_event (id, user, kind, title, rev, project, weight, detail, date, status, field) values (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)",
		id, user, kind, title, rev, project,
		Baro_itoa(Baro_contrib_kind_weight(kind)), Baro_trunc(detail, 500), Baro_now(), field,
	)

	Baro_calendar_bump(db, user, kind)

	return id
}

// Baro_calendar_bump : 활동 캘린더 갱신 (7-8 — 스트릭 압박 없이 기록만)
func Baro_calendar_bump(db *sql.DB, user string, kind string) {
	today := Baro_today()

	num, detail := "0", ""
	found := QueryRow_DB(db, "select num, detail from baro_calendar where user = ? and date = ?", []any{&num, &detail}, user, today)

	kinds := Baro_json_dec_str(detail)
	kinds[kind] = Baro_itoa(Str_to_int(kinds[kind]) + 1)

	if found {
		Exec_DB(
			db,
			"update baro_calendar set num = ?, detail = ? where user = ? and date = ?",
			Baro_itoa(Str_to_int(num)+1), Baro_json_enc(kinds), user, today,
		)

		return
	}

	Exec_DB(
		db,
		"insert into baro_calendar (user, date, num, detail) values (?, ?, '1', ?)",
		user, today, Baro_json_enc(kinds),
	)
}

// Baro_impact_get : 영향력 지표 읽기
func Baro_impact_get(db *sql.DB, user string, name string) int {
	data := "0"
	QueryRow_DB(db, "select data from baro_impact where user = ? and name = ?", []any{&data}, user, name)

	return Str_to_int(data)
}

// Baro_impact_set : 영향력 지표 저장
func Baro_impact_set(db *sql.DB, user string, name string, value int) {
	Exec_DB(db, "delete from baro_impact where user = ? and name = ?", user, name)
	Exec_DB(
		db,
		"insert into baro_impact (user, name, data, date) values (?, ?, ?, ?)",
		user, name, Baro_itoa(value), Baro_now(),
	)
}

// Baro_impact_bump : 영향력 지표 증가
func Baro_impact_bump(db *sql.DB, user string, name string, delta int) {
	Baro_impact_set(db, user, name, Baro_impact_get(db, user, name)+delta)
}

// ------------------------------------------------------------
//  문서 부가 정보
// ------------------------------------------------------------

// Baro_doc_meta_get : 문서 메타 값
func Baro_doc_meta_get(db *sql.DB, title string, name string) string {
	data := ""
	QueryRow_DB(db, "select data from baro_doc_meta where title = ? and name = ?", []any{&data}, title, name)

	return data
}

// Baro_doc_meta_set : 문서 메타 값 저장
func Baro_doc_meta_set(db *sql.DB, title string, name string, data string) {
	Exec_DB(db, "delete from baro_doc_meta where title = ? and name = ?", title, name)
	Exec_DB(db, "insert into baro_doc_meta (title, name, data) values (?, ?, ?)", title, name, data)
}

// Baro_doc_field : 문서가 속한 분야 (없으면 "")
func Baro_doc_field(db *sql.DB, title string) string {
	if title == "" {
		return ""
	}

	return Baro_doc_meta_get(db, title, "field")
}

// Baro_doc_status_list : 문서 상태 (9-1)
func Baro_doc_status_list() [][]string {
	return [][]string{
		{"draft", "초안", "info"},
		{"need_review", Baro_t("검토 필요"), "warn"},
		{"need_source", Baro_t("출처 필요"), "warn"},
		{"need_expand", Baro_t("내용 보강 필요"), "warn"},
		{"disputed", Baro_t("논쟁 중"), "danger"},
		{"reviewed", Baro_t("검토 완료"), "ok"},
		{"featured", Baro_t("우수 문서"), "ok"},
		{"quarantined", Baro_t("격리 중"), "danger"},
		{"archived", Baro_t("보존 문서"), "muted"},
	}
}

// Baro_doc_status_name : 문서 상태 이름
func Baro_doc_status_name(status string) string {
	for _, row := range Baro_doc_status_list() {
		if row[0] == status {
			return row[1]
		}
	}

	return status
}

// Baro_doc_status_kind : 문서 상태 색상 종류
func Baro_doc_status_kind(status string) string {
	for _, row := range Baro_doc_status_list() {
		if row[0] == status {
			return row[2]
		}
	}

	return "muted"
}

// Baro_doc_status_get : 문서 상태 읽기
func Baro_doc_status_get(db *sql.DB, title string) string {
	status := ""
	QueryRow_DB(db, "select status from baro_doc_status where title = ?", []any{&status}, title)

	return status
}

// Baro_doc_status_set : 문서 상태 저장
func Baro_doc_status_set(db *sql.DB, title string, status string, reason string, actor string) {
	Exec_DB(db, "delete from baro_doc_status where title = ?", title)
	if status == "" {
		return
	}

	Exec_DB(
		db,
		"insert into baro_doc_status (title, status, reason, actor, date) values (?, ?, ?, ?, ?)",
		title, status, reason, actor, Baro_now(),
	)
}

// Baro_doc_quarantined : 문서가 격리 중인지 (2번 원칙의 핵심 상태)
func Baro_doc_quarantined(db *sql.DB, title string) bool {
	status := ""
	QueryRow_DB(
		db,
		"select status from baro_quarantine where title = ? and status = 'open' limit 1",
		[]any{&status},
		title,
	)

	return status != ""
}

// ------------------------------------------------------------
//  제재 (10-3) — 문서가 아니라 사람에게만 적용된다 (1번 원칙)
// ------------------------------------------------------------

// Baro_sanction_kind_list : 제재 종류 (약한 것부터)
func Baro_sanction_kind_list() [][]string {
	return [][]string{
		{"warn", Baro_t("경고")},
		{"slow", Baro_t("편집 속도 제한")},
		{"doc_limit", Baro_t("특정 문서 제한")},
		{"field_limit", Baro_t("특정 분야 제한")},
		{"discussion_limit", Baro_t("토론 제한")},
		{"upload_limit", Baro_t("파일 업로드 제한")},
		{"short_block", Baro_t("단기 차단")},
		{"long_block", Baro_t("장기 차단")},
		{"perm_block", Baro_t("영구 차단")},
	}
}

// Baro_sanction_name : 제재 종류 이름
func Baro_sanction_name(kind string) string {
	for _, row := range Baro_sanction_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// Baro_sanction_active : 사용자에게 적용 중인 제재 목록 (만료된 것은 자동 제외)
func Baro_sanction_active(db *sql.DB, user string) [][]string {
	out := [][]string{}
	if !Baro_is_user(user) {
		return out
	}

	rows := Query_DB(
		db,
		"select id, kind, scope, reason, end_date, case_id from baro_sanction where user = ? and status = 'active'",
		user,
	)
	defer rows.Close()

	expired := []string{}
	for rows.Next() {
		id, kind, scope, reason, end_date, case_id := "", "", "", "", "", ""
		if err := rows.Scan(&id, &kind, &scope, &reason, &end_date, &case_id); err != nil {
			continue
		}

		if Baro_is_past(end_date) {
			expired = append(expired, id)
			continue
		}

		out = append(out, []string{id, kind, scope, reason, end_date, case_id})
	}

	for _, id := range expired {
		Exec_DB(db, "update baro_sanction set status = 'expired' where id = ?", id)
	}

	return out
}

// Baro_sanction_has : 특정 제재가 적용 중인지 (scope 가 비어 있으면 종류만 확인)
func Baro_sanction_has(db *sql.DB, user string, kind string, scope string) bool {
	for _, row := range Baro_sanction_active(db, user) {
		if row[1] != kind {
			continue
		}

		if scope == "" || row[2] == "" || row[2] == scope {
			return true
		}
	}

	return false
}

// ------------------------------------------------------------
//  대량 작업 안전장치 (10-6) — 바로위키의 시그니처
// ------------------------------------------------------------

// Baro_mass_threshold : 이 수를 넘는 대량 작업은 승인 절차를 거쳐야 한다
func Baro_mass_threshold(db *sql.DB) int {
	return Baro_get_int(db, "mass_threshold", 10)
}

// Baro_mass_grace_hours : 실행까지의 기본 유예 시간
func Baro_mass_grace_hours(db *sql.DB) int {
	return Baro_get_int(db, "mass_grace_hours", 24)
}

// Baro_mass_kind_list : 대량 작업 종류
func Baro_mass_kind_list() [][]string {
	return [][]string{
		{"delete", Baro_t("대량 삭제")},
		{"quarantine", Baro_t("대량 격리")},
		{"revert", Baro_t("대량 되돌리기")},
		{"block", Baro_t("대량 차단")},
		{"protect", Baro_t("대량 보호")},
		{"category", Baro_t("대량 분류 변경")},
	}
}

// Baro_mass_kind_name : 대량 작업 종류 이름
func Baro_mass_kind_name(kind string) string {
	for _, row := range Baro_mass_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// ------------------------------------------------------------
//  기능 플래그 (16-8)
// ------------------------------------------------------------

// Baro_flag_on : 기능 플래그가 이 사용자에게 켜져 있는지
//
//	scope : all / admin / founder / percent:NN / project:ID / off
func Baro_flag_on(db *sql.DB, user string, key_name string) bool {
	scope, value := "", ""
	if !QueryRow_DB(db, "select scope, value from baro_flag where key_name = ?", []any{&scope, &value}, key_name) {
		return true
	}

	switch scope {
	case "off":
		return false
	case "all", "":
		return true
	case "admin":
		return Baro_is_admin(db, user)
	case "founder":
		return Baro_founder_number(db, user) != ""
	}

	if strings.HasPrefix(scope, "percent:") {
		percent := Str_to_int(strings.TrimPrefix(scope, "percent:"))
		if percent <= 0 {
			return false
		}
		if percent >= 100 {
			return true
		}

		bucket := 0
		for _, ch := range Sha224(user + key_name) {
			bucket = (bucket*31 + int(ch)) % 100
		}

		return bucket < percent
	}

	if strings.HasPrefix(scope, "project:") {
		project := strings.TrimPrefix(scope, "project:")
		found := ""
		QueryRow_DB(
			db,
			"select user from baro_project_member where project = ? and user = ? and status = 'active'",
			[]any{&found}, project, user,
		)

		return found != ""
	}

	return true
}

// Baro_founder_number : 창립 기여자 번호 (없으면 "")
func Baro_founder_number(db *sql.DB, user string) string {
	if !Baro_is_user(user) {
		return ""
	}

	number := ""
	QueryRow_DB(
		db,
		"select number from baro_founder where user = ? and status = 'confirmed'",
		[]any{&number}, user,
	)

	return number
}

// ------------------------------------------------------------
//  요청 제한 (17-2)
// ------------------------------------------------------------

// Baro_rate_ok : 시간 창(window_sec) 안에서 limit 회까지 허용
func Baro_rate_ok(db *sql.DB, key_name string, who string, limit int, window_sec int) bool {
	now := time.Now()

	num, window_start := "0", ""
	found := QueryRow_DB(
		db,
		"select num, window_start from baro_rate where key_name = ? and who = ?",
		[]any{&num, &window_start}, key_name, who,
	)

	start := Baro_parse_time(window_start)
	if !found || start.IsZero() || now.Sub(start) > time.Duration(window_sec)*time.Second {
		Exec_DB(db, "delete from baro_rate where key_name = ? and who = ?", key_name, who)
		Exec_DB(
			db,
			"insert into baro_rate (key_name, who, num, window_start) values (?, ?, '1', ?)",
			key_name, who, now.Format("2006-01-02 15:04:05"),
		)

		return true
	}

	count := Str_to_int(num)
	if count >= limit {
		return false
	}

	Exec_DB(
		db,
		"update baro_rate set num = ? where key_name = ? and who = ?",
		Baro_itoa(count+1), key_name, who,
	)

	return true
}

// ------------------------------------------------------------
//  지표 (18장)
// ------------------------------------------------------------

// Baro_metric_bump : 지표 증가 (period 는 날짜 또는 월)
func Baro_metric_bump(db *sql.DB, key_name string, period string, delta int) {
	if period == "" {
		period = Baro_today()
	}

	value := "0"
	found := QueryRow_DB(
		db,
		"select value from baro_metric where key_name = ? and period = ?",
		[]any{&value}, key_name, period,
	)

	if found {
		Exec_DB(
			db,
			"update baro_metric set value = ?, date = ? where key_name = ? and period = ?",
			Baro_itoa(Str_to_int(value)+delta), Baro_now(), key_name, period,
		)

		return
	}

	Exec_DB(
		db,
		"insert into baro_metric (id, key_name, period, value, detail, date) values (?, ?, ?, ?, '', ?)",
		Baro_next_id(db, "baro_metric"), key_name, period, Baro_itoa(delta), Baro_now(),
	)
}

// Baro_metric_get : 지표 값
func Baro_metric_get(db *sql.DB, key_name string, period string) int {
	value := "0"
	QueryRow_DB(db, "select value from baro_metric where key_name = ? and period = ?", []any{&value}, key_name, period)

	return Str_to_int(value)
}

// ------------------------------------------------------------
//  읽기 전용 모드 (17-4)
// ------------------------------------------------------------

// Baro_read_only : 위키가 읽기 전용인지
func Baro_read_only(db *sql.DB) bool {
	return Baro_get(db, "read_only") == "on"
}

// Baro_read_only_reason : 읽기 전용 사유
func Baro_read_only_reason(db *sql.DB) string {
	return Baro_get_def(db, "read_only_reason", "점검 중입니다.")
}
