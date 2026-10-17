package tool

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dlclark/regexp2"
)

func Get_file_max_size(db *sql.DB) int {
	data := "0"

	QueryRow_DB(
		db,
		"select data from other where name = 'upload'",
		[]any{&data},
	)

	file_max_size := Str_to_int(data)

	return file_max_size
}

func Get_file_main_dir(db *sql.DB) string {
	data := ""

	QueryRow_DB(
		db,
		"select data from other where name = 'image_where'",
		[]any{&data},
	)

	if data == "" {
		data = filepath.Join("data", "images")
	} else {
		data = filepath.Clean(data)
	}

	return data
}

func Get_ext_allow_list(db *sql.DB) map[string]bool {
	rows := Query_DB(db, "select html from html_filter where kind = 'extension'")
	defer rows.Close()

	data_list := map[string]bool{}

	for rows.Next() {
		data := ""

		err := rows.Scan(&data)
		if err != nil {
			panic(err)
		}

		data = strings.ToLower(data)
		data = strings.TrimPrefix(data, ".")

		data_list[data] = true
	}

	return data_list
}

func Get_file_name_unallow_check(db *sql.DB, file_name string) bool {
	rows := Query_DB(db, "select html from html_filter where kind = 'file_name'")
	defer rows.Close()

	for rows.Next() {
		data := ""

		err := rows.Scan(&data)
		if err != nil {
			panic(err)
		}

		r, err := regexp2.Compile(data, 0)
		if err != nil {
			continue
		}

		m, err := r.MatchString(file_name)
		if err == nil && m {
			return true
		}
	}

	return false
}

func File_name_to_dir(file_name string, file_ext string) string {
	h := sha256.New224()
	io.WriteString(h, file_name)
	hash_hex := hex.EncodeToString(h.Sum(nil))

	return hash_hex + "." + file_ext
}

// Get_file_default_ext_list : 기본 허용 확장자 목록
func Get_file_default_ext_list() []string {
	return []string{"jpg", "jpeg", "png", "gif", "webp", "svg"}
}

// Svg_script_check : SVG 안에 스크립트가 들어 있는가
//
//	SVG 는 그림이면서 동시에 문서라서 <script> 와 onload= 를 품을 수 있다.
//	그대로 올려 두면 그 파일 주소를 연 사람의 브라우저에서 위키 이름으로 스크립트가 돈다.
//	내보낼 때 CSP 로 한 번 더 막지만, 애초에 받지 않는 것이 먼저다.
//
//	그림에는 쓸 일이 없는 것만 막으므로 보통 SVG 는 그대로 통과한다.
func Svg_script_check(data []byte) bool {
	// 태그와 속성 이름만 보면 되므로 대소문자를 지우고 견준다.
	// 널 문자를 끼워 넣어 검사를 피하는 수법이 있어 먼저 걷어 낸다.
	lower := strings.ToLower(string(data))
	lower = strings.ReplaceAll(lower, "\x00", "")

	for _, bad := range []string{
		"<script",     // 스크립트 그 자체
		"<foreignobject", // 안에 HTML 을 통째로 넣을 수 있다
		"<handler",    // SVG 이벤트 처리기
		"javascript:", // 링크로 스크립트를 부른다
		"data:text/html",
	} {
		if strings.Contains(lower, bad) {
			return true
		}
	}

	// onload= · onclick= 같은 이벤트 속성 (사이에 빈칸이 끼어 있어도 잡는다)
	return svg_event_attr_regex.MatchString(lower)
}

// svg_event_attr_regex : on... = 모양의 이벤트 속성
var svg_event_attr_regex = regexp.MustCompile(`(?s)<[^>]*\son[a-z]+\s*=`)

// Get_file_media_ext_list : 기본 허용 미디어 확장자 목록
func Get_file_media_ext_list() []string {
	return []string{"mp3", "mp4", "webm", "ogg", "wav", "m4a"}
}

// Get_file_media_type : 확장자로 미디어 종류 판별 (image / audio / video / other)
func Get_file_media_type(extension string) string {
	extension = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(extension), "."))

	switch extension {
	case "jpg", "jpeg", "png", "gif", "webp", "bmp", "tif", "tiff", "ico", "svg", "avif", "heic":
		return "image"
	case "mp3", "wav", "flac", "aac", "m4a", "oga", "opus", "amr", "weba":
		return "audio"
	case "mp4", "m4v", "webm", "mkv", "mov", "avi", "mpeg", "mpg", "ts", "3gp", "3g2":
		return "video"
	case "ogg":
		// ogg 는 오디오 사용이 일반적
		return "audio"
	}

	return "other"
}
