package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestJSONImportIsAtomicOnceAndKeepsOriginal(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	_, _ = s.AddWithPin("导入任务", "中文备注", 3, true)
	_, _ = s.SaveTask(0, "提醒任务", "", 2, true, 2000000, true, false, 0)
	_, _ = s.SetDone(1, true, 1)
	original := m.profileState(p.saved.ID)
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	p.close()
	if err = legacyProfileFixture(p.path, data); err != nil {
		t.Fatal(err)
	}
	restored, service, p2 := profileFixture(t, dir)
	if !reflect.DeepEqual(original, restored.profileState(p2.saved.ID)) {
		t.Fatal("import changed state or reminder identity")
	}
	source, _ := os.ReadFile(filepath.Join(dir, "profile.json"))
	if !bytes.Equal(data, source) {
		t.Fatal("source backup changed")
	}
	_, err = service.Add("SQLite 后续任务", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := restored.profileState(p2.saved.ID)
	p2.close()
	// An obsolete or invalid JSON backup must never override committed SQLite.
	if err = os.WriteFile(filepath.Join(dir, "profile.json"), []byte("obsolete backup"), 0600); err != nil {
		t.Fatal(err)
	}
	again, _, p3 := profileFixture(t, dir)
	if !reflect.DeepEqual(want, again.profileState(p3.saved.ID)) {
		t.Fatal("reimported old JSON")
	}
	header, err := os.ReadFile(p3.path)
	if err != nil || !bytes.HasPrefix(header, []byte("SQLite format 3\x00")) {
		t.Fatal("not a SQLite database", err)
	}
}
