package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  3-11 분류 도구 — /baro/category
//
//  openNAMU 본체에도 분류 이름공간과 분류 문서 화면은 있다.
//  여기서는 본체가 하지 못하는 "분류 전체를 가로질러 보는" 일을 맡는다.
//    · 분류 목록 · 분류 트리 · 분류별 최근 문서와 기여자
//    · 분류 통계 · 중복/순환/빈 분류 점검 · 분류 없는 문서
// ============================================================

// baro_cat_stats_key : 분류 통계 캐시 키
const baro_cat_stats_key = "baro_cat_stats"

// baro_cat_dup_key : 중복 분류 검사 캐시 키
const baro_cat_dup_key = "baro_cat_dup"

// View_baro_cat_list : 분류 목록 (/baro/category)
func View_baro_cat_list(config tool.Config, page string, query string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	page_num := baro_cat_page_no(page)
	query = strings.TrimSpace(query)

	idx := baro_cat_index_load(db)
	names, counts := baro_cat_index_cats(idx)
	_, down := baro_cat_index_edges(idx)

	// 검색어가 있으면 이름으로 걸러 낸다
	if query != "" {
		need := tool.Baro_norm(query)
		filtered := []string{}
		for _, name := range names {
			if strings.Contains(tool.Baro_norm(baro_cat_disp(name)), need) {
				filtered = append(filtered, name)
			}
		}
		names = filtered
	}

	// 소속 문서가 많은 분류부터
	sort.SliceStable(names, func(i int, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}

		return names[i] < names[j]
	})

	total := len(names)

	rows := [][]string{}
	for _, name := range baro_cat_slice(names, page_num) {
		exist_html := tool.Baro_badge(tool.Baro_t("문서 있음"), "ok")
		if !baro_cat_index_exists(db, idx, name) {
			exist_html = tool.Baro_badge(tool.Baro_t("분류 문서 없음"), "warn")
		}

		rows = append(rows, []string{
			baro_cat_cat_link(name) + " " + exist_html,
			tool.Baro_comma(counts[name]),
			tool.Baro_comma(len(down[name])),
			tool.Baro_time_cell(baro_cat_index_cat_last(idx, name)),
			`<a class="bw_btn" href="/baro/category/view/` + tool.Url_parser(name) + tool.Baro_t(`">자세히</a> `) +
				`<a class="bw_btn" href="/w/` + tool.Url_parser(name) + tool.Baro_t(`">분류 문서</a>`),
		})
	}

	body := tool.Baro_hero(
		tool.Baro_t("분류 도구"),
		tool.Baro_t("분류를 가로질러 보고, 빠지거나 잘못 달린 분류를 찾아냅니다."),
		"",
	)
	body += baro_cat_tabs("/baro/category")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("분류 목록")}})

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("분류 수"), tool.Baro_comma(total), tool.Baro_t("이름이 확인된 분류")},
		{tool.Baro_t("이 쪽"), tool.Baro_comma(len(rows)), tool.Baro_itoa(page_num) + tool.Baro_t("쪽")},
		{tool.Baro_t("전체 문서"), tool.Baro_comma(idx.DocAll), tool.Baro_t("휴지통 제외")},
		{tool.Baro_t("검색어"), func() string {
			if query == "" {
				return tool.Baro_t("전체")
			}

			return tool.Baro_trunc(query, 20)
		}(), tool.Baro_t("이름으로 거르기")},
	})

	body += baro_cat_index_note(idx)

	body += tool.Baro_card(tool.Baro_t("분류 찾기"), baro_cat_go_form("/baro/category", "q", tool.Baro_t("분류 이름"), query, tool.Baro_t("이름의 일부만 넣어도 됩니다.")))

	list_html := tool.Baro_table(
		[]string{"분류", tool.Baro_t("문서 수"), tool.Baro_t("하위 분류 수"), tool.Baro_t("마지막 변경"), tool.Baro_t("도구")},
		rows,
		"",
	)

	if len(rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("조건에 맞는 분류가 없습니다. 문서에 [[분류:이름]] 을 달면 여기에 나타납니다."))
	}

	list_url := "/baro/category?page={}"
	if query != "" {
		list_url += "&q=" + url.QueryEscape(query)
	}

	list_html += tool.Baro_paging(page_num, len(rows), baro_cat_page_size, list_url)

	body += tool.Baro_card(tool.Baro_t("분류 목록 (문서 수 많은 순)"), list_html)

	return tool.Baro_page(db, config, tool.Baro_t("분류 목록"), body, baro_cat_menu())
}

// baro_cat_tree_style : 트리 화면 전용 스타일과 펼치기/접기 동작
func baro_cat_tree_style() string {
	return `<style id="bw_cat_tree_css">
.bw_cat_tree ul{list-style:none;margin:0;padding-left:18px;border-left:1px dashed rgba(128,128,128,.35);}
.bw_cat_tree>ul{padding-left:0;border-left:0;}
.bw_cat_tree li{margin:2px 0;line-height:1.6;}
.bw_cat_tog{display:inline-block;width:18px;text-align:center;cursor:pointer;user-select:none;font-weight:700;opacity:.7;}
.bw_cat_tog:hover{opacity:1;}
.bw_cat_leaf{display:inline-block;width:18px;text-align:center;opacity:.35;}
.bw_cat_n{font-size:.85em;opacity:.7;margin-left:4px;}
.bw_cat_off{display:none;}
</style>
<script id="bw_cat_tree_js">
document.addEventListener('click', function (event) {
  var target = event.target;
  if (!target || !target.classList || !target.classList.contains('bw_cat_tog')) { return; }
  var box = target.parentNode.querySelector('ul');
  if (!box) { return; }
  if (box.classList.contains('bw_cat_off')) {
    box.classList.remove('bw_cat_off');
    target.textContent = '-';
  } else {
    box.classList.add('bw_cat_off');
    target.textContent = '+';
  }
});
function bw_cat_all(open) {
  var list = document.querySelectorAll('.bw_cat_tree ul ul');
  for (var i = 0; i < list.length; i++) {
    if (open) { list[i].classList.remove('bw_cat_off'); } else { list[i].classList.add('bw_cat_off'); }
  }
  var togs = document.querySelectorAll('.bw_cat_tog');
  for (var j = 0; j < togs.length; j++) { togs[j].textContent = open ? '-' : '+'; }
}
</script>`
}

// View_baro_cat_tree : 분류 트리 (/baro/category/tree)
//
//	분류 문서 본문에 다른 분류가 달려 있으면 그 분류의 하위 분류로 본다.
//	(category:역사 안에 [[분류:한국사]] 가 있으면 역사 는 한국사 의 하위 분류)
func View_baro_cat_tree(config tool.Config, depth string, root string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	max_depth := tool.Str_to_int(depth)
	if max_depth < 1 {
		max_depth = 3
	}
	if max_depth > 8 {
		max_depth = 8
	}

	idx := baro_cat_index_load(db)
	up, down := baro_cat_index_edges(idx)
	_, counts := baro_cat_index_cats(idx)

	// 뿌리 : 상위 분류가 없는 분류
	roots := []string{}
	for name := range counts {
		if len(up[name]) == 0 {
			roots = append(roots, name)
		}
	}
	sort.SliceStable(roots, func(i int, j int) bool {
		if len(down[roots[i]]) != len(down[roots[j]]) {
			return len(down[roots[i]]) > len(down[roots[j]])
		}

		return roots[i] < roots[j]
	})

	root = baro_cat_norm_cat(root)
	if root != "" {
		roots = []string{root}
	}

	// 그리기 (마디 수 상한을 둔다)
	budget := 1500
	visited := map[string]bool{}

	var draw func(name string, level int, path map[string]bool) string
	draw = func(name string, level int, path map[string]bool) string {
		if budget <= 0 {
			return ""
		}
		budget--
		visited[name] = true

		label := baro_cat_cat_link(name)
		label += tool.Baro_t(`<span class="bw_cat_n">문서 `) + tool.Baro_comma(counts[name]) + `</span>`

		kids := down[name]
		if len(kids) == 0 {
			return `<li><span class="bw_cat_leaf">·</span>` + label + `</li>`
		}

		if path[name] {
			return `<li><span class="bw_cat_leaf">·</span>` + label +
				` ` + tool.Baro_badge(tool.Baro_t("순환"), "danger") + `</li>`
		}

		if level >= max_depth {
			return `<li><span class="bw_cat_leaf">·</span>` + label +
				` <a class="bw_btn" href="/baro/category/tree?root=` + tool.Url_parser(name) +
				`&depth=` + tool.Baro_itoa(max_depth) + `">하위 ` + tool.Baro_comma(len(kids)) + tool.Baro_t(`개 더 보기</a></li>`)
		}

		path[name] = true

		inner := ""
		for _, kid := range kids {
			inner += draw(kid, level+1, path)
			if budget <= 0 {
				break
			}
		}

		delete(path, name)

		open_class := ""
		mark := "-"
		if level >= 1 {
			open_class = " bw_cat_off"
			mark = "+"
		}

		return `<li><span class="bw_cat_tog">` + mark + `</span>` + label +
			`<ul class="` + strings.TrimSpace(open_class) + `">` + inner + `</ul></li>`
	}

	tree_html := `<div class="bw_cat_tree"><ul>`
	for _, name := range roots {
		tree_html += draw(name, 0, map[string]bool{})
		if budget <= 0 {
			break
		}
	}
	tree_html += `</ul></div>`

	if len(roots) == 0 {
		tree_html = tool.Baro_empty(tool.Baro_t("아직 분류 사이의 상하 관계가 없습니다. 분류 문서 안에 상위 분류를 달아 주세요."))
	}

	// 트리에 못 들어간 분류 (순환에 갇힌 분류)
	// 시작 분류를 지정한 경우에는 다른 가지를 그리지 않았을 뿐이므로 세지 않는다.
	left_out := []string{}
	if root == "" {
		for name := range counts {
			if !visited[name] && len(up[name]) > 0 {
				left_out = append(left_out, name)
			}
		}
		sort.Strings(left_out)
	}

	body := tool.Baro_hero(tool.Baro_t("분류 트리"), tool.Baro_t("상위 분류와 하위 분류의 관계를 들여다봅니다."), "")
	body += baro_cat_tabs("/baro/category/tree")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/category", tool.Baro_t("분류 도구")}, {"", tool.Baro_t("분류 트리")}})
	body += baro_cat_tree_style()

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("분류 수"), tool.Baro_comma(len(counts)), tool.Baro_t("이름이 확인된 분류")},
		{tool.Baro_t("상하 관계"), tool.Baro_comma(len(down)), tool.Baro_t("하위 분류를 가진 분류")},
		{tool.Baro_t("최상위 분류"), tool.Baro_comma(len(roots)), tool.Baro_t("상위 분류가 없는 분류")},
		{tool.Baro_t("깊이 제한"), tool.Baro_itoa(max_depth) + tool.Baro_t("단계"), tool.Baro_t("더 깊은 곳은 눌러서 이어 봅니다")},
	})

	body += baro_cat_index_note(idx)

	control := `<form class="bw_form" method="get" action="/baro/category/tree">`
	control += tool.Baro_input("root", tool.Baro_t("시작 분류"), func() string {
		if root == "" {
			return ""
		}

		return baro_cat_disp(root)
	}(), tool.Baro_t("비우면 최상위 분류 전부"), tool.Baro_t("특정 분류부터 펼쳐 볼 수 있습니다."))
	control += tool.Baro_select("depth", tool.Baro_t("깊이 제한"), [][]string{
		{"2", tool.Baro_t("2단계")}, {"3", tool.Baro_t("3단계")}, {"4", tool.Baro_t("4단계")}, {"5", tool.Baro_t("5단계")}, {"6", tool.Baro_t("6단계")}, {"8", tool.Baro_t("8단계")},
	}, tool.Baro_itoa(max_depth), tool.Baro_t("트리가 크면 얕게 보는 편이 빠릅니다."))
	control += tool.Baro_submit(tool.Baro_t("적용"), "primary")
	control += `</form>`
	control += tool.Baro_t(`<p><button type="button" class="bw_btn" onclick="bw_cat_all(true)">모두 펼치기</button> `) +
		tool.Baro_t(`<button type="button" class="bw_btn" onclick="bw_cat_all(false)">모두 접기</button></p>`)

	body += tool.Baro_card(tool.Baro_t("보기 설정"), control)

	note := tool.Baro_t("＋ 를 눌러 하위 분류를 펼칩니다. 깊이 ") + tool.Baro_itoa(max_depth) + tool.Baro_t("단계까지만 그립니다.")
	if budget <= 0 {
		note += tool.Baro_t(" 트리가 매우 커서 일부만 그렸습니다.")
	}

	body += tool.Baro_card(tool.Baro_t("트리"), tool.Baro_note(note)+tree_html)

	if len(left_out) > 0 {
		items := []string{}
		for index, name := range left_out {
			if index >= 50 {
				break
			}

			items = append(items, baro_cat_cat_link(name))
		}

		body += tool.Baro_card(tool.Baro_t("트리에 나오지 않는 분류"),
			tool.Baro_alert("warn", tool.Baro_t("상위 분류를 따라 올라가도 최상위에 닿지 못하는 분류입니다. 대개 분류가 서로를 상위로 지정한 순환 때문입니다."))+
				tool.Baro_list(items)+
				tool.Baro_button("/baro/category/check", tool.Baro_t("분류 점검에서 순환 확인"), "primary"),
		)
	}

	return tool.Baro_page(db, config, tool.Baro_t("분류 트리"), body, baro_cat_menu())
}

// View_baro_cat_view : 분류별 최근 문서와 기여자 (/baro/category/view/<분류이름>)
func View_baro_cat_view(config tool.Config, name string, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	name = baro_cat_norm_cat(name)
	page_num := baro_cat_page_no(page)

	if name == "" {
		body := tool.Baro_hero(tool.Baro_t("분류 보기"), tool.Baro_t("분류 이름을 입력하세요."), "")
		body += baro_cat_tabs("/baro/category")
		body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/category", tool.Baro_t("분류 목록")}, {"", tool.Baro_t("분류 고르기")}})
		body += tool.Baro_card(tool.Baro_t("분류 고르기"), baro_cat_go_form("/baro/category/view/", "name", tool.Baro_t("분류 이름"), "", tool.Baro_t("분류 목록에서 고를 수도 있습니다.")))
		body += tool.Baro_button("/baro/category", tool.Baro_t("분류 목록으로"), "primary")

		return tool.Baro_page(db, config, tool.Baro_t("분류 보기"), body, baro_cat_menu())
	}

	idx := baro_cat_index_load(db)
	_, down := baro_cat_index_edges(idx)

	members := baro_cat_index_by_date(idx, idx.CatDocs[name])
	total := len(members)

	// 최근 편집순 문서 목록
	doc_rows := [][]string{}
	for _, title := range baro_cat_slice(members, page_num) {
		kind := tool.Baro_badge("문서", "info")
		if baro_cat_is_cat(title) {
			kind = tool.Baro_badge(tool.Baro_t("하위 분류"), "ok")
		}

		doc_rows = append(doc_rows, []string{
			tool.Baro_doc_link(title) + " " + kind,
			tool.Baro_time_cell(idx.Last[title]),
			tool.Baro_status_badge(tool.Baro_doc_status_get(db, title)),
			baro_cat_doc_tools(title),
		})
	}

	// 분류 기여자 (그 분류에 속한 문서를 편집한 사람)
	editors, edit_all := baro_cat_index_editors(db, members, 20)

	who_rows := [][]string{}
	for _, one := range editors {
		count := tool.Str_to_int(one[1])

		who_rows = append(who_rows, []string{
			baro_cat_who(db, one[0], config.IP),
			tool.Baro_comma(count),
			tool.Baro_pct_text(count, edit_all),
		})
	}

	up_list := idx.DocCats[name]
	down_list := down[name]
	exist := baro_cat_index_exists(db, idx, name)

	body := tool.Baro_hero(baro_cat_disp(name), tool.Baro_t("이 분류에 속한 문서와 기여자입니다."), "")
	body += baro_cat_tabs("/baro/category")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/category", tool.Baro_t("분류 목록")}, {"", baro_cat_disp(name)}})

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("소속 문서"), tool.Baro_comma(total), tool.Baro_t("하위 분류 포함")},
		{tool.Baro_t("하위 분류"), tool.Baro_comma(len(down_list)), tool.Baro_t("이 분류를 상위로 둔 분류")},
		{tool.Baro_t("상위 분류"), tool.Baro_comma(len(up_list)), tool.Baro_t("이 분류가 속한 분류")},
		{tool.Baro_t("마지막 변경"), tool.Baro_time_ago(baro_cat_index_cat_last(idx, name)), tool.Baro_t("소속 문서 기준")},
	})

	body += baro_cat_index_note(idx)

	if !exist {
		body += tool.Baro_alert("warn", tool.Baro_t("분류 문서가 아직 없습니다. 문서를 만들어 이 분류가 무엇을 모으는지 설명해 주세요."))
		body += tool.Baro_button("/edit/"+tool.Url_parser(name), "분류 문서 만들기", "primary")
	}

	head := ""
	head += tool.Baro_button("/w/"+tool.Url_parser(name), "분류 문서 보기", "")
	head += " " + tool.Baro_button("/baro/category/tree?root="+tool.Url_parser(name), "이 분류부터 트리 보기", "")
	head += " " + tool.Baro_button("/baro/link/doc/"+tool.Url_parser(name), "연결 관계 보기", "")
	body += tool.Baro_card(tool.Baro_t("바로 가기"), head)

	if len(up_list) > 0 {
		items := []string{}
		for _, parent := range up_list {
			items = append(items, baro_cat_cat_link(parent))
		}

		body += tool.Baro_card(tool.Baro_t("상위 분류"), tool.Baro_list(items))
	}

	if len(down_list) > 0 {
		items := []string{}
		for index, kid := range down_list {
			if index >= 100 {
				break
			}

			items = append(items, baro_cat_cat_link(kid))
		}

		body += tool.Baro_card(tool.Baro_t("하위 분류"), tool.Baro_list(items))
	}

	doc_html := tool.Baro_table(
		[]string{"문서", tool.Baro_t("마지막 편집"), tool.Baro_t("문서 상태"), tool.Baro_t("도구")},
		doc_rows,
		"",
	)
	if len(doc_rows) == 0 {
		doc_html = tool.Baro_empty(tool.Baro_t("이 분류에 속한 문서가 없습니다. 문서 맨 아래에 [[분류:") + baro_cat_disp(name) + tool.Baro_t("]] 를 달면 여기에 나타납니다."))
	}
	doc_html += tool.Baro_paging(page_num, len(doc_rows), baro_cat_page_size, "/baro/category/view/"+tool.Url_parser(name)+"?page={}")

	body += tool.Baro_card(tool.Baro_t("최근 편집된 문서"), doc_html)

	who_html := tool.Baro_table([]string{tool.Baro_t("기여자"), tool.Baro_t("이 분류 편집 수"), tool.Baro_t("비중")}, who_rows, "")
	if len(who_rows) == 0 {
		who_html = tool.Baro_empty(tool.Baro_t("아직 이 분류의 문서를 편집한 기록이 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("분류 기여자"),
		tool.Baro_note(tool.Baro_t("이 분류에 속한 문서를 편집한 횟수를 모아 보여 줍니다. 횟수는 순위가 아니라 참고 값입니다. ")+
			tool.Baro_t("비중은 이 분류의 전체 편집 ")+tool.Baro_comma(edit_all)+tool.Baro_t("회 가운데 차지하는 몫입니다."))+who_html)

	return tool.Baro_page(db, config, baro_cat_disp(name), body, baro_cat_menu())
}

// baro_cat_stats_build : 분류 통계 집계 (저장은 하지 않는다)
func baro_cat_stats_build(idx *baro_cat_index) map[string]string {
	_, counts := baro_cat_index_cats(idx)

	single := []string{}
	empty := []string{}
	top := [][]string{}

	for name, count := range counts {
		switch {
		case count == 0:
			empty = append(empty, name)
		case count == 1:
			single = append(single, name)
		}

		top = append(top, []string{name, tool.Baro_itoa(count)})
	}

	sort.Strings(single)
	sort.Strings(empty)
	sort.SliceStable(top, func(i int, j int) bool {
		if tool.Str_to_int(top[i][1]) != tool.Str_to_int(top[j][1]) {
			return tool.Str_to_int(top[i][1]) > tool.Str_to_int(top[j][1])
		}

		return top[i][0] < top[j][0]
	})

	top_line := []string{}
	for index, row := range top {
		if index >= 30 {
			break
		}

		top_line = append(top_line, row[0]+"\t"+row[1])
	}

	no_cat := 0
	for _, title := range idx.Docs {
		if baro_cat_is_cat(title) {
			continue
		}

		if len(idx.DocCats[title]) == 0 {
			no_cat++
		}
	}

	return map[string]string{
		"cat_all":    tool.Baro_itoa(len(counts)),
		"single":     tool.Baro_itoa(len(single)),
		"empty":      tool.Baro_itoa(len(empty)),
		"no_cat":     tool.Baro_itoa(no_cat),
		"doc_all":    tool.Baro_itoa(idx.DocAll),
		"top_list":   strings.Join(top_line, "\n"),
		"single_lst": strings.Join(baro_cat_head(single, 50), "\n"),
		"empty_lst":  strings.Join(baro_cat_head(empty, 50), "\n"),
	}
}

// baro_cat_head : 목록 앞부분만 자르기
func baro_cat_head(list []string, max int) []string {
	if len(list) > max {
		return list[:max]
	}

	return list
}

// baro_cat_lines : 캐시에 줄 단위로 저장한 값을 다시 목록으로
func baro_cat_lines(data string) []string {
	out := []string{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		out = append(out, line)
	}

	return out
}

// View_baro_cat_stats : 분류 통계 (/baro/category/stats)
func View_baro_cat_stats(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if values != nil {
		if deny, ok := baro_cat_recalc_guard(db, config, "/baro/category/stats"); !ok {
			return deny
		}

		detail := baro_cat_stats_build(baro_cat_index_load(db))
		baro_cat_cache_put(db, baro_cat_stats_key, tool.Str_to_int(detail["cat_all"]), detail)
		baro_cat_recalc_log(db, config, tool.Baro_t("분류 통계"), "분류 "+detail["cat_all"]+tool.Baro_t("개 재집계"))

		return tool.Get_redirect("/baro/category/stats")
	}

	// 저장된 집계가 없으면 지금 즉석에서 센다 (보기만 하는 화면이므로 저장하지는 않는다)
	detail, date := baro_cat_cache_get(db, baro_cat_stats_key)

	var idx *baro_cat_index
	if date == "" {
		idx = baro_cat_index_load(db)
		detail = baro_cat_stats_build(idx)
	}

	cat_all := baro_cat_cache_num(detail, "cat_all")
	doc_all := baro_cat_cache_num(detail, "doc_all")
	no_cat := baro_cat_cache_num(detail, "no_cat")

	top_rows := [][]string{}
	for _, line := range baro_cat_lines(detail["top_list"]) {
		part := strings.SplitN(line, "\t", 2)
		if len(part) < 2 {
			continue
		}

		top_rows = append(top_rows, []string{
			baro_cat_cat_link(part[0]),
			tool.Baro_comma(tool.Str_to_int(part[1])),
			tool.Baro_pct_text(tool.Str_to_int(part[1]), doc_all),
		})
	}

	single_items := []string{}
	for _, name := range baro_cat_lines(detail["single_lst"]) {
		single_items = append(single_items, baro_cat_cat_link(name))
	}

	empty_items := []string{}
	for _, name := range baro_cat_lines(detail["empty_lst"]) {
		empty_items = append(empty_items, baro_cat_cat_link(name)+
			` <a class="bw_btn" href="/w/`+tool.Url_parser(name)+tool.Baro_t(`">분류 문서</a>`))
	}

	body := tool.Baro_hero(tool.Baro_t("분류 통계"), tool.Baro_t("분류가 고르게 쓰이고 있는지 살펴봅니다."), "")
	body += baro_cat_tabs("/baro/category/stats")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/category", tool.Baro_t("분류 도구")}, {"", tool.Baro_t("분류 통계")}})

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("분류 수"), tool.Baro_comma(cat_all), tool.Baro_t("이름이 확인된 분류")},
		{tool.Baro_t("문서 수"), tool.Baro_comma(doc_all), tool.Baro_t("휴지통 제외")},
		{tool.Baro_t("분류 없는 문서"), tool.Baro_comma(no_cat), tool.Baro_pct_text(no_cat, doc_all) + tool.Baro_t(" 가 분류 미지정")},
		{tool.Baro_t("문서 하나뿐인 분류"), tool.Baro_comma(baro_cat_cache_num(detail, "single")), tool.Baro_t("합치거나 넓힐 후보")},
	})

	body += tool.Baro_progress(doc_all-no_cat, doc_all, tool.Baro_t("분류가 달린 문서 비율"))

	if idx != nil {
		body += baro_cat_index_note(idx)
	}

	body += baro_cat_recalc_box(db, config, "/baro/category/stats", date)

	body += tool.Baro_card(tool.Baro_t("문서 수 상위 분류"),
		tool.Baro_table([]string{"분류", tool.Baro_t("문서 수"), tool.Baro_t("전체 문서 대비")}, top_rows, tool.Baro_t("아직 분류가 달린 문서가 없습니다.")))

	single_body := tool.Baro_note(tool.Baro_t("문서가 하나뿐인 분류입니다. 비슷한 분류로 합치거나, 같은 분류에 들어갈 문서를 더 만들면 좋습니다."))
	if len(single_items) == 0 {
		single_body += tool.Baro_empty(tool.Baro_t("문서가 하나뿐인 분류가 없습니다."))
	} else {
		single_body += tool.Baro_list(single_items)
	}
	body += tool.Baro_card(tool.Baro_t("문서가 하나뿐인 분류 (")+tool.Baro_comma(baro_cat_cache_num(detail, "single"))+tool.Baro_t("개)"), single_body)

	empty_body := tool.Baro_note(tool.Baro_t("분류 문서는 있는데 속한 문서가 하나도 없습니다. 문서를 채우거나 분류를 정리하세요."))
	if len(empty_items) == 0 {
		empty_body += tool.Baro_empty(tool.Baro_t("비어 있는 분류가 없습니다."))
	} else {
		empty_body += tool.Baro_list(empty_items)
	}
	body += tool.Baro_card(tool.Baro_t("비어 있는 분류 (")+tool.Baro_comma(baro_cat_cache_num(detail, "empty"))+tool.Baro_t("개)"), empty_body)

	body += tool.Baro_card(tool.Baro_t("분류가 하나도 없는 문서"),
		tool.Baro_note(tool.Baro_t("전체 ")+tool.Baro_comma(doc_all)+tool.Baro_t("개 문서 가운데 ")+tool.Baro_comma(no_cat)+tool.Baro_t("개에 분류가 없습니다."))+
			tool.Baro_button("/baro/category/nocategory", tool.Baro_t("분류 없는 문서 보기"), "primary"))

	return tool.Baro_page(db, config, tool.Baro_t("분류 통계"), body, baro_cat_menu())
}

// baro_cat_dup_build : 한 문서에 같은 분류가 두 번 달린 경우를 본문에서 찾아낸다
//
//	back 표는 같은 분류를 한 번만 담기 때문에 중복은 본문을 읽어야만 알 수 있다.
func baro_cat_dup_build(db *sql.DB) map[string]string {
	lines := []string{}
	scanned := 0

	rows := tool.Query_DB(
		db,
		"select d.title, d.data, d.type from data d where"+baro_cat_real_d_sql+"order by d.title asc limit ?",
		baro_cat_text_scan_max,
	)
	defer rows.Close()

	for rows.Next() {
		title, data, markup := "", "", ""
		if rows.Scan(&title, &data, &markup) != nil {
			continue
		}

		scanned++

		if markup == "raw" {
			continue
		}

		count := map[string]int{}
		order := []string{}
		for _, name := range baro_cat_scan_categories(data) {
			key := strings.ToLower(name)
			if count[key] == 0 {
				order = append(order, name)
			}
			count[key]++
		}

		for _, name := range order {
			num := count[strings.ToLower(name)]
			if num < 2 {
				continue
			}

			lines = append(lines, title+"\t"+name+"\t"+tool.Baro_itoa(num))
		}

		if len(lines) >= 200 {
			break
		}
	}

	return map[string]string{
		"num":     tool.Baro_itoa(len(lines)),
		"scanned": tool.Baro_itoa(scanned),
		"list":    strings.Join(lines, "\n"),
	}
}

// baro_cat_cycles : 분류 순환 찾기 (깊이 우선 탐색)
func baro_cat_cycles(idx *baro_cat_index) [][]string {
	up, _ := baro_cat_index_edges(idx)

	names := make([]string, 0, len(up))
	for name := range up {
		names = append(names, name)
	}
	sort.Strings(names)

	state := map[string]int{} // 0 안 봄 / 1 보는 중 / 2 끝
	stack := []string{}
	found := [][]string{}
	seen := map[string]bool{}

	var walk func(name string)
	walk = func(name string) {
		if len(found) >= 50 {
			return
		}

		if state[name] == 1 {
			// 순환 발견 : 스택에서 name 이 처음 나온 곳부터가 고리다
			start := 0
			for index, item := range stack {
				if item == name {
					start = index
					break
				}
			}

			cycle := append([]string{}, stack[start:]...)
			cycle = append(cycle, name)

			key := strings.Join(baro_cat_cycle_key(cycle), ">")
			if !seen[key] {
				seen[key] = true
				found = append(found, cycle)
			}

			return
		}

		if state[name] == 2 {
			return
		}

		state[name] = 1
		stack = append(stack, name)

		for _, parent := range up[name] {
			walk(parent)
		}

		stack = stack[:len(stack)-1]
		state[name] = 2
	}

	for _, name := range names {
		walk(name)
	}

	return found
}

// baro_cat_cycle_key : 같은 고리를 두 번 세지 않기 위한 정렬 키
func baro_cat_cycle_key(cycle []string) []string {
	out := append([]string{}, cycle...)
	sort.Strings(out)

	return out
}

// View_baro_cat_check : 분류 중복·순환·없는 분류 점검 (/baro/category/check)
func View_baro_cat_check(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if values != nil {
		if deny, ok := baro_cat_recalc_guard(db, config, "/baro/category/check"); !ok {
			return deny
		}

		detail := baro_cat_dup_build(db)
		baro_cat_cache_put(db, baro_cat_dup_key, tool.Str_to_int(detail["num"]), detail)
		baro_cat_recalc_log(db, config, tool.Baro_t("분류 중복 검사"), tool.Baro_t("중복 ")+detail["num"]+tool.Baro_t("건 발견"))

		return tool.Get_redirect("/baro/category/check")
	}

	idx := baro_cat_index_load(db)

	detail, date := baro_cat_cache_get(db, baro_cat_dup_key)
	if date == "" {
		detail = baro_cat_dup_build(db)
	}

	// 1) 중복 분류
	dup_rows := [][]string{}
	for _, line := range baro_cat_lines(detail["list"]) {
		part := strings.SplitN(line, "\t", 3)
		if len(part) < 3 {
			continue
		}

		dup_rows = append(dup_rows, []string{
			tool.Baro_doc_link(part[0]),
			baro_cat_cat_link(part[1]),
			tool.Baro_comma(tool.Str_to_int(part[2])) + tool.Baro_t("번"),
			`<a class="bw_btn" href="/edit/` + tool.Url_parser(part[0]) + tool.Baro_t(`">고치기</a>`),
		})
	}

	// 2) 순환
	cycle_items := []string{}
	for _, cycle := range baro_cat_cycles(idx) {
		parts := []string{}
		for _, name := range cycle {
			parts = append(parts, baro_cat_cat_link(name))
		}

		cycle_items = append(cycle_items, strings.Join(parts, ` <span class="bw_crumb_s">→</span> `)+
			` <a class="bw_btn" href="/edit/`+tool.Url_parser(cycle[0])+tool.Baro_t(`">고리 끊기</a>`))
	}

	// 3) 존재하지 않는 분류 (분류 문서가 없는 분류)
	ghost := [][]string{}
	for name, list := range idx.CatDocs {
		if baro_cat_index_exists(db, idx, name) {
			continue
		}

		ghost = append(ghost, []string{name, tool.Baro_itoa(len(list))})
	}
	sort.SliceStable(ghost, func(i int, j int) bool {
		if tool.Str_to_int(ghost[i][1]) != tool.Str_to_int(ghost[j][1]) {
			return tool.Str_to_int(ghost[i][1]) > tool.Str_to_int(ghost[j][1])
		}

		return ghost[i][0] < ghost[j][0]
	})

	ghost_rows := [][]string{}
	for index, row := range ghost {
		if index >= 100 {
			break
		}

		ghost_rows = append(ghost_rows, []string{
			baro_cat_cat_link(row[0]),
			tool.Baro_comma(tool.Str_to_int(row[1])),
			`<a class="bw_btn" href="/edit/` + tool.Url_parser(row[0]) + tool.Baro_t(`">분류 문서 만들기</a> `) +
				`<a class="bw_btn" href="/baro/category/view/` + tool.Url_parser(row[0]) + tool.Baro_t(`">문서 보기</a>`),
		})
	}

	body := tool.Baro_hero(tool.Baro_t("분류 점검"), tool.Baro_t("잘못 달린 분류를 찾아 고칠 곳을 알려 줍니다."), "")
	body += baro_cat_tabs("/baro/category/check")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/category", tool.Baro_t("분류 도구")}, {"", tool.Baro_t("분류 점검")}})

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("중복 분류"), tool.Baro_comma(len(dup_rows)), tool.Baro_t("같은 분류가 두 번")},
		{tool.Baro_t("순환"), tool.Baro_comma(len(cycle_items)), tool.Baro_t("서로가 서로의 하위")},
		{tool.Baro_t("없는 분류"), tool.Baro_comma(len(ghost)), tool.Baro_t("분류 문서가 없음")},
		{tool.Baro_t("검사한 문서"), tool.Baro_comma(baro_cat_cache_num(detail, "scanned")), tool.Baro_t("본문 검사 기준")},
	})

	body += baro_cat_index_note(idx)
	body += baro_cat_recalc_box(db, config, "/baro/category/check", date)

	dup_body := tool.Baro_note(tool.Baro_t("한 문서에 같은 분류가 두 번 이상 달려 있습니다. 문서를 열어 중복된 [[분류:...]] 한 줄을 지우면 됩니다."))
	if len(dup_rows) == 0 {
		dup_body += tool.Baro_empty(tool.Baro_t("중복으로 달린 분류가 없습니다."))
	} else {
		dup_body += tool.Baro_table([]string{"문서", "분류", tool.Baro_t("달린 횟수"), tool.Baro_t("할 일")}, dup_rows, "")
	}
	body += tool.Baro_card(tool.Baro_t("1. 중복 분류"), dup_body)

	cycle_body := tool.Baro_note(tool.Baro_t("분류가 서로를 상위 분류로 지정하고 있습니다. 고리 안에서 한쪽의 상위 분류 지정을 지우면 풀립니다."))
	if len(cycle_items) == 0 {
		cycle_body += tool.Baro_empty(tool.Baro_t("순환하는 분류가 없습니다."))
	} else {
		cycle_body += tool.Baro_list(cycle_items)
	}
	body += tool.Baro_card(tool.Baro_t("2. 분류 순환"), cycle_body)

	ghost_body := tool.Baro_note(tool.Baro_t("문서에는 달려 있지만 분류 문서 자체가 없습니다. 분류 문서를 만들어 설명을 적거나, 문서에서 오타 난 분류를 고치세요."))
	if len(ghost_rows) == 0 {
		ghost_body += tool.Baro_empty(tool.Baro_t("존재하지 않는 분류에 달린 문서가 없습니다."))
	} else {
		ghost_body += tool.Baro_table([]string{"분류", tool.Baro_t("달린 문서 수"), tool.Baro_t("할 일")}, ghost_rows, "")
	}
	body += tool.Baro_card(tool.Baro_t("3. 존재하지 않는 분류"), ghost_body)

	return tool.Baro_page(db, config, tool.Baro_t("분류 점검"), body, baro_cat_menu())
}

// View_baro_cat_nocategory : 분류가 없는 문서 (/baro/category/nocategory)
func View_baro_cat_nocategory(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	page_num := baro_cat_page_no(page)
	idx := baro_cat_index_load(db)

	list := []string{}
	for _, title := range idx.Docs {
		if baro_cat_is_cat(title) {
			continue
		}

		if len(idx.DocCats[title]) == 0 {
			list = append(list, title)
		}
	}

	rows := [][]string{}
	for _, title := range baro_cat_slice(list, page_num) {
		rows = append(rows, []string{
			tool.Baro_doc_link(title),
			tool.Baro_time_cell(idx.Last[title]),
			`<a class="bw_btn bw_btn_primary" href="/edit/` + tool.Url_parser(title) + tool.Baro_t(`">분류 달기</a> `) +
				`<a class="bw_btn" href="/baro/task/new?title=` + tool.Url_parser(title) +
				tool.Baro_t(`&kind=category">과제로 등록</a> `) +
				`<a class="bw_btn" href="/baro/link/doc/` + tool.Url_parser(title) + tool.Baro_t(`">연결</a>`),
		})
	}

	body := tool.Baro_hero(tool.Baro_t("분류 없는 문서"), tool.Baro_t("분류를 달아 주면 문서를 찾기 쉬워집니다."), "")
	body += baro_cat_tabs("/baro/category/nocategory")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/category", tool.Baro_t("분류 도구")}, {"", tool.Baro_t("분류 없는 문서")}})

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("분류 없는 문서"), tool.Baro_comma(len(list)), tool.Baro_t("휴지통과 분류 문서 제외")},
		{tool.Baro_t("전체 문서"), tool.Baro_comma(idx.DocAll), tool.Baro_t("휴지통 제외")},
		{tool.Baro_t("이 쪽"), tool.Baro_comma(len(rows)), tool.Baro_itoa(page_num) + tool.Baro_t("쪽")},
	})

	body += baro_cat_index_note(idx)

	body += tool.Baro_alert("info", tool.Baro_t("문서 맨 아래에 [[분류:이름]] 을 한 줄 넣으면 됩니다. 어떤 분류가 있는지는 분류 목록에서 확인하세요."))

	table := tool.Baro_table([]string{"문서", tool.Baro_t("마지막 편집"), tool.Baro_t("할 일")}, rows, "")
	if len(rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("이 쪽에는 분류가 없는 문서가 없습니다. 모든 문서에 분류가 달렸다면 훌륭합니다."))
	}
	table += tool.Baro_paging(page_num, len(rows), baro_cat_page_size, "/baro/category/nocategory?page={}")

	body += tool.Baro_card(tool.Baro_t("목록"), table)

	return tool.Baro_page(db, config, tool.Baro_t("분류 없는 문서"), body, baro_cat_menu())
}
