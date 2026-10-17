package route

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestPreviousRevisionWithDeletedGaps(t *testing.T) {
	entries := []history_rev_entry{{1, "1"}, {4, "4"}, {12, "12"}, {101, "101"}}
	for _, test := range []struct {
		rev  int
		want string
	}{{0, ""}, {1, ""}, {2, "1"}, {4, "1"}, {5, "4"}, {12, "4"}, {100, "12"}, {101, "12"}, {102, "101"}} {
		if got := history_rev_previous(entries, test.rev); got != test.want {
			t.Errorf("r%d: %q, want %q", test.rev, got, test.want)
		}
	}
	if history_rev_previous(nil, 42) != "" {
		t.Fatal("empty history has previous revision")
	}
}

func TestRecentChangeBatchKeepsMetadataSeparate(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("create table history (id text, title text, date text, ip text, send text, leng text, hide text, type text)"); err != nil {
		t.Fatal(err)
	}
	for _, row := range [][]any{
		{"1", "Public", "today", "editor", "public summary", "10", "", ""},
		{"1", "Private", "yesterday", "secret editor", "hidden summary", "20", "O", "move"},
		{"2", "Public", "today", "another", "revision 2", "30", "", ""},
		{"1", "Public", "today", "editor", "public summary", "10", "", ""},
	} {
		if _, err := db.Exec("insert into history values (?, ?, ?, ?, ?, ?, ?, ?)", row...); err != nil {
			t.Fatal(err)
		}
	}
	keys := []recent_change_key{{"1", "Public"}, {"1", "Private"}, {"2", "Public"}, {"1", "Public"}, {"99", "Missing"}}
	got := recent_change_details(db, keys)
	if len(got) != 3 {
		t.Fatalf("duplicates or missing history changed metadata count: %d", len(got))
	}
	if got[keys[0]].send != "public summary" || got[keys[1]].hide != "O" || got[keys[1]].ip != "secret editor" || got[keys[2]].length != "30" {
		t.Fatal("metadata crossed document or revision boundaries")
	}
	if len(recent_change_details(db, nil)) != 0 {
		t.Fatal("empty list returned history")
	}
}
