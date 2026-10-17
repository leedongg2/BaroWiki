package tool

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// acl_rule_test_db : ACL 규칙이 부르는 표만 있는 작은 위키
func acl_rule_test_db(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "acl.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	for _, query := range []string{
		"create table baro_acl_group (id text default '', group_name text default '', kind text default '', target text default '', note text default '', actor text default '', date text default '', end_date text default '')",
		"create table baro_geoip (id text default '', start_num text default '', end_num text default '', country text default '', area text default '', note text default '', actor text default '', date text default '')",
		"create table user_set (name text default '', id text default '', data text default '')",
		"create table history (id text default '', title text default '', data text default '', date text default '', ip text default '', send text default '', leng text default '', hide text default '', type text default '')",
	} {
		Exec_DB(db, query)
	}

	return db
}

// TestAclRuleParse : 규칙 읽기
func TestAclRuleParse(t *testing.T) {
	cases := []struct {
		source string
		allow  bool
		kind   string
		value  string
	}{
		{"허용:perm:admin", true, "perm", "admin"},
		{"거부:any", false, "any", ""},
		{"allow:member", true, "member", ""},
		{"deny:ip:192.168.0.0/16", false, "ip", "192.168.0.0/16"},
		{"  허용 : user : 홍길동  ", true, "user", "홍길동"},
		{"허용:geoip:KR,JP", true, "geoip", "KR,JP"},
		{"거부:aclgroup:장난 대역", false, "aclgroup", "장난 대역"},

		// 옛 값 : 허용·거부를 안 적으면 허용으로 본다
		{"admin", true, "admin", ""},
		{"50_edit", true, "50_edit", ""},
		{"all", true, "any", ""}, // all 은 any 로 맞춘다
	}

	for _, one := range cases {
		rule, ok := Acl_rule_parse_line(one.source)
		if !ok {
			t.Errorf("%q : 읽지 못했다", one.source)

			continue
		}
		if rule.Allow != one.allow || rule.Kind != one.kind || rule.Value != one.value {
			t.Errorf("%q : {%v %q %q} 를 바랐는데 {%v %q %q}",
				one.source, one.allow, one.kind, one.value, rule.Allow, rule.Kind, rule.Value)
		}
	}

	// 빈 줄은 규칙이 아니다
	for _, one := range []string{"", "   ", "\t", "허용:", "거부:  "} {
		if _, ok := Acl_rule_parse_line(one); ok {
			t.Errorf("%q : 규칙이 아닌데 읽었다", one)
		}
	}

	// 여러 줄
	rules := Acl_rule_parse("허용:perm:admin\r\n거부:any\n\n")
	if len(rules) != 2 {
		t.Fatalf("2줄이어야 하는데 %d줄 : %v", len(rules), rules)
	}

	// 저장 글로 되돌리기
	if got := Acl_rule_join(rules); got != "허용:perm:admin\n거부:any" {
		t.Fatalf("되돌린 글이 다르다 : %q", got)
	}
}

// TestAclRuleOrder : 위에서부터 훑어 처음 걸린 줄이 결론인가
func TestAclRuleOrder(t *testing.T) {
	db := acl_rule_test_db(t)

	admin := Acl_rule_context{IP: "운영자", Auth: map[string]bool{"treat_as_admin": true}}
	member := Acl_rule_context{IP: "사람", Auth: map[string]bool{}}
	guest := Acl_rule_context{IP: "1.2.3.4", IsIP: true, Auth: map[string]bool{}}

	rules := Acl_rule_parse("허용:perm:admin\n거부:ip:1.2.3.0/24\n허용:member\n거부:any")

	if Acl_rule_check(db, rules, admin) != Acl_rule_allow {
		t.Fatalf("관리자가 막혔다")
	}
	if Acl_rule_check(db, rules, guest) != Acl_rule_deny {
		t.Fatalf("막아야 할 대역이 통과했다")
	}
	if Acl_rule_check(db, rules, member) != Acl_rule_allow {
		t.Fatalf("로그인 사용자가 막혔다")
	}

	// 차례가 바뀌면 결과도 바뀐다 (거부가 위로 가면 관리자도 막힌다)
	flipped := Acl_rule_parse("거부:any\n허용:perm:admin")
	if Acl_rule_check(db, flipped, admin) != Acl_rule_deny {
		t.Fatalf("위의 거부가 먼저 걸려야 한다")
	}

	// 아무 줄도 안 걸리면 none (부르는 쪽이 다음 자리로 넘긴다)
	if Acl_rule_check(db, Acl_rule_parse("허용:perm:owner"), member) != Acl_rule_none {
		t.Fatalf("걸린 줄이 없는데 none 이 아니다")
	}
	if Acl_rule_check(db, nil, member) != Acl_rule_none {
		t.Fatalf("규칙이 없는데 none 이 아니다")
	}
}

// TestAclRuleUserAndIP : user · ip 조건
func TestAclRuleUserAndIP(t *testing.T) {
	db := acl_rule_test_db(t)

	hong := Acl_rule_context{IP: "홍길동", Auth: map[string]bool{}}
	kim := Acl_rule_context{IP: "김철수", Auth: map[string]bool{}}

	rules := Acl_rule_parse("허용:user:홍길동\n거부:any")
	if Acl_rule_check(db, rules, hong) != Acl_rule_allow {
		t.Fatalf("지목한 계정이 막혔다")
	}
	if Acl_rule_check(db, rules, kim) != Acl_rule_deny {
		t.Fatalf("다른 계정이 통과했다")
	}

	// 계정 이름은 대소문자를 가리지 않는다
	if Acl_rule_check(db, Acl_rule_parse("허용:user:HongGilDong"),
		Acl_rule_context{IP: "honggildong", Auth: map[string]bool{}}) != Acl_rule_allow {
		t.Fatalf("대소문자만 다른데 못 찾았다")
	}

	// 아이피 하나와 대역
	cases := map[string]bool{
		"192.168.0.1":    true,
		"192.168.0.2":    false,
		"192.168.99.7":   false,
		"10.0.0.1":       false,
	}
	one_rule := Acl_rule_parse("허용:ip:192.168.0.1\n거부:any")
	for ip, want := range cases {
		got := Acl_rule_check(db, one_rule, Acl_rule_context{IP: ip, IsIP: true, Auth: map[string]bool{}}) == Acl_rule_allow
		if got != want {
			t.Errorf("ip:192.168.0.1 / %s : %v 를 바랐는데 %v", ip, want, got)
		}
	}

	band_rule := Acl_rule_parse("허용:ip:192.168.0.0/16\n거부:any")
	for ip, want := range map[string]bool{
		"192.168.0.1":  true,
		"192.168.99.7": true,
		"192.169.0.1":  false,
		"10.0.0.1":     false,
	} {
		got := Acl_rule_check(db, band_rule, Acl_rule_context{IP: ip, IsIP: true, Auth: map[string]bool{}}) == Acl_rule_allow
		if got != want {
			t.Errorf("ip:192.168.0.0/16 / %s : %v 를 바랐는데 %v", ip, want, got)
		}
	}

	// 계정 이름은 대역에 들지 않는다
	if Acl_rule_check(db, band_rule, Acl_rule_context{IP: "홍길동", Auth: map[string]bool{}}) != Acl_rule_deny {
		t.Fatalf("계정 이름이 아이피 대역에 들어갔다")
	}
}

// TestAclRuleAclGroup : aclgroup 조건
func TestAclRuleAclGroup(t *testing.T) {
	db := acl_rule_test_db(t)

	Acl_group_add(db, "장난 대역", "user", "말썽", "", "운영자", "")
	Acl_group_add(db, "장난 대역", "ip", "203.0.113.0/24", "", "운영자", "")
	Acl_group_add(db, "지난 것", "user", "옛사람", "", "운영자", "2000-01-01 00:00:00")

	rules := Acl_rule_parse("거부:aclgroup:장난 대역\n허용:any")

	deny := []string{"말썽", "203.0.113.9"}
	for _, one := range deny {
		if Acl_rule_check(db, rules, Acl_rule_context{IP: one, IsIP: strings.Contains(one, "."), Auth: map[string]bool{}}) != Acl_rule_deny {
			t.Errorf("%q : 그룹에 들었는데 통과했다", one)
		}
	}

	allow := []string{"착한이", "198.51.100.1"}
	for _, one := range allow {
		if Acl_rule_check(db, rules, Acl_rule_context{IP: one, IsIP: strings.Contains(one, "."), Auth: map[string]bool{}}) != Acl_rule_allow {
			t.Errorf("%q : 그룹에 없는데 막혔다", one)
		}
	}

	// 만료된 항목은 세지 않는다
	if Acl_group_has(db, "지난 것", "옛사람") {
		t.Fatalf("만료된 항목이 아직 걸린다")
	}

	// 없는 그룹은 아무도 안 든다
	if Acl_group_has(db, "없는 그룹", "말썽") {
		t.Fatalf("없는 그룹에 누군가 들었다")
	}
}

// TestAclRuleGeoip : geoip 조건
func TestAclRuleGeoip(t *testing.T) {
	db := acl_rule_test_db(t)
	Acl_geoip_cache_clear()

	if !Acl_geoip_range_add(db, "203.0.113.0/24", "KR", "시험", "운영자") {
		t.Fatalf("대역을 넣지 못했다")
	}
	if !Acl_geoip_range_add(db, "198.51.100.0-198.51.100.255", "JP", "시험", "운영자") {
		t.Fatalf("범위 모양을 넣지 못했다")
	}

	for ip, want := range map[string]string{
		"203.0.113.7":   "KR",
		"198.51.100.20": "JP",
		"192.0.2.1":     "",
	} {
		country, ok := Acl_geoip_country(db, ip)
		if want == "" {
			if ok {
				t.Errorf("%s : 모르는 대역인데 %q 가 나왔다", ip, country)
			}

			continue
		}
		if !ok || country != want {
			t.Errorf("%s : %q 를 바랐는데 %q(%v)", ip, want, country, ok)
		}
	}

	// 규칙 판정
	rules := Acl_rule_parse("거부:geoip:KR\n허용:any")

	kr := Acl_rule_context{IP: "203.0.113.7", IsIP: true, Auth: map[string]bool{}, Country: "KR", CountryOK: true}
	if Acl_rule_check(db, rules, kr) != Acl_rule_deny {
		t.Fatalf("한국을 막지 못했다")
	}

	jp := Acl_rule_context{IP: "198.51.100.20", IsIP: true, Auth: map[string]bool{}, Country: "JP", CountryOK: true}
	if Acl_rule_check(db, rules, jp) != Acl_rule_allow {
		t.Fatalf("일본이 막혔다")
	}

	// 나라를 모르면 geoip 규칙은 걸리지 않는다 (허용도 거부도 아님)
	unknown := Acl_rule_context{IP: "192.0.2.1", IsIP: true, Auth: map[string]bool{}}
	if Acl_rule_check(db, Acl_rule_parse("거부:geoip:KR"), unknown) != Acl_rule_none {
		t.Fatalf("나라를 모르는데 규칙이 걸렸다")
	}

	// 여러 나라를 쉼표로
	many := Acl_rule_parse("거부:geoip:CN,RU,KR\n허용:any")
	if Acl_rule_check(db, many, kr) != Acl_rule_deny {
		t.Fatalf("쉼표로 이어 적은 나라를 못 읽었다")
	}

	// 앞단이 알려 준 나라가 대역표보다 먼저다
	Acl_geoip_remember("192.0.2.1", "US")
	if country, ok := Acl_geoip_country(db, "192.0.2.1"); !ok || country != "US" {
		t.Fatalf("앞단이 알려 준 나라를 쓰지 않았다 : %q(%v)", country, ok)
	}
}

// TestAclGeoipRangeParse : 대역 적는 모양
func TestAclGeoipRangeParse(t *testing.T) {
	good := map[string][2]string{
		"1.2.3.0/24":          {"16909056", "16909311"},
		"1.2.3.4":             {"16909060", "16909060"},
		"1.2.3.0-1.2.3.255":   {"16909056", "16909311"},
	}
	for area, want := range good {
		start, end, ok := Acl_geoip_range_parse(area)
		if !ok {
			t.Errorf("%q : 읽지 못했다", area)

			continue
		}
		if Baro_itoa(int(start)) != want[0] || Baro_itoa(int(end)) != want[1] {
			t.Errorf("%q : %v 를 바랐는데 %d~%d", area, want, start, end)
		}
	}

	bad := []string{"", "어쩌구", "1.2.3.4/33", "2001:db8::/32", "1.2.3.9-1.2.3.1", "1.2.3"}
	for _, area := range bad {
		if _, _, ok := Acl_geoip_range_parse(area); ok {
			t.Errorf("%q : 잘못된 대역인데 읽혔다", area)
		}
	}
}

// TestAclRulePerm : perm 조건
func TestAclRulePerm(t *testing.T) {
	db := acl_rule_test_db(t)

	admin := Acl_rule_context{IP: "운영자", Auth: map[string]bool{"treat_as_admin": true, "acl": true}}
	plain := Acl_rule_context{IP: "사람", Auth: map[string]bool{}}

	if Acl_rule_check(db, Acl_rule_parse("허용:perm:admin"), admin) != Acl_rule_allow {
		t.Fatalf("perm:admin 이 안 걸린다")
	}
	if Acl_rule_check(db, Acl_rule_parse("허용:perm:acl"), admin) != Acl_rule_allow {
		t.Fatalf("perm:acl 이 안 걸린다")
	}
	if Acl_rule_check(db, Acl_rule_parse("허용:perm:admin"), plain) != Acl_rule_none {
		t.Fatalf("권한이 없는데 걸렸다")
	}
	if Acl_rule_check(db, Acl_rule_parse("허용:perm:"), admin) != Acl_rule_none {
		t.Fatalf("권한 이름이 비었는데 걸렸다")
	}
}
