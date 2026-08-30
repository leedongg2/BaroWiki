package tool

import (
	"strconv"
	"strings"
)

// Section_heading : 문단 정보
type Section_heading struct {
	Number string
	Level  int
	Line   int
	Text   string
}

// Section_parse_heading : 문단 제목 줄인지 확인하고 (단계, 제목) 반환
func Section_parse_heading(line string) (int, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, "", false
	}

	level := 0
	for level < len(line) && level < 6 && line[level] == '=' {
		level++
	}

	if level == 0 || level == len(line) {
		return 0, "", false
	}

	heading_data := strings.TrimSpace(line[level:])
	heading_folding := strings.HasPrefix(heading_data, "#")
	if heading_folding {
		heading_data = strings.TrimSpace(heading_data[1:])
	}

	trailing := 0
	for trailing < len(heading_data) && trailing < 6 && heading_data[len(heading_data)-trailing-1] == '=' {
		trailing++
	}
	if trailing > 0 {
		heading_data = strings.TrimSpace(heading_data[:len(heading_data)-trailing])
	}

	trailing_folding := strings.HasSuffix(heading_data, "#")
	if heading_folding != trailing_folding {
		return 0, "", false
	}
	if trailing_folding {
		heading_data = strings.TrimSpace(strings.TrimSuffix(heading_data, "#"))
	}

	if heading_data == "" {
		return 0, "", false
	}

	return level, heading_data, true
}

// Section_list : 문서 원문에서 문단 목록 (렌더러와 같은 번호 규칙)
func Section_list(raw_data string) []Section_heading {
	lines := strings.Split(raw_data, "\n")
	heading_stack := [6]int{}
	section_list := []Section_heading{}

	for index, line := range lines {
		level, text, ok := Section_parse_heading(line)
		if !ok {
			continue
		}

		heading_stack[level-1]++
		for stack_index := level; stack_index < len(heading_stack); stack_index++ {
			heading_stack[stack_index] = 0
		}

		number_list := []string{}
		for stack_index := 0; stack_index < level; stack_index++ {
			if heading_stack[stack_index] > 0 {
				number_list = append(number_list, strconv.Itoa(heading_stack[stack_index]))
			}
		}

		section_list = append(section_list, Section_heading{
			Number: strings.Join(number_list, "."),
			Level:  level,
			Line:   index,
			Text:   text,
		})
	}

	return section_list
}

// Section_range : 문단 번호에 해당하는 줄 범위 (끝 줄은 포함하지 않음)
func Section_range(raw_data string, number string) (int, int, bool) {
	if number == "" {
		return 0, 0, false
	}

	section_list := Section_list(raw_data)
	lines := strings.Split(raw_data, "\n")

	for index, section := range section_list {
		if section.Number != number {
			continue
		}

		end_line := len(lines)
		for next_index := index + 1; next_index < len(section_list); next_index++ {
			if section_list[next_index].Level <= section.Level {
				end_line = section_list[next_index].Line

				break
			}
		}

		return section.Line, end_line, true
	}

	return 0, 0, false
}

// Section_text : 문단 원문
func Section_text(raw_data string, number string) (string, bool) {
	start_line, end_line, ok := Section_range(raw_data, number)
	if !ok {
		return "", false
	}

	lines := strings.Split(raw_data, "\n")

	return strings.Join(lines[start_line:end_line], "\n"), true
}

// Section_replace : 문단 원문 교체
func Section_replace(raw_data string, number string, new_data string) (string, bool) {
	start_line, end_line, ok := Section_range(raw_data, number)
	if !ok {
		return raw_data, false
	}

	lines := strings.Split(raw_data, "\n")
	new_lines := strings.Split(strings.ReplaceAll(new_data, "\r", ""), "\n")

	result := []string{}
	result = append(result, lines[:start_line]...)
	result = append(result, new_lines...)
	result = append(result, lines[end_line:]...)

	return strings.Join(result, "\n"), true
}

// Section_edit_able : 문단 편집을 지원하는 문서인지 확인
func Section_edit_able(raw_data string) bool {
	lower_data := strings.ToLower(raw_data)

	// include 로 다른 문서를 불러오면 문단 번호가 달라질 수 있어 문단 편집을 막는다
	if strings.Contains(lower_data, "[include(") {
		return false
	}

	return len(Section_list(raw_data)) > 0
}
