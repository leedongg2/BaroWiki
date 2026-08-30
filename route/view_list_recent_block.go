package route

import (
	"database/sql"
	"opennamu/route/tool"
)

func Get_ui_recent_block(db *sql.DB, config tool.Config, data_all [][]string) string {
	auth_info := tool.Get_user_auth_info(db, config.IP)
	ban_auth := auth_info["owner"] || auth_info["ban"]

	body_html := ""
	for _, in_data := range data_all {
		why := in_data[0]
		block_raw := in_data[1]
		block_render := in_data[2]
		blocker_render := in_data[4]
		end := in_data[5]
		today := in_data[6]
		band := in_data[7]
		ongoing := in_data[8]

		// 차단 종류
		type_text := tool.Get_language(db, "normal", true)
		url_match := "ban"
		switch band {
		case "private":
			type_text = tool.Get_language(db, "private", true)
		case "cidr":
			type_text = tool.Get_language(db, "cidr", true)
			url_match = "ban_cidr"
		case "regex":
			type_text = tool.Get_language(db, "regex", true)
			url_match = "ban_regex"
		case "warn":
			type_text = tool.Get_language(db, "warn", true)
		}

		target_html := block_render
		if ban_auth && band != "warn" {
			target_html = `<a href="/auth/` + url_match + `/` + tool.Url_parser(block_raw) + `">` + block_render + `</a>`
		}

		// 만료 / 상태
		end_text := tool.Get_language(db, "limitless", true)
		state_text := `<span class="namu_block_state_on">` + tool.Get_language(db, "ongoing", true) + `</span>`

		if end == "release" {
			end_text = "-"
			state_text = `<span class="namu_block_state_off">` + tool.Get_language(db, "ban_state_release", true) + `</span>`
		} else if ongoing != "1" {
			if end != "" {
				end_text = tool.HTML_escape(end)
			}
			state_text = `<span class="namu_block_state_off">` + tool.Get_language(db, "ban_state_end", true) + `</span>`
		} else if end != "" {
			end_text = tool.HTML_escape(end)
			if tool.Ban_is_expired(end) {
				state_text = `<span class="namu_block_state_off">` + tool.Get_language(db, "expire", true) + `</span>`
			} else {
				end_text += ` <span class="namu_block_remain">(` + tool.HTML_escape(tool.Ban_duration_text(end)) + ` ` + tool.Get_language(db, "remain", true) + `)</span>`
			}
		}

		why_html := "-"
		if why != "" {
			if why == "edit filter" {
				why_html = `<a href="/edit_filter/` + tool.Url_parser(block_raw) + `">edit filter</a>`
			} else {
				why_html = Get_safe_send_data(tool.HTML_escape(why))
			}
		}

		tool_html := "-"
		if ban_auth && ongoing == "1" && end != "release" && band != "warn" {
			tool_html = `<a class="namu_button_small" href="/auth/` + url_match + `/` + tool.Url_parser(block_raw) + `">` + tool.Get_language(db, "ban_release_change", true) + `</a>`
		}

		body_html += `<tr>`
		body_html += `<td>` + target_html + `</td>`
		body_html += `<td>` + tool.HTML_escape(type_text) + `</td>`
		body_html += `<td>` + why_html + `</td>`
		body_html += `<td>` + tool.HTML_escape(today) + `</td>`
		body_html += `<td>` + end_text + `</td>`
		body_html += `<td>` + blocker_render + `</td>`
		body_html += `<td>` + state_text + `</td>`
		body_html += `<td>` + tool_html + `</td>`
		body_html += `</tr>`
	}

	if body_html == "" {
		body_html = `<tr><td colspan="8" class="namu_table_empty">` + tool.Get_language(db, "ban_record_empty", true) + `</td></tr>`
	}

	data_html := `<div class="namu_table_wrap"><table class="namu_table namu_block_table">`
	data_html += `<thead><tr><th>` + tool.Get_language(db, "target", true) + `</th><th>` + tool.Get_language(db, "type", true) + `</th><th>` + tool.Get_language(db, "why", true) + `</th><th>` + tool.Get_language(db, "ban_time", true) + `</th><th>` + tool.Get_language(db, "expire", true) + `</th><th>` + tool.Get_language(db, "blocker", true) + `</th><th>` + tool.Get_language(db, "state", true) + `</th><th>` + tool.Get_language(db, "manage_col", true) + `</th></tr></thead>`
	data_html += `<tbody>` + body_html + `</tbody></table></div>`

	return data_html
}

func View_list_recent_block(config tool.Config, num string, set_type string, why string, user_name string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	data_html := ""
	sub := ""

	if set_type == "" {
		set_type = "all"
	}

	menu_option := [][]string{
		{"all", tool.Get_language(db, "all", true)},
		{"regex", tool.Get_language(db, "regex", true)},
		{"cidr", tool.Get_language(db, "cidr", true)},
		{"private", tool.Get_language(db, "private", true)},
		{"ongoing", tool.Get_language(db, "in_progress", true)},
	}
	for _, option := range menu_option {
		data_html += `<a href="/recent_block/` + option[0] + `/1">(` + option[1] + `)</a> `

		if option[0] == set_type && set_type != "all" {
			sub = "(" + option[1] + ")"
		}
	}

	menu_option = [][]string{
		{"/manager/11", tool.Get_language(db, "blocked", true)},
		{"/manager/12", tool.Get_language(db, "admin", true)},
		{"/manager/19", tool.Get_language(db, "why", true)},
	}
	for _, option := range menu_option {
		data_html += `<a href="` + option[0] + `">(` + option[1] + `)</a> `
	}

	data_html += "<hr class=\"main_hr\">"

	api_data := Api_list_recent_block(config, num, set_type, why, user_name)
	api_data_list := api_data["data"].([][]string)

	data_html += Get_ui_recent_block(db, config, api_data_list)

	base_url := "/recent_block/" + tool.Url_parser(set_type)

	if user_name != "" {
		base_url += "/" + tool.Url_parser(user_name)
	}

	base_url += "/{}"

	if why != "" {
		base_url += "/" + tool.Url_parser(why)
	}

	data_html += tool.Get_page_control(
		db,
		tool.Str_to_int(num),
		len(api_data_list),
		50,
		base_url,
	)

	out := tool.Get_template(
		db,
		config,
		tool.Get_language(db, "recent_ban", true),
		data_html,
		[]any{sub},
		[][]any{
			{"other", tool.Get_language(db, "return", true)},
		},
		map[string]string{},
	)

	return out
}
