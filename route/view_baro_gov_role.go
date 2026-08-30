package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  11-3 운영자 역할 분리  (/baro/role)
//
//  한 명이 신고 접수부터 판결 · 재심까지 다 하는 구조를 피한다.
//  제재 담당과 이의제기 담당을 한 사람이 겸하면 화면에 경고를 띄운다.
// ============================================================

// baro_gov_role_conflict_pairs : 함께 가지면 안 되는 역할 짝
func baro_gov_role_conflict_pairs() [][]string {
	return [][]string{
		{"sanction", "appeal", tool.Baro_t("제재를 집행한 사람이 그 제재에 대한 이의제기를 다시 심사하게 됩니다. 재심의 뜻이 사라집니다.")},
	}
}

// baro_gov_role_users : 역할별 담당자 목록
func baro_gov_role_users(db *sql.DB, role string) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(db, "select user, granted_by, date, memo from baro_role where role = ? order by date asc", role)
	defer rows.Close()

	for rows.Next() {
		user, by, date, memo := "", "", "", ""
		if err := rows.Scan(&user, &by, &date, &memo); err != nil {
			continue
		}

		out = append(out, []string{user, by, date, memo})
	}

	return out
}

// baro_gov_role_has_row : baro_role 표에 실제로 역할 줄이 있는지
//
//	tool.Baro_has_role 은 소유자에게 늘 참을 돌려주므로, 줄이 실제로 있는지 볼 때는 이 함수를 쓴다.
func baro_gov_role_has_row(db *sql.DB, user string, role string) bool {
	found := ""
	tool.QueryRow_DB(db, "select role from baro_role where user = ? and role = ? limit 1", []any{&found}, user, role)

	return found != ""
}

// baro_gov_role_all_users : 역할을 하나라도 가진 사람 목록 (가나다 순)
func baro_gov_role_all_users(db *sql.DB) []string {
	rows := tool.Query_DB(db, "select distinct user from baro_role")
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		user := ""
		if err := rows.Scan(&user); err != nil {
			continue
		}

		if strings.TrimSpace(user) == "" {
			continue
		}

		out = append(out, user)
	}

	sort.Strings(out)

	return out
}

// baro_gov_role_conflict_users : 겸직 경고 대상 (제재 + 이의제기)
func baro_gov_role_conflict_users(db *sql.DB) [][]string {
	out := [][]string{}

	for _, pair := range baro_gov_role_conflict_pairs() {
		rows := tool.Query_DB(
			db,
			"select a.user from baro_role a, baro_role b where a.user = b.user and a.role = ? and b.role = ?",
			pair[0], pair[1],
		)

		users := []string{}
		for rows.Next() {
			user := ""
			if err := rows.Scan(&user); err != nil {
				continue
			}

			users = append(users, user)
		}
		rows.Close()

		sort.Strings(users)

		for _, user := range users {
			out = append(out, []string{user, pair[0], pair[1], pair[2]})
		}
	}

	return out
}

// baro_gov_role_principle : 역할 분리 원칙 안내 (11-3 의 핵심 문구)
func baro_gov_role_principle() string {
	return tool.Baro_alert("warn",
		tool.Baro_t(`<b>한 명이 신고 접수부터 판결 · 재심까지 다 하는 구조를 피합니다.</b>`)+
			tool.Baro_t(` 순찰자가 신고를 접수하고, 제재 담당이 집행하고, 이의제기 담당이 다시 살핍니다.`)+
			tool.Baro_t(` 제재를 집행한 사람은 그 사건의 이의제기를 심사하지 않습니다.`)+
			tool.Baro_t(` 프로젝트 · 창립 시즌 · 어워드 운영은 콘텐츠 역할이라 제재 권한과 분리돼 있습니다.`),
	)
}

// View_baro_gov_role_list : 역할별 담당자 목록
func View_baro_gov_role_list(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	// 사용자 찾기 상자에서 넘어온 값이면 그 사람의 역할 화면으로 보낸다
	if target := baro_gov_q(query, "user"); target != "" {
		return tool.Get_redirect("/baro/role/user/" + tool.Url_parser(target))
	}

	can_manage := tool.Baro_can(db, config.IP, "system") || tool.Baro_is_owner(db, config.IP)

	body := baro_gov_css()
	body += tool.Baro_hero(tool.Baro_t("운영자 역할"), tool.Baro_t("역할을 나누어 한 사람에게 권한이 몰리지 않게 합니다."), "")
	body += baro_gov_tabs("/baro/role")
	body += baro_gov_role_principle()

	// 겸직 경고
	conflicts := baro_gov_role_conflict_users(db)
	if len(conflicts) > 0 {
		rows := [][]string{}
		for _, one := range conflicts {
			rows = append(rows, []string{
				tool.Baro_user_link(db, one[0], config.IP),
				tool.Baro_badge(tool.Baro_role_name(one[1]), "danger") + " + " + tool.Baro_badge(tool.Baro_role_name(one[2]), "danger"),
				tool.HTML_escape(one[3]),
				tool.Baro_button("/baro/role/user/"+tool.Url_parser(one[0]), "역할 조정", ""),
			})
		}

		body += tool.Baro_card_ex(
			tool.Baro_t("겸직 경고"),
			tool.Baro_t("한 사람이 서로 견제해야 할 역할을 함께 가지고 있습니다."),
			tool.Baro_alert("danger", tool.Baro_t("아래 담당자는 <b>제재</b>와 <b>이의제기</b> 역할을 동시에 가지고 있습니다. 역할을 나누어 주세요."))+
				tool.Baro_table([]string{"담당자", tool.Baro_t("겹치는 역할"), tool.Baro_t("왜 문제인가"), ""}, rows, ""),
			"",
		)
	} else {
		body += tool.Baro_alert("ok", tool.Baro_t("제재와 이의제기를 동시에 맡은 담당자가 없습니다. 역할 분리가 지켜지고 있습니다."))
	}

	// 역할별 담당자
	role_rows := [][]string{}
	total_slots := 0

	for _, role := range tool.Baro_role_list() {
		users := baro_gov_role_users(db, role[0])
		total_slots += len(users)

		user_html := tool.Baro_t(`<span class="bw_muted">아직 담당자가 없습니다.</span>`)
		if len(users) > 0 {
			parts := []string{}
			for _, one := range users {
				parts = append(parts, tool.Baro_user_link(db, one[0], config.IP))
			}

			user_html = strings.Join(parts, ", ")
		}

		role_rows = append(role_rows, []string{
			`<b>` + tool.HTML_escape(role[1]) + `</b><div class="bw_note">` + tool.HTML_escape(role[0]) + `</div>`,
			tool.HTML_escape(role[2]),
			tool.Baro_itoa(len(users)) + tool.Baro_t("명"),
			user_html,
		})
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("역할 종류"), tool.Baro_itoa(len(tool.Baro_role_list())), tool.Baro_t("나뉜 역할 수")},
		{tool.Baro_t("역할 보유자"), tool.Baro_itoa(len(baro_gov_role_all_users(db))), tool.Baro_t("사람 수")},
		{tool.Baro_t("부여된 역할"), tool.Baro_itoa(total_slots), tool.Baro_t("사람 × 역할")},
		{tool.Baro_t("겸직 경고"), tool.Baro_itoa(len(conflicts)), tool.Baro_t("제재 + 이의제기")},
	})

	body += tool.Baro_card_ex(
		tool.Baro_t("역할별 담당자"),
		tool.Baro_t("각 역할이 무엇을 할 수 있는지와 지금 누가 맡고 있는지를 보여 줍니다."),
		tool.Baro_table([]string{"역할", tool.Baro_t("하는 일"), tool.Baro_t("인원"), "담당자"}, role_rows, tool.Baro_t("역할 정보가 없습니다.")),
		"",
	)

	// 사람별 역할
	users := baro_gov_role_all_users(db)

	page := baro_gov_page_num(baro_gov_q(query, "page"))
	size := tool.Baro_page_size

	start := (page - 1) * size
	end := start + size

	view := []string{}
	if start < len(users) {
		if end > len(users) {
			end = len(users)
		}

		view = users[start:end]
	}

	person_rows := [][]string{}

	for _, user := range view {
		roles := tool.Baro_user_roles(db, user)
		sort.Strings(roles)

		badges := []string{}
		has_sanction, has_appeal := false, false

		for _, role := range roles {
			kind := "info"
			if role == "sanction" || role == "appeal" {
				kind = "warn"
			}

			badges = append(badges, tool.Baro_badge(tool.Baro_role_name(role), kind))

			if role == "sanction" {
				has_sanction = true
			}
			if role == "appeal" {
				has_appeal = true
			}
		}

		warn := ""
		if has_sanction && has_appeal {
			warn = " " + tool.Baro_badge(tool.Baro_t("겸직 경고"), "danger")
		}

		person_rows = append(person_rows, []string{
			tool.Baro_user_link(db, user, config.IP) + warn,
			strings.Join(badges, " "),
			tool.Baro_itoa(len(roles)) + tool.Baro_t("개"),
			tool.Baro_button("/baro/role/user/"+tool.Url_parser(user), "자세히", ""),
		})
	}

	person_body := ""
	if len(person_rows) == 0 {
		person_body = tool.Baro_empty(tool.Baro_t("아직 역할을 부여받은 사람이 없습니다. 시스템 관리자가 역할을 나누어 주세요."))
	} else {
		person_body = tool.Baro_table([]string{"담당자", tool.Baro_t("가진 역할"), tool.Baro_t("개수"), ""}, person_rows, "")
	}

	person_body += tool.Baro_paging(page, len(view), size, baro_gov_url("/baro/role", [][]string{{"page", "{}"}}))

	find := `<div class="bw_gov_filter">`
	find += tool.Baro_input("user", tool.Baro_t("사용자 이름"), "", tool.Baro_t("역할을 보거나 조정할 사용자"), "")
	find += `<div class="bw_gov_filter_btn">` + tool.Baro_submit(tool.Baro_t("이 사람 보기"), "primary") + `</div>`
	find += `</div>`

	foot := baro_gov_get_form("/baro/role", find)
	if !can_manage {
		foot += tool.Baro_note(tool.Baro_t("역할 부여와 회수는 시스템 관리자 권한 또는 소유자만 할 수 있습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("사람별 역할"), tool.Baro_t("역할을 가진 사람 ")+tool.Baro_itoa(len(users))+tool.Baro_t("명"), person_body, foot)

	body += tool.Baro_card(tool.Baro_t("이 화면에서 이어지는 곳"),
		tool.Baro_button("/baro/role/coi", tool.Baro_t("이해충돌 회피 기록"), "")+" "+
			tool.Baro_button("/baro/role/activity", tool.Baro_t("관리자 활동 기록"), "")+" "+
			tool.Baro_button("/baro/policy/view/principle", tool.Baro_t("고정 원칙 정책 보기"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("운영자 역할"), body, baro_gov_menu())
}

// View_baro_gov_role_user : 한 사람의 역할 부여 · 회수
func View_baro_gov_role_user(config tool.Config, user string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	user = strings.TrimSpace(user)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/role/user/"+user); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "system") && !tool.Baro_is_owner(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("역할 부여와 회수는 시스템 관리자 권한 또는 소유자만 할 수 있습니다."))
	}

	if user == "" {
		body := baro_gov_css()
		body += tool.Baro_alert("warn", tool.Baro_t("역할을 조정할 사용자 이름이 비어 있습니다."))
		body += tool.Baro_card(tool.Baro_t("사용자 고르기"), tool.Baro_button("/baro/role", tool.Baro_t("역할 목록으로"), "primary"))

		return tool.Baro_page(db, config, "역할 조정", body, baro_gov_menu())
	}

	alert := ""

	if values != nil {
		action := strings.TrimSpace(values.Get("action"))
		role := strings.TrimSpace(values.Get("role"))
		memo := strings.TrimSpace(values.Get("memo"))

		known := false
		for _, one := range tool.Baro_role_list() {
			if one[0] == role {
				known = true
				break
			}
		}

		switch {
		case tool.Baro_read_only(db):
			alert = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.HTML_escape(tool.Baro_read_only_reason(db)))
		case !tool.Baro_is_user(user):
			alert = tool.Baro_alert("danger", tool.Baro_t("역할은 로그인 사용자에게만 줄 수 있습니다. IP 사용자에게는 줄 수 없습니다."))
		case !known:
			alert = tool.Baro_alert("danger", tool.Baro_t("그런 역할이 없습니다."))
		case memo == "":
			alert = tool.Baro_alert("danger", tool.Baro_t("왜 주거나 거두는지 사유를 적어 주세요. 사유는 공개 운영 로그에 남습니다."))
		case action == "grant":
			if baro_gov_role_has_row(db, user, role) {
				alert = tool.Baro_alert("warn", tool.Baro_t("이미 가지고 있는 역할입니다."))
				break
			}

			tool.Baro_grant_role(db, user, role, config.IP, memo)

			// 소유자는 모든 역할을 가진 것으로 보기 때문에 기반 계층이 역할 줄을 만들지 않는다.
			if !baro_gov_role_has_row(db, user, role) {
				alert = tool.Baro_alert("info",
					tool.Baro_t("이 사람은 위키 소유자라 이미 모든 역할을 가진 것으로 봅니다. 따로 역할 줄을 만들지 않았습니다."),
				)

				break
			}

			tool.Baro_audit(db, config, "role_grant", user, "", role, memo)
			tool.Baro_notify(db, user, "system", tool.Baro_role_name(role)+" 역할을 받았습니다.", "/baro/role/user/"+tool.Url_parser(user), memo)

			return tool.Get_redirect("/baro/role/user/" + tool.Url_parser(user))
		case action == "revoke":
			// 가지고 있지도 않은 역할을 거두면 공개 로그와 알림만 헛되이 쌓인다.
			if !baro_gov_role_has_row(db, user, role) {
				if tool.Baro_is_owner(db, user) {
					alert = tool.Baro_alert("info", tool.Baro_t("이 사람은 위키 소유자라 역할 표에 줄이 없습니다. 여기서 거둘 것이 없습니다."))
				} else {
					alert = tool.Baro_alert("warn", tool.Baro_t("이 사람은 그 역할을 가지고 있지 않습니다. 거둘 것이 없어 아무것도 기록하지 않았습니다."))
				}

				break
			}

			tool.Baro_revoke_role(db, user, role, config.IP, memo)
			tool.Baro_audit(db, config, "role_revoke", user, role, "", memo)
			tool.Baro_notify(db, user, "system", tool.Baro_role_name(role)+" 역할이 회수됐습니다.", "/baro/role/user/"+tool.Url_parser(user), memo)

			return tool.Get_redirect("/baro/role/user/" + tool.Url_parser(user))
		default:
			alert = tool.Baro_alert("danger", tool.Baro_t("무엇을 할지 고르지 않았습니다."))
		}
	}

	roles := tool.Baro_user_roles(db, user)
	sort.Strings(roles)

	has := map[string]bool{}
	for _, role := range roles {
		has[role] = true
	}

	body := baro_gov_css()
	body += tool.Baro_crumb([][]string{{"/baro/role", tool.Baro_t("운영자 역할")}, {"", user}})
	body += tool.Baro_hero(user+tool.Baro_t(" 의 역할"), tool.Baro_t("역할을 주거나 거둘 수 있습니다. 모든 변경은 공개 운영 로그에 남습니다."), tool.Baro_user_link(db, user, config.IP))
	body += baro_gov_tabs("/baro/role")
	body += alert
	body += baro_gov_role_principle()

	if !tool.Baro_is_user(user) {
		body += tool.Baro_alert("danger", tool.Baro_t("이 이름은 로그인 사용자가 아닙니다(IP 로 보입니다). 역할은 로그인 사용자에게만 줄 수 있습니다."))
	}

	if has["sanction"] && has["appeal"] {
		body += tool.Baro_alert("danger",
			tool.Baro_t(`<b>겸직 경고 — 제재 담당과 이의제기 담당을 동시에 가지고 있습니다.</b>`)+
				tool.Baro_t(` 자기가 내린 제재를 자기가 다시 심사하게 되어 재심의 뜻이 사라집니다. 둘 중 하나를 거두어 주세요.`),
		)
	}

	if tool.Baro_is_owner(db, user) {
		body += tool.Baro_alert("info",
			tool.Baro_t("이 사람은 위키 소유자입니다. 소유자는 역할 표에 없어도 모든 권한을 가지므로, 여기서 역할을 줘도 새 역할 줄이 만들어지지 않습니다.")+
				tool.Baro_t(" 권한을 실제로 나누려면 소유자가 아닌 계정에 역할을 주세요."),
		)
	}

	// 가진 역할
	own_rows := [][]string{}
	for _, one := range tool.Baro_role_list() {
		if !has[one[0]] {
			continue
		}

		by, date, memo := "", "", ""
		tool.QueryRow_DB(db, "select granted_by, date, memo from baro_role where user = ? and role = ? limit 1", []any{&by, &date, &memo}, user, one[0])

		form := tool.Baro_form("/baro/role/user/"+tool.Url_parser(user),
			tool.Baro_hidden("action", "revoke")+
				tool.Baro_hidden("role", one[0])+
				tool.Baro_input("memo", tool.Baro_t("회수 사유"), "", tool.Baro_t("왜 거두는지 적습니다"), "")+
				tool.Baro_submit(tool.Baro_t("회수"), "danger"),
		)

		own_rows = append(own_rows, []string{
			`<b>` + tool.HTML_escape(one[1]) + `</b><div class="bw_note">` + tool.HTML_escape(one[2]) + `</div>`,
			baro_gov_actor_html(db, by, config.IP),
			tool.Baro_time_cell(date),
			baro_gov_safe(tool.Baro_trunc(memo, 60)),
			form,
		})
	}

	own_body := ""
	if len(own_rows) == 0 {
		own_body = tool.Baro_empty(tool.Baro_t("아직 역할이 없습니다. 아래에서 역할을 줄 수 있습니다."))
	} else {
		own_body = tool.Baro_table([]string{"역할", tool.Baro_t("준 사람"), tool.Baro_t("부여 시각"), "사유", tool.Baro_t("회수")}, own_rows, "")
	}

	body += tool.Baro_card_ex(tool.Baro_t("가진 역할"), "역할 "+tool.Baro_itoa(len(roles))+tool.Baro_t("개"), own_body, "")

	// 줄 수 있는 역할
	options := [][]string{}
	for _, one := range tool.Baro_role_list() {
		if has[one[0]] {
			continue
		}

		options = append(options, []string{one[0], one[1] + " — " + one[2]})
	}

	grant_body := ""
	if len(options) == 0 {
		grant_body = tool.Baro_empty(tool.Baro_t("줄 수 있는 역할이 더 없습니다."))
	} else {
		grant_body = tool.Baro_form("/baro/role/user/"+tool.Url_parser(user),
			tool.Baro_hidden("action", "grant")+
				tool.Baro_select("role", tool.Baro_t("줄 역할"), options, "", tool.Baro_t("역할마다 할 수 있는 일이 다릅니다."))+
				tool.Baro_input("memo", tool.Baro_t("부여 사유"), "", tool.Baro_t("왜 이 사람에게 이 역할을 주는지"), tool.Baro_t("사유는 공개 운영 로그에 남습니다."))+
				tool.Baro_submit(tool.Baro_t("역할 주기"), "primary"),
		)
	}

	if has["sanction"] {
		grant_body += tool.Baro_alert("warn", tool.Baro_t("이 사람은 <b>제재 담당</b>입니다. 여기에 <b>이의제기 담당</b>을 더하면 역할 분리가 깨집니다."))
	}
	if has["appeal"] {
		grant_body += tool.Baro_alert("warn", tool.Baro_t("이 사람은 <b>이의제기 담당</b>입니다. 여기에 <b>제재 담당</b>을 더하면 역할 분리가 깨집니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("역할 주기"), tool.Baro_t("역할은 하나씩 줍니다."), grant_body, "")

	// 이 사람에 대한 역할 변경 기록
	log_rows := [][]string{}
	rows := tool.Query_DB(
		db,
		"select id, kind, actor, reason, detail, date from baro_public_log where target = ? and (kind = 'role_grant' or kind = 'role_revoke') order by id + 0 desc limit 30",
		user,
	)
	defer rows.Close()

	for rows.Next() {
		id, kind, actor, reason, detail, date := "", "", "", "", "", ""
		if err := rows.Scan(&id, &kind, &actor, &reason, &detail, &date); err != nil {
			continue
		}

		log_rows = append(log_rows, []string{
			tool.Baro_time_cell(date),
			baro_gov_log_kind_badge(kind),
			baro_gov_safe(detail),
			baro_gov_actor_html(db, actor, config.IP),
			baro_gov_safe(tool.Baro_trunc(reason, 50)),
			tool.Baro_button("/baro/log/view/"+tool.Url_parser(id), "보기", ""),
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("역할 변경 기록"),
		tool.Baro_t("공개 운영 로그에 남은 이 사람의 권한 변경 이력입니다."),
		tool.Baro_table([]string{"시각", "종류", "역할", tool.Baro_t("담당"), "사유", ""}, log_rows, tool.Baro_t("역할 변경 기록이 없습니다.")),
		tool.Baro_button(baro_gov_url("/baro/log", [][]string{{"q", user}}), tool.Baro_t("공개 로그에서 이 사람 찾기"), ""),
	)

	// 이해충돌 회피 기록
	body += tool.Baro_card_ex(
		tool.Baro_t("이 사람의 이해충돌 회피 기록"),
		tool.Baro_t("사건 당사자이거나 깊이 관여한 사건에서 스스로 빠진 기록입니다."),
		baro_gov_coi_table(db, "", user, config.IP, 20),
		tool.Baro_button(baro_gov_url("/baro/role/coi", [][]string{{"actor", user}}), tool.Baro_t("이해충돌 목록에서 보기"), ""),
	)

	body += tool.Baro_button("/baro/role", tool.Baro_t("역할 목록으로"), "")

	return tool.Baro_page(db, config, user+tool.Baro_t(" 의 역할"), body, baro_gov_menu())
}
