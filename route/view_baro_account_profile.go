package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  2-4 사용자 프로필
// ============================================================

// baro_account_profile_item_list : 공개 여부를 고를 수 있는 프로필 항목
func baro_account_profile_item_list() [][]string {
	return [][]string{
		{"join", tool.Baro_t("가입일")},
		{"founder", tool.Baro_t("창립 기여자 번호")},
		{"interest", tool.Baro_t("관심 분야")},
		{"expertise", tool.Baro_t("전문 분야")},
		{"docs", tool.Baro_t("작성한 문서")},
		{"best", tool.Baro_t("대표 문서")},
		{"citation", tool.Baro_t("추가한 출처 수")},
		{"contrib", tool.Baro_t("유효 기여 수")},
		{"impact", tool.Baro_t("문서 열람 영향력")},
		{"project", tool.Baro_t("참여 프로젝트")},
		{"badge", "배지"},
		{"thanks", tool.Baro_t("받은 감사")},
		{"calendar", "활동 캘린더"},
	}
}

// baro_account_profile_public : 항목별 공개 설정 (기록이 없으면 모두 공개)
func baro_account_profile_public(db *sql.DB, user string) map[string]bool {
	out := map[string]bool{}
	raw := baro_account_uset_get(db, user, "profile_public")

	if strings.TrimSpace(raw) == "" {
		for _, row := range baro_account_profile_item_list() {
			out[row[0]] = true
		}

		return out
	}

	saved := tool.Baro_json_dec_str(raw)
	for _, row := range baro_account_profile_item_list() {
		out[row[0]] = saved[row[0]] == "on"
	}

	return out
}

// baro_account_profile_hidden : 비공개 항목 안내
func baro_account_profile_hidden(name string) string {
	return tool.Baro_card(name, tool.Baro_note(tool.Baro_t("이 항목은 본인이 비공개로 설정했습니다.")))
}

// baro_account_profile_best_docs : 대표 문서 목록
func baro_account_profile_best_docs(db *sql.DB, user string) []string {
	return tool.Baro_json_dec_list(baro_account_uset_get(db, user, "best_docs"))
}

// baro_account_profile_citation_count : 이 사용자가 붙인 출처 수 (baro_citation 기준)
//
//	baro_citation 에는 사용자 칸이 없다. 그래서 이 사람의 기여 이벤트(문서 + 판)와 짝이 맞는
//	출처를 세어 준다. 되돌려진(rejected) 기여는 빼고, 같은 출처를 두 번 세지 않는다.
func baro_account_profile_citation_count(db *sql.DB, user string) int {
	count := 0
	tool.QueryRow_DB(
		db,
		"select count(distinct c.id) from baro_citation c join baro_contrib_event e "+
			"on e.title = c.title and (e.rev = c.rev or c.rev = '' or e.rev = '') "+
			"where e.user = ? and e.status != 'rejected'",
		[]any{&count}, user,
	)

	return count
}

// View_baro_account_profile : 사용자 프로필 (/baro/profile/<사용자이름>)
func View_baro_account_profile(config tool.Config, user string, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	user = strings.TrimSpace(user)
	_ = values

	if user == "" {
		if tool.Baro_is_user(config.IP) {
			return tool.Get_redirect("/baro/profile/" + tool.Url_parser(config.IP))
		}

		wall, _ := tool.Baro_login_wall(db, config, "/baro/profile")

		return wall
	}

	if !tool.Baro_is_user(user) {
		body := tool.Baro_alert("info", tool.Baro_t("바로위키 프로필은 로그인 사용자에게만 있습니다. IP 기여 기록은 위키 본체에서 확인하세요."))
		body += tool.Baro_button("/baro", tool.Baro_t("바로위키 홈"), "primary")

		return tool.Baro_page(db, config, tool.Baro_t("프로필"), body, baro_account_menu())
	}

	mine := config.IP == user
	public := baro_account_profile_public(db, user)

	// 계정이 실제로 있는지 (없어도 500 이 아니라 안내 화면)
	exists := ""
	has_account := tool.QueryRow_DB(db, "select id from user_set where id = ? limit 1", []any{&exists}, user)
	has_activity := baro_account_contrib_count(db, user) > 0

	if !has_account && !has_activity {
		body := tool.Baro_alert("warn", tool.Baro_t("그런 사용자를 찾지 못했습니다. 이름을 다시 확인해 주세요."))
		body += tool.Baro_dl([][]string{{tool.Baro_t("찾은 이름"), tool.HTML_escape(user)}})
		body += tool.Baro_button("/baro/rank", tool.Baro_t("기여자 순위 보기"), "primary") + " " + tool.Baro_button("/baro", tool.Baro_t("바로위키 홈"), "")

		return tool.Baro_page(db, config, tool.Baro_t("프로필"), body, baro_account_menu())
	}

	bio := baro_account_uset_get(db, user, "bio")
	founder := tool.Baro_founder_number(db, user)
	rank_public := baro_account_uset_get(db, user, "rank_public") != "off"

	hero_sub := bio
	if strings.TrimSpace(hero_sub) == "" {
		hero_sub = tool.Baro_t("아직 한 줄 소개가 없습니다.")
	}

	hero_action := tool.Baro_button("/w/"+tool.Url_parser(tool.Namespace_normalize("사용자:"+user)), "사용자 소개 문서", "") + " " +
		tool.Baro_button("/baro/thanks?to="+tool.Url_parser(user), "감사 보내기", "")
	if mine {
		hero_action = tool.Baro_button("/baro/profile/edit", tool.Baro_t("프로필 설정"), "primary") + " " +
			tool.Baro_button("/baro/account", tool.Baro_t("내 계정"), "") + " " + hero_action
	}

	body := tool.Baro_hero(user+tool.Baro_t(" 님의 기여"), hero_sub, hero_action)
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/rank", tool.Baro_t("기여자")}, {"", user}})

	if mine {
		body += baro_account_tabs("")
	}

	// --- 요약 지표 ---
	stats := [][]string{}
	if public["contrib"] || mine {
		stats = append(stats, []string{tool.Baro_t("유효 기여"), tool.Baro_comma(baro_account_contrib_count(db, user)), "건"})
	}
	if public["docs"] || mine {
		doc_count := 0
		tool.QueryRow_DB(db, "select count(*) from baro_contrib_event where user = ? and kind = 'new_document' and status != 'rejected'", []any{&doc_count}, user)
		stats = append(stats, []string{tool.Baro_t("작성한 문서"), tool.Baro_comma(doc_count), "건"})
	}
	if public["citation"] || mine {
		stats = append(stats, []string{tool.Baro_t("추가한 출처"), tool.Baro_comma(baro_account_profile_citation_count(db, user)), "건"})
	}
	if public["impact"] || mine {
		stats = append(stats, []string{tool.Baro_t("누적 문서 열람"), tool.Baro_comma(tool.Baro_impact_get(db, user, "view")), tool.Baro_t("회")})
	}

	if len(stats) > 0 {
		body += tool.Baro_stat_row(stats)
	}

	// --- 기본 정보 ---
	pairs := [][]string{}

	if public["join"] || mine {
		join_date := baro_account_join_date(db, user)
		if join_date == "" {
			pairs = append(pairs, []string{tool.Baro_t("가입일"), tool.Baro_t(`<span class="bw_muted">기록이 없습니다.</span>`)})
		} else {
			pairs = append(pairs, []string{tool.Baro_t("가입일"), tool.HTML_escape(tool.Baro_date_only(join_date)) + ` <span class="bw_muted">(` + tool.HTML_escape(tool.Baro_time_ago(join_date)) + `)</span>`})
		}
	} else {
		pairs = append(pairs, []string{tool.Baro_t("가입일"), tool.Baro_t(`<span class="bw_muted">비공개</span>`)})
	}

	if public["founder"] || mine {
		if founder == "" {
			pairs = append(pairs, []string{tool.Baro_t("창립 기여자 번호"), tool.Baro_t(`<span class="bw_muted">없습니다.</span>`)})
		} else {
			pairs = append(pairs, []string{tool.Baro_t("창립 기여자 번호"), tool.Baro_badge("#"+founder, "ok")})
		}
	}

	last_active := baro_account_last_active(db, user)
	if last_active == "" {
		pairs = append(pairs, []string{tool.Baro_t("마지막 활동"), tool.Baro_t(`<span class="bw_muted">기여 기록이 없습니다.</span>`)})
	} else {
		pairs = append(pairs, []string{tool.Baro_t("마지막 활동"), tool.Baro_time_cell(last_active)})
	}

	pairs = append(pairs, []string{tool.Baro_t("소개 문서"), baro_account_user_doc_link(user)})

	if !rank_public {
		pairs = append(pairs, []string{tool.Baro_t("공개 순위"), tool.Baro_badge(tool.Baro_t("순위 집계 제외"), "muted") + tool.Baro_t(` <span class="bw_muted">본인이 공개 랭킹 참여를 껐습니다.</span>`)})
	} else {
		pairs = append(pairs, []string{tool.Baro_t("공개 순위"), tool.Baro_t(`<a href="/baro/rank">기여자 순위에 참여 중</a>`)})
	}

	body += tool.Baro_card(tool.Baro_t("기본 정보"), tool.Baro_dl(pairs))

	// --- 분야 ---
	if public["interest"] || public["expertise"] || mine {
		field_html := ""

		if public["interest"] || mine {
			names := []string{}
			for _, field := range tool.Baro_user_interest(db, user) {
				names = append(names, tool.Baro_field_name(db, field))
			}

			if len(names) == 0 {
				field_html += tool.Baro_section(tool.Baro_t("관심 분야"), tool.Baro_empty(tool.Baro_t("아직 고른 관심 분야가 없습니다.")))
			} else {
				field_html += tool.Baro_section(tool.Baro_t("관심 분야"), tool.Baro_pills(names))
			}
		}

		if public["expertise"] || mine {
			names := []string{}
			for _, field := range baro_account_expertise_main(db, user) {
				names = append(names, tool.Baro_field_name(db, field))
			}

			if len(names) == 0 {
				field_html += tool.Baro_section(tool.Baro_t("전문 분야"), tool.Baro_empty(tool.Baro_t("아직 정한 전문 분야가 없습니다.")))
			} else {
				field_html += tool.Baro_section(tool.Baro_t("전문 분야"), tool.Baro_pills(names))
			}
		}

		body += tool.Baro_card("분야", field_html)
	} else {
		body += baro_account_profile_hidden("분야")
	}

	// --- 대표 문서 ---
	if public["best"] || mine {
		docs := baro_account_profile_best_docs(db, user)
		items := []string{}
		for _, doc := range docs {
			items = append(items, tool.Baro_doc_link(doc)+" "+tool.Baro_status_badge(tool.Baro_doc_status_get(db, doc)))
		}

		if len(items) == 0 {
			body += tool.Baro_card(tool.Baro_t("대표 문서"), tool.Baro_empty(tool.Baro_t("아직 대표 문서를 고르지 않았습니다.")))
		} else {
			body += tool.Baro_card(tool.Baro_t("대표 문서"), tool.Baro_list(items))
		}
	} else {
		body += baro_account_profile_hidden(tool.Baro_t("대표 문서"))
	}

	// --- 작성한 문서 ---
	if public["docs"] || mine {
		body += baro_account_profile_doc_card(db, user, page)
	} else {
		body += baro_account_profile_hidden(tool.Baro_t("작성한 문서"))
	}

	// --- 배지 ---
	if public["badge"] || mine {
		body += tool.Baro_card("배지", baro_account_profile_badge_html(db, user))
	} else {
		body += baro_account_profile_hidden("배지")
	}

	// --- 참여 프로젝트 ---
	if public["project"] || mine {
		body += tool.Baro_card(tool.Baro_t("참여 프로젝트"), baro_account_profile_project_html(db, user))
	} else {
		body += baro_account_profile_hidden(tool.Baro_t("참여 프로젝트"))
	}

	// --- 받은 감사 ---
	if public["thanks"] || mine {
		body += tool.Baro_card(tool.Baro_t("감사받은 기여"), baro_account_profile_thanks_html(db, config, user))
	} else {
		body += baro_account_profile_hidden(tool.Baro_t("감사받은 기여"))
	}

	// --- 활동 캘린더 ---
	if public["calendar"] || mine {
		body += tool.Baro_card(tool.Baro_t("활동 캘린더 (최근 1년)"), baro_account_calendar_html(db, user))
	} else {
		body += baro_account_profile_hidden("활동 캘린더")
	}

	if mine {
		body += tool.Baro_note(tool.Baro_t("비공개로 설정한 항목은 다른 사람에게 보이지 않습니다. 지금 보시는 화면에는 본인이라 모두 표시됩니다."))
	}

	return tool.Baro_page(db, config, user+tool.Baro_t(" 님의 기여"), body, baro_account_menu())
}

// baro_account_profile_doc_card : 작성한 문서 목록 (쪽 넘기기 포함)
func baro_account_profile_doc_card(db *sql.DB, user string, page string) string {
	page_int := baro_account_page_int(page)
	max_count := tool.Baro_page_size

	rows := tool.Query_DB(
		db,
		"select title, date, status, field from baro_contrib_event where user = ? and kind = 'new_document' and status != 'rejected' order by id + 0 desc limit ?, ?",
		user, (page_int-1)*max_count, max_count,
	)
	defer rows.Close()

	list_rows := [][]string{}
	for rows.Next() {
		title, date, status, field := "", "", "", ""
		if err := rows.Scan(&title, &date, &status, &field); err != nil {
			continue
		}

		list_rows = append(list_rows, []string{
			tool.Baro_doc_link(title),
			tool.HTML_escape(tool.Baro_field_name(db, field)),
			tool.Baro_status_badge(tool.Baro_doc_status_get(db, title)),
			tool.Baro_badge(status, "muted"),
			tool.Baro_time_cell(date),
		})
	}

	html := ""
	if len(list_rows) == 0 {
		html = tool.Baro_empty(tool.Baro_t("아직 작성한 문서가 없습니다."))
	} else {
		html = tool.Baro_table([]string{"문서", "분야", tool.Baro_t("문서 상태"), tool.Baro_t("기여 상태"), tool.Baro_t("작성")}, list_rows, tool.Baro_t("아직 작성한 문서가 없습니다."))
	}

	html += tool.Baro_paging(page_int, len(list_rows), max_count, "/baro/profile/"+tool.Url_parser(user)+"?page={}")
	html += tool.Baro_note(tool.Baro_t("기여 상태가 pending 이면 아직 점수가 확정되지 않은 기여입니다."))

	return tool.Baro_card(tool.Baro_t("작성한 문서"), html)
}

// baro_account_profile_badge_html : 배지 목록 (baro_user_badge + baro_badge)
func baro_account_profile_badge_html(db *sql.DB, user string) string {
	rows := tool.Query_DB(
		db,
		"select b.icon, b.name, b.description, b.kind, u.date, u.detail from baro_user_badge u left join baro_badge b on b.key_name = u.badge where u.user = ? order by u.id + 0 desc",
		user,
	)
	defer rows.Close()

	items := []string{}
	for rows.Next() {
		icon, name, description, kind, date, detail := "", "", "", "", "", ""
		if err := rows.Scan(&icon, &name, &description, &kind, &date, &detail); err != nil {
			continue
		}

		if name == "" {
			name = tool.Baro_t("알 수 없는 배지")
		}

		item := `<span class="bw_founder">` + tool.HTML_escape(icon) + `</span> <b>` + tool.HTML_escape(name) + `</b> ` +
			tool.Baro_badge(kind, "info") + `<div class="bw_note">` + tool.HTML_escape(description) + `</div>` +
			`<div class="bw_note">` + tool.HTML_escape(tool.Baro_date_only(date)) + " " + tool.HTML_escape(tool.Baro_trunc(detail, 60)) + `</div>`

		items = append(items, item)
	}

	if len(items) == 0 {
		return tool.Baro_empty(tool.Baro_t("아직 받은 배지가 없습니다.")) + tool.Baro_button("/baro/badge", tool.Baro_t("배지 목록 보기"), "")
	}

	return tool.Baro_grid(items, 3) + tool.Baro_button("/baro/badge", tool.Baro_t("배지 목록 보기"), "")
}

// baro_account_profile_project_html : 참여 프로젝트 목록
func baro_account_profile_project_html(db *sql.DB, user string) string {
	rows := tool.Query_DB(
		db,
		"select project, role, date, status from baro_project_member where user = ? order by id + 0 desc limit 30",
		user,
	)
	defer rows.Close()

	list_rows := [][]string{}
	for rows.Next() {
		project, role, date, status := "", "", "", ""
		if err := rows.Scan(&project, &role, &date, &status); err != nil {
			continue
		}

		name, field, project_status := "", "", ""
		tool.QueryRow_DB(db, "select name, field, status from baro_project where id = ?", []any{&name, &field, &project_status}, project)

		if name == "" {
			name = project + tool.Baro_t("번 프로젝트")
		}

		list_rows = append(list_rows, []string{
			`<a href="/baro/project/view/` + tool.Url_parser(project) + `">` + tool.HTML_escape(name) + `</a>`,
			tool.HTML_escape(tool.Baro_field_name(db, field)),
			tool.HTML_escape(baro_account_project_role_name(role)),
			tool.Baro_badge(status, "info"),
			tool.Baro_time_cell(date),
		})
	}

	if len(list_rows) == 0 {
		return tool.Baro_empty(tool.Baro_t("참여 중인 프로젝트가 없습니다.")) + tool.Baro_button("/baro/project", tool.Baro_t("프로젝트 둘러보기"), "")
	}

	return tool.Baro_table([]string{"프로젝트", "분야", "역할", "상태", tool.Baro_t("참여")}, list_rows, tool.Baro_t("참여 중인 프로젝트가 없습니다."))
}

// baro_account_project_role_name : 프로젝트 역할 이름 (6-5 의 역할 값)
func baro_account_project_role_name(role string) string {
	switch role {
	case "member":
		return tool.Baro_t("참가자")
	case "reviewer":
		return tool.Baro_t("검토자")
	case "source":
		return tool.Baro_t("자료 담당")
	case "file":
		return tool.Baro_t("파일 담당")
	case "owner", "manager":
		return tool.Baro_t("프로젝트 운영자")
	}

	if strings.TrimSpace(role) == "" {
		return tool.Baro_t("참가자")
	}

	return role
}

// baro_account_profile_thanks_html : 공개된 감사 기록
func baro_account_profile_thanks_html(db *sql.DB, config tool.Config, user string) string {
	rows := tool.Query_DB(
		db,
		"select from_user, title, reason, date from baro_thanks where to_user = ? and is_public = 'on' order by id + 0 desc limit 20",
		user,
	)
	defer rows.Close()

	list_rows := [][]string{}
	for rows.Next() {
		from_user, title, reason, date := "", "", "", ""
		if err := rows.Scan(&from_user, &title, &reason, &date); err != nil {
			continue
		}

		list_rows = append(list_rows, []string{
			tool.Baro_user_link(db, from_user, config.IP),
			tool.Baro_doc_link(title),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(reason), 80)),
			tool.Baro_time_cell(date),
		})
	}

	if len(list_rows) == 0 {
		return tool.Baro_empty(tool.Baro_t("아직 공개된 감사 기록이 없습니다."))
	}

	return tool.Baro_table([]string{tool.Baro_t("보낸 사람"), "문서", tool.Baro_t("이유"), tool.Baro_t("받은 때")}, list_rows, tool.Baro_t("아직 공개된 감사 기록이 없습니다."))
}

// View_baro_account_profile_edit : 내 프로필 설정 (/baro/profile/edit)
func View_baro_account_profile_edit(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/profile/edit"); need {
		return wall
	}

	user := config.IP
	_ = page
	message := ""

	// --- 쓰기 처리 ---
	if values != nil {
		if tool.Baro_read_only(db) {
			return tool.Baro_page(db, config, tool.Baro_t("프로필 설정"),
				tool.Baro_alert("warn", tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.HTML_escape(tool.Baro_read_only_reason(db))),
				baro_account_menu())
		}

		switch values.Get("action") {
		case "expertise":
			baro_account_expertise_put(db, user, values["main_field"])

			return tool.Get_redirect("/baro/profile/edit")

		default:
			bio := tool.Baro_trunc(tool.Baro_one_line(values.Get("bio")), 200)
			baro_account_uset_put(db, user, "bio", bio)

			docs := []string{}
			for _, name := range []string{"best1", "best2", "best3"} {
				doc := tool.Namespace_normalize(tool.Baro_one_line(values.Get(name)))
				if strings.TrimSpace(doc) == "" {
					continue
				}

				docs = append(docs, doc)
			}
			if len(docs) > 3 {
				docs = docs[:3]
			}
			baro_account_uset_put(db, user, "best_docs", tool.Baro_json_enc(docs))

			rank_public := "on"
			if values.Get("rank_public") != "on" {
				rank_public = "off"
			}
			baro_account_uset_put(db, user, "rank_public", rank_public)

			public := map[string]string{}
			checked := map[string]bool{}
			for _, item := range values["public_item"] {
				checked[item] = true
			}
			for _, row := range baro_account_profile_item_list() {
				if checked[row[0]] {
					public[row[0]] = "on"

					continue
				}

				public[row[0]] = ""
			}
			baro_account_uset_put(db, user, "profile_public", tool.Baro_json_enc(public))

			baro_account_interest_put(db, user, values["interest"])

			tool.Baro_audit(db, config, "profile_edit", user, "", "bio/best/public", "본인 프로필 설정 변경")

			return tool.Get_redirect("/baro/profile/edit")
		}
	}

	bio := baro_account_uset_get(db, user, "bio")
	best := baro_account_profile_best_docs(db, user)
	for len(best) < 3 {
		best = append(best, "")
	}

	rank_public := baro_account_uset_get(db, user, "rank_public") != "off"
	public := baro_account_profile_public(db, user)

	interest := map[string]bool{}
	for _, field := range tool.Baro_user_interest(db, user) {
		interest[field] = true
	}

	form := tool.Baro_textarea("bio", tool.Baro_t("한 줄 소개"), bio, 3, tool.Baro_t("예: 근현대사 자료를 정리합니다."), tool.Baro_t("프로필 맨 위에 보입니다. 200자까지."))

	best_html := ""
	for index, name := range []string{"best1", "best2", "best3"} {
		best_html += tool.Baro_input(name, tool.Baro_itoa(index+1)+tool.Baro_t("번 대표 문서"), best[index], tool.Baro_t("문서 이름"), "")
	}
	form += tool.Baro_field_row_html(tool.Baro_t("대표 문서 (최대 3개)"), best_html, tool.Baro_t("내가 특히 공들인 문서를 골라 두면 다른 사람이 찾아보기 좋습니다."))

	interest_checks := []string{}
	for _, option := range baro_account_field_options(db) {
		interest_checks = append(interest_checks, tool.Baro_checkbox("interest", option[1], option[0], interest[option[0]], ""))
	}
	form += tool.Baro_field_row_html(tool.Baro_t("관심 분야"), tool.Baro_grid(interest_checks, 3), tool.Baro_t("할 일 추천에 쓰입니다."))

	form += tool.Baro_checkbox("rank_public", tool.Baro_t("공개 랭킹에 참여합니다."), "on", rank_public, tool.Baro_t("끄면 기여자 순위 화면에서 빠집니다. 기여 기록 자체는 그대로 남습니다."))

	public_checks := []string{}
	for _, row := range baro_account_profile_item_list() {
		public_checks = append(public_checks, tool.Baro_checkbox("public_item", row[1], row[0], public[row[0]], ""))
	}
	form += tool.Baro_field_row_html(tool.Baro_t("공개 프로필 항목"), tool.Baro_grid(public_checks, 3), tool.Baro_t("체크한 항목만 다른 사람에게 보입니다. 본인에게는 항상 전부 보입니다."))

	form += tool.Baro_submit("저장", "primary")

	body := tool.Baro_hero(tool.Baro_t("프로필 설정"), tool.Baro_t("다른 사람에게 무엇을 보일지 직접 고릅니다."),
		tool.Baro_button("/baro/profile/"+tool.Url_parser(user), "내 프로필 보기", "primary")+" "+
			tool.Baro_button("/baro/account", tool.Baro_t("내 계정"), ""))

	body += baro_account_tabs("/baro/profile/edit")

	if message != "" {
		body += tool.Baro_alert("warn", tool.HTML_escape(message))
	}

	body += tool.Baro_card(tool.Baro_t("프로필"), tool.Baro_form("/baro/profile/edit", form))

	// --- 전문 분야 자동 제안 ---
	body += baro_account_profile_suggest_card(db, user)

	body += tool.Baro_card(tool.Baro_t("이름 · 비밀번호"),
		tool.Baro_note(tool.Baro_t("사용자 이름과 비밀번호, 이메일은 위키 본체 설정에서 바꿉니다."))+
			tool.Baro_button("/change", tool.Baro_t("계정 설정 열기"), ""))

	return tool.Baro_page(db, config, tool.Baro_t("프로필 설정"), body, baro_account_menu())
}

// baro_account_profile_suggest_card : 전문 분야 자동 제안 (2-4)
func baro_account_profile_suggest_card(db *sql.DB, user string) string {
	suggest := baro_account_field_suggest(db, user)
	current := map[string]bool{}
	for _, field := range baro_account_expertise_main(db, user) {
		current[field] = true
	}

	// 제안할 거리가 없어도 대표 분야는 직접 고를 수 있어야 한다 (최대 3개)
	if len(suggest) == 0 {
		checks := []string{}
		for _, option := range baro_account_field_options(db) {
			checks = append(checks, tool.Baro_checkbox("main_field", option[1], option[0], current[option[0]], ""))
		}

		html := tool.Baro_empty(tool.Baro_t("기여 기록이 아직 적어 제안할 분야가 없습니다."))
		html += tool.Baro_note(tool.Baro_t("문서를 몇 개 작성하면 분야별 기록을 모아 자동으로 제안해 드립니다. 그 전에도 직접 고를 수 있습니다."))
		html += tool.Baro_form("/baro/profile/edit",
			tool.Baro_hidden("action", "expertise")+
				tool.Baro_field_row_html(tool.Baro_t("대표 분야 직접 고르기 (최대 3개)"), tool.Baro_grid(checks, 3), tool.Baro_t("4개 이상 고르면 앞의 3개만 저장됩니다."))+
				tool.Baro_submit(tool.Baro_t("대표 분야 저장"), "primary"))

		return tool.Baro_card(tool.Baro_t("전문 분야 제안"), html)
	}

	suggested := map[string]bool{}
	rows := [][]string{}
	for _, row := range suggest {
		name := tool.Baro_field_name(db, row[0])
		suggested[row[0]] = true
		rows = append(rows, []string{
			tool.HTML_escape(name),
			tool.Baro_comma(tool.Str_to_int(row[1])) + tool.Baro_t("건"),
			func() string {
				if current[row[0]] {
					return tool.Baro_badge(tool.Baro_t("대표 분야"), "ok")
				}

				return tool.Baro_badge(tool.Baro_t("제안"), "info")
			}(),
		})
	}

	// 제안 외의 분야도 직접 고를 수 있게 전체 분야를 함께 보여 준다
	checks := []string{}
	for _, option := range baro_account_field_options(db) {
		label := option[1]
		if suggested[option[0]] {
			label += tool.Baro_t(" (제안)")
		}

		checks = append(checks, tool.Baro_checkbox("main_field", label, option[0], current[option[0]], ""))
	}

	html := tool.Baro_alert("info", tool.Baro_t("기여 기록을 보니 <b>이런 분야</b> 같습니다. 최종 선택은 본인이 합니다."))
	html += tool.Baro_table([]string{"분야", tool.Baro_t("내 기여"), "상태"}, rows, "")
	html += tool.Baro_form("/baro/profile/edit",
		tool.Baro_hidden("action", "expertise")+
			tool.Baro_field_row_html(tool.Baro_t("대표 분야로 저장 (최대 3개)"), tool.Baro_grid(checks, 3), tool.Baro_t("선택한 것만 대표 분야로 남습니다. 4개 이상 고르면 앞의 3개만 저장됩니다."))+
			tool.Baro_submit(tool.Baro_t("대표 분야 저장"), "primary"))

	return tool.Baro_card(tool.Baro_t("전문 분야 제안"), html)
}
