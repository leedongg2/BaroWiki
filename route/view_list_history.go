package route

import (
	"strconv"
	"strings"

	"opennamu/route/tool"
)

func View_list_history(config tool.Config, doc_name string, set_type string, num string, query_values ...string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	sub := ""
	if set_type == "" {
		set_type = "normal"
	} else {
		sub = " (" + tool.Get_language(db, set_type, true) + ")"
	}

	data_html := ""

	menu_option := []string{"normal", "edit", "move", "delete", "revert", "r1", "setting"}
	for _, option := range menu_option {
		label := tool.Get_language(db, option, true)
		data_html += `<a href="/history_page/1/` + option + `/` + tool.Url_parser(doc_name) + `">(` + label + `)</a> `
	}

	offset := -1
	commit_uuid := ""
	if len(query_values) > 0 {
		commit_uuid = strings.TrimSpace(query_values[0])
	}
	if len(query_values) > 1 && query_values[1] != "" {
		if parsed_offset, err := strconv.Atoi(query_values[1]); err == nil && parsed_offset >= 0 {
			offset = parsed_offset
		}
	}
	if offset < 0 && tool.Baro_uuid_is(commit_uuid) {
		db_revision := tool.Baro_rev_resolve(db, doc_name, commit_uuid)
		if tool.Str_to_int(db_revision) > 0 {
			offset = Api_history_revision_offset(doc_name, set_type, db_revision)
		}
	}

	var api_data map[string]any
	if offset >= 0 {
		api_data = api_list_history_offset(config, doc_name, set_type, offset)
	} else {
		api_data = Api_list_history(config, doc_name, set_type, num)
	}
	api_data_list := api_data["data"].([][]string)

	history_ui, select_ui := Get_ui_history(db, config, api_data_list)

	data_html += history_ui
	if offset >= 0 {
		page_url := "/history_page/1/" + set_type + "/" + tool.Url_parser(doc_name) + "?offset="
		data_html += `<hr class="main_hr">`
		if offset > 0 {
			previous_offset := offset - 50
			if previous_offset < 0 {
				previous_offset = 0
			}
			data_html += `<a href="` + page_url + strconv.Itoa(previous_offset) + `">(` + tool.Get_language(db, "previous", true) + `)</a> `
		}
		if len(api_data_list) == 50 {
			data_html += `<a href="` + page_url + strconv.Itoa(offset+50) + `">(` + tool.Get_language(db, "next", true) + `)</a> `
		}
	} else {
		data_html += tool.Get_page_control(
			db,
			tool.Str_to_int(num),
			len(api_data_list),
			50,
			"/history_page/{}/"+set_type+"/"+tool.Url_parser(doc_name),
		)
	}

	data_html += `<form method="get" action="/history/` + tool.Url_parser(doc_name) + `" class="opennamu_history_jump_form" style="display:flex;align-items:center;width:max-content;margin:16px 0;">` +
		`<input class="__ON_INPUT__" type="text" name="rev" inputmode="numeric" placeholder="r" aria-label="판 번호" style="width:8rem;margin:0;">` +
		`<button class="__ON_BUTTON__" type="submit" aria-label="해당 판으로 이동" title="해당 판으로 이동" style="margin:0;">&#x21AA;</button></form>`

	data_html = `
	        <form method="post">
	            <select name="a">` + select_ui + `</select>
	            <select name="b">` + select_ui + `</select>
	            <button type="submit">` + tool.Get_language(db, "compare", true) + `</button>
	        </form>
	        <hr class="main_hr">
	    ` + data_html

	menu := [][]any{
		{"w/" + tool.Url_parser(doc_name), tool.Get_language(db, "return", true)},
	}
	if tool.Check_acl(db, "", "", "owner_auth", config.IP) {
		menu = append(menu, []any{"history_add/" + tool.Url_parser(doc_name), tool.Get_language(db, "history_add", true)})
		menu = append(menu, []any{"history_reset/" + tool.Url_parser(doc_name), tool.Get_language(db, "history_reset", true)})
	}

	out := tool.Get_template(
		db,
		config,
		doc_name,
		data_html,
		[]any{"(" + tool.Get_language(db, "history", true) + ")" + sub},
		menu,
		map[string]string{},
	)

	return out
}
