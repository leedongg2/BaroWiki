package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  13-3 기존 URL 연결 — /baro/import/url
//
//  친구 위키에서 쓰던 예전 주소로 들어와도 새 문서로 이어지게 한다.
// ============================================================

// baro_imp_map_row : 옛 주소 매핑 한 줄
type baro_imp_map_row struct {
	ID     string
	OldUrl string
	NewUrl string
	Kind   string
	Date   string
}

// baro_imp_map_kind_list : 매핑 종류
func baro_imp_map_kind_list() [][]string {
	return [][]string{
		{"url", tool.Baro_t("주소 매핑 (예전 주소 → 새 주소)")},
		{"title", tool.Baro_t("제목 변경 매핑 (옛 문서 제목 → 새 문서 제목)")},
		{"file", tool.Baro_t("파일 경로 매핑 (옛 파일 경로 → 새 파일 문서)")},
	}
}

// baro_imp_map_kind_name : 매핑 종류 이름
func baro_imp_map_kind_name(kind string) string {
	for _, row := range baro_imp_map_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// baro_imp_url_key : 옛 주소를 찾기 쉬운 형태로 다듬는다
func baro_imp_url_key(data string) string {
	data = strings.TrimSpace(data)
	if data == "" {
		return ""
	}

	// 앞에 붙은 사이트 주소는 떼어 낸다.
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(strings.ToLower(data), prefix) {
			rest := data[len(prefix):]
			if index := strings.Index(rest, "/"); index >= 0 {
				data = rest[index:]
			} else {
				data = "/"
			}

			break
		}
	}

	data = strings.TrimSpace(data)
	if data == "" {
		return ""
	}

	return data
}

// baro_imp_url_target : 새 주소를 실제로 옮겨 갈 수 있는 주소로 바꾼다
//
//	바깥 사이트로 튕겨 나가지 않도록 이 위키 안의 주소만 허용한다.
func baro_imp_url_target(new_url string) string {
	new_url = strings.TrimSpace(new_url)
	if new_url == "" {
		return ""
	}

	lower := strings.ToLower(new_url)

	// 스킴이 붙은 주소, 프로토콜 상대 주소는 받지 않는다. (열린 리다이렉트 방지)
	if strings.HasPrefix(lower, "//") {
		return ""
	}

	if strings.Contains(lower, "://") || strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
		return ""
	}

	if strings.HasPrefix(new_url, "/") {
		return new_url
	}

	return "/w/" + tool.Doc_url(tool.Namespace_normalize(new_url))
}

// baro_imp_url_doc : 새 주소가 가리키는 문서 제목 (문서 주소가 아니면 "")
func baro_imp_url_doc(new_url string) string {
	new_url = strings.TrimSpace(new_url)
	if new_url == "" {
		return ""
	}

	if strings.Contains(new_url, "://") {
		return ""
	}

	if strings.HasPrefix(new_url, "/w/") {
		title, err := url.PathUnescape(strings.TrimPrefix(new_url, "/w/"))
		if err != nil {
			title = strings.TrimPrefix(new_url, "/w/")
		}

		return tool.Namespace_normalize(title)
	}

	if strings.HasPrefix(new_url, "/") {
		return ""
	}

	return tool.Namespace_normalize(new_url)
}

// baro_imp_map_find : 옛 주소로 매핑 찾기
func baro_imp_map_find(db *sql.DB, old_url string) (baro_imp_map_row, bool) {
	row := baro_imp_map_row{}

	key := baro_imp_url_key(old_url)
	if key == "" {
		return row, false
	}

	candidates := []string{key}
	if strings.HasPrefix(key, "/") {
		candidates = append(candidates, strings.TrimPrefix(key, "/"))
	} else {
		candidates = append(candidates, "/"+key)
	}

	if unescaped, err := url.PathUnescape(key); err == nil && unescaped != key {
		candidates = append(candidates, unescaped)
	}

	for _, one := range candidates {
		found := tool.QueryRow_DB(
			db,
			"select id, old_url, new_url, kind, date from baro_url_map where old_url = ?",
			[]any{&row.ID, &row.OldUrl, &row.NewUrl, &row.Kind, &row.Date},
			one,
		)

		if found {
			return row, true
		}
	}

	return row, false
}

// ------------------------------------------------------------
//  /baro/import/url : 매핑 목록 · 여러 줄 등록
// ------------------------------------------------------------

// View_baro_imp_url : 옛 주소 연결 관리
func View_baro_imp_url(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/import/url"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "import") {
		return tool.Baro_deny(db, config, tool.Baro_t("옛 주소 연결은 시스템 관리자 역할이 있어야 씁니다."))
	}

	notice := ""
	notice_kind := "ok"

	if values != nil {
		if values.Get("do") == "delete" {
			target := strings.TrimSpace(values.Get("id"))
			if target != "" {
				old_url := ""
				tool.QueryRow_DB(db, "select old_url from baro_url_map where id = ?", []any{&old_url}, target)

				tool.Exec_DB(db, "delete from baro_url_map where id = ?", target)
				tool.Baro_public_log(db, "url_map_delete", config.IP, old_url, "", "옛 주소 매핑 삭제")
				tool.Baro_audit(db, config, "url_map_delete", "baro_url_map#"+target, old_url, "", "")
			}

			return tool.Get_redirect("/baro/import/url")
		}

		added, skipped, bad := baro_imp_url_add(db, config, values.Get("kind"), values.Get("lines"))

		notice = tool.Baro_t("새 매핑 ") + tool.Baro_itoa(added) + tool.Baro_t("개를 등록했습니다.")
		if skipped > 0 {
			notice += tool.Baro_t(" 이미 있던 ") + tool.Baro_itoa(skipped) + tool.Baro_t("개는 새 주소로 고쳤습니다.")
		}

		if len(bad) > 0 {
			notice_kind = "warn"
			notice += " 읽지 못한 줄 " + tool.Baro_itoa(len(bad)) + "개 : " + strings.Join(bad, " / ")
		}

		if added == 0 && skipped == 0 && len(bad) == 0 {
			notice_kind = "warn"
			notice = tool.Baro_t("등록할 내용이 없습니다. \"옛주소 새주소\" 형식으로 한 줄에 하나씩 적으세요.")
		}
	}

	page_int := tool.Str_to_int(page)
	if page_int < 1 {
		page_int = 1
	}

	max_count := tool.Baro_page_size
	list := []baro_imp_map_row{}

	func() {
		rows := tool.Query_DB(
			db,
			"select id, old_url, new_url, kind, date from baro_url_map order by id + 0 desc limit ? offset ?",
			max_count, (page_int-1)*max_count,
		)
		defer rows.Close()

		for rows.Next() {
			row := baro_imp_map_row{}
			if err := rows.Scan(&row.ID, &row.OldUrl, &row.NewUrl, &row.Kind, &row.Date); err != nil {
				continue
			}

			list = append(list, row)
		}
	}()

	table_rows := [][]string{}
	for _, row := range list {
		target := baro_imp_url_target(row.NewUrl)

		new_html := tool.HTML_escape(row.NewUrl)
		if target != "" {
			new_html = `<a href="` + tool.HTML_escape(target) + `">` + tool.HTML_escape(row.NewUrl) + `</a>`
		}

		state := tool.Baro_badge(tool.Baro_t("연결됨"), "ok")
		if target == "" {
			state = tool.Baro_badge(tool.Baro_t("주소 형식 오류"), "danger")
		} else if title := baro_imp_url_doc(row.NewUrl); title != "" && !baro_imp_doc_exist(db, title) {
			state = tool.Baro_badge(tool.Baro_t("대상 문서 없음"), "warn")
		}

		delete_form := tool.Baro_form("/baro/import/url",
			tool.Baro_hidden("do", "delete")+tool.Baro_hidden("id", row.ID)+tool.Baro_submit("삭제", "danger"))

		table_rows = append(table_rows, []string{
			tool.HTML_escape("#" + row.ID),
			tool.HTML_escape(baro_imp_map_kind_name(row.Kind)),
			`<a href="/baro/import/go?u=` + tool.Url_parser(row.OldUrl) + `">` + tool.HTML_escape(tool.Baro_trunc(row.OldUrl, 60)) + `</a>`,
			new_html,
			state,
			tool.Baro_time_cell(row.Date),
			delete_form,
		})
	}

	total := 0
	tool.QueryRow_DB(db, "select count(*) from baro_url_map", []any{&total})

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/import", "자료 이전"}, {"", tool.Baro_t("옛 주소 연결")}})
	body += baro_imp_tabs("/baro/import/url")

	if notice != "" {
		body += tool.Baro_alert(notice_kind, tool.HTML_escape(notice))
	}

	body += tool.Baro_alert("info",
		tool.Baro_t("예전 위키 주소로 들어온 사람을 새 문서로 넘겨 줍니다. 실제 이동 주소는 ")+
			tool.Baro_t(`<code>/baro/import/go?u=옛주소</code> 입니다.`))

	form := tool.Baro_select("kind", tool.Baro_t("매핑 종류"), baro_imp_map_kind_list(), "url", "")
	form += tool.Baro_textarea("lines", tool.Baro_t("한 줄에 하나씩 (옛주소 새주소)"), "", 8,
		"/wiki/옛문서 새문서\n/old/page.html /w/새문서\nfile/사진.png 파일:사진.png",
		tool.Baro_t("옛 주소와 새 주소를 빈칸이나 탭으로 나눠 적습니다. 이미 있는 옛 주소는 새 주소로 고쳐집니다."))
	form += tool.Baro_submit(tool.Baro_t("매핑 등록"), "primary")

	body += tool.Baro_card(tool.Baro_t("여러 줄 한 번에 등록"), tool.Baro_form("/baro/import/url", form))

	table := tool.Baro_table(
		[]string{"번호", "종류", tool.Baro_t("옛 주소"), tool.Baro_t("새 주소"), "상태", tool.Baro_t("등록 때"), "관리"},
		table_rows,
		tool.Baro_t("등록된 매핑이 없습니다."),
	)

	if len(table_rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("등록된 매핑이 없습니다. 위 칸에 \"옛주소 새주소\" 를 적어 등록하세요."))
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("등록된 매핑"),
		tool.Baro_t("모두 ")+tool.Baro_comma(total)+tool.Baro_t("개"),
		table+tool.Baro_paging(page_int, len(list), max_count, "/baro/import/url/list/{}"),
		tool.Baro_button("/baro/import/url/broken", tool.Baro_t("깨진 링크 보고서"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("옛 주소 연결"), body, baro_imp_menu())
}

// baro_imp_url_add : 여러 줄 매핑 등록 (새로 넣은 수, 고친 수, 읽지 못한 줄)
func baro_imp_url_add(db *sql.DB, config tool.Config, kind string, lines string) (int, int, []string) {
	if kind == "" {
		kind = "url"
	}

	added, updated := 0, 0
	bad := []string{}

	next_id := tool.Str_to_int(tool.Baro_next_id(db, "baro_url_map"))

	for _, line := range tool.Baro_split_lines(lines) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.ReplaceAll(line, "\t", " ")

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			bad = append(bad, tool.Baro_trunc(line, 30))

			continue
		}

		old_url := baro_imp_url_key(parts[0])
		new_url := strings.TrimSpace(parts[1])

		if old_url == "" || new_url == "" {
			bad = append(bad, tool.Baro_trunc(line, 30))

			continue
		}

		if baro_imp_url_target(new_url) == "" {
			bad = append(bad, tool.Baro_trunc(line, 30)+tool.Baro_t(" (새 주소가 이 위키 안이 아님)"))

			continue
		}

		exist := ""
		tool.QueryRow_DB(db, "select id from baro_url_map where old_url = ?", []any{&exist}, old_url)

		if exist != "" {
			tool.Exec_DB(db, "update baro_url_map set new_url = ?, kind = ?, date = ? where id = ?",
				new_url, kind, tool.Baro_now(), exist)
			updated++

			continue
		}

		tool.Exec_DB(
			db,
			"insert into baro_url_map (id, old_url, new_url, kind, date) values (?, ?, ?, ?, ?)",
			tool.Baro_itoa(next_id), old_url, new_url, kind, tool.Baro_now(),
		)
		next_id++
		added++
	}

	if added > 0 || updated > 0 {
		tool.Baro_public_log(db, "url_map_add", config.IP, baro_imp_map_kind_name(kind), "",
			"새 매핑 "+tool.Baro_itoa(added)+"개 / 고친 매핑 "+tool.Baro_itoa(updated)+"개")
		tool.Baro_audit(db, config, "url_map_add", "baro_url_map", "",
			"added="+tool.Baro_itoa(added)+" updated="+tool.Baro_itoa(updated), kind)
	}

	return added, updated, bad
}

// ------------------------------------------------------------
//  /baro/import/url/broken : 깨진 링크 보고서
// ------------------------------------------------------------

// View_baro_imp_url_broken : 새 주소가 가리키는 문서가 실제로 없는 매핑 목록
func View_baro_imp_url_broken(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/import/url/broken"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "import") {
		return tool.Baro_deny(db, config, tool.Baro_t("옛 주소 연결은 시스템 관리자 역할이 있어야 씁니다."))
	}

	page_int := tool.Str_to_int(page)
	if page_int < 1 {
		page_int = 1
	}

	max_count := tool.Baro_page_size

	all := []baro_imp_map_row{}

	func() {
		rows := tool.Query_DB(db, "select id, old_url, new_url, kind, date from baro_url_map order by id + 0 asc")
		defer rows.Close()

		for rows.Next() {
			row := baro_imp_map_row{}
			if err := rows.Scan(&row.ID, &row.OldUrl, &row.NewUrl, &row.Kind, &row.Date); err != nil {
				continue
			}

			all = append(all, row)
		}
	}()

	broken := []baro_imp_map_row{}
	checked := 0

	for _, row := range all {
		checked++

		target := baro_imp_url_target(row.NewUrl)
		if target == "" {
			broken = append(broken, row)

			continue
		}

		title := baro_imp_url_doc(row.NewUrl)
		if title == "" {
			continue
		}

		if !baro_imp_doc_exist(db, title) {
			broken = append(broken, row)
		}
	}

	start := (page_int - 1) * max_count
	end := start + max_count

	if start > len(broken) {
		start = len(broken)
	}

	if end > len(broken) {
		end = len(broken)
	}

	table_rows := [][]string{}
	for _, row := range broken[start:end] {
		title := baro_imp_url_doc(row.NewUrl)

		reason := tool.Baro_t("새 주소가 이 위키 안의 주소가 아닙니다.")
		fix := `<span class="bw_muted">-</span>`

		if title != "" {
			reason = tool.Baro_t("가리키는 문서가 아직 없습니다.")
			fix = tool.Baro_button("/w/" + tool.Doc_url(title)+"?edit=1", "문서 만들기", "")
		}

		table_rows = append(table_rows, []string{
			tool.HTML_escape("#" + row.ID),
			tool.HTML_escape(tool.Baro_trunc(row.OldUrl, 60)),
			tool.HTML_escape(tool.Baro_trunc(row.NewUrl, 60)),
			tool.HTML_escape(reason),
			fix,
		})
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/import", "자료 이전"}, {"/baro/import/url", tool.Baro_t("옛 주소 연결")}, {"", tool.Baro_t("깨진 링크")}})
	body += baro_imp_tabs("/baro/import/url")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("검사한 매핑"), tool.Baro_comma(checked), tool.Baro_t("등록된 전체")},
		{tool.Baro_t("깨진 매핑"), tool.Baro_comma(len(broken)), tool.Baro_t("대상이 없는 것")},
		{tool.Baro_t("정상"), tool.Baro_comma(checked - len(broken)), tool.Baro_t("잘 이어지는 것")},
	})

	table := tool.Baro_table(
		[]string{"번호", tool.Baro_t("옛 주소"), tool.Baro_t("새 주소"), tool.Baro_t("문제"), "고치기"},
		table_rows,
		tool.Baro_t("깨진 매핑이 없습니다."),
	)

	if len(table_rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("깨진 매핑이 없습니다. 등록된 옛 주소가 모두 살아 있는 문서를 가리킵니다."))
	}

	body += tool.Baro_card(tool.Baro_t("깨진 링크"),
		table+tool.Baro_paging(page_int, len(table_rows), max_count, "/baro/import/url/broken/{}"))

	return tool.Baro_page(db, config, tool.Baro_t("깨진 링크 보고서"), body, baro_imp_menu())
}

// ------------------------------------------------------------
//  /baro/import/go : 실제 리다이렉트
// ------------------------------------------------------------

// View_baro_imp_go : 옛 주소로 들어오면 새 주소로 옮겨 준다 (누구나 쓸 수 있다)
func View_baro_imp_go(config tool.Config, old_url string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	key := baro_imp_url_key(old_url)

	row, found := baro_imp_map_find(db, key)
	if found {
		target := baro_imp_url_target(row.NewUrl)
		if target != "" {
			tool.Baro_metric_bump(db, "url_map_hit", tool.Baro_today(), 1)

			return tool.Get_redirect(target)
		}
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("옛 주소")}})

	if key == "" {
		body += tool.Baro_alert("warn", tool.Baro_t("옮겨 갈 옛 주소가 지정되지 않았습니다."))
	} else {
		body += tool.Baro_alert("warn",
			tool.Baro_t("옛 주소 <b>")+tool.HTML_escape(tool.Baro_trunc(key, 200))+tool.Baro_t("</b> 에 연결된 새 주소가 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("이렇게 해 보세요"),
		tool.Baro_list([]string{
			tool.Baro_t(`제목으로 <a href="/search">검색</a>해 보세요.`),
			tool.Baro_t(`문서 이름을 알고 있다면 주소창에 <code>/w/문서이름</code> 을 넣어 보세요.`),
			tool.Baro_t(`관리자라면 <a href="/baro/import/url">옛 주소 연결</a>에서 매핑을 등록할 수 있습니다.`),
		}))

	return tool.Baro_page(db, config, tool.Baro_t("옛 주소"), body, baro_imp_menu())
}
