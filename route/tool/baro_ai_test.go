package tool

import (
	"strings"
	"testing"
)

// ============================================================
//  AI 참고 표시 시험
// ============================================================

// TestBaroAiSend : 편집 요약 뒤에 표시 붙이기
func TestBaroAiSend(t *testing.T) {
	// 체크하지 않으면 손대지 않는다
	if got := Baro_ai_send("오타 고침", false); got != "오타 고침" {
		t.Fatalf("체크하지 않았는데 붙었다: %q", got)
	}
	if got := Baro_ai_send("", false); got != "" {
		t.Fatalf("빈 요약이 바뀌었다: %q", got)
	}

	// 체크하면 뒤에 붙는다
	if got := Baro_ai_send("오타 고침", true); got != "오타 고침 (AI 참고)" {
		t.Fatalf("표시가 다르다: %q", got)
	}

	// 요약이 비어 있으면 표시만 남는다
	if got := Baro_ai_send("", true); got != "(AI 참고)" {
		t.Fatalf("빈 요약에 붙인 모양이 다르다: %q", got)
	}
	if got := Baro_ai_send("   ", true); got != "(AI 참고)" {
		t.Fatalf("공백만 있는 요약을 빈 것으로 보지 않았다: %q", got)
	}

	// 두 번 붙이지 않는다
	once := Baro_ai_send("고침", true)
	twice := Baro_ai_send(once, true)

	if once != twice {
		t.Fatalf("두 번 붙었다: %q → %q", once, twice)
	}
	if strings.Count(twice, Baro_ai_mark) != 1 {
		t.Fatalf("표시가 여러 번 들어갔다: %q", twice)
	}

	// 손으로 적어 넣은 것도 그대로 둔다
	if got := Baro_ai_send("(AI 참고) 로 고침", true); strings.Count(got, Baro_ai_mark) != 1 {
		t.Fatalf("이미 적혀 있는데 또 붙었다: %q", got)
	}
}

// TestBaroAiOn : 체크 여부 읽기
func TestBaroAiOn(t *testing.T) {
	for _, on := range []string{"yes", "on", "1", "아무거나"} {
		if !Baro_ai_on(on) {
			t.Fatalf("체크한 값인데 아니라고 한다: %q", on)
		}
	}

	for _, off := range []string{"", "   ", "\t"} {
		if Baro_ai_on(off) {
			t.Fatalf("체크하지 않은 값인데 했다고 한다: %q", off)
		}
	}
}

// TestBaroAiHas : 표시가 있는지 보기
func TestBaroAiHas(t *testing.T) {
	if !Baro_ai_has("고침 (AI 참고)") {
		t.Fatal("붙어 있는데 없다고 한다")
	}
	if Baro_ai_has("고침") {
		t.Fatal("없는데 있다고 한다")
	}

	// 비슷하지만 다른 말은 잡지 않는다
	if Baro_ai_has("AI 참고") {
		t.Fatal("괄호 없는 말을 표시로 봤다")
	}
}

// TestBaroAiUi : 체크 칸이 성한 모양으로 나온다
func TestBaroAiUi(t *testing.T) {
	html := Baro_ai_check_ui()

	for _, want := range []string{
		`name="` + Baro_ai_field + `"`,
		`type="checkbox"`,
		"AI를 참고하였습니다.",
		"3일 이상",
		"7일 이상",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("%q 가 빠졌다: %s", want, html)
		}
	}

	// 처음에는 꺼져 있어야 한다. 스스로 밝히는 것이지 기본값이 아니다.
	if strings.Contains(html, "checked") {
		t.Fatalf("체크 칸이 미리 켜져 있다: %s", html)
	}
}
