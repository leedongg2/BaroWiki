package route

import (
	"strings"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

func Api_edit_post(config tool.Config, doc_name string, data string, send string, agree string, expected_revision string) map[string]any {
	// 판 확인부터 저장과 역사 기록까지 같은 문서의 편집을 순서대로 처리한다.
	unlock := tool.Lock_document_edit(doc_name)
	defer unlock()
	db := tool.DB_connect()
	defer tool.DB_close(db)

	return_data := make(map[string]any)

	date := tool.Get_time()
	data = strings.ReplaceAll(data, "\r", "")
	current_revision := tool.Get_document_revision(db, doc_name)

	if doc_name == "" {
		return_data["response"] = "error"
		return_data["data"] = "empty title"

		return return_data
	} else if !tool.Do_title_length_check(db, doc_name, "document") {
		return_data["response"] = "error"
		return_data["data"] = "title length"

		return return_data
	} else if !tool.Check_acl(db, doc_name, "", "document_edit", config.IP) {
		return_data["response"] = "error"
		return_data["data"] = "permission denied"

		return return_data
	} else if current_revision == "0" && !tool.Check_acl(db, doc_name, "", "document_make_acl", config.IP) {
		return_data["response"] = "error"
		return_data["data"] = "permission denied"

		return return_data
	} else if expected_revision != "" && expected_revision != current_revision {
		return_data["response"] = "error"
		return_data["data"] = "edit conflict"

		return return_data
	} else if !tool.Do_edit_slow_check(db, config, "edit") {
		return_data["response"] = "error"
		return_data["data"] = "slow edit limit"

		return return_data
	} else if !tool.Do_edit_filter(db, config, doc_name, data) {
		return_data["response"] = "error"
		return_data["data"] = "edit filter (content)"

		return return_data
	} else if !tool.Do_edit_filter(db, config, doc_name, send) {
		return_data["response"] = "error"
		return_data["data"] = "edit filter (send)"

		return return_data
	} else if !tool.Do_edit_send_require_check(db, config, send) {
		return_data["response"] = "error"
		return_data["data"] = "send require"

		return return_data
	} else if !tool.Do_edit_text_checkbox_check(db, config, agree) {
		return_data["response"] = "error"
		return_data["data"] = "checkbox check require"

		return return_data
	} else if !tool.Do_edit_max_length_check(db, config, data) {
		return_data["response"] = "error"
		return_data["data"] = "overflow max length"

		return return_data
	}

	// 바로위키 : 점검 중 / 격리 / 제재 / 편집 필터 검사
	if baro_reason, baro_ok := tool.Baro_hook_before_edit(db, config, doc_name, data, send); !baro_ok {
		return_data["response"] = "error"
		return_data["data"] = "baro:" + baro_reason

		return return_data
	}

	var old_data string

	document_exists := tool.QueryRow_DB(
		db,
		`select data from data where title = ?`,
		[]any{&old_data},
		doc_name,
	)

	// 내용이 하나도 바뀌지 않았으면 판을 새로 만들지 않는다.
	// (빈 편집으로 기여 기록만 늘리는 것을 막는다)
	//
	// 새로 들어온 글은 위에서 \r 을 지웠지만, 옛날에 들여온 문서에는 \r 이 남아 있을 수 있다.
	// 견줄 때 양쪽을 같은 모양으로 맞추지 않으면 「바뀐 게 없는데 늘 다르다」가 된다.
	if document_exists && tool.Same_document_data(old_data, data) {
		return_data["response"] = "error"
		return_data["data"] = "same content"

		return return_data
	}

	length := tool.Get_edit_length_diff(old_data, data)

	if err := tool.Replace_document_data(db, doc_name, data); err != nil {
		panic(err)
	}

	tool.Do_watchlist_alarm_send(db, config, doc_name)

	// 바로위키 : 편집방에 들어와 있는 동안 한 편집이면 요약 뒤에 방과 함께한 사람을 적는다
	send = tool.Baro_coop_edit_mark(db, config.IP, doc_name, send)

	tool.Do_add_history(
		db,
		doc_name,
		data,
		date,
		config.IP,
		send,
		length,
		"",
		"",
	)

	markup.Get_render(db, doc_name, data, "backlink")

	// 바로위키 : 기여 이벤트 · 출처 · 검토 큐 · 지표 기록
	tool.Baro_hook_after_edit(db, config, doc_name, old_data, data, send)

	return_data["response"] = "ok"

	return return_data
}
