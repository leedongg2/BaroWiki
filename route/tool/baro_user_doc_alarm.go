package tool

import (
	"database/sql"
	"strings"
)

// ============================================================
//  사용자 문서 알림
//
//  내 사용자 문서를 누가 편집하거나, 거기에 토론을 열면 알린다.
//  (바로위키 알림함 /baro/notify 와 openNAMU 알림함 /alarm 둘 다에 남긴다)
// ============================================================

// Baro_user_doc_owner : 사용자 문서의 주인 이름 (사용자 문서가 아니면 "")
//
//	"user:홍길동"        -> "홍길동"
//	"user:홍길동/연습장" -> "홍길동"
func Baro_user_doc_owner(doc_name string) string {
	namespace, rest := Namespace_split(doc_name)
	if namespace.Prefix != "user:" || rest == "" {
		return ""
	}

	if index := strings.Index(rest, "/"); index >= 0 {
		rest = rest[:index]
	}

	rest = strings.TrimSpace(rest)
	if rest == "" || !Baro_is_user(rest) {
		return ""
	}

	return rest
}

// Baro_user_doc_notify : 사용자 문서 주인에게 알린다
//
//	kind 는 "edit"(문서 편집) 또는 "topic"(토론 개설).
//	actor 가 주인 본인이면 알리지 않는다.
func Baro_user_doc_notify(db *sql.DB, actor string, doc_name string, kind string, url string) {
	owner := Baro_user_doc_owner(doc_name)
	if owner == "" || owner == actor {
		return
	}

	lang_key := "user_doc_edit_alarm"
	if kind == "topic" {
		lang_key = "user_doc_topic_alarm"
	}

	title := Namespace_display(doc_name) + " — " + Get_language(db, lang_key, true)

	if url == "" {
		url = "/w/" + Url_parser(doc_name)
	}

	// 바로위키 알림함
	Baro_notify(db, owner, "user_doc", title, url, actor)

	// openNAMU 알림함 (같은 내용을 링크로 남긴다)
	Send_alarm(
		db,
		actor,
		owner,
		`<a href="`+url+`">`+HTML_escape(Namespace_display(doc_name))+`</a> `+HTML_escape(Get_language(db, lang_key, true)),
	)
}
