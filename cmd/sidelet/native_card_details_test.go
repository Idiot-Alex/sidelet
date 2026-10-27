package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestCardDetailsSavePersistsAndKeepsDesktopMembership(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			dir := t.TempDir()
			m, s, p := profileFixture(t, dir)
			if _, err := s.Theme(theme); err != nil {
				t.Fatal(err)
			}
			if _, err := s.SaveTask(0, "原任务", "原备注", 0, true, 0, false, true, 0); err != nil {
				t.Fatal(err)
			}
			m.open(0)
			m.edit()
			v := &views{m: m}
			u := ui.NewTester(v.webCard, 320, m.cardEditorHeight())
			u.SetFocused(true)
			m.Draft, m.DraftNote = "更新任务", "更新备注"
			m.cardDetails.formDue = "2026/11/02 18:30"
			u.Frame()
			for _, label := range []string{"紧急", "到时提醒", "保存"} {
				if err := u.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			item := m.Tasks[0]
			if m.Editing || item.Title != "更新任务" || item.Note != "更新备注" || item.Priority != 3 || !item.Remind || !item.Temporary || !m.eligible(0) || item.DueAt != item.ReminderAt || localDue(item.DueAt, m.now().Location()) != "2026/11/02 18:30" {
				t.Fatalf("card details not saved: %+v error=%s", item, m.EditError)
			}
			p.close()
			restored, _, _ := profileFixture(t, dir)
			if restored.Tasks[0] != item {
				t.Fatal("card edit did not persist")
			}
		})
	}
}

func TestCardDetailsCancelValidationConflictAndLayeredEscape(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	m.Tasks[0].DueAt = time.Now().Add(time.Hour).UnixMilli()
	m.open(0)
	m.edit()
	v := &views{m: m}
	u := ui.NewTester(v.webCard, 320, 430)
	u.SetFocused(true)
	original := m.Tasks[0]
	d := m.cardDetails
	d.formDue = "invalid"
	if m.save() || !m.Editing || m.EditError == "" || m.Tasks[0] != original {
		t.Fatal("invalid date changed task or discarded draft")
	}
	d.formDue = d.originalDueText
	d.newPriority, d.formRemind = "重要", true
	u.Frame()
	if err := u.Click("选择截止日期与时间"); err != nil {
		t.Fatal(err)
	}
	u.SetSize(320, m.cardEditorHeight())
	if err := u.Click("选择截止日期"); err != nil {
		t.Fatal(err)
	}
	u.SetSize(320, 430)
	u.Key(0, ui.KeyEscape)
	if !m.Editing || !d.datePickerOpen || d.pickerCalendarOpen {
		t.Fatal("Escape should only close calendar")
	}
	u.Key(0, ui.KeyEscape)
	if !m.Editing || d.datePickerOpen {
		t.Fatal("Escape should only close date settings")
	}
	m.Tasks[0].Version++
	if m.save() || !m.Editing || m.cardDetails != d || d.newPriority != "重要" {
		t.Fatal("conflict discarded details")
	}
	u.Frame()
	u.Key(0, ui.KeyEscape)
	if m.Editing || m.cardDetails != nil || m.Tasks[0].Priority != original.Priority || m.Tasks[0].Remind != original.Remind {
		t.Fatalf("cancel applied details: editing=%v details=%v task=%+v", m.Editing, m.cardDetails != nil, m.Tasks[0])
	}
	m.edit()
	if !m.save() || m.Tasks[0].DueAt != original.DueAt {
		t.Fatal("unchanged date lost sub-minute precision")
	}
}
