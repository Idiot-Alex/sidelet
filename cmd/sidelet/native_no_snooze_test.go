package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestRetiredSnoozeRestoresDesktopAndReminderOnReopen(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	due := m.now().Add(10 * time.Minute).UnixMilli()
	for i := 0; i < 3; i++ {
		if _, err := s.SaveTask(0, "旧隐藏任务", "保留备注", 2, i != 2, due, true, false, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Snooze(i+1, "1h", m.Tasks[i].Version); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.commit(func() error {
		m.Tasks[1].ReminderSentAt = m.now().UnixMilli()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sentID := m.Tasks[1].ReminderID
	p.close()
	restored, _, p2 := profileFixture(t, dir)
	for i, task := range restored.Tasks {
		if task.SnoozedUntil != 0 || task.DueAt != due || task.ReminderAt != due || task.Note != "保留备注" || task.Unpinned != (i == 2) {
			t.Fatalf("migration changed task fields: %+v", task)
		}
		if restored.eligible(i) != (i != 2) {
			t.Fatal("migration did not preserve desktop membership")
		}
	}
	if restored.Tasks[1].ReminderSentAt == 0 || restored.Tasks[1].ReminderID != sentID || restored.Tasks[0].ReminderSentAt != 0 {
		t.Fatal("migration replayed a sent reminder or acknowledged a pending one")
	}
	version := restored.Tasks[0].Version
	p2.close()
	again, _, _ := profileFixture(t, dir)
	if again.Tasks[0].Version != version || again.Tasks[0].SnoozedUntil != 0 {
		t.Fatal("migration was not durable and idempotent")
	}
}

func TestCardHasNoTemporaryHideActions(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		m := newModel()
		m.UITheme = theme
		m.open(0)
		v := &views{m: m}
		u := ui.NewTester(v.webCard, 320, 220)
		for _, label := range []string{"稍后", "30 分钟", "1 小时", "明天 09:00", "现在恢复"} {
			if u.HasText(label) {
				t.Fatalf("retired action still visible: %s", label)
			}
		}
		if !u.HasText("完成") || !u.HasText("编辑") {
			t.Fatal("task actions missing")
		}
	}
}

func TestQuickAddReminderUsesExpectedCompletionTime(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	q := quickFixture(s)
	q.text, q.pin = "到时提醒任务", true
	u := ui.NewTester(q.render, 540, 340)
	if err := u.Click("到时提醒"); err != nil {
		t.Fatal(err)
	}
	q.submit(q.session.Revision)
	if len(m.Tasks) != 1 || !m.Tasks[0].Remind || m.Tasks[0].Unpinned || m.Tasks[0].ReminderAt != m.Tasks[0].DueAt || !p.saved.Tasks[0].Remind {
		t.Fatal("reminder or desktop membership not saved")
	}
}
