package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  13-4 대량 가져오기 봇 — /baro/import/bot
//
//  봇 계정 지정 · 속도 제한 · 사전 검토(테스트 가져오기) · 전체 취소 ·
//  봇 편집 표시 / 최근 변경 숨기기.
// ============================================================

// baro_imp_bot_list : 봇으로 지정된 계정 목록
func baro_imp_bot_list(db *sql.DB) []string {
	out := []string{}
	seen := map[string]bool{}

	func() {
		rows := tool.Query_DB(db, "select user from baro_user_set where name = 'is_bot' and data = 'on' order by user asc")
		defer rows.Close()

		for rows.Next() {
			user := ""
			if err := rows.Scan(&user); err != nil {
				continue
			}

			if user == "" || seen[user] {
				continue
			}

			seen[user] = true
			out = append(out, user)
		}
	}()

	// baro_set 의 bot_users 목록도 함께 본다.
	for _, one := range tool.Baro_json_dec_list(tool.Baro_get(db, "bot_users")) {
		one = strings.TrimSpace(one)
		if one == "" || seen[one] {
			continue
		}

		seen[one] = true
		out = append(out, one)
	}

	return out
}

// baro_imp_bot_sync : baro_set 의 bot_users 목록을 다시 맞춘다
func baro_imp_bot_sync(db *sql.DB) {
	tool.Baro_put(db, "bot_users", tool.Baro_json_enc(baro_imp_bot_list(db)))
}

// baro_imp_is_bot : 이 계정이 봇으로 지정돼 있는지
func baro_imp_is_bot(db *sql.DB, user string) bool {
	if strings.TrimSpace(user) == "" {
		return false
	}

	data := ""
	tool.QueryRow_DB(db, "select data from baro_user_set where user = ? and name = 'is_bot'", []any{&data}, user)

	if data == "on" {
		return true
	}

	for _, one := range tool.Baro_json_dec_list(tool.Baro_get(db, "bot_users")) {
		if strings.TrimSpace(one) == user {
			return true
		}
	}

	return false
}

// ------------------------------------------------------------
//  /baro/import/bot : 봇 설정 화면
// ------------------------------------------------------------

// View_baro_imp_bot : 대량 가져오기 봇 설정
func View_baro_imp_bot(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/import/bot"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "import") {
		return tool.Baro_deny(db, config, tool.Baro_t("대량 가져오기 봇 설정은 시스템 관리자 역할이 있어야 씁니다."))
	}

	notice := ""
	notice_kind := "ok"

	if values != nil {
		switch values.Get("do") {
		case "bot_add":
			user := strings.TrimSpace(values.Get("user"))
			switch {
			case user == "":
				notice, notice_kind = tool.Baro_t("봇으로 지정할 계정 이름을 적으세요."), "warn"
			case !tool.Baro_is_user(user):
				notice, notice_kind = tool.Baro_t("계정 이름이 아닙니다. IP 는 봇으로 지정할 수 없습니다."), "warn"
			default:
				tool.Exec_DB(db, "delete from baro_user_set where user = ? and name = 'is_bot'", user)
				tool.Exec_DB(db, "insert into baro_user_set (user, name, data) values (?, 'is_bot', 'on')", user)
				baro_imp_bot_sync(db)

				tool.Baro_public_log(db, "bot_grant", config.IP, user, "", "대량 가져오기 봇 계정 지정")
				tool.Baro_audit(db, config, "bot_grant", user, "", "is_bot=on", "대량 가져오기")
				tool.Baro_notify(db, user, "system", "계정이 대량 가져오기 봇으로 지정됐습니다.", "/baro/import/bot", "")

				notice = user + tool.Baro_t(" 계정을 봇으로 지정했습니다.")
			}
		case "bot_del":
			user := strings.TrimSpace(values.Get("user"))
			if user != "" {
				tool.Exec_DB(db, "delete from baro_user_set where user = ? and name = 'is_bot'", user)

				left := []string{}
				for _, one := range tool.Baro_json_dec_list(tool.Baro_get(db, "bot_users")) {
					if strings.TrimSpace(one) != user {
						left = append(left, one)
					}
				}
				tool.Baro_put(db, "bot_users", tool.Baro_json_enc(left))
				baro_imp_bot_sync(db)

				tool.Baro_public_log(db, "bot_revoke", config.IP, user, "", "대량 가져오기 봇 지정 해제")
				tool.Baro_audit(db, config, "bot_revoke", user, "is_bot=on", "", "대량 가져오기")

				notice = user + tool.Baro_t(" 계정의 봇 지정을 풀었습니다.")
			}
		case "setting":
			rate := tool.Str_to_int(values.Get("rate"))
			if rate < 1 {
				rate = 1
			}

			if rate > 3000 {
				rate = 3000
			}

			tool.Baro_put(db, "imp_bot_rate", tool.Baro_itoa(rate))
			tool.Baro_put(db, "imp_bot_mark", map[bool]string{true: "on", false: ""}[values.Get("mark") == "on"])
			tool.Baro_put(db, "imp_bot_hide", map[bool]string{true: "on", false: ""}[values.Get("hide") == "on"])

			tool.Baro_audit(db, config, "bot_setting", "baro_set", "", "rate="+tool.Baro_itoa(rate), "대량 가져오기 봇")

			notice = tool.Baro_t("봇 설정을 저장했습니다.")
		default:
			notice, notice_kind = tool.Baro_t("알 수 없는 요청입니다."), "warn"
		}

		if notice_kind == "ok" {
			return tool.Get_redirect("/baro/import/bot")
		}
	}

	bots := baro_imp_bot_list(db)
	rate := tool.Baro_get_int(db, "imp_bot_rate", 120)
	mark := tool.Baro_get_on(db, "imp_bot_mark")
	hide := tool.Baro_get_on(db, "imp_bot_hide")

	bot_rows := [][]string{}
	for _, user := range bots {
		bot_rows = append(bot_rows, []string{
			tool.Baro_user_link(db, user, config.IP),
			tool.Baro_badge("봇", "info"),
			tool.Baro_form("/baro/import/bot",
				tool.Baro_hidden("do", "bot_del")+tool.Baro_hidden("user", user)+tool.Baro_submit(tool.Baro_t("지정 해제"), "danger")),
		})
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/import", "자료 이전"}, {"", "대량 가져오기 봇"}})
	body += baro_imp_tabs("/baro/import/bot")

	if notice != "" {
		body += tool.Baro_alert(notice_kind, tool.HTML_escape(notice))
	}

	body += tool.Baro_alert("info",
		tool.Baro_t("대량 가져오기는 한 번에 많은 문서를 만듭니다. 먼저 <b>사전 검토(10건)</b>로 결과를 확인한 뒤 전체를 실행하세요. ")+
			tool.Baro_t("잘못됐으면 <b>전체 취소</b>로 한 번에 되돌릴 수 있습니다."))

	bot_table := tool.Baro_table([]string{"계정", tool.Baro_t("표시"), "관리"}, bot_rows, tool.Baro_t("지정된 봇 계정이 없습니다."))
	if len(bot_rows) == 0 {
		bot_table = tool.Baro_empty(tool.Baro_t("지정된 봇 계정이 없습니다."))
	}

	add_form := tool.Baro_hidden("do", "bot_add")
	add_form += tool.Baro_input("user", tool.Baro_t("봇으로 지정할 계정"), "", tool.Baro_t("계정 이름"), tool.Baro_t("IP 는 지정할 수 없습니다. 로그인 계정만 봇이 될 수 있습니다."))
	add_form += tool.Baro_submit(tool.Baro_t("봇으로 지정"), "primary")

	body += tool.Baro_card(tool.Baro_t("봇 계정"), bot_table+tool.Baro_form("/baro/import/bot", add_form))

	set_form := tool.Baro_hidden("do", "setting")
	set_form += tool.Baro_input_type("rate", tool.Baro_t("분당 처리 건수"), tool.Baro_itoa(rate), "number",
		tool.Baro_t("가져오기 실행이 1분 동안 처리할 수 있는 문서 수입니다. 넘으면 중간에 멈추고, 다시 실행하면 이어서 처리합니다."))
	set_form += tool.Baro_checkbox("mark", tool.Baro_t("새 가져오기의 봇 편집 표시를 기본으로 켠다"), "on", mark,
		tool.Baro_t("편집 요약 끝에 (봇) 이 붙습니다."))
	set_form += tool.Baro_checkbox("hide", tool.Baro_t("새 가져오기의 최근 변경 숨김을 기본으로 켠다"), "on", hide,
		tool.Baro_t("봇 편집을 최근 변경에서 걸러 낼 수 있도록 문서마다 표시를 남깁니다."))
	set_form += tool.Baro_submit(tool.Baro_t("설정 저장"), "primary")

	body += tool.Baro_card(tool.Baro_t("속도 제한과 표시"), tool.Baro_form("/baro/import/bot", set_form))

	// 최근 가져오기 작업 : 사전 검토와 전체 취소로 이어 준다.
	jobs := []baro_imp_job{}

	func() {
		rows := tool.Query_DB(
			db,
			"select id, kind, source, file, status, actor, date, report, num, license, options from baro_import order by id + 0 desc limit 20",
		)
		defer rows.Close()

		for rows.Next() {
			job := baro_imp_job{}
			if err := rows.Scan(&job.ID, &job.Kind, &job.Source, &job.File, &job.Status, &job.Actor, &job.Date, &job.Report, &job.Num, &job.License, &job.Options); err != nil {
				continue
			}

			jobs = append(jobs, job)
		}
	}()

	job_rows := [][]string{}
	for _, job := range jobs {
		name, kind := baro_imp_status_name(job.Status)
		done := baro_imp_item_count(db, job.ID, "imported")

		test := tool.Baro_badge(tool.Baro_t("아직 안 함"), "muted")
		if baro_imp_opt(job, "test_date") != "" {
			test = tool.Baro_badge(tool.Baro_t("사전 검토 ")+tool.Baro_date_only(baro_imp_opt(job, "test_date")), "ok")
		}

		job_rows = append(job_rows, []string{
			`<a href="/baro/import/view/` + tool.Url_parser(job.ID) + `">#` + tool.HTML_escape(job.ID) + `</a>`,
			tool.HTML_escape(tool.Baro_trunc(job.Source, 40)),
			tool.Baro_badge(name, kind),
			test,
			tool.Baro_comma(done) + " / " + tool.Baro_comma(tool.Str_to_int(job.Num)),
			tool.Baro_button("/baro/import/test/"+tool.Url_parser(job.ID), "사전 검토(10건)", "") + " " +
				tool.Baro_button("/baro/import/undo/"+tool.Url_parser(job.ID), "전체 취소", "danger"),
		})
	}

	job_table := tool.Baro_table(
		[]string{"작업", tool.Baro_t("원본"), "상태", tool.Baro_t("사전 검토"), tool.Baro_t("가져옴 / 전체"), tool.Baro_t("대량 처리")},
		job_rows,
		tool.Baro_t("가져오기 작업이 없습니다."),
	)

	if len(job_rows) == 0 {
		job_table = tool.Baro_empty(tool.Baro_t("가져오기 작업이 없습니다. 먼저 새 가져오기를 만드세요."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("최근 가져오기 작업"), tool.Baro_t("최근 20건"), job_table,
		tool.Baro_button("/baro/import", tool.Baro_t("전체 목록 보기"), ""))

	body += tool.Baro_card(tool.Baro_t("대량 작업 안전장치"), tool.Baro_list([]string{
		tool.Baro_t("사전 검토 — 처음 10건만 실행해 결과를 눈으로 확인합니다."),
		tool.Baro_t("속도 제한 — 분당 처리 건수를 넘으면 스스로 멈춥니다."),
		tool.Baro_t("스냅샷 — 문서를 넣기 전에 원래 내용을 저장해 둡니다."),
		tool.Baro_t("전체 취소 — 이 가져오기로 만들어진 문서를 한 번에 되돌립니다."),
		tool.Baro_t("기록 — 공개 운영 로그와 내부 감사 기록이 함께 남습니다."),
	}))

	return tool.Baro_page(db, config, "대량 가져오기 봇", body, baro_imp_menu())
}

// ------------------------------------------------------------
//  /baro/import/test/<id> : 사전 검토 (테스트 가져오기 10건)
// ------------------------------------------------------------

// View_baro_imp_test : 처음 10건만 가져와 결과를 보여준다
func View_baro_imp_test(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/import/bot"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "import") {
		return tool.Baro_deny(db, config, tool.Baro_t("대량 가져오기는 시스템 관리자 역할이 있어야 씁니다."))
	}

	job, found := baro_imp_job_get(db, id)
	if !found {
		return baro_imp_not_found(db, config, id)
	}

	notice := ""

	if values != nil {
		notice = baro_imp_run_post(db, config, job, values, 10)
		if strings.HasPrefix(notice, "/baro/") {
			job, _ = baro_imp_job_get(db, id)
			baro_imp_opt_set(db, job, map[string]string{"test_date": tool.Baro_now(), "test_by": config.IP})

			return tool.Get_redirect("/baro/import/test/" + tool.Url_parser(id))
		}

		job, _ = baro_imp_job_get(db, id)
	}

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/import", "자료 이전"}, {"/baro/import/bot", "대량 가져오기 봇"}, {"", tool.Baro_t("사전 검토 #") + job.ID}})
	body += baro_imp_tabs("/baro/import/bot")

	if notice != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(notice))
	}

	body += baro_imp_job_head(db, config, job)

	if date := baro_imp_opt(job, "test_date"); date != "" {
		body += tool.Baro_alert("ok",
			tool.Baro_t("사전 검토를 ")+tool.HTML_escape(date)+tool.Baro_t(" 에 실행했습니다. 실행한 사람 : ")+
				tool.Baro_user_link(db, baro_imp_opt(job, "test_by"), config.IP))
	}

	// 사전 검토로 들어간 문서 (가장 최근 10건)
	imported := baro_imp_item_list(db, job.ID, "imported", 0, 10)

	rows := [][]string{}
	for _, item := range imported {
		meta := baro_imp_body_meta(db, item.ID)

		length := meta["length"]
		if length == "" {
			length = "0"
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(item.Title),
			tool.Baro_comma(tool.Str_to_int(length)) + tool.Baro_t("자"),
			tool.HTML_escape(tool.Baro_trunc(item.Note, 80)),
			`<a href="/w/` + tool.Url_parser(item.Title) + tool.Baro_t(`">문서 보기</a> · <a href="/history/`) + tool.Url_parser(item.Title) + tool.Baro_t(`">역사</a>`),
		})
	}

	table := tool.Baro_table([]string{"문서", "길이", tool.Baro_t("비고"), "확인"}, rows, tool.Baro_t("아직 들어간 문서가 없습니다."))
	if len(rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("아직 들어간 문서가 없습니다. 아래에서 사전 검토를 실행하세요."))
	}

	body += tool.Baro_card(tool.Baro_t("사전 검토 결과 (처음 10건)"), table)

	if tool.Baro_read_only(db) {
		body += tool.Baro_alert("warn", tool.Baro_t("읽기 전용 상태라 실행할 수 없습니다."))
	}

	form := tool.Baro_alert("info", tool.Baro_t("처음 <b>10건</b>만 실제로 가져옵니다. 결과가 마음에 들지 않으면 전체 취소로 되돌리세요."))
	form += baro_imp_gate(db, job, tool.Baro_t("사전 검토 실행 (10건)"))

	body += tool.Baro_card("사전 검토 실행", tool.Baro_form("/baro/import/test/"+tool.Url_parser(job.ID), form))

	body += tool.Baro_card(tool.Baro_t("다음 단계"),
		tool.Baro_button("/baro/import/view/"+tool.Url_parser(job.ID), "미리보기로 돌아가기", "")+" "+
			tool.Baro_button("/baro/import/run/"+tool.Url_parser(job.ID), "전체 실행 화면", "primary")+" "+
			tool.Baro_button("/baro/import/undo/"+tool.Url_parser(job.ID), "전체 취소", "danger"))

	return tool.Baro_page(db, config, tool.Baro_t("사전 검토 #")+job.ID, body, baro_imp_menu())
}

// ------------------------------------------------------------
//  /baro/import/undo/<id> : 전체 취소
// ------------------------------------------------------------

// View_baro_imp_undo : 이 가져오기로 만들어진 문서를 한 번에 되돌린다
func View_baro_imp_undo(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/import/bot"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "import") {
		return tool.Baro_deny(db, config, tool.Baro_t("전체 취소는 시스템 관리자 역할이 있어야 씁니다."))
	}

	job, found := baro_imp_job_get(db, id)
	if !found {
		return baro_imp_not_found(db, config, id)
	}

	notice := ""
	notice_kind := "warn"

	if values != nil {
		reason := strings.TrimSpace(values.Get("reason"))

		switch {
		case tool.Baro_read_only(db):
			notice = tool.Baro_t("읽기 전용 상태라 되돌릴 수 없습니다.")
		case values.Get("confirm") != "on":
			notice = tool.Baro_t("되돌리기 확인란에 체크해야 실행할 수 있습니다.")
		case reason == "":
			notice = tool.Baro_t("되돌리는 이유를 적어야 합니다.")
		default:
			count, kept := baro_imp_undo_run(db, config, job, reason)

			switch {
			case count == 0 && kept == 0:
				notice = tool.Baro_t("되돌릴 문서가 없습니다.")
			case count == 0:
				notice = tool.Baro_t("되돌린 문서가 없습니다. ") + tool.Baro_itoa(kept) +
					tool.Baro_t("건은 가져온 뒤 다른 사람이 고쳐서 그대로 두었습니다.")
			default:
				return tool.Get_redirect("/baro/import/view/" + tool.Url_parser(job.ID))
			}
		}

		job, _ = baro_imp_job_get(db, id)
	}

	imported := baro_imp_item_list(db, job.ID, "imported", 0, 200)

	rows := [][]string{}
	changed_count := 0

	for _, item := range imported {
		exist := tool.Baro_badge(tool.Baro_t("문서 있음"), "ok")
		if !baro_imp_doc_exist(db, item.Title) {
			exist = tool.Baro_badge(tool.Baro_t("이미 없음"), "muted")
		} else if baro_imp_undo_changed(db, item) {
			exist = tool.Baro_badge(tool.Baro_t("가져온 뒤 고쳐짐"), "warn")
			changed_count++
		}

		snap := tool.Baro_badge(tool.Baro_t("복구 자료 있음"), "ok")
		if !baro_imp_undo_has(db, item.ID) {
			snap = tool.Baro_badge(tool.Baro_t("복구 자료 없음"), "danger")
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(item.Title),
			exist,
			snap,
			tool.Baro_time_cell(item.Date),
		})
	}

	total := baro_imp_item_count(db, job.ID, "imported")
	threshold := tool.Baro_mass_threshold(db)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/import", "자료 이전"}, {"/baro/import/view/" + tool.Url_parser(job.ID), "가져오기 #" + job.ID}, {"", "전체 취소"}})
	body += baro_imp_tabs("/baro/import/bot")

	if notice != "" {
		body += tool.Baro_alert(notice_kind, tool.HTML_escape(notice))
	}

	body += baro_imp_job_head(db, config, job)

	if total >= threshold {
		body += tool.Baro_alert("warn",
			tool.Baro_t("되돌릴 문서가 <b>")+tool.Baro_comma(total)+tool.Baro_t("</b>개로 대량 작업 기준(")+tool.Baro_itoa(threshold)+
				tool.Baro_t("개)을 넘습니다. 실행 기록이 대량 작업 목록에도 남습니다."))
	}

	table := tool.Baro_table([]string{"문서", tool.Baro_t("현재 상태"), tool.Baro_t("복구 자료"), tool.Baro_t("가져온 때")}, rows, tool.Baro_t("되돌릴 문서가 없습니다."))
	if len(rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("이 작업으로 들어간 문서가 없습니다."))
	}

	if changed_count > 0 {
		body += tool.Baro_alert("info",
			tool.Baro_t("이 가운데 <b>")+tool.Baro_comma(changed_count)+tool.Baro_t("</b>개는 가져온 뒤 다른 사람이 고친 문서입니다. ")+
				tool.Baro_t("남의 기여를 지우지 않기 위해 그대로 두고, 출처·라이선스 기록도 남겨 둡니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("되돌릴 문서"), tool.Baro_t("모두 ")+tool.Baro_comma(total)+tool.Baro_t("개 (최대 200개까지 보여 줍니다)"), table, "")

	form := tool.Baro_note(tool.Baro_t("되돌리기 전에 지금 내용을 스냅샷으로 저장합니다. 가져오기로 새로 만들어진 문서는 지워지고, 덮어쓴 문서는 예전 내용으로 돌아갑니다. ") +
		tool.Baro_t("가져온 뒤 다른 사람이 고친 문서는 건드리지 않습니다."))
	form += tool.Baro_input("reason", tool.Baro_t("되돌리는 이유"), "", tool.Baro_t("예: 라이선스 확인이 잘못됨"), tool.Baro_t("공개 운영 로그에 남습니다."))
	form += tool.Baro_checkbox("confirm", tool.Baro_t("위 문서들을 되돌린다는 것을 확인했습니다."), "on", false, "")
	form += tool.Baro_submit(tool.Baro_t("전체 취소 실행"), "danger")

	body += tool.Baro_danger_zone("전체 취소", tool.Baro_form("/baro/import/undo/"+tool.Url_parser(job.ID), form))

	return tool.Baro_page(db, config, tool.Baro_t("전체 취소 #")+job.ID, body, baro_imp_menu())
}

// baro_imp_undo_has : 되돌리기용 스냅샷이 있는지
func baro_imp_undo_has(db *sql.DB, item_id string) bool {
	found := ""
	tool.QueryRow_DB(
		db,
		"select id from baro_snapshot where mass_id = ? and kind = 'import_undo'",
		[]any{&found}, baro_imp_undo_key(item_id),
	)

	return found != ""
}

// baro_imp_undo_changed : 가져온 뒤 다른 사람이 문서를 고쳤는지
//
//	0장 원칙 2(삭제보다 보존) : 남이 손댄 문서는 전체 취소로 지우지 않는다.
func baro_imp_undo_changed(db *sql.DB, item baro_imp_item) bool {
	source := strings.TrimSpace(baro_imp_body_get(db, item.ID))
	if source == "" {
		// 가져올 때의 본문이 남아 있지 않으면 비교할 수 없다.
		return false
	}

	return strings.TrimSpace(baro_imp_doc_text(db, item.Title)) != source
}

// baro_imp_undo_run : 전체 취소 실제 처리 (되돌린 문서 수, 그대로 둔 문서 수)
func baro_imp_undo_run(db *sql.DB, config tool.Config, job baro_imp_job, reason string) (int, int) {
	items := baro_imp_item_list(db, job.ID, "imported", 0, 20000)
	if len(items) == 0 {
		return 0, 0
	}

	next_snap := tool.Str_to_int(tool.Baro_next_id(db, "baro_snapshot"))
	titles := []string{}
	count := 0
	kept := 0

	for _, item := range items {
		title := item.Title
		if strings.TrimSpace(title) == "" {
			continue
		}

		// 가져온 뒤 다른 사람이 고친 문서는 건드리지 않는다. (출처·라이선스 기록도 그대로 둔다)
		if baro_imp_undo_changed(db, item) {
			tool.Exec_DB(db, "update baro_import_item set note = ? where id = ?",
				"가져온 뒤 다른 사람이 고쳐서 되돌리지 않았습니다.", item.ID)
			kept++

			continue
		}

		now_text := baro_imp_doc_text(db, title)

		// 대량 작업 원칙 3 : 되돌리기 전에 현재 내용을 반드시 저장한다.
		tool.Exec_DB(db, "delete from baro_snapshot where mass_id = ? and kind = 'import_revert'", "imprevert:"+item.ID)
		tool.Exec_DB(
			db,
			"insert into baro_snapshot (id, mass_id, kind, title, data, meta, date) values (?, ?, 'import_revert', ?, ?, ?, ?)",
			tool.Baro_itoa(next_snap), "imprevert:"+item.ID, title, now_text,
			tool.Baro_json_enc(map[string]string{"import_id": job.ID, "item_id": item.ID, "reason": reason}), tool.Baro_now(),
		)
		next_snap++

		old_text := ""
		meta := map[string]string{}

		data, meta_data := "", ""
		tool.QueryRow_DB(
			db,
			"select data, meta from baro_snapshot where mass_id = ? and kind = 'import_undo'",
			[]any{&data, &meta_data}, baro_imp_undo_key(item.ID),
		)

		old_text = data
		if decoded := tool.Baro_json_dec_str(meta_data); decoded != nil {
			meta = decoded
		}

		if meta["exist"] == "on" && strings.TrimSpace(old_text) != "" {
			// 덮어쓰기로 들어간 문서 : 예전 내용으로 되돌린다.
			tool.Exec_DB(db, "delete from data where title = ?", title)
			tool.Exec_DB(db, "insert into data (title, data) values (?, ?)", title, old_text)
			tool.Do_add_history(db, title, old_text, tool.Get_time(), config.IP, "가져오기 취소",
				tool.Get_edit_length_diff(now_text, old_text), "", "")
			baro_imp_render_safe(db, title, old_text)
		} else {
			// 가져오기로 새로 만들어진 문서 : 흔적을 지운다.
			tool.Exec_DB(db, "delete from data where title = ?", title)
			tool.Exec_DB(db, "delete from history where title = ?", title)
			tool.Exec_DB(db, "delete from data_set where doc_name = ?", title)

			// back(역링크) 표는 양쪽을 모두 지워야 한다. (openNAMU 의 문서 삭제와 같은 방식)
			//   link = 지운 문서  : 이 문서가 걸어 둔 링크 행 (남겨 두면 지운 제목이 남의 역링크로 계속 보인다)
			//   title = 지운 문서 : 이 문서를 가리키던 링크 행
			tool.Exec_DB(db, "delete from back where link = ?", title)
			tool.Exec_DB(db, "delete from back where title = ?", title)
		}

		tool.Exec_DB(db, "delete from baro_doc_meta where title = ? and name like 'import_%'", title)
		tool.Exec_DB(db, "update baro_contrib_event set status = 'rejected' where kind = 'import' and title = ? and detail = ?",
			title, "가져오기 작업 #"+job.ID)

		tool.Exec_DB(db, "update baro_import_item set status = 'undone', note = ? where id = ?",
			"전체 취소로 되돌림", item.ID)

		if len(titles) < 300 {
			titles = append(titles, title)
		}

		count++
	}

	if count == 0 {
		return 0, kept
	}

	// 대량 작업 기록을 남긴다. (0장 원칙 3)
	mass_id := tool.Baro_next_id(db, "baro_mass_op")
	tool.Exec_DB(
		db,
		"insert into baro_mass_op (id, kind, actor, target_list, num, reason, status, approver, created, approved_date, execute_date, grace_until, urgent, undo_status, detail) "+
			"values (?, 'revert', ?, ?, ?, ?, 'done', ?, ?, ?, ?, '', '', 'snapshot', ?)",
		mass_id, config.IP, tool.Baro_json_enc(titles), tool.Baro_itoa(count), reason,
		config.IP, tool.Baro_now(), tool.Baro_now(), tool.Baro_now(),
		"가져오기 #"+job.ID+" 전체 취소",
	)

	undo_status := "undone"
	kept_text := ""

	if kept > 0 {
		// 남이 고쳐서 그대로 둔 문서가 있으면 "일부만" 되돌린 것이다.
		undo_status = "partial"
		kept_text = " 문서 " + tool.Baro_itoa(kept) + tool.Baro_t("건은 가져온 뒤 다른 사람이 고쳐서 그대로 두었습니다.")
	}

	tool.Exec_DB(db, "update baro_import set status = ? where id = ?", undo_status, job.ID)

	tool.Baro_public_log(db, "import_undo", config.IP, "가져오기 #"+job.ID, reason,
		"문서 "+tool.Baro_itoa(count)+"건을 되돌렸습니다."+kept_text)
	tool.Baro_audit(db, config, "import_undo", "baro_import#"+job.ID,
		"imported="+tool.Baro_itoa(count), "undone kept="+tool.Baro_itoa(kept), reason)
	tool.Baro_notify(db, job.Actor, "system",
		"가져오기 #"+job.ID+tool.Baro_t(" 이(가) 전체 취소됐습니다."),
		"/baro/import/view/"+tool.Url_parser(job.ID), "")
	tool.Baro_metric_bump(db, "import_undo", tool.Baro_today(), count)

	return count, kept
}
