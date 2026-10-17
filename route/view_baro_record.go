package route

import (
	"database/sql"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 [record] 모듈 — 통합 기여 기록
//
//  흩어져 있던 기록을 한 화면에 모으고, 무엇을 한 기록인지로 잘게 나눈다.
//    - 문서        (history)  : 전체 · 생성 · 삭제 · 복원 · 이동 · 되돌림
//    - 토론        (topic)
//    - 편집 요청   (baro_edit_req · baro_edit_req_log) : 생성한 것 · 승인하고 닫은 것
//    - 게시판      (bbs_data)  : 전체 · 게시글 · 댓글
//
//  「전체」 탭에서는 모두가 시간순으로 섞여 나온다.
//
//  문서 기록의 갈래는 history.type 으로 가른다.
//    r1 첫 판(생성) · delete 삭제 · move·move_all 이동 · revert 되돌림 · 그 밖은 보통 편집
//  「복원」 은 type 에 따로 남지 않는다. 바로 앞 판이 삭제였는지를 보고 가른다.
// ============================================================

// baro_record_page_size : 한 쪽에 보여 주는 줄 수
const baro_record_page_size = 50

// baro_record_kind : 고를 수 있는 갈래 하나
type baro_record_kind struct {
	Key    string // 주소에 쓰는 열쇠말
	Name   string // 화면에 보이는 이름
	Parent string // 큰 갈래 ("" 면 그 자체가 큰 갈래)
}

// baro_record_kind_all : 고를 수 있는 갈래 전부 (차례대로)
func baro_record_kind_all() []baro_record_kind {
	return []baro_record_kind{
		{"all", tool.Baro_t("전체"), ""},

		{"edit", tool.Baro_t("문서"), ""},
		{"edit_all", tool.Baro_t("문서 전체"), "edit"},
		{"edit_new", tool.Baro_t("문서 생성"), "edit"},
		{"edit_normal", tool.Baro_t("문서 편집"), "edit"},
		{"edit_delete", tool.Baro_t("문서 삭제"), "edit"},
		{"edit_restore", tool.Baro_t("문서 복원"), "edit"},
		{"edit_move", tool.Baro_t("문서 이동"), "edit"},
		{"edit_revert", tool.Baro_t("문서 되돌림"), "edit"},

		{"topic", tool.Baro_t("토론"), ""},

		{"req", tool.Baro_t("편집 요청"), ""},
		{"req_all", tool.Baro_t("편집 요청 전체"), "req"},
		{"req_make", tool.Baro_t("생성한 편집 요청"), "req"},
		{"req_handle", tool.Baro_t("승인·닫은 편집 요청"), "req"},

		{"bbs", tool.Baro_t("게시판"), ""},
		{"bbs_all", tool.Baro_t("게시판 전체"), "bbs"},
		{"bbs_post", tool.Baro_t("게시글"), "bbs"},
		{"bbs_comment", tool.Baro_t("댓글"), "bbs"},
	}
}

// baro_record_kind_get : 갈래 하나 찾기
func baro_record_kind_get(key string) (baro_record_kind, bool) {
	for _, one := range baro_record_kind_all() {
		if one.Key == key {
			return one, true
		}
	}

	return baro_record_kind{}, false
}

// Baro_record_kind_ok : 아는 갈래 이름인가 (길 등록 쪽에서 부른다)
func Baro_record_kind_ok(key string) bool {
	_, ok := baro_record_kind_get(key)

	return ok
}

// baro_record_kind_name : 갈래 이름
func baro_record_kind_name(key string) string {
	if one, ok := baro_record_kind_get(key); ok {
		return one.Name
	}

	return key
}

// baro_record_top : 이 갈래가 딸린 큰 갈래
//
//	`edit_new` 는 `edit` 에, `edit` 자신은 `edit` 에 딸린다.
func baro_record_top(key string) string {
	one, ok := baro_record_kind_get(key)
	if !ok {
		return "all"
	}

	if one.Parent != "" {
		return one.Parent
	}

	return one.Key
}

// baro_record_row : 통합 기록 한 줄
type baro_record_row struct {
	date   string
	kind   string
	target string
	detail string
	extra  string
}

// baro_record_esc : 화면에 넣기 전 이스케이프
func baro_record_esc(data string) string {
	return tool.HTML_escape(data)
}

// baro_record_send_esc : 편집 사유를 화면에 넣기 전에 다듬는다
//
//	이동 기록의 사유에는 예부터 <a> 가 글자 그대로 담겨 있다. (Do_add_history 가 붙인다)
//	그대로 escape 하면 표에 태그가 보이므로 껍데기만 떼고 escape 한다.
func baro_record_send_esc(data string) string {
	return tool.HTML_escape(baro_record_send_clean.Replace(data))
}

// baro_record_send_clean : 사유에서 떼어 낼 껍데기
var baro_record_send_clean = strings.NewReplacer("<a>", "", "</a>", "")

// baro_record_page_no : 쪽 번호
func baro_record_page_no(page string) int {
	num := tool.Str_to_int(strings.TrimSpace(page))
	if num < 1 {
		num = 1
	}
	if num > 2000 {
		num = 2000
	}

	return num
}

// baro_record_kind_badge : 종류 뱃지
func baro_record_kind_badge(db *sql.DB, kind string) string {
	switch kind {
	case "edit_normal":
		return tool.Baro_badge(tool.Baro_t("편집"), "info")
	case "edit_new":
		return tool.Baro_badge(tool.Baro_t("생성"), "ok")
	case "edit_delete":
		return tool.Baro_badge(tool.Baro_t("삭제"), "danger")
	case "edit_restore":
		return tool.Baro_badge(tool.Baro_t("복원"), "ok")
	case "edit_move":
		return tool.Baro_badge(tool.Baro_t("이동"), "warn")
	case "edit_revert":
		return tool.Baro_badge(tool.Baro_t("되돌림"), "warn")
	case "topic":
		return tool.Baro_badge(tool.Get_language(db, "discussion", true), "ok")
	case "req_make":
		return tool.Baro_badge(tool.Get_language(db, "edit_request", true), "warn")
	case "req_accept":
		return tool.Baro_badge(tool.Get_language(db, "edit_request_accept", true), "ok")
	case "req_reject":
		return tool.Baro_badge(tool.Get_language(db, "edit_request_reject", true), "muted")
	case "req_delete":
		return tool.Baro_badge(tool.Get_language(db, "edit_request_delete", true), "danger")
	case "bbs_post":
		return tool.Baro_badge(tool.Baro_t("게시글"), "info")
	case "bbs_comment":
		return tool.Baro_badge(tool.Baro_t("댓글"), "muted")
	}

	return tool.Baro_badge(kind, "muted")
}

// baro_record_edit_kind : 판 하나가 어떤 일이었는지 가른다
//
//	prev 는 같은 문서의 바로 앞 판 종류다. 앞 판이 삭제였다면 이 판은 문서를 되살린 것이다.
func baro_record_edit_kind(id string, kind string, prev string) string {
	switch kind {
	case "delete":
		return "edit_delete"
	case "move", "move_all":
		return "edit_move"
	case "revert":
		return "edit_revert"
	}

	if prev == "delete" {
		return "edit_restore"
	}

	if kind == "r1" || id == "1" {
		return "edit_new"
	}

	return "edit_normal"
}

// baro_record_edit_rows : 문서 편집 기록
func baro_record_edit_rows(db *sql.DB, user_name string, limit int, want string) []baro_record_row {
	// 앞 판의 종류까지 함께 읽는다. 「복원」 은 그것으로만 가릴 수 있다.
	rows := tool.Query_DB(
		db,
		"select h.id, h.title, h.date, h.send, h.leng, h.hide, h.type, "+
			"coalesce((select h2.type from history h2 where h2.title = h.title and h2.id + 0 < h.id + 0 "+
			"order by h2.id + 0 desc limit 1), '') "+
			"from history h where h.ip = ? order by h.date desc limit ?",
		user_name, limit,
	)
	defer rows.Close()

	list := []baro_record_row{}
	for rows.Next() {
		id, title, date, send, leng, hide := "", "", "", "", "", ""
		edit_type, prev_type := "", ""
		if rows.Scan(&id, &title, &date, &send, &leng, &hide, &edit_type, &prev_type) != nil {
			continue
		}

		if hide != "" {
			continue
		}

		row_kind := baro_record_edit_kind(id, edit_type, prev_type)
		if want != "" && want != row_kind {
			continue
		}

		length_html := baro_record_esc(leng)
		switch {
		case leng == "0" || leng == "":
			length_html = `<span style="color: var(--namu-length-same-color, #808080);">` + baro_record_esc(leng) + `</span>`
		case strings.Contains(leng, "+"):
			length_html = `<span style="color: var(--namu-length-plus-color, #008000);">` + baro_record_esc(leng) + `</span>`
		default:
			length_html = `<span style="color: var(--namu-length-minus-color, #f00);">` + baro_record_esc(leng) + `</span>`
		}

		target := tool.Baro_doc_link(title)
		// 지워진 판 때문에 번호에 구멍이 나도 실제로 남아 있는 앞 판을 가리키게 한다.
		if previous := history_tool_previous_revision(db, title, tool.Str_to_int(id)); previous != "" {
			target += ` <a href="/diff/` + tool.Url_parser(previous) + `/` + tool.Url_parser(id) + `/` + tool.Url_parser(title) + `">r` + baro_record_esc(id) + `</a>`
		} else {
			target += ` <a href="/render/` + tool.Url_parser(id) + `/` + tool.Url_parser(title) + `">r` + baro_record_esc(id) + `</a>`
		}

		list = append(list, baro_record_row{
			date:   date,
			kind:   row_kind,
			target: target,
			detail: baro_record_send_esc(tool.Baro_trunc(send, 80)),
			extra:  length_html,
		})
	}

	return list
}

// baro_record_topic_rows : 토론 참여 기록
func baro_record_topic_rows(db *sql.DB, user_name string, limit int) []baro_record_row {
	rows := tool.Query_DB(
		db,
		"select code, id, date, data from topic where ip = ? order by date desc limit ?",
		user_name, limit,
	)
	defer rows.Close()

	type one_topic struct {
		code    string
		comment string
		date    string
		data    string
	}

	found := []one_topic{}
	for rows.Next() {
		code, comment_id, date, data := "", "", "", ""
		if rows.Scan(&code, &comment_id, &date, &data) != nil {
			continue
		}

		found = append(found, one_topic{code, comment_id, date, data})
	}
	rows.Close()

	list := []baro_record_row{}
	for _, one := range found {
		topic_title, topic_sub := "", ""
		tool.QueryRow_DB(db, "select title, sub from rd where code = ?", []any{&topic_title, &topic_sub}, one.code)

		target := `<a href="/thread/` + tool.Url_parser(one.code) + `#` + tool.Url_parser(one.comment) + `">` +
			baro_record_esc(topic_sub) + `</a>`
		if topic_title != "" {
			target += ` <span class="bw_muted">(` + baro_record_esc(tool.Namespace_display(topic_title)) + `)</span>`
		}

		list = append(list, baro_record_row{
			date:   one.date,
			kind:   "topic",
			target: target,
			detail: baro_record_esc(tool.Baro_trunc(tool.Baro_one_line(one.data), 80)),
			extra:  "#" + baro_record_esc(one.comment),
		})
	}

	return list
}

// baro_record_req_rows : 편집 요청 생성 기록
func baro_record_req_rows(db *sql.DB, user_name string, limit int) []baro_record_row {
	rows := tool.Query_DB(
		db,
		"select id, title, summary, status, date from baro_edit_req where user = ? order by date desc limit ?",
		user_name, limit,
	)
	defer rows.Close()

	list := []baro_record_row{}
	for rows.Next() {
		id, title, summary, status, date := "", "", "", "", ""
		if rows.Scan(&id, &title, &summary, &status, &date) != nil {
			continue
		}

		target := tool.Baro_doc_link(title) +
			` <a href="/baro/req/view/` + tool.Url_parser(id) + `">#` + baro_record_esc(id) + `</a>`

		list = append(list, baro_record_row{
			date:   date,
			kind:   "req_make",
			target: target,
			detail: baro_record_esc(tool.Baro_trunc(summary, 80)),
			extra:  baro_req_status_badge(db, status),
		})
	}

	return list
}

// baro_record_req_handle_rows : 편집 요청 처리 기록 (승인 · 거절 · 삭제)
func baro_record_req_handle_rows(db *sql.DB, user_name string, limit int) []baro_record_row {
	rows := tool.Query_DB(
		db,
		"select req_id, title, action, detail, date from baro_edit_req_log where actor = ? and action != 'create' order by date desc limit ?",
		user_name, limit,
	)
	defer rows.Close()

	list := []baro_record_row{}
	for rows.Next() {
		req_id, title, action, detail, date := "", "", "", "", ""
		if rows.Scan(&req_id, &title, &action, &detail, &date) != nil {
			continue
		}

		target := tool.Baro_doc_link(title) +
			` <a href="/baro/req/view/` + tool.Url_parser(req_id) + `">#` + baro_record_esc(req_id) + `</a>`

		list = append(list, baro_record_row{
			date:   date,
			kind:   "req_" + action,
			target: target,
			detail: baro_record_esc(tool.Baro_trunc(detail, 80)),
			extra:  baro_record_esc(baro_req_action_name(db, action)),
		})
	}

	return list
}

// baro_record_bbs_rows : 게시판 이용 기록 (게시글 · 댓글)
//
//	게시판은 이름-값 표(bbs_data) 하나에 모든 것이 들어 있다.
//	글은 set_name='user_id', 댓글은 set_name='comment_user_id' 에 쓴 사람이 담긴다.
//	같은 (set_id, set_code) 자리에서 제목과 때를 다시 찾아 붙인다.
func baro_record_bbs_rows(db *sql.DB, user_name string, limit int, want string) []baro_record_row {
	list := []baro_record_row{}

	if want == "" || want == "bbs_post" {
		list = append(list, baro_record_bbs_post_rows(db, user_name, limit)...)
	}

	if want == "" || want == "bbs_comment" {
		list = append(list, baro_record_bbs_comment_rows(db, user_name, limit)...)
	}

	return list
}

// baro_record_bbs_one : bbs_data 한 칸 읽기
func baro_record_bbs_one(db *sql.DB, name string, set_id string, set_code string) string {
	data := ""
	tool.QueryRow_DB(
		db,
		"select set_data from bbs_data where set_name = ? and set_id = ? and set_code = ? limit 1",
		[]any{&data}, name, set_id, set_code,
	)

	return data
}

// baro_record_bbs_link : 글 하나로 가는 주소
func baro_record_bbs_link(set_id string, set_code string) string {
	return "/bbs/w/" + tool.Doc_url(set_id) + "/" + tool.Url_parser(set_code)
}

// baro_record_bbs_name : 게시판 번호로 이름 찾기
//
//	bbs_data 의 set_id 는 게시판 번호다. 사람에게는 이름을 보여 준다.
func baro_record_bbs_name(db *sql.DB, set_id string) string {
	name := ""
	tool.QueryRow_DB(
		db,
		"select set_data from bbs_set where set_name = 'bbs_name' and set_id = ? limit 1",
		[]any{&name}, set_id,
	)

	if name == "" {
		return set_id
	}

	return name
}

// baro_record_bbs_post_rows : 쓴 게시글
func baro_record_bbs_post_rows(db *sql.DB, user_name string, limit int) []baro_record_row {
	rows := tool.Query_DB(
		db,
		"select set_id, set_code from bbs_data where set_name = 'user_id' and set_data = ? "+
			"order by set_code + 0 desc limit ?",
		user_name, limit,
	)
	defer rows.Close()

	type one struct{ set_id, set_code string }

	found := []one{}
	for rows.Next() {
		set_id, set_code := "", ""
		if rows.Scan(&set_id, &set_code) != nil {
			continue
		}

		found = append(found, one{set_id, set_code})
	}
	rows.Close()

	list := []baro_record_row{}
	for _, row := range found {
		title := baro_record_bbs_one(db, "title", row.set_id, row.set_code)
		if title == "" {
			// 지워진 글은 제목 칸이 없다. 기록에서도 빼 준다.
			continue
		}

		target := `<a href="` + baro_record_bbs_link(row.set_id, row.set_code) + `">` +
			baro_record_esc(title) + `</a>` +
			` <span class="bw_muted">(` + baro_record_esc(baro_record_bbs_name(db, row.set_id)) + `)</span>`

		list = append(list, baro_record_row{
			date:   baro_record_bbs_one(db, "date", row.set_id, row.set_code),
			kind:   "bbs_post",
			target: target,
			detail: "",
			extra:  "#" + baro_record_esc(row.set_code),
		})
	}

	return list
}

// baro_record_bbs_comment_rows : 쓴 댓글
func baro_record_bbs_comment_rows(db *sql.DB, user_name string, limit int) []baro_record_row {
	rows := tool.Query_DB(
		db,
		"select set_id, set_code from bbs_data where set_name = 'comment_user_id' and set_data = ? "+
			"order by set_id desc, set_code + 0 desc limit ?",
		user_name, limit,
	)
	defer rows.Close()

	type one struct{ parent, code string }

	found := []one{}
	for rows.Next() {
		parent, code := "", ""
		if rows.Scan(&parent, &code) != nil {
			continue
		}

		found = append(found, one{parent, code})
	}
	rows.Close()

	boards := baro_record_bbs_board_list(db)

	list := []baro_record_row{}
	for _, row := range found {
		board, post_code := baro_record_bbs_where(db, row.parent, boards)
		if board == "" {
			continue
		}

		title := baro_record_bbs_one(db, "title", board, post_code)
		if title == "" {
			continue
		}

		target := `<a href="` + baro_record_bbs_link(board, post_code) +
			`?comment_select=` + tool.Url_parser(row.code) + `#opennamu_comment_select">` +
			baro_record_esc(title) + `</a>` +
			` <span class="bw_muted">(` + baro_record_esc(baro_record_bbs_name(db, board)) + `)</span>`

		list = append(list, baro_record_row{
			date:   baro_record_bbs_one(db, "comment_date", row.parent, row.code),
			kind:   "bbs_comment",
			target: target,
			detail: baro_record_esc(tool.Baro_trunc(tool.Baro_one_line(
				baro_record_bbs_one(db, "comment", row.parent, row.code)), 80)),
			extra: "#" + baro_record_esc(row.code),
		})
	}

	return list
}

// baro_record_bbs_board_list : 게시판 이름 목록
func baro_record_bbs_board_list(db *sql.DB) []string {
	rows := tool.Query_DB(db, "select set_id from bbs_set where set_name = 'bbs_name'")
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		name := ""
		if rows.Scan(&name) != nil {
			continue
		}

		out = append(out, name)
	}

	return out
}

// baro_record_bbs_where : 댓글이 달린 자리를 게시판과 글 번호로 푼다
//
//	댓글의 set_id 는 "<게시판>-<글번호>" 이고, 답글이면 "-<윗댓글>" 이 더 붙는다.
//	게시판 이름에도 붙임표가 들어갈 수 있으므로, 실제 게시판 목록과 맞춰 본다.
func baro_record_bbs_where(db *sql.DB, parent_id string, boards []string) (string, string) {
	for _, board := range boards {
		if !strings.HasPrefix(parent_id, board+"-") {
			continue
		}

		rest := strings.TrimPrefix(parent_id, board+"-")

		code, _, _ := strings.Cut(rest, "-")
		if code == "" {
			continue
		}

		return board, code
	}

	return "", ""
}

// baro_record_count : 갈래별 건수
//
//	문서 쪽은 판 종류를 세어야 해서 한 번에 묶어 센다.
func baro_record_count(db *sql.DB, user_name string) map[string]int {
	count := map[string]int{}

	baro_record_edit_count(db, user_name, count)

	topic_count := 0
	tool.QueryRow_DB(db, "select count(*) from topic where ip = ?", []any{&topic_count}, user_name)
	count["topic"] = topic_count

	req_count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_edit_req where user = ?", []any{&req_count}, user_name)
	count["req_make"] = req_count

	handle_count := 0
	tool.QueryRow_DB(
		db,
		"select count(*) from baro_edit_req_log where actor = ? and action != 'create'",
		[]any{&handle_count}, user_name,
	)
	count["req_handle"] = handle_count
	count["req"] = req_count + handle_count
	count["req_all"] = count["req"]

	post_count := 0
	tool.QueryRow_DB(
		db,
		"select count(*) from bbs_data where set_name = 'user_id' and set_data = ?",
		[]any{&post_count}, user_name,
	)
	count["bbs_post"] = post_count

	comment_count := 0
	tool.QueryRow_DB(
		db,
		"select count(*) from bbs_data where set_name = 'comment_user_id' and set_data = ?",
		[]any{&comment_count}, user_name,
	)
	count["bbs_comment"] = comment_count
	count["bbs"] = post_count + comment_count
	count["bbs_all"] = count["bbs"]

	count["all"] = count["edit"] + count["topic"] + count["req"] + count["bbs"]

	return count
}

// baro_record_edit_count : 문서 기록을 갈래별로 센다
func baro_record_edit_count(db *sql.DB, user_name string, count map[string]int) {
	rows := tool.Query_DB(
		db,
		"select h.id, h.type, h.hide, "+
			"coalesce((select h2.type from history h2 where h2.title = h.title and h2.id + 0 < h.id + 0 "+
			"order by h2.id + 0 desc limit 1), '') "+
			"from history h where h.ip = ?",
		user_name,
	)
	defer rows.Close()

	for rows.Next() {
		id, edit_type, hide, prev_type := "", "", "", ""
		if rows.Scan(&id, &edit_type, &hide, &prev_type) != nil {
			continue
		}

		if hide != "" {
			continue
		}

		count["edit"]++
		count[baro_record_edit_kind(id, edit_type, prev_type)]++
	}

	count["edit_all"] = count["edit"]
}

// View_baro_record : 통합 기여 기록
func View_baro_record(config tool.Config, user_name string, kind string, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	user_name = strings.TrimSpace(user_name)
	if user_name == "" {
		user_name = config.IP
	}

	if kind == "" {
		kind = "all"
	}

	page_num := baro_record_page_no(page)
	offset := (page_num - 1) * baro_record_page_size
	need := offset + baro_record_page_size

	if _, ok := baro_record_kind_get(kind); !ok {
		kind = "all"
	}

	list := baro_record_pick(db, user_name, kind, need)
	if len(list) > 1 {
		sort.SliceStable(list, func(a int, b int) bool {
			return list[a].date > list[b].date
		})
	}

	if offset >= len(list) {
		list = []baro_record_row{}
	} else {
		list = list[offset:]
	}
	if len(list) > baro_record_page_size {
		list = list[:baro_record_page_size]
	}

	table_rows := [][]string{}
	for _, one := range list {
		table_rows = append(table_rows, []string{
			tool.Baro_time_cell(one.date),
			baro_record_kind_badge(db, one.kind),
			one.target,
			one.detail,
			one.extra,
		})
	}

	count := baro_record_count(db, user_name)
	base := "/baro/record/user/" + tool.Url_parser(user_name) + "/"
	top := baro_record_top(kind)

	// 1행 : 큰 갈래. 2행 : 그 안의 잔 갈래 (있을 때만)
	big := [][]string{}
	small := [][]string{}

	for _, one := range baro_record_kind_all() {
		label := one.Name
		if number, ok := count[one.Key]; ok {
			label += " " + tool.Baro_comma(number)
		}

		row := []string{base + one.Key, label}

		switch {
		case one.Parent == "":
			big = append(big, row)
		case one.Parent == top:
			small = append(small, row)
		}
	}

	body := tool.Baro_hero_html(
		tool.Get_language(db, "contribution", true),
		tool.Baro_user_link(db, user_name, config.IP),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("문서"), tool.Baro_comma(count["edit"]), tool.Baro_t("생성 ") + tool.Baro_comma(count["edit_new"])},
			{tool.Baro_t("토론"), tool.Baro_comma(count["topic"]), ""},
			{tool.Baro_t("편집 요청"), tool.Baro_comma(count["req"]), tool.Baro_t("처리 ") + tool.Baro_comma(count["req_handle"])},
			{tool.Baro_t("게시판"), tool.Baro_comma(count["bbs"]), tool.Baro_t("댓글 ") + tool.Baro_comma(count["bbs_comment"])},
		}),
	)

	// 큰 갈래 탭은 잔 갈래를 고른 상태에서도 그 큰 갈래가 켜져 보여야 한다.
	inner := tool.Baro_tabs(big, base+top)
	if len(small) > 0 {
		inner += tool.Baro_tabs(small, base+kind)
	}

	inner += tool.Baro_table(
		[]string{
			tool.Get_language(db, "date", true),
			tool.Get_language(db, "type", true),
			tool.Get_language(db, "content", true),
			tool.Get_language(db, "why", true),
			tool.Get_language(db, "etc", true),
		},
		table_rows,
		tool.Baro_t("기록이 없습니다."),
	)
	inner += tool.Baro_paging(
		page_num, len(table_rows), baro_record_page_size,
		"/baro/record/user/"+tool.Url_parser(user_name)+"/"+tool.Url_parser(kind)+"/{}",
	)

	body += tool.Baro_card("", inner)

	link := tool.Baro_button("/baro/contrib/user/"+tool.Url_parser(user_name), "기여 이벤트", "") + " " +
		tool.Baro_button("/user/"+tool.Url_parser(user_name), tool.Get_language(db, "user_tool", true), "") + " " +
		tool.Baro_button("/baro/req/new", tool.Get_language(db, "edit_request_make", true), "")

	body += tool.Baro_card("", link)

	return tool.Baro_page(
		db, config,
		tool.Get_language(db, "contribution", true),
		body,
		[][]any{
			{"baro/record", tool.Get_language(db, "contribution", true)},
			{"baro", "바로위키"},
		},
	)
}

// baro_record_pick : 고른 갈래에 해당하는 줄들을 모은다
func baro_record_pick(db *sql.DB, user_name string, kind string, need int) []baro_record_row {
	list := []baro_record_row{}

	switch kind {
	case "edit", "edit_all":
		return baro_record_edit_rows(db, user_name, need, "")

	case "edit_new", "edit_normal", "edit_delete", "edit_restore", "edit_move", "edit_revert":
		return baro_record_edit_rows(db, user_name, need, kind)

	case "topic":
		return baro_record_topic_rows(db, user_name, need)

	case "req", "req_all":
		list = append(list, baro_record_req_rows(db, user_name, need)...)
		list = append(list, baro_record_req_handle_rows(db, user_name, need)...)

		return list

	case "req_make":
		return baro_record_req_rows(db, user_name, need)

	case "req_handle":
		return baro_record_req_handle_rows(db, user_name, need)

	case "bbs", "bbs_all":
		return baro_record_bbs_rows(db, user_name, need, "")

	case "bbs_post", "bbs_comment":
		return baro_record_bbs_rows(db, user_name, need, kind)
	}

	// 전체
	list = append(list, baro_record_edit_rows(db, user_name, need, "")...)
	list = append(list, baro_record_topic_rows(db, user_name, need)...)
	list = append(list, baro_record_req_rows(db, user_name, need)...)
	list = append(list, baro_record_req_handle_rows(db, user_name, need)...)
	list = append(list, baro_record_bbs_rows(db, user_name, need, "")...)

	return list
}
