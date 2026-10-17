package route

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestBackupIncludesUncheckpointedWAL(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, query := range []string{"pragma journal_mode=WAL", "create table document (body text)", "insert into document values ('old')"} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	reader, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Rollback()
	var body string
	if err := reader.QueryRow("select body from document").Scan(&body); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("update document set body = 'latest'"); err != nil {
		t.Fatal(err)
	}
	var busy, pages, flushed int
	if err := db.QueryRow("pragma wal_checkpoint(passive)").Scan(&busy, &pages, &flushed); err != nil {
		t.Fatal(err)
	}
	if pages <= flushed {
		t.Fatal("test must leave uncheckpointed changes")
	}
	dst := filepath.Join(t.TempDir(), "snapshot.db")
	if size, err := baro_data_sqlite_snapshot(db, dst); err != nil || size == 0 {
		t.Fatalf("snapshot: %d %v", size, err)
	}
	backup, err := sql.Open("sqlite", dst)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	if err := backup.QueryRow("select body from document").Scan(&body); err != nil || body != "latest" {
		t.Fatalf("snapshot lost WAL: %s %v", body, err)
	}
	if err := backup.QueryRow("pragma integrity_check").Scan(&body); err != nil || body != "ok" {
		t.Fatalf("invalid snapshot: %s %v", body, err)
	}
	if _, err := baro_data_sqlite_snapshot(db, dst); err == nil {
		t.Fatal("existing backup overwritten")
	}
}
