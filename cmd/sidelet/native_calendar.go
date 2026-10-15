package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The calendar navigates civil days in UTC. Only committing converts a wall
// time to the task's local zone, so a DST gap cannot silently change a date.
func calendarDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func calendarMonth(day time.Time, delta int) time.Time {
	first := time.Date(day.Year(), day.Month()+time.Month(delta), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(day.Day(), last)-1)
}

func chineseDay(t time.Time) string { return t.Format("2006年1月2日") }

func (v *nativeTasksView) webDateInput(c *ui.Context) {
	t := v.visual
	b := webButton(c, "选择截止日期", false).Key("calendar-field").Height(36).Background(t.Field).TextColor(t.Text).Role(ui.RolePopUpButton).Value(chineseDay(v.pickerDate)).Justify(ui.SpaceBetween)
	open := &v.pickerCalendarOpen
	cursor := ui.Local(b, "cursor", func() time.Time { return calendarDay(v.pickerDate) })
	if b.Clicked() {
		*open = !*open
		*cursor = calendarDay(v.pickerDate)
	}
	b.Expanded(*open).Children(func() {
		ui.Text(c, chineseDay(v.pickerDate)).FontSize(12).FontFeatures("tnum").NoWrap()
		webIcon(c, "clock", 14)
	})
	ui.PopoverBase(c, b, open, func(panel *ui.Element) {
		panel.Padding(10).Gap(4).Border(1, t.Border).Radius(t.Radius).Background(t.Surface)
		cur := *cursor
		first := time.Date(cur.Year(), cur.Month(), 1, 0, 0, 0, 0, time.UTC)
		grid := ui.Column(c).Key("calendar-grid").Gap(3).Focusable().Role(ui.RoleTable).Label(first.Format("2006年1月")).AutoFocus()
		move := func(day time.Time) { *cursor = day; c.Invalidate() }
		choose := func(day time.Time) {
			// The clock draft is separate; choosing a day doesn't normalize a
			// nonexistent hour before the user has a chance to correct it.
			v.pickerDate = day
			*open = false
			v.pickerError = ""
			b.Focus()
			c.Invalidate()
		}
		switch {
		case grid.Shortcut(0, ui.KeyLeft):
			move(cur.AddDate(0, 0, -1))
		case grid.Shortcut(0, ui.KeyRight):
			move(cur.AddDate(0, 0, 1))
		case grid.Shortcut(0, ui.KeyUp):
			move(cur.AddDate(0, 0, -7))
		case grid.Shortcut(0, ui.KeyDown):
			move(cur.AddDate(0, 0, 7))
		case grid.Shortcut(0, ui.KeyPageUp):
			move(calendarMonth(cur, -1))
		case grid.Shortcut(0, ui.KeyPageDown):
			move(calendarMonth(cur, 1))
		case grid.Shortcut(0, ui.KeyHome):
			move(first)
		case grid.Shortcut(0, ui.KeyEnd):
			move(first.AddDate(0, 1, -1))
		case grid.Shortcut(0, ui.KeyEnter), grid.Shortcut(0, ui.KeySpace):
			choose(cur)
		}
		grid.Children(func() {
			ui.Row(c).Gap(4).Children(func() {
				previous := webButton(c, "上个月", false).Size(28, 28).Padding(0)
				previous.Children(func() { ui.Text(c, "‹").FontSize(18) })
				if previous.Clicked() {
					move(calendarMonth(cur, -1))
					grid.Focus()
				}
				ui.Text(c, first.Format("2006年1月")).Grow(1).TextAlign(ui.Center).FontSize(13).FontWeight(550)
				next := webButton(c, "下个月", false).Size(28, 28).Padding(0)
				next.Children(func() { ui.Text(c, "›").FontSize(18) })
				if next.Clicked() {
					move(calendarMonth(cur, 1))
					grid.Focus()
				}
			})
			ui.Row(c).Children(func() {
				for _, name := range []string{"一", "二", "三", "四", "五", "六", "日"} {
					ui.Text(c, name).Width(32).FixedLineHeight(24).TextAlign(ui.Center).FontSize(11).TextColor(t.TextMuted).Role(ui.RoleColumnHeader)
				}
			})
			start := first.AddDate(0, 0, -((int(first.Weekday()) + 6) % 7))
			for week := range 6 {
				ui.Row(c).Role(ui.RoleRow).Children(func() {
					for column := range 7 {
						day := start.AddDate(0, 0, week*7+column)
						chosen := day.Equal(calendarDay(v.pickerDate))
						cell := ui.Box(c).Size(32, 28).Center().Radius(5).Role(ui.RoleButton).Label(chineseDay(day)).Checked(chosen)
						fg := t.Text
						if day.Month() != first.Month() {
							fg = t.Subtle
						}
						if chosen {
							cell.Background(t.Accent)
							fg = t.AccentText
						} else if cell.Hovered() || day.Equal(cur) {
							cell.Background(t.Soft)
						}
						if day.Equal(calendarDay(v.service.m.now())) && !chosen {
							cell.Border(1, t.Accent)
						}
						cell.Children(func() { ui.Text(c, strconv.Itoa(day.Day())).FontSize(12).FontFeatures("tnum").TextColor(fg) })
						if day.Equal(cur) {
							grid.ActiveDescendant(cell)
						}
						if cell.Clicked() {
							choose(day)
						}
					}
				})
			}
		})
	})
}

func (v *nativeTasksView) webTimeInput(c *ui.Context) {
	t := v.visual
	ui.Row(c).Key("time-field").Label("选择截止时间").Height(36).Padding(0, 9).Gap(3).Border(1, t.Border).Radius(t.Theme.Radius).Background(t.Field).Children(func() {
		for i, value := range []*string{&v.pickerHour, &v.pickerMinute} {
			if i == 1 {
				ui.Text(c, ":").FontSize(13).TextColor(t.TextMuted)
			}
			label, limit := "截止时间小时", 24
			if i == 1 {
				label, limit = "截止时间分钟", 60
			}
			field := webTextInput(c, value).Label(label).Width(40).Height(32).Padding(4).Border(0, ui.Transparent).Background(ui.Transparent).FontSize(13).FontFeatures("tnum")
			if field.Changed() {
				v.pickerError = ""
			}
			delta := 0
			if field.Shortcut(0, ui.KeyUp) {
				delta = 1
			} else if field.Shortcut(0, ui.KeyDown) {
				delta = -1
			}
			if delta != 0 {
				if n, err := pickerNumber(*value, limit); err == nil {
					*value = fmt.Sprintf("%02d", (n+delta+limit)%limit)
					v.pickerError = ""
				}
			}
		}
	})
}

func pickerNumber(text string, limit int) (int, error) {
	text = strings.TrimSpace(text)
	if len(text) < 1 || len(text) > 2 {
		return 0, errors.New("时间格式无效")
	}
	for _, digit := range text {
		if digit < '0' || digit > '9' {
			return 0, errors.New("时间格式无效")
		}
	}
	n, err := strconv.Atoi(text)
	if err != nil || n >= limit {
		return 0, errors.New("时间超出范围")
	}
	return n, nil
}

func (v *nativeTasksView) applyPicker() {
	hour, hErr := pickerNumber(v.pickerHour, 24)
	minute, mErr := pickerNumber(v.pickerMinute, 60)
	if hErr != nil || mErr != nil {
		v.pickerError = "请输入 0–23 时、0–59 分。"
		return
	}
	text := fmt.Sprintf("%04d/%02d/%02d %02d:%02d", v.pickerDate.Year(), v.pickerDate.Month(), v.pickerDate.Day(), hour, minute)
	// Reuse the task commit validator, including DST gaps and timestamp
	// precision for an unchanged existing date. No task changes until Save.
	if _, err := parseDue(text, v.originalDue, v.originalDueText, v.service.m.now().Location()); err != nil {
		v.pickerError = err.Error()
		return
	}
	v.formDue, v.datePickerOpen, v.pickerError = text, false, ""
}
