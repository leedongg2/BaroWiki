package route

import (
	"database/sql"
	"net/url"

	"opennamu/route/tool"
)

// ============================================================
//  로그인 2단계 인증 관문 (17-1)
//
//  2단계 인증을 켜 두었으면 아이디·비밀번호만으로는 로그인이 끝나지 않는다.
//  비밀번호가 맞은 다음에 인증 앱 코드(또는 백업 코드)를 한 번 더 확인한다.
//
//  비밀번호가 틀렸을 때는 2단계 인증 화면을 보여 주지 않는다.
//  (그 계정이 2단계 인증을 쓰는지 아닌지가 새어 나가지 않게 하려는 것)
// ============================================================

// View_baro_login2fa_gate : 로그인 전에 2단계 인증을 확인한다
//
//	돌려주는 값이 (화면, true) 이면 로그인을 멈추고 그 화면을 보여 준다.
//	(", false) 이면 그대로 로그인을 이어 간다.
func View_baro_login2fa_gate(config tool.Config, id string, password string, code string) (string, bool) {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if id == "" || password == "" {
		return "", false
	}

	// 비밀번호부터 맞아야 한다. 틀리면 원래 로그인 화면이 처리하게 넘긴다.
	if !tool.Password_check(db, id, password) {
		return "", false
	}

	if !baro_sec_2fa_active(db, id) {
		return "", false
	}

	// 코드를 아직 안 냈으면 물어본다
	if code == "" {
		return baro_login2fa_form(db, config, id, password, ""), true
	}

	// 코드 맞히기 시도 제한 (17-2)
	if !tool.Baro_rate_ok(db, "login_2fa", id, 10, 300) {
		return baro_login2fa_form(db, config, id, password,
			tool.Baro_t("코드 확인을 너무 자주 시도했습니다. 5분 뒤에 다시 해 주세요.")), true
	}

	secret, _, _, _ := baro_sec_2fa_row(db, id)

	if baro_sec_totp_ok(secret, code) {
		baro_login2fa_log(db, config, id, "totp")

		return "", false
	}

	if baro_sec_backup_use(db, id, code) {
		baro_login2fa_log(db, config, id, "backup")

		return "", false
	}

	return baro_login2fa_form(db, config, id, password,
		tool.Baro_t("코드가 맞지 않습니다. 인증 앱의 지금 코드나 백업 코드를 넣어 주세요.")), true
}

// baro_login2fa_log : 2단계 인증 통과 기록
func baro_login2fa_log(db *sql.DB, config tool.Config, id string, kind string) {
	tool.Exec_DB(
		db,
		"insert into baro_login_log (id, user, ip, ua, date, kind, detail) values (?, ?, ?, ?, ?, '2fa', ?)",
		tool.Baro_next_id(db, "baro_login_log"), id, config.IP,
		tool.Baro_trunc(config.UserAgent, 300), tool.Baro_now(), kind,
	)
}

// baro_login2fa_form : 코드 입력 화면
func baro_login2fa_form(db *sql.DB, config tool.Config, id string, password string, message string) string {
	body := ""

	if message != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(message))
	} else {
		body += tool.Baro_alert("info",
			tool.Baro_t("비밀번호는 확인됐습니다. 마지막으로 <b>인증 앱의 여섯 자리 코드</b>를 넣어 주세요."))
	}

	form := tool.Baro_hidden("id", id)
	form += tool.Baro_hidden("password", password)
	form += tool.Baro_input("baro_2fa_code", tool.Baro_t("인증 코드"), "", tool.Baro_t("여섯 자리 숫자 또는 백업 코드"),
		tool.Baro_t("인증 앱의 코드는 30초마다 바뀝니다. 앱을 못 쓰면 백업 코드를 넣으세요. (백업 코드는 한 번 쓰면 사라집니다)"))
	form += tool.Baro_submit(tool.Baro_t("확인하고 로그인"), "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("2단계 인증"),
		tool.HTML_escape(id)+tool.Baro_t(" 계정은 2단계 인증을 쓰고 있습니다."),
		tool.Baro_form("/login", form),
		tool.Baro_t("인증 앱과 백업 코드를 모두 잃어버렸다면 위키 운영자에게 문의해 해제를 요청하세요."),
	)

	return tool.Baro_page(db, config, tool.Baro_t("2단계 인증"), body, [][]any{{"login", tool.Baro_t("로그인으로")}})
}

// baro_login2fa_code : 폼에서 코드 값을 꺼낸다
func baro_login2fa_code(values url.Values) string {
	if values == nil {
		return ""
	}

	return values.Get("baro_2fa_code")
}
