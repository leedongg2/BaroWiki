package route

// 바로위키 8장 — 커뮤니티 · 협업 (공통 도우미와 토론 허브)
//
// 이 파일의 비공개 식별자는 모두 baro_community_ 로 시작한다.

import (
	"database/sql"
	"net/url"
	"regexp"
	"strings"

	"opennamu/route/tool"
)

// baro_community_page_size : 목록 한 쪽에 보여줄 수
const baro_community_page_size = 30

// baro_community_new_days : 신규 이용자로 보는 기간(일)
const baro_community_new_days = 30

// baro_community_mention_re : 본문 안의 @사용자이름 찾기
var baro_community_mention_re = regexp.MustCompile(`@([\p{L}\p{N}_.\-]{2,32})`)

// baro_community_menu : 이 모듈 화면의 공통 메뉴
func baro_community_menu() [][]any {
	return [][]any{
		{"baro", "바로위키"},
		{"baro/talk", tool.Baro_t("토론")},
		{"baro/watch", tool.Baro_t("주시 목록")},
		{"baro/mentor", tool.Baro_t("멘토링")},
	}
}

// baro_community_page_int : 쪽 번호 정규화
func baro_community_page_int(page string) int {
	num := tool.Str_to_int(page)
	if num < 1 {
		num = 1
	}

	return num
}

// baro_community_get : url.Values 에서 공백을 다듬어 꺼낸다
func baro_community_get(values url.Values, name string) string {
	if values == nil {
		return ""
	}

	return strings.TrimSpace(values.Get(name))
}

// baro_community_trim_path : 경로 조각(*param) 앞뒤의 / 를 지운다
func baro_community_trim_path(data string) string {
	return strings.Trim(strings.TrimSpace(data), "/")
}

// baro_community_user_exists : 가입된 사용자인지 확인
func baro_community_user_exists(db *sql.DB, user string) bool {
	if !tool.Baro_is_user(user) {
		return false
	}

	found := ""
	tool.QueryRow_DB(db, "select id from user_set where id = ? and name = 'date' limit 1", []any{&found}, user)

	return found != ""
}

// baro_community_join_date : 가입 시각 (없으면 "")
func baro_community_join_date(db *sql.DB, user string) string {
	if !tool.Baro_is_user(user) {
		return ""
	}

	date := ""
	tool.QueryRow_DB(db, "select data from user_set where id = ? and name = 'date' limit 1", []any{&date}, user)

	return date
}

// baro_community_is_new_user : 가입 30일 이내인지
//
//	가입 시각을 읽지 못하는 계정은 신규로 보지 않는다.
//	(읽지 못한 값을 0일로 계산해 오래된 계정까지 신규로 표시하던 문제를 막는다)
func baro_community_is_new_user(db *sql.DB, user string) bool {
	date := baro_community_join_date(db, user)
	if date == "" || tool.Baro_parse_time(date).IsZero() {
		return false
	}

	days := tool.Baro_days_between(date, tool.Baro_now())

	return days >= 0 && days <= baro_community_new_days
}

// baro_community_new_user_list : 보호 기간(가입 30일 이내)에 있는 기여자 목록
//
//	가입 최근순으로 훑어 보호 기간에 있는 계정을 고르고, 기여 수를 함께 센다.
func baro_community_new_user_list(db *sql.DB, limit int) [][]string {
	out := [][]string{}
	if limit < 1 {
		limit = 10
	}

	rows := tool.Query_DB(db, "select id, data from user_set where name = 'date' order by data desc limit 200")
	defer rows.Close()

	for rows.Next() {
		user, date := "", ""
		if err := rows.Scan(&user, &date); err != nil {
			continue
		}

		if !baro_community_is_new_user(db, user) {
			continue
		}

		count := 0
		tool.QueryRow_DB(db, "select count(*) from baro_contrib_event where user = ?", []any{&count}, user)

		out = append(out, []string{user, date, tool.Baro_itoa(count)})
		if len(out) >= limit {
			break
		}
	}

	return out
}

// baro_community_subject_norm : 글 제목을 다듬는다
//
//	제목을 note 칸에 저장하는데 답글은 "re:<부모 id>" 로 적으므로,
//	제목이 re: 로 시작하면 답글로 오인되어 목록에서 사라진다. 그래서 앞머리를 바꾼다.
func baro_community_subject_norm(subject string) string {
	subject = strings.TrimSpace(subject)
	if len(subject) >= 3 && strings.EqualFold(subject[:3], "re:") {
		subject = strings.TrimSpace(tool.Baro_t("답글 ") + strings.TrimSpace(subject[3:]))
	}

	return subject
}

// baro_community_welcome_box : 신규 이용자 환영 안내 (8-2)
func baro_community_welcome_box(db *sql.DB, user string) string {
	if !baro_community_is_new_user(db, user) {
		return ""
	}

	body := `<p>` + tool.HTML_escape(user) + tool.Baro_t(` 님은 가입한 지 얼마 되지 않은 새 이용자입니다. 반갑습니다.</p>`)
	body += tool.Baro_t(`<p>처음이라 서툰 점이 있을 수 있습니다. 되돌리기보다 먼저 말을 걸어 주세요.</p>`)
	body += tool.Baro_button("/baro/help", tool.Baro_t("바로위키 도움말"), "primary") + " "
	body += tool.Baro_button("/w/help:편집 방법", tool.Baro_t("편집 방법"), "") + " "
	body += tool.Baro_button("/baro/mentor", tool.Baro_t("멘토 찾기"), "")

	return tool.Baro_card(tool.Baro_t("새 이용자를 환영합니다"), body)
}

// baro_community_newbie_guard : 초보 기여자 보호 기간 안내 문구 (8-5)
func baro_community_newbie_guard(db *sql.DB, user string) string {
	if !baro_community_is_new_user(db, user) {
		return ""
	}

	left := baro_community_new_days - tool.Baro_days_between(baro_community_join_date(db, user), tool.Baro_now())
	if left < 0 {
		left = 0
	}

	return tool.Baro_alert(
		"info",
		tool.Baro_t("초보 기여자 보호 기간입니다 (남은 기간 ")+tool.Baro_itoa(left)+tool.Baro_t("일). ")+
			tool.Baro_t("이 사용자의 편집은 되돌리기 전에 먼저 토론으로 말을 걸어 주세요."),
	)
}

// baro_community_mention_list : 본문에서 @사용자이름 을 뽑는다 (중복 제거)
func baro_community_mention_list(data string) []string {
	out := []string{}
	seen := map[string]bool{}

	for _, match := range baro_community_mention_re.FindAllStringSubmatch(data, 40) {
		name := strings.Trim(match[1], ".-")
		if name == "" || seen[name] {
			continue
		}

		seen[name] = true
		out = append(out, name)
	}

	return out
}

// baro_community_notify_mentions : 본문 속 언급 대상에게 알림을 보낸다
//
//	보낸 사람과 같은 사용자, 가입하지 않은 이름은 건너뛴다. 보낸 수를 돌려준다.
func baro_community_notify_mentions(db *sql.DB, data string, from string, title string, link string) int {
	sent := 0
	for _, name := range baro_community_mention_list(data) {
		if name == from || !baro_community_user_exists(db, name) {
			continue
		}

		tool.Baro_notify(db, name, "mention", title, link, tool.Baro_trunc(tool.Baro_one_line(data), 200))
		sent++
	}

	return sent
}

// baro_community_thread_count : 스레드의 글 수
func baro_community_thread_count(db *sql.DB, code string) int {
	count := 0
	tool.QueryRow_DB(db, "select count(*) from topic where code = ?", []any{&count}, code)

	return count
}

// baro_community_thread_last : 스레드의 마지막 활동 시각
func baro_community_thread_last(db *sql.DB, code string) string {
	date := ""
	tool.QueryRow_DB(db, "select date from topic where code = ? order by id + 0 desc limit 1", []any{&date}, code)

	return date
}

// baro_community_meta_name : 스레드별 부가 정보 이름표 (baro_doc_meta 에 저장)
func baro_community_meta_name(prefix string, code string) string {
	return "talk_" + prefix + "_" + code
}

// baro_community_report_link : 신고 화면으로 가는 링크
func baro_community_report_link(target_kind string, target string, text string) string {
	link := "/baro/report/new?target_kind=" + tool.Url_parser(target_kind) + "&target=" + tool.Url_parser(target)

	return tool.Baro_button(link, text, "")
}

// baro_community_talk_tabs : 토론 화면 탭
func baro_community_talk_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/talk", tool.Baro_t("토론 허브")},
		{"/baro/talk/project/", "프로젝트 토론"},
		{"/baro/talk/mention", tool.Baro_t("언급 처리")},
		{"/baro/watch", tool.Baro_t("주시 목록")},
		{"/baro/mentor", tool.Baro_t("멘토링")},
	}, active)
}

// baro_community_write_block : 읽기 전용 점검 중이면 안내 문구를 돌려준다
func baro_community_write_block(db *sql.DB) string {
	if !tool.Baro_read_only(db) {
		return ""
	}

	reason := tool.Baro_read_only_reason(db)
	if reason == "" {
		reason = "점검 중입니다."
	}

	return tool.HTML_escape(tool.Baro_t("지금은 읽기 전용 상태라 글을 남길 수 없습니다. (") + reason + ")")
}

// View_baro_community_talk_home : 8장 토론 허브 — 최근 토론을 한자리에 모은다
func View_baro_community_talk_home(config tool.Config, query url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	// 문서 이름을 넣었으면 그 문서 토론으로 보낸다
	find := baro_community_get(query, "doc")
	if find != "" {
		return tool.Get_redirect("/baro/talk/doc/" + tool.Url_parser(tool.Namespace_normalize(find)))
	}

	// 사용자 이름을 넣었으면 그 사용자 토론으로 보낸다
	who := baro_community_get(query, "user")
	if who != "" {
		return tool.Get_redirect("/baro/talk/user/" + tool.Url_parser(who))
	}

	body := tool.Baro_hero(
		tool.Baro_t("토론 · 협업"),
		tool.Baro_t("문서 토론, 사용자 토론, 프로젝트 토론을 한자리에서 봅니다."),
		tool.Baro_button("/baro/watch", tool.Baro_t("내 주시 목록"), "primary")+" "+
			tool.Baro_button("/baro/talk/project/", "프로젝트 토론", "")+" "+
			tool.Baro_button("/baro/mentor", tool.Baro_t("멘토링"), "")+" "+
			tool.Baro_button("/baro/talk/mention", tool.Baro_t("언급 처리"), ""),
	)
	body += baro_community_talk_tabs("/baro/talk")

	form := tool.Baro_input("doc", tool.Baro_t("문서 이름"), "", tool.Baro_t("예: 바로위키"), tool.Baro_t("그 문서의 토론을 바로위키 형식으로 봅니다."))
	form += tool.Baro_submit(tool.Baro_t("문서 토론 열기"), "primary")

	user_form := tool.Baro_input("user", tool.Baro_t("사용자 이름"), "", tool.Baro_t("예: 김기록"), tool.Baro_t("그 사용자에게 공개 메시지를 남기는 화면으로 갑니다."))
	user_form += tool.Baro_submit(tool.Baro_t("사용자 토론 열기"), "")

	body += tool.Baro_grid([]string{
		tool.Baro_card(tool.Baro_t("문서 토론 찾아가기"), tool.Baro_form("/baro/talk", form)),
		tool.Baro_card(tool.Baro_t("사용자 토론 찾아가기"), tool.Baro_form("/baro/talk", user_form)),
	}, 2)

	// 최근 문서 토론
	doc_rows := [][]string{}
	rows := tool.Query_DB(db, "select title, sub, code, date, stop from rd order by date desc limit 15")
	defer rows.Close()

	for rows.Next() {
		title, sub, code, date, stop := "", "", "", "", ""
		if err := rows.Scan(&title, &sub, &code, &date, &stop); err != nil {
			continue
		}

		state := tool.Baro_badge(tool.Baro_t("진행 중"), "info")
		if stop == "O" {
			state = tool.Baro_badge(tool.Baro_t("종료"), "muted")
		}
		if tool.Baro_doc_meta_get(db, title, baro_community_meta_name("solved", code)) != "" {
			state = tool.Baro_badge(tool.Baro_t("해결됨"), "ok")
		}

		doc_rows = append(doc_rows, []string{
			tool.Baro_doc_link(title),
			`<a href="/baro/talk/doc/` + tool.Url_parser(title) + `">` + tool.HTML_escape(tool.Baro_trunc(sub, 60)) + `</a>`,
			tool.Baro_itoa(baro_community_thread_count(db, code)),
			tool.Baro_time_cell(date),
			state,
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("최근 문서 토론"),
		tool.Baro_table([]string{"문서", tool.Baro_t("주제"), tool.Baro_t("글 수"), tool.Baro_t("시작"), "상태"}, doc_rows, tool.Baro_t("아직 문서 토론이 없습니다.")),
	)

	// 최근 사용자 토론
	user_rows := [][]string{}
	user_list := tool.Query_DB(
		db,
		"select id, title, note, user, date from baro_workspace where project = 'usertalk' and status = 'open' order by id + 0 desc limit 10",
	)
	defer user_list.Close()

	for user_list.Next() {
		id, title, note, user, date := "", "", "", "", ""
		if err := user_list.Scan(&id, &title, &note, &user, &date); err != nil {
			continue
		}

		subject := note
		if strings.HasPrefix(note, "re:") {
			subject = tool.Baro_t("답글")
		}

		user_rows = append(user_rows, []string{
			`<a href="/baro/talk/user/` + tool.Url_parser(title) + `">` + tool.HTML_escape(title) + `</a>`,
			tool.HTML_escape(tool.Baro_trunc(subject, 50)),
			tool.Baro_user_link(db, user, config.IP),
			tool.Baro_time_cell(date),
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("최근 사용자 토론"),
		tool.Baro_table([]string{tool.Baro_t("대상"), tool.Baro_t("제목"), tool.Baro_t("남긴 사람"), "시각"}, user_rows, tool.Baro_t("아직 사용자 토론이 없습니다.")),
	)

	// 최근 프로젝트 토론
	project_rows := [][]string{}
	project_list := tool.Query_DB(
		db,
		"select id, project, title, note, user, date from baro_workspace where project like 'projecttalk:%' and status = 'open' order by id + 0 desc limit 10",
	)
	defer project_list.Close()

	for project_list.Next() {
		id, project, title, note, user, date := "", "", "", "", "", ""
		if err := project_list.Scan(&id, &project, &title, &note, &user, &date); err != nil {
			continue
		}

		project_id := strings.TrimPrefix(project, "projecttalk:")
		project_rows = append(project_rows, []string{
			`<a href="/baro/talk/project/` + tool.Url_parser(project_id) + `">` + tool.HTML_escape(baro_community_project_name(db, project_id)) + `</a>`,
			tool.HTML_escape(tool.Baro_trunc(title, 50)),
			tool.HTML_escape(baro_community_kind_name(strings.TrimPrefix(note, "kind:"))),
			tool.Baro_user_link(db, user, config.IP),
			tool.Baro_time_cell(date),
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("최근 프로젝트 토론"),
		tool.Baro_table([]string{"프로젝트", tool.Baro_t("제목"), "종류", tool.Baro_t("글쓴이"), "시각"}, project_rows, tool.Baro_t("아직 프로젝트 토론이 없습니다.")),
	)

	body += tool.Baro_note(tool.Baro_t("토론은 실시간 대화가 아니라 검색되고 남는 기록입니다. 결론은 요약해서 남겨 주세요."))

	return tool.Baro_page(db, config, tool.Baro_t("토론 · 협업"), body, baro_community_menu())
}
