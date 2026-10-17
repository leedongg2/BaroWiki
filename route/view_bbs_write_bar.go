package route

import (
	"database/sql"

	"opennamu/route/tool"
)

// ============================================================
//  게시판 글쓰기 줄
//
//  「글 작성이 안 된다」는 말의 절반은 「글쓰기 단추가 어디 있는지 모르겠다」는 뜻이다.
//  글쓰기는 오른쪽 위 메뉴 대신 본문 맨 위에 큰 단추로 둔다.
//
//  - 게시판 메인 : 게시판을 고르는 칸 + 글쓰기 단추
//  - 게시판 안   : 그 게시판으로 바로 가는 글쓰기 단추
//
//  글을 쓸 수 없는 상태(권한 없음 · 잠김 · 차단)면 그 이유를 그 자리에서 알려 준다.
// ============================================================

// bbs_write_deny_reason : 이 게시판에 글을 쓸 수 없는 이유 (쓸 수 있으면 빈 글)
func bbs_write_deny_reason(db *sql.DB, config tool.Config, bbs_id string) string {
	if tool.Get_bbs_lock(db, bbs_id) {
		return tool.Get_language(db, "bbs_locked_notice", true)
	}

	if block_info := tool.Get_bbs_block_info(db, bbs_id, config.IP); block_info.Exist {
		return tool.Get_language(db, "bbs_blocked_notice", true)
	}

	if !tool.Check_acl(db, bbs_id, "", "bbs_edit", config.IP) {
		return tool.Get_language(db, "authority_error", true)
	}

	return ""
}

// bbs_write_button : 게시판 하나로 가는 글쓰기 단추
func bbs_write_button(db *sql.DB, config tool.Config, bbs_id string) string {
	if reason := bbs_write_deny_reason(db, config, bbs_id); reason != "" {
		return `<span class="namu_bbs_write_deny">` + tool.HTML_escape(reason) + `</span>`
	}

	return `<a class="namu_bbs_write" href="/bbs/edit/` + tool.Url_parser(bbs_id) + `">` +
		`<span class="namu_bbs_write_icon" aria-hidden="true">+</span> ` +
		tool.Lang_esc("post_add") + `</a>`
}

// bbs_write_bar : 글쓰기 줄
//
//	bbs_id 가 비어 있으면 게시판을 고르는 칸을 같이 보여 준다.
func bbs_write_bar(db *sql.DB, config tool.Config, bbs_list [][]string, bbs_id string) string {
	if bbs_id != "" {
		return `<div class="namu_bbs_write_bar">` + bbs_write_button(db, config, bbs_id) + `</div>`
	}

	// 글을 쓸 수 있는 게시판만 고르는 칸에 넣는다
	options := ""
	first_id := ""
	for _, one := range bbs_list {
		if len(one) < 2 {
			continue
		}

		name := one[0]
		id := one[1]
		if bbs_write_deny_reason(db, config, id) != "" {
			continue
		}

		if first_id == "" {
			first_id = id
		}
		options += `<option value="` + tool.HTML_escape(id) + `">` + tool.HTML_escape(name) + `</option>`
	}

	if options == "" {
		return `<div class="namu_bbs_write_bar"><span class="namu_bbs_write_deny">` +
			tool.Lang_esc("bbs_no_writable") + `</span></div>`
	}

	return `<div class="namu_bbs_write_bar">` +
		`<span class="__ON_SELECT_DIV__"><select class="__ON_SELECT__" id="opennamu_bbs_write_select" aria-label="` +
		tool.Lang_esc("bbs") + `">` + options + `</select></span> ` +
		`<a class="namu_bbs_write" id="opennamu_bbs_write_link" href="/bbs/edit/` + tool.Url_parser(first_id) + `">` +
		`<span class="namu_bbs_write_icon" aria-hidden="true">+</span> ` + tool.Lang_esc("post_add") + `</a>` +
		`<script>
		(function () {
			var select = document.getElementById('opennamu_bbs_write_select');
			var link = document.getElementById('opennamu_bbs_write_link');
			if (select === null || link === null) { return; }
			var sync = function () { link.href = '/bbs/edit/' + encodeURIComponent(select.value); };
			select.addEventListener('change', sync);
			sync();
		})();
		</script>` +
		`</div>`
}
