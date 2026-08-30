package route

import (
	"database/sql"
	"net/url"
	"strings"
	"time"

	"opennamu/route/tool"
)

// ============================================================
//  17-3 내부 감사 로그 — /baro/security/audit
//
//  누가 / 언제 / 무엇을 / 어떤 값에서 어떤 값으로 / 어떤 권한으로 / 왜.
//
//  이 화면에는 수정·삭제 기능이 없다. 일부러 만들지 않는다.
//  대신 sign_data 해시로 위변조 여부를 검증한다.
// ============================================================

// baro_sec_audit_style : 검증 실패 줄을 붉게 보이게 하는 최소한의 덧칠
const baro_sec_audit_style = `<style id="bw_sec_audit_css">
.bw_sec_bad{display:block;background:rgba(220,67,67,.14);color:#b12b2b;padding:.15rem .3rem;border-radius:4px;}
.bw_sec_diff{font-size:.86rem;word-break:break-all;}
</style>`

// baro_sec_audit_row : 감사 로그 한 줄
type baro_sec_audit_row struct {
	ID     string
	Actor  string
	Action string
	Target string
	Before string
	After  string
	Auth   string
	Reason string
	IP     string
	UA     string
	Date   string
	Sign   string
}

// baro_sec_audit_truncated : 이전·이후 값이 2000자로 잘려 저장된 줄인지
//
//	잘린 값은 원본과 구분되지 않으므로 여기서는 보수적으로 "검증 불가" 로 둔다.
func baro_sec_audit_truncated(data string) bool {
	return strings.HasSuffix(data, "…") && tool.Get_len(data) >= 2000
}

// baro_sec_audit_check : 위변조 검증 결과 ("ok" / "fail" / "unknown")
//
//	지금 계산식은 tool.Baro_audit_sign 이며
//	담당자 · 동작 · 대상 · 바뀐 값에 더해 사유 · 접속 정보 · 권한까지 덮는다.
//	계산식이 넓어지기 전에 쌓인 줄은 옛 계산식으로도 확인해 준다.
func baro_sec_audit_check(row baro_sec_audit_row) string {
	if strings.TrimSpace(row.Sign) == "" {
		return "unknown"
	}

	if baro_sec_audit_truncated(row.Before) || baro_sec_audit_truncated(row.After) {
		return "unknown"
	}

	// 지금 계산식 : 사유 · 접속 정보 · 권한까지 전부 덮는다
	if tool.Baro_audit_sign(row.ID, row.Actor, row.Action, row.Target,
		row.Before, row.After, row.Auth, row.Reason, row.IP, row.UA, row.Date) == row.Sign {
		return "ok"
	}

	// 계산식이 넓어지기 전에 쌓인 줄은 옛 식으로도 인정한다
	base := row.ID + row.IP + row.Action + row.Target + row.Before + row.After

	stamp := tool.Baro_parse_time(row.Date)
	if stamp.IsZero() {
		return "unknown"
	}

	for back := 0; back <= 2; back++ {
		moment := stamp.Add(-time.Duration(back) * time.Second).Format("2006-01-02 15:04:05")
		if tool.Sha224(base+moment) == row.Sign {
			return "ok"
		}
	}

	return "fail"
}

// baro_sec_audit_badge : 검증 결과 배지
func baro_sec_audit_badge(result string) string {
	switch result {
	case "ok":
		return tool.Baro_badge(tool.Baro_t("검증됨"), "ok")
	case "fail":
		return tool.Baro_badge(tool.Baro_t("위변조 의심"), "danger")
	}

	return tool.Baro_badge(tool.Baro_t("검증 불가"), "muted")
}

// baro_sec_audit_query : 조건에 맞는 감사 로그 읽기
func baro_sec_audit_query(db *sql.DB, actor string, action string, from string, to string, offset int, limit int) []baro_sec_audit_row {
	out := []baro_sec_audit_row{}

	query := "select id, actor, action, target, before_data, after_data, auth, reason, ip, ua, date, sign_data from baro_audit_log where 1 = 1"
	args := []any{}

	if actor != "" {
		query += " and actor = ?"
		args = append(args, actor)
	}

	if action != "" {
		query += " and action like ?"
		args = append(args, "%"+action+"%")
	}

	if from != "" {
		query += " and date >= ?"
		args = append(args, from)
	}

	if to != "" {
		query += " and date <= ?"
		args = append(args, to+" 23:59:59")
	}

	query += " order by id + 0 desc limit ? offset ?"
	args = append(args, limit, offset)

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	for rows.Next() {
		row := baro_sec_audit_row{}
		if err := rows.Scan(
			&row.ID, &row.Actor, &row.Action, &row.Target,
			&row.Before, &row.After, &row.Auth, &row.Reason,
			&row.IP, &row.UA, &row.Date, &row.Sign,
		); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_sec_audit_count : 감사 로그 총 줄 수
func baro_sec_audit_count(db *sql.DB) int {
	count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_audit_log", []any{&count})

	return count
}

// baro_sec_audit_cell : 검증 실패면 붉게 칠한 칸
func baro_sec_audit_cell(html string, bad bool) string {
	if bad {
		return `<span class="bw_sec_bad">` + html + `</span>`
	}

	return html
}

// baro_sec_audit_gate : 감사 로그 열람 권한 확인 (권한이 없으면 안내 화면)
func baro_sec_audit_gate(db *sql.DB, config tool.Config) (string, bool) {
	if wall, need := tool.Baro_login_wall(db, config, "/baro/security/audit"); need {
		return wall, false
	}

	if !tool.Baro_can(db, config.IP, "system") && !tool.Baro_is_owner(db, config.IP) {
		return tool.Baro_deny(db, config,
			tool.Baro_t("감사 로그는 system 권한(sysadmin 역할)이나 소유자만 볼 수 있습니다.")), false
	}

	return "", true
}

// baro_sec_audit_note_view : 열람 사실 자체를 감사 로그에 남긴다
func baro_sec_audit_note_view(db *sql.DB, config tool.Config, detail string) {
	// 쪽을 넘길 때마다 무한히 쌓이지 않도록 한 시간에 30줄까지만 남긴다
	if !tool.Baro_rate_ok(db, "sec_audit_view", config.IP, 30, 3600) {
		return
	}

	tool.Baro_audit(db, config, "audit_view", detail, "", "", "감사 로그 열람")
}

// ------------------------------------------------------------
//  /baro/security/audit : 감사 로그 목록
// ------------------------------------------------------------

// View_baro_sec_audit : 감사 로그 화면
func View_baro_sec_audit(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if page, ok := baro_sec_audit_gate(db, config); !ok {
		return page
	}

	if values != nil {
		// 감사 로그에는 쓰기 기능을 두지 않는다
		return tool.Get_redirect("/baro/security/audit")
	}

	actor := baro_sec_q(query, "actor")
	action := baro_sec_q(query, "action")
	from := baro_sec_q(query, "from")
	to := baro_sec_q(query, "to")
	page := baro_sec_page_num(query)

	baro_sec_audit_note_view(db, config,
		tool.Baro_t("목록 (담당자=")+actor+tool.Baro_t(" 동작=")+action+tool.Baro_t(" 기간=")+from+"~"+to+tool.Baro_t(" 쪽=")+tool.Baro_itoa(page)+")")

	rows := baro_sec_audit_query(db, actor, action, from, to, (page-1)*tool.Baro_page_size, tool.Baro_page_size)

	body := baro_sec_audit_style
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"", tool.Baro_t("감사 로그")}})
	body += tool.Baro_hero(tool.Baro_t("내부 감사 로그"), tool.Baro_t("공개 로그보다 자세한 기록입니다. 운영자도 고칠 수 없습니다."), "")
	body += baro_sec_tabs("/baro/security/audit")

	body += tool.Baro_alert("info",
		tool.Baro_t("이 화면에는 <b>수정·삭제 기능이 없습니다.</b> 일부러 만들지 않았습니다. ")+
			tool.Baro_t("각 줄은 <code>sign_data</code> 해시로 검증합니다. 검증에 실패한 줄은 붉게 표시됩니다.<br>")+
			tool.Baro_t("열람 사실도 감사 로그에 남습니다. (같은 사람은 한 시간에 30줄까지 기록)"),
	)

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 기록"), tool.Baro_comma(baro_sec_audit_count(db)), "baro_audit_log"},
		{tool.Baro_t("이 쪽 기록"), tool.Baro_itoa(len(rows)), tool.Baro_t("조건에 맞는 줄")},
		{tool.Baro_t("검증 도구"), tool.Baro_t("전체 검증"), tool.Baro_t("아래 단추로 실행")},
	})

	filter := tool.Baro_input("actor", "담당자", actor, tool.Baro_t("사용자 이름 또는 IP"), tool.Baro_t("정확히 일치하는 값만 봅니다."))
	filter += tool.Baro_input("action", tool.Baro_t("동작"), action, tool.Baro_t("예 : sec_"), tool.Baro_t("일부만 적어도 됩니다."))
	filter += tool.Baro_input_type("from", tool.Baro_t("시작 날짜"), from, "date", "")
	filter += tool.Baro_input_type("to", tool.Baro_t("끝 날짜"), to, "date", "")
	filter += tool.Baro_submit(tool.Baro_t("거르기"), "primary")

	body += tool.Baro_card(tool.Baro_t("거르기"), `<form class="bw_form" method="get" action="/baro/security/audit">`+filter+`</form>`)

	table_rows := [][]string{}
	fail_count := 0

	for _, row := range rows {
		result := baro_sec_audit_check(row)
		bad := result == "fail"
		if bad {
			fail_count++
		}

		change := `<div class="bw_sec_diff">`
		change += tool.Baro_t(`이전 : `) + tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Before), 60)) + `<br>`
		change += tool.Baro_t(`이후 : `) + tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.After), 60)) + `</div>`

		table_rows = append(table_rows, []string{
			baro_sec_audit_cell(tool.HTML_escape(row.ID), bad),
			baro_sec_audit_cell(tool.Baro_time_cell(row.Date), bad),
			baro_sec_audit_cell(tool.Baro_user_link(db, row.Actor, config.IP), bad),
			baro_sec_audit_cell(tool.HTML_escape(row.Action), bad),
			baro_sec_audit_cell(tool.HTML_escape(tool.Baro_trunc(row.Target, 40)), bad),
			baro_sec_audit_cell(change, bad),
			baro_sec_audit_cell(tool.HTML_escape(row.Auth), bad),
			baro_sec_audit_cell(tool.HTML_escape(tool.Baro_trunc(row.Reason, 40)), bad),
			baro_sec_audit_badge(result) + ` ` + tool.Baro_button("/baro/security/audit/detail?id="+url.QueryEscape(row.ID), "자세히", ""),
		})
	}

	if fail_count > 0 {
		body += tool.Baro_alert("danger",
			tool.Baro_t("이 쪽에서 검증에 실패한 줄이 <b>")+tool.Baro_itoa(fail_count)+tool.Baro_t("개</b> 있습니다. ")+
				tool.Baro_t("기록이 나중에 손댄 것일 수 있으니 즉시 확인하세요."))
	}

	if len(table_rows) == 0 {
		body += tool.Baro_empty(tool.Baro_t("조건에 맞는 감사 기록이 없습니다."))
	} else {
		body += tool.Baro_table(
			[]string{"번호", tool.Baro_t("언제"), tool.Baro_t("누가"), tool.Baro_t("무엇을"), tool.Baro_t("대상"), tool.Baro_t("어떤 값에서 어떤 값으로"), tool.Baro_t("권한"), tool.Baro_t("왜"), tool.Baro_t("검증")},
			table_rows, "",
		)
	}

	page_url := "/baro/security/audit?page={}"
	if actor != "" {
		page_url += "&actor=" + url.QueryEscape(actor)
	}
	if action != "" {
		page_url += "&action=" + url.QueryEscape(action)
	}
	if from != "" {
		page_url += "&from=" + url.QueryEscape(from)
	}
	if to != "" {
		page_url += "&to=" + url.QueryEscape(to)
	}

	body += tool.Baro_paging(page, len(rows), tool.Baro_page_size, page_url)

	body += tool.Baro_card(tool.Baro_t("검증은 어떻게 하나요"),
		tool.Baro_list([]string{
			tool.Baro_t("각 줄을 저장할 때 <code>번호 + 담당자 + 동작 + 대상 + 이전 값 + 이후 값 + 권한 + 사유 + 접속 IP + 접속 프로그램 + 시각</code> 을 ") +
				tool.Baro_t("이어 붙여 SHA-224 로 해시합니다."),
			tool.Baro_t("서명은 <b>사유 · 접속 정보 · 권한까지</b> 덮습니다. 사유만 몰래 고쳐도 해시가 달라집니다."),
			tool.Baro_t("화면은 같은 방식으로 다시 계산해 저장된 해시와 비교합니다."),
			tool.Baro_t("값이 하나라도 바뀌면 해시가 달라져 <b>위변조 의심</b> 으로 표시됩니다."),
			tool.Baro_t("계산식이 넓어지기 전에 쌓인 오래된 줄은 옛 계산식(<code>번호 + IP + 동작 + 대상 + 이전 값 + 이후 값 + 시각</code>)으로도 확인해 줍니다."),
			tool.Baro_t("서명이 없는 줄과, 이전·이후 값이 2000자를 넘어 잘려 저장된 줄은 <b>검증 불가</b> 로 표시합니다. 위변조와는 다릅니다."),
		})+tool.Baro_button("/baro/security/audit/verify", tool.Baro_t("전체 검증 실행"), "primary"),
	)

	return baro_sec_page(db, config, tool.Baro_t("내부 감사 로그"), body)
}

// ------------------------------------------------------------
//  /baro/security/audit/verify : 전체 위변조 검증
// ------------------------------------------------------------

// View_baro_sec_audit_verify : 감사 로그 전체 검증 화면
func View_baro_sec_audit_verify(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if page, ok := baro_sec_audit_gate(db, config); !ok {
		return page
	}

	if values != nil {
		return tool.Get_redirect("/baro/security/audit/verify")
	}

	limit := baro_sec_int_def(baro_sec_q(query, "limit"), 2000)
	if limit > 20000 {
		limit = 20000
	}

	baro_sec_audit_note_view(db, config, tool.Baro_t("전체 검증 (")+tool.Baro_itoa(limit)+tool.Baro_t("줄)"))

	rows := baro_sec_audit_query(db, "", "", "", "", 0, limit)

	ok_count, fail_count, unknown_count := 0, 0, 0
	fail_rows := [][]string{}

	for _, row := range rows {
		switch baro_sec_audit_check(row) {
		case "ok":
			ok_count++
		case "fail":
			fail_count++

			fail_rows = append(fail_rows, []string{
				`<span class="bw_sec_bad">` + tool.HTML_escape(row.ID) + `</span>`,
				tool.Baro_time_cell(row.Date),
				tool.Baro_user_link(db, row.Actor, config.IP),
				tool.HTML_escape(row.Action),
				tool.HTML_escape(tool.Baro_trunc(row.Target, 40)),
				tool.Baro_button("/baro/security/audit/detail?id="+url.QueryEscape(row.ID), "자세히", ""),
			})
		default:
			unknown_count++
		}
	}

	body := baro_sec_audit_style
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"/baro/security/audit", tool.Baro_t("감사 로그")}, {"", tool.Baro_t("전체 검증")}})
	body += tool.Baro_hero(tool.Baro_t("감사 로그 전체 검증"), tool.Baro_t("저장된 해시를 다시 계산해 위변조 여부를 확인합니다."), "")
	body += baro_sec_tabs("/baro/security/audit")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("검사한 줄"), tool.Baro_comma(len(rows)), tool.Baro_t("최신 ") + tool.Baro_comma(limit) + tool.Baro_t("줄까지")},
		{tool.Baro_t("검증 통과"), tool.Baro_comma(ok_count), tool.Baro_t("해시 일치")},
		{tool.Baro_t("위변조 의심"), tool.Baro_comma(fail_count), tool.Baro_t("해시 불일치")},
		{tool.Baro_t("검증 불가"), tool.Baro_comma(unknown_count), tool.Baro_t("서명 없음 또는 잘린 값")},
	})

	if fail_count == 0 {
		body += tool.Baro_alert("ok", tool.Baro_t("검사한 범위에서 위변조가 의심되는 기록은 없습니다."))
	} else {
		body += tool.Baro_alert("danger",
			tool.Baro_t("<b>위변조가 의심되는 기록이 ")+tool.Baro_itoa(fail_count)+tool.Baro_t("개 있습니다.</b> ")+
				tool.Baro_t("DB 를 직접 손댄 흔적일 수 있습니다. 백업과 대조하고 사고로 처리하세요."))
		body += tool.Baro_table([]string{"번호", tool.Baro_t("언제"), tool.Baro_t("누가"), tool.Baro_t("무엇을"), tool.Baro_t("대상"), "이동"}, fail_rows, "")
	}

	if unknown_count > 0 {
		body += tool.Baro_note(
			tool.Baro_t("검증 불가 ") + tool.Baro_itoa(unknown_count) + tool.Baro_t("줄은 서명이 없는 오래된 기록이거나, ") +
				tool.Baro_t("이전·이후 값이 2000자를 넘어 잘려 저장된 기록입니다. 위변조와는 다릅니다."),
		)
	}

	body += tool.Baro_card(tool.Baro_t("검사 범위 바꾸기"),
		`<form class="bw_form" method="get" action="/baro/security/audit/verify">`+
			tool.Baro_input_type("limit", tool.Baro_t("검사할 줄 수"), tool.Baro_itoa(limit), "number", tool.Baro_t("최신 기록부터 셉니다. 최대 20000줄."))+
			tool.Baro_submit(tool.Baro_t("다시 검증"), "primary")+
			`</form>`,
	)

	return baro_sec_page(db, config, tool.Baro_t("감사 로그 전체 검증"), body)
}

// ------------------------------------------------------------
//  /baro/security/audit/detail : 감사 로그 한 줄 자세히
// ------------------------------------------------------------

// View_baro_sec_audit_detail : 감사 기록 상세 화면
func View_baro_sec_audit_detail(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if page, ok := baro_sec_audit_gate(db, config); !ok {
		return page
	}

	if values != nil {
		return tool.Get_redirect("/baro/security/audit")
	}

	id := baro_sec_q(query, "id")

	body := baro_sec_audit_style
	body += tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"/baro/security/audit", tool.Baro_t("감사 로그")}, {"", "자세히"}})
	body += baro_sec_tabs("/baro/security/audit")

	if id == "" {
		body += tool.Baro_empty(tool.Baro_t("볼 기록 번호가 없습니다. 목록에서 골라 주세요."))
		body += tool.Baro_button("/baro/security/audit", tool.Baro_t("목록으로"), "")

		return baro_sec_page(db, config, tool.Baro_t("감사 기록"), body)
	}

	row := baro_sec_audit_row{}
	found := tool.QueryRow_DB(
		db,
		"select id, actor, action, target, before_data, after_data, auth, reason, ip, ua, date, sign_data from baro_audit_log where id = ?",
		[]any{
			&row.ID, &row.Actor, &row.Action, &row.Target,
			&row.Before, &row.After, &row.Auth, &row.Reason,
			&row.IP, &row.UA, &row.Date, &row.Sign,
		},
		id,
	)

	if !found {
		body += tool.Baro_empty(id + tool.Baro_t("번 감사 기록이 없습니다. 번호를 다시 확인하세요."))
		body += tool.Baro_button("/baro/security/audit", tool.Baro_t("목록으로"), "")

		return baro_sec_page(db, config, tool.Baro_t("감사 기록"), body)
	}

	baro_sec_audit_note_view(db, config, tool.Baro_t("상세 열람 (기록 ")+id+")")

	result := baro_sec_audit_check(row)

	body += tool.Baro_hero(tool.Baro_t("감사 기록 ")+row.ID, row.Action, "")

	if result == "fail" {
		body += tool.Baro_alert("danger", tool.Baro_t("<b>이 기록은 저장된 해시와 맞지 않습니다.</b> 저장 뒤에 값이 바뀐 것으로 보입니다."))
	} else if result == "unknown" {
		body += tool.Baro_alert("warn", tool.Baro_t("이 기록은 검증할 수 없습니다. 서명이 없거나, 값이 잘려 저장된 기록입니다."))
	} else {
		body += tool.Baro_alert("ok", tool.Baro_t("이 기록은 저장 당시의 해시와 일치합니다."))
	}

	body += tool.Baro_card(tool.Baro_t("무슨 일이 있었나"), tool.Baro_dl([][]string{
		{tool.Baro_t("누가"), tool.Baro_user_link(db, row.Actor, config.IP)},
		{tool.Baro_t("언제"), tool.HTML_escape(row.Date)},
		{tool.Baro_t("무엇을"), tool.HTML_escape(row.Action)},
		{tool.Baro_t("대상"), tool.HTML_escape(row.Target)},
		{tool.Baro_t("어떤 권한으로"), tool.HTML_escape(row.Auth)},
		{tool.Baro_t("왜"), tool.HTML_escape(row.Reason)},
		{tool.Baro_t("접속 IP"), tool.HTML_escape(row.IP)},
		{tool.Baro_t("접속 프로그램"), tool.HTML_escape(row.UA)},
	}))

	body += tool.Baro_card(tool.Baro_t("값의 변화"),
		tool.Baro_section(tool.Baro_t("이전 값"), tool.Baro_code(row.Before))+
			tool.Baro_section(tool.Baro_t("이후 값"), tool.Baro_code(row.After)),
	)

	body += tool.Baro_card(tool.Baro_t("서명"),
		tool.Baro_dl([][]string{
			{tool.Baro_t("저장된 해시"), tool.Baro_code(row.Sign)},
			{tool.Baro_t("검증 결과"), baro_sec_audit_badge(result)},
			{tool.Baro_t("계산식"), tool.Baro_code(tool.Baro_t("Sha224(번호 + 담당자 + 동작 + 대상 + 이전 값 + 이후 값 + 권한 + 사유 + 접속 IP + 접속 프로그램 + 시각)"))},
			{tool.Baro_t("덮는 범위"), tool.Baro_t("바뀐 값뿐 아니라 <b>사유 · 접속 정보 · 권한</b> 까지 서명에 들어갑니다.")},
		}),
	)

	body += tool.Baro_alert("info", tool.Baro_t("이 화면에는 고치기·지우기가 없습니다. 감사 기록은 어떤 권한으로도 바꿀 수 없어야 합니다."))
	body += tool.Baro_button("/baro/security/audit", tool.Baro_t("목록으로"), "")

	return baro_sec_page(db, config, tool.Baro_t("감사 기록 ")+row.ID, body)
}
