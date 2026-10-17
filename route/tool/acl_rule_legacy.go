package tool

import (
	"database/sql"
	"time"
)

// ============================================================
//  옛 ACL 조건
//
//  규칙 방식으로 바뀌기 전에 쓰던 조건들이다.
//  이미 저장된 ACL 이 이 값을 쓰고 있으므로 그대로 판정해 준다.
//  새 규칙에서도 갈래 이름으로 그냥 적을 수 있다. (`허용:50_edit`)
// ============================================================

// acl_rule_legacy : 옛 조건 하나를 판정한다
func acl_rule_legacy(db *sql.DB, kind string, context Acl_rule_context) bool {
	switch kind {
	case "all":
		return true

	case "admin":
		return context.Auth["treat_as_admin"]

	case "owner":
		return context.Auth["owner"]

	case "ban_admin":
		// 차단 여부는 부르는 쪽에서 이미 가려 냈다
		return context.Auth["treat_as_admin"]

	case "email":
		if context.IsIP {
			return false
		}

		exist := ""
		QueryRow_DB(db, "select data from user_set where id = ? and name = 'email'", []any{&exist}, context.IP)

		return exist != ""

	case "before":
		exist := ""
		QueryRow_DB(
			db,
			"select ip from history where title = ? and ip = ? and type != 'edit_request'",
			[]any{&exist},
			context.DocName, context.IP,
		)

		return exist != ""

	case "50_edit":
		return acl_rule_edit_count(db, context) >= 50

	case "30_day":
		return acl_rule_signup_passed(db, context, 30)

	case "90_day":
		return acl_rule_signup_passed(db, context, 90)

	case "30_day_50_edit":
		return acl_rule_signup_passed(db, context, 30) && acl_rule_edit_count(db, context) >= 50

	case "up_to_level_3":
		return context.Level >= 3

	case "up_to_level_10":
		return context.Level >= 10
	}

	return false
}

// acl_rule_edit_count : 편집 횟수
func acl_rule_edit_count(db *sql.DB, context Acl_rule_context) int {
	if context.IsIP {
		return 0
	}

	count := 0
	QueryRow_DB(db, "select count(*) from history where ip = ?", []any{&count}, context.IP)

	return count
}

// acl_rule_signup_passed : 가입한 지 정해진 날수가 지났는가
func acl_rule_signup_passed(db *sql.DB, context Acl_rule_context, days int) bool {
	if context.IsIP {
		return false
	}

	signup_date := Get_time()
	QueryRow_DB(db, "select data from user_set where id = ? and name = 'date'", []any{&signup_date}, context.IP)

	signup, err := time.Parse("2006-01-02 15:04:05", signup_date)
	if err != nil {
		return false
	}

	return time.Now().After(signup.AddDate(0, 0, days))
}
