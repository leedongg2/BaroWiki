package tool

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"hash/fnv"
	"log"
	"strings"
	"sync"
)

// ============================================================
//  고유 번호 (UUID)
//
//  판 번호(r3)와 계정 이름은 둘 다 바뀐다.
//    - 판 번호는 앞 판이 지워지면 밀리고, 역사를 초기화하면 처음부터 다시 매겨진다.
//    - 계정 이름은 사용자가 바꿀 수 있다.
//  그래서 「그때 그 판」 · 「그 사람」 을 가리키는 변하지 않는 이름이 따로 필요하다.
//
//  여기서 만드는 고유 번호는 RFC 4122 판 4 모양이고,
//  openNAMU 본체 표는 건드리지 않고 baro_ 표에만 담는다.
//    baro_rev_uuid  : 판 고유 번호   (uuid ↔ title + rev)
//    baro_user_set  : 계정 고유 번호 (name = 'uuid')
//
//  바깥에서 들어온 값은 언제나 Baro_uuid_is 로 모양부터 확인한 뒤 쓴다.
// ============================================================

// baro_uuid_locks : 같은 대상에 고유 번호가 두 개 생기지 않게 막는 잠금
//
//	문서 이름이 아무리 많아도 잠금 수는 고정이라 메모리가 늘지 않는다.
//	Lock_document_edit 과는 일부러 다른 잠금을 쓴다.
//	편집 잠금을 잡은 채로 판을 저장하는 경로가 있어서, 같은 잠금을 다시 잡으면 멈춰 버린다.
var baro_uuid_locks [64]sync.Mutex

// baro_uuid_lock : 열쇠말 하나를 잠그고, 푸는 함수를 돌려준다
func baro_uuid_lock(key string) func() {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))

	lock := &baro_uuid_locks[hash.Sum32()%uint32(len(baro_uuid_locks))]
	lock.Lock()

	return lock.Unlock
}

// Baro_uuid_new : 새 고유 번호 하나 (RFC 4122 판 4)
func Baro_uuid_new() string {
	buf := make([]byte, 16)

	if _, err := rand.Read(buf); err != nil {
		// 운영체제 난수를 못 읽는 드문 경우에도 번호는 나와야 한다
		raw, _ := hex.DecodeString(Sha224(Get_time() + Get_random_key(32))[:32])
		copy(buf, raw)
	}

	buf[6] = (buf[6] & 0x0f) | 0x40 // 판 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // 변형 RFC 4122

	out := hex.EncodeToString(buf)

	return out[0:8] + "-" + out[8:12] + "-" + out[12:16] + "-" + out[16:20] + "-" + out[20:32]
}

// Baro_uuid_is : 고유 번호 모양인가 (8-4-4-4-12 16진수)
func Baro_uuid_is(data string) bool {
	if len(data) != 36 {
		return false
	}

	for index := 0; index < len(data); index++ {
		one := data[index]

		if index == 8 || index == 13 || index == 18 || index == 23 {
			if one != '-' {
				return false
			}

			continue
		}

		is_hex := (one >= '0' && one <= '9') || (one >= 'a' && one <= 'f') || (one >= 'A' && one <= 'F')
		if !is_hex {
			return false
		}
	}

	return true
}

// baro_uuid_norm : 견주기 전에 소문자로 맞춘다
func baro_uuid_norm(data string) string {
	return strings.ToLower(strings.TrimSpace(data))
}

// baro_rev_uuid_key : 판 하나를 가리키는 열쇠말
func baro_rev_uuid_key(title string, rev string) string {
	return title + "\n" + rev
}

// baro_uuid_write : 번호 한 줄 넣기 (실패해도 화면을 죽이지 않는다)
//
//	번호를 붙이는 일은 문서를 읽는 도중에도 일어난다.
//	곁다리 기록 하나가 안 써졌다고 역사 화면 전체가 오류가 되면 안 되므로
//	여기서만 실패를 받아 넘기고 기록으로 남긴다. 다음에 읽을 때 다시 붙는다.
func baro_uuid_write(db *sql.DB, what string, query string, values ...any) (ok bool) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("[BARO UUID] %s 고유 번호를 남기지 못했습니다 : %v", what, err)
			ok = false
		}
	}()

	Exec_DB(db, query, values...)

	return true
}

// baro_rev_uuid_read : 이미 붙어 있는 판 고유 번호 (없으면 "")
func baro_rev_uuid_read(db *sql.DB, title string, rev string) string {
	value := ""
	QueryRow_DB(
		db,
		"select uuid from baro_rev_uuid where title = ? and rev = ?",
		[]any{&value},
		title, rev,
	)

	return value
}

// Baro_rev_uuid : 판 하나의 고유 번호 (아직 없으면 그 자리에서 붙인다)
//
//	예전부터 있던 판도 처음 물어볼 때 번호를 받고, 그 뒤로는 바뀌지 않는다.
func Baro_rev_uuid(db *sql.DB, title string, rev string) string {
	if title == "" || rev == "" {
		return ""
	}

	if value := baro_rev_uuid_read(db, title, rev); value != "" {
		return value
	}

	unlock := baro_uuid_lock("rev\n" + baro_rev_uuid_key(title, rev))
	defer unlock()

	// 잠금을 기다리는 사이에 다른 요청이 먼저 붙였을 수 있다
	if value := baro_rev_uuid_read(db, title, rev); value != "" {
		return value
	}

	value := Baro_uuid_new()
	if !baro_uuid_write(
		db, "r"+rev+" ("+title+")",
		"insert into baro_rev_uuid (uuid, title, rev, date) values (?, ?, ?, ?)",
		value, title, rev, Get_time(),
	) {
		return ""
	}

	return value
}

// Baro_rev_uuid_list : 여러 판의 고유 번호를 한 번에 (역사 목록 · 최근 변경용)
//
//	pairs 는 {문서 이름, 판 번호} 쌍이고, 돌려주는 map 의 열쇠말은 "문서\n판" 이다.
//	줄마다 따로 물으면 50줄에 50번을 묻게 되므로 있는 것을 먼저 한 번에 읽고,
//	그때 없던 것만 하나씩 붙인다.
func Baro_rev_uuid_list(db *sql.DB, pairs [][2]string) map[string]string {
	out := map[string]string{}
	if len(pairs) == 0 {
		return out
	}

	title_list := []string{}
	rev_list := []string{}
	title_seen := map[string]bool{}
	rev_seen := map[string]bool{}
	want := map[string]bool{}

	for _, one := range pairs {
		if one[0] == "" || one[1] == "" {
			continue
		}

		want[baro_rev_uuid_key(one[0], one[1])] = true

		if !title_seen[one[0]] {
			title_seen[one[0]] = true
			title_list = append(title_list, one[0])
		}
		if !rev_seen[one[1]] {
			rev_seen[one[1]] = true
			rev_list = append(rev_list, one[1])
		}
	}

	if len(want) == 0 {
		return out
	}

	values := []any{}
	for _, one := range title_list {
		values = append(values, one)
	}
	for _, one := range rev_list {
		values = append(values, one)
	}

	query := "select uuid, title, rev from baro_rev_uuid where title in (" +
		baro_uuid_marks(len(title_list)) + ") and rev in (" + baro_uuid_marks(len(rev_list)) + ")"

	rows := Query_DB(db, query, values...)
	for rows.Next() {
		value, title, rev := "", "", ""
		if rows.Scan(&value, &title, &rev) != nil {
			continue
		}

		key := baro_rev_uuid_key(title, rev)
		if want[key] {
			out[key] = value
		}
	}
	rows.Close()

	// 아직 번호가 없던 판만 하나씩 붙인다
	for _, one := range pairs {
		if one[0] == "" || one[1] == "" {
			continue
		}

		key := baro_rev_uuid_key(one[0], one[1])
		if out[key] == "" {
			out[key] = Baro_rev_uuid(db, one[0], one[1])
		}
	}

	return out
}

// baro_uuid_marks : "?, ?, ?" 만들기 (값은 언제나 자리표시자로 넘긴다)
func baro_uuid_marks(count int) string {
	if count < 1 {
		return "''"
	}

	return strings.TrimSuffix(strings.Repeat("?, ", count), ", ")
}

// Baro_rev_from_uuid : 고유 번호로 판 번호 찾기 (없으면 "")
//
//	title 을 함께 주면 그 문서 안에서 먼저 찾고, 못 찾으면 번호만으로 찾는다.
func Baro_rev_from_uuid(db *sql.DB, title string, value string) string {
	value = baro_uuid_norm(value)
	if !Baro_uuid_is(value) {
		return ""
	}

	rev := ""
	if title != "" {
		if QueryRow_DB(db, "select rev from baro_rev_uuid where uuid = ? and title = ?", []any{&rev}, value, title) {
			return rev
		}
	}

	QueryRow_DB(db, "select rev from baro_rev_uuid where uuid = ?", []any{&rev}, value)

	return rev
}

// Baro_rev_resolve : 판 자리에 고유 번호가 들어왔으면 판 번호로 바꿔 준다
//
//	고유 번호가 아니면 받은 값을 그대로 돌려주므로 어느 경로에 끼워도 안전하다.
func Baro_rev_resolve(db *sql.DB, title string, rev string) string {
	if !Baro_uuid_is(rev) {
		return rev
	}

	if found := Baro_rev_from_uuid(db, title, rev); found != "" {
		return found
	}

	// 모르는 번호면 없는 판으로 다뤄지도록 그대로 넘긴다
	return rev
}

// Baro_user_uuid : 계정 고유 번호 (아직 없으면 그 자리에서 붙인다)
//
//	비로그인 아이피는 계정이 아니므로 번호를 주지 않는다.
func Baro_user_uuid(db *sql.DB, user string) string {
	if user == "" || IP_or_user(user) {
		return ""
	}

	value := ""
	QueryRow_DB(db, "select data from baro_user_set where user = ? and name = 'uuid'", []any{&value}, user)
	if value != "" {
		return value
	}

	unlock := baro_uuid_lock("user\n" + user)
	defer unlock()

	QueryRow_DB(db, "select data from baro_user_set where user = ? and name = 'uuid'", []any{&value}, user)
	if value != "" {
		return value
	}

	value = Baro_uuid_new()
	if !baro_uuid_write(
		db, user,
		"insert into baro_user_set (user, name, data) values (?, 'uuid', ?)",
		user, value,
	) {
		return ""
	}

	return value
}

// Baro_user_from_uuid : 계정 고유 번호로 계정 찾기 (없으면 "")
func Baro_user_from_uuid(db *sql.DB, value string) string {
	value = baro_uuid_norm(value)
	if !Baro_uuid_is(value) {
		return ""
	}

	user := ""
	QueryRow_DB(db, "select user from baro_user_set where name = 'uuid' and data = ?", []any{&user}, value)

	return user
}

// Baro_user_name_log_add : 계정 이름이 바뀐 것을 이력에 남긴다
//
//	이름은 바뀌어도 고유 번호는 그대로이므로, 이력은 고유 번호에 매단다.
func Baro_user_name_log_add(db *sql.DB, user string, old_name string, new_name string, actor string) {
	if user == "" || old_name == new_name {
		return
	}

	user_uuid := Baro_user_uuid(db, user)
	if user_uuid == "" {
		return
	}

	Exec_DB(
		db,
		"insert into baro_user_name_log (id, uuid, user, old_name, new_name, actor, date) values (?, ?, ?, ?, ?, ?, ?)",
		Baro_next_id(db, "baro_user_name_log"),
		user_uuid, user, old_name, new_name, actor, Get_time(),
	)
}
