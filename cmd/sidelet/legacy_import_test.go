package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"sidelet/internal/settings"
	"sidelet/internal/storage"
	"sidelet/internal/todo"
)

func TestLegacyImportBacksUpEmptyJSONAndRejectsJSONTasks(t *testing.T) {
	source, _, _, now := legacyFixture(t)
	for _, hasTasks := range []bool{false, true} {
		dest := t.TempDir()
		m := newModel()
		m.UITheme = "mac"
		m.Tasks = nil
		if hasTasks {
			m.Tasks = []task{{Title: "保留原生任务", Version: 1}}
		}
		v := m.profileState("00112233445566778899aabbccddeeff")
		raw, _ := json.Marshal(v)
		path := filepath.Join(dest, "profile.json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		r, err := importLegacyProfile(source, dest, now)
		if hasTasks {
			if err == nil {
				t.Fatal("JSON tasks overwritten")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		backup, e := os.ReadFile(filepath.Join(filepath.Dir(r.Backup), "native-profile-before.json"))
		if e != nil || string(backup) != string(raw) {
			t.Fatal("empty native profile backup missing")
		}
		kept, _ := os.ReadFile(path)
		if string(kept) != string(raw) {
			t.Fatal("source JSON changed")
		}
		p, e := openProfile(dest, m)
		if e != nil {
			t.Fatal(e)
		}
		if p.saved.ID != v.ID || p.saved.LegacyImport == nil {
			t.Fatal("profile identity lost")
		}
		p.close()
	}
}

func TestLegacyImportFailureRollsBackAndAllowsRetry(t *testing.T) {
	source, _, _, now := legacyFixture(t)
	dest := t.TempDir()
	store, err := storage.OpenNativeProfile(dest)
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	db, err := sql.Open("sqlite", filepath.Join(dest, storage.NativeProfileFilename))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_import BEFORE INSERT ON native_tasks BEGIN SELECT RAISE(ABORT,'disk write failed'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = importLegacyProfile(source, dest, now); err == nil {
		t.Fatal("write failure ignored")
	}
	var count int
	if err = db.QueryRow("SELECT count(*) FROM native_profile").Scan(&count); err != nil || count != 0 {
		t.Fatal("partial metadata committed", count, err)
	}
	if _, err = db.Exec("DROP TRIGGER fail_import"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if r, err := importLegacyProfile(source, dest, now); err != nil || r.AlreadyImported || r.Tasks != 2 {
		t.Fatal("retry failed", r, err)
	}
}

func legacyFixture(t *testing.T) (string, todo.Snapshot, settings.Value, time.Time) {
	t.Helper()
	dir := t.TempDir()
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	s, err := storage.Open(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	due := now.Add(time.Hour).UnixMilli()
	remind := true
	priority := 3
	for _, a := range []todo.Action{
		{Type: "create", Title: "中文任务", Description: "多行\n备注", Pin: true, DueAt: &due, Remind: &remind, Priority: &priority},
		{Type: "create", Title: "已完成的任务"},
	} {
		if _, err = s.Apply(a, now); err != nil {
			t.Fatal(err)
		}
	}
	state, err := s.Snapshot(now)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range state.Todos {
		if item.Title == "已完成的任务" {
			if _, err = s.Apply(todo.Action{Type: "complete", ID: item.ID}, now); err != nil {
				t.Fatal(err)
			}
		}
	}
	reminders, err := s.Reminders()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.MarkReminderSent(reminders[0].ID, now); err != nil {
		t.Fatal(err)
	}
	stack := state.Stacks[0]
	stack.Side, stack.Offset, stack.Density = "left", .72, "compact"
	if _, err = s.SaveStack(stack, now); err != nil {
		t.Fatal(err)
	}
	state, err = s.Snapshot(now)
	if err != nil {
		t.Fatal(err)
	}
	prefs := settings.Defaults()
	prefs.Appearance.Theme = "paper"
	prefs.Startup.ShowMainWindow = true
	if err = settings.Open(dir).Save(prefs); err != nil {
		t.Fatal(err)
	}
	return dir, state, prefs, now
}

func TestLegacyImportRoundTripAndDurableDeduplication(t *testing.T) {
	source, want, prefs, now := legacyFixture(t)
	dest := t.TempDir()
	r, err := importLegacyProfile(source, dest, now)
	if err != nil {
		t.Fatal(err)
	}
	if r.Tasks != len(want.Todos) || r.AlreadyImported {
		t.Fatal(r)
	}
	backup, err := storage.ReadLegacyBackup(r.Backup)
	if err != nil || !reflect.DeepEqual(backup.Todos, want.Todos) {
		t.Fatal("backup mismatch", err)
	}
	backupPrefs := settings.Open(filepath.Dir(r.Backup))
	if backupPrefs.Value != prefs {
		t.Fatal("preference backup mismatch")
	}
	m := newModel()
	m.UITheme = "mac"
	m.clock = func() time.Time { return now }
	p, err := openProfile(dest, m)
	if err != nil {
		t.Fatal(err)
	}
	if m.Preferences != prefs || m.UITheme != "paper" || m.Side != "left" || m.Offset != .72 || m.ItemHeight != 40 {
		t.Fatal("settings/layout lost")
	}
	for i, old := range want.Todos {
		got := m.Tasks[i]
		if got.Title != old.Title || got.Note != old.Description || got.Done != old.Completed || got.CompletedAt != old.CompletedAt || got.DueAt != old.DueAt || got.Remind != old.Remind || got.ReminderSentAt != old.ReminderSentAt || got.Priority != old.Priority || got.Unpinned != (old.DisplayMode != "EDGE") || got.CreatedAt != old.CreatedAt || got.UpdatedAt != old.UpdatedAt {
			t.Fatal("task metadata lost", i)
		}
	}
	if err = m.commit(func() error { m.Tasks[0].Note = "导入后修改"; return nil }); err != nil {
		t.Fatal(err)
	}
	p.close()
	r2, err := importLegacyProfile(source, dest, now.Add(time.Hour))
	if err != nil || !r2.AlreadyImported || r2.Backup != r.Backup {
		t.Fatal("reimport was not a no-op", r2, err)
	}
	p, err = openProfile(dest, m)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()
	if m.Tasks[0].Note != "导入后修改" || len(m.Tasks) != len(want.Todos) {
		t.Fatal("reimport overwrote native tasks")
	}
}

func TestLegacyImportRejectsExistingTasksAndInvalidSource(t *testing.T) {
	source, _, _, now := legacyFixture(t)
	dest := t.TempDir()
	m := newModel()
	m.UITheme = "mac"
	p, err := openProfile(dest, m)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.commit(func() error { m.Tasks = append(m.Tasks, task{Title: "已有任务", Version: 1}); return nil }); err != nil {
		t.Fatal(err)
	}
	want := p.saved
	p.close()
	if _, err = importLegacyProfile(source, dest, now); err == nil {
		t.Fatal("existing tasks overwritten")
	}
	p, err = openProfile(dest, m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, p.saved) {
		t.Fatal("rejected import changed state")
	}
	p.close()
	if _, err = importLegacyProfile(source, source, now); err == nil {
		t.Fatal("source accepted as destination")
	}
	empty := t.TempDir()
	if err = os.WriteFile(filepath.Join(source, "settings.json"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = importLegacyProfile(source, empty, now); err == nil {
		t.Fatal("invalid preferences accepted")
	}
	db, err := storage.OpenNativeProfile(empty)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	data, err := db.Read()
	if err != nil || data != nil {
		t.Fatal("failed import committed partial data")
	}
}

func TestLegacyConversionRejectsMultipleGroupsAndInvalidFields(t *testing.T) {
	_, snapshot, prefs, _ := legacyFixture(t)
	snapshot.Stacks = append(snapshot.Stacks, snapshot.Stacks[0])
	if _, err := convertLegacy(snapshot, prefs); err == nil {
		t.Fatal("multiple layouts silently flattened")
	}
	snapshot.Stacks = snapshot.Stacks[:1]
	snapshot.Todos[0].Title = ""
	if _, err := convertLegacy(snapshot, prefs); err == nil {
		t.Fatal("invalid task accepted")
	}
}
