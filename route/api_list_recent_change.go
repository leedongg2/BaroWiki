package route

import (
	"database/sql"
	"opennamu/route/tool"
	"strings"
)

type recent_change_key struct{ id, title string }
type recent_change_detail struct{ date, ip, send, length, hide, kind string }

// 한 페이지의 이력만 묶어서 가져온다. 빠진 이력과 중복 이력이 있어도
// 최근 변경 목록의 순서와 줄 수는 원래 목록을 따른다.
func recent_change_details(db *sql.DB, keys []recent_change_key) map[recent_change_key]recent_change_detail {
	result := map[recent_change_key]recent_change_detail{}
	if len(keys) == 0 {
		return result
	}
	conditions := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys)*2)
	seen := map[recent_change_key]bool{}
	for _, key := range keys {
		if seen[key] {
			continue
		}
		seen[key] = true
		conditions = append(conditions, "(title = ? and id = ?)")
		args = append(args, key.title, key.id)
	}
	rows := tool.Query_DB(db, "select id, title, date, ip, send, leng, hide, type from history where "+strings.Join(conditions, " or "), args...)
	defer rows.Close()
	for rows.Next() {
		var key recent_change_key
		var detail recent_change_detail
		if err := rows.Scan(&key.id, &key.title, &detail.date, &detail.ip, &detail.send, &detail.length, &detail.hide, &detail.kind); err != nil {
			panic(err)
		}
		if _, exists := result[key]; !exists {
			result[key] = detail
		}
	}
	if err := rows.Err(); err != nil {
		panic(err)
	}
	return result
}

func Api_list_recent_change(config tool.Config, set_type string, limit string, num string) map[string]any {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if set_type == "edit" {
		set_type = ""
	}

	limit_int := tool.Str_to_int(limit)
	if limit_int > 50 || limit_int < 0 {
		limit_int = 50
	}

	page_int := tool.Str_to_int(num)
	if page_int > 0 {
		page_int = (page_int * limit_int) - limit_int
	} else {
		page_int = 0
	}

	rows := tool.Query_DB(
		db,
		"select id, title from rc where type = ? order by date desc limit ?, ?",
		set_type,
		page_int,
		limit_int,
	)
	defer rows.Close()
	keys := []recent_change_key{}
	for rows.Next() {
		var key recent_change_key
		if err := rows.Scan(&key.id, &key.title); err != nil {
			panic(err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		panic(err)
	}
	rows.Close()
	details := recent_change_details(db, keys)

	data_list := [][]string{}

	admin_auth := tool.Check_acl(db, "", "", "hidel_auth", config.IP)
	ip_parser_temp := map[string][]string{}

	for _, key := range keys {
		id, title := key.id, key.title
		detail := details[key]
		date, ip, send := detail.date, detail.ip, detail.send
		leng, hide, type_data := detail.length, detail.hide, detail.kind

		var ip_pre string
		var ip_render string

		if _, ok := ip_parser_temp[ip]; ok {
			ip_pre = ip_parser_temp[ip][0]
			ip_render = ip_parser_temp[ip][1]
		} else {
			ip_pre = tool.IP_preprocess(db, ip, config.IP)[0]
			ip_render = tool.IP_parser(db, ip, config.IP)

			ip_parser_temp[ip] = []string{ip_pre, ip_render}
		}

		if hide == "" || admin_auth {
			data_list = append(data_list, []string{
				id,
				tool.Namespace_display(title),
				date,
				ip_pre,
				send,
				leng,
				hide,
				ip_render,
				type_data,
			})
		} else {
			data_list = append(data_list, []string{"", "", "", "", "", "", hide, "", ""})
		}
	}

	return_data := make(map[string]any)
	return_data["response"] = "ok"
	return_data["language"] = api_history_language(db)
	return_data["auth"] = api_history_auth(db, config.IP)
	return_data["data"] = data_list

	return return_data
}
