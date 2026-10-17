package route

import (
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  6-3 문서 요청 · 과제
// ============================================================

// View_baro_project_task_list : 과제 목록 (/baro/task)
func View_baro_project_task_list(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	field := query.Get("field")
	kind := query.Get("kind")
	difficulty := query.Get("difficulty")
	status := query.Get("status")
	noassign := query.Get("noassign")
	beginner := query.Get("beginner")

	page := baro_project_page_num(query)

	where := []string{}
	args := []any{}

	if field != "" {
		where = append(where, "field = ?")
		args = append(args, field)
	}
	if kind != "" {
		cond, cond_args := baro_project_task_kind_cond(kind)
		where = append(where, cond)
		args = append(args, cond_args...)
	}
	if difficulty != "" {
		where = append(where, "difficulty = ?")
		args = append(args, difficulty)
	}
	if status != "" {
		where = append(where, "status = ?")
		args = append(args, status)
	} else {
		where = append(where, "(status = '' or status = 'open' or status = 'active')")
	}
	if noassign == "on" {
		where = append(where, "(assignee = '' or assignee is null)")
	}
	if beginner == "on" {
		cond, cond_args := baro_project_task_kind_cond("beginner")
		where = append(where, "("+cond+" or difficulty = 'easy')")
		args = append(args, cond_args...)
	}

	list := baro_project_task_load(db, where, args, (page-1)*baro_project_page_size, baro_project_page_size)

	body := baro_project_tabs("/baro/task")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("과제")}})
	body += tool.Baro_hero(
		tool.Baro_t("과제 목록"),
		tool.Baro_t("필요한 문서, 부족한 출처, 없는 사진 — 지금 바로 맡을 수 있는 일입니다."),
		tool.Baro_button("/baro/task/new", "과제 등록", "primary")+" "+
			tool.Baro_button("/baro/task/mine", tool.Baro_t("내가 맡은 과제"), "")+" "+
			tool.Baro_button("/baro/task?beginner=on", tool.Baro_t("초보자용만 보기"), ""),
	)

	// 종류별 안내
	kind_items := []string{}
	for _, row := range baro_project_task_kind_list() {
		cond, cond_args := baro_project_task_kind_cond(row[0])
		count := baro_project_count(
			db,
			"select count(*) from baro_project_task where "+cond+" and (status = '' or status = 'open' or status = 'active')",
			cond_args...,
		)

		kind_items = append(kind_items, tool.Baro_card_ex(
			row[1],
			tool.Baro_comma(count)+tool.Baro_t("건 열려 있음"),
			tool.Baro_note(row[2]),
			tool.Baro_button("/baro/task?kind="+url.QueryEscape(row[0]), tool.Baro_t("이 종류만 보기"), ""),
		))
	}
	body += tool.Baro_section(tool.Baro_t("과제 종류"), tool.Baro_grid(kind_items, 3))

	// 필터 폼
	filter := tool.Baro_select("field", "분야", baro_project_field_options(db, tool.Baro_t("전체 분야")), field, "")
	filter += tool.Baro_select("kind", "종류", append([][]string{{"", tool.Baro_t("전체 종류")}}, baro_project_task_kind_options()...), baro_project_task_kind_norm(kind), "")
	filter += tool.Baro_select("difficulty", tool.Baro_t("난이도"), append([][]string{{"", tool.Baro_t("전체 난이도")}}, baro_project_difficulty_list()...), difficulty, "")
	filter += tool.Baro_select("status", "상태", [][]string{
		{"", tool.Baro_t("열린 과제")},
		{"open", tool.Baro_t("모집 중")},
		{"active", tool.Baro_t("진행 중")},
		{"done", tool.Baro_t("완료")},
		{"closed", tool.Baro_t("닫힘")},
	}, status, "")
	filter += tool.Baro_checkbox("noassign", "담당자 없는 과제만", "on", noassign == "on", "")
	filter += tool.Baro_checkbox("beginner", "초보자용 과제만", "on", beginner == "on", "초보자용 종류이거나 난이도가 쉬움인 과제")
	filter += tool.Baro_submit("걸러 보기", "primary") + " " + tool.Baro_button("/baro/task", tool.Baro_t("필터 지우기"), "")

	body += tool.Baro_card(tool.Baro_t("찾기"), `<form class="bw_form" method="get" action="/baro/task">`+filter+`</form>`)

	rows := [][]string{}
	for _, row := range list {
		rows = append(rows, baro_project_task_row_cells(db, config, row))
	}

	table := tool.Baro_table(baro_project_task_headers(), rows, "")
	if len(list) == 0 {
		table = tool.Baro_empty(tool.Baro_t("조건에 맞는 과제가 없습니다. 필터를 지우거나 새 과제를 등록해 보세요."))
	}

	body += tool.Baro_card(tool.Baro_t("과제"), table)

	body += tool.Baro_paging(page, len(list), baro_project_page_size, baro_project_query_string("/baro/task", [][]string{
		{"field", field},
		{"kind", kind},
		{"difficulty", difficulty},
		{"status", status},
		{"noassign", noassign},
		{"beginner", beginner},
	}))

	return tool.Baro_page(db, config, tool.Baro_t("과제 목록"), body, baro_project_menu())
}

// View_baro_project_task_mine : 내가 맡은 과제 (/baro/task/mine)
func View_baro_project_task_mine(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if page, need := tool.Baro_login_wall(db, config, "/baro/task/mine"); need {
		return page
	}

	page := baro_project_page_num(query)

	mine := baro_project_task_load(
		db,
		[]string{"assignee = ?", "(status = '' or status = 'open' or status = 'active')"},
		[]any{config.IP},
		(page-1)*baro_project_page_size, baro_project_page_size,
	)
	done := baro_project_task_load(
		db,
		[]string{"assignee = ?", "status = 'done'"},
		[]any{config.IP},
		0, 20,
	)
	asked := baro_project_task_load(
		db,
		[]string{"requester = ?"},
		[]any{config.IP},
		0, 20,
	)

	body := baro_project_tabs("/baro/task/mine")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/task", tool.Baro_t("과제")}, {"", tool.Baro_t("내 과제")}})
	body += tool.Baro_hero(tool.Baro_t("내가 맡은 과제"), tool.Baro_t("맡은 일과 내가 부탁한 일을 함께 봅니다."), "")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("진행 중"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_task where assignee = ? and (status = '' or status = 'open' or status = 'active')", config.IP)), tool.Baro_t("맡고 있는 과제")},
		{tool.Baro_t("완료"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_task where assignee = ? and status = 'done'", config.IP)), tool.Baro_t("끝낸 과제")},
		{tool.Baro_t("내가 등록"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_task where requester = ?", config.IP)), tool.Baro_t("부탁한 과제")},
	})

	mine_rows := [][]string{}
	for _, row := range mine {
		mine_rows = append(mine_rows, baro_project_task_row_cells(db, config, row))
	}
	body += tool.Baro_card(tool.Baro_t("진행 중인 과제"), tool.Baro_table(baro_project_task_headers(), mine_rows, tool.Baro_t("맡고 있는 과제가 없습니다. 과제 목록에서 하나 골라 보세요.")))

	body += tool.Baro_paging(page, len(mine), baro_project_page_size, "/baro/task/mine?page={}")

	done_rows := [][]string{}
	for _, row := range done {
		done_rows = append(done_rows, []string{
			`<a href="/baro/task/view/` + tool.Url_parser(row.ID) + `">` + tool.HTML_escape(tool.Baro_trunc(row.Title, 60)) + `</a>`,
			tool.HTML_escape(baro_project_task_kind_name(row.Kind)),
			tool.Baro_doc_link(row.Doc),
			tool.Baro_time_cell(row.DoneDate),
		})
	}
	body += tool.Baro_card(tool.Baro_t("완료한 과제"), tool.Baro_table([]string{tool.Baro_t("과제"), "종류", tool.Baro_t("만든 문서"), tool.Baro_t("완료")}, done_rows, tool.Baro_t("아직 완료한 과제가 없습니다.")))

	asked_rows := [][]string{}
	for _, row := range asked {
		asked_rows = append(asked_rows, []string{
			`<a href="/baro/task/view/` + tool.Url_parser(row.ID) + `">` + tool.HTML_escape(tool.Baro_trunc(row.Title, 60)) + `</a>`,
			baro_project_task_status_badge(row.Status),
			func() string {
				if row.Assignee == "" {
					return tool.Baro_t(`<span class="bw_muted">담당자 없음</span>`)
				}

				return tool.Baro_user_link(db, row.Assignee, config.IP)
			}(),
			tool.Baro_time_cell(row.Date),
		})
	}
	body += tool.Baro_card(tool.Baro_t("내가 등록한 과제"), tool.Baro_table([]string{tool.Baro_t("과제"), "상태", "담당자", tool.Baro_t("등록")}, asked_rows, tool.Baro_t("등록한 과제가 없습니다.")))

	return tool.Baro_page(db, config, tool.Baro_t("내 과제"), body, baro_project_menu())
}

// View_baro_project_task_new : 과제 등록 (/baro/task/new)
func View_baro_project_task_new(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if page, need := tool.Baro_login_wall(db, config, "/baro/task/new"); need {
		return page
	}

	if page, blocked := baro_project_readonly_block(db, config); blocked {
		return page
	}

	message := ""

	if values != nil {
		title := strings.TrimSpace(values.Get("title"))
		kind := baro_project_task_kind_norm(values.Get("kind"))
		if !baro_project_task_kind_known(kind) {
			kind = "need_doc"
		}
		field := values.Get("field")
		project := strings.TrimSpace(values.Get("project"))
		difficulty := values.Get("difficulty")
		est_time := strings.TrimSpace(values.Get("est_time"))
		detail := values.Get("detail")

		switch {
		case title == "":
			message = tool.Baro_t("과제 제목을 적어 주세요.")
		case !tool.Baro_rate_ok(db, "baro_task_new", config.IP, 20, 3600):
			message = tool.Baro_t("과제 등록이 너무 잦습니다. 잠시 뒤에 다시 시도해 주세요.")
		default:
			if project != "" {
				if _, ok := baro_project_get(db, project); !ok {
					project = ""
				}
			}

			if field == "" && project != "" {
				if row, ok := baro_project_get(db, project); ok {
					field = row.Field
				}
			}

			id := tool.Baro_next_id(db, "baro_project_task")
			tool.Exec_DB(
				db,
				`insert into baro_project_task
					(id, project, kind, title, detail, difficulty, est_time, assignee, status, doc, date, done_date, requester, field)
					values (?, ?, ?, ?, ?, ?, ?, '', 'open', '', ?, '', ?, ?)`,
				id, project, kind, tool.Baro_trunc(title, 200), tool.Baro_trunc(detail, 4000),
				difficulty, tool.Baro_trunc(est_time, 40), tool.Baro_now(), config.IP, field,
			)

			tool.Baro_public_log(db, "task_new", config.IP, "task:"+id, baro_project_task_kind_name(kind), tool.Baro_trunc(title, 100))
			tool.Baro_audit(db, config, "baro_task_new", "task:"+id, "", tool.Baro_trunc(title, 200), "과제 등록")
			tool.Baro_metric_bump(db, "baro_task_new", tool.Baro_today(), 1)

			return tool.Get_redirect("/baro/task/view/" + tool.Url_parser(id))
		}
	}

	form := ""
	if values != nil {
		form += tool.Baro_input("title", tool.Baro_t("과제 제목"), values.Get("title"), tool.Baro_t("예: 독립유공자 홍길동 문서 만들기"), tool.Baro_t("무엇을 해야 하는지 한 줄로 적어 주세요."))
		form += tool.Baro_select("kind", "종류", baro_project_task_kind_options(), baro_project_task_kind_norm(values.Get("kind")), "")
		form += tool.Baro_select("field", "분야", baro_project_field_options(db, tool.Baro_t("분야 미지정")), values.Get("field"), "")
		form += tool.Baro_input("project", tool.Baro_t("프로젝트 번호"), values.Get("project"), tool.Baro_t("예: 3"), tool.Baro_t("특정 프로젝트에 속한 과제라면 번호를 적습니다. 비워도 됩니다."))
		form += tool.Baro_select("difficulty", tool.Baro_t("예상 난이도"), baro_project_difficulty_list(), values.Get("difficulty"), "")
		form += tool.Baro_input("est_time", tool.Baro_t("예상 소요 시간"), values.Get("est_time"), tool.Baro_t("예: 30분"), tool.Baro_t("맡는 사람이 시간을 가늠할 수 있게 적어 주세요."))
		form += tool.Baro_textarea("detail", tool.Baro_t("자세한 설명"), values.Get("detail"), 8, tool.Baro_t("참고할 자료, 주의할 점, 어디까지 하면 완료인지"), "")
	} else {
		form += tool.Baro_input("title", tool.Baro_t("과제 제목"), "", tool.Baro_t("예: 독립유공자 홍길동 문서 만들기"), tool.Baro_t("무엇을 해야 하는지 한 줄로 적어 주세요."))
		form += tool.Baro_select("kind", "종류", baro_project_task_kind_options(), baro_project_task_kind_norm(query.Get("kind")), "")
		form += tool.Baro_select("field", "분야", baro_project_field_options(db, tool.Baro_t("분야 미지정")), query.Get("field"), "")
		form += tool.Baro_input("project", tool.Baro_t("프로젝트 번호"), query.Get("project"), tool.Baro_t("예: 3"), tool.Baro_t("특정 프로젝트에 속한 과제라면 번호를 적습니다. 비워도 됩니다."))
		form += tool.Baro_select("difficulty", tool.Baro_t("예상 난이도"), baro_project_difficulty_list(), "normal", "")
		form += tool.Baro_input("est_time", tool.Baro_t("예상 소요 시간"), "", tool.Baro_t("예: 30분"), tool.Baro_t("맡는 사람이 시간을 가늠할 수 있게 적어 주세요."))
		form += tool.Baro_textarea("detail", tool.Baro_t("자세한 설명"), "", 8, tool.Baro_t("참고할 자료, 주의할 점, 어디까지 하면 완료인지"), "")
	}

	form += tool.Baro_submit("과제 등록", "primary") + " " + tool.Baro_button("/baro/task", "취소", "")

	body := baro_project_tabs("/baro/task")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/task", tool.Baro_t("과제")}, {"", tool.Baro_t("새 과제")}})

	if message != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(message))
	}

	body += tool.Baro_card(tool.Baro_t("새 과제 등록"), tool.Baro_form("/baro/task/new", form))
	body += tool.Baro_note(tool.Baro_t("과제는 부탁이지 명령이 아닙니다. 맡는 사람이 판단할 수 있도록 난이도와 예상 시간을 솔직하게 적어 주세요."))

	return tool.Baro_page(db, config, "과제 등록", body, baro_project_menu())
}

// View_baro_project_task_view : 과제 상세 (/baro/task/view/<id>)
func View_baro_project_task_view(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	task, ok := baro_project_task_get(db, id)
	if !ok {
		return baro_project_missing(
			db, config, tool.Baro_t("과제를 찾을 수 없음"),
			tool.Baro_t("과제 ")+id+tool.Baro_t(" 번은 없습니다. 이미 지워졌거나 주소가 잘못됐습니다."),
			"/baro/task", tool.Baro_t("과제 목록으로"),
		)
	}

	message := ""
	message_kind := "danger"

	if values != nil {
		if page, blocked := baro_project_readonly_block(db, config); blocked {
			return page
		}

		if page, need := tool.Baro_login_wall(db, config, "/baro/task/view/"+id); need {
			return page
		}

		action := values.Get("do")
		can_manage := config.IP == task.Requester || tool.Baro_can(db, config.IP, "project")

		switch action {
		case "take":
			if task.Assignee != "" && task.Assignee != config.IP {
				message = tool.Baro_t("이미 다른 사람이 맡고 있는 과제입니다.")
			} else if task.Status == "done" {
				message = tool.Baro_t("이미 완료된 과제입니다.")
			} else {
				tool.Exec_DB(
					db,
					"update baro_project_task set assignee = ?, status = 'active' where id = ?",
					config.IP, task.ID,
				)
				tool.Baro_public_log(db, "task_take", config.IP, "task:"+task.ID, "", tool.Baro_trunc(task.Title, 100))
				tool.Baro_notify(db, task.Requester, "project", "요청하신 과제를 누군가 맡았습니다 : "+task.Title, "/baro/task/view/"+tool.Url_parser(task.ID), "")

				return tool.Get_redirect("/baro/task/view/" + tool.Url_parser(task.ID))
			}

		case "drop":
			if task.Assignee != config.IP && !can_manage {
				message = tool.Baro_t("담당자 본인이나 요청자만 놓을 수 있습니다.")
			} else {
				tool.Exec_DB(db, "update baro_project_task set assignee = '', status = 'open' where id = ?", task.ID)
				tool.Baro_public_log(db, "task_drop", config.IP, "task:"+task.ID, values.Get("reason"), tool.Baro_trunc(task.Title, 100))

				return tool.Get_redirect("/baro/task/view/" + tool.Url_parser(task.ID))
			}

		case "done":
			doc := tool.Namespace_normalize(strings.TrimSpace(values.Get("doc")))

			switch {
			case task.Assignee != config.IP && !can_manage:
				message = tool.Baro_t("담당자만 완료 제출을 할 수 있습니다. 먼저 과제를 맡아 주세요.")
			case doc == "":
				message = tool.Baro_t("완료한 문서 이름을 적어 주세요.")
			default:
				exist := ""
				if !tool.QueryRow_DB(db, "select title from data where title = ?", []any{&exist}, doc) {
					message = "'" + doc + tool.Baro_t("' 문서를 찾지 못했습니다. 문서를 먼저 만든 뒤 이름을 정확히 적어 주세요.")
					break
				}

				rev := tool.Get_document_revision(db, doc)

				// 기여는 실제로 맡아 일한 사람에게 남긴다.
				// (요청자가 대신 마무리하더라도 담당자의 몫을 가져가지 않는다)
				credit := task.Assignee
				if credit == "" {
					credit = config.IP
				}

				tool.Exec_DB(
					db,
					"update baro_project_task set status = 'done', done_date = ?, doc = ?, assignee = ? where id = ?",
					tool.Baro_now(), doc, credit, task.ID,
				)

				tool.Baro_contrib_add(db, credit, "request_done", doc, rev, task.Project, "과제 완료 : "+tool.Baro_trunc(task.Title, 100))
				tool.Baro_public_log(db, "task_done", config.IP, "task:"+task.ID, "", tool.Baro_trunc(task.Title, 100))
				tool.Baro_audit(db, config, "baro_task_done", "task:"+task.ID, task.Status, "done:"+doc, "과제 완료 제출")
				tool.Baro_metric_bump(db, "baro_task_done", tool.Baro_today(), 1)

				tool.Baro_notify(
					db, task.Requester, "project",
					tool.Baro_t("요청하신 과제가 완료됐습니다 : ")+task.Title,
					"/baro/task/view/"+tool.Url_parser(task.ID), doc,
				)

				if task.Project != "" {
					if row, found := baro_project_get(db, task.Project); found {
						baro_project_recount(db, row)
					}
				}

				return tool.Get_redirect("/baro/task/view/" + tool.Url_parser(task.ID))
			}

		case "close":
			if !can_manage {
				message = tool.Baro_t("요청자만 과제를 닫을 수 있습니다.")
			} else {
				tool.Exec_DB(db, "update baro_project_task set status = 'closed' where id = ?", task.ID)
				tool.Baro_public_log(db, "task_close", config.IP, "task:"+task.ID, values.Get("reason"), tool.Baro_trunc(task.Title, 100))

				return tool.Get_redirect("/baro/task/view/" + tool.Url_parser(task.ID))
			}

		case "reopen":
			if !can_manage {
				message = tool.Baro_t("요청자만 과제를 다시 열 수 있습니다.")
			} else {
				tool.Exec_DB(db, "update baro_project_task set status = 'open', assignee = '', done_date = '' where id = ?", task.ID)

				return tool.Get_redirect("/baro/task/view/" + tool.Url_parser(task.ID))
			}

		case "delete":
			// 잘못 올린 과제를 거두는 길.
			//
			//	2번 원칙(삭제보다 보존)에 따라 내용은 지우지 않고 상태만 바꾼다.
			//	목록에서는 빠지고, 주소를 아는 사람만 볼 수 있으며 되돌릴 수 있다.
			switch {
			case !can_manage:
				message = tool.Baro_t("요청한 본인이나 프로젝트 담당자만 지울 수 있습니다.")
			case task.Status == "deleted":
				message = tool.Baro_t("이미 지운 과제입니다.")
			case task.Assignee != "" && task.Assignee != config.IP:
				message = tool.Baro_t("누군가 맡고 있는 과제는 지울 수 없습니다. 먼저 닫아 주세요.")
			default:
				tool.Exec_DB(db, "update baro_project_task set status = 'deleted' where id = ?", task.ID)
				tool.Baro_public_log(db, "task_delete", config.IP, "task:"+task.ID, values.Get("reason"), tool.Baro_trunc(task.Title, 100))
				tool.Baro_audit(db, config, "baro_task_delete", "task:"+task.ID, task.Status, "deleted", values.Get("reason"))

				if task.Project != "" {
					if row, found := baro_project_get(db, task.Project); found {
						baro_project_recount(db, row)
					}
				}

				return tool.Get_redirect("/baro/task")
			}

		case "undelete":
			if !can_manage {
				message = tool.Baro_t("요청한 본인이나 프로젝트 담당자만 되돌릴 수 있습니다.")
			} else if task.Status != "deleted" {
				message = tool.Baro_t("지운 과제가 아닙니다.")
			} else {
				tool.Exec_DB(db, "update baro_project_task set status = 'open', assignee = '', done_date = '' where id = ?", task.ID)
				tool.Baro_public_log(db, "task_undelete", config.IP, "task:"+task.ID, "", tool.Baro_trunc(task.Title, 100))

				return tool.Get_redirect("/baro/task/view/" + tool.Url_parser(task.ID))
			}

		default:
			message = tool.Baro_t("알 수 없는 요청입니다.")
		}

		task, _ = baro_project_task_get(db, task.ID)
	}

	body := baro_project_tabs("/baro/task")
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/task", tool.Baro_t("과제")},
		{"", tool.Baro_trunc(task.Title, 30)},
	})

	if message != "" {
		body += tool.Baro_alert(message_kind, tool.HTML_escape(message))
	}

	body += tool.Baro_hero(
		task.Title,
		baro_project_task_kind_name(task.Kind)+" · "+tool.Baro_field_name(db, task.Field),
		baro_project_task_status_badge(task.Status)+" "+baro_project_difficulty_badge(task.Difficulty),
	)

	assignee_html := tool.Baro_t(`<span class="bw_muted">아직 아무도 맡지 않았습니다</span>`)
	if task.Assignee != "" {
		assignee_html = tool.Baro_user_link(db, task.Assignee, config.IP)
	}

	project_html := tool.Baro_t(`<span class="bw_muted">소속 프로젝트 없음</span>`)
	if task.Project != "" {
		project_html = baro_project_link(db, task.Project)
	}

	body += tool.Baro_card(tool.Baro_t("과제 정보"), tool.Baro_dl([][]string{
		{"종류", tool.HTML_escape(baro_project_task_kind_name(task.Kind))},
		{"분야", baro_project_field_link(db, task.Field)},
		{"프로젝트", project_html},
		{tool.Baro_t("예상 난이도"), baro_project_difficulty_badge(task.Difficulty)},
		{tool.Baro_t("예상 소요 시간"), tool.HTML_escape(func() string {
			if task.EstTime == "" {
				return tool.Baro_t("적혀 있지 않음")
			}

			return task.EstTime
		}())},
		{"담당자", assignee_html},
		{tool.Baro_t("요청자"), tool.Baro_user_link(db, task.Requester, config.IP)},
		{tool.Baro_t("등록"), tool.Baro_time_cell(task.Date)},
		{tool.Baro_t("완료"), tool.Baro_time_cell(task.DoneDate)},
		{tool.Baro_t("만든 문서"), tool.Baro_doc_link(task.Doc)},
	}))

	detail_html := tool.Baro_empty(tool.Baro_t("자세한 설명이 없습니다."))
	if strings.TrimSpace(task.Detail) != "" {
		detail_html = tool.Baro_code(task.Detail)
	}
	body += tool.Baro_card(tool.Baro_t("자세한 설명"), detail_html)

	// 잘못 적은 것을 고치는 길 (완료된 과제는 고치지 않는다)
	if task.Status != "done" && task.Status != "deleted" &&
		(config.IP == task.Requester || tool.Baro_can(db, config.IP, "project")) {
		body += tool.Baro_card("",
			tool.Baro_button("/baro/task/edit/"+tool.Url_parser(task.ID), tool.Baro_t("과제 고치기"), "primary"))
	}

	// 동작 영역
	if task.Status == "deleted" {
		// 지운 과제는 목록에 안 나오지만 내용은 남아 있다 (2번 원칙)
		action := tool.Baro_alert("danger", tool.Baro_t("지운 과제입니다. 목록에는 나오지 않지만 내용은 남아 있습니다."))
		if config.IP == task.Requester || tool.Baro_can(db, config.IP, "project") {
			action += tool.Baro_form("/baro/task/view/"+tool.Url_parser(task.ID),
				tool.Baro_hidden("do", "undelete")+
					tool.Baro_note(tool.Baro_t("되돌리면 모집 중 상태로 다시 열립니다."))+
					tool.Baro_submit(tool.Baro_t("되돌리기"), "primary"))
		}

		body += tool.Baro_card(tool.Baro_t("지운 과제"), action)
	} else if !tool.Baro_is_user(config.IP) {
		body += tool.Baro_alert("info", tool.Baro_t(`과제를 맡으려면 로그인이 필요합니다. `)+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary"))
	} else if task.Status == "done" {
		action := tool.Baro_alert("ok", tool.Baro_t("완료된 과제입니다. 결과 문서 : ")+tool.Baro_doc_link(task.Doc))
		if config.IP == task.Requester || tool.Baro_can(db, config.IP, "project") {
			action += tool.Baro_form("/baro/task/view/"+tool.Url_parser(task.ID),
				tool.Baro_hidden("do", "reopen")+tool.Baro_submit(tool.Baro_t("다시 열기"), ""))
		}

		body += tool.Baro_card(tool.Baro_t("결과"), action)
	} else {
		action := ""

		if task.Assignee == config.IP {
			action += tool.Baro_section(tool.Baro_t("작업 완료 제출"),
				tool.Baro_form("/baro/task/view/"+tool.Url_parser(task.ID),
					tool.Baro_hidden("do", "done")+
						tool.Baro_input("doc", tool.Baro_t("완성한 문서 이름"), task.Doc, tool.Baro_t("예: 홍길동"), tool.Baro_t("이미 만들어 둔 문서 이름을 정확히 적어 주세요. 문서가 있어야 완료로 처리됩니다."))+
						tool.Baro_submit(tool.Baro_t("완료로 제출"), "primary")))

			action += tool.Baro_section(tool.Baro_t("과제 놓기"),
				tool.Baro_form("/baro/task/view/"+tool.Url_parser(task.ID),
					tool.Baro_hidden("do", "drop")+
						tool.Baro_input("reason", tool.Baro_t("놓는 이유 (선택)"), "", tool.Baro_t("시간이 부족해서"), "")+
						tool.Baro_submit(tool.Baro_t("놓기"), "")))
		} else if task.Assignee == "" {
			action += tool.Baro_form("/baro/task/view/"+tool.Url_parser(task.ID),
				tool.Baro_hidden("do", "take")+
					tool.Baro_note(tool.Baro_t("맡으면 다른 사람에게도 진행 중으로 보입니다. 언제든 놓을 수 있습니다."))+
					tool.Baro_submit(tool.Baro_t("이 과제 맡기"), "primary"))
		} else {
			action += tool.Baro_alert("info", tool.Baro_user_link(db, task.Assignee, config.IP)+tool.Baro_t(" 님이 맡고 있습니다."))
		}

		if config.IP == task.Requester || tool.Baro_can(db, config.IP, "project") {
			action += tool.Baro_section(tool.Baro_t("과제 닫기"),
				tool.Baro_form("/baro/task/view/"+tool.Url_parser(task.ID),
					tool.Baro_hidden("do", "close")+
						tool.Baro_input("reason", tool.Baro_t("닫는 이유"), "", tool.Baro_t("더 이상 필요 없어서"), "")+
						tool.Baro_submit(tool.Baro_t("과제 닫기"), "danger")))
		}

		body += tool.Baro_card(tool.Baro_t("이 과제 하기"), action)

		// 잘못 올린 과제 거두기
		//
		//	「닫기」 는 더 할 필요가 없어진 과제를 마무리하는 것이고,
		//	「지우기」 는 애초에 잘못 올린 과제를 목록에서 거두는 것이다.
		if config.IP == task.Requester || tool.Baro_can(db, config.IP, "project") {
			remove := tool.Baro_form("/baro/task/view/"+tool.Url_parser(task.ID),
				tool.Baro_hidden("do", "delete")+
					tool.Baro_input("reason", tool.Baro_t("지우는 이유"), "", tool.Baro_t("잘못 올렸습니다"), "")+
					tool.Baro_submit(tool.Baro_t("과제 지우기"), "danger"))

			body += tool.Baro_danger_zone(tool.Baro_t("과제 지우기"),
				tool.Baro_note(tool.Baro_t("잘못 올린 과제를 목록에서 거둡니다. 내용은 남아 있어 되돌릴 수 있습니다."))+remove)
		}
	}

	// 비슷한 과제
	similar := baro_project_task_load(
		db,
		[]string{"field = ?", "id <> ?", "(status = '' or status = 'open' or status = 'active')"},
		[]any{task.Field, task.ID},
		0, 5,
	)
	similar_rows := [][]string{}
	for _, row := range similar {
		similar_rows = append(similar_rows, baro_project_task_row_cells(db, config, row))
	}
	body += tool.Baro_card(tool.Baro_t("같은 분야의 다른 과제"), tool.Baro_table(baro_project_task_headers(), similar_rows, tool.Baro_t("같은 분야에 열린 과제가 없습니다.")))

	return tool.Baro_page(db, config, tool.Baro_t("과제 : ")+tool.Baro_trunc(task.Title, 40), body, baro_project_menu())
}
