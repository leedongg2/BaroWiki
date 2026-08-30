package route

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 17장 : 보안 · 안정성 · 성능 · 접근성 (sec 모듈)
//
//  이 파일은 sec 모듈 전체가 함께 쓰는 공통 도구만 담는다.
//  화면은 view_baro_sec_*.go 에 나뉘어 있다.
// ============================================================

// baro_sec_boot : 이 프로세스가 시작한 시각 (가동 시간 계산용)
var baro_sec_boot = time.Now()

// ------------------------------------------------------------
//  공통 화면 뼈대
// ------------------------------------------------------------

// baro_sec_menu : 스킨 메뉴 줄
func baro_sec_menu() [][]any {
	return [][]any{
		{"baro", "바로위키"},
		{"baro/security", tool.Baro_t("보안")},
		{"baro/status", "상태"},
	}
}

// baro_sec_tabs : 보안 화면 공통 탭
func baro_sec_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/security", tool.Baro_t("계정 보안")},
		{"/baro/security/2fa", tool.Baro_t("2단계 인증")},
		{"/baro/security/login_log", tool.Baro_t("접속 기록")},
		{"/baro/security/session", tool.Baro_t("세션")},
		{"/baro/security/abuse", tool.Baro_t("악용 방지")},
		{"/baro/security/audit", tool.Baro_t("감사 로그")},
		{"/baro/security/perf", tool.Baro_t("성능")},
		{"/baro/security/a11y", tool.Baro_t("접근성")},
		{"/baro/status", tool.Baro_t("상태 페이지")},
	}, active)
}

// baro_sec_banner : 모든 sec 화면 맨 위에 붙는 안내 (읽기 전용 · 긴급 편집 중단)
func baro_sec_banner(db *sql.DB) string {
	html := ""

	if tool.Baro_read_only(db) {
		html += tool.Baro_alert("danger",
			tool.Baro_t(`<b>지금 위키는 읽기 전용입니다.</b> 문서 편집·업로드·토론 작성을 중단한 상태입니다.<br>`)+
				tool.Baro_t(`사유 : `)+tool.HTML_escape(tool.Baro_read_only_reason(db))+`<br>`+
				tool.Baro_t(`<a href="/baro/status">상태 페이지에서 자세히 보기</a>`),
		)
	}

	if baro_sec_flag(db, "sec_emergency_stop", false) {
		html += tool.Baro_alert("danger",
			tool.Baro_t(`<b>긴급 편집 중단이 걸려 있습니다.</b> 사유 : `)+
				tool.HTML_escape(tool.Baro_get_def(db, "sec_emergency_reason", "사유 없음"))+
				tool.Baro_t(` <a href="/baro/status/emergency">긴급 편집 중단 화면</a>`),
		)
	}

	return html
}

// baro_sec_page : sec 모듈 표준 페이지 (읽기 전용 안내를 항상 위에 붙인다)
func baro_sec_page(db *sql.DB, config tool.Config, title string, body string) string {
	return tool.Baro_page(db, config, title, baro_sec_banner(db)+body, baro_sec_menu())
}

// ------------------------------------------------------------
//  입력 다루기
// ------------------------------------------------------------

// baro_sec_ip_key : 라우트가 실제 접속 IP 를 실어 보내는 질의 이름
const baro_sec_ip_key = "baro_sec_ip"

// baro_sec_real_ip : 이 요청의 실제 접속 IP
//
//	로그인한 사람은 config.IP 가 사용자 이름이므로 접속 지점을 알 수 없다.
//	라우트(routes_baro_sec.go)가 질의에 실어 보낸 실제 IP 를 쓰고,
//	그 값이 없으면 비로그인 접속에 한해 config.IP 를 그대로 쓴다.
func baro_sec_real_ip(config tool.Config, query url.Values) string {
	if query != nil {
		if ip := strings.TrimSpace(query.Get(baro_sec_ip_key)); ip != "" {
			return ip
		}
	}

	if !tool.Baro_is_user(config.IP) {
		return strings.TrimSpace(config.IP)
	}

	return ""
}

// baro_sec_ip_view : 화면에 보여줄 IP 대역 (알 수 없으면 안내 문구)
func baro_sec_ip_view(ip string) string {
	band := baro_sec_ip_band(ip)
	if band == "" {
		return tool.Baro_t("확인할 수 없음")
	}

	return band
}

// baro_sec_q : 질의 문자열 값 (없으면 "")
func baro_sec_q(query url.Values, name string) string {
	if query == nil {
		return ""
	}

	return strings.TrimSpace(query.Get(name))
}

// baro_sec_v : 보낸 값 (없으면 "")
func baro_sec_v(values url.Values, name string) string {
	if values == nil {
		return ""
	}

	return strings.TrimSpace(values.Get(name))
}

// baro_sec_checked : 체크 상자가 켜져 있는지
func baro_sec_checked(values url.Values, name string) bool {
	return baro_sec_v(values, name) != ""
}

// baro_sec_page_num : 쪽 번호 (1 미만이면 1)
func baro_sec_page_num(query url.Values) int {
	page := tool.Str_to_int(baro_sec_q(query, "page"))
	if page < 1 {
		page = 1
	}

	return page
}

// baro_sec_int_def : 문자열을 정수로 (비었거나 0 이하이면 기본값)
func baro_sec_int_def(data string, def int) int {
	value := tool.Str_to_int(strings.TrimSpace(data))
	if value <= 0 {
		return def
	}

	return value
}

// ------------------------------------------------------------
//  설정 (baro_set)
// ------------------------------------------------------------

// baro_sec_hours_between : 두 시각 사이의 시간 차 (절댓값, 시간 단위)
func baro_sec_hours_between(a string, b string) float64 {
	ta := tool.Baro_parse_time(a)
	tb := tool.Baro_parse_time(b)
	if ta.IsZero() || tb.IsZero() {
		return -1
	}

	gap := tb.Sub(ta).Hours()
	if gap < 0 {
		gap = -gap
	}

	return gap
}

// baro_sec_window_live : 시간 창이 아직 살아 있는지 (요청 제한 현황 판정용)
func baro_sec_window_live(start string, window_sec int) bool {
	begin := tool.Baro_parse_time(start)
	if begin.IsZero() {
		return false
	}

	return time.Since(begin) < time.Duration(window_sec)*time.Second
}

// baro_sec_flag : 켬/끔 설정 (저장된 값이 없으면 기본값)
func baro_sec_flag(db *sql.DB, name string, def bool) bool {
	data := strings.TrimSpace(tool.Baro_get(db, name))
	if data == "" {
		return def
	}

	return data == "on"
}

// baro_sec_flag_put : 켬/끔 설정 저장 (끈 상태도 명시적으로 남긴다)
func baro_sec_flag_put(db *sql.DB, name string, on bool) {
	if on {
		tool.Baro_put(db, name, "on")

		return
	}

	tool.Baro_put(db, name, "off")
}

// baro_sec_on_off : 켬/끔 배지
func baro_sec_on_off(on bool) string {
	if on {
		return tool.Baro_badge(tool.Baro_t("켜짐"), "ok")
	}

	return tool.Baro_badge(tool.Baro_t("꺼짐"), "muted")
}

// baro_sec_yes_no : 두 상태 배지
func baro_sec_yes_no(yes bool, yes_text string, no_text string) string {
	if yes {
		return tool.Baro_badge(yes_text, "ok")
	}

	return tool.Baro_badge(no_text, "danger")
}

// ------------------------------------------------------------
//  권한 / 위키 정보
// ------------------------------------------------------------

// baro_sec_is_system : 시스템 설정을 만질 수 있는가
func baro_sec_is_system(db *sql.DB, ip string) bool {
	return tool.Baro_can(db, ip, "system")
}

// baro_sec_wiki_name : 위키 이름 (2단계 인증 발급자 이름 등에 쓴다)
func baro_sec_wiki_name(db *sql.DB) string {
	name := ""
	tool.QueryRow_DB(db, "select data from other where name = 'name'", []any{&name})

	name = strings.TrimSpace(name)
	if name == "" {
		name = "바로위키"
	}

	return name
}

// ------------------------------------------------------------
//  TOTP (2단계 인증) — 외부 라이브러리 없이 표준 라이브러리로만 구현
//
//  RFC 6238 : 30초 시간 창 · HMAC-SHA1 · 6자리
// ------------------------------------------------------------

const (
	// baro_sec_totp_period : 시간 창 (초)
	baro_sec_totp_period = 30

	// baro_sec_totp_skew : 앞뒤로 허용하는 시간 창 수 (시계 오차 대비)
	baro_sec_totp_skew = 1
)

// baro_sec_b32 : 패딩 없는 base32 (인증 앱이 쓰는 형식)
var baro_sec_b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// baro_sec_new_secret : 새 비밀키를 base32 문자열로 만든다 (160비트)
func baro_sec_new_secret() string {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		// 예비 경로 : 시스템 난수를 못 쓰면 위키 공통 난수 생성기를 쓴다
		return baro_sec_b32.EncodeToString([]byte(tool.Get_random_key(20)))
	}

	return baro_sec_b32.EncodeToString(raw)
}

// baro_sec_secret_bytes : base32 비밀키를 바이트로 되돌린다
func baro_sec_secret_bytes(secret string) ([]byte, bool) {
	clean := strings.ToUpper(strings.TrimSpace(secret))
	clean = strings.ReplaceAll(clean, " ", "")
	clean = strings.ReplaceAll(clean, "-", "")
	clean = strings.TrimRight(clean, "=")

	if clean == "" {
		return nil, false
	}

	raw, err := baro_sec_b32.DecodeString(clean)
	if err != nil || len(raw) == 0 {
		return nil, false
	}

	return raw, true
}

// baro_sec_totp_at : 지정한 시간 창의 코드
func baro_sec_totp_at(secret string, counter int64) string {
	key, ok := baro_sec_secret_bytes(secret)
	if !ok {
		return ""
	}

	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, uint64(counter))

	mac := hmac.New(sha1.New, key)
	mac.Write(buffer)
	sum := mac.Sum(nil)

	offset := int(sum[len(sum)-1] & 0x0f)
	value := (int64(sum[offset]&0x7f) << 24) |
		(int64(sum[offset+1]) << 16) |
		(int64(sum[offset+2]) << 8) |
		int64(sum[offset+3])

	return fmt.Sprintf("%06d", value%1000000)
}

// baro_sec_code_clean : 사용자가 입력한 코드 정리 (공백·하이픈 제거)
func baro_sec_code_clean(code string) string {
	code = strings.TrimSpace(code)
	code = strings.ReplaceAll(code, " ", "")
	code = strings.ReplaceAll(code, "-", "")

	return code
}

// baro_sec_totp_ok : 코드 검증 (앞뒤 한 칸까지 허용)
func baro_sec_totp_ok(secret string, code string) bool {
	code = baro_sec_code_clean(code)
	if len(code) != 6 {
		return false
	}

	now := time.Now().Unix() / baro_sec_totp_period
	for delta := int64(-baro_sec_totp_skew); delta <= baro_sec_totp_skew; delta++ {
		expect := baro_sec_totp_at(secret, now+delta)
		if expect == "" {
			return false
		}

		if hmac.Equal([]byte(expect), []byte(code)) {
			return true
		}
	}

	return false
}

// baro_sec_otpauth : 인증 앱에 넣을 otpauth:// 주소
func baro_sec_otpauth(db *sql.DB, user string, secret string) string {
	issuer := baro_sec_wiki_name(db)

	return "otpauth://totp/" + url.PathEscape(issuer+":"+user) +
		"?secret=" + url.QueryEscape(secret) +
		"&issuer=" + url.QueryEscape(issuer) +
		"&algorithm=SHA1&digits=6&period=" + tool.Baro_itoa(baro_sec_totp_period)
}

// baro_sec_secret_pretty : 수동 입력용으로 4글자씩 끊어 보여준다
func baro_sec_secret_pretty(secret string) string {
	out := []string{}
	for index := 0; index < len(secret); index += 4 {
		end := index + 4
		if end > len(secret) {
			end = len(secret)
		}

		out = append(out, secret[index:end])
	}

	return strings.Join(out, " ")
}

// ------------------------------------------------------------
//  2단계 인증 저장 (baro_2fa)
// ------------------------------------------------------------

// baro_sec_2fa_row : 사용자의 2단계 인증 정보 (비밀키, 백업코드 JSON, 상태, 시각)
func baro_sec_2fa_row(db *sql.DB, user string) (string, string, string, string) {
	secret, backup, status, date := "", "", "", ""
	tool.QueryRow_DB(
		db,
		"select secret, backup, status, date from baro_2fa where user = ?",
		[]any{&secret, &backup, &status, &date}, user,
	)

	return secret, backup, status, date
}

// baro_sec_2fa_save : 2단계 인증 정보 저장 (덮어쓰기)
func baro_sec_2fa_save(db *sql.DB, user string, secret string, backup string, status string) {
	tool.Exec_DB(db, "delete from baro_2fa where user = ?", user)
	tool.Exec_DB(
		db,
		"insert into baro_2fa (user, secret, backup, status, date) values (?, ?, ?, ?, ?)",
		user, secret, backup, status, tool.Baro_now(),
	)
}

// baro_sec_2fa_drop : 2단계 인증 해제
func baro_sec_2fa_drop(db *sql.DB, user string) {
	tool.Exec_DB(db, "delete from baro_2fa where user = ?", user)
}

// baro_sec_2fa_active : 2단계 인증을 실제로 쓰고 있는지
func baro_sec_2fa_active(db *sql.DB, user string) bool {
	if !tool.Baro_is_user(user) {
		return false
	}

	_, _, status, _ := baro_sec_2fa_row(db, user)

	return status == "active"
}

// baro_sec_backup_make : 백업 코드 10개를 새로 만든다 (원본, 해시 목록)
func baro_sec_backup_make() ([]string, []string) {
	codes := []string{}
	hashes := []string{}

	for index := 0; index < 10; index++ {
		raw := strings.ToLower(tool.Get_random_key(10))
		code := raw[:5] + "-" + raw[5:]

		codes = append(codes, code)
		hashes = append(hashes, tool.Sha224(baro_sec_code_clean(code)))
	}

	return codes, hashes
}

// baro_sec_backup_use : 백업 코드가 맞으면 그 코드를 소모하고 true
func baro_sec_backup_use(db *sql.DB, user string, code string) bool {
	code = strings.ToLower(baro_sec_code_clean(code))
	if code == "" {
		return false
	}

	secret, backup, status, _ := baro_sec_2fa_row(db, user)
	if status == "" {
		return false
	}

	target := tool.Sha224(code)
	left := []string{}
	found := false

	for _, saved := range tool.Baro_json_dec_list(backup) {
		if !found && hmac.Equal([]byte(saved), []byte(target)) {
			found = true

			continue
		}

		left = append(left, saved)
	}

	if !found {
		return false
	}

	baro_sec_2fa_save(db, user, secret, tool.Baro_json_enc(left), status)

	return true
}

// baro_sec_backup_left : 남은 백업 코드 수
func baro_sec_backup_left(backup string) int {
	return len(tool.Baro_json_dec_list(backup))
}

// baro_sec_2fa_check : 2단계 인증 코드 또는 백업 코드 확인
func baro_sec_2fa_check(db *sql.DB, user string, code string) bool {
	secret, _, status, _ := baro_sec_2fa_row(db, user)
	if status == "" {
		return false
	}

	if baro_sec_totp_ok(secret, code) {
		return true
	}

	return baro_sec_backup_use(db, user, code)
}

// ------------------------------------------------------------
//  사용자별 보안 설정 (baro_user_set)
// ------------------------------------------------------------

// baro_sec_user_get : 사용자 설정 읽기
func baro_sec_user_get(db *sql.DB, user string, name string) string {
	data := ""
	tool.QueryRow_DB(db, "select data from baro_user_set where user = ? and name = ?", []any{&data}, user, name)

	return data
}

// baro_sec_user_put : 사용자 설정 저장
func baro_sec_user_put(db *sql.DB, user string, name string, data string) {
	tool.Exec_DB(db, "delete from baro_user_set where user = ? and name = ?", user, name)
	tool.Exec_DB(db, "insert into baro_user_set (user, name, data) values (?, ?, ?)", user, name, data)
}

// ------------------------------------------------------------
//  접속 기록 (baro_login_log)
// ------------------------------------------------------------

// baro_sec_ip_band : IP 대역 (IPv4 는 앞 두 칸, IPv6 는 앞 세 묶음)
func baro_sec_ip_band(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}

	if strings.Contains(ip, ":") {
		parts := strings.Split(ip, ":")
		if len(parts) >= 3 {
			return strings.Join(parts[:3], ":") + "::"
		}

		return ip
	}

	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".*.*"
	}

	return ip
}

// baro_sec_login_add : 접속 기록 한 줄 남기기
func baro_sec_login_add(db *sql.DB, user string, ip string, ua string, kind string, detail string) {
	tool.Exec_DB(
		db,
		"insert into baro_login_log (id, user, ip, ua, date, kind, detail) values (?, ?, ?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_login_log"), user, ip, tool.Baro_trunc(ua, 300), tool.Baro_now(), kind, detail,
	)
}

// baro_sec_known_band : 이 사용자가 전에 쓰던 대역인지
func baro_sec_known_band(db *sql.DB, user string, band string) bool {
	if band == "" {
		return true
	}

	rows := tool.Query_DB(db, "select ip from baro_login_log where user = ? order by id + 0 desc limit 200", user)
	defer rows.Close()

	for rows.Next() {
		ip := ""
		if err := rows.Scan(&ip); err != nil {
			continue
		}

		if baro_sec_ip_band(ip) == band {
			return true
		}
	}

	return false
}

// baro_sec_watch : 보안 화면 접속을 기록하고, 처음 보는 IP 대역이면 알린다
//
//	openNAMU 본체의 로그인 처리는 이 확장이 건드리지 않으므로
//	바로위키 보안 화면에 들어온 순간을 관측 지점으로 삼는다.
//	같은 대역이 짧은 시간에 여러 번 기록되지 않도록 요청 제한을 함께 쓴다.
func baro_sec_watch(db *sql.DB, config tool.Config, real_ip string) {
	user := config.IP
	if !tool.Baro_is_user(user) {
		return
	}

	ip_band := baro_sec_ip_band(real_ip)
	known := baro_sec_known_band(db, user, ip_band)

	// 같은 사용자·같은 대역은 한 시간에 한 번만 기록한다
	if known && !tool.Baro_rate_ok(db, "sec_watch:"+ip_band, user, 1, 3600) {
		return
	}

	kind := "access"
	detail := tool.Baro_t("바로위키 보안 화면 접속")
	if !known {
		kind = "new_band"
		detail = tool.Baro_t("이전 기록에 없던 IP 대역(") + ip_band + tool.Baro_t(")에서 접속")
	}

	baro_sec_login_add(db, user, real_ip, config.UserAgent, kind, detail)

	if known || !baro_sec_flag(db, "sec_suspicious_notify", true) {
		return
	}

	tool.Baro_notify(
		db, user, "system",
		tool.Baro_t("처음 보는 IP 대역(")+ip_band+tool.Baro_t(")에서 접속이 확인됐습니다."),
		"/baro/security/login_log",
		tool.Baro_t("본인이 아니라면 즉시 비밀번호를 바꾸고 세션을 모두 끊으세요."),
	)
}

// ------------------------------------------------------------
//  기록 남기기 (운영 조치는 공개 로그 + 내부 감사 로그 둘 다)
// ------------------------------------------------------------

// baro_sec_log_op : 운영 조치 기록
func baro_sec_log_op(db *sql.DB, config tool.Config, kind string, target string, before string, after string, reason string) {
	tool.Baro_public_log(db, kind, config.IP, target, reason, "")
	tool.Baro_audit(db, config, kind, target, before, after, reason)
}
