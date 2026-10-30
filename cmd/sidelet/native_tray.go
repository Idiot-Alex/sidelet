package main

import (
	_ "embed"
	"github.com/egoist/mygo"
	"runtime"
)

// Generated from the small-size mascot SVG by scripts/prepare-icons.go.
//
//go:embed design/tray-icon.png
var trayIconPNG []byte

func nativeTrayIcon() []byte { return trayIconPNG }
func (h *nativeApp) installNativeTray() error {
	h.quietItem = &mygo.MenuItem{Label: "隐藏桌面任务", Click: func(*mygo.MenuItem, *mygo.Window) {
		_, _ = h.service.SetQuiet(!h.service.m.Quiet)
	}}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开主界面", Click: func(*mygo.MenuItem, *mygo.Window) { h.show() }},
		{Label: "快速添加", Click: func(*mygo.MenuItem, *mygo.Window) { h.showQuickAdd() }},
		h.quietItem,
		{Label: "设置", Click: func(*mygo.MenuItem, *mygo.Window) {
			_ = h.service.CheckLogin()
			h.show()
			h.view.settingsOpen = true
			h.view.workLogOpen = false
			h.window.Invalidate()
		}},
		mygo.Separator(),
		{Label: "退出 Sidelet", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
	options := mygo.TrayOptions{Icon: nativeTrayIcon(), ToolTip: "Sidelet"}
	// Linux AppIndicator requires a system menu; Windows and macOS can
	// anchor our themed assistant menu directly to the status icon.
	if runtime.GOOS == "linux" {
		options.Menu = menu
	}
	tray, err := mygo.NewTray(options)
	if err != nil {
		return err
	}
	h.tray = tray
	if runtime.GOOS != "linux" {
		h.trayMenu = h.newAssistantMenu()
		h.trayMenu.panelQuit = func() { mygo.App.Quit() }
		showMenu := func() {
			r := tray.Bounds()
			if r.Width <= 0 || r.Height <= 0 {
				point := mygo.Screen.CursorScreenPoint()
				r = mygo.Rectangle{X: point.X, Y: point.Y, Width: 1, Height: 1}
			}
			h.trayMenu.popupAt(r, h.window)
		}
		tray.OnClick(showMenu)
		tray.OnRightClick(showMenu)
		onQuitCleanup(h.trayMenu.closePanel)
	}
	onQuitCleanup(func() { tray.Destroy() })
	if dockPreferenceAvailable() {
		h.service.applyDock = applyDockPreference
		return applyDockPreference(h.service.m.Preferences.Appearance.ShowDockIcon)
	}
	return nil
}
