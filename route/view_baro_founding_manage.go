package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  1-2 창립 시즌 운영 (/baro/founding/manage)
//    지원자 검토 · 수동 번호 발급/취소 · 봇 의심 표시 · 정원 조정
//    권한 : founding
// ============================================================

// baro_founding_manage_msg : 처리 결과 코드를 안내 문구로
func baro_founding_manage_msg(code string) (string, string) {
	switch code {
	case "goal":
		return "ok", tool.Baro_t("창립 시즌 설정을 저장했습니다.")
	case "grant":
		return "ok", tool.Baro_t("창립 번호를 발급했습니다.")
	case "revoke":
		return "ok", tool.Baro_t("창립 번호를 취소하고 보류 상태로 두었습니다. 보류 중에는 번호가 자동으로 다시 발급되지 않습니다.")
	case "reject":
		return "ok", tool.Baro_t("지원을 반려했습니다.")
	case "restore":
		return "ok", tool.Baro_t("대기 상태로 되돌렸습니다. 첫 유효 기여가 확인되면 다시 자동으로 번호가 발급됩니다.")
	case "full":
		return "danger", tool.Baro_t("정원이 가득 차 번호를 발급할 수 없습니다. 정원을 먼저 조정해 주세요.")
	case "no_user":
		return "danger", tool.Baro_t("대상 사용자를 찾을 수 없습니다.")
	case "no_reason":
		return "danger", tool.Baro_t("사유를 입력해야 합니다. (0장 원칙 : 운영 근거는 반드시 남긴다)")
	case "state":
		return "danger", tool.Baro_t("지금 상태에서는 할 수 없는 조치입니다.")
	case "read_only":
		return "danger", tool.Baro_t("읽기 전용 모드에서는 운영 조치를 할 수 없습니다.")
	}

	return "", ""
}

// baro_founding_manage_grant : 수동 번호 발급
func baro_founding_manage_grant(db *sql.DB, config tool.Config, user string, reason string) string {
	person, found := baro_founding_find(db, user)
	if !found {
		return "no_user"
	}

	if person.Status == "confirmed" {
		return "state"
	}

	if strings.TrimSpace(reason) == "" {
		return "no_reason"
	}

	next := baro_founding_max_number(db) + 1
	if next > baro_founding_goal(db) {
		return "full"
	}

	number := tool.Baro_itoa(next)
	event := ""
	if first := baro_founding_first_valid(db, user); first != nil {
		event = first[0]
	}

	tool.Exec_DB(
		db,
		"update baro_founder set status = 'confirmed', number = ?, date = ?, first_event = ? where user = ?",
		number, tool.Baro_now(), event, user,
	)

	baro_founding_give_badge(db, user, "founder", "창립 기여자 #"+number)
	tool.Baro_notify(db, user, "badge", "창립 기여자 번호 #"+number+" 가 확정되었습니다.", "/baro/founding/check", "founder")
	tool.Baro_public_log(db, "founder_confirm", config.IP, user, tool.Baro_one_line(reason), "수동 발급 : 창립 기여자 #"+number)
	tool.Baro_audit(db, config, "founder_grant", user, person.Status, "confirmed#"+number, reason)
	tool.Baro_metric_bump(db, "founder_confirm", tool.Baro_today(), 1)

	return "grant"
}

// baro_founding_manage_revoke : 번호 취소 (보류 상태로 둔다)
//
//	대기(applied) 로 되돌리면 다음 명단 조회 때 자동 승격이 다시 번호를 발급해
//	운영자의 조치가 스스로 뒤집힌다. 그래서 보류(pending) 로 두어 자동 발급 대상에서 뺀다.
func baro_founding_manage_revoke(db *sql.DB, config tool.Config, user string, reason string) string {
	person, found := baro_founding_find(db, user)
	if !found {
		return "no_user"
	}

	if person.Status != "confirmed" {
		return "state"
	}

	if strings.TrimSpace(reason) == "" {
		return "no_reason"
	}

	tool.Exec_DB(
		db,
		"update baro_founder set status = 'pending', number = '', date = '', note = ? where user = ?",
		tool.Baro_trunc(tool.Baro_one_line(reason), 300), user,
	)
	tool.Exec_DB(db, "delete from baro_user_badge where user = ? and badge = ?", user, "founder")

	tool.Baro_notify(db, user, "system", "창립 기여자 번호가 취소되었습니다. 사유를 확인해 주세요.", "/baro/founding/check", "founder_revoke")
	tool.Baro_public_log(db, "founder_revoke", config.IP, user, tool.Baro_one_line(reason), "창립 기여자 #"+person.Number+" 취소")
	tool.Baro_audit(db, config, "founder_revoke", user, "confirmed#"+person.Number, "pending", reason)

	return "revoke"
}

// baro_founding_manage_reject : 지원 반려
func baro_founding_manage_reject(db *sql.DB, config tool.Config, user string, reason string) string {
	person, found := baro_founding_find(db, user)
	if !found {
		return "no_user"
	}

	if person.Status == "rejected" {
		return "state"
	}

	if strings.TrimSpace(reason) == "" {
		return "no_reason"
	}

	tool.Exec_DB(
		db,
		"update baro_founder set status = 'rejected', number = '', note = ? where user = ?",
		tool.Baro_trunc(tool.Baro_one_line(reason), 300), user,
	)
	tool.Exec_DB(db, "delete from baro_user_badge where user = ? and badge = ?", user, "founder")

	tool.Baro_notify(db, user, "system", "창립 기여자 지원이 반려되었습니다.", "/baro/founding/check", "founder_reject")
	tool.Baro_public_log(db, "founder_reject", config.IP, user, tool.Baro_one_line(reason), "창립 기여자 지원 반려")
	tool.Baro_audit(db, config, "founder_reject", user, person.Status, "rejected", reason)

	return "reject"
}

// baro_founding_manage_restore : 반려·보류 취소 (다시 대기 상태로)
func baro_founding_manage_restore(db *sql.DB, config tool.Config, user string, reason string) string {
	person, found := baro_founding_find(db, user)
	if !found {
		return "no_user"
	}

	if person.Status != "rejected" && person.Status != "pending" {
		return "state"
	}

	if strings.TrimSpace(reason) == "" {
		return "no_reason"
	}

	before := person.Status
	tool.Exec_DB(db, "update baro_founder set status = 'applied', note = '' where user = ?", user)
	tool.Baro_notify(db, user, "system", "창립 기여자 지원이 다시 대기 상태가 되었습니다.", "/baro/founding/check", "founder_restore")
	tool.Baro_public_log(db, "founder_restore", config.IP, user, tool.Baro_one_line(reason), "창립 기여자 지원 "+baro_founding_status_name(before)+" 취소")
	tool.Baro_audit(db, config, "founder_restore", user, before, "applied", reason)

	return "restore"
}

// baro_founding_manage_goal : 정원 조정
func baro_founding_manage_goal(db *sql.DB, config tool.Config, values url.Values) string {
	before := tool.Baro_get_def(db, "founding_goal", "100") + "/" + tool.Baro_get_def(db, "founding_open", "on")

	goal := tool.Str_to_int(values.Get("goal"))
	if goal < 1 {
		goal = 1
	}
	if goal > 100000 {
		goal = 100000
	}

	// 빈 값으로 저장하면 Baro_get_def 의 기본값("on")으로 읽히므로 꺼짐은 "off" 로 남긴다.
	open := "off"
	if strings.TrimSpace(values.Get("open")) != "" {
		open = "on"
	}

	tool.Baro_put(db, "founding_goal", tool.Baro_itoa(goal))
	tool.Baro_put(db, "founding_open", open)

	after := tool.Baro_itoa(goal) + "/" + open
	tool.Baro_public_log(db, "founding_set", config.IP, "founding", tool.Baro_one_line(values.Get("reason")), "정원 "+tool.Baro_itoa(goal)+"명")
	tool.Baro_audit(db, config, "founding_set", "founding", before, after, values.Get("reason"))

	return "goal"
}

// baro_founding_manage_action : 운영 조치 처리
func baro_founding_manage_action(db *sql.DB, config tool.Config, values url.Values) string {
	if tool.Baro_read_only(db) {
		return "read_only"
	}

	action := values.Get("action")
	user := strings.TrimSpace(values.Get("user"))
	reason := values.Get("reason")

	switch action {
	case "goal":
		return baro_founding_manage_goal(db, config, values)
	case "grant":
		return baro_founding_manage_grant(db, config, user, reason)
	case "revoke":
		return baro_founding_manage_revoke(db, config, user, reason)
	case "reject":
		return baro_founding_manage_reject(db, config, user, reason)
	case "restore":
		return baro_founding_manage_restore(db, config, user, reason)
	}

	return "state"
}

// baro_founding_manage_form : 한 사람에 대한 조치 폼
func baro_founding_manage_form(action string, user string, label string, kind string, tab string) string {
	form := tool.Baro_hidden("action", action)
	form += tool.Baro_hidden("user", user)
	form += tool.Baro_hidden("tab", tab)
	form += tool.Baro_t(`<input class="bw_in" name="reason" placeholder="사유 (필수)">`)
	form += tool.Baro_submit(label, kind)

	return tool.Baro_form("/baro/founding/manage", form)
}

// View_baro_founding_manage : 창립 시즌 운영 (1-2)
func View_baro_founding_manage(config tool.Config, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "founding") {
		return tool.Baro_deny(db, config, tool.Baro_t("창립 시즌 운영은 창립 시즌 운영자(founding) 역할이 있어야 볼 수 있습니다."))
	}

	tab := "applied"
	if query != nil && query.Get("tab") != "" {
		tab = query.Get("tab")
	}

	if values != nil {
		if values.Get("tab") != "" {
			tab = values.Get("tab")
		}

		code := baro_founding_manage_action(db, config, values)

		return tool.Get_redirect("/baro/founding/manage?tab=" + tool.Url_parser(tab) + "&done=" + tool.Url_parser(code))
	}

	switch tab {
	case "applied", "confirmed", "pending", "rejected":
	default:
		tab = "applied"
	}

	page_int := baro_founding_page_num(page)
	list := baro_founding_status_rows(db, tab, (page_int-1)*tool.Baro_page_size, tool.Baro_page_size)

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/founding", tool.Baro_t("창립 시즌")},
		{"", tool.Baro_t("창립 시즌 운영")},
	})
	body += tool.Baro_hero(tool.Baro_t("창립 시즌 운영"), tool.Baro_t("지원자 검토 · 번호 발급 · 정원 조정"), "")
	body += baro_founding_tabs("")

	if query != nil {
		if kind, text := baro_founding_manage_msg(query.Get("done")); text != "" {
			body += tool.Baro_alert(kind, tool.HTML_escape(text))
		}
	}

	body += tool.Baro_alert("info",
		tool.Baro_t("이 화면은 <b>창립 시즌 운영</b> 전용입니다. 제재·격리 같은 이용자 조치는 여기서 하지 않습니다. ")+
			tool.Baro_t("(0장 원칙 1 : 이용자 제재와 콘텐츠 판단은 분리합니다)"))

	body += baro_founding_seat_html(db)

	// --- 정원 조정 ---
	goal_form := tool.Baro_hidden("action", "goal")
	goal_form += tool.Baro_hidden("tab", tab)
	goal_form += tool.Baro_input_type("goal", tool.Baro_t("창립단 정원"), tool.Baro_itoa(baro_founding_goal(db)), "number", tool.Baro_t("정원을 넘는 번호는 발급되지 않습니다."))
	goal_form += tool.Baro_checkbox("open", tool.Baro_t("창립 기여자 지원을 받는 중"), "on", baro_founding_open(db), tool.Baro_t("끄면 새 지원을 받지 않습니다. 이미 접수된 지원은 유지됩니다."))
	goal_form += tool.Baro_input("reason", tool.Baro_t("변경 사유"), "", tool.Baro_t("예 : 창립 시즌 2차 모집 시작"), tool.Baro_t("공개 운영 로그에 남습니다."))
	goal_form += tool.Baro_submit(tool.Baro_t("설정 저장"), "primary")

	body += tool.Baro_card(tool.Baro_t("정원 조정"), tool.Baro_form("/baro/founding/manage", goal_form))

	// --- 지원자 목록 ---
	body += tool.Baro_tabs([][]string{
		{"/baro/founding/manage?tab=applied", tool.Baro_t("검토 대기 ") + tool.Baro_itoa(baro_founding_status_count(db, "applied"))},
		{"/baro/founding/manage?tab=confirmed", tool.Baro_t("확정 ") + tool.Baro_itoa(baro_founding_status_count(db, "confirmed"))},
		{"/baro/founding/manage?tab=pending", tool.Baro_t("보류 ") + tool.Baro_itoa(baro_founding_status_count(db, "pending"))},
		{"/baro/founding/manage?tab=rejected", tool.Baro_t("반려 ") + tool.Baro_itoa(baro_founding_status_count(db, "rejected"))},
	}, "/baro/founding/manage?tab="+tab)

	rows := [][]string{}
	for _, person := range list {
		suspect := baro_founding_suspect(db, person)
		suspect_html := tool.Baro_badge(tool.Baro_t("특이사항 없음"), "ok")
		if len(suspect) > 0 {
			parts := []string{}
			for _, reason := range suspect {
				kind := "warn"
				if reason == "유효 기여 없음" {
					kind = "muted"
				}

				parts = append(parts, tool.Baro_badge(reason, kind))
			}

			suspect_html = strings.Join(parts, " ")
		}

		suspect_html += tool.Baro_note("가입 IP " + tool.Baro_mask_ip(baro_founding_register_ip(db, person.User)))

		action_html := ""
		switch person.Status {
		case "applied":
			action_html = baro_founding_manage_form("grant", person.User, tool.Baro_t("번호 발급"), "primary", tab)
			action_html += baro_founding_manage_form("reject", person.User, tool.Baro_t("반려"), "danger", tab)
		case "confirmed":
			action_html = baro_founding_manage_form("revoke", person.User, tool.Baro_t("번호 취소"), "danger", tab)
		case "pending":
			action_html = baro_founding_manage_form("grant", person.User, tool.Baro_t("번호 재발급"), "primary", tab)
			action_html += baro_founding_manage_form("restore", person.User, tool.Baro_t("대기로 되돌리기"), "", tab)
			action_html += baro_founding_manage_form("reject", person.User, tool.Baro_t("반려"), "danger", tab)
		case "rejected":
			action_html = baro_founding_manage_form("restore", person.User, tool.Baro_t("반려 취소"), "", tab)
		}

		number_html := `<span class="bw_muted">-</span>`
		if person.Number != "" {
			number_html = `<b>#` + tool.HTML_escape(person.Number) + `</b>`
		}

		rows = append(rows, []string{
			number_html,
			baro_founding_state_html(person),
			tool.Baro_user_link(db, person.User, config.IP),
			tool.HTML_escape(tool.Baro_field_name(db, person.Field)),
			tool.Baro_time_cell(person.ApplyDate),
			baro_founding_event_text(db, person.FirstEvent),
			suspect_html,
			tool.HTML_escape(tool.Baro_trunc(person.Note, 60)),
			action_html,
		})
	}

	if len(rows) == 0 {
		body += tool.Baro_card(tool.Baro_t("지원자 검토"), tool.Baro_empty(tool.Baro_t("이 상태에 해당하는 지원자가 없습니다.")))
	} else {
		body += tool.Baro_card(
			tool.Baro_t("지원자 검토"),
			tool.Baro_table(
				[]string{"번호", "상태", tool.Baro_t("사용자"), "분야", tool.Baro_t("지원일"), tool.Baro_t("첫 기여"), tool.Baro_t("의심 신호"), tool.Baro_t("메모"), tool.Baro_t("조치")},
				rows,
				tool.Baro_t("이 상태에 해당하는 지원자가 없습니다."),
			),
		)
	}

	body += tool.Baro_paging(page_int, len(rows), tool.Baro_page_size, "/baro/founding/manage/{}?tab="+tool.Url_parser(tab))

	body += tool.Baro_card(
		tool.Baro_t("의심 신호를 읽는 방법"),
		tool.Baro_list([]string{
			tool.Baro_t("<b>같은 가입 IP 계정 N개</b> — 같은 IP 에서 만들어진 계정 수입니다. 가족·학교·회사 공용망일 수 있으니 그 자체로 증거가 아닙니다."),
			tool.Baro_t("<b>접속 IP 공유 계정 N개</b> — 접속 기록(baro_login_log)에서 IP 가 겹치는 다른 계정 수입니다."),
			tool.Baro_t("<b>30분 안 지원 N건 몰림</b> — 짧은 시간에 지원이 몰린 구간입니다. 홍보 직후에도 나타날 수 있습니다."),
			tool.Baro_t("<b>유효 기여 없음</b> — 아직 첫 유효 기여가 없어 자동 발급 대상이 아닙니다."),
		})+
			tool.Baro_note(tool.Baro_t("IP 는 부분 가림 처리해 표시합니다. 이 화면의 정보를 공개 화면이나 공개 로그에 그대로 옮기지 마세요.")),
	)

	body += tool.Baro_danger_zone(
		tool.Baro_t("번호 취소 시 주의"),
		tool.Baro_t(`<p>번호를 취소하면 창립 기여자 배지가 회수되고 <b>보류</b> 상태가 됩니다. `)+
			tool.Baro_t(`보류 중에는 자동 승격이 번호를 다시 발급하지 않으므로 운영자의 조치가 저절로 뒤집히지 않습니다. `)+
			tool.Baro_t(`다시 심사하려면 <b>대기로 되돌리기</b> 를 누르세요. `)+
			tool.Baro_t(`이미 남긴 <b>기여 기록과 문서는 그대로 보존</b>되며 삭제되지 않습니다. `)+
			tool.Baro_t(`(0장 원칙 2 : 삭제보다 보존과 복구)</p>`)+
			tool.Baro_note(tool.Baro_t("모든 조치는 사유가 필수이며 공개 운영 로그와 내부 감사 기록에 함께 남습니다.")),
	)

	return tool.Baro_page(db, config, tool.Baro_t("창립 시즌 운영"), body, baro_founding_menu())
}
