package tool

import (
	"database/sql"
	"strings"
	"time"
)

// Bbs_block_all : 모든 게시판을 뜻하는 게시판 번호
const Bbs_block_all = "all"

// Bbs_block_info : 게시판 차단 정보
type Bbs_block_info struct {
	Exist   bool
	Target  string
	BbsID   string
	End     string
	Why     string
	Blocker string
	Date    string
}

// bbs_block_row_parse : 저장된 차단 데이터 (만료||사유||차단자||시각) 분리
func bbs_block_row_parse(set_data string) (string, string, string, string) {
	parts := strings.Split(set_data, "||")
	for len(parts) < 4 {
		parts = append(parts, "")
	}

	return parts[0], parts[1], parts[2], parts[3]
}

// bbs_block_row_make : 차단 데이터 문자열 생성
func bbs_block_row_make(end string, why string, blocker string, date string) string {
	replacer := strings.NewReplacer("||", "//")

	return strings.Join([]string{
		replacer.Replace(end),
		replacer.Replace(why),
		replacer.Replace(blocker),
		replacer.Replace(date),
	}, "||")
}

// Do_bbs_block_insert : 게시판 차단 등록 (bbs_id 가 all 이면 전체 게시판)
func Do_bbs_block_insert(db *sql.DB, bbs_id string, user_name string, end string, why string, blocker string) {
	if bbs_id == "" {
		bbs_id = Bbs_block_all
	}

	Exec_DB(
		db,
		"delete from bbs_set where set_name = 'bbs_block' and set_id = ? and set_code = ?",
		bbs_id,
		user_name,
	)

	Exec_DB(
		db,
		"insert into bbs_set (set_name, set_code, set_id, set_data) values ('bbs_block', ?, ?, ?)",
		user_name,
		bbs_id,
		bbs_block_row_make(end, why, blocker, Get_time()),
	)
}

// Do_bbs_block_release : 게시판 차단 해제
func Do_bbs_block_release(db *sql.DB, bbs_id string, user_name string) {
	if bbs_id == "" {
		bbs_id = Bbs_block_all
	}

	Exec_DB(
		db,
		"delete from bbs_set where set_name = 'bbs_block' and set_id = ? and set_code = ?",
		bbs_id,
		user_name,
	)
}

// Get_bbs_block_info : 게시판 차단 정보 조회 (차단 중이 아니면 Exist = false)
func Get_bbs_block_info(db *sql.DB, bbs_id string, user_name string) Bbs_block_info {
	block_info := Bbs_block_info{}
	if user_name == "" {
		return block_info
	}

	check_list := []string{Bbs_block_all}
	if bbs_id != "" && bbs_id != Bbs_block_all {
		check_list = append(check_list, bbs_id)
	}

	for _, check_id := range check_list {
		set_data := ""
		if !QueryRow_DB(
			db,
			"select set_data from bbs_set where set_name = 'bbs_block' and set_id = ? and set_code = ? limit 1",
			[]any{&set_data},
			check_id,
			user_name,
		) {
			continue
		}

		end, why, blocker, date := bbs_block_row_parse(set_data)

		// 기간이 지난 차단은 자동 해제
		if end != "" {
			if end_time, ok := Ban_time_parse(end); ok {
				if !end_time.After(time.Now()) {
					Do_bbs_block_release(db, check_id, user_name)
					continue
				}
			}
		}

		block_info.Exist = true
		block_info.Target = user_name
		block_info.BbsID = check_id
		block_info.End = end
		block_info.Why = why
		block_info.Blocker = blocker
		block_info.Date = date

		return block_info
	}

	return block_info
}

// Get_bbs_blocked : 게시판 이용이 차단되었는지 확인 (게시판 관리자는 제외)
func Get_bbs_blocked(db *sql.DB, bbs_id string, user_name string) bool {
	if user_name == "" {
		return false
	}

	auth_info := Get_user_auth_info(db, user_name)
	if auth_info["bbs"] || auth_info["owner"] {
		return false
	}

	return Get_bbs_block_info(db, bbs_id, user_name).Exist
}

// Get_bbs_lock : 게시판 잠금 여부 (읽기 전용)
func Get_bbs_lock(db *sql.DB, bbs_id string) bool {
	if bbs_id == "" {
		return false
	}

	set_data := ""
	QueryRow_DB(
		db,
		"select set_data from bbs_set where set_name = 'bbs_lock' and set_id = ? limit 1",
		[]any{&set_data},
		bbs_id,
	)

	return set_data != ""
}

// Do_bbs_lock_set : 게시판 잠금 설정
func Do_bbs_lock_set(db *sql.DB, bbs_id string, lock bool) {
	Exec_DB(db, "delete from bbs_set where set_name = 'bbs_lock' and set_id = ?", bbs_id)

	if lock {
		Exec_DB(
			db,
			"insert into bbs_set (set_name, set_code, set_id, set_data) values ('bbs_lock', '', ?, '1')",
			bbs_id,
		)
	}
}

// Get_bbs_block_list : 게시판 차단 목록
func Get_bbs_block_list(db *sql.DB) [][]string {
	rows := Query_DB(
		db,
		"select set_id, set_code, set_data from bbs_set where set_name = 'bbs_block' order by set_id asc",
	)
	defer rows.Close()

	data_list := [][]string{}
	for rows.Next() {
		set_id := ""
		set_code := ""
		set_data := ""

		if rows.Scan(&set_id, &set_code, &set_data) != nil {
			continue
		}

		end, why, blocker, date := bbs_block_row_parse(set_data)
		data_list = append(data_list, []string{set_id, set_code, end, why, blocker, date})
	}

	return data_list
}

// Get_bbs_block_banner : 게시판 차단 안내 UI
func Get_bbs_block_banner(db *sql.DB, config Config, bbs_id string) string {
	block_info := Get_bbs_block_info(db, bbs_id, config.IP)
	if !block_info.Exist {
		return ""
	}

	end_text := Lang_esc("limitless")
	if block_info.End != "" {
		end_text = HTML_escape(block_info.End) + " (" + HTML_escape(Ban_duration_text(block_info.End)) + " " + Lang_esc("remain") + ")"
	}

	target_text := Lang_esc("all_bbs")
	if block_info.BbsID != Bbs_block_all {
		target_text = HTML_escape(Get_bbs_name(db, block_info.BbsID)) + " " + Lang_esc("bbs")
	}

	why := block_info.Why
	if why == "" {
		why = Lang("no_reason")
	}

	data_html := `<div class="namu_ban_box"><div class="namu_ban_title">` + Lang_esc("bbs_banned_notice") + `</div><ul class="namu_ban_list">`
	data_html += `<li>` + Lang_esc("target") + ` : ` + target_text + `</li>`
	data_html += `<li>` + Lang_esc("why") + ` : ` + HTML_escape(why) + `</li>`
	data_html += `<li>` + Lang_esc("expire") + ` : ` + end_text + `</li>`
	data_html += `</ul></div>`

	return data_html
}

// Get_bbs_name : 게시판 이름
func Get_bbs_name(db *sql.DB, bbs_id string) string {
	bbs_name := ""
	QueryRow_DB(
		db,
		"select set_data from bbs_set where set_name = 'bbs_name' and set_id = ? limit 1",
		[]any{&bbs_name},
		bbs_id,
	)

	if bbs_name == "" {
		return bbs_id
	}

	return bbs_name
}
