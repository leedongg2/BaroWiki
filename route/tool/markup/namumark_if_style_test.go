package markup

import (
	"database/sql"
	"strings"
	"testing"
)

// markup_render_mode : 다크 모드를 정해서 그려 본다
func markup_render_mode(t *testing.T, db *sql.DB, darkmode string, source string) string {
	t.Helper()

	out := render_namumark_compat(db, "시험", source, "view", map[string]any{
		"__opennamu_skin_set": map[string]string{"main_css_darkmode": darkmode},
	}, 0, false)
	data, _ := out["data"].(string)

	return data
}

// TestMarkupIfCondition : 조건문 읽기
func TestMarkupIfCondition(t *testing.T) {
	parameter := map[string]any{
		"완료": "1",
		"수":  "7",
		"빈값": nil,
	}
	lookup := func(name string) (any, bool) {
		value, exists := parameter[name]

		return value, exists
	}

	cases := map[string]bool{
		// null 견주기
		"중단 == null": true,
		"중단 != null": false,
		"완료 == null": false,
		"완료 != null": true,
		"빈값 == null": true,

		// 값 견주기
		"완료 == 1": true,
		"완료 == 2": false,
		"완료 != 2": true,
		`완료 == "1"`: true,
		"중단 == 1":   false,
		"중단 != 1":   true,

		// 이어 붙이기
		"완료 != null && 중단 == null": true,
		"완료 == null && 중단 == null": false,
		"완료 == null || 중단 == null": true,
		"완료 == null || 중단 != null": false,

		// && 가 || 보다 세다 : false || (true && true)
		"완료 == null || 완료 != null && 중단 == null": true,

		// 괄호
		"(완료 == null || 중단 == null) && 완료 != null": true,
		"(완료 != null && 중단 != null) || 완료 == 1":    true,
		"(완료 == null)":                            false,

		// 크고 작음 (숫자일 때만)
		"수 > 3":  true,
		"수 < 3":  false,
		"수 >= 7": true,
		"수 <= 6": false,
		"완료 > 가나": false, // 숫자가 아니면 거짓

		// 모양이 어긋나면 거짓
		"완료":        false,
		"완료 ==":     false,
		"== null":   false,
		"완료 == null && ": false,
		"(완료 == null":    false,
		"완료 === null":    false,
		"":               false,
	}

	for condition, want := range cases {
		if got := Namumark_if_eval(condition, lookup); got != want {
			t.Errorf("%q : %v 를 바랐는데 %v", condition, want, got)
		}
	}
}

// TestMarkupIfOneLine : 한 줄로 쓴 조건문
//
//	조건이 어디서 끝나는지를 파서가 스스로 찾아야 한다.
func TestMarkupIfOneLine(t *testing.T) {
	parameter := map[string]any{"완료": "1", "수": "7"}
	lookup := func(name string) (any, bool) {
		value, exists := parameter[name]

		return value, exists
	}

	cases := []struct {
		source string
		want   bool
		rest   string
	}{
		{"완료 == null 라벨", false, "라벨"},
		{"중단 == null 라벨", true, "라벨"},
		{"완료 != null 이 사용자는 참가자입니다.", true, "이 사용자는 참가자입니다."},
		{"완료 == 1 빈칸 여러 개가 든 내용", true, "빈칸 여러 개가 든 내용"},
		{"완료 != null && 중단 == null 둘 다 맞다", true, "둘 다 맞다"},
		{"완료 == null || 중단 == null 하나만 맞아도", true, "하나만 맞아도"},
		{"(완료 == null || 중단 == null) && 완료 != null 괄호", true, "괄호"},
		{"수 > 3 크다", true, "크다"},
		{"수 < 3 작다", false, "작다"},

		// 값에 빈칸이 있으면 따옴표로 묶는다
		{`완료 == "1" 따옴표`, true, "따옴표"},
		{`종류 == "우리 프로젝트" 내용`, false, "내용"},

		// 내용이 없어도 된다
		{"중단 == null", true, ""},
		{"완료 == null", false, ""},
	}

	for _, one := range cases {
		matched, index, ok := Namumark_if_eval_line(one.source, lookup)
		if !ok {
			t.Errorf("%q : 읽지 못했다", one.source)

			continue
		}
		if matched != one.want {
			t.Errorf("%q : %v 를 바랐는데 %v", one.source, one.want, matched)
		}
		if rest := one.source[index:]; rest != one.rest {
			t.Errorf("%q : 남은 내용이 %q 여야 하는데 %q", one.source, one.rest, rest)
		}
	}

	// 모양이 어긋나면 읽지 못한 것으로 알려 준다
	for _, one := range []string{"완료", "== null 내용", "(완료 == null 내용", "완료 === null 내용"} {
		if _, _, ok := Namumark_if_eval_line(one, lookup); ok {
			t.Errorf("%q : 어긋난 조건인데 읽었다고 한다", one)
		}
	}
}

// TestMarkupIfOneLineRender : 한 줄 조건문을 실제로 그려 본다
func TestMarkupIfOneLineRender(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	// 참이면 내용이, 거짓이면 아무것도 나오지 않는다
	if got := syntax_render(t, db, "{{{#!if 중단 == null 보인다}}}"); !strings.Contains(got, "보인다") {
		t.Fatalf("참인데 내용이 없다 : %s", got)
	}
	if got := syntax_render(t, db, "{{{#!if 중단 != null 안 보여야}}}"); strings.Contains(got, "안 보여야") {
		t.Fatalf("거짓인데 내용이 나왔다 : %s", got)
	}

	// && 가 escape 된 채로 넘어와도 읽어야 한다
	if got := syntax_render(t, db, "{{{#!if 완료 == null && 중단 == null 둘 다 맞다}}}"); !strings.Contains(got, "둘 다 맞다") {
		t.Fatalf("&& 를 읽지 못했다 : %s", got)
	}

	// 앞뒤 글과 섞여도 자리를 지킨다
	got := syntax_render(t, db, "앞{{{#!if 중단 == null 가운데}}}뒤")
	for _, want := range []string{"앞", "가운데", "뒤"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q 가 없다 : %s", want, got)
		}
	}

	// 내용 안의 다른 문법도 그대로 산다
	if got := syntax_render(t, db, "{{{#!if 중단 == null '''굵게'''}}}"); !strings.Contains(got, "<b>굵게</b>") {
		t.Fatalf("내용 안의 문법이 죽었다 : %s", got)
	}

	// 여러 줄 쓰기는 그대로다 (값에 빈칸이 들어가도 된다)
	if got := syntax_render(t, db, "{{{#!if 종류 == 우리 프로젝트\n보인다}}}"); strings.Contains(got, "보인다") {
		t.Fatalf("여러 줄 조건이 거꾸로 읽혔다 : %s", got)
	}
}

// TestMarkupIfOneLineEscape : 한 줄 조건문의 내용이 그대로 새어 나가지 않는가
//
//	조건은 escape 를 되돌려 읽지만, 내용은 원래 글 그대로 내보내야 한다.
//	되돌린 채로 내보내면 이용자가 적어 둔 태그가 살아난다.
func TestMarkupIfOneLineEscape(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	cases := []string{
		"{{{#!if 중단 == null <script>alert(1)</script>}}}",
		"{{{#!if 완료 == null && 중단 == null <script>alert(1)</script>}}}",
		"{{{#!if 중단 == null <img src=x onerror=alert(1)>}}}",
		"{{{#!if 중단 == null 앞 <script>alert(1)</script> 뒤}}}",
	}

	// 태그가 태그로 살아나면 안 된다.
	// (`&lt;script&gt;` 처럼 글자로 보이는 것은 안전하다)
	for _, source := range cases {
		got := syntax_render(t, db, source)
		for _, bad := range []string{"<script", "<img"} {
			if strings.Contains(got, bad) {
				t.Errorf("%q 가 태그로 살아났다\n입력 : %s\n결과 : %s", bad, source, got)
			}
		}
	}

	// 글자 자체는 그대로 보여야 한다
	got := syntax_render(t, db, "{{{#!if 중단 == null <b>글자</b>}}}")
	if !strings.Contains(got, "글자") {
		t.Fatalf("내용이 사라졌다 : %s", got)
	}
}

// TestMarkupIfUserbox : 유저박스처럼 겹겹이 쌓인 조건문
//
//	완료 · 중단이 둘 다 넘어오지 않았으므로 「입니다.」 만 나와야 한다.
//	예전에는 && 를 읽지 못해 거꾸로 「였습니다.」 가 나왔다.
func TestMarkupIfUserbox(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	source := `{{{#!wiki style="border:1px solid gray;padding:12px;text-align:center"
[[프로젝트|{{{#!if 완료 == null && 중단 == null
[[파일:프로젝트.png|width=33]]}}}]] {{{#!wiki style="display:inline;"
{{{+1 이 사용자는 참가자{{{#!if 완료 == null && 중단 == null
입니다.}}}{{{#!if 완료 != null || 중단 != null
였습니다.}}}}}}}}}}}}`

	got := syntax_render(t, db, source)

	if !strings.Contains(got, "이 사용자는 참가자입니다.") {
		t.Fatalf("「입니다.」 가 나와야 한다\n결과 : %s", got)
	}
	if strings.Contains(got, "였습니다.") {
		t.Fatalf("「였습니다.」 는 나오면 안 된다\n결과 : %s", got)
	}
	// 링크 라벨 안의 조건문이 링크를 깨뜨리지 않아야 한다
	if !strings.Contains(got, `href="/w/%ED%94%84%EB%A1%9C%EC%A0%9D%ED%8A%B8"`) {
		t.Fatalf("링크가 그려지지 않았다\n결과 : %s", got)
	}
	if strings.Contains(got, "[[") {
		t.Fatalf("문법이 그대로 남았다\n결과 : %s", got)
	}
}

// TestMarkupSizeColorNest : 굵게 + 크기 + 색을 겹쳐 써도 어긋나지 않는가
func TestMarkupSizeColorNest(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	got := syntax_render(t, db, `'''{{{+2 {{{#0047ab,#393939 하하}}}}}}'''`)

	for _, want := range []string{"<b>", "font-size:140%", "color:#0047ab", "하하"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q 가 없다\n결과 : %s", want, got)
		}
	}
	if strings.Contains(got, "{{{") || strings.Contains(got, "}}}") {
		t.Fatalf("문법이 그대로 남았다\n결과 : %s", got)
	}

	// 다크 색을 골랐을 때는 뒤쪽 색을 쓴다
	dark := markup_render_mode(t, db, "1", `{{{#0047ab,#393939 하하}}}`)
	if !strings.Contains(dark, "color:#393939") {
		t.Fatalf("다크 색을 쓰지 않았다\n결과 : %s", dark)
	}
}

// TestMarkupStyleBlock : 문서 안 CSS
func TestMarkupStyleBlock(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	css := `.button { padding: .5rem 1rem; }
.button--danger a { color: #ce2c31 !important; }

@theseed-dark-mode {
    .button--danger a { color: #ff9592 !important; }
}`

	// 블록 형태와 지시문 형태가 같은 결과를 낸다
	block := markup_render_mode(t, db, "0", "{{{#!style\n"+css+"\n}}}\n본문")
	directive := markup_render_mode(t, db, "0", "#!style\n"+css)

	for _, got := range []string{block, directive} {
		if !strings.Contains(got, "<style>") {
			t.Fatalf("<style> 가 나오지 않았다\n결과 : %s", got)
		}
		// 본문 상자 안으로 울타리가 쳐져야 한다
		if !strings.Contains(got, ".opennamu_main .button{") {
			t.Fatalf("울타리가 없다\n결과 : %s", got)
		}
		if !strings.Contains(got, ".opennamu_main .button--danger a{") {
			t.Fatalf("겹친 고르개에 울타리가 없다\n결과 : %s", got)
		}
		// 라이트 모드에서는 다크 묶음이 통째로 빠진다
		if strings.Contains(got, "#ff9592") {
			t.Fatalf("라이트인데 다크 색이 남았다\n결과 : %s", got)
		}
		if strings.Contains(got, "@theseed-dark-mode") {
			t.Fatalf("다크 묶음 머리가 그대로 남았다\n결과 : %s", got)
		}
	}

	// 블록 뒤의 본문은 그대로 남는다
	if !strings.Contains(block, "본문") {
		t.Fatalf("뒤따르는 본문이 사라졌다\n결과 : %s", block)
	}

	// 다크 모드에서는 다크 묶음 안쪽을 꺼내 쓴다
	dark := markup_render_mode(t, db, "1", "{{{#!style\n"+css+"\n}}}")
	if !strings.Contains(dark, "#ff9592") {
		t.Fatalf("다크인데 다크 색이 없다\n결과 : %s", dark)
	}
	if !strings.Contains(dark, ".opennamu_main .button--danger a{") {
		t.Fatalf("다크 규칙에 울타리가 없다\n결과 : %s", dark)
	}

	// @media 안쪽에도 울타리가 쳐져야 한다
	media := markup_render_mode(t, db, "0", "{{{#!style\n@media (max-width: 500px) { .button { padding: 0; } }\n}}}")
	if !strings.Contains(media, "@media (max-width: 500px){.opennamu_main .button{") {
		t.Fatalf("@media 안쪽에 울타리가 없다\n결과 : %s", media)
	}
}

// TestMarkupStyleBlocked : 문서 안 CSS 로 할 수 없어야 하는 것
//
//	CSS 는 그림처럼 보이지만 남의 화면을 덮거나 바깥을 부를 수 있다.
//	하나라도 섞여 있으면 그 덩이를 통째로 버린다.
func TestMarkupStyleBlocked(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	cases := map[string]string{
		"태그 탈출":      ".a{}</style><script>alert(1)</script>",
		"바깥 그림 부르기":  ".a{background:url(http://evil.test/x.png)}",
		"바깥 CSS 불러오기": "@import url(http://evil.test/x.css);",
		"옛 브라우저 스크립트": ".a{width:expression(alert(1))}",
		"링크 스크립트":    ".a{background:javascript:alert(1)}",
		"화면 덮기":      ".a{position:fixed;top:0;left:0;width:100%;height:100%}",
		"붙박이":        ".a{position:sticky;top:0}",
		"글자 집합 바꾸기":  "@charset \"utf-8\"; .a{color:red}",
		"괄호 안 맞음":    ".a{color:red;",
		"닫는 괄호가 더 많음": ".a{color:red}}",
		"닫히지 않은 다크":  "@theseed-dark-mode { .a{color:red}",
	}

	for name, css := range cases {
		for _, source := range []string{"{{{#!style\n" + css + "\n}}}", "#!style\n" + css} {
			got := markup_render_mode(t, db, "0", source)
			if strings.Contains(got, "<style") {
				t.Errorf("%s : 막혔어야 하는데 통과했다\n결과 : %s", name, got)
			}
			if strings.Contains(got, "<script") {
				t.Errorf("%s : 스크립트가 새어 나왔다\n결과 : %s", name, got)
			}
		}
	}
}

// TestMarkupStyleNotDirective : `#!style` 처럼 보이지만 지시문이 아닌 것
func TestMarkupStyleNotDirective(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	for _, source := range []string{
		"#!styles\n.a{color:red}",   // 이름이 다르다
		"#!style 뒤에 글\n.a{color:red}", // 뒤에 다른 글자가 붙었다
		"본문\n#!style\n.a{color:red}",  // 첫 줄이 아니다
	} {
		got := markup_render_mode(t, db, "0", source)
		if strings.Contains(got, "<style") {
			t.Errorf("%q : 지시문이 아닌데 CSS 로 다뤘다\n결과 : %s", source, got)
		}
	}
}
