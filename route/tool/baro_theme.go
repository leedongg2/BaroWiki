package tool

import (
	"database/sql"
	"regexp"
	"strings"
)

// ============================================================
//  사용자 지정 테마
//
//  누구나 테마를 만들어 두고 불러다 쓴다.
//    · 제목 · 설명 · 제작자가 붙는다.
//    · 테마마다 변하지 않는 고유 번호(UUID)가 붙고, 주소는 /baro/theme/<uuid> 다.
//    · 공개로 두면 테마 목록에 제목과 제작자가 오른다.
//      비공개면 목록에 오르지 않고 링크를 아는 사람만 연다.
//    · 운영진이 공식 테마로 채택하면 공식 목록에도 오른다.
//
//  값은 스킨의 --namu-* 토큰을 덮어쓰는 것이 전부다. (baro_theme_token.go)
// ============================================================

// Baro_theme_title_max : 제목 길이 한도
const Baro_theme_title_max = 60

// Baro_theme_desc_max : 설명 길이 한도
const Baro_theme_desc_max = 500

// Baro_theme_row : 테마 한 줄
type Baro_theme_row struct {
	ID          string
	UUID        string
	Title       string
	Description string
	Maker       string
	Data        map[string]string
	Status      string // public / private
	Official    bool
	Date        string
	EditDate    string
	Views       int
}

// Baro_theme_public : 목록에 오르는 테마인가
func (row Baro_theme_row) Baro_theme_public() bool {
	return row.Status == "public"
}

// baro_theme_color_re : 받아 주는 색 표기
//
//	#rgb · #rrggbb · #rrggbbaa 와 rgb()/rgba()/hsl()/hsla() 만 받는다.
//	url() 이나 expression() 같은 것이 토큰을 타고 들어오지 못하게 한다.
var baro_theme_color_re = regexp.MustCompile(`^#[0-9A-Fa-f]{3}$|^#[0-9A-Fa-f]{4}$|^#[0-9A-Fa-f]{6}$|^#[0-9A-Fa-f]{8}$|^(?:rgb|rgba|hsl|hsla)\([0-9A-Za-z%.,/ +-]{1,60}\)$`)

// baro_theme_word : 이름으로 적어도 받아 주는 색
//
//	표에 `#fff｜white` 처럼 두 가지로 적힌 것들이다.
var baro_theme_word = map[string]string{
	"white":       "#ffffff",
	"black":       "#000000",
	"red":         "#ff0000",
	"green":       "#008000",
	"blue":        "#0000ff",
	"gray":        "#808080",
	"grey":        "#808080",
	"orange":      "#ffa500",
	"transparent": "transparent",
}

// Baro_theme_color_norm : 적어 낸 색 하나를 다듬는다. ("" 면 쓸 수 없는 값)
func Baro_theme_color_norm(data string) string {
	data = strings.TrimSpace(data)
	if data == "" {
		return ""
	}

	if word, ok := baro_theme_word[strings.ToLower(data)]; ok {
		return word
	}

	// `fff` 처럼 우물 정을 빠뜨린 것은 붙여 준다.
	if !strings.HasPrefix(data, "#") && regexp.MustCompile(`^[0-9A-Fa-f]{3,8}$`).MatchString(data) {
		data = "#" + data
	}

	if !baro_theme_color_re.MatchString(data) {
		return ""
	}

	return strings.ToLower(data)
}

// Baro_theme_value_split : 담긴 값을 밝은 쪽과 어두운 쪽으로 가른다
func Baro_theme_value_split(data string) (string, string) {
	light, dark, _ := strings.Cut(data, "|")

	return strings.TrimSpace(light), strings.TrimSpace(dark)
}

// Baro_theme_value_join : 두 쪽을 한 줄로 묶는다 ("" 면 담지 않는다)
func Baro_theme_value_join(light string, dark string) string {
	light = Baro_theme_color_norm(light)
	dark = Baro_theme_color_norm(dark)

	if light == "" && dark == "" {
		return ""
	}

	return light + "|" + dark
}

// ------------------------------------------------------------
//  읽고 쓰기
// ------------------------------------------------------------

// baro_theme_select : 공통 조회 칸
const baro_theme_select = "select id, uuid, title, description, maker, data, status, official, date, edit_date, views from baro_theme"

// baro_theme_scan : 한 줄 읽기
func baro_theme_scan(rows *sql.Rows) (Baro_theme_row, bool) {
	row := Baro_theme_row{}
	data, official, views := "", "", ""

	if rows.Scan(
		&row.ID, &row.UUID, &row.Title, &row.Description, &row.Maker,
		&data, &row.Status, &official, &row.Date, &row.EditDate, &views,
	) != nil {
		return row, false
	}

	row.Data = Baro_json_dec_str(data)
	row.Official = official == "on"
	row.Views = Str_to_int(views)

	if row.Status != "public" {
		row.Status = "private"
	}

	return row, true
}

// Baro_theme_get : 고유 번호로 테마 하나
func Baro_theme_get(db *sql.DB, uuid string) (Baro_theme_row, bool) {
	uuid = strings.TrimSpace(uuid)
	if !Baro_uuid_is(uuid) {
		return Baro_theme_row{}, false
	}

	rows := Query_DB(db, baro_theme_select+" where uuid = ?", uuid)
	defer rows.Close()

	if !rows.Next() {
		return Baro_theme_row{}, false
	}

	return baro_theme_scan(rows)
}

// Baro_theme_list : 조건에 맞는 테마 목록
//
//	kind : "official" 공식 · "public" 공개 · "mine" 내가 만든 것
func Baro_theme_list(db *sql.DB, kind string, maker string, offset int, size int) []Baro_theme_row {
	if size <= 0 {
		size = Baro_page_size
	}
	if offset < 0 {
		offset = 0
	}

	query := baro_theme_select
	args := []any{}

	switch kind {
	case "official":
		query += " where official = 'on'"
	case "mine":
		query += " where maker = ?"
		args = append(args, maker)
	default:
		query += " where status = 'public'"
	}

	query += " order by id + 0 desc limit ?, ?"
	args = append(args, offset, size)

	rows := Query_DB(db, query, args...)
	defer rows.Close()

	out := []Baro_theme_row{}
	for rows.Next() {
		if row, ok := baro_theme_scan(rows); ok {
			out = append(out, row)
		}
	}

	return out
}

// Baro_theme_count : 조건에 맞는 테마 수
func Baro_theme_count(db *sql.DB, kind string, maker string) int {
	count := 0

	switch kind {
	case "official":
		QueryRow_DB(db, "select count(*) from baro_theme where official = 'on'", []any{&count})
	case "mine":
		QueryRow_DB(db, "select count(*) from baro_theme where maker = ?", []any{&count}, maker)
	default:
		QueryRow_DB(db, "select count(*) from baro_theme where status = 'public'", []any{&count})
	}

	return count
}

// Baro_theme_add : 테마를 새로 만든다. 만들어진 고유 번호를 돌려준다.
func Baro_theme_add(db *sql.DB, row Baro_theme_row) string {
	uuid := Baro_uuid_new()
	if uuid == "" {
		return ""
	}

	official := ""
	if row.Official {
		official = "on"
	}

	Exec_DB(
		db,
		"insert into baro_theme (id, uuid, title, description, maker, data, status, official, date, edit_date, views, note) "+
			"values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '0', '')",
		Baro_next_id(db, "baro_theme"), uuid,
		Baro_trunc(row.Title, Baro_theme_title_max),
		Baro_trunc(row.Description, Baro_theme_desc_max),
		row.Maker, Baro_json_enc(row.Data), row.Status, official,
		Baro_now(), Baro_now(),
	)

	return uuid
}

// Baro_theme_save : 이미 있는 테마를 고친다
func Baro_theme_save(db *sql.DB, row Baro_theme_row) {
	Exec_DB(
		db,
		"update baro_theme set title = ?, description = ?, data = ?, status = ?, edit_date = ? where uuid = ?",
		Baro_trunc(row.Title, Baro_theme_title_max),
		Baro_trunc(row.Description, Baro_theme_desc_max),
		Baro_json_enc(row.Data), row.Status, Baro_now(), row.UUID,
	)
}

// Baro_theme_official_set : 공식 테마로 채택하거나 내린다
func Baro_theme_official_set(db *sql.DB, uuid string, on bool) {
	value := ""
	if on {
		value = "on"
	}

	Exec_DB(db, "update baro_theme set official = ? where uuid = ?", value, uuid)
}

// Baro_theme_delete : 테마를 지운다. 쓰고 있던 사람은 기본 색으로 돌아간다.
func Baro_theme_delete(db *sql.DB, uuid string) {
	Exec_DB(db, "delete from baro_theme where uuid = ?", uuid)
	Exec_DB(db, "delete from baro_user_set where name = 'theme' and data = ?", uuid)
}

// Baro_theme_view_bump : 열어 본 횟수 올리기
func Baro_theme_view_bump(db *sql.DB, uuid string) {
	Exec_DB(db, "update baro_theme set views = views + 1 where uuid = ?", uuid)
}

// Baro_theme_can_edit : 이 사람이 고치거나 지울 수 있는가
//
//	만든 사람 본인과 운영진만 건드린다.
func Baro_theme_can_edit(db *sql.DB, row Baro_theme_row, ip string) bool {
	if !Baro_is_user(ip) {
		return false
	}

	return row.Maker == ip || Baro_can(db, ip, "system")
}

// ------------------------------------------------------------
//  쓰고 있는 테마
// ------------------------------------------------------------

// Baro_theme_now : 이 사람이 쓰고 있는 테마 고유 번호 ("" 면 기본 색)
func Baro_theme_now(db *sql.DB, ip string) string {
	if !Baro_is_user(ip) {
		return ""
	}

	data := ""
	QueryRow_DB(db, "select data from baro_user_set where user = ? and name = 'theme'", []any{&data}, ip)

	return strings.TrimSpace(data)
}

// Baro_theme_use : 쓸 테마를 고른다 ("" 면 기본 색으로 돌아간다)
func Baro_theme_use(db *sql.DB, ip string, uuid string) {
	if !Baro_is_user(ip) {
		return
	}

	Exec_DB(db, "delete from baro_user_set where user = ? and name = 'theme'", ip)

	if uuid != "" {
		Exec_DB(db, "insert into baro_user_set (user, name, data) values (?, 'theme', ?)", ip, uuid)
	}
}

// ------------------------------------------------------------
//  CSS 만들기
// ------------------------------------------------------------

// Baro_theme_css : 테마 하나를 <style> 한 덩어리로 만든다
//
//	dark 가 참이면 어두운 쪽 값을, 아니면 밝은 쪽 값을 쓴다.
//	값이 비어 있는 토큰은 건드리지 않아 스킨 값이 그대로 남는다.
func Baro_theme_css(row Baro_theme_row, dark bool) string {
	lines := []string{}

	for _, token := range Baro_theme_token_list() {
		saved, ok := row.Data[token.Key]
		if !ok {
			continue
		}

		light_value, dark_value := Baro_theme_value_split(saved)

		want := light_value
		if dark {
			want = dark_value
		}

		// 값이 없으면 스킨 값을 그대로 쓴다.
		if want = Baro_theme_color_norm(want); want == "" {
			continue
		}

		lines = append(lines, token.Var+": "+want+";")
	}

	css := ""
	if len(lines) > 0 {
		css = ":root{" + strings.Join(lines, "") + "}"
	}

	// 문단 접기를 끄면 표시 기호를 감춘다.
	if row.Data["fold"] == "off" {
		css += ".opennamu_main .opennamu_heading_fold{display:none;}"
	}

	if css == "" {
		return ""
	}

	return `<style id="bw_theme_css">` + css + `</style>`
}

// Baro_theme_head : 지금 이 사람에게 넣을 테마 CSS
//
//	화면을 그리는 길목에서 부른다. 테마를 안 쓰거나 없는 테마면 빈 문자열이다.
//	여기서 난 잘못이 화면을 죽이면 안 되므로 받아 넘긴다.
func Baro_theme_head(db *sql.DB, ip string, dark bool) (css string) {
	defer func() {
		if recover() != nil {
			css = ""
		}
	}()

	uuid := Baro_theme_now(db, ip)
	if uuid == "" {
		return ""
	}

	row, found := Baro_theme_get(db, uuid)
	if !found {
		return ""
	}

	return Baro_theme_css(row, dark)
}

// Baro_theme_preview : 테마를 미리 보여 주는 작은 상자
//
//	토큰 몇 개만 골라 실제 색으로 칠한다. 쓰기 전에 눈으로 가늠하라는 것이다.
func Baro_theme_preview(row Baro_theme_row, dark bool) string {
	pick := func(key string) string {
		token, ok := Baro_theme_token_get(key)
		if !ok {
			return ""
		}

		fallback := token.Light
		if dark {
			fallback = token.Dark
		}

		saved, has := row.Data[key]
		if !has {
			return fallback
		}

		light_value, dark_value := Baro_theme_value_split(saved)

		want := light_value
		if dark {
			want = dark_value
		}

		if want = Baro_theme_color_norm(want); want == "" {
			return fallback
		}

		return want
	}

	side := Baro_t("밝은 쪽")
	if dark {
		side = Baro_t("어두운 쪽")
	}

	html := `<div class="bw_theme_pv" style="background:` + HTML_escape(pick("page_bg")) +
		`;color:` + HTML_escape(pick("text")) + `;border-color:` + HTML_escape(pick("table_border")) + `">`
	html += `<div class="bw_theme_pv_h" style="background:` + HTML_escape(pick("nav_bg")) +
		`;color:` + HTML_escape(pick("nav_text")) + `">` + Baro_t_esc(side) + `</div>`
	html += `<div class="bw_theme_pv_b">`
	html += `<p style="color:` + HTML_escape(pick("text")) + `">` + Baro_t_esc("본문 글자입니다.") + ` ` +
		`<a style="color:` + HTML_escape(pick("link")) + `">` + Baro_t_esc("있는 문서") + `</a> · ` +
		`<a style="color:` + HTML_escape(pick("link_none")) + `">` + Baro_t_esc("없는 문서") + `</a> · ` +
		`<span style="color:` + HTML_escape(pick("comment")) + `">` + Baro_t_esc("주석") + `</span></p>`
	html += `<div class="bw_theme_pv_q" style="background:` + HTML_escape(pick("quote_bg")) +
		`;border-color:` + HTML_escape(pick("quote_border")) +
		`;border-left-color:` + HTML_escape(pick("quote_accent")) + `">` + Baro_t_esc("인용문") + `</div>`
	html += `<div class="bw_theme_pv_t" style="background:` + HTML_escape(pick("table_bg")) +
		`;border-color:` + HTML_escape(pick("table_border")) + `">` + Baro_t_esc("표 칸") + `</div>`
	html += `</div></div>`

	return html
}

// Baro_theme_preview_css : 미리보기 상자 모양
func Baro_theme_preview_css() string {
	return `<style>
.bw_theme_pv { border: 1px solid; border-radius: 8px; overflow: hidden; font-size: 13px; }
.bw_theme_pv_h { padding: 8px 12px; font-weight: 600; }
.bw_theme_pv_b { padding: 12px; display: grid; gap: 8px; }
.bw_theme_pv_b p { margin: 0; }
.bw_theme_pv_q { padding: 8px 10px; border: 1px solid; border-left-width: 4px; border-radius: 4px; }
.bw_theme_pv_t { padding: 8px 10px; border: 1px solid; border-radius: 4px; }
.bw_theme_sw { display: inline-block; width: 18px; height: 18px; border-radius: 4px;
    border: 1px solid rgba(128,128,128,0.5); vertical-align: middle; margin-right: 6px; }
.bw_theme_row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.bw_theme_row input[type=color] { width: 42px; height: 30px; padding: 0; border: 0; background: none; }
.bw_theme_row input[type=text] { width: 120px; }
</style>`
}
