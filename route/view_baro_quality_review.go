package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  9-2 편집 검토 큐 — /baro/review
//
//  모든 편집을 사전 승인제로 만들면 위키의 속도가 죽는다.
//  그래서 바로위키는 "위험 신호가 있는 편집만" 사후 검토 큐에 올린다.
// ============================================================

// baro_quality_edit : 검토 위험도를 계산할 편집 한 건
type baro_quality_edit struct {
	Title string
	Rev   string
	User  string
	Date  string
	Send  string
	Leng  string
	Data  string
}

// baro_quality_review_kind_name : 검토 항목 종류 이름
func baro_quality_review_kind_name(kind string) string {
	switch kind {
	case "edit":
		return "편집"
	case "dup":
		return tool.Baro_t("중복 후보")
	case "filter":
		return tool.Baro_t("필터 적발")
	case "manual":
		return tool.Baro_t("직접 요청")
	case "":
		return "편집"
	}

	return kind
}

// baro_quality_review_status_name : 검토 상태 이름
func baro_quality_review_status_name(status string) string {
	switch status {
	case "pending":
		return tool.Baro_t("검토 대기")
	case "approved":
		return "승인"
	case "fix_request":
		return tool.Baro_t("수정 요청")
	case "reverted":
		return tool.Baro_t("되돌림")
	case "":
		return tool.Baro_t("검토 대기")
	}

	return status
}

// baro_quality_review_status_badge : 검토 상태 배지
func baro_quality_review_status_badge(status string) string {
	switch status {
	case "approved":
		return tool.Baro_badge("승인", "ok")
	case "fix_request":
		return tool.Baro_badge(tool.Baro_t("수정 요청"), "warn")
	case "reverted":
		return tool.Baro_badge(tool.Baro_t("되돌림"), "danger")
	}

	return tool.Baro_badge(tool.Baro_t("검토 대기"), "info")
}

// baro_quality_risk_badge : 위험도 배지
func baro_quality_risk_badge(risk string) string {
	value := tool.Str_to_int(risk)
	switch {
	case value >= 70:
		return tool.Baro_badge(risk, "danger")
	case value >= 40:
		return tool.Baro_badge(risk, "warn")
	case value >= 20:
		return tool.Baro_badge(risk, "info")
	}

	return tool.Baro_badge(risk, "muted")
}

// baro_quality_review_min_risk : 검토 큐에 올릴 최소 위험도
func baro_quality_review_min_risk(db *sql.DB) int {
	value := tool.Baro_get_int(db, "review_risk_min", 25)
	if value < 1 {
		value = 1
	}
	if value > 100 {
		value = 100
	}

	return value
}

// baro_quality_recent_edit_count : 그 편집 직전 5분 동안 같은 사람이 한 편집 수
func baro_quality_recent_edit_count(db *sql.DB, user string, date string) int {
	if user == "" || date == "" {
		return 0
	}

	return baro_quality_count(db,
		"select count(*) from history where ip = ? and date >= ? and date <= ?",
		user, tool.Baro_time_add(date, 0, 0, -5), date)
}

// baro_quality_risk : 편집 한 건의 위험도(0~100)와 위험 신호 목록
//
//	위험 신호 : 신규 가입자 · IP 편집 · 대규모 편집 · 출처 없는 새 문서 ·
//	            편집 필터 적발 · 문서 전체 삭제에 가까운 편집 · 초고속 반복 편집
func baro_quality_risk(db *sql.DB, edit baro_quality_edit) (int, []string) {
	risk := 0
	signals := []string{}

	new_user_days := tool.Baro_get_int(db, "review_risk_new_user_days", 30)
	big_length := tool.Baro_get_int(db, "review_big_edit_length", 3000)

	// 1. 비로그인 IP 편집
	if !tool.Baro_is_user(edit.User) {
		risk += 20
		signals = append(signals, tool.Baro_t("비로그인 IP 편집"))
	} else {
		joined := ""
		tool.QueryRow_DB(db,
			"select data from user_set where id = ? and name = 'date'",
			[]any{&joined}, edit.User)

		if joined != "" {
			days := tool.Baro_days_between(joined, edit.Date)
			if days >= 0 && days <= new_user_days {
				risk += 25
				signals = append(signals,
					"가입 "+tool.Baro_itoa(new_user_days)+tool.Baro_t("일 이내 신규 사용자 (가입 후 ")+tool.Baro_itoa(days)+tool.Baro_t("일째 편집)"))
			}
		}
	}

	// 2. 대규모 편집
	delta := tool.Str_to_int(edit.Leng)
	size := delta
	if size < 0 {
		size = -size
	}

	if big_length > 0 && size >= big_length {
		risk += 20
		signals = append(signals,
			tool.Baro_t("대규모 편집 (길이 변화 ")+tool.Baro_comma(size)+tool.Baro_t("자 / 기준 ")+tool.Baro_comma(big_length)+tool.Baro_t("자)"))
	}

	// 3. 문서 전체 삭제에 가까운 편집
	now_length := tool.Get_len(edit.Data)
	prev_length := now_length - delta

	if prev_length > 200 && now_length*5 < prev_length {
		risk += 35
		signals = append(signals,
			tool.Baro_t("본문 대부분 삭제 (")+tool.Baro_comma(prev_length)+tool.Baro_t("자 → ")+tool.Baro_comma(now_length)+tool.Baro_t("자)"))
	} else if prev_length > 50 && strings.TrimSpace(edit.Data) == "" {
		risk += 35
		signals = append(signals, tool.Baro_t("본문 전체 삭제"))
	}

	// 4. 출처 없는 새 문서
	if tool.Str_to_int(edit.Rev) == 1 && strings.TrimSpace(edit.Data) != "" && !baro_quality_has_source(edit.Data) {
		risk += 20
		signals = append(signals, tool.Baro_t("출처가 하나도 없는 새 문서"))
	}

	// 5. 초고속 반복 편집
	recent := baro_quality_recent_edit_count(db, edit.User, edit.Date)
	if recent >= 10 {
		risk += 15
		signals = append(signals, tool.Baro_t("5분 안에 ")+tool.Baro_itoa(recent)+tool.Baro_t("회 편집 (초고속 반복 편집)"))
	}

	// 6. 편집 필터에 걸린 편집 (지금 규칙으로 다시 대조)
	for _, row := range baro_quality_filter_match(db, edit.Title, edit.User, edit.Data, edit.Send, recent) {
		add := 30
		if row.Action == "log" {
			add = 10
		}

		risk += add
		signals = append(signals, tool.Baro_t("편집 필터 적발 : ")+row.Name+" ("+baro_quality_filter_action_name(row.Action)+")")
	}

	// 7. 이 편집 무렵 이미 남아 있는 필터 기록
	if edit.Date != "" {
		logged := baro_quality_count(db,
			"select count(*) from baro_filter_log where title = ? and user = ? and date >= ? and date <= ?",
			edit.Title, edit.User,
			tool.Baro_time_add(edit.Date, 0, 0, -2), tool.Baro_time_add(edit.Date, 0, 0, 2))

		if logged > 0 {
			risk += 15
			signals = append(signals, tool.Baro_t("이 편집 무렵 필터 기록 ")+tool.Baro_itoa(logged)+tool.Baro_t("건"))
		}
	}

	if risk > 100 {
		risk = 100
	}

	return risk, signals
}

// baro_quality_review_exists : 이미 검토 큐에 올라간 편집인가
func baro_quality_review_exists(db *sql.DB, title string, rev string, kind string) bool {
	found := ""

	return tool.QueryRow_DB(db,
		"select id from baro_review where title = ? and rev = ? and kind = ?",
		[]any{&found}, title, rev, kind)
}

// baro_quality_review_insert : 검토 큐에 한 건 넣는다
func baro_quality_review_insert(db *sql.DB, title string, rev string, user string, kind string, risk int, date string) string {
	if date == "" {
		date = tool.Baro_now()
	}

	id := tool.Baro_next_id(db, "baro_review")

	tool.Exec_DB(db,
		"insert into baro_review (id, title, rev, user, kind, status, reviewer, review_date, comment, risk, date) values (?, ?, ?, ?, ?, 'pending', '', '', '', ?, ?)",
		id, title, rev, user, kind, tool.Baro_itoa(risk), date)

	return id
}

// baro_quality_review_scan_run : history 를 훑어 검토가 필요한 편집을 큐에 넣는다
func baro_quality_review_scan_run(db *sql.DB, limit int) (int, int) {
	if limit < 1 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}

	edits := []baro_quality_edit{}

	rows := tool.Query_DB(db,
		"select title, id, ip, date, send, leng, data from history order by date desc limit ?",
		limit)
	defer rows.Close()

	for rows.Next() {
		edit := baro_quality_edit{}
		if err := rows.Scan(&edit.Title, &edit.Rev, &edit.User, &edit.Date, &edit.Send, &edit.Leng, &edit.Data); err != nil {
			continue
		}

		edits = append(edits, edit)
	}

	min_risk := baro_quality_review_min_risk(db)
	added := 0

	for _, edit := range edits {
		if edit.Title == "" || edit.Rev == "" {
			continue
		}

		if baro_quality_review_exists(db, edit.Title, edit.Rev, "edit") {
			continue
		}

		risk, _ := baro_quality_risk(db, edit)
		if risk < min_risk {
			continue
		}

		baro_quality_review_insert(db, edit.Title, edit.Rev, edit.User, "edit", risk, edit.Date)
		added++
	}

	if added > 0 {
		tool.Baro_metric_bump(db, "review_queued", tool.Baro_today(), added)
	}

	return len(edits), added
}

// View_baro_quality_review_list : 검토 대기 목록 (위험도 높은 순)
func View_baro_quality_review_list(config tool.Config, page string, status string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_num := baro_quality_page_num(page)
	offset := (page_num - 1) * tool.Baro_page_size

	switch status {
	case "pending", "approved", "fix_request", "reverted", "all":
	default:
		status = "pending"
	}

	pending := baro_quality_count(db, "select count(*) from baro_review where status = 'pending'")
	approved := baro_quality_count(db, "select count(*) from baro_review where status = 'approved'")
	fix_count := baro_quality_count(db, "select count(*) from baro_review where status = 'fix_request'")
	reverted := baro_quality_count(db, "select count(*) from baro_review where status = 'reverted'")
	high := baro_quality_count(db, "select count(*) from baro_review where status = 'pending' and risk + 0 >= 70")

	body := baro_quality_crumb(tool.Baro_t("편집 검토 큐"))
	body += tool.Baro_hero(
		tool.Baro_t("편집 검토 큐"),
		tool.Baro_t("모든 편집을 사전 승인하지 않습니다. 위험 신호가 있는 편집만 골라 사후에 확인합니다."),
		tool.Baro_button("/baro/review/scan", tool.Baro_t("검토 대상 찾기"), "primary")+" "+
			tool.Baro_button("/baro/review/mine", tool.Baro_t("내가 검토한 내역"), "")+" "+
			tool.Baro_button("/baro/quality/filter", tool.Baro_t("편집 필터 관리"), ""),
	)
	body += baro_quality_nav("/baro/review")

	body += tool.Baro_alert("info",
		tool.Baro_t("위험 신호는 <b>가입 ")+tool.Baro_itoa(tool.Baro_get_int(db, "review_risk_new_user_days", 30))+"일 이내 사용자 · IP 편집 · "+
			tool.Baro_comma(tool.Baro_get_int(db, "review_big_edit_length", 3000))+"자를 넘는 대규모 편집 · 출처 없는 새 문서 · "+
			tool.Baro_t("편집 필터 적발 · 본문 대부분 삭제 · 초고속 반복 편집</b>입니다. ")+
			tool.Baro_t("신호가 있다고 잘못된 편집이라는 뜻이 아니며, 사람 눈으로 확인하기 위한 줄 세우기일 뿐입니다."))

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("검토 대기"), tool.Baro_comma(pending), tool.Baro_t("위험도 순")},
		{tool.Baro_t("위험도 70 이상"), tool.Baro_comma(high), tool.Baro_t("먼저 볼 것")},
		{"승인", tool.Baro_comma(approved), ""},
		{tool.Baro_t("수정 요청"), tool.Baro_comma(fix_count), ""},
		{tool.Baro_t("되돌림"), tool.Baro_comma(reverted), ""},
	})

	body += tool.Baro_tabs([][]string{
		{"/baro/review", tool.Baro_t("검토 대기 (") + tool.Baro_itoa(pending) + ")"},
		{"/baro/review?status=approved", tool.Baro_t("승인 (") + tool.Baro_itoa(approved) + ")"},
		{"/baro/review?status=fix_request", tool.Baro_t("수정 요청 (") + tool.Baro_itoa(fix_count) + ")"},
		{"/baro/review?status=reverted", tool.Baro_t("되돌림 (") + tool.Baro_itoa(reverted) + ")"},
		{"/baro/review?status=all", tool.Baro_t("전체")},
	}, baro_quality_review_tab_url(status))

	query := "select id, title, rev, user, kind, status, reviewer, risk, date from baro_review"
	args := []any{}
	if status != "all" {
		query += " where status = ?"
		args = append(args, status)
	}
	query += " order by risk + 0 desc, id + 0 desc limit ?, ?"
	args = append(args, offset, tool.Baro_page_size)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		id, title, rev, user, kind, row_status, reviewer, risk, date := "", "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &title, &rev, &user, &kind, &row_status, &reviewer, &risk, &date); err != nil {
			continue
		}

		rev_cell := "r" + tool.HTML_escape(rev)
		if kind == "dup" {
			rev_cell = `<a href="/baro/quality/dup/view/` + tool.Url_parser(rev) + `">중복 #` + tool.HTML_escape(rev) + `</a>`
		}

		table_rows = append(table_rows, []string{
			baro_quality_risk_badge(risk),
			baro_quality_doc_cell(title),
			rev_cell,
			tool.Baro_user_link(db, user, config.IP),
			tool.HTML_escape(baro_quality_review_kind_name(kind)),
			baro_quality_review_status_badge(row_status),
			tool.Baro_user_link(db, reviewer, config.IP),
			tool.Baro_time_cell(date),
			tool.Baro_button("/baro/review/view/"+tool.Url_parser(id), "검토", "primary"),
		})
	}

	list_html := tool.Baro_table(
		[]string{tool.Baro_t("위험도"), "문서", "판", tool.Baro_t("편집자"), "종류", "상태", tool.Baro_t("검토자"), tool.Baro_t("편집 시각"), "관리"},
		table_rows, "")

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("이 조건에 해당하는 검토 항목이 없습니다. [검토 대상 찾기] 로 최근 편집을 훑어 보세요."))
	}

	page_url := "/baro/review?status=" + tool.Url_parser(status) + "&page={}"
	list_html += tool.Baro_paging(page_num, len(table_rows), tool.Baro_page_size, page_url)

	body += tool.Baro_card_ex(tool.Baro_t("검토 목록"), tool.Baro_t("위험도가 높은 순서로 보여 줍니다."), list_html, "")

	return tool.Baro_page(db, config, tool.Baro_t("편집 검토 큐"), body, baro_quality_menu())
}

// baro_quality_review_tab_url : 현재 탭 주소
func baro_quality_review_tab_url(status string) string {
	if status == "pending" {
		return "/baro/review"
	}

	return "/baro/review?status=" + tool.Url_parser(status)
}

// View_baro_quality_review_scan : 검토 대상 찾기 (권한 review)
func View_baro_quality_review_scan(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/review/scan"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "review") {
		return tool.Baro_deny(db, config, tool.Baro_t("검토 대상 찾기는 검토 권한이 있는 사용자만 실행할 수 있습니다."))
	}

	message := ""

	if values != nil {
		if tool.Baro_read_only(db) {
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
		} else if !tool.Baro_rate_ok(db, "quality_review_scan", config.IP, 6, 600) {
			message = tool.Baro_alert("warn", tool.Baro_t("검사를 너무 자주 실행했습니다. 잠시 후 다시 시도하세요."))
		} else {
			limit := tool.Str_to_int(baro_quality_val(values, "limit"))
			if limit < 1 {
				limit = 200
			}

			checked, added := baro_quality_review_scan_run(db, limit)

			tool.Baro_public_log(db, "review_scan", config.IP, "",
				"최근 편집 "+tool.Baro_itoa(checked)+"건 점검", "검토 큐 등록 "+tool.Baro_itoa(added)+"건")
			tool.Baro_audit(db, config, "review_scan", "history",
				"", "checked="+tool.Baro_itoa(checked)+" added="+tool.Baro_itoa(added), "검토 대상 찾기")

			return tool.Get_redirect("/baro/review")
		}
	}

	history_total := baro_quality_count(db, "select count(*) from history")
	queued := baro_quality_count(db, "select count(*) from baro_review where status = 'pending'")

	form := tool.Baro_select("limit", tool.Baro_t("훑어볼 최근 편집 수"), [][]string{
		{"100", tool.Baro_t("최근 100건")},
		{"200", tool.Baro_t("최근 200건")},
		{"300", tool.Baro_t("최근 300건")},
		{"500", tool.Baro_t("최근 500건")},
	}, "200", tool.Baro_t("최근 편집부터 거꾸로 훑습니다. 이미 큐에 있는 편집은 건너뜁니다."))
	form += tool.Baro_submit(tool.Baro_t("검토 대상 찾기"), "primary")

	body := baro_quality_crumb(tool.Baro_t("검토 대상 찾기"))
	body += tool.Baro_hero(tool.Baro_t("검토 대상 찾기"), tool.Baro_t("최근 편집을 훑어 위험 신호가 있는 편집만 검토 큐에 올립니다."), "")
	body += baro_quality_nav("/baro/review")
	body += message

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 편집 기록"), tool.Baro_comma(history_total), ""},
		{tool.Baro_t("검토 대기"), tool.Baro_comma(queued), ""},
		{tool.Baro_t("등록 기준 위험도"), tool.Baro_itoa(baro_quality_review_min_risk(db)) + tool.Baro_t(" 이상"), tool.Baro_t("0~100점")},
	})

	body += tool.Baro_card_ex("실행", tool.Baro_t("검토 권한이 있는 사용자만 실행할 수 있습니다."),
		tool.Baro_form("/baro/review/scan", form),
		tool.Baro_button("/baro/review", tool.Baro_t("검토 큐로 돌아가기"), ""))

	body += tool.Baro_card(tool.Baro_t("점수 계산 방법"), tool.Baro_table(
		[]string{tool.Baro_t("위험 신호"), tool.Baro_t("점수")},
		[][]string{
			{tool.Baro_t("비로그인 IP 편집"), "+20"},
			{"가입 " + tool.Baro_itoa(tool.Baro_get_int(db, "review_risk_new_user_days", 30)) + "일 이내 신규 사용자", "+25"},
			{"길이 변화 " + tool.Baro_comma(tool.Baro_get_int(db, "review_big_edit_length", 3000)) + "자 이상 대규모 편집", "+20"},
			{tool.Baro_t("본문 대부분 삭제 / 전체 삭제"), "+35"},
			{tool.Baro_t("출처 없는 새 문서"), "+20"},
			{tool.Baro_t("5분 안에 10회 이상 편집"), "+15"},
			{tool.Baro_t("편집 필터 적발 (규칙 1건마다)"), tool.Baro_t("+30 (기록만 하는 규칙은 +10)")},
			{tool.Baro_t("편집 무렵 필터 기록 존재"), "+15"},
		}, ""))

	body += tool.Baro_note(tool.Baro_t("점수는 100점을 넘지 않습니다. 설정 값은 baro_set 의 review_risk_new_user_days · review_big_edit_length 로 바꿉니다."))

	return tool.Baro_page(db, config, tool.Baro_t("검토 대상 찾기"), body, baro_quality_menu())
}

// baro_quality_review_prev_rev : 문제가 된 판의 "바로 이전 판" 번호
//
//	되돌리기 도구(/revert/<판>/<문서>)는 넘겨준 판의 내용을 되살린다.
//	그러므로 문제가 된 판 자체를 넘기면 문제 판이 다시 저장된다.
//	되돌릴 대상은 그 앞의 판이므로 역사에서 한 칸 앞 판을 찾아 준다.
//	첫 판이거나 앞 판이 없으면 "" 를 돌려준다.
func baro_quality_review_prev_rev(db *sql.DB, title string, rev string) string {
	rev_num := tool.Str_to_int(rev)
	if title == "" || rev_num <= 1 {
		return ""
	}

	prev := ""
	if !tool.QueryRow_DB(db,
		"select id from history where title = ? and id + 0 < ? order by id + 0 desc limit 1",
		[]any{&prev}, title, rev_num) {
		return ""
	}

	return prev
}

// View_baro_quality_review_view : 검토 화면 (승인 / 수정 요청 / 되돌리기)
func View_baro_quality_review_view(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	id = strings.TrimSpace(id)

	title, rev, user, kind, status := "", "", "", "", ""
	reviewer, review_date, comment, risk, date := "", "", "", "", ""

	found := tool.QueryRow_DB(db,
		"select title, rev, user, kind, status, reviewer, review_date, comment, risk, date from baro_review where id = ?",
		[]any{&title, &rev, &user, &kind, &status, &reviewer, &review_date, &comment, &risk, &date},
		id)

	if !found {
		body := baro_quality_crumb(tool.Baro_t("검토 화면"))
		body += tool.Baro_hero(tool.Baro_t("검토 항목을 찾을 수 없습니다"), tool.Baro_t("번호가 잘못되었거나 이미 삭제된 항목입니다."), "")
		body += baro_quality_nav("/baro/review")
		body += tool.Baro_alert("warn", tool.Baro_t("검토 번호 ")+tool.HTML_escape(tool.Baro_trunc(id, 40))+tool.Baro_t(" 에 해당하는 항목이 없습니다."))
		body += tool.Baro_card(tool.Baro_t("다음 할 일"), tool.Baro_button("/baro/review", tool.Baro_t("검토 큐 목록"), "primary")+" "+
			tool.Baro_button("/baro/review/scan", tool.Baro_t("검토 대상 찾기"), ""))

		return tool.Baro_page(db, config, tool.Baro_t("검토 화면"), body, baro_quality_menu())
	}

	message := ""

	// 처리
	if values != nil {
		switch {
		case !tool.Baro_is_user(config.IP):
			message = tool.Baro_alert("danger", tool.Baro_t("검토는 로그인한 사용자만 할 수 있습니다."))
		case !tool.Baro_can(db, config.IP, "review"):
			message = tool.Baro_alert("danger", tool.Baro_t("검토 권한이 없습니다."))
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
		case config.IP == user:
			message = tool.Baro_alert("danger", tool.Baro_t("자기 편집은 스스로 검토할 수 없습니다. 다른 검토자에게 맡겨 주세요."))
		default:
			act := baro_quality_val(values, "act")
			post_comment := tool.Baro_trunc(baro_quality_val(values, "comment"), 1000)

			new_status := ""
			log_kind := ""
			notify_text := ""

			switch act {
			case "approve":
				new_status, log_kind = "approved", "review_approve"
				notify_text = tool.Baro_t("검토 결과 : 문제 없음으로 승인되었습니다.")
			case "fix":
				new_status, log_kind = "fix_request", "review_fix"
				notify_text = tool.Baro_t("검토 결과 : 보완이 필요합니다. 검토자 의견을 확인해 주세요.")
			case "revert":
				new_status, log_kind = "reverted", "review_revert"
				notify_text = tool.Baro_t("검토 결과 : 편집이 되돌려졌습니다. 이의가 있으면 토론을 열어 주세요.")
			}

			if new_status == "" {
				message = tool.Baro_alert("danger", tool.Baro_t("알 수 없는 처리입니다."))
			} else if act != "approve" && post_comment == "" {
				message = tool.Baro_alert("danger", tool.Baro_t("수정 요청과 되돌리기는 사유를 반드시 남겨야 합니다."))
			} else {
				tool.Exec_DB(db,
					"update baro_review set status = ?, reviewer = ?, review_date = ?, comment = ? where id = ?",
					new_status, config.IP, tool.Baro_now(), post_comment, id)

				// 검토자에게 기여 기록
				tool.Baro_contrib_add(db, config.IP, "review", title, rev, "",
					baro_quality_review_status_name(new_status)+" / "+tool.Baro_trunc(post_comment, 200))
				tool.Baro_impact_bump(db, config.IP, "review_done", 1)

				// 근거 공개 + 내부 상세
				tool.Baro_public_log(db, log_kind, config.IP, title,
					tool.Baro_trunc(post_comment, 300),
					"r"+rev+" "+baro_quality_review_status_name(new_status))
				tool.Baro_audit(db, config, log_kind, title+" r"+rev, status, new_status, post_comment)

				// 편집자에게 알림
				if tool.Baro_is_user(user) && user != config.IP {
					tool.Baro_notify(db, user, "review",
						tool.Namespace_display(title)+tool.Baro_t(" 편집 검토 결과 : ")+baro_quality_review_status_name(new_status),
						"/baro/review/view/"+tool.Url_parser(id),
						notify_text)
				}

				tool.Baro_metric_bump(db, "review_done", tool.Baro_today(), 1)

				return tool.Get_redirect("/baro/review")
			}
		}
	}

	// 편집 본문 읽기
	edit := baro_quality_edit{Title: title, Rev: rev, User: user, Date: date}
	edit_found := false
	if kind != "dup" {
		edit_found = tool.QueryRow_DB(db,
			"select data, send, leng, date, ip from history where title = ? and id = ?",
			[]any{&edit.Data, &edit.Send, &edit.Leng, &edit.Date, &edit.User},
			title, rev)
	}

	signals := []string{}
	live_risk := tool.Str_to_int(risk)
	if edit_found {
		live_risk, signals = baro_quality_risk(db, edit)
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/review", tool.Baro_t("검토 큐")},
		{"", tool.Baro_t("검토 #") + id},
	})

	body += tool.Baro_hero(
		tool.Baro_t("검토 #")+id,
		tool.Namespace_display(title)+" r"+rev,
		tool.Baro_button("/w/" + tool.Doc_url(title), "문서 보기", "")+" "+
			tool.Baro_button("/history/"+tool.Url_parser(title), "역사", "")+" "+
			tool.Baro_button("/baro/quality/doc/"+tool.Url_parser(title), "문서 품질 정보", ""),
	)

	body += baro_quality_nav("/baro/review")
	body += message

	diff_cell := baro_quality_diff_link(db, title, rev)
	if kind == "dup" {
		diff_cell = `<a href="/baro/quality/dup/view/` + tool.Url_parser(rev) + `">중복 후보 #` + tool.HTML_escape(rev) + tool.Baro_t(` 보기</a>`)
	}

	body += tool.Baro_kv_grid([][]string{
		{"종류", tool.HTML_escape(baro_quality_review_kind_name(kind))},
		{"문서", baro_quality_doc_cell(title)},
		{tool.Baro_t("변경 내용"), diff_cell},
		{tool.Baro_t("편집자"), tool.Baro_user_link(db, user, config.IP)},
		{tool.Baro_t("편집 시각"), tool.Baro_time_cell(date)},
		{tool.Baro_t("접수 당시 위험도"), baro_quality_risk_badge(risk)},
		{tool.Baro_t("현재 다시 계산"), baro_quality_risk_badge(tool.Baro_itoa(live_risk))},
		{"상태", baro_quality_review_status_badge(status)},
		{tool.Baro_t("검토자"), tool.Baro_user_link(db, reviewer, config.IP)},
		{tool.Baro_t("검토 시각"), tool.Baro_time_cell(review_date)},
	})

	// 위험 신호
	signal_html := tool.Baro_empty(tool.Baro_t("다시 계산한 위험 신호가 없습니다. 접수 당시의 조건이 사라졌을 수 있습니다."))
	if len(signals) > 0 {
		items := []string{}
		for _, signal := range signals {
			items = append(items, tool.HTML_escape(signal))
		}
		signal_html = tool.Baro_list(items)
	}

	if !edit_found && kind != "dup" {
		signal_html = tool.Baro_alert("warn", tool.Baro_t("해당 판의 편집 기록을 찾을 수 없습니다. 역사에서 판이 지워졌을 수 있습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("위험 신호"), signal_html)

	// 출처 유무
	cite_count := baro_quality_count(db, "select count(*) from baro_citation where title = ?", title)
	source_html := ""
	if edit_found {
		notes := baro_quality_footnotes(edit.Data)
		urls := baro_quality_url_list(edit.Data)

		source_html = tool.Baro_kv_grid([][]string{
			{tool.Baro_t("이 판의 각주"), tool.Baro_itoa(len(notes)) + tool.Baro_t("개")},
			{tool.Baro_t("이 판의 URL"), tool.Baro_itoa(len(urls)) + tool.Baro_t("개")},
			{tool.Baro_t("등록된 출처"), tool.Baro_itoa(cite_count) + tool.Baro_t("개")},
			{tool.Baro_t("출처 유무"), baro_quality_yes_no(baro_quality_has_source(edit.Data), tool.Baro_t("있음"), "없음")},
			{tool.Baro_t("본문 길이"), tool.Baro_comma(tool.Get_len(edit.Data)) + tool.Baro_t("자")},
			{tool.Baro_t("편집 요약"), tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(edit.Send), 80))},
		})

		if len(urls) > 0 {
			links := []string{}
			for index, raw := range urls {
				if index >= 10 {
					break
				}
				links = append(links, baro_quality_ext_link(raw))
			}
			source_html += tool.Baro_section(tool.Baro_t("이 판에 들어 있는 링크"), tool.Baro_list(links))
		}
	} else {
		source_html = tool.Baro_kv_grid([][]string{
			{tool.Baro_t("등록된 출처"), tool.Baro_itoa(cite_count) + tool.Baro_t("개")},
		})
	}

	body += tool.Baro_card_ex(tool.Baro_t("출처 유무"), tool.Baro_t("출처가 없다고 곧바로 지우지 않습니다. 먼저 출처 필요 상태를 달아 주세요."), source_html,
		tool.Baro_button("/baro/quality/set?title="+tool.Url_parser(title), "출처 필요 상태 달기", ""))

	// 본문 미리보기
	if edit_found {
		preview := tool.Baro_trunc(edit.Data, 1500)
		if strings.TrimSpace(preview) == "" {
			preview = tool.Baro_t("(본문이 비어 있습니다)")
		}
		body += tool.Baro_card(tool.Baro_t("이 판 본문 미리보기 (앞 1500자)"), tool.Baro_code(preview))
	}

	// 처리 폼
	if !tool.Baro_is_user(config.IP) {
		body += tool.Baro_alert("info", tool.Baro_t("검토 결과를 남기려면 로그인이 필요합니다."))
	} else if !tool.Baro_can(db, config.IP, "review") {
		body += tool.Baro_alert("info", tool.Baro_t("이 화면은 볼 수 있지만, 검토 결과를 남기려면 검토 권한이 필요합니다."))
	} else if config.IP == user {
		body += tool.Baro_alert("warn", tool.Baro_t("자기 편집은 스스로 검토할 수 없습니다. (이해충돌 방지)"))
	} else {
		form := tool.Baro_textarea("comment", tool.Baro_t("검토자 코멘트"), comment, 4,
			tool.Baro_t("무엇을 확인했고 왜 이렇게 판단했는지 적어 주세요."),
			tool.Baro_t("승인 외의 처리는 사유가 필수입니다. 코멘트는 편집자에게 알림으로 전달됩니다."))
		form += `<div class="bw_f">` +
			tool.Baro_t(`<button type="submit" class="bw_btn bw_btn_primary" name="act" value="approve">승인</button> `) +
			tool.Baro_t(`<button type="submit" class="bw_btn" name="act" value="fix">수정 요청</button> `) +
			tool.Baro_t(`<button type="submit" class="bw_btn bw_btn_danger" name="act" value="revert">되돌리기로 기록</button>`) +
			`</div>`

		// 되돌리기 도구는 "문제가 된 판" 이 아니라 그 바로 앞 판을 가리켜야 한다
		revert_help := tool.Baro_note(tool.Baro_t("[되돌리기로 기록] 은 검토 판단만 기록합니다. 실제 문서를 되돌리려면 아래 되돌리기 도구를 쓰세요."))

		prev_rev := ""
		if kind != "dup" {
			prev_rev = baro_quality_review_prev_rev(db, title, rev)
		}

		switch {
		case kind == "dup":
			revert_help += tool.Baro_note(tool.Baro_t("이 검토는 중복 후보를 다루는 것이라 되돌릴 판이 없습니다. 중복 감지 화면에서 처리해 주세요."))
		case prev_rev != "":
			revert_help += tool.Baro_button(
				"/revert/"+tool.Url_parser(prev_rev)+"/"+tool.Url_parser(title),
				tool.Baro_t("문서 되돌리기 도구 열기 (r")+prev_rev+tool.Baro_t(" 로 되돌림)"), "danger")
		default:
			revert_help += tool.Baro_note(tool.Baro_t("이 판(r") + rev + tool.Baro_t(")은 첫 판이라 되돌릴 이전 판이 없습니다. ") +
				tool.Baro_t("내용에 문제가 있다면 되돌리기 대신 문서 상태를 달거나 격리를 검토해 주세요."))
		}

		body += tool.Baro_card_ex(tool.Baro_t("검토 결과 남기기"),
			tool.Baro_t("승인 / 수정 요청 / 되돌리기 중 하나를 고릅니다."),
			tool.Baro_form("/baro/review/view/"+tool.Url_parser(id), form),
			revert_help)
	}

	if comment != "" {
		body += tool.Baro_card(tool.Baro_t("남겨진 검토 코멘트"), tool.Baro_code(comment))
	}

	return tool.Baro_page(db, config, tool.Baro_t("검토 #")+id, body, baro_quality_menu())
}

// View_baro_quality_review_mine : 내가 검토한 내역
func View_baro_quality_review_mine(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/review/mine"); need {
		return wall
	}

	page_num := baro_quality_page_num(page)
	offset := (page_num - 1) * tool.Baro_page_size

	total := baro_quality_count(db, "select count(*) from baro_review where reviewer = ?", config.IP)
	approved := baro_quality_count(db, "select count(*) from baro_review where reviewer = ? and status = 'approved'", config.IP)
	fix_count := baro_quality_count(db, "select count(*) from baro_review where reviewer = ? and status = 'fix_request'", config.IP)
	reverted := baro_quality_count(db, "select count(*) from baro_review where reviewer = ? and status = 'reverted'", config.IP)
	contrib := baro_quality_count(db, "select count(*) from baro_contrib_event where user = ? and kind = 'review'", config.IP)

	rows := tool.Query_DB(db,
		"select id, title, rev, user, kind, status, comment, review_date from baro_review where reviewer = ? order by review_date desc limit ?, ?",
		config.IP, offset, tool.Baro_page_size)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		id, title, rev, user, kind, status, comment, review_date := "", "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &title, &rev, &user, &kind, &status, &comment, &review_date); err != nil {
			continue
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/review/view/` + tool.Url_parser(id) + `">#` + tool.HTML_escape(id) + `</a>`,
			baro_quality_doc_cell(title),
			"r" + tool.HTML_escape(rev),
			tool.HTML_escape(baro_quality_review_kind_name(kind)),
			tool.Baro_user_link(db, user, config.IP),
			baro_quality_review_status_badge(status),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(comment), 60)),
			tool.Baro_time_cell(review_date),
		})
	}

	list_html := tool.Baro_table(
		[]string{"번호", "문서", "판", "종류", tool.Baro_t("편집자"), tool.Baro_t("판단"), tool.Baro_t("코멘트"), tool.Baro_t("검토 시각")},
		table_rows, "")

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("아직 검토한 항목이 없습니다. 검토 큐에서 한 건씩 확인해 주세요."))
	}

	list_html += tool.Baro_paging(page_num, len(table_rows), tool.Baro_page_size, "/baro/review/mine?page={}")

	body := baro_quality_crumb(tool.Baro_t("내가 검토한 내역"))
	body += tool.Baro_hero(tool.Baro_t("내가 검토한 내역"), tool.Baro_t("검토는 기여로 기록됩니다. 되돌린 횟수가 많다고 좋은 검토는 아닙니다."), "")
	body += baro_quality_nav("/baro/review")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("검토 건수"), tool.Baro_comma(total), ""},
		{"승인", tool.Baro_comma(approved), ""},
		{tool.Baro_t("수정 요청"), tool.Baro_comma(fix_count), ""},
		{tool.Baro_t("되돌림"), tool.Baro_comma(reverted), ""},
		{tool.Baro_t("검토 기여 기록"), tool.Baro_comma(contrib), "baro_contrib_event"},
	})

	body += tool.Baro_card_ex(tool.Baro_t("검토 내역"), tool.Baro_t("최근에 검토한 순서입니다."), list_html,
		tool.Baro_button("/baro/review", tool.Baro_t("검토 큐로"), "primary"))

	return tool.Baro_page(db, config, tool.Baro_t("내가 검토한 내역"), body, baro_quality_menu())
}
