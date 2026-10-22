package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Render synthetic task/settings states with the real native renderer. These
// artifacts are layout evidence, not OS window or notification screenshots.
func TestNativeUIVisualArtifacts(t *testing.T) {
	dir := os.Getenv("SIDELET_UI_VISUAL_DIR")
	if dir == "" {
		t.Skip("set SIDELET_UI_VISUAL_DIR to render UI review artifacts")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"mac", "paper", "graphite"} {
		m, v, u := webUITester(t)
		m.UITheme, v.visual = theme, webTheme(theme)
		now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.Local)
		m.clock = func() time.Time { return now }
		_, err := v.service.SaveTask(1, "整理今天的工作", "先完成最重要的一件事", 2, true, now.Add(time.Hour).UnixMilli(), true, false, m.Tasks[0].Version)
		if err != nil {
			t.Fatal(err)
		}
		v.service.notificationAuthorization = "systemManaged"
		v.service.notificationStatus = notificationMessage("systemManaged", nil)
		for _, state := range []string{"tasks", "compact", "settings", "notification-denied", "notification-error"} {
			v.settingsOpen = state == "settings" || state == "notification-denied"
			w, h := 1120, 800
			if state == "compact" {
				w, h = 820, 600
			}
			if v.settingsOpen {
				h = 2000
			}
			if state == "notification-denied" {
				v.service.notificationAuthorization = "denied"
				v.service.notificationStatus = notificationMessage("denied", nil)
			}
			if state == "notification-error" {
				w, h = 820, 600
				v.service.notificationFailed = true
				v.service.notificationStatus = "系统提醒暂时不可用，将稍后重试。"
			}
			u.SetSize(w, h)
			u.Frame()
			f, err := os.Create(filepath.Join(dir, theme+"-"+state+".png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, u.Image())
			closeErr := f.Close()
			if err != nil {
				t.Fatal(err)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
		}
	}
}

// Opt-in artifacts capture the native card themes. This uses MyGo's
// renderer, not a desktop window capture or evidence of pointer handoff.
func TestCardVisualArtifacts(t *testing.T) {
	dir := os.Getenv("SIDELET_CARD_VISUAL_DIR")
	if dir == "" {
		t.Skip("set SIDELET_CARD_VISUAL_DIR to render comparison artifacts")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"mac", "paper", "graphite"} {
		m := newModel()
		m.UITheme = theme
		m.open(0)
		v := &views{m: m}
		u := ui.NewTester(v.webCard, 320, 390)
		u.SetSize(320, m.CardReadHeight)
		for _, mode := range []string{"read", "edit"} {
			if mode == "edit" {
				m.edit()
				u.SetSize(320, 390)
				u.Frame()
			}
			f, err := os.Create(filepath.Join(dir, "native-"+theme+"-"+mode+".png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, u.Image())
			closeErr := f.Close()
			if err != nil {
				t.Fatal(err)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
		}
	}
}
