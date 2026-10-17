package route

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"opennamu/route/tool"
)

// ============================================================
//  고유 번호 · 계정명 이력 · 편집 요청 잠금을 실제 위키에서 시험한다
//
//  화면 함수는 tool.DB_connect() 로 스스로 연결을 잡으므로
//  전역 설정까지 세운 진짜 위키 하나를 만들어 두고 돌린다.
// ============================================================

// baro_flow_boot : 표가 다 있는 빈 위키 하나
//
//	DB_make 는 넘긴 연결이 아니라 전역 풀(DB_connect)을 쓴다.
//	그래서 스키마를 미리 만들어 두고 베껴 쓰는 꾀를 부릴 수 없다.
//	시험마다 온전히 새로 세운다.
func baro_flow_boot(t *testing.T) *sql.DB {
	t.Helper()

	t.Setenv("NAMU_DB", filepath.Join(t.TempDir(), "flow"))
	t.Setenv("NAMU_DB_TYPE", "sqlite")
	tool.DB_shutdown()
	tool.DB_boot()
	t.Cleanup(func() { tool.DB_shutdown() })

	db := tool.DB_connect()
	if err := tool.DB_make(db, tool.Get_DB_set()); err != nil {
		t.Fatal(err)
	}

	tool.Exec_DB(db, "insert into alist (name, acl) values ('ip', 'ip')")
	tool.Exec_DB(db, "insert into alist (name, acl) values ('user', 'user')")
	tool.Exec_DB(db, "insert into alist (name, acl) values ('owner', 'owner')")

	return db
}

// baro_flow_req_call : 편집 요청 화면에 처리 하나를 보낸다
//
//	막히는 경우에는 「권한 없음」 화면을 그리는데, 시험에는 스킨도 세션도 없어서
//	그리다가 죽는다. 여기서 보고 싶은 것은 화면이 아니라 「DB 가 안 바뀌었는가」 이므로
//	그리다 죽는 것은 받아 넘기고 넘어간다.
func baro_flow_req_call(config tool.Config, req_id string, action string) {
	defer func() { _ = recover() }()

	View_baro_req_view(config, req_id, url.Values{"action": {action}, "reason": {"시험"}})
}

// baro_flow_user : 계정 하나 만들기 (가입 화면을 거치지 않고 곧바로)
func baro_flow_user(db *sql.DB, name string, acl string, date string) {
	tool.Exec_DB(db, "insert into user_set (id, name, data) values (?, 'pw', 'x')", name)
	tool.Exec_DB(db, "insert into user_set (id, name, data) values (?, 'acl', ?)", name, acl)
	tool.Exec_DB(db, "insert into user_set (id, name, data) values (?, 'date', ?)", name, date)
}

// TestBaroRevUuidOnEdit : 문서를 저장할 때마다 판 고유 번호가 붙는가
func TestBaroRevUuidOnEdit(t *testing.T) {
	db := baro_flow_boot(t)

	config := tool.Config{IP: "글쓴이"}
	Api_edit_post(config, "시험 문서", "첫 판입니다.", "", "yes", "")
	Api_edit_post(config, "시험 문서", "둘째 판입니다. 내용이 늘었습니다.", "", "yes", "")

	rows := tool.Query_DB(db, "select id from history where title = ? order by id + 0 asc", "시험 문서")
	rev_list := []string{}
	for rows.Next() {
		id := ""
		if rows.Scan(&id) == nil {
			rev_list = append(rev_list, id)
		}
	}
	rows.Close()

	if len(rev_list) < 2 {
		t.Fatalf("판이 2개는 있어야 하는데 %v", rev_list)
	}

	seen := map[string]string{}
	for _, rev := range rev_list {
		value := ""
		if !tool.QueryRow_DB(db, "select uuid from baro_rev_uuid where title = ? and rev = ?", []any{&value}, "시험 문서", rev) {
			t.Fatalf("r%s 에 고유 번호가 안 붙었다", rev)
		}
		if !tool.Baro_uuid_is(value) {
			t.Fatalf("r%s 의 번호가 이상하다 : %q", rev, value)
		}
		if before, ok := seen[value]; ok {
			t.Fatalf("r%s 와 r%s 의 번호가 같다", before, rev)
		}

		seen[value] = rev

		// 번호를 판 자리에 그대로 넣어도 같은 판을 가리켜야 한다
		if got := tool.Baro_rev_resolve(db, "시험 문서", value); got != rev {
			t.Fatalf("r%s : 번호를 판으로 못 바꿨다 (%q)", rev, got)
		}
	}
}

func TestHistoryLinksUseRevisionUuids(t *testing.T) {
	db := baro_flow_boot(t)
	title := "UUID 역사 링크 시험"
	config := tool.Config{IP: "글쓴이"}

	Api_edit_post(config, title, "첫 판 내용입니다.", "첫 판", "yes", "")
	Api_edit_post(config, title, "둘째 판 내용입니다.", "둘째 판", "yes", "")
	Api_edit_post(config, title, "셋째 판 내용입니다.", "셋째 판", "yes", "")

	before_uuid := tool.Baro_rev_uuid(db, title, "1")
	after_uuid := tool.Baro_rev_uuid(db, title, "2")
	if offset := Api_history_revision_offset(title, "normal", "2"); offset != 1 {
		t.Fatalf("r2 위에는 최신 r3 한 판만 있어야 한다: offset=%d", offset)
	}
	positioned := api_list_history_offset(config, title, "normal", Api_history_revision_offset(title, "normal", "2"))["data"].([][]string)
	if len(positioned) == 0 || positioned[0][0] != "2" {
		t.Fatalf("commit 판이 목록 첫 줄이어야 한다: %v", positioned)
	}

	api_data := Api_list_history(config, title, "normal", "1")
	data_all := api_data["data"].([][]string)
	history_html, _ := Get_ui_history(db, config, data_all)

	want_diff := "/diff/" + tool.Url_parser(title) + "?uuid=" + tool.Url_parser(after_uuid) + "&olduuid=" + tool.Url_parser(before_uuid)
	for _, want := range []string{
		want_diff,
		"/render/" + tool.Url_parser(title) + "?uuid=" + tool.Url_parser(after_uuid),
		"/raw/" + tool.Url_parser(title) + "?uuid=" + tool.Url_parser(after_uuid),
		"/blame/" + tool.Url_parser(title) + "?uuid=" + tool.Url_parser(after_uuid),
		"/revert/" + tool.Url_parser(title) + "?uuid=" + tool.Url_parser(after_uuid),
	} {
		if !strings.Contains(history_html, want) {
			t.Errorf("역사 링크에 %q 가 없다 : %s", want, history_html)
		}
	}
}

func TestMoveTitleChangeShownInHistory(t *testing.T) {
	db := baro_flow_boot(t)
	title := "양홍원"
	config := tool.Config{IP: "글쓴이"}
	tool.Exec_DB(db, "insert into data (title, data, type) values (?, ?, '')", title, "문서 본문")
	tool.Do_add_history(db, title, "문서 본문", tool.Get_time(), config.IP, "문서 이동", "0", "move", move_history_title_change("YoungB", title))

	api_data := Api_list_history(config, title, "normal", "1")
	data_all := api_data["data"].([][]string)
	history_html, _ := Get_ui_history(db, config, data_all)
	if !strings.Contains(history_html, "(YoungB -&gt; "+title+")") {
		t.Fatalf("문서 이동 전후 제목이 역사에 보여야 한다: %s", history_html)
	}
}

// TestBaroRevUuidSurvivesRenumber : 앞 판을 지워 번호가 밀려도 고유 번호는 그 판을 가리키는가
//
//	이 기능이 있는 까닭 자체를 못 박아 두는 시험이다.
func TestBaroRevUuidSurvivesRenumber(t *testing.T) {
	db := baro_flow_boot(t)

	config := tool.Config{IP: "글쓴이"}
	Api_edit_post(config, "밀림 문서", "첫 판입니다.", "", "yes", "")
	Api_edit_post(config, "밀림 문서", "둘째 판입니다.", "", "yes", "")

	// 둘째 판의 고유 번호와 본문을 적어 둔다
	second_uuid := tool.Baro_rev_uuid(db, "밀림 문서", "2")
	second_body := ""
	tool.QueryRow_DB(db, "select data from history where title = ? and id = '2'", []any{&second_body}, "밀림 문서")

	// 첫 판을 지우고 남은 판의 번호를 1 로 당긴다 (역사 정리와 같은 모양)
	tool.Exec_DB(db, "delete from history where title = ? and id = '1'", "밀림 문서")
	tool.Exec_DB(db, "update history set id = '1' where title = ? and id = '2'", "밀림 문서")
	tool.Exec_DB(db, "update baro_rev_uuid set rev = '1' where title = ? and rev = '2'", "밀림 문서")

	// 고유 번호는 그대로 그 판을 가리켜야 한다
	rev := tool.Baro_rev_resolve(db, "밀림 문서", second_uuid)
	body := ""
	tool.QueryRow_DB(db, "select data from history where title = ? and id = ?", []any{&body}, "밀림 문서", rev)

	if body != second_body {
		t.Fatalf("번호가 밀린 뒤에 다른 판을 가리킨다 : %q", body)
	}
}

// TestBaroAccountNameHistory : 이름을 바꾸면 이력이 쌓이고 차례대로 보이는가
func TestBaroAccountNameHistory(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "계정하나", "user", "2026-01-01 00:00:00")
	user_save(db, "계정하나", "user_name", "첫이름")

	config := tool.Config{IP: "계정하나"}

	// 이름을 두 번 바꾼다
	for _, name := range []string{"둘째이름", "셋째이름"} {
		out := View_user_name_for(config, "", url.Values{"new_user_name": {name}})
		if target, ok := tool.Get_redirect_target(out); !ok || target != "/change/user_name" {
			t.Fatalf("%s 로 바꾸는 데 실패했다 : %q", name, tool.Baro_trunc(out, 200))
		}
	}

	user_uuid := tool.Baro_user_uuid(db, "계정하나")
	list := baro_account_name_history(db, "계정하나", user_uuid)

	if len(list) != 3 {
		t.Fatalf("구간이 3개여야 하는데 %d개다 : %v", len(list), list)
	}

	want := []string{"첫이름", "둘째이름", "셋째이름"}
	for index, one := range list {
		if one.Name != want[index] {
			t.Fatalf("%d번째 이름이 %q 여야 하는데 %q", index, want[index], one.Name)
		}
	}

	// 첫 구간은 가입 시각에서 시작하고, 마지막 구간은 아직 안 끝났다
	if list[0].Start != "2026-01-01 00:00:00" {
		t.Fatalf("첫 구간이 가입 시각에서 시작하지 않는다 : %q", list[0].Start)
	}
	if list[2].End != "" {
		t.Fatalf("지금 쓰는 이름인데 끝난 것으로 되어 있다 : %q", list[2].End)
	}

	// 구간이 끊기지 않고 이어져야 한다
	for index := 1; index < len(list); index++ {
		if list[index].Start != list[index-1].End {
			t.Fatalf("%d번째 구간이 앞과 안 이어진다 : %q ≠ %q", index, list[index].Start, list[index-1].End)
		}
	}

	// 이름 대신 고유 번호를 넣어도 같은 계정을 가리켜야 한다
	if got := baro_account_name_target(db, config, user_uuid); got != "계정하나" {
		t.Fatalf("고유 번호로 계정을 못 찾았다 : %q", got)
	}
	if got := baro_account_name_target(db, config, ""); got != "계정하나" {
		t.Fatalf("비우면 본인이어야 하는데 %q", got)
	}
	if got := baro_account_name_target(db, config, tool.Baro_uuid_new()); got != "" {
		t.Fatalf("없는 번호인데 뭔가 찾아 줬다 : %q", got)
	}
}

// TestBaroReqLockAndReopen : 편집 요청 잠금과 다시 열기
func TestBaroReqLockAndReopen(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "부탁한이", "user", "2026-01-01 00:00:00")
	baro_flow_user(db, "운영자", "owner", "2026-01-01 00:00:00")

	Api_edit_post(tool.Config{IP: "운영자"}, "요청 문서", "원래 본문입니다.", "", "yes", "")

	// 거절된 요청 하나를 만들어 둔다
	req_id := "1"
	tool.Exec_DB(
		db,
		"insert into baro_edit_req (id, title, base_rev, user, data, summary, status, handler, handle_date, reason, date, new_rev, kind, target, locked) "+
			"values (?, ?, '1', '부탁한이', '바꾼 본문입니다.', '고쳐 주세요', 'rejected', '운영자', ?, '아직은 아닙니다', ?, '', 'edit', '', '0')",
		req_id, "요청 문서", tool.Baro_now(), tool.Baro_now(),
	)

	mine := tool.Config{IP: "부탁한이"}
	admin := tool.Config{IP: "운영자"}

	status_now := func() string {
		status := ""
		tool.QueryRow_DB(db, "select status from baro_edit_req where id = ?", []any{&status}, req_id)

		return status
	}
	locked_now := func() string {
		locked := ""
		tool.QueryRow_DB(db, "select locked from baro_edit_req where id = ?", []any{&locked}, req_id)

		return locked
	}

	// 남이 다시 열 수는 없다
	baro_flow_req_call(admin, req_id, "reopen")
	if status_now() != "rejected" {
		t.Fatalf("요청한 사람이 아닌데 다시 열렸다")
	}

	// 본인은 다시 열 수 있고, 기준 판은 지금 판으로 새로 잡힌다
	baro_flow_req_call(mine, req_id, "reopen")
	if status_now() != "open" {
		t.Fatalf("본인이 다시 열지 못했다 : %q", status_now())
	}

	base_rev, handler := "", ""
	tool.QueryRow_DB(db, "select base_rev, handler from baro_edit_req where id = ?", []any{&base_rev, &handler}, req_id)
	if base_rev != tool.Get_document_revision(db, "요청 문서") {
		t.Fatalf("기준 판을 새로 안 잡았다 : %q", base_rev)
	}
	if handler != "" {
		t.Fatalf("처리한 사람이 그대로 남아 있다 : %q", handler)
	}

	// 관리자만 잠글 수 있다
	baro_flow_req_call(mine, req_id, "lock")
	if locked_now() == "1" {
		t.Fatalf("관리자가 아닌데 잠갔다")
	}

	baro_flow_req_call(admin, req_id, "lock")
	if locked_now() != "1" {
		t.Fatalf("관리자가 못 잠갔다")
	}

	// 잠긴 요청은 승인도 안 되고 다시 열리지도 않는다
	baro_flow_req_call(admin, req_id, "accept")
	if status_now() != "open" {
		t.Fatalf("잠긴 요청이 처리됐다 : %q", status_now())
	}

	tool.Exec_DB(db, "update baro_edit_req set status = 'rejected' where id = ?", req_id)
	baro_flow_req_call(mine, req_id, "reopen")
	if status_now() != "rejected" {
		t.Fatalf("잠긴 요청이 다시 열렸다")
	}

	// 잠금을 풀면 다시 열린다
	baro_flow_req_call(admin, req_id, "unlock")
	if locked_now() != "0" {
		t.Fatalf("잠금이 안 풀렸다")
	}

	baro_flow_req_call(mine, req_id, "reopen")
	if status_now() != "open" {
		t.Fatalf("잠금을 풀었는데도 못 열었다 : %q", status_now())
	}

	// 처리 기록이 남아야 한다
	actions := []string{}
	rows := tool.Query_DB(db, "select action from baro_edit_req_log where req_id = ? order by id + 0 asc", req_id)
	for rows.Next() {
		action := ""
		if rows.Scan(&action) == nil {
			actions = append(actions, action)
		}
	}
	rows.Close()

	if strings.Join(actions, ",") != "reopen,lock,unlock,reopen" {
		t.Fatalf("처리 기록이 다르다 : %v", actions)
	}
}
