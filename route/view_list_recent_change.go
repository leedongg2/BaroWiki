package route

import (
	"database/sql"
	"html"
	"regexp"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

var re_esc_a = regexp.MustCompile(`&lt;a(([^&]|&[^;]+;)*)&gt;([^\r\n]*?)&lt;/a&gt;`)
var re_safe_url = regexp.MustCompile(`(^| )(https?://[^ ]+)`)

func Get_safe_send_data(data string) string {
	data = tool.HTML_escape(data)
	if data == "&lt;br&gt;" || data == "" || strings.TrimSpace(data) == "" {
		return "<br>"
	}

	data = strings.ReplaceAll(data, "javascript:", "")
	data = re_safe_url.ReplaceAllStringFunc(data, func(match string) string {
		prefix := ""
		url := match
		if strings.HasPrefix(match, " ") {
			prefix = " "
			url = match[1:]
		}
		return prefix + "<a href=\"" + url + "\">" + url + "</a>"
	})

	return re_esc_a.ReplaceAllStringFunc(data, func(match string) string {
		parts := re_esc_a.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}

		inner_escaped := parts[3]
		inner_text := strings.TrimSpace(html.UnescapeString(inner_escaped))
		if inner_text == "" || strings.ContainsAny(inner_text, "<>") {
			return match
		}

		return "<a href=\"/w/" + tool.Url_parser(inner_text) + "\">" + strings.TrimSpace(inner_escaped) + "</a>"
	})
}

func Get_ui_history(db *sql.DB, config tool.Config, data_all [][]string) (string, string) {
	auth_name := tool.Get_user_auth(db, config.IP)
	auth_info := tool.Get_auth_group_info(db, auth_name)

	// 역사 목록은 50줄이 모두 같은 문서다. 줄마다 앞 판을 물으면
	// 판이 많은 문서에서 같은 조회를 50번 하게 되므로 한 번만 읽어 둔다.
	rev_lookup := new_history_rev_lookup(db)

	// 바로위키 : 판 고유 번호도 줄마다 묻지 않고 한 번에 읽어 둔다
	uuid_pairs := [][2]string{}
	previous_revs := map[string]string{}
	for _, in_data := range data_all {
		if in_data[1] == "" || in_data[0] == "" {
			continue
		}

		uuid_pairs = append(uuid_pairs, [2]string{in_data[1], in_data[0]})
		previous := rev_lookup.previous_revision(in_data[1], tool.Str_to_int(in_data[0]))
		key := in_data[1] + "\n" + in_data[0]
		previous_revs[key] = previous
		if previous != "" {
			uuid_pairs = append(uuid_pairs, [2]string{in_data[1], previous})
		}
	}
	rev_uuid := tool.Baro_rev_uuid_list(db, uuid_pairs)
	revision_url := func(action string, doc_url string, uuid string, revision string) string {
		if uuid != "" {
			return "/" + action + "/" + doc_url + "?uuid=" + tool.Url_parser(uuid)
		}
		if action == "raw" {
			return "/raw_rev/" + revision + "/" + doc_url
		}
		return "/" + action + "/" + revision + "/" + doc_url
	}

	date_heading := ""
	data_html := ""
	data_select := ""
	for_count := 1

	for _, in_data := range data_all {
		for_count_str := strconv.Itoa(for_count)
		for_count += 1

		if in_data[6] != "" && in_data[1] == "" {
			if date_heading != "----" {
				data_html += "<h2>----</h2>"
				date_heading = "----"
			}

			data_html += tool.Get_list_ui("----", "", "", "")
			continue
		}

		data_select += `<option value="` + in_data[0] + `">` + in_data[0] + `</option>`
		doc_name := in_data[1]
		doc_name_url := tool.Url_parser(doc_name)
		rev_str := in_data[0]

		left := `<a href="/w/` + doc_name_url + `">` + tool.HTML_escape(doc_name) + `</a> `
		rev := ""

		// 바로위키 : 판 번호 위에 손을 올리면 변하지 않는 고유 번호가 보인다
		row_uuid := rev_uuid[doc_name+"\n"+rev_str]
		rev_title := ""
		if row_uuid != "" {
			rev_title = ` title="` + tool.Baro_t("고유 번호") + ` : ` + tool.HTML_escape(row_uuid) + `"`
		}

		if in_data[6] != "" {
			rev = `<span style="color: red;"` + rev_title + `>r` + rev_str + `</span>`
		} else {
			rev = `<span` + rev_title + `>r` + rev_str + `</span>`
		}

		// 판을 지우면 번호에 구멍이 나므로 (예: 1 · 5 · 9) "r - 1" 로 어림잡으면
		// 없는 판을 가리켜 링크가 죽는다. 실제로 남아 있는 앞 판을 한 번만 찾아 둔다.
		row_key := doc_name + "\n" + rev_str
		before_rev_str := previous_revs[row_key]
		previous_uuid := rev_uuid[doc_name+"\n"+before_rev_str]
		row_uuid = rev_uuid[row_key]
		diff_url := "/diff/" + before_rev_str + "/" + rev_str + "/" + doc_name_url
		if row_uuid != "" {
			diff_url = "/diff/" + doc_name_url + "?uuid=" + tool.Url_parser(row_uuid)
			if previous_uuid != "" {
				diff_url += "&olduuid=" + tool.Url_parser(previous_uuid)
			}
		}
		if before_rev_str != "" {
			rev = `<a href="` + diff_url + `">` + rev + `</a>`
		}

		right := ""
		right += `<span id="opennamu_list_history_` + for_count_str + `_over">`
		right += `<a id="opennamu_list_history_` + for_count_str + `" href="javascript:void(0);">`
		right += `<span class="opennamu_svg opennamu_svg_tool">&nbsp;</span></a>`
		right += `<span class="opennamu_popup_footnote" id="opennamu_list_history_` + for_count_str + `_load" style="display: none;"></span>`
		right += `</span> | `
		right += rev + " | "

		diff_size := in_data[5]
		if diff_size == "0" {
			right += `<span style="color: gray;">` + diff_size + `</span>`
		} else if strings.Contains(diff_size, "+") {
			right += `<span style="color: green;">` + diff_size + `</span>`
		} else {
			right += `<span style="color: red;">` + diff_size + `</span>`
		}

		right += " | "
		right += in_data[7] + " | "

		edit_type := "edit"
		if in_data[8] != "" {
			edit_type = in_data[8]
		}

		right += tool.Get_language(db, edit_type, true) + " | "

		time_split := strings.Split(in_data[2], " ")
		if date_heading != time_split[0] {
			data_html += "<h2>" + time_split[0] + "</h2>"
			date_heading = time_split[0]
		}

		if len(time_split) > 1 {
			right += time_split[1]
		}

		right += `<span style="display: none;" id="opennamu_history_tool_` + for_count_str + `">`
		right += `<a href="` + revision_url("render", doc_name_url, row_uuid, rev_str) + `">` + tool.Get_language(db, "view", true) + `</a>`
		right += ` | <a href="` + revision_url("raw", doc_name_url, row_uuid, rev_str) + `">` + tool.Get_language(db, "raw", true) + `</a>`
		right += ` | <a href="` + revision_url("blame", doc_name_url, row_uuid, rev_str) + `">` + tool.Get_language(db, "blame", true) + `</a>`
		right += ` | <a href="` + revision_url("revert", doc_name_url, row_uuid, rev_str) + `">` + tool.Get_language(db, "revert", true) + ` (r` + rev_str + `)</a>`

		if before_rev_str != "" {
			right += ` | <a href="` + revision_url("revert", doc_name_url, previous_uuid, before_rev_str) + `">` + tool.Get_language(db, "revert", true) + ` (` + tool.Get_language(db, "r1", true) + before_rev_str + `)</a>`
			right += ` | <a href="` + diff_url + `">` + tool.Get_language(db, "compare", true) + `</a>`
		}

		right += ` | <a href="/history/` + doc_name_url + `">` + tool.Get_language(db, "history", true) + `</a>`

		// 바로위키 : 판 번호는 앞 판이 지워지면 밀리므로, 변하지 않는 고유 번호도 함께 준다.
		// 고유 번호는 판 번호 자리에 그대로 넣을 수 있다. (`/render/<고유 번호>/<문서>`)
		if row_uuid != "" {
			right += ` | <a href="/render/` + doc_name_url + `?uuid=` + tool.Url_parser(row_uuid) + `">` + tool.Baro_t("고유 주소") + `</a>`
			right += ` | <a href="javascript:void(0);" id="opennamu_rev_uuid_` + for_count_str + `">` + tool.Baro_t("고유 번호 복사") + `</a>`
		}

		if _, ok := auth_info["owner"]; ok {
			right += ` | <a href="/history_hidden/` + rev_str + `/` + doc_name_url + `">` + tool.Get_language(db, "hide", true) + `</a>`
			right += ` | <a href="/history_delete/` + rev_str + `/` + doc_name_url + `">` + tool.Get_language(db, "history_delete", true) + `</a>`
			right += ` | <a href="/history_send/` + rev_str + `/` + doc_name_url + `">` + tool.Get_language(db, "send_edit", true) + `</a>`
		} else if _, ok := auth_info["hidel"]; ok {
			right += ` | <a href="/history_hidden/` + rev_str + `/` + doc_name_url + `">` + tool.Get_language(db, "hide", true) + `</a>`
		}

		right += `</span>`

		bottom := ``
		if in_data[4] != "" {
			bottom = Get_safe_send_data(in_data[4])
		}

		data_html += tool.Get_list_ui(left, right, bottom, "")
		data_html += `<script>
            document.getElementById('opennamu_list_history_` + for_count_str + `').addEventListener("click", function() {
                opennamu_do_footnote_popover('opennamu_list_history_` + for_count_str + `', '', 'opennamu_history_tool_` + for_count_str + `', 'open');
            });
            document.addEventListener("click", function() {
                opennamu_do_footnote_popover('opennamu_list_history_` + for_count_str + `', '', 'opennamu_history_tool_` + for_count_str + `', 'close');
            });
        </script>`

		// 바로위키 : 고유 번호 복사
		// 클립보드를 못 쓰는 자리(http · 옛 브라우저)에서도 번호는 보이도록 prompt 로 물러선다.
		if row_uuid != "" {
			data_html += `<script>
            (function() {
                var button = document.getElementById('opennamu_rev_uuid_` + for_count_str + `');
                if (!button) { return; }

                var value = '` + tool.JS_escape(row_uuid) + `';
                button.addEventListener("click", function() {
                    if (navigator.clipboard && navigator.clipboard.writeText) {
                        navigator.clipboard.writeText(value).then(function() {
                            button.textContent = '` + tool.JS_escape(tool.Baro_t("복사했습니다")) + `';
                        }, function() {
                            window.prompt('` + tool.JS_escape(tool.Baro_t("고유 번호")) + `', value);
                        });
                        return;
                    }

                    window.prompt('` + tool.JS_escape(tool.Baro_t("고유 번호")) + `', value);
                });
            })();
        </script>`
		}
	}

	return data_html, data_select
}

func View_list_recent_change(config tool.Config, set_type string, limit string, num string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	sub := ""
	if set_type == "" {
		set_type = "normal"
	} else {
		sub = "(" + tool.Get_language(db, set_type, true) + ")"
	}

	data_html := ""

	menu_option := []string{"normal", "edit", "move", "delete", "revert", "r1", "file", "category"}
	for _, option := range menu_option {
		label := tool.Get_language(db, option, true)
		data_html += `<a href="/recent_change/1/` + option + `">(` + label + `)</a> `
	}
	data_html += `<a href="/recent_change/1/user">(` + tool.Get_language(db, "user_document", true) + `)</a> `

	api_data := Api_list_recent_change(config, set_type, limit, num)
	api_data_list := api_data["data"].([][]string)

	history_ui, _ := Get_ui_history(db, config, api_data_list)

	data_html += history_ui
	data_html += tool.Get_page_control(
		db,
		tool.Str_to_int(num),
		len(api_data_list),
		tool.Str_to_int(limit),
		"/recent_change/{}/"+set_type,
	)

	out := tool.Get_template(
		db,
		config,
		tool.Get_language(db, "recent_change", true),
		data_html,
		[]any{sub},
		[][]any{{"other", tool.Get_language(db, "return", true)}},
		map[string]string{},
	)

	return out
}
