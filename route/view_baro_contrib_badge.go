package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  7-6 배지 — /baro/badge
//
//  배지를 300개 뿌리지 않는다. 의미 있는 것만 둔다.
//  출석 도장판이 아니라 "이 사람이 무엇을 해 왔는가" 를 보여 주는 표시다.
// ============================================================

// baro_contrib_badge_check_limit : 한 번에 검사할 사용자 수
const baro_contrib_badge_check_limit = 200

// baro_contrib_badge_row : 배지 정의 한 줄
type baro_contrib_badge_row struct {
	Key         string
	Name        string
	Description string
	Kind        string
	Icon        string
	Criteria    string
	Sort        string
}

// baro_contrib_badge_list : 배지 정의 목록
func baro_contrib_badge_list(db *sql.DB) []baro_contrib_badge_row {
	out := []baro_contrib_badge_row{}

	rows := tool.Query_DB(db, "select key_name, name, description, kind, icon, criteria, sort from baro_badge")
	defer rows.Close()

	for rows.Next() {
		row := baro_contrib_badge_row{}
		if err := rows.Scan(&row.Key, &row.Name, &row.Description, &row.Kind, &row.Icon, &row.Criteria, &row.Sort); err != nil {
			continue
		}

		out = append(out, row)
	}

	sort.SliceStable(out, func(i int, j int) bool {
		return tool.Str_to_int(out[i].Sort) < tool.Str_to_int(out[j].Sort)
	})

	return out
}

// baro_contrib_badge_get : 배지 하나
func baro_contrib_badge_get(db *sql.DB, key_name string) baro_contrib_badge_row {
	row := baro_contrib_badge_row{}
	tool.QueryRow_DB(
		db,
		"select key_name, name, description, kind, icon, criteria, sort from baro_badge where key_name = ?",
		[]any{&row.Key, &row.Name, &row.Description, &row.Kind, &row.Icon, &row.Criteria, &row.Sort},
		key_name,
	)

	return row
}

// baro_contrib_badge_kind_name : 배지 갈래 표시 이름
func baro_contrib_badge_kind_name(kind string) string {
	switch kind {
	case "founding":
		return tool.Baro_t("창립")
	case "contribution":
		return tool.Baro_t("기여")
	case "quality":
		return tool.Baro_t("품질")
	case "project":
		return "프로젝트"
	case "care":
		return tool.Baro_t("돌봄")
	case "time":
		return tool.Baro_t("시간")
	case "award":
		return tool.Baro_t("어워드")
	}

	return kind
}

// baro_contrib_badge_criteria_text : 조건을 사람이 읽을 수 있는 문장으로
func baro_contrib_badge_criteria_text(criteria string) string {
	key, need := baro_contrib_badge_criteria_parse(criteria)

	switch key {
	case "founder":
		return tool.Baro_t("창립 시즌에 첫 유효 기여를 완료")
	case "award_nominee":
		return tool.Baro_t("어워드 후보로 등록")
	case "active_days":
		return tool.Baro_t("활동한 날 ") + tool.Baro_comma(need) + tool.Baro_t("일")
	case "project_finish":
		return tool.Baro_t("완료된 프로젝트 ") + tool.Baro_comma(need) + tool.Baro_t("개 참여")
	case "restore":
		return tool.Baro_t("격리에서 되살린 문서 ") + tool.Baro_comma(need) + tool.Baro_t("개")
	case "review":
		return tool.Baro_t("검토 기여 ") + tool.Baro_comma(need) + tool.Baro_t("건")
	}

	if baro_contrib_kind_valid(key) {
		return tool.Baro_contrib_kind_name(key) + " " + tool.Baro_comma(need) + tool.Baro_t("건")
	}

	return criteria
}

// baro_contrib_badge_criteria_parse : "종류:개수" 를 나눈다 (개수가 없으면 1)
func baro_contrib_badge_criteria_parse(criteria string) (string, int) {
	criteria = strings.TrimSpace(criteria)
	if criteria == "" {
		return "", 0
	}

	parts := strings.SplitN(criteria, ":", 2)
	key := strings.TrimSpace(parts[0])

	need := 1
	if len(parts) > 1 {
		if num := tool.Str_to_int(strings.TrimSpace(parts[1])); num > 0 {
			need = num
		}
	}

	return key, need
}

// baro_contrib_badge_progress : 사용자의 조건 달성 정도 (달성치, 필요치)
func baro_contrib_badge_progress(db *sql.DB, user string, criteria string) (int, int) {
	key, need := baro_contrib_badge_criteria_parse(criteria)
	if key == "" {
		return 0, 0
	}

	switch key {
	case "founder":
		if tool.Baro_founder_number(db, user) != "" {
			return 1, need
		}

		return 0, need

	case "award_nominee":
		return baro_contrib_count(
			db,
			"select count(*) from baro_nomination where target = ? and target_kind = 'user'",
			user,
		), need

	case "active_days":
		return baro_contrib_count(db, "select count(*) from baro_calendar where user = ?", user), need

	case "project_finish":
		return baro_contrib_count(
			db,
			"select count(*) from baro_project_member m where m.user = ?"+
				" and exists (select 1 from baro_project p where p.id = m.project and p.status = 'done')",
			user,
		), need

	case "restore":
		return baro_contrib_count(
			db,
			"select count(*) from baro_quarantine where actor = ? and status = 'released'",
			user,
		), need
	}

	// 나머지는 기여 행동 종류로 본다
	if !baro_contrib_kind_valid(key) {
		return 0, need
	}

	return baro_contrib_count(
		db,
		"select count(*) from baro_contrib_event where user = ? and kind = ? and status <> 'rejected'",
		user, key,
	), need
}

// baro_contrib_badge_has : 이미 가지고 있는가
func baro_contrib_badge_has(db *sql.DB, user string, badge string) bool {
	found := ""

	return tool.QueryRow_DB(db, "select id from baro_user_badge where user = ? and badge = ?", []any{&found}, user, badge)
}

// baro_contrib_badge_give : 배지 지급 (이미 있으면 아무 일도 하지 않는다)
func baro_contrib_badge_give(db *sql.DB, user string, badge baro_contrib_badge_row, detail string) bool {
	if !tool.Baro_is_user(user) || badge.Key == "" {
		return false
	}

	if baro_contrib_badge_has(db, user, badge.Key) {
		return false
	}

	tool.Exec_DB(
		db,
		"insert into baro_user_badge (id, user, badge, date, detail) values (?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_user_badge"), user, badge.Key, tool.Baro_now(), tool.Baro_trunc(detail, 300),
	)

	tool.Baro_notify(
		db, user, "badge",
		tool.Baro_t("배지를 받았습니다 : ")+badge.Name,
		"/baro/badge/user/"+tool.Url_parser(user),
		badge.Description,
	)

	return true
}

// baro_contrib_badge_user_list : 사용자가 가진 배지
func baro_contrib_badge_user_list(db *sql.DB, user string) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(db, "select badge, date, detail from baro_user_badge where user = ? order by id + 0 desc", user)
	defer rows.Close()

	for rows.Next() {
		badge, date, detail := "", "", ""
		if err := rows.Scan(&badge, &date, &detail); err != nil {
			continue
		}

		out = append(out, []string{badge, date, detail})
	}

	return out
}

// baro_contrib_badge_icon_html : 배지 아이콘과 이름
func baro_contrib_badge_icon_html(row baro_contrib_badge_row) string {
	icon := row.Icon
	if icon == "" {
		icon = "◆"
	}

	return `<b>` + baro_contrib_esc(icon) + ` ` + baro_contrib_esc(row.Name) + `</b>`
}

// ------------------------------------------------------------
//  화면 : /baro/badge
// ------------------------------------------------------------

// View_baro_contrib_badge : 배지 목록과 획득 조건
func View_baro_contrib_badge(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	list := baro_contrib_badge_list(db)
	kind_filter := baro_contrib_get(query, "kind")
	page := baro_contrib_page_num(baro_contrib_get(query, "page"))

	given_total := baro_contrib_count(db, "select count(*) from baro_user_badge")
	holder_total := baro_contrib_count(db, "select count(distinct user) from baro_user_badge")

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", "배지"}})
	body += tool.Baro_hero(
		"배지",
		tool.Baro_t("의미 있는 것만 둡니다. 출석 도장판을 만들지 않습니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("배지 종류"), tool.Baro_comma(len(list)), "개"},
			{tool.Baro_t("지급된 배지"), tool.Baro_comma(given_total), "개"},
			{tool.Baro_t("배지를 가진 사람"), tool.Baro_comma(holder_total), "명"},
		}),
	)
	body += baro_contrib_tabs("/baro/badge")

	body += tool.Baro_alert("info",
		tool.Baro_t("<b>바로위키는 배지를 300개씩 뿌리지 않습니다.</b> ")+
			tool.Baro_t("배지가 흔해지면 아무 의미도 남지 않기 때문입니다. ")+
			tool.Baro_t("여기 있는 배지는 모두 '실제로 무엇을 했는가' 에 붙는 표시이며, 접속 일수나 출석으로는 받을 수 없습니다. ")+
			tool.Baro_t("배지 수로 사람을 줄 세우지도 않습니다."),
	)

	// --- 갈래 탭 ---
	kinds := []string{}
	seen := map[string]bool{}
	for _, row := range list {
		if row.Kind == "" || seen[row.Kind] {
			continue
		}

		seen[row.Kind] = true
		kinds = append(kinds, row.Kind)
	}

	kind_tabs := [][]string{{"/baro/badge", tool.Baro_t("전체")}}
	for _, kind := range kinds {
		kind_tabs = append(kind_tabs, []string{
			baro_contrib_query("/baro/badge", [][]string{{"kind", kind}}),
			baro_contrib_badge_kind_name(kind),
		})
	}

	active := "/baro/badge"
	if kind_filter != "" {
		active = baro_contrib_query("/baro/badge", [][]string{{"kind", kind_filter}})
	}
	body += tool.Baro_tabs(kind_tabs, active)

	// --- 배지 표 ---
	rows := [][]string{}
	for _, row := range list {
		if kind_filter != "" && row.Kind != kind_filter {
			continue
		}

		holders := baro_contrib_count(db, "select count(*) from baro_user_badge where badge = ?", row.Key)

		mine := `<span class="bw_muted">-</span>`
		if tool.Baro_is_user(config.IP) {
			done, need := baro_contrib_badge_progress(db, config.IP, row.Criteria)

			switch {
			case baro_contrib_badge_has(db, config.IP, row.Key):
				mine = tool.Baro_badge(tool.Baro_t("보유"), "ok")
			case need > 0 && done >= need:
				mine = tool.Baro_badge(tool.Baro_t("조건 충족"), "warn")
			case need > 0:
				mine = `<span class="bw_muted">` + tool.Baro_comma(done) + " / " + tool.Baro_comma(need) + `</span>`
			}
		}

		rows = append(rows, []string{
			baro_contrib_badge_icon_html(row),
			baro_contrib_esc(baro_contrib_badge_kind_name(row.Kind)),
			baro_contrib_esc(row.Description),
			baro_contrib_esc(baro_contrib_badge_criteria_text(row.Criteria)),
			tool.Baro_comma(holders) + tool.Baro_t("명"),
			mine,
		})
	}

	table_html := tool.Baro_table(
		[]string{"배지", tool.Baro_t("갈래"), tool.Baro_t("설명"), tool.Baro_t("받는 조건"), tool.Baro_t("가진 사람"), tool.Baro_t("내 상태")},
		rows,
		tool.Baro_t("등록된 배지가 없습니다."),
	)
	if len(rows) == 0 {
		table_html = tool.Baro_empty(tool.Baro_t("이 갈래에 해당하는 배지가 없습니다."))
	}

	foot := tool.Baro_button("/baro/badge/check", tool.Baro_t("조건 확인하고 받기"), "primary")
	if tool.Baro_is_user(config.IP) {
		foot += " " + tool.Baro_button("/baro/badge/user/"+tool.Url_parser(config.IP), "내 배지 보기", "")
	}

	body += tool.Baro_card_ex(tool.Baro_t("배지 목록"), tool.Baro_t("조건은 모두 공개합니다."), table_html, foot)

	// --- 최근 지급 ---
	recent_rows := [][]string{}

	recent := tool.Query_DB(
		db,
		"select user, badge, date from baro_user_badge order by id + 0 desc limit ?, ?",
		(page-1)*baro_contrib_page_size, baro_contrib_page_size,
	)
	defer recent.Close()

	for recent.Next() {
		user, badge, date := "", "", ""
		if err := recent.Scan(&user, &badge, &date); err != nil {
			continue
		}

		info := baro_contrib_badge_get(db, badge)
		if info.Key == "" {
			info = baro_contrib_badge_row{Key: badge, Name: badge}
		}

		recent_rows = append(recent_rows, []string{
			tool.Baro_time_cell(date),
			tool.Baro_user_link(db, user, config.IP),
			baro_contrib_badge_icon_html(info),
		})
	}

	recent_html := tool.Baro_table([]string{"시각", tool.Baro_t("사람"), "배지"}, recent_rows, tool.Baro_t("아직 지급된 배지가 없습니다."))
	if len(recent_rows) == 0 {
		recent_html = tool.Baro_empty(tool.Baro_t("아직 지급된 배지가 없습니다. 조건을 채우면 자동으로 지급됩니다."))
	}

	recent_html += tool.Baro_paging(page, len(recent_rows), baro_contrib_page_size,
		baro_contrib_query("/baro/badge", [][]string{{"kind", kind_filter}, {"page", "{}"}}))

	body += tool.Baro_card_ex(tool.Baro_t("최근 지급"), tool.Baro_t("받은 순서대로 보여 줍니다. 많이 가진 순으로 줄 세우지 않습니다."), recent_html, "")

	return tool.Baro_page(db, config, "배지", body, baro_contrib_menu())
}

// ------------------------------------------------------------
//  화면 : /baro/badge/user/<사용자>
// ------------------------------------------------------------

// View_baro_contrib_badge_user : 한 사람의 배지
func View_baro_contrib_badge_user(config tool.Config, user_raw string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	user := baro_contrib_user_param(user_raw)

	if user == "" {
		body := tool.Baro_crumb([][]string{{"/baro/badge", "배지"}, {"", tool.Baro_t("사용자")}})
		body += tool.Baro_alert("warn", tool.Baro_t("사용자 이름이 비어 있습니다."))
		body += tool.Baro_button("/baro/badge", tool.Baro_t("배지 목록으로"), "")

		return tool.Baro_page(db, config, tool.Baro_t("사용자 배지"), body, baro_contrib_menu())
	}

	owned := baro_contrib_badge_user_list(db, user)
	owned_set := map[string][]string{}
	for _, row := range owned {
		owned_set[row[0]] = row
	}

	list := baro_contrib_badge_list(db)

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/badge", "배지"},
		{"", tool.Namespace_display(user)},
	})

	body += tool.Baro_hero(
		user+tool.Baro_t(" 님의 배지"),
		tool.Baro_t("배지 수로 사람을 비교하지 않습니다. 무엇을 해 왔는지를 보여 줄 뿐입니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("가진 배지"), tool.Baro_comma(len(owned)), "개"},
			{tool.Baro_t("전체 배지"), tool.Baro_comma(len(list)), "개"},
		}),
	)

	body += tool.Baro_tabs([][]string{
		{"/baro/contrib/user/" + tool.Url_parser(user), "기여 기록"},
		{"/baro/contrib/impact/" + tool.Url_parser(user), "기여 영향력"},
		{"/baro/contrib/calendar/" + tool.Url_parser(user), "활동 캘린더"},
		{"/baro/badge/user/" + tool.Url_parser(user), "배지"},
	}, "/baro/badge/user/"+tool.Url_parser(user))

	if !tool.Baro_is_user(user) {
		body += tool.Baro_alert("info", tool.Baro_t("로그인하지 않은 편집(IP 편집)에는 배지가 붙지 않습니다."))
	}

	// --- 가진 배지 ---
	owned_cards := []string{}
	for _, row := range list {
		got, has := owned_set[row.Key]
		if !has {
			continue
		}

		owned_cards = append(owned_cards, tool.Baro_card_ex(
			row.Icon+" "+row.Name,
			baro_contrib_badge_kind_name(row.Kind),
			baro_contrib_esc(row.Description),
			tool.Baro_note(tool.Baro_t("받은 때 : ")+got[1]),
		))
	}

	owned_html := tool.Baro_grid(owned_cards, 3)
	if len(owned_cards) == 0 {
		owned_html = tool.Baro_empty(tool.Baro_t("아직 받은 배지가 없습니다. 배지가 없다고 해서 기여가 없다는 뜻은 아닙니다."))
	}

	body += tool.Baro_card(tool.Baro_t("가진 배지"), owned_html)

	// --- 아직 못 받은 배지와 진행 상황 ---
	rest_rows := [][]string{}
	for _, row := range list {
		if _, has := owned_set[row.Key]; has {
			continue
		}

		done, need := baro_contrib_badge_progress(db, user, row.Criteria)

		progress := `<span class="bw_muted">-</span>`
		if need > 0 {
			progress = tool.Baro_progress(done, need, "")
		}

		rest_rows = append(rest_rows, []string{
			baro_contrib_badge_icon_html(row),
			baro_contrib_esc(baro_contrib_badge_criteria_text(row.Criteria)),
			progress,
		})
	}

	rest_html := tool.Baro_table([]string{"배지", tool.Baro_t("받는 조건"), tool.Baro_t("진행")}, rest_rows, tool.Baro_t("모든 배지를 받았습니다."))
	if len(rest_rows) == 0 {
		rest_html = tool.Baro_empty(tool.Baro_t("남은 배지가 없습니다."))
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("아직 받지 않은 배지"),
		tool.Baro_t("진행 막대는 재촉이 아니라 안내입니다."),
		rest_html,
		tool.Baro_button("/baro/badge/check", tool.Baro_t("조건 확인하고 받기"), ""),
	)

	return tool.Baro_page(db, config, user+tool.Baro_t(" 님의 배지"), body, baro_contrib_menu())
}

// ------------------------------------------------------------
//  화면 : /baro/badge/check
// ------------------------------------------------------------

// View_baro_contrib_badge_check : 조건을 만족한 배지 자동 지급
func View_baro_contrib_badge_check(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/badge/check"); need {
		return wall
	}

	list := baro_contrib_badge_list(db)
	can_all := tool.Baro_can(db, config.IP, "system")

	// --- 실행 ---
	if values != nil {
		if tool.Baro_read_only(db) {
			body := tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 실행할 수 없습니다. ")+baro_contrib_esc(tool.Baro_read_only_reason(db)))

			return tool.Baro_page(db, config, tool.Baro_t("배지 조건 확인"), body, baro_contrib_menu())
		}

		scope := strings.TrimSpace(values.Get("scope"))
		if scope == "all" && !can_all {
			return tool.Baro_deny(db, config, tool.Baro_t("전체 사용자 검사는 시스템 관리자만 실행할 수 있습니다."))
		}

		if !tool.Baro_rate_ok(db, "badge_check", config.IP, 30, 3600) {
			body := tool.Baro_alert("warn", tool.Baro_t("너무 자주 검사했습니다. 잠시 뒤에 다시 시도해 주세요."))

			return tool.Baro_page(db, config, tool.Baro_t("배지 조건 확인"), body, baro_contrib_menu())
		}

		targets := []string{config.IP}

		if scope == "all" {
			targets = []string{}

			rows := tool.Query_DB(
				db,
				"select distinct user from baro_contrib_event order by user limit ?, ?",
				0, baro_contrib_badge_check_limit,
			)
			defer rows.Close()

			for rows.Next() {
				user := ""
				if err := rows.Scan(&user); err != nil {
					continue
				}

				if baro_contrib_is_person(user) {
					targets = append(targets, user)
				}
			}
		}

		given := 0
		for _, user := range targets {
			for _, badge := range list {
				done, need := baro_contrib_badge_progress(db, user, badge.Criteria)
				if need <= 0 || done < need {
					continue
				}

				if baro_contrib_badge_give(db, user, badge, tool.Baro_t("조건 충족 : ")+baro_contrib_badge_criteria_text(badge.Criteria)) {
					given++
				}
			}
		}

		if scope == "all" {
			tool.Baro_public_log(db, "badge_check", config.IP, "baro_user_badge",
				"배지 조건 일괄 확인", tool.Baro_itoa(given)+"개 지급")
			tool.Baro_audit(db, config, "badge_check", "baro_user_badge", "",
				tool.Baro_itoa(given)+"개 지급", "사용자 "+tool.Baro_itoa(len(targets))+"명 검사")
		} else {
			tool.Baro_audit(db, config, "badge_check_self", config.IP, "", tool.Baro_itoa(given)+"개 지급", "본인 배지 조건 확인")
		}

		tool.Baro_metric_bump(db, "badge_given", tool.Baro_today(), given)

		return tool.Get_redirect(baro_contrib_query("/baro/badge/check", [][]string{
			{"given", tool.Baro_itoa(given)},
			{"users", tool.Baro_itoa(len(targets))},
		}))
	}

	body := tool.Baro_crumb([][]string{{"/baro/badge", "배지"}, {"", tool.Baro_t("조건 확인")}})
	body += tool.Baro_hero(
		tool.Baro_t("배지 조건 확인"),
		tool.Baro_t("조건을 만족한 배지를 찾아 지급합니다. 지급되면 알림이 갑니다."),
		"",
	)
	body += baro_contrib_tabs("/baro/badge")

	if given := baro_contrib_get(query, "given"); given != "" {
		if tool.Str_to_int(given) > 0 {
			body += tool.Baro_alert("ok", tool.Baro_t("배지 <b>")+baro_contrib_esc(given)+tool.Baro_t("개</b>를 지급했습니다. (검사한 사람 ")+
				baro_contrib_esc(baro_contrib_get(query, "users"))+tool.Baro_t("명)"))
		} else {
			body += tool.Baro_alert("info", tool.Baro_t("새로 지급할 배지가 없었습니다. 조건을 채우면 다음 검사에서 받습니다."))
		}
	}

	// --- 내 진행 상황 ---
	rows := [][]string{}
	ready := 0

	for _, badge := range list {
		done, need := baro_contrib_badge_progress(db, config.IP, badge.Criteria)

		state := `<span class="bw_muted">-</span>`
		switch {
		case baro_contrib_badge_has(db, config.IP, badge.Key):
			state = tool.Baro_badge(tool.Baro_t("이미 보유"), "ok")
		case need > 0 && done >= need:
			state = tool.Baro_badge(tool.Baro_t("지금 받을 수 있음"), "warn")
			ready++
		case need > 0:
			state = `<span class="bw_muted">` + tool.Baro_comma(done) + " / " + tool.Baro_comma(need) + `</span>`
		}

		rows = append(rows, []string{
			baro_contrib_badge_icon_html(badge),
			baro_contrib_esc(baro_contrib_badge_criteria_text(badge.Criteria)),
			state,
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("내 조건 진행 상황"),
		tool.Baro_t("지금 받을 수 있는 배지 ")+tool.Baro_itoa(ready)+tool.Baro_t("개"),
		tool.Baro_table([]string{"배지", tool.Baro_t("조건"), "상태"}, rows, tool.Baro_t("등록된 배지가 없습니다.")),
		tool.Baro_form("/baro/badge/check",
			tool.Baro_hidden("scope", "me")+tool.Baro_submit(tool.Baro_t("내 배지 조건 확인"), "primary"),
		),
	)

	if can_all {
		body += tool.Baro_danger_zone(tool.Baro_t("전체 사용자 검사"),
			tool.Baro_note(tool.Baro_t("기여 기록이 있는 사용자 최대 ")+tool.Baro_comma(baro_contrib_badge_check_limit)+tool.Baro_t("명을 한 번에 검사합니다. 지급 결과는 공개 운영 로그에 남습니다."))+
				tool.Baro_form("/baro/badge/check",
					tool.Baro_hidden("scope", "all")+tool.Baro_submit(tool.Baro_t("전체 사용자 검사 실행"), ""),
				),
		)
	}

	body += tool.Baro_card(tool.Baro_t("배지 방침"), tool.Baro_list([]string{
		tool.Baro_t("배지는 <b>의미 있는 것만</b> 둡니다. 수를 늘리지 않습니다."),
		tool.Baro_t("접속 일수나 출석 자체로 받는 배지는 만들지 않습니다."),
		tool.Baro_t("배지 개수로 순위를 매기지 않습니다."),
		tool.Baro_t("조건은 전부 공개하며, 조건을 채우면 자동으로 지급합니다."),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("배지 조건 확인"), body, baro_contrib_menu())
}
