package tool

import (
	"database/sql"
	"strings"
	"sync/atomic"
	"time"
)

// Namespace : 위키 이름공간 (나무위키 형식)
//
// Prefix   : 내부 저장용 접두사 (기본 이름공간은 "")
// Name     : 내부 키로도 쓰는 한국어 이름 ("문서", "사용자" 등)
//
//	ACL 저장 키(namespace:<Name>)가 이 값에 묶여 있으므로 절대 바꾸지 않는다.
//
// Names    : 언어별 표시 이름 ("ko-KR" / "en-US" / "ja-JP")
// Aliases  : 입력에서 허용하는 접두사 목록 (전부 소문자 비교)
// Defaults : 이름공간 기본 ACL (문서/이름공간 ACL이 없을 때 적용)
type Namespace struct {
	Prefix   string
	Name     string
	Names    map[string]string
	Aliases  []string
	Defaults map[string]string
}

// Namespace_acl_type_list : 이름공간 및 문서 ACL 종류
func Namespace_acl_type_list() []string {
	return []string{
		"view",
		"decu",
		"document_edit_acl",
		"document_move_acl",
		"document_delete_acl",
		"dis",
		"acl_edit",
	}
}

// Namespace_language_list : 이름공간 이름을 제공하는 언어 목록
//
//	한국어판 · 영어판 · 일본어판을 나누어 운영하기 위한 기준 언어다.
func Namespace_language_list() []string {
	return []string{"ko-KR", "en-US", "ja-JP"}
}

// namespace_lang_now : 지금 위키가 쓰는 언어 (Get_language 가 갱신한다)
var namespace_lang_now atomic.Value

// Set_lang_now : 현재 언어 캐시 갱신
func Set_lang_now(language string) {
	if language == "" {
		return
	}

	namespace_lang_now.Store(language)
}

// Lang_now : 현재 언어 (모르면 ko-KR)
func Lang_now() string {
	if data, ok := namespace_lang_now.Load().(string); ok && data != "" {
		return data
	}

	return "ko-KR"
}

// Wiki_language : 위키에 설정된 언어를 읽고 캐시까지 갱신
func Wiki_language(db *sql.DB) string {
	language := Wiki_setting(db, "language")
	if language == "" {
		language = "ko-KR"
	}

	Set_lang_now(language)

	return language
}

// namespace_names : 언어별 이름 묶음을 만드는 도우미
func namespace_names(ko string, en string, ja string) map[string]string {
	return map[string]string{
		"ko-KR": ko,
		"en-US": en,
		"ja-JP": ja,
	}
}

// namespace_list : 이름공간 정의
//
//	접두사가 서로를 잡아먹지 않도록 긴 접두사를 먼저 둔다. (filetrash: 가 file: 보다 앞)
var namespace_list = []Namespace{
	{
		Prefix:   "",
		Name:     "문서",
		Names:    namespace_names("문서", "document", "ぶんしょう"),
		Aliases:  []string{"문서:", "document:", "ぶんしょう:", "文書:"},
		Defaults: map[string]string{},
	},
	{
		Prefix:  "filetrash:",
		Name:    "파일휴지통",
		Names:   namespace_names("파일휴지통", "file_trash", "ファイルごみびこ"),
		Aliases: []string{"파일휴지통:", "파일 휴지통:", "filetrash:", "file_trash:", "ファイルごみびこ:", "ファイルごみばこ:", "ファイルゴミ箱:"},
		Defaults: map[string]string{
			"view":                "admin",
			"document_edit_acl":   "owner",
			"document_move_acl":   "owner",
			"document_delete_acl": "admin",
			"dis":                 "owner",
			"acl_edit":            "owner",
		},
	},
	{
		Prefix:  "trash:",
		Name:    "휴지통",
		Names:   namespace_names("휴지통", "trash", "ごみびこ"),
		Aliases: []string{"휴지통:", "trash:", "ごみびこ:", "ごみばこ:", "ゴミ箱:"},
		Defaults: map[string]string{
			"view":                "admin",
			"document_edit_acl":   "owner",
			"document_move_acl":   "owner",
			"document_delete_acl": "admin",
			"dis":                 "owner",
			"acl_edit":            "owner",
		},
	},
	{
		Prefix:  "deleteduser:",
		Name:    "삭제된사용자",
		Names:   namespace_names("삭제된사용자", "deleted_user", "たいかいしたユーザー"),
		Aliases: []string{"삭제된사용자:", "삭제된 사용자:", "deleteduser:", "deleted_user:", "たいかいしたユーザー:", "退会したユーザー:"},
		Defaults: map[string]string{
			"document_edit_acl":   "admin",
			"document_move_acl":   "admin",
			"document_delete_acl": "admin",
			"acl_edit":            "admin",
		},
	},
	{
		Prefix:  "ipuser:",
		Name:    "아이피사용자",
		Names:   namespace_names("아이피사용자", "ip_user", "アイピーユーザー"),
		Aliases: []string{"아이피사용자:", "아이피 사용자:", "ipuser:", "ip_user:", "アイピーユーザー:", "IPユーザー:"},
		Defaults: map[string]string{
			"document_edit_acl":   "admin",
			"document_move_acl":   "owner",
			"document_delete_acl": "admin",
			"acl_edit":            "admin",
		},
	},
	{
		Prefix:  "user:",
		Name:    "사용자",
		Names:   namespace_names("사용자", "user", "ユーザー"),
		Aliases: []string{"사용자:", "user:", "ユーザー:", "利用者:"},
		Defaults: map[string]string{
			"document_move_acl":   "admin",
			"document_delete_acl": "admin",
		},
	},
	{
		Prefix:  "file:",
		Name:    "파일",
		Names:   namespace_names("파일", "file", "ファイル"),
		Aliases: []string{"파일:", "file:", "ファイル:"},
		Defaults: map[string]string{
			"document_move_acl":   "admin",
			"document_delete_acl": "admin",
		},
	},
	{
		Prefix:  "manage:",
		Name:    "위키운영",
		Names:   namespace_names("위키운영", "manage", "うんえい"),
		Aliases: []string{"위키운영:", "위키 운영:", "manage:", "wiki_manage:", "うんえい:", "運営:"},
		Defaults: map[string]string{
			"document_edit_acl":   "admin",
			"document_move_acl":   "admin",
			"document_delete_acl": "admin",
			"acl_edit":            "admin",
		},
	},
	{
		Prefix:  "baro:",
		Name:    "바로위키",
		Names:   namespace_names("바로위키", "barowiki", "バロウィキ"),
		Aliases: []string{"바로위키:", "baro:", "barowiki:", "バロウィキ:"},
		Defaults: map[string]string{
			"document_move_acl":   "admin",
			"document_delete_acl": "admin",
		},
	},
	{
		Prefix:  "template:",
		Name:    "틀",
		Names:   namespace_names("틀", "template", "テンプレート"),
		Aliases: []string{"틀:", "template:", "テンプレート:"},
		Defaults: map[string]string{
			"document_move_acl":   "admin",
			"document_delete_acl": "admin",
		},
	},
	{
		Prefix:  "boilerplate:",
		Name:    "템플릿",
		Names:   namespace_names("템플릿", "boilerplate", "はながた"),
		Aliases: []string{"템플릿:", "boilerplate:", "bolilerplate:", "はながた:", "ひながた:", "雛形:"},
		Defaults: map[string]string{
			"document_move_acl":   "admin",
			"document_delete_acl": "admin",
		},
	},
	{
		Prefix:   "word:",
		Name:     "낱말",
		Names:    namespace_names("낱말", "word", "たんご"),
		Aliases:  []string{"낱말:", "word:", "たんご:", "単語:"},
		Defaults: map[string]string{},
	},
	{
		Prefix:   "vote:",
		Name:     "투표",
		Names:    namespace_names("투표", "vote", "とうひょう"),
		Aliases:  []string{"투표:", "vote:", "とうひょう:", "投票:"},
		Defaults: map[string]string{},
	},
	{
		Prefix:  "editfilter:",
		Name:    "편집필터",
		Names:   namespace_names("편집필터", "edit_filter", "へんしゅうフィルター"),
		Aliases: []string{"편집필터:", "편집 필터:", "editfilter:", "edit_filter:", "へんしゅうフィルター:", "編集フィルター:"},
		Defaults: map[string]string{
			"document_edit_acl":   "admin",
			"document_move_acl":   "admin",
			"document_delete_acl": "admin",
			"acl_edit":            "admin",
		},
	},
	{
		Prefix:   "project:",
		Name:     "프로젝트",
		Names:    namespace_names("프로젝트", "project", "プロジェクト"),
		Aliases:  []string{"프로젝트:", "project:", "プロジェクト:"},
		Defaults: map[string]string{},
	},
	{
		Prefix:  "help:",
		Name:    "도움말",
		Names:   namespace_names("도움말", "help", "ヘルプ"),
		Aliases: []string{"도움말:", "help:", "ヘルプ:"},
		Defaults: map[string]string{
			"document_move_acl":   "admin",
			"document_delete_acl": "admin",
		},
	},
	{
		Prefix:   "discussion:",
		Name:     "토론",
		Names:    namespace_names("토론", "discussion", "ぎろん"),
		Aliases:  []string{"토론:", "discussion:", "ぎろん:", "議論:"},
		Defaults: map[string]string{},
	},
	{
		Prefix:   "category:",
		Name:     "분류",
		Names:    namespace_names("분류", "category", "ぶんるい"),
		Aliases:  []string{"분류:", "category:", "ぶんるい:", "分類:"},
		Defaults: map[string]string{},
	},
}

// Namespace_list : 전체 이름공간 목록
func Namespace_list() []Namespace {
	return namespace_list
}

// Namespace_default : 기본(문서) 이름공간
func Namespace_default() Namespace {
	return namespace_list[0]
}

// Namespace_name : 언어에 맞는 이름공간 표시 이름
func Namespace_name(namespace Namespace, language string) string {
	if namespace.Names != nil {
		if name, ok := namespace.Names[language]; ok && name != "" {
			return name
		}
	}

	return namespace.Name
}

// Namespace_name_now : 현재 언어의 이름공간 표시 이름
func Namespace_name_now(namespace Namespace) string {
	return Namespace_name(namespace, Lang_now())
}

// Namespace_by_prefix : 내부 접두사로 이름공간 찾기
func Namespace_by_prefix(prefix string) Namespace {
	for _, namespace := range namespace_list {
		if namespace.Prefix == prefix {
			return namespace
		}
	}

	return Namespace_default()
}

// Namespace_by_name : 표시 이름으로 이름공간 찾기 (세 언어 이름을 모두 받는다)
func Namespace_by_name(name string) Namespace {
	for _, namespace := range namespace_list {
		if namespace.Name == name {
			return namespace
		}

		for _, one := range namespace.Names {
			if one == name {
				return namespace
			}
		}
	}

	return Namespace_default()
}

// Namespace_exist_name : 표시 이름이 실제 이름공간인지 확인 (세 언어 이름을 모두 받는다)
func Namespace_exist_name(name string) bool {
	for _, namespace := range namespace_list {
		if namespace.Name == name {
			return true
		}

		for _, one := range namespace.Names {
			if one == name {
				return true
			}
		}
	}

	return false
}

// Namespace_of : 문서 이름이 속한 이름공간
func Namespace_of(doc_name string) Namespace {
	lower_name := strings.ToLower(doc_name)

	for _, namespace := range namespace_list {
		if namespace.Prefix == "" {
			continue
		}

		if strings.HasPrefix(lower_name, namespace.Prefix) {
			return namespace
		}
	}

	return Namespace_default()
}

// Namespace_split : 문서 이름을 (이름공간, 이름공간 제외 이름)으로 분리
func Namespace_split(doc_name string) (Namespace, string) {
	namespace := Namespace_of(doc_name)

	return namespace, doc_name[len(namespace.Prefix):]
}

// Namespace_normalize : 입력된 문서 이름의 이름공간 접두사를 내부 형식으로 변환
//
//	한국어 · 영어 · 일본어 접두사를 모두 같은 내부 이름으로 모은다.
func Namespace_normalize(doc_name string) string {
	doc_name = strings.TrimSpace(doc_name)
	if doc_name == "" {
		return doc_name
	}

	lower_name := strings.ToLower(doc_name)
	for _, namespace := range namespace_list {
		for _, alias := range namespace.Aliases {
			if !strings.HasPrefix(lower_name, alias) {
				continue
			}

			rest := doc_name[len(alias):]
			if rest == "" {
				continue
			}

			return namespace.Prefix + rest
		}
	}

	return doc_name
}

// Namespace_display_lang : 내부 문서 이름을 해당 언어의 표시 이름으로 변환
func Namespace_display_lang(doc_name string, language string) string {
	if doc_name == "" {
		return doc_name
	}

	namespace, rest := Namespace_split(doc_name)
	if namespace.Prefix == "" || rest == "" {
		return doc_name
	}

	return Namespace_name(namespace, language) + ":" + Namespace_display_lang(rest, language)
}

// Namespace_display : 내부 문서 이름을 현재 언어의 표시 이름으로 변환
func Namespace_display(doc_name string) string {
	return Namespace_display_lang(doc_name, Lang_now())
}

// Namespace_display_db : 위키 언어를 직접 확인해서 표시 이름으로 변환
func Namespace_display_db(db *sql.DB, doc_name string) string {
	return Namespace_display_lang(doc_name, Wiki_language(db))
}

// Doc_url : 문서 주소에 넣을 이름
//
//	저장되는 제목은 `file:그림.png` 처럼 내부 접두사를 쓰지만,
//	이용자에게 보이는 주소는 `파일:그림.png` 로 통일한다.
//	화면에 보이는 이름과 주소가 달라 보이지 않게 하기 위함이다.
//
//	주소로 들어온 한국어 접두사는 Namespace_normalize 가 다시 내부 이름으로 되돌리므로
//	예전에 만들어 둔 영어 접두사 주소도 그대로 열린다.
func Doc_url(doc_name string) string {
	return Url_parser(Namespace_display(doc_name))
}

// Namespace_acl_key : 이름공간 ACL 저장에 사용하는 키
//
//	언어와 무관하게 한국어 이름을 쓴다. (언어를 바꿔도 ACL 이 그대로 남아야 한다)
func Namespace_acl_key(namespace Namespace) string {
	return "namespace:" + namespace.Name
}

// Get_namespace_acl : 이름공간에 설정된 ACL 값
func Get_namespace_acl(db *sql.DB, namespace Namespace, acl_type string) string {
	acl_data := ""
	QueryRow_DB(
		db,
		"select data from acl where title = ? and type = ? limit 1",
		[]any{&acl_data},
		Namespace_acl_key(namespace),
		acl_type,
	)

	return acl_data
}

// Do_set_namespace_acl : 이름공간 ACL 저장
func Do_set_namespace_acl(db *sql.DB, namespace Namespace, acl_type string, acl_data string) {
	acl_key := Namespace_acl_key(namespace)

	Exec_DB(db, "delete from acl where title = ? and type = ?", acl_key, acl_type)
	if acl_data != "" {
		Exec_DB(db, "insert into acl (title, data, type) values (?, ?, ?)", acl_key, acl_data, acl_type)
	}
}

// Get_acl_expire : 문서 ACL 만료일 (없으면 "")
func Get_acl_expire(db *sql.DB, doc_name string, acl_type string) string {
	expire_date := ""
	QueryRow_DB(
		db,
		"select set_data from data_set where doc_name = ? and doc_rev = ? and set_name = 'acl_date' limit 1",
		[]any{&expire_date},
		doc_name,
		acl_type,
	)

	return expire_date
}

// Get_acl_expired : ACL 만료 여부 확인
func Get_acl_expired(expire_date string) bool {
	if expire_date == "" {
		return false
	}

	expire_time, err := time.Parse("2006-01-02", expire_date)
	if err != nil {
		return false
	}

	// 만료일 당일까지는 유지
	expire_time = expire_time.AddDate(0, 0, 1)

	return time.Now().After(expire_time)
}

// Get_document_acl_source : ACL 값과 출처 (document / namespace / default) 조회
func Get_document_acl_source(db *sql.DB, doc_name string, acl_type string) (string, string) {
	if doc_name == "" {
		return "", ""
	}

	acl_data := ""
	QueryRow_DB(
		db,
		"select data from acl where title = ? and type = ? limit 1",
		[]any{&acl_data},
		doc_name,
		acl_type,
	)

	if acl_data != "" {
		if !Get_acl_expired(Get_acl_expire(db, doc_name, acl_type)) {
			return acl_data, "document"
		}
	}

	namespace := Namespace_of(doc_name)
	if namespace_acl := Get_namespace_acl(db, namespace, acl_type); namespace_acl != "" {
		return namespace_acl, "namespace"
	}

	if default_acl, ok := namespace.Defaults[acl_type]; ok {
		return default_acl, "default"
	}

	return "", ""
}

// Get_document_acl : 문서 ACL -> 이름공간 ACL -> 이름공간 기본값 순으로 조회
func Get_document_acl(db *sql.DB, doc_name string, acl_type string) string {
	acl_data, _ := Get_document_acl_source(db, doc_name, acl_type)

	return acl_data
}

// Namespace_document_count : 이름공간에 속한 문서 수
// Namespace_all_document_count : 위키 전체 문서 수
//
//	이름공간을 가리지 않고 센다. 넘겨주기·없는 문서 같은 것은 빼는 기준을
//	이름공간별 집계와 똑같이 맞춘다.
func Namespace_all_document_count(db *sql.DB) int {
	count := 0
	QueryRow_DB(db, "select count(*) from data", []any{&count})

	return count
}

func Namespace_document_count(db *sql.DB, namespace Namespace) int {
	count := 0

	if namespace.Prefix == "" {
		QueryRow_DB(
			db,
			"select count(*) from data where "+Get_except_document_name_SQL("title"),
			[]any{&count},
		)
	} else {
		QueryRow_DB(
			db,
			"select count(*) from data where title like ?",
			[]any{&count},
			namespace.Prefix+"%",
		)
	}

	return count
}

// Get_title_html : 문서 제목 HTML (나무위키처럼 이름공간 부분을 강조)
func Get_title_html(doc_name string) string {
	display_name := Namespace_display(doc_name)
	if display_name == "" {
		return ""
	}

	namespace := Namespace_of(doc_name)
	if namespace.Prefix == "" {
		return HTML_escape(display_name)
	}

	prefix := Namespace_name_now(namespace) + ":"
	if !strings.HasPrefix(display_name, prefix) {
		return HTML_escape(display_name)
	}

	return `<span class="namu_title_namespace">` + HTML_escape(prefix) + `</span>` + HTML_escape(display_name[len(prefix):])
}

// Get_title_link_html : 문서 제목을 그 문서로 가는 링크로 감싼다
//
//	나무위키처럼 문서 화면의 큰 제목을 누르면 그 문서로 다시 간다.
//	(주소에 붙은 판·앵커·물음표가 떨어져 나가므로 「지금 문서로 돌아가기」 로도 쓰인다)
//
//	link_target 이 비어 있으면 감싸지 않는다.
//	「최근 변경」 처럼 문서가 아닌 화면의 제목까지 링크가 되면 안 되기 때문에,
//	문서 화면이 스스로 이 값을 넘겨 줄 때만 링크가 된다.
func Get_title_link_html(doc_name string, link_target string) string {
	title_html := Get_title_html(doc_name)
	if title_html == "" || link_target == "" {
		return title_html
	}

	return `<a class="namu_title_link" href="/w/` + Doc_url(link_target) + `">` + title_html + `</a>`
}
