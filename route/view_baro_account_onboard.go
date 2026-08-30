package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  2-2 첫 가입 온보딩 (3분 안에 끝나는 4단계 마법사)
// ============================================================

// baro_account_onboard_titles : 단계별 제목
func baro_account_onboard_titles() [][]string {
	return [][]string{
		{"1", tool.Baro_t("관심 분야 고르기"), tool.Baro_t("무엇을 쓰고 싶은지 골라 주세요. 나중에 바꿀 수 있습니다.")},
		{"2", tool.Baro_t("닉네임 확인과 한 줄 소개"), tool.Baro_t("다른 기여자에게 보일 이름과 소개입니다.")},
		{"3", tool.Baro_t("편집 방법 30초 안내"), tool.Baro_t("문법은 여덟 줄이면 충분합니다.")},
		{"4", tool.Baro_t("연습장에서 한 번 써보기"), tool.Baro_t("실제 문서에 영향을 주지 않습니다.")},
	}
}

// baro_account_onboard_progress : 진행 막대와 단계 표시
func baro_account_onboard_progress(db *sql.DB, user string, now_step int) string {
	marks := baro_account_onboard_marks(db, user)

	done := 0
	items := []string{}
	for _, row := range baro_account_onboard_titles() {
		mark := marks[row[0]]
		badge := tool.Baro_badge(tool.Baro_t("아직"), "muted")

		switch mark {
		case "done":
			badge = tool.Baro_badge(tool.Baro_t("완료"), "ok")
			done++
		case "skip":
			badge = tool.Baro_badge(tool.Baro_t("건너뜀"), "warn")
			done++
		}

		if tool.Str_to_int(row[0]) == now_step && mark == "" {
			badge = tool.Baro_badge(tool.Baro_t("진행 중"), "info")
		}

		items = append(items,
			`<a href="/baro/onboarding?step=`+tool.Url_parser(row[0])+`">`+tool.HTML_escape(row[0]+tool.Baro_t("단계 · ")+row[1])+`</a> `+badge)
	}

	return tool.Baro_progress(done, len(baro_account_onboard_titles()), tool.Baro_t("첫걸음 진행")) + tool.Baro_list(items)
}

// baro_account_onboard_step_form : 각 단계 화면
func baro_account_onboard_step_form(db *sql.DB, config tool.Config, step int) string {
	user := config.IP

	switch step {
	case 1:
		mine := map[string]bool{}
		for _, field := range tool.Baro_user_interest(db, user) {
			mine[field] = true
		}

		checks := []string{}
		for _, option := range baro_account_field_options(db) {
			row := tool.Baro_field_get(db, option[0])
			help := row.Description
			checks = append(checks, tool.Baro_checkbox("field", option[1], option[0], mine[option[0]], help))
		}

		body := tool.Baro_note(tool.Baro_t("여러 개를 골라도 됩니다. 고른 분야에 맞춰 할 일을 추천해 드립니다."))
		body += tool.Baro_grid(checks, 3)
		body += tool.Baro_hidden("action", "step1")
		body += tool.Baro_submit(tool.Baro_t("저장하고 다음"), "primary")

		return tool.Baro_form("/baro/onboarding", body) +
			baro_account_onboard_skip_form(1)

	case 2:
		bio := baro_account_uset_get(db, user, "bio")

		body := tool.Baro_dl([][]string{
			{tool.Baro_t("내 이름"), tool.HTML_escape(user)},
			{tool.Baro_t("이름 바꾸기"), tool.Baro_t(`<a href="/change/user_name">위키 계정 설정에서 바꿀 수 있습니다</a>`)},
		})
		body += tool.Baro_textarea("bio", tool.Baro_t("한 줄 소개"), bio, 3, tool.Baro_t("예: 근현대사 자료를 정리합니다. 출처 확인을 좋아합니다."), tool.Baro_t("프로필에 그대로 보입니다. 200자까지 저장됩니다."))
		body += tool.Baro_hidden("action", "step2")
		body += tool.Baro_submit(tool.Baro_t("저장하고 다음"), "primary")

		return tool.Baro_form("/baro/onboarding", body) +
			baro_account_onboard_skip_form(2)

	case 3:
		body := tool.Baro_note(tool.Baro_t("아래 여덟 줄만 알면 문서를 쓸 수 있습니다. 나머지는 필요할 때 찾아보면 됩니다."))
		body += baro_account_syntax_guide()
		body += tool.Baro_section(tool.Baro_t("실제로 이렇게 씁니다"), tool.Baro_code(baro_account_onboard_example()))
		body += tool.Baro_alert("info", tool.Baro_t("가장 중요한 것은 <b>각주(출처)</b> 입니다. 바로위키는 출처가 있는 서술을 우대합니다."))
		body += tool.Baro_form("/baro/onboarding",
			tool.Baro_hidden("action", "step3")+tool.Baro_submit(tool.Baro_t("읽었습니다. 다음"), "primary"))

		return body + baro_account_onboard_skip_form(3)

	case 4:
		sandbox := baro_account_sandbox_get(db, user)

		body := tool.Baro_note(tool.Baro_t("연습장은 실제 문서에 전혀 영향을 주지 않습니다. 마음껏 시험해 보세요."))
		if sandbox != nil && strings.TrimSpace(sandbox["data"]) != "" {
			body += tool.Baro_alert("ok", tool.Baro_t("연습장에 저장한 내용이 있습니다. 바로 완료 처리할 수 있습니다."))
			body += tool.Baro_code(tool.Baro_trunc(sandbox["data"], 300))
		} else {
			body += tool.Baro_alert("info", tool.Baro_t("아직 연습장에 저장한 내용이 없습니다. 한 번 써 보고 돌아오세요."))
		}

		body += tool.Baro_button("/baro/draft/sandbox?from=onboarding", tool.Baro_t("연습장 열기"), "primary")
		body += " "
		body += tool.Baro_form("/baro/onboarding",
			tool.Baro_hidden("action", "step4")+tool.Baro_submit(tool.Baro_t("연습을 마쳤습니다"), ""))

		return body + baro_account_onboard_skip_form(4)
	}

	return tool.Baro_empty(tool.Baro_t("표시할 단계가 없습니다."))
}

// baro_account_onboard_example : 문법 예시 본문
func baro_account_onboard_example() string {
	return "== 개요 ==\n" +
		tool.Baro_t("'''홍길동'''(1900~1980)은 대한민국의 인물이다.[* 한국인물사전 편찬위원회, 《한국인물사전》, 보기출판, 2019, 120쪽.]\n") +
		"\n" +
		"== 생애 ==\n" +
		tool.Baro_t(" * 1900년 출생\n") +
		tool.Baro_t(" * 1925년 [[보기 학교]] 졸업\n") +
		"\n" +
		"== 각주 ==\n" +
		"\n" +
		tool.Baro_t("[[분류:인물]]")
}

// baro_account_onboard_skip_form : 건너뛰기 단추
func baro_account_onboard_skip_form(step int) string {
	return tool.Baro_form("/baro/onboarding",
		tool.Baro_hidden("action", "skip")+
			tool.Baro_hidden("step", tool.Baro_itoa(step))+
			tool.Baro_submit(tool.Baro_t("이 단계 건너뛰기"), ""))
}

// View_baro_account_onboarding : 첫 가입 온보딩 (/baro/onboarding)
func View_baro_account_onboarding(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/onboarding"); need {
		return wall
	}

	user := config.IP

	// --- 쓰기 처리 ---
	if values != nil {
		if tool.Baro_read_only(db) {
			return tool.Baro_page(db, config, tool.Baro_t("첫걸음"),
				tool.Baro_alert("warn", tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.HTML_escape(tool.Baro_read_only_reason(db))),
				baro_account_menu())
		}

		switch values.Get("action") {
		case "step1":
			baro_account_interest_put(db, user, values["field"])
			baro_account_onboard_advance(db, user, 1, "done")

			return tool.Get_redirect("/baro/onboarding?step=2")

		case "step2":
			bio := tool.Baro_one_line(values.Get("bio"))
			baro_account_uset_put(db, user, "bio", tool.Baro_trunc(bio, 200))
			baro_account_onboard_advance(db, user, 2, "done")

			return tool.Get_redirect("/baro/onboarding?step=3")

		case "step3":
			baro_account_onboard_advance(db, user, 3, "done")

			return tool.Get_redirect("/baro/onboarding?step=4")

		case "step4":
			baro_account_onboard_advance(db, user, 4, "done")
			baro_account_onboard_finish(db, config)

			return tool.Get_redirect("/baro/onboarding/done")

		case "skip":
			step := tool.Str_to_int(values.Get("step"))
			if step < 1 || step > baro_account_onboard_last {
				step = 1
			}

			baro_account_onboard_advance(db, user, step, "skip")
			if step >= baro_account_onboard_last {
				baro_account_onboard_finish(db, config)

				return tool.Get_redirect("/baro/onboarding/done")
			}

			return tool.Get_redirect("/baro/onboarding?step=" + tool.Baro_itoa(step+1))

		case "restart":
			baro_account_uset_put(db, user, "onboard_steps", "")
			baro_account_onboard_put(db, user, 1, "open")

			return tool.Get_redirect("/baro/onboarding?step=1")
		}

		return tool.Get_redirect("/baro/onboarding")
	}

	// --- 보여주기 ---
	now_step, status := baro_account_onboard_get(db, user)

	step := tool.Str_to_int(page)
	if step < 1 || step > baro_account_onboard_last {
		step = now_step
	}
	if step > baro_account_onboard_last {
		step = baro_account_onboard_last
	}

	titles := baro_account_onboard_titles()
	title_row := titles[step-1]

	body := tool.Baro_hero(tool.Baro_t("바로위키 첫걸음"), tool.Baro_t("3분이면 끝납니다. 각 단계는 건너뛸 수 있고, 나중에 이어서 할 수 있습니다."), "")
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/onboarding", tool.Baro_t("첫걸음")},
		{"", title_row[0] + tool.Baro_t("단계")},
	})

	if status == "done" {
		body += tool.Baro_alert("ok", tool.Baro_t("첫걸음을 이미 마쳤습니다. 아래에서 아무 단계나 다시 볼 수 있습니다."))
		body += tool.Baro_button("/baro/onboarding/done", tool.Baro_t("추천 할 일 다시 보기"), "primary")
	}

	body += baro_account_onboard_progress(db, user, now_step)
	body += tool.Baro_card_ex(
		title_row[0]+tool.Baro_t("단계 · ")+title_row[1],
		title_row[2],
		baro_account_onboard_step_form(db, config, step),
		tool.Baro_t("단계를 건너뛰어도 언제든 다시 돌아올 수 있습니다."),
	)

	nav := []string{}
	if step > 1 {
		nav = append(nav, tool.Baro_button("/baro/onboarding?step="+tool.Baro_itoa(step-1), tool.Baro_t("이전 단계"), ""))
	}
	if step < baro_account_onboard_last {
		nav = append(nav, tool.Baro_button("/baro/onboarding?step="+tool.Baro_itoa(step+1), tool.Baro_t("다음 단계 보기"), ""))
	}
	nav = append(nav, tool.Baro_button("/baro/onboarding/done", tool.Baro_t("추천 할 일 보기"), ""))
	nav = append(nav, tool.Baro_button("/baro/account", tool.Baro_t("내 계정"), ""))

	body += tool.Baro_card("이동", strings.Join(nav, " "))
	body += tool.Baro_form("/baro/onboarding",
		tool.Baro_hidden("action", "restart")+tool.Baro_submit(tool.Baro_t("처음부터 다시 하기"), ""))

	return tool.Baro_page(db, config, tool.Baro_t("첫걸음"), body, baro_account_menu())
}

// baro_account_onboard_finish : 온보딩 완료 처리 (알림 · 기록)
func baro_account_onboard_finish(db *sql.DB, config tool.Config) {
	user := config.IP

	if baro_account_uset_get(db, user, "onboard_done") == "on" {
		return
	}

	baro_account_uset_put(db, user, "onboard_done", "on")
	tool.Baro_metric_bump(db, "onboard_done", tool.Baro_today(), 1)
	tool.Baro_notify(db, user, "system", "첫걸음을 마쳤습니다. 추천 할 일을 확인해 보세요.", "/baro/onboarding/done", "")
	tool.Baro_audit(db, config, "onboard_done", user, "", "done", "온보딩 완료")
}

// View_baro_account_onboarding_done : 온보딩 완료 · 첫 과제 추천 (/baro/onboarding/done)
func View_baro_account_onboarding_done(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/onboarding/done"); need {
		return wall
	}

	user := config.IP
	_ = values

	interests := tool.Baro_user_interest(db, user)

	body := tool.Baro_hero(tool.Baro_t("첫걸음 완료"), tool.Baro_t("이제 무엇을 하면 되는지 알려드립니다."),
		tool.Baro_button("/baro/draft/new", tool.Baro_t("첫 초안 만들기"), "primary")+" "+
			tool.Baro_button("/baro/draft/sandbox", tool.Baro_t("연습장 열기"), "")+" "+
			tool.Baro_button("/baro/onboarding", tool.Baro_t("단계 다시 보기"), ""))

	if len(interests) == 0 {
		body += tool.Baro_alert("warn", tool.Baro_t("관심 분야를 아직 고르지 않았습니다. 고르면 더 맞는 과제를 추천할 수 있습니다."))
		body += tool.Baro_button("/baro/onboarding?step=1", tool.Baro_t("관심 분야 고르기"), "")
	} else {
		names := []string{}
		for _, field := range interests {
			names = append(names, tool.Baro_field_name(db, field))
		}

		body += tool.Baro_card(tool.Baro_t("내 관심 분야"), tool.Baro_pills(names)+
			tool.Baro_button("/baro/onboarding?step=1", "바꾸기", ""))
	}

	body += baro_account_onboard_task_card(db, config, interests, page)

	body += tool.Baro_card(tool.Baro_t("그 다음에 할 수 있는 일"),
		tool.Baro_list([]string{
			tool.Baro_t(`<a href="/baro/draft">내 초안</a> 에서 문서를 준비하고, 준비되면 실제 문서로 올립니다.`),
			tool.Baro_t(`<a href="/baro/profile/edit">프로필 설정</a> 에서 한 줄 소개와 대표 문서를 정리합니다.`),
			tool.Baro_t(`<a href="/baro/field">분야 허브</a> 에서 개척이 필요한 주제를 찾습니다.`),
			tool.Baro_t(`<a href="/baro/project">프로젝트</a> 에 참여해 다른 기여자와 함께 씁니다.`),
			tool.Baro_t(`<a href="/baro/notify/set">알림 설정</a> 에서 알림 양을 조절합니다.`),
		}))

	body += tool.Baro_note(tool.Baro_t("혼자 시작하지 않아도 됩니다. 막히면 프로젝트나 분야 허브에서 도움을 요청하세요."))

	return tool.Baro_page(db, config, tool.Baro_t("첫걸음 완료"), body, baro_account_menu())
}

// baro_account_onboard_task_card : 첫 과제 추천 카드 (관심 분야 · 난이도 낮은 것 3개)
func baro_account_onboard_task_card(db *sql.DB, config tool.Config, interests []string, page string) string {
	page_int := baro_account_page_int(page)
	_ = page_int

	rows := [][]string{}

	if len(interests) > 0 {
		holders := []string{}
		args := []any{}
		for _, field := range interests {
			holders = append(holders, "?")
			args = append(args, field)
		}

		query := "select id, title, detail, difficulty, est_time, field, project from baro_project_task " +
			"where status = 'open' and (assignee = '' or assignee is null) and field in (" + strings.Join(holders, ", ") + ") " +
			"order by " + baro_account_difficulty_order() + ", id + 0 desc limit 3"

		task_rows := tool.Query_DB(db, query, args...)
		defer task_rows.Close()

		for task_rows.Next() {
			id, title, detail, difficulty, est_time, field, project := "", "", "", "", "", "", ""
			if err := task_rows.Scan(&id, &title, &detail, &difficulty, &est_time, &field, &project); err != nil {
				continue
			}

			rows = append(rows, []string{
				`<a href="/baro/task/view/` + tool.Url_parser(id) + `">` + tool.HTML_escape(tool.Baro_trunc(title, 60)) + `</a>` +
					`<div class="bw_note">` + tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(detail), 90)) + `</div>`,
				tool.HTML_escape(tool.Baro_field_name(db, field)),
				tool.Baro_badge(tool.Baro_t("난이도 ")+baro_account_difficulty_name(difficulty), baro_account_difficulty_kind(difficulty)),
				func() string {
					if strings.TrimSpace(est_time) == "" {
						return `<span class="bw_muted">-</span>`
					}

					return tool.HTML_escape(est_time)
				}(),
			})
		}
	}

	if len(rows) > 0 {
		html := tool.Baro_note(tool.Baro_t("관심 분야 중에서 난이도가 낮은 순서로 골랐습니다."))
		html += tool.Baro_table([]string{tool.Baro_t("과제"), "분야", tool.Baro_t("난이도"), tool.Baro_t("예상 시간")}, rows, tool.Baro_t("추천할 과제가 없습니다."))
		html += tool.Baro_button("/baro/task", tool.Baro_t("할 일 목록 전체 보기"), "primary")

		return tool.Baro_card(tool.Baro_t("첫 과제 추천"), html)
	}

	// 과제가 없으면 아직 작성되지 않은 프로젝트 대상 항목을 보여준다
	target_rows := tool.Query_DB(
		db,
		"select id, name, note, project from baro_project_target where status = 'todo' or status = '' order by id + 0 desc limit 5",
	)
	defer target_rows.Close()

	list_rows := [][]string{}
	for target_rows.Next() {
		id, name, note, project := "", "", "", ""
		if err := target_rows.Scan(&id, &name, &note, &project); err != nil {
			continue
		}

		project_name := ""
		tool.QueryRow_DB(db, "select name from baro_project where id = ?", []any{&project_name}, project)

		list_rows = append(list_rows, []string{
			tool.Baro_doc_link(name),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(note), 80)),
			`<a href="/baro/project/view/` + tool.Url_parser(project) + `">` + tool.HTML_escape(project_name) + `</a>`,
			tool.Baro_button("/baro/draft/new?title="+tool.Url_parser(name), "초안 시작", ""),
		})

		_ = id
	}

	if len(list_rows) == 0 {
		html := tool.Baro_empty(tool.Baro_t("아직 등록된 과제도, 미작성 목록도 없습니다."))
		html += tool.Baro_alert("info", tool.Baro_t("그렇다면 직접 고르셔도 됩니다. 잘 아는 주제를 하나 골라 초안을 시작해 보세요."))
		html += tool.Baro_button("/baro/draft/new", tool.Baro_t("초안 만들기"), "primary") + " " +
			tool.Baro_button("/baro/field", tool.Baro_t("분야 둘러보기"), "") + " " +
			tool.Baro_button("/baro/task", tool.Baro_t("할 일 목록"), "")

		return tool.Baro_card(tool.Baro_t("첫 과제 추천"), html)
	}

	html := tool.Baro_alert("info", tool.Baro_t("관심 분야에 맞는 과제가 아직 없어, 프로젝트에서 아직 작성되지 않은 항목을 보여드립니다."))
	html += tool.Baro_table([]string{tool.Baro_t("항목"), tool.Baro_t("메모"), "프로젝트", ""}, list_rows, tool.Baro_t("미작성 항목이 없습니다."))
	html += tool.Baro_button("/baro/project", "프로젝트 보기", "")

	_ = config

	return tool.Baro_card(tool.Baro_t("첫 과제 추천"), html)
}
