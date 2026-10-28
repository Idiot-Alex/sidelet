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
		_, err := v.service.SaveTask(1, "整理下周产品评审资料：核对设计稿、补充需求说明并确认所有待办负责人", "核对最新版本与评审意见。\n补齐移动端和桌面端的差异说明。\n会前发送给所有参与者。", 2, true, now.Add(time.Hour).UnixMilli(), true, false, m.Tasks[0].Version)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = v.service.SaveTask(2, "联系客户确认交付时间", "等待对方回复", 3, false, now.Add(-time.Hour).UnixMilli(), false, false, m.Tasks[1].Version)
		_, _ = v.service.SetDone(3, true, m.Tasks[2].Version)
		v.filterAll = true
		v.service.notificationAuthorization = "systemManaged"
		v.service.notificationStatus = notificationMessage("systemManaged", nil)
		for _, state := range []string{"tasks", "compact", "delete-confirmation", "settings", "notification-denied", "notification-error"} {
			v.moreID, v.deleteID = 0, 0
			if state == "delete-confirmation" {
				v.moreID, v.deleteID = 2, 2
				v.menuFocus = true
			}
			v.settingsOpen = state == "settings" || state == "notification-denied"
			w, h := 1120, 800
			if state == "compact" || state == "delete-confirmation" {
				w, h = 820, 600
			}
			if v.settingsOpen {
				h = 2000
			}
			if state == "notification-denied" {
				v.settingsTab = "system"
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

func TestTaskFormVisualArtifacts(t *testing.T) {
	dir := os.Getenv("SIDELET_FORM_VISUAL_DIR")
	if dir == "" {
		t.Skip("set SIDELET_FORM_VISUAL_DIR to render form review artifacts")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"mac", "paper", "graphite"} {
		m, v, u := webUITester(t)
		m.UITheme, v.visual = theme, webTheme(theme)
		now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.Local)
		m.clock = func() time.Time { return now }
		v.openNewTask()
		v.newTitle, v.formNote = "整理下周的计划", "先排好顺序，再开始第一件事。"
		v.formDue, v.newPriority = "2026/10/24 14:30", "重要"
		v.formRemind, v.formTemporary, v.formPin = true, true, true
		for _, state := range []string{"new", "edit", "restored"} {
			if state == "edit" {
				v.beginEdit(v.service.List().Tasks[0])
				v.formDue, v.formRemind, v.draftPriority = "2026/10/24 15:00", true, "紧急"
			} else if state == "restored" {
				v.cancelEdit()
			}
			v.optionsOpen = false
			u.SetSize(820, 700)
			u.Frame()
			summary := visualRect(t, u, "已选任务选项")
			action := "添加任务"
			if state == "edit" {
				action = "保存修改"
			}
			button := visualRect(t, u, action)
			if summary.Y+summary.H > button.Y || summary.X+summary.W > 797 {
				t.Fatal("summary overlaps the form action or overflows the compact window", summary, button)
			}
			f, err := os.Create(filepath.Join(dir, theme+"-"+state+".png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, u.Image())
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				t.Fatal(err, closeErr)
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
		for _, mode := range []string{"read", "edit", "date"} {
			if mode == "edit" {
				m.edit()
				u.SetSize(320, m.cardEditorHeight())
				u.Frame()
			}
			if mode == "date" {
				if err := u.Click("选择截止日期与时间"); err != nil {
					t.Fatal(err)
				}
				u.SetSize(320, m.cardEditorHeight())
				if err := u.Click("选择截止日期"); err != nil {
					t.Fatal(err)
				}
				u.SetSize(320, m.cardEditorHeight())
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
