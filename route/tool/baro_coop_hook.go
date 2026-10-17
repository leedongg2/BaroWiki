package tool

import (
	"database/sql"
	"strings"
)

// ============================================================
//  편집방과 문서 편집을 잇는 고리
//
//  방에 편집 자리로 들어와 있는 동안 한 편집은 그 방에서 한 편집으로 잡힌다.
//  편집 화면을 따로 두지 않는다. 여느 때처럼 문서를 고치면 방이 알아서 적어 둔다.
//
//  방이 닫히면 이 고리는 아무 일도 하지 않는다.
//  그래서 「편집 중에 방이 닫혀도 이어서 편집할 수 있고, 그 편집은 방 밖에서 한 편집으로 남는다」.
// ============================================================

// Baro_coop_now : 이 사람이 지금 편집 자리로 들어와 있는 방
//
//	여러 방에 이름이 올라 있으면 가장 늦게 들어온 방을 본다.
//	관전 자리이거나 편집이 막힌 사람은 방에 들어와 있어도 잡지 않는다.
func Baro_coop_now(db *sql.DB, user string) (Baro_coop_room_row, bool) {
	if !Baro_is_user(user) {
		return Baro_coop_room_row{}, false
	}

	row, ok := baro_coop_room_one(
		db,
		baro_coop_room_select+" where id in ("+
			"select room from baro_coop_member where user = ? and status = 'in' "+
			"and role <> 'watch' and block <> 'on') "+
			"and status <> 'closed' order by id + 0 desc limit 1",
		user,
	)
	if !ok {
		return row, false
	}

	row = baro_coop_room_tick(db, row)
	if !row.Baro_coop_room_live() {
		return row, false
	}

	return row, true
}

// Baro_coop_edit_mark : 방에서 한 편집임을 편집 요약 뒤에 붙인다
//
//	붙는 모양 :
//	  (원래 요약) [편집방: 방 이름 · 함께한 사람 A, B, C · /baro/cooperation/이름표]
//
//	함께한 사람은 그때 방에 편집 자리로 있던 사람 모두다.
//	편집 자체를 누른 사람은 history 의 글쓴이 자리에 그대로 남으므로,
//	여기서는 「누구와 함께 했는가」 와 「어느 방이었는가」 를 남긴다.
func Baro_coop_edit_mark(db *sql.DB, user string, doc_name string, send string) string {
	defer func() {
		_ = recover()
	}()

	room, found := Baro_coop_now(db, user)
	if !found {
		return send
	}

	// 손댄 문서로 적어 둔다. 같은 문서를 또 고치면 횟수가 올라간다.
	num := Baro_coop_doc_bump(db, room.ID, doc_name)

	// 그때 방에 있던 편집 인원
	names := []string{}
	for _, member := range Baro_coop_member_list(db, room.ID, "in") {
		if !member.Baro_coop_member_editing() || member.Block {
			continue
		}

		names = append(names, member.User)
	}

	Baro_coop_log_add(
		db, room.ID, "edit", user,
		doc_name, Baro_itoa(num)+"번째 · 함께한 사람 "+strings.Join(names, ", "),
	)

	mark := "[" + Baro_t("편집방") + ": " + room.Name
	if len(names) > 0 {
		mark += " · " + strings.Join(names, ", ")
	}
	mark += " · /baro/cooperation/" + room.Link + "]"

	if strings.TrimSpace(send) == "" {
		return mark
	}

	return send + " " + mark
}

// Baro_coop_block_check : 이 사람이 이 방에서 편집을 막혔는가
func Baro_coop_block_check(db *sql.DB, room string, user string) bool {
	member, found := Baro_coop_member_get(db, room, user)

	return found && member.Block
}
