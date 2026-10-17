package tool

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ============================================================
//  프로필 사진
//
//  위키 파일 문서로 올리지 않는다. 사람 얼굴은 문서가 아니고,
//  문서로 만들면 역사·토론·분류가 달라붙어 지우기가 어려워진다.
//  그래서 파일 보관함 안의 baro_avatar 칸에 따로 두고,
//  주인이 언제든 한 번에 지울 수 있게 한다.
//
//  올린 사진이 있으면 그라바타 자리를 대신한다. (없으면 그라바타 그대로)
// ============================================================

// Baro_avatar_max_bytes : 프로필 사진 한 장의 최대 크기
const Baro_avatar_max_bytes = 2 * 1000 * 1000

// Baro_avatar_ext_list : 받아 주는 확장자
//
//	svg 는 그림이면서 문서라 스크립트를 품을 수 있으므로 프로필 사진으로는 받지 않는다.
func Baro_avatar_ext_list() []string {
	return []string{"png", "jpg", "jpeg", "gif", "webp"}
}

// baro_avatar_ext_ok : 확장자가 목록에 있는가
func baro_avatar_ext_ok(ext string) bool {
	for _, one := range Baro_avatar_ext_list() {
		if one == ext {
			return true
		}
	}

	return false
}

// baro_avatar_sniff : 실제 내용이 그림인지 앞머리로 확인하고 참 확장자를 돌려준다.
//
//	확장자만 믿으면 이름만 바꾼 아무 파일이나 들어온다.
func baro_avatar_sniff(raw []byte) string {
	switch {
	case len(raw) > 8 && bytes.HasPrefix(raw, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "png"
	case len(raw) > 3 && bytes.HasPrefix(raw, []byte{0xFF, 0xD8, 0xFF}):
		return "jpg"
	case len(raw) > 6 && (bytes.HasPrefix(raw, []byte("GIF87a")) || bytes.HasPrefix(raw, []byte("GIF89a"))):
		return "gif"
	case len(raw) > 12 && bytes.HasPrefix(raw, []byte("RIFF")) && bytes.Equal(raw[8:12], []byte("WEBP")):
		return "webp"
	}

	return ""
}

// baro_avatar_dir : 프로필 사진 보관 칸
func baro_avatar_dir(db *sql.DB) string {
	return filepath.Join(Get_file_main_dir(db), "baro_avatar")
}

// baro_avatar_file : 이 사람의 사진 파일 이름
//
//	계정 이름을 그대로 파일 이름에 쓰면 경로 문자·대소문자 때문에 깨지므로 해시를 쓴다.
func baro_avatar_file(user string, ext string) string {
	return Sha224(strings.ToLower(strings.TrimSpace(user))) + "." + ext
}

// baro_avatar_set : baro_user_set 의 avatar 칸 ("<확장자>|<판>")
func baro_avatar_set(db *sql.DB, user string) (string, string) {
	data := ""
	QueryRow_DB(db, "select data from baro_user_set where user = ? and name = 'avatar'", []any{&data}, user)

	if data == "" {
		return "", ""
	}

	part := strings.SplitN(data, "|", 2)
	ext := strings.TrimSpace(part[0])
	if !baro_avatar_ext_ok(ext) {
		return "", ""
	}

	version := ""
	if len(part) > 1 {
		version = part[1]
	}

	return ext, version
}

// Baro_avatar_has : 올린 프로필 사진이 있는가
func Baro_avatar_has(db *sql.DB, user string) bool {
	if !Baro_is_user(user) {
		return false
	}

	ext, _ := baro_avatar_set(db, user)

	return ext != ""
}

// Baro_avatar_path : 실제 파일 경로 ("" 면 없음)
func Baro_avatar_path(db *sql.DB, user string) string {
	ext, _ := baro_avatar_set(db, user)
	if ext == "" {
		return ""
	}

	path := filepath.Join(baro_avatar_dir(db), baro_avatar_file(user, ext))
	if _, err := os.Stat(path); err != nil {
		return ""
	}

	return path
}

// Baro_avatar_url : 프로필 사진 주소 ("" 면 올린 사진이 없음)
//
//	판 번호를 붙여 사진을 바꾼 즉시 새 그림이 보이게 한다.
func Baro_avatar_url(db *sql.DB, user string) string {
	if !Baro_is_user(user) {
		return ""
	}

	ext, version := baro_avatar_set(db, user)
	if ext == "" {
		return ""
	}

	url := "/baro/avatar/file/" + Url_parser(user)
	if version != "" {
		url += "?v=" + Url_parser(version)
	}

	return url
}

// Baro_avatar_save : 프로필 사진을 올린다.
//
//	같은 사람이 다시 올리면 앞의 사진을 지우고 갈아 끼운다.
func Baro_avatar_save(db *sql.DB, user string, raw []byte) error {
	if !Baro_is_user(user) {
		return errors.New("로그인한 사용자만 올릴 수 있습니다.")
	}
	if len(raw) == 0 {
		return errors.New("빈 파일입니다.")
	}
	if len(raw) > Baro_avatar_max_bytes {
		return errors.New("사진이 너무 큽니다. " + strconv.Itoa(Baro_avatar_max_bytes/1000/1000) + "MB 이하로 올려 주세요.")
	}

	ext := baro_avatar_sniff(raw)
	if ext == "" {
		return errors.New("그림 파일이 아닙니다. png · jpg · gif · webp 만 올릴 수 있습니다.")
	}

	dir := baro_avatar_dir(db)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return errors.New("보관 칸을 만들지 못했습니다.")
	}

	// 확장자가 달라진 경우를 대비해 앞서 올린 것부터 치운다.
	baro_avatar_remove_files(db, user)

	if err := os.WriteFile(filepath.Join(dir, baro_avatar_file(user, ext)), raw, 0o644); err != nil {
		return errors.New("사진을 저장하지 못했습니다.")
	}

	Exec_DB(db, "delete from baro_user_set where user = ? and name = 'avatar'", user)
	Exec_DB(
		db,
		"insert into baro_user_set (user, name, data) values (?, 'avatar', ?)",
		user, ext+"|"+strconv.FormatInt(Baro_parse_time(Baro_now()).Unix(), 10),
	)

	return nil
}

// baro_avatar_remove_files : 이 사람 몫의 사진 파일을 모두 지운다
func baro_avatar_remove_files(db *sql.DB, user string) {
	dir := baro_avatar_dir(db)

	for _, ext := range Baro_avatar_ext_list() {
		_ = os.Remove(filepath.Join(dir, baro_avatar_file(user, ext)))
	}
}

// Baro_avatar_delete : 프로필 사진을 지운다 (그라바타로 돌아간다)
func Baro_avatar_delete(db *sql.DB, user string) {
	if !Baro_is_user(user) {
		return
	}

	baro_avatar_remove_files(db, user)
	Exec_DB(db, "delete from baro_user_set where user = ? and name = 'avatar'", user)
}

// ------------------------------------------------------------
//  그라바타 자리를 대신하기
// ------------------------------------------------------------

// Baro_user_avatar_url : 이 사람의 얼굴 그림 주소
//
//	올린 프로필 사진이 있으면 그것을, 없으면 그라바타를 돌려준다.
func Baro_user_avatar_url(db *sql.DB, user string, size int) string {
	if url := Baro_avatar_url(db, user); url != "" {
		return url
	}

	return Get_user_gravatar_url(db, user, size)
}

// Baro_user_avatar_ui : 얼굴 그림 <img> (없으면 "")
func Baro_user_avatar_ui(db *sql.DB, user string, size int) string {
	url := Baro_avatar_url(db, user)
	if url == "" {
		return Get_user_gravatar_ui(db, user, size)
	}

	size_text := strconv.Itoa(size)

	return `<img class="namu_gravatar bw_avatar_img" width="` + size_text + `" height="` + size_text +
		`" loading="lazy" alt="" src="` + HTML_escape(url) + `">`
}
