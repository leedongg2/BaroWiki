package tool

import (
	"database/sql"
	"strconv"
	"strings"
	"sync"
)

// ============================================================
//  밐 (Mik) — 바로위키 가상 화폐
//
//  얻는 길은 셋뿐이다.
//    1. 접속 보상 : 하루에 한 번, 로그인한 채로 위키를 열면 들어온다.
//    2. 도전 과제 : 배지를 새로 받으면 들어온다.
//    3. 관리자 지급 : 사유를 반드시 적어야 한다.
//  쓰는 길은 바로샵과 양도뿐이다.
//
//  잔액은 baro_user_set 의 name = 'mik' 한 칸에 두고,
//  오간 기록은 baro_mik_log 에 한 줄씩 남긴다.
//  (5번 원칙 — 값이 왜 그렇게 되었는지 언제나 되짚을 수 있어야 한다)
// ============================================================

// Baro_mik_unit : 화폐 단위
const Baro_mik_unit = "밐"

// Baro_mik_max : 한 계정이 가질 수 있는 최대치 (자릿수가 터지지 않게 둔 상한)
const Baro_mik_max = 1000000000

// baro_mik_lock : 잔액을 고치는 동안 다른 요청이 끼어들지 못하게 한다.
//
//	SQLite 한 파일을 여러 요청이 함께 쓰므로 읽고-더하고-쓰는 사이를 잠근다.
var baro_mik_lock sync.Mutex

// Baro_mik_kind_name : 기록 종류 이름
func Baro_mik_kind_name(kind string) string {
	switch kind {
	case "attend":
		return Baro_t("접속 보상")
	case "badge":
		return Baro_t("도전 과제 달성")
	case "admin":
		return Baro_t("관리자 지급")
	case "admin_take":
		return Baro_t("관리자 회수")
	case "buy":
		return Baro_t("바로샵 구매")
	case "refund":
		return Baro_t("구매 취소 환불")
	case "send_out":
		return Baro_t("양도 보냄")
	case "send_in":
		return Baro_t("양도 받음")
	}

	return kind
}

// Baro_mik_get : 지금 가진 밐
func Baro_mik_get(db *sql.DB, user string) int {
	if !Baro_is_user(user) {
		return 0
	}

	data := ""
	QueryRow_DB(db, "select data from baro_user_set where user = ? and name = 'mik'", []any{&data}, user)

	return Str_to_int(data)
}

// baro_mik_put : 잔액 자리에 값 하나를 둔다 (잠금 안에서만 부른다)
func baro_mik_put(db *sql.DB, user string, value int) {
	Exec_DB(db, "delete from baro_user_set where user = ? and name = 'mik'", user)
	Exec_DB(db, "insert into baro_user_set (user, name, data) values (?, 'mik', ?)", user, strconv.Itoa(value))
}

// Baro_mik_add : 밐을 더하거나 뺀다.
//
//	delta 가 음수면 잔액보다 많이 뺄 수 없다. 못 빼면 아무 일도 하지 않고 false 를 돌려준다.
//	kind 는 Baro_mik_kind_name 이 아는 열쇠말이며, 언어와 무관하게 그대로 저장한다.
func Baro_mik_add(db *sql.DB, user string, delta int, kind string, reason string, actor string, target string) bool {
	if !Baro_is_user(user) || delta == 0 {
		return false
	}

	baro_mik_lock.Lock()
	defer baro_mik_lock.Unlock()

	now := Baro_mik_get(db, user)
	next := now + delta

	if next < 0 {
		return false
	}
	if next > Baro_mik_max {
		next = Baro_mik_max
		delta = next - now

		if delta == 0 {
			return false
		}
	}

	baro_mik_put(db, user, next)
	baro_mik_mark(db, user, next, delta)

	Exec_DB(
		db,
		"insert into baro_mik_log (id, user, delta, balance, kind, reason, actor, target, date) "+
			"values (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		Baro_next_id(db, "baro_mik_log"), user, strconv.Itoa(delta), strconv.Itoa(next),
		kind, Baro_trunc(reason, 300), actor, target, Baro_now(),
	)

	return true
}

// Baro_mik_text : "1,234밐"
func Baro_mik_text(value int) string {
	return Baro_comma(value) + Baro_mik_unit
}

// ------------------------------------------------------------
//  얻는 길 1 — 접속 보상
// ------------------------------------------------------------

// Baro_mik_attend_amount : 하루 접속 보상액
func Baro_mik_attend_amount(db *sql.DB) int {
	value := Baro_get_int(db, "mik_attend", 5)
	if value < 0 {
		return 0
	}

	return value
}

// Baro_mik_attend_date : 마지막으로 접속 보상을 받은 날
func Baro_mik_attend_date(db *sql.DB, user string) string {
	data := ""
	QueryRow_DB(db, "select data from baro_user_set where user = ? and name = 'mik_attend'", []any{&data}, user)

	return data
}

// Baro_mik_attend_streak : 며칠 잇달아 들렀는가
func Baro_mik_attend_streak(db *sql.DB, user string) int {
	data := ""
	QueryRow_DB(db, "select data from baro_user_set where user = ? and name = 'mik_attend_streak'", []any{&data}, user)

	return Str_to_int(data)
}

// Baro_mik_attend : 오늘 몫의 접속 보상을 준다.
//
//	이미 오늘 받았으면 0 을 돌려준다. 페이지를 열 때마다 불리므로 값싸게 끝나야 한다.
func Baro_mik_attend(db *sql.DB, user string) int {
	if !Baro_is_user(user) {
		return 0
	}

	today := Baro_today()
	if Baro_mik_attend_date(db, user) == today {
		return 0
	}

	amount := Baro_mik_attend_amount(db)
	if amount <= 0 {
		return 0
	}

	// 어제도 들렀으면 이어 세고, 아니면 다시 1일째부터 센다.
	streak := 1
	if last := Baro_mik_attend_date(db, user); last != "" && Baro_days_between(last, today) == 1 {
		streak = Baro_mik_attend_streak(db, user) + 1
	}

	Exec_DB(db, "delete from baro_user_set where user = ? and name = 'mik_attend'", user)
	Exec_DB(db, "insert into baro_user_set (user, name, data) values (?, 'mik_attend', ?)", user, today)
	Exec_DB(db, "delete from baro_user_set where user = ? and name = 'mik_attend_streak'", user)
	Exec_DB(db, "insert into baro_user_set (user, name, data) values (?, 'mik_attend_streak', ?)", user, strconv.Itoa(streak))

	if !Baro_mik_add(db, user, amount, "attend", "접속 보상 ("+today+")", "system", "") {
		return 0
	}

	Baro_metric_bump(db, "baro_mik_attend", today, 1)

	return amount
}

// ------------------------------------------------------------
//  얻는 길 2 — 도전 과제 (배지)
// ------------------------------------------------------------

// Baro_mik_badge_default : 도전 과제 보상의 기본값 (따로 정하지 않은 배지에 쓴다)
func Baro_mik_badge_default(db *sql.DB) int {
	value := Baro_get_int(db, "mik_badge", 20)
	if value < 0 {
		return 0
	}

	return value
}

// baro_mik_badge_key : 배지 하나의 보상액을 담는 설정 이름
func baro_mik_badge_key(badge_key string) string {
	badge_key = strings.TrimSpace(badge_key)
	if badge_key == "" {
		return ""
	}

	return "mik_badge_" + badge_key
}

// Baro_mik_badge_amount : 이 도전 과제를 깰 때 주는 밐
//
//	배지마다 따로 정할 수 있고, 정하지 않았으면 기본값을 쓴다.
func Baro_mik_badge_amount(db *sql.DB, badge_key string) int {
	name := baro_mik_badge_key(badge_key)
	if name == "" {
		return Baro_mik_badge_default(db)
	}

	// 빈 값은 「따로 정하지 않음」 이다. 0 을 적으면 그 배지만 보상이 없다.
	saved := strings.TrimSpace(Baro_get(db, name))
	if saved == "" {
		return Baro_mik_badge_default(db)
	}

	value := Str_to_int(saved)
	if value < 0 {
		return 0
	}

	return value
}

// Baro_mik_badge_own : 이 배지에 따로 정해 둔 값이 있는가 (없으면 -1)
func Baro_mik_badge_own(db *sql.DB, badge_key string) int {
	name := baro_mik_badge_key(badge_key)
	if name == "" {
		return -1
	}

	saved := strings.TrimSpace(Baro_get(db, name))
	if saved == "" {
		return -1
	}

	value := Str_to_int(saved)
	if value < 0 {
		return 0
	}

	return value
}

// Baro_mik_badge_set : 배지 하나의 보상액을 정한다 ("" 를 주면 기본값을 따른다)
func Baro_mik_badge_set(db *sql.DB, badge_key string, value string) {
	name := baro_mik_badge_key(badge_key)
	if name == "" {
		return
	}

	Baro_put(db, name, strings.TrimSpace(value))
}

// Baro_mik_badge_reward : 도전 과제를 깬 사람에게 밐을 준다.
//
//	배지를 실제로 새로 지급한 자리에서만 부른다. (같은 배지로 두 번 받을 수 없다)
func Baro_mik_badge_reward(db *sql.DB, user string, badge_key string, badge_name string) int {
	amount := Baro_mik_badge_amount(db, badge_key)
	if amount <= 0 || !Baro_is_user(user) {
		return 0
	}

	if !Baro_mik_add(db, user, amount, "badge", "도전 과제 달성 : "+badge_name, "system", badge_key) {
		return 0
	}

	return amount
}

// ------------------------------------------------------------
//  산 것 — 칭호 · 캡차 면제
// ------------------------------------------------------------

// Baro_mik_title_row : 칭호 하나 (이름과 글자색)
//
//	색은 산 시점의 상품 내용을 그대로 담는다. 나중에 상품이 바뀌어도
//	이미 산 사람의 칭호 색은 바뀌지 않는다.
type Baro_mik_title_row struct {
	Name  string
	Color string // 밝은 쪽 글자색 ("" 면 기본 색)
	Dark  string // 어두운 쪽 글자색 ("" 면 밝은 쪽을 그대로 쓴다)
}

// Baro_mik_title_rows : 이 사람이 가진 칭호 (색까지)
func Baro_mik_title_rows(db *sql.DB, user string) []Baro_mik_title_row {
	out := []Baro_mik_title_row{}
	if !Baro_is_user(user) {
		return out
	}

	rows := Query_DB(
		db,
		"select data from baro_shop_buy where user = ? and kind = 'title' and status = 'active' order by id + 0",
		user,
	)
	defer rows.Close()

	seen := map[string]bool{}
	for rows.Next() {
		data := ""
		if rows.Scan(&data) != nil {
			continue
		}

		saved := Baro_json_dec_str(data)

		name := strings.TrimSpace(saved["title"])
		if name == "" || seen[name] {
			continue
		}

		seen[name] = true
		out = append(out, Baro_mik_title_row{
			Name:  name,
			Color: Baro_theme_color_norm(saved["color"]),
			Dark:  Baro_theme_color_norm(saved["color_dark"]),
		})
	}

	return out
}

// Baro_mik_title_list : 이 사람이 가진 칭호 이름
func Baro_mik_title_list(db *sql.DB, user string) []string {
	out := []string{}

	for _, row := range Baro_mik_title_rows(db, user) {
		out = append(out, row.Name)
	}

	return out
}

// Baro_mik_title_get : 달고 있는 칭호를 색까지 (이름이 "" 면 없음)
func Baro_mik_title_get(db *sql.DB, user string) Baro_mik_title_row {
	now := Baro_mik_title(db, user)
	if now == "" {
		return Baro_mik_title_row{}
	}

	for _, row := range Baro_mik_title_rows(db, user) {
		if row.Name == now {
			return row
		}
	}

	return Baro_mik_title_row{}
}

// Baro_mik_title_html : 칭호 하나를 색까지 입혀 그린다 ("" 면 그릴 것이 없다)
//
//	어두운 쪽 색은 CSS 변수로 넘겨 둔다. 인라인 스타일로는 다크 모드를 가릴 수 없기 때문이다.
//	실제로 갈아 끼우는 규칙은 Baro_mik_title_css 에 있다.
func Baro_mik_title_html(row Baro_mik_title_row) string {
	if strings.TrimSpace(row.Name) == "" {
		return ""
	}

	if row.Color == "" && row.Dark == "" {
		return `<span class="bw_mik_title">` + HTML_escape(row.Name) + `</span>`
	}

	light := row.Color
	if light == "" {
		light = row.Dark
	}

	dark := row.Dark
	if dark == "" {
		dark = light
	}

	return `<span class="bw_mik_title bw_mik_title_c" style="--bw-title-l:` + HTML_escape(light) +
		`;--bw-title-d:` + HTML_escape(dark) + `">` + HTML_escape(row.Name) + `</span>`
}

// Baro_mik_title_user_html : 이 사람이 달고 있는 칭호를 그린다
func Baro_mik_title_user_html(db *sql.DB, user string) string {
	return Baro_mik_title_html(Baro_mik_title_get(db, user))
}

// Baro_mik_title_css : 칭호 색 규칙
//
//	이 위키의 다크 모드는 쿠키로 정해 dark.css 를 덧씌우는 방식이라
//	CSS 만으로는 지금이 어느 쪽인지 알 수 없다. 그래서 화면을 그리는 자리에서
//	어느 쪽인지 받아 쓸 변수를 정해 준다.
func Baro_mik_title_css(dark bool) string {
	pick := "--bw-title-l"
	if dark {
		pick = "--bw-title-d"
	}

	return `<style id="bw_mik_title_css">.bw_mik_title{font-weight:600;}` +
		`.bw_mik_title_c{color:var(` + pick + `);}</style>`
}

// Baro_mik_title : 지금 달고 있는 칭호 ("" 면 없음)
//
//	가지고 있지 않은 칭호가 설정에 남아 있으면 보여 주지 않는다.
func Baro_mik_title(db *sql.DB, user string) string {
	if !Baro_is_user(user) {
		return ""
	}

	now := ""
	QueryRow_DB(db, "select data from baro_user_set where user = ? and name = 'mik_title'", []any{&now}, user)
	if now == "" {
		return ""
	}

	for _, one := range Baro_mik_title_list(db, user) {
		if one == now {
			return now
		}
	}

	return ""
}

// Baro_mik_title_set : 달 칭호를 고른다 ("" 면 뗀다)
func Baro_mik_title_set(db *sql.DB, user string, title string) {
	Exec_DB(db, "delete from baro_user_set where user = ? and name = 'mik_title'", user)

	if title != "" {
		Exec_DB(db, "insert into baro_user_set (user, name, data) values (?, 'mik_title', ?)", user, title)
	}
}

// Baro_mik_captcha_until : 캡차 면제가 끝나는 시각 ("" 면 면제 없음)
func Baro_mik_captcha_until(db *sql.DB, user string) string {
	if !Baro_is_user(user) {
		return ""
	}

	data := ""
	QueryRow_DB(db, "select data from baro_user_set where user = ? and name = 'mik_captcha_until'", []any{&data}, user)
	if data == "" || Baro_is_past(data) {
		return ""
	}

	return data
}

// Baro_mik_captcha_free : 지금 캡차를 건너뛸 수 있는가
func Baro_mik_captcha_free(db *sql.DB, user string) bool {
	return Baro_mik_captcha_until(db, user) != ""
}

// Baro_mik_captcha_extend : 캡차 면제를 days 만큼 늘린다.
//
//	이미 면제 중이면 남은 기간 뒤에 이어 붙인다. (겹쳐 사면 손해 보지 않게)
func Baro_mik_captcha_extend(db *sql.DB, user string, days int) string {
	if days <= 0 || !Baro_is_user(user) {
		return ""
	}

	from := Baro_mik_captcha_until(db, user)
	if from == "" {
		from = Baro_now()
	}

	until := Baro_time_add(from, days, 0, 0)

	Exec_DB(db, "delete from baro_user_set where user = ? and name = 'mik_captcha_until'", user)
	Exec_DB(db, "insert into baro_user_set (user, name, data) values (?, 'mik_captcha_until', ?)", user, until)

	return until
}

// Baro_mik_tick : 페이지를 열 때마다 불리는 접속 보상 처리
//
//	위키 화면 전체가 지나는 길목이므로 여기서 난 잘못이 화면을 죽이면 안 된다.
//	(표가 아직 없는 오래된 설치 · 읽기 전용 파일 따위)
func Baro_mik_tick(db *sql.DB, user string) {
	defer func() {
		_ = recover()
	}()

	if db == nil || !Baro_is_user(user) {
		return
	}

	Baro_mik_attend(db, user)
}

// ------------------------------------------------------------
//  하루 양도 한도
// ------------------------------------------------------------

// Baro_mik_send_limit : 한 사람이 하루에 보낼 수 있는 밐 (0 이하면 아예 못 보낸다)
func Baro_mik_send_limit(db *sql.DB) int {
	value := Baro_get_int(db, "mik_send_daily", 100)
	if value < 0 {
		return 0
	}

	return value
}

// Baro_mik_send_today : 오늘 이미 보낸 밐
func Baro_mik_send_today(db *sql.DB, user string) int {
	if !Baro_is_user(user) {
		return 0
	}

	sent := 0
	rows := Query_DB(
		db,
		"select delta from baro_mik_log where user = ? and kind = 'send_out' and date like ?",
		user, Baro_today()+"%",
	)
	defer rows.Close()

	for rows.Next() {
		delta := ""
		if rows.Scan(&delta) != nil {
			continue
		}

		// 보낸 기록은 음수로 남는다.
		if amount := Str_to_int(delta); amount < 0 {
			sent += -amount
		}
	}

	return sent
}

// Baro_mik_send_left : 오늘 더 보낼 수 있는 밐
func Baro_mik_send_left(db *sql.DB, user string) int {
	left := Baro_mik_send_limit(db) - Baro_mik_send_today(db, user)
	if left < 0 {
		return 0
	}

	return left
}

// ------------------------------------------------------------
//  순위에 쓰는 누계
//
//  순위를 매길 때마다 오간 기록을 처음부터 더하면 사람이 늘수록 느려진다.
//  그래서 밐이 움직일 때 두 가지를 그 자리에서 함께 적어 둔다.
//    mik_max   : 지금까지 가졌던 가장 많은 밐
//    mik_total : 지금까지 들어온 밐을 모두 더한 값 (나간 것은 빼지 않는다)
// ------------------------------------------------------------

// baro_mik_mark : 누계를 갱신한다 (잠금 안에서만 부른다)
func baro_mik_mark(db *sql.DB, user string, balance int, delta int) {
	if balance > baro_mik_num(db, user, "mik_max") {
		baro_mik_num_put(db, user, "mik_max", balance)
	}

	if delta > 0 {
		baro_mik_num_put(db, user, "mik_total", baro_mik_num(db, user, "mik_total")+delta)
	}
}

// baro_mik_num : baro_user_set 에 적어 둔 수 하나
func baro_mik_num(db *sql.DB, user string, name string) int {
	data := ""
	QueryRow_DB(db, "select data from baro_user_set where user = ? and name = ?", []any{&data}, user, name)

	return Str_to_int(data)
}

// baro_mik_num_put : 수 하나를 써 넣는다
func baro_mik_num_put(db *sql.DB, user string, name string, value int) {
	Exec_DB(db, "delete from baro_user_set where user = ? and name = ?", user, name)
	Exec_DB(db, "insert into baro_user_set (user, name, data) values (?, ?, ?)", user, name, strconv.Itoa(value))
}

// Baro_mik_best : 지금까지 가졌던 가장 많은 밐
func Baro_mik_best(db *sql.DB, user string) int {
	if !Baro_is_user(user) {
		return 0
	}

	max_value := baro_mik_num(db, user, "mik_max")
	if now := Baro_mik_get(db, user); now > max_value {
		return now
	}

	return max_value
}

// Baro_mik_total : 지금까지 들어온 밐을 모두 더한 값
func Baro_mik_total(db *sql.DB, user string) int {
	if !Baro_is_user(user) {
		return 0
	}

	return baro_mik_num(db, user, "mik_total")
}

// Baro_mik_rank_kind_list : 순위 갈래 {열쇠, 이름, 설명}
func Baro_mik_rank_kind_list() [][]string {
	return [][]string{
		{"now", Baro_t("지금 가진 밐"), Baro_t("이 순간의 잔액 순서입니다.")},
		{"max", Baro_t("최대 보유 밐"), Baro_t("지금까지 가졌던 가장 많은 밐의 순서입니다. 쓴다고 줄지 않습니다.")},
		{"total", Baro_t("누적 획득 밐"), Baro_t("지금까지 들어온 밐을 모두 더한 순서입니다. 많이 모은 사람이 앞섭니다.")},
	}
}

// Baro_mik_rank_name : 순위 갈래에 쓰는 baro_user_set 이름
func Baro_mik_rank_name(kind string) string {
	switch kind {
	case "max":
		return "mik_max"
	case "total":
		return "mik_total"
	}

	return "mik"
}

// Baro_mik_rank_row : 순위 한 줄
type Baro_mik_rank_row struct {
	User  string
	Value int
}

// Baro_mik_rank : 갈래별 순위 목록
//
//	「최대 보유」 는 누계를 적기 전에 쌓인 밐이 있을 수 있으므로 지금 잔액과 견주어 큰 쪽을 쓴다.
func Baro_mik_rank(db *sql.DB, kind string, offset int, size int) []Baro_mik_rank_row {
	name := Baro_mik_rank_name(kind)

	if offset < 0 {
		offset = 0
	}
	if size <= 0 {
		size = Baro_page_size
	}

	rows := Query_DB(
		db,
		"select user, data from baro_user_set where name = ? order by data + 0 desc, user limit ?, ?",
		name, offset, size,
	)
	defer rows.Close()

	out := []Baro_mik_rank_row{}
	for rows.Next() {
		row := Baro_mik_rank_row{}
		data := ""

		if rows.Scan(&row.User, &data) != nil {
			continue
		}

		row.Value = Str_to_int(data)
		if kind == "max" {
			if now := Baro_mik_get(db, row.User); now > row.Value {
				row.Value = now
			}
		}

		out = append(out, row)
	}

	return out
}

// Baro_mik_rank_count : 갈래별 순위에 오른 사람 수
func Baro_mik_rank_count(db *sql.DB, kind string) int {
	count := 0
	QueryRow_DB(
		db,
		"select count(*) from baro_user_set where name = ? and data + 0 > 0",
		[]any{&count}, Baro_mik_rank_name(kind),
	)

	return count
}

// Baro_mik_backfill : 순위 누계를 한 번만 채워 넣는다
//
//	mik_max · mik_total 은 밐이 움직일 때 함께 적는다. 그런데 이 두 칸이 생기기 전에
//	쌓인 밐은 적힌 적이 없어 순위에서 통째로 빠진다.
//	baro_mik_log 에는 움직임마다 「그때의 잔액」 이 남아 있으므로 거기서 정확히 되살린다.
func Baro_mik_backfill(db *sql.DB) {
	defer func() {
		_ = recover()
	}()

	done := ""
	if QueryRow_DB(db, "select name from baro_set where name = 'mik_rank_filled'", []any{&done}) {
		return
	}

	best := map[string]int{}
	total := map[string]int{}

	rows := Query_DB(db, "select user, delta, balance from baro_mik_log")
	for rows.Next() {
		user, delta, balance := "", "", ""
		if rows.Scan(&user, &delta, &balance) != nil {
			continue
		}

		if value := Str_to_int(balance); value > best[user] {
			best[user] = value
		}

		if amount := Str_to_int(delta); amount > 0 {
			total[user] += amount
		}
	}
	rows.Close()

	// 기록이 아예 없던 시절에 들어온 밐도 빠뜨리지 않는다.
	now_rows := Query_DB(db, "select user, data from baro_user_set where name = 'mik'")
	for now_rows.Next() {
		user, data := "", ""
		if now_rows.Scan(&user, &data) != nil {
			continue
		}

		value := Str_to_int(data)
		if value > best[user] {
			best[user] = value
		}
		if value > total[user] {
			total[user] = value
		}
	}
	now_rows.Close()

	for user, value := range best {
		if value > baro_mik_num(db, user, "mik_max") {
			baro_mik_num_put(db, user, "mik_max", value)
		}
	}

	for user, value := range total {
		if value > baro_mik_num(db, user, "mik_total") {
			baro_mik_num_put(db, user, "mik_total", value)
		}
	}

	Baro_put(db, "mik_rank_filled", "on")
}
