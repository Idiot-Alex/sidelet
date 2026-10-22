package main

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNotificationUIOnlyInterruptsPendingReminders(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, v, u := webUITester(t)
			m.UITheme, v.visual = theme, webTheme(theme)
			s := v.service
			if _, err := s.SaveTask(1, m.Tasks[0].Title, "", 0, true, time.Now().Add(time.Hour).UnixMilli(), true, false, m.Tasks[0].Version); err != nil {
				t.Fatal(err)
			}
			s.notificationAuthorization, s.notificationStatus = "systemManaged", notificationMessage("systemManaged", nil)
			u.Frame()
			if u.HasText(s.notificationStatus) || u.HasText("检查通知权限") || u.HasText("刷新提醒状态") {
				t.Fatal("normal Windows state interrupts tasks")
			}
			baseline := visualRect(t, u, "新建任务")
			s.notificationAuthorization, s.notificationStatus = "denied", notificationMessage("denied", nil)
			u.Frame()
			if !u.HasText(s.notificationStatus) || !u.HasText("打开系统通知设置") || u.HasText("检查通知权限") {
				t.Fatal("denied notification lacks actionable settings link")
			}
			if rect := visualRect(t, u, "新建任务"); rect.Y <= baseline.Y {
				t.Fatal("actionable warning overlaps task content")
			}
			_, _ = s.SetDone(1, true, m.Tasks[0].Version)
			u.Frame()
			if u.HasText(s.notificationStatus) {
				t.Fatal("completed reminder leaves task-page warning")
			}
			v.formRemind = true
			u.Frame()
			if !u.HasText(s.notificationStatus) {
				t.Fatal("new reminder draft lost authorization guidance")
			}
			v.formRemind = false
			s.notificationFailed, s.notificationStatus = true, notificationMessage("systemManaged", errors.New("failure"))
			_, _ = s.SetDone(1, false, m.Tasks[0].Version)
			u.SetSize(820, 600)
			if !u.HasText("重试提醒") {
				t.Fatal("real delivery failure lacks retry")
			}
			message, action := visualRect(t, u, s.notificationStatus), visualRect(t, u, "重试提醒")
			if message.X+message.W > action.X || action.X+action.W > 820 {
				t.Fatal("compact reminder warning overlaps action")
			}
		})
	}
}

func TestNotificationRetryClearsErrorWithoutLosingTask(t *testing.T) {
	m, s, n, backend := reminderFixture(t)
	_, err := s.SaveTask(1, "失败后重试", "", 0, true, 900000, true, false, m.Tasks[0].Version)
	if err != nil {
		t.Fatal(err)
	}
	backend.fail = true
	if runReminderJob(t, n) == nil || !s.notificationFailed || m.Tasks[0].ReminderSentAt != 0 {
		t.Fatal("failed delivery consumed task or hid error")
	}
	v := newNativeTasksView(s)
	u := ui.NewTester(v.render, 820, 600)
	u.SetFocused(true)
	backend.fail = false
	if err := u.Click("重试提醒"); err != nil {
		t.Fatal(err)
	}
	if len(n.jobs) != 1 {
		t.Fatal("retry did not schedule reconciliation")
	}
	if err := runReminderJob(t, n); err != nil {
		t.Fatal(err)
	}
	u.Frame()
	if s.notificationFailed || u.HasText("重试提醒") || m.Tasks[0].ReminderSentAt == 0 || len(backend.sent) != 1 {
		t.Fatal("successful retry did not clear error or duplicated send")
	}
}
