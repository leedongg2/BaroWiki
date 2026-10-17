package markup

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"opennamu/route/tool"
)

// syntax_test_db : 문법 시험용 빈 위키
func syntax_test_db(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "syntax.db"))
	if err != nil {
		t.Fatal(err)
	}

	for _, query := range []string{
		"create table other (name text default '', data text default '', coverage text default '')",
		"create table data (title text default '', data text default '', type text default '')",
		"create table back (title text default '', link text default '', type text default '', data text default '')",
		"create table data_set (doc_name text default '', doc_rev text default '', set_name text default '', set_data text default '')",
		"create table user_set (name text default '', id text default '', data text default '')",
		"create table html_filter (html text default '', kind text default '', plus text default '', plus_t text default '')",
		"create table history (id text default '', title text default '', data text default '', date text default '', ip text default '', send text default '', leng text default '', hide text default '', type text default '')",
	} {
		tool.Exec_DB(db, query)
	}

	return db
}

// syntax_render : 본문 한 덩이를 그려 본다
func syntax_render(t *testing.T, db *sql.DB, source string) string {
	t.Helper()

	out := render_namumark_compat(db, "시험", source, "view", nil, 0, false)
	data, _ := out["data"].(string)

	return data
}

// TestMarkupFontSyntax : 글꼴 문법
func TestMarkupFontSyntax(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	cases := []struct {
		source string
		want   []string
	}{
		{"{{{#!font 나눔고딕\n본문\n}}}", []string{"font-family:나눔고딕", "본문"}},
		{"{{{#!font \"Times New Roman\", serif 본문}}}", []string{"Times New Roman", "serif", "본문"}},
		{"{{{#!font 나눔고딕, 크기=140%, 색=#c00\n본문\n}}}", []string{"font-size:140%", "color:#c00"}},
		{"[font(나눔고딕, 글자)]", []string{"font-family:나눔고딕", "글자"}},
		{"[글꼴(serif, 글자)]", []string{"font-family:serif"}},
	}

	for _, one := range cases {
		got := syntax_render(t, db, one.source)
		for _, want := range one.want {
			if !strings.Contains(got, want) {
				t.Fatalf("%q : %q 가 없다\n결과 : %s", one.source, want, got)
			}
		}
	}

	// 글꼴 이름이 아니면 그대로 둔다 (아무 CSS 나 새어 들어가지 않게)
	if got := syntax_render(t, db, "{{{#!font url(evil) 본문}}}"); strings.Contains(got, "url(") && strings.Contains(got, "font-family") {
		t.Fatalf("이상한 값이 글꼴로 들어갔다 : %s", got)
	}
}

// TestMarkupFoldingSyntax : 접기 · 펼치기 문법
func TestMarkupFoldingSyntax(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	cases := []struct {
		source string
		want   []string
		avoid  []string
	}{
		{"{{{#!folding 제목\n내용\n}}}", []string{"<details", "제목", "내용"}, []string{" open>"}},
		{"{{{#!folding\n내용\n}}}", []string{"펼치기 · 접기"}, nil},
		{"{{{#!folding+ 미리 펼침\n내용\n}}}", []string{"open>", "미리 펼침"}, nil},
		{"{{{#!detail 자세히\n내용\n}}}", []string{"<details", "자세히"}, nil},
		{"{{{#!접기 [펼치기·접기]\n내용\n}}}", []string{"<details", "펼치기·접기"}, nil},
		{"{{{#!folding style=\"color:red\" 제목\n내용\n}}}", []string{"color:red", "제목"}, nil},
	}

	for _, one := range cases {
		got := syntax_render(t, db, one.source)
		for _, want := range one.want {
			if !strings.Contains(got, want) {
				t.Fatalf("%q : %q 가 없다\n결과 : %s", one.source, want, got)
			}
		}
		for _, avoid := range one.avoid {
			if strings.Contains(got, avoid) {
				t.Fatalf("%q : %q 가 있으면 안 된다\n결과 : %s", one.source, avoid, got)
			}
		}
	}
}

// TestMarkupFootnoteSyntax : 각주 문법
func TestMarkupFootnoteSyntax(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	// [각주] 를 적은 자리에 각주가 모여야 한다
	got := syntax_render(t, db, "본문[* 각주 내용]\n[각주]")
	if !strings.Contains(got, "opennamu_footnote") || !strings.Contains(got, "각주 내용") {
		t.Fatalf("각주가 나오지 않았다 : %s", got)
	}

	// 이름을 붙인 각주를 두 번 부르면 본문은 한 번만 실린다
	got = syntax_render(t, db, "가[*A 첫 각주] 나[*A]\n[각주]")
	if strings.Count(got, `opennamu_footnote_title`) != 1 {
		t.Fatalf("이름 붙인 각주가 두 줄로 갈라졌다 : %s", got)
	}
	if strings.Count(got, `>[A]<`) != 2 {
		t.Fatalf("각주 이름표가 본문에 두 번 나와야 한다 : %s", got)
	}

	// 먼저 부르고 나중에 내용을 적어도 내용이 붙는다
	got = syntax_render(t, db, "가[*가] 나[*가 나중 내용]\n[각주]")
	if !strings.Contains(got, "나중 내용") {
		t.Fatalf("뒤에 적은 각주 내용이 붙지 않았다 : %s", got)
	}

	// 표 안과 접기 안의 각주도 모인다
	got = syntax_render(t, db, "||칸[* 표 각주]||\n{{{#!folding 제목\n안[* 접힘 각주]\n}}}\n[각주]")
	if !strings.Contains(got, "표 각주") || !strings.Contains(got, "접힘 각주") {
		t.Fatalf("표·접기 안 각주가 빠졌다 : %s", got)
	}

	// [각주] 를 적지 않아도 문서 끝에 모인다
	got = syntax_render(t, db, "본문[* 끝 각주]")
	if !strings.Contains(got, "끝 각주") {
		t.Fatalf("[각주] 없이도 모여야 한다 : %s", got)
	}
}

// TestMarkupTableAlign : 표 정렬 문법
func TestMarkupTableAlign(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	cases := []struct {
		source string
		want   string
	}{
		{"||<:>가운데||", "text-align:center"},
		{"||<(>왼쪽||", "text-align:left"},
		{"||<)>오른쪽||", "text-align:right"},
		{"||<^>위||", "vertical-align:top"},
		{"||<v>아래||", "vertical-align:bottom"},
		{"||<^|2>위||", "vertical-align:top"},
		{"||<table align=center>표||", "margin:auto"},
		{"||<tablealign=right>표||", "float:right"},
		{"||<table caption=제목>표||", "<caption>"},
		{"||<-2>병합||", `colspan="2"`},
		{"||<|2>병합||", `rowspan="2"`},
		{"||<colspan=3>병합||", `colspan="3"`},
		{"||<#fff>칸||", "background:#fff"},
		{"||<#fff,#000>칸||", "background:#fff"},
		{"||<tablewidth=640><tablealign=center><tablebordercolor=#39c5bb,#50545e><white,black><nopad>[[파일:약 먹고 자자.jpg|width=100%]]||", "padding:0 !important;"},
		{"||<-3><tablealign=right><tablebordercolor=#39c5bb,#50545e><tablebgcolor=transparent><tablecolor=#212529,#ddd><colbgcolor=#ecfffb,#1c1d1f><#39c5bb,#383b40><color=white>제목||", "background:#39c5bb"},
		{"||<bgcolor=#f00>칸||", "background:#f00"},
	}

	for _, one := range cases {
		got := syntax_render(t, db, one.source)
		if !strings.Contains(got, one.want) {
			t.Fatalf("%q : %q 가 없다\n결과 : %s", one.source, one.want, got)
		}
	}

	sample := `||<tablewidth=640><tablealign=center><tablebordercolor=#39c5bb,#50545e><white,black><nopad> [[파일:약 먹고 자자.jpg|width=100%]] ||

||<-3><tablealign=right><tablebordercolor=#39c5bb,#50545e><tablebgcolor=transparent><tablecolor=#212529,#ddd><colbgcolor=#ecfffb,#1c1d1f><#39c5bb,#383b40><color=white> {{{+2 '''おくすり飲んで寝よう'''}}}[br]Okusuri Nonde Neyou | 약 먹고 자자 ||
|| '''가수''' |||| [[하츠네 미쿠]] ||
|| '''작곡가''' ||||<|3> [[모치우츠네]] ||
|| '''작사가''' ||||
|| '''영상 제작''' ||||
|| '''페이지''' |||| [[https://www.nicovideo.jp/watch/sm41188413|[[파일:니코니코 동화 아이콘.svg|width=24]]]] [include(틀:유튜브 아이콘, 링크=GWAtnzcbfFQ, 크기=27)] ||
|| '''투고일''' ||||<-2> [[2022년]] [[10월 7일]],,(니코니코 동화),, [br][[2022년]] [[10월 10일]],,(유튜브),, ||
|| '''달성 기록''' |||| [[VOCALOID 전당입성]][br][[VOCALOID 전설입성]] ||`
	got := syntax_render(t, db, sample)
	for _, literal := range []string{"&lt;white,black&gt;", "&lt;nopad&gt;", "&lt;#39c5bb,#383b40&gt;", "&lt;color=white&gt;"} {
		if strings.Contains(got, literal) {
			t.Fatalf("표 속성 문법이 화면에 그대로 노출됐다 (%s) : %s", literal, got)
		}
	}
	for _, style := range []string{"width:640px", "margin:auto", "border:2px solid #39c5bb", "background:#39c5bb", "color:white", "padding:0"} {
		if !strings.Contains(got, style) {
			t.Fatalf("표 속성 스타일이 적용되지 않았다 (%s) : %s", style, got)
		}
	}

	// 색이 아닌 낱말이 배경색으로 새어 들어가면 안 된다
	if got := syntax_render(t, db, "||<tablesortable>칸||"); strings.Contains(got, "background:tablesortable") {
		t.Fatalf("색이 아닌 값이 배경색이 됐다 : %s", got)
	}
}

func TestMarkupOuterLinkWithNestedFile(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	file_name := "니코니코 동화 아이콘.svg"
	file_link := "[[파일:" + file_name + "|width=24]]"
	outer_link := "[[https://www.nicovideo.jp/watch/sm41188413|" + file_link + "]]"

	got := syntax_render(t, db, outer_link)
	outer_href := `href="https://www.nicovideo.jp/watch/sm41188413">`
	outer_start := strings.Index(got, outer_href)
	outer_end := -1
	if outer_start >= 0 {
		outer_end = strings.Index(got[outer_start:], "</a>")
	}
	if outer_start < 0 || outer_end < 0 {
		t.Fatalf("외부 URL 링크가 만들어지지 않았다 : %s", got)
	}
	outer_label := got[outer_start : outer_start+outer_end]
	if !strings.Contains(outer_label, "(파일:"+file_name+")") || strings.Contains(outer_label, "<a ") {
		t.Fatalf("없는 파일은 파일 이름 대체 표시가 나와야 한다 : %s", got)
	}
	if strings.Contains(outer_label, "/upload?name=") {
		t.Fatalf("없는 파일의 업로드 링크가 바깥 URL 링크 안에 남았다 : %s", got)
	}

	tool.Exec_DB(db, "insert into data (title, data, type) values (?, ?, ?)", "file:"+file_name, "", "")
	got = syntax_render(t, db, outer_link)
	outer_start = strings.Index(got, outer_href)
	outer_end = -1
	if outer_start >= 0 {
		outer_end = strings.Index(got[outer_start:], "</a>")
	}
	if outer_start < 0 || outer_end < 0 {
		t.Fatalf("파일을 넣은 뒤 외부 URL 링크가 사라졌다 : %s", got)
	}
	outer_label = got[outer_start : outer_start+outer_end]
	if !strings.Contains(outer_label, "<img") || !strings.Contains(outer_label, "width:24px") || strings.Contains(outer_label, "<a ") {
		t.Fatalf("있는 파일은 이미지가 외부 링크 안에 표시되어야 한다 : %s", got)
	}
}

// TestMarkupColorDarkPair : 밝은 화면 · 어두운 화면 색 짝
func TestMarkupColorDarkPair(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	for _, source := range []string{"{{{#red,#blue 다크}}}", "{{{#f00,#00f 다크}}}"} {
		got := syntax_render(t, db, source)
		if !strings.Contains(got, "<span style=\"color:") {
			t.Fatalf("%q : 색이 적용되지 않았다\n결과 : %s", source, got)
		}
		if strings.Contains(got, ",#") {
			t.Fatalf("%q : 짝이 그대로 새어 나왔다\n결과 : %s", source, got)
		}
	}
}

// TestMarkupSocialMacro : 바깥 서비스 문법
func TestMarkupSocialMacro(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	cases := []struct {
		source string
		want   string
	}{
		{"[youtube(dQw4w9WgXcQ)]", "youtube.com/embed/dQw4w9WgXcQ"},
		{"[instagram(abc123)]", "instagram.com/p/abc123/embed/"},
		{"[naverblog(myid, 12345)]", "blog.naver.com/myid/12345"},
		{"[naverblog(https://blog.naver.com/myid/222333)]", "blog.naver.com/myid/222333"},
		{"[tistory(myblog, 77)]", "myblog.tistory.com/77"},
		{"[tistory(https://myblog.tistory.com/99)]", "myblog.tistory.com/99"},
		{"[chzzk(abc123)]", "chzzk.naver.com/embed/video/abc123"},
		{"[계정(유튜브, @handle)]", "youtube.com/@handle"},
		{"[account(tistory, myblog)]", "myblog.tistory.com"},
		{"[계정(네이버블로그, someid)]", "blog.naver.com/someid"},
		{"[계정(인스타그램, insta_id)]", "instagram.com/insta_id/"},
	}

	for _, one := range cases {
		got := syntax_render(t, db, one.source)
		if !strings.Contains(got, one.want) {
			t.Fatalf("%q : %q 가 없다\n결과 : %s", one.source, one.want, got)
		}
	}

	// 아이디 모양이 아니면 그대로 둔다
	if got := syntax_render(t, db, "[계정(유튜브, <script>)]"); strings.Contains(got, "<script>") {
		t.Fatalf("이상한 아이디가 그대로 나갔다 : %s", got)
	}
}

// TestMarkupAccountHelper : 사용자 설정에서 쓰는 계정 도우미
func TestMarkupAccountHelper(t *testing.T) {
	if got := Markup_account_id("youtube", "https://www.youtube.com/@barowiki"); got != "barowiki" {
		t.Fatalf("유튜브 아이디를 뽑지 못했다 : %q", got)
	}
	if got := Markup_account_id("tistory", "https://barowiki.tistory.com/"); got != "barowiki" {
		t.Fatalf("티스토리 이름을 뽑지 못했다 : %q", got)
	}
	if got := Markup_account_id("naverblog", "barowiki"); got != "barowiki" {
		t.Fatalf("네이버 블로그 아이디를 뽑지 못했다 : %q", got)
	}
	if got := Markup_account_id("youtube", "이상한 값 <>"); got != "" {
		t.Fatalf("아이디가 아닌 값을 받아들였다 : %q", got)
	}
	if got := Markup_account_id("없는것", "abc"); got != "" {
		t.Fatal("모르는 종류를 받아들였다")
	}

	html := Markup_account_html("instagram", "@baro_wiki")
	if !strings.Contains(html, "instagram.com/baro_wiki/") || !strings.Contains(html, "opennamu_account") {
		t.Fatalf("계정 상자를 만들지 못했다 : %s", html)
	}
}

// TestMarkupCategoryLink : 분류 달기와 분류로 가는 링크
func TestMarkupCategoryLink(t *testing.T) {
	db := syntax_test_db(t)
	defer db.Close()

	// [[분류:X]] 는 문서를 그 분류에 넣는다 (아래 분류 상자)
	got := syntax_render(t, db, "본문\n[[분류:시험]]")
	if !strings.Contains(got, `class="opennamu_category"`) {
		t.Fatalf("분류 상자가 없다 : %s", got)
	}

	// [[:분류:X]] 는 그냥 링크다 (분류 상자가 생기면 안 된다)
	got = syntax_render(t, db, "본문 [[:분류:시험]]")
	if strings.Contains(got, `class="opennamu_category"`) {
		t.Fatalf("앞에 : 를 붙였는데 분류가 달렸다 : %s", got)
	}

	// 주소에도 한국어 이름공간을 쓴다 (내부 이름인 category: 가 새어 나오면 안 된다)
	if !strings.Contains(got, `href="/w/`+url.QueryEscape("분류")+`:`) {
		t.Fatalf("분류로 가는 한국어 주소가 없다 : %s", got)
	}
	if strings.Contains(got, "/w/category") {
		t.Fatalf("주소에 내부 이름공간이 그대로 나왔다 : %s", got)
	}
	if !strings.Contains(got, `title="분류:시험"`) {
		t.Fatalf("링크 설명이 한국어가 아니다 : %s", got)
	}

	// 분류 상자에는 펼치기 단추가 함께 붙는다
	got = syntax_render(t, db, "[[분류:가]]\n[[분류:나]]")
	if !strings.Contains(got, "opennamu_category_button") {
		t.Fatalf("분류 펼치기 단추가 없다 : %s", got)
	}
}
