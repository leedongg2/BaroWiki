package tool

import (
	"database/sql"
	"strconv"
)

// Trash_prefix_of : 문서가 삭제될 때 이동할 휴지통 이름공간 접두사
func Trash_prefix_of(doc_name string) string {
	namespace := Namespace_of(doc_name)

	switch namespace.Prefix {
	case "trash:", "filetrash:":
		return ""
	case "file:":
		return "filetrash:"
	}

	return "trash:"
}

// Trash_title_make : 중복되지 않는 휴지통 문서 이름 생성
func Trash_title_make(db *sql.DB, doc_name string) string {
	trash_prefix := Trash_prefix_of(doc_name)
	if trash_prefix == "" {
		return ""
	}

	base_name := doc_name
	if trash_prefix == "filetrash:" {
		base_name = doc_name[len("file:"):]
	}

	trash_title := trash_prefix + base_name
	for index := 2; ; index++ {
		exist_title := ""
		if !QueryRow_DB(db, "select title from data where title = ? limit 1", []any{&exist_title}, trash_title) {
			return trash_title
		}

		trash_title = trash_prefix + base_name + " (" + strconv.Itoa(index) + ")"
	}
}

// Do_set_trash_info : 휴지통 문서 메타 정보 저장
func Do_set_trash_info(db *sql.DB, trash_title string, set_name string, set_data string) {
	Exec_DB(db, "delete from data_set where doc_name = ? and set_name = ?", trash_title, set_name)
	Exec_DB(
		db,
		"insert into data_set (doc_name, doc_rev, set_name, set_data) values (?, '', ?, ?)",
		trash_title,
		set_name,
		set_data,
	)
}

// Get_trash_info : 휴지통 문서 메타 정보 조회
func Get_trash_info(db *sql.DB, trash_title string, set_name string) string {
	set_data := ""
	QueryRow_DB(
		db,
		"select set_data from data_set where doc_name = ? and set_name = ? limit 1",
		[]any{&set_data},
		trash_title,
		set_name,
	)

	return set_data
}

// Do_move_to_trash : 삭제되는 문서를 휴지통 이름공간으로 이동
func Do_move_to_trash(db *sql.DB, doc_name string, raw_data string, ip string, why string) string {
	trash_title := Trash_title_make(db, doc_name)
	if trash_title == "" {
		return ""
	}

	trash_off := ""
	QueryRow_DB(db, "select data from other where name = 'trash_off'", []any{&trash_off})
	if trash_off != "" {
		return ""
	}

	now_time := Get_time()

	Exec_DB(db, "delete from data where title = ?", trash_title)
	Exec_DB(db, "insert into data (title, data, type) values (?, ?, '')", trash_title, raw_data)

	Do_set_trash_info(db, trash_title, "trash_origin", doc_name)
	Do_set_trash_info(db, trash_title, "trash_date", now_time)
	Do_set_trash_info(db, trash_title, "trash_ip", ip)
	Do_set_trash_info(db, trash_title, "trash_why", why)
	Do_set_trash_info(db, trash_title, "last_edit", now_time)
	Do_set_trash_info(db, trash_title, "length", strconv.Itoa(Get_len(raw_data)))

	return trash_title
}

// Do_trash_delete : 휴지통 문서 완전 삭제
func Do_trash_delete(db *sql.DB, trash_title string) {
	Exec_DB(db, "delete from data where title = ?", trash_title)
	Exec_DB(db, "delete from data_set where doc_name = ?", trash_title)
	Exec_DB(db, "delete from back where link = ?", trash_title)
	Exec_DB(db, "delete from back where title = ?", trash_title)
}

// Get_trash_list : 휴지통 문서 목록 (원본 문서 이름, 휴지통 문서 이름, 삭제 시간, 삭제자)
func Get_trash_list(db *sql.DB, trash_prefix string, start int, count int) [][]string {
	rows := Query_DB(
		db,
		"select title from data where title like ? order by title asc limit ?, ?",
		trash_prefix+"%",
		start,
		count,
	)
	defer rows.Close()

	data_list := [][]string{}
	for rows.Next() {
		trash_title := ""
		if rows.Scan(&trash_title) != nil {
			continue
		}

		data_list = append(data_list, []string{
			Get_trash_info(db, trash_title, "trash_origin"),
			trash_title,
			Get_trash_info(db, trash_title, "trash_date"),
			Get_trash_info(db, trash_title, "trash_ip"),
			Get_trash_info(db, trash_title, "trash_why"),
		})
	}

	return data_list
}
