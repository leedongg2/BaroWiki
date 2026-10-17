package markup

import (
	"regexp"
	"strings"
)

// ============================================================
//  {{{#!wiki …}}} 의 속성
//
//    {{{#!wiki style="color:#f00" class="form" tag="span"
//    내용}}}
//
//  style      … 늘 적용
//  dark-style … 다크 모드에서만 덧붙인다
//  class      … 문서가 {{{#!style …}}} 로 정의한 이름을 붙인다
//  tag        … 감쌀 태그 (적지 않으면 div)
//
//  class 와 tag 는 문서에 적는 값이므로 그대로 믿지 않는다.
//  글자를 걸러 속성에서 빠져나가지 못하게 하고,
//  스킨이 쓰는 이름은 문서가 가져다 쓰지 못하게 막는다.
// ============================================================

// namumark_compat_wiki_attr_regex : 본문 앞에 붙은 `이름="값"` 하나
var namumark_compat_wiki_attr_regex = regexp.MustCompile(
	`(?is)^(style|dark-style|class|tag)\s*=\s*` +
		`(?:\x22([^\x22]*)\x22|\x27([^\x27]*)\x27|&quot;(.*?)&quot;|&#x27;(.*?)&#x27;)(?:[ \t\n]+)`,
)

// namumark_wiki_class_regex : 쓸 수 있는 class 이름 (영문·숫자·밑줄·붙임표)
var namumark_wiki_class_regex = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// namumark_wiki_class_max : 한 번에 붙일 수 있는 class 개수
const namumark_wiki_class_max = 5

// namumark_wiki_class_reserved : 스킨이 쓰는 이름 (문서가 가져다 쓸 수 없다)
//
//	문서 하나가 이 이름을 붙이면 위키의 차림표나 머리글을 흉내 내거나
//	가려 버릴 수 있으므로 앞머리째로 막는다.
var namumark_wiki_class_reserved = []string{
	"opennamu", "namu", "bw", "article", "layout", "wiki",
	"header", "footer", "sidebar", "menu", "title", "main",
	"search", "nav", "skin", "ringo",
}

// compat_wiki_class : 문서가 적은 class 이름을 걸러 낸다
//
//	쓸 수 없는 이름은 조용히 버리고, 남은 이름만 돌려준다.
func compat_wiki_class(value string) string {
	out := []string{}
	seen := map[string]bool{}

	for _, one := range strings.Fields(value) {
		if len(out) >= namumark_wiki_class_max {
			break
		}
		if len(one) > 64 || !namumark_wiki_class_regex.MatchString(one) {
			continue
		}
		if namumark_wiki_class_is_reserved(one) {
			continue
		}
		if seen[one] {
			continue
		}

		seen[one] = true
		out = append(out, one)
	}

	return strings.Join(out, " ")
}

// namumark_wiki_class_is_reserved : 스킨이 쓰는 이름인가
func namumark_wiki_class_is_reserved(name string) bool {
	lower := strings.ToLower(name)

	for _, one := range namumark_wiki_class_reserved {
		if lower == one || strings.HasPrefix(lower, one+"_") || strings.HasPrefix(lower, one+"-") {
			return true
		}
	}

	return false
}

// namumark_wiki_tag_list : 감쌀 수 있는 태그
//
//	글을 담는 태그만 받는다. 표·목록·입력 칸처럼 짜임새를 바꾸는 태그는 받지 않는다.
var namumark_wiki_tag_list = map[string]bool{
	"div": true, "span": true, "p": true,
	"b": true, "strong": true, "i": true, "em": true,
	"u": true, "s": true, "del": true, "ins": true,
	"sub": true, "sup": true, "small": true, "big": true,
	"code": true, "mark": true, "q": true, "cite": true,
	"abbr": true, "kbd": true, "samp": true, "var": true,
	"bdi": true, "bdo": true, "wbr": false,
}

// compat_wiki_tag : 감쌀 태그 이름 (모르는 태그는 "" 를 돌려준다)
func compat_wiki_tag(value string) string {
	name := strings.ToLower(strings.TrimSpace(value))
	if namumark_wiki_tag_list[name] {
		return name
	}

	return ""
}
