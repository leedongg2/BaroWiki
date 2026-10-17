package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로샵  /baro/shop
//
//  밐으로 살 수 있는 것을 늘어놓는다.
//  지금 있는 종류는 둘이다.
//    title   : 이름 옆에 다는 칭호
//    captcha : 며칠 동안 캡차를 묻지 않는 이용권
//  상품 자체는 관리자가 등록 · 수정 · 내림 한다. (/baro/shop/admin)
// ============================================================

// baro_mik_shop_kind_list : 상품 종류 {열쇠, 이름}
func baro_mik_shop_kind_list() [][]string {
	return [][]string{
		{"title", tool.Baro_t("칭호")},
		{"captcha", tool.Baro_t("캡차 면제권 (기간)")},
		{"captcha_once", tool.Baro_t("캡차 1회 면제권")},
	}
}

// baro_mik_shop_kind_name : 종류 이름
func baro_mik_shop_kind_name(kind string) string {
	for _, row := range baro_mik_shop_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// baro_mik_shop_row : 상품 한 줄
type baro_mik_shop_row struct {
	ID          string
	Name        string
	Description string
	Kind        string
	Price       int
	Data        map[string]string
	Status      string
	Stock       string
	Sort        int
}

// baro_mik_shop_scan : 조회 결과 한 줄 읽기
func baro_mik_shop_scan(rows *sql.Rows) (baro_mik_shop_row, bool) {
	row := baro_mik_shop_row{}
	price, data, sort_text := "", "", ""

	if rows.Scan(&row.ID, &row.Name, &row.Description, &row.Kind, &price, &data, &row.Status, &row.Stock, &sort_text) != nil {
		return row, false
	}

	row.Price = tool.Str_to_int(price)
	row.Data = tool.Baro_json_dec_str(data)
	row.Sort = tool.Str_to_int(sort_text)

	return row, true
}

// baro_mik_shop_select : 공통 조회 칸
const baro_mik_shop_select = "select id, name, description, kind, price, data, status, stock, sort from baro_shop_item"

// baro_mik_shop_list : 상품 목록 (only_open 이면 팔고 있는 것만)
func baro_mik_shop_list(db *sql.DB, only_open bool) []baro_mik_shop_row {
	query := baro_mik_shop_select
	if only_open {
		query += " where status = 'open'"
	}
	query += " order by sort + 0, id + 0"

	rows := tool.Query_DB(db, query)
	defer rows.Close()

	out := []baro_mik_shop_row{}
	for rows.Next() {
		if row, ok := baro_mik_shop_scan(rows); ok {
			out = append(out, row)
		}
	}

	return out
}

// baro_mik_shop_get : 상품 하나
func baro_mik_shop_get(db *sql.DB, id string) (baro_mik_shop_row, bool) {
	rows := tool.Query_DB(db, baro_mik_shop_select+" where id = ?", id)
	defer rows.Close()

	if !rows.Next() {
		return baro_mik_shop_row{}, false
	}

	return baro_mik_shop_scan(rows)
}

// baro_mik_shop_sold : 이 상품이 팔린 횟수
func baro_mik_shop_sold(db *sql.DB, id string) int {
	count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_shop_buy where item = ?", []any{&count}, id)

	return count
}

// baro_mik_shop_left : 남은 재고 (-1 이면 무제한)
func baro_mik_shop_left(db *sql.DB, row baro_mik_shop_row) int {
	if strings.TrimSpace(row.Stock) == "" {
		return -1
	}

	left := tool.Str_to_int(row.Stock) - baro_mik_shop_sold(db, row.ID)
	if left < 0 {
		return 0
	}

	return left
}

// baro_mik_shop_detail : 상품의 부가 설명 (칭호 이름 · 며칠짜리인지)
func baro_mik_shop_detail(row baro_mik_shop_row) string {
	switch row.Kind {
	case "title":
		if name := row.Data["title"]; name != "" {
			return tool.Baro_t("칭호") + " : " + name
		}
	case "captcha":
		if days := row.Data["days"]; days != "" {
			return days + tool.Baro_t("일 동안 캡차를 묻지 않습니다")
		}
	case "captcha_once":
		if count := row.Data["count"]; count != "" {
			return tool.Baro_t("캡차를 ") + count + tool.Baro_t("번 건너뜁니다")
		}
	}

	return ""
}

// View_baro_mik_shop : 바로샵 (/baro/shop)
func View_baro_mik_shop(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	alert := ""

	// 사기
	if values != nil && values.Get("do") == "buy" {
		if wall, stop := tool.Baro_login_wall(db, config, "/baro/shop"); stop {
			return wall
		}

		text, kind := baro_mik_shop_buy(db, config, values.Get("item"))
		alert = tool.Baro_alert(kind, text)
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", "바로샵"}})
	body += tool.Baro_hero(
		"바로샵",
		tool.Baro_t("모아 둔 밐으로 칭호와 이용권을 삽니다."),
		"",
	)
	body += baro_mik_tabs("/baro/shop")
	body += alert

	if tool.Baro_is_user(config.IP) {
		body += tool.Baro_stat_row([][]string{
			{tool.Baro_t("가진 밐"), tool.Baro_mik_text(tool.Baro_mik_get(db, config.IP)), ""},
		})
	} else {
		body += tool.Baro_note(tool.Baro_t("로그인하면 밐을 모으고 상품을 살 수 있습니다."))
	}

	body += baro_mik_shop_board(db, config)

	if tool.Baro_is_user(config.IP) {
		body += baro_mik_shop_mine(db, config.IP)
	}

	if tool.Baro_can(db, config.IP, "system") {
		body += tool.Baro_card(
			tool.Baro_t("관리"),
			tool.Baro_button("/baro/shop/admin", tool.Baro_t("상품 관리하기"), "")+
				tool.Baro_button("/baro/mik/admin", tool.Baro_t("밐 지급하기"), "")+
				tool.Baro_button("/baro/mik/setting", tool.Baro_t("밐 · 캡차 설정"), ""),
		)
	}

	return tool.Baro_page(db, config, "바로샵", body, baro_mik_menu())
}

// baro_mik_shop_board : 팔고 있는 상품 진열
func baro_mik_shop_board(db *sql.DB, config tool.Config) string {
	list := baro_mik_shop_list(db, true)
	if len(list) == 0 {
		return tool.Baro_card("", tool.Baro_empty(tool.Baro_t("지금 팔고 있는 상품이 없습니다.")))
	}

	balance := tool.Baro_mik_get(db, config.IP)
	is_user := tool.Baro_is_user(config.IP)
	owned := map[string]bool{}
	for _, one := range tool.Baro_mik_title_list(db, config.IP) {
		owned[one] = true
	}

	cards := []string{}
	for _, row := range list {
		left := baro_mik_shop_left(db, row)

		info := tool.Baro_badge(baro_mik_shop_kind_name(row.Kind), "info") + " " +
			tool.Baro_badge(tool.Baro_mik_text(row.Price), "muted")
		if left >= 0 {
			info += " " + tool.Baro_badge(tool.Baro_t("남은 수량 ")+tool.Baro_itoa(left), "warn")
		}

		// Baro_card_ex 의 부제는 글자로만 나가므로, 표딱지는 본문 맨 위에 둔다.
		inner := `<p>` + info + `</p>`
		inner += `<p>` + tool.HTML_escape(row.Description) + `</p>`

		// 칭호는 실제로 달렸을 때의 색 그대로 보여 준다.
		if row.Kind == "title" {
			sample := tool.Baro_mik_title_html(tool.Baro_mik_title_row{
				Name:  row.Data["title"],
				Color: row.Data["color"],
				Dark:  row.Data["color_dark"],
			})

			if sample != "" {
				inner += `<p>` + tool.Baro_t_esc("이렇게 보입니다 : ") + sample + `</p>`
			}
		}
		if detail := baro_mik_shop_detail(row); detail != "" {
			inner += tool.Baro_note(detail)
		}

		// 살 수 있는지 따져 단추 대신 까닭을 보여 준다.
		switch {
		case !is_user:
			inner += tool.Baro_note(tool.Baro_t("로그인한 뒤에 살 수 있습니다."))
		case row.Kind == "title" && owned[row.Data["title"]]:
			inner += tool.Baro_badge(tool.Baro_t("이미 가진 칭호"), "ok")
		case left == 0:
			inner += tool.Baro_badge(tool.Baro_t("다 팔렸습니다"), "danger")
		case balance < row.Price:
			inner += tool.Baro_note(tool.Baro_t("밐이 ") + tool.Baro_mik_text(row.Price-balance) + tool.Baro_t(" 모자랍니다."))
		default:
			inner += tool.Baro_form("/baro/shop",
				tool.Baro_hidden("do", "buy")+
					tool.Baro_hidden("item", row.ID)+
					tool.Baro_submit(tool.Baro_t("사기"), "primary"))
		}

		cards = append(cards, tool.Baro_card_ex(row.Name, "", inner, ""))
	}

	return tool.Baro_grid(cards, 2)
}

// baro_mik_shop_mine : 내가 산 것
func baro_mik_shop_mine(db *sql.DB, user string) string {
	rows := tool.Query_DB(
		db,
		"select name, kind, price, status, date, end_date from baro_shop_buy where user = ? order by id + 0 desc limit 30",
		user,
	)
	defer rows.Close()

	out := [][]string{}
	for rows.Next() {
		name, kind, price, status, date, end_date := "", "", "", "", "", ""
		if rows.Scan(&name, &kind, &price, &status, &date, &end_date) != nil {
			continue
		}

		state := tool.Baro_badge(tool.Baro_t("가지고 있음"), "ok")
		switch {
		case status != "active":
			state = tool.Baro_badge(tool.Baro_t("다 씀"), "muted")
		case end_date != "" && tool.Baro_is_past(end_date):
			state = tool.Baro_badge(tool.Baro_t("기간 끝남"), "muted")
		case end_date != "":
			state = tool.Baro_badge(tool.Baro_duration_left(end_date), "ok")
		}

		out = append(out, []string{
			tool.Baro_time_cell(date),
			tool.HTML_escape(name),
			tool.Baro_t_esc(baro_mik_shop_kind_name(kind)),
			tool.HTML_escape(tool.Baro_mik_text(tool.Str_to_int(price))),
			state,
		})
	}

	return tool.Baro_card(tool.Baro_t("내가 산 것"), tool.Baro_table(
		[]string{tool.Baro_t("산 때"), tool.Baro_t("상품"), tool.Baro_t("종류"), tool.Baro_t("값"), tool.Baro_t("상태")},
		out,
		tool.Baro_t("아직 산 것이 없습니다."),
	))
}

// baro_mik_shop_buy : 사기 처리. (안내 문구, 안내 종류)
func baro_mik_shop_buy(db *sql.DB, config tool.Config, item_id string) (string, string) {
	me := config.IP

	if tool.Baro_read_only(db) {
		return tool.Baro_t("지금은 읽기 전용입니다. 잠시 뒤에 다시 시도해 주세요."), "warn"
	}
	if !tool.Baro_rate_ok(db, "shop_buy", me, 20, 3600) {
		return tool.Baro_t("잠시 뒤에 다시 시도해 주세요."), "warn"
	}

	row, found := baro_mik_shop_get(db, strings.TrimSpace(item_id))
	if !found || row.Status != "open" {
		return tool.Baro_t("지금 살 수 없는 상품입니다."), "danger"
	}
	if row.Price < 0 {
		return tool.Baro_t("값이 잘못된 상품입니다. 운영진에게 알려 주세요."), "danger"
	}
	if left := baro_mik_shop_left(db, row); left == 0 {
		return tool.Baro_t("다 팔렸습니다."), "warn"
	}

	// 종류별로 살 수 있는지 먼저 따진다. (밐을 뺀 뒤에 막히면 안 된다)
	title_name := strings.TrimSpace(row.Data["title"])
	days := tool.Str_to_int(row.Data["days"])
	count := tool.Str_to_int(row.Data["count"])

	switch row.Kind {
	case "title":
		if title_name == "" {
			return tool.Baro_t("칭호 이름이 비어 있는 상품입니다. 운영진에게 알려 주세요."), "danger"
		}
		if baro_mik_has_title(db, me, title_name) {
			return tool.Baro_t("이미 가지고 있는 칭호입니다."), "warn"
		}
	case "captcha":
		if days <= 0 {
			return tool.Baro_t("기간이 비어 있는 상품입니다. 운영진에게 알려 주세요."), "danger"
		}
	case "captcha_once":
		if count <= 0 {
			return tool.Baro_t("횟수가 비어 있는 상품입니다. 운영진에게 알려 주세요."), "danger"
		}
	default:
		return tool.Baro_t("아직 다룰 수 없는 종류의 상품입니다."), "danger"
	}

	if tool.Baro_mik_get(db, me) < row.Price {
		return tool.Baro_t("밐이 모자랍니다."), "warn"
	}

	if row.Price > 0 && !tool.Baro_mik_add(db, me, -row.Price, "buy", "바로샵 : "+row.Name, me, row.ID) {
		return tool.Baro_t("사지 못했습니다. 잔액을 다시 확인해 주세요."), "danger"
	}

	buy_id := tool.Baro_next_id(db, "baro_shop_buy")
	end_date := ""
	status := "active"
	done := ""

	wear_title := false

	switch row.Kind {
	case "title":
		done = tool.Baro_t("칭호를 얻었습니다 : ") + title_name

		// 달고 있는 칭호가 없으면 방금 산 것을 바로 달아 준다.
		wear_title = tool.Baro_mik_title(db, me) == ""
	case "captcha":
		end_date = tool.Baro_mik_captcha_extend(db, me, days)
		status = "done"
		done = tool.Baro_t("캡차 면제가 ") + tool.Baro_date_only(end_date) + tool.Baro_t(" 까지로 늘었습니다.")
	case "captcha_once":
		left := tool.Baro_captcha_once_add(db, me, count)
		status = "done"
		done = tool.Baro_t("캡차 1회 면제권을 얻었습니다. 남은 장수 ") + tool.Baro_itoa(left) + tool.Baro_t("장")
	}

	tool.Exec_DB(
		db,
		"insert into baro_shop_buy (id, item, user, name, kind, price, data, status, date, end_date) "+
			"values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		buy_id, row.ID, me, row.Name, row.Kind, tool.Baro_itoa(row.Price),
		tool.Baro_json_enc(row.Data), status, tool.Baro_now(), end_date,
	)

	// 산 기록이 들어간 뒤라야 "가진 칭호" 로 인정된다.
	if wear_title {
		tool.Baro_mik_title_set(db, me, title_name)
	}

	tool.Baro_audit(db, config, "baro_shop_buy", row.ID, "", row.Name, "바로샵 구매")
	tool.Baro_metric_bump(db, "baro_shop_buy", tool.Baro_today(), 1)

	return done, "ok"
}
