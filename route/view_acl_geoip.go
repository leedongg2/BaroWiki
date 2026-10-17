package route

import (
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// View_acl_geoip : GeoIP 대역 관리
//
//	앞단(CDN)이 나라를 붙여 주면 이 표는 없어도 된다.
//	앞단이 없을 때 「이 대역은 어느 나라」 를 직접 넣어 두는 곳이다.
func View_acl_geoip(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Check_acl(db, "", "", "acl_auth", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	message := ""

	if values != nil {
		switch values.Get("action") {
		case "add":
			if tool.Acl_geoip_range_add(db, values.Get("area"), values.Get("country"), values.Get("note"), config.IP) {
				tool.Do_insert_auth_history(db, config.IP, "geoip add ("+values.Get("area")+" / "+values.Get("country")+")")
			} else {
				message = tool.Baro_t("대역이나 나라 코드를 알아보지 못했습니다. IPv4 대역과 두 글자 나라 코드로 적어 주세요.")
			}
		case "delete":
			tool.Acl_geoip_range_delete(db, values.Get("id"))
			tool.Do_insert_auth_history(db, config.IP, "geoip delete ("+values.Get("id")+")")
		case "header":
			header_name := strings.TrimSpace(values.Get("geoip_header"))
			tool.Exec_DB(db, "delete from other where name = ?", tool.Acl_geoip_header_setting)
			tool.Exec_DB(
				db,
				"insert into other (name, data, coverage) values (?, ?, '')",
				tool.Acl_geoip_header_setting, header_name,
			)
			tool.Acl_geoip_cache_clear()
			tool.Do_insert_auth_history(db, config.IP, "geoip header ("+header_name+")")
		}

		if message == "" {
			return tool.Get_redirect("/acl_geoip")
		}
	}

	body := ""
	if message != "" {
		body += `<div class="namu_form_help">` + tool.HTML_escape(message) + `</div><hr class="main_hr">`
	}

	// --- 앞단 머리글 설정 ---
	header_now := tool.Wiki_setting(db, tool.Acl_geoip_header_setting)
	body += `<h2>` + tool.HTML_escape(tool.Baro_t("앞단이 알려 주는 나라")) + `</h2>`
	body += `<form method="post"><input type="hidden" name="action" value="header">`
	body += `<input class="namu_input" name="geoip_header" value="` + tool.HTML_escape(header_now) + `" placeholder="CF-IPCountry"> `
	body += `<button type="submit">` + tool.HTML_escape(tool.Get_language(db, "save", true)) + `</button></form>`
	body += `<div class="namu_form_help">` +
		tool.HTML_escape(tool.Baro_t("Cloudflare 같은 앞단 뒤에 있을 때만 적으세요. 위키가 바로 열려 있으면 이용자가 이 머리글을 지어 보낼 수 있습니다.")) +
		` ` + tool.HTML_escape(tool.Baro_t("비워 두면 쓰지 않습니다.")) + `</div>`
	body += `<hr class="main_hr">`

	// --- 대역 넣기 ---
	body += `<h2>` + tool.HTML_escape(tool.Baro_t("대역 넣기")) + `</h2>`
	body += `<form method="post"><input type="hidden" name="action" value="add">`
	body += `<input class="namu_input" name="area" placeholder="1.2.3.0/24"> `
	body += `<input class="namu_input" name="country" placeholder="KR" maxlength="2" size="4"> `
	body += `<input class="namu_input" name="note" placeholder="` + tool.HTML_escape(tool.Baro_t("메모")) + `"> `
	body += `<button type="submit">` + tool.HTML_escape(tool.Baro_t("넣기")) + `</button></form>`
	body += `<div class="namu_form_help">` +
		tool.HTML_escape(tool.Baro_t("1.2.3.0/24 · 1.2.3.0-1.2.3.255 · 1.2.3.4 모양을 받습니다. (IPv4 만)")) + `</div>`
	body += `<hr class="main_hr">`

	// --- 넣어 둔 대역 ---
	rows := tool.Query_DB(db, "select id, area, country, note, actor, date from baro_geoip order by start_num + 0 asc")
	defer rows.Close()

	table := `<div class="namu_table_wrap"><table class="namu_table"><thead><tr>` +
		`<th>` + tool.HTML_escape(tool.Baro_t("대역")) + `</th>` +
		`<th>` + tool.HTML_escape(tool.Baro_t("나라")) + `</th>` +
		`<th>` + tool.HTML_escape(tool.Baro_t("메모")) + `</th>` +
		`<th>` + tool.HTML_escape(tool.Baro_t("넣은 사람")) + `</th>` +
		`<th></th></tr></thead><tbody>`

	count := 0
	for rows.Next() {
		id, area, country, note, actor, date := "", "", "", "", "", ""
		if rows.Scan(&id, &area, &country, &note, &actor, &date) != nil {
			continue
		}
		count++

		table += `<tr>`
		table += `<td>` + tool.HTML_escape(area) + `</td>`
		table += `<td>` + tool.HTML_escape(country) + `</td>`
		table += `<td>` + tool.HTML_escape(note) + `</td>`
		table += `<td>` + tool.HTML_escape(actor) + `</td>`
		table += `<td><form method="post"><input type="hidden" name="action" value="delete">` +
			`<input type="hidden" name="id" value="` + tool.HTML_escape(id) + `">` +
			`<button type="submit">` + tool.HTML_escape(tool.Baro_t("빼기")) + `</button></form></td>`
		table += `</tr>`
	}

	if count == 0 {
		table += `<tr><td colspan="5">` + tool.HTML_escape(tool.Baro_t("넣어 둔 대역이 없습니다.")) + `</td></tr>`
	}

	table += `</tbody></table></div>`
	body += `<h2>` + tool.HTML_escape(tool.Baro_t("넣어 둔 대역")) + `</h2>` + table

	body += `<div class="namu_form_help">` +
		tool.HTML_escape(tool.Baro_t("ACL 에서 이렇게 부릅니다 :")) + ` <code>거부:geoip:KR</code></div>`

	return tool.Get_template(
		db, config,
		tool.Baro_t("GeoIP 대역"),
		body,
		[]any{},
		[][]any{
			{"acl_group", tool.Baro_t("ACL 그룹")},
			{"manager", tool.Get_language(db, "admin", true)},
		},
		map[string]string{},
	)
}
