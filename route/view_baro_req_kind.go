package route

import (
	"database/sql"
	"strconv"
	"strings"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

// ============================================================
//  요청 종류 나누기
//
//  예전에는 「편집 요청」 하나에 모든 부탁이 섞여 있었다.
//  부탁하는 일이 서로 달라서 적어야 하는 것도, 처리하는 권한도 다르다.
//  그래서 네 갈래로 나눈다.
//
//    edit   … 본문을 이렇게 바꿔 주세요      (문서 편집 권한으로 처리)
//    move   … 문서 이름을 바꿔 주세요        (문서 이동 권한으로 처리)
//    delete … 문서를 지워 주세요             (문서 삭제 권한으로 처리)
//    revert … 예전 판으로 되돌려 주세요      (문서 편집 권한으로 처리)
//
//  넷 다 같은 표(baro_edit_req)에 담기고 같은 화면에서 취합해 볼 수 있다.
//  kind 가 비어 있는 옛 기록은 전부 edit 으로 본다.
// ============================================================

// baro_req_kind_row : 요청 종류 하나
type baro_req_kind_row struct {
	Key   string // 열쇠말
	Name  string // 표시 이름
	Badge string // 뱃지 색
	Note  string // 한 줄 설명
	ACL   string // 처리에 필요한 권한
	Verb  string // 처리 단추 이름
}

// baro_req_kind_list : 요청 종류 목록
func baro_req_kind_list() []baro_req_kind_row {
	return []baro_req_kind_row{
		{
			Key: "edit", Name: tool.Baro_t("편집 요청"), Badge: "info", ACL: "document_edit",
			Note: tool.Baro_t("문서 본문을 이렇게 바꿔 달라고 부탁합니다."),
			Verb: tool.Baro_t("승인하고 반영"),
		},
		{
			Key: "move", Name: tool.Baro_t("이동 요청"), Badge: "warn", ACL: "document_move",
			Note: tool.Baro_t("문서 이름을 바꿔 달라고 부탁합니다."),
			Verb: tool.Baro_t("승인하고 이동"),
		},
		{
			Key: "delete", Name: tool.Baro_t("삭제 요청"), Badge: "danger", ACL: "document_delete",
			Note: tool.Baro_t("문서를 지워 달라고 부탁합니다."),
			Verb: tool.Baro_t("승인하고 삭제"),
		},
		{
			Key: "revert", Name: tool.Baro_t("되돌림 요청"), Badge: "muted", ACL: "document_edit",
			Note: tool.Baro_t("문서를 예전 판으로 되돌려 달라고 부탁합니다."),
			Verb: tool.Baro_t("승인하고 되돌림"),
		},
	}
}

// baro_req_kind_normalize : 빈 값·모르는 값은 edit 으로
func baro_req_kind_normalize(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	for _, one := range baro_req_kind_list() {
		if one.Key == kind {
			return kind
		}
	}

	return "edit"
}

// baro_req_kind_find : 종류 정보 찾기
func baro_req_kind_find(kind string) baro_req_kind_row {
	kind = baro_req_kind_normalize(kind)
	for _, one := range baro_req_kind_list() {
		if one.Key == kind {
			return one
		}
	}

	return baro_req_kind_list()[0]
}

// baro_req_kind_badge : 종류 뱃지
func baro_req_kind_badge(kind string) string {
	one := baro_req_kind_find(kind)

	return tool.Baro_badge(one.Name, one.Badge)
}

// baro_req_kind_count : 종류별 열려 있는 요청 수
func baro_req_kind_count(db *sql.DB) map[string]int {
	count := map[string]int{}
	for _, one := range baro_req_kind_list() {
		number := 0
		if one.Key == "edit" {
			// kind 가 비어 있는 옛 기록도 편집 요청으로 센다
			tool.QueryRow_DB(
				db,
				"select count(*) from baro_edit_req where status = 'open' and (kind = 'edit' or kind = '' or kind is null)",
				[]any{&number},
			)
		} else {
			tool.QueryRow_DB(db, "select count(*) from baro_edit_req where status = 'open' and kind = ?", []any{&number}, one.Key)
		}

		count[one.Key] = number
	}

	return count
}

// baro_req_kind_where : 종류로 거르는 조건과 값
func baro_req_kind_where(kind string) (string, []any) {
	if kind == "" {
		return "", nil
	}
	if kind == "edit" {
		return " (kind = 'edit' or kind = '' or kind is null)", nil
	}

	return " kind = ?", []any{kind}
}

// baro_req_can_handle_kind : 이 종류의 요청을 처리할 수 있는가
func baro_req_can_handle_kind(db *sql.DB, config tool.Config, kind string, title string) bool {
	if !tool.Baro_is_user(config.IP) {
		return false
	}

	return tool.Check_acl(db, title, "", baro_req_kind_find(kind).ACL, config.IP)
}

// baro_req_target_label : 종류마다 target 칸이 뜻하는 것
func baro_req_target_label(kind string) string {
	switch baro_req_kind_normalize(kind) {
	case "move":
		return tool.Baro_t("새 문서 이름")
	case "revert":
		return tool.Baro_t("되돌릴 판")
	}

	return ""
}

// baro_req_apply_kind : 승인된 요청을 실제로 반영한다
//
//	4번 원칙 : 반영된 결과의 주인은 부탁한 사람이다.
//	돌려주는 값은 (새 판 번호, 오류 사유) 이며 오류 사유가 비어 있으면 성공이다.
func baro_req_apply_kind(db *sql.DB, config tool.Config, row map[string]string, reason string) (string, string) {
	kind := baro_req_kind_normalize(row["kind"])
	title := row["title"]

	// 처리한 사람과 사유를 편집 요약에 남긴다
	send := baro_req_kind_find(kind).Name + " #" + row["id"] + tool.Baro_t(" 승인 (처리 : ") + config.IP + ")"
	if reason != "" {
		send += " — " + reason
	}
	send = tool.Baro_trunc(send, 400)

	switch kind {
	case "move":
		new_name := tool.Namespace_normalize(strings.TrimSpace(row["target"]))
		if new_name == "" {
			return "", tool.Baro_t("옮길 이름이 비어 있습니다.")
		}
		if new_name == title {
			return "", tool.Baro_t("지금 이름과 같습니다.")
		}
		if err_name := move_document_options(config, db, title, new_name, send, "normal", "normal", "normal"); err_name != "" {
			return "", tool.Baro_t("옮기지 못했습니다 : ") + err_name
		}

		return tool.Get_document_revision(db, new_name), ""

	case "delete":
		raw_data := ""
		if !tool.QueryRow_DB(db, "select data from data where title = ?", []any{&raw_data}, title) {
			return "", tool.Baro_t("이미 지워진 문서입니다.")
		}

		tool.Do_add_history(
			db, title, "", tool.Get_time(), row["user"], send,
			"-"+strconv.Itoa(tool.Get_len(raw_data)), "delete", "",
		)
		tool.Do_move_to_trash(db, title, raw_data, row["user"], send)
		baro_req_clear_backlink(db, title)
		tool.Exec_DB(db, "delete from data where title = ?", title)

		return tool.Get_document_revision(db, title), ""

	case "revert":
		revision := strings.TrimPrefix(strings.TrimSpace(row["target"]), "r")
		if revision == "" {
			return "", tool.Baro_t("되돌릴 판 번호가 비어 있습니다.")
		}

		old_body := ""
		hide := ""
		if !tool.QueryRow_DB(db, "select data, hide from history where title = ? and id = ?", []any{&old_body, &hide}, title, revision) {
			return "", tool.Baro_t("그 판을 찾지 못했습니다.")
		}
		if hide != "" {
			return "", tool.Baro_t("가려진 판으로는 되돌릴 수 없습니다.")
		}

		old_body = strings.ReplaceAll(old_body, "\r", "")
		now_body := ""
		exists := tool.QueryRow_DB(db, "select data from data where title = ?", []any{&now_body}, title)
		if exists && tool.Same_document_data(now_body, old_body) {
			return "", tool.Baro_t("지금 본문과 같습니다. 되돌릴 것이 없습니다.")
		}

		length := tool.Get_edit_length_diff(now_body, old_body)
		if exists {
			tool.Exec_DB(db, "update data set data = ? where title = ?", old_body, title)
		} else {
			tool.Exec_DB(db, "insert into data (title, data) values (?, ?)", title, old_body)
		}

		tool.Do_watchlist_alarm_send(db, config, title)
		tool.Do_add_history(db, title, old_body, tool.Get_time(), row["user"], send, length, "revert", "r"+revision)
		markup.Get_render(db, title, old_body, "backlink")
		tool.Baro_hook_after_edit(db, tool.Config{IP: row["user"]}, title, now_body, old_body, send)

		return tool.Get_document_revision(db, title), ""
	}

	// edit
	new_body := row["data"]
	old_body := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&old_body}, title)
	if tool.Same_document_data(old_body, new_body) {
		return "", tool.Baro_t("지금 본문과 같습니다. 반영할 것이 없습니다.")
	}

	length := tool.Get_edit_length_diff(old_body, new_body)

	tool.Exec_DB(db, "delete from data where title = ?", title)
	tool.Exec_DB(db, "insert into data (title, data) values (?, ?)", title, new_body)

	tool.Do_watchlist_alarm_send(db, config, title)
	tool.Do_add_history(db, title, new_body, tool.Get_time(), row["user"], send, length, "", "")
	markup.Get_render(db, title, new_body, "backlink")
	tool.Baro_hook_after_edit(db, tool.Config{IP: row["user"]}, title, old_body, new_body, send)

	return tool.Get_document_revision(db, title), ""
}

// baro_req_clear_backlink : 문서를 지울 때 역링크를 「없는 문서」 로 돌린다
func baro_req_clear_backlink(db *sql.DB, doc_name string) {
	rows := tool.Query_DB(
		db,
		"select title, link from back where title = ? and not type = 'cat' and not type = 'no'",
		doc_name,
	)

	link_list := [][]string{}
	for rows.Next() {
		title, link := "", ""
		if rows.Scan(&title, &link) != nil {
			continue
		}

		link_list = append(link_list, []string{title, link})
	}
	rows.Close()

	for _, one := range link_list {
		tool.Exec_DB(db, "insert into back (title, link, type, data) values (?, ?, 'no', '')", one[0], one[1])
	}

	tool.Exec_DB(db, "delete from back where link = ?", doc_name)
}
