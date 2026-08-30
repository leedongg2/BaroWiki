package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  11-1 공개 운영 로그  (/baro/log)
//
//  누가 무엇을 왜 했는지는 공개하고, 개인정보는 공개하지 않는다.
//  (0장 5번 원칙 — 근거 공개 · 개인정보 보호)
// ============================================================

// baro_gov_log_doc_kind : 대상이 문서인 조치 종류
var baro_gov_log_doc_kind = map[string]bool{
	"protect": true, "unprotect": true, "quarantine": true, "release": true,
	"delete": true, "restore": true, "stable": true, "merge": true, "rename": true,
}

// baro_gov_log_user_kind : 대상이 사용자인 조치 종류
var baro_gov_log_user_kind = map[string]bool{
	"block": true, "unblock": true, "sanction": true,
	"role_grant": true, "role_revoke": true, "warn": true,
}

// baro_gov_log_source_tabs : 공개 로그 · 위키 관리 기록 사이를 오가는 탭
//
//	11-1 은 openNAMU 본체의 관리 기록(re_admin)도 함께 볼 수 있어야 하므로
//	두 화면 모두에 같은 탭을 붙인다.
func baro_gov_log_source_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/log", tool.Baro_t("바로위키 공개 로그")},
		{"/baro/log/legacy", tool.Baro_t("위키 관리 기록 (re_admin)")},
	}, active)
}

// baro_gov_log_notice : 공개 로그 화면 위에 늘 붙는 안내
func baro_gov_log_notice() string {
	return tool.Baro_alert("info",
		tool.Baro_t(`<b>공개 운영 로그</b>는 차단 · 보호 · 문서 격리 · 삭제 · 복원 · 관리자 권한 부여와 회수 · 정책 변경 · 대량 작업을 누구나 볼 수 있게 남긴 기록입니다.`)+
			tool.Baro_t(` 조치 사유와 적용 규정은 공개하지만 IP · 이메일 · 신고 원문 같은 개인정보는 공개하지 않습니다.`),
	)
}

// baro_gov_log_target_html : 로그 대상 표시 (조치 종류로 문서인지 사용자인지 가린다)
func baro_gov_log_target_html(db *sql.DB, kind string, target string, my_ip string) string {
	if strings.TrimSpace(target) == "" {
		return `<span class="bw_muted">-</span>`
	}

	if baro_gov_log_doc_kind[kind] {
		return tool.Baro_doc_link(target)
	}

	if baro_gov_log_user_kind[kind] && tool.Baro_is_user(target) {
		return tool.Baro_user_link(db, target, my_ip)
	}

	if kind == "policy" || kind == "policy_new" {
		return `<a href="/baro/policy/view/` + tool.Url_parser(target) + `">` + baro_gov_safe(target) + `</a>`
	}

	return baro_gov_safe(tool.Baro_trunc(target, 60))
}

// View_baro_gov_log_list : 공개 운영 로그 목록
func View_baro_gov_log_list(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	kind := baro_gov_q(query, "kind")
	from := baro_gov_q(query, "from")
	to := baro_gov_q(query, "to")
	word := baro_gov_q(query, "q")
	page := baro_gov_page_num(baro_gov_q(query, "page"))

	where := " where 1 = 1"
	args := []any{}

	if kind != "" {
		where += " and kind = ?"
		args = append(args, kind)
	}

	if len(from) == 10 {
		where += " and date >= ?"
		args = append(args, from+" 00:00:00")
	}

	if len(to) == 10 {
		where += " and date <= ?"
		args = append(args, to+" 23:59:59")
	}

	if word != "" {
		where += " and (target like ? or reason like ? or detail like ? or actor like ?)"
		like := "%" + word + "%"
		args = append(args, like, like, like, like)
	}

	total := 0
	tool.QueryRow_DB(db, "select count(*) from baro_public_log"+where, []any{&total}, args...)

	size := tool.Baro_page_size
	list_args := append(append([]any{}, args...), (page-1)*size, size)

	rows := tool.Query_DB(
		db,
		"select id, kind, actor, target, reason, detail, date from baro_public_log"+where+" order by id + 0 desc limit ?, ?",
		list_args...,
	)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0

	for rows.Next() {
		id, log_kind, actor, target, reason, detail, date := "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &log_kind, &actor, &target, &reason, &detail, &date); err != nil {
			continue
		}

		count++

		reason_html := tool.Baro_t(`<span class="bw_muted">사유 없음</span>`)
		if strings.TrimSpace(reason) != "" {
			reason_html = baro_gov_safe(tool.Baro_trunc(tool.Baro_one_line(reason), 70))
		}

		if strings.TrimSpace(detail) != "" {
			reason_html += `<div class="bw_note">` + baro_gov_safe(tool.Baro_trunc(tool.Baro_one_line(detail), 60)) + `</div>`
		}

		table_rows = append(table_rows, []string{
			tool.Baro_time_cell(date),
			baro_gov_log_kind_badge(log_kind),
			baro_gov_log_target_html(db, log_kind, target, config.IP),
			baro_gov_actor_html(db, actor, config.IP),
			reason_html,
			tool.Baro_button("/baro/log/view/"+tool.Url_parser(id), "자세히", ""),
		})
	}

	body := baro_gov_css()
	body += tool.Baro_hero(tool.Baro_t("공개 운영 로그"), tool.Baro_t("바로위키에서 일어난 운영 조치를 누구나 확인할 수 있습니다."), "")
	body += baro_gov_tabs("/baro/log")
	body += baro_gov_log_source_tabs("/baro/log")
	body += baro_gov_log_notice()

	today := tool.Baro_today()
	month := today[:7]

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 기록"), tool.Baro_comma(baro_gov_count(db, "select count(*) from baro_public_log")), tool.Baro_t("공개된 모든 조치")},
		{"이번 달", tool.Baro_comma(baro_gov_count(db, "select count(*) from baro_public_log where date >= ?", month+"-01 00:00:00")), baro_gov_period_name(month)},
		{tool.Baro_t("오늘"), tool.Baro_comma(baro_gov_count(db, "select count(*) from baro_public_log where date >= ?", today+" 00:00:00")), today},
		{tool.Baro_t("거른 결과"), tool.Baro_comma(total), tool.Baro_t("현재 조건")},
	})

	filter := `<div class="bw_gov_filter">`
	filter += tool.Baro_select("kind", "종류", baro_gov_log_kind_options(db), kind, "")
	filter += tool.Baro_input_type("from", tool.Baro_t("시작일"), from, "date", "")
	filter += tool.Baro_input_type("to", tool.Baro_t("종료일"), to, "date", "")
	filter += tool.Baro_input("q", tool.Baro_t("대상 검색"), word, tool.Baro_t("문서 이름 · 사용자 이름 · 사유"), "")
	filter += `<div class="bw_gov_filter_btn">` + tool.Baro_submit(tool.Baro_t("찾기"), "primary") + ` ` + tool.Baro_button("/baro/log", tool.Baro_t("조건 지우기"), "") + `</div>`
	filter += `</div>`

	body += tool.Baro_card(tool.Baro_t("거르기"), baro_gov_get_form("/baro/log", filter))

	list_body := ""
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("조건에 맞는 운영 기록이 없습니다. 종류 · 기간 조건을 지우고 다시 찾아 보세요."))
	} else {
		list_body = tool.Baro_table(
			[]string{"시각", "종류", tool.Baro_t("대상"), tool.Baro_t("담당"), "사유", ""},
			table_rows,
			tool.Baro_t("조건에 맞는 운영 기록이 없습니다."),
		)
	}

	list_body += tool.Baro_paging(page, count, size, baro_gov_url("/baro/log", [][]string{
		{"kind", kind}, {"from", from}, {"to", to}, {"q", word}, {"page", "{}"},
	}))

	body += tool.Baro_card_ex(tool.Baro_t("운영 기록"), tool.Baro_t("최근 조치가 위에 옵니다. 조건에 맞는 기록 ")+tool.Baro_comma(total)+tool.Baro_t("건."), list_body, "")

	body += tool.Baro_card(tool.Baro_t("종류별 건수"),
		baro_gov_log_kind_summary(db, from, to),
	)

	return tool.Baro_page(db, config, tool.Baro_t("공개 운영 로그"), body, baro_gov_menu())
}

// baro_gov_log_kind_summary : 종류별 건수 표 (기간 조건을 함께 적용한다)
func baro_gov_log_kind_summary(db *sql.DB, from string, to string) string {
	where := " where 1 = 1"
	args := []any{}

	if len(from) == 10 {
		where += " and date >= ?"
		args = append(args, from+" 00:00:00")
	}

	if len(to) == 10 {
		where += " and date <= ?"
		args = append(args, to+" 23:59:59")
	}

	rows := tool.Query_DB(db, "select kind, count(*) from baro_public_log"+where+" group by kind", args...)
	defer rows.Close()

	pairs := [][]string{}
	for rows.Next() {
		kind := ""
		num := 0
		if err := rows.Scan(&kind, &num); err != nil {
			continue
		}

		pairs = append(pairs, []string{kind, tool.Baro_itoa(num)})
	}

	if len(pairs) == 0 {
		return tool.Baro_empty(tool.Baro_t("아직 집계할 운영 기록이 없습니다."))
	}

	sort.Slice(pairs, func(i int, j int) bool {
		return tool.Str_to_int(pairs[i][1]) > tool.Str_to_int(pairs[j][1])
	})

	table_rows := [][]string{}
	chart := [][]string{}

	for _, pair := range pairs {
		table_rows = append(table_rows, []string{
			baro_gov_log_kind_badge(pair[0]),
			tool.Baro_comma(tool.Str_to_int(pair[1])),
			tool.Baro_button(baro_gov_url("/baro/log", [][]string{{"kind", pair[0]}, {"from", from}, {"to", to}}), tool.Baro_t("이 종류만 보기"), ""),
		})

		chart = append(chart, []string{baro_gov_log_kind_name(pair[0]), pair[1]})
	}

	html := tool.Baro_table([]string{"종류", tool.Baro_t("건수"), ""}, table_rows, tool.Baro_t("집계할 기록이 없습니다."))
	html += baro_gov_bar_svg(chart, tool.Baro_t("종류별 운영 기록 건수"))

	return html
}

// View_baro_gov_log_view : 공개 운영 로그 한 건 상세
func View_baro_gov_log_view(config tool.Config, id string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	id = strings.TrimSpace(id)

	kind, actor, target, reason, detail, date := "", "", "", "", "", ""
	found := false

	if id != "" {
		found = tool.QueryRow_DB(
			db,
			"select kind, actor, target, reason, detail, date from baro_public_log where id = ?",
			[]any{&kind, &actor, &target, &reason, &detail, &date},
			id,
		)
	}

	if !found {
		body := baro_gov_css()
		body += tool.Baro_crumb([][]string{{"/baro/log", tool.Baro_t("공개 운영 로그")}, {"", tool.Baro_t("기록 없음")}})
		body += tool.Baro_alert("warn", tool.Baro_t("요청하신 번호의 운영 기록을 찾지 못했습니다. 기록이 아직 없거나 번호가 잘못됐을 수 있습니다."))
		body += tool.Baro_card(tool.Baro_t("무엇을 할 수 있나요"),
			tool.Baro_empty("번호 "+tool.Baro_trunc(id, 30)+tool.Baro_t(" 에 해당하는 기록이 없습니다."))+
				tool.Baro_button("/baro/log", tool.Baro_t("공개 운영 로그로 돌아가기"), "primary"),
		)

		return tool.Baro_page(db, config, tool.Baro_t("운영 기록"), body, baro_gov_menu())
	}

	body := baro_gov_css()
	body += tool.Baro_crumb([][]string{{"/baro/log", tool.Baro_t("공개 운영 로그")}, {"", tool.Baro_t("기록 ") + tool.Baro_trunc(id, 30)}})
	body += tool.Baro_hero(baro_gov_log_kind_name(kind), tool.Baro_t("기록 번호 ")+id, "")

	reason_html := tool.Baro_t(`<span class="bw_muted">사유가 기록되지 않았습니다.</span>`)
	if strings.TrimSpace(reason) != "" {
		reason_html = baro_gov_pre(reason)
	}

	pairs := [][]string{
		{"종류", baro_gov_log_kind_badge(kind)},
		{"시각", tool.HTML_escape(date) + " (" + tool.HTML_escape(tool.Baro_time_ago(date)) + ")"},
		{tool.Baro_t("대상"), baro_gov_log_target_html(db, kind, target, config.IP)},
		{tool.Baro_t("담당"), baro_gov_actor_html(db, actor, config.IP)},
	}

	body += tool.Baro_card(tool.Baro_t("조치 개요"), tool.Baro_kv_grid(pairs))
	body += tool.Baro_card("사유", reason_html)

	detail_html := tool.Baro_t(`<span class="bw_muted">추가로 공개된 내용이 없습니다.</span>`)
	if strings.TrimSpace(detail) != "" {
		decoded := tool.Baro_json_dec_str(detail)
		if len(decoded) > 0 {
			detail_pairs := [][]string{}
			keys := []string{}
			for key := range decoded {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, key := range keys {
				detail_pairs = append(detail_pairs, []string{key, baro_gov_safe(decoded[key])})
			}

			detail_html = tool.Baro_dl(detail_pairs)
		} else {
			detail_html = baro_gov_pre(detail)
		}
	}

	body += tool.Baro_card("상세", detail_html)

	// 같은 대상에 대한 다른 기록
	if strings.TrimSpace(target) != "" {
		rows := tool.Query_DB(
			db,
			"select id, kind, actor, reason, date from baro_public_log where target = ? and id != ? order by id + 0 desc limit 20",
			target, id,
		)
		defer rows.Close()

		near_rows := [][]string{}
		for rows.Next() {
			near_id, near_kind, near_actor, near_reason, near_date := "", "", "", "", ""
			if err := rows.Scan(&near_id, &near_kind, &near_actor, &near_reason, &near_date); err != nil {
				continue
			}

			near_rows = append(near_rows, []string{
				tool.Baro_time_cell(near_date),
				baro_gov_log_kind_badge(near_kind),
				baro_gov_actor_html(db, near_actor, config.IP),
				baro_gov_safe(tool.Baro_trunc(tool.Baro_one_line(near_reason), 50)),
				tool.Baro_button("/baro/log/view/"+tool.Url_parser(near_id), "보기", ""),
			})
		}

		body += tool.Baro_card_ex(
			tool.Baro_t("같은 대상의 다른 기록"),
			tool.Baro_t("대상 : ")+tool.Baro_trunc(target, 60),
			tool.Baro_table([]string{"시각", "종류", tool.Baro_t("담당"), "사유", ""}, near_rows, tool.Baro_t("같은 대상에 대한 다른 공개 기록이 없습니다.")),
			tool.Baro_button(baro_gov_url("/baro/log", [][]string{{"q", target}}), tool.Baro_t("이 대상으로 검색"), ""),
		)
	}

	body += tool.Baro_note(tool.Baro_t("이 화면은 공개용 기록입니다. IP · 이메일 · 신고 원문 같은 내부 상세는 감사 기록(baro_audit_log)에만 남으며 공개되지 않습니다."))
	body += tool.Baro_button("/baro/log", tool.Baro_t("목록으로"), "")

	return tool.Baro_page(db, config, tool.Baro_t("운영 기록 ")+id, body, baro_gov_menu())
}

// View_baro_gov_log_legacy : openNAMU 본체 관리 기록 (re_admin)
func View_baro_gov_log_legacy(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	word := baro_gov_q(query, "q")
	from := baro_gov_q(query, "from")
	to := baro_gov_q(query, "to")
	page := baro_gov_page_num(baro_gov_q(query, "page"))

	where := " where 1 = 1"
	args := []any{}

	if word != "" {
		where += " and (what like ? or who like ?)"
		like := "%" + word + "%"
		args = append(args, like, like)
	}

	if len(from) == 10 {
		where += " and time >= ?"
		args = append(args, from+" 00:00:00")
	}

	if len(to) == 10 {
		where += " and time <= ?"
		args = append(args, to+" 23:59:59")
	}

	total := 0
	tool.QueryRow_DB(db, "select count(*) from re_admin"+where, []any{&total}, args...)

	size := tool.Baro_page_size
	list_args := append(append([]any{}, args...), (page-1)*size, size)

	rows := tool.Query_DB(db, "select who, what, time from re_admin"+where+" order by time desc limit ?, ?", list_args...)
	defer rows.Close()

	table_rows := [][]string{}
	count := 0

	for rows.Next() {
		who, what, when := "", "", ""
		if err := rows.Scan(&who, &what, &when); err != nil {
			continue
		}

		count++

		table_rows = append(table_rows, []string{
			tool.Baro_time_cell(when),
			baro_gov_actor_html(db, who, config.IP),
			baro_gov_safe(tool.Baro_trunc(what, 200)),
		})
	}

	body := baro_gov_css()
	body += tool.Baro_hero(tool.Baro_t("위키 관리 기록"), tool.Baro_t("바로위키 확장 이전부터 쓰던 openNAMU 본체의 관리 기록입니다."), "")
	body += baro_gov_tabs("/baro/log")

	body += baro_gov_log_source_tabs("/baro/log/legacy")

	body += tool.Baro_alert("warn",
		tool.Baro_t(`이 표는 위키 엔진이 자체적으로 남기는 관리 기록입니다. 바로위키의 공개 로그와 달리 <b>형식이 정해져 있지 않고</b> 조치 사유가 없을 수 있습니다.`)+
			tool.Baro_t(` 담당자 표시는 개인정보 보호를 위해 IP 일부를 가려 보여 줍니다.`),
	)

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 기록"), tool.Baro_comma(baro_gov_count(db, "select count(*) from re_admin")), tool.Baro_t("위키 관리 기록")},
		{tool.Baro_t("거른 결과"), tool.Baro_comma(total), tool.Baro_t("현재 조건")},
	})

	filter := `<div class="bw_gov_filter">`
	filter += tool.Baro_input("q", tool.Baro_t("내용 검색"), word, tool.Baro_t("예) block, acl, delete"), "")
	filter += tool.Baro_input_type("from", tool.Baro_t("시작일"), from, "date", "")
	filter += tool.Baro_input_type("to", tool.Baro_t("종료일"), to, "date", "")
	filter += `<div class="bw_gov_filter_btn">` + tool.Baro_submit(tool.Baro_t("찾기"), "primary") + ` ` + tool.Baro_button("/baro/log/legacy", tool.Baro_t("조건 지우기"), "") + `</div>`
	filter += `</div>`

	body += tool.Baro_card(tool.Baro_t("거르기"), baro_gov_get_form("/baro/log/legacy", filter))

	list_body := ""
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("조건에 맞는 위키 관리 기록이 없습니다."))
	} else {
		list_body = tool.Baro_table([]string{"시각", tool.Baro_t("담당"), tool.Baro_t("내용")}, table_rows, tool.Baro_t("조건에 맞는 기록이 없습니다."))
	}

	list_body += tool.Baro_paging(page, count, size, baro_gov_url("/baro/log/legacy", [][]string{
		{"q", word}, {"from", from}, {"to", to}, {"page", "{}"},
	}))

	body += tool.Baro_card_ex(tool.Baro_t("위키 관리 기록"), tool.Baro_t("최근 기록이 위에 옵니다. 조건에 맞는 기록 ")+tool.Baro_comma(total)+tool.Baro_t("건."), list_body, "")

	return tool.Baro_page(db, config, tool.Baro_t("위키 관리 기록"), body, baro_gov_menu())
}
