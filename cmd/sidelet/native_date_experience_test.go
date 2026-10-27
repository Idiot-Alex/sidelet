package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestDateShortcutsAndSaveWithoutExtraConfirmation(t *testing.T) {
	for _, surface := range []string{"main", "quick", "card"} {
		t.Run(surface, func(t *testing.T) {
			m := newModel()
			m.UITheme = "mac"
			m.clock = func() time.Time { return time.Date(2026, 12, 31, 10, 0, 0, 0, time.Local) }
			s := testService(m)
			var d *nativeTasksView
			var render func(*ui.Context)
			var submit func()
			index := 0
			switch surface {
			case "main":
				d = newNativeTasksView(s)
				d.beginEdit(s.List().Tasks[0])
				render, submit = d.render, d.webSubmit
			case "quick":
				q := quickFixture(s)
				q.text = "跨年任务"
				_, _ = q.input()
				d, render = q.details, q.render
				submit = func() { q.submit(q.session.Revision) }
				index = len(m.Tasks)
			case "card":
				m.open(0)
				m.edit()
				v := &views{m: m}
				d, render = m.cardDetails, v.webCard
				submit = func() { m.save() }
			}
			u := ui.NewTester(render, 1120, 1100)
			u.SetFocused(true)
			for _, label := range []string{"选择截止日期与时间", "明天", "设为09:00"} {
				if err := u.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			// An invalid visible clock must block submission instead of using
			// the previous committed date behind the open picker.
			d.pickerHour = "25"
			before := len(m.Tasks)
			submit()
			if len(m.Tasks) != before || !d.datePickerOpen || d.pickerError == "" {
				t.Fatal("invalid open picker was silently ignored")
			}
			d.pickerHour = "09"
			submit()
			if len(m.Tasks) <= index || localDue(m.Tasks[index].DueAt, time.Local) != "2027/01/01 09:00" {
				t.Fatal("save did not apply the open picker")
			}
		})
	}
}
