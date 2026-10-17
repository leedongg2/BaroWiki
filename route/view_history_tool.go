package route

import (
	"database/sql"
	"sort"
	"strconv"

	"opennamu/route/tool"
)

// history_tool_previous_revision : 실제로 남아 있는 바로 앞 판의 번호를 찾는다.
//
//	판을 지우면 번호에 구멍이 나므로 (예: 1 · 2 · 4) "r - 1" 로 어림잡으면
//	비교 · 되돌리기 링크가 없는 판을 가리켜 역사 목록으로 튕긴다.
//	앞 판이 없으면 빈 문자열을 준다.
func history_tool_previous_revision(db *sql.DB, doc_name string, rev int) string {
	previous := ""
	tool.QueryRow_DB(
		db,
		"select id from history where title = ? and id + 0 > 0 and id + 0 < ? order by id + 0 desc limit 1",
		[]any{&previous},
		doc_name,
		rev,
	)

	return previous
}

// history_rev_entry : 판 번호(정렬용)와 실제로 저장된 판 이름
type history_rev_entry struct {
	num int
	id  string
}

// history_rev_lookup : 한 화면에서 앞 판을 여러 번 묻는 비용을 줄인다.
//
//	최근 변경은 줄마다 문서가 다르고, 역사 목록은 50줄이 모두 같은 문서다.
//	그래서 처음 묻는 문서는 한 줄만 읽고, 같은 문서를 또 물으면 그때서야
//	판 목록을 통째로 한 번 읽어 둔다. 두 화면 모두 조회가 늘지 않는다.
//	한 요청 안에서만 쓰고 버린다.
type history_rev_lookup struct {
	db    *sql.DB
	asked map[string]bool
	cache map[string][]history_rev_entry
}

func new_history_rev_lookup(db *sql.DB) *history_rev_lookup {
	return &history_rev_lookup{
		db:    db,
		asked: map[string]bool{},
		cache: map[string][]history_rev_entry{},
	}
}

func (lookup *history_rev_lookup) load_rev_list(doc_name string) []history_rev_entry {
	data := []history_rev_entry{}
	rows := tool.Query_DB(
		lookup.db,
		"select id from history where title = ? and id + 0 > 0 order by id + 0 asc",
		doc_name,
	)

	for rows.Next() {
		id := ""
		if rows.Scan(&id) != nil {
			continue
		}

		data = append(data, history_rev_entry{num: tool.Str_to_int(id), id: id})
	}
	rows.Close()

	return data
}

func history_rev_previous(data []history_rev_entry, rev int) string {
	index := sort.Search(len(data), func(index int) bool { return data[index].num >= rev })
	if index == 0 {
		return ""
	}
	return data[index-1].id
}

// previous_revision : rev 바로 앞에 실제로 남아 있는 판. 없으면 빈 문자열.
func (lookup *history_rev_lookup) previous_revision(doc_name string, rev int) string {
	if data, ok := lookup.cache[doc_name]; ok {
		return history_rev_previous(data, rev)
	}

	if !lookup.asked[doc_name] {
		lookup.asked[doc_name] = true

		return history_tool_previous_revision(lookup.db, doc_name, rev)
	}

	data := lookup.load_rev_list(doc_name)
	lookup.cache[doc_name] = data

	return history_rev_previous(data, rev)
}

func history_tool_revision(value string) (string, int, bool) {
	revision, err := strconv.Atoi(value)
	if err != nil {
		return "", 0, false
	}
	return strconv.Itoa(revision), revision, true
}

func View_history_tool(config tool.Config, doc_name string, rev string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	revision, revision_number, ok := history_tool_revision(rev)
	if !ok || doc_name == "" {
		return tool.Get_error_page(db, config, "error")
	}

	doc_name_url := tool.Url_parser(doc_name)
	revision_uuid := tool.Baro_rev_uuid(db, doc_name, revision)
	revision_url := func(action string, uuid string, rev string) string {
		if uuid != "" {
			return "/" + action + "/" + doc_name_url + "?uuid=" + tool.Url_parser(uuid)
		}
		if action == "raw" {
			return "/raw_rev/" + rev + "/" + doc_name_url
		}
		return "/" + action + "/" + rev + "/" + doc_name_url
	}
	data := `<h2>` + tool.Get_language(db, "tool", true) + `</h2><ul>`
	data += `<li><a href="` + revision_url("raw", revision_uuid, revision) + `">` + tool.Get_language(db, "raw", true) + `</a></li>`
	data += `<li><a href="` + revision_url("blame", revision_uuid, revision) + `">` + tool.Get_language(db, "blame", true) + `</a></li>`
	data += `<li><a href="` + revision_url("revert", revision_uuid, revision) + `">` + tool.Get_language(db, "revert", true) + ` (r` + revision + `)</a></li>`

	if previous := history_tool_previous_revision(db, doc_name, revision_number); previous != "" {
		previous_uuid := tool.Baro_rev_uuid(db, doc_name, previous)
		data += `<li><a href="` + revision_url("revert", previous_uuid, previous) + `">` + tool.Get_language(db, "revert", true) + ` (r` + previous + `)</a></li>`
		diff_url := "/diff/" + previous + "/" + revision + "/" + doc_name_url
		if revision_uuid != "" {
			diff_url = "/diff/" + doc_name_url + "?uuid=" + tool.Url_parser(revision_uuid)
			if previous_uuid != "" {
				diff_url += "&olduuid=" + tool.Url_parser(previous_uuid)
			}
		}
		data += `<li><a href="` + diff_url + `">` + tool.Get_language(db, "compare", true) + `</a></li>`
	}

	data += `<li><a href="/history/` + doc_name_url + `">` + tool.Get_language(db, "history", true) + `</a></li></ul>`

	// 이 판의 본문을 그대로 집어 가기.
	//   RAW 화면으로 옮겨 가 손으로 긁는 수고를 덜어 준다. 본문은 ACL 을 거쳐 온 것만 담는다.
	data += history_tool_copy_html(config, db, doc_name, revision)

	if tool.Check_acl(db, "", "", "hidel_auth", config.IP) {
		hide := ""
		tool.QueryRow_DB(db, "select hide from history where title = ? and id = ?", []any{&hide}, doc_name, revision)
		hide_name := "hide"
		if hide == "O" {
			hide_name = "hide_release"
		}
		data += `<h3>` + tool.Get_language(db, "admin", true) + `</h3><ul><li><a href="/history_hidden/` + revision + `/` + doc_name_url + `">` + tool.Get_language(db, hide_name, true) + `</a></li></ul>`
	}

	if tool.Check_acl(db, "", "", "owner_auth", config.IP) {
		data += `<h3>` + tool.Get_language(db, "owner", true) + `</h3><ul>`
		data += `<li><a href="/history_delete/` + revision + `/` + doc_name_url + `">` + tool.Get_language(db, "history_delete", true) + `</a></li>`
		data += `<li><a href="/history_send/` + revision + `/` + doc_name_url + `">` + tool.Get_language(db, "send_edit", true) + `</a></li></ul>`
	}

	return tool.Get_template(
		db,
		config,
		doc_name,
		data,
		[]any{`(r` + revision + `)`},
		[][]any{{`history/` + tool.Url_parser(doc_name), tool.Get_language(db, "return", true)}},
		map[string]string{},
	)
}

// history_tool_copy_html : 이 판 내용 복사 칸
//
//	본문을 화면 안에 감춰 두고 단추 하나로 집어 간다.
//	클립보드 API 는 https 나 localhost 에서만 열리므로, 막히면 옛 방식으로 한 번 더 해 보고
//	그것도 안 되면 칸을 펴서 손으로 긁을 수 있게 둔다. (같은 공유기 안 http 접속 대비)
func history_tool_copy_html(config tool.Config, db *sql.DB, doc_name string, revision string) string {
	api := Api_w_raw(config, doc_name, "", revision)

	response, _ := api["response"].(string)
	if response != "ok" {
		return ""
	}

	body, _ := api["data"].(string)

	html := `<h3>` + tool.HTML_escape(tool.Baro_t("이 판 내용 복사")) + `</h3>`
	html += `<p class="opennamu_rev_copy_info">r` + tool.HTML_escape(revision) + ` · ` +
		tool.HTML_escape(tool.Baro_t("글자 수 ")) + tool.HTML_escape(tool.Baro_comma(tool.Get_len(body))) + `</p>`
	html += `<textarea id="opennamu_rev_copy" class="opennamu_rev_copy" readonly>` +
		tool.HTML_escape(body) + `</textarea>`
	html += `<p><button type="button" id="opennamu_rev_copy_button" onclick="opennamu_rev_copy();">` +
		tool.HTML_escape(tool.Baro_t("내용 복사")) + `</button> ` +
		`<span id="opennamu_rev_copy_said"></span></p>`

	return html + history_tool_copy_script()
}

// history_tool_copy_script : 복사 단추가 하는 일
func history_tool_copy_script() string {
	return `<style>
.opennamu_rev_copy { width: 100%; height: 1px; opacity: 0; position: absolute; left: -9999px; }
.opennamu_rev_copy.opennamu_rev_copy_open { position: static; left: auto; height: 16rem; opacity: 1; }
.opennamu_rev_copy_info { font-size: 12px; opacity: 0.75; margin: 0 0 6px; }
#opennamu_rev_copy_said { font-size: 12px; margin-left: 6px; }
</style><script>
function opennamu_rev_copy() {
    var box = document.getElementById('opennamu_rev_copy');
    var said = document.getElementById('opennamu_rev_copy_said');
    if(box === null) { return; }

    function tell(text) { if(said !== null) { said.textContent = text; } }

    function fallback() {
        // 클립보드 API 가 막힌 곳 (https 가 아닌 주소 등)
        box.classList.add('opennamu_rev_copy_open');
        box.removeAttribute('readonly');
        box.focus();
        box.setSelectionRange(0, box.value.length);

        var done = false;
        try { done = document.execCommand('copy'); } catch(e) { done = false; }

        box.setAttribute('readonly', 'readonly');

        if(done) {
            box.classList.remove('opennamu_rev_copy_open');
            tell('복사했습니다.');
        } else {
            tell('복사가 막혀 있습니다. 위 칸의 내용을 직접 긁어 복사해 주세요.');
        }
    }

    if(navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(box.value).then(function() {
            tell('복사했습니다.');
        }, fallback);
        return;
    }

    fallback();
}
</script>`
}
