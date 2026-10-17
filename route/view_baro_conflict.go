package route

import (
	"opennamu/route/tool"
)

// ============================================================
//  편집 충돌 해결 화면 (3-3)
//
//  "편집 충돌입니다" 하고 끝내면 쓰던 글이 사라지고 신규 기여자도 같이 사라진다.
//  여기서는 내 글을 먼저 초안으로 보존한 뒤,
//  기준 판 · 내 수정 · 최신 수정을 3자 병합해 고를 수 있게 한다.
// ============================================================

// View_baro_conflict : 편집 충돌 해결
func View_baro_conflict(config tool.Config, doc_name string, my_data string, send string, base_rev string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	now_rev := tool.Get_document_revision(db, doc_name)

	base_text := tool.Baro_revision_text(db, doc_name, base_rev)

	now_text := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&now_text}, doc_name)

	merged_text, conflict_count, merged_count := tool.Baro_merge3(base_text, my_data, now_text)

	// 무엇보다 먼저 내 글을 잃지 않게 초안으로 보존한다
	draft_id := baro_conflict_save_draft(config, doc_name, my_data, send)

	// 그동안 누가 무엇을 바꿨는지
	other_user := ""
	other_send := ""
	other_date := ""
	tool.QueryRow_DB(
		db,
		"select ip, send, date from history where title = ? and id = ? limit 1",
		[]any{&other_user, &other_send, &other_date},
		doc_name, now_rev,
	)

	body := ""

	if conflict_count > 0 {
		body += tool.Baro_alert("warn",
			tool.Baro_t(`<b>편집 충돌이 났습니다.</b> 내가 편집하는 동안 다른 사람이 이 문서를 고쳤습니다. `)+
				tool.Baro_t(`겹치지 않는 부분은 자동으로 합쳤고, 겹치는 <b>`)+tool.Baro_itoa(conflict_count)+tool.Baro_t(`군데</b>만 직접 골라 주시면 됩니다.`))
	} else {
		body += tool.Baro_alert("ok",
			tool.Baro_t(`<b>자동으로 합쳤습니다.</b> 내가 편집하는 동안 다른 사람이 이 문서를 고쳤지만, `)+
				tool.Baro_t(`서로 겹치지 않아 그대로 합칠 수 있었습니다. 내용을 확인하고 저장해 주세요.`))
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("자동 병합"), tool.Baro_itoa(merged_count) + tool.Baro_t("군데"), tool.Baro_t("그대로 합쳐졌습니다")},
		{tool.Baro_t("직접 선택 필요"), tool.Baro_itoa(conflict_count) + tool.Baro_t("군데"), tool.Baro_t("아래에서 고르세요")},
		{tool.Baro_t("기준 판"), "r" + tool.HTML_escape(base_rev), tool.Baro_t("내가 편집을 시작한 판")},
		{tool.Baro_t("최신 판"), "r" + tool.HTML_escape(now_rev), tool.Baro_t("지금 저장된 판")},
	})

	other_html := `<ul class="bw_list">`
	other_html += tool.Baro_t(`<li>고친 사람 : `) + tool.Baro_user_link(db, other_user, config.IP) + `</li>`
	other_html += tool.Baro_t(`<li>고친 시각 : `) + tool.Baro_time_cell(other_date) + `</li>`
	if other_send != "" {
		other_html += tool.Baro_t(`<li>편집 요약 : `) + tool.HTML_escape(other_send) + `</li>`
	}
	other_html += `<li><a href="/diff/` + tool.Url_parser(base_rev) + `/` + tool.Url_parser(now_rev) + `/` + tool.Url_parser(doc_name) + tool.Baro_t(`">위키 기본 비교 화면에서 보기</a></li>`)
	other_html += `</ul>`

	body += tool.Baro_card(tool.Baro_t("그동안 무엇이 바뀌었나"), other_html+
		tool.Baro_diff_html(base_text, now_text, tool.Baro_t("기준 판 r")+base_rev, tool.Baro_t("최신 판 r")+now_rev))

	body += tool.Baro_card(tool.Baro_t("내가 고친 내용"), tool.Baro_diff_html(base_text, my_data, tool.Baro_t("기준 판 r")+base_rev, tool.Baro_t("내 수정")))

	if draft_id != "" {
		body += tool.Baro_alert("info",
			tool.Baro_t(`내가 쓰던 글은 초안으로 안전하게 보관했습니다. 이 화면을 닫아도 사라지지 않습니다. `)+
				tool.Baro_button("/baro/draft", tool.Baro_t("내 초안 보관함"), ""))
	}

	// 세 가지 본문을 숨겨 두고 버튼으로 갈아 끼운다
	form := tool.Baro_hidden("ver", now_rev)
	form += tool.Baro_hidden("copyright_agreement", "on")
	form += `<textarea id="baro_conflict_area" class="bw_in bw_ta" name="content" rows="22" spellcheck="false">` + tool.HTML_escape(merged_text) + `</textarea>`

	form += tool.Baro_t(`<div class="bw_f"><label class="bw_f_l">본문 고르기</label>`)
	form += tool.Baro_t(`<button type="button" class="bw_btn" onclick="baro_conflict_pick('merged')">병합본 (권장)</button> `)
	form += tool.Baro_t(`<button type="button" class="bw_btn" onclick="baro_conflict_pick('mine')">내 수정만</button> `)
	form += tool.Baro_t(`<button type="button" class="bw_btn" onclick="baro_conflict_pick('theirs')">최신 수정만</button>`)
	form += tool.Baro_t(`<div class="bw_f_h">병합본에는 겹치는 부분이 아래 표시로 남아 있습니다. 저장하기 전에 표시를 지우고 하나로 정리해 주세요.</div></div>`)

	form += tool.Baro_code(tool.Baro_mark_mine + tool.Baro_t("\n(내가 쓴 내용)\n") + tool.Baro_mark_split + tool.Baro_t("\n(다른 사람이 쓴 내용)\n") + tool.Baro_mark_theirs)

	form += tool.Baro_input("send", "편집 요약", send, "무엇을 고쳤는지 적어 주세요", "충돌을 어떻게 정리했는지 적어 두면 다른 기여자가 이해하기 쉽습니다.")
	form += tool.Baro_submit(tool.Baro_t("이 내용으로 저장"), "primary")

	body += tool.Baro_card_ex("합쳐진 본문", "확인하고 저장하세요", tool.Baro_form("/edit/"+tool.Url_parser(doc_name), form), "")

	body += `<script>
var baro_conflict_data = {
  merged: ` + baro_conflict_js(merged_text) + `,
  mine: ` + baro_conflict_js(my_data) + `,
  theirs: ` + baro_conflict_js(now_text) + `
};
function baro_conflict_pick(which){
  var area = document.getElementById('baro_conflict_area');
  if(!area) return;
  if(!confirm('편집창의 내용을 바꿉니다. 계속할까요?')) return;
  area.value = baro_conflict_data[which];
}
</script>`

	menu := [][]any{
		{"w/" + tool.Url_parser(doc_name), "문서로 돌아가기"},
		{"baro/draft", "내 초안"},
		{"baro/help", "도움말"},
	}

	return tool.Baro_page(db, config, doc_name+" (편집 충돌)", body, menu)
}

// baro_conflict_js : 본문을 자바스크립트 문자열 리터럴로
func baro_conflict_js(data string) string {
	return `"` + tool.JS_escape(data) + `"`
}

// baro_conflict_save_draft : 충돌 난 내 글을 초안으로 보존한다
func baro_conflict_save_draft(config tool.Config, doc_name string, my_data string, send string) string {
	if !tool.Baro_is_user(config.IP) {
		return ""
	}

	db := tool.DB_connect()
	defer tool.DB_close(db)

	title := doc_name + " (충돌 보존본)"

	// 같은 문서의 보존본이 이미 있으면 덮어쓴다
	old_id := ""
	if tool.QueryRow_DB(
		db,
		"select id from baro_workspace where user = ? and project = 'conflict' and title = ? limit 1",
		[]any{&old_id}, config.IP, title,
	) {
		tool.Exec_DB(
			db,
			"update baro_workspace set data = ?, note = ?, date = ? where id = ?",
			my_data, send, tool.Baro_now(), old_id,
		)

		return old_id
	}

	id := tool.Baro_next_id(db, "baro_workspace")
	tool.Exec_DB(
		db,
		"insert into baro_workspace (id, project, title, data, user, date, status, note) values (?, 'conflict', ?, ?, ?, ?, 'draft', ?)",
		id, title, my_data, config.IP, tool.Baro_now(), send,
	)

	return id
}
