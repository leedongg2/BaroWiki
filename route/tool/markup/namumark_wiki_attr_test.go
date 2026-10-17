package markup

import (
	"net/url"
	"strings"
	"testing"
)

// TestMarkupWikiClass : {{{#!wiki class="…"}}}
func TestMarkupWikiClass(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	got := syntax_render(t, db, "{{{#!wiki class=\"form\"\n내용}}}")
	if !strings.Contains(got, `<div class="form"`) {
		t.Fatalf("class 가 붙지 않았다 : %s", got)
	}
	if !strings.Contains(got, "내용") {
		t.Fatalf("내용이 사라졌다 : %s", got)
	}

	// class 와 style 을 어느 차례로 적어도 둘 다 붙는다
	for _, source := range []string{
		"{{{#!wiki class=\"form\" style=\"color:#f00\"\n내용}}}",
		"{{{#!wiki style=\"color:#f00\" class=\"form\"\n내용}}}",
	} {
		got := syntax_render(t, db, source)
		if !strings.Contains(got, `class="form"`) || !strings.Contains(got, "color:#f00") {
			t.Fatalf("%q : 둘 다 붙지 않았다 : %s", source, got)
		}
	}

	// 이름 여러 개
	got = syntax_render(t, db, "{{{#!wiki class=\"form wide\"\n내용}}}")
	if !strings.Contains(got, `class="form wide"`) {
		t.Fatalf("이름 여러 개가 안 붙었다 : %s", got)
	}

	// #!style 로 정의한 이름과 짝이 맞아야 한다
	got = syntax_render(t, db, "{{{#!style\n.form { padding: 1rem; }\n}}}\n{{{#!wiki class=\"form\"\n내용}}}")
	if !strings.Contains(got, ".opennamu_main .form{") {
		t.Fatalf("#!style 이 울타리와 함께 나오지 않았다 : %s", got)
	}
	if !strings.Contains(got, `class="form"`) {
		t.Fatalf("#!wiki 의 class 가 없다 : %s", got)
	}
}

// TestMarkupWikiTag : {{{#!wiki tag="…"}}}
func TestMarkupWikiTag(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	got := syntax_render(t, db, "{{{#!wiki tag=\"span\" style=\"font-weight:700\"\n내용}}}")
	if !strings.Contains(got, `<span style="font-weight:700">`) || !strings.Contains(got, "</span>") {
		t.Fatalf("span 으로 감싸지 않았다 : %s", got)
	}

	// 적지 않으면 div
	got = syntax_render(t, db, "{{{#!wiki style=\"color:#f00\"\n내용}}}")
	if !strings.Contains(got, "<div style=\"color:#f00\">") {
		t.Fatalf("기본이 div 가 아니다 : %s", got)
	}

	// 모르는 태그는 div 로 돌아간다
	for _, bad := range []string{"script", "iframe", "table", "input", "style", "a", "임의"} {
		got := syntax_render(t, db, "{{{#!wiki tag=\""+bad+"\"\n내용}}}")
		if !strings.Contains(got, "<div") {
			t.Errorf("tag=%q : div 로 돌아가지 않았다 : %s", bad, got)
		}
		if strings.Contains(got, "<"+bad) {
			t.Errorf("tag=%q : 그 태그로 감쌌다 : %s", bad, got)
		}
	}
}

// TestMarkupWikiClassBlocked : 문서가 가져다 쓸 수 없는 class
//
//	스킨이 쓰는 이름을 문서가 붙이면 차림표나 머리글을 흉내 내거나 가려 버릴 수 있다.
func TestMarkupWikiClassBlocked(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	blocked := []string{
		"opennamu_category_blur", "opennamu_main", "namu_title_link",
		"bw_card", "article_head", "layout_row", "wiki-macro",
		"header", "footer", "sidebar", "menu", "title", "main", "search",
	}
	for _, one := range blocked {
		got := syntax_render(t, db, "{{{#!wiki class=\""+one+"\"\n내용}}}")
		if strings.Contains(got, `class="`) {
			t.Errorf("%q : 막혔어야 하는데 붙었다 : %s", one, got)
		}
	}

	// 속성 밖으로 빠져나가려는 값
	//
	//	못 알아본 글자는 본문 글로 밀려나 escape 된 채로 보인다.
	//	(`onclick=&quot;…&quot;` 처럼 글자로 보이는 것은 안전하다)
	//	여기서 막아야 하는 것은 「진짜 속성으로 살아나는 것」 이다.
	escape := []string{
		`a&quot; onclick=&quot;alert(1)`,
		`a" onclick="alert(1)`,
		`a><script>alert(1)</script`,
		`a onload=alert(1)`,
	}
	for _, one := range escape {
		got := syntax_render(t, db, "{{{#!wiki class=\""+one+"\"\n내용}}}")
		for _, bad := range []string{`onclick="`, `onload="`, "<script", "<img"} {
			if strings.Contains(got, bad) {
				t.Errorf("%q : %q 가 살아났다 : %s", one, bad, got)
			}
		}
	}

	// 개수 제한 (6개를 적으면 5개까지만)
	got := syntax_render(t, db, "{{{#!wiki class=\"a b c d e f\"\n내용}}}")
	if !strings.Contains(got, `class="a b c d e"`) {
		t.Fatalf("개수 제한이 걸리지 않았다 : %s", got)
	}
}

// TestMarkupNamespaceKoreanUrl : 주소에도 한국어 이름공간을 쓰는가
//
//	저장되는 제목은 `file:그림.png` 이지만 이용자에게 보이는 주소는 `파일:그림.png` 다.
func TestMarkupNamespaceKoreanUrl(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	// `[[파일:…]]` 은 그림을 끼워 넣는 문법이라 링크가 아니다. 여기서는 빼 둔다.
	cases := map[string]string{
		"[[틀:시험]]":   "틀",
		"[[사용자:홍길동]]": "사용자",
		"[[도움말:문법]]":  "도움말",
		"[[분류:시험]] 뒤": "분류", // 분류 상자 안의 링크
		"[[:파일:그림.png]]": "파일", // 앞에 : 를 붙이면 그냥 링크
	}

	for source, want := range cases {
		got := syntax_render(t, db, source)
		if !strings.Contains(got, `/w/`+url.QueryEscape(want)+`:`) {
			t.Errorf("%q : 한국어 주소가 아니다 : %s", source, got)
		}
	}

	// 내부 이름이 주소에 새어 나오면 안 된다
	got := syntax_render(t, db, "[[틀:시험]] [[사용자:홍길동]] [[도움말:문법]]")
	for _, bad := range []string{"/w/template:", "/w/user:", "/w/help:"} {
		if strings.Contains(got, bad) {
			t.Errorf("%q 가 주소에 그대로 나왔다 : %s", bad, got)
		}
	}

	// 이름공간이 없는 문서는 그대로다
	if got := syntax_render(t, db, "[[FrontPage]]"); !strings.Contains(got, `href="/w/FrontPage"`) {
		t.Fatalf("보통 문서 주소가 바뀌었다 : %s", got)
	}
}
