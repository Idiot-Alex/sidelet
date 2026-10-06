package integration

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"sidelet/internal/quickadd"
	"sidelet/internal/storage"
	"sidelet/internal/todo"
)

func TestQuickAddFailureRetryAndDefaultPresentation(t *testing.T) {
	now := time.Date(2026, 10, 6, 23, 59, 0, 0, time.FixedZone("test", 8*3600))
	dir := t.TempDir()
	s, err := storage.Open(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var session quickadd.Session
	session.Begin(now)
	parsed, err := quickadd.Parse("明天下午3点 联系客户", true, session.Reference)
	if err != nil {
		t.Fatal(err)
	}
	before := view(t, s, now)
	db, err := sql.Open("sqlite", filepath.Join(dir, "sidelet.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER quick_add_failure BEFORE INSERT ON todos BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if !session.StartSave(session.Revision) {
		t.Fatal("save rejected")
	}
	a := todo.Action{Type: "create", Title: parsed.Title, DueAt: &parsed.DueAt}
	if _, err = s.Apply(a, now); err == nil {
		t.Fatal("injected write succeeded")
	}
	session.FinishSave(false)
	if !session.Open || session.ResetVersion != 0 || !reflect.DeepEqual(view(t, s, now), before) {
		t.Fatal("failed create changed data or dismissed input")
	}
	if _, err = db.Exec(`DROP TRIGGER quick_add_failure`); err != nil {
		t.Fatal(err)
	}
	// Retry after midnight keeps the preview's day, rather than silently moving
	// the due date. The record's actual creation time is still the commit time.
	if !session.StartSave(session.Revision) || session.StartSave(session.Revision) {
		t.Fatal("duplicate save accepted")
	}
	parsedAgain, err := quickadd.Parse("明天下午3点 联系客户", true, session.Reference)
	if err != nil || parsedAgain != parsed {
		t.Fatal("retry changed preview")
	}
	v, err := s.Apply(a, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	session.FinishSave(true)
	if len(v.Todos) != 1 || v.Todos[0].DisplayMode != "NONE" || v.Todos[0].Remind || v.Todos[0].RemindAt != 0 || time.UnixMilli(v.Todos[0].DueAt).In(now.Location()).Day() != 7 {
		t.Fatalf("defaults/parsed date: %+v", v)
	}
	if session.StartSave(session.Revision) {
		t.Fatal("replayed save after success accepted")
	}
	session.Begin(now.Add(2 * time.Minute))
	parsed, err = quickadd.Parse("明天下午3点 联系客户", false, session.Reference)
	if err != nil {
		t.Fatal(err)
	}
	session.StartSave(session.Revision)
	v, err = s.Apply(todo.Action{Type: "create", Title: parsed.Title, DueAt: &parsed.DueAt, Pin: true}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	session.FinishSave(true)
	if len(v.Todos) != 2 {
		t.Fatal("unexpected task count")
	}
	found := false
	for _, x := range v.Todos {
		if x.Title == "明天下午3点 联系客户" {
			found = true
			if x.DueAt != 0 || x.DisplayMode != "EDGE" || x.Remind {
				t.Fatal("literal title/pinning failed")
			}
		}
	}
	if !found {
		t.Fatal("literal title lost")
	}
}
