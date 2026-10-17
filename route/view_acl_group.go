package route

import (
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  ACL 그룹 · GeoIP 대역 관리 화면
//
//  ACL 규칙에서 `aclgroup:이름` 과 `geoip:KR` 로 부르는 것들을 여기서 채운다.
//  둘 다 ACL 을 다루는 일이므로 ACL 권한(`acl_auth`)을 가진 운영자만 들어온다.
// ============================================================

// View_acl_group : ACL 그룹 관리
func View_acl_group(config tool.Config, group string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Check_acl(db, "", "", "acl_auth", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	if values != nil {
		switch values.Get("action") {
		case "add":
			tool.Acl_group_add(
				db,
				values.Get("group_name"),
				values.Get("kind"),
				values.Get("target"),
				values.Get("note"),
				config.IP,
				strings.TrimSpace(values.Get("end_date")),
			)
			tool.Do_insert_auth_history(db, config.IP, "acl_group add ("+values.Get("group_name")+" / "+values.Get("target")+")")
		case "delete":
			tool.Acl_group_delete(db, values.Get("id"))
			tool.Do_insert_auth_history(db, config.IP, "acl_group delete ("+values.Get("id")+")")
		}

		target_group := values.Get("group_name")
		if target_group == "" {
			target_group = group
		}
		if target_group != "" {
			return tool.Get_redirect("/acl_group/" + tool.Url_parser(target_group))
		}

		return tool.Get_redirect("/acl_group")
	}

	body := ""

	// --- 그룹 목록 ---
	names := tool.Acl_group_name_list(db)
	list_html := ""
	for _, one := range names {
		mark := ""
		if one == group {
			mark = ` class="opennamu_same_link"`
		}
		list_html += `<a` + mark + ` href="/acl_group/` + tool.Url_parser(one) + `">` + tool.HTML_escape(one) + `</a> `
	}
	if list_html == "" {
		list_html = tool.HTML_escape(tool.Baro_t("아직 만들어 둔 그룹이 없습니다."))
	}
	body += `<h2>` + tool.HTML_escape(tool.Baro_t("그룹")) + `</h2>` + list_html + `<hr class="main_hr">`

	// --- 넣기 ---
	kind_select := `<select name="kind">`
	for _, one := range tool.Acl_group_kind_list() {
		kind_select += `<option value="` + one[0] + `">` + tool.HTML_escape(tool.Baro_t(one[1])) + `</option>`
	}
	kind_select += `</select>`

	body += `<h2>` + tool.HTML_escape(tool.Baro_t("넣기")) + `</h2>`
	body += `<form method="post">`
	body += `<input type="hidden" name="action" value="add">`
	body += `<input class="namu_input" name="group_name" value="` + tool.HTML_escape(group) + `" placeholder="` + tool.HTML_escape(tool.Baro_t("그룹 이름")) + `"> `
	body += kind_select + ` `
	body += `<input class="namu_input" name="target" placeholder="` + tool.HTML_escape(tool.Baro_t("계정 이름 또는 아이피 · 대역")) + `"> `
	body += `<input class="namu_input" type="datetime-local" name="end_date"> `
	body += `<input class="namu_input" name="note" placeholder="` + tool.HTML_escape(tool.Baro_t("메모")) + `"> `
	body += `<button type="submit">` + tool.HTML_escape(tool.Baro_t("넣기")) + `</button>`
	body += `</form>`
	body += `<div class="namu_form_help">` + tool.HTML_escape(tool.Baro_t("만료 일시를 비우면 무기한입니다. 아이피는 192.168.0.1 또는 192.168.0.0/16 처럼 적습니다.")) + `</div>`
	body += `<hr class="main_hr">`

	// --- 든 것 ---
	if group != "" {
		rows := tool.Acl_group_list(db, group)
		table := `<div class="namu_table_wrap"><table class="namu_table"><thead><tr>` +
			`<th>` + tool.HTML_escape(tool.Baro_t("갈래")) + `</th>` +
			`<th>` + tool.HTML_escape(tool.Baro_t("대상")) + `</th>` +
			`<th>` + tool.HTML_escape(tool.Baro_t("만료")) + `</th>` +
			`<th>` + tool.HTML_escape(tool.Baro_t("메모")) + `</th>` +
			`<th>` + tool.HTML_escape(tool.Baro_t("넣은 사람")) + `</th>` +
			`<th></th></tr></thead><tbody>`

		if len(rows) == 0 {
			table += `<tr><td colspan="6">` + tool.HTML_escape(tool.Baro_t("아직 든 것이 없습니다.")) + `</td></tr>`
		}

		for _, one := range rows {
			kind_name := tool.Baro_t("계정")
			if one.Kind == "ip" {
				kind_name = tool.Baro_t("아이피 · 대역")
			}

			end_text := tool.Baro_t("무기한")
			if one.EndDate != "" {
				end_text = one.EndDate
				if tool.Baro_is_past(one.EndDate) {
					end_text += " " + tool.Baro_t("(지남)")
				}
			}

			table += `<tr>`
			table += `<td>` + tool.HTML_escape(kind_name) + `</td>`
			table += `<td>` + tool.HTML_escape(one.Target) + `</td>`
			table += `<td>` + tool.HTML_escape(end_text) + `</td>`
			table += `<td>` + tool.HTML_escape(one.Note) + `</td>`
			table += `<td>` + tool.HTML_escape(one.Actor) + `</td>`
			table += `<td><form method="post"><input type="hidden" name="action" value="delete">` +
				`<input type="hidden" name="id" value="` + tool.HTML_escape(one.ID) + `">` +
				`<input type="hidden" name="group_name" value="` + tool.HTML_escape(group) + `">` +
				`<button type="submit">` + tool.HTML_escape(tool.Baro_t("빼기")) + `</button></form></td>`
			table += `</tr>`
		}

		table += `</tbody></table></div>`
		body += `<h2>` + tool.HTML_escape(group) + `</h2>` + table
	}

	body += `<div class="namu_form_help">` +
		tool.HTML_escape(tool.Baro_t("ACL 에서 이렇게 부릅니다 :")) + ` <code>거부:aclgroup:` + tool.HTML_escape(group) + `</code></div>`

	return tool.Get_template(
		db, config,
		tool.Baro_t("ACL 그룹"),
		body,
		[]any{},
		[][]any{
			{"acl_geoip", tool.Baro_t("GeoIP 대역")},
			{"manager", tool.Get_language(db, "admin", true)},
		},
		map[string]string{},
	)
}
