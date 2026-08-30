package route

import (
	baro_imp_json "encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  13장 기존 위키 파일 · 자료 이전 — 읽어들이기 / 변환 계층
//
//  친구가 이미 만들어 둔 위키 파일을 그대로 이어서 쓰기 위한 부분이다.
//  형식이 제각각이므로 "관대하게 읽고, 무엇을 어떻게 바꿨는지 전부 보여준다"를
//  원칙으로 삼는다.
// ============================================================

// baro_imp_doc : 가져오기 후보 문서 하나
type baro_imp_doc struct {
	Title   string // 문서 제목
	Text    string // 본문 (변환 전)
	Author  string // 원 작성자
	Url     string // 원본 출처 주소
	License string // 개별 라이선스 (비면 작업 기본값을 쓴다)
	Note    string // 비고
	Err     string // 오류 사유 (비어 있으면 정상)
}

// baro_imp_conv : 본문 변환 단계별 처리 건수
type baro_imp_conv struct {
	Br       int // 줄바꿈 태그 변환
	Category int // 분류 변환
	File     int // 파일 문법 변환
	Template int // 틀 변환
	Ext      int // 바깥 링크 변환
	Link     int // 문서 링크 개수
}

// baro_imp_kind_list : 지원하는 원본 형식
func baro_imp_kind_list() [][]string {
	return [][]string{
		{"json", tool.Baro_t("JSON 파일 ([{title,text}] 또는 {\"문서명\":\"본문\"})")},
		{"heading", "여러 문서를 담은 텍스트 (== 제목 == 또는 사용자 지정 구분자)"},
		{"files", tool.Baro_t("개별 텍스트 파일 여러 개 (파일 이름이 문서 제목)")},
		{"opennamu", tool.Baro_t("openNAMU / 나무위키식 백업 JSON (관대한 해석)")},
	}
}

// baro_imp_kind_name : 원본 형식 표시 이름
func baro_imp_kind_name(kind string) string {
	for _, row := range baro_imp_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// ------------------------------------------------------------
//  JSON 해석
// ------------------------------------------------------------

var (
	baro_imp_title_keys   = []string{"title", "name", "doc", "document", "doc_name", "docname", "제목", "문서", "문서명"}
	baro_imp_text_keys    = []string{"text", "data", "content", "body", "wiki", "markup", "source_text", "본문", "내용"}
	baro_imp_author_keys  = []string{"author", "ip", "user", "editor", "contributor", "writer", "작성자", "원작성자", "기여자"}
	baro_imp_url_keys     = []string{"url", "source", "link", "origin", "source_url", "출처", "원본"}
	baro_imp_license_keys = []string{"license", "licence", "라이선스", "라이센스"}
)

// baro_imp_to_str : JSON 값 하나를 문자열로
func baro_imp_to_str(value any) string {
	switch data := value.(type) {
	case string:
		return data
	case float64:
		if data == float64(int64(data)) {
			return strconv.FormatInt(int64(data), 10)
		}

		return strconv.FormatFloat(data, 'f', -1, 64)
	case bool:
		if data {
			return "on"
		}

		return ""
	}

	return ""
}

// baro_imp_map_lower : 키를 소문자로 낮춘 사본
func baro_imp_map_lower(row map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range row {
		out[strings.ToLower(strings.TrimSpace(key))] = value
	}

	return out
}

// baro_imp_map_pick : 여러 후보 키 중 먼저 발견되는 문자열 값
func baro_imp_map_pick(row map[string]any, keys []string) (string, bool) {
	lower := baro_imp_map_lower(row)
	for _, key := range keys {
		value, ok := lower[strings.ToLower(key)]
		if !ok {
			continue
		}

		data := baro_imp_to_str(value)
		if strings.TrimSpace(data) != "" {
			return data, true
		}
	}

	return "", false
}

// baro_imp_map_has_str : 후보 키가 (값이 비었더라도) 글자 값으로 있는지
//
//	값이 배열이나 객체이면 문서 칸이 아니라 감싸개(예: {"data": [...]})로 본다.
func baro_imp_map_has_str(row map[string]any, keys []string) bool {
	lower := baro_imp_map_lower(row)
	for _, key := range keys {
		value, ok := lower[strings.ToLower(key)]
		if !ok {
			continue
		}

		switch value.(type) {
		case string, float64, bool, nil:
			return true
		}
	}

	return false
}

// baro_imp_sorted_keys : 재현 가능한 순서를 위해 키를 정렬
func baro_imp_sorted_keys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys
}

// baro_imp_json_walk : JSON 구조를 훑어 문서를 찾아낸다
//
//	백업 파일 형태가 제각각이므로 관대하게 훑는다.
func baro_imp_json_walk(value any, out *[]baro_imp_doc, depth int) {
	if depth > 10 || len(*out) > 20000 {
		return
	}

	switch data := value.(type) {
	case []any:
		for _, item := range data {
			baro_imp_json_walk(item, out, depth+1)
		}
	case map[string]any:
		title, has_title := baro_imp_map_pick(data, baro_imp_title_keys)
		text, has_text := baro_imp_map_pick(data, baro_imp_text_keys)

		if has_title && has_text {
			author, _ := baro_imp_map_pick(data, baro_imp_author_keys)
			source, _ := baro_imp_map_pick(data, baro_imp_url_keys)
			license, _ := baro_imp_map_pick(data, baro_imp_license_keys)

			*out = append(*out, baro_imp_doc{
				Title:   title,
				Text:    text,
				Author:  author,
				Url:     source,
				License: license,
			})

			return
		}

		title_key := baro_imp_map_has_str(data, baro_imp_title_keys)
		text_key := baro_imp_map_has_str(data, baro_imp_text_keys)

		// 문서 모양이지만 한쪽이 비어 있는 경우는 오류로 남긴다.
		if title_key || text_key {
			switch {
			case has_title:
				*out = append(*out, baro_imp_doc{Title: title, Err: tool.Baro_t("본문이 비어 있습니다.")})
			case has_text:
				*out = append(*out, baro_imp_doc{Err: tool.Baro_t("문서 제목을 읽지 못했습니다.")})
			default:
				*out = append(*out, baro_imp_doc{Err: tool.Baro_t("제목과 본문을 모두 읽지 못했습니다.")})
			}

			return
		}

		// 문서명을 키로 삼은 형태인지 살펴본다.
		plain, nested := 0, 0
		for _, item := range data {
			switch item.(type) {
			case string:
				plain++
			case map[string]any, []any:
				nested++
			}
		}

		if plain > 0 && nested == 0 {
			for _, key := range baro_imp_sorted_keys(data) {
				body := baro_imp_to_str(data[key])
				if strings.TrimSpace(body) == "" {
					*out = append(*out, baro_imp_doc{Title: key, Err: tool.Baro_t("본문이 비어 있습니다.")})

					continue
				}

				*out = append(*out, baro_imp_doc{Title: key, Text: body})
			}

			return
		}

		for _, key := range baro_imp_sorted_keys(data) {
			baro_imp_json_walk(data[key], out, depth+1)
		}
	}
}

// baro_imp_parse_json : JSON / JSON Lines 해석
func baro_imp_parse_json(content string) ([]baro_imp_doc, string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, tool.Baro_t("내용이 비어 있습니다.")
	}

	out := []baro_imp_doc{}

	var root any
	if err := baro_imp_json.Unmarshal([]byte(content), &root); err == nil {
		baro_imp_json_walk(root, &out, 0)
	} else {
		// 한 줄에 하나씩 들어 있는 JSON Lines 도 받아들인다.
		ok_line := 0
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ","))
			if line == "" || line == "[" || line == "]" {
				continue
			}

			var one any
			if baro_imp_json.Unmarshal([]byte(line), &one) != nil {
				continue
			}

			ok_line++
			baro_imp_json_walk(one, &out, 0)
		}

		if ok_line == 0 {
			return nil, tool.Baro_t("JSON 을 읽지 못했습니다: ") + err.Error()
		}
	}

	if len(out) == 0 {
		return nil, tool.Baro_t("JSON 안에서 문서를 찾지 못했습니다. title/text 짝 또는 문서명을 키로 쓴 형태를 지원합니다.")
	}

	return out, ""
}

// ------------------------------------------------------------
//  텍스트 해석
// ------------------------------------------------------------

// baro_imp_re_head : 제목 줄 (== 제목 == 형태)
var baro_imp_re_head = regexp.MustCompile(`(?m)^[ \t]*(={1,6})[ \t]*([^=\n][^\n]*?)[ \t]*(={1,6})[ \t]*$`)

// baro_imp_head_hit : 찾아낸 제목 줄 위치
type baro_imp_head_hit struct {
	Start      int
	End        int
	TitleStart int
	TitleEnd   int
	Level      int
}

// baro_imp_parse_head : 제목 줄로 나뉜 여러 문서 파일 해석
//
//	파일 안에서 가장 얕은 제목 단계를 문서 경계로 삼는다.
func baro_imp_parse_head(content string) []baro_imp_doc {
	found := baro_imp_re_head.FindAllStringSubmatchIndex(content, -1)

	hits := []baro_imp_head_hit{}
	for _, one := range found {
		left := one[3] - one[2]
		right := one[7] - one[6]
		if left != right {
			continue
		}

		hits = append(hits, baro_imp_head_hit{
			Start:      one[0],
			End:        one[1],
			TitleStart: one[4],
			TitleEnd:   one[5],
			Level:      left,
		})
	}

	if len(hits) == 0 {
		return nil
	}

	level := 9
	for _, hit := range hits {
		if hit.Level < level {
			level = hit.Level
		}
	}

	picked := []baro_imp_head_hit{}
	for _, hit := range hits {
		if hit.Level == level {
			picked = append(picked, hit)
		}
	}

	out := []baro_imp_doc{}
	for index, hit := range picked {
		end := len(content)
		if index+1 < len(picked) {
			end = picked[index+1].Start
		}

		title := strings.TrimSpace(content[hit.TitleStart:hit.TitleEnd])
		body := strings.TrimSpace(content[hit.End:end])

		doc := baro_imp_doc{Title: title, Text: body}
		if body == "" {
			doc.Err = tool.Baro_t("본문이 비어 있습니다.")
		}

		out = append(out, doc)
	}

	return out
}

// baro_imp_parse_delim : 사용자 지정 구분자로 나뉜 여러 문서 파일 해석
//
//	구분자 줄 다음의 첫 줄을 제목으로, 나머지를 본문으로 본다.
func baro_imp_parse_delim(content string, delim string) []baro_imp_doc {
	delim = strings.TrimSpace(delim)
	if delim == "" {
		return nil
	}

	chunks := [][]string{}
	current := []string{}

	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == delim {
			chunks = append(chunks, current)
			current = []string{}

			continue
		}

		current = append(current, line)
	}
	chunks = append(chunks, current)

	out := []baro_imp_doc{}
	for _, chunk := range chunks {
		title := ""
		body_start := 0

		for index, line := range chunk {
			if strings.TrimSpace(line) == "" {
				continue
			}

			title = strings.TrimSpace(line)
			body_start = index + 1

			break
		}

		if title == "" {
			continue
		}

		body := strings.TrimSpace(strings.Join(chunk[body_start:], "\n"))

		doc := baro_imp_doc{Title: title, Text: body}
		if body == "" {
			doc.Err = tool.Baro_t("본문이 비어 있습니다.")
		}

		out = append(out, doc)
	}

	return out
}

// baro_imp_file_title : 파일 이름에서 문서 제목을 뽑는다
func baro_imp_file_title(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if index := strings.LastIndex(name, "/"); index >= 0 {
		name = name[index+1:]
	}

	for _, ext := range []string{".txt", ".md", ".wiki", ".namu", ".text", ".json"} {
		if strings.HasSuffix(strings.ToLower(name), ext) {
			name = name[:len(name)-len(ext)]

			break
		}
	}

	name = strings.ReplaceAll(name, "_", " ")

	return strings.TrimSpace(name)
}

// baro_imp_parse_all : 형식에 맞춰 원본 하나를 문서 목록으로 바꾼다
func baro_imp_parse_all(kind string, delim string, name string, content string) ([]baro_imp_doc, string) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	switch kind {
	case "json", "opennamu":
		return baro_imp_parse_json(content)
	case "files":
		title := baro_imp_file_title(name)
		if title == "" {
			return nil, tool.Baro_t("파일 이름에서 문서 제목을 얻지 못했습니다.")
		}

		doc := baro_imp_doc{Title: title, Text: strings.TrimSpace(content)}
		if doc.Text == "" {
			doc.Err = tool.Baro_t("본문이 비어 있습니다.")
		}

		return []baro_imp_doc{doc}, ""
	case "heading":
		if strings.TrimSpace(delim) != "" {
			docs := baro_imp_parse_delim(content, delim)
			if len(docs) == 0 {
				return nil, tool.Baro_t("지정한 구분자로 나눌 수 있는 문서를 찾지 못했습니다.")
			}

			return docs, ""
		}

		docs := baro_imp_parse_head(content)
		if len(docs) == 0 {
			return nil, "제목 줄(== 제목 ==) 형태의 문서 경계를 찾지 못했습니다. 구분자를 직접 지정해 보세요."
		}

		return docs, ""
	}

	return nil, tool.Baro_t("알 수 없는 원본 형식입니다: ") + kind
}

// ------------------------------------------------------------
//  본문 변환
// ------------------------------------------------------------

var (
	// 분류 변환 : Category / :분류 형태를 분류 로 통일
	baro_imp_re_cat = regexp.MustCompile(`\[\[[ \t]*:?[ \t]*(?i:분류|category)[ \t]*:[ \t]*([^\]|]+?)[ \t]*(?:\|[^\]]*)?\]\]`)
	// 파일 변환 : File / Image / 그림 형태를 파일 로 통일
	baro_imp_re_file = regexp.MustCompile(`\[\[[ \t]*:?[ \t]*(?i:파일|그림|file|image)[ \t]*:[ \t]*([^\]|]+?)[ \t]*(\|[^\]]*)?\]\]`)
	// 틀 변환 : 중괄호 틀 호출을 include 문법으로
	baro_imp_re_tpl = regexp.MustCompile(`\{\{[ \t]*(?i:틀|template)[ \t]*:[ \t]*([^}|]+?)[ \t]*(?:\|[^}]*)?\}\}`)
	// 바깥 링크 변환 : 대괄호 하나짜리 링크를 위키 링크로
	baro_imp_re_ext = regexp.MustCompile(`\[(https?://[^\s\]|]+)[ \t]+([^\]\n]+)\]`)
	// 줄바꿈 태그
	baro_imp_re_br = regexp.MustCompile(`(?i)<br[ \t]*/?>`)
	// 문서 링크 세기
	baro_imp_re_link = regexp.MustCompile(`\[\[([^\]\n]+)\]\]`)
	// 변환이 끝난 뒤의 파일 링크
	baro_imp_re_file_used = regexp.MustCompile(`\[\[[ \t]*파일[ \t]*:[ \t]*([^\]|\n]+?)[ \t]*(?:\|[^\]\n]*)?\]\]`)
)

// baro_imp_convert : 본문 변환 (분류 · 링크 · 틀 · 파일 문법을 이 위키 문법으로)
func baro_imp_convert(text string) (string, baro_imp_conv) {
	conv := baro_imp_conv{}

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	conv.Br = len(baro_imp_re_br.FindAllString(text, -1))
	text = baro_imp_re_br.ReplaceAllString(text, "\n")

	conv.Category = len(baro_imp_re_cat.FindAllString(text, -1))
	text = baro_imp_re_cat.ReplaceAllString(text, "[[분류:${1}]]")

	conv.File = len(baro_imp_re_file.FindAllString(text, -1))
	text = baro_imp_re_file.ReplaceAllString(text, "[[파일:${1}${2}]]")

	conv.Template = len(baro_imp_re_tpl.FindAllString(text, -1))
	text = baro_imp_re_tpl.ReplaceAllString(text, "[include(틀:${1})]")

	conv.Ext = len(baro_imp_re_ext.FindAllString(text, -1))
	text = baro_imp_re_ext.ReplaceAllString(text, "[[${1}|${2}]]")

	conv.Link = len(baro_imp_re_link.FindAllString(text, -1))

	for strings.Contains(text, "\n\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n\n", "\n\n\n")
	}

	return strings.TrimSpace(text), conv
}

// baro_imp_conv_note : 변환 결과를 사람이 읽을 수 있는 한 줄로
func baro_imp_conv_note(conv baro_imp_conv) string {
	parts := []string{}
	if conv.Category > 0 {
		parts = append(parts, "분류 "+tool.Baro_itoa(conv.Category))
	}
	if conv.Link > 0 {
		parts = append(parts, tool.Baro_t("링크 ")+tool.Baro_itoa(conv.Link))
	}
	if conv.Template > 0 {
		parts = append(parts, tool.Baro_t("틀 ")+tool.Baro_itoa(conv.Template))
	}
	if conv.File > 0 {
		parts = append(parts, tool.Baro_t("파일 ")+tool.Baro_itoa(conv.File))
	}
	if conv.Ext > 0 {
		parts = append(parts, tool.Baro_t("바깥링크 ")+tool.Baro_itoa(conv.Ext))
	}
	if conv.Br > 0 {
		parts = append(parts, tool.Baro_t("줄바꿈 ")+tool.Baro_itoa(conv.Br))
	}

	if len(parts) == 0 {
		return tool.Baro_t("변환 없음")
	}

	return strings.Join(parts, " · ")
}

// baro_imp_file_links : 본문이 쓰는 파일 문서 이름 목록
func baro_imp_file_links(text string) []string {
	out := []string{}
	seen := map[string]bool{}

	for _, one := range baro_imp_re_file_used.FindAllStringSubmatch(text, -1) {
		name := strings.TrimSpace(one[1])
		if name == "" || seen[name] {
			continue
		}

		seen[name] = true
		out = append(out, name)
	}

	return out
}

// baro_imp_norm_title : 문서 제목 다듬기 (빈 문자열이면 쓸 수 없는 제목)
func baro_imp_norm_title(title string) string {
	title = strings.ReplaceAll(title, "\r", " ")
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.ReplaceAll(title, "\t", " ")

	for strings.Contains(title, "  ") {
		title = strings.ReplaceAll(title, "  ", " ")
	}

	title = strings.TrimSpace(title)
	title = strings.Trim(title, "/")
	title = strings.TrimSpace(title)

	if title == "" {
		return ""
	}

	if tool.Get_len(title) > 200 {
		title = tool.Get_slice(title, 0, 200)
	}

	return tool.Namespace_normalize(title)
}
