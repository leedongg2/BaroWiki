package tool

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
	"sync"

	lang_data "opennamu/lang"
)

var global_lang_data = map[string]string{}
var global_lang_load = map[string]bool{}
var global_lang_lock sync.RWMutex

// Lang_fallback_list : 해당 언어에 없는 문구를 대신 찾아볼 언어 순서
//
//	번역이 아직 없는 열쇠말이 화면에 "key (ja-JP)" 처럼 튀어나오지 않게 한다.
func Lang_fallback_list(language string) []string {
	switch language {
	case "ko-KR":
		return []string{"en-US"}
	case "en-US":
		return []string{"ko-KR"}
	}

	return []string{"en-US", "ko-KR"}
}

func Get_language(db *sql.DB, data string, safe bool) string {
	language := Wiki_setting(db, "language")
	if language == "" {
		language = "ko-KR"
	}

	// 이름공간 표시 이름이 같은 언어를 따라가도록 현재 언어를 기억해 둔다.
	Set_lang_now(language)

	lang_value, ok := Get_lang_cache(language, data)
	if !ok {
		Load_lang_data(language)
		lang_value, ok = Get_lang_cache(language, data)
	}

	if !ok {
		for _, fallback := range Lang_fallback_list(language) {
			lang_value, ok = Get_lang_cache(fallback, data)
			if !ok {
				Load_lang_data(fallback)
				lang_value, ok = Get_lang_cache(fallback, data)
			}

			if ok {
				break
			}
		}
	}

	if ok {
		if safe {
			return lang_value
		} else {
			return HTML_escape(lang_value)
		}
	} else {
		log.Default().Println(data + " (" + language + ")")
		return data + " (" + language + ")"
	}
}

func Get_lang_cache(language string, data string) (string, bool) {
	global_lang_lock.RLock()
	defer global_lang_lock.RUnlock()

	lang_value, ok := global_lang_data[language+"_"+data]
	return lang_value, ok
}

func Load_lang_data(language string) {
	global_lang_lock.RLock()
	lang_load := global_lang_load[language]
	global_lang_lock.RUnlock()
	if lang_load {
		return
	}

	raw, err := lang_data.Read(language)
	if err != nil && language != "ko-KR" {
		raw, err = lang_data.Read("ko-KR")
	}
	if err != nil {
		panic(err)
	}

	lang_json := map[string]string{}
	if err := json.Unmarshal(raw, &lang_json); err != nil {
		panic(err)
	}

	global_lang_lock.Lock()
	defer global_lang_lock.Unlock()

	for k, v := range lang_json {
		global_lang_data[language+"_"+k] = v
	}
	global_lang_load[language] = true
}

// Lang : DB 손잡이 없이 현재 언어의 문구를 얻는다
//
//	현재 언어는 Get_language / Wiki_language 가 갱신해 둔 값을 쓴다.
//	문구가 없으면 en-US -> ko-KR 순으로 찾고, 그래도 없으면 열쇠말을 그대로 돌려준다.
func Lang(key string) string {
	language := Lang_now()

	value, ok := Get_lang_cache(language, key)
	if !ok {
		Load_lang_data(language)
		value, ok = Get_lang_cache(language, key)
	}

	if !ok {
		for _, fallback := range Lang_fallback_list(language) {
			value, ok = Get_lang_cache(fallback, key)
			if !ok {
				Load_lang_data(fallback)
				value, ok = Get_lang_cache(fallback, key)
			}

			if ok {
				break
			}
		}
	}

	if !ok {
		return key
	}

	return value
}

// Lang_esc : Lang 을 HTML 에 바로 넣을 수 있게 이스케이프한 것
func Lang_esc(key string) string {
	return HTML_escape(Lang(key))
}

// Lang_n : 문구 안의 {n} 을 숫자로 바꾼다 (예 : "{n}일" -> "3일")
func Lang_n(key string, number int) string {
	return strings.ReplaceAll(Lang(key), "{n}", strconv.Itoa(number))
}

// Lang_v : 문구 안의 {0} {1} ... 을 차례로 바꾼다
func Lang_v(key string, values ...string) string {
	text := Lang(key)
	for index, value := range values {
		text = strings.ReplaceAll(text, "{"+strconv.Itoa(index)+"}", value)
	}

	return text
}
