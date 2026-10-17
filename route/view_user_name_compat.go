package route

import (
	"net/url"

	"opennamu/route/tool"
)

func View_user_name_for(config tool.Config, target string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)
	if target == "" {
		if !user_auth(db, config) {
			return tool.Get_redirect("/login")
		}
		target = config.IP
	} else if !tool.Check_acl(db, "", "", "owner_auth", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	if values != nil {
		value := values.Get("new_user_name")
		if value == "" {
			value = values.Get("data")
		}
		current := user_value(db, target, "user_name")
		if value == "" || (!tool.Get_user_name_check(db, value) && value != current) {
			return tool.Get_error_page(db, config, "user name error")
		}
		user_save(db, target, "user_name", value)

		// 바로위키 : 이름은 바뀌어도 계정 고유 번호는 그대로다.
		// 누가 언제 어떤 이름을 썼는지 고유 번호에 매달아 남긴다.
		if current == "" {
			current = target
		}
		tool.Baro_user_name_log_add(db, target, current, value, config.IP)

		if target != config.IP {
			return tool.Get_redirect("/change/user_name/" + tool.Url_parser(target))
		}
		return tool.Get_redirect("/change/user_name")
	}

	current := user_value(db, target, "user_name")
	if current == "" {
		current = target
	}
	body := `<form method="post"><input name="new_user_name" value="` + tool.HTML_escape(current) + `"><hr class="main_hr"><button type="submit">` + tool.Get_language(db, "save", true) + `</button></form>`

	// 바로위키 : 지금까지 쓴 이름은 계정 고유 번호에 매달려 남는다
	body += `<hr class="main_hr"><a href="/baro/account/name?user=` + tool.Url_parser(target) + `">` + tool.Baro_t("계정명 변경 이력") + `</a>`

	return user_form_page(db, config, tool.Get_language(db, "change_user_name", true), body)
}
