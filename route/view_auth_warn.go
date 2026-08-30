package route

import (
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// View_auth_warn : 사용자 경고 발급
func View_auth_warn(config tool.Config, user_name string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Check_acl(db, "", "", "ban_auth", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	if values != nil {
		target_list := []string{user_name}
		if user_name == "" {
			target_list = strings.Split(strings.ReplaceAll(values.Get("user_name"), "\r", ""), "\n")
		}

		why := values.Get("why")

		for _, target := range target_list {
			target = strings.TrimSpace(target)
			if target == "" || tool.IP_or_user(target) {
				continue
			}

			exist_user := ""
			if !tool.QueryRow_DB(db, "select id from user_set where id = ? limit 1", []any{&exist_user}, target) {
				continue
			}

			tool.Do_warn_insert(db, target, why, config.IP)
			tool.Do_insert_auth_history(db, config.IP, "warn ("+target+")")
		}

		return tool.Get_redirect("/auth/warn")
	}

	target_input := `<textarea class="opennamu_textarea_100 namu_textarea" name="user_name" placeholder="` + tool.Get_language(db, "many_delete_help", true) + `"></textarea>`
	if user_name != "" {
		target_input = `<input class="namu_input" name="user_name" value="` + tool.HTML_escape(user_name) + `" readonly>`
	}

	data_html := `<div class="namu_form_help">` + tool.Get_language(db, "warn_help", false) + `</div>`
	data_html += `<form method="post" class="namu_form">`
	data_html += `<h2 class="namu_form_title">` + tool.Get_language(db, "target", true) + `</h2>` + target_input
	data_html += `<h2 class="namu_form_title">` + tool.Get_language(db, "why", true) + `</h2><input class="namu_input" name="why" placeholder="` + tool.Get_language(db, "why", true) + `">`
	data_html += `<hr class="main_hr"><button class="namu_button" type="submit">` + tool.Get_language(db, "warn_give", true) + `</button></form>`

	// 최근 경고 목록
	rows := tool.Query_DB(db, "select block, why, today, blocker, ongoing from rb where band = 'warn' order by today desc limit 50")
	list_html := ""
	for rows.Next() {
		block := ""
		why := ""
		today := ""
		blocker := ""
		ongoing := ""

		if rows.Scan(&block, &why, &today, &blocker, &ongoing) != nil {
			continue
		}

		state := tool.Get_language(db, "checked", true)
		if ongoing == "1" {
			state = tool.Get_language(db, "unchecked", true)
		}

		list_html += `<tr><td>` + tool.IP_parser(db, block, config.IP) + `</td><td>` + tool.HTML_escape(why) + `</td><td>` + tool.HTML_escape(today) + `</td><td>` + tool.IP_parser(db, blocker, config.IP) + `</td><td>` + state + `</td></tr>`
	}
	rows.Close()

	if list_html == "" {
		list_html = `<tr><td colspan="5" class="namu_table_empty">` + tool.Get_language(db, "warn_record_empty", true) + `</td></tr>`
	}

	data_html += `<h2 class="namu_form_title">` + tool.Get_language(db, "warn_recent", true) + `</h2>`
	data_html += `<div class="namu_table_wrap"><table class="namu_table"><thead><tr><th>` + tool.Get_language(db, "target", true) + `</th><th>` + tool.Get_language(db, "why", true) + `</th><th>` + tool.Get_language(db, "time", true) + `</th><th>` + tool.Get_language(db, "writer", true) + `</th><th>` + tool.Get_language(db, "state", true) + `</th></tr></thead><tbody>` + list_html + `</tbody></table></div>`

	menu := [][]any{
		{"auth/ban", tool.Get_language(db, "ban", true)},
		{"manager", tool.Get_language(db, "return", true)},
	}

	return tool.Get_template(db, config, tool.Get_language(db, "warn", true), data_html, []any{}, menu, map[string]string{})
}

// View_warn_check : 사용자가 받은 경고 확인 처리
func View_warn_check(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if tool.IP_or_user(config.IP) {
		return tool.Get_redirect("/")
	}

	tool.Do_warn_check(db, config.IP)

	return tool.Get_redirect("/warn")
}

// View_user_warn : 내가 받은 경고 목록
func View_user_warn(config tool.Config, user_name string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if user_name == "" {
		user_name = config.IP
	}

	if user_name != config.IP && !tool.Check_acl(db, "", "", "ban_auth", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	if tool.IP_or_user(user_name) {
		return tool.Get_error_page(db, config, "not found")
	}

	warn_list := tool.Get_warn_list(db, user_name, false)

	list_html := ""
	for _, warn_data := range warn_list {
		state := tool.Get_language(db, "checked", true)
		if warn_data[3] == "1" {
			state = tool.Get_language(db, "unchecked", true)
		}

		list_html += `<tr><td>` + tool.HTML_escape(warn_data[0]) + `</td><td>` + tool.HTML_escape(warn_data[1]) + `</td><td>` + tool.IP_parser(db, warn_data[2], config.IP) + `</td><td>` + state + `</td></tr>`
	}

	if list_html == "" {
		list_html = `<tr><td colspan="4" class="namu_table_empty">` + tool.Get_language(db, "warn_received_empty", true) + `</td></tr>`
	}

	data_html := `<div class="namu_table_wrap"><table class="namu_table"><thead><tr><th>` + tool.Get_language(db, "why", true) + `</th><th>` + tool.Get_language(db, "time", true) + `</th><th>` + tool.Get_language(db, "writer", true) + `</th><th>` + tool.Get_language(db, "state", true) + `</th></tr></thead><tbody>` + list_html + `</tbody></table></div>`

	menu := [][]any{
		{"user", tool.Get_language(db, "return", true)},
	}

	return tool.Get_template(db, config, user_name, data_html, []any{"(" + tool.Get_language(db, "warn", true) + ")"}, menu, map[string]string{})
}
