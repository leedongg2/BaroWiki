package route

import (
	"database/sql"
	"net/url"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// namespace_acl_label : ACL 종류 표시 이름
func namespace_acl_label(acl_type string) string {
	switch acl_type {
	case "view":
		return tool.Lang("acl_type_view")
	case "decu":
		return tool.Lang("acl_type_decu")
	case "document_edit_acl":
		return tool.Lang("edit")
	case "document_move_acl":
		return tool.Lang("move")
	case "document_delete_acl":
		return tool.Lang("delete")
	case "dis":
		return tool.Lang("discussion")
	case "acl_edit":
		return "ACL"
	}

	return acl_type
}

// namespace_acl_value_label : ACL 값 표시 이름
func namespace_acl_value_label(acl_data string) string {
	switch acl_data {
	case "":
		return tool.Lang("default")
	case "all":
		return tool.Lang("acl_value_all")
	case "user":
		return tool.Lang("acl_value_user")
	case "admin":
		return tool.Lang("acl_value_admin")
	case "owner":
		return tool.Lang("acl_value_owner")
	case "50_edit":
		return tool.Lang("acl_value_50_edit")
	case "email":
		return tool.Lang("acl_value_email")
	case "ban":
		return tool.Lang("acl_value_ban")
	case "before":
		return tool.Lang("acl_value_before")
	case "30_day":
		return tool.Lang("acl_value_30_day")
	case "90_day":
		return tool.Lang("acl_value_90_day")
	case "ban_admin":
		return tool.Lang("acl_value_ban_admin")
	case "not_all":
		return tool.Lang("acl_value_not_all")
	case "up_to_level_3":
		return tool.Lang("up_to_level_3")
	case "up_to_level_10":
		return tool.Lang("up_to_level_10")
	case "30_day_50_edit":
		return tool.Lang("acl_value_30_day_50_edit")
	}

	return acl_data
}

// namespace_acl_select : ACL 규칙 편집 UI
//
//	문서 ACL 과 같은 모양으로 규칙을 여러 줄 적는다.
func namespace_acl_select(db *sql.DB, name string, selected string) string {
	return acl_rule_editor(db, name, selected)
}

// View_namespace_list : 이름공간 목록
func View_namespace_list(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	can_acl := tool.Check_acl(db, "", "", "acl_auth", config.IP)

	language := tool.Wiki_language(db)

	data_html := `<div class="namu_table_wrap"><table class="namu_table"><thead><tr><th>` + tool.Get_language(db, "namespace", true) + `</th><th>` + tool.Get_language(db, "namespace_other_name", true) + `</th><th>` + tool.Get_language(db, "namespace_prefix", true) + `</th><th>` + tool.Get_language(db, "document_count", true) + `</th><th>` + tool.Get_language(db, "tool", true) + `</th></tr></thead><tbody>`
	for _, namespace := range tool.Namespace_list() {
		show_name := tool.Namespace_name(namespace, language)

		prefix_text := show_name + ":"
		if namespace.Prefix == "" {
			prefix_text = "-"
		}

		// 다른 언어판에서 쓰는 이름도 같이 보여 준다. (세 언어판이 같은 이름공간을 쓴다)
		other_name_list := []string{}
		for _, one_language := range tool.Namespace_language_list() {
			if one_language == language {
				continue
			}

			other_name_list = append(other_name_list, tool.Namespace_name(namespace, one_language))
		}

		tool_html := `<a class="namu_button_small" href="/namespace/` + tool.Url_parser(show_name) + `/1">` + tool.Get_language(db, "document_list", true) + `</a>`
		if can_acl {
			tool_html += ` <a class="namu_button_small" href="/namespace_acl/` + tool.Url_parser(show_name) + `">ACL</a>`
		}

		data_html += `<tr>`
		data_html += `<td><b>` + tool.HTML_escape(show_name) + `</b></td>`
		data_html += `<td>` + tool.HTML_escape(strings.Join(other_name_list, " / ")) + `</td>`
		data_html += `<td>` + tool.HTML_escape(prefix_text) + `</td>`
		data_html += `<td>` + strconv.Itoa(tool.Namespace_document_count(db, namespace)) + `</td>`
		data_html += `<td>` + tool_html + `</td>`
		data_html += `</tr>`
	}
	data_html += `</tbody></table></div>`

	menu := [][]any{
		{"other", tool.Get_language(db, "return", true)},
	}
	if can_acl {
		menu = append(menu, []any{"manager", tool.Get_language(db, "admin", true)})
	}

	return tool.Get_template(db, config, tool.Get_language(db, "namespace", true), data_html, []any{}, menu, map[string]string{})
}

// View_namespace_document_list : 이름공간별 문서 목록
func View_namespace_document_list(config tool.Config, namespace_name string, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Namespace_exist_name(namespace_name) {
		return tool.Get_error_page(db, config, "not found")
	}

	namespace := tool.Namespace_by_name(namespace_name)
	page_num := list_extra_page_number(page)
	offset := (page_num - 1) * 50

	var rows *sql.Rows
	if namespace.Prefix == "" {
		rows = tool.Query_DB(
			db,
			"select title from data where "+tool.Get_except_document_name_SQL("title")+" order by title asc limit ?, 50",
			offset,
		)
	} else {
		rows = tool.Query_DB(
			db,
			"select title from data where title like ? order by title asc limit ?, 50",
			namespace.Prefix+"%",
			offset,
		)
	}

	body := strings.Builder{}
	count := 0
	for rows.Next() {
		title := ""
		if rows.Scan(&title) != nil {
			continue
		}

		if namespace.Prefix != "" && tool.Namespace_of(title).Prefix != namespace.Prefix {
			continue
		}

		// 읽기 권한이 없는 문서는 제외
		if !tool.Check_acl(db, title, "", "render", config.IP) {
			continue
		}

		body.WriteString(`<li><a href="/w/` + tool.Url_parser(title) + `">` + tool.HTML_escape(tool.Namespace_display(title)) + `</a></li>`)
		count++
	}
	rows.Close()

	show_name := tool.Namespace_name(namespace, tool.Wiki_language(db))

	data_html := `<div class="namu_namespace_info"><b>` + tool.HTML_escape(show_name) + `</b> — ` + tool.Get_language(db, "document_list", true) + `</div>`
	if count == 0 {
		data_html += `<div class="namu_alert">` + tool.Get_language(db, "no_document", true) + `</div>`
	} else {
		data_html += `<ul class="namu_doc_list">` + body.String() + `</ul>`
	}

	data_html += tool.Get_page_control(db, page_num, count, 50, "/namespace/"+tool.Url_parser(show_name)+"/{}")

	menu := [][]any{
		{"namespace", tool.Get_language(db, "namespace", true)},
	}
	if tool.Check_acl(db, "", "", "acl_auth", config.IP) {
		menu = append(menu, []any{"namespace_acl/" + tool.Url_parser(show_name), "ACL"})
	}

	return tool.Get_template(db, config, show_name, data_html, []any{"(" + tool.Get_language(db, "namespace", true) + ")"}, menu, map[string]string{})
}

// View_namespace_acl : 이름공간 ACL 설정
func View_namespace_acl(config tool.Config, namespace_name string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Check_acl(db, "", "", "acl_auth", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	if !tool.Namespace_exist_name(namespace_name) {
		return tool.Get_error_page(db, config, "not found")
	}

	namespace := tool.Namespace_by_name(namespace_name)

	if values != nil {
		for _, acl_type := range tool.Namespace_acl_type_list() {
			tool.Do_set_namespace_acl(db, namespace, acl_type, values.Get(acl_type))
		}

		tool.Do_insert_auth_history(db, config.IP, "namespace_acl ("+namespace.Name+")")

		return tool.Get_redirect("/namespace_acl/" + tool.Url_parser(namespace.Name))
	}

	data_html := `<div class="namu_namespace_info">` + tool.Get_language(db, "namespace_acl_help", false) + `</div>`
	data_html += `<form method="post"><div class="namu_table_wrap"><table class="namu_table"><thead><tr><th>` + tool.Get_language(db, "authority", true) + `</th><th>` + tool.Get_language(db, "setting", true) + `</th><th>` + tool.Get_language(db, "default", true) + `</th></tr></thead><tbody>`

	for _, acl_type := range tool.Namespace_acl_type_list() {
		default_acl := tool.Get_language(db, "default", true)
		if value, ok := namespace.Defaults[acl_type]; ok {
			default_acl = namespace_acl_value_label(value)
		}

		data_html += `<tr>`
		data_html += `<td>` + tool.HTML_escape(namespace_acl_label(acl_type)) + `</td>`
		data_html += `<td>` + namespace_acl_select(db, acl_type, tool.Get_namespace_acl(db, namespace, acl_type)) + `</td>`
		data_html += `<td>` + tool.HTML_escape(default_acl) + `</td>`
		data_html += `</tr>`
	}

	data_html += `</tbody></table></div>` + acl_rule_help(db)
	data_html += `<hr class="main_hr"><button class="namu_button" type="submit">` + tool.Get_language(db, "save", true) + `</button></form>`

	acl_show_name := tool.Namespace_name(namespace, tool.Wiki_language(db))

	menu := [][]any{
		{"namespace", tool.Get_language(db, "namespace", true)},
		{"namespace/" + tool.Url_parser(acl_show_name) + "/1", tool.Get_language(db, "document_list", true)},
		{"manager", tool.Get_language(db, "admin", true)},
	}

	return tool.Get_template(db, config, acl_show_name, data_html, []any{"(" + tool.Get_language(db, "namespace", true) + " ACL)"}, menu, map[string]string{})
}
