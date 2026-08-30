package route

import (
	"database/sql"

	"opennamu/route/tool"
)

func View_bbs_in(config tool.Config, set_id string, page_num string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	bbs_name := Api_bbs_num_to_name(db, set_id)["data"].(string)
	if bbs_name == "" {
		return tool.Get_redirect("/bbs/main")
	}

	data_api := Api_bbs(config, set_id, page_num)
	data_api_in := data_api["data"].([]map[string]string)

	data_html := tool.Get_bbs_block_banner(db, config, set_id)
	if tool.Get_bbs_lock(db, set_id) {
		data_html += `<div class="namu_alert namu_alert_danger">` + tool.Get_language(db, "bbs_locked_notice", true) + `</div>`
	}
	data_html += Get_bbs_list_ui(config, data_api_in, map[string]string{})
	data_html += tool.Get_page_control(
		db,
		tool.Str_to_int(page_num),
		len(data_api_in),
		50,
		"/bbs/in/"+tool.Url_parser(set_id)+"/{}",
	)

	out := tool.Get_template(
		db,
		config,
		bbs_name,
		data_html,
		[]any{},
		bbs_in_menu(db, config, set_id),
		map[string]string{},
	)

	return out
}

// bbs_in_menu : 게시판 메뉴
func bbs_in_menu(db *sql.DB, config tool.Config, set_id string) [][]any {
	menu := [][]any{
		{"bbs/main", tool.Get_language(db, "return", true)},
		{"bbs/edit/" + tool.Url_parser(set_id), tool.Get_language(db, "add", true)},
		{"bbs/set/" + tool.Url_parser(set_id), tool.Get_language(db, "bbs_set", true)},
	}

	if tool.Check_acl(db, "", "", "bbs_auth", config.IP) {
		menu = append(menu, []any{"bbs/lock/" + tool.Url_parser(set_id), tool.Get_language(db, "bbs_lock", true)})
		menu = append(menu, []any{"bbs/block", tool.Get_language(db, "bbs_block", true)})
	}

	return menu
}
