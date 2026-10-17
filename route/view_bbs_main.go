package route

import (
	"strconv"

	"opennamu/route/tool"
)

func Get_bbs_list_ui(config tool.Config, bbs_all_data []map[string]string, bbs_id_to_name map[string]string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	count := 0
	body_html := ""

	for _, in_data := range bbs_all_data {
		count_str := strconv.Itoa(count)
		count += 1

		bbs_title := in_data["title"]
		bbs_id := in_data["set_id"]
		bbs_code := in_data["set_code"]
		bbs_date := in_data["date"]
		bbs_user_id := in_data["user_id_render"]

		bbs_comment_length_api := Api_bbs_w_comment(config, "length", bbs_id+"-"+bbs_code)
		bbs_comment_length := bbs_comment_length_api["data"].(int)

		bbs_view_count := "0"
		if _, ok := in_data["view_count"]; ok {
			bbs_view_count = in_data["view_count"]
		}

		title_html := ""
		if in_data["prefix"] != "" {
			title_html += `<span class="namu_bbs_prefix">[` + tool.HTML_escape(in_data["prefix"]) + `]</span> `
		}

		title_html += `<a class="namu_bbs_title" href="/bbs/w/` + tool.HTML_escape(bbs_id) + `/` + tool.HTML_escape(bbs_code) + `">` + tool.HTML_escape(bbs_title) + `</a>`

		if bbs_comment_length > 0 {
			title_html += ` <span class="namu_bbs_comment_count">[` + strconv.Itoa(bbs_comment_length) + `]</span>`
		}

		if bbs_name := bbs_id_to_name[bbs_id]; bbs_name != "" {
			title_html += ` <a class="namu_bbs_board" href="/bbs/in/` + tool.HTML_escape(bbs_id) + `">(` + tool.HTML_escape(bbs_name) + `)</a>`
		}

		tag_html := ""
		for _, tag := range bbs_tag_list(in_data["tags"]) {
			tag_html += `<span class="namu_bbs_tag">#` + tool.HTML_escape(tag) + `</span>`
		}
		if tag_html != "" {
			title_html += `<div class="namu_bbs_tag_list">` + tag_html + `</div>`
		}

		row_class := ""
		number_html := bbs_code
		if in_data["pinned"] == "1" {
			row_class = ` class="namu_bbs_notice"`
			number_html = `<span class="namu_bbs_notice_mark">` + tool.Lang_esc("notice_mark") + `</span>`
		}

		body_html += `<tr` + row_class + `>`
		body_html += `<td class="namu_bbs_num">` + tool.HTML_escape(number_html) + `</td>`
		body_html += `<td class="namu_bbs_subject">` + title_html + `<span id="opennamu_bbs_comment_` + count_str + `"></span></td>`
		body_html += `<td class="namu_bbs_writer">` + bbs_user_id + `</td>`
		body_html += `<td class="namu_bbs_date">` + tool.HTML_escape(bbs_date) + `</td>`
		body_html += `<td class="namu_bbs_view">` + tool.HTML_escape(bbs_view_count) + `</td>`
		body_html += `</tr>`
	}

	if body_html == "" {
		body_html = `<tr><td colspan="5" class="namu_table_empty">` + tool.Lang_esc("no_post") + `</td></tr>`
	}

	data_html := `<div class="namu_table_wrap"><table class="namu_table namu_bbs_table">`
	data_html += `<thead><tr><th class="namu_bbs_num">` + tool.Lang_esc("number") + `</th><th class="namu_bbs_subject">` + tool.Lang_esc("title") + `</th><th class="namu_bbs_writer">` + tool.Lang_esc("writer") + `</th><th class="namu_bbs_date">` + tool.Lang_esc("write_date") + `</th><th class="namu_bbs_view">` + tool.Lang_esc("view_count") + `</th></tr></thead>`
	data_html += `<tbody>` + body_html + `</tbody></table></div>`

	return data_html
}

func View_bbs_main(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	bbs_list_api_data := Api_bbs_list(config)

	bbs_id_to_name := map[string]string{}

	data_html := tool.Get_bbs_block_banner(db, config, "")
	data_html += `<div class="namu_bbs_board_list">`
	for _, in_data := range bbs_list_api_data["data"].([][]string) {
		bbs_name := in_data[0]
		bbs_id := in_data[1]
		bbs_type := in_data[2]

		bbs_id_to_name[bbs_id] = bbs_name

		type_text := tool.Get_language(db, "thread_base", false)
		if bbs_type == "comment" {
			type_text = tool.Get_language(db, "comment_base", false)
		}

		lock_text := ""
		if tool.Get_bbs_lock(db, bbs_id) {
			lock_text = `<span class="namu_bbs_lock">` + tool.Lang_esc("lock") + `</span>`
		}

		data_html += `<a class="namu_bbs_board_item" href="/bbs/in/` + tool.Url_parser(bbs_id) + `">`
		data_html += `<span class="namu_bbs_board_name">` + tool.HTML_escape(bbs_name) + `</span>`
		data_html += `<span class="namu_bbs_board_type">` + type_text + `</span>` + lock_text
		data_html += `</a>`
	}
	data_html += `</div>`

	// 글을 쓸 수 있는 게시판 고르기 — 게시판 메인에서 바로 글쓰기로 갈 수 있게 한다
	data_html += bbs_write_bar(db, config, bbs_list_api_data["data"].([][]string), "")

	bbs_api_data := Api_bbs(config, "", page)
	data_html += Get_bbs_list_ui(config, bbs_api_data["data"].([]map[string]string), bbs_id_to_name)

	menu := [][]any{{"other", tool.Get_language(db, "other_tool", false)}}
	if tool.Check_acl(db, "", "", "owner_auth", config.IP) {
		menu = append(menu, []any{"bbs/make", tool.Get_language(db, "add", false)})
	}
	if tool.Check_acl(db, "", "", "bbs_auth", config.IP) {
		menu = append(menu, []any{"bbs/block", tool.Get_language(db, "bbs_block", true)})
	}

	out := tool.Get_template(
		db,
		config,
		tool.Get_language(db, "bbs_main", true),
		data_html,
		[]any{},
		menu,
		map[string]string{},
	)

	return out
}
