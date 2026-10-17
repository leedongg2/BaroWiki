package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  9-3 안정판 — /baro/quality/stable
//
//  논쟁이 잦거나 중요한 문서는 "지금 보이는 최신 편집판" 과
//  "검토를 마친 안정판" 을 구분할 수 있어야 한다.
//  안정판은 최신판을 감추지 않는다. 어느 쪽을 볼지는 읽는 사람이 고른다.
// ============================================================

// baro_quality_stable_off : 기능 플래그가 꺼져 있을 때의 안내 화면
func baro_quality_stable_off(db *sql.DB, config tool.Config, page_title string) string {
	body := baro_quality_crumb(tool.Baro_t("안정판"))
	body += tool.Baro_hero(tool.Baro_t("안정판"), tool.Baro_t("논쟁 문서의 검토 완료판을 따로 표시하는 기능입니다."), "")
	body += baro_quality_nav("/baro/quality/stable")
	body += tool.Baro_alert("warn",
		tool.Baro_t("안정판 기능(stable_version)이 지금 이 계정에게 꺼져 있습니다. 기능 플래그가 켜지면 이 화면에서 안정판을 지정하고 확인할 수 있습니다."))

	body += tool.Baro_card(tool.Baro_t("안정판이란"), `<p>같은 문서라도 지금 이 순간의 최신 편집판과, 검토자가 확인을 마친 판은 다를 수 있습니다.
바로위키는 최신판을 감추지 않고 두 판을 <b>나란히</b> 보여 줍니다.</p>`+
		tool.Baro_list([]string{
			tool.Baro_t("안정판 지정은 편집을 막는 보호 조치가 아닙니다. 누구나 계속 편집할 수 있습니다."),
			tool.Baro_t("안정판보다 최신판이 앞서 있으면 목록에서 강조해 다시 검토하도록 알립니다."),
			tool.Baro_t("안정판 지정과 해제는 공개 운영 로그에 사유와 함께 남습니다."),
		}))

	body += tool.Baro_card(tool.Baro_t("다음 할 일"), tool.Baro_button("/baro/quality", tool.Baro_t("문서 상태"), "primary")+" "+
		tool.Baro_button("/baro/review", tool.Baro_t("검토 큐"), ""))

	return tool.Baro_page(db, config, page_title, body, baro_quality_menu())
}

// View_baro_quality_stable_list : 안정판이 지정된 문서 목록
func View_baro_quality_stable_list(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_flag_on(db, config.IP, "stable_version") {
		return baro_quality_stable_off(db, config, tool.Baro_t("안정판"))
	}

	page_num := baro_quality_page_num(page)
	offset := (page_num - 1) * tool.Baro_page_size

	total := baro_quality_count(db, "select count(*) from baro_stable")

	rows := tool.Query_DB(db,
		"select title, rev, actor, date, note from baro_stable order by date desc limit ?, ?",
		offset, tool.Baro_page_size)
	defer rows.Close()

	type stable_row struct {
		Title, Rev, Actor, Date, Note string
	}

	list := []stable_row{}
	for rows.Next() {
		row := stable_row{}
		if err := rows.Scan(&row.Title, &row.Rev, &row.Actor, &row.Date, &row.Note); err != nil {
			continue
		}

		list = append(list, row)
	}

	table_rows := [][]string{}
	behind := 0

	for _, row := range list {
		latest := baro_quality_rev(db, row.Title)

		state := tool.Baro_badge(tool.Baro_t("최신판과 같음"), "ok")
		gap := tool.Str_to_int(latest) - tool.Str_to_int(row.Rev)
		if gap > 0 {
			behind++
			state = tool.Baro_badge(tool.Baro_t("최신판이 ")+tool.Baro_itoa(gap)+tool.Baro_t("판 앞섬"), "warn")
		}

		table_rows = append(table_rows, []string{
			baro_quality_doc_cell(row.Title),
			`<a href="/render/` + tool.Url_parser(row.Rev) + `/` + tool.Url_parser(row.Title) + `">r` + tool.HTML_escape(row.Rev) + `</a>`,
			"r" + tool.HTML_escape(latest),
			state,
			tool.Baro_user_link(db, row.Actor, config.IP),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(row.Note), 50)),
			tool.Baro_time_cell(row.Date),
			tool.Baro_button("/baro/quality/stable/set?title="+tool.Url_parser(row.Title), "바꾸기", "") + " " +
				baro_quality_stable_diff(row.Title, row.Rev, latest),
		})
	}

	list_html := tool.Baro_table(
		[]string{"문서", tool.Baro_t("안정판"), tool.Baro_t("최신판"), "상태", tool.Baro_t("지정자"), tool.Baro_t("메모"), tool.Baro_t("지정 시각"), "관리"},
		table_rows, "")

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("안정판이 지정된 문서가 아직 없습니다. 논쟁이 잦은 문서부터 지정해 보세요."))
	}

	list_html += tool.Baro_paging(page_num, len(table_rows), tool.Baro_page_size, "/baro/quality/stable?page={}")

	body := baro_quality_crumb(tool.Baro_t("안정판"))
	body += tool.Baro_hero(
		tool.Baro_t("안정판"),
		tool.Baro_t("검토를 마친 판을 따로 표시합니다. 최신판을 감추지 않습니다."),
		tool.Baro_button("/baro/quality/stable/set", tool.Baro_t("안정판 지정·해제"), "primary")+" "+
			tool.Baro_button("/baro/review", tool.Baro_t("검토 큐"), ""),
	)
	body += baro_quality_nav("/baro/quality/stable")

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("안정판 지정 문서"), tool.Baro_comma(total), ""},
		{tool.Baro_t("이 쪽에서 뒤처진 문서"), tool.Baro_comma(behind), tool.Baro_t("최신판이 앞서 있음")},
	})

	if behind > 0 {
		body += tool.Baro_alert("warn",
			tool.Baro_t("안정판보다 최신판이 앞서 있는 문서가 이 쪽에 ")+tool.Baro_itoa(behind)+tool.Baro_t("건 있습니다. 새 편집을 검토해 안정판을 갱신해 주세요."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("안정판 목록"), tool.Baro_t("최근에 지정된 순서입니다."), list_html, "")

	body += tool.Baro_card(tool.Baro_t("안정판 운영 원칙"), tool.Baro_list([]string{
		tool.Baro_t("안정판은 편집 제한이 아닙니다. 지정된 문서도 누구나 계속 편집할 수 있습니다."),
		tool.Baro_t("안정판 지정·해제는 검토 권한이 있는 사용자만 할 수 있고, 공개 운영 로그에 남습니다."),
		tool.Baro_t("최신판이 안정판보다 앞서 있으면 다시 검토해 안정판을 올려 주세요."),
		tool.Baro_t("논쟁 중 상태인 문서라면 안정판을 함께 지정해 두면 읽는 사람이 덜 혼란스럽습니다."),
	}))

	return tool.Baro_page(db, config, tool.Baro_t("안정판"), body, baro_quality_menu())
}

// baro_quality_stable_diff : 안정판과 최신판 사이 차이 링크
func baro_quality_stable_diff(title string, stable string, latest string) string {
	if tool.Str_to_int(latest) <= tool.Str_to_int(stable) {
		return ""
	}

	return `<a class="bw_btn" href="/diff/` + tool.Url_parser(stable) + `/` + tool.Url_parser(latest) + `/` +
		tool.Url_parser(title) + tool.Baro_t(`">차이 보기</a>`)
}

// View_baro_quality_stable_set : 안정판 지정 / 해제 (권한 review)
func View_baro_quality_stable_set(config tool.Config, title string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_flag_on(db, config.IP, "stable_version") {
		return baro_quality_stable_off(db, config, "안정판 지정")
	}

	if wall, need := tool.Baro_login_wall(db, config, "/baro/quality/stable/set"); need {
		return wall
	}

	if !tool.Baro_can(db, config.IP, "review") {
		return tool.Baro_deny(db, config, tool.Baro_t("안정판 지정은 검토 권한이 있는 사용자만 할 수 있습니다."))
	}

	title = strings.TrimSpace(title)
	message := ""

	if values != nil {
		if tool.Baro_read_only(db) {
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태입니다."))
		} else {
			post_title := tool.Namespace_normalize(baro_quality_val(values, "title"))
			post_rev := baro_quality_val(values, "rev")
			post_note := tool.Baro_trunc(baro_quality_val(values, "note"), 500)
			act := baro_quality_val(values, "act")

			latest := ""
			if post_title != "" {
				latest = baro_quality_rev(db, post_title)
			}

			before_rev := ""
			tool.QueryRow_DB(db, "select rev from baro_stable where title = ?", []any{&before_rev}, post_title)

			switch {
			case post_title == "":
				message = tool.Baro_alert("danger", tool.Baro_t("문서 이름을 입력하세요."))
			case act == "clear":
				if before_rev == "" {
					message = tool.Baro_alert("warn", tool.Baro_t("이 문서에는 안정판이 지정되어 있지 않습니다."))
					break
				}

				tool.Exec_DB(db, "delete from baro_stable where title = ?", post_title)
				tool.Baro_public_log(db, "stable_clear", config.IP, post_title, post_note, "r"+before_rev+" 해제")
				tool.Baro_audit(db, config, "stable_clear", post_title, before_rev, "", post_note)

				return tool.Get_redirect("/baro/quality/stable")
			case !baro_quality_doc_exists(db, post_title):
				message = tool.Baro_alert("danger", tool.Baro_t("본문이 없는 문서에는 안정판을 지정할 수 없습니다."))
			case tool.Str_to_int(post_rev) < 1:
				message = tool.Baro_alert("danger", tool.Baro_t("판 번호는 1 이상의 숫자여야 합니다."))
			case tool.Str_to_int(post_rev) > tool.Str_to_int(latest):
				message = tool.Baro_alert("danger", tool.Baro_t("이 문서의 최신 판은 r")+tool.HTML_escape(latest)+tool.Baro_t(" 입니다. 그보다 큰 판은 지정할 수 없습니다."))
			case !baro_quality_stable_rev_exists(db, post_title, post_rev):
				message = tool.Baro_alert("danger", tool.Baro_t("그 판이 역사에 없습니다. 판 번호를 다시 확인하세요."))
			default:
				tool.Exec_DB(db, "delete from baro_stable where title = ?", post_title)
				tool.Exec_DB(db,
					"insert into baro_stable (title, rev, actor, date, note) values (?, ?, ?, ?, ?)",
					post_title, post_rev, config.IP, tool.Baro_now(), post_note)

				tool.Baro_public_log(db, "stable_set", config.IP, post_title, post_note, "r"+post_rev+" 안정판 지정")
				tool.Baro_audit(db, config, "stable_set", post_title, before_rev, post_rev, post_note)

				// 검토 완료 상태가 없으면 안내만 남긴다 (자동으로 상태를 바꾸지 않는다)
				return tool.Get_redirect("/baro/quality/stable")
			}
		}

		title = tool.Namespace_normalize(baro_quality_val(values, "title"))
	}

	current_rev, current_note := "", ""
	has_stable := false
	latest := ""

	if title != "" {
		has_stable = tool.QueryRow_DB(db,
			"select rev, note from baro_stable where title = ?",
			[]any{&current_rev, &current_note}, title)
		latest = baro_quality_rev(db, title)
	}

	default_rev := current_rev
	if default_rev == "" {
		default_rev = latest
	}

	form := tool.Baro_input("title", tool.Baro_t("문서 이름"), title, tool.Baro_t("예: 바로위키"), tool.Baro_t("안정판을 지정할 문서입니다."))
	form += tool.Baro_input("rev", tool.Baro_t("판 번호"), default_rev, tool.Baro_t("예: 12"), tool.Baro_t("검토를 마친 판의 번호를 적습니다. (문서 역사에서 확인)"))
	form += tool.Baro_textarea("note", tool.Baro_t("메모 / 사유"), current_note, 3, tool.Baro_t("무엇을 검토했는지 적어 주세요."), tool.Baro_t("이 메모는 공개 운영 로그에 남습니다."))
	form += `<div class="bw_f">` +
		tool.Baro_t(`<button type="submit" class="bw_btn bw_btn_primary" name="act" value="set">안정판으로 지정</button> `) +
		tool.Baro_t(`<button type="submit" class="bw_btn bw_btn_danger" name="act" value="clear">안정판 해제</button>`) +
		`</div>`

	body := baro_quality_crumb("안정판 지정")
	body += tool.Baro_hero(tool.Baro_t("안정판 지정·해제"), tool.Baro_t("검토를 마친 판을 안정판으로 표시합니다."), "")
	body += baro_quality_nav("/baro/quality/stable")
	body += message

	if title != "" {
		state := tool.Baro_badge(tool.Baro_t("지정 안 됨"), "muted")
		if has_stable {
			state = tool.Baro_badge("r"+tool.HTML_escape(current_rev), "ok")
		}

		body += tool.Baro_kv_grid([][]string{
			{"문서", baro_quality_doc_cell(title)},
			{tool.Baro_t("현재 안정판"), state},
			{tool.Baro_t("최신 판"), "r" + tool.HTML_escape(latest)},
			{tool.Baro_t("문서 상태"), tool.Baro_status_badge(tool.Baro_doc_status_get(db, title))},
		})

		body += baro_quality_stable_recent(db, title, config.IP)
	}

	body += tool.Baro_card_ex("안정판 지정", tool.Baro_t("검토 권한이 있는 사용자만 사용할 수 있습니다."),
		tool.Baro_form("/baro/quality/stable/set", form),
		tool.Baro_button("/baro/quality/stable", tool.Baro_t("안정판 목록"), ""))

	body += tool.Baro_note(tool.Baro_t("안정판을 지정해도 문서 편집은 계속 열려 있습니다. 편집을 막는 보호 기능이 아닙니다."))

	return tool.Baro_page(db, config, "안정판 지정", body, baro_quality_menu())
}

// baro_quality_stable_rev_exists : 그 판이 역사에 있는가
func baro_quality_stable_rev_exists(db *sql.DB, title string, rev string) bool {
	found := ""

	return tool.QueryRow_DB(db, "select id from history where title = ? and id = ?", []any{&found}, title, rev)
}

// baro_quality_stable_recent : 안정판 후보로 고를 만한 최근 판 목록
//
//	편집자 칸은 공개 화면이므로 IP 를 그대로 쓰지 않는다 (0장 원칙 5).
func baro_quality_stable_recent(db *sql.DB, title string, my_ip string) string {
	rows := tool.Query_DB(db,
		"select id, date, ip, send, leng from history where title = ? order by id + 0 desc limit 10",
		title)
	defer rows.Close()

	table_rows := [][]string{}
	for rows.Next() {
		id, date, ip, send, leng := "", "", "", "", ""
		if err := rows.Scan(&id, &date, &ip, &send, &leng); err != nil {
			continue
		}

		table_rows = append(table_rows, []string{
			`<a href="/render/` + tool.Url_parser(id) + `/` + tool.Url_parser(title) + `">r` + tool.HTML_escape(id) + `</a>`,
			tool.Baro_user_link(db, ip, my_ip),
			tool.HTML_escape(tool.Baro_trunc(tool.Baro_one_line(send), 50)),
			baro_quality_leng_cell(leng),
			tool.Baro_time_cell(date),
		})
	}

	return tool.Baro_card(tool.Baro_t("최근 판 (안정판 후보)"), tool.Baro_table(
		[]string{"판", tool.Baro_t("편집자"), tool.Baro_t("요약"), "길이 변화", "시각"},
		table_rows, tool.Baro_t("이 문서에는 편집 기록이 없습니다.")))
}
