package storage

import (
	"database/sql"
	"path/filepath"
	"sidelet/internal/todo"
	"testing"
	"time"
)

func remindersTest(t *testing.T, s *Store) []Reminder {
	t.Helper()
	r, e := s.Reminders()
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func TestReminderLifecycleAndDeduplication(t *testing.T) {
	s, dir := openTest(t)
	due := testNow.Add(time.Minute).UnixMilli()
	state := applyTest(t, s, todo.Action{Type: "create", Title: "visual only", DueAt: &due, Pin: true}, testNow)
	if len(remindersTest(t, s)) != 0 || state.Todos[0].Remind {
		t.Fatal("default created a system reminder")
	}
	state = applyTest(t, s, todo.Action{Type: "edit", ID: 1, Title: "notify", Remind: boolptr(true)}, testNow)
	first := remindersTest(t, s)[0]
	if first.At != due || !state.Todos[0].Remind {
		t.Fatal("enable reminder failed")
	}
	if e := s.MarkReminderSent(first.ID, testNow); e != nil {
		t.Fatal(e)
	}
	applyTest(t, s, todo.Action{Type: "edit", ID: 1, Title: "new title"}, testNow)
	applyTest(t, s, todo.Action{Type: "complete", ID: 1}, testNow)
	applyTest(t, s, todo.Action{Type: "undo"}, testNow.Add(time.Second))
	applyTest(t, s, todo.Action{Type: "unpin", ID: 1}, testNow)
	current := remindersTest(t, s)[0]
	if current.ID != first.ID || current.SentAt == 0 || current.Completed {
		t.Fatal("edit/undo/unpin rearmed delivery")
	}
	s.Close()
	reopened, e := Open(dir, testNow)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if remindersTest(t, reopened)[0].SentAt == 0 {
		t.Fatal("restart lost delivery receipt")
	}
	state = applyTest(t, reopened, todo.Action{Type: "snooze", ID: 1, Duration: "30m"}, testNow)
	snoozed := remindersTest(t, reopened)[0]
	if snoozed.ID == first.ID || snoozed.SentAt != 0 || snoozed.At != testNow.Add(30*time.Minute).UnixMilli() || state.Todos[0].DueAt != due {
		t.Fatal("snooze did not defer/rearm independently")
	}
	applyTest(t, reopened, todo.Action{Type: "delete", ID: 1}, testNow)
	if len(remindersTest(t, reopened)) != 0 {
		t.Fatal("delete orphaned reminder")
	}
}
func TestReminderDeadlineChangesDisableAndValidation(t *testing.T) {
	s, _ := openTest(t)
	due := testNow.Add(time.Hour).UnixMilli()
	for _, a := range []todo.Action{{Type: "create", Title: "bad", Remind: boolptr(true)}, {Type: "create", Title: "bad", Remind: boolptr(true), DueAt: int64ptr(0)}} {
		if _, e := s.Apply(a, testNow); e == nil {
			t.Fatal("reminder without deadline accepted")
		}
	}
	applyTest(t, s, todo.Action{Type: "create", Title: "A", Remind: boolptr(true), DueAt: &due}, testNow)
	first := remindersTest(t, s)[0]
	later := due + 60000
	applyTest(t, s, todo.Action{Type: "edit", ID: 1, Title: "A", DueAt: &later}, testNow)
	if r := remindersTest(t, s)[0]; r.ID == first.ID || r.At != later || r.SentAt != 0 {
		t.Fatal("deadline change retained old generation")
	}
	state := applyTest(t, s, todo.Action{Type: "edit", ID: 1, Title: "A", DueAt: int64ptr(0)}, testNow)
	if state.Todos[0].Remind || len(remindersTest(t, s)) != 0 {
		t.Fatal("cleared deadline retained reminder")
	}
	if _, e := s.Apply(todo.Action{Type: "edit", ID: 1, Title: "A", Remind: boolptr(true)}, testNow); e == nil {
		t.Fatal("edit enabled without a deadline")
	}
	applyTest(t, s, todo.Action{Type: "edit", ID: 1, Title: "A", DueAt: &due, Remind: boolptr(true)}, testNow)
	applyTest(t, s, todo.Action{Type: "edit", ID: 1, Title: "A", Remind: boolptr(false)}, testNow)
	if len(remindersTest(t, s)) != 0 {
		t.Fatal("explicit off retained reminder")
	}
}
func TestReminderGenerationFailureRollsBackTaskEdit(t *testing.T) {
	s, _ := openTest(t)
	due := testNow.UnixMilli()
	applyTest(t, s, todo.Action{Type: "create", Title: "original", DueAt: &due, Remind: boolptr(true)}, testNow)
	before := remindersTest(t, s)[0]
	if _, e := s.db.Exec(`CREATE TRIGGER fail_reminder BEFORE INSERT ON reminders BEGIN SELECT RAISE(ABORT,'injected'); END`); e != nil {
		t.Fatal(e)
	}
	due += 1000
	if _, e := s.Apply(todo.Action{Type: "edit", ID: 1, Title: "must rollback", DueAt: &due}, testNow); e == nil {
		t.Fatal("injected failure accepted")
	}
	if remindersTest(t, s)[0] != before || viewTest(t, s, testNow).Todos[0].Title != "original" {
		t.Fatal("partial reminder/task transaction")
	}
}
func TestVersionOneMigrationPreservesTasksAndDefaultsToVisual(t *testing.T) {
	dir := t.TempDir()
	db, e := sql.Open("sqlite", filepath.Join(dir, "sidelet.sqlite3"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(initialSchema + `;CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,applied_at INTEGER NOT NULL);
 INSERT INTO schema_migrations VALUES(1,1);PRAGMA user_version=1;
 INSERT INTO todos(id,title,created_at,updated_at,due_at) VALUES(1,'existing',1,2,3000);
 INSERT INTO desktop_presentations VALUES(1,'NONE',1,2);`)
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	s, e := Open(dir, testNow)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	task := viewTest(t, s, testNow).Todos[0]
	if task.Title != "existing" || task.DueAt != 3000 || task.UpdatedAt != 2 || task.Remind || len(remindersTest(t, s)) != 0 {
		t.Fatal("migration changed task or opted into notifications")
	}
	var version int
	s.db.QueryRow(`PRAGMA user_version`).Scan(&version)
	if version != 2 {
		t.Fatal("migration version not advanced")
	}
}
