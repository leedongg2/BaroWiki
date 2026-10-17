package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  3-15 동명이인 · 별칭
//
//  인물 위키에서는 한 사람이 여러 이름으로 불린다.
//  한자명 · 개명 전 이름 · 호 · 외국어 표기 · 잘못 알려진 이름을
//  모두 baro_doc_alias 에 모아 검색이 한 문서로 이어지게 한다.
// ============================================================

// baro_find_alias_kind_options : 별칭 종류 선택 상자 항목
func baro_find_alias_kind_options(head string) [][]string {
	out := [][]string{}
	if head != "" {
		out = append(out, []string{"", head})
	}

	out = append(out, baro_find_alias_kind_list()...)

	return out
}

// baro_find_alias_add : 별칭 등록 (중복이면 건너뛴다)
func baro_find_alias_add(db *sql.DB, config tool.Config, title string, alias string, kind string, note string) string {
	title = tool.Namespace_normalize(strings.TrimSpace(title))
	alias = strings.TrimSpace(alias)

	if title == "" || alias == "" {
		return tool.Baro_t("문서 이름과 별칭을 모두 적어 주세요.")
	}
	if !baro_find_alias_kind_ok(kind) {
		return tool.Baro_t("별칭 종류를 골라 주세요.")
	}
	if tool.Get_len(alias) > 100 {
		return tool.Baro_t("별칭이 너무 깁니다. (100자 이하)")
	}
	if !baro_find_doc_exists(db, title) {
		return "그런 문서가 없습니다 : " + title
	}

	found := ""
	if tool.QueryRow_DB(
		db,
		"select id from baro_doc_alias where title = ? and alias = ? and kind = ?",
		[]any{&found}, title, alias, kind,
	) {
		return tool.Baro_t("이미 등록된 별칭입니다.")
	}

	id := tool.Baro_next_id(db, "baro_doc_alias")
	tool.Exec_DB(
		db,
		"insert into baro_doc_alias (id, title, alias, kind, note, date) values (?, ?, ?, ?, ?, ?)",
		id, title, alias, kind, tool.Baro_trunc(note, 200), tool.Baro_now(),
	)

	// 한자명은 문서 메타에도 함께 넣어 두면 검색·동명이인 안내에서 바로 쓸 수 있다.
	if kind == "hanja" && tool.Baro_doc_meta_get(db, title, "hanja") == "" {
		tool.Baro_doc_meta_set(db, title, "hanja", alias)
	}

	tool.Baro_public_log(db, "alias_add", config.IP, title, baro_find_alias_kind_name(kind), alias)
	tool.Baro_audit(db, config, "alias_add", title, "", kind+" / "+alias, note)
	tool.Baro_contrib_add(db, config.IP, "category", title, "", "", "별칭 등록 : "+alias)

	return ""
}

// baro_find_alias_del : 별칭 삭제
func baro_find_alias_del(db *sql.DB, config tool.Config, id string) string {
	if strings.TrimSpace(id) == "" {
		return tool.Baro_t("지울 별칭을 고르지 못했습니다.")
	}

	title, alias, kind := "", "", ""
	if !tool.QueryRow_DB(
		db,
		"select title, alias, kind from baro_doc_alias where id = ?",
		[]any{&title, &alias, &kind}, id,
	) {
		return tool.Baro_t("이미 지워진 별칭입니다.")
	}

	tool.Exec_DB(db, "delete from baro_doc_alias where id = ?", id)
	tool.Baro_public_log(db, "alias_del", config.IP, title, baro_find_alias_kind_name(kind), alias)
	tool.Baro_audit(db, config, "alias_del", title, kind+" / "+alias, "", "")

	return ""
}

// View_baro_find_alias : 별칭 목록과 등록·삭제 (/baro/doc/alias)
func View_baro_find_alias(config tool.Config, page string, query url.Values, post url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if post != nil {
		if wall, need := tool.Baro_login_wall(db, config, "/baro/doc/alias"); need {
			return wall
		}
		if tool.Baro_read_only(db) {
			return tool.Baro_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db))
		}

		action := baro_find_val(post, "action")
		message := ""

		switch action {
		case "add":
			if !tool.Baro_rate_ok(db, "baro_find_alias", config.IP, 30, 3600) {
				message = tool.Baro_t("짧은 시간에 너무 많이 등록했습니다. 잠시 뒤에 다시 시도해 주세요.")
			} else {
				message = baro_find_alias_add(db, config,
					baro_find_val(post, "title"), baro_find_val(post, "alias"),
					baro_find_val(post, "kind"), baro_find_val(post, "note"))
			}
		case "del":
			if !tool.Baro_can(db, config.IP, "review") {
				return tool.Baro_deny(db, config, tool.Baro_t("별칭을 지우려면 문서 검토 권한이 필요합니다."))
			}

			message = baro_find_alias_del(db, config, baro_find_val(post, "id"))
		}

		next := "/baro/doc/alias"
		if back := baro_find_val(post, "back"); strings.HasPrefix(back, "/baro/doc/alias") {
			next = back
		}
		if message != "" {
			next += "?msg=" + tool.Url_parser(message)
		}

		return tool.Get_redirect(next)
	}

	keyword := baro_find_val(query, "q")
	kind := baro_find_val(query, "kind")
	message := baro_find_val(query, "msg")

	page_int := baro_find_page_int(page)
	if raw_page := baro_find_val(query, "page"); raw_page != "" {
		page_int = baro_find_page_int(raw_page)
	}

	offset := (page_int - 1) * tool.Baro_page_size

	sql_text := "select id, title, alias, kind, note, date from baro_doc_alias where 1 = 1"
	args := []any{}

	if kind != "" {
		sql_text += " and kind = ?"
		args = append(args, kind)
	}
	if keyword != "" {
		sql_text += " and (lower(alias) like ? or lower(title) like ?)"
		args = append(args, "%"+strings.ToLower(keyword)+"%", "%"+strings.ToLower(keyword)+"%")
	}

	sql_text += " order by id + 0 desc limit ?, ?"
	args = append(args, offset, tool.Baro_page_size)

	rows := tool.Query_DB(db, sql_text, args...)
	defer rows.Close()

	list := [][]string{}
	for rows.Next() {
		id, title, alias, alias_kind, note, date := "", "", "", "", "", ""
		if err := rows.Scan(&id, &title, &alias, &alias_kind, &note, &date); err != nil {
			continue
		}

		list = append(list, []string{id, title, alias, alias_kind, note, date})
	}

	total := 0
	tool.QueryRow_DB(db, "select count(*) from baro_doc_alias", []any{&total})

	can_del := tool.Baro_can(db, config.IP, "review")

	table_rows := [][]string{}
	for _, row := range list {
		manage := tool.Baro_button("/baro/doc/alias/"+tool.Url_parser(row[1]), "문서 관리", "")
		if can_del {
			manage += " " + tool.Baro_form("/baro/doc/alias",
				tool.Baro_hidden("action", "del")+tool.Baro_hidden("id", row[0])+
					tool.Baro_hidden("back", "/baro/doc/alias")+tool.Baro_submit("삭제", "danger"))
		}

		table_rows = append(table_rows, []string{
			tool.HTML_escape(row[2]),
			tool.Baro_badge(baro_find_alias_kind_name(row[3]), "info"),
			tool.Baro_doc_link(row[1]),
			tool.HTML_escape(tool.Baro_trunc(row[4], 40)),
			tool.Baro_time_cell(row[5]),
			manage,
		})
	}

	page_url := url.Values{}
	page_url.Set("q", keyword)
	page_url.Set("kind", kind)

	list_html := tool.Baro_table(
		[]string{"별칭", "종류", tool.Baro_t("연결된 문서"), tool.Baro_t("메모"), tool.Baro_t("등록"), "관리"},
		table_rows,
		"",
	)
	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("등록된 별칭이 없습니다. 한자명이나 개명 전 이름부터 채워 보세요."))
	}
	list_html += tool.Baro_paging(page_int, len(table_rows), tool.Baro_page_size, "/baro/doc/alias?"+page_url.Encode()+"&page={}")

	filter := tool.Baro_input("q", tool.Baro_t("검색"), keyword, tool.Baro_t("별칭 또는 문서 이름"), "")
	filter += tool.Baro_select("kind", "종류", baro_find_alias_kind_options(tool.Baro_t("종류 전체")), kind, "")
	filter += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("찾기"), "primary") + `</div>`

	body := baro_find_tabs("/baro/doc/alias")
	body += tool.Baro_hero(tool.Baro_t("별칭 · 동명이인"), tool.Baro_t("한 사람의 여러 이름을 한 문서로 모읍니다."),
		tool.Baro_button("/baro/doc/dab", tool.Baro_t("동명이인 문서 만들기 도우미"), "primary")+" "+
			tool.Baro_button("/baro/search", tool.Baro_t("검색으로 확인"), ""))

	if message != "" {
		kind_alert := "warn"
		if strings.Contains(message, "이미") {
			kind_alert = "info"
		}
		body += tool.Baro_alert(kind_alert, tool.HTML_escape(message))
	}

	counts := [][]string{}
	for _, row := range baro_find_alias_kind_list() {
		count := 0
		tool.QueryRow_DB(db, "select count(*) from baro_doc_alias where kind = ?", []any{&count}, row[0])
		counts = append(counts, []string{row[1], tool.Baro_comma(count), ""})
	}
	body += tool.Baro_stat_row(append([][]string{{tool.Baro_t("전체 별칭"), tool.Baro_comma(total), ""}}, counts...))

	body += tool.Baro_card(tool.Baro_t("찾기"), `<form class="bw_form" method="get" action="/baro/doc/alias">`+filter+`</form>`)

	add_form := tool.Baro_hidden("action", "add")
	add_form += tool.Baro_input("title", "문서 이름", baro_find_val(query, "doc"), "예: 김성범", "이미 있는 문서만 연결할 수 있습니다.")
	add_form += tool.Baro_input("alias", "별칭", keyword, tool.Baro_t("예: 金成範"), tool.Baro_t("한자명·개명 전 이름·호·외국어 표기·잘못 알려진 이름·검색 별칭"))
	add_form += tool.Baro_select("kind", "종류", baro_find_alias_kind_options(""), "hanja", "")
	add_form += tool.Baro_input("note", tool.Baro_t("메모"), "", tool.Baro_t("출처나 근거"), tool.Baro_t("어디서 확인한 이름인지 적어 두면 나중에 다투지 않습니다."))
	add_form += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("별칭 등록"), "primary") + `</div>`

	if tool.Baro_is_user(config.IP) {
		body += tool.Baro_card(tool.Baro_t("별칭 등록"), tool.Baro_form("/baro/doc/alias", add_form))
	} else {
		body += tool.Baro_card(tool.Baro_t("별칭 등록"), tool.Baro_alert("info",
			tool.Baro_t("별칭 등록은 로그인한 기여자만 할 수 있습니다. ")+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary")))
	}

	body += tool.Baro_card(tool.Baro_t("등록된 별칭"), list_html)

	body += tool.Baro_card(tool.Baro_t("별칭 종류 안내"), tool.Baro_dl([][]string{
		{tool.Baro_t("한자명 (hanja)"), tool.Baro_t("金成範 처럼 한자로 적힌 이름. 옛 기록·신문·족보에서 자주 쓰입니다.")},
		{tool.Baro_t("개명 전 이름 (former)"), tool.Baro_t("법적으로 바꾸기 전의 이름.")},
		{tool.Baro_t("호·아명·이명 (ho)"), tool.Baro_t("호, 어릴 적 이름, 필명, 예명.")},
		{tool.Baro_t("외국어 표기 (foreign)"), tool.Baro_t("Kim Seong-beom 처럼 로마자·일본어 등 다른 문자 표기.")},
		{tool.Baro_t("잘못 알려진 이름 (wrong)"), tool.Baro_t("널리 잘못 쓰이는 이름. 검색은 되게 하되 본문에서는 바로잡습니다.")},
		{tool.Baro_t("검색 별칭 (search)"), tool.Baro_t("위 어디에도 들어가지 않지만 사람들이 실제로 검색하는 말.")},
	}))

	return tool.Baro_page(db, config, tool.Baro_t("별칭 · 동명이인"), body, baro_find_menu())
}

// View_baro_find_alias_doc : 문서별 별칭·인물 정보 관리 (/baro/doc/alias/<문서>)
func View_baro_find_alias_doc(config tool.Config, doc_name string, post url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	doc_name = tool.Namespace_normalize(strings.TrimSpace(doc_name))

	if doc_name == "" {
		return tool.Get_redirect("/baro/doc/alias")
	}

	if post != nil {
		if wall, need := tool.Baro_login_wall(db, config, "/baro/doc/alias"); need {
			return wall
		}
		if tool.Baro_read_only(db) {
			return tool.Baro_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db))
		}

		back := "/baro/doc/alias/" + tool.Url_parser(doc_name)
		message := ""

		switch baro_find_val(post, "action") {
		case "add":
			message = baro_find_alias_add(db, config, doc_name,
				baro_find_val(post, "alias"), baro_find_val(post, "kind"), baro_find_val(post, "note"))
		case "del":
			if !tool.Baro_can(db, config.IP, "review") {
				return tool.Baro_deny(db, config, tool.Baro_t("별칭을 지우려면 문서 검토 권한이 필요합니다."))
			}

			message = baro_find_alias_del(db, config, baro_find_val(post, "id"))
		case "meta":
			message = baro_find_meta_save(db, config, doc_name, post)
		}

		if message != "" {
			back += "?msg=" + tool.Url_parser(message)
		}

		return tool.Get_redirect(back)
	}

	body := baro_find_tabs("/baro/doc/alias")
	body += tool.Baro_crumb([][]string{
		{"/baro/search", tool.Baro_t("검색")},
		{"/baro/doc/alias", tool.Baro_t("별칭 · 동명이인")},
		{"", tool.Namespace_display(doc_name)},
	})

	if !baro_find_doc_exists(db, doc_name) {
		body += tool.Baro_alert("warn", `<b>`+tool.HTML_escape(tool.Namespace_display(doc_name))+tool.Baro_t(`</b> 문서가 아직 없습니다. 문서를 만든 뒤에 별칭을 등록할 수 있습니다.`))
		body += tool.Baro_card(tool.Baro_t("무엇을 할 수 있나요"),
			tool.Baro_button("/edit/"+tool.Url_parser(doc_name), "이 문서 만들기", "primary")+" "+
				tool.Baro_button("/baro/search?q="+tool.Url_parser(doc_name), "비슷한 문서 찾기", "")+" "+
				tool.Baro_button("/baro/doc/alias", tool.Baro_t("별칭 목록으로"), ""))

		return tool.Baro_page(db, config, tool.Baro_t("별칭 : ")+tool.Namespace_display(doc_name), body, baro_find_menu())
	}

	alias_list := baro_find_alias_of(db, doc_name)
	meta := baro_find_meta_all(db, doc_name)
	can_del := tool.Baro_can(db, config.IP, "review")

	body += tool.Baro_hero(tool.Namespace_display(doc_name), baro_find_person_line(meta),
		tool.Baro_button("/w/" + tool.Doc_url(doc_name), "문서 보기", "primary")+" "+
			tool.Baro_button("/baro/doc/seo/"+tool.Url_parser(doc_name), "외부 검색 설정", "")+" "+
			tool.Baro_button("/baro/doc/graph/"+tool.Url_parser(doc_name), "지식 그래프", ""))

	rows := [][]string{}
	for _, row := range alias_list {
		manage := `<span class="bw_muted">-</span>`
		if can_del {
			manage = tool.Baro_form("/baro/doc/alias/"+tool.Url_parser(doc_name),
				tool.Baro_hidden("action", "del")+tool.Baro_hidden("id", row[0])+tool.Baro_submit("삭제", "danger"))
		}

		rows = append(rows, []string{
			tool.HTML_escape(row[1]),
			tool.Baro_badge(baro_find_alias_kind_name(row[2]), "info"),
			tool.HTML_escape(row[3]),
			tool.Baro_time_cell(row[4]),
			manage,
		})
	}

	list_html := tool.Baro_table([]string{"별칭", "종류", tool.Baro_t("메모"), tool.Baro_t("등록"), "관리"}, rows, "")
	if len(rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("이 문서에 등록된 별칭이 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("등록된 별칭"), list_html)

	if tool.Baro_is_user(config.IP) {
		add_form := tool.Baro_hidden("action", "add")
		add_form += tool.Baro_input("alias", "별칭", "", tool.Baro_t("예: 金成範"), "")
		add_form += tool.Baro_select("kind", "종류", baro_find_alias_kind_options(""), "hanja", "")
		add_form += tool.Baro_input("note", tool.Baro_t("메모"), "", tool.Baro_t("출처나 근거"), "")
		add_form += `<div class="bw_f">` + tool.Baro_submit("별칭 추가", "primary") + `</div>`

		body += tool.Baro_card("별칭 추가", tool.Baro_form("/baro/doc/alias/"+tool.Url_parser(doc_name), add_form))
		body += tool.Baro_card("인물 정보", baro_find_meta_form(db, doc_name, meta, "/baro/doc/alias/"+tool.Url_parser(doc_name)))
	} else {
		body += tool.Baro_card("별칭 추가", tool.Baro_alert("info",
			tool.Baro_t("별칭 등록과 인물 정보 편집은 로그인한 기여자만 할 수 있습니다. ")+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary")))
		body += tool.Baro_card("인물 정보", tool.Baro_dl([][]string{
			{tool.Baro_t("출생연도"), tool.HTML_escape(meta["birth_year"])},
			{tool.Baro_t("사망연도"), tool.HTML_escape(meta["death_year"])},
			{tool.Baro_t("직업"), tool.HTML_escape(meta["job"])},
			{"한자명", tool.HTML_escape(meta["hanja"])},
			{"분야", tool.HTML_escape(tool.Baro_field_name(db, meta["field"]))},
		}))
	}

	// 같은 이름을 가진 다른 문서 안내
	same := baro_find_same_name(db, tool.Namespace_display(doc_name), doc_name)
	same_rows := [][]string{}
	for _, title := range same {
		other := baro_find_meta_all(db, title)
		same_rows = append(same_rows, []string{
			tool.Baro_doc_link(title),
			tool.HTML_escape(baro_find_person_line(other)),
			tool.Baro_button("/baro/doc/alias/"+tool.Url_parser(title), "별칭 관리", ""),
		})
	}

	body += tool.Baro_card_ex(tool.Baro_t("같은 이름을 가진 다른 문서"), tool.Baro_t("동명이인일 수 있습니다"),
		tool.Baro_table([]string{"문서", "인물 정보", "관리"}, same_rows, tool.Baro_t("같은 이름의 다른 문서를 찾지 못했습니다.")),
		tool.Baro_button("/baro/doc/dab?name="+tool.Url_parser(baro_find_base_name(tool.Namespace_display(doc_name))), "동명이인 안내 문서 만들기", ""))

	return tool.Baro_page(db, config, tool.Baro_t("별칭 : ")+tool.Namespace_display(doc_name), body, baro_find_menu())
}

// baro_find_meta_form : 인물 정보(출생연도·직업·한자명) 편집 폼
func baro_find_meta_form(db *sql.DB, doc_name string, meta map[string]string, action string) string {
	form := tool.Baro_hidden("action", "meta")
	form += tool.Baro_input("birth_year", tool.Baro_t("출생연도"), meta["birth_year"], tool.Baro_t("예: 1890"), tool.Baro_t("동명이인을 가르는 가장 중요한 값입니다."))
	form += tool.Baro_input("death_year", tool.Baro_t("사망연도"), meta["death_year"], tool.Baro_t("예: 1945"), "")
	form += tool.Baro_input("job", tool.Baro_t("직업"), meta["job"], tool.Baro_t("예: 독립운동가"), tool.Baro_t("'김성범 독립운동가' 같은 검색어를 이 값으로 맞춥니다."))
	form += tool.Baro_input("hanja", "한자명", meta["hanja"], tool.Baro_t("예: 金成範"), tool.Baro_t("한자명은 별칭(hanja)에도 함께 등록됩니다."))
	form += tool.Baro_select("field", "분야", baro_find_field_options(db), meta["field"], tool.Baro_t("발견·추천 화면에서 쓰입니다."))
	form += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("인물 정보 저장"), "primary") + `</div>`

	_ = doc_name

	return tool.Baro_form(action, form)
}

// baro_find_meta_save : 인물 정보 저장
func baro_find_meta_save(db *sql.DB, config tool.Config, doc_name string, post url.Values) string {
	if !baro_find_doc_exists(db, doc_name) {
		return tool.Baro_t("그런 문서가 없습니다.")
	}

	before := tool.Baro_json_enc(baro_find_meta_all(db, doc_name))

	birth := baro_find_val(post, "birth_year")
	death := baro_find_val(post, "death_year")
	job := baro_find_val(post, "job")
	hanja := baro_find_val(post, "hanja")
	field := baro_find_val(post, "field")

	for _, pair := range [][]string{{"birth_year", birth}, {"death_year", death}} {
		if pair[1] != "" && (len(pair[1]) != 4 || strings.Trim(pair[1], "0123456789") != "") {
			return tool.Baro_t("연도는 네 자리 숫자로 적어 주세요.")
		}
	}

	tool.Baro_doc_meta_set(db, doc_name, "birth_year", birth)
	tool.Baro_doc_meta_set(db, doc_name, "death_year", death)
	tool.Baro_doc_meta_set(db, doc_name, "job", tool.Baro_trunc(job, 40))
	tool.Baro_doc_meta_set(db, doc_name, "hanja", tool.Baro_trunc(hanja, 40))
	if field != "" {
		tool.Baro_doc_meta_set(db, doc_name, "field", field)
	}

	// 한자명이 별칭에 없으면 함께 등록해 검색이 이어지게 한다.
	if hanja != "" {
		found := ""
		if !tool.QueryRow_DB(
			db,
			"select id from baro_doc_alias where title = ? and alias = ? and kind = 'hanja'",
			[]any{&found}, doc_name, hanja,
		) {
			tool.Exec_DB(
				db,
				"insert into baro_doc_alias (id, title, alias, kind, note, date) values (?, ?, ?, 'hanja', '인물 정보에서 자동 등록', ?)",
				tool.Baro_next_id(db, "baro_doc_alias"), doc_name, hanja, tool.Baro_now(),
			)
		}
	}

	after := tool.Baro_json_enc(baro_find_meta_all(db, doc_name))
	tool.Baro_audit(db, config, "doc_meta_edit", doc_name, before, after, "인물 정보 편집")
	tool.Baro_public_log(db, "doc_meta_edit", config.IP, doc_name, "인물 정보 편집", baro_find_person_line(baro_find_meta_all(db, doc_name)))
	tool.Baro_contrib_add(db, config.IP, "fix", doc_name, "", "", "인물 정보 편집")

	return ""
}

// baro_find_base_name : "김성범(독립운동가)" 에서 "김성범" 만 뽑는다
func baro_find_base_name(title string) string {
	for _, mark := range []string{"(", "（"} {
		if index := strings.Index(title, mark); index > 0 {
			title = title[:index]
		}
	}

	return strings.TrimSpace(title)
}

// baro_find_same_name : 같은 이름을 가진 다른 문서 후보
func baro_find_same_name(db *sql.DB, display_name string, except string) []string {
	base := baro_find_base_name(display_name)
	if base == "" {
		return []string{}
	}

	hidden := baro_find_hidden_set(db)
	seen := map[string]bool{except: true}
	out := []string{}

	add := func(title string) {
		if title == "" || seen[title] || baro_find_excluded(title, hidden) {
			return
		}

		seen[title] = true
		out = append(out, title)
	}

	rows := tool.Query_DB(
		db,
		"select title from data where title = ? or title like ? or title like ? limit 200",
		base, base+"(%", base+" (%",
	)
	defer rows.Close()

	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		add(title)
	}

	alias_rows := tool.Query_DB(db, "select title from baro_doc_alias where alias = ? limit 200", base)
	defer alias_rows.Close()

	for alias_rows.Next() {
		title := ""
		if err := alias_rows.Scan(&title); err != nil {
			continue
		}

		add(title)
	}

	sort.Strings(out)

	return out
}

// View_baro_find_dab : 동명이인 문서 만들기 도우미 (/baro/doc/dab)
func View_baro_find_dab(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	name := baro_find_val(query, "name")

	body := baro_find_tabs("/baro/doc/alias")
	body += tool.Baro_crumb([][]string{
		{"/baro/doc/alias", tool.Baro_t("별칭 · 동명이인")},
		{"", tool.Baro_t("동명이인 문서 도우미")},
	})
	body += tool.Baro_hero(tool.Baro_t("동명이인 문서 도우미"),
		tool.Baro_t("같은 이름을 가진 문서를 찾아 안내 문서 본문을 대신 써 드립니다."), "")

	form := tool.Baro_input("name", "이름", name, "예: 김성범", "괄호 없이 이름만 적어 주세요.")
	form += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("후보 찾기"), "primary") + `</div>`
	body += tool.Baro_card(tool.Baro_t("이름 입력"), `<form class="bw_form" method="get" action="/baro/doc/dab">`+form+`</form>`)

	if name == "" {
		body += tool.Baro_card(tool.Baro_t("동명이인 안내 문서란"), tool.Baro_list([]string{
			tool.Baro_t(`같은 이름을 가진 사람이 둘 이상일 때, 이름만 적힌 문서를 <b>안내 문서</b>로 만들어 각 인물로 보냅니다.`),
			tool.Baro_t(`구분은 <b>출생연도</b>와 <b>직업</b>으로 합니다. (예: 김성범(1890), 김성범(독립운동가))`),
			tool.Baro_t(`한자명이 다르면 한자명도 함께 적어 두면 좋습니다.`),
			tool.Baro_t(`인물 정보는 <b>baro_doc_meta</b> 의 출생연도·직업·한자명 값에서 가져옵니다.`),
		}))

		return tool.Baro_page(db, config, tool.Baro_t("동명이인 문서 도우미"), body, baro_find_menu())
	}

	base := baro_find_base_name(name)
	candidates := baro_find_same_name(db, base, "")

	rows := [][]string{}
	lines := []string{}

	for _, title := range candidates {
		meta := baro_find_meta_all(db, title)
		mark := baro_find_person_line(meta)
		if mark == "" {
			mark = tool.Baro_t("구분 정보 없음")
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(title),
			tool.HTML_escape(meta["birth_year"]),
			tool.HTML_escape(meta["job"]),
			tool.HTML_escape(meta["hanja"]),
			baro_find_alias_text(baro_find_alias_of(db, title)),
			tool.Baro_button("/baro/doc/alias/"+tool.Url_parser(title), "인물 정보 채우기", ""),
		})

		lines = append(lines, " * [["+tool.Namespace_display(title)+"]] - "+mark)
	}

	body += tool.Baro_card_ex(tool.Baro_t("동명이인 후보"), tool.Baro_t("모두 ")+tool.Baro_itoa(len(candidates))+tool.Baro_t("건"),
		tool.Baro_table([]string{"문서", tool.Baro_t("출생연도"), tool.Baro_t("직업"), "한자명", tool.Baro_t("등록된 별칭"), "관리"}, rows,
			tool.Baro_t("같은 이름을 가진 문서를 찾지 못했습니다. 아직 동명이인 안내 문서가 필요하지 않습니다.")),
		tool.Baro_note(tool.Baro_t("출생연도·직업이 비어 있으면 안내 문서에서 사람을 가릴 수 없습니다. 먼저 채워 주세요.")))

	if len(candidates) < 2 {
		body += tool.Baro_alert("info", tool.Baro_t("동명이인 안내 문서는 같은 이름의 문서가 둘 이상일 때 만듭니다. 지금은 ")+tool.Baro_itoa(len(candidates))+tool.Baro_t("건입니다."))

		return tool.Baro_page(db, config, tool.Baro_t("동명이인 : ")+base, body, baro_find_menu())
	}

	doc_body := "'''" + base + "'''은(는) 다음을 가리킨다.\n\n"
	doc_body += strings.Join(lines, "\n") + "\n\n"
	doc_body += "== 같이 보기 ==\n"
	doc_body += " * 이 문서는 동명이인 안내 문서입니다. 찾는 인물이 없다면 새 문서를 만들어 주세요.\n"

	body += tool.Baro_card_ex(tool.Baro_t("자동으로 만든 안내 문서 본문"), base+tool.Baro_t(" 문서에 넣을 내용"),
		tool.Baro_textarea("dab_body", "본문", doc_body, 12, "", "그대로 복사해 문서 편집 화면에 붙여 넣으세요.")+
			tool.Baro_note(tool.Baro_t("이 도우미는 문서를 직접 만들지 않습니다. 마지막 확인은 사람이 합니다.")),
		tool.Baro_button("/edit/"+tool.Url_parser(base), base+" 문서 편집하기", "primary")+" "+
			tool.Baro_button("/baro/doc/alias?doc="+tool.Url_parser(base), "별칭으로 등록하기", ""))

	return tool.Baro_page(db, config, tool.Baro_t("동명이인 : ")+base, body, baro_find_menu())
}

// View_baro_find_meta : 인물·문서 메타 편집 목록 (/baro/doc/meta)
func View_baro_find_meta(config tool.Config, page string, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	keyword := baro_find_val(query, "q")
	name := baro_find_val(query, "name")

	page_int := baro_find_page_int(page)
	if raw_page := baro_find_val(query, "page"); raw_page != "" {
		page_int = baro_find_page_int(raw_page)
	}

	offset := (page_int - 1) * tool.Baro_page_size

	sql_text := "select title, name, data from baro_doc_meta where 1 = 1"
	args := []any{}

	if name != "" {
		sql_text += " and name = ?"
		args = append(args, name)
	}
	if keyword != "" {
		sql_text += " and (lower(title) like ? or lower(data) like ?)"
		args = append(args, "%"+strings.ToLower(keyword)+"%", "%"+strings.ToLower(keyword)+"%")
	}

	sql_text += " order by title limit ?, ?"
	args = append(args, offset, tool.Baro_page_size)

	rows := tool.Query_DB(db, sql_text, args...)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		title, meta_name, data := "", "", ""
		if err := rows.Scan(&title, &meta_name, &data); err != nil {
			continue
		}

		label := meta_name
		for _, pair := range baro_find_meta_name_list() {
			if pair[0] == meta_name {
				label = pair[1]
				break
			}
		}

		table_rows = append(table_rows, []string{
			tool.Baro_doc_link(title),
			tool.HTML_escape(label),
			tool.HTML_escape(tool.Baro_trunc(data, 80)),
			tool.Baro_button("/baro/doc/alias/"+tool.Url_parser(title), "편집", ""),
		})
	}

	name_options := [][]string{{"", tool.Baro_t("항목 전체")}}
	name_options = append(name_options, baro_find_meta_name_list()...)

	filter := tool.Baro_input("q", tool.Baro_t("검색"), keyword, tool.Baro_t("문서 이름 또는 값"), "")
	filter += tool.Baro_select("name", "항목", name_options, name, "")
	filter += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("찾기"), "primary") + `</div>`

	page_url := url.Values{}
	page_url.Set("q", keyword)
	page_url.Set("name", name)

	list_html := tool.Baro_table([]string{"문서", tool.Baro_t("항목"), tool.Baro_t("값"), "관리"}, table_rows, "")
	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("저장된 문서 메타가 없습니다. 문서별 화면에서 출생연도·직업·한자명을 채워 주세요."))
	}
	list_html += tool.Baro_paging(page_int, len(table_rows), tool.Baro_page_size, "/baro/doc/meta?"+page_url.Encode()+"&page={}")

	body := baro_find_tabs("/baro/doc/alias")
	body += tool.Baro_hero(tool.Baro_t("문서 메타"), tool.Baro_t("출생연도 · 직업 · 한자명 · 메타 설명을 모아 봅니다."), "")
	body += tool.Baro_card(tool.Baro_t("찾기"), `<form class="bw_form" method="get" action="/baro/doc/meta">`+filter+`</form>`)
	body += tool.Baro_card(tool.Baro_t("저장된 값"), list_html)
	body += tool.Baro_card(tool.Baro_t("항목 설명"), tool.Baro_dl(func() [][]string {
		out := [][]string{}
		for _, row := range baro_find_meta_name_list() {
			out = append(out, []string{row[0], tool.HTML_escape(row[1])})
		}

		return out
	}()))

	return tool.Baro_page(db, config, tool.Baro_t("문서 메타"), body, baro_find_menu())
}
