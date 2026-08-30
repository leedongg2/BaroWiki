package route

import (
	"database/sql"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 창립 시즌 모듈 (founding)
//
//  1-2 100인 창립단 · 1-3 관심 분야 · 1-4 분야별 모집 ·
//  1-5 추천/초대 · 1-6 공유 카드
//
//  이 파일은 위 화면들이 함께 쓰는 도우미만 모아 둔다.
//  다른 모듈의 Go 함수는 부르지 않고 필요한 표는 SQL 로 직접 읽는다.
// ============================================================

// baro_founding_person : baro_founder 표 한 줄
type baro_founding_person struct {
	User       string
	Number     string
	Date       string
	Status     string
	FirstEvent string
	Field      string
	ApplyDate  string
	Note       string
}

// baro_founding_menu : 창립 시즌 화면의 스킨 메뉴
func baro_founding_menu() [][]any {
	return [][]any{
		{"baro", "바로위키"},
		{"baro/founding", tool.Baro_t("창립 시즌")},
		{"baro/founding/list", tool.Baro_t("창립단 명단")},
		{"baro/founding/hall", tool.Baro_t("기록관")},
	}
}

// baro_founding_tabs : 창립 시즌 공통 탭 줄
func baro_founding_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/founding", tool.Baro_t("창립 시즌")},
		{"/baro/founding/apply", tool.Baro_t("창립단 지원")},
		{"/baro/founding/check", tool.Baro_t("내 지원 상태")},
		{"/baro/founding/field", tool.Baro_t("분야별 모집")},
		{"/baro/founding/list", tool.Baro_t("창립단 명단")},
		{"/baro/founding/hall", tool.Baro_t("기록관")},
		{"/baro/interest", tool.Baro_t("관심 분야")},
		{"/baro/invite", tool.Baro_t("초대")},
		{"/baro/card", tool.Baro_t("공유 카드")},
	}, active)
}

// baro_founding_page_num : 쪽 번호 정규화 (1 미만이면 1)
func baro_founding_page_num(page string) int {
	num := tool.Str_to_int(page)
	if num < 1 {
		num = 1
	}

	return num
}

// baro_founding_goal : 창립단 정원 (baro_set 의 founding_goal)
func baro_founding_goal(db *sql.DB) int {
	goal := tool.Baro_get_int(db, "founding_goal", 100)
	if goal < 1 {
		goal = 1
	}

	return goal
}

// baro_founding_open : 창립 지원을 받는 중인지
func baro_founding_open(db *sql.DB) bool {
	return tool.Baro_get_def(db, "founding_open", "on") == "on"
}

// baro_founding_find : 사용자의 창립단 기록
func baro_founding_find(db *sql.DB, user string) (baro_founding_person, bool) {
	row := baro_founding_person{}
	found := tool.QueryRow_DB(
		db,
		"select user, number, date, status, first_event, field, apply_date, note from baro_founder where user = ?",
		[]any{&row.User, &row.Number, &row.Date, &row.Status, &row.FirstEvent, &row.Field, &row.ApplyDate, &row.Note},
		user,
	)

	return row, found
}

// baro_founding_status_count : 상태별 인원 수
func baro_founding_status_count(db *sql.DB, status string) int {
	num := 0
	tool.QueryRow_DB(db, "select count(*) from baro_founder where status = ?", []any{&num}, status)

	return num
}

// baro_founding_field_count : 분야별 지원(또는 확정) 인원 수
func baro_founding_field_count(db *sql.DB, field string, status string) int {
	num := 0
	if status == "" {
		tool.QueryRow_DB(db, "select count(*) from baro_founder where field = ? and status != 'rejected'", []any{&num}, field)

		return num
	}

	tool.QueryRow_DB(db, "select count(*) from baro_founder where field = ? and status = ?", []any{&num}, field, status)

	return num
}

// baro_founding_max_number : 이미 확정된 번호 중 최대값
func baro_founding_max_number(db *sql.DB) int {
	number := "0"
	tool.QueryRow_DB(
		db,
		"select number from baro_founder where status = 'confirmed' and number != '' order by number + 0 desc limit 1",
		[]any{&number},
	)

	return tool.Str_to_int(number)
}

// baro_founding_first_valid : 번호 발급 근거가 되는 첫 유효 기여
//
//	가입 시점이 아니라 "승인된 첫 유효 기여" 를 기준으로 삼는다.
//	(계정만 100개 만들어 번호를 파는 행위를 막기 위한 규칙)
//	반환값 : {이벤트 ID, 종류, 문서, 시각} / 없으면 nil
func baro_founding_first_valid(db *sql.DB, user string) []string {
	if !tool.Baro_is_user(user) {
		return nil
	}

	id, kind, title, date := "", "", "", ""
	found := tool.QueryRow_DB(
		db,
		"select id, kind, title, date from baro_contrib_event where user = ? and (status = 'confirmed' or kind = 'new_document' or kind = 'source') order by id + 0 asc limit 1",
		[]any{&id, &kind, &title, &date},
		user,
	)
	if !found {
		return nil
	}

	return []string{id, kind, title, date}
}

// baro_founding_event_text : 기여 이벤트 ID 를 사람이 읽는 형태로
func baro_founding_event_text(db *sql.DB, event string) string {
	if strings.TrimSpace(event) == "" {
		return `<span class="bw_muted">-</span>`
	}

	kind, title, date := "", "", ""
	if !tool.QueryRow_DB(
		db,
		"select kind, title, date from baro_contrib_event where id = ?",
		[]any{&kind, &title, &date},
		event,
	) {
		return tool.Baro_t(`<span class="bw_muted">기록 없음</span>`)
	}

	html := tool.Baro_badge(tool.Baro_contrib_kind_name(kind), "info")
	if title != "" {
		html += " " + tool.Baro_doc_link(title)
	}

	return html
}

// baro_founding_give_badge : 배지 지급 (이미 있으면 아무것도 하지 않는다)
func baro_founding_give_badge(db *sql.DB, user string, badge string, detail string) bool {
	if !tool.Baro_is_user(user) {
		return false
	}

	found := ""
	if tool.QueryRow_DB(db, "select id from baro_user_badge where user = ? and badge = ?", []any{&found}, user, badge) {
		return false
	}

	tool.Exec_DB(
		db,
		"insert into baro_user_badge (id, user, badge, date, detail) values (?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_user_badge"), user, badge, tool.Baro_now(), detail,
	)

	return true
}

// baro_founding_badge_name : 배지 표시 이름
func baro_founding_badge_name(db *sql.DB, badge string) string {
	name := ""
	tool.QueryRow_DB(db, "select name from baro_badge where key_name = ?", []any{&name}, badge)
	if name == "" {
		return badge
	}

	return name
}

// baro_founding_promote : 지원자에게 창립 번호를 발급한다 (자동 승격)
//
//	조건 : status 가 applied 이고, 유효 기여가 하나라도 확인되어야 한다.
//	정원을 넘으면 발급하지 않고 대기 상태로 둔다.
//	반환값 : 발급된 번호 (발급하지 않았으면 "")
func baro_founding_promote(db *sql.DB, user string) string {
	person, found := baro_founding_find(db, user)
	if !found || person.Status != "applied" {
		return ""
	}

	event := baro_founding_first_valid(db, user)
	if event == nil {
		return ""
	}

	next := baro_founding_max_number(db) + 1
	if next > baro_founding_goal(db) {
		return ""
	}

	number := tool.Baro_itoa(next)
	tool.Exec_DB(
		db,
		"update baro_founder set status = 'confirmed', number = ?, date = ?, first_event = ? where user = ?",
		number, tool.Baro_now(), event[0], user,
	)

	baro_founding_give_badge(db, user, "founder", "창립 기여자 #"+number)
	tool.Baro_notify(
		db, user, "badge",
		"창립 기여자 번호 #"+number+" 가 확정되었습니다.",
		"/baro/founding/check", "founder",
	)
	tool.Baro_public_log(db, "founder_confirm", "system", user, "첫 유효 기여 확인", "창립 기여자 #"+number)
	tool.Baro_metric_bump(db, "founder_confirm", tool.Baro_today(), 1)

	return number
}

// baro_founding_pending_users : 아직 번호를 받지 못한 지원자
func baro_founding_pending_users(db *sql.DB, limit int) []string {
	out := []string{}
	if limit < 1 {
		limit = 300
	}

	rows := tool.Query_DB(
		db,
		"select user from baro_founder where status = 'applied' order by apply_date asc limit ?, ?",
		0, limit,
	)
	defer rows.Close()

	for rows.Next() {
		user := ""
		if err := rows.Scan(&user); err != nil {
			continue
		}

		out = append(out, user)
	}

	return out
}

// baro_founding_promote_all : 대기 중인 지원자 전체에 대해 승격 검사
func baro_founding_promote_all(db *sql.DB) int {
	count := 0
	for _, user := range baro_founding_pending_users(db, 300) {
		if baro_founding_promote(db, user) != "" {
			count++
		}
	}

	return count
}

// baro_founding_list_rows : 창립단 명단 (번호순)
func baro_founding_list_rows(db *sql.DB, offset int, limit int) []baro_founding_person {
	out := []baro_founding_person{}

	rows := tool.Query_DB(
		db,
		"select user, number, date, status, first_event, field, apply_date, note from baro_founder where status = 'confirmed' order by number + 0 asc limit ?, ?",
		offset, limit,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_founding_person{}
		if err := rows.Scan(&row.User, &row.Number, &row.Date, &row.Status, &row.FirstEvent, &row.Field, &row.ApplyDate, &row.Note); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_founding_status_rows : 상태로 걸러낸 지원자 목록 (지원 순)
func baro_founding_status_rows(db *sql.DB, status string, offset int, limit int) []baro_founding_person {
	out := []baro_founding_person{}

	rows := tool.Query_DB(
		db,
		"select user, number, date, status, first_event, field, apply_date, note from baro_founder where status = ? order by apply_date asc limit ?, ?",
		status, offset, limit,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_founding_person{}
		if err := rows.Scan(&row.User, &row.Number, &row.Date, &row.Status, &row.FirstEvent, &row.Field, &row.ApplyDate, &row.Note); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_founding_field_options : 분야 선택 상자 항목
func baro_founding_field_options(db *sql.DB, head string) [][]string {
	out := [][]string{}
	if head != "" {
		out = append(out, []string{"", head})
	}

	for _, field := range tool.Baro_field_list(db) {
		out = append(out, []string{field.Key, field.Name})
	}

	return out
}

// baro_founding_field_ok : 실제로 등록된 분야인지
func baro_founding_field_ok(db *sql.DB, field string) bool {
	if strings.TrimSpace(field) == "" {
		return false
	}

	for _, row := range tool.Baro_field_list(db) {
		if row.Key == field {
			return true
		}
	}

	return false
}

// baro_founding_interest_add : 관심 분야를 아직 고르지 않았다면 하나 넣어 준다
func baro_founding_interest_add(db *sql.DB, user string, field string, kind string) {
	if !tool.Baro_is_user(user) || strings.TrimSpace(field) == "" {
		return
	}

	found := ""
	if tool.QueryRow_DB(db, "select field from baro_interest where user = ? and field = ?", []any{&found}, user, field) {
		return
	}

	tool.Exec_DB(
		db,
		"insert into baro_interest (user, field, kind, date) values (?, ?, ?, ?)",
		user, field, kind, tool.Baro_now(),
	)
}

// baro_founding_benefit_html : 창립 기여자 혜택 안내 (1-2)
func baro_founding_benefit_html() string {
	return tool.Baro_list([]string{
		tool.Baro_t("<b>프로필 영구 표시</b> — 창립 기여자 번호가 프로필과 기여 기록 옆에 영구히 남습니다."),
		tool.Baro_t("<b>전용 배지</b> — 창립 기여자 배지가 지급되며 회수되지 않습니다."),
		tool.Baro_t("<b>기록관 영구 등재</b> — 창립 시즌 기록관에 이름과 첫 기여가 영구히 보존됩니다."),
		tool.Baro_t("<b>초대 코드 지급</b> — 함께할 사람을 부를 수 있는 초대 링크를 받습니다."),
		tool.Baro_t("<b>창립 시즌 어워드 후보 자격</b> — 창립 시즌 어워드의 후보가 될 수 있습니다."),
	}) + tool.Baro_note(tool.Baro_t("혜택은 명예와 기록에 대한 것입니다. 현금성 보상이나 운영 권한은 포함되지 않습니다."))
}

// baro_founding_rule_html : 번호 확정 규칙 안내 (봇 대량 가입 방지)
func baro_founding_rule_html() string {
	html := tool.Baro_t(`<p>창립 기여자 번호는 <b>가입한 순서가 아니라 첫 유효 기여가 확인된 순서</b>로 확정됩니다.</p>`)
	html += tool.Baro_list([]string{
		tool.Baro_t("지원하면 먼저 <b>대기(applied)</b> 상태가 됩니다. 이때는 번호가 없습니다."),
		tool.Baro_t("새 문서 작성 또는 출처 추가 같은 <b>유효 기여</b>가 한 건이라도 확인되면 번호가 발급됩니다."),
		tool.Baro_t("검토를 통과한(confirmed) 기여도 유효 기여로 인정됩니다."),
		tool.Baro_t("번호는 이미 확정된 번호 중 가장 큰 값 + 1 로 정해집니다."),
		tool.Baro_t("정원이 가득 차면 번호를 발급하지 않고 대기 상태로 둡니다."),
	})
	html += tool.Baro_note(tool.Baro_t("계정만 여러 개 만들어 번호를 선점하거나 사고파는 일을 막기 위한 규칙입니다."))

	return html
}

// baro_founding_power_html : 분야 개척자의 권한 범위 안내 (1-4 필수 문구)
func baro_founding_power_html() string {
	return tool.Baro_alert("warn",
		tool.Baro_t(`<b>분야 개척자는 그 분야를 소유하거나 독점하지 않습니다.</b>`)+
			tool.Baro_t(`<p>분야 개척자에게 주어지는 것은 <b>명예와 프로젝트 안에서의 역할</b>뿐입니다. `)+
			tool.Baro_t(`문서를 잠그거나 다른 사람의 기여를 막을 수 없고, 그 분야의 편집 방향을 혼자 결정할 수도 없습니다. `)+
			tool.Baro_t(`제재·격리·검토 같은 <b>운영 권력은 분야 개척과 완전히 분리</b>되어 별도의 운영 역할이 담당합니다.</p>`),
	)
}

// baro_founding_invite_policy_html : 추천 정책 안내 (1-5 필수 문구)
func baro_founding_invite_policy_html() string {
	return tool.Baro_alert("info",
		tool.Baro_t(`<b>바로위키의 추천은 경쟁이 아닙니다.</b>`)+
			tool.Baro_t(`<p>초대에는 <b>현금성 포인트가 없고</b>, 초대 수를 끝없이 늘려 순위를 겨루는 <b>무한 추천 경쟁도 만들지 않습니다</b>. `)+
			tool.Baro_t(`한 사람이 발급할 수 있는 초대 링크 수에는 한도가 있고, 초대는 <b>초대받은 사람이 실제로 첫 유효 기여를 마쳤을 때만</b> 완료로 기록됩니다. `)+
			tool.Baro_t(`계정만 늘리는 행위는 인정되지 않습니다.</p>`),
	)
}

// baro_founding_register_ip : 계정 가입 시 IP (없으면 "")
func baro_founding_register_ip(db *sql.DB, user string) string {
	data := ""
	tool.QueryRow_DB(db, "select data from user_set where name = 'register_ip' and id = ?", []any{&data}, user)

	return data
}

// baro_founding_same_ip_count : 같은 가입 IP 를 쓴 계정 수
func baro_founding_same_ip_count(db *sql.DB, ip string) int {
	if strings.TrimSpace(ip) == "" {
		return 0
	}

	num := 0
	tool.QueryRow_DB(db, "select count(*) from user_set where name = 'register_ip' and data = ?", []any{&num}, ip)

	return num
}

// baro_founding_login_share : 접속 IP 를 공유하는 다른 계정 수 (baro_login_log 기준)
func baro_founding_login_share(db *sql.DB, user string) int {
	num := 0
	tool.QueryRow_DB(
		db,
		"select count(distinct user) from baro_login_log where user != ? and ip != '' and ip in (select ip from baro_login_log where user = ?)",
		[]any{&num}, user, user,
	)

	return num
}

// baro_founding_apply_burst : 지원 시각 앞뒤 30분 안의 지원 건수
func baro_founding_apply_burst(db *sql.DB, apply_date string) int {
	if strings.TrimSpace(apply_date) == "" {
		return 0
	}

	num := 0
	tool.QueryRow_DB(
		db,
		"select count(*) from baro_founder where apply_date >= ? and apply_date <= ?",
		[]any{&num},
		tool.Baro_time_add(apply_date, 0, 0, -30), tool.Baro_time_add(apply_date, 0, 0, 30),
	)

	return num
}

// baro_founding_suspect : 봇·중복 계정 의심 사유 추정 (운영 화면 전용)
//
//	확정이 아니라 "사람이 확인해 볼 만한 신호" 만 모은다.
func baro_founding_suspect(db *sql.DB, person baro_founding_person) []string {
	out := []string{}

	ip := baro_founding_register_ip(db, person.User)
	if strings.TrimSpace(ip) == "" {
		out = append(out, tool.Baro_t("가입 IP 기록 없음"))
	} else if num := baro_founding_same_ip_count(db, ip); num >= 2 {
		out = append(out, tool.Baro_t("같은 가입 IP 계정 ")+tool.Baro_itoa(num)+tool.Baro_t("개"))
	}

	if num := baro_founding_login_share(db, person.User); num >= 2 {
		out = append(out, tool.Baro_t("접속 IP 공유 계정 ")+tool.Baro_itoa(num)+tool.Baro_t("개"))
	}

	if num := baro_founding_apply_burst(db, person.ApplyDate); num >= 5 {
		out = append(out, tool.Baro_t("30분 안 지원 ")+tool.Baro_itoa(num)+tool.Baro_t("건 몰림"))
	}

	if baro_founding_first_valid(db, person.User) == nil {
		out = append(out, "유효 기여 없음")
	}

	return out
}

// baro_founding_status_name : 창립 지원 상태 이름
//
//	pending(보류) 은 운영자가 번호를 취소해 자동 재발급을 멈춘 상태다.
func baro_founding_status_name(status string) string {
	switch status {
	case "confirmed":
		return tool.Baro_t("번호 확정")
	case "rejected":
		return tool.Baro_t("반려")
	case "pending":
		return tool.Baro_t("보류")
	}

	return tool.Baro_t("번호 대기")
}

// baro_founding_state_html : 지원 상태 배지 (여러 화면이 함께 쓴다)
func baro_founding_state_html(person baro_founding_person) string {
	switch person.Status {
	case "confirmed":
		return tool.Baro_badge("창립 기여자 #"+tool.HTML_escape(person.Number), "ok")
	case "rejected":
		return tool.Baro_badge(tool.Baro_t("반려"), "danger")
	case "pending":
		return tool.Baro_badge(tool.Baro_t("보류"), "muted")
	}

	return tool.Baro_badge(tool.Baro_t("번호 대기"), "warn")
}

// baro_founding_field_recorded : 분야에 기록된 문서 수 (개척률의 분자)
//
//	개척률을 지어내지 않기 위해 project 모듈과 같은 방식으로 센다.
//	(프로젝트 목표 달성 수와 분야 표시가 붙은 문서 수 중 큰 값)
func baro_founding_field_recorded(db *sql.DB, field string) int {
	done := baro_founding_count_sql(
		db,
		"select count(*) from baro_project_target where status = 'done' and project in (select id from baro_project where field = ?)",
		field,
	)
	docs := baro_founding_count_sql(
		db,
		"select count(*) from baro_doc_meta where name = 'field' and data = ?",
		field,
	)

	if docs > done {
		return docs
	}

	return done
}

// baro_founding_my_field_contrib : 내가 그 분야에 남긴 기여 수
//
//	기여 이벤트의 field 값 또는 그 문서에 붙은 분야 표시로 센다.
func baro_founding_my_field_contrib(db *sql.DB, user string, field string) int {
	return baro_founding_count_sql(
		db,
		"select count(*) from baro_contrib_event e where e.user = ? and (e.field = ? or exists "+
			"(select 1 from baro_doc_meta m where m.title = e.title and m.name = 'field' and m.data = ?))",
		user, field, field,
	)
}

// baro_founding_my_doc_event : 내가 그 문서에 남긴 기록이 있는가
//
//	공유 카드에 "내가 기록했다" 고 적으려면 실제 기록이 있어야 한다.
//	기여 이벤트 또는 문서 편집 기록 중 하나만 있으면 인정한다.
func baro_founding_my_doc_event(db *sql.DB, user string, title string) bool {
	if strings.TrimSpace(title) == "" || !tool.Baro_is_user(user) {
		return false
	}

	found := ""
	if tool.QueryRow_DB(
		db,
		"select id from baro_contrib_event where user = ? and title = ?",
		[]any{&found}, user, title,
	) {
		return true
	}

	return tool.QueryRow_DB(
		db,
		"select id from history where ip = ? and title = ?",
		[]any{&found}, user, title,
	)
}
