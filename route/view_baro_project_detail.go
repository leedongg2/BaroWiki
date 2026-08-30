package route

import (
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  6-4 목표 목록 · 자료 모음 · 진행 기록 · 결과 보고서
//  6-5 프로젝트 역할
// ============================================================

// baro_project_sub_tabs : 프로젝트 안쪽 이동 탭
func baro_project_sub_tabs(id string, active string) string {
	key := tool.Url_parser(id)

	return tool.Baro_tabs([][]string{
		{"/baro/project/view/" + key, tool.Baro_t("개요")},
		{"/baro/project/target/" + key, "목표 목록"},
		{"/baro/project/task/" + key, tool.Baro_t("과제")},
		{"/baro/project/source/" + key, "자료 모음"},
		{"/baro/project/progress/" + key, "진행 기록"},
		{"/baro/project/member/" + key, "역할"},
		{"/baro/project/room/" + key, "공동 작업실"},
		{"/baro/project/report/" + key, "결과 보고서"},
	}, active)
}

// ------------------------------------------------------------
//  목표 목록 관리 (/baro/project/target/<id>)
// ------------------------------------------------------------

// View_baro_project_target : 목표 목록 관리
func View_baro_project_target(config tool.Config, id string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, ok := baro_project_get(db, id)
	if !ok {
		return baro_project_missing(
			db, config, tool.Baro_t("프로젝트를 찾을 수 없음"),
			tool.Baro_t("프로젝트 ")+id+tool.Baro_t(" 번은 없습니다."),
			"/baro/project", tool.Baro_t("프로젝트 목록으로"),
		)
	}

	message := ""
	message_kind := "danger"
	can_manage := baro_project_can_manage(db, config, row)
	is_member := baro_project_is_member(db, row.ID, config.IP)

	if values != nil {
		if page, blocked := baro_project_readonly_block(db, config); blocked {
			return page
		}

		if page, need := tool.Baro_login_wall(db, config, "/baro/project/target/"+id); need {
			return page
		}

		action := values.Get("do")
		target_id := values.Get("target")

		switch action {
		case "add":
			if !can_manage && !is_member {
				message = tool.Baro_t("목표를 추가하려면 먼저 프로젝트에 참여해 주세요.")
				break
			}

			lines := tool.Baro_split_lines(values.Get("targets"))
			if len(lines) == 0 {
				message = tool.Baro_t("추가할 목표를 한 줄에 하나씩 적어 주세요.")
				break
			}

			if len(lines) > 500 {
				lines = lines[:500]
			}

			added := 0
			for _, line := range lines {
				name := tool.Baro_trunc(line, 200)

				exist := ""
				if tool.QueryRow_DB(db, "select id from baro_project_target where project = ? and name = ?", []any{&exist}, row.ID, name) {
					continue
				}

				tool.Exec_DB(
					db,
					"insert into baro_project_target (id, project, name, note, status, doc, date, assignee) values (?, ?, ?, ?, 'todo', '', ?, '')",
					tool.Baro_next_id(db, "baro_project_target"), row.ID, name, tool.Baro_trunc(values.Get("note"), 300), tool.Baro_now(),
				)

				added++
			}

			tool.Baro_public_log(db, "project_target_add", config.IP, "project:"+row.ID, tool.Baro_itoa(added)+"건 추가", tool.Baro_trunc(row.Name, 100))
			tool.Baro_audit(db, config, "baro_project_target_add", "project:"+row.ID, "", tool.Baro_itoa(added), "목표 일괄 등록")

			if refreshed, found := baro_project_get(db, row.ID); found {
				baro_project_recount(db, refreshed)
			}

			return tool.Get_redirect("/baro/project/target/" + tool.Url_parser(row.ID))

		case "sync":
			done, total := baro_project_target_sync(db, row.ID)
			if refreshed, found := baro_project_get(db, row.ID); found {
				baro_project_recount(db, refreshed)
			}

			message = tool.Baro_t("문서 대조를 마쳤습니다. 완료 ") + tool.Baro_itoa(done) + tool.Baro_t(" / 전체 ") + tool.Baro_itoa(total) + tool.Baro_t("건.")
			message_kind = "ok"

		case "assign":
			if !tool.Baro_is_user(config.IP) {
				message = tool.Baro_t("로그인이 필요합니다.")
				break
			}

			// 이미 다른 사람이 맡은 목표를 가로채지 못하게 한다.
			holder := ""
			tool.QueryRow_DB(db, "select coalesce(assignee, '') from baro_project_target where id = ? and project = ?", []any{&holder}, target_id, row.ID)
			if holder != "" && holder != config.IP {
				message = tool.Baro_t("이미 다른 사람이 맡고 있는 목표입니다.")
				break
			}

			tool.Exec_DB(
				db,
				"update baro_project_target set assignee = ? where id = ? and project = ? and (assignee = '' or assignee is null or assignee = ?)",
				config.IP, target_id, row.ID, config.IP,
			)

			return tool.Get_redirect("/baro/project/target/" + tool.Url_parser(row.ID))

		case "unassign":
			// 맡은 사람 본인이나 프로젝트 담당자만 놓을 수 있다.
			holder := ""
			tool.QueryRow_DB(db, "select coalesce(assignee, '') from baro_project_target where id = ? and project = ?", []any{&holder}, target_id, row.ID)
			if holder != config.IP && !can_manage {
				message = tool.Baro_t("맡은 사람 본인이나 프로젝트 담당자만 담당을 놓을 수 있습니다.")
				break
			}

			tool.Exec_DB(db, "update baro_project_target set assignee = '' where id = ? and project = ?", target_id, row.ID)

			return tool.Get_redirect("/baro/project/target/" + tool.Url_parser(row.ID))

		case "hold":
			if !can_manage {
				message = tool.Baro_t("보류 처리는 프로젝트 담당자만 할 수 있습니다.")
				break
			}

			tool.Exec_DB(db, "update baro_project_target set status = 'hold' where id = ? and project = ?", target_id, row.ID)

			return tool.Get_redirect("/baro/project/target/" + tool.Url_parser(row.ID))

		case "todo":
			if !can_manage {
				message = tool.Baro_t("상태 되돌리기는 프로젝트 담당자만 할 수 있습니다.")
				break
			}

			tool.Exec_DB(db, "update baro_project_target set status = 'todo' where id = ? and project = ?", target_id, row.ID)

			return tool.Get_redirect("/baro/project/target/" + tool.Url_parser(row.ID))

		case "delete":
			if !can_manage {
				message = tool.Baro_t("목표 삭제는 프로젝트 담당자만 할 수 있습니다.")
				break
			}

			name := ""
			tool.QueryRow_DB(db, "select coalesce(name, '') from baro_project_target where id = ?", []any{&name}, target_id)

			tool.Exec_DB(db, "delete from baro_project_target where id = ? and project = ?", target_id, row.ID)
			tool.Baro_public_log(db, "project_target_delete", config.IP, "project:"+row.ID, values.Get("reason"), tool.Baro_trunc(name, 100))
			tool.Baro_audit(db, config, "baro_project_target_delete", "project:"+row.ID, name, "", values.Get("reason"))

			return tool.Get_redirect("/baro/project/target/" + tool.Url_parser(row.ID))

		default:
			message = tool.Baro_t("알 수 없는 요청입니다.")
		}
	}

	row, _ = baro_project_get(db, row.ID)
	row = baro_project_recount(db, row)

	status := query.Get("status")
	page := baro_project_page_num(query)
	list := baro_project_target_list(db, row.ID, status, (page-1)*baro_project_page_size, baro_project_page_size)

	body := baro_project_sub_tabs(row.ID, "/baro/project/target/"+tool.Url_parser(row.ID))
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/project", "프로젝트"},
		{"/baro/project/view/" + tool.Url_parser(row.ID), tool.Baro_trunc(row.Name, 24)},
		{"", "목표 목록"},
	})

	if message != "" {
		body += tool.Baro_alert(message_kind, tool.HTML_escape(message))
	}

	body += tool.Baro_hero(
		row.Name+" 목표 목록",
		tool.Baro_t("목표 이름과 같은 문서가 만들어지면 자동으로 완료 표시됩니다."),
		baro_project_progress_bar(db, row, tool.Baro_t("진행률")),
	)

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 목표"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_target where project = ?", row.ID)), ""},
		{tool.Baro_t("미작성"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_target where project = ? and status = 'todo'", row.ID)), ""},
		{tool.Baro_t("완료"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_target where project = ? and status = 'done'", row.ID)), ""},
		{tool.Baro_t("보류"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_target where project = ? and status = 'hold'", row.ID)), ""},
	})

	// 일괄 등록
	if tool.Baro_is_user(config.IP) {
		add_form := tool.Baro_textarea("targets", tool.Baro_t("목표 이름 (한 줄에 하나)"), "", 10,
			tool.Baro_t("홍길동\n김철수\n이영희"),
			tool.Baro_t("한 번에 최대 500줄까지 넣을 수 있습니다. 이미 있는 이름은 건너뜁니다."))
		add_form += tool.Baro_input("note", tool.Baro_t("공통 메모 (선택)"), "", tool.Baro_t("예: 1차 명단"), "")
		add_form += tool.Baro_hidden("do", "add")
		add_form += tool.Baro_submit(tool.Baro_t("목표 한 번에 등록"), "primary")

		body += tool.Baro_card("목표 여러 줄로 등록", tool.Baro_form("/baro/project/target/"+tool.Url_parser(row.ID), add_form))
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("목표를 등록하려면 로그인이 필요합니다. ")+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary"))
	}

	// 문서 대조
	body += tool.Baro_card(
		tool.Baro_t("문서 대조"),
		tool.Baro_note(tool.Baro_t("목표 이름(또는 지정된 문서 이름)이 위키에 실제로 있으면 완료로 바꿉니다. 화면을 열 때마다 자동으로 한 번 대조합니다."))+
			tool.Baro_form("/baro/project/target/"+tool.Url_parser(row.ID),
				tool.Baro_hidden("do", "sync")+tool.Baro_submit(tool.Baro_t("지금 다시 대조하기"), "")),
	)

	// 필터
	body += tool.Baro_card("걸러 보기", `<form class="bw_form" method="get" action="/baro/project/target/`+tool.Url_parser(row.ID)+`">`+
		tool.Baro_select("status", "상태", [][]string{
			{"", tool.Baro_t("전체")},
			{"todo", tool.Baro_t("미작성")},
			{"done", tool.Baro_t("완료")},
			{"hold", tool.Baro_t("보류")},
		}, status, "")+
		tool.Baro_submit("걸러 보기", "primary")+`</form>`)

	rows := [][]string{}
	for _, target := range list {
		status_badge := tool.Baro_badge(tool.Baro_t("미작성"), "warn")
		switch target.Status {
		case "done":
			status_badge = tool.Baro_badge(tool.Baro_t("완료"), "ok")
		case "hold":
			status_badge = tool.Baro_badge(tool.Baro_t("보류"), "muted")
		}

		doc := `<span class="bw_muted">-</span>`
		if target.Doc != "" {
			doc = tool.Baro_doc_link(target.Doc)
		} else {
			doc = `<a href="/edit/` + tool.Url_parser(target.Name) + tool.Baro_t(`">문서 만들기</a>`)
		}

		assignee := tool.Baro_t(`<span class="bw_muted">없음</span>`)
		if target.Assignee != "" {
			assignee = tool.Baro_user_link(db, target.Assignee, config.IP)
		}

		action := ""
		if tool.Baro_is_user(config.IP) {
			base := "/baro/project/target/" + tool.Url_parser(row.ID)

			if target.Assignee == config.IP {
				action += tool.Baro_form(base, tool.Baro_hidden("do", "unassign")+tool.Baro_hidden("target", target.ID)+tool.Baro_submit(tool.Baro_t("담당 놓기"), ""))
			} else if target.Assignee == "" && target.Status != "done" {
				action += tool.Baro_form(base, tool.Baro_hidden("do", "assign")+tool.Baro_hidden("target", target.ID)+tool.Baro_submit(tool.Baro_t("내가 맡기"), ""))
			}

			if can_manage {
				if target.Status == "hold" {
					action += tool.Baro_form(base, tool.Baro_hidden("do", "todo")+tool.Baro_hidden("target", target.ID)+tool.Baro_submit(tool.Baro_t("미작성으로"), ""))
				} else if target.Status != "done" {
					action += tool.Baro_form(base, tool.Baro_hidden("do", "hold")+tool.Baro_hidden("target", target.ID)+tool.Baro_submit(tool.Baro_t("보류"), ""))
				}

				action += tool.Baro_form(base, tool.Baro_hidden("do", "delete")+tool.Baro_hidden("target", target.ID)+tool.Baro_submit("삭제", "danger"))
			}
		}

		if action == "" {
			action = `<span class="bw_muted">-</span>`
		}

		rows = append(rows, []string{
			tool.HTML_escape(target.Name),
			status_badge,
			doc,
			assignee,
			tool.HTML_escape(tool.Baro_trunc(target.Note, 40)),
			action,
		})
	}

	table := tool.Baro_table([]string{tool.Baro_t("이름"), "상태", "문서", tool.Baro_t("담당"), tool.Baro_t("메모"), "관리"}, rows, "")
	if len(list) == 0 {
		table = tool.Baro_empty(tool.Baro_t("등록된 목표가 없습니다. 위 상자에 이름을 한 줄에 하나씩 적어 한 번에 등록해 보세요."))
	}

	body += tool.Baro_card(tool.Baro_t("목표"), table)
	body += tool.Baro_paging(page, len(list), baro_project_page_size, baro_project_query_string(
		"/baro/project/target/"+tool.Url_parser(row.ID), [][]string{{"status", status}},
	))

	return tool.Baro_page(db, config, tool.Baro_t("목표 목록 : ")+tool.Baro_trunc(row.Name, 30), body, baro_project_menu())
}

// ------------------------------------------------------------
//  프로젝트 과제 (/baro/project/task/<id>)
// ------------------------------------------------------------

// View_baro_project_task_of : 프로젝트에 속한 과제 목록
func View_baro_project_task_of(config tool.Config, id string, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, ok := baro_project_get(db, id)
	if !ok {
		return baro_project_missing(
			db, config, tool.Baro_t("프로젝트를 찾을 수 없음"),
			tool.Baro_t("프로젝트 ")+id+tool.Baro_t(" 번은 없습니다."),
			"/baro/project", tool.Baro_t("프로젝트 목록으로"),
		)
	}

	page := baro_project_page_num(query)
	list := baro_project_task_load(db, []string{"project = ?"}, []any{row.ID}, (page-1)*baro_project_page_size, baro_project_page_size)

	body := baro_project_sub_tabs(row.ID, "/baro/project/task/"+tool.Url_parser(row.ID))
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/project/view/" + tool.Url_parser(row.ID), tool.Baro_trunc(row.Name, 24)},
		{"", tool.Baro_t("과제")},
	})
	body += tool.Baro_hero(row.Name+tool.Baro_t(" 과제"), tool.Baro_t("이 프로젝트 안에서 나뉜 일입니다."),
		tool.Baro_button("/baro/task/new?project="+tool.Url_parser(row.ID)+"&field="+url.QueryEscape(row.Field), "과제 등록", "primary"))

	rows := [][]string{}
	for _, task := range list {
		rows = append(rows, baro_project_task_row_cells(db, config, task))
	}

	table := tool.Baro_table(baro_project_task_headers(), rows, "")
	if len(list) == 0 {
		table = tool.Baro_empty(tool.Baro_t("이 프로젝트에 등록된 과제가 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("과제"), table)
	body += tool.Baro_paging(page, len(list), baro_project_page_size, "/baro/project/task/"+tool.Url_parser(row.ID)+"?page={}")

	return tool.Baro_page(db, config, tool.Baro_t("프로젝트 과제 : ")+tool.Baro_trunc(row.Name, 30), body, baro_project_menu())
}

// ------------------------------------------------------------
//  자료 모음 (/baro/project/source/<id>)
// ------------------------------------------------------------

// View_baro_project_source : 프로젝트 자료 모음
func View_baro_project_source(config tool.Config, id string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, ok := baro_project_get(db, id)
	if !ok {
		return baro_project_missing(
			db, config, tool.Baro_t("프로젝트를 찾을 수 없음"),
			tool.Baro_t("프로젝트 ")+id+tool.Baro_t(" 번은 없습니다."),
			"/baro/project", tool.Baro_t("프로젝트 목록으로"),
		)
	}

	message := ""
	can_manage := baro_project_can_manage(db, config, row)

	if values != nil {
		if page, blocked := baro_project_readonly_block(db, config); blocked {
			return page
		}

		if page, need := tool.Baro_login_wall(db, config, "/baro/project/source/"+id); need {
			return page
		}

		switch values.Get("do") {
		case "add":
			title := strings.TrimSpace(values.Get("title"))
			if title == "" {
				message = tool.Baro_t("자료 이름을 적어 주세요.")
				break
			}

			source_id := tool.Baro_next_id(db, "baro_project_source")
			tool.Exec_DB(
				db,
				"insert into baro_project_source (id, project, title, url, note, user, date) values (?, ?, ?, ?, ?, ?, ?)",
				source_id, row.ID, tool.Baro_trunc(title, 200),
				tool.Baro_trunc(strings.TrimSpace(values.Get("url")), 500),
				tool.Baro_trunc(values.Get("note"), 1000), config.IP, tool.Baro_now(),
			)

			tool.Baro_contrib_add(db, config.IP, "source", "", "", row.ID, "프로젝트 자료 등록 : "+tool.Baro_trunc(title, 80))
			baro_project_room_log(db, row.ID, config.IP, "자료 등록", tool.Baro_trunc(title, 100))

			return tool.Get_redirect("/baro/project/source/" + tool.Url_parser(row.ID))

		case "delete":
			source_id := values.Get("source")

			owner := ""
			tool.QueryRow_DB(db, "select coalesce(user, '') from baro_project_source where id = ?", []any{&owner}, source_id)

			if owner != config.IP && !can_manage {
				message = tool.Baro_t("자료를 올린 사람이나 프로젝트 담당자만 지울 수 있습니다.")
				break
			}

			tool.Exec_DB(db, "delete from baro_project_source where id = ? and project = ?", source_id, row.ID)
			baro_project_room_log(db, row.ID, config.IP, tool.Baro_t("자료 삭제"), tool.Baro_t("자료 ")+source_id)
			tool.Baro_audit(db, config, "baro_project_source_delete", "project:"+row.ID, source_id, "", values.Get("reason"))

			return tool.Get_redirect("/baro/project/source/" + tool.Url_parser(row.ID))

		default:
			message = tool.Baro_t("알 수 없는 요청입니다.")
		}
	}

	page := baro_project_page_num(query)
	list := baro_project_source_list(db, row.ID, (page-1)*baro_project_page_size, baro_project_page_size)

	body := baro_project_sub_tabs(row.ID, "/baro/project/source/"+tool.Url_parser(row.ID))
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/project/view/" + tool.Url_parser(row.ID), tool.Baro_trunc(row.Name, 24)},
		{"", "자료 모음"},
	})

	if message != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(message))
	}

	body += tool.Baro_hero(row.Name+" 자료 모음", tool.Baro_t("같은 자료를 여러 사람이 다시 찾지 않도록 한 곳에 모읍니다."), "")

	if tool.Baro_is_user(config.IP) {
		form := tool.Baro_hidden("do", "add")
		form += tool.Baro_input("title", tool.Baro_t("자료 이름"), "", tool.Baro_t("예: 공훈전자사료관 독립유공자 공적조서"), "")
		form += tool.Baro_input("url", tool.Baro_t("주소 (선택)"), "", "https://...", tool.Baro_t("http 또는 https 주소만 링크로 만들어집니다."))
		form += tool.Baro_textarea("note", tool.Baro_t("메모"), "", 4, tool.Baro_t("어떤 부분에 쓸 수 있는 자료인지"), "")
		form += tool.Baro_submit("자료 등록", "primary")

		body += tool.Baro_card("자료 등록", tool.Baro_form("/baro/project/source/"+tool.Url_parser(row.ID), form))
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("자료를 등록하려면 로그인이 필요합니다. ")+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary"))
	}

	rows := [][]string{}
	for _, source := range list {
		action := `<span class="bw_muted">-</span>`
		if source[4] == config.IP || can_manage {
			action = tool.Baro_form("/baro/project/source/"+tool.Url_parser(row.ID),
				tool.Baro_hidden("do", "delete")+tool.Baro_hidden("source", source[0])+tool.Baro_submit("삭제", "danger"))
		}

		rows = append(rows, []string{
			tool.HTML_escape(source[1]),
			baro_project_link_html(source[2]),
			tool.HTML_escape(tool.Baro_trunc(source[3], 80)),
			tool.Baro_user_link(db, source[4], config.IP),
			tool.Baro_time_cell(source[5]),
			action,
		})
	}

	table := tool.Baro_table([]string{tool.Baro_t("자료"), tool.Baro_t("주소"), tool.Baro_t("메모"), tool.Baro_t("올린 사람"), tool.Baro_t("등록"), "관리"}, rows, "")
	if len(list) == 0 {
		table = tool.Baro_empty(tool.Baro_t("모아 둔 자료가 없습니다. 먼저 쓴 자료를 남겨 두면 다음 사람이 빨라집니다."))
	}

	body += tool.Baro_card(tool.Baro_t("자료"), table)
	body += tool.Baro_paging(page, len(list), baro_project_page_size, "/baro/project/source/"+tool.Url_parser(row.ID)+"?page={}")

	return tool.Baro_page(db, config, tool.Baro_t("자료 모음 : ")+tool.Baro_trunc(row.Name, 30), body, baro_project_menu())
}

// ------------------------------------------------------------
//  진행 기록 (/baro/project/progress/<id>)
// ------------------------------------------------------------

// View_baro_project_progress : 진행 기록과 꺾은선 그래프
func View_baro_project_progress(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, ok := baro_project_get(db, id)
	if !ok {
		return baro_project_missing(
			db, config, tool.Baro_t("프로젝트를 찾을 수 없음"),
			tool.Baro_t("프로젝트 ")+id+tool.Baro_t(" 번은 없습니다."),
			"/baro/project", tool.Baro_t("프로젝트 목록으로"),
		)
	}

	message := ""

	if values != nil {
		if page, blocked := baro_project_readonly_block(db, config); blocked {
			return page
		}

		if page, need := tool.Baro_login_wall(db, config, "/baro/project/progress/"+id); need {
			return page
		}

		if !baro_project_can_manage(db, config, row) && !baro_project_is_member(db, row.ID, config.IP) {
			message = tool.Baro_t("진행 기록은 프로젝트 참여자만 남길 수 있습니다.")
		} else {
			refreshed := baro_project_recount(db, row)
			baro_project_progress_save(db, row.ID, refreshed.DoneCount, refreshed.TargetCount, tool.Baro_trunc(values.Get("note"), 300))
			baro_project_room_log(db, row.ID, config.IP, "진행 기록", tool.Baro_trunc(values.Get("note"), 100))

			return tool.Get_redirect("/baro/project/progress/" + tool.Url_parser(row.ID))
		}
	}

	row = baro_project_recount(db, row)
	list := baro_project_progress_list(db, row.ID, 180)

	body := baro_project_sub_tabs(row.ID, "/baro/project/progress/"+tool.Url_parser(row.ID))
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/project/view/" + tool.Url_parser(row.ID), tool.Baro_trunc(row.Name, 24)},
		{"", "진행 기록"},
	})

	if message != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(message))
	}

	body += tool.Baro_hero(row.Name+" 진행 기록", tool.Baro_t("날짜별로 얼마나 채워졌는지 남깁니다."), baro_project_progress_bar(db, row, tool.Baro_t("현재 진행률")))

	body += tool.Baro_card(tool.Baro_t("추이"), baro_project_chart(list))

	rows := [][]string{}
	for index := len(list) - 1; index >= 0; index-- {
		item := list[index]
		done := tool.Str_to_int(item[2])
		total := tool.Str_to_int(item[3])

		delta := ""
		if index > 0 {
			diff := done - tool.Str_to_int(list[index-1][2])
			if diff > 0 {
				delta = tool.Baro_badge("+"+tool.Baro_itoa(diff), "ok")
			} else if diff < 0 {
				delta = tool.Baro_badge(tool.Baro_itoa(diff), "danger")
			} else {
				delta = `<span class="bw_muted">0</span>`
			}
		} else {
			delta = `<span class="bw_muted">-</span>`
		}

		pct := `<span class="bw_muted">-</span>`
		if total > 0 {
			pct = tool.HTML_escape(tool.Baro_pct_text(done, total))
		}

		rows = append(rows, []string{
			tool.HTML_escape(item[1]),
			tool.Baro_comma(done),
			tool.Baro_comma(total),
			pct,
			delta,
			tool.HTML_escape(tool.Baro_trunc(item[4], 60)),
		})
	}

	table := tool.Baro_table([]string{tool.Baro_t("날짜"), tool.Baro_t("완료"), tool.Baro_t("목표"), tool.Baro_t("비율"), tool.Baro_t("증감"), tool.Baro_t("메모")}, rows, "")
	if len(list) == 0 {
		table = tool.Baro_empty(tool.Baro_t("아직 진행 기록이 없습니다. 목표를 등록하면 날마다 자동으로 쌓입니다."))
	}

	body += tool.Baro_card(tool.Baro_t("날짜별 기록"), table)

	if tool.Baro_is_user(config.IP) {
		body += tool.Baro_card(
			tool.Baro_t("오늘 기록 남기기"),
			tool.Baro_note(tool.Baro_t("지금 상태를 다시 세어 오늘 자 기록으로 저장합니다."))+
				tool.Baro_form("/baro/project/progress/"+tool.Url_parser(row.ID),
					tool.Baro_input("note", tool.Baro_t("메모"), "", tool.Baro_t("예: 1차 명단 100건 정리 완료"), "")+
						tool.Baro_submit(tool.Baro_t("오늘 기록 저장"), "primary")),
		)
	}

	return tool.Baro_page(db, config, tool.Baro_t("진행 기록 : ")+tool.Baro_trunc(row.Name, 30), body, baro_project_menu())
}

// ------------------------------------------------------------
//  결과 보고서 (/baro/project/report/<id>)
// ------------------------------------------------------------

// View_baro_project_report : 결과 보고서 작성 / 보기
func View_baro_project_report(config tool.Config, id string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, ok := baro_project_get(db, id)
	if !ok {
		return baro_project_missing(
			db, config, tool.Baro_t("프로젝트를 찾을 수 없음"),
			tool.Baro_t("프로젝트 ")+id+tool.Baro_t(" 번은 없습니다."),
			"/baro/project", tool.Baro_t("프로젝트 목록으로"),
		)
	}

	can_manage := baro_project_can_manage(db, config, row)
	message := ""

	if values != nil {
		if page, blocked := baro_project_readonly_block(db, config); blocked {
			return page
		}

		if page, need := tool.Baro_login_wall(db, config, "/baro/project/report/"+id); need {
			return page
		}

		if !can_manage {
			return tool.Baro_deny(db, config, tool.Baro_t("결과 보고서는 프로젝트 담당자나 운영자만 쓸 수 있습니다."))
		}

		before := tool.Baro_trunc(row.Report, 500)

		tool.Exec_DB(
			db,
			"update baro_project set report = ?, summary = ?, status = ? where id = ?",
			tool.Baro_trunc(values.Get("report"), 20000),
			tool.Baro_trunc(values.Get("summary"), 500),
			values.Get("status"),
			row.ID,
		)

		tool.Baro_public_log(db, "project_report", config.IP, "project:"+row.ID, "결과 보고서 갱신", tool.Baro_trunc(row.Name, 100))
		tool.Baro_audit(db, config, "baro_project_report", "project:"+row.ID, before, tool.Baro_trunc(values.Get("report"), 500), "결과 보고서 작성")

		for _, member := range baro_project_member_list(db, row.ID) {
			if member.Status != "active" || member.User == config.IP {
				continue
			}

			tool.Baro_notify(
				db, member.User, "project",
				tool.Baro_t("프로젝트 결과 보고서가 올라왔습니다 : ")+row.Name,
				"/baro/project/report/"+tool.Url_parser(row.ID), "",
			)
		}

		return tool.Get_redirect("/baro/project/report/" + tool.Url_parser(row.ID))
	}

	row = baro_project_recount(db, row)

	body := baro_project_sub_tabs(row.ID, "/baro/project/report/"+tool.Url_parser(row.ID))
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/project/view/" + tool.Url_parser(row.ID), tool.Baro_trunc(row.Name, 24)},
		{"", "결과 보고서"},
	})

	if message != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(message))
	}

	body += tool.Baro_hero(row.Name+" 결과 보고서", tool.Baro_t("무엇을 했고 무엇이 남았는지 정리합니다."), baro_project_status_badge(row.Status))

	// 자동 집계 요약
	member_count := baro_project_count(db, "select count(*) from baro_project_member where project = ? and status = 'active'", row.ID)
	task_done := baro_project_count(db, "select count(*) from baro_project_task where project = ? and status = 'done'", row.ID)
	source_count := baro_project_count(db, "select count(*) from baro_project_source where project = ?", row.ID)
	event_count := baro_project_count(db, "select count(*) from baro_contrib_event where project = ?", row.ID)

	body += tool.Baro_card(tool.Baro_t("자동 집계"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("목표 수"), tool.Baro_comma(row.TargetCount), ""},
		{tool.Baro_t("완료 수"), tool.Baro_comma(row.DoneCount), ""},
		{tool.Baro_t("진행률"), func() string {
			if row.TargetCount <= 0 {
				return "-"
			}

			return tool.Baro_pct_text(row.DoneCount, row.TargetCount)
		}(), ""},
		{"참여자", tool.Baro_comma(member_count), ""},
		{tool.Baro_t("완료한 과제"), tool.Baro_comma(task_done), ""},
		{tool.Baro_t("모은 자료"), tool.Baro_comma(source_count), ""},
		{"기여 기록", tool.Baro_comma(event_count), ""},
	}))

	report_html := tool.Baro_empty(tool.Baro_t("아직 결과 보고서가 작성되지 않았습니다."))
	if strings.TrimSpace(row.Report) != "" {
		report_html = tool.Baro_code(row.Report)
	}

	body += tool.Baro_card("보고서", report_html)

	if can_manage {
		template := row.Report
		if strings.TrimSpace(template) == "" {
			template = tool.Baro_t("1. 무엇을 했나\n\n2. 목표 대비 결과\n\n3. 잘 된 점\n\n4. 아쉬운 점\n\n5. 다음에 이어서 할 일\n")
		}

		form := tool.Baro_textarea("report", "결과 보고서", template, 16, "", tool.Baro_t("참여자 모두에게 알림이 갑니다."))
		form += tool.Baro_input("summary", tool.Baro_t("한 줄 요약"), row.Summary, "", "")
		form += tool.Baro_select("status", tool.Baro_t("프로젝트 상태"), baro_project_status_list(), row.Status, "")
		form += tool.Baro_submit(tool.Baro_t("보고서 저장"), "primary")

		body += tool.Baro_card("보고서 쓰기", tool.Baro_form("/baro/project/report/"+tool.Url_parser(row.ID), form))
	} else {
		body += tool.Baro_note(tool.Baro_t("결과 보고서는 프로젝트 담당자나 운영자가 작성합니다."))
	}

	_ = query

	return tool.Baro_page(db, config, tool.Baro_t("결과 보고서 : ")+tool.Baro_trunc(row.Name, 30), body, baro_project_menu())
}

// ------------------------------------------------------------
//  6-5 프로젝트 역할 (/baro/project/member/<id>)
// ------------------------------------------------------------

// View_baro_project_member : 역할 지정
//
//	이 화면은 콘텐츠 역할만 다룬다. 어떤 경우에도 이용자 제재 기능을 제공하지 않는다.
func View_baro_project_member(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, ok := baro_project_get(db, id)
	if !ok {
		return baro_project_missing(
			db, config, tool.Baro_t("프로젝트를 찾을 수 없음"),
			tool.Baro_t("프로젝트 ")+id+tool.Baro_t(" 번은 없습니다."),
			"/baro/project", tool.Baro_t("프로젝트 목록으로"),
		)
	}

	can_manage := baro_project_can_manage(db, config, row)
	message := ""
	message_kind := "danger"

	if values != nil {
		if page, blocked := baro_project_readonly_block(db, config); blocked {
			return page
		}

		if page, need := tool.Baro_login_wall(db, config, "/baro/project/member/"+id); need {
			return page
		}

		if !can_manage {
			return tool.Baro_deny(db, config, tool.Baro_t("역할을 바꿀 권한이 없습니다. 프로젝트 담당자에게 요청해 주세요."))
		}

		action := values.Get("do")
		target_user := strings.TrimSpace(values.Get("user"))

		switch action {
		case "set_role":
			role := baro_project_role_norm(values.Get("role"))
			if !baro_project_role_known(role) {
				message = tool.Baro_t("알 수 없는 역할입니다.")
				break
			}

			before := baro_project_member_role(db, row.ID, target_user)
			if before == "" {
				message = tool.Baro_t("참여 중인 사용자가 아닙니다.")
				break
			}

			tool.Exec_DB(db, "update baro_project_member set role = ? where project = ? and user = ?", role, row.ID, target_user)

			tool.Baro_public_log(db, "project_role", config.IP, "project:"+row.ID,
				baro_project_role_name(before)+" → "+baro_project_role_name(role), target_user)
			tool.Baro_audit(db, config, "baro_project_role", "project:"+row.ID, before, role, "프로젝트 역할 변경")
			tool.Baro_notify(db, target_user, "project",
				tool.Baro_t("프로젝트 역할이 바뀌었습니다 : ")+baro_project_role_name(role),
				"/baro/project/view/"+tool.Url_parser(row.ID), "")

			return tool.Get_redirect("/baro/project/member/" + tool.Url_parser(row.ID))

		case "invite":
			if !tool.Baro_is_user(target_user) {
				message = tool.Baro_t("초대할 사용자 이름을 정확히 적어 주세요. (로그인 계정만 참여할 수 있습니다)")
				break
			}

			exist := ""
			if !tool.QueryRow_DB(db, "select id from user_set where name = 'pw' and id = ?", []any{&exist}, target_user) {
				message = "'" + target_user + tool.Baro_t("' 계정을 찾지 못했습니다.")
				break
			}

			baro_project_member_add(db, row.ID, target_user, values.Get("role"))
			tool.Baro_notify(db, target_user, "project",
				tool.Baro_t("프로젝트에 초대됐습니다 : ")+row.Name,
				"/baro/project/view/"+tool.Url_parser(row.ID), "")
			tool.Baro_public_log(db, "project_invite", config.IP, "project:"+row.ID, baro_project_role_name(values.Get("role")), target_user)

			return tool.Get_redirect("/baro/project/member/" + tool.Url_parser(row.ID))

		case "remove":
			if target_user == row.Owner {
				message = tool.Baro_t("프로젝트를 만든 사람은 참여자 목록에서 뺄 수 없습니다.")
				break
			}

			tool.Exec_DB(db, "update baro_project_member set status = 'closed' where project = ? and user = ?", row.ID, target_user)
			tool.Baro_public_log(db, "project_member_out", config.IP, "project:"+row.ID, values.Get("reason"), target_user)
			tool.Baro_audit(db, config, "baro_project_member_out", "project:"+row.ID, target_user, "", values.Get("reason"))

			message = tool.Baro_t("참여자 목록에서 뺐습니다. 이 조치는 이 프로젝트 안에서만 유효하며 위키 전체 이용에는 아무 영향이 없습니다.")
			message_kind = "ok"

		default:
			message = tool.Baro_t("알 수 없는 요청입니다.")
		}
	}

	body := baro_project_sub_tabs(row.ID, "/baro/project/member/"+tool.Url_parser(row.ID))
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/project/view/" + tool.Url_parser(row.ID), tool.Baro_trunc(row.Name, 24)},
		{"", "역할"},
	})

	if message != "" {
		body += tool.Baro_alert(message_kind, tool.HTML_escape(message))
	}

	body += tool.Baro_hero(row.Name+tool.Baro_t(" 참여자와 역할"), tool.Baro_t("누가 무엇을 맡고 있는지 정리합니다."), "")

	// 0장 1번 원칙 : 콘텐츠 역할과 제재 권한의 분리를 화면에 명시한다.
	body += tool.Baro_alert("warn",
		tool.Baro_t(`<b>프로젝트 운영자는 이용자를 차단할 수 없습니다.</b><br>`)+
			tool.Baro_t(`프로젝트 역할은 문서를 함께 쓰기 위한 <b>콘텐츠 역할</b>입니다. `)+
			tool.Baro_t(`경고·편집 제한·차단 같은 <b>제재 권한은 완전히 분리</b>되어 있으며, `)+
			tool.Baro_t(`사건 기록과 이의제기 절차를 거쳐 제재 담당 역할만 집행합니다. `)+
			tool.Baro_t(`그래서 이 화면에는 제재 기능이 하나도 없습니다.`))

	members := baro_project_member_list(db, row.ID)

	// 역할별 인원 (예전 이름으로 저장된 역할도 표준 역할로 세어 준다)
	role_count := map[string]int{}
	for _, member := range members {
		if member.Status != "active" {
			continue
		}

		role_count[baro_project_role_norm(member.Role)]++
	}

	role_items := []string{}
	for _, role := range baro_project_role_list() {
		role_items = append(role_items, tool.Baro_card_ex(
			role[1],
			tool.Baro_itoa(role_count[role[0]])+tool.Baro_t("명"),
			tool.Baro_note(role[2]),
			"",
		))
	}
	body += tool.Baro_section(tool.Baro_t("역할 종류"), tool.Baro_grid(role_items, 3))

	rows := [][]string{}
	for _, member := range members {
		status_badge := tool.Baro_badge(tool.Baro_t("참여 중"), "ok")
		if member.Status != "active" {
			status_badge = tool.Baro_badge(tool.Baro_t("나감"), "muted")
		}

		control := `<span class="bw_muted">-</span>`
		if can_manage && member.Status == "active" {
			control = tool.Baro_form("/baro/project/member/"+tool.Url_parser(row.ID),
				tool.Baro_hidden("do", "set_role")+
					tool.Baro_hidden("user", member.User)+
					tool.Baro_select("role", "", baro_project_role_options(), baro_project_role_norm(member.Role), "")+
					tool.Baro_submit(tool.Baro_t("역할 저장"), ""))

			if member.User != row.Owner {
				control += tool.Baro_form("/baro/project/member/"+tool.Url_parser(row.ID),
					tool.Baro_hidden("do", "remove")+
						tool.Baro_hidden("user", member.User)+
						tool.Baro_input("reason", "", tool.Baro_t("빼는 이유"), "", "")+
						tool.Baro_submit(tool.Baro_t("이 프로젝트에서 빼기"), "danger"))
			}
		}

		owner_mark := ""
		if member.User == row.Owner {
			owner_mark = " " + tool.Baro_badge(tool.Baro_t("만든 사람"), "info")
		}

		rows = append(rows, []string{
			tool.Baro_user_link(db, member.User, config.IP) + owner_mark,
			tool.HTML_escape(baro_project_role_name(member.Role)),
			status_badge,
			tool.Baro_time_cell(member.Date),
			control,
		})
	}

	body += tool.Baro_card("참여자", tool.Baro_table([]string{tool.Baro_t("사용자"), "역할", "상태", tool.Baro_t("참여"), tool.Baro_t("역할 지정")}, rows, tool.Baro_t("참여자가 없습니다.")))

	if can_manage {
		form := tool.Baro_hidden("do", "invite")
		form += tool.Baro_input("user", tool.Baro_t("사용자 이름"), "", tool.Baro_t("가입한 계정 이름"), tool.Baro_t("로그인 계정만 참여할 수 있습니다."))
		form += tool.Baro_select("role", "역할", baro_project_role_options(), "member", "")
		form += tool.Baro_submit(tool.Baro_t("참여자로 추가"), "primary")

		body += tool.Baro_card("참여자 추가", tool.Baro_form("/baro/project/member/"+tool.Url_parser(row.ID), form))
	}

	body += tool.Baro_card(
		tool.Baro_t("이 화면에서 할 수 없는 일"),
		tool.Baro_list([]string{
			tool.HTML_escape(tool.Baro_t("이용자 차단 · 편집 제한 · 경고 같은 제재")),
			tool.HTML_escape(tool.Baro_t("다른 프로젝트나 위키 전체에 영향을 주는 조치")),
			tool.HTML_escape(tool.Baro_t("참여자가 이미 쓴 문서의 일괄 삭제")),
		})+
			tool.Baro_note(tool.Baro_t("참여자를 프로젝트에서 빼도 그 사람이 쓴 문서는 그대로 남습니다. 제재와 콘텐츠 판단은 분리되어 있습니다.")),
	)

	return tool.Baro_page(db, config, tool.Baro_t("역할 : ")+tool.Baro_trunc(row.Name, 30), body, baro_project_menu())
}
