package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  15-1 개인정보 최소 수집 / 15-2 개인정보 요청 센터
//
//  URL : /baro/privacy ...
// ============================================================

// baro_legal_privacy_req_row : 개인정보 요청 한 건
type baro_legal_privacy_req_row struct {
	ID        string
	User      string
	Kind      string
	Detail    string
	Status    string
	Handler   string
	Date      string
	CloseDate string
	Verify    string
	Result    string
}

// baro_legal_privacy_req_get : 요청 한 건 읽기
func baro_legal_privacy_req_get(db *sql.DB, id string) baro_legal_privacy_req_row {
	row := baro_legal_privacy_req_row{}
	if strings.TrimSpace(id) == "" {
		return row
	}

	tool.QueryRow_DB(
		db,
		"select id, user, kind, detail, status, handler, date, close_date, verify, result from baro_privacy_req where id = ?",
		[]any{&row.ID, &row.User, &row.Kind, &row.Detail, &row.Status, &row.Handler, &row.Date, &row.CloseDate, &row.Verify, &row.Result},
		id,
	)

	return row
}

// baro_legal_privacy_req_list : 조건에 맞는 요청 목록
func baro_legal_privacy_req_list(db *sql.DB, where string, limit int, offset int, values ...any) []baro_legal_privacy_req_row {
	out := []baro_legal_privacy_req_row{}

	query := "select id, user, kind, detail, status, handler, date, close_date, verify, result from baro_privacy_req"
	if strings.TrimSpace(where) != "" {
		query += " where " + where
	}
	query += " order by id + 0 desc limit " + tool.Baro_itoa(limit) + " offset " + tool.Baro_itoa(offset)

	rows := tool.Query_DB(db, query, values...)
	defer rows.Close()

	for rows.Next() {
		row := baro_legal_privacy_req_row{}
		if err := rows.Scan(&row.ID, &row.User, &row.Kind, &row.Detail, &row.Status, &row.Handler, &row.Date, &row.CloseDate, &row.Verify, &row.Result); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_legal_privacy_del_row : 계정 탈퇴 요청 한 건
type baro_legal_privacy_del_row struct {
	ID        string
	User      string
	Kind      string
	Reason    string
	Status    string
	Date      string
	CloseDate string
	Handler   string
	Result    string
}

// baro_legal_privacy_del_get : 탈퇴 요청 한 건 읽기
func baro_legal_privacy_del_get(db *sql.DB, id string) baro_legal_privacy_del_row {
	row := baro_legal_privacy_del_row{}
	if strings.TrimSpace(id) == "" {
		return row
	}

	tool.QueryRow_DB(
		db,
		"select id, user, kind, reason, status, date, close_date, handler, result from baro_deletion_req where id = ?",
		[]any{&row.ID, &row.User, &row.Kind, &row.Reason, &row.Status, &row.Date, &row.CloseDate, &row.Handler, &row.Result},
		id,
	)

	return row
}

// baro_legal_privacy_del_list : 탈퇴 요청 목록
func baro_legal_privacy_del_list(db *sql.DB, where string, limit int, offset int, values ...any) []baro_legal_privacy_del_row {
	out := []baro_legal_privacy_del_row{}

	query := "select id, user, kind, reason, status, date, close_date, handler, result from baro_deletion_req"
	if strings.TrimSpace(where) != "" {
		query += " where " + where
	}
	query += " order by id + 0 desc limit " + tool.Baro_itoa(limit) + " offset " + tool.Baro_itoa(offset)

	rows := tool.Query_DB(db, query, values...)
	defer rows.Close()

	for rows.Next() {
		row := baro_legal_privacy_del_row{}
		if err := rows.Scan(&row.ID, &row.User, &row.Kind, &row.Reason, &row.Status, &row.Date, &row.CloseDate, &row.Handler, &row.Result); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// ------------------------------------------------------------
//  15-2 요청 센터 첫 화면 : /baro/privacy
// ------------------------------------------------------------

// View_baro_legal_privacy_home : 개인정보 요청 센터
func View_baro_legal_privacy_home(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = page
	_ = values

	body := tool.Baro_hero(
		tool.Baro_t("개인정보 요청 센터"),
		tool.Baro_t("내 정보를 열람하고, 고치고, 내려받고, 지울 수 있습니다."),
		tool.Baro_button("/baro/privacy/new", tool.Baro_t("요청 접수하기"), "primary")+" "+
			tool.Baro_button("/baro/privacy/mine", tool.Baro_t("내 요청 보기"), "")+" "+
			tool.Baro_button("/baro/privacy/policy", tool.Baro_t("개인정보 처리 방침"), "")+" "+
			tool.Baro_button("/baro/privacy/leave", tool.Baro_t("계정 탈퇴"), "danger"),
	)

	// 요청 종류 안내
	cards := []string{}
	for _, row := range baro_legal_privacy_kind_list() {
		link := "/baro/privacy/new?kind=" + tool.Url_parser(row[0])
		if row[0] == "leave" {
			link = "/baro/privacy/leave"
		}

		cards = append(cards, tool.Baro_card_ex(
			row[1],
			"",
			`<p>`+tool.HTML_escape(row[2])+`</p>`,
			tool.Baro_button(link, row[1]+tool.Baro_t(" 요청"), ""),
		))
	}

	body += tool.Baro_card(tool.Baro_t("요청할 수 있는 것"), tool.Baro_grid(cards, 3))

	// 처리 절차 안내
	body += tool.Baro_card(
		tool.Baro_t("처리 절차"),
		tool.Baro_list([]string{
			tool.Baro_t("요청을 접수합니다. (로그인 필요 — 본인 정보에 대한 요청이므로 계정 확인이 필요합니다)"),
			tool.Baro_t("비밀번호 재확인으로 본인 확인을 마칩니다. 본인 확인이 끝나야 열람·다운로드 결과를 받을 수 있습니다."),
			tool.Baro_t("개인정보 담당자가 요청을 검토하고 승인 · 일부 승인 · 거부 중 하나로 처리합니다."),
			tool.Baro_t("처리 결과는 알림으로 전달되고, 요청 상세 화면에 처리자 · 처리 시각 · 결과가 남습니다."),
			tool.Baro_t("통계는 ") + tool.Baro_t(`<a href="/baro/legal">법적 요청 통계</a>`) + tool.Baro_t(" 에 이름 없이 집계만 공개됩니다."),
		}),
	)

	// 내 요청 요약
	if tool.Baro_is_user(config.IP) {
		open_count := baro_legal_count(db, "select count(*) from baro_privacy_req where user = ? and "+baro_legal_open_where, config.IP)
		all_count := baro_legal_count(db, "select count(*) from baro_privacy_req where user = ?", config.IP)
		leave_count := baro_legal_count(db, "select count(*) from baro_deletion_req where user = ?", config.IP)

		body += tool.Baro_card_ex(
			tool.Baro_t("내 요청"),
			tool.Baro_t("자세한 내용은 내 요청 화면에서 볼 수 있습니다."),
			tool.Baro_stat_row([][]string{
				{tool.Baro_t("전체 요청"), tool.Baro_comma(all_count), ""},
				{tool.Baro_t("처리 중"), tool.Baro_comma(open_count), ""},
				{tool.Baro_t("탈퇴 요청"), tool.Baro_comma(leave_count), ""},
			}),
			tool.Baro_button("/baro/privacy/mine", tool.Baro_t("내 요청 자세히 보기"), "primary"),
		)
	} else {
		body += tool.Baro_alert("info", tool.Baro_t(`개인정보 요청은 본인 계정으로 로그인한 뒤에 접수할 수 있습니다. `)+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary"))
	}

	// 담당자용 바로가기
	if tool.Baro_can(db, config.IP, "privacy") {
		queue_count := baro_legal_count(db, "select count(*) from baro_privacy_req where "+baro_legal_open_where)
		del_count := baro_legal_count(db, "select count(*) from baro_deletion_req where "+baro_legal_open_where)

		body += tool.Baro_card_ex(
			tool.Baro_t("담당자 메뉴"),
			tool.Baro_t("개인정보 · 법적 요청 담당(privacy) 역할에게만 보입니다."),
			tool.Baro_stat_row([][]string{
				{tool.Baro_t("처리 대기 요청"), tool.Baro_comma(queue_count), ""},
				{tool.Baro_t("처리 대기 탈퇴"), tool.Baro_comma(del_count), ""},
			}),
			tool.Baro_button("/baro/privacy/queue/open/1", tool.Baro_t("처리 큐"), "primary")+" "+
				tool.Baro_button("/baro/privacy/retention", tool.Baro_t("보존 기간 정리"), "")+" "+
				tool.Baro_button("/baro/legal/make", tool.Baro_t("통계 집계"), ""),
		)
	}

	return baro_legal_page(db, config, tool.Baro_t("개인정보 요청 센터"), "/baro/privacy", body)
}

// ------------------------------------------------------------
//  15-1 개인정보 처리 방침 : /baro/privacy/policy
// ------------------------------------------------------------

// baro_legal_privacy_item_catalog : user_set 에 저장되는 개인정보 항목 목록
//
//	{저장 이름, 표시 이름, 목적, 공개 여부, 보관 기간}
func baro_legal_privacy_item_catalog() [][]string {
	return [][]string{
		{"pw", tool.Baro_t("비밀번호 (되돌릴 수 없는 해시)"), tool.Baro_t("로그인 확인"), tool.Baro_t("비공개"), tool.Baro_t("계정 유지 기간")},
		{"encode", tool.Baro_t("비밀번호 해시 방식"), tool.Baro_t("로그인 확인"), tool.Baro_t("비공개"), tool.Baro_t("계정 유지 기간")},
		{"email", tool.Baro_t("이메일"), tool.Baro_t("계정 확인 · 알림 발송"), tool.Baro_t("비공개"), tool.Baro_t("계정 유지 기간")},
		{"date", tool.Baro_t("가입 시각"), tool.Baro_t("가입 순서 · 신규 사용자 판단"), "공개", tool.Baro_t("계정 유지 기간")},
		{"acl", tool.Baro_t("권한 등급"), tool.Baro_t("권한 확인"), "공개", tool.Baro_t("계정 유지 기간")},
		{"user_name", tool.Baro_t("표시 이름"), tool.Baro_t("기여 표시"), tool.Baro_t("공개 (선택)"), tool.Baro_t("계정 유지 기간")},
		{"sub_user_name", tool.Baro_t("보조 표시 이름"), tool.Baro_t("기여 표시"), tool.Baro_t("공개 (선택)"), tool.Baro_t("계정 유지 기간")},
		{"user_title", tool.Baro_t("자기소개"), tool.Baro_t("프로필 표시"), tool.Baro_t("공개 (선택)"), tool.Baro_t("계정 유지 기간")},
		{"top_menu", tool.Baro_t("상단 메뉴 설정"), tool.Baro_t("화면 표시"), tool.Baro_t("비공개"), tool.Baro_t("계정 유지 기간")},
		{"skin_set", tool.Baro_t("스킨 설정"), tool.Baro_t("화면 표시"), tool.Baro_t("비공개"), tool.Baro_t("계정 유지 기간")},
		{"watchlist", tool.Baro_t("관심 문서 목록"), tool.Baro_t("관심 문서 알림"), tool.Baro_t("비공개"), tool.Baro_t("계정 유지 기간")},
		{"experience", tool.Baro_t("활동 경험치"), tool.Baro_t("활동 표시"), "공개", tool.Baro_t("계정 유지 기간")},
		{"level", tool.Baro_t("활동 등급"), tool.Baro_t("활동 표시"), "공개", tool.Baro_t("계정 유지 기간")},
	}
}

// baro_legal_privacy_item_count : user_set 항목별 실제 저장 건수
func baro_legal_privacy_item_count(db *sql.DB) map[string]int {
	out := map[string]int{}

	rows := tool.Query_DB(db, "select name, count(*) from user_set group by name")
	defer rows.Close()

	for rows.Next() {
		name := ""
		count := 0
		if err := rows.Scan(&name, &count); err != nil {
			continue
		}

		out[name] = count
	}

	return out
}

// View_baro_legal_privacy_policy : 개인정보 처리 방침 (15-1)
func View_baro_legal_privacy_policy(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	days := baro_legal_retention_days(db)
	policy := baro_legal_policy_get(db, "privacy")

	body := tool.Baro_hero(
		tool.Baro_t("개인정보 처리 방침"),
		tool.Baro_t("바로위키는 위키를 굴리는 데 꼭 필요한 정보만 모읍니다."),
		baro_legal_policy_head(policy),
	)

	// 원칙 요약
	body += tool.Baro_card(
		tool.Baro_t("최소 수집 원칙"),
		tool.Baro_grid([]string{
			tool.Baro_card_ex(tool.Baro_t("이메일은 비공개"), "", tool.Baro_t("<p>이메일은 계정 확인과 알림에만 씁니다. 다른 이용자에게 보이지 않고, 검색에도 걸리지 않습니다.</p>"), ""),
			tool.Baro_card_ex(tool.Baro_t("실명은 받지 않습니다"), "", tool.Baro_t("<p>가입할 때 실명을 요구하지 않습니다. 기여 기록에는 계정 이름만 남습니다.</p>"), ""),
			tool.Baro_card_ex(tool.Baro_t("전화번호를 받지 않습니다"), "", tool.Baro_t("<p>본인 확인에 휴대전화 인증을 쓰지 않습니다. 비밀번호 재확인으로 본인 확인을 합니다.</p>"), ""),
			tool.Baro_card_ex(tool.Baro_t("생년월일을 받지 않습니다"), "", tool.Baro_t("<p>나이나 생년월일을 저장하지 않습니다.</p>"), ""),
			tool.Baro_card_ex(tool.Baro_t("공개 항목은 직접 고릅니다"), "", tool.Baro_t("<p>표시 이름 · 자기소개 같은 공개 프로필 항목은 이용자가 직접 채우거나 비워 둘 수 있습니다.</p>"), ""),
			tool.Baro_card_ex(tool.Baro_t("운영 데이터는 기간이 지나면 지웁니다"), "", tool.Baro_t("<p>접속 기록 등 운영 데이터는 <strong>")+tool.Baro_itoa(days)+tool.Baro_t("일</strong> 보관 후 정리합니다.</p>"), ""),
		}, 3),
	)

	// 실제로 수집 중인 항목
	counts := baro_legal_privacy_item_count(db)
	item_rows := [][]string{}
	for _, row := range baro_legal_privacy_item_catalog() {
		count := counts[row[0]]

		open_badge := tool.Baro_badge(row[3], "muted")
		if strings.HasPrefix(row[3], "공개") {
			open_badge = tool.Baro_badge(row[3], "info")
		}

		state := tool.Baro_badge(tool.Baro_t("저장 중 ")+tool.Baro_comma(count)+tool.Baro_t("건"), "ok")
		if count == 0 {
			state = tool.Baro_badge(tool.Baro_t("현재 저장된 값 없음"), "muted")
		}

		item_rows = append(item_rows, []string{
			tool.HTML_escape(row[1]),
			tool.Baro_code(row[0]),
			tool.HTML_escape(row[2]),
			open_badge,
			tool.HTML_escape(row[4]),
			state,
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("실제로 수집 중인 항목"),
		tool.Baro_t("위키 엔진이 user_set 표에 저장하는 항목을 그대로 옮겨 놓은 것입니다. 건수는 지금 데이터베이스에서 센 값입니다."),
		tool.Baro_table(
			[]string{tool.Baro_t("항목"), tool.Baro_t("저장 이름"), tool.Baro_t("목적"), tool.Baro_t("공개 여부"), tool.Baro_t("보관 기간"), tool.Baro_t("현재 상태")},
			item_rows,
			tool.Baro_t("수집 항목 정의가 비어 있습니다."),
		),
		tool.Baro_note(tool.Baro_t("비밀번호는 되돌릴 수 없는 해시로만 저장하며, 운영자도 원문을 볼 수 없습니다.")),
	)

	// 수집하지 않는 항목
	body += tool.Baro_card_ex(
		tool.Baro_t("수집하지 않는 항목"),
		tool.Baro_t("아래 항목은 저장할 자리 자체를 만들지 않았습니다."),
		tool.Baro_table(
			[]string{tool.Baro_t("항목"), "상태", tool.Baro_t("설명")},
			[][]string{
				{tool.Baro_t("실명"), tool.Baro_badge(tool.Baro_t("수집 안 함"), "ok"), tool.Baro_t("기여 기록에는 계정 이름만 남습니다.")},
				{tool.Baro_t("전화번호"), tool.Baro_badge(tool.Baro_t("수집 안 함"), "ok"), tool.Baro_t("휴대전화 본인 인증을 쓰지 않습니다.")},
				{tool.Baro_t("생년월일 · 나이"), tool.Baro_badge(tool.Baro_t("수집 안 함"), "ok"), tool.Baro_t("연령 제한 기능을 두지 않습니다.")},
				{tool.Baro_t("주소"), tool.Baro_badge(tool.Baro_t("수집 안 함"), "ok"), tool.Baro_t("배송이나 청구가 없습니다.")},
				{tool.Baro_t("주민등록번호"), tool.Baro_badge(tool.Baro_t("수집 안 함"), "ok"), tool.Baro_t("편집 필터가 본문 게시도 막습니다.")},
				{tool.Baro_t("결제 정보"), tool.Baro_badge(tool.Baro_t("수집 안 함"), "ok"), tool.Baro_t("유료 기능이 없습니다.")},
			},
			"",
		),
		"",
	)

	// 운영 데이터 보존 기간
	cutoff := tool.Baro_time_add(tool.Baro_now(), -days, 0, 0)
	op_rows := [][]string{}
	for _, row := range baro_legal_privacy_op_targets() {
		total := baro_legal_count(db, "select count(*) from "+row[0])
		old := baro_legal_count(db, "select count(*) from "+row[0]+" where "+row[3]+" != '' and "+row[3]+" < ?", cutoff)

		badge := tool.Baro_badge(tool.Baro_t("정리 대상 없음"), "ok")
		if old > 0 {
			badge = tool.Baro_badge(tool.Baro_t("정리 대상 ")+tool.Baro_comma(old)+tool.Baro_t("건"), "warn")
		}

		op_rows = append(op_rows, []string{
			tool.HTML_escape(row[1]),
			tool.Baro_code(row[0]),
			tool.HTML_escape(row[2]),
			tool.Baro_comma(total) + tool.Baro_t("건"),
			badge,
		})
	}

	retention_foot := tool.Baro_note(tool.Baro_t("보존 기간은 baro_set 의 log_retention_days 값을 그대로 씁니다. 현재 값 : ") + tool.Baro_itoa(days) + tool.Baro_t("일"))
	if tool.Baro_can(db, config.IP, "privacy") {
		retention_foot += tool.Baro_button("/baro/privacy/retention", tool.Baro_t("보존 기간이 지난 데이터 정리"), "primary")
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("운영 데이터와 보존 기간"),
		tool.Baro_t("기준 시각 ")+cutoff+tool.Baro_t(" 보다 오래된 기록이 정리 대상입니다."),
		tool.Baro_table(
			[]string{tool.Baro_t("운영 데이터"), tool.Baro_t("저장 위치"), tool.Baro_t("목적"), tool.Baro_t("전체"), tool.Baro_t("보존 기간 경과")},
			op_rows,
			"",
		),
		retention_foot,
	)

	// 정책 본문
	body += tool.Baro_card_ex(
		tool.Baro_t("방침 본문"),
		tool.Baro_t("아래 본문은 baro_policy 의 privacy 최신 판에서 그대로 읽어 온 것입니다."),
		baro_legal_text_html(policy.Data),
		tool.Baro_button("/baro/policy", tool.Baro_t("정책 판 이력 보기"), "")+" "+tool.Baro_button("/baro/privacy/new", tool.Baro_t("개인정보 요청하기"), "primary"),
	)

	return baro_legal_page(db, config, tool.Baro_t("개인정보 처리 방침"), "/baro/privacy/policy", body)
}

// baro_legal_privacy_retention_total : 지금 정리 대상이 몇 건인지
func baro_legal_privacy_retention_total(db *sql.DB, targets [][]string, cutoff string) int {
	total := 0
	for _, row := range targets {
		total += baro_legal_count(db, "select count(*) from "+row[0]+" where "+row[3]+" != '' and "+row[3]+" < ?", cutoff)
	}

	return total
}

// baro_legal_privacy_op_targets : 보존 기간이 걸리는 운영 데이터
//
//	{표 이름, 표시 이름, 목적, 날짜 컬럼}
func baro_legal_privacy_op_targets() [][]string {
	return [][]string{
		{"baro_login_log", tool.Baro_t("접속 기록"), tool.Baro_t("계정 도용 확인 · 로그인 알림"), "date"},
		{"baro_rate", tool.Baro_t("요청 제한 계수기"), tool.Baro_t("도배와 자동 접수 방지"), "window_start"},
		{"baro_filter_log", tool.Baro_t("편집 필터 기록"), tool.Baro_t("반복 훼손 대응"), "date"},
	}
}

// ------------------------------------------------------------
//  15-1 보존 기간 경과 데이터 정리 : /baro/privacy/retention
// ------------------------------------------------------------

// View_baro_legal_privacy_retention : 보존 기간이 지난 운영 데이터 정리
func View_baro_legal_privacy_retention(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "privacy") {
		return tool.Baro_deny(db, config, tool.Baro_t("보존 기간 정리는 개인정보·법적 요청 담당(privacy) 역할만 실행할 수 있습니다."))
	}

	days := baro_legal_retention_days(db)
	cutoff := tool.Baro_time_add(tool.Baro_now(), -days, 0, 0)
	targets := baro_legal_privacy_op_targets()

	message := ""

	// 실행
	if values != nil {
		if tool.Baro_read_only(db) {
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 정리를 실행할 수 없습니다. 사유 : ")+tool.HTML_escape(tool.Baro_read_only_reason(db)))
		} else if !baro_legal_form_on(values, "agree") {
			message = tool.Baro_alert("danger", tool.Baro_t("정리 대상 수를 확인했다는 동의가 필요합니다."))
		} else {
			total := 0
			detail := []string{}

			if baro_legal_privacy_retention_total(db, targets, cutoff) < 1 {
				return tool.Get_redirect("/baro/privacy/retention")
			}

			for _, row := range targets {
				count := baro_legal_count(db, "select count(*) from "+row[0]+" where "+row[3]+" != '' and "+row[3]+" < ?", cutoff)
				if count < 1 {
					continue
				}

				tool.Exec_DB(db, "delete from "+row[0]+" where "+row[3]+" != '' and "+row[3]+" < ?", cutoff)

				total += count
				detail = append(detail, row[1]+" "+tool.Baro_comma(count)+tool.Baro_t("건"))
			}

			// 공개 기록에는 표 이름과 건수만 남긴다 (원칙 5)
			tool.Baro_public_log(
				db, "privacy_retention", "system", "운영 데이터",
				"보존 기간("+tool.Baro_itoa(days)+"일) 경과 데이터 정리",
				"총 "+tool.Baro_comma(total)+"건",
			)
			tool.Baro_audit(
				db, config, "privacy_retention_clean", "operational_data",
				"cutoff="+cutoff, strings.Join(detail, " / "),
				"보존 기간 경과 데이터 정리",
			)
			tool.Baro_metric_bump(db, "privacy_retention_clean", tool.Baro_today(), total)

			return tool.Get_redirect("/baro/privacy/retention")
		}
	}

	// 대상 수 표시
	rows_html := [][]string{}
	total := 0
	for _, row := range targets {
		all := baro_legal_count(db, "select count(*) from "+row[0])
		old := baro_legal_count(db, "select count(*) from "+row[0]+" where "+row[3]+" != '' and "+row[3]+" < ?", cutoff)
		total += old

		badge := tool.Baro_badge("없음", "ok")
		if old > 0 {
			badge = tool.Baro_badge(tool.Baro_comma(old)+tool.Baro_t("건"), "warn")
		}

		rows_html = append(rows_html, []string{
			tool.HTML_escape(row[1]),
			tool.Baro_code(row[0]),
			tool.HTML_escape(row[2]),
			tool.Baro_comma(all) + tool.Baro_t("건"),
			badge,
		})
	}

	body := tool.Baro_crumb([][]string{
		{"/baro/privacy", tool.Baro_t("개인정보 요청 센터")},
		{"/baro/privacy/policy", tool.Baro_t("처리 방침")},
		{"", tool.Baro_t("보존 기간 정리")},
	})

	body += message

	body += tool.Baro_hero(
		tool.Baro_t("보존 기간 경과 데이터 정리"),
		tool.Baro_t("보존 기간 ")+tool.Baro_itoa(days)+tool.Baro_t("일 / 기준 시각 ")+cutoff,
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("보존 기간"), tool.Baro_itoa(days) + tool.Baro_t("일"), "baro_set: log_retention_days"},
			{tool.Baro_t("정리 대상"), tool.Baro_comma(total) + tool.Baro_t("건"), tool.Baro_t("지금 지워질 기록 수")},
		}),
	)

	body += tool.Baro_card_ex(
		tool.Baro_t("정리 대상"),
		tool.Baro_t("먼저 대상 수를 확인한 뒤에 실행합니다."),
		tool.Baro_table(
			[]string{tool.Baro_t("운영 데이터"), tool.Baro_t("저장 위치"), tool.Baro_t("목적"), tool.Baro_t("전체"), tool.Baro_t("정리 대상")},
			rows_html,
			tool.Baro_t("정리할 운영 데이터가 없습니다."),
		),
		tool.Baro_note(tool.Baro_t("문서 편집 이력과 감사 로그(baro_audit_log)는 정리 대상이 아닙니다. 편집 이력은 라이선스상 출처 표기에 필요하고, 감사 로그는 운영 근거를 남기기 위해 보존합니다.")),
	)

	if total < 1 {
		body += tool.Baro_alert("ok", tool.Baro_t("보존 기간이 지난 운영 데이터가 없습니다. 지금은 실행할 것이 없습니다."))

		return baro_legal_page(db, config, tool.Baro_t("보존 기간 정리"), "/baro/privacy/policy", body)
	}

	form := tool.Baro_alert("danger", tool.Baro_t("이 작업은 되돌릴 수 없습니다. 대상 기록은 데이터베이스에서 완전히 지워집니다."))
	form += tool.Baro_checkbox("agree", tool.Baro_t("정리 대상 ")+tool.Baro_comma(total)+tool.Baro_t("건을 확인했으며, 되돌릴 수 없다는 것을 이해합니다."), "on", false, "")
	form += tool.Baro_submit(tool.Baro_t("보존 기간 경과 데이터 ")+tool.Baro_comma(total)+tool.Baro_t("건 정리"), "danger")

	body += tool.Baro_danger_zone(tool.Baro_t("정리 실행"), tool.Baro_form("/baro/privacy/retention", form))

	return baro_legal_page(db, config, tool.Baro_t("보존 기간 정리"), "/baro/privacy/policy", body)
}

// ------------------------------------------------------------
//  15-2 요청 접수 : /baro/privacy/new
// ------------------------------------------------------------

// View_baro_legal_privacy_new : 개인정보 요청 접수
func View_baro_legal_privacy_new(config tool.Config, kind string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/privacy/new"); need {
		return wall
	}

	message := ""
	detail := ""

	if values != nil {
		kind = baro_legal_form_get(values, "kind")
		detail = baro_legal_form_raw(values, "detail")
		password := baro_legal_form_raw(values, "password")

		switch {
		case kind == "leave":
			return tool.Get_redirect("/baro/privacy/leave")
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 요청을 접수할 수 없습니다. 사유 : ")+tool.HTML_escape(tool.Baro_read_only_reason(db)))
		case !baro_legal_privacy_kind_ok(kind):
			message = tool.Baro_alert("danger", tool.Baro_t("요청 종류를 골라 주세요."))
		case tool.Get_len(strings.TrimSpace(detail)) < 5:
			message = tool.Baro_alert("danger", tool.Baro_t("어떤 정보에 대한 요청인지 다섯 글자 이상으로 적어 주세요."))
		case !tool.Baro_rate_ok(db, "privacy_new", config.IP, 5, 3600):
			message = tool.Baro_alert("danger", tool.Baro_t("한 시간 안에 접수할 수 있는 요청 수를 넘었습니다. 잠시 뒤에 다시 시도해 주세요."))
		case password != "" && !tool.Password_check(db, config.IP, password):
			message = tool.Baro_alert("danger", tool.Baro_t("비밀번호가 맞지 않습니다. 본인 확인을 다시 해 주세요."))
		default:
			verify := ""
			if password != "" {
				verify = "on"
			}

			id := tool.Baro_next_id(db, "baro_privacy_req")
			tool.Exec_DB(
				db,
				"insert into baro_privacy_req (id, user, kind, detail, status, handler, date, close_date, verify, result) values (?, ?, ?, ?, 'open', '', ?, '', ?, '')",
				id, config.IP, kind, tool.Baro_trunc(detail, 4000), tool.Baro_now(), verify,
			)

			// 공개 기록에는 누가 냈는지 남기지 않는다 (원칙 5)
			tool.Baro_public_log(db, "privacy_req", "system", "", "개인정보 요청 접수", baro_legal_privacy_kind_name(kind))
			tool.Baro_audit(db, config, "privacy_req_new", "baro_privacy_req/"+id, "", kind, tool.Baro_trunc(detail, 500))
			tool.Baro_metric_bump(db, "privacy_req", tool.Baro_today(), 1)

			tool.Baro_notify(
				db, config.IP, "system",
				tool.Baro_t("개인정보 ")+baro_legal_privacy_kind_name(kind)+tool.Baro_t(" 요청이 접수되었습니다"),
				"/baro/privacy/req/"+id, "",
			)

			return tool.Get_redirect("/baro/privacy/req/" + id)
		}
	}

	if !baro_legal_privacy_kind_ok(kind) {
		kind = "view"
	}

	body := tool.Baro_crumb([][]string{
		{"/baro/privacy", tool.Baro_t("개인정보 요청 센터")},
		{"", tool.Baro_t("요청 접수")},
	})

	body += message

	body += tool.Baro_hero(
		"개인정보 요청 접수",
		tool.Baro_t("본인 계정(")+config.IP+tool.Baro_t(")에 대한 요청만 접수할 수 있습니다."),
		"",
	)

	kind_help := []string{}
	for _, row := range baro_legal_privacy_kind_list() {
		kind_help = append(kind_help, `<strong>`+tool.HTML_escape(row[1])+`</strong> — `+tool.HTML_escape(row[2]))
	}

	body += tool.Baro_card(tool.Baro_t("요청 종류 안내"), tool.Baro_list(kind_help))

	form := tool.Baro_select("kind", tool.Baro_t("요청 종류"), baro_legal_privacy_kind_options(), kind, tool.Baro_t("계정 탈퇴를 고르면 안내와 동의 화면으로 넘어갑니다."))
	form += tool.Baro_textarea("detail", tool.Baro_t("요청 내용"), detail, 7, tool.Baro_t("예) 가입할 때 넣은 이메일이 잘못되어 있습니다. 어떤 값으로 저장되어 있는지 확인하고 싶습니다."), tool.Baro_t("어떤 정보에 대한 요청인지 구체적으로 적어 주세요. 이 내용은 담당자만 볼 수 있습니다."))
	form += tool.Baro_input_type("password", tool.Baro_t("본인 확인 (계정 비밀번호)"), "", "password", tool.Baro_t("비밀번호를 다시 입력하면 본인 확인이 바로 끝납니다. 비워 두면 나중에 요청 화면에서 확인할 수 있지만, 확인 전에는 열람·다운로드 결과를 받을 수 없습니다."))
	form += tool.Baro_submit(tool.Baro_t("요청 접수"), "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("요청 내용"),
		tool.Baro_t("접수한 요청은 개인정보 담당자만 볼 수 있습니다."),
		tool.Baro_form("/baro/privacy/new", form),
		tool.Baro_note(tool.Baro_t("한 시간에 5건까지 접수할 수 있습니다.")),
	)

	return baro_legal_page(db, config, "개인정보 요청 접수", "/baro/privacy", body)
}

// ------------------------------------------------------------
//  15-2 내 요청 : /baro/privacy/mine
// ------------------------------------------------------------

// View_baro_legal_privacy_mine : 내가 낸 요청과 처리 상태
func View_baro_legal_privacy_mine(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/privacy/mine"); need {
		return wall
	}

	page_num := baro_legal_page_num(page)
	size := tool.Baro_page_size
	list := baro_legal_privacy_req_list(db, "user = ?", size, (page_num-1)*size, config.IP)

	table_rows := [][]string{}
	for _, row := range list {
		verify := tool.Baro_badge(tool.Baro_t("본인 확인 필요"), "warn")
		if row.Verify == "on" {
			verify = tool.Baro_badge(tool.Baro_t("본인 확인 완료"), "ok")
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/privacy/req/` + tool.Url_parser(row.ID) + `">#` + tool.HTML_escape(row.ID) + `</a>`,
			tool.HTML_escape(baro_legal_privacy_kind_name(row.Kind)),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Detail), 50)),
			baro_legal_status_badge(row.Status),
			verify,
			baro_legal_result_cell(row.Result),
			tool.Baro_time_cell(row.Date),
		})
	}

	body := tool.Baro_crumb([][]string{
		{"/baro/privacy", tool.Baro_t("개인정보 요청 센터")},
		{"", tool.Baro_t("내 요청")},
	})

	body += tool.Baro_hero(tool.Baro_t("내 개인정보 요청"), tool.Baro_t("요청과 처리 상태를 한눈에 볼 수 있습니다."), "")

	list_html := tool.Baro_table(
		[]string{"번호", "종류", tool.Baro_t("내용"), "상태", "본인 확인", tool.Baro_t("처리 결과"), "접수"},
		table_rows,
		tool.Baro_t("아직 접수한 요청이 없습니다."),
	)

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("아직 접수한 개인정보 요청이 없습니다. 열람 · 수정 · 다운로드 · 삭제 요청을 새로 낼 수 있습니다."))
	}

	list_html += tool.Baro_paging(page_num, len(list), size, "/baro/privacy/mine/{}")

	body += tool.Baro_card_ex(
		tool.Baro_t("요청 목록"),
		tool.Baro_t("요청 번호를 누르면 처리 기록을 볼 수 있습니다."),
		list_html,
		tool.Baro_button("/baro/privacy/new", tool.Baro_t("새 요청 접수"), "primary"),
	)

	// 탈퇴 요청
	del_list := baro_legal_privacy_del_list(db, "user = ?", size, 0, config.IP)
	del_rows := [][]string{}
	for _, row := range del_list {
		del_rows = append(del_rows, []string{
			`<a href="/baro/privacy/del/` + tool.Url_parser(row.ID) + `">#` + tool.HTML_escape(row.ID) + `</a>`,
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Reason), 50)),
			baro_legal_status_badge(row.Status),
			baro_legal_result_cell(row.Result),
			tool.Baro_time_cell(row.Date),
		})
	}

	del_html := tool.Baro_table(
		[]string{"번호", "사유", "상태", tool.Baro_t("처리 결과"), "접수"},
		del_rows,
		tool.Baro_t("접수한 탈퇴 요청이 없습니다."),
	)

	if len(del_rows) == 0 {
		del_html = tool.Baro_empty(tool.Baro_t("접수한 계정 탈퇴 요청이 없습니다."))
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("계정 탈퇴 요청"),
		tool.Baro_t("탈퇴 요청은 따로 관리합니다."),
		del_html,
		tool.Baro_button("/baro/privacy/leave", tool.Baro_t("계정 탈퇴 안내 보기"), ""),
	)

	return baro_legal_page(db, config, tool.Baro_t("내 개인정보 요청"), "/baro/privacy", body)
}

// ------------------------------------------------------------
//  15-2 처리 큐 : /baro/privacy/queue
// ------------------------------------------------------------

// View_baro_legal_privacy_queue : 개인정보 요청 처리 큐 (권한 privacy)
func View_baro_legal_privacy_queue(config tool.Config, mode string, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "privacy") {
		return tool.Baro_deny(db, config, tool.Baro_t("개인정보 요청 처리 큐는 개인정보·법적 요청 담당(privacy) 역할만 볼 수 있습니다."))
	}

	if mode != "done" && mode != "leave" {
		mode = "open"
	}

	page_num := baro_legal_page_num(page)
	size := tool.Baro_page_size

	body := tool.Baro_crumb([][]string{
		{"/baro/privacy", tool.Baro_t("개인정보 요청 센터")},
		{"", tool.Baro_t("처리 큐")},
	})

	open_count := baro_legal_count(db, "select count(*) from baro_privacy_req where "+baro_legal_open_where)
	done_count := baro_legal_count(db, "select count(*) from baro_privacy_req where "+baro_legal_closed_where)
	leave_count := baro_legal_count(db, "select count(*) from baro_deletion_req where "+baro_legal_open_where)
	verify_count := baro_legal_count(db, "select count(*) from baro_privacy_req where verify != 'on' and "+baro_legal_open_where)

	body += tool.Baro_hero(
		tool.Baro_t("개인정보 요청 처리 큐"),
		tool.Baro_t("요청 내용에는 개인정보가 들어 있습니다. 필요한 범위 밖으로 옮기지 마세요."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("처리 대기"), tool.Baro_comma(open_count), ""},
			{tool.Baro_t("본인 확인 전"), tool.Baro_comma(verify_count), tool.Baro_t("확인 전에는 결과를 주지 않습니다")},
			{"처리 완료", tool.Baro_comma(done_count), ""},
			{tool.Baro_t("탈퇴 대기"), tool.Baro_comma(leave_count), ""},
		}),
	)

	body += tool.Baro_tabs([][]string{
		{"/baro/privacy/queue/open/1", tool.Baro_t("처리 대기")},
		{"/baro/privacy/queue/done/1", "처리 완료"},
		{"/baro/privacy/queue/leave/1", tool.Baro_t("계정 탈퇴")},
	}, "/baro/privacy/queue/"+mode+"/1")

	if mode == "leave" {
		del_list := baro_legal_privacy_del_list(db, "", size, (page_num-1)*size)

		del_rows := [][]string{}
		for _, row := range del_list {
			del_rows = append(del_rows, []string{
				`<a href="/baro/privacy/del/` + tool.Url_parser(row.ID) + `">#` + tool.HTML_escape(row.ID) + `</a>`,
				baro_legal_who_cell(db, row.User, config.IP),
				tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Reason), 50)),
				baro_legal_status_badge(row.Status),
				baro_legal_result_cell(row.Result),
				tool.Baro_time_cell(row.Date),
			})
		}

		list_html := tool.Baro_table(
			[]string{"번호", tool.Baro_t("요청자"), "사유", "상태", tool.Baro_t("처리 결과"), "접수"},
			del_rows,
			tool.Baro_t("계정 탈퇴 요청이 없습니다."),
		)

		if len(del_rows) == 0 {
			list_html = tool.Baro_empty(tool.Baro_t("접수된 계정 탈퇴 요청이 없습니다."))
		}

		list_html += tool.Baro_paging(page_num, len(del_list), size, "/baro/privacy/queue/leave/{}")

		body += tool.Baro_card_ex(
			tool.Baro_t("계정 탈퇴 요청"),
			tool.Baro_t("탈퇴 처리는 계정만 닫습니다. 이미 공개된 정상 기여는 지우지 않습니다. (원칙 1 · 2)"),
			list_html,
			"",
		)

		return baro_legal_page(db, config, tool.Baro_t("개인정보 처리 큐"), "/baro/privacy", body)
	}

	where := baro_legal_open_where
	if mode == "done" {
		where = baro_legal_closed_where
	}

	list := baro_legal_privacy_req_list(db, where, size, (page_num-1)*size)

	table_rows := [][]string{}
	for _, row := range list {
		verify := tool.Baro_badge(tool.Baro_t("확인 전"), "warn")
		if row.Verify == "on" {
			verify = tool.Baro_badge(tool.Baro_t("확인됨"), "ok")
		}

		handler := `<span class="bw_muted">-</span>`
		if row.Handler != "" {
			handler = baro_legal_who_cell(db, row.Handler, config.IP)
		}

		table_rows = append(table_rows, []string{
			`<a href="/baro/privacy/req/` + tool.Url_parser(row.ID) + `">#` + tool.HTML_escape(row.ID) + `</a>`,
			baro_legal_who_cell(db, row.User, config.IP),
			tool.HTML_escape(baro_legal_privacy_kind_name(row.Kind)),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Detail), 40)),
			verify,
			baro_legal_status_badge(row.Status),
			handler,
			baro_legal_result_cell(row.Result),
			tool.Baro_time_cell(row.Date),
		})
	}

	list_html := tool.Baro_table(
		[]string{"번호", tool.Baro_t("요청자"), "종류", tool.Baro_t("내용"), "본인 확인", "상태", tool.Baro_t("처리자"), tool.Baro_t("결과"), "접수"},
		table_rows,
		tool.Baro_t("처리할 요청이 없습니다."),
	)

	if len(table_rows) == 0 {
		if mode == "done" {
			list_html = tool.Baro_empty(tool.Baro_t("아직 처리를 끝낸 요청이 없습니다."))
		} else {
			list_html = tool.Baro_empty(tool.Baro_t("처리를 기다리는 개인정보 요청이 없습니다."))
		}
	}

	list_html += tool.Baro_paging(page_num, len(list), size, "/baro/privacy/queue/"+mode+"/{}")

	body += tool.Baro_card_ex(
		tool.Baro_t("요청 목록"),
		tool.Baro_t("요청 번호를 누르면 처리 화면으로 들어갑니다."),
		list_html,
		tool.Baro_note(tool.Baro_t("요청 내용은 담당자 화면에만 표시됩니다. 공개 로그와 통계에는 종류와 건수만 남습니다.")),
	)

	return baro_legal_page(db, config, tool.Baro_t("개인정보 처리 큐"), "/baro/privacy", body)
}

// ------------------------------------------------------------
//  15-2 요청 상세 / 처리 : /baro/privacy/req/<id>
// ------------------------------------------------------------

// View_baro_legal_privacy_req : 개인정보 요청 상세와 처리
func View_baro_legal_privacy_req(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row := baro_legal_privacy_req_get(db, id)
	if row.ID == "" {
		return baro_legal_not_found(db, config, "/baro/privacy", tool.Baro_t("그런 번호의 개인정보 요청이 없습니다."), "/baro/privacy/mine", tool.Baro_t("내 요청으로"))
	}

	is_owner := tool.Baro_is_user(config.IP) && config.IP == row.User
	is_handler := tool.Baro_can(db, config.IP, "privacy")

	if !is_owner && !is_handler {
		return tool.Baro_deny(db, config, tool.Baro_t("개인정보 요청은 요청한 본인과 개인정보 담당자만 볼 수 있습니다."))
	}

	message := ""

	if values != nil {
		action := baro_legal_form_get(values, "action")

		switch {
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 처리할 수 없습니다."))
		case action == "verify" && is_owner:
			password := baro_legal_form_raw(values, "password")
			if password == "" || !tool.Password_check(db, config.IP, password) {
				message = tool.Baro_alert("danger", tool.Baro_t("비밀번호가 맞지 않습니다."))
				break
			}

			tool.Exec_DB(db, "update baro_privacy_req set verify = 'on' where id = ?", row.ID)
			tool.Baro_audit(db, config, "privacy_req_verify", "baro_privacy_req/"+row.ID, row.Verify, "on", "본인 확인")

			return tool.Get_redirect("/baro/privacy/req/" + row.ID)
		case action == "cancel" && is_owner:
			if !baro_legal_status_open(row.Status) {
				message = tool.Baro_alert("danger", tool.Baro_t("이미 처리가 끝난 요청은 취소할 수 없습니다."))
				break
			}

			tool.Exec_DB(
				db,
				"update baro_privacy_req set status = 'closed', close_date = ?, result = ? where id = ?",
				tool.Baro_now(), baro_legal_result_enc("", "요청자가 직접 취소했습니다."), row.ID,
			)
			tool.Baro_audit(db, config, "privacy_req_cancel", "baro_privacy_req/"+row.ID, row.Status, "closed", "요청자 취소")
			tool.Baro_public_log(db, "privacy_req_close", "system", "", "개인정보 요청 종료", "요청자 취소")

			return tool.Get_redirect("/baro/privacy/req/" + row.ID)
		case action == "handle" && is_handler:
			status := baro_legal_form_get(values, "status")
			code := baro_legal_form_get(values, "result_code")
			note := baro_legal_form_raw(values, "result_note")

			if !baro_legal_status_ok(status) {
				message = tool.Baro_alert("danger", tool.Baro_t("처리 상태를 골라 주세요."))
				break
			}

			if (status == "done" || status == "rejected") && strings.TrimSpace(note) == "" {
				message = tool.Baro_alert("danger", tool.Baro_t("처리를 끝낼 때는 요청자에게 보여 줄 설명을 반드시 남겨야 합니다."))
				break
			}

			if status == "done" && code == "" {
				message = tool.Baro_alert("danger", tool.Baro_t("처리 완료로 바꿀 때는 승인 · 일부 승인 · 거부 중 하나를 골라야 합니다."))
				break
			}

			close_date := ""
			if status == "done" || status == "rejected" || status == "closed" {
				close_date = tool.Baro_now()
			}

			tool.Exec_DB(
				db,
				"update baro_privacy_req set status = ?, handler = ?, close_date = ?, result = ? where id = ?",
				status, config.IP, close_date, baro_legal_result_enc(code, tool.Baro_trunc(note, 2000)), row.ID,
			)

			tool.Baro_audit(
				db, config, "privacy_req_handle", "baro_privacy_req/"+row.ID,
				row.Status+" / "+baro_legal_result_code(row.Result), status+" / "+code,
				tool.Baro_trunc(note, 500),
			)
			tool.Baro_public_log(
				db, "privacy_req_handle", "system", "",
				"개인정보 요청 처리", baro_legal_privacy_kind_name(row.Kind)+" / "+baro_legal_status_name(status),
			)
			tool.Baro_metric_bump(db, "privacy_req_handle", tool.Baro_today(), 1)

			tool.Baro_notify(
				db, row.User, "system",
				tool.Baro_t("개인정보 ")+baro_legal_privacy_kind_name(row.Kind)+tool.Baro_t(" 요청이 ")+baro_legal_status_name(status)+tool.Baro_t(" 상태로 바뀌었습니다"),
				"/baro/privacy/req/"+row.ID, "",
			)

			return tool.Get_redirect("/baro/privacy/req/" + row.ID)
		default:
			message = tool.Baro_alert("danger", tool.Baro_t("이 동작을 할 권한이 없습니다."))
		}
	}

	row = baro_legal_privacy_req_get(db, id)

	body := tool.Baro_crumb([][]string{
		{"/baro/privacy", tool.Baro_t("개인정보 요청 센터")},
		{"/baro/privacy/mine", tool.Baro_t("내 요청")},
		{"", tool.Baro_t("요청 #") + row.ID},
	})

	body += message

	verify_badge := tool.Baro_badge(tool.Baro_t("본인 확인 전"), "warn")
	if row.Verify == "on" {
		verify_badge = tool.Baro_badge(tool.Baro_t("본인 확인 완료"), "ok")
	}

	handler_html := tool.Baro_t(`<span class="bw_muted">아직 배정되지 않음</span>`)
	if row.Handler != "" {
		handler_html = baro_legal_who_cell(db, row.Handler, config.IP)
	}

	body += tool.Baro_hero(
		tool.Baro_t("개인정보 ")+baro_legal_privacy_kind_name(row.Kind)+tool.Baro_t(" 요청 #")+row.ID,
		"접수 "+row.Date,
		tool.Baro_kv_grid([][]string{
			{tool.Baro_t("요청 종류"), tool.HTML_escape(baro_legal_privacy_kind_name(row.Kind))},
			{tool.Baro_t("요청자"), baro_legal_who_cell(db, row.User, config.IP)},
			{"상태", baro_legal_status_badge(row.Status)},
			{"본인 확인", verify_badge},
			{tool.Baro_t("처리자"), handler_html},
			{tool.Baro_t("처리 시각"), tool.Baro_time_cell(row.CloseDate)},
			{tool.Baro_t("처리 결과"), baro_legal_result_cell(row.Result)},
		}),
	)

	body += tool.Baro_card(tool.Baro_t("요청 내용"), baro_legal_quote_html(row.Detail))

	if note := baro_legal_result_note(row.Result); note != "" {
		body += tool.Baro_card(tool.Baro_t("처리 설명"), baro_legal_quote_html(note))
	}

	// 승인된 요청의 결과 받기
	if baro_legal_result_approved(row.Status, row.Result) && is_owner {
		switch row.Kind {
		case "view":
			if row.Verify == "on" {
				body += tool.Baro_card_ex(
					tool.Baro_t("열람 결과"),
					tool.Baro_t("승인된 열람 요청입니다."),
					tool.Baro_t("<p>우리가 보관하고 있는 내 정보를 화면에서 바로 볼 수 있습니다.</p>"),
					tool.Baro_button("/baro/privacy/view/"+tool.Url_parser(row.ID), "내 정보 열람", "primary"),
				)
			} else {
				body += tool.Baro_alert("warn", tool.Baro_t("열람 결과를 보려면 아래에서 본인 확인을 먼저 마쳐야 합니다."))
			}
		case "download":
			if row.Verify == "on" {
				body += tool.Baro_card_ex(
					tool.Baro_t("다운로드"),
					tool.Baro_t("승인된 다운로드 요청입니다."),
					tool.Baro_t("<p>내 정보를 JSON 파일로 내려받을 수 있습니다. 다른 서비스로 옮기거나 보관할 때 쓸 수 있습니다.</p>"),
					tool.Baro_button("/baro/privacy/download/"+tool.Url_parser(row.ID), "JSON 파일 내려받기", "primary"),
				)
			} else {
				body += tool.Baro_alert("warn", tool.Baro_t("다운로드를 받으려면 아래에서 본인 확인을 먼저 마쳐야 합니다."))
			}
		case "edit":
			body += tool.Baro_card_ex(
				tool.Baro_t("수정 안내"),
				tool.Baro_t("승인된 수정 요청입니다."),
				tool.Baro_t("<p>아래 화면에서 직접 고칠 수 있는 항목은 바로 고쳐 주세요. 직접 고칠 수 없는 항목은 담당자가 처리 설명에 방법을 적어 두었습니다.</p>"),
				tool.Baro_button("/change/email", tool.Baro_t("이메일 바꾸기"), "")+" "+
					tool.Baro_button("/change/pw", tool.Baro_t("비밀번호 바꾸기"), "")+" "+
					tool.Baro_button("/change/user_name", tool.Baro_t("계정 이름 바꾸기"), ""),
			)
		}
	}

	// 본인 확인
	if is_owner && row.Verify != "on" {
		form := tool.Baro_hidden("action", "verify")
		form += tool.Baro_input_type("password", tool.Baro_t("계정 비밀번호"), "", "password", tool.Baro_t("본인 확인은 비밀번호 재확인으로만 합니다. 전화번호나 신분증을 요구하지 않습니다."))
		form += tool.Baro_submit("본인 확인", "primary")

		body += tool.Baro_card_ex(
			"본인 확인",
			tool.Baro_t("본인 확인이 끝나야 열람 · 다운로드 결과를 받을 수 있습니다."),
			tool.Baro_form("/baro/privacy/req/"+tool.Url_parser(row.ID), form),
			"",
		)
	}

	// 요청자 취소
	if is_owner && baro_legal_status_open(row.Status) {
		form := tool.Baro_hidden("action", "cancel")
		form += tool.Baro_submit(tool.Baro_t("이 요청 취소"), "danger")

		body += tool.Baro_card_ex(
			tool.Baro_t("요청 취소"),
			tool.Baro_t("아직 처리되지 않은 요청은 직접 취소할 수 있습니다."),
			tool.Baro_form("/baro/privacy/req/"+tool.Url_parser(row.ID), form),
			"",
		)
	}

	// 담당자 처리
	if is_handler {
		status_options := [][]string{}
		for _, status_row := range baro_legal_status_list() {
			status_options = append(status_options, []string{status_row[0], status_row[1]})
		}

		current_status := row.Status
		if current_status == "" {
			current_status = "open"
		}

		form := tool.Baro_hidden("action", "handle")
		form += tool.Baro_select("status", tool.Baro_t("처리 상태"), status_options, current_status, "")
		form += tool.Baro_select("result_code", tool.Baro_t("처리 결과"), baro_legal_result_list(), baro_legal_result_code(row.Result), tool.Baro_t("처리 완료로 바꿀 때는 반드시 골라야 합니다."))
		form += tool.Baro_textarea("result_note", tool.Baro_t("요청자에게 보여 줄 설명"), baro_legal_result_note(row.Result), 5, tool.Baro_t("예) 요청하신 이메일 값을 확인해 수정했습니다."), tool.Baro_t("이 설명은 요청자에게 그대로 보입니다. 다른 사람의 개인정보를 적지 마세요."))
		form += tool.Baro_submit(tool.Baro_t("처리 기록 저장"), "primary")

		warn := ""
		if row.Verify != "on" {
			warn = tool.Baro_alert("warn", tool.Baro_t("아직 본인 확인이 끝나지 않은 요청입니다. 열람 · 다운로드 요청은 본인 확인 뒤에 승인해 주세요."))
		}

		body += tool.Baro_card_ex(
			tool.Baro_t("담당자 처리"),
			tool.Baro_t("처리자 · 처리 시각 · 결과가 함께 기록됩니다."),
			warn+tool.Baro_form("/baro/privacy/req/"+tool.Url_parser(row.ID), form),
			tool.Baro_note(tool.Baro_t("처리 기록은 감사 로그에 남고, 공개 통계에는 이름 없이 건수만 반영됩니다.")),
		)
	}

	return baro_legal_page(db, config, tool.Baro_t("개인정보 요청 #")+row.ID, "/baro/privacy", body)
}

// ------------------------------------------------------------
//  내 정보 모으기 (열람 / 다운로드 공통)
// ------------------------------------------------------------

// baro_legal_privacy_pairs : 이름-값 쌍 목록을 읽는다
func baro_legal_privacy_pairs(db *sql.DB, query string, values ...any) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(db, query, values...)
	defer rows.Close()

	for rows.Next() {
		a, b := "", ""
		if err := rows.Scan(&a, &b); err != nil {
			continue
		}

		out = append(out, []string{a, b})
	}

	return out
}

// baro_legal_privacy_account : 계정 정보 (비밀번호는 절대 내보내지 않는다)
func baro_legal_privacy_account(db *sql.DB, user string) [][]string {
	out := [][]string{}

	for _, pair := range baro_legal_privacy_pairs(db, "select name, data from user_set where id = ? order by name asc", user) {
		switch pair[0] {
		case "pw", "encode":
			out = append(out, []string{pair[0], tool.Baro_t("(저장됨 — 되돌릴 수 없는 형태라 내용은 제공하지 않습니다)")})
		default:
			out = append(out, []string{pair[0], pair[1]})
		}
	}

	return out
}

// baro_legal_privacy_login_log : 접속 기록
func baro_legal_privacy_login_log(db *sql.DB, user string, limit int) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(
		db,
		"select date, kind, ip, ua from baro_login_log where user = ? order by id + 0 desc limit "+tool.Baro_itoa(limit),
		user,
	)
	defer rows.Close()

	for rows.Next() {
		date, kind, ip, ua := "", "", "", ""
		if err := rows.Scan(&date, &kind, &ip, &ua); err != nil {
			continue
		}

		out = append(out, []string{date, kind, ip, ua})
	}

	return out
}

// baro_legal_privacy_history : 내 편집 기록 (라이선스상 지워지지 않는 부분)
func baro_legal_privacy_history(db *sql.DB, user string, limit int) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(
		db,
		"select id, title, date, leng, type from history where ip = ? order by id + 0 desc limit "+tool.Baro_itoa(limit),
		user,
	)
	defer rows.Close()

	for rows.Next() {
		id, title, date, leng, kind := "", "", "", "", ""
		if err := rows.Scan(&id, &title, &date, &leng, &kind); err != nil {
			continue
		}

		out = append(out, []string{id, title, date, leng, kind})
	}

	return out
}

// baro_legal_privacy_collect : 내 정보를 한 덩어리로 모은다
func baro_legal_privacy_collect(db *sql.DB, user string) map[string]any {
	out := map[string]any{}

	out["생성_시각"] = tool.Baro_now()
	out["계정"] = user
	out["안내"] = "바로위키가 이 계정에 대해 보관하고 있는 정보입니다. 비밀번호는 되돌릴 수 없는 해시로만 저장되어 제공되지 않습니다."

	account := map[string]string{}
	for _, pair := range baro_legal_privacy_account(db, user) {
		account[pair[0]] = pair[1]
	}
	out["계정_설정"] = account

	baro_set := map[string]string{}
	for _, pair := range baro_legal_privacy_pairs(db, "select name, data from baro_user_set where user = ? order by name asc", user) {
		baro_set[pair[0]] = pair[1]
	}
	out["바로위키_설정"] = baro_set

	out["역할"] = tool.Baro_user_roles(db, user)
	out["관심_분야"] = tool.Baro_user_interest(db, user)

	consents := []map[string]string{}
	for _, pair := range baro_legal_privacy_pairs(db, "select kind, version from baro_consent where user = ? order by id + 0 desc", user) {
		consents = append(consents, map[string]string{"종류": pair[0], "정책_판": pair[1]})
	}
	out["동의_이력"] = consents

	logs := []map[string]string{}
	for _, pair := range baro_legal_privacy_login_log(db, user, 200) {
		logs = append(logs, map[string]string{"시각": pair[0], "종류": pair[1], "IP": pair[2], "브라우저": pair[3]})
	}
	out["접속_기록"] = logs

	edits := []map[string]string{}
	for _, pair := range baro_legal_privacy_history(db, user, 1000) {
		edits = append(edits, map[string]string{"판_번호": pair[0], "문서": pair[1], "시각": pair[2], "길이": pair[3], "종류": pair[4]})
	}
	out["편집_기록"] = edits

	reqs := []map[string]string{}
	for _, req := range baro_legal_privacy_req_list(db, "user = ?", 500, 0, user) {
		reqs = append(reqs, map[string]string{
			"번호": req.ID, "종류": req.Kind, "상태": req.Status,
			"접수": req.Date, "처리": req.CloseDate,
		})
	}
	out["개인정보_요청"] = reqs

	dels := []map[string]string{}
	for _, del := range baro_legal_privacy_del_list(db, "user = ?", 500, 0, user) {
		dels = append(dels, map[string]string{"번호": del.ID, "상태": del.Status, "접수": del.Date, "처리": del.CloseDate})
	}
	out["탈퇴_요청"] = dels

	out["집계"] = map[string]int{
		"편집_기록_전체":  baro_legal_count(db, "select count(*) from history where ip = ?", user),
		"기여_이벤트":    baro_legal_count(db, "select count(*) from baro_contrib_event where user = ?", user),
		"받은_알림":     baro_legal_count(db, "select count(*) from baro_notify where user = ?", user),
		"보관중_접속기록":  baro_legal_count(db, "select count(*) from baro_login_log where user = ?", user),
		"보관중_동의기록":  baro_legal_count(db, "select count(*) from baro_consent where user = ?", user),
		"저작권_신고_접수": baro_legal_count(db, "select count(*) from baro_copyright_req where claimant = ?", user),
	}

	return out
}

// ------------------------------------------------------------
//  15-2 열람 결과 : /baro/privacy/view/<id>
// ------------------------------------------------------------

// View_baro_legal_privacy_view : 승인된 열람 요청의 결과 화면
func View_baro_legal_privacy_view(config tool.Config, id string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/privacy/mine"); need {
		return wall
	}

	row := baro_legal_privacy_req_get(db, id)
	if row.ID == "" {
		return baro_legal_not_found(db, config, "/baro/privacy", tool.Baro_t("그런 번호의 개인정보 요청이 없습니다."), "/baro/privacy/mine", tool.Baro_t("내 요청으로"))
	}

	if config.IP != row.User {
		return tool.Baro_deny(db, config, tool.Baro_t("열람 결과는 요청한 본인만 볼 수 있습니다."))
	}

	if row.Kind != "view" {
		return baro_legal_not_found(db, config, "/baro/privacy", tool.Baro_t("이 요청은 열람 요청이 아닙니다."), "/baro/privacy/req/"+row.ID, tool.Baro_t("요청으로 돌아가기"))
	}

	if !baro_legal_result_approved(row.Status, row.Result) {
		body := tool.Baro_alert("warn", tool.Baro_t("아직 승인되지 않은 열람 요청입니다. 담당자가 승인하면 이 화면에서 내 정보를 볼 수 있습니다."))
		body += tool.Baro_card("", tool.Baro_button("/baro/privacy/req/"+tool.Url_parser(row.ID), "요청 상태 보기", "primary"))

		return baro_legal_page(db, config, tool.Baro_t("열람 결과"), "/baro/privacy", body)
	}

	if row.Verify != "on" {
		body := tool.Baro_alert("warn", tool.Baro_t("본인 확인이 끝나지 않았습니다. 요청 화면에서 비밀번호 재확인을 먼저 해 주세요."))
		body += tool.Baro_card("", tool.Baro_button("/baro/privacy/req/"+tool.Url_parser(row.ID), "본인 확인하러 가기", "primary"))

		return baro_legal_page(db, config, tool.Baro_t("열람 결과"), "/baro/privacy", body)
	}

	// 열람 사실을 감사 로그에 남긴다
	tool.Baro_audit(db, config, "privacy_req_view", "baro_privacy_req/"+row.ID, "", "본인 열람", "승인된 열람 요청")

	body := tool.Baro_crumb([][]string{
		{"/baro/privacy", tool.Baro_t("개인정보 요청 센터")},
		{"/baro/privacy/mine", tool.Baro_t("내 요청")},
		{"/baro/privacy/req/" + row.ID, tool.Baro_t("요청 #") + row.ID},
		{"", tool.Baro_t("열람 결과")},
	})

	body += tool.Baro_hero(
		"내 정보 열람",
		tool.Baro_t("요청 #")+row.ID+tool.Baro_t(" 승인 결과입니다."),
		tool.Baro_button("/baro/privacy/download/"+tool.Url_parser(row.ID), "같은 내용 JSON 으로 내려받기", "")+" "+
			tool.Baro_button("/baro/privacy/new?kind=edit", tool.Baro_t("잘못된 값 수정 요청"), ""),
	)

	// 계정 설정
	account_rows := [][]string{}
	catalog := map[string][]string{}
	for _, item := range baro_legal_privacy_item_catalog() {
		catalog[item[0]] = item
	}

	for _, pair := range baro_legal_privacy_account(db, row.User) {
		label := pair[0]
		open := "-"
		if item, ok := catalog[pair[0]]; ok {
			label = item[1]
			open = item[3]
		}

		account_rows = append(account_rows, []string{
			tool.HTML_escape(label),
			tool.Baro_code(pair[0]),
			tool.HTML_escape(tool.Baro_trunc(pair[1], 300)),
			tool.HTML_escape(open),
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("계정 정보"),
		tool.Baro_t("user_set 표에 저장된 값 그대로입니다."),
		tool.Baro_table([]string{tool.Baro_t("항목"), tool.Baro_t("저장 이름"), tool.Baro_t("저장된 값"), tool.Baro_t("공개 여부")}, account_rows, tool.Baro_t("저장된 계정 설정이 없습니다.")),
		"",
	)

	// 역할 · 관심 분야 · 동의
	roles := tool.Baro_user_roles(db, row.User)
	role_names := []string{}
	for _, role := range roles {
		role_names = append(role_names, tool.Baro_role_name(role))
	}

	interests := []string{}
	for _, field := range tool.Baro_user_interest(db, row.User) {
		interests = append(interests, tool.Baro_field_name(db, field))
	}

	consent_rows := [][]string{}
	consent_list := tool.Query_DB(db, "select kind, version, date from baro_consent where user = ? order by id + 0 desc limit 100", row.User)
	defer consent_list.Close()

	for consent_list.Next() {
		kind, version, date := "", "", ""
		if err := consent_list.Scan(&kind, &version, &date); err != nil {
			continue
		}

		consent_rows = append(consent_rows, []string{
			tool.HTML_escape(kind),
			tool.HTML_escape(version) + tool.Baro_t("판"),
			tool.Baro_time_cell(date),
		})
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("역할 · 관심 분야 · 동의"),
		"",
		tool.Baro_dl([][]string{
			{tool.Baro_t("운영 역할"), func() string {
				if len(role_names) == 0 {
					return tool.Baro_t(`<span class="bw_muted">없음</span>`)
				}

				return tool.Baro_pills(role_names)
			}()},
			{tool.Baro_t("관심 분야"), func() string {
				if len(interests) == 0 {
					return tool.Baro_t(`<span class="bw_muted">없음</span>`)
				}

				return tool.Baro_pills(interests)
			}()},
			{tool.Baro_t("동의 이력"), tool.Baro_table([]string{"종류", tool.Baro_t("정책 판"), tool.Baro_t("동의 시각")}, consent_rows, tool.Baro_t("동의 기록이 없습니다."))},
		}),
		"",
	)

	// 접속 기록
	log_rows := [][]string{}
	for _, log := range baro_legal_privacy_login_log(db, row.User, 30) {
		log_rows = append(log_rows, []string{
			tool.Baro_time_cell(log[0]),
			tool.HTML_escape(log[1]),
			tool.HTML_escape(log[2]),
			tool.HTML_escape(tool.Baro_trunc(log[3], 60)),
		})
	}

	days := baro_legal_retention_days(db)

	body += tool.Baro_card_ex(
		tool.Baro_t("접속 기록"),
		tool.Baro_t("최근 30건만 보여 줍니다. 접속 기록은 ")+tool.Baro_itoa(days)+tool.Baro_t("일이 지나면 정리됩니다."),
		tool.Baro_table([]string{"시각", "종류", "IP", "브라우저"}, log_rows, tool.Baro_t("보관 중인 접속 기록이 없습니다.")),
		"",
	)

	// 편집 기록
	edit_rows := [][]string{}
	for _, edit := range baro_legal_privacy_history(db, row.User, 30) {
		edit_rows = append(edit_rows, []string{
			tool.HTML_escape(edit[0]),
			tool.Baro_doc_link(edit[1]),
			tool.Baro_time_cell(edit[2]),
			tool.HTML_escape(edit[3]),
		})
	}

	edit_total := baro_legal_count(db, "select count(*) from history where ip = ?", row.User)

	body += tool.Baro_card_ex(
		tool.Baro_t("편집 기록"),
		tool.Baro_t("전체 ")+tool.Baro_comma(edit_total)+tool.Baro_t("건 가운데 최근 30건입니다."),
		tool.Baro_table([]string{tool.Baro_t("판 번호"), "문서", "시각", "길이"}, edit_rows, tool.Baro_t("편집 기록이 없습니다.")),
		tool.Baro_note(tool.Baro_t("편집 기록은 라이선스상 출처 표기에 쓰이므로, 계정을 지워도 자동으로 사라지지 않습니다.")),
	)

	return baro_legal_page(db, config, "내 정보 열람", "/baro/privacy", body)
}

// ------------------------------------------------------------
//  15-2 다운로드 : /baro/privacy/download/<id>
// ------------------------------------------------------------

// View_baro_legal_privacy_download : 승인된 다운로드 요청 결과
//
//	반환값 : 내용 종류, 파일 이름, 본문
//	내용 종류가 JSON 이면 파일로 내려보내고, 아니면 안내 화면(HTML)이다.
func View_baro_legal_privacy_download(config tool.Config, id string) (string, string, string) {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	html := "text/html; charset=utf-8"

	if wall, need := tool.Baro_login_wall(db, config, "/baro/privacy/mine"); need {
		return html, "", wall
	}

	row := baro_legal_privacy_req_get(db, id)
	if row.ID == "" {
		return html, "", baro_legal_not_found(db, config, "/baro/privacy", tool.Baro_t("그런 번호의 개인정보 요청이 없습니다."), "/baro/privacy/mine", tool.Baro_t("내 요청으로"))
	}

	if config.IP != row.User {
		return html, "", tool.Baro_deny(db, config, tool.Baro_t("다운로드는 요청한 본인만 받을 수 있습니다."))
	}

	if row.Kind != "download" {
		return html, "", baro_legal_not_found(db, config, "/baro/privacy", tool.Baro_t("이 요청은 다운로드 요청이 아닙니다."), "/baro/privacy/req/"+row.ID, tool.Baro_t("요청으로 돌아가기"))
	}

	if !baro_legal_result_approved(row.Status, row.Result) {
		body := tool.Baro_alert("warn", tool.Baro_t("아직 승인되지 않은 다운로드 요청입니다. 담당자가 승인하면 파일을 받을 수 있습니다."))
		body += tool.Baro_card("", tool.Baro_button("/baro/privacy/req/"+tool.Url_parser(row.ID), "요청 상태 보기", "primary"))

		return html, "", baro_legal_page(db, config, tool.Baro_t("다운로드"), "/baro/privacy", body)
	}

	if row.Verify != "on" {
		body := tool.Baro_alert("warn", tool.Baro_t("본인 확인이 끝나지 않았습니다. 요청 화면에서 비밀번호 재확인을 먼저 해 주세요."))
		body += tool.Baro_card("", tool.Baro_button("/baro/privacy/req/"+tool.Url_parser(row.ID), "본인 확인하러 가기", "primary"))

		return html, "", baro_legal_page(db, config, tool.Baro_t("다운로드"), "/baro/privacy", body)
	}

	data := baro_legal_privacy_collect(db, row.User)

	tool.Baro_audit(db, config, "privacy_req_download", "baro_privacy_req/"+row.ID, "", "본인 다운로드", "승인된 다운로드 요청")
	tool.Baro_metric_bump(db, "privacy_download", tool.Baro_today(), 1)

	file_name := "barowiki_" + tool.Sha224(row.User)[:12] + "_" + tool.Baro_today() + ".json"

	return "application/json; charset=utf-8", file_name, tool.Baro_json_enc(data)
}

// ------------------------------------------------------------
//  15-2 계정 탈퇴 : /baro/privacy/leave
// ------------------------------------------------------------

// View_baro_legal_privacy_leave : 계정 탈퇴 안내와 접수
func View_baro_legal_privacy_leave(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/privacy/leave"); need {
		return wall
	}

	policy := baro_legal_policy_get(db, "license")
	privacy_policy := baro_legal_policy_get(db, "privacy")
	license_name := tool.Baro_get_def(db, "license", "CC BY-SA 4.0")

	message := ""
	reason := ""

	if values != nil {
		reason = baro_legal_form_raw(values, "reason")
		password := baro_legal_form_raw(values, "password")

		open_count := baro_legal_count(db, "select count(*) from baro_deletion_req where user = ? and "+baro_legal_open_where, config.IP)

		switch {
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 탈퇴 요청을 접수할 수 없습니다."))
		case open_count > 0:
			message = tool.Baro_alert("danger", tool.Baro_t("이미 처리 중인 탈퇴 요청이 있습니다. 내 요청 화면에서 진행 상태를 확인해 주세요."))
		case !baro_legal_form_on(values, "agree_contrib"):
			message = tool.Baro_alert("danger", tool.Baro_t("이미 공개된 정상 기여가 자동으로 삭제되지 않는다는 안내에 동의해야 접수할 수 있습니다."))
		case !baro_legal_form_on(values, "agree_final"):
			message = tool.Baro_alert("danger", tool.Baro_t("계정 탈퇴가 되돌리기 어렵다는 점에 동의해야 접수할 수 있습니다."))
		case tool.Get_len(strings.TrimSpace(reason)) < 2:
			message = tool.Baro_alert("danger", tool.Baro_t("탈퇴 사유를 간단히 적어 주세요. (두 글자 이상)"))
		case password == "" || !tool.Password_check(db, config.IP, password):
			message = tool.Baro_alert("danger", tool.Baro_t("본인 확인을 위해 계정 비밀번호를 정확히 입력해야 합니다."))
		default:
			del_id := tool.Baro_next_id(db, "baro_deletion_req")
			tool.Exec_DB(
				db,
				"insert into baro_deletion_req (id, user, kind, reason, status, date, close_date, handler, result) values (?, ?, 'account', ?, 'open', ?, '', '', '')",
				del_id, config.IP, tool.Baro_trunc(reason, 2000), tool.Baro_now(),
			)

			// 요청 센터에서도 같이 보이도록 개인정보 요청으로도 남긴다
			req_id := tool.Baro_next_id(db, "baro_privacy_req")
			tool.Exec_DB(
				db,
				"insert into baro_privacy_req (id, user, kind, detail, status, handler, date, close_date, verify, result) values (?, ?, 'leave', ?, 'open', '', ?, '', 'on', '')",
				req_id, config.IP,
				"계정 탈퇴 요청 #"+del_id+"\n사유 : "+tool.Baro_trunc(reason, 1000),
				tool.Baro_now(),
			)

			// 탈퇴 안내에 대한 동의를 남긴다
			version := privacy_policy.Version
			if version == "" {
				version = "0"
			}

			tool.Exec_DB(
				db,
				"insert into baro_consent (id, user, kind, version, date, ip) values (?, ?, 'deletion_notice', ?, ?, ?)",
				tool.Baro_next_id(db, "baro_consent"), config.IP, version, tool.Baro_now(), tool.Baro_mask_ip(config.IP),
			)

			tool.Baro_public_log(db, "deletion_req", "system", "", "계정 탈퇴 요청 접수", "")
			tool.Baro_audit(db, config, "deletion_req_new", "baro_deletion_req/"+del_id, "", "open", tool.Baro_trunc(reason, 500))
			tool.Baro_metric_bump(db, "deletion_req", tool.Baro_today(), 1)

			tool.Baro_notify(
				db, config.IP, "system",
				tool.Baro_t("계정 탈퇴 요청이 접수되었습니다"),
				"/baro/privacy/del/"+del_id, "",
			)

			return tool.Get_redirect("/baro/privacy/del/" + del_id)
		}
	}

	body := tool.Baro_crumb([][]string{
		{"/baro/privacy", tool.Baro_t("개인정보 요청 센터")},
		{"", tool.Baro_t("계정 탈퇴")},
	})

	body += message

	body += tool.Baro_hero(
		tool.Baro_t("계정 탈퇴"),
		tool.Baro_t("탈퇴하기 전에 꼭 읽어야 하는 안내가 있습니다."),
		"",
	)

	body += tool.Baro_alert(
		"danger",
		tool.Baro_t(`<strong>탈퇴해도 이미 공개된 정상 기여는 자동으로 삭제되지 않습니다.</strong><br>`)+
			tool.Baro_t(`문서 본문과 편집 이력은 `)+tool.HTML_escape(license_name)+tool.Baro_t(` 라이선스로 이미 공개되어 있고, `)+
			tool.Baro_t(`다른 사람이 그 위에 이어서 편집했을 수 있습니다. 출처 표기 의무 때문에 편집 이력의 계정 이름도 남습니다.`),
	)

	body += tool.Baro_card_ex(
		tool.Baro_t("탈퇴하면 어떻게 되나요"),
		tool.Baro_t("바뀌는 것과 그대로 남는 것을 나눠서 보여 드립니다."),
		tool.Baro_table(
			[]string{tool.Baro_t("항목"), tool.Baro_t("탈퇴 뒤"), tool.Baro_t("이유")},
			[][]string{
				{tool.Baro_t("로그인"), tool.Baro_badge(tool.Baro_t("불가"), "danger"), tool.Baro_t("계정을 닫습니다.")},
				{tool.Baro_t("이메일 · 접속 기록"), tool.Baro_badge("삭제", "ok"), tool.Baro_t("운영에 더 필요하지 않습니다.")},
				{tool.Baro_t("개인 설정 · 관심 분야"), tool.Baro_badge("삭제", "ok"), tool.Baro_t("본인만 쓰는 값입니다.")},
				{tool.Baro_t("문서 본문"), tool.Baro_badge(tool.Baro_t("그대로 유지"), "warn"), tool.Baro_t("라이선스로 이미 공개된 기여입니다. (원칙 2)")},
				{tool.Baro_t("편집 이력의 계정 이름"), tool.Baro_badge(tool.Baro_t("그대로 유지"), "warn"), tool.Baro_t("출처 표기 의무 때문에 지울 수 없습니다.")},
				{tool.Baro_t("토론 · 신고 기록"), tool.Baro_badge(tool.Baro_t("그대로 유지"), "warn"), tool.Baro_t("운영 근거를 남기기 위해 보존합니다. (원칙 5)")},
			},
			"",
		),
		tool.Baro_note(tool.Baro_t("문제가 있는 콘텐츠(저작권 침해 · 개인정보 노출 등)는 탈퇴와 별개로 저작권 요청이나 개인정보 삭제 요청으로 따로 처리합니다. 이용자 제재와 콘텐츠 판단을 섞지 않는 것이 원칙 1입니다.")),
	)

	// 내 기여 규모
	edit_total := baro_legal_count(db, "select count(*) from history where ip = ?", config.IP)
	contrib_total := baro_legal_count(db, "select count(*) from baro_contrib_event where user = ?", config.IP)

	body += tool.Baro_card_ex(
		tool.Baro_t("지금 내 기여"),
		tool.Baro_t("탈퇴해도 아래 기록은 그대로 남습니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("편집 기록"), tool.Baro_comma(edit_total), "history"},
			{"기여 이벤트", tool.Baro_comma(contrib_total), "baro_contrib_event"},
		}),
		tool.Baro_button("/baro/privacy/new?kind=download", tool.Baro_t("탈퇴 전에 내 정보 내려받기"), "primary"),
	)

	if policy.Data != "" {
		body += tool.Baro_card_ex(
			tool.Baro_t("기여 라이선스 본문"),
			tool.Baro_t("탈퇴 안내의 근거가 되는 라이선스 정책입니다. (")+policy.Version+tool.Baro_t("판)"),
			baro_legal_text_html(policy.Data),
			tool.Baro_button("/baro/legal/license", tool.Baro_t("라이선스 안내 자세히 보기"), ""),
		)
	}

	form := tool.Baro_textarea("reason", tool.Baro_t("탈퇴 사유"), reason, 4, tool.Baro_t("예) 더 이상 위키 활동을 하지 않습니다."), tool.Baro_t("담당자만 볼 수 있습니다."))
	form += tool.Baro_checkbox("agree_contrib", tool.Baro_t("이미 공개된 정상 기여와 편집 이력의 계정 이름이 자동으로 삭제되지 않는다는 안내를 읽고 동의합니다."), "on", false, "")
	form += tool.Baro_checkbox("agree_final", tool.Baro_t("계정 탈퇴는 되돌리기 어렵다는 점을 이해합니다."), "on", false, "")
	form += tool.Baro_input_type("password", tool.Baro_t("본인 확인 (계정 비밀번호)"), "", "password", tool.Baro_t("탈퇴 요청은 반드시 비밀번호 재확인을 거칩니다."))
	form += tool.Baro_submit(tool.Baro_t("계정 탈퇴 요청"), "danger")

	body += tool.Baro_danger_zone(tool.Baro_t("탈퇴 요청 접수"), tool.Baro_form("/baro/privacy/leave", form))

	return baro_legal_page(db, config, tool.Baro_t("계정 탈퇴"), "/baro/privacy", body)
}

// ------------------------------------------------------------
//  15-2 탈퇴 요청 상세 / 처리 : /baro/privacy/del/<id>
// ------------------------------------------------------------

// View_baro_legal_privacy_del : 계정 탈퇴 요청 상세
func View_baro_legal_privacy_del(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row := baro_legal_privacy_del_get(db, id)
	if row.ID == "" {
		return baro_legal_not_found(db, config, "/baro/privacy", tool.Baro_t("그런 번호의 계정 탈퇴 요청이 없습니다."), "/baro/privacy/mine", tool.Baro_t("내 요청으로"))
	}

	is_owner := tool.Baro_is_user(config.IP) && config.IP == row.User
	is_handler := tool.Baro_can(db, config.IP, "privacy")

	if !is_owner && !is_handler {
		return tool.Baro_deny(db, config, tool.Baro_t("계정 탈퇴 요청은 본인과 개인정보 담당자만 볼 수 있습니다."))
	}

	message := ""

	if values != nil {
		action := baro_legal_form_get(values, "action")

		switch {
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 처리할 수 없습니다."))
		case action == "cancel" && is_owner:
			if !baro_legal_status_open(row.Status) {
				message = tool.Baro_alert("danger", tool.Baro_t("이미 처리가 끝난 요청은 취소할 수 없습니다."))
				break
			}

			tool.Exec_DB(
				db,
				"update baro_deletion_req set status = 'closed', close_date = ?, result = ? where id = ?",
				tool.Baro_now(), baro_legal_result_enc("", "요청자가 직접 취소했습니다."), row.ID,
			)
			tool.Exec_DB(
				db,
				"update baro_privacy_req set status = 'closed', close_date = ?, result = ? where user = ? and kind = 'leave' and "+baro_legal_open_where,
				tool.Baro_now(), baro_legal_result_enc("", "요청자가 직접 취소했습니다."), row.User,
			)

			tool.Baro_audit(db, config, "deletion_req_cancel", "baro_deletion_req/"+row.ID, row.Status, "closed", "요청자 취소")
			tool.Baro_public_log(db, "deletion_req_close", "system", "", "계정 탈퇴 요청 종료", "요청자 취소")

			return tool.Get_redirect("/baro/privacy/del/" + row.ID)
		case action == "handle" && is_handler:
			status := baro_legal_form_get(values, "status")
			code := baro_legal_form_get(values, "result_code")
			note := baro_legal_form_raw(values, "result_note")

			if !baro_legal_status_ok(status) {
				message = tool.Baro_alert("danger", tool.Baro_t("처리 상태를 골라 주세요."))
				break
			}

			if (status == "done" || status == "rejected") && strings.TrimSpace(note) == "" {
				message = tool.Baro_alert("danger", tool.Baro_t("처리를 끝낼 때는 요청자에게 보여 줄 설명을 반드시 남겨야 합니다."))
				break
			}

			close_date := ""
			if status == "done" || status == "rejected" || status == "closed" {
				close_date = tool.Baro_now()
			}

			tool.Exec_DB(
				db,
				"update baro_deletion_req set status = ?, handler = ?, close_date = ?, result = ? where id = ?",
				status, config.IP, close_date, baro_legal_result_enc(code, tool.Baro_trunc(note, 2000)), row.ID,
			)

			tool.Baro_audit(
				db, config, "deletion_req_handle", "baro_deletion_req/"+row.ID,
				row.Status, status+" / "+code, tool.Baro_trunc(note, 500),
			)
			tool.Baro_public_log(db, "deletion_req_handle", "system", "", "계정 탈퇴 요청 처리", baro_legal_status_name(status))
			tool.Baro_metric_bump(db, "deletion_req_handle", tool.Baro_today(), 1)

			tool.Baro_notify(
				db, row.User, "system",
				tool.Baro_t("계정 탈퇴 요청이 ")+baro_legal_status_name(status)+tool.Baro_t(" 상태로 바뀌었습니다"),
				"/baro/privacy/del/"+row.ID, "",
			)

			return tool.Get_redirect("/baro/privacy/del/" + row.ID)
		default:
			message = tool.Baro_alert("danger", tool.Baro_t("이 동작을 할 권한이 없습니다."))
		}
	}

	row = baro_legal_privacy_del_get(db, id)

	body := tool.Baro_crumb([][]string{
		{"/baro/privacy", tool.Baro_t("개인정보 요청 센터")},
		{"/baro/privacy/mine", tool.Baro_t("내 요청")},
		{"", tool.Baro_t("탈퇴 요청 #") + row.ID},
	})

	body += message

	handler_html := tool.Baro_t(`<span class="bw_muted">아직 배정되지 않음</span>`)
	if row.Handler != "" {
		handler_html = baro_legal_who_cell(db, row.Handler, config.IP)
	}

	body += tool.Baro_hero(
		tool.Baro_t("계정 탈퇴 요청 #")+row.ID,
		"접수 "+row.Date,
		tool.Baro_kv_grid([][]string{
			{tool.Baro_t("요청자"), baro_legal_who_cell(db, row.User, config.IP)},
			{"상태", baro_legal_status_badge(row.Status)},
			{tool.Baro_t("처리자"), handler_html},
			{tool.Baro_t("처리 시각"), tool.Baro_time_cell(row.CloseDate)},
			{tool.Baro_t("처리 결과"), baro_legal_result_cell(row.Result)},
		}),
	)

	body += tool.Baro_card(tool.Baro_t("탈퇴 사유"), baro_legal_quote_html(row.Reason))

	if note := baro_legal_result_note(row.Result); note != "" {
		body += tool.Baro_card(tool.Baro_t("처리 설명"), baro_legal_quote_html(note))
	}

	body += tool.Baro_alert(
		"warn",
		tool.Baro_t("탈퇴 처리는 계정만 닫습니다. 이미 공개된 정상 기여와 편집 이력은 라이선스와 원칙 1 · 2 때문에 자동으로 삭제되지 않습니다."),
	)

	if is_owner && baro_legal_status_open(row.Status) {
		form := tool.Baro_hidden("action", "cancel")
		form += tool.Baro_submit(tool.Baro_t("탈퇴 요청 취소"), "")

		body += tool.Baro_card_ex(
			tool.Baro_t("요청 취소"),
			tool.Baro_t("아직 처리되지 않은 탈퇴 요청은 직접 취소할 수 있습니다."),
			tool.Baro_form("/baro/privacy/del/"+tool.Url_parser(row.ID), form),
			"",
		)
	}

	if is_handler {
		status_options := [][]string{}
		for _, status_row := range baro_legal_status_list() {
			status_options = append(status_options, []string{status_row[0], status_row[1]})
		}

		current_status := row.Status
		if current_status == "" {
			current_status = "open"
		}

		form := tool.Baro_hidden("action", "handle")
		form += tool.Baro_select("status", tool.Baro_t("처리 상태"), status_options, current_status, "")
		form += tool.Baro_select("result_code", tool.Baro_t("처리 결과"), baro_legal_result_list(), baro_legal_result_code(row.Result), "")
		form += tool.Baro_textarea("result_note", tool.Baro_t("요청자에게 보여 줄 설명"), baro_legal_result_note(row.Result), 5, tool.Baro_t("예) 계정을 닫았습니다. 편집 이력은 라이선스에 따라 그대로 남습니다."), tool.Baro_t("이 설명은 요청자에게 그대로 보입니다."))
		form += tool.Baro_submit(tool.Baro_t("처리 기록 저장"), "primary")

		body += tool.Baro_card_ex(
			tool.Baro_t("담당자 처리"),
			tool.Baro_t("계정을 닫는 실제 작업은 위키 관리 화면에서 하고, 여기에는 처리 기록만 남깁니다."),
			tool.Baro_form("/baro/privacy/del/"+tool.Url_parser(row.ID), form),
			tool.Baro_note(tool.Baro_t("이 화면에는 '이 사용자 문서 전부 삭제' 같은 단추를 두지 않습니다. 제재와 콘텐츠 판단을 분리하는 원칙 1 때문입니다.")),
		)
	}

	return baro_legal_page(db, config, tool.Baro_t("계정 탈퇴 요청 #")+row.ID, "/baro/privacy", body)
}
