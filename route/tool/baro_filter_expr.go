package tool

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ============================================================
//  편집 필터 조건식 문법
//
//  정규식 하나로는 「본문에 이 낱말이 있고, 이름공간이 문서이고,
//  아이피 사용자일 때만」 같은 조건을 적을 수 없다.
//  그래서 짧은 조건식을 쓸 수 있게 한다.
//
//    content contains "카지노" and namespace == "문서"
//    title matches "^테스트" or user_is_ip
//    added > 5000 and user_is_ip
//    not (user_is_member) and links > 10
//
//  쓸 수 있는 값
//    content    본문 전체            (글)
//    old        고치기 전 본문       (글)
//    title      문서 이름            (글)
//    summary    편집 요약            (글)
//    user       편집한 사람 이름     (글)
//    namespace  이름공간 이름        (글) — 일반 문서는 "문서"
//    length     본문 길이            (수)
//    old_length 고치기 전 길이       (수)
//    added      늘어난 글자 수       (수)
//    removed    줄어든 글자 수       (수)
//    links      바깥 링크 수         (수)
//    recent     5분 안 편집 횟수     (수)
//    user_is_ip     아이피 사용자인가        (참·거짓)
//    user_is_member 로그인 사용자인가        (참·거짓)
//    is_new         새로 만드는 문서인가     (참·거짓)
//    is_blank       본문을 통째로 비우는가   (참·거짓)
//
//  쓸 수 있는 견주기
//    ==  !=  >  <  >=  <=
//    contains  !contains        (글 안에 들어 있는가)
//    matches   !matches         (정규식에 맞는가)
//    in        !in              (쉼표로 나눈 목록 안에 있는가)
//
//  이어 붙이기 : and / or / not  (그리고 · 또는 · 아님 도 된다)
//  괄호로 묶을 수 있다.
// ============================================================

// Baro_filter_env : 조건식이 볼 수 있는 값 묶음
type Baro_filter_env struct {
	Content   string
	Old       string
	Title     string
	Summary   string
	User      string
	Namespace string
	Recent    int
}

// baro_filter_expr_link_regex : 바깥 링크 세기
var baro_filter_expr_link_regex = regexp.MustCompile(`https?://`)

// Baro_filter_env_make : 편집 하나에서 값 묶음을 만든다
func Baro_filter_env_make(db_namespace string, title string, user string, content string, old string, summary string, recent int) Baro_filter_env {
	return Baro_filter_env{
		Content:   content,
		Old:       old,
		Title:     title,
		Summary:   summary,
		User:      user,
		Namespace: db_namespace,
		Recent:    recent,
	}
}

// baro_filter_expr_value : 이름 하나의 값 (글, 수, 참거짓 가운데 하나)
type baro_filter_expr_value struct {
	text    string
	number  int
	boolean bool
	kind    string // "text" / "number" / "bool"
}

// baro_filter_expr_lookup : 이름으로 값 찾기
func baro_filter_expr_lookup(name string, env Baro_filter_env) (baro_filter_expr_value, bool) {
	text := func(value string) baro_filter_expr_value {
		return baro_filter_expr_value{text: value, kind: "text"}
	}
	number := func(value int) baro_filter_expr_value {
		return baro_filter_expr_value{number: value, kind: "number"}
	}
	boolean := func(value bool) baro_filter_expr_value {
		return baro_filter_expr_value{boolean: value, kind: "bool"}
	}

	new_length := Get_len(env.Content)
	old_length := Get_len(env.Old)

	switch strings.ToLower(name) {
	case "content", "본문":
		return text(env.Content), true
	case "old", "이전본문":
		return text(env.Old), true
	case "title", "문서", "제목":
		return text(env.Title), true
	case "summary", "send", "요약":
		return text(env.Summary), true
	case "user", "사용자":
		return text(env.User), true
	case "namespace", "이름공간":
		return text(env.Namespace), true
	case "length", "길이":
		return number(new_length), true
	case "old_length", "이전길이":
		return number(old_length), true
	case "added", "늘어난양":
		if new_length > old_length {
			return number(new_length - old_length), true
		}

		return number(0), true
	case "removed", "줄어든양":
		if old_length > new_length {
			return number(old_length - new_length), true
		}

		return number(0), true
	case "links", "링크수":
		return number(len(baro_filter_expr_link_regex.FindAllString(env.Content, -1))), true
	case "recent", "최근편집수":
		return number(env.Recent), true
	case "user_is_ip", "아이피사용자":
		return boolean(IP_or_user(env.User)), true
	case "user_is_member", "로그인사용자":
		return boolean(env.User != "" && !IP_or_user(env.User)), true
	case "is_new", "새문서":
		return boolean(strings.TrimSpace(env.Old) == ""), true
	case "is_blank", "빈문서":
		return boolean(strings.TrimSpace(env.Content) == ""), true
	}

	return baro_filter_expr_value{}, false
}

// Baro_filter_expr_names : 쓸 수 있는 이름 목록 (도움말용)
func Baro_filter_expr_names() [][2]string {
	return [][2]string{
		{"content", "본문 전체"},
		{"old", "고치기 전 본문"},
		{"title", "문서 이름"},
		{"summary", "편집 요약"},
		{"user", "편집한 사람 이름"},
		{"namespace", "이름공간 (일반 문서는 문서)"},
		{"length", "본문 길이"},
		{"old_length", "고치기 전 길이"},
		{"added", "늘어난 글자 수"},
		{"removed", "줄어든 글자 수"},
		{"links", "바깥 링크 수"},
		{"recent", "5분 안 편집 횟수"},
		{"user_is_ip", "아이피 사용자인가"},
		{"user_is_member", "로그인 사용자인가"},
		{"is_new", "새로 만드는 문서인가"},
		{"is_blank", "본문을 통째로 비우는가"},
	}
}

// ------------------------------------------------------------
//  토큰 나누기
// ------------------------------------------------------------

type baro_filter_token struct {
	kind string // "word" / "string" / "number" / "op" / "(" / ")"
	text string
}

// baro_filter_expr_tokens : 조건식을 토큰으로 나눈다
func baro_filter_expr_tokens(source string) ([]baro_filter_token, string) {
	runes := []rune(source)
	out := []baro_filter_token{}

	for index := 0; index < len(runes); {
		one := runes[index]

		if unicode.IsSpace(one) {
			index++

			continue
		}

		if one == '(' || one == ')' {
			out = append(out, baro_filter_token{kind: string(one), text: string(one)})
			index++

			continue
		}

		if one == '"' || one == '\'' {
			quote := one
			index++
			start := index
			var builder strings.Builder
			closed := false
			for index < len(runes) {
				if runes[index] == '\\' && index+1 < len(runes) {
					builder.WriteRune(runes[index+1])
					index += 2

					continue
				}
				if runes[index] == quote {
					closed = true
					index++

					break
				}

				builder.WriteRune(runes[index])
				index++
			}
			if !closed {
				return nil, "따옴표를 닫지 않았습니다."
			}
			_ = start

			out = append(out, baro_filter_token{kind: "string", text: builder.String()})

			continue
		}

		// 견주기 기호
		if strings.ContainsRune("=!<>&|", one) {
			two := string(one)
			if index+1 < len(runes) {
				pair := string(one) + string(runes[index+1])
				if pair == "==" || pair == "!=" || pair == ">=" || pair == "<=" || pair == "&&" || pair == "||" {
					two = pair
				}
			}
			if two == "!" && index+1 < len(runes) && !unicode.IsSpace(runes[index+1]) && runes[index+1] != '(' {
				// !contains / !matches / !in 처럼 낱말이 붙어 온다
				index++
				start := index
				for index < len(runes) && (unicode.IsLetter(runes[index]) || runes[index] == '_') {
					index++
				}
				out = append(out, baro_filter_token{kind: "op", text: "!" + strings.ToLower(string(runes[start:index]))})

				continue
			}

			out = append(out, baro_filter_token{kind: "op", text: two})
			index += len([]rune(two))

			continue
		}

		if unicode.IsDigit(one) {
			start := index
			for index < len(runes) && unicode.IsDigit(runes[index]) {
				index++
			}
			out = append(out, baro_filter_token{kind: "number", text: string(runes[start:index])})

			continue
		}

		// 낱말 (이름 · and · or · not · contains …)
		start := index
		for index < len(runes) && !unicode.IsSpace(runes[index]) && !strings.ContainsRune("()=!<>&|\"'", runes[index]) {
			index++
		}
		if index == start {
			return nil, "알 수 없는 글자가 있습니다 : " + string(one)
		}

		out = append(out, baro_filter_token{kind: "word", text: string(runes[start:index])})
	}

	return out, ""
}

// ------------------------------------------------------------
//  풀이
// ------------------------------------------------------------

type baro_filter_parser struct {
	tokens []baro_filter_token
	at     int
	env    Baro_filter_env
	detail []string
	fail   string
}

func (parser *baro_filter_parser) peek() baro_filter_token {
	if parser.at >= len(parser.tokens) {
		return baro_filter_token{}
	}

	return parser.tokens[parser.at]
}

func (parser *baro_filter_parser) next() baro_filter_token {
	one := parser.peek()
	parser.at++

	return one
}

// baro_filter_is_or : or 이음말인가
func baro_filter_is_or(one baro_filter_token) bool {
	if one.kind == "op" && one.text == "||" {
		return true
	}

	return one.kind == "word" && (strings.EqualFold(one.text, "or") || one.text == "또는")
}

// baro_filter_is_and : and 이음말인가
func baro_filter_is_and(one baro_filter_token) bool {
	if one.kind == "op" && one.text == "&&" {
		return true
	}

	return one.kind == "word" && (strings.EqualFold(one.text, "and") || one.text == "그리고")
}

// baro_filter_is_not : not 이음말인가
func baro_filter_is_not(one baro_filter_token) bool {
	if one.kind == "op" && one.text == "!" {
		return true
	}

	return one.kind == "word" && (strings.EqualFold(one.text, "not") || one.text == "아님")
}

func (parser *baro_filter_parser) parse_or() bool {
	left := parser.parse_and()
	for baro_filter_is_or(parser.peek()) {
		parser.next()
		right := parser.parse_and()
		left = left || right
	}

	return left
}

func (parser *baro_filter_parser) parse_and() bool {
	left := parser.parse_not()
	for baro_filter_is_and(parser.peek()) {
		parser.next()
		right := parser.parse_not()
		left = left && right
	}

	return left
}

func (parser *baro_filter_parser) parse_not() bool {
	if baro_filter_is_not(parser.peek()) {
		parser.next()

		return !parser.parse_not()
	}

	return parser.parse_primary()
}

func (parser *baro_filter_parser) parse_primary() bool {
	one := parser.next()

	if one.kind == "(" {
		inside := parser.parse_or()
		if parser.peek().kind != ")" {
			parser.set_fail("괄호를 닫지 않았습니다.")

			return false
		}
		parser.next()

		return inside
	}

	if one.kind != "word" {
		parser.set_fail("여기에는 값 이름이 와야 합니다.")

		return false
	}

	left, ok := baro_filter_expr_lookup(one.text, parser.env)
	if !ok {
		parser.set_fail("모르는 값 이름입니다 : " + one.text)

		return false
	}

	// 뒤에 견주기가 없으면 그 값 자체를 참·거짓으로 본다
	operator := parser.peek()
	if !baro_filter_is_operator(operator) {
		switch left.kind {
		case "bool":
			if left.boolean {
				parser.add_detail(one.text)
			}

			return left.boolean
		case "number":
			if left.number != 0 {
				parser.add_detail(one.text + "=" + strconv.Itoa(left.number))
			}

			return left.number != 0
		default:
			if strings.TrimSpace(left.text) != "" {
				parser.add_detail(one.text)
			}

			return strings.TrimSpace(left.text) != ""
		}
	}

	parser.next()
	right := parser.next()
	if right.kind != "word" && right.kind != "string" && right.kind != "number" {
		parser.set_fail("견줄 값이 없습니다.")

		return false
	}

	return parser.compare(one.text, left, baro_filter_operator_name(operator), right.text)
}

// baro_filter_is_operator : 견주기 기호인가
func baro_filter_is_operator(one baro_filter_token) bool {
	if one.kind == "op" {
		switch one.text {
		case "==", "!=", ">", "<", ">=", "<=", "!contains", "!matches", "!in":
			return true
		}

		return false
	}

	if one.kind == "word" {
		switch strings.ToLower(one.text) {
		case "contains", "matches", "in", "포함", "일치", "안에":
			return true
		}
	}

	return false
}

// baro_filter_operator_name : 견주기 기호를 표준 이름으로
func baro_filter_operator_name(one baro_filter_token) string {
	name := strings.ToLower(one.text)
	switch name {
	case "포함":
		return "contains"
	case "일치":
		return "matches"
	case "안에":
		return "in"
	}

	return name
}

func (parser *baro_filter_parser) set_fail(message string) {
	if parser.fail == "" {
		parser.fail = message
	}
}

func (parser *baro_filter_parser) add_detail(message string) {
	if len(parser.detail) < 6 {
		parser.detail = append(parser.detail, message)
	}
}

// compare : 값 하나를 견준다
func (parser *baro_filter_parser) compare(name string, left baro_filter_expr_value, operator string, right string) bool {
	switch operator {
	case ">", "<", ">=", "<=":
		left_number := left.number
		if left.kind == "text" {
			left_number = Get_len(left.text)
		}
		right_number := Str_to_int(right)

		result := false
		switch operator {
		case ">":
			result = left_number > right_number
		case "<":
			result = left_number < right_number
		case ">=":
			result = left_number >= right_number
		case "<=":
			result = left_number <= right_number
		}
		if result {
			parser.add_detail(name + " " + operator + " " + right + " (" + strconv.Itoa(left_number) + ")")
		}

		return result

	case "==", "!=":
		left_text := left.text
		if left.kind == "number" {
			left_text = strconv.Itoa(left.number)
		}
		if left.kind == "bool" {
			left_text = "false"
			if left.boolean {
				left_text = "true"
			}
		}

		same := strings.EqualFold(strings.TrimSpace(left_text), strings.TrimSpace(right))
		result := same
		if operator == "!=" {
			result = !same
		}
		if result {
			parser.add_detail(name + " " + operator + " " + right)
		}

		return result

	case "contains", "!contains":
		found := right != "" && strings.Contains(left.text, right)
		result := found
		if operator == "!contains" {
			result = !found
		}
		if result {
			parser.add_detail(name + " " + operator + " " + Baro_trunc(right, 40))
		}

		return result

	case "matches", "!matches":
		pattern, err := regexp.Compile(right)
		if err != nil {
			parser.set_fail("정규식을 읽을 수 없습니다 : " + right)

			return false
		}

		found := pattern.MatchString(left.text)
		result := found
		if operator == "!matches" {
			result = !found
		}
		if result {
			hit := pattern.FindString(left.text)
			if hit == "" {
				hit = right
			}
			parser.add_detail(name + " " + operator + " " + Baro_trunc(Baro_one_line(hit), 40))
		}

		return result

	case "in", "!in":
		left_text := strings.TrimSpace(left.text)
		if left.kind == "number" {
			left_text = strconv.Itoa(left.number)
		}

		found := false
		for _, item := range strings.Split(right, ",") {
			if strings.EqualFold(strings.TrimSpace(item), left_text) {
				found = true

				break
			}
		}

		result := found
		if operator == "!in" {
			result = !found
		}
		if result {
			parser.add_detail(name + " " + operator + " " + Baro_trunc(right, 40))
		}

		return result
	}

	parser.set_fail("모르는 견주기입니다 : " + operator)

	return false
}

// Baro_filter_expr_run : 조건식을 실제 편집에 대고 풀어 본다
//
//	돌려주는 값 : (걸렸는가, 걸린 까닭, 문법 오류)
//	문법 오류가 있으면 걸리지 않은 것으로 본다 (규칙이 잘못돼서 편집을 막으면 안 된다).
func Baro_filter_expr_run(source string, env Baro_filter_env) (bool, string, string) {
	tokens, token_error := baro_filter_expr_tokens(source)
	if token_error != "" {
		return false, "", token_error
	}
	if len(tokens) == 0 {
		return false, "", "조건식이 비어 있습니다."
	}

	parser := &baro_filter_parser{tokens: tokens, env: env}
	result := parser.parse_or()

	if parser.fail != "" {
		return false, "", parser.fail
	}
	if parser.at < len(parser.tokens) {
		return false, "", "조건식 뒤에 읽지 못한 부분이 있습니다 : " + parser.tokens[parser.at].text
	}

	return result, strings.Join(parser.detail, " · "), ""
}

// Baro_filter_expr_check : 조건식 문법만 살펴본다 (저장하기 전 검사용)
func Baro_filter_expr_check(source string) string {
	_, _, check_error := Baro_filter_expr_run(source, Baro_filter_env{
		Content:   "보기 글",
		Old:       "예전 글",
		Title:     "보기 문서",
		Summary:   "보기 요약",
		User:      "127.0.0.1",
		Namespace: "문서",
		Recent:    0,
	})

	return check_error
}
