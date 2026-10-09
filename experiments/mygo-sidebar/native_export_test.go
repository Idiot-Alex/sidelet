package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"sidelet/internal/exportdata"
)

func exportFixture(s *TasksService) (*nativeExport, <-chan struct{}) {
	finished := make(chan struct{}, 10)
	e := newNativeExport(s, nil)
	e.notify = func() {
		if !e.busy {
			finished <- struct{}{}
		}
	}
	s.export = e
	return e, finished
}

func awaitExport(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("export did not finish")
	}
}

func TestNativeExportProjectsAllCommittedStatusesAndHiddenOrder(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	for range 6 {
		_, _ = s.AddWithPin("中文，带\"引号\"", "多行\n第二行", 2, true)
	}
	_, _ = s.SaveTask(1, "=测试公式", "多行\n第二行", 3, true, 2000000, true, false, 1)
	_, _ = s.SetDone(2, true, 1)
	_, _ = s.Delete(3, 1)
	_, _ = s.Snooze(4, "1h", 1)
	_, _ = s.SetPinned(5, false, 1)
	_, _ = s.SaveTask(6, "临时任务", "备注", 0, true, 0, false, true, 1)
	_, _ = s.Arrange(true)
	ids := m.eligibleIDs()
	_, _ = s.Reorder(ids, moveOrder(ids, 6, 1, false))
	_, _ = s.Arrange(false)
	_, _ = s.Layout("left", .62, 56)
	before, _ := os.ReadFile(p.path)
	m.Draft, m.Quiet, m.Dragging, m.Offset = "未保存的编辑", true, true, .9
	m.Tasks[0].Title = "未保存的模型变化"
	snapshot := nativeExportSnapshot(m)
	if len(snapshot.Todos) != 5 || snapshot.Stacks[0].Side != "left" || snapshot.Stacks[0].Offset != .62 || snapshot.Stacks[0].Density != "relaxed" {
		t.Fatal(snapshot)
	}
	for _, item := range snapshot.Todos {
		original := p.saved.Tasks[item.ID-1]
		if item.Title != original.Title || item.Description != original.Note || item.Completed != original.Done || item.CompletedAt != original.CompletedAt || item.SnoozedUntil != original.SnoozedUntil || item.DueAt != original.DueAt || item.RemindAt != original.ReminderAt || item.Remind != original.Remind || item.Temporary != original.Temporary {
			t.Fatal("projection lost task fields", item)
		}
		if item.ID == 3 {
			t.Fatal("deleted task exported")
		}
		if original.Unpinned && (item.DisplayMode != "NONE" || item.StackID != 0) {
			t.Fatal("unpinned layout", item)
		}
		if !original.Unpinned && (item.DisplayMode != "EDGE" || item.StackID != 1) {
			t.Fatal("hidden pinned layout", item)
		}
		if item.CreatedAt != 0 || item.UpdatedAt != 0 {
			t.Fatal("fabricated timestamps")
		}
	}
	if snapshot.Todos[0].ID != 6 || snapshot.Todos[1].ID != 2 {
		t.Fatal("hidden order changed", snapshot.Todos)
	}
	for _, format := range []string{"json", "csv"} {
		data, err := exportdata.Encode(snapshot, format, "test", m.now())
		if err != nil {
			t.Fatal(err)
		}
		if format == "json" {
			var doc exportdata.Document
			if json.Unmarshal(data, &doc) != nil || !reflect.DeepEqual(doc.Todos, snapshot.Todos) || !reflect.DeepEqual(doc.Stacks, snapshot.Stacks) {
				t.Fatal("JSON lost fields")
			}
		} else {
			rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))).ReadAll()
			if err != nil || len(rows) != 6 || len(rows[0]) != 21 {
				t.Fatal("CSV row count", err, rows)
			}
			found := false
			for _, row := range rows[1:] {
				if row[0] == "1" {
					found = row[1] == "'=测试公式" && row[2] == "多行\n第二行" && row[6] != ""
				}
			}
			if !found {
				t.Fatal("CSV Chinese/multiline/formula/deadline lost", rows)
			}
		}
	}
	after, _ := os.ReadFile(p.path)
	if !bytes.Equal(before, after) || m.Draft != "未保存的编辑" || !m.Dragging || !m.Quiet {
		t.Fatal("export changed runtime or profile")
	}
}

func TestNativeExportCapturesClickSnapshotWithoutBlockingMutations(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	_, _ = s.AddWithPin("点击时标题", "备注", 2, false)
	e, finished := exportFixture(s)
	entered, release := make(chan struct{}), make(chan struct{})
	path := filepath.Join(t.TempDir(), "任务.json")
	e.dialog = func(_ *mygo.Window, format, filename string) (string, error) {
		if format != "json" || !strings.HasSuffix(filename, ".json") {
			t.Error("dialog filter/filename")
		}
		close(entered)
		<-release
		return path, nil
	}
	e.start("json", nil)
	<-entered
	_, err := s.UpdateDetails(1, "对话框期间的新标题", "新备注", 3, 1)
	if err != nil {
		t.Fatal("dialog blocked task update", err)
	}
	e.start("csv", nil) // Must not open a second dialog or replace the snapshot.
	close(release)
	awaitExport(t, finished)
	data, _ := os.ReadFile(path)
	var doc exportdata.Document
	if json.Unmarshal(data, &doc) != nil || len(doc.Todos) != 1 || doc.Todos[0].Title != "点击时标题" {
		t.Fatal("snapshot changed during dialog", string(data))
	}
	if m.Tasks[0].Title != "对话框期间的新标题" || p.saved.Tasks[0].Title != m.Tasks[0].Title || e.busy || e.failed || !strings.Contains(e.status, "已导出 1 个任务") {
		t.Fatal("result or later commit lost", e.status)
	}
}

func TestNativeExportCancelRetryAndDialogError(t *testing.T) {
	s := testService(newModel())
	e, finished := exportFixture(s)
	saves := 0
	e.save = func(string, string, []byte, string) error { saves++; return nil }
	e.dialog = func(*mygo.Window, string, string) (string, error) { return "", nil }
	e.start("csv", nil)
	awaitExport(t, finished)
	if e.busy || e.failed || saves != 0 || e.status != "已取消导出。" {
		t.Fatal("cancel wrote file", e.status)
	}
	e.dialog = func(*mygo.Window, string, string) (string, error) {
		return "", errors.New("无法打开保存对话框")
	}
	e.start("json", nil)
	awaitExport(t, finished)
	if !e.failed || e.busy || saves != 0 || !strings.Contains(e.status, "无法打开保存对话框") {
		t.Fatal(e.status)
	}
	e.dialog = func(*mygo.Window, string, string) (string, error) { return "任务.csv", nil }
	e.start("csv", nil)
	awaitExport(t, finished)
	if e.failed || saves != 1 || !strings.Contains(e.status, "已导出 3 个任务") {
		t.Fatal("retry failed", e.status)
	}
}

func TestNativeExportFailureProtectsProfileAndAllowsRetry(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	_, _ = s.Add("保留的任务", 0)
	e, finished := exportFixture(s)
	before, _ := os.ReadFile(p.path)
	e.dialog = func(*mygo.Window, string, string) (string, error) { return p.path, nil }
	e.start("json", nil)
	awaitExport(t, finished)
	after, _ := os.ReadFile(p.path)
	if !e.failed || !bytes.Equal(before, after) || !strings.Contains(e.status, "资料目录以外") {
		t.Fatal("profile protection failed", e.status)
	}
	e.dialog = func(*mygo.Window, string, string) (string, error) {
		return filepath.Join(t.TempDir(), "缺少目录", "任务.json"), nil
	}
	e.start("json", nil)
	awaitExport(t, finished)
	if !e.failed || !strings.Contains(e.status, "检查文件夹权限") || strings.Contains(e.status, "缺少目录") {
		t.Fatal("path error not sanitized", e.status)
	}
	path := filepath.Join(t.TempDir(), "成功.json")
	e.dialog = func(*mygo.Window, string, string) (string, error) { return path, nil }
	e.start("json", nil)
	awaitExport(t, finished)
	if e.failed || m.totalCount() != 1 {
		t.Fatal("failure prevented retry")
	}
}

func TestNativeExportEmptyAndInvalidFormat(t *testing.T) {
	m, s, _ := profileFixture(t, t.TempDir())
	e, finished := exportFixture(s)
	called := false
	e.dialog = func(*mygo.Window, string, string) (string, error) { called = true; return "", nil }
	e.start("pdf", nil)
	awaitExport(t, finished)
	if called || !e.failed || e.busy {
		t.Fatal("invalid format opened dialog")
	}
	data, err := exportdata.Encode(nativeExportSnapshot(m), "json", "test", m.now())
	if err != nil || !bytes.Contains(data, []byte("\"todos\": []")) {
		t.Fatal("empty tasks encoded as null", err)
	}
}

func TestNativeExportUIBusyFeedbackDraftAndReopenedView(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m := newModel()
			m.UITheme = theme
			s := testService(m)
			e, finished := exportFixture(s)
			entered, release := make(chan struct{}), make(chan struct{})
			e.dialog = func(*mygo.Window, string, string) (string, error) { close(entered); <-release; return "", nil }
			v := newNativeTasksView(s)
			v.settingsOpen, v.newTitle, v.formNote = true, "未提交标题", "未提交备注"
			v.onExport = func(f string) { e.start(f, nil) }
			u := ui.NewTester(v.render, 1120, 800)
			u.Scroll(600, 600, 0, 2400)
			if err := u.Click("导出 JSON"); err != nil {
				t.Fatal(err)
			}
			<-entered
			u.Frame()
			if !u.HasText("请选择保存位置…") {
				t.Fatal("missing progress")
			}
			_ = u.Click("返回我的任务")
			_ = u.Click("导出 CSV")
			if !v.settingsOpen || v.newTitle != "未提交标题" || v.formNote != "未提交备注" {
				t.Fatal("busy UI lost drafts")
			}
			// Destroy the view while the worker remains in a dialog.
			v = newNativeTasksView(s)
			v.settingsOpen = true
			close(release)
			awaitExport(t, finished)
			u = ui.NewTester(v.render, 1120, 800)
			u.Scroll(600, 600, 0, 2400)
			if !u.HasText("已取消导出。") || e.busy {
				t.Fatal("reopened view lost completion")
			}
			if err := u.Click("返回我的任务"); err != nil || v.settingsOpen {
				t.Fatal("return remained disabled", err)
			}
		})
	}
}
