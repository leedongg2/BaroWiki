package route

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  12-4 연말 Wrapped — /baro/wrapped
//
//    /baro/wrapped        내 올해 결산
//    /baro/wrapped/<연도>  고른 해의 내 결산
//    /baro/wrapped/site   사이트 전체 결산
//    /baro/wrapped/list   저장된 결산 목록
//
//  baro_contrib_event · baro_calendar · baro_impact 를 모아
//  "올해 214개 문서를 편집했습니다" 같은 문장으로 보여 주고
//  결과를 baro_wrapped 에 JSON 으로 저장해 다시 볼 수 있게 한다.
// ============================================================

// baro_award_wrap : 한 해 결산 수치
type baro_award_wrap struct {
	Year        string
	Events      int
	Score       int
	Docs        int
	NewDoc      int
	Person      int
	Source      int
	File        int
	Review      int
	Fix         int
	Expand      int
	Discussion  int
	ProjectNum  int
	Days        int
	Acts        int
	Reads       int
	Thanks      int
	Badges      int
	TopMonth    string
	TopMonthNum int
	FieldTop    string
	FieldTopNum int
	People      int
	NewUser     int
	Citation    int
	ProjectDone int
	ReviewDone  int
}

// baro_award_wrapped_css : 결산 화면 전용 스타일 (이 화면 안에서만 쓴다)
func baro_award_wrapped_css() string {
	return `<style id="bw_award_wrapped_css">
.bw_wrapped_n{font-size:2.5rem;font-weight:800;line-height:1.1;letter-spacing:-.02em;}
.bw_wrapped_s{font-size:1.05rem;font-weight:600;margin-top:.35rem;}
.bw_wrapped_h{font-size:.82rem;font-weight:700;letter-spacing:.08em;text-transform:uppercase;opacity:.65;}
@media (max-width:640px){.bw_wrapped_n{font-size:1.85rem;}}
</style>`
}

// baro_award_wrap_card : 큰 숫자와 문장을 담은 공유용 카드 (bw_share)
func baro_award_wrap_card(head string, number string, sentence string, sub string) string {
	html := `<div class="bw_share">`
	if head != "" {
		html += `<div class="bw_wrapped_h">` + tool.HTML_escape(head) + `</div>`
	}

	html += `<div class="bw_wrapped_n">` + tool.HTML_escape(number) + `</div>`
	html += `<div class="bw_wrapped_s">` + tool.HTML_escape(sentence) + `</div>`

	if sub != "" {
		html += `<p class="bw_note">` + tool.HTML_escape(sub) + `</p>`
	}

	return html + `</div>`
}

// baro_award_year_ok : 연도 네 자리인지
func baro_award_year_ok(year string) bool {
	if tool.Get_len(year) != 4 {
		return false
	}

	num := tool.Str_to_int(year)

	return num >= 2000 && num <= 2999
}

// baro_award_year_fix : 연도 다듬기 (이상하면 올해)
func baro_award_year_fix(year string) string {
	year = strings.TrimSpace(year)
	if !baro_award_year_ok(year) {
		return baro_award_this_year()
	}

	return year
}

// baro_award_read_sum : 읽힌 횟수 지표 합계
func baro_award_read_sum(db *sql.DB, user string) int {
	total := 0

	query := "select data from baro_impact where name in ('read', 'reads', 'view', 'views', 'pageview')"
	args := []any{}

	if user != "" {
		query += " and user = ?"
		args = append(args, user)
	}

	query += " limit 200000"

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	for rows.Next() {
		data := ""
		if err := rows.Scan(&data); err != nil {
			continue
		}

		total += tool.Str_to_int(data)
	}

	return total
}

// baro_award_calendar_sum : 활동 캘린더에서 활동한 날 수와 활동 수, 가장 활발했던 달
func baro_award_calendar_sum(db *sql.DB, user string, year string) (int, int, string, int) {
	days, acts := 0, 0
	months := map[string]int{}

	rows := tool.Query_DB(
		db,
		"select date, num from baro_calendar where user = ? and date like ? limit 400",
		user, year+"%",
	)
	defer rows.Close()

	for rows.Next() {
		date, num := "", "0"
		if err := rows.Scan(&date, &num); err != nil {
			continue
		}

		days++
		acts += tool.Str_to_int(num)
		months[tool.Get_slice(date, 0, 7)] += tool.Str_to_int(num)
	}

	top, top_num := "", 0
	keys := []string{}
	for key := range months {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if months[key] > top_num {
			top = key
			top_num = months[key]
		}
	}

	return days, acts, top, top_num
}

// baro_award_wrap_user : 한 사람의 한 해 결산 계산
func baro_award_wrap_user(db *sql.DB, user string, year string) baro_award_wrap {
	out := baro_award_wrap{Year: year}

	agg := baro_award_user_agg(db, user, year)
	out.Events = agg.Events
	out.Score = agg.Score
	out.Docs = len(agg.Docs)
	out.NewDoc = agg.Kinds["new_document"]
	out.Source = agg.Kinds["source"]
	out.File = agg.Kinds["file"]
	out.Review = agg.Kinds["review"]
	out.Fix = agg.Kinds["fix"]
	out.Expand = agg.Kinds["expand"]
	out.Discussion = agg.Kinds["discussion"]
	out.ProjectNum = agg.ProjectNum

	keys := []string{}
	for key := range agg.FieldNum {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if agg.FieldNum[key] > out.FieldTopNum {
			out.FieldTop = key
			out.FieldTopNum = agg.FieldNum[key]
		}
	}

	tool.QueryRow_DB(
		db,
		"select count(*) from baro_contrib_event where user = ? and date like ? and kind = 'new_document' and field = 'person'",
		[]any{&out.Person}, user, year+"%",
	)
	tool.QueryRow_DB(
		db,
		"select count(*) from baro_thanks where to_user = ? and date like ?",
		[]any{&out.Thanks}, user, year+"%",
	)
	tool.QueryRow_DB(
		db,
		"select count(*) from baro_user_badge where user = ? and date like ?",
		[]any{&out.Badges}, user, year+"%",
	)

	out.Days, out.Acts, out.TopMonth, out.TopMonthNum = baro_award_calendar_sum(db, user, year)
	out.Reads = baro_award_read_sum(db, user)

	return out
}

// baro_award_wrap_site : 사이트 전체의 한 해 결산 계산
func baro_award_wrap_site(db *sql.DB, year string) baro_award_wrap {
	out := baro_award_wrap{Year: year}

	agg := baro_award_collect(db, year)
	docs := map[string]bool{}

	for _, row := range agg {
		out.Events += row.Events
		out.Score += row.Score
		out.NewDoc += row.Kinds["new_document"]
		out.Source += row.Kinds["source"]
		out.File += row.Kinds["file"]
		out.Review += row.Kinds["review"]
		out.Fix += row.Kinds["fix"]
		out.Expand += row.Kinds["expand"]
		out.Discussion += row.Kinds["discussion"]
		out.ProjectNum += row.ProjectNum

		for title := range row.Docs {
			docs[title] = true
		}
	}

	out.People = len(agg)
	out.Docs = len(docs)

	first := baro_award_first_map(db)
	for _, date := range first {
		if strings.HasPrefix(date, year) {
			out.NewUser++
		}
	}

	tool.QueryRow_DB(
		db,
		"select count(*) from baro_contrib_event where date like ? and kind = 'new_document' and field = 'person'",
		[]any{&out.Person}, year+"%",
	)
	tool.QueryRow_DB(db, "select count(*) from baro_citation where date like ?", []any{&out.Citation}, year+"%")
	tool.QueryRow_DB(
		db,
		"select count(*) from baro_project where status = 'done' and (end_date like ? or date like ?)",
		[]any{&out.ProjectDone}, year+"%", year+"%",
	)
	tool.QueryRow_DB(db, "select count(*) from baro_review where review_date like ?", []any{&out.ReviewDone}, year+"%")
	tool.QueryRow_DB(db, "select count(distinct date) from baro_calendar where date like ?", []any{&out.Days}, year+"%")
	tool.QueryRow_DB(db, "select count(*) from baro_thanks where date like ?", []any{&out.Thanks}, year+"%")

	out.Reads = baro_award_read_sum(db, "")

	return out
}

// baro_award_wrap_json : 결산 수치를 저장용 JSON 으로
func baro_award_wrap_json(wrap baro_award_wrap) string {
	return tool.Baro_json_enc(map[string]string{
		"year":          wrap.Year,
		"events":        tool.Baro_itoa(wrap.Events),
		"score":         tool.Baro_itoa(wrap.Score),
		"docs":          tool.Baro_itoa(wrap.Docs),
		"new_document":  tool.Baro_itoa(wrap.NewDoc),
		"person":        tool.Baro_itoa(wrap.Person),
		"source":        tool.Baro_itoa(wrap.Source),
		"file":          tool.Baro_itoa(wrap.File),
		"review":        tool.Baro_itoa(wrap.Review),
		"fix":           tool.Baro_itoa(wrap.Fix),
		"expand":        tool.Baro_itoa(wrap.Expand),
		"discussion":    tool.Baro_itoa(wrap.Discussion),
		"project":       tool.Baro_itoa(wrap.ProjectNum),
		"days":          tool.Baro_itoa(wrap.Days),
		"acts":          tool.Baro_itoa(wrap.Acts),
		"reads":         tool.Baro_itoa(wrap.Reads),
		"thanks":        tool.Baro_itoa(wrap.Thanks),
		"badges":        tool.Baro_itoa(wrap.Badges),
		"top_month":     wrap.TopMonth,
		"top_month_num": tool.Baro_itoa(wrap.TopMonthNum),
		"field_top":     wrap.FieldTop,
		"people":        tool.Baro_itoa(wrap.People),
		"new_user":      tool.Baro_itoa(wrap.NewUser),
		"citation":      tool.Baro_itoa(wrap.Citation),
		"project_done":  tool.Baro_itoa(wrap.ProjectDone),
		"review_done":   tool.Baro_itoa(wrap.ReviewDone),
	})
}

// baro_award_wrapped_save : 결산 결과 저장 (다시 볼 수 있게)
func baro_award_wrapped_save(db *sql.DB, user string, year string, wrap baro_award_wrap) {
	if tool.Baro_read_only(db) {
		return
	}

	tool.Exec_DB(db, "delete from baro_wrapped where user = ? and year_data = ?", user, year)
	tool.Exec_DB(
		db,
		"insert into baro_wrapped (id, user, year_data, data, date) values (?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_wrapped"), user, year, baro_award_wrap_json(wrap), tool.Baro_now(),
	)
}

// baro_award_wrapped_years : 저장된 결산 연도 목록
func baro_award_wrapped_years(db *sql.DB, user string) []string {
	out := []string{}

	rows := tool.Query_DB(
		db,
		"select year_data from baro_wrapped where user = ? group by year_data order by year_data desc limit 30",
		user,
	)
	defer rows.Close()

	for rows.Next() {
		year := ""
		if err := rows.Scan(&year); err != nil {
			continue
		}

		out = append(out, year)
	}

	return out
}

// baro_award_year_pills : 연도 고르기 줄
func baro_award_year_pills(db *sql.DB, user string, year string, base string) string {
	years := map[string]bool{baro_award_this_year(): true, year: true}
	for _, item := range baro_award_wrapped_years(db, user) {
		years[item] = true
	}

	now := tool.Str_to_int(baro_award_this_year())
	for step := 0; step < 3; step++ {
		years[strconv.Itoa(now-step)] = true
	}

	keys := []string{}
	for key := range years {
		if baro_award_year_ok(key) {
			keys = append(keys, key)
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(keys)))

	items := [][]string{}
	for _, key := range keys {
		items = append(items, []string{base + key, key + tool.Baro_t("년")})
	}

	return tool.Baro_tabs(items, base+year)
}

// baro_award_share_link : 공유 카드 만들기 링크 (카드 화면은 다른 모듈이 만든다)
func baro_award_share_link(kind string, year string, user string) string {
	link := "/baro/card/make?kind=" + tool.Url_parser(kind) + "&year=" + tool.Url_parser(year)
	if user != "" {
		link += "&user=" + tool.Url_parser(user)
	}

	return link
}

// ------------------------------------------------------------
//  내 결산
// ------------------------------------------------------------

// View_baro_award_wrapped : /baro/wrapped — 내 올해 결산
func View_baro_award_wrapped(config tool.Config, year string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	year = baro_award_year_fix(year)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/wrapped"); need {
		return wall
	}

	wrap := baro_award_wrap_user(db, config.IP, year)
	baro_award_wrapped_save(db, config.IP, year, wrap)

	body := baro_award_wrapped_css()
	body += baro_award_tabs("/baro/wrapped")
	body += baro_award_year_pills(db, config.IP, year, "/baro/wrapped/")

	body += tool.Baro_hero(
		year+tool.Baro_t(" 결산"),
		tool.HTML_escape(config.IP)+tool.Baro_t("님이 올해 남긴 기록입니다."),
		tool.Baro_note(tool.Baro_t("모든 수치는 기여 이벤트·활동 캘린더·영향력 지표에서 그대로 계산합니다. 남과 견주기 위한 순위표가 아니라 내 기록을 돌아보기 위한 화면입니다.")),
	)

	if wrap.Events == 0 {
		body += tool.Baro_empty(year + tool.Baro_t("년에는 아직 기록이 없습니다. 한 글자만 고쳐도 이 화면이 채워집니다."))
		body += tool.Baro_card(tool.Baro_t("무엇부터 할까요"),
			tool.Baro_button("/baro/task", tool.Baro_t("할 일 찾기"), "primary")+" "+
				tool.Baro_button("/baro/field", tool.Baro_t("분야 둘러보기"), "")+" "+
				tool.Baro_button("/baro/wrapped/site", tool.Baro_t("사이트 결산 보기"), ""),
		)

		return tool.Baro_page(db, config, year+tool.Baro_t(" 결산"), body, baro_award_menu())
	}

	// --- 큰 숫자 카드 ---
	cards := []string{
		baro_award_wrap_card("문서", tool.Baro_comma(wrap.Docs)+tool.Baro_t("개"),
			tool.Baro_t("올해 ")+tool.Baro_comma(wrap.Docs)+tool.Baro_t("개 문서를 편집했습니다."),
			tool.Baro_t("편집한 문서를 겹치지 않게 센 수입니다.")),
		baro_award_wrap_card(tool.Baro_t("새 문서"), tool.Baro_comma(wrap.NewDoc)+tool.Baro_t("개"),
			tool.Baro_comma(wrap.NewDoc)+tool.Baro_t("개의 문서를 세상에 처음 내놓았습니다."),
			tool.Baro_t("아무도 쓰지 않았던 문서를 만든 횟수입니다.")),
		baro_award_wrap_card(tool.Baro_t("인물"), tool.Baro_comma(wrap.Person)+tool.Baro_t("명"),
			tool.Baro_comma(wrap.Person)+tool.Baro_t("명의 인물을 처음 기록했습니다."),
			tool.Baro_t("인물 분야로 분류된 새 문서를 센 수입니다.")),
		baro_award_wrap_card(tool.Baro_t("읽힘"), tool.Baro_comma(wrap.Reads)+tool.Baro_t("번"),
			tool.Baro_t("당신의 기여는 ")+tool.Baro_comma(wrap.Reads)+tool.Baro_t("번 읽혔습니다."),
			tool.Baro_t("영향력 지표에 쌓인 조회 수입니다.")),
		baro_award_wrap_card("출처", tool.Baro_comma(wrap.Source)+tool.Baro_t("건"),
			tool.Baro_comma(wrap.Source)+tool.Baro_t("건의 출처를 붙였습니다."),
			tool.Baro_t("근거가 붙은 문서는 오래 살아남습니다.")),
		baro_award_wrap_card(tool.Baro_t("함께한 날"), tool.Baro_comma(wrap.Days)+tool.Baro_t("일"),
			year+tool.Baro_t("년 중 ")+tool.Baro_comma(wrap.Days)+tool.Baro_t("일 동안 기록을 남겼습니다."),
			tool.Baro_t("연속 기록을 재촉하지 않습니다. 쉰 날도 괜찮습니다.")),
	}

	body += tool.Baro_grid(cards, 3)

	// --- 자세한 수치 ---
	body += tool.Baro_card(year+" 자세히", tool.Baro_stat_row([][]string{
		{tool.Baro_t("기여 건수"), tool.Baro_comma(wrap.Events), tool.Baro_t("행동 종류로 분류된 기록")},
		{tool.Baro_t("기여 점수"), tool.Baro_comma(wrap.Score), tool.Baro_t("확정 점수 합계")},
		{tool.Baro_t("본문 보강"), tool.Baro_comma(wrap.Expand), tool.Baro_t("이미 있던 문서를 키운 횟수")},
		{tool.Baro_t("오류 수정"), tool.Baro_comma(wrap.Fix), tool.Baro_t("틀린 곳을 바로잡은 횟수")},
		{tool.Baro_t("문서 검토"), tool.Baro_comma(wrap.Review), tool.Baro_t("남의 편집을 살펴본 횟수")},
		{tool.Baro_t("파일"), tool.Baro_comma(wrap.File), tool.Baro_t("올린 사진·자료")},
		{tool.Baro_t("토론 해결"), tool.Baro_comma(wrap.Discussion), tool.Baro_t("이야기를 매듭지은 횟수")},
		{tool.Baro_t("프로젝트 기여"), tool.Baro_comma(wrap.ProjectNum), tool.Baro_t("프로젝트에 묶인 기여")},
		{tool.Baro_t("받은 감사"), tool.Baro_comma(wrap.Thanks), tool.Baro_t("다른 사람이 보낸 감사")},
		{"배지", tool.Baro_comma(wrap.Badges), tool.Baro_t("올해 받은 배지")},
	}))

	extra := [][]string{}
	if wrap.TopMonth != "" {
		extra = append(extra, []string{tool.Baro_t("가장 활발했던 달"), tool.HTML_escape(wrap.TopMonth) + " (" + tool.Baro_comma(wrap.TopMonthNum) + tool.Baro_t("회)")})
	}
	if wrap.FieldTop != "" {
		extra = append(extra, []string{tool.Baro_t("가장 많이 손댄 분야"), tool.HTML_escape(tool.Baro_field_name(db, wrap.FieldTop)) + " (" + tool.Baro_comma(wrap.FieldTopNum) + "건)"})
	}
	extra = append(extra, []string{tool.Baro_t("총 활동 수"), tool.Baro_comma(wrap.Acts) + tool.Baro_t("회")})

	if number := tool.Baro_founder_number(db, config.IP); number != "" {
		extra = append(extra, []string{tool.Baro_t("창립 기여자 번호"), "#" + tool.HTML_escape(number)})
	}

	body += tool.Baro_card(tool.Baro_t("올해의 나"), tool.Baro_kv_grid(extra))

	// --- 공유 · 저장 ---
	body += tool.Baro_card_ex(
		tool.Baro_t("공유하기"),
		tool.Baro_t("결산 결과는 baro_wrapped 에 저장되어 언제든 다시 볼 수 있습니다"),
		tool.Baro_button(baro_award_share_link("wrapped", year, config.IP), tool.Baro_t("공유 카드 만들기"), "primary")+" "+
			tool.Baro_button("/baro/wrapped/list", tool.Baro_t("저장된 결산 목록"), "")+" "+
			tool.Baro_button("/baro/wrapped/site", tool.Baro_t("사이트 결산"), "")+" "+
			tool.Baro_button("/baro/award", tool.Baro_t("어워드 보기"), ""),
		tool.Baro_note(tool.Baro_t("공유 카드에는 이 화면의 숫자만 담기며 IP·이메일 같은 개인정보는 들어가지 않습니다.")),
	)

	return tool.Baro_page(db, config, year+tool.Baro_t(" 결산"), body, baro_award_menu())
}

// ------------------------------------------------------------
//  사이트 결산
// ------------------------------------------------------------

// View_baro_award_wrapped_site : /baro/wrapped/site — 사이트 전체 결산
func View_baro_award_wrapped_site(config tool.Config, year string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	year = baro_award_year_fix(year)

	wrap := baro_award_wrap_site(db, year)
	baro_award_wrapped_save(db, "@site", year, wrap)

	body := baro_award_wrapped_css()
	body += baro_award_tabs("/baro/wrapped/site")
	body += baro_award_year_pills(db, "@site", year, "/baro/wrapped/site?year=")

	body += tool.Baro_hero(
		year+tool.Baro_t(" 사이트 결산"),
		tool.Baro_t("바로위키가 올해 함께 만든 기록입니다."),
		tool.Baro_note(tool.Baro_t("모든 수치는 공개 기록에서 계산합니다. 개인을 특정할 수 있는 정보는 담지 않습니다.")),
	)

	if wrap.Events == 0 {
		body += tool.Baro_empty(year + tool.Baro_t("년에는 아직 집계할 기록이 없습니다. 다른 해를 골라 보세요."))
		body += tool.Baro_button("/baro/wrapped", tool.Baro_t("내 결산 보기"), "primary")

		return tool.Baro_page(db, config, year+tool.Baro_t(" 사이트 결산"), body, baro_award_menu())
	}

	cards := []string{
		baro_award_wrap_card(tool.Baro_t("인물"), tool.Baro_comma(wrap.Person)+tool.Baro_t("명"),
			tool.Baro_t("올해 ")+tool.Baro_comma(wrap.Person)+tool.Baro_t("명의 인물이 새로 기록됐습니다."),
			tool.Baro_t("인물 분야로 분류된 새 문서입니다.")),
		baro_award_wrap_card("출처", tool.Baro_comma(wrap.Citation)+tool.Baro_t("개"),
			tool.Baro_comma(wrap.Citation)+tool.Baro_t("개의 출처가 추가됐습니다."),
			tool.Baro_t("근거가 붙은 문서는 오래 살아남습니다.")),
		baro_award_wrap_card(tool.Baro_t("새 문서"), tool.Baro_comma(wrap.NewDoc)+tool.Baro_t("개"),
			tool.Baro_comma(wrap.NewDoc)+tool.Baro_t("개의 문서가 새로 만들어졌습니다."),
			tool.Baro_t("올해 처음 쓰인 문서입니다.")),
		baro_award_wrap_card(tool.Baro_t("사람"), tool.Baro_comma(wrap.NewUser)+tool.Baro_t("명"),
			tool.Baro_comma(wrap.NewUser)+tool.Baro_t("명이 올해 첫 기여를 남겼습니다."),
			tool.Baro_t("올해 처음 기록을 남긴 사람입니다.")),
		baro_award_wrap_card(tool.Baro_t("읽힘"), tool.Baro_comma(wrap.Reads)+tool.Baro_t("번"),
			tool.Baro_t("우리가 쓴 글은 ")+tool.Baro_comma(wrap.Reads)+tool.Baro_t("번 읽혔습니다."),
			tool.Baro_t("영향력 지표에 쌓인 조회 수 합계입니다.")),
		baro_award_wrap_card("검토", tool.Baro_comma(wrap.ReviewDone)+tool.Baro_t("건"),
			tool.Baro_comma(wrap.ReviewDone)+tool.Baro_t("건의 검토가 처리됐습니다."),
			tool.Baro_t("검토는 눈에 잘 띄지 않지만 문서를 지켜 줍니다.")),
	}

	body += tool.Baro_grid(cards, 3)

	body += tool.Baro_card(year+" 자세히", tool.Baro_stat_row([][]string{
		{tool.Baro_t("기여자"), tool.Baro_comma(wrap.People), tool.Baro_t("올해 기록을 남긴 사람")},
		{tool.Baro_t("기여 건수"), tool.Baro_comma(wrap.Events), tool.Baro_t("행동 종류로 분류된 기록")},
		{tool.Baro_t("기여 점수"), tool.Baro_comma(wrap.Score), tool.Baro_t("확정 점수 합계")},
		{tool.Baro_t("편집된 문서"), tool.Baro_comma(wrap.Docs), tool.Baro_t("겹치지 않게 센 수")},
		{tool.Baro_t("본문 보강"), tool.Baro_comma(wrap.Expand), tool.Baro_t("문서를 키운 횟수")},
		{tool.Baro_t("파일"), tool.Baro_comma(wrap.File), tool.Baro_t("올라온 사진·자료")},
		{tool.Baro_t("마친 프로젝트"), tool.Baro_comma(wrap.ProjectDone), tool.Baro_t("완료된 프로젝트")},
		{tool.Baro_t("오간 감사"), tool.Baro_comma(wrap.Thanks), tool.Baro_t("서로에게 보낸 감사")},
		{tool.Baro_t("기록이 있던 날"), tool.Baro_comma(wrap.Days), tool.Baro_t("누군가 기여한 날")},
	}))

	body += tool.Baro_card_ex(
		tool.Baro_t("공유하기"),
		tool.Baro_t("사이트 결산도 baro_wrapped 에 저장됩니다"),
		tool.Baro_button(baro_award_share_link("wrapped_site", year, ""), tool.Baro_t("공유 카드 만들기"), "primary")+" "+
			tool.Baro_button("/baro/wrapped", tool.Baro_t("내 결산"), "")+" "+
			tool.Baro_button("/baro/wrapped/list", tool.Baro_t("저장된 결산 목록"), "")+" "+
			tool.Baro_button("/baro/award", tool.Baro_t("어워드 보기"), ""),
		"",
	)

	return tool.Baro_page(db, config, year+tool.Baro_t(" 사이트 결산"), body, baro_award_menu())
}

// ------------------------------------------------------------
//  저장된 결산 목록
// ------------------------------------------------------------

// View_baro_award_wrapped_list : /baro/wrapped/list — 저장된 결산 다시 보기
func View_baro_award_wrapped_list(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_int := tool.Str_to_int(page)
	if page_int < 1 {
		page_int = 1
	}

	page_size := tool.Baro_page_size
	offset := (page_int - 1) * page_size

	me := config.IP
	if !tool.Baro_is_user(me) {
		me = "\x00"
	}

	rows := tool.Query_DB(
		db,
		"select id, user, year_data, date from baro_wrapped where user = ? or user = '@site' order by id + 0 desc limit "+
			strconv.Itoa(page_size)+" offset "+strconv.Itoa(offset),
		me,
	)
	defer rows.Close()

	table := [][]string{}
	count := 0

	for rows.Next() {
		id, user, year, date := "", "", "", ""
		if err := rows.Scan(&id, &user, &year, &date); err != nil {
			continue
		}

		count++

		who := tool.Baro_badge(tool.Baro_t("사이트 전체"), "info")
		link := "/baro/wrapped/site?year=" + tool.Url_parser(year)
		if user != "@site" {
			who = tool.Baro_user_link(db, user, config.IP)
			link = "/baro/wrapped/" + tool.Url_parser(year)
		}

		table = append(table, []string{
			tool.HTML_escape(year) + tool.Baro_t("년"),
			who,
			tool.Baro_time_cell(date),
			tool.Baro_button(link, tool.Baro_t("다시 보기"), ""),
		})
	}

	body := baro_award_tabs("/baro/wrapped")
	body += tool.Baro_hero(tool.Baro_t("저장된 결산"), tool.Baro_t("한 번 계산한 결산은 baro_wrapped 에 남아 언제든 다시 볼 수 있습니다."), "")

	list_body := tool.Baro_table([]string{tool.Baro_t("연도"), tool.Baro_t("대상"), tool.Baro_t("저장 시각"), "보기"}, table, "")
	if count == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("저장된 결산이 없습니다. 결산 화면을 한 번 열면 자동으로 저장됩니다."))
	}

	list_body += tool.Baro_paging(page_int, count, page_size, "/baro/wrapped/list?page={}")
	body += tool.Baro_card(tool.Baro_t("결산 목록"), list_body)

	body += tool.Baro_card(tool.Baro_t("바로 가기"),
		tool.Baro_button("/baro/wrapped", tool.Baro_t("내 올해 결산"), "primary")+" "+
			tool.Baro_button("/baro/wrapped/site", tool.Baro_t("사이트 결산"), "")+" "+
			tool.Baro_button("/baro/award", tool.Baro_t("어워드"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("저장된 결산"), body, baro_award_menu())
}
