package route

import (
	"database/sql"
	"net/url"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 [ceremony] 모듈 — 위키 수상식
//
//  규칙
//   - 개최는 운영자(소유자 · 관리자 · 어워드 담당)만 할 수 있다.
//   - 출마와 추천은 가입한 사용자만 할 수 있다.
//   - 투표는 가입한 사용자만, 부문마다 한 표씩만 할 수 있다.
//   - 수상식은 문서로 남는다. (투표: 이름공간)
// ============================================================

// baro_ceremony_ns : 수상식 문서가 들어가는 이름공간
const baro_ceremony_ns = "vote:"

// baro_ceremony_page_size : 한 쪽에 보여 주는 항목 수
const baro_ceremony_page_size = 30

// baro_ceremony_status_list : 진행 단계
func baro_ceremony_status_list() [][]string {
	return [][]string{
		{"run", tool.Baro_t("출마 접수 중"), "info"},
		{"vote", tool.Baro_t("투표 중"), "ok"},
		{"closed", tool.Baro_t("마감"), "muted"},
	}
}

// baro_ceremony_status_name_ko : 문서에 적을 단계 이름
//
//	문서 본문은 위키에 저장되는 글이라 언어와 무관하게 한국어로 남긴다.
func baro_ceremony_status_name_ko(status string) string {
	switch status {
	case "run":
		return "출마 접수 중"
	case "vote":
		return "투표 중"
	case "closed":
		return "마감"
	}

	return status
}

// baro_ceremony_status_badge : 단계 뱃지
func baro_ceremony_status_badge(status string) string {
	for _, row := range baro_ceremony_status_list() {
		if row[0] == status {
			return tool.Baro_badge(row[1], row[2])
		}
	}

	return tool.Baro_badge(status, "muted")
}

// baro_ceremony_esc : 짧은 HTML escape
func baro_ceremony_esc(data string) string {
	return tool.HTML_escape(data)
}

// baro_ceremony_page_no : 쪽 번호
func baro_ceremony_page_no(page string) int {
	num := tool.Str_to_int(page)
	if num < 1 {
		num = 1
	}
	if num > 2000 {
		num = 2000
	}

	return num
}

// baro_ceremony_can_open : 수상식을 개최할 수 있는 사람인가 (운영자만)
func baro_ceremony_can_open(db *sql.DB, ip string) bool {
	if !tool.Baro_is_user(ip) {
		return false
	}

	return tool.Baro_is_owner(db, ip) || tool.Baro_is_admin(db, ip) || tool.Baro_can(db, ip, "award")
}

// baro_ceremony_can_vote : 투표할 수 있는 사람인가 (가입한 사용자만)
func baro_ceremony_can_vote(db *sql.DB, config tool.Config) bool {
	if !tool.Baro_is_user(config.IP) {
		return false
	}

	// 차단·제재 중이면 투표하지 않는다
	if _, ok := tool.Baro_hook_can_act(db, config, "vote"); !ok {
		return false
	}

	return true
}

// baro_ceremony_menu : 공통 메뉴
func baro_ceremony_menu() [][]any {
	return [][]any{
		{"baro/ceremony", tool.Baro_t("위키 수상식")},
		{"baro", "바로위키"},
	}
}

// baro_ceremony_doc_title : 수상식 문서 이름 (투표: 이름공간)
func baro_ceremony_doc_title(name string) string {
	return baro_ceremony_ns + "위키 수상식/" + name
}

// baro_ceremony_one : 수상식 한 건 읽기
func baro_ceremony_one(db *sql.DB, id string) map[string]string {
	row := map[string]string{}

	name, doc, opener, status, run_end, vote_end := "", "", "", "", "", ""
	note, date, close_date, result := "", "", "", ""

	found := tool.QueryRow_DB(
		db,
		"select name, doc, opener, status, run_end, vote_end, note, date, close_date, result from baro_ceremony where id = ?",
		[]any{&name, &doc, &opener, &status, &run_end, &vote_end, &note, &date, &close_date, &result},
		id,
	)
	if !found {
		return row
	}

	row["id"] = id
	row["name"] = name
	row["doc"] = doc
	row["opener"] = opener
	row["status"] = status
	row["run_end"] = run_end
	row["vote_end"] = vote_end
	row["note"] = note
	row["date"] = date
	row["close_date"] = close_date
	row["result"] = result

	return row
}

// baro_ceremony_cat_list : 부문 목록
func baro_ceremony_cat_list(db *sql.DB, ceremony_id string) [][]string {
	rows := tool.Query_DB(
		db,
		"select id, name, description, sort, winner from baro_ceremony_cat where ceremony = ? order by sort + 0 asc, id + 0 asc",
		ceremony_id,
	)
	defer rows.Close()

	list := [][]string{}
	for rows.Next() {
		id, name, description, sort, winner := "", "", "", "", ""
		if rows.Scan(&id, &name, &description, &sort, &winner) != nil {
			continue
		}

		list = append(list, []string{id, name, description, sort, winner})
	}

	return list
}

// baro_ceremony_run_list : 한 부문의 후보 목록 (득표 수 포함)
func baro_ceremony_run_list(db *sql.DB, ceremony_id string, cat_id string) [][]string {
	rows := tool.Query_DB(
		db,
		"select id, target_kind, target, runner, reason, status from baro_ceremony_run where ceremony = ? and cat = ? and status != 'deleted' order by id + 0 asc",
		ceremony_id, cat_id,
	)
	defer rows.Close()

	list := [][]string{}
	for rows.Next() {
		id, target_kind, target, runner, reason, status := "", "", "", "", "", ""
		if rows.Scan(&id, &target_kind, &target, &runner, &reason, &status) != nil {
			continue
		}

		list = append(list, []string{id, target_kind, target, runner, reason, status})
	}
	rows.Close()

	for index, one := range list {
		count := 0
		tool.QueryRow_DB(db, "select count(*) from baro_ceremony_vote where run_id = ?", []any{&count}, one[0])
		list[index] = append(one, strconv.Itoa(count))
	}

	return list
}

// baro_ceremony_my_vote : 이 부문에 내가 이미 넣은 표 (없으면 "")
func baro_ceremony_my_vote(db *sql.DB, ceremony_id string, cat_id string, voter string) string {
	run_id := ""
	tool.QueryRow_DB(
		db,
		"select run_id from baro_ceremony_vote where ceremony = ? and cat = ? and voter = ? limit 1",
		[]any{&run_id}, ceremony_id, cat_id, voter,
	)

	return run_id
}

// baro_ceremony_target_html : 후보 표시 (사용자 또는 문서)
func baro_ceremony_target_html(db *sql.DB, config tool.Config, target_kind string, target string) string {
	if target_kind == "document" {
		return tool.Baro_doc_link(target)
	}

	return tool.Baro_user_link(db, target, config.IP)
}

// View_baro_ceremony_list : 위키 수상식 목록
func View_baro_ceremony_list(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_num := baro_ceremony_page_no(page)
	offset := (page_num - 1) * baro_ceremony_page_size

	rows := tool.Query_DB(
		db,
		"select id, name, doc, opener, status, run_end, vote_end, date from baro_ceremony order by date desc, id + 0 desc limit ?, ?",
		offset, baro_ceremony_page_size,
	)
	defer rows.Close()

	list := [][]string{}
	count := 0
	for rows.Next() {
		id, name, doc, opener, status, run_end, vote_end, date := "", "", "", "", "", "", "", ""
		if rows.Scan(&id, &name, &doc, &opener, &status, &run_end, &vote_end, &date) != nil {
			continue
		}

		count++

		period := baro_ceremony_esc(run_end) + " / " + baro_ceremony_esc(vote_end)
		if run_end == "" && vote_end == "" {
			period = `<span class="bw_muted">-</span>`
		}

		list = append(list, []string{
			`<a href="/baro/ceremony/view/` + tool.Url_parser(id) + `">` + baro_ceremony_esc(name) + `</a>`,
			baro_ceremony_status_badge(status),
			tool.Baro_user_link(db, opener, config.IP),
			period,
			tool.Baro_doc_link(doc),
			tool.Baro_time_cell(date),
		})
	}
	rows.Close()

	action := ""
	if baro_ceremony_can_open(db, config.IP) {
		action = tool.Baro_button("/baro/ceremony/open", tool.Get_language(db, "wiki_award_open", true), "primary")
	}

	body := tool.Baro_hero(
		tool.Get_language(db, "wiki_award", true),
		tool.Baro_t("운영자가 열고, 가입한 사용자가 출마하고, 가입한 사용자가 투표합니다."),
		action,
	)

	if !baro_ceremony_can_open(db, config.IP) {
		body += tool.Baro_alert("info", baro_ceremony_esc(tool.Get_language(db, "wiki_award_only_admin", true)))
	}

	inner := tool.Baro_table(
		[]string{tool.Get_language(db, "name", true), tool.Get_language(db, "state", true),
			tool.Baro_t("개최자"), tool.Baro_t("출마 마감 / 투표 마감"), tool.Get_language(db, "document", true), tool.Get_language(db, "date", true)},
		list, tool.Baro_t("아직 열린 수상식이 없습니다."),
	)
	inner += tool.Baro_paging(page_num, count, baro_ceremony_page_size, "/baro/ceremony/page/{}")

	body += tool.Baro_card("", inner)

	return tool.Baro_page(db, config, tool.Get_language(db, "wiki_award_list", true), body, baro_ceremony_menu())
}

// View_baro_ceremony_open : 수상식 개최 (운영자만)
func View_baro_ceremony_open(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !baro_ceremony_can_open(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Get_language(db, "wiki_award_only_admin", true))
	}

	if values != nil {
		name := tool.Baro_one_line(values.Get("name"))
		note := strings.ReplaceAll(values.Get("note"), "\r", "")
		run_end := tool.Baro_one_line(values.Get("run_end"))
		vote_end := tool.Baro_one_line(values.Get("vote_end"))
		cat_text := strings.ReplaceAll(values.Get("category"), "\r", "")

		if name == "" {
			return tool.Baro_deny(db, config, tool.Baro_t("수상식 이름을 적어 주세요."))
		}
		if strings.Contains(name, "/") {
			return tool.Baro_deny(db, config, tool.Baro_t("수상식 이름에는 / 를 쓸 수 없습니다."))
		}

		cat_lines := []string{}
		for _, line := range tool.Baro_split_lines(cat_text) {
			line = strings.TrimSpace(line)
			if line != "" {
				cat_lines = append(cat_lines, line)
			}
		}
		if len(cat_lines) == 0 {
			return tool.Baro_deny(db, config, tool.Baro_t("부문을 한 줄에 하나씩 적어 주세요."))
		}

		exist := ""
		if tool.QueryRow_DB(db, "select id from baro_ceremony where name = ? limit 1", []any{&exist}, name) {
			return tool.Baro_deny(db, config, tool.Baro_t("같은 이름의 수상식이 이미 있습니다."))
		}

		id := tool.Baro_next_id(db, "baro_ceremony")
		doc := baro_ceremony_doc_title(name)

		tool.Exec_DB(
			db,
			"insert into baro_ceremony (id, name, doc, opener, status, run_end, vote_end, note, date, close_date, result)"+
				" values (?, ?, ?, ?, 'run', ?, ?, ?, ?, '', '')",
			id, name, doc, config.IP, run_end, vote_end, note, tool.Baro_now(),
		)

		for index, one := range cat_lines {
			description := ""
			if pos := strings.Index(one, "|"); pos >= 0 {
				description = strings.TrimSpace(one[pos+1:])
				one = strings.TrimSpace(one[:pos])
			}
			if one == "" {
				continue
			}

			tool.Exec_DB(
				db,
				"insert into baro_ceremony_cat (id, ceremony, name, description, sort, winner) values (?, ?, ?, ?, ?, '')",
				tool.Baro_next_id(db, "baro_ceremony_cat"), id, one, description, strconv.Itoa(index+1),
			)
		}

		baro_ceremony_doc_write(db, config, id)

		tool.Baro_public_log(db, "ceremony_open", config.IP, doc, name, "수상식 번호 "+id)

		return tool.Get_redirect("/baro/ceremony/view/" + tool.Url_parser(id))
	}

	form := tool.Baro_input("name", tool.Get_language(db, "name", true), "", tool.Baro_t("예) 2026 상반기"), "문서 이름은 투표:위키 수상식/<이름> 이 됩니다.")
	form += tool.Baro_textarea("category", tool.Get_language(db, "wiki_award_category", true), tool.Baro_t("올해의 기여자\n올해의 문서\n올해의 신인"), 6, "",
		tool.Baro_t("한 줄에 부문 하나. 「부문 이름 | 설명」 처럼 세로줄로 설명을 붙일 수 있습니다."))
	form += tool.Baro_input_type("run_end", tool.Baro_t("출마 마감"), "", "date", "")
	form += tool.Baro_input_type("vote_end", tool.Baro_t("투표 마감"), "", "date", "")
	form += tool.Baro_textarea("note", tool.Get_language(db, "explanation", true), "", 5, "", tool.Baro_t("수상식 문서 맨 위에 들어갑니다."))
	form += tool.Baro_submit(tool.Get_language(db, "wiki_award_open", true), "primary")

	body := tool.Baro_hero(tool.Get_language(db, "wiki_award_open", true), tool.Baro_t("운영자만 열 수 있습니다."), "")
	body += tool.Baro_card("", tool.Baro_form("/baro/ceremony/open", form))

	return tool.Baro_page(db, config, tool.Get_language(db, "wiki_award_open", true), body, baro_ceremony_menu())
}

// View_baro_ceremony_view : 수상식 보기 · 투표
func View_baro_ceremony_view(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row := baro_ceremony_one(db, id)
	if len(row) == 0 {
		return tool.Baro_deny(db, config, tool.Baro_t("없는 수상식입니다."))
	}

	if values != nil {
		return baro_ceremony_vote_post(db, config, row, values)
	}

	cat_list := baro_ceremony_cat_list(db, id)

	head := [][]string{
		{tool.Get_language(db, "state", true), baro_ceremony_status_badge(row["status"])},
		{tool.Baro_t("개최자"), tool.Baro_user_link(db, row["opener"], config.IP)},
		{tool.Get_language(db, "document", true), tool.Baro_doc_link(row["doc"])},
		{tool.Baro_t("출마 마감"), baro_ceremony_esc(row["run_end"])},
		{tool.Baro_t("투표 마감"), baro_ceremony_esc(row["vote_end"])},
		{tool.Get_language(db, "date", true), tool.Baro_time_cell(row["date"])},
	}

	body := tool.Baro_hero(tool.Get_language(db, "wiki_award", true)+" — "+baro_ceremony_esc(row["name"]), "", "")
	body += tool.Baro_card("", tool.Baro_dl(head))

	if row["note"] != "" {
		body += tool.Baro_card(tool.Get_language(db, "explanation", true), `<pre class="bw_pre">`+baro_ceremony_esc(row["note"])+`</pre>`)
	}

	can_vote := baro_ceremony_can_vote(db, config)
	if !tool.Baro_is_user(config.IP) {
		body += tool.Baro_alert("info", baro_ceremony_esc(tool.Get_language(db, "wiki_award_only_user", true)))
	}

	for _, cat := range cat_list {
		cat_id := cat[0]
		run_list := baro_ceremony_run_list(db, id, cat_id)
		my_vote := ""
		if tool.Baro_is_user(config.IP) {
			my_vote = baro_ceremony_my_vote(db, id, cat_id, config.IP)
		}

		table_rows := [][]string{}
		for _, one := range run_list {
			mark := ""
			if one[0] == my_vote {
				mark = " " + tool.Baro_badge(tool.Baro_t("내 표"), "ok")
			}
			if cat[4] != "" && cat[4] == one[0] {
				mark += " " + tool.Baro_badge(tool.Baro_t("수상"), "ok")
			}

			vote_cell := `<span class="bw_muted">-</span>`
			if row["status"] == "vote" && can_vote && my_vote == "" {
				form := tool.Baro_hidden("action", "vote")
				form += tool.Baro_hidden("cat", cat_id)
				form += tool.Baro_hidden("run", one[0])
				form += tool.Baro_submit(tool.Get_language(db, "wiki_award_vote", true), "primary")

				vote_cell = tool.Baro_form("/baro/ceremony/view/"+tool.Url_parser(id), form)
			}

			// 결과가 나오기 전에는 득표 수를 감춘다 (표 쏠림을 막는다)
			vote_count := `<span class="bw_muted">-</span>`
			if row["status"] == "closed" {
				vote_count = baro_ceremony_esc(one[6])
			}

			table_rows = append(table_rows, []string{
				baro_ceremony_target_html(db, config, one[1], one[2]) + mark,
				tool.Baro_user_link(db, one[3], config.IP),
				baro_ceremony_esc(tool.Baro_trunc(one[4], 80)),
				vote_count,
				vote_cell,
			})
		}

		inner := ""
		if cat[2] != "" {
			inner += tool.Baro_note(cat[2])
		}
		inner += tool.Baro_table(
			[]string{tool.Get_language(db, "wiki_award_candidate", true), tool.Baro_t("올린 사람"),
				tool.Get_language(db, "why", true), tool.Get_language(db, "result_vote", true), tool.Get_language(db, "vote", true)},
			table_rows, tool.Baro_t("아직 후보가 없습니다."),
		)

		if my_vote != "" {
			inner += tool.Baro_note(tool.Get_language(db, "wiki_award_already_voted", true))
		}

		if row["status"] == "run" && tool.Baro_is_user(config.IP) {
			inner += tool.Baro_button("/baro/ceremony/run/"+tool.Url_parser(id)+"?cat="+tool.Url_parser(cat_id),
				tool.Get_language(db, "wiki_award_run", true), "")
		}

		body += tool.Baro_card(cat[1], inner)
	}

	if baro_ceremony_can_open(db, config.IP) && row["status"] != "closed" {
		manage := ""
		if row["status"] == "run" {
			manage += tool.Baro_button("/baro/ceremony/step/"+tool.Url_parser(id), "출마 마감하고 투표 시작", "primary") + " "
		}
		manage += tool.Baro_button("/baro/ceremony/close/"+tool.Url_parser(id), tool.Get_language(db, "wiki_award_close", true), "danger")

		body += tool.Baro_card(tool.Baro_t("운영"), manage)
	}

	if row["status"] == "closed" && row["result"] != "" {
		body += tool.Baro_card(tool.Get_language(db, "wiki_award_result", true), `<pre class="bw_pre">`+baro_ceremony_esc(row["result"])+`</pre>`)
	}

	return tool.Baro_page(db, config, tool.Get_language(db, "wiki_award", true), body, baro_ceremony_menu())
}

// baro_ceremony_vote_post : 투표 처리 (가입한 사용자만 · 부문마다 한 표)
func baro_ceremony_vote_post(db *sql.DB, config tool.Config, row map[string]string, values url.Values) string {
	if values.Get("action") != "vote" {
		return tool.Baro_deny(db, config, tool.Baro_t("알 수 없는 처리입니다."))
	}

	if !tool.Baro_is_user(config.IP) {
		return tool.Baro_deny(db, config, tool.Get_language(db, "wiki_award_only_user", true))
	}
	if !baro_ceremony_can_vote(db, config) {
		return tool.Baro_deny(db, config, tool.Baro_t("지금은 투표할 수 없습니다."))
	}
	if row["status"] != "vote" {
		return tool.Baro_deny(db, config, tool.Baro_t("지금은 투표 기간이 아닙니다."))
	}

	cat_id := values.Get("cat")
	run_id := values.Get("run")

	check := ""
	if !tool.QueryRow_DB(
		db,
		"select id from baro_ceremony_run where id = ? and ceremony = ? and cat = ? and status != 'deleted'",
		[]any{&check}, run_id, row["id"], cat_id,
	) {
		return tool.Baro_deny(db, config, tool.Baro_t("없는 후보입니다."))
	}

	if baro_ceremony_my_vote(db, row["id"], cat_id, config.IP) != "" {
		return tool.Baro_deny(db, config, tool.Get_language(db, "wiki_award_already_voted", true))
	}

	tool.Exec_DB(
		db,
		"insert into baro_ceremony_vote (id, ceremony, cat, run_id, voter, date) values (?, ?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_ceremony_vote"), row["id"], cat_id, run_id, config.IP, tool.Baro_now(),
	)

	return tool.Get_redirect("/baro/ceremony/view/" + tool.Url_parser(row["id"]))
}

// View_baro_ceremony_run : 출마 · 추천 (가입한 사용자만)
func View_baro_ceremony_run(config tool.Config, id string, cat_id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row := baro_ceremony_one(db, id)
	if len(row) == 0 {
		return tool.Baro_deny(db, config, tool.Baro_t("없는 수상식입니다."))
	}
	if !tool.Baro_is_user(config.IP) {
		return tool.Baro_deny(db, config, tool.Get_language(db, "wiki_award_only_user", true))
	}
	if row["status"] != "run" {
		return tool.Baro_deny(db, config, tool.Baro_t("지금은 출마 기간이 아닙니다."))
	}

	cat_list := baro_ceremony_cat_list(db, id)
	cat_options := [][]string{}
	for _, one := range cat_list {
		cat_options = append(cat_options, []string{one[0], one[1]})
	}
	if len(cat_options) == 0 {
		return tool.Baro_deny(db, config, tool.Baro_t("부문이 없습니다."))
	}

	if values != nil {
		want_cat := values.Get("cat")
		target_kind := values.Get("target_kind")
		target := strings.TrimSpace(values.Get("target"))
		reason := tool.Baro_one_line(values.Get("reason"))

		found := false
		for _, one := range cat_list {
			if one[0] == want_cat {
				found = true
				break
			}
		}
		if !found {
			return tool.Baro_deny(db, config, tool.Baro_t("없는 부문입니다."))
		}

		if target_kind != "document" {
			target_kind = "user"
		}
		if target == "" {
			target = config.IP
		}
		if target_kind == "document" {
			target = tool.Namespace_normalize(target)

			exist := ""
			if !tool.QueryRow_DB(db, "select title from data where title = ?", []any{&exist}, target) {
				return tool.Baro_deny(db, config, tool.Baro_t("없는 문서입니다."))
			}
		} else {
			if !tool.Baro_is_user(target) {
				return tool.Baro_deny(db, config, tool.Baro_t("사용자 이름을 확인해 주세요."))
			}

			exist := ""
			if !tool.QueryRow_DB(db, "select id from user_set where id = ? limit 1", []any{&exist}, target) {
				return tool.Baro_deny(db, config, tool.Baro_t("없는 사용자입니다."))
			}
		}

		dup := ""
		if tool.QueryRow_DB(
			db,
			"select id from baro_ceremony_run where ceremony = ? and cat = ? and target_kind = ? and target = ? and status != 'deleted' limit 1",
			[]any{&dup}, id, want_cat, target_kind, target,
		) {
			return tool.Baro_deny(db, config, tool.Baro_t("이미 같은 부문에 올라와 있는 후보입니다."))
		}

		run_id := tool.Baro_next_id(db, "baro_ceremony_run")
		tool.Exec_DB(
			db,
			"insert into baro_ceremony_run (id, ceremony, cat, target_kind, target, runner, reason, status, date) values (?, ?, ?, ?, ?, ?, ?, 'open', ?)",
			run_id, id, want_cat, target_kind, target, config.IP, reason, tool.Baro_now(),
		)

		if target_kind == "user" && target != config.IP {
			tool.Baro_notify(
				db, target, "award",
				row["name"]+tool.Baro_t(" 수상식 후보로 추천됐습니다."),
				"/baro/ceremony/view/"+tool.Url_parser(id), "",
			)
		}

		baro_ceremony_doc_write(db, config, id)

		return tool.Get_redirect("/baro/ceremony/view/" + tool.Url_parser(id))
	}

	form := tool.Baro_select("cat", tool.Get_language(db, "wiki_award_category", true), cat_options, cat_id, "")
	form += tool.Baro_select("target_kind", tool.Get_language(db, "type", true),
		[][]string{{"user", tool.Get_language(db, "user", true)}, {"document", tool.Get_language(db, "document", true)}}, "user", "")
	form += tool.Baro_input("target", tool.Get_language(db, "wiki_award_candidate", true), config.IP, "", tool.Baro_t("비워 두면 나 자신이 후보가 됩니다."))
	form += tool.Baro_textarea("reason", tool.Get_language(db, "why", true), "", 5, "", tool.Baro_t("왜 이 후보인지 적어 주세요."))
	form += tool.Baro_submit(tool.Get_language(db, "wiki_award_run", true), "primary")

	body := tool.Baro_hero(tool.Get_language(db, "wiki_award_run", true), baro_ceremony_esc(row["name"]), "")
	body += tool.Baro_card("", tool.Baro_form("/baro/ceremony/run/"+tool.Url_parser(id), form))

	return tool.Baro_page(db, config, tool.Get_language(db, "wiki_award_run", true), body, baro_ceremony_menu())
}

// View_baro_ceremony_step : 출마 마감 → 투표 시작 (운영자만)
func View_baro_ceremony_step(config tool.Config, id string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !baro_ceremony_can_open(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Get_language(db, "wiki_award_only_admin", true))
	}

	row := baro_ceremony_one(db, id)
	if len(row) == 0 {
		return tool.Baro_deny(db, config, tool.Baro_t("없는 수상식입니다."))
	}
	if row["status"] != "run" {
		return tool.Get_redirect("/baro/ceremony/view/" + tool.Url_parser(id))
	}

	tool.Exec_DB(db, "update baro_ceremony set status = 'vote' where id = ?", id)
	tool.Baro_public_log(db, "ceremony_vote_start", config.IP, row["doc"], row["name"], "수상식 번호 "+id)

	baro_ceremony_doc_write(db, config, id)

	return tool.Get_redirect("/baro/ceremony/view/" + tool.Url_parser(id))
}

// View_baro_ceremony_close : 투표 마감 · 결과 확정 (운영자만)
func View_baro_ceremony_close(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !baro_ceremony_can_open(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Get_language(db, "wiki_award_only_admin", true))
	}

	row := baro_ceremony_one(db, id)
	if len(row) == 0 {
		return tool.Baro_deny(db, config, tool.Baro_t("없는 수상식입니다."))
	}

	if values != nil {
		if row["status"] == "closed" {
			return tool.Get_redirect("/baro/ceremony/view/" + tool.Url_parser(id))
		}

		result_lines := []string{}

		for _, cat := range baro_ceremony_cat_list(db, id) {
			run_list := baro_ceremony_run_list(db, id, cat[0])

			best_id, best_text, best_count := "", "", -1
			tie := false
			for _, one := range run_list {
				count := tool.Str_to_int(one[6])
				if count > best_count {
					best_id, best_count, tie = one[0], count, false
					best_text = one[2]
				} else if count == best_count {
					tie = true
				}
			}

			if best_id == "" || best_count <= 0 {
				result_lines = append(result_lines, cat[1]+" : 수상자 없음")
				continue
			}

			// 5번 원칙 : 근거를 공개한다 (동점이면 동점이라고 적는다)
			line := cat[1] + " : " + best_text + " (" + strconv.Itoa(best_count) + "표)"
			if tie {
				line += " — 동점이 있어 운영자 확인이 필요합니다."
			}
			result_lines = append(result_lines, line)

			tool.Exec_DB(db, "update baro_ceremony_cat set winner = ? where id = ?", best_id, cat[0])

			// 사용자 후보가 이겼으면 알린다
			for _, one := range run_list {
				if one[0] != best_id {
					continue
				}
				if one[1] == "user" {
					tool.Baro_notify(
						db, one[2], "award",
						row["name"]+tool.Baro_t(" 수상식에서 ")+cat[1]+tool.Baro_t(" 부문에 선정됐습니다."),
						"/baro/ceremony/view/"+tool.Url_parser(id), "",
					)
				}
			}
		}

		result := strings.Join(result_lines, "\n")

		tool.Exec_DB(
			db,
			"update baro_ceremony set status = 'closed', close_date = ?, result = ? where id = ?",
			tool.Baro_now(), result, id,
		)

		tool.Baro_public_log(db, "ceremony_close", config.IP, row["doc"], row["name"], result)

		baro_ceremony_doc_write(db, config, id)

		return tool.Get_redirect("/baro/ceremony/view/" + tool.Url_parser(id))
	}

	form := tool.Baro_submit(tool.Get_language(db, "wiki_award_close", true), "danger")

	body := tool.Baro_hero(tool.Get_language(db, "wiki_award_close", true), baro_ceremony_esc(row["name"]), "")
	body += tool.Baro_alert("warn", tool.Baro_t("마감하면 부문마다 가장 많은 표를 받은 후보가 수상자로 확정되고, 결과가 수상식 문서에 남습니다."))
	body += tool.Baro_card("", tool.Baro_form("/baro/ceremony/close/"+tool.Url_parser(id), form))

	return tool.Baro_page(db, config, tool.Get_language(db, "wiki_award_close", true), body, baro_ceremony_menu())
}

// baro_ceremony_doc_write : 수상식 내용을 투표: 문서에 그대로 적는다
//
//	수상식은 화면에만 있는 것이 아니라 문서로 남는다.
func baro_ceremony_doc_write(db *sql.DB, config tool.Config, id string) {
	row := baro_ceremony_one(db, id)
	if len(row) == 0 {
		return
	}

	title := row["doc"]
	if title == "" {
		return
	}

	text := "= " + row["name"] + " 위키 수상식 =\n"
	text += "|| 상태 || " + baro_ceremony_status_name_ko(row["status"]) + " ||\n"
	text += "|| 개최자 || " + row["opener"] + " ||\n"
	text += "|| 출마 마감 || " + row["run_end"] + " ||\n"
	text += "|| 투표 마감 || " + row["vote_end"] + " ||\n\n"

	if row["note"] != "" {
		text += row["note"] + "\n\n"
	}

	text += "출마와 투표는 수상식 화면에서 합니다. (주소 : /baro/ceremony/view/" + id + ")\n\n"

	for _, cat := range baro_ceremony_cat_list(db, id) {
		text += "== " + cat[1] + " ==\n"
		if cat[2] != "" {
			text += cat[2] + "\n\n"
		}

		run_list := baro_ceremony_run_list(db, id, cat[0])
		if len(run_list) == 0 {
			text += "아직 후보가 없습니다.\n\n"
			continue
		}

		for _, one := range run_list {
			mark := ""
			if cat[4] != "" && cat[4] == one[0] {
				mark = " '''(수상)'''"
			}

			target := one[2]
			if one[1] == "document" {
				target = "[[" + one[2] + "]]"
			}

			line := " * " + target + mark
			if one[4] != "" {
				line += " — " + tool.Baro_one_line(one[4])
			}
			if row["status"] == "closed" {
				line += " (" + one[6] + "표)"
			}

			text += line + "\n"
		}

		text += "\n"
	}

	if row["status"] == "closed" && row["result"] != "" {
		text += "== 결과 ==\n"
		for _, line := range tool.Baro_split_lines(row["result"]) {
			if strings.TrimSpace(line) == "" {
				continue
			}

			text += " * " + line + "\n"
		}
		text += "\n"
	}

	text += "[[분류:위키 수상식]]\n"

	old_data := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&old_data}, title)
	if old_data == text {
		return
	}

	length := tool.Get_edit_length_diff(old_data, text)

	tool.Exec_DB(db, "delete from data where title = ?", title)
	tool.Exec_DB(db, "insert into data (title, data) values (?, ?)", title, text)
	tool.Do_add_history(db, title, text, tool.Get_time(), config.IP, "위키 수상식 갱신", length, "", "")
}
