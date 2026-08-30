package route

import (
	"database/sql"
	"net/url"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 허브 홈 (/baro)
//
//  로그인 여부에 따라 다른 화면을 보여준다.
//   - 비로그인 : 창립 시즌 요약 · 가입 유도 · 오늘의 문서 · 최근 생성 문서
//   - 로그인   : 내 기여 요약 · 추천 과제 · 관심 문서 변경 · 읽지 않은 알림
//  두 경우 모두 사이트 전체 지표를 함께 보여준다.
// ============================================================

// View_baro_home_index : 바로위키 홈
func View_baro_home_index(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	body := baro_home_notice(query)

	if tool.Baro_read_only(db) {
		body += tool.Baro_alert("warn", tool.HTML_escape(tool.Baro_t("읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db)))
	}

	if tool.Baro_is_user(config.IP) {
		body += baro_home_index_user(db, config)
	} else {
		body += baro_home_index_guest(db, config)
	}

	body += baro_home_index_metrics(db)
	body += baro_home_index_recent(db, query)
	body += baro_home_index_shortcut(db, config)

	return tool.Baro_page(db, config, baro_home_wiki_name(db), body, baro_home_menu())
}

// ------------------------------------------------------------
//  비로그인 화면
// ------------------------------------------------------------

// baro_home_index_guest : 창립 시즌 요약 + 가입 유도 + 오늘의 문서
func baro_home_index_guest(db *sql.DB, config tool.Config) string {
	name := baro_home_wiki_name(db)

	confirmed := baro_home_founder_confirmed(db)
	goal := baro_home_founding_goal(db)

	buttons := tool.Baro_button("/baro/founding", tool.Baro_t("창립 시즌 보기"), "primary") + " " +
		tool.Baro_button("/register", tool.Baro_t("가입하기"), "") + " " +
		tool.Baro_button("/login", tool.Baro_t("로그인"), "")

	html := tool.Baro_hero(
		name,
		tool.Baro_t("출처를 붙여 제대로 남기는 위키. 지금은 창립 시즌입니다."),
		`<p>`+tool.HTML_escape(tool.Baro_t("창립 기여자 ")+tool.Baro_comma(confirmed)+" / "+tool.Baro_comma(goal)+tool.Baro_t("명이 모였습니다."))+`</p>`+
			tool.Baro_progress(confirmed, goal, tool.Baro_t("창립 기여자 모집"))+
			`<div>`+buttons+`</div>`,
	)

	// 창립 시즌 요약
	summary := tool.Baro_t(`<p>바로위키는 문서를 지우는 대신 출처를 붙여 남깁니다. `) +
		tool.Baro_t(`분야마다 남은 몫을 숫자로 공개하고, 첫 기여를 마친 사람에게 창립 번호를 드립니다.</p>`)

	open_fields := []string{}
	counts := baro_home_founder_by_field(db)
	for _, field := range tool.Baro_field_list(db) {
		if !field.RecruitOpen {
			continue
		}

		open_fields = append(open_fields, field.Name+" 분야 "+tool.Baro_itoa(counts[field.Key])+tool.Baro_t("명"))
	}

	if len(open_fields) > 0 {
		summary += tool.Baro_pills(open_fields)
	} else {
		summary += tool.Baro_note(tool.Baro_t("지금 모집 중인 분야가 없습니다."))
	}

	summary += tool.Baro_button("/baro/founding", tool.Baro_t("창립 시즌 자세히 보기"), "") + " " +
		tool.Baro_button("/baro/field", tool.Baro_t("분야 지도 보기"), "")

	// 가입 유도
	join := tool.Baro_t(`<p>계정을 만들면 기여 이력이 사람 단위로 남습니다. 이름 · 실명 · 전화번호는 요구하지 않습니다.</p>`)
	join += tool.Baro_list([]string{
		tool.Baro_t("문서를 쓰거나 출처를 붙이면 창립 번호가 발급됩니다."),
		tool.Baro_t("관심 분야를 고르면 나에게 맞는 할 일을 추천해 드립니다."),
		"내 기여는 " + tool.HTML_escape(tool.Baro_get_def(db, "license", "CC BY-SA 4.0")) + " 로 공개됩니다.",
	})
	join += tool.Baro_button("/register", tool.Baro_t("가입하고 시작하기"), "primary") + " " +
		tool.Baro_button("/baro/help", tool.Baro_t("먼저 도움말 보기"), "")

	html += tool.Baro_grid([]string{
		tool.Baro_card(tool.Baro_t("창립 시즌 요약"), summary),
		tool.Baro_card(tool.Baro_t("지금 참여하기"), join),
	}, 2)

	html += baro_home_index_today(db)

	return html
}

// baro_home_index_today : 오늘의 문서
func baro_home_index_today(db *sql.DB) string {
	title, excerpt := baro_home_pick_doc(db, tool.Baro_today())

	if title == "" {
		return tool.Baro_card(
			tool.Baro_t("오늘의 문서"),
			tool.Baro_empty(tool.Baro_t("아직 소개할 문서가 없습니다. 첫 문서를 기다리고 있습니다.")),
		)
	}

	body := `<h3 class="bw_sec_t">` + tool.Baro_doc_link(title) + `</h3>`
	if excerpt != "" {
		body += `<p>` + tool.HTML_escape(excerpt) + `</p>`
	} else {
		body += tool.Baro_note(tool.Baro_t("본문이 아직 짧아 발췌를 만들지 못했습니다."))
	}

	if status := tool.Baro_doc_status_get(db, title); status != "" {
		body += `<p>` + tool.Baro_status_badge(status) + `</p>`
	}

	citation := baro_home_count(db, "select count(*) from baro_citation where title = ?", title)
	body += tool.Baro_kv_grid([][]string{
		{tool.Baro_t("달린 출처"), tool.Baro_comma(citation) + tool.Baro_t("개")},
		{"분야", tool.HTML_escape(tool.Baro_field_name(db, tool.Baro_doc_field(db, title)))},
	})

	body += tool.Baro_button("/w/"+tool.Url_parser(title), "문서 읽기", "primary") + " " +
		tool.Baro_button("/baro/discover", tool.Baro_t("다른 문서 발견하기"), "")

	return tool.Baro_card_ex(tool.Baro_t("오늘의 문서"), tool.Baro_t("매일 한 편씩 소개합니다."), body, "")
}

// ------------------------------------------------------------
//  로그인 화면
// ------------------------------------------------------------

// baro_home_index_user : 내 기여 요약 · 추천 과제 · 관심 문서 · 알림
func baro_home_index_user(db *sql.DB, config tool.Config) string {
	user := config.IP

	event_count := baro_home_count(db, "select count(*) from baro_contrib_event where user = ?", user)
	badge_count := baro_home_count(db, "select count(*) from baro_user_badge where user = ?", user)
	notify_count := tool.Baro_notify_count(db, user)
	number := tool.Baro_founder_number(db, user)

	number_text := tool.Baro_t("미발급")
	if number != "" {
		number_text = "#" + number
	}

	week_events := baro_home_count(
		db,
		"select count(*) from baro_contrib_event where user = ? and date >= ?",
		user, baro_home_week_ago(),
	)

	hero_buttons := tool.Baro_button("/baro/profile/"+tool.Url_parser(user), "내 기여 자세히 보기", "primary") + " " +
		tool.Baro_button("/baro/task", tool.Baro_t("할 일 찾기"), "") + " " +
		tool.Baro_button("/baro/notify", tool.Baro_t("알림 ")+tool.Baro_itoa(notify_count)+tool.Baro_t("건"), "")

	html := tool.Baro_hero(
		tool.Baro_t("어서 오세요, ")+user+tool.Baro_t("님"),
		tool.Baro_t("오늘도 기록을 한 줄 늘려 봅시다."),
		tool.Baro_stat_row([][]string{
			{"기여 기록", tool.Baro_comma(event_count) + tool.Baro_t("건"), tool.Baro_t("전체 누적")},
			{tool.Baro_t("이번 주 기여"), tool.Baro_comma(week_events) + tool.Baro_t("건"), tool.Baro_t("최근 7일")},
			{"배지", tool.Baro_comma(badge_count) + tool.Baro_t("개"), tool.Baro_t("받은 배지")},
			{tool.Baro_t("창립 번호"), number_text, tool.Baro_t("창립 시즌 참여")},
			{tool.Baro_t("읽지 않은 알림"), tool.Baro_comma(notify_count) + tool.Baro_t("건"), tool.Baro_t("확인이 필요합니다")},
		})+`<div>`+hero_buttons+`</div>`,
	)

	if number == "" && tool.Baro_get_on(db, "founding_open") {
		html += tool.Baro_alert("info",
			tool.Baro_t("아직 창립 번호가 없습니다. 첫 유효 기여를 마치면 번호가 발급됩니다. ")+
				tool.Baro_button("/baro/founding/apply", "창립 기여자 지원", "primary"))
	}

	html += tool.Baro_grid([]string{
		baro_home_index_tasks(db, config),
		baro_home_index_watch(db, config),
	}, 2)

	html += baro_home_index_notify(db, config)
	html += baro_home_index_today(db)

	return html
}

// baro_home_index_tasks : 관심 분야에 맞는 추천 과제
func baro_home_index_tasks(db *sql.DB, config tool.Config) string {
	interests := tool.Baro_user_interest(db, config.IP)

	tasks := baro_home_tasks_for_fields(db, interests, 8)
	matched := len(interests) > 0

	if len(tasks) == 0 && matched {
		// 관심 분야에 맞는 과제가 없으면 분야를 가리지 않고 다시 찾는다
		tasks = baro_home_tasks_for_fields(db, nil, 8)
		matched = false
	}

	rows := [][]string{}
	for _, task := range tasks {
		title := tool.HTML_escape(tool.Baro_trunc(task[1], 60))
		if task[0] != "" {
			title = `<a href="/baro/task/` + tool.Url_parser(task[0]) + `">` + title + `</a>`
		}

		est := "-"
		if task[6] != "" {
			est = tool.HTML_escape(task[6])
		}

		rows = append(rows, []string{
			title,
			tool.HTML_escape(tool.Baro_field_name(db, task[4])),
			baro_home_difficulty_badge(task[3]),
			est,
		})
	}

	body := ""
	if len(interests) == 0 {
		body += tool.Baro_alert("info",
			tool.Baro_t("관심 분야를 아직 고르지 않았습니다. ")+tool.Baro_button("/baro/onboarding", tool.Baro_t("관심 분야 고르기"), ""))
	} else {
		names := []string{}
		for _, field := range interests {
			names = append(names, tool.Baro_field_name(db, field))
		}

		body += tool.Baro_pills(names)

		if !matched && len(tasks) > 0 {
			body += tool.Baro_note(tool.Baro_t("관심 분야에 열린 과제가 없어 전체 과제에서 골랐습니다."))
		}
	}

	body += tool.Baro_table(
		[]string{tool.Baro_t("과제"), "분야", tool.Baro_t("난이도"), tool.Baro_t("예상 시간")},
		rows,
		tool.Baro_t("지금 열려 있는 과제가 없습니다. 프로젝트에서 새 과제가 열리면 이곳에 표시됩니다."),
	)

	body += tool.Baro_button("/baro/task", tool.Baro_t("할 일 전체 보기"), "") + " " +
		tool.Baro_button("/baro/project", tool.Baro_t("프로젝트 둘러보기"), "")

	return tool.Baro_card(tool.Baro_t("나에게 맞는 할 일"), body)
}

// baro_home_index_watch : 관심 문서의 최근 변경
func baro_home_index_watch(db *sql.DB, config tool.Config) string {
	changes := baro_home_watch_changes(db, config.IP, 8)

	rows := [][]string{}
	for _, change := range changes {
		summary := tool.Baro_trunc(tool.Baro_one_line(change[4]), 40)
		if summary == "" {
			summary = "-"
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(change[0]),
			`r` + tool.HTML_escape(change[1]),
			tool.Baro_time_cell(change[2]),
			tool.Baro_user_link(db, change[3], config.IP),
			tool.HTML_escape(summary),
		})
	}

	body := tool.Baro_table(
		[]string{"문서", "판", "시각", tool.Baro_t("편집자"), tool.Baro_t("요약")},
		rows,
		tool.Baro_t("관심 문서가 없거나 아직 변경이 없습니다. 문서에서 관심 등록을 누르면 이곳에 모입니다."),
	)

	total := baro_home_count(db, "select count(*) from baro_watch where user = ?", config.IP)
	body += tool.Baro_note(tool.Baro_t("관심 목록에 ") + tool.Baro_comma(total) + tool.Baro_t("건이 등록돼 있습니다."))
	body += tool.Baro_button("/baro/profile/"+tool.Url_parser(config.IP), "내 관심 목록", "")

	return tool.Baro_card(tool.Baro_t("관심 문서 변경"), body)
}

// baro_home_index_notify : 읽지 않은 알림 요약
func baro_home_index_notify(db *sql.DB, config tool.Config) string {
	count := tool.Baro_notify_count(db, config.IP)

	rows := [][]string{}

	list_rows := tool.Query_DB(
		db,
		"select kind, title, url, date from baro_notify where user = ? and readme = '' order by id + 0 desc limit 6",
		config.IP,
	)
	defer list_rows.Close()

	for list_rows.Next() {
		kind, title, target, date := "", "", "", ""
		if err := list_rows.Scan(&kind, &title, &target, &date); err != nil {
			continue
		}

		kind_name := kind
		for _, row := range tool.Baro_notify_kind_list() {
			if row[0] == kind {
				kind_name = row[1]

				break
			}
		}

		title_html := tool.HTML_escape(tool.Baro_trunc(title, 70))
		if target != "" {
			title_html = `<a href="` + tool.HTML_escape(target) + `">` + title_html + `</a>`
		}

		rows = append(rows, []string{
			tool.Baro_badge(kind_name, "info"),
			title_html,
			tool.Baro_time_cell(date),
		})
	}

	body := tool.Baro_table(
		[]string{"종류", tool.Baro_t("내용"), "시각"},
		rows,
		tool.Baro_t("읽지 않은 알림이 없습니다."),
	)

	body += tool.Baro_button("/baro/notify", tool.Baro_t("알림 전체 보기 (")+tool.Baro_itoa(count)+")", "")

	return tool.Baro_card(tool.Baro_t("읽지 않은 알림"), body)
}

// ------------------------------------------------------------
//  공통 구역
// ------------------------------------------------------------

// baro_home_index_metrics : 사이트 전체 지표
func baro_home_index_metrics(db *sql.DB) string {
	since := baro_home_week_ago()

	doc_total := baro_home_doc_total(db)
	week_user := baro_home_count(
		db,
		"select count(distinct user) from baro_contrib_event where date >= ? and user != ''",
		since,
	)
	week_doc := baro_home_new_doc_count(db, since)
	week_citation := baro_home_count(db, "select count(*) from baro_citation where date >= ?", since)
	citation_total := baro_home_count(db, "select count(*) from baro_citation")

	body := tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 문서"), tool.Baro_comma(doc_total) + tool.Baro_t("개"), tool.Baro_t("일반 이름공간 기준")},
		{tool.Baro_t("이번 주 기여자"), tool.Baro_comma(week_user) + tool.Baro_t("명"), tool.Baro_t("최근 7일 유효 기여")},
		{tool.Baro_t("이번 주 새 문서"), tool.Baro_comma(week_doc) + tool.Baro_t("개"), tool.Baro_t("최근 7일")},
		{tool.Baro_t("이번 주 새 출처"), tool.Baro_comma(week_citation) + tool.Baro_t("개"), tool.Baro_t("누적 ") + tool.Baro_comma(citation_total) + tool.Baro_t("개")},
	})

	body += tool.Baro_note(tool.Baro_t("최근 7일(") + tool.Baro_date_only(since) + tool.Baro_t(" 이후) 기준으로 셉니다. 편집 횟수가 아니라 기여 종류로 집계합니다."))
	body += tool.Baro_button("/baro/metrics", tool.Baro_t("지표 자세히 보기"), "") + " " +
		tool.Baro_button("/baro/transparency", tool.Baro_t("투명성 보고서"), "")

	return tool.Baro_card(tool.Baro_t("사이트 지표"), body)
}

// baro_home_index_recent : 최근 생성된 문서 (쪽 넘기기 포함)
func baro_home_index_recent(db *sql.DB, query url.Values) string {
	page := baro_home_page_num(query, "page")
	size := 10

	// 한 개 더 읽어 두면 "다음" 단추를 헛되이 띄우지 않는다
	docs := baro_home_recent_docs(db, (page-1)*size, size+1, true)

	more := len(docs) > size
	if more {
		docs = docs[:size]
	}

	rows := [][]string{}
	for _, doc := range docs {
		excerpt := doc[2]
		if excerpt == "" {
			excerpt = "-"
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(doc[0]),
			tool.HTML_escape(tool.Baro_trunc(excerpt, 80)),
			tool.Baro_status_badge(tool.Baro_doc_status_get(db, doc[0])),
			tool.Baro_time_cell(doc[1]),
		})
	}

	body := tool.Baro_table(
		[]string{"문서", tool.Baro_t("앞부분"), "상태", tool.Baro_t("만든 시각")},
		rows,
		tool.Baro_t("아직 만들어진 문서가 없습니다."),
	)

	if len(rows) == 0 && page > 1 {
		body += tool.Baro_empty(tool.Baro_t("이 쪽에는 문서가 없습니다. 앞 쪽으로 돌아가 주세요."))
	}

	count := 0
	if more {
		count = size
	}

	body += tool.Baro_paging(page, count, size, "/baro?page={}")

	return tool.Baro_card(tool.Baro_t("최근 생성된 문서"), body)
}

// baro_home_index_shortcut : 주요 화면 바로가기
func baro_home_index_shortcut(db *sql.DB, config tool.Config) string {
	items := [][]string{
		{"/baro/field", "분야", tool.Baro_t("분야별 개척률과 남은 몫")},
		{"/baro/project", "프로젝트", tool.Baro_t("함께 채우는 목표")},
		{"/baro/task", tool.Baro_t("할 일"), tool.Baro_t("지금 열려 있는 과제")},
		{"/baro/rank", tool.Baro_t("기여"), tool.Baro_t("기여 기록과 순위")},
		{"/baro/discover", tool.Baro_t("발견"), tool.Baro_t("읽을 만한 문서 찾기")},
		{"/baro/search", tool.Baro_t("검색"), tool.Baro_t("문서 찾기")},
		{"/baro/log", tool.Baro_t("공개 운영 로그"), tool.Baro_t("무엇을 왜 했는지")},
		{"/baro/policy", "정책", tool.Baro_t("규정 전문과 개정 이력")},
	}

	if tool.Baro_can(db, config.IP, "review") {
		items = append(items, []string{"/baro/review", tool.Baro_t("검토 큐"), tool.Baro_t("검토가 필요한 편집")})
	}

	if tool.Baro_is_admin(db, config.IP) {
		items = append(items, []string{"/baro/admin", tool.Baro_t("백오피스"), tool.Baro_t("운영 도구 모음")})
	}

	cards := []string{}
	for _, item := range items {
		cards = append(cards, tool.Baro_card(item[1],
			tool.Baro_note(item[2])+tool.Baro_button(item[0], tool.Baro_t("열기"), "")))
	}

	body := tool.Baro_grid(cards, 4)
	body += tool.Baro_button("/baro/map", tool.Baro_t("화면 지도 전체 보기"), "primary") + " " +
		tool.Baro_button("/baro/about", tool.Baro_t("바로위키 소개"), "") + " " +
		tool.Baro_button("/baro/help", tool.Baro_t("도움말"), "")

	return tool.Baro_card(tool.Baro_t("바로가기"), body)
}
