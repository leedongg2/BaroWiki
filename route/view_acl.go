package route

import (
	"database/sql"
	"net/url"
	"regexp"
	"strings"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

var document_acl_fields = tool.Namespace_acl_type_list()

func acl_field_title(db *sql.DB, field string) string {
	return namespace_acl_label(field)
}

func acl_value(db *sql.DB, doc_name string, field string) string {
	value := ""
	tool.QueryRow_DB(db, "select data from acl where title = ? and type = ? limit 1", []any{&value}, doc_name, field)
	return value
}

func save_acl(db *sql.DB, doc_name string, values url.Values) {
	for _, field := range document_acl_fields {
		tool.Exec_DB(db, "delete from acl where title = ? and type = ?", doc_name, field)
		tool.Exec_DB(db, "insert into acl (title, data, type) values (?, ?, ?)", doc_name, values.Get(field), field)

		tool.Exec_DB(db, "delete from data_set where doc_name = ? and doc_rev = ? and set_name = 'acl_date'", doc_name, field)
		date_value := values.Get(field + "_date")
		if regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`).MatchString(date_value) {
			tool.Exec_DB(db, "insert into data_set (doc_name, doc_rev, set_name, set_data) values (?, ?, 'acl_date', ?)", doc_name, field, date_value)
		}
	}

	tool.Exec_DB(db, "delete from data_set where doc_name = ? and set_name = 'document_markup'", doc_name)
	tool.Exec_DB(db, "insert into data_set (doc_name, doc_rev, set_name, set_data) values (?, '', 'document_markup', ?)", doc_name, values.Get("document_markup"))
	for _, field := range []string{"document_top", "document_editor_top"} {
		tool.Exec_DB(db, "delete from data_set where doc_name = ? and set_name = ?", doc_name, field)
		tool.Exec_DB(db, "insert into data_set (doc_name, doc_rev, set_name, set_data) values (?, '', ?, ?)", doc_name, field, values.Get(field))
	}

	tool.Exec_DB(db, "delete from acl where title = ? and type = 'why'", doc_name)
	tool.Exec_DB(db, "insert into acl (title, data, type) values (?, ?, 'why')", doc_name, values.Get("why"))
}

func View_acl(config tool.Config, doc_name string, multiple bool, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if values != nil && doc_name == "" && !multiple {
		doc_name = values.Get("name")
	}
	if doc_name == "" && !multiple && values == nil {
		data := `<form method="post"><input name="name" placeholder="` + tool.Get_language(db, "document_name", true) + `"><hr class="main_hr"><button type="submit">` + tool.Get_language(db, "go", true) + `</button></form>`
		return tool.Get_template(db, config, tool.Get_language(db, "document_setting", true), data, []any{}, [][]any{{"manager", tool.Get_language(db, "return", true)}}, map[string]string{})
	}

	allowed := tool.Check_acl(db, "", "", "acl_auth", config.IP)
	if !allowed && !multiple && doc_name != "" {
		// 문서별 ACL 편집 권한 (ACL 편집 항목) 확인
		allowed = tool.Check_acl(db, doc_name, "", "document_acl_edit", config.IP)
	}
	if strings.HasPrefix(doc_name, "user:") && strings.TrimPrefix(doc_name, "user:") == config.IP {
		allowed = true
	}
	if !allowed {
		return tool.Get_error_page(db, config, "auth")
	}

	if values != nil {
		if multiple {
			for _, name := range strings.Split(strings.ReplaceAll(values.Get("title_name"), "\r", ""), "\n") {
				name = strings.TrimSpace(name)
				if name != "" {
					save_acl(db, name, values)
					acl_history(db, config, name, values)
				}
			}
			return tool.Get_redirect("/manager")
		}
		if doc_name == "" {
			return tool.Get_redirect("/acl")
		}
		old_markup := document_set_value(db, doc_name, "document_markup")
		if old_markup == "" {
			old_markup = tool.Get_document_markup(db, "", "document")
		}
		save_values := values
		if tool.Check_acl(db, "", "", "owner_auth", config.IP) {
			save_values = url.Values{}
			for key, value := range values {
				save_values[key] = append([]string{}, value...)
			}
			save_values.Set("document_top", document_set_value(db, doc_name, "document_top"))
			save_values.Set("document_editor_top", document_set_value(db, doc_name, "document_editor_top"))
		}
		save_acl(db, doc_name, save_values)
		new_markup := document_set_value(db, doc_name, "document_markup")
		if new_markup == "" {
			new_markup = tool.Get_document_markup(db, "", "document")
		}
		if old_markup != new_markup {
			data := ""
			if tool.QueryRow_DB(db, "select data from data where title = ?", []any{&data}, doc_name) {
				markup.Get_render(db, doc_name, data, "backlink")
			}
		}
		acl_history(db, config, doc_name, save_values)
		return tool.Get_redirect("/acl/" + tool.Url_parser(doc_name))
	}

	data := `<form method="post">`
	if multiple {
		data += `<textarea class="opennamu_textarea_500" name="title_name" placeholder="` + tool.Get_language(db, "many_delete_help", true) + `"></textarea><hr class="main_hr">`
	} else {
		data += `<input type="hidden" name="name" value="` + tool.HTML_escape(doc_name) + `">`
	}

	data += `<div class="namu_table_wrap"><table class="namu_table namu_acl_table"><thead><tr><th>` + tool.Get_language(db, "authority", true) + `</th><th>` + tool.Get_language(db, "condition", true) + `</th><th>` + tool.Get_language(db, "expire_date", true) + `</th><th>` + tool.Get_language(db, "namespace_default", true) + `</th></tr></thead><tbody>`
	namespace := tool.Namespace_of(doc_name)
	for _, field := range document_acl_fields {
		selected := ""
		if !multiple {
			selected = acl_value(db, doc_name, field)
		}

		date_value := ""
		if !multiple {
			date_value = tool.Get_acl_expire(db, doc_name, field)
		}

		namespace_acl := tool.Get_namespace_acl(db, namespace, field)
		if namespace_acl == "" {
			if default_acl, ok := namespace.Defaults[field]; ok {
				namespace_acl = default_acl
			}
		}

		data += `<tr>`
		data += `<td>` + acl_field_title(db, field) + `</td>`
		data += `<td>` + acl_rule_editor(db, field, selected) + `</td>`
		data += `<td><input class="namu_input" type="date" name="` + field + `_date" value="` + tool.HTML_escape(date_value) + `"></td>`
		data += `<td>` + tool.HTML_escape(namespace_acl_value_label(namespace_acl)) + `</td>`
		data += `</tr>`
	}
	data += `</tbody></table></div>`
	data += `<div class="namu_form_help">` + tool.Get_language(db, "acl_condition_help", false) + `</div>`
	data += acl_rule_help(db)

	why := ""
	if !multiple {
		why = acl_value(db, doc_name, "why")
	}
	data += `<h3>` + tool.Get_language(db, "why", true) + `</h3><input name="why" value="` + tool.HTML_escape(why) + `"><hr class="main_hr">`

	markup := ""
	if !multiple {
		markup = tool.Get_document_markup(db, doc_name, "document")
	}
	data += `<h2>` + tool.Get_language(db, "markup", true) + `</h2>` + tool.Get_markup_select_ui(db, config, doc_name, markup, "", "")
	top_disabled := ""
	if tool.Check_acl(db, "", "", "owner_auth", config.IP) {
		top_disabled = ` disabled`
	}
	data += `<h2>` + tool.Get_language(db, "document_top", true) + `</h2><textarea class="opennamu_textarea_100" name="document_top"` + top_disabled + `>` + tool.HTML_escape(document_set_value(db, doc_name, "document_top")) + `</textarea>`
	data += `<h2>` + tool.Get_language(db, "document_editor_top", true) + `</h2><textarea class="opennamu_textarea_100" name="document_editor_top"` + top_disabled + `>` + tool.HTML_escape(document_set_value(db, doc_name, "document_editor_top")) + `</textarea><hr class="main_hr">`
	data += `<button type="submit">` + tool.Get_language(db, "save", true) + `</button></form>`

	menu := [][]any{{"manager", tool.Get_language(db, "admin", true)}}
	if !multiple {
		menu = [][]any{{"w/" + tool.Url_parser(doc_name), tool.Get_language(db, "return", true)}, {"acl_multiple", tool.Get_language(db, "mutiple_document_setting", true)}, {"manager", tool.Get_language(db, "admin", true)}}
	}
	title := doc_name
	if multiple {
		title = tool.Get_language(db, "mutiple_document_setting", true)
	}
	return tool.Get_template(db, config, title, data, []any{}, menu, map[string]string{})
}

func acl_history(db *sql.DB, config tool.Config, doc_name string, values url.Values) {
	data := ""
	for _, field := range append(document_acl_fields, "why", "document_markup") {
		data += field + "\n" + values.Get(field) + "\n\n"
	}
	tool.Do_add_history(db, doc_name, data, tool.Get_time(), config.IP, values.Get("why"), "0", "setting", "")
	tool.Do_insert_auth_history(db, config.IP, "document_set ("+doc_name+")")
}

func document_set_value(db *sql.DB, doc_name string, set_name string) string {
	value := ""
	tool.QueryRow_DB(db, "select set_data from data_set where doc_name = ? and set_name = ? limit 1", []any{&value}, doc_name, set_name)
	return value
}
