package route

// 바로위키 8-5 멘토링
//
// baro_mentor 표를 쓴다.
//
//	멘토 등록 : mentor = 사용자, mentee = "", status = active, field = 분야
//	멘토링 관계 : mentor = 멘토, mentee = 멘티, status = pending / active / rejected / closed

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// baro_community_mentor_flag : 멘토링 기능 플래그 이름
const baro_community_mentor_flag = "mentoring"

// baro_community_mentor_row : 멘토링 표 한 줄
type baro_community_mentor_row struct {
	Id     string
	Mentor string
	Mentee string
	Status string
	Date   string
	Field  string
	Note   string
}

// baro_community_mentor_field_options : 분야 고르기 목록
func baro_community_mentor_field_options(db *sql.DB) [][]string {
	options := [][]string{{"", tool.Baro_t("분야 없음 (전체)")}}
	for _, row := range tool.Baro_field_list(db) {
		options = append(options, []string{row.Key, row.Name})
	}

	return options
}

// baro_community_mentor_is_mentor : 이미 멘토로 등록했는지 (등록 id 를 돌려준다)
func baro_community_mentor_is_mentor(db *sql.DB, user string) string {
	id := ""
	tool.QueryRow_DB(
		db,
		"select id from baro_mentor where mentor = ? and mentee = '' and status = 'active' order by id + 0 desc limit 1",
		[]any{&id}, user,
	)

	return id
}

// baro_community_mentor_status_badge : 관계 상태 배지
func baro_community_mentor_status_badge(status string) string {
	switch status {
	case "active":
		return tool.Baro_badge(tool.Baro_t("진행 중"), "ok")
	case "pending":
		return tool.Baro_badge(tool.Baro_t("수락 대기"), "warn")
	case "rejected":
		return tool.Baro_badge(tool.Baro_t("거절됨"), "danger")
	case "closed":
		return tool.Baro_badge(tool.Baro_t("종료"), "muted")
	}

	return tool.Baro_badge(status, "muted")
}

// baro_community_mentor_off_notice : 기능 플래그가 꺼져 있을 때의 안내 화면
func baro_community_mentor_off_notice(db *sql.DB, config tool.Config) string {
	body := tool.Baro_hero(tool.Baro_t("멘토링"), tool.Baro_t("지금은 멘토링 기능이 열려 있지 않습니다."), "")
	body += tool.Baro_alert("info", tool.Baro_t("멘토링 기능(mentoring)이 꺼져 있어 안내만 보여 드립니다. 기능이 열리면 멘토 등록과 신청을 할 수 있습니다."))
	body += tool.Baro_card(
		tool.Baro_t("그동안 도움을 받으려면"),
		tool.Baro_list([]string{
			tool.Baro_button("/baro/help", tool.Baro_t("바로위키 도움말"), "primary"),
			tool.Baro_button("/w/help:편집 방법", tool.Baro_t("편집 방법 문서"), ""),
			tool.Baro_button("/baro/talk", tool.Baro_t("토론에서 물어보기"), ""),
		}),
	)

	return tool.Baro_page(db, config, tool.Baro_t("멘토링"), body, baro_community_menu())
}

// baro_community_mentor_newbie_card : 보호 기간 중인 신규 기여자 안내 (8-5)
//
//	되돌리기 대신 먼저 말을 걸도록 안내하고, 그 사용자의 기여 목록으로 바로 이어 준다.
func baro_community_mentor_newbie_card(db *sql.DB, config tool.Config) string {
	list := baro_community_new_user_list(db, 10)
	if len(list) == 0 {
		return ""
	}

	rows := [][]string{}
	for _, row := range list {
		left := baro_community_new_days - tool.Baro_days_between(row[1], tool.Baro_now())
		if left < 0 {
			left = 0
		}

		rows = append(rows, []string{
			tool.Baro_user_link(db, row[0], config.IP),
			tool.Baro_time_cell(row[1]),
			tool.HTML_escape(row[2] + tool.Baro_t("건")),
			tool.Baro_badge(tool.Baro_t("보호 기간 ")+tool.Baro_itoa(left)+tool.Baro_t("일 남음"), "warn"),
			`<a class="bw_btn" href="/baro/contrib/user/` + tool.Url_parser(row[0]) + tool.Baro_t(`">기여 목록</a> `) +
				`<a class="bw_btn" href="/baro/talk/user/` + tool.Url_parser(row[0]) + tool.Baro_t(`">말 걸기</a>`),
		})
	}

	body := tool.Baro_alert(
		"info",
		tool.Baro_t("가입 30일이 지나지 않은 기여자입니다. <strong>되돌리기 전에 먼저 말을 걸어 주세요.</strong> ")+
			tool.Baro_t("고칠 곳이 있으면 무엇을 어떻게 고치면 되는지 알려 주는 편이 훨씬 도움이 됩니다."),
	)
	body += tool.Baro_table([]string{tool.Baro_t("기여자"), "가입", tool.Baro_t("기여 수"), "상태", tool.Baro_t("이어가기")}, rows, "")

	return tool.Baro_card(tool.Baro_t("보호 기간 중인 신규 기여자"), body)
}

// View_baro_community_mentor : 8-5 멘토 목록 · 등록 · 신청
func View_baro_community_mentor(config tool.Config, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_flag_on(db, config.IP, baro_community_mentor_flag) {
		return baro_community_mentor_off_notice(db, config)
	}

	// ---------- 처리 ----------
	if values != nil {
		return baro_community_mentor_post(db, config, values)
	}

	field := baro_community_get(query, "field")
	page_int := baro_community_page_int(page)
	offset := (page_int - 1) * baro_community_page_size

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"", tool.Baro_t("멘토링")},
	})
	body += tool.Baro_hero(
		tool.Baro_t("멘토링"),
		tool.Baro_t("처음 오신 분이 헤매지 않도록 분야별 멘토가 함께합니다."),
		tool.Baro_button("/baro/mentor/mine", tool.Baro_t("내 멘토링"), "primary")+" "+
			tool.Baro_button("/baro/talk", tool.Baro_t("토론 허브"), ""),
	)

	// 초보 기여자 보호 기간 안내 (8-5)
	body += baro_community_newbie_guard(db, config.IP)

	// 분야별 도움 담당 (지금 도와줄 수 있는 멘토 이름까지 보여 준다)
	field_stats := [][]string{}
	for _, row := range tool.Baro_field_list(db) {
		count := 0
		tool.QueryRow_DB(
			db,
			"select count(*) from baro_mentor where mentee = '' and status = 'active' and field = ?",
			[]any{&count}, row.Key,
		)

		if count == 0 {
			continue
		}

		names := []string{}
		helpers := tool.Query_DB(
			db,
			"select mentor from baro_mentor where mentee = '' and status = 'active' and field = ? order by id + 0 desc limit 3",
			row.Key,
		)

		for helpers.Next() {
			mentor := ""
			if err := helpers.Scan(&mentor); err != nil {
				continue
			}

			names = append(names, tool.Baro_user_link(db, mentor, config.IP))
		}

		helpers.Close()

		value := tool.Baro_itoa(count) + tool.Baro_t("명")
		if len(names) > 0 {
			value += " · " + strings.Join(names, ", ")
		}
		if count > len(names) {
			value += tool.Baro_t(" 외 ") + tool.Baro_itoa(count-len(names)) + tool.Baro_t("명")
		}

		field_stats = append(field_stats, []string{
			`<a href="/baro/mentor?field=` + tool.Url_parser(row.Key) + `">` + tool.HTML_escape(row.Name) + `</a>`,
			value,
		})
	}

	if len(field_stats) > 0 {
		body += tool.Baro_card(
			tool.Baro_t("분야별 도움 담당"),
			tool.Baro_kv_grid(field_stats)+tool.Baro_note(tool.Baro_t("지금 그 분야에서 도와줄 수 있는 멘토입니다.")),
		)
	}

	// 초보 기여자 보호 기간 안내 (8-5) — 보호 기간 중인 기여자와 그 기여 목록을 함께 보여 준다
	body += baro_community_mentor_newbie_card(db, config)

	// 멘토 목록
	list := [][]string{}
	count := 0

	sql_text := "select id, mentor, status, date, field, note from baro_mentor where mentee = '' and status = 'active'"
	args := []any{}

	if field != "" {
		sql_text += " and field = ?"
		args = append(args, field)
	}

	sql_text += " order by id + 0 desc limit ?, ?"
	args = append(args, offset, baro_community_page_size)

	rows := tool.Query_DB(db, sql_text, args...)
	defer rows.Close()

	for rows.Next() {
		row := baro_community_mentor_row{}
		if err := rows.Scan(&row.Id, &row.Mentor, &row.Status, &row.Date, &row.Field, &row.Note); err != nil {
			continue
		}

		count++

		mentee_count := 0
		tool.QueryRow_DB(
			db,
			"select count(*) from baro_mentor where mentor = ? and mentee != '' and status = 'active'",
			[]any{&mentee_count}, row.Mentor,
		)

		apply := `<span class="bw_muted">-</span>`
		if tool.Baro_is_user(config.IP) && config.IP != row.Mentor {
			form := tool.Baro_hidden("act", "apply")
			form += tool.Baro_hidden("mentor", row.Mentor)
			form += tool.Baro_hidden("field", row.Field)
			form += tool.Baro_submit(tool.Baro_t("멘토 신청"), "primary")
			apply = tool.Baro_form("/baro/mentor", form)
		}

		list = append(list, []string{
			tool.Baro_user_link(db, row.Mentor, config.IP),
			tool.HTML_escape(tool.Baro_field_name(db, row.Field)),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Note), 80)),
			tool.Baro_itoa(mentee_count) + tool.Baro_t("명"),
			tool.Baro_time_cell(row.Date),
			apply,
		})
	}

	// 분야 탭
	tabs := [][]string{{"/baro/mentor", tool.Baro_t("전체")}}
	for _, row := range tool.Baro_field_list(db) {
		tabs = append(tabs, []string{"/baro/mentor?field=" + row.Key, row.Name})
	}

	active := "/baro/mentor"
	if field != "" {
		active = "/baro/mentor?field=" + field
	}
	body += tool.Baro_tabs(tabs, active)

	body += tool.Baro_card(
		tool.Baro_t("멘토 목록"),
		tool.Baro_table(
			[]string{tool.Baro_t("멘토"), "분야", tool.Baro_t("소개"), tool.Baro_t("돕는 멘티"), tool.Baro_t("등록"), tool.Baro_t("신청")},
			list,
			tool.Baro_t("아직 등록된 멘토가 없습니다. 먼저 멘토가 되어 주세요."),
		),
	)

	paging_url := "/baro/mentor?page={}"
	if field != "" {
		paging_url += "&field=" + tool.Url_parser(field)
	}
	body += tool.Baro_paging(page_int, count, baro_community_page_size, paging_url)

	// 멘토 등록 / 해제
	if tool.Baro_is_user(config.IP) {
		if id := baro_community_mentor_is_mentor(db, config.IP); id == "" {
			form := tool.Baro_hidden("act", "register")
			form += tool.Baro_select("field", tool.Baro_t("도울 분야"), baro_community_mentor_field_options(db), field, tool.Baro_t("가장 자신 있는 분야를 고릅니다."))
			form += tool.Baro_textarea("note", tool.Baro_t("소개"), "", 4, tool.Baro_t("어떤 도움을 줄 수 있는지 적습니다."), tool.Baro_t("멘토 목록에 보입니다."))
			form += tool.Baro_submit(tool.Baro_t("멘토로 등록"), "primary")

			body += tool.Baro_card("멘토 등록", tool.Baro_form("/baro/mentor", form))
		} else {
			form := tool.Baro_hidden("act", "unregister")
			form += tool.Baro_hidden("id", id)
			form += tool.Baro_submit("멘토 등록 해제", "danger")

			// 방금 처음 등록한 경우와 원래 등록돼 있던 경우를 구분해 알린다
			notice := tool.Baro_alert("ok", tool.Baro_t("이미 멘토로 등록되어 있습니다."))
			if baro_community_get(query, "done") == "register" {
				notice = tool.Baro_alert("ok", tool.Baro_t("멘토로 등록했습니다. 이제 멘토 목록에 보이고 멘티가 신청할 수 있습니다."))
			}

			body += tool.Baro_card(
				tool.Baro_t("멘토 등록 상태"),
				notice+
					tool.Baro_note(tool.Baro_t("해제해도 진행 중인 멘토링 관계는 남습니다. 내 멘토링 화면에서 정리해 주세요."))+
					tool.Baro_form("/baro/mentor", form),
			)
		}
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("멘토 등록과 신청은 로그인한 사용자만 할 수 있습니다."))
	}

	body += tool.Baro_note(tool.Baro_t("멘토는 대신 써 주는 사람이 아니라 길을 알려 주는 사람입니다. 되돌리기 전에 먼저 말을 걸어 주세요."))

	return tool.Baro_page(db, config, tool.Baro_t("멘토링"), body, baro_community_menu())
}

// baro_community_mentor_post : 멘토 등록 · 해제 · 신청 처리
func baro_community_mentor_post(db *sql.DB, config tool.Config, values url.Values) string {
	back := "/baro/mentor"

	if wall, need := tool.Baro_login_wall(db, config, back); need {
		return wall
	}

	if block := baro_community_write_block(db); block != "" {
		return tool.Baro_deny(db, config, block)
	}

	if !tool.Baro_rate_ok(db, "community_mentor", config.IP, 20, 600) {
		return tool.Baro_deny(db, config, tool.Baro_t("짧은 시간에 너무 많이 요청했습니다. 잠시 뒤에 다시 시도해 주세요."))
	}

	switch baro_community_get(values, "act") {
	case "register":
		// 이미 등록돼 있으면 새로 넣지 않고 그대로 돌아간다 (안내 문구도 "이미 등록" 이 된다)
		if baro_community_mentor_is_mentor(db, config.IP) != "" {
			return tool.Get_redirect(back)
		}

		field := baro_community_get(values, "field")
		note := tool.Baro_trunc(baro_community_get(values, "note"), 1000)

		tool.Exec_DB(
			db,
			"insert into baro_mentor (id, mentor, mentee, status, date, field, note) values (?, ?, '', 'active', ?, ?, ?)",
			tool.Baro_next_id(db, "baro_mentor"), config.IP, tool.Baro_now(), field, note,
		)
		tool.Baro_public_log(db, "mentor_register", config.IP, config.IP, tool.Baro_field_name(db, field), "멘토 등록")

		// 처음 등록에 성공했다는 표시를 달고 돌아간다
		return tool.Get_redirect(back + "?done=register")

	case "unregister":
		id := baro_community_get(values, "id")
		owner := ""
		if !tool.QueryRow_DB(db, "select mentor from baro_mentor where id = ? and mentee = ''", []any{&owner}, id) {
			return tool.Baro_deny(db, config, tool.Baro_t("그런 멘토 등록이 없습니다."))
		}

		if owner != config.IP && !tool.Baro_is_admin(db, config.IP) {
			return tool.Baro_deny(db, config, tool.Baro_t("본인 등록만 해제할 수 있습니다."))
		}

		tool.Exec_DB(db, "update baro_mentor set status = 'closed' where id = ?", id)
		tool.Baro_public_log(db, "mentor_unregister", config.IP, owner, "", "멘토 등록 해제")

	case "apply":
		mentor := baro_community_get(values, "mentor")
		if mentor == "" || mentor == config.IP {
			return tool.Baro_deny(db, config, tool.Baro_t("신청할 멘토를 고르지 못했습니다."))
		}

		if baro_community_mentor_is_mentor(db, mentor) == "" {
			return tool.Baro_deny(db, config, tool.Baro_t("지금은 멘토로 등록되어 있지 않은 사용자입니다."))
		}

		found := ""
		if tool.QueryRow_DB(
			db,
			"select id from baro_mentor where mentor = ? and mentee = ? and status in ('pending', 'active')",
			[]any{&found}, mentor, config.IP,
		) {
			return tool.Get_redirect("/baro/mentor/mine")
		}

		field := baro_community_get(values, "field")
		note := tool.Baro_trunc(baro_community_get(values, "note"), 1000)

		tool.Exec_DB(
			db,
			"insert into baro_mentor (id, mentor, mentee, status, date, field, note) values (?, ?, ?, 'pending', ?, ?, ?)",
			tool.Baro_next_id(db, "baro_mentor"), mentor, config.IP, tool.Baro_now(), field, note,
		)

		tool.Baro_notify(
			db, mentor, "system",
			config.IP+tool.Baro_t(" 님이 멘토링을 신청했습니다"),
			"/baro/mentor/mine", tool.Baro_field_name(db, field),
		)

		return tool.Get_redirect("/baro/mentor/mine")
	}

	return tool.Get_redirect(back)
}

// View_baro_community_mentor_mine : 8-5 내 멘토링 관계
func View_baro_community_mentor_mine(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_flag_on(db, config.IP, baro_community_mentor_flag) {
		return baro_community_mentor_off_notice(db, config)
	}

	if wall, need := tool.Baro_login_wall(db, config, "/baro/mentor/mine"); need {
		return wall
	}

	// ---------- 처리 ----------
	if values != nil {
		return baro_community_mentor_mine_post(db, config, values)
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/mentor", tool.Baro_t("멘토링")},
		{"", tool.Baro_t("내 멘토링")},
	})
	body += tool.Baro_hero(
		tool.Baro_t("내 멘토링"),
		tool.Baro_t("멘토로서 돕는 사람과 멘티로서 도움받는 사람을 모아 봅니다."),
		tool.Baro_button("/baro/mentor", tool.Baro_t("멘토 목록"), "primary"),
	)
	body += tool.Baro_tabs([][]string{
		{"/baro/mentor", tool.Baro_t("멘토 목록")},
		{"/baro/mentor/mine", tool.Baro_t("내 멘토링")},
	}, "/baro/mentor/mine")

	body += baro_community_newbie_guard(db, config.IP)

	if done := baro_community_get(query, "done"); done != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("요청을 처리했습니다."))
	}

	// 멘토로서
	as_mentor := [][]string{}
	rows := tool.Query_DB(
		db,
		"select id, mentee, status, date, field, note from baro_mentor where mentor = ? and mentee != '' order by id + 0 desc limit 100",
		config.IP,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_community_mentor_row{}
		if err := rows.Scan(&row.Id, &row.Mentee, &row.Status, &row.Date, &row.Field, &row.Note); err != nil {
			continue
		}

		manage := `<span class="bw_muted">-</span>`
		if row.Status == "pending" {
			accept := tool.Baro_hidden("act", "accept") + tool.Baro_hidden("id", row.Id) + tool.Baro_submit(tool.Baro_t("수락"), "primary")
			reject := tool.Baro_hidden("act", "reject") + tool.Baro_hidden("id", row.Id) + tool.Baro_submit(tool.Baro_t("거절"), "")
			manage = tool.Baro_form("/baro/mentor/mine", accept) + tool.Baro_form("/baro/mentor/mine", reject)
		} else if row.Status == "active" {
			closer := tool.Baro_hidden("act", "close") + tool.Baro_hidden("id", row.Id) + tool.Baro_submit(tool.Baro_t("종료"), "")
			manage = tool.Baro_form("/baro/mentor/mine", closer)
		}

		guard := ""
		if baro_community_is_new_user(db, row.Mentee) {
			guard = tool.Baro_badge(tool.Baro_t("보호 기간"), "warn")
		}

		as_mentor = append(as_mentor, []string{
			tool.Baro_user_link(db, row.Mentee, config.IP) + " " + guard,
			tool.HTML_escape(tool.Baro_field_name(db, row.Field)),
			baro_community_mentor_status_badge(row.Status),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Note), 100)),
			tool.Baro_time_cell(row.Date),
			manage + " " + `<a class="bw_btn" href="/baro/talk/user/` + tool.Url_parser(row.Mentee) + tool.Baro_t(`">말 걸기</a>`),
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("멘토로서"),
		tool.Baro_table(
			[]string{tool.Baro_t("멘티"), "분야", "상태", tool.Baro_t("메모"), "시각", "관리"},
			as_mentor,
			tool.Baro_t("아직 신청받은 멘티가 없습니다."),
		),
	)

	// 멘티로서
	as_mentee := [][]string{}
	mine := tool.Query_DB(
		db,
		"select id, mentor, status, date, field, note from baro_mentor where mentee = ? order by id + 0 desc limit 100",
		config.IP,
	)
	defer mine.Close()

	active_mentor := []string{}
	for mine.Next() {
		row := baro_community_mentor_row{}
		if err := mine.Scan(&row.Id, &row.Mentor, &row.Status, &row.Date, &row.Field, &row.Note); err != nil {
			continue
		}

		if row.Status == "active" {
			active_mentor = append(active_mentor, row.Id+"|"+row.Mentor)
		}

		manage := `<a class="bw_btn" href="/baro/talk/user/` + tool.Url_parser(row.Mentor) + tool.Baro_t(`">말 걸기</a>`)
		if row.Status == "pending" || row.Status == "active" {
			closer := tool.Baro_hidden("act", "close") + tool.Baro_hidden("id", row.Id) + tool.Baro_submit(tool.Baro_t("그만두기"), "")
			manage += tool.Baro_form("/baro/mentor/mine", closer)
		}

		as_mentee = append(as_mentee, []string{
			tool.Baro_user_link(db, row.Mentor, config.IP),
			tool.HTML_escape(tool.Baro_field_name(db, row.Field)),
			baro_community_mentor_status_badge(row.Status),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Note), 100)),
			tool.Baro_time_cell(row.Date),
			manage,
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("멘티로서"),
		tool.Baro_table(
			[]string{tool.Baro_t("멘토"), "분야", "상태", tool.Baro_t("메모"), "시각", "관리"},
			as_mentee,
			tool.Baro_t("아직 신청한 멘토가 없습니다. 멘토 목록에서 신청해 보세요."),
		),
	)

	// 첫 문서 검토 요청
	if len(active_mentor) > 0 {
		options := [][]string{}
		for _, item := range active_mentor {
			parts := strings.SplitN(item, "|", 2)
			options = append(options, []string{parts[0], parts[1] + tool.Baro_t(" 멘토")})
		}

		// 내 연습장 초안 목록
		draft_options := [][]string{{"", tool.Baro_t("문서 이름을 직접 적기")}}
		drafts := tool.Query_DB(
			db,
			"select id, title, status from baro_workspace where user = ? and project not like 'projecttalk:%' and project != 'usertalk' and project != 'sandbox' and status != 'done' order by id + 0 desc limit 20",
			config.IP,
		)
		defer drafts.Close()

		for drafts.Next() {
			id, title, status := "", "", ""
			if err := drafts.Scan(&id, &title, &status); err != nil {
				continue
			}

			open_text := tool.Baro_t("비공개")
			if status == "open" {
				open_text = "공개"
			}

			draft_options = append(draft_options, []string{"draft:" + id, tool.Baro_t("초안 : ") + tool.Baro_trunc(title, 40) + " (" + open_text + ")"})
		}

		form := tool.Baro_hidden("act", "review")
		form += tool.Baro_select("id", tool.Baro_t("검토를 부탁할 멘토"), options, "", tool.Baro_t("진행 중인 멘토링 관계만 고를 수 있습니다."))
		form += tool.Baro_select("draft", tool.Baro_t("연습장 초안"), draft_options, "", tool.Baro_t("연습장에 있는 초안을 고르거나 아래에 문서 이름을 적습니다."))
		form += tool.Baro_input("target", tool.Baro_t("문서 이름"), "", tool.Baro_t("예: 바로위키"), tool.Baro_t("초안을 고르지 않았다면 문서 이름을 적습니다."))
		form += tool.Baro_textarea("note", tool.Baro_t("부탁할 내용"), "", 4, tool.Baro_t("어디를 봐 주면 좋을지 적습니다."), "")
		form += tool.Baro_submit(tool.Baro_t("검토 요청 보내기"), "primary")

		body += tool.Baro_card(tool.Baro_t("첫 문서 검토 요청"), tool.Baro_form("/baro/mentor/mine", form))
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("멘토가 신청을 수락하면 첫 문서 검토를 요청할 수 있습니다."))
	}

	body += tool.Baro_note(tool.Baro_t("멘토링 기록은 공개 로그에 남습니다. 개인적인 연락처를 주고받지 마세요."))

	return tool.Baro_page(db, config, tool.Baro_t("내 멘토링"), body, baro_community_menu())
}

// baro_community_mentor_mine_post : 수락 · 거절 · 종료 · 검토 요청 처리
func baro_community_mentor_mine_post(db *sql.DB, config tool.Config, values url.Values) string {
	back := "/baro/mentor/mine"

	if block := baro_community_write_block(db); block != "" {
		return tool.Baro_deny(db, config, block)
	}

	if !tool.Baro_rate_ok(db, "community_mentor_mine", config.IP, 30, 600) {
		return tool.Baro_deny(db, config, "짧은 시간에 너무 많이 요청했습니다. 잠시 뒤에 다시 시도해 주세요.")
	}

	id := baro_community_get(values, "id")
	row := baro_community_mentor_row{Id: id}
	if !tool.QueryRow_DB(
		db,
		"select mentor, mentee, status, field, note from baro_mentor where id = ?",
		[]any{&row.Mentor, &row.Mentee, &row.Status, &row.Field, &row.Note}, id,
	) {
		return tool.Baro_deny(db, config, "그런 멘토링 관계가 없습니다.")
	}

	if row.Mentor != config.IP && row.Mentee != config.IP {
		return tool.Baro_deny(db, config, "내 멘토링 관계만 고칠 수 있습니다.")
	}

	switch baro_community_get(values, "act") {
	case "accept":
		if row.Mentor != config.IP {
			return tool.Baro_deny(db, config, "수락은 멘토만 할 수 있습니다.")
		}

		tool.Exec_DB(db, "update baro_mentor set status = 'active' where id = ?", id)
		tool.Baro_notify(db, row.Mentee, "system", config.IP+" 멘토가 신청을 수락했습니다", back, "")
		tool.Baro_public_log(db, "mentor_accept", config.IP, row.Mentee, tool.Baro_field_name(db, row.Field), "멘토링 수락")

	case "reject":
		if row.Mentor != config.IP {
			return tool.Baro_deny(db, config, "거절은 멘토만 할 수 있습니다.")
		}

		tool.Exec_DB(db, "update baro_mentor set status = 'rejected' where id = ?", id)
		tool.Baro_notify(db, row.Mentee, "system", "멘토링 신청이 받아들여지지 않았습니다", "/baro/mentor", "다른 멘토에게 신청해 보세요.")

	case "close":
		tool.Exec_DB(db, "update baro_mentor set status = 'closed' where id = ?", id)

		other := row.Mentee
		if config.IP == row.Mentee {
			other = row.Mentor
		}
		tool.Baro_notify(db, other, "system", config.IP+" 님이 멘토링을 마쳤습니다", back, "")

	case "review":
		if row.Mentee != config.IP {
			return tool.Baro_deny(db, config, "검토 요청은 멘티만 보낼 수 있습니다.")
		}

		if row.Status != "active" {
			return tool.Baro_deny(db, config, "진행 중인 멘토링 관계에서만 검토를 요청할 수 있습니다.")
		}

		draft := baro_community_get(values, "draft")
		target := tool.Baro_trunc(baro_community_get(values, "target"), 200)
		note := tool.Baro_trunc(baro_community_get(values, "note"), 1000)

		link := "/baro/mentor/mine"
		what := ""

		if strings.HasPrefix(draft, "draft:") {
			draft_id := strings.TrimPrefix(draft, "draft:")
			draft_user, draft_title := "", ""
			if !tool.QueryRow_DB(
				db,
				"select user, title from baro_workspace where id = ?",
				[]any{&draft_user, &draft_title}, draft_id,
			) || draft_user != config.IP {
				return tool.Baro_deny(db, config, "내 초안만 검토 요청할 수 있습니다.")
			}

			what = "연습장 초안 : " + tool.Baro_trunc(draft_title, 100)
			link = "/baro/draft/edit/" + tool.Url_parser(draft_id)
		} else {
			if target == "" {
				return tool.Baro_deny(db, config, "검토받을 초안이나 문서를 골라 주세요.")
			}

			target = tool.Namespace_normalize(target)
			what = "문서 : " + target
			link = "/w/" + tool.Url_parser(target)
		}

		memo := tool.Baro_trunc(row.Note+tool.Baro_t("\n[검토 요청 ")+tool.Baro_now()+"] "+what+" / "+note, 3000)
		tool.Exec_DB(db, "update baro_mentor set note = ? where id = ?", memo, id)

		tool.Baro_notify(
			db, row.Mentor, "review",
			config.IP+" 님이 첫 문서 검토를 요청했습니다",
			link, what,
		)
		tool.Baro_metric_bump(db, "mentor_review_req", tool.Baro_today(), 1)

	default:
		return tool.Get_redirect(back)
	}

	return tool.Get_redirect(back + "?done=1")
}
