package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"sidelet/internal/todo"
)

var testNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))

func openTest(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir, testNow)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}
func applyTest(t *testing.T, s *Store, action todo.Action, now time.Time) todo.Snapshot {
	t.Helper()
	state, err := s.Apply(action, now)
	if err != nil {
		t.Fatal(err)
	}
	return state
}
func viewTest(t *testing.T, s *Store, now time.Time) todo.Snapshot {
	t.Helper()
	state, err := s.Snapshot(now)
	if err != nil {
		t.Fatal(err)
	}
	return state
}
func int64ptr(value int64) *int64 { return &value }
func boolptr(value bool) *bool    { return &value }

func TestEmptyInstallationAndCurrentVersion(t *testing.T) {
	s, dir := openTest(t)
	state := viewTest(t, s, testNow)
	if len(state.Todos) != 0 || len(state.Stacks) != 1 || state.Storage != "sqlite" {
		t.Fatalf("unexpected initial state: %+v", state)
	}
	var version, count int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version=1`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion || count != 1 {
		t.Fatal("schema version was not recorded once")
	}
	s.Close()
	reopened, err := Open(dir, testNow.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if len(viewTest(t, reopened, testNow).Todos) != 0 {
		t.Fatal("reopen introduced fixture tasks")
	}
}

func TestCRUDPinLayoutAndRestart(t *testing.T) {
	s, dir := openTest(t)
	due := testNow.Add(24 * time.Hour).UnixMilli()
	state := applyTest(t, s, todo.Action{Type: "create", Title: "  持久任务 ' ; DROP TABLE todos; --  ", Description: "第一行\n第二行", DueAt: &due, Pin: true}, testNow)
	id := state.Todos[0].ID
	if state.Todos[0].Title != "持久任务 ' ; DROP TABLE todos; --" || state.Todos[0].DisplayMode != "EDGE" || state.Todos[0].SortOrder != 10 {
		t.Fatal("create/pin failed")
	}
	stack := state.Stacks[0]
	stack.Side = "left"
	stack.Offset = .77
	stack.DisplayID = "display-a"
	stack.Density = "relaxed"
	if _, err := s.SaveStack(stack, testNow); err != nil {
		t.Fatal(err)
	}
	state = applyTest(t, s, todo.Action{Type: "edit", ID: id, Title: "更新标题", Description: "保存后的备注"}, testNow.Add(time.Minute))
	if state.Todos[0].DueAt != due {
		t.Fatal("Quick Card edit cleared the deadline")
	}
	state = applyTest(t, s, todo.Action{Type: "complete", ID: id}, testNow.Add(2*time.Minute))
	completedAt := state.Todos[0].CompletedAt
	if state.UndoID != id || !state.Todos[0].Completed {
		t.Fatal("completion was not committed")
	}
	s.Close()
	reopened, err := Open(dir, testNow.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	state = viewTest(t, reopened, testNow.Add(time.Hour))
	item := state.Todos[0]
	if item.ID != id || item.Title != "更新标题" || item.Description != "保存后的备注" || item.DueAt != due || !item.Completed || item.CompletedAt != completedAt || item.SortOrder != 10 || item.StackID != stack.ID {
		t.Fatalf("restart lost task state: %+v", item)
	}
	if state.Stacks[0].Side != "left" || state.Stacks[0].Offset != .77 || state.Stacks[0].DisplayID != "display-a" || state.Stacks[0].Density != "relaxed" {
		t.Fatal("restart lost layout")
	}
	if state.UndoID != 0 {
		t.Fatal("restart restored an expired session Undo")
	}
	applyTest(t, reopened, todo.Action{Type: "reopen", ID: id}, testNow.Add(time.Hour))
	state = applyTest(t, reopened, todo.Action{Type: "delete", ID: id}, testNow.Add(time.Hour))
	if len(state.Todos) != 0 {
		t.Fatal("deleted task remained in state")
	}
	for _, table := range []string{"desktop_presentations", "edge_stack_items", "reminders"} {
		var count int
		if err := reopened.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("delete orphaned %s", table)
		}
	}
}

func TestFailedPinCreationRollsBackEverything(t *testing.T) {
	s, _ := openTest(t)
	if _, err := s.Apply(todo.Action{Type: "create", Title: "必须回滚", Pin: true, StackID: 999}, testNow); err == nil {
		t.Fatal("invalid target succeeded")
	}
	if len(viewTest(t, s, testNow).Todos) != 0 {
		t.Fatal("failed transaction left a task")
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM desktop_presentations`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed transaction left a presentation")
	}
	state := applyTest(t, s, todo.Action{Type: "create", Title: "未固定"}, testNow)
	if state.Todos[0].DisplayMode != "NONE" || state.Todos[0].StackID != 0 {
		t.Fatal("create pinned without user intent")
	}
}

func TestFailedWritePreservesContentAndUndo(t *testing.T) {
	s, _ := openTest(t)
	first := applyTest(t, s, todo.Action{Type: "create", Title: "first"}, testNow).Todos[0].ID
	state := applyTest(t, s, todo.Action{Type: "create", Title: "second"}, testNow)
	second := state.Todos[1].ID
	state = applyTest(t, s, todo.Action{Type: "complete", ID: first}, testNow)
	if _, err := s.db.Exec(`CREATE TRIGGER fail_update BEFORE UPDATE ON todos BEGIN SELECT RAISE(ABORT, 'forced write failure'); END`); err != nil {
		t.Fatal(err)
	}
	for _, action := range []todo.Action{{Type: "edit", ID: second, Title: "unsaved draft"}, {Type: "complete", ID: second}} {
		if _, err := s.Apply(action, testNow.Add(time.Second)); err == nil {
			t.Fatal("write should have failed")
		}
		current := viewTest(t, s, testNow.Add(time.Second))
		if current.Todos[1].Title != "second" || current.Todos[1].Completed || current.UndoID != first || current.UndoUntil != state.UndoUntil {
			t.Fatalf("failed transaction changed saved state: %+v", current)
		}
	}
	if _, err := s.db.Exec(`DROP TRIGGER fail_update`); err != nil {
		t.Fatal(err)
	}
	state = applyTest(t, s, todo.Action{Type: "undo"}, testNow.Add(2*time.Second))
	if state.Todos[0].Completed {
		t.Fatal("prior Undo was lost after a failed write")
	}
}

func TestUndoSnoozeAndMembershipPreserveOrder(t *testing.T) {
	s, dir := openTest(t)
	applyTest(t, s, todo.Action{Type: "create", Title: "A", Pin: true}, testNow)
	state := applyTest(t, s, todo.Action{Type: "create", Title: "B", Pin: true, DueAt: int64ptr(testNow.UnixMilli())}, testNow)
	id := state.Todos[0].ID
	order := state.Todos[0].SortOrder
	state = applyTest(t, s, todo.Action{Type: "complete", ID: id}, testNow)
	deadline := state.UndoUntil
	state = applyTest(t, s, todo.Action{Type: "complete", ID: id}, testNow.Add(time.Second))
	if state.UndoUntil != deadline || state.Todos[0].SortOrder != order {
		t.Fatal("repeated completion modified Undo or order")
	}
	state = applyTest(t, s, todo.Action{Type: "undo"}, testNow.Add(4999*time.Millisecond))
	if state.Todos[0].Completed || state.Todos[0].CompletedAt != 0 || state.Todos[0].SortOrder != order {
		t.Fatal("Undo lost original membership")
	}
	state = applyTest(t, s, todo.Action{Type: "snooze", ID: id, Duration: "tomorrow"}, testNow)
	want := time.Date(2026, 10, 6, 9, 0, 0, 0, testNow.Location()).UnixMilli()
	if state.Todos[0].SnoozedUntil != want || state.Todos[0].SortOrder != order {
		t.Fatal("snooze changed layout")
	}
	s.Close()
	other, err := Open(dir, testNow.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if viewTest(t, other, testNow).Todos[0].SnoozedUntil != want {
		t.Fatal("restart lost snooze")
	}
	applyTest(t, other, todo.Action{Type: "complete", ID: id}, testNow)
	if _, err := other.Apply(todo.Action{Type: "undo"}, testNow.Add(5001*time.Millisecond)); err == nil {
		t.Fatal("expired Undo succeeded")
	}
	if !viewTest(t, other, testNow).Todos[0].Completed {
		t.Fatal("expired Undo modified content")
	}
}

func TestTemporaryCleanupAndUndoBoundary(t *testing.T) {
	s, dir := openTest(t)
	state := applyTest(t, s, todo.Action{Type: "create", Title: "临时", Temporary: boolptr(true), Pin: true}, testNow)
	id := state.Todos[0].ID
	applyTest(t, s, todo.Action{Type: "complete", ID: id}, testNow)
	state = applyTest(t, s, todo.Action{Type: "cleanup"}, testNow.Add(5000*time.Millisecond))
	if len(state.Todos) != 1 {
		t.Fatal("cleanup deleted a task inside the inclusive Undo boundary")
	}
	state = applyTest(t, s, todo.Action{Type: "cleanup"}, testNow.Add(5001*time.Millisecond))
	if len(state.Todos) != 0 {
		t.Fatal("expired temporary task remained")
	}
	state = applyTest(t, s, todo.Action{Type: "create", Title: "重启清理", Temporary: boolptr(true), Pin: true}, testNow)
	applyTest(t, s, todo.Action{Type: "complete", ID: state.Todos[0].ID}, testNow)
	s.Close()
	other, err := Open(dir, testNow.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if len(viewTest(t, other, testNow).Todos) != 0 {
		t.Fatal("restart failed to clean completed temporary tasks")
	}
}

func TestMigrationFailureAndFutureSchemaDoNotResetData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sidelet.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE todos(legacy_payload TEXT);INSERT INTO todos VALUES('keep me')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := Open(dir, testNow); err == nil {
		s.Close()
		t.Fatal("incompatible schema was replaced")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var payload string
	if err = db.QueryRow(`SELECT legacy_payload FROM todos`).Scan(&payload); err != nil || payload != "keep me" {
		t.Fatal("migration failure changed old data")
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='schema_migrations'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed migration was partially committed")
	}
	s, futureDir := openTest(t)
	state := applyTest(t, s, todo.Action{Type: "create", Title: "新版数据"}, testNow)
	if _, err = s.db.Exec(`INSERT INTO schema_migrations VALUES(3,123);PRAGMA user_version=3`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if other, err := Open(futureDir, testNow); err == nil {
		other.Close()
		t.Fatal("older application accepted a future schema")
	}
	future, err := sql.Open("sqlite", filepath.Join(futureDir, "sidelet.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer future.Close()
	if err = future.QueryRow(`SELECT title FROM todos WHERE id=?`, state.Todos[0].ID).Scan(&payload); err != nil || payload != "新版数据" {
		t.Fatal("future schema rejection erased content")
	}
}

func TestValidationAndForeignKeys(t *testing.T) {
	s, _ := openTest(t)
	for _, action := range []todo.Action{{Type: "create", Title: "  "}, {Type: "create", Title: strings.Repeat("字", 501)}, {Type: "create", Title: "A", DueAt: int64ptr(-1)}, {Type: "reset"}} {
		if _, err := s.Apply(action, testNow); err == nil {
			t.Fatalf("invalid action accepted: %+v", action)
		}
	}
	state := applyTest(t, s, todo.Action{Type: "create", Title: "约束测试", Pin: true}, testNow)
	if _, err := s.db.Exec(`INSERT INTO edge_stack_items(stack_id,todo_id,sort_order) VALUES(?,?,20)`, state.Stacks[0].ID, state.Todos[0].ID); err == nil {
		t.Fatal("one task was assigned twice")
	}
	if _, err := s.db.Exec(`INSERT INTO reminders(todo_id,remind_at,created_at) VALUES(999,1,1)`); err == nil {
		t.Fatal("connection has foreign keys disabled")
	}
	state = applyTest(t, s, todo.Action{Type: "edit", ID: state.Todos[0].ID, Title: "约束测试", DueAt: int64ptr(0)}, testNow)
	if state.Todos[0].DueAt != 0 {
		t.Fatal("clearing a deadline failed")
	}
}

func TestConcurrentWritesAndUnreadableDatabase(t *testing.T) {
	s, _ := openTest(t)
	var writers sync.WaitGroup
	errorsOut := make(chan error, 16)
	for i := 0; i < 16; i++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			_, err := s.Apply(todo.Action{Type: "create", Title: "并发任务", Pin: true}, testNow)
			errorsOut <- err
		}()
	}
	writers.Wait()
	close(errorsOut)
	for err := range errorsOut {
		if err != nil {
			t.Fatal(err)
		}
	}
	state := viewTest(t, s, testNow)
	if len(state.Todos) != 16 {
		t.Fatal("concurrent write lost a task")
	}
	for i, item := range state.Todos {
		if item.SortOrder != (i+1)*10 {
			t.Fatal("concurrent pin changed order")
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "sidelet.sqlite3")
	content := []byte("this is not a database")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	if store, err := Open(dir, testNow); err == nil {
		store.Close()
		t.Fatal("corrupt database was accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(content) {
		t.Fatal("open failure overwrote corrupt data")
	}
}
