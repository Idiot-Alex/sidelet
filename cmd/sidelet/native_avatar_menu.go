package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const avatarMenuWidth, avatarMenuHeight = 224, 198

func avatarPanelBounds(anchor, area mygo.Rectangle) mygo.Rectangle {
	w, h := min(avatarMenuWidth, area.Width), min(avatarMenuHeight, area.Height)
	x, y := anchor.X+anchor.Width-w, anchor.Y+anchor.Height+8
	if y+h > area.Y+area.Height {
		y = anchor.Y - h - 8
	}
	return mygo.Rectangle{X: max(area.X, min(x, area.X+area.Width-w)), Y: max(area.Y, min(y, area.Y+area.Height-h)), Width: w, Height: h}
}

func (a *nativeAvatar) closePanel() {
	if w := a.panel; w != nil {
		a.panel = nil
		w.Close()
	}
}

func (a *nativeAvatar) popup() {
	if a.panel != nil {
		a.closePanel()
		return
	}
	if a.window == nil {
		return
	}
	a.panelFocus, a.panelIndex = true, 0
	r := a.window.Bounds()
	area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}).WorkArea
	bounds := avatarPanelBounds(r, area)
	w := mygo.NewWindow(mygo.WindowOptions{
		Title: "Sidelet 助手菜单", Parent: a.window,
		X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: bounds.Height,
		Frameless: true, DisableResize: true, AlwaysOnTop: true, SkipTaskbar: true,
		Hidden: true, Content: ui.View(a.renderPanel),
	})
	a.panel = w
	w.SetTitle("Sidelet 助手菜单")
	w.OnBlur(func() {
		if a.panel == w {
			a.closePanel()
		}
	})
	w.OnClosed(func() {
		if a.panel == w {
			a.panel = nil
		}
	})
	// Finish the avatar's pointer-up dispatch before transferring keyboard focus.
	// Otherwise that dispatch can return focus to the avatar after Show.
	go mygo.RunOnMain(func() {
		if a.panel == w {
			w.Show()
			w.Focus()
		}
	})
}

func (a *nativeAvatar) renderPanel(c *ui.Context) {
	t := webTheme(a.service.m.UITheme)
	c.SetTheme(t.Theme)
	// Paint the whole surface: transparent margins turn black on Windows.
	c.Root().Background(t.Surface)
	quietLabel, quietIcon := "隐藏桌面任务", "desktop-remove"
	if a.service.m.Quiet {
		quietLabel, quietIcon = "显示桌面任务", "desktop-add"
	}
	actions := []struct {
		label, icon string
		run         func()
	}{
		{"快速添加", "plus", a.quickAdd},
		{"打开主界面", "list", a.openMain},
		{quietLabel, quietIcon, func() { _, _ = a.service.SetQuiet(!a.service.m.Quiet) }},
		{"设置", "settings", a.openSettings},
	}
	ui.Column(c).Fill().Padding(8).Border(1, t.Border).Gap(2).Role(ui.RoleMenu).Label("助手操作").Children(func() {
		for i, action := range actions {
			if i == 2 {
				ui.Box(c).Height(1).Margin(5, 4).Background(t.Border)
			}
			b := ui.ButtonBase(c.Key(action.label)).Bind(&a.panelItems[i]).Label(action.label).Role(ui.RoleMenuItem).Height(40).Shrink(0).WidthPercent(100).Padding(0, 10).Gap(10).Radius(6).Justify(ui.Start).TextColor(t.Text)
			webFocus(c, b, 6)
			if b.Hovered() || b.Pressed() {
				b.Background(t.Soft)
			}
			if a.panelFocus && i == a.panelIndex {
				b.Focus()
			}
			if b.Focused() {
				a.panelIndex = i
			}
			b.Children(func() {
				ui.Box(c).TextColor(t.TextMuted).Children(func() { webIcon(c, action.icon, 17) })
				ui.Text(c, action.label).FontSize(14)
			})
			if b.Clicked() {
				a.closePanel()
				if action.run != nil {
					action.run()
				}
			}
		}
	})
	a.panelFocus = false
	if c.Root().Shortcut(0, ui.KeyEscape) {
		a.closePanel()
	}
	index := a.panelIndex
	if c.Root().Shortcut(0, ui.KeyDown) {
		index = (index + 1) % len(actions)
	}
	if c.Root().Shortcut(0, ui.KeyUp) {
		index = (index + len(actions) - 1) % len(actions)
	}
	if c.Root().Shortcut(0, ui.KeyHome) {
		index = 0
	}
	if c.Root().Shortcut(0, ui.KeyEnd) {
		index = len(actions) - 1
	}
	if index != a.panelIndex {
		a.panelIndex = index
		a.panelItems[index].Focus()
	}
}
