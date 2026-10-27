package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestTaskEntryDefaultsRadiosAndNotes(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, s, _ := profileFixture(t, t.TempDir())
			m.UITheme = theme
			m.Preferences.Appearance.Theme = theme
			now := time.Date(2026, 10, 26, 20, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
			m.clock = func() time.Time { return now }
			v := newNativeTasksView(s)
			v.openNewTask()
			u := ui.NewTester(v.render, 1120, 900)
			if v.formDue != "2026/10/26 18:00" || !u.HasText("预计完成时间") {
				t.Fatal("missing local default time")
			}
			v.newTitle = "主界面新增"
			if err := u.Click("重要"); err != nil {
				t.Fatal(err)
			}
			if err := u.Click("展开备注"); err != nil {
				t.Fatal(err)
			}
			v.formNote = "可选备注"
			if err := u.Click("添加任务"); err != nil {
				t.Fatal(err)
			}
			if len(m.Tasks) != 1 || m.Tasks[0].Priority != 2 || m.Tasks[0].Note != "可选备注" || m.Tasks[0].DueAt != defaultTaskDue(now) {
				t.Fatal("main form lost fields")
			}
			q := quickFixture(s)
			q.text = "快速新增"
			u = ui.NewTester(q.render, 540, 340)
			if err := u.Click("紧急"); err != nil {
				t.Fatal(err)
			}
			if err := u.Click("快速添加备注"); err != nil {
				t.Fatal(err)
			}
			u.SetSize(540, 410)
			q.details.formNote = "第一行\n第二行"
			if err := u.Click("添加"); err != nil {
				t.Fatal(err)
			}
			if len(m.Tasks) != 2 || m.Tasks[1].Priority != 3 || m.Tasks[1].Note != "第一行\n第二行" || m.Tasks[1].DueAt != defaultTaskDue(now) {
				t.Fatal("quick add lost fields")
			}
		})
	}
}

func TestQuickTitleDoesNotChangeManualTime(t *testing.T) {
	q := quickFixture(testService(newModel()))
	q.text = "明天15:30 联系客户"
	parsed, err := q.input()
	if err != nil || parsed.DueAt == 0 {
		t.Fatal(err)
	}
	q.details.formDue = "2027/01/02 12:30"
	parsed, err = q.input()
	want, _ := parseDue(q.details.formDue, 0, "", q.session.Reference.Location())
	if err != nil || parsed.DueAt != want {
		t.Fatal("recognition overwrote manual date")
	}
	q.details.formDue = ""
	parsed, err = q.input()
	if err != nil || parsed.DueAt != 0 {
		t.Fatal("cleared date was restored")
	}
}
