package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestNativeDraftMigratesVersionOneWithoutChangingTasks(t *testing.T) {
	dir := t.TempDir()
	p, err := OpenNativeProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Write("", []byte(`{"schema":2,"tasks":[{"title":"迁移前任务"}],"theme":"mac"}`)); err != nil {
		t.Fatal(err)
	}
	before, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	// Recreate the schema used by released builds through build 42.
	if _, err = p.db.Exec(`DROP TABLE native_new_task_draft; PRAGMA user_version=1;`); err != nil {
		t.Fatal(err)
	}
	p.Close()
	for range 2 {
		p, err = OpenNativeProfile(dir)
		if err != nil {
			t.Fatal(err)
		}
		var version int
		if err = p.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 2 {
			t.Fatalf("version=%d: %v", version, err)
		}
		after, err := p.Read()
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("migration changed committed state: %s: %v", after, err)
		}
		p.Close()
	}
}

func TestNativeDraftWritesDoNotTouchTasksOrMetadata(t *testing.T) {
	p, err := OpenNativeProfile(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err = p.Write("", []byte(`{"schema":2,"tasks":[{"title":"保持不变"}],"theme":"paper"}`)); err != nil {
		t.Fatal(err)
	}
	before, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"native_tasks", "native_profile"} {
		for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
			query := fmt.Sprintf("CREATE TRIGGER forbid_%s_%s BEFORE %s ON %s BEGIN SELECT RAISE(ABORT, 'draft touched task data'); END", table, op, op, table)
			if _, err = p.db.Exec(query); err != nil {
				t.Fatal(err)
			}
		}
	}
	draft := []byte(`{"version":1,"title":"尚未提交","due":"2027// :"}`)
	if err = p.WriteDraft(draft); err != nil {
		t.Fatal(err)
	}
	after, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(after, &fields); err != nil || !bytes.Equal(fields["newTaskDraft"], draft) {
		t.Fatalf("draft not read back: %s: %v", after, err)
	}
	delete(fields, "newTaskDraft")
	withoutDraft, _ := json.Marshal(fields)
	if !bytes.Equal(before, withoutDraft) {
		t.Fatal("draft changed domain state")
	}
	for _, invalid := range [][]byte{[]byte(`{"unfinished":`), []byte(`"` + strings.Repeat("x", 1<<20) + `"`)} {
		if err = p.WriteDraft(invalid); err == nil {
			t.Fatal("invalid draft accepted")
		}
		unchanged, err := p.Read()
		if err != nil || !bytes.Equal(after, unchanged) {
			t.Fatal("failed draft write changed data", err)
		}
	}
	if err = p.WriteDraft(nil); err != nil {
		t.Fatal(err)
	}
	cleared, err := p.Read()
	if err != nil || !bytes.Equal(before, cleared) {
		t.Fatal("clear changed task data", err)
	}
}

func TestNativeDraftConsumptionRollsBackTaskCreationAndSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	p, err := OpenNativeProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { p.Close() }()
	initial := []byte(`{"schema":2,"tasks":[],"newTaskDraft":{"version":1,"title":"待添加"}}`)
	if err = p.Write("", initial); err != nil {
		t.Fatal(err)
	}
	before, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.db.Exec(`CREATE TRIGGER fail_clear BEFORE DELETE ON native_new_task_draft BEGIN SELECT RAISE(ABORT, 'cannot clear draft'); END`); err != nil {
		t.Fatal(err)
	}
	created := []byte(`{"schema":2,"tasks":[{"title":"待添加"}]}`)
	if err = p.Write("", created); err == nil {
		t.Fatal("creation succeeded despite draft clear failure")
	}
	after, err := p.Read()
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("task created or draft lost on rollback", err)
	}
	if _, err = p.db.Exec("DROP TRIGGER fail_clear"); err != nil {
		t.Fatal(err)
	}
	if err = p.Write("", created); err != nil {
		t.Fatal(err)
	}
	p.Close()
	p, err = OpenNativeProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	after, err = p.Read()
	if err != nil || !bytes.Equal(created, after) {
		t.Fatalf("restart restored consumed draft or lost task: %s: %v", after, err)
	}
}
