package tool

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"strconv"
	"strings"
)

// Get_gravatar_hash : 그라바타 해시 (이메일 기준, 없으면 사용자 이름 기준)
func Get_gravatar_hash(email string, user_name string) string {
	key := strings.ToLower(strings.TrimSpace(email))
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(user_name)) + "@opennamu.local"
	}

	hash_data := md5.Sum([]byte(key))

	return hex.EncodeToString(hash_data[:])
}

// Get_gravatar_url : 그라바타 이미지 주소
func Get_gravatar_url(hash_data string, size int) string {
	if hash_data == "" {
		return ""
	}

	if size <= 0 {
		size = 80
	}

	return "https://secure.gravatar.com/avatar/" + hash_data + "?d=retro&s=" + strconv.Itoa(size)
}

// Get_user_set_data : 사용자 설정 값 조회
func Get_user_set_data(db *sql.DB, user_name string, set_name string) string {
	set_data := ""
	QueryRow_DB(
		db,
		"select data from user_set where id = ? and name = ? limit 1",
		[]any{&set_data},
		user_name,
		set_name,
	)

	return set_data
}

// Do_set_user_set_data : 사용자 설정 값 저장 (1개만 유지)
func Do_set_user_set_data(db *sql.DB, user_name string, set_name string, set_data string) {
	Exec_DB(db, "delete from user_set where id = ? and name = ?", user_name, set_name)

	if set_data != "" {
		Exec_DB(
			db,
			"insert into user_set (id, name, data) values (?, ?, ?)",
			user_name,
			set_name,
			set_data,
		)
	}
}

// Get_user_gravatar_url : 사용자 그라바타 주소 (없으면 생성)
func Get_user_gravatar_url(db *sql.DB, user_name string, size int) string {
	if user_name == "" || IP_or_user(user_name) {
		return ""
	}

	gravatar_off := ""
	QueryRow_DB(db, "select data from other where name = 'gravatar_off'", []any{&gravatar_off})
	if gravatar_off != "" {
		return ""
	}

	hash_data := Get_user_set_data(db, user_name, "gravatar")
	if hash_data == "" {
		hash_data = Get_gravatar_hash(Get_user_set_data(db, user_name, "email"), user_name)
		Do_set_user_set_data(db, user_name, "gravatar", hash_data)
	}

	return Get_gravatar_url(hash_data, size)
}

// Get_user_gravatar_ui : 사용자 아바타 UI
func Get_user_gravatar_ui(db *sql.DB, user_name string, size int) string {
	gravatar_url := Get_user_gravatar_url(db, user_name, size*2)
	if gravatar_url == "" {
		return ""
	}

	size_text := strconv.Itoa(size)

	return `<img class="namu_gravatar" width="` + size_text + `" height="` + size_text + `" loading="lazy" alt="" src="` + HTML_escape(gravatar_url) + `">`
}

// Do_user_register_info_save : 가입 시점 정보 (IP / 접속 정보 / 그라바타) 저장
func Do_user_register_info_save(db *sql.DB, config Config, user_name string, email string) {
	register_ip := config.IP
	if register_ip == "" || !IP_or_user(register_ip) {
		// 이미 로그인한 사용자가 계정을 만든 경우에도 실제 접속 IP 를 남길 수 없으므로 비워둔다
		register_ip = ""
	}

	register_date := Get_time()
	gravatar_hash := Get_gravatar_hash(email, user_name)

	Do_set_user_set_data(db, user_name, "register_ip", register_ip)
	Do_set_user_set_data(db, user_name, "register_agent", Get_slice(config.UserAgent, 0, 512))
	Do_set_user_set_data(db, user_name, "gravatar", gravatar_hash)

	user_json_data := map[string]string{
		"id":            user_name,
		"ip":            register_ip,
		"date":          register_date,
		"email":         email,
		"gravatar":      gravatar_hash,
		"gravatar_url":  Get_gravatar_url(gravatar_hash, 160),
		"user_agent":    Get_slice(config.UserAgent, 0, 512),
		"register_type": "web",
	}

	user_json_text, err := json.Marshal(user_json_data)
	if err != nil {
		return
	}

	Do_set_user_set_data(db, user_name, "user_json", string(user_json_text))
}

// Get_user_register_info : 가입 시점 정보 조회
func Get_user_register_info(db *sql.DB, user_name string) map[string]string {
	info := map[string]string{}

	user_json_text := Get_user_set_data(db, user_name, "user_json")
	if user_json_text != "" {
		if err := json.Unmarshal([]byte(user_json_text), &info); err == nil {
			return info
		}
	}

	info["id"] = user_name
	info["ip"] = Get_user_set_data(db, user_name, "register_ip")
	info["date"] = Get_user_set_data(db, user_name, "date")
	info["email"] = Get_user_set_data(db, user_name, "email")
	info["gravatar"] = Get_user_set_data(db, user_name, "gravatar")

	return info
}
