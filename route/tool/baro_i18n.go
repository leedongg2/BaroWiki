package tool

import (
	"strings"
	"sync"

	lang_data "opennamu/lang"
)

// ============================================================
//  바로위키 화면 문구 번역
//
//  바로위키 모듈은 화면 문구를 한국어로 쓴다. (BARO_CONTRACT 10번 규칙)
//  그래서 열쇠말을 따로 만들지 않고 **한국어 원문 자체를 열쇠말로** 쓴다.
//
//    lang/baro-en-US.json : {"바로위키": "Baro Wiki", ...}
//    lang/baro-ja-JP.json : {"바로위키": "バロウィキ", ...}
//
//  번역이 없으면 한국어를 그대로 보여 준다. (화면이 비지 않는다)
//  번역은 화면에 내보내는 자리(UI 부품)에서만 한다.
//  DB 에 저장하거나 값을 비교하는 자리에서는 절대 부르지 않는다.
// ============================================================

var baro_t_data = map[string]map[string]string{}
var baro_t_lock sync.RWMutex

// baro_t_load : 해당 언어의 번역표를 한 번만 읽는다
func baro_t_load(language string) map[string]string {
	baro_t_lock.RLock()
	table, ok := baro_t_data[language]
	baro_t_lock.RUnlock()
	if ok {
		return table
	}

	table = map[string]string{}

	if raw, err := lang_data.Read("baro-" + language); err == nil {
		loaded := map[string]string{}
		if json.Unmarshal(raw, &loaded) == nil {
			table = loaded
		}
	}

	baro_t_lock.Lock()
	baro_t_data[language] = table
	baro_t_lock.Unlock()

	return table
}

// Baro_t : 바로위키 화면 문구를 현재 언어로 바꾼다
//
//	앞뒤 공백은 그대로 두고 가운데 글자만 찾는다.
//	(" 남음)" 처럼 앞에 칸이 붙은 문구도 같은 번역을 쓴다)
func Baro_t(text string) string {
	if text == "" {
		return text
	}

	language := Lang_now()
	if language == "ko-KR" {
		return text
	}

	core := strings.TrimSpace(text)
	if core == "" {
		return text
	}

	table := baro_t_load(language)

	value, ok := table[core]
	if !ok {
		// 번역이 없으면 다른 언어판에서 찾아본다
		for _, fallback := range Lang_fallback_list(language) {
			if fallback == "ko-KR" {
				break
			}

			if value, ok = baro_t_load(fallback)[core]; ok {
				break
			}
		}
	}

	if !ok {
		return text
	}

	head := text[:strings.Index(text, core)]
	tail := text[len(head)+len(core):]

	return head + value + tail
}

// Baro_t_esc : Baro_t 결과를 HTML 에 바로 넣을 수 있게 이스케이프
func Baro_t_esc(text string) string {
	return HTML_escape(Baro_t(text))
}

// Baro_t_list : 문구 여러 개를 한꺼번에
func Baro_t_list(list []string) []string {
	if len(list) == 0 {
		return list
	}

	out := make([]string, len(list))
	for index, one := range list {
		out[index] = Baro_t(one)
	}

	return out
}

// Baro_t_pairs : [라벨, 값] 쌍에서 라벨만 번역
func Baro_t_pairs(pairs [][]string) [][]string {
	if len(pairs) == 0 {
		return pairs
	}

	out := make([][]string, len(pairs))
	for index, pair := range pairs {
		if len(pair) == 0 {
			out[index] = pair
			continue
		}

		row := make([]string, len(pair))
		copy(row, pair)
		row[0] = Baro_t(row[0])
		out[index] = row
	}

	return out
}

// Baro_t_menu : 메뉴 [주소, 이름] 에서 이름만 번역
func Baro_t_menu(menu [][]any) [][]any {
	if len(menu) == 0 {
		return menu
	}

	out := make([][]any, len(menu))
	for index, item := range menu {
		row := make([]any, len(item))
		copy(row, item)

		if len(row) > 1 {
			if name, ok := row[1].(string); ok {
				row[1] = Baro_t(name)
			}
		}

		out[index] = row
	}

	return out
}
