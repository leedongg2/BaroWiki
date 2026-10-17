package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  편집방 — 누름 하나하나 처리하기 · 방 만들기 · 방 설정
// ============================================================

// baro_coop_do : 방 화면에서 온 처리. 돌려줄 것이 있으면 그것을 돌려준다.
func baro_coop_do(
	db *sql.DB, config tool.Config, room tool.Baro_coop_room_row,
	member tool.Baro_coop_member_row, is_in bool, can_manage bool, values url.Values,
) string {
	base := baro_coop_url(room.Link)
	act := values.Get("do")

	if !tool.Baro_is_user(config.IP) {
		return baro_coop_back(base, tool.Baro_t("로그인이 필요합니다."), "warn")
	}

	if tool.Baro_read_only(db) {
		return baro_coop_back(base, tool.Baro_t("지금은 읽기 전용입니다."), "warn")
	}

	switch act {
	// --- 참가 · 자리 바꾸기 · 나가기 ---
	case "join_edit", "to_edit":
		if !room.Baro_coop_room_live() {
			return baro_coop_back(base, tool.Baro_t("닫힌 방입니다."), "warn")
		}

		if !tool.Baro_coop_can_join(db, config.IP) {
			return baro_coop_back(base, tool.Baro_t("편집 참여 권한이 없습니다."), "warn")
		}

		// 편집이 막힌 사람은 관전으로만 있을 수 있다.
		if is_in && member.Block {
			return baro_coop_back(base,
				tool.Baro_t("편집 제한 중에는 편집 유저로 바꿀 수 없습니다."), "warn")
		}

		// 이미 편집 자리면 정원을 다시 세지 않는다. 방장은 정원 밖이다.
		if !(is_in && member.Baro_coop_member_editing()) && tool.Baro_coop_full_for(db, room, config.IP) {
			return baro_coop_back(base,
				tool.Baro_t("해당 방은 현재 최대 수용 상태입니다. 관전으로 들어오거나 자리가 나기를 기다려 주세요."), "warn")
		}

		tool.Baro_coop_member_set(db, room.ID, config.IP, tool.Baro_coop_edit)

		if !is_in {
			tool.Baro_coop_log_add(db, room.ID, "join", config.IP, "편집 유저로 참가", "")
		}

		return baro_coop_back(base, tool.Baro_t("편집 유저로 들어왔습니다."), "ok")

	case "join_watch", "to_watch":
		if !room.Baro_coop_room_live() {
			return baro_coop_back(base, tool.Baro_t("닫힌 방입니다."), "warn")
		}

		tool.Baro_coop_member_set(db, room.ID, config.IP, tool.Baro_coop_watch)

		if !is_in {
			tool.Baro_coop_log_add(db, room.ID, "join", config.IP, "관전 유저로 참가", "")
		}

		return baro_coop_back(base, tool.Baro_t("관전 유저가 되었습니다."), "ok")

	case "leave":
		if !is_in {
			return baro_coop_back(base, tool.Baro_t("들어와 있지 않습니다."), "warn")
		}

		tool.Baro_coop_member_out(db, room.ID, config.IP)
		tool.Baro_coop_log_add(db, room.ID, "leave", config.IP, "방에서 나감", "")

		if room.Owner == config.IP {
			tool.Baro_coop_log_add(db, room.ID, "system", "", "방장이 자리를 비웠습니다.", "")
		}

		return baro_coop_back("/baro/cooperation", tool.Baro_t("방에서 나왔습니다."), "ok")

	// --- 댓글 ---
	case "talk":
		if !is_in {
			return baro_coop_back(base+"/talk", tool.Baro_t("방에 들어와야 댓글을 달 수 있습니다."), "warn")
		}
		if !tool.Baro_coop_can(db, room, config.IP, "comment") {
			return baro_coop_back(base+"/talk", tool.Baro_t("이 방에서 댓글을 달 수 있는 자리가 아닙니다."), "warn")
		}
		if !room.Baro_coop_room_live() {
			return baro_coop_back(base+"/talk", tool.Baro_t("닫힌 방에는 댓글을 달 수 없습니다."), "warn")
		}

		data := strings.TrimSpace(values.Get("data"))
		if data == "" {
			return baro_coop_back(base+"/talk", tool.Baro_t("할 말을 적어 주세요."), "warn")
		}

		if !tool.Baro_rate_ok(db, "coop_talk", config.IP, 60, 600) {
			return baro_coop_back(base+"/talk", tool.Baro_t("잠시 뒤에 다시 달아 주세요."), "warn")
		}

		after, hit := tool.Baro_coop_ban_apply(data, tool.Baro_coop_ban_parse(room.Banned))

		tool.Baro_coop_log_add(db, room.ID, "comment", config.IP, after, "")

		if len(hit) > 0 {
			return baro_coop_back(base+"/talk", tool.Baro_coop_ban_notice(hit, after), "warn")
		}

		return tool.Get_redirect(base + "/talk")

	case "talk_hide":
		if !tool.Baro_coop_can(db, room, config.IP, "talk_hide") {
			return baro_coop_back(base+"/talk", tool.Baro_t("댓글을 지울 수 있는 자리가 아닙니다."), "warn")
		}

		tool.Baro_coop_log_hide(db, room.ID, values.Get("id"))

		return baro_coop_back(base+"/talk", tool.Baro_t("댓글을 지웠습니다."), "ok")

	// --- 방장 몫 ---
	case "notice":
		if !tool.Baro_coop_can(db, room, config.IP, "notice") {
			return baro_coop_back(base, tool.Baro_t("공지를 올릴 수 있는 자리가 아닙니다."), "warn")
		}

		notice := strings.TrimSpace(values.Get("notice"))
		tool.Baro_coop_room_notice_set(db, room.ID, notice)
		tool.Baro_coop_log_add(db, room.ID, "notice", config.IP, tool.Baro_trunc(notice, 200), "")

		return baro_coop_back(base, tool.Baro_t("공지를 올렸습니다."), "ok")

	case "sub_on", "sub_off":
		if room.Owner != config.IP && !tool.Baro_can(db, config.IP, "system") {
			return baro_coop_back(base, tool.Baro_t("부방장은 방장만 정할 수 있습니다."), "warn")
		}

		who := strings.TrimSpace(values.Get("user"))
		target, found := tool.Baro_coop_member_get(db, room.ID, who)

		if !found || target.Status != "in" {
			return baro_coop_back(base, tool.Baro_t("그 사람은 방에 없습니다."), "warn")
		}

		if act == "sub_on" {
			tool.Baro_coop_member_set(db, room.ID, who, tool.Baro_coop_sub)
			tool.Baro_coop_log_add(db, room.ID, "system", config.IP, who+" 님을 부방장으로 세웠습니다.", "")

			return baro_coop_back(base, tool.Baro_t("부방장으로 세웠습니다."), "ok")
		}

		tool.Baro_coop_member_set(db, room.ID, who, tool.Baro_coop_edit)
		tool.Baro_coop_log_add(db, room.ID, "system", config.IP, who+" 님을 부방장에서 내렸습니다.", "")

		return baro_coop_back(base, tool.Baro_t("부방장에서 내렸습니다."), "ok")

	case "block", "unblock":
		if !tool.Baro_coop_can(db, room, config.IP, "block") {
			return baro_coop_back(base, tool.Baro_t("유저 편집 제한을 할 수 있는 자리가 아닙니다."), "warn")
		}

		who := strings.TrimSpace(values.Get("user"))
		if who == room.Owner {
			return baro_coop_back(base, tool.Baro_t("방장은 제한할 수 없습니다."), "warn")
		}

		on := act == "block"
		reason := strings.TrimSpace(values.Get("reason"))

		if on && reason == "" {
			return baro_coop_back(base,
				tool.Baro_t("제한 사유를 적어 주세요. 사유 없는 제한은 규정 위반으로 다뤄집니다."), "warn")
		}

		tool.Baro_coop_member_block(db, room.ID, who, on)

		text := who + " 님의 편집 제한을 풀었습니다."
		if on {
			text = who + " 님이 이 방에서 편집할 수 없게 했습니다."
		}

		tool.Baro_coop_log_add(db, room.ID, "system", config.IP, text, reason)

		// 제한은 공개 기록으로도 남긴다. 24시간 안에 신고했는지 나중에 따질 수 있어야 한다.
		if on {
			tool.Baro_public_log(db, "coop_block", config.IP, "coop:"+room.Link, reason, who)
		} else {
			tool.Baro_public_log(db, "coop_unblock", config.IP, "coop:"+room.Link, reason, who)
		}

		tool.Baro_audit(db, config, "baro_coop_block", "coop:"+room.Link, "", who, reason)
		tool.Baro_notify(db, who, "system", tool.Baro_t("편집방 : ")+room.Name, base, text+" "+reason)

		if on {
			text += " " + tool.Baro_t("24시간 안에 신고하지 않으면 제한한 사람이 제재를 받을 수 있습니다.")
		}

		return baro_coop_back(base, text, "ok")

	// --- 초대 ---
	case "invite":
		if !tool.Baro_coop_can(db, room, config.IP, "invite") {
			return baro_coop_back(base, tool.Baro_t("사람을 부를 수 있는 자리가 아닙니다."), "warn")
		}

		who := strings.TrimSpace(values.Get("user"))
		if who == "" {
			return baro_coop_back(base, tool.Baro_t("부를 사람의 계정 이름을 적어 주세요."), "warn")
		}

		if !baro_coop_user_exist(db, who) {
			return baro_coop_back(base, tool.Baro_t("그런 사용자가 없습니다."), "danger")
		}

		if !tool.Baro_rate_ok(db, "coop_invite", config.IP, 30, 3600) {
			return baro_coop_back(base, tool.Baro_t("잠시 뒤에 다시 불러 주세요."), "warn")
		}

		if !tool.Baro_coop_invite(db, room, config.IP, who) {
			return baro_coop_back(base, tool.Baro_t("부르지 못했습니다."), "danger")
		}

		return baro_coop_back(base, who+tool.Baro_t(" 님을 불렀습니다."), "ok")

	// --- 방장 넘기기 ---
	case "hand_over":
		if room.Owner != config.IP {
			return baro_coop_back(base, tool.Baro_t("방장만 넘길 수 있습니다."), "warn")
		}

		who := strings.TrimSpace(values.Get("user"))
		if who == "" {
			return baro_coop_back(base, tool.Baro_t("넘길 사람을 골라 주세요."), "warn")
		}

		if !tool.Baro_coop_room_owner_set(db, room, who) {
			return baro_coop_back(base,
				tool.Baro_t("이 방에 든 적이 있는 사람에게만 넘길 수 있습니다."), "warn")
		}

		tool.Baro_coop_log_add(db, room.ID, "system", config.IP, who+" 님에게 방장직을 넘겼습니다.", "")
		tool.Baro_public_log(db, "coop_owner", config.IP, "coop:"+room.Link, "", who)
		tool.Baro_notify(db, who, "system",
			tool.Baro_t("편집방 방장이 되었습니다 : ")+room.Name, base, config.IP+tool.Baro_t(" 님이 넘겼습니다."))

		return baro_coop_back(base, who+tool.Baro_t(" 님에게 방장직을 넘겼습니다."), "ok")

	// --- 영구 폐쇄 (관리자만) ---
	case "seal":
		if !tool.Baro_can(db, config.IP, "system") {
			return baro_coop_back(base, tool.Baro_t("영구 폐쇄는 관리자만 할 수 있습니다."), "warn")
		}

		reason := strings.TrimSpace(values.Get("reason"))
		if reason == "" {
			return baro_coop_back(base, tool.Baro_t("영구 폐쇄 사유를 적어 주세요."), "warn")
		}

		tool.Baro_coop_room_seal(db, room.ID, config.IP, reason)
		tool.Baro_coop_log_add(db, room.ID, "system", config.IP, "방이 영구 폐쇄되었습니다.", reason)
		tool.Baro_public_log(db, "coop_seal", config.IP, "coop:"+room.Link, reason, room.Name)
		tool.Baro_audit(db, config, "baro_coop_seal", "coop:"+room.Link, room.Status, "sealed", reason)

		for _, one := range tool.Baro_coop_member_list(db, room.ID, "in") {
			tool.Baro_notify(db, one.User, "system",
				tool.Baro_t("편집방이 영구 폐쇄되었습니다 : ")+room.Name, base, reason)
		}

		return baro_coop_back(base, tool.Baro_t("영구 폐쇄했습니다."), "warn")

	case "unseal":
		if !tool.Baro_can(db, config.IP, "system") {
			return baro_coop_back(base, tool.Baro_t("영구 폐쇄는 관리자만 풀 수 있습니다."), "warn")
		}

		reason := strings.TrimSpace(values.Get("reason"))

		tool.Baro_coop_room_unseal(db, room.ID)
		tool.Baro_coop_log_add(db, room.ID, "system", config.IP, "영구 폐쇄를 풀었습니다.", reason)
		tool.Baro_public_log(db, "coop_unseal", config.IP, "coop:"+room.Link, reason, room.Name)
		tool.Baro_audit(db, config, "baro_coop_unseal", "coop:"+room.Link, "sealed", "open", reason)

		return baro_coop_back(base, tool.Baro_t("영구 폐쇄를 풀었습니다."), "ok")

	case "close":
		if room.Owner != config.IP && !tool.Baro_can(db, config.IP, "system") {
			return baro_coop_back(base, tool.Baro_t("방은 방장만 닫을 수 있습니다."), "warn")
		}

		if room.Baro_coop_room_sealed() {
			return baro_coop_back(base, tool.Baro_t("이미 영구 폐쇄된 방입니다."), "warn")
		}

		tool.Baro_coop_room_close(db, room.ID)
		tool.Baro_coop_log_add(db, room.ID, "system", config.IP, "10분 후 방이 폐쇄됩니다.", "")

		// 방에 있는 사람 모두에게 알린다.
		for _, one := range tool.Baro_coop_member_list(db, room.ID, "in") {
			if one.User == config.IP {
				continue
			}

			tool.Baro_notify(db, one.User, "system",
				tool.Baro_t("10분 후 방이 폐쇄됩니다.")+" : "+room.Name, base,
				tool.Baro_t("편집 중이라면 이어서 끝내도 됩니다. 방이 닫힌 뒤의 편집은 방 밖에서 한 편집으로 남습니다."))
		}

		return baro_coop_back(base, tool.Baro_t("10분 후 방이 폐쇄됩니다."), "warn")

	case "reopen":
		if room.Owner != config.IP && !tool.Baro_can(db, config.IP, "system") {
			return baro_coop_back(base, tool.Baro_t("방은 방장만 열 수 있습니다."), "warn")
		}

		// 영구 폐쇄는 관리자만 푼다. 방장이 이 길로 풀 수 없다.
		if room.Baro_coop_room_sealed() && !tool.Baro_can(db, config.IP, "system") {
			return baro_coop_back(base,
				tool.Baro_t("영구 폐쇄된 방입니다. 관리자만 풀 수 있습니다."), "warn")
		}

		tool.Baro_coop_room_reopen(db, room.ID)
		tool.Baro_coop_log_add(db, room.ID, "system", config.IP, "방을 다시 열었습니다.", "")

		return baro_coop_back(base, tool.Baro_t("방을 다시 열었습니다."), "ok")
	}

	return ""
}

// ------------------------------------------------------------
//  방 만들기
// ------------------------------------------------------------

// View_baro_coop_new : 편집방 만들기 (/baro/cooperation/new)
func View_baro_coop_new(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, stop := tool.Baro_login_wall(db, config, "/baro/cooperation/new"); stop {
		return wall
	}

	if !tool.Baro_coop_can_open(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("지금은 편집방을 열 수 있는 권한이 없습니다."))
	}

	row := tool.Baro_coop_room_row{OpenKind: tool.Baro_coop_public, MaxUser: 20, Owner: config.IP}
	row.Acl = tool.Baro_coop_acl_text(map[string]string{})
	alert := ""

	if values != nil && values.Get("do") == "add" {
		next, problem := baro_coop_form_read(values, row)
		row = next

		switch {
		case problem != "":
			alert = tool.Baro_alert("warn", problem)
		case !tool.Baro_rate_ok(db, "coop_add", config.IP, 10, 3600):
			alert = tool.Baro_alert("warn", tool.Baro_t("잠시 뒤에 다시 시도해 주세요."))
		default:
			row.Owner = config.IP

			link := tool.Baro_coop_room_add(db, row)
			if link == "" {
				alert = tool.Baro_alert("danger", tool.Baro_t("방을 만들지 못했습니다. 다시 시도해 주세요."))
			} else {
				tool.Baro_audit(db, config, "baro_coop_add", "coop:"+link, "", row.Name, "")

				return baro_coop_back(baro_coop_url(link), tool.Baro_t("편집방을 열었습니다."), "ok")
			}
		}
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"}, {"/baro/cooperation", tool.Baro_t("편집방")}, {"", tool.Baro_t("만들기")},
	})
	body += tool.Baro_hero(tool.Baro_t("편집방 만들기"), tool.Baro_t("모여서 함께 손볼 자리를 엽니다."), "")
	body += baro_coop_tabs("/baro/cooperation/new")
	body += alert
	body += tool.Baro_card("", baro_coop_form_html("/baro/cooperation/new", "add", row, tool.Baro_t("방 열기")))

	return tool.Baro_page(db, config, tool.Baro_t("편집방 만들기"), body, baro_coop_menu())
}

// baro_coop_form_read : 폼에서 방 설정을 읽는다. (방, 잘못된 곳)
func baro_coop_form_read(values url.Values, before tool.Baro_coop_room_row) (tool.Baro_coop_room_row, string) {
	row := before
	row.Name = strings.TrimSpace(values.Get("name"))
	row.Description = strings.TrimSpace(values.Get("description"))
	row.MaxUser = tool.Str_to_int(values.Get("max_user"))

	kind := strings.TrimSpace(values.Get("open_kind"))
	row.OpenKind = tool.Baro_coop_private

	for _, one := range tool.Baro_coop_open_kind_list() {
		if one[0] == kind {
			row.OpenKind = kind
			break
		}
	}

	row.Banned = tool.Baro_coop_ban_text(tool.Baro_coop_ban_parse(values.Get("banned")))

	// 행위별로 어느 자리까지 열어 줄지
	pick := map[string]string{}
	for _, one := range tool.Baro_coop_act_list() {
		pick[one[0]] = values.Get("acl_" + one[0])
	}

	row.Acl = tool.Baro_coop_acl_text(pick)

	switch {
	case row.Name == "":
		return row, tool.Baro_t("방 이름을 적어 주세요.")
	case tool.Get_len(row.Name) > 60:
		return row, tool.Baro_t("방 이름이 너무 깁니다.")
	case row.MaxUser < 1 || row.MaxUser > tool.Baro_coop_max_limit:
		return row, tool.Baro_t("최대 수용 인원은 1명부터 ") + tool.Baro_itoa(tool.Baro_coop_max_limit) + tool.Baro_t("명까지입니다.")
	}

	return row, ""
}

// baro_coop_form_html : 방 설정 폼
func baro_coop_form_html(action string, do string, row tool.Baro_coop_room_row, submit string) string {
	form := tool.Baro_hidden("do", do)
	form += tool.Baro_input("name", tool.Baro_t("방 이름"), row.Name, tool.Baro_t("예: 독립운동가 문서 정리"), "")
	form += tool.Baro_textarea("description", tool.Baro_t("방 설명"), row.Description, 3,
		tool.Baro_t("무엇을 할 방인지 적어 주세요."), "")

	kinds := [][]string{}
	for _, one := range tool.Baro_coop_open_kind_list() {
		kinds = append(kinds, []string{one[0], one[1]})
	}

	form += tool.Baro_select("open_kind", tool.Baro_t("공개 상태"), kinds, row.OpenKind,
		tool.Baro_t("공개는 목록에 오르고, 일부 공개는 링크나 12자리 열쇠로만 들어옵니다. 비공개는 방장이 부른 사람만 봅니다."))

	form += tool.Baro_input_type("max_user", tool.Baro_t("최대 수용 인원"), tool.Baro_itoa(row.MaxUser), "number",
		tool.Baro_t("1명부터 200명까지. 편집 유저만 셉니다. 관전은 세지 않습니다."))

	form += tool.Baro_textarea("banned", tool.Baro_t("금칙어"), row.Banned, 4,
		"A|(No)\npl|ㄱㄴ",
		tool.Baro_t("한 줄에 하나씩 「금칙어|바꿀 글자」 로 적습니다. 댓글에 그 말이 있으면 바꿔 넣고 알려 줍니다."))

	form += baro_coop_acl_form(row)
	form += tool.Baro_submit(submit, "primary")

	return tool.Baro_form(action, form)
}

// baro_coop_acl_form : 방 안에서 무엇을 어느 자리까지 열어 줄지 고르는 칸
//
//	방장은 무엇이든 할 수 있으므로 고를 수 있는 것은 「방장만 / 부방장까지 / 참여자 모두」 셋이다.
func baro_coop_acl_form(row tool.Baro_coop_room_row) string {
	scopes := [][]string{}
	for _, one := range tool.Baro_coop_scope_list() {
		scopes = append(scopes, []string{one[0], one[1]})
	}

	inner := ""
	for _, one := range tool.Baro_coop_act_list() {
		inner += tool.Baro_select("acl_"+one[0], one[1], scopes, tool.Baro_coop_acl_get(row, one[0]), one[3])
	}

	return tool.Baro_section(tool.Baro_t("방 안 권한"), inner)
}

// ------------------------------------------------------------
//  방 설정 고치기
// ------------------------------------------------------------

// View_baro_coop_setting : 방 설정 (/baro/cooperation/<이름표>/setting)
func View_baro_coop_setting(config tool.Config, link string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	room, found := tool.Baro_coop_room_get(db, link)
	if !found {
		return baro_coop_back("/baro/cooperation", tool.Baro_t("그런 편집방이 없습니다."), "danger")
	}

	if !tool.Baro_coop_can(db, room, config.IP, "setting") {
		return tool.Baro_deny(db, config, tool.Baro_t("이 방의 설정을 고칠 수 있는 자리가 아닙니다."))
	}

	base := baro_coop_url(room.Link)
	alert := ""

	if values != nil && values.Get("do") == "edit" {
		next, problem := baro_coop_form_read(values, room)

		// 정원은 지금 있는 사람보다 적게 줄일 수 없다.
		edit_count, _ := tool.Baro_coop_count(db, room.ID)

		switch {
		case problem != "":
			alert = tool.Baro_alert("warn", problem)
			room = next
		case next.MaxUser < edit_count:
			alert = tool.Baro_alert("warn", tool.Baro_t("현재 인원보다 크거나 같은 값을 입력해주세요."))
			room = next
		default:
			next.ID = room.ID
			tool.Baro_coop_room_save(db, next)
			tool.Baro_audit(db, config, "baro_coop_edit", "coop:"+room.Link, room.Name, next.Name, "")

			return baro_coop_back(base, tool.Baro_t("방 설정을 고쳤습니다."), "ok")
		}
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"}, {"/baro/cooperation", tool.Baro_t("편집방")},
		{base, room.Name}, {"", tool.Baro_t("설정")},
	})
	body += tool.Baro_hero(tool.Baro_t("방 설정"), room.Name, "")
	body += alert

	edit_count, _ := tool.Baro_coop_count(db, room.ID)
	body += tool.Baro_note(tool.Baro_t("지금 편집 자리에 있는 사람은 ") + tool.Baro_itoa(edit_count) +
		tool.Baro_t("명입니다. 최대 수용 인원을 그보다 적게 줄일 수 없습니다."))

	body += tool.Baro_card("", baro_coop_form_html(base+"/setting", "edit", room, tool.Baro_t("고치기")))
	body += tool.Baro_button(base, tool.Baro_t("방으로 돌아가기"), "")

	return tool.Baro_page(db, config, tool.Baro_t("방 설정"), body, baro_coop_menu())
}

// baro_coop_user_exist : 그런 계정이 있는가
func baro_coop_user_exist(db *sql.DB, user string) bool {
	if !tool.Baro_is_user(user) {
		return false
	}

	found := ""

	return tool.QueryRow_DB(db, "select id from user_set where id = ? limit 1", []any{&found}, user)
}
