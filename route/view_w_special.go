package route

import (
	"database/sql"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

func view_w_redirect_target(db *sql.DB, doc_name string) (string, string) {
	target := ""
	anchor := ""
	tool.QueryRow_DB(
		db,
		"select title, data from back where link = ? and type = 'redirect' limit 1",
		[]any{&target, &anchor},
		doc_name,
	)
	if target != "" {
		target_exists := ""
		if !tool.QueryRow_DB(db, "select title from data where title = ?", []any{&target_exists}, target) {
			return "", ""
		}
	}

	return target, anchor
}

func view_w_child_exists(db *sql.DB, doc_name string) bool {
	child := ""
	return tool.QueryRow_DB(
		db,
		"select title from data where title like ? limit 1",
		[]any{&child},
		doc_name+"/%",
	)
}

func view_w_user_data(db *sql.DB, doc_name string) string {
	user_name := strings.TrimPrefix(doc_name, "user:")
	if slash_index := strings.Index(user_name, "/"); slash_index >= 0 {
		user_name = user_name[:slash_index]
	}

	phrase := ""
	if tool.Get_user_document(db, user_name) && !tool.Check_acl(db, "", "", "all_admin_auth", user_name) {
		phrase_name := "phrase_user_page_owner"
		if tool.Check_acl(db, "", "", "owner_auth", user_name) {
			phrase_name = "phrase_user_page_admin"
		}
		tool.QueryRow_DB(db, "select data from other where name = ?", []any{&phrase}, phrase_name)
	}
	if phrase != "" {
		phrase += "<br>"
	}
	return phrase + `<div id="opennamu_get_user_info">` + tool.HTML_escape(user_name) + `</div><hr class="main_hr">`
}

// view_w_acl_notice : 문서에 걸린 ACL 안내 (나무위키 편집 보호 안내 줄)
func view_w_acl_notice(db *sql.DB, config tool.Config, doc_name string) string {
	notice_list := [][2]string{
		{"view", tool.Lang("acl_type_view")},
		{"document_edit_acl", tool.Lang("edit")},
		{"document_move_acl", tool.Lang("move")},
		{"document_delete_acl", tool.Lang("delete")},
		{"dis", tool.Lang("discussion")},
		{"acl_edit", "ACL"},
	}

	set_list := []string{}
	view_limited := false
	for _, notice_data := range notice_list {
		acl_data, acl_source := tool.Get_document_acl_source(db, doc_name, notice_data[0])
		if acl_data == "" || acl_data == "all" || acl_source == "default" {
			continue
		}

		if notice_data[0] == "view" {
			view_limited = true
		}

		set_list = append(set_list, notice_data[1]+" : "+namespace_acl_value_label(acl_data))
	}

	if len(set_list) == 0 {
		return ""
	}

	notice_text := tool.Lang("doc_protected_edit")
	if view_limited {
		notice_text = tool.Lang("doc_protected_view")
	}

	lock_icon := `<span class="namu_acl_notice_icon"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" width="14" height="14" aria-hidden="true"><path fill="currentColor" d="M144 144l0 48 160 0 0-48c0-44.2-35.8-80-80-80s-80 35.8-80 80zM80 192l0-48C80 64.5 144.5 0 224 0s144 64.5 144 144l0 48 16 0c35.3 0 64 28.7 64 64l0 192c0 35.3-28.7 64-64 64L64 512c-35.3 0-64-28.7-64-64L0 256c0-35.3 28.7-64 64-64l16 0z"/></svg></span>`

	return `<div class="namu_acl_notice" title="` + tool.HTML_escape(strings.Join(set_list, " / ")) + `">` +
		lock_icon + notice_text + ` ` + tool.Lang_esc("acl_tab_notice") + ` <a href="/acl/` + tool.Url_parser(doc_name) + `">ACL</a></div>`
}

// view_w_ipuser_data : 아이피사용자 문서 상단 정보
func view_w_ipuser_data(db *sql.DB, config tool.Config, doc_name string) string {
	ip_name := strings.TrimPrefix(doc_name, "ipuser:")
	if slash_index := strings.Index(ip_name, "/"); slash_index >= 0 {
		ip_name = ip_name[:slash_index]
	}

	if ip_name == "" {
		return ""
	}

	data_html := `<div id="opennamu_get_user_info">` + tool.HTML_escape(ip_name) + `</div>`

	// 최근 기여
	rows := tool.Query_DB(
		db,
		"select title, date, send from history where ip = ? order by date desc limit 10",
		ip_name,
	)
	list_html := ""
	for rows.Next() {
		title := ""
		date := ""
		send := ""

		if rows.Scan(&title, &date, &send) != nil {
			continue
		}

		list_html += `<tr><td><a href="/w/` + tool.Url_parser(title) + `">` + tool.HTML_escape(tool.Namespace_display(title)) + `</a></td><td>` + tool.HTML_escape(date) + `</td><td>` + tool.HTML_escape(send) + `</td></tr>`
	}
	rows.Close()

	if list_html == "" {
		list_html = `<tr><td colspan="3" class="namu_table_empty">` + tool.Get_language(db, "contribution_empty", true) + `</td></tr>`
	}

	data_html += `<h2>` + tool.Get_language(db, "recent_contribution", true) + `</h2><div class="namu_table_wrap"><table class="namu_table"><thead><tr><th>` + tool.Get_language(db, "document", true) + `</th><th>` + tool.Get_language(db, "time", true) + `</th><th>` + tool.Get_language(db, "summary", true) + `</th></tr></thead><tbody>` + list_html + `</tbody></table></div>`
	data_html += `<div class="namu_ipuser_menu"><a class="namu_button_small" href="/record/` + tool.Url_parser(ip_name) + `">` + tool.Get_language(db, "all_contribution_list", true) + `</a></div>`
	data_html += `<hr class="main_hr">`

	return data_html
}

func view_w_category_data(db *sql.DB, config tool.Config, doc_name string) string {
	type category_entry struct {
		name string
		view string
	}

	category_list := []category_entry{}
	category_seen := map[string]bool{}
	category_blur := map[string]bool{}
	add_category := func(name string, view string) {
		if name == "" || category_seen[name] {
			return
		}
		if view == "" {
			view = name
		}
		category_seen[name] = true
		category_list = append(category_list, category_entry{name, view})
	}
	show_category_view := tool.Get_main_skin_set(db, config, "main_css_category_change_title") != "off"
	get_category_meta := func(name string) (string, bool) {
		view := ""
		blur_value := ""
		if show_category_view {
			tool.QueryRow_DB(db, "select data from back where title = ? and link = ? and type = 'cat_view' limit 1", []any{&view}, doc_name, name)
		}
		blur := tool.QueryRow_DB(db, "select data from back where title = ? and link = ? and type = 'cat_blur' limit 1", []any{&blur_value}, doc_name, name)
		return view, blur
	}

	rows := tool.Query_DB(db, "select link, data from back where title = ? and type = 'cat' order by link", doc_name)
	for rows.Next() {
		name, view := "", ""
		if rows.Scan(&name, &view) == nil {
			category_view, blur := get_category_meta(name)
			if category_view != "" {
				view = category_view
			}
			if blur {
				category_blur[name] = true
			}
			add_category(name, view)
		}
	}
	rows.Close()

	if len(category_list) == 0 {
		rows = tool.Query_DB(db, "select title, data from back where link = ? and (type = 'cat' or type = '') order by title", doc_name)
		for rows.Next() {
			name, view := "", ""
			if rows.Scan(&name, &view) == nil {
				add_category(name, view)
			}
		}
		rows.Close()
	}

	if len(category_list) == 0 {
		return ""
	}

	category_sub := ""
	category_doc := ""
	category_sub_count := 0
	category_doc_count := 0
	for _, entry := range category_list {
		class_name := ""
		if category_blur[entry.name] {
			class_name = ` class="opennamu_category_blur"`
		}
		item := `<li><a` + class_name + ` href="/w/` + tool.Url_parser(entry.name) + `">` + tool.HTML_escape(entry.view) + `</a>`
		if strings.HasPrefix(entry.name, "category:") {
			category_sub += item + `</li>`
			category_sub_count++
		} else {
			category_doc += item + ` <a class="opennamu_link_inter" href="/xref/` + tool.Url_parser(entry.name) + `">(` + tool.Get_language(db, "backlink", true) + `)</a></li>`
			category_doc_count++
		}
	}

	data := ""
	if category_sub != "" {
		data += `<h2>` + tool.Get_language(db, "under_category", true) + `</h2><ul><li>` + tool.Get_language(db, "all", true) + " : " + strconv.Itoa(category_sub_count) + `</li>` + category_sub + `</ul>`
	}
	if category_doc != "" {
		data += `<h2>` + tool.Get_language(db, "category_title", true) + `</h2><ul><li>` + tool.Get_language(db, "all", true) + " : " + strconv.Itoa(category_doc_count) + `</li>` + category_doc + `</ul>`
	}
	return data + `<hr class="main_hr">`
}

func view_w_file_data(db *sql.DB, doc_name string) string {
	file_name := strings.TrimPrefix(doc_name, "file:")
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(file_name)), ".")
	if extension == "" {
		return ""
	}

	base_name := strings.TrimSuffix(file_name, filepath.Ext(file_name))
	storage_name := tool.File_name_to_dir(base_name, extension)
	file_path := filepath.Join(tool.Get_file_main_dir(db), storage_name)
	file_info, err := os.Stat(file_path)
	if err != nil || file_info.IsDir() {
		return ""
	}

	cache_revision := "1"
	tool.QueryRow_DB(db, "select id from history where title = ? order by date desc limit 1", []any{&cache_revision}, doc_name)
	image_url := "/image/" + tool.Url_parser(storage_name) + ".cache_v" + tool.Url_parser(cache_revision)

	media_type := tool.Get_file_media_type(extension)

	resolution := "Vector"
	preview_html := `<img src="` + image_url + `">`

	switch media_type {
	case "audio":
		resolution = "-"
		preview_html = `<div class="opennamu_media opennamu_media_audio"><audio class="opennamu_audio" controls preload="metadata" src="` + image_url + `"></audio></div>`
	case "video":
		resolution = "-"
		preview_html = `<div class="opennamu_media opennamu_media_video"><video class="opennamu_video" controls preload="metadata" src="` + image_url + `"></video></div>`
	case "image":
		if file, open_err := os.Open(file_path); open_err == nil {
			image_config, _, decode_err := image.DecodeConfig(file)
			_ = file.Close()
			if decode_err == nil {
				resolution = strconv.Itoa(image_config.Width) + "x" + strconv.Itoa(image_config.Height)
			}
		}
	default:
		resolution = "-"
		preview_html = `<div class="opennamu_media"><a class="namu_button" href="` + image_url + `">` + tool.Get_language(db, "file_download", true) + `</a></div>`
	}

	return preview_html + `<h2>` + tool.Get_language(db, "data", true) + `</h2><table><tr><td>` + tool.Get_language(db, "url", true) + `</td><td><a href="/image/` + tool.Url_parser(storage_name) + `">` + tool.Get_language(db, "link", true) + `</a></td></tr><tr><td>` + tool.Get_language(db, "volume", true) + `</td><td>` + strconv.FormatFloat(float64(file_info.Size())/1000, 'f', 1, 64) + `KB</td></tr><tr><td>` + tool.Get_language(db, "resolution", true) + `</td><td>` + resolution + `</td></tr></table><h2>` + tool.Get_language(db, "content", true) + `</h2>`
}
