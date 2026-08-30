package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  7-3 기여 영향력 — /baro/contrib/impact/<사용자>
//  7-4 전문 분야   — /baro/contrib/expertise
// ============================================================

// baro_contrib_impact_def : 영향력 지표 정의 (저장 이름, 표시 이름, 설명)
func baro_contrib_impact_def() [][]string {
	return [][]string{
		{"new_doc", tool.Baro_t("내가 처음 만든 문서"), tool.Baro_t("이 사람이 처음 만들어 지금까지 남아 있는 문서 수입니다.")},
		{"alive", tool.Baro_t("지금 유지 중인 기여"), tool.Baro_t("되돌려지지 않고 문서가 살아 있는 기여 수입니다.")},
		{"views", tool.Baro_t("누적 비봇 열람"), tool.Baro_t("내가 만든 문서를 사람이 읽은 횟수입니다. 열람 집계가 켜져 있을 때만 쌓입니다.")},
		{"sources", tool.Baro_t("추가한 유효 출처"), tool.Baro_t("내가 손댄 문서에 붙어 있는, 무효 처리되지 않은 출처 수입니다.")},
		{"request_done", tool.Baro_t("해결한 문서 요청"), tool.Baro_t("다른 사람이 요청한 문서를 완성하거나 과제를 끝낸 수입니다.")},
		{"projects", tool.Baro_t("참여 프로젝트"), tool.Baro_t("구성원으로 이름을 올린 프로젝트 수입니다.")},
		{"thanks", tool.Baro_t("감사받은 수정"), tool.Baro_t("다른 사람이 내 특정 기여에 보낸 감사 수입니다. 스스로 보낸 것은 세지 않습니다.")},
		{"reviewed", tool.Baro_t("검토 완료된 문서"), tool.Baro_t("내가 기여한 문서 가운데 검토 완료 상태가 된 문서 수입니다.")},
	}
}

// baro_contrib_impact_name : 지표 표시 이름
func baro_contrib_impact_name(name string) string {
	for _, row := range baro_contrib_impact_def() {
		if row[0] == name {
			return row[1]
		}
	}

	return name
}

// baro_contrib_impact_views : 내가 만든 문서의 누적 열람 수
//
//	openNAMU 본체에는 문서 열람 집계가 없으므로 baro_metric 에 쌓인 값만 읽는다.
//	집계가 꺼져 있으면 0 이 나오며, 화면에서 그 사실을 밝힌다.
func baro_contrib_impact_views(db *sql.DB, user string) int {
	total := 0

	rows := tool.Query_DB(
		db,
		"select distinct title from baro_contrib_event where user = ? and kind = 'new_document' and status <> 'rejected' limit ?, ?",
		user, 0, 500,
	)
	defer rows.Close()

	titles := []string{}
	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		titles = append(titles, title)
	}

	for _, title := range titles {
		if title == "" {
			continue
		}

		total += tool.Baro_metric_get(db, "doc_view:"+title, "all")
	}

	return total
}

// baro_contrib_impact_calc : 영향력 지표를 다시 계산해 baro_impact 에 저장한다
func baro_contrib_impact_calc(db *sql.DB, user string) map[string]int {
	out := map[string]int{}

	// 내가 처음 만들었고 지금도 있는 문서
	out["new_doc"] = baro_contrib_count(
		db,
		"select count(distinct title) from baro_contrib_event e where e.user = ? and e.kind = 'new_document' and e.status <> 'rejected'"+
			" and exists (select 1 from data where data.title = e.title)",
		user,
	)

	// 되돌려지지 않고 문서가 살아 있는 기여
	out["alive"] = baro_contrib_count(
		db,
		"select count(*) from baro_contrib_event e where e.user = ? and e.status <> 'rejected'"+
			" and exists (select 1 from data where data.title = e.title)",
		user,
	)

	out["views"] = baro_contrib_impact_views(db, user)

	// 내가 손댄 문서에 붙어 있는 유효 출처
	out["sources"] = baro_contrib_count(
		db,
		"select count(*) from baro_citation c where c.status <> 'rejected'"+
			" and exists (select 1 from baro_contrib_event e where e.user = ? and e.title = c.title)",
		user,
	)

	// 해결한 문서 요청 (기여 이벤트 + 프로젝트 과제)
	out["request_done"] = baro_contrib_count(
		db,
		"select count(*) from baro_contrib_event where user = ? and kind = 'request_done' and status <> 'rejected'",
		user,
	) + baro_contrib_count(
		db,
		"select count(*) from baro_project_task where assignee = ? and status = 'done'",
		user,
	)

	// 참여 프로젝트
	out["projects"] = baro_contrib_count(
		db,
		"select count(distinct project) from baro_project_member where user = ? and status <> 'rejected'",
		user,
	)

	// 감사받은 수정 (셀프 추천 제외)
	out["thanks"] = baro_contrib_count(
		db,
		"select count(*) from baro_thanks where to_user = ? and from_user <> to_user",
		user,
	)

	// 검토 완료된 문서
	out["reviewed"] = baro_contrib_count(
		db,
		"select count(distinct e.title) from baro_contrib_event e where e.user = ?"+
			" and exists (select 1 from baro_doc_status s where s.title = e.title and (s.status = 'reviewed' or s.status = 'featured'))",
		user,
	)

	for name, value := range out {
		tool.Baro_impact_set(db, user, name, value)
	}

	return out
}

// baro_contrib_impact_load : baro_impact 에 저장해 둔 값만 읽는다 (다시 계산하지 않는다)
func baro_contrib_impact_load(db *sql.DB, user string) map[string]int {
	out := map[string]int{}

	for _, row := range baro_contrib_impact_def() {
		out[row[0]] = tool.Baro_impact_get(db, user, row[0])
	}

	return out
}

// View_baro_contrib_impact : 기여 영향력 화면
func View_baro_contrib_impact(config tool.Config, user_raw string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	user := baro_contrib_user_param(user_raw)

	if user == "" {
		body := tool.Baro_crumb([][]string{{"/baro/contrib", "기여 기록"}, {"", "기여 영향력"}})
		body += tool.Baro_alert("warn", tool.Baro_t("사용자 이름이 비어 있습니다. 주소 뒤에 사용자 이름을 붙여 주세요."))
		body += tool.Baro_card(tool.Baro_t("보고 싶은 사람 고르기"),
			`<form class="bw_form" method="get" action="/baro/contrib">`+
				tool.Baro_input("user", tool.Baro_t("사용자 이름"), "", tool.Baro_t("사용자 이름"), "")+
				tool.Baro_submit(tool.Baro_t("기여 기록에서 찾기"), "primary")+
				`</form>`,
		)

		return tool.Baro_page(db, config, "기여 영향력", body, baro_contrib_menu())
	}

	// 화면에 들어올 때마다 다시 계산해 갱신한다.
	// 다만 읽기 전용 상태에서는 쓰기를 하지 않고 저장된 값을 그대로 보여 준다.
	read_only := tool.Baro_read_only(db)

	values := map[string]int{}
	if read_only {
		values = baro_contrib_impact_load(db, user)
	} else {
		values = baro_contrib_impact_calc(db, user)
	}

	updated := ""
	tool.QueryRow_DB(db, "select date from baro_impact where user = ? order by date desc limit 1", []any{&updated}, user)

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/contrib", "기여 기록"},
		{"/baro/contrib/user/" + tool.Url_parser(user), tool.Namespace_display(user)},
		{"", "기여 영향력"},
	})

	body += tool.Baro_hero(
		user+tool.Baro_t(" 님의 기여 영향력"),
		tool.Baro_t("몇 번 편집했는지가 아니라, 그 기여가 무엇을 바꿨는지를 봅니다."),
		"",
	)

	body += tool.Baro_tabs([][]string{
		{"/baro/contrib/user/" + tool.Url_parser(user), "기여 기록"},
		{"/baro/contrib/impact/" + tool.Url_parser(user), "기여 영향력"},
		{"/baro/contrib/calendar/" + tool.Url_parser(user), "활동 캘린더"},
		{"/baro/badge/user/" + tool.Url_parser(user), "배지"},
	}, "/baro/contrib/impact/"+tool.Url_parser(user))

	if !tool.Baro_is_user(user) {
		body += tool.Baro_alert("info", tool.Baro_t("로그인하지 않은 편집(IP 편집)은 사람 단위 영향력으로 집계하지 않습니다."))
	}

	if read_only {
		body += tool.Baro_alert("warn", tool.Baro_t("지금은 읽기 전용 상태라 지표를 다시 계산하지 않고 마지막으로 저장된 값을 보여 줍니다. ")+
			baro_contrib_esc(tool.Baro_read_only_reason(db)))
	}

	// --- 큰 지표 ---
	stats := [][]string{}
	for _, row := range baro_contrib_impact_def() {
		stats = append(stats, []string{baro_contrib_impact_name(row[0]), tool.Baro_comma(values[row[0]]), ""})
	}
	body += tool.Baro_card_ex(
		tool.Baro_t("영향력 지표"),
		tool.Baro_t("화면을 열 때마다 다시 계산해 baro_impact 에 저장합니다."),
		tool.Baro_stat_row(stats),
		tool.Baro_note(tool.Baro_t("마지막 갱신 : ")+updated),
	)

	// --- 설명 표 ---
	detail_rows := [][]string{}
	for _, row := range baro_contrib_impact_def() {
		detail_rows = append(detail_rows, []string{
			`<b>` + baro_contrib_esc(row[1]) + `</b>`,
			tool.Baro_comma(values[row[0]]),
			baro_contrib_esc(row[2]),
		})
	}
	body += tool.Baro_card(tool.Baro_t("지표가 뜻하는 것"), tool.Baro_table([]string{tool.Baro_t("지표"), tool.Baro_t("값"), tool.Baro_t("설명")}, detail_rows, tool.Baro_t("지표가 없습니다.")))

	if values["views"] == 0 {
		body += tool.Baro_note(tool.Baro_t("누적 열람 수가 0 인 것은 아직 열람 집계가 쌓이지 않았기 때문일 수 있습니다. 없는 숫자를 지어내지 않습니다."))
	}

	// --- 대표 문서 ---
	rows := tool.Query_DB(
		db,
		"select "+baro_contrib_event_cols+" from baro_contrib_event where user = ? and kind = 'new_document' order by id + 0 desc limit ?, ?",
		user, 0, 10,
	)
	defer rows.Close()

	doc_list := baro_contrib_event_scan(rows)

	doc_rows := [][]string{}
	for _, row := range doc_list {
		alive := tool.Baro_badge(tool.Baro_t("유지 중"), "ok")
		if !baro_contrib_doc_exists(db, row.Title) {
			alive = tool.Baro_badge(tool.Baro_t("지금은 없음"), "muted")
		}

		doc_rows = append(doc_rows, []string{
			tool.Baro_doc_link(row.Title),
			baro_contrib_esc(tool.Baro_field_name(db, row.Field)),
			tool.Baro_time_cell(row.Date),
			alive,
			tool.Baro_status_badge(tool.Baro_doc_status_get(db, row.Title)),
		})
	}

	doc_html := tool.Baro_table([]string{"문서", "분야", tool.Baro_t("만든 때"), tool.Baro_t("생존"), tool.Baro_t("문서 상태")}, doc_rows, tool.Baro_t("처음 만든 문서 기록이 없습니다."))
	if len(doc_rows) == 0 {
		doc_html = tool.Baro_empty(tool.Baro_t("이 사람이 처음 만든 문서로 기록된 것이 아직 없습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("처음 만든 문서 (최근 10개)"), tool.Baro_t("빈 자리를 처음 채운 기여입니다."), doc_html,
		tool.Baro_button(baro_contrib_query("/baro/contrib", [][]string{{"user", user}, {"kind", "new_document"}}), tool.Baro_t("전체 보기"), ""))

	return tool.Baro_page(db, config, user+tool.Baro_t(" 님의 기여 영향력"), body, baro_contrib_menu())
}

// ------------------------------------------------------------
//  7-4 전문 분야 — /baro/contrib/expertise
// ------------------------------------------------------------

// baro_contrib_expertise_max_main : 대표 분야 최대 개수
const baro_contrib_expertise_max_main = 3

// baro_contrib_expertise_row : 저장된 전문 분야
type baro_contrib_expertise_row struct {
	Field  string
	Score  string
	IsMain string
	Date   string
}

// baro_contrib_expertise_load : 저장된 전문 분야 읽기
func baro_contrib_expertise_load(db *sql.DB, user string) []baro_contrib_expertise_row {
	out := []baro_contrib_expertise_row{}

	rows := tool.Query_DB(db, "select field, score, is_main, date from baro_expertise where user = ?", user)
	defer rows.Close()

	for rows.Next() {
		row := baro_contrib_expertise_row{}
		if err := rows.Scan(&row.Field, &row.Score, &row.IsMain, &row.Date); err != nil {
			continue
		}

		out = append(out, row)
	}

	sort.SliceStable(out, func(i int, j int) bool {
		return tool.Str_to_int(out[i].Score) > tool.Str_to_int(out[j].Score)
	})

	return out
}

// baro_contrib_expertise_suggest : 활동을 바탕으로 분야별 점수를 계산한다
func baro_contrib_expertise_suggest(db *sql.DB, user string) [][]string {
	scores := map[string]int{}
	counts := map[string]int{}

	rows := tool.Query_DB(
		db,
		"select "+baro_contrib_field_expr("baro_contrib_event")+" as f, count(*), coalesce(sum(weight + 0), 0)"+
			" from baro_contrib_event where user = ? and status <> 'rejected' group by f",
		user,
	)
	defer rows.Close()

	for rows.Next() {
		field, count, weight := "", 0, 0
		if err := rows.Scan(&field, &count, &weight); err != nil {
			continue
		}

		if field == "" {
			field = "etc"
		}

		counts[field] += count
		scores[field] += weight
	}

	// 관심 분야로 등록해 둔 곳은 약간 더 쳐 준다 (다만 활동이 없으면 제안하지 않는다)
	for _, field := range tool.Baro_user_interest(db, user) {
		if counts[field] > 0 {
			scores[field] += 5
		}
	}

	out := [][]string{}
	for field, score := range scores {
		out = append(out, []string{field, tool.Baro_itoa(score), tool.Baro_itoa(counts[field])})
	}

	sort.SliceStable(out, func(i int, j int) bool {
		return tool.Str_to_int(out[i][1]) > tool.Str_to_int(out[j][1])
	})

	return out
}

// baro_contrib_expertise_save : 계산 결과와 사용자가 고른 대표 분야를 저장한다
func baro_contrib_expertise_save(db *sql.DB, user string, suggest [][]string, mains []string) {
	main_set := map[string]bool{}
	for _, field := range mains {
		main_set[field] = true
	}

	tool.Exec_DB(db, "delete from baro_expertise where user = ?", user)

	now := tool.Baro_now()

	for _, row := range suggest {
		is_main := ""
		if main_set[row[0]] {
			is_main = "on"
		}

		tool.Exec_DB(
			db,
			"insert into baro_expertise (user, field, score, is_main, date) values (?, ?, ?, ?, ?)",
			user, row[0], row[1], is_main, now,
		)
	}

	// 활동은 없지만 사용자가 직접 고른 분야도 남긴다
	for field := range main_set {
		found := false
		for _, row := range suggest {
			if row[0] == field {
				found = true
				break
			}
		}

		if found {
			continue
		}

		tool.Exec_DB(
			db,
			"insert into baro_expertise (user, field, score, is_main, date) values (?, ?, '0', 'on', ?)",
			user, field, now,
		)
	}
}

// View_baro_contrib_expertise : 전문 분야 계산 · 제안 · 저장
func View_baro_contrib_expertise(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/contrib/expertise"); need {
		return wall
	}

	user := config.IP
	suggest := baro_contrib_expertise_suggest(db, user)

	// --- 저장 ---
	if values != nil {
		if tool.Baro_read_only(db) {
			body := tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 저장할 수 없습니다. ")+baro_contrib_esc(tool.Baro_read_only_reason(db)))

			return tool.Baro_page(db, config, tool.Baro_t("전문 분야"), body, baro_contrib_menu())
		}

		if !tool.Baro_rate_ok(db, "contrib_expertise", user, 20, 3600) {
			body := tool.Baro_alert("warn", tool.Baro_t("너무 자주 저장했습니다. 잠시 뒤에 다시 시도해 주세요."))

			return tool.Baro_page(db, config, tool.Baro_t("전문 분야"), body, baro_contrib_menu())
		}

		valid := map[string]bool{}
		for _, row := range tool.Baro_field_list(db) {
			valid[row.Key] = true
		}

		mains := []string{}
		for _, field := range values["main"] {
			field = strings.TrimSpace(field)
			if field == "" || !valid[field] {
				continue
			}
			if len(mains) >= baro_contrib_expertise_max_main {
				break
			}

			mains = append(mains, field)
		}

		baro_contrib_expertise_save(db, user, suggest, mains)

		tool.Baro_audit(db, config, "expertise_save", user, "", strings.Join(mains, ","), "대표 분야 선택")

		return tool.Get_redirect("/baro/contrib/expertise?saved=1")
	}

	saved := baro_contrib_expertise_load(db, user)

	main_now := map[string]bool{}
	for _, row := range saved {
		if row.IsMain == "on" {
			main_now[row.Field] = true
		}
	}

	// 저장된 것이 없으면 상위 분야를 미리 골라 둔다 (자동 제안)
	if len(main_now) == 0 {
		for index, row := range suggest {
			if index >= baro_contrib_expertise_max_main {
				break
			}

			main_now[row[0]] = true
		}
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/contrib", "기여 기록"}, {"", tool.Baro_t("전문 분야")}})
	body += tool.Baro_hero(
		tool.Baro_t("전문 분야"),
		tool.Baro_t("활동을 바탕으로 자동 제안하고, 마지막 선택은 본인이 합니다."),
		"",
	)
	body += baro_contrib_tabs("/baro/contrib/expertise")

	if baro_contrib_get(query, "saved") != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("전문 분야를 저장했습니다."))
	}

	body += tool.Baro_alert("info",
		tool.Baro_t("전문 분야는 <b>자랑거리가 아니라 안내판</b>입니다. 어떤 분야를 물어보면 좋은지 다른 기여자가 알 수 있게 하는 것이 목적입니다. ")+
			tool.Baro_t("대표 분야는 최대 ")+tool.Baro_itoa(baro_contrib_expertise_max_main)+tool.Baro_t("개까지 고를 수 있습니다."),
	)

	// --- 자동 제안 ---
	suggest_rows := [][]string{}
	for index, row := range suggest {
		mark := ""
		if index < baro_contrib_expertise_max_main {
			mark = tool.Baro_badge(tool.Baro_t("추천"), "ok")
		}

		suggest_rows = append(suggest_rows, []string{
			baro_contrib_esc(tool.Baro_field_name(db, row[0])),
			tool.Baro_comma(tool.Str_to_int(row[2])) + tool.Baro_t("건"),
			tool.Baro_comma(tool.Str_to_int(row[1])),
			mark,
		})
	}

	suggest_html := tool.Baro_table([]string{"분야", tool.Baro_t("기여 건수"), tool.Baro_t("활동 점수"), ""}, suggest_rows, tool.Baro_t("아직 분야별 활동이 없습니다."))
	if len(suggest_rows) == 0 {
		suggest_html = tool.Baro_empty(tool.Baro_t("아직 분야가 정해진 기여가 없습니다. 문서에 분야가 지정되면 여기에 쌓입니다."))
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("자동 제안"),
		tool.Baro_t("기여한 문서의 분야와 행동 가중치를 더해 계산합니다."),
		suggest_html,
		tool.Baro_note(tool.Baro_t("활동 점수는 제안 순서를 정하기 위한 값일 뿐, 기여 가치 점수와 다릅니다.")),
	)

	// --- 최종 선택 ---
	form_body := ""
	for _, field := range tool.Baro_field_list(db) {
		count := 0
		for _, row := range suggest {
			if row[0] == field.Key {
				count = tool.Str_to_int(row[2])
				break
			}
		}

		help := tool.Baro_t("기여 ") + tool.Baro_comma(count) + tool.Baro_t("건")
		if field.Description != "" {
			help += " · " + field.Description
		}

		form_body += tool.Baro_checkbox("main", field.Name, field.Key, main_now[field.Key], help)
	}

	form_body += tool.Baro_note(tool.Baro_t("4개 이상 고르면 앞의 ") + tool.Baro_itoa(baro_contrib_expertise_max_main) + tool.Baro_t("개만 저장됩니다."))
	form_body += tool.Baro_submit(tool.Baro_t("대표 분야 저장"), "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("대표 분야 고르기"),
		tool.Baro_t("자동 제안을 참고하되, 최종 결정은 본인이 합니다."),
		tool.Baro_form("/baro/contrib/expertise", form_body),
		tool.Baro_note(tool.Baro_t("저장하면 자동 제안 점수도 함께 갱신됩니다. 프로필 화면 표시는 계정 모듈이 담당합니다.")),
	)

	// --- 저장된 값 ---
	saved_rows := [][]string{}
	for _, row := range saved {
		mark := tool.Baro_badge(tool.Baro_t("보조"), "muted")
		if row.IsMain == "on" {
			mark = tool.Baro_badge(tool.Baro_t("대표"), "ok")
		}

		saved_rows = append(saved_rows, []string{
			baro_contrib_esc(tool.Baro_field_name(db, row.Field)),
			tool.Baro_comma(tool.Str_to_int(row.Score)),
			mark,
			tool.Baro_time_cell(row.Date),
		})
	}

	saved_html := tool.Baro_table([]string{"분야", tool.Baro_t("활동 점수"), tool.Baro_t("구분"), tool.Baro_t("저장 시각")}, saved_rows, tool.Baro_t("저장된 전문 분야가 없습니다."))
	if len(saved_rows) == 0 {
		saved_html = tool.Baro_empty(tool.Baro_t("아직 저장된 전문 분야가 없습니다. 위에서 골라 저장해 보세요."))
	}

	body += tool.Baro_card(tool.Baro_t("지금 저장된 전문 분야"), saved_html)

	return tool.Baro_page(db, config, tool.Baro_t("전문 분야"), body, baro_contrib_menu())
}
