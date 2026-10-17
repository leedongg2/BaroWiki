package tool

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================
//  편집방 시험
// ============================================================

// baro_coop_boot : 편집방 표만 갖춘 가벼운 시험 DB
//
//	방과 사람만 다루는 시험은 이것으로 충분하다. 위키 표를 통째로 세우면 시험 하나에 몇 초씩 든다.
//	권한(Baro_coop_can)까지 보는 시험은 baro_coop_boot_full 을 쓴다.
func baro_coop_boot(t *testing.T) *sql.DB {
	t.Helper()

	db := baro_uuid_test_db(t)

	for _, query := range []string{
		"create table baro_set (name text default '', data text default '')",
		"create table baro_coop_room (id text default '', link text default '', code text default '', " +
			"name text default '', description text default '', owner text default '', open_kind text default '', " +
			"max_user text default '', status text default '', close_at text default '', notice text default '', " +
			"banned text default '', date text default '', close_date text default '', note text default '', " +
			"acl text default '', seal_by text default '', seal_reason text default '', seal_date text default '')",
		"create table baro_coop_member (id text default '', room text default '', user text default '', " +
			"role text default '', status text default '', block text default '', date text default '', " +
			"out_date text default '', note text default '')",
		"create table baro_coop_doc (id text default '', room text default '', title text default '', " +
			"num text default '', date text default '', status text default '', opener text default '', " +
			"open_date text default '')",
		"create table baro_coop_log (id text default '', room text default '', kind text default '', " +
			"user text default '', data text default '', detail text default '', date text default '', " +
			"status text default '')",
		"create table baro_notify (id text default '', user text default '', kind text default '', " +
			"title text default '', url text default '', data text default '', readme text default '', " +
			"date text default '')",
		"create table baro_notify_set (user text default '', kind text default '', mode text default '')",
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}

	return db
}

// baro_coop_boot_full : 위키 표까지 모두 갖춘 시험 DB
//
//	편집방 권한은 위키 권한 그룹(user_set · alist · rb …)을 타고 오므로,
//	그것까지 보는 시험은 온전한 스키마가 있어야 한다.
func baro_coop_boot_full(t *testing.T) *sql.DB {
	t.Helper()

	t.Setenv("NAMU_DB", filepath.Join(t.TempDir(), "coop"))
	t.Setenv("NAMU_DB_TYPE", "sqlite")

	DB_shutdown()
	DB_boot()
	t.Cleanup(func() { DB_shutdown() })

	db := DB_connect()
	if err := DB_make(db, Get_DB_set()); err != nil {
		t.Fatal(err)
	}

	for _, row := range [][2]string{{"ip", "ip"}, {"user", "user"}, {"owner", "owner"}} {
		Exec_DB(db, "insert into alist (name, acl) values (?, ?)", row[0], row[1])
	}

	return db
}

// baro_coop_user : 시험용 계정 하나 세우기
//
//	편집방 권한은 위키 권한 그룹을 타고 오므로, 계정이 실제로 있어야 한다.
func baro_coop_user(db *sql.DB, name string) {
	Exec_DB(db, "insert into user_set (id, name, data) values (?, 'pw', 'x')", name)
	Exec_DB(db, "insert into user_set (id, name, data) values (?, 'acl', 'user')", name)
}

// TestBaroCoopBanApply : 금칙어 바꿔 넣기 — 물어본 예시 그대로
func TestBaroCoopBanApply(t *testing.T) {
	list := Baro_coop_ban_parse("A|(No)\npl|ㄱㄴ")

	if len(list) != 2 {
		t.Fatalf("금칙어를 둘로 읽지 못했다: %+v", list)
	}

	out, hit := Baro_coop_ban_apply("Apple PARTY", list)

	if out != "(No)pㄱㄴe P(No)RTY" {
		t.Fatalf("바뀐 글이 다르다: %q", out)
	}
	if len(hit) != 2 || hit[0] != "A" || hit[1] != "pl" {
		t.Fatalf("걸린 금칙어가 다르다: %v", hit)
	}

	notice := Baro_coop_ban_notice(hit, out)
	want := "경고: 금칙어 'A', 'pl'(이)가 포함되어 있으며, 입력시 다음으로 조정됩니다:\n> (No)pㄱㄴe P(No)RTY"

	if notice != want {
		t.Fatalf("안내 문구가 다르다:\n받음: %q\n기대: %q", notice, want)
	}

	// 걸리는 말이 없으면 아무 말도 하지 않는다.
	clean, none := Baro_coop_ban_apply("조용한 글", list)
	if clean != "조용한 글" || len(none) != 0 {
		t.Fatalf("걸리지 않아야 하는데 바뀌었다: %q %v", clean, none)
	}
	if Baro_coop_ban_notice(none, clean) != "" {
		t.Fatal("걸린 것이 없는데 안내가 나왔다")
	}

	// 바꿀 글자를 비우면 그 말이 지워진다.
	drop := Baro_coop_ban_parse("욕|")
	got, _ := Baro_coop_ban_apply("이런 욕 하지 마", drop)
	if got != "이런  하지 마" {
		t.Fatalf("빈 글자로 지워지지 않았다: %q", got)
	}
}

// TestBaroCoopLinkAndCode : 이름표와 열쇠
func TestBaroCoopLinkAndCode(t *testing.T) {
	seen := map[string]bool{}

	for i := 0; i < 300; i++ {
		link := Baro_coop_link_new()

		if len(link) < 6 {
			t.Fatalf("이름표가 너무 짧다: %q", link)
		}

		for _, one := range link {
			if one < 'a' || one > 'z' {
				t.Fatalf("이름표에 영소문자가 아닌 것이 있다: %q", link)
			}
		}

		seen[link] = true
	}

	// 300번 뽑아 거의 다 달라야 한다.
	if len(seen) < 280 {
		t.Fatalf("이름표가 너무 자주 겹친다: %d/300", len(seen))
	}

	for i := 0; i < 50; i++ {
		code := Baro_coop_code_new()

		if len(code) != 12 {
			t.Fatalf("열쇠가 12자리가 아니다: %q", code)
		}

		for _, one := range code {
			if one < '0' || one > '9' {
				t.Fatalf("열쇠에 숫자가 아닌 것이 있다: %q", code)
			}
		}
	}

	// 적어 낸 값에서 숫자만 남긴다
	if got := Baro_coop_code_norm(" 1234-5678 9012 "); got != "123456789012" {
		t.Fatalf("열쇠 다듬기가 다르다: %q", got)
	}
}

// TestBaroCoopRoom : 방 만들고 사람 드나들기
func TestBaroCoopRoom(t *testing.T) {
	db := baro_coop_boot(t)

	link := Baro_coop_room_add(db, Baro_coop_room_row{
		Name: "정리방", Description: "설명", Owner: "방장",
		OpenKind: Baro_coop_public, MaxUser: 2, Banned: "A|(No)",
	})
	if link == "" {
		t.Fatal("방을 만들지 못했다")
	}

	room, found := Baro_coop_room_get(db, link)
	if !found || room.Name != "정리방" || room.MaxUser != 2 {
		t.Fatalf("만든 방을 되읽지 못했다: %+v", room)
	}
	if !room.Baro_coop_room_live() {
		t.Fatal("갓 만든 방이 살아 있지 않다")
	}

	// 방을 연 사람이 방장으로 들어가 있다
	owner, ok := Baro_coop_member_get(db, room.ID, "방장")
	if !ok || owner.Role != Baro_coop_owner || owner.Status != "in" {
		t.Fatalf("방장이 제대로 앉지 않았다: %+v", owner)
	}
	if Baro_coop_owner_away(db, room) {
		t.Fatal("방장이 있는데 부재라고 한다")
	}

	// 방장은 정원에 들지 않는다
	edit_count, watch_count := Baro_coop_count(db, room.ID)
	if edit_count != 0 || watch_count != 0 {
		t.Fatalf("인원이 다르다: %d %d", edit_count, watch_count)
	}

	// 두 사람이 들어와야 정원(2)이 찬다
	Baro_coop_member_set(db, room.ID, "둘째", Baro_coop_edit)
	if Baro_coop_full(db, room) {
		t.Fatal("한 사람뿐인데 두 자리 방이 찼다고 한다")
	}

	Baro_coop_member_set(db, room.ID, "넷째", Baro_coop_edit)
	if !Baro_coop_full(db, room) {
		t.Fatal("정원이 찼는데 차지 않았다고 한다")
	}

	// 관전은 정원에 들지 않는다
	Baro_coop_member_set(db, room.ID, "셋째", Baro_coop_watch)

	edit_count, watch_count = Baro_coop_count(db, room.ID)
	if edit_count != 2 || watch_count != 1 {
		t.Fatalf("관전이 정원에 섞였다: %d %d", edit_count, watch_count)
	}

	// 나가면 기록이 남고 자리가 빈다
	Baro_coop_member_out(db, room.ID, "둘째")

	if Baro_coop_full(db, room) {
		t.Fatal("나갔는데 정원이 그대로다")
	}
	if len(Baro_coop_member_list(db, room.ID, "out")) != 1 {
		t.Fatal("나간 기록이 남지 않았다")
	}

	// 방장이 나가면 부재
	Baro_coop_member_out(db, room.ID, "방장")
	if !Baro_coop_owner_away(db, room) {
		t.Fatal("방장이 나갔는데 부재가 아니다")
	}
}

// TestBaroCoopClose : 닫고 다시 열기
func TestBaroCoopClose(t *testing.T) {
	db := baro_coop_boot(t)

	link := Baro_coop_room_add(db, Baro_coop_room_row{
		Name: "닫을 방", Owner: "방장", OpenKind: Baro_coop_public, MaxUser: 5,
	})

	room, _ := Baro_coop_room_get(db, link)

	Baro_coop_room_close(db, room.ID)

	room, _ = Baro_coop_room_get(db, link)
	if room.Status != Baro_coop_closing {
		t.Fatalf("닫는 중이 아니다: %s", room.Status)
	}
	if !room.Baro_coop_room_live() {
		t.Fatal("닫는 중에는 아직 쓸 수 있어야 한다")
	}

	// 때가 지나면 읽을 때 저절로 닫힌다
	Exec_DB(db, "update baro_coop_room set close_at = '2000-01-01 00:00:00' where id = ?", room.ID)

	room, _ = Baro_coop_room_get(db, link)
	if room.Status != Baro_coop_closed {
		t.Fatalf("때가 지났는데 닫히지 않았다: %s", room.Status)
	}
	if room.Baro_coop_room_live() {
		t.Fatal("닫힌 방이 살아 있다고 한다")
	}

	Baro_coop_room_reopen(db, room.ID)

	room, _ = Baro_coop_room_get(db, link)
	if room.Status != Baro_coop_open {
		t.Fatalf("다시 열리지 않았다: %s", room.Status)
	}
}

// TestBaroCoopNow : 지금 들어가 있는 방 고르기
func TestBaroCoopNow(t *testing.T) {
	db := baro_coop_boot(t)

	link := Baro_coop_room_add(db, Baro_coop_room_row{
		Name: "일하는 방", Owner: "방장", OpenKind: Baro_coop_public, MaxUser: 5,
	})
	room, _ := Baro_coop_room_get(db, link)

	if _, ok := Baro_coop_now(db, "방장"); !ok {
		t.Fatal("편집 자리에 있는데 방을 찾지 못했다")
	}

	// 관전은 잡지 않는다
	Baro_coop_member_set(db, room.ID, "구경꾼", Baro_coop_watch)
	if _, ok := Baro_coop_now(db, "구경꾼"); ok {
		t.Fatal("관전인데 방이 잡혔다")
	}

	// 편집이 막힌 사람도 잡지 않는다
	Baro_coop_member_set(db, room.ID, "막힌이", Baro_coop_edit)
	Baro_coop_member_block(db, room.ID, "막힌이", true)

	if _, ok := Baro_coop_now(db, "막힌이"); ok {
		t.Fatal("편집이 막혔는데 방이 잡혔다")
	}

	Baro_coop_member_block(db, room.ID, "막힌이", false)
	if _, ok := Baro_coop_now(db, "막힌이"); !ok {
		t.Fatal("제한을 풀었는데 방이 잡히지 않았다")
	}

	// 나가면 잡히지 않는다
	Baro_coop_member_out(db, room.ID, "방장")
	if _, ok := Baro_coop_now(db, "방장"); ok {
		t.Fatal("나갔는데 방이 잡혔다")
	}

	// 아이피에게는 방이 없다
	if _, ok := Baro_coop_now(db, "192.0.2.1"); ok {
		t.Fatal("비로그인에게 방이 잡혔다")
	}
}

// TestBaroCoopEditMark : 방에서 한 편집에 방이 붙는다
func TestBaroCoopEditMark(t *testing.T) {
	db := baro_coop_boot(t)

	link := Baro_coop_room_add(db, Baro_coop_room_row{
		Name: "정리방", Owner: "갑", OpenKind: Baro_coop_public, MaxUser: 5,
	})
	room, _ := Baro_coop_room_get(db, link)
	Baro_coop_member_set(db, room.ID, "을", Baro_coop_edit)

	got := Baro_coop_edit_mark(db, "갑", "어떤 문서", "다듬음")

	if !strings.Contains(got, "다듬음") {
		t.Fatalf("원래 요약이 사라졌다: %q", got)
	}
	for _, want := range []string{"편집방", "정리방", "갑", "을", "/baro/cooperation/" + link} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q 가 빠졌다: %q", want, got)
		}
	}

	// 손댄 문서로 적히고, 또 고치면 횟수가 오른다
	list := Baro_coop_doc_list(db, room.ID)
	if len(list) != 1 || list[0].Title != "어떤 문서" || list[0].Num != 1 {
		t.Fatalf("손댄 문서가 적히지 않았다: %+v", list)
	}

	Baro_coop_edit_mark(db, "갑", "어떤 문서", "또 다듬음")

	list = Baro_coop_doc_list(db, room.ID)
	if len(list) != 1 || list[0].Num != 2 {
		t.Fatalf("같은 문서 횟수가 오르지 않았다: %+v", list)
	}

	if text := Baro_coop_doc_text(list); len(text) != 1 || text[0] != "어떤 문서*2" {
		t.Fatalf("*n 이 붙지 않았다: %v", text)
	}

	// 방에 들어가 있지 않은 사람의 요약은 그대로다
	if got := Baro_coop_edit_mark(db, "남", "어떤 문서", "혼자 함"); got != "혼자 함" {
		t.Fatalf("방 밖 편집에 방이 붙었다: %q", got)
	}

	// 방이 닫히면 더는 붙지 않는다
	Baro_coop_room_close(db, room.ID)
	Exec_DB(db, "update baro_coop_room set close_at = '2000-01-01 00:00:00' where id = ?", room.ID)

	if got := Baro_coop_edit_mark(db, "갑", "어떤 문서", "닫힌 뒤"); got != "닫힌 뒤" {
		t.Fatalf("닫힌 방인데 붙었다: %q", got)
	}
}

// TestBaroCoopCodeLookup : 12자리 열쇠로 방 찾기
func TestBaroCoopCodeLookup(t *testing.T) {
	db := baro_coop_boot(t)

	link := Baro_coop_room_add(db, Baro_coop_room_row{
		Name: "조용한 방", Owner: "방장", OpenKind: Baro_coop_code, MaxUser: 5,
	})

	room, _ := Baro_coop_room_get(db, link)
	if len(room.Code) != 12 {
		t.Fatalf("열쇠가 12자리가 아니다: %q", room.Code)
	}

	found, ok := Baro_coop_room_by_code(db, room.Code)
	if !ok || found.Link != link {
		t.Fatal("열쇠로 방을 찾지 못했다")
	}

	// 사이에 다른 글자가 끼어도 숫자만 보고 찾는다
	spaced := room.Code[:4] + "-" + room.Code[4:8] + " " + room.Code[8:]
	if _, ok := Baro_coop_room_by_code(db, spaced); !ok {
		t.Fatalf("띄어 적은 열쇠를 못 읽었다: %q", spaced)
	}

	if _, ok := Baro_coop_room_by_code(db, "999999999999"); ok {
		t.Fatal("없는 열쇠로 방이 나왔다")
	}
	if _, ok := Baro_coop_room_by_code(db, "123"); ok {
		t.Fatal("12자리가 아닌 열쇠가 통했다")
	}

	// 일부 공개 방은 공개 목록에 오르지 않는다
	if len(Baro_coop_room_list(db, "public", "", 0, 10)) != 0 {
		t.Fatal("일부 공개 방이 공개 목록에 올랐다")
	}
}

// TestBaroCoopOwnerNotCounted : 방장은 정원에 들지 않는다
func TestBaroCoopOwnerNotCounted(t *testing.T) {
	db := baro_coop_boot(t)

	link := Baro_coop_room_add(db, Baro_coop_room_row{
		Name: "한 자리 방", Owner: "방장", OpenKind: Baro_coop_public, MaxUser: 1,
	})
	room, _ := Baro_coop_room_get(db, link)

	// 방장만 있을 때는 아직 한 자리가 남아 있다.
	edit_count, _ := Baro_coop_count(db, room.ID)
	if edit_count != 0 {
		t.Fatalf("방장이 정원에 들었다: %d", edit_count)
	}
	if Baro_coop_full(db, room) {
		t.Fatal("방장뿐인데 자리가 찼다고 한다")
	}

	Baro_coop_member_set(db, room.ID, "둘째", Baro_coop_edit)

	if !Baro_coop_full(db, room) {
		t.Fatal("한 자리 방에 한 사람이 들었는데 차지 않았다")
	}

	// 자리가 차도 방장은 언제나 제 방에 들어갈 수 있다.
	if Baro_coop_full_for(db, room, "방장") {
		t.Fatal("방장이 제 방에 못 들어간다")
	}
	if !Baro_coop_full_for(db, room, "셋째") {
		t.Fatal("남인데 자리가 있다고 한다")
	}
}

// TestBaroCoopBlockedIsWatcher : 편집이 막힌 사람은 관전으로 친다
func TestBaroCoopBlockedIsWatcher(t *testing.T) {
	db := baro_coop_boot(t)

	link := Baro_coop_room_add(db, Baro_coop_room_row{
		Name: "방", Owner: "방장", OpenKind: Baro_coop_public, MaxUser: 5,
	})
	room, _ := Baro_coop_room_get(db, link)

	Baro_coop_member_set(db, room.ID, "막힐이", Baro_coop_edit)

	edit_count, watch_count := Baro_coop_count(db, room.ID)
	if edit_count != 1 || watch_count != 0 {
		t.Fatalf("막기 전 인원이 다르다: %d %d", edit_count, watch_count)
	}

	Baro_coop_member_block(db, room.ID, "막힐이", true)

	edit_count, watch_count = Baro_coop_count(db, room.ID)
	if edit_count != 0 || watch_count != 1 {
		t.Fatalf("막힌 사람이 관전으로 가지 않았다: %d %d", edit_count, watch_count)
	}

	member, _ := Baro_coop_member_get(db, room.ID, "막힐이")
	if member.Baro_coop_member_editing() {
		t.Fatal("막혔는데 편집 자리라고 한다")
	}

	// 편집도 방에 잡히지 않는다
	if got := Baro_coop_edit_mark(db, "막힐이", "문서", "혼자"); got != "혼자" {
		t.Fatalf("막힌 사람의 편집에 방이 붙었다: %q", got)
	}

	Baro_coop_member_block(db, room.ID, "막힐이", false)

	edit_count, _ = Baro_coop_count(db, room.ID)
	if edit_count != 1 {
		t.Fatalf("제한을 풀었는데 편집 자리로 안 돌아왔다: %d", edit_count)
	}
}

// TestBaroCoopSeal : 영구 폐쇄는 보통 폐쇄와 다르다
func TestBaroCoopSeal(t *testing.T) {
	db := baro_coop_boot(t)

	link := Baro_coop_room_add(db, Baro_coop_room_row{
		Name: "방", Owner: "방장", OpenKind: Baro_coop_public, MaxUser: 5,
	})
	room, _ := Baro_coop_room_get(db, link)

	Baro_coop_room_seal(db, room.ID, "관리자", "규정 위반")

	room, _ = Baro_coop_room_get(db, link)
	if !room.Baro_coop_room_sealed() {
		t.Fatalf("영구 폐쇄가 되지 않았다: %s", room.Status)
	}
	if room.Baro_coop_room_live() {
		t.Fatal("영구 폐쇄된 방이 살아 있다고 한다")
	}
	if room.SealBy != "관리자" || room.SealReason != "규정 위반" {
		t.Fatalf("폐쇄 기록이 남지 않았다: %+v", room)
	}

	// 때가 지나도 저절로 풀리지 않는다
	Exec_DB(db, "update baro_coop_room set close_at = '2000-01-01 00:00:00' where id = ?", room.ID)

	room, _ = Baro_coop_room_get(db, link)
	if !room.Baro_coop_room_sealed() {
		t.Fatal("영구 폐쇄가 저절로 풀렸다")
	}

	// 공개 목록에도 오르지 않는다
	if len(Baro_coop_room_list(db, "public", "", 0, 10)) != 0 {
		t.Fatal("영구 폐쇄된 방이 목록에 남아 있다")
	}

	// 방에서 한 편집도 잡히지 않는다
	if got := Baro_coop_edit_mark(db, "방장", "문서", "요약"); got != "요약" {
		t.Fatalf("영구 폐쇄된 방에서 편집이 잡혔다: %q", got)
	}

	Baro_coop_room_unseal(db, room.ID)

	room, _ = Baro_coop_room_get(db, link)
	if room.Status != Baro_coop_open || room.SealBy != "" {
		t.Fatalf("영구 폐쇄가 풀리지 않았다: %+v", room)
	}
}

// TestBaroCoopHandOver : 방장직 넘기기
func TestBaroCoopHandOver(t *testing.T) {
	db := baro_coop_boot(t)

	link := Baro_coop_room_add(db, Baro_coop_room_row{
		Name: "방", Owner: "갑", OpenKind: Baro_coop_public, MaxUser: 5,
	})
	room, _ := Baro_coop_room_get(db, link)

	// 든 적이 없는 사람에게는 못 넘긴다
	if Baro_coop_room_owner_set(db, room, "남") {
		t.Fatal("든 적 없는 사람에게 넘어갔다")
	}

	Baro_coop_member_set(db, room.ID, "을", Baro_coop_edit)

	if !Baro_coop_room_owner_set(db, room, "을") {
		t.Fatal("넘기지 못했다")
	}

	room, _ = Baro_coop_room_get(db, link)
	if room.Owner != "을" {
		t.Fatalf("방장이 바뀌지 않았다: %s", room.Owner)
	}

	next, _ := Baro_coop_member_get(db, room.ID, "을")
	if next.Role != Baro_coop_owner {
		t.Fatalf("새 방장의 자리가 다르다: %s", next.Role)
	}

	before, _ := Baro_coop_member_get(db, room.ID, "갑")
	if before.Role != Baro_coop_edit {
		t.Fatalf("앞 방장이 편집 자리로 내려오지 않았다: %s", before.Role)
	}

	// 나간 사람에게도 넘길 수 있다
	Baro_coop_member_set(db, room.ID, "병", Baro_coop_edit)
	Baro_coop_member_out(db, room.ID, "병")

	room, _ = Baro_coop_room_get(db, link)
	if !Baro_coop_room_owner_set(db, room, "병") {
		t.Fatal("나갔던 사람에게 넘기지 못했다")
	}

	// 가장 먼저 들어온 사람 고르기 (방장 제외)
	room, _ = Baro_coop_room_get(db, link)
	if got := Baro_coop_room_oldest(db, room); got != "갑" {
		t.Fatalf("가장 오래된 사람이 다르다: %s", got)
	}
}

// TestBaroCoopRoomAcl : 방 안 권한
func TestBaroCoopRoomAcl(t *testing.T) {
	db := baro_coop_boot_full(t)

	link := Baro_coop_room_add(db, Baro_coop_room_row{
		Name: "방", Owner: "방장", OpenKind: Baro_coop_public, MaxUser: 9,
		Acl: Baro_coop_acl_text(map[string]string{
			"comment": Baro_coop_to_member,
			"notice":  Baro_coop_only_owner,
			"block":   Baro_coop_to_sub,
		}),
	})
	room, _ := Baro_coop_room_get(db, link)

	if got := Baro_coop_acl_get(room, "notice"); got != Baro_coop_only_owner {
		t.Fatalf("담아 둔 범위가 다르다: %s", got)
	}

	// 적어 두지 않은 것은 기본값을 따른다
	if got := Baro_coop_acl_get(room, "invite"); got != Baro_coop_act_default("invite") {
		t.Fatalf("기본 범위를 따르지 않는다: %s", got)
	}

	for _, one := range []string{"방장", "부방장", "참여자", "남"} {
		baro_coop_user(db, one)
	}

	Baro_coop_member_set(db, room.ID, "부방장", Baro_coop_sub)
	Baro_coop_member_set(db, room.ID, "참여자", Baro_coop_edit)

	// 방장은 무엇이든
	for _, act := range []string{"comment", "notice", "block", "setting"} {
		if !Baro_coop_can(db, room, "방장", act) {
			t.Fatalf("방장이 %s 를 못 한다", act)
		}
	}

	// 부방장은 공지를 못 한다 (방장만으로 두었다)
	if Baro_coop_can(db, room, "부방장", "notice") {
		t.Fatal("부방장이 방장 전용 공지를 했다")
	}
	if !Baro_coop_can(db, room, "부방장", "block") {
		t.Fatal("부방장이 편집 제한을 못 한다")
	}

	// 참여자는 댓글만
	if !Baro_coop_can(db, room, "참여자", "comment") {
		t.Fatal("참여자가 댓글을 못 단다")
	}
	if Baro_coop_can(db, room, "참여자", "block") {
		t.Fatal("참여자가 편집 제한을 했다")
	}

	// 방에 없는 사람은 아무것도 못 한다
	if Baro_coop_can(db, room, "남", "comment") {
		t.Fatal("방 밖 사람이 댓글을 달았다")
	}
}

// TestBaroCoopDocTab : 작업 공간의 문서 탭
func TestBaroCoopDocTab(t *testing.T) {
	db := baro_coop_boot(t)

	link := Baro_coop_room_add(db, Baro_coop_room_row{
		Name: "작업방", Owner: "방장", OpenKind: Baro_coop_public, MaxUser: 5,
	})
	room, _ := Baro_coop_room_get(db, link)

	if len(Baro_coop_doc_open_list(db, room.ID)) != 0 {
		t.Fatal("갓 만든 방에 열린 문서가 있다")
	}

	Baro_coop_doc_open(db, room.ID, "가 문서", "방장")
	Baro_coop_doc_open(db, room.ID, "나 문서", "방장")

	open_list := Baro_coop_doc_open_list(db, room.ID)
	if len(open_list) != 2 {
		t.Fatalf("열린 문서가 둘이 아니다: %d", len(open_list))
	}

	// 연 차례를 지킨다
	if open_list[0].Title != "가 문서" || open_list[1].Title != "나 문서" {
		t.Fatalf("연 차례가 다르다: %+v", open_list)
	}

	one, found := Baro_coop_doc_get(db, room.ID, "가 문서")
	if !found || !one.Baro_coop_doc_open_now() || one.Opener != "방장" {
		t.Fatalf("연 기록이 다르다: %+v", one)
	}

	// 아직 손대지 않았으니 횟수는 0
	if one.Num != 0 {
		t.Fatalf("열기만 했는데 횟수가 올랐다: %d", one.Num)
	}

	// 닫아도 기록은 남는다
	Baro_coop_doc_close(db, room.ID, "가 문서")

	if len(Baro_coop_doc_open_list(db, room.ID)) != 1 {
		t.Fatal("닫았는데 탭이 그대로다")
	}
	if len(Baro_coop_doc_list(db, room.ID)) != 2 {
		t.Fatal("닫으면서 기록까지 지워졌다")
	}

	// 다시 열 수 있다
	Baro_coop_doc_open(db, room.ID, "가 문서", "둘째")

	again, _ := Baro_coop_doc_get(db, room.ID, "가 문서")
	if !again.Baro_coop_doc_open_now() || again.Opener != "둘째" {
		t.Fatalf("다시 열리지 않았다: %+v", again)
	}

	// 편집하면 횟수만 오르고 탭 상태는 건드리지 않는다
	Baro_coop_doc_close(db, room.ID, "가 문서")
	Baro_coop_doc_bump(db, room.ID, "가 문서")

	after, _ := Baro_coop_doc_get(db, room.ID, "가 문서")
	if after.Num != 1 {
		t.Fatalf("손댄 횟수가 오르지 않았다: %d", after.Num)
	}
	if after.Baro_coop_doc_open_now() {
		t.Fatal("닫아 둔 탭이 편집 때문에 다시 열렸다")
	}

	// 작업 공간을 거치지 않고 처음 손댄 문서는 탭이 열리지 않는다
	Baro_coop_doc_bump(db, room.ID, "다 문서")

	fresh, _ := Baro_coop_doc_get(db, room.ID, "다 문서")
	if fresh.Baro_coop_doc_open_now() {
		t.Fatal("그냥 편집했는데 탭이 열렸다")
	}
	if fresh.Num != 1 {
		t.Fatalf("첫 편집 횟수가 1이 아니다: %d", fresh.Num)
	}
}

// TestBaroCoopDocAct : 문서 추가 · 닫기 권한
func TestBaroCoopDocAct(t *testing.T) {
	db := baro_coop_boot_full(t)

	for _, one := range []string{"방장", "부방장", "참여자"} {
		baro_coop_user(db, one)
	}

	link := Baro_coop_room_add(db, Baro_coop_room_row{
		Name: "작업방", Owner: "방장", OpenKind: Baro_coop_public, MaxUser: 5,
	})
	room, _ := Baro_coop_room_get(db, link)

	Baro_coop_member_set(db, room.ID, "부방장", Baro_coop_sub)
	Baro_coop_member_set(db, room.ID, "참여자", Baro_coop_edit)

	// 기본값 : 추가는 참여자 모두, 닫기는 부방장까지
	if !Baro_coop_can(db, room, "참여자", "doc_add") {
		t.Fatal("참여자가 문서를 추가하지 못한다")
	}
	if Baro_coop_can(db, room, "참여자", "doc_close") {
		t.Fatal("참여자가 문서를 닫을 수 있다")
	}
	if !Baro_coop_can(db, room, "부방장", "doc_close") {
		t.Fatal("부방장이 문서를 닫지 못한다")
	}

	// 방마다 바꿀 수 있다
	room.Acl = Baro_coop_acl_text(map[string]string{
		"doc_add":   Baro_coop_only_owner,
		"doc_close": Baro_coop_to_member,
	})
	Baro_coop_room_save(db, room)

	room, _ = Baro_coop_room_get(db, link)

	if Baro_coop_can(db, room, "참여자", "doc_add") {
		t.Fatal("방장만으로 두었는데 참여자가 추가했다")
	}
	if !Baro_coop_can(db, room, "참여자", "doc_close") {
		t.Fatal("참여자 모두로 두었는데 닫지 못한다")
	}
	if !Baro_coop_can(db, room, "방장", "doc_add") {
		t.Fatal("방장이 문서를 추가하지 못한다")
	}
}
