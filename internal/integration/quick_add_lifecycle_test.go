package integration

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"sidelet/internal/exportdata"
	"sidelet/internal/quickadd"
	"sidelet/internal/reminder"
	"sidelet/internal/storage"
	"sidelet/internal/todo"
)

func TestQuickAddThroughReminderExportAndRestart(t *testing.T) {
	now := time.Date(2026, 10, 6, 14, 59, 0, 0, time.FixedZone("test", 8*3600))
	dir := t.TempDir()
	open := func(at time.Time) *storage.Store {
		t.Helper()
		s, err := storage.Open(dir, at)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := open(now)
	defer func() { s.Close() }()
	parsed, err := quickadd.Parse("今天下午3点 =1+1 联系客户", true, now)
	if err != nil {
		t.Fatal(err)
	}
	v := mutate(t, s, now, todo.Action{Type: "create", Title: parsed.Title, DueAt: &parsed.DueAt})
	id := v.Todos[0].ID
	os := &notificationSystem{state: reminder.State{Authorization: "authorized"}}
	syncAt(t, s, os, now.Add(time.Minute))
	if len(os.sent) != 0 || task(t, v, id).Remind || task(t, v, id).DisplayMode != "NONE" {
		t.Fatal("Quick Add enabled reminders or pinned without an explicit choice")
	}
	mutate(t, s, now, todo.Action{Type: "pin", ID: id})
	mutate(t, s, now, todo.Action{Type: "edit", ID: id, Title: parsed.Title, Description: "中文备注，含逗号,\n第二行", Remind: ptr(true)})
	v = mutate(t, s, now, todo.Action{Type: "snooze", ID: id, Duration: "1h"})
	delayed := task(t, v, id)
	if delayed.DueAt != parsed.DueAt || delayed.RemindAt != now.Add(time.Hour).UnixMilli() {
		t.Fatal("Snooze changed the parsed due time or failed to delay the reminder")
	}
	syncAt(t, s, os, now.Add(time.Minute))
	if len(os.sent) != 0 {
		t.Fatal("Snoozed Quick Add task delivered at its original due time")
	}
	mutate(t, s, now.Add(2*time.Minute), todo.Action{Type: "complete", ID: id})
	mutate(t, s, now.Add(2*time.Minute+time.Second), todo.Action{Type: "undo"})
	before := view(t, s, now.Add(3*time.Minute))
	s.Close()
	s = open(now.Add(3 * time.Minute))
	after := view(t, s, now.Add(3*time.Minute))
	if !reflect.DeepEqual(before.Todos, after.Todos) || !reflect.DeepEqual(before.Stacks, after.Stacks) {
		t.Fatal("Restart lost the parsed date, edit, pin, Snooze or Undo result")
	}
	syncAt(t, s, os, now.Add(time.Hour))
	if len(os.sent) != 1 || task(t, view(t, s, now), id).ReminderSentAt == 0 {
		t.Fatal("Delayed reminder did not persist its delivery receipt")
	}
	before = view(t, s, now.Add(time.Hour))
	data, err := exportdata.Encode(before, "json", "0.1.0", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	var doc exportdata.Document
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(doc.Todos, before.Todos) || !reflect.DeepEqual(doc.Stacks, before.Stacks) {
		t.Fatal("JSON lost the Quick Add task or reminder receipt")
	}
	data, err = exportdata.Encode(before, "csv", "0.1.0", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))).ReadAll()
	if err != nil || len(rows) != 2 || rows[1][1] != "'"+parsed.Title || rows[1][11] == "" {
		t.Fatalf("CSV lost formula protection or reminder receipt: %v %v", rows, err)
	}
	if !reflect.DeepEqual(before, view(t, s, now.Add(time.Hour))) {
		t.Fatal("Export changed task, reminder or layout state")
	}
	s.Close()
	s = open(now.Add(2 * time.Hour))
	syncAt(t, s, os, now.Add(2*time.Hour))
	after = view(t, s, now.Add(2*time.Hour))
	if len(os.sent) != 1 || !reflect.DeepEqual(before.Todos, after.Todos) || !reflect.DeepEqual(before.Stacks, after.Stacks) {
		t.Fatal("Restart after export duplicated the reminder or changed persisted data")
	}
}
