package tool

import (
	"strings"
	"testing"
)

// TestSvgScriptCheck : 스크립트가 든 SVG 를 걸러 내는가
func TestSvgScriptCheck(t *testing.T) {
	// 그냥 그림인 SVG 는 그대로 지나가야 한다
	good := []string{
		`<svg xmlns="http://www.w3.org/2000/svg"><circle cx="5" cy="5" r="4"/></svg>`,
		`<svg viewBox="0 0 10 10"><path d="M0 0 L10 10" stroke="#000"/></svg>`,
		`<svg><text font-size="10">onload 이야기</text></svg>`, // 글 속의 낱말은 속성이 아니다
		`<svg><style>.a{fill:red}</style><rect class="a"/></svg>`,
		"",
	}
	for _, one := range good {
		if Svg_script_check([]byte(one)) {
			t.Errorf("그림인데 막았다 : %q", one)
		}
	}

	// 스크립트가 될 수 있는 것은 막는다
	bad := []string{
		`<svg><script>alert(1)</script></svg>`,
		`<svg><SCRIPT>alert(1)</SCRIPT></svg>`,
		`<svg onload="alert(1)"></svg>`,
		`<svg><circle onclick="alert(1)"/></svg>`,
		"<svg\n  onload =\n  \"alert(1)\"></svg>",
		`<svg><a xlink:href="javascript:alert(1)"><rect/></a></svg>`,
		`<svg><foreignObject><body xmlns="http://www.w3.org/1999/xhtml">x</body></foreignObject></svg>`,
		`<svg><handler>alert(1)</handler></svg>`,
		`<svg><image href="data:text/html,<script>alert(1)</script>"/></svg>`,
		"<svg><scr\x00ipt>alert(1)</scr\x00ipt></svg>",
	}
	for _, one := range bad {
		if !Svg_script_check([]byte(one)) {
			t.Errorf("스크립트가 들었는데 지나갔다 : %q", one)
		}
	}
}

// TestSvgInDefaultExtList : svg 가 기본 허용 확장자에 들어 있는가
func TestSvgInDefaultExtList(t *testing.T) {
	found := false
	for _, one := range Get_file_default_ext_list() {
		if one == "svg" {
			found = true
		}
	}

	if !found {
		t.Fatalf("기본 허용 확장자에 svg 가 없다 : %v", Get_file_default_ext_list())
	}

	if Get_file_media_type("svg") != "image" {
		t.Fatalf("svg 를 그림으로 보지 않는다")
	}
}

// TestGetTitleLinkHtml : 문서 제목을 자기 문서로 가는 링크로 감싸는가
func TestGetTitleLinkHtml(t *testing.T) {
	// 링크 대상이 없으면 예전 그대로다 (문서가 아닌 화면의 제목)
	plain := Get_title_link_html("최근 변경", "")
	if strings.Contains(plain, "<a ") {
		t.Fatalf("문서가 아닌데 링크가 걸렸다 : %s", plain)
	}
	if plain != Get_title_html("최근 변경") {
		t.Fatalf("링크가 없을 때는 예전과 같아야 한다 : %s", plain)
	}

	// 문서 화면은 자기 자신으로 가는 링크가 된다
	linked := Get_title_link_html("FrontPage", "FrontPage")
	if !strings.Contains(linked, `href="/w/FrontPage"`) {
		t.Fatalf("자기 문서로 가는 링크가 없다 : %s", linked)
	}
	if !strings.Contains(linked, "namu_title_link") {
		t.Fatalf("제목 링크 표시가 없다 : %s", linked)
	}

	// 이름공간 강조는 링크 안에 그대로 남는다
	namespaced := Get_title_link_html("file:그림.png", "file:그림.png")
	if !strings.Contains(namespaced, "namu_title_namespace") {
		t.Fatalf("이름공간 강조가 사라졌다 : %s", namespaced)
	}
	if !strings.Contains(namespaced, "파일:") {
		t.Fatalf("이름공간이 한국어로 보이지 않는다 : %s", namespaced)
	}

	// 따옴표가 섞인 이름도 주소가 깨지지 않는다
	tricky := Get_title_link_html(`"><script>`, `"><script>`)
	if strings.Contains(tricky, "<script>") {
		t.Fatalf("제목으로 태그를 넣을 수 있다 : %s", tricky)
	}

	// 빈 제목은 링크로 감싸지 않는다
	if Get_title_link_html("", "") != "" {
		t.Fatalf("빈 제목에 무언가 나왔다")
	}
}
