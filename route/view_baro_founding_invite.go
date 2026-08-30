package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  1-5 추천 · 초대 시스템
//    /baro/invite           내 초대 링크 발급 / 목록
//    /baro/invite/use/<코드>  초대 코드 확인
// ============================================================

// baro_founding_invite_row : baro_invite 한 줄
type baro_founding_invite_row struct {
	ID       string
	Code     string
	Owner    string
	Target   string
	Date     string
	UsedDate string
	Status   string
}

// baro_founding_invite_limit : 한 사람이 만들 수 있는 초대 링크 수
func baro_founding_invite_limit(db *sql.DB) int {
	limit := tool.Baro_get_int(db, "invite_limit", 10)
	if limit < 0 {
		limit = 0
	}

	return limit
}

// baro_founding_invite_by_code : 초대 코드로 찾기
func baro_founding_invite_by_code(db *sql.DB, code string) (baro_founding_invite_row, bool) {
	row := baro_founding_invite_row{}
	if strings.TrimSpace(code) == "" {
		return row, false
	}

	found := tool.QueryRow_DB(
		db,
		"select id, code, owner, target, date, used_date, status from baro_invite where code = ?",
		[]any{&row.ID, &row.Code, &row.Owner, &row.Target, &row.Date, &row.UsedDate, &row.Status},
		code,
	)

	return row, found
}

// baro_founding_invite_rows : 내 초대 목록
func baro_founding_invite_rows(db *sql.DB, owner string, offset int, limit int) []baro_founding_invite_row {
	out := []baro_founding_invite_row{}

	rows := tool.Query_DB(
		db,
		"select id, code, owner, target, date, used_date, status from baro_invite where owner = ? order by id + 0 desc limit ?, ?",
		owner, offset, limit,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_founding_invite_row{}
		if err := rows.Scan(&row.ID, &row.Code, &row.Owner, &row.Target, &row.Date, &row.UsedDate, &row.Status); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_founding_invite_open_rows : 아직 완료되지 않은 초대 (완료 검사 대상)
func baro_founding_invite_open_rows(db *sql.DB, owner string) []baro_founding_invite_row {
	out := []baro_founding_invite_row{}

	rows := tool.Query_DB(
		db,
		"select id, code, owner, target, date, used_date, status from baro_invite where owner = ? and status = 'open' and target != ''",
		owner,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_founding_invite_row{}
		if err := rows.Scan(&row.ID, &row.Code, &row.Owner, &row.Target, &row.Date, &row.UsedDate, &row.Status); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_founding_invite_badge_def : inviter_3 배지 정의가 없으면 만들어 둔다
func baro_founding_invite_badge_def(db *sql.DB) {
	found := ""
	if tool.QueryRow_DB(db, "select key_name from baro_badge where key_name = ?", []any{&found}, "inviter_3") {
		return
	}

	tool.Exec_DB(
		db,
		"insert into baro_badge (key_name, name, description, kind, icon, criteria, sort) values (?, ?, ?, ?, ?, ?, ?)",
		"inviter_3", "창립 동행자",
		"창립 기여자가 된 사람 3명을 초대했습니다.",
		"founding", "◎", "invite_founder:3", "12",
	)
}

// baro_founding_invite_done_count : 초대해서 창립 기여자가 된 사람 수
func baro_founding_invite_done_count(db *sql.DB, owner string) int {
	num := 0
	tool.QueryRow_DB(
		db,
		"select count(*) from baro_invite where owner = ? and status = 'done' and target != '' and target in (select user from baro_founder where status = 'confirmed')",
		[]any{&num}, owner,
	)

	return num
}

// baro_founding_invite_check : 초대 완료 처리 + 배지 검사
//
//	초대받은 사람이 첫 유효 기여를 마쳤을 때만 완료로 기록한다.
//	(계정만 늘리는 추천은 인정하지 않는다는 정책)
func baro_founding_invite_check(db *sql.DB, owner string) int {
	done := 0

	for _, row := range baro_founding_invite_open_rows(db, owner) {
		if baro_founding_first_valid(db, row.Target) == nil {
			continue
		}

		tool.Exec_DB(
			db,
			"update baro_invite set status = 'done', used_date = ? where id = ?",
			tool.Baro_now(), row.ID,
		)
		tool.Baro_notify(
			db, owner, "system",
			tool.Baro_t("초대한 ")+row.Target+tool.Baro_t(" 님이 첫 유효 기여를 마쳤습니다."),
			"/baro/invite", "invite_done",
		)
		tool.Baro_public_log(db, "invite_done", owner, row.Target, "초대받은 사용자의 첫 유효 기여 완료", "")
		tool.Baro_metric_bump(db, "invite_done", tool.Baro_today(), 1)

		done++
	}

	// 창립 기여자 3명을 초대하면 배지를 준다.
	if baro_founding_invite_done_count(db, owner) >= 3 {
		baro_founding_invite_badge_def(db)
		if baro_founding_give_badge(db, owner, "inviter_3", "창립 기여자 3명 초대") {
			tool.Baro_notify(db, owner, "badge", "창립 동행자 배지를 받았습니다.", "/baro/invite", "inviter_3")
			tool.Baro_public_log(db, "badge_give", "system", owner, "창립 기여자 3명 초대", "창립 동행자")
		}
	}

	return done
}

// baro_founding_invite_make : 초대 링크 발급
func baro_founding_invite_make(db *sql.DB, config tool.Config) string {
	if tool.Baro_read_only(db) {
		return "read_only"
	}

	if !tool.Baro_rate_ok(db, "invite_make", config.IP, 20, 3600) {
		return "rate"
	}

	limit := baro_founding_invite_limit(db)
	used := 0
	tool.QueryRow_DB(db, "select count(*) from baro_invite where owner = ? and status != 'closed'", []any{&used}, config.IP)

	if used >= limit {
		return "limit"
	}

	code := ""
	for i := 0; i < 6; i++ {
		candidate := tool.Get_random_key(12)
		if _, found := baro_founding_invite_by_code(db, candidate); !found {
			code = candidate
			break
		}
	}

	if code == "" {
		return "code"
	}

	tool.Exec_DB(
		db,
		"insert into baro_invite (id, code, owner, target, date, used_date, status) values (?, ?, ?, '', ?, '', 'open')",
		tool.Baro_next_id(db, "baro_invite"), code, config.IP, tool.Baro_now(),
	)
	tool.Baro_metric_bump(db, "invite_make", tool.Baro_today(), 1)

	return "made"
}

// baro_founding_invite_cancel : 아직 쓰이지 않은 초대 링크 닫기
func baro_founding_invite_cancel(db *sql.DB, config tool.Config, id string) string {
	owner, status, target := "", "", ""
	if !tool.QueryRow_DB(
		db,
		"select owner, status, target from baro_invite where id = ?",
		[]any{&owner, &status, &target}, id,
	) {
		return "no_invite"
	}

	if owner != config.IP {
		return "not_mine"
	}

	if status != "open" {
		return "state"
	}

	tool.Exec_DB(db, "update baro_invite set status = 'closed' where id = ?", id)

	return "closed"
}

// baro_founding_invite_msg : 초대 화면 안내 문구
func baro_founding_invite_msg(code string) (string, string) {
	switch code {
	case "made":
		return "ok", tool.Baro_t("새 초대 링크를 만들었습니다. 아래 목록에서 복사해 전달하세요.")
	case "closed":
		return "ok", tool.Baro_t("초대 링크를 닫았습니다.")
	case "limit":
		return "danger", tool.Baro_t("초대 한도를 모두 썼습니다. 스팸을 막기 위해 한 사람이 만들 수 있는 초대 수에는 한도가 있습니다.")
	case "rate":
		return "danger", tool.Baro_t("짧은 시간에 너무 많이 만들었습니다. 잠시 뒤에 다시 시도해 주세요.")
	case "read_only":
		return "danger", tool.Baro_t("지금은 읽기 전용 모드라 초대 링크를 만들 수 없습니다.")
	case "code":
		return "danger", tool.Baro_t("초대 코드를 만들지 못했습니다. 다시 시도해 주세요.")
	case "no_invite":
		return "danger", tool.Baro_t("초대 링크를 찾을 수 없습니다.")
	case "not_mine":
		return "danger", tool.Baro_t("내가 만든 초대 링크만 닫을 수 있습니다.")
	case "state":
		return "danger", tool.Baro_t("이미 사용되었거나 닫힌 초대 링크입니다.")
	}

	return "", ""
}

// View_baro_founding_invite : 내 초대 링크 (1-5)
func View_baro_founding_invite(config tool.Config, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/invite"); need {
		return wall
	}

	if values != nil {
		code := "state"
		switch values.Get("action") {
		case "make":
			code = baro_founding_invite_make(db, config)
		case "cancel":
			code = baro_founding_invite_cancel(db, config, values.Get("id"))
		}

		return tool.Get_redirect("/baro/invite?done=" + tool.Url_parser(code))
	}

	// 초대 완료 검사 (접속할 때마다)
	baro_founding_invite_check(db, config.IP)

	page_int := baro_founding_page_num(page)
	list := baro_founding_invite_rows(db, config.IP, (page_int-1)*tool.Baro_page_size, tool.Baro_page_size)

	limit := baro_founding_invite_limit(db)
	used := 0
	tool.QueryRow_DB(db, "select count(*) from baro_invite where owner = ? and status != 'closed'", []any{&used}, config.IP)

	done := 0
	tool.QueryRow_DB(db, "select count(*) from baro_invite where owner = ? and status = 'done'", []any{&done}, config.IP)

	founder_done := baro_founding_invite_done_count(db, config.IP)

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/founding", tool.Baro_t("창립 시즌")},
		{"", tool.Baro_t("초대")},
	})
	body += tool.Baro_hero(tool.Baro_t("초대 링크"), tool.Baro_t("함께 기록할 사람을 부를 수 있습니다."), "")
	body += baro_founding_tabs("/baro/invite")

	if query != nil {
		if kind, text := baro_founding_invite_msg(query.Get("done")); text != "" {
			body += tool.Baro_alert(kind, tool.HTML_escape(text))
		}
	}

	body += baro_founding_invite_policy_html()

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("만든 초대"), tool.Baro_itoa(used) + " / " + tool.Baro_itoa(limit), tool.Baro_t("닫은 링크는 한도에서 빠집니다")},
		{tool.Baro_t("완료된 초대"), tool.Baro_itoa(done) + tool.Baro_t("건"), tool.Baro_t("첫 유효 기여까지 마친 초대")},
		{tool.Baro_t("창립 기여자 초대"), tool.Baro_itoa(founder_done) + tool.Baro_t("명"), tool.Baro_t("3명이면 창립 동행자 배지")},
	})
	body += tool.Baro_progress(founder_done, 3, tool.Baro_t("창립 동행자 배지"))

	make_form := tool.Baro_hidden("action", "make")
	make_form += tool.Baro_submit(tool.Baro_t("새 초대 링크 만들기"), "primary")

	if used >= limit {
		body += tool.Baro_card(
			tool.Baro_t("초대 링크 발급"),
			tool.Baro_alert("warn", tool.Baro_t("초대 한도(")+tool.Baro_itoa(limit)+tool.Baro_t("개)를 모두 썼습니다. 쓰지 않는 링크를 닫으면 다시 만들 수 있습니다.")),
		)
	} else {
		body += tool.Baro_card_ex(
			tool.Baro_t("초대 링크 발급"),
			tool.Baro_t("남은 발급 가능 수 ")+tool.Baro_itoa(limit-used)+tool.Baro_t("개"),
			tool.Baro_form("/baro/invite", make_form),
			tool.Baro_note("초대 링크는 /baro/invite/use/<코드> 형태로 만들어집니다."),
		)
	}

	rows := [][]string{}
	for _, row := range list {
		link := "/baro/invite/use/" + row.Code

		status_html := tool.Baro_badge(tool.Baro_t("사용 대기"), "info")
		switch row.Status {
		case "done":
			status_html = tool.Baro_badge(tool.Baro_t("완료"), "ok")
		case "closed":
			status_html = tool.Baro_badge(tool.Baro_t("닫힘"), "muted")
		}

		target_html := `<span class="bw_muted">-</span>`
		if row.Target != "" {
			target_html = tool.Baro_user_link(db, row.Target, config.IP)
		}

		action_html := `<span class="bw_muted">-</span>`
		if row.Status == "open" {
			cancel_form := tool.Baro_hidden("action", "cancel")
			cancel_form += tool.Baro_hidden("id", row.ID)
			cancel_form += tool.Baro_submit(tool.Baro_t("닫기"), "danger")
			action_html = tool.Baro_form("/baro/invite", cancel_form)
		}

		rows = append(rows, []string{
			`<code>` + tool.HTML_escape(row.Code) + `</code>`,
			`<a href="` + tool.HTML_escape(link) + `">` + tool.HTML_escape(link) + `</a>` +
				` <button type="button" class="bw_btn" data-copy="` + tool.HTML_escape(link) + tool.Baro_t(`" onclick="baro_founding_copy(this)">복사</button>`),
			status_html,
			target_html,
			tool.Baro_time_cell(row.Date),
			tool.Baro_time_cell(row.UsedDate),
			action_html,
		})
	}

	if len(rows) == 0 {
		body += tool.Baro_card(tool.Baro_t("내 초대 링크"), tool.Baro_empty(tool.Baro_t("아직 만든 초대 링크가 없습니다. 위에서 만들어 보세요.")))
	} else {
		body += tool.Baro_card(
			tool.Baro_t("내 초대 링크"),
			tool.Baro_table(
				[]string{tool.Baro_t("코드"), tool.Baro_t("링크"), "상태", tool.Baro_t("초대받은 사람"), tool.Baro_t("만든 날"), tool.Baro_t("완료일"), "관리"},
				rows,
				tool.Baro_t("아직 만든 초대 링크가 없습니다."),
			),
		)
	}

	body += tool.Baro_paging(page_int, len(rows), tool.Baro_page_size, "/baro/invite?page={}")

	body += tool.Baro_card(
		tool.Baro_t("초대가 완료되는 조건"),
		tool.Baro_list([]string{
			tool.Baro_t("초대 링크를 연 사람이 <b>로그인 상태</b>면 그 사람이 초대 대상으로 기록됩니다."),
			tool.Baro_t("초대 대상이 <b>첫 유효 기여</b>(새 문서 작성 또는 출처 추가)를 마치면 완료로 바뀝니다."),
			tool.Baro_t("가입만 하고 기여가 없으면 완료로 기록되지 않습니다."),
			tool.Baro_t("자기 자신은 초대할 수 없습니다."),
			tool.Baro_t("창립 기여자가 된 사람 3명을 초대하면 <b>창립 동행자</b> 배지를 받습니다."),
		}),
	)

	body += `<script>
function baro_founding_copy(button) {
	var full = location.origin + (button.getAttribute('data-copy') || '');
	if (navigator.clipboard && navigator.clipboard.writeText) {
		navigator.clipboard.writeText(full).then(function () { alert('링크를 복사했습니다.\n' + full); });
		return;
	}
	window.prompt('아래 링크를 복사하세요', full);
}
</script>`

	return tool.Baro_page(db, config, "초대", body, baro_founding_menu())
}

// View_baro_founding_invite_use : 초대 코드 확인 (1-5)
func View_baro_founding_invite_use(config tool.Config, code string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/founding", tool.Baro_t("창립 시즌")},
		{"", tool.Baro_t("초대 확인")},
	})

	invite, found := baro_founding_invite_by_code(db, strings.TrimSpace(code))
	if !found {
		body += tool.Baro_hero(tool.Baro_t("초대 코드를 찾을 수 없습니다"), tool.Baro_t("코드가 잘못되었거나 이미 삭제된 초대입니다."), "")
		body += tool.Baro_alert("warn", tool.Baro_t("받은 링크를 다시 확인해 주세요. 초대 코드가 없어도 누구나 가입하고 기여할 수 있습니다."))
		body += tool.Baro_card(
			tool.Baro_t("그래도 시작할 수 있습니다"),
			tool.Baro_button("/register", tool.Baro_t("가입하기"), "primary")+" "+
				tool.Baro_button("/baro/founding", tool.Baro_t("창립 시즌 보기"), "")+" "+
				tool.Baro_button("/baro/founding/field", tool.Baro_t("분야별 모집 보기"), ""),
		)

		return tool.Baro_page(db, config, tool.Baro_t("초대 확인"), body, baro_founding_menu())
	}

	me := config.IP
	logged_in := tool.Baro_is_user(me)
	self_invite := logged_in && invite.Owner == me

	// 로그인 상태로 들어오면 초대 대상으로 기록한다. (자기 자신 초대는 금지)
	recorded := false
	if logged_in && !self_invite && invite.Status == "open" && invite.Target == "" && !tool.Baro_read_only(db) {
		tool.Exec_DB(db, "update baro_invite set target = ? where id = ? and target = ''", me, invite.ID)
		invite.Target = me
		recorded = true

		tool.Baro_notify(
			db, invite.Owner, "system",
			tool.Baro_t("초대 링크를 ")+me+tool.Baro_t(" 님이 열었습니다."),
			"/baro/invite", "invite_open",
		)
	}

	body += tool.Baro_hero(
		tool.Baro_t("바로위키 초대장"),
		tool.Baro_t("창립 시즌에 함께 기록할 사람을 찾고 있습니다."),
		"",
	)
	body += tool.Baro_card_ex(
		tool.Baro_t("초대한 사람"),
		tool.Baro_t("창립 시즌에 함께하자는 초대입니다."),
		tool.Baro_kv_grid([][]string{
			{tool.Baro_t("초대한 사람"), tool.Baro_user_link(db, invite.Owner, config.IP)},
			{tool.Baro_t("초대 코드"), `<code>` + tool.HTML_escape(invite.Code) + `</code>`},
			{tool.Baro_t("만든 날"), tool.Baro_time_cell(invite.Date)},
			{"상태", func() string {
				switch invite.Status {
				case "done":
					return tool.Baro_badge(tool.Baro_t("완료"), "ok")
				case "closed":
					return tool.Baro_badge(tool.Baro_t("닫힘"), "muted")
				}

				return tool.Baro_badge(tool.Baro_t("사용 가능"), "info")
			}()},
		}),
		"",
	)

	switch {
	case self_invite:
		body += tool.Baro_alert("danger", tool.Baro_t("자기 자신은 초대할 수 없습니다. 이 링크는 다른 사람에게 전달해 주세요."))
	case invite.Status == "closed":
		body += tool.Baro_alert("warn", tool.Baro_t("이 초대 링크는 닫혔습니다. 가입과 기여는 초대 없이도 할 수 있습니다."))
	case !logged_in:
		body += tool.Baro_alert("info", tool.Baro_t("가입한 뒤 이 링크를 다시 열면 초대가 연결됩니다."))
	case recorded:
		body += tool.Baro_alert("ok", tool.Baro_t("초대가 연결되었습니다. 첫 유효 기여를 마치면 초대가 완료로 기록됩니다."))
	case invite.Target == me:
		body += tool.Baro_alert("ok", tool.Baro_t("이미 이 초대에 연결되어 있습니다."))
	default:
		body += tool.Baro_alert("warn", tool.Baro_t("이 초대 링크는 이미 다른 사람이 사용했습니다. 가입과 기여는 초대 없이도 할 수 있습니다."))
	}

	if logged_in {
		body += tool.Baro_card(
			tool.Baro_t("다음 할 일"),
			tool.Baro_button("/baro/founding/apply", tool.Baro_t("창립 기여자로 지원하기"), "primary")+" "+
				tool.Baro_button("/baro/interest", tool.Baro_t("관심 분야 고르기"), "")+" "+
				tool.Baro_button("/baro/founding/check", tool.Baro_t("내 지원 상태"), ""),
		)
	} else {
		body += tool.Baro_card(
			tool.Baro_t("먼저 가입해 주세요"),
			tool.Baro_t(`<p>바로위키의 기여 기능은 로그인한 사용자만 쓸 수 있습니다. 가입한 뒤 이 초대 링크를 다시 열면 초대가 연결됩니다.</p>`)+
				tool.Baro_button("/register", tool.Baro_t("가입하기"), "primary")+" "+
				tool.Baro_button("/login", tool.Baro_t("로그인"), "")+" "+
				tool.Baro_button("/baro/founding", tool.Baro_t("창립 시즌 먼저 보기"), ""),
		)
	}

	body += baro_founding_invite_policy_html()
	body += tool.Baro_card(tool.Baro_t("창립 기여자 혜택"), baro_founding_benefit_html())

	return tool.Baro_page(db, config, tool.Baro_t("초대 확인"), body, baro_founding_menu())
}
