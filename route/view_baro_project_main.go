package route

import (
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  6-4 프로젝트 생성 / 목록 / 상세
// ============================================================

// View_baro_project_list : 프로젝트 목록 (/baro/project)
func View_baro_project_list(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	field := query.Get("field")
	status := query.Get("status")
	keyword := strings.TrimSpace(query.Get("q"))
	page := baro_project_page_num(query)

	list := baro_project_list_load(db, field, status, keyword, (page-1)*baro_project_page_size, baro_project_page_size)

	body := baro_project_tabs("/baro/project")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", "프로젝트"}})
	body += tool.Baro_hero(
		"프로젝트",
		tool.Baro_t("혼자 쓰면 문서 하나, 같이 쓰면 분야 하나. 목표를 정해 함께 채웁니다."),
		tool.Baro_button("/baro/project/new", tool.Baro_t("프로젝트 만들기"), "primary")+" "+
			tool.Baro_button("/baro/field", tool.Baro_t("분야 허브 보기"), ""),
	)

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 프로젝트"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project")), ""},
		{tool.Baro_t("진행 중"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project where status = 'active' or status = 'open'")), ""},
		{tool.Baro_t("목표 달성"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project where status = 'done'")), ""},
		{tool.Baro_t("참여자 기록"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_member where status = 'active'")), tool.Baro_t("역할이 배정된 사람")},
	})

	filter := tool.Baro_select("field", "분야", baro_project_field_options(db, tool.Baro_t("전체 분야")), field, "")
	filter += tool.Baro_select("status", "상태", append([][]string{{"", tool.Baro_t("전체 상태")}}, baro_project_status_list()...), status, "")
	filter += tool.Baro_input("q", tool.Baro_t("이름 검색"), keyword, tool.Baro_t("프로젝트 이름"), "")
	filter += tool.Baro_submit("걸러 보기", "primary") + " " + tool.Baro_button("/baro/project", tool.Baro_t("필터 지우기"), "")

	body += tool.Baro_card(tool.Baro_t("찾기"), `<form class="bw_form" method="get" action="/baro/project">`+filter+`</form>`)

	rows := [][]string{}
	for _, row := range list {
		bar := tool.Baro_progress_open(row.DoneCount, 0, "")
		if row.TargetCount > 0 {
			bar = tool.Baro_progress(row.DoneCount, row.TargetCount, "")
		}

		period := tool.Baro_t(`<span class="bw_muted">기간 미정</span>`)
		if row.StartDate != "" || row.EndDate != "" {
			period = tool.HTML_escape(row.StartDate + " ~ " + row.EndDate)
		}

		rows = append(rows, []string{
			`<a href="/baro/project/view/` + tool.Url_parser(row.ID) + `">` + tool.HTML_escape(row.Name) + `</a>`,
			baro_project_field_link(db, row.Field),
			baro_project_status_badge(row.Status),
			bar,
			tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_member where project = ? and status = 'active'", row.ID)) + tool.Baro_t("명"),
			period,
			tool.Baro_user_link(db, row.Owner, config.IP),
		})
	}

	table := tool.Baro_table(
		[]string{"프로젝트", "분야", "상태", tool.Baro_t("진행률"), "참여자", "기간", "담당자"},
		rows, "",
	)
	if len(list) == 0 {
		table = tool.Baro_empty(tool.Baro_t("조건에 맞는 프로젝트가 없습니다. 새 프로젝트를 만들어 첫 번째가 되어 보세요."))
	}

	body += tool.Baro_card(tool.Baro_t("프로젝트 목록"), table)
	body += tool.Baro_paging(page, len(list), baro_project_page_size, baro_project_query_string("/baro/project", [][]string{
		{"field", field},
		{"status", status},
		{"q", keyword},
	}))

	return tool.Baro_page(db, config, "프로젝트", body, baro_project_menu())
}

// View_baro_project_new : 프로젝트 만들기 (/baro/project/new)
func View_baro_project_new(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if page, need := tool.Baro_login_wall(db, config, "/baro/project/new"); need {
		return page
	}

	if page, blocked := baro_project_readonly_block(db, config); blocked {
		return page
	}

	message := ""

	if values != nil {
		name := strings.TrimSpace(values.Get("name"))

		// 담당자 : 비워 두면 만든 사람이 담당자가 된다.
		owner := strings.TrimSpace(values.Get("owner"))
		if owner == "" {
			owner = config.IP
		}

		owner_ok := owner == config.IP
		if !owner_ok {
			exist := ""
			owner_ok = tool.QueryRow_DB(db, "select id from user_set where name = 'pw' and id = ?", []any{&exist}, owner)
		}

		switch {
		case name == "":
			message = tool.Baro_t("프로젝트 이름을 적어 주세요.")
		case !owner_ok:
			message = "'" + owner + tool.Baro_t("' 계정을 찾지 못했습니다. 담당자는 가입한 계정이어야 합니다. 비워 두면 만든 사람이 담당자가 됩니다.")
		case !tool.Baro_rate_ok(db, "baro_project_new", config.IP, 10, 3600):
			message = tool.Baro_t("프로젝트 등록이 너무 잦습니다. 잠시 뒤에 다시 시도해 주세요.")
		default:
			id := tool.Baro_next_id(db, "baro_project")

			tool.Exec_DB(
				db,
				`insert into baro_project
					(id, name, field, goal, scope, dataset, start_date, end_date, owner, status,
					 target_count, done_count, rule, finish_cond, date, summary, report)
					values (?, ?, ?, ?, ?, ?, ?, ?, ?, 'open', ?, '0', ?, ?, ?, ?, '')`,
				id,
				tool.Baro_trunc(name, 150),
				values.Get("field"),
				tool.Baro_trunc(values.Get("goal"), 2000),
				tool.Baro_trunc(values.Get("scope"), 2000),
				tool.Baro_trunc(values.Get("dataset"), 2000),
				values.Get("start_date"),
				values.Get("end_date"),
				owner,
				tool.Baro_itoa(tool.Str_to_int(values.Get("target_count"))),
				tool.Baro_trunc(values.Get("rule"), 4000),
				tool.Baro_trunc(values.Get("finish_cond"), 2000),
				tool.Baro_now(),
				tool.Baro_trunc(values.Get("summary"), 500),
			)

			baro_project_member_add(db, id, owner, "manager")
			if owner != config.IP {
				baro_project_member_add(db, id, config.IP, "manager")
				tool.Baro_notify(
					db, owner, "project",
					tool.Baro_t("프로젝트 담당자로 지정됐습니다 : ")+tool.Baro_trunc(name, 60),
					"/baro/project/view/"+tool.Url_parser(id), "",
				)
			}

			// 목표를 한 번에 넣었으면 함께 등록한다.
			for _, line := range tool.Baro_split_lines(values.Get("targets")) {
				tool.Exec_DB(
					db,
					"insert into baro_project_target (id, project, name, note, status, doc, date, assignee) values (?, ?, ?, '', 'todo', '', ?, '')",
					tool.Baro_next_id(db, "baro_project_target"), id, tool.Baro_trunc(line, 200), tool.Baro_now(),
				)
			}

			tool.Baro_public_log(db, "project_new", config.IP, "project:"+id, tool.Baro_field_name(db, values.Get("field")), tool.Baro_trunc(name, 100))
			tool.Baro_audit(db, config, "baro_project_new", "project:"+id, "", tool.Baro_trunc(name, 150), "프로젝트 생성")
			tool.Baro_metric_bump(db, "baro_project_new", tool.Baro_today(), 1)

			if row, ok := baro_project_get(db, id); ok {
				baro_project_recount(db, row)
			}

			return tool.Get_redirect("/baro/project/view/" + tool.Url_parser(id))
		}
	}

	get := func(name string, def string) string {
		if values != nil {
			return values.Get(name)
		}

		if query.Get(name) != "" {
			return query.Get(name)
		}

		return def
	}

	form := tool.Baro_input("name", tool.Baro_t("프로젝트 이름"), get("name", ""), tool.Baro_t("예: 독립유공자 기록 프로젝트"), tool.Baro_t("무엇을 하는 모임인지 드러나게 적어 주세요."))
	form += tool.Baro_select("field", "분야", baro_project_field_options(db, tool.Baro_t("분야 미지정")), get("field", ""), "")
	form += tool.Baro_textarea("goal", tool.Baro_t("목표"), get("goal", ""), 4, tool.Baro_t("예: 국가 공인 독립유공자 18,000명의 기본 문서를 만든다"), tool.Baro_t("무엇을 얼마나 만들 것인지 적습니다."))
	form += tool.Baro_textarea("scope", tool.Baro_t("범위"), get("scope", ""), 4, tool.Baro_t("예: 1919~1945년 활동 기록이 있는 인물만"), tool.Baro_t("무엇을 포함하고 무엇을 빼는지 적으면 다툼이 줄어듭니다."))
	form += tool.Baro_textarea("dataset", tool.Baro_t("참고 데이터셋"), get("dataset", ""), 3, tool.Baro_t("예: 국가보훈부 공훈전자사료관 공개 목록"), tool.Baro_t("어떤 자료를 근거로 목록을 만들지 적습니다."))
	form += tool.Baro_input("owner", "담당자", get("owner", config.IP), tool.Baro_t("가입한 계정 이름"), tool.Baro_t("비워 두면 만든 사람이 담당자가 됩니다. 담당자는 일정과 분담을 관리하는 콘텐츠 역할이며 이용자 제재 권한은 없습니다."))
	form += tool.Baro_input_type("start_date", tool.Baro_t("시작일"), get("start_date", tool.Baro_today()), "date", "")
	form += tool.Baro_input_type("end_date", tool.Baro_t("종료일"), get("end_date", ""), "date", tool.Baro_t("비워 두면 기간 제한 없음"))
	form += tool.Baro_input_type("target_count", tool.Baro_t("목표 수"), get("target_count", "0"), "number", tool.Baro_t("분모가 확정된 경우에만 적습니다. 모르면 0 으로 두세요."))
	form += tool.Baro_textarea("rule", tool.Baro_t("작업 규칙"), get("rule", ""), 6, tool.Baro_t("예: 출처 없는 서술은 넣지 않는다 / 문서 첫 문단에 생몰년을 쓴다"), tool.Baro_t("참여자가 같은 방식으로 쓰도록 규칙을 정합니다."))
	form += tool.Baro_textarea("finish_cond", tool.Baro_t("완료 조건"), get("finish_cond", ""), 3, tool.Baro_t("예: 목표 목록의 90% 이상이 검토 완료 상태가 되면 종료"), tool.Baro_t("언제 끝났다고 할지 미리 정합니다."))
	form += tool.Baro_input("summary", tool.Baro_t("한 줄 소개"), get("summary", ""), tool.Baro_t("목록에 함께 보일 짧은 소개"), "")
	form += tool.Baro_textarea("targets", tool.Baro_t("목표 목록 (한 줄에 하나)"), get("targets", ""), 8, tool.Baro_t("홍길동\n김철수\n이영희"), tool.Baro_t("지금 넣지 않아도 나중에 목표 관리 화면에서 여러 줄로 한 번에 넣을 수 있습니다."))
	form += tool.Baro_submit(tool.Baro_t("프로젝트 만들기"), "primary") + " " + tool.Baro_button("/baro/project", "취소", "")

	body := baro_project_tabs("/baro/project")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/project", "프로젝트"}, {"", tool.Baro_t("새 프로젝트")}})

	if message != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(message))
	}

	body += tool.Baro_alert("info",
		tool.Baro_t("프로젝트를 만들면 자동으로 <b>프로젝트 운영자</b> 역할을 받습니다. ")+
			tool.Baro_t("운영자는 일정과 분담을 관리하는 콘텐츠 역할일 뿐이며, 이용자 제재 권한은 갖지 않습니다."))
	body += tool.Baro_card(tool.Baro_t("새 프로젝트"), tool.Baro_form("/baro/project/new", form))

	return tool.Baro_page(db, config, tool.Baro_t("프로젝트 만들기"), body, baro_project_menu())
}

// View_baro_project_view : 프로젝트 상세 (/baro/project/view/<id>)
func View_baro_project_view(config tool.Config, id string, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, ok := baro_project_get(db, id)
	if !ok {
		return baro_project_missing(
			db, config, tool.Baro_t("프로젝트를 찾을 수 없음"),
			"프로젝트 "+id+tool.Baro_t(" 번은 없습니다. 목록에서 다시 골라 주세요."),
			"/baro/project", tool.Baro_t("프로젝트 목록으로"),
		)
	}

	row = baro_project_recount(db, row)

	can_manage := baro_project_can_manage(db, config, row)
	my_role := baro_project_member_role(db, row.ID, config.IP)

	body := baro_project_tabs("/baro/project")
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/project", "프로젝트"},
		{"", tool.Baro_trunc(row.Name, 30)},
	})

	buttons := tool.Baro_button("/baro/project/room/"+tool.Url_parser(row.ID), "공동 작업실", "primary")
	buttons += " " + tool.Baro_button("/baro/project/target/"+tool.Url_parser(row.ID), "목표 목록", "")
	buttons += " " + tool.Baro_button("/baro/project/source/"+tool.Url_parser(row.ID), "자료 모음", "")
	buttons += " " + tool.Baro_button("/baro/project/progress/"+tool.Url_parser(row.ID), "진행 기록", "")
	buttons += " " + tool.Baro_button("/baro/project/member/"+tool.Url_parser(row.ID), "역할", "")
	buttons += " " + tool.Baro_button("/baro/project/report/"+tool.Url_parser(row.ID), "결과 보고서", "")
	if can_manage {
		buttons += " " + tool.Baro_button("/baro/project/edit/"+tool.Url_parser(row.ID), "설정 고치기", "")
	}

	body += tool.Baro_hero(
		row.Name,
		row.Summary,
		baro_project_status_badge(row.Status)+" "+
			tool.Baro_badge(tool.Baro_field_name(db, row.Field), "info")+
			`<div style="margin-top:.7rem">`+buttons+`</div>`,
	)

	// 진행률
	progress_body := baro_project_progress_bar(db, row, tool.Baro_t("진행률"))
	if row.TargetCount <= 0 {
		progress_body += tool.Baro_note(tool.Baro_t("목표 수가 정해지지 않아 완료 수만 표시합니다. 설정에서 목표 수를 정하면 진행률 막대가 나타납니다."))
	}
	progress_body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("목표 수"), tool.Baro_comma(row.TargetCount), "target_count"},
		{tool.Baro_t("완료 수"), tool.Baro_comma(row.DoneCount), "done_count"},
		{tool.Baro_t("남은 수"), tool.Baro_comma(func() int {
			left := row.TargetCount - row.DoneCount
			if left < 0 {
				return 0
			}

			return left
		}()), ""},
		{"참여자", tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_member where project = ? and status = 'active'", row.ID)), tool.Baro_t("역할 배정")},
		{tool.Baro_t("열린 과제"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_task where project = ? and (status = '' or status = 'open' or status = 'active')", row.ID)), ""},
		{tool.Baro_t("자료"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_source where project = ?", row.ID)), tool.Baro_t("모아 둔 출처")},
	})

	body += tool.Baro_card(tool.Baro_t("진행률"), progress_body)

	// 참여 / 나가기
	join_body := ""
	if !tool.Baro_is_user(config.IP) {
		join_body = tool.Baro_alert("info", tool.Baro_t("프로젝트에 참여하려면 로그인이 필요합니다. ")+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary"))
	} else if my_role == "" {
		join_body = tool.Baro_form("/baro/project/join/"+tool.Url_parser(row.ID),
			tool.Baro_select("role", tool.Baro_t("맡고 싶은 역할"), baro_project_role_options(), "member", tool.Baro_t("나중에 바꿀 수 있습니다."))+
				tool.Baro_submit(tool.Baro_t("참여하기"), "primary"))
	} else {
		join_body = tool.Baro_alert("ok", tool.Baro_t("현재 역할 : ")+tool.HTML_escape(baro_project_role_name(my_role)))
		join_body += tool.Baro_form("/baro/project/leave/"+tool.Url_parser(row.ID),
			tool.Baro_submit(tool.Baro_t("참여 그만두기"), ""))
	}

	body += tool.Baro_grid([]string{
		tool.Baro_card(tool.Baro_t("참여"), join_body),
		tool.Baro_card(tool.Baro_t("기본 정보"), tool.Baro_dl([][]string{
			{"분야", baro_project_field_link(db, row.Field)},
			{"담당자", tool.Baro_user_link(db, row.Owner, config.IP)},
			{"기간", tool.HTML_escape(func() string {
				if row.StartDate == "" && row.EndDate == "" {
					return tool.Baro_t("정해지지 않음")
				}

				return row.StartDate + " ~ " + row.EndDate
			}())},
			{tool.Baro_t("만든 날"), tool.Baro_time_cell(row.Date)},
			{"상태", baro_project_status_badge(row.Status)},
		})),
	}, 2)

	// 목표 / 범위 / 데이터셋 / 규칙 / 완료 조건
	text_card := func(title string, text string, empty string) string {
		if strings.TrimSpace(text) == "" {
			return tool.Baro_card(title, tool.Baro_empty(empty))
		}

		return tool.Baro_card(title, tool.Baro_code(text))
	}

	body += tool.Baro_grid([]string{
		text_card(tool.Baro_t("목표"), row.Goal, tool.Baro_t("목표가 적혀 있지 않습니다.")),
		text_card(tool.Baro_t("범위"), row.Scope, tool.Baro_t("범위가 적혀 있지 않습니다.")),
	}, 2)
	body += tool.Baro_grid([]string{
		text_card(tool.Baro_t("참고 데이터셋"), row.Dataset, tool.Baro_t("참고 데이터셋이 적혀 있지 않습니다.")),
		text_card(tool.Baro_t("완료 조건"), row.FinishCond, tool.Baro_t("완료 조건이 적혀 있지 않습니다.")),
	}, 2)
	body += text_card(tool.Baro_t("작업 규칙"), row.Rule, tool.Baro_t("작업 규칙이 적혀 있지 않습니다. 규칙을 정해 두면 문서 모양이 들쭉날쭉해지지 않습니다."))

	// 목표 목록 (앞부분)
	targets := baro_project_target_list(db, row.ID, "", 0, 15)
	target_rows := [][]string{}
	for _, target := range targets {
		status_badge := tool.Baro_badge(tool.Baro_t("미작성"), "warn")
		if target.Status == "done" {
			status_badge = tool.Baro_badge(tool.Baro_t("완료"), "ok")
		} else if target.Status == "hold" {
			status_badge = tool.Baro_badge(tool.Baro_t("보류"), "muted")
		}

		doc := `<span class="bw_muted">-</span>`
		if target.Doc != "" {
			doc = tool.Baro_doc_link(target.Doc)
		}

		assignee := `<span class="bw_muted">-</span>`
		if target.Assignee != "" {
			assignee = tool.Baro_user_link(db, target.Assignee, config.IP)
		}

		target_rows = append(target_rows, []string{
			tool.HTML_escape(target.Name),
			status_badge,
			doc,
			assignee,
			tool.HTML_escape(tool.Baro_trunc(target.Note, 40)),
		})
	}

	body += tool.Baro_card(
		"목표 목록",
		tool.Baro_table([]string{tool.Baro_t("이름"), "상태", "문서", tool.Baro_t("담당"), tool.Baro_t("메모")}, target_rows, tool.Baro_t("등록된 목표가 없습니다."))+
			tool.Baro_button("/baro/project/target/"+tool.Url_parser(row.ID), "목표 목록 관리", "primary"),
	)

	// 과제 목록
	tasks := baro_project_task_load(db, []string{"project = ?"}, []any{row.ID}, 0, 15)
	task_rows := [][]string{}
	for _, task := range tasks {
		task_rows = append(task_rows, baro_project_task_row_cells(db, config, task))
	}

	body += tool.Baro_card(
		tool.Baro_t("과제 목록"),
		tool.Baro_table(baro_project_task_headers(), task_rows, tool.Baro_t("이 프로젝트에 등록된 과제가 없습니다."))+
			tool.Baro_button("/baro/task/new?project="+tool.Url_parser(row.ID)+"&field="+url.QueryEscape(row.Field), "이 프로젝트에 과제 등록", ""),
	)

	// 자료 모음 (앞부분)
	sources := baro_project_source_list(db, row.ID, 0, 8)
	source_rows := [][]string{}
	for _, source := range sources {
		source_rows = append(source_rows, []string{
			tool.HTML_escape(source[1]),
			baro_project_link_html(source[2]),
			tool.Baro_user_link(db, source[4], config.IP),
			tool.Baro_time_cell(source[5]),
		})
	}

	body += tool.Baro_card(
		"자료 모음",
		tool.Baro_table([]string{tool.Baro_t("자료"), tool.Baro_t("주소"), tool.Baro_t("올린 사람"), tool.Baro_t("등록")}, source_rows, tool.Baro_t("모아 둔 자료가 없습니다."))+
			tool.Baro_button("/baro/project/source/"+tool.Url_parser(row.ID), "자료 모음 열기", ""),
	)

	// 참여자
	members := baro_project_member_list(db, row.ID)
	member_rows := [][]string{}
	for _, member := range members {
		if member.Status != "active" {
			continue
		}

		member_rows = append(member_rows, []string{
			tool.Baro_user_link(db, member.User, config.IP),
			tool.HTML_escape(baro_project_role_name(member.Role)),
			tool.Baro_time_cell(member.Date),
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("참여자와 역할"),
		tool.Baro_table([]string{tool.Baro_t("사용자"), "역할", tool.Baro_t("참여")}, member_rows, tool.Baro_t("참여자가 없습니다."))+
			tool.Baro_note(tool.Baro_t("프로젝트 역할은 콘텐츠 역할입니다. 어떤 역할도 사이트 전체 이용자를 차단할 권한을 갖지 않습니다."))+
			tool.Baro_button("/baro/project/member/"+tool.Url_parser(row.ID), "역할 관리", ""),
	)

	// 최근 활동
	recent := [][]string{}
	activity := tool.Query_DB(
		db,
		`select coalesce(user, ''), coalesce(kind, ''), coalesce(title, ''), coalesce(date, '')
			from baro_contrib_event where project = ? order by id + 0 desc limit 15`,
		row.ID,
	)
	for activity.Next() {
		user, kind, title, date := "", "", "", ""
		if err := activity.Scan(&user, &kind, &title, &date); err != nil {
			continue
		}

		recent = append(recent, []string{
			tool.Baro_user_link(db, user, config.IP),
			tool.HTML_escape(tool.Baro_contrib_kind_name(kind)),
			tool.Baro_doc_link(title),
			tool.Baro_time_cell(date),
		})
	}
	activity.Close()

	body += tool.Baro_card(tool.Baro_t("최근 활동"), tool.Baro_table([]string{tool.Baro_t("사용자"), tool.Baro_t("한 일"), "문서", "시각"}, recent, tool.Baro_t("아직 활동 기록이 없습니다.")))

	// 토론 / 보고서 링크
	discussion := tool.Baro_t("프로젝트:") + row.Name
	body += tool.Baro_card(
		tool.Baro_t("토론과 보고서"),
		tool.Baro_kv_grid([][]string{
			{"프로젝트 토론", `<a href="/thread/` + tool.Url_parser(discussion) + `">` + tool.HTML_escape(discussion) + tool.Baro_t(` 토론 열기</a>`)},
			{tool.Baro_t("프로젝트 문서"), tool.Baro_doc_link(discussion)},
			{"결과 보고서", `<a href="/baro/project/report/` + tool.Url_parser(row.ID) + tool.Baro_t(`">보고서 보기</a>`)},
		})+
			tool.Baro_note(tool.Baro_t("토론은 위키 본체의 토론 기능을 그대로 씁니다. 프로젝트 문서를 만들어 두면 토론이 한 곳에 모입니다.")),
	)

	return tool.Baro_page(db, config, tool.Baro_t("프로젝트 : ")+tool.Baro_trunc(row.Name, 40), body, baro_project_menu())
}

// View_baro_project_edit : 프로젝트 설정 고치기 (/baro/project/edit/<id>)
func View_baro_project_edit(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, ok := baro_project_get(db, id)
	if !ok {
		return baro_project_missing(
			db, config, tool.Baro_t("프로젝트를 찾을 수 없음"),
			"프로젝트 "+id+tool.Baro_t(" 번은 없습니다."),
			"/baro/project", tool.Baro_t("프로젝트 목록으로"),
		)
	}

	if page, need := tool.Baro_login_wall(db, config, "/baro/project/edit/"+id); need {
		return page
	}

	if !baro_project_can_manage(db, config, row) {
		return tool.Baro_deny(db, config, tool.Baro_t("이 프로젝트의 설정을 고칠 권한이 없습니다. 프로젝트 담당자나 운영자에게 요청해 주세요."))
	}

	message := ""

	if values != nil {
		if page, blocked := baro_project_readonly_block(db, config); blocked {
			return page
		}

		name := strings.TrimSpace(values.Get("name"))

		owner := strings.TrimSpace(values.Get("owner"))
		if owner == "" {
			owner = row.Owner
		}

		owner_ok := owner == row.Owner || owner == config.IP
		if !owner_ok {
			exist := ""
			owner_ok = tool.QueryRow_DB(db, "select id from user_set where name = 'pw' and id = ?", []any{&exist}, owner)
		}

		if name == "" {
			message = tool.Baro_t("프로젝트 이름을 적어 주세요.")
		} else if !owner_ok {
			message = "'" + owner + tool.Baro_t("' 계정을 찾지 못했습니다. 담당자는 가입한 계정이어야 합니다.")
		} else {
			before := tool.Baro_json_enc(map[string]string{
				"name":   row.Name,
				"field":  row.Field,
				"status": row.Status,
				"goal":   tool.Baro_trunc(row.Goal, 200),
			})

			tool.Exec_DB(
				db,
				`update baro_project set name = ?, field = ?, goal = ?, scope = ?, dataset = ?,
					start_date = ?, end_date = ?, status = ?, target_count = ?, rule = ?, finish_cond = ?, summary = ?, owner = ?
					where id = ?`,
				tool.Baro_trunc(name, 150),
				values.Get("field"),
				tool.Baro_trunc(values.Get("goal"), 2000),
				tool.Baro_trunc(values.Get("scope"), 2000),
				tool.Baro_trunc(values.Get("dataset"), 2000),
				values.Get("start_date"),
				values.Get("end_date"),
				values.Get("status"),
				tool.Baro_itoa(tool.Str_to_int(values.Get("target_count"))),
				tool.Baro_trunc(values.Get("rule"), 4000),
				tool.Baro_trunc(values.Get("finish_cond"), 2000),
				tool.Baro_trunc(values.Get("summary"), 500),
				owner,
				row.ID,
			)

			// 담당자를 넘겼으면 새 담당자를 운영자로 넣고 알린다.
			if owner != row.Owner {
				baro_project_member_add(db, row.ID, owner, "manager")
				tool.Baro_notify(
					db, owner, "project",
					tool.Baro_t("프로젝트 담당자로 지정됐습니다 : ")+tool.Baro_trunc(name, 60),
					"/baro/project/view/"+tool.Url_parser(row.ID), "",
				)
			}

			after := tool.Baro_json_enc(map[string]string{
				"name":   name,
				"field":  values.Get("field"),
				"status": values.Get("status"),
				"goal":   tool.Baro_trunc(values.Get("goal"), 200),
			})

			tool.Baro_public_log(db, "project_edit", config.IP, "project:"+row.ID, values.Get("reason"), tool.Baro_trunc(name, 100))
			tool.Baro_audit(db, config, "baro_project_edit", "project:"+row.ID, before, after, values.Get("reason"))

			return tool.Get_redirect("/baro/project/view/" + tool.Url_parser(row.ID))
		}
	}

	form := tool.Baro_input("name", tool.Baro_t("프로젝트 이름"), row.Name, "", "")
	form += tool.Baro_select("field", "분야", baro_project_field_options(db, tool.Baro_t("분야 미지정")), row.Field, "")
	form += tool.Baro_select("status", "상태", baro_project_status_list(), row.Status, "")
	form += tool.Baro_textarea("goal", tool.Baro_t("목표"), row.Goal, 4, "", "")
	form += tool.Baro_textarea("scope", tool.Baro_t("범위"), row.Scope, 4, "", "")
	form += tool.Baro_textarea("dataset", tool.Baro_t("참고 데이터셋"), row.Dataset, 3, "", "")
	form += tool.Baro_input("owner", "담당자", row.Owner, tool.Baro_t("가입한 계정 이름"), tool.Baro_t("다른 사람에게 넘기면 그 사람이 프로젝트 운영자가 됩니다. (콘텐츠 역할이며 제재 권한과는 무관합니다)"))
	form += tool.Baro_input_type("start_date", tool.Baro_t("시작일"), row.StartDate, "date", "")
	form += tool.Baro_input_type("end_date", tool.Baro_t("종료일"), row.EndDate, "date", "")
	form += tool.Baro_input_type("target_count", tool.Baro_t("목표 수"), tool.Baro_itoa(row.TargetCount), "number", tool.Baro_t("분모가 확정된 경우에만 적습니다. 0 이면 진행률 대신 완료 수만 보입니다."))
	form += tool.Baro_textarea("rule", tool.Baro_t("작업 규칙"), row.Rule, 6, "", "")
	form += tool.Baro_textarea("finish_cond", tool.Baro_t("완료 조건"), row.FinishCond, 3, "", "")
	form += tool.Baro_input("summary", tool.Baro_t("한 줄 소개"), row.Summary, "", "")
	form += tool.Baro_input("reason", tool.Baro_t("고치는 이유"), "", tool.Baro_t("공개 운영 로그에 남습니다"), "")
	form += tool.Baro_submit("저장", "primary") + " " + tool.Baro_button("/baro/project/view/"+tool.Url_parser(row.ID), "취소", "")

	body := baro_project_tabs("/baro/project")
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/project", "프로젝트"},
		{"/baro/project/view/" + tool.Url_parser(row.ID), tool.Baro_trunc(row.Name, 24)},
		{"", "설정"},
	})

	if message != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(message))
	}

	body += tool.Baro_card("프로젝트 설정", tool.Baro_form("/baro/project/edit/"+tool.Url_parser(row.ID), form))
	body += tool.Baro_danger_zone(
		tool.Baro_t("이 화면에서 할 수 없는 일"),
		tool.Baro_note(tool.Baro_t("프로젝트 설정 화면에서는 이용자 차단·경고 같은 제재를 할 수 없습니다. ")+
			tool.Baro_t("제재는 사건 기록을 거쳐 제재 담당 역할이 따로 처리합니다. 콘텐츠 역할과 제재 권한은 분리되어 있습니다.")),
	)

	return tool.Baro_page(db, config, "프로젝트 설정", body, baro_project_menu())
}

// View_baro_project_join : 프로젝트 참여 (/baro/project/join/<id>)
func View_baro_project_join(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, ok := baro_project_get(db, id)
	if !ok {
		return baro_project_missing(
			db, config, tool.Baro_t("프로젝트를 찾을 수 없음"),
			"프로젝트 "+id+tool.Baro_t(" 번은 없습니다."),
			"/baro/project", tool.Baro_t("프로젝트 목록으로"),
		)
	}

	if page, need := tool.Baro_login_wall(db, config, "/baro/project/join/"+id); need {
		return page
	}

	if values == nil {
		body := baro_project_tabs("/baro/project")
		body += tool.Baro_crumb([][]string{
			{"/baro", "바로위키"},
			{"/baro/project/view/" + tool.Url_parser(row.ID), tool.Baro_trunc(row.Name, 24)},
			{"", tool.Baro_t("참여")},
		})
		body += tool.Baro_card(
			row.Name+tool.Baro_t(" 참여하기"),
			tool.Baro_form("/baro/project/join/"+tool.Url_parser(row.ID),
				tool.Baro_select("role", tool.Baro_t("맡고 싶은 역할"), baro_project_role_options(), "member", tool.Baro_t("나중에 바꿀 수 있습니다."))+
					tool.Baro_submit(tool.Baro_t("참여하기"), "primary")+" "+
					tool.Baro_button("/baro/project/view/"+tool.Url_parser(row.ID), "취소", "")),
		)

		return tool.Baro_page(db, config, "프로젝트 참여", body, baro_project_menu())
	}

	if page, blocked := baro_project_readonly_block(db, config); blocked {
		return page
	}

	role := baro_project_role_norm(values.Get("role"))
	if !baro_project_role_known(role) {
		role = "member"
	}

	baro_project_member_add(db, row.ID, config.IP, role)

	tool.Baro_public_log(db, "project_join", config.IP, "project:"+row.ID, baro_project_role_name(role), tool.Baro_trunc(row.Name, 100))
	tool.Baro_audit(db, config, "baro_project_join", "project:"+row.ID, "", role, "프로젝트 참여")

	if row.Owner != "" && row.Owner != config.IP {
		tool.Baro_notify(
			db, row.Owner, "project",
			tool.Baro_t("프로젝트에 새 참여자가 들어왔습니다 : ")+row.Name,
			"/baro/project/member/"+tool.Url_parser(row.ID), "",
		)
	}

	return tool.Get_redirect("/baro/project/view/" + tool.Url_parser(row.ID))
}

// View_baro_project_leave : 프로젝트 나가기 (/baro/project/leave/<id>)
func View_baro_project_leave(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, ok := baro_project_get(db, id)
	if !ok {
		return baro_project_missing(
			db, config, tool.Baro_t("프로젝트를 찾을 수 없음"),
			"프로젝트 "+id+tool.Baro_t(" 번은 없습니다."),
			"/baro/project", tool.Baro_t("프로젝트 목록으로"),
		)
	}

	if page, need := tool.Baro_login_wall(db, config, "/baro/project/leave/"+id); need {
		return page
	}

	if values == nil {
		body := baro_project_tabs("/baro/project")
		body += tool.Baro_card(
			row.Name+tool.Baro_t(" 참여 그만두기"),
			tool.Baro_note(tool.Baro_t("나가더라도 지금까지의 기여 기록은 그대로 남습니다. 언제든 다시 참여할 수 있습니다."))+
				tool.Baro_form("/baro/project/leave/"+tool.Url_parser(row.ID),
					tool.Baro_submit(tool.Baro_t("참여 그만두기"), "danger")+" "+
						tool.Baro_button("/baro/project/view/"+tool.Url_parser(row.ID), "취소", "")),
		)

		return tool.Baro_page(db, config, tool.Baro_t("프로젝트 나가기"), body, baro_project_menu())
	}

	if page, blocked := baro_project_readonly_block(db, config); blocked {
		return page
	}

	tool.Exec_DB(db, "update baro_project_member set status = 'closed' where project = ? and user = ?", row.ID, config.IP)
	tool.Baro_public_log(db, "project_leave", config.IP, "project:"+row.ID, "", tool.Baro_trunc(row.Name, 100))

	return tool.Get_redirect("/baro/project/view/" + tool.Url_parser(row.ID))
}
