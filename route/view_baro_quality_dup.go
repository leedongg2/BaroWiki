package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  9-5 중복 · 표절 · 복붙 감지 — /baro/quality/dup
//
//  방침 : 자동으로 차단하거나 지우지 않는다.
//         기계는 "닮았다" 까지만 말하고, 판단은 사람이 한다.
//         그래서 감지 결과는 baro_review 에 kind=dup 으로 올라간다.
// ============================================================

// baro_quality_dup_kind_name : 중복 종류 이름
func baro_quality_dup_kind_name(kind string) string {
	switch kind {
	case "cross":
		return tool.Baro_t("문서 간 중복")
	case "repeat":
		return tool.Baro_t("문서 안 반복")
	case "paste":
		return tool.Baro_t("대량 복붙")
	}

	return kind
}

// baro_quality_dup_status_badge : 중복 후보 상태 배지
func baro_quality_dup_status_badge(status string) string {
	switch status {
	case "ignored":
		return tool.Baro_badge(tool.Baro_t("무시함"), "muted")
	case "pending":
		return tool.Baro_badge(tool.Baro_t("검토 요청됨"), "info")
	case "done":
		return tool.Baro_badge("처리 완료", "ok")
	}

	return tool.Baro_badge(tool.Baro_t("확인 필요"), "warn")
}

// baro_quality_dup_norm : 문단 비교용 정규화
//
//	마크업 기호와 공백 차이를 지워, 표기만 살짝 바꾼 복사도 같은 문단으로 본다.
func baro_quality_dup_norm(text string) string {
	replacer := strings.NewReplacer(
		"'", "", `"`, "", "[", "", "]", "", "{", "", "}", "",
		"|", "", "=", "", "*", "", "#", "", ">", "", "-", "",
		"~", "", "^", "", ",", "", ".", "", "·", "", "…", "",
		"(", "", ")", "", "《", "", "》", "", "〈", "", "〉", "",
		"\t", " ", "\r", " ", "\n", " ",
	)

	text = replacer.Replace(text)
	text = strings.ToLower(text)

	return strings.Join(strings.Fields(text), " ")
}

// baro_quality_dup_paragraphs : 본문을 문단 단위로 자른다 (짧은 문단은 버린다)
func baro_quality_dup_paragraphs(data string, min_length int) [][]string {
	out := [][]string{}

	blocks := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n\n")
	for _, block := range blocks {
		normalized := baro_quality_dup_norm(block)
		if tool.Get_len(normalized) < min_length {
			continue
		}

		out = append(out, []string{tool.Sha224(normalized), tool.Baro_trunc(tool.Baro_one_line(block), 200), normalized})
	}

	return out
}

// baro_quality_dup_min_length : 중복으로 볼 최소 문단 길이
func baro_quality_dup_min_length(db *sql.DB) int {
	value := tool.Baro_get_int(db, "dup_min_paragraph", 80)
	if value < 30 {
		value = 30
	}

	return value
}

// baro_quality_dup_exists : 이미 기록된 중복 후보인가
func baro_quality_dup_exists(db *sql.DB, title string, target string, kind string, detail string) bool {
	found := ""

	return tool.QueryRow_DB(db,
		"select id from baro_dup where title = ? and target = ? and kind = ? and detail = ?",
		[]any{&found}, title, target, kind, detail)
}

// baro_quality_dup_add : 중복 후보 한 건을 기록하고 검토 큐에 올린다
func baro_quality_dup_add(db *sql.DB, title string, target string, kind string, ratio int, detail string) bool {
	detail = tool.Baro_trunc(detail, 200)

	if baro_quality_dup_exists(db, title, target, kind, detail) {
		return false
	}

	id := tool.Baro_next_id(db, "baro_dup")

	tool.Exec_DB(db,
		"insert into baro_dup (id, title, target, kind, ratio, date, status, detail) values (?, ?, ?, ?, ?, ?, 'new', ?)",
		id, title, target, kind, tool.Baro_itoa(ratio), tool.Baro_now(), detail)

	// 자동 차단이 아니라 검토 큐로 보낸다
	if !baro_quality_review_exists(db, title, id, "dup") {
		risk := ratio
		if risk > 80 {
			risk = 80
		}
		if risk < 20 {
			risk = 20
		}

		baro_quality_review_insert(db, title, id, "", "dup", risk, tool.Baro_now())
	}

	return true
}

// baro_quality_dup_scan_run : 문서 본문을 문단 단위로 비교해 중복 후보를 찾는다
//
//	한 번에 최대 200문서
func baro_quality_dup_scan_run(db *sql.DB, limit int) (int, int) {
	if limit < 1 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}

	offset := tool.Baro_get_int(db, "quality_dup_offset", 0)
	if offset < 0 {
		offset = 0
	}

	type doc_row struct {
		Title, Data string
	}

	list := []doc_row{}

	rows := tool.Query_DB(db,
		"select title, data from data where "+tool.Get_except_document_name_SQL("title")+" order by title limit ?, ?",
		offset, limit)
	defer rows.Close()

	for rows.Next() {
		row := doc_row{}
		if err := rows.Scan(&row.Title, &row.Data); err != nil {
			continue
		}

		list = append(list, row)
	}

	min_length := baro_quality_dup_min_length(db)
	added := 0

	// 해시 → 문단이 나온 문서 목록
	type hit struct {
		Title, Sample string
		Length        int
	}

	index := map[string][]hit{}
	doc_length := map[string]int{}

	for _, row := range list {
		doc_length[row.Title] = tool.Get_len(baro_quality_dup_norm(row.Data))

		seen := map[string]int{}
		for _, paragraph := range baro_quality_dup_paragraphs(row.Data, min_length) {
			hash, sample, normalized := paragraph[0], paragraph[1], paragraph[2]

			seen[hash]++
			if seen[hash] == 2 {
				// 같은 문서 안에서 같은 문단이 되풀이된다
				ratio := 100
				if doc_length[row.Title] > 0 {
					ratio = tool.Get_len(normalized) * 100 / doc_length[row.Title]
				}

				if baro_quality_dup_add(db, row.Title, row.Title, "repeat", ratio, sample) {
					added++
				}
			}

			if seen[hash] > 1 {
				continue
			}

			index[hash] = append(index[hash], hit{Title: row.Title, Sample: sample, Length: tool.Get_len(normalized)})
		}
	}

	// 문서 사이 중복
	for _, hits := range index {
		if len(hits) < 2 {
			continue
		}

		base := hits[0]
		for _, other := range hits[1:] {
			total := doc_length[base.Title]
			if doc_length[other.Title] < total {
				total = doc_length[other.Title]
			}

			ratio := 100
			if total > 0 {
				ratio = base.Length * 100 / total
			}
			if ratio > 100 {
				ratio = 100
			}

			if baro_quality_dup_add(db, base.Title, other.Title, "cross", ratio, base.Sample) {
				added++
			}
		}
	}

	if len(list) < limit {
		tool.Baro_put(db, "quality_dup_offset", "0")
	} else {
		tool.Baro_put(db, "quality_dup_offset", tool.Baro_itoa(offset+limit))
	}

	if added > 0 {
		tool.Baro_metric_bump(db, "dup_found", tool.Baro_today(), added)
	}

	return len(list), added
}

// View_baro_quality_dup_list : 중복 후보 목록
func View_baro_quality_dup_list(config tool.Config, page string, status string, kind string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_num := baro_quality_page_num(page)
	offset := (page_num - 1) * tool.Baro_page_size

	switch status {
	case "new", "ignored", "pending", "done", "all":
	default:
		status = "new"
	}

	if kind != "cross" && kind != "repeat" {
		kind = ""
	}

	new_count := baro_quality_count(db, "select count(*) from baro_dup where status = 'new'")
	ignored := baro_quality_count(db, "select count(*) from baro_dup where status = 'ignored'")
	pending := baro_quality_count(db, "select count(*) from baro_dup where status = 'pending'")
	done_count := baro_quality_count(db, "select count(*) from baro_dup where status = 'done'")
	cross := baro_quality_count(db, "select count(*) from baro_dup where kind = 'cross'")
	repeat := baro_quality_count(db, "select count(*) from baro_dup where kind = 'repeat'")

	where := []string{}
	args := []any{}

	if status != "all" {
		where = append(where, "status = ?")
		args = append(args, status)
	}
	if kind != "" {
		where = append(where, "kind = ?")
		args = append(args, kind)
	}

	query := "select id, title, target, kind, ratio, date, status, detail from baro_dup"
	if len(where) > 0 {
		query += " where " + strings.Join(where, " and ")
	}
	query += " order by ratio + 0 desc, id + 0 desc limit ?, ?"
	args = append(args, offset, tool.Baro_page_size)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		id, title, target, row_kind, ratio, date, row_status, detail := "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &title, &target, &row_kind, &ratio, &date, &row_status, &detail); err != nil {
			continue
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/quality/dup/view/` + tool.Url_parser(id) + `">#` + tool.HTML_escape(id) + `</a>`,
			tool.HTML_escape(baro_quality_dup_kind_name(row_kind)),
			baro_quality_doc_cell(title),
			baro_quality_doc_cell(target),
			tool.HTML_escape(ratio) + "%",
			tool.HTML_escape(tool.Baro_trunc(detail, 60)),
			baro_quality_dup_status_badge(row_status),
			tool.Baro_time_cell(date),
			tool.Baro_button("/baro/quality/dup/view/"+tool.Url_parser(id), "확인", "primary"),
		})
	}

	list_html := tool.Baro_table(
		[]string{"번호", "종류", "문서", tool.Baro_t("상대 문서"), tool.Baro_t("겹침"), tool.Baro_t("문단 앞부분"), "상태", tool.Baro_t("감지"), "관리"},
		table_rows, "")

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("이 조건에 해당하는 중복 후보가 없습니다. [중복 검사] 를 실행해 보세요."))
	}

	page_url := "/baro/quality/dup?status=" + tool.Url_parser(status) + "&kind=" + tool.Url_parser(kind) + "&page={}"
	list_html += tool.Baro_paging(page_num, len(table_rows), tool.Baro_page_size, page_url)

	// 탭 주소는 상태와 종류를 함께 담는다 (한쪽을 바꿔도 다른 쪽 조건이 풀리지 않게)
	dup_url := func(new_status string, new_kind string) string {
		return "/baro/quality/dup?status=" + tool.Url_parser(new_status) + "&kind=" + tool.Url_parser(new_kind)
	}

	active := dup_url(status, kind)

	body := baro_quality_crumb(tool.Baro_t("중복 감지"))
	body += tool.Baro_hero(
		tool.Baro_t("중복 · 복붙 감지"),
		tool.Baro_t("닮은 문단을 찾아 알려 줍니다. 지우지도 막지도 않습니다."),
		tool.Baro_button("/baro/quality/dup/scan", tool.Baro_t("중복 검사 실행"), "primary")+" "+
			tool.Baro_button("/baro/quality/dup/paste", tool.Baro_t("대량 복붙 경고"), "")+" "+
			tool.Baro_button("/baro/review?status=pending", tool.Baro_t("검토 큐"), ""),
	)
	body += baro_quality_nav("/baro/quality/dup")

	body += tool.Baro_alert("info",
		tool.Baro_t("<b>이 기능은 자동 차단을 하지 않습니다.</b> 문단이 겹친다고 표절이라고 단정할 수 없기 때문입니다. ")+
			tool.Baro_t("(같은 법령 조문, 같은 공식 소개문, 같은 인용문일 수 있습니다.) ")+
			tool.Baro_t("감지된 후보는 모두 편집 검토 큐(kind=dup)로 올라가 사람이 확인합니다."))

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("확인 필요"), tool.Baro_comma(new_count), ""},
		{tool.Baro_t("검토 요청됨"), tool.Baro_comma(pending), ""},
		{tool.Baro_t("무시함"), tool.Baro_comma(ignored), ""},
		{tool.Baro_t("문서 간 중복"), tool.Baro_comma(cross), ""},
		{tool.Baro_t("문서 안 반복"), tool.Baro_comma(repeat), ""},
	})

	body += tool.Baro_tabs([][]string{
		{dup_url("new", kind), tool.Baro_t("확인 필요 (") + tool.Baro_itoa(new_count) + ")"},
		{dup_url("pending", kind), tool.Baro_t("검토 요청됨 (") + tool.Baro_itoa(pending) + ")"},
		{dup_url("ignored", kind), tool.Baro_t("무시함 (") + tool.Baro_itoa(ignored) + ")"},
		{dup_url("done", kind), tool.Baro_t("처리 완료 (") + tool.Baro_itoa(done_count) + ")"},
		{dup_url("all", kind), tool.Baro_t("전체")},
	}, active)

	// 종류 거르개
	body += tool.Baro_tabs([][]string{
		{dup_url(status, ""), tool.Baro_t("모든 종류")},
		{dup_url(status, "cross"), tool.Baro_t("문서 간 중복 (") + tool.Baro_itoa(cross) + ")"},
		{dup_url(status, "repeat"), tool.Baro_t("문서 안 반복 (") + tool.Baro_itoa(repeat) + ")"},
	}, active)

	body += tool.Baro_card_ex(tool.Baro_t("중복 후보"), tool.Baro_t("겹치는 비율이 높은 순서입니다."), list_html, "")

	body += tool.Baro_card(tool.Baro_t("겹친 문단을 발견했다면"), tool.Baro_list([]string{
		tool.Baro_t("두 문서가 같은 주제를 다루고 있다면 <b>병합</b>을 검토하세요. 지우는 것이 아니라 합치는 것입니다."),
		tool.Baro_t("한쪽이 다른 쪽의 요약이라면 요약을 남기고 본문 링크를 거는 편이 좋습니다."),
		tool.Baro_t("바깥 글을 그대로 옮긴 것으로 보이면 저작권 문제이므로 격리 절차를 밟습니다. 즉시 삭제하지 않습니다."),
		tool.Baro_t("공식 소개문·법령·인용문처럼 겹치는 것이 당연한 문단이면 [무시]로 표시하세요."),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("중복 감지"), body, baro_quality_menu())
}

// View_baro_quality_dup_scan : 중복 검사 실행 (권한 review)
func View_baro_quality_dup_scan(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/quality/dup/scan"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "review") {
		return tool.Baro_deny(db, config, tool.Baro_t("중복 검사는 검토 권한이 있는 사용자만 실행할 수 있습니다."))
	}

	message := ""

	if values != nil {
		if tool.Baro_read_only(db) {
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
		} else if !tool.Baro_rate_ok(db, "quality_dup_scan", config.IP, 5, 600) {
			message = tool.Baro_alert("warn", tool.Baro_t("중복 검사를 너무 자주 실행했습니다. 잠시 후 다시 시도하세요."))
		} else {
			limit := tool.Str_to_int(baro_quality_val(values, "limit"))
			docs, added := baro_quality_dup_scan_run(db, limit)

			tool.Baro_public_log(db, "dup_scan", config.IP, "",
				"문서 "+tool.Baro_itoa(docs)+"개 비교", "중복 후보 "+tool.Baro_itoa(added)+"건 등록")
			tool.Baro_audit(db, config, "dup_scan", "data", "",
				"docs="+tool.Baro_itoa(docs)+" added="+tool.Baro_itoa(added), "중복 검사")

			return tool.Get_redirect("/baro/quality/dup")
		}
	}

	offset := tool.Baro_get_int(db, "quality_dup_offset", 0)
	doc_total := baro_quality_count(db,
		"select count(*) from data where "+tool.Get_except_document_name_SQL("title"))

	form := tool.Baro_select("limit", tool.Baro_t("한 번에 비교할 문서 수"), [][]string{
		{"50", tool.Baro_t("50문서")},
		{"100", tool.Baro_t("100문서")},
		{"200", tool.Baro_t("200문서 (최대)")},
	}, "100", tool.Baro_t("이어서 실행하면 다음 묶음을 비교합니다."))
	form += tool.Baro_submit(tool.Baro_t("중복 검사 실행"), "primary")

	body := baro_quality_crumb(tool.Baro_t("중복 검사"))
	body += tool.Baro_hero(tool.Baro_t("중복 검사"), tool.Baro_t("문단을 정규화해 해시로 견줍니다. 표기만 바꾼 복사도 같은 문단으로 봅니다."), "")
	body += baro_quality_nav("/baro/quality/dup")
	body += message

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("일반 문서"), tool.Baro_comma(doc_total), ""},
		{tool.Baro_t("다음 시작 위치"), tool.Baro_comma(offset) + tool.Baro_t("번째"), tool.Baro_t("이어서 비교")},
		{tool.Baro_t("최소 문단 길이"), tool.Baro_itoa(baro_quality_dup_min_length(db)) + tool.Baro_t("자"), tool.Baro_t("이보다 짧으면 넘어감")},
	})

	body += tool.Baro_card_ex(tool.Baro_t("검사 실행"), tool.Baro_t("한 번에 최대 200문서까지 비교합니다."),
		tool.Baro_form("/baro/quality/dup/scan", form),
		tool.Baro_button("/baro/quality/dup", tool.Baro_t("중복 후보 목록"), ""))

	body += tool.Baro_card(tool.Baro_t("비교 방법"), tool.Baro_list([]string{
		tool.Baro_t("본문을 빈 줄 기준으로 문단으로 자릅니다."),
		tool.Baro_t("따옴표·대괄호·글머리표 같은 마크업 기호와 공백 차이를 지웁니다."),
		tool.Baro_t("문단마다 해시를 내고, 서로 다른 문서에서 같은 해시가 나오면 문서 간 중복으로 봅니다."),
		tool.Baro_t("한 문서 안에서 같은 해시가 두 번 이상 나오면 문서 안 반복으로 봅니다."),
		tool.Baro_t("찾은 후보는 지우지 않고 검토 큐(kind=dup)에 올립니다."),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("중복 검사"), body, baro_quality_menu())
}

// View_baro_quality_dup_view : 중복 후보 한 건 확인 / 처리
func View_baro_quality_dup_view(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	id = strings.TrimSpace(id)

	title, target, kind, ratio, date, status, detail := "", "", "", "", "", "", ""

	found := tool.QueryRow_DB(db,
		"select title, target, kind, ratio, date, status, detail from baro_dup where id = ?",
		[]any{&title, &target, &kind, &ratio, &date, &status, &detail}, id)

	if !found {
		body := baro_quality_crumb(tool.Baro_t("중복 후보"))
		body += tool.Baro_hero(tool.Baro_t("중복 후보를 찾을 수 없습니다"), tool.Baro_t("번호가 잘못되었거나 이미 지워진 기록입니다."), "")
		body += baro_quality_nav("/baro/quality/dup")
		body += tool.Baro_alert("warn", tool.Baro_t("중복 번호 ")+tool.HTML_escape(tool.Baro_trunc(id, 40))+tool.Baro_t(" 에 해당하는 기록이 없습니다."))
		body += tool.Baro_card(tool.Baro_t("다음 할 일"), tool.Baro_button("/baro/quality/dup", tool.Baro_t("중복 후보 목록"), "primary")+" "+
			tool.Baro_button("/baro/quality/dup/scan", tool.Baro_t("중복 검사"), ""))

		return tool.Baro_page(db, config, tool.Baro_t("중복 후보"), body, baro_quality_menu())
	}

	message := ""

	if values != nil {
		switch {
		case !tool.Baro_is_user(config.IP):
			message = tool.Baro_alert("danger", tool.Baro_t("처리하려면 로그인이 필요합니다."))
		case !tool.Baro_can(db, config.IP, "review"):
			message = tool.Baro_alert("danger", tool.Baro_t("중복 후보 처리는 검토 권한이 있는 사용자만 할 수 있습니다."))
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
		default:
			act := baro_quality_val(values, "act")
			reason := tool.Baro_trunc(baro_quality_val(values, "reason"), 500)

			switch act {
			case "ignore":
				tool.Exec_DB(db, "update baro_dup set status = 'ignored' where id = ?", id)
				tool.Exec_DB(db,
					"update baro_review set status = 'approved', reviewer = ?, review_date = ?, comment = ? where kind = 'dup' and rev = ? and status = 'pending'",
					config.IP, tool.Baro_now(), "중복 후보 무시 : "+reason, id)

				tool.Baro_public_log(db, "dup_ignore", config.IP, title, reason, "중복 후보 #"+id+" 무시")
				tool.Baro_audit(db, config, "dup_ignore", "dup:"+id, status, "ignored", reason)

				return tool.Get_redirect("/baro/quality/dup")
			case "review":
				tool.Exec_DB(db, "update baro_dup set status = 'pending' where id = ?", id)

				if !baro_quality_review_exists(db, title, id, "dup") {
					baro_quality_review_insert(db, title, id, "", "dup", 40, tool.Baro_now())
				}

				tool.Baro_public_log(db, "dup_review", config.IP, title, reason, "중복 후보 #"+id+" 검토 요청")
				tool.Baro_audit(db, config, "dup_review", "dup:"+id, status, "pending", reason)

				return tool.Get_redirect("/baro/quality/dup?status=pending")
			case "done":
				if reason == "" {
					message = tool.Baro_alert("danger", tool.Baro_t("처리 완료로 표시하려면 어떻게 처리했는지 적어야 합니다."))
					break
				}

				tool.Exec_DB(db, "update baro_dup set status = 'done' where id = ?", id)
				tool.Exec_DB(db,
					"update baro_review set status = 'approved', reviewer = ?, review_date = ?, comment = ? where kind = 'dup' and rev = ? and status = 'pending'",
					config.IP, tool.Baro_now(), "중복 처리 완료 : "+reason, id)

				tool.Baro_contrib_add(db, config.IP, "review", title, "", "", "중복 후보 처리 : "+tool.Baro_trunc(reason, 200))

				tool.Baro_public_log(db, "dup_done", config.IP, title, reason, "중복 후보 #"+id+" 처리 완료")
				tool.Baro_audit(db, config, "dup_done", "dup:"+id, status, "done", reason)

				return tool.Get_redirect("/baro/quality/dup")
			default:
				message = tool.Baro_alert("danger", tool.Baro_t("알 수 없는 처리입니다."))
			}
		}
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/quality/dup", tool.Baro_t("중복 감지")},
		{"", tool.Baro_t("후보 #") + id},
	})

	body += tool.Baro_hero(
		"중복 후보 #"+id,
		baro_quality_dup_kind_name(kind)+tool.Baro_t(" · 겹침 ")+ratio+"%",
		tool.Baro_button("/w/"+tool.Url_parser(title), "문서 보기", "")+" "+
			tool.Baro_button("/w/"+tool.Url_parser(target), "상대 문서 보기", ""),
	)

	body += baro_quality_nav("/baro/quality/dup")
	body += message

	body += tool.Baro_kv_grid([][]string{
		{"종류", tool.HTML_escape(baro_quality_dup_kind_name(kind))},
		{"문서", baro_quality_doc_cell(title)},
		{tool.Baro_t("상대 문서"), baro_quality_doc_cell(target)},
		{tool.Baro_t("겹치는 비율"), tool.HTML_escape(ratio) + "%"},
		{"상태", baro_quality_dup_status_badge(status)},
		{tool.Baro_t("감지 시각"), tool.Baro_time_cell(date)},
		{tool.Baro_t("문서 상태"), tool.Baro_status_badge(tool.Baro_doc_status_get(db, title))},
		{tool.Baro_t("상대 문서 상태"), tool.Baro_status_badge(tool.Baro_doc_status_get(db, target))},
	})

	body += tool.Baro_card(tool.Baro_t("겹친 문단 앞부분"), tool.Baro_code(detail))

	// 두 문서 길이 비교
	title_length := tool.Get_len(baro_quality_doc_data(db, title))
	target_length := tool.Get_len(baro_quality_doc_data(db, target))

	body += tool.Baro_card(tool.Baro_t("두 문서 크기"), tool.Baro_table(
		[]string{"문서", tool.Baro_t("본문 길이"), "상태", tool.Baro_t("출처 수")},
		[][]string{
			{
				baro_quality_doc_cell(title),
				tool.Baro_comma(title_length) + tool.Baro_t("자"),
				tool.Baro_status_badge(tool.Baro_doc_status_get(db, title)),
				tool.Baro_itoa(baro_quality_count(db, "select count(*) from baro_citation where title = ?", title)),
			},
			{
				baro_quality_doc_cell(target),
				tool.Baro_comma(target_length) + tool.Baro_t("자"),
				tool.Baro_status_badge(tool.Baro_doc_status_get(db, target)),
				tool.Baro_itoa(baro_quality_count(db, "select count(*) from baro_citation where title = ?", target)),
			},
		}, ""))

	// 병합 안내
	merge_target, merge_source := title, target
	if title_length < target_length {
		merge_target, merge_source = target, title
	}

	if kind == "repeat" {
		body += tool.Baro_card(tool.Baro_t("반복 문단 정리 안내"), tool.Baro_list([]string{
			tool.Baro_t("같은 문단이 한 문서 안에서 되풀이되고 있습니다. 보통 문단을 옮기다 생긴 실수입니다."),
			tool.Baro_t("어느 위치가 맞는지 고른 뒤 나머지를 지우고, 편집 요약에 <b>반복 문단 정리</b> 라고 적어 주세요."),
			tool.Baro_t("목차 구조 자체가 바뀌어야 한다면 토론을 먼저 여는 편이 좋습니다."),
		}))
	} else {
		body += tool.Baro_card(tool.Baro_t("병합 안내"), tool.Baro_list([]string{
			tool.Baro_t("내용이 더 갖춰진 <b>") + tool.HTML_escape(tool.Namespace_display(merge_target)) + tool.Baro_t("</b> 쪽으로 모으는 편이 보통 낫습니다."),
			tool.Baro_t("합칠 때는 두 문서의 출처를 모두 옮겨 주세요. 출처가 사라지면 검증할 수 없게 됩니다."),
			"<b>" + tool.HTML_escape(tool.Namespace_display(merge_source)) + tool.Baro_t("</b> 는 지우지 말고 넘겨주기로 바꿉니다. 편집 이력이 라이선스상 필요합니다."),
			tool.Baro_t("편집 요약에 어느 문서에서 가져왔는지 반드시 적어 주세요. (CC BY-SA 저작자 표시)"),
			tool.Baro_t("판단이 서지 않으면 두 문서의 토론을 먼저 여세요."),
		}))
	}

	if tool.Baro_can(db, config.IP, "review") {
		form := tool.Baro_textarea("reason", tool.Baro_t("처리 사유 / 메모"), "", 3,
			tool.Baro_t("왜 이렇게 판단했는지 적어 주세요."), tool.Baro_t("무시 외의 처리는 사유가 필요합니다. 공개 운영 로그에 남습니다."))
		form += `<div class="bw_f">` +
			tool.Baro_t(`<button type="submit" class="bw_btn" name="act" value="ignore">무시 (겹쳐도 되는 문단)</button> `) +
			tool.Baro_t(`<button type="submit" class="bw_btn bw_btn_primary" name="act" value="review">검토 요청</button> `) +
			tool.Baro_t(`<button type="submit" class="bw_btn" name="act" value="done">처리 완료</button>`) +
			`</div>`

		body += tool.Baro_card_ex("처리", tool.Baro_t("자동 차단은 하지 않습니다. 사람이 고르는 세 갈래뿐입니다."),
			tool.Baro_form("/baro/quality/dup/view/"+tool.Url_parser(id), form),
			tool.Baro_button("/baro/quality/dup", tool.Baro_t("중복 후보 목록"), ""))
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("이 화면은 누구나 볼 수 있지만, 처리하려면 검토 권한이 필요합니다."))
	}

	return tool.Baro_page(db, config, "중복 후보 #"+id, body, baro_quality_menu())
}

// View_baro_quality_dup_paste : 대량 복붙 경고 (history 의 leng 이용)
func View_baro_quality_dup_paste(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_num := baro_quality_page_num(page)
	threshold := tool.Baro_get_int(db, "dup_paste_length", 5000)
	if threshold < 500 {
		threshold = 500
	}

	message := ""

	// 검토 큐로 보내기
	if values != nil {
		switch {
		case !tool.Baro_can(db, config.IP, "review"):
			message = tool.Baro_alert("danger", tool.Baro_t("검토 큐로 보내려면 검토 권한이 필요합니다."))
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
		default:
			post_title := tool.Namespace_normalize(baro_quality_val(values, "title"))
			post_rev := baro_quality_val(values, "rev")

			if post_title == "" || post_rev == "" {
				message = tool.Baro_alert("danger", tool.Baro_t("문서와 판 번호가 필요합니다."))
				break
			}

			if baro_quality_review_exists(db, post_title, post_rev, "edit") {
				message = tool.Baro_alert("info", tool.Baro_t("이미 검토 큐에 올라간 편집입니다."))
				break
			}

			edit := baro_quality_edit{Title: post_title, Rev: post_rev}
			tool.QueryRow_DB(db,
				"select data, send, leng, date, ip from history where title = ? and id = ?",
				[]any{&edit.Data, &edit.Send, &edit.Leng, &edit.Date, &edit.User},
				post_title, post_rev)

			risk, _ := baro_quality_risk(db, edit)
			if risk < 30 {
				risk = 30
			}

			baro_quality_review_insert(db, post_title, post_rev, edit.User, "edit", risk, edit.Date)

			tool.Baro_public_log(db, "paste_review", config.IP, post_title, "대량 복붙 확인 요청", "r"+post_rev)
			tool.Baro_audit(db, config, "paste_review", post_title+" r"+post_rev, "", "pending", "대량 복붙 검토 요청")

			return tool.Get_redirect("/baro/quality/dup/paste")
		}
	}

	// 길이가 크게 늘어난 편집을 찾는다
	type paste_row struct {
		Title, Rev, User, Date, Send string
		Delta                        int
	}

	list := []paste_row{}

	// 기준 자릿수 이상으로 늘어난 편집만 먼저 거른다.
	// (기준이 5,000 이면 "+5000" 처럼 부호를 더해 다섯 칸 이상인 값만 본다)
	min_digits := len(tool.Baro_itoa(threshold)) + 1

	rows := tool.Query_DB(db,
		"select title, id, ip, date, send, leng from history where leng like '+%' and length(leng) >= ? order by date desc limit 800",
		min_digits)
	defer rows.Close()

	for rows.Next() {
		title, rev, user, date, send, leng := "", "", "", "", "", ""
		if err := rows.Scan(&title, &rev, &user, &date, &send, &leng); err != nil {
			continue
		}

		delta := tool.Str_to_int(leng)
		if delta < threshold {
			continue
		}

		list = append(list, paste_row{Title: title, Rev: rev, User: user, Date: date, Send: send, Delta: delta})
	}

	start := (page_num - 1) * tool.Baro_page_size
	end := start + tool.Baro_page_size
	if start > len(list) {
		start = len(list)
	}
	if end > len(list) {
		end = len(list)
	}

	table_rows := [][]string{}
	for _, row := range list[start:end] {
		queued := baro_quality_review_exists(db, row.Title, row.Rev, "edit")

		action := tool.Baro_badge(tool.Baro_t("검토 큐에 있음"), "info")
		if !queued && tool.Baro_can(db, config.IP, "review") {
			action = tool.Baro_form("/baro/quality/dup/paste",
				tool.Baro_hidden("title", row.Title)+
					tool.Baro_hidden("rev", row.Rev)+
					tool.Baro_submit(tool.Baro_t("검토 큐로"), ""))
		} else if !queued {
			action = tool.Baro_badge(tool.Baro_t("미등록"), "muted")
		}

		table_rows = append(table_rows, []string{
			baro_quality_doc_cell(row.Title),
			baro_quality_diff_link(row.Title, row.Rev),
			tool.Baro_user_link(db, row.User, config.IP),
			`<b>+` + tool.Baro_comma(row.Delta) + `</b>`,
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Send), 40)),
			tool.Baro_time_cell(row.Date),
			action,
		})
	}

	list_html := tool.Baro_table(
		[]string{"문서", "판", tool.Baro_t("편집자"), tool.Baro_t("늘어난 길이"), tool.Baro_t("요약"), "시각", "관리"},
		table_rows, "")

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("한 번에 ") + tool.Baro_comma(threshold) + tool.Baro_t("자 넘게 들어온 편집이 없습니다."))
	}

	list_html += tool.Baro_paging(page_num, len(table_rows), tool.Baro_page_size, "/baro/quality/dup/paste?page={}")

	body := baro_quality_crumb(tool.Baro_t("대량 복붙 경고"))
	body += tool.Baro_hero(
		tool.Baro_t("대량 복붙 경고"),
		tool.Baro_t("한 편집에서 아주 큰 덩어리가 한꺼번에 들어온 경우입니다."),
		"",
	)
	body += baro_quality_nav("/baro/quality/dup")
	body += message

	body += tool.Baro_alert("warn",
		tool.Baro_t("큰 덩어리가 들어왔다고 잘못된 편집이 아닙니다. 오래 준비한 새 문서일 수 있습니다. ")+
			tool.Baro_t("다만 다른 곳의 글을 그대로 옮겨 온 경우일 수도 있어, 사람 눈으로 한 번 보자는 목록입니다. ")+
			tool.Baro_t("<b>자동으로 되돌리지 않습니다.</b>"))

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("경고 기준"), tool.Baro_comma(threshold) + tool.Baro_t("자"), tool.Baro_t("baro_set 의 dup_paste_length")},
		{tool.Baro_t("찾은 편집"), tool.Baro_comma(len(list)), tool.Baro_t("최근 800건 중")},
	})

	body += tool.Baro_card_ex(tool.Baro_t("큰 덩어리가 들어온 편집"), tool.Baro_t("최근 편집부터 봅니다."), list_html,
		tool.Baro_button("/baro/quality/dup", tool.Baro_t("중복 후보 목록"), "")+" "+
			tool.Baro_button("/baro/review", tool.Baro_t("검토 큐"), ""))

	body += tool.Baro_card(tool.Baro_t("확인할 것"), tool.Baro_list([]string{
		tool.Baro_t("들어온 글에 출처가 붙어 있는가"),
		tool.Baro_t("문체가 문서의 다른 부분과 크게 다르지 않은가"),
		tool.Baro_t("같은 문단이 다른 문서에도 있는가 (중복 검사로 확인)"),
		tool.Baro_t("바깥 글을 그대로 옮긴 것이라면 저작권 문제이므로 격리 절차를 밟는다"),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("대량 복붙 경고"), body, baro_quality_menu())
}
