package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

// ============================================================
//  13-1 가져오기 미리보기 · 선택적 가져오기 · 실행 · 오류 보고서
//  13-2 라이선스 · 출처 기록
// ============================================================

// baro_imp_not_found : 없는 작업 번호로 들어왔을 때 (500 대신 안내 화면)
func baro_imp_not_found(db *sql.DB, config tool.Config, id string) string {
	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/import", "자료 이전"}, {"", tool.Baro_t("찾을 수 없음")}})
	body += tool.Baro_alert("warn", tool.Baro_t("가져오기 작업 <b>#")+tool.HTML_escape(id)+tool.Baro_t("</b> 을(를) 찾지 못했습니다. 이미 지워졌거나 번호가 잘못됐습니다."))
	body += tool.Baro_card("이동", tool.Baro_button("/baro/import", tool.Baro_t("가져오기 작업 목록"), "primary")+" "+tool.Baro_button("/baro/import/new", tool.Baro_t("새 가져오기 만들기"), ""))

	return tool.Baro_page(db, config, tool.Baro_t("가져오기 작업 없음"), body, baro_imp_menu())
}

// baro_imp_job_head : 작업 요약 카드
func baro_imp_job_head(db *sql.DB, config tool.Config, job baro_imp_job) string {
	status_name, status_kind := baro_imp_status_name(job.Status)

	ready := baro_imp_item_count(db, job.ID, "ready")
	dup := baro_imp_item_count(db, job.ID, "duplicate")
	fail := baro_imp_item_count(db, job.ID, "error")
	done := baro_imp_item_count(db, job.ID, "imported")
	skip := baro_imp_item_count(db, job.ID, "skipped")

	pairs := [][]string{
		{"상태", tool.Baro_badge(status_name, status_kind)},
		{tool.Baro_t("원본 형식"), tool.HTML_escape(baro_imp_kind_name(job.Kind))},
		{tool.Baro_t("원본 출처"), tool.HTML_escape(tool.Baro_trunc(job.Source, 120))},
		{tool.Baro_t("올린 파일"), tool.HTML_escape(tool.Baro_trunc(job.File, 120))},
		{"라이선스", tool.HTML_escape(job.License)},
		{tool.Baro_t("원 작성자"), tool.HTML_escape(baro_imp_opt(job, "orig_author"))},
		{tool.Baro_t("만든 사람"), tool.Baro_user_link(db, job.Actor, config.IP)},
		{tool.Baro_t("만든 때"), tool.Baro_time_cell(job.Date)},
	}

	if confirmed := baro_imp_opt(job, "license_ok_by"); confirmed != "" {
		pairs = append(pairs, []string{
			tool.Baro_t("라이선스 확인"),
			tool.Baro_user_link(db, confirmed, config.IP) + " · " + tool.HTML_escape(baro_imp_opt(job, "license_ok_date")),
		})
	}

	if baro_imp_opt_on(job, "history_keep") {
		pairs = append(pairs, []string{tool.Baro_t("수정 이력 보존"), tool.Baro_badge(tool.Baro_t("보존함"), "ok")})
	} else {
		pairs = append(pairs, []string{tool.Baro_t("수정 이력 보존"), tool.Baro_badge(tool.Baro_t("보존하지 않음"), "muted")})
	}

	if baro_imp_opt_on(job, "bot") {
		pairs = append(pairs, []string{tool.Baro_t("봇 편집 표시"), tool.Baro_badge(tool.Baro_t("켜짐"), "info")})
	}

	if baro_imp_opt_on(job, "bot_hide") {
		pairs = append(pairs, []string{tool.Baro_t("최근 변경에서 숨김"), tool.Baro_badge(tool.Baro_t("켜짐"), "info")})
	}

	html := tool.Baro_kv_grid(pairs)
	html += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체"), tool.Baro_comma(tool.Str_to_int(job.Num)), tool.Baro_t("읽어들인 문서")},
		{tool.Baro_t("가져올 수 있음"), tool.Baro_comma(ready), tool.Baro_t("새 문서")},
		{tool.Baro_t("중복"), tool.Baro_comma(dup), tool.Baro_t("이미 있는 제목")},
		{tool.Baro_t("오류"), tool.Baro_comma(fail), tool.Baro_t("읽지 못한 항목")},
		{tool.Baro_t("가져옴"), tool.Baro_comma(done), tool.Baro_t("실제로 들어감")},
		{tool.Baro_t("건너뜀"), tool.Baro_comma(skip), tool.Baro_t("선택하지 않음")},
	})

	return tool.Baro_card_ex("가져오기 작업 #"+job.ID, "", html,
		tool.Baro_button("/baro/import/view/"+tool.Url_parser(job.ID), "미리보기", "")+" "+
			tool.Baro_button("/baro/import/run/"+tool.Url_parser(job.ID), "실행 화면", "")+" "+
			tool.Baro_button("/baro/import/license/"+tool.Url_parser(job.ID), "라이선스 기록", "")+" "+
			tool.Baro_button("/baro/import/report/"+tool.Url_parser(job.ID), "오류 보고서", "")+" "+
			tool.Baro_button("/baro/import/undo/"+tool.Url_parser(job.ID), "전체 취소", "danger"),
	)
}

// baro_imp_steps : 13-1 처리 단계 표
func baro_imp_steps(db *sql.DB, job baro_imp_job) string {
	total := tool.Str_to_int(job.Num)
	ready := baro_imp_item_count(db, job.ID, "ready")
	dup := baro_imp_item_count(db, job.ID, "duplicate")
	fail := baro_imp_item_count(db, job.ID, "error")
	done := baro_imp_item_count(db, job.ID, "imported")

	report := tool.Baro_json_dec_str(job.Report)

	rows := [][]string{
		{tool.Baro_t("1. 제목 추출"), tool.Baro_comma(total) + tool.Baro_t("건"), tool.Baro_t("파일에서 문서 제목을 뽑았습니다.")},
		{tool.Baro_t("2. 본문 변환"), tool.Baro_comma(total) + tool.Baro_t("건"), tool.Baro_t("줄바꿈 태그와 여분의 빈 줄을 정리했습니다.")},
		{tool.Baro_t("3. 분류 변환"), report["category"] + "", tool.Baro_t("Category / :분류 표기를 분류 로 통일했습니다.")},
		{tool.Baro_t("4. 링크 변환"), report["link"] + "", tool.Baro_t("대괄호 하나짜리 바깥 링크를 위키 링크로 바꿨습니다.")},
		{tool.Baro_t("5. 틀 변환"), report["template"] + "", tool.Baro_t("중괄호 틀 호출을 include 문법으로 바꿨습니다.")},
		{tool.Baro_t("6. 파일 연결"), report["file"] + "", tool.Baro_t("파일 문법을 통일하고, 아직 없는 파일을 비고에 적었습니다.")},
		{tool.Baro_t("7. 중복 문서 탐지"), tool.Baro_comma(dup) + tool.Baro_t("건"), tool.Baro_t("이 위키에 같은 제목이 있는 문서입니다.")},
		{tool.Baro_t("8. 가져오기 미리보기"), tool.Baro_comma(ready) + tool.Baro_t("건"), tool.Baro_t("바로 가져올 수 있는 문서입니다.")},
		{tool.Baro_t("9. 오류 보고서"), tool.Baro_comma(fail) + tool.Baro_t("건"), tool.Baro_t("읽지 못한 항목입니다.")},
		{tool.Baro_t("10. 선택적 가져오기"), tool.Baro_comma(done) + tool.Baro_t("건"), tool.Baro_t("고른 문서만 실제로 들어갔습니다.")},
	}

	table_rows := [][]string{}
	for _, row := range rows {
		value := row[1]
		if strings.TrimSpace(value) == "" {
			value = "-"
		}

		table_rows = append(table_rows, []string{
			tool.HTML_escape(row[0]),
			tool.HTML_escape(value),
			tool.HTML_escape(row[2]),
		})
	}

	return tool.Baro_table([]string{tool.Baro_t("처리 단계"), tool.Baro_t("건수"), tool.Baro_t("설명")}, table_rows, "")
}

// baro_imp_gate : 13-2 라이선스 호환 확인 관문 + 실행 선택지
//
//	★ 확인 체크를 하지 않으면 실행 단추가 눌리지 않는다.
func baro_imp_gate(db *sql.DB, job baro_imp_job, button string) string {
	compat, why := baro_imp_license_compat(job.License)

	html := ""
	if compat {
		html += tool.Baro_alert("ok", tool.Baro_t("적용 라이선스 <b>")+tool.HTML_escape(job.License)+"</b> · "+tool.HTML_escape(why))
	} else {
		html += tool.Baro_alert("danger", tool.Baro_t("적용 라이선스 <b>")+tool.HTML_escape(job.License)+"</b> · "+tool.HTML_escape(why))
	}

	html += tool.Baro_note(tool.Baro_t("바로위키 본문 라이선스 : ") + baro_imp_wiki_license(db))
	html += tool.Baro_note(tool.Baro_t("다른 위키 문서를 대량 복제하는 일은 라이선스 호환을 사람이 확인하기 전에는 실행할 수 없습니다. 확인한 사람과 시각을 기록으로 남깁니다."))

	html += `<div class="bw_f"><label class="bw_cb"><input type="checkbox" id="bw_imp_ok" name="license_ok" value="on"> ` +
		tool.Baro_t(`<span>원본 라이선스가 이 위키 본문 라이선스와 호환되는지 직접 확인했습니다.</span></label></div>`)

	if !compat {
		html += `<div class="bw_f"><label class="bw_cb"><input type="checkbox" id="bw_imp_force" name="license_force" value="on"> ` +
			tool.Baro_t(`<span>자동 판단으로는 호환되지 않지만, 저작자 허락 등 별도의 근거가 있어 진행합니다.</span></label></div>`)
		html += tool.Baro_input("license_reason", tool.Baro_t("별도 근거"), "", tool.Baro_t("예: 원작자에게 서면 허락을 받음"), tool.Baro_t("호환되지 않는 라이선스를 진행할 때는 근거를 남겨야 합니다."))
	}

	html += tool.Baro_checkbox("overwrite", tool.Baro_t("이미 있는 문서를 덮어쓴다"), "on", false,
		tool.Baro_t("기본은 꺼짐입니다. 꺼져 있으면 같은 제목의 문서는 건드리지 않고 건너뜁니다."))
	html += tool.Baro_checkbox("bot", tool.Baro_t("봇 편집으로 표시한다"), "on", baro_imp_opt_on(job, "bot"),
		tool.Baro_t("편집 요약 끝에 (봇) 이 붙습니다."))
	html += tool.Baro_checkbox("bot_hide", tool.Baro_t("최근 변경에서 숨김으로 기록한다"), "on", baro_imp_opt_on(job, "bot_hide"),
		tool.Baro_t("봇 편집을 최근 변경 목록에서 걸러 낼 수 있도록 표시만 남깁니다."))

	html += `<button type="submit" id="bw_imp_go" class="bw_btn bw_btn_primary" disabled>` + tool.HTML_escape(button) + `</button>`
	html += `<script>
(function(){
var ok=document.getElementById('bw_imp_ok');
var force=document.getElementById('bw_imp_force');
var go=document.getElementById('bw_imp_go');
if(!ok||!go){return;}
function sync(){go.disabled=!(ok.checked&&(!force||force.checked));}
ok.addEventListener('change',sync);
if(force){force.addEventListener('change',sync);}
sync();
})();
</script>`

	return html
}

// ------------------------------------------------------------
//  /baro/import/view/<id> : 가져오기 작업 상세 + 미리보기
// ------------------------------------------------------------

// View_baro_imp_view : 가져오기 작업 상세와 미리보기 표
func View_baro_imp_view(config tool.Config, id string, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/import"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "import") {
		return tool.Baro_deny(db, config, tool.Baro_t("가져오기 기능은 시스템 관리자 역할이 있어야 씁니다."))
	}

	job, found := baro_imp_job_get(db, id)
	if !found {
		return baro_imp_not_found(db, config, id)
	}

	page_int := tool.Str_to_int(page)
	if page_int < 1 {
		page_int = 1
	}

	max_count := tool.Baro_page_size
	items := baro_imp_item_list(db, job.ID, "", (page_int-1)*max_count, max_count)

	rows := [][]string{}
	shown := []string{}

	for _, item := range items {
		name, kind := baro_imp_item_status_name(item.Status)
		meta := baro_imp_body_meta(db, item.ID)

		pick := `<span class="bw_muted">-</span>`

		// 건너뛴 항목도 다시 고를 수 있어야 한다. (한 번 빠졌다고 영영 못 넣으면 안 된다)
		if item.Status == "ready" || item.Status == "duplicate" || item.Status == "skipped" {
			shown = append(shown, item.ID)

			checked := ""
			if item.Status == "ready" {
				checked = " checked"
			}

			pick = `<input type="checkbox" name="pick" value="` + tool.HTML_escape(item.ID) + `"` + checked + `>`
		}

		dup := tool.Baro_badge(tool.Baro_t("새 문서"), "ok")
		switch {
		case strings.TrimSpace(item.Title) == "":
			dup = `<span class="bw_muted">-</span>`
		case baro_imp_doc_exist(db, item.Title):
			dup = `<a href="/w/` + tool.Doc_url(item.Title) + tool.Baro_t(`">이미 있음</a>`)
		}

		length := meta["length"]
		if length == "" {
			length = "0"
		}

		rows = append(rows, []string{
			pick,
			tool.Baro_doc_link(item.Title),
			tool.Baro_comma(tool.Str_to_int(length)) + tool.Baro_t("자"),
			tool.Baro_badge(name, kind),
			dup,
			tool.HTML_escape(tool.Baro_trunc(item.Note, 90)),
		})
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/import", "자료 이전"}, {"", "가져오기 #" + job.ID}})
	body += baro_imp_tabs("/baro/import")
	body += baro_imp_job_head(db, config, job)

	body += tool.Baro_card(tool.Baro_t("처리 단계"), baro_imp_steps(db, job))

	if tool.Baro_read_only(db) {
		body += tool.Baro_alert("warn", tool.Baro_t("읽기 전용 상태라 가져오기를 실행할 수 없습니다. (")+tool.HTML_escape(tool.Baro_read_only_reason(db))+")")
	}

	table := tool.Baro_table(
		[]string{tool.Baro_t("고르기"), tool.Baro_t("제목"), "길이", "상태", tool.Baro_t("중복 여부"), tool.Baro_t("비고")},
		rows,
		tool.Baro_t("이 작업에는 항목이 없습니다."),
	)

	if len(rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("이 작업에는 항목이 없습니다."))
	}

	form := tool.Baro_note(tool.Baro_t("체크한 문서만 실제로 들어갑니다. 중복 문서와 앞서 건너뛴 문서는 기본으로 체크가 꺼져 있습니다. ") +
		tool.Baro_t("체크는 지금 보고 있는 쪽에만 적용되고, 다른 쪽의 항목은 이 실행에서 건드리지 않습니다."))
	form += tool.Baro_hidden("shown", strings.Join(shown, ","))
	form += table
	form += tool.Baro_paging(page_int, len(items), max_count, "/baro/import/view/"+tool.Url_parser(job.ID)+"/{}")
	form += baro_imp_gate(db, job, tool.Baro_t("고른 문서만 가져오기"))

	body += tool.Baro_card("가져오기 미리보기", tool.Baro_form("/baro/import/run/"+tool.Url_parser(job.ID), form))

	body += tool.Baro_card(tool.Baro_t("한 번에 실행하기"),
		tool.Baro_note(tool.Baro_t("쪽을 넘기지 않고 \"가져올 수 있음\" 상태의 문서 전체를 한 번에 넣으려면 실행 화면을 쓰세요."))+
			tool.Baro_button("/baro/import/run/"+tool.Url_parser(job.ID), "실행 화면으로", "")+" "+
			tool.Baro_button("/baro/import/bot", tool.Baro_t("사전 검토(10건)로 먼저 시험"), ""),
	)

	return tool.Baro_page(db, config, "가져오기 #"+job.ID, body, baro_imp_menu())
}

// ------------------------------------------------------------
//  /baro/import/run/<id> : 실제로 가져오기
// ------------------------------------------------------------

// View_baro_imp_run : 가져오기 실행 (GET 이면 확인 화면, POST 면 실행)
func View_baro_imp_run(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/import"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "import") {
		return tool.Baro_deny(db, config, tool.Baro_t("가져오기 기능은 시스템 관리자 역할이 있어야 씁니다."))
	}

	job, found := baro_imp_job_get(db, id)
	if !found {
		return baro_imp_not_found(db, config, id)
	}

	notice := ""

	if values != nil {
		notice = baro_imp_run_post(db, config, job, values, 0)
		if strings.HasPrefix(notice, "/baro/") {
			return tool.Get_redirect(notice)
		}

		job, _ = baro_imp_job_get(db, id)
	}

	ready := baro_imp_item_count(db, job.ID, "ready")
	dup := baro_imp_item_count(db, job.ID, "duplicate")

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/import", "자료 이전"}, {"/baro/import/view/" + tool.Url_parser(job.ID), "가져오기 #" + job.ID}, {"", "실행"}})
	body += baro_imp_tabs("/baro/import")

	if notice != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(notice))
	}

	body += baro_imp_job_head(db, config, job)

	if tool.Baro_read_only(db) {
		body += tool.Baro_alert("warn", tool.Baro_t("읽기 전용 상태입니다. (")+tool.HTML_escape(tool.Baro_read_only_reason(db))+tool.Baro_t(") 지금은 실행할 수 없습니다."))
	}

	form := tool.Baro_alert("info",
		tool.Baro_t("지금 실행하면 <b>")+tool.Baro_comma(ready)+tool.Baro_t("</b>개의 새 문서가 들어갑니다. ")+
			tool.Baro_t("중복 문서 <b>")+tool.Baro_comma(dup)+tool.Baro_t("</b>개는 덮어쓰기를 켜지 않는 한 건드리지 않습니다."))
	form += tool.Baro_note(tool.Baro_t("편집 요약에는 \"가져오기\" 가 적히고, 문서마다 기여 기록(문서 이전)이 남습니다."))
	form += baro_imp_gate(db, job, "가져오기 실행")

	body += tool.Baro_card("가져오기 실행", tool.Baro_form("/baro/import/run/"+tool.Url_parser(job.ID), form))

	body += tool.Baro_card(tool.Baro_t("실행하면 남는 기록"), tool.Baro_list([]string{
		tool.Baro_t("문서 역사 : 편집 요약 \"가져오기\" (봇 표시를 켜면 (봇) 이 붙습니다)"),
		tool.Baro_t("기여 기록 : 문서마다 문서 이전(import) 이벤트"),
		tool.Baro_t("문서 부가 정보 : 원 작성자 · 원본 출처 · 라이선스 · 가져온 날짜 · 기존 기여자 · 수정 이력 보존 여부"),
		tool.Baro_t("공개 운영 로그와 내부 감사 기록"),
		tool.Baro_t("되돌리기용 스냅샷 (전체 취소에서 씁니다)"),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("가져오기 실행 #")+job.ID, body, baro_imp_menu())
}

// baro_imp_run_post : 실행 요청 처리 (성공하면 옮겨 갈 주소, 실패하면 오류 문장)
func baro_imp_run_post(db *sql.DB, config tool.Config, job baro_imp_job, values url.Values, limit int) string {
	if tool.Baro_read_only(db) {
		return tool.Baro_t("읽기 전용 상태라 실행할 수 없습니다.")
	}

	if values.Get("license_ok") != "on" {
		return tool.Baro_t("라이선스 호환 확인란에 체크해야 실행할 수 있습니다.")
	}

	compat, _ := baro_imp_license_compat(job.License)
	reason := strings.TrimSpace(values.Get("license_reason"))

	if !compat {
		if values.Get("license_force") != "on" {
			return tool.Baro_t("이 라이선스는 자동 판단으로 호환되지 않습니다. 별도 근거 확인란에도 체크하세요.")
		}

		if reason == "" {
			return tool.Baro_t("호환되지 않는 라이선스를 진행하려면 별도 근거를 적어야 합니다.")
		}
	}

	overwrite := values.Get("overwrite") == "on"
	bot := values.Get("bot") == "on"
	bot_hide := values.Get("bot_hide") == "on"

	picks := map[string]bool{}
	for _, one := range values["pick"] {
		if strings.TrimSpace(one) != "" {
			picks[one] = true
		}
	}

	// 미리보기에서 실제로 보여 준 항목만 "건너뜀" 으로 바꾼다.
	// (쪽을 넘겨 화면에 없던 항목까지 건너뜀으로 바꾸면 다시 가져올 수 없게 된다)
	shown := map[string]bool{}
	for _, one := range strings.Split(values.Get("shown"), ",") {
		one = strings.TrimSpace(one)
		if one != "" {
			shown[one] = true
		}
	}

	// 라이선스 호환을 확인한 사람과 시각을 남긴다. (13-2)
	baro_imp_opt_set(db, job, map[string]string{
		"license_ok_by":   config.IP,
		"license_ok_date": tool.Baro_now(),
		"license_reason":  reason,
		"overwrite":       map[bool]string{true: "on", false: ""}[overwrite],
		"bot":             map[bool]string{true: "on", false: ""}[bot],
		"bot_hide":        map[bool]string{true: "on", false: ""}[bot_hide],
	})

	job, _ = baro_imp_job_get(db, job.ID)

	done, failed, stopped := baro_imp_do_run(db, config, job, picks, shown, overwrite, bot, limit)

	if done == 0 && failed == 0 {
		if stopped != "" {
			return stopped
		}

		return tool.Baro_t("가져올 문서를 고르지 않았습니다.")
	}

	return "/baro/import/view/" + tool.Url_parser(job.ID)
}

// baro_imp_render_safe : 문서 렌더링 (한 문서가 실패해도 전체가 멈추지 않게 한다)
func baro_imp_render_safe(db *sql.DB, title string, text string) {
	defer func() {
		_ = recover()
	}()

	markup.Get_render(db, title, text, "backlink")
}

// baro_imp_do_run : 실제 가져오기 (가져온 수, 실패한 수, 멈춘 사유)
//
//	picks 가 비어 있으면 "가져올 수 있음" · "중복" 상태 전부가 대상이다.
//	shown 은 미리보기에서 실제로 보여 준 항목이다. 여기에 없는 항목은 이 실행에서 건드리지 않는다.
//	limit 이 0 보다 크면 그 수만큼만 처리한다. (사전 검토 · 테스트 가져오기)
func baro_imp_do_run(
	db *sql.DB,
	config tool.Config,
	job baro_imp_job,
	picks map[string]bool,
	shown map[string]bool,
	overwrite bool,
	bot bool,
	limit int,
) (int, int, string) {
	rate := tool.Baro_get_int(db, "imp_bot_rate", 120)
	if rate < 1 {
		rate = 1
	}

	// 봇 계정으로 지정된 사람이 실행하면 자동으로 봇 편집으로 표시한다. (13-4)
	if !bot && baro_imp_is_bot(db, config.IP) {
		bot = true
	}

	send := "가져오기"
	if bot {
		send = "가져오기 (봇)"
	}

	license := job.License
	contributors := baro_imp_opt(job, "contributors")
	history_keep := ""
	if baro_imp_opt_on(job, "history_keep") {
		history_keep = "on"
	}

	items := baro_imp_item_list(db, job.ID, "", 0, 20000)

	next_snap := tool.Str_to_int(tool.Baro_next_id(db, "baro_snapshot"))

	done, failed := 0, 0
	stopped := ""

	for _, item := range items {
		if limit > 0 && done >= limit {
			break
		}

		// 건너뛴 항목은 사람이 다시 고른 경우에만 대상이 된다.
		eligible := item.Status == "ready" || item.Status == "duplicate"
		if !eligible && item.Status == "skipped" && picks[item.ID] {
			eligible = true
		}

		if !eligible {
			continue
		}

		if len(picks) > 0 && !picks[item.ID] {
			// 화면에 보여 준 항목만 건너뜀으로 표시한다. 나머지는 그대로 둔다.
			if shown[item.ID] && item.Status != "skipped" {
				tool.Exec_DB(db, "update baro_import_item set status = 'skipped', note = ? where id = ?",
					"선택하지 않아 건너뜀", item.ID)
			}

			continue
		}

		// 13-4 속도 제한 : 분당 처리 건수를 넘으면 멈춘다.
		if !tool.Baro_rate_ok(db, "baro_imp_run", config.IP, rate, 60) {
			stopped = "분당 처리 건수(" + tool.Baro_itoa(rate) + "건)를 넘어 중간에 멈췄습니다. 잠시 뒤 다시 실행하면 이어서 처리합니다."

			break
		}

		title := item.Title
		text := baro_imp_body_get(db, item.ID)

		if strings.TrimSpace(title) == "" || strings.TrimSpace(text) == "" {
			tool.Exec_DB(db, "update baro_import_item set status = 'error', note = ? where id = ?",
				"본문을 찾지 못했습니다.", item.ID)
			failed++

			continue
		}

		exist := baro_imp_doc_exist(db, title)
		if exist && !overwrite {
			tool.Exec_DB(db, "update baro_import_item set status = 'duplicate', note = ? where id = ?",
				"이미 있는 문서라 건너뛰었습니다. (덮어쓰기 꺼짐)", item.ID)

			continue
		}

		old_text := ""
		if exist {
			old_text = baro_imp_doc_text(db, title)
		}

		// 되돌리기용 스냅샷을 먼저 남긴다. (0장 원칙 3 · 13-4 전체 취소)
		meta := map[string]string{
			"import_id": job.ID,
			"item_id":   item.ID,
			"exist":     map[bool]string{true: "on", false: ""}[exist],
		}

		tool.Exec_DB(db, "delete from baro_snapshot where mass_id = ? and kind = 'import_undo'", baro_imp_undo_key(item.ID))
		tool.Exec_DB(
			db,
			"insert into baro_snapshot (id, mass_id, kind, title, data, meta, date) values (?, ?, 'import_undo', ?, ?, ?, ?)",
			tool.Baro_itoa(next_snap), baro_imp_undo_key(item.ID), title, old_text, tool.Baro_json_enc(meta), tool.Baro_now(),
		)
		next_snap++

		length := "+" + tool.Baro_itoa(tool.Get_len(text))
		if exist {
			length = tool.Get_edit_length_diff(old_text, text)
		}

		tool.Exec_DB(db, "delete from data where title = ?", title)
		tool.Exec_DB(db, "insert into data (title, data) values (?, ?)", title, text)

		tool.Do_add_history(db, title, text, tool.Get_time(), config.IP, send, length, "", "")
		baro_imp_render_safe(db, title, text)

		rev := tool.Get_document_revision(db, title)

		// 13-2 문서마다 라이선스 · 출처 기록
		item_license := item.License
		if strings.TrimSpace(item_license) == "" {
			item_license = license
		}

		tool.Baro_doc_meta_set(db, title, "import_job", job.ID)
		tool.Baro_doc_meta_set(db, title, "import_license", item_license)
		tool.Baro_doc_meta_set(db, title, "import_source", item.Url)
		tool.Baro_doc_meta_set(db, title, "import_author", item.Author)
		tool.Baro_doc_meta_set(db, title, "import_date", tool.Baro_now())
		tool.Baro_doc_meta_set(db, title, "import_contributors", contributors)
		tool.Baro_doc_meta_set(db, title, "import_history_keep", history_keep)

		if bot {
			tool.Baro_doc_meta_set(db, title, "import_bot", "on")
		}

		if baro_imp_opt_on(job, "bot_hide") {
			tool.Baro_doc_meta_set(db, title, "import_hide_recent", "on")
		}

		// 0장 원칙 4 : 횟수가 아니라 행동 종류로 기록한다.
		tool.Baro_contrib_add(db, config.IP, "import", title, rev, "", "가져오기 작업 #"+job.ID)

		tool.Exec_DB(db, "update baro_import_item set status = 'imported', note = ? where id = ?",
			"가져옴 · 판 "+rev, item.ID)

		tool.Baro_metric_bump(db, "import_done", tool.Baro_today(), 1)
		done++
	}

	// 작업 상태 정리 : 아직 넣을 수 있는 항목이 남아 있으면 "일부만 가져옴" 이다.
	left := baro_imp_item_count(db, job.ID, "ready") + baro_imp_item_count(db, job.ID, "duplicate")
	status := "partial"
	if left == 0 {
		status = "done"
	}

	if done == 0 && failed == 0 && stopped == "" {
		status = job.Status
	}

	report := tool.Baro_json_dec_str(job.Report)
	if report == nil {
		report = map[string]string{}
	}

	report["run_date"] = tool.Baro_now()
	report["run_done"] = tool.Baro_itoa(done)
	report["run_failed"] = tool.Baro_itoa(failed)
	report["run_stopped"] = stopped
	report["run_actor"] = config.IP

	tool.Exec_DB(db, "update baro_import set status = ?, report = ? where id = ?", status, tool.Baro_json_enc(report), job.ID)

	if done > 0 || failed > 0 {
		tool.Baro_public_log(db, "import_run", config.IP, "가져오기 #"+job.ID,
			"라이선스 "+license, "가져온 문서 "+tool.Baro_itoa(done)+"건 / 실패 "+tool.Baro_itoa(failed)+"건")
		tool.Baro_audit(db, config, "import_run", "baro_import#"+job.ID, "",
			"done="+tool.Baro_itoa(done)+" failed="+tool.Baro_itoa(failed)+" overwrite="+map[bool]string{true: "on", false: ""}[overwrite],
			"라이선스 확인 후 실행")

		tool.Baro_notify(db, job.Actor, "system",
			"가져오기 #"+job.ID+" 에서 문서 "+tool.Baro_itoa(done)+"건을 가져왔습니다.",
			"/baro/import/view/"+tool.Url_parser(job.ID), "")
	}

	return done, failed, stopped
}

// ------------------------------------------------------------
//  /baro/import/report/<id> : 오류 보고서
// ------------------------------------------------------------

// View_baro_imp_report : 실패한 항목과 이유
func View_baro_imp_report(config tool.Config, id string, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/import"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "import") {
		return tool.Baro_deny(db, config, tool.Baro_t("가져오기 기능은 시스템 관리자 역할이 있어야 씁니다."))
	}

	job, found := baro_imp_job_get(db, id)
	if !found {
		return baro_imp_not_found(db, config, id)
	}

	page_int := tool.Str_to_int(page)
	if page_int < 1 {
		page_int = 1
	}

	max_count := tool.Baro_page_size
	items := baro_imp_item_list(db, job.ID, "error", (page_int-1)*max_count, max_count)

	rows := [][]string{}
	for _, item := range items {
		title := item.Title
		if strings.TrimSpace(title) == "" {
			title = tool.Baro_t("(제목 없음)")
		}

		rows = append(rows, []string{
			tool.HTML_escape("#" + item.ID),
			tool.HTML_escape(tool.Baro_trunc(title, 80)),
			tool.HTML_escape(item.Note),
			tool.Baro_time_cell(item.Date),
		})
	}

	report := tool.Baro_json_dec_str(job.Report)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/import", "자료 이전"}, {"/baro/import/view/" + tool.Url_parser(job.ID), "가져오기 #" + job.ID}, {"", "오류 보고서"}})
	body += baro_imp_tabs("/baro/import")
	body += baro_imp_job_head(db, config, job)

	if note := report["file_note"]; strings.TrimSpace(note) != "" {
		body += tool.Baro_card(tool.Baro_t("파일 단위 오류"), tool.Baro_code(note))
	}

	table := tool.Baro_table([]string{tool.Baro_t("항목"), tool.Baro_t("제목"), tool.Baro_t("이유"), tool.Baro_t("등록 때")}, rows, tool.Baro_t("오류로 기록된 항목이 없습니다."))
	if len(rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("오류로 기록된 항목이 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("오류 항목"),
		table+tool.Baro_paging(page_int, len(items), max_count, "/baro/import/report/"+tool.Url_parser(job.ID)+"/{}"))

	if stopped := report["run_stopped"]; strings.TrimSpace(stopped) != "" {
		body += tool.Baro_alert("warn", tool.HTML_escape(stopped))
	}

	body += tool.Baro_card(tool.Baro_t("자주 나오는 이유"), tool.Baro_list([]string{
		tool.Baro_t("본문이 비어 있습니다 — 제목만 있고 내용이 없는 항목입니다."),
		tool.Baro_t("문서 제목을 읽지 못했습니다 — 제목 자리에 빈 값이나 줄바꿈만 있었습니다."),
		tool.Baro_t("같은 파일 안에 제목이 겹치는 문서가 있습니다 — 뒤에 온 항목이 오류로 기록됩니다."),
		tool.Baro_t("본문을 찾지 못했습니다 — 미리보기 뒤에 보관해 둔 본문이 지워진 경우입니다."),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("오류 보고서 #")+job.ID, body, baro_imp_menu())
}

// ------------------------------------------------------------
//  13-2 /baro/import/license/<id> : 라이선스 · 출처 기록
// ------------------------------------------------------------

// View_baro_imp_license : 가져온 문서들의 라이선스 정보 표 (공개 화면)
func View_baro_imp_license(config tool.Config, id string, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	job, found := baro_imp_job_get(db, id)
	if !found {
		return baro_imp_not_found(db, config, id)
	}

	page_int := tool.Str_to_int(page)
	if page_int < 1 {
		page_int = 1
	}

	max_count := tool.Baro_page_size
	items := baro_imp_item_list(db, job.ID, "", (page_int-1)*max_count, max_count)

	rows := [][]string{}
	for _, item := range items {
		author := strings.TrimSpace(item.Author)
		if author == "" {
			author = tool.Baro_t("(적히지 않음)")
		}

		source := strings.TrimSpace(item.Url)
		source_html := tool.Baro_t(`<span class="bw_muted">(적히지 않음)</span>`)
		if source != "" {
			if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
				source_html = `<a href="` + tool.HTML_escape(source) + `" rel="nofollow noopener">` + tool.HTML_escape(tool.Baro_trunc(source, 60)) + `</a>`
			} else {
				source_html = tool.HTML_escape(tool.Baro_trunc(source, 60))
			}
		}

		license_text := item.License
		if strings.TrimSpace(license_text) == "" {
			license_text = job.License
		}

		keep := tool.Baro_badge(tool.Baro_t("보존하지 않음"), "muted")
		if baro_imp_opt_on(job, "history_keep") {
			keep = tool.Baro_badge(tool.Baro_t("보존함"), "ok")
		}

		// 13-2 "가져온 날짜" 는 문서에 남긴 기록을 우선한다. (항목 등록 시각이 아니다)
		imported := `<span class="bw_muted">-</span>`
		if item.Status == "imported" {
			when := tool.Baro_doc_meta_get(db, item.Title, "import_date")
			if strings.TrimSpace(when) == "" {
				when = item.Date
			}

			imported = tool.Baro_time_cell(when)
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(item.Title),
			tool.HTML_escape(author),
			source_html,
			tool.HTML_escape(license_text),
			imported,
			keep,
		})
	}

	contributors := baro_imp_opt(job, "contributors")
	compat, why := baro_imp_license_compat(job.License)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/import", "자료 이전"}, {"", tool.Baro_t("라이선스 기록 #") + job.ID}})

	body += tool.Baro_hero(
		tool.Baro_t("가져온 자료의 라이선스와 출처"),
		"가져오기 작업 #"+job.ID+tool.Baro_t(" 에 딸린 문서들의 저작권 정보입니다."),
		"",
	)

	if compat {
		body += tool.Baro_alert("ok", tool.Baro_t("적용 라이선스 <b>")+tool.HTML_escape(job.License)+"</b> · "+tool.HTML_escape(why))
	} else {
		body += tool.Baro_alert("warn", tool.Baro_t("적용 라이선스 <b>")+tool.HTML_escape(job.License)+"</b> · "+tool.HTML_escape(why))
	}

	pairs := [][]string{
		{tool.Baro_t("원본 출처"), tool.HTML_escape(tool.Baro_trunc(job.Source, 200))},
		{tool.Baro_t("적용 라이선스"), tool.HTML_escape(job.License)},
		{tool.Baro_t("바로위키 본문 라이선스"), tool.HTML_escape(baro_imp_wiki_license(db))},
		{tool.Baro_t("가져온 날짜"), tool.HTML_escape(tool.Baro_date_only(job.Date))},
		{tool.Baro_t("원 작성자(작업 기본값)"), tool.HTML_escape(baro_imp_opt(job, "orig_author"))},
	}

	if by := baro_imp_opt(job, "license_ok_by"); by != "" {
		pairs = append(pairs, []string{tool.Baro_t("라이선스 호환 확인"), tool.Baro_user_link(db, by, config.IP) + " · " + tool.HTML_escape(baro_imp_opt(job, "license_ok_date"))})
	}

	if reason := baro_imp_opt(job, "license_reason"); strings.TrimSpace(reason) != "" {
		pairs = append(pairs, []string{tool.Baro_t("별도 근거"), tool.HTML_escape(reason)})
	}

	body += tool.Baro_card(tool.Baro_t("작업 라이선스 정보"), tool.Baro_kv_grid(pairs))

	if strings.TrimSpace(contributors) != "" {
		body += tool.Baro_card(tool.Baro_t("기존 기여자 목록"), tool.Baro_code(contributors))
	}

	table := tool.Baro_table(
		[]string{"문서", tool.Baro_t("원 작성자"), tool.Baro_t("원본 출처"), "라이선스", tool.Baro_t("가져온 때"), tool.Baro_t("수정 이력 보존")},
		rows,
		tool.Baro_t("기록된 문서가 없습니다."),
	)

	if len(rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("기록된 문서가 없습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("문서별 기록"),
		table+tool.Baro_paging(page_int, len(items), max_count, "/baro/import/license/"+tool.Url_parser(job.ID)+"/{}"))

	return tool.Baro_page(db, config, tool.Baro_t("라이선스 기록 #")+job.ID, body, baro_imp_menu())
}
