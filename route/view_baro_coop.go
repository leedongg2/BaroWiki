package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  편집방  /baro/cooperation
//
//    /baro/cooperation            열린 편집방 목록 · 열쇠로 들어가기 · 방 만들기
//    /baro/cooperation/<이름표>    방 (참가 · 댓글 · 문서 · 상세 · 설정)
//
//  공개 방은 목록에 오른다. 일부 공개 방은 링크나 12자리 열쇠로만 들어온다.
//  비공개 방은 이미 이름이 올라 있는 사람만 본다.
// ============================================================

// baro_coop_menu : 화면 아래 차림표
func baro_coop_menu() [][]any {
	return [][]any{{"/baro", "바로위키"}, {"/baro/cooperation", tool.Baro_t("편집방")}}
}

// baro_coop_url : 방 주소
func baro_coop_url(link string) string {
	return "/baro/cooperation/" + tool.Url_parser(link)
}

// baro_coop_tabs : 탭
func baro_coop_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/cooperation", tool.Baro_t("열린 편집방")},
		{"/baro/cooperation/mine", tool.Baro_t("내 편집방")},
		{"/baro/cooperation/new", tool.Baro_t("편집방 만들기")},
	}, active)
}

// baro_coop_alert : 주소에 실려 온 안내 문구
func baro_coop_alert(values url.Values) string {
	if values == nil {
		return ""
	}

	text := values.Get("bw_alert")
	if text == "" {
		return ""
	}

	kind := values.Get("bw_kind")
	if kind == "" {
		kind = "ok"
	}

	return tool.Baro_alert(kind, text)
}

// View_baro_coop_list : 편집방 목록 (/baro/cooperation)
func View_baro_coop_list(config tool.Config, kind string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if kind != "mine" {
		kind = "public"
	}

	// 12자리 열쇠로 들어가기
	if values != nil && values.Get("do") == "code" {
		code := tool.Baro_coop_code_norm(values.Get("code"))

		if len(code) != 12 {
			return baro_coop_back("/baro/cooperation", tool.Baro_t("12자리 숫자 열쇠를 적어 주세요."), "warn")
		}

		room, found := tool.Baro_coop_room_by_code(db, code)
		if !found {
			return baro_coop_back("/baro/cooperation", tool.Baro_t("그 열쇠를 가진 편집방이 없습니다."), "danger")
		}

		return tool.Get_redirect(baro_coop_url(room.Link))
	}

	page := 1
	if values != nil {
		page = tool.Str_to_int(values.Get("page"))
	}
	if page < 1 {
		page = 1
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("편집방")}})
	body += tool.Baro_hero(
		tool.Baro_t("편집방"),
		tool.Baro_t("여럿이 한자리에 모여 문서를 함께 손봅니다."),
		"",
	)

	tab := "/baro/cooperation"
	if kind == "mine" {
		tab = "/baro/cooperation/mine"
	}
	body += baro_coop_tabs(tab)
	body += baro_coop_alert(values)

	// --- 열쇠로 들어가기 ---
	if kind != "mine" {
		form := tool.Baro_hidden("do", "code")
		form += tool.Baro_input("code", tool.Baro_t("12자리 열쇠"), "", "000000000000",
			tool.Baro_t("일부 공개 편집방은 목록에 오르지 않습니다. 받은 열쇠를 적으면 들어갑니다."))
		form += tool.Baro_submit(tool.Baro_t("들어가기"), "")

		body += tool.Baro_card(tool.Baro_t("열쇠로 들어가기"), tool.Baro_form("/baro/cooperation", form))
	}

	// --- 목록 ---
	user := config.IP
	list := tool.Baro_coop_room_list(db, kind, user, (page-1)*tool.Baro_page_size, tool.Baro_page_size)

	cards := []string{}
	for _, room := range list {
		cards = append(cards, baro_coop_card(db, config, room))
	}

	if len(cards) == 0 {
		empty := tool.Baro_t("지금 열려 있는 편집방이 없습니다.")
		if kind == "mine" {
			empty = tool.Baro_t("들어가 있는 편집방이 없습니다.")
		}

		body += tool.Baro_card("", tool.Baro_empty(empty))
	} else {
		body += tool.Baro_grid(cards, 2)
	}

	body += tool.Baro_paging(page, tool.Baro_coop_room_count(db, kind, user), tool.Baro_page_size,
		tab+"?page={}")

	if tool.Baro_coop_can_open(db, config.IP) {
		body += tool.Baro_card("", tool.Baro_button("/baro/cooperation/new", tool.Baro_t("편집방 만들기"), "primary"))
	} else if tool.Baro_is_user(config.IP) {
		body += tool.Baro_note(tool.Baro_t("지금은 편집방을 열 수 있는 권한이 없습니다. 운영진에게 물어봐 주세요."))
	}

	return tool.Baro_page(db, config, tool.Baro_t("편집방"), body, baro_coop_menu())
}

// baro_coop_back : 안내 문구를 들고 되돌아가기
func baro_coop_back(where string, text string, kind string) string {
	return tool.Get_redirect(where + "?bw_alert=" + tool.Url_parser(text) + "&bw_kind=" + tool.Url_parser(kind))
}

// baro_coop_card : 목록에 놓는 방 한 칸
//
//	요청받은 모양 그대로 —
//	  (방 이름) / (공개 상태) / 문서: … / 인원: n/n / 참여자 / 관전: n명
func baro_coop_card(db *sql.DB, config tool.Config, room tool.Baro_coop_room_row) string {
	edit_count, watch_count := tool.Baro_coop_count(db, room.ID)

	title := `<a href="` + baro_coop_url(room.Link) + `">` + tool.HTML_escape(room.Name) + `</a>`
	if tool.Baro_coop_owner_away(db, room) {
		title += ` <span class="bw_coop_away">(` + tool.Baro_t_esc("방장 부재") + `)</span>`
	}

	inner := `<p class="bw_coop_kind">` + tool.Baro_badge(tool.Baro_coop_open_kind_name(room.OpenKind), "info")
	switch room.Status {
	case tool.Baro_coop_closing:
		inner += ` ` + tool.Baro_badge(tool.Baro_t("곧 닫음"), "warn")
	case tool.Baro_coop_closed:
		inner += ` ` + tool.Baro_badge(tool.Baro_t("닫힘"), "muted")
	}
	inner += `</p>`

	// 문서
	docs := tool.Baro_coop_doc_text(tool.Baro_coop_doc_list(db, room.ID))
	doc_text := tool.Baro_t("아직 손댄 문서가 없습니다.")
	if len(docs) > 0 {
		shown := docs
		more := ""

		if len(shown) > 6 {
			more = tool.Baro_t(" 외 ") + tool.Baro_itoa(len(shown)-6) + tool.Baro_t("개")
			shown = shown[:6]
		}

		doc_text = tool.HTML_escape(strings.Join(shown, ", ")) + tool.HTML_escape(more)
	} else {
		doc_text = tool.HTML_escape(doc_text)
	}

	inner += `<p><b>` + tool.Baro_t_esc("문서") + `:</b> ` + doc_text + `</p>`
	inner += `<p><b>` + tool.Baro_t_esc("인원") + `:</b> ` +
		tool.HTML_escape(tool.Baro_itoa(edit_count)+"/"+tool.Baro_itoa(room.MaxUser)) + `</p>`

	inner += baro_coop_member_html(db, config, room)
	inner += `<p><b>` + tool.Baro_t_esc("관전") + `:</b> ` +
		tool.HTML_escape(tool.Baro_itoa(watch_count)) + tool.Baro_t_esc("명") + `</p>`

	return tool.Baro_card_ex(room.Name, "", `<p class="bw_coop_title">`+title+`</p>`+inner, "") +
		baro_coop_css()
}

// baro_coop_member_html : 참여자 목록 (넷을 넘으면 접는다)
func baro_coop_member_html(db *sql.DB, config tool.Config, room tool.Baro_coop_room_row) string {
	list := []tool.Baro_coop_member_row{}
	for _, one := range tool.Baro_coop_member_list(db, room.ID, "in") {
		if one.Baro_coop_member_editing() {
			list = append(list, one)
		}
	}

	if len(list) == 0 {
		return `<p><b>` + tool.Baro_t_esc("참여자") + `:</b> ` + tool.Baro_t_esc("아직 없습니다.") + `</p>`
	}

	line := func(one tool.Baro_coop_member_row) string {
		text := `<li>` + tool.Baro_user_tag(db, one.User, config.IP)

		if one.Role == tool.Baro_coop_owner || one.Role == tool.Baro_coop_sub {
			text += `: ` + tool.Baro_t_esc(tool.Baro_coop_role_name(one.Role))
		}

		if one.Block {
			text += ` ` + tool.Baro_badge(tool.Baro_t("편집 제한"), "danger")
		}

		return text + `</li>`
	}

	html := `<div><b>` + tool.Baro_t_esc("참여자") + `</b><ul class="bw_coop_who">`

	head := list
	rest := []tool.Baro_coop_member_row{}
	if len(head) > tool.Baro_coop_member_show {
		rest = head[tool.Baro_coop_member_show:]
		head = head[:tool.Baro_coop_member_show]
	}

	for _, one := range head {
		html += line(one)
	}

	html += `</ul>`

	if len(rest) > 0 {
		html += `<details class="bw_coop_more"><summary>[ ` + tool.Baro_t_esc("더보기") + ` ] ` +
			tool.HTML_escape(tool.Baro_itoa(len(rest))) + tool.Baro_t_esc("명") + `</summary><ul class="bw_coop_who">`

		for _, one := range rest {
			html += line(one)
		}

		html += `</ul></details>`
	}

	return html + `</div>`
}

// baro_coop_css : 편집방 화면 모양
func baro_coop_css() string {
	return `<style id="bw_coop_css">
.bw_coop_title { margin: 0 0 4px; font-weight: 600; }
.bw_coop_kind { margin: 0 0 8px; }
.bw_coop_away { color: var(--namu-length-minus-color, #f00); font-size: 12px; }
.bw_coop_who { margin: 4px 0 8px 1.1rem; padding: 0; }
.bw_coop_who li { margin: 2px 0; }
.bw_coop_more > summary { cursor: pointer; font-size: 13px; }
.bw_coop_talk { display: grid; gap: 10px; }
.bw_coop_talk_one { border: 1px solid var(--namu-card-border-color, #ddd); border-radius: 6px; padding: 8px 10px; }
.bw_coop_talk_head { font-size: 12px; opacity: 0.8; margin-bottom: 4px; }
.bw_coop_ban { white-space: pre-wrap; }
</style>`
}

// baro_coop_recent_html : 최근 토론 화면에 끼워 넣는 「열린 편집방」 목록
//
//	토론과 같은 자리에서 보게 하되, 방 칸 모양은 편집방 목록의 것을 그대로 쓴다.
func baro_coop_recent_html(db *sql.DB, config tool.Config) string {
	list := tool.Baro_coop_room_list(db, "public", config.IP, 0, 20)

	if len(list) == 0 {
		return tool.Baro_empty(tool.Baro_t("지금 열려 있는 편집방이 없습니다.")) +
			tool.Baro_button("/baro/cooperation", tool.Baro_t("편집방으로 가기"), "primary")
	}

	cards := []string{}
	for _, room := range list {
		cards = append(cards, baro_coop_card(db, config, room))
	}

	return tool.Baro_grid(cards, 2) +
		tool.Baro_button("/baro/cooperation", tool.Baro_t("편집방 모두 보기"), "primary")
}
