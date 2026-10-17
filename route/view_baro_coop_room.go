package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  편집방 하나 보기  /baro/cooperation/<이름표>
// ============================================================

// View_baro_coop_room : 방 하나
func View_baro_coop_room(config tool.Config, link string, tab string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	room, found := tool.Baro_coop_room_get(db, link)
	if !found {
		body := tool.Baro_alert("danger", tool.Baro_t("그런 편집방이 없습니다. 주소를 다시 확인해 주세요."))
		body += tool.Baro_button("/baro/cooperation", tool.Baro_t("편집방 목록"), "primary")

		return tool.Baro_page(db, config, tool.Baro_t("편집방"), body, baro_coop_menu())
	}

	member, in_room := tool.Baro_coop_member_get(db, room.ID, config.IP)
	is_in := in_room && member.Status == "in"
	can_manage := tool.Baro_coop_can_manage(db, room, config.IP)

	// 열람 권한 자체가 없으면 어떤 방도 보지 못한다.
	if !tool.Baro_coop_can_view(db, config.IP) {
		return baro_coop_shut(db, config, tool.Baro_t("편집방을 열람할 수 있는 권한이 없습니다."))
	}

	// 비공개 방은 이름이 올라 있는 사람과 운영진만 본다.
	// 링크를 타고 들어와도 마찬가지다.
	if room.OpenKind == tool.Baro_coop_private && !in_room && !tool.Baro_can(db, config.IP, "system") {
		return baro_coop_shut(db, config,
			tool.Baro_t("열람이 제한된 방입니다.")+" "+
				tool.Baro_t("비공개 편집방은 방장이 불러 준 사람만 볼 수 있습니다."))
	}

	// 처리
	if values != nil && values.Get("do") != "" {
		if out := baro_coop_do(db, config, room, member, is_in, can_manage, values); out != "" {
			return out
		}
	}

	// 처리하면서 바뀐 것이 있으니 다시 읽는다.
	room, _ = tool.Baro_coop_room_get(db, link)
	member, in_room = tool.Baro_coop_member_get(db, room.ID, config.IP)
	is_in = in_room && member.Status == "in"
	can_manage = tool.Baro_coop_can_manage(db, room, config.IP)

	edit_count, watch_count := tool.Baro_coop_count(db, room.ID)

	if tab == "" {
		tab = "room"
	}

	sub := tool.Baro_badge(tool.Baro_coop_open_kind_name(room.OpenKind), "info")
	if tool.Baro_coop_owner_away(db, room) {
		sub += ` <span class="bw_coop_away">(` + tool.Baro_t_esc("방장 부재") + `)</span>`
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"}, {"/baro/cooperation", tool.Baro_t("편집방")}, {"", room.Name},
	})
	body += tool.Baro_hero_html(room.Name, sub, "")
	body += baro_coop_css()
	body += baro_coop_alert(values)
	body += baro_coop_state_alert(db, room)

	base := baro_coop_url(room.Link)
	body += tool.Baro_tabs([][]string{
		{base, tool.Baro_t("방")},
		{base + "/work", tool.Baro_t("작업")},
		{base + "/talk", tool.Baro_t("댓글")},
		{base + "/detail", tool.Baro_t("상세")},
	}, base+baro_coop_tab_suffix(tab))

	// 공지
	if strings.TrimSpace(room.Notice) != "" {
		body += tool.Baro_card(tool.Baro_t("공지"), `<p class="bw_coop_ban">`+tool.HTML_escape(room.Notice)+`</p>`)
	}

	switch tab {
	case "talk":
		body += baro_coop_talk_html(db, config, room, is_in, can_manage)
	case "detail":
		body += baro_coop_detail_html(db, config, room)
	default:
		body += baro_coop_room_html(db, config, room, member, is_in, can_manage, edit_count, watch_count)
	}

	if can_manage {
		body += tool.Baro_card("", tool.Baro_button(base+"/setting", tool.Baro_t("방 설정"), "primary"))
	}

	return tool.Baro_page(db, config, room.Name, body, baro_coop_menu())
}

// baro_coop_tab_suffix : 지금 탭의 주소 꼬리
func baro_coop_tab_suffix(tab string) string {
	switch tab {
	case "talk":
		return "/talk"
	case "detail":
		return "/detail"
	}

	return ""
}

// baro_coop_state_alert : 방 상태 안내 (곧 닫음 · 닫힘)
func baro_coop_state_alert(db *sql.DB, room tool.Baro_coop_room_row) string {
	switch room.Status {
	case tool.Baro_coop_closing:
		return tool.Baro_alert("warn",
			tool.Baro_t("10분 후 방이 폐쇄됩니다.")+" ("+tool.Baro_duration_left(room.CloseAt)+") "+
				tool.Baro_t("편집 중이라면 이어서 끝내도 됩니다. 방이 닫힌 뒤의 편집은 방 밖에서 한 편집으로 남습니다."))
	case tool.Baro_coop_closed:
		return tool.Baro_alert("muted",
			tool.Baro_t("닫힌 편집방입니다. 여기서 한 일은 그대로 남아 있고, 방장이 다시 열 수 있습니다."))
	case tool.Baro_coop_sealed:
		text := tool.Baro_t("영구 폐쇄된 편집방입니다. 관리자만 풀 수 있습니다.")
		if room.SealReason != "" {
			text += " " + tool.Baro_t("사유 : ") + room.SealReason
		}

		return tool.Baro_alert("danger", text)
	}

	return ""
}

// baro_coop_shut : 볼 수 없는 방을 알리는 화면
func baro_coop_shut(db *sql.DB, config tool.Config, text string) string {
	body := tool.Baro_alert("warn", text)
	body += tool.Baro_button("/baro/cooperation", tool.Baro_t("편집방 목록"), "primary")

	return tool.Baro_page(db, config, tool.Baro_t("편집방"), body, baro_coop_menu())
}

// baro_coop_room_html : 「방」 탭
func baro_coop_room_html(
	db *sql.DB, config tool.Config, room tool.Baro_coop_room_row,
	member tool.Baro_coop_member_row, is_in bool, can_manage bool,
	edit_count int, watch_count int,
) string {
	base := baro_coop_url(room.Link)

	html := ""

	if strings.TrimSpace(room.Description) != "" {
		html += tool.Baro_card(tool.Baro_t("방 설명"), `<p class="bw_coop_ban">`+tool.HTML_escape(room.Description)+`</p>`)
	}

	html += tool.Baro_stat_row([][]string{
		{tool.Baro_t("인원"), tool.Baro_itoa(edit_count) + "/" + tool.Baro_itoa(room.MaxUser), tool.Baro_t("편집 자리")},
		{tool.Baro_t("관전"), tool.Baro_itoa(watch_count) + tool.Baro_t("명"), ""},
		{tool.Baro_t("손댄 문서"), tool.Baro_itoa(len(tool.Baro_coop_doc_list(db, room.ID))) + tool.Baro_t("개"), ""},
	})

	// --- 참여자 ---
	html += tool.Baro_card(tool.Baro_t("참여자"), baro_coop_member_html(db, config, room)+
		baro_coop_watch_html(db, config, room))

	// --- 손댄 문서 ---
	html += baro_coop_doc_card(db, room)

	// --- 들어가기 · 나가기 ---
	html += baro_coop_join_html(db, config, room, member, is_in, base)

	// --- 방장 몫 ---
	//   자리마다 할 수 있는 것이 다르므로, 하나라도 할 수 있으면 칸을 보여 준다.
	if can_manage || tool.Baro_coop_can(db, room, config.IP, "invite") {
		html += baro_coop_manage_html(db, config, room, base)
	}

	return html
}

// baro_coop_watch_html : 관전하는 사람
func baro_coop_watch_html(db *sql.DB, config tool.Config, room tool.Baro_coop_room_row) string {
	names := []string{}

	for _, one := range tool.Baro_coop_member_list(db, room.ID, "in") {
		if one.Baro_coop_member_editing() {
			continue
		}

		text := tool.Baro_user_tag(db, one.User, config.IP)

		// 편집이 막힌 사람은 관전으로 잡히므로 여기에 나온다. 까닭을 함께 보여 준다.
		if one.Block {
			text += " " + tool.Baro_badge(tool.Baro_t("편집 제한"), "danger")
		}

		names = append(names, text)
	}

	if len(names) == 0 {
		return `<p><b>` + tool.Baro_t_esc("관전") + `:</b> 0` + tool.Baro_t_esc("명") + `</p>`
	}

	return `<p><b>` + tool.Baro_t_esc("관전") + `:</b> ` +
		tool.HTML_escape(tool.Baro_itoa(len(names))) + tool.Baro_t_esc("명") + ` — ` +
		strings.Join(names, ", ") + `</p>`
}

// baro_coop_doc_card : 손댄 문서
func baro_coop_doc_card(db *sql.DB, room tool.Baro_coop_room_row) string {
	list := tool.Baro_coop_doc_list(db, room.ID)

	rows := [][]string{}
	for _, one := range list {
		num := tool.Baro_itoa(one.Num)
		if one.Num > 1 {
			num = `<b>*` + num + `</b>`
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(one.Title),
			num,
			tool.Baro_time_cell(one.Date),
		})
	}

	return tool.Baro_card_ex(
		tool.Baro_t("손댄 문서"),
		tool.Baro_t("방에 편집 자리로 들어와 있는 동안 고친 문서가 저절로 적힙니다."),
		tool.Baro_table(
			[]string{tool.Baro_t("문서"), tool.Baro_t("횟수"), tool.Baro_t("마지막")},
			rows, tool.Baro_t("아직 손댄 문서가 없습니다."),
		),
		"",
	)
}

// baro_coop_join_html : 들어가기 · 자리 바꾸기 · 나가기
func baro_coop_join_html(
	db *sql.DB, config tool.Config, room tool.Baro_coop_room_row,
	member tool.Baro_coop_member_row, is_in bool, base string,
) string {
	if !tool.Baro_is_user(config.IP) {
		return tool.Baro_note(tool.Baro_t("로그인하면 편집방에 들어올 수 있습니다."))
	}

	if !room.Baro_coop_room_live() {
		return tool.Baro_note(tool.Baro_t("닫힌 방이라 새로 들어올 수 없습니다."))
	}

	// 방장은 정원 밖이므로 언제나 제 방에 들어갈 수 있다.
	full := tool.Baro_coop_full_for(db, room, config.IP)

	if !is_in {
		inner := ""

		if full {
			inner += tool.Baro_alert("warn",
				tool.Baro_t("해당 방은 현재 최대 수용 상태입니다.")+" [ "+tool.Baro_t("관전하기")+" ] [ "+tool.Baro_t("나가기")+" ]")
			inner += tool.Baro_form(base, tool.Baro_hidden("do", "join_watch")+
				tool.Baro_submit(tool.Baro_t("관전하기"), "primary"))
			inner += tool.Baro_button("/baro/cooperation", tool.Baro_t("나가기"), "")
		} else {
			inner += tool.Baro_form(base, tool.Baro_hidden("do", "join_edit")+
				tool.Baro_submit(tool.Baro_t("편집 유저로 참가"), "primary"))
			inner += tool.Baro_form(base, tool.Baro_hidden("do", "join_watch")+
				tool.Baro_submit(tool.Baro_t("관전 유저로 참가"), ""))
		}

		return tool.Baro_card(tool.Baro_t("참가"), inner)
	}

	inner := tool.Baro_dl([][]string{
		{tool.Baro_t("내 자리"), tool.Baro_t_esc(tool.Baro_coop_role_name(member.Role))},
	})

	if member.Block {
		inner += tool.Baro_alert("danger",
			tool.Baro_t("방장이 이 방에서의 편집을 제한했습니다. 여기서 한 편집은 방 기록에 들어가지 않습니다."))
	}

	switch {
	case member.Block:
		// 편집이 막힌 사람은 관전으로 보고, 편집 유저로 바꿀 수 없다.
		inner += tool.Baro_note(tool.Baro_t("편집 제한 중에는 관전 유저로만 있을 수 있습니다."))
	case member.Role == tool.Baro_coop_watch && !tool.Baro_coop_can_join(db, config.IP):
		inner += tool.Baro_note(tool.Baro_t("편집 참여 권한이 없습니다."))
	case member.Role == tool.Baro_coop_watch && full:
		inner += tool.Baro_note(tool.Baro_t("편집 자리가 다 차서 지금은 편집 유저로 바꿀 수 없습니다."))
	case member.Role == tool.Baro_coop_watch:
		inner += tool.Baro_form(base, tool.Baro_hidden("do", "to_edit")+
			tool.Baro_submit(tool.Baro_t("편집 유저로 바꾸기"), "primary"))
	case member.Role == tool.Baro_coop_edit:
		inner += tool.Baro_form(base, tool.Baro_hidden("do", "to_watch")+
			tool.Baro_submit(tool.Baro_t("관전 유저로 바꾸기"), ""))
	}

	if member.Baro_coop_member_editing() && !member.Block && room.Baro_coop_room_live() {
		inner += tool.Baro_note(tool.Baro_t("지금부터 고치는 문서는 이 방에서 한 편집으로 적힙니다."))
	}

	inner += tool.Baro_form(base, tool.Baro_hidden("do", "leave")+
		tool.Baro_submit(tool.Baro_t("방에서 나가기"), "danger"))

	return tool.Baro_card(tool.Baro_t("내 자리"), inner)
}

// baro_coop_manage_html : 방장·부방장 몫
func baro_coop_manage_html(db *sql.DB, config tool.Config, room tool.Baro_coop_room_row, base string) string {
	inner := ""

	// 공지
	if tool.Baro_coop_can(db, room, config.IP, "notice") {
		inner += baro_coop_notice_form(room, base)
	}

	inner += baro_coop_people_html(db, config, room, base)

	return baro_coop_manage_rest(db, config, room, base, inner)
}

// baro_coop_notice_form : 공지 칸
func baro_coop_notice_form(room tool.Baro_coop_room_row, base string) string {
	notice_form := tool.Baro_hidden("do", "notice")
	notice_form += tool.Baro_textarea("notice", tool.Baro_t("공지"), room.Notice, 3,
		tool.Baro_t("방에 들어온 사람 모두에게 보입니다."), "")
	notice_form += tool.Baro_submit(tool.Baro_t("공지 올리기"), "")

	return tool.Baro_section(tool.Baro_t("공지"), tool.Baro_form(base, notice_form))
}

// baro_coop_people_html : 사람 다루기 칸
func baro_coop_people_html(db *sql.DB, config tool.Config, room tool.Baro_coop_room_row, base string) string {
	can_block := tool.Baro_coop_can(db, room, config.IP, "block")

	people := [][]string{}
	for _, one := range tool.Baro_coop_member_list(db, room.ID, "in") {
		if one.User == room.Owner {
			continue
		}

		act := ""

		if one.Role == tool.Baro_coop_sub {
			act += tool.Baro_form(base, tool.Baro_hidden("do", "sub_off")+
				tool.Baro_hidden("user", one.User)+tool.Baro_submit(tool.Baro_t("부방장 내리기"), ""))
		} else if one.Baro_coop_member_editing() && room.Owner == config.IP {
			act += tool.Baro_form(base, tool.Baro_hidden("do", "sub_on")+
				tool.Baro_hidden("user", one.User)+tool.Baro_submit(tool.Baro_t("부방장 주기"), ""))
		}

		if can_block {
			if one.Block {
				act += tool.Baro_form(base, tool.Baro_hidden("do", "unblock")+
					tool.Baro_hidden("user", one.User)+tool.Baro_submit(tool.Baro_t("편집 제한 풀기"), ""))
			} else {
				act += tool.Baro_form(base, tool.Baro_hidden("do", "block")+
					tool.Baro_hidden("user", one.User)+
					`<input class="bw_in" type="text" name="reason" placeholder="`+
					tool.Baro_t_esc("제한 사유")+`">`+
					tool.Baro_submit(tool.Baro_t("편집 제한"), "danger"))
			}
		}

		state := tool.Baro_t_esc(tool.Baro_coop_role_name(one.Role))
		if one.Block {
			state = tool.Baro_t_esc(tool.Baro_coop_role_name(tool.Baro_coop_watch)) + " " +
				tool.Baro_badge(tool.Baro_t("편집 제한"), "danger")
		}

		people = append(people, []string{
			tool.Baro_user_tag(db, one.User, config.IP),
			state,
			act,
		})
	}

	out := tool.Baro_section(tool.Baro_t("사람"), tool.Baro_table(
		[]string{tool.Baro_t("사람"), tool.Baro_t("자리"), tool.Baro_t("다루기")},
		people, tool.Baro_t("방장 말고는 아직 아무도 없습니다."),
	))

	if can_block {
		out += tool.Baro_note(tool.Baro_t(
			"편집 제한은 규정을 어긴 것이 뚜렷할 때만 씁니다. 제한한 뒤 24시간 안에 신고하지 않거나 " +
				"적법하지 않은 사유로 제한하면, 제한한 사람이 5일에서 30일까지 차단되고 방장직을 잃을 수 있습니다."))
	}

	return out
}

// baro_coop_manage_rest : 초대 · 방장 넘기기 · 열고 닫기
func baro_coop_manage_rest(
	db *sql.DB, config tool.Config, room tool.Baro_coop_room_row, base string, inner string,
) string {

	// 초대
	if tool.Baro_coop_can(db, room, config.IP, "invite") {
		invite := tool.Baro_hidden("do", "invite")
		invite += tool.Baro_input("user", tool.Baro_t("부를 사람"), "", tool.Baro_t("계정 이름"),
			tool.Baro_t("알림으로 방 링크와 열쇠를 함께 보냅니다."))
		invite += tool.Baro_submit(tool.Baro_t("초대하기"), "")

		inner += tool.Baro_section(tool.Baro_t("초대"), tool.Baro_form(base, invite))
	}

	// 방장 넘기기 (방장만)
	if room.Owner == config.IP {
		options := [][]string{{"", tool.Baro_t("(고르지 않음)")}}

		for _, one := range tool.Baro_coop_room_ever_list(db, room.ID) {
			if one.User == room.Owner {
				continue
			}

			label := one.User
			if one.Status != "in" {
				label += " " + tool.Baro_t("(나간 사람)")
			}

			options = append(options, []string{one.User, label})
		}

		if len(options) > 1 {
			hand := tool.Baro_hidden("do", "hand_over")
			hand += tool.Baro_select("user", tool.Baro_t("넘길 사람"), options, "",
				tool.Baro_t("이 방에 든 적이 있는 사람에게만 넘길 수 있습니다. 넘기면 나는 편집 자리가 됩니다."))
			hand += tool.Baro_submit(tool.Baro_t("방장직 넘기기"), "danger")

			inner += tool.Baro_section(tool.Baro_t("방장 넘기기"), tool.Baro_form(base, hand))
		} else {
			inner += tool.Baro_section(tool.Baro_t("방장 넘기기"),
				tool.Baro_note(tool.Baro_t("이 방에 든 적이 있는 다른 사람이 아직 없습니다.")))
		}
	}

	// 열고 닫기 (방장만)
	if room.Owner == config.IP || tool.Baro_can(db, config.IP, "system") {
		switch room.Status {
		case tool.Baro_coop_closed:
			inner += tool.Baro_section(tool.Baro_t("방 열기"),
				tool.Baro_form(base, tool.Baro_hidden("do", "reopen")+
					tool.Baro_submit(tool.Baro_t("방 다시 열기"), "primary")))
		case tool.Baro_coop_closing:
			inner += tool.Baro_section(tool.Baro_t("방 닫기"),
				tool.Baro_note(tool.Baro_t("닫는 중입니다. ")+tool.Baro_duration_left(room.CloseAt))+
					tool.Baro_form(base, tool.Baro_hidden("do", "reopen")+
						tool.Baro_submit(tool.Baro_t("닫기 그만두기"), "")))
		default:
			inner += tool.Baro_section(tool.Baro_t("방 닫기"),
				tool.Baro_note(tool.Baro_t("닫겠다고 알린 뒤 10분 있다가 닫힙니다."))+
					tool.Baro_form(base, tool.Baro_hidden("do", "close")+
						tool.Baro_submit(tool.Baro_t("방 폐쇄하기"), "danger")))
		}
	}

	return tool.Baro_card(tool.Baro_t("방 다루기"), inner) +
		baro_coop_seal_html(db, config, room, base)
}

// baro_coop_seal_html : 영구 폐쇄 칸 (관리자만 보인다)
//
//	방장이 하는 보통 폐쇄와 달리, 걸고 푸는 것을 모두 관리자만 한다.
func baro_coop_seal_html(db *sql.DB, config tool.Config, room tool.Baro_coop_room_row, base string) string {
	if !tool.Baro_can(db, config.IP, "system") {
		return ""
	}

	if room.Baro_coop_room_sealed() {
		inner := tool.Baro_dl([][]string{
			{tool.Baro_t("폐쇄한 사람"), tool.Baro_user_tag(db, room.SealBy, config.IP)},
			{tool.Baro_t("사유"), tool.HTML_escape(room.SealReason)},
			{tool.Baro_t("때"), tool.Baro_time_cell(room.SealDate)},
		})
		inner += tool.Baro_form(base, tool.Baro_hidden("do", "unseal")+
			tool.Baro_input("reason", tool.Baro_t("푸는 까닭"), "", "", "")+
			tool.Baro_submit(tool.Baro_t("영구 폐쇄 풀기"), "primary"))

		return tool.Baro_card(tool.Baro_t("영구 폐쇄"), inner)
	}

	return tool.Baro_danger_zone(tool.Baro_t("영구 폐쇄"),
		tool.Baro_note(tool.Baro_t("관리자만 걸 수 있고, 관리자만 풀 수 있습니다. 방장은 스스로 풀지 못합니다."))+
			tool.Baro_note(tool.Baro_t("적법하지 않은 사유로 영구 폐쇄하면 5일 이상 차단될 수 있습니다."))+
			tool.Baro_form(base, tool.Baro_hidden("do", "seal")+
				tool.Baro_input("reason", tool.Baro_t("폐쇄 사유"), "", "", tool.Baro_t("반드시 적어야 합니다."))+
				tool.Baro_submit(tool.Baro_t("영구 폐쇄하기"), "danger")))
}

// baro_coop_talk_html : 「댓글」 탭
func baro_coop_talk_html(
	db *sql.DB, config tool.Config, room tool.Baro_coop_room_row, is_in bool, can_manage bool,
) string {
	base := baro_coop_url(room.Link)

	html := ""

	can_talk := tool.Baro_coop_can(db, room, config.IP, "comment")

	if is_in && room.Baro_coop_room_live() && can_talk {
		form := tool.Baro_hidden("do", "talk")
		form += tool.Baro_textarea("data", tool.Baro_t("댓글"), "", 3, tool.Baro_t("하고 싶은 말"), "")
		form += tool.Baro_submit(tool.Baro_t("댓글 달기"), "primary")

		note := ""
		if list := tool.Baro_coop_ban_parse(room.Banned); len(list) > 0 {
			words := []string{}
			for _, one := range list {
				words = append(words, "'"+one.Word+"'")
			}

			note = tool.Baro_note(tool.Baro_t("이 방의 금칙어 : ") + strings.Join(words, ", "))
		}

		html += tool.Baro_card(tool.Baro_t("댓글 달기"), tool.Baro_form(base+"/talk", form)+note)
	} else if !is_in {
		html += tool.Baro_note(tool.Baro_t("방에 들어와야 댓글을 달 수 있습니다."))
	} else if !can_talk {
		html += tool.Baro_note(tool.Baro_t("이 방에서 댓글을 달 수 있는 자리가 아닙니다."))
	}

	list := tool.Baro_coop_log_list(db, room.ID, "comment", 200)

	inner := `<div class="bw_coop_talk">`
	for _, one := range list {
		head := tool.Baro_user_tag(db, one.User, config.IP) + ` · ` + tool.Baro_time_cell(one.Date)

		if tool.Baro_coop_can(db, room, config.IP, "talk_hide") {
			head += ` ` + tool.Baro_form(base+"/talk",
				tool.Baro_hidden("do", "talk_hide")+tool.Baro_hidden("id", one.ID)+
					tool.Baro_submit(tool.Baro_t("지우기"), ""))
		}

		inner += `<div class="bw_coop_talk_one"><div class="bw_coop_talk_head">` + head + `</div>` +
			`<div class="bw_coop_ban">` + tool.HTML_escape(one.Data) + `</div></div>`
	}
	inner += `</div>`

	if len(list) == 0 {
		inner = tool.Baro_empty(tool.Baro_t("아직 댓글이 없습니다."))
	}

	return html + tool.Baro_card(tool.Baro_t("댓글"), inner)
}

// baro_coop_detail_html : 「상세」 탭
func baro_coop_detail_html(db *sql.DB, config tool.Config, room tool.Baro_coop_room_row) string {
	pairs := [][]string{
		{tool.Baro_t("방 이름"), tool.HTML_escape(room.Name)},
		{tool.Baro_t("공개 상태"), tool.Baro_t_esc(tool.Baro_coop_open_kind_name(room.OpenKind))},
		{tool.Baro_t("방장"), tool.Baro_user_tag(db, room.Owner, config.IP)},
		{tool.Baro_t("정원"), tool.HTML_escape(tool.Baro_itoa(room.MaxUser)) + tool.Baro_t_esc("명")},
		{tool.Baro_t("연 때"), tool.Baro_time_cell(room.Date)},
		{tool.Baro_t("방 주소"), tool.Baro_code(baro_coop_url(room.Link))},
	}

	// 열쇠는 들어와 있는 사람과 운영진에게만 보여 준다.
	if room.OpenKind == tool.Baro_coop_code {
		member, found := tool.Baro_coop_member_get(db, room.ID, config.IP)
		if (found && member.Status == "in") || tool.Baro_can(db, config.IP, "system") {
			pairs = append(pairs, []string{tool.Baro_t("12자리 열쇠"), tool.Baro_code(room.Code)})
		}
	}

	html := tool.Baro_card(tool.Baro_t("방 정보"), tool.Baro_dl(pairs))

	// 방에 있었던 사람
	rows := [][]string{}
	for _, one := range tool.Baro_coop_member_list(db, room.ID, "out") {
		rows = append(rows, []string{
			tool.Baro_user_tag(db, one.User, config.IP),
			tool.Baro_t_esc(tool.Baro_coop_role_name(one.Role)),
			tool.Baro_time_cell(one.Date),
			tool.Baro_time_cell(one.OutDate),
		})
	}

	html += tool.Baro_card_ex(
		tool.Baro_t("방에 있었던 사람"),
		tool.Baro_t("나간 사람도 기록에 남습니다."),
		tool.Baro_table(
			[]string{tool.Baro_t("사람"), tool.Baro_t("자리"), tool.Baro_t("들어온 때"), tool.Baro_t("나간 때")},
			rows, tool.Baro_t("나간 사람이 없습니다."),
		),
		"",
	)

	// 방에서 일어난 일
	log_rows := [][]string{}
	for _, one := range tool.Baro_coop_log_list(db, room.ID, "", 100) {
		if one.Kind == "comment" {
			continue
		}

		who := ""
		if one.User != "" {
			who = tool.Baro_user_tag(db, one.User, config.IP)
		}

		what := tool.HTML_escape(one.Data)
		if one.Kind == "edit" {
			what = tool.Baro_doc_link(one.Data)
		}

		log_rows = append(log_rows, []string{
			tool.Baro_time_cell(one.Date),
			tool.Baro_t_esc(baro_coop_log_kind_name(one.Kind)),
			who,
			what,
			tool.HTML_escape(one.Detail),
		})
	}

	html += tool.Baro_card(tool.Baro_t("방에서 일어난 일"), tool.Baro_table(
		[]string{tool.Baro_t("때"), tool.Baro_t("갈래"), tool.Baro_t("사람"), tool.Baro_t("내용"), tool.Baro_t("덧붙임")},
		log_rows, tool.Baro_t("아직 기록이 없습니다."),
	))

	return html
}

// baro_coop_log_kind_name : 기록 갈래 이름
func baro_coop_log_kind_name(kind string) string {
	switch kind {
	case "edit":
		return tool.Baro_t("편집")
	case "join":
		return tool.Baro_t("참가")
	case "leave":
		return tool.Baro_t("나감")
	case "notice":
		return tool.Baro_t("공지")
	case "invite":
		return tool.Baro_t("초대")
	case "doc_open":
		return tool.Baro_t("문서 염")
	case "doc_close":
		return tool.Baro_t("문서 닫음")
	case "system":
		return tool.Baro_t("방")
	case "comment":
		return tool.Baro_t("댓글")
	}

	return kind
}
