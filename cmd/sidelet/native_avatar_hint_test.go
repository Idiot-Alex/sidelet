package main

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestAvatarHintAcknowledgementSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	a := &nativeAvatar{service: s}
	if !a.needsHint() {
		t.Fatal("first use needs guidance")
	}
	m.Preferences.Appearance.HideSidebarAvatar = true
	if a.needsHint() {
		t.Fatal("hidden avatar should not show guidance")
	}
	m.Preferences.Appearance.HideSidebarAvatar = false
	u := ui.NewTester(a.renderHint, 224, 136)
	u.SetPreferences(ui.Preferences{ReduceMotion: true})
	u.Frame()
	button := visualRect(t, u, "知道了")
	if button.Y+button.H > 136 {
		t.Fatal("hint action clipped", button)
	}
	if path := os.Getenv("SIDELET_HINT_VISUAL"); path != "" {
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
	if err := u.Click("知道了"); err != nil {
		t.Fatal(err)
	}
	if a.needsHint() || !m.Preferences.Appearance.AvatarHintDismissed {
		t.Fatal("acknowledgement not saved")
	}
	p.close()
	_, restored, _ := profileFixture(t, dir)
	if (&nativeAvatar{service: restored}).needsHint() {
		t.Fatal("guidance repeated after restart")
	}
}

func TestAvatarHintDragAndMenuDismiss(t *testing.T) {
	for _, action := range []string{"drag", "menu"} {
		t.Run(action, func(t *testing.T) {
			_, s, _ := profileFixture(t, t.TempDir())
			a := &nativeAvatar{service: s}
			if action == "menu" {
				a.popup()
			} else {
				a.pressed = true
				a.pointer(ui.InputEvent{Kind: ui.InputPointerMove}, mygo.Point{X: 6})
			}
			if a.needsHint() || !s.m.Preferences.Appearance.AvatarHintDismissed {
				t.Fatal("interaction did not dismiss hint")
			}
		})
	}
}

func TestAvatarHintStaysInsideDisplay(t *testing.T) {
	for _, area := range []mygo.Rectangle{{Width: 1920, Height: 1080}, {X: -1920, Y: 40, Width: 1920, Height: 1040}, {Width: 160, Height: 120}} {
		for _, anchor := range []mygo.Rectangle{{X: area.X, Y: area.Y, Width: 56, Height: 56}, {X: area.X + area.Width - 56, Y: area.Y + area.Height - 56, Width: 56, Height: 56}} {
			r := avatarHintBounds(anchor, area)
			if r.X < area.X || r.Y < area.Y || r.X+r.Width > area.X+area.Width || r.Y+r.Height > area.Y+area.Height {
				t.Fatal("hint escaped screen", r)
			}
		}
	}
}
