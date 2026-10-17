package tool

import (
	"database/sql"
	"strings"
)

// ============================================================
//  편집방 — 방과 사람 다루기
// ============================================================

// Baro_coop_room_row : 방 하나
type Baro_coop_room_row struct {
	ID          string
	Link        string
	Code        string
	Name        string
	Description string
	Owner       string
	OpenKind    string
	MaxUser     int
	Status      string
	CloseAt     string
	Notice      string
	Banned      string
	Date        string
	CloseDate   string
	Acl         string // 행위별로 어느 자리까지 열어 줄지 (JSON)
	SealBy      string // 영구 폐쇄한 관리자
	SealReason  string // 영구 폐쇄 사유
	SealDate    string
}

// Baro_coop_member_row : 방에 든 사람 하나
type Baro_coop_member_row struct {
	ID      string
	Room    string
	User    string
	Role    string
	Status  string // in / out
	Block   bool   // 이 방에서 편집 제한
	Date    string
	OutDate string
}

// Baro_coop_member_editing : 편집 자리에 앉은 사람인가
//
//	편집이 막힌 사람은 자리가 무엇이든 관전으로 본다.
//	그래서 정원에도 들지 않고, 편집 유저로 바꿀 수도 없다.
func (row Baro_coop_member_row) Baro_coop_member_editing() bool {
	return row.Role != Baro_coop_watch && !row.Block
}

// baro_coop_room_select : 공통 조회 칸
const baro_coop_room_select = "select id, link, code, name, description, owner, open_kind, max_user, " +
	"status, close_at, notice, banned, date, close_date, acl, seal_by, seal_reason, seal_date from baro_coop_room"

// baro_coop_room_scan : 한 줄 읽기
func baro_coop_room_scan(rows *sql.Rows) (Baro_coop_room_row, bool) {
	row := Baro_coop_room_row{}
	max_user := ""

	if rows.Scan(
		&row.ID, &row.Link, &row.Code, &row.Name, &row.Description, &row.Owner,
		&row.OpenKind, &max_user, &row.Status, &row.CloseAt, &row.Notice, &row.Banned,
		&row.Date, &row.CloseDate, &row.Acl, &row.SealBy, &row.SealReason, &row.SealDate,
	) != nil {
		return row, false
	}

	row.MaxUser = Str_to_int(max_user)

	return row, true
}

// Baro_coop_room_get : 이름표로 방 하나
func Baro_coop_room_get(db *sql.DB, link string) (Baro_coop_room_row, bool) {
	link = strings.TrimSpace(link)
	if link == "" {
		return Baro_coop_room_row{}, false
	}

	row, ok := baro_coop_room_one(db, baro_coop_room_select+" where link = ?", link)
	if !ok {
		return row, false
	}

	return baro_coop_room_tick(db, row), true
}

// baro_coop_room_one : 한 줄만 읽고 조회를 바로 닫는다
//
//	읽은 뒤에 닫을 때가 지났는지 보고 그 자리에서 쓰기를 한다.
//	조회를 연 채로 쓰면 SQLite 가 잠기므로, 쓰기 전에 반드시 닫아야 한다.
func baro_coop_room_one(db *sql.DB, query string, args ...any) (Baro_coop_room_row, bool) {
	rows := Query_DB(db, query, args...)

	if !rows.Next() {
		rows.Close()

		return Baro_coop_room_row{}, false
	}

	row, ok := baro_coop_room_scan(rows)
	rows.Close()

	return row, ok
}

// Baro_coop_room_by_code : 12자리 열쇠로 방 하나
func Baro_coop_room_by_code(db *sql.DB, code string) (Baro_coop_room_row, bool) {
	code = Baro_coop_code_norm(code)
	if len(code) != 12 {
		return Baro_coop_room_row{}, false
	}

	row, ok := baro_coop_room_one(db, baro_coop_room_select+" where code = ?", code)
	if !ok {
		return row, false
	}

	return baro_coop_room_tick(db, row), true
}

// baro_coop_room_tick : 닫을 때가 지났으면 그 자리에서 닫는다
//
//	따로 도는 일꾼을 두지 않는다. 방을 읽을 때마다 시각을 견주어 스스로 닫는다.
func baro_coop_room_tick(db *sql.DB, row Baro_coop_room_row) Baro_coop_room_row {
	if row.Status != Baro_coop_closing || row.CloseAt == "" {
		return row
	}

	if !Baro_is_past(row.CloseAt) {
		return row
	}

	Exec_DB(
		db,
		"update baro_coop_room set status = ?, close_date = ? where id = ?",
		Baro_coop_closed, Baro_now(), row.ID,
	)
	Baro_coop_log_add(db, row.ID, "system", "", "방이 닫혔습니다.", "")

	row.Status = Baro_coop_closed
	row.CloseDate = Baro_now()

	return row
}

// Baro_coop_room_live : 지금 들어가 편집할 수 있는 방인가
func (row Baro_coop_room_row) Baro_coop_room_live() bool {
	return row.Status == Baro_coop_open || row.Status == Baro_coop_closing
}

// Baro_coop_room_sealed : 영구 폐쇄된 방인가
func (row Baro_coop_room_row) Baro_coop_room_sealed() bool {
	return row.Status == Baro_coop_sealed
}

// Baro_coop_room_list : 방 목록
//
//	kind : "public" 공개방 · "mine" 내가 든 방
func Baro_coop_room_list(db *sql.DB, kind string, user string, offset int, size int) []Baro_coop_room_row {
	if size <= 0 {
		size = Baro_page_size
	}
	if offset < 0 {
		offset = 0
	}

	query := baro_coop_room_select
	args := []any{}

	switch kind {
	case "mine":
		query += " where id in (select room from baro_coop_member where user = ? and status = 'in')"
		args = append(args, user)
	default:
		query += " where open_kind = 'public' and status <> 'closed' and status <> 'sealed'"
	}

	query += " order by id + 0 desc limit ?, ?"
	args = append(args, offset, size)

	rows := Query_DB(db, query, args...)

	out := []Baro_coop_room_row{}
	for rows.Next() {
		if row, ok := baro_coop_room_scan(rows); ok {
			out = append(out, row)
		}
	}
	rows.Close()

	// 다 읽고 조회를 닫은 뒤에 손본다. 조회를 연 채로 쓰면 SQLite 가 잠긴다.
	for i := range out {
		out[i] = baro_coop_room_tick(db, out[i])
	}

	return out
}

// Baro_coop_room_count : 방 수
func Baro_coop_room_count(db *sql.DB, kind string, user string) int {
	count := 0

	switch kind {
	case "mine":
		QueryRow_DB(
			db,
			"select count(*) from baro_coop_room where id in "+
				"(select room from baro_coop_member where user = ? and status = 'in')",
			[]any{&count}, user,
		)
	default:
		QueryRow_DB(
			db,
			"select count(*) from baro_coop_room where open_kind = 'public' "+
				"and status <> 'closed' and status <> 'sealed'",
			[]any{&count},
		)
	}

	return count
}

// Baro_coop_room_add : 방을 새로 연다. 만들어진 이름표를 돌려준다.
func Baro_coop_room_add(db *sql.DB, row Baro_coop_room_row) string {
	link := ""

	// 아주 드물게 이름표가 겹칠 수 있다. 몇 번 더 뽑아 본다.
	for i := 0; i < 8; i++ {
		candidate := Baro_coop_link_new()
		if candidate == "" {
			continue
		}

		found := ""
		if QueryRow_DB(db, "select id from baro_coop_room where link = ?", []any{&found}, candidate) {
			continue
		}

		link = candidate
		break
	}

	if link == "" {
		return ""
	}

	code := ""
	for i := 0; i < 8; i++ {
		candidate := Baro_coop_code_new()
		if candidate == "" {
			continue
		}

		found := ""
		if QueryRow_DB(db, "select id from baro_coop_room where code = ?", []any{&found}, candidate) {
			continue
		}

		code = candidate
		break
	}

	id := Baro_next_id(db, "baro_coop_room")

	Exec_DB(
		db,
		"insert into baro_coop_room (id, link, code, name, description, owner, open_kind, max_user, "+
			"status, close_at, notice, banned, date, close_date, note, acl, seal_by, seal_reason, seal_date) "+
			"values (?, ?, ?, ?, ?, ?, ?, ?, 'open', '', '', ?, ?, '', '', ?, '', '', '')",
		id, link, code, Baro_trunc(row.Name, 60), Baro_trunc(row.Description, 1000),
		row.Owner, row.OpenKind, Baro_itoa(row.MaxUser), row.Banned, Baro_now(), row.Acl,
	)

	// 방을 연 사람이 곧 방장이다.
	Baro_coop_member_set(db, id, row.Owner, Baro_coop_owner)
	Baro_coop_log_add(db, id, "system", row.Owner, "방을 열었습니다.", "")

	return link
}

// Baro_coop_room_save : 방 설정을 고친다
func Baro_coop_room_save(db *sql.DB, row Baro_coop_room_row) {
	Exec_DB(
		db,
		"update baro_coop_room set name = ?, description = ?, open_kind = ?, max_user = ?, "+
			"banned = ?, acl = ? where id = ?",
		Baro_trunc(row.Name, 60), Baro_trunc(row.Description, 1000),
		row.OpenKind, Baro_itoa(row.MaxUser), row.Banned, row.Acl, row.ID,
	)
}

// Baro_coop_room_notice_set : 공지 갈아 끼우기
func Baro_coop_room_notice_set(db *sql.DB, room string, notice string) {
	Exec_DB(db, "update baro_coop_room set notice = ? where id = ?", Baro_trunc(notice, 1000), room)
}

// Baro_coop_room_close : 폐쇄를 알린다. 실제로 닫히는 것은 10분 뒤다.
func Baro_coop_room_close(db *sql.DB, room string) string {
	close_at := Baro_time_add(Baro_now(), 0, 0, Baro_coop_close_minute)

	Exec_DB(
		db,
		"update baro_coop_room set status = ?, close_at = ? where id = ?",
		Baro_coop_closing, close_at, room,
	)

	return close_at
}

// Baro_coop_room_reopen : 다시 연다
func Baro_coop_room_reopen(db *sql.DB, room string) {
	Exec_DB(
		db,
		"update baro_coop_room set status = ?, close_at = '', close_date = '' where id = ?",
		Baro_coop_open, room,
	)
}

// ------------------------------------------------------------
//  사람
// ------------------------------------------------------------

// baro_coop_member_select : 공통 조회 칸
const baro_coop_member_select = "select id, room, user, role, status, block, date, out_date from baro_coop_member"

// baro_coop_member_scan : 한 줄 읽기
func baro_coop_member_scan(rows *sql.Rows) (Baro_coop_member_row, bool) {
	row := Baro_coop_member_row{}
	block := ""

	if rows.Scan(&row.ID, &row.Room, &row.User, &row.Role, &row.Status, &block, &row.Date, &row.OutDate) != nil {
		return row, false
	}

	row.Block = block == "on"

	return row, true
}

// Baro_coop_member_get : 이 사람이 이 방에서 어떤 자리인가
func Baro_coop_member_get(db *sql.DB, room string, user string) (Baro_coop_member_row, bool) {
	if user == "" {
		return Baro_coop_member_row{}, false
	}

	rows := Query_DB(db, baro_coop_member_select+" where room = ? and user = ?", room, user)
	defer rows.Close()

	if !rows.Next() {
		return Baro_coop_member_row{}, false
	}

	return baro_coop_member_scan(rows)
}

// Baro_coop_member_list : 방에 든 사람들
//
//	status 가 "" 면 모두, "in" 이면 지금 있는 사람만, "out" 이면 나간 사람만.
func Baro_coop_member_list(db *sql.DB, room string, status string) []Baro_coop_member_row {
	query := baro_coop_member_select + " where room = ?"
	args := []any{room}

	if status != "" {
		query += " and status = ?"
		args = append(args, status)
	}

	// 방장 · 부방장 · 편집 · 관전 차례로
	query += " order by case role when 'owner' then 0 when 'sub' then 1 when 'edit' then 2 else 3 end, id + 0"

	rows := Query_DB(db, query, args...)
	defer rows.Close()

	out := []Baro_coop_member_row{}
	for rows.Next() {
		if row, ok := baro_coop_member_scan(rows); ok {
			out = append(out, row)
		}
	}

	return out
}

// Baro_coop_member_set : 자리를 새로 주거나 바꾼다 (들어오는 것도 이것으로 한다)
func Baro_coop_member_set(db *sql.DB, room string, user string, role string) {
	if _, found := Baro_coop_member_get(db, room, user); found {
		Exec_DB(
			db,
			"update baro_coop_member set role = ?, status = 'in', out_date = '' where room = ? and user = ?",
			role, room, user,
		)

		return
	}

	Exec_DB(
		db,
		"insert into baro_coop_member (id, room, user, role, status, block, date, out_date, note) "+
			"values (?, ?, ?, ?, 'in', '', ?, '', '')",
		Baro_next_id(db, "baro_coop_member"), room, user, role, Baro_now(),
	)
}

// Baro_coop_member_out : 방에서 나간다 (기록은 남는다)
func Baro_coop_member_out(db *sql.DB, room string, user string) {
	Exec_DB(
		db,
		"update baro_coop_member set status = 'out', out_date = ? where room = ? and user = ?",
		Baro_now(), room, user,
	)
}

// Baro_coop_member_block : 이 방에서 편집을 막거나 푼다
func Baro_coop_member_block(db *sql.DB, room string, user string, on bool) {
	value := ""
	if on {
		value = "on"
	}

	Exec_DB(db, "update baro_coop_member set block = ? where room = ? and user = ?", value, room, user)
}

// Baro_coop_count : 지금 방에 있는 편집 인원과 관전 인원
//
//	방장은 정원에 들지 않는다.
//	방장이 잠깐 나온 사이에 자리가 다 차서 제 방에 못 들어가는 일을 막기 위해서다.
//	편집이 막힌 사람은 관전으로 센다.
func Baro_coop_count(db *sql.DB, room string) (int, int) {
	edit_count, watch_count := 0, 0

	for _, one := range Baro_coop_member_list(db, room, "in") {
		if one.Role == Baro_coop_owner {
			continue
		}

		if one.Baro_coop_member_editing() {
			edit_count++
			continue
		}

		watch_count++
	}

	return edit_count, watch_count
}

// Baro_coop_full : 편집 자리가 다 찼는가
func Baro_coop_full(db *sql.DB, row Baro_coop_room_row) bool {
	edit_count, _ := Baro_coop_count(db, row.ID)

	return edit_count >= row.MaxUser
}

// Baro_coop_full_for : 이 사람이 편집 자리로 들어가려 할 때 자리가 찼는가
//
//	방장은 정원 밖이므로 언제나 들어갈 수 있다.
func Baro_coop_full_for(db *sql.DB, row Baro_coop_room_row, user string) bool {
	if row.Owner == user {
		return false
	}

	return Baro_coop_full(db, row)
}

// Baro_coop_owner_away : 방장이 자리를 비웠는가
func Baro_coop_owner_away(db *sql.DB, row Baro_coop_room_row) bool {
	member, found := Baro_coop_member_get(db, row.ID, row.Owner)
	if !found {
		return true
	}

	return member.Status != "in"
}

// Baro_coop_can_manage : 방 설정을 만질 수 있는 사람인가 (방장 · 부방장 · 운영진)
func Baro_coop_can_manage(db *sql.DB, row Baro_coop_room_row, ip string) bool {
	if !Baro_is_user(ip) {
		return false
	}

	if row.Owner == ip || Baro_can(db, ip, "system") {
		return true
	}

	member, found := Baro_coop_member_get(db, row.ID, ip)

	return found && member.Status == "in" && member.Role == Baro_coop_sub
}

// Baro_coop_auth : 위키 권한 그룹이 준 편집방 권한이 있는가
//
//	권한 그룹 화면(/auth/list)에서 주고 뺄 수 있다.
//	로그인 사용자는 기본으로 다 가지고 있고, 뺀 그룹에 든 사람만 막힌다.
func Baro_coop_auth(db *sql.DB, ip string, name string) bool {
	auth := Get_user_auth_info(db, ip)

	if auth["owner"] || auth["treat_as_admin"] {
		return true
	}

	return auth[name]
}

// Baro_coop_can_view : 편집방을 들여다볼 수 있는 사람인가
func Baro_coop_can_view(db *sql.DB, ip string) bool {
	return Baro_coop_auth(db, ip, "coop_view")
}

// Baro_coop_can_join : 편집 자리로 들어갈 수 있는 사람인가
func Baro_coop_can_join(db *sql.DB, ip string) bool {
	return Baro_is_user(ip) && Baro_coop_auth(db, ip, "coop_join")
}

// Baro_coop_can_open : 편집방을 새로 열 수 있는 사람인가
//
//	baro_set 의 coop_open_role 로 어느 자리부터 열 수 있는지 정하고,
//	권한 그룹의 coop_open 을 뺀 사람은 그것과 상관없이 막는다.
func Baro_coop_can_open(db *sql.DB, ip string) bool {
	if !Baro_is_user(ip) {
		return false
	}

	if !Baro_coop_auth(db, ip, "coop_open") {
		return false
	}

	switch Baro_get_def(db, "coop_open_role", "user") {
	case "owner":
		return Baro_is_owner(db, ip)
	case "sub":
		return Baro_is_admin(db, ip) || Baro_can(db, ip, "system")
	}

	return true
}

// Baro_coop_open_role_list : 방을 열 수 있는 사람 {열쇠, 이름}
func Baro_coop_open_role_list() [][]string {
	return [][]string{
		{"user", Baro_t("로그인한 사람 누구나")},
		{"sub", Baro_t("부매니저(관리자)부터")},
		{"owner", Baro_t("사측 관리자만")},
	}
}

// ------------------------------------------------------------
//  손댄 문서 · 기록
// ------------------------------------------------------------

// Baro_coop_doc_row : 방에서 다루는 문서 하나
type Baro_coop_doc_row struct {
	Title    string
	Num      int    // 이 방에서 손댄 횟수
	Date     string // 마지막으로 손댄 때
	Status   string // open 이면 작업 공간에 탭이 열려 있다
	Opener   string // 탭을 연 사람
	OpenDate string
}

// Baro_coop_doc_open_now : 작업 공간에 탭이 열려 있는가
func (row Baro_coop_doc_row) Baro_coop_doc_open_now() bool {
	return row.Status == "open"
}

// baro_coop_doc_select : 공통 조회 칸
const baro_coop_doc_select = "select title, num, date, status, opener, open_date from baro_coop_doc"

// baro_coop_doc_read : 조회 결과를 줄줄이 읽는다
func baro_coop_doc_read(rows *sql.Rows) []Baro_coop_doc_row {
	out := []Baro_coop_doc_row{}

	for rows.Next() {
		row := Baro_coop_doc_row{}
		num := ""

		if rows.Scan(&row.Title, &num, &row.Date, &row.Status, &row.Opener, &row.OpenDate) != nil {
			continue
		}

		row.Num = Str_to_int(num)
		out = append(out, row)
	}

	return out
}

// Baro_coop_doc_list : 방에서 다룬 문서들 (탭을 닫은 것까지)
func Baro_coop_doc_list(db *sql.DB, room string) []Baro_coop_doc_row {
	rows := Query_DB(db, baro_coop_doc_select+" where room = ? order by date desc, id + 0 desc", room)
	defer rows.Close()

	return baro_coop_doc_read(rows)
}

// Baro_coop_doc_open_list : 작업 공간에 열려 있는 탭들 (연 차례대로)
func Baro_coop_doc_open_list(db *sql.DB, room string) []Baro_coop_doc_row {
	rows := Query_DB(
		db,
		baro_coop_doc_select+" where room = ? and status = 'open' order by id + 0",
		room,
	)
	defer rows.Close()

	return baro_coop_doc_read(rows)
}

// Baro_coop_doc_get : 방에서 다루는 문서 하나
func Baro_coop_doc_get(db *sql.DB, room string, title string) (Baro_coop_doc_row, bool) {
	rows := Query_DB(db, baro_coop_doc_select+" where room = ? and title = ?", room, title)
	defer rows.Close()

	list := baro_coop_doc_read(rows)
	if len(list) == 0 {
		return Baro_coop_doc_row{}, false
	}

	return list[0], true
}

// Baro_coop_doc_open : 작업 공간에 문서를 연다. 이미 있으면 다시 연다.
func Baro_coop_doc_open(db *sql.DB, room string, title string, opener string) {
	if _, found := Baro_coop_doc_get(db, room, title); found {
		Exec_DB(
			db,
			"update baro_coop_doc set status = 'open', opener = ?, open_date = ? where room = ? and title = ?",
			opener, Baro_now(), room, title,
		)

		return
	}

	Exec_DB(
		db,
		"insert into baro_coop_doc (id, room, title, num, date, status, opener, open_date) "+
			"values (?, ?, ?, '0', ?, 'open', ?, ?)",
		Baro_next_id(db, "baro_coop_doc"), room, title, Baro_now(), opener, Baro_now(),
	)
}

// Baro_coop_doc_close : 작업 공간에서 문서를 닫는다 (손댄 기록은 남는다)
func Baro_coop_doc_close(db *sql.DB, room string, title string) {
	Exec_DB(db, "update baro_coop_doc set status = '' where room = ? and title = ?", room, title)
}

// Baro_coop_doc_bump : 손댄 문서를 적어 둔다. 이미 있으면 횟수를 올린다.
//
//	작업 공간을 거치지 않고 그냥 문서를 고쳐도 이 길로 들어온다.
//	그때는 탭을 열지 않는다. 탭은 작업 공간에서 손수 열고 닫는 것이다.
func Baro_coop_doc_bump(db *sql.DB, room string, title string) int {
	num := ""
	if QueryRow_DB(db, "select num from baro_coop_doc where room = ? and title = ?", []any{&num}, room, title) {
		next := Str_to_int(num) + 1
		Exec_DB(
			db,
			"update baro_coop_doc set num = ?, date = ? where room = ? and title = ?",
			Baro_itoa(next), Baro_now(), room, title,
		)

		return next
	}

	Exec_DB(
		db,
		"insert into baro_coop_doc (id, room, title, num, date, status, opener, open_date) "+
			"values (?, ?, ?, '1', ?, '', '', '')",
		Baro_next_id(db, "baro_coop_doc"), room, title, Baro_now(),
	)

	return 1
}

// Baro_coop_doc_text : 손댄 문서를 한 줄로 (예 : 가·나*2·다)
func Baro_coop_doc_text(list []Baro_coop_doc_row) []string {
	out := []string{}

	for _, one := range list {
		text := one.Title
		if one.Num > 1 {
			text += "*" + Baro_itoa(one.Num)
		}

		out = append(out, text)
	}

	return out
}

// Baro_coop_log_row : 방 기록 한 줄
type Baro_coop_log_row struct {
	ID     string
	Kind   string
	User   string
	Data   string
	Detail string
	Date   string
	Status string
}

// Baro_coop_log_add : 방 기록 한 줄 남기기
func Baro_coop_log_add(db *sql.DB, room string, kind string, user string, data string, detail string) string {
	id := Baro_next_id(db, "baro_coop_log")

	Exec_DB(
		db,
		"insert into baro_coop_log (id, room, kind, user, data, detail, date, status) "+
			"values (?, ?, ?, ?, ?, ?, ?, 'on')",
		id, room, kind, user, Baro_trunc(data, 2000), Baro_trunc(detail, 500), Baro_now(),
	)

	return id
}

// Baro_coop_log_list : 방 기록 읽기 (kind 가 "" 면 모두)
func Baro_coop_log_list(db *sql.DB, room string, kind string, limit int) []Baro_coop_log_row {
	if limit <= 0 {
		limit = 100
	}

	query := "select id, kind, user, data, detail, date, status from baro_coop_log where room = ? and status = 'on'"
	args := []any{room}

	if kind != "" {
		query += " and kind = ?"
		args = append(args, kind)
	}

	query += " order by id + 0 desc limit ?"
	args = append(args, limit)

	rows := Query_DB(db, query, args...)
	defer rows.Close()

	out := []Baro_coop_log_row{}
	for rows.Next() {
		row := Baro_coop_log_row{}
		if rows.Scan(&row.ID, &row.Kind, &row.User, &row.Data, &row.Detail, &row.Date, &row.Status) != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// Baro_coop_log_hide : 기록 한 줄 감추기 (댓글 지우기)
func Baro_coop_log_hide(db *sql.DB, room string, id string) {
	Exec_DB(db, "update baro_coop_log set status = '' where room = ? and id = ?", room, id)
}

// ------------------------------------------------------------
//  영구 폐쇄 · 방장 넘기기 · 초대
// ------------------------------------------------------------

// Baro_coop_room_seal : 영구 폐쇄. 관리자만 걸 수 있고 관리자만 푼다.
//
//	방장이 하는 보통 폐쇄와 다르다. 방장은 이것을 스스로 풀 수 없다.
func Baro_coop_room_seal(db *sql.DB, room string, by string, reason string) {
	Exec_DB(
		db,
		"update baro_coop_room set status = ?, seal_by = ?, seal_reason = ?, seal_date = ?, close_date = ? where id = ?",
		Baro_coop_sealed, by, Baro_trunc(reason, 500), Baro_now(), Baro_now(), room,
	)
}

// Baro_coop_room_unseal : 영구 폐쇄 풀기 (관리자만)
func Baro_coop_room_unseal(db *sql.DB, room string) {
	Exec_DB(
		db,
		"update baro_coop_room set status = ?, close_at = '', close_date = '', "+
			"seal_by = '', seal_reason = '', seal_date = '' where id = ?",
		Baro_coop_open, room,
	)
}

// Baro_coop_room_owner_set : 방장을 다른 사람에게 넘긴다
//
//	넘겨받는 사람은 이 방에 든 적이 있어야 한다. 지금 나가 있어도 된다.
//	앞 방장은 편집 자리로 남는다.
func Baro_coop_room_owner_set(db *sql.DB, row Baro_coop_room_row, next string) bool {
	if next == "" || next == row.Owner || !Baro_is_user(next) {
		return false
	}

	member, found := Baro_coop_member_get(db, row.ID, next)

	// 불리기만 하고 한 번도 들어오지 않은 사람에게는 넘기지 않는다.
	if !found || member.Status == "invited" {
		return false
	}

	Exec_DB(db, "update baro_coop_room set owner = ? where id = ?", next, row.ID)

	// 새 방장은 편집 자리로 앉힌다. 나가 있었다면 그대로 나가 있는 채로 자리만 바꾼다.
	if member.Status == "in" {
		Baro_coop_member_set(db, row.ID, next, Baro_coop_owner)
	} else {
		Exec_DB(db, "update baro_coop_member set role = ? where room = ? and user = ?",
			Baro_coop_owner, row.ID, next)
	}

	// 앞 방장은 편집 자리로 내린다.
	if _, ok := Baro_coop_member_get(db, row.ID, row.Owner); ok {
		Exec_DB(db, "update baro_coop_member set role = ? where room = ? and user = ?",
			Baro_coop_edit, row.ID, row.Owner)
	}

	return true
}

// Baro_coop_room_ever_list : 이 방에 든 적이 있는 사람 (지금 있든 나갔든)
//
//	불리기만 하고 들어오지 않은 사람은 뺀다.
func Baro_coop_room_ever_list(db *sql.DB, room string) []Baro_coop_member_row {
	out := []Baro_coop_member_row{}

	for _, one := range Baro_coop_member_list(db, room, "") {
		if one.Status == "invited" {
			continue
		}

		out = append(out, one)
	}

	return out
}

// Baro_coop_room_oldest : 이 방에서 가장 먼저 들어온 사람 (방장은 뺀다)
//
//	방장직을 빼앗을 때 누구에게 넘길지 정하는 데 쓴다.
func Baro_coop_room_oldest(db *sql.DB, row Baro_coop_room_row) string {
	user := ""
	QueryRow_DB(
		db,
		"select user from baro_coop_member where room = ? and user <> ? and status <> 'invited' "+
			"order by id + 0 asc limit 1",
		[]any{&user}, row.ID, row.Owner,
	)

	return user
}

// Baro_coop_invite : 다른 사람을 이 방에 부른다
//
//	부르기만 할 뿐 자리에 앉히지는 않는다. 들어올지는 받은 사람이 정한다.
//	알림에 링크와 열쇠를 함께 넣어, 목록에 오르지 않는 방도 찾아올 수 있게 한다.
func Baro_coop_invite(db *sql.DB, row Baro_coop_room_row, by string, target string) bool {
	if !Baro_is_user(target) || target == by {
		return false
	}

	text := Baro_now() + "\n" +
		by + " 사용자가 " + target + " 사용자를 '" + row.Name + "' 방에 초대하였습니다.\n" +
		"링크: /baro/cooperation/" + row.Link

	if row.Code != "" {
		text += "\n코드: " + row.Code
	}

	// 비공개 방은 이름이 올라 있어야 볼 수 있다.
	// 불린 사람을 「불림」 자리로 올려 두어, 들어올지는 스스로 정하게 한다.
	if _, found := Baro_coop_member_get(db, row.ID, target); !found {
		Exec_DB(
			db,
			"insert into baro_coop_member (id, room, user, role, status, block, date, out_date, note) "+
				"values (?, ?, ?, ?, 'invited', '', ?, '', ?)",
			Baro_next_id(db, "baro_coop_member"), row.ID, target, Baro_coop_watch, Baro_now(), by,
		)
	}

	Baro_notify(
		db, target, "system",
		Baro_t("편집방 초대 : ")+row.Name,
		"/baro/cooperation/"+row.Link,
		text,
	)

	Baro_coop_log_add(db, row.ID, "invite", by, target+" 님을 불렀습니다.", "")

	return true
}

// ------------------------------------------------------------
//  방 안에서의 권한
// ------------------------------------------------------------

// Baro_coop_acl_get : 이 방에서 그 행위를 어느 자리까지 열어 두었는가
func Baro_coop_acl_get(row Baro_coop_room_row, act string) string {
	saved := Baro_json_dec_str(row.Acl)

	want := strings.TrimSpace(saved[act])
	for _, one := range Baro_coop_scope_list() {
		if one[0] == want {
			return want
		}
	}

	return Baro_coop_act_default(act)
}

// Baro_coop_acl_set : 행위별 범위를 한꺼번에 담는다
func Baro_coop_acl_text(pick map[string]string) string {
	out := map[string]string{}

	for _, one := range Baro_coop_act_list() {
		want := strings.TrimSpace(pick[one[0]])

		for _, scope := range Baro_coop_scope_list() {
			if scope[0] == want {
				out[one[0]] = want
				break
			}
		}
	}

	return Baro_json_enc(out)
}

// Baro_coop_can : 이 사람이 이 방에서 그 행위를 할 수 있는가
//
//	방장은 언제나 할 수 있고, 위키 운영진도 막지 않는다.
//	그 밖에는 방에 들어와 있어야 하고, 자리가 방이 정한 범위에 들어야 한다.
func Baro_coop_can(db *sql.DB, row Baro_coop_room_row, ip string, act string) bool {
	if !Baro_is_user(ip) {
		return false
	}

	if row.Owner == ip || Baro_can(db, ip, "system") {
		return true
	}

	member, found := Baro_coop_member_get(db, row.ID, ip)
	if !found || member.Status != "in" {
		return false
	}

	// 편집이 막힌 사람은 방 안에서 아무것도 하지 못한다. (댓글은 따로 본다)
	if member.Block && act != "comment" {
		return false
	}

	// 위키 권한 그룹에서 뺀 것은 방이 열어 두어도 할 수 없다.
	switch act {
	case "comment":
		if !Baro_coop_auth(db, ip, "coop_comment") {
			return false
		}
	case "notice":
		if !Baro_coop_auth(db, ip, "coop_notice") {
			return false
		}
	case "block":
		if !Baro_coop_auth(db, ip, "coop_block") {
			return false
		}
	}

	return Baro_coop_role_rank(member.Role) >= Baro_coop_scope_rank(Baro_coop_acl_get(row, act))
}
