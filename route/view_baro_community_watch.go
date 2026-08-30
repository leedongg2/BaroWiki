package route

// 바로위키 8-4 관심 문서 · 주시 목록
//
// baro_watch 표를 쓴다. kind 는 document / project / field 세 가지뿐이다.
// ★ 사람을 팔로우하는 SNS 식 기능은 만들지 않는다. 주시 대상은 언제나 "주제"다.

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// baro_community_watch_kinds : 주시할 수 있는 대상 (사용자는 없다)
func baro_community_watch_kinds() [][]string {
	return [][]string{
		{"document", "문서"},
		{"project", "프로젝트"},
		{"field", "분야"},
	}
}

// baro_community_watch_kind_ok : 허용된 주시 종류인지
func baro_community_watch_kind_ok(kind string) bool {
	for _, row := range baro_community_watch_kinds() {
		if row[0] == kind {
			return true
		}
	}

	return false
}

// baro_community_watch_kind_name : 종류 표시 이름
func baro_community_watch_kind_name(kind string) string {
	for _, row := range baro_community_watch_kinds() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// baro_community_watch_modes : 변경 알림 방식
func baro_community_watch_modes() [][]string {
	return [][]string{
		{"on", tool.Baro_t("즉시 알림")},
		{"daily", tool.Baro_t("일일 요약")},
		{"off", tool.Baro_t("끄기")},
	}
}

// baro_community_watch_mode_name : 알림 방식 이름
func baro_community_watch_mode_name(mode string) string {
	for _, row := range baro_community_watch_modes() {
		if row[0] == mode {
			return row[1]
		}
	}

	return tool.Baro_t("즉시 알림")
}

// baro_community_watch_target_link : 주시 대상으로 가는 링크
func baro_community_watch_target_link(db *sql.DB, kind string, target string) string {
	switch kind {
	case "document":
		return tool.Baro_doc_link(target)
	case "project":
		return `<a href="/baro/project/view/` + tool.Url_parser(target) + `">` + tool.HTML_escape(baro_community_project_name(db, target)) + `</a>`
	case "field":
		return `<a href="/baro/field/view/` + tool.Url_parser(target) + `">` + tool.HTML_escape(tool.Baro_field_name(db, target)) + `</a>`
	}

	return tool.HTML_escape(target)
}

// baro_community_watch_last : 주시 대상의 최근 변경 시각과 설명
func baro_community_watch_last(db *sql.DB, config tool.Config, kind string, target string) (string, string) {
	switch kind {
	case "document":
		date, who, send := "", "", ""
		if tool.QueryRow_DB(
			db,
			"select date, ip, send from history where title = ? order by id + 0 desc limit 1",
			[]any{&date, &who, &send}, target,
		) {
			text := tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(send), 60))
			if text == "" {
				text = tool.Baro_t(`<span class="bw_muted">(요약 없음)</span>`)
			}

			return date, tool.Baro_user_link(db, who, config.IP) + " " + text
		}

	case "project":
		date, title := "", ""
		if tool.QueryRow_DB(
			db,
			"select date, title from baro_project_task where project = ? order by id + 0 desc limit 1",
			[]any{&date, &title}, target,
		) {
			return date, tool.HTML_escape(tool.Baro_t("최근 과제 : ") + tool.Baro_trunc(title, 60))
		}

		project_date := ""
		if tool.QueryRow_DB(db, "select date from baro_project where id = ?", []any{&project_date}, target) {
			return project_date, tool.HTML_escape(tool.Baro_t("프로젝트가 열렸습니다"))
		}

	case "field":
		date, who, title := "", "", ""
		if tool.QueryRow_DB(
			db,
			"select date, user, title from baro_contrib_event where field = ? order by id + 0 desc limit 1",
			[]any{&date, &who, &title}, target,
		) {
			return date, tool.Baro_user_link(db, who, config.IP) + " " + tool.HTML_escape(tool.Baro_trunc(title, 50))
		}
	}

	return "", tool.Baro_t(`<span class="bw_muted">최근 변경 없음</span>`)
}

// baro_community_notify_watchers : 주시 중인 사람에게 알림을 보낸다
//
//	mode 가 off 면 건너뛰고, daily 면 요약 피드에서만 보이도록 알림을 만들지 않는다.
func baro_community_notify_watchers(db *sql.DB, kind string, target string, actor string, title string, link string) int {
	if !baro_community_watch_kind_ok(kind) {
		return 0
	}

	sent := 0
	rows := tool.Query_DB(
		db,
		"select user, mode from baro_watch where kind = ? and target = ? limit 300",
		kind, target,
	)
	defer rows.Close()

	targets := [][]string{}
	for rows.Next() {
		user, mode := "", ""
		if err := rows.Scan(&user, &mode); err != nil {
			continue
		}

		if user == actor || mode == "off" || mode == "daily" {
			continue
		}

		targets = append(targets, []string{user, mode})
	}

	for _, row := range targets {
		tool.Baro_notify(db, row[0], "watch", title, link, kind+":"+target)
		sent++
	}

	return sent
}

// baro_community_watch_policy : 사람이 아니라 주제를 따라간다는 방침
func baro_community_watch_policy() string {
	return tool.Baro_alert(
		"info",
		tool.Baro_t("바로위키는 <strong>사람이 아니라 주제를 따라갑니다.</strong> ")+
			tool.Baro_t("특정 사용자의 편집을 따라다니는 팔로우 기능은 제공하지 않습니다. ")+
			tool.Baro_t("문서 · 프로젝트 · 분야만 주시할 수 있습니다."),
	)
}

// baro_community_watch_count : 종류별 주시 개수
func baro_community_watch_count(db *sql.DB, user string, kind string) int {
	count := 0
	tool.QueryRow_DB(
		db,
		"select count(*) from baro_watch where user = ? and kind = ?",
		[]any{&count}, user, kind,
	)

	return count
}

// baro_community_watch_kind_card : 한 종류의 주시 목록 카드 (카드 HTML 과 이 쪽의 줄 수를 돌려준다)
//
//	각 줄에서 알림 방식을 바꾸거나 주시를 해제할 수 있다.
func baro_community_watch_kind_card(db *sql.DB, config tool.Config, kind string, offset int) (string, int) {
	list := [][]string{}
	count := 0

	rows := tool.Query_DB(
		db,
		"select id, target, date, mode from baro_watch where user = ? and kind = ? order by id + 0 desc limit ?, ?",
		config.IP, kind, offset, baro_community_page_size,
	)
	defer rows.Close()

	for rows.Next() {
		id, target, date, mode := "", "", "", ""
		if err := rows.Scan(&id, &target, &date, &mode); err != nil {
			continue
		}

		count++
		last_date, last_text := baro_community_watch_last(db, config, kind, target)

		mode_kind := "info"
		if mode == "off" {
			mode_kind = "muted"
		}

		mode_form := tool.Baro_badge(baro_community_watch_mode_name(mode), mode_kind)
		mode_form += tool.Baro_hidden("act", "mode")
		mode_form += tool.Baro_hidden("id", id)
		mode_form += tool.Baro_hidden("kind", kind)
		mode_form += tool.Baro_select("mode", "", baro_community_watch_modes(), mode, "")
		mode_form += tool.Baro_submit("저장", "")

		off_form := tool.Baro_hidden("act", "remove")
		off_form += tool.Baro_hidden("id", id)
		off_form += tool.Baro_hidden("kind", kind)
		off_form += tool.Baro_submit("해제", "danger")

		list = append(list, []string{
			baro_community_watch_target_link(db, kind, target),
			tool.Baro_time_cell(date),
			tool.Baro_time_cell(last_date),
			last_text,
			tool.Baro_form("/baro/watch", mode_form),
			tool.Baro_form("/baro/watch", off_form),
		})
	}

	name := baro_community_watch_kind_name(kind)

	return tool.Baro_card(
		name+tool.Baro_t(" 주시 목록"),
		tool.Baro_table(
			[]string{tool.Baro_t("대상"), tool.Baro_t("주시 시작"), tool.Baro_t("최근 변경"), tool.Baro_t("변경 내용"), tool.Baro_t("알림 방식"), "해제"},
			list,
			tool.Baro_t("아직 주시하는 ")+name+tool.Baro_t("가 없습니다."),
		),
	), count
}

// View_baro_community_watch : 8-4 내 주시 목록
func View_baro_community_watch(config tool.Config, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/watch"); need {
		return wall
	}

	// ---------- 처리 ----------
	if values != nil {
		return baro_community_watch_post(db, config, values)
	}

	// 주시 추가 폼에서 미리 골라 둘 종류 (없으면 문서)
	kind := baro_community_get(query, "kind")
	if !baro_community_watch_kind_ok(kind) {
		kind = "document"
	}

	page_int := baro_community_page_int(page)
	offset := (page_int - 1) * baro_community_page_size

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"", tool.Baro_t("주시 목록")},
	})
	body += tool.Baro_hero(
		tool.Baro_t("주시 목록"),
		tool.Baro_t("관심 있는 문서 · 프로젝트 · 분야의 변화를 모아 봅니다."),
		tool.Baro_button("/baro/watch/feed", tool.Baro_t("변경 모아 보기"), "primary")+" "+
			tool.Baro_button("/baro/talk", tool.Baro_t("토론 허브"), ""),
	)
	body += baro_community_watch_policy()

	// 종류별 주시 개수
	stats := [][]string{}
	total := 0
	for _, row := range baro_community_watch_kinds() {
		count := baro_community_watch_count(db, config.IP, row[0])
		total += count
		stats = append(stats, []string{row[1], tool.Baro_itoa(count), tool.Baro_t("주시 중")})
	}

	body += tool.Baro_stat_row(stats)

	// 문서 · 프로젝트 · 분야 세 종류를 한 화면에 모두 보여 준다.
	// (예전에는 고른 종류 하나만 보여 주어서 프로젝트 · 분야 주시를 해제할 방법이 없었다)
	max_count := 0
	for _, row := range baro_community_watch_kinds() {
		card, count := baro_community_watch_kind_card(db, config, row[0], offset)
		body += card

		if count > max_count {
			max_count = count
		}
	}

	if total == 0 {
		body += tool.Baro_empty(tool.Baro_t("아직 주시하는 대상이 없습니다. 아래에서 문서 · 프로젝트 · 분야를 추가해 보세요."))
	}

	body += tool.Baro_paging(page_int, max_count, baro_community_page_size, "/baro/watch?page={}")

	// 직접 추가
	add := tool.Baro_select("kind", "종류", baro_community_watch_kinds(), kind, tool.Baro_t("문서 · 프로젝트 · 분야만 주시할 수 있습니다."))
	add += tool.Baro_input("target", tool.Baro_t("대상"), "", tool.Baro_t("문서 이름 / 프로젝트 id / 분야 키"), "")
	add += tool.Baro_select("mode", tool.Baro_t("알림 방식"), baro_community_watch_modes(), "on", tool.Baro_t("즉시 알림 · 일일 요약 · 끄기 중에서 고릅니다."))
	add += tool.Baro_submit(tool.Baro_t("주시 추가"), "primary")
	body += tool.Baro_card(tool.Baro_t("주시 추가"), tool.Baro_form("/baro/watch/add", add))

	body += tool.Baro_note("문서 화면에서 /baro/watch/add?kind=document&target=<문서> 를 눌러도 바로 추가할 수 있습니다.")

	return tool.Baro_page(db, config, tool.Baro_t("주시 목록"), body, baro_community_menu())
}

// baro_community_watch_post : 주시 목록의 알림 방식 변경 · 해제
func baro_community_watch_post(db *sql.DB, config tool.Config, values url.Values) string {
	// 세 종류를 한 화면에 모두 보여 주므로 처리 뒤에는 목록으로 그대로 돌아간다
	back := "/baro/watch"

	if block := baro_community_write_block(db); block != "" {
		return tool.Baro_deny(db, config, block)
	}

	id := baro_community_get(values, "id")
	owner := ""
	if !tool.QueryRow_DB(db, "select user from baro_watch where id = ?", []any{&owner}, id) {
		return tool.Baro_deny(db, config, tool.Baro_t("그런 주시 항목이 없습니다."))
	}

	if owner != config.IP {
		return tool.Baro_deny(db, config, tool.Baro_t("내 주시 목록만 고칠 수 있습니다."))
	}

	switch baro_community_get(values, "act") {
	case "mode":
		mode := baro_community_get(values, "mode")
		if mode != "on" && mode != "daily" && mode != "off" {
			mode = "on"
		}

		tool.Exec_DB(db, "update baro_watch set mode = ? where id = ?", mode, id)

	case "remove":
		tool.Exec_DB(db, "delete from baro_watch where id = ?", id)
	}

	return tool.Get_redirect(back)
}

// View_baro_community_watch_add : 8-4 주시 추가 (/baro/watch/add?kind=&target=)
func View_baro_community_watch_add(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/watch"); need {
		return wall
	}

	kind := baro_community_get(query, "kind")
	target := baro_community_get(query, "target")

	if values != nil {
		if v := baro_community_get(values, "kind"); v != "" {
			kind = v
		}
		if v := baro_community_get(values, "target"); v != "" {
			target = v
		}
	}

	// 사람 팔로우는 만들지 않는다 (8-4 방침)
	if kind == "user" {
		return tool.Baro_deny(db, config, tool.Baro_t("사용자 팔로우는 제공하지 않습니다. 바로위키는 사람이 아니라 주제를 따라갑니다."))
	}

	if !baro_community_watch_kind_ok(kind) {
		kind = "document"
	}

	// ---------- 처리 ----------
	if values != nil {
		if block := baro_community_write_block(db); block != "" {
			return tool.Baro_deny(db, config, block)
		}

		if target == "" {
			return tool.Baro_deny(db, config, tool.Baro_t("주시할 대상을 적어 주세요."))
		}

		if !tool.Baro_rate_ok(db, "community_watch_add", config.IP, 60, 600) {
			return tool.Baro_deny(db, config, tool.Baro_t("짧은 시간에 너무 많이 추가했습니다. 잠시 뒤에 다시 시도해 주세요."))
		}

		if kind == "document" {
			target = tool.Namespace_normalize(target)
		}

		mode := baro_community_get(values, "mode")
		if mode != "on" && mode != "daily" && mode != "off" {
			mode = "on"
		}

		found := ""
		if tool.QueryRow_DB(
			db,
			"select id from baro_watch where user = ? and kind = ? and target = ?",
			[]any{&found}, config.IP, kind, target,
		) {
			tool.Exec_DB(db, "update baro_watch set mode = ? where id = ?", mode, found)
		} else {
			tool.Exec_DB(
				db,
				"insert into baro_watch (id, user, kind, target, date, mode) values (?, ?, ?, ?, ?, ?)",
				tool.Baro_next_id(db, "baro_watch"), config.IP, kind, target, tool.Baro_now(), mode,
			)
			tool.Baro_metric_bump(db, "watch_add", tool.Baro_today(), 1)
		}

		return tool.Get_redirect("/baro/watch?kind=" + tool.Url_parser(kind))
	}

	// ---------- 확인 화면 ----------
	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/watch", tool.Baro_t("주시 목록")},
		{"", tool.Baro_t("주시 추가")},
	})
	body += tool.Baro_hero(tool.Baro_t("주시 추가"), tool.Baro_t("이 대상의 변화를 주시 목록에서 모아 봅니다."), "")
	body += baro_community_watch_policy()

	form := tool.Baro_select("kind", "종류", baro_community_watch_kinds(), kind, tool.Baro_t("문서 · 프로젝트 · 분야만 주시할 수 있습니다."))
	form += tool.Baro_input("target", tool.Baro_t("대상"), target, tool.Baro_t("문서 이름 / 프로젝트 id / 분야 키"), "")
	form += tool.Baro_select("mode", tool.Baro_t("알림 방식"), baro_community_watch_modes(), "on", tool.Baro_t("즉시 알림 · 일일 요약 · 끄기"))
	form += tool.Baro_submit(tool.Baro_t("주시 추가"), "primary")

	body += tool.Baro_card("확인", tool.Baro_form("/baro/watch/add", form))

	return tool.Baro_page(db, config, tool.Baro_t("주시 추가"), body, baro_community_menu())
}

// View_baro_community_watch_feed : 8-4 주시 중인 대상의 최근 변경 모아 보기
func View_baro_community_watch_feed(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/watch/feed"); need {
		return wall
	}

	span := baro_community_get(query, "span")
	days := 1
	span_name := tool.Baro_t("일일 요약")
	if span == "weekly" {
		days = 7
		span_name = tool.Baro_t("주간 요약")
	} else {
		span = "daily"
	}

	from := tool.Baro_time_add(tool.Baro_now(), -days, 0, 0)

	// 주시 목록 읽기
	type watch_row struct {
		Kind, Target, Mode string
	}

	watches := []watch_row{}
	rows := tool.Query_DB(
		db,
		"select kind, target, mode from baro_watch where user = ? order by id + 0 desc limit 200",
		config.IP,
	)
	defer rows.Close()

	for rows.Next() {
		row := watch_row{}
		if err := rows.Scan(&row.Kind, &row.Target, &row.Mode); err != nil {
			continue
		}

		if row.Mode == "off" {
			continue
		}

		watches = append(watches, row)
	}

	doc_rows := [][]string{}
	project_rows := [][]string{}
	field_rows := [][]string{}

	for _, watch := range watches {
		switch watch.Kind {
		case "document":
			list := tool.Query_DB(
				db,
				"select id, date, ip, send, leng from history where title = ? and date >= ? order by id + 0 desc limit 5",
				watch.Target, from,
			)

			for list.Next() {
				id, date, who, send, leng := "", "", "", "", ""
				if err := list.Scan(&id, &date, &who, &send, &leng); err != nil {
					continue
				}

				summary := tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(send), 60))
				if summary == "" {
					summary = tool.Baro_t(`<span class="bw_muted">(요약 없음)</span>`)
				}

				doc_rows = append(doc_rows, []string{
					tool.Baro_doc_link(watch.Target),
					`<a href="/w/` + tool.Url_parser(watch.Target) + `?rev=` + tool.Url_parser(id) + `">r` + tool.HTML_escape(id) + `</a>`,
					tool.Baro_user_link(db, who, config.IP),
					summary,
					tool.Baro_time_cell(date),
				})
			}

			list.Close()

		case "project":
			list := tool.Query_DB(
				db,
				"select id, title, status, date from baro_project_task where project = ? and date >= ? order by id + 0 desc limit 5",
				watch.Target, from,
			)

			for list.Next() {
				id, title, status, date := "", "", "", ""
				if err := list.Scan(&id, &title, &status, &date); err != nil {
					continue
				}

				project_rows = append(project_rows, []string{
					baro_community_watch_target_link(db, "project", watch.Target),
					`<a href="/baro/task/view/` + tool.Url_parser(id) + `">` + tool.HTML_escape(tool.Baro_trunc(title, 60)) + `</a>`,
					tool.Baro_badge(status, "info"),
					tool.Baro_time_cell(date),
				})
			}

			list.Close()

		case "field":
			list := tool.Query_DB(
				db,
				"select user, kind, title, date from baro_contrib_event where field = ? and date >= ? order by id + 0 desc limit 5",
				watch.Target, from,
			)

			for list.Next() {
				who, kind, title, date := "", "", "", ""
				if err := list.Scan(&who, &kind, &title, &date); err != nil {
					continue
				}

				field_rows = append(field_rows, []string{
					baro_community_watch_target_link(db, "field", watch.Target),
					tool.Baro_user_link(db, who, config.IP),
					tool.HTML_escape(tool.Baro_contrib_kind_name(kind)),
					tool.Baro_doc_link(title),
					tool.Baro_time_cell(date),
				})
			}

			list.Close()
		}
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/watch", tool.Baro_t("주시 목록")},
		{"", tool.Baro_t("변경 모아 보기")},
	})
	body += tool.Baro_hero(
		tool.Baro_t("주시 대상 변경 모아 보기"),
		span_name+" · "+from+tool.Baro_t(" 이후"),
		tool.Baro_button("/baro/watch", tool.Baro_t("주시 목록으로"), ""),
	)
	body += tool.Baro_tabs([][]string{
		{"/baro/watch/feed?span=daily", tool.Baro_t("일일 요약")},
		{"/baro/watch/feed?span=weekly", tool.Baro_t("주간 요약")},
	}, "/baro/watch/feed?span="+span)
	body += baro_community_watch_policy()

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("주시 대상"), tool.Baro_itoa(len(watches)), tool.Baro_t("알림이 켜진 항목")},
		{tool.Baro_t("문서 변경"), tool.Baro_itoa(len(doc_rows)), span_name},
		{tool.Baro_t("프로젝트 변화"), tool.Baro_itoa(len(project_rows)), span_name},
		{tool.Baro_t("분야 기여"), tool.Baro_itoa(len(field_rows)), span_name},
	})

	body += tool.Baro_card(
		tool.Baro_t("문서 변경"),
		tool.Baro_table([]string{"문서", "판", tool.Baro_t("편집자"), tool.Baro_t("요약"), "시각"}, doc_rows, tool.Baro_t("주시 중인 문서에 변경이 없습니다.")),
	)
	body += tool.Baro_card(
		tool.Baro_t("프로젝트 변화"),
		tool.Baro_table([]string{"프로젝트", tool.Baro_t("과제"), "상태", "시각"}, project_rows, tool.Baro_t("주시 중인 프로젝트에 변화가 없습니다.")),
	)
	body += tool.Baro_card(
		tool.Baro_t("분야 기여"),
		tool.Baro_table([]string{"분야", tool.Baro_t("기여자"), "종류", "문서", "시각"}, field_rows, tool.Baro_t("주시 중인 분야에 기여가 없습니다.")),
	)

	if len(watches) == 0 {
		body += tool.Baro_empty(tool.Baro_t("아직 주시하는 대상이 없습니다. 문서나 프로젝트 화면에서 주시를 추가해 보세요."))
	}

	body += tool.Baro_note(tool.Baro_t("일일 요약으로 설정한 항목은 즉시 알림 대신 이 화면에 모입니다."))

	return tool.Baro_page(db, config, tool.Baro_t("주시 변경 모아 보기"), body, baro_community_menu())
}

// baro_community_watch_hint : 다른 화면에서 쓸 주시 추가 링크 (문서용)
func baro_community_watch_hint(title string) string {
	return tool.Baro_button(
		"/baro/watch/add?kind=document&target="+tool.Url_parser(strings.TrimSpace(title)),
		tool.Baro_t("이 문서 주시"),
		"",
	)
}
