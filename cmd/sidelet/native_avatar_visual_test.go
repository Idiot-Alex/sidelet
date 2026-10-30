package main

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Render the actual vector through the application's renderer for visual review.
func TestAvatarVectorPreview(t *testing.T) {
	path := os.Getenv("SIDELET_AVATAR_VISUAL")
	if path == "" {
		t.Skip("set SIDELET_AVATAR_VISUAL to render the avatar")
	}
	u := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#f2f4ef"))
		ui.Row(c).Padding(24).Gap(24).AlignItems(ui.Center).Children(func() {
			ui.Image(c, sidebarAvatarVector()).Size(224, 224)
			ui.Image(c, sidebarAvatarVector()).Size(56, 56)
		})
	}, 352, 272)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, u.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestTrayAssistantMenuSharesThemeAndActions(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		m := newModel()
		m.UITheme = theme
		quit, opened, added, settings := false, false, false, false
		a := &nativeAvatar{service: testService(m), panelFocus: true,
			openMain: func() { opened = true }, quickAdd: func() { added = true }, openSettings: func() { settings = true }, panelQuit: func() { quit = true }}
		u := ui.NewTester(a.renderPanel, avatarMenuWidth, avatarMenuHeight+53)
		u.SetFocused(true)
		for _, label := range []string{"快速添加", "打开主界面", "隐藏桌面任务", "设置", "退出 Sidelet"} {
			r := visualRect(t, u, label)
			if r.Y < 0 || r.Y+r.H > avatarMenuHeight+53 || r.H < 40 {
				t.Fatalf("clipped tray action %s: %v", label, r)
			}
		}
		u.Key(0, ui.KeyEnd)
		if !u.Focused("退出 Sidelet") {
			t.Fatal("quit is not keyboard accessible")
		}
		for _, label := range []string{"快速添加", "打开主界面", "设置", "隐藏桌面任务", "显示桌面任务", "退出 Sidelet"} {
			if err := u.Click(label); err != nil {
				t.Fatal(err)
			}
		}
		if !quit || !opened || !added || !settings || m.Quiet {
			t.Fatal("menu actions do not match assistant")
		}
		if path := os.Getenv("SIDELET_TRAY_MENU_VISUAL"); path != "" && theme == "mac" {
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, u.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
