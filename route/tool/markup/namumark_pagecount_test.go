package markup

import (
	"strings"
	"testing"

	"opennamu/route/tool"
)

// TestMarkupPagecount : [pagecount] · [pagecount(이름공간)]
func TestMarkupPagecount(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	for _, one := range []string{
		"일반 문서 하나", "일반 문서 둘",
		"trash:버린 문서", "trash:버린 문서 둘", "trash:버린 문서 셋",
		"file:그림.png",
		"category:분류 하나",
	} {
		tool.Exec_DB(db, "insert into data (title, data) values (?, '본문')", one)
	}

	cases := map[string]string{
		// 이름공간별
		"[pagecount(휴지통)]": "3",
		"[pagecount(파일)]":  "1",
		"[pagecount(분류)]":  "1",
		"[pagecount(문서)]":  "2", // 이름공간이 없는 문서
		"[pagecount(사용자)]": "0", // 하나도 없는 이름공간

		// 전체
		"[pagecount]":            "7",
		"[pagecount()]":          "7",
		"[pagecount(아무 말이나)]":    "7", // 이름공간이 아니면 전체
		"[pagecount(존재하지 않는 것)]": "7",

		// 영어 · 일본어 이름도 받는다
		"[pagecount(trash)]":   "3",
		"[pagecount(file)]":    "1",
		"[pagecount(ごみびこ)]":    "3",

		// 앞뒤 빈칸은 무시한다
		"[pagecount( 휴지통 )]": "3",
	}

	for source, want := range cases {
		got := strings.TrimSpace(syntax_render(t, db, source))
		if got != want {
			t.Errorf("%s : %q 를 바랐는데 %q", source, want, got)
		}
	}
}

// TestMarkupPagecountInText : 글 속에 섞여 있어도 되는가
func TestMarkupPagecountInText(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	tool.Exec_DB(db, "insert into data (title, data) values ('문서 하나', '본문')")
	tool.Exec_DB(db, "insert into data (title, data) values ('trash:버린 것', '본문')")

	got := syntax_render(t, db, "이 위키에는 [pagecount]개의 문서가 있고, 휴지통에는 [pagecount(휴지통)]개가 있습니다.")

	for _, want := range []string{"2개의 문서", "1개가 있습니다"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q 가 없다\n결과 : %s", want, got)
		}
	}
	if strings.Contains(got, "pagecount") {
		t.Fatalf("매크로가 그대로 남았다\n결과 : %s", got)
	}
}

// TestMarkupEmptyMacroArgs : 인자가 빈 괄호를 매크로로 보되, 보통 글은 건드리지 않는가
//
//	`[pagecount()]` 를 받으려고 겹 매크로 정규식이 빈 인자를 받게 바뀌었다.
//	모르는 이름은 그대로 두어야 한다.
func TestMarkupEmptyMacroArgs(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	for _, source := range []string{"[없는매크로()]", "[foo()]", "[123()]"} {
		got := syntax_render(t, db, source)
		if !strings.Contains(got, strings.Trim(source, "[]")) {
			t.Errorf("%q : 모르는 매크로인데 글이 사라졌다\n결과 : %s", source, got)
		}
	}
}
