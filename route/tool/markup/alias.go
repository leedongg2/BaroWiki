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

// ============================================================
//  접기 · 펼치기 / 글꼴 문법 (the seed 호환 확장)
// ============================================================

// markup_folding_names : 접기 블록 이름 (뒤에 + 가 붙으면 펼친 채로 시작)
var markup_folding_names = []string{
	"#!folding",
	"#!fold",
	"#!detail",
	"#!details",
	"#!접기",
	"#!펼치기",
	"#!펼치기접기",
	"#!折りたたみ",
}

// Markup_is_folding_name : 접기 블록 이름인지 (`#!folding+` 처럼 + 가 붙어도 된다)
func markup_is_folding_name(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimSuffix(name, "+")

	for _, one := range markup_folding_names {
		if name == one {
			return true
		}
	}

	return false
}

// markup_folding_default_title : 제목을 적지 않았을 때 쓰는 글
func markup_folding_default_title() string {
	return "펼치기 · 접기"
}

// markup_macro_alias : 매크로 이름 별칭 (한국어 · 줄임말 → 표준 이름)
func markup_macro_alias() map[string]string {
	return map[string]string{
		"유튜브": "youtube", "유투브": "youtube", "yt": "youtube",
		"인스타그램": "instagram", "인스타": "instagram", "ig": "instagram",
		"네이버블로그": "naverblog", "네이버 블로그": "naverblog", "블로그": "naverblog",
		"티스토리": "tistory",
		"트위터":  "twitter", "엑스": "twitter",
		"페이스북":  "facebook",
		"틱톡":    "tiktok",
		"네이버티비": "navertv", "네이버tv": "navertv",
		"카카오티비": "kakaotv", "카카오tv": "kakaotv",
		"니코동": "nicovideo", "니코니코": "nicovideo",
		"비메오": "vimeo",
		"치지직": "chzzk",
		"계정":  "account", "소셜": "account",
		"글꼴": "font", "폰트": "font",
	}
}
