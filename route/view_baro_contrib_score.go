package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  7-2 기여 가치 계산 — /baro/contrib/score
//
//  요소별로 나누어 저장한다.
//    base     행동 종류 가중치
//    source   출처의 수와 품질
//    novelty  새로 만든 문서인지
//    review   다른 편집자의 검토·감사가 있었는지
//    survive  일정 기간 살아남았는지
//    penalty  되돌림·중복·도배
//    final    합계
//
//  정확한 계수는 공개하지 않는다. 대신 평가 요소와 그 의미는 전부 공개한다.
//  (점수를 노리고 행동을 꾸미는 일을 막으면서도 기준은 알 수 있게 한다)
// ============================================================

// baro_contrib_score_element_list : 공개하는 평가 요소 설명 (배점표는 공개하지 않는다)
func baro_contrib_score_element_list() [][]string {
	return [][]string{
		{"base", tool.Baro_t("행동 가치"), tool.Baro_t("무엇을 했는지에 따라 달라집니다. 새 문서 작성과 오타 수정은 같지 않습니다.")},
		{"source", "출처", tool.Baro_t("그 문서에 붙은 출처의 수와 품질을 봅니다. 검증할 수 있는 글을 더 인정합니다.")},
		{"novelty", tool.Baro_t("새로움"), tool.Baro_t("비어 있던 자리를 처음 채운 기여인지를 봅니다.")},
		{"review", tool.Baro_t("검토·감사"), tool.Baro_t("다른 편집자가 검토했거나 감사를 보냈는지를 봅니다. 스스로 보낸 감사는 세지 않습니다.")},
		{"survive", tool.Baro_t("지속"), tool.Baro_t("일정 기간이 지난 뒤에도 그 기여가 남아 있는지를 봅니다.")},
		{"penalty", tool.Baro_t("감점"), tool.Baro_t("되돌림, 중복 문서, 짧은 시간의 도배성 편집을 덜어냅니다.")},
		{"final", "합계", tool.Baro_t("위 요소를 더하고 감점을 뺀 값입니다. 0 아래로는 내려가지 않습니다.")},
	}
}

// baro_contrib_score_guard_list : 명세가 요구하는 안전장치 설명
func baro_contrib_score_guard_list() []string {
	return []string{
		tool.Baro_t("<b>쪼개기 방지</b> — 같은 사람이 같은 문서를 ") + tool.Baro_itoa(baro_contrib_merge_minutes) + tool.Baro_t("분 안에 여러 번 편집하면 하나의 기여로 묶습니다."),
		tool.Baro_t("<b>셀프 추천 제한</b> — 자기 자신에게 보낸 감사는 점수에 넣지 않습니다."),
		tool.Baro_t("<b>반복 오타 수정 감소</b> — 같은 날 같은 사람의 오타 수정은 3건째부터 가치가 절반으로 줄어듭니다."),
		tool.Baro_t("<b>확정 유예</b> — 기여가 이루어진 뒤 정해진 기간이 지나야 점수가 확정됩니다. 그 전에는 언제든 바뀔 수 있습니다."),
		tool.Baro_t("<b>사라진 기여 제외</b> — 문서가 지금 없거나 기여가 되돌려졌다면 점수에 넣지 않습니다."),
		tool.Baro_t("<b>계수 비공개 · 요소 공개</b> — 어떤 요소를 보는지는 전부 공개하지만, 각 요소의 정확한 배점표는 공개하지 않습니다."),
	}
}

// baro_contrib_score_row : 저장된 기여 가치 한 줄
type baro_contrib_score_row struct {
	ID          string
	User        string
	Event       string
	Base        string
	Source      string
	Novelty     string
	Review      string
	Survive     string
	Penalty     string
	Final       string
	Confirmed   string
	ConfirmDate string
	Date        string
}

// baro_contrib_score_cols : 가치 조회 컬럼
const baro_contrib_score_cols = "id, user, event, base, source, novelty, review, survive, penalty, final, confirmed, confirm_date, date"

// baro_contrib_score_scan : 조회 결과를 구조체 목록으로
func baro_contrib_score_scan(rows *sql.Rows) []baro_contrib_score_row {
	out := []baro_contrib_score_row{}

	for rows.Next() {
		row := baro_contrib_score_row{}
		if err := rows.Scan(
			&row.ID, &row.User, &row.Event, &row.Base, &row.Source, &row.Novelty,
			&row.Review, &row.Survive, &row.Penalty, &row.Final,
			&row.Confirmed, &row.ConfirmDate, &row.Date,
		); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_contrib_score_confirm_days : 점수 확정까지 기다리는 날 수 (7~14일)
func baro_contrib_score_confirm_days(db *sql.DB) int {
	days := tool.Baro_get_int(db, "score_confirm_days", 7)
	if days < 7 {
		days = 7
	}
	if days > 14 {
		days = 14
	}

	return days
}

// baro_contrib_score_parts : 요소별 계산 결과
type baro_contrib_score_parts struct {
	Base    int
	Source  int
	Novelty int
	Review  int
	Survive int
	Penalty int
	Final   int
	Merged  bool
	Dropped bool
	Note    string
}

// baro_contrib_score_merged : 같은 사용자·같은 문서의 앞선 편집이 묶음 시간 안에 있는가
func baro_contrib_score_merged(db *sql.DB, ev baro_contrib_event_row) bool {
	if ev.Title == "" || ev.User == "" || ev.Date == "" {
		return false
	}

	from := tool.Baro_time_add(ev.Date, 0, 0, -baro_contrib_merge_minutes)

	found := ""
	return tool.QueryRow_DB(
		db,
		"select id from baro_contrib_event where user = ? and title = ? and id + 0 < ? and date >= ? and date <= ? order by date desc limit 1",
		[]any{&found},
		ev.User, ev.Title, tool.Str_to_int(ev.ID), from, ev.Date,
	)
}

// baro_contrib_score_typo_index : 같은 날 같은 사용자의 몇 번째 오타 수정인가 (0부터)
func baro_contrib_score_typo_index(db *sql.DB, ev baro_contrib_event_row) int {
	return baro_contrib_count(
		db,
		"select count(*) from baro_contrib_event where user = ? and kind = 'typo' and date like ? and id + 0 < ?",
		ev.User, tool.Baro_date_only(ev.Date)+"%", tool.Str_to_int(ev.ID),
	)
}

// baro_contrib_score_day_index : 같은 날 같은 사용자의 몇 번째 기여인가 (도배 판단용)
func baro_contrib_score_day_index(db *sql.DB, ev baro_contrib_event_row) int {
	return baro_contrib_count(
		db,
		"select count(*) from baro_contrib_event where user = ? and date like ? and id + 0 < ?",
		ev.User, tool.Baro_date_only(ev.Date)+"%", tool.Str_to_int(ev.ID),
	)
}

// baro_contrib_score_calc : 이벤트 하나의 기여 가치를 요소별로 계산한다
//
//	여기서 쓰는 계수는 화면에 표시하지 않는다. (7-2 : 계수 비공개 · 요소 공개)
func baro_contrib_score_calc(db *sql.DB, ev baro_contrib_event_row) baro_contrib_score_parts {
	parts := baro_contrib_score_parts{}

	// --- 사라진 기여는 제외한다 ---
	if ev.Status == "rejected" {
		parts.Dropped = true
		parts.Note = tool.Baro_t("되돌려진 기여라 점수에 넣지 않습니다.")

		return parts
	}

	if !baro_contrib_doc_exists(db, ev.Title) {
		parts.Dropped = true
		parts.Note = tool.Baro_t("문서가 지금 존재하지 않아 점수에 넣지 않습니다.")

		return parts
	}

	// --- 쪼갠 편집 묶기 ---
	if baro_contrib_score_merged(db, ev) {
		parts.Merged = true
		parts.Note = tool.Baro_t("같은 문서를 짧은 시간에 이어 편집해 앞선 기여에 묶었습니다.")

		return parts
	}

	// --- base : 행동 종류 가중치 ---
	parts.Base = tool.Baro_contrib_kind_weight(ev.Kind)

	if ev.Kind == "typo" && baro_contrib_score_typo_index(db, ev) >= baro_contrib_typo_free {
		parts.Base = parts.Base / 2
		parts.Note = tool.Baro_t("같은 날 반복된 오타 수정이라 가치를 줄였습니다.")
	}

	// --- source : 출처의 수와 품질 ---
	if ev.Title != "" {
		cite := baro_contrib_count(
			db,
			"select count(*) from baro_citation where title = ? and status <> 'rejected'",
			ev.Title,
		)
		good := baro_contrib_count(
			db,
			"select count(*) from baro_citation where title = ? and status <> 'rejected' and quality = 'high'",
			ev.Title,
		)

		switch ev.Kind {
		case "source":
			parts.Source = cite*6 + good*6
		case "new_document", "expand", "request_done", "project":
			parts.Source = cite*3 + good*3
		default:
			parts.Source = cite
		}

		if parts.Source > 60 {
			parts.Source = 60
		}
	}

	// --- novelty : 새 문서인지 ---
	if ev.Kind == "new_document" {
		parts.Novelty = 25
	} else if ev.Kind == "request_done" {
		parts.Novelty = 15
	}

	// --- review : 다른 편집자의 검토 / 감사 ---
	if ev.Title != "" {
		reviewed := baro_contrib_count(
			db,
			"select count(*) from baro_review where title = ? and status = 'done' and reviewer <> '' and reviewer <> ?",
			ev.Title, ev.User,
		)
		if reviewed > 0 {
			parts.Review += 15
		}

		// 셀프 추천 제한 : from_user = to_user 는 세지 않는다
		thanks := baro_contrib_count(
			db,
			"select count(*) from baro_thanks where title = ? and rev = ? and from_user <> to_user",
			ev.Title, ev.Rev,
		)
		if thanks > 4 {
			thanks = 4
		}
		parts.Review += thanks * 5
	}

	// --- survive : 확정 기간 뒤에도 남아 있는가 ---
	days := baro_contrib_score_confirm_days(db)
	if ev.Date != "" && tool.Baro_days_between(ev.Date, tool.Baro_now()) >= days {
		parts.Survive = 15
	}

	// --- penalty : 되돌림 · 중복 · 도배 ---
	if ev.Title != "" {
		if baro_contrib_count(db, "select count(*) from baro_dup where title = ? and status = 'confirmed'", ev.Title) > 0 {
			parts.Penalty += 10
		}

		if baro_contrib_count(db, "select count(*) from baro_quarantine where title = ? and status = 'active'", ev.Title) > 0 {
			parts.Penalty += 15
		}
	}

	if index := baro_contrib_score_day_index(db, ev); index >= 50 {
		over := index - 49
		if over > 20 {
			over = 20
		}

		parts.Penalty += over
		parts.Note = tool.Baro_t("하루에 지나치게 많은 기여가 몰려 도배 감점이 적용됐습니다.")
	}

	parts.Final = parts.Base + parts.Source + parts.Novelty + parts.Review + parts.Survive - parts.Penalty
	if parts.Final < 0 {
		parts.Final = 0
	}

	return parts
}

// baro_contrib_score_save : 계산 결과를 baro_contrib_score 에 저장한다
func baro_contrib_score_save(db *sql.DB, ev baro_contrib_event_row, parts baro_contrib_score_parts, confirm bool) {
	tool.Exec_DB(db, "delete from baro_contrib_score where event = ?", ev.ID)

	confirmed, confirm_date := "", ""
	if confirm {
		confirmed = "on"
		confirm_date = tool.Baro_now()
	}

	tool.Exec_DB(
		db,
		"insert into baro_contrib_score (id, user, event, base, source, novelty, review, survive, penalty, final, confirmed, confirm_date, date) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_contrib_score"), ev.User, ev.ID,
		tool.Baro_itoa(parts.Base), tool.Baro_itoa(parts.Source), tool.Baro_itoa(parts.Novelty),
		tool.Baro_itoa(parts.Review), tool.Baro_itoa(parts.Survive), tool.Baro_itoa(parts.Penalty),
		tool.Baro_itoa(parts.Final), confirmed, confirm_date, tool.Baro_now(),
	)
}

// baro_contrib_score_stat : 확정 실행 결과 요약
type baro_contrib_score_stat struct {
	Looked    int
	Scored    int
	Confirmed int
	Merged    int
	Dropped   int
	Sum       int
}

// baro_contrib_score_target_count : 확정할 수 있는 이벤트 수
func baro_contrib_score_target_count(db *sql.DB) int {
	days := baro_contrib_score_confirm_days(db)
	limit_date := tool.Baro_time_add(tool.Baro_now(), -days, 0, 0)

	return baro_contrib_count(
		db,
		"select count(*) from baro_contrib_event where date <= ?"+
			" and not exists (select 1 from baro_contrib_score where baro_contrib_score.event = baro_contrib_event.id and baro_contrib_score.confirmed = 'on')",
		limit_date,
	)
}

// baro_contrib_score_pending_count : 아직 확정되지 않은 이벤트 수 (기간 조건 없이)
func baro_contrib_score_pending_count(db *sql.DB) int {
	return baro_contrib_count(
		db,
		"select count(*) from baro_contrib_event"+
			" where not exists (select 1 from baro_contrib_score where baro_contrib_score.event = baro_contrib_event.id and baro_contrib_score.confirmed = 'on')",
	)
}

// baro_contrib_score_run : 확정 처리 실행
func baro_contrib_score_run(db *sql.DB, limit int) baro_contrib_score_stat {
	stat := baro_contrib_score_stat{}
	days := baro_contrib_score_confirm_days(db)

	rows := tool.Query_DB(
		db,
		"select "+baro_contrib_event_cols+" from baro_contrib_event"+
			" where not exists (select 1 from baro_contrib_score where baro_contrib_score.event = baro_contrib_event.id and baro_contrib_score.confirmed = 'on')"+
			" order by id + 0 asc limit ?, ?",
		0, limit,
	)
	defer rows.Close()

	list := baro_contrib_event_scan(rows)

	for _, ev := range list {
		stat.Looked++

		parts := baro_contrib_score_calc(db, ev)

		// 확정 기간이 지났는가
		confirm := ev.Date != "" && tool.Baro_days_between(ev.Date, tool.Baro_now()) >= days

		baro_contrib_score_save(db, ev, parts, confirm)
		stat.Scored++
		stat.Sum += parts.Final

		if parts.Merged {
			stat.Merged++
		}
		if parts.Dropped {
			stat.Dropped++
		}

		if !confirm {
			continue
		}

		stat.Confirmed++

		status := "confirmed"
		switch {
		case parts.Dropped && ev.Status == "rejected":
			status = "rejected"
		case parts.Merged:
			status = "merged"
		case parts.Dropped:
			status = "rejected"
		}

		tool.Exec_DB(db, "update baro_contrib_event set status = ? where id = ?", status, ev.ID)
	}

	return stat
}

// ------------------------------------------------------------
//  화면 : /baro/contrib/score
// ------------------------------------------------------------

// View_baro_contrib_score : 기여 가치 계산 안내와 최근 계산 결과
func View_baro_contrib_score(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page := baro_contrib_page_num(baro_contrib_get(query, "page"))
	user := baro_contrib_get(query, "user")

	where := " where 1 = 1"
	args := []any{}

	if user != "" {
		where += " and user = ?"
		args = append(args, user)
	}

	list_args := append(append([]any{}, args...), (page-1)*baro_contrib_page_size, baro_contrib_page_size)

	rows := tool.Query_DB(
		db,
		"select "+baro_contrib_score_cols+" from baro_contrib_score"+where+" order by id + 0 desc limit ?, ?",
		list_args...,
	)
	defer rows.Close()

	list := baro_contrib_score_scan(rows)

	days := baro_contrib_score_confirm_days(db)
	confirmed_count := baro_contrib_count(db, "select count(*) from baro_contrib_score where confirmed = 'on'")
	waiting_count := baro_contrib_score_pending_count(db)
	ready_count := baro_contrib_score_target_count(db)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/contrib", "기여 기록"}, {"", tool.Baro_t("기여 가치")}})
	body += tool.Baro_hero(
		tool.Baro_t("기여 가치 계산"),
		tool.Baro_t("횟수가 아니라 가치를 봅니다. 무엇을 보는지는 공개하고, 정확한 배점은 공개하지 않습니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("확정된 기여"), tool.Baro_comma(confirmed_count), "건"},
			{tool.Baro_t("확정 대기"), tool.Baro_comma(waiting_count), "건"},
			{tool.Baro_t("지금 확정 가능"), tool.Baro_comma(ready_count), "건"},
			{tool.Baro_t("확정 유예"), tool.Baro_itoa(days) + tool.Baro_t("일"), tool.Baro_t("이 기간이 지나야 확정")},
		}),
	)
	body += baro_contrib_tabs("/baro/contrib/score")

	body += tool.Baro_alert("info",
		tool.Baro_t("<b>평가 요소는 공개하고 정확한 배점표는 공개하지 않습니다.</b> ")+
			tool.Baro_t("배점을 그대로 공개하면 점수를 노리고 행동을 꾸미기 쉬워지기 때문입니다. ")+
			tool.Baro_t("대신 아래에 어떤 요소를 어떤 이유로 보는지를 빠짐없이 적어 둡니다. ")+
			tool.Baro_t("계산이 잘못됐다고 생각되면 운영 로그와 함께 문의할 수 있습니다."),
	)

	// --- 평가 요소 설명 ---
	element_rows := [][]string{}
	for _, row := range baro_contrib_score_element_list() {
		element_rows = append(element_rows, []string{
			`<b>` + baro_contrib_esc(row[1]) + `</b>`,
			`<code>` + baro_contrib_esc(row[0]) + `</code>`,
			baro_contrib_esc(row[2]),
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("평가 요소"),
		tool.Baro_t("각 기여는 아래 요소로 나뉘어 저장됩니다."),
		tool.Baro_table([]string{tool.Baro_t("요소"), tool.Baro_t("저장 이름"), tool.Baro_t("무엇을 보는가")}, element_rows, tool.Baro_t("요소가 없습니다.")),
		tool.Baro_note(tool.Baro_t("배점표(각 요소가 몇 점인지)는 공개하지 않습니다. 이것이 바로위키의 공식 방침입니다.")),
	)

	// --- 안전장치 ---
	body += tool.Baro_card(tool.Baro_t("안전장치"), tool.Baro_list(baro_contrib_score_guard_list()))

	// --- 내 점수 ---
	if tool.Baro_is_user(config.IP) {
		my_sum, my_count := 0, 0
		tool.QueryRow_DB(db, "select coalesce(sum(final + 0), 0), count(*) from baro_contrib_score where user = ? and confirmed = 'on'", []any{&my_sum, &my_count}, config.IP)

		my_wait := baro_contrib_count(db, "select count(*) from baro_contrib_score where user = ? and confirmed <> 'on'", config.IP)

		body += tool.Baro_card_ex(
			tool.Baro_t("내 기여 가치"),
			tool.Baro_t("확정된 기여만 합산합니다."),
			tool.Baro_stat_row([][]string{
				{tool.Baro_t("확정 합계"), tool.Baro_comma(my_sum), tool.Baro_t("점")},
				{tool.Baro_t("확정된 기여"), tool.Baro_comma(my_count), "건"},
				{tool.Baro_t("확정 대기"), tool.Baro_comma(my_wait), "건"},
			}),
			tool.Baro_button(baro_contrib_query("/baro/contrib/score", [][]string{{"user", config.IP}}), tool.Baro_t("내 계산 내역만 보기"), "")+" "+
				tool.Baro_button("/baro/contrib/user/"+tool.Url_parser(config.IP), "내 기여 기록", ""),
		)
	}

	// --- 최근 계산 결과 ---
	table_rows := [][]string{}
	for _, row := range list {
		ev, _ := baro_contrib_event_one(db, row.Event)

		confirmed_html := tool.Baro_badge(tool.Baro_t("확정 대기"), "info")
		if row.Confirmed == "on" {
			confirmed_html = tool.Baro_badge(tool.Baro_t("확정"), "ok")
		}

		element_html := strings.Join([]string{
			tool.Baro_t("행동 ") + baro_contrib_esc(row.Base),
			"출처 " + baro_contrib_esc(row.Source),
			tool.Baro_t("새로움 ") + baro_contrib_esc(row.Novelty),
			"검토 " + baro_contrib_esc(row.Review),
			tool.Baro_t("지속 ") + baro_contrib_esc(row.Survive),
			tool.Baro_t("감점 ") + baro_contrib_esc(row.Penalty),
		}, ` <span class="bw_muted">·</span> `)

		table_rows = append(table_rows, []string{
			tool.Baro_time_cell(row.Date),
			tool.Baro_user_link(db, row.User, config.IP),
			tool.Baro_badge(tool.Baro_contrib_kind_name(ev.Kind), "info"),
			tool.Baro_doc_link(ev.Title),
			`<span class="bw_muted">` + element_html + `</span>`,
			`<b>` + baro_contrib_esc(row.Final) + `</b>`,
			confirmed_html,
		})
	}

	list_html := ""
	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("아직 계산된 기여 가치가 없습니다. 확정 처리를 한 번도 실행하지 않았을 수 있습니다."))
	} else {
		list_html = tool.Baro_table(
			[]string{tool.Baro_t("계산 시각"), tool.Baro_t("사용자"), tool.Baro_t("행동"), "문서", tool.Baro_t("요소"), "합계", "상태"},
			table_rows,
			tool.Baro_t("계산된 기여 가치가 없습니다."),
		)
	}

	paging_url := baro_contrib_query("/baro/contrib/score", [][]string{{"user", user}, {"page", "{}"}})
	list_html += tool.Baro_paging(page, len(list), baro_contrib_page_size, paging_url)

	filter := `<form class="bw_form" method="get" action="/baro/contrib/score">` +
		tool.Baro_input("user", tool.Baro_t("사용자"), user, tool.Baro_t("사용자 이름"), "") +
		tool.Baro_submit("걸러 보기", "") + " " + tool.Baro_button("/baro/contrib/score", tool.Baro_t("필터 지우기"), "") +
		`</form>`

	body += tool.Baro_card_ex(
		tool.Baro_t("계산 내역"),
		tool.Baro_t("요소별 값은 그대로 보여 주되, 각 요소의 배점 기준은 공개하지 않습니다."),
		filter+list_html,
		tool.Baro_button("/baro/contrib/score/run", tool.Baro_t("확정 처리 실행"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("기여 가치 계산"), body, baro_contrib_menu())
}

// ------------------------------------------------------------
//  화면 : /baro/contrib/score/run (권한 system)
// ------------------------------------------------------------

// View_baro_contrib_score_run : 기여 가치 확정 처리 실행
func View_baro_contrib_score_run(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/contrib/score/run"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("기여 가치 확정 처리는 시스템 관리자만 실행할 수 있습니다."))
	}

	days := baro_contrib_score_confirm_days(db)
	ready := baro_contrib_score_target_count(db)
	pending := baro_contrib_score_pending_count(db)

	// --- 실행 ---
	if values != nil {
		if tool.Baro_read_only(db) {
			body := tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 실행할 수 없습니다. ")+baro_contrib_esc(tool.Baro_read_only_reason(db)))

			return tool.Baro_page(db, config, tool.Baro_t("기여 가치 확정"), body, baro_contrib_menu())
		}

		limit := tool.Str_to_int(strings.TrimSpace(values.Get("limit")))
		if limit < 1 || limit > baro_contrib_scan_limit {
			limit = baro_contrib_scan_limit
		}

		stat := baro_contrib_score_run(db, limit)

		tool.Baro_public_log(db, "score_confirm", config.IP, "baro_contrib_score",
			"기여 가치 확정 처리", tool.Baro_itoa(stat.Confirmed)+"건 확정 / "+tool.Baro_itoa(stat.Scored)+"건 계산")
		tool.Baro_audit(db, config, "score_confirm", "baro_contrib_score",
			tool.Baro_itoa(pending)+"건 대기",
			tool.Baro_itoa(stat.Confirmed)+"건 확정",
			"확정 유예 "+tool.Baro_itoa(days)+"일 기준")
		tool.Baro_metric_bump(db, "score_confirm", tool.Baro_today(), stat.Confirmed)

		return tool.Get_redirect(baro_contrib_query("/baro/contrib/score/run", [][]string{
			{"scored", tool.Baro_itoa(stat.Scored)},
			{"confirmed", tool.Baro_itoa(stat.Confirmed)},
			{"merged", tool.Baro_itoa(stat.Merged)},
			{"dropped", tool.Baro_itoa(stat.Dropped)},
		}))
	}

	body := tool.Baro_crumb([][]string{
		{"/baro/contrib", "기여 기록"},
		{"/baro/contrib/score", tool.Baro_t("기여 가치")},
		{"", tool.Baro_t("확정 처리")},
	})

	body += tool.Baro_hero(
		tool.Baro_t("기여 가치 확정 처리"),
		tool.Baro_t("확정 유예가 지난 기여의 점수를 계산해 확정합니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("확정 대상"), tool.Baro_comma(ready), "건"},
			{tool.Baro_t("전체 미확정"), tool.Baro_comma(pending), "건"},
			{tool.Baro_t("확정 유예"), tool.Baro_itoa(days) + tool.Baro_t("일"), "baro_set.score_confirm_days"},
			{tool.Baro_t("한 번에 처리"), tool.Baro_comma(baro_contrib_scan_limit), "건"},
		}),
	)
	body += baro_contrib_tabs("/baro/contrib/score")

	if scored := baro_contrib_get(query, "scored"); scored != "" {
		body += tool.Baro_alert("ok",
			tool.Baro_t("직전 실행 결과 — 계산 <b>")+baro_contrib_esc(scored)+tool.Baro_t("건</b>, ")+
				tool.Baro_t("확정 <b>")+baro_contrib_esc(baro_contrib_get(query, "confirmed"))+tool.Baro_t("건</b>, ")+
				tool.Baro_t("묶임 <b>")+baro_contrib_esc(baro_contrib_get(query, "merged"))+tool.Baro_t("건</b>, ")+
				tool.Baro_t("제외 <b>")+baro_contrib_esc(baro_contrib_get(query, "dropped"))+tool.Baro_t("건</b>."),
		)
	}

	if ready == 0 {
		body += tool.Baro_alert("info", tool.Baro_t("지금 확정할 수 있는 기여가 없습니다. 확정 유예가 지나야 대상이 됩니다."))
	}

	body += tool.Baro_card(tool.Baro_t("실행하면 일어나는 일"), tool.Baro_list([]string{
		tool.Baro_t("각 기여의 요소별 값을 다시 계산해 <code>baro_contrib_score</code> 에 저장합니다."),
		tool.Baro_t("확정 유예 ") + tool.Baro_itoa(days) + tool.Baro_t("일이 지난 기여만 <code>confirmed = on</code> 으로 확정합니다."),
		tool.Baro_t("쪼갠 편집은 앞선 기여에 묶여 <code>merged</code> 로 표시됩니다."),
		tool.Baro_t("문서가 사라졌거나 되돌려진 기여는 <code>rejected</code> 로 표시되고 점수에 넣지 않습니다."),
		tool.Baro_t("공개 운영 로그와 내부 감사 기록을 함께 남깁니다."),
	}))

	body += tool.Baro_card_ex(
		tool.Baro_t("확정 실행"),
		tool.Baro_t("대상 ")+tool.Baro_comma(ready)+tool.Baro_t("건"),
		tool.Baro_form("/baro/contrib/score/run",
			tool.Baro_input_type("limit", tool.Baro_t("한 번에 처리할 건수"), tool.Baro_itoa(baro_contrib_scan_limit), "number",
				tool.Baro_t("1 이상 ")+tool.Baro_itoa(baro_contrib_scan_limit)+tool.Baro_t(" 이하로 적어 주세요."))+
				tool.Baro_submit(tool.Baro_t("확정 처리 실행"), "primary"),
		),
		tool.Baro_button("/baro/contrib/score", tool.Baro_t("계산 내역으로"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("기여 가치 확정"), body, baro_contrib_menu())
}
