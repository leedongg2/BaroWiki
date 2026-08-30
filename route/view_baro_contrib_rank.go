package route

import (
	"database/sql"
	"net/url"

	"opennamu/route/tool"
)

// ============================================================
//  7-5 분야별 순위 — /baro/rank
//
//  전체 생태계를 한 줄로 세우는 통합 랭킹은 만들지 않는다.
//  역사 기여자와 게임 기여자를 하나의 점수로 비교하지 않기 위해서다.
//  기간 · 종류 · 분야로 잘게 나눈 순위만 제공한다.
// ============================================================

// baro_contrib_rank_period_list : 기간 (키, 이름, 일 수)
func baro_contrib_rank_period_list() [][]string {
	return [][]string{
		{"week", tool.Baro_t("주간"), "7"},
		{"month", tool.Baro_t("월간"), "30"},
		{"year", tool.Baro_t("연간"), "365"},
	}
}

// baro_contrib_rank_kind_list : 순위 종류 (키, 이름, 설명)
func baro_contrib_rank_kind_list() [][]string {
	return [][]string{
		{"new_contributor", tool.Baro_t("신규 기여자"), tool.Baro_t("이 기간에 처음 기여를 시작한 사람들입니다.")},
		{"new_document", tool.Baro_t("신규 문서"), tool.Baro_t("이 기간에 새 문서를 만든 기여입니다.")},
		{"source", tool.Baro_t("출처 보강"), tool.Baro_t("이 기간에 출처를 붙인 기여입니다.")},
		{"review", tool.Baro_t("검토 기여"), tool.Baro_t("이 기간에 다른 사람의 편집을 검토한 기여입니다.")},
	}
}

// baro_contrib_rank_period_days : 기간 키를 일 수로
func baro_contrib_rank_period_days(period string) int {
	for _, row := range baro_contrib_rank_period_list() {
		if row[0] == period {
			return tool.Str_to_int(row[2])
		}
	}

	return 7
}

// baro_contrib_rank_period_name : 기간 표시 이름
func baro_contrib_rank_period_name(period string) string {
	for _, row := range baro_contrib_rank_period_list() {
		if row[0] == period {
			return row[1]
		}
	}

	return period
}

// baro_contrib_rank_kind_name : 순위 종류 표시 이름
func baro_contrib_rank_kind_name(kind string) string {
	for _, row := range baro_contrib_rank_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// baro_contrib_rank_kind_desc : 순위 종류 설명
func baro_contrib_rank_kind_desc(kind string) string {
	for _, row := range baro_contrib_rank_kind_list() {
		if row[0] == kind {
			return row[2]
		}
	}

	return ""
}

// baro_contrib_rank_period_ok : 알 수 없는 기간을 걸러낸다
func baro_contrib_rank_period_ok(period string) bool {
	for _, row := range baro_contrib_rank_period_list() {
		if row[0] == period {
			return true
		}
	}

	return false
}

// baro_contrib_rank_kind_ok : 알 수 없는 종류를 걸러낸다
func baro_contrib_rank_kind_ok(kind string) bool {
	for _, row := range baro_contrib_rank_kind_list() {
		if row[0] == kind {
			return true
		}
	}

	return false
}

// baro_contrib_rank_entry : 순위 한 줄
type baro_contrib_rank_entry struct {
	User  string
	Score int
	No    int
}

// baro_contrib_rank_calc : 즉석 계산 (저장된 값이 없을 때 쓴다)
func baro_contrib_rank_calc(db *sql.DB, period string, kind string, field string, want int) []baro_contrib_rank_entry {
	days := baro_contrib_rank_period_days(period)
	since := tool.Baro_time_add(tool.Baro_now(), -days, 0, 0)

	where := " where e.status <> 'rejected' and e.date >= ?"
	args := []any{since}

	if field != "" {
		// 이벤트에 굳어 있는 분야 대신 문서의 지금 분야로 거른다.
		// 문서에 분야를 나중에 지정해도 예전 기여가 분야별 순위에 잡히게 하기 위해서다.
		where += " and " + baro_contrib_field_expr("e") + " = ?"
		args = append(args, field)
	}

	value_expr := "count(*)"

	switch kind {
	case "new_document":
		where += " and e.kind = 'new_document'"
	case "source":
		where += " and e.kind = 'source'"
	case "review":
		where += " and e.kind = 'review'"
	case "new_contributor":
		// 이 기간에 처음 기여를 시작한 사람만
		where += " and not exists (select 1 from baro_contrib_event f where f.user = e.user and f.date < ?)"
		args = append(args, since)
		value_expr = "coalesce(sum(e.weight + 0), 0)"
	}

	// 순위에서 빠지길 원한 사람을 걸러내기 위해 넉넉히 가져온다
	fetch := want * 4
	if fetch < 40 {
		fetch = 40
	}

	list_args := append(append([]any{}, args...), 0, fetch)

	rows := tool.Query_DB(
		db,
		"select e.user, "+value_expr+" as sc from baro_contrib_event e"+where+
			" group by e.user order by sc desc limit ?, ?",
		list_args...,
	)
	defer rows.Close()

	out := []baro_contrib_rank_entry{}
	for rows.Next() {
		user, score := "", 0
		if err := rows.Scan(&user, &score); err != nil {
			continue
		}

		if user == "" || score <= 0 {
			continue
		}

		// 사람 단위 기여만 순위에 넣는다 (IP 편집과 위키 내부 계정은 뺀다)
		if !baro_contrib_is_person(user) {
			continue
		}

		// rank_public 이 off 인 사용자는 제외한다
		if !baro_contrib_rank_public(db, user) {
			continue
		}

		out = append(out, baro_contrib_rank_entry{User: user, Score: score, No: len(out) + 1})
		if len(out) >= want {
			break
		}
	}

	return out
}

// baro_contrib_rank_load : 저장된 순위 읽기
func baro_contrib_rank_load(db *sql.DB, period string, kind string, field string, want int) ([]baro_contrib_rank_entry, string) {
	rows := tool.Query_DB(
		db,
		"select user, score, date from baro_rank where period = ? and kind = ? and field = ? order by rank_no + 0 asc limit ?, ?",
		period, kind, field, 0, want,
	)
	defer rows.Close()

	out := []baro_contrib_rank_entry{}
	saved_at := ""

	for rows.Next() {
		user, score, date := "", "", ""
		if err := rows.Scan(&user, &score, &date); err != nil {
			continue
		}

		// 예전에 저장해 둔 값에 사람이 아닌 계정이 섞여 있으면 지금 시점에서 걸러낸다
		if !baro_contrib_is_person(user) {
			continue
		}

		// 저장 뒤에 순위 공개를 끈 사람은 지금 시점에서 다시 걸러낸다
		if !baro_contrib_rank_public(db, user) {
			continue
		}

		// 저장해 둔 rank_no 를 그대로 쓰면 걸러낸 사람 자리에 번호 구멍이 생기므로
		// 남은 사람들만으로 다시 1번부터 매긴다. (저장 순서는 그대로 지킨다)
		saved_at = date
		out = append(out, baro_contrib_rank_entry{User: user, Score: tool.Str_to_int(score), No: len(out) + 1})
	}

	return out, saved_at
}

// baro_contrib_rank_store : 계산 결과를 baro_rank 에 저장
func baro_contrib_rank_store(db *sql.DB, period string, kind string, field string, list []baro_contrib_rank_entry) {
	tool.Exec_DB(db, "delete from baro_rank where period = ? and kind = ? and field = ?", period, kind, field)

	now := tool.Baro_now()

	for _, row := range list {
		tool.Exec_DB(
			db,
			"insert into baro_rank (id, period, kind, field, user, score, rank_no, date) values (?, ?, ?, ?, ?, ?, ?, ?)",
			tool.Baro_next_id(db, "baro_rank"), period, kind, field, row.User,
			tool.Baro_itoa(row.Score), tool.Baro_itoa(row.No), now,
		)
	}
}

// baro_contrib_rank_table : 순위 표 HTML
func baro_contrib_rank_table(db *sql.DB, list []baro_contrib_rank_entry, my_ip string, unit string) string {
	if len(list) == 0 {
		return tool.Baro_empty(tool.Baro_t("이 조건으로 집계된 기여가 없습니다. 아직 이 분야에 기록이 쌓이지 않았을 수 있습니다."))
	}

	rows := [][]string{}
	for _, row := range list {
		mark := ""
		if row.User == my_ip {
			mark = " " + tool.Baro_badge(tool.Baro_t("나"), "info")
		}

		rows = append(rows, []string{
			tool.Baro_itoa(row.No),
			tool.Baro_user_link(db, row.User, my_ip) + mark,
			tool.Baro_comma(row.Score) + tool.Baro_t(unit),
			tool.Baro_button("/baro/contrib/user/"+tool.Url_parser(row.User), "기여 보기", ""),
		})
	}

	return tool.Baro_table([]string{tool.Baro_t("순위"), tool.Baro_t("기여자"), tool.Baro_t("값"), ""}, rows, tool.Baro_t("집계된 기여가 없습니다."))
}

// View_baro_contrib_rank : 분야별 순위 화면
func View_baro_contrib_rank(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	period := baro_contrib_get(query, "period")
	if !baro_contrib_rank_period_ok(period) {
		period = "week"
	}

	kind := baro_contrib_get(query, "kind")
	if !baro_contrib_rank_kind_ok(kind) {
		kind = "new_document"
	}

	field := baro_contrib_get(query, "field")

	field_ok := field == ""
	for _, row := range tool.Baro_field_list(db) {
		if row.Key == field {
			field_ok = true
			break
		}
	}
	if !field_ok {
		field = ""
	}

	want := 20

	list, saved_at := baro_contrib_rank_load(db, period, kind, field, want)
	source_text := tool.Baro_t("저장된 순위 (") + saved_at + ")"

	if len(list) == 0 {
		list = baro_contrib_rank_calc(db, period, kind, field, want)
		source_text = tool.Baro_t("저장된 값이 없어 지금 즉석에서 계산한 결과입니다.")
	}

	unit := "건"
	if kind == "new_contributor" {
		unit = ""
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"", tool.Baro_t("분야별 순위")}})
	body += tool.Baro_hero(
		tool.Baro_t("분야별 순위"),
		tool.Baro_t("기간과 종류와 분야로 나누어 봅니다. 하나의 통합 순위는 두지 않습니다."),
		"",
	)
	body += baro_contrib_tabs("/baro/rank")

	body += tool.Baro_alert("info",
		tool.Baro_t("<b>바로위키에는 전체를 한 줄로 세우는 통합 순위가 없습니다.</b> ")+
			tool.Baro_t("역사 문서를 쓰는 사람과 게임 문서를 쓰는 사람을 하나의 점수로 비교하는 것은 어느 쪽에도 공정하지 않기 때문입니다. ")+
			tool.Baro_t("그래서 기간·종류·분야로 잘게 나눈 순위만 보여 주고, 그 안에서도 등수를 강조하지 않습니다. ")+
			tool.Baro_t("순위에 나오고 싶지 않다면 설정에서 순위 공개를 끄면 이 화면에서 제외됩니다."),
	)

	// --- 기간 탭 ---
	period_tabs := [][]string{}
	for _, row := range baro_contrib_rank_period_list() {
		period_tabs = append(period_tabs, []string{
			baro_contrib_query("/baro/rank", [][]string{{"period", row[0]}, {"kind", kind}, {"field", field}}),
			row[1],
		})
	}
	body += tool.Baro_section("기간", tool.Baro_tabs(period_tabs,
		baro_contrib_query("/baro/rank", [][]string{{"period", period}, {"kind", kind}, {"field", field}})))

	// --- 종류 탭 ---
	kind_tabs := [][]string{}
	for _, row := range baro_contrib_rank_kind_list() {
		kind_tabs = append(kind_tabs, []string{
			baro_contrib_query("/baro/rank", [][]string{{"period", period}, {"kind", row[0]}, {"field", field}}),
			row[1],
		})
	}
	body += tool.Baro_section("종류", tool.Baro_tabs(kind_tabs,
		baro_contrib_query("/baro/rank", [][]string{{"period", period}, {"kind", kind}, {"field", field}})))

	// --- 분야 탭 ---
	field_tabs := [][]string{{
		baro_contrib_query("/baro/rank", [][]string{{"period", period}, {"kind", kind}}),
		tool.Baro_t("전체 분야"),
	}}
	for _, row := range tool.Baro_field_list(db) {
		field_tabs = append(field_tabs, []string{
			baro_contrib_query("/baro/rank", [][]string{{"period", period}, {"kind", kind}, {"field", row.Key}}),
			row.Name,
		})
	}
	body += tool.Baro_section("분야", tool.Baro_tabs(field_tabs,
		baro_contrib_query("/baro/rank", [][]string{{"period", period}, {"kind", kind}, {"field", field}})))

	title_text := baro_contrib_rank_period_name(period) + " · " + baro_contrib_rank_kind_name(kind)
	if field != "" {
		title_text += " · " + tool.Baro_field_name(db, field)
	} else {
		title_text += tool.Baro_t(" · 전체 분야")
	}

	body += tool.Baro_card_ex(
		title_text,
		baro_contrib_rank_kind_desc(kind),
		baro_contrib_rank_table(db, list, config.IP, unit),
		tool.Baro_note(source_text)+tool.Baro_note(tool.Baro_t("순위 공개를 끈 사용자는 이 목록에서 빠져 있습니다.")),
	)

	// --- 분야별 요약 ---
	summary_rows := [][]string{}
	for _, row := range tool.Baro_field_list(db) {
		top, _ := baro_contrib_rank_load(db, period, kind, row.Key, 1)
		if len(top) == 0 {
			top = baro_contrib_rank_calc(db, period, kind, row.Key, 1)
		}

		top_html := tool.Baro_t(`<span class="bw_muted">아직 없음</span>`)
		if len(top) > 0 {
			top_html = tool.Baro_user_link(db, top[0].User, config.IP) + ` <span class="bw_muted">` + tool.Baro_comma(top[0].Score) + tool.Baro_t(unit) + `</span>`
		}

		summary_rows = append(summary_rows, []string{
			`<a href="` + baro_contrib_esc(baro_contrib_query("/baro/rank", [][]string{{"period", period}, {"kind", kind}, {"field", row.Key}})) + `">` + baro_contrib_esc(row.Name) + `</a>`,
			baro_contrib_esc(row.Description),
			top_html,
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("분야별 한눈에 보기"),
		baro_contrib_rank_period_name(period)+" · "+baro_contrib_rank_kind_name(kind)+tool.Baro_t(" 기준"),
		tool.Baro_table([]string{"분야", tool.Baro_t("설명"), tool.Baro_t("가장 많이 기여한 사람")}, summary_rows, tool.Baro_t("분야가 없습니다.")),
		tool.Baro_button("/baro/rank/run", tool.Baro_t("순위 다시 계산"), "")+" "+tool.Baro_button("/baro/contrib", tool.Baro_t("기여 기록 보기"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("분야별 순위"), body, baro_contrib_menu())
}

// View_baro_contrib_rank_run : 순위 계산 후 baro_rank 에 저장 (권한 system)
func View_baro_contrib_rank_run(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/rank/run"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("순위 계산은 시스템 관리자만 실행할 수 있습니다."))
	}

	// --- 실행 ---
	if values != nil {
		if tool.Baro_read_only(db) {
			body := tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 실행할 수 없습니다. ")+baro_contrib_esc(tool.Baro_read_only_reason(db)))

			return tool.Baro_page(db, config, tool.Baro_t("순위 계산"), body, baro_contrib_menu())
		}

		fields := []string{""}
		for _, row := range tool.Baro_field_list(db) {
			fields = append(fields, row.Key)
		}

		combo, saved := 0, 0

		for _, period_row := range baro_contrib_rank_period_list() {
			for _, kind_row := range baro_contrib_rank_kind_list() {
				for _, field := range fields {
					combo++

					list := baro_contrib_rank_calc(db, period_row[0], kind_row[0], field, 20)
					baro_contrib_rank_store(db, period_row[0], kind_row[0], field, list)
					saved += len(list)
				}
			}
		}

		tool.Baro_public_log(db, "rank_run", config.IP, "baro_rank",
			"분야별 순위 계산", tool.Baro_itoa(combo)+"개 조합 / "+tool.Baro_itoa(saved)+"줄")
		tool.Baro_audit(db, config, "rank_run", "baro_rank", "", tool.Baro_itoa(saved)+"줄 저장", "분야별 순위 재계산")
		tool.Baro_metric_bump(db, "rank_run", tool.Baro_today(), 1)

		return tool.Get_redirect(baro_contrib_query("/baro/rank/run", [][]string{
			{"combo", tool.Baro_itoa(combo)},
			{"saved", tool.Baro_itoa(saved)},
		}))
	}

	stored := baro_contrib_count(db, "select count(*) from baro_rank")
	last := ""
	tool.QueryRow_DB(db, "select date from baro_rank order by id + 0 desc limit 1", []any{&last})

	field_count := len(tool.Baro_field_list(db)) + 1
	combo_count := len(baro_contrib_rank_period_list()) * len(baro_contrib_rank_kind_list()) * field_count

	body := tool.Baro_crumb([][]string{{"/baro/rank", tool.Baro_t("분야별 순위")}, {"", tool.Baro_t("순위 계산")}})
	body += tool.Baro_hero(
		tool.Baro_t("분야별 순위 계산"),
		tool.Baro_t("기간 · 종류 · 분야의 모든 조합을 계산해 저장합니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("계산할 조합"), tool.Baro_comma(combo_count), "개"},
			{tool.Baro_t("저장된 순위 줄"), tool.Baro_comma(stored), tool.Baro_t("줄")},
			{tool.Baro_t("조합당 저장"), "20", tool.Baro_t("명까지")},
			{tool.Baro_t("마지막 계산"), last, ""},
		}),
	)
	body += baro_contrib_tabs("/baro/rank")

	if combo := baro_contrib_get(query, "combo"); combo != "" {
		body += tool.Baro_alert("ok",
			tool.Baro_t("직전 실행 결과 — 조합 <b>")+baro_contrib_esc(combo)+tool.Baro_t("개</b>, ")+
				tool.Baro_t("저장 <b>")+baro_contrib_esc(baro_contrib_get(query, "saved"))+tool.Baro_t("줄</b>."))
	}

	body += tool.Baro_card(tool.Baro_t("실행하면 일어나는 일"), tool.Baro_list([]string{
		"기간 " + tool.Baro_itoa(len(baro_contrib_rank_period_list())) + tool.Baro_t("종 × 종류 ") + tool.Baro_itoa(len(baro_contrib_rank_kind_list())) + tool.Baro_t("종 × 분야 ") + tool.Baro_itoa(field_count) + tool.Baro_t("종을 모두 계산합니다."),
		tool.Baro_t("조합마다 상위 20명을 <code>baro_rank</code> 에 저장하고 이전 값은 지웁니다."),
		tool.Baro_t("<code>baro_user_set</code> 의 <code>rank_public</code> 이 <code>off</code> 인 사용자는 저장하지 않습니다."),
		tool.Baro_t("통합 순위는 만들지 않습니다. 분야를 가로지르는 하나의 등수는 저장하지 않습니다."),
	}))

	body += tool.Baro_card_ex(
		tool.Baro_t("순위 계산 실행"),
		tool.Baro_t("저장된 값이 없으면 화면에서 즉석 계산이 쓰이므로, 계산은 언제 눌러도 안전합니다."),
		tool.Baro_form("/baro/rank/run", tool.Baro_submit(tool.Baro_t("순위 계산 실행"), "primary")),
		tool.Baro_button("/baro/rank", tool.Baro_t("순위 화면으로"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("순위 계산"), body, baro_contrib_menu())
}
