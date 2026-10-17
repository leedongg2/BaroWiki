package route

import (
	"net/url"

	"opennamu/route/tool"
)

// ============================================================
//  밐 순위  /baro/mik/rank
//
//  세 가지로 나누어 본다.
//    now   : 지금 가진 밐     — 이 순간의 잔액
//    max   : 최대 보유 밐     — 지금까지 가졌던 가장 많은 밐 (쓴다고 줄지 않는다)
//    total : 누적 획득 밐     — 지금까지 들어온 밐을 모두 더한 값
//
//  하나로 줄을 세우지 않는 것은 기여 순위와 같은 까닭이다.
//  「지금 많이 가진 사람」 과 「많이 모았던 사람」 과 「많이 벌어 온 사람」 은 서로 다른 이야기다.
// ============================================================

// baro_mik_rank_kind_ok : 아는 갈래인가
func baro_mik_rank_kind_ok(kind string) bool {
	for _, row := range tool.Baro_mik_rank_kind_list() {
		if row[0] == kind {
			return true
		}
	}

	return false
}

// baro_mik_rank_note : 갈래 설명
func baro_mik_rank_note(kind string) string {
	for _, row := range tool.Baro_mik_rank_kind_list() {
		if row[0] == kind {
			return row[2]
		}
	}

	return ""
}

// baro_mik_rank_name : 갈래 이름
func baro_mik_rank_name(kind string) string {
	for _, row := range tool.Baro_mik_rank_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// View_baro_mik_rank : 밐 순위 (/baro/mik/rank)
func View_baro_mik_rank(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	kind := ""
	page_text := ""
	if query != nil {
		kind = query.Get("kind")
		page_text = query.Get("page")
	}

	if !baro_mik_rank_kind_ok(kind) {
		kind = "now"
	}

	page := tool.Str_to_int(page_text)
	if page < 1 {
		page = 1
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/mik", "밐"}, {"", tool.Baro_t("순위")}})
	body += tool.Baro_hero(
		tool.Baro_t("밐 순위"),
		tool.Baro_t("모은 밐을 세 가지 눈으로 봅니다."),
		"",
	)
	body += baro_mik_tabs("/baro/mik/rank")

	// --- 갈래 탭 ---
	kind_tabs := [][]string{}
	for _, row := range tool.Baro_mik_rank_kind_list() {
		kind_tabs = append(kind_tabs, []string{"/baro/mik/rank?kind=" + row[0], row[1]})
	}
	body += tool.Baro_section(tool.Baro_t("갈래"), tool.Baro_tabs(kind_tabs, "/baro/mik/rank?kind="+kind))

	body += tool.Baro_note(baro_mik_rank_note(kind))

	// --- 내 자리 ---
	if tool.Baro_is_user(config.IP) {
		body += tool.Baro_stat_row([][]string{
			{tool.Baro_t("지금 가진 밐"), tool.Baro_mik_text(tool.Baro_mik_get(db, config.IP)), ""},
			{tool.Baro_t("최대 보유 밐"), tool.Baro_mik_text(tool.Baro_mik_best(db, config.IP)), ""},
			{tool.Baro_t("누적 획득 밐"), tool.Baro_mik_text(tool.Baro_mik_total(db, config.IP)), ""},
		})
	}

	// --- 순위표 ---
	size := tool.Baro_page_size
	list := tool.Baro_mik_rank(db, kind, (page-1)*size, size)

	rows := [][]string{}
	for i, row := range list {
		if row.Value <= 0 {
			continue
		}

		name := tool.Baro_user_link(db, row.User, config.IP)
		if title := tool.Baro_mik_title_user_html(db, row.User); title != "" {
			name += " " + title
		}

		rows = append(rows, []string{
			tool.Baro_itoa((page-1)*size + i + 1),
			name,
			tool.HTML_escape(tool.Baro_mik_text(row.Value)),
		})
	}

	body += tool.Baro_card(baro_mik_rank_name(kind), tool.Baro_table(
		[]string{tool.Baro_t("자리"), tool.Baro_t("사람"), baro_mik_rank_name(kind)},
		rows,
		tool.Baro_t("아직 순위에 오른 사람이 없습니다."),
	))

	body += tool.Baro_paging(page, tool.Baro_mik_rank_count(db, kind), size,
		"/baro/mik/rank?kind="+tool.Url_parser(kind)+"&page={}")

	body += tool.Baro_card(tool.Baro_t("어떻게 세나"), tool.Baro_list([]string{
		tool.Baro_t_esc("지금 가진 밐 — 이 순간의 잔액입니다. 쓰면 줄고 받으면 늘어납니다."),
		tool.Baro_t_esc("최대 보유 밐 — 지금까지 한 번이라도 가져 본 가장 많은 밐입니다. 쓴다고 줄지 않습니다."),
		tool.Baro_t_esc("누적 획득 밐 — 접속 보상 · 도전 과제 · 지급 · 양도로 들어온 밐을 모두 더한 값입니다. 나간 것은 빼지 않습니다."),
		tool.Baro_t_esc("세 가지를 하나로 합친 통합 순위는 두지 않습니다."),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("밐 순위"), body, baro_mik_menu())
}
