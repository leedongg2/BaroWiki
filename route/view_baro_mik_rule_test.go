package route

import (
	"database/sql"
	"net/url"
	"strings"
	"testing"

	"opennamu/route/tool"
)

// ============================================================
//  밐 한도 · 도전 과제별 보상 · 순위 · 캡차 시험
// ============================================================

// TestBaroMikSendDailyLimit : 하루 양도 한도
func TestBaroMikSendDailyLimit(t *testing.T) {
	db := baro_flow_boot(t)
	tool.Baro_init(db)
	baro_mik_test_user(db, "보내는이")
	baro_mik_test_user(db, "받는이")
	tool.Baro_mik_add(db, "보내는이", 1000, "admin", "시험", "운영자", "")

	if got := tool.Baro_mik_send_limit(db); got != 100 {
		t.Fatalf("기본 한도가 100 이 아니다: %d", got)
	}

	config := tool.Config{IP: "보내는이"}
	uuid := tool.Baro_user_uuid(db, "받는이")

	// 한 번에 한도를 넘기면 막힌다.
	done, text, _ := baro_mik_send_run(db, config, uuid, "101", "")
	if done {
		t.Fatal("한 번에 한도를 넘겨 보내졌다")
	}
	if !strings.Contains(text, "하루에 보낼 수 있는 밐은") {
		t.Fatalf("안내 문구가 다르다: %s", text)
	}

	// 나누어 보내도 하루 합이 한도를 넘지 못한다.
	if done, text, _ := baro_mik_send_run(db, config, uuid, "60", ""); !done {
		t.Fatalf("한도 안인데 막혔다: %s", text)
	}
	if got := tool.Baro_mik_send_left(db, "보내는이"); got != 40 {
		t.Fatalf("남은 한도가 40 이 아니다: %d", got)
	}
	if done, _, _ := baro_mik_send_run(db, config, uuid, "41", ""); done {
		t.Fatal("나누어 보내 한도를 넘겼다")
	}
	if done, text, _ := baro_mik_send_run(db, config, uuid, "40", ""); !done {
		t.Fatalf("남은 한도만큼도 못 보냈다: %s", text)
	}
	if got := tool.Baro_mik_send_left(db, "보내는이"); got != 0 {
		t.Fatalf("한도를 다 썼는데 남아 있다: %d", got)
	}
	if done, _, _ := baro_mik_send_run(db, config, uuid, "1", ""); done {
		t.Fatal("한도를 다 쓰고도 보내졌다")
	}

	// 받은 쪽에는 100 만 들어가 있어야 한다.
	if got := tool.Baro_mik_get(db, "받는이"); got != 100 {
		t.Fatalf("받은 쪽 잔액이 100 이 아니다: %d", got)
	}

	// 운영자가 한도를 올리면 그만큼 더 보낼 수 있다.
	tool.Baro_put(db, "mik_send_daily", "150")
	if got := tool.Baro_mik_send_left(db, "보내는이"); got != 50 {
		t.Fatalf("한도를 올렸는데 남은 양이 50 이 아니다: %d", got)
	}
	if done, text, _ := baro_mik_send_run(db, config, uuid, "50", ""); !done {
		t.Fatalf("올린 한도 안에서 못 보냈다: %s", text)
	}

	// 0 으로 두면 양도 자체가 막힌다.
	tool.Baro_put(db, "mik_send_daily", "0")
	if done, text, _ := baro_mik_send_run(db, config, uuid, "1", ""); done {
		t.Fatalf("한도를 0 으로 뒀는데 보내졌다: %s", text)
	}
}

// TestBaroMikBadgeAmountPerBadge : 도전 과제마다 보상액을 따로 정한다
func TestBaroMikBadgeAmountPerBadge(t *testing.T) {
	db := baro_flow_boot(t)
	tool.Baro_init(db)

	list := baro_contrib_badge_list(db)
	if len(list) < 2 {
		t.Skip("기본 배지가 둘 미만이다")
	}

	first, second := list[0], list[1]

	// 따로 정하지 않으면 기본값
	want := tool.Baro_mik_badge_default(db)
	if got := tool.Baro_mik_badge_amount(db, first.Key); got != want {
		t.Fatalf("기본값을 따르지 않는다: %d (기대 %d)", got, want)
	}
	if tool.Baro_mik_badge_own(db, first.Key) != -1 {
		t.Fatal("정하지 않았는데 따로 정한 값이 있다고 한다")
	}

	// 하나만 따로 정한다
	tool.Baro_mik_badge_set(db, first.Key, "77")
	if got := tool.Baro_mik_badge_amount(db, first.Key); got != 77 {
		t.Fatalf("따로 정한 값이 안 먹는다: %d", got)
	}
	if got := tool.Baro_mik_badge_amount(db, second.Key); got != want {
		t.Fatalf("다른 배지까지 바뀌었다: %d", got)
	}

	// 0 을 적으면 그 배지만 보상이 없다
	tool.Baro_mik_badge_set(db, second.Key, "0")
	if got := tool.Baro_mik_badge_amount(db, second.Key); got != 0 {
		t.Fatalf("0 이 먹지 않는다: %d", got)
	}

	baro_mik_test_user(db, "받는이")
	if reward := tool.Baro_mik_badge_reward(db, "받는이", second.Key, second.Name); reward != 0 {
		t.Fatalf("보상 0 인 배지로 밐이 나갔다: %d", reward)
	}
	if reward := tool.Baro_mik_badge_reward(db, "받는이", first.Key, first.Name); reward != 77 {
		t.Fatalf("따로 정한 보상이 나가지 않았다: %d", reward)
	}

	// 빈 값을 넣으면 기본값으로 돌아간다
	tool.Baro_mik_badge_set(db, first.Key, "")
	if got := tool.Baro_mik_badge_amount(db, first.Key); got != want {
		t.Fatalf("기본값으로 돌아가지 않았다: %d", got)
	}
}

// TestBaroMikSettingSave : 설정 화면에서 값을 고친다
func TestBaroMikSettingSave(t *testing.T) {
	db := baro_flow_boot(t)
	tool.Baro_init(db)
	baro_flow_user(db, "관리자", "owner", tool.Get_time())

	config := tool.Config{IP: "관리자"}

	baro_mik_setting_save(db, config, url.Values{
		"mik_attend":     {"9"},
		"mik_badge":      {"30"},
		"mik_send_daily": {"250"},
		"captcha_mode":   {"all"},
		"reason":         {"시험 조정"},
	})

	if got := tool.Baro_mik_attend_amount(db); got != 9 {
		t.Fatalf("접속 보상이 9 가 아니다: %d", got)
	}
	if got := tool.Baro_mik_badge_default(db); got != 30 {
		t.Fatalf("도전 과제 기본 보상이 30 이 아니다: %d", got)
	}
	if got := tool.Baro_mik_send_limit(db); got != 250 {
		t.Fatalf("양도 한도가 250 이 아니다: %d", got)
	}
	if got := tool.Baro_captcha_mode(db); got != "all" {
		t.Fatalf("캡차 범위가 all 이 아니다: %s", got)
	}

	// 음수는 받지 않는다
	baro_mik_setting_save(db, config, url.Values{"mik_attend": {"-5"}})
	if got := tool.Baro_mik_attend_amount(db); got != 9 {
		t.Fatalf("음수가 들어갔다: %d", got)
	}

	// 공개 운영 로그에 남는다
	detail := ""
	if !tool.QueryRow_DB(db,
		"select detail from baro_public_log where kind = 'mik_setting' order by id + 0 desc limit 1",
		[]any{&detail},
	) {
		t.Fatal("공개 운영 로그에 남지 않았다")
	}
	if detail == "" {
		t.Fatal("바뀐 내용이 비어 있다")
	}
}

// TestBaroMikRank : 지금·최대·누적 세 가지 순위
func TestBaroMikRank(t *testing.T) {
	db := baro_flow_boot(t)
	baro_mik_test_user(db, "많이쓴이")
	baro_mik_test_user(db, "모아둔이")

	// 많이쓴이 : 300 벌고 250 씀 → 지금 50, 최대 300, 누적 300
	tool.Baro_mik_add(db, "많이쓴이", 300, "admin", "시험", "운영자", "")
	tool.Baro_mik_add(db, "많이쓴이", -250, "buy", "시험", "많이쓴이", "")

	// 모아둔이 : 120 벌고 그대로 → 지금 120, 최대 120, 누적 120
	tool.Baro_mik_add(db, "모아둔이", 120, "admin", "시험", "운영자", "")

	if got := tool.Baro_mik_get(db, "많이쓴이"); got != 50 {
		t.Fatalf("지금 잔액이 50 이 아니다: %d", got)
	}
	if got := tool.Baro_mik_best(db, "많이쓴이"); got != 300 {
		t.Fatalf("최대 보유가 300 이 아니다: %d", got)
	}
	if got := tool.Baro_mik_total(db, "많이쓴이"); got != 300 {
		t.Fatalf("누적 획득이 300 이 아니다: %d", got)
	}

	// 지금 가진 밐은 모아둔이가 앞선다
	if head := baro_mik_rank_head(t, db, "now"); head != "모아둔이" {
		t.Fatalf("지금 순위 1등이 다르다: %s", head)
	}

	// 최대·누적은 많이쓴이가 앞선다
	if head := baro_mik_rank_head(t, db, "max"); head != "많이쓴이" {
		t.Fatalf("최대 순위 1등이 다르다: %s", head)
	}
	if head := baro_mik_rank_head(t, db, "total"); head != "많이쓴이" {
		t.Fatalf("누적 순위 1등이 다르다: %s", head)
	}

	// 양도로 받은 것도 누적에 들어간다
	tool.Baro_mik_add(db, "모아둔이", 500, "send_in", "시험", "많이쓴이", "")
	if got := tool.Baro_mik_total(db, "모아둔이"); got != 620 {
		t.Fatalf("받은 밐이 누적에 들어가지 않았다: %d", got)
	}
	if head := baro_mik_rank_head(t, db, "total"); head != "모아둔이" {
		t.Fatalf("누적 순위가 바뀌지 않았다: %s", head)
	}
}

// baro_mik_rank_head : 갈래별 1등
func baro_mik_rank_head(t *testing.T, db *sql.DB, kind string) string {
	t.Helper()

	list := tool.Baro_mik_rank(db, kind, 0, 10)
	if len(list) == 0 {
		t.Fatalf("%s 순위가 비었다", kind)
	}

	return list[0].User
}

// TestBaroMikBackfill : 누계가 생기기 전에 쌓인 밐도 순위에 들어온다
func TestBaroMikBackfill(t *testing.T) {
	db := baro_flow_boot(t)
	baro_mik_test_user(db, "옛날이")
	baro_mik_test_user(db, "기록없는이")

	tool.Baro_mik_add(db, "옛날이", 300, "admin", "시험", "운영자", "")
	tool.Baro_mik_add(db, "옛날이", -250, "buy", "시험", "옛날이", "")

	// 누계 칸이 없던 시절을 흉내 낸다.
	for _, name := range []string{"mik_max", "mik_total"} {
		tool.Exec_DB(db, "delete from baro_user_set where name = ?", name)
	}

	// 오간 기록조차 없이 잔액만 있는 사람
	tool.Exec_DB(db, "insert into baro_user_set (user, name, data) values ('기록없는이', 'mik', '80')")

	tool.Baro_mik_backfill(db)

	if got := tool.Baro_mik_best(db, "옛날이"); got != 300 {
		t.Fatalf("기록에서 최대 보유를 되살리지 못했다: %d", got)
	}
	if got := tool.Baro_mik_total(db, "옛날이"); got != 300 {
		t.Fatalf("기록에서 누적 획득을 되살리지 못했다: %d", got)
	}
	if got := tool.Baro_mik_best(db, "기록없는이"); got != 80 {
		t.Fatalf("잔액만 있는 사람을 놓쳤다: %d", got)
	}
	if got := tool.Baro_mik_total(db, "기록없는이"); got != 80 {
		t.Fatalf("잔액만 있는 사람의 누적을 놓쳤다: %d", got)
	}

	// 두 번 돌아도 값이 부풀지 않는다.
	tool.Baro_put(db, "mik_rank_filled", "")
	tool.Exec_DB(db, "delete from baro_set where name = 'mik_rank_filled'")
	tool.Baro_mik_backfill(db)

	if got := tool.Baro_mik_total(db, "옛날이"); got != 300 {
		t.Fatalf("두 번 돌아 값이 부풀었다: %d", got)
	}
}
