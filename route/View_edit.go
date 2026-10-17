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
			tool.Baro_button("/w/"+tool.Doc_url(doc_name), tool.Lang("back_to_document"), "")+" "+
			tool.Baro_button("/baro/quarantine", tool.Lang("quarantine_list"), "")+" "+
			tool.Baro_button("/baro/appeal", tool.Lang("appeal"), ""))

		return tool.Baro_page(db, config, doc_name+" ("+tool.Lang("cannot_edit")+")", body,
			[][]any{{"w/" + tool.Url_parser(doc_name), tool.Lang("return")}})
	}

	// 편집할 수 없는 문서라면 저장을 눌러 헛수고하기 전에 먼저 알리고,
	// 그 자리에서 편집 요청으로 갈 수 있게 한다.
	if !tool.Check_acl(db, doc_name, "", "document_edit", config.IP) {
		body := tool.Baro_alert("warn", tool.Lang_esc("edit_locked_notice"))
		body += tool.Baro_card("", tool.Baro_kv_grid([][]string{
			{tool.Lang("document"), tool.HTML_escape(tool.Namespace_display(doc_name))},
		})+tool.Baro_note(tool.Lang("edit_request_instead_notice"))+
			tool.Baro_button("/baro/req/new?kind=edit&doc="+tool.Url_parser(doc_name), tool.Lang("edit_request_make"), "primary")+" "+
			tool.Baro_button("/baro/req/doc/"+tool.Url_parser(doc_name), tool.Lang("request_center"), "")+" "+
			tool.Baro_button("/w/"+tool.Doc_url(doc_name), tool.Lang("return"), "")+" "+
			tool.Baro_button("/acl/"+tool.Url_parser(doc_name), "ACL", ""))

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

	// AI 를 참고했는지 스스로 밝히는 칸. 저작권 동의 칸 위에 둔다.
	check_box := tool.Baro_ai_check_ui() + tool.Get_edit_check_box_ui(db)
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
