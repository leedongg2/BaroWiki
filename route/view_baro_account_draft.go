package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

// ============================================================
//  2-5 사용자 문서 · 연습장 (초안 목록 / 작성 / 수정 / 삭제 / 게시)
// ============================================================

// baro_account_draft_deny : 초안 접근 권한이 없을 때
func baro_account_draft_deny(db *sql.DB, config tool.Config, message string) string {
	body := tool.Baro_alert("warn", message)
	body += tool.Baro_button("/baro/draft", tool.Baro_t("내 초안 목록"), "primary")

	return tool.Baro_page(db, config, tool.Baro_t("내 초안"), body, baro_account_menu())
}

// baro_account_draft_mine : 내 초안인지 확인하고 돌려준다
func baro_account_draft_mine(db *sql.DB, config tool.Config, id string) (map[string]string, string) {
	draft := baro_account_draft_get(db, id)
	if draft == nil {
		return nil, baro_account_draft_deny(db, config, tool.Baro_t("그런 초안이 없습니다. 이미 지워졌거나 주소가 잘못됐을 수 있습니다."))
	}

	if draft["user"] != config.IP {
		return nil, baro_account_draft_deny(db, config, tool.Baro_t("다른 사람의 초안은 열 수 없습니다."))
	}

	return draft, ""
}

// baro_account_draft_block_reason : 제재 때문에 게시가 막히는지 (1번 원칙 : 사람에게만 적용)
func baro_account_draft_block_reason(db *sql.DB, user string) string {
	for _, row := range tool.Baro_sanction_active(db, user) {
		switch row[1] {
		case "short_block", "long_block", "perm_block":
			return tool.Baro_sanction_name(row[1]) + tool.Baro_t(" 상태라 문서를 게시할 수 없습니다. 사유 : ") + row[3]
		}
	}

	return ""
}

// baro_account_backlink : 게시 후 역링크 갱신 (실패해도 게시는 유지된다)
func baro_account_backlink(db *sql.DB, title string, data string) {
	defer func() {
		_ = recover()
	}()

	markup.Get_render(db, title, data, "backlink")
}

// View_baro_account_draft_list : 내 초안 목록 (/baro/draft)
func View_baro_account_draft_list(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/draft"); need {
		return wall
	}

	user := config.IP

	// --- 쓰기 처리 (공개 설정 바꾸기) ---
	if values != nil {
		if tool.Baro_read_only(db) {
			return baro_account_draft_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db))
		}

		if values.Get("action") == "status" {
			draft, deny := baro_account_draft_mine(db, config, strings.TrimSpace(values.Get("id")))
			if deny != "" {
				return deny
			}

			status := values.Get("status")
			if status != "open" && status != "closed" {
				status = "closed"
			}

			if draft["status"] != "done" {
				tool.Exec_DB(db, "update baro_workspace set status = ? where id = ? and user = ?", status, draft["id"], user)
			}
		}

		return tool.Get_redirect("/baro/draft")
	}

	page_int := baro_account_page_int(page)
	max_count := tool.Baro_page_size

	rows := tool.Query_DB(
		db,
		"select id, project, title, data, date, status, note from baro_workspace where user = ? order by id + 0 desc limit ?, ?",
		user, (page_int-1)*max_count, max_count,
	)
	defer rows.Close()

	list_rows := [][]string{}
	for rows.Next() {
		id, project, title, data, date, status, note := "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &project, &title, &data, &date, &status, &note); err != nil {
			continue
		}

		show_title := title
		if strings.TrimSpace(show_title) == "" {
			show_title = tool.Baro_t("제목 없는 초안")
		}

		place := tool.Baro_t("개인 초안")
		if project == "sandbox" {
			place = tool.Baro_t("연습장")
		} else if project != "" {
			project_name := ""
			tool.QueryRow_DB(db, "select name from baro_project where id = ?", []any{&project_name}, project)
			if project_name != "" {
				place = project_name
			}
		}

		manage := tool.Baro_button("/baro/draft/edit/"+tool.Url_parser(id), "수정", "")
		if status != "done" {
			manage += " " + tool.Baro_button("/baro/draft/publish/"+tool.Url_parser(id), "문서로 올리기", "primary")
		}
		manage += " " + tool.Baro_button("/baro/draft/delete/"+tool.Url_parser(id), "삭제", "danger")

		toggle := ""
		if status != "done" {
			next := "open"
			label := tool.Baro_t("공개로 바꾸기")
			if status == "open" {
				next = "closed"
				label = tool.Baro_t("비공개로 바꾸기")
			}

			toggle = tool.Baro_form("/baro/draft",
				tool.Baro_hidden("action", "status")+
					tool.Baro_hidden("id", id)+
					tool.Baro_hidden("status", next)+
					tool.Baro_submit(label, ""))
		}

		list_rows = append(list_rows, []string{
			`<a href="/baro/draft/edit/` + tool.Url_parser(id) + `">` + tool.HTML_escape(tool.Baro_trunc(show_title, 50)) + `</a>` +
				`<div class="bw_note">` + tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(data), 80)) + `</div>`,
			tool.HTML_escape(place),
			tool.Baro_badge(baro_account_draft_status_name(status), baro_account_draft_status_kind(status)) + toggle,
			tool.Baro_comma(tool.Get_len(data)) + tool.Baro_t("자"),
			tool.Baro_time_cell(date),
			manage,
		})
	}

	total := 0
	tool.QueryRow_DB(db, "select count(*) from baro_workspace where user = ?", []any{&total}, user)

	body := tool.Baro_hero(tool.Baro_t("내 초안"), tool.Baro_t("쓰다 만 문서를 여기에 모아 둡니다. 준비되면 실제 문서로 올립니다."),
		tool.Baro_button("/baro/draft/new", tool.Baro_t("새 초안 만들기"), "primary")+" "+
			tool.Baro_button("/baro/draft/sandbox", tool.Baro_t("연습장"), "")+" "+
			tool.Baro_button("/baro/draft/cite", tool.Baro_t("출처 입력 도우미"), ""))

	body += baro_account_tabs("/baro/draft")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 초안"), tool.Baro_comma(total), "건"},
		{tool.Baro_t("이번 쪽"), tool.Baro_comma(len(list_rows)), "건"},
	})

	if len(list_rows) == 0 {
		body += tool.Baro_empty(tool.Baro_t("아직 초안이 없습니다. 새 초안을 만들거나 연습장에서 먼저 연습해 보세요."))
	} else {
		body += tool.Baro_table(
			[]string{tool.Baro_t("제목"), tool.Baro_t("위치"), tool.Baro_t("공개 설정"), "길이", tool.Baro_t("마지막 저장"), "관리"},
			list_rows,
			tool.Baro_t("아직 초안이 없습니다."),
		)
	}

	body += tool.Baro_paging(page_int, len(list_rows), max_count, "/baro/draft?page={}")

	body += tool.Baro_card(tool.Baro_t("알아두면 좋은 것"),
		tool.Baro_list([]string{
			tool.Baro_t("초안은 하위 문서처럼 여러 개를 만들 수 있습니다. 제목을 붙여 구분하세요."),
			tool.Baro_t("비공개 초안은 나만 볼 수 있고, 공개로 바꾸면 다른 기여자가 도와줄 수 있습니다."),
			tool.Baro_t("자기 소개는 초안이 아니라 ") + baro_account_user_doc_link(config.IP) + tool.Baro_t(" 문서에 씁니다."),
			tool.Baro_t(`연습만 하고 싶다면 <a href="/baro/draft/sandbox">연습장</a> 을 쓰세요. 실제 문서에 영향을 주지 않습니다.`),
		}))

	return tool.Baro_page(db, config, tool.Baro_t("내 초안"), body, baro_account_menu())
}

// View_baro_account_draft_new : 새 초안 (/baro/draft/new)
func View_baro_account_draft_new(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/draft/new"); need {
		return wall
	}

	user := config.IP

	if values != nil {
		if tool.Baro_read_only(db) {
			return baro_account_draft_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db))
		}

		if !tool.Baro_rate_ok(db, "baro_account_draft_new", user, 30, 3600) {
			return baro_account_draft_deny(db, config, tool.Baro_t("짧은 시간에 너무 많은 초안을 만들었습니다. 잠시 뒤 다시 시도해 주세요."))
		}

		title := tool.Baro_one_line(values.Get("title"))
		if strings.TrimSpace(title) == "" {
			title = tool.Baro_t("제목 없는 초안")
		}

		status := values.Get("status")
		if status != "open" && status != "closed" {
			status = "closed"
		}

		id := tool.Baro_next_id(db, "baro_workspace")
		tool.Exec_DB(
			db,
			"insert into baro_workspace (id, project, title, data, user, date, status, note) values (?, '', ?, ?, ?, ?, ?, ?)",
			id, tool.Baro_trunc(title, 150), values.Get("data"), user, tool.Baro_now(), status,
			tool.Baro_trunc(tool.Baro_one_line(values.Get("note")), 200),
		)

		return tool.Get_redirect("/baro/draft/edit/" + tool.Url_parser(id))
	}

	prefill := ""
	if page != "" {
		prefill = tool.Baro_one_line(page)
	}

	form := tool.Baro_input("title", tool.Baro_t("초안 제목"), prefill, tool.Baro_t("예: 홍길동 (독립운동가)"), tool.Baro_t("실제 문서 이름과 같게 해도 되고, 나중에 바꿔도 됩니다."))
	form += tool.Baro_select("status", tool.Baro_t("공개 설정"), baro_account_draft_status_list(), "closed", tool.Baro_t("공개로 두면 다른 기여자가 보고 도와줄 수 있습니다."))
	form += tool.Baro_field_row_html(tool.Baro_t("본문"),
		`<textarea id="bw_acc_new_area" class="bw_in bw_ta" rows="18" name="data">`+tool.HTML_escape(baro_account_onboard_example())+`</textarea>`,
		tool.Baro_t("아래 문법 안내를 참고하세요. 예시 내용은 지우고 쓰시면 됩니다."))
	form += baro_account_editor_js("bw_acc_new_area", "new")
	form += tool.Baro_input("note", tool.Baro_t("메모"), "", tool.Baro_t("예: 출처 두 개 더 찾을 것"), tool.Baro_t("나만 보는 메모입니다."))
	form += tool.Baro_submit(tool.Baro_t("초안 만들기"), "primary")

	body := tool.Baro_hero("새 초안", tool.Baro_t("먼저 초안으로 써 두고, 준비되면 문서로 올립니다."), "")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/draft", tool.Baro_t("내 초안")}, {"", "새 초안"}})
	body += tool.Baro_card(tool.Baro_t("초안 작성"), tool.Baro_form("/baro/draft/new", form))
	body += tool.Baro_card(tool.Baro_t("문법 안내"), baro_account_syntax_guide())
	body += tool.Baro_note(tool.Baro_t("본문 입력칸은 이 브라우저에 임시 저장됩니다. 서버에 저장하려면 아래 단추를 눌러야 합니다."))

	return tool.Baro_page(db, config, "새 초안", body, baro_account_menu())
}

// View_baro_account_draft_edit : 초안 수정 (/baro/draft/edit/<id>)
func View_baro_account_draft_edit(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/draft"); need {
		return wall
	}

	draft, deny := baro_account_draft_mine(db, config, strings.TrimSpace(id))
	if deny != "" {
		return deny
	}

	preview := ""
	body_data := draft["data"]
	title_data := draft["title"]

	if values != nil {
		if tool.Baro_read_only(db) {
			return baro_account_draft_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db))
		}

		title_data = tool.Baro_one_line(values.Get("title"))
		body_data = values.Get("data")

		status := values.Get("status")
		if status != "open" && status != "closed" {
			status = draft["status"]
		}

		if values.Get("action") == "preview" {
			preview = baro_account_render(db, body_data)
		} else {
			tool.Exec_DB(
				db,
				"update baro_workspace set title = ?, data = ?, status = ?, note = ?, date = ? where id = ? and user = ?",
				tool.Baro_trunc(title_data, 150), body_data, status,
				tool.Baro_trunc(tool.Baro_one_line(values.Get("note")), 200),
				tool.Baro_now(), draft["id"], config.IP,
			)

			return tool.Get_redirect("/baro/draft/edit/" + tool.Url_parser(draft["id"]))
		}
	}

	check := baro_account_scan(db, body_data)

	form := tool.Baro_input("title", tool.Baro_t("초안 제목"), title_data, "", "")
	form += tool.Baro_select("status", tool.Baro_t("공개 설정"), baro_account_draft_status_list(), draft["status"], "")
	form += tool.Baro_field_row_html(tool.Baro_t("본문"),
		`<textarea id="bw_acc_edit_area" class="bw_in bw_ta" rows="20" name="data">`+tool.HTML_escape(body_data)+`</textarea>`,
		tool.Baro_t("간단 편집 단추로 문법을 넣을 수 있습니다."))
	form += baro_account_editor_js("bw_acc_edit_area", "edit_"+draft["id"])
	form += tool.Baro_input("note", tool.Baro_t("메모"), draft["note"], "", tool.Baro_t("나만 보는 메모입니다."))
	form += tool.Baro_submit("저장", "primary")
	form += tool.Baro_t(` <button type="submit" class="bw_btn" name="action" value="preview">미리보기</button>`)

	body := tool.Baro_hero(tool.Baro_t("초안 수정"), tool.Baro_trunc(title_data, 60), "")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/draft", tool.Baro_t("내 초안")}, {"", "수정"}})

	if draft["status"] == "done" {
		body += tool.Baro_alert("ok", tool.Baro_t("이미 문서로 올린 초안입니다. 여기서 고쳐도 실제 문서는 바뀌지 않습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("초안 편집"), tool.Baro_form("/baro/draft/edit/"+draft["id"], form))

	if preview != "" {
		body += tool.Baro_card("미리보기", preview)
	}

	body += tool.Baro_card(tool.Baro_t("문법 자동 안내"), baro_account_scan_html(check))
	body += tool.Baro_card(tool.Baro_t("제출 전 체크리스트"), baro_account_checklist_html(title_data, check))

	body += tool.Baro_card(tool.Baro_t("다음 단계"),
		tool.Baro_button("/baro/draft/publish/"+tool.Url_parser(draft["id"]), "문서로 올리기", "primary")+" "+
			tool.Baro_button("/baro/draft/cite", tool.Baro_t("출처 입력 도우미"), "")+" "+
			tool.Baro_button("/baro/draft", tool.Baro_t("목록으로"), "")+" "+
			tool.Baro_button("/baro/draft/delete/"+tool.Url_parser(draft["id"]), "삭제", "danger"))

	return tool.Baro_page(db, config, tool.Baro_t("초안 수정"), body, baro_account_menu())
}

// View_baro_account_draft_delete : 초안 삭제 (/baro/draft/delete/<id>)
func View_baro_account_draft_delete(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/draft"); need {
		return wall
	}

	draft, deny := baro_account_draft_mine(db, config, strings.TrimSpace(id))
	if deny != "" {
		return deny
	}

	if values != nil {
		if tool.Baro_read_only(db) {
			return baro_account_draft_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db))
		}

		if values.Get("agree") != "on" {
			return baro_account_draft_deny(db, config, tool.Baro_t("삭제 확인란을 체크해야 지울 수 있습니다."))
		}

		tool.Exec_DB(db, "delete from baro_workspace where id = ? and user = ?", draft["id"], config.IP)
		tool.Baro_audit(db, config, "draft_delete", draft["id"], tool.Baro_trunc(draft["data"], 500), "", "본인 초안 삭제")

		return tool.Get_redirect("/baro/draft")
	}

	show_title := draft["title"]
	if strings.TrimSpace(show_title) == "" {
		show_title = tool.Baro_t("제목 없는 초안")
	}

	body := tool.Baro_hero(tool.Baro_t("초안 삭제"), show_title, "")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/draft", tool.Baro_t("내 초안")}, {"", "삭제"}})
	body += tool.Baro_card(tool.Baro_t("지울 내용"), tool.Baro_code(tool.Baro_trunc(draft["data"], 1000)))
	body += tool.Baro_danger_zone(tool.Baro_t("정말 지울까요?"),
		tool.Baro_alert("warn", tool.Baro_t("초안 삭제는 되돌릴 수 없습니다. 나중에 쓸 수도 있다면 비공개로 두는 편이 좋습니다."))+
			tool.Baro_form("/baro/draft/delete/"+draft["id"],
				tool.Baro_checkbox("agree", tool.Baro_t("이 초안을 완전히 지우는 데 동의합니다."), "on", false, "")+
					tool.Baro_submit("삭제", "danger"))+
			" "+tool.Baro_button("/baro/draft/edit/"+tool.Url_parser(draft["id"]), "취소하고 돌아가기", ""))

	return tool.Baro_page(db, config, tool.Baro_t("초안 삭제"), body, baro_account_menu())
}

// View_baro_account_draft_publish : 초안을 실제 문서로 올리기 (/baro/draft/publish/<id>)
func View_baro_account_draft_publish(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/draft"); need {
		return wall
	}

	user := config.IP

	draft, deny := baro_account_draft_mine(db, config, strings.TrimSpace(id))
	if deny != "" {
		return deny
	}

	license := baro_account_policy(db, "license")
	block := baro_account_draft_block_reason(db, user)
	want_title := tool.Baro_one_line(draft["title"])
	message := ""

	if values != nil {
		if tool.Baro_read_only(db) {
			return baro_account_draft_deny(db, config, tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.Baro_read_only_reason(db))
		}

		want_title = tool.Baro_one_line(values.Get("doc_name"))
		field := values.Get("field")

		switch {
		case block != "":
			message = block
		case tool.Get_len(strings.TrimSpace(want_title)) < 1:
			message = tool.Baro_t("문서 이름을 입력해 주세요.")
		case values.Get("agree") != "on":
			message = tool.Baro_t("라이선스 안내를 읽고 동의해야 게시할 수 있습니다.")
		case strings.TrimSpace(draft["data"]) == "":
			message = tool.Baro_t("본문이 비어 있습니다. 먼저 내용을 채워 주세요.")
		case !tool.Baro_rate_ok(db, "baro_account_publish", user, 20, 3600):
			message = tool.Baro_t("짧은 시간에 너무 많은 문서를 올렸습니다. 잠시 뒤 다시 시도해 주세요.")
		}

		doc_name := tool.Namespace_normalize(strings.TrimSpace(want_title))

		if message == "" && baro_account_doc_exists(db, doc_name) {
			message = tool.Baro_t("이미 같은 이름의 문서가 있습니다. 덮어쓰지 않습니다. 그 문서를 직접 편집하거나 다른 이름을 쓰세요.")
		}

		if message == "" && !tool.Check_acl(db, doc_name, "", "document_edit", config.IP) {
			message = tool.Baro_t("이 문서를 편집할 권한이 없습니다.")
		}

		if message == "" && !tool.Check_acl(db, doc_name, "", "document_make_acl", config.IP) {
			message = tool.Baro_t("새 문서를 만들 권한이 없습니다.")
		}

		if message == "" {
			// 라이선스 동의 기록 (15-4)
			baro_account_consent_add(db, config, "license", license[0])

			data := draft["data"]
			date := tool.Get_time()

			tool.Exec_DB(db, "delete from data where title = ?", doc_name)
			tool.Exec_DB(db, "insert into data (title, data) values (?, ?)", doc_name, data)

			tool.Do_add_history(
				db, doc_name, data, date, config.IP,
				"바로위키 초안 게시", tool.Get_edit_length_diff("", data), "", "",
			)

			baro_account_backlink(db, doc_name, data)

			if strings.TrimSpace(field) != "" {
				tool.Baro_doc_meta_set(db, doc_name, "field", field)
			}

			tool.Baro_doc_status_set(db, doc_name, "draft", "초안에서 새로 올린 문서", user)

			rev := tool.Get_document_revision(db, doc_name)
			if rev == "" {
				rev = "1"
			}

			tool.Baro_contrib_add(db, user, "new_document", doc_name, rev, "", "초안 "+draft["id"]+" 에서 게시")

			tool.Exec_DB(
				db,
				"update baro_workspace set status = 'done', note = ?, date = ? where id = ? and user = ?",
				tool.Baro_trunc("게시함 : "+doc_name, 200), tool.Baro_now(), draft["id"], user,
			)

			tool.Baro_metric_bump(db, "draft_publish", tool.Baro_today(), 1)
			tool.Baro_notify(db, user, "system", "초안을 문서로 올렸습니다 : "+doc_name, "/w/" + tool.Doc_url(doc_name), "")

			// 공개용(개인정보 없음)과 내부 상세 기록을 함께 남긴다 (5번 원칙)
			tool.Baro_public_log(db, "doc_new", user, doc_name, "초안에서 새 문서 게시", tool.Baro_field_name(db, field))
			tool.Baro_audit(db, config, "draft_publish", doc_name, "", tool.Baro_trunc(data, 500), "초안 게시")

			return tool.Get_redirect("/w/" + tool.Doc_url(doc_name))
		}
	}

	check := baro_account_scan(db, draft["data"])

	fields := [][]string{{"", tool.Baro_t("고르지 않음")}}
	fields = append(fields, baro_account_field_options(db)...)

	default_field := tool.Baro_doc_field(db, tool.Namespace_normalize(want_title))
	if default_field == "" {
		interests := tool.Baro_user_interest(db, user)
		if len(interests) > 0 {
			default_field = interests[0]
		}
	}

	form := tool.Baro_input("doc_name", tool.Baro_t("문서 이름"), want_title, tool.Baro_t("예: 홍길동 (독립운동가)"), tool.Baro_t("이미 있는 이름이면 게시하지 않고 알려드립니다."))
	form += tool.Baro_select("field", "분야", fields, default_field, tool.Baro_t("분야를 지정하면 개척률과 추천에 반영됩니다."))
	form += tool.Baro_checkbox("agree", tool.Baro_t("위 라이선스 안내를 읽었고, 내 기여가 이 라이선스로 공개되는 데 동의합니다."), "on", false, "")
	form += tool.Baro_submit("문서로 올리기", "primary")

	body := tool.Baro_hero(tool.Baro_t("초안을 문서로 올리기"), tool.Baro_trunc(want_title, 60), "")
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/draft", tool.Baro_t("내 초안")}, {"", tool.Baro_t("게시")}})

	if message != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(message))
	}

	if block != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(block))
	}

	if baro_account_doc_exists(db, tool.Namespace_normalize(want_title)) {
		body += tool.Baro_alert("warn",
			`같은 이름의 문서가 이미 있습니다 : `+tool.Baro_doc_link(tool.Namespace_normalize(want_title))+
				tool.Baro_t(` — 덮어쓰지 않습니다. 그 문서를 직접 편집하거나 다른 이름을 고르세요.`))
	}

	body += tool.Baro_card(tool.Baro_t("제출 전 체크리스트"), baro_account_checklist_html(want_title, check))
	body += tool.Baro_card(tool.Baro_t("문법 자동 안내"), baro_account_scan_html(check))

	license_body := tool.Baro_code(license[2])
	if license[2] == "" {
		license_body = tool.Baro_alert("warn", tool.Baro_t("라이선스 정책 본문이 등록되어 있지 않습니다. 관리자에게 알려 주세요."))
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("라이선스 안내"),
		func() string {
			if license[0] == "" {
				return tool.Baro_t("정책 판 정보 없음")
			}

			return license[1] + " " + license[0] + tool.Baro_t("판")
		}(),
		license_body,
		tool.Baro_t(`동의 기록은 <a href="/baro/account">내 계정</a> 에서 확인할 수 있습니다.`),
	)

	body += tool.Baro_card(tool.Baro_t("게시"), tool.Baro_form("/baro/draft/publish/"+draft["id"], form))
	body += tool.Baro_card(tool.Baro_t("본문 미리보기"), baro_account_render(db, draft["data"]))
	body += tool.Baro_note(tool.Baro_t("게시하면 편집 역사가 남고 기여 이벤트로 기록됩니다. 점수는 나중에 확정됩니다."))

	return tool.Baro_page(db, config, "초안 게시", body, baro_account_menu())
}
