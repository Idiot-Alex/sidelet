package main

import (
	"image/png"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestQuickContinueKeepsOptionsAndRejectsOldSubmission(t *testing.T) {
	m, s, _ := profileFixture(t, t.TempDir())
	q := quickFixture(s)
	q.text, q.pin = "第一条", true
	q.input()
	q.details.newPriority, q.details.formNote = "紧急", "第一条备注"
	q.details.formRemind, q.details.noteOpen = true, true
	due := q.details.formDue
	focus := &testQuickFocus{}
	q.previous = focus
	hidden := 0
	q.hide = func() { hidden++ }
	old := q.session.Revision
	q.submit(old)
	if q.savedTitle != "第一条" {
		t.Fatal("receipt did not identify saved task")
	}
	if !q.session.Open || q.text != "" || q.details.formNote != "" || !q.focus || !q.saved || hidden != 0 || focus.releases != 0 {
		t.Fatal("continuous add lost focus/session or kept previous text")
	}
	if !q.pin || !q.details.formRemind || q.details.newPriority != "紧急" || q.details.formDue != due || !q.details.noteOpen {
		t.Fatal("continuous add changed options or panel height")
	}
	q.text = "第二条"
	q.submit(old)
	if len(m.Tasks) != 1 || q.text != "第二条" {
		t.Fatal("stale submission saved next task")
	}
	q.submit(q.session.Revision)
	if len(m.Tasks) != 2 || m.Tasks[1].Title != "第二条" || m.Tasks[1].Note != "" || m.Tasks[1].DueAt != m.Tasks[0].DueAt {
		t.Fatal("second task incorrect", m.Tasks)
	}
	q.cancel()
	if hidden != 1 || focus.releases != 1 {
		t.Fatal("original focus token was not released on exit")
	}
}

func TestQuickContinueKeyboardAndCloseShortcut(t *testing.T) {
	m, s, _ := profileFixture(t, t.TempDir())
	q := quickFixture(s)
	u := ui.NewTester(q.render, 540, q.contentHeight())
	u.SetFocused(true)
	titleBefore := visualRect(t, u, "快速添加任务标题")
	buttonBefore := visualRect(t, u, "添加")
	u.Type("连续第一条")
	u.Key(0, ui.KeyEnter)
	u.Frame()
	if len(m.Tasks) != 1 || !q.session.Open || !u.Focused("快速添加任务标题") {
		t.Fatal("Enter did not continue with title focus")
	}
	if !u.HasText("已添加成功，可继续输入") || !u.HasText("连续第一条") {
		t.Fatal("missing visible success receipt")
	}
	if visualRect(t, u, "快速添加任务标题") != titleBefore || visualRect(t, u, "添加") != buttonBefore {
		t.Fatal("success feedback moved the input or submit button")
	}
	if path := os.Getenv("SIDELET_QUICK_CONTINUE_VISUAL"); path != "" {
		u.SetPreferences(ui.Preferences{ReduceMotion: true})
		for _, label := range []string{"固定到桌面", "到时提醒", "添加并关闭"} {
			r := visualRect(t, u, label)
			if r.W < 50 || r.H < 14 {
				t.Fatal("clipped action", label, r)
			}
		}
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, u.Image())
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	u.Type("连续第二条")
	if !q.saved || q.savedTitle != "连续第一条" || !u.HasText("连续第一条") {
		t.Fatal("typing hid the previous task receipt")
	}
	mods, _, hint := quickAddKeys(runtime.GOOS)
	if !strings.Contains(strings.Join(u.Texts(), " "), hint) {
		t.Fatal("wrong platform hint")
	}
	u.Key(mods, ui.KeyEnter)
	if len(m.Tasks) != 2 || m.Tasks[1].Title != "连续第二条" || q.session.Open {
		t.Fatal("close shortcut failed or duplicated", m.Tasks, q.session)
	}
}

func TestQuickContinueMouseAndNoteShortcut(t *testing.T) {
	m, s, _ := profileFixture(t, t.TempDir())
	q := quickFixture(s)
	q.text = "鼠标添加"
	u := ui.NewTester(q.render, 540, 410)
	u.SetFocused(true)
	if err := u.Click("添加"); err != nil {
		t.Fatal(err)
	}
	u.Frame()
	if !u.Focused("快速添加任务标题") || !q.session.Open {
		t.Fatal("mouse add did not restore title focus")
	}
	if !u.HasText("固定到桌面") || !u.HasText("到时提醒") {
		t.Fatal("options disappeared after continuous save")
	}
	u.Type("备注快捷键")
	if err := u.Click("快速添加备注"); err != nil {
		t.Fatal(err)
	}
	u.Type("多行备注")
	u.Key(0, ui.KeyEnter)
	if len(m.Tasks) != 1 {
		t.Fatal("plain Enter in note must insert a newline")
	}
	mods, _, _ := quickAddKeys(runtime.GOOS)
	u.Key(mods, ui.KeyEnter)
	if len(m.Tasks) != 2 || q.session.Open {
		t.Fatal("note shortcut did not add and close")
	}
	for _, os := range []string{"windows", "linux", "darwin"} {
		mods, enter, close := quickAddKeys(os)
		if os == "darwin" {
			if mods != ui.Super || enter != "Return ↩" || close != "⌘ + Return" {
				t.Fatal(os)
			}
		} else if mods != ui.Ctrl || enter != "Enter" || close != "Ctrl + Enter" {
			t.Fatal(os)
		}
	}
}
