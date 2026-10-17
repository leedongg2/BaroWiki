package route

import (
	"net/url"
	"opennamu/route/tool"
	"strings"
)

func View_alarm(config tool.Config, user_name string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)
	if user_name == "" {
		user_name = config.IP
	}
	if user_name != config.IP && !tool.Check_acl(db, "", "", "owner_auth", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}
	if values != nil && (values.Get("all") != "" || values.Get("delete") != "" || len(values["pick"]) > 0) {
		if values.Get("all") != "" {
			tool.Exec_DB(db, "delete from user_notice where name = ?", user_name)
		} else if id := values.Get("delete"); id != "" {
			tool.Exec_DB(db, "delete from user_notice where name = ? and id = ?", user_name, id)
		} else {
			// 고른 것만 지우기 (남의 알림을 지울 수 없게 이름까지 함께 맞춘다)
			for _, one := range values["pick"] {
				one = strings.TrimSpace(one)
				if one == "" {
					continue
				}

				tool.Exec_DB(db, "delete from user_notice where name = ? and id = ?", user_name, one)
			}
		}
		return tool.Get_redirect("/alarm/" + tool.Url_parser(user_name))
	}

	delete_url := "/alarm/delete"
	if user_name != config.IP {
		delete_url = "/alarm/" + tool.Url_parser(user_name) + "/delete"
	}
	page := 1
	if values != nil {
		page = tool.Str_to_int(values.Get("num"))
		if page < 1 {
			page = 1
		}
	}
	offset := (page - 1) * 50
	rows := tool.Query_DB(db, "select id, data, date, readme from user_notice where name = ? order by date desc limit ?, 50", user_name, offset)
	body := `<ul>`
	row_count := 0
	for rows.Next() {
		id, data, date, readme := "", "", "", ""
		if rows.Scan(&id, &data, &date, &readme) != nil {
			continue
		}
		row_count++
		data_split := strings.Split(data, " | ")
		data_html := tool.IP_parser(db, data_split[0], config.IP)
		if len(data_split) > 1 {
			data_html += " | " + strings.Join(data_split[1:], " | ")
		}
		data_style := ""
		if readme == "1" {
			data_style = ` style="opacity: 0.75;"`
		}
		body += `<li` + data_style + `><input type="checkbox" class="opennamu_alarm_pick" name="pick" value="` + tool.HTML_escape(id) + `"> ` + data_html + ` | ` + tool.HTML_escape(date) + ` <a href="` + delete_url + `/` + tool.Url_parser(id) + `">(` + tool.Get_language(db, "delete", true) + `)</a></li>`
	}
	rows.Close()
	body += `</ul>`

	if row_count > 0 {
		// 고르기 칸을 담는 폼. 고른 것만 한 번에 지운다.
		pick_action := "/alarm"
		if user_name != config.IP {
			pick_action = "/alarm/" + tool.Url_parser(user_name)
		}

		body = `<form method="post" action="` + pick_action + `">` +
			`<label><input type="checkbox" id="opennamu_alarm_pick_all"> ` +
			tool.HTML_escape(tool.Baro_t("모두 고르기")) + `</label><hr class="main_hr">` +
			body +
			`<button type="submit">` + tool.HTML_escape(tool.Baro_t("고른 알림 지우기")) + `</button></form>` +
			alarm_pick_script()
	}
	read_url := "/alarm/read"
	if user_name != config.IP {
		read_url = "/alarm/" + tool.Url_parser(user_name) + "/read"
	}
	read_form := `<form method="post" action="` + read_url + `"><button type="submit">` + tool.Get_language(db, "read_all", true) + `</button></form><hr class="main_hr">`
	if row_count > 0 {
		body = read_form + `<a href="` + delete_url + `">(` + tool.Get_language(db, "delete", true) + `)</a><hr class="main_hr">` + body
	} else {
		body = read_form + body
	}
	page_url := "/alarm?num={}"
	if user_name != config.IP {
		page_url = "/alarm/" + tool.Url_parser(user_name) + "?num={}"
	}
	body += tool.Get_page_control(db, page, row_count, 50, page_url)
	return user_form_page(db, config, tool.Get_language(db, "alarm", true), body)
}

// alarm_pick_script : 알림 목록의 「모두 고르기」
func alarm_pick_script() string {
	return `<script>
(function() {
    var all = document.getElementById('opennamu_alarm_pick_all');
    if (all === null) { return; }

    var list = document.querySelectorAll('.opennamu_alarm_pick');
    all.addEventListener('change', function() {
        for (var i = 0; i < list.length; i++) { list[i].checked = all.checked; }
    });
})();
</script>`
}
