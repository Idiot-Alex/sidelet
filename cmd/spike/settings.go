//go:build windows || darwin

package main

import (
	"errors"
	"github.com/wailsapp/wails/v3/pkg/application"
	"log"
	"sidelet/internal/platform"
	"sidelet/internal/settings"
)

func (c *controller) settingsEvent() map[string]any {
	message := ""
	if c.preferences.LoadError != nil {
		message = "设置文件无法读取，当前使用默认偏好。原文件已保留，请检查 settings.json 后重启。"
	}
	return map[string]any{"appVersion": appVersion, "appBuild": appBuild, "value": c.preferences.Value, "loginStatus": platform.LoginStatus(), "loginAvailable": c.loginAvailable, "error": message}
}
func (c *controller) processSettings(m message) {
	if c.preferences == nil {
		return
	}
	var err error
	switch m.Type {
	case "settings-save":
		v := m.Settings
		// Login registration has a separate operation and cannot be forged by a preference save.
		v.Startup.Enabled = c.preferences.Value.Startup.Enabled
		err = c.preferences.SaveWithDock(v, func(visible bool) (applyError error) {
			application.InvokeSync(func() { applyError = platform.SetDockVisible(visible, c.control.NativeWindow()) })
			return
		})
	case "settings-login":
		if !c.loginAvailable {
			err = errors.New("请在默认资料目录的 macOS 版本中设置登录启动。")
			break
		}
		status := platform.LoginStatus()
		err = c.preferences.ChangeLogin(m.Enabled, status == "enabled" || status == "requiresApproval", platform.SetLogin)
	}
	event := c.settingsEvent()
	theme := c.preferences.Value.Appearance.Theme
	showDock := c.preferences.Value.Appearance.ShowDockIcon
	application.InvokeSync(func() {
		if dockError := platform.SetDockVisible(showDock, c.control.NativeWindow()); dockError != nil {
			if err == nil {
				err = dockError
			}
			if m.RequestID == "" {
				c.control.EmitEvent("spike:error", dockError.Error())
			}
		}
		platform.SetControlTheme(c.control.NativeWindow(), theme)
		c.logFocus("settings-applied")
		c.app.Event.Emit("settings:state", event)
		if m.RequestID != "" && m.Window != nil {
			message := ""
			if err != nil {
				message = "保存设置失败：" + settings.UserMessage(err)
			}
			m.Window.EmitEvent("todo:result", map[string]any{"requestId": m.RequestID, "error": message})
		}
	})
	if err == nil && m.Type != "settings-refresh" {
		log.Printf("settings committed operation=%s login=%s dock=%t", m.Type, event["loginStatus"], showDock)
	}
	if err != nil {
		log.Printf("settings operation=%s failed: %v", m.Type, err)
	}
}
