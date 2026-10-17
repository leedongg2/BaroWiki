package tool

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================
//  고유 번호 시험
//
//  여기서 지키는 것은 세 가지다.
//    1. 번호 모양이 RFC 4122 판 4 그대로일 것
//    2. 같은 판 · 같은 계정에는 언제 물어도 같은 번호가 나올 것
//    3. 바깥에서 들어온 값을 번호로 착각하지 않을 것
// ============================================================

// baro_uuid_test_db : 고유 번호 표만 있는 작은 위키
func baro_uuid_test_db(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "uuid.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	for _, query := range []string{
		"create table baro_rev_uuid (uuid text default '', title text default '', rev text default '', date text default '')",
		"create table baro_user_set (user text default '', name text default '', data text default '')",
		"create table baro_user_name_log (id text default '', uuid text default '', user text default '', old_name text default '', new_name text default '', actor text default '', date text default '')",
	} {
		Exec_DB(db, query)
	}

	return db
}

// TestBaroUuidShape : 번호 모양이 판 4 인가
func TestBaroUuidShape(t *testing.T) {
	seen := map[string]bool{}

	for i := 0; i < 500; i++ {
		value := Baro_uuid_new()

		if !Baro_uuid_is(value) {
			t.Fatalf("만든 번호를 스스로 못 알아본다 : %q", value)
		}
		if value != strings.ToLower(value) {
			t.Fatalf("번호는 소문자여야 한다 : %q", value)
		}
		if value[14] != '4' {
			t.Fatalf("판 4 가 아니다 : %q", value)
		}
		if one := value[19]; one != '8' && one != '9' && one != 'a' && one != 'b' {
			t.Fatalf("RFC 4122 변형이 아니다 : %q", value)
		}
		if seen[value] {
			t.Fatalf("번호가 겹쳤다 : %q", value)
		}

		seen[value] = true
	}
}

// TestBaroUuidIs : 번호가 아닌 것을 번호로 보지 않는가
func TestBaroUuidIs(t *testing.T) {
	good := []string{
		"49270f19-6a14-4ee6-84de-59c29bd33700",
		"49270F19-6A14-4EE6-84DE-59C29BD33700",
		"00000000-0000-0000-0000-000000000000",
	}
	for _, one := range good {
		if !Baro_uuid_is(one) {
			t.Fatalf("%q 는 번호인데 아니라고 한다", one)
		}
	}

	bad := []string{
		"", "3", "r3", "FrontPage", "legacy-rev-3",
		"49270f19-6a14-4ee6-84de-59c29bd3370",   // 한 글자 짧다
		"49270f19-6a14-4ee6-84de-59c29bd337000", // 한 글자 길다
		"49270f19_6a14_4ee6_84de_59c29bd33700",  // 구분자가 다르다
		"49270f19-6a14-4ee6-84de-59c29bd3370g",  // 16진수가 아니다
		"49270f196a144ee684de59c29bd33700",      // 구분자가 없다
		"'; drop table baro_rev_uuid; --",
	}
	for _, one := range bad {
		if Baro_uuid_is(one) {
			t.Fatalf("%q 는 번호가 아닌데 번호라고 한다", one)
		}
	}
}

// TestBaroRevUuidStable : 같은 판에는 언제 물어도 같은 번호가 나오는가
func TestBaroRevUuidStable(t *testing.T) {
	db := baro_uuid_test_db(t)

	first := Baro_rev_uuid(db, "문서 하나", "1")
	if !Baro_uuid_is(first) {
		t.Fatalf("판 고유 번호를 못 만들었다 : %q", first)
	}

	// 몇 번을 물어도 같아야 한다
	for i := 0; i < 5; i++ {
		if again := Baro_rev_uuid(db, "문서 하나", "1"); again != first {
			t.Fatalf("같은 판인데 번호가 달라졌다 : %q → %q", first, again)
		}
	}

	// 판이 다르면 번호도 달라야 한다
	second := Baro_rev_uuid(db, "문서 하나", "2")
	if second == first {
		t.Fatalf("다른 판인데 번호가 같다 : %q", second)
	}

	// 문서가 다르면 판 번호가 같아도 달라야 한다
	other := Baro_rev_uuid(db, "문서 둘", "1")
	if other == first {
		t.Fatalf("다른 문서인데 번호가 같다 : %q", other)
	}

	// 줄 수를 세어 덧붙이기만 했는지 본다 (물을 때마다 쌓이면 안 된다)
	count := 0
	QueryRow_DB(db, "select count(*) from baro_rev_uuid", []any{&count})
	if count != 3 {
		t.Fatalf("번호가 3개여야 하는데 %d개다", count)
	}
}

// TestBaroRevResolve : 판 자리에 온 번호를 판 번호로 바꾸는가
func TestBaroRevResolve(t *testing.T) {
	db := baro_uuid_test_db(t)

	value := Baro_rev_uuid(db, "문서 하나", "7")

	if got := Baro_rev_resolve(db, "문서 하나", value); got != "7" {
		t.Fatalf("번호를 판으로 못 바꿨다 : %q", got)
	}

	// 문서를 몰라도 번호만으로 찾는다
	if got := Baro_rev_resolve(db, "", value); got != "7" {
		t.Fatalf("문서 없이 못 찾았다 : %q", got)
	}

	// 번호가 아니면 받은 값을 그대로 돌려줘야 기존 주소가 그대로 돈다
	for _, one := range []string{"7", "r7", "", "FrontPage"} {
		if got := Baro_rev_resolve(db, "문서 하나", one); got != one {
			t.Fatalf("%q 를 건드렸다 : %q", one, got)
		}
	}

	// 모르는 번호는 없는 판으로 다뤄지도록 그대로 넘긴다
	unknown := Baro_uuid_new()
	if got := Baro_rev_resolve(db, "문서 하나", unknown); got != unknown {
		t.Fatalf("모르는 번호를 멋대로 바꿨다 : %q", got)
	}

	// 다른 문서의 번호로는 이 문서의 판을 못 가져온다
	Baro_rev_uuid(db, "문서 둘", "99")
	two := Baro_rev_uuid(db, "문서 둘", "99")
	if got := Baro_rev_resolve(db, "문서 하나", two); got != "99" {
		// 문서를 못 찾으면 번호만으로 찾는 것이 규칙이라 99 가 맞다
		t.Fatalf("번호만으로 찾는 갈래가 깨졌다 : %q", got)
	}
}

// TestBaroRevUuidList : 여러 판을 한 번에 물어도 하나씩 물은 것과 같은가
func TestBaroRevUuidList(t *testing.T) {
	db := baro_uuid_test_db(t)

	// 절반은 미리 번호를 받아 두고, 절반은 목록에서 처음 받는다
	want := map[string]string{}
	pairs := [][2]string{}
	for i := 1; i <= 10; i++ {
		rev := Baro_itoa(i)
		pairs = append(pairs, [2]string{"문서 하나", rev})

		if i%2 == 0 {
			want["문서 하나\n"+rev] = Baro_rev_uuid(db, "문서 하나", rev)
		}
	}
	pairs = append(pairs, [2]string{"문서 둘", "1"})

	got := Baro_rev_uuid_list(db, pairs)
	if len(got) != 11 {
		t.Fatalf("11개를 바랐는데 %d개다", len(got))
	}

	for key, value := range want {
		if got[key] != value {
			t.Fatalf("%q : %q 를 바랐는데 %q", key, value, got[key])
		}
	}

	// 목록에서 새로 붙인 번호도 그대로 굳어야 한다
	for key, value := range got {
		if !Baro_uuid_is(value) {
			t.Fatalf("%q 의 번호가 이상하다 : %q", key, value)
		}

		parts := strings.SplitN(key, "\n", 2)
		if again := Baro_rev_uuid(db, parts[0], parts[1]); again != value {
			t.Fatalf("%q : 목록은 %q 인데 하나씩 물으면 %q", key, value, again)
		}
	}

	// 다른 문서의 같은 판 번호를 섞어 오지 않았는가
	if got["문서 둘\n1"] == got["문서 하나\n1"] {
		t.Fatalf("문서가 다른데 번호가 같다")
	}

	// 빈 목록은 빈 map
	if len(Baro_rev_uuid_list(db, nil)) != 0 {
		t.Fatalf("빈 목록에서 뭔가 나왔다")
	}
}

// TestBaroUserUuid : 계정 고유 번호가 굳어 있고 되짚을 수 있는가
func TestBaroUserUuid(t *testing.T) {
	db := baro_uuid_test_db(t)

	value := Baro_user_uuid(db, "홍길동")
	if !Baro_uuid_is(value) {
		t.Fatalf("계정 고유 번호를 못 만들었다 : %q", value)
	}
	if again := Baro_user_uuid(db, "홍길동"); again != value {
		t.Fatalf("같은 계정인데 번호가 달라졌다 : %q → %q", value, again)
	}
	if other := Baro_user_uuid(db, "김철수"); other == value {
		t.Fatalf("다른 계정인데 번호가 같다")
	}

	if got := Baro_user_from_uuid(db, value); got != "홍길동" {
		t.Fatalf("번호로 계정을 못 찾았다 : %q", got)
	}
	if got := Baro_user_from_uuid(db, strings.ToUpper(value)); got != "홍길동" {
		t.Fatalf("대문자 번호로 못 찾았다 : %q", got)
	}
	if got := Baro_user_from_uuid(db, "홍길동"); got != "" {
		t.Fatalf("번호가 아닌데 찾아 줬다 : %q", got)
	}

	// 비로그인 아이피는 계정이 아니므로 번호를 주지 않는다
	for _, one := range []string{"1.2.3.4", "2001:db8::1", ""} {
		if got := Baro_user_uuid(db, one); got != "" {
			t.Fatalf("%q 에 번호를 줬다 : %q", one, got)
		}
	}
}

// TestBaroUserNameLog : 이름을 바꾼 기록이 고유 번호에 매달리는가
func TestBaroUserNameLog(t *testing.T) {
	db := baro_uuid_test_db(t)

	user_uuid := Baro_user_uuid(db, "계정")

	Baro_user_name_log_add(db, "계정", "첫이름", "둘째이름", "계정")
	Baro_user_name_log_add(db, "계정", "둘째이름", "셋째이름", "운영자")

	// 바뀐 것이 없으면 남기지 않는다
	Baro_user_name_log_add(db, "계정", "셋째이름", "셋째이름", "계정")

	rows := Query_DB(db, "select uuid, old_name, new_name, actor from baro_user_name_log where uuid = ? order by id + 0 asc", user_uuid)
	defer rows.Close()

	list := [][]string{}
	for rows.Next() {
		a, b, c, d := "", "", "", ""
		if rows.Scan(&a, &b, &c, &d) != nil {
			t.Fatal("기록을 못 읽었다")
		}

		list = append(list, []string{a, b, c, d})
	}

	if len(list) != 2 {
		t.Fatalf("기록이 2줄이어야 하는데 %d줄이다", len(list))
	}
	if list[0][1] != "첫이름" || list[0][2] != "둘째이름" || list[0][3] != "계정" {
		t.Fatalf("첫 줄이 다르다 : %v", list[0])
	}
	if list[1][1] != "둘째이름" || list[1][2] != "셋째이름" || list[1][3] != "운영자" {
		t.Fatalf("둘째 줄이 다르다 : %v", list[1])
	}
	if list[0][0] != user_uuid || list[1][0] != user_uuid {
		t.Fatalf("기록이 계정 고유 번호에 안 매달렸다 : %v", list)
	}
}
