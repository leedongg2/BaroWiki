package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  11-4 이해충돌 회피  (/baro/role/coi)
//
//  운영자가 사건 당사자이거나 그 사건에 이미 손을 댔다면
//  스스로 빠지고 다른 담당자에게 넘긴다.
// ============================================================

// baro_gov_case_row : 사건 요약
type baro_gov_case_row struct {
	ID         string
	Number     string
	Kind       string
	TargetKind string
	Target     string
	Actor      string
	Status     string
	Date       string
}

// baro_gov_case_find : 사건 번호 또는 내부 번호로 사건 찾기
func baro_gov_case_find(db *sql.DB, key string) (baro_gov_case_row, bool) {
	row := baro_gov_case_row{}
	key = strings.TrimSpace(key)

	if key == "" {
		return row, false
	}

	query := "select id, number, kind, target_kind, target, actor, status, date from baro_case where number = ? limit 1"
	found := tool.QueryRow_DB(
		db, query,
		[]any{&row.ID, &row.Number, &row.Kind, &row.TargetKind, &row.Target, &row.Actor, &row.Status, &row.Date},
		key,
	)

	if !found {
		found = tool.QueryRow_DB(
			db,
			"select id, number, kind, target_kind, target, actor, status, date from baro_case where id = ? limit 1",
			[]any{&row.ID, &row.Number, &row.Kind, &row.TargetKind, &row.Target, &row.Actor, &row.Status, &row.Date},
			key,
		)
	}

	return row, found
}

// baro_gov_case_label : 사건 표시 이름
func baro_gov_case_label(row baro_gov_case_row) string {
	if row.Number != "" {
		return "사건 " + row.Number
	}

	return "사건 #" + row.ID
}

// baro_gov_coi_check : 이 사람이 이 사건을 맡아도 되는지 살핀다
//
//	사건의 target 이 본인이거나, baro_case_action 에 이미 본인이 조치한 기록이 있으면 이해충돌이다.
//	신고자 · 이의제기 신청자 · 이미 선언한 회피도 함께 알려 준다.
func baro_gov_coi_check(db *sql.DB, case_row baro_gov_case_row, actor string) []string {
	out := []string{}

	actor = strings.TrimSpace(actor)
	if actor == "" || case_row.ID == "" {
		return out
	}

	// 1. 사건 당사자
	if case_row.Target != "" && case_row.Target == actor {
		out = append(out, tool.Baro_t("이 사건의 대상(당사자)이 본인입니다."))
	}

	// 2. 이미 이 사건에 조치한 기록
	acted := baro_gov_count(db, "select count(*) from baro_case_action where case_id = ? and actor = ?", case_row.ID, actor)
	if acted > 0 {
		out = append(out, tool.Baro_t("이 사건에 이미 ")+tool.Baro_itoa(acted)+tool.Baro_t("건의 조치를 한 기록이 있습니다."))
	}

	// 3. 사건 담당자로 지정돼 있는데 이의제기 심사를 맡으려는 경우
	if case_row.Actor != "" && case_row.Actor == actor {
		out = append(out, tool.Baro_t("이 사건의 원 담당자입니다. 같은 사건의 이의제기는 다른 담당자가 심사해야 합니다."))
	}

	// 4. 신고자
	reported := baro_gov_count(db, "select count(*) from baro_report where case_id = ? and reporter = ?", case_row.ID, actor)
	if reported > 0 {
		out = append(out, tool.Baro_t("이 사건의 신고자입니다."))
	}

	// 5. 이의제기 신청자
	appealed := baro_gov_count(db, "select count(*) from baro_appeal where case_id = ? and user = ?", case_row.ID, actor)
	if appealed > 0 {
		out = append(out, tool.Baro_t("이 사건에 이의제기를 낸 당사자입니다."))
	}

	// 6. 이미 회피 선언
	declared := baro_gov_count(db, "select count(*) from baro_coi where case_id = ? and actor = ?", case_row.ID, actor)
	if declared > 0 {
		out = append(out, tool.Baro_t("이미 이 사건에서 회피를 선언했습니다."))
	}

	return out
}

// baro_gov_coi_table : 이해충돌 회피 기록 표
func baro_gov_coi_table(db *sql.DB, case_key string, actor string, my_ip string, limit int) string {
	if limit < 1 {
		limit = 20
	}

	where := " where 1 = 1"
	args := []any{}

	if case_key != "" {
		where += " and case_id = ?"
		args = append(args, case_key)
	}

	if actor != "" {
		where += " and actor = ?"
		args = append(args, actor)
	}

	args = append(args, limit)

	rows := tool.Query_DB(db, "select id, case_id, actor, reason, date from baro_coi"+where+" order by id + 0 desc limit ?", args...)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		id, case_id, who, reason, date := "", "", "", "", ""
		if err := rows.Scan(&id, &case_id, &who, &reason, &date); err != nil {
			continue
		}

		case_row, found := baro_gov_case_find(db, case_id)
		case_html := tool.Baro_t(`<span class="bw_muted">사건 #`) + tool.HTML_escape(case_id) + `</span>`
		if found {
			case_html = tool.HTML_escape(baro_gov_case_label(case_row))
			if case_row.Status != "" {
				case_html += " " + tool.Baro_badge(case_row.Status, "muted")
			}
		}

		table_rows = append(table_rows, []string{
			tool.Baro_time_cell(date),
			case_html,
			baro_gov_actor_html(db, who, my_ip),
			baro_gov_safe(tool.Baro_trunc(tool.Baro_one_line(reason), 90)),
		})
	}

	return tool.Baro_table([]string{"시각", "사건", tool.Baro_t("회피한 운영자"), "사유"}, table_rows, tool.Baro_t("이해충돌 회피 기록이 없습니다."))
}

// baro_gov_coi_notice : 이해충돌 원칙 안내
func baro_gov_coi_notice() string {
	return tool.Baro_alert("info",
		tool.Baro_t(`운영자가 <b>사건 당사자</b>이거나 그 <b>토론에 깊이 참여</b>했다면 스스로 빠지고 다른 담당자에게 넘깁니다.`)+
			tool.Baro_t(` 회피 선언은 지워지지 않으며, 나중에 그 사건의 처리 과정을 되짚을 때 근거가 됩니다.`),
	)
}

// View_baro_gov_coi_list : 이해충돌 회피 목록 + 담당 가능 여부 확인
func View_baro_gov_coi_list(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	case_key := baro_gov_q(query, "case")
	actor := baro_gov_q(query, "actor")
	check_case := baro_gov_q(query, "check_case")
	check_user := baro_gov_q(query, "check_user")
	page := baro_gov_page_num(baro_gov_q(query, "page"))

	where := " where 1 = 1"
	args := []any{}

	if case_key != "" {
		where += " and case_id = ?"
		args = append(args, case_key)
	}

	if actor != "" {
		where += " and actor = ?"
		args = append(args, actor)
	}

	total := 0
	tool.QueryRow_DB(db, "select count(*) from baro_coi"+where, []any{&total}, args...)

	size := tool.Baro_page_size
	list_args := append(append([]any{}, args...), (page-1)*size, size)

	rows := tool.Query_DB(db, "select id, case_id, actor, reason, date from baro_coi"+where+" order by id + 0 desc limit ?, ?", list_args...)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0

	for rows.Next() {
		id, case_id, who, reason, date := "", "", "", "", ""
		if err := rows.Scan(&id, &case_id, &who, &reason, &date); err != nil {
			continue
		}

		count++

		case_row, found := baro_gov_case_find(db, case_id)
		case_html := tool.Baro_t(`<span class="bw_muted">사건 #`) + tool.HTML_escape(case_id) + `</span>`
		if found {
			case_html = `<b>` + tool.HTML_escape(baro_gov_case_label(case_row)) + `</b>`
			case_html += `<div class="bw_note">` + tool.HTML_escape(case_row.Kind) + " · " + tool.HTML_escape(case_row.Status) + `</div>`
		}

		table_rows = append(table_rows, []string{
			tool.Baro_time_cell(date),
			case_html,
			baro_gov_actor_html(db, who, config.IP),
			baro_gov_safe(tool.Baro_trunc(tool.Baro_one_line(reason), 90)),
			tool.Baro_button(baro_gov_url("/baro/role/coi", [][]string{{"case", case_id}}), tool.Baro_t("이 사건만"), ""),
		})
	}

	body := baro_gov_css()
	body += tool.Baro_hero(tool.Baro_t("이해충돌 회피"), tool.Baro_t("사건 당사자이거나 이미 손을 댄 사람은 그 사건을 맡지 않습니다."), "")
	body += baro_gov_tabs("/baro/role/coi")
	body += baro_gov_coi_notice()

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 회피 선언"), tool.Baro_comma(baro_gov_count(db, "select count(*) from baro_coi")), "누적"},
		{tool.Baro_t("거른 결과"), tool.Baro_comma(total), tool.Baro_t("현재 조건")},
		{tool.Baro_t("회피한 운영자"), tool.Baro_comma(baro_gov_count(db, "select count(distinct actor) from baro_coi")), tool.Baro_t("사람 수")},
		{tool.Baro_t("회피가 있는 사건"), tool.Baro_comma(baro_gov_count(db, "select count(distinct case_id) from baro_coi")), tool.Baro_t("사건 수")},
	})

	// 담당 가능 여부 확인 도구
	//
	//	확인 결과에는 그 사람이 이 사건의 신고자인지 · 이의제기 신청자인지가 드러난다.
	//	신고자 신원은 공개 대상이 아니므로(0장 5번 원칙) 운영 역할을 가진 사람에게만 보여 준다.
	check_body := ""

	if baro_gov_is_staff(db, config.IP) {
		filter_body := `<div class="bw_gov_filter">`
		filter_body += tool.Baro_input("check_case", "사건 번호", check_case, tool.Baro_t("사건 번호 또는 내부 번호"), "")
		filter_body += tool.Baro_input("check_user", tool.Baro_t("담당 후보"), check_user, tool.Baro_t("확인할 운영자 이름"), "")
		filter_body += `<div class="bw_gov_filter_btn">` + tool.Baro_submit("확인", "primary") + `</div>`
		filter_body += `</div>`

		check_body = baro_gov_get_form("/baro/role/coi", filter_body)

		if check_case != "" || check_user != "" {
			check_body += baro_gov_coi_check_html(db, check_case, check_user, config.IP)
		} else {
			check_body += tool.Baro_note(tool.Baro_t("사건 번호와 담당 후보를 넣으면 그 사람이 이 사건을 맡아도 되는지 확인해 줍니다."))
		}
	} else {
		check_body = tool.Baro_alert("info",
			tool.Baro_t(`담당 가능 여부 확인은 <b>운영 역할을 가진 사람</b>만 쓸 수 있습니다.`)+
				tool.Baro_t(` 확인 결과에는 그 사람이 이 사건의 신고자인지 같은 내용이 담기는데, 신고자 신원은 공개하지 않기 때문입니다.`)+
				tool.Baro_t(` 아래 회피 선언 기록은 누구나 볼 수 있습니다.`),
		)
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("담당 가능 여부 확인"),
		tool.Baro_t("사건 담당자를 정하기 전에 이해충돌이 있는지 살핍니다."),
		check_body,
		"",
	)

	// 거르기
	filter := `<div class="bw_gov_filter">`
	filter += tool.Baro_input("case", "사건 번호", case_key, tool.Baro_t("사건 번호로 거르기"), "")
	filter += tool.Baro_input("actor", tool.Baro_t("회피한 운영자"), actor, tool.Baro_t("사용자 이름"), "")
	filter += `<div class="bw_gov_filter_btn">` + tool.Baro_submit(tool.Baro_t("찾기"), "") + ` ` + tool.Baro_button("/baro/role/coi", tool.Baro_t("조건 지우기"), "") + `</div>`
	filter += `</div>`

	body += tool.Baro_card(tool.Baro_t("거르기"), baro_gov_get_form("/baro/role/coi", filter))

	list_body := ""
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("조건에 맞는 이해충돌 회피 기록이 없습니다."))
	} else {
		list_body = tool.Baro_table([]string{"시각", "사건", tool.Baro_t("회피한 운영자"), "사유", ""}, table_rows, "")
	}

	list_body += tool.Baro_paging(page, count, size, baro_gov_url("/baro/role/coi", [][]string{
		{"case", case_key}, {"actor", actor}, {"page", "{}"},
	}))

	foot := tool.Baro_button("/baro/role/coi/new", tool.Baro_t("회피 선언하기"), "primary")

	body += tool.Baro_card_ex(tool.Baro_t("회피 선언 기록"), tool.Baro_t("조건에 맞는 기록 ")+tool.Baro_comma(total)+tool.Baro_t("건"), list_body, foot)

	return tool.Baro_page(db, config, tool.Baro_t("이해충돌 회피"), body, baro_gov_menu())
}

// baro_gov_coi_check_html : 담당 가능 여부 확인 결과
func baro_gov_coi_check_html(db *sql.DB, case_key string, user string, my_ip string) string {
	if case_key == "" {
		return tool.Baro_alert("warn", tool.Baro_t("확인할 사건 번호를 넣어 주세요."))
	}

	if user == "" {
		return tool.Baro_alert("warn", tool.Baro_t("확인할 담당 후보 이름을 넣어 주세요."))
	}

	case_row, found := baro_gov_case_find(db, case_key)
	if !found {
		return tool.Baro_alert("warn", tool.Baro_t("그런 사건이 없습니다. 사건 번호를 다시 확인해 주세요. (넣은 값 : ")+tool.HTML_escape(tool.Baro_trunc(case_key, 40))+")")
	}

	reasons := baro_gov_coi_check(db, case_row, user)

	info := tool.Baro_kv_grid([][]string{
		{"사건", tool.HTML_escape(baro_gov_case_label(case_row))},
		{"종류", tool.HTML_escape(case_row.Kind)},
		{tool.Baro_t("대상"), baro_gov_safe(tool.Baro_trunc(case_row.Target, 50))},
		{tool.Baro_t("현재 담당"), baro_gov_actor_html(db, case_row.Actor, my_ip)},
		{"상태", tool.Baro_badge(case_row.Status, "muted")},
		{tool.Baro_t("확인 대상"), baro_gov_actor_html(db, user, my_ip)},
	})

	if len(reasons) == 0 {
		return info + tool.Baro_alert("ok", tool.Baro_t("<b>이해충돌이 확인되지 않았습니다.</b> 이 사람이 이 사건을 맡아도 됩니다. 다만 토론에 깊이 참여한 적이 있다면 스스로 판단해 회피해 주세요."))
	}

	items := []string{}
	for _, reason := range reasons {
		items = append(items, tool.HTML_escape(reason))
	}

	html := info
	html += tool.Baro_alert("danger", tool.Baro_t("<b>이해충돌이 있습니다.</b> 이 사람은 이 사건을 맡지 않는 것이 좋습니다."))
	html += tool.Baro_list(items)
	html += tool.Baro_button(baro_gov_url("/baro/role/coi/new", [][]string{{"case", case_key}}), tool.Baro_t("회피 선언 화면으로"), "primary")

	return html
}

// View_baro_gov_coi_new : 회피 선언
func View_baro_gov_coi_new(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/role/coi/new"); need {
		return wall
	}

	if !baro_gov_is_staff(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("회피 선언은 운영 역할을 가진 사람만 할 수 있습니다. 사건 처리에 참여하지 않는다면 선언할 것도 없습니다."))
	}

	case_key := baro_gov_q(query, "case")
	reason := ""
	alert := ""

	if values != nil {
		case_key = strings.TrimSpace(values.Get("case_id"))
		reason = strings.TrimSpace(values.Get("reason"))

		case_row, found := baro_gov_case_find(db, case_key)

		already := false
		if found {
			already = baro_gov_count(db, "select count(*) from baro_coi where case_id = ? and actor = ?", case_row.ID, config.IP) > 0
		}

		switch {
		case tool.Baro_read_only(db):
			alert = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.HTML_escape(tool.Baro_read_only_reason(db)))
		case case_key == "":
			alert = tool.Baro_alert("danger", tool.Baro_t("어느 사건에서 빠지는지 사건 번호를 적어 주세요."))
		case !found:
			alert = tool.Baro_alert("danger", tool.Baro_t("그런 사건이 없습니다. 사건 번호를 다시 확인해 주세요."))
		case reason == "":
			alert = tool.Baro_alert("danger", tool.Baro_t("회피 사유를 적어 주세요. 사유 없는 회피는 기록으로서 뜻이 없습니다."))
		case already:
			alert = tool.Baro_alert("warn", tool.Baro_t("이미 이 사건에서 회피를 선언했습니다."))
		default:
			tool.Exec_DB(
				db,
				"insert into baro_coi (id, case_id, actor, reason, date) values (?, ?, ?, ?, ?)",
				tool.Baro_next_id(db, "baro_coi"), case_row.ID, config.IP, reason, tool.Baro_now(),
			)

			tool.Baro_public_log(
				db, "coi", config.IP, baro_gov_case_label(case_row),
				tool.Baro_trunc(reason, 300),
				tool.Baro_json_enc(map[string]string{"사건": baro_gov_case_label(case_row)}),
			)
			tool.Baro_audit(db, config, "coi_declare", case_row.ID, "", config.IP, reason)

			baro_gov_coi_notify_others(db, case_row, config.IP)

			return tool.Get_redirect(baro_gov_url("/baro/role/coi", [][]string{{"case", case_row.ID}}))
		}
	}

	body := baro_gov_css()
	body += tool.Baro_crumb([][]string{{"/baro/role", tool.Baro_t("운영자 역할")}, {"/baro/role/coi", tool.Baro_t("이해충돌 회피")}, {"", tool.Baro_t("회피 선언")}})
	body += tool.Baro_hero(tool.Baro_t("회피 선언"), tool.Baro_t("이 사건을 맡지 않겠다고 남깁니다."), "")
	body += baro_gov_tabs("/baro/role/coi")
	body += alert
	body += baro_gov_coi_notice()

	form := tool.Baro_input("case_id", "사건 번호", case_key, tool.Baro_t("회피할 사건 번호"), tool.Baro_t("사건 번호 또는 내부 번호를 적습니다."))
	form += tool.Baro_textarea("reason", tool.Baro_t("회피 사유"), reason, 5, tool.Baro_t("예) 이 문서의 주요 편집자라 판단이 치우칠 수 있습니다."), tool.Baro_t("사유는 공개 운영 로그에 남습니다. 개인정보는 적지 마세요."))
	form += tool.Baro_submit(tool.Baro_t("회피 선언"), "primary")

	body += tool.Baro_card(tool.Baro_t("선언 내용"), tool.Baro_form("/baro/role/coi/new", form))

	// 미리 살펴 보기
	if case_key != "" {
		body += tool.Baro_card_ex(tool.Baro_t("이 사건과 나의 관계"), tool.Baro_t("지금 로그인한 사람 기준으로 살핍니다."), baro_gov_coi_check_html(db, case_key, config.IP, config.IP), "")
	}

	body += tool.Baro_card(tool.Baro_t("내가 이미 선언한 회피"), baro_gov_coi_table(db, "", config.IP, config.IP, 20))
	body += tool.Baro_button("/baro/role/coi", tool.Baro_t("이해충돌 목록으로"), "")

	return tool.Baro_page(db, config, tool.Baro_t("회피 선언"), body, baro_gov_menu())
}

// baro_gov_coi_notify_others : 이 사건을 넘겨받을 수 있는 담당자에게 알린다
func baro_gov_coi_notify_others(db *sql.DB, case_row baro_gov_case_row, actor string) {
	rows := tool.Query_DB(db, "select distinct user from baro_role where role = 'sanction' or role = 'mediator'")
	defer rows.Close()

	users := []string{}
	for rows.Next() {
		user := ""
		if err := rows.Scan(&user); err != nil {
			continue
		}

		if user == "" || user == actor {
			continue
		}

		users = append(users, user)
	}

	for _, user := range users {
		if len(baro_gov_coi_check(db, case_row, user)) > 0 {
			continue
		}

		tool.Baro_notify(
			db, user, "case",
			baro_gov_case_label(case_row)+tool.Baro_t(" 담당자가 이해충돌로 빠졌습니다. 맡아 줄 사람이 필요합니다."),
			baro_gov_url("/baro/role/coi", [][]string{{"case", case_row.ID}}),
			"",
		)
	}
}
