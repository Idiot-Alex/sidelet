package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestLocalizedCalendarNavigatesMonthEndsAndSaves(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, v, u := webUITester(t)
			m.UITheme, v.visual = theme, webTheme(theme)
			m.clock = func() time.Time { return time.Date(2028, 1, 30, 16, 0, 0, 0, time.Local) }
			v.newTitle, v.formDue = "月底计划", "2028/01/31 17:45"
			u.SetSize(1120, 1100)
			click := func(label string) {
				t.Helper()
				if err := u.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			click("截止时间与更多选项")
			click("选择截止日期与时间")
			click("选择截止日期")
			if !u.HasText("2028年1月") || !u.HasText("一") || u.HasText("January") || u.HasText("Mo") {
				t.Fatal("calendar isn't localized")
			}
			u.Key(0, ui.KeyPageDown)
			if !u.HasText("2028年2月") {
				t.Fatal("month navigation skipped February")
			}
			u.Key(0, ui.KeyEnter)
			if !u.HasText("2028年2月29日") {
				t.Fatal("leap-year month-end not clamped")
			}
			click("截止时间小时")
			u.Command("selectAll")
			u.Type("23")
			click("截止时间分钟")
			u.Command("selectAll")
			u.Type("59")
			u.Key(0, ui.KeyUp)
			if v.pickerMinute != "00" || v.pickerHour != "23" {
				t.Fatal("minute step rolled date/hour")
			}
			u.Key(0, ui.KeyDown)
			click("确定截止时间")
			if v.datePickerOpen || v.formDue != "2028/02/29 23:59" {
				t.Fatal("localized selection not applied", v.formDue)
			}
			click("选择截止日期与时间")
			click("选择截止日期")
			click("上个月")
			click("上个月")
			if !u.HasText("2027年12月") {
				t.Fatal("year navigation failed")
			}
			u.Key(0, ui.KeyEscape)
			if v.pickerCalendarOpen || !v.datePickerOpen {
				t.Fatal("nested Escape closed wrong layer")
			}
			u.Key(0, ui.KeyEscape)
			if v.datePickerOpen || v.formDue != "2028/02/29 23:59" {
				t.Fatal("cancel changed form")
			}
			click("提醒我")
			click("添加任务")
			want := time.Date(2028, 2, 29, 23, 59, 0, 0, time.Local).UnixMilli()
			if len(m.Tasks) != 4 || m.Tasks[3].DueAt != want || !m.Tasks[3].Remind {
				t.Fatal("localized date not saved")
			}
			if v.formDue != "" || v.formRemind {
				t.Fatal("new form inherited schedule")
			}
			// A shorter non-leap February and backwards year crossing.
			if d := calendarMonth(time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC), 1); d.Month() != time.February || d.Day() != 28 {
				t.Fatal(d)
			}
		})
	}
}

func TestLocalizedPickerRejectsInvalidClockAndDSTGapKeepsPrecision(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	m, v, u := webUITester(t)
	m.clock = func() time.Time { return time.Date(2026, 3, 7, 0, 0, 0, 0, zone) }
	v.formDue, v.newTitle = "2026/03/07 02:30", "未提交计划"
	u.SetSize(1120, 1100)
	_ = u.Click("截止时间与更多选项")
	_ = u.Click("选择截止日期与时间")
	_ = u.Click("选择截止日期")
	u.Key(0, ui.KeyRight)
	u.Key(0, ui.KeyEnter)
	_ = u.Click("确定截止时间")
	if !v.datePickerOpen || v.pickerError == "" || v.formDue != "2026/03/07 02:30" || len(m.Tasks) != 3 {
		t.Fatal("DST gap normalized or consumed form")
	}
	for _, bad := range []struct{ h, m string }{{"24", "00"}, {"12", "60"}, {"-1", "30"}, {"12", ""}, {"002", "00"}, {"ab", "15"}} {
		v.pickerHour, v.pickerMinute = bad.h, bad.m
		u.Frame()
		_ = u.Click("确定截止时间")
		if !v.datePickerOpen || !strings.Contains(v.pickerError, "0–23") || v.formDue != "2026/03/07 02:30" {
			t.Fatal("invalid clock accepted", bad)
		}
	}
	v.pickerHour, v.pickerMinute = "03", "30"
	u.Frame()
	_ = u.Click("确定截止时间")
	if v.formDue != "2026/03/08 03:30" || v.datePickerOpen {
		t.Fatal("valid corrected clock failed")
	}
	// Existing ambiguous local time with seconds/milliseconds stays exact
	// when the picker confirms unchanged displayed wall time.
	due := time.Date(2026, 11, 1, 1, 30, 12, 345000000, zone).UnixMilli()
	_, err = v.service.SaveTask(1, m.Tasks[0].Title, m.Tasks[0].Note, 0, true, due, false, false, 1)
	if err != nil {
		t.Fatal(err)
	}
	u.Frame()
	_ = u.Click("编辑：整理今天的工作")
	_ = u.Click("选择截止日期与时间")
	_ = u.Click("确定截止时间")
	_ = u.Click("保存修改")
	if m.Tasks[0].DueAt != due {
		t.Fatal("picker confirmation rounded original timestamp", m.Tasks[0].DueAt, due)
	}
}

func TestReadingCardUsesRenderedHeightAndSettlesAfterContentChanges(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m := newModel()
			m.UITheme = theme
			m.open(0)
			m.Tasks[0].Title, m.Tasks[0].Note = "短标题", ""
			v := &views{m: m}
			resizes := 0
			v.changed = func(reason string) {
				if reason == "card-size" {
					resizes++
				}
			}
			u := ui.NewTester(v.webCard, 320, 390)
			u.SetFocused(true)
			if m.CardReadHeight != 160 {
				t.Fatal("short card did not reach minimum", m.CardReadHeight)
			}
			fit := func() {
				t.Helper()
				u.SetSize(320, m.CardReadHeight)
				for _, label := range []string{"完成", "稍后", "编辑"} {
					r := visualRect(t, u, label)
					if r.Y < 0 || r.Y+r.H > float32(m.CardReadHeight)-1 {
						t.Fatal("footer clipped", label, r, m.CardReadHeight)
					}
				}
			}
			fit()
			m.Tasks[0].Note = "第一行备注\n第二行备注\n第三行备注"
			u.Frame()
			short := m.CardReadHeight
			if short <= 160 || short >= 390 {
				t.Fatal("multiline content not measured", short)
			}
			fit()
			note := visualRect(t, u, m.Tasks[0].Note)
			footer := visualRect(t, u, "完成")
			if note.Y+note.H > footer.Y-12 {
				t.Fatal("note overlaps footer", note, footer)
			}
			m.Tasks[0].Title = strings.Repeat("很长的任务标题需要换行", 12)
			m.Tasks[0].Note = strings.Repeat("很长的中文备注\n", 80)
			m.Tasks[0].Priority = 3
			m.Tasks[0].DueAt = m.now().Add(-time.Minute).UnixMilli()
			u.Frame()
			if m.CardReadHeight != 390 {
				t.Fatal("long card not capped", m.CardReadHeight)
			}
			fit()
			m.Tasks[0].Title, m.Tasks[0].Note, m.Tasks[0].Priority, m.Tasks[0].DueAt = "短标题", "", 0, 0
			u.Frame()
			if m.CardReadHeight != 160 {
				t.Fatal("stale layout kept long height", m.CardReadHeight)
			}
			fit()
			_ = u.Click("稍后")
			if m.CardReadHeight <= 160 {
				t.Fatal("snooze footer not measured")
			}
			u.SetSize(320, m.CardReadHeight)
			r := visualRect(t, u, "返回操作")
			if r.Y+r.H > float32(m.CardReadHeight)-1 {
				t.Fatal("snooze return clipped")
			}
			_ = u.Click("返回操作")
			fit()
			before := resizes
			for range 5 {
				u.Frame()
			}
			if resizes != before {
				t.Fatal("unchanged frames kept resizing", before, resizes)
			}
			m.EditError = "保存失败；请重试。"
			u.Frame()
			if m.CardReadHeight <= 160 {
				t.Fatal("new error was not measured")
			}
			fit()
			message := visualRect(t, u, m.EditError)
			footer = visualRect(t, u, "完成")
			if message.Y+message.H > footer.Y-12 {
				t.Fatal("error overlaps footer")
			}
			m.EditError = ""
			u.Frame()
			if m.CardReadHeight != 160 {
				t.Fatal("removed error left extra height")
			}
		})
	}
}
