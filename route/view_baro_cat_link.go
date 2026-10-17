package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  3-14 역링크 · 연결 문서 도구 — /baro/link
//
//  본체의 /xref 는 "이 문서를 링크한 문서" 와 "이 문서가 링크한 문서" 를
//  한 번에 한쪽씩만 보여 준다. 여기서는 양쪽을 한 화면에 놓고,
//  위키 전체의 연결 상태(고립·끊어진 링크·막다른 문서)를 함께 살핀다.
// ============================================================

// baro_cat_link_key : 연결 상태 집계 캐시 키
const baro_cat_link_key = "baro_cat_link"

// baro_cat_link_type_name : 링크 종류를 사람이 읽는 이름으로
func baro_cat_link_type_name(kind string) string {
	switch kind {
	case "":
		return tool.Baro_t("일반 링크")
	case "cat":
		return "분류"
	case "include":
		return tool.Baro_t("포함")
	case "redirect":
		return tool.Baro_t("넘겨주기")
	case "file":
		return tool.Baro_t("파일")
	case "no":
		return tool.Baro_t("대상 없음")
	}

	return kind
}

// baro_cat_link_type_badge : 링크 종류 배지
func baro_cat_link_type_badge(kind string) string {
	style := "muted"
	switch kind {
	case "":
		style = "info"
	case "cat":
		style = "ok"
	case "redirect":
		style = "warn"
	case "no":
		style = "danger"
	}

	return tool.Baro_badge(baro_cat_link_type_name(kind), style)
}

// baro_cat_broken : 끊어진 링크 한 줄 (없는 문서 하나)
type baro_cat_broken struct {
	Name string
	Num  int
	From []string
}

// baro_cat_link_orphans : 고립 문서 (아무 문서도 링크하지 않는 문서)
func baro_cat_link_orphans(idx *baro_cat_index) []string {
	out := []string{}
	for _, title := range idx.Docs {
		if len(idx.In[title]) == 0 {
			out = append(out, title)
		}
	}

	return out
}

// baro_cat_link_deadends : 막다른 문서 (밖으로 나가는 링크가 없는 문서)
func baro_cat_link_deadends(idx *baro_cat_index) []string {
	out := []string{}
	for _, title := range idx.Docs {
		if len(baro_cat_index_out_real(idx, title)) == 0 {
			out = append(out, title)
		}
	}

	return out
}

// baro_cat_link_broken_list : 끊어진 링크 (요청이 많은 없는 문서부터)
func baro_cat_link_broken_list(db *sql.DB, idx *baro_cat_index) []baro_cat_broken {
	seen := map[string]bool{}
	order := []string{}
	from := map[string][]string{}

	names := make([]string, 0, len(idx.Out))
	for name := range idx.Out {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, source := range names {
		for _, one := range idx.Out[source] {
			if baro_cat_index_exists(db, idx, one.Name) {
				continue
			}

			key := one.Name + "\n" + source
			if seen[key] {
				continue
			}
			seen[key] = true

			if _, ok := from[one.Name]; !ok {
				order = append(order, one.Name)
			}

			from[one.Name] = append(from[one.Name], source)
		}
	}

	out := make([]baro_cat_broken, 0, len(order))
	for _, name := range order {
		out = append(out, baro_cat_broken{Name: name, Num: len(from[name]), From: from[name]})
	}

	sort.SliceStable(out, func(i int, j int) bool {
		if out[i].Num != out[j].Num {
			return out[i].Num > out[j].Num
		}

		return out[i].Name < out[j].Name
	})

	return out
}

// baro_cat_link_build : 연결 상태 집계 (저장은 하지 않는다)
func baro_cat_link_build(db *sql.DB, idx *baro_cat_index) map[string]string {
	broken := baro_cat_link_broken_list(db, idx)

	hit := 0
	for _, one := range broken {
		hit += one.Num
	}

	return map[string]string{
		"doc_all":    tool.Baro_itoa(idx.DocAll),
		"orphan":     tool.Baro_itoa(len(baro_cat_link_orphans(idx))),
		"dead":       tool.Baro_itoa(len(baro_cat_link_deadends(idx))),
		"broken":     tool.Baro_itoa(len(broken)),
		"broken_hit": tool.Baro_itoa(hit),
	}
}

// View_baro_cat_link_home : 연결 요약 대시보드 (/baro/link)
func View_baro_cat_link_home(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if values != nil {
		if deny, ok := baro_cat_recalc_guard(db, config, "/baro/link"); !ok {
			return deny
		}

		detail := baro_cat_link_build(db, baro_cat_index_load(db))
		baro_cat_cache_put(db, baro_cat_link_key,
			tool.Str_to_int(detail["orphan"])+tool.Str_to_int(detail["dead"])+tool.Str_to_int(detail["broken"]), detail)
		baro_cat_recalc_log(db, config, tool.Baro_t("연결 상태"),
			tool.Baro_t("고립 ")+detail["orphan"]+tool.Baro_t(" · 끊어진 링크 ")+detail["broken"]+tool.Baro_t(" · 막다른 ")+detail["dead"])

		return tool.Get_redirect("/baro/link")
	}

	// 저장된 집계가 없으면 지금 즉석에서 센다 (보기만 하는 화면이므로 저장하지는 않는다)
	detail, date := baro_cat_cache_get(db, baro_cat_link_key)

	var idx *baro_cat_index
	if date == "" {
		idx = baro_cat_index_load(db)
		detail = baro_cat_link_build(db, idx)
	}

	doc_all := baro_cat_cache_num(detail, "doc_all")
	orphan := baro_cat_cache_num(detail, "orphan")
	dead := baro_cat_cache_num(detail, "dead")
	broken := baro_cat_cache_num(detail, "broken")
	broken_hit := baro_cat_cache_num(detail, "broken_hit")

	body := tool.Baro_hero(
		tool.Baro_t("연결 문서 도구"),
		tool.Baro_t("문서가 서로 얼마나 이어져 있는지 보고, 끊어진 곳을 찾아 고칩니다."),
		"",
	)
	body += baro_cat_link_tabs("/baro/link")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("연결 요약")}})

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 문서"), tool.Baro_comma(doc_all), tool.Baro_t("휴지통 제외")},
		{tool.Baro_t("고립 문서"), tool.Baro_comma(orphan), tool.Baro_t("아무도 링크하지 않음")},
		{tool.Baro_t("끊어진 링크"), tool.Baro_comma(broken), tool.Baro_t("없는 문서 ") + tool.Baro_comma(broken) + tool.Baro_t("개 / 요청 ") + tool.Baro_comma(broken_hit) + tool.Baro_t("회")},
		{tool.Baro_t("막다른 문서"), tool.Baro_comma(dead), tool.Baro_t("밖으로 나가는 링크 없음")},
	})

	body += tool.Baro_progress(doc_all-orphan, doc_all, tool.Baro_t("다른 문서에서 이어지는 문서 비율"))

	if idx != nil {
		body += baro_cat_index_note(idx)
	}

	body += baro_cat_recalc_box(db, config, "/baro/link", date)

	body += tool.Baro_card(tool.Baro_t("문서 하나의 연결 관계 보기"),
		tool.Baro_note(tool.Baro_t("이 문서를 링크한 문서와 이 문서가 링크한 문서를 한 화면에서 봅니다."))+
			baro_cat_go_form("/baro/link/doc/", "name", tool.Baro_t("문서 이름"), "", tool.Baro_t("본체의 역링크 화면과 달리 양쪽을 함께 보여 줍니다.")))

	cards := []string{}

	cards = append(cards, tool.Baro_card_ex(
		tool.Baro_t("고립 문서"),
		tool.Baro_comma(orphan)+tool.Baro_t("건"),
		tool.Baro_t("<p>아무 문서도 링크하지 않는 문서입니다. 관련 문서에서 링크를 걸어 주면 찾아가기 쉬워집니다.</p>"),
		tool.Baro_button("/baro/link/orphan", tool.Baro_t("목록 보기"), "primary"),
	))

	cards = append(cards, tool.Baro_card_ex(
		tool.Baro_t("끊어진 링크"),
		tool.Baro_comma(broken)+tool.Baro_t("건"),
		tool.Baro_t("<p>존재하지 않는 문서를 가리키는 링크입니다. 많이 요청된 문서부터 만들면 가장 도움이 됩니다.</p>"),
		tool.Baro_button("/baro/link/broken", tool.Baro_t("목록 보기"), "primary"),
	))

	cards = append(cards, tool.Baro_card_ex(
		tool.Baro_t("막다른 문서"),
		tool.Baro_comma(dead)+tool.Baro_t("건"),
		tool.Baro_t("<p>밖으로 나가는 링크가 하나도 없는 문서입니다. 본문 속 낱말을 링크로 만들어 주세요.</p>"),
		tool.Baro_button("/baro/link/deadend", tool.Baro_t("목록 보기"), "primary"),
	))

	cards = append(cards, tool.Baro_card_ex(
		tool.Baro_t("분류 도구"),
		"",
		tool.Baro_t("<p>연결 못지않게 분류도 문서를 찾는 길입니다. 분류가 없는 문서를 함께 살펴보세요.</p>"),
		tool.Baro_button("/baro/category", tool.Baro_t("분류 도구로"), ""),
	))

	body += tool.Baro_grid(cards, 2)

	return tool.Baro_page(db, config, tool.Baro_t("연결 요약"), body, baro_cat_menu())
}

// baro_cat_link_rows : 링크 목록 한 쪽을 표로
func baro_cat_link_rows(db *sql.DB, idx *baro_cat_index, list []baro_cat_pair) [][]string {
	out := [][]string{}

	for _, one := range list {
		exist := tool.Baro_badge(tool.Baro_t("있음"), "ok")
		if !baro_cat_index_exists(db, idx, one.Name) {
			exist = tool.Baro_badge("없음", "danger")
		}

		out = append(out, []string{
			tool.Baro_doc_link(one.Name),
			baro_cat_link_type_badge(one.Kind),
			exist,
			`<a class="bw_btn" href="/baro/link/doc/` + tool.Url_parser(one.Name) + tool.Baro_t(`">연결</a>`),
		})
	}

	return out
}

// baro_cat_link_related : 관련 문서 추천
//
//	점수 = 공통 분류 수 * 2 + 공통 역링크 수
func baro_cat_link_related(db *sql.DB, idx *baro_cat_index, name string) [][]string {
	score := map[string]int{}
	same_cat := map[string]int{}
	same_back := map[string]int{}

	// 1) 같은 분류에 속한 문서
	for index, cat := range idx.DocCats[name] {
		if index >= 20 {
			break
		}

		for count, mate := range idx.CatDocs[cat] {
			if count >= 300 {
				break
			}

			if mate == "" || mate == name {
				continue
			}

			same_cat[mate]++
			score[mate] += 2
		}
	}

	// 2) 나를 링크한 문서가 함께 링크한 문서 (공통 역링크)
	done := map[string]bool{}
	for index, one := range idx.In[name] {
		if index >= 100 {
			break
		}

		if done[one.Name] {
			continue
		}
		done[one.Name] = true

		for count, two := range baro_cat_index_out_real(idx, one.Name) {
			if count >= 200 {
				break
			}

			if two.Name == "" || two.Name == name {
				continue
			}

			same_back[two.Name]++
			score[two.Name]++
		}
	}

	names := make([]string, 0, len(score))
	for mate := range score {
		names = append(names, mate)
	}

	sort.SliceStable(names, func(i int, j int) bool {
		if score[names[i]] != score[names[j]] {
			return score[names[i]] > score[names[j]]
		}

		return names[i] < names[j]
	})

	out := [][]string{}
	for _, mate := range names {
		if len(out) >= 10 {
			break
		}

		if !baro_cat_index_exists(db, idx, mate) {
			continue
		}

		out = append(out, []string{
			tool.Baro_doc_link(mate),
			tool.Baro_comma(score[mate]),
			tool.Baro_comma(same_cat[mate]),
			tool.Baro_comma(same_back[mate]),
			`<a class="bw_btn" href="/baro/link/doc/` + tool.Url_parser(mate) + tool.Baro_t(`">연결</a>`),
		})
	}

	return out
}

// View_baro_cat_link_doc : 문서 하나의 양방향 연결 (/baro/link/doc/<문서이름>)
func View_baro_cat_link_doc(config tool.Config, name string, in_page string, out_page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	name = tool.Namespace_normalize(strings.TrimSpace(name))

	if name == "" {
		body := tool.Baro_hero("연결 관계 보기", tool.Baro_t("문서 이름을 입력하세요."), "")
		body += baro_cat_link_tabs("/baro/link")
		body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/link", tool.Baro_t("연결 도구")}, {"", tool.Baro_t("문서 고르기")}})
		body += tool.Baro_card(tool.Baro_t("문서 고르기"), baro_cat_go_form("/baro/link/doc/", "name", tool.Baro_t("문서 이름"), "", tool.Baro_t("예 : 대한민국")))
		body += tool.Baro_button("/baro/link", tool.Baro_t("연결 요약으로"), "primary")

		return tool.Baro_page(db, config, "연결 관계 보기", body, baro_cat_menu())
	}

	in_num := baro_cat_page_no(in_page)
	out_num := baro_cat_page_no(out_page)

	idx := baro_cat_index_load(db)

	in_list := idx.In[name]
	out_list := idx.Out[name]

	in_rows := baro_cat_link_rows(db, idx, baro_cat_slice_pair(in_list, in_num))
	out_rows := baro_cat_link_rows(db, idx, baro_cat_slice_pair(out_list, out_num))

	// 들어오고 나가는 문서 수 (같은 문서에 종류가 여럿이면 하나로 센다)
	in_all := baro_cat_link_uniq(in_list)
	out_all := baro_cat_link_uniq(out_list)

	broken_out := []string{}
	for _, one := range out_list {
		if baro_cat_index_exists(db, idx, one.Name) {
			continue
		}

		broken_out = append(broken_out, one.Name)
		if len(broken_out) >= 100 {
			break
		}
	}

	exist := baro_cat_index_exists(db, idx, name)
	cats := idx.DocCats[name]

	body := tool.Baro_hero(baro_cat_disp(name), tool.Baro_t("이 문서의 들어오는 링크와 나가는 링크를 함께 봅니다."), "")
	body += baro_cat_link_tabs("/baro/link")
	body += tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/link", tool.Baro_t("연결 도구")},
		{"", baro_cat_disp(name)},
	})

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("들어오는 링크"), tool.Baro_comma(in_all), tool.Baro_t("이 문서를 링크한 문서")},
		{tool.Baro_t("나가는 링크"), tool.Baro_comma(out_all), tool.Baro_t("이 문서가 링크한 문서")},
		{tool.Baro_t("끊어진 나가는 링크"), tool.Baro_comma(len(broken_out)), tool.Baro_t("없는 문서를 가리킴")},
		{"분류", tool.Baro_comma(len(cats)), tool.Baro_t("이 문서에 달린 분류")},
	})

	body += baro_cat_index_note(idx)

	if !exist {
		body += tool.Baro_alert("warn", tool.Baro_t("이 이름의 문서가 아직 없습니다. 다른 문서가 이 이름을 링크하고 있다면 만들어 볼 만합니다."))
		body += tool.Baro_button("/edit/"+tool.Url_parser(name), "문서 만들기", "primary")
	}

	tools := tool.Baro_button("/w/" + tool.Doc_url(name), "문서 보기", "")
	tools += " " + tool.Baro_button("/xref/"+tool.Url_parser(name), "본체 역링크", "")
	tools += " " + tool.Baro_button("/edit/"+tool.Url_parser(name), "편집", "")
	if len(cats) > 0 {
		tools += " " + tool.Baro_button("/baro/category/view/"+tool.Url_parser(cats[0]), "대표 분류 보기", "")
	}
	body += tool.Baro_card(tool.Baro_t("바로 가기"), tools)

	if len(cats) > 0 {
		items := []string{}
		for _, cat := range cats {
			items = append(items, baro_cat_cat_link(cat))
		}

		body += tool.Baro_card(tool.Baro_t("이 문서의 분류"), tool.Baro_list(items))
	} else {
		body += tool.Baro_card(tool.Baro_t("이 문서의 분류"),
			tool.Baro_empty(tool.Baro_t("분류가 없습니다."))+
				tool.Baro_button("/baro/category", tool.Baro_t("어떤 분류가 있는지 보기"), ""))
	}

	in_html := tool.Baro_table([]string{"문서", tool.Baro_t("링크 종류"), tool.Baro_t("대상"), tool.Baro_t("도구")}, in_rows, "")
	if len(in_rows) == 0 {
		in_html = tool.Baro_empty(tool.Baro_t("이 문서를 링크한 문서가 없습니다. 관련 문서에서 링크를 걸어 주세요."))
	}
	in_html += tool.Baro_paging(in_num, len(in_rows), baro_cat_page_size,
		"/baro/link/doc/"+tool.Url_parser(name)+"?in_page={}&out_page="+tool.Baro_itoa(out_num))

	out_html := tool.Baro_table([]string{"문서", tool.Baro_t("링크 종류"), tool.Baro_t("대상"), tool.Baro_t("도구")}, out_rows, "")
	if len(out_rows) == 0 {
		out_html = tool.Baro_empty(tool.Baro_t("이 문서가 링크한 문서가 없습니다. 본문의 낱말을 링크로 만들어 보세요."))
	}
	out_html += tool.Baro_paging(out_num, len(out_rows), baro_cat_page_size,
		"/baro/link/doc/"+tool.Url_parser(name)+"?out_page={}&in_page="+tool.Baro_itoa(in_num))

	body += tool.Baro_grid([]string{
		tool.Baro_card(tool.Baro_t("이 문서를 링크한 문서"), in_html),
		tool.Baro_card(tool.Baro_t("이 문서가 링크한 문서"), out_html),
	}, 2)

	if len(broken_out) > 0 {
		items := []string{}
		for _, title := range broken_out {
			items = append(items, tool.HTML_escape(baro_cat_disp(title))+
				` <a class="bw_btn" href="/edit/`+tool.Url_parser(title)+tool.Baro_t(`">만들기</a>`)+
				` <a class="bw_btn" href="/baro/task/new?title=`+tool.Url_parser(title)+tool.Baro_t(`&kind=create">과제로 등록</a>`))
		}

		body += tool.Baro_card(tool.Baro_t("이 문서에서 나가는 끊어진 링크"),
			tool.Baro_alert("warn", tool.Baro_t("아래 문서들은 아직 없습니다. 만들거나, 링크를 지우거나, 다른 이름으로 고치세요."))+
				tool.Baro_list(items))
	}

	related := baro_cat_link_related(db, idx, name)
	related_html := tool.Baro_table(
		[]string{"문서", tool.Baro_t("점수"), tool.Baro_t("공통 분류"), tool.Baro_t("공통 역링크"), tool.Baro_t("도구")},
		related,
		"",
	)
	if len(related) == 0 {
		related_html = tool.Baro_empty(tool.Baro_t("추천할 만한 관련 문서를 찾지 못했습니다. 분류를 달거나 링크를 늘리면 추천이 생깁니다."))
	}

	body += tool.Baro_card(tool.Baro_t("관련 문서 추천"),
		tool.Baro_note(tool.Baro_t("점수 = 공통 분류 수 × 2 + 공통 역링크 수 로 매긴 상위 10개입니다."))+related_html)

	return tool.Baro_page(db, config, baro_cat_disp(name)+tool.Baro_t(" 연결"), body, baro_cat_menu())
}

// baro_cat_link_uniq : 링크 목록에 나오는 문서 수 (종류가 여럿이면 하나로)
func baro_cat_link_uniq(list []baro_cat_pair) int {
	seen := map[string]bool{}
	for _, one := range list {
		seen[one.Name] = true
	}

	return len(seen)
}

// View_baro_cat_link_orphan : 고립 문서 (/baro/link/orphan)
func View_baro_cat_link_orphan(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	page_num := baro_cat_page_no(page)
	idx := baro_cat_index_load(db)
	list := baro_cat_link_orphans(idx)

	rows := [][]string{}
	for _, title := range baro_cat_slice(list, page_num) {
		cats := idx.DocCats[title]
		cat_html := tool.Baro_t(`<span class="bw_muted">없음</span>`)
		if len(cats) > 0 {
			cat_html = baro_cat_cat_link(cats[0])
			if len(cats) > 1 {
				cat_html += tool.Baro_t(` 외 `) + tool.Baro_itoa(len(cats)-1)
			}
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(title),
			cat_html,
			tool.Baro_time_cell(idx.Last[title]),
			baro_cat_doc_tools(title),
		})
	}

	body := tool.Baro_hero(tool.Baro_t("고립 문서"), tool.Baro_t("아무 문서도 링크하지 않는 문서입니다."), "")
	body += baro_cat_link_tabs("/baro/link/orphan")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/link", tool.Baro_t("연결 도구")}, {"", tool.Baro_t("고립 문서")}})

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("고립 문서"), tool.Baro_comma(len(list)), tool.Baro_t("휴지통 제외")},
		{tool.Baro_t("전체 문서"), tool.Baro_comma(idx.DocAll), tool.Baro_t("휴지통 제외")},
		{tool.Baro_t("이 쪽"), tool.Baro_comma(len(rows)), tool.Baro_itoa(page_num) + tool.Baro_t("쪽")},
	})

	body += baro_cat_index_note(idx)

	body += tool.Baro_alert("info", tool.Baro_t("고립 문서는 검색으로만 닿을 수 있습니다. 관련된 문서 본문에서 링크를 걸어 주면 좋습니다."))

	table := tool.Baro_table([]string{"문서", "분류", tool.Baro_t("마지막 편집"), tool.Baro_t("도구")}, rows, "")
	if len(rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("이 쪽에는 고립 문서가 없습니다."))
	}
	table += tool.Baro_paging(page_num, len(rows), baro_cat_page_size, "/baro/link/orphan?page={}")

	body += tool.Baro_card(tool.Baro_t("목록"), table)

	return tool.Baro_page(db, config, tool.Baro_t("고립 문서"), body, baro_cat_menu())
}

// View_baro_cat_link_broken : 끊어진 링크 (/baro/link/broken)
func View_baro_cat_link_broken(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	page_num := baro_cat_page_no(page)
	idx := baro_cat_index_load(db)
	list := baro_cat_link_broken_list(db, idx)

	start := baro_cat_offset(page_num)
	end := start + baro_cat_page_size
	if start > len(list) {
		start = len(list)
	}
	if end > len(list) {
		end = len(list)
	}

	hit := 0
	for _, one := range list {
		hit += one.Num
	}

	rows := [][]string{}
	for _, one := range list[start:end] {
		from_html := []string{}
		for index, from := range one.From {
			if index >= 3 {
				break
			}

			from_html = append(from_html, tool.Baro_doc_link(from))
		}
		if one.Num > len(from_html) {
			from_html = append(from_html, tool.Baro_t(`<span class="bw_muted">외 `)+tool.Baro_comma(one.Num-len(from_html))+tool.Baro_t(`개</span>`))
		}

		kind_html := tool.Baro_badge("문서", "info")
		if baro_cat_is_cat(one.Name) {
			kind_html = tool.Baro_badge("분류", "ok")
		}

		rows = append(rows, []string{
			tool.HTML_escape(baro_cat_disp(one.Name)) + " " + kind_html,
			tool.Baro_comma(one.Num),
			strings.Join(from_html, ", "),
			`<a class="bw_btn bw_btn_primary" href="/edit/` + tool.Url_parser(one.Name) + tool.Baro_t(`">문서 만들기</a> `) +
				`<a class="bw_btn" href="/baro/task/new?title=` + tool.Url_parser(one.Name) +
				tool.Baro_t(`&kind=create">과제로 등록</a> `) +
				`<a class="bw_btn" href="/baro/link/doc/` + tool.Url_parser(one.Name) + tool.Baro_t(`">연결</a>`),
		})
	}

	body := tool.Baro_hero(tool.Baro_t("끊어진 링크"), tool.Baro_t("많이 요청된 없는 문서부터 만들면 가장 도움이 됩니다."), "")
	body += baro_cat_link_tabs("/baro/link/broken")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/link", tool.Baro_t("연결 도구")}, {"", tool.Baro_t("끊어진 링크")}})

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("없는 문서"), tool.Baro_comma(len(list)), tool.Baro_t("링크는 있지만 문서가 없음")},
		{tool.Baro_t("요청 횟수"), tool.Baro_comma(hit), tool.Baro_t("이 문서들을 가리키는 링크 수")},
		{tool.Baro_t("이 쪽"), tool.Baro_comma(len(rows)), tool.Baro_itoa(page_num) + tool.Baro_t("쪽")},
	})

	body += baro_cat_index_note(idx)

	body += tool.Baro_alert("info", tool.Baro_t("요청이 많은 문서일수록 위키 전체에 도움이 됩니다. 만들기 어려우면 과제로 등록해 다른 사람에게 넘길 수 있습니다."))

	table := tool.Baro_table([]string{tool.Baro_t("없는 문서"), tool.Baro_t("요청 수"), tool.Baro_t("링크한 문서"), tool.Baro_t("할 일")}, rows, "")
	if len(rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("이 쪽에는 끊어진 링크가 없습니다."))
	}
	table += tool.Baro_paging(page_num, len(rows), baro_cat_page_size, "/baro/link/broken?page={}")

	body += tool.Baro_card(tool.Baro_t("요청이 많은 순"), table)

	return tool.Baro_page(db, config, tool.Baro_t("끊어진 링크"), body, baro_cat_menu())
}

// View_baro_cat_link_deadend : 막다른 문서 (/baro/link/deadend)
func View_baro_cat_link_deadend(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	page_num := baro_cat_page_no(page)
	idx := baro_cat_index_load(db)
	list := baro_cat_link_deadends(idx)

	rows := [][]string{}
	for _, title := range baro_cat_slice(list, page_num) {
		rows = append(rows, []string{
			tool.Baro_doc_link(title),
			tool.Baro_comma(baro_cat_link_uniq(idx.In[title])),
			tool.Baro_time_cell(idx.Last[title]),
			`<a class="bw_btn bw_btn_primary" href="/edit/` + tool.Url_parser(title) + tool.Baro_t(`">링크 넣기</a> `) +
				baro_cat_doc_tools(title),
		})
	}

	body := tool.Baro_hero(tool.Baro_t("막다른 문서"), tool.Baro_t("밖으로 나가는 링크가 하나도 없는 문서입니다."), "")
	body += baro_cat_link_tabs("/baro/link/deadend")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/link", tool.Baro_t("연결 도구")}, {"", tool.Baro_t("막다른 문서")}})

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("막다른 문서"), tool.Baro_comma(len(list)), tool.Baro_t("휴지통 제외")},
		{tool.Baro_t("전체 문서"), tool.Baro_comma(idx.DocAll), tool.Baro_t("휴지통 제외")},
		{tool.Baro_t("이 쪽"), tool.Baro_comma(len(rows)), tool.Baro_itoa(page_num) + tool.Baro_t("쪽")},
	})

	body += baro_cat_index_note(idx)

	body += tool.Baro_note(tool.Baro_t("분류만 달려 있고 본문에 다른 문서 링크가 없는 문서도 막다른 문서로 봅니다."))

	body += tool.Baro_alert("info", tool.Baro_t("본문에 나오는 다른 문서 이름을 [[대괄호 두 개]] 로 감싸면 링크가 됩니다. 분류를 다는 것도 좋은 출발점입니다."))

	table := tool.Baro_table([]string{"문서", tool.Baro_t("들어오는 링크"), tool.Baro_t("마지막 편집"), tool.Baro_t("할 일")}, rows, "")
	if len(rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("이 쪽에는 막다른 문서가 없습니다."))
	}
	table += tool.Baro_paging(page_num, len(rows), baro_cat_page_size, "/baro/link/deadend?page={}")

	body += tool.Baro_card(tool.Baro_t("목록"), table)

	return tool.Baro_page(db, config, tool.Baro_t("막다른 문서"), body, baro_cat_menu())
}
