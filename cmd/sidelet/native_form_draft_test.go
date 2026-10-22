package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestMainNewDraftSurvivesEditExitAndFilter(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		for _, exit := range []string{"cancel", "escape", "save", "filter", "complete"} {
			t.Run(theme+"/"+exit, func(t *testing.T) {
				m, v, u := webUITester(t)
				m.UITheme, v.visual = theme, webTheme(theme)
				m.clock = func() time.Time { return time.Date(2026, 10, 23, 10, 0, 0, 0, time.Local) }
				u.SetSize(1120, 1100)
				mustClick := func(label string) {
					t.Helper()
					if err := u.Click(label); err != nil {
						t.Fatal(err)
					}
				}
				mustClick("新任务标题")
				u.Type("新建草稿")
				mustClick("展开备注")
				mustClick("任务备注")
				u.Type("新建备注\n第二行")
				v.formDue, v.newPriority = "2026/10/24 14:30", "紧急"
				v.formRemind, v.formTemporary, v.formPin, v.optionsOpen = true, true, true, true
				u.Frame()
				// Changing the list must not clear the separate new-task form.
				mustClick("全部 3")
				if v.formNote != "新建备注\n第二行" || v.formDue != "2026/10/24 14:30" {
					t.Fatal("filter cleared the new-task form")
				}
				mustClick("待办 3")
				mustClick("编辑：整理今天的工作")
				// Switching between existing tasks must not replace the held new draft.
				mustClick("编辑：确认设计稿")
				mustClick("编辑：整理今天的工作")
				v.draft, v.formNote, v.formDue = "已有任务修改", "编辑备注", ""
				v.formTemporary, v.formRemind = false, false
				u.Frame()
				switch exit {
				case "cancel":
					mustClick("取消编辑")
				case "escape":
					u.Key(0, ui.KeyEscape)
				case "save":
					mustClick("保存修改")
					if m.Tasks[0].Title != "已有任务修改" || m.Tasks[0].Note != "编辑备注" {
						t.Fatal("existing edit did not save")
					}
				case "filter":
					mustClick("已完成 0")
				case "complete":
					if _, err := v.service.SetDone(1, true, m.Tasks[0].Version); err != nil {
						t.Fatal(err)
					}
					u.Frame()
				}
				if v.editID != 0 || v.newTitle != "新建草稿" || v.formNote != "新建备注\n第二行" || v.formDue != "2026/10/24 14:30" || v.newPriority != "紧急" || !v.formRemind || !v.formTemporary || !v.formPin || !v.optionsOpen {
					t.Fatalf("new draft not restored: title=%q note=%q due=%q edit=%d", v.newTitle, v.formNote, v.formDue, v.editID)
				}
				mustClick("添加任务")
				if len(m.Tasks) != 4 {
					t.Fatalf("restored draft not added: %s", v.status)
				}
				added := m.Tasks[3]
				if added.Title != "新建草稿" || added.Note != "新建备注\n第二行" || added.Priority != 3 || added.Unpinned || !added.Remind || !added.Temporary || localDue(added.DueAt, time.Local) != "2026/10/24 14:30" {
					t.Fatalf("restored values saved incorrectly: %+v", added)
				}
			})
		}
	}
}

func TestMainRepeatedSelectionPreservesEditAndConflict(t *testing.T) {
	m, v, u := webUITester(t)
	_ = u.Click("编辑：整理今天的工作")
	v.draft, v.formNote = "未保存标题", "未保存备注"
	u.Frame()
	_ = u.Click("待办 3")
	_ = u.Click("编辑：整理今天的工作")
	_ = u.Click("编辑任务：整理今天的工作")
	if v.editID != 1 || v.draft != "未保存标题" || v.formNote != "未保存备注" || !u.Focused("编辑任务标题") {
		t.Fatal("selecting the active filter or row discarded the draft")
	}
	_, _ = v.service.UpdateDetails(1, "卡片新标题", "卡片新备注", 2, m.Tasks[0].Version)
	u.Frame()
	_ = u.Click("编辑：卡片新标题")
	_ = u.Click("任务备注")
	u.Key(ui.Super, ui.KeyEnter)
	if !v.failed || v.draft != "未保存标题" || v.formNote != "未保存备注" || m.Tasks[0].Title != "卡片新标题" {
		t.Fatal("repeated selection bypassed version protection or discarded draft")
	}
}

func TestMainPartialDeadlineSurvivesEditAndRemainsInvalid(t *testing.T) {
	m, v, u := webUITester(t)
	u.SetSize(1120, 1100)
	v.newTitle = "日期尚未填完"
	_ = u.Click("截止时间与更多选项")
	_ = u.Click("截止年份")
	u.Type("2027")
	want := v.formDue
	if want == "" || v.deadline.parts[0] != "2027" {
		t.Fatal("partial-date fixture was not entered")
	}
	_ = u.Click("编辑：整理今天的工作")
	_ = u.Click("取消编辑")
	if v.formDue != want || v.deadline.parts != [5]string{"2027", "", "", "", ""} {
		t.Fatalf("partial date changed: %q, %+v", v.formDue, v.deadline.parts)
	}
	_ = u.Click("截止时间与更多选项")
	if !u.HasText("日期待完善") {
		t.Fatalf("collapsed form hides incomplete date: open=%v due=%q", v.optionsOpen, v.formDue)
	}
	_ = u.Click("添加任务")
	if len(m.Tasks) != 3 || !v.failed || v.newTitle != "日期尚未填完" || v.formDue != want {
		t.Fatal("incomplete date saved or draft lost")
	}
}

func TestMainNoteShortcutKeepsNewlinesCompositionAndValidation(t *testing.T) {
	for _, mods := range []ui.Modifiers{ui.Super, ui.Ctrl} {
		t.Run(fmt.Sprint(mods), func(t *testing.T) {
			m, v, u := webUITester(t)
			_ = u.Click("新任务标题")
			u.Type("快捷添加")
			if err := u.Click("展开备注"); err != nil {
				t.Fatal(err)
			}
			_ = u.Click("任务备注")
			u.Type("第一行")
			u.Key(0, ui.KeyEnter)
			u.Type("第二行")
			if v.formNote != "第一行\n第二行" || len(m.Tasks) != 3 {
				t.Fatal("plain Enter did not keep a newline")
			}
			u.Compose("pinyin", 6)
			u.Key(mods, ui.KeyEnter)
			if len(m.Tasks) != 3 {
				t.Fatal("composition submitted a task")
			}
			u.Compose("", 0)
			u.Key(mods, ui.KeyEnter)
			if len(m.Tasks) != 4 || m.Tasks[3].Note != "第一行\n第二行" || v.formNote != "" {
				t.Fatalf("shortcut did not add exactly once: count=%d note=%q", len(m.Tasks), v.formNote)
			}
			_ = u.Click("编辑：快捷添加")
			_ = u.Click("任务备注")
			u.Command("selectAll")
			u.Type("修改备注")
			v.formDue = "2027/02/30 12:00"
			u.Frame()
			u.Key(mods, ui.KeyEnter)
			if v.editID != 4 || !v.failed || v.formNote != "修改备注" || m.Tasks[3].Note != "第一行\n第二行" {
				t.Fatal("shortcut bypassed validation or lost draft")
			}
			v.formDue = ""
			u.Frame()
			u.Key(mods, ui.KeyEnter)
			if v.editID != 0 || m.Tasks[3].Note != "修改备注" || m.Tasks[3].Version != 2 {
				t.Fatalf("shortcut failed to save the edit exactly once: edit=%d note=%q version=%d draft=%q status=%q focus=%v", v.editID, m.Tasks[3].Note, m.Tasks[3].Version, v.formNote, v.status, u.Focused("任务备注"))
			}
		})
	}
}
