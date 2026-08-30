package route

import (
	"database/sql"
	"net/url"
	"regexp"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  9-6 편집 필터 — /baro/quality/filter
//
//  필터는 사람을 판단하지 않는다. 글자를 볼 뿐이다.
//  그래서 기본 조치는 차단이 아니라 "검토 큐로 보내기" 다.
// ============================================================

// baro_quality_filter_row : 편집 필터 한 줄
type baro_quality_filter_row struct {
	ID      string
	Name    string
	Pattern string
	Kind    string
	Action  string
	Status  string
	Note    string
	Date    string
	Hits    string
}

// baro_quality_filter_kind_options : 규칙 종류
func baro_quality_filter_kind_options() [][]string {
	return [][]string{
		{"content", tool.Baro_t("본문 (정규식)")},
		{"send", tool.Baro_t("편집 요약 (정규식)")},
		{"title", tool.Baro_t("문서 이름 (정규식)")},
		{"empty", tool.Baro_t("전체 삭제 (본문이 비었을 때)")},
		{"rate", tool.Baro_t("속도 (5분 안 편집 횟수)")},
	}
}

// baro_quality_filter_kind_name : 규칙 종류 이름
func baro_quality_filter_kind_name(kind string) string {
	for _, row := range baro_quality_filter_kind_options() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// baro_quality_filter_action_options : 조치 종류
func baro_quality_filter_action_options() [][]string {
	return [][]string{
		{"review", tool.Baro_t("검토 큐로 보내기")},
		{"block", tool.Baro_t("차단 (편집을 막음)")},
		{"log", tool.Baro_t("기록만 남기기")},
	}
}

// baro_quality_filter_action_name : 조치 이름
func baro_quality_filter_action_name(action string) string {
	for _, row := range baro_quality_filter_action_options() {
		if row[0] == action {
			return row[1]
		}
	}

	return action
}

// baro_quality_filter_action_badge : 조치 배지
func baro_quality_filter_action_badge(action string) string {
	switch action {
	case "block":
		return tool.Baro_badge(tool.Baro_t("차단"), "danger")
	case "log":
		return tool.Baro_badge(tool.Baro_t("기록만"), "muted")
	}

	return tool.Baro_badge(tool.Baro_t("검토 큐"), "info")
}

// baro_quality_filter_name : 규칙 번호로 이름 찾기
func baro_quality_filter_name(db *sql.DB, id string) string {
	name := ""
	if !tool.QueryRow_DB(db, "select name from baro_edit_filter where id = ?", []any{&name}, id) {
		return tool.Baro_t("삭제된 규칙 #") + id
	}

	return name
}

// baro_quality_filter_list : 편집 필터 목록
func baro_quality_filter_list(db *sql.DB, only_on bool) []baro_quality_filter_row {
	query := "select id, name, pattern, kind, action, status, note, date, hits from baro_edit_filter"
	if only_on {
		query += " where status = 'on'"
	}
	query += " order by id + 0 asc"

	rows := tool.Query_DB(db, query)
	defer rows.Close()

	out := []baro_quality_filter_row{}
	for rows.Next() {
		row := baro_quality_filter_row{}
		if err := rows.Scan(&row.ID, &row.Name, &row.Pattern, &row.Kind, &row.Action, &row.Status, &row.Note, &row.Date, &row.Hits); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_quality_filter_hit : 규칙 한 개가 이 편집에 걸리는지
//
//	recent 는 그 편집 직전 5분 동안의 편집 횟수 (모르면 0)
func baro_quality_filter_hit(row baro_quality_filter_row, title string, data string, send string, recent int) (bool, string) {
	switch row.Kind {
	case "empty":
		if strings.TrimSpace(data) == "" {
			return true, tool.Baro_t("본문이 비어 있음")
		}

		return false, ""
	case "rate":
		limit := tool.Str_to_int(row.Pattern)
		if limit > 0 && recent >= limit {
			return true, tool.Baro_t("5분 안에 ") + tool.Baro_itoa(recent) + tool.Baro_t("회 편집 (기준 ") + tool.Baro_itoa(limit) + tool.Baro_t("회)")
		}

		return false, ""
	}

	pattern, err := regexp.Compile(row.Pattern)
	if err != nil {
		return false, ""
	}

	subject := data
	switch row.Kind {
	case "send":
		subject = send
	case "title":
		subject = title
	}

	match := pattern.FindString(subject)
	if match == "" {
		if !pattern.MatchString(subject) {
			return false, ""
		}

		return true, tool.Baro_t("빈 문자열에 일치")
	}

	return true, tool.Baro_trunc(tool.Baro_one_line(match), 120)
}

// baro_quality_filter_match : 켜져 있는 규칙 중 이 편집에 걸리는 것들
func baro_quality_filter_match(db *sql.DB, title string, user string, data string, send string, recent int) []baro_quality_filter_row {
	_ = user

	out := []baro_quality_filter_row{}
	for _, row := range baro_quality_filter_list(db, true) {
		hit, _ := baro_quality_filter_hit(row, title, data, send, recent)
		if hit {
			out = append(out, row)
		}
	}

	return out
}

// baro_quality_filter_preset : 권장 규칙 (이름, 정규식, 종류, 조치, 설명)
func baro_quality_filter_preset() [][]string {
	return [][]string{
		{
			"광고 · 홍보 문구",
			`(?i)(카지노|바카라|토토\s*사이트|먹튀|대출\s*상담|비아그라|성인용품|무료\s*체험|지금\s*바로\s*클릭|최저가\s*보장)`,
			"content", "review",
			"상업 광고에 자주 쓰이는 낱말입니다. 문서 주제가 그 낱말 자체일 수 있으니 차단이 아니라 검토로 둡니다.",
		},
		{
			"욕설 도배",
			`((시발|씨발|병신|개새끼|ㅅㅂ|ㅄ|ㅂㅅ)\s*){5,}`,
			"content", "review",
			"같은 욕설이 다섯 번 넘게 이어지는 경우만 봅니다. 인용된 욕설 한두 번은 걸리지 않습니다.",
		},
		{
			"반복 링크 도배",
			`(https?://\S+\s*){10,}`,
			"content", "review",
			"링크가 열 개 넘게 잇달아 붙는 경우입니다. 출처 목록일 수도 있어 검토로 둡니다.",
		},
		{
			"주민등록번호 형식",
			`\d{6}\s*-\s*[1-4]\d{6}`,
			"content", "block",
			"주민등록번호는 어떤 경우에도 게시할 수 없어 차단합니다.",
		},
		{
			"전화번호 노출",
			`0\d{1,2}[-. ]?\d{3,4}[-. ]?\d{4}`,
			"content", "review",
			"공공기관 대표번호처럼 공개해도 되는 번호가 있어 검토로 둡니다.",
		},
		{
			"이메일 주소 노출",
			`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`,
			"content", "review",
			"개인 이메일이 본문에 그대로 들어가지 않게 확인합니다.",
		},
		{
			"계좌번호 형식",
			`\d{2,6}-\d{2,6}-\d{2,8}`,
			"content", "log",
			"계좌번호처럼 보이는 숫자입니다. 오탐이 많아 기록만 남깁니다.",
		},
		{
			"본문 전체 삭제",
			`^$`,
			"empty", "review",
			"본문이 통째로 비워진 편집입니다. 정당한 통합일 수도 있어 검토로 보냅니다.",
		},
		{
			"초고속 반복 편집",
			`10`,
			"rate", "review",
			"5분 안에 10회 넘게 편집하면 검토합니다. 연속 오타 수정일 수도 있습니다.",
		},
		{
			"의심스러운 계정 행동 (요약 도배)",
			`(?i)(asdf|ㅁㄴㅇㄹ|ㅋㅋㅋㅋㅋㅋ|test\s*\d{3,})`,
			"send", "log",
			"편집 요약에 아무 글자나 넣는 행동입니다. 기록만 남겨 흐름을 봅니다.",
		},
		{
			"문서 이름 광고",
			`(?i)(https?://|\.com$|광고|홍보문의)`,
			"title", "review",
			"문서 이름 자체가 광고인 경우입니다.",
		},
	}
}

// baro_quality_filter_guard : 편집 필터 관리 권한 확인 (없으면 안내 화면)
func baro_quality_filter_guard(db *sql.DB, config tool.Config) (string, bool) {
	if wall, need := tool.Baro_login_wall(db, config, "/baro/quality/filter"); need {
		return wall, true
	}

	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("편집 필터는 시스템 관리 권한이 있는 사용자만 다룰 수 있습니다. 규칙이 공개되면 우회가 쉬워지기 때문입니다.")), true
	}

	return "", false
}

// View_baro_quality_filter_list : 편집 필터 목록 (권한 system)
func View_baro_quality_filter_list(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, need := baro_quality_filter_guard(db, config); need {
		return deny
	}

	list := baro_quality_filter_list(db, false)

	on_count, block_count, total_hits := 0, 0, 0
	table_rows := [][]string{}

	for _, row := range list {
		if row.Status == "on" {
			on_count++
		}
		if row.Action == "block" {
			block_count++
		}
		total_hits += tool.Str_to_int(row.Hits)

		state := tool.Baro_badge(tool.Baro_t("꺼짐"), "muted")
		toggle_text := tool.Baro_t("켜기")
		if row.Status == "on" {
			state = tool.Baro_badge(tool.Baro_t("켜짐"), "ok")
			toggle_text = tool.Baro_t("끄기")
		}

		bad_pattern := ""
		if row.Kind != "empty" && row.Kind != "rate" {
			if _, err := regexp.Compile(row.Pattern); err != nil {
				bad_pattern = tool.Baro_badge(tool.Baro_t("정규식 오류"), "danger")
			}
		}

		manage := tool.Baro_form("/baro/quality/filter/toggle/"+tool.Url_parser(row.ID),
			tool.Baro_submit(toggle_text, ""))
		manage += tool.Baro_button("/baro/quality/filter/edit/"+tool.Url_parser(row.ID), "수정", "")
		manage += " " + tool.Baro_button("/baro/quality/filter/delete/"+tool.Url_parser(row.ID), "삭제", "danger")

		table_rows = append(table_rows, []string{
			"#" + tool.HTML_escape(row.ID),
			tool.HTML_escape(row.Name) + " " + bad_pattern,
			tool.HTML_escape(baro_quality_filter_kind_name(row.Kind)),
			tool.Baro_code(tool.Baro_trunc(row.Pattern, 80)),
			baro_quality_filter_action_badge(row.Action),
			state,
			tool.Baro_comma(tool.Str_to_int(row.Hits)),
			manage,
		})
	}

	list_html := tool.Baro_table(
		[]string{"번호", tool.Baro_t("이름"), "종류", tool.Baro_t("정규식 / 기준"), tool.Baro_t("조치"), "상태", tool.Baro_t("적발"), "관리"},
		table_rows, "")

	if len(list) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("등록된 편집 필터가 없습니다. 아래 권장 규칙에서 골라 넣을 수 있습니다."))
	}

	// 권장 규칙
	preset_rows := [][]string{}
	for _, row := range baro_quality_filter_preset() {
		exists := ""
		tool.QueryRow_DB(db, "select id from baro_edit_filter where name = ?", []any{&exists}, row[0])

		action_cell := tool.Baro_badge(tool.Baro_t("이미 등록됨"), "muted")
		if exists == "" {
			action_cell = tool.Baro_form("/baro/quality/filter/add",
				tool.Baro_hidden("preset", row[0])+tool.Baro_submit("추가", "primary"))
		}

		preset_rows = append(preset_rows, []string{
			tool.HTML_escape(row[0]),
			tool.HTML_escape(baro_quality_filter_kind_name(row[2])),
			tool.Baro_code(tool.Baro_trunc(row[1], 60)),
			baro_quality_filter_action_badge(row[3]),
			tool.HTML_escape(row[4]),
			action_cell,
		})
	}

	log_count := baro_quality_count(db, "select count(*) from baro_filter_log")

	body := baro_quality_crumb(tool.Baro_t("편집 필터"))
	body += tool.Baro_hero(
		tool.Baro_t("편집 필터"),
		tool.Baro_t("정해 둔 글자 규칙에 걸리는 편집을 골라 냅니다."),
		tool.Baro_button("/baro/quality/filter/add", tool.Baro_t("규칙 추가"), "primary")+" "+
			tool.Baro_button("/baro/quality/filter/test", tool.Baro_t("정규식 시험"), "")+" "+
			tool.Baro_button("/baro/quality/filter/log", tool.Baro_t("적발 기록"), ""),
	)
	body += baro_quality_nav("/baro/quality/filter")

	body += tool.Baro_alert("info",
		tool.Baro_t("기본 조치는 <b>검토 큐로 보내기</b> 입니다. 차단은 주민등록번호처럼 어떤 경우에도 게시할 수 없는 것에만 씁니다. ")+
			tool.Baro_t("규칙이 사람을 판단하지 않게 하려면, 애매한 것은 사람이 보게 하는 편이 낫습니다."))

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 규칙"), tool.Baro_comma(len(list)), ""},
		{tool.Baro_t("켜진 규칙"), tool.Baro_comma(on_count), ""},
		{tool.Baro_t("차단 규칙"), tool.Baro_comma(block_count), tool.Baro_t("꼭 필요한 것만")},
		{tool.Baro_t("누적 적발"), tool.Baro_comma(total_hits), ""},
		{tool.Baro_t("적발 기록"), tool.Baro_comma(log_count), "baro_filter_log"},
	})

	body += tool.Baro_card_ex(tool.Baro_t("규칙 목록"), tool.Baro_t("번호 순서입니다."), list_html, "")

	body += tool.Baro_card_ex(tool.Baro_t("권장 규칙"),
		tool.Baro_t("광고 · 욕설 도배 · 반복 링크 · 개인정보 · 전화번호 · 이메일 · 대량 삭제 · 초고속 반복 편집 · 의심스러운 계정 행동"),
		tool.Baro_table([]string{tool.Baro_t("이름"), "종류", tool.Baro_t("정규식 / 기준"), tool.Baro_t("조치"), tool.Baro_t("설명"), "추가"}, preset_rows, ""),
		tool.Baro_note(tool.Baro_t("추가한 뒤에도 정규식과 조치를 자유롭게 고칠 수 있습니다.")))

	return tool.Baro_page(db, config, tool.Baro_t("편집 필터"), body, baro_quality_menu())
}

// View_baro_quality_filter_add : 규칙 추가 (권한 system)
func View_baro_quality_filter_add(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, need := baro_quality_filter_guard(db, config); need {
		return deny
	}

	name, pattern, kind, action, note := "", "", "content", "review", ""
	message := ""

	if values != nil {
		// 권장 규칙 한 줄 넣기
		if preset := baro_quality_val(values, "preset"); preset != "" {
			for _, row := range baro_quality_filter_preset() {
				if row[0] != preset {
					continue
				}

				name, pattern, kind, action, note = row[0], row[1], row[2], row[3], row[4]
			}

			if name == "" {
				message = tool.Baro_alert("danger", tool.Baro_t("알 수 없는 권장 규칙입니다."))
			}
		} else {
			name = baro_quality_val(values, "name")
			pattern = baro_quality_val(values, "pattern")
			kind = baro_quality_val(values, "kind")
			action = baro_quality_val(values, "action")
			note = baro_quality_val(values, "note")
		}

		if message == "" {
			if problem := baro_quality_filter_check_input(name, pattern, kind, action); problem != "" {
				message = tool.Baro_alert("danger", tool.HTML_escape(problem))
			} else if tool.Baro_read_only(db) {
				message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
			} else {
				id := tool.Baro_next_id(db, "baro_edit_filter")

				tool.Exec_DB(db,
					"insert into baro_edit_filter (id, name, pattern, kind, action, status, note, date, hits) values (?, ?, ?, ?, ?, 'on', ?, ?, '0')",
					id, tool.Baro_trunc(name, 100), tool.Baro_trunc(pattern, 500), kind, action,
					tool.Baro_trunc(note, 500), tool.Baro_now())

				tool.Baro_public_log(db, "filter_add", config.IP, "필터 #"+id, tool.Baro_trunc(note, 200), tool.Baro_trunc(name, 100))
				tool.Baro_audit(db, config, "filter_add", "filter:"+id, "", kind+"/"+action+"/"+pattern, note)

				return tool.Get_redirect("/baro/quality/filter")
			}
		}
	}

	form := tool.Baro_input("name", tool.Baro_t("규칙 이름"), name, tool.Baro_t("예: 광고 문구"), tool.Baro_t("무엇을 잡는 규칙인지 알아볼 수 있게 적으세요."))
	form += tool.Baro_select("kind", tool.Baro_t("규칙 종류"), baro_quality_filter_kind_options(), kind,
		tool.Baro_t("전체 삭제는 정규식을 쓰지 않고, 속도는 정규식 칸에 숫자(5분 안 편집 횟수)를 넣습니다."))
	form += tool.Baro_textarea("pattern", tool.Baro_t("정규식 / 기준"), pattern, 3, tool.Baro_t(`예: (?i)(카지노|토토\s*사이트)`),
		tool.Baro_t("Go 의 regexp(RE2) 문법입니다. 뒤돌아보기(lookbehind)와 역참조는 쓸 수 없습니다."))
	form += tool.Baro_select("action", tool.Baro_t("조치"), baro_quality_filter_action_options(), action,
		tool.Baro_t("차단은 어떤 경우에도 게시할 수 없는 것에만 쓰세요."))
	form += tool.Baro_textarea("note", tool.Baro_t("메모"), note, 3, tool.Baro_t("왜 이 규칙이 필요한지, 오탐 가능성은 무엇인지"), "")
	form += tool.Baro_submit(tool.Baro_t("규칙 추가"), "primary")

	body := baro_quality_crumb(tool.Baro_t("규칙 추가"))
	body += tool.Baro_hero(tool.Baro_t("편집 필터 규칙 추가"), tool.Baro_t("정규식이 잘못 적히면 저장하지 않습니다."), "")
	body += baro_quality_nav("/baro/quality/filter")
	body += message

	body += tool.Baro_card_ex(tool.Baro_t("새 규칙"), tool.Baro_t("저장하기 전에 정규식을 컴파일해 확인합니다."),
		tool.Baro_form("/baro/quality/filter/add", form),
		tool.Baro_button("/baro/quality/filter/test", tool.Baro_t("정규식 먼저 시험해 보기"), "")+" "+
			tool.Baro_button("/baro/quality/filter", tool.Baro_t("규칙 목록"), ""))

	body += tool.Baro_card(tool.Baro_t("자주 쓰는 조각"), tool.Baro_table(
		[]string{tool.Baro_t("조각"), tool.Baro_t("뜻")},
		[][]string{
			{tool.Baro_code(`(?i)`), tool.Baro_t("대소문자를 가리지 않음")},
			{tool.Baro_code(`\s*`), tool.Baro_t("공백이 없거나 여러 개")},
			{tool.Baro_code(`\d{6}`), tool.Baro_t("숫자 6개")},
			{tool.Baro_code(tool.Baro_t(`(가|나|다)`)), tool.Baro_t("셋 중 하나")},
			{tool.Baro_code(`{5,}`), tool.Baro_t("다섯 번 이상 되풀이")},
			{tool.Baro_code(`^$`), tool.Baro_t("빈 줄")},
		}, ""))

	return tool.Baro_page(db, config, tool.Baro_t("규칙 추가"), body, baro_quality_menu())
}

// baro_quality_filter_check_input : 규칙 입력 검사 (문제가 있으면 그 이유)
func baro_quality_filter_check_input(name string, pattern string, kind string, action string) string {
	if strings.TrimSpace(name) == "" {
		return tool.Baro_t("규칙 이름을 적으세요.")
	}

	valid_kind := false
	for _, row := range baro_quality_filter_kind_options() {
		if row[0] == kind {
			valid_kind = true
		}
	}
	if !valid_kind {
		return tool.Baro_t("알 수 없는 규칙 종류입니다.")
	}

	valid_action := false
	for _, row := range baro_quality_filter_action_options() {
		if row[0] == action {
			valid_action = true
		}
	}
	if !valid_action {
		return tool.Baro_t("알 수 없는 조치입니다.")
	}

	if tool.Get_len(pattern) > 500 {
		return tool.Baro_t("정규식이 너무 깁니다. 500자 안으로 줄여 주세요.")
	}

	switch kind {
	case "empty":
		return ""
	case "rate":
		if tool.Str_to_int(pattern) < 2 {
			return tool.Baro_t("속도 규칙은 2 이상의 숫자를 넣어야 합니다. (5분 안 편집 횟수)")
		}

		return ""
	}

	if strings.TrimSpace(pattern) == "" {
		return tool.Baro_t("정규식을 적으세요.")
	}

	if _, err := regexp.Compile(pattern); err != nil {
		return tool.Baro_t("정규식을 읽을 수 없습니다 : ") + err.Error()
	}

	return ""
}

// View_baro_quality_filter_edit : 규칙 수정 (권한 system)
func View_baro_quality_filter_edit(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, need := baro_quality_filter_guard(db, config); need {
		return deny
	}

	id = strings.TrimSpace(id)

	name, pattern, kind, action, status, note, date, hits := "", "", "", "", "", "", "", ""
	found := tool.QueryRow_DB(db,
		"select name, pattern, kind, action, status, note, date, hits from baro_edit_filter where id = ?",
		[]any{&name, &pattern, &kind, &action, &status, &note, &date, &hits}, id)

	if !found {
		body := baro_quality_crumb(tool.Baro_t("규칙 수정"))
		body += tool.Baro_hero(tool.Baro_t("규칙을 찾을 수 없습니다"), tool.Baro_t("번호가 잘못되었거나 이미 지워진 규칙입니다."), "")
		body += baro_quality_nav("/baro/quality/filter")
		body += tool.Baro_alert("warn", tool.Baro_t("규칙 번호 ")+tool.HTML_escape(tool.Baro_trunc(id, 40))+tool.Baro_t(" 이 없습니다."))
		body += tool.Baro_card(tool.Baro_t("다음 할 일"), tool.Baro_button("/baro/quality/filter", tool.Baro_t("규칙 목록"), "primary"))

		return tool.Baro_page(db, config, tool.Baro_t("규칙 수정"), body, baro_quality_menu())
	}

	message := ""

	if values != nil {
		new_name := baro_quality_val(values, "name")
		new_pattern := baro_quality_val(values, "pattern")
		new_kind := baro_quality_val(values, "kind")
		new_action := baro_quality_val(values, "action")
		new_note := baro_quality_val(values, "note")

		if problem := baro_quality_filter_check_input(new_name, new_pattern, new_kind, new_action); problem != "" {
			message = tool.Baro_alert("danger", tool.HTML_escape(problem))
			name, pattern, kind, action, note = new_name, new_pattern, new_kind, new_action, new_note
		} else if tool.Baro_read_only(db) {
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
		} else {
			before := kind + "/" + action + "/" + pattern

			tool.Exec_DB(db,
				"update baro_edit_filter set name = ?, pattern = ?, kind = ?, action = ?, note = ? where id = ?",
				tool.Baro_trunc(new_name, 100), tool.Baro_trunc(new_pattern, 500), new_kind, new_action,
				tool.Baro_trunc(new_note, 500), id)

			tool.Baro_public_log(db, "filter_edit", config.IP, "필터 #"+id, tool.Baro_trunc(new_note, 200), tool.Baro_trunc(new_name, 100))
			tool.Baro_audit(db, config, "filter_edit", "filter:"+id, before,
				new_kind+"/"+new_action+"/"+new_pattern, "편집 필터 수정")

			return tool.Get_redirect("/baro/quality/filter")
		}
	}

	form := tool.Baro_input("name", tool.Baro_t("규칙 이름"), name, "", "")
	form += tool.Baro_select("kind", tool.Baro_t("규칙 종류"), baro_quality_filter_kind_options(), kind, "")
	form += tool.Baro_textarea("pattern", tool.Baro_t("정규식 / 기준"), pattern, 3, "", tool.Baro_t("저장하기 전에 컴파일해 확인합니다."))
	form += tool.Baro_select("action", tool.Baro_t("조치"), baro_quality_filter_action_options(), action, "")
	form += tool.Baro_textarea("note", tool.Baro_t("메모"), note, 3, "", "")
	form += tool.Baro_submit("저장", "primary")

	state := tool.Baro_badge(tool.Baro_t("꺼짐"), "muted")
	toggle_text := tool.Baro_t("이 규칙 켜기")
	if status == "on" {
		state = tool.Baro_badge(tool.Baro_t("켜짐"), "ok")
		toggle_text = tool.Baro_t("이 규칙 끄기")
	}

	hit_count := baro_quality_count(db, "select count(*) from baro_filter_log where filter_id = ?", id)

	body := baro_quality_crumb(tool.Baro_t("규칙 수정"))
	body += tool.Baro_hero(tool.Baro_t("편집 필터 #")+id, name, "")
	body += baro_quality_nav("/baro/quality/filter")
	body += message

	body += tool.Baro_kv_grid([][]string{
		{"상태", state},
		{tool.Baro_t("조치"), baro_quality_filter_action_badge(action)},
		{"종류", tool.HTML_escape(baro_quality_filter_kind_name(kind))},
		{tool.Baro_t("누적 적발"), tool.Baro_comma(tool.Str_to_int(hits))},
		{tool.Baro_t("기록된 적발"), tool.Baro_comma(hit_count)},
		{tool.Baro_t("만든 시각"), tool.Baro_time_cell(date)},
	})

	body += tool.Baro_card_ex(tool.Baro_t("규칙 내용"), tool.Baro_t("정규식이 잘못 적히면 저장하지 않습니다."),
		tool.Baro_form("/baro/quality/filter/edit/"+tool.Url_parser(id), form),
		tool.Baro_form("/baro/quality/filter/toggle/"+tool.Url_parser(id), tool.Baro_submit(toggle_text, ""))+
			tool.Baro_button("/baro/quality/filter/test?id="+tool.Url_parser(id), "이 규칙 시험", "")+" "+
			tool.Baro_button("/baro/quality/filter", tool.Baro_t("규칙 목록"), ""))

	body += tool.Baro_danger_zone(tool.Baro_t("규칙 삭제"),
		tool.Baro_note(tool.Baro_t("삭제하면 되돌릴 수 없습니다. 잠시 쓰지 않을 것이라면 끄기를 쓰세요."))+
			tool.Baro_button("/baro/quality/filter/delete/"+tool.Url_parser(id), "이 규칙 삭제", "danger"))

	return tool.Baro_page(db, config, tool.Baro_t("규칙 수정"), body, baro_quality_menu())
}

// View_baro_quality_filter_toggle : 규칙 켜고 끄기 (권한 system / POST)
func View_baro_quality_filter_toggle(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, need := baro_quality_filter_guard(db, config); need {
		return deny
	}

	_ = values
	id = strings.TrimSpace(id)

	name, status := "", ""
	if !tool.QueryRow_DB(db, "select name, status from baro_edit_filter where id = ?", []any{&name, &status}, id) {
		return tool.Get_redirect("/baro/quality/filter")
	}

	if tool.Baro_read_only(db) {
		return tool.Get_redirect("/baro/quality/filter")
	}

	new_status := "on"
	if status == "on" {
		new_status = ""
	}

	tool.Exec_DB(db, "update baro_edit_filter set status = ? where id = ?", new_status, id)

	state_name := "켜짐"
	if new_status == "" {
		state_name = "꺼짐"
	}

	tool.Baro_public_log(db, "filter_toggle", config.IP, "필터 #"+id, state_name, tool.Baro_trunc(name, 100))
	tool.Baro_audit(db, config, "filter_toggle", "filter:"+id, status, new_status, "편집 필터 켜고 끄기")

	return tool.Get_redirect("/baro/quality/filter")
}

// View_baro_quality_filter_delete : 규칙 삭제 (권한 system)
func View_baro_quality_filter_delete(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, need := baro_quality_filter_guard(db, config); need {
		return deny
	}

	id = strings.TrimSpace(id)

	name, pattern, kind, action, status := "", "", "", "", ""
	found := tool.QueryRow_DB(db,
		"select name, pattern, kind, action, status from baro_edit_filter where id = ?",
		[]any{&name, &pattern, &kind, &action, &status}, id)

	if !found {
		body := baro_quality_crumb(tool.Baro_t("규칙 삭제"))
		body += tool.Baro_hero(tool.Baro_t("규칙을 찾을 수 없습니다"), tool.Baro_t("번호가 잘못되었거나 이미 지워진 규칙입니다."), "")
		body += baro_quality_nav("/baro/quality/filter")
		body += tool.Baro_alert("warn", tool.Baro_t("규칙 번호 ")+tool.HTML_escape(tool.Baro_trunc(id, 40))+tool.Baro_t(" 이 없습니다."))
		body += tool.Baro_card(tool.Baro_t("다음 할 일"), tool.Baro_button("/baro/quality/filter", tool.Baro_t("규칙 목록"), "primary"))

		return tool.Baro_page(db, config, tool.Baro_t("규칙 삭제"), body, baro_quality_menu())
	}

	if values != nil {
		if tool.Baro_read_only(db) {
			return tool.Get_redirect("/baro/quality/filter")
		}

		reason := tool.Baro_trunc(baro_quality_val(values, "reason"), 300)
		if reason == "" {
			reason = "사유 없음"
		}

		tool.Exec_DB(db, "delete from baro_edit_filter where id = ?", id)

		tool.Baro_public_log(db, "filter_delete", config.IP, "필터 #"+id, reason, tool.Baro_trunc(name, 100))
		tool.Baro_audit(db, config, "filter_delete", "filter:"+id, kind+"/"+action+"/"+pattern, "", reason)

		return tool.Get_redirect("/baro/quality/filter")
	}

	body := baro_quality_crumb(tool.Baro_t("규칙 삭제"))
	body += tool.Baro_hero(tool.Baro_t("편집 필터 삭제"), name, "")
	body += baro_quality_nav("/baro/quality/filter")

	body += tool.Baro_kv_grid([][]string{
		{tool.Baro_t("이름"), tool.HTML_escape(name)},
		{"종류", tool.HTML_escape(baro_quality_filter_kind_name(kind))},
		{tool.Baro_t("조치"), baro_quality_filter_action_badge(action)},
		{tool.Baro_t("정규식"), tool.Baro_code(pattern)},
	})

	body += tool.Baro_danger_zone(tool.Baro_t("정말 삭제할까요"),
		tool.Baro_alert("danger", tool.Baro_t("삭제하면 되돌릴 수 없습니다. 잠시 쓰지 않을 것이라면 <b>끄기</b>를 쓰세요. 적발 기록(baro_filter_log)은 남습니다."))+
			tool.Baro_form("/baro/quality/filter/delete/"+tool.Url_parser(id),
				tool.Baro_textarea("reason", tool.Baro_t("삭제 사유"), "", 3, tool.Baro_t("왜 지우는지 적어 주세요."), tool.Baro_t("공개 운영 로그에 남습니다."))+
					tool.Baro_submit("삭제", "danger"))+
			tool.Baro_button("/baro/quality/filter", tool.Baro_t("그만두기"), ""))

	return tool.Baro_page(db, config, tool.Baro_t("규칙 삭제"), body, baro_quality_menu())
}

// View_baro_quality_filter_test : 정규식 시험 (실제로 대조한다)
func View_baro_quality_filter_test(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, need := baro_quality_filter_guard(db, config); need {
		return deny
	}

	pattern, kind, sample, send, title := "", "content", "", "", ""
	result := ""

	// 기존 규칙 불러오기
	if id = strings.TrimSpace(id); id != "" && values == nil {
		tool.QueryRow_DB(db, "select pattern, kind from baro_edit_filter where id = ?", []any{&pattern, &kind}, id)
	}

	if sample == "" && values == nil {
		sample = "이 문서는 카지노 사이트 광고와 전화번호 010-1234-5678 을 담고 있습니다.\n출처 : https://example.org/news"
	}

	if values != nil {
		pattern = values.Get("pattern")
		kind = baro_quality_val(values, "kind")
		sample = values.Get("sample")
		send = values.Get("send")
		title = baro_quality_val(values, "title")

		result = baro_quality_filter_test_result(pattern, kind, title, sample, send)
	}

	form := tool.Baro_select("kind", tool.Baro_t("규칙 종류"), baro_quality_filter_kind_options(), kind, tool.Baro_t("본문 · 편집 요약 · 문서 이름 중 어디에 맞춰 볼지 고릅니다."))
	form += tool.Baro_textarea("pattern", tool.Baro_t("정규식 / 기준"), pattern, 3, tool.Baro_t(`예: (?i)(카지노|토토\s*사이트)`), tool.Baro_t("Go 의 regexp(RE2) 문법입니다."))
	form += tool.Baro_input("title", tool.Baro_t("문서 이름 (문서 이름 규칙 시험용)"), title, tool.Baro_t("예: 광고 문서"), "")
	form += tool.Baro_input("send", tool.Baro_t("편집 요약 (편집 요약 규칙 시험용)"), send, tool.Baro_t("예: asdf"), "")
	form += tool.Baro_textarea("sample", tool.Baro_t("본문 샘플"), sample, 8, tool.Baro_t("여기에 시험할 본문을 붙여 넣으세요."), "")
	form += tool.Baro_submit(tool.Baro_t("시험 실행"), "primary")

	body := baro_quality_crumb(tool.Baro_t("정규식 시험"))
	body += tool.Baro_hero(tool.Baro_t("정규식 시험"), tool.Baro_t("규칙을 저장하기 전에 본문 샘플에 실제로 맞춰 봅니다."), "")
	body += baro_quality_nav("/baro/quality/filter")

	body += tool.Baro_card_ex(tool.Baro_t("시험"), tool.Baro_t("실제 Go regexp 로 대조합니다."),
		tool.Baro_form("/baro/quality/filter/test", form),
		tool.Baro_button("/baro/quality/filter", tool.Baro_t("규칙 목록"), "")+" "+
			tool.Baro_button("/baro/quality/filter/add", tool.Baro_t("규칙 추가"), ""))

	if result != "" {
		body += tool.Baro_card(tool.Baro_t("시험 결과"), result)
	}

	// 켜진 규칙 전체로 시험
	if values != nil && strings.TrimSpace(sample+send+title) != "" {
		matched := baro_quality_filter_match(db, title, "", sample, send, 0)

		rows := [][]string{}
		for _, row := range matched {
			_, hit_text := baro_quality_filter_hit(row, title, sample, send, 0)

			rows = append(rows, []string{
				"#" + tool.HTML_escape(row.ID),
				tool.HTML_escape(row.Name),
				tool.HTML_escape(baro_quality_filter_kind_name(row.Kind)),
				baro_quality_filter_action_badge(row.Action),
				tool.Baro_code(hit_text),
			})
		}

		body += tool.Baro_card_ex(tool.Baro_t("켜져 있는 모든 규칙으로 시험"),
			tool.Baro_t("이 샘플이 실제 편집이었다면 아래 규칙에 걸렸을 것입니다."),
			tool.Baro_table([]string{"번호", tool.Baro_t("이름"), "종류", tool.Baro_t("조치"), tool.Baro_t("걸린 부분")}, rows,
				tool.Baro_t("켜져 있는 규칙 중 걸리는 것이 없습니다.")), "")
	}

	return tool.Baro_page(db, config, tool.Baro_t("정규식 시험"), body, baro_quality_menu())
}

// baro_quality_filter_test_result : 정규식 한 개를 샘플에 맞춰 본 결과 HTML
func baro_quality_filter_test_result(pattern string, kind string, title string, sample string, send string) string {
	switch kind {
	case "empty":
		if strings.TrimSpace(sample) == "" {
			return tool.Baro_alert("warn", tool.Baro_t("본문이 비어 있으므로 <b>전체 삭제</b> 규칙에 걸립니다."))
		}

		return tool.Baro_alert("ok", tool.Baro_t("본문이 비어 있지 않으므로 전체 삭제 규칙에 걸리지 않습니다."))
	case "rate":
		limit := tool.Str_to_int(pattern)
		if limit < 2 {
			return tool.Baro_alert("danger", tool.Baro_t("속도 규칙은 2 이상의 숫자를 넣어야 합니다."))
		}

		return tool.Baro_alert("info",
			tool.Baro_t("속도 규칙은 본문이 아니라 편집 횟수를 봅니다. 5분 안에 ")+tool.Baro_itoa(limit)+tool.Baro_t("회 이상 편집하면 걸립니다."))
	}

	if strings.TrimSpace(pattern) == "" {
		return tool.Baro_alert("danger", tool.Baro_t("정규식을 적으세요."))
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return tool.Baro_alert("danger",
			tool.Baro_t("정규식을 읽을 수 없습니다. 저장도 거부됩니다.<br>")+tool.Baro_code(err.Error()))
	}

	subject := sample
	subject_name := tool.Baro_t("본문")
	switch kind {
	case "send":
		subject, subject_name = send, tool.Baro_t("편집 요약")
	case "title":
		subject, subject_name = title, tool.Baro_t("문서 이름")
	}

	matches := compiled.FindAllString(subject, 30)
	if len(matches) == 0 {
		return tool.Baro_alert("ok", subject_name+tool.Baro_t("에서 걸리는 부분이 없습니다."))
	}

	items := []string{}
	for _, match := range matches {
		items = append(items, tool.Baro_code(tool.Baro_trunc(match, 200)))
	}

	return tool.Baro_alert("warn", subject_name+tool.Baro_t("에서 ")+tool.Baro_itoa(len(matches))+tool.Baro_t("곳이 걸렸습니다.")) +
		tool.Baro_list(items)
}

// baro_quality_filter_hits_bump : 규칙의 누적 적발 수를 1 늘린다
func baro_quality_filter_hits_bump(db *sql.DB, id string) {
	hits := "0"
	tool.QueryRow_DB(db, "select hits from baro_edit_filter where id = ?", []any{&hits}, id)

	tool.Exec_DB(db, "update baro_edit_filter set hits = ? where id = ?",
		tool.Baro_itoa(tool.Str_to_int(hits)+1), id)
}

// baro_quality_filter_run : 최근 편집을 켜진 규칙에 대조해 적발 기록을 남긴다
func baro_quality_filter_run(db *sql.DB, limit int) (int, int) {
	if limit < 1 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}

	edits := []baro_quality_edit{}

	rows := tool.Query_DB(db,
		"select title, id, ip, date, send, leng, data from history order by date desc limit ?",
		limit)
	defer rows.Close()

	for rows.Next() {
		edit := baro_quality_edit{}
		if err := rows.Scan(&edit.Title, &edit.Rev, &edit.User, &edit.Date, &edit.Send, &edit.Leng, &edit.Data); err != nil {
			continue
		}

		edits = append(edits, edit)
	}

	filters := baro_quality_filter_list(db, true)
	added := 0

	for _, edit := range edits {
		recent := baro_quality_recent_edit_count(db, edit.User, edit.Date)

		for _, filter := range filters {
			hit, hit_text := baro_quality_filter_hit(filter, edit.Title, edit.Data, edit.Send, recent)
			if !hit {
				continue
			}

			found := ""
			if tool.QueryRow_DB(db,
				"select id from baro_filter_log where filter_id = ? and title = ? and user = ? and date = ?",
				[]any{&found}, filter.ID, edit.Title, edit.User, edit.Date) {
				continue
			}

			tool.Exec_DB(db,
				"insert into baro_filter_log (id, filter_id, user, title, action, detail, date) values (?, ?, ?, ?, ?, ?, ?)",
				tool.Baro_next_id(db, "baro_filter_log"), filter.ID, edit.User, edit.Title, filter.Action,
				"r"+edit.Rev+" : "+tool.Baro_trunc(hit_text, 150), edit.Date)

			baro_quality_filter_hits_bump(db, filter.ID)

			added++

			// 검토 큐로 보내는 규칙이면 편집도 큐에 올린다
			if filter.Action != "log" && !baro_quality_review_exists(db, edit.Title, edit.Rev, "edit") {
				risk, _ := baro_quality_risk(db, edit)
				if risk < 30 {
					risk = 30
				}

				baro_quality_review_insert(db, edit.Title, edit.Rev, edit.User, "edit", risk, edit.Date)
			}
		}
	}

	if added > 0 {
		tool.Baro_metric_bump(db, "filter_hit", tool.Baro_today(), added)
	}

	return len(edits), added
}

// View_baro_quality_filter_log : 필터에 걸린 기록 (권한 system)
func View_baro_quality_filter_log(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if deny, need := baro_quality_filter_guard(db, config); need {
		return deny
	}

	message := ""

	if values != nil {
		if tool.Baro_read_only(db) {
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
		} else if !tool.Baro_rate_ok(db, "quality_filter_run", config.IP, 6, 600) {
			message = tool.Baro_alert("warn", tool.Baro_t("필터 점검을 너무 자주 실행했습니다. 잠시 후 다시 시도하세요."))
		} else {
			limit := tool.Str_to_int(baro_quality_val(values, "limit"))
			checked, added := baro_quality_filter_run(db, limit)

			tool.Baro_public_log(db, "filter_run", config.IP, "",
				"최근 편집 "+tool.Baro_itoa(checked)+"건 대조", "적발 "+tool.Baro_itoa(added)+"건")
			tool.Baro_audit(db, config, "filter_run", "history", "",
				"checked="+tool.Baro_itoa(checked)+" hit="+tool.Baro_itoa(added), "편집 필터 일괄 점검")

			return tool.Get_redirect("/baro/quality/filter/log")
		}
	}

	page_num := baro_quality_page_num(page)
	offset := (page_num - 1) * tool.Baro_page_size

	rows := tool.Query_DB(db,
		"select id, filter_id, user, title, action, detail, date from baro_filter_log order by id + 0 desc limit ?, ?",
		offset, tool.Baro_page_size)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		id, filter_id, user, title, action, detail, date := "", "", "", "", "", "", ""
		if err := rows.Scan(&id, &filter_id, &user, &title, &action, &detail, &date); err != nil {
			continue
		}

		table_rows = append(table_rows, []string{
			"#" + tool.HTML_escape(id),
			`<a href="/baro/quality/filter/edit/` + tool.Url_parser(filter_id) + `">` +
				tool.HTML_escape(baro_quality_filter_name(db, filter_id)) + `</a>`,
			baro_quality_doc_cell(title),
			tool.Baro_user_link(db, user, config.IP),
			baro_quality_filter_action_badge(action),
			tool.Baro_code(tool.Baro_trunc(detail, 100)),
			tool.Baro_time_cell(date),
		})
	}

	list_html := tool.Baro_table(
		[]string{"번호", tool.Baro_t("규칙"), "문서", tool.Baro_t("사용자"), tool.Baro_t("조치"), tool.Baro_t("걸린 부분"), "시각"},
		table_rows, "")

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("적발 기록이 없습니다. 아래에서 최근 편집을 규칙에 대조해 볼 수 있습니다."))
	}

	list_html += tool.Baro_paging(page_num, len(table_rows), tool.Baro_page_size, "/baro/quality/filter/log?page={}")

	total := baro_quality_count(db, "select count(*) from baro_filter_log")
	block_count := baro_quality_count(db, "select count(*) from baro_filter_log where action = 'block'")
	review_count := baro_quality_count(db, "select count(*) from baro_filter_log where action = 'review'")

	form := tool.Baro_select("limit", tool.Baro_t("대조할 최근 편집 수"), [][]string{
		{"100", tool.Baro_t("최근 100건")},
		{"200", tool.Baro_t("최근 200건")},
		{"500", tool.Baro_t("최근 500건")},
	}, "200", tool.Baro_t("켜져 있는 규칙만 씁니다. 이미 기록된 적발은 건너뜁니다."))
	form += tool.Baro_submit(tool.Baro_t("필터 점검 실행"), "primary")

	body := baro_quality_crumb(tool.Baro_t("필터 적발 기록"))
	body += tool.Baro_hero(tool.Baro_t("편집 필터 적발 기록"), tool.Baro_t("어떤 규칙이 무엇을 걸렀는지 남깁니다."), "")
	body += baro_quality_nav("/baro/quality/filter")
	body += message

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 적발"), tool.Baro_comma(total), ""},
		{tool.Baro_t("검토 큐로"), tool.Baro_comma(review_count), ""},
		{tool.Baro_t("차단"), tool.Baro_comma(block_count), ""},
	})

	body += tool.Baro_card_ex(tool.Baro_t("최근 편집 대조"), tool.Baro_t("켜져 있는 규칙을 최근 편집에 대조해 적발 기록을 만듭니다."),
		tool.Baro_form("/baro/quality/filter/log", form),
		tool.Baro_note(tool.Baro_t("검토 큐로 보내는 규칙에 걸린 편집은 검토 큐에도 함께 올라갑니다."))+
			tool.Baro_button("/baro/review", tool.Baro_t("검토 큐 보기"), ""))

	body += tool.Baro_card_ex(tool.Baro_t("적발 기록"), tool.Baro_t("최근 순서입니다."), list_html,
		tool.Baro_button("/baro/quality/filter", tool.Baro_t("규칙 목록"), ""))

	return tool.Baro_page(db, config, tool.Baro_t("필터 적발 기록"), body, baro_quality_menu())
}
