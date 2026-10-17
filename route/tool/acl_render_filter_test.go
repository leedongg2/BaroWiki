package tool

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// acl_filter_test_db : 거르개 시험용 위키
func acl_filter_test_db(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "acl.db"))
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
		"create table data_set (doc_name text default '', doc_rev text default '', set_name text default '', set_data text default '')",
	} {
		Exec_DB(db, query)
	}

	Exec_DB(db, "insert into alist (name, acl) values ('ip', 'ip')")
	Exec_DB(db, "insert into alist (name, acl) values ('user', 'user')")
	Exec_DB(db, "insert into alist (name, acl) values ('owner', 'owner')")

	return db
}

// TestAclRenderFilterMatchesCheckAcl : 거르개 답이 Check_acl 과 완전히 같은가
//
//	빠르게 만든다고 답이 달라지면 그건 권한 구멍이다. 한 건이라도 다르면 실패다.
func TestAclRenderFilterMatchesCheckAcl(t *testing.T) {
	db := acl_filter_test_db(t)
	defer db.Close()

	titles := []string{
		"평범한 문서", "또 다른 평범한 문서", "세 번째 평범한 문서",
		"막힌 문서", "관리자만 문서", "회원만 문서",
		"user:someone", "user:someone/하위", "category:분류하나", "file:그림.png",
		"template:틀하나", "trash:지운 문서", "manage:운영 문서", "baro:바로 문서",
		"ipuser:1.2.3.4",
	}
	for _, one := range titles {
		Exec_DB(db, "insert into data (title, data) values (?, 'x')", one)
	}

	// 문서별 읽기 권한
	Exec_DB(db, "insert into acl (title, data, type) values ('막힌 문서', 'owner', 'view')")
	Exec_DB(db, "insert into acl (title, data, type) values ('관리자만 문서', 'admin', 'view')")
	Exec_DB(db, "insert into acl (title, data, type) values ('회원만 문서', 'user', 'view')")
	// 읽기와 상관없는 줄도 섞어 둔다 (거르개가 view 만 봐야 한다)
	Exec_DB(db, "insert into acl (title, data, type) values ('또 다른 평범한 문서', 'admin', 'document_edit_acl')")

	for _, ip := range []string{"127.0.0.1", "1.2.3.4"} {
		filter := New_acl_render_filter(db, ip)

		for _, title := range titles {
			want := Check_acl(db, title, "", "render", ip)
			got := filter.Allow(title)
			if want != got {
				t.Fatalf("%s / %s : Check_acl=%v 인데 거르개는 %v", ip, title, want, got)
			}
		}

		// 두 번째로 물어도 (캐시를 타도) 답이 같아야 한다
		for _, title := range titles {
			if Check_acl(db, title, "", "render", ip) != filter.Allow(title) {
				t.Fatalf("%s / %s : 캐시를 탄 뒤 답이 달라졌다", ip, title)
			}
		}
	}
}

// TestAclRenderFilterCallCount : 실제로 부르는 횟수가 줄었는가
func TestAclRenderFilterCallCount(t *testing.T) {
	db := acl_filter_test_db(t)
	defer db.Close()

	titles := []string{}
	for index := 0; index < 500; index++ {
		title := "문서" + Baro_itoa(index)
		titles = append(titles, title)
		Exec_DB(db, "insert into data (title, data) values (?, 'x')", title)
	}
	Exec_DB(db, "insert into acl (title, data, type) values ('문서7', 'owner', 'view')")

	filter := New_acl_render_filter(db, "127.0.0.1")
	for _, one := range titles {
		filter.Allow(one)
	}

	// 이름공간 하나 + 따로 정해 둔 문서 하나 = 2 이하여야 한다
	if filter.Count() > 3 {
		t.Fatalf("문서 500개인데 Check_acl 을 %d 종류나 불렀다", filter.Count())
	}
}
