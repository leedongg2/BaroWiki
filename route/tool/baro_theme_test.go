package tool

import (
	"database/sql"
	"strings"
	"testing"
)

// ============================================================
//  테마 시험
// ============================================================

// baro_theme_boot : 테마에 필요한 표만 갖춘 작은 DB
func baro_theme_boot(t *testing.T) *sql.DB {
	t.Helper()

	db := baro_uuid_test_db(t)

	for _, query := range []string{
		"create table baro_set (name text default '', data text default '')",
		"create table baro_theme (id text default '', uuid text default '', title text default '', " +
			"description text default '', maker text default '', data text default '', status text default '', " +
			"official text default '', date text default '', edit_date text default '', views text default '', " +
			"note text default '')",
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}

	return db
}

// TestBaroThemeColorNorm : 색 표기 다듬기
func TestBaroThemeColorNorm(t *testing.T) {
	ok := map[string]string{
		"#fff":       "#fff",
		"#FFFFFF":    "#ffffff",
		"fff":        "#fff",
		"  #0047ab ": "#0047ab",
		"white":      "#ffffff",
		"RED":        "#ff0000",
		"gray":       "#808080",
		"grey":       "#808080",
		"green":      "#008000",
		"#12345678":  "#12345678",
	}

	for typed, want := range ok {
		if got := Baro_theme_color_norm(typed); got != want {
			t.Fatalf("%q → %q (기대 %q)", typed, got, want)
		}
	}

	if got := Baro_theme_color_norm("rgba(0, 71, 171, 0.5)"); got != "rgba(0, 71, 171, 0.5)" {
		t.Fatalf("rgba 가 막혔다: %q", got)
	}

	// 스타일을 타고 바깥을 부르는 값은 받지 않는다.
	for _, bad := range []string{
		"", "   ", "보라색", "url(https://a.example/x.png)", "expression(alert(1))",
		"#fff;background:url(//x)", "javascript:alert(1)", "red; }", "#gggggg",
		"var(--namu-text-color)", "rgb(0,0,0) url(x)",
	} {
		if got := Baro_theme_color_norm(bad); got != "" {
			t.Fatalf("받으면 안 되는 값이 통과했다: %q → %q", bad, got)
		}
	}
}

// TestBaroThemeValue : 밝은 쪽과 어두운 쪽 묶고 가르기
func TestBaroThemeValue(t *testing.T) {
	joined := Baro_theme_value_join("#fff", "#000")
	if joined != "#fff|#000" {
		t.Fatalf("묶은 값이 다르다: %q", joined)
	}

	light, dark := Baro_theme_value_split(joined)
	if light != "#fff" || dark != "#000" {
		t.Fatalf("가른 값이 다르다: %q %q", light, dark)
	}

	// 한쪽만 적어도 된다.
	if got := Baro_theme_value_join("", "#000"); got != "|#000" {
		t.Fatalf("한쪽만 적은 값이 다르다: %q", got)
	}

	// 둘 다 못 쓰는 값이면 담지 않는다.
	if got := Baro_theme_value_join("보라", "초록"); got != "" {
		t.Fatalf("못 쓰는 값이 담겼다: %q", got)
	}
}

// TestBaroThemeCss : 담긴 색만 CSS 로 나온다
func TestBaroThemeCss(t *testing.T) {
	row := Baro_theme_row{Data: map[string]string{
		"page_bg": "#f7f9ff|#0b1020",
		"link":    "#0047ab|",
	}}

	light := Baro_theme_css(row, false)
	if !strings.Contains(light, "--namu-background-color: #f7f9ff") {
		t.Fatalf("밝은 쪽 배경이 없다: %s", light)
	}
	if !strings.Contains(light, "--namu-link-color: #0047ab") {
		t.Fatalf("밝은 쪽 링크가 없다: %s", light)
	}

	dark := Baro_theme_css(row, true)
	if !strings.Contains(dark, "--namu-background-color: #0b1020") {
		t.Fatalf("어두운 쪽 배경이 없다: %s", dark)
	}

	// 어두운 쪽 값을 비워 두면 스킨 값을 그대로 쓰므로 건드리지 않는다.
	if strings.Contains(dark, "--namu-link-color") {
		t.Fatalf("비워 둔 어두운 쪽 값이 덮어써졌다: %s", dark)
	}

	// 담은 것이 없으면 아무것도 내놓지 않는다.
	if got := Baro_theme_css(Baro_theme_row{Data: map[string]string{}}, false); got != "" {
		t.Fatalf("빈 테마가 CSS 를 내놓았다: %q", got)
	}

	// 문단 접기를 끄면 규칙이 붙는다.
	fold := Baro_theme_css(Baro_theme_row{Data: map[string]string{"fold": "off"}}, false)
	if !strings.Contains(fold, "opennamu_heading_fold") {
		t.Fatalf("문단 접기 규칙이 없다: %s", fold)
	}
}

// TestBaroThemeCssSafe : 못 쓰는 값은 CSS 로 새어 나가지 않는다
func TestBaroThemeCssSafe(t *testing.T) {
	// 표를 거치지 않고 직접 DB 에 밀어 넣은 값도 그릴 때 한 번 더 거른다.
	row := Baro_theme_row{Data: map[string]string{
		"page_bg": "#fff;} body{background:url(//x)|#000",
		"text":    "url(//evil.example/x.png)|#fff",
	}}

	css := Baro_theme_css(row, false)
	for _, bad := range []string{"url(", "}", "evil.example"} {
		if strings.Contains(css, bad) {
			t.Fatalf("걸러졌어야 할 값이 CSS 에 들어갔다 (%s): %s", bad, css)
		}
	}
}

// TestBaroThemeCrud : 만들고 고치고 지우기
func TestBaroThemeCrud(t *testing.T) {
	db := baro_theme_boot(t)

	uuid := Baro_theme_add(db, Baro_theme_row{
		Title: "밤하늘", Description: "설명", Maker: "만든이",
		Status: "public", Data: map[string]string{"page_bg": "#fff|#000"},
	})
	if !Baro_uuid_is(uuid) {
		t.Fatalf("고유 번호가 판 4 가 아니다: %q", uuid)
	}

	row, found := Baro_theme_get(db, uuid)
	if !found || row.Title != "밤하늘" || row.Maker != "만든이" {
		t.Fatalf("만든 테마를 되읽지 못했다: %+v", row)
	}
	if !row.Baro_theme_public() {
		t.Fatal("공개로 만들었는데 비공개다")
	}

	// 공개 목록에 오른다
	if len(Baro_theme_list(db, "public", "", 0, 10)) != 1 {
		t.Fatal("공개 목록에 오르지 않았다")
	}

	// 비공개로 바꾸면 목록에서 빠지되 번호로는 열린다
	row.Status = "private"
	row.Title = "밤하늘 (고침)"
	Baro_theme_save(db, row)

	if len(Baro_theme_list(db, "public", "", 0, 10)) != 0 {
		t.Fatal("비공개인데 공개 목록에 남아 있다")
	}
	if len(Baro_theme_list(db, "mine", "만든이", 0, 10)) != 1 {
		t.Fatal("내 목록에서 빠졌다")
	}

	again, found := Baro_theme_get(db, uuid)
	if !found || again.Title != "밤하늘 (고침)" {
		t.Fatalf("고친 내용이 남지 않았다: %+v", again)
	}

	// 공식 채택
	Baro_theme_official_set(db, uuid, true)
	if len(Baro_theme_list(db, "official", "", 0, 10)) != 1 {
		t.Fatal("공식 목록에 오르지 않았다")
	}

	// 없는 번호로는 아무것도 나오지 않는다
	if _, ok := Baro_theme_get(db, "00000000-0000-4000-8000-000000000000"); ok {
		t.Fatal("없는 번호로 테마가 나왔다")
	}
	if _, ok := Baro_theme_get(db, "그런거없음"); ok {
		t.Fatal("번호 꼴이 아닌 값으로 테마가 나왔다")
	}

	// 지우면 쓰고 있던 사람도 기본 색으로 돌아간다
	Baro_theme_use(db, "쓰는이", uuid)
	if Baro_theme_now(db, "쓰는이") != uuid {
		t.Fatal("테마를 고르지 못했다")
	}

	Baro_theme_delete(db, uuid)

	if _, ok := Baro_theme_get(db, uuid); ok {
		t.Fatal("지웠는데 남아 있다")
	}
	if Baro_theme_now(db, "쓰는이") != "" {
		t.Fatal("지운 테마를 아직 쓰고 있다")
	}
}

// TestBaroThemeHead : 쓰고 있는 테마만 CSS 로 나온다
func TestBaroThemeHead(t *testing.T) {
	db := baro_theme_boot(t)

	uuid := Baro_theme_add(db, Baro_theme_row{
		Title: "밤하늘", Maker: "만든이", Status: "public",
		Data: map[string]string{"page_bg": "#f7f9ff|#0b1020"},
	})

	if got := Baro_theme_head(db, "쓰는이", false); got != "" {
		t.Fatalf("고르지 않았는데 CSS 가 나왔다: %s", got)
	}

	Baro_theme_use(db, "쓰는이", uuid)

	if got := Baro_theme_head(db, "쓰는이", false); !strings.Contains(got, "#f7f9ff") {
		t.Fatalf("밝은 쪽 CSS 가 나오지 않았다: %s", got)
	}
	if got := Baro_theme_head(db, "쓰는이", true); !strings.Contains(got, "#0b1020") {
		t.Fatalf("어두운 쪽 CSS 가 나오지 않았다: %s", got)
	}

	// 아이피(비로그인)에게는 테마가 붙지 않는다
	if got := Baro_theme_head(db, "192.0.2.1", false); got != "" {
		t.Fatalf("비로그인에게 테마가 붙었다: %s", got)
	}
}

// TestBaroThemeTokenList : 토큰 정의가 성하다
func TestBaroThemeTokenList(t *testing.T) {
	seen_key := map[string]bool{}
	seen_var := map[string]bool{}

	for _, token := range Baro_theme_token_list() {
		switch {
		case token.Key == "":
			t.Fatal("열쇠가 빈 토큰이 있다")
		case token.Name == "":
			t.Fatalf("이름이 빈 토큰이 있다: %s", token.Key)
		case !strings.HasPrefix(token.Var, "--namu-"):
			t.Fatalf("스킨 토큰이 아니다: %s → %s", token.Key, token.Var)
		case seen_key[token.Key]:
			t.Fatalf("열쇠가 겹친다: %s", token.Key)
		case seen_var[token.Var]:
			t.Fatalf("CSS 변수가 겹친다: %s", token.Var)
		}

		// 적어 둔 기본값도 실제로 쓸 수 있는 색이어야 한다.
		if Baro_theme_color_norm(token.Light) == "" {
			t.Fatalf("밝은 쪽 기본값을 알아볼 수 없다: %s = %q", token.Key, token.Light)
		}
		if Baro_theme_color_norm(token.Dark) == "" {
			t.Fatalf("어두운 쪽 기본값을 알아볼 수 없다: %s = %q", token.Key, token.Dark)
		}

		seen_key[token.Key] = true
		seen_var[token.Var] = true
	}

	if len(seen_key) < 40 {
		t.Fatalf("토큰이 너무 적다: %d", len(seen_key))
	}
}
