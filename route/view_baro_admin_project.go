package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  16-5 프로젝트 관리
//
//  프로젝트 운영 역할(project)은 제재 권한과 분리되어 있다. (11-3)
//  여기서는 승인 · 운영자 지정 · 통계 · 종료 · 부정 점수 조작 감지를 맡는다.
// ============================================================

// baro_admin_project_status_badge : 프로젝트 진행 상태 배지 (프로젝트 화면과 같은 이름을 쓴다)
func baro_admin_project_status_badge(status string) string {
	switch status {
	case "open":
		return tool.Baro_badge(tool.Baro_t("모집 중"), "info")
	case "active":
		return tool.Baro_badge(tool.Baro_t("진행 중"), "ok")
	case "pending":
		return tool.Baro_badge(tool.Baro_t("잠시 멈춤"), "warn")
	case "done":
		return tool.Baro_badge(tool.Baro_t("목표 달성"), "ok")
	case "closed":
		return tool.Baro_badge(tool.Baro_t("종료 · 보관"), "muted")
	case "draft":
		return tool.Baro_badge(tool.Baro_t("승인 대기"), "warn")
	case "rejected":
		return tool.Baro_badge(tool.Baro_t("반려"), "danger")
	}

	return baro_admin_status_badge(status)
}

// baro_admin_project_self_done : 자기 프로젝트 목표를 스스로 완료 처리한 비율
//
//	돌려주는 값 : (완료 수, 그중 운영자 본인이 맡은 수)
func baro_admin_project_self_done(db *sql.DB, project string, owner string) (int, int) {
	done := 0
	self := 0

	for _, one := range baro_admin_rows(
		db, 1,
		"select assignee from baro_project_target where project = ? and status = 'done' limit 500",
		project,
	) {
		done++
		if one[0] != "" && one[0] == owner {
			self++
		}
	}

	for _, one := range baro_admin_rows(
		db, 2,
		"select assignee, requester from baro_project_task where project = ? and status = 'done' limit 500",
		project,
	) {
		done++
		if one[0] != "" && one[0] == owner && one[1] == owner {
			self++
		}
	}

	return done, self
}

// baro_admin_project_risk : 부정 점수 조작 위험 표시
func baro_admin_project_risk(done int, self int) (string, bool) {
	if done < 5 {
		return tool.Baro_badge(tool.Baro_t("표본 부족"), "muted"), false
	}

	ratio := tool.Baro_pct(self, done)

	switch {
	case ratio >= 80:
		return tool.Baro_badge(tool.Baro_t("주의 ")+tool.Baro_pct_text(self, done), "danger"), true
	case ratio >= 60:
		return tool.Baro_badge(tool.Baro_t("확인 필요 ")+tool.Baro_pct_text(self, done), "warn"), true
	}

	return tool.Baro_badge(tool.Baro_t("정상 ")+tool.Baro_pct_text(self, done), "ok"), false
}

// baro_admin_project_post : 프로젝트 관리 쓰기 동작
func baro_admin_project_post(db *sql.DB, config tool.Config, values url.Values) (string, string) {
	do := baro_admin_get(values, "do")
	id := baro_admin_get(values, "id")
	reason := baro_admin_get(values, "reason")

	row := baro_admin_rows(db, 3, "select id, name, owner from baro_project where id = ?", id)
	if len(row) == 0 {
		return "", tool.Baro_alert("warn", tool.Baro_t("그 번호의 프로젝트가 없습니다."))
	}

	name := row[0][1]
	owner := row[0][2]

	switch do {
	case "approve":
		tool.Exec_DB(db, "update baro_project set status = 'open', start_date = ? where id = ?", tool.Baro_today(), id)
		baro_admin_log(db, config, "project_approve", "project:"+id, reason, "draft", "open")

		if owner != "" && tool.Baro_is_user(owner) {
			tool.Baro_notify(db, owner, "project", "프로젝트 '"+name+"' 가 승인되었습니다.", "/baro/project/view/"+tool.Url_parser(id), reason)
		}

		return tool.Get_redirect("/baro/admin/project?done=approve"), ""

	case "reject":
		if reason == "" {
			return "", tool.Baro_alert("warn", tool.Baro_t("반려 사유는 반드시 적어야 합니다."))
		}

		tool.Exec_DB(db, "update baro_project set status = 'rejected' where id = ?", id)
		baro_admin_log(db, config, "project_reject", "project:"+id, reason, "draft", "rejected")

		if owner != "" && tool.Baro_is_user(owner) {
			tool.Baro_notify(db, owner, "project", "프로젝트 '"+name+"' 가 반려되었습니다.", "/baro/project/view/"+tool.Url_parser(id), reason)
		}

		return tool.Get_redirect("/baro/admin/project?done=reject"), ""

	case "owner":
		new_owner := baro_admin_get(values, "owner")
		if new_owner == "" {
			return "", tool.Baro_alert("warn", tool.Baro_t("운영자로 지정할 계정 이름을 넣어 주세요."))
		}

		tool.Exec_DB(db, "update baro_project set owner = ? where id = ?", new_owner, id)

		found := baro_admin_one(db, "select id from baro_project_member where project = ? and user = ?", id, new_owner)
		if found == "" {
			tool.Exec_DB(
				db,
				"insert into baro_project_member (id, project, user, role, date, status) values (?, ?, ?, 'owner', ?, 'active')",
				tool.Baro_next_id(db, "baro_project_member"), id, new_owner, tool.Baro_now(),
			)
		} else {
			tool.Exec_DB(db, "update baro_project_member set role = 'owner', status = 'active' where id = ?", found)
		}

		baro_admin_log(db, config, "project_owner", "project:"+id, reason, owner, new_owner)

		if tool.Baro_is_user(new_owner) {
			tool.Baro_notify(db, new_owner, "project", "프로젝트 '"+name+"' 운영자로 지정되었습니다.", "/baro/project/view/"+tool.Url_parser(id), reason)
		}

		return tool.Get_redirect("/baro/admin/project?id=" + tool.Url_parser(id) + "&done=owner"), ""

	case "finish":
		tool.Exec_DB(db, "update baro_project set status = 'done', end_date = ?, report = ? where id = ?", tool.Baro_today(), baro_admin_get(values, "report"), id)
		baro_admin_log(db, config, "project_finish", "project:"+id, reason, "open", "done")

		return tool.Get_redirect("/baro/admin/project?done=finish"), ""

	case "archive":
		tool.Exec_DB(db, "update baro_project set status = 'closed', end_date = ? where id = ?", tool.Baro_today(), id)
		baro_admin_log(db, config, "project_archive", "project:"+id, reason, "", "closed")

		return tool.Get_redirect("/baro/admin/project?done=archive"), ""

	case "reopen":
		tool.Exec_DB(db, "update baro_project set status = 'open' where id = ?", id)
		baro_admin_log(db, config, "project_reopen", "project:"+id, reason, "", "open")

		return tool.Get_redirect("/baro/admin/project?done=reopen"), ""

	case "status":
		// 진행 상태를 곧바로 바꾼다 (모집 중 <-> 진행 중 <-> 잠시 멈춤)
		status := baro_admin_get(values, "status")

		switch status {
		case "open", "active", "pending":
		default:
			return "", tool.Baro_alert("warn", tool.Baro_t("모집 중 · 진행 중 · 잠시 멈춤 중에서 고르세요."))
		}

		before := baro_admin_one(db, "select status from baro_project where id = ?", id)

		tool.Exec_DB(db, "update baro_project set status = ? where id = ?", status, id)
		baro_admin_log(db, config, "project_status", "project:"+id, reason, before, status)

		return tool.Get_redirect("/baro/admin/project?id=" + tool.Url_parser(id) + "&done=status"), ""
	}

	return "", tool.Baro_alert("warn", tool.Baro_t("알 수 없는 동작입니다."))
}

// View_baro_admin_project : 16-5 프로젝트 관리
func View_baro_admin_project(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if html, stop := baro_admin_gate(db, config, "project"); stop {
		return html
	}

	notice := ""
	if values != nil {
		redirect, message := baro_admin_project_post(db, config, values)
		if redirect != "" {
			return redirect
		}

		notice = message
	}

	body := tool.Baro_hero(
		tool.Baro_t("프로젝트 관리"),
		tool.Baro_t("승인 · 운영자 지정 · 통계 · 종료 · 부정 점수 조작 감지를 처리합니다."),
		tool.Baro_button("/baro/project", tool.Baro_t("프로젝트 목록"), "")+" "+tool.Baro_button("/baro/field", tool.Baro_t("분야 허브"), ""),
	)
	body += baro_admin_tabs("/baro/admin/project")

	if notice != "" {
		body += notice
	}
	if done := baro_admin_get(query, "done"); done != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("처리했습니다. (")+baro_admin_esc(done)+")")
	}

	if id := baro_admin_get(query, "id"); id != "" {
		body += baro_admin_project_detail(db, config, id)

		return baro_admin_page(db, config, tool.Baro_t("프로젝트 관리"), body)
	}

	status := baro_admin_get(query, "status")
	if status == "" {
		status = "all"
	}

	page := baro_admin_page_num(query)
	size := tool.Baro_page_size

	body += tool.Baro_tabs([][]string{
		{"/baro/admin/project?status=all", tool.Baro_t("전체")},
		{"/baro/admin/project?status=draft", tool.Baro_t("승인 대기")},
		{"/baro/admin/project?status=running", tool.Baro_t("모집 · 진행 중")},
		{"/baro/admin/project?status=pending", tool.Baro_t("잠시 멈춤")},
		{"/baro/admin/project?status=done", tool.Baro_t("완료")},
		{"/baro/admin/project?status=closed", tool.Baro_t("보관")},
		{"/baro/admin/project?status=rejected", tool.Baro_t("반려")},
	}, "/baro/admin/project?status="+status)

	// 진행 중인 프로젝트는 open(모집 중) 과 active(진행 중) 두 가지 값을 쓴다
	list_sql := "select id, name, field, owner, target_count, done_count, date, status from baro_project"
	list_args := []any{}

	switch status {
	case "all":
	case "running":
		list_sql += " where status in ('open', 'active')"
	default:
		list_sql += " where status = ?"
		list_args = append(list_args, status)
	}

	list_sql += " order by id + 0 desc limit ?, ?"
	list_args = append(list_args, (page-1)*size, size)

	list := baro_admin_rows(db, 8, list_sql, list_args...)

	rows := [][]string{}
	risk_num := 0

	for _, one := range list {
		done, self := baro_admin_project_self_done(db, one[0], one[3])
		risk_html, risky := baro_admin_project_risk(done, self)
		if risky {
			risk_num++
		}

		target_total := tool.Str_to_int(one[4])
		done_count := tool.Str_to_int(one[5])

		progress := tool.Baro_t(`<span class="bw_muted">목표 미확정</span>`)
		if target_total > 0 {
			progress = baro_admin_esc(tool.Baro_pct_text(done_count, target_total)) + " (" + baro_admin_esc(tool.Baro_comma(done_count)) + "/" + baro_admin_esc(tool.Baro_comma(target_total)) + ")"
		}

		member_num := baro_admin_num(db, "select count(*) from baro_project_member where project = ? and status = 'active'", one[0])

		rows = append(rows, []string{
			`#` + baro_admin_esc(one[0]),
			`<a href="/baro/admin/project?id=` + tool.Url_parser(one[0]) + `">` + baro_admin_esc(one[1]) + `</a>`,
			baro_admin_project_status_badge(one[7]) + " " + baro_admin_esc(tool.Baro_field_name(db, one[2])),
			baro_admin_actor(db, config, one[3]),
			progress,
			baro_admin_esc(tool.Baro_comma(member_num)),
			risk_html,
			tool.Baro_time_cell(one[6]),
			tool.Baro_button("/baro/admin/project?id="+tool.Url_parser(one[0]), "관리", "primary"),
		})
	}

	table := tool.Baro_table(
		[]string{"번호", tool.Baro_t("이름"), tool.Baro_t("상태 · 분야"), tool.Baro_t("운영자"), tool.Baro_t("진행률"), tool.Baro_t("참여"), tool.Baro_t("점수 조작 위험"), tool.Baro_t("만든 시각"), ""},
		rows,
		tool.Baro_t("이 상태의 프로젝트가 없습니다."),
	)

	if len(rows) == 0 {
		table += tool.Baro_empty(tool.Baro_t("표시할 프로젝트가 없습니다. 다른 상태 탭을 눌러 보세요."))
	}

	table += tool.Baro_paging(page, len(rows), size, "/baro/admin/project?status="+tool.Url_parser(status)+"&page={}")

	if status == "draft" && len(rows) == 0 {
		body += tool.Baro_note(tool.Baro_t("승인 대기(draft) 프로젝트가 없습니다. '전체' 탭에서 모든 프로젝트를 볼 수 있습니다."))
	}

	if risk_num > 0 {
		body += tool.Baro_alert("warn", tool.Baro_t("점수 조작이 의심되는 프로젝트가 이 쪽에 <b>")+baro_admin_esc(tool.Baro_itoa(risk_num))+tool.Baro_t("</b>개 있습니다. 운영자가 자기 목표를 스스로 완료 처리한 비율이 높습니다."))
	}

	total_num := baro_admin_num(db, "select count(*) from baro_project")
	open_num := baro_admin_num(db, "select count(*) from baro_project where status in ('open', 'active')")
	draft_num := baro_admin_num(db, "select count(*) from baro_project where status = 'draft'")
	target_num := baro_admin_num(db, "select count(*) from baro_project_target")
	target_done := baro_admin_num(db, "select count(*) from baro_project_target where status = 'done'")

	body += tool.Baro_card(tool.Baro_t("전체 통계"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 프로젝트"), tool.Baro_comma(total_num), "baro_project"},
		{tool.Baro_t("진행 중"), tool.Baro_comma(open_num), "status = open · active"},
		{tool.Baro_t("승인 대기"), tool.Baro_comma(draft_num), "status = draft"},
		{tool.Baro_t("전체 목표"), tool.Baro_comma(target_num), "baro_project_target"},
		{tool.Baro_t("완료된 목표"), tool.Baro_comma(target_done), tool.Baro_pct_text(target_done, target_num)},
	}))

	body += tool.Baro_card_ex(tool.Baro_t("프로젝트 목록"), tool.Baro_t("이름을 눌러 관리 화면으로 이동합니다."), table, "")

	return baro_admin_page(db, config, tool.Baro_t("프로젝트 관리"), body)
}

// baro_admin_project_detail : 프로젝트 한 개 관리 화면
func baro_admin_project_detail(db *sql.DB, config tool.Config, id string) string {
	row := baro_admin_rows(
		db, 12,
		"select id, name, field, goal, owner, status, target_count, done_count, start_date, end_date, rule, summary from baro_project where id = ?",
		id,
	)

	if len(row) == 0 {
		return tool.Baro_alert("warn", tool.Baro_t("그 번호의 프로젝트가 없습니다.")) + tool.Baro_button("/baro/admin/project", tool.Baro_t("프로젝트 목록으로"), "primary")
	}

	one := row[0]
	action := "/baro/admin/project"
	owner := one[4]

	body := tool.Baro_crumb([][]string{
		{"/baro/admin", tool.Baro_t("운영 대시보드")},
		{"/baro/admin/project", tool.Baro_t("프로젝트 관리")},
		{"", one[1]},
	})

	// --- 통계 ---
	member_num := baro_admin_num(db, "select count(*) from baro_project_member where project = ? and status = 'active'", id)
	target_num := baro_admin_num(db, "select count(*) from baro_project_target where project = ?", id)
	target_done := baro_admin_num(db, "select count(*) from baro_project_target where project = ? and status = 'done'", id)
	task_num := baro_admin_num(db, "select count(*) from baro_project_task where project = ?", id)
	task_done := baro_admin_num(db, "select count(*) from baro_project_task where project = ? and status = 'done'", id)
	source_num := baro_admin_num(db, "select count(*) from baro_project_source where project = ?", id)

	stat := tool.Baro_kv_grid([][]string{
		{"상태", baro_admin_project_status_badge(one[5])},
		{"분야", baro_admin_esc(tool.Baro_field_name(db, one[2]))},
		{tool.Baro_t("운영자"), baro_admin_actor(db, config, owner)},
		{"기간", baro_admin_esc(one[8]) + " ~ " + baro_admin_esc(one[9])},
		{tool.Baro_t("목표"), baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(one[3]), 80))},
	})

	stat += tool.Baro_stat_row([][]string{
		{"참여자", tool.Baro_comma(member_num), "active"},
		{tool.Baro_t("목표"), tool.Baro_comma(target_num), tool.Baro_t("완료 ") + tool.Baro_comma(target_done)},
		{tool.Baro_t("과제"), tool.Baro_comma(task_num), tool.Baro_t("완료 ") + tool.Baro_comma(task_done)},
		{"출처", tool.Baro_comma(source_num), "baro_project_source"},
	})

	if target_num > 0 {
		stat += tool.Baro_progress(target_done, target_num, tool.Baro_t("목표 달성률"))
	} else {
		stat += tool.Baro_progress_open(target_done, tool.Str_to_int(one[6]), tool.Baro_t("기록된 완료 목표"))
	}

	body += tool.Baro_card_ex(one[1], "프로젝트 #"+id, stat, tool.Baro_button("/baro/project/view/"+tool.Url_parser(id), "이용자 화면 보기", ""))

	// --- 부정 점수 조작 감지 ---
	done, self := baro_admin_project_self_done(db, id, owner)
	risk_html, risky := baro_admin_project_risk(done, self)

	risk_body := tool.Baro_stat_row([][]string{
		{tool.Baro_t("완료 처리 건수"), tool.Baro_comma(done), tool.Baro_t("목표 + 과제")},
		{tool.Baro_t("운영자 본인 처리"), tool.Baro_comma(self), tool.Baro_pct_text(self, done)},
	})
	risk_body += "<p>" + risk_html + "</p>"

	if risky {
		risk_body += tool.Baro_alert("danger", tool.Baro_t("운영자가 자기 프로젝트 목표를 스스로 완료 처리한 비율이 높습니다. 실제 문서를 표본으로 확인하세요."))
	} else {
		risk_body += tool.Baro_alert("ok", tool.Baro_t("뚜렷한 자기 완료 처리 편중이 보이지 않습니다."))
	}

	// 짧은 시간 안에 몰아서 완료 처리한 흔적
	burst_rows := baro_admin_rows(
		db, 3,
		"select name, assignee, date from baro_project_target where project = ? and status = 'done' order by date desc limit 10",
		id,
	)

	burst_html := [][]string{}
	for _, item := range burst_rows {
		burst_html = append(burst_html, []string{
			baro_admin_esc(tool.Baro_trunc(item[0], 40)),
			baro_admin_actor(db, config, item[1]),
			tool.Baro_time_cell(item[2]),
		})
	}

	risk_body += tool.Baro_section(tool.Baro_t("최근 완료 처리"), tool.Baro_table([]string{tool.Baro_t("목표"), tool.Baro_t("담당"), "시각"}, burst_html, tool.Baro_t("완료 처리된 목표가 없습니다.")))
	risk_body += tool.Baro_note(tool.Baro_t("판단 근거일 뿐입니다. 곧바로 제재하지 말고 사건을 열어 기록을 남기세요."))

	body += tool.Baro_card_ex(tool.Baro_t("부정 점수 조작 감지"), tool.Baro_t("자기 목표를 스스로 완료 처리한 비율을 봅니다."), risk_body, "")

	// --- 목표 목록 ---
	target_rows := [][]string{}
	for _, item := range baro_admin_rows(
		db, 5,
		"select name, status, doc, assignee, date from baro_project_target where project = ? order by id + 0 desc limit 50",
		id,
	) {
		doc_html := `<span class="bw_muted">-</span>`
		if item[2] != "" {
			doc_html = tool.Baro_doc_link(item[2])
		}

		target_rows = append(target_rows, []string{
			baro_admin_esc(tool.Baro_trunc(item[0], 50)),
			baro_admin_status_badge(item[1]),
			doc_html,
			baro_admin_actor(db, config, item[3]),
			tool.Baro_time_cell(item[4]),
		})
	}

	body += tool.Baro_card_ex(
		"목표 목록",
		tool.Baro_t("baro_project_target 표를 직접 읽습니다. (최근 50개)"),
		tool.Baro_table([]string{tool.Baro_t("목표"), "상태", "문서", tool.Baro_t("담당"), tool.Baro_t("등록")}, target_rows, tool.Baro_t("등록된 목표가 없습니다.")),
		"",
	)

	// --- 참여자 ---
	member_rows := [][]string{}
	for _, item := range baro_admin_rows(
		db, 4,
		"select user, role, date, status from baro_project_member where project = ? order by id + 0 asc limit 50",
		id,
	) {
		member_rows = append(member_rows, []string{
			tool.Baro_user_link(db, item[0], config.IP),
			baro_admin_esc(item[1]),
			tool.Baro_time_cell(item[2]),
			baro_admin_status_badge(item[3]),
		})
	}

	body += tool.Baro_card_ex(
		"참여자",
		tool.Baro_t("baro_project_member 표를 직접 읽습니다."),
		tool.Baro_table([]string{tool.Baro_t("사용자"), "역할", tool.Baro_t("참여 시각"), "상태"}, member_rows, tool.Baro_t("참여자가 없습니다.")),
		"",
	)

	// --- 조치 ---
	owner_form := tool.Baro_hidden("do", "owner")
	owner_form += tool.Baro_hidden("id", id)
	owner_form += tool.Baro_input("owner", tool.Baro_t("운영자"), owner, tool.Baro_t("계정 이름"), tool.Baro_t("지정되면 참여자 목록에도 운영자로 들어갑니다."))
	owner_form += tool.Baro_input("reason", "사유", "", tool.Baro_t("왜 바꾸는지"), "")
	owner_form += tool.Baro_submit(tool.Baro_t("운영자 지정"), "primary")

	approve_form := tool.Baro_hidden("do", "approve")
	approve_form += tool.Baro_hidden("id", id)
	approve_form += tool.Baro_input("reason", tool.Baro_t("승인 사유"), "", tool.Baro_t("검토 결과"), "")
	approve_form += tool.Baro_submit(tool.Baro_t("승인 (draft → open)"), "primary")

	reject_form := tool.Baro_hidden("do", "reject")
	reject_form += tool.Baro_hidden("id", id)
	reject_form += tool.Baro_input("reason", tool.Baro_t("반려 사유 (필수)"), "", tool.Baro_t("무엇을 고쳐야 하는지"), "")
	reject_form += tool.Baro_submit(tool.Baro_t("반려"), "danger")

	finish_form := tool.Baro_hidden("do", "finish")
	finish_form += tool.Baro_hidden("id", id)
	finish_form += tool.Baro_textarea("report", tool.Baro_t("결과 보고"), "", 4, tool.Baro_t("무엇을 얼마나 만들었는지"), tool.Baro_t("프로젝트 화면에 공개됩니다."))
	finish_form += tool.Baro_input("reason", "사유", "", "", "")
	finish_form += tool.Baro_submit(tool.Baro_t("종료 처리"), "")

	archive_form := tool.Baro_hidden("do", "archive")
	archive_form += tool.Baro_hidden("id", id)
	archive_form += tool.Baro_input("reason", tool.Baro_t("보관 사유"), "", "", "")
	archive_form += tool.Baro_submit(tool.Baro_t("보관"), "")

	reopen_form := tool.Baro_hidden("do", "reopen")
	reopen_form += tool.Baro_hidden("id", id)
	reopen_form += tool.Baro_input("reason", tool.Baro_t("재개 사유"), "", "", "")
	reopen_form += tool.Baro_submit(tool.Baro_t("다시 열기"), "")

	status_form := tool.Baro_hidden("do", "status")
	status_form += tool.Baro_hidden("id", id)
	status_form += tool.Baro_select("status", tool.Baro_t("진행 상태"), [][]string{
		{"open", tool.Baro_t("모집 중")},
		{"active", tool.Baro_t("진행 중")},
		{"pending", tool.Baro_t("잠시 멈춤")},
	}, one[5], tool.Baro_t("프로젝트 화면에 그대로 반영됩니다."))
	status_form += tool.Baro_input("reason", "사유", "", "", "")
	status_form += tool.Baro_submit(tool.Baro_t("진행 상태 저장"), "primary")

	body += tool.Baro_card_ex(tool.Baro_t("운영 조치"), tool.Baro_t("모든 조치는 공개 운영 로그와 감사 로그에 남습니다."), tool.Baro_grid([]string{
		tool.Baro_section("승인", tool.Baro_form(action, approve_form)),
		tool.Baro_section(tool.Baro_t("진행 상태"), tool.Baro_form(action, status_form)),
		tool.Baro_section(tool.Baro_t("반려"), tool.Baro_form(action, reject_form)),
		tool.Baro_section(tool.Baro_t("운영자 지정"), tool.Baro_form(action, owner_form)),
		tool.Baro_section(tool.Baro_t("종료"), tool.Baro_form(action, finish_form)),
		tool.Baro_section(tool.Baro_t("보관"), tool.Baro_form(action, archive_form)),
		tool.Baro_section(tool.Baro_t("다시 열기"), tool.Baro_form(action, reopen_form)),
	}, 3), "")

	if strings.TrimSpace(one[10]) != "" {
		body += tool.Baro_card(tool.Baro_t("프로젝트 규칙"), tool.Baro_code(tool.Baro_trunc(one[10], 1500)))
	}

	return body
}
