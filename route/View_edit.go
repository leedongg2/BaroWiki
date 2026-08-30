package route

import (
	"opennamu/route/tool"
)

func View_edit(config tool.Config, doc_name string, load_doc_name string, section string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Do_title_length_check(db, doc_name, "document") {
		return tool.Get_error_page(db, config, "title length")
	}

	// 바로위키 : 격리 검토 중인 문서는 편집창을 열기 전에 먼저 알린다
	// (저장을 눌러야 막히면 쓴 글이 헛수고가 된다)
	if tool.Baro_doc_hidden(db, config, doc_name) {
		reason := ""
		tool.QueryRow_DB(
			db,
			"select reason from baro_quarantine where title = ? and status = 'open' limit 1",
			[]any{&reason},
			doc_name,
		)

		body := tool.Baro_alert("danger",
			"<b>"+tool.Lang_esc("quarantine_edit_block")+"</b><br>"+
				tool.Lang_esc("quarantine_keep_notice"))

		info := [][]string{{tool.Lang("document"), tool.HTML_escape(tool.Namespace_display(doc_name))}}
		if reason != "" {
			info = append(info, []string{tool.Lang("quarantine_reason"), tool.HTML_escape(reason)})
		}

		body += tool.Baro_card("", tool.Baro_kv_grid(info)+
			tool.Baro_note(tool.Lang("quarantine_appeal_notice"))+
			tool.Baro_button("/w/"+tool.Url_parser(doc_name), tool.Lang("back_to_document"), "")+" "+
			tool.Baro_button("/baro/quarantine", tool.Lang("quarantine_list"), "")+" "+
			tool.Baro_button("/baro/appeal", tool.Lang("appeal"), ""))

		return tool.Baro_page(db, config, doc_name+" ("+tool.Lang("cannot_edit")+")", body,
			[][]any{{"w/" + tool.Url_parser(doc_name), tool.Lang("return")}})
	}

	var raw_data map[string]any
	raw_data_get := ""
	if load_doc_name == "" {
		raw_data = Api_w_raw(config, doc_name, "", "")
	} else {
		raw_data = Api_w_raw(config, load_doc_name, "", "")
	}

	if raw_data["response"].(string) == "ok" {
		raw_data_get = raw_data["data"].(string)
	}

	// 문단 편집 (나무위키 [편집] 링크)
	section_ui := ""
	section_title := "(" + tool.Get_language(db, "edit", true) + ")"
	if section != "" && load_doc_name == "" {
		section_data, section_exist := tool.Section_text(raw_data_get, section)
		if !section_exist {
			return tool.Get_redirect("/edit/" + tool.Url_parser(doc_name))
		}

		raw_data_get = section_data
		section_ui = `<input type="hidden" name="section" value="` + tool.HTML_escape(section) + `">`
		section_title = "(" + tool.Lang_esc("edit_section") + " " + tool.HTML_escape(section) + ")"
	}

	check_box := tool.Get_edit_check_box_ui(db)
	bottom_text := tool.Get_edit_bottom_text_ui(db, "edit")

	editor_top_text := ""
	if load_doc_name == "" {
		editor_top_text += `<a href="/manager/15/` + tool.Url_parser(doc_name) + `">(` + tool.Get_language(db, "load", true) + `)</a> `
	}

	if editor_top_text != "" {
		editor_top_text += `<hr class="main_hr">`
	}

	revision := tool.Get_document_revision(db, doc_name)

	// 요약 입력은 나무위키와 동일하게 편집창 아래에 둔다
	summary_ui := `<div class="namu_edit_summary"><span class="namu_edit_summary_label">` + tool.Get_language(db, "why", true) + `</span><input class="namu_input" type="text" name="send" placeholder="` + tool.Get_language(db, "why", true) + `"></div>`

	form_data := editor_top_text + `<form action="/edit/` + tool.Url_parser(doc_name) + `" method="post">
        <input type="hidden" name="ver" value="` + tool.HTML_escape(revision) + `">
        ` + section_ui + `
        ` + tool.Get_editor_ui(db, config, raw_data_get, "edit", summary_ui+check_box+bottom_text, doc_name) + `
    </form>`

	out := tool.Get_template(
		db,
		config,
		doc_name,
		form_data,
		[]any{section_title},
		[][]any{
			{"w/" + tool.Url_parser(doc_name), tool.Get_language(db, "return", true)},
			{"delete/" + tool.Url_parser(doc_name), tool.Get_language(db, "delete", true)},
			{"move/" + tool.Url_parser(doc_name), tool.Get_language(db, "move", true)},
		},
		map[string]string{},
	)

	return out
}
