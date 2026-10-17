package route

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  기여 땅따먹기 — /baro/land
//
//  「누가 얼마나 기여했는가」 를 숫자표 대신 땅으로 보여 준다.
//
//    - 문서 한 개 = 땅 한 칸
//    - 그 문서에 글자를 가장 많이 넣은 사람이 그 칸의 주인
//    - 주인마다 색이 다르고, 색이 많이 보이는 사람이 땅을 많이 가진 사람
//
//  화면 세 가지
//    /baro/land                  … 위키 전체 지도
//    /baro/land/user/<이름>      … 그 사람의 땅만 밝게
//    /baro/land/doc/<문서>       … 문서 한 개 안에서의 기여 비율 띠
//
//  「가장 많이 넣은 사람」은 history 의 글자 수 증감(leng)을 더해서 정한다.
//  글자를 지운 편집은 땅을 넓히지 못한다 (음수는 0 으로 본다).
// ============================================================

// baro_land_max_cell : 지도에 그리는 최대 칸 수
const baro_land_max_cell = 900

// baro_land_owner : 문서 한 개의 주인
type baro_land_owner struct {
	Title string
	Owner string
	Score int
	Total int
}

// baro_land_user : 사람 한 명의 땅
type baro_land_user struct {
	Name  string
	Cells int
	Bytes int
}

// baro_land_color : 이름에서 늘 같은 색을 만든다
//
//	같은 사람은 어느 화면에서도 같은 색으로 보인다.
func baro_land_color(name string) string {
	if name == "" {
		return "#c9ced6"
	}

	sum := 0
	for index, one := range []byte(name) {
		sum = (sum*31 + int(one)*(index+1)) % 360
	}

	// 채도·밝기를 고정해 글자가 늘 읽히게 한다
	return "hsl(" + strconv.Itoa(sum) + ", 62%, 58%)"
}

// baro_land_history_max : 한 번에 훑는 편집 기록 수 (지도 한 장을 그리는 데 드는 상한)
//
//	위키가 아주 커져도 이 화면 하나 때문에 서버가 멈추지 않게 막아 둔다.
//	넘치면 최근 기록부터 본다.
const baro_land_history_max = 200000

// baro_land_owner_list : 문서마다 주인을 정한다
//
//	limit 은 살펴볼 문서 수 (편집이 많은 문서부터)
//	읽을 수 없는 문서는 지도에 넣지 않는다 (제목만으로도 새면 안 된다)
func baro_land_owner_list(db *sql.DB, config tool.Config, limit int) ([]baro_land_owner, int) {
	if limit < 1 || limit > baro_land_max_cell {
		limit = baro_land_max_cell
	}

	// 지금 살아 있는 문서만 땅으로 센다
	rows := tool.Query_DB(
		db,
		"select h.title, h.ip, h.leng from history h where exists (select 1 from data d where d.title = h.title)"+
			" order by h.date desc limit ?",
		baro_land_history_max,
	)
	defer rows.Close()

	type score_map map[string]int
	document := map[string]score_map{}
	order := []string{}

	for rows.Next() {
		title, ip_name, leng := "", "", ""
		if rows.Scan(&title, &ip_name, &leng) != nil {
			continue
		}
		if title == "" || ip_name == "" {
			continue
		}

		add := tool.Str_to_int(strings.TrimPrefix(strings.TrimSpace(leng), "+"))
		if add <= 0 {
			// 글자를 지운 편집도 「참여」 로는 세도록 아주 작은 점수를 준다
			add = 1
		}

		if _, exists := document[title]; !exists {
			document[title] = score_map{}
			order = append(order, title)
		}
		document[title][ip_name] += add
	}
	rows.Close()

	list := []baro_land_owner{}
	for _, title := range order {
		best_name := ""
		best_score := 0
		total := 0
		names := []string{}
		for name := range document[title] {
			names = append(names, name)
		}
		// 점수가 같을 때 화면이 흔들리지 않도록 이름 차례로 본다
		sort.Strings(names)
		for _, name := range names {
			score := document[title][name]
			total += score
			if score > best_score {
				best_score = score
				best_name = name
			}
		}

		list = append(list, baro_land_owner{Title: title, Owner: best_name, Score: best_score, Total: total})
	}

	// 땅이 큰(글자가 많은) 문서부터 보여 준다
	sort.SliceStable(list, func(a int, b int) bool {
		if list[a].Total != list[b].Total {
			return list[a].Total > list[b].Total
		}

		return list[a].Title < list[b].Title
	})

	// 읽을 수 없는 문서는 뺀다. 큰 문서부터 채우다 limit 에 닿으면 멈춘다.
	filter := tool.New_acl_render_filter(db, config.IP)
	allowed := []baro_land_owner{}
	for _, one := range list {
		if !filter.Allow(one.Title) {
			continue
		}

		allowed = append(allowed, one)
		if len(allowed) >= limit {
			break
		}
	}

	// 잘라 낸 칸 수를 함께 돌려준다 (조용한 절단 금지)
	return allowed, len(list) - len(allowed)
}

// baro_land_user_list : 사람마다 가진 땅 수
func baro_land_user_list(owners []baro_land_owner) []baro_land_user {
	cells := map[string]int{}
	bytes_sum := map[string]int{}
	for _, one := range owners {
		cells[one.Owner]++
		bytes_sum[one.Owner] += one.Score
	}

	list := []baro_land_user{}
	for name, number := range cells {
		list = append(list, baro_land_user{Name: name, Cells: number, Bytes: bytes_sum[name]})
	}

	sort.SliceStable(list, func(a int, b int) bool {
		if list[a].Cells != list[b].Cells {
			return list[a].Cells > list[b].Cells
		}

		return list[a].Name < list[b].Name
	})

	return list
}

// baro_land_map_html : 땅 지도
//
//	highlight 가 비어 있지 않으면 그 사람의 땅만 밝게 보인다.
func baro_land_map_html(owners []baro_land_owner, highlight string) string {
	if len(owners) == 0 {
		return tool.Baro_empty("아직 땅이 없습니다. 문서를 하나 만들면 첫 칸이 생깁니다.")
	}

	html := `<div class="bw_land">`
	for _, one := range owners {
		class_name := "bw_land_cell"
		if highlight != "" && one.Owner != highlight {
			class_name += " bw_land_dim"
		}

		title := tool.Namespace_display(one.Title) + " — " + one.Owner + " (" + tool.Baro_comma(one.Score) + "/" + tool.Baro_comma(one.Total) + ")"

		html += `<a class="` + class_name + `" style="background:` + tool.HTML_escape(baro_land_color(one.Owner)) + `"` +
			` href="/baro/land/doc/` + tool.Url_parser(one.Title) + `" title="` + tool.HTML_escape(title) + `">` +
			`<span class="bw_land_label">` + tool.HTML_escape(tool.Namespace_display(one.Title)) + `</span></a>`
	}

	return html + `</div>`
}

// baro_land_legend_html : 색과 사람 이름을 잇는 범례
func baro_land_legend_html(db *sql.DB, config tool.Config, users []baro_land_user, total_cell int, limit int) string {
	if len(users) == 0 {
		return ""
	}
	if limit < 1 {
		limit = 12
	}

	rows := [][]string{}
	other_cell := 0
	for index, one := range users {
		if index >= limit {
			other_cell += one.Cells
			continue
		}

		share := 0
		if total_cell > 0 {
			share = one.Cells * 100 / total_cell
		}

		rows = append(rows, []string{
			`<span class="bw_land_chip" style="background:` + tool.HTML_escape(baro_land_color(one.Name)) + `"></span>`,
			tool.Baro_user_link(db, one.Name, config.IP),
			tool.Baro_comma(one.Cells),
			tool.Baro_comma(one.Bytes),
			strconv.Itoa(share) + "%",
			tool.Baro_button("/baro/land/user/"+tool.Url_parser(one.Name), tool.Baro_t("이 사람 땅만"), ""),
		})
	}

	if other_cell > 0 {
		rows = append(rows, []string{
			`<span class="bw_land_chip" style="background:#c9ced6"></span>`,
			tool.Baro_t("그 밖의 사람들"),
			tool.Baro_comma(other_cell), "-",
			strconv.Itoa(other_cell*100/max_int(total_cell, 1)) + "%",
			"",
		})
	}

	return tool.Baro_table(
		[]string{"", tool.Get_language(db, "user", true), tool.Baro_t("가진 칸"), tool.Baro_t("글자 수"), tool.Baro_t("비율"), ""},
		rows, tool.Baro_t("아직 땅이 없습니다."),
	)
}

// max_int : 둘 중 큰 값
func max_int(a int, b int) int {
	if a > b {
		return a
	}

	return b
}

// baro_land_hidden_note : 지도에 넣지 못한 칸이 있다고 알리는 줄
//
//	읽을 수 없는 문서와 상한을 넘어 잘린 문서가 여기 들어간다.
//	조용히 자르면 「전부 보고 있다」고 잘못 읽힌다.
func baro_land_hidden_note(hidden int) string {
	if hidden <= 0 {
		return ""
	}

	return tool.Baro_note("읽을 수 없거나 상한(" + strconv.Itoa(baro_land_max_cell) + "칸)을 넘어 지도에 넣지 못한 문서가 " +
		tool.Baro_comma(hidden) + "개 있습니다.")
}

// baro_land_menu : 공통 메뉴
func baro_land_menu() [][]any {
	return [][]any{
		{"baro/land", tool.Baro_t("기여 땅따먹기")},
		{"baro", "바로위키"},
	}
}

// baro_land_tabs : 탭 줄
func baro_land_tabs(config tool.Config, active string) string {
	items := [][]string{
		{"/baro/land", tool.Baro_t("전체 지도")},
		{"/baro/rank", tool.Baro_t("기여 순위")},
		{"/baro/record", tool.Baro_t("기여 기록")},
	}
	if tool.Baro_is_user(config.IP) {
		items = append(items, []string{"/baro/land/user/" + tool.Url_parser(config.IP), tool.Baro_t("내 땅")})
	}

	return tool.Baro_tabs(items, active)
}

// View_baro_land : 전체 땅 지도
func View_baro_land(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	owners, hidden := baro_land_owner_list(db, config, baro_land_max_cell)
	users := baro_land_user_list(owners)

	body := tool.Baro_hero(
		tool.Baro_t("기여 땅따먹기"),
		tool.Baro_t("문서 한 개가 땅 한 칸입니다. 그 문서에 글자를 가장 많이 넣은 사람이 칸의 주인이 됩니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("땅 (문서)"), tool.Baro_comma(len(owners)), ""},
			{tool.Baro_t("주인 (사람)"), tool.Baro_comma(len(users)), ""},
			{tool.Baro_t("1등"), baro_land_top_name(users), baro_land_top_note(users, len(owners))},
		}),
	)

	inner := baro_land_tabs(config, "/baro/land")
	inner += baro_land_map_html(owners, "")
	inner += tool.Baro_note("칸을 누르면 그 문서 안에서 누가 얼마나 썼는지 볼 수 있습니다.")
	inner += baro_land_hidden_note(hidden)

	body += tool.Baro_card("", inner)
	body += tool.Baro_card(tool.Baro_t("땅 순위"), baro_land_legend_html(db, config, users, len(owners), 20))

	return tool.Baro_page(db, config, tool.Baro_t("기여 땅따먹기"), body, baro_land_menu())
}

// baro_land_top_name : 1등 이름
func baro_land_top_name(users []baro_land_user) string {
	if len(users) == 0 {
		return "-"
	}

	return users[0].Name
}

// baro_land_top_note : 1등이 가진 비율
func baro_land_top_note(users []baro_land_user, total int) string {
	if len(users) == 0 || total == 0 {
		return ""
	}

	return strconv.Itoa(users[0].Cells*100/total) + "%"
}

// View_baro_land_user : 한 사람의 땅
func View_baro_land_user(config tool.Config, user_name string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	user_name = strings.TrimSpace(user_name)
	if user_name == "" {
		user_name = config.IP
	}

	owners, hidden := baro_land_owner_list(db, config, baro_land_max_cell)
	users := baro_land_user_list(owners)

	mine := []baro_land_owner{}
	my_bytes := 0
	for _, one := range owners {
		if one.Owner == user_name {
			mine = append(mine, one)
			my_bytes += one.Score
		}
	}

	rank := 0
	for index, one := range users {
		if one.Name == user_name {
			rank = index + 1

			break
		}
	}

	share := "0%"
	if len(owners) > 0 {
		share = strconv.Itoa(len(mine)*100/len(owners)) + "%"
	}

	rank_text := "-"
	if rank > 0 {
		rank_text = strconv.Itoa(rank) + tool.Baro_t("등")
	}

	body := tool.Baro_hero_html(
		tool.Baro_t("내 땅"),
		tool.Baro_user_link(db, user_name, config.IP),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("가진 칸"), tool.Baro_comma(len(mine)), tool.Baro_t("전체 ") + tool.Baro_comma(len(owners)) + tool.Baro_t(" 칸 가운데")},
			{tool.Baro_t("차지한 비율"), share, ""},
			{tool.Baro_t("땅 순위"), rank_text, ""},
			{tool.Baro_t("쌓은 글자"), tool.Baro_comma(my_bytes), ""},
		}),
	)

	inner := baro_land_tabs(config, "/baro/land/user/"+tool.Url_parser(user_name))
	inner += baro_land_map_html(owners, user_name)
	inner += tool.Baro_note("밝게 보이는 칸이 이 사람이 주인인 문서입니다.")
	inner += baro_land_hidden_note(hidden)

	body += tool.Baro_card("", inner)

	list := [][]string{}
	for _, one := range mine {
		percent := 0
		if one.Total > 0 {
			percent = one.Score * 100 / one.Total
		}

		list = append(list, []string{
			tool.Baro_doc_link(one.Title),
			tool.Baro_comma(one.Score),
			strconv.Itoa(percent) + "%",
			tool.Baro_button("/baro/land/doc/"+tool.Url_parser(one.Title), tool.Baro_t("자세히"), ""),
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("내가 주인인 문서"),
		tool.Baro_table(
			[]string{tool.Get_language(db, "document", true), tool.Baro_t("쌓은 글자"), tool.Baro_t("비율"), ""},
			list, tool.Baro_t("아직 주인인 문서가 없습니다. 문서를 크게 채우면 그 칸의 주인이 됩니다."),
		),
	)

	body += tool.Baro_card("", tool.Baro_button("/baro/record/user/"+tool.Url_parser(user_name)+"/all", tool.Baro_t("기여 기록 보기"), "")+" "+
		tool.Baro_button("/baro/land", tool.Baro_t("전체 지도"), ""))

	return tool.Baro_page(db, config, tool.Baro_t("기여 땅따먹기"), body, baro_land_menu())
}

// View_baro_land_doc : 문서 한 개 안에서의 기여 비율
func View_baro_land_doc(config tool.Config, doc_name string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	title := tool.Namespace_normalize(strings.TrimSpace(doc_name))
	if title == "" {
		return tool.Get_redirect("/baro/land")
	}

	// 읽을 수 없는 문서는 기여 나눔도 보여 주지 않는다
	if !tool.Check_acl(db, title, "", "render", config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("이 문서를 읽을 권한이 없습니다."))
	}

	rows := tool.Query_DB(db, "select ip, leng from history where title = ?", title)
	defer rows.Close()

	score := map[string]int{}
	count := map[string]int{}
	total := 0
	for rows.Next() {
		ip_name, leng := "", ""
		if rows.Scan(&ip_name, &leng) != nil {
			continue
		}
		if ip_name == "" {
			continue
		}

		add := tool.Str_to_int(strings.TrimPrefix(strings.TrimSpace(leng), "+"))
		if add <= 0 {
			add = 1
		}

		score[ip_name] += add
		count[ip_name]++
		total += add
	}
	rows.Close()

	names := []string{}
	for name := range score {
		names = append(names, name)
	}
	sort.SliceStable(names, func(a int, b int) bool {
		if score[names[a]] != score[names[b]] {
			return score[names[a]] > score[names[b]]
		}

		return names[a] < names[b]
	})

	bar := `<div class="bw_land_bar">`
	for _, name := range names {
		percent := 0
		if total > 0 {
			percent = score[name] * 100 / total
		}
		if percent < 1 {
			percent = 1
		}

		bar += `<span class="bw_land_bar_part" style="width:` + strconv.Itoa(percent) + `%;background:` +
			tool.HTML_escape(baro_land_color(name)) + `" title="` + tool.HTML_escape(name+" "+strconv.Itoa(percent)+"%") + `"></span>`
	}
	bar += `</div>`

	list := [][]string{}
	for _, name := range names {
		percent := 0
		if total > 0 {
			percent = score[name] * 100 / total
		}

		list = append(list, []string{
			`<span class="bw_land_chip" style="background:` + tool.HTML_escape(baro_land_color(name)) + `"></span>`,
			tool.Baro_user_link(db, name, config.IP),
			tool.Baro_comma(score[name]),
			tool.Baro_comma(count[name]),
			strconv.Itoa(percent) + "%",
			tool.Baro_button("/baro/land/user/"+tool.Url_parser(name), tool.Baro_t("이 사람 땅"), ""),
		})
	}

	owner := "-"
	if len(names) > 0 {
		owner = names[0]
	}

	body := tool.Baro_hero(
		tool.Baro_t("문서 땅 나눔"),
		tool.Namespace_display(title),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("주인"), owner, ""},
			{tool.Baro_t("참여한 사람"), tool.Baro_comma(len(names)), ""},
			{tool.Baro_t("쌓인 글자"), tool.Baro_comma(total), ""},
		}),
	)

	inner := bar
	inner += tool.Baro_table(
		[]string{"", tool.Get_language(db, "user", true), tool.Baro_t("쌓은 글자"), tool.Baro_t("편집 수"), tool.Baro_t("비율"), ""},
		list, tool.Baro_t("기여 기록이 없습니다."),
	)

	body += tool.Baro_card("", inner)
	body += tool.Baro_card("", tool.Baro_button("/w/" + tool.Doc_url(title), tool.Baro_t("문서 보기"), "primary")+" "+
		tool.Baro_button("/history/"+tool.Url_parser(title), tool.Get_language(db, "history", true), "")+" "+
		tool.Baro_button("/baro/land", tool.Baro_t("전체 지도"), ""))

	return tool.Baro_page(db, config, tool.Baro_t("문서 땅 나눔"), body, baro_land_menu())
}
