package route

import (
	"database/sql"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  14-3 백업  (/baro/backup)
//
//  일일 · 주간 전체 백업, 목록, 복구 테스트, 스냅샷 현황,
//  다른 저장소 복제, 운영자도 지우기 어려운 보존본, 자동 백업 주기.
// ============================================================

// baro_data_backup_kind_options : 만들 수 있는 백업 종류
func baro_data_backup_kind_options() [][]string {
	return [][]string{
		{"daily", tool.Baro_t("일일 백업")},
		{"weekly", tool.Baro_t("주간 백업")},
		{"manual", tool.Baro_t("수동 백업")},
	}
}

// baro_data_backup_cycle_options : 자동 백업 주기
func baro_data_backup_cycle_options() [][]string {
	return [][]string{
		{"6", tool.Baro_t("6시간마다")},
		{"12", tool.Baro_t("12시간마다")},
		{"24", tool.Baro_t("하루에 한 번 (권장)")},
		{"72", tool.Baro_t("3일에 한 번")},
		{"168", tool.Baro_t("일주일에 한 번")},
	}
}

// baro_data_backup_protected : 보존본인가
func baro_data_backup_protected(row baro_data_backup_row) bool {
	return row.Note["protected"] == "on"
}

// baro_data_copy_file : 파일 복사 (덮어쓰지 않는다)
func baro_data_copy_file(src string, dst string) (int64, error) {
	if !tool.File_exist_check(src) {
		return 0, errors.New(tool.Baro_t("원본 파일이 없습니다 : ") + src)
	}

	if tool.File_exist_check(dst) {
		return 0, errors.New(tool.Baro_t("같은 이름의 파일이 이미 있습니다 : ") + dst)
	}

	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	size, err := io.Copy(out, in)
	if err != nil {
		return 0, err
	}

	if err := out.Sync(); err != nil {
		return size, err
	}

	return size, nil
}

// baro_data_wal_checkpoint : 복사 전에 WAL 을 본 파일로 합친다 (실패해도 넘어간다)
func baro_data_wal_checkpoint(db *sql.DB) {
	defer func() {
		_ = recover()
	}()

	if tool.Get_DB_type() != "sqlite" {
		return
	}

	_, _ = db.Exec("pragma wal_checkpoint(truncate)")
}

// baro_data_backup_make : 전체 백업 만들기 (SQLite 파일 복사)
func baro_data_backup_make(db *sql.DB, config tool.Config, kind string, memo string) (string, error) {
	if tool.Get_DB_type() != "sqlite" {
		return "", errors.New(tool.Baro_t("MySQL 은 파일 복사 백업을 쓸 수 없습니다. 정기 덤프 기능을 쓰세요."))
	}

	src := baro_data_db_file()
	if !tool.File_exist_check(src) {
		return "", errors.New(tool.Baro_t("데이터 파일을 찾지 못했습니다 : ") + src)
	}

	if err := baro_data_ensure_dir(baro_data_backup_dir); err != nil {
		return "", err
	}

	baro_data_wal_checkpoint(db)

	dst := baro_data_path(baro_data_backup_dir, "data_"+kind+"_"+baro_data_stamp()+".db")

	size, err := baro_data_copy_file(src, dst)
	if err != nil {
		return "", err
	}

	checksum, real_size, hash_err := baro_data_file_hash(dst)
	if hash_err != nil {
		checksum = ""
		real_size = size
	}

	note := baro_data_note_put(map[string]string{
		"memo":      memo,
		"protected": "",
		"source":    src,
		"actor":     config.IP,
		"db_type":   tool.Get_DB_type(),
	})

	id := tool.Baro_next_id(db, "baro_backup")
	tool.Exec_DB(
		db,
		"insert into baro_backup (id, kind, path, size, status, date, note, checksum) values (?, ?, ?, ?, 'done', ?, ?, ?)",
		id, kind, dst, tool.Baro_itoa(int(real_size)), tool.Baro_now(), note, checksum,
	)

	tool.Baro_put(db, "backup_last_date", tool.Baro_now())
	tool.Baro_metric_bump(db, "backup_made", tool.Baro_today(), 1)
	tool.Baro_public_log(db, "backup", config.IP, baro_data_kind_name(kind), memo, "전체 백업 생성")

	return id, nil
}

// baro_data_backup_auto : 자동 백업 주기가 지났으면 백업을 수행한다 (14-3)
//
//	설정만 저장해 두고, 다음 접속 때 주기가 지났으면 실제로 만든다.
func baro_data_backup_auto(db *sql.DB, config tool.Config) string {
	if !tool.Baro_get_on(db, "backup_auto") {
		return ""
	}

	if tool.Get_DB_type() != "sqlite" {
		return ""
	}

	cycle := tool.Baro_get_int(db, "backup_cycle_hours", 24)
	if cycle < 1 {
		cycle = 24
	}

	last := tool.Baro_get(db, "backup_last_date")
	if last != "" {
		next := tool.Baro_time_add(last, 0, cycle, 0)
		if !tool.Baro_is_past(next) {
			return ""
		}
	}

	id, err := baro_data_backup_make(db, config, "auto", tool.Baro_t("자동 백업 (주기 ")+tool.Baro_itoa(cycle)+tool.Baro_t("시간)"))
	if err != nil {
		tool.Baro_put(db, "backup_last_error", err.Error())

		return tool.Baro_alert("danger", tool.Baro_t("자동 백업에 실패했습니다 : ")+tool.HTML_escape(err.Error()))
	}

	tool.Baro_put(db, "backup_last_error", "")
	tool.Baro_audit(db, config, "backup_auto", id, "", "", "주기가 지나 자동 백업을 수행했습니다")

	return tool.Baro_alert("ok", tool.Baro_t("자동 백업 주기가 지나 백업을 새로 만들었습니다. (번호 ")+tool.HTML_escape(id)+")")
}

// View_baro_data_backup : 백업 목록 화면
func View_baro_data_backup(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("백업 화면은 시스템 관리 권한이 있는 사람만 볼 수 있습니다."))
	}

	if values != nil {
		return baro_data_backup_post(db, config, values)
	}

	page_num := baro_data_page_num(page)
	offset := (page_num - 1) * baro_data_page_size

	body := baro_data_tabs("/baro/backup")
	body += tool.Baro_hero(
		"백업",
		tool.Baro_t("기여를 잃지 않으려면 백업이 실제로 복구되는지까지 확인해야 합니다."),
		"",
	)

	// 주기가 지났으면 여기서 자동 백업을 수행한다.
	body += baro_data_backup_auto(db, config)

	db_type := tool.Get_DB_type()

	if db_type == "sqlite" {
		body += tool.Baro_alert("info",
			tool.Baro_t(`<b>실시간 백업</b> : SQLite 는 이미 <code>WAL</code>(Write-Ahead Logging) 모드로 열려 있어 `)+
				tool.Baro_t(`쓰기 중에도 데이터가 곧바로 로그 파일에 남습니다. 갑작스러운 종료에도 마지막 정상 상태로 복구됩니다. `)+
				tool.Baro_t(`아래 백업은 그 위에 얹는 <b>시점 사본</b>이며, 만들기 전에 WAL 을 본 파일로 합칩니다.`))
	} else {
		body += tool.Baro_alert("warn",
			tool.Baro_t(`지금 이 위키는 <b>MySQL</b> 을 쓰고 있어 데이터 파일을 그대로 복사할 수 없습니다. `)+
				tool.Baro_t(`MySQL 은 <code>mysqldump</code> 같은 서버 쪽 도구로 백업하고, 공개 데이터는 `)+
				tool.Baro_t(`<a href="/baro/export/dump">정기 데이터 덤프</a> 로 배포하세요.`))
	}

	// --- 현황 ---
	src := baro_data_db_file()
	src_size := int64(-1)
	if info, err := os.Stat(src); err == nil {
		src_size = info.Size()
	}

	total_count, protected_count, total_size := baro_data_backup_summary(db)
	last := tool.Baro_get(db, "backup_last_date")

	auto_text := tool.Baro_t("꺼짐")
	if tool.Baro_get_on(db, "backup_auto") {
		auto_text = tool.Baro_itoa(tool.Baro_get_int(db, "backup_cycle_hours", 24)) + "시간마다"
	}

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("백업 수"), tool.Baro_comma(total_count), tool.Baro_t("보존본 ") + tool.Baro_itoa(protected_count) + tool.Baro_t("건")},
		{tool.Baro_t("백업 총 용량"), baro_data_size_text(total_size), ""},
		{tool.Baro_t("원본 크기"), baro_data_size_text(src_size), src},
		{tool.Baro_t("마지막 백업"), tool.Baro_time_ago(last), last},
		{tool.Baro_t("자동 백업"), auto_text, tool.Baro_t("다음 접속 때 주기 확인")},
	})

	if err_text := tool.Baro_get(db, "backup_last_error"); err_text != "" {
		body += tool.Baro_alert("danger", tool.Baro_t("지난 자동 백업 오류 : ")+tool.HTML_escape(err_text))
	}

	// --- 만들기 ---
	make_form := tool.Baro_select("kind", tool.Baro_t("백업 종류"), baro_data_backup_kind_options(), "daily", tool.Baro_t("일일 · 주간 전체 백업을 구분해 남깁니다."))
	make_form += tool.Baro_input("memo", tool.Baro_t("메모"), "", tool.Baro_t("예) 정기 점검 전 백업"), tool.Baro_t("무엇을 위한 백업인지 적어 두면 나중에 찾기 쉽습니다."))
	make_form += tool.Baro_hidden("action", "make")
	make_form += tool.Baro_submit(tool.Baro_t("지금 백업 만들기"), "primary")

	if db_type == "sqlite" {
		body += tool.Baro_card_ex(
			tool.Baro_t("전체 백업 만들기"),
			tool.Baro_t("data/backup/ 아래에 데이터 파일 사본을 만듭니다."),
			tool.Baro_form("/baro/backup", make_form),
			tool.Baro_note(tool.Baro_t("백업 파일에는 IP·이메일을 포함한 모든 운영 데이터가 들어 있습니다. 외부에 공개하지 마세요.")),
		)
	} else {
		// MySQL 은 파일 복사가 불가능하므로 만들기 단추 대신 안내를 둔다.
		body += tool.Baro_card_ex(
			tool.Baro_t("전체 백업 만들기"),
			tool.Baro_t("이 위키에서는 파일 복사 방식의 전체 백업을 쓸 수 없습니다."),
			tool.Baro_alert("warn",
				tool.Baro_t("MySQL 은 데이터가 서버 안에 있어 이 화면에서 파일을 복사할 수 없습니다. ")+
					tool.Baro_t("전체 백업은 서버에서 <code>mysqldump</code> 로 뜨고, 공개 데이터는 정기 덤프로 배포하세요."))+
				tool.Baro_list([]string{
					tool.Baro_t("전체 백업 : <code>mysqldump -u &lt;사용자&gt; -p &lt;데이터베이스&gt; &gt; backup.sql</code>"),
					tool.Baro_t("공개 데이터 배포 : 아래 <b>정기 데이터 덤프</b> (민감 정보 제외, JSON Lines)"),
					tool.Baro_t("만들어 둔 덤프 파일은 이 화면의 목록에서도 무결성을 확인할 수 있습니다."),
				})+
				tool.Baro_button("/baro/export/dump", tool.Baro_t("정기 데이터 덤프 만들기"), "primary"),
			tool.Baro_note(tool.Baro_t("아래 목록에는 이미 만들어 둔 덤프와 지난 백업 기록이 그대로 남습니다.")),
		)
	}

	body += tool.Baro_card(
		tool.Baro_t("설정과 다른 화면"),
		tool.Baro_button("/baro/backup/set", tool.Baro_t("자동 백업 주기 · 복제 대상 설정"), "")+" "+
			tool.Baro_button("/baro/backup/snapshot", tool.Baro_t("대량 삭제 전 스냅샷 현황"), "")+" "+
			tool.Baro_button("/baro/export/dump", tool.Baro_t("공개 덤프 (민감 정보 제외)"), ""),
	)

	// --- 목록 ---
	list := baro_data_backup_list(db, "", offset, baro_data_page_size)
	rows := [][]string{}
	is_owner := tool.Baro_is_owner(db, config.IP)

	for _, row := range list {
		exists := tool.File_exist_check(row.Path)
		protected := baro_data_backup_protected(row)

		kind_html := tool.HTML_escape(baro_data_kind_name(row.Kind))
		if protected {
			kind_html += " " + tool.Baro_badge(tool.Baro_t("보존본"), "info")
		}

		path_html := tool.Baro_code(row.Path)
		if !exists {
			path_html += tool.Baro_badge(tool.Baro_t("파일 없음"), "danger")
		}

		checksum_html := `<span class="bw_muted">-</span>`
		if row.Checksum != "" {
			checksum_html = `<code title="` + tool.HTML_escape(row.Checksum) + `">` + tool.HTML_escape(tool.Get_slice(row.Checksum, 0, 12)) + `…</code>`
		}

		status_kind := "ok"
		if row.Status != "done" {
			status_kind = "warn"
		}

		buttons := tool.Baro_button("/baro/backup/verify?id="+tool.Url_parser(row.ID), "복구 테스트", "")

		if is_owner && exists && row.Kind != "dump" {
			buttons += " " + tool.Baro_button("/baro/backup/file?id="+tool.Url_parser(row.ID), "내려받기", "")
		}

		if row.Kind == "dump" {
			buttons += " " + tool.Baro_button("/baro/export/dump/file?id="+tool.Url_parser(row.ID), "내려받기", "")
		}

		// 보존본은 화면에서 삭제 버튼을 감춘다. (지우려면 아래 위험 구역 + 소유자 권한)
		if !protected {
			buttons += " " + tool.Baro_button("/baro/backup/verify?id="+tool.Url_parser(row.ID)+"#delete", "삭제", "danger")
		}

		rows = append(rows, []string{
			tool.HTML_escape(row.ID),
			kind_html,
			path_html,
			tool.HTML_escape(baro_data_size_text(baro_data_atoi64(row.Size))),
			tool.Baro_time_cell(row.Date),
			checksum_html,
			tool.Baro_badge(row.Status, status_kind),
			buttons,
		})
	}

	table := tool.Baro_table(
		[]string{"번호", "종류", tool.Baro_t("경로"), tool.Baro_t("크기"), "시각", tool.Baro_t("체크섬(SHA224)"), "상태", "관리"},
		rows,
		tool.Baro_t("아직 만들어진 백업이 없습니다."),
	)

	if len(rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("아직 만들어진 백업이 없습니다. 위에서 첫 백업을 만들어 보세요."))
	}

	table += tool.Baro_paging(page_num, len(rows), baro_data_page_size, "/baro/backup/list/{}")

	body += tool.Baro_card_ex(tool.Baro_t("백업 목록"), tool.Baro_t("종류 · 경로 · 크기 · 시각 · 체크섬 · 상태"), table, "")

	return tool.Baro_page(db, config, "백업", body, baro_data_menu())
}

// baro_data_backup_summary : 백업 수 / 보존본 수 / 총 용량
func baro_data_backup_summary(db *sql.DB) (int, int, int64) {
	count := 0
	protected := 0
	size := int64(0)

	rows := tool.Query_DB(db, "select size, note from baro_backup")
	defer rows.Close()

	for rows.Next() {
		size_text, note := "", ""
		if err := rows.Scan(&size_text, &note); err != nil {
			continue
		}

		count++
		size += baro_data_atoi64(size_text)

		if baro_data_note_get(note)["protected"] == "on" {
			protected++
		}
	}

	return count, protected, size
}

// baro_data_backup_post : 백업 화면의 쓰기 동작
func baro_data_backup_post(db *sql.DB, config tool.Config, values url.Values) string {
	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("백업을 다룰 권한이 없습니다."))
	}

	action := baro_data_q(values, "action")

	switch action {
	case "make":
		kind := baro_data_q(values, "kind")
		allow := false
		for _, row := range baro_data_backup_kind_options() {
			if row[0] == kind {
				allow = true
			}
		}

		if !allow {
			kind = "manual"
		}

		id, err := baro_data_backup_make(db, config, kind, baro_data_q(values, "memo"))
		if err != nil {
			return baro_data_error_page(db, config, "백업", tool.Baro_t("백업을 만들지 못했습니다 : ")+err.Error())
		}

		tool.Baro_audit(db, config, "backup_make", id, "", kind, baro_data_q(values, "memo"))
		tool.Baro_notify(db, config.IP, "system", "백업을 만들었습니다", "/baro/backup/verify?id="+tool.Url_parser(id), baro_data_kind_name(kind))

		return tool.Get_redirect("/baro/backup/verify?id=" + tool.Url_parser(id))
	case "protect":
		return baro_data_backup_protect(db, config, values)
	case "replicate":
		return baro_data_backup_replicate(db, config, values)
	case "delete":
		return baro_data_backup_delete(db, config, values)
	}

	return tool.Get_redirect("/baro/backup")
}

// baro_data_backup_protect : 보존 표시 켜고 끄기
func baro_data_backup_protect(db *sql.DB, config tool.Config, values url.Values) string {
	id := baro_data_q(values, "id")
	row, found := baro_data_backup_get(db, id)
	if !found {
		return baro_data_error_page(db, config, "백업", tool.Baro_t("그런 백업이 없습니다 : ")+id)
	}

	before := "off"
	if baro_data_backup_protected(row) {
		before = "on"
	}

	// 보존 표시를 푸는 것은 소유자만 할 수 있다.
	if before == "on" && !tool.Baro_is_owner(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("보존본 표시를 푸는 것은 소유자만 할 수 있습니다."))
	}

	note := row.Note
	if note == nil {
		note = map[string]string{}
	}

	after := "on"
	if before == "on" {
		after = ""
	}

	note["protected"] = after
	note["protect_by"] = config.IP
	note["protect_date"] = tool.Baro_now()

	tool.Exec_DB(db, "update baro_backup set note = ? where id = ?", baro_data_note_put(note), id)

	tool.Baro_audit(db, config, "backup_protect", id, before, after, baro_data_q(values, "reason"))
	tool.Baro_public_log(db, "backup_protect", config.IP, "백업 "+id, baro_data_q(values, "reason"), "보존 표시 "+after)

	return tool.Get_redirect("/baro/backup/verify?id=" + tool.Url_parser(id))
}

// baro_data_backup_replicate : 다른 저장소(복제 대상 경로)로 복사
func baro_data_backup_replicate(db *sql.DB, config tool.Config, values url.Values) string {
	id := baro_data_q(values, "id")
	row, found := baro_data_backup_get(db, id)
	if !found {
		return baro_data_error_page(db, config, "백업", "그런 백업이 없습니다 : "+id)
	}

	target_dir := tool.Baro_get(db, "backup_replica_path")
	if strings.TrimSpace(target_dir) == "" {
		return baro_data_error_page(db, config, "백업", "복제 대상 경로가 설정돼 있지 않습니다. 설정 화면에서 먼저 지정하세요.")
	}

	if !tool.File_exist_check(row.Path) {
		return baro_data_error_page(db, config, "백업", "원본 백업 파일이 없어 복제할 수 없습니다.")
	}

	if err := baro_data_ensure_dir(target_dir); err != nil {
		return baro_data_error_page(db, config, "백업", "복제 대상 폴더를 만들지 못했습니다 : "+err.Error())
	}

	dst := baro_data_path(target_dir, filepath.Base(row.Path))

	size, err := baro_data_copy_file(row.Path, dst)
	if err != nil {
		return baro_data_error_page(db, config, "백업", "복제하지 못했습니다 : "+err.Error())
	}

	checksum, real_size, hash_err := baro_data_file_hash(dst)
	if hash_err != nil {
		checksum = row.Checksum
		real_size = size
	}

	note := baro_data_note_put(map[string]string{
		"memo":      "백업 " + id + " 의 복제본",
		"protected": "on",
		"origin":    row.ID,
		"source":    row.Path,
		"actor":     config.IP,
	})

	new_id := tool.Baro_next_id(db, "baro_backup")
	tool.Exec_DB(
		db,
		"insert into baro_backup (id, kind, path, size, status, date, note, checksum) values (?, 'replica', ?, ?, 'done', ?, ?, ?)",
		new_id, dst, tool.Baro_itoa(int(real_size)), tool.Baro_now(), note, checksum,
	)

	same := "체크섬 일치"
	if row.Checksum != "" && checksum != "" && row.Checksum != checksum {
		same = "체크섬 불일치 — 다시 복제하세요"
	}

	tool.Baro_audit(db, config, "backup_replicate", new_id, row.Path, dst, same)
	tool.Baro_public_log(db, "backup_replicate", config.IP, "백업 "+id, "별도 저장소 복제", same)

	return tool.Get_redirect("/baro/backup/verify?id=" + tool.Url_parser(new_id))
}

// baro_data_backup_delete : 백업 삭제 (사유 필수 / 보존본은 소유자만)
func baro_data_backup_delete(db *sql.DB, config tool.Config, values url.Values) string {
	id := baro_data_q(values, "id")
	reason := baro_data_q(values, "reason")

	row, found := baro_data_backup_get(db, id)
	if !found {
		return baro_data_error_page(db, config, "백업", tool.Baro_t("그런 백업이 없습니다 : ")+id)
	}

	if tool.Get_len(reason) < 5 {
		return baro_data_error_page(db, config, "백업", tool.Baro_t("백업을 지우려면 사유를 5자 이상 적어야 합니다."))
	}

	if baro_data_backup_protected(row) && !tool.Baro_is_owner(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("보존본으로 표시된 백업은 소유자만 지울 수 있습니다. 운영자라도 임의로 지울 수 없습니다."))
	}

	// 파일이 남아 있으면 함께 지운다. (백업 폴더 밖의 경로는 건드리지 않는다)
	clean := filepath.ToSlash(filepath.Clean(row.Path))
	inside := strings.HasPrefix(clean, baro_data_backup_dir+"/") || strings.HasPrefix(clean, baro_data_dump_dir+"/")

	if inside && tool.File_exist_check(row.Path) {
		if err := os.Remove(row.Path); err != nil {
			return baro_data_error_page(db, config, "백업", tool.Baro_t("백업 파일을 지우지 못했습니다 : ")+err.Error())
		}
	}

	tool.Exec_DB(db, "delete from baro_backup where id = ?", id)

	tool.Baro_audit(db, config, "backup_delete", id, row.Path, "", reason)
	tool.Baro_public_log(db, "backup_delete", config.IP, "백업 "+id, reason, baro_data_kind_name(row.Kind))

	return tool.Get_redirect("/baro/backup")
}

// View_baro_data_backup_verify : 복구 테스트 (파일 존재 + 크기 + 체크섬 재계산 비교)
func View_baro_data_backup_verify(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("복구 테스트는 시스템 관리 권한이 있는 사람만 할 수 있습니다."))
	}

	if values != nil {
		return baro_data_backup_post(db, config, values)
	}

	id := baro_data_q(query, "id")
	row, found := baro_data_backup_get(db, id)

	body := baro_data_tabs("/baro/backup")
	body += tool.Baro_crumb([][]string{{"/baro/backup", "백업"}, {"", "복구 테스트"}})

	if !found {
		body += tool.Baro_alert("warn", tool.Baro_t("그런 백업이 없습니다."))
		body += tool.Baro_button("/baro/backup", tool.Baro_t("백업 목록으로"), "primary")

		return tool.Baro_page(db, config, "복구 테스트", body, baro_data_menu())
	}

	protected := baro_data_backup_protected(row)

	// --- 1. 파일 존재 ---
	exists := tool.File_exist_check(row.Path)
	checks := [][]string{}

	if exists {
		checks = append(checks, []string{tool.Baro_t("파일 존재"), tool.Baro_badge(tool.Baro_t("정상"), "ok")})
	} else {
		checks = append(checks, []string{tool.Baro_t("파일 존재"), tool.Baro_badge(tool.Baro_t("파일 없음"), "danger")})
	}

	// --- 2. 크기 비교 ---
	real_size := int64(-1)
	if info, err := os.Stat(row.Path); err == nil {
		real_size = info.Size()
	}

	saved_size := baro_data_atoi64(row.Size)
	size_html := tool.Baro_badge(tool.Baro_t("확인 불가"), "muted")
	if real_size >= 0 {
		if real_size == saved_size {
			size_html = tool.Baro_badge(tool.Baro_t("일치"), "ok") + " " + tool.HTML_escape(baro_data_size_text(real_size))
		} else {
			size_html = tool.Baro_badge(tool.Baro_t("불일치"), "danger") + tool.Baro_t(" 기록 ") + tool.HTML_escape(baro_data_size_text(saved_size)) +
				tool.Baro_t(" / 실제 ") + tool.HTML_escape(baro_data_size_text(real_size))
		}
	}

	checks = append(checks, []string{tool.Baro_t("크기 비교"), size_html})

	// --- 3. 체크섬 재계산 ---
	checksum_html := tool.Baro_badge(tool.Baro_t("확인 불가"), "muted")
	new_checksum := ""
	if exists {
		var err error
		new_checksum, _, err = baro_data_file_hash(row.Path)
		if err != nil {
			checksum_html = tool.Baro_badge(tool.Baro_t("읽기 실패"), "danger")
		} else if row.Checksum == "" {
			checksum_html = tool.Baro_badge(tool.Baro_t("기록 없음"), "warn") + tool.Baro_t(" 새로 계산 : <code>") + tool.HTML_escape(new_checksum) + "</code>"
		} else if row.Checksum == new_checksum {
			checksum_html = tool.Baro_badge(tool.Baro_t("일치"), "ok") + " <code>" + tool.HTML_escape(new_checksum) + "</code>"
		} else {
			checksum_html = tool.Baro_badge(tool.Baro_t("불일치 — 손상 의심"), "danger") +
				tool.Baro_t(" 기록 <code>") + tool.HTML_escape(row.Checksum) + tool.Baro_t("</code> / 실제 <code>") + tool.HTML_escape(new_checksum) + "</code>"
		}
	}

	checks = append(checks, []string{tool.Baro_t("체크섬(SHA224) 재계산"), checksum_html})

	// --- 4. 실제로 열어 보기 ---
	open_html := tool.Baro_badge(tool.Baro_t("건너뜀"), "muted")
	if exists && strings.HasSuffix(strings.ToLower(row.Path), ".db") {
		doc_count, open_err := baro_data_backup_open_test(row.Path)
		if open_err != nil {
			open_html = tool.Baro_badge(tool.Baro_t("열지 못함"), "danger") + " " + tool.HTML_escape(open_err.Error())
		} else {
			open_html = tool.Baro_badge(tool.Baro_t("정상"), "ok") + " 문서 " + tool.Baro_comma(doc_count) + tool.Baro_t("개를 읽었습니다.")
		}
	} else if exists {
		lines, open_err := baro_data_dump_line_test(row.Path)
		if open_err != nil {
			open_html = tool.Baro_badge(tool.Baro_t("열지 못함"), "danger") + " " + tool.HTML_escape(open_err.Error())
		} else {
			open_html = tool.Baro_badge(tool.Baro_t("정상"), "ok") + " JSON " + tool.Baro_comma(lines) + tool.Baro_t("줄을 읽었습니다.")
		}
	}

	checks = append(checks, []string{tool.Baro_t("파일 열어 보기"), open_html})

	body += tool.Baro_card_ex(
		tool.Baro_t("복구 테스트 결과 — 백업 ")+row.ID,
		tool.Baro_t("백업이 있다는 것만으로는 부족합니다. 실제로 열리는지까지 확인합니다."),
		tool.Baro_dl(checks),
		"",
	)

	// --- 백업 정보 ---
	protected_html := tool.Baro_t("아니요")
	if protected {
		protected_html = tool.Baro_badge(tool.Baro_t("보존본"), "info") + tool.Baro_t(" 운영자도 임의로 지울 수 없습니다.")
	}

	body += tool.Baro_card(tool.Baro_t("백업 정보"), tool.Baro_kv_grid([][]string{
		{"종류", tool.HTML_escape(baro_data_kind_name(row.Kind))},
		{tool.Baro_t("경로"), tool.Baro_code(row.Path)},
		{tool.Baro_t("기록된 크기"), tool.HTML_escape(baro_data_size_text(saved_size))},
		{tool.Baro_t("만든 시각"), tool.HTML_escape(row.Date)},
		{"상태", tool.Baro_badge(row.Status, "ok")},
		{"보존 표시", protected_html},
		{tool.Baro_t("메모"), tool.HTML_escape(row.Note["memo"])},
		{tool.Baro_t("원본"), tool.HTML_escape(row.Note["source"])},
	}))

	// --- 조치 ---
	protect_form := tool.Baro_hidden("action", "protect") + tool.Baro_hidden("id", row.ID)
	protect_form += tool.Baro_input("reason", "사유", "", tool.Baro_t("예) 창립 시즌 종료 시점 보존"), "")

	if protected {
		protect_form += tool.Baro_submit(tool.Baro_t("보존 표시 풀기 (소유자만)"), "danger")
	} else {
		protect_form += tool.Baro_submit(tool.Baro_t("보존본으로 표시하기"), "primary")
	}

	replica_path := tool.Baro_get(db, "backup_replica_path")
	replica_body := ""
	if strings.TrimSpace(replica_path) == "" {
		replica_body = tool.Baro_alert("warn", tool.Baro_t("복제 대상 경로가 없습니다.")) +
			tool.Baro_button("/baro/backup/set", tool.Baro_t("복제 대상 설정하기"), "")
	} else {
		replica_form := tool.Baro_hidden("action", "replicate") + tool.Baro_hidden("id", row.ID)
		replica_form += tool.Baro_submit(tool.Baro_t("복제 대상으로 복사하기"), "")
		replica_body = tool.Baro_note(tool.Baro_t("복제 대상 : ")+replica_path) + tool.Baro_form("/baro/backup/verify", replica_form)
	}

	body += tool.Baro_grid([]string{
		tool.Baro_card(tool.Baro_t("보존본 표시"), tool.Baro_form("/baro/backup/verify", protect_form)),
		tool.Baro_card(tool.Baro_t("다른 저장소로 복제"), replica_body),
	}, 2)

	// --- 삭제 (위험 구역) ---
	if protected {
		body += tool.Baro_danger_zone(tool.Baro_t("백업 삭제"),
			tool.Baro_alert("danger", tool.Baro_t("이 백업은 <b>보존본</b>입니다. 목록에서 삭제 버튼을 감췄고, 지우려면 소유자 권한과 사유가 필요합니다."))+
				baro_data_backup_delete_form(db, config, row.ID))
	} else {
		body += `<div id="delete"></div>`
		body += tool.Baro_danger_zone(tool.Baro_t("백업 삭제"),
			tool.Baro_alert("warn", tool.Baro_t("지운 백업은 되돌릴 수 없습니다. 사유를 5자 이상 적어야 합니다."))+
				baro_data_backup_delete_form(db, config, row.ID))
	}

	body += tool.Baro_button("/baro/backup", tool.Baro_t("백업 목록으로"), "")

	return tool.Baro_page(db, config, "복구 테스트", body, baro_data_menu())
}

// baro_data_backup_delete_form : 삭제 폼
func baro_data_backup_delete_form(db *sql.DB, config tool.Config, id string) string {
	if !tool.Baro_is_owner(db, config.IP) {
		row, _ := baro_data_backup_get(db, id)
		if baro_data_backup_protected(row) {
			return tool.Baro_note(tool.Baro_t("소유자 계정으로만 지울 수 있습니다."))
		}
	}

	form := tool.Baro_hidden("action", "delete") + tool.Baro_hidden("id", id)
	form += tool.Baro_textarea("reason", tool.Baro_t("삭제 사유 (필수)"), "", 3, tool.Baro_t("왜 이 백업을 지우는지 적어 주세요."), tool.Baro_t("공개 운영 로그와 내부 감사 기록에 함께 남습니다."))
	form += tool.Baro_submit(tool.Baro_t("이 백업을 지웁니다"), "danger")

	return tool.Baro_form("/baro/backup/verify", form)
}

// baro_data_backup_open_test : 백업 파일을 실제로 열어 문서 수를 세어 본다
func baro_data_backup_open_test(path string) (count int, err error) {
	defer func() {
		if r := recover(); r != nil {
			count = 0
			err = errors.New(tool.Baro_t("백업 파일을 여는 중 오류가 났습니다"))
		}
	}()

	backup_db, open_err := sql.Open("sqlite", path+"?mode=ro")
	if open_err != nil {
		return 0, open_err
	}
	defer backup_db.Close()

	if ping_err := backup_db.Ping(); ping_err != nil {
		return 0, ping_err
	}

	value := 0
	if scan_err := backup_db.QueryRow("select count(*) from data").Scan(&value); scan_err != nil {
		return 0, scan_err
	}

	return value, nil
}

// baro_data_dump_line_test : 덤프 파일 줄 수 세기
func baro_data_dump_line_test(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}

	return count, nil
}

// View_baro_data_backup_set : 자동 백업 주기 · 복제 대상 설정
func View_baro_data_backup_set(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("백업 설정은 시스템 관리 권한이 있는 사람만 바꿀 수 있습니다."))
	}

	if values != nil {
		before := tool.Baro_json_enc(map[string]string{
			"backup_auto":         tool.Baro_get(db, "backup_auto"),
			"backup_cycle_hours":  tool.Baro_get(db, "backup_cycle_hours"),
			"backup_replica_path": tool.Baro_get(db, "backup_replica_path"),
			"dump_max_docs":       tool.Baro_get(db, "dump_max_docs"),
		})

		auto := ""
		if baro_data_q(values, "backup_auto") == "on" {
			auto = "on"
		}

		cycle := tool.Str_to_int(baro_data_q(values, "backup_cycle_hours"))
		if cycle < 1 {
			cycle = 24
		}

		replica := baro_data_q(values, "backup_replica_path")

		max_docs := tool.Str_to_int(baro_data_q(values, "dump_max_docs"))
		if max_docs < 10 {
			max_docs = 300
		}

		tool.Baro_put(db, "backup_auto", auto)
		tool.Baro_put(db, "backup_cycle_hours", tool.Baro_itoa(cycle))
		tool.Baro_put(db, "backup_replica_path", replica)
		tool.Baro_put(db, "dump_max_docs", tool.Baro_itoa(max_docs))

		after := tool.Baro_json_enc(map[string]string{
			"backup_auto":         auto,
			"backup_cycle_hours":  tool.Baro_itoa(cycle),
			"backup_replica_path": replica,
			"dump_max_docs":       tool.Baro_itoa(max_docs),
		})

		tool.Baro_audit(db, config, "backup_set", "backup", before, after, "백업 설정 변경")
		tool.Baro_public_log(db, "backup_set", config.IP, "백업 설정", "자동 백업 주기 · 복제 대상 변경", "")

		return tool.Get_redirect("/baro/backup/set")
	}

	body := baro_data_tabs("/baro/backup")
	body += tool.Baro_crumb([][]string{{"/baro/backup", "백업"}, {"", "설정"}})

	body += tool.Baro_alert("info",
		tool.Baro_t(`자동 백업은 별도 예약 프로그램 없이 동작합니다. 설정한 주기가 지난 뒤 <b>이 백업 화면에 처음 들어올 때</b> `)+
			tool.Baro_t(`백업을 한 번 수행합니다. 접속이 없으면 백업도 미뤄집니다.`))

	form := tool.Baro_checkbox("backup_auto", "자동 백업 사용", "on", tool.Baro_get_on(db, "backup_auto"), "주기가 지났으면 다음 접속 때 백업합니다.")
	form += tool.Baro_select("backup_cycle_hours", tool.Baro_t("자동 백업 주기"),
		baro_data_backup_cycle_options(), tool.Baro_itoa(tool.Baro_get_int(db, "backup_cycle_hours", 24)), "")
	form += tool.Baro_input("backup_replica_path", "복제 대상 경로", tool.Baro_get(db, "backup_replica_path"),
		tool.Baro_t("예) D:/wiki_backup 또는 /mnt/nas/baro"), tool.Baro_t("다른 공급자·다른 디스크를 가리키게 하면 한 곳이 망가져도 사본이 남습니다."))
	form += tool.Baro_input_type("dump_max_docs", tool.Baro_t("공개 덤프 회차당 문서 수"),
		tool.Baro_itoa(tool.Baro_get_int(db, "dump_max_docs", 300)), "number", "10 ~ 5000")
	form += tool.Baro_submit(tool.Baro_t("설정 저장"), "primary")

	body += tool.Baro_card_ex(tool.Baro_t("자동 백업과 복제 대상"), tool.Baro_t("설정 값만 저장하고, 다음 접속 때 주기를 확인합니다."), tool.Baro_form("/baro/backup/set", form), "")

	last := tool.Baro_get(db, "backup_last_date")
	next := "-"
	if last != "" {
		next = tool.Baro_time_add(last, 0, tool.Baro_get_int(db, "backup_cycle_hours", 24), 0)
	}

	body += tool.Baro_card(tool.Baro_t("현재 상태"), tool.Baro_kv_grid([][]string{
		{tool.Baro_t("데이터베이스 종류"), tool.HTML_escape(tool.Get_DB_type())},
		{tool.Baro_t("데이터 파일"), tool.Baro_code(baro_data_db_file())},
		{tool.Baro_t("WAL 실시간 로그"), tool.Baro_badge(tool.Baro_t("SQLite 기본 켜짐"), "ok")},
		{tool.Baro_t("마지막 백업"), tool.HTML_escape(last)},
		{tool.Baro_t("다음 예정"), tool.HTML_escape(next)},
		{"복제 대상", tool.Baro_code(tool.Baro_get_def(db, "backup_replica_path", "(설정 없음)"))},
	}))

	body += tool.Baro_button("/baro/backup", tool.Baro_t("백업 목록으로"), "")

	return tool.Baro_page(db, config, "백업 설정", body, baro_data_menu())
}

// View_baro_data_backup_snapshot : 대량 삭제 전 스냅샷 현황 (0장 3번 원칙)
func View_baro_data_backup_snapshot(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "system") {
		return tool.Baro_deny(db, config, tool.Baro_t("스냅샷 현황은 시스템 관리 권한이 있는 사람만 볼 수 있습니다."))
	}

	page_num := baro_data_page_num(page)
	offset := (page_num - 1) * baro_data_page_size

	body := baro_data_tabs("/baro/backup/snapshot")
	body += tool.Baro_hero(
		tool.Baro_t("대량 작업 스냅샷"),
		tool.Baro_t("대량 삭제·이동 전에 만들어 둔 복구용 스냅샷이 얼마나 쌓였는지 봅니다."),
		"",
	)

	body += tool.Baro_alert("info",
		tool.Baro_t("0장 3번 원칙에 따라 대량 작업은 실행 전에 <b>복구 가능한 스냅샷</b>을 남깁니다. ")+
			tool.Baro_t("스냅샷은 <code>baro_snapshot</code> 에 쌓이며, 여기서는 현황만 보여 줍니다. ")+
			tool.Baro_t("되돌리기는 대량 작업 화면에서 합니다."))

	// 총계
	total := "0"
	tool.QueryRow_DB(db, "select count(*) from baro_snapshot", []any{&total})

	mass_total := "0"
	tool.QueryRow_DB(db, "select count(*) from baro_mass_op", []any{&mass_total})

	size_total := 0
	size_rows := tool.Query_DB(db, "select data from baro_snapshot limit 5000")
	for size_rows.Next() {
		data := ""
		if err := size_rows.Scan(&data); err != nil {
			continue
		}

		size_total += len(data)
	}
	size_rows.Close()

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("스냅샷 수"), tool.Baro_comma(tool.Str_to_int(total)), ""},
		{tool.Baro_t("대량 작업 수"), tool.Baro_comma(tool.Str_to_int(mass_total)), "baro_mass_op"},
		{tool.Baro_t("본문 용량"), baro_data_size_text(int64(size_total)), tool.Baro_t("최근 5000건 기준")},
		{tool.Baro_t("대량 기준"), tool.Baro_itoa(tool.Baro_mass_threshold(db)) + tool.Baro_t("개"), tool.Baro_t("이보다 크면 대량 작업")},
	})

	// 대량 작업별 묶음
	rows := [][]string{}
	list := tool.Query_DB(
		db,
		"select mass_id, count(*), max(date) from baro_snapshot group by mass_id order by max(date) desc limit ?, ?",
		offset, baro_data_page_size,
	)
	defer list.Close()

	for list.Next() {
		mass_id, count, last_date := "", "", ""
		if err := list.Scan(&mass_id, &count, &last_date); err != nil {
			continue
		}

		kind, actor, status, num, reason := "", "", "", "", ""
		tool.QueryRow_DB(
			db,
			"select kind, actor, status, num, reason from baro_mass_op where id = ?",
			[]any{&kind, &actor, &status, &num, &reason}, mass_id,
		)

		status_kind := "muted"
		switch status {
		case "done":
			status_kind = "ok"
		case "pending":
			status_kind = "warn"
		case "rejected":
			status_kind = "danger"
		}

		rows = append(rows, []string{
			tool.HTML_escape(mass_id),
			tool.HTML_escape(tool.Baro_mass_kind_name(kind)),
			tool.Baro_user_link(db, actor, config.IP),
			tool.Baro_comma(tool.Str_to_int(count)) + " 건",
			tool.HTML_escape(tool.Baro_trunc(reason, 40)),
			tool.Baro_badge(tool.Choose(status, "-"), status_kind),
			tool.Baro_time_cell(last_date),
		})
	}

	table := tool.Baro_table(
		[]string{tool.Baro_t("대량 작업 번호"), "종류", tool.Baro_t("실행자"), tool.Baro_t("스냅샷"), "사유", "상태", tool.Baro_t("마지막 시각")},
		rows,
		tool.Baro_t("아직 스냅샷이 없습니다."),
	)

	if len(rows) == 0 {
		table = tool.Baro_empty(tool.Baro_t("아직 쌓인 스냅샷이 없습니다. 대량 작업이 실행되면 여기에 나타납니다."))
	}

	table += tool.Baro_paging(page_num, len(rows), baro_data_page_size, "/baro/backup/snapshot/{}")

	body += tool.Baro_card_ex(tool.Baro_t("대량 작업별 스냅샷"), tool.Baro_t("되돌릴 수 있는 상태가 남아 있는지 확인하세요."), table, "")

	// 스냅샷이 없는 대량 작업을 찾아 보여 준다.
	// (스냅샷만 훑으면 "스냅샷을 아예 남기지 않은 작업" 이 화면에서 사라진다)
	body += baro_data_snapshot_gap(db, config)

	body += tool.Baro_button("/baro/backup", tool.Baro_t("백업 목록으로"), "")

	return tool.Baro_page(db, config, tool.Baro_t("대량 작업 스냅샷"), body, baro_data_menu())
}

// baro_data_snapshot_gap : 스냅샷 보유 여부로 본 대량 작업 목록 (0장 3번 원칙 점검)
func baro_data_snapshot_gap(db *sql.DB, config tool.Config) string {
	rows := [][]string{}
	missing := 0

	list := tool.Query_DB(
		db,
		"select id, kind, actor, num, status, reason, created, execute_date, undo_status from baro_mass_op order by id + 0 desc limit 100",
	)
	defer list.Close()

	for list.Next() {
		id, kind, actor, num, status, reason, created, execute_date, undo_status := "", "", "", "", "", "", "", "", ""
		if err := list.Scan(&id, &kind, &actor, &num, &status, &reason, &created, &execute_date, &undo_status); err != nil {
			continue
		}

		count := "0"
		tool.QueryRow_DB(db, "select count(*) from baro_snapshot where mass_id = ?", []any{&count}, id)

		snap_html := tool.Baro_badge(tool.Baro_comma(tool.Str_to_int(count))+tool.Baro_t("건"), "ok")
		if tool.Str_to_int(count) == 0 {
			missing++

			if status == "done" {
				snap_html = tool.Baro_badge(tool.Baro_t("없음 — 되돌릴 수 없습니다"), "danger")
			} else {
				snap_html = tool.Baro_badge(tool.Baro_t("아직 없음"), "warn")
			}
		}

		when := execute_date
		if when == "" {
			when = created
		}

		undo_html := `<span class="bw_muted">-</span>`
		if undo_status != "" {
			undo_html = tool.Baro_badge(undo_status, "info")
		}

		rows = append(rows, []string{
			tool.HTML_escape(id),
			tool.HTML_escape(tool.Baro_mass_kind_name(kind)),
			tool.Baro_user_link(db, actor, config.IP),
			tool.Baro_comma(tool.Str_to_int(num)) + " 건",
			tool.Baro_badge(tool.Choose(status, "-"), "muted"),
			snap_html,
			undo_html,
			tool.Baro_time_cell(when),
		})
	}

	table := tool.Baro_table(
		[]string{"번호", "종류", tool.Baro_t("실행자"), tool.Baro_t("대상 수"), "상태", tool.Baro_t("스냅샷"), tool.Baro_t("되돌리기"), "시각"},
		rows,
		tool.Baro_t("등록된 대량 작업이 없습니다."),
	)

	head := tool.Baro_note(tool.Baro_t("대량 작업은 실행 전에 스냅샷을 남겨야 합니다. 스냅샷이 없는 실행 완료 건은 되돌릴 수 없으니 확인이 필요합니다."))
	if missing > 0 {
		head = tool.Baro_alert("warn",
			tool.Baro_t("스냅샷이 없는 대량 작업이 <b>")+tool.Baro_itoa(missing)+tool.Baro_t("건</b> 있습니다. 실행이 끝난 건이라면 되돌리기가 불가능합니다."))
	}

	return tool.Baro_card_ex(tool.Baro_t("대량 작업 스냅샷 보유 점검"), tool.Baro_t("스냅샷을 남기지 않은 작업이 있는지 봅니다."), head+table, "")
}

// View_baro_data_backup_file : 백업 파일 내려받기 (소유자만)
func View_baro_data_backup_file(config tool.Config, query url.Values) (string, string, string) {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_is_owner(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("백업 파일에는 모든 운영 데이터가 들어 있어 소유자만 내려받을 수 있습니다.")),
			"text/html; charset=utf-8", ""
	}

	id := baro_data_q(query, "id")
	row, found := baro_data_backup_get(db, id)
	if !found {
		return baro_data_error_page(db, config, "백업", tool.Baro_t("그런 백업이 없습니다 : ")+id), "text/html; charset=utf-8", ""
	}

	clean := filepath.ToSlash(filepath.Clean(row.Path))
	if !strings.HasPrefix(clean, baro_data_backup_dir+"/") {
		return baro_data_error_page(db, config, "백업", tool.Baro_t("백업 폴더 밖의 파일은 내려받을 수 없습니다.")), "text/html; charset=utf-8", ""
	}

	raw, err := os.ReadFile(row.Path)
	if err != nil {
		return baro_data_error_page(db, config, "백업", tool.Baro_t("백업 파일을 열 수 없습니다.")), "text/html; charset=utf-8", ""
	}

	baro_data_export_add(db, config.IP, "backup", row.Path, row.Path)
	tool.Baro_audit(db, config, "backup_download", id, "", "", "백업 파일 내려받기")

	return string(raw), "application/octet-stream", filepath.Base(row.Path)
}
