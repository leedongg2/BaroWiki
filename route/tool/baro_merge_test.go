package tool

import (
	"strings"
	"testing"
)

func TestBaroMerge3NoOverlap(t *testing.T) {
	base := "머리말\n첫째 줄\n둘째 줄\n셋째 줄\n꼬리말"
	mine := "머리말\n첫째 줄 (내가 고침)\n둘째 줄\n셋째 줄\n꼬리말"
	theirs := "머리말\n첫째 줄\n둘째 줄\n셋째 줄 (남이 고침)\n꼬리말"

	merged, conflicts, auto := Baro_merge3(base, mine, theirs)
	if conflicts != 0 {
		t.Fatalf("겹치지 않는 수정인데 충돌 %d건: %q", conflicts, merged)
	}
	if auto == 0 {
		t.Fatalf("자동 병합 건수가 0: %q", merged)
	}

	if !strings.Contains(merged, "첫째 줄 (내가 고침)") {
		t.Errorf("내 수정이 사라짐: %q", merged)
	}
	if !strings.Contains(merged, "셋째 줄 (남이 고침)") {
		t.Errorf("상대 수정이 사라짐: %q", merged)
	}
	if Baro_has_conflict_mark(merged) {
		t.Errorf("불필요한 충돌 표시: %q", merged)
	}
}

func TestBaroMerge3Overlap(t *testing.T) {
	base := "머리말\n같은 줄\n꼬리말"
	mine := "머리말\n내가 바꾼 줄\n꼬리말"
	theirs := "머리말\n남이 바꾼 줄\n꼬리말"

	merged, conflicts, _ := Baro_merge3(base, mine, theirs)
	if conflicts != 1 {
		t.Fatalf("충돌 1건이어야 하는데 %d건: %q", conflicts, merged)
	}
	if !Baro_has_conflict_mark(merged) {
		t.Fatalf("충돌 표시가 없음: %q", merged)
	}

	// 양쪽 내용이 모두 보존되어야 한다
	if !strings.Contains(merged, "내가 바꾼 줄") || !strings.Contains(merged, "남이 바꾼 줄") {
		t.Errorf("충돌 블록에 양쪽 내용이 다 없음: %q", merged)
	}

	// 머리말/꼬리말은 충돌 밖에 한 번씩만
	if strings.Count(merged, "머리말") != 1 || strings.Count(merged, "꼬리말") != 1 {
		t.Errorf("공통 부분이 중복됨: %q", merged)
	}

	mine_only := Baro_pick_side(merged, "mine")
	if !strings.Contains(mine_only, "내가 바꾼 줄") || strings.Contains(mine_only, "남이 바꾼 줄") {
		t.Errorf("내 쪽 고르기 실패: %q", mine_only)
	}
	if Baro_has_conflict_mark(mine_only) {
		t.Errorf("고른 뒤에도 충돌 표시가 남음: %q", mine_only)
	}

	theirs_only := Baro_pick_side(merged, "theirs")
	if !strings.Contains(theirs_only, "남이 바꾼 줄") || strings.Contains(theirs_only, "내가 바꾼 줄") {
		t.Errorf("상대 쪽 고르기 실패: %q", theirs_only)
	}
}

func TestBaroMerge3SameEdit(t *testing.T) {
	base := "가\n나\n다"
	same := "가\n나 고침\n다"

	merged, conflicts, _ := Baro_merge3(base, same, same)
	if conflicts != 0 {
		t.Fatalf("같은 수정인데 충돌 %d건: %q", conflicts, merged)
	}
	if merged != same {
		t.Errorf("결과가 다름: %q", merged)
	}
}

func TestBaroMerge3AddBoth(t *testing.T) {
	base := "가\n나"
	mine := "가\n내가 넣은 줄\n나"
	theirs := "가\n나\n남이 넣은 줄"

	merged, conflicts, _ := Baro_merge3(base, mine, theirs)
	if conflicts != 0 {
		t.Fatalf("떨어진 위치의 추가인데 충돌 %d건: %q", conflicts, merged)
	}
	if !strings.Contains(merged, "내가 넣은 줄") || !strings.Contains(merged, "남이 넣은 줄") {
		t.Errorf("추가한 줄이 사라짐: %q", merged)
	}
}

func TestBaroDiffLine(t *testing.T) {
	ops := Baro_diff_line("가\n나\n다", "가\n라\n다")

	add, del, same := 0, 0, 0
	for _, op := range ops {
		switch op[0] {
		case "add":
			add++
		case "del":
			del++
		case "same":
			same++
		}
	}

	if add != 1 || del != 1 || same != 2 {
		t.Errorf("비교 결과가 이상함 add=%d del=%d same=%d", add, del, same)
	}
}

func TestBaroClassifyEdit(t *testing.T) {
	cases := []struct {
		name, old, now, send, rev, want string
	}{
		{"새 문서", "", "새로 쓴 내용입니다.", "", "1", "new_document"},
		{"오타", "안녕하세요 여러분", "안녕하세요 여러뷴", "", "5", "typo"},
		{"출처 추가", "본문", "본문[*출처 어딘가]", "", "5", "source"},
		{"보강", "짧은 본문", "짧은 본문" + strings.Repeat("가", 400), "", "5", "expand"},
	}

	for _, one := range cases {
		got := Baro_classify_edit(one.old, one.now, one.send, one.rev)
		if got != one.want {
			t.Errorf("%s : %s 를 기대했는데 %s", one.name, one.want, got)
		}
	}
}

func TestBaroChosung(t *testing.T) {
	if got := Baro_chosung("김성범"); got != "ㄱㅅㅂ" {
		t.Errorf("초성 추출 실패: %q", got)
	}
	if !Baro_is_chosung_only("ㄱㅅㅂ") {
		t.Error("초성 판별 실패")
	}
	if Baro_is_chosung_only("김성범") {
		t.Error("한글 완성형을 초성으로 잘못 판별")
	}
}

func TestBaroPct(t *testing.T) {
	if got := Baro_pct(4310, 18000); got != 23.9 {
		t.Errorf("개척률 계산 실패: %v", got)
	}
	if got := Baro_pct(0, 0); got != 0 {
		t.Errorf("분모 0 처리 실패: %v", got)
	}
	if got := Baro_comma(1234567); got != "1,234,567" {
		t.Errorf("천 단위 구분 실패: %q", got)
	}
}

func TestBaroMaskIP(t *testing.T) {
	if got := Baro_mask_ip("192.168.10.24"); got != "192.168.*.*" {
		t.Errorf("IP 가림 실패: %q", got)
	}
	if got := Baro_mask_ip("사용자이름"); got != "사용자이름" {
		t.Errorf("사용자 이름을 가리면 안 됨: %q", got)
	}
}

func TestBaroCountFootnote(t *testing.T) {
	if got := Baro_count_footnote("본문[*가 내용] 그리고[* 또]"); got != 2 {
		t.Errorf("각주 세기 실패: %d", got)
	}
	if got := Baro_count_category("[[분류:인물]]\n[[category:역사]]"); got != 2 {
		t.Errorf("분류 세기 실패: %d", got)
	}
	if got := len(Baro_extract_urls("보기 https://a.example/x 와 https://a.example/x 및 http://b.kr")); got != 2 {
		t.Errorf("링크 추출 실패: %d", got)
	}
}
