package main

import (
	"github.com/egoist/mygo/ui"
	"time"
)

func (v *nativeTasksView) resetScheduleDraft() {
	v.formDue, v.originalDueText = "", ""
	v.originalDue = 0
	v.formRemind, v.formTemporary, v.datePickerOpen = false, false, false
}
func (v *nativeTasksView) webDeadline(c *ui.Context) {
	t := v.visual
	ui.Row(c).Key("deadline-input").Height(36).Border(1, t.Border).Radius(t.Theme.Radius).Background(t.Field).AlignItems(ui.Center).Children(func() {
		field := ui.TextInput(c, &v.formDue).Label("任务截止时间").Placeholder("年/月/日 --:--").Grow(1).MinWidth(0).Height(34).Padding(8, 9).Border(0, ui.Transparent).Background(ui.Transparent)
		if field.Changed() && v.formDue == "" {
			v.formRemind = false
		}
		b := webIconButton(c, "选择截止日期与时间", "clock", 14).Size(26, 28).Shrink(0)
		if b.Clicked() {
			if !v.datePickerOpen {
				v.pickerDate = v.service.m.now().Add(time.Hour).Truncate(time.Minute)
				if at, err := parseDue(v.formDue, v.originalDue, v.originalDueText, v.service.m.now().Location()); err == nil && at > 0 {
					v.pickerDate = time.UnixMilli(at).In(v.service.m.now().Location())
				}
			}
			v.datePickerOpen = !v.datePickerOpen
		}
		ui.PopoverBase(c, b, &v.datePickerOpen, func(panel *ui.Element) {
			// Leave room below the nested calendar on short laptop windows.
			panel.AttachTo(b, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 8).Width(280).Padding(14).Gap(12).Border(1, t.Border).Radius(t.Radius).Background(t.Surface)
			ui.Text(c, "截止日期与时间").FontSize(13).FontWeight(550)
			ui.Box(c).Key("calendar-field").Children(func() { ui.DateInput(c, &v.pickerDate).Label("选择截止日期").Height(36).Background(t.Field) })
			ui.Box(c).Key("time-field").Children(func() { ui.TimeInput(c, &v.pickerDate).Label("选择截止时间").Height(36).Background(t.Field) })
			ui.Row(c).Gap(8).Children(func() {
				b := webButton(c, "清除截止时间", false).Grow(1)
				b.Children(func() { ui.Text(c, "清除") })
				if b.Clicked() {
					v.formDue = ""
					v.formRemind = false
					v.datePickerOpen = false
				}
				b = webButton(c, "确定截止时间", true).Grow(1)
				b.Children(func() { ui.Text(c, "确定") })
				if b.Clicked() {
					v.formDue = v.pickerDate.Format("2006-01-02T15:04")
					v.datePickerOpen = false
				}
			})
		})
	})
}
func (v *nativeTasksView) webNotificationStatus(c *ui.Context) {
	message := v.service.notificationStatus
	if message == "" {
		return
	}
	visible := v.formRemind
	for _, t := range v.service.m.Tasks {
		if !t.Deleted && t.Remind {
			visible = true
			break
		}
	}
	if !visible {
		return
	}
	t := v.visual
	ui.Row(c).Gap(12).Padding(12, 16).Margin(0, 0, 18).Radius(t.Theme.Radius).Background(t.Soft).TextColor(t.Accent).Children(func() {
		ui.Text(c, message).FontSize(12).Grow(1).LineHeight(1.5)
		label := "检查通知权限"
		if v.service.notificationAuthorization == "notDetermined" {
			label = "开启系统通知"
		}
		b := webButton(c, label, false).Border(0, ui.Transparent).Background(ui.Transparent).Padding(4)
		b.Children(func() { ui.Text(c, label) })
		if b.Clicked() && v.service.notices != nil {
			if v.service.notificationAuthorization == "notDetermined" {
				v.service.notices.permission()
			} else {
				v.service.notices.request()
			}
		}
	})
}
