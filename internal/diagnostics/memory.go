// Package diagnostics provides explicit, local-only memory snapshots.
package diagnostics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"time"
)

var labelPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// Reserve rejects duplicate snapshots and labels that can escape the directory.
func Reserve(dir, label string) (string, error) {
	if !labelPattern.MatchString(label) {
		return "", fmt.Errorf("invalid memory snapshot label")
	}
	path := filepath.Join(dir, label)
	return path, os.Mkdir(path, 0700)
}

func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

// Capture runs on a worker, never on the AppKit thread. GC makes snapshots
// comparable but also perturbs memory, so these are not idle baselines.
func Capture(path string) error {
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	for _, name := range []string{"heap", "allocs", "goroutine"} {
		f, err := os.OpenFile(filepath.Join(path, name+".pprof"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		err = pprof.Lookup(name).WriteTo(f, 0)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return WriteJSON(filepath.Join(path, "go.json"), map[string]any{
		"at": time.Now().UTC(), "pid": os.Getpid(), "forcedGC": true,
		"memProfileRate": runtime.MemProfileRate, "goroutines": runtime.NumGoroutine(), "stats": stats,
	})
}
