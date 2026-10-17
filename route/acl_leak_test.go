package route

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"opennamu/route/tool"
)

// ============================================================
//  읽기 권한이 없는 문서가 새는지 지키는 시험
//
//  새로 만든 화면(땅따먹기 · 바로게임 · 요청)은 문서를 목록으로 늘어놓는다.
//  여기서 권한을 빠뜨리면 제목만으로도, 문서 맞히기는 본문까지 새어 나간다.
//  그래서 「비공개 문서가 절대 끼지 않는다」를 시험으로 못 박아 둔다.
// ============================================================

// acl_leak_db : 공개 문서와 비공개 문서가 섞인 위키
func acl_leak_db(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "leak.db"))
	if err != nil {
		t.Fatal(err)
	}

	for _, query := range []string{
		"create table other (name text default '', data text default '', coverage text default '')",
		"create table data (title text default '', data text default '', type text default '')",
		"create table acl (title text default '', data text default '', type text default '')",
		"create table alist (name text default '', acl text default '')",
		"create table user_set (name text default '', id text default '', data text default '')",
		"create table rb (block text default '', end text default '', today text default '', blocker text default '', why text default '', band text default '', login text default '', ongoing text default '')",
		"create table history (id text default '', title text default '', data text default '', date text default '', ip text default '', send text default '', leng text default '', hide text default '', type text default '')",
		"create table back (title text default '', link text default '', type text default '', data text default '')",
		"create table data_set (doc_name text default '', doc_rev text default '', set_name text default '', set_data text default '')",
		"create table baro_edit_req (id text default '', title text default '', base_rev text default '', user text default '', data text default '', summary text default '', status text default '', handler text default '', handle_date text default '', reason text default '', date text default '', new_rev text default '', kind text default '', target text default '', locked text default '')",
	} {
		tool.Exec_DB(db, query)
	}

	tool.Exec_DB(db, "insert into alist (name, acl) values ('ip', 'ip')")
	tool.Exec_DB(db, "insert into alist (name, acl) values ('user', 'user')")
	tool.Exec_DB(db, "insert into alist (name, acl) values ('owner', 'owner')")

	// 본문은 충분히 길게 (문서 맞히기가 200자 넘는 문서만 쓴다)
	long := strings.Repeat("이 문서는 시험을 위해 넉넉히 길게 적어 둔 본문입니다. 여러 문장이 이어집니다. ", 8)

	public := []string{"공개 문서 하나", "공개 문서 둘", "공개 문서 셋", "공개 문서 넷", "공개 문서 다섯"}
	for _, one := range public {
		tool.Exec_DB(db, "insert into data (title, data) values (?, ?)", one, long)
		tool.Exec_DB(db, "insert into history (id, title, data, date, ip, send, leng, hide, type) values ('1', ?, ?, '2026-09-01 00:00:00', 'wy', '', '+400', '', '')", one, long)
	}

	// 비공개 문서 : 읽기 권한이 owner 로 잠겨 있다
	secret := "비밀 문서"
	secret_body := long + " 여기에는 절대로 새면 안 되는 비밀낱말이 들어 있습니다."
	tool.Exec_DB(db, "insert into data (title, data) values (?, ?)", secret, secret_body)
	tool.Exec_DB(db, "insert into history (id, title, data, date, ip, send, leng, hide, type) values ('1', ?, ?, '2026-09-02 00:00:00', 'wy', '', '+9999', '', '')", secret, secret_body)
	tool.Exec_DB(db, "insert into acl (title, data, type) values (?, 'owner', 'view')", secret)

	// 공개 문서에서 비밀 문서로 링크를 걸어 둔다 (레이스가 그 링크를 보여 주면 안 된다)
	tool.Exec_DB(db, "insert into back (link, title, type, data) values (?, ?, '', '')", "공개 문서 하나", secret)
	tool.Exec_DB(db, "insert into back (link, title, type, data) values (?, ?, '', '')", "공개 문서 하나", "공개 문서 둘")

	return db
}

const acl_leak_secret = "비밀 문서"

// TestLandDoesNotLeakRestrictedDocument : 땅따먹기 지도에 비공개 문서가 끼지 않는가
func TestLandDoesNotLeakRestrictedDocument(t *testing.T) {
	db := acl_leak_db(t)
	defer db.Close()

	// 비밀 문서는 글자 수가 가장 많아 거르지 않으면 지도 맨 앞에 온다
	owners, hidden := baro_land_owner_list(db, tool.Config{IP: "1.2.3.4"}, 900)

	for _, one := range owners {
		if one.Title == acl_leak_secret {
			t.Fatal("땅따먹기 지도에 읽을 수 없는 문서가 나왔다")
		}
	}
	if len(owners) == 0 {
		t.Fatal("공개 문서까지 사라졌다")
	}
	if hidden < 1 {
		t.Fatalf("걸러 낸 문서 수를 알려 주지 않았다 (hidden=%d)", hidden)
	}

	// 관리자에게는 보여야 한다 (지나치게 막아도 버그다)
	tool.Exec_DB(db, "insert into user_set (name, id, data) values ('acl', 'boss', 'owner')")
	admin_owners, _ := baro_land_owner_list(db, tool.Config{IP: "boss"}, 900)

	found := false
	for _, one := range admin_owners {
		if one.Title == acl_leak_secret {
			found = true
		}
	}
	if !found {
		t.Fatal("관리자에게도 비공개 문서가 안 보인다")
	}
}

// TestGameDoesNotLeakRestrictedDocument : 바로게임이 비공개 문서를 쓰지 않는가
func TestGameDoesNotLeakRestrictedDocument(t *testing.T) {
	db := acl_leak_db(t)
	defer db.Close()

	config := tool.Config{IP: "1.2.3.4"}

	// 여러 번 뽑아도 한 번도 끼면 안 된다 (섞기 때문에 한 번으로는 모자라다)
	for round := 0; round < 30; round++ {
		for _, title := range baro_game_random_titles(db, config, 60) {
			if title == acl_leak_secret {
				t.Fatal("문서 맞히기 후보에 읽을 수 없는 문서가 끼었다")
			}
		}
	}

	// 레이스에서 밟을 수 있는 링크에도 끼면 안 된다
	filter := tool.New_acl_render_filter(db, config.IP)
	links := baro_game_out_links(db, filter, "공개 문서 하나")

	if len(links) == 0 {
		t.Fatal("공개 링크까지 사라졌다")
	}
	for _, one := range links {
		if one == acl_leak_secret {
			t.Fatal("레이스 링크에 읽을 수 없는 문서가 끼었다")
		}
	}
}

// TestGameSnippetNeverShowsSecretBody : 문제 본문에 비밀 문서 내용이 섞이지 않는가
func TestGameSnippetNeverShowsSecretBody(t *testing.T) {
	db := acl_leak_db(t)
	defer db.Close()

	config := tool.Config{IP: "1.2.3.4"}

	for round := 0; round < 30; round++ {
		state := baro_game_quiz_make(db, config, 0)
		if state["answer"] == acl_leak_secret {
			t.Fatal("정답이 읽을 수 없는 문서였다")
		}
		if strings.Contains(state["snippet"], "비밀낱말") {
			t.Fatalf("문제 본문에 비공개 문서 내용이 새어 나왔다 : %s", state["snippet"])
		}
		for _, one := range strings.Split(state["choices"], "|") {
			if one == acl_leak_secret {
				t.Fatal("보기에 읽을 수 없는 문서가 끼었다")
			}
		}
	}
}
