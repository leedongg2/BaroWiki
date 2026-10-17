package route

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"opennamu/route/tool"
)

func verify533_boot(t *testing.T) *sql.DB {
	t.Helper()
	t.Setenv("NAMU_DB", filepath.Join(t.TempDir(), "v533"))
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

func TestVerify533_RedirectDeadEnd(t *testing.T) {
	db := verify533_boot(t)

	write := func(title, data string) {
		r := Api_edit_post(tool.Config{IP: "tester"}, title, data, "", "yes", "")
		t.Logf("edit %s -> %v", title, r)
	}

	write("도착문서", "도착이다. [[출발문서]]")
	write("넘김문서", "#redirect 도착문서")
	write("출발문서", "[[넘김문서]]\n[[도착문서]]")

	rows := tool.Query_DB(db, "select link, title, type from back order by link, title, type")
	for rows.Next() {
		l, ti, ty := "", "", ""
		_ = rows.Scan(&l, &ti, &ty)
		t.Logf("back: link=%q title=%q type=%q", l, ti, ty)
	}
	rows.Close()

	filter := tool.New_acl_render_filter(db, "tester")
	t.Logf("out(출발문서) = %v", baro_game_out_links(db, filter, "출발문서"))
	t.Logf("out(넘김문서) = %v", baro_game_out_links(db, filter, "넘김문서"))
	t.Logf("out(도착문서) = %v", baro_game_out_links(db, filter, "도착문서"))

	if len(baro_game_out_links(db, filter, "넘김문서")) != 0 {
		t.Errorf("리다이렉트 문서에 나가는 링크가 있다 (주장과 다름)")
	}

	// 막다른 화면 실제 렌더
	config := tool.Config{IP: "tester"}
	seat := baro_game_seat(config)
	baro_game_state_set(db, seat, "race", map[string]string{
		"start": "출발문서", "goal": "도착문서", "current": "넘김문서",
		"moves": "1", "path": "출발문서 › 넘김문서", "start_date": tool.Baro_now(),
	})

	defer func() {
		if e := recover(); e != nil {
			t.Logf("render panic (스킨 없음): %v", e)
		}
	}()
	html := View_baro_game_race(config, nil)
	for _, part := range strings.Split(html, `name="action"`) [1:] {
		if i := strings.Index(part, `value="`); i >= 0 {
			rest := part[i+7:]
			if j := strings.Index(rest, `"`); j >= 0 {
				t.Logf("화면에 있는 action = %q", rest[:j])
			}
		}
	}
	t.Logf("막다른 경고 있음? %v", strings.Contains(html, "막다른 길"))

	// give_up 이 기록을 남기는지
	View_baro_game_race(config, map[string]string{"action": "give_up"})
	rows2 := tool.Query_DB(db, "select user, game, score, detail from baro_game_play")
	for rows2.Next() {
		u, g, s, d := "", "", "", ""
		_ = rows2.Scan(&u, &g, &s, &d)
		t.Logf("baro_game_play: user=%q game=%q score=%q detail=%q", u, g, s, d)
	}
	rows2.Close()
}
