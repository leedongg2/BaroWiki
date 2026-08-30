package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  17-2 악용 방지 — /baro/security/abuse
//
//   요청 속도 제한 · 봇 차단 · 스팸 필터 · 대량 계정 생성 제한 ·
//   편집 폭주 제한 · 파일 검사 · 위험 링크 검사 · 관리자 작업 추가 확인
//
//  모든 설정은 baro_set 에 저장하고 켜고 끌 수 있다.
// ============================================================

// ------------------------------------------------------------
//  설정 기본값
// ------------------------------------------------------------

// baro_sec_abuse_bot_default : 봇 차단 기본 목록 (접속 프로그램 이름 조각)
const baro_sec_abuse_bot_default = "curl\nwget\npython-requests\npython-urllib\nscrapy\nhttpclient\nheadlesschrome\nphantomjs\nlibwww-perl"

// baro_sec_abuse_spam_default : 스팸 낱말 기본 목록
const baro_sec_abuse_spam_default = "카지노\n토토사이트\n먹튀검증\n출장마사지\n비아그라\n대출상담\nviagra\ncasino\nfree money\n주식리딩방"

// baro_sec_abuse_link_default : 위험·단축 주소 기본 목록
const baro_sec_abuse_link_default = "bit.ly\ntinyurl.com\ngoo.gl\nis.gd\nt.co\ncutt.ly\now.ly\nbuff.ly\nrebrand.ly\nshorturl.at\nadf.ly\nbit.do\nme2.do\nurl.kr\nvo.la\nhan.gl\nlnk.to"

// baro_sec_abuse_ext_default : 업로드 허용 확장자 기본 목록
const baro_sec_abuse_ext_default = "jpg\njpeg\npng\ngif\nwebp\nsvg\npdf\nmp3\nmp4\nwebm\nogg"

// baro_sec_lines_set : 여러 줄 설정 읽기
func baro_sec_lines_set(db *sql.DB, name string, def string) []string {
	return tool.Baro_split_lines(tool.Baro_get_def(db, name, def))
}

// ------------------------------------------------------------
//  실제로 동작하는 검사 함수
// ------------------------------------------------------------

// baro_sec_abuse_rate_ok : 무거운 검사 화면에 거는 요청 속도 제한
//
//	이 모듈의 검사 화면(파일 검사·링크 검사)은 DB 와 디스크를 많이 읽으므로
//	설정된 속도 제한을 실제로 적용한다.
func baro_sec_abuse_rate_ok(db *sql.DB, who string) bool {
	if !baro_sec_flag(db, "abuse_rate_on", true) {
		return true
	}

	return tool.Baro_rate_ok(
		db, "abuse_scan", who,
		tool.Baro_get_int(db, "abuse_rate_limit", 60),
		tool.Baro_get_int(db, "abuse_rate_window", 60),
	)
}

// baro_sec_bot_hit : 접속 프로그램 이름이 봇 목록에 걸리는지 (걸린 낱말 반환)
func baro_sec_bot_hit(db *sql.DB, ua string) string {
	if !baro_sec_flag(db, "abuse_bot_on", true) {
		return ""
	}

	lower := strings.ToLower(ua)
	if strings.TrimSpace(lower) == "" {
		return tool.Baro_t("(접속 프로그램 정보 없음)")
	}

	for _, word := range baro_sec_lines_set(db, "abuse_bot_list", baro_sec_abuse_bot_default) {
		word = strings.ToLower(strings.TrimSpace(word))
		if word != "" && strings.Contains(lower, word) {
			return word
		}
	}

	return ""
}

// baro_sec_spam_hits : 글에서 스팸 낱말을 찾는다
func baro_sec_spam_hits(db *sql.DB, text string) []string {
	out := []string{}
	if !baro_sec_flag(db, "abuse_spam_on", true) {
		return out
	}

	lower := strings.ToLower(text)
	for _, word := range baro_sec_lines_set(db, "abuse_spam_words", baro_sec_abuse_spam_default) {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}

		if strings.Contains(lower, strings.ToLower(word)) {
			out = append(out, word)
		}
	}

	return out
}

// baro_sec_link_hits : 글에서 위험·단축 주소를 찾는다
func baro_sec_link_hits(db *sql.DB, text string) []string {
	out := []string{}
	if !baro_sec_flag(db, "abuse_link_on", true) {
		return out
	}

	lower := strings.ToLower(text)
	for _, domain := range baro_sec_lines_set(db, "abuse_link_domains", baro_sec_abuse_link_default) {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" {
			continue
		}

		if strings.Contains(lower, "//"+domain) || strings.Contains(lower, "."+domain) || strings.Contains(lower, " "+domain) {
			out = append(out, domain)
		}
	}

	return out
}

// baro_sec_ext_allowed : 허용 확장자인지
func baro_sec_ext_allowed(db *sql.DB, ext string) bool {
	ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
	for _, allowed := range baro_sec_lines_set(db, "abuse_file_ext", baro_sec_abuse_ext_default) {
		if strings.ToLower(strings.TrimSpace(allowed)) == ext {
			return true
		}
	}

	return false
}

// ------------------------------------------------------------
//  /baro/security/abuse : 악용 방지 설정과 현황
// ------------------------------------------------------------

// View_baro_sec_abuse : 악용 방지 화면
func View_baro_sec_abuse(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/security/abuse"); need {
		return wall
	}

	if !baro_sec_is_system(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("악용 방지 설정은 system 권한(sysadmin 역할·관리자·소유자)만 볼 수 있습니다."))
	}

	if values != nil {
		return baro_sec_abuse_post(db, config, values)
	}

	baro_sec_watch(db, config, baro_sec_real_ip(config, query))

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"", tool.Baro_t("악용 방지")}})
	body += tool.Baro_hero(tool.Baro_t("악용 방지"), tool.Baro_t("자동 도구와 스팸이 위키를 망치지 못하게 막는 장치들입니다."), "")
	body += baro_sec_tabs("/baro/security/abuse")
	body += baro_sec_abuse_sub_tabs("/baro/security/abuse")

	if baro_sec_q(query, "done") != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("설정을 저장했습니다."))
	}
	if baro_sec_q(query, "fail") != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(baro_sec_q(query, "fail")))
	}

	// --- 현황 요약 ---
	rate_all := baro_sec_rate_rows(db, "%", 200)
	live := 0
	for _, row := range rate_all {
		if baro_sec_window_live(row[3], tool.Baro_get_int(db, "abuse_rate_window", 60)) {
			live++
		}
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("제한 기록"), tool.Baro_itoa(len(rate_all)), tool.Baro_t("baro_rate 표에 남은 줄")},
		{tool.Baro_t("살아 있는 창"), tool.Baro_itoa(live), tool.Baro_t("지금 세고 있는 시간 창")},
		{tool.Baro_t("봇 낱말"), tool.Baro_itoa(len(baro_sec_lines_set(db, "abuse_bot_list", baro_sec_abuse_bot_default))), tool.Baro_t("접속 프로그램 검사")},
		{tool.Baro_t("위험 주소"), tool.Baro_itoa(len(baro_sec_lines_set(db, "abuse_link_domains", baro_sec_abuse_link_default))), tool.Baro_t("링크 검사 목록")},
	})

	// --- 내 접속에 대한 즉석 검사 ---
	bot_hit := baro_sec_bot_hit(db, config.UserAgent)
	self_body := tool.Baro_kv_grid([][]string{
		{tool.Baro_t("내 접속 프로그램"), tool.HTML_escape(tool.Baro_trunc(config.UserAgent, 120))},
		{tool.Baro_t("봇 판정"), func() string {
			if bot_hit == "" {
				return tool.Baro_badge(tool.Baro_t("사람으로 봄"), "ok")
			}

			return tool.Baro_badge(tool.Baro_t("봇 낱말 '")+bot_hit+tool.Baro_t("' 일치"), "danger")
		}()},
		{tool.Baro_t("내 IP 대역"), tool.HTML_escape(baro_sec_ip_view(baro_sec_real_ip(config, query)))},
	})
	self_body += tool.Baro_note(tool.Baro_t("이 판정은 지금 이 요청에 실제 검사 함수를 그대로 돌려 본 결과입니다."))
	body += tool.Baro_card(tool.Baro_t("지금 이 접속은 어떻게 보이나요"), self_body)

	// --- 항목별 설정 ---
	body += tool.Baro_card(tool.Baro_t("한눈에 보기"), tool.Baro_table(
		[]string{tool.Baro_t("항목"), "상태", tool.Baro_t("지금 적용되는 곳")},
		[][]string{
			{tool.Baro_t("요청 속도 제한"), baro_sec_on_off(baro_sec_flag(db, "abuse_rate_on", true)),
				tool.Baro_t("바로위키의 무거운 검사 화면(파일·링크 검사)에 즉시 적용")},
			{tool.Baro_t("봇 차단"), baro_sec_on_off(baro_sec_flag(db, "abuse_bot_on", true)),
				tool.Baro_t("접속 프로그램 이름 검사 (이 화면과 검사 도구에서 사용)")},
			{tool.Baro_t("스팸 필터"), baro_sec_on_off(baro_sec_flag(db, "abuse_spam_on", true)),
				tool.Baro_t(`<a href="/baro/security/abuse/spam">문서 본문 검사</a> 와 시험 도구`)},
			{tool.Baro_t("대량 계정 생성 제한"), baro_sec_on_off(baro_sec_flag(db, "abuse_signup_on", true)),
				tool.Baro_t(`<a href="/baro/security/abuse/signup">같은 IP 다중 가입 탐지</a>`)},
			{tool.Baro_t("편집 폭주 제한"), baro_sec_on_off(baro_sec_flag(db, "abuse_edit_on", true)),
				tool.Baro_t(`<a href="/baro/security/abuse/rate">최근 편집 속도 점검</a>`)},
			{tool.Baro_t("파일 악성코드 검사"), baro_sec_on_off(baro_sec_flag(db, "abuse_file_on", true)),
				tool.Baro_t(`<a href="/baro/security/abuse/file">확장자·매직바이트 검사</a>`)},
			{tool.Baro_t("위험 링크 검사"), baro_sec_on_off(baro_sec_flag(db, "abuse_link_on", true)),
				tool.Baro_t(`<a href="/baro/security/abuse/link">문서 본문 링크 검사</a>`)},
			{tool.Baro_t("관리자 작업 추가 확인"), baro_sec_on_off(baro_sec_flag(db, "abuse_confirm_on", true)),
				tool.Baro_t("읽기 전용 전환·긴급 편집 중단 같은 위험 조치에 확인 문구 요구")},
		},
		"",
	))

	// --- 설정 폼 ---
	form := tool.Baro_hidden("op", "save")

	form += tool.Baro_section(tool.Baro_t("요청 속도 제한"),
		tool.Baro_checkbox("rate_on", tool.Baro_t("요청 속도 제한 사용"), "on", baro_sec_flag(db, "abuse_rate_on", true), "")+
			tool.Baro_input_type("rate_limit", "시간 창당 허용 요청 수", tool.Baro_itoa(tool.Baro_get_int(db, "abuse_rate_limit", 60)), "number", "")+
			tool.Baro_input_type("rate_window", "시간 창 (초)", tool.Baro_itoa(tool.Baro_get_int(db, "abuse_rate_window", 60)), "number", ""),
	)

	form += tool.Baro_section(tool.Baro_t("봇 차단"),
		tool.Baro_checkbox("bot_on", tool.Baro_t("봇 차단 사용"), "on", baro_sec_flag(db, "abuse_bot_on", true), "")+
			tool.Baro_textarea("bot_list", tool.Baro_t("접속 프로그램 낱말 (한 줄에 하나)"),
				strings.Join(baro_sec_lines_set(db, "abuse_bot_list", baro_sec_abuse_bot_default), "\n"), 6, "",
				tool.Baro_t("이름에 이 낱말이 들어 있으면 자동 도구로 봅니다. 대소문자를 가리지 않습니다.")),
	)

	form += tool.Baro_section(tool.Baro_t("스팸 필터"),
		tool.Baro_checkbox("spam_on", tool.Baro_t("스팸 필터 사용"), "on", baro_sec_flag(db, "abuse_spam_on", true), "")+
			tool.Baro_textarea("spam_words", tool.Baro_t("스팸 낱말 (한 줄에 하나)"),
				strings.Join(baro_sec_lines_set(db, "abuse_spam_words", baro_sec_abuse_spam_default), "\n"), 6, "",
				tool.Baro_t("본문에 이 낱말이 있으면 검사 화면에서 표시합니다.")),
	)

	form += tool.Baro_section(tool.Baro_t("대량 계정 생성 제한"),
		tool.Baro_checkbox("signup_on", tool.Baro_t("대량 계정 생성 탐지 사용"), "on", baro_sec_flag(db, "abuse_signup_on", true), "")+
			tool.Baro_input_type("signup_limit", "같은 IP 허용 계정 수", tool.Baro_itoa(tool.Baro_get_int(db, "abuse_signup_limit", 3)), "number", "")+
			tool.Baro_input_type("signup_hour", "판정 기간 (시간)", tool.Baro_itoa(tool.Baro_get_int(db, "abuse_signup_hour", 24)), "number", ""),
	)

	form += tool.Baro_section(tool.Baro_t("편집 폭주 제한"),
		tool.Baro_checkbox("edit_on", tool.Baro_t("편집 폭주 제한 사용"), "on", baro_sec_flag(db, "abuse_edit_on", true), "")+
			tool.Baro_input_type("edit_limit", "허용 편집 수", tool.Baro_itoa(tool.Baro_get_int(db, "abuse_edit_limit", 20)), "number", "")+
			tool.Baro_input_type("edit_window", "시간 창 (분)", tool.Baro_itoa(tool.Baro_get_int(db, "abuse_edit_window", 10)), "number", ""),
	)

	form += tool.Baro_section(tool.Baro_t("파일 검사"),
		tool.Baro_checkbox("file_on", tool.Baro_t("파일 검사 사용"), "on", baro_sec_flag(db, "abuse_file_on", true), "")+
			tool.Baro_textarea("file_ext", tool.Baro_t("허용 확장자 (한 줄에 하나)"),
				strings.Join(baro_sec_lines_set(db, "abuse_file_ext", baro_sec_abuse_ext_default), "\n"), 6, "", ""),
	)

	form += tool.Baro_section(tool.Baro_t("위험 링크 검사"),
		tool.Baro_checkbox("link_on", tool.Baro_t("위험 링크 검사 사용"), "on", baro_sec_flag(db, "abuse_link_on", true), "")+
			tool.Baro_textarea("link_domains", tool.Baro_t("의심 도메인·단축 주소 (한 줄에 하나)"),
				strings.Join(baro_sec_lines_set(db, "abuse_link_domains", baro_sec_abuse_link_default), "\n"), 6, "", ""),
	)

	form += tool.Baro_section(tool.Baro_t("관리자 작업 추가 확인"),
		tool.Baro_checkbox("confirm_on", tool.Baro_t("위험 조치에 확인 문구 요구"), "on", baro_sec_flag(db, "abuse_confirm_on", true),
			tool.Baro_t("켜면 읽기 전용 전환·긴급 편집 중단 화면에서 정해진 문구를 그대로 입력해야 실행됩니다.")),
	)

	form += baro_sec_reauth_field(db, config)
	form += tool.Baro_submit(tool.Baro_t("악용 방지 설정 저장"), "primary")

	body += tool.Baro_card("설정", tool.Baro_form("/baro/security/abuse", form))

	body += tool.Baro_card(tool.Baro_t("이 장치들의 한계"),
		tool.Baro_list([]string{
			tool.Baro_t("봇 차단은 접속 프로그램 이름만 봅니다. 이름을 속이는 도구는 걸러내지 못합니다."),
			tool.Baro_t("스팸 필터는 낱말 일치입니다. 우회 표기(사이에 기호 넣기)는 놓칠 수 있습니다."),
			tool.Baro_t("파일 검사는 확장자와 매직바이트만 봅니다. <b>바이러스 백신이 아닙니다.</b>"),
			tool.Baro_t("위험 링크 검사는 알려진 목록 기반입니다. 새 도메인은 목록에 넣어야 잡힙니다."),
			tool.Baro_t("openNAMU 본체의 편집·업로드 경로에는 이 확장이 끼어들지 않습니다. 여기의 검사 도구는 <b>사후 점검</b>용입니다."),
		}),
	)

	return baro_sec_page(db, config, tool.Baro_t("악용 방지"), body)
}

// baro_sec_abuse_sub_tabs : 악용 방지 하위 탭
func baro_sec_abuse_sub_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/security/abuse", "설정"},
		{"/baro/security/abuse/rate", tool.Baro_t("차단 현황")},
		{"/baro/security/abuse/signup", tool.Baro_t("대량 가입")},
		{"/baro/security/abuse/spam", tool.Baro_t("스팸 검사")},
		{"/baro/security/abuse/link", tool.Baro_t("위험 링크")},
		{"/baro/security/abuse/file", tool.Baro_t("파일 검사")},
	}, active)
}

// baro_sec_abuse_post : 악용 방지 설정 저장
func baro_sec_abuse_post(db *sql.DB, config tool.Config, values url.Values) string {
	if !baro_sec_is_system(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("악용 방지 설정은 system 권한만 바꿀 수 있습니다."))
	}

	if baro_sec_v(values, "op") != "save" {
		return tool.Get_redirect("/baro/security/abuse")
	}

	if ok, message := baro_sec_reauth_ok(db, config, values); !ok {
		return tool.Get_redirect("/baro/security/abuse?fail=" + url.QueryEscape(message))
	}

	before := baro_sec_abuse_state(db)

	baro_sec_flag_put(db, "abuse_rate_on", baro_sec_checked(values, "rate_on"))
	tool.Baro_put(db, "abuse_rate_limit", tool.Baro_itoa(baro_sec_int_def(baro_sec_v(values, "rate_limit"), 60)))
	tool.Baro_put(db, "abuse_rate_window", tool.Baro_itoa(baro_sec_int_def(baro_sec_v(values, "rate_window"), 60)))

	baro_sec_flag_put(db, "abuse_bot_on", baro_sec_checked(values, "bot_on"))
	tool.Baro_put(db, "abuse_bot_list", strings.Join(tool.Baro_split_lines(baro_sec_v(values, "bot_list")), "\n"))

	baro_sec_flag_put(db, "abuse_spam_on", baro_sec_checked(values, "spam_on"))
	tool.Baro_put(db, "abuse_spam_words", strings.Join(tool.Baro_split_lines(baro_sec_v(values, "spam_words")), "\n"))

	baro_sec_flag_put(db, "abuse_signup_on", baro_sec_checked(values, "signup_on"))
	tool.Baro_put(db, "abuse_signup_limit", tool.Baro_itoa(baro_sec_int_def(baro_sec_v(values, "signup_limit"), 3)))
	tool.Baro_put(db, "abuse_signup_hour", tool.Baro_itoa(baro_sec_int_def(baro_sec_v(values, "signup_hour"), 24)))

	baro_sec_flag_put(db, "abuse_edit_on", baro_sec_checked(values, "edit_on"))
	tool.Baro_put(db, "abuse_edit_limit", tool.Baro_itoa(baro_sec_int_def(baro_sec_v(values, "edit_limit"), 20)))
	tool.Baro_put(db, "abuse_edit_window", tool.Baro_itoa(baro_sec_int_def(baro_sec_v(values, "edit_window"), 10)))

	baro_sec_flag_put(db, "abuse_file_on", baro_sec_checked(values, "file_on"))
	tool.Baro_put(db, "abuse_file_ext", strings.Join(tool.Baro_split_lines(baro_sec_v(values, "file_ext")), "\n"))

	baro_sec_flag_put(db, "abuse_link_on", baro_sec_checked(values, "link_on"))
	tool.Baro_put(db, "abuse_link_domains", strings.Join(tool.Baro_split_lines(baro_sec_v(values, "link_domains")), "\n"))

	baro_sec_flag_put(db, "abuse_confirm_on", baro_sec_checked(values, "confirm_on"))

	baro_sec_log_op(db, config, "sec_abuse_set", tool.Baro_t("악용 방지 설정"), before, baro_sec_abuse_state(db), tool.Baro_t("악용 방지 설정 변경"))

	return tool.Get_redirect("/baro/security/abuse?done=1")
}

// baro_sec_abuse_state : 감사 로그에 남길 설정 요약
func baro_sec_abuse_state(db *sql.DB) string {
	return "rate=" + tool.Baro_get(db, "abuse_rate_on") +
		"/" + tool.Baro_itoa(tool.Baro_get_int(db, "abuse_rate_limit", 60)) +
		" bot=" + tool.Baro_get(db, "abuse_bot_on") +
		" spam=" + tool.Baro_get(db, "abuse_spam_on") +
		" signup=" + tool.Baro_get(db, "abuse_signup_on") +
		" edit=" + tool.Baro_get(db, "abuse_edit_on") +
		" file=" + tool.Baro_get(db, "abuse_file_on") +
		" link=" + tool.Baro_get(db, "abuse_link_on") +
		" confirm=" + tool.Baro_get(db, "abuse_confirm_on")
}

// ------------------------------------------------------------
//  /baro/security/abuse/rate : 현재 차단 현황 · 편집 폭주 점검
// ------------------------------------------------------------

// View_baro_sec_abuse_rate : 요청 제한 현황
func View_baro_sec_abuse_rate(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/security/abuse/rate"); need {
		return wall
	}

	if !baro_sec_is_system(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("차단 현황은 system 권한만 볼 수 있습니다."))
	}

	if values != nil {
		if baro_sec_v(values, "op") == "clear" {
			key_name := baro_sec_v(values, "key_name")
			who := baro_sec_v(values, "who")

			if key_name != "" && who != "" {
				tool.Exec_DB(db, "delete from baro_rate where key_name = ? and who = ?", key_name, who)
				baro_sec_log_op(db, config, "sec_rate_clear", key_name, tool.Baro_mask_ip(who), "", tool.Baro_t("요청 제한 수동 해제"))
			}
		}

		return tool.Get_redirect("/baro/security/abuse/rate")
	}

	baro_sec_watch(db, config, baro_sec_real_ip(config, query))

	page := baro_sec_page_num(query)
	like := "%"
	if key := baro_sec_q(query, "key"); key != "" {
		like = "%" + key + "%"
	}

	rows := baro_sec_rate_page(db, like, (page-1)*tool.Baro_page_size, tool.Baro_page_size)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"/baro/security/abuse", tool.Baro_t("악용 방지")}, {"", tool.Baro_t("차단 현황")}})
	body += tool.Baro_hero(tool.Baro_t("차단 현황"), tool.Baro_t("요청 제한(baro_rate)에 잡혀 있는 대상입니다."), "")
	body += baro_sec_tabs("/baro/security/abuse")
	body += baro_sec_abuse_sub_tabs("/baro/security/abuse/rate")

	body += `<form class="bw_form" method="get" action="/baro/security/abuse/rate">` +
		tool.Baro_input("key", tool.Baro_t("제한 키로 거르기"), baro_sec_q(query, "key"), tool.Baro_t("예 : sec_login"), tool.Baro_t("비워 두면 전체를 봅니다.")) +
		tool.Baro_submit(tool.Baro_t("거르기"), "") + `</form>`

	window := tool.Baro_get_int(db, "abuse_rate_window", 60)
	table_rows := [][]string{}

	for _, row := range rows {
		live := baro_sec_window_live(row[3], window)
		state := tool.Baro_badge(tool.Baro_t("시간 창 지남"), "muted")
		if live {
			state = tool.Baro_badge(tool.Baro_t("세는 중"), "warn")
		}

		table_rows = append(table_rows, []string{
			tool.HTML_escape(row[0]),
			tool.HTML_escape(tool.Baro_mask_ip(row[1])),
			tool.HTML_escape(row[2]),
			tool.Baro_time_cell(row[3]),
			state,
			tool.Baro_form("/baro/security/abuse/rate",
				tool.Baro_hidden("op", "clear")+
					tool.Baro_hidden("key_name", row[0])+
					tool.Baro_hidden("who", row[1])+
					tool.Baro_submit("해제", ""),
			),
		})
	}

	if len(table_rows) == 0 {
		body += tool.Baro_empty(tool.Baro_t("지금 요청 제한에 걸린 대상이 없습니다."))
	} else {
		body += tool.Baro_table([]string{tool.Baro_t("제한 키"), tool.Baro_t("대상"), tool.Baro_t("횟수"), tool.Baro_t("창 시작"), "상태", "관리"}, table_rows, "")
	}

	page_url := "/baro/security/abuse/rate?page={}"
	if key := baro_sec_q(query, "key"); key != "" {
		page_url += "&key=" + url.QueryEscape(key)
	}
	body += tool.Baro_paging(page, len(rows), tool.Baro_page_size, page_url)

	// --- 편집 폭주 점검 ---
	edit_limit := tool.Baro_get_int(db, "abuse_edit_limit", 20)
	edit_window := tool.Baro_get_int(db, "abuse_edit_window", 10)
	since := tool.Baro_time_add(tool.Baro_now(), 0, 0, -edit_window)

	burst_rows := [][]string{}
	burst := tool.Query_DB(
		db,
		"select ip, count(*) from history where date >= ? group by ip order by count(*) desc limit 20",
		since,
	)
	defer burst.Close()

	for burst.Next() {
		ip, count := "", 0
		if err := burst.Scan(&ip, &count); err != nil {
			continue
		}

		state := tool.Baro_badge(tool.Baro_t("정상"), "ok")
		if baro_sec_flag(db, "abuse_edit_on", true) && count >= edit_limit {
			state = tool.Baro_badge(tool.Baro_t("폭주 의심"), "danger")
		}

		burst_rows = append(burst_rows, []string{
			tool.HTML_escape(tool.Baro_mask_ip(ip)),
			tool.Baro_comma(count),
			state,
		})
	}

	burst_body := tool.Baro_note(
		tool.Baro_t("최근 ") + tool.Baro_itoa(edit_window) + tool.Baro_t("분 동안의 편집 수입니다. ") +
			tool.Baro_itoa(edit_limit) + tool.Baro_t("회 이상이면 폭주로 봅니다. (기준 시각 : ") + since + ")",
	)
	burst_body += tool.Baro_table([]string{tool.Baro_t("편집자"), tool.Baro_t("편집 수"), tool.Baro_t("판정")}, burst_rows, tool.Baro_t("최근 편집이 없습니다."))
	burst_body += tool.Baro_note(tool.Baro_t("폭주가 보이면 사건으로 다루고, 되돌리기는 대량 작업 절차(2인 승인·스냅샷·유예)를 거치세요."))

	body += tool.Baro_card(tool.Baro_t("편집 폭주 점검"), burst_body)

	return baro_sec_page(db, config, tool.Baro_t("차단 현황"), body)
}

// baro_sec_rate_page : 요청 제한 표 쪽 단위 읽기
func baro_sec_rate_page(db *sql.DB, like string, offset int, limit int) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(
		db,
		"select key_name, who, num, window_start from baro_rate where key_name like ? order by window_start desc limit ? offset ?",
		like, limit, offset,
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

// ------------------------------------------------------------
//  /baro/security/abuse/signup : 대량 계정 생성 탐지
// ------------------------------------------------------------

// baro_sec_signup_group : 같은 IP 에서 만들어진 계정 묶음
type baro_sec_signup_group struct {
	IP    string
	Users []string
	Dates []string
	Close int
}

// View_baro_sec_abuse_signup : 대량 계정 생성 탐지 화면
func View_baro_sec_abuse_signup(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/security/abuse/signup"); need {
		return wall
	}

	if !baro_sec_is_system(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("가입 기록 조회는 system 권한만 할 수 있습니다."))
	}

	if values != nil {
		return tool.Get_redirect("/baro/security/abuse/signup")
	}

	baro_sec_watch(db, config, baro_sec_real_ip(config, query))

	limit := tool.Baro_get_int(db, "abuse_signup_limit", 3)
	hour := tool.Baro_get_int(db, "abuse_signup_hour", 24)

	// 가입 시각 읽기
	date_map := map[string]string{}
	date_rows := tool.Query_DB(db, "select id, data from user_set where name = 'date' limit 5000")
	defer date_rows.Close()

	for date_rows.Next() {
		id, data := "", ""
		if err := date_rows.Scan(&id, &data); err != nil {
			continue
		}

		date_map[id] = data
	}

	// 가입 IP 읽기
	group_map := map[string]*baro_sec_signup_group{}
	ip_rows := tool.Query_DB(db, "select id, data from user_set where name = 'register_ip' and data != '' limit 5000")
	defer ip_rows.Close()

	for ip_rows.Next() {
		id, ip := "", ""
		if err := ip_rows.Scan(&id, &ip); err != nil {
			continue
		}

		if group_map[ip] == nil {
			group_map[ip] = &baro_sec_signup_group{IP: ip}
		}

		group_map[ip].Users = append(group_map[ip].Users, id)
		group_map[ip].Dates = append(group_map[ip].Dates, date_map[id])
	}

	groups := []*baro_sec_signup_group{}
	for _, group := range group_map {
		// 정해진 기간 안에 몰려 있는 가입 수를 센다
		for index, base := range group.Dates {
			near := 0
			for other, compare := range group.Dates {
				if other == index {
					near++

					continue
				}

				if base == "" || compare == "" {
					continue
				}

				gap := baro_sec_hours_between(base, compare)
				if gap >= 0 && gap <= float64(hour) {
					near++
				}
			}

			if near > group.Close {
				group.Close = near
			}
		}

		groups = append(groups, group)
	}

	sort.SliceStable(groups, func(i int, j int) bool {
		if len(groups[i].Users) != len(groups[j].Users) {
			return len(groups[i].Users) > len(groups[j].Users)
		}

		return groups[i].IP < groups[j].IP
	})

	page := baro_sec_page_num(query)
	start := (page - 1) * tool.Baro_page_size
	end := start + tool.Baro_page_size
	if start > len(groups) {
		start = len(groups)
	}
	if end > len(groups) {
		end = len(groups)
	}

	table_rows := [][]string{}
	danger := 0

	for _, group := range groups {
		if len(group.Users) >= limit {
			danger++
		}
	}

	for _, group := range groups[start:end] {
		state := tool.Baro_badge(tool.Baro_t("정상"), "ok")
		if baro_sec_flag(db, "abuse_signup_on", true) && len(group.Users) >= limit {
			state = tool.Baro_badge(tool.Baro_t("대량 가입 의심"), "danger")
		} else if group.Close >= limit {
			state = tool.Baro_badge(tool.Baro_t("짧은 기간에 몰림"), "warn")
		}

		names := []string{}
		for _, user := range group.Users {
			names = append(names, tool.Baro_user_link(db, user, config.IP))
		}

		table_rows = append(table_rows, []string{
			tool.HTML_escape(tool.Baro_mask_ip(group.IP)),
			tool.Baro_itoa(len(group.Users)),
			tool.Baro_itoa(group.Close),
			strings.Join(names, ", "),
			state,
		})
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"/baro/security/abuse", tool.Baro_t("악용 방지")}, {"", tool.Baro_t("대량 가입")}})
	body += tool.Baro_hero(tool.Baro_t("대량 계정 생성 탐지"), tool.Baro_t("같은 IP 에서 짧은 시간에 만들어진 계정을 찾습니다."), "")
	body += baro_sec_tabs("/baro/security/abuse")
	body += baro_sec_abuse_sub_tabs("/baro/security/abuse/signup")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("검사한 IP"), tool.Baro_itoa(len(groups)), tool.Baro_t("가입 IP 가 남아 있는 경우만")},
		{tool.Baro_t("의심 IP"), tool.Baro_itoa(danger), "계정 " + tool.Baro_itoa(limit) + tool.Baro_t("개 이상")},
		{tool.Baro_t("판정 기간"), tool.Baro_itoa(hour) + tool.Baro_t("시간"), tool.Baro_t("이 기간 안에 몰린 가입")},
	})

	body += tool.Baro_alert("info",
		tool.Baro_t("가입 IP 는 openNAMU 가 <code>user_set</code> 의 <code>register_ip</code> 에 남긴 값입니다. ")+
			tool.Baro_t("로그인 상태에서 만든 계정은 IP 가 비어 있어 여기 나오지 않습니다. 화면에서는 IP 를 가려서 보여줍니다."),
	)

	if len(table_rows) == 0 {
		body += tool.Baro_empty(tool.Baro_t("가입 IP 기록이 남은 계정이 없습니다."))
	} else {
		body += tool.Baro_table([]string{"가입 IP", tool.Baro_t("계정 수"), tool.Baro_t("기간 내 몰린 수"), "계정", tool.Baro_t("판정")}, table_rows, "")
	}

	body += tool.Baro_paging(page, len(table_rows), tool.Baro_page_size, "/baro/security/abuse/signup?page={}")

	body += tool.Baro_card(tool.Baro_t("의심 묶음을 찾았다면"),
		tool.Baro_list([]string{
			tool.Baro_t("바로 차단하지 마세요. 학교·회사·공용 와이파이는 같은 IP 를 씁니다."),
			tool.Baro_t("기여 내용을 먼저 보세요. 서로 같은 문서를 밀어주는 편집이 있는지 확인합니다."),
			tool.Baro_t("조치가 필요하면 사건으로 만들어 사유와 근거를 남기고, 제재는 사람에게만 적용합니다."),
			tool.Baro_t("계정이 많다는 이유만으로 문서를 지우지 않습니다. (0장 1번 원칙)"),
		}),
	)

	return baro_sec_page(db, config, tool.Baro_t("대량 계정 생성 탐지"), body)
}
