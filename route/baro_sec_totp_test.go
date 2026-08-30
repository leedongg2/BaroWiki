package route

import (
	"testing"
	"time"
)

// TestBaroSecTotpRFC6238 : RFC 6238 표준 시험값으로 2단계 인증 코드 계산을 검증한다.
//
//	비밀키 "12345678901234567890" (base32 : GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ)
//	RFC 의 8자리 값에서 뒤 6자리가 우리가 쓰는 6자리 코드다.
func TestBaroSecTotpRFC6238(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},         // RFC : 94287082
		{1111111109, "081804"}, // RFC : 07081804
		{1111111111, "050471"}, // RFC : 14050471
		{1234567890, "005924"}, // RFC : 89005924
		{2000000000, "279037"}, // RFC : 69279037
	}

	for _, one := range cases {
		got := baro_sec_totp_at(secret, one.unix/30)
		if got != one.want {
			t.Errorf("T=%d : %s 를 기대했는데 %s", one.unix, one.want, got)
		}
	}
}

// TestBaroSecTotpBadSecret : 잘못된 비밀키는 빈 코드를 돌려줘야 한다.
func TestBaroSecTotpBadSecret(t *testing.T) {
	if got := baro_sec_totp_at("이건 base32 가 아님!!!", 1); got != "" {
		t.Errorf("잘못된 비밀키인데 코드가 나옴: %q", got)
	}
}

// TestBaroSecTotpVerify : 코드 검증이 지금 시각의 코드를 받아들이고 엉뚱한 코드는 거부하는지.
func TestBaroSecTotpVerify(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

	if !baro_sec_totp_ok(secret, baro_sec_totp_at(secret, time.Now().Unix()/baro_sec_totp_period)) {
		t.Error("지금 시각의 코드를 거부함")
	}

	if baro_sec_totp_ok(secret, "000000") && baro_sec_totp_at(secret, time.Now().Unix()/baro_sec_totp_period) != "000000" {
		t.Error("엉뚱한 코드를 받아들임")
	}

	if baro_sec_totp_ok(secret, "12345") {
		t.Error("6자리가 아닌 코드를 받아들임")
	}

	// 공백과 하이픈은 지워서 받아야 한다
	code := baro_sec_totp_at(secret, time.Now().Unix()/baro_sec_totp_period)
	if !baro_sec_totp_ok(secret, code[:3]+" "+code[3:]) {
		t.Error("공백이 들어간 코드를 거부함")
	}
}
