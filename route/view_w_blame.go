package route

import (
	"database/sql"
	"strconv"
	"strings"

	"opennamu/route/tool"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type blame_line struct {
	rev    string
	prev   string
	user   string
	date   string
	hidden bool
	data   string
}

func blame_split_line(data string) []string {
	if data == "" {
		return []string{}
	}

	line_list := strings.SplitAfter(data, "\n")
	if line_list[len(line_list)-1] == "" {
		line_list = line_list[:len(line_list)-1]
	}

	for index := range line_list {
		line_list[index] = strings.TrimSuffix(line_list[index], "\n")
	}

	return line_list
}

func blame_merge(diff_tool *diffmatchpatch.DiffMatchPatch, before_data string, after_data string, before_blame []blame_line, now blame_line) []blame_line {
	before_char, after_char, line_array := diff_tool.DiffLinesToChars(before_data, after_data)
	diff_list := diff_tool.DiffCharsToLines(diff_tool.DiffMain(before_char, after_char, false), line_array)

	after_blame := []blame_line{}
	index := 0

	for _, diff_data := range diff_list {
		line_list := blame_split_line(diff_data.Text)

		switch diff_data.Type {
		case diffmatchpatch.DiffEqual:
			for _, line := range line_list {
				if index < len(before_blame) {
					after_blame = append(after_blame, before_blame[index])
				} else {
					now.data = line
					after_blame = append(after_blame, now)
				}

				index += 1
			}
		case diffmatchpatch.DiffDelete:
			index += len(line_list)
		case diffmatchpatch.DiffInsert:
			for _, line := range line_list {
				now.data = line
				after_blame = append(after_blame, now)
			}
		}
	}

	return after_blame
}

func Get_blame_line_list(db *sql.DB, config tool.Config, doc_name string, rev int) ([]blame_line, bool) {
	hidden_auth := tool.Check_acl(db, "", "", "hidel_auth", config.IP)

	rows := tool.Query_DB(
		db,
		"select id, data, date, ip, hide from history where title = ? and id + 0 > 0 and id + 0 <= ? order by id + 0 asc",
		doc_name,
		rev,
	)
	defer rows.Close()

	diff_tool := diffmatchpatch.New()
	blame_list := []blame_line{}
	before_data := ""

	// 바로 앞 판의 번호. 판이 지워져 번호에 구멍이 나도 (예: 1 · 2 · 4)
	// "r-1" 로 어림잡지 않고 실제로 남아 있는 앞 판을 가리키게 한다.
	before_rev := ""
	exist := false

	for rows.Next() {
		var id string
		var data string
		var date string
		var ip string
		var hide string

		if rows.Scan(&id, &data, &date, &ip, &hide) != nil {
			continue
		}

		exist = true
		data = strings.ReplaceAll(data, "\r", "")

		now := blame_line{
			rev:    id,
			prev:   before_rev,
			user:   tool.IP_parser(db, ip, config.IP),
			date:   date,
			hidden: hide != "" && !hidden_auth,
		}

		blame_list = blame_merge(diff_tool, before_data, data, blame_list, now)
		before_data = data
		before_rev = id
	}

	return blame_list, exist
}

func View_w_blame(config tool.Config, doc_name string, rev string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Check_acl(db, doc_name, "", "render", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	doc_name_url := tool.Url_parser(doc_name)
	revision, revision_number, ok := history_tool_revision(rev)
	if !ok || revision_number < 1 || doc_name == "" {
		return tool.Get_redirect("/history/" + doc_name_url)
	}

	hide := ""
	if !tool.QueryRow_DB(db, "select hide from history where title = ? and id = ?", []any{&hide}, doc_name, revision) {
		return tool.Get_redirect("/history/" + doc_name_url)
	}

	if hide != "" && !tool.Check_acl(db, "", "", "hidel_auth", config.IP) {
		return tool.Get_error_page(db, config, "auth")
	}

	blame_list, exist := Get_blame_line_list(db, config, doc_name, revision_number)
	if !exist {
		return tool.Get_redirect("/history/" + doc_name_url)
	}

	// 줄마다 조각을 이어 붙이면 문서가 길어질수록 제곱으로 느려진다.
	// (2만 줄 기준 25초) 그래서 버퍼에 한 번에 쌓는다.
	var builder strings.Builder
	builder.Grow(len(blame_list)*256 + 256)

	builder.WriteString(`<table style="width: 100%; white-space: pre-wrap;">`)
	builder.WriteString(`<tr><td colspan="3">r`)
	builder.WriteString(tool.HTML_escape(revision))
	builder.WriteString(`</td></tr>`)

	for index, line_data := range blame_list {
		line_html := tool.HTML_escape(line_data.data)
		if strings.TrimSpace(line_data.data) == "" {
			line_html = "&nbsp;"
		}

		builder.WriteString(`<tr>
            <td style="width: 40px; user-select: none;">`)
		builder.WriteString(strconv.Itoa(index + 1))
		builder.WriteString(`</td>
            <td style="width: 180px; user-select: none;">`)

		if line_data.hidden {
			builder.WriteString(`<span style="color: gray;">r?</span>`)
		} else {
			line_url := "/render/" + line_data.rev + "/" + doc_name_url
			if line_data.prev != "" {
				line_url = "/diff/" + line_data.prev + "/" + line_data.rev + "/" + doc_name_url
			}

			builder.WriteString(`<a href="`)
			builder.WriteString(line_url)
			builder.WriteString(`" title="`)
			builder.WriteString(tool.HTML_escape(line_data.date))
			builder.WriteString(`">r`)
			builder.WriteString(tool.HTML_escape(line_data.rev))
			builder.WriteString(`</a> `)
			builder.WriteString(line_data.user)
		}

		builder.WriteString(`</td>
            <td>`)
		builder.WriteString(line_html)
		builder.WriteString(`</td>
        </tr>`)
	}

	builder.WriteString(`</table>`)

	return tool.Get_template(
		db,
		config,
		doc_name,
		builder.String(),
		[]any{`(Blame r` + revision + `)`},
		[][]any{
			{"history/" + doc_name_url, tool.Get_language(db, "return", true)},
			{"render/" + revision + "/" + doc_name_url, tool.Get_language(db, "view", true)},
		},
		map[string]string{},
	)
}
