// Package integration tests the real persistence/scheduler boundary with a deterministic OS stand-in.
package integration

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"sidelet/internal/reminder"
	"sidelet/internal/settings"
	"sidelet/internal/storage"
	"sidelet/internal/todo"
)

type notificationSystem struct {
	state reminder.State
	sent  []string
}

func (s *notificationSystem) State() (reminder.State, error) { return s.state, nil }
func (s *notificationSystem) Deliver(id, title, body string) error {
	if title == "" || body == "" {
		return errors.New("empty notification")
	}
	s.sent = append(s.sent, id)
	s.state.Delivered = append(s.state.Delivered, id)
	return nil
}
func (s *notificationSystem) Remove(ids []string) error {
	s.state.Delivered = slices.DeleteFunc(s.state.Delivered, func(id string) bool { return slices.Contains(ids, id) })
	s.state.Pending = slices.DeleteFunc(s.state.Pending, func(id string) bool { return slices.Contains(ids, id) })
	return nil
}
func ptr[T any](value T) *T { return &value }
func mutate(t *testing.T, s *storage.Store, now time.Time, a todo.Action) todo.Snapshot {
	t.Helper()
	v, e := s.Apply(a, now)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func view(t *testing.T, s *storage.Store, now time.Time) todo.Snapshot {
	t.Helper()
	v, e := s.Snapshot(now)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func syncAt(t *testing.T, s *storage.Store, os *notificationSystem, now time.Time) reminder.Result {
	t.Helper()
	v, e := reminder.Sync(s, os, "regression.", now)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func task(t *testing.T, v todo.Snapshot, id int64) todo.Todo {
	t.Helper()
	for _, x := range v.Todos {
		if x.ID == id {
			return x
		}
	}
	t.Fatalf("missing task %d", id)
	return todo.Todo{}
}
func TestTaskLifecycleAcrossPersistenceAndReminders(t *testing.T) {
	now := time.Date(2026, 10, 5, 9, 0, 0, 0, time.FixedZone("test", 8*3600))
	dir := t.TempDir()
	pref := settings.Open(dir)
	p := pref.Value
	p.Edge = settings.Edge{DefaultSide: "left", DefaultDensity: "compact"}
	p.Startup.ShowMainWindow = true
	if e := pref.Save(p); e != nil {
		t.Fatal(e)
	}
	open := func(at time.Time) *storage.Store {
		t.Helper()
		p := settings.Open(dir)
		s, e := storage.OpenWithDefaults(dir, at, p.Value.Edge.DefaultSide, p.Value.Edge.DefaultDensity)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	s := open(now)
	defer func() { s.Close() }()
	v := mutate(t, s, now, todo.Action{Type: "create", Title: "Draft", DueAt: ptr(now.Add(10 * time.Minute).UnixMilli()), Remind: ptr(true)})
	a := v.Todos[0].ID
	if task(t, v, a).DisplayMode != "NONE" {
		t.Fatal("new task unexpectedly pinned")
	}
	v = mutate(t, s, now, todo.Action{Type: "create", Title: "Second", Pin: true})
	var b int64
	for _, x := range v.Todos {
		if x.ID != a {
			b = x.ID
		}
	}
	v = mutate(t, s, now, todo.Action{Type: "pin", ID: a})
	stack := v.Stacks[0]
	if stack.Side != "left" || stack.Density != "compact" {
		t.Fatal(stack)
	}
	v = mutate(t, s, now, todo.Action{Type: "reorder", StackID: stack.ID, PreviousOrder: []int64{b, a}, Order: []int64{a, b}})
	if task(t, v, a).SortOrder >= task(t, v, b).SortOrder {
		t.Fatal("sort order unchanged")
	}
	// Native startup resolves the initially empty display ID before movement.
	stack.DisplayID = "test-display"
	placed, placeErr := s.SaveStack(stack, now)
	if placeErr != nil {
		t.Fatal(placeErr)
	}
	v, stack = placed, placed.Stacks[0]
	target := stack
	target.Side = "right"
	target.Offset = .65
	before := v.Todos
	v, e := s.MoveStack(stack, target, now)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, v.Todos) {
		t.Fatal("move changed task content or order")
	}
	stack = v.Stacks[0]
	v = mutate(t, s, now, todo.Action{Type: "snooze", ID: a, Duration: "30m"})
	delayed := task(t, v, a)
	if delayed.DueAt != now.Add(10*time.Minute).UnixMilli() || delayed.RemindAt != now.Add(30*time.Minute).UnixMilli() {
		t.Fatal(delayed)
	}
	os := &notificationSystem{state: reminder.State{Authorization: "authorized", Delivered: []string{"other-profile.1"}}}
	result := syncAt(t, s, os, now.Add(10*time.Minute))
	if len(os.sent) != 0 || !result.Next.Equal(now.Add(30*time.Minute)) {
		t.Fatal(result, os.sent)
	}
	v = mutate(t, s, now.Add(29*time.Minute), todo.Action{Type: "complete", ID: a})
	syncAt(t, s, os, now.Add(29*time.Minute+time.Second))
	v = mutate(t, s, now.Add(29*time.Minute+2*time.Second), todo.Action{Type: "undo"})
	if task(t, v, a).Completed || task(t, v, a).SortOrder != delayed.SortOrder {
		t.Fatal("Undo lost ordering")
	}
	os.state.Authorization = "denied"
	syncAt(t, s, os, now.Add(30*time.Minute))
	if len(os.sent) != 0 || task(t, view(t, s, now), a).ReminderSentAt != 0 {
		t.Fatal("denied reminder consumed")
	}
	p.Edge = settings.Edge{DefaultSide: "left", DefaultDensity: "relaxed"}
	if e = pref.Save(p); e != nil {
		t.Fatal(e)
	}
	beforeRestart := view(t, s, now.Add(31*time.Minute))
	s.Close()
	s = open(now.Add(31 * time.Minute))
	afterRestart := view(t, s, now.Add(31*time.Minute))
	if !reflect.DeepEqual(beforeRestart.Todos, afterRestart.Todos) || afterRestart.Stacks[0] != stack {
		t.Fatal("restart lost tasks/layout")
	}
	if settings.Open(dir).Value != p {
		t.Fatal("preferences not restored")
	}
	os.state.Authorization = "authorized"
	syncAt(t, s, os, now.Add(31*time.Minute))
	if len(os.sent) != 1 || task(t, view(t, s, now), a).ReminderSentAt == 0 {
		t.Fatal("overdue reminder not delivered")
	}
	// Completion removes its system notification; Undo still must not send that generation twice.
	mutate(t, s, now.Add(32*time.Minute), todo.Action{Type: "complete", ID: a})
	syncAt(t, s, os, now.Add(32*time.Minute))
	if !reflect.DeepEqual(os.state.Delivered, []string{"other-profile.1"}) {
		t.Fatal("notification cleanup scope", os.state)
	}
	mutate(t, s, now.Add(32*time.Minute+time.Second), todo.Action{Type: "undo"})
	syncAt(t, s, os, now.Add(32*time.Minute+time.Second))
	s.Close()
	s = open(now.Add(33 * time.Minute))
	syncAt(t, s, os, now.Add(33*time.Minute))
	if len(os.sent) != 1 {
		t.Fatal("restart/Undo sent duplicate")
	}
	v = mutate(t, s, now.Add(34*time.Minute), todo.Action{Type: "create", Title: "Temporary", Temporary: ptr(true), Pin: true})
	var temporary int64
	for _, x := range v.Todos {
		if x.Temporary {
			temporary = x.ID
		}
	}
	mutate(t, s, now.Add(34*time.Minute), todo.Action{Type: "complete", ID: temporary})
	s.Close()
	s = open(now.Add(34*time.Minute + time.Second))
	if len(view(t, s, now).Todos) != 2 {
		t.Fatal("restart failed to clean temporary task")
	}
	mutate(t, s, now.Add(35*time.Minute), todo.Action{Type: "edit", ID: a, Title: "Draft", Remind: ptr(false)})
	syncAt(t, s, os, now.Add(35*time.Minute))
	rows, e := s.Reminders()
	if e != nil || len(rows) != 0 {
		t.Fatal("disabled reminder remains", rows, e)
	}
}

func TestSystemAcceptanceRecoversAfterDatabaseReceiptFailure(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1800000000, 0)
	s, e := storage.Open(dir, now)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { s.Close() }()
	v := mutate(t, s, now, todo.Action{Type: "create", Title: "Receipt recovery", DueAt: ptr(now.UnixMilli()), Remind: ptr(true), Pin: true})
	id := v.Todos[0].ID
	db, e := sql.Open("sqlite", filepath.Join(dir, "sidelet.sqlite3"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	_, e = db.Exec(`CREATE TRIGGER fail_receipt BEFORE UPDATE OF delivered_at ON reminders BEGIN SELECT RAISE(ABORT,'injected receipt failure'); END`)
	if e != nil {
		t.Fatal(e)
	}
	os := &notificationSystem{state: reminder.State{Authorization: "authorized"}}
	if _, e = reminder.Sync(s, os, "regression.", now); e == nil {
		t.Fatal("injected failure did not occur")
	}
	if len(os.sent) != 1 || task(t, view(t, s, now), id).ReminderSentAt != 0 {
		t.Fatal("failure evidence inconsistent")
	}
	s.Close()
	s, e = storage.Open(dir, now.Add(time.Minute))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`DROP TRIGGER fail_receipt`); e != nil {
		t.Fatal(e)
	}
	syncAt(t, s, os, now.Add(time.Minute))
	if len(os.sent) != 1 || task(t, view(t, s, now), id).ReminderSentAt == 0 {
		t.Fatal("recovery duplicated or lost receipt")
	}
	before := view(t, s, now)
	if _, e = db.Exec(`CREATE TRIGGER fail_edit BEFORE UPDATE ON todos BEGIN SELECT RAISE(ABORT,'injected task failure'); END`); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Apply(todo.Action{Type: "edit", ID: id, Title: "Must not save", DueAt: ptr(now.Add(time.Hour).UnixMilli())}, now); e == nil {
		t.Fatal("failed edit accepted")
	}
	if !reflect.DeepEqual(before, view(t, s, now)) {
		t.Fatal("failed edit changed tasks/layout/reminder")
	}
	syncAt(t, s, os, now)
	if len(os.sent) != 1 {
		t.Fatal("failed edit rearmed reminder")
	}
}
