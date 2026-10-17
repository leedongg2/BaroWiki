package tool

import (
	"database/sql"
	"net"
	"strings"

	"github.com/3th1nk/cidr"
)

// ============================================================
//  ACL 규칙 (나무위키 방식)
//
//  예전에는 항목마다 조건을 하나만 골랐다. (예: 편집 = 관리자)
//  그래서 「관리자와 특정 이용자만」 처럼 둘을 섞을 수가 없었다.
//
//  이제는 항목마다 규칙을 여러 줄 적고 위에서부터 훑는다.
//  처음 걸린 규칙이 곧 결론이고, 아무 규칙도 안 걸리면 거부한다.
//
//    허용:perm:admin
//    거부:ip:192.168.0.0/16
//    허용:user:홍길동
//    거부:any
//
//  규칙이 하나도 없으면 이름공간 ACL → 이름공간 기본값 순으로 넘어간다.
//  (예전과 같은 자리로 떨어진다)
//
//  저장은 acl 표의 data 칸에 줄바꿈으로 이어 붙여 둔다.
//  줄바꿈도 없고 `허용:`/`거부:` 도 없는 옛 값(`admin` 등)은
//  「허용:admin」 한 줄로 읽어 예전과 똑같이 움직인다.
// ============================================================

// Acl_rule : 규칙 한 줄
type Acl_rule struct {
	Allow bool   // true 면 허용, false 면 거부
	Kind  string // 조건 갈래 (any · member · perm · user · ip · aclgroup · geoip · 그 밖의 옛 조건)
	Value string // 갈래에 딸린 값 (perm:admin 의 admin)
}

// Acl_rule_result : 규칙을 훑은 결과
type Acl_rule_result int

const (
	// Acl_rule_none : 걸린 규칙이 없다 (다음 자리로 넘어간다)
	Acl_rule_none Acl_rule_result = iota
	// Acl_rule_allow : 허용
	Acl_rule_allow
	// Acl_rule_deny : 거부
	Acl_rule_deny
)

// Acl_rule_kind_list : 조건 갈래와 값이 필요한지
//
//	{갈래, 값이 필요한가}
func Acl_rule_kind_list() [][2]string {
	return [][2]string{
		{"any", ""},
		{"member", ""},
		{"perm", "값"},
		{"user", "값"},
		{"ip", "값"},
		{"aclgroup", "값"},
		{"geoip", "값"},
	}
}

// acl_rule_allow_word : 허용을 나타내는 말 (한국어와 영어를 모두 받는다)
var acl_rule_allow_word = map[string]bool{"허용": true, "allow": true}

// acl_rule_deny_word : 거부를 나타내는 말
var acl_rule_deny_word = map[string]bool{"거부": true, "deny": true}

// Acl_rule_parse : 저장된 값을 규칙 목록으로 읽는다
func Acl_rule_parse(data string) []Acl_rule {
	data = strings.ReplaceAll(data, "\r", "")
	if strings.TrimSpace(data) == "" {
		return nil
	}

	rules := []Acl_rule{}
	for _, line := range strings.Split(data, "\n") {
		if rule, ok := Acl_rule_parse_line(line); ok {
			rules = append(rules, rule)
		}
	}

	return rules
}

// Acl_rule_parse_line : 한 줄을 규칙 하나로
//
//	`허용:perm:admin` · `거부:any` · 옛 값 `admin`
func Acl_rule_parse_line(line string) (Acl_rule, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Acl_rule{}, false
	}

	rule := Acl_rule{Allow: true}

	head, rest, has_head := strings.Cut(line, ":")
	head_lower := strings.ToLower(strings.TrimSpace(head))

	if has_head && (acl_rule_allow_word[head_lower] || acl_rule_deny_word[head_lower]) {
		rule.Allow = acl_rule_allow_word[head_lower]
		line = strings.TrimSpace(rest)
	}

	if line == "" {
		return Acl_rule{}, false
	}

	kind, value, has_value := strings.Cut(line, ":")
	rule.Kind = strings.ToLower(strings.TrimSpace(kind))
	if has_value {
		rule.Value = strings.TrimSpace(value)
	}

	// 옛 이름을 새 이름으로 맞춘다
	switch rule.Kind {
	case "all":
		rule.Kind = "any"
	case "member":
		// 그대로
	}

	if rule.Kind == "" {
		return Acl_rule{}, false
	}

	return rule, true
}

// Acl_rule_join : 규칙 목록을 저장할 글로
func Acl_rule_join(rules []Acl_rule) string {
	lines := []string{}
	for _, rule := range rules {
		lines = append(lines, Acl_rule_line(rule))
	}

	return strings.Join(lines, "\n")
}

// Acl_rule_line : 규칙 하나를 저장할 글로
func Acl_rule_line(rule Acl_rule) string {
	head := "거부"
	if rule.Allow {
		head = "허용"
	}

	body := rule.Kind
	if rule.Value != "" {
		body += ":" + rule.Value
	}

	return head + ":" + body
}

// Acl_rule_kind_needs_value : 값을 함께 적어야 하는 갈래인가
func Acl_rule_kind_needs_value(kind string) bool {
	for _, one := range Acl_rule_kind_list() {
		if one[0] == kind {
			return one[1] != ""
		}
	}

	return false
}

// Acl_rule_context : 규칙을 판정할 때 쓰는 이용자 정보
//
//	Check_acl 이 이미 읽어 둔 것을 그대로 넘겨받아 다시 읽지 않는다.
type Acl_rule_context struct {
	IP        string
	IsIP      bool // 비로그인 아이피인가
	Auth      map[string]bool
	Level     int
	DocName   string
	CountryOK bool   // 나라를 알아냈는가
	Country   string // 두 글자 나라 코드 (대문자)
}

// Acl_rule_check : 규칙 목록을 위에서부터 훑는다
//
//	처음 걸린 규칙이 결론이다. 아무것도 안 걸리면 Acl_rule_none.
func Acl_rule_check(db *sql.DB, rules []Acl_rule, context Acl_rule_context) Acl_rule_result {
	for _, rule := range rules {
		if !Acl_rule_match(db, rule, context) {
			continue
		}

		if rule.Allow {
			return Acl_rule_allow
		}

		return Acl_rule_deny
	}

	return Acl_rule_none
}

// acl_rule_list_takes_banned : 차단된 이용자까지 헤아리는 규칙이 섞여 있는가
//
//	`ban` · `ban_admin` 은 「차단된 사람도 포함」 이라는 뜻의 옛 조건이다.
//	이런 규칙이 있으면 차단만으로 먼저 잘라 내지 않고 규칙에 맡긴다.
func acl_rule_list_takes_banned(rules []Acl_rule) bool {
	for _, rule := range rules {
		if rule.Kind == "ban" || rule.Kind == "ban_admin" {
			return true
		}
	}

	return false
}

// acl_rule_list_uses_geoip : 나라를 따지는 규칙이 있는가
//
//	있을 때만 나라를 알아본다. 없으면 괜히 표를 읽지 않는다.
func acl_rule_list_uses_geoip(rules []Acl_rule) bool {
	for _, rule := range rules {
		if rule.Kind == "geoip" {
			return true
		}
	}

	return false
}

// Acl_rule_match : 규칙 하나가 이 이용자에게 걸리는가
func Acl_rule_match(db *sql.DB, rule Acl_rule, context Acl_rule_context) bool {
	switch rule.Kind {
	case "any", "ban":
		return true
	case "not_all":
		return false
	case "member", "user_login":
		return !context.IsIP

	case "perm":
		return acl_rule_perm(context, rule.Value)

	case "user":
		// `user:이름` 은 그 계정만, 값이 없으면 옛 뜻(로그인 사용자)
		if rule.Value == "" {
			return !context.IsIP
		}

		return !context.IsIP && strings.EqualFold(context.IP, rule.Value)

	case "ip":
		if rule.Value == "" {
			return context.IsIP
		}

		return Acl_ip_match(context.IP, rule.Value)

	case "aclgroup":
		return Acl_group_has(db, rule.Value, context.IP)

	case "geoip":
		if !context.CountryOK {
			return false
		}

		return acl_rule_country_match(context.Country, rule.Value)
	}

	// 옛 조건들 (admin · 50_edit · 30_day …)
	return acl_rule_legacy(db, rule.Kind, context)
}

// acl_rule_perm : 권한을 가지고 있는가
func acl_rule_perm(context Acl_rule_context, name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}

	// 화면에서 고르기 쉬운 이름을 실제 권한 이름으로 바꿔 준다
	switch name {
	case "admin":
		name = "treat_as_admin"
	case "member":
		name = "user"
	}

	return context.Auth[name]
}

// acl_rule_country_match : 나라 코드가 맞는가 (쉼표로 여럿 적을 수 있다)
func acl_rule_country_match(country string, value string) bool {
	country = strings.ToUpper(strings.TrimSpace(country))
	if country == "" {
		return false
	}

	for _, one := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(one), country) {
			return true
		}
	}

	return false
}

// Acl_ip_match : 아이피가 그 주소 또는 대역에 드는가
//
//	`192.168.0.1` 처럼 하나만 적어도 되고 `192.168.0.0/16` 처럼 대역으로 적어도 된다.
func Acl_ip_match(ip string, value string) bool {
	ip = strings.TrimSpace(ip)
	value = strings.TrimSpace(value)
	if ip == "" || value == "" {
		return false
	}

	if !strings.Contains(value, "/") {
		return strings.EqualFold(ip, value)
	}

	// 대역은 아이피일 때만 따진다 (계정 이름이 대역에 들 리 없다)
	if net.ParseIP(ip) == nil {
		return false
	}

	parsed, err := cidr.Parse(value)
	if err != nil {
		return false
	}

	return parsed.Contains(ip)
}
