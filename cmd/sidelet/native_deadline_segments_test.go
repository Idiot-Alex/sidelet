package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestDeadlineSegmentsKeyboardSaveClearAndPrecision(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, v, u := webUITester(t)
			m.UITheme, v.visual = theme, webTheme(theme)
			m.clock = func() time.Time { return time.Date(2028, 1, 30, 12, 30, 0, 0, time.Local) }
			u.SetSize(1120, 1100)
			_ = u.Click("截止时间与更多选项")
			if v.formDue != "" || v.formRemind {
				t.Fatal("ghost segments created a deadline")
			}
			_ = u.Click("截止年份")
			u.Type("2")
			u.Key(ui.Shift, ui.KeyTab)
			u.Key(0, ui.KeyTab)
			u.Type("0")
			if v.deadline.parts[0] != "0000" {
				t.Fatal("re-entering focus appended previous typing", v.deadline.parts[0])
			}
			_ = u.Click("截止年份")
			for i, digits := range []string{"2028", "02", "29", "23", "59"} {
				if !u.Focused(deadlineSegmentLabels[i]) {
					t.Fatal("auto advance failed", i)
				}
				u.Type(digits)
			}
			if v.formDue != "2028/02/29 23:59" {
				t.Fatal(v.formDue)
			}
			u.Command("selectAll")
			u.Command("copy")
			if u.Clipboard() != v.formDue {
				t.Fatal("copy did not include the selected date")
			}
			u.SetClipboard("00")
			u.Command("paste")
			if v.formDue != "2028/02/29 23:00" {
				t.Fatal("segment paste failed", v.formDue)
			}
			u.SetClipboard("2028-02-29T23:59")
			u.Command("paste")
			if v.formDue != "2028/02/29 23:59" {
				t.Fatal("whole date paste failed", v.formDue)
			}
			u.Key(0, ui.KeyUp)
			if v.formDue != "2028/02/29 23:00" {
				t.Fatal("minute wrapped date/hour", v.formDue)
			}
			u.Key(0, ui.KeyDown)
			u.Key(0, ui.KeyLeft)
			if !u.Focused("截止小时") {
				t.Fatal("left didn't move focus")
			}
			u.Key(0, ui.KeyTab)
			if !u.Focused("截止分钟") {
				t.Fatal("Tab didn't move focus")
			}
			v.newTitle = "分段日期验收"
			_ = u.Click("提醒我")
			_ = u.Click("添加任务")
			want := time.Date(2028, 2, 29, 23, 59, 0, 0, time.Local).UnixMilli()
			if len(m.Tasks) != 4 || m.Tasks[3].DueAt != want || !m.Tasks[3].Remind {
				t.Fatal("segmented date wasn't saved")
			}
			// Read/step/revert is still an unchanged wall time: retain original
			// seconds and milliseconds, including on an ambiguous DST date.
			due := want + 12345
			_, err := v.service.SaveTask(4, m.Tasks[3].Title, "", 0, true, due, true, false, m.Tasks[3].Version)
			if err != nil {
				t.Fatal(err)
			}
			u.Frame()
			_ = u.Click("编辑：分段日期验收")
			_ = u.Click("截止分钟")
			u.Key(0, ui.KeyUp)
			u.Key(0, ui.KeyDown)
			_ = u.Click("保存修改")
			if m.Tasks[3].DueAt != due {
				t.Fatal("lost original precision")
			}
			_ = u.Click("编辑：分段日期验收")
			_ = u.Click("截止年份")
			u.Command("selectAll")
			u.Key(0, ui.KeyBackspace)
			if v.formDue != "" || v.formRemind {
				t.Fatal("clear didn't reset reminder")
			}
			_ = u.Click("保存修改")
			if m.Tasks[3].DueAt != 0 || m.Tasks[3].Remind {
				t.Fatal("cleared deadline not saved")
			}
		})
	}
}

func TestDeadlineSegmentsRejectPartialInvalidAndDSTDrafts(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	m, v, u := webUITester(t)
	m.clock = func() time.Time { return time.Date(2026, 3, 7, 12, 0, 0, 0, zone) }
	u.SetSize(1120, 1100)
	_ = u.Click("截止时间与更多选项")
	v.newTitle = "不得归一化日期"
	_ = u.Click("截止年份")
	u.Type("2026")
	_ = u.Click("添加任务")
	if !v.failed || len(m.Tasks) != 3 || v.formDue != "2026// :" {
		t.Fatal("partial draft accepted/lost", v.formDue)
	}
	for _, text := range []string{"2026/02/28 12:30", "2026/03/08 01:30"} {
		v.formDue = text
		u.Frame()
		label, digits := "截止日期", "30"
		if text == "2026/03/08 01:30" {
			label, digits = "截止小时", "02"
		}
		_ = u.Click(label)
		u.Type(digits)
		before := v.formDue
		_ = u.Click("添加任务")
		if !v.failed || v.formDue != before || len(m.Tasks) != 3 {
			t.Fatal("invalid wall time normalized", before)
		}
	}
	_ = u.Click("截止小时")
	u.Type("03")
	_ = u.Click("添加任务")
	if v.failed || len(m.Tasks) != 4 || m.Tasks[3].DueAt != time.Date(2026, 3, 8, 3, 30, 0, 0, zone).UnixMilli() {
		t.Fatal("corrected date not saved")
	}
	_ = u.Click("截止时间与更多选项")
	_ = u.Click("截止年份")
	u.Key(ui.Alt, ui.KeyDown)
	if !v.datePickerOpen {
		t.Fatal("keyboard calendar trigger missing")
	}
	u.Key(0, ui.KeyEscape)
	if v.datePickerOpen || v.formDue != "" {
		t.Fatal("calendar cancellation changed empty date")
	}
}

func TestSidebarRevealLeavesNativeRegionsStableAndHonorsReducedMotion(t *testing.T) {
	for _, side := range []string{"left", "right"} {
		m := newModel()
		m.UITheme, m.Side = "mac", side
		v := &views{m: m}
		u := ui.NewTester(v.webStack, stackWidth, stackHeight)
		m.Hover = 0
		u.Frame()
		before := stackInputRegions(m)
		start := visualRect(t, u, m.Tasks[0].Title)
		u.SetPreferences(ui.Preferences{ReduceMotion: true})
		end := visualRect(t, u, m.Tasks[0].Title)
		if start.X-end.X < 7 || start.X-end.X > 8.1 {
			t.Fatal("title reveal is not 8px", start, end)
		}
		after := stackInputRegions(m)
		if len(before) != len(after) {
			t.Fatal("motion changed input regions")
		}
		for i := range before {
			if before[i] != after[i] {
				t.Fatal("motion changed hit geometry")
			}
		}
		if u.Image().RGBAAt(140, 80).A != 0 {
			t.Fatal("motion painted an empty sidebar region")
		}
	}
	var reveal edgeTitleReveal
	now := time.Unix(1000, 0)
	if p, active := reveal.frame(true, now, false); p != 0 || !active {
		t.Fatal("missing enter motion")
	}
	if p, active := reveal.frame(true, now.Add(100*time.Millisecond), false); p <= .5 || p >= 1 || !active {
		t.Fatal("missing ease-out middle frame")
	}
	if p, _ := reveal.frame(true, now.Add(100*time.Millisecond), false); p < .684 || p > .685 {
		t.Fatal("reveal easing differs from CSS ease-out", p)
	}
	if p, active := reveal.frame(true, now.Add(200*time.Millisecond), false); p != 1 || active {
		t.Fatal("motion didn't stop")
	}
	if _, active := reveal.frame(false, now, false); active {
		t.Fatal("collapse scheduled invisible motion")
	}
	if p, active := reveal.frame(true, now, true); p != 1 || active {
		t.Fatal("reduced motion ignored")
	}
}
