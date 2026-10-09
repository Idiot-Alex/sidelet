package main

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func visualRect(t *testing.T, u *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := u.Find(label)
	if !ok {
		t.Fatal("missing control", label)
	}
	return r
}

func TestEmptyTaskPageOffersFocusedFirstTaskInEveryTheme(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, s, _ := profileFixture(t, t.TempDir())
			_, _ = s.Theme(theme)
			v := newNativeTasksView(s)
			u := ui.NewTester(v.render, 1120, 800)
			u.SetFocused(true)
			if !u.HasText("从一件小事开始") || !u.HasText("写下接下来想做的事，让它留在视野里。") {
				t.Fatal("missing empty-page guidance")
			}
			if err := u.Click("写下第一件事"); err != nil {
				t.Fatal(err)
			}
			if !u.Focused("新任务标题") || len(m.Tasks) != 0 {
				t.Fatal("first-task action did not focus editor or created task")
			}
			u.Type("第一件事")
			if err := u.Click("添加任务"); err != nil {
				t.Fatal(err)
			}
			if len(m.Tasks) != 1 || m.Tasks[0].Title != "第一件事" || u.HasText("写下第一件事") {
				t.Fatal("empty state did not reconcile")
			}
			tabs := visualRect(t, u, "待办 1")
			note := visualRect(t, u, "自动保存在本机")
			if math.Abs(float64(tabs.Y+tabs.H/2-note.Y-note.H/2)) > 1 || note.X < tabs.X+tabs.W {
				t.Fatal("save note not aligned with filter bar", tabs, note)
			}
			u.SetSize(820, 600)
			if u.HasText("自动保存在本机") {
				t.Fatal("compact filter bar kept wide save note")
			}
			if err := u.Click("已完成 0"); err != nil {
				t.Fatal(err)
			}
			if !u.HasText("还没有已完成的任务。") || u.HasText("写下第一件事") {
				t.Fatal("completed filter shows new-user guidance")
			}
		})
	}
}

func TestLongTaskStatusDoesNotOverlapEditorOrActions(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, s, _ := profileFixture(t, t.TempDir())
			_, _ = s.Theme(theme)
			title := strings.Repeat("任务标题需要完整阅读", 18)
			due := m.now().Add(10 * time.Minute).UnixMilli()
			if _, err := s.SaveTask(0, title, "这是长任务的备注", 3, true, due, false, false, 0); err != nil {
				t.Fatal(err)
			}
			v := newNativeTasksView(s)
			u := ui.NewTester(v.render, 1120, 800)
			u.SetFocused(true)
			for _, width := range []int{1120, 900, 850, 820} {
				u.SetSize(width, 800)
				button := visualRect(t, u, "编辑："+title)
				more := visualRect(t, u, "更多操作："+title)
				date := visualRect(t, u, dueText(m.Tasks[0], m.now()))
				editor := visualRect(t, u, "新任务标题")
				if button.W <= 0 || button.X+button.W > more.X+.5 || more.X+more.W >= editor.X || date.X+date.W >= editor.X {
					t.Fatalf("%d: controls overlap title=%+v more=%+v date=%+v editor=%+v", width, button, more, date, editor)
				}
				if width <= 850 && date.Y < button.Y+button.H {
					t.Fatal("compact status did not move below title", width, date, button)
				}
			}
			if err := u.Click("编辑：" + title); err != nil {
				t.Fatal(err)
			}
			u.Command("selectAll")
			u.Type("长标题编辑草稿")
			u.SetSize(1120, 800)
			if v.draft != "长标题编辑草稿" || !u.Focused("编辑任务标题") {
				t.Fatal("reflow lost editor or draft")
			}
		})
	}
}

func TestCardFooterKeepsGeometryAndThemeFeedback(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m := newModel()
			m.UITheme = theme
			m.open(0)
			v := &views{m: m}
			u := ui.NewTester(v.webCard, 320, 220)
			u.SetFocused(true)
			finish := visualRect(t, u, "完成")
			later := visualRect(t, u, "稍后")
			edit := visualRect(t, u, "编辑")
			if math.Abs(float64(finish.W-later.W)) > 1 || math.Abs(float64(later.W-edit.W)) > 1 || later.Y != finish.Y || later.H != 36 {
				t.Fatal("footer alignment", finish, later, edit)
			}
			u.Move(later.X+later.W/2, later.Y+later.H/2)
			pixel := u.Image().RGBAAt(int(later.X+later.W/2), int(later.Y+5))
			want := webTheme(theme).Soft
			if pixel.R != want.R || pixel.G != want.G || pixel.B != want.B {
				t.Fatal("secondary hover not formal accent-soft", pixel, want)
			}
			if err := u.Click("稍后"); err != nil {
				t.Fatal(err)
			}
			tomorrow := visualRect(t, u, "明天 09:00")
			back := visualRect(t, u, "返回操作")
			if tomorrow.W < 60 || back.Y < tomorrow.Y+tomorrow.H {
				t.Fatal("snooze controls clipped", tomorrow, back)
			}
			if err := u.Click("返回操作"); err != nil {
				t.Fatal(err)
			}
			if err := u.Click("编辑"); err != nil {
				t.Fatal(err)
			}
			u.SetSize(320, 390)
			save := visualRect(t, u, "保存")
			cancel := visualRect(t, u, "取消")
			if math.Abs(float64(save.W-cancel.W)) > 1 || save.Y != cancel.Y {
				t.Fatal("editing footer not aligned")
			}
		})
	}
}
