package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"sidelet/internal/quickadd"
)

type testQuickFocus struct{ restores, releases int }

func (f *testQuickFocus) restore() bool { f.restores++; return true }
func (f *testQuickFocus) release()      { f.releases++ }
func (*testQuickFocus) pid() int        { return 100 }

func quickFixture(s *TasksService) *nativeQuickAdd {
	q := &nativeQuickAdd{service: s}
	q.reset()
	q.session.Begin(s.m.now())
	return q
}

func TestNativeQuickAddParsePreviewAndSaveShareOpeningDate(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	zone := time.FixedZone("CST", 8*3600)
	ref := time.Date(2026, 12, 31, 23, 59, 0, 0, zone)
	m.clock = func() time.Time { return ref }
	q := quickFixture(s)
	q.text = "明天下午三点半 联系客户"
	u := ui.NewTester(q.render, 540, 260)
	if !u.HasText("1/1 15:30") || !u.HasText("联系客户") {
		t.Fatal("no parsed preview")
	}
	revision := q.session.Revision
	m.clock = func() time.Time { return ref.Add(2 * time.Minute) }
	q.submit(revision)
	want := time.Date(2027, 1, 1, 15, 30, 0, 0, zone).UnixMilli()
	if len(m.Tasks) != 1 || m.Tasks[0].Title != "联系客户" || m.Tasks[0].DueAt != want || !m.Tasks[0].Unpinned || m.Tasks[0].Remind || m.Tasks[0].ReminderID != 0 || p.saved.Tasks[0].DueAt != want {
		t.Fatal("preview/save date changed or default reminder enabled", m.Tasks)
	}
	if q.session.Open || q.session.Saving || q.text != "" || q.pin || !q.recognize {
		t.Fatal("success did not reset")
	}
	q.submit(revision)
	q.session.Begin(m.now())
	q.text = "新的会话"
	q.submit(revision)
	if len(m.Tasks) != 1 || q.text != "新的会话" {
		t.Fatal("old revision submitted into a new session")
	}
}

func TestNativeQuickAddRecognizeOffPinAndInvalidInput(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	s := testService(m)
	q := quickFixture(s)
	q.text, q.recognize, q.pin = "明天下午3点 普通文字", false, true
	q.submit(q.session.Revision)
	item := m.Tasks[3]
	if item.Title != "明天下午3点 普通文字" || item.DueAt != 0 || item.Unpinned || item.Remind {
		t.Fatal("recognition switch ignored", item)
	}
	for _, text := range []string{"明天25:00 错误", "明天下午3点", "两行\n任务", strings.Repeat("字", 501)} {
		t.Run(text[:min(len(text), 20)], func(t *testing.T) {
			q.session.Begin(m.now())
			q.text = text
			q.submit(q.session.Revision)
			if q.error == "" || !q.session.Open || q.session.Saving || q.text != text || len(m.Tasks) != 4 {
				t.Fatal("invalid input changed tasks or lost draft")
			}
			q.cancel()
		})
	}
}

func TestNativeQuickAddStorageFailureKeepsInputOptionsAndRetryDate(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	ref := time.Date(2026, 10, 9, 23, 59, 0, 0, time.FixedZone("CST", 8*3600))
	m.clock = func() time.Time { return ref }
	q := quickFixture(s)
	q.text, q.pin = "明天09:30 失败重试", true
	before, _ := os.ReadFile(p.path)
	p.write = func(string, []byte) error { return errors.New("模拟资料写入失败") }
	q.submit(q.session.Revision)
	after, _ := os.ReadFile(p.path)
	if !bytes.Equal(before, after) || len(m.Tasks) != 0 || q.text != "明天09:30 失败重试" || !q.pin || !q.recognize || !q.session.Open || q.session.Saving || q.error == "" || !q.focus {
		t.Fatal("write failure lost state")
	}
	p.write = atomicProfileWrite
	m.clock = func() time.Time { return ref.Add(2 * time.Minute) }
	q.submit(q.session.Revision)
	want, _ := quickadd.Parse("明天09:30 失败重试", true, ref)
	if len(m.Tasks) != 1 || m.Tasks[0].DueAt != want.DueAt || m.Tasks[0].Unpinned || q.error != "" {
		t.Fatal("retry did not use opening date")
	}
}

func TestNativeQuickAddBlurPreservesDraftAndDoesNotStealFocus(t *testing.T) {
	m := newModel()
	s := testService(m)
	q := quickFixture(s)
	q.text, q.pin, q.recognize = "失焦的草稿", true, false
	token := &testQuickFocus{}
	q.previous = token
	q.ours = func() bool { return true }
	hides := 0
	q.hide = func() { hides++; q.blur() }
	q.blur()
	if token.restores != 0 || token.releases != 1 || hides != 1 || q.session.Open || q.text != "失焦的草稿" || !q.pin || q.recognize {
		t.Fatal("blur discarded draft or restored focus")
	}
	q.session.Begin(m.now())
	token = &testQuickFocus{}
	q.previous = token
	q.cancel()
	if token.restores != 1 || token.releases != 1 || q.text != "" || q.pin || !q.recognize {
		t.Fatal("explicit cancel did not restore/reset")
	}
	q.session.Begin(m.now())
	q.text = "切到外部应用后取消"
	token = &testQuickFocus{}
	q.previous = token
	q.ours = func() bool { return false }
	q.cancel()
	if token.restores != 0 || token.releases != 1 {
		t.Fatal("cancel stole foreground")
	}
}

func TestNativeQuickAddReentryAndBlurDuringSave(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			m, s, p := profileFixture(t, t.TempDir())
			q := quickFixture(s)
			q.text = "保存中失焦"
			token := &testQuickFocus{}
			q.previous = token
			q.ours = func() bool { return true }
			p.write = func(path string, data []byte) error {
				q.submit(q.session.Revision)
				q.cancel()
				if !q.session.Open || q.text != "保存中失焦" || q.session.Begin(m.now()) {
					t.Fatal("save allowed duplicate/cancel/reopen")
				}
				q.blur()
				if fail {
					return errors.New("写入失败")
				}
				return atomicProfileWrite(path, data)
			}
			q.submit(q.session.Revision)
			if token.restores != 0 || token.releases != 1 || q.session.Open || q.session.Saving {
				t.Fatal("save result stole focus or stayed busy")
			}
			if fail && (q.text != "保存中失焦" || q.error == "" || len(m.Tasks) != 0) {
				t.Fatal("background failure lost draft")
			}
			if !fail && (q.text != "" || len(m.Tasks) != 1) {
				t.Fatal("success duplicated task or lost reset")
			}
		})
	}
}

func TestNativeQuickAddThreeThemePreviewSwitchIMEAndEscape(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m := newModel()
			m.UITheme = theme
			m.clock = func() time.Time { return time.Date(2026, 10, 9, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)) }
			q := quickFixture(testService(m))
			q.text = "明天下午3点 联系客户"
			u := ui.NewTester(q.render, 540, 260)
			u.SetFocused(true)
			if !u.HasText("10/10 15:00") || !u.HasText("联系客户") {
				t.Fatal("preview missing")
			}
			if err := u.Click("识别时间"); err != nil {
				t.Fatal(err)
			}
			u.Frame()
			if q.recognize || !u.HasText("整句保存为标题，不设置截止时间。") {
				t.Fatal("recognition checkbox inactive")
			}
			if err := u.Click("固定到桌面"); err != nil {
				t.Fatal(err)
			}
			if err := u.Click("添加"); err != nil {
				t.Fatal(err)
			}
			if len(m.Tasks) != 4 || m.Tasks[3].Title != "明天下午3点 联系客户" || m.Tasks[3].DueAt != 0 || m.Tasks[3].Unpinned || m.Tasks[3].Remind {
				t.Fatal("UI save lost option state")
			}
			q.session.Begin(m.now())
			q.focus = true
			u.Frame()
			u.Command("selectAll")
			u.Compose("中文", 2)
			u.Key(0, ui.KeyEnter)
			if len(m.Tasks) != 4 {
				t.Fatal("IME confirmation submitted a task")
			}
			u.Type("中文新任务")
			u.Key(0, ui.KeyEscape)
			if q.session.Open || q.text != "" || len(m.Tasks) != 4 {
				t.Fatal("Escape saved or retained cancelled input")
			}
		})
	}
}

func TestNativeQuickShortcutConflictCleanupAndStaleCallbacks(t *testing.T) {
	s := &nativeQuickShortcut{}
	opened, stopped := 0, 0
	var delivered func()
	register := func(fn func()) (func(), error) { delivered = fn; return func() { stopped++ }, nil }
	s.register(func(func()) (func(), error) { return nil, errors.New("occupied") }, func() { opened++ })
	if s.registered || !strings.Contains(s.error, "其他应用占用") {
		t.Fatal("missing conflict fallback")
	}
	s.register(register, func() { opened++ })
	first := delivered
	delivered()
	s.register(func(func()) (func(), error) { t.Fatal("registered twice"); return nil, nil }, func() {})
	s.close()
	s.close()
	first()
	if opened != 1 || stopped != 1 || s.registered {
		t.Fatal("stale callback or cleanup", opened, stopped)
	}
	s.register(register, func() { opened++ })
	first()
	delivered()
	s.close()
	if opened != 2 || stopped != 2 {
		t.Fatal("previous registration replayed into new owner")
	}
	m, v, u := webUITester(t)
	_ = m
	v.service.quickShortcut = &nativeQuickShortcut{error: "快捷键注册失败，可能已被其他应用占用。"}
	v.settingsOpen = true
	u.Frame()
	u.Scroll(600, 600, 0, 2400)
	if !u.HasText(v.service.quickShortcut.error) {
		t.Fatal("settings hides shortcut error")
	}
}

func TestNativeQuickAddBoundsStayInsideSmallAndNegativeDisplays(t *testing.T) {
	for _, area := range []mygo.Rectangle{{X: -1920, Y: -80, Width: 1920, Height: 1080}, {X: 20, Y: 30, Width: 320, Height: 200}, {Width: 1, Height: 1}, {Width: 0, Height: 600}} {
		bounds, ok := quickAddBounds(area)
		if area.Width == 0 {
			if ok {
				t.Fatal("invalid display accepted")
			}
			continue
		}
		if !ok || bounds.X < area.X || bounds.Y < area.Y || bounds.X+bounds.Width > area.X+area.Width || bounds.Y+bounds.Height > area.Y+area.Height || bounds.Width > 540 || bounds.Height > 260 {
			t.Fatal("quick add left work area", area, bounds)
		}
	}
}

func TestNativeQuickAddReusedWindowDoesNotBlockApplicationQuit(t *testing.T) {
	q := quickFixture(testService(newModel()))
	q.text = "明确关闭时取消"
	e := &mygo.CloseEvent{}
	q.handleClose(e, false)
	if !e.DefaultPrevented() || q.session.Open || q.text != "" {
		t.Fatal("window close did not cancel/hide")
	}
	e = &mygo.CloseEvent{}
	q.handleClose(e, true)
	if e.DefaultPrevented() {
		t.Fatal("hidden Quick Add vetoed app quit")
	}
	q.session.Begin(q.service.m.now())
	q.session.StartSave(q.session.Revision)
	e = &mygo.CloseEvent{}
	q.handleClose(e, false)
	if !e.DefaultPrevented() || !q.session.Saving {
		t.Fatal("close interrupted an in-flight save")
	}
}
