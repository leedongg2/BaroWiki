package tool

import "database/sql"

// ============================================================
//  바로샵 기본 상품
//
//  처음 한 번만 심는다. 관리자가 지우거나 값을 고쳐도 되살아나지 않는다.
//  (심었다는 사실을 baro_set 에 남겨 둔다)
// ============================================================

// baro_shop_default : {열쇠, 이름, 설명, 종류, 값, 부가값(JSON), 정렬}
func baro_shop_default() [][]string {
	return [][]string{
		{
			"칭호 · 문서 사냥꾼",
			"이름 옆에 「문서 사냥꾼」 칭호를 답니다.",
			"title", "100",
			Baro_json_enc(map[string]string{"title": "문서 사냥꾼"}),
			"10",
		},
		{
			"칭호 · 출처의 달인",
			"이름 옆에 「출처의 달인」 칭호를 답니다.",
			"title", "100",
			Baro_json_enc(map[string]string{"title": "출처의 달인"}),
			"20",
		},
		{
			"칭호 · 토론의 중재자",
			"이름 옆에 「토론의 중재자」 칭호를 답니다.",
			"title", "100",
			Baro_json_enc(map[string]string{"title": "토론의 중재자"}),
			"30",
		},
		{
			"칭호 · 새벽의 편집자",
			"이름 옆에 「새벽의 편집자」 칭호를 답니다.",
			"title", "100",
			Baro_json_enc(map[string]string{"title": "새벽의 편집자"}),
			"40",
		},
		{
			"캡차 1회 면제권",
			"편집이나 파일 올리기에서 캡차를 한 번 건너뜁니다. 여러 장 모아 둘 수 있습니다.",
			"captcha_once", "20",
			Baro_json_enc(map[string]string{"count": "1"}),
			"90",
		},
		{
			"캡차 면제 15일권",
			"15일 동안 편집·올리기에서 캡차를 묻지 않습니다. 이미 면제 중이면 남은 기간 뒤에 이어 붙습니다.",
			"captcha", "150",
			Baro_json_enc(map[string]string{"days": "15"}),
			"100",
		},
	}
}

// Baro_seed_shop : 바로샵 기본 상품 심기 (여러 번 불러도 안전)
func Baro_seed_shop(db *sql.DB) {
	done := ""
	if QueryRow_DB(db, "select name from baro_set where name = 'shop_seeded'", []any{&done}) {
		return
	}

	for _, row := range baro_shop_default() {
		Exec_DB(
			db,
			"insert into baro_shop_item (id, name, description, kind, price, data, status, stock, sort, actor, date) "+
				"values (?, ?, ?, ?, ?, ?, 'open', '', ?, 'system', ?)",
			Baro_next_id(db, "baro_shop_item"), row[0], row[1], row[2], row[3], row[4], row[5], Baro_now(),
		)
	}

	Baro_put(db, "shop_seeded", "on")
}
