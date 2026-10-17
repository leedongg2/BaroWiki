package tool

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"
)

// ============================================================
//  바로 캡차 시험
// ============================================================

// baro_captcha_boot : 캡차에 필요한 표만 갖춘 작은 DB
func baro_captcha_boot(t *testing.T) *sql.DB {
	t.Helper()

	db := baro_uuid_test_db(t)

	for _, query := range []string{
		"create table baro_set (name text default '', data text default '')",
		"create table baro_captcha (id text default '', code text default '', answer text default '', " +
			"who text default '', status text default '', date text default '')",
		"create table other (name text default '', data text default '', coverage text default '')",
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}

	return db
}

// TestBaroCaptchaVerify : 맞히기 · 한 번 쓰면 사라지기 · 대소문자
func TestBaroCaptchaVerify(t *testing.T) {
	db := baro_captcha_boot(t)

	code := Baro_captcha_new(db, "192.0.2.1")
	if code == "" {
		t.Fatal("문제를 만들지 못했다")
	}

	answer := Baro_captcha_answer(db, code)
	if len(answer) != Baro_captcha_len {
		t.Fatalf("답 길이가 다르다: %q", answer)
	}

	// 헷갈리는 글자는 쓰지 않는다
	for _, one := range answer {
		if !strings.ContainsRune(baro_captcha_alphabet, one) {
			t.Fatalf("쓰지 않기로 한 글자가 들어갔다: %q", answer)
		}
	}

	// 틀린 답
	if Baro_captcha_verify(db, code, "zzzzz") {
		t.Fatal("틀린 답이 통과했다")
	}

	// 틀려도 그 문제는 사라진다
	if Baro_captcha_answer(db, code) != "" {
		t.Fatal("틀린 뒤에도 문제가 남아 있다")
	}

	// 맞는 답
	code = Baro_captcha_new(db, "192.0.2.1")
	answer = Baro_captcha_answer(db, code)

	if !Baro_captcha_verify(db, code, strings.ToLower(answer)) {
		t.Fatal("소문자로 적은 맞는 답이 막혔다")
	}

	// 같은 문제를 두 번 쓸 수 없다
	if Baro_captcha_verify(db, code, answer) {
		t.Fatal("같은 문제를 두 번 썼다")
	}

	// 없는 이름표
	if Baro_captcha_verify(db, "그런거없음", "ABCDE") {
		t.Fatal("없는 문제로 통과했다")
	}
	if Baro_captcha_verify(db, "", "") {
		t.Fatal("빈 값으로 통과했다")
	}
}

// TestBaroCaptchaPack : 폼 값 묶고 풀기
func TestBaroCaptchaPack(t *testing.T) {
	packed := Baro_captcha_pack("abc123", "QWERT")

	code, typed, ok := Baro_captcha_unpack(packed)
	if !ok || code != "abc123" || typed != "QWERT" {
		t.Fatalf("묶은 값이 그대로 풀리지 않는다: %q → %q %q %v", packed, code, typed, ok)
	}

	// 바깥 캡차 토큰을 바로 캡차로 잘못 읽으면 안 된다
	if _, _, ok := Baro_captcha_unpack("03AGdBq26abcdef"); ok {
		t.Fatal("바깥 캡차 토큰을 바로 캡차로 읽었다")
	}
	if Baro_captcha_pack("", "QWERT") != "" {
		t.Fatal("이름표 없이 묶였다")
	}
}

// TestBaroCaptchaPng : 그림이 PNG 로 나온다
func TestBaroCaptchaPng(t *testing.T) {
	raw := Baro_captcha_png("ACDEF")
	if len(raw) < 100 {
		t.Fatalf("그림이 너무 작다: %d 바이트", len(raw))
	}

	if !bytes.HasPrefix(raw, []byte{0x89, 'P', 'N', 'G'}) {
		t.Fatal("PNG 가 아니다")
	}

	// 답 글자가 그림 속에 글자 그대로 들어 있으면 안 된다
	if bytes.Contains(raw, []byte("ACDEF")) {
		t.Fatal("답이 그림 파일에 글자 그대로 들어 있다")
	}
}

// TestBaroCaptchaMode : 누구에게 묻는지
func TestBaroCaptchaMode(t *testing.T) {
	db := baro_captcha_boot(t)

	// 기본값은 비로그인에게만
	if got := Baro_captcha_mode(db); got != "ip" {
		t.Fatalf("기본 범위가 ip 가 아니다: %s", got)
	}
	if !Baro_captcha_target(db, "192.0.2.1") {
		t.Fatal("비로그인에게 묻지 않는다")
	}
	if Baro_captcha_target(db, "글쓴이") {
		t.Fatal("ip 범위인데 로그인 사용자에게 물었다")
	}

	Baro_put(db, "captcha_mode", "all")
	if !Baro_captcha_target(db, "글쓴이") {
		t.Fatal("all 범위인데 로그인 사용자에게 묻지 않는다")
	}

	Baro_put(db, "captcha_mode", "off")
	if Baro_captcha_target(db, "192.0.2.1") {
		t.Fatal("off 인데 물었다")
	}

	// 아는 값이 아니면 기본값으로 돌아간다
	Baro_put(db, "captcha_mode", "아무거나")
	if got := Baro_captcha_mode(db); got != "ip" {
		t.Fatalf("모르는 값이 기본값으로 돌아가지 않았다: %s", got)
	}
}

// TestBaroCaptchaOnce : 1회 면제권
func TestBaroCaptchaOnce(t *testing.T) {
	db := baro_captcha_boot(t)

	if got := Baro_captcha_once_left(db, "글쓴이"); got != 0 {
		t.Fatalf("처음부터 면제권이 있다: %d", got)
	}
	if Baro_captcha_once_use(db, "글쓴이") {
		t.Fatal("없는 면제권을 썼다")
	}

	if got := Baro_captcha_once_add(db, "글쓴이", 3); got != 3 {
		t.Fatalf("면제권이 3 장이 아니다: %d", got)
	}

	for want := 2; want >= 0; want-- {
		if !Baro_captcha_once_use(db, "글쓴이") {
			t.Fatal("남아 있는 면제권을 쓰지 못했다")
		}
		if got := Baro_captcha_once_left(db, "글쓴이"); got != want {
			t.Fatalf("남은 장수가 %d 가 아니다: %d", want, got)
		}
	}

	if Baro_captcha_once_use(db, "글쓴이") {
		t.Fatal("다 쓴 뒤에도 면제권이 나왔다")
	}

	// 아이피에게는 주지 않는다
	if got := Baro_captcha_once_add(db, "192.0.2.1", 5); got != 0 {
		t.Fatalf("비로그인에게 면제권이 들어갔다: %d", got)
	}
}
