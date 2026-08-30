package route

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

// trash_can_manage : 휴지통 관리 권한
func trash_can_manage(config tool.Config, doc_name string) bool {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if tool.Check_acl(db, "", "", "all_admin_auth", config.IP) {
		return true
	}

	return tool.Check_acl(db, doc_name, "", "document_delete", config.IP)
}

// View_trash_list : 휴지통 목록
func View_trash_list(config tool.Config, page string, mode string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	trash_prefix := "trash:"
	title := tool.Get_language(db, "trash", true)
	if mode == "file" {
		trash_prefix = "filetrash:"
		title = tool.Get_language(db, "filetrash", true)
	}

	namespace := tool.Namespace_by_prefix(trash_prefix)
	if !tool.Check_acl(db, namespace.Prefix+"__", "", "render", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	page_int := tool.Str_to_int(page)
	if page_int < 1 {
		page_int = 1
	}

	trash_list := tool.Get_trash_list(db, trash_prefix, (page_int-1)*50, 50)

	data_html := `<div class="namu_trash_head">` + tool.Get_language(db, "trash_help", false) + `</div>`
	data_html += `<div class="namu_table_wrap"><table class="namu_table"><thead><tr><th>` + tool.Get_language(db, "trash_origin_doc", true) + `</th><th>` + tool.Get_language(db, "trash_doc", true) + `</th><th>` + tool.Get_language(db, "delete_time", true) + `</th><th>` + tool.Get_language(db, "deleter", true) + `</th><th>` + tool.Get_language(db, "why", true) + `</th><th>` + tool.Get_language(db, "manage_col", true) + `</th></tr></thead><tbody>`

	if len(trash_list) == 0 {
		data_html += `<tr><td colspan="6" class="namu_table_empty">` + tool.Get_language(db, "trash_empty", true) + `</td></tr>`
	}

	for _, trash_data := range trash_list {
		origin_name := trash_data[0]
		trash_title := trash_data[1]

		origin_html := "-"
		if origin_name != "" {
			origin_html = `<a href="/w/` + tool.Url_parser(origin_name) + `">` + tool.HTML_escape(tool.Namespace_display(origin_name)) + `</a>`
		}

		data_html += `<tr>`
		data_html += `<td>` + origin_html + `</td>`
		data_html += `<td><a href="/w/` + tool.Url_parser(trash_title) + `">` + tool.HTML_escape(tool.Namespace_display(trash_title)) + `</a></td>`
		data_html += `<td>` + tool.HTML_escape(trash_data[2]) + `</td>`
		data_html += `<td>` + tool.IP_parser(db, trash_data[3], config.IP) + `</td>`
		data_html += `<td>` + tool.HTML_escape(trash_data[4]) + `</td>`
		data_html += `<td><a class="namu_button_small" href="/trash_restore/` + tool.Url_parser(trash_title) + `">` + tool.Get_language(db, "restore", true) + `</a> <a class="namu_button_small namu_button_danger" href="/trash_delete/` + tool.Url_parser(trash_title) + `">` + tool.Get_language(db, "trash_delete", true) + `</a></td>`
		data_html += `</tr>`
	}

	data_html += `</tbody></table></div>`

	list_url := "/trash/{}"
	if mode == "file" {
		list_url = "/file_trash/{}"
	}
	data_html += tool.Get_page_control(db, page_int, len(trash_list), 50, list_url)

	menu := [][]any{
		{"trash/1", tool.Get_language(db, "trash", true)},
		{"file_trash/1", tool.Get_language(db, "filetrash", true)},
		{"manager", tool.Get_language(db, "admin", true)},
	}

	return tool.Get_template(db, config, title, data_html, []any{}, menu, map[string]string{})
}

// View_trash_restore : 휴지통 문서 복원
func View_trash_restore(config tool.Config, trash_title string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	namespace := tool.Namespace_of(trash_title)
	if namespace.Prefix != "trash:" && namespace.Prefix != "filetrash:" {
		return tool.Get_error_page(db, config, "invalid data")
	}

	trash_data := ""
	if !tool.QueryRow_DB(db, "select data from data where title = ?", []any{&trash_data}, trash_title) {
		return tool.Get_error_page(db, config, "not found")
	}

	origin_name := tool.Get_trash_info(db, trash_title, "trash_origin")
	if origin_name == "" {
		return tool.Get_error_page(db, config, "invalid data")
	}

	if !trash_can_manage(config, origin_name) {
		return tool.Get_error_page(db, config, "auth")
	}

	origin_exist := ""
	already_exist := tool.QueryRow_DB(db, "select title from data where title = ?", []any{&origin_exist}, origin_name)

	if values != nil {
		if already_exist {
			return tool.Get_error_page(db, config, "already exist")
		}

		tool.Exec_DB(db, "insert into data (title, data, type) values (?, ?, '')", origin_name, trash_data)
		tool.Do_add_history(
			db,
			origin_name,
			trash_data,
			tool.Get_time(),
			config.IP,
			values.Get("send"),
			"+"+strconv.Itoa(tool.Get_len(trash_data)),
			"",
			tool.Get_language(db, "trash_restore", true),
		)
		markup.Get_render(db, origin_name, trash_data, "backlink")

		// 파일 문서라면 실제 파일도 복원
		if strings.HasPrefix(origin_name, "file:") {
			restore_file_from_trash(db, origin_name)
		}

		tool.Do_trash_delete(db, trash_title)
		tool.Do_insert_auth_history(db, config.IP, "trash_restore ("+origin_name+")")

		return tool.Get_redirect("/w/" + tool.Url_parser(origin_name))
	}

	data_html := `<div class="namu_trash_head">` + tool.Get_language(db, "trash_restore_help", true) + `</div>`
	data_html += `<ul class="namu_info_list">`
	data_html += `<li>` + tool.Get_language(db, "trash_origin_doc", true) + ` : <b>` + tool.HTML_escape(tool.Namespace_display(origin_name)) + `</b></li>`
	data_html += `<li>` + tool.Get_language(db, "delete_time", true) + ` : ` + tool.HTML_escape(tool.Get_trash_info(db, trash_title, "trash_date")) + `</li>`
	data_html += `<li>` + tool.Get_language(db, "delete_reason", true) + ` : ` + tool.HTML_escape(tool.Get_trash_info(db, trash_title, "trash_why")) + `</li>`
	data_html += `</ul>`

	if already_exist {
		data_html += `<div class="namu_alert namu_alert_danger">` + tool.Get_language(db, "trash_restore_conflict", true) + `</div>`
	} else {
		data_html += `<form method="post"><input name="send" placeholder="` + tool.Get_language(db, "why", true) + `"><hr class="main_hr"><button type="submit">` + tool.Get_language(db, "restore", true) + `</button></form>`
	}

	menu := [][]any{
		{"w/" + tool.Url_parser(trash_title), tool.Get_language(db, "return", true)},
		{"trash/1", tool.Get_language(db, "trash", true)},
	}

	return tool.Get_template(db, config, trash_title, data_html, []any{"(" + tool.Get_language(db, "restore", true) + ")"}, menu, map[string]string{})
}

// View_trash_delete : 휴지통 문서 완전 삭제
func View_trash_delete(config tool.Config, trash_title string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	namespace := tool.Namespace_of(trash_title)
	if namespace.Prefix != "trash:" && namespace.Prefix != "filetrash:" {
		return tool.Get_error_page(db, config, "invalid data")
	}

	exist_title := ""
	if !tool.QueryRow_DB(db, "select title from data where title = ?", []any{&exist_title}, trash_title) {
		return tool.Get_error_page(db, config, "not found")
	}

	if !tool.Check_acl(db, "", "", "all_admin_auth", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	origin_name := tool.Get_trash_info(db, trash_title, "trash_origin")

	if values != nil {
		if strings.HasPrefix(origin_name, "file:") {
			remove_file_in_trash(db, origin_name)
		}

		tool.Do_trash_delete(db, trash_title)
		tool.Do_insert_auth_history(db, config.IP, "trash_delete ("+trash_title+")")

		if namespace.Prefix == "filetrash:" {
			return tool.Get_redirect("/file_trash/1")
		}

		return tool.Get_redirect("/trash/1")
	}

	data_html := `<div class="namu_alert namu_alert_danger">` + tool.Get_language(db, "trash_delete_warn", true) + `</div>`
	data_html += `<ul class="namu_info_list"><li>` + tool.Get_language(db, "trash_doc", true) + ` : <b>` + tool.HTML_escape(tool.Namespace_display(trash_title)) + `</b></li></ul>`
	data_html += `<form method="post"><button type="submit" class="namu_button_danger">` + tool.Get_language(db, "trash_delete", true) + `</button></form>`

	menu := [][]any{
		{"trash/1", tool.Get_language(db, "trash", true)},
	}

	return tool.Get_template(db, config, trash_title, data_html, []any{"(" + tool.Get_language(db, "trash_delete", true) + ")"}, menu, map[string]string{})
}

// file_trash_dir : 삭제된 파일 보관 폴더
func file_trash_dir(main_dir string) string {
	return filepath.Join(main_dir, "trash")
}

// file_trash_path : 파일 문서에 대응하는 (원본 경로, 휴지통 경로)
func file_trash_path(db *sql.DB, doc_name string) (string, string, bool) {
	main_dir := tool.Get_file_main_dir(db)

	file_path, valid := file_storage_path(main_dir, doc_name)
	if !valid {
		return "", "", false
	}

	return file_path, filepath.Join(file_trash_dir(main_dir), filepath.Base(file_path)), true
}

// Do_move_file_to_trash : 실제 파일을 파일휴지통 폴더로 이동
func Do_move_file_to_trash(db *sql.DB, doc_name string) bool {
	file_path, trash_path, valid := file_trash_path(db, doc_name)
	if !valid {
		return false
	}

	if _, err := os.Stat(file_path); err != nil {
		return false
	}

	if err := os.MkdirAll(filepath.Dir(trash_path), 0o755); err != nil {
		return false
	}

	_ = os.Remove(trash_path)

	return os.Rename(file_path, trash_path) == nil
}

// restore_file_from_trash : 파일휴지통 폴더의 파일을 원래 위치로 복원
func restore_file_from_trash(db *sql.DB, doc_name string) bool {
	file_path, trash_path, valid := file_trash_path(db, doc_name)
	if !valid {
		return false
	}

	if _, err := os.Stat(trash_path); err != nil {
		return false
	}

	if _, err := os.Stat(file_path); err == nil {
		return false
	}

	if err := os.MkdirAll(filepath.Dir(file_path), 0o755); err != nil {
		return false
	}

	return os.Rename(trash_path, file_path) == nil
}

// remove_file_in_trash : 파일휴지통 폴더의 파일 완전 삭제
func remove_file_in_trash(db *sql.DB, doc_name string) bool {
	_, trash_path, valid := file_trash_path(db, doc_name)
	if !valid {
		return false
	}

	if err := os.Remove(trash_path); err != nil && !os.IsNotExist(err) {
		return false
	}

	return true
}
