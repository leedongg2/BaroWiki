package route

import (
	"strings"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

func Api_edit_revert_post(config tool.Config, doc_name string, rev string, send string, agree string) map[string]any {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	return_data := make(map[string]any)

	data := ""
	hide := ""
	exist := tool.QueryRow_DB(
		db,
		"select data, hide from history where title = ? and id = ?",
		[]any{&data, &hide},
		doc_name,
		rev,
	)

	if !exist {
		return_data["response"] = "not exist"

		return return_data
	} else if hide != "" && !tool.Check_acl(db, "", "", "hidel_auth", config.IP) {
		return_data["response"] = "require auth"

		return return_data
	} else if !tool.Check_acl(db, doc_name, "", "document_edit", config.IP) {
		return_data["response"] = "require auth"

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

	data = strings.ReplaceAll(data, "\r", "")

	old_data := ""
	old_exist := tool.QueryRow_DB(
		db,
		"select data from data where title = ?",
		[]any{&old_data},
		doc_name,
	)

	// 되돌려도 지금 본문과 똑같으면 판을 새로 만들지 않는다.
	// (저장 때와 같은 잣대를 쓴다 — 한쪽만 막으면 그쪽으로 빈 편집이 샌다)
	if old_exist && tool.Same_document_data(old_data, data) {
		return_data["response"] = "error"
		return_data["data"] = "same content"

		return return_data
	}

	length := tool.Get_edit_length_diff(old_data, data)
	if old_exist {
		tool.Exec_DB(
			db,
			"update data set data = ? where title = ?",
			data,
			doc_name,
		)
	} else {
		tool.Exec_DB(
			db,
			"insert into data (title, data) values (?, ?)",
			doc_name,
			data,
		)
	}

	tool.Do_add_history(
		db,
		doc_name,
		data,
		tool.Get_time(),
		config.IP,
		send,
		length,
		"revert",
		"r"+rev,
	)

	markup.Get_render(db, doc_name, data, "backlink")

	return_data["response"] = "ok"

	return return_data
}
