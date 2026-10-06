//go:build windows || darwin

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"log"
	"sidelet/internal/platform"
	"sidelet/internal/reminder"
	"strings"
	"time"
)

func (c *controller) syncReminders(requestPermission bool) {
	if c.store == nil {
		return
	}
	if c.reminderTimer != nil {
		c.reminderTimer.Stop()
		c.reminderTimer = nil
	}
	result, err := reminder.Sync(c.store, platform.Notifications{}, c.reminderPrefix, time.Now())
	message := ""
	if err != nil {
		log.Printf("reminders sync failed: %v", err)
		message = "系统提醒暂时不可用，将稍后重试。"
		result.Next = time.Now().Add(time.Minute)
	}
	switch result.Authorization {
	case "denied":
		message = "系统通知未获允许。请在 macOS 系统设置的通知中允许 Sidelet，然后检查权限。"
	case "notDetermined":
		message = "开启系统通知后，勾选“提醒我”的任务才会发送通知。"
	case "unsupported":
		message = "当前平台暂不支持系统通知，桌面到期状态仍可使用。"
	}
	var stateChanged bool
	if result.Changed {
		state, e := c.store.Snapshot(time.Now())
		if e != nil {
			log.Print(e)
		} else {
			application.InvokeSync(func() {
				state.SelectedID = c.snapshot.SelectedID
				state.OverflowIDs = c.snapshot.OverflowIDs
				c.snapshot = state
				c.app.Event.Emit("spike:state", state.Copy())
			})
			stateChanged = true
		}
	}
	application.InvokeSync(func() {
		if c.notificationStatus == nil || c.notificationStatus["authorization"] != result.Authorization {
			log.Printf("notifications authorization=%s", result.Authorization)
		}
		if c.notificationDelivered != result.SystemDelivered {
			log.Printf("reminders system-delivered=%d", result.SystemDelivered)
			c.notificationDelivered = result.SystemDelivered
		}
		c.notificationStatus = map[string]any{"authorization": result.Authorization, "message": message}
		c.app.Event.Emit("reminder:status", c.notificationStatus)
		if requestPermission && result.Authorization == "notDetermined" {
			platform.RequestNotificationPermission()
		}
	})
	if !result.Next.IsZero() {
		delay := max(time.Millisecond, time.Until(result.Next))
		c.reminderTimer = time.AfterFunc(delay, func() { c.post(messageForReminders()) })
	}
	if stateChanged {
		log.Print("reminders submitted to system and recorded")
	}
}
func messageForReminders() message { return message{Type: "reminders-sync"} }

func (c *controller) notificationEvent(m message) {
	if m.Source == "open" && strings.HasPrefix(m.Label, c.reminderPrefix) {
		c.cancelStackDrag()
		settingsOpen := false
		c.openControl(&settingsOpen)
	} else if m.Source == "refresh" {
		c.post(messageForReminders())
	}
}
