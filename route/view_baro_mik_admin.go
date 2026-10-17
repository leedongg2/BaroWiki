package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로샵 · 밐 관리  /baro/shop/admin · /baro/mik/admin
//
//  관리자가 상품을 등록 · 수정 · 내림 · 지움 하고, 사유를 적어 밐을 직접 준다.
//  준 밐과 회수한 밐은 모두 공개 운영 로그와 감사 기록에 남는다. (5번 원칙)
// ============================================================

// baro_mik_admin_gate : 관리 권한 확인
func baro_mik_admin_gate(db *sql.DB, config tool.Config) (string, bool) {
	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("바로샵과 밐을 관리할 수 있는 권한이 필요합니다.")), true
	}

	return "", false
}

// baro_mik_admin_form : 상품 등록·수정 폼
//
//	row 가 비어 있으면 새로 등록하는 폼이 된다.
func baro_mik_admin_form(action string, row baro_mik_shop_row, submit string) string {
	form := tool.Baro_input("name", tool.Baro_t("상품 이름"), row.Name, tool.Baro_t("예: 칭호 · 문서 사냥꾼"), "")
	form += tool.Baro_textarea("description", tool.Baro_t("설명"), row.Description, 3,
		tool.Baro_t("사는 사람이 무엇을 얻는지 한두 줄로"), "")
	form += tool.Baro_select("kind", tool.Baro_t("종류"), baro_mik_shop_kind_list(), row.Kind, "")
	form += tool.Baro_input_type("price", tool.Baro_t("값 (밐)"), tool.Baro_itoa(row.Price), "number", "")
	form += tool.Baro_input("title_name", tool.Baro_t("칭호 이름"), row.Data["title"], tool.Baro_t("예: 문서 사냥꾼"),
		tool.Baro_t("종류가 「칭호」 일 때만 씁니다. 이름 옆에 이대로 붙습니다."))
	form += tool.Baro_input("title_color", tool.Baro_t("칭호 글자색 (밝은 쪽)"), row.Data["color"], "#0047ab",
		tool.Baro_t("비우면 본문 글자색을 그대로 씁니다. #rrggbb 또는 red 처럼 적습니다."))
	form += tool.Baro_input("title_color_dark", tool.Baro_t("칭호 글자색 (어두운 쪽)"), row.Data["color_dark"], "#ec9f19",
		tool.Baro_t("다크 모드에서 쓸 색입니다. 비우면 밝은 쪽 색을 그대로 씁니다."))
	form += tool.Baro_input_type("days", tool.Baro_t("면제 일수"), row.Data["days"], "number",
		tool.Baro_t("종류가 「캡차 면제권 (기간)」 일 때만 씁니다."))
	form += tool.Baro_input_type("count", tool.Baro_t("면제 횟수"), row.Data["count"], "number",
		tool.Baro_t("종류가 「캡차 1회 면제권」 일 때만 씁니다. 한 장에 몇 번을 건너뛰는지."))
	form += tool.Baro_input("stock", tool.Baro_t("수량 (선택)"), row.Stock, tool.Baro_t("비우면 무제한"),
		tool.Baro_t("한정 상품으로 만들 때만 적습니다."))
	form += tool.Baro_input_type("sort", tool.Baro_t("차례"), tool.Baro_itoa(row.Sort), "number",
		tool.Baro_t("작은 값이 앞에 놓입니다."))
	form += tool.Baro_checkbox("open", tool.Baro_t("지금 팔기"), "on", row.Status != "closed", "")
	form += tool.Baro_input("reason", tool.Baro_t("사유 (선택)"), "", "", tool.Baro_t("공개 운영 로그에 함께 남습니다."))
	form += tool.Baro_submit(submit, "primary")

	return tool.Baro_form(action, form)
}

// baro_mik_admin_read : 폼에서 상품 값 읽기
func baro_mik_admin_read(values url.Values) (baro_mik_shop_row, string) {
	row := baro_mik_shop_row{
		Name:        strings.TrimSpace(values.Get("name")),
		Description: strings.TrimSpace(values.Get("description")),
		Kind:        strings.TrimSpace(values.Get("kind")),
		Price:       tool.Str_to_int(values.Get("price")),
		Stock:       strings.TrimSpace(values.Get("stock")),
		Sort:        tool.Str_to_int(values.Get("sort")),
		Data:        map[string]string{},
		Status:      "closed",
	}

	if values.Get("open") != "" {
		row.Status = "open"
	}

	switch row.Kind {
	case "title":
		row.Data["title"] = strings.TrimSpace(values.Get("title_name"))

		// 색은 담기 전에 다듬는다. 쓸 수 없는 값은 담지 않는다.
		if color := tool.Baro_theme_color_norm(values.Get("title_color")); color != "" {
			row.Data["color"] = color
		}
		if color := tool.Baro_theme_color_norm(values.Get("title_color_dark")); color != "" {
			row.Data["color_dark"] = color
		}
	case "captcha":
		row.Data["days"] = strings.TrimSpace(values.Get("days"))
	case "captcha_once":
		row.Data["count"] = strings.TrimSpace(values.Get("count"))
	}

	switch {
	case row.Name == "":
		return row, tool.Baro_t("상품 이름을 적어 주세요.")
	case baro_mik_shop_kind_name(row.Kind) == row.Kind:
		return row, tool.Baro_t("상품 종류를 골라 주세요.")
	case row.Price < 0:
		return row, tool.Baro_t("값은 0 이상이어야 합니다.")
	case row.Kind == "title" && row.Data["title"] == "":
		return row, tool.Baro_t("칭호 이름을 적어 주세요.")
	case row.Kind == "title" && strings.TrimSpace(values.Get("title_color")) != "" && row.Data["color"] == "":
		return row, tool.Baro_t("칭호 글자색을 알아볼 수 없습니다. #rrggbb 또는 red 처럼 적어 주세요.")
	case row.Kind == "title" && strings.TrimSpace(values.Get("title_color_dark")) != "" && row.Data["color_dark"] == "":
		return row, tool.Baro_t("어두운 쪽 칭호 글자색을 알아볼 수 없습니다.")
	case row.Kind == "captcha" && tool.Str_to_int(row.Data["days"]) <= 0:
		return row, tool.Baro_t("면제 일수를 1 이상으로 적어 주세요.")
	case row.Kind == "captcha_once" && tool.Str_to_int(row.Data["count"]) <= 0:
		return row, tool.Baro_t("면제 횟수를 1 이상으로 적어 주세요.")
	case row.Stock != "" && tool.Str_to_int(row.Stock) < 0:
		return row, tool.Baro_t("수량은 0 이상이어야 합니다.")
	}

	return row, ""
}

// View_baro_mik_shop_admin : 상품 관리 (/baro/shop/admin)
func View_baro_mik_shop_admin(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, stop := baro_mik_admin_gate(db, config); stop {
		return deny
	}

	alert := ""

	if values != nil {
		switch values.Get("do") {
		case "add":
			row, problem := baro_mik_admin_read(values)
			if problem != "" {
				alert = tool.Baro_alert("warn", problem)
				break
			}

			id := tool.Baro_next_id(db, "baro_shop_item")
			tool.Exec_DB(
				db,
				"insert into baro_shop_item (id, name, description, kind, price, data, status, stock, sort, actor, date) "+
					"values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
				id, row.Name, row.Description, row.Kind, tool.Baro_itoa(row.Price),
				tool.Baro_json_enc(row.Data), row.Status, row.Stock, tool.Baro_itoa(row.Sort),
				config.IP, tool.Baro_now(),
			)

			tool.Baro_public_log(db, "shop_add", config.IP, "shop:"+id, values.Get("reason"), row.Name)
			tool.Baro_audit(db, config, "baro_shop_add", "shop:"+id, "", row.Name, values.Get("reason"))

			alert = tool.Baro_alert("ok", tool.Baro_t("상품을 등록했습니다."))

		case "close", "open":
			id := strings.TrimSpace(values.Get("item"))
			if row, found := baro_mik_shop_get(db, id); found {
				status := "closed"
				if values.Get("do") == "open" {
					status = "open"
				}

				tool.Exec_DB(db, "update baro_shop_item set status = ? where id = ?", status, id)
				tool.Baro_public_log(db, "shop_"+values.Get("do"), config.IP, "shop:"+id, values.Get("reason"), row.Name)
				tool.Baro_audit(db, config, "baro_shop_status", "shop:"+id, row.Status, status, values.Get("reason"))

				alert = tool.Baro_alert("ok", tool.Baro_t("상품 상태를 바꿨습니다."))
			}

		case "delete":
			alert = baro_mik_admin_delete(db, config, values)
		}
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/shop", "바로샵"}, {"", tool.Baro_t("상품 관리")}})
	body += tool.Baro_hero(
		tool.Baro_t("바로샵 상품 관리"),
		tool.Baro_t("상품을 등록하고, 내용을 고치고, 진열에서 내립니다."),
		"",
	)
	body += alert
	body += baro_mik_admin_table(db)
	body += tool.Baro_card(tool.Baro_t("새 상품 등록"), baro_mik_admin_form("/baro/shop/admin", baro_mik_shop_row{
		Data: map[string]string{}, Status: "open", Sort: 100,
	}, tool.Baro_t("등록하기")))

	return tool.Baro_page(db, config, tool.Baro_t("바로샵 상품 관리"), body, baro_mik_menu())
}

// baro_mik_admin_delete : 상품 완전 삭제
//
//	2번 원칙(삭제보다 보존)에 따라, 한 번이라도 팔린 상품은 지우지 않고 내리기만 한다.
//	이미 산 사람의 기록이 가리킬 곳을 없애지 않기 위해서다.
func baro_mik_admin_delete(db *sql.DB, config tool.Config, values url.Values) string {
	id := strings.TrimSpace(values.Get("item"))

	row, found := baro_mik_shop_get(db, id)
	if !found {
		return tool.Baro_alert("warn", tool.Baro_t("이미 없는 상품입니다."))
	}

	if sold := baro_mik_shop_sold(db, id); sold > 0 {
		return tool.Baro_alert("warn",
			tool.Baro_t("이미 ")+tool.Baro_itoa(sold)+tool.Baro_t("번 팔린 상품이라 지울 수 없습니다. 「내리기」 를 쓰면 진열에서만 빠지고 산 사람의 기록은 그대로 남습니다."))
	}

	reason := strings.TrimSpace(values.Get("reason"))
	if reason == "" {
		return tool.Baro_alert("warn", tool.Baro_t("지우는 까닭을 적어 주세요."))
	}

	tool.Exec_DB(db, "delete from baro_shop_item where id = ?", id)
	tool.Baro_public_log(db, "shop_delete", config.IP, "shop:"+id, reason, row.Name)
	tool.Baro_audit(db, config, "baro_shop_delete", "shop:"+id, row.Name, "", reason)

	return tool.Baro_alert("ok", tool.Baro_t("상품을 지웠습니다."))
}

// baro_mik_admin_table : 상품 목록 표 (관리용)
func baro_mik_admin_table(db *sql.DB) string {
	list := baro_mik_shop_list(db, false)

	out := [][]string{}
	for _, row := range list {
		state := tool.Baro_badge(tool.Baro_t("팔고 있음"), "ok")
		flip := "close"
		flip_text := tool.Baro_t("내리기")
		if row.Status == "closed" {
			state = tool.Baro_badge(tool.Baro_t("내림"), "muted")
			flip = "open"
			flip_text = tool.Baro_t("다시 팔기")
		}

		stock := tool.Baro_t_esc("무제한")
		if row.Stock != "" {
			stock = tool.HTML_escape(row.Stock)
		}

		sold := baro_mik_shop_sold(db, row.ID)

		menu := `<a class="bw_btn" href="/baro/shop/admin/edit/` + tool.Url_parser(row.ID) + `">` +
			tool.Baro_t_esc("고치기") + `</a> `
		menu += tool.Baro_form("/baro/shop/admin",
			tool.Baro_hidden("do", flip)+tool.Baro_hidden("item", row.ID)+tool.Baro_submit(flip_text, ""))

		if sold == 0 {
			menu += tool.Baro_form("/baro/shop/admin",
				tool.Baro_hidden("do", "delete")+tool.Baro_hidden("item", row.ID)+
					`<input class="bw_in" type="text" name="reason" placeholder="`+
					tool.Baro_t_esc("지우는 까닭")+`">`+
					tool.Baro_submit(tool.Baro_t("지우기"), "danger"))
		}

		out = append(out, []string{
			tool.HTML_escape(row.ID),
			tool.HTML_escape(row.Name),
			tool.Baro_t_esc(baro_mik_shop_kind_name(row.Kind)),
			tool.HTML_escape(tool.Baro_mik_text(row.Price)),
			stock,
			tool.Baro_itoa(sold),
			state,
			menu,
		})
	}

	return tool.Baro_card(tool.Baro_t("등록된 상품"), tool.Baro_table(
		[]string{
			tool.Baro_t("번호"), tool.Baro_t("이름"), tool.Baro_t("종류"), tool.Baro_t("값"),
			tool.Baro_t("수량"), tool.Baro_t("팔린 수"), tool.Baro_t("상태"), tool.Baro_t("관리"),
		},
		out,
		tool.Baro_t("아직 등록된 상품이 없습니다."),
	))
}

// View_baro_mik_shop_edit : 상품 고치기 (/baro/shop/admin/edit/<번호>)
func View_baro_mik_shop_edit(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, stop := baro_mik_admin_gate(db, config); stop {
		return deny
	}

	id = strings.TrimSpace(id)

	row, found := baro_mik_shop_get(db, id)
	if !found {
		return tool.Baro_page(db, config, tool.Baro_t("상품 고치기"),
			tool.Baro_alert("danger", tool.Baro_t("그런 상품이 없습니다."))+
				tool.Baro_button("/baro/shop/admin", tool.Baro_t("상품 관리로"), ""),
			baro_mik_menu())
	}

	alert := ""

	if values != nil && values.Get("do") == "edit" {
		next, problem := baro_mik_admin_read(values)
		if problem != "" {
			alert = tool.Baro_alert("warn", problem)
			next.ID = row.ID
			row = next
		} else {
			tool.Exec_DB(
				db,
				"update baro_shop_item set name = ?, description = ?, kind = ?, price = ?, data = ?, "+
					"status = ?, stock = ?, sort = ? where id = ?",
				next.Name, next.Description, next.Kind, tool.Baro_itoa(next.Price),
				tool.Baro_json_enc(next.Data), next.Status, next.Stock, tool.Baro_itoa(next.Sort), id,
			)

			tool.Baro_public_log(db, "shop_edit", config.IP, "shop:"+id, values.Get("reason"), next.Name)
			tool.Baro_audit(db, config, "baro_shop_edit", "shop:"+id, row.Name, next.Name, values.Get("reason"))

			next.ID = id
			row = next
			alert = tool.Baro_alert("ok", tool.Baro_t("상품을 고쳤습니다."))
		}
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"}, {"/baro/shop", "바로샵"},
		{"/baro/shop/admin", tool.Baro_t("상품 관리")}, {"", row.Name},
	})
	body += tool.Baro_hero(tool.Baro_t("상품 고치기"), row.Name, "")
	body += alert
	body += tool.Baro_card("", baro_mik_admin_form(
		"/baro/shop/admin/edit/"+tool.Url_parser(id),
		row,
		tool.Baro_t("고치기"),
	))
	body += tool.Baro_note(tool.Baro_t("이미 산 사람이 가진 것은 여기서 고쳐도 바뀌지 않습니다. 산 시점의 내용이 그대로 남습니다."))
	body += tool.Baro_button("/baro/shop/admin", tool.Baro_t("상품 관리로 돌아가기"), "")

	return tool.Baro_page(db, config, tool.Baro_t("상품 고치기"), body, baro_mik_menu())
}

// ------------------------------------------------------------
//  밐 지급 · 회수
// ------------------------------------------------------------

// View_baro_mik_admin : 관리자 밐 지급 (/baro/mik/admin)
func View_baro_mik_admin(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, stop := baro_mik_admin_gate(db, config); stop {
		return deny
	}

	alert := ""
	if values != nil && values.Get("do") == "give" {
		text, kind := baro_mik_admin_give(db, config, values)
		alert = tool.Baro_alert(kind, text)
	}

	form := tool.Baro_hidden("do", "give")
	form += tool.Baro_input("user", tool.Baro_t("받을 사람"), "", tool.Baro_t("계정 이름 또는 고유 번호"),
		tool.Baro_t("고유 번호를 적으면 계정 이름이 바뀌어도 정확히 그 사람에게 갑니다."))
	form += tool.Baro_input_type("amount", tool.Baro_t("수량"), "", "number",
		tool.Baro_t("음수를 적으면 그만큼 회수합니다."))
	form += tool.Baro_textarea("reason", tool.Baro_t("사유"), "", 3,
		tool.Baro_t("예: 위키 수상식 부문 수상 보상"),
		tool.Baro_t("반드시 적어야 합니다. 공개 운영 로그에 그대로 남습니다."))
	form += tool.Baro_submit(tool.Baro_t("지급하기"), "primary")

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/mik", "밐"}, {"", tool.Baro_t("지급")}})
	body += tool.Baro_hero(
		tool.Baro_t("밐 지급"),
		tool.Baro_t("사유를 적어 밐을 주거나 거둡니다."),
		"",
	)
	body += alert
	body += tool.Baro_card("", tool.Baro_form("/baro/mik/admin", form))
	body += tool.Baro_card(tool.Baro_t("함께 보기"),
		tool.Baro_button("/baro/mik/setting", tool.Baro_t("밐 · 캡차 설정"), "")+" "+
			tool.Baro_button("/baro/shop/admin", tool.Baro_t("바로샵 상품 관리"), ""))
	body += baro_mik_admin_recent(db)

	return tool.Baro_page(db, config, tool.Baro_t("밐 지급"), body, baro_mik_menu())
}

// baro_mik_admin_give : 지급 처리. (안내 문구, 안내 종류)
func baro_mik_admin_give(db *sql.DB, config tool.Config, values url.Values) (string, string) {
	who := strings.TrimSpace(values.Get("user"))
	reason := strings.TrimSpace(values.Get("reason"))
	amount := tool.Str_to_int(values.Get("amount"))

	if who == "" {
		return tool.Baro_t("받을 사람을 적어 주세요."), "warn"
	}
	if reason == "" {
		return tool.Baro_t("사유를 적어 주세요. 사유 없는 지급은 할 수 없습니다."), "warn"
	}
	if amount == 0 {
		return tool.Baro_t("수량을 0 이 아닌 값으로 적어 주세요."), "warn"
	}

	// 고유 번호로 적었으면 번호를 먼저 풀어 본다.
	target := who
	if tool.Baro_uuid_is(who) {
		target = tool.Baro_user_from_uuid(db, who)
		if target == "" {
			return tool.Baro_t("해당 UUID를 가진 사용자는 없습니다."), "danger"
		}
	} else if !baro_mik_user_exist(db, target) {
		return tool.Baro_t("그런 사용자가 없습니다."), "danger"
	}

	kind := "admin"
	if amount < 0 {
		kind = "admin_take"
	}

	if !tool.Baro_mik_add(db, target, amount, kind, reason, config.IP, "") {
		if amount < 0 {
			return tool.Baro_t("가진 밐보다 많이 거둘 수 없습니다."), "warn"
		}

		return tool.Baro_t("지급하지 못했습니다."), "danger"
	}

	tool.Baro_public_log(db, "mik_"+kind, config.IP, "user:"+target, reason, tool.Baro_itoa(amount)+tool.Baro_mik_unit)
	tool.Baro_audit(db, config, "baro_mik_"+kind, "user:"+target, "", tool.Baro_itoa(amount), reason)

	notify := tool.Baro_t("운영진이 밐을 주었습니다 : ") + tool.Baro_mik_text(amount)
	if amount < 0 {
		notify = tool.Baro_t("운영진이 밐을 거두었습니다 : ") + tool.Baro_mik_text(-amount)
	}
	tool.Baro_notify(db, target, "system", notify, "/baro/mik", reason)

	return tool.Baro_t("처리했습니다. ") + target + " : " + tool.Baro_mik_text(amount), "ok"
}

// baro_mik_user_exist : 그런 계정이 있는가
func baro_mik_user_exist(db *sql.DB, user string) bool {
	if !tool.Baro_is_user(user) {
		return false
	}

	found := ""

	return tool.QueryRow_DB(db, "select id from user_set where id = ? limit 1", []any{&found}, user)
}

// baro_mik_admin_recent : 최근 지급·회수 기록
func baro_mik_admin_recent(db *sql.DB) string {
	rows := tool.Query_DB(
		db,
		"select user, delta, kind, reason, actor, date from baro_mik_log "+
			"where kind in ('admin', 'admin_take') order by id + 0 desc limit 30",
	)
	defer rows.Close()

	out := [][]string{}
	for rows.Next() {
		user, delta, kind, reason, actor, date := "", "", "", "", "", ""
		if rows.Scan(&user, &delta, &kind, &reason, &actor, &date) != nil {
			continue
		}

		amount := tool.Str_to_int(delta)
		amount_kind := "ok"
		amount_text := "+" + tool.Baro_comma(amount)
		if amount < 0 {
			amount_kind = "danger"
			amount_text = tool.Baro_comma(amount)
		}

		out = append(out, []string{
			tool.Baro_time_cell(date),
			tool.HTML_escape(user),
			tool.Baro_badge(amount_text, amount_kind),
			tool.Baro_t_esc(tool.Baro_mik_kind_name(kind)),
			tool.HTML_escape(tool.Baro_trunc(reason, 100)),
			tool.HTML_escape(actor),
		})
	}

	return tool.Baro_card(tool.Baro_t("최근 지급·회수"), tool.Baro_table(
		[]string{tool.Baro_t("때"), tool.Baro_t("받은 사람"), tool.Baro_t("증감"), tool.Baro_t("종류"), tool.Baro_t("사유"), tool.Baro_t("한 사람")},
		out,
		tool.Baro_t("아직 지급 기록이 없습니다."),
	))
}
