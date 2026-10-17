package route

import (
	"net/url"
	"testing"

	"opennamu/route/tool"
)

// ============================================================
//  과제 지우기
//
//  「닫기」 는 더 할 필요가 없어진 과제를 마무리하는 것이고,
//  「지우기」 는 애초에 잘못 올린 과제를 목록에서 거두는 것이다.
//
//  2번 원칙(삭제보다 보존)에 따라 내용은 지우지 않고 상태만 바꾼다.
//  목록에서는 빠지지만 되돌릴 수 있다.
// ============================================================

// task_action : 과제 화면에 처리 하나를 보낸다
//
//	막히면 안내 화면을 그리는데, 시험에는 스킨도 세션도 없어서 그리다 죽는다.
//	여기서 보고 싶은 것은 화면이 아니라 「DB 가 어떻게 바뀌었는가」 이므로 받아 넘긴다.
func task_action(config tool.Config, id string, values url.Values) (out string) {
	defer func() {
		if err := recover(); err != nil {
			out = ""
		}
	}()

	return View_baro_project_task_view(config, id, values)
}

// task_status_now : 지금 저장된 과제 상태
func task_status_now(t *testing.T, id string) string {
	t.Helper()

	db := tool.DB_connect()
	defer tool.DB_close(db)

	status := ""
	tool.QueryRow_DB(db, "select status from baro_project_task where id = ?", []any{&status}, id)

	return status
}

// TestBaroTaskDelete : 잘못 올린 과제를 지우고 되돌릴 수 있는가
func TestBaroTaskDelete(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "요청자", "user", "2026-01-01 00:00:00")
	baro_flow_user(db, "남", "user", "2026-01-01 00:00:00")

	tool.Exec_DB(db,
		"insert into baro_project_task (id, project, kind, title, detail, difficulty, est_time, assignee, status, doc, date, done_date, requester, field) "+
			"values ('1', '', 'need_doc', '잘못 올린 과제', '', 'easy', '', '', 'open', '', ?, '', '요청자', 'history')",
		tool.Baro_now(),
	)

	mine := tool.Config{IP: "요청자"}
	other := tool.Config{IP: "남"}

	// 남은 지울 수 없다
	task_action(other, "1", url.Values{"do": {"delete"}})
	if got := task_status_now(t, "1"); got != "open" {
		t.Fatalf("남이 지웠다 : %q", got)
	}

	// 요청자는 지울 수 있다
	task_action(mine, "1", url.Values{"do": {"delete"}, "reason": {"잘못 올렸습니다"}})
	if got := task_status_now(t, "1"); got != "deleted" {
		t.Fatalf("지워지지 않았다 : %q", got)
	}

	// 내용은 남아 있다 (2번 원칙)
	title := ""
	tool.QueryRow_DB(db, "select title from baro_project_task where id = '1'", []any{&title})
	if title != "잘못 올린 과제" {
		t.Fatalf("내용이 사라졌다 : %q", title)
	}

	// 목록에는 나오지 않는다
	list := baro_project_task_load(db, []string{"field = ?"}, []any{"history"}, 0, 50)
	for _, one := range list {
		if one.ID == "1" {
			t.Fatalf("지운 과제가 목록에 남아 있다")
		}
	}

	// 되돌리면 다시 열린다
	task_action(mine, "1", url.Values{"do": {"undelete"}})
	if got := task_status_now(t, "1"); got != "open" {
		t.Fatalf("되돌아오지 않았다 : %q", got)
	}

	list = baro_project_task_load(db, []string{"field = ?"}, []any{"history"}, 0, 50)
	found := false
	for _, one := range list {
		if one.ID == "1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("되돌렸는데 목록에 없다")
	}
}

// TestBaroTaskDeleteBlockedWhenTaken : 남이 맡고 있는 과제는 지울 수 없다
func TestBaroTaskDeleteBlockedWhenTaken(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "요청자", "user", "2026-01-01 00:00:00")
	baro_flow_user(db, "맡은이", "user", "2026-01-01 00:00:00")

	tool.Exec_DB(db,
		"insert into baro_project_task (id, project, kind, title, detail, difficulty, est_time, assignee, status, doc, date, done_date, requester, field) "+
			"values ('1', '', 'need_doc', '누가 맡은 과제', '', 'easy', '', '맡은이', 'active', '', ?, '', '요청자', 'history')",
		tool.Baro_now(),
	)

	task_action(tool.Config{IP: "요청자"}, "1", url.Values{"do": {"delete"}})

	if status := task_status_now(t, "1"); status != "active" {
		t.Fatalf("맡고 있는 과제가 지워졌다 : %q", status)
	}

	// 맡은 사람이 본인이면 지울 수 있다
	tool.Exec_DB(tool.DB_connect(), "update baro_project_task set assignee = '요청자' where id = '1'")
	task_action(tool.Config{IP: "요청자"}, "1", url.Values{"do": {"delete"}})
	if status := task_status_now(t, "1"); status != "deleted" {
		t.Fatalf("본인이 맡은 과제를 못 지웠다 : %q", status)
	}
}
