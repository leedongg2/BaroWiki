package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  편집방 작업 공간  /baro/cooperation/<이름표>/work
//
//  방에 문서를 열어 두고 여럿이 함께 손본다.
//  열어 둔 문서는 방에 있는 모두에게 탭으로 보인다.
//
//  문서마다 ACL 이 다르므로, 같은 탭이라도 사람마다 다르게 보인다.
//    · 편집까지 되는 사람 : 편집 칸이 열린다
//    · 편집만 막힌 사람   : 본문은 보이고 「편집할 수 없습니다」 알림이 뜬다
//    · 열람도 막힌 사람   : 본문을 감추고 「열람할 수 없습니다」 알림만 뜬다
//
//  저장은 여느 편집과 똑같은 길(Api_edit_post)을 탄다.
//  그래서 편집 필터 · 판 충돌 · 같은 내용 검사 · 역사 기록이 모두 그대로 걸린다.
// ============================================================

// baro_coop_work_acl : 이 사람이 그 문서를 어디까지 할 수 있는가
type baro_coop_work_acl struct {
	CanView  bool
	CanEdit  bool
	ViewNeed string // 열람에 필요한 자리
	EditNeed string // 편집에 필요한 자리
}

// baro_coop_work_acl_of : 문서 하나에 대한 이 사람의 자리
func baro_coop_work_acl_of(db *sql.DB, doc_name string, ip string) baro_coop_work_acl {
	return baro_coop_work_acl{
		CanView:  tool.Check_acl(db, doc_name, "", "render", ip),
		CanEdit:  tool.Check_acl(db, doc_name, "", "document_edit", ip),
		ViewNeed: baro_coop_acl_need(db, doc_name, "view"),
		EditNeed: baro_coop_acl_need(db, doc_name, "document_edit_acl"),
	}
}

// baro_coop_acl_need : 그 문서를 다루려면 무엇이어야 하는지 사람 말로
//
//	규칙으로 적힌 ACL 은 줄마다 풀어 쓰고, 옛 방식으로 한 값만 적힌 것은 그 이름을 쓴다.
func baro_coop_acl_need(db *sql.DB, doc_name string, acl_type string) string {
	acl_data := tool.Get_document_acl(db, doc_name, acl_type)

	if strings.TrimSpace(acl_data) == "" || acl_data == "normal" {
		return tool.Baro_t("기본값")
	}

	rules := tool.Acl_rule_parse(acl_data)
	if len(rules) == 0 {
		return namespace_acl_value_label(acl_data)
	}

	// 한 줄짜리 옛 값은 규칙 하나로 읽히므로 그대로 이름만 보여 준다.
	if len(rules) == 1 && rules[0].Allow && rules[0].Value == "" {
		return namespace_acl_value_label(rules[0].Kind)
	}

	parts := []string{}
	for _, rule := range rules {
		head := tool.Baro_t("허용")
		if !rule.Allow {
			head = tool.Baro_t("거부")
		}

		body := namespace_acl_value_label(rule.Kind)
		if rule.Value != "" {
			body = rule.Kind + ":" + rule.Value
		}

		parts = append(parts, head+" "+body)
	}

	return strings.Join(parts, ", ")
}

// View_baro_coop_work : 작업 공간
func View_baro_coop_work(config tool.Config, link string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	room, found := tool.Baro_coop_room_get(db, strings.TrimSpace(link))
	if !found {
		return baro_coop_back("/baro/cooperation", tool.Baro_t("그런 편집방이 없습니다."), "danger")
	}

	if !tool.Baro_coop_can_view(db, config.IP) {
		return baro_coop_shut(db, config, tool.Baro_t("편집방을 열람할 수 있는 권한이 없습니다."))
	}

	member, in_room := tool.Baro_coop_member_get(db, room.ID, config.IP)
	is_in := in_room && member.Status == "in"

	if room.OpenKind == tool.Baro_coop_private && !in_room && !tool.Baro_can(db, config.IP, "system") {
		return baro_coop_shut(db, config,
			tool.Baro_t("열람이 제한된 방입니다.")+" "+
				tool.Baro_t("비공개 편집방은 방장이 불러 준 사람만 볼 수 있습니다."))
	}

	// 처리
	if values != nil && values.Get("do") != "" {
		if out := baro_coop_work_do(db, config, room, member, is_in, values); out != "" {
			return out
		}
	}

	room, _ = tool.Baro_coop_room_get(db, link)

	base := baro_coop_url(room.Link)
	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"}, {"/baro/cooperation", tool.Baro_t("편집방")},
		{base, room.Name}, {"", tool.Baro_t("작업")},
	})
	body += tool.Baro_hero(tool.Baro_t("작업 공간"), room.Name, "")
	body += baro_coop_css()
	body += baro_coop_work_css()
	body += baro_coop_alert(values)
	body += baro_coop_state_alert(db, room)

	body += tool.Baro_tabs([][]string{
		{base, tool.Baro_t("방")},
		{base + "/work", tool.Baro_t("작업")},
		{base + "/talk", tool.Baro_t("댓글")},
		{base + "/detail", tool.Baro_t("상세")},
	}, base+"/work")

	if !is_in {
		body += tool.Baro_note(tool.Baro_t("방에 들어와야 함께 손볼 수 있습니다. 지금은 열린 문서만 볼 수 있습니다."))
	}

	// --- 탭 줄 ---
	open_list := tool.Baro_coop_doc_open_list(db, room.ID)
	now := ""
	if values != nil {
		now = strings.TrimSpace(values.Get("doc"))
	}

	if now == "" && len(open_list) > 0 {
		now = open_list[0].Title
	}

	body += baro_coop_work_tabs(db, config, room, open_list, now, base)

	// --- 문서 추가 ---
	if tool.Baro_coop_can(db, room, config.IP, "doc_add") && room.Baro_coop_room_live() {
		form := tool.Baro_hidden("do", "doc_add")
		form += tool.Baro_input("title", tool.Baro_t("문서 제목"), "", tool.Baro_t("예: 홍길동"),
			tool.Baro_t("없는 문서 이름을 적으면 새 문서를 만듭니다."))
		form += tool.Baro_submit(tool.Baro_t("문서 추가"), "primary")

		body += tool.Baro_card(tool.Baro_t("문서 추가"), tool.Baro_form(base+"/work", form))
	}

	// --- 편집 칸 ---
	if now != "" {
		body += baro_coop_work_panel(db, config, room, member, is_in, now, base)
	} else {
		body += tool.Baro_card("", tool.Baro_empty(tool.Baro_t("열어 둔 문서가 없습니다. 위에서 문서를 추가해 주세요.")))
	}

	return tool.Baro_page(db, config, tool.Baro_t("작업 공간"), body, baro_coop_menu())
}

// baro_coop_work_tabs : VS Code 처럼 늘어놓는 문서 탭 줄
func baro_coop_work_tabs(
	db *sql.DB, config tool.Config, room tool.Baro_coop_room_row,
	open_list []tool.Baro_coop_doc_row, now string, base string,
) string {
	if len(open_list) == 0 {
		return ""
	}

	can_close := tool.Baro_coop_can(db, room, config.IP, "doc_close")

	html := `<div class="bw_work_tabs">`
	for _, one := range open_list {
		acl := baro_coop_work_acl_of(db, one.Title, config.IP)

		mark := ""
		switch {
		case !acl.CanView:
			mark = ` <span class="bw_work_lock">` + tool.Baro_t_esc("열람 불가") + `</span>`
		case !acl.CanEdit:
			mark = ` <span class="bw_work_lock">` + tool.Baro_t_esc("편집 불가") + `</span>`
		}

		on := ""
		if one.Title == now {
			on = " bw_work_tab_on"
		}

		html += `<span class="bw_work_tab` + on + `">`
		html += `<a href="` + base + `/work?doc=` + tool.Url_parser(one.Title) + `">` +
			tool.HTML_escape(tool.Namespace_display(one.Title)) + `</a>` + mark

		if can_close {
			html += `<form class="bw_work_x" method="post" action="` + base + `/work">` +
				tool.Baro_hidden("do", "doc_close") + tool.Baro_hidden("title", one.Title) +
				`<button type="submit" class="bw_work_x_btn" title="` + tool.Baro_t_esc("문서 닫기") + `">&times;</button>` +
				`</form>`
		}

		html += `</span>`
	}

	return html + `</div>`
}

// baro_coop_work_panel : 고른 문서의 편집 칸
func baro_coop_work_panel(
	db *sql.DB, config tool.Config, room tool.Baro_coop_room_row,
	member tool.Baro_coop_member_row, is_in bool, doc_name string, base string,
) string {
	acl := baro_coop_work_acl_of(db, doc_name, config.IP)
	shown := tool.Namespace_display(doc_name)

	// --- 열람도 막힌 사람 ---
	//     관전 유저에게도 똑같이 띄운다. 본문은 한 글자도 내보내지 않는다.
	if !acl.CanView {
		return tool.Baro_card(shown,
			tool.Baro_alert("danger",
				"[ "+tool.Baro_t("알림")+" ] "+tool.Baro_t("해당 문서는 열람할 수 없습니다.")+" "+
					acl.ViewNeed+tool.Baro_t("여야 합니다.")))
	}

	body, revision := baro_coop_work_read(db, doc_name)

	head := ""
	if revision == "0" {
		head += tool.Baro_alert("info", tool.Baro_t("아직 없는 문서입니다. 저장하면 새로 만들어집니다."))
	}

	// --- 편집만 막힌 사람 ---
	if !acl.CanEdit {
		head += tool.Baro_alert("warn",
			"[ "+tool.Baro_t("알림")+" ] "+tool.Baro_t("해당 문서는 편집할 수 없습니다.")+" "+
				acl.EditNeed+tool.Baro_t("여야 합니다."))

		return tool.Baro_card(shown, head+
			`<textarea class="bw_in bw_work_box" readonly>`+tool.HTML_escape(body)+`</textarea>`+
			baro_coop_work_link(doc_name))
	}

	// --- 방에서 손볼 수 있는 사람인가 ---
	switch {
	case !is_in:
		head += tool.Baro_note(tool.Baro_t("방에 들어와야 여기서 고칠 수 있습니다."))
	case member.Block:
		head += tool.Baro_alert("warn", tool.Baro_t("방장이 이 방에서의 편집을 제한했습니다."))
	case !member.Baro_coop_member_editing():
		head += tool.Baro_note(tool.Baro_t("관전 유저입니다. 편집 유저로 바꾸면 여기서 고칠 수 있습니다."))
	case !room.Baro_coop_room_live():
		head += tool.Baro_note(tool.Baro_t("닫힌 방입니다. 여기서는 더 고칠 수 없습니다."))
	default:
		return tool.Baro_card(shown, head+baro_coop_work_form(db, doc_name, body, revision, base))
	}

	return tool.Baro_card(shown, head+
		`<textarea class="bw_in bw_work_box" readonly>`+tool.HTML_escape(body)+`</textarea>`+
		baro_coop_work_link(doc_name))
}

// baro_coop_work_form : 실제로 고치는 칸
func baro_coop_work_form(db *sql.DB, doc_name string, body string, revision string, base string) string {
	form := tool.Baro_hidden("do", "doc_save")
	form += tool.Baro_hidden("title", doc_name)
	form += tool.Baro_hidden("ver", revision)

	form += `<textarea class="bw_in bw_work_box" id="bw_work_text" name="content">` +
		tool.HTML_escape(body) + `</textarea>`
	form += `<textarea class="bw_work_seed" id="bw_work_seed" readonly>` + tool.HTML_escape(body) + `</textarea>`

	form += tool.Baro_input("send", tool.Baro_t("편집 요약"), "", tool.Baro_t("무엇을 왜 고쳤는지"), "")
	form += tool.Baro_checkbox(tool.Baro_ai_field, tool.Baro_ai_text(), "yes", false, tool.Baro_ai_help())
	form += tool.Baro_checkbox("agree", tool.Baro_t("기여한 내용을 위키 라이선스로 내놓는 데 동의합니다"), "on", true, "")
	form += tool.Baro_submit(tool.Baro_t("저장"), "primary")

	inner := `<form class="bw_form" id="bw_work_form" method="post" action="` + base + `/work">` + form + `</form>`

	return inner + baro_coop_work_link(doc_name) + baro_coop_work_script()
}

// baro_coop_work_link : 문서로 바로 가는 길
func baro_coop_work_link(doc_name string) string {
	return `<p class="bw_work_go"><a href="/w/` + tool.Doc_url(doc_name) + `">` +
		tool.Baro_t_esc("문서 보기") + `</a> · <a href="/history/` + tool.Url_parser(doc_name) + `">` +
		tool.Baro_t_esc("역사") + `</a></p>`
}

// baro_coop_work_read : 문서 본문과 판 번호
func baro_coop_work_read(db *sql.DB, doc_name string) (string, string) {
	body := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&body}, doc_name)

	return body, tool.Get_document_revision(db, doc_name)
}

// baro_coop_work_css : 작업 공간 모양
func baro_coop_work_css() string {
	return `<style id="bw_work_css">
.bw_work_tabs { display: flex; flex-wrap: wrap; gap: 2px; margin: 10px 0;
    border-bottom: 1px solid var(--namu-card-border-color, #ddd); }
.bw_work_tab { display: inline-flex; align-items: center; gap: 4px; padding: 6px 10px;
    border: 1px solid var(--namu-card-border-color, #ddd); border-bottom: 0;
    border-radius: 6px 6px 0 0; background: var(--namu-ground-color, #f5f5f5); font-size: 13px; }
.bw_work_tab_on { background: var(--namu-background-color, #fff); font-weight: 600; }
.bw_work_tab a { text-decoration: none; }
.bw_work_lock { font-size: 11px; opacity: 0.8; color: var(--namu-length-minus-color, #f00); }
.bw_work_x { display: inline; margin: 0; }
.bw_work_x_btn { border: 0; background: none; cursor: pointer; font-size: 15px; line-height: 1;
    padding: 0 2px; opacity: 0.6; }
.bw_work_x_btn:hover { opacity: 1; }
.bw_work_box { display: block; width: 100%; min-height: 24rem; font-family: var(--namu-font-family-mono, monospace); }
.bw_work_seed { display: none; }
.bw_work_go { font-size: 13px; margin: 8px 0 0; }
.bw_work_ask { position: fixed; inset: 0; z-index: 60; display: flex; align-items: center;
    justify-content: center; background: rgba(0,0,0,0.45); }
.bw_work_ask_in { background: var(--namu-card-background-color, #fff); color: var(--namu-text-color, #212529);
    border-radius: 8px; padding: 18px 20px; max-width: 28rem; }
.bw_work_ask_in p { margin: 0 0 14px; }
.bw_work_ask_in button { margin-right: 6px; }
</style>`
}

// baro_coop_work_script : 저장하지 않고 닫으려 할 때 물어보기
//
//	여느 문서 편집과 같은 물음을 띄운다.
//	  저장이 안 됐습니다. 정말 문서를 닫으시겠습니까?
//	  [ 저장하고 문서 닫기 ] [ 저장 안 하고 문서 닫기 ] [ 취소 ]
func baro_coop_work_script() string {
	return `<script>
(function() {
    var box = document.getElementById('bw_work_text');
    var seed = document.getElementById('bw_work_seed');
    var form = document.getElementById('bw_work_form');
    if(box === null || seed === null || form === null) { return; }

    var saving = false;

    function changed() { return box.value !== seed.value; }

    form.addEventListener('submit', function(e) {
        // 바뀐 것이 없으면 보내지 않는다. 서버도 같은 것을 한 번 더 본다.
        if(!changed()) {
            e.preventDefault();
            alert('문서 내용이 같습니다. 바꾼 내용이 없으면 저장할 수 없습니다.');
            return;
        }

        saving = true;
    });

    // 탭의 X 와 다른 문서로 옮겨 가는 링크를 가로채 물어본다
    function ask(run) {
        var wrap = document.createElement('div');
        wrap.className = 'bw_work_ask';
        wrap.innerHTML = '<div class="bw_work_ask_in"><p>저장이 안 됐습니다. 정말 문서를 닫으시겠습니까?</p>' +
            '<button type="button" id="bw_work_ask_save">저장하고 문서 닫기</button>' +
            '<button type="button" id="bw_work_ask_drop">저장 안 하고 문서 닫기</button>' +
            '<button type="button" id="bw_work_ask_no">취소</button></div>';
        document.body.appendChild(wrap);

        document.getElementById('bw_work_ask_save').onclick = function() {
            // 저장한 뒤에 닫도록, 저장 폼에 「닫고 나서」 표시를 달아 보낸다.
            var after = document.createElement('input');
            after.type = 'hidden';
            after.name = 'after';
            after.value = 'close';
            form.appendChild(after);

            saving = true;
            form.submit();
        };

        document.getElementById('bw_work_ask_drop').onclick = function() {
            seed.value = box.value;
            wrap.remove();
            run();
        };

        document.getElementById('bw_work_ask_no').onclick = function() { wrap.remove(); };
    }

    var forms = document.querySelectorAll('.bw_work_x');
    for(var i = 0; i < forms.length; i++) {
        (function(one) {
            one.addEventListener('submit', function(e) {
                if(!changed()) { return; }

                e.preventDefault();
                ask(function() { one.submit(); });
            });
        })(forms[i]);
    }

    var links = document.querySelectorAll('.bw_work_tab a');
    for(var i = 0; i < links.length; i++) {
        (function(one) {
            one.addEventListener('click', function(e) {
                if(!changed()) { return; }

                e.preventDefault();
                ask(function() { location.href = one.href; });
            });
        })(links[i]);
    }

    window.addEventListener('beforeunload', function(e) {
        if(saving || !changed()) { return; }

        e.preventDefault();
        e.returnValue = '';
    });
})();
</script>`
}

// baro_coop_work_do : 작업 공간에서 온 처리
func baro_coop_work_do(
	db *sql.DB, config tool.Config, room tool.Baro_coop_room_row,
	member tool.Baro_coop_member_row, is_in bool, values url.Values,
) string {
	base := baro_coop_url(room.Link) + "/work"

	if !tool.Baro_is_user(config.IP) {
		return baro_coop_back(base, tool.Baro_t("로그인이 필요합니다."), "warn")
	}
	if tool.Baro_read_only(db) {
		return baro_coop_back(base, tool.Baro_t("지금은 읽기 전용입니다."), "warn")
	}

	title := tool.Namespace_normalize(strings.TrimSpace(values.Get("title")))

	switch values.Get("do") {
	// --- 문서 추가 ---
	case "doc_add":
		if !tool.Baro_coop_can(db, room, config.IP, "doc_add") {
			return baro_coop_back(base, tool.Baro_t("문서를 추가할 수 있는 자리가 아닙니다."), "warn")
		}
		if !room.Baro_coop_room_live() {
			return baro_coop_back(base, tool.Baro_t("닫힌 방입니다."), "warn")
		}
		if title == "" {
			return baro_coop_back(base, tool.Baro_t("문서 제목을 적어 주세요."), "warn")
		}
		if !tool.Do_title_length_check(db, title, "document") {
			return baro_coop_back(base, tool.Baro_t("문서 제목이 너무 깁니다."), "warn")
		}

		tool.Baro_coop_doc_open(db, room.ID, title, config.IP)
		tool.Baro_coop_log_add(db, room.ID, "doc_open", config.IP, title, "")

		return baro_coop_back(base+"?doc="+tool.Url_parser(title),
			tool.Namespace_display(title)+tool.Baro_t(" 문서를 열었습니다."), "ok")

	// --- 문서 닫기 ---
	case "doc_close":
		if !tool.Baro_coop_can(db, room, config.IP, "doc_close") {
			return baro_coop_back(base, tool.Baro_t("문서를 닫을 수 있는 자리가 아닙니다."), "warn")
		}
		if title == "" {
			return baro_coop_back(base, tool.Baro_t("닫을 문서를 고르지 못했습니다."), "warn")
		}

		tool.Baro_coop_doc_close(db, room.ID, title)
		tool.Baro_coop_log_add(db, room.ID, "doc_close", config.IP, title, "")

		return baro_coop_back(base, tool.Namespace_display(title)+tool.Baro_t(" 문서를 닫았습니다."), "ok")

	// --- 저장 ---
	case "doc_save":
		return baro_coop_work_save(db, config, room, member, is_in, title, values, base)
	}

	return ""
}

// baro_coop_work_save : 작업 공간에서 저장하기
//
//	저장 자체는 여느 편집과 같은 길을 탄다. 방 쪽 조건만 여기서 먼저 본다.
func baro_coop_work_save(
	db *sql.DB, config tool.Config, room tool.Baro_coop_room_row,
	member tool.Baro_coop_member_row, is_in bool, title string, values url.Values, base string,
) string {
	back := base + "?doc=" + tool.Url_parser(title)

	switch {
	case title == "":
		return baro_coop_back(base, tool.Baro_t("어느 문서인지 알 수 없습니다."), "warn")
	case !is_in:
		return baro_coop_back(back, tool.Baro_t("방에 들어와야 고칠 수 있습니다."), "warn")
	case member.Block:
		return baro_coop_back(back, tool.Baro_t("방장이 이 방에서의 편집을 제한했습니다."), "warn")
	case !member.Baro_coop_member_editing():
		return baro_coop_back(back, tool.Baro_t("관전 유저는 고칠 수 없습니다."), "warn")
	case !room.Baro_coop_room_live():
		return baro_coop_back(back, tool.Baro_t("닫힌 방입니다."), "warn")
	}

	// 문서 자체의 ACL 은 아래 Api_edit_post 가 다시 본다.
	// 여기서 먼저 보는 것은 무엇이 모자란지 사람 말로 알려 주기 위해서다.
	acl := baro_coop_work_acl_of(db, title, config.IP)

	if !acl.CanView {
		return baro_coop_back(back,
			"[ "+tool.Baro_t("알림")+" ] "+tool.Baro_t("해당 문서는 열람할 수 없습니다.")+" "+
				acl.ViewNeed+tool.Baro_t("여야 합니다."), "danger")
	}

	if !acl.CanEdit {
		return baro_coop_back(back,
			"[ "+tool.Baro_t("알림")+" ] "+tool.Baro_t("해당 문서는 편집할 수 없습니다.")+" "+
				acl.EditNeed+tool.Baro_t("여야 합니다."), "danger")
	}

	send := tool.Baro_ai_send(values.Get("send"), tool.Baro_ai_on(values.Get(tool.Baro_ai_field)))

	api := Api_edit_post(config, title, values.Get("content"), send, values.Get("agree"), values.Get("ver"))

	response, _ := api["response"].(string)
	if response == "ok" {
		// 저장한 뒤에 닫기로 했다면 여기서 닫는다.
		if values.Get("after") == "close" && tool.Baro_coop_can(db, room, config.IP, "doc_close") {
			tool.Baro_coop_doc_close(db, room.ID, title)
			tool.Baro_coop_log_add(db, room.ID, "doc_close", config.IP, title, "저장한 뒤 닫음")

			return baro_coop_back(base, tool.Baro_t("저장하고 문서를 닫았습니다."), "ok")
		}

		return baro_coop_back(back, tool.Baro_t("저장했습니다."), "ok")
	}

	kind, _ := api["data"].(string)

	return baro_coop_back(back, baro_coop_work_why(kind), "danger")
}

// baro_coop_work_why : 저장이 막힌 까닭을 사람 말로
func baro_coop_work_why(kind string) string {
	switch kind {
	case "same content":
		return tool.Baro_t("문서 내용이 같습니다. 바꾼 내용이 없으면 저장할 수 없습니다.")
	case "permission denied":
		return tool.Baro_t("이 문서를 편집할 권한이 없습니다.")
	case "edit conflict":
		return tool.Baro_t("그 사이에 다른 사람이 이 문서를 고쳤습니다. 다시 열어 확인해 주세요.")
	case "slow edit limit":
		return tool.Baro_t("잠시 뒤에 다시 저장해 주세요.")
	case "send require":
		return tool.Baro_t("편집 요약을 적어 주세요.")
	case "checkbox check require":
		return tool.Baro_t("라이선스 동의에 표시해 주세요.")
	case "overflow max length":
		return tool.Baro_t("문서가 너무 깁니다.")
	case "title length":
		return tool.Baro_t("문서 제목이 너무 깁니다.")
	case "empty title":
		return tool.Baro_t("문서 제목이 비어 있습니다.")
	}

	if strings.HasPrefix(kind, "edit filter") {
		return tool.Baro_t("편집 필터에 걸렸습니다.")
	}

	if reason := strings.TrimPrefix(kind, "baro:"); reason != kind {
		return reason
	}

	return tool.Baro_t("저장하지 못했습니다.") + " (" + kind + ")"
}
