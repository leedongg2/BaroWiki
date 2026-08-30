package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  16장 관리자 백오피스 — 공통 부품
//
//  이 파일의 비공개 식별자는 모두 baro_admin_ 로 시작한다.
//  다른 모듈의 Go 함수는 부르지 않고 표를 직접 SQL 로 읽는다.
// ============================================================

// baro_admin_menu : 관리자 화면 공통 메뉴 (Get_template 용, 주소는 앞 / 없이)
func baro_admin_menu() [][]any {
	return [][]any{
		{"baro/admin", tool.Baro_t("운영 대시보드")},
		{"baro/admin/map", tool.Baro_t("관리자 화면 지도")},
		{"baro", "바로위키"},
	}
}

// baro_admin_tabs : 백오피스 상단 탭
func baro_admin_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/admin", tool.Baro_t("대시보드")},
		{"/baro/admin/user", tool.Baro_t("사용자")},
		{"/baro/admin/doc", "문서"},
		{"/baro/admin/case", tool.Baro_t("신고·사건")},
		{"/baro/admin/project", "프로젝트"},
		{"/baro/admin/founding", tool.Baro_t("창립 시즌")},
		{"/baro/admin/award", tool.Baro_t("어워드")},
		{"/baro/admin/flag", tool.Baro_t("기능 플래그")},
		{"/baro/admin/map", tool.Baro_t("화면 지도")},
	}, active)
}

// baro_admin_gate : 백오피스 공통 진입 검사
//
//	로그인하지 않았으면 로그인 안내, 권한이 없으면 Baro_deny 화면을 200 으로 돌려준다.
//	두 번째 값이 true 면 첫 번째 값을 그대로 반환해야 한다.
func baro_admin_gate(db *sql.DB, config tool.Config, capability string) (string, bool) {
	// 권한 검사를 먼저 한다. 위키 설정에 따라 로그인하지 않은 접속에도
	// 관리자 권한이 열려 있을 수 있으므로 로그인 안내보다 앞에 둔다.
	if capability != "" && tool.Baro_can(db, config.IP, capability) {
		return "", false
	}

	if tool.Baro_is_admin(db, config.IP) {
		return "", false
	}

	if html, need := tool.Baro_login_wall(db, config, "/baro/admin"); need {
		return html, true
	}

	return tool.Baro_deny(db, config, tool.Baro_t("관리자 백오피스를 사용할 권한이 없습니다. 필요한 역할을 부여받아야 합니다.")), true
}

// baro_admin_page : 백오피스 표준 페이지 마감
func baro_admin_page(db *sql.DB, config tool.Config, title string, body string) string {
	return tool.Baro_page(db, config, title, body, baro_admin_menu())
}

// baro_admin_esc : 사용자 입력을 화면에 넣기 전 반드시 거치는 escape
func baro_admin_esc(data string) string {
	return tool.HTML_escape(data)
}

// baro_admin_get : url.Values 가 nil 이어도 안전하게 값 읽기
func baro_admin_get(values url.Values, key string) string {
	if values == nil {
		return ""
	}

	return strings.TrimSpace(values.Get(key))
}

// baro_admin_page_num : 쪽 번호 (1 미만이면 1)
func baro_admin_page_num(values url.Values) int {
	page := tool.Str_to_int(baro_admin_get(values, "page"))
	if page < 1 {
		page = 1
	}

	return page
}

// baro_admin_rows : 여러 칸을 문자열로 읽어 온다 (NULL 은 빈 문자열)
func baro_admin_rows(db *sql.DB, column_num int, query string, args ...any) [][]string {
	out := [][]string{}
	if column_num < 1 {
		return out
	}

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	for rows.Next() {
		cell := make([]sql.NullString, column_num)
		ptr := make([]any, column_num)
		for i := range cell {
			ptr[i] = &cell[i]
		}

		if err := rows.Scan(ptr...); err != nil {
			continue
		}

		line := make([]string, column_num)
		for i, one := range cell {
			if one.Valid {
				line[i] = one.String
			}
		}

		out = append(out, line)
	}

	return out
}

// baro_admin_one : 한 칸만 읽어 온다
func baro_admin_one(db *sql.DB, query string, args ...any) string {
	out := sql.NullString{}
	tool.QueryRow_DB(db, query, []any{&out}, args...)

	if !out.Valid {
		return ""
	}

	return out.String
}

// baro_admin_num : 개수 세기
func baro_admin_num(db *sql.DB, query string, args ...any) int {
	return tool.Str_to_int(baro_admin_one(db, query, args...))
}

// baro_admin_min : 두 수 중 작은 값
func baro_admin_min(a int, b int) int {
	if a < b {
		return a
	}

	return b
}

// baro_admin_stat_card : 숫자 + 이동 단추 카드 (기한 초과가 있으면 빨갛게)
func baro_admin_stat_card(label string, value int, sub string, target string, overdue int) string {
	body := tool.Baro_stat(label, tool.Baro_comma(value), sub)
	if overdue > 0 {
		body += tool.Baro_alert("danger", tool.Baro_t("처리 기한이 지난 항목 ")+baro_admin_esc(tool.Baro_comma(overdue))+tool.Baro_t("건"))
	}

	return tool.Baro_card_ex("", "", body, tool.Baro_button(target, tool.Baro_t("자세히 보기"), ""))
}

// baro_admin_actor : 조치자 표시 (IP 는 가림)
func baro_admin_actor(db *sql.DB, config tool.Config, who string) string {
	if who == "" {
		return `<span class="bw_muted">-</span>`
	}

	if tool.Baro_is_user(who) {
		return tool.Baro_user_link(db, who, config.IP)
	}

	return baro_admin_esc(tool.Baro_mask_ip(who))
}

// baro_admin_deadline_cell : 기한 칸 (지났으면 빨간 배지)
func baro_admin_deadline_cell(deadline string) string {
	if strings.TrimSpace(deadline) == "" {
		return tool.Baro_t(`<span class="bw_muted">기한 없음</span>`)
	}

	if tool.Baro_is_past(deadline) {
		return tool.Baro_badge(tool.Baro_t("기한 초과"), "danger") + " " + baro_admin_esc(tool.Baro_date_only(deadline))
	}

	return baro_admin_esc(tool.Baro_date_only(deadline)) + " " + tool.Baro_badge(tool.Baro_duration_left(deadline), "info")
}

// baro_admin_status_badge : 처리 상태 배지
func baro_admin_status_badge(status string) string {
	kind := "muted"
	name := status

	switch status {
	case "new":
		kind, name = "danger", "접수"
	case "pending":
		kind, name = "warn", tool.Baro_t("대기")
	case "open":
		kind, name = "warn", tool.Baro_t("진행")
	case "active":
		kind, name = "info", tool.Baro_t("활성")
	case "review":
		kind, name = "warn", tool.Baro_t("검토 중")
	case "approved":
		kind, name = "info", tool.Baro_t("승인됨")
	case "draft":
		kind, name = "muted", "초안"
	case "done", "closed", "confirmed":
		kind, name = "ok", tool.Baro_t("완료")
	case "rejected":
		kind, name = "danger", tool.Baro_t("반려")
	case "expired":
		kind, name = "muted", tool.Baro_t("만료")
	case "released":
		kind, name = "ok", tool.Baro_t("복구됨")
	case "deleted":
		kind, name = "danger", tool.Baro_t("삭제됨")
	case "undone":
		kind, name = "info", tool.Baro_t("일괄 복구됨")
	case "todo":
		kind, name = "muted", tool.Baro_t("대기")
	case "doing":
		kind, name = "warn", tool.Baro_t("진행 중")
	case "hold":
		kind, name = "warn", tool.Baro_t("보류")
	case "used":
		kind, name = "ok", tool.Baro_t("사용됨")
	case "":
		return `<span class="bw_muted">-</span>`
	}

	return tool.Baro_badge(name, kind)
}

// baro_admin_priority_list : 우선순위 값 (이 저장소의 신고·사건 화면과 같은 0~3 을 쓴다)
func baro_admin_priority_list() [][]string {
	return [][]string{
		{"0", tool.Baro_t("긴급")},
		{"1", tool.Baro_t("높음")},
		{"2", tool.Baro_t("보통")},
		{"3", tool.Baro_t("낮음")},
	}
}

// baro_admin_priority_norm : 예전 값(high/normal/low)도 0~3 으로 맞춘다
func baro_admin_priority_norm(priority string) string {
	switch strings.TrimSpace(priority) {
	case "0", "urgent":
		return "0"
	case "1", "high":
		return "1"
	case "3", "low":
		return "3"
	case "2", "normal", "":
		return "2"
	}

	return "2"
}

// baro_admin_priority_badge : 우선순위 배지
func baro_admin_priority_badge(priority string) string {
	switch baro_admin_priority_norm(priority) {
	case "0":
		return tool.Baro_badge(tool.Baro_t("긴급"), "danger")
	case "1":
		return tool.Baro_badge(tool.Baro_t("높음"), "warn")
	case "3":
		return tool.Baro_badge(tool.Baro_t("낮음"), "muted")
	}

	return tool.Baro_badge(tool.Baro_t("보통"), "info")
}

// baro_admin_priority_order : 목록 정렬용 SQL 조각 (숫자·예전 값 모두 위로 올린다)
func baro_admin_priority_order(column string) string {
	return "case " + column +
		" when '0' then 0 when 'urgent' then 0" +
		" when '1' then 1 when 'high' then 1" +
		" when '3' then 3 when 'low' then 3" +
		" else 2 end"
}

// baro_admin_action_name : 운영 조치 종류를 한국어로
func baro_admin_action_name(kind string) string {
	names := map[string]string{
		"doc_quarantine":  tool.Baro_t("문서 격리"),
		"doc_release":     tool.Baro_t("격리 해제 · 복구"),
		"doc_purge":       tool.Baro_t("보관본 완전 삭제"),
		"doc_move":        tool.Baro_t("문서 이동"),
		"doc_merge":       tool.Baro_t("문서 병합"),
		"doc_protect":     tool.Baro_t("문서 보호"),
		"rev_hide":        tool.Baro_t("리비전 숨김"),
		"report_assign":   tool.Baro_t("신고 배정"),
		"report_status":   tool.Baro_t("신고 상태 변경"),
		"report_merge":    "중복 신고 병합",
		"report_reply":    "신고 공개 답변",
		"case_assign":     tool.Baro_t("사건 배정"),
		"case_public":     tool.Baro_t("사건 공개 답변"),
		"case_close":      tool.Baro_t("사건 종결"),
		"project_approve": tool.Baro_t("프로젝트 승인"),
		"project_reject":  tool.Baro_t("프로젝트 반려"),
		"project_owner":   tool.Baro_t("프로젝트 운영자 지정"),
		"project_status":  tool.Baro_t("프로젝트 상태 변경"),
		"project_finish":  tool.Baro_t("프로젝트 종료"),
		"project_archive": tool.Baro_t("프로젝트 보관"),
		"project_reopen":  tool.Baro_t("프로젝트 재개"),
		"founder_confirm": tool.Baro_t("창립 번호 발급"),
		"founder_revoke":  tool.Baro_t("창립 번호 회수"),
		"founder_hold":    tool.Baro_t("창립 심사 보류"),
		"flag_set":        tool.Baro_t("기능 플래그 저장"),
		"flag_off":        tool.Baro_t("기능 플래그 끄기"),
		"flag_delete":     tool.Baro_t("기능 플래그 삭제"),
		"award_create":    tool.Baro_t("어워드 만들기"),
		"award_status":    tool.Baro_t("어워드 단계 변경"),
		"award_verify":    tool.Baro_t("어워드 자격 검증"),
		"award_announce":  tool.Baro_t("어워드 결과 발표"),
		"privacy_view":    tool.Baro_t("개인정보 열람"),
		"role_grant":      tool.Baro_t("역할 부여"),
		"role_revoke":     tool.Baro_t("역할 회수"),
	}

	if name, ok := names[kind]; ok {
		return name
	}

	return kind
}

// baro_admin_log : 운영 조치 기록 (공개 로그 + 내부 감사 로그)
func baro_admin_log(db *sql.DB, config tool.Config, kind string, target string, reason string, before string, after string) {
	tool.Baro_public_log(db, kind, config.IP, target, reason, after)
	tool.Baro_audit(db, config, kind, target, before, after, reason)
}

// baro_admin_recent_edit_table : 최근 편집 표
func baro_admin_recent_edit_table(db *sql.DB, config tool.Config, limit int) string {
	if limit < 1 {
		limit = 10
	}

	list := baro_admin_rows(
		db, 6,
		"select title, id, date, ip, send, leng from history order by date desc limit ?",
		limit,
	)

	rows := [][]string{}
	for _, one := range list {
		summary := tool.Baro_trunc(tool.Baro_one_line(one[4]), 40)
		if summary == "" {
			summary = "-"
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(one[0]),
			`<a href="/history/` + tool.Url_parser(one[0]) + `">r` + baro_admin_esc(one[1]) + `</a>`,
			tool.Baro_time_cell(one[2]),
			baro_admin_actor(db, config, one[3]),
			baro_admin_esc(summary),
			baro_admin_esc(one[5]),
		})
	}

	return tool.Baro_table(
		[]string{"문서", "판", "시각", tool.Baro_t("편집자"), tool.Baro_t("요약"), tool.Baro_t("크기 변화")},
		rows,
		tool.Baro_t("최근 편집이 없습니다."),
	)
}

// baro_admin_active_user_count : 최근 몇 분 안에 활동한 이용자 수
//
//	편집 기록 · 접속 기록(ua_d) · 바로위키 로그인 기록을 합쳐 중복을 제거한다.
func baro_admin_active_user_count(db *sql.DB, minutes int) (int, []string) {
	since := tool.Baro_time_add(tool.Baro_now(), 0, 0, -minutes)

	seen := map[string]bool{}
	order := []string{}

	add := func(who string) {
		who = strings.TrimSpace(who)
		if who == "" || seen[who] {
			return
		}

		seen[who] = true
		order = append(order, who)
	}

	for _, one := range baro_admin_rows(db, 1, "select distinct ip from history where date >= ? limit 300", since) {
		add(one[0])
	}

	for _, one := range baro_admin_rows(db, 1, "select distinct name from ua_d where today >= ? limit 300", since) {
		add(one[0])
	}

	for _, one := range baro_admin_rows(db, 1, "select distinct user from baro_login_log where date >= ? limit 300", since) {
		add(one[0])
	}

	return len(order), order
}

// View_baro_admin_dash : 16-1 운영 대시보드
func View_baro_admin_dash(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if html, stop := baro_admin_gate(db, config, ""); stop {
		return html
	}

	now := tool.Baro_now()
	today := tool.Baro_today()

	// --- 현재 이용자 (최근 15분) ---
	active_num, active_list := baro_admin_active_user_count(db, 15)

	active_html := []string{}
	for _, who := range active_list {
		if len(active_html) >= 20 {
			break
		}

		active_html = append(active_html, baro_admin_actor(db, config, who))
	}

	// --- 신규 문서 (오늘 만들어진 문서) ---
	new_doc := baro_admin_num(
		db,
		"select count(*) from history where id = '1' and date >= ?",
		today+" 00:00:00",
	)

	// --- 신고 대기 ---
	report_deadline_days := tool.Baro_get_int(db, "report_deadline_days", 7)
	report_wait := 0
	report_over := 0
	for _, one := range baro_admin_rows(db, 2, "select id, date from baro_report where status in ('new', 'open') limit 500") {
		report_wait++
		if tool.Baro_is_past(tool.Baro_time_add(one[1], report_deadline_days, 0, 0)) {
			report_over++
		}
	}

	// --- 격리 문서 ---
	quarantine_num := baro_admin_num(db, "select count(*) from baro_quarantine where status = 'open'")
	quarantine_over := 0
	for _, one := range baro_admin_rows(db, 1, "select release_date from baro_quarantine where status = 'open' limit 500") {
		if one[0] != "" && tool.Baro_is_past(one[0]) {
			quarantine_over++
		}
	}

	// --- 이의제기 ---
	appeal_wait := 0
	appeal_over := 0
	for _, one := range baro_admin_rows(db, 1, "select deadline from baro_appeal where status in ('pending', 'open') limit 500") {
		appeal_wait++
		if one[0] != "" && tool.Baro_is_past(one[0]) {
			appeal_over++
		}
	}

	// --- 시스템 경고 ---
	incident_num := baro_admin_num(db, "select count(*) from baro_incident where status = 'open'")
	incident_high := baro_admin_num(db, "select count(*) from baro_incident where status = 'open' and severity in ('high', 'critical')")

	// --- 사건 큐 ---
	case_open := 0
	case_over := 0
	for _, one := range baro_admin_rows(db, 1, "select deadline from baro_case where status in ('open', 'pending', 'review') limit 500") {
		case_open++
		if one[0] != "" && tool.Baro_is_past(one[0]) {
			case_over++
		}
	}

	// --- 대량 작업 (승인 대기 / 유예 중) ---
	// 대량 작업 : 이 저장소는 승인 대기를 draft, 승인 뒤 유예 중을 approved 로 쓴다
	mass_draft := baro_admin_num(db, "select count(*) from baro_mass_op where status = 'draft'")
	mass_approved := baro_admin_num(db, "select count(*) from baro_mass_op where status = 'approved'")
	mass_wait := mass_draft + mass_approved

	mass_grace := 0
	for _, one := range baro_admin_rows(db, 1, "select grace_until from baro_mass_op where status = 'approved' limit 300") {
		if one[0] != "" && !tool.Baro_is_past(one[0]) {
			mass_grace++
		}
	}

	// --- 상단 요약 ---
	body := tool.Baro_hero(
		tool.Baro_t("운영 대시보드"),
		tool.Baro_t("바로위키 운영에 필요한 지표를 한 화면에 모았습니다. (")+baro_admin_esc(now)+tool.Baro_t(" 기준)"),
		tool.Baro_button("/baro/admin/map", tool.Baro_t("관리자 화면 지도"), "primary")+" "+
			tool.Baro_button("/baro/admin/case", tool.Baro_t("신고·사건 큐"), "")+" "+
			tool.Baro_button("/baro/admin/doc", "문서 관리", ""),
	)

	body += baro_admin_tabs("/baro/admin")

	if tool.Baro_read_only(db) {
		body += tool.Baro_alert("danger", tool.Baro_t("현재 위키가 읽기 전용 상태입니다. 사유 : ")+baro_admin_esc(tool.Baro_read_only_reason(db)))
	}

	total_over := report_over + appeal_over + case_over + quarantine_over
	if total_over > 0 {
		body += tool.Baro_alert("danger", tool.Baro_t("처리 기한이 지난 항목이 모두 <b>")+baro_admin_esc(tool.Baro_comma(total_over))+tool.Baro_t("</b>건 있습니다. 아래 빨간 카드를 먼저 처리하세요."))
	} else {
		body += tool.Baro_alert("ok", tool.Baro_t("처리 기한이 지난 항목이 없습니다."))
	}

	// --- 숫자 카드 ---
	cards := []string{
		baro_admin_stat_card(tool.Baro_t("현재 이용자"), active_num, tool.Baro_t("최근 15분 활동"), "/baro/admin/user", 0),
		baro_admin_stat_card(tool.Baro_t("오늘 신규 문서"), new_doc, today, "/recent_change", 0),
		baro_admin_stat_card(tool.Baro_t("신고 대기"), report_wait, "status = new · open", "/baro/admin/case", report_over),
		baro_admin_stat_card(tool.Baro_t("사건 진행"), case_open, tool.Baro_t("열려 있는 사건"), "/baro/admin/case?tab=case", case_over),
		baro_admin_stat_card(tool.Baro_t("격리 문서"), quarantine_num, "status = open", "/baro/admin/doc?act=quarantine", quarantine_over),
		baro_admin_stat_card(tool.Baro_t("이의제기"), appeal_wait, "status = pending · open", "/baro/appeal", appeal_over),
		baro_admin_stat_card(tool.Baro_t("시스템 경고"), incident_num, tool.Baro_t("열린 장애 기록"), "/baro/status", incident_high),
		baro_admin_stat_card(tool.Baro_t("대량 작업"), mass_wait, tool.Baro_t("승인 대기 ")+tool.Baro_itoa(mass_draft)+tool.Baro_t(" · 유예 중 ")+tool.Baro_itoa(mass_grace), "/baro/mass", 0),
	}

	body += tool.Baro_card(tool.Baro_t("한눈에 보기"), tool.Baro_grid(cards, 4))

	// --- 현재 이용자 상세 ---
	who_body := ""
	if len(active_html) == 0 {
		who_body = tool.Baro_empty(tool.Baro_t("최근 15분 안에 기록된 활동이 없습니다."))
	} else {
		who_body = tool.Baro_list(active_html)
		if active_num > len(active_html) {
			who_body += tool.Baro_note(tool.Baro_t("이 밖에 ") + tool.Baro_itoa(active_num-len(active_html)) + tool.Baro_t("명이 더 있습니다."))
		}
	}

	// --- 대량 작업 목록 ---
	mass_rows := [][]string{}
	for _, one := range baro_admin_rows(
		db, 7,
		"select id, kind, actor, num, status, grace_until, reason from baro_mass_op where status in ('draft', 'approved') order by id + 0 desc limit 10",
	) {
		mass_rows = append(mass_rows, []string{
			`<a href="/baro/mass/view/` + tool.Url_parser(one[0]) + `">#` + baro_admin_esc(one[0]) + `</a>`,
			baro_admin_esc(tool.Baro_mass_kind_name(one[1])),
			baro_admin_actor(db, config, one[2]),
			baro_admin_esc(tool.Baro_comma(tool.Str_to_int(one[3]))),
			baro_admin_status_badge(one[4]),
			baro_admin_deadline_cell(one[5]),
			baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(one[6]), 30)),
		})
	}

	// --- 프로젝트 진행률 ---
	project_body := ""
	project_list := baro_admin_rows(
		db, 6,
		"select id, name, field, done_count, target_count, status from baro_project where status in ('open', 'active') order by id + 0 desc limit 6",
	)

	if len(project_list) == 0 {
		project_body = tool.Baro_empty(tool.Baro_t("진행 중인 프로젝트가 없습니다."))
	} else {
		for _, one := range project_list {
			done := tool.Str_to_int(one[3])
			total := tool.Str_to_int(one[4])

			label := one[1]
			if one[2] != "" {
				label += " (" + tool.Baro_field_name(db, one[2]) + ")"
			}

			if total > 0 {
				project_body += tool.Baro_progress(done, total, label)
			} else {
				project_body += tool.Baro_progress_open(done, 0, label)
			}
		}
	}

	body += tool.Baro_grid([]string{
		tool.Baro_card_ex(tool.Baro_t("현재 이용자"), tool.Baro_t("최근 15분 안에 편집·접속 기록이 있는 사람"), who_body, tool.Baro_button("/baro/admin/user", tool.Baro_t("사용자 관리"), "")),
		tool.Baro_card_ex(tool.Baro_t("프로젝트 진행률"), tool.Baro_t("진행 중인 프로젝트 상위 6개"), project_body, tool.Baro_button("/baro/admin/project", tool.Baro_t("프로젝트 관리"), "")),
	}, 2)

	body += tool.Baro_card_ex(
		tool.Baro_t("최근 편집"),
		tool.Baro_t("가장 최근 편집 10건"),
		baro_admin_recent_edit_table(db, config, 10),
		tool.Baro_button("/recent_change", tool.Baro_t("최근 변경 전체"), ""),
	)

	body += tool.Baro_card_ex(
		tool.Baro_t("승인 대기·유예 중인 대량 작업"),
		tool.Baro_t("대상 수가 ")+tool.Baro_itoa(tool.Baro_mass_threshold(db))+tool.Baro_t("개를 넘는 작업은 2인 승인과 유예를 거칩니다."),
		tool.Baro_table(
			[]string{"번호", "종류", tool.Baro_t("신청자"), tool.Baro_t("대상 수"), "상태", tool.Baro_t("유예 만료"), "사유"},
			mass_rows,
			tool.Baro_t("대기 중인 대량 작업이 없습니다."),
		),
		tool.Baro_button("/baro/mass", tool.Baro_t("대량 작업 전체"), ""),
	)

	body += tool.Baro_note(tool.Baro_t("모든 숫자는 표를 직접 읽어 계산합니다. 각 카드의 '자세히 보기'로 해당 화면으로 이동할 수 있습니다."))

	return baro_admin_page(db, config, tool.Baro_t("운영 대시보드"), body)
}

// View_baro_admin_map : 16-9 관리자 화면 지도
func View_baro_admin_map(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if html, stop := baro_admin_gate(db, config, ""); stop {
		return html
	}

	// {주소, 이름, 설명, 담당}
	group_list := [][]any{
		{tool.Baro_t("운영 기본"), [][]string{
			{"/baro/admin", tool.Baro_t("운영 대시보드"), tool.Baro_t("현재 이용자·최근 편집·대기 큐를 한 화면에서 확인합니다.")},
			{"/baro/admin/case", tool.Baro_t("신고 큐"), tool.Baro_t("접수된 신고를 우선순위·긴급도로 정리하고 담당자를 배정합니다.")},
			{"/baro/report", tool.Baro_t("신고 접수 화면"), tool.Baro_t("이용자가 신고를 넣는 화면입니다.")},
			{"/baro/case", tool.Baro_t("사건 상세"), tool.Baro_t("사건 번호별 조치·증거·공개 요약을 봅니다.")},
		}},
		{tool.Baro_t("문서 보호"), [][]string{
			{"/baro/admin/doc", "문서 관리", tool.Baro_t("삭제·복구·보호·이동·병합·격리·리비전 숨김을 처리합니다.")},
			{"/baro/admin/doc?act=quarantine", tool.Baro_t("격리 문서"), tool.Baro_t("삭제 대신 임시 격리된 문서 목록입니다.")},
			{"/baro/quality", tool.Baro_t("문서 품질"), tool.Baro_t("문서 상태와 검토 큐를 봅니다.")},
			{"/baro/review", tool.Baro_t("검토 큐"), tool.Baro_t("위험도 높은 편집을 검토합니다.")},
			{"/trash", tool.Baro_t("휴지통"), tool.Baro_t("openNAMU 기본 휴지통입니다.")},
		}},
		{tool.Baro_t("대량 작업"), [][]string{
			{"/baro/mass", tool.Baro_t("대량 작업 목록"), tool.Baro_t("2인 승인·스냅샷·유예를 거치는 작업 목록입니다.")},
			{"/baro/mass/new", tool.Baro_t("대량 작업 신청"), tool.Baro_t("대상 수가 기준을 넘는 조치는 여기로 보냅니다.")},
		}},
		{tool.Baro_t("사람"), [][]string{
			{"/baro/admin/user", tool.Baro_t("사용자 관리"), tool.Baro_t("계정 상태·권한·제재 이력·의심 활동을 봅니다.")},
			{"/baro/role", tool.Baro_t("권한 관리"), tool.Baro_t("운영자 역할을 나누어 부여·회수합니다.")},
			{"/baro/appeal", tool.Baro_t("이의제기"), tool.Baro_t("제재에 대한 이의제기를 심사합니다.")},
			{"/baro/admin/founding", tool.Baro_t("창립단 관리"), tool.Baro_t("지원자·초대 코드·번호 발급을 관리합니다.")},
		}},
		{tool.Baro_t("프로젝트 · 어워드"), [][]string{
			{"/baro/admin/project", tool.Baro_t("프로젝트 관리"), tool.Baro_t("승인·운영자 지정·종료·부정 점수 감지.")},
			{"/baro/admin/award", tool.Baro_t("어워드 관리"), tool.Baro_t("후보 자격 검증과 투표·심사 진행.")},
			{"/baro/project", tool.Baro_t("프로젝트 목록"), tool.Baro_t("이용자가 보는 프로젝트 화면입니다.")},
		}},
		{tool.Baro_t("법무 · 시스템"), [][]string{
			{"/baro/legal", tool.Baro_t("법적 요청"), tool.Baro_t("수사기관·법원 요청 처리 기록입니다.")},
			{"/baro/privacy", tool.Baro_t("개인정보 요청"), tool.Baro_t("열람·정정·삭제 요청 처리 화면입니다.")},
			{"/baro/copyright", tool.Baro_t("저작권 요청"), tool.Baro_t("저작권 침해 신고 처리 화면입니다.")},
			{"/baro/log", tool.Baro_t("공개 운영 로그"), tool.Baro_t("누가 무엇을 왜 했는지 공개하는 기록입니다.")},
			{"/baro/security/audit", tool.Baro_t("시스템 감사 로그"), tool.Baro_t("누가·언제·무엇을·어떤 값에서 어떤 값으로 바꿨는지 남기는 내부 기록입니다.")},
			{"/baro/security", tool.Baro_t("계정 보안"), tool.Baro_t("2단계 인증과 로그인 기록 설정입니다.")},
			{"/baro/status", tool.Baro_t("장애 · 점검"), tool.Baro_t("장애 기록과 읽기 전용 전환입니다.")},
			{"/baro/admin/flag", tool.Baro_t("기능 플래그"), tool.Baro_t("기능을 켜고 끄는 스위치입니다.")},
			{"/baro/metrics", tool.Baro_t("분석 지표"), tool.Baro_t("지표 대시보드입니다.")},
		}},
	}

	body := tool.Baro_hero(tool.Baro_t("관리자 화면 지도"), tool.Baro_t("16장이 요구하는 관리자 화면 전부로 가는 링크 모음입니다."), "")
	body += baro_admin_tabs("/baro/admin/map")
	body += tool.Baro_note(tool.Baro_t("일부 화면은 다른 모듈이 담당합니다. 아직 준비되지 않은 주소는 위키의 기본 안내 화면이 대신 뜹니다."))

	for _, group := range group_list {
		name, _ := group[0].(string)
		items, _ := group[1].([][]string)

		cards := []string{}
		for _, item := range items {
			cards = append(cards, tool.Baro_card_ex(
				item[1],
				item[2],
				tool.Baro_code(item[0]),
				tool.Baro_button(item[0], "이동", "primary"),
			))
		}

		body += tool.Baro_section(name, tool.Baro_grid(cards, 3))
	}

	return baro_admin_page(db, config, tool.Baro_t("관리자 화면 지도"), body)
}
