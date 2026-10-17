package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  사용자 지정 테마  /baro/theme
//
//    /baro/theme              공식 테마 · 공개 테마 · 내 테마 목록
//    /baro/theme/<uuid>       테마 하나 (비공개도 링크로는 열린다)
//    /baro/theme/new          만들기
//    /baro/theme/edit/<uuid>  고치기
//    /baro/theme/delete/<uuid> 지우기
//    /baro/theme/use          쓰기 · 그만 쓰기
//    /baro/theme/official/<uuid> 공식 채택 · 내림 (운영진)
// ============================================================

// baro_theme_menu : 화면 아래 차림표
func baro_theme_menu() [][]any {
	return [][]any{{"/baro", "바로위키"}, {"/baro/theme", tool.Baro_t("테마")}}
}

// baro_theme_tabs : 탭
func baro_theme_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/theme", tool.Baro_t("테마 목록")},
		{"/baro/theme/new", tool.Baro_t("테마 만들기")},
	}, active)
}

// baro_theme_page : 쪽 번호
func baro_theme_page(values url.Values, name string) int {
	if values == nil {
		return 1
	}

	page := tool.Str_to_int(values.Get(name))
	if page < 1 {
		return 1
	}

	return page
}

// View_baro_theme_list : 테마 목록 (/baro/theme)
func View_baro_theme_list(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	alert := ""
	if values != nil {
		alert = values.Get("bw_alert")
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("테마")}})
	body += tool.Baro_hero(
		tool.Baro_t("테마"),
		tool.Baro_t("위키 색을 내 마음대로 꾸며 두고 불러다 씁니다."),
		"",
	)
	body += baro_theme_tabs("/baro/theme")

	if alert != "" {
		body += tool.Baro_alert("ok", alert)
	}

	body += tool.Baro_theme_preview_css()

	// --- 지금 쓰고 있는 테마 ---
	body += baro_theme_now_card(db, config)

	// --- 공식 테마 ---
	body += tool.Baro_card_ex(
		tool.Baro_t("공식 테마"),
		tool.Baro_t("운영진이 채택한 테마입니다."),
		baro_theme_table(db, config, "official", ""),
		"",
	)

	// --- 공개 테마 ---
	page := baro_theme_page(values, "page")
	body += tool.Baro_card_ex(
		tool.Baro_t("공개 테마"),
		tool.Baro_t("만든 사람이 공개로 둔 테마입니다."),
		baro_theme_table_page(db, config, "public", "", page),
		"",
	)
	body += tool.Baro_paging(page, tool.Baro_theme_count(db, "public", ""), tool.Baro_page_size,
		"/baro/theme?page={}")

	// --- 내 테마 ---
	if tool.Baro_is_user(config.IP) {
		body += tool.Baro_card_ex(
			tool.Baro_t("내가 만든 테마"),
			tool.Baro_t("비공개 테마도 여기에 모두 보입니다."),
			baro_theme_table(db, config, "mine", config.IP)+
				tool.Baro_button("/baro/theme/new", tool.Baro_t("테마 만들기"), "primary"),
			"",
		)
	} else {
		body += tool.Baro_note(tool.Baro_t("로그인하면 테마를 만들고 쓸 수 있습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("알아 두기"), tool.Baro_list([]string{
		tool.Baro_t_esc("테마마다 변하지 않는 고유 번호가 붙습니다. 주소는 /baro/theme/<고유 번호> 입니다."),
		tool.Baro_t_esc("공개로 두면 이 목록에 제목과 제작자가 오릅니다. 비공개면 링크를 아는 사람만 열 수 있습니다."),
		tool.Baro_t_esc("테마는 밝은 쪽과 어두운 쪽 색을 따로 담습니다. 다크 모드를 켜면 어두운 쪽이 쓰입니다."),
		tool.Baro_t_esc("값을 비워 둔 색은 스킨 기본값을 그대로 씁니다. 고치고 싶은 것만 고치면 됩니다."),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("테마"), body, baro_theme_menu())
}

// baro_theme_now_card : 지금 쓰고 있는 테마
func baro_theme_now_card(db *sql.DB, config tool.Config) string {
	if !tool.Baro_is_user(config.IP) {
		return ""
	}

	uuid := tool.Baro_theme_now(db, config.IP)
	if uuid == "" {
		return tool.Baro_card(tool.Baro_t("지금 쓰는 테마"),
			tool.Baro_note(tool.Baro_t("기본 색을 쓰고 있습니다.")))
	}

	row, found := tool.Baro_theme_get(db, uuid)
	if !found {
		return tool.Baro_card(tool.Baro_t("지금 쓰는 테마"),
			tool.Baro_alert("warn", tool.Baro_t("쓰고 있던 테마가 사라졌습니다. 기본 색으로 보입니다.")))
	}

	inner := tool.Baro_dl([][]string{
		{tool.Baro_t("제목"), `<a href="/baro/theme/` + tool.Url_parser(row.UUID) + `">` +
			tool.HTML_escape(row.Title) + `</a>`},
		{tool.Baro_t("제작자"), tool.Baro_user_link(db, row.Maker, config.IP)},
	})
	inner += tool.Baro_form("/baro/theme/use",
		tool.Baro_hidden("uuid", "")+tool.Baro_submit(tool.Baro_t("기본 색으로 돌아가기"), ""))

	return tool.Baro_card(tool.Baro_t("지금 쓰는 테마"), inner)
}

// baro_theme_table : 테마 목록 표
func baro_theme_table(db *sql.DB, config tool.Config, kind string, maker string) string {
	return baro_theme_rows_html(db, config, tool.Baro_theme_list(db, kind, maker, 0, tool.Baro_page_size), kind)
}

// baro_theme_table_page : 쪽을 나눈 테마 목록 표
func baro_theme_table_page(db *sql.DB, config tool.Config, kind string, maker string, page int) string {
	offset := (page - 1) * tool.Baro_page_size

	return baro_theme_rows_html(db, config, tool.Baro_theme_list(db, kind, maker, offset, tool.Baro_page_size), kind)
}

// baro_theme_rows_html : 목록 줄들을 표로
func baro_theme_rows_html(db *sql.DB, config tool.Config, list []tool.Baro_theme_row, kind string) string {
	now := tool.Baro_theme_now(db, config.IP)

	rows := [][]string{}
	for _, row := range list {
		title := `<a href="/baro/theme/` + tool.Url_parser(row.UUID) + `">` + tool.HTML_escape(row.Title) + `</a>`

		mark := ""
		if row.Official {
			mark += " " + tool.Baro_badge(tool.Baro_t("공식"), "ok")
		}
		if !row.Baro_theme_public() {
			mark += " " + tool.Baro_badge(tool.Baro_t("비공개"), "muted")
		}
		if row.UUID == now {
			mark += " " + tool.Baro_badge(tool.Baro_t("쓰는 중"), "info")
		}

		rows = append(rows, []string{
			title + mark,
			tool.Baro_user_link(db, row.Maker, config.IP),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Description), 60)),
			tool.Baro_time_cell(row.Date),
		})
	}

	empty := tool.Baro_t("아직 공개된 테마가 없습니다.")
	switch kind {
	case "official":
		empty = tool.Baro_t("아직 공식 테마가 없습니다.")
	case "mine":
		empty = tool.Baro_t("아직 만든 테마가 없습니다.")
	}

	return tool.Baro_table(
		[]string{tool.Baro_t("제목"), tool.Baro_t("제작자"), tool.Baro_t("설명"), tool.Baro_t("만든 때")},
		rows, empty,
	)
}

// View_baro_theme_view : 테마 하나 (/baro/theme/<uuid>)
//
//	비공개 테마도 링크를 알면 열린다. 목록에 오르지 않을 뿐이다.
func View_baro_theme_view(config tool.Config, uuid string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, found := tool.Baro_theme_get(db, strings.TrimSpace(uuid))
	if !found {
		body := tool.Baro_alert("danger", tool.Baro_t("그런 테마가 없습니다. 주소를 다시 확인해 주세요."))
		body += tool.Baro_button("/baro/theme", tool.Baro_t("테마 목록"), "primary")

		return tool.Baro_page(db, config, tool.Baro_t("테마"), body, baro_theme_menu())
	}

	alert := ""
	if values != nil {
		alert = values.Get("bw_alert")
	}

	tool.Baro_theme_view_bump(db, row.UUID)

	can_edit := tool.Baro_theme_can_edit(db, row, config.IP)
	now := tool.Baro_theme_now(db, config.IP)

	mark := ""
	if row.Official {
		mark += " " + tool.Baro_badge(tool.Baro_t("공식 테마"), "ok")
	}
	if row.Baro_theme_public() {
		mark += " " + tool.Baro_badge(tool.Baro_t("공개"), "info")
	} else {
		mark += " " + tool.Baro_badge(tool.Baro_t("비공개"), "muted")
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"}, {"/baro/theme", tool.Baro_t("테마")}, {"", row.Title},
	})
	body += tool.Baro_hero_html(row.Title,
		tool.Baro_t_esc("만든 사람 ")+tool.Baro_user_link(db, row.Maker, config.IP)+mark, "")

	if alert != "" {
		body += tool.Baro_alert("ok", alert)
	}

	body += tool.Baro_theme_preview_css()

	if strings.TrimSpace(row.Description) != "" {
		body += tool.Baro_card(tool.Baro_t("설명"), `<p>`+tool.HTML_escape(row.Description)+`</p>`)
	}

	// --- 미리보기 ---
	body += tool.Baro_card(tool.Baro_t("미리보기"), tool.Baro_grid([]string{
		tool.Baro_theme_preview(row, false),
		tool.Baro_theme_preview(row, true),
	}, 2))

	// --- 쓰기 ---
	if tool.Baro_is_user(config.IP) {
		use := ""
		if row.UUID == now {
			use = tool.Baro_alert("info", tool.Baro_t("지금 쓰고 있는 테마입니다."))
			use += tool.Baro_form("/baro/theme/use",
				tool.Baro_hidden("uuid", "")+tool.Baro_submit(tool.Baro_t("그만 쓰기"), ""))
		} else {
			use = tool.Baro_form("/baro/theme/use",
				tool.Baro_hidden("uuid", row.UUID)+tool.Baro_submit(tool.Baro_t("이 테마 쓰기"), "primary"))
		}

		body += tool.Baro_card(tool.Baro_t("쓰기"), use+
			tool.Baro_note(tool.Baro_t("다크 모드를 켜면 어두운 쪽 색이 쓰입니다.")))
	} else {
		body += tool.Baro_note(tool.Baro_t("로그인하면 이 테마를 쓸 수 있습니다."))
	}

	// --- 담긴 색 ---
	body += baro_theme_color_card(row)

	// --- 테마 정보 ---
	body += tool.Baro_card(tool.Baro_t("테마 정보"), tool.Baro_dl([][]string{
		{tool.Baro_t("고유 번호"), tool.Baro_code(row.UUID)},
		{tool.Baro_t("테마 링크"), tool.Baro_code("/baro/theme/" + row.UUID)},
		{tool.Baro_t("만든 때"), tool.Baro_time_cell(row.Date)},
		{tool.Baro_t("마지막 고친 때"), tool.Baro_time_cell(row.EditDate)},
		{tool.Baro_t("열어 본 횟수"), tool.HTML_escape(tool.Baro_comma(row.Views))},
	}))

	// --- 관리 ---
	menu := ""
	if can_edit {
		menu += tool.Baro_button("/baro/theme/edit/"+tool.Url_parser(row.UUID), tool.Baro_t("고치기"), "primary") + " "
	}

	if tool.Baro_can(db, config.IP, "system") {
		text := tool.Baro_t("공식 테마로 채택")
		if row.Official {
			text = tool.Baro_t("공식 테마에서 내리기")
		}

		menu += tool.Baro_form("/baro/theme/official/"+tool.Url_parser(row.UUID),
			tool.Baro_input("reason", tool.Baro_t("까닭 (선택)"), "", "", "")+tool.Baro_submit(text, ""))
	}

	if menu != "" {
		body += tool.Baro_card(tool.Baro_t("관리"), menu)
	}

	if can_edit {
		body += tool.Baro_danger_zone(tool.Baro_t("테마 지우기"),
			tool.Baro_note(tool.Baro_t("지우면 되돌릴 수 없습니다. 이 테마를 쓰고 있던 사람은 기본 색으로 돌아갑니다."))+
				tool.Baro_form("/baro/theme/delete/"+tool.Url_parser(row.UUID),
					tool.Baro_input("reason", tool.Baro_t("지우는 까닭"), "", "", "")+
						tool.Baro_checkbox("sure", tool.Baro_t("정말 지웁니다"), "on", false, "")+
						tool.Baro_submit(tool.Baro_t("테마 지우기"), "danger")))
	}

	return tool.Baro_page(db, config, row.Title, body, baro_theme_menu())
}

// baro_theme_color_card : 이 테마가 담고 있는 색 목록
func baro_theme_color_card(row tool.Baro_theme_row) string {
	body := ""

	for _, group := range tool.Baro_theme_group_list() {
		rows := [][]string{}

		for _, token := range tool.Baro_theme_token_list() {
			if token.Group != group {
				continue
			}

			saved, ok := row.Data[token.Key]
			if !ok {
				continue
			}

			light_value, dark_value := tool.Baro_theme_value_split(saved)
			if light_value == "" && dark_value == "" {
				continue
			}

			rows = append(rows, []string{
				tool.Baro_t_esc(token.Name),
				baro_theme_swatch(light_value, token.Light),
				baro_theme_swatch(dark_value, token.Dark),
			})
		}

		if len(rows) == 0 {
			continue
		}

		body += tool.Baro_section(group, tool.Baro_table(
			[]string{tool.Baro_t("항목"), tool.Baro_t("밝은 쪽"), tool.Baro_t("어두운 쪽")}, rows, ""))
	}

	if row.Data["fold"] == "off" {
		body += tool.Baro_note(tool.Baro_t("문단 접기 표시를 감춥니다."))
	}

	if body == "" {
		body = tool.Baro_empty(tool.Baro_t("고친 색이 없습니다. 기본 색 그대로입니다."))
	}

	return tool.Baro_card(tool.Baro_t("담긴 색"), body)
}

// baro_theme_swatch : 색 한 칸 (네모 + 값)
func baro_theme_swatch(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return tool.Baro_t_esc("스킨 기본값 ") + tool.HTML_escape(fallback)
	}

	return `<span class="bw_theme_sw" style="background:` + tool.HTML_escape(value) + `"></span>` +
		tool.HTML_escape(value)
}
