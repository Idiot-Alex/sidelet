package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *nativeAvatar) needsHint() bool {
	p := a.service.m.Preferences.Appearance
	return !a.hintDismissed && !p.AvatarHintDismissed && !p.HideSidebarAvatar && !a.service.m.desktopArrange()
}

func (a *nativeAvatar) closeHint() {
	if w := a.hint; w != nil {
		a.hint = nil
		w.Close()
	}
}

func (a *nativeAvatar) dismissHint() {
	// Set the session flag before saving, since saving refreshes desktop windows.
	a.hintDismissed = true
	a.closeHint()
	value := a.service.m.Preferences
	if value.Appearance.AvatarHintDismissed {
		return
	}
	value.Appearance.AvatarHintDismissed = true
	if _, err := a.service.SavePreferences(value); err != nil {
		log.Printf("avatar hint preference save failed: %v", err)
	}
}

func avatarHintBounds(anchor, area mygo.Rectangle) mygo.Rectangle {
	w, h := min(224, area.Width), min(136, area.Height)
	x := anchor.X - w - 10
	if x < area.X {
		x = anchor.X + anchor.Width + 10
	}
	return mygo.Rectangle{X: max(area.X, min(x, area.X+area.Width-w)), Y: max(area.Y, min(anchor.Y, area.Y+area.Height-h)), Width: w, Height: h}
}

func (a *nativeAvatar) syncHint() {
	if !a.needsHint() || a.window == nil || !a.window.IsVisible() || a.panel != nil || a.dragging {
		a.closeHint()
		return
	}
	r := a.window.Bounds()
	area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}).WorkArea
	b := avatarHintBounds(r, area)
	if a.hint == nil {
		w := newAuxiliaryWindow(mygo.WindowOptions{Title: "Sidelet 精灵提示", Parent: a.window,
			X: b.X, Y: b.Y, Width: b.Width, Height: b.Height,
			Frameless: true, DisableResize: true, AlwaysOnTop: true, SkipTaskbar: true,
			Hidden: true, Content: ui.View(a.renderHint)})
		a.hint = w
		w.OnClosed(func() {
			if a.hint == w {
				a.hint = nil
			}
		})
		w.ShowInactive()
	} else {
		a.hint.SetBounds(b)
		a.hint.Invalidate()
	}
}

func (a *nativeAvatar) renderHint(c *ui.Context) {
	t := webTheme(a.service.m.UITheme)
	c.SetTheme(t.Theme)
	c.Root().Background(t.Surface)
	enterSurface(c, ui.Column(c).Fill().Padding(14).Gap(8).Border(1, t.Border), 0).Children(func() {
		ui.Text(c, "我是你的桌面精灵").FontSize(14).FontWeight(600).TextColor(t.Text)
		ui.Text(c, "点我添加任务、打开主界面\n拖动我，可以调整位置").FontSize(12).LineHeight(1.5).TextColor(t.TextMuted)
		ui.Row(c).Justify(ui.End).Children(func() {
			b := webButton(c, "知道了", false).Height(28).Border(0, ui.Transparent).TextColor(t.Accent)
			b.Children(func() { webActionText(c, "知道了") })
			if b.Clicked() {
				a.dismissHint()
			}
		})
	})
	if c.Root().Shortcut(0, ui.KeyEscape) {
		a.dismissHint()
	}
}
