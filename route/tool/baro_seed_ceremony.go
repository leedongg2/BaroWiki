package tool

import (
	"database/sql"
	"strings"
)

// ============================================================
//  위키 수상식 기본 심기
//
//  - 대문에 위키 수상식 안내 한 칸을 넣는다. (한 번만 · 지우면 다시 넣지 않는다)
//  - 안내 문서와 분류 문서를 만든다.
// ============================================================

// Baro_frontpage_title : 대문 문서 이름
func Baro_frontpage_title(db *sql.DB) string {
	frontpage := "FrontPage"
	QueryRow_DB(db, `select data from other where name = "frontpage"`, []any{&frontpage})
	if strings.TrimSpace(frontpage) == "" {
		frontpage = "FrontPage"
	}

	return Namespace_normalize(strings.TrimSpace(frontpage))
}

// baro_ceremony_frontpage_block : 대문에 붙일 안내 (나무마크)
func baro_ceremony_frontpage_block() string {
	return "\n\n== 위키 수상식 ==\n" +
		"바로위키 운영자가 여는 수상식입니다. 가입한 사용자면 누구나 출마하고 투표할 수 있습니다.\n" +
		" * 수상식 목록과 투표 : {{{#!html <a href=\"/baro/ceremony\">위키 수상식 바로가기</a>}}}\n" +
		" * 진행 방법 : [[바로위키:위키 수상식]]\n" +
		" * 지난 수상식 : [[분류:위키 수상식]]\n"
}

// Baro_seed_ceremony : 위키 수상식 관련 기본 내용 심기
func Baro_seed_ceremony(db *sql.DB) {
	done := ""
	if QueryRow_DB(db, "select name from baro_set where name = 'seed_ceremony'", []any{&done}) {
		return
	}

	// --- 안내 문서 ---
	for _, row := range [][]string{
		{
			"baro:위키 수상식",
			"= 위키 수상식 =\n" +
				"바로위키의 기여를 함께 돌아보는 자리입니다.\n" +
				"\n" +
				"== 누가 무엇을 할 수 있나 ==\n" +
				" * '''개최''' : 운영자만 할 수 있습니다.\n" +
				" * '''출마·추천''' : 가입한 사용자만 할 수 있습니다. 자기 자신도, 다른 사람이나 문서도 올릴 수 있습니다.\n" +
				" * '''투표''' : 가입한 사용자만, 부문마다 한 표씩 할 수 있습니다.\n" +
				"\n" +
				"== 진행 순서 ==\n" +
				" 1. 운영자가 수상식을 열고 부문을 정합니다.\n" +
				" 1. 출마 기간 동안 후보를 올립니다.\n" +
				" 1. 운영자가 출마를 마감하면 투표가 시작됩니다.\n" +
				" 1. 투표 기간에는 표가 몇 개인지 보이지 않습니다. (표 쏠림을 막기 위해서입니다)\n" +
				" 1. 운영자가 마감하면 부문마다 가장 많은 표를 받은 후보가 수상자로 확정되고, 결과가 문서에 남습니다.\n" +
				"\n" +
				"수상식 문서는 [[분류:위키 수상식]] 아래에 투표 이름공간으로 쌓입니다.\n" +
				"\n" +
				"{{{#!html <a href=\"/baro/ceremony\">위키 수상식 바로가기</a>}}}\n" +
				"\n" +
				"[[분류:바로위키]]",
		},
		{
			"category:위키 수상식",
			"역대 위키 수상식 문서를 모아 둔 분류입니다.\n\n[[분류:바로위키]]",
		},
	} {
		title := row[0]
		data := row[1]

		exist := ""
		if QueryRow_DB(db, "select title from data where title = ?", []any{&exist}, title) {
			continue
		}

		Exec_DB(db, "insert into data (title, data, type) values (?, ?, '')", title, data)
		Do_add_history(
			db, title, data, Get_time(), "system", "바로위키 기본 문서",
			"+"+Baro_itoa(Get_len(data)), "", "",
		)
	}

	// --- 대문 안내 ---
	frontpage := Baro_frontpage_title(db)
	if frontpage != "" {
		old_data := ""
		QueryRow_DB(db, "select data from data where title = ?", []any{&old_data}, frontpage)

		if !strings.Contains(old_data, "위키 수상식") {
			new_data := old_data + baro_ceremony_frontpage_block()

			if old_data == "" {
				Exec_DB(db, "insert into data (title, data, type) values (?, ?, '')", frontpage, new_data)
			} else {
				Exec_DB(db, "delete from data where title = ?", frontpage)
				Exec_DB(db, "insert into data (title, data, type) values (?, ?, '')", frontpage, new_data)
			}

			Do_add_history(
				db, frontpage, new_data, Get_time(), "system", "대문에 위키 수상식 안내 추가",
				Get_edit_length_diff(old_data, new_data), "", "",
			)
		}
	}

	Baro_put(db, "seed_ceremony", "on")
}
