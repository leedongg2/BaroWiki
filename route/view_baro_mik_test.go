package route

import (
	"database/sql"
	"net/url"
	"strings"
	"testing"

	"opennamu/route/tool"
)

// ============================================================
//  밐 · 바로샵 시험
//
//  화면 함수는 스킨과 세션이 없는 시험에서 그리다 죽으므로,
//  부르는 자리마다 받아 넘기고 「DB 가 어떻게 되었는가」 로 확인한다.
// ============================================================

// baro_mik_test_user : 시험용 계정
func baro_mik_test_user(db *sql.DB, name string) {
	baro_flow_user(db, name, "user", tool.Get_time())
}

// TestBaroMikAddAndSpend : 넣고 빼기 · 잔액보다 많이 뺄 수 없음
func TestBaroMikAddAndSpend(t *testing.T) {
	db := baro_flow_boot(t)
	baro_mik_test_user(db, "돈많은이")

	if got := tool.Baro_mik_get(db, "돈많은이"); got != 0 {
		t.Fatalf("처음 잔액이 0 이 아니다: %d", got)
	}

	if !tool.Baro_mik_add(db, "돈많은이", 100, "admin", "시험 지급", "운영자", "") {
		t.Fatal("지급이 되지 않았다")
	}
	if got := tool.Baro_mik_get(db, "돈많은이"); got != 100 {
		t.Fatalf("잔액이 100 이 아니다: %d", got)
	}

	// 가진 것보다 많이 빼려 하면 아무 일도 일어나지 않아야 한다.
	if tool.Baro_mik_add(db, "돈많은이", -101, "buy", "너무 큰 값", "돈많은이", "") {
		t.Fatal("잔액보다 많이 빠져나갔다")
	}
	if got := tool.Baro_mik_get(db, "돈많은이"); got != 100 {
		t.Fatalf("실패한 뒤 잔액이 달라졌다: %d", got)
	}

	if !tool.Baro_mik_add(db, "돈많은이", -40, "buy", "시험 구매", "돈많은이", "") {
		t.Fatal("구매 차감이 되지 않았다")
	}
	if got := tool.Baro_mik_get(db, "돈많은이"); got != 60 {
		t.Fatalf("차감 뒤 잔액이 60 이 아니다: %d", got)
	}

	// 기록이 세 줄이 아니라 두 줄이어야 한다 (실패한 것은 남지 않는다)
	count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_mik_log where user = ?", []any{&count}, "돈많은이")
	if count != 2 {
		t.Fatalf("오간 기록 줄 수가 2 가 아니다: %d", count)
	}
}

// TestBaroMikAttendOncePerDay : 접속 보상은 하루 한 번만
func TestBaroMikAttendOncePerDay(t *testing.T) {
	db := baro_flow_boot(t)
	tool.Baro_init(db)
	baro_mik_test_user(db, "출석이")

	first := tool.Baro_mik_attend(db, "출석이")
	if first <= 0 {
		t.Fatalf("첫 접속 보상을 받지 못했다: %d", first)
	}

	if second := tool.Baro_mik_attend(db, "출석이"); second != 0 {
		t.Fatalf("같은 날에 두 번 받았다: %d", second)
	}

	if got := tool.Baro_mik_get(db, "출석이"); got != first {
		t.Fatalf("잔액이 한 번 몫과 다르다: %d (보상 %d)", got, first)
	}

	// 아이피(비로그인)에게는 주지 않는다.
	if tool.Baro_mik_attend(db, "192.0.2.1") != 0 {
		t.Fatal("비로그인에게 접속 보상이 나갔다")
	}
}

// TestBaroMikBuyTitle : 칭호를 사면 밐이 빠지고 칭호가 생긴다
func TestBaroMikBuyTitle(t *testing.T) {
	db := baro_flow_boot(t)
	tool.Baro_init(db)
	baro_mik_test_user(db, "사는이")
	tool.Baro_mik_add(db, "사는이", 300, "admin", "시험", "운영자", "")

	item := baro_mik_test_find(t, db, "title")

	config := tool.Config{IP: "사는이"}
	text, kind := baro_mik_shop_buy(db, config, item.ID)
	if kind != "ok" {
		t.Fatalf("사지 못했다: %s (%s)", text, kind)
	}

	if got := tool.Baro_mik_get(db, "사는이"); got != 300-item.Price {
		t.Fatalf("값이 제대로 빠지지 않았다: %d", got)
	}

	want := item.Data["title"]
	if !baro_mik_has_title(db, "사는이", want) {
		t.Fatalf("칭호를 얻지 못했다: %s", want)
	}

	// 처음 산 칭호는 바로 달려 있어야 한다.
	if now := tool.Baro_mik_title(db, "사는이"); now != want {
		t.Fatalf("칭호가 달리지 않았다: %q", now)
	}

	// 같은 칭호를 두 번 살 수 없다.
	text, kind = baro_mik_shop_buy(db, config, item.ID)
	if kind == "ok" {
		t.Fatalf("같은 칭호를 두 번 샀다: %s", text)
	}
}

// TestBaroMikBuyCaptcha : 캡차 면제권을 사면 캡차를 묻지 않는다
func TestBaroMikBuyCaptcha(t *testing.T) {
	db := baro_flow_boot(t)
	tool.Baro_init(db)
	baro_mik_test_user(db, "면제이")
	tool.Baro_mik_add(db, "면제이", 400, "admin", "시험", "운영자", "")

	item := baro_mik_test_find(t, db, "captcha")

	if tool.Baro_mik_captcha_free(db, "면제이") {
		t.Fatal("사기 전부터 면제 상태다")
	}

	config := tool.Config{IP: "면제이"}
	if text, kind := baro_mik_shop_buy(db, config, item.ID); kind != "ok" {
		t.Fatalf("사지 못했다: %s", text)
	}

	if !tool.Baro_mik_captcha_free(db, "면제이") {
		t.Fatal("사고 나서도 면제가 되지 않았다")
	}

	first := tool.Baro_mik_captcha_until(db, "면제이")

	// 겹쳐 사면 남은 기간 뒤로 이어 붙어야 한다.
	if text, kind := baro_mik_shop_buy(db, config, item.ID); kind != "ok" {
		t.Fatalf("두 번째로 사지 못했다: %s", text)
	}

	second := tool.Baro_mik_captcha_until(db, "면제이")
	if tool.Baro_days_between(first, second) < 14 {
		t.Fatalf("면제 기간이 이어 붙지 않았다: %s → %s", first, second)
	}
}

// TestBaroMikBuyNotEnough : 밐이 모자라면 사지지 않는다
func TestBaroMikBuyNotEnough(t *testing.T) {
	db := baro_flow_boot(t)
	tool.Baro_init(db)
	baro_mik_test_user(db, "가난이")
	tool.Baro_mik_add(db, "가난이", 10, "admin", "시험", "운영자", "")

	item := baro_mik_test_find(t, db, "title")

	if _, kind := baro_mik_shop_buy(db, tool.Config{IP: "가난이"}, item.ID); kind == "ok" {
		t.Fatal("밐이 모자란데도 샀다")
	}

	if got := tool.Baro_mik_get(db, "가난이"); got != 10 {
		t.Fatalf("실패했는데 잔액이 줄었다: %d", got)
	}

	count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_shop_buy where user = ?", []any{&count}, "가난이")
	if count != 0 {
		t.Fatalf("실패했는데 산 기록이 남았다: %d", count)
	}
}

// TestBaroMikSendByUuid : 고유 번호로 양도하기
func TestBaroMikSendByUuid(t *testing.T) {
	db := baro_flow_boot(t)
	baro_mik_test_user(db, "보내는이")
	baro_mik_test_user(db, "받는이")
	tool.Baro_mik_add(db, "보내는이", 100, "admin", "시험", "운영자", "")

	uuid := tool.Baro_user_uuid(db, "받는이")
	if uuid == "" {
		t.Fatal("받는 사람의 고유 번호가 만들어지지 않았다")
	}

	config := tool.Config{IP: "보내는이"}

	done, text, _ := baro_mik_send_run(db, config, uuid, "30", "고마워요")
	if !done {
		t.Fatalf("양도가 되지 않았다: %s", text)
	}

	if got := tool.Baro_mik_get(db, "보내는이"); got != 70 {
		t.Fatalf("보낸 쪽 잔액이 70 이 아니다: %d", got)
	}
	if got := tool.Baro_mik_get(db, "받는이"); got != 30 {
		t.Fatalf("받은 쪽 잔액이 30 이 아니다: %d", got)
	}
}

// TestBaroMikSendNoSuchUuid : 없는 번호로 보내면 딱 그 문구가 나오고 밐은 그대로다
func TestBaroMikSendNoSuchUuid(t *testing.T) {
	db := baro_flow_boot(t)
	baro_mik_test_user(db, "보내는이")
	tool.Baro_mik_add(db, "보내는이", 100, "admin", "시험", "운영자", "")

	config := tool.Config{IP: "보내는이"}

	for _, bad := range []string{
		"00000000-0000-4000-8000-000000000000", // 꼴은 맞지만 없는 번호
		"그런거없음",                                // 꼴부터 틀린 값
	} {
		done, text, _ := baro_mik_send_run(db, config, bad, "10", "")
		if done {
			t.Fatalf("없는 번호(%s)로 보내졌다", bad)
		}
		if !strings.Contains(text, "해당 UUID를 가진 사용자는 없습니다.") {
			t.Fatalf("안내 문구가 다르다(%s): %s", bad, text)
		}
	}

	if got := tool.Baro_mik_get(db, "보내는이"); got != 100 {
		t.Fatalf("실패했는데 잔액이 줄었다: %d", got)
	}
}

// TestBaroMikSendLimits : 자기 자신 · 0 이하 · 잔액 초과는 막힌다
func TestBaroMikSendLimits(t *testing.T) {
	db := baro_flow_boot(t)
	baro_mik_test_user(db, "혼자이")
	baro_mik_test_user(db, "남이")
	tool.Baro_mik_add(db, "혼자이", 50, "admin", "시험", "운영자", "")

	config := tool.Config{IP: "혼자이"}
	mine := tool.Baro_user_uuid(db, "혼자이")
	other := tool.Baro_user_uuid(db, "남이")

	if done, _, _ := baro_mik_send_run(db, config, mine, "10", ""); done {
		t.Fatal("자기 자신에게 보내졌다")
	}
	if done, _, _ := baro_mik_send_run(db, config, other, "0", ""); done {
		t.Fatal("0 을 보냈다")
	}
	if done, _, _ := baro_mik_send_run(db, config, other, "-5", ""); done {
		t.Fatal("음수를 보냈다")
	}
	if done, _, _ := baro_mik_send_run(db, config, other, "51", ""); done {
		t.Fatal("잔액보다 많이 보냈다")
	}

	if got := tool.Baro_mik_get(db, "혼자이"); got != 50 {
		t.Fatalf("막혔는데 잔액이 달라졌다: %d", got)
	}
	if got := tool.Baro_mik_get(db, "남이"); got != 0 {
		t.Fatalf("막혔는데 상대에게 들어갔다: %d", got)
	}
}

// TestBaroMikAdminGive : 관리자 지급은 사유가 있어야 하고 기록에 남는다
func TestBaroMikAdminGive(t *testing.T) {
	db := baro_flow_boot(t)
	baro_flow_user(db, "관리자", "owner", tool.Get_time())
	baro_mik_test_user(db, "받는이")

	config := tool.Config{IP: "관리자"}

	// 사유가 없으면 주지 않는다.
	if _, kind := baro_mik_admin_give(db, config, url.Values{
		"user": {"받는이"}, "amount": {"50"}, "reason": {"  "},
	}); kind == "ok" {
		t.Fatal("사유 없이 지급됐다")
	}
	if got := tool.Baro_mik_get(db, "받는이"); got != 0 {
		t.Fatalf("사유 없는 지급이 들어갔다: %d", got)
	}

	// 없는 사람에게는 주지 않는다.
	if _, kind := baro_mik_admin_give(db, config, url.Values{
		"user": {"그런사람없음"}, "amount": {"50"}, "reason": {"시험"},
	}); kind == "ok" {
		t.Fatal("없는 사용자에게 지급됐다")
	}

	// 제대로 주면 들어가고 공개 로그에 남는다.
	if text, kind := baro_mik_admin_give(db, config, url.Values{
		"user": {"받는이"}, "amount": {"50"}, "reason": {"위키 수상식 보상"},
	}); kind != "ok" {
		t.Fatalf("지급되지 않았다: %s", text)
	}
	if got := tool.Baro_mik_get(db, "받는이"); got != 50 {
		t.Fatalf("지급 뒤 잔액이 50 이 아니다: %d", got)
	}

	reason := ""
	if !tool.QueryRow_DB(db,
		"select reason from baro_public_log where kind = 'mik_admin' order by id + 0 desc limit 1",
		[]any{&reason},
	) {
		t.Fatal("공개 운영 로그에 남지 않았다")
	}
	if reason != "위키 수상식 보상" {
		t.Fatalf("공개 로그의 사유가 다르다: %q", reason)
	}

	// 회수도 된다. 가진 것보다 많이는 못 거둔다.
	if _, kind := baro_mik_admin_give(db, config, url.Values{
		"user": {"받는이"}, "amount": {"-80"}, "reason": {"잘못 지급"},
	}); kind == "ok" {
		t.Fatal("가진 것보다 많이 거둬졌다")
	}
	if _, kind := baro_mik_admin_give(db, config, url.Values{
		"user": {"받는이"}, "amount": {"-20"}, "reason": {"잘못 지급"},
	}); kind != "ok" {
		t.Fatal("회수가 되지 않았다")
	}
	if got := tool.Baro_mik_get(db, "받는이"); got != 30 {
		t.Fatalf("회수 뒤 잔액이 30 이 아니다: %d", got)
	}
}

// TestBaroMikAdminGate : 권한이 없으면 관리 화면에 들어갈 수 없다
func TestBaroMikAdminGate(t *testing.T) {
	db := baro_flow_boot(t)
	baro_mik_test_user(db, "그냥사용자")

	if !baro_mik_test_gate_stop(db, "그냥사용자") {
		t.Fatal("권한 없는 사용자가 관리 화면에 들어갔다")
	}

	baro_flow_user(db, "관리자", "owner", tool.Get_time())
	if baro_mik_test_gate_stop(db, "관리자") {
		t.Fatal("관리자가 막혔다")
	}
}

// baro_mik_test_gate_stop : 관리 화면 문이 막았는가
//
//	막을 때는 「권한 없음」 화면을 그리는데 시험에는 스킨도 세션도 없어 그리다 죽는다.
//	죽었다는 것 자체가 막혔다는 뜻이므로 그렇게 읽는다.
func baro_mik_test_gate_stop(db *sql.DB, ip string) (stop bool) {
	defer func() {
		if recover() != nil {
			stop = true
		}
	}()

	_, stop = baro_mik_admin_gate(db, tool.Config{IP: ip})

	return stop
}

// TestBaroMikShopDeleteKeepsSold : 팔린 상품은 지워지지 않는다
func TestBaroMikShopDeleteKeepsSold(t *testing.T) {
	db := baro_flow_boot(t)
	tool.Baro_init(db)
	baro_flow_user(db, "관리자", "owner", tool.Get_time())
	baro_mik_test_user(db, "사는이")
	tool.Baro_mik_add(db, "사는이", 300, "admin", "시험", "운영자", "")

	item := baro_mik_test_find(t, db, "title")
	config := tool.Config{IP: "관리자"}

	// 아직 안 팔렸으면 사유를 적어 지울 수 있다.
	spare := baro_mik_test_find_other(t, db, "title", item.ID)
	baro_mik_admin_delete(db, config, url.Values{"item": {spare.ID}, "reason": {"시험"}})

	if _, found := baro_mik_shop_get(db, spare.ID); found {
		t.Fatal("안 팔린 상품이 지워지지 않았다")
	}

	// 사유가 없으면 지우지 않는다.
	third := baro_mik_test_find_other(t, db, "title", item.ID)
	baro_mik_admin_delete(db, config, url.Values{"item": {third.ID}, "reason": {"  "}})
	if _, found := baro_mik_shop_get(db, third.ID); !found {
		t.Fatal("사유 없이 지워졌다")
	}

	// 한 번이라도 팔린 것은 지울 수 없다.
	if _, kind := baro_mik_shop_buy(db, tool.Config{IP: "사는이"}, item.ID); kind != "ok" {
		t.Fatal("사지 못했다")
	}

	baro_mik_admin_delete(db, config, url.Values{"item": {item.ID}, "reason": {"시험"}})
	if _, found := baro_mik_shop_get(db, item.ID); !found {
		t.Fatal("팔린 상품이 지워졌다")
	}
}

// TestBaroMikShopClosedNotForSale : 내린 상품은 살 수 없다
func TestBaroMikShopClosedNotForSale(t *testing.T) {
	db := baro_flow_boot(t)
	tool.Baro_init(db)
	baro_mik_test_user(db, "사는이")
	tool.Baro_mik_add(db, "사는이", 300, "admin", "시험", "운영자", "")

	item := baro_mik_test_find(t, db, "title")
	tool.Exec_DB(db, "update baro_shop_item set status = 'closed' where id = ?", item.ID)

	if _, kind := baro_mik_shop_buy(db, tool.Config{IP: "사는이"}, item.ID); kind == "ok" {
		t.Fatal("내린 상품이 팔렸다")
	}

	for _, row := range baro_mik_shop_list(db, true) {
		if row.ID == item.ID {
			t.Fatal("내린 상품이 진열에 남아 있다")
		}
	}
}

// TestBaroMikBadgeReward : 배지를 새로 받으면 밐이 들어온다
func TestBaroMikBadgeReward(t *testing.T) {
	db := baro_flow_boot(t)
	tool.Baro_init(db)
	baro_mik_test_user(db, "배지이")

	badge := baro_contrib_badge_list(db)
	if len(badge) == 0 {
		t.Skip("기본 배지가 없다")
	}

	if !baro_contrib_badge_give(db, "배지이", badge[0], "시험") {
		t.Fatal("배지를 주지 못했다")
	}

	want := tool.Baro_mik_badge_amount(db, badge[0].Key)
	if got := tool.Baro_mik_get(db, "배지이"); got != want {
		t.Fatalf("배지 보상이 들어오지 않았다: %d (기대 %d)", got, want)
	}

	// 같은 배지를 다시 줘도 두 번 들어오지 않는다.
	baro_contrib_badge_give(db, "배지이", badge[0], "시험")
	if got := tool.Baro_mik_get(db, "배지이"); got != want {
		t.Fatalf("같은 배지로 두 번 받았다: %d", got)
	}
}

// baro_mik_test_find : 씨앗 상품 가운데 이 종류인 것 하나
func baro_mik_test_find(t *testing.T, db *sql.DB, kind string) baro_mik_shop_row {
	t.Helper()

	for _, row := range baro_mik_shop_list(db, true) {
		if row.Kind == kind {
			return row
		}
	}

	t.Fatalf("씨앗 상품에 %s 종류가 없다", kind)

	return baro_mik_shop_row{}
}

// baro_mik_test_find_other : 같은 종류이면서 지정한 것이 아닌 상품 하나
func baro_mik_test_find_other(t *testing.T, db *sql.DB, kind string, not_id string) baro_mik_shop_row {
	t.Helper()

	for _, row := range baro_mik_shop_list(db, true) {
		if row.Kind == kind && row.ID != not_id {
			return row
		}
	}

	t.Fatalf("%s 종류의 다른 상품이 없다", kind)

	return baro_mik_shop_row{}
}
