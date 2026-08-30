package markup

import "strings"

// ============================================================
//  문법 키워드 별칭 (한국어 · 영어 · 일본어)
//
//  한국어판 · 영어판 · 일본어판이 같은 문서를 나눠 쓰므로
//  문법 키워드도 세 언어를 모두 알아들어야 한다.
//  이름공간 접두사는 tool.Namespace_normalize 가 따로 처리한다.
// ============================================================

// markup_redirect_prefixes : 넘겨주기 줄 머리
var markup_redirect_prefixes = []string{
	"#redirect ",
	"#넘겨주기 ",
	"#リダイレクト ",
	"#転送 ",
	"#てんそう ",
}

// markup_file_prefixes : 파일 링크 접두사
var markup_file_prefixes = []string{
	"file:",
	"파일:",
	"ファイル:",
}

// markup_out_prefixes : 바깥 그림 접두사
var markup_out_prefixes = []string{
	"out:",
	"외부:",
	"外部:",
	"がいぶ:",
}

// markup_category_prefixes : 분류 접두사
var markup_category_prefixes = []string{
	"category:",
	"분류:",
	"分類:",
	"ぶんるい:",
}

// markup_inter_prefixes : 인터위키 접두사
var markup_inter_prefixes = []string{
	"inter:",
	"인터:",
	"インター:",
}

// markup_toc_names : 목차 매크로 이름
var markup_toc_names = []string{
	"toc",
	"tableofcontents",
	"목차",
	"目次",
	"もくじ",
}

// markup_footnote_names : 각주 모아 보기 매크로 이름
var markup_footnote_names = []string{
	"footnote",
	"각주",
	"脚注",
	"きゃくちゅう",
}

// markup_has_prefix : 별칭 목록 가운데 하나로 시작하는지 (대소문자 무시)
func markup_has_prefix(data string, prefix_list []string) (string, bool) {
	lower := strings.ToLower(data)

	for _, prefix := range prefix_list {
		if strings.HasPrefix(lower, prefix) {
			return prefix, true
		}
	}

	return "", false
}

// markup_trim_prefix : 별칭 접두사를 떼어 낸다 (없으면 그대로)
func markup_trim_prefix(data string, prefix_list []string) string {
	if prefix, ok := markup_has_prefix(data, prefix_list); ok {
		return data[len(prefix):]
	}

	return data
}

// markup_is_name : 별칭 목록에 들어 있는 이름인지 (대소문자 무시)
func markup_is_name(data string, name_list []string) bool {
	lower := strings.ToLower(strings.TrimSpace(data))

	for _, name := range name_list {
		if lower == name {
			return true
		}
	}

	return false
}

// Markup_redirect_prefix : 넘겨주기 줄이면 그 머리를 돌려준다
func Markup_redirect_prefix(line string) (string, bool) {
	return markup_has_prefix(line, markup_redirect_prefixes)
}

// Markup_is_file_link : 파일 링크인지
func Markup_is_file_link(target string) bool {
	_, ok := markup_has_prefix(target, markup_file_prefixes)

	return ok
}

// Markup_is_out_link : 바깥 그림 링크인지
func Markup_is_out_link(target string) bool {
	_, ok := markup_has_prefix(target, markup_out_prefixes)

	return ok
}

// Markup_is_category_link : 분류 링크인지
func Markup_is_category_link(target string) bool {
	_, ok := markup_has_prefix(target, markup_category_prefixes)

	return ok
}

// Markup_is_inter_link : 인터위키 링크인지
func Markup_is_inter_link(target string) bool {
	_, ok := markup_has_prefix(target, markup_inter_prefixes)

	return ok
}

// Markup_trim_category_prefix : 분류 접두사 제거
func Markup_trim_category_prefix(target string) string {
	return markup_trim_prefix(target, markup_category_prefixes)
}

// Markup_trim_file_prefix : 파일 접두사 제거
func Markup_trim_file_prefix(target string) string {
	return markup_trim_prefix(target, markup_file_prefixes)
}

// Markup_is_toc_macro : 목차 매크로 이름인지
func Markup_is_toc_macro(name string) bool {
	return markup_is_name(name, markup_toc_names)
}

// Markup_is_footnote_macro : 각주 매크로 이름인지 (`[각주]` 처럼 대괄호가 붙어 있어도 된다)
func Markup_is_footnote_macro(name string) bool {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(strings.TrimPrefix(name, "["), "]")

	return markup_is_name(name, markup_footnote_names)
}

// Markup_footnote_macro_regex : 각주 모아 보기 매크로 정규식 조각
func Markup_footnote_macro_regex() string {
	return strings.Join(markup_footnote_names, "|")
}

// Markup_escape_link_prefixes : `[[:분류:X]]` 처럼 앞에 : 를 붙여 일반 링크로 쓰는 접두사
func Markup_escape_link_prefixes() []string {
	list := []string{}
	for _, one := range markup_category_prefixes {
		list = append(list, ":"+one)
	}
	for _, one := range markup_file_prefixes {
		list = append(list, ":"+one)
	}

	return list
}

// Markup_is_escape_link : `:분류:` `:file:` 처럼 앞에 : 가 붙은 링크인지
func Markup_is_escape_link(label string) bool {
	_, ok := markup_has_prefix(label, Markup_escape_link_prefixes())

	return ok
}
