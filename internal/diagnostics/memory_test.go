package diagnostics

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotCannotEscapeOrOverwrite(t *testing.T) {
	dir := t.TempDir()
	for _, label := range []string{"", "../escape", "/absolute", "a/b", "a\\b", "."} {
		if _, err := Reserve(dir, label); err == nil {
			t.Fatalf("accepted %q", label)
		}
	}
	if _, err := Reserve(dir, "baseline"); err != nil {
		t.Fatal(err)
	}
	if _, err := Reserve(dir, "baseline"); err == nil {
		t.Fatal("overwrote prior snapshot")
	}
}

func TestCaptureProducesReadableProfilesAndStats(t *testing.T) {
	path, err := Reserve(t.TempDir(), "baseline")
	if err != nil {
		t.Fatal(err)
	}
	if err := Capture(path); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"heap", "allocs", "goroutine"} {
		f, err := os.Open(filepath.Join(path, name+".pprof"))
		if err != nil {
			t.Fatal(err)
		}
		z, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		var b [1]byte
		if n, err := z.Read(b[:]); n != 1 || err != nil {
			t.Fatalf("empty profile: %s", name)
		}
		z.Close()
		f.Close()
	}
	data, err := os.ReadFile(filepath.Join(path, "go.json"))
	if err != nil {
		t.Fatal(err)
	}
	var stats struct {
		ForcedGC bool
		PID      int
		Stats    struct{ HeapObjects uint64 }
	}
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatal(err)
	}
	if !stats.ForcedGC || stats.PID != os.Getpid() || stats.Stats.HeapObjects == 0 {
		t.Fatal("missing live stats")
	}
}
