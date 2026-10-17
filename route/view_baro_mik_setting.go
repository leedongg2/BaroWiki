package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  밐 · 캡차 설정  /baro/mik/setting
//
//  숫자를 코드에 박아 두지 않고 운영진이 화면에서 고친다.
//  바꾼 값은 공개 운영 로그와 감사 기록에 남는다. (5번 원칙)
// ============================================================

// baro_mik_setting_row : 고칠 수 있는 설정 한 줄
type baro_mik_setting_row struct {
	Name  string // baro_set 의 이름
	Label string
	Def   string
	Help  string
}

// baro_mik_setting_list : 숫자로 적는 설정
func baro_mik_setting_list() []baro_mik_setting_row {
	return []baro_mik_setting_row{
		{
			"mik_attend", tool.Baro_t("접속 보상 (하루 한 번)"), "5",
			tool.Baro_t("로그인한 채로 위키를 연 첫 순간에 들어옵니다. 0 으로 두면 접속 보상이 없습니다."),
		},
		{
			"mik_badge", tool.Baro_t("도전 과제 기본 보상"), "20",
			tool.Baro_t("따로 값을 정하지 않은 배지에 쓰는 값입니다. 배지마다 다르게 주려면 아래 표에서 고칩니다."),
		},
		{
			"mik_send_daily", tool.Baro_t("하루 양도 한도"), "100",
			tool.Baro_t("한 사람이 하루에 남에게 보낼 수 있는 밐입니다. 0 으로 두면 양도를 막습니다."),
		},
	}
}

// View_baro_mik_setting : 밐 · 캡차 설정 (/baro/mik/setting)
func View_baro_mik_setting(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, stop := baro_mik_admin_gate(db, config); stop {
		return deny
	}

	alert := ""

	if values != nil {
		switch values.Get("do") {
		case "save":
			alert = baro_mik_setting_save(db, config, values)
		case "badge":
			alert = baro_mik_setting_badge_save(db, config, values)
		}
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/mik", "밐"}, {"", tool.Baro_t("설정")}})
	body += tool.Baro_hero(
		tool.Baro_t("밐 · 캡차 설정"),
		tool.Baro_t("보상액과 한도, 캡차를 누구에게 물을지 정합니다."),
		"",
	)
	body += alert

	// --- 숫자 설정 ---
	form := tool.Baro_hidden("do", "save")
	for _, row := range baro_mik_setting_list() {
		form += tool.Baro_input_type(
			row.Name, row.Label,
			tool.Baro_get_def(db, row.Name, row.Def), "number", row.Help,
		)
	}

	form += tool.Baro_select("captcha_mode", tool.Baro_t("캡차를 누구에게 묻나"),
		tool.Baro_captcha_mode_list(), tool.Baro_captcha_mode(db),
		tool.Baro_t("운영진과 no_captcha 권한을 받은 사람, 면제권을 산 사람은 어느 값이든 묻지 않습니다."))

	form += tool.Baro_input("reason", tool.Baro_t("바꾸는 까닭 (선택)"), "", "",
		tool.Baro_t("공개 운영 로그에 함께 남습니다."))
	form += tool.Baro_submit(tool.Baro_t("저장하기"), "primary")

	body += tool.Baro_card(tool.Baro_t("값 정하기"), tool.Baro_form("/baro/mik/setting", form))

	// --- 배지별 보상 ---
	body += baro_mik_setting_badge_card(db)

	body += tool.Baro_card(tool.Baro_t("캡차가 어떻게 도나"), tool.Baro_list([]string{
		tool.Baro_t_esc("바깥 캡차(reCAPTCHA · hCaptcha · 턴스타일) 열쇠가 꽂혀 있으면 그쪽을 씁니다."),
		tool.Baro_t_esc("열쇠가 없으면 위키가 스스로 그림 문제를 냅니다. 바깥으로 나가는 정보가 없습니다."),
		tool.Baro_t_esc("사측 관리자(owner) · 관리자(admin) 에게는 묻지 않습니다."),
		tool.Baro_t_esc("권한 그룹에서 「캡차 묻지 않기 권한」(no_captcha) 을 준 사람에게도 묻지 않습니다."),
		tool.Baro_t_esc("바로샵 「캡차 면제 15일권」 을 산 사람은 그 기간 동안 묻지 않습니다."),
		tool.Baro_t_esc("바로샵 「캡차 1회 면제권」 이 남아 있으면 보내는 순간 한 장을 쓰고 넘어갑니다."),
	}))

	body += tool.Baro_button("/baro/mik/admin", tool.Baro_t("밐 지급 화면"), "") + " " +
		tool.Baro_button("/baro/shop/admin", tool.Baro_t("바로샵 상품 관리"), "") + " " +
		tool.Baro_button("/auth/list", tool.Baro_t("권한 그룹 관리"), "")

	return tool.Baro_page(db, config, tool.Baro_t("밐 · 캡차 설정"), body, baro_mik_menu())
}

// baro_mik_setting_save : 숫자 설정과 캡차 모드 저장
func baro_mik_setting_save(db *sql.DB, config tool.Config, values url.Values) string {
	reason := strings.TrimSpace(values.Get("reason"))
	changed := []string{}

	for _, row := range baro_mik_setting_list() {
		typed := strings.TrimSpace(values.Get(row.Name))
		if typed == "" {
			continue
		}

		next := tool.Str_to_int(typed)
		if next < 0 {
			return tool.Baro_alert("warn", row.Label+tool.Baro_t(" 은(는) 0 이상이어야 합니다."))
		}

		before := tool.Baro_get_def(db, row.Name, row.Def)
		after := tool.Baro_itoa(next)
		if before == after {
			continue
		}

		tool.Baro_put(db, row.Name, after)
		tool.Baro_audit(db, config, "baro_mik_setting", row.Name, before, after, reason)
		changed = append(changed, row.Label+" "+before+" → "+after)
	}

	// 캡차 모드
	want := strings.TrimSpace(values.Get("captcha_mode"))
	for _, row := range tool.Baro_captcha_mode_list() {
		if row[0] != want {
			continue
		}

		before := tool.Baro_captcha_mode(db)
		if before != want {
			tool.Baro_put(db, "captcha_mode", want)
			tool.Baro_audit(db, config, "baro_captcha_mode", "captcha_mode", before, want, reason)
			changed = append(changed, tool.Baro_t("캡차 범위 ")+before+" → "+want)
		}

		break
	}

	if len(changed) == 0 {
		return tool.Baro_alert("info", tool.Baro_t("바뀐 값이 없습니다."))
	}

	tool.Baro_public_log(db, "mik_setting", config.IP, "baro_set", reason, strings.Join(changed, " · "))

	return tool.Baro_alert("ok", tool.Baro_t("저장했습니다. ")+strings.Join(changed, " · "))
}

// baro_mik_setting_badge_card : 배지마다 보상액을 정하는 표
func baro_mik_setting_badge_card(db *sql.DB) string {
	list := baro_contrib_badge_list(db)
	if len(list) == 0 {
		return tool.Baro_card(tool.Baro_t("도전 과제별 보상"),
			tool.Baro_empty(tool.Baro_t("등록된 배지가 없습니다.")))
	}

	inner := tool.Baro_hidden("do", "badge")
	rows := [][]string{}

	for _, row := range list {
		value := ""
		if own := tool.Baro_mik_badge_own(db, row.Key); own >= 0 {
			value = tool.Baro_itoa(own)
		}

		box := `<input class="bw_in" type="number" min="0" style="width: 110px;" name="badge_` +
			tool.HTML_escape(row.Key) + `" value="` + tool.HTML_escape(value) + `" placeholder="` +
			tool.Baro_t_esc("기본값") + `">`

		rows = append(rows, []string{
			tool.HTML_escape(row.Icon) + " " + tool.HTML_escape(row.Name),
			tool.HTML_escape(tool.Baro_trunc(row.Description, 60)),
			tool.HTML_escape(tool.Baro_mik_text(tool.Baro_mik_badge_amount(db, row.Key))),
			box,
		})
	}

	inner += tool.Baro_table(
		[]string{tool.Baro_t("배지"), tool.Baro_t("조건"), tool.Baro_t("지금 주는 밐"), tool.Baro_t("따로 정하기")},
		rows,
		"",
	)
	inner += tool.Baro_input("reason", tool.Baro_t("바꾸는 까닭 (선택)"), "", "", "")
	inner += tool.Baro_submit(tool.Baro_t("배지별 보상 저장"), "primary")

	return tool.Baro_card_ex(
		tool.Baro_t("도전 과제별 보상"),
		tool.Baro_t("칸을 비우면 기본 보상을 따릅니다. 0 을 적으면 그 배지만 보상이 없습니다."),
		tool.Baro_form("/baro/mik/setting", inner),
		"",
	)
}

// baro_mik_setting_badge_save : 배지별 보상액 저장
func baro_mik_setting_badge_save(db *sql.DB, config tool.Config, values url.Values) string {
	reason := strings.TrimSpace(values.Get("reason"))
	changed := []string{}

	for _, row := range baro_contrib_badge_list(db) {
		typed := strings.TrimSpace(values.Get("badge_" + row.Key))

		if typed != "" && tool.Str_to_int(typed) < 0 {
			return tool.Baro_alert("warn", row.Name+tool.Baro_t(" 의 보상은 0 이상이어야 합니다."))
		}

		before := ""
		if own := tool.Baro_mik_badge_own(db, row.Key); own >= 0 {
			before = tool.Baro_itoa(own)
		}

		after := ""
		if typed != "" {
			after = tool.Baro_itoa(tool.Str_to_int(typed))
		}

		if before == after {
			continue
		}

		tool.Baro_mik_badge_set(db, row.Key, after)
		tool.Baro_audit(db, config, "baro_mik_badge_amount", "badge:"+row.Key, before, after, reason)

		show := after
		if show == "" {
			show = tool.Baro_t("기본값")
		}
		changed = append(changed, row.Name+" → "+show)
	}

	if len(changed) == 0 {
		return tool.Baro_alert("info", tool.Baro_t("바뀐 값이 없습니다."))
	}

	tool.Baro_public_log(db, "mik_badge_amount", config.IP, "baro_badge", reason, strings.Join(changed, " · "))

	return tool.Baro_alert("ok", tool.Baro_t("저장했습니다. ")+strings.Join(changed, " · "))
}
