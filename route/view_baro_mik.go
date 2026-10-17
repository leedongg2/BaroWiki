package route

import (
	"database/sql"
	"net/url"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  밐 (Mik) — 내 지갑  /baro/mik
//
//  잔액 · 오간 기록 · 칭호 고르기 · 양도를 한 화면에서 본다.
//  얻는 길과 쓰는 길이 모두 여기서 설명된다.
// ============================================================

// baro_mik_crumb : 빵부스러기
func baro_mik_crumb(now string) [][]string {
	return [][]string{{"/baro", "바로위키"}, {"/baro/mik", "밐"}, {"", now}}
}

// baro_mik_menu : 화면 아래 차림표
func baro_mik_menu() [][]any {
	return [][]any{{"/baro", "바로위키"}, {"/baro/mik", "밐"}, {"/baro/shop", "바로샵"}}
}

// baro_mik_tabs : 탭
func baro_mik_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/mik", tool.Baro_t("내 지갑")},
		{"/baro/mik/log", tool.Baro_t("오간 기록")},
		{"/baro/mik/send", tool.Baro_t("양도")},
		{"/baro/mik/rank", tool.Baro_t("밐 순위")},
		{"/baro/shop", "바로샵"},
	}, active)
}

// baro_mik_how_html : 밐을 얻는 길 안내
func baro_mik_how_html(db *sql.DB) string {
	return tool.Baro_list([]string{
		tool.Baro_t_esc("접속 보상 — 로그인한 채로 위키를 열면 하루에 한 번 ") +
			tool.HTML_escape(tool.Baro_mik_text(tool.Baro_mik_attend_amount(db))) +
			tool.Baro_t_esc(" 이 들어옵니다."),
		tool.Baro_t_esc("도전 과제 — 배지를 새로 받을 때마다 밐이 들어옵니다. (기본 ") +
			tool.HTML_escape(tool.Baro_mik_text(tool.Baro_mik_badge_default(db))) +
			tool.Baro_t_esc(", 배지마다 따로 정해 둘 수 있습니다)"),
		tool.Baro_t_esc("관리자 지급 — 운영진이 사유를 적어 직접 줄 수 있습니다."),
		tool.Baro_t_esc("양도 — 다른 사용자에게서 받을 수 있습니다."),
	})
}

// View_baro_mik : 내 지갑 (/baro/mik)
func View_baro_mik(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, stop := tool.Baro_login_wall(db, config, "/baro/mik"); stop {
		return wall
	}

	me := config.IP
	message := ""

	// 칭호 바꾸기
	if values != nil && values.Get("do") == "title" {
		want := strings.TrimSpace(values.Get("title"))

		switch {
		case want == "":
			tool.Baro_mik_title_set(db, me, "")
			message = tool.Baro_t("칭호를 뗐습니다.")
		case baro_mik_has_title(db, me, want):
			tool.Baro_mik_title_set(db, me, want)
			message = tool.Baro_t("칭호를 바꿨습니다.")
		default:
			message = tool.Baro_t("가지고 있지 않은 칭호입니다.")
		}
	}

	balance := tool.Baro_mik_get(db, me)
	uuid := tool.Baro_user_uuid(db, me)

	body := tool.Baro_crumb(baro_mik_crumb(tool.Baro_t("내 지갑")))
	body += tool.Baro_hero(
		tool.Baro_t("밐 지갑"),
		tool.Baro_t("바로위키에서 쓰는 가상 화폐입니다. 기여하면 쌓이고 바로샵에서 씁니다."),
		"",
	)
	body += baro_mik_tabs("/baro/mik")

	if message != "" {
		body += tool.Baro_alert("info", message)
	}

	// --- 잔액과 오늘 상태 ---
	attend_today := tool.Baro_t("아직 받지 않았습니다")
	if tool.Baro_mik_attend_date(db, me) == tool.Baro_today() {
		attend_today = tool.Baro_t("오늘 몫을 받았습니다")
	}

	captcha_state := tool.Baro_t("없음")
	if until := tool.Baro_mik_captcha_until(db, me); until != "" {
		captcha_state = tool.HTML_escape(tool.Baro_date_only(until)) + tool.Baro_t_esc(" 까지") +
			" (" + tool.HTML_escape(tool.Baro_duration_left(until)) + ")"
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("가진 밐"), tool.Baro_mik_text(balance), ""},
		{tool.Baro_t("접속 보상"), attend_today, tool.Baro_t("연속 ") + tool.Baro_itoa(tool.Baro_mik_attend_streak(db, me)) + tool.Baro_t("일째")},
		{tool.Baro_t("가진 칭호"), tool.Baro_itoa(len(tool.Baro_mik_title_list(db, me))) + tool.Baro_t("개"), ""},
	})

	body += tool.Baro_card(tool.Baro_t("캡차 면제"), tool.Baro_dl([][]string{
		{tool.Baro_t("남은 기간"), captcha_state},
	})+tool.Baro_note(tool.Baro_t("바로샵에서 「캡차 면제 15일권」 을 사면 그 동안 편집·올리기에서 캡차를 묻지 않습니다.")))

	// --- 칭호 고르기 ---
	body += baro_mik_title_card(db, me)

	// --- 얻는 길 ---
	body += tool.Baro_card(tool.Baro_t("밐을 얻는 길"), baro_mik_how_html(db))

	// --- 내 고유 번호 (양도받을 때 알려 준다) ---
	uuid_html := tool.Baro_note(tool.Baro_t("아직 고유 번호가 없습니다."))
	if uuid != "" {
		uuid_html = tool.Baro_code(uuid) +
			tool.Baro_note(tool.Baro_t("밐을 받으려면 이 번호를 상대에게 알려 주세요. 계정 이름을 바꿔도 번호는 그대로입니다."))
	}
	body += tool.Baro_card(tool.Baro_t("내 고유 번호"), uuid_html)

	// --- 최근 기록 ---
	body += tool.Baro_card_ex(
		tool.Baro_t("최근에 오간 밐"), "",
		baro_mik_log_table(db, me, 1, 10),
		`<a class="bw_btn" href="/baro/mik/log">`+tool.Baro_t_esc("전체 기록 보기")+`</a>`,
	)

	return tool.Baro_page(db, config, tool.Baro_t("밐 지갑"), body, baro_mik_menu())
}

// baro_mik_has_title : 이 칭호를 가지고 있는가
func baro_mik_has_title(db *sql.DB, user string, title string) bool {
	for _, one := range tool.Baro_mik_title_list(db, user) {
		if one == title {
			return true
		}
	}

	return false
}

// baro_mik_title_card : 칭호 고르기 칸
func baro_mik_title_card(db *sql.DB, user string) string {
	owned := tool.Baro_mik_title_list(db, user)
	if len(owned) == 0 {
		return tool.Baro_card(
			tool.Baro_t("칭호"),
			tool.Baro_empty(tool.Baro_t("아직 가진 칭호가 없습니다."))+
				tool.Baro_button("/baro/shop", tool.Baro_t("바로샵에서 칭호 사기"), ""),
		)
	}

	options := [][]string{{"", tool.Baro_t("(달지 않음)")}}
	for _, one := range owned {
		options = append(options, []string{one, one})
	}

	form := tool.Baro_hidden("do", "title")
	form += tool.Baro_select("title", tool.Baro_t("달 칭호"), options, tool.Baro_mik_title(db, user),
		tool.Baro_t("고른 칭호가 프로필과 사용자 정보 틀에 함께 보입니다."))
	form += tool.Baro_submit(tool.Baro_t("칭호 바꾸기"), "primary")

	// 가진 칭호가 실제로 어떤 색으로 보이는지 함께 늘어놓는다.
	marks := []string{}
	for _, one := range tool.Baro_mik_title_rows(db, user) {
		marks = append(marks, tool.Baro_mik_title_html(one))
	}

	inner := tool.Baro_form("/baro/mik", form)
	if len(marks) > 0 {
		inner += tool.Baro_pills(marks)
	}

	return tool.Baro_card(tool.Baro_t("칭호"), inner)
}

// baro_mik_log_table : 밐이 오간 기록 표
func baro_mik_log_table(db *sql.DB, user string, page int, size int) string {
	if page < 1 {
		page = 1
	}

	rows := tool.Query_DB(
		db,
		"select delta, balance, kind, reason, actor, date from baro_mik_log where user = ? order by id + 0 desc limit ?, ?",
		user, (page-1)*size, size,
	)
	defer rows.Close()

	out := [][]string{}
	for rows.Next() {
		delta, balance, kind, reason, actor, date := "", "", "", "", "", ""
		if rows.Scan(&delta, &balance, &kind, &reason, &actor, &date) != nil {
			continue
		}

		amount := tool.Str_to_int(delta)
		amount_html := tool.Baro_badge("+"+tool.Baro_comma(amount), "ok")
		if amount < 0 {
			amount_html = tool.Baro_badge(tool.Baro_comma(amount), "danger")
		}

		who := tool.Baro_t_esc("시스템")
		if actor != "" && actor != "system" {
			who = tool.HTML_escape(actor)
		}

		out = append(out, []string{
			tool.Baro_time_cell(date),
			tool.Baro_t_esc(tool.Baro_mik_kind_name(kind)),
			amount_html,
			tool.HTML_escape(tool.Baro_comma(tool.Str_to_int(balance))),
			tool.HTML_escape(tool.Baro_trunc(reason, 80)),
			who,
		})
	}

	return tool.Baro_table(
		[]string{tool.Baro_t("때"), tool.Baro_t("종류"), tool.Baro_t("증감"), tool.Baro_t("남은 밐"), tool.Baro_t("사유"), tool.Baro_t("한 사람")},
		out,
		tool.Baro_t("아직 오간 밐이 없습니다."),
	)
}

// View_baro_mik_log : 밐이 오간 기록 전체 (/baro/mik/log)
func View_baro_mik_log(config tool.Config, page_text string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, stop := tool.Baro_login_wall(db, config, "/baro/mik/log"); stop {
		return wall
	}

	page := tool.Str_to_int(page_text)
	if page < 1 {
		page = 1
	}

	count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_mik_log where user = ?", []any{&count}, config.IP)

	body := tool.Baro_crumb(baro_mik_crumb(tool.Baro_t("오간 기록")))
	body += tool.Baro_hero(tool.Baro_t("밐이 오간 기록"), tool.Baro_t("언제 무엇으로 얼마가 오갔는지 모두 남습니다."), "")
	body += baro_mik_tabs("/baro/mik/log")
	body += tool.Baro_card("", baro_mik_log_table(db, config.IP, page, tool.Baro_page_size))
	body += tool.Baro_paging(page, count, tool.Baro_page_size, "/baro/mik/log?page={}")

	return tool.Baro_page(db, config, tool.Baro_t("밐이 오간 기록"), body, baro_mik_menu())
}

// ------------------------------------------------------------
//  양도
// ------------------------------------------------------------

// View_baro_mik_send : 밐 양도 (/baro/mik/send)
//
//	받을 사람은 계정 이름이 아니라 고유 번호(UUID)로 지목한다.
//	이름은 바뀔 수 있어도 번호는 바뀌지 않으므로, 엉뚱한 사람에게 보낼 일이 없다.
func View_baro_mik_send(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, stop := tool.Baro_login_wall(db, config, "/baro/mik/send"); stop {
		return wall
	}

	me := config.IP
	alert := ""
	want_uuid := ""
	want_amount := ""

	if values != nil && values.Get("do") == "send" {
		want_uuid = strings.TrimSpace(values.Get("uuid"))
		want_amount = strings.TrimSpace(values.Get("amount"))

		done, text, kind := baro_mik_send_run(db, config, want_uuid, want_amount, values.Get("reason"))
		alert = tool.Baro_alert(kind, text)

		if done {
			want_uuid = ""
			want_amount = ""
		}
	}

	balance := tool.Baro_mik_get(db, me)

	body := tool.Baro_crumb(baro_mik_crumb(tool.Baro_t("양도")))
	body += tool.Baro_hero(
		tool.Baro_t("밐 양도"),
		tool.Baro_t("받을 사람의 고유 번호와 수량을 적습니다."),
		"",
	)
	body += baro_mik_tabs("/baro/mik/send")
	body += alert

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("지금 가진 밐"), tool.Baro_mik_text(balance), ""},
		{
			tool.Baro_t("오늘 더 보낼 수 있는 밐"),
			tool.Baro_mik_text(tool.Baro_mik_send_left(db, me)),
			tool.Baro_t("하루 한도 ") + tool.Baro_mik_text(tool.Baro_mik_send_limit(db)),
		},
	})

	form := tool.Baro_hidden("do", "send")
	form += tool.Baro_input("uuid", tool.Baro_t("받을 사람의 고유 번호"), want_uuid,
		"00000000-0000-4000-8000-000000000000",
		tool.Baro_t("상대가 자기 지갑 화면에서 알려 주는 번호입니다. 계정 이름으로는 보낼 수 없습니다."))
	form += tool.Baro_input_type("amount", tool.Baro_t("보낼 수량"), want_amount, "number",
		tool.Baro_t("가진 밐보다 많이 보낼 수 없습니다."))
	form += tool.Baro_input("reason", tool.Baro_t("메모 (선택)"), "", tool.Baro_t("예: 문서 도와줘서 고마워요"),
		tool.Baro_t("받는 사람에게 함께 전달되고 양쪽 기록에 남습니다."))
	form += tool.Baro_submit(tool.Baro_t("보내기"), "primary")

	body += tool.Baro_card(tool.Baro_t("보내기"), tool.Baro_form("/baro/mik/send", form))
	body += tool.Baro_card(tool.Baro_t("알아 두기"), tool.Baro_list([]string{
		tool.Baro_t_esc("한 번 보낸 밐은 스스로 되돌릴 수 없습니다. 번호를 꼭 다시 확인해 주세요."),
		tool.Baro_t_esc("보낸 기록과 받은 기록이 양쪽에 모두 남습니다."),
		tool.Baro_t_esc("자기 자신에게는 보낼 수 없습니다."),
		tool.Baro_t_esc("하루에 보낼 수 있는 양에 한도가 있습니다. 한도는 운영진이 정합니다."),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("밐 양도"), body, baro_mik_menu())
}

// baro_mik_send_run : 양도 처리. (성공 여부, 안내 문구, 안내 종류)
func baro_mik_send_run(db *sql.DB, config tool.Config, uuid string, amount_text string, reason string) (bool, string, string) {
	me := config.IP

	if tool.Baro_read_only(db) {
		return false, tool.Baro_t("지금은 읽기 전용입니다. 잠시 뒤에 다시 시도해 주세요."), "warn"
	}
	if !tool.Baro_rate_ok(db, "mik_send", me, 10, 3600) {
		return false, tool.Baro_t("잠시 뒤에 다시 시도해 주세요. (한 시간에 열 번까지 보낼 수 있습니다)"), "warn"
	}

	if uuid == "" {
		return false, tool.Baro_t("받을 사람의 고유 번호를 적어 주세요."), "warn"
	}

	// 번호 꼴이 아니면 그 번호를 가진 사람도 없다. 같은 말로 돌려준다.
	target := ""
	if tool.Baro_uuid_is(uuid) {
		target = tool.Baro_user_from_uuid(db, uuid)
	}
	if target == "" {
		return false, tool.Baro_t("해당 UUID를 가진 사용자는 없습니다."), "danger"
	}

	if target == me {
		return false, tool.Baro_t("자기 자신에게는 보낼 수 없습니다."), "warn"
	}

	amount := tool.Str_to_int(amount_text)
	if amount_text == "" || amount <= 0 {
		return false, tool.Baro_t("보낼 수량을 1 이상으로 적어 주세요."), "warn"
	}

	if tool.Baro_mik_get(db, me) < amount {
		return false, tool.Baro_t("가진 밐보다 많이 보낼 수 없습니다."), "warn"
	}

	// 하루 한도. 운영진이 /baro/mik/admin 에서 조정한다.
	limit := tool.Baro_mik_send_limit(db)
	if limit <= 0 {
		return false, tool.Baro_t("지금은 양도가 막혀 있습니다."), "warn"
	}

	if left := tool.Baro_mik_send_left(db, me); amount > left {
		return false, tool.Baro_t("하루에 보낼 수 있는 밐은 ") + tool.Baro_mik_text(limit) +
			tool.Baro_t(" 까지입니다. 오늘 더 보낼 수 있는 밐은 ") + tool.Baro_mik_text(left) +
			tool.Baro_t(" 입니다."), "warn"
	}

	memo := tool.Baro_trunc(strings.TrimSpace(reason), 200)

	// 먼저 빼고, 뺀 것이 확인되면 넣는다. (없는 밐이 생기지 않게)
	if !tool.Baro_mik_add(db, me, -amount, "send_out", baro_mik_send_memo("에게 보냄", target, memo), me, target) {
		return false, tool.Baro_t("보내지 못했습니다. 잔액을 다시 확인해 주세요."), "danger"
	}

	if !tool.Baro_mik_add(db, target, amount, "send_in", baro_mik_send_memo("에게서 받음", me, memo), me, me) {
		// 넣지 못했으면 뺀 것을 되돌린다.
		tool.Baro_mik_add(db, me, amount, "refund", "양도 실패로 되돌림", "system", target)

		return false, tool.Baro_t("보내지 못했습니다. 잠시 뒤에 다시 시도해 주세요."), "danger"
	}

	notify := tool.Baro_t("밐을 받았습니다 : ") + tool.Baro_mik_text(amount)
	tool.Baro_notify(db, target, "system", notify, "/baro/mik", memo)

	tool.Baro_audit(db, config, "baro_mik_send", target, "", strconv.Itoa(amount), memo)

	return true, tool.Baro_t("보냈습니다. ") + tool.Baro_mik_text(amount), "ok"
}

// baro_mik_send_memo : 양도 기록에 남길 사유 (저장값이므로 번역하지 않는다)
func baro_mik_send_memo(what string, who string, memo string) string {
	text := who + " " + what
	if memo != "" {
		text += " : " + memo
	}

	return text
}
