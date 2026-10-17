package tool

import (
	"database/sql"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func settingsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	Exec_DB(db, "create table other (name text, data text, coverage text)")
	return db
}

func TestSettingsCacheReflectsChangesAndScopes(t *testing.T) {
	db := settingsTestDB(t)
	Exec_DB(db, "insert into other values ('skin', 'main_css', ''), ('skin', 'custom', 'mobile')")
	if Wiki_setting(db, "skin", "") != "main_css" || Wiki_setting(db, "skin", "mobile") != "custom" {
		t.Fatal("setting scopes mixed")
	}
	Exec_DB(db, "update other set data = 'updated' where name = 'skin' and coverage = ''")
	if Wiki_setting(db, "skin", "") != "updated" {
		t.Fatal("app update not reflected immediately")
	}
	Exec_DB(db, "delete from other where name = 'skin'")
	if Wiki_setting(db, "skin") != "" {
		t.Fatal("deleted setting remains cached")
	}
	Exec_DB(db, "insert into other values ('skin', 'new', '')")
	if Wiki_setting(db, "skin") != "new" {
		t.Fatal("new setting not visible")
	}
	// External database changes refresh on expiry, without sleeping in the test.
	if _, err := db.Exec("update other set data = 'external' where name = 'skin'"); err != nil {
		t.Fatal(err)
	}
	wiki_setting_lock.Lock()
	wiki_settings.expires = time.Time{}
	wiki_setting_lock.Unlock()
	if Wiki_setting(db, "skin") != "external" {
		t.Fatal("external change not refreshed")
	}
	other := settingsTestDB(t)
	Exec_DB(other, "insert into other values ('skin', 'other database', '')")
	if Wiki_setting(other, "skin") != "other database" || Wiki_setting(db, "skin") != "external" {
		t.Fatal("database values mixed")
	}
}

func TestSensitiveSettingsNeverCached(t *testing.T) {
	db := settingsTestDB(t)
	for _, key := range []string{"password", "edit_acl", "ban", "read_only"} {
		Exec_DB(db, "insert into other values (?, 'before', '')", key)
		if Wiki_setting(db, key) != "before" {
			t.Fatal(key)
		}
		if _, err := db.Exec("update other set data = 'after' where name = ?", key); err != nil {
			t.Fatal(err)
		}
		if Wiki_setting(db, key) != "after" {
			t.Fatalf("security setting cached: %s", key)
		}
	}
}

func TestSettingsConcurrentReadersAndInvalidation(t *testing.T) {
	db := settingsTestDB(t)
	db.SetMaxOpenConns(1)
	Exec_DB(db, "insert into other values ('name', 'Before', '')")
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 50; j++ {
				value := Wiki_setting(db, "name")
				if value != "Before" && value != "After" {
					t.Errorf("invalid value %q", value)
				}
			}
		}()
	}
	Exec_DB(db, "update other set data = 'After' where name = 'name'")
	workers.Wait()
	if Wiki_setting(db, "name") != "After" {
		t.Fatal("update lost during cache refresh")
	}
}

func TestPerformanceIndexesAvoidSortingHistoryAndRecentChanges(t *testing.T) {
	db := settingsTestDB(t)
	for _, query := range []string{
		"create table history (title text, id text, date text)",
		"create table rc (title text, id text, date text, type text)",
		"create table data (title text)",
		"create table acl (title text, type text)",
		"create table alist (name text)",
		"create table html_filter (kind text, plus text)",
	} {
		Exec_DB(db, query)
	}
	create_performance_indexes(db)
	create_performance_indexes(db) // Repeated startup must preserve usable indexes.
	for _, test := range []struct{ query, index string }{
		{"select id from history where title = 'Doc' and id + 0 > 0 and id + 0 < 100 order by id + 0 desc limit 1", "history_title_numeric_idx"},
		{"select id, title from rc where type = '' order by date desc limit 50", "rc_type_date_idx"},
		{"select title, date from history where id = '1' and exists (select 1 from data where data.title = history.title) order by date desc limit 11", "history_id_date_title_idx"},
		{"select title from data where title = 'doc' collate nocase", "data_title_nocase_idx"},
	} {
		rows, err := db.Query("explain query plan " + test.query)
		if err != nil {
			t.Fatal(err)
		}
		plan := ""
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan += detail + "\n"
		}
		rows.Close()
		if !strings.Contains(plan, test.index) || strings.Contains(plan, "TEMP B-TREE") {
			t.Errorf("unexpected plan for %s: %s", test.query, plan)
		}
	}
}
