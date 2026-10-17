package route

import (
	"database/sql"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  ACL 규칙 편집 화면
//
//  예전에는 항목마다 조건을 하나만 고르는 선택 상자였다.
//  이제는 규칙을 여러 줄 적는 칸이 된다.
//
//    허용:perm:admin
//    거부:ip:192.168.0.0/16
//    허용:user:홍길동
//    거부:any
//
//  줄을 비워 두면 예전과 같이 이름공간 ACL 로 넘어간다.
//  옛 값(`admin` 한 줄)이 들어 있어도 그대로 열리고, 그대로 움직인다.
//
//  고르기 쉬우라고 선택 상자를 함께 두어, 고르면 칸에 한 줄이 붙는다.
// ============================================================

// acl_rule_editor : 규칙을 적는 칸 하나
func acl_rule_editor(db *sql.DB, field string, value string) string {
	area_id := "acl_rule_" + field

	data := `<textarea class="namu_input opennamu_acl_rule" id="` + area_id + `" name="` + tool.HTML_escape(field) + `" rows="4" spellcheck="false"` +
		` placeholder="` + tool.HTML_escape(tool.Baro_t("비워 두면 이름공간 설정을 따릅니다")) + `">` + tool.HTML_escape(value) + `</textarea>`

	data += `<div class="opennamu_acl_rule_add">`
	data += `<select id="` + area_id + `_allow">` +
		`<option value="허용">` + tool.HTML_escape(tool.Baro_t("허용")) + `</option>` +
		`<option value="거부">` + tool.HTML_escape(tool.Baro_t("거부")) + `</option>` +
		`</select>`

	data += `<select id="` + area_id + `_kind">`
	for _, one := range acl_rule_kind_options(db) {
		data += `<option value="` + tool.HTML_escape(one[0]) + `">` + tool.HTML_escape(one[1]) + `</option>`
	}
	data += `</select>`

	data += `<input id="` + area_id + `_value" class="namu_input" placeholder="` + tool.HTML_escape(tool.Baro_t("값")) + `">`
	data += `<button type="button" onclick="opennamu_acl_rule_add('` + tool.JS_escape(area_id) + `');">` +
		tool.HTML_escape(tool.Baro_t("줄 추가")) + `</button>`
	data += `</div>`

	return data
}

// acl_rule_kind_options : 고를 수 있는 조건 갈래 {값, 보여 줄 이름}
//
//	새 갈래를 먼저 두고, 예전부터 쓰던 조건을 뒤에 둔다.
func acl_rule_kind_options(db *sql.DB) [][2]string {
	list := [][2]string{
		{"any", tool.Baro_t("모두")},
		{"member", tool.Baro_t("로그인 사용자")},
		{"perm", tool.Baro_t("권한 (perm:admin)")},
		{"user", tool.Baro_t("특정 계정 (user:이름)")},
		{"ip", tool.Baro_t("아이피 · 대역 (ip:192.168.0.0/16)")},
		{"aclgroup", tool.Baro_t("ACL 그룹 (aclgroup:이름)")},
		{"geoip", tool.Baro_t("나라 (geoip:KR)")},
	}

	for _, one := range tool.List_acl("normal") {
		if one == "" || one == "all" || one == "user" {
			continue
		}

		list = append(list, [2]string{one, namespace_acl_value_label(one)})
	}

	return list
}

// acl_rule_help : 규칙 적는 법 안내
func acl_rule_help(db *sql.DB) string {
	rows := [][]string{
		{"허용:any", tool.Baro_t("모두 허용")},
		{"거부:any", tool.Baro_t("모두 거부")},
		{"허용:member", tool.Baro_t("로그인한 사용자")},
		{"허용:perm:admin", tool.Baro_t("관리자 권한을 가진 사람")},
		{"허용:user:홍길동", tool.Baro_t("그 계정 하나")},
		{"거부:ip:192.168.0.1", tool.Baro_t("그 아이피 하나")},
		{"거부:ip:192.168.0.0/16", tool.Baro_t("그 대역 전체")},
		{"거부:aclgroup:장난 대역", tool.Baro_t("ACL 그룹에 든 사람")},
		{"허용:geoip:KR", tool.Baro_t("대한민국에서 접속한 사람")},
		{"거부:geoip:CN,RU", tool.Baro_t("여러 나라는 쉼표로")},
	}

	body := `<ul class="opennamu_acl_rule_help">`
	for _, one := range rows {
		body += `<li><code>` + tool.HTML_escape(one[0]) + `</code> — ` + tool.HTML_escape(one[1]) + `</li>`
	}
	body += `</ul>`

	note := tool.Baro_t("위에서부터 훑어 처음 걸린 줄이 결론입니다. 아무 줄도 걸리지 않으면 거부합니다.") + " " +
		tool.Baro_t("칸을 비우면 이름공간 설정을 따릅니다.")

	return `<details class="opennamu_acl_rule_box"><summary>` + tool.HTML_escape(tool.Baro_t("규칙 적는 법")) + `</summary>` +
		`<p>` + tool.HTML_escape(note) + `</p>` + body + `</details>` + acl_rule_script()
}

// acl_rule_script : 「줄 추가」 단추가 하는 일
func acl_rule_script() string {
	return `<script>
function opennamu_acl_rule_add(area_id) {
    var area = document.getElementById(area_id);
    var allow = document.getElementById(area_id + '_allow');
    var kind = document.getElementById(area_id + '_kind');
    var value = document.getElementById(area_id + '_value');
    if(area === null || allow === null || kind === null) { return; }

    var line = allow.value + ':' + kind.value;
    if(value !== null && value.value.trim() !== '') {
        line += ':' + value.value.trim();
    }

    if(area.value.trim() === '') {
        area.value = line;
    } else {
        area.value = area.value.replace(/\s+$/, '') + '\n' + line;
    }

    if(value !== null) { value.value = ''; }
}
</script>`
}

// acl_rule_summary : 규칙 목록을 한 줄로 간추린다 (목록 화면에서 쓴다)
func acl_rule_summary(value string) string {
	rules := tool.Acl_rule_parse(value)
	if len(rules) == 0 {
		return namespace_acl_value_label(value)
	}
	if len(rules) == 1 {
		return tool.Acl_rule_line(rules[0])
	}

	parts := []string{}
	for _, one := range rules {
		parts = append(parts, tool.Acl_rule_line(one))
	}

	return strings.Join(parts, " / ")
}
