package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestGlassRenderingKeepsForegroundOpaqueAndFallsBack(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, v, u := webUITester(t)
			m.UITheme = theme
			m.Preferences.Appearance.Translucent = true
			u.SetVibrancy(true)
			base := webTheme(theme)
			if v.visual == base || v.visual.Text != base.Text || v.visual.Accent != base.Accent || v.visual.TextMuted.A != 255 {
				t.Fatal("glass changed text/action opacity or did not select the glass theme")
			}
			if alpha := u.Image().RGBAAt(10, 10).A; alpha == 0 || alpha == 255 {
				t.Fatal("main background did not expose the system material", alpha)
			}
			if base.Background.A != 255 || base.Surface.A != 255 || base.Field.A != 255 {
				t.Fatal("glass mutated shared opaque theme")
			}
			if cardInputBackingAlpha(m) <= .01 || cardInputBackingAlpha(m) >= .1 {
				t.Fatal("glass backing no longer balances hit testing and transparency")
			}
			u.SetVibrancy(false)
			if u.Image().RGBAAt(10, 10).A != 255 || v.visual != base {
				t.Fatal("unsupported compositor did not restore an opaque surface")
			}
			u.SetVibrancy(true)
			u.SetPreferences(ui.Preferences{HighContrast: true})
			if u.Image().RGBAAt(10, 10).A != 255 {
				t.Fatal("high contrast kept a translucent background")
			}
			u.SetPreferences(ui.Preferences{})
			m.reduceTransparency = true
			u.Frame()
			if u.Image().RGBAAt(10, 10).A != 255 || cardInputBackingAlpha(m) != 1 {
				t.Fatal("system transparency preference left translucent paint/backing")
			}
			m.reduceTransparency = false
			m.open(0)
			views := &views{m: m}
			card := ui.NewTester(views.webCard, 320, 220)
			card.SetVibrancy(true)
			if alpha := card.Image().RGBAAt(160, 6).A; alpha == 0 || alpha == 255 {
				t.Fatal("card background did not expose the material", alpha)
			}
			card.SetVibrancy(false)
			if card.Image().RGBAAt(160, 6).A != 255 {
				t.Fatal("card lacks opaque fallback")
			}
			m.close()
			stack := ui.NewTester(views.webStack, stackWidth, stackHeight)
			if stack.Image().RGBAAt(140, 30).A != 0 {
				t.Fatal("glass painted a background behind empty sidebar space")
			}
		})
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
	if err := u.Click("启用半透明玻璃"); err != nil {
		t.Fatal(err)
	}
	if !m.Preferences.Appearance.Translucent || !p.saved.Preferences.Appearance.Translucent || v.newTitle != "切换玻璃时保留的草稿" || !reflect.DeepEqual(tasks, m.Tasks) {
		t.Fatal("glass toggle did not persist or affected tasks/draft")
	}
	write := p.write
	p.write = func(string, []byte) error { return errors.New("glass preference disk failure") }
	if err := u.Click("启用半透明玻璃"); err != nil {
		t.Fatal(err)
	}
	if !v.failed || !m.Preferences.Appearance.Translucent || v.visual.Background.A == 255 {
		t.Fatal("failed preference write published a different appearance")
	}
	p.write = write
	if !v.flushNewTaskDraft() {
		t.Fatal(v.draftSaveError)
	}
	p.close()
	m2, s2, _ := profileFixture(t, dir)
	if !m2.Preferences.Appearance.Translucent || newNativeTasksView(s2).newTitle != "切换玻璃时保留的草稿" || !reflect.DeepEqual(tasks, m2.Tasks) {
		t.Fatal("glass setting or draft did not survive restart")
	}
}
