package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

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
