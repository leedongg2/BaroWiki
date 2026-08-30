package tool

import (
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// Ban_time_parse : DB에 저장된 차단 만료 시각 문자열을 시간으로 변환
func Ban_time_parse(end string) (time.Time, bool) {
	end = strings.TrimSpace(end)
	if end == "" || end == "0" || end == "release" {
		return time.Time{}, false
	}

	if end_time, err := time.ParseInLocation("2006-01-02 15:04:05", end, time.Local); err == nil {
		return end_time, true
	}

	if end_time, err := time.ParseInLocation("2006-01-02", end, time.Local); err == nil {
		return end_time, true
	}

	return time.Time{}, false
}

// Ban_expired : 차단 기간이 끝났는지 확인 (끝났으면 진행 상태 해제)
func Ban_expired(db *sql.DB, block string, band string, end string) bool {
	end_time, ok := Ban_time_parse(end)
	if !ok {
		return false
	}

	if time.Now().Before(end_time) {
		return false
	}

	Exec_DB(
		db,
		"update rb set ongoing = '' where block = ? and band = ? and end = ? and ongoing = '1'",
		block,
		band,
		end,
	)

	return true
}

// Ban_end_from_duration : 기간(초)으로 만료 시각 문자열 생성 (0 이하는 무기한)
func Ban_end_from_duration(second int) string {
	if second <= 0 {
		return ""
	}

	return time.Now().Add(time.Duration(second) * time.Second).Format("2006-01-02 15:04:05")
}

// Ban_duration_second : 년/개월/주/일/시/분/초 입력을 초로 변환
func Ban_duration_second(year string, month string, week string, day string, hour string, minute string, second string) int {
	total := 0
	total += Str_to_int(year) * 365 * 24 * 60 * 60
	total += Str_to_int(month) * 30 * 24 * 60 * 60
	total += Str_to_int(week) * 7 * 24 * 60 * 60
	total += Str_to_int(day) * 24 * 60 * 60
	total += Str_to_int(hour) * 60 * 60
	total += Str_to_int(minute) * 60
	total += Str_to_int(second)

	return total
}

// Ban_is_expired : 차단이 이미 끝났는지 (문구가 아니라 시각으로 판단한다)
//
//	표시 문구는 언어에 따라 달라지므로 절대 문구를 비교하지 않는다.
func Ban_is_expired(end string) bool {
	end_time, ok := Ban_time_parse(end)
	if !ok {
		return false
	}

	return time.Until(end_time) <= 0
}

// Ban_duration_text : 남은 기간을 사람이 읽을 수 있는 형태로 변환
func Ban_duration_text(end string) string {
	end_time, ok := Ban_time_parse(end)
	if !ok {
		return Lang("limitless")
	}

	remain := time.Until(end_time)
	if remain <= 0 {
		return Lang("expired")
	}

	total_second := int(remain.Seconds())
	day := total_second / (24 * 60 * 60)
	hour := (total_second % (24 * 60 * 60)) / (60 * 60)
	minute := (total_second % (60 * 60)) / 60
	second := total_second % 60

	text := ""
	if day > 0 {
		text += strconv.Itoa(day) + Lang("day") + " "
	}
	if hour > 0 {
		text += strconv.Itoa(hour) + Lang("hour") + " "
	}
	if minute > 0 {
		text += strconv.Itoa(minute) + Lang("minute") + " "
	}
	if second > 0 || text == "" {
		text += strconv.Itoa(second) + Lang("second")
	}

	return strings.TrimSpace(text)
}

// Ban_info : 현재 적용 중인 차단 정보
type Ban_info struct {
	Exist   bool
	Target  string
	End     string
	Why     string
	Blocker string
	Date    string
	Band    string
	Login   string
}

// Get_ban_info : 사용자(또는 IP)에게 적용 중인 차단 정보 조회
func Get_ban_info(db *sql.DB, ip string) Ban_info {
	ban_info := Ban_info{}

	ban_data := Get_user_ban(db, ip, "")
	if ban_data[0] != "true" {
		return ban_info
	}

	ban_info.Exist = true
	ban_info.Target = ip

	// 사용자 이름 / IP 직접 차단
	if QueryRow_DB(
		db,
		"select end, why, blocker, today, band, login from rb where block = ? and (band = '' or band = 'private') and ongoing = '1' order by today desc limit 1",
		[]any{&ban_info.End, &ban_info.Why, &ban_info.Blocker, &ban_info.Date, &ban_info.Band, &ban_info.Login},
		ip,
	) {
		return ban_info
	}

	// 그룹(acl = ban) 차단
	acl_data := ""
	if QueryRow_DB(db, "select data from user_set where id = ? and name = 'acl'", []any{&acl_data}, ip) {
		if acl_data == "ban" {
			ban_info.Why = Lang("ban_group")

			return ban_info
		}
	}

	// 정규식 / CIDR 차단
	ban_info.Why = Lang("ban_regex_or_range")

	return ban_info
}

// Do_warn_insert : 사용자에게 경고 발급
func Do_warn_insert(db *sql.DB, user_name string, why string, blocker string) {
	Exec_DB(
		db,
		"insert into rb (block, end, today, blocker, why, band, ongoing, login) values (?, '', ?, ?, ?, 'warn', '1', '')",
		user_name,
		Get_time(),
		blocker,
		why,
	)

	Send_alarm(db, blocker, user_name, Lang("warn_received")+" "+Lang("why")+" : "+why)
}

// Get_warn_list : 사용자의 경고 목록 (확인하지 않은 경고만 조회 가능)
func Get_warn_list(db *sql.DB, user_name string, only_new bool) [][]string {
	query := "select why, today, blocker, ongoing from rb where band = 'warn' and block = ? order by today desc limit 100"
	if only_new {
		query = "select why, today, blocker, ongoing from rb where band = 'warn' and block = ? and ongoing = '1' order by today desc limit 100"
	}

	rows := Query_DB(db, query, user_name)
	defer rows.Close()

	data_list := [][]string{}
	for rows.Next() {
		why := ""
		today := ""
		blocker := ""
		ongoing := ""

		if rows.Scan(&why, &today, &blocker, &ongoing) != nil {
			continue
		}

		data_list = append(data_list, []string{why, today, blocker, ongoing})
	}

	return data_list
}

// Do_warn_check : 사용자가 경고를 확인 처리
func Do_warn_check(db *sql.DB, user_name string) {
	Exec_DB(
		db,
		"update rb set ongoing = '' where band = 'warn' and block = ? and ongoing = '1'",
		user_name,
	)
}

// Get_warn_count : 확인하지 않은 경고 수
func Get_warn_count(db *sql.DB, user_name string) int {
	if user_name == "" || IP_or_user(user_name) {
		return 0
	}

	count := 0
	QueryRow_DB(
		db,
		"select count(*) from rb where band = 'warn' and block = ? and ongoing = '1'",
		[]any{&count},
		user_name,
	)

	return count
}

// Get_warning_banner : 미확인 경고 알림 UI
func Get_warning_banner(db *sql.DB, config Config) string {
	if config.IP == "" || IP_or_user(config.IP) {
		return ""
	}

	warn_list := Get_warn_list(db, config.IP, true)
	if len(warn_list) == 0 {
		return ""
	}

	data_html := `<div class="namu_warn_box"><div class="namu_warn_title">` + Lang_esc("warn_received") + `</div><ul class="namu_warn_list">`
	for _, warn_data := range warn_list {
		why := warn_data[0]
		if why == "" {
			why = Lang("no_reason")
		}

		data_html += `<li><b>` + HTML_escape(why) + `</b> <span class="namu_warn_date">` + HTML_escape(warn_data[1]) + `</span></li>`
	}
	data_html += `</ul><form method="post" action="/warn_check"><button class="namu_button" type="submit">` + Lang_esc("warn_confirm") + `</button></form></div>`

	return data_html
}

// Get_ban_banner : 차단된 사용자에게 보여줄 안내 UI
func Get_ban_banner(db *sql.DB, config Config) string {
	if config.IP == "" {
		return ""
	}

	ban_info := Get_ban_info(db, config.IP)
	if !ban_info.Exist {
		return ""
	}

	end_text := Lang_esc("limitless")
	if ban_info.End != "" {
		end_text = HTML_escape(ban_info.End) + " (" + HTML_escape(Ban_duration_text(ban_info.End)) + " " + Lang_esc("remain") + ")"
	}

	why := ban_info.Why
	if why == "" {
		why = Lang("no_reason")
	}

	data_html := `<div class="namu_ban_box"><div class="namu_ban_title">` + Lang_esc("banned_user_notice") + `</div><ul class="namu_ban_list">`
	data_html += `<li>` + Lang_esc("why") + ` : ` + HTML_escape(why) + `</li>`
	data_html += `<li>` + Lang_esc("expire") + ` : ` + end_text + `</li>`
	if ban_info.Blocker != "" {
		data_html += `<li>` + Lang_esc("blocker") + ` : ` + HTML_escape(ban_info.Blocker) + `</li>`
	}
	data_html += `</ul></div>`

	return data_html
}
