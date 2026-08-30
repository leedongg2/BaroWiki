package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  16-7 어워드 관리
//
//  후보 · 자격 검증 · 투표 현황 · 심사 진행 · 결과 발표 · 기록관
// ============================================================

// baro_admin_award_status_list : 어워드 진행 단계
func baro_admin_award_status_list() [][]string {
	return [][]string{
		{"draft", tool.Baro_t("준비 중")},
		{"open", tool.Baro_t("후보 접수")},
		{"review", tool.Baro_t("자격 검증")},
		{"active", tool.Baro_t("투표 진행")},
		{"closed", tool.Baro_t("발표 완료")},
	}
}

// baro_admin_award_status_name : 진행 단계 이름
func baro_admin_award_status_name(status string) string {
	for _, row := range baro_admin_award_status_list() {
		if row[0] == status {
			return row[1]
		}
	}

	if status == "" {
		return tool.Baro_t("미정")
	}

	return status
}

// baro_admin_award_vote_sum : 후보 한 명의 득표 (표 수, 가중치 합)
func baro_admin_award_vote_sum(db *sql.DB, nomination string) (int, int) {
	num := 0
	weight := 0

	for _, one := range baro_admin_rows(db, 1, "select weight from baro_award_vote where nomination = ? limit 5000", nomination) {
		num++

		value := tool.Str_to_int(one[0])
		if value <= 0 {
			value = 1
		}

		weight += value
	}

	return num, weight
}

// baro_admin_award_check : 후보 자격 검증
//
//	돌려주는 값 : (안내 HTML, 자격 있음)
func baro_admin_award_check(db *sql.DB, target_kind string, target string) (string, bool) {
	if target == "" {
		return tool.Baro_badge(tool.Baro_t("대상 없음"), "danger"), false
	}

	if target_kind == "doc" || target_kind == "document" {
		if baro_admin_one(db, "select title from data where title = ? limit 1", target) == "" {
			return tool.Baro_badge(tool.Baro_t("문서 없음"), "danger"), false
		}

		if tool.Baro_doc_quarantined(db, target) {
			return tool.Baro_badge(tool.Baro_t("격리 중"), "danger"), false
		}

		return tool.Baro_badge(tool.Baro_t("자격 있음"), "ok"), true
	}

	if !baro_admin_user_exists(db, target) {
		return tool.Baro_badge(tool.Baro_t("계정 없음"), "danger"), false
	}

	// 활동 중인 제재가 있으면 후보 자격이 없다
	if len(tool.Baro_sanction_active(db, target)) > 0 {
		return tool.Baro_badge(tool.Baro_t("제재 진행 중"), "danger"), false
	}

	contrib := baro_admin_num(db, "select count(*) from baro_contrib_event where user = ?", target)
	if contrib == 0 {
		return tool.Baro_badge(tool.Baro_t("기여 기록 없음"), "warn"), false
	}

	return tool.Baro_badge(tool.Baro_t("자격 있음 (기여 ")+tool.Baro_comma(contrib)+")", "ok"), true
}

// baro_admin_award_post : 어워드 관리 쓰기 동작
func baro_admin_award_post(db *sql.DB, config tool.Config, values url.Values) (string, string) {
	do := baro_admin_get(values, "do")
	id := baro_admin_get(values, "id")
	reason := baro_admin_get(values, "reason")

	switch do {
	case "create":
		name := baro_admin_get(values, "name")
		if name == "" {
			return "", tool.Baro_alert("warn", tool.Baro_t("어워드 이름을 넣어 주세요."))
		}

		new_id := tool.Baro_next_id(db, "baro_award")
		tool.Exec_DB(
			db,
			"insert into baro_award (id, season, category, name, description, status, date, result_reason, winner) values (?, ?, ?, ?, ?, 'draft', ?, '', '')",
			new_id, baro_admin_get(values, "season"), baro_admin_get(values, "category"), name, baro_admin_get(values, "description"), tool.Baro_now(),
		)

		baro_admin_log(db, config, "award_create", "award:"+new_id, reason, "", name)

		return tool.Get_redirect("/baro/admin/award?id=" + tool.Url_parser(new_id)), ""

	case "status":
		status := baro_admin_get(values, "status")
		before := baro_admin_one(db, "select status from baro_award where id = ?", id)

		tool.Exec_DB(db, "update baro_award set status = ? where id = ?", status, id)
		baro_admin_log(db, config, "award_status", "award:"+id, reason, before, status)

		return tool.Get_redirect("/baro/admin/award?id=" + tool.Url_parser(id) + "&done=status"), ""

	case "verify":
		nomination := baro_admin_get(values, "nomination")
		status := baro_admin_get(values, "status")

		switch status {
		case "open", "pending", "confirmed", "rejected":
		default:
			return "", tool.Baro_alert("warn", tool.Baro_t("후보 상태 값이 올바르지 않습니다."))
		}

		if len(baro_admin_rows(db, 1, "select id from baro_nomination where id = ?", nomination)) == 0 {
			return "", tool.Baro_alert("warn", tool.Baro_t("그 번호의 후보가 없습니다."))
		}

		tool.Exec_DB(db, "update baro_nomination set status = ? where id = ?", status, nomination)
		baro_admin_log(db, config, "award_verify", "nomination:"+nomination, reason, "", status)

		return tool.Get_redirect("/baro/admin/award?id=" + tool.Url_parser(id) + "&done=verify"), ""

	case "announce":
		winner := baro_admin_get(values, "winner")
		if winner == "" || reason == "" {
			return "", tool.Baro_alert("warn", tool.Baro_t("수상자와 선정 사유를 모두 넣어야 발표할 수 있습니다."))
		}

		tool.Exec_DB(
			db,
			"update baro_award set winner = ?, result_reason = ?, status = 'closed' where id = ?",
			winner, reason, id,
		)

		name := baro_admin_one(db, "select name from baro_award where id = ?", id)

		tool.Baro_public_log(db, "award_announce", config.IP, "award:"+id, reason, winner)
		tool.Baro_audit(db, config, "award_announce", "award:"+id, "", winner, reason)

		if tool.Baro_is_user(winner) {
			tool.Baro_notify(db, winner, "award", name+" 수상자로 선정되었습니다.", "/baro/award", reason)
		}

		return tool.Get_redirect("/baro/admin/award?id=" + tool.Url_parser(id) + "&done=announce"), ""
	}

	return "", tool.Baro_alert("warn", tool.Baro_t("알 수 없는 동작입니다."))
}

// View_baro_admin_award : 16-7 어워드 관리
func View_baro_admin_award(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if html, stop := baro_admin_gate(db, config, "award"); stop {
		return html
	}

	notice := ""
	if values != nil {
		redirect, message := baro_admin_award_post(db, config, values)
		if redirect != "" {
			return redirect
		}

		notice = message
	}

	body := tool.Baro_hero(
		tool.Baro_t("어워드 관리"),
		tool.Baro_t("후보 접수 · 자격 검증 · 투표 · 심사 · 결과 발표 · 기록관"),
		tool.Baro_button("/baro/award", tool.Baro_t("어워드 화면"), "")+" "+tool.Baro_button("/baro/wrapped", "Wrapped", ""),
	)
	body += baro_admin_tabs("/baro/admin/award")

	if notice != "" {
		body += notice
	}
	if done := baro_admin_get(query, "done"); done != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("처리했습니다. (")+baro_admin_esc(done)+")")
	}

	if id := baro_admin_get(query, "id"); id != "" {
		body += baro_admin_award_detail(db, config, id)

		return baro_admin_page(db, config, tool.Baro_t("어워드 관리"), body)
	}

	page := baro_admin_page_num(query)
	size := tool.Baro_page_size

	list := baro_admin_rows(
		db, 7,
		"select id, season, category, name, status, winner, date from baro_award order by id + 0 desc limit ?, ?",
		(page-1)*size, size,
	)

	rows := [][]string{}
	for _, one := range list {
		nomination_num := baro_admin_num(db, "select count(*) from baro_nomination where award = ?", one[0])
		vote_num := baro_admin_num(db, "select count(*) from baro_award_vote where award = ?", one[0])

		winner := tool.Baro_t(`<span class="bw_muted">미정</span>`)
		if one[5] != "" {
			winner = tool.Baro_badge(one[5], "ok")
		}

		rows = append(rows, []string{
			`#` + baro_admin_esc(one[0]),
			baro_admin_esc(one[1]),
			baro_admin_esc(one[2]),
			`<a href="/baro/admin/award?id=` + tool.Url_parser(one[0]) + `">` + baro_admin_esc(one[3]) + `</a>`,
			tool.Baro_badge(baro_admin_award_status_name(one[4]), "info"),
			baro_admin_esc(tool.Baro_comma(nomination_num)),
			baro_admin_esc(tool.Baro_comma(vote_num)),
			winner,
			tool.Baro_time_cell(one[6]),
		})
	}

	table := tool.Baro_table(
		[]string{"번호", tool.Baro_t("시즌"), "부문", tool.Baro_t("이름"), tool.Baro_t("진행"), tool.Baro_t("후보"), tool.Baro_t("투표"), tool.Baro_t("수상자"), tool.Baro_t("만든 시각")},
		rows,
		tool.Baro_t("등록된 어워드가 없습니다."),
	)

	if len(rows) == 0 {
		table += tool.Baro_empty(tool.Baro_t("아직 어워드가 없습니다. 아래에서 새로 만드세요."))
	}

	table += tool.Baro_paging(page, len(rows), size, "/baro/admin/award?page={}")

	body += tool.Baro_card_ex(tool.Baro_t("어워드 목록"), tool.Baro_t("이름을 눌러 관리 화면으로 이동합니다."), table, "")

	// --- 새 어워드 ---
	create_form := tool.Baro_hidden("do", "create")
	create_form += tool.Baro_input("season", tool.Baro_t("시즌"), tool.Get_slice(tool.Baro_today(), 0, 4), tool.Baro_t("예: 2026"), "")
	create_form += tool.Baro_input("category", "부문", "", tool.Baro_t("예: 올해의 위키인 / 올해의 문서"), "")
	create_form += tool.Baro_input("name", tool.Baro_t("이름 (필수)"), "", tool.Baro_t("예: 2026 올해의 위키인"), "")
	create_form += tool.Baro_textarea("description", tool.Baro_t("설명"), "", 3, tool.Baro_t("선정 기준과 절차"), tool.Baro_t("공개 화면에 나옵니다."))
	create_form += tool.Baro_input("reason", "사유", "", "", "")
	create_form += tool.Baro_submit(tool.Baro_t("어워드 만들기"), "primary")

	body += tool.Baro_card_ex(tool.Baro_t("새 어워드"), tool.Baro_t("만들면 '준비 중' 상태로 시작합니다."), tool.Baro_form("/baro/admin/award", create_form), "")

	// --- 기록관 ---
	archive_rows := [][]string{}
	for _, one := range baro_admin_rows(
		db, 5,
		"select id, season, name, winner, result_reason from baro_award where status = 'closed' order by id + 0 desc limit 30",
	) {
		archive_rows = append(archive_rows, []string{
			baro_admin_esc(one[1]),
			`<a href="/baro/admin/award?id=` + tool.Url_parser(one[0]) + `">` + baro_admin_esc(one[2]) + `</a>`,
			tool.Baro_badge(one[3], "ok"),
			baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(one[4]), 60)),
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("기록관"),
		tool.Baro_t("발표가 끝난 어워드입니다. 선정 사유를 함께 남겨 근거를 공개합니다."),
		tool.Baro_table([]string{tool.Baro_t("시즌"), tool.Baro_t("어워드"), tool.Baro_t("수상자"), tool.Baro_t("선정 사유")}, archive_rows, tool.Baro_t("발표된 어워드가 없습니다.")),
		tool.Baro_button("/baro/award", tool.Baro_t("공개 기록관 보기"), ""),
	)

	return baro_admin_page(db, config, tool.Baro_t("어워드 관리"), body)
}

// baro_admin_award_detail : 어워드 한 개 관리 화면
func baro_admin_award_detail(db *sql.DB, config tool.Config, id string) string {
	row := baro_admin_rows(
		db, 8,
		"select id, season, category, name, description, status, winner, result_reason from baro_award where id = ?",
		id,
	)

	if len(row) == 0 {
		return tool.Baro_alert("warn", tool.Baro_t("그 번호의 어워드가 없습니다.")) + tool.Baro_button("/baro/admin/award", tool.Baro_t("어워드 목록으로"), "primary")
	}

	one := row[0]
	action := "/baro/admin/award"

	body := tool.Baro_crumb([][]string{
		{"/baro/admin", tool.Baro_t("운영 대시보드")},
		{"/baro/admin/award", tool.Baro_t("어워드 관리")},
		{"", one[3]},
	})

	info := tool.Baro_kv_grid([][]string{
		{tool.Baro_t("시즌"), baro_admin_esc(one[1])},
		{"부문", baro_admin_esc(one[2])},
		{tool.Baro_t("진행 단계"), tool.Baro_badge(baro_admin_award_status_name(one[5]), "info")},
		{tool.Baro_t("수상자"), func() string {
			if one[6] == "" {
				return tool.Baro_t(`<span class="bw_muted">미정</span>`)
			}

			return tool.Baro_user_link(db, one[6], config.IP)
		}()},
	})

	if strings.TrimSpace(one[4]) != "" {
		info += tool.Baro_section(tool.Baro_t("설명"), tool.Baro_code(tool.Baro_trunc(one[4], 800)))
	}
	if strings.TrimSpace(one[7]) != "" {
		info += tool.Baro_section(tool.Baro_t("선정 사유"), tool.Baro_code(tool.Baro_trunc(one[7], 800)))
	}

	body += tool.Baro_card_ex(one[3], tool.Baro_t("어워드 #")+id, info, "")

	// --- 후보 목록 · 자격 검증 · 투표 현황 ---
	nomination_rows := [][]string{}
	total_vote := 0
	ok_num := 0

	for _, item := range baro_admin_rows(
		db, 7,
		"select id, target_kind, target, nominator, status, reason, date from baro_nomination where award = ? order by id + 0 asc limit 200",
		id,
	) {
		check_html, ok := baro_admin_award_check(db, item[1], item[2])
		if ok {
			ok_num++
		}

		vote_num, vote_weight := baro_admin_award_vote_sum(db, item[0])
		total_vote += vote_num

		verify_form := tool.Baro_hidden("do", "verify")
		verify_form += tool.Baro_hidden("id", id)
		verify_form += tool.Baro_hidden("nomination", item[0])
		verify_form += tool.Baro_select("status", "", [][]string{
			{"open", tool.Baro_t("접수됨")},
			{"pending", tool.Baro_t("검증 대기")},
			{"confirmed", tool.Baro_t("자격 확인")},
			{"rejected", tool.Baro_t("자격 미달")},
		}, item[4], "")
		verify_form += tool.Baro_hidden("reason", tool.Baro_t("자격 검증"))
		verify_form += tool.Baro_submit("저장", "")

		nomination_rows = append(nomination_rows, []string{
			`#` + baro_admin_esc(item[0]),
			baro_admin_case_target_link(db, config, item[1], item[2]),
			baro_admin_actor(db, config, item[3]),
			check_html,
			baro_admin_status_badge(item[4]),
			baro_admin_esc(tool.Baro_comma(vote_num)) + " (" + baro_admin_esc(tool.Baro_comma(vote_weight)) + ")",
			baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(item[5]), 30)),
			tool.Baro_form(action, verify_form),
		})
	}

	nomination_body := tool.Baro_stat_row([][]string{
		{tool.Baro_t("후보"), tool.Baro_comma(len(nomination_rows)), "baro_nomination"},
		{tool.Baro_t("자격 통과"), tool.Baro_comma(ok_num), tool.Baro_pct_text(ok_num, len(nomination_rows))},
		{tool.Baro_t("투표 수"), tool.Baro_comma(total_vote), "baro_award_vote"},
	})

	nomination_body += tool.Baro_table(
		[]string{"번호", tool.Baro_t("후보"), tool.Baro_t("추천자"), tool.Baro_t("자격 검증"), "상태", tool.Baro_t("득표 (가중치)"), tool.Baro_t("추천 사유"), "심사"},
		nomination_rows,
		tool.Baro_t("등록된 후보가 없습니다."),
	)

	if len(nomination_rows) == 0 {
		nomination_body += tool.Baro_empty(tool.Baro_t("후보가 아직 없습니다. 후보 접수 단계로 바꾸면 이용자가 추천할 수 있습니다."))
	}

	nomination_body += tool.Baro_note(tool.Baro_t("자격 검증은 계정 존재 · 활동 중인 제재 · 기여 기록을 자동으로 확인한 결과입니다. 최종 판단은 사람이 합니다."))

	body += tool.Baro_card_ex(tool.Baro_t("후보 · 자격 검증 · 투표 현황"), tool.Baro_t("baro_nomination · baro_award_vote 표를 직접 읽습니다."), nomination_body, "")

	// --- 심사 진행 / 결과 발표 ---
	status_form := tool.Baro_hidden("do", "status")
	status_form += tool.Baro_hidden("id", id)
	status_form += tool.Baro_select("status", tool.Baro_t("진행 단계"), baro_admin_award_status_list(), one[5], tool.Baro_t("준비 → 후보 접수 → 자격 검증 → 투표 → 발표"))
	status_form += tool.Baro_input("reason", "사유", "", "", "")
	status_form += tool.Baro_submit(tool.Baro_t("단계 저장"), "primary")

	announce_form := tool.Baro_hidden("do", "announce")
	announce_form += tool.Baro_hidden("id", id)
	announce_form += tool.Baro_input("winner", tool.Baro_t("수상자 (필수)"), one[6], tool.Baro_t("계정 이름 또는 문서 이름"), tool.Baro_t("발표하면 알림이 갑니다."))
	announce_form += tool.Baro_textarea("reason", tool.Baro_t("선정 사유 (필수)"), one[7], 4, tool.Baro_t("어떤 근거로 뽑았는지"), tool.Baro_t("공개 로그에 그대로 남습니다."))
	announce_form += tool.Baro_submit(tool.Baro_t("결과 발표"), "primary")

	body += tool.Baro_grid([]string{
		tool.Baro_card(tool.Baro_t("심사 진행"), tool.Baro_form(action, status_form)),
		tool.Baro_card(tool.Baro_t("결과 발표"), tool.Baro_form(action, announce_form)),
	}, 2)

	return body
}
