package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"sidelet/internal/storage"
)

// Exercise the packaged executable's startup migration, not openProfile in the
// test process. The fixture has the schema used through build 42, not an old
// executable; installer version replacement is covered separately.
func TestPackagedApplicationUpgrade(t *testing.T) {
	binary := packagedExecutable(t)
	root := packagedFixtureRoot(t, "packaged-upgrade-")
	dir := filepath.Join(root, "profile 中文")
	m, s, p := profileFixture(t, dir)
	m.clock = time.Now
	if _, err := s.SaveTask(0, "升级前的中文任务", "原备注\n第二行", 2, true, 0, false, false, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveTask(0, "升级前已完成提醒", "回执和版本需要保留", 3, false, time.Now().Add(-time.Minute).UnixMilli(), true, false, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetDone(2, true, m.Tasks[1].Version); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Theme("graphite"); err != nil {
		t.Fatal(err)
	}
	if err := m.commit(func() error {
		m.UndoID, m.UndoUntil = 0, 0
		m.Tasks[1].ReminderSentAt = time.Now().Add(-30 * time.Second).UnixMilli()
		m.Order, m.Side, m.Offset = []int{2, 1}, "left", .62
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := m.profileState(p.saved.ID)
	p.close()
	path := filepath.Join(dir, storage.NativeProfileFilename)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Only this newly created synthetic fixture is changed. No public storage
	// opener is used after this point until the EXE has completed migration.
	_, err = db.Exec("DROP TABLE native_new_task_draft; PRAGMA user_version=1;")
	closeErr := db.Close()
	if err != nil || closeErr != nil {
		t.Fatal("create version-one fixture", err, closeErr)
	}
	checkPackagedSchema(t, path, 1, 0)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(root, "before-upgrade.sqlite3")
	if err = os.WriteFile(backup, before, 0600); err != nil {
		t.Fatal(err)
	}
	stages := []string{}
	defer func() {
		data, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "stages": stages,
			"scope": "packaged executable upgrades a synthetic SQLite v1 fixture; no historical EXE or personal data"}, "", "  ")
		if err := os.WriteFile(filepath.Join(root, "upgrade.json"), data, 0600); err != nil {
			t.Error(err)
		}
	}()
	logPath := runPackagedProfile(t, binary, dir, root, "upgrade")
	checkPackagedSchema(t, path, 2, 0) // Must precede readSmokeProfile: that helper can migrate.
	checkSmokeStartup(t, logPath, want.ID, nil)
	if got := readSmokeProfile(t, dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("packaged migration changed tasks, IDs, order, preferences or receipts\ngot=%+v\nwant=%+v", got, want)
	}
	stages = append(stages, "packaged-startup-migrates-v1-to-v2", "tasks-settings-order-and-reminder-receipts-preserved")

	// The migrated profile must release its lock and accept the new draft row.
	// Seed via the regular persistence path while the EXE is stopped, then
	// verify that the actual next process restores this draft into its view.
	m2, s2, p2 := profileFixture(t, dir)
	v := newNativeTasksView(s2)
	v.newTitle, v.formNote, v.noteOpen = "迁移后新建草稿", "升级后仍可保存\n重启继续填写", true
	if !v.flushNewTaskDraft() {
		t.Fatal(v.draftSaveError)
	}
	want = m2.profileState(p2.saved.ID)
	p2.close()
	logPath = runPackagedProfile(t, binary, dir, root, "restart-with-draft")
	checkPackagedSchema(t, path, 2, 1)
	checkSmokeStartup(t, logPath, want.ID, want.NewTaskDraft)
	if got := readSmokeProfile(t, dir); !reflect.DeepEqual(got, want) {
		t.Fatal("restart changed migrated profile or lost new draft")
	}
	_, _, p3 := profileFixture(t, dir)
	p3.close()
	stages = append(stages, "new-draft-writable-and-restored-after-upgrade", "normal-exit-releases-profile-lock")
	retained, err := os.ReadFile(backup)
	if err != nil || !bytes.Equal(before, retained) {
		t.Fatal("pre-upgrade backup was changed", err)
	}
	stages = append(stages, "pre-upgrade-backup-unchanged")
}

func checkPackagedSchema(t *testing.T, path string, version, drafts int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var actual, tables int
	var integrity string
	if err := db.QueryRow("PRAGMA user_version").Scan(&actual); err != nil || actual != version {
		t.Fatalf("SQLite version=%d want=%d: %v", actual, version, err)
	}
	if err := db.QueryRow("PRAGMA quick_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal("SQLite integrity", integrity, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='native_new_task_draft'").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if version == 1 {
		if tables != 0 {
			t.Fatal("old-schema fixture already has a draft table")
		}
		return
	}
	if tables != 1 {
		t.Fatal("packaged app did not create draft table")
	}
	if err := db.QueryRow("SELECT count(*) FROM native_new_task_draft").Scan(&actual); err != nil || actual != drafts {
		t.Fatal("unexpected persisted draft count", actual, err)
	}
}
