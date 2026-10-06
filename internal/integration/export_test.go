package integration

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"sidelet/internal/exportdata"
	"sidelet/internal/settings"
	"sidelet/internal/storage"
	"sidelet/internal/todo"
)

func TestExportRealDatabaseWithoutMutation(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	profile := t.TempDir()
	prefs := settings.Open(profile)
	if err := prefs.Save(prefs.Value); err != nil {
		t.Fatal(err)
	}
	settingsBefore, err := os.ReadFile(filepath.Join(profile, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := storage.Open(profile, now)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	v := mutate(t, s, now, todo.Action{Type: "create", Title: "未固定的中文任务", Description: "逗号,与\"引号\"\n两行", DueAt: ptr(now.Add(time.Hour).UnixMilli()), Remind: ptr(true)})
	first := v.Todos[0].ID
	v = mutate(t, s, now, todo.Action{Type: "create", Title: "=1+1", Pin: true, Temporary: ptr(true)})
	var second int64
	for _, x := range v.Todos {
		if x.ID != first {
			second = x.ID
		}
	}
	v = mutate(t, s, now, todo.Action{Type: "create", Title: "稍后任务", Pin: true})
	var third int64
	for _, x := range v.Todos {
		if x.ID != first && x.ID != second {
			third = x.ID
		}
	}
	mutate(t, s, now, todo.Action{Type: "snooze", ID: third, Duration: "30m"})
	mutate(t, s, now, todo.Action{Type: "complete", ID: second})
	before := view(t, s, now)
	output := t.TempDir()
	for _, format := range []string{"json", "csv"} {
		data, err := exportdata.Encode(before, format, "0.1.0", now)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(output, "tasks."+format)
		if err := exportdata.Save(path, format, data, profile); err != nil {
			t.Fatal(err)
		}
		saved, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if format == "json" {
			var doc exportdata.Document
			if err := json.Unmarshal(saved, &doc); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(doc.Todos, before.Todos) || !reflect.DeepEqual(doc.Stacks, before.Stacks) {
				t.Fatal("export lost database fields")
			}
		} else {
			rows, err := csv.NewReader(bytes.NewReader(saved[3:])).ReadAll()
			if err != nil || len(rows) != len(before.Todos)+1 {
				t.Fatalf("CSV task count: %v %v", rows, err)
			}
		}
	}
	if !reflect.DeepEqual(before, view(t, s, now)) {
		t.Fatal("export modified tasks, reminders, layout or undo state")
	}
	settingsAfter, err := os.ReadFile(filepath.Join(profile, "settings.json"))
	if err != nil || !bytes.Equal(settingsBefore, settingsAfter) {
		t.Fatal("export changed preferences")
	}
}
