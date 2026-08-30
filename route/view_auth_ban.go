package route

import (
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"opennamu/route/tool"
)

func selected_attr(value string, target string) string {
	if value == target {
		return ` selected`
	}
	return ""
}

// ban_option_list : 차단 종류 (로그인 허용 범위)
func ban_option_list() [][2]string {
	return [][2]string{
		{"", tool.Lang("ban_type_normal")},
		{"login_able", tool.Lang("ban_type_login")},
		{"login_able_and_regsiter_disable", tool.Lang("ban_type_login_no_register")},
		{"completely_ban", tool.Lang("ban_type_no_login")},
		{"dont_come_this_site", tool.Lang("ban_type_no_access")},
		{"release", tool.Lang("release")},
	}
}

// ban_option_to_login : 차단 종류를 DB 저장 값으로 변환
func ban_option_to_login(ban_option string) string {
	switch ban_option {
	case "login_able":
		return "L"
	case "login_able_and_regsiter_disable":
		return "O"
	case "completely_ban":
		return "A"
	case "dont_come_this_site":
		return "D"
	}

	return ""
}

// ban_preset_list : 자주 쓰는 차단 기간 (초 단위)
func ban_preset_list() [][2]string {
	return [][2]string{
		{"", tool.Lang("direct_input")},
		{"3600", "1 " + tool.Lang("hour")},
		{"10800", "3 " + tool.Lang("hour")},
		{"86400", "1 " + tool.Lang("day")},
		{"259200", "3 " + tool.Lang("day")},
		{"604800", "7 " + tool.Lang("day")},
		{"1209600", "14 " + tool.Lang("day")},
		{"2592000", "1 " + tool.Lang("month")},
		{"7776000", "3 " + tool.Lang("month")},
		{"31536000", "1 " + tool.Lang("year")},
		{"0", tool.Lang("limitless")},
	}
}

// ban_duration_input : 기간 입력 UI
func ban_duration_input(name string, label string) string {
	return `<label class="namu_duration_item"><span>` + label + `</span><input class="namu_input_number" type="number" min="0" name="` + name + `" placeholder="0"></label>`
}

// View_auth_ban : 사용자 차단 / 차단 해제
func View_auth_ban(config tool.Config, name string, ban_type string, multiple bool, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	ban_data := tool.Get_user_ban(db, config.IP, "login")
	can_ban := tool.Check_acl(db, "", "", "ban_auth", config.IP)
	if ban_data[0] == "true" && (tool.IP_or_user(config.IP) || tool.Check_acl(db, "", "", "all_admin_auth", config.IP)) {
		can_ban = false
	}
	if !can_ban {
		return tool.Get_error_page(db, config, "auth")
	}

	if values != nil {
		end := ""
		release := values.Get("ban_option") == "release"

		if values.Get("date_forever") == "" {
			preset_second := tool.Str_to_int(values.Get("date_preset"))
			duration_second := tool.Ban_duration_second(
				values.Get("date_year"),
				values.Get("date_month"),
				values.Get("date_week"),
				values.Get("date_day"),
				values.Get("date_hour"),
				values.Get("date_minute"),
				values.Get("date_second"),
			)

			if values.Get("date_preset") != "" {
				duration_second = preset_second
			}

			if date_value := strings.TrimSpace(values.Get("date_direct")); date_value != "" {
				date_value = strings.ReplaceAll(date_value, "T", " ")
				if len(date_value) == 16 {
					date_value += ":00"
				}

				if end_time, err := time.ParseInLocation("2006-01-02 15:04:05", date_value, time.Local); err == nil {
					end = end_time.Format("2006-01-02 15:04:05")
					duration_second = 0
				}
			}

			if end == "" && duration_second > 0 {
				end = tool.Ban_end_from_duration(duration_second)
			}
		}

		login := ban_option_to_login(values.Get("ban_option"))

		type_name := ""
		switch values.Get("do_ban_type") {
		case "regex":
			type_name = "regex"
		case "cidr":
			type_name = "cidr"
		case "private":
			type_name = "private"
			if !tool.Check_acl(db, "", "", "owner_auth", config.IP) {
				return tool.Get_error_page(db, config, "auth")
			}
		}

		if !multiple && name == "" {
			name = values.Get("name")
		}
		names := []string{name}
		if multiple {
			names = strings.Split(strings.ReplaceAll(values.Get("name"), "\r", ""), "\n")
		}

		for _, target := range names {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}

			if type_name == "regex" {
				if _, err := regexp.Compile(target); err != nil {
					return tool.Get_error_page(db, config, "error")
				}
			} else if type_name == "cidr" {
				if _, _, err := net.ParseCIDR(target); err != nil {
					return tool.Get_error_page(db, config, "error")
				}
			}

			if type_name != "private" && target == config.IP {
				if !tool.Check_acl(db, "", "", "all_admin_auth", config.IP) {
					return tool.Get_error_page(db, config, "auth")
				}
			} else if type_name != "private" && !tool.Check_acl(db, "", "", "ban_auth", config.IP) {
				return tool.Get_error_page(db, config, "auth")
			}

			// 관리자는 차단할 수 없음 (소유자 제외)
			if type_name == "" && !tool.IP_or_user(target) && !release {
				if tool.Check_acl(db, "", "", "all_admin_auth", target) && !tool.Check_acl(db, "", "", "owner_auth", config.IP) {
					return tool.Get_error_page(db, config, "auth")
				}
			}

			ban_end := end
			if ban_end == "" {
				ban_end = "0"
			}

			tool.Do_ban_insert(db, target, ban_end, values.Get("why"), login, config.IP, type_name, release)

			log_text := "ban (" + target + ")"
			if release {
				log_text = "ban release (" + target + ")"
			}
			tool.Do_insert_auth_history(db, config.IP, log_text)

			if !release && !tool.IP_or_user(target) {
				end_text := tool.Lang("limitless")
				if end != "" {
					end_text = end
				}

				tool.Send_alarm(
					db, config.IP, target,
					tool.Lang("banned_notice")+" "+tool.Lang("expire")+" : "+end_text+
						" / "+tool.Lang("why")+" : "+values.Get("why"),
				)
			}
		}

		return tool.Get_redirect("/recent_block")
	}

	// 현재 차단 상태 표시
	now_ban_html := ""
	if name != "" && !multiple {
		ban_info := tool.Get_ban_info(db, name)
		if ban_info.Exist {
			end_text := tool.Lang_esc("limitless")
			if ban_info.End != "" {
				end_text = tool.HTML_escape(ban_info.End) + " (" + tool.HTML_escape(tool.Ban_duration_text(ban_info.End)) + " " + tool.Lang_esc("remain") + ")"
			}

			now_ban_html += `<div class="namu_ban_now"><b>` + tool.Lang_esc("ban_now") + `</b><ul class="namu_info_list">`
			now_ban_html += `<li>` + tool.Lang_esc("expire") + ` : ` + end_text + `</li>`
			now_ban_html += `<li>` + tool.Lang_esc("why") + ` : ` + tool.HTML_escape(ban_info.Why) + `</li>`
			now_ban_html += `<li>` + tool.Lang_esc("blocker") + ` : ` + tool.HTML_escape(ban_info.Blocker) + `</li>`
			now_ban_html += `</ul></div>`
		}
	}

	target_input := `<input class="namu_input" name="name" value="` + tool.HTML_escape(name) + `" placeholder="` + tool.Get_language(db, "name_or_ip_or_regex_or_cidr", true) + `">`
	if multiple {
		target_input = `<textarea class="opennamu_textarea_500 namu_textarea" name="name" placeholder="` + tool.Get_language(db, "name_or_ip_or_regex_or_cidr_multiple", true) + `"></textarea>`
	}

	type_options := `<option value="normal">` + tool.Get_language(db, "normal", true) + `</option>`
	type_options += `<option value="regex"` + selected_attr(ban_type, "regex") + `>` + tool.Get_language(db, "regex", true) + `</option>`
	type_options += `<option value="cidr"` + selected_attr(ban_type, "cidr") + `>` + tool.Get_language(db, "cidr", true) + `</option>`
	if tool.Check_acl(db, "", "", "owner_auth", config.IP) {
		type_options += `<option value="private"` + selected_attr(ban_type, "private") + `>` + tool.Get_language(db, "private", true) + `</option>`
	}

	ban_option_html := `<select class="namu_select" name="ban_option">`
	for _, option := range ban_option_list() {
		ban_option_html += `<option value="` + option[0] + `">` + option[1] + `</option>`
	}
	ban_option_html += `</select>`

	preset_html := `<select class="namu_select" name="date_preset">`
	for _, option := range ban_preset_list() {
		preset_html += `<option value="` + option[0] + `">` + option[1] + `</option>`
	}
	preset_html += `</select>`

	data := now_ban_html
	data += `<form method="post" class="namu_form">`
	data += `<h2 class="namu_form_title">` + tool.Get_language(db, "target", true) + `</h2>` + target_input
	data += `<hr class="main_hr"><select class="namu_select" name="do_ban_type">` + type_options + `</select>`
	data += `<hr class="main_hr">` + ban_option_html

	data += `<h2 class="namu_form_title">` + tool.Get_language(db, "ban_period", true) + `</h2>`
	data += `<div class="namu_duration_row">` + preset_html + `</div>`
	data += `<div class="namu_duration_row">`
	data += ban_duration_input("date_year", tool.Get_language(db, "year", true))
	data += ban_duration_input("date_month", tool.Get_language(db, "month", true))
	data += ban_duration_input("date_week", tool.Get_language(db, "week", true))
	data += ban_duration_input("date_day", tool.Get_language(db, "day", true))
	data += ban_duration_input("date_hour", tool.Get_language(db, "hour", true))
	data += ban_duration_input("date_minute", tool.Get_language(db, "minute", true))
	data += ban_duration_input("date_second", tool.Get_language(db, "second", true))
	data += `</div>`
	data += `<div class="namu_duration_row"><label class="namu_check_label"><input type="checkbox" name="date_forever" value="1"> ` + tool.Get_language(db, "ban_forever", true) + `</label></div>`
	data += `<div class="namu_duration_row"><label class="namu_duration_item"><span>` + tool.Get_language(db, "direct_set", true) + `</span><input class="namu_input" type="datetime-local" step="1" name="date_direct"></label></div>`
	data += `<div class="namu_form_help">` + tool.Get_language(db, "ban_duration_help", false) + `</div>`

	data += `<h2 class="namu_form_title">` + tool.Get_language(db, "why", true) + `</h2><input class="namu_input" name="why" placeholder="` + tool.Get_language(db, "why", true) + `">`
	data += `<hr class="main_hr"><button class="namu_button" type="submit">` + tool.Get_language(db, "save", true) + `</button></form>`

	title := tool.Get_language(db, "ban", true)
	if multiple {
		title = tool.Get_language(db, "multiple_ban", true)
	}

	menu := [][]any{
		{"recent_block", tool.Get_language(db, "recent_ban", true)},
		{"auth/warn", tool.Get_language(db, "warn", true)},
		{"manager", tool.Get_language(db, "return", true)},
	}

	return tool.Get_template(db, config, title, data, []any{}, menu, map[string]string{})
}
