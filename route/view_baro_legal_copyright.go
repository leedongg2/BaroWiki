package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  15-3 저작권 요청 센터
//
//  URL : /baro/copyright ...
//
//  접수되면 즉시 삭제가 아니라 임시 격리(baro_quarantine)로 넘긴다. (원칙 2)
// ============================================================

// baro_legal_copyright_row : 저작권 요청 한 건
type baro_legal_copyright_row struct {
	ID         string
	Claimant   string
	Contact    string
	TargetKind string
	Target     string
	Proof      string
	Status     string
	Handler    string
	Date       string
	CloseDate  string
	Result     string
	Counter    string
}

// baro_legal_copyright_get : 요청 한 건 읽기
func baro_legal_copyright_get(db *sql.DB, id string) baro_legal_copyright_row {
	row := baro_legal_copyright_row{}
	if strings.TrimSpace(id) == "" {
		return row
	}

	tool.QueryRow_DB(
		db,
		"select id, claimant, contact, target_kind, target, proof, status, handler, date, close_date, result, counter from baro_copyright_req where id = ?",
		[]any{
			&row.ID, &row.Claimant, &row.Contact, &row.TargetKind, &row.Target, &row.Proof,
			&row.Status, &row.Handler, &row.Date, &row.CloseDate, &row.Result, &row.Counter,
		},
		id,
	)

	return row
}

// baro_legal_copyright_list : 요청 목록
func baro_legal_copyright_list(db *sql.DB, where string, limit int, offset int, values ...any) []baro_legal_copyright_row {
	out := []baro_legal_copyright_row{}

	query := "select id, claimant, contact, target_kind, target, proof, status, handler, date, close_date, result, counter from baro_copyright_req"
	if strings.TrimSpace(where) != "" {
		query += " where " + where
	}
	query += " order by id + 0 desc limit " + tool.Baro_itoa(limit) + " offset " + tool.Baro_itoa(offset)

	rows := tool.Query_DB(db, query, values...)
	defer rows.Close()

	for rows.Next() {
		row := baro_legal_copyright_row{}
		err := rows.Scan(
			&row.ID, &row.Claimant, &row.Contact, &row.TargetKind, &row.Target, &row.Proof,
			&row.Status, &row.Handler, &row.Date, &row.CloseDate, &row.Result, &row.Counter,
		)
		if err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_legal_copyright_target_split : "문서#판번호" 를 문서와 판으로 나눈다
func baro_legal_copyright_target_split(target string) (string, string) {
	pos := strings.LastIndex(target, "#")
	if pos < 1 {
		return target, ""
	}

	rev := target[pos+1:]
	if strings.TrimSpace(rev) == "" || tool.Str_to_int(rev) < 1 {
		return target, ""
	}

	return target[:pos], rev
}

// baro_legal_copyright_doc_exists : 신고 대상 문서가 실제로 있는지
//
//	없는 문서를 격리하면 되살릴 수 없는 껍데기 기록만 남으므로 접수 단계에서 막는다.
func baro_legal_copyright_doc_exists(db *sql.DB, title string) bool {
	if strings.TrimSpace(title) == "" {
		return false
	}

	found := ""
	tool.QueryRow_DB(db, "select title from data where title = ? limit 1", []any{&found}, title)
	if found != "" {
		return true
	}

	tool.QueryRow_DB(db, "select id from history where title = ? limit 1", []any{&found}, title)

	return found != ""
}

// baro_legal_copyright_reject_count : 같은 신고자가 기각당한 횟수
func baro_legal_copyright_reject_count(db *sql.DB, claimant string) int {
	if strings.TrimSpace(claimant) == "" {
		return 0
	}

	return baro_legal_count(db, "select count(*) from baro_copyright_req where claimant = ? and status = 'rejected'", claimant)
}

// baro_legal_copyright_quarantine_open : 이 문서에 열린 격리가 있는지 (격리 번호)
func baro_legal_copyright_quarantine_open(db *sql.DB, title string) string {
	id := ""
	tool.QueryRow_DB(db, "select id from baro_quarantine where title = ? and status = 'open' order by id + 0 desc limit 1", []any{&id}, title)

	return id
}

// baro_legal_copyright_quarantine_for : 이 요청이 만든 격리 번호
func baro_legal_copyright_quarantine_for(db *sql.DB, req_id string) string {
	id := ""
	tool.QueryRow_DB(db, "select id from baro_quarantine where case_id = ? and kind = 'copyright' order by id + 0 desc limit 1", []any{&id}, "copyright:"+req_id)

	return id
}

// baro_legal_copyright_quarantine_status : 격리 상태
func baro_legal_copyright_quarantine_status(db *sql.DB, id string) string {
	status := ""
	if strings.TrimSpace(id) == "" {
		return ""
	}

	tool.QueryRow_DB(db, "select status from baro_quarantine where id = ?", []any{&status}, id)

	return status
}

// baro_legal_copyright_do_quarantine : 신고 대상을 임시 격리한다 (원본은 backup 에 보존)
func baro_legal_copyright_do_quarantine(db *sql.DB, req_id string, target_kind string, target string, reason string) string {
	title, rev := baro_legal_copyright_target_split(target)
	if strings.TrimSpace(title) == "" {
		return ""
	}

	// 원본 보존
	backup := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&backup}, title)

	if rev == "" {
		rev = tool.Get_document_revision(db, title)
	}

	reason = tool.Baro_trunc(baro_legal_target_kind_name(target_kind)+tool.Baro_t(" 대상 ")+reason, 1000)

	id := tool.Baro_next_id(db, "baro_quarantine")
	tool.Exec_DB(
		db,
		"insert into baro_quarantine (id, title, rev, kind, reason, actor, date, status, backup, release_date, case_id) values (?, ?, ?, 'copyright', ?, 'system', ?, 'open', ?, '', ?)",
		id, title, rev, reason, tool.Baro_now(), backup, "copyright:"+req_id,
	)

	// 문서 상태를 격리 중으로 바꾼다
	tool.Baro_doc_status_set(db, title, "quarantined", "저작권 신고 접수 (요청 #"+req_id+")", "system")

	return id
}

// ------------------------------------------------------------
//  15-3 첫 화면 : /baro/copyright
// ------------------------------------------------------------

// View_baro_legal_copyright_home : 저작권 요청 센터
func View_baro_legal_copyright_home(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	total := baro_legal_count(db, "select count(*) from baro_copyright_req")
	open_count := baro_legal_count(db, "select count(*) from baro_copyright_req where "+baro_legal_open_where)
	done_count := baro_legal_count(db, "select count(*) from baro_copyright_req where status = 'done'")
	reject_count := baro_legal_count(db, "select count(*) from baro_copyright_req where status = 'rejected'")
	quarantine_count := baro_legal_count(db, "select count(*) from baro_quarantine where kind = 'copyright' and status = 'open'")

	body := tool.Baro_hero(
		tool.Baro_t("저작권 요청 센터"),
		tool.Baro_t("권리자는 침해를 신고할 수 있고, 원 작성자는 반론을 낼 수 있습니다."),
		tool.Baro_button("/baro/copyright/new", tool.Baro_t("저작권 침해 신고"), "primary")+" "+
			tool.Baro_button("/baro/copyright/mine", tool.Baro_t("내가 낸 신고"), "")+" "+
			tool.Baro_button("/baro/legal", tool.Baro_t("처리 통계"), ""),
	)

	body += tool.Baro_card_ex(
		tool.Baro_t("지금까지의 처리"),
		tool.Baro_t("이름과 문서명 없이 건수만 공개합니다. (원칙 5)"),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("전체 신고"), tool.Baro_comma(total), ""},
			{tool.Baro_t("처리 중"), tool.Baro_comma(open_count), ""},
			{tool.Baro_t("침해 인정"), tool.Baro_comma(done_count), ""},
			{"기각", tool.Baro_comma(reject_count), ""},
			{tool.Baro_t("격리 중"), tool.Baro_comma(quarantine_count), tool.Baro_t("복구 가능한 상태")},
		}),
		"",
	)

	body += tool.Baro_card(
		tool.Baro_t("처리 절차"),
		tool.Baro_list([]string{
			tool.Baro_t("<strong>접수</strong> — 권리자가 대상(문서 · 판 · 파일)과 권리 증빙, 연락처를 적어 신고합니다. 로그인 없이도 낼 수 있습니다."),
			tool.Baro_t("<strong>즉시 임시 격리</strong> — 접수되면 바로 지우지 않고 임시로 가립니다. 원본은 그대로 보존되어 언제든 되돌릴 수 있습니다. (원칙 2)"),
			tool.Baro_t("<strong>작성자 소명</strong> — 원 작성자가 반론을 낼 수 있습니다. 직접 만든 것이거나 인용 범위 안이라면 이 단계에서 밝힐 수 있습니다."),
			tool.Baro_t("<strong>결정</strong> — 담당자가 침해 인정 · 일부 인정 · 기각 중 하나로 결정하고, 격리를 유지할지 풀지 정합니다."),
			tool.Baro_t("<strong>기록</strong> — 처리자 · 처리 시각 · 결과가 남고, 공개 통계에는 건수만 반영됩니다."),
		}),
	)

	body += tool.Baro_alert(
		"warn",
		tool.Baro_t("허위 신고를 반복하면 새 접수가 막힙니다. 기각된 신고가 ")+tool.Baro_itoa(baro_legal_reject_limit(db))+tool.Baro_t("건을 넘으면 같은 신고자의 접수를 받지 않습니다."),
	)

	if tool.Baro_can(db, config.IP, "copyright") {
		body += tool.Baro_card_ex(
			tool.Baro_t("담당자 메뉴"),
			tool.Baro_t("개인정보·법적 요청 담당(privacy) 역할에게만 보입니다."),
			tool.Baro_stat_row([][]string{
				{tool.Baro_t("처리 대기"), tool.Baro_comma(open_count), ""},
				{tool.Baro_t("열린 격리"), tool.Baro_comma(quarantine_count), ""},
			}),
			tool.Baro_button("/baro/copyright/queue/open/1", tool.Baro_t("처리 큐"), "primary")+" "+
				tool.Baro_button("/baro/copyright/queue/done/1", tool.Baro_t("처리 기록"), ""),
		)
	}

	return baro_legal_page(db, config, tool.Baro_t("저작권 요청 센터"), "/baro/copyright", body)
}

// ------------------------------------------------------------
//  15-3 신고 접수 : /baro/copyright/new
// ------------------------------------------------------------

// View_baro_legal_copyright_new : 권리자 신고 접수 (비로그인 가능 / 요청 제한 있음)
func View_baro_legal_copyright_new(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	claimant := config.IP
	reject_count := baro_legal_copyright_reject_count(db, claimant)
	reject_limit := baro_legal_reject_limit(db)
	blocked := reject_count >= reject_limit

	message := ""
	target_kind := "doc"
	target := ""
	rev := ""
	contact := ""
	proof := ""

	if values != nil {
		target_kind = baro_legal_form_get(values, "target_kind")
		target = tool.Namespace_normalize(baro_legal_form_get(values, "target"))
		rev = baro_legal_form_get(values, "rev")
		contact = baro_legal_form_get(values, "contact")
		proof = baro_legal_form_raw(values, "proof")

		full_target := target
		if target_kind == "rev" && tool.Str_to_int(rev) > 0 {
			full_target = target + "#" + rev
		}

		switch {
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 신고를 접수할 수 없습니다."))
		case blocked:
			message = tool.Baro_alert("danger", tool.Baro_t("기각된 신고가 ")+tool.Baro_itoa(reject_count)+tool.Baro_t("건이라 새 신고를 접수할 수 없습니다. 담당자에게 문의해 주세요."))
		case !baro_legal_target_kind_ok(target_kind):
			message = tool.Baro_alert("danger", tool.Baro_t("신고 대상 종류를 골라 주세요."))
		case strings.TrimSpace(target) == "":
			message = tool.Baro_alert("danger", tool.Baro_t("대상 문서 이름을 적어 주세요."))
		case !baro_legal_copyright_doc_exists(db, target):
			message = tool.Baro_alert("danger", tool.Baro_t("그런 이름의 문서를 찾을 수 없습니다. 문서 제목을 주소창에 보이는 그대로 적어 주세요."))
		case target_kind == "rev" && tool.Str_to_int(rev) < 1:
			message = tool.Baro_alert("danger", tool.Baro_t("판(리비전) 번호를 적어 주세요."))
		case strings.TrimSpace(contact) == "":
			message = tool.Baro_alert("danger", tool.Baro_t("처리 결과를 알려 드릴 연락처가 필요합니다."))
		case tool.Get_len(strings.TrimSpace(proof)) < 10:
			message = tool.Baro_alert("danger", tool.Baro_t("권리 증빙을 열 글자 이상으로 적어 주세요. 원본 주소나 권리 관계를 알 수 있는 내용이면 됩니다."))
		case !tool.Baro_rate_ok(db, "copyright_new", config.IP, baro_legal_rate_limit(db), 3600):
			message = tool.Baro_alert("danger", tool.Baro_t("한 시간 안에 낼 수 있는 신고 수를 넘었습니다. 잠시 뒤에 다시 시도해 주세요."))
		default:
			id := tool.Baro_next_id(db, "baro_copyright_req")
			tool.Exec_DB(
				db,
				"insert into baro_copyright_req (id, claimant, contact, target_kind, target, proof, status, handler, date, close_date, result, counter) values (?, ?, ?, ?, ?, ?, 'open', '', ?, '', '', '')",
				id, claimant, tool.Baro_trunc(contact, 300), target_kind, tool.Baro_trunc(full_target, 500),
				tool.Baro_trunc(proof, 4000), tool.Baro_now(),
			)

			// 즉시 임시 격리 (원본은 backup 에 보존)
			quarantine_id := baro_legal_copyright_do_quarantine(db, id, target_kind, full_target, tool.Baro_t("저작권 신고 접수"))

			// 공개 기록에는 신고자와 연락처를 남기지 않는다
			tool.Baro_public_log(db, "copyright_req", "system", "", "저작권 신고 접수 후 임시 격리", baro_legal_target_kind_name(target_kind))
			tool.Baro_audit(
				db, config, "copyright_req_new", "baro_copyright_req/"+id,
				"", full_target+" / 격리 "+quarantine_id, tool.Baro_trunc(proof, 500),
			)
			tool.Baro_metric_bump(db, "copyright_req", tool.Baro_today(), 1)

			// 원 작성자에게 소명 기회를 알린다
			title, _ := baro_legal_copyright_target_split(full_target)
			for _, author := range baro_legal_copyright_authors(db, title, 5) {
				tool.Baro_notify(
					db, author, "system",
					tool.Baro_t("내가 기여한 문서에 저작권 신고가 접수되었습니다 — 소명할 수 있습니다"),
					"/baro/copyright/counter/"+id, "",
				)
			}

			return tool.Get_redirect("/baro/copyright/req/" + id)
		}
	}

	body := tool.Baro_crumb([][]string{
		{"/baro/copyright", tool.Baro_t("저작권 요청 센터")},
		{"", tool.Baro_t("신고 접수")},
	})

	body += message

	body += tool.Baro_hero(
		tool.Baro_t("저작권 침해 신고"),
		tool.Baro_t("접수되면 바로 지우지 않고 임시로 가립니다. 원본은 보존되어 되돌릴 수 있습니다."),
		"",
	)

	if reject_count > 0 {
		kind := "warn"
		if blocked {
			kind = "danger"
		}

		body += tool.Baro_alert(
			kind,
			tool.Baro_t("이 접수자 앞으로 기각된 신고가 <strong>")+tool.Baro_itoa(reject_count)+tool.Baro_t("건</strong> 있습니다. ")+
				tool.Baro_itoa(reject_limit)+tool.Baro_t("건을 넘으면 새 접수가 막힙니다."),
		)
	}

	if blocked {
		body += tool.Baro_card(
			tool.Baro_t("접수가 막혀 있습니다"),
			tool.Baro_t(`<p>기각된 신고가 너무 많아 새 신고를 받지 않습니다. 잘못된 판단이라고 생각하면 처리 큐 담당자에게 문의해 주세요.</p>`)+
				tool.Baro_button("/baro/copyright", tool.Baro_t("저작권 요청 센터로"), "primary"),
		)

		return baro_legal_page(db, config, tool.Baro_t("저작권 침해 신고"), "/baro/copyright", body)
	}

	if !tool.Baro_is_user(config.IP) {
		body += tool.Baro_alert("info", tool.Baro_t("로그인하지 않아도 신고할 수 있습니다. 다만 접수 뒤 진행 상황을 보려면 적어 주신 연락처로 안내를 받게 됩니다. 로그인하면 내 신고 목록에서 바로 확인할 수 있습니다."))
	}

	form := tool.Baro_select("target_kind", tool.Baro_t("신고 대상"), baro_legal_target_kind_list(), target_kind, tool.Baro_t("특정 판만 문제라면 '특정 판(리비전)' 을 고르고 판 번호를 적어 주세요."))
	form += tool.Baro_input("target", tool.Baro_t("대상 문서 이름"), target, tool.Baro_t("예) 저작권 침해가 있는 문서 제목"), tool.Baro_t("파일이면 파일 문서 이름을 그대로 적어 주세요."))
	form += tool.Baro_input_type("rev", tool.Baro_t("판(리비전) 번호"), rev, "number", tool.Baro_t("대상 종류가 '특정 판' 일 때만 씁니다."))
	form += tool.Baro_input("contact", tool.Baro_t("연락처"), contact, tool.Baro_t("예) 이메일 주소"), tool.Baro_t("처리 결과를 알려 드릴 때만 씁니다. 공개 화면에는 가려서 표시됩니다."))
	form += tool.Baro_textarea("proof", tool.Baro_t("권리 증빙"), proof, 7, tool.Baro_t("예) 원본 주소, 저작권자와의 관계, 최초 공개 시점 등"), tool.Baro_t("권리 관계를 알 수 있는 내용을 적어 주세요. 이 내용은 담당자만 봅니다."))
	form += tool.Baro_submit(tool.Baro_t("신고 접수"), "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("신고 내용"),
		tool.Baro_t("거짓 신고는 다른 사람의 기여를 부당하게 가립니다. 사실만 적어 주세요."),
		tool.Baro_form("/baro/copyright/new", form),
		tool.Baro_note(tool.Baro_t("한 시간에 ")+tool.Baro_itoa(baro_legal_rate_limit(db))+tool.Baro_t("건까지 접수할 수 있습니다.")),
	)

	return baro_legal_page(db, config, tool.Baro_t("저작권 침해 신고"), "/baro/copyright", body)
}

// baro_legal_copyright_authors : 대상 문서에 기여한 로그인 사용자 (소명 대상)
func baro_legal_copyright_authors(db *sql.DB, title string, limit int) []string {
	out := []string{}
	if strings.TrimSpace(title) == "" {
		return out
	}

	rows := tool.Query_DB(
		db,
		"select ip from history where title = ? order by id + 0 desc limit 100",
		title,
	)
	defer rows.Close()

	seen := map[string]bool{}
	for rows.Next() {
		who := ""
		if err := rows.Scan(&who); err != nil {
			continue
		}

		if !tool.Baro_is_user(who) || seen[who] {
			continue
		}

		seen[who] = true
		out = append(out, who)

		if len(out) >= limit {
			break
		}
	}

	return out
}

// baro_legal_copyright_is_author : 이 사용자가 대상 문서의 기여자인지
func baro_legal_copyright_is_author(db *sql.DB, title string, user string) bool {
	if !tool.Baro_is_user(user) || strings.TrimSpace(title) == "" {
		return false
	}

	found := ""
	tool.QueryRow_DB(db, "select id from history where title = ? and ip = ? limit 1", []any{&found}, title, user)

	return found != ""
}

// ------------------------------------------------------------
//  15-3 내 신고 : /baro/copyright/mine
// ------------------------------------------------------------

// View_baro_legal_copyright_mine : 내가 낸 저작권 요청
func View_baro_legal_copyright_mine(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/copyright/mine"); need {
		return wall
	}

	page_num := baro_legal_page_num(page)
	size := tool.Baro_page_size
	list := baro_legal_copyright_list(db, "claimant = ?", size, (page_num-1)*size, config.IP)

	table_rows := [][]string{}
	for _, row := range list {
		counter := tool.Baro_badge("없음", "muted")
		if strings.TrimSpace(row.Counter) != "" {
			counter = tool.Baro_badge(tool.Baro_t("소명 있음"), "warn")
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/copyright/req/` + tool.Url_parser(row.ID) + `">#` + tool.HTML_escape(row.ID) + `</a>`,
			tool.HTML_escape(baro_legal_target_kind_name(row.TargetKind)),
			baro_legal_target_cell(row.TargetKind, row.Target),
			baro_legal_status_badge(row.Status),
			counter,
			baro_legal_result_cell(row.Result),
			tool.Baro_time_cell(row.Date),
		})
	}

	list_html := tool.Baro_table(
		[]string{"번호", "종류", tool.Baro_t("대상"), "상태", tool.Baro_t("소명"), tool.Baro_t("결과"), "접수"},
		table_rows,
		tool.Baro_t("낸 신고가 없습니다."),
	)

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("아직 접수한 저작권 신고가 없습니다."))
	}

	list_html += tool.Baro_paging(page_num, len(list), size, "/baro/copyright/mine/{}")

	reject_count := baro_legal_copyright_reject_count(db, config.IP)

	body := tool.Baro_crumb([][]string{
		{"/baro/copyright", tool.Baro_t("저작권 요청 센터")},
		{"", tool.Baro_t("내가 낸 신고")},
	})

	body += tool.Baro_hero(
		tool.Baro_t("내가 낸 저작권 신고"),
		tool.Baro_t("처리 상태와 결과를 확인할 수 있습니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("전체"), tool.Baro_comma(baro_legal_count(db, "select count(*) from baro_copyright_req where claimant = ?", config.IP)), ""},
			{"기각", tool.Baro_comma(reject_count), tool.Baro_t("누적 ") + tool.Baro_itoa(baro_legal_reject_limit(db)) + tool.Baro_t("건이면 접수가 막힙니다")},
		}),
	)

	if reject_count > 0 {
		body += tool.Baro_alert("warn", tool.Baro_t("기각된 신고가 ")+tool.Baro_itoa(reject_count)+tool.Baro_t("건 있습니다. 근거가 분명한 신고만 내 주세요."))
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("신고 목록"),
		"",
		list_html,
		tool.Baro_button("/baro/copyright/new", tool.Baro_t("새 신고 접수"), "primary"),
	)

	return baro_legal_page(db, config, tool.Baro_t("내가 낸 저작권 신고"), "/baro/copyright", body)
}

// ------------------------------------------------------------
//  15-3 처리 큐 : /baro/copyright/queue
// ------------------------------------------------------------

// View_baro_legal_copyright_queue : 저작권 요청 처리 큐 (권한 copyright)
func View_baro_legal_copyright_queue(config tool.Config, mode string, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "copyright") {
		return tool.Baro_deny(db, config, tool.Baro_t("저작권 처리 큐는 개인정보·법적 요청 담당(privacy) 역할만 볼 수 있습니다."))
	}

	if mode != "done" {
		mode = "open"
	}

	message := ""

	// 반복 악용 방지 설정 저장
	if values != nil {
		if baro_legal_form_get(values, "action") == "setting" {
			reject_limit := tool.Str_to_int(baro_legal_form_get(values, "reject_limit"))
			rate_limit := tool.Str_to_int(baro_legal_form_get(values, "rate_limit"))

			if reject_limit < 1 || rate_limit < 1 {
				message = tool.Baro_alert("danger", tool.Baro_t("설정 값은 1 이상이어야 합니다."))
			} else {
				before := tool.Baro_itoa(baro_legal_reject_limit(db)) + " / " + tool.Baro_itoa(baro_legal_rate_limit(db))

				tool.Baro_put(db, "copyright_reject_limit", tool.Baro_itoa(reject_limit))
				tool.Baro_put(db, "copyright_rate_limit", tool.Baro_itoa(rate_limit))

				tool.Baro_audit(
					db, config, "copyright_setting", "baro_set",
					before, tool.Baro_itoa(reject_limit)+" / "+tool.Baro_itoa(rate_limit),
					"저작권 접수 제한 설정 변경",
				)
				tool.Baro_public_log(db, "copyright_setting", config.IP, "저작권 접수 제한", "설정 변경", "기각 한도 "+tool.Baro_itoa(reject_limit)+"건 / 시간당 "+tool.Baro_itoa(rate_limit)+"건")

				return tool.Get_redirect("/baro/copyright/queue/" + mode + "/1")
			}
		}
	}

	page_num := baro_legal_page_num(page)
	size := tool.Baro_page_size

	where := baro_legal_open_where
	if mode == "done" {
		where = baro_legal_closed_where
	}

	list := baro_legal_copyright_list(db, where, size, (page_num-1)*size)

	table_rows := [][]string{}
	for _, row := range list {
		title, _ := baro_legal_copyright_target_split(row.Target)

		quarantine := tool.Baro_badge(tool.Baro_t("격리 없음"), "muted")
		if baro_legal_copyright_quarantine_open(db, title) != "" {
			quarantine = tool.Baro_badge(tool.Baro_t("격리 중"), "danger")
		}

		counter := tool.Baro_badge("없음", "muted")
		if strings.TrimSpace(row.Counter) != "" {
			counter = tool.Baro_badge(tool.Baro_t("소명 있음"), "warn")
		}

		abuse := ""
		if count := baro_legal_copyright_reject_count(db, row.Claimant); count > 0 {
			abuse = " " + tool.Baro_badge("기각 "+tool.Baro_itoa(count)+tool.Baro_t("건"), "danger")
		}

		handler := `<span class="bw_muted">-</span>`
		if row.Handler != "" {
			handler = baro_legal_who_cell(db, row.Handler, config.IP)
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/copyright/req/` + tool.Url_parser(row.ID) + `">#` + tool.HTML_escape(row.ID) + `</a>`,
			baro_legal_who_cell(db, row.Claimant, config.IP) + abuse,
			tool.HTML_escape(baro_legal_target_kind_name(row.TargetKind)),
			baro_legal_target_cell(row.TargetKind, row.Target),
			quarantine,
			counter,
			baro_legal_status_badge(row.Status),
			handler,
			tool.Baro_time_cell(row.Date),
		})
	}

	list_html := tool.Baro_table(
		[]string{"번호", "신고자", "종류", tool.Baro_t("대상"), "격리", tool.Baro_t("소명"), "상태", tool.Baro_t("처리자"), "접수"},
		table_rows,
		tool.Baro_t("처리할 신고가 없습니다."),
	)

	if len(table_rows) == 0 {
		if mode == "done" {
			list_html = tool.Baro_empty(tool.Baro_t("아직 처리를 끝낸 저작권 신고가 없습니다."))
		} else {
			list_html = tool.Baro_empty(tool.Baro_t("처리를 기다리는 저작권 신고가 없습니다."))
		}
	}

	list_html += tool.Baro_paging(page_num, len(list), size, "/baro/copyright/queue/"+mode+"/{}")

	open_count := baro_legal_count(db, "select count(*) from baro_copyright_req where "+baro_legal_open_where)
	done_count := baro_legal_count(db, "select count(*) from baro_copyright_req where "+baro_legal_closed_where)
	quarantine_count := baro_legal_count(db, "select count(*) from baro_quarantine where kind = 'copyright' and status = 'open'")
	counter_count := baro_legal_count(db, "select count(*) from baro_copyright_req where counter != '' and "+baro_legal_open_where)

	body := tool.Baro_crumb([][]string{
		{"/baro/copyright", tool.Baro_t("저작권 요청 센터")},
		{"", tool.Baro_t("처리 큐")},
	})

	body += message

	body += tool.Baro_hero(
		tool.Baro_t("저작권 요청 처리 큐"),
		tool.Baro_t("신고 원문과 연락처는 이 화면 밖으로 옮기지 마세요."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("처리 대기"), tool.Baro_comma(open_count), ""},
			{tool.Baro_t("소명 접수"), tool.Baro_comma(counter_count), tool.Baro_t("반론이 들어온 건")},
			{tool.Baro_t("열린 격리"), tool.Baro_comma(quarantine_count), ""},
			{"처리 완료", tool.Baro_comma(done_count), ""},
		}),
	)

	body += tool.Baro_tabs([][]string{
		{"/baro/copyright/queue/open/1", tool.Baro_t("처리 대기")},
		{"/baro/copyright/queue/done/1", tool.Baro_t("처리 기록")},
	}, "/baro/copyright/queue/"+mode+"/1")

	body += tool.Baro_card_ex(tool.Baro_t("신고 목록"), tool.Baro_t("번호를 누르면 처리 화면으로 들어갑니다."), list_html, "")

	// 반복 악용 방지 설정
	setting_form := tool.Baro_hidden("action", "setting")
	setting_form += tool.Baro_input_type("reject_limit", tool.Baro_t("새 접수를 막는 기각 누적 횟수"), tool.Baro_itoa(baro_legal_reject_limit(db)), "number", tool.Baro_t("같은 신고자의 기각 건수가 이 값에 이르면 새 접수를 받지 않습니다."))
	setting_form += tool.Baro_input_type("rate_limit", tool.Baro_t("시간당 접수 가능 건수"), tool.Baro_itoa(baro_legal_rate_limit(db)), "number", tool.Baro_t("한 시간 안에 같은 접수자가 낼 수 있는 신고 수입니다."))
	setting_form += tool.Baro_submit(tool.Baro_t("설정 저장"), "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("반복 악용 방지 설정"),
		tool.Baro_t("baro_set 에 저장되며 바꾼 사실은 공개 로그에 남습니다."),
		tool.Baro_form("/baro/copyright/queue/"+mode+"/"+tool.Baro_itoa(page_num), setting_form),
		"",
	)

	return baro_legal_page(db, config, tool.Baro_t("저작권 처리 큐"), "/baro/copyright", body)
}

// ------------------------------------------------------------
//  15-3 신고 상세 / 처리 : /baro/copyright/req/<id>
// ------------------------------------------------------------

// View_baro_legal_copyright_req : 저작권 요청 상세와 처리
func View_baro_legal_copyright_req(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row := baro_legal_copyright_get(db, id)
	if row.ID == "" {
		return baro_legal_not_found(db, config, "/baro/copyright", "그런 번호의 저작권 요청이 없습니다.", "/baro/copyright", "저작권 요청 센터로")
	}

	title, rev := baro_legal_copyright_target_split(row.Target)

	is_claimant := config.IP == row.Claimant
	is_handler := tool.Baro_can(db, config.IP, "copyright")
	is_author := baro_legal_copyright_is_author(db, title, config.IP)

	message := ""

	// 이해충돌 배제 : 자기가 낸 신고는 자기가 결정할 수 없다
	can_decide := is_handler && !is_claimant

	if values != nil && can_decide {
		action := baro_legal_form_get(values, "action")

		switch {
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", "지금은 읽기 전용 상태라 처리할 수 없습니다.")
		case action == "handle":
			status := baro_legal_form_get(values, "status")
			code := baro_legal_form_get(values, "result_code")
			note := baro_legal_form_raw(values, "result_note")
			quarantine := baro_legal_form_get(values, "quarantine")

			if !baro_legal_status_ok(status) {
				message = tool.Baro_alert("danger", "처리 상태를 골라 주세요.")
				break
			}

			if (status == "done" || status == "rejected") && strings.TrimSpace(note) == "" {
				message = tool.Baro_alert("danger", "처리를 끝낼 때는 결정 근거를 반드시 적어야 합니다.")
				break
			}

			if status == "done" && code == "" {
				message = tool.Baro_alert("danger", "처리 완료로 바꿀 때는 인정 · 일부 인정 · 거부 중 하나를 골라야 합니다.")
				break
			}

			// 격리를 어떻게 할지 정하지 않은 채로 신고를 닫으면 문서가 가려진 채 남는다 (원칙 2)
			quarantine_id := baro_legal_copyright_quarantine_for(db, row.ID)
			doc_quarantined := title != "" && baro_legal_copyright_quarantine_open(db, title) != ""
			need_choice := doc_quarantined ||
				baro_legal_copyright_quarantine_status(db, quarantine_id) == "open" ||
				code == "approved" || code == "partial"

			if (status == "done" || status == "rejected") && quarantine == "" && need_choice {
				message = tool.Baro_alert("danger", "격리를 해제할지 유지할지 반드시 골라야 합니다. 정하지 않고 신고를 닫으면 문서가 가려진 채로 남습니다.")
				break
			}

			close_date := ""
			if status == "done" || status == "rejected" || status == "closed" {
				close_date = tool.Baro_now()
			}

			tool.Exec_DB(
				db,
				"update baro_copyright_req set status = ?, handler = ?, close_date = ?, result = ? where id = ?",
				status, config.IP, close_date, baro_legal_result_enc(code, tool.Baro_trunc(note, 2000)), row.ID,
			)

			// 격리 유지 / 해제 결정
			switch quarantine {
			case "release":
				if quarantine_id != "" {
					tool.Exec_DB(
						db,
						"update baro_quarantine set status = 'closed', release_date = ? where id = ?",
						tool.Baro_now(), quarantine_id,
					)
				}

				// 이 문서에 다른 격리가 남아 있지 않으면 상태를 되돌린다
				if title != "" && baro_legal_copyright_quarantine_open(db, title) == "" {
					tool.Baro_doc_status_set(db, title, "", "저작권 신고 종결 (요청 #"+row.ID+")", "system")
				}

				tool.Baro_public_log(db, "copyright_release", "system", "", "저작권 신고 종결 후 격리 해제", baro_legal_result_name(code))
			case "keep":
				if quarantine_id == "" {
					// 접수 때 격리 기록이 만들어지지 않은 신고라면 지금 만든다.
					// 기록 없이 문서 상태만 바꾸면 원본 보존본이 없어 되돌릴 수 없다. (원칙 2)
					quarantine_id = baro_legal_copyright_do_quarantine(db, row.ID, row.TargetKind, row.Target, "저작권 침해 인정 후 격리")
				} else {
					tool.Exec_DB(db, "update baro_quarantine set status = 'open', release_date = '' where id = ?", quarantine_id)
				}

				if title != "" {
					tool.Baro_doc_status_set(db, title, "quarantined", "저작권 침해 인정 (요청 #"+row.ID+")", "system")
				}

				tool.Baro_public_log(db, "copyright_keep", "system", "", "저작권 신고 인정 후 격리 유지", baro_legal_result_name(code))
			}

			tool.Baro_audit(
				db, config, "copyright_req_handle", "baro_copyright_req/"+row.ID,
				row.Status+" / "+baro_legal_result_code(row.Result),
				status+" / "+code+" / 격리 "+quarantine,
				tool.Baro_trunc(note, 500),
			)
			tool.Baro_metric_bump(db, "copyright_req_handle", tool.Baro_today(), 1)

			if tool.Baro_is_user(row.Claimant) {
				tool.Baro_notify(
					db, row.Claimant, "system",
					"저작권 신고가 "+baro_legal_status_name(status)+" 상태로 바뀌었습니다",
					"/baro/copyright/req/"+row.ID, "",
				)
			}

			for _, author := range baro_legal_copyright_authors(db, title, 5) {
				tool.Baro_notify(
					db, author, "system",
					"내가 기여한 문서의 저작권 신고 처리 결과가 나왔습니다",
					"/baro/copyright/req/"+row.ID, "",
				)
			}

			return tool.Get_redirect("/baro/copyright/req/" + row.ID)
		default:
			message = tool.Baro_alert("danger", "알 수 없는 동작입니다.")
		}
	} else if values != nil {
		if is_handler && is_claimant {
			message = tool.Baro_alert("danger", "자기가 낸 신고는 자기가 결정할 수 없습니다. 다른 담당자에게 넘겨 주세요.")
		} else {
			message = tool.Baro_alert("danger", "이 동작을 할 권한이 없습니다.")
		}
	}

	row = baro_legal_copyright_get(db, id)

	quarantine_id := baro_legal_copyright_quarantine_for(db, row.ID)
	quarantine_status := baro_legal_copyright_quarantine_status(db, quarantine_id)

	quarantine_badge := tool.Baro_badge("격리 기록 없음", "muted")
	switch quarantine_status {
	case "open":
		quarantine_badge = tool.Baro_badge("격리 중 (원본 보존됨)", "danger")
	case "closed":
		quarantine_badge = tool.Baro_badge("격리 해제됨", "ok")
	}

	// 이 신고가 만든 격리가 없어도 대상 문서 자체가 격리 중일 수 있다
	if quarantine_status == "" && title != "" && baro_legal_copyright_quarantine_open(db, title) != "" {
		quarantine_badge = tool.Baro_badge("이 신고의 격리 기록은 없지만 대상 문서가 격리 중", "warn")
	}

	body := tool.Baro_crumb([][]string{
		{"/baro/copyright", "저작권 요청 센터"},
		{"", "신고 #" + row.ID},
	})

	body += message

	rev_html := tool.Baro_t(`<span class="bw_muted">전체</span>`)
	if rev != "" {
		rev_html = tool.HTML_escape(rev) + tool.Baro_t("판")
	}

	// 신고자와 담당자에게만 연락처 원문을 보여 준다
	contact_html := baro_legal_contact_mask(row.Contact)
	if is_handler || is_claimant {
		contact_html = tool.HTML_escape(row.Contact)
	}

	body += tool.Baro_hero(
		"저작권 신고 #"+row.ID,
		"접수 "+row.Date,
		tool.Baro_kv_grid([][]string{
			{"대상 종류", tool.HTML_escape(baro_legal_target_kind_name(row.TargetKind))},
			{"대상", baro_legal_target_cell(row.TargetKind, title)},
			{"판", rev_html},
			{"신고자", baro_legal_who_cell(db, row.Claimant, config.IP)},
			{"연락처", contact_html},
			{"상태", baro_legal_status_badge(row.Status)},
			{"격리", quarantine_badge},
			{"처리 결과", baro_legal_result_cell(row.Result)},
			{"처리 시각", tool.Baro_time_cell(row.CloseDate)},
		}),
	)

	body += tool.Baro_alert(
		"info",
		"저작권 신고가 들어오면 문서를 지우는 대신 임시로 가립니다. 원본은 격리 기록의 backup 에 그대로 남아 있어 결정이 뒤집히면 되돌릴 수 있습니다. (원칙 2)",
	)

	// 권리 증빙은 담당자와 신고자에게만
	if is_handler || is_claimant {
		body += tool.Baro_card_ex("권리 증빙", "담당자와 신고자만 볼 수 있습니다.", baro_legal_quote_html(row.Proof), "")
	} else {
		body += tool.Baro_card("권리 증빙", tool.Baro_note("신고 원문은 담당자와 신고자만 볼 수 있습니다. (원칙 5)"))
	}

	// 소명
	counter_foot := ""
	if is_author || is_handler {
		counter_foot = tool.Baro_button("/baro/copyright/counter/"+tool.Url_parser(row.ID), "작성자 소명 쓰기", "primary")
	}

	counter_body := tool.Baro_empty("아직 원 작성자의 소명이 없습니다.")
	if strings.TrimSpace(row.Counter) != "" {
		counter_body = baro_legal_quote_html(row.Counter)
	}

	body += tool.Baro_card_ex(
		"작성자 소명",
		"원 작성자는 직접 만든 것이거나 인용 범위 안이라는 점을 여기에 밝힐 수 있습니다.",
		counter_body,
		counter_foot,
	)

	if note := baro_legal_result_note(row.Result); note != "" {
		body += tool.Baro_card("처리 설명", baro_legal_quote_html(note))
	}

	// 담당자 처리
	if is_handler && is_claimant {
		body += tool.Baro_card_ex(
			"담당자 결정",
			"이해충돌 때문에 이 신고는 직접 결정할 수 없습니다.",
			tool.Baro_alert("warn", "이 신고는 지금 로그인한 계정이 낸 것입니다. 자기가 낸 신고를 자기가 결정하면 판단이 공정하지 않으므로, 다른 담당자가 처리해야 합니다."),
			"",
		)
	}

	if can_decide {
		status_options := [][]string{}
		for _, status_row := range baro_legal_status_list() {
			status_options = append(status_options, []string{status_row[0], status_row[1]})
		}

		current_status := row.Status
		if current_status == "" {
			current_status = "open"
		}

		result_options := [][]string{
			{"", "결정 없음"},
			{"approved", "침해 인정"},
			{"partial", "일부 인정"},
			{"rejected", "기각 (침해 아님)"},
		}

		quarantine_options := [][]string{
			{"", "아직 정하지 않음"},
			{"release", "격리 해제하고 문서 되살리기"},
			{"keep", "격리 유지 (기록이 없으면 지금 격리하고 원본을 보존)"},
		}

		form := tool.Baro_hidden("action", "handle")
		form += tool.Baro_select("status", "처리 상태", status_options, current_status, "")
		form += tool.Baro_select("result_code", "결정", result_options, baro_legal_result_code(row.Result), "처리 완료로 바꿀 때는 반드시 골라야 합니다.")
		form += tool.Baro_select("quarantine", "격리 처리", quarantine_options, "", "기각이면 격리를 풀어 문서를 되살립니다. 신고를 완료 · 기각으로 닫을 때는 반드시 골라야 합니다.")
		form += tool.Baro_textarea("result_note", "결정 근거", baro_legal_result_note(row.Result), 6, "예) 제출된 원본 주소와 문서 내용이 같아 침해로 인정합니다.", "신고자와 원 작성자 모두에게 보입니다. 개인정보를 적지 마세요.")
		form += tool.Baro_submit("결정 저장", "primary")

		warn := ""
		if strings.TrimSpace(row.Counter) == "" {
			warn = tool.Baro_alert("warn", "아직 원 작성자의 소명이 없습니다. 되도록 소명 기회를 준 뒤에 결정해 주세요.")
		}

		abuse := baro_legal_copyright_reject_count(db, row.Claimant)
		if abuse > 0 {
			warn += tool.Baro_alert("danger", tool.Baro_t("이 신고자는 기각된 신고가 <strong>")+tool.Baro_itoa(abuse)+tool.Baro_t("건</strong> 있습니다. 반복 악용인지 함께 살펴 주세요."))
		}

		body += tool.Baro_card_ex(
			"담당자 결정",
			"결정과 격리 처리를 함께 저장합니다.",
			warn+tool.Baro_form("/baro/copyright/req/"+tool.Url_parser(row.ID), form),
			tool.Baro_note("이 화면에는 사용자 제재 단추를 두지 않습니다. 콘텐츠 판단과 이용자 제재는 분리합니다. (원칙 1)"),
		)
	}

	return baro_legal_page(db, config, "저작권 신고 #"+row.ID, "/baro/copyright", body)
}

// ------------------------------------------------------------
//  15-3 작성자 소명 : /baro/copyright/counter/<id>
// ------------------------------------------------------------

// View_baro_legal_copyright_counter : 원 작성자의 반론 접수
func View_baro_legal_copyright_counter(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row := baro_legal_copyright_get(db, id)
	if row.ID == "" {
		return baro_legal_not_found(db, config, "/baro/copyright", tool.Baro_t("그런 번호의 저작권 요청이 없습니다."), "/baro/copyright", tool.Baro_t("저작권 요청 센터로"))
	}

	if wall, need := tool.Baro_login_wall(db, config, "/baro/copyright/counter/"+row.ID); need {
		return wall
	}

	title, _ := baro_legal_copyright_target_split(row.Target)
	is_author := baro_legal_copyright_is_author(db, title, config.IP)
	is_handler := tool.Baro_can(db, config.IP, "copyright")

	if !is_author && !is_handler {
		return tool.Baro_deny(db, config, tool.Baro_t("소명은 대상 문서에 기여한 사람과 담당자만 쓸 수 있습니다. 대상 문서 편집 이력에 계정이 없으면 쓸 수 없습니다."))
	}

	message := ""
	counter := ""

	if values != nil {
		counter = baro_legal_form_raw(values, "counter")

		switch {
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 소명을 접수할 수 없습니다."))
		case !baro_legal_status_open(row.Status):
			message = tool.Baro_alert("danger", tool.Baro_t("이미 처리가 끝난 신고에는 소명을 낼 수 없습니다. 결정에 이의가 있으면 담당자에게 문의해 주세요."))
		case tool.Get_len(strings.TrimSpace(counter)) < 10:
			message = tool.Baro_alert("danger", tool.Baro_t("소명 내용을 열 글자 이상으로 적어 주세요."))
		case !tool.Baro_rate_ok(db, "copyright_counter", config.IP, 10, 3600):
			message = tool.Baro_alert("danger", tool.Baro_t("한 시간 안에 낼 수 있는 소명 수를 넘었습니다."))
		default:
			block := "[" + tool.Baro_now() + "] " + config.IP + "\n" + strings.TrimSpace(counter)

			merged := block
			if strings.TrimSpace(row.Counter) != "" {
				merged = row.Counter + "\n\n----\n\n" + block
			}

			tool.Exec_DB(db, "update baro_copyright_req set counter = ? where id = ?", tool.Baro_trunc(merged, 8000), row.ID)

			tool.Baro_public_log(db, "copyright_counter", "system", "", "저작권 신고에 작성자 소명 접수", "")
			tool.Baro_audit(db, config, "copyright_counter", "baro_copyright_req/"+row.ID, "", "counter", tool.Baro_trunc(counter, 500))
			tool.Baro_metric_bump(db, "copyright_counter", tool.Baro_today(), 1)

			if tool.Baro_is_user(row.Claimant) {
				tool.Baro_notify(
					db, row.Claimant, "system",
					tool.Baro_t("내가 낸 저작권 신고에 작성자 소명이 들어왔습니다"),
					"/baro/copyright/req/"+row.ID, "",
				)
			}

			return tool.Get_redirect("/baro/copyright/req/" + row.ID)
		}
	}

	body := tool.Baro_crumb([][]string{
		{"/baro/copyright", tool.Baro_t("저작권 요청 센터")},
		{"/baro/copyright/req/" + row.ID, "신고 #" + row.ID},
		{"", tool.Baro_t("작성자 소명")},
	})

	body += message

	body += tool.Baro_hero(
		tool.Baro_t("작성자 소명"),
		"신고 #"+row.ID+tool.Baro_t(" 에 대한 반론을 남깁니다."),
		tool.Baro_kv_grid([][]string{
			{tool.Baro_t("대상"), baro_legal_target_cell(row.TargetKind, title)},
			{tool.Baro_t("신고 상태"), baro_legal_status_badge(row.Status)},
			{"접수", tool.Baro_time_cell(row.Date)},
		}),
	)

	body += tool.Baro_card(
		tool.Baro_t("이런 내용을 적으면 도움이 됩니다"),
		tool.Baro_list([]string{
			tool.Baro_t("직접 쓴 글이라면 언제 어떻게 작성했는지"),
			tool.Baro_t("인용이라면 인용한 범위와 출처를 어디에 밝혔는지"),
			tool.Baro_t("공개된 자료를 옮겨 왔다면 그 자료의 라이선스가 무엇인지"),
			tool.Baro_t("신고 대상이 잘못 지정되었다면 어느 부분이 다른지"),
		}),
	)

	if strings.TrimSpace(row.Counter) != "" {
		body += tool.Baro_card(tool.Baro_t("지금까지 들어온 소명"), baro_legal_quote_html(row.Counter))
	}

	if !baro_legal_status_open(row.Status) {
		body += tool.Baro_alert("warn", tool.Baro_t("이 신고는 이미 처리가 끝나 새 소명을 받지 않습니다."))

		return baro_legal_page(db, config, tool.Baro_t("작성자 소명"), "/baro/copyright", body)
	}

	form := tool.Baro_textarea("counter", tool.Baro_t("소명 내용"), counter, 8, tool.Baro_t("예) 이 문단은 제가 직접 조사해서 쓴 것이며, 인용한 두 문장은 출처를 각주로 밝혔습니다."), tool.Baro_t("담당자와 신고자가 함께 봅니다."))
	form += tool.Baro_submit(tool.Baro_t("소명 제출"), "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("소명 쓰기"),
		tool.Baro_t("소명이 들어오면 신고자에게도 알림이 갑니다."),
		tool.Baro_form("/baro/copyright/counter/"+tool.Url_parser(row.ID), form),
		"",
	)

	return baro_legal_page(db, config, tool.Baro_t("작성자 소명"), "/baro/copyright", body)
}
