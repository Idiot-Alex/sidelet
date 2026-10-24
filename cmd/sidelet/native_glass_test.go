package main

import (
	"errors"
	"github.com/egoist/mygo/ui"
	"reflect"
	"testing"
)

func TestMainWindowAlwaysOpaque(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		m, v, u := webUITester(t)
		m.UITheme = theme
		// Old saved preferences must never reactivate main-window transparency.
		m.Preferences.Appearance.Translucent = true
		m.Preferences.Appearance.DesktopTranslucent = true
		u.SetVibrancy(true)
		u.Frame()
		if v.visual != webTheme(theme) || u.Image().RGBAAt(10, 10).A != 255 {
			t.Fatal("main window must remain opaque", theme)
		}
		m.open(0)
		views := &views{m: m}
		card := ui.NewTester(views.webCard, 320, 220)
		card.SetVibrancy(true)
		if alpha := card.Image().RGBAAt(160, 6).A; alpha == 0 || alpha == 255 {
			t.Fatal("desktop card lost transparency", alpha)
		}
		m.Preferences.Appearance.DesktopTranslucent = false
		card.Frame()
		if card.Image().RGBAAt(160, 6).A != 255 || cardInputBackingAlpha(m) != 1 {
			t.Fatal("desktop card did not restore opaque background")
		}
	}
}
func TestGlassPreferencePreservesDraftAndOnlyPublishesAfterCommit(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	_, _ = s.AddWithPin("保留原任务", "备注", 2, true)
	tasks := append([]task(nil), m.Tasks...)
	v := newNativeTasksView(s)
	u := ui.NewTester(v.render, 1120, 800)
	u.SetVibrancy(true)
	u.SetFocused(true)
	for _, label := range []string{"新建任务", "新任务标题"} {
		if err := u.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	u.Type("切换玻璃时保留的草稿")
	if err := u.Click("设置"); err != nil {
		t.Fatal(err)
	}
	if err := u.Click("启用悬浮任务半透明"); err != nil {
		t.Fatal(err)
	}
	if !m.Preferences.Appearance.DesktopTranslucent || !p.saved.Preferences.Appearance.DesktopTranslucent || v.newTitle != "切换玻璃时保留的草稿" || !reflect.DeepEqual(tasks, m.Tasks) {
		t.Fatal("glass toggle did not persist or affected tasks/draft")
	}
	write := p.write
	p.write = func(string, []byte) error { return errors.New("glass preference disk failure") }
	if err := u.Click("启用悬浮任务半透明"); err != nil {
		t.Fatal(err)
	}
	if !v.failed || !m.Preferences.Appearance.DesktopTranslucent || v.visual.Background.A != 255 {
		t.Fatal("failed preference write published a different appearance")
	}
	p.write = write
	if !v.flushNewTaskDraft() {
		t.Fatal(v.draftSaveError)
	}
	p.close()
	m2, s2, _ := profileFixture(t, dir)
	if !m2.Preferences.Appearance.DesktopTranslucent || newNativeTasksView(s2).newTitle != "切换玻璃时保留的草稿" || !reflect.DeepEqual(tasks, m2.Tasks) {
		t.Fatal("glass setting or draft did not survive restart")
	}
}
