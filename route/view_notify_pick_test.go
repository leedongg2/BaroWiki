package route

import (
	"net/url"
	"strings"
	"testing"

	"opennamu/route/tool"
)

// ============================================================
//  알림 골라 지우기
//
//  예전에는 「읽은 알림 모두 삭제」 와 한 줄씩 지우기만 있었다.
//  이제 고르기 칸으로 여러 개를 골라 한 번에 지운다.
//  남의 알림을 지울 수 없어야 하므로 언제나 받는 사람까지 함께 맞춘다.
// ============================================================

// TestBaroNotifyDeletePick : 고른 알림만 지우는가
func TestBaroNotifyDeletePick(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "나", "user", "2026-01-01 00:00:00")
	baro_flow_user(db, "남", "user", "2026-01-01 00:00:00")

	for _, one := range [][]string{
		{"1", "나", "첫째"},
		{"2", "나", "둘째"},
		{"3", "나", "셋째"},
		{"4", "남", "남의 것"},
	} {
		tool.Exec_DB(db,
			"insert into baro_notify (id, user, kind, title, url, data, readme, date) values (?, ?, 'system', ?, '', '', '', ?)",
			one[0], one[1], one[2], tool.Baro_now(),
		)
	}

	left := func(user string) []string {
		rows := tool.Query_DB(db, "select id from baro_notify where user = ? order by id + 0 asc", user)
		defer rows.Close()

		out := []string{}
		for rows.Next() {
			id := ""
			if rows.Scan(&id) == nil {
				out = append(out, id)
			}
		}

		return out
	}

	// 1번과 3번만 고른다
	View_baro_account_notify(tool.Config{IP: "나"}, "", "", "", url.Values{
		"action": {"delete_pick"},
		"pick":   {"1", "3"},
	})

	if got := strings.Join(left("나"), ","); got != "2" {
		t.Fatalf("고른 것만 지워지지 않았다 : %q", got)
	}

	// 남의 알림은 그대로다
	if got := strings.Join(left("남"), ","); got != "4" {
		t.Fatalf("남의 알림이 사라졌다 : %q", got)
	}
}

// TestBaroNotifyDeletePickOther : 남의 알림 번호를 골라 보내도 지워지지 않는가
func TestBaroNotifyDeletePickOther(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "나", "user", "2026-01-01 00:00:00")
	baro_flow_user(db, "남", "user", "2026-01-01 00:00:00")

	tool.Exec_DB(db,
		"insert into baro_notify (id, user, kind, title, url, data, readme, date) values ('9', '남', 'system', '남의 것', '', '', '', ?)",
		tool.Baro_now(),
	)

	// 내가 남의 알림 번호를 보내도 지워지면 안 된다
	View_baro_account_notify(tool.Config{IP: "나"}, "", "", "", url.Values{
		"action": {"delete_pick"},
		"pick":   {"9"},
	})

	count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_notify where id = '9'", []any{&count})
	if count != 1 {
		t.Fatalf("남의 알림을 지울 수 있다")
	}
}

// TestAlarmDeletePick : openNAMU 알림함도 골라 지우는가
func TestAlarmDeletePick(t *testing.T) {
	db := baro_flow_boot(t)

	baro_flow_user(db, "나", "user", "2026-01-01 00:00:00")

	for _, id := range []string{"1", "2", "3"} {
		tool.Exec_DB(db,
			"insert into user_notice (id, name, data, date, readme) values (?, '나', ?, ?, '')",
			id, "알림 "+id, tool.Baro_now(),
		)
	}

	View_alarm(tool.Config{IP: "나"}, "", url.Values{"pick": {"1", "2"}})

	rows := tool.Query_DB(db, "select id from user_notice where name = '나' order by id + 0 asc")
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		id := ""
		if rows.Scan(&id) == nil {
			out = append(out, id)
		}
	}

	if got := strings.Join(out, ","); got != "3" {
		t.Fatalf("고른 것만 지워지지 않았다 : %q", got)
	}
}

