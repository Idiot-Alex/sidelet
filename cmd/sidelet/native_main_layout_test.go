package main

import (
	"errors"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestMainEditorDisclosureKeepsDraftAcrossCollapseEditAndRestart(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			dir := t.TempDir()
			_, s, p := profileFixture(t, dir)
			_, _ = s.Theme(theme)
			_, _ = s.AddWithPin("已有任务", "原备注", 0, false)
			v := newNativeTasksView(s)
			u := ui.NewTester(v.render, 1120, 800)
			u.SetFocused(true)
			click := func(label string) {
				t.Helper()
				if err := u.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			if _, found := u.Find("新任务标题"); found {
				t.Fatal("idle list still renders a permanent editor")
			}
			entry := visualRect(t, u, "新建任务")
			if entry.W < 900 {
				t.Fatal("collapsed editor still reserves an empty column", entry)
			}
			click("新建任务")
			if !u.Focused("新任务标题") {
				t.Fatal("entry did not focus the new editor")
			}
			u.Type("待续草稿")
			click("展开备注")
			u.Type("第一行\n第二行")
			click("收起新建任务")
			if v.newFormOpen || !u.Focused("新建任务") || p.saved.NewTaskDraft == nil || p.saved.NewTaskDraft.Note != "第一行\n第二行" {
				t.Fatal("collapse lost focus or failed to persist the complete draft")
			}
			if !u.HasText("继续新建草稿 · 待续草稿") {
				t.Fatal("collapsed draft has no visible recovery entry")
			}
			click("编辑：已有任务")
			u.Command("selectAll")
			u.Type("不要保存的修改")
			click("取消编辑")
			if v.newFormOpen || v.newTitle != "待续草稿" || v.formNote != "第一行\n第二行" || s.m.Tasks[0].Title != "已有任务" {
				t.Fatal("cancel did not restore the previously collapsed draft")
			}
			click("新建任务")
			u.Command("selectAll")
			u.Compose("zhongwen", 8)
			u.SetSize(820, 600)
			if _, active := u.TextCaret(); !active || !u.Focused("新任务标题") {
				t.Fatal("editor reflow lost composition or focus")
			}
			u.Type("跨窗口草稿")
			click("收起新建任务")
			p.close()
			m2, s2, _ := profileFixture(t, dir)
			v2 := newNativeTasksView(s2)
			u2 := ui.NewTester(v2.render, 1120, 800)
			if _, found := u2.Find("新任务标题"); !found || v2.newTitle != "跨窗口草稿" || v2.formNote != "第一行\n第二行" {
				t.Fatal("restart did not reveal the complete saved draft")
			}
			if err := u2.Click("添加任务"); err != nil {
				t.Fatal(err)
			}
			if len(m2.Tasks) != 2 || m2.Tasks[1].Title != "跨窗口草稿" || m2.Tasks[1].Note != "第一行\n第二行" || v2.newFormOpen {
				t.Fatal("submission did not save once and return to the list")
			}
		})
	}
}

func TestMainEditorCollapseFailureLeavesDraftAvailable(t *testing.T) {
	_, s, p := profileFixture(t, t.TempDir())
	v := newNativeTasksView(s)
	u := ui.NewTester(v.render, 820, 600)
	_ = u.Click("新建任务")
	u.Type("磁盘失败也要保留")
	write := p.writeDraft
	p.writeDraft = func([]byte) error { return errors.New("collapse disk failure fixture") }
	if err := u.Click("收起新建任务"); err != nil {
		t.Fatal(err)
	}
	if !v.newFormOpen || v.newTitle != "磁盘失败也要保留" || v.draftSaveError == "" {
		t.Fatal("failed collapse hid or cleared the unsaved draft")
	}
	p.writeDraft = write
	if err := u.Click("收起新建任务"); err != nil {
		t.Fatal(err)
	}
	if v.newFormOpen || p.saved.NewTaskDraft == nil || p.saved.NewTaskDraft.Title != "磁盘失败也要保留" {
		t.Fatal("collapse could not recover after storage became writable")
	}
}

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
			chooseWindowAction(t, u, "快速添加")
			if quick != 1 {
				t.Fatal("quick add entry stopped working")
			}
			if err := u.Click("编辑：整理今天的工作"); err != nil {
				t.Fatal(err)
			}
			u.Command("selectAll")
			u.Type("保持编辑草稿")
			if err := u.Click("更多窗口操作"); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(u.Menu(), []string{"快速添加", "-", "整理桌面", "隐藏标签", "-", "隐藏窗口"}) {
				t.Fatal("menu contents", u.Menu())
			}
			u.CloseMenu()
			u.Key(0, ui.KeyDown)
			if len(u.Menu()) != 6 {
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
			click("新建任务")
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
