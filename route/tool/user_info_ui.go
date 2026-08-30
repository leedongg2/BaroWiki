package tool

import (
	"database/sql"
	"strconv"
	"strings"
)

func Get_user_info_ui(db *sql.DB, config Config, user_name string) string {
	is_ip := IP_or_user(user_name)

	auth_name := Get_user_auth(db, user_name)
	auth_date := Get_auth_date(db, user_name)
	if auth_date != "0" {
		auth_name += " (~" + auth_date + ")"
	}

	ban_data := Get_user_ban(db, user_name, "")
	ban_state := `<span class="namu_user_state_normal">` + Get_language(db, "normal", false) + `</span>`
	if len(ban_data) > 0 && ban_data[0] != "" {
		ban_type := ""
		if len(ban_data) > 1 {
			ban_type = ban_data[1]
		}

		ban_link := "/recent_block/user/" + Url_parser(user_name)
		switch {
		case strings.HasPrefix(ban_type, "a"):
			ban_link = "/recent_block/regex"
		case strings.HasPrefix(ban_type, "b"):
			ban_link = "/recent_block/cidr"
		}

		ban_info := Get_ban_info(db, user_name)
		end_text := Lang_esc("limitless")
		if ban_info.End != "" {
			end_text = HTML_escape(ban_info.End) + " (" + HTML_escape(Ban_duration_text(ban_info.End)) + " " + Lang_esc("remain") + ")"
		}

		ban_state = `<a class="namu_user_state_ban" href="` + ban_link + `">` + Lang_esc("banned") + `</a>`
		ban_state += `<br>` + Lang_esc("expire") + ` : ` + end_text
		if ban_info.Why != "" {
			ban_state += `<br>` + Lang_esc("why") + ` : ` + HTML_escape(ban_info.Why)
		}
		if ban_type != "" {
			ban_state += `<br>` + Get_language(db, "type", false) + ` : ` + HTML_escape(ban_type)
		}
	}

	level_data := Get_level(db, user_name)

	data_html := `<div class="namu_user_card">`

	gravatar_html := Get_user_gravatar_ui(db, user_name, 72)
	if gravatar_html != "" {
		data_html += `<div class="namu_user_card_avatar">` + gravatar_html + `</div>`
	}

	data_html += `<div class="namu_user_card_body"><table class="user_info_table namu_user_table">`
	data_html += `<tr><td>` + Get_language(db, "user_name", false) + `</td><td>` + IP_parser(db, user_name, config.IP) + `</td></tr>`
	data_html += `<tr><td>` + Get_language(db, "authority", false) + `</td><td>` + HTML_escape(auth_name) + `</td></tr>`
	data_html += `<tr><td>` + Get_language(db, "state", false) + `</td><td>` + ban_state + `</td></tr>`

	if !is_ip {
		data_html += `<tr><td>` + Get_language(db, "level", false) + `</td><td>` + HTML_escape(level_data[0]) + ` (` + HTML_escape(level_data[1]) + ` / ` + HTML_escape(level_data[2]) + `)</td></tr>`

		register_info := Get_user_register_info(db, user_name)
		if register_info["date"] != "" {
			data_html += `<tr><td>` + Lang_esc("join_date") + `</td><td>` + HTML_escape(register_info["date"]) + `</td></tr>`
		}

		warn_count := Get_warn_count(db, user_name)
		if warn_count > 0 {
			data_html += `<tr><td>` + Lang_esc("warn") + `</td><td><a href="/warn/` + Url_parser(user_name) + `">` + Lang_esc("unchecked") + ` ` + strconv.Itoa(warn_count) + Lang_esc("count_unit") + `</a></td></tr>`
		}

		// 관리자 전용 정보
		if Check_acl(db, "", "", "check_auth", config.IP) {
			if register_info["ip"] != "" {
				data_html += `<tr><td>` + Lang_esc("join_ip") + `</td><td>` + HTML_escape(register_info["ip"]) + `</td></tr>`
			}
			if register_info["email"] != "" {
				data_html += `<tr><td>` + Lang_esc("email") + `</td><td>` + HTML_escape(register_info["email"]) + `</td></tr>`
			}
		}
	}

	data_html += `</table>`

	// 관리 메뉴
	admin_menu := ""
	if !is_ip && Check_acl(db, "", "", "give_auth", config.IP) {
		admin_menu += `<a class="namu_button_small" href="/auth/user/` + Url_parser(user_name) + `">` + Lang_esc("auth_manage") + `</a> `
	}
	if Check_acl(db, "", "", "ban_auth", config.IP) {
		admin_menu += `<a class="namu_button_small" href="/auth/ban/` + Url_parser(user_name) + `">` + Lang_esc("ban") + `</a> `
		if !is_ip {
			admin_menu += `<a class="namu_button_small" href="/auth/warn/` + Url_parser(user_name) + `">` + Lang_esc("warn") + `</a> `
		}
	}
	if admin_menu != "" {
		data_html += `<div class="namu_user_card_menu">` + admin_menu + `</div>`
	}

	data_html += `</div></div>`

	return data_html
}

func Replace_user_info_ui(db *sql.DB, config Config, data string) string {
	const start_tag = `<div id="opennamu_get_user_info">`
	const end_tag = `</div>`

	for {
		start_index := strings.Index(data, start_tag)
		if start_index < 0 {
			return data
		}
		content_start := start_index + len(start_tag)
		end_offset := strings.Index(data[content_start:], end_tag)
		if end_offset < 0 {
			return data
		}
		end_index := content_start + end_offset
		user_name := HTML_unescape(data[content_start:end_index])
		data = data[:start_index] + Get_user_info_ui(db, config, user_name) + data[end_index+len(end_tag):]
	}
}
