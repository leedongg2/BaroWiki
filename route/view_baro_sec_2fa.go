package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  17-1 (2) 2단계 인증 — /baro/security/2fa
//
//  외부 라이브러리 없이 표준 라이브러리만으로 TOTP 를 구현한다.
//  QR 이미지 대신 otpauth:// 주소와 수동 입력용 키를 보여준다.
// ============================================================

// baro_sec_2fa_codes_page : 백업 코드를 한 번만 보여주는 화면
//
//	백업 코드는 해시로만 저장하므로 이 화면을 벗어나면 다시 볼 수 없다.
//	그래서 이 응답만 예외적으로 넘겨보내기(redirect) 대신 직접 화면을 그린다.
func baro_sec_2fa_codes_page(db *sql.DB, config tool.Config, title string, message string, codes []string) string {
	items := []string{}
	for _, code := range codes {
		items = append(items, tool.Baro_code(code))
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"", tool.Baro_t("백업 코드")}})
	body += tool.Baro_hero(title, tool.Baro_t("이 화면을 벗어나면 다시 볼 수 없습니다."), "")
	body += baro_sec_tabs("/baro/security/2fa")
	body += tool.Baro_alert("ok", message)
	body += tool.Baro_alert("danger",
		tool.Baro_t("<b>지금 바로 안전한 곳에 옮겨 적으세요.</b> 백업 코드는 해시로만 저장되므로 위키도 원본을 모릅니다. ")+
			tool.Baro_t("코드 하나는 한 번만 쓸 수 있고, 쓰면 사라집니다."),
	)
	body += tool.Baro_card(tool.Baro_t("백업 코드 10개"), tool.Baro_grid(items, 2))
	body += tool.Baro_button("/baro/security/2fa", tool.Baro_t("확인했습니다"), "primary")

	return baro_sec_page(db, config, title, body)
}

// baro_sec_2fa_fail : 인증 실패를 기록한다
func baro_sec_2fa_fail(db *sql.DB, config tool.Config, real_ip string, detail string) {
	baro_sec_login_add(db, config.IP, real_ip, config.UserAgent, "2fa_fail", detail)
}

// View_baro_sec_2fa : 2단계 인증 설정 화면
func View_baro_sec_2fa(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/security/2fa"); need {
		return wall
	}

	if values != nil {
		return baro_sec_2fa_post(db, config, baro_sec_real_ip(config, query), values)
	}

	baro_sec_watch(db, config, baro_sec_real_ip(config, query))

	secret, backup, status, date := baro_sec_2fa_row(db, config.IP)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"", tool.Baro_t("2단계 인증")}})
	body += tool.Baro_hero(
		tool.Baro_t("2단계 인증"),
		tool.Baro_t("비밀번호가 새어도 계정을 지킬 수 있게, 30초마다 바뀌는 코드를 함께 요구합니다."),
		"",
	)
	body += baro_sec_tabs("/baro/security/2fa")

	if baro_sec_q(query, "done") != "" {
		body += tool.Baro_alert("ok", tool.HTML_escape(baro_sec_q(query, "done")))
	}
	if baro_sec_q(query, "fail") != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(baro_sec_q(query, "fail")))
	}

	if baro_sec_flag(db, "sec_admin_2fa_force", false) && tool.Baro_is_admin(db, config.IP) && status != "active" {
		body += tool.Baro_alert("warn",
			tool.Baro_t("이 위키는 관리자에게 2단계 인증을 요구합니다. 아직 설정하지 않았습니다."),
		)
	}

	state_badge := tool.Baro_badge(tool.Baro_t("미설정"), "danger")
	switch status {
	case "pending":
		state_badge = tool.Baro_badge(tool.Baro_t("설정 중 (코드 확인 필요)"), "warn")
	case "active":
		state_badge = tool.Baro_badge(tool.Baro_t("사용 중"), "ok")
	}

	// 2단계 인증은 이 화면 안에서만 쓰이는 것이 아니라 로그인 자체를 막는다
	if status == "active" {
		body += tool.Baro_alert("info",
			tool.Baro_t("이 계정은 <b>로그인할 때도</b> 2단계 인증을 요구합니다. 아이디와 비밀번호가 맞아도 ")+
				tool.Baro_t("인증 앱의 6자리 코드나 <b>백업 코드</b>를 한 번 더 넣어야 로그인이 끝납니다. ")+
				tool.Baro_t("바로위키 안에서 권한을 바꿀 때 하는 재인증에도 같은 코드를 씁니다."),
		)
	} else {
		body += tool.Baro_alert("info",
			tool.Baro_t("2단계 인증을 켜면 <b>로그인 자체가 막힙니다.</b> 아이디와 비밀번호가 맞아도 ")+
				tool.Baro_t("인증 앱의 6자리 코드나 백업 코드를 한 번 더 넣어야 로그인이 끝납니다."),
		)
	}

	login_gate := tool.Baro_badge(tool.Baro_t("적용 안 됨"), "muted")
	if status == "active" {
		login_gate = tool.Baro_badge(tool.Baro_t("로그인 시 코드 요구"), "ok")
	}

	body += tool.Baro_card(tool.Baro_t("현재 상태"), tool.Baro_kv_grid([][]string{
		{"상태", state_badge},
		{tool.Baro_t("로그인에 적용"), login_gate},
		{tool.Baro_t("방식"), tool.Baro_t("TOTP (HMAC-SHA1 · 6자리 · 30초)")},
		{tool.Baro_t("남은 백업 코드"), tool.Baro_itoa(baro_sec_backup_left(backup)) + tool.Baro_t("개")},
		{tool.Baro_t("마지막 변경"), func() string {
			if date == "" {
				return tool.Baro_badge(tool.Baro_t("기록 없음"), "muted")
			}

			return tool.HTML_escape(date)
		}()},
	}))

	switch status {
	case "":
		start_body := tool.Baro_list([]string{
			tool.Baro_t("인증 앱(시간 기반 OTP 를 지원하는 아무 앱)을 준비합니다."),
			tool.Baro_t("아래 <b>설정 시작</b> 을 누르면 이 계정 전용 비밀키가 만들어집니다."),
			tool.Baro_t("앱에 비밀키를 넣고, 앱이 보여주는 6자리 코드를 한 번 입력하면 켜집니다."),
			tool.Baro_t("마지막으로 백업 코드 10개를 받아 안전한 곳에 보관합니다."),
			tool.Baro_t("켠 다음부터는 <b>로그인할 때마다</b> 코드를 한 번 더 물어봅니다."),
		})
		start_body += tool.Baro_form("/baro/security/2fa",
			tool.Baro_hidden("op", "start")+tool.Baro_submit(tool.Baro_t("설정 시작"), "primary"),
		)

		body += tool.Baro_card(tool.Baro_t("2단계 인증 켜기"), start_body)
	case "pending":
		otpauth := baro_sec_otpauth(db, config.IP, secret)

		setup_body := tool.Baro_alert("info",
			tool.Baro_t("QR 이미지는 외부 라이브러리 없이 만들 수 없어 제공하지 않습니다. ")+
				tool.Baro_t("아래 주소를 인증 앱에 붙여 넣거나, 수동 입력용 키를 직접 입력하세요."),
		)
		setup_body += tool.Baro_section(tool.Baro_t("otpauth 주소"), tool.Baro_code(otpauth))
		setup_body += tool.Baro_section(tool.Baro_t("수동 입력용 키"), tool.Baro_code(baro_sec_2fa_secret_view(secret)))
		setup_body += tool.Baro_note(tool.Baro_t("계정 이름 : ") + config.IP + tool.Baro_t(" / 발급자 : ") + baro_sec_wiki_name(db) + tool.Baro_t(" / 종류 : 시간 기반(TOTP) / 자릿수 : 6 / 주기 : 30초"))

		setup_body += tool.Baro_form("/baro/security/2fa",
			tool.Baro_hidden("op", "confirm")+
				tool.Baro_input("code", tool.Baro_t("앱이 보여주는 6자리 코드"), "", "000000", tool.Baro_t("코드가 맞아야 2단계 인증이 켜집니다."))+
				tool.Baro_submit(tool.Baro_t("코드 확인하고 켜기"), "primary"),
		)

		setup_body += tool.Baro_form("/baro/security/2fa",
			tool.Baro_hidden("op", "cancel")+tool.Baro_submit(tool.Baro_t("설정 취소"), ""),
		)

		body += tool.Baro_card(tool.Baro_t("인증 앱에 등록하기"), setup_body)
	case "active":
		test_body := tool.Baro_note(tool.Baro_t("지금 앱에 뜬 코드를 넣어, 시계가 어긋나지 않았는지 확인할 수 있습니다."))
		test_body += tool.Baro_form("/baro/security/2fa",
			tool.Baro_hidden("op", "test")+
				tool.Baro_input("code", tool.Baro_t("인증 코드"), "", "000000", "")+
				tool.Baro_submit(tool.Baro_t("코드 시험"), ""),
		)
		body += tool.Baro_card(tool.Baro_t("코드 시험"), test_body)

		backup_body := tool.Baro_note(tool.Baro_t("남은 백업 코드가 부족하면 새로 10개를 만들 수 있습니다. 새로 만들면 이전 코드는 모두 무효가 됩니다."))
		backup_body += tool.Baro_form("/baro/security/2fa",
			tool.Baro_hidden("op", "backup")+
				tool.Baro_input_type("code", tool.Baro_t("인증 코드"), "", "password", tool.Baro_t("본인 확인을 위해 앱의 6자리 코드를 입력합니다."))+
				tool.Baro_submit(tool.Baro_t("백업 코드 새로 만들기"), ""),
		)
		body += tool.Baro_card(tool.Baro_t("백업 코드 재발급"), backup_body)

		body += tool.Baro_danger_zone("2단계 인증 해제",
			tool.Baro_note(tool.Baro_t("해제하려면 인증 코드나 백업 코드가 필요합니다. 코드를 모두 잃어버렸다면 위키 관리자에게 문의하세요."))+
				tool.Baro_form("/baro/security/2fa",
					tool.Baro_hidden("op", "disable")+
						tool.Baro_input_type("code", tool.Baro_t("인증 코드 또는 백업 코드"), "", "password", "")+
						tool.Baro_submit(tool.Baro_t("2단계 인증 끄기"), "danger"),
				),
		)
	}

	body += tool.Baro_card(tool.Baro_t("알아두면 좋은 것"),
		tool.Baro_list([]string{
			tool.Baro_t("코드는 <b>기기 시계</b>를 기준으로 만들어집니다. 코드가 계속 틀리면 휴대전화 시간을 자동으로 맞추세요."),
			tool.Baro_t("이 위키는 앞뒤 30초까지 여유를 두고 코드를 받아 줍니다."),
			tool.Baro_t("백업 코드는 인증 앱을 잃어버렸을 때를 위한 것입니다. 사진첩이 아니라 비밀번호 관리 프로그램이나 종이에 보관하세요."),
			tool.Baro_t("<b>로그인 화면에서도 백업 코드를 쓸 수 있습니다.</b> 6자리 코드 자리에 백업 코드를 그대로 넣으면 됩니다. (한 번 쓰면 사라집니다)"),
			tool.Baro_t("인증 앱과 백업 코드를 모두 잃어버리면 로그인할 수 없습니다. 그때는 위키 운영자에게 해제를 요청하세요."),
			tool.Baro_t("인증 시도가 잦으면 <a href=\"/baro/security\">시도 제한</a> 에 걸려 잠시 막힙니다. 로그인 화면의 코드 확인도 5분에 10번까지만 시도할 수 있습니다."),
		}),
	)

	return baro_sec_page(db, config, tool.Baro_t("2단계 인증"), body)
}

// baro_sec_2fa_post : 2단계 인증 화면의 쓰기 처리
func baro_sec_2fa_post(db *sql.DB, config tool.Config, real_ip string, values url.Values) string {
	user := config.IP
	op := baro_sec_v(values, "op")
	code := baro_sec_v(values, "code")

	secret, backup, status, _ := baro_sec_2fa_row(db, user)

	switch op {
	case "start":
		if status == "active" {
			return tool.Get_redirect("/baro/security/2fa?fail=" + url.QueryEscape(tool.Baro_t("이미 2단계 인증을 쓰고 있습니다.")))
		}

		baro_sec_2fa_save(db, user, baro_sec_new_secret(), "[]", "pending")
		tool.Baro_audit(db, config, "sec_2fa_start", user, status, "pending", "2단계 인증 설정 시작")

		return tool.Get_redirect("/baro/security/2fa?done=" + url.QueryEscape(tool.Baro_t("비밀키를 만들었습니다. 인증 앱에 등록한 뒤 코드를 확인하세요.")))
	case "cancel":
		if status != "pending" {
			return tool.Get_redirect("/baro/security/2fa")
		}

		baro_sec_2fa_drop(db, user)
		tool.Baro_audit(db, config, "sec_2fa_cancel", user, "pending", "", "2단계 인증 설정 취소")

		return tool.Get_redirect("/baro/security/2fa?done=" + url.QueryEscape(tool.Baro_t("설정을 취소했습니다.")))
	case "confirm":
		if status != "pending" {
			return tool.Get_redirect("/baro/security/2fa?fail=" + url.QueryEscape(tool.Baro_t("설정 중인 2단계 인증이 없습니다.")))
		}

		if !baro_sec_login_rate_ok(db, user) {
			return tool.Get_redirect("/baro/security/2fa?fail=" + url.QueryEscape(tool.Baro_t("인증 시도가 너무 잦습니다. 잠시 뒤에 다시 시도하세요.")))
		}

		if !baro_sec_totp_ok(secret, code) {
			baro_sec_2fa_fail(db, config, real_ip, tool.Baro_t("2단계 인증 설정 확인 실패"))

			return tool.Get_redirect("/baro/security/2fa?fail=" + url.QueryEscape(tool.Baro_t("코드가 맞지 않습니다. 앱의 시계가 정확한지 확인하세요.")))
		}

		baro_sec_login_rate_reset(db, user)

		codes, hashes := baro_sec_backup_make()
		baro_sec_2fa_save(db, user, secret, tool.Baro_json_enc(hashes), "active")

		baro_sec_login_add(db, user, real_ip, config.UserAgent, "2fa_on", "2단계 인증 사용 시작")
		tool.Baro_notify(db, user, "system", "2단계 인증을 켰습니다.", "/baro/security/2fa", "백업 코드 10개를 안전한 곳에 보관하세요.")
		tool.Baro_audit(db, config, "sec_2fa_enable", user, "pending", "active", "2단계 인증 사용 시작")

		return baro_sec_2fa_codes_page(db, config, tool.Baro_t("2단계 인증을 켰습니다"),
			tool.Baro_t("이제 로그인할 때 아이디·비밀번호 다음으로 인증 앱의 6자리 코드가 필요합니다. 앱을 못 쓸 때는 아래 백업 코드를 대신 넣으면 됩니다."), codes)
	case "test":
		if status != "active" {
			return tool.Get_redirect("/baro/security/2fa")
		}

		if !baro_sec_login_rate_ok(db, user) {
			return tool.Get_redirect("/baro/security/2fa?fail=" + url.QueryEscape(tool.Baro_t("인증 시도가 너무 잦습니다. 잠시 뒤에 다시 시도하세요.")))
		}

		if !baro_sec_totp_ok(secret, code) {
			baro_sec_2fa_fail(db, config, real_ip, tool.Baro_t("코드 시험 실패"))

			return tool.Get_redirect("/baro/security/2fa?fail=" + url.QueryEscape(tool.Baro_t("코드가 맞지 않습니다.")))
		}

		baro_sec_login_rate_reset(db, user)

		return tool.Get_redirect("/baro/security/2fa?done=" + url.QueryEscape(tool.Baro_t("코드가 정상입니다.")))
	case "backup":
		if status != "active" {
			return tool.Get_redirect("/baro/security/2fa")
		}

		if !baro_sec_login_rate_ok(db, user) {
			return tool.Get_redirect("/baro/security/2fa?fail=" + url.QueryEscape(tool.Baro_t("인증 시도가 너무 잦습니다. 잠시 뒤에 다시 시도하세요.")))
		}

		if !baro_sec_totp_ok(secret, code) {
			baro_sec_2fa_fail(db, config, real_ip, tool.Baro_t("백업 코드 재발급 실패"))

			return tool.Get_redirect("/baro/security/2fa?fail=" + url.QueryEscape(tool.Baro_t("코드가 맞지 않습니다.")))
		}

		baro_sec_login_rate_reset(db, user)

		codes, hashes := baro_sec_backup_make()
		baro_sec_2fa_save(db, user, secret, tool.Baro_json_enc(hashes), "active")

		baro_sec_login_add(db, user, real_ip, config.UserAgent, "backup_new", tool.Baro_t("백업 코드 재발급"))
		tool.Baro_audit(db, config, "sec_2fa_backup", user,
			tool.Baro_itoa(baro_sec_backup_left(backup))+"개 남아 있었음", "10개 새로 발급", "백업 코드 재발급")

		return baro_sec_2fa_codes_page(db, config, tool.Baro_t("백업 코드를 새로 만들었습니다"), tool.Baro_t("이전 백업 코드는 모두 무효가 됐습니다."), codes)
	case "disable":
		if status == "" {
			return tool.Get_redirect("/baro/security/2fa")
		}

		if !baro_sec_login_rate_ok(db, user) {
			return tool.Get_redirect("/baro/security/2fa?fail=" + url.QueryEscape(tool.Baro_t("인증 시도가 너무 잦습니다. 잠시 뒤에 다시 시도하세요.")))
		}

		if status == "active" && !baro_sec_2fa_check(db, user, code) {
			baro_sec_2fa_fail(db, config, real_ip, tool.Baro_t("2단계 인증 해제 실패"))

			return tool.Get_redirect("/baro/security/2fa?fail=" + url.QueryEscape(tool.Baro_t("인증 코드도 백업 코드도 맞지 않습니다.")))
		}

		baro_sec_login_rate_reset(db, user)
		baro_sec_2fa_drop(db, user)

		baro_sec_login_add(db, user, real_ip, config.UserAgent, "2fa_off", "2단계 인증 해제")
		tool.Baro_notify(db, user, "system", "2단계 인증을 껐습니다.", "/baro/security/2fa", "본인이 한 조치가 아니라면 즉시 비밀번호를 바꾸세요.")
		tool.Baro_audit(db, config, "sec_2fa_disable", user, status, "", "2단계 인증 해제")

		if baro_sec_flag(db, "sec_admin_2fa_force", false) && tool.Baro_is_admin(db, user) {
			return tool.Get_redirect("/baro/security/2fa?fail=" +
				url.QueryEscape(tool.Baro_t("해제했지만, 이 위키는 관리자에게 2단계 인증을 요구합니다. 다시 설정하세요.")))
		}

		return tool.Get_redirect("/baro/security/2fa?done=" + url.QueryEscape("2단계 인증을 껐습니다."))
	}

	return tool.Get_redirect("/baro/security/2fa")
}

// baro_sec_2fa_missing_admin : 강제 설정이 켜졌을 때 2단계 인증이 없는 관리자 목록 문자열
func baro_sec_2fa_missing_admin(db *sql.DB) []string {
	out := []string{}
	for _, admin := range baro_sec_admin_list(db) {
		if !baro_sec_2fa_active(db, admin) {
			out = append(out, admin)
		}
	}

	return out
}

// baro_sec_2fa_summary : 다른 화면에서 쓰는 짧은 요약
func baro_sec_2fa_summary(db *sql.DB) string {
	total := len(baro_sec_admin_list(db))
	missing := len(baro_sec_2fa_missing_admin(db))

	if total == 0 {
		return tool.Baro_t("확인된 관리자 계정이 없습니다.")
	}

	return tool.Baro_t("관리자 ") + tool.Baro_itoa(total) + tool.Baro_t("명 중 ") + tool.Baro_itoa(total-missing) + tool.Baro_t("명이 2단계 인증 사용 중")
}

// baro_sec_2fa_secret_view : 비밀키를 화면에 보여줄 때 쓰는 안전 문자열
func baro_sec_2fa_secret_view(secret string) string {
	if strings.TrimSpace(secret) == "" {
		return "-"
	}

	return baro_sec_secret_pretty(secret)
}
