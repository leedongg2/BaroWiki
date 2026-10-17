package route

import (
	"database/sql"
	"net"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  아이피사용자 문서 ↔ 실제 아이피 사용자 연동
//
//  `ipuser:1.2.3.4` 문서는 그냥 빈 문서가 아니라
//  그 아이피로 실제 활동한 기록을 그대로 비춰 주는 창이어야 한다.
//
//  이 화면이 보여 주는 것
//    - 편집 · 토론 · 게시판 활동 수와 처음 · 마지막 활동 시각
//    - 지금 차단되어 있는지
//    - 이 아이피가 속한 대역(/24 · /16)과 그 대역에서 함께 활동한 다른 아이피
//    - 최근 기여 열 줄
//    - 기여 기록 · 사용자 도구 · 차단 화면으로 가는 링크
//
//  `ipuser:1.2.3.0/24` 처럼 대역을 적으면 그 대역에서 활동한 아이피를 모아 보여 준다.
// ============================================================

// ipuser_stat : 아이피 하나의 활동 요약
type ipuser_stat struct {
	Edit   int
	Topic  int
	Bbs    int
	First  string
	Last   string
	Titles int
}

// ipuser_gather_stat : 활동 요약 모으기
func ipuser_gather_stat(db *sql.DB, ip_name string) ipuser_stat {
	stat := ipuser_stat{}

	tool.QueryRow_DB(db, "select count(*) from history where ip = ?", []any{&stat.Edit}, ip_name)
	tool.QueryRow_DB(db, "select count(distinct title) from history where ip = ?", []any{&stat.Titles}, ip_name)
	tool.QueryRow_DB(db, "select count(*) from topic where ip = ?", []any{&stat.Topic}, ip_name)
	tool.QueryRow_DB(db, "select count(*) from bbs_data where set_name = 'user_id' and set_data = ?", []any{&stat.Bbs}, ip_name)
	tool.QueryRow_DB(db, "select date from history where ip = ? order by date asc limit 1", []any{&stat.First}, ip_name)
	tool.QueryRow_DB(db, "select date from history where ip = ? order by date desc limit 1", []any{&stat.Last}, ip_name)

	return stat
}

// ipuser_band : 아이피가 속한 대역 (IPv4 는 /24 와 /16, IPv6 는 /64)
func ipuser_band(ip_name string) []string {
	parsed := net.ParseIP(ip_name)
	if parsed == nil {
		return nil
	}

	if four := parsed.To4(); four != nil {
		return []string{
			strconv.Itoa(int(four[0])) + "." + strconv.Itoa(int(four[1])) + "." + strconv.Itoa(int(four[2])) + ".0/24",
			strconv.Itoa(int(four[0])) + "." + strconv.Itoa(int(four[1])) + ".0.0/16",
		}
	}

	masked := parsed.Mask(net.CIDRMask(64, 128))
	if masked == nil {
		return nil
	}

	return []string{masked.String() + "/64"}
}

// ipuser_band_prefix : 대역 표기에서 앞부분 문자열을 뽑는다 (like 검색용)
//
//	1.2.3.0/24 → "1.2.3."   /   1.2.0.0/16 → "1.2."
func ipuser_band_prefix(band string) string {
	address, network, err := net.ParseCIDR(band)
	if err != nil {
		return ""
	}
	_ = address

	ones, _ := network.Mask.Size()
	four := network.IP.To4()
	if four == nil {
		// IPv6 는 콜론 단위로만 자른다
		text := network.IP.String()
		if index := strings.LastIndex(text, "::"); index > 0 {
			return text[:index]
		}

		return text
	}

	switch {
	case ones >= 24:
		return strconv.Itoa(int(four[0])) + "." + strconv.Itoa(int(four[1])) + "." + strconv.Itoa(int(four[2])) + "."
	case ones >= 16:
		return strconv.Itoa(int(four[0])) + "." + strconv.Itoa(int(four[1])) + "."
	case ones >= 8:
		return strconv.Itoa(int(four[0])) + "."
	}

	return ""
}

// ipuser_band_members : 같은 대역에서 활동한 아이피 목록 (편집 수가 많은 차례)
func ipuser_band_members(db *sql.DB, prefix string, exclude string, limit int) [][2]string {
	if prefix == "" {
		return nil
	}

	rows := tool.Query_DB(
		db,
		"select ip, count(*) as num from history where ip like ? group by ip order by num desc limit ?",
		prefix+"%", limit+8,
	)
	defer rows.Close()

	out := [][2]string{}
	for rows.Next() {
		ip_name, number := "", ""
		if rows.Scan(&ip_name, &number) != nil {
			continue
		}
		if ip_name == exclude || !tool.IP_or_user(ip_name) {
			continue
		}
		// like 는 1.2.3. 로 시작하는 다른 대역(1.2.3X)도 잡을 수 있어 한 번 더 거른다
		if !strings.HasPrefix(ip_name, prefix) {
			continue
		}

		out = append(out, [2]string{ip_name, number})
		if len(out) >= limit {
			break
		}
	}

	return out
}

// ipuser_info_row : 이름-값 한 줄
func ipuser_info_row(name string, value string) string {
	return `<tr><th class="namu_ipuser_key">` + name + `</th><td>` + value + `</td></tr>`
}

// view_w_ipuser_data : 아이피사용자 문서 위에 붙는 정보 상자
func view_w_ipuser_data(db *sql.DB, config tool.Config, doc_name string) string {
	ip_name := strings.TrimPrefix(doc_name, "ipuser:")
	if ip_name == "" {
		return ""
	}

	// 대역 문서(`ipuser:1.2.3.0/24`)는 슬래시를 그대로 둔다.
	// 그 밖의 슬래시는 하위 문서 이름이므로 떼어 낸다.
	if _, _, err := net.ParseCIDR(ip_name); err == nil {
		return view_w_ipuser_band_data(db, config, ip_name)
	}
	if slash_index := strings.Index(ip_name, "/"); slash_index >= 0 {
		ip_name = ip_name[:slash_index]
	}

	if !tool.IP_or_user(ip_name) {
		return ""
	}

	stat := ipuser_gather_stat(db, ip_name)
	ban := tool.Get_user_ban(db, ip_name, "")

	info := ipuser_info_row(
		tool.Get_language(db, "user", true),
		`<span id="opennamu_get_user_info">`+tool.HTML_escape(ip_name)+`</span>`,
	)

	state_html := `<span class="namu_ipuser_ok">` + tool.Lang_esc("normal") + `</span>`
	if ban[0] == "true" {
		state_html = `<span class="namu_ipuser_ban">` + tool.Lang_esc("blocked") + ` ` + tool.HTML_escape(ban[1]) + `</span>`
	}
	info += ipuser_info_row(tool.Get_language(db, "state", true), state_html)

	info += ipuser_info_row(
		tool.Get_language(db, "edit_record", true),
		tool.HTML_escape(strconv.Itoa(stat.Edit))+" ("+tool.Get_language(db, "document", true)+" "+
			tool.HTML_escape(strconv.Itoa(stat.Titles))+")",
	)
	info += ipuser_info_row(tool.Get_language(db, "discussion_record", true), tool.HTML_escape(strconv.Itoa(stat.Topic)))
	info += ipuser_info_row(tool.Lang_esc("bbs"), tool.HTML_escape(strconv.Itoa(stat.Bbs)))

	if stat.First != "" {
		info += ipuser_info_row(tool.Lang_esc("first_edit"), tool.HTML_escape(stat.First))
	}
	if stat.Last != "" {
		info += ipuser_info_row(tool.Lang_esc("last_edit_time"), tool.HTML_escape(stat.Last))
	}

	band_list := ipuser_band(ip_name)
	if len(band_list) > 0 {
		band_html := ""
		for _, band := range band_list {
			band_html += `<a href="/w/` + tool.Url_parser("ipuser:"+band) + `">` + tool.HTML_escape(band) + `</a> `
		}
		info += ipuser_info_row(tool.Lang_esc("ip_band"), strings.TrimSpace(band_html))
	}

	data_html := `<div class="namu_ipuser_box"><table class="namu_table namu_ipuser_table"><tbody>` + info + `</tbody></table></div>`

	// 최근 기여
	rows := tool.Query_DB(
		db,
		"select title, date, send from history where ip = ? order by date desc limit 10",
		ip_name,
	)
	// 읽을 수 없는 문서는 기여 목록에서도 뺀다
	ipuser_filter := tool.New_acl_render_filter(db, config.IP)
	list_html := ""
	for rows.Next() {
		title, date, send := "", "", ""
		if rows.Scan(&title, &date, &send) != nil {
			continue
		}
		if !ipuser_filter.Allow(title) {
			continue
		}

		list_html += `<tr><td><a href="/w/` + tool.Url_parser(title) + `">` + tool.HTML_escape(tool.Namespace_display(title)) + `</a></td><td>` + tool.HTML_escape(date) + `</td><td>` + tool.HTML_escape(send) + `</td></tr>`
	}
	rows.Close()

	if list_html == "" {
		list_html = `<tr><td colspan="3" class="namu_table_empty">` + tool.Get_language(db, "contribution_empty", true) + `</td></tr>`
	}

	data_html += `<h2>` + tool.Get_language(db, "recent_contribution", true) + `</h2><div class="namu_table_wrap"><table class="namu_table"><thead><tr><th>` +
		tool.Get_language(db, "document", true) + `</th><th>` + tool.Get_language(db, "time", true) + `</th><th>` +
		tool.Get_language(db, "summary", true) + `</th></tr></thead><tbody>` + list_html + `</tbody></table></div>`

	// 같은 대역에서 활동한 다른 아이피
	if len(band_list) > 0 {
		members := ipuser_band_members(db, ipuser_band_prefix(band_list[0]), ip_name, 12)
		if len(members) > 0 {
			member_html := ""
			for _, one := range members {
				member_html += `<a class="namu_button_small" href="/w/` + tool.Url_parser("ipuser:"+one[0]) + `">` +
					tool.HTML_escape(one[0]) + ` <span class="namu_ipuser_count">` + tool.HTML_escape(one[1]) + `</span></a> `
			}

			data_html += `<h2>` + tool.Lang_esc("same_band_ip") + `</h2><div class="namu_ipuser_menu">` + member_html + `</div>`
		}
	}

	data_html += `<div class="namu_ipuser_menu">` +
		`<a class="namu_button_small" href="/baro/record/user/` + tool.Url_parser(ip_name) + `/all">` + tool.Get_language(db, "contribution", true) + `</a> ` +
		`<a class="namu_button_small" href="/record/` + tool.Url_parser(ip_name) + `">` + tool.Get_language(db, "all_contribution_list", true) + `</a> ` +
		`<a class="namu_button_small" href="/user/` + tool.Url_parser(ip_name) + `">` + tool.Get_language(db, "user_tool", true) + `</a> ` +
		`<a class="namu_button_small" href="/baro/land/user/` + tool.Url_parser(ip_name) + `">` + tool.Lang_esc("contribution_land") + `</a>`

	if tool.Check_acl(db, "", "", "ban", config.IP) {
		data_html += ` <a class="namu_button_small" href="/auth/ban/` + tool.Url_parser(ip_name) + `">` + tool.Get_language(db, "ban", true) + `</a>`
	}

	data_html += `</div><hr class="main_hr">`

	return data_html
}

// view_w_ipuser_band_data : 대역 문서 (`ipuser:1.2.3.0/24`)
func view_w_ipuser_band_data(db *sql.DB, config tool.Config, band string) string {
	prefix := ipuser_band_prefix(band)
	members := ipuser_band_members(db, prefix, "", 200)

	total := 0
	list_html := ""
	for _, one := range members {
		total += tool.Str_to_int(one[1])

		ban := tool.Get_user_ban(db, one[0], "")
		state := ""
		if ban[0] == "true" {
			state = `<span class="namu_ipuser_ban">` + tool.Lang_esc("blocked") + `</span>`
		}

		last := ""
		tool.QueryRow_DB(db, "select date from history where ip = ? order by date desc limit 1", []any{&last}, one[0])

		list_html += `<tr><td><a href="/w/` + tool.Url_parser("ipuser:"+one[0]) + `">` + tool.HTML_escape(one[0]) + `</a></td>` +
			`<td>` + tool.HTML_escape(one[1]) + `</td><td>` + tool.HTML_escape(last) + `</td><td>` + state + `</td></tr>`
	}

	if list_html == "" {
		list_html = `<tr><td colspan="4" class="namu_table_empty">` + tool.Get_language(db, "contribution_empty", true) + `</td></tr>`
	}

	info := ipuser_info_row(tool.Lang_esc("ip_band"), tool.HTML_escape(band))
	info += ipuser_info_row(tool.Lang_esc("same_band_ip"), tool.HTML_escape(strconv.Itoa(len(members))))
	info += ipuser_info_row(tool.Get_language(db, "edit_record", true), tool.HTML_escape(strconv.Itoa(total)))

	data_html := `<div class="namu_ipuser_box"><table class="namu_table namu_ipuser_table"><tbody>` + info + `</tbody></table></div>`
	data_html += `<h2>` + tool.Lang_esc("same_band_ip") + `</h2><div class="namu_table_wrap"><table class="namu_table"><thead><tr><th>` +
		tool.Get_language(db, "user", true) + `</th><th>` + tool.Get_language(db, "edit_record", true) + `</th><th>` +
		tool.Lang_esc("last_edit_time") + `</th><th>` + tool.Get_language(db, "state", true) + `</th></tr></thead><tbody>` +
		list_html + `</tbody></table></div>`

	if tool.Check_acl(db, "", "", "ban", config.IP) {
		// 대역은 CIDR 전용 차단 화면으로 보낸다
		data_html += `<div class="namu_ipuser_menu"><a class="namu_button_small" href="/auth/ban_cidr/` + tool.Url_parser(band) + `">` +
			tool.Get_language(db, "band_ban", true) + `</a></div>`
	}

	return data_html + `<hr class="main_hr">`
}
