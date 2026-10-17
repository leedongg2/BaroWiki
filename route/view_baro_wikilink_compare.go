package route

import (
	"bytes"
	"database/sql"
	encoding_json "encoding/json"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"opennamu/route/tool"
)

// ============================================================
//  같은 대상을 다룬 두 문서 비교
//
//  비교는 두 단계로 한다.
//
//   1) 규칙 비교 (늘 돌아간다)
//        문장을 나눠 양쪽에 다 있는 문장 · 우리에만 있는 문장 ·
//        상대에만 있는 문장으로 가른다.
//        숫자와 날짜가 어긋나는 자리는 따로 모아 보여 준다.
//
//   2) AI 요약 (관리자가 주소를 넣어 둔 경우에만)
//        baro_set 의 ai_compare_url · ai_compare_key · ai_compare_model 이
//        채워져 있으면 두 글을 보내 요약을 받아 온다.
//        주소가 없으면 1) 결과만 보여 주고, 그렇다고 화면에 적어 둔다.
//
//  둘 다 「무엇이 다른가」 만 말한다. 어느 쪽이 옳은지는 사람이 정한다.
// ============================================================

var baro_compare_sentence_regex = regexp.MustCompile(`(?m)[^.!?。\n]+[.!?。]?`)
var baro_compare_number_regex = regexp.MustCompile(`[0-9]+(?:[.,][0-9]+)*`)
var baro_compare_date_regex = regexp.MustCompile(`[0-9]{3,4}\s*년(?:\s*[0-9]{1,2}\s*월)?(?:\s*[0-9]{1,2}\s*일)?|[0-9]{4}-[0-9]{2}-[0-9]{2}`)
var baro_compare_markup_block_regex = regexp.MustCompile(`(?s)\{\{\{.*?\}\}\}|<[^>]{1,200}>`)
var baro_compare_markup_inline_regex = regexp.MustCompile(`\[\[([^\]\[|]*)(?:\|[^\]\[]*)?\]\]|\[[^\]\[]*\]|'''|''|__|\^\^|,,|--|~~|\|\||^\s*[=#*]+`)
var baro_compare_space_regex = regexp.MustCompile(`\s+`)

// baro_compare_result : 비교 결과
type baro_compare_result struct {
	Same        []string
	OnlyMine    []string
	OnlyOther   []string
	NumberGap   [][3]string // {숫자, 우리 문장, 상대 문장}
	DateGap     [][3]string
	MineLength  int
	OtherLength int
	MineCut     int // 상한을 넘어 견주지 못한 우리 문장 수
	OtherCut    int // 상한을 넘어 견주지 못한 상대 문장 수
	Similarity  int
	AISummary   string
	AINote      string
}

// baro_compare_clean : 문법을 걷어 내고 견줄 수 있는 글만 남긴다
func baro_compare_clean(data string) string {
	data = strings.ReplaceAll(data, "\r", "")
	data = baro_compare_markup_block_regex.ReplaceAllString(data, " ")
	data = baro_compare_markup_inline_regex.ReplaceAllString(data, "$1 ")
	data = tool.HTML_unescape(data)

	return strings.TrimSpace(baro_compare_space_regex.ReplaceAllString(data, " "))
}

// baro_compare_sentences : 문장으로 나눈다
func baro_compare_sentences(data string) []string {
	out := []string{}
	for _, one := range baro_compare_sentence_regex.FindAllString(data, -1) {
		one = strings.TrimSpace(one)
		if tool.Get_len(one) < 8 {
			continue
		}

		out = append(out, one)
	}

	return out
}

// baro_compare_key : 견주기 쉬운 형태로 (빈칸·기호를 뺀 글자만)
func baro_compare_key(data string) string {
	var builder strings.Builder
	for _, one := range data {
		switch {
		case one >= '0' && one <= '9':
			builder.WriteRune(one)
		case one >= 'a' && one <= 'z':
			builder.WriteRune(one)
		case one >= 'A' && one <= 'Z':
			builder.WriteRune(one - 'A' + 'a')
		case one > 0x7f:
			builder.WriteRune(one)
		}
	}

	return builder.String()
}

// baro_compare_bigram : 두 글의 닮은 정도 (0~100)
//
//	두 글자씩 잘라 겹치는 비율을 센다. 짧은 글에서도 무난하게 맞는다.
func baro_compare_bigram(left string, right string) int {
	left_key := []rune(baro_compare_key(left))
	right_key := []rune(baro_compare_key(right))
	if len(left_key) < 2 || len(right_key) < 2 {
		if baro_compare_key(left) == baro_compare_key(right) {
			return 100
		}

		return 0
	}

	count := map[string]int{}
	for index := 0; index+1 < len(left_key); index++ {
		count[string(left_key[index:index+2])]++
	}

	hit := 0
	total := 0
	for index := 0; index+1 < len(right_key); index++ {
		pair := string(right_key[index : index+2])
		total++
		if count[pair] > 0 {
			count[pair]--
			hit++
		}
	}

	if total == 0 {
		return 0
	}

	// 양쪽 길이를 함께 반영해 한쪽이 아주 길 때 점수가 부풀지 않게 한다
	longer := len(left_key) - 1
	if len(right_key)-1 > longer {
		longer = len(right_key) - 1
	}

	return hit * 100 / longer
}

// baro_compare_max_sentence : 한쪽에서 견주는 최대 문장 수
//
//	긴 문서 둘을 문장마다 다 맞대어 보면 문장 수의 제곱으로 값이 늘어난다.
//	실제로 재 보니 문장 3,000개씩이면 2분 가까이 걸렸다. 그래서 상한을 둔다.
//	넘친 만큼은 화면에 몇 줄을 못 봤는지 적어 준다.
const baro_compare_max_sentence = 1200

// baro_compare_candidate : 문장 하나마다 자세히 견줄 후보 수
const baro_compare_candidate = 24

// baro_compare_common_pair : 이보다 흔한 두 글자 조각은 후보를 고르는 데 쓰지 않는다
//
//	「입니다」 같은 조각은 거의 모든 문장에 나와 후보를 좁혀 주지 못한다.
const baro_compare_common_pair = 200

// baro_compare_pairs : 문장을 두 글자 조각으로 (같은 조각은 한 번만)
func baro_compare_pairs(data string) []string {
	key := []rune(baro_compare_key(data))
	if len(key) < 2 {
		return nil
	}

	seen := map[string]bool{}
	out := []string{}
	for index := 0; index+1 < len(key); index++ {
		pair := string(key[index : index+2])
		if seen[pair] {
			continue
		}

		seen[pair] = true
		out = append(out, pair)
	}

	return out
}

// baro_compare_run : 규칙 비교
//
//	모든 문장을 서로 맞대어 보는 대신, 두 글자 조각으로 색인을 만들어
//	겹치는 조각이 있는 문장만 자세히 견준다. 결과는 같고 값은 훨씬 적게 든다.
func baro_compare_run(mine string, other string) baro_compare_result {
	result := baro_compare_result{}

	mine_clean := baro_compare_clean(mine)
	other_clean := baro_compare_clean(other)

	result.MineLength = tool.Get_len(mine_clean)
	result.OtherLength = tool.Get_len(other_clean)
	result.Similarity = baro_compare_bigram(mine_clean, other_clean)

	mine_list := baro_compare_sentences(mine_clean)
	other_list := baro_compare_sentences(other_clean)

	if len(mine_list) > baro_compare_max_sentence {
		result.MineCut = len(mine_list) - baro_compare_max_sentence
		mine_list = mine_list[:baro_compare_max_sentence]
	}
	if len(other_list) > baro_compare_max_sentence {
		result.OtherCut = len(other_list) - baro_compare_max_sentence
		other_list = other_list[:baro_compare_max_sentence]
	}

	// 상대 문장을 두 글자 조각으로 색인한다
	pair_index := map[string][]int{}
	for index, one := range other_list {
		for _, pair := range baro_compare_pairs(one) {
			pair_index[pair] = append(pair_index[pair], index)
		}
	}

	used := make([]bool, len(other_list))
	overlap := make([]int, len(other_list))
	touched := []int{}

	for _, one := range mine_list {
		// 겹치는 조각 수를 세어 후보를 좁힌다
		for _, index := range touched {
			overlap[index] = 0
		}
		touched = touched[:0]

		for _, pair := range baro_compare_pairs(one) {
			list := pair_index[pair]
			if len(list) > baro_compare_common_pair {
				continue
			}

			for _, index := range list {
				if used[index] {
					continue
				}
				if overlap[index] == 0 {
					touched = append(touched, index)
				}
				overlap[index]++
			}
		}

		// 겹치는 조각이 많은 차례로 몇 개만 자세히 견준다
		sort.SliceStable(touched, func(a int, b int) bool {
			return overlap[touched[a]] > overlap[touched[b]]
		})

		best := -1
		best_score := 0
		for rank, index := range touched {
			if rank >= baro_compare_candidate {
				break
			}

			score := baro_compare_bigram(one, other_list[index])
			if score > best_score {
				best_score = score
				best = index
			}
		}

		if best >= 0 && best_score >= 70 {
			used[best] = true
			result.Same = append(result.Same, one)

			// 거의 같은 문장인데 숫자·날짜만 다르면 확인이 필요하다
			baro_compare_gap(&result, one, other_list[best])

			continue
		}

		result.OnlyMine = append(result.OnlyMine, one)
	}

	for index, one := range other_list {
		if !used[index] {
			result.OnlyOther = append(result.OnlyOther, one)
		}
	}

	return result
}

// baro_compare_gap : 거의 같은 두 문장에서 숫자 · 날짜가 어긋나는 자리를 모은다
func baro_compare_gap(result *baro_compare_result, mine string, other string) {
	mine_numbers := baro_compare_number_regex.FindAllString(mine, -1)
	other_numbers := baro_compare_number_regex.FindAllString(other, -1)
	if !baro_compare_same_set(mine_numbers, other_numbers) {
		result.NumberGap = append(result.NumberGap, [3]string{
			strings.Join(mine_numbers, ", ") + " ↔ " + strings.Join(other_numbers, ", "),
			mine, other,
		})
	}

	mine_dates := baro_compare_date_regex.FindAllString(mine, -1)
	other_dates := baro_compare_date_regex.FindAllString(other, -1)
	if !baro_compare_same_set(mine_dates, other_dates) {
		result.DateGap = append(result.DateGap, [3]string{
			strings.Join(mine_dates, ", ") + " ↔ " + strings.Join(other_dates, ", "),
			mine, other,
		})
	}
}

// baro_compare_same_set : 두 목록이 같은 값 묶음인지
func baro_compare_same_set(left []string, right []string) bool {
	if len(left) == 0 && len(right) == 0 {
		return true
	}
	if len(left) != len(right) {
		return false
	}

	left_copy := append([]string{}, left...)
	right_copy := append([]string{}, right...)
	sort.Strings(left_copy)
	sort.Strings(right_copy)

	for index := range left_copy {
		if left_copy[index] != right_copy[index] {
			return false
		}
	}

	return true
}

// baro_compare_ai_setting : AI 요약 설정 (주소, 열쇠, 모형 이름)
func baro_compare_ai_setting(db *sql.DB) (string, string, string) {
	return tool.Baro_get(db, "ai_compare_url"),
		tool.Baro_get(db, "ai_compare_key"),
		tool.Baro_get(db, "ai_compare_model")
}

// baro_compare_ai : AI 로 두 글을 견준 요약을 받아 온다
//
//	OpenAI 호환 chat/completions 규격으로 보낸다.
//	주소가 없으면 아무것도 하지 않고 까닭만 돌려준다.
func baro_compare_ai(db *sql.DB, mine_title string, mine string, other_title string, other string) (string, string) {
	endpoint, key, model := baro_compare_ai_setting(db)
	if endpoint == "" {
		return "", tool.Baro_t("AI 요약 주소가 설정되어 있지 않아 규칙 비교만 했습니다. 관리자가 /baro/wikilink/set 에서 넣을 수 있습니다.")
	}
	if _, check_error := tool.Baro_fetch_url_check(endpoint); check_error != "" {
		return "", tool.Baro_t("AI 요약 주소가 올바르지 않습니다 : ") + check_error
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	prompt := "너는 위키 편집자를 돕는 비교 도우미다. 아래 두 글은 같은 대상을 다룬 서로 다른 위키의 문서다.\n" +
		"어느 쪽이 옳은지 판정하지 말고, 다음만 한국어로 정리해라.\n" +
		"1. 두 글이 공통으로 말하는 핵심\n" +
		"2. 우리 문서에만 있는 내용\n" +
		"3. 상대 문서에만 있는 내용\n" +
		"4. 사실이 서로 어긋나 보여 사람이 확인해야 하는 자리 (숫자·날짜·이름)\n\n" +
		"[우리 문서 : " + mine_title + "]\n" + tool.Get_slice(mine, 0, 8000) +
		"\n\n[상대 문서 : " + other_title + "]\n" + tool.Get_slice(other, 0, 8000)

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
	}

	body, err := encoding_json.Marshal(payload)
	if err != nil {
		return "", tool.Baro_t("요청을 만들지 못했습니다.")
	}

	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", tool.Baro_t("요청을 만들지 못했습니다.")
	}
	request.Header.Set("Content-Type", "application/json")
	if key != "" {
		request.Header.Set("Authorization", "Bearer "+key)
		request.Header.Set("x-api-key", key)
		request.Header.Set("anthropic-version", "2023-06-01")
	}

	client := &http.Client{Timeout: 60 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", tool.Baro_t("AI 요약을 받아 오지 못했습니다 : ") + err.Error()
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", tool.Baro_t("AI 응답을 읽지 못했습니다.")
	}
	if response.StatusCode != http.StatusOK {
		return "", tool.Baro_t("AI 서버가 ") + response.Status + tool.Baro_t(" 를 돌려줬습니다.")
	}

	return baro_compare_ai_text(raw)
}

// baro_compare_ai_text : 여러 AI 응답 모양에서 글만 뽑아낸다
func baro_compare_ai_text(raw []byte) (string, string) {
	parsed := map[string]any{}
	if err := encoding_json.Unmarshal(raw, &parsed); err != nil {
		return "", tool.Baro_t("AI 응답을 읽지 못했습니다.")
	}

	// OpenAI 호환 : choices[0].message.content
	if choices, ok := parsed["choices"].([]any); ok && len(choices) > 0 {
		if first, ok := choices[0].(map[string]any); ok {
			if message, ok := first["message"].(map[string]any); ok {
				if content, ok := message["content"].(string); ok && content != "" {
					return content, ""
				}
			}
			if text, ok := first["text"].(string); ok && text != "" {
				return text, ""
			}
		}
	}

	// Anthropic 규격 : content[0].text
	if content, ok := parsed["content"].([]any); ok && len(content) > 0 {
		if first, ok := content[0].(map[string]any); ok {
			if text, ok := first["text"].(string); ok && text != "" {
				return text, ""
			}
		}
	}

	return "", tool.Baro_t("AI 응답에서 요약을 찾지 못했습니다.")
}

// baro_compare_list_html : 문장 목록 한 칸
func baro_compare_list_html(title string, items []string, limit int) string {
	html := `<div class="bw_diff_col"><div class="bw_diff_t">` + tool.Baro_t_esc(title) +
		` <span class="bw_muted">(` + strconv.Itoa(len(items)) + `)</span></div>`

	if len(items) == 0 {
		return html + `<div class="bw_diff_l bw_muted">` + tool.Baro_t_esc("없습니다.") + `</div></div>`
	}

	for index, one := range items {
		if index >= limit {
			html += `<div class="bw_diff_l bw_muted">… ` + strconv.Itoa(len(items)-limit) + tool.Baro_t_esc("줄 더 있습니다.") + `</div>`

			break
		}

		html += `<div class="bw_diff_l">` + tool.HTML_escape(one) + `</div>`
	}

	return html + `</div>`
}

// baro_compare_gap_html : 어긋나는 자리 표
func baro_compare_gap_html(db_title string, gaps [][3]string) string {
	rows := [][]string{}
	for _, one := range gaps {
		rows = append(rows, []string{
			tool.HTML_escape(one[0]),
			tool.HTML_escape(tool.Baro_trunc(one[1], 160)),
			tool.HTML_escape(tool.Baro_trunc(one[2], 160)),
		})
	}

	return tool.Baro_table(
		[]string{db_title, tool.Baro_t("우리 문서"), tool.Baro_t("상대 문서")},
		rows, tool.Baro_t("어긋나는 자리를 찾지 못했습니다."),
	)
}

// ============================================================
//  받아 온 글에서 본문만 꺼내기
//
//  상대 위키의 「원문」 주소가 늘 plain text 인 것은 아니다.
//  openNAMU 는 원문도 편집창이 든 HTML 한 장으로 준다.
//  그래서 받아 온 글의 모양을 보고 본문만 꺼낸다.
//
//    1) openNAMU 원문 화면  … 편집창(textarea) 안의 글
//    2) 그 밖의 HTML        … 스크립트·스타일을 버리고 태그를 벗긴 글
//    3) 그냥 글             … 그대로
// ============================================================

var baro_compare_textarea_regex = regexp.MustCompile(`(?is)<textarea[^>]*id="opennamu_edit_textarea"[^>]*>(.*?)</textarea>`)

// 되돌아 참조(\1)는 Go 정규식이 지원하지 않아 태그마다 따로 적는다
var baro_compare_drop_regex = regexp.MustCompile(
	`(?is)<script[^>]*>.*?</script>` +
		`|<style[^>]*>.*?</style>` +
		`|<head[^>]*>.*?</head>` +
		`|<nav[^>]*>.*?</nav>` +
		`|<footer[^>]*>.*?</footer>` +
		`|<!--.*?-->`,
)
var baro_compare_tag_regex = regexp.MustCompile(`(?s)<[^>]*>`)
var baro_compare_html_regex = regexp.MustCompile(`(?i)<!doctype html|<html\b`)

// baro_compare_extract_remote : 받아 온 글에서 견줄 본문만 꺼낸다
func baro_compare_extract_remote(raw string) string {
	if match := baro_compare_textarea_regex.FindStringSubmatch(raw); len(match) == 2 {
		return tool.HTML_unescape(match[1])
	}

	if !baro_compare_html_regex.MatchString(raw) {
		return raw
	}

	body := baro_compare_drop_regex.ReplaceAllString(raw, "\n")
	body = baro_compare_tag_regex.ReplaceAllString(body, " ")

	return tool.HTML_unescape(body)
}
