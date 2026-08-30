package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  1-6 공유 카드
//    /baro/card/make  카드 만들기
//    /baro/card/<id>  카드 보기 (Open Graph 미리보기 지원)
//    /baro/card       내가 만든 카드 목록
// ============================================================

// baro_founding_card_row : baro_share_card 한 줄
type baro_founding_card_row struct {
	ID    string
	User  string
	Kind  string
	Title string
	Data  string
	Date  string
	Views string
}

// baro_founding_card_kind_list : 공유 카드 종류
func baro_founding_card_kind_list() [][]string {
	return [][]string{
		{"doc", tool.Baro_t("문서 기록")},
		{"field", tool.Baro_t("분야 개척률 상승")},
		{"badge", tool.Baro_t("배지 획득")},
		{"founder", tool.Baro_t("창립 번호")},
	}
}

// baro_founding_card_kind_name : 카드 종류 이름
func baro_founding_card_kind_name(kind string) string {
	for _, row := range baro_founding_card_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// baro_founding_card_field_value : 분야 카드에 적힐 값 (기록에서 계산한 값)
//
//	전체 목록이 확정된 분야에만 개척률(%)을 붙이고, 나머지는 기록 수만 쓴다.
func baro_founding_card_field_value(field tool.Baro_field_row, recorded int) string {
	if field.Kind == "closed" && field.TargetCount > 0 {
		before := tool.Baro_pct_text(recorded-1, field.TargetCount)
		after := tool.Baro_pct_text(recorded, field.TargetCount)

		if before != after {
			return before + " → " + after
		}

		return tool.Baro_comma(recorded) + tool.Baro_t("번째 기록 (개척률 ") + after + ")"
	}

	return tool.Baro_comma(recorded) + tool.Baro_t("번째 기록")
}

// baro_founding_card_get : 카드 하나 읽기
func baro_founding_card_get(db *sql.DB, id string) (baro_founding_card_row, bool) {
	row := baro_founding_card_row{}
	if strings.TrimSpace(id) == "" {
		return row, false
	}

	found := tool.QueryRow_DB(
		db,
		"select id, user, kind, title, data, date, views from baro_share_card where id = ?",
		[]any{&row.ID, &row.User, &row.Kind, &row.Title, &row.Data, &row.Date, &row.Views},
		id,
	)

	return row, found
}

// baro_founding_card_rows : 내가 만든 카드 목록
func baro_founding_card_rows(db *sql.DB, user string, offset int, limit int) []baro_founding_card_row {
	out := []baro_founding_card_row{}

	rows := tool.Query_DB(
		db,
		"select id, user, kind, title, data, date, views from baro_share_card where user = ? order by id + 0 desc limit ?, ?",
		user, offset, limit,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_founding_card_row{}
		if err := rows.Scan(&row.ID, &row.User, &row.Kind, &row.Title, &row.Data, &row.Date, &row.Views); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_founding_card_my_badges : 내가 가진 배지 (카드 선택용)
func baro_founding_card_my_badges(db *sql.DB, user string) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(db, "select badge from baro_user_badge where user = ? order by id + 0 desc limit ?, ?", user, 0, 50)
	defer rows.Close()

	for rows.Next() {
		badge := ""
		if err := rows.Scan(&badge); err != nil {
			continue
		}

		out = append(out, []string{badge, baro_founding_badge_name(db, badge)})
	}

	return out
}

// baro_founding_card_my_events : 내 최근 기여 (카드 만들기 도우미)
func baro_founding_card_my_events(db *sql.DB, user string) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(
		db,
		"select id, kind, title, date from baro_contrib_event where user = ? and title != '' order by id + 0 desc limit ?, ?",
		user, 0, 10,
	)
	defer rows.Close()

	for rows.Next() {
		id, kind, title, date := "", "", "", ""
		if err := rows.Scan(&id, &kind, &title, &date); err != nil {
			continue
		}

		out = append(out, []string{id, kind, title, date})
	}

	return out
}

// baro_founding_card_build : 입력값으로 카드 문구를 만든다
//
//	반환값 : (문구, 부제, 저장할 데이터, 실패 사유)
func baro_founding_card_build(db *sql.DB, config tool.Config, values url.Values) (string, string, map[string]string, string) {
	kind := values.Get("kind")
	data := map[string]string{"kind": kind}

	switch kind {
	case "doc":
		title := tool.Baro_one_line(strings.TrimSpace(values.Get("title")))
		if title == "" {
			return "", "", data, tool.Baro_t("문서 제목을 입력해 주세요.")
		}

		title = tool.Baro_trunc(title, 80)

		// 하지 않은 기록을 했다고 적을 수 없게 내 기록을 확인한다.
		if !baro_founding_my_doc_event(db, config.IP, title) {
			return "", "", data, tool.Baro_t("내가 기록한 문서만 카드로 만들 수 있습니다. 문서 이름을 정확히 적었는지 확인하거나 아래 \"최근 내 기여에서 바로 만들기\" 를 이용해 주세요.")
		}

		data["doc"] = title

		return tool.Baro_t("내가 오늘 바로위키에 《") + title + tool.Baro_t("》 문서를 기록했습니다"),
			tool.Baro_t("기록은 사라지지 않습니다."), data, ""

	case "field":
		key := values.Get("field")
		if !baro_founding_field_ok(db, key) {
			return "", "", data, tool.Baro_t("분야를 골라 주세요.")
		}

		// 개척률은 사람이 적어 넣는 값이 아니라 기록에서 계산한 값만 쓴다.
		// (지어낸 개척률을 카드로 퍼뜨리지 않기 위한 규칙)
		if baro_founding_my_field_contrib(db, config.IP, key) < 1 {
			return "", "", data, tool.Baro_t("이 분야에 남긴 기여 기록이 있어야 만들 수 있는 카드입니다.")
		}

		field := tool.Baro_field_get(db, key)
		recorded := baro_founding_field_recorded(db, key)
		if recorded < 1 {
			return "", "", data, tool.Baro_t("아직 이 분야에 집계된 기록이 없어 카드를 만들 수 없습니다.")
		}

		data["field"] = key
		data["recorded"] = tool.Baro_itoa(recorded)

		if field.Kind == "closed" && field.TargetCount > 0 {
			// 전체 목록이 확정된 분야에만 개척률(%)을 붙인다.
			before := tool.Baro_pct_text(recorded-1, field.TargetCount)
			after := tool.Baro_pct_text(recorded, field.TargetCount)

			data["before"] = before
			data["after"] = after
			data["target"] = tool.Baro_itoa(field.TargetCount)

			sub := tool.Baro_t("공인 목록 ") + tool.Baro_comma(field.TargetCount) + tool.Baro_t("건 가운데 ") + tool.Baro_comma(recorded) + tool.Baro_t("건이 기록되었습니다.")

			// 소수점 아래에서 반올림이 같아지면 "0.0%에서 0.0%로" 같은 말이 되므로 기록 수로 적는다.
			if before == after {
				return field.Name + tool.Baro_t(" 분야에 ") + tool.Baro_comma(recorded) + tool.Baro_t("번째 기록을 더했습니다 (개척률 ") + after + ")", sub, data, ""
			}

			return field.Name + tool.Baro_t(" 분야 개척률을 ") + before + tool.Baro_t("에서 ") + after + tool.Baro_t("로 높였습니다"), sub, data, ""
		}

		// 전체 목록이 확정되지 않은 분야는 분모가 없으므로 개척률(%)을 쓰지 않는다.
		return field.Name + tool.Baro_t(" 분야에 ") + tool.Baro_comma(recorded) + tool.Baro_t("번째 기록을 더했습니다"),
			tool.Baro_t("이 분야는 전체 목록이 확정되지 않아 개척률(%) 대신 기록 수를 씁니다."), data, ""

	case "badge":
		badge := strings.TrimSpace(values.Get("badge"))
		if badge == "" {
			return "", "", data, tool.Baro_t("배지를 골라 주세요.")
		}

		found := ""
		if !tool.QueryRow_DB(db, "select id from baro_user_badge where user = ? and badge = ?", []any{&found}, config.IP, badge) {
			return "", "", data, tool.Baro_t("내가 가진 배지만 카드로 만들 수 있습니다.")
		}

		name := baro_founding_badge_name(db, badge)
		data["badge"] = badge

		return tool.Baro_t("바로위키에서 《") + name + tool.Baro_t("》 배지를 얻었습니다"),
			tool.Baro_t("배지는 횟수가 아니라 행동에 대한 기록입니다."), data, ""

	case "founder":
		number := tool.Baro_founder_number(db, config.IP)
		if number == "" {
			return "", "", data, tool.Baro_t("창립 기여자 번호가 확정된 뒤에 만들 수 있는 카드입니다.")
		}

		data["number"] = number

		return tool.Baro_t("바로위키 창립 기여자 #") + number + tool.Baro_t(" 가 되었습니다"),
			tool.Baro_t("100인 창립단의 한 사람입니다."), data, ""
	}

	return "", "", data, tool.Baro_t("카드 종류를 골라 주세요.")
}

// View_baro_founding_card_make : 공유 카드 만들기 (1-6)
func View_baro_founding_card_make(config tool.Config, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/card/make"); need {
		return wall
	}

	fail := ""
	if values != nil {
		switch {
		case tool.Baro_read_only(db):
			fail = tool.Baro_t("지금은 읽기 전용 모드라 카드를 만들 수 없습니다.")
		case !tool.Baro_rate_ok(db, "card_make", config.IP, 30, 3600):
			fail = tool.Baro_t("짧은 시간에 카드를 너무 많이 만들었습니다. 잠시 뒤에 다시 시도해 주세요.")
		}

		if fail == "" {
			text, sub, data, build_fail := baro_founding_card_build(db, config, values)
			fail = build_fail

			if fail == "" {
				data["sub"] = sub
				data["text"] = text

				id := tool.Baro_next_id(db, "baro_share_card")
				tool.Exec_DB(
					db,
					"insert into baro_share_card (id, user, kind, title, data, date, views) values (?, ?, ?, ?, ?, ?, '0')",
					id, config.IP, values.Get("kind"), tool.Baro_trunc(text, 300), tool.Baro_json_enc(data), tool.Baro_now(),
				)
				tool.Baro_metric_bump(db, "share_card", tool.Baro_today(), 1)

				return tool.Get_redirect("/baro/card/" + id)
			}
		}
	}

	kind := "doc"
	if query != nil && query.Get("kind") != "" {
		kind = query.Get("kind")
	}
	if values != nil && values.Get("kind") != "" {
		kind = values.Get("kind")
	}

	doc_title := ""
	if query != nil {
		doc_title = query.Get("title")
	}
	if values != nil {
		doc_title = values.Get("title")
	}

	field_key := ""
	if query != nil {
		field_key = query.Get("field")
	}
	if values != nil {
		field_key = values.Get("field")
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/card", tool.Baro_t("공유 카드")},
		{"", tool.Baro_t("카드 만들기")},
	})
	body += tool.Baro_hero(tool.Baro_t("공유 카드 만들기"), tool.Baro_t("기록한 것을 한 장으로 만들어 친구에게 보여 주세요."), "")
	body += baro_founding_tabs("/baro/card")

	if fail != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(fail))
	}

	form := tool.Baro_select("kind", tool.Baro_t("카드 종류"), baro_founding_card_kind_list(), kind, tool.Baro_t("종류를 고르고 아래에서 해당 칸만 채우면 됩니다."))
	form += tool.Baro_input("title", tool.Baro_t("문서 제목 (문서 기록 카드)"), doc_title, tool.Baro_t("예 : 최동환 독립운동가"), tool.Baro_t(`내가 기록한 문서만 쓸 수 있습니다. "내가 오늘 바로위키에 《문서》 문서를 기록했습니다" 형태로 만들어집니다.`))
	form += tool.Baro_select("field", tool.Baro_t("분야 (분야 개척률 카드)"), baro_founding_field_options(db, tool.Baro_t("분야를 고르세요")), field_key,
		tool.Baro_t("개척률 숫자는 직접 적는 것이 아니라 기록에서 계산해 채웁니다. 그 분야에 남긴 기여가 있어야 만들 수 있습니다."))

	badges := baro_founding_card_my_badges(db, config.IP)
	if len(badges) == 0 {
		form += tool.Baro_field_row_html(tool.Baro_t("배지 (배지 획득 카드)"), tool.Baro_t(`<div class="bw_note">아직 가진 배지가 없습니다.</div>`), "")
	} else {
		form += tool.Baro_select("badge", tool.Baro_t("배지 (배지 획득 카드)"), badges, "", tool.Baro_t("내가 가진 배지만 고를 수 있습니다."))
	}

	number := tool.Baro_founder_number(db, config.IP)
	if number == "" {
		form += tool.Baro_field_row_html(tool.Baro_t("창립 번호 (창립 번호 카드)"), tool.Baro_t(`<div class="bw_note">번호가 확정되면 창립 번호 카드를 만들 수 있습니다.</div>`), "")
	} else {
		form += tool.Baro_field_row_html(tool.Baro_t("창립 번호 (창립 번호 카드)"), tool.Baro_t(`<div class="bw_note">창립 기여자 #`)+tool.HTML_escape(number)+tool.Baro_t(` 로 만들어집니다.</div>`), "")
	}

	form += tool.Baro_submit(tool.Baro_t("카드 만들기"), "primary")

	body += tool.Baro_card(tool.Baro_t("카드 내용"), tool.Baro_form("/baro/card/make", form))

	// 최근 기여에서 바로 만들기
	events := baro_founding_card_my_events(db, config.IP)
	event_rows := [][]string{}
	for _, event := range events {
		event_rows = append(event_rows, []string{
			tool.Baro_badge(tool.Baro_contrib_kind_name(event[1]), "info"),
			tool.Baro_doc_link(event[2]),
			tool.Baro_time_cell(event[3]),
			tool.Baro_button("/baro/card/make?kind=doc&title="+tool.Url_parser(event[2]), "이 문서로 카드 만들기", ""),
		})
	}

	if len(event_rows) == 0 {
		body += tool.Baro_card(tool.Baro_t("최근 내 기여"), tool.Baro_empty(tool.Baro_t("아직 기여 기록이 없습니다. 첫 기여를 남기면 여기에서 바로 카드를 만들 수 있습니다.")))
	} else {
		body += tool.Baro_card(
			tool.Baro_t("최근 내 기여에서 바로 만들기"),
			tool.Baro_table([]string{"종류", "문서", "시각", tool.Baro_t("카드")}, event_rows, tool.Baro_t("아직 기여 기록이 없습니다.")),
		)
	}

	// 분야별로 지금 계산되는 문구를 미리 보여 준다. (숫자를 사람이 적지 않는다는 것을 눈으로 확인)
	field_rows := [][]string{}
	for _, row := range tool.Baro_field_list(db) {
		mine := baro_founding_my_field_contrib(db, config.IP, row.Key)
		recorded := baro_founding_field_recorded(db, row.Key)

		text := tool.Baro_t(`<span class="bw_muted">집계된 기록이 없어 아직 만들 수 없습니다.</span>`)
		switch {
		case mine < 1:
			text = tool.Baro_t(`<span class="bw_muted">이 분야에 남긴 기여가 없습니다.</span>`)
		case recorded < 1:
		default:
			text = tool.HTML_escape(baro_founding_card_field_value(row, recorded))
			if row.Kind != "closed" || row.TargetCount < 1 {
				text += " " + tool.Baro_badge(tool.Baro_t("분모 미확정"), "muted")
			}
		}

		field_rows = append(field_rows, []string{
			tool.HTML_escape(row.Name),
			tool.Baro_itoa(mine) + tool.Baro_t("건"),
			tool.Baro_comma(recorded) + tool.Baro_t("건"),
			text,
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("분야 개척률 카드에 들어갈 숫자"),
		tool.Baro_t("지금 기록으로 계산한 값입니다."),
		tool.Baro_table([]string{"분야", tool.Baro_t("내 기여"), tool.Baro_t("기록 수"), tool.Baro_t("카드에 적힐 값")}, field_rows, tool.Baro_t("등록된 분야가 없습니다.")),
		tool.Baro_note(tool.Baro_t("전체 목록이 확정된 분야에만 개척률(%)을 붙입니다. 분모를 아무도 모르는 분야에 지어낸 백분율을 붙이지 않습니다.")),
	)

	body += tool.Baro_card(
		tool.Baro_t("카드에 대해"),
		tool.Baro_list([]string{
			tool.Baro_t("카드 문구의 숫자와 문서 이름은 <b>실제 기록에서 가져옵니다</b>. 직접 적어 넣을 수 없습니다."),
			tool.Baro_t("카드는 <b>누구나 볼 수 있는 공개 주소</b>로 만들어집니다."),
			tool.Baro_t("카카오톡·디스코드·SNS 에 붙이면 <b>미리보기</b>가 뜹니다."),
			tool.Baro_t("카드에는 사용자 이름과 기록 내용만 담기며 IP·이메일은 담기지 않습니다."),
			tool.Baro_t("만든 카드는 내 카드 목록에서 언제든 지울 수 있습니다."),
		}),
	)

	return tool.Baro_page(db, config, tool.Baro_t("공유 카드 만들기"), body, baro_founding_menu())
}

// View_baro_founding_card_list : 내가 만든 카드 목록 (1-6)
func View_baro_founding_card_list(config tool.Config, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/card"); need {
		return wall
	}

	if values != nil {
		if values.Get("action") == "delete" {
			id := values.Get("id")
			owner := ""
			if tool.QueryRow_DB(db, "select user from baro_share_card where id = ?", []any{&owner}, id) && owner == config.IP {
				tool.Exec_DB(db, "delete from baro_share_card where id = ?", id)

				return tool.Get_redirect("/baro/card?done=deleted")
			}

			return tool.Get_redirect("/baro/card?done=fail")
		}

		return tool.Get_redirect("/baro/card")
	}

	page_int := baro_founding_page_num(page)
	list := baro_founding_card_rows(db, config.IP, (page_int-1)*tool.Baro_page_size, tool.Baro_page_size)

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/founding", tool.Baro_t("창립 시즌")},
		{"", tool.Baro_t("공유 카드")},
	})
	body += tool.Baro_hero(tool.Baro_t("내 공유 카드"), tool.Baro_t("기록을 한 장으로 만들어 남기고 나눕니다."), "")
	body += baro_founding_tabs("/baro/card")

	if query != nil {
		switch query.Get("done") {
		case "deleted":
			body += tool.Baro_alert("ok", tool.Baro_t("카드를 지웠습니다."))
		case "fail":
			body += tool.Baro_alert("danger", tool.Baro_t("내가 만든 카드만 지울 수 있습니다."))
		}
	}

	body += tool.Baro_card(
		tool.Baro_t("새 카드"),
		tool.Baro_button("/baro/card/make", tool.Baro_t("공유 카드 만들기"), "primary")+" "+
			tool.Baro_button("/baro/card/make?kind=founder", tool.Baro_t("창립 번호 카드"), "")+" "+
			tool.Baro_button("/baro/card/make?kind=field", tool.Baro_t("분야 개척률 카드"), ""),
	)

	rows := [][]string{}
	for _, card := range list {
		delete_form := tool.Baro_hidden("action", "delete")
		delete_form += tool.Baro_hidden("id", card.ID)
		delete_form += tool.Baro_submit(tool.Baro_t("지우기"), "danger")

		rows = append(rows, []string{
			tool.Baro_badge(baro_founding_card_kind_name(card.Kind), "info"),
			`<a href="/baro/card/` + tool.Url_parser(card.ID) + `">` + tool.HTML_escape(tool.Baro_trunc(card.Title, 60)) + `</a>`,
			tool.Baro_time_cell(card.Date),
			tool.Baro_comma(tool.Str_to_int(card.Views)),
			tool.Baro_form("/baro/card", delete_form),
		})
	}

	if len(rows) == 0 {
		body += tool.Baro_card(tool.Baro_t("내 카드"), tool.Baro_empty(tool.Baro_t("아직 만든 카드가 없습니다. 기여를 남기고 첫 카드를 만들어 보세요.")))
	} else {
		body += tool.Baro_card(
			tool.Baro_t("내 카드"),
			tool.Baro_table([]string{"종류", tool.Baro_t("문구"), tool.Baro_t("만든 날"), tool.Baro_t("조회"), "관리"}, rows, tool.Baro_t("아직 만든 카드가 없습니다.")),
		)
	}

	body += tool.Baro_paging(page_int, len(rows), tool.Baro_page_size, "/baro/card?page={}")

	return tool.Baro_page(db, config, tool.Baro_t("내 공유 카드"), body, baro_founding_menu())
}

// baro_founding_card_svg : 카드 배경 장식
func baro_founding_card_svg() string {
	return `<svg class="bwc_svg" viewBox="0 0 600 340" preserveAspectRatio="none" aria-hidden="true">` +
		`<circle cx="520" cy="60" r="120" fill="rgba(255,255,255,.10)"></circle>` +
		`<circle cx="70" cy="300" r="150" fill="rgba(255,255,255,.07)"></circle>` +
		`<path d="M0 300 L120 210 L240 265 L360 150 L480 205 L600 110 L600 340 L0 340 Z" fill="rgba(255,255,255,.06)"></path>` +
		`</svg>`
}

// View_baro_founding_card_view : 공유 카드 보기 (1-6)
//
//	카카오톡·디스코드·SNS 미리보기를 위해 Open Graph 메타태그가 필요하므로
//	이 화면만 스킨을 거치지 않고 독립된 문서를 그린다.
//	base 는 요청이 들어온 주소(https://example.com)로, og:url 을 절대 주소로 적기 위해 받는다.
func View_baro_founding_card_view(config tool.Config, id string, base string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	card, found := baro_founding_card_get(db, strings.TrimSpace(id))
	if !found {
		body := tool.Baro_crumb([][]string{
			{"/baro", "바로위키"},
			{"/baro/card", tool.Baro_t("공유 카드")},
			{"", tool.Baro_t("카드 없음")},
		})
		body += tool.Baro_hero(tool.Baro_t("카드를 찾을 수 없습니다"), tool.Baro_t("주소가 잘못되었거나 만든 사람이 카드를 지웠습니다."), "")
		body += tool.Baro_alert("warn", tool.Baro_t("카드가 사라져도 기여 기록은 그대로 남아 있습니다."))
		body += tool.Baro_card(
			tool.Baro_t("다른 곳으로 가기"),
			tool.Baro_button("/baro/card/make", tool.Baro_t("내 카드 만들기"), "primary")+" "+
				tool.Baro_button("/baro/founding/hall", tool.Baro_t("창립 시즌 기록관"), "")+" "+
				tool.Baro_button("/baro", tool.Baro_t("바로위키 홈"), ""),
		)

		return tool.Baro_page(db, config, tool.Baro_t("카드를 찾을 수 없습니다"), body, baro_founding_menu())
	}

	// 조회수 (17장 요청 제한과 별개로 단순 누적)
	tool.Exec_DB(db, "update baro_share_card set views = ? where id = ?", tool.Baro_itoa(tool.Str_to_int(card.Views)+1), card.ID)

	data := tool.Baro_json_dec_str(card.Data)
	text := data["text"]
	if strings.TrimSpace(text) == "" {
		text = card.Title
	}

	sub := data["sub"]
	if strings.TrimSpace(sub) == "" {
		sub = tool.Baro_t("바로위키에 남긴 기록")
	}

	wiki_name := tool.Baro_get_def(db, "wiki_name", "바로위키")

	// og:url 은 절대 주소여야 미리보기가 뜬다. 설정값이 있으면 그것을, 없으면 요청 주소를 쓴다.
	site_url := strings.TrimSuffix(tool.Baro_get_def(db, "site_url", ""), "/")
	if site_url == "" {
		site_url = strings.TrimSuffix(strings.TrimSpace(base), "/")
	}

	page_path := "/baro/card/" + tool.Url_parser(card.ID)
	og_url := page_path
	if site_url != "" {
		og_url = site_url + page_path
	}

	kind_name := baro_founding_card_kind_name(card.Kind)
	og_description := kind_name + " · " + wiki_name + tool.Baro_t(" 창립 시즌 기록")

	badge_line := ""
	if number := tool.Baro_founder_number(db, card.User); number != "" {
		badge_line = tool.Baro_t(`<span class="bwc_tag">창립 기여자 #`) + tool.HTML_escape(number) + `</span>`
	}

	html := `<!doctype html>
<html lang="ko">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + tool.HTML_escape(tool.Baro_trunc(text, 60)) + ` - ` + tool.HTML_escape(wiki_name) + `</title>
<meta name="description" content="` + tool.HTML_escape(og_description) + `">
<meta property="og:type" content="article">
<meta property="og:site_name" content="` + tool.HTML_escape(wiki_name) + `">
<meta property="og:title" content="` + tool.HTML_escape(tool.Baro_trunc(text, 100)) + `">
<meta property="og:description" content="` + tool.HTML_escape(og_description) + `">
<meta property="og:url" content="` + tool.HTML_escape(og_url) + `">
<meta property="og:locale" content="ko_KR">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="` + tool.HTML_escape(tool.Baro_trunc(text, 100)) + `">
<meta name="twitter:description" content="` + tool.HTML_escape(og_description) + `">
<style>
*{box-sizing:border-box;}
body{margin:0;padding:1.2rem;background:#f4f6f8;color:#1f2328;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","Noto Sans KR",sans-serif;line-height:1.6;}
.bwc_wrap{max-width:660px;margin:0 auto;}
.bwc_card{position:relative;overflow:hidden;border-radius:18px;padding:2.4rem 2rem;color:#fff;background:linear-gradient(135deg,#010080 0%,#0014de 55%,#000642 100%);box-shadow:0 10px 30px rgba(0,0,0,.18);}
.bwc_svg{position:absolute;inset:0;width:100%;height:100%;}
.bwc_in{position:relative;}
.bwc_kind{display:inline-block;padding:.2rem .7rem;border-radius:999px;background:rgba(255,255,255,.22);font-size:.82rem;font-weight:700;letter-spacing:.02em;}
.bwc_text{margin:1rem 0 .6rem 0;font-size:1.75rem;font-weight:800;line-height:1.4;word-break:keep-all;}
.bwc_sub{margin:0;font-size:1rem;opacity:.92;}
.bwc_foot{margin-top:1.6rem;display:flex;flex-wrap:wrap;align-items:center;gap:.6rem;font-size:.9rem;opacity:.95;}
.bwc_tag{display:inline-block;padding:.15rem .6rem;border-radius:999px;background:rgba(255,255,255,.2);font-weight:700;}
.bwc_brand{margin-left:auto;font-weight:800;letter-spacing:.04em;}
.bwc_panel{margin-top:1.2rem;background:#fff;border:1px solid #e2e6ea;border-radius:14px;padding:1.1rem 1.2rem;}
.bwc_row{display:flex;flex-wrap:wrap;gap:.5rem;margin:.6rem 0;}
.bwc_btn{display:inline-block;padding:.5rem 1rem;border-radius:8px;border:1px solid #d9dde2;background:#fff;color:#1f2328;font-size:.92rem;font-weight:700;text-decoration:none;cursor:pointer;}
.bwc_btn:hover{background:#f0f2f4;}
.bwc_btn_main{background:linear-gradient(90deg,#010080,#0014de);border-color:transparent;color:#fff;}
.bwc_meta{color:#6b7280;font-size:.86rem;margin:.3rem 0 0 0;}
.bwc_link{width:100%;padding:.5rem .6rem;border:1px solid #d9dde2;border-radius:8px;font-size:.88rem;}
@media (max-width:520px){.bwc_text{font-size:1.35rem;}.bwc_card{padding:1.8rem 1.3rem;}}
</style>
</head>
<body>
<div class="bwc_wrap">
<div class="bwc_card">` + baro_founding_card_svg() + `<div class="bwc_in">
<span class="bwc_kind">` + tool.HTML_escape(kind_name) + `</span>
<p class="bwc_text">` + tool.HTML_escape(text) + `</p>
<p class="bwc_sub">` + tool.HTML_escape(sub) + `</p>
<div class="bwc_foot">
<span>` + tool.HTML_escape(card.User) + `</span>` + badge_line + `
<span>` + tool.HTML_escape(tool.Baro_date_only(card.Date)) + `</span>
<span class="bwc_brand">` + tool.HTML_escape(wiki_name) + `</span>
</div>
</div></div>

<div class="bwc_panel">
<p class="bwc_meta">조회 ` + tool.HTML_escape(tool.Baro_comma(tool.Str_to_int(card.Views)+1)) + `회 · 만든 날 ` + tool.HTML_escape(card.Date) + `</p>
<input class="bwc_link" id="bwc_url" readonly value="" data-share="` + tool.HTML_escape(tool.Baro_trunc(text, 100)) + `">
<div class="bwc_row">
<button type="button" class="bwc_btn bwc_btn_main" onclick="bwc_copy()">링크 복사</button>
<a class="bwc_btn" id="bwc_kakao" href="#" target="_blank" rel="noopener">카카오로 공유</a>
<a class="bwc_btn" id="bwc_twitter" href="#" target="_blank" rel="noopener">트위터(X)로 공유</a>
</div>
<p class="bwc_meta">카카오톡으로 보낼 때는 링크를 복사해 대화창에 붙여 넣으면 미리보기가 함께 보입니다.</p>
<div class="bwc_row">
<a class="bwc_btn" href="/baro/card/make">나도 카드 만들기</a>
<a class="bwc_btn" href="/baro/founding">창립 시즌 보기</a>
<a class="bwc_btn" href="/baro">` + tool.HTML_escape(wiki_name) + ` 홈</a>
</div>
</div>
</div>
<script>
(function () {
	var here = location.href;
	var box = document.getElementById('bwc_url');
	box.value = here;
	document.getElementById('bwc_kakao').href = 'https://story.kakao.com/share?url=' + encodeURIComponent(here);
	var share = box.getAttribute('data-share') || '';
	document.getElementById('bwc_twitter').href = 'https://twitter.com/intent/tweet?url=' + encodeURIComponent(here) + '&text=' + encodeURIComponent(share);
})();
function bwc_copy() {
	var box = document.getElementById('bwc_url');
	if (navigator.clipboard && navigator.clipboard.writeText) {
		navigator.clipboard.writeText(box.value).then(function () { alert('링크를 복사했습니다.'); });
		return;
	}
	box.select();
	document.execCommand('copy');
	alert('링크를 복사했습니다.');
}
</script>
</body>
</html>`

	return html
}
