package route

import (
	"database/sql"
	"net/url"
	"strconv"
	"strings"

	"opennamu/route/tool"
	"opennamu/route/tool/markup"
)

// ============================================================
//  16-3 문서 관리
//
//  0장 원칙 2 : 삭제보다 보존과 복구. 삭제의 기본 경로는 임시 격리다.
//  0장 원칙 3 : 대상이 기준을 넘으면 대량 작업(/baro/mass/new)으로 보낸다.
// ============================================================

// baro_admin_doc_acts : 문서 관리 하위 화면
func baro_admin_doc_acts() [][]string {
	return [][]string{
		{"", tool.Baro_t("개요")},
		{"quarantine", tool.Baro_t("격리 문서")},
		{"delete", tool.Baro_t("격리 · 삭제")},
		{"protect", tool.Baro_t("보호")},
		{"move", "이동"},
		{"merge", tool.Baro_t("병합")},
		{"revhide", tool.Baro_t("리비전 숨김")},
		{"impact", tool.Baro_t("영향 미리보기")},
		{"purge", "완전 삭제"},
	}
}

// baro_admin_doc_sub_tabs : 문서 관리 안쪽 탭
func baro_admin_doc_sub_tabs(active string) string {
	items := [][]string{}
	for _, row := range baro_admin_doc_acts() {
		target := "/baro/admin/doc"
		if row[0] != "" {
			target += "?act=" + row[0]
		}

		items = append(items, []string{target, row[1]})
	}

	target := "/baro/admin/doc"
	if active != "" {
		target += "?act=" + active
	}

	return tool.Baro_tabs(items, target)
}

// baro_admin_doc_exists : 문서가 있는지
func baro_admin_doc_exists(db *sql.DB, title string) bool {
	if strings.TrimSpace(title) == "" {
		return false
	}

	return baro_admin_one(db, "select title from data where title = ? limit 1", title) != ""
}

// baro_admin_doc_body : 문서 본문
func baro_admin_doc_body(db *sql.DB, title string) string {
	return baro_admin_one(db, "select data from data where title = ?", title)
}

// baro_admin_doc_titles : 여러 줄로 입력된 문서 이름을 정리한다
func baro_admin_doc_titles(data string) []string {
	seen := map[string]bool{}
	out := []string{}

	for _, line := range tool.Baro_split_lines(data) {
		title := tool.Namespace_normalize(strings.TrimSpace(line))
		if title == "" || seen[title] {
			continue
		}

		seen[title] = true
		out = append(out, title)
	}

	return out
}

// baro_admin_doc_backlinks : 이 문서를 링크하는 문서 목록 (영향 미리보기용)
func baro_admin_doc_backlinks(db *sql.DB, title string, limit int) []string {
	out := []string{}
	if limit < 1 {
		limit = 100
	}

	for _, one := range baro_admin_rows(
		db, 1,
		"select distinct link from back where title = ? and type not in ('no', 'nothing') order by link limit ?",
		title, limit,
	) {
		if one[0] == "" || one[0] == title {
			continue
		}

		out = append(out, one[0])
	}

	return out
}

// baro_admin_doc_last_editor : 문서의 마지막 편집자
func baro_admin_doc_last_editor(db *sql.DB, title string) string {
	return baro_admin_one(db, "select ip from history where title = ? order by id + 0 desc limit 1", title)
}

// baro_admin_doc_mass_handoff : 대량 작업으로 넘기는 확인 화면
//
//	기준을 넘는 조치는 이 화면에서 직접 처리하지 않는다.
//	대상 목록을 그대로 실어 대량 작업 신청 화면으로 보내고,
//	거기서 목록 공개 · 2인 승인 · 스냅샷 · 유예를 거치게 한다.
func baro_admin_doc_mass_handoff(db *sql.DB, kind string, reason string, titles []string) string {
	threshold := tool.Baro_mass_threshold(db)
	grace := tool.Baro_mass_grace_hours(db)

	items := []string{}
	for _, one := range titles {
		if len(items) >= 50 {
			break
		}

		items = append(items, tool.Baro_doc_link(one))
	}

	form := tool.Baro_hidden("kind", kind)
	form += tool.Baro_hidden("targets", strings.Join(titles, "\n"))
	form += tool.Baro_hidden("reason", reason)
	form += tool.Baro_submit(tool.Baro_t("대량 작업으로 신청하기 (아직 실행되지 않습니다)"), "primary")

	body := tool.Baro_alert("danger",
		tool.Baro_t("대상이 <b>")+baro_admin_esc(tool.Baro_itoa(len(titles)))+tool.Baro_t("개</b>로 기준(")+
			baro_admin_esc(tool.Baro_itoa(threshold))+tool.Baro_t("개)을 넘었습니다. ")+
			tool.Baro_t("이 화면에서는 처리하지 않고 대량 작업 절차로 넘깁니다."))

	body += tool.Baro_list([]string{
		tool.Baro_t("대상 수 표시와 목록 공개"),
		tool.Baro_t("서로 다른 운영자 2명의 승인"),
		tool.Baro_t("되돌리기용 스냅샷 저장"),
		baro_admin_esc(tool.Baro_itoa(grace)) + tool.Baro_t("시간 유예 뒤 실행"),
		tool.Baro_t("실행 공지와 한 번에 되돌리기"),
	})

	body += tool.Baro_dl([][]string{
		{tool.Baro_t("작업 종류"), baro_admin_esc(tool.Baro_mass_kind_name(kind))},
		{"사유", baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(reason), 200))},
		{tool.Baro_t("대상 수"), baro_admin_esc(tool.Baro_comma(len(titles)))},
	})

	body += tool.Baro_section(tool.Baro_t("대상 목록 (최대 50개 미리 보기)"), tool.Baro_list(items))
	body += tool.Baro_form("/baro/mass/new", form)
	body += tool.Baro_note(tool.Baro_t("단추를 누르면 대량 작업 신청만 만들어집니다. 실행은 승인과 유예를 거친 뒤에야 가능합니다."))

	return tool.Baro_danger_zone(tool.Baro_t("대량 작업으로 넘기기"), body)
}

// baro_admin_doc_quarantine_one : 문서 한 개를 임시 격리한다 (삭제의 기본 경로)
func baro_admin_doc_quarantine_one(db *sql.DB, config tool.Config, title string, reason string, days int, case_id string) bool {
	if !baro_admin_doc_exists(db, title) {
		return false
	}

	body := baro_admin_doc_body(db, title)
	rev := tool.Get_document_revision(db, title)

	release_date := ""
	if days > 0 {
		release_date = tool.Baro_time_add(tool.Baro_now(), days, 0, 0)
	}

	tool.Exec_DB(
		db,
		"insert into baro_quarantine (id, title, rev, kind, reason, actor, date, status, backup, release_date, case_id) values (?, ?, ?, 'admin', ?, ?, ?, 'open', ?, ?, ?)",
		tool.Baro_next_id(db, "baro_quarantine"), title, rev, reason, config.IP, tool.Baro_now(), body, release_date, case_id,
	)

	tool.Do_add_history(
		db, title, "", tool.Get_time(), config.IP,
		"[격리] "+reason, "-"+strconv.Itoa(tool.Get_len(body)), "delete", "",
	)

	// 이 문서를 가리키던 링크는 빨간 링크로 되돌린다
	link_list := baro_admin_rows(
		db, 2,
		"select title, link from back where title = ? and type not in ('cat', 'no')",
		title,
	)
	for _, one := range link_list {
		tool.Exec_DB(db, "insert into back (title, link, type, data) values (?, ?, 'no', '')", one[0], one[1])
	}

	tool.Exec_DB(db, "delete from back where link = ?", title)
	tool.Exec_DB(db, "delete from data where title = ?", title)

	tool.Baro_doc_status_set(db, title, "quarantined", reason, config.IP)

	baro_admin_log(db, config, "doc_quarantine", title, reason, tool.Baro_trunc(body, 300), tool.Baro_t("격리 (복구 가능)"))

	if editor := baro_admin_doc_last_editor(db, title); editor != "" && tool.Baro_is_user(editor) {
		tool.Baro_notify(db, editor, "moderation", "문서가 임시 격리되었습니다 : "+title, "/baro/admin/doc?act=quarantine", reason)
	}

	return true
}

// baro_admin_doc_release_one : 격리를 풀고 문서를 되살린다
func baro_admin_doc_release_one(db *sql.DB, config tool.Config, id string, reason string) (string, bool) {
	row := baro_admin_rows(db, 3, "select title, backup, status from baro_quarantine where id = ?", id)
	if len(row) == 0 {
		return "", false
	}

	title := row[0][0]
	backup := row[0][1]

	if baro_admin_doc_exists(db, title) {
		// 같은 이름의 문서가 이미 있으면 덮어쓰지 않는다
		tool.Exec_DB(db, "update baro_quarantine set status = 'released', release_date = ? where id = ?", tool.Baro_now(), id)
		tool.Baro_doc_status_set(db, title, "", "", config.IP)

		return title, true
	}

	tool.Exec_DB(db, "insert into data (title, data, type) values (?, ?, '')", title, backup)
	tool.Do_add_history(
		db, title, backup, tool.Get_time(), config.IP,
		"[격리 해제] "+reason, "+"+strconv.Itoa(tool.Get_len(backup)), "", "",
	)
	markup.Get_render(db, title, backup, "backlink")

	tool.Exec_DB(db, "update baro_quarantine set status = 'released', release_date = ? where id = ?", tool.Baro_now(), id)
	tool.Baro_doc_status_set(db, title, "", "", config.IP)

	baro_admin_log(db, config, "doc_release", title, reason, "격리", tool.Baro_t("복구 완료"))

	return title, true
}

// baro_admin_doc_protect_one : 문서 보호 (ACL 을 바꾼다)
func baro_admin_doc_protect_one(db *sql.DB, config tool.Config, title string, level string, fields []string, reason string) {
	before := []string{}

	for _, field := range fields {
		old := baro_admin_one(db, "select data from acl where title = ? and type = ? limit 1", title, field)
		before = append(before, field+"="+old)

		tool.Exec_DB(db, "delete from acl where title = ? and type = ?", title, field)
		tool.Exec_DB(db, "insert into acl (title, data, type) values (?, ?, ?)", title, level, field)
	}

	tool.Exec_DB(db, "delete from acl where title = ? and type = 'why'", title)
	tool.Exec_DB(db, "insert into acl (title, data, type) values (?, ?, 'why')", title, reason)

	baro_admin_log(db, config, "doc_protect", title, reason, strings.Join(before, " / "), strings.Join(fields, ",")+"="+level)
	tool.Do_insert_auth_history(db, config.IP, "baro_doc_protect ("+title+")")
}

// baro_admin_doc_move_one : 문서 이동 (원래 이름은 넘겨주기로 남길 수 있다)
func baro_admin_doc_move_one(db *sql.DB, config tool.Config, old_title string, new_title string, reason string, leave_redirect bool) bool {
	if !baro_admin_doc_exists(db, old_title) || baro_admin_doc_exists(db, new_title) {
		return false
	}

	body := baro_admin_doc_body(db, old_title)

	tool.Exec_DB(db, "insert into data (title, data, type) values (?, ?, '')", new_title, body)
	tool.Exec_DB(db, "update history set title = ? where title = ?", new_title, old_title)
	tool.Exec_DB(db, "update baro_doc_status set title = ? where title = ?", new_title, old_title)
	tool.Exec_DB(db, "update baro_doc_meta set title = ? where title = ?", new_title, old_title)
	tool.Exec_DB(db, "delete from back where link = ?", old_title)
	tool.Exec_DB(db, "delete from data where title = ?", old_title)

	tool.Do_add_history(
		db, new_title, body, tool.Get_time(), config.IP,
		"[이동] "+old_title+" → "+new_title+" : "+reason, "0", "move", "",
	)
	markup.Get_render(db, new_title, body, "backlink")

	if leave_redirect {
		redirect_body := "#redirect " + new_title

		tool.Exec_DB(db, "insert into data (title, data, type) values (?, ?, '')", old_title, redirect_body)
		tool.Do_add_history(
			db, old_title, redirect_body, tool.Get_time(), config.IP,
			"[이동 넘겨주기] "+new_title, "+"+strconv.Itoa(tool.Get_len(redirect_body)), "", "",
		)
		markup.Get_render(db, old_title, redirect_body, "backlink")
	}

	baro_admin_log(db, config, "doc_move", old_title, reason, old_title, new_title)
	tool.Do_insert_auth_history(db, config.IP, "baro_doc_move ("+old_title+" -> "+new_title+")")

	return true
}

// baro_admin_doc_merge_one : 문서 병합 (원본은 넘겨주기로 바꾼다)
func baro_admin_doc_merge_one(db *sql.DB, config tool.Config, from_title string, to_title string, reason string) bool {
	if !baro_admin_doc_exists(db, from_title) || !baro_admin_doc_exists(db, to_title) || from_title == to_title {
		return false
	}

	from_body := baro_admin_doc_body(db, from_title)
	to_body := baro_admin_doc_body(db, to_title)

	merged := to_body + "\n\n== " + from_title + " 에서 옮겨 온 내용 ==\n" + from_body

	tool.Exec_DB(db, "update data set data = ? where title = ?", merged, to_title)
	tool.Do_add_history(
		db, to_title, merged, tool.Get_time(), config.IP,
		"[병합] "+from_title+" → "+to_title+" : "+reason,
		"+"+strconv.Itoa(tool.Get_len(merged)-tool.Get_len(to_body)), "", "",
	)
	markup.Get_render(db, to_title, merged, "backlink")

	redirect_body := "#redirect " + to_title

	tool.Exec_DB(db, "update data set data = ? where title = ?", redirect_body, from_title)
	tool.Exec_DB(db, "delete from back where link = ?", from_title)
	tool.Do_add_history(
		db, from_title, redirect_body, tool.Get_time(), config.IP,
		"[병합 넘겨주기] "+to_title, "-"+strconv.Itoa(tool.Get_len(from_body)), "", "",
	)
	markup.Get_render(db, from_title, redirect_body, "backlink")

	baro_admin_log(db, config, "doc_merge", to_title, reason, from_title, to_title)
	tool.Do_insert_auth_history(db, config.IP, "baro_doc_merge ("+from_title+" -> "+to_title+")")

	return true
}

// baro_admin_doc_post : 문서 관리 쓰기 동작 처리
//
//	돌려주는 값이 "" 가 아니면 그대로 반환(리다이렉트)하고, 아니면 두 번째 값을 안내로 쓴다.
func baro_admin_doc_post(db *sql.DB, config tool.Config, values url.Values) (string, string) {
	do := baro_admin_get(values, "do")
	reason := baro_admin_get(values, "reason")
	threshold := tool.Baro_mass_threshold(db)

	if tool.Baro_read_only(db) {
		return "", tool.Baro_alert("danger", tool.Baro_t("읽기 전용 상태라 문서를 바꿀 수 없습니다. 사유 : ")+baro_admin_esc(tool.Baro_read_only_reason(db)))
	}

	if reason == "" {
		return "", tool.Baro_alert("warn", tool.Baro_t("사유는 반드시 적어야 합니다. 모든 조치는 근거와 함께 기록됩니다."))
	}

	switch do {
	case "quarantine":
		titles := baro_admin_doc_titles(baro_admin_get(values, "titles"))
		if len(titles) == 0 {
			return "", tool.Baro_alert("warn", tool.Baro_t("문서 이름을 한 줄에 하나씩 넣어 주세요."))
		}

		if len(titles) >= threshold {
			return "", baro_admin_doc_mass_handoff(db, "quarantine", reason, titles)
		}

		days := tool.Str_to_int(baro_admin_get(values, "days"))
		case_id := baro_admin_get(values, "case_id")

		done := 0
		for _, title := range titles {
			if baro_admin_doc_quarantine_one(db, config, title, reason, days, case_id) {
				done++
			}
		}

		return tool.Get_redirect("/baro/admin/doc?act=quarantine&done=" + tool.Baro_itoa(done)), ""

	case "release":
		id := baro_admin_get(values, "id")
		if _, ok := baro_admin_doc_release_one(db, config, id, reason); !ok {
			return "", tool.Baro_alert("warn", tool.Baro_t("격리 기록을 찾지 못했습니다."))
		}

		return tool.Get_redirect("/baro/admin/doc?act=quarantine&released=1"), ""

	case "purge":
		// 완전 삭제 : 별도 확인 문구가 필요하다 (0장 원칙 2)
		if baro_admin_get(values, "confirm") != "완전삭제" {
			return "", tool.Baro_alert("danger", tool.Baro_t("확인 칸에 <b>완전삭제</b> 라고 정확히 입력해야 진행됩니다."))
		}

		id := baro_admin_get(values, "id")
		row := baro_admin_rows(db, 3, "select title, backup, status from baro_quarantine where id = ?", id)
		if len(row) == 0 {
			return "", tool.Baro_alert("warn", tool.Baro_t("격리 기록을 찾지 못했습니다. 완전 삭제는 격리된 문서에만 할 수 있습니다."))
		}

		if row[0][2] == "deleted" {
			return "", tool.Baro_alert("warn", tool.Baro_t("이미 완전 삭제된 기록입니다."))
		}

		title := row[0][0]

		tool.Exec_DB(db, "update baro_quarantine set status = 'deleted', backup = '', release_date = ? where id = ?", tool.Baro_now(), id)
		tool.Exec_DB(db, "delete from data where title = ?", title)
		tool.Exec_DB(db, "delete from back where link = ?", title)
		tool.Baro_doc_status_set(db, title, "archived", reason, config.IP)

		baro_admin_log(db, config, "doc_purge", title, reason, tool.Baro_t("격리 보관본 있음"), tool.Baro_t("보관본 삭제 (되돌릴 수 없음)"))
		tool.Do_insert_auth_history(db, config.IP, "baro_doc_purge ("+title+")")

		return tool.Get_redirect("/baro/admin/doc?act=quarantine&purged=1"), ""

	case "protect":
		titles := baro_admin_doc_titles(baro_admin_get(values, "titles"))
		if len(titles) == 0 {
			return "", tool.Baro_alert("warn", tool.Baro_t("문서 이름을 한 줄에 하나씩 넣어 주세요."))
		}

		if len(titles) >= threshold {
			return "", baro_admin_doc_mass_handoff(db, "protect", reason, titles)
		}

		level := baro_admin_get(values, "level")

		switch level {
		case "", "all", "user", "30_day", "50_edit", "30_day_50_edit", "admin", "owner":
		default:
			return "", tool.Baro_alert("warn", tool.Baro_t("허용 범위 값이 올바르지 않습니다."))
		}

		fields := []string{"document_edit_acl"}
		if baro_admin_get(values, "move") != "" {
			fields = append(fields, "document_move_acl")
		}
		if baro_admin_get(values, "delete") != "" {
			fields = append(fields, "document_delete_acl")
		}

		for _, title := range titles {
			baro_admin_doc_protect_one(db, config, title, level, fields, reason)
		}

		return tool.Get_redirect("/baro/admin/doc?act=protect&done=" + tool.Baro_itoa(len(titles))), ""

	case "move":
		old_title := tool.Namespace_normalize(baro_admin_get(values, "from"))
		new_title := tool.Namespace_normalize(baro_admin_get(values, "to"))

		if old_title == "" || new_title == "" {
			return "", tool.Baro_alert("warn", tool.Baro_t("옮길 문서와 새 이름을 모두 넣어 주세요."))
		}

		if !baro_admin_doc_move_one(db, config, old_title, new_title, reason, baro_admin_get(values, "redirect") != "") {
			return "", tool.Baro_alert("danger", tool.Baro_t("이동할 수 없습니다. 원본이 없거나 새 이름의 문서가 이미 있습니다."))
		}

		return tool.Get_redirect("/w/" + tool.Doc_url(new_title)), ""

	case "merge":
		from_title := tool.Namespace_normalize(baro_admin_get(values, "from"))
		to_title := tool.Namespace_normalize(baro_admin_get(values, "to"))

		if !baro_admin_doc_merge_one(db, config, from_title, to_title, reason) {
			return "", tool.Baro_alert("danger", tool.Baro_t("병합할 수 없습니다. 두 문서가 모두 있어야 하고 서로 달라야 합니다."))
		}

		return tool.Get_redirect("/w/" + tool.Doc_url(to_title)), ""

	case "revhide":
		title := tool.Namespace_normalize(baro_admin_get(values, "title"))
		rev := baro_admin_get(values, "rev")

		if title == "" || rev == "" {
			return "", tool.Baro_alert("warn", tool.Baro_t("문서 이름과 판 번호를 넣어 주세요."))
		}

		found := baro_admin_rows(db, 1, "select hide from history where title = ? and id = ?", title, rev)
		if len(found) == 0 {
			return "", tool.Baro_alert("warn", tool.Baro_t("그런 판이 없습니다."))
		}

		hide := "O"
		action := tool.Baro_t("리비전 숨김")
		if found[0][0] != "" {
			hide = ""
			action = tool.Baro_t("리비전 숨김 해제")
		}

		tool.Exec_DB(db, "update history set hide = ? where title = ? and id = ?", hide, title, rev)

		baro_admin_log(db, config, "rev_hide", title+" r"+rev, reason, found[0][0], action)
		tool.Do_insert_auth_history(db, config.IP, "baro_rev_hide ("+title+" r"+rev+")")

		return tool.Get_redirect("/baro/admin/doc?act=revhide&title=" + tool.Url_parser(title)), ""
	}

	return "", tool.Baro_alert("warn", tool.Baro_t("알 수 없는 동작입니다."))
}

// View_baro_admin_doc : 16-3 문서 관리
func View_baro_admin_doc(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if html, stop := baro_admin_gate(db, config, "quarantine"); stop {
		return html
	}

	notice := ""
	if values != nil {
		redirect, message := baro_admin_doc_post(db, config, values)
		if redirect != "" {
			return redirect
		}

		notice = message
	}

	act := baro_admin_get(query, "act")
	threshold := tool.Baro_mass_threshold(db)

	body := tool.Baro_hero(
		"문서 관리",
		tool.Baro_t("삭제·복구·보호·이동·병합·격리·리비전 숨김을 한곳에서 처리합니다."),
		tool.Baro_button("/baro/mass", tool.Baro_t("대량 작업 목록"), "")+" "+tool.Baro_button("/trash", tool.Baro_t("휴지통"), ""),
	)
	body += baro_admin_tabs("/baro/admin/doc")
	body += baro_admin_doc_sub_tabs(act)

	if notice != "" {
		body += notice
	}

	if baro_admin_get(query, "done") != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("요청한 조치를 ")+baro_admin_esc(baro_admin_get(query, "done"))+tool.Baro_t("건 처리했습니다."))
	}
	if baro_admin_get(query, "released") != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("격리를 해제하고 문서를 되살렸습니다."))
	}
	if baro_admin_get(query, "purged") != "" {
		body += tool.Baro_alert("warn", tool.Baro_t("보관본을 완전히 삭제했습니다. 이 조치는 되돌릴 수 없습니다."))
	}

	body += tool.Baro_alert("info",
		tool.Baro_t("대상이 <b>")+baro_admin_esc(tool.Baro_itoa(threshold))+tool.Baro_t("개 이상</b>이면 직접 처리하지 않고 대량 작업 신청 화면으로 넘어갑니다. ")+
			tool.Baro_t("거기서 대상 목록 공개 · 2인 승인 · 스냅샷 · 유예를 거칩니다."))

	action := "/baro/admin/doc"

	switch act {
	case "quarantine":
		body += baro_admin_doc_quarantine_list(db, config, query)

	case "delete":
		form := tool.Baro_hidden("do", "quarantine")
		form += tool.Baro_textarea("titles", tool.Baro_t("격리할 문서"), baro_admin_get(query, "title"), 6, tool.Baro_t("한 줄에 하나씩"), tool.Baro_t("삭제하지 않고 임시 격리합니다. 본문은 그대로 보관되어 언제든 되살릴 수 있습니다."))
		form += tool.Baro_textarea("reason", tool.Baro_t("사유 (필수)"), "", 3, tool.Baro_t("어떤 규정의 어느 항목에 해당하는지"), tool.Baro_t("공개 운영 로그에 그대로 남습니다."))
		form += tool.Baro_input_type("days", tool.Baro_t("자동 해제까지 (일)"), "", "number", tool.Baro_t("비워 두면 사람이 직접 풀 때까지 유지됩니다."))
		form += tool.Baro_input("case_id", tool.Baro_t("관련 사건 번호"), baro_admin_get(query, "case_id"), tool.Baro_t("예: 12"), tool.Baro_t("사건이 있으면 번호를 적어 두면 추적이 쉬워집니다."))
		form += tool.Baro_submit(tool.Baro_t("임시 격리"), "danger")

		body += tool.Baro_card_ex(
			tool.Baro_t("격리 (삭제의 기본 경로)"),
			tool.Baro_t("바로위키에서 문서 삭제는 곧바로 지우는 것이 아니라 임시 격리입니다."),
			tool.Baro_form(action, form),
			tool.Baro_note(tool.Baro_t("완전 삭제는 격리된 문서에 한해 '완전 삭제' 탭에서 별도 확인을 받습니다.")),
		)

		body += baro_admin_doc_impact_card(db, baro_admin_get(query, "title"))

	case "protect":
		level_options := [][]string{
			{"", tool.Baro_t("기본값 (이름공간 설정을 따름)")},
			{"all", tool.Baro_t("모두")},
			{"user", tool.Baro_t("로그인 사용자")},
			{"30_day", tool.Baro_t("가입 30일 이상")},
			{"50_edit", tool.Baro_t("편집 50회 이상")},
			{"30_day_50_edit", tool.Baro_t("가입 30일 + 편집 50회")},
			{"admin", tool.Baro_t("관리자")},
			{"owner", tool.Baro_t("소유자")},
		}

		form := tool.Baro_hidden("do", "protect")
		form += tool.Baro_textarea("titles", tool.Baro_t("보호할 문서"), baro_admin_get(query, "title"), 5, tool.Baro_t("한 줄에 하나씩"), "")
		form += tool.Baro_select("level", tool.Baro_t("편집 허용 범위"), level_options, "", tool.Baro_t("고를수록 더 좁은 범위만 편집할 수 있습니다."))
		form += tool.Baro_checkbox("move", tool.Baro_t("이동 권한도 같은 값으로"), "on", false, "")
		form += tool.Baro_checkbox("delete", tool.Baro_t("삭제 권한도 같은 값으로"), "on", false, "")
		form += tool.Baro_textarea("reason", tool.Baro_t("사유 (필수)"), "", 3, tool.Baro_t("왜 보호하는지"), tool.Baro_t("문서 ACL 사유 칸에도 함께 저장됩니다."))
		form += tool.Baro_submit(tool.Baro_t("보호 적용"), "primary")

		body += tool.Baro_card_ex(tool.Baro_t("문서 보호"), tool.Baro_t("acl 표의 편집·이동·삭제 권한을 바꿉니다."), tool.Baro_form(action, form), "")
		body += baro_admin_doc_protect_state_card(db, baro_admin_get(query, "title"))

	case "move":
		form := tool.Baro_hidden("do", "move")
		form += tool.Baro_input("from", tool.Baro_t("옮길 문서"), baro_admin_get(query, "title"), tool.Baro_t("현재 이름"), "")
		form += tool.Baro_input("to", tool.Baro_t("새 이름"), "", tool.Baro_t("바꿀 이름"), tool.Baro_t("이미 있는 이름으로는 옮길 수 없습니다."))
		form += tool.Baro_checkbox("redirect", tool.Baro_t("원래 이름을 넘겨주기로 남긴다"), "on", true, tool.Baro_t("끄면 원래 이름은 사라집니다. 링크가 많으면 켜 두세요."))
		form += tool.Baro_textarea("reason", tool.Baro_t("사유 (필수)"), "", 3, tool.Baro_t("왜 옮기는지"), "")
		form += tool.Baro_submit(tool.Baro_t("문서 이동"), "primary")

		body += tool.Baro_card_ex(tool.Baro_t("문서 이동"), tool.Baro_t("편집 이력을 그대로 가지고 이름만 바꿉니다."), tool.Baro_form(action, form), "")
		body += baro_admin_doc_impact_card(db, baro_admin_get(query, "title"))

	case "merge":
		form := tool.Baro_hidden("do", "merge")
		form += tool.Baro_input("from", tool.Baro_t("합칠 문서 (원본)"), baro_admin_get(query, "title"), tool.Baro_t("이 문서의 본문을 옮깁니다"), tool.Baro_t("처리 뒤 넘겨주기 문서가 됩니다."))
		form += tool.Baro_input("to", tool.Baro_t("받는 문서 (대상)"), "", tool.Baro_t("본문이 붙는 쪽"), "")
		form += tool.Baro_textarea("reason", tool.Baro_t("사유 (필수)"), "", 3, tool.Baro_t("왜 합치는지"), "")
		form += tool.Baro_submit(tool.Baro_t("문서 병합"), "primary")

		body += tool.Baro_card_ex(
			tool.Baro_t("문서 병합"),
			tool.Baro_t("원본 본문을 대상 문서 끝에 붙이고, 원본은 대상으로 가는 넘겨주기로 바꿉니다."),
			tool.Baro_form(action, form),
			tool.Baro_note(tool.Baro_t("원본 문서의 편집 이력은 그대로 남습니다. 저작자 표시를 위해 이력을 지우지 않습니다.")),
		)
		body += baro_admin_doc_dup_card(db)

	case "revhide":
		title := tool.Namespace_normalize(baro_admin_get(query, "title"))

		find_form := tool.Baro_input("title", tool.Baro_t("문서 이름"), title, tool.Baro_t("판 목록을 볼 문서"), "")
		find_form += tool.Baro_hidden("act", "revhide")
		find_form += tool.Baro_submit(tool.Baro_t("판 목록 보기"), "")

		body += tool.Baro_card(tool.Baro_t("문서 고르기"), `<form class="bw_form" method="get" action="/baro/admin/doc">`+find_form+`</form>`)

		if title != "" {
			rows := [][]string{}
			for _, one := range baro_admin_rows(
				db, 6,
				"select id, date, ip, send, leng, hide from history where title = ? order by id + 0 desc limit 100",
				title,
			) {
				state := tool.Baro_badge(tool.Baro_t("보임"), "ok")
				button_text := tool.Baro_t("숨기기")
				if one[5] != "" {
					state = tool.Baro_badge(tool.Baro_t("숨김"), "danger")
					button_text = tool.Baro_t("숨김 해제")
				}

				one_form := tool.Baro_hidden("do", "revhide")
				one_form += tool.Baro_hidden("title", title)
				one_form += tool.Baro_hidden("rev", one[0])
				one_form += tool.Baro_hidden("reason", tool.Baro_t("리비전 숨김 처리"))
				one_form += tool.Baro_submit(button_text, "danger")

				rows = append(rows, []string{
					`<a href="/render/` + tool.Url_parser(one[0]) + `/` + tool.Url_parser(title) + `">r` + baro_admin_esc(one[0]) + `</a>`,
					tool.Baro_time_cell(one[1]),
					baro_admin_actor(db, config, one[2]),
					baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(one[3]), 30)),
					baro_admin_esc(one[4]),
					state,
					tool.Baro_form(action, one_form),
				})
			}

			body += tool.Baro_card_ex(
				tool.Baro_t("판 목록 : ")+title,
				tool.Baro_t("history 표의 hide 칸을 켜고 끕니다."),
				tool.Baro_table([]string{"판", "시각", tool.Baro_t("편집자"), tool.Baro_t("요약"), tool.Baro_t("크기"), "상태", tool.Baro_t("조치")}, rows, tool.Baro_t("판 기록이 없습니다.")),
				tool.Baro_note(tool.Baro_t("숨긴 판은 일반 이용자에게 보이지 않지만 내용은 지워지지 않습니다.")),
			)
		} else {
			body += tool.Baro_empty(tool.Baro_t("문서 이름을 넣으면 판 목록이 나옵니다."))
		}

	case "impact":
		title := tool.Namespace_normalize(baro_admin_get(query, "title"))

		find_form := tool.Baro_input("title", tool.Baro_t("문서 이름"), title, tool.Baro_t("영향을 볼 문서"), "")
		find_form += tool.Baro_hidden("act", "impact")
		find_form += tool.Baro_submit(tool.Baro_t("영향 보기"), "primary")

		body += tool.Baro_card(tool.Baro_t("영향 미리보기"), `<form class="bw_form" method="get" action="/baro/admin/doc">`+find_form+`</form>`)
		body += baro_admin_doc_impact_card(db, title)

	case "purge":
		body += baro_admin_doc_purge_card(db, config, query)

	default:
		body += baro_admin_doc_overview(db, config)
	}

	return baro_admin_page(db, config, "문서 관리", body)
}

// baro_admin_doc_overview : 문서 관리 개요
func baro_admin_doc_overview(db *sql.DB, config tool.Config) string {
	quarantine_num := baro_admin_num(db, "select count(*) from baro_quarantine where status = 'open'")
	released_num := baro_admin_num(db, "select count(*) from baro_quarantine where status = 'released'")
	deleted_num := baro_admin_num(db, "select count(*) from baro_quarantine where status = 'deleted'")
	protected_num := baro_admin_num(db, "select count(*) from acl where type = 'document_edit_acl' and data != ''")
	hidden_num := baro_admin_num(db, "select count(*) from history where hide != ''")
	doc_num := baro_admin_num(db, "select count(*) from data")

	body := tool.Baro_stat_row([][]string{
		{tool.Baro_t("전체 문서"), tool.Baro_comma(doc_num), tool.Baro_t("data 표")},
		{tool.Baro_t("격리 중"), tool.Baro_comma(quarantine_num), "baro_quarantine"},
		{tool.Baro_t("복구됨"), tool.Baro_comma(released_num), tool.Baro_t("되살린 문서")},
		{"완전 삭제", tool.Baro_comma(deleted_num), tool.Baro_t("보관본까지 지움")},
		{tool.Baro_t("보호 중"), tool.Baro_comma(protected_num), tool.Baro_t("acl 편집 제한")},
		{tool.Baro_t("숨긴 판"), tool.Baro_comma(hidden_num), "history.hide"},
	})

	rows := [][]string{}
	for _, one := range baro_admin_rows(
		db, 5,
		"select kind, actor, target, reason, date from baro_public_log where kind in ('doc_quarantine', 'doc_release', 'doc_purge', 'doc_move', 'doc_merge', 'doc_protect', 'rev_hide') order by id + 0 desc limit 15",
	) {
		rows = append(rows, []string{
			baro_admin_esc(baro_admin_action_name(one[0])),
			baro_admin_actor(db, config, one[1]),
			baro_admin_esc(tool.Namespace_display(one[2])),
			baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(one[3]), 40)),
			tool.Baro_time_cell(one[4]),
		})
	}

	body += tool.Baro_section(tool.Baro_t("최근 문서 조치"), tool.Baro_table(
		[]string{"종류", tool.Baro_t("조치자"), tool.Baro_t("대상"), "사유", "시각"},
		rows,
		tool.Baro_t("기록된 문서 조치가 없습니다."),
	))

	body += tool.Baro_section(tool.Baro_t("빠른 이동"), tool.Baro_grid([]string{
		tool.Baro_card_ex(tool.Baro_t("격리 · 삭제"), tool.Baro_t("삭제 대신 임시 격리로 보냅니다."), tool.Baro_note(tool.Baro_t("복구 가능한 보관본을 남깁니다.")), tool.Baro_button("/baro/admin/doc?act=delete", tool.Baro_t("열기"), "primary")),
		tool.Baro_card_ex(tool.Baro_t("보호"), tool.Baro_t("편집·이동·삭제 권한을 좁힙니다."), tool.Baro_note(tool.Baro_t("acl 표를 바꿉니다.")), tool.Baro_button("/baro/admin/doc?act=protect", tool.Baro_t("열기"), "primary")),
		tool.Baro_card_ex(tool.Baro_t("이동 · 병합"), tool.Baro_t("이름을 바꾸거나 두 문서를 합칩니다."), tool.Baro_note(tool.Baro_t("원본은 넘겨주기로 남습니다.")), tool.Baro_button("/baro/admin/doc?act=move", tool.Baro_t("열기"), "primary")),
		tool.Baro_card_ex(tool.Baro_t("리비전 숨김"), tool.Baro_t("특정 판만 가립니다."), tool.Baro_note(tool.Baro_t("history 의 hide 칸을 씁니다.")), tool.Baro_button("/baro/admin/doc?act=revhide", tool.Baro_t("열기"), "primary")),
	}, 4))

	return tool.Baro_card_ex(tool.Baro_t("문서 관리 개요"), tool.Baro_t("표를 직접 읽어 계산한 현재 상태입니다."), body, "")
}

// baro_admin_doc_quarantine_list : 격리 문서 목록
func baro_admin_doc_quarantine_list(db *sql.DB, config tool.Config, query url.Values) string {
	page := baro_admin_page_num(query)
	size := tool.Baro_page_size
	status := baro_admin_get(query, "status")

	switch status {
	case "open", "released", "deleted":
	default:
		status = "open"
	}

	list := baro_admin_rows(
		db, 8,
		"select id, title, reason, actor, date, status, release_date, case_id from baro_quarantine where status = ? order by id + 0 desc limit ?, ?",
		status, (page-1)*size, size,
	)

	rows := [][]string{}
	for _, one := range list {
		release_form := tool.Baro_hidden("do", "release")
		release_form += tool.Baro_hidden("id", one[0])
		release_form += tool.Baro_hidden("reason", tool.Baro_t("격리 해제"))
		release_form += tool.Baro_submit("복구", "primary")

		buttons := ""
		if one[5] == "open" || one[5] == "" {
			buttons = tool.Baro_form("/baro/admin/doc", release_form)
			buttons += tool.Baro_button("/baro/admin/doc?act=purge&id="+tool.Url_parser(one[0]), "완전 삭제", "danger")
		} else {
			buttons = baro_admin_status_badge(one[5])
		}

		case_html := `<span class="bw_muted">-</span>`
		if one[7] != "" {
			case_html = `<a href="/baro/case/view/` + tool.Url_parser(one[7]) + `">#` + baro_admin_esc(one[7]) + `</a>`
		}

		rows = append(rows, []string{
			`#` + baro_admin_esc(one[0]),
			baro_admin_esc(tool.Namespace_display(one[1])),
			baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(one[2]), 40)),
			baro_admin_actor(db, config, one[3]),
			tool.Baro_time_cell(one[4]),
			baro_admin_deadline_cell(one[6]),
			case_html,
			buttons,
		})
	}

	table := tool.Baro_table(
		[]string{"번호", "문서", "사유", tool.Baro_t("조치자"), tool.Baro_t("격리 시각"), tool.Baro_t("자동 해제"), "사건", tool.Baro_t("조치")},
		rows,
		tool.Baro_t("격리된 문서가 없습니다."),
	)

	if len(rows) == 0 {
		table += tool.Baro_empty(tool.Baro_t("이 상태의 격리 기록이 없습니다. 상태를 바꿔 보세요."))
	}

	table += tool.Baro_paging(page, len(rows), size, "/baro/admin/doc?act=quarantine&status="+tool.Url_parser(status)+"&page={}")

	filter := tool.Baro_tabs([][]string{
		{"/baro/admin/doc?act=quarantine&status=open", tool.Baro_t("격리 중")},
		{"/baro/admin/doc?act=quarantine&status=released", tool.Baro_t("복구됨")},
		{"/baro/admin/doc?act=quarantine&status=deleted", tool.Baro_t("완전 삭제됨")},
	}, "/baro/admin/doc?act=quarantine&status="+status)

	return tool.Baro_card_ex(
		tool.Baro_t("격리 문서"),
		tool.Baro_t("삭제 대신 보관 중인 문서입니다. 본문은 baro_quarantine.backup 에 그대로 있습니다."),
		filter+table,
		tool.Baro_button("/baro/admin/doc?act=delete", tool.Baro_t("새로 격리하기"), "primary"),
	)
}

// baro_admin_doc_purge_card : 완전 삭제 확인 화면
func baro_admin_doc_purge_card(db *sql.DB, config tool.Config, query url.Values) string {
	id := baro_admin_get(query, "id")

	if id == "" {
		rows := [][]string{}
		for _, one := range baro_admin_rows(
			db, 4,
			"select id, title, reason, date from baro_quarantine where status = 'open' order by id + 0 desc limit 50",
		) {
			rows = append(rows, []string{
				`#` + baro_admin_esc(one[0]),
				baro_admin_esc(tool.Namespace_display(one[1])),
				baro_admin_esc(tool.Baro_trunc(tool.Baro_one_line(one[2]), 40)),
				tool.Baro_time_cell(one[3]),
				tool.Baro_button("/baro/admin/doc?act=purge&id="+tool.Url_parser(one[0]), "완전 삭제 검토", "danger"),
			})
		}

		return tool.Baro_danger_zone(
			"완전 삭제",
			tool.Baro_alert("danger", tool.Baro_t("완전 삭제는 보관본까지 지웁니다. 되돌릴 수 없으므로 저작권 침해·개인정보 노출처럼 보관 자체가 문제인 경우에만 씁니다."))+
				tool.Baro_table([]string{"번호", "문서", tool.Baro_t("격리 사유"), tool.Baro_t("격리 시각"), ""}, rows, tool.Baro_t("격리 중인 문서가 없습니다.")),
		)
	}

	row := baro_admin_rows(db, 5, "select title, reason, actor, date, backup from baro_quarantine where id = ?", id)
	if len(row) == 0 {
		return tool.Baro_alert("warn", tool.Baro_t("그 번호의 격리 기록이 없습니다."))
	}

	form := tool.Baro_hidden("do", "purge")
	form += tool.Baro_hidden("id", id)
	form += tool.Baro_textarea("reason", tool.Baro_t("완전 삭제 사유 (필수)"), "", 3, tool.Baro_t("예: 저작권 침해 통보 접수"), tool.Baro_t("감사 로그에 남습니다."))
	form += tool.Baro_input("confirm", tool.Baro_t("확인 문구"), "", "완전삭제", tool.Baro_t("정확히 '완전삭제' 라고 입력해야 진행됩니다."))
	form += tool.Baro_submit(tool.Baro_t("보관본까지 완전 삭제"), "danger")

	body := tool.Baro_alert("danger", tool.Baro_t("이 조치는 되돌릴 수 없습니다. 먼저 격리 해제로 되살릴 수 있는지 검토하세요."))
	body += tool.Baro_dl([][]string{
		{"문서", baro_admin_esc(tool.Namespace_display(row[0][0]))},
		{tool.Baro_t("격리 사유"), baro_admin_esc(row[0][1])},
		{tool.Baro_t("격리자"), baro_admin_actor(db, config, row[0][2])},
		{tool.Baro_t("격리 시각"), baro_admin_esc(row[0][3])},
		{tool.Baro_t("보관본 길이"), baro_admin_esc(tool.Baro_comma(tool.Get_len(row[0][4]))) + tool.Baro_t("자")},
	})
	body += tool.Baro_section(tool.Baro_t("보관본 미리보기"), tool.Baro_code(tool.Baro_trunc(row[0][4], 600)))
	body += tool.Baro_form("/baro/admin/doc", form)

	return tool.Baro_danger_zone(tool.Baro_t("완전 삭제 확인"), body)
}

// baro_admin_doc_impact_card : 영향 미리보기 (back 표)
func baro_admin_doc_impact_card(db *sql.DB, title string) string {
	title = tool.Namespace_normalize(strings.TrimSpace(title))
	if title == "" {
		return tool.Baro_card(tool.Baro_t("영향 미리보기"), tool.Baro_note(tool.Baro_t("문서 이름을 넣으면 이 문서를 링크하는 문서 목록을 보여 줍니다.")))
	}

	links := baro_admin_doc_backlinks(db, title, 200)

	items := []string{}
	for _, one := range links {
		items = append(items, tool.Baro_doc_link(one))
	}

	body := tool.Baro_stat_row([][]string{
		{tool.Baro_t("이 문서를 링크하는 문서"), tool.Baro_comma(len(links)), tool.Baro_t("back 표")},
		{tool.Baro_t("현재 상태"), tool.Baro_doc_status_name(tool.Baro_doc_status_get(db, title)), title},
		{tool.Baro_t("판 수"), tool.Baro_comma(baro_admin_num(db, "select count(*) from history where title = ?", title)), "history"},
	})

	if len(links) == 0 {
		body += tool.Baro_empty(tool.Baro_t("이 문서를 링크하는 문서가 없습니다. 조치해도 다른 문서에 빨간 링크가 생기지 않습니다."))
	} else {
		if len(links) >= 20 {
			body += tool.Baro_alert("warn", tool.Baro_t("링크가 <b>")+baro_admin_esc(tool.Baro_itoa(len(links)))+tool.Baro_t("개</b>입니다. 지우면 그만큼 빨간 링크가 생깁니다. 이동·병합을 먼저 검토하세요."))
		}

		body += tool.Baro_list(items)
	}

	return tool.Baro_card_ex(tool.Baro_t("영향 미리보기 : ")+title, tool.Baro_t("조치 전에 어떤 문서가 영향을 받는지 확인합니다."), body, "")
}

// baro_admin_doc_protect_state_card : 현재 보호 상태
func baro_admin_doc_protect_state_card(db *sql.DB, title string) string {
	title = tool.Namespace_normalize(strings.TrimSpace(title))
	if title == "" {
		title_rows := [][]string{}
		for _, one := range baro_admin_rows(
			db, 3,
			"select title, data, type from acl where type in ('document_edit_acl', 'document_move_acl', 'document_delete_acl') and data != '' order by title limit 40",
		) {
			title_rows = append(title_rows, []string{
				tool.Baro_doc_link(one[0]),
				baro_admin_esc(one[2]),
				tool.Baro_badge(one[1], "warn"),
			})
		}

		return tool.Baro_card_ex(
			tool.Baro_t("현재 보호된 문서"),
			tool.Baro_t("acl 표에서 값이 지정된 문서입니다."),
			tool.Baro_table([]string{"문서", tool.Baro_t("항목"), tool.Baro_t("허용 범위")}, title_rows, tool.Baro_t("보호된 문서가 없습니다.")),
			"",
		)
	}

	rows := [][]string{}
	for _, field := range []string{"document_edit_acl", "document_move_acl", "document_delete_acl", "view"} {
		value := baro_admin_one(db, "select data from acl where title = ? and type = ? limit 1", title, field)
		if value == "" {
			value = tool.Baro_t("(기본값)")
		}

		rows = append(rows, []string{baro_admin_esc(field), baro_admin_esc(value)})
	}

	return tool.Baro_card_ex(tool.Baro_t("현재 보호 상태 : ")+title, tool.Baro_t("acl 표를 직접 읽었습니다."), tool.Baro_table([]string{tool.Baro_t("항목"), tool.Baro_t("허용 범위")}, rows, ""), "")
}

// baro_admin_doc_dup_card : 중복 문서 후보 (병합 대상 찾기)
func baro_admin_doc_dup_card(db *sql.DB) string {
	rows := [][]string{}
	for _, one := range baro_admin_rows(
		db, 5,
		"select id, title, target, ratio, status from baro_dup where status in ('open', 'pending', 'new') order by id + 0 desc limit 20",
	) {
		rows = append(rows, []string{
			tool.Baro_doc_link(one[1]),
			tool.Baro_doc_link(one[2]),
			baro_admin_esc(one[3]),
			baro_admin_status_badge(one[4]),
			tool.Baro_button("/baro/admin/doc?act=merge&title="+tool.Url_parser(one[1]), "병합 준비", ""),
		})
	}

	return tool.Baro_card_ex(
		tool.Baro_t("중복 문서 후보"),
		tool.Baro_t("baro_dup 표에 기록된 중복 의심 짝입니다."),
		tool.Baro_table([]string{"문서", tool.Baro_t("짝"), tool.Baro_t("유사도"), "상태", ""}, rows, tool.Baro_t("중복 후보가 없습니다.")),
		"",
	)
}
