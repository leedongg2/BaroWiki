package tool

import (
	"database/sql"
	"strconv"
	"strings"
)

// ============================================================
//  바로위키 UI 부품
//
//  모든 바로위키 화면은 이 파일의 부품만 조합해서 만든다.
//  (화면마다 제각각인 HTML 을 쓰지 않기 위한 규칙)
// ============================================================

// Baro_page : 바로위키 표준 페이지
func Baro_page(db *sql.DB, config Config, title string, body string, menu [][]any) string {
	// 화면을 내보내기 직전에 언어를 확인한다 (Baro_t 가 이 값을 쓴다)
	Wiki_language(db)

	full := Baro_css() + Baro_top_nav(db, config) + `<div class="bw_wrap">` + body + `</div>`

	if len(menu) == 0 {
		menu = [][]any{{"baro", "바로위키"}}
	}

	return Get_template(db, config, Baro_t(title), full, []any{}, Baro_t_menu(menu), map[string]string{})
}

// Baro_top_nav : 바로위키 공통 상단 이동 줄
func Baro_top_nav(db *sql.DB, config Config) string {
	items := [][]string{
		{"/baro", "홈"},
		{"/baro/founding", "창립 시즌"},
		{"/baro/field", "분야"},
		{"/baro/project", "프로젝트"},
		{"/baro/task", "할 일"},
		{"/baro/rank", "기여"},
		{"/baro/discover", "발견"},
		{"/baro/ceremony", "위키 수상식"},
		{"/baro/req", "편집 요청"},
		{"/baro/log", "운영 로그"},
		{"/baro/policy", "정책"},
	}

	html := `<nav class="bw_nav"><div class="bw_nav_in">`
	for _, item := range items {
		html += `<a class="bw_nav_a" href="` + item[0] + `">` + Baro_t_esc(item[1]) + `</a>`
	}

	if Baro_is_user(config.IP) {
		count := Baro_notify_count(db, config.IP)
		alarm := Baro_t("알림")
		if count > 0 {
			alarm = Baro_t("알림") + " " + strconv.Itoa(count)
		}

		html += `<span class="bw_nav_sp"></span>`
		html += `<a class="bw_nav_a bw_nav_me" href="/baro/record/user/` + Url_parser(config.IP) + `/all">` + Baro_t_esc("내 기록") + `</a>`
		html += `<a class="bw_nav_a bw_nav_me" href="/baro/profile/` + Url_parser(config.IP) + `">` + Baro_t_esc("내 기여") + `</a>`
		html += `<a class="bw_nav_a" href="/baro/notify">` + HTML_escape(alarm) + `</a>`
	} else {
		html += `<span class="bw_nav_sp"></span>`
		html += `<a class="bw_nav_a bw_nav_me" href="/baro/founding">` + Baro_t_esc("창립 기여자 지원") + `</a>`
	}

	html += `</div></nav>`

	return html
}

// Baro_hero : 큰 머리말 영역
func Baro_hero(title string, sub string, body string) string {
	html := `<section class="bw_hero"><h1 class="bw_hero_t">` + Baro_t_esc(title) + `</h1>`
	if sub != "" {
		html += `<p class="bw_hero_s">` + Baro_t_esc(sub) + `</p>`
	}
	if body != "" {
		html += `<div class="bw_hero_b">` + body + `</div>`
	}

	return html + `</section>`
}

// Baro_card : 제목 있는 카드
func Baro_card(title string, body string) string {
	html := `<section class="bw_card">`
	if title != "" {
		html += `<h2 class="bw_card_t">` + Baro_t_esc(title) + `</h2>`
	}

	return html + `<div class="bw_card_b">` + body + `</div></section>`
}

// Baro_card_ex : 부제·꼬리말이 있는 카드
func Baro_card_ex(title string, sub string, body string, foot string) string {
	html := `<section class="bw_card">`
	if title != "" {
		html += `<h2 class="bw_card_t">` + Baro_t_esc(title) + `</h2>`
	}
	if sub != "" {
		html += `<p class="bw_card_s">` + Baro_t_esc(sub) + `</p>`
	}
	html += `<div class="bw_card_b">` + body + `</div>`
	if foot != "" {
		html += `<div class="bw_card_f">` + foot + `</div>`
	}

	return html + `</section>`
}

// Baro_section : 카드 안의 작은 구획
func Baro_section(title string, body string) string {
	return `<div class="bw_sec"><h3 class="bw_sec_t">` + Baro_t_esc(title) + `</h3><div>` + body + `</div></div>`
}

// Baro_grid : 반응형 격자 (열 수는 최대치, 화면이 좁으면 자동으로 줄어든다)
func Baro_grid(items []string, cols int) string {
	if cols < 1 {
		cols = 1
	}
	if cols > 4 {
		cols = 4
	}

	html := `<div class="bw_grid bw_grid` + strconv.Itoa(cols) + `">`
	for _, item := range items {
		html += `<div class="bw_grid_i">` + item + `</div>`
	}

	return html + `</div>`
}

// Baro_stat : 지표 한 칸
func Baro_stat(label string, value string, sub string) string {
	html := `<div class="bw_stat"><div class="bw_stat_v">` + Baro_t_esc(value) + `</div>`
	html += `<div class="bw_stat_l">` + Baro_t_esc(label) + `</div>`
	if sub != "" {
		html += `<div class="bw_stat_s">` + Baro_t_esc(sub) + `</div>`
	}

	return html + `</div>`
}

// Baro_stat_row : 지표 여러 칸 ([라벨, 값, 부가설명])
func Baro_stat_row(stats [][]string) string {
	html := `<div class="bw_stats">`
	for _, row := range stats {
		sub := ""
		if len(row) > 2 {
			sub = row[2]
		}
		if len(row) < 2 {
			continue
		}

		html += Baro_stat(row[0], row[1], sub)
	}

	return html + `</div>`
}

// Baro_progress : 진행 막대
func Baro_progress(done int, total int, label string) string {
	pct := Baro_pct(done, total)
	width := strconv.FormatFloat(pct, 'f', 1, 64)

	html := `<div class="bw_prog_wrap">`
	if label != "" {
		html += `<div class="bw_prog_l"><span>` + Baro_t_esc(label) + `</span><span>` + Baro_comma(done) + " / " + Baro_comma(total) + ` (` + width + `%)</span></div>`
	}
	html += `<div class="bw_prog"><div class="bw_prog_in" style="width:` + width + `%"></div></div>`

	return html + `</div>`
}

// Baro_progress_open : 분모가 확정되지 않은 분야용 (가짜 정밀도 금지 — 6-2)
func Baro_progress_open(done int, goal int, label string) string {
	if goal <= 0 {
		return `<div class="bw_prog_wrap"><div class="bw_prog_l"><span>` + Baro_t_esc(label) + `</span><span>` + Baro_comma(done) + Baro_t_esc("개 기록됨") + `</span></div>` +
			Baro_t(`<div class="bw_note">전체 목록이 확정되지 않은 분야라 개척률 대신 기록 수만 표시합니다.</div></div>`)
	}

	return Baro_progress(done, goal, label+" "+Baro_t("(프로젝트 목표 기준)"))
}

// Baro_badge : 상태 표시 (kind : ok / warn / danger / info / muted)
func Baro_badge(text string, kind string) string {
	if kind == "" {
		kind = "muted"
	}

	return `<span class="bw_badge bw_badge_` + kind + `">` + Baro_t_esc(text) + `</span>`
}

// Baro_alert : 안내 상자 (kind : ok / warn / danger / info)
func Baro_alert(kind string, text string) string {
	if kind == "" {
		kind = "info"
	}

	return `<div class="bw_alert bw_alert_` + kind + `">` + text + `</div>`
}

// Baro_note : 작은 회색 설명 줄
func Baro_note(text string) string {
	return `<p class="bw_note">` + Baro_t_esc(text) + `</p>`
}

// Baro_empty : 목록이 비었을 때
func Baro_empty(text string) string {
	return `<div class="bw_empty">` + Baro_t_esc(text) + `</div>`
}

// Baro_table : 표 (rows 의 각 칸은 이미 HTML 이어야 한다)
func Baro_table(headers []string, rows [][]string, empty string) string {
	html := `<div class="bw_table_wrap"><table class="bw_table"><thead><tr>`
	for _, header := range headers {
		html += `<th>` + Baro_t_esc(header) + `</th>`
	}
	html += `</tr></thead><tbody>`

	if len(rows) == 0 {
		if empty == "" {
			empty = "표시할 내용이 없습니다."
		}
		html += `<tr><td class="bw_table_e" colspan="` + strconv.Itoa(len(headers)) + `">` + Baro_t_esc(empty) + `</td></tr>`
	}

	for _, row := range rows {
		html += `<tr>`
		for _, cell := range row {
			html += `<td>` + cell + `</td>`
		}
		html += `</tr>`
	}

	return html + `</tbody></table></div>`
}

// Baro_dl : 이름-값 목록
func Baro_dl(pairs [][]string) string {
	html := `<dl class="bw_dl">`
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}

		html += `<dt>` + Baro_t_esc(pair[0]) + `</dt><dd>` + pair[1] + `</dd>`
	}

	return html + `</dl>`
}

// Baro_list : 단순 목록 (항목은 이미 HTML)
func Baro_list(items []string) string {
	if len(items) == 0 {
		return Baro_empty("표시할 내용이 없습니다.")
	}

	html := `<ul class="bw_list">`
	for _, item := range items {
		html += `<li>` + item + `</li>`
	}

	return html + `</ul>`
}

// Baro_pills : 태그 모양 목록
func Baro_pills(items []string) string {
	html := `<div class="bw_pills">`
	for _, item := range items {
		html += `<span class="bw_pill">` + Baro_t_esc(item) + `</span>`
	}

	return html + `</div>`
}

// Baro_button : 버튼 모양 링크 (kind : primary / danger / "" )
func Baro_button(url string, text string, kind string) string {
	class := "bw_btn"
	if kind != "" {
		class += " bw_btn_" + kind
	}

	return `<a class="` + class + `" href="` + HTML_escape(url) + `">` + Baro_t_esc(text) + `</a>`
}

// Baro_submit : 제출 버튼
func Baro_submit(text string, kind string) string {
	class := "bw_btn"
	if kind != "" {
		class += " bw_btn_" + kind
	}

	return `<button type="submit" class="` + class + `">` + Baro_t_esc(text) + `</button>`
}

// Baro_form : 폼 감싸기
func Baro_form(action string, body string) string {
	return `<form class="bw_form" method="post" action="` + HTML_escape(action) + `">` + body + `</form>`
}

// Baro_form_file : 파일 업로드 폼
func Baro_form_file(action string, body string) string {
	return `<form class="bw_form" method="post" enctype="multipart/form-data" action="` + HTML_escape(action) + `">` + body + `</form>`
}

// Baro_field_row_html : 폼 한 줄
func Baro_field_row_html(label string, control string, help string) string {
	html := `<div class="bw_f"><label class="bw_f_l">` + Baro_t_esc(label) + `</label>` + control
	if help != "" {
		html += `<div class="bw_f_h">` + Baro_t_esc(help) + `</div>`
	}

	return html + `</div>`
}

// Baro_input : 한 줄 입력
func Baro_input(name string, label string, value string, placeholder string, help string) string {
	control := `<input class="bw_in" name="` + HTML_escape(name) + `" value="` + HTML_escape(value) + `" placeholder="` + Baro_t_esc(placeholder) + `">`

	return Baro_field_row_html(label, control, help)
}

// Baro_input_type : 종류를 지정한 입력 (date / datetime-local / number / password / email / url)
func Baro_input_type(name string, label string, value string, kind string, help string) string {
	control := `<input class="bw_in" type="` + HTML_escape(kind) + `" name="` + HTML_escape(name) + `" value="` + HTML_escape(value) + `">`

	return Baro_field_row_html(label, control, help)
}

// Baro_textarea : 여러 줄 입력
func Baro_textarea(name string, label string, value string, rows int, placeholder string, help string) string {
	if rows < 2 {
		rows = 5
	}

	control := `<textarea class="bw_in bw_ta" rows="` + strconv.Itoa(rows) + `" name="` + HTML_escape(name) + `" placeholder="` + Baro_t_esc(placeholder) + `">` + HTML_escape(value) + `</textarea>`

	return Baro_field_row_html(label, control, help)
}

// Baro_select : 선택 상자 (options : [값, 표시 이름])
func Baro_select(name string, label string, options [][]string, selected string, help string) string {
	control := `<select class="bw_in" name="` + HTML_escape(name) + `">`
	for _, option := range options {
		if len(option) < 2 {
			continue
		}

		mark := ""
		if option[0] == selected {
			mark = ` selected`
		}

		control += `<option value="` + HTML_escape(option[0]) + `"` + mark + `>` + Baro_t_esc(option[1]) + `</option>`
	}
	control += `</select>`

	return Baro_field_row_html(label, control, help)
}

// Baro_checkbox : 체크 상자
func Baro_checkbox(name string, label string, value string, checked bool, help string) string {
	mark := ""
	if checked {
		mark = ` checked`
	}

	control := `<label class="bw_cb"><input type="checkbox" name="` + HTML_escape(name) + `" value="` + HTML_escape(value) + `"` + mark + `> <span>` + Baro_t_esc(label) + `</span></label>`
	if help != "" {
		control += `<div class="bw_f_h">` + Baro_t_esc(help) + `</div>`
	}

	return `<div class="bw_f">` + control + `</div>`
}

// Baro_hidden : 숨은 값
func Baro_hidden(name string, value string) string {
	return `<input type="hidden" name="` + HTML_escape(name) + `" value="` + HTML_escape(value) + `">`
}

// Baro_tabs : 탭 줄 (items : [주소, 이름])
func Baro_tabs(items [][]string, active string) string {
	html := `<div class="bw_tabs">`
	for _, item := range items {
		if len(item) < 2 {
			continue
		}

		class := "bw_tab"
		if item[0] == active {
			class += " bw_tab_on"
		}

		html += `<a class="` + class + `" href="` + HTML_escape(item[0]) + `">` + Baro_t_esc(item[1]) + `</a>`
	}

	return html + `</div>`
}

// Baro_crumb : 이동 경로 (items : [주소, 이름] / 주소가 비면 현재 위치)
func Baro_crumb(items [][]string) string {
	parts := []string{}
	for _, item := range items {
		if len(item) < 2 {
			continue
		}

		if item[0] == "" {
			parts = append(parts, `<span>`+Baro_t_esc(item[1])+`</span>`)
			continue
		}

		parts = append(parts, `<a href="`+HTML_escape(item[0])+`">`+Baro_t_esc(item[1])+`</a>`)
	}

	return `<div class="bw_crumb">` + strings.Join(parts, ` <span class="bw_crumb_s">›</span> `) + `</div>`
}

// Baro_paging : 쪽 넘기기
func Baro_paging(page int, count int, max_count int, url string) string {
	html := `<div class="bw_paging">`

	if page > 1 {
		html += `<a class="bw_btn" href="` + HTML_escape(strings.ReplaceAll(url, "{}", strconv.Itoa(page-1))) + `">` + Baro_t_esc("이전") + `</a>`
	}

	html += `<span class="bw_paging_n">` + strconv.Itoa(page) + Baro_t_esc("쪽") + `</span>`

	if count >= max_count {
		html += `<a class="bw_btn" href="` + HTML_escape(strings.ReplaceAll(url, "{}", strconv.Itoa(page+1))) + `">` + Baro_t_esc("다음") + `</a>`
	}

	return html + `</div>`
}

// Baro_doc_link : 문서 링크
func Baro_doc_link(title string) string {
	if title == "" {
		return `<span class="bw_muted">-</span>`
	}

	return `<a href="/w/` + Url_parser(title) + `">` + HTML_escape(Namespace_display(title)) + `</a>`
}

// Baro_user_link : 사용자 링크 (바로위키 프로필로 연결)
func Baro_user_link(db *sql.DB, user string, my_ip string) string {
	if user == "" {
		return `<span class="bw_muted">-</span>`
	}

	if !Baro_is_user(user) {
		return IP_parser(db, user, my_ip)
	}

	html := `<a class="bw_user" href="/baro/profile/` + Url_parser(user) + `">` + HTML_escape(user) + `</a>`

	if number := Baro_founder_number(db, user); number != "" {
		html += Baro_t(` <span class="bw_founder" title="창립 기여자">#`) + HTML_escape(number) + `</span>`
	}

	return html
}

// Baro_time_cell : 표에 들어갈 시각 (제목에 정확한 시각)
func Baro_time_cell(data string) string {
	if data == "" {
		return `<span class="bw_muted">-</span>`
	}

	return `<span class="bw_time" title="` + HTML_escape(data) + `">` + HTML_escape(Baro_time_ago(data)) + `</span>`
}

// Baro_status_badge : 문서 상태 배지
func Baro_status_badge(status string) string {
	if status == "" {
		return `<span class="bw_muted">-</span>`
	}

	return Baro_badge(Baro_doc_status_name(status), Baro_doc_status_kind(status))
}

// Baro_kv_grid : 값이 이미 HTML 인 이름-값 격자
func Baro_kv_grid(pairs [][]string) string {
	html := `<div class="bw_kv">`
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}

		html += `<div class="bw_kv_i"><span class="bw_kv_k">` + Baro_t_esc(pair[0]) + `</span><span class="bw_kv_v">` + pair[1] + `</span></div>`
	}

	return html + `</div>`
}

// Baro_code : 고정폭 상자
func Baro_code(text string) string {
	return `<pre class="bw_code">` + HTML_escape(text) + `</pre>`
}

// Baro_danger_zone : 되돌릴 수 없는 작업 구역
func Baro_danger_zone(title string, body string) string {
	return `<section class="bw_card bw_danger"><h2 class="bw_card_t">` + Baro_t_esc(title) + `</h2><div class="bw_card_b">` + body + `</div></section>`
}

// Baro_css : 바로위키 화면 공통 스타일 (스킨 위에 얹는다)
func Baro_css() string {
	return `<style id="bw_css">
.bw_wrap{--bw-line:#d9dde2;--bw-soft:#f6f8fa;--bw-mut:#6b7280;--bw-acc:#010080;--bw-acc2:#0014de;}
.bw_nav{margin:0 0 1rem 0;border-bottom:1px solid var(--bw-line,#d9dde2);}
.bw_nav_in{display:flex;flex-wrap:wrap;align-items:center;gap:.15rem;padding:.15rem 0 .5rem 0;}
.bw_nav_a{display:inline-block;padding:.35rem .7rem;border-radius:999px;font-size:.92rem;font-weight:500;text-decoration:none;}
.bw_nav_a:hover{background:rgba(0,166,156,.12);text-decoration:none;}
.bw_nav_me{font-weight:700;}
.bw_nav_sp{flex:1 1 auto;}
.bw_wrap{line-height:1.65;}
.bw_hero{padding:1.6rem 0 1.2rem 0;border-bottom:1px solid var(--bw-line,#d9dde2);margin-bottom:1.2rem;}
.bw_hero_t{margin:0 0 .4rem 0;font-size:1.9rem;font-weight:800;border:0;}
.bw_hero_s{margin:0 0 .8rem 0;color:var(--bw-mut,#6b7280);font-size:1.02rem;}
.bw_hero_b{margin-top:.6rem;}
.bw_card{border:1px solid var(--bw-line,#d9dde2);border-radius:8px;padding:1.1rem 1.2rem;margin:0 0 1rem 0;background:rgba(127,127,127,.02);}
.bw_card_t{margin:0 0 .2rem 0;font-size:1.15rem;font-weight:700;border:0;padding:0;}
.bw_card_s{margin:.1rem 0 .7rem 0;color:var(--bw-mut,#6b7280);font-size:.9rem;}
.bw_card_b{margin-top:.7rem;}
.bw_card_f{margin-top:.9rem;padding-top:.7rem;border-top:1px solid var(--bw-line,#d9dde2);font-size:.9rem;}
.bw_sec{margin:1rem 0;}
.bw_sec_t{margin:0 0 .4rem 0;font-size:1rem;font-weight:700;border:0;padding:0;}
.bw_grid{display:grid;gap:.8rem;}
.bw_grid2{grid-template-columns:repeat(auto-fit,minmax(240px,1fr));}
.bw_grid3{grid-template-columns:repeat(auto-fit,minmax(200px,1fr));}
.bw_grid4{grid-template-columns:repeat(auto-fit,minmax(165px,1fr));}
.bw_grid1{grid-template-columns:1fr;}
.bw_stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(130px,1fr));gap:.7rem;margin:.6rem 0;}
.bw_stat{border:1px solid var(--bw-line,#d9dde2);border-radius:8px;padding:.7rem .8rem;text-align:center;}
.bw_stat_v{font-size:1.5rem;font-weight:800;line-height:1.2;}
.bw_stat_l{font-size:.85rem;color:var(--bw-mut,#6b7280);margin-top:.15rem;}
.bw_stat_s{font-size:.78rem;color:var(--bw-mut,#6b7280);margin-top:.2rem;}
.bw_prog_wrap{margin:.6rem 0;}
.bw_prog_l{display:flex;justify-content:space-between;gap:.5rem;font-size:.88rem;margin-bottom:.3rem;flex-wrap:wrap;}
.bw_prog{height:.6rem;border-radius:999px;background:rgba(127,127,127,.18);overflow:hidden;}
.bw_prog_in{height:100%;border-radius:999px;background:linear-gradient(90deg,#010080,#0014de);}
.bw_badge{display:inline-block;padding:.1rem .5rem;border-radius:999px;font-size:.78rem;font-weight:600;line-height:1.6;white-space:nowrap;}
.bw_badge_ok{background:rgba(40,180,114,.16);color:#1a7f4f;}
.bw_badge_warn{background:rgba(230,160,20,.18);color:#8a6100;}
.bw_badge_danger{background:rgba(220,67,67,.16);color:#b12b2b;}
.bw_badge_info{background:rgba(2,117,216,.14);color:#1064ad;}
.bw_badge_muted{background:rgba(127,127,127,.16);color:#5b6169;}
.bw_alert{border-radius:8px;padding:.75rem .9rem;margin:.6rem 0;border:1px solid transparent;font-size:.94rem;}
.bw_alert_info{background:rgba(2,117,216,.08);border-color:rgba(2,117,216,.28);}
.bw_alert_ok{background:rgba(40,180,114,.1);border-color:rgba(40,180,114,.3);}
.bw_alert_warn{background:rgba(230,160,20,.12);border-color:rgba(230,160,20,.35);}
.bw_alert_danger{background:rgba(220,67,67,.1);border-color:rgba(220,67,67,.32);}
.bw_note{color:var(--bw-mut,#6b7280);font-size:.86rem;margin:.35rem 0;}
.bw_muted{color:var(--bw-mut,#6b7280);}
.bw_empty{padding:1.4rem;text-align:center;color:var(--bw-mut,#6b7280);border:1px dashed var(--bw-line,#d9dde2);border-radius:8px;font-size:.92rem;}
.bw_table_wrap{overflow-x:auto;margin:.6rem 0;}
.bw_table{width:100%;border-collapse:collapse;font-size:.92rem;background:transparent;}
.bw_table th,.bw_table td{border:1px solid var(--bw-line,#d9dde2);padding:.45rem .6rem;text-align:left;vertical-align:top;}
.bw_table th{background:rgba(127,127,127,.08);font-weight:700;white-space:nowrap;}
.bw_table_e{text-align:center;color:var(--bw-mut,#6b7280);padding:1.2rem;}
.bw_dl{margin:.5rem 0;}
.bw_dl dt{font-weight:700;font-size:.88rem;color:var(--bw-mut,#6b7280);margin-top:.55rem;}
.bw_dl dd{margin:.1rem 0 0 0;}
.bw_list{margin:.4rem 0 .4rem 1.2rem;}
.bw_list li{margin:.25rem 0;}
.bw_pills{display:flex;flex-wrap:wrap;gap:.35rem;margin:.4rem 0;}
.bw_pill{display:inline-block;padding:.15rem .6rem;border-radius:999px;border:1px solid var(--bw-line,#d9dde2);font-size:.83rem;}
.bw_btn{display:inline-block;padding:.35rem .85rem;border-radius:6px;border:1px solid var(--bw-line,#d9dde2);font-size:.9rem;font-weight:600;text-decoration:none;cursor:pointer;background:transparent;color:inherit;}
.bw_btn:hover{background:rgba(127,127,127,.1);text-decoration:none;}
.bw_btn_primary{background:linear-gradient(90deg,#010080,#0014de);border-color:transparent;color:#fff;}
.bw_btn_primary:hover{opacity:.9;background:linear-gradient(90deg,#010080,#0014de);color:#fff;}
.bw_btn_danger{border-color:rgba(220,67,67,.55);color:#c33;}
.bw_btn_danger:hover{background:rgba(220,67,67,.1);}
.bw_form{margin:.6rem 0;}
.bw_f{margin:.7rem 0;}
.bw_f_l{display:block;font-size:.88rem;font-weight:700;margin-bottom:.25rem;}
.bw_f_h{font-size:.82rem;color:var(--bw-mut,#6b7280);margin-top:.25rem;}
.bw_in{width:100%;max-width:100%;box-sizing:border-box;padding:.45rem .6rem;border:1px solid var(--bw-line,#d9dde2);border-radius:6px;font-size:.95rem;font-family:inherit;background:transparent;color:inherit;}
.bw_ta{min-height:5rem;resize:vertical;line-height:1.6;}
.bw_cb{display:flex;align-items:center;gap:.4rem;font-size:.93rem;cursor:pointer;}
.bw_cb input{width:auto;margin:0;}
.bw_tabs{display:flex;flex-wrap:wrap;gap:.3rem;margin:.8rem 0;border-bottom:1px solid var(--bw-line,#d9dde2);padding-bottom:.4rem;}
.bw_tab{padding:.3rem .75rem;border-radius:6px;font-size:.9rem;text-decoration:none;}
.bw_tab:hover{background:rgba(127,127,127,.1);text-decoration:none;}
.bw_tab_on{background:rgba(0,166,156,.14);font-weight:700;}
.bw_crumb{font-size:.86rem;color:var(--bw-mut,#6b7280);margin:.2rem 0 .8rem 0;}
.bw_crumb_s{opacity:.6;}
.bw_paging{display:flex;align-items:center;gap:.6rem;margin:1rem 0;}
.bw_paging_n{font-size:.88rem;color:var(--bw-mut,#6b7280);}
.bw_kv{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:.5rem;margin:.5rem 0;}
.bw_kv_i{display:flex;gap:.5rem;font-size:.92rem;border-bottom:1px dotted var(--bw-line,#d9dde2);padding-bottom:.3rem;}
.bw_kv_k{color:var(--bw-mut,#6b7280);min-width:6.5rem;font-weight:600;}
.bw_kv_v{flex:1;}
.bw_code{background:rgba(127,127,127,.1);border:1px solid var(--bw-line,#d9dde2);border-radius:6px;padding:.7rem .8rem;overflow-x:auto;font-size:.86rem;white-space:pre-wrap;word-break:break-all;}
.bw_danger{border-color:rgba(220,67,67,.5);}
.bw_founder{font-size:.78rem;font-weight:700;color:#1a7f4f;}
.bw_user{font-weight:600;}
.bw_time{white-space:nowrap;}
.bw_cal{display:grid;grid-template-columns:repeat(auto-fill,12px);gap:3px;margin:.6rem 0;}
.bw_cal_d{width:12px;height:12px;border-radius:2px;background:rgba(127,127,127,.14);}
.bw_cal_1{background:rgba(40,180,114,.35);}
.bw_cal_2{background:rgba(40,180,114,.55);}
.bw_cal_3{background:rgba(40,180,114,.75);}
.bw_cal_4{background:rgba(40,180,114,.95);}
.bw_share{border:1px solid var(--bw-line,#d9dde2);border-radius:10px;padding:1.2rem;background:linear-gradient(135deg,rgba(0,166,156,.1),rgba(40,180,114,.05));}
.bw_share_t{font-size:1.15rem;font-weight:800;margin-bottom:.4rem;}
@media (max-width:640px){
.bw_hero_t{font-size:1.45rem;}
.bw_card{padding:.9rem;}
.bw_nav_a{padding:.3rem .55rem;font-size:.86rem;}
.bw_stat_v{font-size:1.25rem;}
}
</style>`
}
