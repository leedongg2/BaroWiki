package route

import (
	"database/sql"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  5-6 지식 그래프
//
//  인물↔사건 / 인물↔단체 / 인물↔지역 / 인물↔학교 / 사건↔연도 관계를
//  baro_doc_meta 의 relation 값에 JSON 으로 저장하고 인라인 SVG 로 그린다.
//  기능 플래그 knowledge_graph 로 노출을 조절한다.
// ============================================================

// baro_find_relation_kind_list : 관계 종류 (키, 이름, 색)
func baro_find_relation_kind_list() [][]string {
	return [][]string{
		{"event", "사건", "#c2410c"},
		{"org", tool.Baro_t("단체"), "#1d4ed8"},
		{"place", tool.Baro_t("지역"), "#047857"},
		{"school", tool.Baro_t("학교"), "#7c3aed"},
		{"person", tool.Baro_t("인물"), "#b91c1c"},
		{"year", tool.Baro_t("연도"), "#525252"},
	}
}

// baro_find_relation_kind_name : 관계 종류 이름
func baro_find_relation_kind_name(kind string) string {
	for _, row := range baro_find_relation_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// baro_find_relation_kind_color : 관계 종류 색
func baro_find_relation_kind_color(kind string) string {
	for _, row := range baro_find_relation_kind_list() {
		if row[0] == kind {
			return row[2]
		}
	}

	return "#525252"
}

// baro_find_relation_get : 문서의 관계 JSON 을 종류별 목록으로
func baro_find_relation_get(db *sql.DB, title string) map[string][]string {
	out := map[string][]string{}
	raw := tool.Baro_doc_meta_get(db, title, "relation")
	if strings.TrimSpace(raw) == "" {
		return out
	}

	decoded := tool.Baro_json_dec(raw)
	for _, row := range baro_find_relation_kind_list() {
		value, ok := decoded[row[0]]
		if !ok {
			continue
		}

		list := []string{}
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					list = append(list, strings.TrimSpace(text))
				}
			}
		case string:
			for _, text := range tool.Baro_split_lines(typed) {
				list = append(list, text)
			}
		}

		if len(list) > 0 {
			out[row[0]] = list
		}
	}

	return out
}

// baro_find_relation_save : 관계 JSON 저장
func baro_find_relation_save(db *sql.DB, config tool.Config, title string, post url.Values) string {
	if !baro_find_doc_exists(db, title) {
		return tool.Baro_t("그런 문서가 없습니다.")
	}

	before := tool.Baro_doc_meta_get(db, title, "relation")
	value := map[string][]string{}
	total := 0

	for _, row := range baro_find_relation_kind_list() {
		list := []string{}
		for _, line := range tool.Baro_split_lines(baro_find_val(post, "rel_"+row[0])) {
			if tool.Get_len(line) > 100 {
				line = tool.Baro_trunc(line, 100)
			}

			list = append(list, line)
			total++

			if len(list) >= 30 {
				break
			}
		}

		if len(list) > 0 {
			value[row[0]] = list
		}
	}

	if total > 120 {
		return tool.Baro_t("관계가 너무 많습니다. 한 문서에 120개까지만 넣을 수 있습니다.")
	}

	after := ""
	if len(value) > 0 {
		after = tool.Baro_json_enc(value)
	}

	tool.Baro_doc_meta_set(db, title, "relation", after)
	tool.Baro_audit(db, config, "doc_relation_edit", title, before, after, "지식 그래프 관계 편집")
	tool.Baro_public_log(db, "doc_relation_edit", config.IP, title, "지식 그래프 관계 편집", tool.Baro_itoa(total)+"개")
	tool.Baro_contrib_add(db, config.IP, "category", title, "", "", "지식 그래프 관계 편집")

	return ""
}

// baro_find_graph_node : 그래프에 그릴 점 하나
type baro_find_graph_node struct {
	Label  string
	Kind   string
	Exists bool
}

// baro_find_graph_svg : 중심 문서 주위에 관계를 원형으로 배치한 인라인 SVG
func baro_find_graph_svg(center string, nodes []baro_find_graph_node) string {
	width, height := 760.0, 480.0
	cx, cy := width/2, height/2
	radius := 178.0

	svg := `<div style="overflow-x:auto">`
	svg += `<svg viewBox="0 0 760 480" width="760" height="480" role="img" aria-label="지식 그래프" style="max-width:100%;height:auto">`
	svg += `<rect x="0" y="0" width="760" height="480" fill="none"/>`

	if len(nodes) == 0 {
		svg += `<text x="380" y="240" text-anchor="middle" font-size="15" fill="#888">등록된 관계가 없습니다.</text>`
		svg += `</svg></div>`

		return svg
	}

	// 선을 먼저 그리고 그 위에 점을 얹는다.
	for index, node := range nodes {
		angle := 2*math.Pi*float64(index)/float64(len(nodes)) - math.Pi/2
		x := cx + radius*math.Cos(angle)
		y := cy + radius*math.Sin(angle)

		svg += `<line x1="` + baro_find_num(cx) + `" y1="` + baro_find_num(cy) + `" x2="` + baro_find_num(x) + `" y2="` + baro_find_num(y) +
			`" stroke="` + baro_find_relation_kind_color(node.Kind) + `" stroke-width="1.4" stroke-opacity="0.55"/>`
	}

	for index, node := range nodes {
		angle := 2*math.Pi*float64(index)/float64(len(nodes)) - math.Pi/2
		x := cx + radius*math.Cos(angle)
		y := cy + radius*math.Sin(angle)
		color := baro_find_relation_kind_color(node.Kind)

		label := tool.Baro_trunc(node.Label, 12)
		anchor := "middle"
		text_y := y + 26

		point := `<circle cx="` + baro_find_num(x) + `" cy="` + baro_find_num(y) + `" r="9" fill="` + color + `"/>`
		point += `<text x="` + baro_find_num(x) + `" y="` + baro_find_num(text_y) + `" text-anchor="` + anchor +
			`" font-size="12" fill="currentColor">` + baro_find_xml_escape(label) + `</text>`

		if node.Exists {
			point = `<a href="/w/` + baro_find_xml_escape(tool.Url_parser(node.Label)) + `">` + point + `</a>`
		}

		svg += point
	}

	svg += `<circle cx="` + baro_find_num(cx) + `" cy="` + baro_find_num(cy) + `" r="15" fill="#111" stroke="#fff" stroke-width="2"/>`
	svg += `<text x="` + baro_find_num(cx) + `" y="` + baro_find_num(cy-24) + `" text-anchor="middle" font-size="14" font-weight="bold" fill="currentColor">` +
		baro_find_xml_escape(tool.Baro_trunc(center, 18)) + `</text>`
	svg += `</svg></div>`

	return svg
}

// baro_find_num : SVG 좌표 문자열
func baro_find_num(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}

// baro_find_timeline_svg : 연도별 사건을 가로축에 배치한 타임라인
func baro_find_timeline_svg(points [][]string) string {
	if len(points) == 0 {
		return tool.Baro_empty(tool.Baro_t("연도가 붙은 관계가 없어 타임라인을 그릴 수 없습니다. 관계에 연도를 넣어 보세요."))
	}

	min_year, max_year := 99999, -99999
	for _, point := range points {
		year := tool.Str_to_int(point[0])
		if year < min_year {
			min_year = year
		}
		if year > max_year {
			max_year = year
		}
	}

	if max_year == min_year {
		min_year--
		max_year++
	}

	width := 760.0
	left, right := 60.0, width-60
	base_y := 92.0
	height := 180.0

	svg := `<div style="overflow-x:auto">`
	svg += `<svg viewBox="0 0 760 ` + baro_find_num(height) + `" width="760" height="` + baro_find_num(height) +
		`" role="img" aria-label="타임라인" style="max-width:100%;height:auto">`
	svg += `<line x1="` + baro_find_num(left) + `" y1="` + baro_find_num(base_y) + `" x2="` + baro_find_num(right) +
		`" y2="` + baro_find_num(base_y) + `" stroke="currentColor" stroke-width="1.5" stroke-opacity="0.5"/>`

	for index, point := range points {
		year := tool.Str_to_int(point[0])
		ratio := float64(year-min_year) / float64(max_year-min_year)
		x := left + (right-left)*ratio

		up := index%2 == 0
		text_y := base_y - 18
		tick_y := base_y - 10
		if !up {
			text_y = base_y + 26
			tick_y = base_y + 10
		}

		svg += `<line x1="` + baro_find_num(x) + `" y1="` + baro_find_num(base_y) + `" x2="` + baro_find_num(x) +
			`" y2="` + baro_find_num(tick_y) + `" stroke="` + baro_find_relation_kind_color(point[2]) + `" stroke-width="2"/>`
		svg += `<circle cx="` + baro_find_num(x) + `" cy="` + baro_find_num(base_y) + `" r="5" fill="` +
			baro_find_relation_kind_color(point[2]) + `"/>`
		svg += `<text x="` + baro_find_num(x) + `" y="` + baro_find_num(text_y) + `" text-anchor="middle" font-size="11" fill="currentColor">` +
			baro_find_xml_escape(point[0]) + ` ` + baro_find_xml_escape(tool.Baro_trunc(point[1], 10)) + `</text>`
	}

	svg += `<text x="` + baro_find_num(left) + `" y="` + baro_find_num(base_y+52) + `" font-size="11" fill="#888">` +
		baro_find_xml_escape(tool.Baro_itoa(min_year)) + `</text>`
	svg += `<text x="` + baro_find_num(right) + `" y="` + baro_find_num(base_y+52) + `" text-anchor="end" font-size="11" fill="#888">` +
		baro_find_xml_escape(tool.Baro_itoa(max_year)) + `</text>`
	svg += `</svg></div>`

	return svg
}

// View_baro_find_graph : 지식 그래프 목록 (/baro/doc/graph)
func View_baro_find_graph(config tool.Config, page string, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	body := baro_find_tabs("/baro/doc/graph")
	body += tool.Baro_hero("지식 그래프", tool.Baro_t("인물과 사건·단체·지역·학교를 이어 봅니다."), "")

	if !tool.Baro_flag_on(db, config.IP, "knowledge_graph") {
		body += tool.Baro_alert("info",
			tool.Baro_t("지식 그래프는 아직 준비 중인 기능입니다. 기능 플래그 <b>knowledge_graph</b> 가 켜진 사용자에게만 보입니다."))
		body += tool.Baro_card(tool.Baro_t("대신 할 수 있는 일"),
			tool.Baro_button("/baro/doc/alias", tool.Baro_t("별칭·동명이인 관리"), "primary")+" "+
				tool.Baro_button("/baro/search", tool.Baro_t("검색으로 찾기"), "")+" "+
				tool.Baro_button("/baro/discover", tool.Baro_t("발견 화면"), ""))

		return tool.Baro_page(db, config, "지식 그래프", body, baro_find_menu())
	}

	keyword := baro_find_val(query, "q")

	page_int := baro_find_page_int(page)
	if raw_page := baro_find_val(query, "page"); raw_page != "" {
		page_int = baro_find_page_int(raw_page)
	}

	offset := (page_int - 1) * tool.Baro_page_size

	sql_text := "select title, data from baro_doc_meta where name = 'relation' and not data = ''"
	args := []any{}
	if keyword != "" {
		sql_text += " and lower(title) like ?"
		args = append(args, "%"+strings.ToLower(keyword)+"%")
	}
	sql_text += " order by title limit ?, ?"
	args = append(args, offset, tool.Baro_page_size)

	rows := tool.Query_DB(db, sql_text, args...)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		title, data := "", ""
		if err := rows.Scan(&title, &data); err != nil {
			continue
		}

		decoded := tool.Baro_json_dec(data)
		parts := []string{}
		total := 0

		for _, kind := range baro_find_relation_kind_list() {
			if value, ok := decoded[kind[0]]; ok {
				if list, ok := value.([]any); ok && len(list) > 0 {
					parts = append(parts, kind[1]+" "+tool.Baro_itoa(len(list)))
					total += len(list)
				}
			}
		}

		table_rows = append(table_rows, []string{
			tool.Baro_doc_link(title),
			tool.HTML_escape(strings.Join(parts, " · ")),
			tool.Baro_itoa(total),
			tool.Baro_button("/baro/doc/graph/"+tool.Url_parser(title), "그래프 보기", ""),
		})
	}

	page_url := url.Values{}
	page_url.Set("q", keyword)

	list_html := tool.Baro_table([]string{"문서", tool.Baro_t("관계 구성"), tool.Baro_t("관계 수"), "보기"}, table_rows, "")
	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("아직 관계가 등록된 문서가 없습니다. 문서 화면에서 관계를 넣어 주세요."))
	}
	list_html += tool.Baro_paging(page_int, len(table_rows), tool.Baro_page_size, "/baro/doc/graph?"+page_url.Encode()+"&page={}")

	find_form := tool.Baro_input("q", tool.Baro_t("문서 찾기"), keyword, tool.Baro_t("문서 이름 일부"), "")
	find_form += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("찾기"), "primary") + `</div>`

	go_form := tool.Baro_input("doc", tool.Baro_t("문서 이름"), "", tool.Baro_t("예: 김성범"), tool.Baro_t("그 문서의 그래프로 바로 갑니다."))
	go_form += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("그래프 열기"), "primary") + `</div>`

	pills := []string{}
	for _, row := range baro_find_relation_kind_list() {
		pills = append(pills, row[1])
	}

	body += tool.Baro_card(tool.Baro_t("관계가 등록된 문서"), `<form class="bw_form" method="get" action="/baro/doc/graph">`+find_form+`</form>`+list_html)
	body += tool.Baro_card(tool.Baro_t("문서로 바로 가기"), `<form class="bw_form" method="get" action="/baro/doc/graph/go">`+go_form+`</form>`+
		tool.Baro_note(tool.Baro_t("다룰 수 있는 관계 종류"))+tool.Baro_pills(pills))

	return tool.Baro_page(db, config, "지식 그래프", body, baro_find_menu())
}

// View_baro_find_graph_doc : 문서별 지식 그래프 (/baro/doc/graph/<문서>)
func View_baro_find_graph_doc(config tool.Config, doc_name string, query url.Values, post url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	doc_name = tool.Namespace_normalize(strings.TrimSpace(doc_name))

	// /baro/doc/graph/go?doc=... 로 들어온 경우
	if doc_name == "" || strings.EqualFold(doc_name, "go") {
		target := tool.Namespace_normalize(baro_find_val(query, "doc"))
		if target == "" {
			return tool.Get_redirect("/baro/doc/graph")
		}

		return tool.Get_redirect("/baro/doc/graph/" + tool.Url_parser(target))
	}

	if post != nil {
		if wall, need := tool.Baro_login_wall(db, config, "/baro/doc/graph"); need {
			return wall
		}
		if tool.Baro_read_only(db) {
			return tool.Baro_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db))
		}

		message := baro_find_relation_save(db, config, doc_name, post)
		back := "/baro/doc/graph/" + tool.Url_parser(doc_name)
		if message != "" {
			back += "?msg=" + tool.Url_parser(message)
		}

		return tool.Get_redirect(back)
	}

	body := baro_find_tabs("/baro/doc/graph")
	body += tool.Baro_crumb([][]string{
		{"/baro/doc/graph", "지식 그래프"},
		{"", tool.Namespace_display(doc_name)},
	})

	if !tool.Baro_flag_on(db, config.IP, "knowledge_graph") {
		body += tool.Baro_alert("info", tool.Baro_t("지식 그래프는 기능 플래그 <b>knowledge_graph</b> 가 켜진 사용자에게만 보입니다."))

		return tool.Baro_page(db, config, "지식 그래프", body, baro_find_menu())
	}

	if !baro_find_doc_exists(db, doc_name) {
		body += tool.Baro_alert("warn", `<b>`+tool.HTML_escape(tool.Namespace_display(doc_name))+tool.Baro_t(`</b> 문서가 없습니다.`))
		body += tool.Baro_card(tool.Baro_t("무엇을 할 수 있나요"),
			tool.Baro_button("/edit/"+tool.Url_parser(doc_name), "이 문서 만들기", "primary")+" "+
				tool.Baro_button("/baro/doc/graph", tool.Baro_t("그래프 목록으로"), ""))

		return tool.Baro_page(db, config, tool.Baro_t("지식 그래프 : ")+tool.Namespace_display(doc_name), body, baro_find_menu())
	}

	if message := baro_find_val(query, "msg"); message != "" {
		body += tool.Baro_alert("warn", tool.HTML_escape(message))
	}

	relation := baro_find_relation_get(db, doc_name)
	meta := baro_find_meta_all(db, doc_name)

	nodes := []baro_find_graph_node{}
	rows := [][]string{}
	timeline := [][]string{}

	for _, kind := range baro_find_relation_kind_list() {
		for _, label := range relation[kind[0]] {
			normalized := tool.Namespace_normalize(label)
			exists := baro_find_doc_exists(db, normalized)

			nodes = append(nodes, baro_find_graph_node{Label: label, Kind: kind[0], Exists: exists})

			link := tool.HTML_escape(label)
			if exists {
				link = tool.Baro_doc_link(normalized)
			}

			state := tool.Baro_badge(tool.Baro_t("문서 있음"), "ok")
			if !exists {
				state = tool.Baro_badge(tool.Baro_t("문서 없음"), "warn") + " " +
					tool.Baro_button("/edit/"+tool.Url_parser(normalized), "만들기", "")
			}

			rows = append(rows, []string{
				tool.Baro_badge(kind[1], "info"),
				link,
				state,
			})

			// 연도 정보가 있으면 타임라인에 올린다.
			if kind[0] == "year" {
				year := tool.Str_to_int(strings.TrimSpace(strings.Split(label, " ")[0]))
				if year > 0 {
					name := strings.TrimSpace(strings.TrimPrefix(label, strings.Split(label, " ")[0]))
					if name == "" {
						name = tool.Namespace_display(doc_name)
					}

					timeline = append(timeline, []string{tool.Baro_itoa(year), name, "year"})
				}

				continue
			}

			if exists {
				other := baro_find_meta_all(db, normalized)
				if other["birth_year"] != "" {
					timeline = append(timeline, []string{other["birth_year"], label, kind[0]})
				}
			}
		}
	}

	if meta["birth_year"] != "" {
		timeline = append(timeline, []string{meta["birth_year"], tool.Namespace_display(doc_name) + tool.Baro_t(" 출생"), "person"})
	}
	if meta["death_year"] != "" {
		timeline = append(timeline, []string{meta["death_year"], tool.Namespace_display(doc_name) + tool.Baro_t(" 사망"), "person"})
	}

	sort.SliceStable(timeline, func(a int, b int) bool {
		return tool.Str_to_int(timeline[a][0]) < tool.Str_to_int(timeline[b][0])
	})

	body += tool.Baro_hero(tool.Namespace_display(doc_name), baro_find_person_line(meta),
		tool.Baro_button("/w/"+tool.Url_parser(doc_name), "문서 보기", "primary")+" "+
			tool.Baro_button("/baro/doc/alias/"+tool.Url_parser(doc_name), "별칭·인물 정보", ""))

	legend := []string{}
	for _, kind := range baro_find_relation_kind_list() {
		legend = append(legend, `<span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:`+
			kind[2]+`"></span> `+tool.HTML_escape(kind[1]))
	}

	body += tool.Baro_card_ex(tool.Baro_t("관계도"), tool.Baro_itoa(len(nodes))+tool.Baro_t("개의 관계"),
		baro_find_graph_svg(tool.Namespace_display(doc_name), nodes)+
			`<div class="bw_note">`+strings.Join(legend, " &nbsp; ")+`</div>`, "")

	body += tool.Baro_card_ex(tool.Baro_t("타임라인"), tool.Baro_t("연도가 붙은 관계를 가로축에 배치했습니다"),
		baro_find_timeline_svg(timeline), "")

	body += tool.Baro_card(tool.Baro_t("관계 목록"), tool.Baro_table([]string{"종류", tool.Baro_t("대상"), tool.Baro_t("문서 상태")}, rows,
		tool.Baro_t("등록된 관계가 없습니다.")))

	if tool.Baro_is_user(config.IP) {
		form := ""
		for _, kind := range baro_find_relation_kind_list() {
			help := tool.Baro_t("한 줄에 하나씩 적습니다.")
			if kind[0] == "year" {
				help = tool.Baro_t("연도를 먼저 적고 이름을 붙입니다. 예: 1919 3·1 운동")
			}

			form += tool.Baro_textarea("rel_"+kind[0], kind[1], strings.Join(relation[kind[0]], "\n"), 4, "", help)
		}
		form += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("관계 저장"), "primary") + `</div>`

		body += tool.Baro_card("관계 편집", tool.Baro_form("/baro/doc/graph/"+tool.Url_parser(doc_name), form)+
			tool.Baro_note(tool.Baro_t("관계는 baro_doc_meta 의 relation 값에 JSON 으로 저장됩니다.")))
	} else {
		body += tool.Baro_card("관계 편집", tool.Baro_alert("info",
			tool.Baro_t("관계 편집은 로그인한 기여자만 할 수 있습니다. ")+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary")))
	}

	return tool.Baro_page(db, config, tool.Baro_t("지식 그래프 : ")+tool.Namespace_display(doc_name), body, baro_find_menu())
}
