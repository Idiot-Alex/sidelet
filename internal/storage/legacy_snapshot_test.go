package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"sidelet/internal/todo"
)

func TestLegacyBackupIncludesWALWithoutRestartCleanup(t *testing.T) {
	s, dir := openTest(t)
	state := applyTest(t, s, todo.Action{Type: "create", Title: "WAL 中的任务", Pin: true}, testNow)
	// Model a completed temporary row still inside its original undo window.
	if _, err := s.db.Exec("UPDATE todos SET temporary=1,completed=1,completed_at=? WHERE id=?", testNow.UnixMilli(), state.Todos[0].ID); err != nil {
		t.Fatal(err)
	}
	want := viewTest(t, s, testNow)
	backup := filepath.Join(t.TempDir(), "backup.sqlite3")
	got, err := BackupLegacy(dir, backup)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("backup changed or dropped source state")
	}
	if !reflect.DeepEqual(viewTest(t, s, testNow), want) {
		t.Fatal("source modified")
	}
	if _, err = BackupLegacy(dir, backup); err == nil {
		t.Fatal("existing backup accepted")
	}
	info, _ := os.Stat(backup)
	if info.Mode().Perm() != 0600 {
		t.Fatal("backup is not private")
	}
	if _, err = s.db.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	if _, err = BackupLegacy(dir, filepath.Join(t.TempDir(), "future.sqlite3")); err == nil {
		t.Fatal("future schema accepted")
	}
}
