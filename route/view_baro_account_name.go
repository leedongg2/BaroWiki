package route

import (
	"database/sql"
	"strconv"
	"time"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 [account] 모듈 — 계정 고유 번호 · 계정명 변경 이력
//
//  계정 이름은 바뀐다. 그래서 「3년 전에 이 문서를 쓴 사람」 을 이름으로만 가리키면
//  이름을 바꾼 순간 이어지지 않는다. 계정마다 변하지 않는 고유 번호를 하나 두고,
//  이름이 바뀔 때마다 그 번호에 이력을 매단다.
//
//  이름은 이미 역사 · 토론 어디에나 남는 공개 정보라서 이력도 공개로 둔다.
//  대신 가입 아이피 · 이메일 같은 것은 여기에 싣지 않는다. (5번 원칙)
// ============================================================

// baro_account_name_row : 이름 하나가 쓰인 구간
type baro_account_name_row struct {
	Name  string // 그때 쓰던 이름
	Start string // 사용 시작
	End   string // 사용 중단 ("" 면 지금도 쓰는 중)
}

// baro_account_name_duration : 사용 기간을 사람이 읽는 말로
//
//	끝이 비어 있으면 지금까지로 친다.
func baro_account_name_duration(start string, end string) string {
	if start == "" {
		return "-"
	}

	from := tool.Baro_parse_time(start)
	if from.IsZero() {
		return "-"
	}

	to := time.Now()
	if end != "" {
		to = tool.Baro_parse_time(end)
		if to.IsZero() {
			return "-"
		}
	}

	gap := to.Sub(from)
	if gap < 0 {
		return "-"
	}

	days := int(gap.Hours()) / 24
	hours := int(gap.Hours()) % 24
	minutes := int(gap.Minutes()) % 60

	if days > 0 {
		return strconv.Itoa(days) + tool.Baro_t("일 ") + strconv.Itoa(hours) + tool.Baro_t("시간")
	}
	if hours > 0 {
		return strconv.Itoa(hours) + tool.Baro_t("시간 ") + strconv.Itoa(minutes) + tool.Baro_t("분")
	}

	return strconv.Itoa(minutes) + tool.Baro_t("분")
}

// baro_account_name_target : 주소로 들어온 값을 계정 이름으로 바꾼다
//
//	계정 이름도 되고 계정 고유 번호도 된다. 비어 있으면 보고 있는 본인이다.
func baro_account_name_target(db *sql.DB, config tool.Config, user string) string {
	if user == "" {
		return config.IP
	}

	if tool.Baro_uuid_is(user) {
		if found := tool.Baro_user_from_uuid(db, user); found != "" {
			return found
		}

		return ""
	}

	return user
}

// baro_account_name_history : 이름이 쓰인 구간을 시간순으로 (가입 → 지금)
func baro_account_name_history(db *sql.DB, user string, user_uuid string) []baro_account_name_row {
	// 가입 시각이 첫 이름의 시작이다. 모르면 첫 변경 시각을 시작으로 둔다.
	start := ""
	tool.QueryRow_DB(db, "select data from user_set where id = ? and name = 'date'", []any{&start}, user)

	rows := tool.Query_DB(
		db,
		"select old_name, new_name, date from baro_user_name_log where uuid = ? order by date asc, id + 0 asc",
		user_uuid,
	)
	defer rows.Close()

	list := []baro_account_name_row{}
	last_end := start

	for rows.Next() {
		old_name, new_name, date := "", "", ""
		if rows.Scan(&old_name, &new_name, &date) != nil {
			continue
		}

		_ = new_name
		list = append(list, baro_account_name_row{Name: old_name, Start: last_end, End: date})
		last_end = date
	}

	// 마지막 줄은 지금 쓰는 이름이다
	now_name := ""
	tool.QueryRow_DB(db, "select data from user_set where id = ? and name = 'user_name'", []any{&now_name}, user)
	if now_name == "" {
		now_name = user
	}

	list = append(list, baro_account_name_row{Name: now_name, Start: last_end, End: ""})

	return list
}

// View_baro_account_name : 계정 고유 번호와 계정명 변경 이력
func View_baro_account_name(config tool.Config, user string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	target := baro_account_name_target(db, config, user)
	if target == "" {
		return tool.Baro_deny(db, config, tool.Baro_t("그 고유 번호를 가진 계정이 없습니다."))
	}
	if tool.IP_or_user(target) {
		return tool.Baro_deny(db, config, tool.Baro_t("비로그인 아이피에는 계정 고유 번호가 없습니다."))
	}

	user_uuid := tool.Baro_user_uuid(db, target)
	if user_uuid == "" {
		return tool.Baro_deny(db, config, tool.Baro_t("계정 고유 번호를 읽지 못했습니다."))
	}

	info := [][]string{
		{tool.Baro_t("지금 계정"), tool.Baro_user_link(db, target, config.IP)},
		{tool.Baro_t("계정 고유 번호"), tool.Baro_code(user_uuid)},
	}

	register_date := ""
	tool.QueryRow_DB(db, "select data from user_set where id = ? and name = 'date'", []any{&register_date}, target)
	if register_date != "" {
		info = append(info, []string{tool.Baro_t("가입"), tool.Baro_time_cell(register_date)})
	}

	body := tool.Baro_hero(tool.Baro_t("계정명 변경 이력"), target, "")
	body += tool.Baro_card("", tool.Baro_dl(info))

	list := baro_account_name_history(db, target, user_uuid)

	table_rows := [][]string{}
	for _, one := range list {
		start_cell := "-"
		if one.Start != "" {
			start_cell = tool.Baro_time_cell(one.Start)
		}

		end_cell := tool.Baro_badge(tool.Baro_t("사용 중"), "ok")
		if one.End != "" {
			end_cell = tool.Baro_time_cell(one.End)
		}

		table_rows = append(table_rows, []string{
			"<b>" + tool.HTML_escape(one.Name) + "</b>",
			start_cell,
			end_cell,
			tool.HTML_escape(baro_account_name_duration(one.Start, one.End)),
		})
	}

	body += tool.Baro_card(
		tool.Baro_t("쓴 이름"),
		tool.Baro_table(
			[]string{tool.Baro_t("이름"), tool.Baro_t("사용 시작"), tool.Baro_t("사용 중단"), tool.Baro_t("사용 기간")},
			table_rows,
			tool.Baro_t("이름을 바꾼 적이 없습니다."),
		),
	)

	body += tool.Baro_note(tool.Baro_t(
		"계정 고유 번호는 이름을 바꿔도 달라지지 않습니다. " +
			"이 쪽 주소에 이름 대신 고유 번호를 넣어도 같은 사람을 가리킵니다.",
	))

	if target == config.IP && tool.Baro_is_user(config.IP) {
		body += tool.Baro_card("", tool.Baro_button("/change/user_name", tool.Baro_t("이름 바꾸기"), ""))
	}

	return tool.Baro_page(
		db, config,
		tool.Baro_t("계정명 변경 이력"),
		body,
		[][]any{{"baro/account", tool.Baro_t("내 계정")}, {"baro", "바로위키"}},
	)
}
