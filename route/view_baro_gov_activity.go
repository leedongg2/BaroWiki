package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  11-5 관리자 활동 기록  (/baro/role/activity)
//
//  이 표는 관리자 점수 놀이가 아니다. 책임성 지표로만 쓴다.
//  순위를 매기지 않고, 가나다 순으로만 늘어놓는다.
// ============================================================

// baro_gov_activity_idle_days : 이만큼 조치가 없으면 장기 미활동으로 본다
const baro_gov_activity_idle_days = 90

// baro_gov_activity_cache_hours : 캐시를 다시 계산하는 간격
const baro_gov_activity_cache_hours = 1

// baro_gov_activity_row : 담당자 한 명의 활동 지표
type baro_gov_activity_row struct {
	User         string
	Roles        []string
	Cases        int // 담당한 사건 수
	Closed       int // 종결한 사건 수
	AvgHours     int // 평균 처리 시간 (시) — 계산 불가면 -1
	Actions      int // 사건 조치 수
	Appeals      int // 내 사건에 들어온 이의제기 수
	AppealAccept int // 그중 인용된 수
	Mass         int // 대량 조치 수
	RoleChange   int // 권한 변경 횟수
	Coi          int // 이해충돌 회피 선언 수
	LastAction   string
	IdleDays     int
}

// baro_gov_activity_accept_words : 이의제기 인용으로 보는 결과 값
var baro_gov_activity_accept_words = map[string]bool{
	"accept": true, "accepted": true, "인용": true, "overturn": true, "취소": true, "reduce": true, "감경": true,
}

// baro_gov_activity_targets : 활동을 살필 대상 (역할 보유자 + 실제로 조치한 사람)
func baro_gov_activity_targets(db *sql.DB) []string {
	seen := map[string]bool{}
	out := []string{}

	add := func(user string) {
		user = strings.TrimSpace(user)
		if user == "" || seen[user] || !tool.Baro_is_user(user) {
			return
		}

		seen[user] = true
		out = append(out, user)
	}

	for _, query := range []string{
		"select distinct user from baro_role",
		"select distinct actor from baro_case_action",
		"select distinct actor from baro_case",
		"select distinct actor from baro_mass_op",
		"select distinct reviewer from baro_appeal",
	} {
		rows := tool.Query_DB(db, query)
		for rows.Next() {
			user := ""
			if err := rows.Scan(&user); err == nil {
				add(user)
			}
		}
		rows.Close()
	}

	sort.Strings(out)

	return out
}

// baro_gov_activity_calc : 담당자 한 명의 지표를 계산한다
func baro_gov_activity_calc(db *sql.DB, user string) baro_gov_activity_row {
	row := baro_gov_activity_row{User: user, AvgHours: -1, IdleDays: -1}

	row.Roles = tool.Baro_user_roles(db, user)
	sort.Strings(row.Roles)

	row.Cases = baro_gov_count(db, "select count(*) from baro_case where actor = ?", user)
	row.Actions = baro_gov_count(db, "select count(*) from baro_case_action where actor = ?", user)
	row.Mass = baro_gov_count(db, "select count(*) from baro_mass_op where actor = ?", user)
	row.RoleChange = baro_gov_count(db, "select count(*) from baro_public_log where actor = ? and (kind = 'role_grant' or kind = 'role_revoke')", user)
	row.Coi = baro_gov_count(db, "select count(*) from baro_coi where actor = ?", user)

	// 평균 처리 시간 — 시작 시각과 종결 시각이 모두 있는 사건만 센다
	total_hours := 0
	counted := 0

	case_rows := tool.Query_DB(db, "select date, close_date from baro_case where actor = ?", user)
	for case_rows.Next() {
		start, end := "", ""
		if err := case_rows.Scan(&start, &end); err != nil {
			continue
		}

		if strings.TrimSpace(end) == "" {
			continue
		}

		row.Closed++

		hours := baro_gov_hours_between(start, end)
		if hours < 0 {
			continue
		}

		total_hours += hours
		counted++
	}
	case_rows.Close()

	if counted > 0 {
		row.AvgHours = total_hours / counted
	}

	// 이의제기 인용률 — 내가 담당한 사건에 들어온 이의제기 기준
	appeal_rows := tool.Query_DB(
		db,
		"select a.status, a.result from baro_appeal a, baro_case c where a.case_id = c.id and c.actor = ?",
		user,
	)
	for appeal_rows.Next() {
		status, result := "", ""
		if err := appeal_rows.Scan(&status, &result); err != nil {
			continue
		}

		row.Appeals++

		key := strings.ToLower(strings.TrimSpace(result))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(status))
		}

		if baro_gov_activity_accept_words[key] {
			row.AppealAccept++
		}
	}
	appeal_rows.Close()

	// 마지막 조치 시각
	last := ""
	for _, query := range [][]string{
		{"select date from baro_case_action where actor = ? order by date desc limit 1"},
		{"select date from baro_public_log where actor = ? order by date desc limit 1"},
		{"select date from baro_case where actor = ? order by date desc limit 1"},
	} {
		one := ""
		if tool.QueryRow_DB(db, query[0], []any{&one}, user) && one > last {
			last = one
		}
	}

	row.LastAction = last
	if last != "" {
		days := tool.Baro_days_between(last, tool.Baro_now())
		if days < 0 {
			days = 0
		}

		row.IdleDays = days
	}

	return row
}

// baro_gov_activity_encode : 캐시에 넣을 형태로
func baro_gov_activity_encode(row baro_gov_activity_row) string {
	return tool.Baro_json_enc(map[string]string{
		"cases":         tool.Baro_itoa(row.Cases),
		"closed":        tool.Baro_itoa(row.Closed),
		"avg_hours":     tool.Baro_itoa(row.AvgHours),
		"actions":       tool.Baro_itoa(row.Actions),
		"appeals":       tool.Baro_itoa(row.Appeals),
		"appeal_accept": tool.Baro_itoa(row.AppealAccept),
		"mass":          tool.Baro_itoa(row.Mass),
		"role_change":   tool.Baro_itoa(row.RoleChange),
		"coi":           tool.Baro_itoa(row.Coi),
		"last_action":   row.LastAction,
		"idle_days":     tool.Baro_itoa(row.IdleDays),
	})
}

// baro_gov_activity_decode : 캐시에서 읽기
func baro_gov_activity_decode(db *sql.DB, user string, data string) baro_gov_activity_row {
	values := tool.Baro_json_dec_str(data)

	row := baro_gov_activity_row{User: user}
	row.Roles = tool.Baro_user_roles(db, user)
	sort.Strings(row.Roles)

	row.Cases = tool.Str_to_int(values["cases"])
	row.Closed = tool.Str_to_int(values["closed"])
	row.Actions = tool.Str_to_int(values["actions"])
	row.Appeals = tool.Str_to_int(values["appeals"])
	row.AppealAccept = tool.Str_to_int(values["appeal_accept"])
	row.Mass = tool.Str_to_int(values["mass"])
	row.RoleChange = tool.Str_to_int(values["role_change"])
	row.Coi = tool.Str_to_int(values["coi"])
	row.LastAction = values["last_action"]

	row.AvgHours = -1
	if values["avg_hours"] != "" {
		row.AvgHours = tool.Str_to_int(values["avg_hours"])
		if values["avg_hours"] == "-1" {
			row.AvgHours = -1
		}
	}

	row.IdleDays = -1
	if values["idle_days"] != "" && values["idle_days"] != "-1" {
		row.IdleDays = tool.Str_to_int(values["idle_days"])
	}

	return row
}

// baro_gov_activity_period : 기간 값을 YYYY-MM 형태로 다듬는다 (어긋나면 이번 달)
func baro_gov_activity_period(period string) string {
	period = strings.TrimSpace(period)

	if len(period) == 7 && period[4] == '-' {
		year := tool.Str_to_int(period[:4])
		month := tool.Str_to_int(period[5:])

		if year >= 2000 && year <= 2200 && month >= 1 && month <= 12 {
			return tool.Baro_itoa(year) + "-" + baro_gov_two(month)
		}
	}

	return tool.Baro_today()[:7]
}

// baro_gov_activity_cache_date : 이 기간 캐시가 마지막으로 갱신된 시각
func baro_gov_activity_cache_date(db *sql.DB, period string) string {
	date := ""
	tool.QueryRow_DB(db, "select date from baro_admin_stat where name = 'activity' and period = ? order by date desc limit 1", []any{&date}, period)

	return date
}

// baro_gov_activity_load : 캐시에서 지표 읽기
func baro_gov_activity_load(db *sql.DB, period string) []baro_gov_activity_row {
	rows := tool.Query_DB(db, "select user, data from baro_admin_stat where name = 'activity' and period = ?", period)
	defer rows.Close()

	out := []baro_gov_activity_row{}
	for rows.Next() {
		user, data := "", ""
		if err := rows.Scan(&user, &data); err != nil {
			continue
		}

		out = append(out, baro_gov_activity_decode(db, user, data))
	}

	sort.Slice(out, func(i int, j int) bool { return out[i].User < out[j].User })

	return out
}

// baro_gov_activity_save : 계산 결과를 baro_admin_stat 에 캐시한다
//
//	같은 기간의 옛 캐시는 통째로 지우고 다시 넣는다.
//	그러지 않으면 역할이 회수된 사람의 낡은 줄이 계속 남아 화면에 되살아난다.
func baro_gov_activity_save(db *sql.DB, period string, list []baro_gov_activity_row) {
	now := tool.Baro_now()

	tool.Exec_DB(db, "delete from baro_admin_stat where name = 'activity' and period = ?", period)

	for _, row := range list {
		tool.Exec_DB(
			db,
			"insert into baro_admin_stat (user, name, data, period, date) values (?, 'activity', ?, ?, ?)",
			row.User, baro_gov_activity_encode(row), period, now,
		)
	}
}

// baro_gov_activity_cache_ok : 캐시가 지금의 담당자 목록을 빠짐없이 담고 있는가
//
//	캐시를 만든 뒤에 역할을 새로 받은 사람이 있으면 그 사람이 화면에서 통째로 빠진다.
//	담당자가 조용히 사라지는 것은 책임성 지표에서 가장 위험한 오류라, 이럴 때는 캐시를 버리고 다시 계산한다.
func baro_gov_activity_cache_ok(cached []baro_gov_activity_row, targets []string) bool {
	if len(cached) != len(targets) {
		return false
	}

	have := map[string]bool{}
	for _, row := range cached {
		have[row.User] = true
	}

	for _, user := range targets {
		if !have[user] {
			return false
		}
	}

	return true
}

// baro_gov_activity_rate_html : 이의제기 인용률 표시
func baro_gov_activity_rate_html(row baro_gov_activity_row) string {
	if row.Appeals == 0 {
		return tool.Baro_t(`<span class="bw_muted">이의제기 없음</span>`)
	}

	text := tool.Baro_pct_text(row.AppealAccept, row.Appeals)
	kind := "ok"

	pct := tool.Baro_pct(row.AppealAccept, row.Appeals)
	switch {
	case pct >= 50:
		kind = "danger"
	case pct >= 25:
		kind = "warn"
	}

	return tool.Baro_badge(text, kind) + `<div class="bw_note">` + tool.Baro_itoa(row.Appeals) + tool.Baro_t(`건 중 `) + tool.Baro_itoa(row.AppealAccept) + tool.Baro_t(`건 인용</div>`)
}

// baro_gov_activity_idle_html : 마지막 조치 표시
func baro_gov_activity_idle_html(row baro_gov_activity_row) string {
	if row.LastAction == "" {
		return tool.Baro_badge(tool.Baro_t("조치 기록 없음"), "muted")
	}

	html := tool.Baro_time_cell(row.LastAction)
	if row.IdleDays >= baro_gov_activity_idle_days {
		html += " " + tool.Baro_badge(tool.Baro_t("장기 미활동"), "warn")
	}

	return html
}

// View_baro_gov_activity : 관리자 활동 기록
func View_baro_gov_activity(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	period := baro_gov_q(query, "period")

	force := false
	message := ""

	if values != nil {
		// 다시 계산 단추는 지금 보고 있는 기간을 그대로 이어받는다.
		if v := strings.TrimSpace(values.Get("period")); v != "" {
			period = v
		}

		if strings.TrimSpace(values.Get("action")) == "recalc" {
			if !baro_gov_is_staff(db, config.IP) {
				return tool.Baro_deny(db, config, tool.Baro_t("지표를 다시 계산하려면 운영 역할이 필요합니다."))
			}

			if tool.Baro_read_only(db) {
				message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 다시 계산할 수 없습니다."))
			} else {
				force = true
			}
		}
	}

	period = baro_gov_activity_period(period)

	cache_date := baro_gov_activity_cache_date(db, period)
	stale := cache_date == "" || tool.Baro_days_between(cache_date, tool.Baro_now()) > 0 ||
		baro_gov_hours_between(cache_date, tool.Baro_now()) >= baro_gov_activity_cache_hours

	targets := baro_gov_activity_targets(db)

	list := []baro_gov_activity_row{}
	from_cache := false

	if !force && !stale {
		list = baro_gov_activity_load(db, period)
		from_cache = baro_gov_activity_cache_ok(list, targets)

		if !from_cache {
			list = []baro_gov_activity_row{}
		}
	}

	if !from_cache {
		for _, user := range targets {
			list = append(list, baro_gov_activity_calc(db, user))
		}

		if !tool.Baro_read_only(db) {
			baro_gov_activity_save(db, period, list)
			cache_date = tool.Baro_now()
		}
	}

	sort.Slice(list, func(i int, j int) bool { return list[i].User < list[j].User })

	body := baro_gov_css()
	body += tool.Baro_hero(tool.Baro_t("관리자 활동 기록"), tool.Baro_t("누가 얼마나 일했는지가 아니라, 운영이 제대로 굴러가는지를 보기 위한 표입니다."), "")
	body += baro_gov_tabs("/baro/role/activity")
	body += message

	body += tool.Baro_alert("warn",
		tool.Baro_t(`<b>이 표는 관리자 점수 놀이가 아닙니다. 책임성 지표로만 씁니다.</b>`)+
			tool.Baro_t(` 그래서 순위를 매기지 않고 가나다 순으로만 늘어놓습니다.`)+
			tool.Baro_t(` 처리 건수가 많다고 좋은 운영이 아니고, 적다고 나쁜 운영도 아닙니다.`)+
			tool.Baro_t(` 이의제기 인용률이 높다면 그 담당자를 벌하기 위한 근거가 아니라, 판단 기준을 다시 살펴야 한다는 신호로 읽습니다.`),
	)

	// 전체 요약
	total_cases, total_closed, total_appeal, total_accept, total_mass := 0, 0, 0, 0, 0
	idle_rows := [][]string{}

	for _, row := range list {
		total_cases += row.Cases
		total_closed += row.Closed
		total_appeal += row.Appeals
		total_accept += row.AppealAccept
		total_mass += row.Mass

		if len(row.Roles) == 0 {
			continue
		}

		if row.LastAction == "" || row.IdleDays >= baro_gov_activity_idle_days {
			idle_text := tool.Baro_t("조치 기록이 없습니다.")
			if row.LastAction != "" {
				idle_text = tool.Baro_itoa(row.IdleDays) + tool.Baro_t("일째 조치가 없습니다.")
			}

			role_names := []string{}
			for _, role := range row.Roles {
				role_names = append(role_names, tool.Baro_role_name(role))
			}

			idle_rows = append(idle_rows, []string{
				tool.Baro_user_link(db, row.User, config.IP),
				tool.HTML_escape(strings.Join(role_names, ", ")),
				baro_gov_activity_idle_html(row),
				tool.HTML_escape(idle_text),
			})
		}
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("살핀 담당자"), tool.Baro_itoa(len(list)), tool.Baro_t("역할 보유자 + 조치 기록자")},
		{tool.Baro_t("담당 사건"), tool.Baro_comma(total_cases), tool.Baro_t("종결 ") + tool.Baro_comma(total_closed) + tool.Baro_t("건")},
		{tool.Baro_t("이의제기"), tool.Baro_comma(total_appeal), "인용 " + tool.Baro_comma(total_accept) + tool.Baro_t("건")},
		{tool.Baro_t("대량 조치"), tool.Baro_comma(total_mass), tool.Baro_t("승인 절차를 거친 작업")},
		{tool.Baro_t("장기 미활동"), tool.Baro_itoa(len(idle_rows)), tool.Baro_itoa(baro_gov_activity_idle_days) + tool.Baro_t("일 이상")},
	})

	// 본 표 (가나다 순으로 자른다 — 순위가 아니다)
	page := baro_gov_page_num(baro_gov_q(query, "page"))
	size := tool.Baro_page_size

	start := (page - 1) * size
	end := start + size

	view := []baro_gov_activity_row{}
	if start < len(list) {
		if end > len(list) {
			end = len(list)
		}

		view = list[start:end]
	}

	table_rows := [][]string{}
	for _, row := range view {
		role_html := tool.Baro_t(`<span class="bw_muted">역할 없음</span>`)
		if len(row.Roles) > 0 {
			badges := []string{}
			for _, role := range row.Roles {
				badges = append(badges, tool.Baro_badge(tool.Baro_role_name(role), "info"))
			}

			role_html = strings.Join(badges, " ")
		}

		table_rows = append(table_rows, []string{
			tool.Baro_user_link(db, row.User, config.IP),
			role_html,
			tool.Baro_comma(row.Cases) + tool.Baro_t(`<div class="bw_note">종결 `) + tool.Baro_itoa(row.Closed) + tool.Baro_t(`건</div>`),
			tool.HTML_escape(baro_gov_hour_text(row.AvgHours)),
			baro_gov_activity_rate_html(row),
			tool.Baro_comma(row.Actions),
			tool.Baro_comma(row.Mass),
			tool.Baro_comma(row.RoleChange),
			tool.Baro_comma(row.Coi),
			baro_gov_activity_idle_html(row),
		})
	}

	list_body := ""
	if len(table_rows) == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("아직 살필 운영 활동이 없습니다. 역할을 부여하거나 사건을 처리하면 여기에 쌓입니다."))
	} else {
		list_body = tool.Baro_table(
			[]string{"담당자", "역할", tool.Baro_t("담당 사건"), tool.Baro_t("평균 처리 시간"), tool.Baro_t("이의제기 인용률"), tool.Baro_t("조치 수"), tool.Baro_t("대량 조치"), tool.Baro_t("권한 변경"), tool.Baro_t("회피 선언"), tool.Baro_t("마지막 조치")},
			table_rows,
			"",
		)
	}

	list_body += tool.Baro_paging(page, len(view), size, baro_gov_url("/baro/role/activity", [][]string{
		{"period", period}, {"page", "{}"},
	}))

	recalc := tool.Baro_form("/baro/role/activity",
		tool.Baro_hidden("action", "recalc")+
			tool.Baro_hidden("period", period)+
			tool.Baro_submit(baro_gov_period_name(period)+tool.Baro_t(" 지금 다시 계산"), ""),
	)

	cache_note := tool.Baro_t("계산 결과는 baro_admin_stat 에 ") + baro_gov_period_name(period) + tool.Baro_t(" 기간으로 저장됩니다.")
	if cache_date != "" {
		cache_note += tool.Baro_t(" 마지막 계산 : ") + cache_date
		if from_cache {
			cache_note += tool.Baro_t(" (저장된 값을 보여 주는 중)")
		}
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("담당자별 활동"),
		tool.Baro_t("가나다 순입니다. 순위가 아닙니다."),
		list_body+tool.Baro_note(cache_note),
		recalc,
	)

	// 장기 미활동
	idle_body := ""
	if len(idle_rows) == 0 {
		idle_body = tool.Baro_alert("ok", tool.Baro_t("역할을 가진 담당자 모두 최근 ")+tool.Baro_itoa(baro_gov_activity_idle_days)+tool.Baro_t("일 안에 조치 기록이 있습니다."))
	} else {
		idle_body = tool.Baro_alert("warn",
			tool.Baro_t("아래 담당자는 ")+tool.Baro_itoa(baro_gov_activity_idle_days)+tool.Baro_t("일 이상 조치 기록이 없습니다. 벌하기 위한 목록이 아니라, ")+
				tool.Baro_t("<b>권한이 잠자고 있지 않은지</b> 확인하기 위한 목록입니다. 계속 쉴 예정이라면 역할을 잠시 내려놓는 편이 안전합니다."),
		)
		idle_body += tool.Baro_table([]string{"담당자", "역할", tool.Baro_t("마지막 조치"), "상태"}, idle_rows, "")
	}

	body += tool.Baro_card_ex(tool.Baro_t("장기 미활동"), tool.Baro_itoa(baro_gov_activity_idle_days)+tool.Baro_t("일 이상 조치가 없는 담당자"), idle_body, "")

	body += tool.Baro_card(tool.Baro_t("지표를 어떻게 읽나요"), tool.Baro_dl([][]string{
		{tool.Baro_t("담당 사건"), tool.Baro_t("그 사람이 담당자로 지정된 baro_case 의 수입니다. 많다고 좋은 것이 아닙니다.")},
		{tool.Baro_t("평균 처리 시간"), tool.Baro_t("사건이 열린 시각부터 종결된 시각까지의 평균입니다. 종결된 사건만 셉니다. 너무 짧으면 대충 처리했을 수 있고, 너무 길면 당사자가 오래 기다린 것입니다.")},
		{tool.Baro_t("이의제기 인용률"), tool.Baro_t("그 사람이 담당한 사건에 들어온 이의제기 중 인용된 비율입니다. 높다면 판단 기준을 다시 살펴야 한다는 뜻입니다.")},
		{tool.Baro_t("대량 조치"), tool.Baro_t("10개 이상에 영향을 주는 작업 수입니다. 2인 승인과 스냅샷을 거쳤는지가 중요합니다.")},
		{tool.Baro_t("권한 변경"), tool.Baro_t("다른 사람에게 역할을 주거나 거둔 횟수입니다.")},
		{tool.Baro_t("회피 선언"), tool.Baro_t("이해충돌로 스스로 빠진 횟수입니다. 많을수록 신중하다는 뜻이지 게으르다는 뜻이 아닙니다.")},
	}))

	period_form := `<div class="bw_gov_filter">`
	period_form += tool.Baro_input("period", "기간", period, "2026-08", tool.Baro_t("캐시를 저장할 기간입니다."))
	period_form += `<div class="bw_gov_filter_btn">` + tool.Baro_submit(tool.Baro_t("이 기간으로 보기"), "") + `</div>`
	period_form += `</div>`

	body += tool.Baro_card(tool.Baro_t("기간 고르기"), baro_gov_get_form("/baro/role/activity", period_form))

	return tool.Baro_page(db, config, tool.Baro_t("관리자 활동 기록"), body, baro_gov_menu())
}
