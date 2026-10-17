package route

import (
	"strings"
	"testing"

	"opennamu/route/tool"
)

// ============================================================
//  분류 문서 아래에 무엇이 보이는가
//
//  back 표의 cat 줄은 「link 문서가 title 분류에 속한다」 는 뜻이다.
//    title = 분류(대상) · link = 그 분류를 단 문서
//
//  그래서 이 분류에 든 것은 title 쪽을 이 분서로 놓고 link 를 읽어야 한다.
//  예전에는 든 것이 하나도 없으면 방향을 뒤집어 다시 찾는 갈래가 있었는데,
//  그러면 이 분류가 속한 **상위** 분류가 「하위 분류」 자리에 찍혔다.
// ============================================================

// TestCategoryShowsMembersNotParents : 상위 분류가 하위 분류로 찍히지 않는가
func TestCategoryShowsMembersNotParents(t *testing.T) {
	db := baro_flow_boot(t)

	config := tool.Config{IP: "글쓴이"}

	// 분류:템플릿 은 특수 문서 · 견본 에 속한다 (= 상위 분류)
	Api_edit_post(config, "category:템플릿", "템플릿 분류입니다.\n[[분류:특수 문서]]\n[[분류:견본]]", "", "yes", "")
	// 분류:템플릿 을 상위로 둔 하위 분류와, 이 분류에 든 문서
	Api_edit_post(config, "category:하위템플릿", "하위입니다.\n[[분류:템플릿]]", "", "yes", "")
	Api_edit_post(config, "예시문서", "본문\n[[분류:템플릿]]", "", "yes", "")
	// 아무도 안 든 분류 (상위 분류만 있다)
	Api_edit_post(config, "category:빈분류", "빈 분류입니다.\n[[분류:특수 문서]]", "", "yes", "")

	got := view_w_category_data(db, config, "category:템플릿")

	// 든 것이 나와야 한다
	if !strings.Contains(got, "하위템플릿") {
		t.Fatalf("하위 분류가 없다\n%s", got)
	}
	if !strings.Contains(got, "예시문서") {
		t.Fatalf("이 분류에 든 문서가 없다\n%s", got)
	}

	// 상위 분류는 나오면 안 된다
	for _, parent := range []string{"견본", "특수 문서"} {
		if strings.Contains(got, parent) {
			t.Fatalf("상위 분류 %q 가 나왔다\n%s", parent, got)
		}
	}

	// 든 것이 하나도 없는 분류는 아무것도 안 보인다
	// (여기서 상위 분류가 새어 나오던 것이 원래 문제였다)
	empty := view_w_category_data(db, config, "category:빈분류")
	if strings.TrimSpace(empty) != "" {
		t.Fatalf("빈 분류인데 무언가 나왔다\n%s", empty)
	}
}

// TestCategoryMemberNameIsKorean : 든 것의 이름과 주소가 한국어 이름공간인가
func TestCategoryMemberNameIsKorean(t *testing.T) {
	db := baro_flow_boot(t)

	config := tool.Config{IP: "글쓴이"}
	Api_edit_post(config, "category:상위", "상위 분류입니다.", "", "yes", "")
	Api_edit_post(config, "category:아래", "아래입니다.\n[[분류:상위]]", "", "yes", "")

	got := view_w_category_data(db, config, "category:상위")

	if !strings.Contains(got, "분류:아래") {
		t.Fatalf("한국어 이름이 아니다\n%s", got)
	}
	if strings.Contains(got, "category:아래") {
		t.Fatalf("내부 이름이 그대로 나왔다\n%s", got)
	}
	if strings.Contains(got, "/w/category:") {
		t.Fatalf("주소에 내부 이름이 그대로 나왔다\n%s", got)
	}
}
