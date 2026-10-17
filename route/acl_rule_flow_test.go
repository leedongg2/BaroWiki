package route

import (
	"testing"

	"opennamu/route/tool"
)

// ============================================================
//  ACL 규칙이 실제 위키에서 끝까지 먹히는가
//
//  규칙 엔진만 따로 시험하면 「Check_acl 이 그 규칙을 정말 쓰는가」 는 알 수 없다.
//  그래서 표가 다 있는 위키를 세워 두고 acl 표에 규칙을 직접 넣어 본다.
// ============================================================

// TestAclRuleInCheckAcl : 문서 ACL 에 넣은 규칙이 Check_acl 에 먹히는가
func TestAclRuleInCheckAcl(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "운영자", "owner", "2026-01-01 00:00:00")
	baro_flow_user(db, "착한이", "user", "2026-01-01 00:00:00")
	baro_flow_user(db, "말썽", "user", "2026-01-01 00:00:00")

	Api_edit_post(tool.Config{IP: "운영자"}, "규칙문서", "본문입니다.", "", "yes", "")

	set_rule := func(rule string) {
		tool.Exec_DB(db, "delete from acl where title = ? and type = 'document_edit_acl'", "규칙문서")
		tool.Exec_DB(db, "insert into acl (title, data, type) values (?, ?, 'document_edit_acl')", "규칙문서", rule)
	}
	can_edit := func(who string) bool {
		return tool.Check_acl(db, "규칙문서", "", "document_edit", who)
	}

	// 특정 계정만 막기 (나머지는 그대로 편집)
	set_rule("거부:user:말썽\n허용:any")
	if can_edit("말썽") {
		t.Fatalf("지목해 막은 계정이 편집됐다")
	}
	if !can_edit("착한이") {
		t.Fatalf("막지 않은 계정이 막혔다")
	}

	// 특정 계정만 허용
	set_rule("허용:user:착한이\n거부:any")
	if !can_edit("착한이") {
		t.Fatalf("허용한 계정이 막혔다")
	}
	if can_edit("말썽") {
		t.Fatalf("허용하지 않은 계정이 편집됐다")
	}

	// 아이피 대역 막기
	set_rule("거부:ip:203.0.113.0/24\n허용:any")
	if can_edit("203.0.113.9") {
		t.Fatalf("막은 대역이 편집됐다")
	}
	if !can_edit("198.51.100.1") {
		t.Fatalf("막지 않은 대역이 막혔다")
	}

	// ACL 그룹
	tool.Acl_group_add(db, "장난 대역", "ip", "203.0.113.0/24", "", "운영자", "")
	tool.Acl_group_add(db, "장난 대역", "user", "말썽", "", "운영자", "")
	set_rule("거부:aclgroup:장난 대역\n허용:any")
	if can_edit("203.0.113.9") || can_edit("말썽") {
		t.Fatalf("ACL 그룹에 든 대상이 편집됐다")
	}
	if !can_edit("착한이") {
		t.Fatalf("그룹에 없는 사람이 막혔다")
	}

	// 나라
	tool.Acl_geoip_cache_clear()
	tool.Acl_geoip_range_add(db, "203.0.113.0/24", "KR", "", "운영자")
	set_rule("거부:geoip:KR\n허용:any")
	if can_edit("203.0.113.9") {
		t.Fatalf("막은 나라에서 편집됐다")
	}
	if !can_edit("198.51.100.1") {
		t.Fatalf("나라를 모르는 곳이 막혔다")
	}

	// 권한
	set_rule("허용:perm:admin\n거부:any")
	if !can_edit("운영자") {
		t.Fatalf("관리자가 막혔다")
	}
	if can_edit("착한이") {
		t.Fatalf("권한이 없는데 편집됐다")
	}

	// 차례가 결론을 바꾼다
	//
	//	ACL 권한을 가진 사람(관리자)은 ACL 자체를 고칠 수 있어 규칙보다 먼저 지나간다.
	//	그래서 차례는 일반 사용자로 확인한다.
	set_rule("허용:member\n거부:any")
	if !can_edit("착한이") {
		t.Fatalf("허용이 위인데 막혔다")
	}

	set_rule("거부:any\n허용:member")
	if can_edit("착한이") {
		t.Fatalf("위의 거부가 먼저 걸려야 한다")
	}
}

// TestAclRuleLegacyStillWorks : 예전에 저장해 둔 한 줄짜리 값이 그대로 도는가
func TestAclRuleLegacyStillWorks(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "운영자", "owner", "2026-01-01 00:00:00")
	baro_flow_user(db, "사람", "user", "2026-01-01 00:00:00")

	Api_edit_post(tool.Config{IP: "운영자"}, "옛문서", "본문입니다.", "", "yes", "")

	set_rule := func(rule string) {
		tool.Exec_DB(db, "delete from acl where title = ? and type = 'document_edit_acl'", "옛문서")
		if rule != "" {
			tool.Exec_DB(db, "insert into acl (title, data, type) values (?, ?, 'document_edit_acl')", "옛문서", rule)
		}
	}
	can_edit := func(who string) bool {
		return tool.Check_acl(db, "옛문서", "", "document_edit", who)
	}

	// admin : 관리자만
	set_rule("admin")
	if !can_edit("운영자") {
		t.Fatalf("admin 인데 관리자가 막혔다")
	}
	if can_edit("사람") || can_edit("1.2.3.4") {
		t.Fatalf("admin 인데 다른 사람이 편집됐다")
	}

	// user : 로그인 사용자만
	set_rule("user")
	if !can_edit("사람") {
		t.Fatalf("user 인데 로그인 사용자가 막혔다")
	}
	if can_edit("1.2.3.4") {
		t.Fatalf("user 인데 비로그인이 편집됐다")
	}

	// all : 모두
	set_rule("all")
	if !can_edit("1.2.3.4") {
		t.Fatalf("all 인데 막혔다")
	}

	// not_all : 아무도 (관리자는 acl 권한으로 지나간다)
	set_rule("not_all")
	if can_edit("사람") {
		t.Fatalf("not_all 인데 편집됐다")
	}

	// 비워 두면 예전처럼 이름공간 쪽으로 넘어간다
	set_rule("")
	if !can_edit("사람") {
		t.Fatalf("설정이 없는데 막혔다")
	}
}
