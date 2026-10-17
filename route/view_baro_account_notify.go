package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  2-6 알림
// ============================================================

// baro_account_notify_filter_tabs : 종류별 필터 탭
func baro_account_notify_filter_tabs(active string) string {
	items := [][]string{
		{"/baro/notify", tool.Baro_t("전체")},
		{"/baro/notify?read=no", tool.Baro_t("안 읽음")},
	}

	for _, row := range tool.Baro_notify_kind_list() {
		items = append(items, []string{"/baro/notify?kind=" + row[0], row[1]})
	}

	return tool.Baro_tabs(items, active)
}

// View_baro_account_notify : 알림 목록 (/baro/notify)
func View_baro_account_notify(config tool.Config, page string, kind string, read string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/notify"); need {
		return wall
	}

	user := config.IP
	kind = strings.TrimSpace(kind)
	read = strings.TrimSpace(read)

	// --- 쓰기 처리 ---
	if values != nil {
		if tool.Baro_read_only(db) {
			return tool.Baro_page(db, config, tool.Baro_t("알림"),
				tool.Baro_alert("warn", tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.HTML_escape(tool.Baro_read_only_reason(db))),
				baro_account_menu())
		}

		id := strings.TrimSpace(values.Get("id"))

		switch values.Get("action") {
		case "read":
			if id != "" {
				tool.Exec_DB(db, "update baro_notify set readme = 'on' where id = ? and user = ?", id, user)
			}
		case "unread":
			if id != "" {
				tool.Exec_DB(db, "update baro_notify set readme = '' where id = ? and user = ?", id, user)
			}
		case "read_all":
			tool.Exec_DB(db, "update baro_notify set readme = 'on' where user = ? and readme = ''", user)
		case "delete":
			if id != "" {
				tool.Exec_DB(db, "delete from baro_notify where id = ? and user = ?", id, user)
			}
		case "delete_read":
			tool.Exec_DB(db, "delete from baro_notify where user = ? and readme = 'on'", user)
		case "delete_pick":
			// 골라 지우기 : 고른 것만 지운다
			//
			//	남의 알림을 지울 수 없도록 언제나 user 까지 함께 맞춰 본다.
			for _, one := range values["pick"] {
				one = strings.TrimSpace(one)
				if one == "" {
					continue
				}

				tool.Exec_DB(db, "delete from baro_notify where id = ? and user = ?", one, user)
			}
		}

		back := "/baro/notify"
		if kind != "" {
			back += "?kind=" + tool.Url_parser(kind)
		} else if read == "no" {
			back += "?read=no"
		}

		return tool.Get_redirect(back)
	}

	page_int := baro_account_page_int(page)
	max_count := tool.Baro_page_size

	query := "select id, kind, title, url, data, readme, date from baro_notify where user = ?"
	args := []any{user}

	if kind != "" {
		query += " and kind = ?"
		args = append(args, kind)
	}
	if read == "no" {
		query += " and readme = ''"
	}
	if read == "yes" {
		query += " and readme = 'on'"
	}

	query += " order by id + 0 desc limit ?, ?"
	args = append(args, (page_int-1)*max_count, max_count)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	list_rows := [][]string{}
	for rows.Next() {
		id, row_kind, title, link, data, readme, date := "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &row_kind, &title, &link, &data, &readme, &date); err != nil {
			continue
		}

		state := tool.Baro_badge(tool.Baro_t("안 읽음"), "warn")
		toggle_action := "read"
		toggle_label := tool.Baro_t("읽음")
		if readme == "on" {
			state = tool.Baro_badge(tool.Baro_t("읽음"), "muted")
			toggle_action = "unread"
			toggle_label = tool.Baro_t("안 읽음으로")
		}

		title_html := tool.HTML_escape(tool.Baro_trunc(title, 90))
		if strings.HasPrefix(link, "/") {
			title_html = `<a href="` + tool.HTML_escape(link) + `">` + title_html + `</a>`
		}
		if strings.TrimSpace(data) != "" {
			title_html += `<div class="bw_note">` + tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(data), 90)) + `</div>`
		}

		manage := tool.Baro_form("/baro/notify",
			tool.Baro_hidden("action", toggle_action)+
				tool.Baro_hidden("id", id)+
				tool.Baro_submit(toggle_label, ""))
		manage += tool.Baro_form("/baro/notify",
			tool.Baro_hidden("action", "delete")+
				tool.Baro_hidden("id", id)+
				tool.Baro_submit("삭제", "danger"))

		// 고르기 칸은 아래쪽 「고른 알림 지우기」 폼에 딸린다.
		// 표 칸 안에 폼을 또 넣으면 겹쳐서 못 쓰므로 form 속성으로 이어 붙인다.
		pick := `<input type="checkbox" class="bw_notify_pick" name="pick" value="` +
			tool.HTML_escape(id) + `" form="` + baro_notify_pick_form_id + `">`

		list_rows = append(list_rows, []string{
			pick,
			state,
			tool.HTML_escape(baro_account_notify_kind_name(row_kind)),
			title_html,
			tool.Baro_time_cell(date),
			manage,
		})
	}

	unread := tool.Baro_notify_count(db, user)

	total := 0
	tool.QueryRow_DB(db, "select count(*) from baro_notify where user = ?", []any{&total}, user)

	daily_kinds := 0
	off_kinds := 0
	for _, row := range tool.Baro_notify_kind_list() {
		switch baro_account_notify_mode(db, user, row[0]) {
		case "daily":
			daily_kinds++
		case "off":
			off_kinds++
		}
	}

	body := tool.Baro_hero(tool.Baro_t("알림"), tool.Baro_t("필요한 것만 골라 받으세요."),
		tool.Baro_button("/baro/notify/set", tool.Baro_t("알림 설정"), "primary")+" "+
			tool.Baro_button("/baro/notify/digest", tool.Baro_t("일일 요약"), "")+" "+
			tool.Baro_button("/baro/account", tool.Baro_t("내 계정"), ""))

	body += baro_account_tabs("/baro/notify")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("안 읽음"), tool.Baro_comma(unread), "건"},
		{tool.Baro_t("전체"), tool.Baro_comma(total), "건"},
		{tool.Baro_t("일일 요약 종류"), tool.Baro_comma(daily_kinds), tool.Baro_t("종")},
		{tool.Baro_t("끈 종류"), tool.Baro_comma(off_kinds), tool.Baro_t("종")},
	})

	active := "/baro/notify"
	if kind != "" {
		active = "/baro/notify?kind=" + kind
	} else if read == "no" {
		active = "/baro/notify?read=no"
	}
	body += baro_account_notify_filter_tabs(active)

	bulk := tool.Baro_form("/baro/notify", tool.Baro_hidden("action", "read_all")+tool.Baro_submit(tool.Baro_t("전체 읽음 처리"), ""))
	bulk += tool.Baro_form("/baro/notify", tool.Baro_hidden("action", "delete_read")+tool.Baro_submit(tool.Baro_t("읽은 알림 모두 삭제"), "danger"))

	// 골라 지우기 : 표의 고르기 칸들이 form 속성으로 이 폼에 딸려 온다
	if len(list_rows) > 0 {
		bulk += `<form class="bw_form" id="` + baro_notify_pick_form_id + `" method="post" action="/baro/notify">` +
			tool.Baro_hidden("action", "delete_pick") +
			tool.Baro_submit(tool.Baro_t("고른 알림 지우기"), "danger") +
			`</form>`
	}

	body += tool.Baro_card(tool.Baro_t("일괄 처리"), bulk)

	if len(list_rows) == 0 {
		body += tool.Baro_empty(tool.Baro_t("표시할 알림이 없습니다. 조용한 것도 좋은 상태입니다."))
	} else {
		body += tool.Baro_table(
			[]string{baro_notify_pick_all_html(), "상태", "종류", tool.Baro_t("내용"), tool.Baro_t("받은 때"), "관리"},
			list_rows,
			tool.Baro_t("표시할 알림이 없습니다."),
		)
		body += baro_notify_pick_script()
	}

	paging_url := "/baro/notify?page={}"
	if kind != "" {
		paging_url = "/baro/notify?kind=" + tool.Url_parser(kind) + "&page={}"
	} else if read != "" {
		paging_url = "/baro/notify?read=" + tool.Url_parser(read) + "&page={}"
	}
	body += tool.Baro_paging(page_int, len(list_rows), max_count, paging_url)

	return tool.Baro_page(db, config, tool.Baro_t("알림"), body, baro_account_menu())
}

// View_baro_account_notify_set : 알림 설정 (/baro/notify/set)
func View_baro_account_notify_set(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/notify/set"); need {
		return wall
	}

	user := config.IP
	_ = page

	if values != nil {
		if tool.Baro_read_only(db) {
			return tool.Baro_page(db, config, tool.Baro_t("알림 설정"),
				tool.Baro_alert("warn", tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.HTML_escape(tool.Baro_read_only_reason(db))),
				baro_account_menu())
		}

		switch values.Get("action") {
		case "all_daily":
			for _, row := range tool.Baro_notify_kind_list() {
				baro_account_notify_mode_put(db, user, row[0], "daily")
			}
		case "all_on":
			for _, row := range tool.Baro_notify_kind_list() {
				baro_account_notify_mode_put(db, user, row[0], "on")
			}
		default:
			for _, row := range tool.Baro_notify_kind_list() {
				baro_account_notify_mode_put(db, user, row[0], values.Get("mode_"+row[0]))
			}
		}

		return tool.Get_redirect("/baro/notify/set")
	}

	form := ""
	for _, row := range tool.Baro_notify_kind_list() {
		form += tool.Baro_select(
			"mode_"+row[0], row[1],
			baro_account_notify_mode_list(),
			baro_account_notify_mode(db, user, row[0]),
			baro_account_notify_help(row[0]),
		)
	}
	form += tool.Baro_submit(tool.Baro_t("설정 저장"), "primary")

	body := tool.Baro_hero(tool.Baro_t("알림 설정"), tool.Baro_t("종류마다 즉시 / 일일 요약 / 끄기를 고를 수 있습니다."),
		tool.Baro_button("/baro/notify", tool.Baro_t("알림 목록"), "")+" "+
			tool.Baro_button("/baro/notify/digest", tool.Baro_t("일일 요약"), ""))

	body += baro_account_tabs("/baro/notify")

	body += tool.Baro_alert("info",
		tool.Baro_t("<b>알림 폭탄을 막기 위한 안내</b><br>")+
			tool.Baro_t("관심 문서가 많아지면 알림이 순식간에 불어납니다. 자주 오는 종류는 <b>일일 요약</b>으로 바꿔 두면 ")+
			tool.Baro_t("하루 한 번 묶어서 볼 수 있습니다. 꼭 필요한 종류만 즉시 받기로 두는 편을 권합니다."))

	body += tool.Baro_card(tool.Baro_t("종류별 설정"), tool.Baro_form("/baro/notify/set", form))

	quick := tool.Baro_form("/baro/notify/set", tool.Baro_hidden("action", "all_daily")+tool.Baro_submit(tool.Baro_t("전부 일일 요약으로"), ""))
	quick += tool.Baro_form("/baro/notify/set", tool.Baro_hidden("action", "all_on")+tool.Baro_submit(tool.Baro_t("전부 즉시 받기로"), ""))
	body += tool.Baro_card(tool.Baro_t("한 번에 바꾸기"), quick+tool.Baro_note(tool.Baro_t("끄기는 종류마다 직접 골라 주세요. 실수로 전부 끄는 것을 막기 위해서입니다.")))

	body += tool.Baro_card(tool.Baro_t("알아두기"),
		tool.Baro_list([]string{
			tool.Baro_t("끄기로 두면 그 종류의 알림은 저장되지 않습니다. 나중에 다시 볼 수 없습니다."),
			tool.Baro_t("일일 요약은 알림을 그대로 저장하되 일일 요약 화면에서 날짜별로 묶어 보여 줍니다."),
			tool.Baro_t("제재·이의제기 결과처럼 중요한 알림은 즉시 받기로 두는 편이 좋습니다."),
		}))

	return tool.Baro_page(db, config, tool.Baro_t("알림 설정"), body, baro_account_menu())
}

// baro_account_notify_help : 종류별 설명
func baro_account_notify_help(kind string) string {
	switch kind {
	case "watch":
		return tool.Baro_t("관심 문서가 많으면 가장 많이 쌓이는 종류입니다. 일일 요약을 권합니다.")
	case "case":
		return tool.Baro_t("신고·이의제기 결과입니다. 즉시 받기를 권합니다.")
	case "thanks":
		return tool.Baro_t("다른 기여자가 보낸 감사입니다.")
	case "revert":
		return tool.Baro_t("내 편집이 되돌려졌을 때입니다. 사유를 확인할 수 있습니다.")
	case "system":
		return tool.Baro_t("점검·정책 변경 같은 운영 안내입니다.")
	}

	return ""
}

// View_baro_account_notify_digest : 일일 요약 (/baro/notify/digest)
func View_baro_account_notify_digest(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/notify/digest"); need {
		return wall
	}

	user := config.IP
	_ = values

	daily_kinds := []string{}
	for _, row := range tool.Baro_notify_kind_list() {
		if baro_account_notify_mode(db, user, row[0]) == "daily" {
			daily_kinds = append(daily_kinds, row[0])
		}
	}

	body := tool.Baro_hero(tool.Baro_t("일일 요약"), tool.Baro_t("일일 요약으로 설정한 종류를 하루 단위로 묶어 보여줍니다."),
		tool.Baro_button("/baro/notify", tool.Baro_t("알림 목록"), "")+" "+
			tool.Baro_button("/baro/notify/set", tool.Baro_t("알림 설정"), "primary"))

	body += baro_account_tabs("/baro/notify")

	if len(daily_kinds) == 0 {
		body += tool.Baro_alert("info", tool.Baro_t("아직 일일 요약으로 설정한 종류가 없습니다. 알림이 많은 종류를 요약으로 바꿔 보세요."))
		body += tool.Baro_empty(tool.Baro_t("묶어서 보여줄 알림이 없습니다."))
		body += tool.Baro_button("/baro/notify/set", tool.Baro_t("알림 설정 열기"), "primary")

		return tool.Baro_page(db, config, tool.Baro_t("일일 요약"), body, baro_account_menu())
	}

	names := []string{}
	for _, kind := range daily_kinds {
		names = append(names, baro_account_notify_kind_name(kind))
	}
	body += tool.Baro_card(tool.Baro_t("요약으로 받는 종류"), tool.Baro_pills(names))

	page_int := baro_account_page_int(page)
	max_count := 7

	body += baro_account_notify_digest_html(db, user, daily_kinds, page_int, max_count)

	return tool.Baro_page(db, config, tool.Baro_t("일일 요약"), body, baro_account_menu())
}

// baro_account_notify_digest_html : 날짜별로 묶은 요약
func baro_account_notify_digest_html(db *sql.DB, user string, kinds []string, page_int int, max_count int) string {
	holders := []string{}
	args := []any{user}
	for _, kind := range kinds {
		holders = append(holders, "?")
		args = append(args, kind)
	}

	rows := tool.Query_DB(
		db,
		"select id, kind, title, url, readme, date from baro_notify where user = ? and kind in ("+strings.Join(holders, ", ")+") order by id + 0 desc limit 500",
		args...,
	)
	defer rows.Close()

	type_order := []string{}
	grouped := map[string][][]string{}

	for rows.Next() {
		id, kind, title, link, readme, date := "", "", "", "", "", ""
		if err := rows.Scan(&id, &kind, &title, &link, &readme, &date); err != nil {
			continue
		}

		day := tool.Baro_date_only(date)
		if _, ok := grouped[day]; !ok {
			type_order = append(type_order, day)
		}

		grouped[day] = append(grouped[day], []string{id, kind, title, link, readme, date})
	}

	if len(type_order) == 0 {
		return tool.Baro_empty(tool.Baro_t("요약할 알림이 아직 없습니다.")) +
			tool.Baro_paging(page_int, 0, max_count, "/baro/notify/digest?page={}")
	}

	start := (page_int - 1) * max_count
	if start > len(type_order) {
		start = len(type_order)
	}
	end := start + max_count
	if end > len(type_order) {
		end = len(type_order)
	}

	shown := type_order[start:end]

	html := ""
	for _, day := range shown {
		items := grouped[day]

		kind_count := map[string]int{}
		unread := 0
		list_rows := [][]string{}

		for _, item := range items {
			kind_count[item[1]]++
			if item[4] == "" {
				unread++
			}

			title_html := tool.HTML_escape(tool.Baro_trunc(item[2], 90))
			if strings.HasPrefix(item[3], "/") {
				title_html = `<a href="` + tool.HTML_escape(item[3]) + `">` + title_html + `</a>`
			}

			list_rows = append(list_rows, []string{
				tool.HTML_escape(baro_account_notify_kind_name(item[1])),
				title_html,
				tool.Baro_time_cell(item[5]),
			})
		}

		summary := []string{}
		for _, row := range tool.Baro_notify_kind_list() {
			if kind_count[row[0]] > 0 {
				summary = append(summary, row[1]+" "+tool.Baro_itoa(kind_count[row[0]])+tool.Baro_t("건"))
			}
		}

		html += tool.Baro_card_ex(
			day,
			strings.Join(summary, " · "),
			tool.Baro_table([]string{"종류", tool.Baro_t("내용"), "시각"}, list_rows, tool.Baro_t("이 날은 알림이 없습니다.")),
			tool.Baro_t("이 날 안 읽은 알림 ")+tool.Baro_itoa(unread)+tool.Baro_t("건"),
		)
	}

	html += tool.Baro_paging(page_int, len(shown), max_count, "/baro/notify/digest?page={}")
	html += tool.Baro_note(tool.Baro_t("요약은 최근 500건까지 묶어서 보여 줍니다. 더 오래된 알림은 목록에서 확인하세요."))

	return html
}

// ============================================================
//  알림 골라 지우기
//
//  표 줄마다 고르기 칸을 두고, 고른 것만 한 번에 지운다.
//  표 칸 안에는 이미 「읽음」·「삭제」 폼이 들어 있어서 폼을 또 넣을 수 없다.
//  (겹친 폼은 브라우저가 버린다)
//  그래서 고르기 칸은 form 속성으로 바깥의 폼 하나에 딸려 보낸다.
// ============================================================

// baro_notify_pick_form_id : 고른 것을 지우는 폼의 이름
const baro_notify_pick_form_id = "bw_notify_pick"

// baro_notify_pick_all_html : 표 머리의 「모두 고르기」 칸
func baro_notify_pick_all_html() string {
	return `<input type="checkbox" id="` + baro_notify_pick_form_id + `_all"` +
		` title="` + tool.HTML_escape(tool.Baro_t("모두 고르기")) + `">`
}

// baro_notify_pick_script : 모두 고르기 단추가 하는 일
func baro_notify_pick_script() string {
	return `<script>
(function() {
    var all = document.getElementById('` + baro_notify_pick_form_id + `_all');
    if (all === null) { return; }

    var list = document.querySelectorAll('.bw_notify_pick');
    all.addEventListener('change', function() {
        for (var i = 0; i < list.length; i++) { list[i].checked = all.checked; }
    });

    for (var i = 0; i < list.length; i++) {
        list[i].addEventListener('change', function() {
            var every = true;
            for (var j = 0; j < list.length; j++) {
                if (!list[j].checked) { every = false; break; }
            }
            all.checked = every;
        });
    }
})();
</script>`
}
