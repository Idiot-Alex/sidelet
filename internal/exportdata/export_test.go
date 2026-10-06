package exportdata

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"sidelet/internal/todo"
)

func exportFixture() todo.Snapshot {
	return todo.Snapshot{UndoID: 2, UndoUntil: 123, SelectedID: 1, OverflowIDs: []int64{2}, Storage: "sqlite",
		Todos: []todo.Todo{
			{ID: 1, Title: "中文，逗号与\"引号\"", Description: "第一行\n第二行 😀", DueAt: 1791244800123, Remind: true, RemindAt: 1791244800123, ReminderSentAt: 1791244800456, Priority: 3, Temporary: true, CreatedAt: 1791158400000, UpdatedAt: 1791158401000, DisplayMode: "EDGE", StackID: 7, SortOrder: 20},
			{ID: 2, Title: "=1+1", Completed: true, CompletedAt: 1791244800000, DisplayMode: "NONE"},
			{ID: 3, Title: "延后任务", SnoozedUntil: 1791250000000, DisplayMode: "EDGE", StackID: 7, SortOrder: 30},
		}, Stacks: []todo.EdgeStack{{ID: 7, DisplayID: "Retina 屏幕", Side: "left", Offset: .42, Density: "compact", CreatedAt: 1791158400000, UpdatedAt: 1791158401000}}}
}

func TestJSONPreservesAllTasksAndLayout(t *testing.T) {
	state := exportFixture()
	now := time.Date(2026, 10, 6, 9, 1, 2, 123000000, time.FixedZone("CST", 8*3600))
	data, err := Encode(state, "json", "0.1.0", now)
	if err != nil {
		t.Fatal(err)
	}
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Format != "sidelet" || doc.Version != Version || doc.AppVersion != "0.1.0" || doc.ExportedAt != "2026-10-06T01:01:02.123Z" {
		t.Fatalf("metadata: %+v", doc)
	}
	if !reflect.DeepEqual(state.Todos, doc.Todos) || !reflect.DeepEqual(state.Stacks, doc.Stacks) {
		t.Fatal("task or layout fields lost")
	}
	for _, key := range []string{"undoID", "undoUntil", "selectedID", "overflowIDs", "storage"} {
		if bytes.Contains(data, []byte(`"`+key+`"`)) {
			t.Fatalf("runtime field exported: %s", key)
		}
	}
}

func TestCSVUnicodeEscapingAndTimes(t *testing.T) {
	state := exportFixture()
	data, err := Encode(state, "csv", "0.1.0", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) || !bytes.Contains(data, []byte("\r\n")) {
		t.Fatal("missing UTF-8 BOM or CRLF")
	}
	rows, err := csv.NewReader(bytes.NewReader(data[3:])).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || len(rows[0]) != 21 {
		t.Fatalf("shape: %v", rows)
	}
	if rows[1][1] != state.Todos[0].Title || rows[1][2] != state.Todos[0].Description {
		t.Fatalf("text escaping: %v", rows[1])
	}
	if rows[1][6] != "2026-10-06T00:00:00.123Z" || rows[1][17] != "Retina 屏幕" || rows[1][18] != "left" || rows[1][19] != "0.42" || rows[1][20] != "compact" {
		t.Fatalf("time/layout: %v", rows[1])
	}
	if rows[2][1] != "'=1+1" || rows[2][3] != "true" || rows[2][6] != "" || rows[2][17] != "" {
		t.Fatalf("completed/unpinned: %v", rows[2])
	}
	if rows[3][8] == "" {
		t.Fatal("snoozed task omitted")
	}
}

func TestCSVFormulaProtection(t *testing.T) {
	for _, value := range []string{"=SUM(A1)", "+1", "-1", "@SUM(A1)", "  =1", "\tplain", "\nplain", "\rplain", "\u3000=1"} {
		if cell(value) != "'"+value {
			t.Errorf("unprotected %q", value)
		}
	}
	for _, value := range []string{"", "中文", "a,b", "two\nlines", "quoted\"text", "'already text", "10"} {
		if cell(value) != value {
			t.Errorf("altered ordinary text %q", value)
		}
	}
}

func TestEmptyExportAndInvalidFormat(t *testing.T) {
	data, err := Encode(todo.Snapshot{}, "json", "test", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"todos": []`)) || !bytes.Contains(data, []byte(`"stacks": []`)) {
		t.Fatal("empty lists must be arrays")
	}
	data, err = Encode(todo.Snapshot{}, "csv", "test", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(data[3:])).ReadAll()
	if err != nil || len(rows) != 1 {
		t.Fatalf("empty CSV: %v %v", rows, err)
	}
	if _, err := Encode(todo.Snapshot{}, "xml", "test", time.Now()); err == nil {
		t.Fatal("invalid format accepted")
	}
}

func TestSaveReplacementAndFailures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.json")
	if err := os.WriteFile(path, []byte("old export"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, "json", []byte("new export"), ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new export" {
		t.Fatalf("save: %q %v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatalf("file permissions: %v %v", info, err)
	}
	if err := Save(path, "csv", []byte("wrong"), ""); err == nil {
		t.Fatal("wrong extension accepted")
	}
	data, _ = os.ReadFile(path)
	if string(data) != "new export" {
		t.Fatal("failed save replaced existing export")
	}
	blocked := filepath.Join(dir, "folder.json")
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := Save(blocked, "json", []byte("new"), ""); err == nil {
		t.Fatal("rename onto directory accepted")
	}
	if err := Save(filepath.Join(dir, "missing", "tasks.json"), "json", []byte("new"), ""); err == nil {
		t.Fatal("missing parent accepted")
	}
	files, _ := os.ReadDir(dir)
	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".sidelet-export-") {
			t.Fatal("staging file leaked after failure")
		}
	}
}

func TestSaveProtectsProfileAndSymlink(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, "profile")
	if err := os.MkdirAll(filepath.Join(profile, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(profile, "settings.json")
	if err := os.WriteFile(settings, []byte("original settings"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	paths := []string{settings, filepath.Join(profile, "child", "tasks.json")}
	if err := os.Symlink(profile, alias); err == nil {
		paths = append(paths, filepath.Join(alias, "settings.json"))
	} else if runtime.GOOS != "windows" {
		t.Fatal(err)
	}
	// This alias exists only on a case-insensitive filesystem (the usual Mac
	// configuration). It must not bypass the live profile protection.
	if info, err := os.Stat(filepath.Join(dir, "PROFILE")); err == nil && info.IsDir() {
		paths = append(paths, filepath.Join(dir, "PROFILE", "child", "tasks.json"))
	}
	for _, path := range paths {
		if err := Save(path, "json", []byte("export"), profile); err == nil {
			t.Fatalf("profile path accepted: %s", path)
		}
	}
	data, _ := os.ReadFile(settings)
	if string(data) != "original settings" {
		t.Fatal("settings overwritten")
	}
	if err := Save(filepath.Join(dir, "profile-other.json"), "json", []byte("okay"), profile); err != nil {
		t.Fatal(err)
	}
}
