package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  5-3 최근 변경 / 5-4 발견 기능
// ============================================================

// baro_find_change : 최근 변경 한 줄
type baro_find_change struct {
	ID     string
	Title  string
	Date   string
	User   string
	Send   string
	Length string
	Kind   string
	Note   string
}

// baro_find_recent_mode_list : 최근 변경 보기 방식
func baro_find_recent_mode_list() [][]string {
	return [][]string{
		{"", tool.Baro_t("전체 최근 변경")},
		{"new", tool.Baro_t("신규 문서만")},
		{"revert", tool.Baro_t("되돌림·삭제만")},
		{"pending", tool.Baro_t("검토 안 된 편집만")},
		{"upload", tool.Baro_t("파일 올리기만")},
		{"move", tool.Baro_t("이동만")},
	}
}

// baro_find_recent_mode_name : 보기 방식 이름
func baro_find_recent_mode_name(mode string) string {
	for _, row := range baro_find_recent_mode_list() {
		if row[0] == mode {
			return row[1]
		}
	}

	return tool.Baro_t("전체 최근 변경")
}

// baro_find_kind_name : history 종류를 한국어로
func baro_find_kind_name(kind string) string {
	switch kind {
	case "":
		return "편집"
	case "r1":
		return tool.Baro_t("새 문서")
	case "delete":
		return "삭제"
	case "revert":
		return tool.Baro_t("되돌림")
	case "move":
		return "이동"
	case "upload":
		return tool.Baro_t("파일 올리기")
	case "setting":
		return "설정 변경"
	case "user":
		return "사용자 문서"
	case "file":
		return tool.Baro_t("파일 문서")
	case "category":
		return tool.Baro_t("분류 문서")
	}

	return kind
}

// baro_find_kind_badge_kind : 종류별 배지 색
func baro_find_kind_badge_kind(kind string) string {
	switch kind {
	case "r1":
		return "ok"
	case "delete", "revert":
		return "danger"
	case "move", "setting":
		return "warn"
	case "upload":
		return "info"
	}

	return "muted"
}

// baro_find_history_row : history 한 줄을 읽어 온다
func baro_find_history_row(db *sql.DB, id string, title string) baro_find_change {
	out := baro_find_change{ID: id, Title: title}

	date, ip, send, leng, hide, kind := "", "", "", "", "", ""
	tool.QueryRow_DB(
		db,
		"select date, ip, send, leng, hide, type from history where id = ? and title = ?",
		[]any{&date, &ip, &send, &leng, &hide, &kind},
		id, title,
	)

	out.Date = date
	out.User = ip
	out.Send = send
	out.Length = leng
	out.Kind = kind

	// 첫 판인데 종류가 비어 있으면 새 문서로 표시한다.
	if id == "1" && out.Kind == "" {
		out.Kind = "r1"
	}

	if hide != "" {
		out.Send = tool.Baro_t("(숨겨진 편집 요약)")
	}

	return out
}

// baro_find_recent_collect : 조건에 맞는 최근 변경을 모은다
func baro_find_recent_collect(db *sql.DB, mode string, want int) []baro_find_change {
	out := []baro_find_change{}
	if want <= 0 {
		want = 200
	}

	switch mode {
	case "pending":
		// 검토 큐에 pending 으로 남아 있는 편집만
		rows := tool.Query_DB(
			db,
			"select id, title, rev, user, date, comment from baro_review where status = 'pending' order by id + 0 desc limit ?",
			want,
		)
		defer rows.Close()

		for rows.Next() {
			id, title, rev, user, date, comment := "", "", "", "", "", ""
			if err := rows.Scan(&id, &title, &rev, &user, &date, &comment); err != nil {
				continue
			}

			row := baro_find_history_row(db, rev, title)
			row.Title = title
			if row.Date == "" {
				row.Date = date
			}
			if row.User == "" {
				row.User = user
			}
			row.Note = tool.Baro_t("검토 대기 #") + id
			if comment != "" {
				row.Note += " · " + tool.Baro_trunc(comment, 40)
			}

			out = append(out, row)
		}

		return out
	case "new":
		// 문서의 첫 판(id = 1)이 곧 새 문서다.
		// openNAMU 는 이름공간 문서의 첫 판에 'r1' 대신 이름공간 이름을 넣으므로 두 조건을 함께 본다.
		return baro_find_history_collect(db, "select id, title from history where type = 'r1' or id = '1' order by date desc limit ?", want)
	case "revert":
		return baro_find_history_collect(db, "select id, title from history where type in ('revert', 'delete') order by date desc limit ?", want)
	case "upload":
		return baro_find_history_collect(db, "select id, title from history where type = 'upload' order by date desc limit ?", want)
	case "move":
		return baro_find_history_collect(db, "select id, title from history where type = 'move' order by date desc limit ?", want)
	}

	// 전체 : rc 표(최근 변경 캐시)를 먼저 읽고, 모자라면 history 로 채운다.
	seen := map[string]bool{}
	rows := tool.Query_DB(db, "select id, title from rc where type = 'normal' order by date desc limit ?", want)
	defer rows.Close()

	for rows.Next() {
		id, title := "", ""
		if err := rows.Scan(&id, &title); err != nil {
			continue
		}

		seen[id+"\x00"+title] = true
		out = append(out, baro_find_history_row(db, id, title))
	}

	if len(out) < want {
		for _, row := range baro_find_history_collect(db, "select id, title from history order by date desc limit ?", want) {
			if seen[row.ID+"\x00"+row.Title] {
				continue
			}

			out = append(out, row)
		}
	}

	sort.SliceStable(out, func(a int, b int) bool { return out[a].Date > out[b].Date })

	return out
}

// baro_find_history_collect : history 질의 결과를 최근 변경 줄로
func baro_find_history_collect(db *sql.DB, query string, want int) []baro_find_change {
	out := []baro_find_change{}

	rows := tool.Query_DB(db, query, want)
	defer rows.Close()

	pairs := [][]string{}
	for rows.Next() {
		id, title := "", ""
		if err := rows.Scan(&id, &title); err != nil {
			continue
		}

		pairs = append(pairs, []string{id, title})
	}

	for _, pair := range pairs {
		out = append(out, baro_find_history_row(db, pair[0], pair[1]))
	}

	return out
}

// View_baro_find_recent : 5-3 최근 변경
func View_baro_find_recent(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	mode := baro_find_val(values, "mode")
	field := baro_find_val(values, "field")
	user := baro_find_val(values, "user")
	hide_bot := baro_find_val(values, "hide_bot")

	page_int := baro_find_page_int(page)
	if raw_page := baro_find_val(values, "page"); raw_page != "" {
		page_int = baro_find_page_int(raw_page)
	}

	// 필터로 걸러 낼 것을 감안해 넉넉히 읽는다.
	want := (page_int + 2) * tool.Baro_page_size * 3
	if want > 900 {
		want = 900
	}

	all := baro_find_recent_collect(db, mode, want)
	hidden := baro_find_hidden_set(db)
	meta_map := baro_find_meta_map(db)

	bot_cache := map[string]bool{}
	kept := []baro_find_change{}

	for _, row := range all {
		if baro_find_excluded(row.Title, hidden) {
			continue
		}
		if user != "" && row.User != user {
			continue
		}
		if field != "" {
			meta := meta_map[row.Title]
			if meta == nil || meta["field"] != field {
				continue
			}
		}
		if hide_bot == "on" {
			is_bot, ok := bot_cache[row.User]
			if !ok {
				is_bot = baro_find_bot_check(db, row.User)
				bot_cache[row.User] = is_bot
			}
			if is_bot {
				continue
			}
		}

		kept = append(kept, row)
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
	for _, row := range kept[start:end] {
		title_cell := tool.Baro_doc_link(row.Title)
		if row.ID != "" {
			title_cell += ` <a class="bw_note" href="/history/` + tool.Url_parser(row.Title) + `">r` + tool.HTML_escape(row.ID) + `</a>`
		}

		meta := meta_map[row.Title]
		if meta != nil && meta["field"] != "" {
			title_cell += `<div class="bw_note">` + tool.HTML_escape(tool.Baro_field_name(db, meta["field"])) + `</div>`
		}

		note := tool.HTML_escape(tool.Baro_trunc(row.Send, 60))
		if row.Note != "" {
			note += `<div class="bw_note">` + tool.HTML_escape(row.Note) + `</div>`
		}
		if note == "" {
			note = `<span class="bw_muted">-</span>`
		}

		length_cell := tool.HTML_escape(row.Length)
		if strings.HasPrefix(row.Length, "-") {
			length_cell = `<span class="bw_badge bw_badge_danger">` + tool.HTML_escape(row.Length) + `</span>`
		} else if strings.HasPrefix(row.Length, "+") {
			length_cell = `<span class="bw_badge bw_badge_ok">` + tool.HTML_escape(row.Length) + `</span>`
		}

		rows = append(rows, []string{
			tool.Baro_time_cell(row.Date),
			title_cell,
			tool.Baro_badge(baro_find_kind_name(row.Kind), baro_find_kind_badge_kind(row.Kind)),
			tool.Baro_user_link(db, row.User, config.IP),
			length_cell,
			note,
		})
	}

	filter_body := tool.Baro_select("mode", tool.Baro_t("보기 방식"), baro_find_recent_mode_list(), mode, tool.Baro_t("검토 안 된 편집은 baro_review 의 pending 항목을 읽습니다."))
	filter_body += tool.Baro_select("field", "분야", baro_find_field_options(db), field, tool.Baro_t("문서에 등록된 분야로 거릅니다."))
	filter_body += tool.Baro_input("user", tool.Baro_t("사용자"), user, tool.Baro_t("사용자 이름 또는 IP"), tool.Baro_t("특정 기여자의 편집만 봅니다."))
	filter_body += tool.Baro_checkbox("hide_bot", "봇 편집 숨기기", "on", hide_bot == "on", "이름이 bot·봇 으로 끝나거나 is_bot 설정이 켜진 계정을 뺍니다.")
	filter_body += `<div class="bw_f">` + tool.Baro_submit(tool.Baro_t("적용"), "primary") + `</div>`

	page_url := url.Values{}
	page_url.Set("mode", mode)
	page_url.Set("field", field)
	page_url.Set("user", user)
	if hide_bot == "on" {
		page_url.Set("hide_bot", "on")
	}

	body := baro_find_tabs("/baro/discover/recent")
	body += tool.Baro_hero(tool.Baro_t("최근 변경"), baro_find_recent_mode_name(mode)+tool.Baro_t(" · 모두 ")+tool.Baro_comma(len(kept))+tool.Baro_t("건"), "")
	body += tool.Baro_card(tool.Baro_t("보기 방식"), `<form class="bw_form" method="get" action="/baro/discover/recent">`+filter_body+`</form>`)

	list := tool.Baro_table(
		[]string{"시각", "문서", "종류", tool.Baro_t("기여자"), tool.Baro_t("분량"), tool.Baro_t("요약")},
		rows,
		"",
	)
	if len(kept) == 0 {
		list = tool.Baro_empty(tool.Baro_t("조건에 맞는 최근 변경이 없습니다. 보기 방식을 바꾸거나 필터를 지워 보세요."))
	}

	list += tool.Baro_paging(page_int, end-start, tool.Baro_page_size, "/baro/discover/recent?"+page_url.Encode()+"&page={}")

	body += tool.Baro_card(tool.Baro_t("변경 목록"), list)

	return tool.Baro_page(db, config, tool.Baro_t("최근 변경"), body, baro_find_menu())
}

// ------------------------------------------------------------
//  5-4 발견
// ------------------------------------------------------------

// baro_find_pick_titles : 발견 화면에 쓸 수 있는 문서 제목 (격리·연습장·휴지통 제외)
func baro_find_pick_titles(db *sql.DB) []string {
	hidden := baro_find_hidden_set(db)
	out := []string{}

	for _, title := range baro_find_title_list(db) {
		if baro_find_excluded(title, hidden) {
			continue
		}
		if tool.Namespace_of(title).Prefix != "" {
			continue
		}

		out = append(out, title)
	}

	sort.Strings(out)

	return out
}

// baro_find_daily_pick : 날짜 문자열을 해시해 결정론적으로 하나를 고른다
func baro_find_daily_pick(list []string, seed string) string {
	if len(list) == 0 {
		return ""
	}

	sum := 0
	for _, ch := range tool.Sha224(seed) {
		sum = (sum*31 + int(ch)) % 1000000007
	}

	return list[sum%len(list)]
}

// baro_find_doc_card : 문서 한 개를 소개하는 작은 카드
func baro_find_doc_card(db *sql.DB, title string, sub string, extra string) string {
	if title == "" {
		return tool.Baro_empty(tool.Baro_t("아직 소개할 문서가 없습니다."))
	}

	raw := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&raw}, title)

	meta := baro_find_meta_all(db, title)
	body := `<p><b>` + tool.Baro_doc_link(title) + `</b></p>`
	if line := baro_find_person_line(meta); line != "" {
		body += tool.Baro_note(line)
	}
	body += `<p>` + baro_find_snippet(raw, "") + `</p>`
	if extra != "" {
		body += extra
	}

	return tool.Baro_card_ex(sub, "", body, tool.Baro_button("/w/"+tool.Url_parser(title), "문서 보기", "primary"))
}

// baro_find_title_rows : 제목 목록을 표 줄로
func baro_find_title_rows(db *sql.DB, titles []string, meta_map map[string]map[string]string, note map[string]string) [][]string {
	rows := [][]string{}
	for _, title := range titles {
		meta := meta_map[title]
		if meta == nil {
			meta = baro_find_meta_all(db, title)
		}

		field_name := ""
		if meta["field"] != "" {
			field_name = tool.Baro_field_name(db, meta["field"])
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(title),
			tool.HTML_escape(baro_find_person_line(meta)),
			tool.HTML_escape(field_name),
			tool.HTML_escape(note[title]),
		})
	}

	return rows
}

// View_baro_find_discover : 5-4 발견 기능
func View_baro_find_discover(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = page
	_ = values

	hidden := baro_find_hidden_set(db)
	meta_map := baro_find_meta_map(db)
	pick_list := baro_find_pick_titles(db)

	body := baro_find_tabs("/baro/discover")
	body += tool.Baro_hero(
		tool.Baro_t("발견"),
		tool.Baro_t("오늘 처음 만나는 기록을 골라 드립니다."),
		tool.Baro_button("/baro/discover/random", tool.Baro_t("랜덤 문서로 이동"), "primary")+" "+
			tool.Baro_button("/baro/discover/recent", tool.Baro_t("최근 변경 보기"), "")+" "+
			tool.Baro_button("/baro/search", tool.Baro_t("검색으로 찾기"), ""),
	)

	// 오늘의 문서 (날짜 해시로 결정론적으로 고른다)
	today := tool.Baro_today()
	today_doc := baro_find_daily_pick(pick_list, "baro_find_today_"+today)

	// 오늘 처음 기록된 인물
	person_rows := [][]string{}
	new_rows := [][]string{}
	new_note := map[string]string{}
	new_titles := []string{}
	person_titles := []string{}
	person_note := map[string]string{}

	rows := tool.Query_DB(db, "select title, date, ip from history where type = 'r1' or id = '1' order by date desc limit 200")
	defer rows.Close()

	for rows.Next() {
		title, date, ip := "", "", ""
		if err := rows.Scan(&title, &date, &ip); err != nil {
			continue
		}
		if baro_find_excluded(title, hidden) {
			continue
		}

		if len(new_titles) < 10 {
			new_titles = append(new_titles, title)
			new_note[title] = tool.Baro_time_ago(date) + " · " + tool.Baro_mask_ip(ip)
		}

		meta := meta_map[title]
		is_person := meta != nil && (meta["birth_year"] != "" || meta["job"] != "" || meta["field"] == "person")
		if is_person && strings.HasPrefix(date, today) && len(person_titles) < 10 {
			person_titles = append(person_titles, title)
			person_note[title] = tool.Baro_t("오늘 ") + tool.Baro_time_ago(date)
		}
	}

	new_rows = baro_find_title_rows(db, new_titles, meta_map, new_note)
	person_rows = baro_find_title_rows(db, person_titles, meta_map, person_note)

	// 크게 보강된 문서 (최근 편집 중 늘어난 분량이 큰 것)
	type grow struct {
		Title string
		Delta int
		Date  string
	}

	grow_list := []grow{}
	grow_rows := tool.Query_DB(db, "select title, leng, date from history order by date desc limit 400")
	defer grow_rows.Close()

	for grow_rows.Next() {
		title, leng, date := "", "", ""
		if err := grow_rows.Scan(&title, &leng, &date); err != nil {
			continue
		}
		if baro_find_excluded(title, hidden) {
			continue
		}

		delta := tool.Str_to_int(strings.TrimPrefix(leng, "+"))
		if delta < 1000 {
			continue
		}

		grow_list = append(grow_list, grow{title, delta, date})
	}

	sort.SliceStable(grow_list, func(a int, b int) bool { return grow_list[a].Delta > grow_list[b].Delta })
	if len(grow_list) > 10 {
		grow_list = grow_list[:10]
	}

	grow_titles := []string{}
	grow_note := map[string]string{}
	for _, row := range grow_list {
		grow_titles = append(grow_titles, row.Title)
		grow_note[row.Title] = "+" + tool.Baro_comma(row.Delta) + tool.Baro_t("자 · ") + tool.Baro_time_ago(row.Date)
	}

	// 출처가 필요한 문서
	source_titles := []string{}
	source_note := map[string]string{}
	source_rows := tool.Query_DB(db, "select title, reason, date from baro_doc_status where status = 'need_source' order by date desc limit 30")
	defer source_rows.Close()

	for source_rows.Next() {
		title, reason, date := "", "", ""
		if err := source_rows.Scan(&title, &reason, &date); err != nil {
			continue
		}
		if baro_find_excluded(title, hidden) || len(source_titles) >= 10 {
			continue
		}

		source_titles = append(source_titles, title)
		source_note[title] = tool.Baro_trunc(reason, 40)
		if source_note[title] == "" {
			source_note[title] = tool.Baro_time_ago(date)
		}
	}

	// 관심 분야 추천
	interest_body := ""
	if tool.Baro_is_user(config.IP) {
		fields := tool.Baro_user_interest(db, config.IP)
		if len(fields) == 0 {
			interest_body = tool.Baro_alert("info", tool.Baro_t(`아직 관심 분야를 고르지 않으셨습니다. `)+tool.Baro_button("/baro/founding", tool.Baro_t("관심 분야 고르기"), "primary"))
		} else {
			pills := []string{}
			for _, key := range fields {
				pills = append(pills, tool.Baro_field_name(db, key))
			}

			rec_titles := []string{}
			rec_note := map[string]string{}
			for _, key := range fields {
				for _, title := range baro_find_meta_title_list(db, "field", key, 60) {
					if baro_find_excluded(title, hidden) || len(rec_titles) >= 12 {
						continue
					}

					rec_titles = append(rec_titles, title)
					rec_note[title] = tool.Baro_field_name(db, key)
				}
			}

			interest_body = tool.Baro_pills(pills)
			interest_body += tool.Baro_table(
				[]string{"문서", "인물 정보", "분야", tool.Baro_t("추천 이유")},
				baro_find_title_rows(db, rec_titles, meta_map, rec_note),
				tool.Baro_t("관심 분야로 등록된 문서가 아직 없습니다. 첫 문서를 만들어 주세요."),
			)
		}
	} else {
		interest_body = tool.Baro_alert("info", tool.Baro_t(`로그인하면 관심 분야에 맞춘 문서를 추천해 드립니다. `)+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary"))
	}

	// 이번 주 숨은 기록 (조회는 적지만 출처가 잘 달린 문서)
	type quiet struct {
		Title string
		Views int
		Cites int
		Date  string
	}

	quiet_list := []quiet{}
	quiet_rows := tool.Query_DB(db, "select title, count(*) from baro_citation group by title limit 500")
	defer quiet_rows.Close()

	for quiet_rows.Next() {
		title := ""
		count := 0
		if err := quiet_rows.Scan(&title, &count); err != nil {
			continue
		}
		if count < 2 || baro_find_excluded(title, hidden) {
			continue
		}

		last := baro_find_last_edit(db, title)
		if last == "" || tool.Baro_days_between(last, tool.Baro_now()) > 7 {
			continue
		}

		views := baro_find_view_count(db, title)
		if views > 50 {
			continue
		}

		quiet_list = append(quiet_list, quiet{title, views, count, last})
	}

	sort.SliceStable(quiet_list, func(a int, b int) bool {
		if quiet_list[a].Views != quiet_list[b].Views {
			return quiet_list[a].Views < quiet_list[b].Views
		}

		return quiet_list[a].Cites > quiet_list[b].Cites
	})
	if len(quiet_list) > 10 {
		quiet_list = quiet_list[:10]
	}

	quiet_titles := []string{}
	quiet_note := map[string]string{}
	for _, row := range quiet_list {
		quiet_titles = append(quiet_titles, row.Title)
		quiet_note[row.Title] = tool.Baro_t("조회 ") + tool.Baro_comma(row.Views) + tool.Baro_t(" · 출처 ") + tool.Baro_itoa(row.Cites) + tool.Baro_t("개")
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("살아 있는 문서"), tool.Baro_comma(len(pick_list)), tool.Baro_t("격리·연습장 제외")},
		{tool.Baro_t("새 문서(최근)"), tool.Baro_comma(len(new_titles)), tool.Baro_t("가장 최근에 만들어진 것")},
		{tool.Baro_t("출처 필요"), tool.Baro_comma(len(source_titles)), tool.Baro_t("지금 도와줄 수 있는 일")},
		{tool.Baro_t("숨은 기록"), tool.Baro_comma(len(quiet_titles)), tool.Baro_t("이번 주")},
	})

	body += tool.Baro_grid([]string{
		baro_find_doc_card(db, today_doc, tool.Baro_t("오늘의 문서 (")+today+")",
			tool.Baro_note(tool.Baro_t("날짜를 해시해 고르므로 하루 동안 모두에게 같은 문서가 보입니다."))),
		tool.Baro_card_ex(tool.Baro_t("오늘 처음 기록된 인물"), tool.Baro_t("오늘 새로 만들어진 인물 문서"),
			tool.Baro_table([]string{"문서", "인물 정보", "분야", tool.Baro_t("기록 시각")}, person_rows, tool.Baro_t("오늘은 아직 새 인물 기록이 없습니다. 첫 인물을 남겨 주세요.")), ""),
	}, 2)

	body += tool.Baro_card(tool.Baro_t("새로 생성된 문서"),
		tool.Baro_table([]string{"문서", "인물 정보", "분야", tool.Baro_t("생성")}, new_rows, tool.Baro_t("아직 새로 생성된 문서가 없습니다.")))

	body += tool.Baro_card_ex(tool.Baro_t("크게 보강된 문서"), tool.Baro_t("최근 편집에서 1,000자 이상 늘어난 문서"),
		tool.Baro_table([]string{"문서", "인물 정보", "분야", tool.Baro_t("늘어난 분량")},
			baro_find_title_rows(db, grow_titles, meta_map, grow_note),
			tool.Baro_t("최근에 크게 보강된 문서가 없습니다.")), "")

	body += tool.Baro_card_ex(tool.Baro_t("출처가 필요한 문서"), tool.Baro_t("문서 상태가 '출처 필요' 인 문서"),
		tool.Baro_table([]string{"문서", "인물 정보", "분야", "사유"},
			baro_find_title_rows(db, source_titles, meta_map, source_note),
			tool.Baro_t("출처가 필요한 문서로 표시된 문서가 없습니다.")),
		tool.Baro_button("/baro/source", tool.Baro_t("출처 품질 화면"), ""))

	body += tool.Baro_card(tool.Baro_t("관심 분야 추천"), interest_body)

	body += tool.Baro_card_ex(tool.Baro_t("이번 주 숨은 기록"), tool.Baro_t("조회는 적지만 출처가 잘 달린 문서"),
		tool.Baro_table([]string{"문서", "인물 정보", "분야", tool.Baro_t("지표")},
			baro_find_title_rows(db, quiet_titles, meta_map, quiet_note),
			tool.Baro_t("조건에 맞는 숨은 기록이 아직 없습니다. (출처 2개 이상 · 최근 7일 편집 · 조회 50회 이하)")),
		tool.Baro_note(tool.Baro_t("잘 만든 기록이 묻히지 않도록 조회수가 아니라 출처 밀도로 골랐습니다.")))

	return tool.Baro_page(db, config, tool.Baro_t("발견"), body, baro_find_menu())
}

// View_baro_find_random : 랜덤 문서로 이동
func View_baro_find_random(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	list := baro_find_pick_titles(db)
	if len(list) == 0 {
		body := baro_find_tabs("/baro/discover")
		body += tool.Baro_alert("warn", tool.Baro_t("아직 보여 드릴 문서가 없습니다. 첫 문서를 만들어 주세요."))
		body += tool.Baro_card(tool.Baro_t("첫 기록 남기기"), tool.Baro_button("/edit/", tool.Baro_t("새 문서 만들기"), "primary"))

		return tool.Baro_page(db, config, tool.Baro_t("랜덤 문서"), body, baro_find_menu())
	}

	// 매번 달라지도록 임의 키를 섞는다.
	pick := baro_find_daily_pick(list, tool.Get_random_key(16))

	return tool.Get_redirect("/w/" + tool.Url_parser(pick))
}
