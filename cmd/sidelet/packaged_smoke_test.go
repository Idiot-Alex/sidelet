package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"sidelet/internal/storage"
)

// This opt-in test launches the already packaged executable, not a freshly
// compiled test replacement. All data and diagnostics live in an owned fixture
// directory. No diagnostic write command is added to the production CLI.
func TestPackagedApplicationSmoke(t *testing.T) {
	binary := os.Getenv("SIDELET_SMOKE_EXE")
	if binary == "" {
		t.Skip("set SIDELET_SMOKE_EXE to test a packaged native executable")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(binary); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("packaged executable unavailable: %v", err)
	}
	root := t.TempDir()
	if output := os.Getenv("SIDELET_SMOKE_OUTPUT"); output != "" {
		if err := os.MkdirAll(output, 0700); err != nil {
			t.Fatal(err)
		}
		root, err = os.MkdirTemp(output, "packaged-smoke-")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("retained synthetic fixture and logs: %s", root)
	}
	profileDir := filepath.Join(root, "profile")
	m := newModel()
	m.UITheme, m.Side = "mac", "right"
	// Seed an expired temporary task in the past. The packaged process must
	// clean it up and commit the tombstone itself at its real startup time.
	m.clock = func() time.Time { return time.Now().Add(-time.Minute) }
	p, err := openProfile(profileDir, m)
	if err != nil {
		t.Fatal(err)
	}
	s := testService(m)
	for _, item := range []struct {
		title, note       string
		priority          int
		pinned, temporary bool
	}{
		{"CI 中文持久任务 🔔", "合成备注\n第二行", 2, true, false},
		{"CI 未固定任务", "只用于自动验收", 0, false, false},
		{"CI 待清理临时任务", "", 0, false, true},
	} {
		if _, err = s.SaveTask(0, item.title, item.note, item.priority, item.pinned, 0, false, item.temporary, 0); err != nil {
			p.close()
			t.Fatal(err)
		}
	}
	if _, err = s.SetDone(3, true, m.Tasks[2].Version); err != nil {
		p.close()
		t.Fatal(err)
	}
	// The packaged view must restore the draft, including an incomplete date,
	// and preserve it through cleanup transactions and normal application quit.
	v := newNativeTasksView(s)
	v.newTitle, v.formNote, v.newPriority = "CI 重启后的未提交草稿", "草稿备注\n尚未创建任务", "重要"
	v.formDue, v.optionsOpen, v.noteOpen, v.formPin = "2027// :", true, true, true
	v.deadline = deadlineSegments{parts: [5]string{"2027", "", "", "", ""}, seen: v.formDue, ready: true, focused: -1}
	if !v.flushNewTaskDraft() {
		p.close()
		t.Fatal(v.draftSaveError)
	}
	want := m.profileState(p.saved.ID)
	p.close()
	initial := readSmokeProfile(t, profileDir)
	if initial.Tasks[2].Deleted {
		t.Fatal("fixture cleaned up before the packaged executable ran")
	}
	want.Tasks[2] = task{Deleted: true, Version: want.Tasks[2].Version + 1}
	want.UndoID, want.UndoUntil = 0, 0
	for run := 1; run <= 2; run++ {
		logPath := filepath.Join(root, fmt.Sprintf("run-%d.log", run))
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		args := []string{"-main", "-data-dir", profileDir,
			"-output", filepath.Join(root, fmt.Sprintf("run-%d", run)), "-log-file", logPath, "-quit-after", "3s"}
		if runtime.GOOS == "darwin" {
			args = append(args, "-quick-add-test-shortcut")
		}
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.WaitDelay = 5 * time.Second
		started := time.Now()
		out, err := cmd.CombinedOutput()
		if len(out) > 0 {
			_ = os.WriteFile(filepath.Join(root, fmt.Sprintf("run-%d-process.log", run)), out, 0600)
		}
		timedOut := ctx.Err() != nil
		cancel()
		if err != nil || timedOut {
			t.Fatalf("packaged run %d failed (timeout=%t): %v\n%s", run, timedOut, err, out)
		}
		if time.Since(started) < 2*time.Second {
			t.Fatalf("run %d exited before its application timer", run)
		}
		checkSmokeStartup(t, logPath, p.saved.ID, want.NewTaskDraft)
		got := readSmokeProfile(t, profileDir)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("SQLite state changed unexpectedly after run %d\ngot: %+v\nwant: %+v", run, got, want)
		}
		// A normal exit must release the profile lock as well as the database.
		probe := newModel()
		probe.UITheme, probe.Side = "mac", "right"
		reopened, err := openProfile(profileDir, probe)
		if err != nil {
			t.Fatalf("profile remains locked after run %d: %v", run, err)
		}
		reopened.close()
	}
}

func readSmokeProfile(t *testing.T, dir string) profileState {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, storage.NativeProfileFilename))
	if err != nil || !bytes.HasPrefix(data, []byte("SQLite format 3\x00")) {
		t.Fatalf("not a SQLite database: %v", err)
	}
	p, err := storage.OpenNativeProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	data, err = p.Read()
	if err != nil {
		t.Fatal(err)
	}
	var state profileState
	if err = json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func checkSmokeStartup(t *testing.T, path, profileID string, draft *persistedNewTaskDraft) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	ready := false
	for scanner.Scan() {
		line := scanner.Text()
		at := strings.IndexByte(line, '{')
		if at < 0 {
			continue
		}
		var entry struct {
			Event        string                 `json:"event"`
			MainOpen     bool                   `json:"mainOpen"`
			MainRenderer string                 `json:"mainRenderer"`
			StorageError string                 `json:"storageError"`
			ProfileID    string                 `json:"profileID"`
			Tasks        taskSnapshot           `json:"taskSnapshot"`
			NewTaskDraft *persistedNewTaskDraft `json:"mainNewTaskDraft"`
		}
		if err = json.Unmarshal([]byte(line[at:]), &entry); err != nil {
			t.Fatalf("invalid diagnostic record: %v", err)
		}
		if entry.StorageError != "" {
			t.Fatalf("packaged app reported storage error: %s", entry.StorageError)
		}
		if entry.Event == "startup-presentation" {
			if !entry.MainOpen || entry.MainRenderer != "native" || entry.ProfileID != profileID || len(entry.Tasks.Tasks) != 2 {
				t.Fatalf("native window/profile did not reach startup: %+v", entry)
			}
			if !equalNewTaskDraft(entry.NewTaskDraft, draft) {
				t.Fatalf("packaged form did not restore draft: got=%+v want=%+v", entry.NewTaskDraft, draft)
			}
			ready = true
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Fatal("packaged executable never reported native startup")
	}
}
