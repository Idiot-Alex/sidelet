package main

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func chooseWindowAction(t *testing.T, u *ui.Tester, label string) {
	t.Helper()
	if err := u.Click("更多窗口操作"); err != nil {
		t.Fatal(err)
	}
	if err := u.ChooseMenuItem(label); err != nil {
		t.Fatal(err)
	}
}

func TestMainWindowMenuPreservesDraftAndExposesModeRecovery(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, v, u := webUITester(t)
			m.UITheme, v.visual = theme, webTheme(theme)
			u.SetSize(820, 600)
			quick, hidden := 0, 0
			v.onQuickAdd = func() { quick++ }
			v.onHide = func() { hidden++ }
			for _, label := range []string{"整理桌面", "隐藏标签", "隐藏主窗口"} {
				if _, found := u.Find(label); found {
					t.Fatal("low-frequency action still occupies header", label)
				}
			}
			if err := u.Click("快速添加"); err != nil || quick != 1 {
				t.Fatal("quick add entry stopped working", err)
			}
			if err := u.Click("编辑：整理今天的工作"); err != nil {
				t.Fatal(err)
			}
			u.Command("selectAll")
			u.Type("保持编辑草稿")
			if err := u.Click("更多窗口操作"); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(u.Menu(), []string{"整理桌面", "隐藏标签", "-", "隐藏窗口"}) {
				t.Fatal("menu contents", u.Menu())
			}
			u.CloseMenu()
			u.Key(0, ui.KeyDown)
			if len(u.Menu()) != 4 {
				t.Fatal("focused menu cannot reopen from keyboard")
			}
			u.CloseMenu()
			if v.editID != 1 || v.draft != "保持编辑草稿" || hidden != 0 {
				t.Fatal("dismissing menu changed draft or hid window")
			}
			chooseWindowAction(t, u, "隐藏标签")
			if !m.Quiet || !u.HasText("显示标签") || v.draft != "保持编辑草稿" {
				t.Fatal("hidden-label mode lost draft or recovery action")
			}
			_ = u.Click("更多窗口操作")
			if err := u.ChooseMenuItem("整理桌面"); err == nil || m.Arranging {
				t.Fatal("arrange enabled while labels are hidden")
			}
			u.CloseMenu()
			if err := u.Click("显示标签"); err != nil || m.Quiet {
				t.Fatal("cannot restore labels from header", err)
			}
			chooseWindowAction(t, u, "整理桌面")
			if !m.Arranging || !u.HasText("完成整理") {
				t.Fatal("arrange mode has no visible exit")
			}
			if err := u.Click("完成整理"); err != nil || m.Arranging {
				t.Fatal("cannot exit arrange from header", err)
			}
			chooseWindowAction(t, u, "隐藏窗口")
			if hidden != 1 || v.editID != 1 || v.draft != "保持编辑草稿" || len(m.Tasks) != 3 {
				t.Fatal("menu action affected unrelated editor state")
			}
		})
	}
}

func TestMainNoteDisclosureKeepsContentAndNewFormState(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, v, u := webUITester(t)
			m.UITheme, v.visual = theme, webTheme(theme)
			u.SetSize(820, 700)
			click := func(label string) {
				t.Helper()
				if err := u.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			if _, found := u.Find("任务备注"); found {
				t.Fatal("new-task note must start collapsed")
			}
			collapsed := visualRect(t, u, "添加任务")
			click("新任务标题")
			u.Type("草稿任务")
			click("展开备注")
			if !u.Focused("任务备注") || visualRect(t, u, "添加任务").Y < collapsed.Y+70 {
				t.Fatal("expansion did not reveal and focus the note")
			}
			u.Type("需要保留的备注")
			click("收起备注")
			if v.formNote != "需要保留的备注" || !u.HasText("备注 · 已填写") {
				t.Fatal("collapsed note became hidden data or lost its value")
			}
			click("编辑：整理今天的工作")
			if _, found := u.Find("任务备注"); !found {
				t.Fatal("existing task editor did not show the note")
			}
			click("取消编辑")
			if v.noteOpen || v.formNote != "需要保留的备注" || v.newTitle != "草稿任务" {
				t.Fatal("returning from edit lost disclosure state or draft")
			}
			click("展开备注")
			u.Type("，继续补充")
			click("收起备注")
			want := v.formNote
			click("添加任务")
			if len(m.Tasks) != 4 || m.Tasks[3].Note != want || v.noteOpen || v.formNote != "" {
				t.Fatal("collapsed note did not save or form failed to reset")
			}
		})
	}
}
