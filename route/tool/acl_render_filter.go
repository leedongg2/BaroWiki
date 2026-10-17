package tool

import (
	"database/sql"
)

// ============================================================
//  목록 화면에서 읽기 권한 빠르게 거르기
//
//  문서 목록을 보여 주는 화면은 읽을 수 없는 문서를 빼야 한다.
//  그런데 Check_acl 한 번이 0.5ms 쯤 들어서 900개를 그냥 부르면 0.5초가 날아간다.
//
//  render 권한은 다음 세 곳만 본다. (acl_and_auth.go 의 render 갈래)
//    1. acl 표의 (문서, 'view') 줄
//    2. 이름공간 ACL 의 'view'
//    3. 이름공간 기본값의 'view'
//
//  그래서 (1) 이 없는 문서라면 답이 **이름공간마다 하나로 정해진다.**
//  (1) 이 있는 문서만 따로 물어보고, 나머지는 이름공간별로 한 번만 물어본다.
//  결과는 Check_acl 을 그대로 부르는 것과 완전히 같고, 부르는 횟수만 줄어든다.
// ============================================================

// Acl_render_filter : 목록용 읽기 권한 거르개
type Acl_render_filter struct {
	db    *sql.DB
	ip    string
	named map[string]bool // 문서별 acl 줄이 있는 제목 (이름공간으로 뭉뚱그릴 수 없다)
	cache map[string]bool // 이름공간 접두사 -> 읽을 수 있는가
}

// New_acl_render_filter : 거르개 만들기 (acl 표를 한 번만 읽는다)
func New_acl_render_filter(db *sql.DB, ip string) *Acl_render_filter {
	filter := &Acl_render_filter{
		db:    db,
		ip:    ip,
		named: map[string]bool{},
		cache: map[string]bool{},
	}

	// 읽기 권한을 문서별로 따로 정해 둔 문서 목록.
	// 만료된 줄도 함께 잡히지만, 그런 문서는 그냥 하나씩 물어보므로 답은 정확하다.
	rows := Query_DB(db, "select distinct title from acl where type = 'view'")
	for rows.Next() {
		title := ""
		if rows.Scan(&title) != nil {
			continue
		}
		if title == "" {
			continue
		}

		filter.named[title] = true
	}
	rows.Close()

	return filter
}

// Allow : 이 문서를 읽을 수 있는가
func (filter *Acl_render_filter) Allow(title string) bool {
	if filter == nil || title == "" {
		return false
	}

	// 문서별로 권한을 정해 둔 문서는 뭉뚱그릴 수 없다
	if filter.named[title] {
		return Check_acl(filter.db, title, "", "render", filter.ip)
	}

	key := Namespace_of(title).Prefix
	if answer, found := filter.cache[key]; found {
		return answer
	}

	answer := Check_acl(filter.db, title, "", "render", filter.ip)
	filter.cache[key] = answer

	return answer
}

// Count : 실제로 Check_acl 을 부른 횟수 (시험용)
func (filter *Acl_render_filter) Count() int {
	if filter == nil {
		return 0
	}

	return len(filter.cache) + len(filter.named)
}
