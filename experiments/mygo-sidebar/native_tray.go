package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/egoist/mygo"
)

// The same 32px mark used by the formal application's trayIcon.
func nativeTrayIcon() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 3; y < 29; y++ {
		for x := 3; x < 29; x++ {
			img.Set(x, y, color.NRGBA{R: 48, G: 57, B: 52, A: 255})
		}
	}
	for x := 8; x < 24; x++ {
		y := 22 - (x - 12)
		if x < 13 {
			y = 16 + (x - 8)
		}
		for d := 0; d < 3; d++ {
			img.Set(x, y+d, color.NRGBA{R: 247, G: 246, B: 243, A: 255})
		}
	}
	var buffer bytes.Buffer
	_ = png.Encode(&buffer, img)
	return buffer.Bytes()
}

func (h *hybridApp) installNativeTray() error {
	h.quietItem = &mygo.MenuItem{Label: "安静模式", Click: func(*mygo.MenuItem, *mygo.Window) {
		_, _ = h.service.SetQuiet(!h.service.m.Quiet)
	}}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开任务窗口", Click: func(*mygo.MenuItem, *mygo.Window) { h.show() }},
		{Label: "快速添加", Click: func(*mygo.MenuItem, *mygo.Window) { h.showQuickAdd() }},
		h.quietItem,
		{Label: "设置", Click: func(*mygo.MenuItem, *mygo.Window) {
			_ = h.service.CheckLogin()
			h.show()
			h.view.settingsOpen = true
			h.window.Invalidate()
		}},
		mygo.Separator(),
		{Label: "退出 Sidelet MyGo Lab", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
	tray, err := mygo.NewTray(mygo.TrayOptions{Icon: nativeTrayIcon(), ToolTip: "Sidelet MyGo Lab", Menu: menu})
	if err != nil {
		return err
	}
	h.tray = tray
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { tray.Destroy() })
	if dockPreferenceAvailable() {
		h.service.applyDock = applyDockPreference
		return applyDockPreference(h.service.m.Preferences.Appearance.ShowDockIcon)
	}
	return nil
}
