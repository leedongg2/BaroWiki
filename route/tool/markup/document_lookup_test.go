package markup

import (
	"database/sql"
	"opennamu/route/tool"
	"path/filepath"
	"testing"
)

func TestDocumentLookupRefreshesBetweenRenders(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "links.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tool.Exec_DB(db, "create table other (name text, data text, coverage text)")
	tool.Exec_DB(db, "create table data (title text, data text)")
	newRenderer := func() *namumark_compat_renderer {
		return new_namumark_compat_renderer(db, "Source", "", "", nil, 0, false)
	}
	first := newRenderer()
	if _, exists := first.find_document("Target"); exists {
		t.Fatal("missing document exists")
	}
	tool.Exec_DB(db, "insert into data values ('Target', 'body')")
	second := newRenderer()
	if title, exists := second.find_document("Target"); !exists || title != "Target" {
		t.Fatal("new document not reflected on next render")
	}
	if _, exists := second.find_document("target"); exists {
		t.Fatal("case-sensitive lookup ignored")
	}
	tool.Exec_DB(db, "insert into other values ('link_case_insensitive', '1', '')")
	if title, exists := newRenderer().find_document("target"); !exists || title != "Target" {
		t.Fatal("case-insensitive link lost canonical title")
	}
	tool.Exec_DB(db, "delete from data where title = 'Target'")
	if _, exists := newRenderer().find_document("Target"); exists {
		t.Fatal("deleted document remains linked")
	}
}
