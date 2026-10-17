package tool

import (
	"crypto/rand"
	"database/sql"
	"math/big"
	"strconv"
	"strings"
)

// ============================================================
//  바로 캡차 — 위키가 스스로 내는 문제
//
//  reCAPTCHA · hCaptcha · 턴스타일은 열쇠를 받아 와야 하고, 바깥 서비스에
//  접속자의 정보를 넘긴다. 열쇠가 없으면 지금까지는 캡차가 아예 없었다.
//  그래서 위키 안에서 그림 하나를 그려 묻는 캡차를 따로 둔다.
//
//  묻지 않는 사람
//    1. 운영진 (관리자 · 사측 관리자)
//    2. no_captcha 권한을 받은 사람
//    3. 바로샵 「캡차 면제 15일권」 을 산 사람 (기간 동안)
//    4. 바로샵 「캡차 1회 면제권」 이 남아 있는 사람 (한 번 쓰고 한 장 줄어든다)
//
//  누구에게 묻는지는 baro_set 의 captcha_mode 로 정한다.
//    off : 묻지 않는다   ip : 비로그인에게만 (기본)   all : 모두에게
// ============================================================

// Baro_captcha_len : 문제에 쓰는 글자 수
const Baro_captcha_len = 5

// Baro_captcha_alive_min : 문제 하나가 살아 있는 시간 (분)
const Baro_captcha_alive_min = 20

// baro_captcha_alphabet : 문제에 쓰는 글자
//
//	사람이 헷갈리는 것(0 O o · 1 I l · 2 Z · 5 S · 8 B)은 뺀다.
const baro_captcha_alphabet = "34679ACDEFGHJKLMNPQRTUVWXY"

// Baro_captcha_mode : 누구에게 묻는가 (off · ip · all)
func Baro_captcha_mode(db *sql.DB) string {
	switch Baro_get_def(db, "captcha_mode", "ip") {
	case "off":
		return "off"
	case "all":
		return "all"
	}

	return "ip"
}

// Baro_captcha_mode_list : 고를 수 있는 값 {값, 이름}
func Baro_captcha_mode_list() [][]string {
	return [][]string{
		{"off", Baro_t("묻지 않음")},
		{"ip", Baro_t("비로그인에게만")},
		{"all", Baro_t("모두에게")},
	}
}

// Baro_captcha_exempt : 캡차를 면제받는가. (면제 여부, 까닭)
//
//	까닭은 화면에 그대로 보여 줄 수 있는 말이다.
//	「1회 면제권」 은 여기서 쓰지 않는다. 실제로 검사할 때 한 장 쓴다.
func Baro_captcha_exempt(db *sql.DB, ip string) (bool, string) {
	auth := Get_user_auth_info(db, ip)

	switch {
	case auth["owner"]:
		return true, Baro_t("사측 관리자에게는 캡차를 묻지 않습니다.")
	case auth["treat_as_admin"], auth["admin"]:
		return true, Baro_t("관리자에게는 캡차를 묻지 않습니다.")
	case auth["no_captcha"]:
		return true, Baro_t("캡차 면제 권한을 가지고 있습니다.")
	}

	if until := Baro_mik_captcha_until(db, ip); until != "" {
		return true, Baro_t("캡차 면제권이 ") + Baro_date_only(until) + Baro_t(" 까지 살아 있습니다.")
	}

	return false, ""
}

// Baro_captcha_target : 이 사람이 캡차를 물어야 하는 무리에 드는가 (면제는 따지지 않는다)
func Baro_captcha_target(db *sql.DB, ip string) bool {
	switch Baro_captcha_mode(db) {
	case "off":
		return false
	case "all":
		return true
	}

	// ip : 비로그인에게만
	return !Baro_is_user(ip)
}

// Baro_captcha_need : 이 사람에게 캡차 문제를 내야 하는가
func Baro_captcha_need(db *sql.DB, ip string) bool {
	if !Baro_captcha_target(db, ip) {
		return false
	}

	free, _ := Baro_captcha_exempt(db, ip)

	return !free
}

// ------------------------------------------------------------
//  1회 면제권
// ------------------------------------------------------------

// Baro_captcha_once_left : 남은 1회 면제권 장수
func Baro_captcha_once_left(db *sql.DB, user string) int {
	if !Baro_is_user(user) {
		return 0
	}

	data := ""
	QueryRow_DB(db, "select data from baro_user_set where user = ? and name = 'mik_captcha_once'", []any{&data}, user)

	num := Str_to_int(data)
	if num < 0 {
		return 0
	}

	return num
}

// baro_captcha_once_put : 장수를 그대로 써 넣는다
func baro_captcha_once_put(db *sql.DB, user string, num int) {
	Exec_DB(db, "delete from baro_user_set where user = ? and name = 'mik_captcha_once'", user)

	if num > 0 {
		Exec_DB(
			db,
			"insert into baro_user_set (user, name, data) values (?, 'mik_captcha_once', ?)",
			user, strconv.Itoa(num),
		)
	}
}

// Baro_captcha_once_add : 1회 면제권을 늘린다
func Baro_captcha_once_add(db *sql.DB, user string, num int) int {
	if !Baro_is_user(user) || num <= 0 {
		return Baro_captcha_once_left(db, user)
	}

	next := Baro_captcha_once_left(db, user) + num
	baro_captcha_once_put(db, user, next)

	return next
}

// Baro_captcha_once_use : 1회 면제권 한 장을 쓴다 (없으면 false)
func Baro_captcha_once_use(db *sql.DB, user string) bool {
	left := Baro_captcha_once_left(db, user)
	if left <= 0 {
		return false
	}

	baro_captcha_once_put(db, user, left-1)

	return true
}

// ------------------------------------------------------------
//  문제 만들기 · 맞히기
// ------------------------------------------------------------

// Baro_captcha_new : 문제 하나를 만들어 그 이름표를 돌려준다
func Baro_captcha_new(db *sql.DB, who string) string {
	baro_captcha_sweep(db)

	answer := baro_captcha_answer_new()
	if answer == "" {
		return ""
	}

	code := Get_random_key(32)
	if code == "" {
		return ""
	}

	Exec_DB(
		db,
		"insert into baro_captcha (id, code, answer, who, status, date) values (?, ?, ?, ?, 'open', ?)",
		Baro_next_id(db, "baro_captcha"), code, answer, who, Baro_now(),
	)

	return code
}

// Baro_captcha_answer : 이름표에 딸린 답 ("" 면 없거나 철 지난 문제)
//
//	그림을 그릴 때만 쓴다. 바깥으로 내보내지 않는다.
func Baro_captcha_answer(db *sql.DB, code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}

	answer, date := "", ""
	if !QueryRow_DB(
		db,
		"select answer, date from baro_captcha where code = ? and status = 'open'",
		[]any{&answer, &date}, code,
	) {
		return ""
	}

	if Baro_is_past(Baro_time_add(date, 0, 0, Baro_captcha_alive_min)) {
		return ""
	}

	return answer
}

// Baro_captcha_verify : 적어 낸 답이 맞는가
//
//	맞든 틀리든 그 문제는 쓴 것으로 치고 지운다. (같은 그림을 두 번 낼 수 없게)
func Baro_captcha_verify(db *sql.DB, code string, typed string) bool {
	answer := Baro_captcha_answer(db, code)

	// 문제 자체는 한 번 쓰면 사라진다.
	if code = strings.TrimSpace(code); code != "" {
		Exec_DB(db, "delete from baro_captcha where code = ?", code)
	}

	if answer == "" {
		return false
	}

	typed = strings.ToUpper(strings.TrimSpace(typed))
	typed = strings.ReplaceAll(typed, " ", "")

	return typed != "" && typed == answer
}

// baro_captcha_sweep : 철 지난 문제 치우기
func baro_captcha_sweep(db *sql.DB) {
	Exec_DB(
		db,
		"delete from baro_captcha where date < ?",
		Baro_time_add(Baro_now(), 0, 0, -Baro_captcha_alive_min*3),
	)
}

// ------------------------------------------------------------
//  본체 캡차 검사와 이어 붙이기
// ------------------------------------------------------------

// Baro_captcha_field : 폼에 실어 보내는 값의 머리말
//
//	본체 캡차 검사는 값을 하나만 받으므로,
//	「이름표와 적어 낸 답」 을 한 줄로 묶어 보낸다.
const Baro_captcha_field = "bwcap"

// Baro_captcha_pack : 이름표와 답을 한 줄로 묶는다
func Baro_captcha_pack(code string, typed string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}

	return Baro_captcha_field + ":" + code + ":" + typed
}

// Baro_captcha_unpack : 묶은 값을 도로 푼다. (이름표, 적어 낸 답, 바로 캡차인가)
func Baro_captcha_unpack(data string) (string, string, bool) {
	head, rest, ok := strings.Cut(data, ":")
	if !ok || head != Baro_captcha_field {
		return "", "", false
	}

	code, typed, _ := strings.Cut(rest, ":")

	return code, typed, true
}

// baro_captcha_answer_new : 헷갈리지 않는 글자만 골라 답 하나를 만든다
func baro_captcha_answer_new() string {
	size := big.NewInt(int64(len(baro_captcha_alphabet)))
	out := make([]byte, Baro_captcha_len)

	for i := range out {
		pick, err := rand.Int(rand.Reader, size)
		if err != nil {
			return ""
		}

		out[i] = baro_captcha_alphabet[pick.Int64()]
	}

	return string(out)
}

// ------------------------------------------------------------
//  한자리에서 내리는 결정
// ------------------------------------------------------------

// 캡차 처리 갈래
const (
	// Baro_captcha_gate_pass : 묻지 않는다
	Baro_captcha_gate_pass = "pass"
	// Baro_captcha_gate_once : 1회 면제권으로 넘긴다 (검사할 때 한 장 쓴다)
	Baro_captcha_gate_once = "once"
	// Baro_captcha_gate_baro : 바로 캡차를 낸다
	Baro_captcha_gate_baro = "baro"
	// Baro_captcha_gate_out : 바깥 캡차(reCAPTCHA 등)를 낸다
	Baro_captcha_gate_out = "out"
)

// baro_captcha_outside_on : 바깥 캡차 열쇠가 꽂혀 있는가
func baro_captcha_outside_on(db *sql.DB) bool {
	pub_key, sec_key := "", ""
	QueryRow_DB(db, `select data from other where name = "recaptcha"`, []any{&pub_key})
	QueryRow_DB(db, `select data from other where name = "sec_re"`, []any{&sec_key})

	return pub_key != "" && sec_key != ""
}

// Baro_captcha_gate : 이 사람에게 캡차를 어떻게 할지 정한다
//
//	그리는 자리와 검사하는 자리가 같은 답을 보게 한자리에 모아 둔다.
//	바깥 캡차 열쇠가 꽂혀 있으면 지금까지처럼 그쪽을 쓰고,
//	없으면 위키가 스스로 내는 바로 캡차를 쓴다.
func Baro_captcha_gate(db *sql.DB, ip string) string {
	if free, _ := Baro_captcha_exempt(db, ip); free {
		return Baro_captcha_gate_pass
	}

	if baro_captcha_outside_on(db) {
		if Check_acl(db, "", "", "recaptcha", ip) {
			return Baro_captcha_gate_pass
		}

		if Baro_captcha_once_left(db, ip) > 0 {
			return Baro_captcha_gate_once
		}

		return Baro_captcha_gate_out
	}

	if !Baro_captcha_target(db, ip) {
		return Baro_captcha_gate_pass
	}

	if Baro_captcha_once_left(db, ip) > 0 {
		return Baro_captcha_gate_once
	}

	return Baro_captcha_gate_baro
}

// baro_captcha_answer_check : 폼으로 온 값 하나로 바로 캡차를 검사한다
func baro_captcha_answer_check(db *sql.DB, response string) bool {
	code, typed, ok := Baro_captcha_unpack(response)
	if !ok {
		return false
	}

	return Baro_captcha_verify(db, code, typed)
}

// ------------------------------------------------------------
//  화면
// ------------------------------------------------------------

// Baro_captcha_ui : 바로 캡차 칸 (그림 · 적는 칸 · 새 문제 단추)
func Baro_captcha_ui(db *sql.DB, ip string) string {
	code := Baro_captcha_new(db, ip)
	if code == "" {
		// 문제를 못 만들었으면 칸을 비워 둔다. 검사하는 쪽에서 막는다.
		return ""
	}

	url := "/captcha/" + Url_parser(code) + ".png"

	html := `<div class="bw_captcha">` + baro_captcha_style()
	html += `<p class="bw_captcha_q">` + Baro_t_esc("그림에 보이는 글자를 그대로 적어 주세요.") + `</p>`
	html += `<div class="bw_captcha_row">`
	html += `<img class="bw_captcha_img" id="bw_captcha_img" src="` + HTML_escape(url) + `" width="220" height="70" alt="` +
		Baro_t_esc("캡차 그림") + `">`
	html += `<button type="button" class="bw_captcha_again" onclick="bw_captcha_again();">` +
		Baro_t_esc("다른 문제") + `</button>`
	html += `</div>`
	html += `<input type="hidden" name="bw_captcha_code" id="bw_captcha_code" value="` + HTML_escape(code) + `">`
	html += `<input class="__ON_INPUT__ bw_captcha_in" type="text" name="bw_captcha" autocomplete="off" ` +
		`autocapitalize="characters" spellcheck="false" maxlength="12" placeholder="` +
		Baro_t_esc("보이는 글자") + `">`
	html += `<p class="bw_captcha_note">` + Baro_t_esc("대문자와 소문자를 가리지 않습니다.") + `</p>`
	html += `</div>` + baro_captcha_script()

	return html
}

// Baro_captcha_once_ui : 1회 면제권으로 넘어간다는 안내
func Baro_captcha_once_ui(db *sql.DB, ip string) string {
	left := Baro_captcha_once_left(db, ip)

	return `<div class="bw_captcha">` + baro_captcha_style() +
		`<p class="bw_captcha_q">` +
		Baro_t_esc("캡차 1회 면제권으로 이번 한 번은 묻지 않습니다.") + ` (` +
		Baro_t_esc("남은 장수 ") + HTML_escape(Baro_itoa(left)) + Baro_t_esc("장") + `)</p>` +
		`<p class="bw_captcha_note">` +
		Baro_t_esc("보내기를 누르는 순간 한 장이 줄어듭니다. 그냥 나가면 줄지 않습니다.") +
		`</p></div>`
}

// baro_captcha_script : 「다른 문제」 단추가 하는 일
func baro_captcha_script() string {
	return `<script>
function bw_captcha_again() {
    var img = document.getElementById('bw_captcha_img');
    var code = document.getElementById('bw_captcha_code');
    if(img === null || code === null) { return; }

    var req = new XMLHttpRequest();
    req.open('GET', '/captcha/new', true);
    req.onload = function() {
        if(req.status !== 200) { return; }

        var next = req.responseText.trim();
        if(next === '') { return; }

        code.value = next;
        img.src = '/captcha/' + encodeURIComponent(next) + '.png?t=' + Date.now();
    };
    req.send();
}
</script>`
}

// baro_captcha_style : 캡차 칸 모양
func baro_captcha_style() string {
	return `<style>
.bw_captcha { margin: 10px 0; }
.bw_captcha_q { margin: 0 0 6px; font-size: 13px; }
.bw_captcha_row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.bw_captcha_img { border: 1px solid rgba(128,128,128,0.4); border-radius: 6px; max-width: 100%; height: auto; }
.bw_captcha_again { padding: 4px 10px; font-size: 12px; cursor: pointer; }
.bw_captcha_in { display: block; margin-top: 8px; width: 220px; max-width: 100%;
    letter-spacing: 3px; text-transform: uppercase; font-size: 16px; }
.bw_captcha_note { margin: 6px 0 0; font-size: 12px; opacity: 0.75; }
</style>`
}
