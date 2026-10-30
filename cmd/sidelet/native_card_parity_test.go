package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCardSaveKeepsTaskOnDesktop(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m := newModel()
			m.UITheme = theme
			m.open(0)
			m.edit()
			v := &views{m: m}
			u := ui.NewTester(v.webCard, 320, 390)
			u.SetFocused(true)
			m.Draft = "修改后仍固定在桌面"
			u.Frame()
			if err := u.Click("保存"); err != nil {
				t.Fatal(err)
			}
			u.Key(0, ui.KeyEnter)
			for i := 0; i < 3; i++ {
				u.Frame()
			}
			if m.Tasks[0].Title != "修改后仍固定在桌面" || m.Tasks[0].Done || m.Tasks[0].Unpinned || !m.eligible(0) {
				t.Fatalf("saved task disappeared: %+v", m.Tasks[0])
			}
			if err := u.Click("完成"); err != nil {
				t.Fatal(err)
			}
			if !m.Tasks[0].Done || m.eligible(0) {
				t.Fatal("explicit completion no longer works")
			}
		})
	}
}

func TestCardCopiesTitleAndNoteWithoutChangingTask(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		m := newModel()
		m.UITheme = theme
		m.Tasks[0].Title, m.Tasks[0].Note = "准备评审材料", "核对设计稿\n补充说明"
		m.open(0)
		v := &views{m: m}
		u := ui.NewTester(v.webCard, 320, 250)
		before := m.Tasks[0]
		buttonBefore := visualRect(t, u, "关闭快速卡片")
		if err := u.Click("复制任务内容"); err != nil {
			t.Fatal(err)
		}
		if u.Clipboard() != "准备评审材料\n\n核对设计稿\n补充说明" || !u.HasText("已复制") {
			t.Fatal("missing copied content or feedback", theme)
		}
		if m.Tasks[0] != before || m.Opened != 0 || m.Editing || visualRect(t, u, "关闭快速卡片") != buttonBefore {
			t.Fatal("copy changed task or layout", theme)
		}
		m.open(1)
		m.Tasks[1].Title, m.Tasks[1].Note = "只有标题", ""
		u.Frame()
		if u.HasText("已复制") {
			t.Fatal("copy feedback leaked into another task")
		}
		if err := u.Click("复制任务内容"); err != nil {
			t.Fatal(err)
		}
		if u.Clipboard() != "只有标题" {
			t.Fatal("empty note added unwanted content")
		}
	}
}

func TestCardNoteSaveShortcutPreservesCompositionAndConflict(t *testing.T) {
	for _, mods := range []ui.Modifiers{ui.Super, ui.Ctrl} {
		m := newModel()
		m.UITheme = "mac"
		m.open(0)
		m.edit()
		v := &views{m: m}
		u := ui.NewTester(v.webCard, 320, 390)
		u.SetFocused(true)
		_ = u.Click("卡片备注")
		u.Compose("pinyin", 6)
		u.Key(mods, ui.KeyEnter)
		if !m.Editing {
			t.Fatal("composition submitted the draft")
		}
		u.Compose("", 0)
		m.DraftNote = "保留草稿"
		m.Tasks[0].Version++
		u.Frame()
		u.Key(mods, ui.KeyEnter)
		if !m.Editing || m.EditError == "" || m.DraftNote != "保留草稿" {
			t.Fatal("conflict discarded draft", m.EditError)
		}
		m.cancel()
		m.edit()
		m.DraftNote = "保存备注"
		u.Frame()
		_ = u.Click("卡片备注")
		u.Key(mods, ui.KeyEnter)
		if m.Editing || m.Tasks[0].Note != "保存备注" {
			t.Fatal("shortcut did not save", mods, m.EditError)
		}
	}
}

func TestCardSwitchResetsScrollAndError(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		m := newModel()
		m.UITheme = theme
		for i := range m.Tasks {
			m.Tasks[i].Note = strings.Repeat("长备注\n", 80)
		}
		m.open(0)
		v := &views{m: m}
		u := ui.NewTester(v.webCard, 320, 390)
		title := visualRect(t, u, m.Tasks[0].Title)
		u.Scroll(160, 180, 0, 500)
		scrolled := visualRect(t, u, m.Tasks[0].Title)
		if scrolled.Y >= title.Y {
			t.Fatal("fixture did not scroll")
		}
		m.EditError = "上一条任务的错误"
		m.open(1)
		u.Frame()
		next := visualRect(t, u, m.Tasks[1].Title)
		if next.Y != title.Y || m.EditError != "" {
			t.Fatal("new card retained old state", theme, next, title, m.EditError)
		}
		m.edit()
		u.Frame()
		field := visualRect(t, u, "任务标题")
		if field.Y < 40 {
			t.Fatal("editor inherited reading scroll", field)
		}
	}
}
