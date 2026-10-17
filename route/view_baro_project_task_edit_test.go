package route

import (
	"net/url"
	"testing"

	"opennamu/route/tool"
)

// ============================================================
//  과제 고치기
//
//  잘못 적은 제목·설명을 고친다.
//  요청한 본인과 프로젝트 담당자만 고칠 수 있고, 완료된 과제는 고치지 않는다.
// ============================================================

// task_field : 과제의 칸 하나를 읽는다
func task_field(t *testing.T, id string, column string) string {
	t.Helper()

	db := tool.DB_connect()
	defer tool.DB_close(db)

	value := ""
	tool.QueryRow_DB(db, "select "+column+" from baro_project_task where id = ?", []any{&value}, id)

	return value
}

// task_edit : 고치기 화면에 값을 보낸다 (스킨이 없어 그리다 죽는 것은 받아 넘긴다)
func task_edit(config tool.Config, id string, values url.Values) {
	defer func() { _ = recover() }()

	View_baro_project_task_edit(config, id, values)
}

// TestBaroTaskEdit : 요청자가 과제를 고칠 수 있는가
func TestBaroTaskEdit(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "요청자", "user", "2026-01-01 00:00:00")
	baro_flow_user(db, "남", "user", "2026-01-01 00:00:00")

	tool.Exec_DB(db,
		"insert into baro_project_task (id, project, kind, title, detail, difficulty, est_time, assignee, status, doc, date, done_date, requester, field) "+
			"values ('1', '', 'need_doc', '잘못 적은 제목', '옛 설명', 'easy', '10분', '', 'open', '', ?, '', '요청자', 'history')",
		tool.Baro_now(),
	)

	good := url.Values{
		"title":      {"고친 제목"},
		"kind":       {"need_source"},
		"field":      {"history"},
		"project":    {""},
		"difficulty": {"normal"},
		"est_time":   {"30분"},
		"detail":     {"새 설명"},
		"reason":     {"더 또렷하게"},
	}

	// 남은 고칠 수 없다
	task_edit(tool.Config{IP: "남"}, "1", good)
	if got := task_field(t, "1", "title"); got != "잘못 적은 제목" {
		t.Fatalf("남이 고쳤다 : %q", got)
	}

	// 요청자는 고칠 수 있다
	task_edit(tool.Config{IP: "요청자"}, "1", good)

	for _, one := range [][2]string{
		{"title", "고친 제목"},
		{"kind", "need_source"},
		{"difficulty", "normal"},
		{"est_time", "30분"},
		{"detail", "새 설명"},
	} {
		if got := task_field(t, "1", one[0]); got != one[1] {
			t.Errorf("%s : %q 를 바랐는데 %q", one[0], one[1], got)
		}
	}

	// 상태와 요청자는 건드리지 않는다
	if got := task_field(t, "1", "status"); got != "open" {
		t.Fatalf("상태가 바뀌었다 : %q", got)
	}
	if got := task_field(t, "1", "requester"); got != "요청자" {
		t.Fatalf("요청자가 바뀌었다 : %q", got)
	}
}

// TestBaroTaskEditEmptyTitle : 제목을 비우면 고쳐지지 않는가
func TestBaroTaskEditEmptyTitle(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "요청자", "user", "2026-01-01 00:00:00")
	tool.Exec_DB(db,
		"insert into baro_project_task (id, project, kind, title, detail, difficulty, est_time, assignee, status, doc, date, done_date, requester, field) "+
			"values ('1', '', 'need_doc', '원래 제목', '', 'easy', '', '', 'open', '', ?, '', '요청자', 'history')",
		tool.Baro_now(),
	)

	task_edit(tool.Config{IP: "요청자"}, "1", url.Values{"title": {"   "}, "detail": {"바뀐 설명"}})

	if got := task_field(t, "1", "title"); got != "원래 제목" {
		t.Fatalf("제목이 비었는데 저장됐다 : %q", got)
	}
	if got := task_field(t, "1", "detail"); got != "" {
		t.Fatalf("막혔어야 하는데 설명이 저장됐다 : %q", got)
	}
}

// TestBaroTaskEditDoneBlocked : 완료된 과제는 고치지 못하는가
func TestBaroTaskEditDoneBlocked(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "요청자", "user", "2026-01-01 00:00:00")
	tool.Exec_DB(db,
		"insert into baro_project_task (id, project, kind, title, detail, difficulty, est_time, assignee, status, doc, date, done_date, requester, field) "+
			"values ('1', '', 'need_doc', '끝난 과제', '', 'easy', '', '요청자', 'done', '어떤 문서', ?, ?, '요청자', 'history')",
		tool.Baro_now(), tool.Baro_now(),
	)

	task_edit(tool.Config{IP: "요청자"}, "1", url.Values{"title": {"뒤늦게 바꾼 제목"}})

	if got := task_field(t, "1", "title"); got != "끝난 과제" {
		t.Fatalf("완료된 과제가 고쳐졌다 : %q", got)
	}
}

// TestBaroTaskEditNotifiesAssignee : 맡은 사람에게 알림이 가는가
func TestBaroTaskEditNotifiesAssignee(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "요청자", "user", "2026-01-01 00:00:00")
	baro_flow_user(db, "맡은이", "user", "2026-01-01 00:00:00")

	tool.Exec_DB(db,
		"insert into baro_project_task (id, project, kind, title, detail, difficulty, est_time, assignee, status, doc, date, done_date, requester, field) "+
			"values ('1', '', 'need_doc', '원래 제목', '', 'easy', '', '맡은이', 'active', '', ?, '', '요청자', 'history')",
		tool.Baro_now(),
	)

	task_edit(tool.Config{IP: "요청자"}, "1", url.Values{
		"title": {"바뀐 제목"}, "kind": {"need_doc"}, "field": {"history"},
		"difficulty": {"easy"}, "est_time": {""}, "detail": {""},
	})

	if got := task_field(t, "1", "title"); got != "바뀐 제목" {
		t.Fatalf("맡고 있는 과제를 못 고쳤다 : %q", got)
	}

	count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_notify where user = '맡은이'", []any{&count})
	if count != 1 {
		t.Fatalf("맡은 사람에게 알림이 가지 않았다 : %d건", count)
	}

	// 바뀐 것이 없으면 알리지 않는다
	task_edit(tool.Config{IP: "요청자"}, "1", url.Values{
		"title": {"바뀐 제목"}, "kind": {"need_doc"}, "field": {"history"},
		"difficulty": {"easy"}, "est_time": {""}, "detail": {""},
	})

	tool.QueryRow_DB(db, "select count(*) from baro_notify where user = '맡은이'", []any{&count})
	if count != 1 {
		t.Fatalf("바뀐 것이 없는데 또 알렸다 : %d건", count)
	}
}
