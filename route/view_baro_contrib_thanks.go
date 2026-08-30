package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  7-7 감사 기능 — /baro/thanks
//
//  문서 전체에 대한 좋아요가 아니라, 특정 기여(문서 + 판)에 감사를 보낸다.
//  감사 수는 순위의 주요 지표로 쓰지 않는다. 인기투표가 되면 의미가 사라진다.
// ============================================================

// baro_contrib_thanks_row : 감사 한 줄
type baro_contrib_thanks_row struct {
	ID       string
	FromUser string
	ToUser   string
	Title    string
	Rev      string
	Reason   string
	IsPublic string
	Date     string
}

// baro_contrib_thanks_cols : 감사 조회 컬럼
const baro_contrib_thanks_cols = "id, from_user, to_user, title, rev, reason, is_public, date"

// baro_contrib_thanks_scan : 조회 결과를 구조체 목록으로
func baro_contrib_thanks_scan(rows *sql.Rows) []baro_contrib_thanks_row {
	out := []baro_contrib_thanks_row{}

	for rows.Next() {
		row := baro_contrib_thanks_row{}
		if err := rows.Scan(
			&row.ID, &row.FromUser, &row.ToUser, &row.Title,
			&row.Rev, &row.Reason, &row.IsPublic, &row.Date,
		); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_contrib_thanks_one : 감사 하나 읽기
func baro_contrib_thanks_one(db *sql.DB, id string) (baro_contrib_thanks_row, bool) {
	row := baro_contrib_thanks_row{}

	found := tool.QueryRow_DB(
		db,
		"select "+baro_contrib_thanks_cols+" from baro_thanks where id = ?",
		[]any{
			&row.ID, &row.FromUser, &row.ToUser, &row.Title,
			&row.Rev, &row.Reason, &row.IsPublic, &row.Date,
		},
		id,
	)

	return row, found
}

// baro_contrib_thanks_editor : 특정 문서·판을 누가 편집했는지 찾는다
func baro_contrib_thanks_editor(db *sql.DB, title string, rev string) (string, bool) {
	if title == "" || rev == "" {
		return "", false
	}

	ip := ""
	found := tool.QueryRow_DB(
		db,
		"select ip from history where title = ? and id = ?",
		[]any{&ip},
		title, rev,
	)

	return ip, found
}

// baro_contrib_thanks_notice : 인기투표화 방지 안내
func baro_contrib_thanks_notice() string {
	return tool.Baro_alert("info",
		tool.Baro_t("<b>감사는 인기투표가 아닙니다.</b> ")+
			tool.Baro_t("감사 수는 순위의 주요 지표로 쓰지 않으며, 이 화면의 목록도 받은 수가 아니라 <b>최신순</b>으로만 보여 줍니다. ")+
			tool.Baro_t("많이 받은 사람을 줄 세우는 화면은 만들지 않습니다. ")+
			tool.Baro_t("감사는 '이 수정이 도움이 됐다' 고 한 사람에게 조용히 전하는 기능입니다."),
	)
}

// baro_contrib_thanks_table : 감사 목록 표
func baro_contrib_thanks_table(db *sql.DB, list []baro_contrib_thanks_row, my_ip string, mode string) string {
	rows := [][]string{}

	for _, row := range list {
		reason := row.Reason
		if reason == "" {
			reason = tool.Baro_t("(이유 없음)")
		}

		// 비공개 감사는 당사자에게만 이유를 보여 준다
		if row.IsPublic != "on" && my_ip != row.FromUser && my_ip != row.ToUser {
			reason = tool.Baro_t("(비공개)")
		}

		who := tool.Baro_user_link(db, row.FromUser, my_ip)
		if mode == "sent" {
			who = tool.Baro_user_link(db, row.ToUser, my_ip)
		}

		public_badge := tool.Baro_badge(tool.Baro_t("비공개"), "muted")
		if row.IsPublic == "on" {
			public_badge = tool.Baro_badge("공개", "ok")
		}

		action := `<span class="bw_muted">-</span>`
		if my_ip != "" && my_ip == row.FromUser {
			action = tool.Baro_button(baro_contrib_query("/baro/thanks/cancel", [][]string{{"id", row.ID}}), "취소", "danger")
		}

		rows = append(rows, []string{
			tool.Baro_time_cell(row.Date),
			who,
			tool.Baro_doc_link(row.Title) + ` <span class="bw_muted">r` + baro_contrib_esc(row.Rev) + `</span>`,
			baro_contrib_esc(tool.Baro_trunc(tool.Baro_one_line(reason), 80)),
			public_badge,
			action,
		})
	}

	header := tool.Baro_t("보낸 사람")
	if mode == "sent" {
		header = tool.Baro_t("받은 사람")
	}

	if len(rows) == 0 {
		return tool.Baro_empty(tool.Baro_t("표시할 감사가 없습니다. 도움이 된 수정을 발견하면 감사를 보내 보세요."))
	}

	return tool.Baro_table([]string{"시각", header, tool.Baro_t("기여"), tool.Baro_t("이유"), "공개", ""}, rows, tool.Baro_t("표시할 감사가 없습니다."))
}

// ------------------------------------------------------------
//  화면 : /baro/thanks
// ------------------------------------------------------------

// View_baro_contrib_thanks : 받은 감사 / 보낸 감사 목록
func View_baro_contrib_thanks(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page := baro_contrib_page_num(baro_contrib_get(query, "page"))
	tab := baro_contrib_get(query, "tab")
	if tab != "sent" && tab != "public" {
		tab = "received"
	}

	target := baro_contrib_get(query, "user")

	// 로그인한 사람은 기본으로 자기 화면을 본다.
	// 로그인하지 않았다면 사람을 정하지 않은 것으로 보고 공개된 감사 전체를 보여 준다.
	// (예전에는 여기에 접속한 IP 주소를 사람 이름 자리에 넣어, 늘 빈 목록이 나왔다)
	if target == "" && tool.Baro_is_user(config.IP) {
		target = config.IP
	}

	if !tool.Baro_is_user(target) {
		target = ""
	}

	// 남의 화면은 공개된 감사만 보여 준다
	is_me := tool.Baro_is_user(config.IP) && target == config.IP
	if !is_me && tab == "sent" {
		tab = "public"
	}

	where := ""
	args := []any{}

	switch {
	case target == "":
		// 사람을 정하지 않았을 때 : 공개된 감사 전체를 최신순으로 보여 준다
		where = " where is_public = 'on'"
	case tab == "sent":
		where = " where from_user = ?"
		args = append(args, target)
	case tab == "public":
		where = " where to_user = ? and is_public = 'on'"
		args = append(args, target)
	default:
		where = " where to_user = ?"
		args = append(args, target)

		if !is_me {
			where += " and is_public = 'on'"
		}
	}

	list_args := append(append([]any{}, args...), (page-1)*baro_contrib_page_size, baro_contrib_page_size)

	rows := tool.Query_DB(
		db,
		"select "+baro_contrib_thanks_cols+" from baro_thanks"+where+" order by id + 0 desc limit ?, ?",
		list_args...,
	)
	defer rows.Close()

	list := baro_contrib_thanks_scan(rows)

	received := 0
	sent := 0
	if target != "" {
		received = baro_contrib_count(db, "select count(*) from baro_thanks where to_user = ? and from_user <> to_user", target)
		sent = baro_contrib_count(db, "select count(*) from baro_thanks where from_user = ?", target)
	}

	total_count := baro_contrib_count(db, "select count(*) from baro_thanks")
	public_count := baro_contrib_count(db, "select count(*) from baro_thanks where is_public = 'on'")

	stats := [][]string{
		{tool.Baro_t("공개된 감사"), tool.Baro_comma(public_count), "건"},
		{tool.Baro_t("전체 감사"), tool.Baro_comma(total_count), "건"},
	}
	if target != "" {
		stats = [][]string{
			{tool.Baro_t("받은 감사"), tool.Baro_comma(received), "건"},
			{tool.Baro_t("보낸 감사"), tool.Baro_comma(sent), "건"},
			{tool.Baro_t("전체 감사"), tool.Baro_comma(total_count), "건"},
		}
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("감사")}})
	body += tool.Baro_hero(
		tool.Baro_t("감사"),
		tool.Baro_t("문서가 아니라 특정 기여에 감사를 보냅니다."),
		tool.Baro_stat_row(stats),
	)
	body += baro_contrib_tabs("/baro/thanks")
	body += baro_contrib_thanks_notice()

	if !tool.Baro_is_user(config.IP) {
		body += tool.Baro_alert("warn", tool.Baro_t("감사를 보내려면 로그인해야 합니다. 목록은 공개된 감사만 보입니다."))
	}

	// --- 사람을 정하지 않았을 때 : 공개된 감사 전체 (최신순) ---
	if target == "" {
		list_html := baro_contrib_thanks_table(db, list, config.IP, "received")
		list_html += tool.Baro_paging(page, len(list), baro_contrib_page_size,
			baro_contrib_query("/baro/thanks", [][]string{{"page", "{}"}}))

		body += tool.Baro_card_ex(
			tool.Baro_t("공개된 감사"),
			tool.Baro_t("누가 많이 받았는지가 아니라, 최근에 어떤 수정이 도움이 됐는지를 최신순으로 보여 줍니다."),
			list_html,
			`<form class="bw_form" method="get" action="/baro/thanks">`+
				tool.Baro_input("user", tool.Baro_t("사람을 골라 보기"), "", tool.Baro_t("사용자 이름"), tool.Baro_t("그 사람이 공개한 감사만 보입니다."))+
				tool.Baro_submit(tool.Baro_t("이 사람의 감사 보기"), "")+
				`</form>`,
		)

		body += tool.Baro_note(tool.Baro_t("로그인하면 자신이 받은 감사와 보낸 감사를 함께 볼 수 있습니다."))

		return tool.Baro_page(db, config, tool.Baro_t("감사"), body, baro_contrib_menu())
	}

	// --- 탭 ---
	tab_items := [][]string{
		{baro_contrib_query("/baro/thanks", [][]string{{"user", target}, {"tab", "received"}}), tool.Baro_t("받은 감사")},
	}
	if is_me {
		tab_items = append(tab_items, []string{
			baro_contrib_query("/baro/thanks", [][]string{{"user", target}, {"tab", "sent"}}), tool.Baro_t("보낸 감사"),
		})
	}

	body += tool.Baro_tabs(tab_items, baro_contrib_query("/baro/thanks", [][]string{{"user", target}, {"tab", tab}}))

	mode := "received"
	if tab == "sent" {
		mode = "sent"
	}

	list_html := baro_contrib_thanks_table(db, list, config.IP, mode)
	paging_url := baro_contrib_query("/baro/thanks", [][]string{{"user", target}, {"tab", tab}, {"page", "{}"}})
	list_html += tool.Baro_paging(page, len(list), baro_contrib_page_size, paging_url)

	title_text := tool.Namespace_display(target) + tool.Baro_t(" 님이 받은 감사")
	if tab == "sent" {
		title_text = tool.Namespace_display(target) + tool.Baro_t(" 님이 보낸 감사")
	}

	body += tool.Baro_card_ex(
		title_text,
		tool.Baro_t("최신순으로만 보여 줍니다. 많이 받은 순으로 줄 세우지 않습니다."),
		list_html,
		tool.Baro_button("/baro/thanks/send", "감사 보내기", "primary")+" "+
			tool.Baro_button("/baro/contrib/user/"+tool.Url_parser(target), "이 사람의 기여 보기", ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("감사"), body, baro_contrib_menu())
}

// ------------------------------------------------------------
//  화면 : /baro/thanks/send
// ------------------------------------------------------------

// View_baro_contrib_thanks_send : 특정 기여에 감사 보내기
func View_baro_contrib_thanks_send(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/thanks/send"); need {
		return wall
	}

	doc := baro_contrib_get(query, "doc")
	rev := baro_contrib_get(query, "rev")
	message := ""
	message_kind := ""

	// --- 처리 ---
	if values != nil {
		doc = tool.Namespace_normalize(strings.TrimSpace(values.Get("doc")))
		rev = strings.TrimSpace(values.Get("rev"))
		reason := strings.TrimSpace(values.Get("reason"))
		is_public := ""
		if values.Get("is_public") != "" {
			is_public = "on"
		}

		switch {
		case tool.Baro_read_only(db):
			message_kind, message = "danger", tool.Baro_t("지금은 읽기 전용 상태라 감사를 보낼 수 없습니다. ")+baro_contrib_esc(tool.Baro_read_only_reason(db))

		case doc == "" || rev == "":
			message_kind, message = "warn", tool.Baro_t("문서 이름과 판 번호를 모두 적어 주세요.")

		case !tool.Baro_rate_ok(db, "thanks_send", config.IP, 30, 3600):
			message_kind, message = "warn", tool.Baro_t("한 시간에 보낼 수 있는 감사 수를 넘었습니다. 잠시 뒤에 다시 시도해 주세요.")

		default:
			editor, found := baro_contrib_thanks_editor(db, doc, rev)

			switch {
			case !found:
				message_kind, message = "danger", tool.Baro_t("그 문서의 해당 판을 찾을 수 없습니다. 문서 이름과 판 번호를 확인해 주세요.")

			case !tool.Baro_is_user(editor):
				message_kind, message = "warn", tool.Baro_t("그 판은 로그인하지 않은 편집이라 감사를 보낼 수 없습니다.")

			case editor == config.IP:
				// 셀프 추천 제한 : 자기 자신에게는 보낼 수 없다
				message_kind, message = "warn", tool.Baro_t("자기 자신의 기여에는 감사를 보낼 수 없습니다.")

			case baro_contrib_count(db, "select count(*) from baro_thanks where from_user = ? and title = ? and rev = ?", config.IP, doc, rev) > 0:
				message_kind, message = "warn", tool.Baro_t("이미 이 수정에 감사를 보냈습니다. 한 수정에는 한 번만 보낼 수 있습니다.")

			default:
				tool.Exec_DB(
					db,
					"insert into baro_thanks (id, from_user, to_user, title, rev, reason, is_public, date) values (?, ?, ?, ?, ?, ?, ?, ?)",
					tool.Baro_next_id(db, "baro_thanks"), config.IP, editor, doc, rev,
					tool.Baro_trunc(reason, 300), is_public, tool.Baro_now(),
				)

				tool.Baro_notify(
					db, editor, "thanks",
					tool.Namespace_display(doc)+tool.Baro_t(" 문서의 수정에 감사가 도착했습니다"),
					"/baro/thanks?tab=received",
					tool.Baro_trunc(reason, 200),
				)

				tool.Baro_impact_bump(db, editor, "thanks", 1)
				tool.Baro_metric_bump(db, "thanks_send", tool.Baro_today(), 1)

				return tool.Get_redirect("/baro/thanks?tab=sent&sent=1")
			}
		}
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/thanks", tool.Baro_t("감사")}, {"", "감사 보내기"}})
	body += tool.Baro_hero(
		"감사 보내기",
		tool.Baro_t("도움이 된 특정 수정에 감사를 전합니다."),
		"",
	)
	body += baro_contrib_tabs("/baro/thanks")
	body += baro_contrib_thanks_notice()

	if message != "" {
		body += tool.Baro_alert(message_kind, message)
	}

	// 문서를 정했다면 최근 판을 보여 준다
	rev_help := tool.Baro_t("역사 화면에 나오는 판 번호(r 뒤의 숫자)를 적어 주세요.")
	recent_html := ""

	if doc != "" {
		rows := tool.Query_DB(
			db,
			"select id, date, ip, send from history where title = ? order by id + 0 desc limit ?, ?",
			doc, 0, 10,
		)
		defer rows.Close()

		recent_rows := [][]string{}
		for rows.Next() {
			id, date, ip, send := "", "", "", ""
			if err := rows.Scan(&id, &date, &ip, &send); err != nil {
				continue
			}

			pick := `<span class="bw_muted">-</span>`
			if tool.Baro_is_user(ip) && ip != config.IP {
				pick = tool.Baro_button(
					baro_contrib_query("/baro/thanks/send", [][]string{{"doc", doc}, {"rev", id}}),
					tool.Baro_t("이 판 고르기"), "",
				)
			}

			recent_rows = append(recent_rows, []string{
				"r" + baro_contrib_esc(id),
				tool.Baro_time_cell(date),
				tool.Baro_user_link(db, ip, config.IP),
				baro_contrib_esc(tool.Baro_trunc(tool.Baro_one_line(send), 60)),
				pick,
			})
		}

		if len(recent_rows) == 0 {
			recent_html = tool.Baro_empty(tool.Baro_t("그 이름의 문서에 편집 기록이 없습니다. 문서 이름을 확인해 주세요."))
		} else {
			recent_html = tool.Baro_table([]string{"판", "시각", tool.Baro_t("편집자"), tool.Baro_t("요약"), ""}, recent_rows, tool.Baro_t("편집 기록이 없습니다."))
		}
	}

	form_body := tool.Baro_input("doc", tool.Baro_t("문서 이름"), tool.Namespace_display(doc), tool.Baro_t("문서 이름"), tool.Baro_t("감사를 보낼 기여가 있는 문서입니다."))
	form_body += tool.Baro_input("rev", tool.Baro_t("판 번호"), rev, tool.Baro_t("예: 12"), rev_help)
	form_body += tool.Baro_textarea("reason", tool.Baro_t("이유 (선택)"), "", 4, tool.Baro_t("어떤 점이 도움이 됐는지 적어 주세요."), tool.Baro_t("적지 않아도 보낼 수 있습니다."))
	form_body += tool.Baro_checkbox("is_public", tool.Baro_t("이 감사를 공개합니다"), "on", true, tool.Baro_t("끄면 보낸 사람과 받은 사람만 이유를 볼 수 있습니다."))
	form_body += tool.Baro_submit("감사 보내기", "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("보낼 기여 고르기"),
		tool.Baro_t("문서 전체가 아니라 '문서 + 판' 하나를 고릅니다."),
		tool.Baro_form("/baro/thanks/send", form_body),
		tool.Baro_note(tool.Baro_t("한 사람은 같은 수정에 한 번만 감사를 보낼 수 있습니다. 자기 자신에게는 보낼 수 없습니다.")),
	)

	if recent_html != "" {
		body += tool.Baro_card(tool.Namespace_display(doc)+tool.Baro_t(" 문서의 최근 판"), recent_html)
	} else {
		body += tool.Baro_card(tool.Baro_t("문서 찾기"),
			`<form class="bw_form" method="get" action="/baro/thanks/send">`+
				tool.Baro_input("doc", tool.Baro_t("문서 이름"), "", tool.Baro_t("문서 이름"), tool.Baro_t("문서를 정하면 최근 판 목록이 나옵니다."))+
				tool.Baro_submit(tool.Baro_t("최근 판 보기"), "")+
				`</form>`,
		)
	}

	return tool.Baro_page(db, config, "감사 보내기", body, baro_contrib_menu())
}

// ------------------------------------------------------------
//  화면 : /baro/thanks/cancel
// ------------------------------------------------------------

// View_baro_contrib_thanks_cancel : 보낸 감사 취소
func View_baro_contrib_thanks_cancel(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/thanks"); need {
		return wall
	}

	id := baro_contrib_get(query, "id")
	if values != nil {
		id = strings.TrimSpace(values.Get("id"))
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/thanks", tool.Baro_t("감사")}, {"", tool.Baro_t("감사 취소")}})

	if id == "" {
		body += tool.Baro_alert("warn", tool.Baro_t("취소할 감사를 정하지 않았습니다."))
		body += tool.Baro_button("/baro/thanks", tool.Baro_t("감사 목록으로"), "")

		return tool.Baro_page(db, config, tool.Baro_t("감사 취소"), body, baro_contrib_menu())
	}

	row, found := baro_contrib_thanks_one(db, id)

	if !found {
		body += tool.Baro_alert("info", tool.Baro_t("이미 취소됐거나 없는 감사입니다."))
		body += tool.Baro_button("/baro/thanks", tool.Baro_t("감사 목록으로"), "")

		return tool.Baro_page(db, config, tool.Baro_t("감사 취소"), body, baro_contrib_menu())
	}

	if row.FromUser != config.IP {
		return tool.Baro_deny(db, config, tool.Baro_t("자신이 보낸 감사만 취소할 수 있습니다."))
	}

	// --- 처리 ---
	if values != nil {
		if tool.Baro_read_only(db) {
			body += tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 취소할 수 없습니다. ")+baro_contrib_esc(tool.Baro_read_only_reason(db)))

			return tool.Baro_page(db, config, tool.Baro_t("감사 취소"), body, baro_contrib_menu())
		}

		tool.Exec_DB(db, "delete from baro_thanks where id = ? and from_user = ?", id, config.IP)

		if row.FromUser != row.ToUser {
			current := tool.Baro_impact_get(db, row.ToUser, "thanks")
			if current > 0 {
				tool.Baro_impact_set(db, row.ToUser, "thanks", current-1)
			}
		}

		tool.Baro_audit(db, config, "thanks_cancel", row.ToUser, row.Title+" r"+row.Rev, "", "보낸 감사 취소")

		return tool.Get_redirect("/baro/thanks?tab=sent&canceled=1")
	}

	body += tool.Baro_hero(tool.Baro_t("감사 취소"), tool.Baro_t("보낸 감사를 되돌립니다."), "")

	body += tool.Baro_card(tool.Baro_t("취소할 감사"), tool.Baro_dl([][]string{
		{tool.Baro_t("받는 사람"), tool.Baro_user_link(db, row.ToUser, config.IP)},
		{tool.Baro_t("기여"), tool.Baro_doc_link(row.Title) + ` <span class="bw_muted">r` + baro_contrib_esc(row.Rev) + `</span>`},
		{tool.Baro_t("이유"), baro_contrib_esc(row.Reason)},
		{tool.Baro_t("보낸 때"), baro_contrib_esc(row.Date)},
	}))

	body += tool.Baro_danger_zone(tool.Baro_t("정말 취소할까요?"),
		tool.Baro_note(tool.Baro_t("취소하면 기록이 지워지고 받는 사람의 감사 수에서도 빠집니다. 되돌릴 수 없습니다."))+
			tool.Baro_form("/baro/thanks/cancel",
				tool.Baro_hidden("id", row.ID)+tool.Baro_submit(tool.Baro_t("감사 취소"), "danger"),
			)+
			" "+tool.Baro_button("/baro/thanks?tab=sent", tool.Baro_t("그냥 두기"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("감사 취소"), body, baro_contrib_menu())
}
