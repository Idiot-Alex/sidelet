package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"time"
)

func defaultTaskDue(now time.Time) int64 {
	return time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location()).UnixMilli()
}

func webPriorityRadios(c *ui.Context, value *string) {
	ui.RadioGroup(c, func() {
		for _, name := range priorityNames {
			ui.Radio(c, value, name, name).FontSize(12)
		}
	}).Label("任务优先级").Row().Gap(14).AlignItems(ui.Center)
}

func (v *nativeTasksView) resetScheduleDraft() {
	v.formDue, v.originalDueText = "", ""
	v.originalDue = 0
	v.formRemind, v.formTemporary, v.datePickerOpen = false, false, false
	v.pickerCalendarOpen = false
	v.pickerError = ""
	v.deadline = deadlineSegments{}
}
func (v *nativeTasksView) webDeadline(c *ui.Context) {
	calendarWasOpen := v.pickerCalendarOpen
	t := v.visual
	field := ui.Row(c.Key("deadline-input")).Label("任务截止时间").Role(ui.RoleGroup).Height(36).Padding(0, 5).Border(1, t.Border).Radius(t.Theme.Radius).Background(t.Field).AlignItems(ui.Center)
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
	})
	if v.datePickerOpen {
		ui.Column(c.Key("inline-deadline")).Label("预计完成时间设置").Padding(12).Gap(10).Margin(8, 0, 0).Border(1, t.Border).Radius(t.Radius).Background(t.Surface).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "截止日期与时间").FontSize(13).FontWeight(550).Grow(1)
				close := webButton(c, "收起时间设置", false).Border(0, ui.Transparent).Padding(3, 0)
				close.Children(func() { ui.Text(c, "收起").FontSize(12) })
				if close.Clicked() {
					v.datePickerOpen = false
					v.pickerCalendarOpen = false
				}
			})
			v.webDateInput(c)
			v.webTimeInput(c)
			if v.pickerError != "" {
				ui.Text(c, v.pickerError).Role(ui.RoleStatus).FontSize(12).TextColor(t.Danger)
			}
			ui.Row(c).Gap(8).Children(func() {
				clear := webButton(c, "清除截止时间", false).Grow(1)
				clear.Children(func() { ui.Text(c, "清除") })
				if clear.Clicked() {
					v.formDue = ""
					v.formRemind = false
					v.deadline = deadlineSegments{}
					v.datePickerOpen = false
					v.pickerCalendarOpen = false
				}
				save := webButton(c, "确定截止时间", true).Grow(1)
				save.Children(func() { ui.Text(c, "确定") })
				if save.Clicked() {
					v.applyPicker()
				}
			})
		})
		if !calendarWasOpen && c.Root().Shortcut(0, ui.KeyEscape) {
			v.datePickerOpen = false
			v.pickerCalendarOpen = false
		}
	}
}
func (v *nativeTasksView) webNotificationStatus(c *ui.Context) {
	p := v.notificationPresentation()
	if !p.attention || p.message == "" {
		return
	}
	visible := v.formRemind
	for _, t := range v.service.m.Tasks {
		if !t.Deleted && !t.Done && t.Remind && t.ReminderSentAt == 0 {
			visible = true
			break
		}
	}
	if !visible {
		return
	}
	t := v.visual
	ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(12, 16).Margin(0, 0, 18).Radius(t.Theme.Radius).Background(p.background).TextColor(p.foreground).Role(ui.RoleStatus).Children(func() {
		ui.Text(c, p.message).FontSize(12).Grow(1).MinWidth(0).LineHeight(1.5)
		if p.action == "" {
			return
		}
		b := webButton(c, p.action, false).Border(0, ui.Transparent).Background(ui.Transparent).TextColor(p.foreground).Padding(6, 8).Shrink(0)
		b.Children(func() { webActionText(c, p.action) })
		if b.Clicked() {
			v.notificationAction(p.action)
		}
	})
}
