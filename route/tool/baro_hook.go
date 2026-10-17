package tool

import (
	"database/sql"
	"regexp"
	"strings"
)

// ============================================================
//  바로위키 편집 연결 고리
//
//  이 파일이 없으면 바로위키 화면들은 "구경만 하는 관리 화면" 이 된다.
//  실제 편집이 일어날 때 기여 이벤트 · 출처 · 검토 큐 · 지표가 여기서 채워진다.
// ============================================================

var (
	baro_footnote_re = regexp.MustCompile(`\[\*[^\]]*\]`)
	baro_category_re = regexp.MustCompile(`(?i)\[\[\s*(?:category|분류)\s*:`)
	baro_url_re      = regexp.MustCompile(`https?://[^\s\]\[<>"']+`)
)

// Baro_count_footnote : 본문의 각주 개수
func Baro_count_footnote(data string) int {
	return len(baro_footnote_re.FindAllString(data, -1))
}

// Baro_count_category : 본문의 분류 개수
func Baro_count_category(data string) int {
	return len(baro_category_re.FindAllString(data, -1))
}

// Baro_extract_urls : 본문에 있는 바깥 링크 목록 (중복 제거)
func Baro_extract_urls(data string) []string {
	seen := map[string]bool{}
	out := []string{}

	for _, url := range baro_url_re.FindAllString(data, -1) {
		url = strings.TrimRight(url, ".,)")
		if url == "" || seen[url] {
			continue
		}

		seen[url] = true
		out = append(out, url)
	}

	return out
}

// Baro_classify_edit : 편집 하나를 기여 행동 종류로 분류한다 (4번 원칙)
//
//	편집 횟수를 그대로 점수로 쓰지 않기 위해, 무슨 일을 했는지를 먼저 정한다.
func Baro_classify_edit(old_data string, new_data string, send string, rev string) string {
	if rev == "" || rev == "1" || strings.TrimSpace(old_data) == "" {
		return "new_document"
	}

	old_len := Get_len(old_data)
	new_len := Get_len(new_data)

	diff := new_len - old_len
	if diff < 0 {
		diff = -diff
	}

	old_note := Baro_count_footnote(old_data)
	new_note := Baro_count_footnote(new_data)
	if new_note > old_note {
		return "source"
	}

	old_category := Baro_count_category(old_data)
	new_category := Baro_count_category(new_data)
	if old_category != new_category && diff < 120 {
		return "category"
	}

	if diff <= 3 {
		return "typo"
	}

	if new_len-old_len >= 300 {
		return "expand"
	}

	// 편집 요약에 드러난 의도도 참고한다
	lower_send := strings.ToLower(send)
	switch {
	case strings.Contains(send, "오타"):
		return "typo"
	case strings.Contains(send, "출처"), strings.Contains(send, "각주"):
		return "source"
	case strings.Contains(send, "분류"):
		return "category"
	case strings.Contains(send, "보강"), strings.Contains(send, "추가"):
		return "expand"
	case strings.Contains(lower_send, "import"), strings.Contains(send, "가져오기"):
		return "import"
	}

	return "fix"
}

// Baro_sync_citation : 문서 본문에서 출처를 뽑아 baro_citation 을 갱신한다
func Baro_sync_citation(db *sql.DB, title string, rev string, data string) {
	urls := Baro_extract_urls(data)

	known := map[string]bool{}
	rows := Query_DB(db, "select url from baro_citation where title = ?", title)
	defer rows.Close()

	for rows.Next() {
		url := ""
		if err := rows.Scan(&url); err != nil {
			continue
		}

		known[url] = true
	}
	rows.Close()

	for _, url := range urls {
		if known[url] {
			continue
		}

		Exec_DB(
			db,
			"insert into baro_citation (id, title, rev, kind, url, author, publisher, pub_date, access_date, archive_url, quality, status, date) values (?, ?, ?, ?, ?, '', ?, '', ?, '', ?, 'new', ?)",
			Baro_next_id(db, "baro_citation"), title, rev,
			baro_citation_kind(url), url, baro_citation_host(url),
			Baro_today(), baro_citation_quality(url), Baro_now(),
		)
	}
}

// baro_citation_host : URL 의 도메인
func baro_citation_host(url string) string {
	rest := url
	for _, prefix := range []string{"https://", "http://"} {
		rest = strings.TrimPrefix(rest, prefix)
	}

	if index := strings.Index(rest, "/"); index > 0 {
		rest = rest[:index]
	}

	return strings.TrimPrefix(rest, "www.")
}

// baro_citation_kind : 출처 종류 추정
func baro_citation_kind(url string) string {
	host := strings.ToLower(baro_citation_host(url))

	switch {
	case strings.Contains(host, "archive.org"):
		return "archive"
	case strings.Contains(host, "news") || strings.Contains(host, "chosun") ||
		strings.Contains(host, "hani") || strings.Contains(host, "khan") ||
		strings.Contains(host, "donga") || strings.Contains(host, "joins") ||
		strings.Contains(host, "yna.co.kr") || strings.Contains(host, "ohmynews"):
		return "news"
	case strings.Contains(host, "riss") || strings.Contains(host, "kiss") ||
		strings.Contains(host, "dbpia") || strings.Contains(host, "scholar") ||
		strings.Contains(host, "doi.org"):
		return "paper"
	case strings.Contains(host, "books.google") || strings.Contains(host, "aladin") ||
		strings.Contains(host, "yes24") || strings.Contains(host, "kyobo"):
		return "book"
	case strings.HasSuffix(host, ".go.kr") || strings.HasSuffix(host, ".or.kr"):
		return "official"
	}

	return "web"
}

// baro_citation_quality : 신뢰도 낮은 출처 표시 (9-4)
func baro_citation_quality(url string) string {
	host := strings.ToLower(baro_citation_host(url))

	for _, weak := range []string{
		"blog.", "tistory.com", "blogspot.", "cafe.naver.com", "cafe.daum.net",
		"namu.wiki", "wikipedia.org", "dcinside.com", "fmkorea.com", "ruliweb.com",
		"post.naver.com", "brunch.co.kr", "velog.io",
	} {
		if strings.Contains(host, weak) {
			return "low"
		}
	}

	if strings.HasSuffix(host, ".go.kr") {
		return "primary"
	}

	return ""
}

// Baro_edit_risk : 편집의 위험 점수(0~100)와 위험 신호 목록 (9-2 위험 기반 검토)
func Baro_edit_risk(db *sql.DB, config Config, doc_name string, old_data string, new_data string, send string) (int, []string) {
	risk := 0
	signals := []string{}

	old_len := Get_len(old_data)
	new_len := Get_len(new_data)

	if !Baro_is_user(config.IP) {
		risk += 25
		signals = append(signals, Baro_t("비로그인 IP 편집"))
	} else {
		register_info := Get_user_register_info(db, config.IP)
		if register_info["date"] != "" {
			days := Baro_days_between(register_info["date"], Baro_now())
			limit := Baro_get_int(db, "review_risk_new_user_days", 30)
			if days >= 0 && days < limit {
				risk += 20
				signals = append(signals, "가입 "+Baro_itoa(limit)+Baro_t("일 이내 사용자"))
			}
		}
	}

	big := Baro_get_int(db, "review_big_edit_length", 3000)
	if new_len-old_len >= big {
		risk += 20
		signals = append(signals, Baro_t("대규모 추가 편집"))
	}

	// 문서가 작아도 대부분이 사라지면 위험하다 (짧은 문서를 통째로 비우는 경우)
	if old_len > 100 && new_len*10 < old_len*3 {
		risk += 35
		signals = append(signals, Baro_t("본문 대부분 삭제 (")+Baro_itoa(old_len)+Baro_t("자 → ")+Baro_itoa(new_len)+Baro_t("자)"))
	}

	if strings.TrimSpace(new_data) == "" && old_len > 0 {
		risk += 45
		signals = append(signals, Baro_t("본문 전체 삭제"))
	}

	if old_len == 0 && Baro_count_footnote(new_data) == 0 && new_len > 300 {
		risk += 15
		signals = append(signals, Baro_t("출처 없는 새 문서"))
	}

	if strings.TrimSpace(send) == "" {
		risk += 5
		signals = append(signals, Baro_t("편집 요약 없음"))
	}

	if len(Baro_extract_urls(new_data))-len(Baro_extract_urls(old_data)) >= 10 {
		risk += 20
		signals = append(signals, Baro_t("링크 대량 추가"))
	}

	// 편집 필터에 걸린 편집은 그 자체가 위험 신호다
	if hits := Baro_filter_recent_hits(db, config.IP, doc_name); len(hits) > 0 {
		risk += 30
		signals = append(signals, "편집 필터 적발 ("+strings.Join(hits, ", ")+")")
	}

	if risk > 100 {
		risk = 100
	}

	return risk, signals
}

// Baro_filter_check : 바로위키 편집 필터 검사
//
//	반환값 : (막혔는지, 사유)  — action 이 review 이면 막지 않고 검토 큐로만 보낸다.
func Baro_filter_check(db *sql.DB, config Config, doc_name string, data string, send string) (bool, string) {
	// 필터 면제 권한이 있으면 통과
	if Check_acl(db, "", "", "hide_document_history", config.IP) && Baro_is_admin(db, config.IP) {
		return false, ""
	}

	// 규칙을 먼저 모두 읽어 커서를 닫는다.
	// 조건식 · 속도 규칙은 다른 표를 또 봐야 하는데, 커서를 연 채 다른 질의를 부르면
	// 연결이 하나뿐인 환경에서 막힐 수 있다.
	type filter_rule struct {
		id, name, pattern, kind, action string
	}

	rules := []filter_rule{}
	rows := Query_DB(db, "select id, name, pattern, kind, action from baro_edit_filter where status = 'on'")
	for rows.Next() {
		one := filter_rule{}
		if err := rows.Scan(&one.id, &one.name, &one.pattern, &one.kind, &one.action); err != nil {
			continue
		}

		rules = append(rules, one)
	}
	rows.Close()

	if len(rules) == 0 {
		return false, ""
	}

	type hit struct {
		id, name, action, detail string
	}

	hits := []hit{}
	blocked := ""

	// 조건식 · 속도 규칙은 고치기 전 본문과 최근 편집 횟수도 본다.
	// 그런 규칙이 하나라도 있을 때만 읽어 온다 (없으면 헛일이다).
	old_data := ""
	recent_count := 0
	for _, one := range rules {
		if one.kind != "expr" && one.kind != "rate" {
			continue
		}

		QueryRow_DB(db, "select data from data where title = ?", []any{&old_data}, doc_name)
		QueryRow_DB(
			db,
			"select count(*) from history where ip = ? and date >= ?",
			[]any{&recent_count}, config.IP, Baro_time_add(Baro_now(), 0, 0, -5),
		)

		break
	}

	for _, one := range rules {
		target := ""
		switch one.kind {
		case "content":
			target = data
		case "send":
			target = send
		case "title":
			target = doc_name
		case "empty":
			if strings.TrimSpace(data) == "" {
				hits = append(hits, hit{one.id, one.name, one.action, Baro_t("본문이 비어 있음")})
			}
			continue
		case "rate":
			if limit := Str_to_int(one.pattern); limit > 0 && recent_count >= limit {
				hits = append(hits, hit{one.id, one.name, one.action, Baro_t("5분 안 편집 ") + Baro_itoa(recent_count) + Baro_t("회")})
			}
			continue
		case "expr":
			env := Baro_filter_env_make(Namespace_of(doc_name).Name, doc_name, config.IP, data, old_data, send, recent_count)
			matched, detail, expr_error := Baro_filter_expr_run(one.pattern, env)
			if expr_error != "" || !matched {
				// 규칙이 잘못 적혀 있으면 편집을 막지 않는다
				continue
			}
			if detail == "" {
				detail = Baro_t("조건식에 맞음")
			}
			hits = append(hits, hit{one.id, one.name, one.action, Baro_trunc(detail, 160)})
			continue
		default:
			continue
		}

		re, err := regexp.Compile(one.pattern)
		if err != nil {
			continue
		}

		if re.MatchString(target) {
			hits = append(hits, hit{one.id, one.name, one.action, ""})
		}
	}

	for _, one := range hits {
		detail := one.name
		if one.detail != "" {
			detail += " — " + one.detail
		}

		Exec_DB(
			db,
			"insert into baro_filter_log (id, filter_id, user, title, action, detail, date) values (?, ?, ?, ?, ?, ?, ?)",
			Baro_next_id(db, "baro_filter_log"), one.id, config.IP, doc_name, one.action, Baro_trunc(detail, 300), Baro_now(),
		)

		count := "0"
		QueryRow_DB(db, "select hits from baro_edit_filter where id = ?", []any{&count}, one.id)
		Exec_DB(db, "update baro_edit_filter set hits = ? where id = ?", Baro_itoa(Str_to_int(count)+1), one.id)

		if one.action == "block" && blocked == "" {
			blocked = one.name
		}
	}

	if blocked != "" {
		return true, blocked
	}

	return false, ""
}

// Baro_hook_before_edit : 편집이 저장되기 전에 부른다
//
//	막아야 하면 (사유, false) 를 돌려준다.
func Baro_hook_before_edit(db *sql.DB, config Config, doc_name string, data string, send string) (string, bool) {
	// 점검 중 (17-4)
	if Baro_read_only(db) && !Baro_is_admin(db, config.IP) {
		return Baro_t("지금은 읽기 전용 상태입니다. ") + Baro_read_only_reason(db), false
	}

	// 격리된 문서는 담당자만 손댈 수 있다 (원칙 2)
	if Baro_doc_quarantined(db, doc_name) && !Baro_can(db, config.IP, "quarantine") {
		return Baro_t("격리 검토 중인 문서입니다. 검토가 끝나면 다시 편집할 수 있습니다."), false
	}

	// 제재는 사람에게만 적용된다 (원칙 1)
	if Baro_is_user(config.IP) {
		if Baro_sanction_has(db, config.IP, "doc_limit", doc_name) {
			return Baro_t("이 문서에 대한 편집이 제한되어 있습니다. 사건 기록에서 사유를 확인할 수 있습니다."), false
		}

		field := Baro_doc_field(db, doc_name)
		if field != "" && Baro_sanction_has(db, config.IP, "field_limit", field) {
			return Baro_t("이 분야에 대한 편집이 제한되어 있습니다."), false
		}

		if Baro_sanction_has(db, config.IP, "slow", "") {
			if !Baro_rate_ok(db, "sanction_slow", config.IP, 1, 600) {
				return Baro_t("편집 속도 제한이 적용 중입니다. 10분에 한 번만 편집할 수 있습니다."), false
			}
		}
	}

	// 바로위키 편집 필터 (9-6)
	if blocked, reason := Baro_filter_check(db, config, doc_name, data, send); blocked {
		return Baro_t("편집 필터에 걸렸습니다 : ") + reason, false
	}

	return "", true
}

// Baro_hook_after_edit : 편집이 저장된 뒤에 부른다
func Baro_hook_after_edit(db *sql.DB, config Config, doc_name string, old_data string, new_data string, send string) {
	rev := Get_document_revision(db, doc_name)

	kind := Baro_classify_edit(old_data, new_data, send, rev)

	// 4번 원칙 : 횟수가 아니라 행동을 기록한다
	Baro_contrib_add(db, config.IP, kind, doc_name, rev, Baro_doc_meta_get(db, doc_name, "project"), Baro_trunc(send, 200))

	// 출처 갱신 (9-4)
	Baro_sync_citation(db, doc_name, rev, new_data)

	// 위험 기반 검토 큐 (9-2)
	risk, signals := Baro_edit_risk(db, config, doc_name, old_data, new_data, send)
	if risk >= Baro_get_int(db, "review_risk_threshold", 30) {
		Exec_DB(
			db,
			"insert into baro_review (id, title, rev, user, kind, status, reviewer, review_date, comment, risk, date) values (?, ?, ?, ?, 'edit', 'pending', '', '', ?, ?, ?)",
			Baro_next_id(db, "baro_review"), doc_name, rev, config.IP,
			strings.Join(signals, " / "), Baro_itoa(risk), Baro_now(),
		)
	}

	// 문서 상태 자동 표시 (9-1)
	if Baro_doc_status_get(db, doc_name) == "" {
		if Baro_count_footnote(new_data) == 0 && Get_len(new_data) > 300 {
			Baro_doc_status_set(db, doc_name, "need_source", "각주가 없는 문서입니다.", "system")
		} else if Get_len(new_data) < 300 {
			Baro_doc_status_set(db, doc_name, "need_expand", "내용이 짧은 문서입니다.", "system")
		}
	}

	// 관심 문서 알림 (2-6 / 8-4)
	Baro_watch_notify(db, config, doc_name)

	// 사용자 문서 알림 : 내 사용자 문서를 누가 고치면 알린다
	Baro_user_doc_notify(db, config.IP, doc_name, "edit", "")

	// 지표 (18장)
	Baro_metric_bump(db, "edit", Baro_today(), 1)
	if kind == "new_document" {
		Baro_metric_bump(db, "new_document", Baro_today(), 1)
	}
}

// Baro_watch_notify : 관심 문서를 보고 있는 사람에게 알린다
func Baro_watch_notify(db *sql.DB, config Config, doc_name string) {
	rows := Query_DB(
		db,
		"select user from baro_watch where kind = 'document' and target = ?",
		doc_name,
	)
	defer rows.Close()

	targets := []string{}
	for rows.Next() {
		user := ""
		if err := rows.Scan(&user); err != nil {
			continue
		}

		if user == config.IP {
			continue
		}

		targets = append(targets, user)
	}
	rows.Close()

	for _, user := range targets {
		Baro_notify(
			db, user, "watch",
			Namespace_display(doc_name)+Baro_t(" 문서가 편집됐습니다."),
			"/w/" + Doc_url(doc_name),
			"",
		)
	}
}

// Baro_doc_banner : 문서를 볼 때 위에 붙는 바로위키 안내 줄
//
//	상태 · 격리 · 안정판 · 출처 부족을 한 줄로 알려 준다.
func Baro_doc_banner(db *sql.DB, config Config, doc_name string) string {
	if doc_name == "" {
		return ""
	}

	html := ""

	if Baro_read_only(db) {
		html += Baro_alert("warn", Baro_t("<b>점검 중</b> — ")+HTML_escape(Baro_read_only_reason(db)))
	}

	if Baro_doc_quarantined(db, doc_name) {
		reason := ""
		QueryRow_DB(
			db,
			"select reason from baro_quarantine where title = ? and status = 'open' limit 1",
			[]any{&reason}, doc_name,
		)

		html += Baro_alert("danger",
			Baro_t(`<b>격리 검토 중인 문서입니다.</b> 내용은 그대로 보존되어 있으며 검토 후 수정·복구·삭제 중 하나로 처리됩니다.`)+
				baro_reason_html(reason)+
				Baro_t(` <a href="/baro/quarantine">격리 목록</a>`))
	}

	status := Baro_doc_status_get(db, doc_name)
	if status != "" && status != "quarantined" {
		reason := ""
		QueryRow_DB(db, "select reason from baro_doc_status where title = ?", []any{&reason}, doc_name)

		kind := Baro_doc_status_kind(status)
		if kind == "ok" || kind == "muted" {
			kind = "info"
		}

		html += Baro_alert(kind,
			`<b>`+HTML_escape(Baro_doc_status_name(status))+`</b>`+baro_reason_html(reason)+
				` <a href="/baro/quality/doc/`+Url_parser(doc_name)+Baro_t(`">품질 정보</a>`))
	}

	stable_rev := ""
	QueryRow_DB(db, "select rev from baro_stable where title = ?", []any{&stable_rev}, doc_name)
	if stable_rev != "" && Baro_flag_on(db, config.IP, "stable_version") {
		now_rev := Get_document_revision(db, doc_name)
		if now_rev != stable_rev {
			html += Baro_alert("info",
				Baro_t(`검토를 마친 <b>안정판</b>이 따로 있습니다. `)+
					`<a href="/render/`+HTML_escape(stable_rev)+`/`+Url_parser(doc_name)+`">안정판 r`+HTML_escape(stable_rev)+Baro_t(` 보기</a>`))
		}
	}

	if html == "" {
		return ""
	}

	return Baro_css() + `<div class="bw_wrap">` + html + `</div>`
}

// baro_reason_html : 사유가 있으면 덧붙인다
func baro_reason_html(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return ""
	}

	return Baro_t(` 사유 : `) + HTML_escape(reason)
}

// Baro_hook_can_act : 편집 말고 다른 쓰기 동작을 할 수 있는지
//
//	kind : discussion / upload / bbs / vote
//	막아야 하면 (사유, false) 를 돌려준다.
func Baro_hook_can_act(db *sql.DB, config Config, kind string) (string, bool) {
	if Baro_read_only(db) && !Baro_is_admin(db, config.IP) {
		return Baro_t("지금은 읽기 전용 상태입니다. ") + Baro_read_only_reason(db), false
	}

	if !Baro_is_user(config.IP) {
		return "", true
	}

	switch kind {
	case "discussion":
		if Baro_sanction_has(db, config.IP, "discussion_limit", "") {
			return Baro_t("토론 참여가 제한되어 있습니다. 사건 기록에서 사유를 확인할 수 있습니다."), false
		}
	case "upload":
		if Baro_sanction_has(db, config.IP, "upload_limit", "") {
			return Baro_t("파일 업로드가 제한되어 있습니다. 사건 기록에서 사유를 확인할 수 있습니다."), false
		}
	}

	return "", true
}

// Baro_doc_hidden : 격리되어 본문을 감춰야 하는 문서인지 (2번 원칙)
//
//	격리는 "지우지 않고 가리는" 조치다. 내용은 baro_quarantine.backup 에 그대로 남지만,
//	검토가 끝날 때까지 일반 이용자에게는 본문을 보여 주지 않는다.
//	격리를 다루는 담당자와 소유자는 검토를 위해 볼 수 있다.
func Baro_doc_hidden(db *sql.DB, config Config, doc_name string) bool {
	if doc_name == "" {
		return false
	}

	if !Baro_doc_quarantined(db, doc_name) {
		return false
	}

	return !Baro_can(db, config.IP, "quarantine")
}

// Baro_filter_recent_hits : 방금 이 편집에서 걸린 편집 필터 이름들
//
//	Baro_filter_check 가 남긴 baro_filter_log 를 되읽는다.
//	action 이 review 인 규칙은 편집을 막지 않는 대신 여기서 위험 신호가 되어 검토 큐로 간다.
func Baro_filter_recent_hits(db *sql.DB, user string, doc_name string) []string {
	out := []string{}

	// 방금 저장한 편집에서 걸린 것만 본다 (1분 안쪽)
	rows := Query_DB(
		db,
		"select detail from baro_filter_log where user = ? and title = ? and date >= ? order by id + 0 desc limit 5",
		user, doc_name, Baro_time_add(Baro_now(), 0, 0, -1),
	)
	defer rows.Close()

	for rows.Next() {
		detail := ""
		if err := rows.Scan(&detail); err != nil {
			continue
		}

		if detail != "" {
			out = append(out, detail)
		}
	}

	return out
}

// Baro_audit_sign : 감사 기록의 위변조 검증용 서명
//
//	담당자·동작·대상·바뀐 값뿐 아니라 사유·접속 정보·권한까지 전부 덮는다.
//	(사유만 몰래 고치는 것을 막기 위해서다)
func Baro_audit_sign(id string, actor string, action string, target string,
	before string, after string, auth string, reason string, ip string, ua string, date string) string {
	return Sha224(strings.Join([]string{
		id, actor, action, target, before, after, auth, reason, ip, ua, date,
	}, ""))
}
