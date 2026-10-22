package main

import "github.com/egoist/mygo/ui"

type notificationPresentation struct {
	label, message, action string
	attention              bool
	foreground, background ui.Color
}

func (v *nativeTasksView) notificationPresentation() notificationPresentation {
	t, s := v.visual, v.service
	p := notificationPresentation{label: "读取中…", foreground: t.TextMuted, background: t.Alt}
	switch s.notificationAuthorization {
	case "authorized":
		p.label, p.action = "已允许", "检查通知权限"
		p.foreground, p.background = t.Accent, t.Soft
	case "systemManaged":
		p.label, p.action = "由系统管理", "刷新提醒状态"
		p.message = "通知显示由 Windows 管理。未收到提醒时，请检查系统通知开关、横幅和勿扰模式。"
	case "denied":
		p.label, p.action = "未允许", "打开系统通知设置"
		p.message, p.attention = notificationMessage("denied", nil), true
		p.foreground, p.background = t.Warning, t.WarningSoft
	case "notDetermined":
		p.label, p.action = "尚未开启", "开启系统通知"
		p.message, p.attention = "开启系统通知后，Sidelet 才能发送任务提醒。", true
		p.foreground, p.background = t.Warning, t.WarningSoft
	case "unsupported":
		p.label = "暂不可用"
		p.message, p.attention = notificationMessage("unsupported", nil), true
	}
	if s.notificationFailed {
		p.label, p.message, p.action, p.attention = "需要重试", s.notificationStatus, "重试提醒", true
		p.foreground, p.background = t.Danger, t.DangerSoft
	}
	return p
}

func (v *nativeTasksView) notificationAction(action string) {
	if action == "打开系统通知设置" {
		openNotificationSettings()
		return
	}
	if n := v.service.notices; n != nil {
		if action == "开启系统通知" {
			n.permission()
		} else {
			n.request()
		}
	}
}
