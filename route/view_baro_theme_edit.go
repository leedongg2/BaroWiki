package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  테마 만들기 · 고치기 · 지우기 · 쓰기 · 공식 채택
// ============================================================

// baro_theme_form_read : 폼에서 테마 값을 읽는다. (테마, 잘못된 곳)
func baro_theme_form_read(values url.Values, before tool.Baro_theme_row) (tool.Baro_theme_row, string) {
	row := before
	row.Title = strings.TrimSpace(values.Get("title"))
	row.Description = strings.TrimSpace(values.Get("description"))

	row.Status = "private"
	if values.Get("open") != "" {
		row.Status = "public"
	}

	data := map[string]string{}

	for _, token := range tool.Baro_theme_token_list() {
		light := values.Get("c_" + token.Key)
		dark := values.Get("d_" + token.Key)

		// 「스킨 기본값 쓰기」 를 고른 칸은 담지 않는다.
		if values.Get("off_"+token.Key) != "" {
			continue
		}

		if joined := tool.Baro_theme_value_join(light, dark); joined != "" {
			data[token.Key] = joined
		}
	}

	if values.Get("fold") == "" {
		data["fold"] = "off"
	}

	row.Data = data

	switch {
	case row.Title == "":
		return row, tool.Baro_t("테마 제목을 적어 주세요.")
	case tool.Get_len(row.Title) > tool.Baro_theme_title_max:
		return row, tool.Baro_t("테마 제목이 너무 깁니다.")
	case tool.Get_len(row.Description) > tool.Baro_theme_desc_max:
		return row, tool.Baro_t("설명이 너무 깁니다.")
	}

	return row, ""
}

// baro_theme_form_html : 테마 편집 폼
func baro_theme_form_html(action string, do string, row tool.Baro_theme_row, submit string) string {
	form := tool.Baro_hidden("do", do)
	form += tool.Baro_input("title", tool.Baro_t("제목"), row.Title, tool.Baro_t("예: 밤하늘"), "")
	form += tool.Baro_textarea("description", tool.Baro_t("설명"), row.Description, 3,
		tool.Baro_t("어떤 느낌의 테마인지 한두 줄로"), "")
	form += tool.Baro_checkbox("open", tool.Baro_t("공개하기"), "on", row.Baro_theme_public(),
		tool.Baro_t("공개하면 테마 목록에 제목과 제작자가 오릅니다. 비공개면 링크를 아는 사람만 열 수 있습니다."))
	form += tool.Baro_checkbox("fold", tool.Baro_t("문단 접기 표시 보이기"), "on", row.Data["fold"] != "off",
		tool.Baro_t("끄면 문단 제목 옆의 V표가 보이지 않습니다."))

	form += `<p class="bw_f_h">` +
		tool.Baro_t_esc("칸을 「스킨 기본값」 으로 두면 그 색은 건드리지 않습니다. 고치고 싶은 것만 고치세요.") +
		`</p>`

	for _, group := range tool.Baro_theme_group_list() {
		inner := ""

		for _, token := range tool.Baro_theme_token_list() {
			if token.Group != group {
				continue
			}

			inner += baro_theme_field_html(token, row)
		}

		form += tool.Baro_section(group, inner)
	}

	form += tool.Baro_submit(submit, "primary")

	return tool.Baro_form(action, form)
}

// baro_theme_field_html : 색 한 줄 (밝은 쪽 · 어두운 쪽 · 기본값 쓰기)
func baro_theme_field_html(token tool.Baro_theme_token, row tool.Baro_theme_row) string {
	saved, has := row.Data[token.Key]
	light, dark := tool.Baro_theme_value_split(saved)

	if light == "" {
		light = token.Light
	}
	if dark == "" {
		dark = token.Dark
	}

	pick := func(prefix string, value string, label string) string {
		name := prefix + token.Key

		// 색 고르개와 글자 칸이 서로를 따라간다. (직접 적어 넣을 수도 있게)
		return `<label class="bw_theme_lb">` + tool.Baro_t_esc(label) + `</label>` +
			`<input type="color" value="` + tool.HTML_escape(value) + `" ` +
			`oninput="bw_theme_sync(this, '` + tool.JS_escape(name) + `')">` +
			`<input type="text" id="bw_theme_` + tool.HTML_escape(name) + `" name="` + tool.HTML_escape(name) +
			`" value="` + tool.HTML_escape(value) + `" maxlength="40">`
	}

	control := `<div class="bw_theme_row">`
	control += pick("c_", light, "밝은 쪽")
	control += pick("d_", dark, "어두운 쪽")
	control += `<label class="bw_theme_lb"><input type="checkbox" name="off_` + tool.HTML_escape(token.Key) +
		`" value="on"` + baro_theme_checked(!has) + `> ` + tool.Baro_t_esc("스킨 기본값") + `</label>`
	control += `</div>`

	return tool.Baro_field_row_html(token.Name, control, "")
}

// baro_theme_checked : 체크 표시
func baro_theme_checked(on bool) string {
	if on {
		return ` checked`
	}

	return ""
}

// baro_theme_script : 색 고르개와 글자 칸 잇기
func baro_theme_script() string {
	return `<style>
.bw_theme_lb { font-size: 12px; opacity: 0.8; margin-right: 2px; }
</style><script>
function bw_theme_sync(source, name) {
    var box = document.getElementById('bw_theme_' + name);
    if(box === null) { return; }

    box.value = source.value;

    var off = document.getElementsByName('off_' + name.substring(2));
    for(var i = 0; i < off.length; i++) { off[i].checked = false; }
}
</script>`
}

// View_baro_theme_new : 테마 만들기 (/baro/theme/new)
func View_baro_theme_new(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, stop := tool.Baro_login_wall(db, config, "/baro/theme/new"); stop {
		return wall
	}

	row := tool.Baro_theme_row{Maker: config.IP, Status: "public", Data: map[string]string{}}
	alert := ""

	if values != nil && values.Get("do") == "add" {
		if !tool.Baro_rate_ok(db, "theme_add", config.IP, 20, 3600) {
			alert = tool.Baro_alert("warn", tool.Baro_t("잠시 뒤에 다시 시도해 주세요."))
		} else {
			next, problem := baro_theme_form_read(values, row)
			if problem != "" {
				alert = tool.Baro_alert("warn", problem)
				row = next
			} else {
				next.Maker = config.IP

				uuid := tool.Baro_theme_add(db, next)
				if uuid == "" {
					alert = tool.Baro_alert("danger", tool.Baro_t("테마를 만들지 못했습니다. 다시 시도해 주세요."))
				} else {
					tool.Baro_audit(db, config, "baro_theme_add", "theme:"+uuid, "", next.Title, "")

					return tool.Get_redirect("/baro/theme/" + tool.Url_parser(uuid) +
						"?bw_alert=" + tool.Url_parser(tool.Baro_t("테마를 만들었습니다.")))
				}
			}
		}
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"}, {"/baro/theme", tool.Baro_t("테마")}, {"", tool.Baro_t("만들기")},
	})
	body += tool.Baro_hero(tool.Baro_t("테마 만들기"), tool.Baro_t("고치고 싶은 색만 골라 담습니다."), "")
	body += baro_theme_tabs("/baro/theme/new")
	body += alert
	body += baro_theme_script()
	body += tool.Baro_card("", baro_theme_form_html("/baro/theme/new", "add", row, tool.Baro_t("테마 만들기")))

	return tool.Baro_page(db, config, tool.Baro_t("테마 만들기"), body, baro_theme_menu())
}

// View_baro_theme_edit : 테마 고치기 (/baro/theme/edit/<uuid>)
func View_baro_theme_edit(config tool.Config, uuid string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, stop := tool.Baro_login_wall(db, config, "/baro/theme"); stop {
		return wall
	}

	row, found := tool.Baro_theme_get(db, strings.TrimSpace(uuid))
	if !found {
		return baro_theme_gone(db, config)
	}

	if !tool.Baro_theme_can_edit(db, row, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("테마를 만든 사람과 운영진만 고칠 수 있습니다."))
	}

	alert := ""

	if values != nil && values.Get("do") == "edit" {
		next, problem := baro_theme_form_read(values, row)
		if problem != "" {
			alert = tool.Baro_alert("warn", problem)
			row = next
		} else {
			before := row.Title
			tool.Baro_theme_save(db, next)
			tool.Baro_audit(db, config, "baro_theme_edit", "theme:"+row.UUID, before, next.Title, "")

			return tool.Get_redirect("/baro/theme/" + tool.Url_parser(row.UUID) +
				"?bw_alert=" + tool.Url_parser(tool.Baro_t("테마를 고쳤습니다.")))
		}
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"}, {"/baro/theme", tool.Baro_t("테마")},
		{"/baro/theme/" + tool.Url_parser(row.UUID), row.Title}, {"", tool.Baro_t("고치기")},
	})
	body += tool.Baro_hero(tool.Baro_t("테마 고치기"), row.Title, "")
	body += alert
	body += baro_theme_script()
	body += tool.Baro_card("", baro_theme_form_html("/baro/theme/edit/"+tool.Url_parser(row.UUID),
		"edit", row, tool.Baro_t("고치기")))
	body += tool.Baro_button("/baro/theme/"+tool.Url_parser(row.UUID), tool.Baro_t("테마로 돌아가기"), "")

	return tool.Baro_page(db, config, tool.Baro_t("테마 고치기"), body, baro_theme_menu())
}

// baro_theme_gone : 없는 테마 안내
func baro_theme_gone(db *sql.DB, config tool.Config) string {
	body := tool.Baro_alert("danger", tool.Baro_t("그런 테마가 없습니다."))
	body += tool.Baro_button("/baro/theme", tool.Baro_t("테마 목록"), "primary")

	return tool.Baro_page(db, config, tool.Baro_t("테마"), body, baro_theme_menu())
}

// View_baro_theme_delete : 테마 지우기 (/baro/theme/delete/<uuid>)
func View_baro_theme_delete(config tool.Config, uuid string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, found := tool.Baro_theme_get(db, strings.TrimSpace(uuid))
	if !found {
		return baro_theme_gone(db, config)
	}

	if !tool.Baro_theme_can_edit(db, row, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("테마를 만든 사람과 운영진만 지울 수 있습니다."))
	}

	if values == nil || values.Get("sure") == "" {
		return tool.Get_redirect("/baro/theme/" + tool.Url_parser(row.UUID) +
			"?bw_alert=" + tool.Url_parser(tool.Baro_t("「정말 지웁니다」 를 표시해 주세요.")))
	}

	reason := strings.TrimSpace(values.Get("reason"))

	tool.Baro_theme_delete(db, row.UUID)
	tool.Baro_audit(db, config, "baro_theme_delete", "theme:"+row.UUID, row.Title, "", reason)

	// 남이 만든 것을 운영진이 지웠다면 공개 기록으로도 남긴다.
	if row.Maker != config.IP {
		tool.Baro_public_log(db, "theme_delete", config.IP, "theme:"+row.UUID, reason, row.Title)
	}

	return tool.Get_redirect("/baro/theme?bw_alert=" + tool.Url_parser(tool.Baro_t("테마를 지웠습니다.")))
}

// View_baro_theme_use : 쓸 테마 고르기 (/baro/theme/use)
func View_baro_theme_use(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, stop := tool.Baro_login_wall(db, config, "/baro/theme"); stop {
		return wall
	}

	uuid := ""
	if values != nil {
		uuid = strings.TrimSpace(values.Get("uuid"))
	}

	if uuid == "" {
		tool.Baro_theme_use(db, config.IP, "")

		return tool.Get_redirect("/baro/theme?bw_alert=" +
			tool.Url_parser(tool.Baro_t("기본 색으로 돌아갔습니다.")))
	}

	row, found := tool.Baro_theme_get(db, uuid)
	if !found {
		return tool.Get_redirect("/baro/theme?bw_alert=" + tool.Url_parser(tool.Baro_t("그런 테마가 없습니다.")))
	}

	tool.Baro_theme_use(db, config.IP, row.UUID)

	return tool.Get_redirect("/baro/theme/" + tool.Url_parser(row.UUID) +
		"?bw_alert=" + tool.Url_parser(tool.Baro_t("이 테마를 쓰기 시작했습니다.")))
}

// View_baro_theme_official : 공식 테마 채택 · 내림 (/baro/theme/official/<uuid>)
func View_baro_theme_official(config tool.Config, uuid string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("공식 테마는 운영진만 정할 수 있습니다."))
	}

	row, found := tool.Baro_theme_get(db, strings.TrimSpace(uuid))
	if !found {
		return baro_theme_gone(db, config)
	}

	reason := ""
	if values != nil {
		reason = strings.TrimSpace(values.Get("reason"))
	}

	next := !row.Official
	tool.Baro_theme_official_set(db, row.UUID, next)

	kind := "theme_official_off"
	text := tool.Baro_t("공식 테마에서 내렸습니다.")
	if next {
		kind = "theme_official_on"
		text = tool.Baro_t("공식 테마로 채택했습니다.")

		// 공식으로 올리면 목록에 보여야 하므로 공개로 바꾼다.
		if !row.Baro_theme_public() {
			row.Status = "public"
			tool.Baro_theme_save(db, row)
			text += tool.Baro_t(" 목록에 보이도록 공개로 바꿨습니다.")
		}
	}

	tool.Baro_public_log(db, kind, config.IP, "theme:"+row.UUID, reason, row.Title)
	tool.Baro_audit(db, config, kind, "theme:"+row.UUID, tool.Baro_itoa(baro_theme_bool(row.Official)),
		tool.Baro_itoa(baro_theme_bool(next)), reason)

	// 만든 사람에게 알린다.
	tool.Baro_notify(db, row.Maker, "system", text+" : "+row.Title,
		"/baro/theme/"+tool.Url_parser(row.UUID), reason)

	return tool.Get_redirect("/baro/theme/" + tool.Url_parser(row.UUID) + "?bw_alert=" + tool.Url_parser(text))
}

// baro_theme_bool : 기록에 남길 참·거짓
func baro_theme_bool(on bool) int {
	if on {
		return 1
	}

	return 0
}
