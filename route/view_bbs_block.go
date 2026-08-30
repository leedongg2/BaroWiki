package route

import (
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// bbs_select_ui : 게시판 선택 UI (all = 모든 게시판)
func bbs_select_ui(config tool.Config, name string, selected string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	data_html := `<select class="namu_select" name="` + name + `">`

	choice := ""
	if selected == tool.Bbs_block_all || selected == "" {
		choice = ` selected`
	}
	data_html += `<option value="` + tool.Bbs_block_all + `"` + choice + `>` + tool.Lang_esc("all_bbs") + `</option>`

	for _, bbs_data := range Api_bbs_list(config)["data"].([][]string) {
		bbs_name := bbs_data[0]
		bbs_id := bbs_data[1]

		choice := ""
		if selected == bbs_id {
			choice = ` selected`
		}

		data_html += `<option value="` + tool.HTML_escape(bbs_id) + `"` + choice + `>` + tool.HTML_escape(bbs_name) + `</option>`
	}

	return data_html + `</select>`
}

// View_bbs_block : 게시판 차단 관리
func View_bbs_block(config tool.Config, user_name string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Check_acl(db, "", "", "bbs_auth", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	if values != nil {
		bbs_id := values.Get("bbs_id")
		if bbs_id == "" {
			bbs_id = tool.Bbs_block_all
		}

		target_text := values.Get("user_name")
		if user_name != "" {
			target_text = user_name
		}

		release := values.Get("do_type") == "release"

		end := ""
		if values.Get("date_forever") == "" {
			duration_second := tool.Str_to_int(values.Get("date_preset"))
			if values.Get("date_preset") == "" {
				duration_second = tool.Ban_duration_second(
					values.Get("date_year"),
					values.Get("date_month"),
					values.Get("date_week"),
					values.Get("date_day"),
					values.Get("date_hour"),
					values.Get("date_minute"),
					values.Get("date_second"),
				)
			}

			end = tool.Ban_end_from_duration(duration_second)
		}

		for _, target := range strings.Split(strings.ReplaceAll(target_text, "\r", ""), "\n") {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}

			if release {
				tool.Do_bbs_block_release(db, bbs_id, target)
				tool.Do_insert_auth_history(db, config.IP, "bbs_block release ("+target+")")

				continue
			}

			tool.Do_bbs_block_insert(db, bbs_id, target, end, values.Get("why"), config.IP)
			tool.Do_insert_auth_history(db, config.IP, "bbs_block ("+target+" / "+bbs_id+")")

			if !tool.IP_or_user(target) {
				end_text := tool.Lang("limitless")
				if end != "" {
					end_text = end
				}

				tool.Send_alarm(
					db, config.IP, target,
					tool.Lang("bbs_banned_notice")+" "+tool.Lang("expire")+" : "+end_text+
						" / "+tool.Lang("why")+" : "+values.Get("why"),
				)
			}
		}

		return tool.Get_redirect("/bbs/block")
	}

	target_input := `<textarea class="opennamu_textarea_100 namu_textarea" name="user_name" placeholder="` + tool.Get_language(db, "user_name_or_ip_multiple", true) + `"></textarea>`
	if user_name != "" {
		target_input = `<input class="namu_input" name="user_name" value="` + tool.HTML_escape(user_name) + `" readonly>`
	}

	preset_html := `<select class="namu_select" name="date_preset">`
	for _, option := range ban_preset_list() {
		preset_html += `<option value="` + option[0] + `">` + option[1] + `</option>`
	}
	preset_html += `</select>`

	data_html := `<form method="post" class="namu_form">`
	data_html += `<h2 class="namu_form_title">` + tool.Get_language(db, "target", true) + `</h2>` + target_input
	data_html += `<h2 class="namu_form_title">` + tool.Get_language(db, "bbs", true) + `</h2>` + bbs_select_ui(config, "bbs_id", "")
	data_html += `<h2 class="namu_form_title">` + tool.Get_language(db, "period", true) + `</h2><div class="namu_duration_row">` + preset_html + `</div>`
	data_html += `<div class="namu_duration_row">`
	data_html += ban_duration_input("date_day", tool.Get_language(db, "day", true))
	data_html += ban_duration_input("date_hour", tool.Get_language(db, "hour", true))
	data_html += ban_duration_input("date_minute", tool.Get_language(db, "minute", true))
	data_html += ban_duration_input("date_second", tool.Get_language(db, "second", true))
	data_html += `</div>`
	data_html += `<div class="namu_duration_row"><label class="namu_check_label"><input type="checkbox" name="date_forever" value="1"> ` + tool.Get_language(db, "limitless", true) + `</label></div>`
	data_html += `<h2 class="namu_form_title">` + tool.Get_language(db, "why", true) + `</h2><input class="namu_input" name="why" placeholder="` + tool.Get_language(db, "why", true) + `">`
	data_html += `<hr class="main_hr"><select class="namu_select" name="do_type"><option value="">` + tool.Get_language(db, "ban", true) + `</option><option value="release">` + tool.Get_language(db, "release", true) + `</option></select> `
	data_html += `<button class="namu_button" type="submit">` + tool.Get_language(db, "apply", true) + `</button></form>`

	// 차단 목록
	list_html := ""
	for _, block_data := range tool.Get_bbs_block_list(db) {
		bbs_text := tool.Get_language(db, "all_bbs", true)
		if block_data[0] != tool.Bbs_block_all {
			bbs_text = tool.HTML_escape(tool.Get_bbs_name(db, block_data[0]))
		}

		end_text := tool.Get_language(db, "limitless", true)
		if block_data[2] != "" {
			end_text = tool.HTML_escape(block_data[2])
		}

		list_html += `<tr>`
		list_html += `<td>` + tool.IP_parser(db, block_data[1], config.IP) + `</td>`
		list_html += `<td>` + bbs_text + `</td>`
		list_html += `<td>` + end_text + `</td>`
		list_html += `<td>` + tool.HTML_escape(block_data[3]) + `</td>`
		list_html += `<td>` + tool.IP_parser(db, block_data[4], config.IP) + `</td>`
		list_html += `<td>` + tool.HTML_escape(block_data[5]) + `</td>`
		list_html += `</tr>`
	}

	if list_html == "" {
		list_html = `<tr><td colspan="6" class="namu_table_empty">` + tool.Get_language(db, "bbs_ban_record_empty", true) + `</td></tr>`
	}

	data_html += `<h2 class="namu_form_title">` + tool.Get_language(db, "bbs_ban_now", true) + `</h2>`
	data_html += `<div class="namu_table_wrap"><table class="namu_table"><thead><tr><th>` + tool.Get_language(db, "target", true) + `</th><th>` + tool.Get_language(db, "bbs", true) + `</th><th>` + tool.Get_language(db, "expire", true) + `</th><th>` + tool.Get_language(db, "why", true) + `</th><th>` + tool.Get_language(db, "blocker", true) + `</th><th>` + tool.Get_language(db, "time", true) + `</th></tr></thead><tbody>` + list_html + `</tbody></table></div>`

	menu := [][]any{
		{"bbs/main", tool.Get_language(db, "bbs", true)},
		{"auth/ban", tool.Get_language(db, "user_ban", true)},
		{"manager", tool.Get_language(db, "return", true)},
	}

	return tool.Get_template(db, config, tool.Get_language(db, "bbs_block", true), data_html, []any{}, menu, map[string]string{})
}

// View_bbs_lock : 게시판 잠금 설정 (읽기 전용 전환)
func View_bbs_lock(config tool.Config, set_id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Check_acl(db, "", "", "bbs_auth", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	bbs_name := Api_bbs_num_to_name(db, set_id)["data"].(string)
	if bbs_name == "" {
		return tool.Get_redirect("/bbs/main")
	}

	if values != nil {
		tool.Do_bbs_lock_set(db, set_id, values.Get("lock") != "")
		tool.Do_insert_auth_history(db, config.IP, "bbs_lock ("+set_id+")")

		return tool.Get_redirect("/bbs/in/" + tool.Url_parser(set_id))
	}

	checked := ""
	if tool.Get_bbs_lock(db, set_id) {
		checked = ` checked`
	}

	data_html := `<div class="namu_form_help">` + tool.Get_language(db, "bbs_lock_help", false) + `</div>`
	data_html += `<form method="post" class="namu_form"><label class="namu_check_label"><input type="checkbox" name="lock" value="1"` + checked + `> ` + tool.Get_language(db, "bbs_lock", true) + `</label>`
	data_html += `<hr class="main_hr"><button class="namu_button" type="submit">` + tool.Get_language(db, "save", true) + `</button></form>`

	menu := [][]any{
		{"bbs/in/" + tool.Url_parser(set_id), tool.Get_language(db, "return", true)},
		{"bbs/block", tool.Get_language(db, "bbs_block", true)},
	}

	return tool.Get_template(db, config, bbs_name, data_html, []any{"(" + tool.Get_language(db, "bbs_lock", true) + ")"}, menu, map[string]string{})
}
