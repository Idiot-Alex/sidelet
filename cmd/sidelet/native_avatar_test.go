package main

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestAvatarVectorLoadsOnlyWhenRenderedAndReusesAsset(t *testing.T) {
	m := newModel()
	m.Preferences.Appearance.HideSidebarAvatar = true
	a := &nativeAvatar{service: testService(m)}
	a.sync()
	if a.image != nil || a.window != nil {
		t.Fatal("hidden avatar allocated its vector or window")
	}
	m.Preferences.Appearance.HideSidebarAvatar = false
	ui.NewTester(a.render, sidebarAvatarSize, sidebarAvatarSize)
	if a.image == nil {
		t.Fatal("visible avatar did not load its image")
	}
	w, h := a.image.Size()
	if w != 128 || h != 128 {
		t.Fatalf("unexpected runtime image size: %gx%g", w, h)
	}
	first := a.image
	ui.NewTester(a.render, sidebarAvatarSize, sidebarAvatarSize)
	if a.image != first {
		t.Fatal("avatar parsed again on a later render")
	}
}

func TestAvatarDragIndependentCancelAndPersistence(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	s.changed = func(string) {}
	m.WorkX, m.WorkY, m.WorkWidth, m.WorkHeight = 0, 0, 1600, 1000
	m.positionStack()
	a := &nativeAvatar{service: s}
	initial := a.bounds()
	start := mygo.Point{X: initial.X + 28, Y: initial.Y + 28}
	x, y, side, offset := m.X, m.Y, m.Side, m.Offset
	unchanged := func() {
		t.Helper()
		if m.X != x || m.Y != y || m.Side != side || m.Offset != offset || m.Dragging {
			t.Fatal("avatar changed sidebar layout")
		}
	}
	a.pointer(ui.InputEvent{Kind: ui.InputPointerDown}, start)
	a.pointer(ui.InputEvent{Kind: ui.InputPointerMove}, mygo.Point{X: start.X + 2, Y: start.Y + 2})
	a.pointer(ui.InputEvent{Kind: ui.InputPointerUp}, mygo.Point{X: start.X + 2, Y: start.Y + 2})
	if m.Preferences.Appearance.AvatarPositionSet {
		t.Fatal("click jitter saved a position")
	}
	unchanged()
	m.Quiet = true
	end := mygo.Point{X: 100, Y: 600}
	a.pointer(ui.InputEvent{Kind: ui.InputPointerDown}, start)
	a.pointer(ui.InputEvent{Kind: ui.InputPointerMove}, end)
	unchanged()
	if !a.dragging || a.bounds() == initial {
		t.Fatal("avatar did not move")
	}
	a.pointer(ui.InputEvent{Kind: ui.InputPointerUp}, end)
	unchanged()
	want := a.bounds()
	if !m.Preferences.Appearance.AvatarPositionSet {
		t.Fatal("position not saved")
	}
	a.pointer(ui.InputEvent{Kind: ui.InputPointerDown}, end)
	a.pointer(ui.InputEvent{Kind: ui.InputPointerMove}, mygo.Point{X: -500, Y: 2000})
	r := a.bounds()
	if r.X != 0 || r.Y+r.Height != 1000 {
		t.Fatal("avatar escaped screen")
	}
	a.cancelDrag()
	if a.bounds() != want {
		t.Fatal("cancel did not restore avatar")
	}
	unchanged()
	p.close()
	m2, s2, _ := profileFixture(t, dir)
	m2.WorkX, m2.WorkY, m2.WorkWidth, m2.WorkHeight = 0, 0, 1600, 1000
	b := &nativeAvatar{service: s2}
	if b.bounds() != want {
		t.Fatal("avatar position did not survive restart")
	}
	m2.X, m2.Y = 300, 200
	if b.bounds() != want {
		t.Fatal("sidebar movement moved avatar")
	}
}
func TestAvatarMenuRoutesActionsAndRestoresHiddenTasks(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	opened, quick, settings := 0, 0, 0
	a := &nativeAvatar{service: testService(m), openMain: func() { opened++ }, quickAdd: func() { quick++ }, openSettings: func() { settings++ }}
	u := ui.NewTester(a.renderPanel, avatarMenuWidth, avatarMenuHeight)
	choose := func(label string) {
		t.Helper()
		if err := u.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	choose("打开主界面")
	choose("快速添加")
	choose("设置")
	if opened != 1 || quick != 1 || settings != 1 {
		t.Fatal("avatar menu failed to route actions")
	}
	choose("隐藏桌面任务")
	if !m.Quiet {
		t.Fatal("tasks were not hidden")
	}
	choose("显示桌面任务")
	if m.Quiet {
		t.Fatal("avatar cannot restore hidden tasks")
	}
}

func TestAvatarStaysWithinWorkAreaOnEitherSide(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	for _, side := range []string{"left", "right"} {
		m.Side = side
		for _, height := range []int{40, 150, 1080} {
			m.WorkX, m.WorkY, m.WorkWidth, m.WorkHeight = -1920, 40, 1920, height
			for _, y := range []int{40, 150, 500, 1050} {
				m.Y = y
				r := avatarBounds(m)
				if r.X < m.WorkX || r.Y < m.WorkY || r.X+r.Width > m.WorkX+m.WorkWidth || r.Y+r.Height > m.WorkY+m.WorkHeight {
					t.Fatalf("avatar escaped work area: %+v", r)
				}
			}
		}
	}
}

func TestAvatarPanelKeyboardAndThemeLayout(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m := newModel()
			m.UITheme = theme
			opened := 0
			a := &nativeAvatar{service: testService(m), panelFocus: true, openMain: func() { opened++ }}
			u := ui.NewTester(a.renderPanel, avatarMenuWidth, avatarMenuHeight)
			u.SetFocused(true)
			if !u.Focused("快速添加") {
				t.Fatal("first action not focused")
			}
			u.Key(0, ui.KeyDown)
			u.Frame()
			if !u.Focused("打开主界面") {
				t.Fatal("arrow navigation failed")
			}
			u.Key(0, ui.KeyEnter)
			if opened != 1 {
				t.Fatal("keyboard action did not run")
			}
			u.Key(0, ui.KeyEnd)
			u.Frame()
			if !u.Focused("设置") {
				t.Fatal("last action unreachable")
			}
			for _, label := range []string{"快速添加", "打开主界面", "隐藏桌面任务", "设置"} {
				r := visualRect(t, u, label)
				if r.Y < 0 || r.Y+r.H > avatarMenuHeight || r.H < 40 {
					t.Fatalf("clipped menu row %s: %+v", label, r)
				}
			}
		})
	}
}

func TestAvatarPanelStaysInsideDisplay(t *testing.T) {
	for _, area := range []mygo.Rectangle{{Width: 1920, Height: 1032}, {X: -1920, Y: 40, Width: 1920, Height: 1040}, {Width: 160, Height: 120}} {
		for _, anchor := range []mygo.Rectangle{{X: area.X, Y: area.Y, Width: 56, Height: 56}, {X: area.X + area.Width - 56, Y: area.Y + area.Height - 56, Width: 56, Height: 56}} {
			r := avatarPanelBounds(anchor, area)
			if r.X < area.X || r.Y < area.Y || r.X+r.Width > area.X+area.Width || r.Y+r.Height > area.Y+area.Height {
				t.Fatalf("panel escaped work area: %+v", r)
			}
			if area.Height > 300 && anchor.Y > area.Y+area.Height/2 && r.Y >= anchor.Y {
				t.Fatal("bottom-edge panel did not open above avatar")
			}
		}
	}
}

func TestAvatarPreferenceSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	v := newNativeTasksView(s)
	v.settingsOpen, v.settingsTab = true, "desktop"
	u := ui.NewTester(v.render, 860, 800)
	if err := u.Click("显示侧栏动漫头像"); err != nil {
		t.Fatal(err)
	}
	if !m.Preferences.Appearance.HideSidebarAvatar {
		t.Fatal("avatar visibility was not saved")
	}
	p.close()
	m2, _, _ := profileFixture(t, dir)
	if !m2.Preferences.Appearance.HideSidebarAvatar {
		t.Fatal("hidden avatar was reset after restart")
	}
}
