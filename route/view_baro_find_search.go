package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  5-1 내부 검색 / 5-2 결과 없음 처리
//
//  한국 인물 위키를 전제로 아래가 모두 같은 문서로 이어져야 한다.
//    김성범 / 金成範 / 김성범 독립운동가 / 김성범 1890 / ㄱㅅㅂ
// ============================================================

// baro_find_hit : 검색 결과 한 건
type baro_find_hit struct {
	Title   string
	Score   int
	Reasons []string
	Alias   string
	Dist    int
}

// baro_find_query : 해석된 검색 조건
type baro_find_query struct {
	Raw     string // 입력 그대로
	Base    string // 연도·직업 토큰을 뺀 핵심 검색어
	Norm    string // 정규화한 핵심 검색어
	Birth   string // 검색어에서 뽑아낸 출생연도
	Job     string // 검색어에서 뽑아낸 직업
	Chosung bool   // 초성만으로 이루어진 검색어인지
	Scope   string
	Field   string
	Cat     string
	Sort    string
	InBirth string // 필터 상자로 직접 고른 출생연도
	InJob   string // 필터 상자로 직접 고른 직업
}

// baro_find_job_hint : 검색어에서 직업을 알아보기 위한 기본 낱말
//
//	baro_doc_meta 에 실제로 쓰인 직업 값과 함께 사용한다.
func baro_find_job_hint() []string {
	return []string{
		tool.Baro_t("독립운동가"), tool.Baro_t("정치인"), tool.Baro_t("언론인"), tool.Baro_t("기자"), tool.Baro_t("군인"), tool.Baro_t("학자"), tool.Baro_t("교육자"), tool.Baro_t("교수"),
		tool.Baro_t("기업인"), tool.Baro_t("작가"), tool.Baro_t("시인"), tool.Baro_t("소설가"), tool.Baro_t("화가"), tool.Baro_t("음악가"), tool.Baro_t("가수"), tool.Baro_t("배우"),
		tool.Baro_t("야구선수"), tool.Baro_t("축구선수"), tool.Baro_t("운동선수"), tool.Baro_t("승려"), tool.Baro_t("목사"), tool.Baro_t("신부"), tool.Baro_t("의사"), tool.Baro_t("법조인"),
		tool.Baro_t("판사"), tool.Baro_t("검사"), tool.Baro_t("변호사"), tool.Baro_t("공무원"), tool.Baro_t("발명가"), tool.Baro_t("과학자"), tool.Baro_t("건축가"), tool.Baro_t("성우"),
	}
}

// baro_find_parse_query : 검색어에서 출생연도·직업 토큰을 분리한다
func baro_find_parse_query(db *sql.DB, raw string, in_birth string, in_job string) baro_find_query {
	out := baro_find_query{Raw: strings.TrimSpace(raw), InBirth: in_birth, InJob: in_job}

	job_set := map[string]bool{}
	for _, value := range baro_find_job_hint() {
		job_set[value] = true
	}
	for _, value := range baro_find_meta_value_list(db, "job") {
		job_set[value] = true
	}

	rest := []string{}
	for _, token := range strings.Fields(out.Raw) {
		// 네 자리 숫자는 출생연도로 본다. (김성범 1890)
		if len(token) == 4 && strings.Trim(token, "0123456789") == "" {
			year := tool.Str_to_int(token)
			if year >= 1000 && year <= 2200 {
				out.Birth = token
				continue
			}
		}

		// 직업 낱말은 필터로 옮긴다. (김성범 독립운동가)
		if job_set[token] {
			out.Job = token
			continue
		}

		rest = append(rest, token)
	}

	out.Base = strings.Join(rest, " ")
	if strings.TrimSpace(out.Base) == "" {
		// 연도나 직업만 입력한 경우 원래 검색어를 그대로 쓴다.
		out.Base = out.Raw
	}

	out.Norm = baro_find_norm(out.Base)
	out.Chosung = tool.Baro_is_chosung_only(out.Base)

	if out.InBirth != "" {
		out.Birth = out.InBirth
	}
	if out.InJob != "" {
		out.Job = out.InJob
	}

	return out
}

// baro_find_hit_add : 후보를 점수와 함께 담는다 (더 높은 점수가 이긴다)
func baro_find_hit_add(box map[string]*baro_find_hit, title string, score int, reason string) *baro_find_hit {
	hit, ok := box[title]
	if !ok {
		hit = &baro_find_hit{Title: title, Dist: -1}
		box[title] = hit
	}

	if score > hit.Score {
		hit.Score = score
	}

	if reason != "" {
		for _, old := range hit.Reasons {
			if old == reason {
				return hit
			}
		}

		hit.Reasons = append(hit.Reasons, reason)
	}

	return hit
}

// baro_find_meta_map : 검색에 쓰는 문서 메타 전체 지도
func baro_find_meta_map(db *sql.DB) map[string]map[string]string {
	out := map[string]map[string]string{}

	rows := tool.Query_DB(
		db,
		"select title, name, data from baro_doc_meta where name in ('birth_year', 'death_year', 'job', 'hanja', 'field', 'description') limit ?",
		baro_find_scan_limit,
	)
	defer rows.Close()

	for rows.Next() {
		title, name, data := "", "", ""
		if err := rows.Scan(&title, &name, &data); err != nil {
			continue
		}

		if out[title] == nil {
			out[title] = map[string]string{}
		}

		out[title][name] = data
	}

	return out
}

// baro_find_last_edit_map : 문서별 마지막 편집 시각 지도 (최신순 정렬용)
func baro_find_last_edit_map(db *sql.DB) map[string]string {
	out := map[string]string{}

	rows := tool.Query_DB(
		db,
		"select doc_name, set_data from data_set where set_name = 'last_edit' and doc_rev = '' limit ?",
		baro_find_scan_limit,
	)
	defer rows.Close()

	for rows.Next() {
		title, date := "", ""
		if err := rows.Scan(&title, &date); err != nil {
			continue
		}

		out[title] = date
	}

	return out
}

// baro_find_cat_name : 입력한 분류 이름에서 접두사를 떼어 낸다 ("분류:인물" → "인물")
func baro_find_cat_name(cat string) string {
	name := strings.TrimSpace(cat)
	lower := strings.ToLower(name)

	for _, prefix := range []string{"category:", "분류:"} {
		if strings.HasPrefix(lower, prefix) {
			name = name[len(prefix):]

			break
		}
	}

	return strings.TrimSpace(name)
}

// baro_find_cat_set : 특정 분류에 들어 있는 문서 집합
//
//	back 표(분류 역색인)에서 읽고, 아직 색인에 오르지 않은 문서를 위해
//	본문의 [[분류:이름]] 표기도 함께 찾아 합친다.
//
//	back 표의 한 줄은 title 이 분류 이름, link 가 그 분류에 속한 문서다.
//	(/baro/category/view 가 쓰는 분류 색인도 link 를 문서, title 을 분류로 읽는다)
func baro_find_cat_set(db *sql.DB, cat string) map[string]bool {
	out := map[string]bool{}

	name := baro_find_cat_name(cat)
	if name == "" {
		return out
	}

	target := "category:" + strings.ToLower(name)

	rows := tool.Query_DB(db, "select link from back where type = 'cat' and lower(title) = ? limit ?", target, baro_find_scan_limit)
	defer rows.Close()

	for rows.Next() {
		doc := ""
		if err := rows.Scan(&doc); err != nil {
			continue
		}

		out[doc] = true
	}

	// 역색인은 문서를 저장할 때 조금씩 쌓이므로 아직 빠진 문서가 있다.
	// 그래서 결과가 있든 없든 본문에서 분류 표기를 직접 찾아 함께 담는다.
	lower_name := strings.ToLower(name)
	body_rows := tool.Query_DB(
		db,
		"select title from data where lower(data) like ? or lower(data) like ? or lower(data) like ? or lower(data) like ? limit ?",
		tool.Baro_t("%[[분류:")+lower_name+"]]%",
		tool.Baro_t("%[[분류:")+lower_name+"|%",
		"%[[category:"+lower_name+"]]%",
		"%[[category:"+lower_name+"|%",
		baro_find_scan_limit,
	)
	defer body_rows.Close()

	for body_rows.Next() {
		title := ""
		if err := body_rows.Scan(&title); err != nil {
			continue
		}

		out[title] = true
	}

	return out
}

// baro_find_alias_scan : 별칭 전수 목록 ({title, alias, kind})
func baro_find_alias_scan(db *sql.DB) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(db, "select title, alias, kind from baro_doc_alias limit ?", baro_find_scan_limit)
	defer rows.Close()

	for rows.Next() {
		title, alias, kind := "", "", ""
		if err := rows.Scan(&title, &alias, &kind); err != nil {
			continue
		}

		out = append(out, []string{title, alias, kind})
	}

	return out
}

// baro_find_collect : 검색어에 대한 후보를 모으고 점수를 매긴다
//
//	관련도 점수 : 제목 완전 일치 > 제목 시작 일치 > 별칭 일치 > 제목 부분 일치 > 본문 일치
func baro_find_collect(db *sql.DB, query baro_find_query) []*baro_find_hit {
	box := map[string]*baro_find_hit{}
	if query.Norm == "" {
		return []*baro_find_hit{}
	}

	// 1) 제목 완전 일치
	baro_find_scan_titles(db, box,
		"select title from data where replace(lower(title), ' ', '') = ? limit 50",
		query.Norm, 1000, tool.Baro_t("제목 완전 일치"))

	// 2) 제목 시작 일치
	baro_find_scan_titles(db, box,
		"select title from data where replace(lower(title), ' ', '') like ? limit 200",
		query.Norm+"%", 700, tool.Baro_t("제목 시작 일치"))

	// 3) 별칭 일치 (한자명·개명 전 이름·호·외국어 표기 등)
	for _, row := range baro_find_alias_scan(db) {
		alias_norm := baro_find_norm(row[1])
		if alias_norm == "" {
			continue
		}

		score := 0
		switch {
		case alias_norm == query.Norm:
			score = 620
		case strings.HasPrefix(alias_norm, query.Norm):
			score = 430
		case strings.Contains(alias_norm, query.Norm):
			score = 350
		}

		if score == 0 && !query.Chosung {
			// 별칭에도 오타 허용을 적용한다.
			if tool.Get_len(query.Norm) >= 2 {
				if dist := baro_find_lev(alias_norm, query.Norm, 2); dist <= 2 {
					score = 300 - dist*60
				}
			}
		}

		if score == 0 && query.Chosung {
			// 초성은 공백을 빼고 맞춘다. ("ㄱ ㅅ ㅂ" 도 "ㄱㅅㅂ" 과 같게 다룬다)
			if strings.Contains(baro_find_norm(tool.Baro_chosung(row[1])), query.Norm) {
				score = 480
			}
		}

		if score == 0 {
			continue
		}

		reason := tool.Baro_t("별칭 일치 (") + baro_find_alias_kind_name(row[2]) + ")"
		hit := baro_find_hit_add(box, row[0], score, reason)
		if hit.Alias == "" {
			hit.Alias = row[1]
		}
	}

	// 4) 한자명 메타 일치
	baro_find_scan_titles(db, box,
		"select title from baro_doc_meta where name = 'hanja' and replace(lower(data), ' ', '') = ? limit 50",
		query.Norm, 600, tool.Baro_t("한자명 일치"))

	// 5) 제목 부분 일치
	baro_find_scan_titles(db, box,
		"select title from data where replace(lower(title), ' ', '') like ? limit 300",
		"%"+query.Norm+"%", 400, tool.Baro_t("제목 부분 일치"))

	// 6) 본문 일치
	baro_find_scan_titles(db, box,
		"select title from data where lower(data) like ? limit 300",
		"%"+strings.ToLower(query.Base)+"%", 120, tool.Baro_t("본문 일치"))

	// 7) 초성 검색 / 오타 허용 (제목 전수 조사)
	if query.Chosung || tool.Get_len(query.Norm) >= 2 {
		for _, title := range baro_find_title_list(db) {
			if query.Chosung {
				// 제목의 초성도 공백을 빼고 맞춘다.
				chosung := baro_find_norm(tool.Baro_chosung(title))
				if chosung == query.Norm {
					baro_find_hit_add(box, title, 620, tool.Baro_t("초성 완전 일치"))
				} else if strings.Contains(chosung, query.Norm) {
					baro_find_hit_add(box, title, 500, tool.Baro_t("초성 일치"))
				}

				continue
			}

			if hit, ok := box[title]; ok && hit.Score >= 400 {
				continue
			}

			dist := baro_find_lev(baro_find_norm(title), query.Norm, 2)
			if dist >= 1 && dist <= 2 {
				hit := baro_find_hit_add(box, title, 320-dist*60, tool.Baro_t("오타 교정 후보 (거리 ")+tool.Baro_itoa(dist)+")")
				hit.Dist = dist
			}
		}
	}

	out := []*baro_find_hit{}
	for _, hit := range box {
		out = append(out, hit)
	}

	return out
}

// baro_find_scan_titles : 제목 하나를 돌려주는 질의를 실행해 후보에 담는다
func baro_find_scan_titles(db *sql.DB, box map[string]*baro_find_hit, query string, param string, score int, reason string) {
	rows := tool.Query_DB(db, query, param)
	defer rows.Close()

	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		baro_find_hit_add(box, title, score, reason)
	}
}

// baro_find_search_url : 검색 주소 만들기 (page 자리에 {} 를 넣으면 쪽 넘기기용)
func baro_find_search_url(query baro_find_query, page string) string {
	values := url.Values{}
	values.Set("q", query.Raw)
	if query.Scope != "" {
		values.Set("scope", query.Scope)
	}
	if query.Sort != "" {
		values.Set("sort", query.Sort)
	}
	if query.Field != "" {
		values.Set("field", query.Field)
	}
	if query.InBirth != "" {
		values.Set("birth", query.InBirth)
	}
	if query.InJob != "" {
		values.Set("job", query.InJob)
	}
	if query.Cat != "" {
		values.Set("cat", query.Cat)
	}

	out := "/baro/search?" + values.Encode()
	if page != "" {
		out += "&page=" + page
	}

	return out
}

// baro_find_search_form : 검색 상자와 필터 (GET 폼)
func baro_find_search_form(db *sql.DB, query baro_find_query) string {
	body := tool.Baro_t(`<div class="bw_f"><label class="bw_f_l">검색어</label>`)
	body += `<input class="bw_in" id="bw_find_q" name="q" list="bw_find_sug" autocomplete="off" value="` + tool.HTML_escape(query.Raw) + `" placeholder="김성범 / 金成範 / 김성범 독립운동가 / 김성범 1890 / ㄱㅅㅂ">`
	body += `<datalist id="bw_find_sug"></datalist>`
	body += tool.Baro_t(`<div class="bw_f_h">한자명·별칭·초성으로도 찾을 수 있고, 오타는 한두 글자까지 자동으로 보정합니다.</div></div>`)

	body += tool.Baro_select("scope", tool.Baro_t("찾을 대상"), baro_find_scope_list(), query.Scope, "")
	body += tool.Baro_select("sort", tool.Baro_t("정렬"), [][]string{
		{"", tool.Baro_t("관련도순")},
		{"recent", tool.Baro_t("최신순")},
	}, query.Sort, "관련도순은 제목 완전 일치 > 제목 시작 일치 > 별칭 일치 > 제목 부분 일치 > 본문 일치 차례로 점수를 줍니다.")
	body += tool.Baro_select("field", "분야", baro_find_field_options(db), query.Field, "")
	body += tool.Baro_select("birth", tool.Baro_t("출생연도"), baro_find_value_options(tool.Baro_t("출생연도 전체"), baro_find_meta_value_list(db, "birth_year")), query.InBirth, tool.Baro_t("인물 문서의 baro_doc_meta 출생연도 값으로 거릅니다."))
	body += tool.Baro_select("job", tool.Baro_t("직업"), baro_find_value_options(tool.Baro_t("직업 전체"), baro_find_meta_value_list(db, "job")), query.InJob, "")
	body += tool.Baro_input("cat", "분류", query.Cat, tool.Baro_t("예: 독립운동가"), tool.Baro_t("분류 이름만 적습니다. (분류: 접두사는 없어도 됩니다)"))
	body += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("검색"), "primary") + `</div>`

	form := `<form class="bw_form" method="get" action="/baro/search">` + body + `</form>`

	// 자동 완성 : /baro/search/suggest 의 JSON 을 datalist 에 붙인다.
	form += `<script>
(function(){
  var box = document.getElementById("bw_find_q");
  var list = document.getElementById("bw_find_sug");
  if(!box || !list){ return; }
  var timer = null;
  box.addEventListener("input", function(){
    if(timer){ clearTimeout(timer); }
    timer = setTimeout(function(){
      var word = box.value;
      if(!word || word.length < 1){ list.innerHTML = ""; return; }
      fetch("/baro/search/suggest?q=" + encodeURIComponent(word))
        .then(function(res){ return res.json(); })
        .then(function(data){
          var items = (data && data.data) ? data.data : [];
          var html = "";
          for(var i = 0; i < items.length; i++){
            var opt = document.createElement("option");
            opt.value = items[i].title;
            opt.label = items[i].note || "";
            html += opt.outerHTML;
          }
          list.innerHTML = html;
        })
        .catch(function(){ list.innerHTML = ""; });
    }, 180);
  });
})();
</script>`

	return form
}

// View_baro_find_search : 5-1 내부 검색 / 5-2 결과 없음 처리
func View_baro_find_search(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	query := baro_find_parse_query(db, baro_find_val(values, "q"), baro_find_val(values, "birth"), baro_find_val(values, "job"))
	query.Scope = baro_find_val(values, "scope")
	query.Field = baro_find_val(values, "field")
	query.Cat = baro_find_val(values, "cat")
	query.Sort = baro_find_val(values, "sort")

	page_int := baro_find_page_int(page)
	if raw_page := baro_find_val(values, "page"); raw_page != "" {
		page_int = baro_find_page_int(raw_page)
	}

	body := baro_find_tabs("/baro/search")
	body += tool.Baro_hero(tool.Baro_t("내부 검색"), tool.Baro_t("제목·본문·별칭·한자명·초성을 한 번에 찾습니다."), "")
	body += tool.Baro_card(tool.Baro_t("검색"), baro_find_search_form(db, query))

	if query.Raw == "" {
		body += tool.Baro_card(tool.Baro_t("검색 도움말"), tool.Baro_list([]string{
			tool.Baro_t(`<b>한자명</b> — 별칭에 한자명(kind=hanja)을 등록해 두면 <b>金成範</b> 으로도 <b>김성범</b> 문서를 찾습니다.`),
			tool.Baro_t(`<b>초성</b> — <b>ㄱㅅㅂ</b> 처럼 초성만 입력해도 제목의 초성과 맞춰 봅니다.`),
			tool.Baro_t(`<b>오타</b> — 편집 거리 1~2 인 제목을 교정 후보로 함께 보여 줍니다.`),
			tool.Baro_t(`<b>구분어</b> — <b>김성범 1890</b>, <b>김성범 독립운동가</b> 처럼 출생연도·직업을 붙이면 그 조건으로 자동으로 좁힙니다.`),
			tool.Baro_t(`격리된 문서와 사용자 연습장은 검색 결과에 나오지 않습니다.`),
		}))

		return tool.Baro_page(db, config, tool.Baro_t("내부 검색"), body, baro_find_menu())
	}

	// 검색어 해석 결과 안내
	notes := []string{}
	if query.Birth != "" {
		notes = append(notes, tool.Baro_t("출생연도 ")+tool.HTML_escape(query.Birth))
	}
	if query.Job != "" {
		notes = append(notes, tool.Baro_t("직업 ")+tool.HTML_escape(query.Job))
	}
	if query.Chosung {
		notes = append(notes, tool.Baro_t("초성 검색"))
	}
	if len(notes) > 0 {
		body += tool.Baro_alert("info", tool.Baro_t(`검색어에서 <b>`)+strings.Join(notes, "</b>, <b>")+tool.Baro_t(`</b> 조건을 알아냈습니다. 핵심 검색어는 <b>`)+tool.HTML_escape(query.Base)+tool.Baro_t(`</b> 입니다.`))
	}

	hits := baro_find_collect(db, query)

	hidden := baro_find_hidden_set(db)
	meta_map := baro_find_meta_map(db)
	edit_map := baro_find_last_edit_map(db)

	cat_set := map[string]bool{}
	if query.Cat != "" {
		cat_set = baro_find_cat_set(db, query.Cat)
	}

	kept := []*baro_find_hit{}
	for _, hit := range hits {
		if baro_find_excluded(hit.Title, hidden) {
			continue
		}
		if !baro_find_scope_ok(hit.Title, query.Scope) {
			continue
		}
		if query.Cat != "" && !cat_set[hit.Title] {
			continue
		}

		meta := meta_map[hit.Title]
		if query.Field != "" && (meta == nil || meta["field"] != query.Field) {
			continue
		}
		if query.InBirth != "" && (meta == nil || meta["birth_year"] != query.InBirth) {
			continue
		}
		if query.InJob != "" && (meta == nil || meta["job"] != query.InJob) {
			continue
		}

		// 검색어에서 알아낸 조건은 거르지 않고 점수를 올린다. (같은 이름의 다른 인물도 함께 보여 준다)
		if meta != nil {
			if query.Birth != "" && meta["birth_year"] == query.Birth {
				hit.Score += 250
				hit.Reasons = append(hit.Reasons, tool.Baro_t("출생연도 일치"))
			}
			if query.Job != "" && meta["job"] == query.Job {
				hit.Score += 250
				hit.Reasons = append(hit.Reasons, tool.Baro_t("직업 일치"))
			}
		}

		kept = append(kept, hit)
	}

	if query.Sort == "recent" {
		sort.SliceStable(kept, func(a int, b int) bool {
			if edit_map[kept[a].Title] != edit_map[kept[b].Title] {
				return edit_map[kept[a].Title] > edit_map[kept[b].Title]
			}

			return kept[a].Title < kept[b].Title
		})
	} else {
		sort.SliceStable(kept, func(a int, b int) bool {
			if kept[a].Score != kept[b].Score {
				return kept[a].Score > kept[b].Score
			}

			return kept[a].Title < kept[b].Title
		})
	}

	if len(kept) == 0 {
		body += baro_find_no_result(db, config, query)

		return tool.Baro_page(db, config, tool.Baro_t("검색 : ")+query.Raw, body, baro_find_menu())
	}

	start := (page_int - 1) * tool.Baro_page_size
	if start > len(kept) {
		start = len(kept)
	}
	end := start + tool.Baro_page_size
	if end > len(kept) {
		end = len(kept)
	}

	rows := [][]string{}
	for _, hit := range kept[start:end] {
		meta := meta_map[hit.Title]
		if meta == nil {
			meta = map[string]string{}
		}

		raw_data := ""
		tool.QueryRow_DB(db, "select data from data where title = ?", []any{&raw_data}, hit.Title)

		title_cell := tool.Baro_doc_link(hit.Title)
		if line := baro_find_person_line(meta); line != "" {
			title_cell += `<div class="bw_note">` + tool.HTML_escape(line) + `</div>`
		}
		if hit.Alias != "" {
			title_cell += tool.Baro_t(`<div class="bw_note">별칭 : `) + tool.HTML_escape(hit.Alias) + `</div>`
		}

		badge_cell := ""
		for _, reason := range hit.Reasons {
			kind := "muted"
			switch {
			case strings.HasPrefix(reason, "제목 완전"):
				kind = "ok"
			case strings.HasPrefix(reason, "별칭"), strings.HasPrefix(reason, "한자명"):
				kind = "info"
			case strings.HasPrefix(reason, "오타"):
				kind = "warn"
			}

			badge_cell += tool.Baro_badge(reason, kind) + " "
		}

		if status := tool.Baro_doc_status_get(db, hit.Title); status != "" {
			badge_cell += tool.Baro_status_badge(status)
		}

		rows = append(rows, []string{
			title_cell,
			baro_find_snippet(raw_data, query.Base),
			badge_cell,
			tool.Baro_time_cell(edit_map[hit.Title]),
			tool.Baro_itoa(hit.Score),
		})
	}

	table := tool.Baro_table(
		[]string{"문서", tool.Baro_t("발췌"), tool.Baro_t("일치 방식"), tool.Baro_t("마지막 편집"), tool.Baro_t("점수")},
		rows,
		tool.Baro_t("이 쪽에는 결과가 없습니다."),
	)
	table += tool.Baro_paging(page_int, end-start, tool.Baro_page_size, baro_find_search_url(query, "{}"))

	body += tool.Baro_card_ex(
		tool.Baro_t("검색 결과"),
		tool.Baro_t("모두 ")+tool.Baro_comma(len(kept))+tool.Baro_t("건"),
		table,
		tool.Baro_button("/baro/doc/alias", tool.Baro_t("별칭·동명이인 관리"), "")+" "+
			tool.Baro_button("/baro/discover", tool.Baro_t("발견 화면"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("검색 : ")+query.Raw, body, baro_find_menu())
}

// baro_find_no_result : 5-2 검색 결과 없음 처리
func baro_find_no_result(db *sql.DB, config tool.Config, query baro_find_query) string {
	_ = config

	hidden := baro_find_hidden_set(db)
	meta_map := baro_find_meta_map(db)

	// 비슷한 문서 (편집 거리 / 초성)
	type near struct {
		Title  string
		Dist   int
		Reason string
	}

	near_list := []near{}
	chosung_key := baro_find_norm(tool.Baro_chosung(query.Base))

	for _, title := range baro_find_title_list(db) {
		if baro_find_excluded(title, hidden) {
			continue
		}

		norm := baro_find_norm(title)
		if norm == "" {
			continue
		}

		limit := 3
		if tool.Get_len(query.Norm) >= 6 {
			limit = 4
		}

		if dist := baro_find_lev(norm, query.Norm, limit); dist <= limit {
			near_list = append(near_list, near{title, dist, tool.Baro_t("편집 거리 ") + tool.Baro_itoa(dist)})
			continue
		}

		if chosung_key != "" && strings.Contains(baro_find_norm(tool.Baro_chosung(title)), chosung_key) {
			near_list = append(near_list, near{title, 9, tool.Baro_t("초성이 비슷함")})
		}
	}

	sort.SliceStable(near_list, func(a int, b int) bool {
		if near_list[a].Dist != near_list[b].Dist {
			return near_list[a].Dist < near_list[b].Dist
		}

		return near_list[a].Title < near_list[b].Title
	})

	if len(near_list) > 10 {
		near_list = near_list[:10]
	}

	near_rows := [][]string{}
	for _, row := range near_list {
		meta := meta_map[row.Title]
		if meta == nil {
			meta = map[string]string{}
		}

		near_rows = append(near_rows, []string{
			tool.Baro_doc_link(row.Title),
			tool.HTML_escape(baro_find_person_line(meta)),
			tool.Baro_badge(row.Reason, "warn"),
		})
	}

	// 별칭 후보 : 검색어를 포함한 별칭
	alias_rows := [][]string{}
	for _, row := range baro_find_alias_scan(db) {
		if baro_find_excluded(row[0], hidden) {
			continue
		}

		alias_norm := baro_find_norm(row[1])
		if alias_norm == "" {
			continue
		}

		hit := strings.Contains(alias_norm, query.Norm) || strings.Contains(query.Norm, alias_norm)
		if !hit && chosung_key != "" {
			hit = strings.Contains(baro_find_norm(tool.Baro_chosung(row[1])), chosung_key)
		}
		if !hit {
			continue
		}

		alias_rows = append(alias_rows, []string{
			tool.HTML_escape(row[1]),
			tool.HTML_escape(baro_find_alias_kind_name(row[2])),
			tool.Baro_doc_link(row[0]),
		})

		if len(alias_rows) >= 10 {
			break
		}
	}

	body := tool.Baro_alert("warn", `<b>`+tool.HTML_escape(query.Raw)+tool.Baro_t(`</b> 에 해당하는 문서를 찾지 못했습니다. 빈손으로 돌려보내지 않기 위해 아래를 준비했습니다.`))

	body += tool.Baro_card(
		tool.Baro_t("혹시 이 문서를 찾으셨나요?"),
		tool.Baro_table([]string{"문서", "인물 정보", tool.Baro_t("비슷한 이유")}, near_rows, tool.Baro_t("비슷한 제목의 문서도 찾지 못했습니다."))+
			tool.Baro_note(tool.Baro_t("편집 거리(글자 몇 개를 고치면 같아지는지)와 초성으로 비슷한 제목을 고른 결과입니다.")),
	)

	body += tool.Baro_card(
		tool.Baro_t("별칭 후보"),
		tool.Baro_table([]string{"별칭", "종류", tool.Baro_t("연결된 문서")}, alias_rows, tool.Baro_t("이 이름으로 등록된 별칭이 없습니다."))+
			tool.Baro_note(tool.Baro_t("찾는 이름이 한자명·개명 전 이름·호라면 별칭으로 등록하면 다음부터 바로 찾을 수 있습니다.")),
	)

	next_title := query.Base
	if next_title == "" {
		next_title = query.Raw
	}

	actions := tool.Baro_button("/baro/task/new?title="+tool.Url_parser(next_title)+"&kind=create", "문서 작성 요청하기", "primary") + " "
	actions += tool.Baro_button("/edit/"+tool.Url_parser(next_title), "직접 새 문서 만들기", "") + " "
	actions += tool.Baro_button("/baro/task/new?title="+tool.Url_parser(next_title)+"&kind=project", "프로젝트 과제로 등록", "") + " "
	actions += tool.Baro_button("/baro/doc/alias?q="+tool.Url_parser(next_title), "별칭으로 등록하기", "")

	body += tool.Baro_card_ex(
		tool.Baro_t("이 기록을 만들어 주세요"),
		tool.Baro_t("없는 기록은 아직 아무도 쓰지 않은 기록입니다."),
		tool.Baro_list([]string{
			tool.Baro_t(`<b>문서 작성 요청</b> — 다른 기여자가 대신 써 줄 수 있게 할 일 목록에 올립니다.`),
			tool.Baro_t(`<b>직접 만들기</b> — 한 줄이라도 좋습니다. 출처만 함께 남겨 주세요.`),
			tool.Baro_t(`<b>프로젝트 과제 등록</b> — 같은 분야를 개척하는 프로젝트의 과제로 넣습니다.`),
			tool.Baro_t(`<b>별칭 등록</b> — 이미 다른 이름으로 문서가 있다면 별칭만 이어 주면 됩니다.`),
		}),
		actions,
	)

	return body
}

// View_baro_find_suggest_json : 자동 완성 JSON (/baro/search/suggest)
func View_baro_find_suggest_json(config tool.Config, raw string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = config

	raw = strings.TrimSpace(raw)
	out := []map[string]string{}

	if raw == "" {
		return tool.Baro_json_enc(map[string]any{"response": "ok", "data": out})
	}

	query := baro_find_parse_query(db, raw, "", "")
	hidden := baro_find_hidden_set(db)
	meta_map := baro_find_meta_map(db)

	hits := baro_find_collect(db, query)
	sort.SliceStable(hits, func(a int, b int) bool {
		if hits[a].Score != hits[b].Score {
			return hits[a].Score > hits[b].Score
		}

		return hits[a].Title < hits[b].Title
	})

	seen := map[string]bool{}
	for _, hit := range hits {
		if baro_find_excluded(hit.Title, hidden) || seen[hit.Title] {
			continue
		}

		seen[hit.Title] = true

		note := baro_find_person_line(meta_map[hit.Title])
		if hit.Alias != "" {
			if note != "" {
				note += " · "
			}
			note += "별칭 " + hit.Alias
		}

		out = append(out, map[string]string{
			"title": tool.Namespace_display(hit.Title),
			"raw":   hit.Title,
			"note":  note,
			"url":   "/w/" + tool.Doc_url(hit.Title),
		})

		if len(out) >= 10 {
			break
		}
	}

	return tool.Baro_json_enc(map[string]any{"response": "ok", "data": out})
}
