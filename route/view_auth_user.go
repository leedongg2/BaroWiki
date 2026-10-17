package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// Lang_auth_name : 권한 표시 이름 열쇠말
func Lang_auth_name(key string) string {
	return tool.Lang("auth_name_" + key)
}

// auth_choice_label : 권한 표시 이름
func auth_choice_label(key string) string {
	label_map := map[string]string{
		"owner":                       Lang_auth_name("owner"),
		"admin":                       Lang_auth_name("admin"),
		"user":                        Lang_auth_name("user"),
		"ip":                          Lang_auth_name("ip"),
		"captcha_pass":                Lang_auth_name("captcha_pass"),
		"no_captcha":                  Lang_auth_name("no_captcha"),
		"coop_view":                   Lang_auth_name("coop_view"),
		"coop_open":                   Lang_auth_name("coop_open"),
		"coop_join":                   Lang_auth_name("coop_join"),
		"coop_comment":                Lang_auth_name("coop_comment"),
		"coop_notice":                 Lang_auth_name("coop_notice"),
		"coop_block":                  Lang_auth_name("coop_block"),
		"document":                    Lang_auth_name("document"),
		"edit":                        Lang_auth_name("edit"),
		"move":                        Lang_auth_name("move"),
		"new_make":                    Lang_auth_name("new_make"),
		"delete":                      Lang_auth_name("delete"),
		"view":                        Lang_auth_name("view"),
		"discuss":                     Lang_auth_name("discuss"),
		"discuss_make_new_thread":     Lang_auth_name("discuss_make_new_thread"),
		"discuss_view":                Lang_auth_name("discuss_view"),
		"upload":                      Lang_auth_name("upload"),
		"vote":                        Lang_auth_name("vote"),
		"bbs_use":                     Lang_auth_name("bbs_use"),
		"bbs_edit":                    Lang_auth_name("bbs_edit"),
		"bbs_comment":                 Lang_auth_name("bbs_comment"),
		"bbs_view":                    Lang_auth_name("bbs_view"),
		"captcha_one_check_five_pass": Lang_auth_name("captcha_one_check_five_pass"),
		"edit_filter_view":            Lang_auth_name("edit_filter_view"),
		"ban":                         Lang_auth_name("ban"),
		"toron":                       Lang_auth_name("toron"),
		"check":                       Lang_auth_name("check"),
		"view_user_watchlist":         Lang_auth_name("view_user_watchlist"),
		"acl":                         Lang_auth_name("acl"),
		"hidel":                       Lang_auth_name("hidel"),
		"give":                        Lang_auth_name("give"),
		"bbs":                         Lang_auth_name("bbs"),
		"vote_fix":                    Lang_auth_name("vote_fix"),
		"admin_default_feature":       Lang_auth_name("admin_default_feature"),
		"doc_watch_list_view":         Lang_auth_name("doc_watch_list_view"),
		"treat_as_admin":              Lang_auth_name("treat_as_admin"),
		"view_hide_user_name":         Lang_auth_name("view_hide_user_name"),
		"user_name_bold":              Lang_auth_name("user_name_bold"),
		"multiple_upload":             Lang_auth_name("multiple_upload"),
		"slow_edit_pass":              Lang_auth_name("slow_edit_pass"),
		"edit_bottom_compulsion_pass": Lang_auth_name("edit_bottom_compulsion_pass"),
		"edit_filter_pass":            Lang_auth_name("edit_filter_pass"),
		"nothing":                     Lang_auth_name("nothing"),
	}

	if label, ok := label_map[key]; ok {
		return label
	}

	return key
}

// View_auth_user : 사용자별 권한 관리 (권한 그룹 + 개별 권한)
func View_auth_user(config tool.Config, user_name string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Check_acl(db, "", "", "give_auth", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	if user_name == "" && values != nil {
		user_name = strings.TrimSpace(values.Get("user_name"))
		if user_name != "" {
			return tool.Get_redirect("/auth/user/" + tool.Url_parser(user_name))
		}
	}

	if user_name == "" {
		data_html := `<form method="post" class="namu_form"><h2 class="namu_form_title">` + tool.Get_language(db, "user_name", true) + `</h2><input class="namu_input" name="user_name" placeholder="` + tool.Get_language(db, "user_name", true) + `"><hr class="main_hr"><button class="namu_button" type="submit">` + tool.Get_language(db, "go", true) + `</button></form>`
		data_html += auth_user_admin_list(db, config)

		return tool.Get_template(db, config, tool.Get_language(db, "user_auth_manage", true), data_html, []any{}, [][]any{{"manager", tool.Get_language(db, "return", true)}}, map[string]string{})
	}

	exist_user := ""
	if !tool.QueryRow_DB(db, "select id from user_set where id = ? limit 1", []any{&exist_user}, user_name) {
		return tool.Get_error_page(db, config, "not found")
	}

	groups := auth_groups(db)

	if values != nil {
		change_auth := values.Get("change_auth")
		if change_auth != "" && tool.Arr_in_str(groups, change_auth) && change_auth != tool.Get_user_auth(db, user_name) {
			result := Api_give_auth_patch(config, "", change_auth, user_name)
			if result["response"] != "ok" {
				return tool.Get_error_page(db, config, "auth")
			}
		}

		// 개별 권한은 소유자만 조정 가능
		if tool.Check_acl(db, "", "", "owner_auth", config.IP) {
			auth_plus := []string{}
			for _, choice := range auth_choices() {
				if choice.key == "owner" {
					continue
				}

				if values.Get("plus_"+choice.key) != "" {
					auth_plus = append(auth_plus, choice.key)
				}
			}

			tool.Do_set_user_auth_plus(db, user_name, auth_plus)
			tool.Do_insert_auth_history(db, config.IP, "auth_plus ("+user_name+") -> "+strings.Join(auth_plus, ","))
		}

		return tool.Get_redirect("/auth/user/" + tool.Url_parser(user_name))
	}

	now_group := tool.Get_user_auth(db, user_name)
	auth_plus_list := tool.Get_user_auth_plus(db, user_name)
	auth_plus_map := map[string]bool{}
	for _, auth_name := range auth_plus_list {
		auth_plus_map[auth_name] = true
	}

	auth_info := tool.Get_user_auth_info(db, user_name)
	now_auth_list := []string{}
	for key, value := range auth_info {
		if value {
			now_auth_list = append(now_auth_list, key)
		}
	}
	sort.Strings(now_auth_list)

	data_html := `<div class="namu_user_auth_head">` + tool.Get_language(db, "auth_manage_of", false) + `</div>`
	data_html = strings.ReplaceAll(data_html, "{0}", `<b>`+tool.HTML_escape(user_name)+`</b>`)

	data_html += `<form method="post" class="namu_form">`
	data_html += `<h2 class="namu_form_title">` + tool.Get_language(db, "admin_group", true) + `</h2>` + auth_select("change_auth", groups, now_group)
	data_html += `<div class="namu_form_help">` + tool.Get_language(db, "auth_group_help", false) + `</div>`

	if tool.Check_acl(db, "", "", "owner_auth", config.IP) {
		data_html += `<h2 class="namu_form_title">` + tool.Get_language(db, "auth_add_single", true) + `</h2>`
		data_html += `<div class="namu_form_help">` + tool.Get_language(db, "auth_add_single_help", false) + `</div>`
		data_html += `<div class="namu_auth_grid">`
		for _, choice := range auth_choices() {
			if choice.key == "owner" || choice.key == "nothing" {
				continue
			}

			checked := ""
			if auth_plus_map[choice.key] {
				checked = ` checked`
			}

			data_html += `<label class="namu_check_label" style="margin-left:` + strconv.Itoa((choice.level-1)*12) + `px"><input type="checkbox" name="plus_` + choice.key + `"` + checked + `> ` + tool.HTML_escape(auth_choice_label(choice.key)) + `</label>`
		}
		data_html += `</div>`
	}

	data_html += `<hr class="main_hr"><button class="namu_button" type="submit">` + tool.Get_language(db, "save", true) + `</button></form>`

	data_html += `<h2 class="namu_form_title">` + tool.Get_language(db, "auth_now", true) + `</h2><div class="namu_auth_now">`
	for _, auth_name := range now_auth_list {
		data_html += `<span class="namu_auth_tag">` + tool.HTML_escape(auth_choice_label(auth_name)) + `</span>`
	}
	data_html += `</div>`

	menu := [][]any{
		{"w/" + tool.Url_parser("user:"+user_name), tool.Get_language(db, "user_document", true)},
		{"auth/ban/" + tool.Url_parser(user_name), tool.Get_language(db, "ban", true)},
		{"auth/warn/" + tool.Url_parser(user_name), tool.Get_language(db, "warn", true)},
		{"manager", tool.Get_language(db, "admin", true)},
	}

	return tool.Get_template(db, config, user_name, data_html, []any{"(" + tool.Get_language(db, "auth_manage", true) + ")"}, menu, map[string]string{})
}

// auth_user_admin_list : 권한 그룹이 지정된 사용자 목록
func auth_user_admin_list(db *sql.DB, config tool.Config) string {
	rows := tool.Query_DB(
		db,
		"select id, data from user_set where name = 'acl' and data != 'user' order by data asc, id asc limit 200",
	)
	defer rows.Close()

	list_html := ""
	for rows.Next() {
		id := ""
		group := ""

		if rows.Scan(&id, &group) != nil {
			continue
		}

		list_html += `<tr><td><a href="/auth/user/` + tool.Url_parser(id) + `">` + tool.HTML_escape(id) + `</a></td><td>` + tool.HTML_escape(group) + `</td><td><a class="namu_button_small" href="/auth/user/` + tool.Url_parser(id) + `">` + tool.Get_language(db, "authority", true) + `</a> <a class="namu_button_small" href="/auth/ban/` + tool.Url_parser(id) + `">` + tool.Get_language(db, "ban", true) + `</a></td></tr>`
	}

	if list_html == "" {
		list_html = `<tr><td colspan="3" class="namu_table_empty">` + tool.Get_language(db, "auth_user_empty", true) + `</td></tr>`
	}

	return `<h2 class="namu_form_title">` + tool.Get_language(db, "auth_user_list", true) + `</h2><div class="namu_table_wrap"><table class="namu_table"><thead><tr><th>` + tool.Get_language(db, "user", true) + `</th><th>` + tool.Get_language(db, "admin_group", true) + `</th><th>` + tool.Get_language(db, "manage_col", true) + `</th></tr></thead><tbody>` + list_html + `</tbody></table></div>`
}
