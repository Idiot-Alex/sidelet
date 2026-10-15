package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"time"
)

func (v *nativeTasksView) resetScheduleDraft() {
	v.formDue, v.originalDueText = "", ""
	v.originalDue = 0
	v.formRemind, v.formTemporary, v.datePickerOpen = false, false, false
	v.pickerCalendarOpen = false
	v.pickerError = ""
	v.deadline = deadlineSegments{}
}
func (v *nativeTasksView) webDeadline(c *ui.Context) {
	t := v.visual
	field := ui.Row(c).Key("deadline-input").Label("任务截止时间").Role(ui.RoleGroup).Height(36).Padding(0, 5).Border(1, t.Border).Radius(t.Theme.Radius).Background(t.Field).AlignItems(ui.Center)
	field.Children(func() {
		v.webDeadlineSegments(c, field)
		// WebKit has no clock glyph on macOS. Keep the remaining field
		// space as an accessible calendar trigger, also reached by Alt+Down.
		b := ui.ButtonBase(c).Label("选择截止日期与时间").Grow(1).MinWidth(12).Height(34).FocusRing(true)
		if b.Clicked() || v.deadline.open {
			v.deadline.open = false
			if !v.datePickerOpen {
				v.pickerCalendarOpen = false
				v.pickerDate = v.service.m.now().Add(time.Hour).Truncate(time.Minute)
				if at, err := parseDue(v.formDue, v.originalDue, v.originalDueText, v.service.m.now().Location()); err == nil && at > 0 {
					v.pickerDate = time.UnixMilli(at).In(v.service.m.now().Location())
				}
				v.pickerHour, v.pickerMinute = fmt.Sprintf("%02d", v.pickerDate.Hour()), fmt.Sprintf("%02d", v.pickerDate.Minute())
				v.pickerError = ""
			}
			v.datePickerOpen = !v.datePickerOpen
		}
		ui.PopoverBase(c, b, &v.datePickerOpen, func(panel *ui.Element) {
			// Leave room below the nested calendar on short laptop windows.
			panel.AttachTo(b, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 8).Width(280).Padding(14).Gap(12).Border(1, t.Border).Radius(t.Radius).Background(t.Surface)
			ui.Text(c, "截止日期与时间").FontSize(13).FontWeight(550)
			v.webDateInput(c)
			v.webTimeInput(c)
			if v.pickerError != "" {
				ui.Text(c, v.pickerError).Role(ui.RoleStatus).FontSize(11).LineHeight(1.5).TextColor(t.Danger)
			}
			ui.Row(c).Gap(8).Children(func() {
				b := webButton(c, "清除截止时间", false).Grow(1)
				b.Children(func() { ui.Text(c, "清除") })
				if b.Clicked() {
					v.formDue = ""
					v.formRemind = false
					v.deadline = deadlineSegments{}
					v.datePickerOpen = false
				}
				b = webButton(c, "确定截止时间", true).Grow(1)
				b.Children(func() { ui.Text(c, "确定") })
				if b.Clicked() {
					v.applyPicker()
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
