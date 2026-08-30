package route

import (
	"net/url"

	"opennamu/route/tool"
)

// ============================================================
//  6-1 분야 허브 / 6-2 개척률
// ============================================================

// View_baro_project_field_list : 분야 목록 (/baro/field)
func View_baro_project_field_list(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	field_list := tool.Baro_field_list(db)

	body := baro_project_tabs("/baro/field")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("분야 허브")}})
	body += tool.Baro_hero(
		tool.Baro_t("분야 허브"),
		tool.Baro_t("어느 분야가 얼마나 채워졌고 무엇이 비어 있는지 한 눈에 봅니다."),
		"",
	)

	// 전체 합계
	total_projects := baro_project_count(db, "select count(*) from baro_project")
	total_targets := baro_project_count(db, "select count(*) from baro_project_target")
	total_done := baro_project_count(db, "select count(*) from baro_project_target where status = 'done'")
	total_tasks := baro_project_count(
		db,
		"select count(*) from baro_project_task where status = '' or status = 'open' or status = 'active'",
	)
	week_events := baro_project_count(
		db,
		"select count(*) from baro_contrib_event where date >= ?",
		tool.Baro_time_add(tool.Baro_now(), -7, 0, 0),
	)

	body += tool.Baro_stat_row([][]string{
		{"분야", tool.Baro_comma(len(field_list)), tool.Baro_t("등록된 개척 분야")},
		{"프로젝트", tool.Baro_comma(total_projects), tool.Baro_t("진행 중 포함")},
		{tool.Baro_t("등록된 목표"), tool.Baro_comma(total_targets), tool.Baro_t("작성 대상 문서")},
		{tool.Baro_t("작성 완료"), tool.Baro_comma(total_done), tool.Baro_t("목표 중 완료")},
		{tool.Baro_t("열린 과제"), tool.Baro_comma(total_tasks), tool.Baro_t("맡을 수 있는 일")},
		{tool.Baro_t("이번 주 기여"), tool.Baro_comma(week_events), tool.Baro_t("최근 7일")},
	})

	body += tool.Baro_card(
		tool.Baro_t("왜 어떤 분야에는 개척률(%)이 없나요"),
		tool.Baro_t(`<p>개척률은 <b>분모가 분명한 분야</b>에만 붙입니다. 예를 들어 독립운동 분야는
			국가가 공인한 독립유공자 명단이 있어 18,000명 중 4,310명 기록 완료 라고 말할 수 있습니다.</p>
		<p>반대로 게임·애니처럼 <b>전체 목록을 아무도 확정할 수 없는 분야</b>에 개척률 32.4% 라고 쓰면
			근거 없는 정밀도가 됩니다. 그래서 그런 분야에는 기록 수와 프로젝트가 스스로 정한 목표 수만 보여 줍니다.</p>`)+
			tool.Baro_pills([]string{tool.Baro_t("분모 확정 = 개척률 표시"), tool.Baro_t("분모 미확정 = 기록 수만 표시")}),
	)

	closed_cards := []string{}
	open_cards := []string{}

	for _, field := range field_list {
		stat := baro_project_field_stat_get(db, field)

		inner := tool.Baro_note(field.Description)
		inner += baro_project_field_progress_html(field, stat)
		inner += tool.Baro_kv_grid([][]string{
			{tool.Baro_t("기록 완료"), tool.Baro_comma(stat.Recorded) + tool.Baro_t("건")},
			{tool.Baro_t("보강 필요"), tool.Baro_comma(stat.NeedExpand) + tool.Baro_t("건")},
			{tool.Baro_t("출처 부족"), tool.Baro_comma(stat.NeedSource) + tool.Baro_t("건")},
			{tool.Baro_t("검토 완료"), tool.Baro_comma(stat.Reviewed) + tool.Baro_t("건")},
			{tool.Baro_t("이번 주 증가"), "+" + tool.Baro_comma(stat.WeekEvents)},
			{"참여자", tool.Baro_comma(stat.PeopleCount) + tool.Baro_t("명")},
		})
		inner += `<div>` + tool.Baro_button("/baro/field/view/"+tool.Url_parser(field.Key), "분야 허브 열기", "primary") + ` ` +
			tool.Baro_button("/baro/task?field="+url.QueryEscape(field.Key), tool.Baro_t("이 분야 과제"), "") + `</div>`

		kind_badge := tool.Baro_badge(tool.Baro_t("분모 확정"), "ok")
		if field.Kind != "closed" {
			kind_badge = tool.Baro_badge(tool.Baro_t("분모 미확정"), "muted")
		}

		card := tool.Baro_card_ex(
			field.Name,
			tool.Baro_t("프로젝트 ")+tool.Baro_itoa(stat.ProjectCount)+tool.Baro_t("개 · 열린 과제 ")+tool.Baro_itoa(stat.TaskOpen)+tool.Baro_t("개"),
			inner,
			kind_badge,
		)

		if field.Kind == "closed" {
			closed_cards = append(closed_cards, card)
		} else {
			open_cards = append(open_cards, card)
		}
	}

	if len(field_list) == 0 {
		body += tool.Baro_empty(tool.Baro_t("등록된 분야가 없습니다."))
	}

	if len(closed_cards) > 0 {
		body += tool.Baro_section(tool.Baro_t("전체 목록이 확정된 분야 (개척률 표시)"), tool.Baro_grid(closed_cards, 2))
	}
	if len(open_cards) > 0 {
		body += tool.Baro_section(tool.Baro_t("전체 목록이 확정되지 않은 분야 (기록 수 표시)"), tool.Baro_grid(open_cards, 2))
	}

	body += baro_project_open_note()

	return tool.Baro_page(db, config, tool.Baro_t("분야 허브"), body, baro_project_menu())
}

// View_baro_project_field_view : 분야 허브 (/baro/field/view/<분야키>)
func View_baro_project_field_view(config tool.Config, key string, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	field := tool.Baro_field_get(db, key)
	if field.Key == "" {
		return baro_project_missing(
			db, config, tool.Baro_t("분야를 찾을 수 없음"),
			"'"+key+tool.Baro_t("' 분야는 등록되어 있지 않습니다. 분야 목록에서 다시 골라 주세요."),
			"/baro/field", tool.Baro_t("분야 목록으로"),
		)
	}

	stat := baro_project_field_stat_get(db, field)

	body := baro_project_tabs("/baro/field")
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/field", tool.Baro_t("분야 허브")},
		{"", field.Name},
	})

	kind_text := tool.Baro_t("전체 목록이 확정된 분야입니다. 개척률(%)을 표시합니다.")
	if field.Kind != "closed" {
		kind_text = tool.Baro_t("전체 목록이 확정되지 않은 분야입니다. 개척률 대신 기록 수와 목표 수만 표시합니다.")
	}

	body += tool.Baro_hero(field.Name+tool.Baro_t(" 개척 현황"), field.Description, tool.Baro_note(kind_text))

	// --- 6-2 개척률 ---
	progress_body := baro_project_field_progress_html(field, stat)
	if field.Kind == "closed" && field.TargetCount > 0 {
		progress_body += tool.Baro_dl([][]string{
			{tool.Baro_t("공인 목록 규모"), tool.Baro_comma(field.TargetCount) + tool.Baro_t("건")},
			{tool.Baro_t("기록 완료"), tool.Baro_comma(stat.Recorded) + tool.Baro_t("건")},
			{tool.Baro_t("남은 수"), tool.Baro_comma(field.TargetCount-stat.Recorded) + tool.Baro_t("건")},
			{tool.Baro_t("개척률"), tool.Baro_pct_text(stat.Recorded, field.TargetCount)},
		})
	} else {
		progress_body += tool.Baro_dl([][]string{
			{tool.Baro_t("기록된 문서"), tool.Baro_comma(stat.DocCount) + tool.Baro_t("건")},
			{tool.Baro_t("프로젝트 목표 수 합"), tool.Baro_comma(stat.ProjectGoal) + tool.Baro_t("건")},
			{tool.Baro_t("목표 중 완료"), tool.Baro_comma(stat.TargetDone) + " / " + tool.Baro_comma(stat.TargetTotal)},
		})
	}
	progress_body += baro_project_field_count_note()
	progress_body += baro_project_open_note()

	body += tool.Baro_card(tool.Baro_t("개척률"), progress_body)

	// --- 6-1 분야 지표 ---
	target_label := tool.Baro_t("목표 인물·문서 수")
	target_value := tool.Baro_comma(field.TargetCount)
	target_sub := tool.Baro_t("분야 전체 목록 기준")
	if field.TargetCount <= 0 {
		target_value = tool.Baro_comma(stat.ProjectGoal)
		target_sub = tool.Baro_t("소속 프로젝트 목표 수 합")
	}

	body += tool.Baro_stat_row([][]string{
		{target_label, target_value, target_sub},
		{tool.Baro_t("작성 완료"), tool.Baro_comma(stat.Recorded), tool.Baro_t("기록된 문서")},
		{tool.Baro_t("보강 필요"), tool.Baro_comma(stat.NeedExpand), "need_expand"},
		{tool.Baro_t("출처 부족"), tool.Baro_comma(stat.NeedSource), "need_source"},
		{tool.Baro_t("검토 완료"), tool.Baro_comma(stat.Reviewed), "reviewed"},
		{tool.Baro_t("이번 주 증가"), "+" + tool.Baro_comma(stat.WeekEvents), tool.Baro_t("최근 7일 기여")},
	})

	// --- 소속 프로젝트 ---
	project_list := baro_project_list_load(db, field.Key, "", "", 0, 20)
	project_rows := [][]string{}
	for _, row := range project_list {
		bar := tool.Baro_progress_open(row.DoneCount, 0, "")
		if row.TargetCount > 0 {
			bar = tool.Baro_progress(row.DoneCount, row.TargetCount, "")
		}

		project_rows = append(project_rows, []string{
			`<a href="/baro/project/view/` + tool.Url_parser(row.ID) + `">` + tool.HTML_escape(row.Name) + `</a>`,
			baro_project_status_badge(row.Status),
			bar,
			tool.Baro_user_link(db, row.Owner, config.IP),
			tool.Baro_time_cell(row.Date),
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("이 분야의 프로젝트"),
		tool.Baro_table(
			[]string{"프로젝트", "상태", tool.Baro_t("진행"), "담당자", tool.Baro_t("만든 날")},
			project_rows,
			tool.Baro_t("아직 이 분야에 프로젝트가 없습니다."),
		)+
			tool.Baro_button("/baro/project?field="+url.QueryEscape(field.Key), tool.Baro_t("프로젝트 전체 보기"), "")+" "+
			tool.Baro_button("/baro/project/new?field="+url.QueryEscape(field.Key), tool.Baro_t("이 분야에 프로젝트 만들기"), "primary"),
	)

	// --- 미작성 목록 ---
	todo_list := baro_project_field_todo(db, field.Key, 20)
	todo_rows := [][]string{}
	for _, row := range todo_list {
		todo_rows = append(todo_rows, []string{
			tool.HTML_escape(row[1]),
			baro_project_link(db, row[2]),
			tool.HTML_escape(tool.Baro_trunc(row[3], 50)),
			tool.Baro_button("/baro/project/target/"+tool.Url_parser(row[2]), "목표 관리", ""),
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("미작성 목록"),
		tool.Baro_note(tool.Baro_t("아직 문서가 만들어지지 않은 목표입니다. 문서를 만들면 자동으로 완료 표시됩니다."))+
			tool.Baro_table(
				[]string{tool.Baro_t("이름"), "프로젝트", tool.Baro_t("메모"), ""},
				todo_rows,
				tool.Baro_t("미작성으로 등록된 목표가 없습니다."),
			),
	)

	// --- 열린 과제 ---
	task_list := baro_project_task_load(
		db,
		[]string{"field = ?", "(status = '' or status = 'open' or status = 'active')"},
		[]any{field.Key},
		0, 10,
	)
	task_rows := [][]string{}
	for _, row := range task_list {
		task_rows = append(task_rows, baro_project_task_row_cells(db, config, row))
	}

	body += tool.Baro_card(
		tool.Baro_t("이 분야에서 맡을 수 있는 과제"),
		tool.Baro_table(baro_project_task_headers(), task_rows, tool.Baro_t("열린 과제가 없습니다."))+
			tool.Baro_button("/baro/task?field="+url.QueryEscape(field.Key), tool.Baro_t("과제 전체 보기"), "")+" "+
			tool.Baro_button("/baro/task/new?field="+url.QueryEscape(field.Key), "과제 등록", "primary"),
	)

	// --- 참여자 / 주요 기여자 ---
	people := baro_project_field_people(db, field.Key)
	people_html := ""
	if len(people) == 0 {
		people_html = tool.Baro_empty(tool.Baro_t("아직 참여자가 없습니다."))
	} else {
		items := []string{}
		for index, user := range people {
			if index >= 40 {
				items = append(items, tool.Baro_t(`<span class="bw_muted">외 `)+tool.Baro_itoa(len(people)-40)+tool.Baro_t(`명</span>`))
				break
			}

			items = append(items, tool.Baro_user_link(db, user, config.IP))
		}

		people_html = `<div class="bw_pills">`
		for _, item := range items {
			people_html += `<span class="bw_pill">` + item + `</span>`
		}
		people_html += `</div>`
	}

	top_list := baro_project_field_top(db, field.Key, 10)
	top_rows := [][]string{}
	for index, row := range top_list {
		top_rows = append(top_rows, []string{
			tool.Baro_itoa(index + 1),
			tool.Baro_user_link(db, row[0], config.IP),
			tool.Baro_comma(tool.Str_to_int(row[2])),
			tool.Baro_comma(tool.Str_to_int(row[1])) + tool.Baro_t("회"),
		})
	}

	body += tool.Baro_grid([]string{
		tool.Baro_card(tool.Baro_t("참여자 (")+tool.Baro_itoa(len(people))+tool.Baro_t("명)"), people_html+tool.Baro_note(tool.Baro_t("관심 분야로 등록했거나 이 분야 프로젝트에 참여 중인 사람입니다."))),
		tool.Baro_card(
			tool.Baro_t("주요 기여자"),
			tool.Baro_table([]string{"#", tool.Baro_t("사용자"), tool.Baro_t("기여 가치"), tool.Baro_t("기여 수")}, top_rows, tool.Baro_t("아직 기여 기록이 없습니다."))+
				tool.Baro_note(tool.Baro_t("편집 횟수가 아니라 기여 종류별 가치 합으로 줄을 세웁니다.")),
		),
	}, 2)

	// --- 최근 작업 ---
	recent := baro_project_field_recent(db, field.Key, 20)
	recent_rows := [][]string{}
	for _, row := range recent {
		doc := `<span class="bw_muted">-</span>`
		if row[2] != "" {
			doc = tool.Baro_doc_link(row[2])
		}

		project := `<span class="bw_muted">-</span>`
		if row[4] != "" {
			project = baro_project_link(db, row[4])
		}

		recent_rows = append(recent_rows, []string{
			tool.Baro_user_link(db, row[0], config.IP),
			tool.HTML_escape(tool.Baro_contrib_kind_name(row[1])),
			doc,
			project,
			tool.Baro_time_cell(row[3]),
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("최근 작업"),
		tool.Baro_table(
			[]string{tool.Baro_t("사용자"), tool.Baro_t("한 일"), "문서", "프로젝트", "시각"},
			recent_rows,
			tool.Baro_t("최근 작업 기록이 없습니다."),
		),
	)

	if field.RecruitOpen {
		body += tool.Baro_alert("info",
			tool.Baro_t(`이 분야는 함께할 사람을 찾고 있습니다. 모집 인원 `)+tool.Baro_itoa(field.Recruit)+tool.Baro_t(`명 · `)+
				tool.Baro_button("/baro/project?field="+url.QueryEscape(field.Key), tool.Baro_t("참여할 프로젝트 고르기"), "primary"),
		)
	}

	return tool.Baro_page(db, config, field.Name+" 분야", body, baro_project_menu())
}
