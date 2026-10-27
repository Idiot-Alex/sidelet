package main

import (
	_ "embed"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed design/sidebar-avatar.png
var sidebarAvatarPNG []byte

var sidebarAvatarImage = func() *ui.Bitmap {
	b, err := ui.DecodeBitmap(sidebarAvatarPNG)
	if err != nil {
		panic(err)
	}
	return b
}()

const sidebarAvatarSize = 56

// A separate window keeps the avatar out of task drag and hit regions.
type nativeAvatar struct {
	window                           *mygo.Window
	service                          *TasksService
	openMain, quickAdd, openSettings func()
	pressed, dragging                bool
	pressPoint                       mygo.Point
	startX, startY                   int
	position                         mygo.Point
	positioned                       bool
	paintedTheme                     string
	panel                            *mygo.Window
	panelFocus                       bool
	panelIndex                       int
	panelItems                       [4]ui.Handle
}

func avatarBounds(m *model) mygo.Rectangle {
	size := min(sidebarAvatarSize, max(1, m.WorkWidth), max(1, m.WorkHeight))
	x := m.X + 6
	if m.Side == "right" {
		x = m.X + stackWidth - size - 6
	}
	y := m.Y - size - 8
	if y < m.WorkY {
		y = m.Y + m.stackHeight() + 8
		if y+size > m.WorkY+m.WorkHeight {
			y = m.Y
			if m.Side == "right" {
				x -= 64
			} else {
				x += 64
			}
		}
	}
	return mygo.Rectangle{X: max(m.WorkX, min(x, m.WorkX+m.WorkWidth-size)), Y: max(m.WorkY, min(y, m.WorkY+m.WorkHeight-size)), Width: size, Height: size}
}

func (a *nativeAvatar) sync() {
	if a == nil {
		return
	}
	m := a.service.m
	if m.Preferences.Appearance.HideSidebarAvatar || m.desktopArrange() {
		a.closePanel()
		if a.window != nil && a.window.IsVisible() {
			a.window.Hide()
		}
		return
	}
	r := a.bounds()
	created := a.window == nil
	if a.window == nil {
		a.window = newRoundedDesktopWindow(mygo.WindowOptions{Title: "Sidelet 侧栏助手", X: r.X, Y: r.Y, Width: r.Width, Height: r.Height,
			MinWidth: 1, MinHeight: 1,
			Frameless: true, Transparent: true, AlwaysOnTop: true, DisableResize: true, DisableShadow: true, Hidden: true, BackgroundColor: "#00000000", Content: ui.View(a.render)})
		a.window.OnBlur(a.cancelDrag)
	}
	placeAvatarWindow(a.window, r, created)
	if a.paintedTheme != m.UITheme {
		a.paintedTheme = m.UITheme
		a.window.Invalidate()
		if a.panel != nil {
			a.panel.Invalidate()
		}
	}
	if !a.window.IsVisible() {
		a.window.ShowInactive()
	}
}

func (a *nativeAvatar) render(c *ui.Context) {
	t := webTheme(a.service.m.UITheme)
	c.SetTheme(t.Theme)
	c.Root().Background(ui.Transparent)
	b := ui.ButtonBase(c).Label("侧栏助手菜单").Fill().Padding(1).Radius(28).Background(t.Surface).Border(1, desktopOutline(t))
	b.Cursor(ui.CursorMove).HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputKeyDown && (ev.Key == ui.KeyEnter || ev.Key == ui.KeySpace) {
			a.popup()
			return true
		}
		if a.window == nil {
			return false
		}
		if ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyEscape && a.pressed {
			a.cancelDrag()
			return true
		}
		r := a.window.Bounds()
		return a.pointer(ev, mygo.Point{X: r.X + int(ev.X), Y: r.Y + int(ev.Y)})
	})
	webFocus(c, b, 28)
	if b.Hovered() {
		b.Background(t.Soft).BorderColor(t.Accent)
	}
	b.Children(func() {
		ui.Box(c).Fill().Radius(26).Clip().Children(func() {
			ui.Image(c, sidebarAvatarImage).Fill().Fit(ui.Cover)
		})
	})
}

// Desktop surfaces need a firmer outline than dividers inside the app,
// since the background behind them can be any wallpaper or window.
func desktopOutline(t *visualTheme) ui.Color {
	if t.ID == "graphite" {
		return ui.Hex("#748177")
	}
	return ui.Hex("#9baa9f")
}

// A click opens the menu only on release. A drag crosses a five-DIP threshold
// and moves only this window, with its own persistent position.
func (a *nativeAvatar) pointer(ev ui.InputEvent, point mygo.Point) bool {
	m := a.service.m
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		if m.Editing || m.Dragging || m.desktopArrange() {
			return true
		}
		a.pressed, a.dragging = true, false
		r := a.bounds()
		a.pressPoint, a.startX, a.startY = point, r.X, r.Y
		return true
	case ui.InputPointerMove:
		if !a.pressed {
			return false
		}
		dx, dy := point.X-a.pressPoint.X, point.Y-a.pressPoint.Y
		if !a.dragging && dx*dx+dy*dy >= 25 {
			a.closePanel()
			a.dragging = true
		}
		if a.dragging {
			a.move(point)
		}
		return true
	case ui.InputPointerUp:
		if !a.pressed || ev.Button != 0 {
			return false
		}
		a.pressed = false
		if a.dragging {
			a.move(point)
			r := a.bounds()
			a.dragging = false
			value := m.Preferences
			value.Appearance.AvatarPositionSet = true
			value.Appearance.AvatarX, value.Appearance.AvatarY = r.X, r.Y
			if _, err := a.service.SavePreferences(value); err != nil {
				a.position = mygo.Point{X: a.startX, Y: a.startY}
			}
			a.refresh()
		} else if a.window != nil {
			a.popup()
		}
		return true
	}
	return false
}

func (a *nativeAvatar) refresh() {
	if a.window != nil {
		a.sync()
	}
}

func (a *nativeAvatar) bounds() mygo.Rectangle {
	m := a.service.m
	r := avatarBounds(m)
	if !a.positioned {
		a.position = mygo.Point{X: r.X, Y: r.Y}
		a.positioned = true
	}
	if !a.dragging && m.Preferences.Appearance.AvatarPositionSet {
		a.position = mygo.Point{X: m.Preferences.Appearance.AvatarX, Y: m.Preferences.Appearance.AvatarY}
	}
	r.X = max(m.WorkX, min(a.position.X, m.WorkX+m.WorkWidth-r.Width))
	r.Y = max(m.WorkY, min(a.position.Y, m.WorkY+m.WorkHeight-r.Height))
	return r
}

func (a *nativeAvatar) move(point mygo.Point) {
	a.position = mygo.Point{X: a.startX + point.X - a.pressPoint.X, Y: a.startY + point.Y - a.pressPoint.Y}
	a.refresh()
}

func (a *nativeAvatar) cancelDrag() {
	a.pressed = false
	if a.dragging {
		a.dragging = false
		a.position = mygo.Point{X: a.startX, Y: a.startY}
		a.refresh()
	}
}

func (h *nativeApp) createAvatar() *nativeAvatar {
	a := &nativeAvatar{service: h.service, quickAdd: h.showQuickAdd}
	a.openMain = func() {
		h.show()
		if h.view != nil {
			h.view.settingsOpen = false
			h.window.Invalidate()
		}
	}
	a.openSettings = func() {
		_ = h.service.CheckLogin()
		h.show()
		if h.view != nil {
			h.view.settingsOpen = true
			h.view.settingsTab = "desktop"
			h.window.Invalidate()
		}
	}
	a.sync()
	return a
}
