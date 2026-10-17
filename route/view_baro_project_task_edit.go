package route

import (
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  과제 고치기 (/baro/task/edit/<번호>)
//
//  잘못 적은 제목이나 설명을 고친다.
//  등록할 때와 같은 칸을 쓰되, 이미 적어 둔 값이 채워져 있다.
//
//  고칠 수 있는 사람 : 요청한 본인 · 프로젝트 담당자
//  담당자가 맡고 있는 과제도 고칠 수 있다.
//  (맡은 사람이 무엇을 해야 하는지 더 또렷하게 적어 주는 일이므로 막지 않는다.
//   다만 무엇이 바뀌었는지 맡은 사람에게 알린다)
//
//  완료된 과제는 고치지 않는다. 이미 끝난 일의 조건을 뒤늦게 바꾸면
//  기여 기록과 아귀가 맞지 않기 때문이다.
// ============================================================

// View_baro_project_task_edit : 과제 고치기
func View_baro_project_task_edit(config tool.Config, id string, values url.Values) string {
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

	if page, need := tool.Baro_login_wall(db, config, "/baro/task/edit/"+id); need {
		return page
	}

	if page, blocked := baro_project_readonly_block(db, config); blocked {
		return page
	}

	can_manage := config.IP == task.Requester || tool.Baro_can(db, config.IP, "project")
	if !can_manage {
		return baro_project_missing(
			db, config, tool.Baro_t("고칠 수 없는 과제"),
			tool.Baro_t("요청한 본인이나 프로젝트 담당자만 과제를 고칠 수 있습니다."),
			"/baro/task/view/"+tool.Url_parser(task.ID), tool.Baro_t("과제로 돌아가기"),
		)
	}

	if task.Status == "done" {
		return baro_project_missing(
			db, config, tool.Baro_t("이미 끝난 과제"),
			tool.Baro_t("완료된 과제는 고칠 수 없습니다. 조건을 바꾸어야 한다면 다시 연 뒤에 고쳐 주세요."),
			"/baro/task/view/"+tool.Url_parser(task.ID), tool.Baro_t("과제로 돌아가기"),
		)
	}

	message := ""

	if values != nil {
		title := strings.TrimSpace(values.Get("title"))
		kind := baro_project_task_kind_norm(values.Get("kind"))
		if !baro_project_task_kind_known(kind) {
			kind = task.Kind
		}
		field := values.Get("field")
		project := strings.TrimSpace(values.Get("project"))
		difficulty := values.Get("difficulty")
		est_time := strings.TrimSpace(values.Get("est_time"))
		detail := values.Get("detail")

		switch {
		case title == "":
			message = tool.Baro_t("과제 제목을 적어 주세요.")
		default:
			// 없는 프로젝트 번호는 비운다 (등록할 때와 같은 규칙)
			if project != "" {
				if _, found := baro_project_get(db, project); !found {
					project = ""
				}
			}
			if field == "" && project != "" {
				if row, found := baro_project_get(db, project); found {
					field = row.Field
				}
			}

			title = tool.Baro_trunc(title, 200)
			detail = tool.Baro_trunc(detail, 4000)
			est_time = tool.Baro_trunc(est_time, 40)

			tool.Exec_DB(
				db,
				`update baro_project_task
					set title = ?, kind = ?, field = ?, project = ?, difficulty = ?, est_time = ?, detail = ?
					where id = ?`,
				title, kind, field, project, difficulty, est_time, detail, task.ID,
			)

			changed := baro_project_task_changed(task, title, kind, field, project, difficulty, est_time, detail)

			tool.Baro_public_log(db, "task_edit", config.IP, "task:"+task.ID, values.Get("reason"), tool.Baro_trunc(title, 100))
			tool.Baro_audit(db, config, "baro_task_edit", "task:"+task.ID,
				tool.Baro_trunc(task.Title, 200), tool.Baro_trunc(title, 200), strings.Join(changed, " · "))

			// 맡고 있는 사람에게는 무엇이 바뀌었는지 알린다
			if task.Assignee != "" && task.Assignee != config.IP && len(changed) > 0 {
				tool.Baro_notify(
					db, task.Assignee, "project",
					tool.Baro_t("맡은 과제의 내용이 바뀌었습니다 : ")+title,
					"/baro/task/view/"+tool.Url_parser(task.ID),
					strings.Join(changed, " · "),
				)
			}

			return tool.Get_redirect("/baro/task/view/" + tool.Url_parser(task.ID))
		}
	}

	// 다시 그릴 때는 방금 적어 보낸 값을, 처음 열 때는 저장된 값을 보여 준다
	pick := func(name string, saved string) string {
		if values != nil {
			return values.Get(name)
		}

		return saved
	}

	form := tool.Baro_input("title", tool.Baro_t("과제 제목"), pick("title", task.Title),
		tool.Baro_t("예: 독립유공자 홍길동 문서 만들기"), tool.Baro_t("무엇을 해야 하는지 한 줄로 적어 주세요."))
	form += tool.Baro_select("kind", "종류", baro_project_task_kind_options(),
		baro_project_task_kind_norm(pick("kind", task.Kind)), "")
	form += tool.Baro_select("field", "분야", baro_project_field_options(db, tool.Baro_t("분야 미지정")),
		pick("field", task.Field), "")
	form += tool.Baro_input("project", tool.Baro_t("프로젝트 번호"), pick("project", task.Project),
		tool.Baro_t("예: 3"), tool.Baro_t("특정 프로젝트에 속한 과제라면 번호를 적습니다. 비워도 됩니다."))
	form += tool.Baro_select("difficulty", tool.Baro_t("예상 난이도"), baro_project_difficulty_list(),
		pick("difficulty", task.Difficulty), "")
	form += tool.Baro_input("est_time", tool.Baro_t("예상 소요 시간"), pick("est_time", task.EstTime),
		tool.Baro_t("예: 30분"), tool.Baro_t("맡는 사람이 시간을 가늠할 수 있게 적어 주세요."))
	form += tool.Baro_textarea("detail", tool.Baro_t("자세한 설명"), pick("detail", task.Detail), 8,
		tool.Baro_t("참고할 자료, 주의할 점, 어디까지 하면 완료인지"), "")
	form += tool.Baro_input("reason", tool.Baro_t("고치는 이유 (선택)"), pick("reason", ""),
		tool.Baro_t("설명을 더 또렷하게"), tool.Baro_t("공개 운영 로그에 남습니다."))

	form += tool.Baro_submit(tool.Baro_t("고치기"), "primary") + " " +
		tool.Baro_button("/baro/task/view/"+tool.Url_parser(task.ID), "취소", "")

	body := baro_project_tabs("/baro/task")
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/task", tool.Baro_t("과제")},
		{"/baro/task/view/" + tool.Url_parser(task.ID), tool.Baro_trunc(task.Title, 30)},
		{"", tool.Baro_t("고치기")},
	})

	if message != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(message))
	}

	if task.Assignee != "" && task.Assignee != config.IP {
		body += tool.Baro_alert("warn",
			tool.Baro_user_link(db, task.Assignee, config.IP)+
				tool.Baro_t(" 님이 맡고 있는 과제입니다. 고치면 무엇이 바뀌었는지 알림이 갑니다."))
	}

	body += tool.Baro_card(tool.Baro_t("과제 고치기"), tool.Baro_form("/baro/task/edit/"+tool.Url_parser(task.ID), form))
	body += tool.Baro_note(tool.Baro_t("맡은 사람이 이미 일을 시작했을 수 있습니다. 할 일의 범위가 달라지는 수정은 이유를 함께 적어 주세요."))

	return tool.Baro_page(db, config, tool.Baro_t("과제 고치기"), body, baro_project_menu())
}

// baro_project_task_changed : 무엇이 바뀌었는지 한국어로 모아 준다
//
//	알림과 감사 기록에 「무엇이」 바뀌었는지 남기기 위한 것이다.
func baro_project_task_changed(old baro_project_task_row, title string, kind string, field string,
	project string, difficulty string, est_time string, detail string) []string {
	changed := []string{}

	if old.Title != title {
		changed = append(changed, tool.Baro_t("제목"))
	}
	if baro_project_task_kind_norm(old.Kind) != kind {
		changed = append(changed, tool.Baro_t("종류"))
	}
	if old.Field != field {
		changed = append(changed, tool.Baro_t("분야"))
	}
	if old.Project != project {
		changed = append(changed, tool.Baro_t("프로젝트"))
	}
	if old.Difficulty != difficulty {
		changed = append(changed, tool.Baro_t("난이도"))
	}
	if old.EstTime != est_time {
		changed = append(changed, tool.Baro_t("예상 시간"))
	}
	if old.Detail != detail {
		changed = append(changed, tool.Baro_t("자세한 설명"))
	}

	return changed
}
