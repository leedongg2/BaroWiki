package route

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestBaroCompareRun : 같은 대상을 다룬 두 글을 견줄 때
//
//	같은 문장 · 한쪽에만 있는 문장 · 숫자가 어긋나는 자리를 제대로 갈라내는지 본다.
func TestBaroCompareRun(t *testing.T) {
	mine := "선우섭은 1885년에 태어났다. 평안북도 정주군 출신이다. " +
		"1920년 11월 11일 통화현에서 피살되었다. 정부는 1998년 건국훈장 애국장을 추서했다."
	other := "선우섭은 1885년에 태어났다. 평안북도 정주군 출신이다. " +
		"1920년 12월 3일 통화현에서 피살되었다. 그는 청년단 검찰로 활동했다."

	result := baro_compare_run(mine, other)

	if len(result.Same) < 2 {
		t.Fatalf("같은 문장을 찾지 못했다 : %v", result.Same)
	}
	if len(result.OnlyMine) == 0 {
		t.Fatal("우리에만 있는 문장을 찾지 못했다")
	}
	if len(result.OnlyOther) == 0 {
		t.Fatal("상대에만 있는 문장을 찾지 못했다")
	}
	if len(result.DateGap) == 0 && len(result.NumberGap) == 0 {
		t.Fatal("날짜가 어긋나는 자리를 찾지 못했다")
	}
	if result.Similarity < 30 {
		t.Fatalf("닮은 정도가 너무 낮다 : %d", result.Similarity)
	}
}

// TestBaroCompareClean : 위키 문법을 걷어 내고 글만 남기는지
func TestBaroCompareClean(t *testing.T) {
	source := "{{{#!wiki style=\"color:red\"\n안내\n}}}\n" +
		"'''굵은 글씨''' 와 [[링크|보이는 이름]] 과 [각주] 가 섞인 문장이다.\n" +
		"||표||칸||"

	clean := baro_compare_clean(source)

	for _, one := range []string{"{{{", "}}}", "'''", "[[", "]]"} {
		if contains(clean, one) {
			t.Fatalf("문법이 남아 있다 (%s) : %s", one, clean)
		}
	}
	if !contains(clean, "굵은 글씨") {
		t.Fatalf("본문이 사라졌다 : %s", clean)
	}
}

// TestBaroCompareBigram : 닮은 정도가 뒤집히지 않는지
func TestBaroCompareBigram(t *testing.T) {
	same := baro_compare_bigram("바로위키는 한국어 위키입니다", "바로위키는 한국어 위키입니다")
	near := baro_compare_bigram("바로위키는 한국어 위키입니다", "바로위키는 한국어 백과입니다")
	far := baro_compare_bigram("바로위키는 한국어 위키입니다", "오늘 날씨가 아주 좋습니다")

	if same != 100 {
		t.Fatalf("같은 글인데 %d", same)
	}
	if !(same > near && near > far) {
		t.Fatalf("차례가 뒤집혔다 : 같음 %d 비슷 %d 다름 %d", same, near, far)
	}
}

// TestBaroCompareSameSet : 숫자 묶음 견주기
func TestBaroCompareSameSet(t *testing.T) {
	if !baro_compare_same_set([]string{"1", "2"}, []string{"2", "1"}) {
		t.Fatal("차례만 다른 같은 묶음을 다르다고 봤다")
	}
	if baro_compare_same_set([]string{"1", "2"}, []string{"1", "3"}) {
		t.Fatal("다른 묶음을 같다고 봤다")
	}
	if !baro_compare_same_set(nil, nil) {
		t.Fatal("빈 묶음끼리는 같아야 한다")
	}
}

func contains(data string, part string) bool {
	return len(part) > 0 && len(data) >= len(part) && indexOf(data, part) >= 0
}

func indexOf(data string, part string) int {
	for index := 0; index+len(part) <= len(data); index++ {
		if data[index:index+len(part)] == part {
			return index
		}
	}

	return -1
}

// TestBaroCompareExtractRemote : 받아 온 글에서 본문만 꺼내는지
func TestBaroCompareExtractRemote(t *testing.T) {
	// openNAMU 원문 화면 (편집창 안의 글만 꺼낸다)
	page := `<!doctype html><html><head><title>x</title></head><body>` +
		`<textarea id="opennamu_edit_textarea" class="a">본문 &amp; 내용</textarea>` +
		`<script>var a = 1;</script></body></html>`
	if got := baro_compare_extract_remote(page); got != "본문 & 내용" {
		t.Fatalf("편집창 본문을 꺼내지 못했다 : %q", got)
	}

	// 그냥 HTML (태그를 벗겨 낸다)
	plain_html := `<!doctype html><html><head><style>.a{color:red}</style></head>` +
		`<body><script>evil()</script><p>보이는 글</p></body></html>`
	got := baro_compare_extract_remote(plain_html)
	if contains(got, "<p>") || contains(got, "evil()") || contains(got, "color:red") {
		t.Fatalf("태그나 스크립트가 남았다 : %q", got)
	}
	if !contains(got, "보이는 글") {
		t.Fatalf("본문이 사라졌다 : %q", got)
	}

	// HTML 이 아니면 그대로 둔다
	if got := baro_compare_extract_remote("그냥 [[문법]] 이 든 글"); got != "그냥 [[문법]] 이 든 글" {
		t.Fatalf("그냥 글을 건드렸다 : %q", got)
	}
}

// baro_compare_brute : 예전 방식 (모든 문장을 서로 맞대어 본다)
//
//	빠르게 고친 방식이 같은 답을 내는지 견주기 위한 기준이다.
func baro_compare_brute(mine string, other string) ([]string, []string, []string) {
	mine_list := baro_compare_sentences(baro_compare_clean(mine))
	other_list := baro_compare_sentences(baro_compare_clean(other))

	used := make([]bool, len(other_list))
	same := []string{}
	only_mine := []string{}

	for _, one := range mine_list {
		best := -1
		best_score := 0
		for index, target := range other_list {
			if used[index] {
				continue
			}
			if score := baro_compare_bigram(one, target); score > best_score {
				best_score = score
				best = index
			}
		}

		if best >= 0 && best_score >= 70 {
			used[best] = true
			same = append(same, one)

			continue
		}

		only_mine = append(only_mine, one)
	}

	only_other := []string{}
	for index, one := range other_list {
		if !used[index] {
			only_other = append(only_other, one)
		}
	}

	return same, only_mine, only_other
}

// TestBaroCompareMatchesBruteForce : 빠르게 고친 비교가 예전 방식과 같은 답을 내는가
//
//	값을 아끼려고 색인을 쓰는데, 그 때문에 찾아야 할 같은 문장을 놓치면 안 된다.
func TestBaroCompareMatchesBruteForce(t *testing.T) {
	mine := strings.Builder{}
	other := strings.Builder{}

	for index := 0; index < 120; index++ {
		line := "이것은 " + strings.Repeat("가나", index%5+2) + " 에 대한 " + itoa(index) + "번째 문장입니다. "
		mine.WriteString(line)

		switch index % 3 {
		case 0:
			other.WriteString(line) // 똑같은 문장
		case 1:
			other.WriteString("이것은 " + strings.Repeat("가나", index%5+2) + " 에 대한 " + itoa(index) + "번째 문장입니다 아마도. ")
		default:
			other.WriteString("전혀 다른 이야기를 여기에 적어 둡니다 " + itoa(index) + " 번. ")
		}
	}

	want_same, want_mine, want_other := baro_compare_brute(mine.String(), other.String())
	got := baro_compare_run(mine.String(), other.String())

	if len(got.Same) != len(want_same) {
		t.Fatalf("같은 문장 수가 다르다 : 예전 %d, 지금 %d", len(want_same), len(got.Same))
	}
	if len(got.OnlyMine) != len(want_mine) {
		t.Fatalf("우리만 있는 문장 수가 다르다 : 예전 %d, 지금 %d", len(want_mine), len(got.OnlyMine))
	}
	if len(got.OnlyOther) != len(want_other) {
		t.Fatalf("상대만 있는 문장 수가 다르다 : 예전 %d, 지금 %d", len(want_other), len(got.OnlyOther))
	}
	for index := range want_same {
		if got.Same[index] != want_same[index] {
			t.Fatalf("%d번째 같은 문장이 다르다", index)
		}
	}
}

// TestBaroCompareBigDocumentIsFast : 긴 문서 두 개를 견주는 데 오래 걸리지 않는가
//
//	예전 방식은 문장 3,000개씩에 2분이 걸려 서버를 통째로 막았다.
func TestBaroCompareBigDocumentIsFast(t *testing.T) {
	mine := strings.Builder{}
	other := strings.Builder{}
	for index := 0; index < 3000; index++ {
		mine.WriteString("이것은 " + strings.Repeat("가", index%7+3) + " 에 대한 " + itoa(index%100) + " 문장입니다. ")
		other.WriteString("이것은 " + strings.Repeat("나", index%5+3) + " 에 대한 " + itoa(index%100) + " 문장입니다. ")
	}

	start := time.Now()
	result := baro_compare_run(mine.String(), other.String())
	spent := time.Since(start)

	if spent > 10*time.Second {
		t.Fatalf("긴 문서 비교가 너무 느리다 : %v", spent)
	}
	if result.MineCut <= 0 || result.OtherCut <= 0 {
		t.Fatalf("상한을 넘겼는데 몇 줄을 못 봤는지 알려 주지 않았다 (%d / %d)", result.MineCut, result.OtherCut)
	}

	t.Logf("문장 3,000개씩 : %v (못 본 줄 %d / %d)", spent, result.MineCut, result.OtherCut)
}

func itoa(num int) string {
	return strconv.Itoa(num)
}
