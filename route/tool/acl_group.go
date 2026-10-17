package tool

import (
	"database/sql"
	"strings"
)

// ============================================================
//  ACL 그룹
//
//  이용자와 아이피 대역을 이름 붙인 묶음으로 모아 둔다.
//  ACL 규칙에서 `거부:aclgroup:차단 대역` 처럼 한 줄로 부를 수 있다.
//
//  차단(rb)과는 다른 것이다.
//    차단   … 위키 전체에서 못 쓰게 막는다
//    ACL 그룹 … 특정 문서·이름공간에서만 막거나 열어 준다
//
//  한 사람을 여러 그룹에 넣을 수 있고, 그룹마다 만료일을 따로 줄 수 있다.
// ============================================================

// Acl_group_row : 그룹에 든 항목 하나
type Acl_group_row struct {
	ID      string
	Group   string
	Kind    string // user | ip
	Target  string // 계정 이름 또는 아이피·대역
	Note    string
	Actor   string
	Date    string
	EndDate string // "" 면 무기한
}

// Acl_group_kind_list : 넣을 수 있는 갈래
func Acl_group_kind_list() [][2]string {
	return [][2]string{
		{"user", "계정"},
		{"ip", "아이피 · 대역"},
	}
}

// Acl_group_name_list : 만들어져 있는 그룹 이름
func Acl_group_name_list(db *sql.DB) []string {
	rows := Query_DB(db, "select distinct group_name from baro_acl_group order by group_name asc")
	defer rows.Close()

	list := []string{}
	for rows.Next() {
		name := ""
		if rows.Scan(&name) != nil || strings.TrimSpace(name) == "" {
			continue
		}

		list = append(list, name)
	}

	return list
}

// Acl_group_list : 그룹 하나에 든 항목
func Acl_group_list(db *sql.DB, group string) []Acl_group_row {
	rows := Query_DB(
		db,
		"select id, group_name, kind, target, note, actor, date, end_date from baro_acl_group where group_name = ? order by id + 0 desc",
		group,
	)
	defer rows.Close()

	list := []Acl_group_row{}
	for rows.Next() {
		one := Acl_group_row{}
		if rows.Scan(&one.ID, &one.Group, &one.Kind, &one.Target, &one.Note, &one.Actor, &one.Date, &one.EndDate) != nil {
			continue
		}

		list = append(list, one)
	}

	return list
}

// Acl_group_add : 그룹에 하나 넣기
func Acl_group_add(db *sql.DB, group string, kind string, target string, note string, actor string, end_date string) {
	group = strings.TrimSpace(group)
	target = strings.TrimSpace(target)
	if group == "" || target == "" {
		return
	}

	if kind != "ip" {
		kind = "user"
	}

	Exec_DB(
		db,
		"insert into baro_acl_group (id, group_name, kind, target, note, actor, date, end_date) values (?, ?, ?, ?, ?, ?, ?, ?)",
		Baro_next_id(db, "baro_acl_group"),
		group, kind, target, Baro_trunc(note, 500), actor, Get_time(), end_date,
	)
}

// Acl_group_delete : 그룹에서 하나 빼기
func Acl_group_delete(db *sql.DB, id string) {
	Exec_DB(db, "delete from baro_acl_group where id = ?", id)
}

// Acl_group_has : 이 이용자가 그룹에 드는가
//
//	계정 이름이 같거나, 아이피가 적어 둔 주소·대역에 들면 참이다.
//	만료일이 지난 항목은 세지 않는다.
func Acl_group_has(db *sql.DB, group string, ip string) bool {
	group = strings.TrimSpace(group)
	if group == "" || ip == "" {
		return false
	}

	rows := Query_DB(db, "select kind, target, end_date from baro_acl_group where group_name = ?", group)
	defer rows.Close()

	for rows.Next() {
		kind, target, end_date := "", "", ""
		if rows.Scan(&kind, &target, &end_date) != nil {
			continue
		}

		if end_date != "" && Baro_is_past(end_date) {
			continue
		}

		if kind == "ip" {
			if Acl_ip_match(ip, target) {
				return true
			}

			continue
		}

		if strings.EqualFold(ip, strings.TrimSpace(target)) {
			return true
		}
	}

	return false
}
