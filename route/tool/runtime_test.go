package tool

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/flosch/pongo2/v6"
)

func TestDocumentReplacementRollsBack(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "atomic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("create table data (title text, data text check(data != 'reject'))"); err != nil {
		t.Fatal(err)
	}
	if err := Replace_document_data(db, "Doc", "original"); err != nil {
		t.Fatal(err)
	}
	if err := Replace_document_data(db, "Doc", "reject"); err == nil {
		t.Fatal("expected insert failure")
	}
	var text string
	if err := db.QueryRow("select data from data where title = 'Doc'").Scan(&text); err != nil || text != "original" {
		t.Fatalf("failed save lost original: %q %v", text, err)
	}
}

func TestSharedDatabasePool(t *testing.T) {
	old := db_set
	DB_shutdown()
	db_set = map[string]string{"db_type": "sqlite", "db_name": filepath.Join(t.TempDir(), "wiki space #한글")}
	t.Cleanup(func() { DB_shutdown(); db_set = old })
	db := DB_connect()
	if db != DB_connect() {
		t.Fatal("pool not reused")
	}
	Exec_DB(db, "create table checks (id integer primary key, value text)")
	for i := 0; i < 2; i++ {
		conn, err := db.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		var mode string
		var timeout int
		if err := conn.QueryRowContext(context.Background(), "pragma journal_mode").Scan(&mode); err != nil || mode != "wal" {
			t.Fatalf("WAL: %s %v", mode, err)
		}
		if err := conn.QueryRowContext(context.Background(), "pragma busy_timeout").Scan(&timeout); err != nil || timeout != 5000 {
			t.Fatalf("timeout: %d %v", timeout, err)
		}
	}
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			Exec_DB(DB_connect(), "insert into checks (id, value) values (?, ?)", i, "값")
			DB_close(db)
		}(i)
	}
	workers.Wait()
	rows := Query_DB(db, "select id from checks")
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("no rows")
	}
	var count int
	if !QueryRow_DB(db, "select count(*) from checks", []any{&count}) || count != 16 {
		t.Fatalf("nested query: %d", count)
	}
	DB_close(db)
	if err := db.Ping(); err != nil {
		t.Fatal("borrower closed shared pool", err)
	}
}

func TestTemplateCacheRefreshAndIsolation(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("views/custom", 0700); err != nil {
		t.Fatal(err)
	}
	file := "views/custom/index.html"
	if err := os.WriteFile(file, []byte("Hello {{name}}"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := load_template("custom/index.html")
	if err != nil {
		t.Fatal(err)
	}
	second, err := load_template("custom/index.html")
	if err != nil || first != second {
		t.Fatal("template not reused")
	}
	var workers sync.WaitGroup
	for _, name := range []string{"Alice", "Bob", "한글"} {
		workers.Add(1)
		go func(name string) {
			defer workers.Done()
			out, err := first.Execute(pongo2.Context{"name": name})
			if err != nil || out != "Hello "+name {
				t.Errorf("context mixed: %s %v", out, err)
			}
		}(name)
	}
	workers.Wait()
	if err := os.WriteFile(file, []byte("Updated {{name}}"), 0600); err != nil {
		t.Fatal(err)
	}
	updated, err := load_template("custom/index.html")
	if err != nil || updated == first {
		t.Fatal("changed template stayed cached")
	}
}
