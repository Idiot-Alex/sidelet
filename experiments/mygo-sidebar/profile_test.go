package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"sidelet/internal/reminder"
)

func profileFixture(t *testing.T, dir string) (*model, *TasksService, *profile) {
	t.Helper()
	m := newModel()
	m.UITheme, m.Side = "mac", "right"
	m.clock = func() time.Time { return time.Unix(1000, 0) }
	p, err := openProfile(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.close)
	return m, testService(m), p
}

func TestProfileRoundTripKeepsIDsOrderFieldsAndLayoutOnly(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	if len(m.Tasks) != 0 {
		t.Fatal("new profile contains synthetic tasks")
	}
	for k := 0; k < 9; k++ {
		if _, err := s.AddWithPin("中文任务", "备注\n第二行", 2, true); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = s.SaveTask(1, "带提醒", "备注", 3, true, 2000000, true, false, m.Tasks[0].Version)
	_, _ = s.SetDone(2, true, m.Tasks[1].Version)
	_, _ = s.Delete(3, m.Tasks[2].Version)
	_, _ = s.Snooze(4, "1h", m.Tasks[3].Version)
	_, _ = s.SetPinned(5, false, m.Tasks[4].Version)
	_, _ = s.Arrange(true)
	ids := m.eligibleIDs()
	if _, err := s.Reorder(ids, moveOrder(ids, 9, 1, false)); err != nil {
		t.Fatal(err)
	}
	_, _ = s.Arrange(false)
	_, _ = s.Theme("graphite")
	_, _ = s.Layout("left", .62, 56)
	want := m.profileState(p.saved.ID)
	m.WorkWidth, m.WorkHeight = 1728, 1000
	m.positionStack()
	m.open(0)
	m.edit()
	m.Draft = "尚未提交的草稿"
	m.Arranging = true
	m.startDrag(m.X, m.Y)
	m.moveDrag(m.X+200, m.Y+80)
	p.close()
	restored, service, p2 := profileFixture(t, dir)
	if !reflect.DeepEqual(want, restored.profileState(p2.saved.ID)) {
		t.Fatal("committed fields differ after reopen")
	}
	if restored.Editing || restored.Arranging || restored.Dragging || restored.Opened != -1 || restored.Draft != "" {
		t.Fatal("transient UI state restored")
	}
	if _, err := service.AddWithPin("新的第十项", "", 0, false); err != nil || len(restored.Tasks) != 10 || !restored.Tasks[2].Deleted {
		t.Fatal("deleted ID reused", err)
	}
	info, _ := os.Stat(p2.path)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("profile permissions", info.Mode())
	}
}

func TestProfileTemporaryCleanupAndUndoSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	now := time.Unix(1000, 0)
	m.clock = func() time.Time { return now }
	for range 2 {
		_, _ = s.SaveTask(0, "临时任务", "", 0, true, 0, false, true, 0)
	}
	_, _ = s.SetDone(1, true, 1)
	now = now.Add(3 * time.Second)
	_, _ = s.SetDone(2, true, 1)
	p.close()
	now = now.Add(3 * time.Second)
	m2 := newModel()
	m2.UITheme = "mac"
	m2.clock = func() time.Time { return now }
	p2, err := openProfile(dir, m2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p2.close)
	if !m2.Tasks[0].Deleted || m2.Tasks[1].Deleted || m2.UndoID != 2 {
		t.Fatal("startup cleanup lost live undo")
	}
	s2 := testService(m2)
	if _, err = s2.Undo(); err != nil || m2.Tasks[1].Done {
		t.Fatal("restarted undo failed", err)
	}
	_, _ = s2.SetDone(2, true, m2.Tasks[1].Version)
	p2.close()
	now = now.Add(6 * time.Second)
	m3 := newModel()
	m3.UITheme = "mac"
	m3.clock = func() time.Time { return now }
	p3, err := openProfile(dir, m3)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p3.close)
	if !m3.Tasks[1].Deleted || m3.UndoID != 0 {
		t.Fatal("expired temporary task restored")
	}
}

func TestProfileCorruptFutureAndInvalidDataNeverOverwritten(t *testing.T) {
	for _, kind := range []string{"truncated", "future", "unknown", "duplicate-order", "bad-layout", "duplicate-reminder"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			m, s, p := profileFixture(t, dir)
			for range 2 {
				_, _ = s.SaveTask(0, "任务", "", 0, true, 900000, true, false, 0)
			}
			v := p.saved
			p.close()
			switch kind {
			case "future":
				v.Schema = 3
			case "duplicate-order":
				v.Order = []int{1, 1}
			case "bad-layout":
				v.Side = "invalid"
			case "duplicate-reminder":
				v.Tasks[1].ReminderID = v.Tasks[0].ReminderID
			}
			data, _ := json.Marshal(v)
			if kind == "truncated" {
				data = []byte(`{"schema":`)
			}
			if kind == "unknown" {
				data = append(data[:len(data)-1], []byte(`,"newField":true}`)...)
			}
			if err := os.WriteFile(p.path, data, 0600); err != nil {
				t.Fatal(err)
			}
			before := m.clone()
			if _, err := openProfile(dir, m); err == nil {
				t.Fatal("invalid file accepted")
			}
			after, _ := os.ReadFile(p.path)
			if !bytes.Equal(data, after) || !reflect.DeepEqual(before.Tasks, m.Tasks) {
				t.Fatal("invalid file replaced or model changed")
			}
		})
	}
}

func TestProfileProcessLockAndRelease(t *testing.T) {
	if dir := os.Getenv("SIDELET_LAB_LOCK_DIR"); dir != "" {
		m := newModel()
		m.UITheme = "mac"
		p, err := openProfile(dir, m)
		if os.Getenv("SIDELET_LAB_LOCK_EXPECT") == "locked" {
			if err == nil {
				p.close()
				t.Fatal("second process acquired profile")
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		p.close()
		return
	}
	dir := t.TempDir()
	_, service, p := profileFixture(t, dir)
	run := func(expect string) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestProfileProcessLockAndRelease$")
		cmd.Env = append(os.Environ(), "SIDELET_LAB_LOCK_DIR="+dir, "SIDELET_LAB_LOCK_EXPECT="+expect)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("child %s: %v %s", expect, err, out)
		}
	}
	run("locked")
	p.close()
	if _, err := service.Add("closed write", 0); err == nil {
		t.Fatal("closed profile wrote without lock")
	}
	run("unlocked")
}

func TestProfileWriteFailureRollsBackEveryMutation(t *testing.T) {
	for _, action := range []string{"add", "update", "schedule", "delete", "done", "pin", "snooze", "undo", "reorder", "theme", "layout"} {
		t.Run(action, func(t *testing.T) {
			m, s, p := profileFixture(t, t.TempDir())
			for range 3 {
				_, _ = s.Add("原始任务", 0)
			}
			if action == "undo" {
				_, _ = s.SetDone(1, true, 1)
			}
			if action == "reorder" {
				_, _ = s.Arrange(true)
			}
			before := m.profileState(p.saved.ID)
			disk, _ := os.ReadFile(p.path)
			revision := s.revision
			p.write = func(string, []byte) error { return errors.New("simulated disk full") }
			var err error
			switch action {
			case "add":
				_, err = s.Add("不能新增", 0)
			case "update":
				_, err = s.UpdateDetails(1, "不能修改", "备注", 2, 1)
			case "schedule":
				_, err = s.SaveTask(1, "新提醒", "", 0, true, 900000, true, true, 1)
			case "delete":
				_, err = s.Delete(1, 1)
			case "done":
				_, err = s.SetDone(1, true, 1)
			case "pin":
				_, err = s.SetPinned(1, false, 1)
			case "snooze":
				_, err = s.Snooze(1, "1h", 1)
			case "undo":
				_, err = s.Undo()
			case "reorder":
				ids := m.eligibleIDs()
				_, err = s.Reorder(ids, moveOrder(ids, 3, 1, false))
			case "theme":
				_, err = s.Theme("paper")
			case "layout":
				_, err = s.Layout("left", .6, 56)
			}
			after, _ := os.ReadFile(p.path)
			if err == nil || !reflect.DeepEqual(before, m.profileState(p.saved.ID)) || !bytes.Equal(disk, after) || s.revision != revision || m.storageError == "" {
				t.Fatal("failed write committed", action, err)
			}
		})
	}
}

func TestProfileFailedEditorCardQuickAddAndDragKeepDrafts(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	_, _ = s.Add("原始标题", 0)
	_, _ = s.Add("卡片任务", 0)
	v := newNativeTasksView(s)
	u := ui.NewTester(v.render, 1120, 778)
	u.SetFocused(true)
	_ = u.Click("编辑：原始标题")
	v.draft = "不能丢失的草稿"
	p.write = func(string, []byte) error { return errors.New("simulated write error") }
	_ = u.Click("保存修改")
	if v.editID != 1 || v.draft != "不能丢失的草稿" || m.Tasks[0].Title != "原始标题" || !u.HasText("保存失败") {
		t.Fatal("failed main save lost draft")
	}
	_ = u.Click("设置")
	_ = u.Click("温暖纸色")
	if m.UITheme != "mac" || v.visual.ID != "mac" || !u.HasText("保存失败") {
		t.Fatal("failed theme changed preview")
	}
	_ = m.commit(func() error { return nil })
	u.Frame()
	if !u.HasText("保存失败") {
		t.Fatal("background status refresh hid write failure")
	}
	m.open(1)
	m.edit()
	m.Draft = "卡片草稿"
	m.DraftNote = "卡片备注"
	card := ui.NewTester((&views{m: m, service: s}).card, 320, 390)
	_ = card.Click("保存")
	if !m.Editing || m.Draft != "卡片草稿" || m.DraftNote != "卡片备注" || m.Tasks[1].Title != "卡片任务" {
		t.Fatal("failed card save lost draft")
	}
	m.cancel()
	card.Frame()
	_ = card.Click("完成")
	if m.Tasks[1].Done || m.Opened != 1 {
		t.Fatal("failed completion closed card")
	}
	q := &nativeQuickAdd{service: s, text: "快速添加草稿", pin: true}
	q.session.Begin(m.now())
	q.submit(q.session.Revision)
	if q.text != "快速添加草稿" || q.error == "" || len(m.Tasks) != 2 {
		t.Fatal("failed quick add lost draft")
	}
	m.close()
	m.WorkWidth, m.WorkHeight = 1728, 1000
	m.positionStack()
	x, y, side, offset := m.X, m.Y, m.Side, m.Offset
	m.startDrag(m.X, m.Y)
	m.moveDrag(0, m.Y+80)
	if m.endDrag() || m.Dragging || m.X != x || m.Y != y || m.Side != side || m.Offset != offset {
		t.Fatal("failed drag not restored")
	}
	p.write = atomicProfileWrite
	m.startDrag(m.X, m.Y)
	m.moveDrag(0, m.Y+80)
	if !m.endDrag() || m.Side != "left" || p.saved.Side != "left" {
		t.Fatal("drag retry not committed")
	}
}

func TestProfileAtomicReplaceFailureLeavesOldFileAndNoTemps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	if err := atomicProfileWrite(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := atomicProfileWrite(path, []byte("second")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "second" {
		t.Fatal("replacement failed")
	}
	blocked := filepath.Join(dir, "blocked")
	_ = os.Mkdir(blocked, 0700)
	_ = os.WriteFile(filepath.Join(blocked, "keep"), []byte("existing"), 0600)
	if err := atomicProfileWrite(blocked, []byte("cannot replace directory")); err == nil {
		t.Fatal("rename unexpectedly succeeded")
	}
	files, _ := filepath.Glob(filepath.Join(dir, ".profile-*.tmp"))
	if len(files) != 0 {
		t.Fatal("failed write leaked temp file")
	}
	b, _ = os.ReadFile(filepath.Join(blocked, "keep"))
	if string(b) != "existing" {
		t.Fatal("old content overwritten")
	}
}

func TestProfileReminderCrashAcceptanceRecoveryAndRestartNoDuplicate(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	_, _ = s.SaveTask(0, "错过的提醒", "", 0, true, 900000, true, false, 0)
	b := &testNotifications{state: reminder.State{Authorization: "authorized", Delivered: []string{"other-profile.1"}}}
	prefix := "sidelet-mygo.profile." + p.saved.ID + "."
	n := newNativeReminders(s, b, prefix)
	s.notices = n
	n.request()
	job := <-n.jobs
	repo := &receiptRepo{rows: job.rows, accepted: map[int64]int64{}}
	if _, err := reminder.Sync(repo, liveNotificationSystem{n}, prefix, m.now()); err != nil {
		t.Fatal(err)
	}
	if len(b.sent) != 1 || m.Tasks[0].ReminderSentAt != 0 {
		t.Fatal("crash gap fixture invalid")
	}
	id := m.Tasks[0].ReminderID
	n.close()
	p.close()
	if len(b.removed) != 0 {
		t.Fatal("persistent quit cleared OS receipt")
	}
	m2, s2, p2 := profileFixture(t, dir)
	n2 := newNativeReminders(s2, b, "sidelet-mygo.profile."+p2.saved.ID+".")
	s2.notices = n2
	if err := runReminderJob(t, n2); err != nil {
		t.Fatal(err)
	}
	if len(b.sent) != 1 || m2.Tasks[0].ReminderSentAt == 0 || m2.Tasks[0].ReminderID != id {
		t.Fatal("restart resent accepted notification")
	}
	n2.close()
	p2.close()
	b.state.Delivered = nil
	m3, s3, p3 := profileFixture(t, dir)
	n3 := newNativeReminders(s3, b, prefix)
	s3.notices = n3
	_ = runReminderJob(t, n3)
	if len(b.sent) != 1 || m3.Tasks[0].ReminderID != id {
		t.Fatal("persisted receipt resent after OS dismissal")
	}
	_, _ = s3.Update(1, "仅改标题", 0, m3.Tasks[0].Version)
	_ = runReminderJob(t, n3)
	if len(b.sent) != 1 || m3.Tasks[0].ReminderID != id {
		t.Fatal("title edit regenerated receipt")
	}
	_, _ = s3.Snooze(1, "30m", m3.Tasks[0].Version)
	if m3.Tasks[0].ReminderID <= id || m3.Tasks[0].ReminderSentAt != 0 || p3.saved.ReminderGeneration != m3.ReminderGeneration {
		t.Fatal("new generation not durable")
	}
	if slices.Contains(b.removed, "other-profile.1") {
		t.Fatal("cleanup touched another profile")
	}
}

func TestProfileReceiptWriteFailureRetriesWithoutRedelivery(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	_, _ = s.SaveTask(0, "提醒", "", 0, true, 900000, true, false, 0)
	b := &testNotifications{state: reminder.State{Authorization: "authorized"}}
	n := newNativeReminders(s, b, "profile-test.")
	s.notices = n
	b.onDeliver = func() { p.write = func(string, []byte) error { return errors.New("receipt write failure") } }
	_ = runReminderJob(t, n)
	if len(b.sent) != 1 || m.Tasks[0].ReminderSentAt != 0 || s.nextReminder != m.now().Add(time.Minute) || m.storageError == "" {
		t.Fatal("failed receipt consumed reminder")
	}
	p.write = atomicProfileWrite
	_ = runReminderJob(t, n)
	if len(b.sent) != 1 || m.Tasks[0].ReminderSentAt == 0 || p.saved.Tasks[0].ReminderSentAt == 0 {
		t.Fatal("receipt retry resent or not saved")
	}
}

func TestProfileNamespaceIsolationAndFormalDirectoryGuard(t *testing.T) {
	_, _, p1 := profileFixture(t, t.TempDir())
	_, _, p2 := profileFixture(t, t.TempDir())
	if p1.saved.ID == p2.saved.ID {
		t.Fatal("different profiles share namespace")
	}
	root, _ := os.UserConfigDir()
	if _, err := profileDirectory(filepath.Join(root, "Sidelet", "nested"), ""); err == nil {
		t.Fatal("formal folder accepted")
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "sidelet.sqlite3"), []byte("formal marker"), 0600)
	if _, err := profileDirectory(dir, ""); err == nil {
		t.Fatal("formal database folder accepted")
	}
	if got, err := profileDirectory("", t.TempDir()); err != nil || !strings.HasSuffix(got, string(filepath.Separator)+"profile") {
		t.Fatal("diagnostic folder not isolated", got, err)
	}
}
