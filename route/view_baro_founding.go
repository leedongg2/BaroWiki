package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  1-2 100인 창립단
//    /baro/founding/apply  지원
//    /baro/founding/check  내 지원 상태와 다음 할 일
//    /baro/founding/list   창립단 명단
//    /baro/founding/hall   창립 시즌 기록관
// ============================================================

// baro_founding_apply_do : 창립단 지원 처리 (분야별 모집 화면에서도 함께 쓴다)
//
//	반환값 : 사용자에게 보여줄 안내 (성공하면 "")
func baro_founding_apply_do(db *sql.DB, config tool.Config, field string, note string) string {
	if tool.Baro_read_only(db) {
		return tool.Baro_t("지금은 읽기 전용 모드입니다. ") + tool.Baro_read_only_reason(db)
	}

	if !baro_founding_open(db) {
		return tool.Baro_t("창립 기여자 지원이 닫혀 있습니다.")
	}

	if !tool.Baro_rate_ok(db, "founding_apply", config.IP, 10, 600) {
		return tool.Baro_t("짧은 시간에 너무 많이 시도했습니다. 잠시 뒤에 다시 시도해 주세요.")
	}

	if !baro_founding_field_ok(db, field) {
		return tool.Baro_t("관심 분야를 골라 주세요.")
	}

	if _, found := baro_founding_find(db, config.IP); found {
		return tool.Baro_t("이미 지원한 기록이 있습니다.")
	}

	tool.Exec_DB(
		db,
		"insert into baro_founder (user, number, date, status, first_event, field, apply_date, note) values (?, '', '', 'applied', '', ?, ?, ?)",
		config.IP, field, tool.Baro_now(), tool.Baro_trunc(tool.Baro_one_line(note), 300),
	)

	// 지원한 분야는 관심 분야에도 함께 넣어 첫 화면 추천에 반영한다.
	baro_founding_interest_add(db, config.IP, field, "apply")

	tool.Baro_public_log(
		db, "founder_apply", config.IP, config.IP,
		"창립 기여자 지원", "분야 : "+tool.Baro_field_name(db, field),
	)
	tool.Baro_audit(db, config, "founder_apply", config.IP, "", "field="+field, "창립 기여자 지원")
	tool.Baro_notify(
		db, config.IP, "system",
		tool.Baro_t("창립 기여자 지원이 접수되었습니다. 첫 유효 기여를 마치면 번호가 확정됩니다."),
		"/baro/founding/check", "founder_apply",
	)
	tool.Baro_metric_bump(db, "founder_apply", tool.Baro_today(), 1)

	return ""
}

// baro_founding_seat_html : 정원 현황 표시
func baro_founding_seat_html(db *sql.DB) string {
	goal := baro_founding_goal(db)
	confirmed := baro_founding_status_count(db, "confirmed")
	applied := baro_founding_status_count(db, "applied")

	left := goal - confirmed
	if left < 0 {
		left = 0
	}

	html := tool.Baro_stat_row([][]string{
		{tool.Baro_t("확정된 창립 기여자"), tool.Baro_comma(confirmed) + tool.Baro_t("명"), tool.Baro_t("번호 발급 완료")},
		{tool.Baro_t("남은 자리"), tool.Baro_comma(left) + tool.Baro_t("명"), "정원 " + tool.Baro_comma(goal) + tool.Baro_t("명")},
		{tool.Baro_t("번호 대기"), tool.Baro_comma(applied) + tool.Baro_t("명"), tool.Baro_t("첫 유효 기여 대기 중")},
	})
	html += tool.Baro_progress(confirmed, goal, tool.Baro_t("창립단 확정"))

	if left == 0 {
		html += tool.Baro_alert("warn", tool.Baro_t("정원이 모두 찼습니다. 새 지원은 대기 상태로 접수되며, 자리가 생기면 순서대로 번호가 발급됩니다."))
	} else if next := baro_founding_max_number(db) + 1; next > goal {
		// 번호를 취소한 자리가 있어도 다음 번호는 언제나 "최대 번호 + 1" 이라 정원을 넘을 수 있다.
		html += tool.Baro_alert("warn",
			tool.Baro_t("다음 번호 #")+tool.Baro_itoa(next)+tool.Baro_t(" 가 정원(")+tool.Baro_itoa(goal)+tool.Baro_t("명)을 넘어 지금은 새 번호를 발급하지 않습니다. ")+
				tool.Baro_t("지원 기록과 기여 기록은 그대로 보존되며, 정원을 늘리면 순서대로 발급됩니다."))
	}

	return html
}

// View_baro_founding_apply : 창립 기여자 지원 (1-2)
func View_baro_founding_apply(config tool.Config, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/founding/apply"); need {
		return wall
	}

	message := ""
	if values != nil {
		if strings.TrimSpace(values.Get("agree")) == "" {
			message = tool.Baro_t("창립 기여자 규칙에 동의해야 지원할 수 있습니다.")
		} else {
			message = baro_founding_apply_do(db, config, values.Get("field"), values.Get("note"))
		}

		if message == "" {
			return tool.Get_redirect("/baro/founding/check")
		}
	}

	person, found := baro_founding_find(db, config.IP)

	// 분야 기본값 : 주소로 넘어온 분야가 먼저, 없으면 저장해 둔 관심 분야를 쓴다.
	// (/baro/interest 안내대로 "고른 분야가 기본값으로 채워집니다"를 실제로 지킨다)
	selected := ""
	if query != nil {
		selected = strings.TrimSpace(query.Get("field"))
	}
	if !baro_founding_field_ok(db, selected) {
		selected = ""
	}

	from_interest := false
	if selected == "" {
		for _, key := range tool.Baro_user_interest(db, config.IP) {
			if baro_founding_field_ok(db, key) {
				selected = key
				from_interest = true

				break
			}
		}
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/founding", tool.Baro_t("창립 시즌")},
		{"", tool.Baro_t("창립단 지원")},
	})
	body += tool.Baro_hero(
		tool.Baro_t("100인 창립단 지원"),
		tool.Baro_t("바로위키를 처음부터 함께 만드는 사람을 찾습니다."),
		"",
	)
	body += baro_founding_tabs("/baro/founding/apply")

	if message != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(message))
	}

	if selected != "" && !found {
		if from_interest {
			body += tool.Baro_alert("info",
				tool.Baro_t("관심 분야로 저장해 둔 <b>")+tool.HTML_escape(tool.Baro_field_name(db, selected))+
					tool.Baro_t("</b> 을(를) 기본값으로 채워 두었습니다. 다른 분야로 바꿔도 됩니다. ")+
					tool.Baro_t(`<a href="/baro/interest">관심 분야 바꾸기</a>`))
		} else {
			body += tool.Baro_alert("info", tool.Baro_t("고른 분야로 지원 내용을 미리 채워 두었습니다. 내용을 확인하고 지원해 주세요."))
		}
	}

	body += baro_founding_seat_html(db)

	if found {
		// 이미 지원한 사람에게는 폼 대신 현재 상태를 보여준다. (중복 지원 방지)
		state := baro_founding_state_html(person)

		body += tool.Baro_card_ex(
			tool.Baro_t("이미 지원했습니다"),
			tool.Baro_t("한 계정은 한 번만 지원할 수 있습니다."),
			tool.Baro_kv_grid([][]string{
				{"상태", state},
				{tool.Baro_t("관심 분야"), tool.HTML_escape(tool.Baro_field_name(db, person.Field))},
				{tool.Baro_t("지원일"), tool.Baro_time_cell(person.ApplyDate)},
				{tool.Baro_t("첫 기여"), baro_founding_event_text(db, person.FirstEvent)},
			}),
			tool.Baro_button("/baro/founding/check", tool.Baro_t("내 지원 상태 보기"), "primary")+" "+
				tool.Baro_button("/baro/founding/list", tool.Baro_t("창립단 명단"), ""),
		)
	} else if !baro_founding_open(db) {
		body += tool.Baro_alert("warn", tool.Baro_t("지금은 창립 기여자 지원을 받지 않습니다."))
	} else {
		field_help := tool.Baro_t("고른 분야는 관심 분야에도 함께 저장되어 추천 과제에 반영됩니다.")
		if from_interest {
			field_help = tool.Baro_t("저장해 둔 관심 분야가 기본으로 골라져 있습니다. 고른 분야는 관심 분야에도 함께 저장되어 추천 과제에 반영됩니다.")
		}

		form := tool.Baro_select("field", tool.Baro_t("관심 분야"), baro_founding_field_options(db, tool.Baro_t("분야를 고르세요")), selected, field_help)
		form += tool.Baro_textarea("note", tool.Baro_t("함께 만들고 싶은 것 (선택)"), "", 4, tool.Baro_t("예 : 지역 신문 기사를 근거로 우리 지역 인물 문서를 만들고 싶습니다."), tool.Baro_t("운영자가 지원 내용을 확인할 때 참고합니다."))
		form += tool.Baro_checkbox("agree", tool.Baro_t("창립 기여자 규칙에 동의합니다."), "on", false, tool.Baro_t("번호는 가입 순서가 아니라 첫 유효 기여가 확인된 순서로 확정됩니다."))
		form += tool.Baro_submit(tool.Baro_t("창립 기여자로 지원하기"), "primary")

		body += tool.Baro_card(tool.Baro_t("창립 기여자 지원서"), tool.Baro_form("/baro/founding/apply", form))
	}

	body += tool.Baro_card(tool.Baro_t("번호는 언제 확정되나요?"), baro_founding_rule_html())
	body += tool.Baro_card(tool.Baro_t("창립 기여자 혜택"), baro_founding_benefit_html())
	body += tool.Baro_card(
		tool.Baro_t("다음 단계"),
		tool.Baro_button("/baro/interest", tool.Baro_t("관심 분야 고르기"), "")+" "+
			tool.Baro_button("/baro/founding/field", tool.Baro_t("분야별 모집 보기"), "")+" "+
			tool.Baro_button("/baro/founding/hall", tool.Baro_t("창립 시즌 기록관"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("창립단 지원"), body, baro_founding_menu())
}

// View_baro_founding_check : 내 지원 상태와 다음 할 일 (1-2)
func View_baro_founding_check(config tool.Config, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/founding/check"); need {
		return wall
	}

	// 이 화면에 들어올 때마다 승격 검사를 한다.
	given := baro_founding_promote(db, config.IP)

	person, found := baro_founding_find(db, config.IP)
	event := baro_founding_first_valid(db, config.IP)
	interest := tool.Baro_user_interest(db, config.IP)

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/founding", tool.Baro_t("창립 시즌")},
		{"", tool.Baro_t("내 지원 상태")},
	})
	body += tool.Baro_hero(tool.Baro_t("내 창립단 지원 상태"), tool.Baro_t("번호가 확정되기까지 남은 단계를 확인하세요."), "")
	body += baro_founding_tabs("/baro/founding/check")

	if given != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("축하합니다. 창립 기여자 번호 <b>#")+tool.HTML_escape(given)+tool.Baro_t("</b> 이(가) 확정되었습니다."))
	}

	if !found {
		body += tool.Baro_alert("info", tool.Baro_t("아직 창립 기여자로 지원하지 않았습니다."))
		body += tool.Baro_card(
			tool.Baro_t("먼저 지원해 주세요"),
			tool.Baro_t(`<p>창립 기여자는 지원 → 첫 유효 기여 → 번호 확정 순서로 진행됩니다.</p>`)+
				tool.Baro_button("/baro/founding/apply", tool.Baro_t("창립 기여자로 지원하기"), "primary")+" "+
				tool.Baro_button("/baro/founding/field", tool.Baro_t("분야별 모집 보기"), ""),
		)
		body += tool.Baro_card(tool.Baro_t("번호 확정 규칙"), baro_founding_rule_html())
		body += tool.Baro_card(tool.Baro_t("창립 기여자 혜택"), baro_founding_benefit_html())

		return tool.Baro_page(db, config, tool.Baro_t("내 지원 상태"), body, baro_founding_menu())
	}

	state_html := baro_founding_state_html(person)

	body += tool.Baro_card(
		tool.Baro_t("내 지원 정보"),
		tool.Baro_kv_grid([][]string{
			{"상태", state_html},
			{tool.Baro_t("관심 분야"), tool.HTML_escape(tool.Baro_field_name(db, person.Field))},
			{tool.Baro_t("지원일"), tool.Baro_time_cell(person.ApplyDate)},
			{tool.Baro_t("번호 확정일"), tool.Baro_time_cell(person.Date)},
			{tool.Baro_t("첫 기여"), baro_founding_event_text(db, person.FirstEvent)},
		}),
	)

	// --- 다음 할 일 ---
	steps := []string{}

	step_done := func(done bool, text string, link string, link_text string) string {
		mark := tool.Baro_badge(tool.Baro_t("해야 할 일"), "warn")
		if done {
			mark = tool.Baro_badge(tool.Baro_t("완료"), "ok")
		}

		html := mark + " " + text
		if !done && link != "" {
			html += " " + tool.Baro_button(link, link_text, "")
		}

		return html
	}

	steps = append(steps, step_done(true, tool.Baro_t("창립 기여자로 지원하기"), "", ""))
	steps = append(steps, step_done(len(interest) > 0, tool.Baro_t("관심 분야 고르기 — 첫 화면과 추천 과제가 개인화됩니다."), "/baro/interest", tool.Baro_t("관심 분야 고르기")))
	steps = append(steps, step_done(event != nil, "첫 유효 기여 남기기 — 새 문서 작성 또는 출처 추가", "/baro/task", "할 일 찾아보기"))
	steps = append(steps, step_done(person.Status == "confirmed", "창립 기여자 번호 확정", "", ""))

	body += tool.Baro_card(tool.Baro_t("다음 할 일"), tool.Baro_list(steps))

	if person.Status == "applied" {
		if event == nil {
			body += tool.Baro_alert("info",
				tool.Baro_t(`아직 유효 기여가 확인되지 않았습니다. <b>새 문서 작성</b> 또는 <b>출처 추가</b> 를 한 건이라도 남기면 이 화면에서 바로 번호가 발급됩니다.`))
		} else if baro_founding_max_number(db)+1 > baro_founding_goal(db) {
			body += tool.Baro_alert("warn",
				tool.Baro_t(`유효 기여는 확인되었지만 <b>정원이 모두 찼습니다</b>. 자리가 생기면 순서대로 번호가 발급됩니다. 기여 기록은 그대로 보존됩니다.`))
		} else {
			body += tool.Baro_alert("info", tool.Baro_t("유효 기여가 확인되었습니다. 잠시 뒤 번호가 발급됩니다."))
		}
	}

	if person.Status == "pending" {
		reason := strings.TrimSpace(person.Note)
		if reason == "" {
			reason = tool.Baro_t("사유가 기록되지 않았습니다.")
		}

		body += tool.Baro_alert("warn",
			tool.Baro_t("운영자 확인을 위해 번호 발급이 <b>보류</b>되었습니다. 사유 : ")+tool.HTML_escape(reason)+
				tool.Baro_t(`<p>보류 중에는 번호가 자동으로 발급되지 않습니다. 이미 남긴 기여 기록과 문서는 그대로 보존됩니다. `)+
				tool.Baro_t(`확인이 끝나면 다시 대기 상태로 돌아갑니다.</p>`))
	}

	if person.Status == "rejected" {
		reason := strings.TrimSpace(person.Note)
		if reason == "" {
			reason = tool.Baro_t("사유가 기록되지 않았습니다.")
		}

		body += tool.Baro_alert("danger", tool.Baro_t("지원이 반려되었습니다. 사유 : ")+tool.HTML_escape(reason))
	}

	if person.Status == "confirmed" {
		body += tool.Baro_card_ex(
			tool.Baro_t("창립 기여자로 확정되었습니다"),
			"창립 기여자 #"+person.Number,
			baro_founding_benefit_html(),
			tool.Baro_button("/baro/invite", tool.Baro_t("초대 링크 받기"), "primary")+" "+
				tool.Baro_button("/baro/card/make?kind=founder", tool.Baro_t("공유 카드 만들기"), "")+" "+
				tool.Baro_button("/baro/founding/hall", tool.Baro_t("기록관에서 보기"), ""),
		)
	} else {
		body += tool.Baro_card(tool.Baro_t("창립 기여자 혜택"), baro_founding_benefit_html())
	}

	body += tool.Baro_card(tool.Baro_t("번호 확정 규칙"), baro_founding_rule_html())
	body += baro_founding_seat_html(db)

	return tool.Baro_page(db, config, tool.Baro_t("내 지원 상태"), body, baro_founding_menu())
}

// View_baro_founding_list : 창립단 명단 (1-2)
func View_baro_founding_list(config tool.Config, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	// 명단을 볼 때도 승격 검사를 돌려 대기자를 정리한다.
	baro_founding_promote_all(db)

	page_int := baro_founding_page_num(page)
	list := baro_founding_list_rows(db, (page_int-1)*tool.Baro_page_size, tool.Baro_page_size)

	rows := [][]string{}
	for _, person := range list {
		rows = append(rows, []string{
			`<b>#` + tool.HTML_escape(person.Number) + `</b>`,
			tool.Baro_user_link(db, person.User, config.IP),
			tool.Baro_time_cell(person.Date),
			tool.HTML_escape(tool.Baro_field_name(db, person.Field)),
			baro_founding_event_text(db, person.FirstEvent),
		})
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/founding", tool.Baro_t("창립 시즌")},
		{"", tool.Baro_t("창립단 명단")},
	})
	body += tool.Baro_hero(tool.Baro_t("100인 창립단 명단"), tool.Baro_t("번호는 첫 유효 기여가 확인된 순서로 확정됩니다."), "")
	body += baro_founding_tabs("/baro/founding/list")
	body += baro_founding_seat_html(db)

	if len(rows) == 0 {
		body += tool.Baro_card(tool.Baro_t("창립단"), tool.Baro_empty(tool.Baro_t("아직 번호가 확정된 창립 기여자가 없습니다. 첫 번째가 되어 보세요.")))
		body += tool.Baro_card("", tool.Baro_button("/baro/founding/apply", tool.Baro_t("창립 기여자로 지원하기"), "primary"))
	} else {
		body += tool.Baro_card(
			tool.Baro_t("창립 기여자 명단"),
			tool.Baro_table(
				[]string{"번호", tool.Baro_t("사용자"), tool.Baro_t("확정일"), "분야", tool.Baro_t("첫 기여")},
				rows,
				tool.Baro_t("아직 번호가 확정된 창립 기여자가 없습니다."),
			),
		)
	}

	body += tool.Baro_paging(page_int, len(rows), tool.Baro_page_size, "/baro/founding/list/{}")
	body += tool.Baro_note(tool.Baro_t("공개 명단에는 사용자 이름과 첫 기여만 표시합니다. IP·이메일 등 개인정보는 공개하지 않습니다."))

	return tool.Baro_page(db, config, tool.Baro_t("창립단 명단"), body, baro_founding_menu())
}

// baro_founding_count_sql : 숫자 하나만 세는 도우미
func baro_founding_count_sql(db *sql.DB, query string, args ...any) int {
	num := 0
	tool.QueryRow_DB(db, query, []any{&num}, args...)

	return num
}

// baro_founding_one_sql : 문자열 하나만 읽는 도우미
func baro_founding_one_sql(db *sql.DB, query string, args ...any) string {
	data := ""
	tool.QueryRow_DB(db, query, []any{&data}, args...)

	return data
}

// View_baro_founding_hall : 창립 시즌 기록관 (1-2)
func View_baro_founding_hall(config tool.Config, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_int := baro_founding_page_num(page)
	list := baro_founding_list_rows(db, (page_int-1)*tool.Baro_page_size, tool.Baro_page_size)

	goal := baro_founding_goal(db)
	confirmed := baro_founding_status_count(db, "confirmed")
	applied := baro_founding_status_count(db, "applied")

	first_date := baro_founding_one_sql(db, "select date from baro_founder where status = 'confirmed' and date != '' order by date asc limit 1")
	last_date := baro_founding_one_sql(db, "select date from baro_founder where status = 'confirmed' and date != '' order by date desc limit 1")

	event_all := baro_founding_count_sql(db, "select count(*) from baro_contrib_event where user in (select user from baro_founder where status = 'confirmed')")
	event_doc := baro_founding_count_sql(db, "select count(*) from baro_contrib_event where kind = 'new_document' and user in (select user from baro_founder where status = 'confirmed')")
	event_src := baro_founding_count_sql(db, "select count(*) from baro_contrib_event where kind = 'source' and user in (select user from baro_founder where status = 'confirmed')")
	invite_done := baro_founding_count_sql(db, "select count(*) from baro_invite where status = 'done'")
	card_count := baro_founding_count_sql(db, "select count(*) from baro_share_card")

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/founding", tool.Baro_t("창립 시즌")},
		{"", tool.Baro_t("기록관")},
	})
	body += tool.Baro_hero(
		tool.Baro_t("창립 시즌 기록관"),
		tool.Baro_t("바로위키를 처음부터 만든 사람들의 기록을 영구히 보존합니다."),
		"",
	)
	body += baro_founding_tabs("/baro/founding/hall")

	body += tool.Baro_card(
		tool.Baro_t("창립 시즌 요약"),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("영구 등재 인원"), tool.Baro_comma(confirmed) + tool.Baro_t("명"), "정원 " + tool.Baro_comma(goal) + tool.Baro_t("명")},
			{tool.Baro_t("번호 대기"), tool.Baro_comma(applied) + tool.Baro_t("명"), tool.Baro_t("첫 유효 기여 대기")},
			{tool.Baro_t("창립단 기여 기록"), tool.Baro_comma(event_all) + tool.Baro_t("건"), tool.Baro_t("전체 기여 이벤트")},
			{tool.Baro_t("새 문서"), tool.Baro_comma(event_doc) + tool.Baro_t("건"), tool.Baro_t("창립단이 만든 문서")},
			{tool.Baro_t("출처 추가"), tool.Baro_comma(event_src) + tool.Baro_t("건"), tool.Baro_t("창립단이 붙인 근거")},
			{tool.Baro_t("완료된 초대"), tool.Baro_comma(invite_done) + tool.Baro_t("건"), tool.Baro_t("첫 기여까지 마친 초대")},
			{tool.Baro_t("공유 카드"), tool.Baro_comma(card_count) + tool.Baro_t("장"), tool.Baro_t("창립 시즌에 만든 카드")},
		})+
			tool.Baro_progress(confirmed, goal, tool.Baro_t("창립단 확정"))+
			tool.Baro_dl([][]string{
				{tool.Baro_t("첫 번호 확정"), tool.Baro_time_cell(first_date)},
				{tool.Baro_t("최근 번호 확정"), tool.Baro_time_cell(last_date)},
			}),
	)

	// 분야별 분포
	field_rows := [][]string{}
	for _, field := range tool.Baro_field_list(db) {
		field_confirmed := baro_founding_field_count(db, field.Key, "confirmed")
		field_applied := baro_founding_field_count(db, field.Key, "applied")
		if field_confirmed == 0 && field_applied == 0 {
			continue
		}

		field_rows = append(field_rows, []string{
			tool.HTML_escape(field.Name),
			tool.Baro_comma(field_confirmed) + tool.Baro_t("명"),
			tool.Baro_comma(field_applied) + tool.Baro_t("명"),
			tool.Baro_progress(field_confirmed, confirmed, ""),
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("분야별 창립 기여자"),
		tool.Baro_table(
			[]string{"분야", tool.Baro_t("확정"), tool.Baro_t("대기"), tool.Baro_t("비중")},
			field_rows,
			tool.Baro_t("아직 분야별 기록이 없습니다."),
		),
	)

	hall_rows := [][]string{}
	for _, person := range list {
		hall_rows = append(hall_rows, []string{
			`<b>#` + tool.HTML_escape(person.Number) + `</b>`,
			tool.Baro_user_link(db, person.User, config.IP),
			tool.Baro_time_cell(person.Date),
			tool.HTML_escape(tool.Baro_field_name(db, person.Field)),
			baro_founding_event_text(db, person.FirstEvent),
		})
	}

	if len(hall_rows) == 0 {
		body += tool.Baro_card(tool.Baro_t("영구 등재 명단"), tool.Baro_empty(tool.Baro_t("아직 등재된 창립 기여자가 없습니다.")))
	} else {
		body += tool.Baro_card(
			tool.Baro_t("영구 등재 명단"),
			tool.Baro_table(
				[]string{"번호", tool.Baro_t("사용자"), tool.Baro_t("확정일"), "분야", tool.Baro_t("첫 기여")},
				hall_rows,
				tool.Baro_t("아직 등재된 창립 기여자가 없습니다."),
			),
		)
	}

	body += tool.Baro_paging(page_int, len(hall_rows), tool.Baro_page_size, "/baro/founding/hall/{}")
	body += tool.Baro_note(tool.Baro_t("기록관의 등재 내용은 삭제되지 않고 보존됩니다. 계정을 탈퇴해도 번호와 첫 기여 기록은 남습니다."))
	body += tool.Baro_card(
		tool.Baro_t("함께 보기"),
		tool.Baro_button("/baro/founding/list", tool.Baro_t("창립단 명단"), "")+" "+
			tool.Baro_button("/baro/founding/apply", tool.Baro_t("창립 기여자로 지원하기"), "primary")+" "+
			tool.Baro_button("/baro/card", tool.Baro_t("공유 카드"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("창립 시즌 기록관"), body, baro_founding_menu())
}
