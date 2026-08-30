package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  17-1 계정 보안
//
//   - 비밀번호 안전 저장 (본체 방식 안내)
//   - 로그인 시도 제한 (tool.Baro_rate_ok)
//   - 2단계 인증 (TOTP 직접 구현)
//   - 의심 접속 알림
//   - 세션 강제 종료
//   - 권한 변경 재인증 / 관리자 강제 2단계 인증
// ============================================================

// ------------------------------------------------------------
//  비밀번호 저장 방식 안내
// ------------------------------------------------------------

// baro_sec_encode_name : openNAMU 비밀번호 저장 방식 이름
func baro_sec_encode_name(encode string) string {
	switch encode {
	case "sha256":
		return tool.Baro_t("SHA-256 (소금 없음)")
	case "sha3":
		return tool.Baro_t("SHA3-256 (소금 없음)")
	case "sha3-512":
		return tool.Baro_t("SHA3-512 (소금 없음)")
	case "sha3-salt":
		return tool.Baro_t("SHA3-256 + 위키 고정 소금")
	case "sha3-512-salt":
		return tool.Baro_t("SHA3-512 + 위키 고정 소금")
	}

	if encode == "" {
		return tool.Baro_t("설정 없음 (기본 SHA3-256)")
	}

	return encode + tool.Baro_t(" (SHA3-512 + 위키 고정 소금으로 처리됨)")
}

// baro_sec_encode_salted : 소금을 쓰는 방식인지
func baro_sec_encode_salted(encode string) bool {
	return strings.Contains(encode, "salt") || (encode != "sha256" && encode != "sha3" && encode != "sha3-512" && encode != "")
}

// ------------------------------------------------------------
//  재인증 (권한 변경 · 위험 조치 전에 다시 본인 확인)
// ------------------------------------------------------------

// baro_sec_reauth_need : 재인증이 필요한 설정인지
func baro_sec_reauth_need(db *sql.DB) bool {
	return baro_sec_flag(db, "sec_reauth_on", true)
}

// baro_sec_reauth_ok : 재인증 값 확인 (통과하면 true, 아니면 안내 문구)
//
//	2단계 인증을 쓰고 있으면 인증 코드를, 아니면 비밀번호를 다시 받는다.
func baro_sec_reauth_ok(db *sql.DB, config tool.Config, values url.Values) (bool, string) {
	if !baro_sec_reauth_need(db) {
		return true, ""
	}

	if !tool.Baro_is_user(config.IP) {
		return false, tool.Baro_t("로그인이 필요합니다.")
	}

	answer := baro_sec_v(values, "reauth")
	if answer == "" {
		if baro_sec_2fa_active(db, config.IP) {
			return false, tool.Baro_t("재인증이 켜져 있습니다. 2단계 인증 코드를 입력해야 이 조치를 실행할 수 있습니다.")
		}

		return false, tool.Baro_t("재인증이 켜져 있습니다. 계정 비밀번호를 다시 입력해야 이 조치를 실행할 수 있습니다.")
	}

	// 재인증 시도에도 시도 제한을 그대로 적용한다
	if !baro_sec_login_rate_ok(db, config.IP) {
		return false, tool.Baro_t("인증 시도가 너무 잦습니다. 잠시 뒤에 다시 시도하세요.")
	}

	if baro_sec_2fa_active(db, config.IP) {
		if baro_sec_2fa_check(db, config.IP, answer) {
			baro_sec_login_rate_reset(db, config.IP)

			return true, ""
		}

		return false, tool.Baro_t("2단계 인증 코드가 맞지 않습니다.")
	}

	if tool.Password_check(db, config.IP, answer) {
		baro_sec_login_rate_reset(db, config.IP)

		return true, ""
	}

	return false, tool.Baro_t("비밀번호가 맞지 않습니다.")
}

// baro_sec_reauth_field : 위험 조치 폼에 붙이는 재인증 입력칸
func baro_sec_reauth_field(db *sql.DB, config tool.Config) string {
	if !baro_sec_reauth_need(db) {
		return tool.Baro_note(tool.Baro_t("재인증 설정이 꺼져 있어 추가 확인 없이 실행됩니다."))
	}

	label := tool.Baro_t("재인증 : 계정 비밀번호")
	help := tool.Baro_t("권한·보안 설정 변경 전 본인 확인을 위해 비밀번호를 다시 입력합니다.")
	if baro_sec_2fa_active(db, config.IP) {
		label = tool.Baro_t("재인증 : 2단계 인증 코드")
		help = tool.Baro_t("2단계 인증을 쓰고 있으므로 인증 앱의 6자리 코드(또는 백업 코드)를 입력합니다.")
	}

	return tool.Baro_input_type("reauth", label, "", "password", help)
}

// ------------------------------------------------------------
//  로그인 · 인증 시도 제한
// ------------------------------------------------------------

// baro_sec_login_limit : 허용 시도 횟수
func baro_sec_login_limit(db *sql.DB) int {
	return tool.Baro_get_int(db, "sec_login_limit", 5)
}

// baro_sec_login_window : 시도를 세는 시간 창 (초)
func baro_sec_login_window(db *sql.DB) int {
	return tool.Baro_get_int(db, "sec_login_window", 300)
}

// baro_sec_login_rate_ok : 인증 시도 제한 확인 (실제로 이 모듈의 인증 동작에 적용된다)
func baro_sec_login_rate_ok(db *sql.DB, who string) bool {
	if !baro_sec_flag(db, "sec_login_limit_on", true) {
		return true
	}

	return tool.Baro_rate_ok(db, "sec_login", who, baro_sec_login_limit(db), baro_sec_login_window(db))
}

// baro_sec_login_rate_reset : 인증에 성공하면 시도 횟수를 지운다
//
//	실패를 세는 것이 목적이므로, 본인 확인에 성공한 뒤에는 계수기를 되돌린다.
func baro_sec_login_rate_reset(db *sql.DB, who string) {
	tool.Exec_DB(db, "delete from baro_rate where key_name = 'sec_login' and who = ?", who)
}

// baro_sec_rate_rows : 요청 제한 표에서 조건에 맞는 줄 읽기
func baro_sec_rate_rows(db *sql.DB, like string, limit int) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(
		db,
		"select key_name, who, num, window_start from baro_rate where key_name like ? order by window_start desc limit ?",
		like, limit,
	)
	defer rows.Close()

	for rows.Next() {
		key_name, who, num, window_start := "", "", "", ""
		if err := rows.Scan(&key_name, &who, &num, &window_start); err != nil {
			continue
		}

		out = append(out, []string{key_name, who, num, window_start})
	}

	return out
}

// baro_sec_admin_list : 관리자 계정 목록 (2단계 인증 점검용, 최대 200명)
func baro_sec_admin_list(db *sql.DB) []string {
	out := []string{}
	seen := map[string]bool{}

	rows := tool.Query_DB(db, "select id from user_set where name = 'acl' and data != 'user' limit 200")
	defer rows.Close()

	for rows.Next() {
		id := ""
		if err := rows.Scan(&id); err != nil {
			continue
		}

		if id == "" || seen[id] {
			continue
		}

		seen[id] = true
		if tool.Baro_is_admin(db, id) {
			out = append(out, id)
		}
	}

	role_rows := tool.Query_DB(db, "select user from baro_role where role = 'sysadmin' limit 200")
	defer role_rows.Close()

	for role_rows.Next() {
		id := ""
		if err := role_rows.Scan(&id); err != nil {
			continue
		}

		if id == "" || seen[id] {
			continue
		}

		seen[id] = true
		out = append(out, id)
	}

	return out
}

// ------------------------------------------------------------
//  /baro/security : 계정 보안 홈
// ------------------------------------------------------------

// View_baro_sec_home : 계정 보안 화면
func View_baro_sec_home(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/security"); need {
		return wall
	}

	baro_sec_watch(db, config, baro_sec_real_ip(config, query))

	notice := ""

	if values != nil {
		return baro_sec_home_post(db, config, values)
	}

	if baro_sec_q(query, "done") != "" {
		notice = tool.Baro_alert("ok", tool.Baro_t("설정을 저장했습니다."))
	}
	if baro_sec_q(query, "fail") != "" {
		notice = tool.Baro_alert("danger", tool.HTML_escape(baro_sec_q(query, "fail")))
	}

	is_system := baro_sec_is_system(db, config.IP)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("보안")}})
	body += tool.Baro_hero(
		tool.Baro_t("계정 보안"),
		tool.Baro_t("내 계정을 지키는 설정과 현재 상태를 한 곳에서 확인합니다."),
		"",
	)
	body += baro_sec_tabs("/baro/security")
	body += notice

	// --- 내 보안 상태 요약 ---
	secret, backup, status, _ := baro_sec_2fa_row(db, config.IP)
	_ = secret

	two_factor := tool.Baro_t("미설정")
	if status == "pending" {
		two_factor = tool.Baro_t("설정 중")
	} else if status == "active" {
		two_factor = tool.Baro_t("사용 중")
	}

	reset_at := baro_sec_user_get(db, config.IP, "session_reset")
	if reset_at == "" {
		reset_at = "없음"
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("2단계 인증"), two_factor, tool.Baro_t("인증 앱 기반 TOTP")},
		{tool.Baro_t("남은 백업 코드"), tool.Baro_itoa(baro_sec_backup_left(backup)), tool.Baro_t("한 번 쓰면 사라집니다")},
		{tool.Baro_t("최근 세션 초기화"), reset_at, tool.Baro_t("이 시각 이전 세션은 무효")},
		{tool.Baro_t("내 접속 기록"), tool.Baro_itoa(baro_sec_login_count(db, config.IP)), tool.Baro_t("바로위키가 관측한 접속")},
	})

	// --- 비밀번호 저장 방식 ---
	main_encode := tool.Get_main_encode(db)
	my_encode := tool.Get_user_encode(db, config.IP)
	min_length := ""
	tool.QueryRow_DB(db, "select data from other where name = 'password_min_length'", []any{&min_length})
	if strings.TrimSpace(min_length) == "" {
		min_length = tool.Baro_t("제한 없음")
	}

	pw_body := tool.Baro_dl([][]string{
		{tool.Baro_t("위키 기본 저장 방식"), tool.HTML_escape(baro_sec_encode_name(main_encode))},
		{tool.Baro_t("내 계정에 적용된 방식"), tool.HTML_escape(baro_sec_encode_name(my_encode))},
		{tool.Baro_t("소금(salt) 사용"), func() string {
			if baro_sec_encode_salted(my_encode) {
				return tool.Baro_badge(tool.Baro_t("사용"), "ok") + tool.Baro_t(" 위키 전체가 공유하는 고정 소금")
			}

			return tool.Baro_badge(tool.Baro_t("미사용"), "warn") + tool.Baro_t(" 같은 비밀번호는 같은 해시가 됩니다")
		}()},
		{tool.Baro_t("최소 비밀번호 길이"), tool.HTML_escape(min_length)},
		{tool.Baro_t("처리 위치"), tool.Baro_t("openNAMU 본체 <code>route/tool/login.go</code> 의 <code>Get_main_encode</code> · <code>Password_encode</code>")},
	})
	pw_body += tool.Baro_alert("info",
		tool.Baro_t("바로위키 확장은 비밀번호 저장 방식을 바꾸지 않습니다. 위 방식은 openNAMU 본체가 정합니다.<br>")+
			tool.Baro_t("현재 방식은 <b>단일 해시</b>라 bcrypt·argon2 같은 반복 해시보다 무차별 대입에 약합니다. ")+
			tool.Baro_t("그래서 이 화면에서는 다음을 권장합니다."),
	)
	pw_body += tool.Baro_list([]string{
		tool.Baro_t("12자 이상, 다른 사이트와 겹치지 않는 비밀번호를 쓰세요."),
		tool.Baro_t("소금이 꺼져 있다면 위키 관리자에게 소금이 포함된 방식(<code>sha3-salt</code> 이상)으로 바꿔 달라고 요청하세요."),
		tool.Baro_t("비밀번호만으로는 부족합니다. 아래 2단계 인증을 켜세요."),
	})
	body += tool.Baro_card(tool.Baro_t("비밀번호는 어떻게 저장되나요"), pw_body)

	// --- 2단계 인증 요약 ---
	two_body := tool.Baro_kv_grid([][]string{
		{"상태", func() string {
			switch status {
			case "active":
				return tool.Baro_badge(tool.Baro_t("사용 중"), "ok")
			case "pending":
				return tool.Baro_badge(tool.Baro_t("설정 중"), "warn")
			}

			return tool.Baro_badge(tool.Baro_t("미설정"), "danger")
		}()},
		{tool.Baro_t("남은 백업 코드"), tool.Baro_itoa(baro_sec_backup_left(backup)) + tool.Baro_t("개")},
	})
	two_body += tool.Baro_note(tool.Baro_t("인증 앱(예 : 시간 기반 OTP 앱)에 비밀키를 넣고, 30초마다 바뀌는 6자리 코드로 본인을 확인합니다."))
	two_body += tool.Baro_button("/baro/security/2fa", tool.Baro_t("2단계 인증 설정 화면"), "primary")
	body += tool.Baro_card(tool.Baro_t("2단계 인증"), two_body)

	// --- 로그인 시도 제한 ---
	limit_rows := [][]string{}
	for _, row := range baro_sec_rate_rows(db, "sec_login%", 30) {
		blocked := tool.Str_to_int(row[2]) >= baro_sec_login_limit(db) && baro_sec_window_live(row[3], baro_sec_login_window(db))
		state := tool.Baro_badge(tool.Baro_t("감시 중"), "info")
		if blocked {
			state = tool.Baro_badge(tool.Baro_t("차단 중"), "danger")
		}

		clear_html := tool.Baro_note(tool.Baro_t("권한 없음"))
		if is_system {
			clear_html = tool.Baro_form("/baro/security",
				tool.Baro_hidden("op", "rate_clear")+
					tool.Baro_hidden("key_name", row[0])+
					tool.Baro_hidden("who", row[1])+
					tool.Baro_submit(tool.Baro_t("차단 해제"), ""),
			)
		}

		limit_rows = append(limit_rows, []string{
			tool.HTML_escape(row[0]),
			tool.HTML_escape(tool.Baro_mask_ip(row[1])),
			tool.HTML_escape(row[2]),
			tool.Baro_time_cell(row[3]),
			state,
			clear_html,
		})
	}

	rate_body := tool.Baro_kv_grid([][]string{
		{tool.Baro_t("제한 사용"), baro_sec_on_off(baro_sec_flag(db, "sec_login_limit_on", true))},
		{tool.Baro_t("허용 시도"), tool.Baro_itoa(baro_sec_login_limit(db)) + tool.Baro_t("회")},
		{tool.Baro_t("시간 창"), tool.Baro_itoa(baro_sec_login_window(db)) + tool.Baro_t("초")},
	})
	rate_body += tool.Baro_note(
		tool.Baro_t("이 설정은 tool.Baro_rate_ok 로 동작하며, 바로위키가 직접 처리하는 인증(2단계 인증 코드 확인, 재인증)에 즉시 적용됩니다. ") +
			tool.Baro_t("로그인 화면의 2단계 인증 코드 확인도 바로위키가 처리하며, 그쪽은 5분에 10번으로 따로 묶여 있습니다. ") +
			tool.Baro_t("아이디·비밀번호 확인 자체는 openNAMU 본체 코드가 맡습니다."),
	)
	rate_body += tool.Baro_table(
		[]string{tool.Baro_t("제한 키"), tool.Baro_t("대상"), tool.Baro_t("시도 수"), tool.Baro_t("시작 시각"), "상태", "관리"},
		limit_rows,
		tool.Baro_t("최근 인증 시도 제한에 걸린 대상이 없습니다."),
	)

	if is_system {
		form := tool.Baro_hidden("op", "rate_set")
		form += tool.Baro_checkbox("login_limit_on", tool.Baro_t("인증 시도 제한 사용"), "on", baro_sec_flag(db, "sec_login_limit_on", true), "")
		form += tool.Baro_input_type("login_limit", tool.Baro_t("허용 시도 횟수"), tool.Baro_itoa(baro_sec_login_limit(db)), "number", tool.Baro_t("시간 창 안에서 허용하는 인증 시도 수"))
		form += tool.Baro_input_type("login_window", "시간 창 (초)", tool.Baro_itoa(baro_sec_login_window(db)), "number", tool.Baro_t("예 : 300 이면 5분"))
		form += baro_sec_reauth_field(db, config)
		form += tool.Baro_submit(tool.Baro_t("시도 제한 설정 저장"), "primary")

		rate_body += tool.Baro_section(tool.Baro_t("설정 바꾸기"), tool.Baro_form("/baro/security", form))
	}

	body += tool.Baro_card(tool.Baro_t("로그인·인증 시도 제한"), rate_body)

	// --- 의심 접속 알림 / 접속 기록 ---
	log_rows := [][]string{}
	for _, row := range baro_sec_login_rows(db, config.IP, "", 0, 5) {
		log_rows = append(log_rows, []string{
			tool.Baro_time_cell(row[4]),
			tool.HTML_escape(baro_sec_login_kind_name(row[5])),
			tool.HTML_escape(baro_sec_ip_band(row[2])),
			tool.HTML_escape(tool.Baro_trunc(row[6], 60)),
		})
	}

	watch_body := tool.Baro_kv_grid([][]string{
		{tool.Baro_t("의심 접속 알림"), baro_sec_on_off(baro_sec_flag(db, "sec_suspicious_notify", true))},
		{tool.Baro_t("내 IP 대역"), tool.HTML_escape(baro_sec_ip_view(baro_sec_real_ip(config, query)))},
	})
	watch_body += tool.Baro_table([]string{"시각", "종류", tool.Baro_t("IP 대역"), tool.Baro_t("설명")}, log_rows, tool.Baro_t("아직 기록된 접속이 없습니다."))
	watch_body += tool.Baro_button("/baro/security/login_log", tool.Baro_t("내 접속 기록 전체 보기"), "")
	body += tool.Baro_card(tool.Baro_t("의심 접속 알림"), watch_body)

	// --- 세션 ---
	session_body := tool.Baro_kv_grid([][]string{
		{tool.Baro_t("마지막 세션 초기화"), tool.HTML_escape(reset_at)},
	})
	session_body += tool.Baro_note(tool.Baro_t("세션을 모두 끊으면, 초기화 시각 이전에 발급된 세션은 바로위키 기준으로 무효로 취급됩니다."))
	session_body += tool.Baro_button("/baro/security/session", tool.Baro_t("세션 강제 종료 화면"), "")
	body += tool.Baro_card(tool.Baro_t("세션 관리"), session_body)

	// --- 권한 변경 재인증 / 관리자 강제 2단계 인증 ---
	force := baro_sec_flag(db, "sec_admin_2fa_force", false)
	policy_body := tool.Baro_kv_grid([][]string{
		{tool.Baro_t("권한 변경·위험 조치 재인증"), baro_sec_on_off(baro_sec_reauth_need(db))},
		{tool.Baro_t("관리자 강제 2단계 인증"), baro_sec_on_off(force)},
	})

	policy_body += tool.Baro_note(baro_sec_2fa_summary(db))

	if force {
		missing := []string{}
		for _, admin := range baro_sec_2fa_missing_admin(db) {
			missing = append(missing, tool.Baro_user_link(db, admin, config.IP))
		}

		if len(missing) == 0 {
			policy_body += tool.Baro_alert("ok", tool.Baro_t("확인된 관리자 계정은 모두 2단계 인증을 켰습니다."))
		} else {
			policy_body += tool.Baro_alert("danger",
				tool.Baro_t("<b>2단계 인증을 켜지 않은 관리자 계정이 ")+tool.Baro_itoa(len(missing))+tool.Baro_t("개 있습니다.</b><br>")+
					strings.Join(missing, ", "),
			)
		}

		if tool.Baro_is_admin(db, config.IP) && !baro_sec_2fa_active(db, config.IP) {
			policy_body += tool.Baro_alert("warn",
				tool.Baro_t("당신은 관리자 권한을 가지고 있지만 2단계 인증이 꺼져 있습니다. ")+
					tool.Baro_t(`<a href="/baro/security/2fa">지금 설정하세요.</a>`))
		}
	} else {
		policy_body += tool.Baro_note(tool.Baro_t("관리자 강제 2단계 인증이 꺼져 있어 미설정 관리자 경고를 표시하지 않습니다."))
	}

	if is_system {
		form := tool.Baro_hidden("op", "policy")
		form += tool.Baro_checkbox("reauth_on", tool.Baro_t("권한 변경·위험 조치에 재인증 요구"), "on", baro_sec_reauth_need(db), tool.Baro_t("이 모듈의 설정 저장·읽기 전용 전환·긴급 편집 중단에 바로 적용됩니다."))
		form += tool.Baro_checkbox("admin_2fa_force", tool.Baro_t("관리자에게 2단계 인증을 요구"), "on", force, tool.Baro_t("켜면 2단계 인증이 없는 관리자 계정을 이 화면에서 경고합니다."))
		form += tool.Baro_checkbox("suspicious_notify", tool.Baro_t("의심 접속 알림 보내기"), "on", baro_sec_flag(db, "sec_suspicious_notify", true), tool.Baro_t("이전에 없던 IP 대역에서 접속하면 알림을 보냅니다."))
		form += baro_sec_reauth_field(db, config)
		form += tool.Baro_submit(tool.Baro_t("보안 정책 저장"), "primary")

		policy_body += tool.Baro_section(tool.Baro_t("설정 바꾸기"), tool.Baro_form("/baro/security", form))
	} else {
		policy_body += tool.Baro_note(tool.Baro_t("이 설정은 system 권한(sysadmin 역할 또는 소유자)만 바꿀 수 있습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("권한 변경 재인증 · 관리자 강제 2단계 인증"), policy_body)

	return baro_sec_page(db, config, tool.Baro_t("계정 보안"), body)
}

// baro_sec_home_post : 계정 보안 화면의 쓰기 처리
func baro_sec_home_post(db *sql.DB, config tool.Config, values url.Values) string {
	op := baro_sec_v(values, "op")

	if !baro_sec_is_system(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("보안 설정은 system 권한(sysadmin 역할 또는 소유자)만 바꿀 수 있습니다."))
	}

	if ok, message := baro_sec_reauth_ok(db, config, values); !ok {
		return tool.Get_redirect("/baro/security?fail=" + url.QueryEscape(message))
	}

	switch op {
	case "rate_set":
		before := "on=" + tool.Baro_get(db, "sec_login_limit_on") +
			" limit=" + tool.Baro_itoa(baro_sec_login_limit(db)) +
			" window=" + tool.Baro_itoa(baro_sec_login_window(db))

		baro_sec_flag_put(db, "sec_login_limit_on", baro_sec_checked(values, "login_limit_on"))
		tool.Baro_put(db, "sec_login_limit", tool.Baro_itoa(baro_sec_int_def(baro_sec_v(values, "login_limit"), 5)))
		tool.Baro_put(db, "sec_login_window", tool.Baro_itoa(baro_sec_int_def(baro_sec_v(values, "login_window"), 300)))

		after := "on=" + tool.Baro_get(db, "sec_login_limit_on") +
			" limit=" + tool.Baro_itoa(baro_sec_login_limit(db)) +
			" window=" + tool.Baro_itoa(baro_sec_login_window(db))

		baro_sec_log_op(db, config, "sec_rate_set", tool.Baro_t("인증 시도 제한"), before, after, tool.Baro_t("인증 시도 제한 설정 변경"))
	case "rate_clear":
		key_name := baro_sec_v(values, "key_name")
		who := baro_sec_v(values, "who")
		if key_name == "" || who == "" {
			return tool.Get_redirect("/baro/security?fail=" + url.QueryEscape(tool.Baro_t("해제할 대상이 없습니다.")))
		}

		tool.Exec_DB(db, "delete from baro_rate where key_name = ? and who = ?", key_name, who)
		baro_sec_log_op(db, config, "sec_rate_clear", key_name, tool.Baro_mask_ip(who), "", tool.Baro_t("인증 시도 제한 수동 해제"))
	case "policy":
		before := "reauth=" + tool.Baro_get(db, "sec_reauth_on") +
			" admin_2fa=" + tool.Baro_get(db, "sec_admin_2fa_force") +
			" notify=" + tool.Baro_get(db, "sec_suspicious_notify")

		baro_sec_flag_put(db, "sec_reauth_on", baro_sec_checked(values, "reauth_on"))
		baro_sec_flag_put(db, "sec_admin_2fa_force", baro_sec_checked(values, "admin_2fa_force"))
		baro_sec_flag_put(db, "sec_suspicious_notify", baro_sec_checked(values, "suspicious_notify"))

		after := "reauth=" + tool.Baro_get(db, "sec_reauth_on") +
			" admin_2fa=" + tool.Baro_get(db, "sec_admin_2fa_force") +
			" notify=" + tool.Baro_get(db, "sec_suspicious_notify")

		baro_sec_log_op(db, config, "sec_policy_set", tool.Baro_t("계정 보안 정책"), before, after, tool.Baro_t("계정 보안 정책 변경"))

		// 강제 2단계 인증을 켰다면 미설정 관리자에게 알린다
		if baro_sec_flag(db, "sec_admin_2fa_force", false) {
			for _, admin := range baro_sec_admin_list(db) {
				if baro_sec_2fa_active(db, admin) {
					continue
				}

				tool.Baro_notify(
					db, admin, "system",
					tool.Baro_t("관리자 계정은 2단계 인증이 필요합니다."),
					"/baro/security/2fa",
					tool.Baro_t("위키 정책이 바뀌어 관리자 계정에 2단계 인증이 요구됩니다."),
				)
			}
		}
	default:
		return tool.Get_redirect("/baro/security?fail=" + url.QueryEscape(tool.Baro_t("알 수 없는 요청입니다.")))
	}

	return tool.Get_redirect("/baro/security?done=1")
}

// ------------------------------------------------------------
//  접속 기록 읽기
// ------------------------------------------------------------

// baro_sec_login_kind_name : 접속 기록 종류 이름
func baro_sec_login_kind_name(kind string) string {
	switch kind {
	case "access":
		return tool.Baro_t("보안 화면 접속")
	case "new_band":
		return tool.Baro_t("새 IP 대역")
	case "session_reset":
		return tool.Baro_t("세션 강제 종료")
	case "2fa_on":
		return "2단계 인증 사용 시작"
	case "2fa_off":
		return "2단계 인증 해제"
	case "2fa_fail":
		return tool.Baro_t("인증 코드 실패")
	case "denied":
		return tool.Baro_t("본인 아님 신고")
	case "backup_new":
		return tool.Baro_t("백업 코드 재발급")
	}

	return kind
}

// baro_sec_login_count : 사용자의 접속 기록 수
func baro_sec_login_count(db *sql.DB, user string) int {
	count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_login_log where user = ?", []any{&count}, user)

	return count
}

// baro_sec_login_rows : 접속 기록 목록 (id, user, ip, ua, date, kind, detail)
func baro_sec_login_rows(db *sql.DB, user string, kind string, offset int, limit int) [][]string {
	out := [][]string{}

	query := "select id, user, ip, ua, date, kind, detail from baro_login_log where user = ?"
	args := []any{user}

	if kind != "" {
		query += " and kind = ?"
		args = append(args, kind)
	}

	query += " order by id + 0 desc limit ? offset ?"
	args = append(args, limit, offset)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	for rows.Next() {
		id, log_user, ip, ua, date, log_kind, detail := "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &log_user, &ip, &ua, &date, &log_kind, &detail); err != nil {
			continue
		}

		out = append(out, []string{id, log_user, ip, ua, date, log_kind, detail})
	}

	return out
}

// ------------------------------------------------------------
//  /baro/security/login_log : 내 접속 기록
// ------------------------------------------------------------

// View_baro_sec_login_log : 접속 기록 화면
func View_baro_sec_login_log(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/security/login_log"); need {
		return wall
	}

	if values != nil {
		return baro_sec_login_log_post(db, config, baro_sec_real_ip(config, query), values)
	}

	baro_sec_watch(db, config, baro_sec_real_ip(config, query))

	target := config.IP
	other := baro_sec_q(query, "user")
	is_system := baro_sec_is_system(db, config.IP)
	mask := false

	if other != "" && other != config.IP {
		if !is_system {
			return tool.Baro_deny(db, config, tool.Baro_t("다른 사용자의 접속 기록은 system 권한만 볼 수 있습니다."))
		}

		target = other
		mask = true
	}

	kind := baro_sec_q(query, "kind")
	page := baro_sec_page_num(query)
	rows := baro_sec_login_rows(db, target, kind, (page-1)*tool.Baro_page_size, tool.Baro_page_size)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"", tool.Baro_t("접속 기록")}})
	body += tool.Baro_hero(tool.Baro_t("접속 기록"), tool.Baro_t("바로위키가 관측한 접속을 시간 순으로 보여줍니다."), "")
	body += baro_sec_tabs("/baro/security/login_log")

	if baro_sec_q(query, "done") != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("처리했습니다. 세션을 초기화하고 알림을 보냈습니다."))
	}

	body += tool.Baro_alert("info",
		tool.Baro_t("openNAMU 본체의 로그인 처리에는 이 확장이 끼어들지 않습니다. 그래서 이 기록은 ")+
			tool.Baro_t("<b>바로위키 보안 화면에 접속한 순간</b>과 <b>2단계 인증·세션 조치</b>를 관측한 결과입니다. ")+
			tool.Baro_t("이전에 없던 IP 대역이 보이면 알림을 보냅니다."),
	)

	filter := tool.Baro_select("kind", "종류", [][]string{
		{"", tool.Baro_t("전체")},
		{"access", tool.Baro_t("보안 화면 접속")},
		{"new_band", tool.Baro_t("새 IP 대역")},
		{"2fa_on", "2단계 인증 사용 시작"},
		{"2fa_off", "2단계 인증 해제"},
		{"2fa_fail", tool.Baro_t("인증 코드 실패")},
		{"session_reset", tool.Baro_t("세션 강제 종료")},
		{"denied", tool.Baro_t("본인 아님 신고")},
	}, kind, "")

	if mask {
		filter += tool.Baro_hidden("user", target)
	}

	filter += tool.Baro_submit(tool.Baro_t("거르기"), "")
	body += `<form class="bw_form" method="get" action="/baro/security/login_log">` + filter + `</form>`

	if mask {
		body += tool.Baro_note(tool.Baro_t("다른 사용자의 기록이므로 IP 는 가려서 표시합니다. 대상 : ") + target)
	}

	table_rows := [][]string{}
	for _, row := range rows {
		ip_html := tool.HTML_escape(row[2])
		if mask {
			ip_html = tool.HTML_escape(tool.Baro_mask_ip(row[2]))
		}

		action := tool.Baro_note("-")
		if !mask && row[5] != "denied" {
			action = tool.Baro_form("/baro/security/login_log",
				tool.Baro_hidden("op", "deny")+
					tool.Baro_hidden("id", row[0])+
					tool.Baro_submit(tool.Baro_t("내가 아님"), "danger"),
			)
		}

		table_rows = append(table_rows, []string{
			tool.Baro_time_cell(row[4]),
			tool.HTML_escape(baro_sec_login_kind_name(row[5])),
			ip_html,
			tool.HTML_escape(tool.Baro_trunc(row[3], 60)),
			tool.HTML_escape(tool.Baro_trunc(row[6], 80)),
			action,
		})
	}

	if len(table_rows) == 0 {
		body += tool.Baro_empty(tool.Baro_t("조건에 맞는 접속 기록이 없습니다."))
	} else {
		body += tool.Baro_table([]string{"시각", "종류", "IP", tool.Baro_t("접속 프로그램"), tool.Baro_t("설명"), "관리"}, table_rows, "")
	}

	page_url := "/baro/security/login_log?page={}"
	if kind != "" {
		page_url += "&kind=" + url.QueryEscape(kind)
	}
	if mask {
		page_url += "&user=" + url.QueryEscape(target)
	}

	body += tool.Baro_paging(page, len(rows), tool.Baro_page_size, page_url)

	body += tool.Baro_card(tool.Baro_t("낯선 접속을 보면"),
		tool.Baro_list([]string{
			tool.Baro_t("<b>내가 아님</b> 을 누르면 그 기록을 신고로 표시하고, 세션을 모두 초기화하며, 알림을 남깁니다."),
			tool.Baro_t("곧바로 비밀번호를 바꾸세요. 비밀번호 변경은 openNAMU 본체의 설정 화면에서 합니다."),
			tool.Baro_t(`<a href="/baro/security/2fa">2단계 인증</a> 을 켜면 비밀번호가 새어도 로그인을 막을 수 있습니다.`),
		}),
	)

	if is_system {
		body += tool.Baro_card(tool.Baro_t("다른 사용자 기록 보기"),
			`<form class="bw_form" method="get" action="/baro/security/login_log">`+
				tool.Baro_input("user", tool.Baro_t("사용자 이름"), other, "", tool.Baro_t("system 권한으로 조회하면 IP 는 가려서 표시됩니다."))+
				tool.Baro_submit(tool.Baro_t("조회"), "")+
				`</form>`,
		)
	}

	return baro_sec_page(db, config, tool.Baro_t("접속 기록"), body)
}

// baro_sec_login_log_post : 접속 기록 화면의 쓰기 처리
func baro_sec_login_log_post(db *sql.DB, config tool.Config, real_ip string, values url.Values) string {
	if baro_sec_v(values, "op") != "deny" {
		return tool.Get_redirect("/baro/security/login_log")
	}

	id := baro_sec_v(values, "id")
	if id == "" {
		return tool.Get_redirect("/baro/security/login_log")
	}

	owner, ip := "", ""
	if !tool.QueryRow_DB(db, "select user, ip from baro_login_log where id = ?", []any{&owner, &ip}, id) {
		return tool.Get_redirect("/baro/security/login_log")
	}

	if owner != config.IP {
		return tool.Baro_deny(db, config, tool.Baro_t("내 접속 기록만 신고할 수 있습니다."))
	}

	// 기록 자체는 남기고, 신고 사실을 별도 줄로 추가한다 (기록을 지우지 않는다)
	tool.Exec_DB(
		db,
		"update baro_login_log set detail = ? where id = ?",
		"본인이 아니라고 신고됨 ("+tool.Baro_now()+")", id,
	)

	baro_sec_login_add(db, config.IP, real_ip, config.UserAgent, "denied", tool.Baro_t("접속 기록 ")+id+tool.Baro_t(" 을 본인이 아니라고 신고"))
	baro_sec_user_put(db, config.IP, "session_reset", tool.Baro_now())
	baro_sec_login_add(db, config.IP, real_ip, config.UserAgent, "session_reset", tool.Baro_t("본인 아님 신고에 따른 자동 세션 초기화"))

	tool.Baro_notify(
		db, config.IP, "system",
		tool.Baro_t("본인이 아닌 접속을 신고했습니다. 세션을 모두 초기화했습니다."),
		"/baro/security/session",
		tool.Baro_t("비밀번호를 바꾸고 2단계 인증을 켜세요."),
	)

	tool.Baro_audit(db, config, "sec_login_deny", "baro_login_log:"+id, tool.Baro_mask_ip(ip), "denied", "사용자가 본인 접속이 아니라고 신고")

	return tool.Get_redirect("/baro/security/login_log?done=1")
}

// ------------------------------------------------------------
//  /baro/security/session : 세션 강제 종료
// ------------------------------------------------------------

// View_baro_sec_session : 세션 강제 종료 화면
func View_baro_sec_session(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/security/session"); need {
		return wall
	}

	real_ip := baro_sec_real_ip(config, query)

	if values != nil {
		if baro_sec_v(values, "op") != "reset" {
			return tool.Get_redirect("/baro/security/session")
		}

		if ok, message := baro_sec_reauth_ok(db, config, values); !ok {
			return tool.Get_redirect("/baro/security/session?fail=" + url.QueryEscape(message))
		}

		before := baro_sec_user_get(db, config.IP, "session_reset")
		now := tool.Baro_now()

		baro_sec_user_put(db, config.IP, "session_reset", now)
		baro_sec_login_add(db, config.IP, real_ip, config.UserAgent, "session_reset", tool.Baro_t("사용자가 세션 강제 종료를 실행"))

		tool.Baro_notify(
			db, config.IP, "system",
			tool.Baro_t("세션을 모두 끊었습니다."),
			"/baro/security/session",
			tool.Baro_t("이 시각 이전에 발급된 세션은 바로위키 기준으로 무효입니다."),
		)

		tool.Baro_audit(db, config, "sec_session_reset", config.IP, before, now, "사용자 요청에 따른 세션 초기화")

		return tool.Get_redirect("/baro/security/session?done=1")
	}

	baro_sec_watch(db, config, real_ip)

	reset_at := baro_sec_user_get(db, config.IP, "session_reset")

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"", tool.Baro_t("세션")}})
	body += tool.Baro_hero(tool.Baro_t("세션 강제 종료"), tool.Baro_t("다른 기기에 남아 있는 로그인을 무효로 만듭니다."), "")
	body += baro_sec_tabs("/baro/security/session")

	if baro_sec_q(query, "done") != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("세션 초기화 시각을 새로 기록했습니다."))
	}
	if baro_sec_q(query, "fail") != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(baro_sec_q(query, "fail")))
	}

	body += tool.Baro_alert("warn",
		tool.Baro_t("<b>무엇이 일어나는지 정확히 알려드립니다.</b><br>")+
			tool.Baro_t("openNAMU 본체의 세션 키는 위키 전체가 공유하므로, 이 확장이 그것을 바꾸면 <b>모든 사용자</b>가 한꺼번에 로그아웃됩니다. ")+
			tool.Baro_t("그래서 바로위키는 본체 세션 키를 건드리지 않습니다.<br>")+
			tool.Baro_t("대신 <code>baro_user_set</code> 에 <b>세션 초기화 시각</b>을 기록하고, ")+
			tool.Baro_t("바로위키는 <b>그 시각 이후에 시작된 접속만 유효</b>한 것으로 봅니다. ")+
			tool.Baro_t("본체 화면(문서 편집 등)의 로그인 상태는 이 조치로 즉시 끊기지 않습니다. ")+
			tool.Baro_t("확실히 끊으려면 비밀번호를 바꾸세요."),
	)

	body += tool.Baro_card(tool.Baro_t("현재 상태"), tool.Baro_dl([][]string{
		{tool.Baro_t("세션 초기화 시각"), func() string {
			if reset_at == "" {
				return tool.Baro_badge(tool.Baro_t("기록 없음"), "muted")
			}

			return tool.HTML_escape(reset_at) + " (" + tool.HTML_escape(tool.Baro_time_ago(reset_at)) + ")"
		}()},
		{tool.Baro_t("이 접속의 IP 대역"), tool.HTML_escape(baro_sec_ip_view(real_ip))},
		{tool.Baro_t("2단계 인증"), func() string {
			if baro_sec_2fa_active(db, config.IP) {
				return tool.Baro_badge(tool.Baro_t("사용 중"), "ok")
			}

			return tool.Baro_badge(tool.Baro_t("미설정"), "danger")
		}()},
	}))

	// 최근 접속 지점 (대역 단위로 묶어서)
	seen := map[string]string{}
	order := []string{}
	for _, row := range baro_sec_login_rows(db, config.IP, "", 0, 200) {
		band := baro_sec_ip_band(row[2])
		if _, ok := seen[band]; !ok {
			order = append(order, band)
		}

		if seen[band] == "" {
			seen[band] = row[4]
		}
	}

	spot_rows := [][]string{}
	for _, band := range order {
		spot_rows = append(spot_rows, []string{
			tool.HTML_escape(band),
			tool.Baro_time_cell(seen[band]),
			func() string {
				if band != "" && band == baro_sec_ip_band(real_ip) {
					return tool.Baro_badge(tool.Baro_t("지금 이 접속"), "ok")
				}

				return tool.Baro_badge(tool.Baro_t("다른 대역"), "muted")
			}(),
		})
	}

	body += tool.Baro_card(tool.Baro_t("확인된 접속 지점"),
		tool.Baro_table([]string{tool.Baro_t("IP 대역"), tool.Baro_t("가장 최근 접속"), tool.Baro_t("비고")}, spot_rows, tool.Baro_t("아직 관측된 접속 지점이 없습니다.")),
	)

	form := tool.Baro_hidden("op", "reset")
	form += baro_sec_reauth_field(db, config)
	form += tool.Baro_submit(tool.Baro_t("내 세션 모두 끊기"), "danger")

	body += tool.Baro_danger_zone(tool.Baro_t("세션 모두 끊기"),
		tool.Baro_note(tool.Baro_t("이 조치는 되돌릴 수 없지만, 문서나 기여 기록에는 아무 영향을 주지 않습니다."))+
			tool.Baro_form("/baro/security/session", form),
	)

	return baro_sec_page(db, config, tool.Baro_t("세션 강제 종료"), body)
}
