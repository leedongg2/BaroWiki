package route

import (
	"database/sql"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  [cat] 모듈 연결 색인
//
//  openNAMU 본체는 문서를 저장하거나 다시 그릴 때에만 back 표(역링크 색인)를
//  채운다. 그래서 밖에서 자료를 부어 넣은 위키나 아직 한 번도 다시 그리지 않은
//  문서는 back 표에 아무 줄도 없다. 그 상태에서 back 표만 믿으면
//  "모든 문서가 고립 문서" · "모든 문서에 분류가 없음" 같은 사실과 다른 결과가 나온다.
//
//  그래서 이 모듈은 다음 순서로 색인을 만든다.
//    1. back 표에 이미 올라와 있는 문서는 back 표를 그대로 믿는다.
//    2. back 표에 한 줄도 없는 문서만 본문을 직접 읽어 링크와 분류를 뽑는다.
//  본문을 읽은 문서 수는 화면에 그대로 밝혀 어떤 근거로 센 값인지 알 수 있게 한다.
//
//  back.link  : 링크를 건 쪽 (출발 문서)
//  back.title : 링크가 가리키는 쪽 (도착 문서 · 분류 이름)
//  back.type  : "" 일반 링크 / cat 분류 / include 포함 / redirect 넘겨주기 /
//               file 파일 / no 대상 없음 / cat_view · cat_blur 분류 표시 옵션
// ============================================================

// baro_cat_edge_max : 한 번에 읽어 들이는 back 표 줄 수 상한
const baro_cat_edge_max = 20000

// baro_cat_real_sql : 휴지통을 뺀 진짜 문서 조건
const baro_cat_real_sql = " title not like 'trash:%' and title not like 'filetrash:%' "

// baro_cat_real_d_sql : 별칭 d 를 쓴 같은 조건
const baro_cat_real_d_sql = " d.title not like 'trash:%' and d.title not like 'filetrash:%' "

// baro_cat_pair : 링크 한 줄 (상대 문서 이름 + 링크 종류)
type baro_cat_pair struct {
	Name string
	Kind string
}

// baro_cat_index : 한 번의 요청 동안 쓰는 연결 색인
type baro_cat_index struct {
	Docs    []string                   // 존재하는 문서 이름 (가나다순)
	Exists  map[string]bool            // 문서가 있는가
	Last    map[string]string          // 문서별 마지막 편집 시각
	DocAll  int                        // 휴지통을 뺀 전체 문서 수
	Out     map[string][]baro_cat_pair // 문서 → 나가는 링크
	In      map[string][]baro_cat_pair // 문서 → 들어오는 링크
	CatDocs map[string][]string        // 분류 → 그 분류에 속한 문서
	DocCats map[string][]string        // 문서 → 그 문서에 달린 분류
	Scanned int                        // 본문을 직접 읽은 문서 수
	Backed  int                        // back 표에서 가져온 줄 수
	DocCut  bool                       // 문서 목록을 상한에서 잘랐는가
	TextCut bool                       // 본문 검사를 상한에서 잘랐는가
	seen    map[string]bool            // 같은 관계를 두 번 넣지 않기 위한 표시
	asked   map[string]bool            // 문서 목록을 잘랐을 때 다시 물어본 결과
}

// baro_cat_index_add : 색인에 관계 한 줄을 넣는다
func baro_cat_index_add(idx *baro_cat_index, from string, to string, kind string) {
	if from == "" || to == "" || from == to {
		return
	}

	// no 는 "대상 없음" 표시일 뿐이고 진짜 링크 줄이 따로 있다.
	// cat_view · cat_blur 는 분류를 어떻게 보여 줄지에 대한 표시라 링크가 아니다.
	switch kind {
	case "no", "nothing", "cat_view", "cat_blur":
		return
	}

	key := from + "\n" + to + "\n" + kind
	if idx.seen[key] {
		return
	}
	idx.seen[key] = true

	idx.Out[from] = append(idx.Out[from], baro_cat_pair{Name: to, Kind: kind})
	idx.In[to] = append(idx.In[to], baro_cat_pair{Name: from, Kind: kind})

	if kind == "cat" {
		idx.CatDocs[to] = append(idx.CatDocs[to], from)
		idx.DocCats[from] = append(idx.DocCats[from], to)
	}
}

// baro_cat_index_load : 연결 색인 만들기
func baro_cat_index_load(db *sql.DB) *baro_cat_index {
	idx := &baro_cat_index{
		Docs:    []string{},
		Exists:  map[string]bool{},
		Last:    map[string]string{},
		Out:     map[string][]baro_cat_pair{},
		In:      map[string][]baro_cat_pair{},
		CatDocs: map[string][]string{},
		DocCats: map[string][]string{},
		seen:    map[string]bool{},
		asked:   map[string]bool{},
	}

	idx.DocAll = baro_cat_count_one(db, "select count(*) from data where"+baro_cat_real_sql)

	// 1) 문서 이름
	func() {
		rows := tool.Query_DB(
			db,
			"select title from data where"+baro_cat_real_sql+"order by title asc limit ?",
			baro_cat_scan_max+1,
		)
		defer rows.Close()

		for rows.Next() {
			title := ""
			if rows.Scan(&title) != nil || title == "" {
				continue
			}

			if len(idx.Docs) >= baro_cat_scan_max {
				idx.DocCut = true
				break
			}

			idx.Docs = append(idx.Docs, title)
			idx.Exists[title] = true
		}
	}()

	// 2) 문서별 마지막 편집 시각
	func() {
		rows := tool.Query_DB(
			db,
			"select title, max(date) from history group by title limit ?",
			baro_cat_scan_max,
		)
		defer rows.Close()

		for rows.Next() {
			title, date := "", ""
			if rows.Scan(&title, &date) != nil {
				continue
			}

			idx.Last[title] = date
		}
	}()

	// 3) back 표에 이미 올라와 있는 관계
	func() {
		rows := tool.Query_DB(db, "select link, title, type from back limit ?", baro_cat_edge_max)
		defer rows.Close()

		for rows.Next() {
			from, to, kind := "", "", ""
			if rows.Scan(&from, &to, &kind) != nil {
				continue
			}

			idx.Backed++
			baro_cat_index_add(idx, from, tool.Namespace_normalize(to), kind)
		}
	}()

	// 4) back 표에 한 줄도 없는 문서는 본문을 직접 읽는다
	func() {
		rows := tool.Query_DB(
			db,
			"select d.title, d.data, d.type from data d where"+baro_cat_real_d_sql+
				"and not exists (select 1 from back b where b.link = d.title) order by d.title asc limit ?",
			baro_cat_text_scan_max+1,
		)
		defer rows.Close()

		for rows.Next() {
			title, data, markup := "", "", ""
			if rows.Scan(&title, &data, &markup) != nil || title == "" {
				continue
			}

			if idx.Scanned >= baro_cat_text_scan_max {
				idx.TextCut = true
				break
			}
			idx.Scanned++

			// raw 문서는 문법을 해석하지 않으므로 링크도 없다
			if markup == "raw" {
				continue
			}

			for _, one := range baro_cat_scan_links(data) {
				baro_cat_index_add(idx, title, one.Name, one.Kind)
			}
		}
	}()

	baro_cat_index_sort(idx)

	return idx
}

// baro_cat_index_sort : 색인 안의 목록을 보기 좋은 차례로 정렬
func baro_cat_index_sort(idx *baro_cat_index) {
	for key := range idx.Out {
		list := idx.Out[key]
		sort.SliceStable(list, func(i int, j int) bool {
			if list[i].Name != list[j].Name {
				return list[i].Name < list[j].Name
			}

			return list[i].Kind < list[j].Kind
		})
	}

	for key := range idx.In {
		list := idx.In[key]
		sort.SliceStable(list, func(i int, j int) bool {
			if list[i].Name != list[j].Name {
				return list[i].Name < list[j].Name
			}

			return list[i].Kind < list[j].Kind
		})
	}

	for key := range idx.CatDocs {
		sort.Strings(idx.CatDocs[key])
	}

	for key := range idx.DocCats {
		sort.Strings(idx.DocCats[key])
	}
}

// baro_cat_index_exists : 문서가 있는지 (목록을 잘랐으면 표를 다시 본다)
func baro_cat_index_exists(db *sql.DB, idx *baro_cat_index, title string) bool {
	if idx.Exists[title] {
		return true
	}

	if !idx.DocCut {
		return false
	}

	// 목록을 잘랐을 때만 표를 다시 본다. 같은 이름을 여러 번 묻지 않게 기억해 둔다.
	if found, ok := idx.asked[title]; ok {
		return found
	}

	found := baro_cat_doc_exists(db, title)
	idx.asked[title] = found

	return found
}

// baro_cat_index_note : 어떤 근거로 센 값인지 밝히는 안내
func baro_cat_index_note(idx *baro_cat_index) string {
	text := tool.Baro_t("역링크 색인(back 표) ") + tool.Baro_comma(idx.Backed) + tool.Baro_t("줄을 기준으로 셌습니다.")

	if idx.Scanned > 0 {
		text += tool.Baro_t(" 색인에 아직 올라오지 않은 문서 ") + tool.Baro_comma(idx.Scanned) +
			tool.Baro_t("개는 본문을 직접 읽어 보탰습니다. (문서를 한 번 저장하면 색인에 올라갑니다)")
	}

	if idx.DocCut || idx.TextCut {
		text += tool.Baro_t(" 문서가 매우 많아 앞쪽 ") + tool.Baro_comma(baro_cat_scan_max) +
			tool.Baro_t("개까지만 훑었으므로 아래 수는 최소값입니다.")
	}

	return tool.Baro_note(text)
}

// baro_cat_index_cats : 분류 이름과 분류별 문서 수
func baro_cat_index_cats(idx *baro_cat_index) ([]string, map[string]int) {
	counts := map[string]int{}

	for name, list := range idx.CatDocs {
		counts[name] = len(list)
	}

	// 문서로만 있고 아직 아무도 달지 않은 분류도 목록에 넣는다
	for _, title := range idx.Docs {
		if !baro_cat_is_cat(title) {
			continue
		}

		if _, ok := counts[title]; !ok {
			counts[title] = 0
		}
	}

	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	return names, counts
}

// baro_cat_index_edges : 분류 사이의 상하 관계
//
//	위쪽 반환값 : 하위분류 → 상위분류 목록
//	아래쪽 반환값 : 상위분류 → 하위분류 목록
func baro_cat_index_edges(idx *baro_cat_index) (map[string][]string, map[string][]string) {
	up := map[string][]string{}
	down := map[string][]string{}

	for name, list := range idx.CatDocs {
		for _, doc := range list {
			if !baro_cat_is_cat(doc) || doc == name {
				continue
			}

			up[doc] = append(up[doc], name)
			down[name] = append(down[name], doc)
		}
	}

	for key := range up {
		sort.Strings(up[key])
	}
	for key := range down {
		sort.Strings(down[key])
	}

	return up, down
}

// baro_cat_index_cat_last : 분류에 속한 문서들의 마지막 편집 시각
func baro_cat_index_cat_last(idx *baro_cat_index, name string) string {
	last := ""
	for _, doc := range idx.CatDocs[name] {
		if date := idx.Last[doc]; date > last {
			last = date
		}
	}

	return last
}

// baro_cat_index_by_date : 문서 목록을 최근 편집순으로
func baro_cat_index_by_date(idx *baro_cat_index, list []string) []string {
	out := append([]string{}, list...)

	sort.SliceStable(out, func(i int, j int) bool {
		if idx.Last[out[i]] != idx.Last[out[j]] {
			return idx.Last[out[i]] > idx.Last[out[j]]
		}

		return out[i] < out[j]
	})

	return out
}

// baro_cat_index_out_real : 밖으로 나가는 진짜 링크 (분류·파일 제외)
func baro_cat_index_out_real(idx *baro_cat_index, title string) []baro_cat_pair {
	out := []baro_cat_pair{}

	for _, one := range idx.Out[title] {
		switch one.Kind {
		case "", "redirect", "include":
			out = append(out, one)
		}
	}

	return out
}

// baro_cat_index_editors : 문서 묶음을 편집한 사람 상위 목록과 전체 편집 수
func baro_cat_index_editors(db *sql.DB, titles []string, top int) ([][]string, int) {
	out := [][]string{}
	if len(titles) == 0 {
		return out, 0
	}

	// 자리표시자가 지나치게 길어지지 않게 앞쪽만 본다
	if len(titles) > 300 {
		titles = titles[:300]
	}

	holes := make([]string, len(titles))
	values := make([]any, 0, len(titles))
	for index, title := range titles {
		holes[index] = "?"
		values = append(values, title)
	}
	hole_sql := "(" + strings.Join(holes, ", ") + ")"

	all := baro_cat_count_one(db, "select count(*) from history where title in "+hole_sql, values...)

	rows := tool.Query_DB(
		db,
		"select ip, count(*) from history where title in "+hole_sql+
			" group by ip order by count(*) desc, ip asc limit ?",
		append(append([]any{}, values...), top)...,
	)
	defer rows.Close()

	for rows.Next() {
		who := ""
		count := 0
		if rows.Scan(&who, &count) != nil {
			continue
		}

		out = append(out, []string{who, tool.Baro_itoa(count)})
	}

	return out, all
}

// ------------------------------------------------------------
//  본문에서 링크 뽑기
// ------------------------------------------------------------

// baro_cat_strip_raw : 링크로 읽으면 안 되는 부분을 지운다
//
//	{{{ }}} 안은 "문법을 그대로 보여 주는 곳" 이라 링크가 아니고,
//	## 로 시작하는 줄은 주석이다.
func baro_cat_strip_raw(data string) string {
	out := ""
	rest := data

	for {
		start := strings.Index(rest, "{{{")
		if start < 0 {
			out += rest
			break
		}

		out += rest[:start]
		rest = rest[start+3:]

		end := strings.Index(rest, "}}}")
		if end < 0 {
			break
		}

		rest = rest[end+3:]
	}

	keep := []string{}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "##") {
			continue
		}

		keep = append(keep, line)
	}

	return strings.Join(keep, "\n")
}

// baro_cat_link_name : [[ ]] 안의 글자를 문서 이름과 링크 종류로
func baro_cat_link_name(inner string) (string, string) {
	if pipe := strings.Index(inner, "|"); pipe >= 0 {
		inner = inner[:pipe]
	}
	if hash := strings.Index(inner, "#"); hash >= 0 {
		inner = inner[:hash]
	}

	inner = strings.TrimSpace(inner)
	inner = strings.TrimPrefix(inner, "<")
	inner = strings.TrimSuffix(inner, ">")
	inner = strings.TrimSpace(inner)

	lower := strings.ToLower(inner)
	if inner == "" ||
		strings.HasPrefix(inner, "/") ||
		strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") {
		return "", ""
	}

	name := tool.Namespace_normalize(inner)
	if name == "" || strings.HasSuffix(name, ":") {
		return "", ""
	}

	switch {
	case baro_cat_is_cat(name):
		return name, "cat"
	case strings.HasPrefix(strings.ToLower(name), "file:"):
		return name, "file"
	}

	return name, ""
}

// baro_cat_scan_links : 본문에서 링크를 나오는 차례대로 뽑는다 (중복도 그대로 둔다)
func baro_cat_scan_links(data string) []baro_cat_pair {
	out := []baro_cat_pair{}
	body := baro_cat_strip_raw(data)

	// 넘겨주기 (#redirect 문서 이름)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)

		prefix := ""
		switch {
		case strings.HasPrefix(lower, "#redirect "):
			prefix = "#redirect "
		case strings.HasPrefix(lower, "#넘겨주기 "):
			prefix = "#넘겨주기 "
		}

		if prefix == "" {
			continue
		}

		target := strings.TrimSpace(line[len(prefix):])
		target = strings.TrimSuffix(strings.TrimPrefix(target, "[["), "]]")

		if name, _ := baro_cat_link_name(target); name != "" {
			out = append(out, baro_cat_pair{Name: name, Kind: "redirect"})
		}

		break
	}

	// [[문서 이름]] 꼴
	rest := body
	for len(out) < 400 {
		start := strings.Index(rest, "[[")
		if start < 0 {
			break
		}
		rest = rest[start+2:]

		end := strings.Index(rest, "]]")
		if end < 0 {
			break
		}

		inner := rest[:end]
		rest = rest[end+2:]

		if name, kind := baro_cat_link_name(inner); name != "" {
			out = append(out, baro_cat_pair{Name: name, Kind: kind})
		}
	}

	// [include(문서 이름, ...)] 꼴
	rest = body
	for len(out) < 400 {
		start := strings.Index(rest, "[include(")
		if start < 0 {
			break
		}
		rest = rest[start+len("[include("):]

		end := strings.Index(rest, ")")
		if end < 0 {
			break
		}

		inner := rest[:end]
		rest = rest[end+1:]

		if comma := strings.Index(inner, ","); comma >= 0 {
			inner = inner[:comma]
		}

		if name, _ := baro_cat_link_name(inner); name != "" {
			out = append(out, baro_cat_pair{Name: name, Kind: "include"})
		}
	}

	return out
}

// baro_cat_scan_categories : 본문에 달린 분류만 차례대로 (중복도 그대로 둔다)
func baro_cat_scan_categories(data string) []string {
	out := []string{}

	for _, one := range baro_cat_scan_links(data) {
		if one.Kind == "cat" {
			out = append(out, one.Name)
		}
	}

	return out
}

// ------------------------------------------------------------
//  화면 안에서 쓰는 쪽 나누기
// ------------------------------------------------------------

// baro_cat_slice : 목록에서 이 쪽에 해당하는 부분만
func baro_cat_slice(list []string, page int) []string {
	start := baro_cat_offset(page)
	if start >= len(list) {
		return []string{}
	}

	end := start + baro_cat_page_size
	if end > len(list) {
		end = len(list)
	}

	return list[start:end]
}

// baro_cat_slice_pair : 링크 목록에서 이 쪽에 해당하는 부분만
func baro_cat_slice_pair(list []baro_cat_pair, page int) []baro_cat_pair {
	start := baro_cat_offset(page)
	if start >= len(list) {
		return []baro_cat_pair{}
	}

	end := start + baro_cat_page_size
	if end > len(list) {
		end = len(list)
	}

	return list[start:end]
}
