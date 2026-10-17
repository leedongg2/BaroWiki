package route

import (
	"database/sql"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"opennamu/route/tool"
)

// ============================================================
//  위키 데이터 연동 — /baro/wikilink
//
//  다른 위키에 같은 대상을 다룬 문서가 있으면 그 둘을 이어 두고
//  「무엇이 다른가」 를 나란히 볼 수 있게 한다.
//
//  누가 무엇을 할 수 있는가
//    - 위키 등록 · 문서 잇기 : 관리자만          (바깥 주소를 부르는 일이라 좁게 연다)
//    - 연동 요청             : 누구나 게시판에서 (「위키 데이터 연동 요청 게시판」)
//    - 비교 보기             : 이어 둔 문서라면 누구나
//
//  비교 자체는 view_baro_wikilink_compare.go 가 한다.
// ============================================================

// baro_link_bbs_name : 연동 요청 게시판 이름
const baro_link_bbs_name = "위키 데이터 연동 요청 게시판"

// baro_link_row : 등록된 위키 한 곳
type baro_link_row struct {
	ID      string
	Name    string
	BaseUrl string
	RawPath string
	Status  string
	Actor   string
	Date    string
	Note    string
}

// baro_link_map_row : 문서 잇기 한 줄
type baro_link_map_row struct {
	ID          string
	Wiki        string
	Title       string
	RemoteTitle string
	Actor       string
	Date        string
	Note        string
}

// baro_link_menu : 공통 메뉴
func baro_link_menu() [][]any {
	return [][]any{
		{"baro/wikilink", tool.Baro_t("위키 연동")},
		{"baro", "바로위키"},
	}
}

// baro_link_tabs : 탭 줄
func baro_link_tabs(db *sql.DB, config tool.Config, active string) string {
	items := [][]string{
		{"/baro/wikilink", tool.Baro_t("이어 둔 문서")},
		{"/baro/wikilink/wiki", tool.Baro_t("등록된 위키")},
	}

	if bbs_id := baro_link_bbs_id(db); bbs_id != "" {
		items = append(items, []string{"/bbs/in/" + tool.Url_parser(bbs_id), tool.Baro_t("연동 요청 게시판")})
	}

	if tool.Baro_is_admin(db, config.IP) {
		items = append(items, []string{"/baro/wikilink/new", tool.Baro_t("위키 등록")})
		items = append(items, []string{"/baro/wikilink/set", tool.Baro_t("AI 비교 설정")})
	}

	return tool.Baro_tabs(items, active)
}

// baro_link_bbs_lock : 게시판을 만드는 동안 다른 요청이 끼어들지 못하게 한다
var baro_link_bbs_lock sync.Mutex

// baro_link_bbs_id : 연동 요청 게시판 번호 (없으면 만든다)
func baro_link_bbs_id(db *sql.DB) string {
	find := func() string {
		set_id := ""
		tool.QueryRow_DB(
			db,
			"select set_id from bbs_set where set_name = 'bbs_name' and set_data = ? order by set_id + 0 asc",
			[]any{&set_id}, baro_link_bbs_name,
		)

		return set_id
	}

	if set_id := find(); set_id != "" {
		return set_id
	}

	// 게시판이 없으면 한 번만 만들어 둔다.
	// 두 사람이 동시에 들어와도 게시판이 두 개 생기지 않게 잠근 뒤 다시 확인한다.
	baro_link_bbs_lock.Lock()
	defer baro_link_bbs_lock.Unlock()

	if set_id := find(); set_id != "" {
		return set_id
	}

	last := ""
	tool.QueryRow_DB(db, "select set_id from bbs_set where set_name = 'bbs_name' order by set_id + 0 desc", []any{&last})
	set_id := strconv.Itoa(tool.Str_to_int(last) + 1)

	tool.Exec_DB(db, "insert into bbs_set (set_name, set_code, set_id, set_data) values ('bbs_name', '', ?, ?)", set_id, baro_link_bbs_name)
	tool.Exec_DB(db, "insert into bbs_set (set_name, set_code, set_id, set_data) values ('bbs_type', '', ?, 'comment')", set_id)

	return set_id
}

// baro_link_wiki_list : 등록된 위키 목록
func baro_link_wiki_list(db *sql.DB, only_on bool) []baro_link_row {
	query := "select id, name, base_url, raw_path, status, actor, date, note from baro_wiki_link"
	if only_on {
		query += " where status = 'on'"
	}
	query += " order by id + 0 asc"

	rows := tool.Query_DB(db, query)
	defer rows.Close()

	out := []baro_link_row{}
	for rows.Next() {
		one := baro_link_row{}
		if rows.Scan(&one.ID, &one.Name, &one.BaseUrl, &one.RawPath, &one.Status, &one.Actor, &one.Date, &one.Note) != nil {
			continue
		}

		out = append(out, one)
	}

	return out
}

// baro_link_wiki_one : 위키 한 곳
func baro_link_wiki_one(db *sql.DB, id string) (baro_link_row, bool) {
	one := baro_link_row{ID: id}
	found := tool.QueryRow_DB(
		db,
		"select name, base_url, raw_path, status, actor, date, note from baro_wiki_link where id = ?",
		[]any{&one.Name, &one.BaseUrl, &one.RawPath, &one.Status, &one.Actor, &one.Date, &one.Note},
		id,
	)

	return one, found
}

// baro_link_map_one : 잇기 한 줄
func baro_link_map_one(db *sql.DB, id string) (baro_link_map_row, bool) {
	one := baro_link_map_row{ID: id}
	found := tool.QueryRow_DB(
		db,
		"select wiki, title, remote_title, actor, date, note from baro_wiki_map where id = ?",
		[]any{&one.Wiki, &one.Title, &one.RemoteTitle, &one.Actor, &one.Date, &one.Note},
		id,
	)

	return one, found
}

// baro_link_map_list : 잇기 목록 (title 이 비면 전체)
func baro_link_map_list(db *sql.DB, title string, limit int) []baro_link_map_row {
	query := "select id, wiki, title, remote_title, actor, date, note from baro_wiki_map"
	values := []any{}
	if title != "" {
		query += " where title = ?"
		values = append(values, title)
	}
	query += " order by id + 0 desc limit ?"
	values = append(values, limit)

	rows := tool.Query_DB(db, query, values...)
	defer rows.Close()

	out := []baro_link_map_row{}
	for rows.Next() {
		one := baro_link_map_row{}
		if rows.Scan(&one.ID, &one.Wiki, &one.Title, &one.RemoteTitle, &one.Actor, &one.Date, &one.Note) != nil {
			continue
		}

		out = append(out, one)
	}

	return out
}

// baro_link_remote_url : 상대 문서를 사람이 볼 주소
func baro_link_remote_url(wiki baro_link_row, remote_title string) string {
	base := strings.TrimSuffix(wiki.BaseUrl, "/")
	if base == "" {
		return ""
	}

	return base + "/w/" + url.PathEscape(remote_title)
}

// baro_link_remote_raw_url : 상대 문서의 원문 주소
//
//	raw_path 안의 {} 자리에 문서 이름이 들어간다. 비어 있으면 /raw/{} 로 본다.
func baro_link_remote_raw_url(wiki baro_link_row, remote_title string) string {
	base := strings.TrimSuffix(wiki.BaseUrl, "/")
	if base == "" {
		return ""
	}

	path := strings.TrimSpace(wiki.RawPath)
	if path == "" {
		path = "/raw/{}"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return base + strings.ReplaceAll(path, "{}", url.PathEscape(remote_title))
}

// View_baro_link : 이어 둔 문서 목록
func View_baro_link(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	wiki_name := map[string]string{}
	for _, one := range baro_link_wiki_list(db, false) {
		wiki_name[one.ID] = one.Name
	}

	list := [][]string{}
	for _, one := range baro_link_map_list(db, "", 100) {
		name := wiki_name[one.Wiki]
		if name == "" {
			name = tool.Baro_t("지워진 위키")
		}

		list = append(list, []string{
			tool.Baro_doc_link(one.Title),
			tool.HTML_escape(name),
			tool.HTML_escape(one.RemoteTitle),
			tool.Baro_user_link(db, one.Actor, config.IP),
			tool.Baro_time_cell(one.Date),
			tool.Baro_button("/baro/wikilink/compare/"+tool.Url_parser(one.ID), tool.Baro_t("비교 보기"), "primary"),
		})
	}

	bbs_id := baro_link_bbs_id(db)

	body := tool.Baro_hero(
		tool.Baro_t("위키 데이터 연동"),
		tool.Baro_t("다른 위키에 같은 대상을 다룬 문서가 있으면 이어 두고 무엇이 다른지 나란히 봅니다."),
		tool.Baro_button("/bbs/edit/"+tool.Url_parser(bbs_id), tool.Baro_t("연동 요청하기"), "primary")+" "+
			tool.Baro_button("/bbs/in/"+tool.Url_parser(bbs_id), tool.Baro_t("연동 요청 게시판"), ""),
	)

	inner := baro_link_tabs(db, config, "/baro/wikilink")
	inner += tool.Baro_alert("info",
		tool.Baro_t("위키를 등록하고 문서를 잇는 일은 <b>관리자만</b> 할 수 있습니다. 이어 주었으면 하는 문서가 있으면 연동 요청 게시판에 적어 주세요."))
	inner += tool.Baro_table(
		[]string{tool.Get_language(db, "document", true), tool.Baro_t("상대 위키"), tool.Baro_t("상대 문서"),
			tool.Baro_t("이은 사람"), tool.Get_language(db, "date", true), ""},
		list, tool.Baro_t("아직 이어 둔 문서가 없습니다."),
	)

	body += tool.Baro_card("", inner)

	return tool.Baro_page(db, config, tool.Baro_t("위키 데이터 연동"), body, baro_link_menu())
}

// View_baro_link_wiki : 등록된 위키 목록
func View_baro_link_wiki(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	is_admin := tool.Baro_is_admin(db, config.IP)

	list := [][]string{}
	for _, one := range baro_link_wiki_list(db, false) {
		state := tool.Baro_badge(tool.Baro_t("연동 중"), "ok")
		if one.Status != "on" {
			state = tool.Baro_badge(tool.Baro_t("멈춤"), "muted")
		}

		count := 0
		tool.QueryRow_DB(db, "select count(*) from baro_wiki_map where wiki = ?", []any{&count}, one.ID)

		action := ""
		if is_admin {
			action = tool.Baro_button("/baro/wikilink/edit/"+tool.Url_parser(one.ID), tool.Get_language(db, "edit", true), "")
		}

		list = append(list, []string{
			tool.HTML_escape(one.Name),
			`<a class="opennamu_link_out" target="_blank" rel="nofollow noopener" href="` + tool.HTML_escape(one.BaseUrl) + `">` + tool.HTML_escape(one.BaseUrl) + `</a>`,
			state,
			tool.Baro_comma(count),
			tool.Baro_user_link(db, one.Actor, config.IP),
			action,
		})
	}

	body := tool.Baro_hero(tool.Baro_t("등록된 위키"), tool.Baro_t("여기에 등록된 위키에서만 문서를 받아 옵니다."), "")

	inner := baro_link_tabs(db, config, "/baro/wikilink/wiki")
	inner += tool.Baro_table(
		[]string{tool.Baro_t("이름"), tool.Baro_t("주소"), tool.Get_language(db, "state", true),
			tool.Baro_t("이어 둔 문서"), tool.Baro_t("등록한 사람"), ""},
		list, tool.Baro_t("등록된 위키가 없습니다."),
	)

	body += tool.Baro_card("", inner)

	return tool.Baro_page(db, config, tool.Baro_t("등록된 위키"), body, baro_link_menu())
}

// View_baro_link_wiki_edit : 위키 등록 · 고치기 (관리자만)
func View_baro_link_wiki_edit(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_is_admin(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("위키 등록은 관리자만 할 수 있습니다. 연동 요청 게시판에 적어 주세요."))
	}

	row := baro_link_row{Status: "on", RawPath: "/raw/{}"}
	if id != "" {
		found := false
		row, found = baro_link_wiki_one(db, id)
		if !found {
			return tool.Baro_deny(db, config, tool.Baro_t("없는 위키입니다."))
		}
	}

	if values != nil {
		if values.Get("action") == "delete" && id != "" {
			tool.Exec_DB(db, "delete from baro_wiki_link where id = ?", id)
			tool.Exec_DB(db, "delete from baro_wiki_map where wiki = ?", id)
			tool.Baro_public_log(db, "wiki_link_delete", config.IP, row.Name, "", "")

			return tool.Get_redirect("/baro/wikilink/wiki")
		}

		name := tool.Baro_one_line(values.Get("name"))
		base_url := strings.TrimSpace(values.Get("base_url"))
		raw_path := tool.Baro_one_line(values.Get("raw_path"))
		note := tool.Baro_one_line(values.Get("note"))
		status := "off"
		if values.Get("status") == "on" {
			status = "on"
		}

		if name == "" {
			return tool.Baro_deny(db, config, tool.Baro_t("위키 이름을 적어 주세요."))
		}

		clean_url, check_error := tool.Baro_fetch_url_check(base_url)
		if check_error != "" {
			return tool.Baro_deny(db, config, check_error)
		}
		if raw_path == "" {
			raw_path = "/raw/{}"
		}
		if !strings.Contains(raw_path, "{}") {
			return tool.Baro_deny(db, config, tool.Baro_t("원문 주소 틀에는 문서 이름이 들어갈 {} 자리가 있어야 합니다."))
		}

		if id == "" {
			id = tool.Baro_next_id(db, "baro_wiki_link")
			tool.Exec_DB(
				db,
				"insert into baro_wiki_link (id, name, base_url, raw_path, status, actor, date, note) values (?, ?, ?, ?, ?, ?, ?, ?)",
				id, name, clean_url, raw_path, status, config.IP, tool.Baro_now(), note,
			)
			tool.Baro_public_log(db, "wiki_link_add", config.IP, name, clean_url, "")
		} else {
			tool.Exec_DB(
				db,
				"update baro_wiki_link set name = ?, base_url = ?, raw_path = ?, status = ?, note = ? where id = ?",
				name, clean_url, raw_path, status, note, id,
			)
			tool.Baro_public_log(db, "wiki_link_edit", config.IP, name, clean_url, "")
		}

		return tool.Get_redirect("/baro/wikilink/wiki")
	}

	form := tool.Baro_input("name", tool.Baro_t("위키 이름"), row.Name, tool.Baro_t("예) 다른위키"), tool.Baro_t("화면에 보일 이름입니다."))
	form += tool.Baro_input("base_url", tool.Baro_t("위키 주소"), row.BaseUrl, "https://example.wiki", tool.Baro_t("맨 앞 주소만 적습니다. 뒤에 /w/문서이름 을 붙여 씁니다."))
	form += tool.Baro_input("raw_path", tool.Baro_t("원문 주소 틀"), row.RawPath, "/raw/{}", tool.Baro_t("문서 이름이 들어갈 자리를 {} 로 적습니다. openNAMU 위키는 /raw/{} 입니다."))
	form += tool.Baro_select("status", tool.Get_language(db, "state", true), [][]string{
		{"on", tool.Baro_t("연동 중")},
		{"off", tool.Baro_t("멈춤")},
	}, row.Status, tool.Baro_t("멈추면 이 위키에서는 문서를 받아 오지 않습니다."))
	form += tool.Baro_input("note", tool.Get_language(db, "explanation", true), row.Note, "", tool.Baro_t("어떤 위키인지, 어떤 약속으로 연동했는지 적어 둡니다."))
	form += tool.Baro_submit(tool.Get_language(db, "save", true), "primary")

	title := tool.Baro_t("위키 등록")
	action := "/baro/wikilink/new"
	if id != "" {
		title = tool.Baro_t("위키 고치기")
		action = "/baro/wikilink/edit/" + tool.Url_parser(id)
	}

	body := tool.Baro_hero(title, tool.Baro_t("여기에 등록된 위키에서만 문서를 받아 옵니다. 내부망 주소는 등록할 수 없습니다."), "")
	body += tool.Baro_card("", baro_link_tabs(db, config, "/baro/wikilink/new")+tool.Baro_form(action, form))

	if id != "" {
		remove := tool.Baro_hidden("action", "delete") + tool.Baro_submit(tool.Get_language(db, "delete", true), "danger")
		body += tool.Baro_danger_zone(tool.Baro_t("이 위키 연동 지우기"), tool.Baro_form(action, remove))
	}

	return tool.Baro_page(db, config, title, body, baro_link_menu())
}

// View_baro_link_set : AI 비교 설정 (관리자만)
func View_baro_link_set(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_is_admin(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("이 설정은 관리자만 볼 수 있습니다."))
	}

	if values != nil {
		endpoint := strings.TrimSpace(values.Get("ai_compare_url"))
		if endpoint != "" {
			if _, check_error := tool.Baro_fetch_url_check(endpoint); check_error != "" {
				return tool.Baro_deny(db, config, check_error)
			}
		}

		tool.Baro_put(db, "ai_compare_url", endpoint)
		tool.Baro_put(db, "ai_compare_model", tool.Baro_one_line(values.Get("ai_compare_model")))
		if key := strings.TrimSpace(values.Get("ai_compare_key")); key != "" {
			tool.Baro_put(db, "ai_compare_key", key)
		}
		if values.Get("clear_key") == "1" {
			tool.Baro_put(db, "ai_compare_key", "")
		}

		tool.Baro_public_log(db, "wiki_link_ai_set", config.IP, "", endpoint, "")

		return tool.Get_redirect("/baro/wikilink/set")
	}

	endpoint, key, model := baro_compare_ai_setting(db)
	key_state := tool.Baro_t("넣지 않음")
	if key != "" {
		key_state = tool.Baro_t("넣어 둠 (다시 넣으면 바뀝니다)")
	}

	form := tool.Baro_input("ai_compare_url", tool.Baro_t("AI 주소"), endpoint, "https://api.openai.com/v1/chat/completions",
		tool.Baro_t("OpenAI 호환 chat/completions 주소를 넣습니다. 비우면 규칙 비교만 합니다."))
	form += tool.Baro_input("ai_compare_model", tool.Baro_t("모형 이름"), model, "gpt-4o-mini", tool.Baro_t("비우면 gpt-4o-mini 로 보냅니다."))
	form += tool.Baro_input_type("ai_compare_key", tool.Baro_t("API 열쇠"), "", "password",
		tool.Baro_t("지금 상태 : ")+key_state+tool.Baro_t(" · 열쇠는 화면에 다시 보여 주지 않습니다."))
	form += tool.Baro_submit(tool.Get_language(db, "save", true), "primary")

	body := tool.Baro_hero(tool.Baro_t("AI 비교 설정"),
		tool.Baro_t("AI 주소를 넣으면 두 문서를 보내 요약을 받아 옵니다. 넣지 않아도 규칙 비교는 늘 됩니다."), "")
	body += tool.Baro_card("", baro_link_tabs(db, config, "/baro/wikilink/set")+tool.Baro_form("/baro/wikilink/set", form))
	body += tool.Baro_card("", tool.Baro_alert("warn",
		tool.Baro_t("문서 본문이 바깥 AI 서버로 나갑니다. 비공개 문서를 다루는 위키라면 켜기 전에 이용자에게 알리세요.")))

	clear_form := tool.Baro_hidden("clear_key", "1") + tool.Baro_submit(tool.Baro_t("열쇠 지우기"), "danger")
	body += tool.Baro_danger_zone(tool.Baro_t("열쇠 지우기"), tool.Baro_form("/baro/wikilink/set", clear_form))

	return tool.Baro_page(db, config, tool.Baro_t("AI 비교 설정"), body, baro_link_menu())
}

// View_baro_link_doc : 문서 하나에 걸린 연동
func View_baro_link_doc(config tool.Config, doc_name string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	title := tool.Namespace_normalize(strings.TrimSpace(doc_name))
	if title == "" {
		return tool.Get_redirect("/baro/wikilink")
	}

	is_admin := tool.Baro_is_admin(db, config.IP)

	if values != nil {
		if !is_admin {
			return tool.Baro_deny(db, config, tool.Baro_t("문서를 잇는 일은 관리자만 할 수 있습니다. 연동 요청 게시판에 적어 주세요."))
		}

		if values.Get("action") == "delete" {
			map_id := values.Get("map_id")
			tool.Exec_DB(db, "delete from baro_wiki_map where id = ?", map_id)
			tool.Baro_public_log(db, "wiki_map_delete", config.IP, title, "", map_id)

			return tool.Get_redirect("/baro/wikilink/doc/" + tool.Url_parser(title))
		}

		wiki := strings.TrimSpace(values.Get("wiki"))
		remote_title := tool.Baro_one_line(values.Get("remote_title"))
		note := tool.Baro_one_line(values.Get("note"))

		if _, found := baro_link_wiki_one(db, wiki); !found {
			return tool.Baro_deny(db, config, tool.Baro_t("등록되지 않은 위키입니다."))
		}
		if remote_title == "" {
			return tool.Baro_deny(db, config, tool.Baro_t("상대 위키의 문서 이름을 적어 주세요."))
		}

		id := tool.Baro_next_id(db, "baro_wiki_map")
		tool.Exec_DB(
			db,
			"insert into baro_wiki_map (id, wiki, title, remote_title, actor, date, note) values (?, ?, ?, ?, ?, ?, ?)",
			id, wiki, title, remote_title, config.IP, tool.Baro_now(), note,
		)
		tool.Baro_public_log(db, "wiki_map_add", config.IP, title, remote_title, id)

		return tool.Get_redirect("/baro/wikilink/compare/" + tool.Url_parser(id))
	}

	wiki_name := map[string]string{}
	options := [][]string{}
	for _, one := range baro_link_wiki_list(db, true) {
		wiki_name[one.ID] = one.Name
		options = append(options, []string{one.ID, one.Name})
	}

	list := [][]string{}
	for _, one := range baro_link_map_list(db, title, 50) {
		name := wiki_name[one.Wiki]
		if name == "" {
			name = tool.Baro_t("멈춘 위키")
		}

		action := tool.Baro_button("/baro/wikilink/compare/"+tool.Url_parser(one.ID), tool.Baro_t("비교 보기"), "primary")
		if is_admin {
			action += " " + tool.Baro_form("/baro/wikilink/doc/"+tool.Url_parser(title),
				tool.Baro_hidden("action", "delete")+tool.Baro_hidden("map_id", one.ID)+tool.Baro_submit(tool.Get_language(db, "delete", true), "danger"))
		}

		list = append(list, []string{
			tool.HTML_escape(name),
			tool.HTML_escape(one.RemoteTitle),
			tool.Baro_user_link(db, one.Actor, config.IP),
			tool.Baro_time_cell(one.Date),
			action,
		})
	}

	bbs_id := baro_link_bbs_id(db)

	body := tool.Baro_hero(tool.Baro_t("이 문서의 위키 연동"), tool.Namespace_display(title),
		tool.Baro_button("/w/" + tool.Doc_url(title), tool.Baro_t("문서 보기"), "")+" "+
			tool.Baro_button("/bbs/edit/"+tool.Url_parser(bbs_id), tool.Baro_t("연동 요청하기"), "primary"))

	inner := baro_link_tabs(db, config, "/baro/wikilink")
	inner += tool.Baro_table(
		[]string{tool.Baro_t("상대 위키"), tool.Baro_t("상대 문서"), tool.Baro_t("이은 사람"), tool.Get_language(db, "date", true), ""},
		list, tool.Baro_t("아직 이어 둔 위키가 없습니다."),
	)

	body += tool.Baro_card("", inner)

	if is_admin {
		if len(options) == 0 {
			body += tool.Baro_card(tool.Baro_t("문서 잇기"), tool.Baro_alert("warn",
				tool.Baro_t("먼저 위키를 등록해야 합니다.")+" "+tool.Baro_button("/baro/wikilink/new", tool.Baro_t("위키 등록"), "primary")))
		} else {
			form := tool.Baro_select("wiki", tool.Baro_t("상대 위키"), options, "", "")
			form += tool.Baro_input("remote_title", tool.Baro_t("상대 문서 이름"), tool.Namespace_display(title), "", tool.Baro_t("상대 위키에서 쓰는 문서 이름입니다."))
			form += tool.Baro_input("note", tool.Get_language(db, "explanation", true), "", "", tool.Baro_t("왜 같은 대상이라고 보는지 적어 둡니다."))
			form += tool.Baro_submit(tool.Baro_t("이어 두기"), "primary")

			body += tool.Baro_card(tool.Baro_t("문서 잇기"), tool.Baro_form("/baro/wikilink/doc/"+tool.Url_parser(title), form))
		}
	} else {
		body += tool.Baro_card("", tool.Baro_alert("info",
			tool.Baro_t("문서를 잇는 일은 관리자만 할 수 있습니다. 이어 주었으면 하는 문서가 있으면 연동 요청 게시판에 적어 주세요.")))
	}

	return tool.Baro_page(db, config, tool.Baro_t("위키 연동"), body, baro_link_menu())
}

// View_baro_link_compare : 이어 둔 두 문서 비교
func View_baro_link_compare(config tool.Config, id string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, found := baro_link_map_one(db, id)
	if !found {
		return tool.Baro_deny(db, config, tool.Baro_t("없는 연동입니다."))
	}

	// 우리 문서 본문을 나란히 보여 주는 화면이므로 읽기 권한을 먼저 본다
	if !tool.Check_acl(db, row.Title, "", "render", config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("이 문서를 읽을 권한이 없습니다."))
	}

	wiki, wiki_found := baro_link_wiki_one(db, row.Wiki)
	if !wiki_found {
		return tool.Baro_deny(db, config, tool.Baro_t("상대 위키가 지워졌습니다."))
	}
	if wiki.Status != "on" {
		return tool.Baro_deny(db, config, tool.Baro_t("이 위키와의 연동은 멈춰 있습니다."))
	}

	mine := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&mine}, row.Title)

	raw_url := baro_link_remote_raw_url(wiki, row.RemoteTitle)
	other_raw, fetch_error := tool.Baro_fetch_text(raw_url)
	other := baro_compare_extract_remote(other_raw)

	body := tool.Baro_hero(
		tool.Baro_t("문서 비교"),
		tool.Namespace_display(row.Title)+" ↔ "+wiki.Name+" : "+row.RemoteTitle,
		tool.Baro_button("/w/" + tool.Doc_url(row.Title), tool.Baro_t("우리 문서"), "primary")+" "+
			tool.Baro_button(baro_link_remote_url(wiki, row.RemoteTitle), tool.Baro_t("상대 문서"), "")+" "+
			tool.Baro_button("/baro/wikilink/doc/"+tool.Url_parser(row.Title), tool.Baro_t("연동 관리"), ""),
	)

	if fetch_error != "" {
		body += tool.Baro_card("", tool.Baro_alert("danger", tool.Baro_t("상대 문서를 받아 오지 못했습니다 : ")+tool.HTML_escape(fetch_error)))
		body += tool.Baro_card("", tool.Baro_note("상대 위키가 원문 주소를 다르게 쓰는 경우일 수 있습니다. 관리자가 「원문 주소 틀」을 고쳐 주세요."))

		return tool.Baro_page(db, config, tool.Baro_t("문서 비교"), body, baro_link_menu())
	}

	result := baro_compare_run(mine, other)
	result.AISummary, result.AINote = baro_compare_ai(db, tool.Namespace_display(row.Title), mine, wiki.Name+" : "+row.RemoteTitle, other)

	body += tool.Baro_card("", tool.Baro_stat_row([][]string{
		{tool.Baro_t("닮은 정도"), strconv.Itoa(result.Similarity) + "%", ""},
		{tool.Baro_t("우리 글자"), tool.Baro_comma(result.MineLength), ""},
		{tool.Baro_t("상대 글자"), tool.Baro_comma(result.OtherLength), ""},
		{tool.Baro_t("같은 문장"), tool.Baro_comma(len(result.Same)), ""},
	}))

	if result.AISummary != "" {
		body += tool.Baro_card(tool.Baro_t("AI 요약"), `<div class="bw_code">`+tool.HTML_escape(result.AISummary)+`</div>`)
	} else if result.AINote != "" {
		body += tool.Baro_card(tool.Baro_t("AI 요약"), tool.Baro_alert("info", tool.HTML_escape(result.AINote)))
	}

	diff := `<div class="bw_diff_grid">`
	diff += baro_compare_list_html("우리 문서에만 있는 내용", result.OnlyMine, 40)
	diff += baro_compare_list_html("상대 문서에만 있는 내용", result.OnlyOther, 40)
	diff += `</div>`

	if result.MineCut > 0 || result.OtherCut > 0 {
		body += tool.Baro_card("", tool.Baro_alert("warn",
			tool.Baro_t("문서가 길어 앞쪽 ")+tool.Baro_comma(baro_compare_max_sentence)+
				tool.Baro_t(" 문장까지만 견줬습니다. 우리 문서 ")+tool.Baro_comma(result.MineCut)+
				tool.Baro_t("줄, 상대 문서 ")+tool.Baro_comma(result.OtherCut)+tool.Baro_t("줄은 보지 못했습니다.")))
	}

	body += tool.Baro_card(tool.Baro_t("서로 다른 내용"), diff)
	body += tool.Baro_card(tool.Baro_t("숫자가 어긋나는 자리"), baro_compare_gap_html(tool.Baro_t("숫자"), result.NumberGap))
	body += tool.Baro_card(tool.Baro_t("날짜가 어긋나는 자리"), baro_compare_gap_html(tool.Baro_t("날짜"), result.DateGap))
	body += tool.Baro_card(tool.Baro_t("양쪽에 다 있는 내용"), baro_compare_list_html("같은 문장", result.Same, 25))

	body += tool.Baro_card("", tool.Baro_note("비교는 무엇이 다른지만 말합니다. 어느 쪽이 옳은지는 출처를 보고 사람이 정합니다.")+
		tool.Baro_button("/edit/"+tool.Url_parser(row.Title), tool.Baro_t("우리 문서 고치기"), "primary"))

	return tool.Baro_page(db, config, tool.Baro_t("문서 비교"), body, baro_link_menu())
}
