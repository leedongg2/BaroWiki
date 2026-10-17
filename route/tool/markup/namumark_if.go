package markup

import (
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  {{{#!if …}}} 조건문
//
//  나무위키 조건문과 같은 모양을 받는다.
//
//    완료 == null
//    완료 != null && 중단 != null
//    (완료 == null || 중단 == null) && 종류 == "프로젝트"
//
//  견주기 : ==  !=  <  >  <=  >=
//  이어 붙이기 : &&  ||   (&& 가 || 보다 세다)
//  괄호로 묶을 수 있다.
//
//  왼쪽은 언제나 틀 매개변수 이름이고 오른쪽은 값이다.
//  값 자리의 null 은 「그 매개변수가 넘어오지 않았다」 는 뜻이다.
//
//  읽다가 모양이 어긋나면 조건 전체를 거짓으로 본다.
//  틀린 조건이 참이 되어 엉뚱한 내용이 나오는 쪽이 더 나쁘기 때문이다.
// ============================================================

// namumark_if_lookup : 매개변수 하나를 찾아 주는 함수 (값, 있는가)
type namumark_if_lookup func(name string) (any, bool)

// namumark_if_parser : 조건 하나를 앞에서부터 읽어 나간다
type namumark_if_parser struct {
	text   string
	index  int
	lookup namumark_if_lookup
	broken bool

	// one_line : 한 줄 쓰기인가
	//
	//	한 줄로 쓰면 조건 뒤에 곧바로 내용이 붙는다.
	//	  {{{#!if 완료 == null 라벨}}}
	//	그래서 값이 어디서 끝나는지 빈칸으로 가른다.
	//	값에 빈칸이 들어가야 하면 따옴표로 묶는다.
	one_line bool
}

// Namumark_if_eval : 조건이 참인가
//
//	markup 바깥에서도 같은 조건 문법을 쓸 수 있게 내보낸다.
func Namumark_if_eval(condition string, lookup namumark_if_lookup) bool {
	if lookup == nil {
		lookup = func(string) (any, bool) { return nil, false }
	}

	parser := &namumark_if_parser{text: condition, lookup: lookup}
	result := parser.parse_or()

	parser.skip_space()
	if parser.broken || parser.index < len(parser.text) {
		return false
	}

	return result
}

// Namumark_if_eval_line : 한 줄로 쓴 조건문을 읽는다
//
//	  {{{#!if 완료 == null 라벨}}}
//
//	조건이 어디서 끝나는지는 읽어 가면서 스스로 알아낸다.
//	돌려주는 값은 (참인가, 조건이 끝난 자리, 제대로 읽었는가) 이다.
//	끝난 자리부터가 보여 줄 내용이다.
//
//	값에 빈칸이 들어가야 하면 따옴표로 묶는다.
//	  {{{#!if 종류 == "우리 프로젝트" 내용}}}
func Namumark_if_eval_line(condition string, lookup namumark_if_lookup) (bool, int, bool) {
	if lookup == nil {
		lookup = func(string) (any, bool) { return nil, false }
	}

	parser := &namumark_if_parser{text: condition, lookup: lookup, one_line: true}
	result := parser.parse_or()

	if parser.broken {
		return false, 0, false
	}

	return result, parser.index, true
}

// skip_space : 빈칸 건너뛰기
func (parser *namumark_if_parser) skip_space() {
	for parser.index < len(parser.text) {
		switch parser.text[parser.index] {
		case ' ', '\t', '\r', '\n':
			parser.index++
		default:
			return
		}
	}
}

// peek : 지금 자리에서 열쇠말이 나오는가
func (parser *namumark_if_parser) peek(mark string) bool {
	return strings.HasPrefix(parser.text[parser.index:], mark)
}

// parse_or : A || B || C
func (parser *namumark_if_parser) parse_or() bool {
	result := parser.parse_and()

	for {
		parser.skip_space()
		if !parser.peek("||") {
			return result
		}
		parser.index += 2

		// 앞이 참이어도 뒤까지 읽어야 모양이 어긋난 것을 잡아낸다
		if parser.parse_and() {
			result = true
		}
	}
}

// parse_and : A && B && C
func (parser *namumark_if_parser) parse_and() bool {
	result := parser.parse_unary()

	for {
		parser.skip_space()
		if !parser.peek("&&") {
			return result
		}
		parser.index += 2

		if !parser.parse_unary() {
			result = false
		}
	}
}

// parse_unary : ( … ) 또는 견주기 하나
func (parser *namumark_if_parser) parse_unary() bool {
	parser.skip_space()

	if parser.index < len(parser.text) && parser.text[parser.index] == '(' {
		parser.index++
		result := parser.parse_or()

		parser.skip_space()
		if parser.index >= len(parser.text) || parser.text[parser.index] != ')' {
			parser.broken = true

			return false
		}
		parser.index++

		return result
	}

	return parser.parse_compare()
}

// parse_compare : 매개변수 op 값
func (parser *namumark_if_parser) parse_compare() bool {
	name, _ := parser.read_operand()
	if name == "" {
		parser.broken = true

		return false
	}

	parser.skip_space()
	operator := parser.read_operator()
	if operator == "" {
		parser.broken = true

		return false
	}

	expected, quoted := parser.read_operand()

	// 값 자리가 비어 있는 것은 `완료 === null` 처럼 기호를 잘못 적었을 때다.
	// 빈 값을 견주려면 따옴표로 묶어 적는다. (`완료 == ""`)
	if expected == "" && !quoted {
		parser.broken = true

		return false
	}

	return namumark_if_compare(parser.lookup, name, operator, expected)
}

// read_operator : 견주기 기호 읽기 (긴 것부터 본다)
func (parser *namumark_if_parser) read_operator() string {
	for _, operator := range []string{"==", "!=", "<=", ">=", "<", ">"} {
		if parser.peek(operator) {
			parser.index += len(operator)

			return operator
		}
	}

	return ""
}

// read_operand : 견줄 것 하나 읽기
//
//	따옴표로 묶으면 기호가 들어 있어도 그대로 값이 된다.
//	돌려주는 둘째 값은 「따옴표로 묶여 있었는가」 다.
//	묶지 않은 빈 값은 적다 만 조건이므로 부르는 쪽에서 가려낸다.
func (parser *namumark_if_parser) read_operand() (string, bool) {
	parser.skip_space()

	if parser.index < len(parser.text) {
		quote := parser.text[parser.index]
		if quote == '"' || quote == '\'' {
			parser.index++
			start := parser.index
			for parser.index < len(parser.text) && parser.text[parser.index] != quote {
				parser.index++
			}
			if parser.index >= len(parser.text) {
				parser.broken = true

				return "", false
			}
			value := parser.text[start:parser.index]
			parser.index++

			return value, true
		}
	}

	start := parser.index
	for parser.index < len(parser.text) {
		switch parser.text[parser.index] {
		case '=', '!', '<', '>', '&', '|', '(', ')':
			return strings.TrimSpace(parser.text[start:parser.index]), false
		case ' ', '\t', '\r', '\n':
			// 한 줄 쓰기에서는 빈칸이 값의 끝이다.
			// 여러 줄 쓰기에서는 첫 줄 전체가 조건이므로 값에 빈칸이 들어가도 된다.
			if parser.one_line {
				return strings.TrimSpace(parser.text[start:parser.index]), false
			}
		}
		parser.index++
	}

	return strings.TrimSpace(parser.text[start:]), false
}

// Namumark_if_origin_index : 되돌린 글에서의 자리를 원래 글에서의 자리로 옮긴다
//
//	조건은 `&amp;&amp;` 를 `&&` 로 되돌린 글에서 읽지만,
//	뒤에 남은 내용은 원래 글 그대로 내보내야 한다.
//	되돌린 채로 내보내면 이용자가 적어 둔 `&lt;script&gt;` 가 살아나기 때문이다.
func Namumark_if_origin_index(origin string, plain_index int) int {
	plain_count := 0
	index := 0

	for index < len(origin) {
		if plain_count >= plain_index {
			return index
		}

		step := 1
		grow := 1

		// `&...;` 모양은 되돌리면 길이가 줄어든다. 줄어든 만큼만 세어 준다.
		if origin[index] == '&' {
			if semi := strings.IndexByte(origin[index:], ';'); semi > 0 && semi <= 12 {
				entity := origin[index : index+semi+1]
				if decoded := tool.HTML_unescape(entity); decoded != entity {
					step = semi + 1
					grow = len(decoded)
				}
			}
		}

		index += step
		plain_count += grow
	}

	return len(origin)
}

// namumark_if_compare : 견주기 한 번
func namumark_if_compare(lookup namumark_if_lookup, name string, operator string, expected string) bool {
	value, exists := lookup(name)
	is_null := !exists || value == nil

	// 값 자리의 null 은 「넘어오지 않았다」 는 뜻이다
	if strings.EqualFold(expected, "null") {
		switch operator {
		case "==":
			return is_null
		case "!=":
			return !is_null
		}

		return false
	}

	// 넘어오지 않은 매개변수는 무엇과도 같지 않다
	if is_null {
		return operator == "!="
	}

	actual := compat_render_parameter_value(value)

	switch operator {
	case "==":
		return actual == expected
	case "!=":
		return actual != expected
	}

	// 크고 작음은 숫자로만 견준다
	left, left_err := strconv.ParseFloat(strings.TrimSpace(actual), 64)
	right, right_err := strconv.ParseFloat(strings.TrimSpace(expected), 64)
	if left_err != nil || right_err != nil {
		return false
	}

	switch operator {
	case "<":
		return left < right
	case ">":
		return left > right
	case "<=":
		return left <= right
	case ">=":
		return left >= right
	}

	return false
}
