package markup

import (
	"regexp"
	"strings"
)

// ============================================================
//  {{{#!style …}}} 문서 안 CSS
//
//  나무위키처럼 문서 하나에서만 쓰는 CSS 를 본문에 적어 둔다.
//
//    {{{#!style
//    .button { padding: .5rem 1rem; }
//
//    @theseed-dark-mode {
//        .button { background: #201314; }
//    }
//    }}}
//
//  @theseed-dark-mode 는 다크 모드용 묶음이다.
//  위키가 지금 다크 모드이면 그 안을 꺼내 쓰고, 아니면 통째로 버린다.
//  (위키 설정으로 모드가 정해지므로 미디어 쿼리로 옮기지 않는다)
//
//  ── 왜 걸러 내는가 ────────────────────────────────────
//  CSS 는 그림처럼 보이지만 글을 읽어 낼 수도, 화면을 덮어 버릴 수도 있다.
//  그래서 본문에 적는 CSS 는 아래를 막는다.
//
//    - </style> 로 빠져나가 아무 태그나 여는 것
//    - url( · @import 로 바깥을 부르는 것 (누가 문서를 읽었는지 새어 나간다)
//    - expression( · javascript: (옛 브라우저에서 스크립트가 된다)
//    - position: fixed 로 화면 전체를 덮고 남의 클릭을 가로채는 것
//    - @charset · @namespace 처럼 문서 전체에 걸리는 것
//
//  중괄호 짝이 맞지 않으면 통째로 버린다. 짝이 어긋난 CSS 는
//  뒤에 오는 규칙까지 함께 먹어 치우기 때문이다.
// ============================================================

// namumark_style_dark_mark : 다크 모드 묶음 머리
const namumark_style_dark_mark = "@theseed-dark-mode"

// namumark_style_comment_regex : /* … */ 주석
var namumark_style_comment_regex = regexp.MustCompile(`(?s)/\*.*?\*/`)

// namumark_style_bad_words : CSS 에 있으면 통째로 버리는 것
var namumark_style_bad_words = []string{
	"</style", "<script", "expression(", "javascript:", "vbscript:",
	"@import", "@charset", "@namespace", "url(", "behavior:", "-moz-binding",
}

// namumark_style_bad_position : 화면을 덮어 버리는 자리잡기
var namumark_style_bad_position = regexp.MustCompile(`(?i)position\s*:\s*(fixed|sticky)`)

// namumark_style_clean : 본문에 적은 CSS 를 안전한 만큼만 남긴다
//
//	쓸 수 없는 것이 하나라도 있으면 빈 문자열을 돌려준다.
//	조각만 지우면 남은 규칙이 뜻하지 않은 모양이 되기 때문이다.
func namumark_style_clean(css string, darkmode bool) string {
	css = compat_html_unescape_style(css)
	css = namumark_style_comment_regex.ReplaceAllString(css, " ")

	css = namumark_style_pick_dark(css, darkmode)
	if strings.TrimSpace(css) == "" {
		return ""
	}

	lower := strings.ToLower(css)
	lower = strings.ReplaceAll(lower, "\x00", "")
	for _, bad := range namumark_style_bad_words {
		if strings.Contains(lower, bad) {
			return ""
		}
	}
	if namumark_style_bad_position.MatchString(lower) {
		return ""
	}
	if strings.ContainsAny(css, "<>") {
		return ""
	}
	if !namumark_style_braces_ok(css) {
		return ""
	}

	return strings.TrimSpace(css)
}

// namumark_style_braces_ok : 중괄호 짝이 맞는가
func namumark_style_braces_ok(css string) bool {
	depth := 0
	for _, one := range css {
		switch one {
		case '{':
			depth++
			if depth > 8 {
				return false
			}
		case '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}

	return depth == 0
}

// namumark_style_pick_dark : @theseed-dark-mode 묶음을 꺼내거나 버린다
//
//	다크 모드이면 묶음 안쪽을 바깥으로 꺼내 쓰고,
//	아니면 묶음을 통째로 지운다.
func namumark_style_pick_dark(css string, darkmode bool) string {
	for {
		index := namumark_style_find_dark(css)
		if index < 0 {
			return css
		}

		open := strings.IndexByte(css[index:], '{')
		if open < 0 {
			// 여는 괄호가 없으면 머리만 지우고 넘어간다
			return css[:index] + css[index+len(namumark_style_dark_mark):]
		}
		open += index

		depth := 0
		close_index := -1
		for scan := open; scan < len(css); scan++ {
			switch css[scan] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					close_index = scan
				}
			}
			if close_index >= 0 {
				break
			}
		}
		if close_index < 0 {
			// 닫히지 않은 묶음은 믿을 수 없다
			return ""
		}

		inner := ""
		if darkmode {
			inner = css[open+1 : close_index]
		}

		css = css[:index] + inner + css[close_index+1:]
	}
}

// namumark_style_find_dark : 다음 @theseed-dark-mode 자리 (없으면 -1)
func namumark_style_find_dark(css string) int {
	lower := strings.ToLower(css)

	return strings.Index(lower, namumark_style_dark_mark)
}

// Namumark_style_scope_prefix : 문서 안 CSS 가 걸릴 수 있는 테두리
//
//	본문을 감싸는 상자다. 이 안쪽에만 규칙이 걸리므로
//	문서 하나가 위키 전체의 차림표나 머리글을 바꿔 놓지 못한다.
const Namumark_style_scope_prefix = ".opennamu_main"

// namumark_style_scope : 문서 안 CSS 가 본문 밖으로 새지 않게 울타리를 친다
//
//	`.button { … }` 을 `.opennamu_main .button { … }` 으로 바꾼다.
//
//	@media 처럼 안에 다시 규칙이 들어가는 묶음은 안쪽을 다시 훑는다.
func namumark_style_scope(css string, scope string) string {
	out := strings.Builder{}
	rest := css

	for {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			// 뒤에 남은 것은 규칙이 아니므로 버린다
			break
		}

		selector := strings.TrimSpace(rest[:open])

		// 짝이 맞는 닫는 괄호 찾기
		depth := 0
		close_index := -1
		for scan := open; scan < len(rest); scan++ {
			switch rest[scan] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					close_index = scan
				}
			}
			if close_index >= 0 {
				break
			}
		}
		if close_index < 0 {
			break
		}

		body := rest[open+1 : close_index]

		if strings.HasPrefix(selector, "@") {
			// @media · @supports 처럼 안에 규칙이 다시 들어가는 묶음
			out.WriteString(selector + "{" + namumark_style_scope(body, scope) + "}")
		} else if inner := namumark_style_scope_selector(selector, scope); inner != "" {
			out.WriteString(inner + "{" + body + "}")
		}

		rest = rest[close_index+1:]
	}

	return out.String()
}

// namumark_style_scope_selector : 고르개 하나하나에 울타리를 붙인다
func namumark_style_scope_selector(selector string, scope string) string {
	parts := []string{}
	for _, one := range strings.Split(selector, ",") {
		one = strings.TrimSpace(one)
		if one == "" {
			continue
		}

		parts = append(parts, scope+" "+one)
	}

	return strings.Join(parts, ",")
}

// Namumark_style_directive_split : 첫 줄이 `#!style` 이면 남은 본문을 CSS 로 떼어 낸다
//
//	문서 하나를 통째로 CSS 로 쓰는 모양이다.
//
//	  #!style
//	  .button { padding: .5rem 1rem; }
//
//	`[include(틀:스타일)]` 로 불러다 쓰기 좋도록 남은 줄 전부를 CSS 로 본다.
//	`#!style` 뒤에 다른 글자가 붙어 있으면 지시문이 아니다.
func Namumark_style_directive_split(data string) (string, bool) {
	trimmed := strings.TrimLeft(data, " \t\r\n")
	if !strings.HasPrefix(trimmed, "#!style") {
		return "", false
	}

	rest := trimmed[len("#!style"):]

	newline := strings.IndexByte(rest, '\n')
	if newline < 0 {
		if strings.TrimSpace(rest) != "" {
			return "", false
		}

		return "", true
	}

	if strings.TrimSpace(rest[:newline]) != "" {
		return "", false
	}

	return rest[newline+1:], true
}

// compat_html_unescape_style : 본문을 지나오며 바뀐 기호를 되돌린다
func compat_html_unescape_style(css string) string {
	return strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&#x27;", "'",
		"&nbsp;", " ",
	).Replace(css)
}
