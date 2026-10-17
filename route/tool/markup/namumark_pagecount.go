package markup

import (
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  [pagecount] · [pagecount(이름공간)]
//
//    [pagecount]            … 위키 전체 문서 수 (예전부터 있던 모양)
//    [pagecount()]          … 위키 전체 문서 수
//    [pagecount(휴지통)]     … 휴지통 이름공간의 문서 수
//    [pagecount(문서)]       … 이름공간이 없는 문서의 수
//    [pagecount(아무 말)]    … 이름공간이 아니면 전체 문서 수
//
//  이름공간 이름은 한국어 · 영어 · 일본어를 모두 받는다.
//  (`휴지통` · `trash` · `ごみびこ` 가 모두 같은 것)
// ============================================================

// compat_pagecount : 문서 수를 센다
func (class *namumark_compat_renderer) compat_pagecount(data string) string {
	if class.db == nil {
		return "0"
	}

	// 인자가 여럿이면 첫 번째만 본다
	name := ""
	if args := compat_split_macro_args(data); len(args) > 0 {
		name = args[0]
	}
	name = strings.TrimSpace(tool.HTML_unescape(name))

	// 이름공간 이름이 아니면 (빈 값 포함) 위키 전체를 센다
	if !tool.Namespace_exist_name(name) {
		return strconv.Itoa(tool.Namespace_all_document_count(class.db))
	}

	return strconv.Itoa(tool.Namespace_document_count(class.db, tool.Namespace_by_name(name)))
}
