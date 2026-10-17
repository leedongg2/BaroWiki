package route

import (
	"net/url"
	"strings"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

// ============================================================
//  6-6 공동 작업실 (/baro/project/room/<id>)
//
//  문서 초안 · 작업 분담 · 체크포인트 · 출처 모음 · 변경 내역 ·
//  검토 요청 · 완성본 게시를 한 화면에서 처리한다.
// ============================================================

// View_baro_project_room : 공동 작업실
func View_baro_project_room(config tool.Config, id string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, ok := baro_project_get(db, id)
	if !ok {
		return baro_project_missing(
			db, config, tool.Baro_t("프로젝트를 찾을 수 없음"),
			"프로젝트 "+id+tool.Baro_t(" 번은 없습니다. 작업실은 프로젝트마다 하나씩 있습니다."),
			"/baro/project", tool.Baro_t("프로젝트 목록으로"),
		)
	}

	can_manage := baro_project_can_manage(db, config, row)
	is_member := baro_project_is_member(db, row.ID, config.IP)
	can_write := can_manage || is_member

	message := ""
	message_kind := "danger"

	if values != nil {
		if page, blocked := baro_project_readonly_block(db, config); blocked {
			return page
		}

		if page, need := tool.Baro_login_wall(db, config, "/baro/project/room/"+id); need {
			return page
		}

		if !can_write {
			return tool.Baro_deny(db, config, tool.Baro_t("공동 작업실은 프로젝트 참여자만 쓸 수 있습니다. 프로젝트 개요 화면에서 먼저 참여해 주세요."))
		}

		action := values.Get("do")
		draft_id := values.Get("draft")
		redirect := "/baro/project/room/" + tool.Url_parser(row.ID)

		switch action {
		case "draft_new":
			title := strings.TrimSpace(values.Get("title"))
			if title == "" {
				message = tool.Baro_t("초안 제목(만들 문서 이름)을 적어 주세요.")
				break
			}

			new_id := tool.Baro_next_id(db, "baro_workspace")
			tool.Exec_DB(
				db,
				"insert into baro_workspace (id, project, title, data, user, date, status, note) values (?, ?, ?, ?, ?, ?, 'draft', ?)",
				new_id, row.ID, tool.Baro_trunc(title, 200), values.Get("data"),
				config.IP, tool.Baro_now(), tool.Baro_trunc(values.Get("note"), 500),
			)

			baro_project_room_log(db, row.ID, config.IP, tool.Baro_t("초안 만들기"), tool.Baro_trunc(title, 100))

			return tool.Get_redirect(redirect + "?draft=" + tool.Url_parser(new_id))

		case "draft_save":
			draft, found := baro_project_draft_get(db, draft_id)
			if !found || draft.Project != row.ID {
				message = tool.Baro_t("초안을 찾지 못했습니다.")
				break
			}

			if draft.Status == "done" {
				message = tool.Baro_t("이미 게시된 초안입니다. 고치려면 위키 문서를 직접 편집해 주세요.")
				break
			}

			tool.Exec_DB(
				db,
				"update baro_workspace set title = ?, data = ?, note = ?, date = ?, user = ? where id = ?",
				tool.Baro_trunc(strings.TrimSpace(values.Get("title")), 200),
				values.Get("data"),
				tool.Baro_trunc(values.Get("note"), 500),
				tool.Baro_now(), config.IP, draft.ID,
			)

			baro_project_room_log(db, row.ID, config.IP, tool.Baro_t("초안 수정"),
				tool.Baro_trunc(draft.Title, 80)+" ("+tool.Baro_itoa(tool.Get_len(values.Get("data")))+tool.Baro_t("자)"))

			return tool.Get_redirect(redirect + "?draft=" + tool.Url_parser(draft.ID))

		case "draft_review":
			draft, found := baro_project_draft_get(db, draft_id)
			if !found || draft.Project != row.ID {
				message = tool.Baro_t("초안을 찾지 못했습니다.")
				break
			}

			tool.Exec_DB(db, "update baro_workspace set status = 'review', date = ? where id = ?", tool.Baro_now(), draft.ID)
			baro_project_room_log(db, row.ID, config.IP, "검토 요청", tool.Baro_trunc(draft.Title, 100))

			for _, member := range baro_project_member_list(db, row.ID) {
				if member.Status != "active" || member.User == config.IP {
					continue
				}

				member_role := baro_project_role_norm(member.Role)
				if member_role != "reviewer" && member_role != "manager" {
					continue
				}

				tool.Baro_notify(
					db, member.User, "review",
					tool.Baro_t("초안 검토 요청 : ")+draft.Title,
					redirect+"?draft="+tool.Url_parser(draft.ID), row.Name,
				)
			}

			return tool.Get_redirect(redirect + "?draft=" + tool.Url_parser(draft.ID))

		case "draft_back":
			draft, found := baro_project_draft_get(db, draft_id)
			if !found || draft.Project != row.ID {
				message = tool.Baro_t("초안을 찾지 못했습니다.")
				break
			}

			tool.Exec_DB(db, "update baro_workspace set status = 'draft', date = ? where id = ?", tool.Baro_now(), draft.ID)
			baro_project_room_log(db, row.ID, config.IP, tool.Baro_t("검토 요청 취소"), tool.Baro_trunc(draft.Title, 100))

			return tool.Get_redirect(redirect + "?draft=" + tool.Url_parser(draft.ID))

		case "draft_publish":
			draft, found := baro_project_draft_get(db, draft_id)
			if !found || draft.Project != row.ID {
				message = tool.Baro_t("초안을 찾지 못했습니다.")
				break
			}

			doc_name := tool.Namespace_normalize(strings.TrimSpace(values.Get("doc")))
			if doc_name == "" {
				doc_name = tool.Namespace_normalize(strings.TrimSpace(draft.Title))
			}

			switch {
			case doc_name == "":
				message = tool.Baro_t("게시할 문서 이름을 적어 주세요.")
			case strings.TrimSpace(draft.Data) == "":
				message = tool.Baro_t("내용이 비어 있는 초안은 게시할 수 없습니다.")
			case !tool.Check_acl(db, doc_name, "", "edit", config.IP):
				message = "'" + doc_name + tool.Baro_t("' 문서를 편집할 권한이 없습니다.")
			default:
				old_data := ""
				exist := tool.QueryRow_DB(db, "select data from data where title = ?", []any{&old_data}, doc_name)

				if exist && values.Get("overwrite") != "on" {
					message = "'" + doc_name + tool.Baro_t("' 문서가 이미 있습니다. 덮어쓰려면 아래 '기존 문서 덮어쓰기'를 확인해 주세요.")
					break
				}

				length := tool.Get_edit_length_diff(old_data, draft.Data)

				tool.Exec_DB(db, "delete from data where title = ?", doc_name)
				tool.Exec_DB(db, "insert into data (title, data) values (?, ?)", doc_name, draft.Data)

				tool.Do_add_history(
					db, doc_name, draft.Data, tool.Get_time(), config.IP,
					"프로젝트 작업실 게시 : "+tool.Baro_trunc(row.Name, 60),
					length, "", "",
				)

				markup.Get_render(db, doc_name, draft.Data, "backlink")

				rev := tool.Get_document_revision(db, doc_name)

				tool.Exec_DB(
					db,
					"update baro_workspace set status = 'done', title = ?, date = ? where id = ?",
					doc_name, tool.Baro_now(), draft.ID,
				)

				// 문서를 프로젝트 분야에 연결해 분야 통계에 잡히게 한다.
				if row.Field != "" && tool.Baro_doc_field(db, doc_name) == "" {
					tool.Baro_doc_meta_set(db, doc_name, "field", row.Field)
				}

				tool.Baro_contrib_add(db, config.IP, "project", doc_name, rev, row.ID,
					"프로젝트 작업실에서 게시 : "+tool.Baro_trunc(row.Name, 80))

				// 같은 이름의 목표가 있으면 완료 처리
				tool.Exec_DB(
					db,
					"update baro_project_target set status = 'done', doc = ?, date = ? where project = ? and (name = ? or doc = ?)",
					doc_name, tool.Baro_now(), row.ID, doc_name, doc_name,
				)

				baro_project_room_log(db, row.ID, config.IP, tool.Baro_t("완성본 게시"), tool.Baro_trunc(doc_name, 100))
				tool.Baro_public_log(db, "project_publish", config.IP, "project:"+row.ID, "작업실 게시", tool.Baro_trunc(doc_name, 100))
				tool.Baro_audit(db, config, "baro_project_publish", doc_name, tool.Baro_trunc(old_data, 300), tool.Baro_trunc(draft.Data, 300), "작업실 완성본 게시")
				tool.Baro_metric_bump(db, "baro_project_publish", tool.Baro_today(), 1)

				for _, member := range baro_project_member_list(db, row.ID) {
					if member.Status != "active" || member.User == config.IP {
						continue
					}

					tool.Baro_notify(
						db, member.User, "project",
						tool.Baro_t("작업실 초안이 문서로 게시됐습니다 : ")+doc_name,
						"/w/" + tool.Doc_url(doc_name), row.Name,
					)
				}

				if refreshed, exists := baro_project_get(db, row.ID); exists {
					baro_project_recount(db, refreshed)
				}

				return tool.Get_redirect(redirect + "?draft=" + tool.Url_parser(draft.ID))
			}

		case "draft_delete":
			draft, found := baro_project_draft_get(db, draft_id)
			if !found || draft.Project != row.ID {
				message = tool.Baro_t("초안을 찾지 못했습니다.")
				break
			}

			if draft.User != config.IP && !can_manage {
				message = tool.Baro_t("초안을 만든 사람이나 프로젝트 담당자만 지울 수 있습니다.")
				break
			}

			// 2번 원칙 : 지우기 전에 내용을 감사 기록에 남겨 되살릴 수 있게 한다.
			tool.Baro_audit(db, config, "baro_project_draft_delete", "workspace:"+draft.ID, tool.Baro_trunc(draft.Data, 2000), "", values.Get("reason"))
			tool.Exec_DB(db, "delete from baro_workspace where id = ? and project = ?", draft.ID, row.ID)
			baro_project_room_log(db, row.ID, config.IP, tool.Baro_t("초안 삭제"), tool.Baro_trunc(draft.Title, 100))

			return tool.Get_redirect(redirect)

		case "checkpoint":
			note := strings.TrimSpace(values.Get("note"))
			if note == "" {
				message = tool.Baro_t("체크포인트 내용을 적어 주세요.")
				break
			}

			refreshed := baro_project_recount(db, row)
			baro_project_progress_save(db, row.ID, refreshed.DoneCount, refreshed.TargetCount, tool.Baro_trunc(note, 300))
			baro_project_room_log(db, row.ID, config.IP, tool.Baro_t("체크포인트"), tool.Baro_trunc(note, 100))

			return tool.Get_redirect(redirect)

		case "source_add":
			title := strings.TrimSpace(values.Get("title"))
			if title == "" {
				message = tool.Baro_t("자료 이름을 적어 주세요.")
				break
			}

			tool.Exec_DB(
				db,
				"insert into baro_project_source (id, project, title, url, note, user, date) values (?, ?, ?, ?, ?, ?, ?)",
				tool.Baro_next_id(db, "baro_project_source"), row.ID, tool.Baro_trunc(title, 200),
				tool.Baro_trunc(strings.TrimSpace(values.Get("url")), 500),
				tool.Baro_trunc(values.Get("note"), 1000), config.IP, tool.Baro_now(),
			)

			baro_project_room_log(db, row.ID, config.IP, tool.Baro_t("출처 추가"), tool.Baro_trunc(title, 100))

			return tool.Get_redirect(redirect)

		case "task_take":
			task_id := values.Get("task")

			task, found := baro_project_task_get(db, task_id)
			if !found || task.Project != row.ID {
				message = tool.Baro_t("과제를 찾지 못했습니다.")
				break
			}

			if task.Assignee != "" && task.Assignee != config.IP {
				message = tool.Baro_t("이미 다른 사람이 맡은 과제입니다.")
				break
			}

			tool.Exec_DB(db, "update baro_project_task set assignee = ?, status = 'active' where id = ?", config.IP, task.ID)
			baro_project_room_log(db, row.ID, config.IP, tool.Baro_t("작업 분담"), tool.Baro_trunc(task.Title, 100))

			return tool.Get_redirect(redirect)

		default:
			message = tool.Baro_t("알 수 없는 요청입니다.")
		}
	}

	row, _ = baro_project_get(db, row.ID)
	row = baro_project_recount(db, row)

	drafts := baro_project_draft_list(db, row.ID)

	// 열려 있는 초안 (주소에 ?draft=... 가 있으면 그 초안을 펼친다)
	open_draft := baro_project_draft_row{}
	has_open := false
	if query.Get("draft") != "" {
		if draft, found := baro_project_draft_get(db, query.Get("draft")); found && draft.Project == row.ID {
			open_draft = draft
			has_open = true
		}
	}

	body := baro_project_sub_tabs(row.ID, "/baro/project/room/"+tool.Url_parser(row.ID))
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/project", "프로젝트"},
		{"/baro/project/view/" + tool.Url_parser(row.ID), tool.Baro_trunc(row.Name, 24)},
		{"", "공동 작업실"},
	})

	if message != "" {
		body += tool.Baro_alert(message_kind, tool.HTML_escape(message))
	}

	body += tool.Baro_hero(
		row.Name+" 공동 작업실",
		tool.Baro_t("초안을 함께 쓰고, 검토를 거쳐, 완성된 것만 문서로 내보냅니다."),
		baro_project_progress_bar(db, row, tool.Baro_t("진행률")),
	)

	if !tool.Baro_is_user(config.IP) {
		body += tool.Baro_alert("info", tool.Baro_t("작업실을 쓰려면 로그인이 필요합니다. ")+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary"))
	} else if !can_write {
		body += tool.Baro_alert("warn",
			tool.Baro_t("작업실 내용은 누구나 볼 수 있지만 쓰기는 참여자만 할 수 있습니다. ")+
				tool.Baro_button("/baro/project/join/"+tool.Url_parser(row.ID), "이 프로젝트에 참여하기", "primary"))
	}

	body += tool.Baro_stat_row([][]string{
		{"초안", tool.Baro_comma(len(drafts)), tool.Baro_t("작업실 안의 문서")},
		{tool.Baro_t("검토 대기"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_workspace where project = ? and status = 'review'", row.ID)), ""},
		{tool.Baro_t("게시 완료"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_workspace where project = ? and status = 'done'", row.ID)), ""},
		{tool.Baro_t("열린 과제"), tool.Baro_comma(baro_project_count(db, "select count(*) from baro_project_task where project = ? and (status = '' or status = 'open' or status = 'active')", row.ID)), ""},
	})

	// --- 초안 하나 펼치기 ---
	if has_open {
		draft_body := tool.Baro_kv_grid([][]string{
			{"상태", baro_project_draft_status_badge(open_draft.Status)},
			{tool.Baro_t("마지막 손댄 사람"), tool.Baro_user_link(db, open_draft.User, config.IP)},
			{tool.Baro_t("마지막 수정"), tool.Baro_time_cell(open_draft.Date)},
			{tool.Baro_t("글자 수"), tool.Baro_comma(tool.Get_len(open_draft.Data)) + tool.Baro_t("자")},
		})

		if open_draft.Status == "done" {
			draft_body += tool.Baro_alert("ok", tool.Baro_t("이미 문서로 게시된 초안입니다. ")+tool.Baro_doc_link(open_draft.Title))
			draft_body += tool.Baro_code(open_draft.Data)
		} else if can_write {
			edit := tool.Baro_hidden("do", "draft_save")
			edit += tool.Baro_hidden("draft", open_draft.ID)
			edit += tool.Baro_input("title", tool.Baro_t("만들 문서 이름"), open_draft.Title, tool.Baro_t("예: 홍길동"), tool.Baro_t("게시할 때 이 이름으로 문서가 만들어집니다."))
			edit += tool.Baro_textarea("data", "초안 본문", open_draft.Data, 20, "", tool.Baro_t("위키 문법을 그대로 쓸 수 있습니다."))
			edit += tool.Baro_textarea("note", tool.Baro_t("작업 메모"), open_draft.Note, 3, tool.Baro_t("남은 일, 확인이 필요한 부분"), "")
			edit += tool.Baro_submit(tool.Baro_t("초안 저장"), "primary")

			draft_body += tool.Baro_form("/baro/project/room/"+tool.Url_parser(row.ID), edit)

			// 검토 요청 / 되돌리기
			if open_draft.Status == "review" {
				draft_body += tool.Baro_section("검토",
					tool.Baro_alert("warn", tool.Baro_t("검토 요청 상태입니다. 검토자 역할을 가진 참여자에게 알림이 갔습니다."))+
						tool.Baro_form("/baro/project/room/"+tool.Url_parser(row.ID),
							tool.Baro_hidden("do", "draft_back")+tool.Baro_hidden("draft", open_draft.ID)+
								tool.Baro_submit(tool.Baro_t("검토 요청 취소"), "")))
			} else {
				draft_body += tool.Baro_section("검토 요청",
					tool.Baro_note(tool.Baro_t("다 썼다면 검토를 요청하세요. 검토자와 운영자에게 알림이 갑니다."))+
						tool.Baro_form("/baro/project/room/"+tool.Url_parser(row.ID),
							tool.Baro_hidden("do", "draft_review")+tool.Baro_hidden("draft", open_draft.ID)+
								tool.Baro_submit(tool.Baro_t("검토 요청하기"), "")))
			}

			// 완성본 게시
			publish := tool.Baro_hidden("do", "draft_publish")
			publish += tool.Baro_hidden("draft", open_draft.ID)
			publish += tool.Baro_input("doc", tool.Baro_t("게시할 문서 이름"), open_draft.Title, "", tool.Baro_t("비워 두면 초안 제목을 그대로 씁니다."))
			publish += tool.Baro_checkbox("overwrite", tool.Baro_t("기존 문서 덮어쓰기"), "on", false, tool.Baro_t("같은 이름의 문서가 이미 있을 때만 확인해 주세요."))
			publish += tool.Baro_submit(tool.Baro_t("완성본으로 게시"), "primary")

			draft_body += tool.Baro_section(tool.Baro_t("완성본 게시"),
				tool.Baro_note(tool.Baro_t("게시하면 실제 위키 문서가 되고 편집 역사와 프로젝트 기여 기록이 남습니다."))+
					tool.Baro_form("/baro/project/room/"+tool.Url_parser(row.ID), publish))

			if open_draft.User == config.IP || can_manage {
				draft_body += tool.Baro_danger_zone(tool.Baro_t("초안 지우기"),
					tool.Baro_note(tool.Baro_t("지운 내용은 내부 감사 기록에 남아 운영자가 되살릴 수 있습니다."))+
						tool.Baro_form("/baro/project/room/"+tool.Url_parser(row.ID),
							tool.Baro_hidden("do", "draft_delete")+tool.Baro_hidden("draft", open_draft.ID)+
								tool.Baro_input("reason", tool.Baro_t("지우는 이유"), "", "", "")+
								tool.Baro_submit(tool.Baro_t("초안 지우기"), "danger")))
			}
		} else {
			draft_body += tool.Baro_code(open_draft.Data)
		}

		body += tool.Baro_card(tool.Baro_t("초안 : ")+open_draft.Title, draft_body)
	}

	// --- 초안 목록 ---
	draft_rows := [][]string{}
	for _, draft := range drafts {
		title := `<a href="/baro/project/room/` + tool.Url_parser(row.ID) + `?draft=` + tool.Url_parser(draft.ID) + `">` +
			tool.HTML_escape(tool.Baro_trunc(draft.Title, 50)) + `</a>`

		draft_rows = append(draft_rows, []string{
			title,
			baro_project_draft_status_badge(draft.Status),
			tool.Baro_user_link(db, draft.User, config.IP),
			tool.Baro_comma(tool.Get_len(draft.Data)) + tool.Baro_t("자"),
			tool.HTML_escape(tool.Baro_trunc(draft.Note, 40)),
			tool.Baro_time_cell(draft.Date),
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("문서 초안"),
		tool.Baro_table([]string{"초안", "상태", tool.Baro_t("마지막 손댄 사람"), tool.Baro_t("분량"), tool.Baro_t("메모"), "수정"}, draft_rows,
			tool.Baro_t("아직 초안이 없습니다. 아래에서 새 초안을 시작해 보세요.")),
	)

	if can_write {
		form := tool.Baro_hidden("do", "draft_new")
		form += tool.Baro_input("title", tool.Baro_t("만들 문서 이름"), query.Get("title"), tool.Baro_t("예: 홍길동"), "")
		form += tool.Baro_textarea("data", "초안 본문", "", 10, "== 개요 ==\n", "지금 비워 두고 나중에 이어 써도 됩니다.")
		form += tool.Baro_textarea("note", tool.Baro_t("작업 메모"), "", 2, "", "")
		form += tool.Baro_submit(tool.Baro_t("새 초안 시작"), "primary")

		body += tool.Baro_card("새 초안", tool.Baro_form("/baro/project/room/"+tool.Url_parser(row.ID), form))
	}

	// --- 작업 분담 ---
	tasks := baro_project_task_load(db, []string{"project = ?"}, []any{row.ID}, 0, 30)
	task_rows := [][]string{}
	for _, task := range tasks {
		assignee := tool.Baro_t(`<span class="bw_muted">담당자 없음</span>`)
		if task.Assignee != "" {
			assignee = tool.Baro_user_link(db, task.Assignee, config.IP)
		}

		control := `<span class="bw_muted">-</span>`
		if can_write && task.Assignee == "" && task.Status != "done" {
			control = tool.Baro_form("/baro/project/room/"+tool.Url_parser(row.ID),
				tool.Baro_hidden("do", "task_take")+tool.Baro_hidden("task", task.ID)+tool.Baro_submit(tool.Baro_t("내가 맡기"), ""))
		}

		task_rows = append(task_rows, []string{
			`<a href="/baro/task/view/` + tool.Url_parser(task.ID) + `">` + tool.HTML_escape(tool.Baro_trunc(task.Title, 50)) + `</a>`,
			tool.HTML_escape(baro_project_task_kind_name(task.Kind)),
			baro_project_difficulty_badge(task.Difficulty),
			assignee,
			baro_project_task_status_badge(task.Status),
			control,
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("작업 분담"),
		tool.Baro_table([]string{tool.Baro_t("과제"), "종류", tool.Baro_t("난이도"), "담당자", "상태", ""}, task_rows,
			tool.Baro_t("나뉜 작업이 없습니다. 과제를 등록해 일을 나눠 보세요."))+
			tool.Baro_button("/baro/task/new?project="+tool.Url_parser(row.ID)+"&field="+url.QueryEscape(row.Field), "과제 등록", ""),
	)

	// --- 체크포인트 ---
	checkpoints := baro_project_progress_list(db, row.ID, 60)
	check_rows := [][]string{}
	for index := len(checkpoints) - 1; index >= 0; index-- {
		item := checkpoints[index]
		if strings.TrimSpace(item[4]) == "" || item[4] == baro_project_progress_auto_note {
			continue
		}

		check_rows = append(check_rows, []string{
			tool.HTML_escape(item[1]),
			tool.HTML_escape(item[4]),
			tool.Baro_comma(tool.Str_to_int(item[2])) + " / " + tool.Baro_comma(tool.Str_to_int(item[3])),
		})
	}

	checkpoint_body := tool.Baro_table([]string{tool.Baro_t("날짜"), tool.Baro_t("체크포인트"), tool.Baro_t("그때 진행")}, check_rows, tool.Baro_t("기록된 체크포인트가 없습니다."))
	if can_write {
		checkpoint_body += tool.Baro_form("/baro/project/room/"+tool.Url_parser(row.ID),
			tool.Baro_hidden("do", "checkpoint")+
				tool.Baro_input("note", tool.Baro_t("오늘의 체크포인트"), "", tool.Baro_t("예: 1차 명단 100건 초안 완료"), tool.Baro_t("지금 진행 수와 함께 날짜별로 남습니다."))+
				tool.Baro_submit(tool.Baro_t("체크포인트 남기기"), ""))
	}

	body += tool.Baro_card(tool.Baro_t("체크포인트"), checkpoint_body)

	// --- 출처 모음 ---
	sources := baro_project_source_list(db, row.ID, 0, 15)
	source_rows := [][]string{}
	for _, source := range sources {
		source_rows = append(source_rows, []string{
			tool.HTML_escape(source[1]),
			baro_project_link_html(source[2]),
			tool.HTML_escape(tool.Baro_trunc(source[3], 60)),
			tool.Baro_user_link(db, source[4], config.IP),
		})
	}

	source_body := tool.Baro_table([]string{tool.Baro_t("자료"), tool.Baro_t("주소"), tool.Baro_t("메모"), tool.Baro_t("올린 사람")}, source_rows, tool.Baro_t("모아 둔 출처가 없습니다."))
	if can_write {
		source_body += tool.Baro_form("/baro/project/room/"+tool.Url_parser(row.ID),
			tool.Baro_hidden("do", "source_add")+
				tool.Baro_input("title", tool.Baro_t("자료 이름"), "", "", "")+
				tool.Baro_input("url", tool.Baro_t("주소"), "", "https://...", "")+
				tool.Baro_input("note", tool.Baro_t("메모"), "", "", "")+
				tool.Baro_submit(tool.Baro_t("출처 추가"), ""))
	}
	source_body += tool.Baro_button("/baro/project/source/"+tool.Url_parser(row.ID), "자료 모음 전체 보기", "")

	body += tool.Baro_card(tool.Baro_t("출처 모음"), source_body)

	// --- 변경 내역 ---
	history := baro_project_room_history(db, row.ID, 40)
	history_rows := [][]string{}
	for _, item := range history {
		history_rows = append(history_rows, []string{
			tool.Baro_user_link(db, item[0], config.IP),
			tool.HTML_escape(item[1]),
			tool.HTML_escape(tool.Baro_trunc(item[2], 60)),
			tool.Baro_time_cell(item[3]),
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("변경 내역"),
		tool.Baro_table([]string{tool.Baro_t("사용자"), tool.Baro_t("한 일"), tool.Baro_t("대상"), "시각"}, history_rows, tool.Baro_t("변경 내역이 없습니다."))+
			tool.Baro_note(tool.Baro_t("작업실 변경 내역은 공개 운영 로그에 남습니다. 개인정보는 남기지 않습니다.")),
	)

	return tool.Baro_page(db, config, tool.Baro_t("공동 작업실 : ")+tool.Baro_trunc(row.Name, 30), body, baro_project_menu())
}
