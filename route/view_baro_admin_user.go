package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  16-2 사용자 관리
//
//  0장 원칙 1 : 제재와 콘텐츠 판단은 분리한다.
//  이 화면에는 "이 사용자 문서 전부 삭제" 같은 단추를 두지 않는다.
//  0장 원칙 5 : 가입 IP·이메일은 privacy 권한자에게만 보이고 열람 기록을 남긴다.
// ============================================================

// baro_admin_user_exists : 계정이 실제로 있는지
func baro_admin_user_exists(db *sql.DB, user string) bool {
	if strings.TrimSpace(user) == "" {
		return false
	}

	return baro_admin_one(db, "select id from user_set where id = ? limit 1", user) != ""
}

// baro_admin_user_burst : 최근 편집을 훑어 짧은 시간 안의 대량 편집을 계산한다
//
//	돌려주는 값 : (하루 편집 수, 10분 창 최대 편집 수, 마지막 편집 시각)
func baro_admin_user_burst(db *sql.DB, user string) (int, int, string) {
	list := baro_admin_rows(
		db, 1,
		"select date from history where ip = ? order by date desc limit 500",
		user,
	)

	if len(list) == 0 {
		return 0, 0, ""
	}

	day_ago := tool.Baro_time_add(tool.Baro_now(), -1, 0, 0)

	day_num := 0
	for _, one := range list {
		if one[0] >= day_ago {
			day_num++
		}
	}

	// 10분 창 최대 편집 수 (목록은 최신순이므로 뒤로 훑는다)
	burst := 0
	for i := range list {
		limit_time := tool.Baro_time_add(list[i][0], 0, 0, -10)
		count := 0

		for j := i; j < len(list); j++ {
			if list[j][0] < limit_time {
				break
			}

			count++
		}

		if count > burst {
			burst = count
		}
	}

	return day_num, burst, list[0][0]
}

// baro_admin_user_ip_list : 이 계정이 쓴 IP 목록과 그 출처
//
//	가입 IP 를 따로 저장하지 않는 위키도 있으므로
//	user_set 의 가입 IP -> 접속 기록(ua_d) -> 바로위키 로그인 기록 순서로 찾는다.
func baro_admin_user_ip_list(db *sql.DB, user string) ([]string, string) {
	if register_ip := strings.TrimSpace(baro_admin_one(db, "select data from user_set where id = ? and name = 'register_ip'", user)); register_ip != "" {
		return []string{register_ip}, "가입 IP"
	}

	seen := map[string]bool{}
	out := []string{}

	add := func(ip string) {
		ip = strings.TrimSpace(ip)
		if ip == "" || seen[ip] {
			return
		}

		seen[ip] = true
		out = append(out, ip)
	}

	for _, one := range baro_admin_rows(db, 1, "select distinct ip from ua_d where name = ? limit 20", user) {
		add(one[0])
	}

	source := tool.Baro_t("접속 기록(ua_d)")
	if len(out) == 0 {
		for _, one := range baro_admin_rows(db, 1, "select distinct ip from baro_login_log where user = ? limit 20", user) {
			add(one[0])
		}

		source = tool.Baro_t("로그인 기록(baro_login_log)")
	}

	if len(out) == 0 {
		return []string{}, tool.Baro_t("기록 없음")
	}

	return out, source
}

// baro_admin_user_same_ip : 같은 IP 를 쓰는 다른 계정
//
//	돌려주는 값 : (대표 IP, 다른 계정 목록, 판단 근거)
func baro_admin_user_same_ip(db *sql.DB, user string) (string, []string, string) {
	ip_list, source := baro_admin_user_ip_list(db, user)
	if len(ip_list) == 0 {
		return "", []string{}, source
	}

	seen := map[string]bool{user: true}
	out := []string{}

	for _, ip := range ip_list {
		found := [][]string{}
		found = append(found, baro_admin_rows(
			db, 1,
			"select distinct id from user_set where name = 'register_ip' and data = ? and id != ? order by id limit 30",
			ip, user,
		)...)
		found = append(found, baro_admin_rows(
			db, 1,
			"select distinct name from ua_d where ip = ? and name != ? limit 30",
			ip, user,
		)...)
		found = append(found, baro_admin_rows(
			db, 1,
			"select distinct user from baro_login_log where ip = ? and user != ? limit 30",
			ip, user,
		)...)

		for _, one := range found {
			if one[0] == "" || seen[one[0]] {
				continue
			}

			seen[one[0]] = true
			out = append(out, one[0])
		}
	}

	return ip_list[0], out, source
}

// View_baro_admin_user : 16-2 사용자 검색
func View_baro_admin_user(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if html, stop := baro_admin_gate(db, config, ""); stop {
		return html
	}

	keyword := baro_admin_get(query, "q")
	page := baro_admin_page_num(query)
	size := tool.Baro_page_size

	body := tool.Baro_hero(tool.Baro_t("사용자 관리"), tool.Baro_t("계정 상태·권한·제재 이력을 확인합니다. 개인정보는 별도 권한이 필요합니다."), "")
	body += baro_admin_tabs("/baro/admin/user")

	search_form := tool.Baro_input("q", tool.Baro_t("사용자 이름"), keyword, tool.Baro_t("이름 일부를 넣으면 됩니다"), tool.Baro_t("user_set 의 name(계정 이름) 을 기준으로 찾습니다."))
	search_form += tool.Baro_submit(tool.Baro_t("검색"), "primary")
	body += tool.Baro_card(tool.Baro_t("사용자 검색"), `<form class="bw_form" method="get" action="/baro/admin/user">`+search_form+`</form>`)

	// 계정에 acl 값이 저장되어 있지 않은 위키도 있으므로
	// user_set 에 이름이 남아 있는 계정 전체에서 찾는다.
	list := [][]string{}
	if keyword == "" {
		list = baro_admin_rows(
			db, 1,
			"select distinct id from user_set order by id limit ?, ?",
			(page-1)*size, size,
		)
	} else {
		list = baro_admin_rows(
			db, 1,
			"select distinct id from user_set where id like ? order by id limit ?, ?",
			"%"+keyword+"%", (page-1)*size, size,
		)
	}

	rows := [][]string{}
	for _, one := range list {
		user := one[0]
		auth := tool.Get_user_auth(db, user)

		roles := tool.Baro_user_roles(db, user)
		role_html := `<span class="bw_muted">-</span>`
		if len(roles) > 0 {
			names := []string{}
			for _, role := range roles {
				names = append(names, tool.Baro_role_name(role))
			}

			role_html = tool.Baro_pills(names)
		}

		sanction_num := baro_admin_num(db, "select count(*) from baro_sanction where user = ? and status = 'active'", user)
		sanction_html := tool.Baro_t(`<span class="bw_muted">없음</span>`)
		if sanction_num > 0 {
			sanction_html = tool.Baro_badge(tool.Baro_t("진행 중 ")+tool.Baro_itoa(sanction_num), "danger")
		}

		founder := tool.Baro_founder_number(db, user)
		founder_html := `<span class="bw_muted">-</span>`
		if founder != "" {
			founder_html = tool.Baro_badge("#"+founder, "ok")
		}

		rows = append(rows, []string{
			tool.Baro_user_link(db, user, config.IP),
			baro_admin_esc(auth),
			role_html,
			sanction_html,
			founder_html,
			tool.Baro_button("/baro/admin/user/view/"+tool.Url_parser(user), "상세", ""),
		})
	}

	table := tool.Baro_table(
		[]string{tool.Baro_t("사용자"), tool.Baro_t("권한"), tool.Baro_t("바로위키 역할"), "제재", tool.Baro_t("창립 번호"), ""},
		rows,
		tool.Baro_t("조건에 맞는 계정이 없습니다."),
	)

	if len(rows) == 0 {
		table += tool.Baro_empty(tool.Baro_t("검색 결과가 없습니다. 이름 일부만 넣어 다시 찾아 보세요."))
	}

	paging_url := "/baro/admin/user?page={}"
	if keyword != "" {
		paging_url = "/baro/admin/user?q=" + tool.Url_parser(keyword) + "&page={}"
	}

	table += tool.Baro_paging(page, len(rows), size, paging_url)

	body += tool.Baro_card_ex(tool.Baro_t("계정 목록"), tool.Baro_t("가입한 계정을 이름순으로 보여줍니다."), table, tool.Baro_note(tool.Baro_t("계정 이름을 눌러 상세 화면으로 이동합니다.")))

	return baro_admin_page(db, config, tool.Baro_t("사용자 관리"), body)
}

// View_baro_admin_user_view : 16-2 사용자 상세
func View_baro_admin_user_view(config tool.Config, user string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if html, stop := baro_admin_gate(db, config, ""); stop {
		return html
	}

	user = strings.TrimSpace(user)
	can_privacy := tool.Baro_can(db, config.IP, "privacy")
	can_system := tool.Baro_can(db, config.IP, "system")

	if !baro_admin_user_exists(db, user) {
		body := tool.Baro_hero(tool.Baro_t("사용자 상세"), tool.Baro_t("계정을 찾지 못했습니다."), "")
		body += baro_admin_tabs("/baro/admin/user")
		body += tool.Baro_alert("warn", "<b>"+baro_admin_esc(user)+tool.Baro_t("</b> 이름의 계정을 찾지 못했습니다. 이름이 정확한지 확인해 주세요."))
		body += tool.Baro_card(tool.Baro_t("다시 찾기"), tool.Baro_button("/baro/admin/user", tool.Baro_t("사용자 검색으로"), "primary"))

		return baro_admin_page(db, config, tool.Baro_t("사용자 상세"), body)
	}

	reveal := false
	notice := ""

	// --- 쓰기 동작 ---
	if values != nil {
		do := baro_admin_get(values, "do")
		reason := baro_admin_get(values, "reason")

		switch do {
		case "privacy":
			// 개인정보 열람 : 고권한 + 열람 기록 필수
			if !can_privacy {
				return tool.Baro_deny(db, config, tool.Baro_t("가입 IP·이메일은 개인정보 담당(privacy) 역할만 볼 수 있습니다."))
			}

			if reason == "" {
				notice = tool.Baro_alert("warn", tool.Baro_t("열람 사유를 적어야 개인정보를 볼 수 있습니다."))
				break
			}

			tool.Baro_audit(db, config, "privacy_view", "user:"+user, "", "가입 IP·이메일 열람", reason)
			tool.Baro_public_log(db, "privacy_view", config.IP, "user:"+user, "개인정보 열람 (항목만 공개)", "가입 IP·이메일")
			tool.Baro_notify(db, user, "system", "회원님의 가입 정보가 운영 목적으로 열람되었습니다.", "/baro/log", reason)

			reveal = true
			notice = tool.Baro_alert("ok", tool.Baro_t("열람 기록을 남겼습니다. 이 열람은 감사 로그와 공개 운영 로그에 모두 남습니다."))

		case "role_grant", "role_revoke":
			if !can_system && !tool.Baro_is_owner(db, config.IP) {
				return tool.Baro_deny(db, config, tool.Baro_t("역할을 부여·회수하려면 시스템 관리자(sysadmin) 역할이 필요합니다."))
			}

			role := baro_admin_get(values, "role")
			if role == "" {
				notice = tool.Baro_alert("warn", tool.Baro_t("역할을 고르지 않았습니다."))
				break
			}

			if do == "role_grant" {
				tool.Baro_grant_role(db, user, role, config.IP, reason)
				tool.Baro_audit(db, config, "role_grant", "user:"+user, "", role, reason)
				tool.Baro_notify(db, user, "system", tool.Baro_role_name(role)+" 역할이 부여되었습니다.", "/baro/role", reason)
			} else {
				tool.Baro_revoke_role(db, user, role, config.IP, reason)
				tool.Baro_audit(db, config, "role_revoke", "user:"+user, role, "", reason)
				tool.Baro_notify(db, user, "system", tool.Baro_role_name(role)+" 역할이 회수되었습니다.", "/baro/role", reason)
			}

			return tool.Get_redirect("/baro/admin/user/view/" + tool.Url_parser(user))
		}
	}

	_ = query

	// --- 계정 상태 ---
	auth := tool.Get_user_auth(db, user)
	join_date := baro_admin_one(db, "select data from user_set where id = ? and name = 'date'", user)
	level := tool.Get_level(db, user)
	ban := tool.Get_user_ban(db, user, "")

	ban_html := tool.Baro_badge(tool.Baro_t("정상"), "ok")
	if len(ban) > 0 && ban[0] == "true" {
		ban_html = tool.Baro_badge(tool.Baro_t("위키 차단 중"), "danger")
	}

	edit_num := baro_admin_num(db, "select count(*) from history where ip = ?", user)
	contrib_num := baro_admin_num(db, "select count(*) from baro_contrib_event where user = ?", user)
	badge_num := baro_admin_num(db, "select count(*) from baro_user_badge where user = ?", user)
	founder := tool.Baro_founder_number(db, user)

	founder_html := tool.Baro_t(`<span class="bw_muted">창립 기여자 아님</span>`)
	if founder != "" {
		founder_html = tool.Baro_badge("창립 기여자 #"+founder, "ok")
	}

	state_body := tool.Baro_kv_grid([][]string{
		{"계정", tool.Baro_user_link(db, user, config.IP)},
		{tool.Baro_t("위키 권한"), baro_admin_esc(auth)},
		{tool.Baro_t("차단 상태"), ban_html},
		{tool.Baro_t("가입 시각"), baro_admin_esc(join_date)},
		{tool.Baro_t("레벨"), baro_admin_esc(level[0]) + tool.Baro_t(" (경험치 ") + baro_admin_esc(level[1]) + "/" + baro_admin_esc(level[2]) + ")"},
		{tool.Baro_t("편집 수"), baro_admin_esc(tool.Baro_comma(edit_num))},
		{"기여 이벤트", baro_admin_esc(tool.Baro_comma(contrib_num))},
		{"배지", baro_admin_esc(tool.Baro_comma(badge_num))},
		{tool.Baro_t("창립 시즌"), founder_html},
	})

	// --- 바로위키 역할 ---
	roles := tool.Baro_user_roles(db, user)
	role_map := map[string]bool{}
	for _, role := range roles {
		role_map[role] = true
	}

	role_rows := [][]string{}
	for _, row := range tool.Baro_role_list() {
		have := tool.Baro_badge("없음", "muted")
		if role_map[row[0]] {
			have = tool.Baro_badge(tool.Baro_t("보유"), "ok")
		}

		granted := baro_admin_one(db, "select date from baro_role where user = ? and role = ?", user, row[0])

		role_rows = append(role_rows, []string{
			baro_admin_esc(row[1]),
			have,
			baro_admin_esc(row[2]),
			baro_admin_esc(granted),
		})
	}

	role_body := tool.Baro_table([]string{"역할", tool.Baro_t("보유"), tool.Baro_t("설명"), tool.Baro_t("부여 시각")}, role_rows, tool.Baro_t("역할 정의가 없습니다."))

	if can_system || tool.Baro_is_owner(db, config.IP) {
		options := [][]string{}
		for _, row := range tool.Baro_role_list() {
			options = append(options, []string{row[0], row[1]})
		}

		grant_form := tool.Baro_hidden("do", "role_grant")
		grant_form += tool.Baro_select("role", tool.Baro_t("부여할 역할"), options, "", tool.Baro_t("역할을 나누어 한 사람이 접수부터 재심까지 처리하지 못하게 합니다."))
		grant_form += tool.Baro_input("reason", "사유", "", tool.Baro_t("왜 이 역할을 주는지"), tool.Baro_t("공개 운영 로그에 남습니다."))
		grant_form += tool.Baro_submit(tool.Baro_t("역할 부여"), "primary")

		revoke_form := tool.Baro_hidden("do", "role_revoke")
		revoke_form += tool.Baro_select("role", tool.Baro_t("회수할 역할"), options, "", "")
		revoke_form += tool.Baro_input("reason", "사유", "", tool.Baro_t("왜 회수하는지"), tool.Baro_t("공개 운영 로그에 남습니다."))
		revoke_form += tool.Baro_submit(tool.Baro_t("역할 회수"), "danger")

		action := "/baro/admin/user/view/" + tool.Url_parser(user)
		role_body += tool.Baro_grid([]string{
			tool.Baro_form(action, grant_form),
			tool.Baro_form(action, revoke_form),
		}, 2)
	} else {
		role_body += tool.Baro_note(tool.Baro_t("역할을 바꾸려면 시스템 관리자(sysadmin) 역할이 필요합니다."))
	}

	// --- 제재 이력 ---
	sanction_rows := [][]string{}
	for _, one := range baro_admin_rows(
		db, 8,
		"select id, kind, scope, reason, case_id, actor, end_date, status from baro_sanction where user = ? order by id + 0 desc limit 50",
		user,
	) {
		case_html := `<span class="bw_muted">-</span>`
		if one[4] != "" {
			case_html = `<a href="/baro/case/view/` + tool.Url_parser(one[4]) + `">#` + baro_admin_esc(one[4]) + `</a>`
		}

		scope := one[2]
		if scope == "" {
			scope = tool.Baro_t("전체")
		}

		sanction_rows = append(sanction_rows, []string{
			baro_admin_esc(tool.Baro_sanction_name(one[1])),
			baro_admin_esc(scope),
			baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(one[3]), 40)),
			case_html,
			baro_admin_actor(db, config, one[5]),
			baro_admin_deadline_cell(one[6]),
			baro_admin_status_badge(one[7]),
		})
	}

	sanction_body := tool.Baro_table(
		[]string{"종류", tool.Baro_t("범위"), "사유", "사건", tool.Baro_t("집행자"), tool.Baro_t("만료"), "상태"},
		sanction_rows,
		tool.Baro_t("제재 이력이 없습니다."),
	)
	sanction_body += tool.Baro_note(tool.Baro_t("제재는 사람에게만 적용되며, 제재를 이유로 이 사용자의 문서를 지우지 않습니다. (0장 원칙 1)"))
	sanction_body += tool.Baro_button("/baro/case?target="+tool.Url_parser(user), "이 사용자 사건 보기", "")

	// --- 이의제기 ---
	appeal_rows := [][]string{}
	for _, one := range baro_admin_rows(
		db, 6,
		"select id, case_id, reason, status, reviewer, deadline from baro_appeal where user = ? order by id + 0 desc limit 30",
		user,
	) {
		appeal_rows = append(appeal_rows, []string{
			`<a href="/baro/appeal/view/` + tool.Url_parser(one[0]) + `">#` + baro_admin_esc(one[0]) + `</a>`,
			baro_admin_esc(one[1]),
			baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(one[2]), 40)),
			baro_admin_status_badge(one[3]),
			baro_admin_actor(db, config, one[4]),
			baro_admin_deadline_cell(one[5]),
		})
	}

	appeal_body := tool.Baro_table(
		[]string{"번호", "사건", "사유", "상태", tool.Baro_t("심사자"), "기한"},
		appeal_rows,
		tool.Baro_t("이의제기 기록이 없습니다."),
	)
	appeal_body += tool.Baro_note(tool.Baro_t("이의제기는 원조치 담당자가 아닌 다른 담당자가 심사합니다."))

	// --- 의심 활동 ---
	day_num, burst, last_edit := baro_admin_user_burst(db, user)
	register_ip, same_ip_list, ip_source := baro_admin_user_same_ip(db, user)

	suspicious := []string{}
	if burst >= 20 {
		suspicious = append(suspicious, tool.Baro_alert("danger", tool.Baro_t("10분 안에 <b>")+baro_admin_esc(tool.Baro_itoa(burst))+tool.Baro_t("</b>회 편집한 구간이 있습니다. 자동화 도구 사용 가능성을 확인하세요.")))
	} else if burst >= 10 {
		suspicious = append(suspicious, tool.Baro_alert("warn", tool.Baro_t("10분 안에 ")+baro_admin_esc(tool.Baro_itoa(burst))+tool.Baro_t("회 편집한 구간이 있습니다.")))
	}

	if day_num >= 200 {
		suspicious = append(suspicious, tool.Baro_alert("danger", tool.Baro_t("최근 24시간 편집이 ")+baro_admin_esc(tool.Baro_comma(day_num))+tool.Baro_t("회입니다.")))
	}

	if len(same_ip_list) >= 3 {
		suspicious = append(suspicious, tool.Baro_alert("danger", tool.Baro_t("같은 IP 를 쓰는 계정이 ")+baro_admin_esc(tool.Baro_itoa(len(same_ip_list)))+tool.Baro_t("개 더 있습니다. (근거 : ")+baro_admin_esc(ip_source)+")"))
	} else if len(same_ip_list) > 0 {
		suspicious = append(suspicious, tool.Baro_alert("warn", tool.Baro_t("같은 IP 를 쓰는 계정이 ")+baro_admin_esc(tool.Baro_itoa(len(same_ip_list)))+tool.Baro_t("개 있습니다. (근거 : ")+baro_admin_esc(ip_source)+")"))
	}

	if len(suspicious) == 0 {
		suspicious = append(suspicious, tool.Baro_alert("ok", tool.Baro_t("눈에 띄는 의심 신호가 없습니다.")))
	}

	sus_body := strings.Join(suspicious, "")
	sus_body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("최근 24시간 편집"), tool.Baro_comma(day_num), tool.Baro_t("history 기준")},
		{tool.Baro_t("10분 창 최대 편집"), tool.Baro_comma(burst), tool.Baro_t("연속 편집 밀도")},
		{tool.Baro_t("같은 IP 계정"), tool.Baro_comma(len(same_ip_list)), ip_source + tool.Baro_t(" · 본인 제외")},
		{tool.Baro_t("마지막 편집"), tool.Baro_time_ago(last_edit), last_edit},
	})

	if len(same_ip_list) > 0 {
		if can_privacy {
			links := []string{}
			for _, other := range same_ip_list {
				links = append(links, tool.Baro_user_link(db, other, config.IP)+` <a href="/baro/admin/user/view/`+tool.Url_parser(other)+tool.Baro_t(`">(상세)</a>`))
			}

			sus_body += tool.Baro_section(tool.Baro_t("같은 IP 를 쓰는 계정 (")+baro_admin_esc(ip_source)+")", tool.Baro_list(links))
		} else {
			sus_body += tool.Baro_section(tool.Baro_t("같은 IP 를 쓰는 계정"), tool.Baro_note(tool.Baro_t("계정 수만 표시합니다. 목록을 보려면 개인정보 담당(privacy) 역할이 필요합니다.")))
		}
	}

	sus_body += tool.Baro_note(tool.Baro_t("의심 신호는 판단 근거일 뿐입니다. 곧바로 제재하지 말고 사건을 열어 근거를 남기세요."))

	// --- 개인정보 (고권한) ---
	privacy_body := ""
	action := "/baro/admin/user/view/" + tool.Url_parser(user)

	if !can_privacy {
		privacy_body = tool.Baro_alert("warn", tool.Baro_t("가입 IP 와 이메일은 개인정보 담당(privacy) 역할을 가진 사람만 볼 수 있습니다. 관리자 권한만으로는 열람할 수 없습니다."))
		privacy_body += tool.Baro_note(tool.Baro_t("이 제한은 0장 원칙 5(근거 공개 · 개인정보 보호)에 따른 것입니다."))
	} else if reveal {
		email := baro_admin_one(db, "select data from user_set where id = ? and name = 'email'", user)
		agent := baro_admin_one(db, "select data from user_set where id = ? and name = 'register_agent'", user)

		if email == "" {
			email = tool.Baro_t("(등록된 이메일 없음)")
		}

		ip_label := "가입 IP"
		if ip_source != "가입 IP" {
			ip_label = tool.Baro_t("최근 접속 IP (") + ip_source + ")"
		}

		if register_ip == "" {
			register_ip = tool.Baro_t("(기록 없음)")
		}

		privacy_body = tool.Baro_alert("danger", tool.Baro_t("아래 정보는 개인정보입니다. 화면 갈무리·외부 공유를 금지하며, 열람 사실이 이미 기록되었습니다."))
		privacy_body += tool.Baro_dl([][]string{
			{ip_label, baro_admin_esc(register_ip)},
			{tool.Baro_t("이메일"), baro_admin_esc(email)},
			{tool.Baro_t("가입 시 접속 정보"), baro_admin_esc(tool.Baro_trunc(agent, 200))},
			{tool.Baro_t("열람자"), baro_admin_esc(config.IP)},
			{tool.Baro_t("열람 시각"), baro_admin_esc(tool.Baro_now())},
		})
	} else {
		form := tool.Baro_hidden("do", "privacy")
		form += tool.Baro_textarea("reason", tool.Baro_t("열람 사유 (필수)"), "", 3, tool.Baro_t("예: 사건 #12 다중 계정 확인"), tool.Baro_t("사유는 감사 로그에 그대로 남습니다."))
		form += tool.Baro_submit("가입 IP·이메일 열람", "danger")

		privacy_body = tool.Baro_alert("warn", tool.Baro_t("가입 IP 와 이메일은 기본으로 가려져 있습니다. 열람하면 <b>감사 로그·공개 운영 로그에 기록되고 본인에게 알림이 갑니다.</b>"))
		privacy_body += tool.Baro_form(action, form)
	}

	// --- 화면 조립 ---
	body := tool.Baro_hero(tool.Baro_t("사용자 상세"), user, "")
	body += baro_admin_tabs("/baro/admin/user")
	body += tool.Baro_crumb([][]string{
		{"/baro/admin", tool.Baro_t("운영 대시보드")},
		{"/baro/admin/user", tool.Baro_t("사용자 관리")},
		{"", user},
	})

	if notice != "" {
		body += notice
	}

	body += tool.Baro_card_ex(tool.Baro_t("계정 상태"), tool.Baro_t("openNAMU 계정 정보와 바로위키 활동 요약"), state_body,
		tool.Baro_button("/w/user:"+tool.Url_parser(user), "사용자 문서", "")+" "+
			tool.Baro_button("/baro/profile/"+tool.Url_parser(user), "바로위키 프로필", "")+" "+
			tool.Baro_button("/contribution/user/"+tool.Url_parser(user)+"/document", "기여 목록", ""))

	body += tool.Baro_card_ex(tool.Baro_t("바로위키 역할"), tool.Baro_t("역할 분리 원칙에 따라 권한을 나누어 부여합니다."), role_body, "")
	body += tool.Baro_card_ex("제재 이력", tool.Baro_t("baro_sanction 표를 직접 읽습니다."), sanction_body, "")
	body += tool.Baro_card_ex(tool.Baro_t("이의제기"), tool.Baro_t("baro_appeal 표를 직접 읽습니다."), appeal_body, "")
	body += tool.Baro_card_ex(tool.Baro_t("의심 활동"), tool.Baro_t("짧은 시간 안의 대량 편집과 같은 IP 다중 계정 신호"), sus_body, "")
	body += tool.Baro_danger_zone(tool.Baro_t("개인정보 접근 (고권한)"), privacy_body)

	return baro_admin_page(db, config, tool.Baro_t("사용자 상세 : ")+user, body)
}
