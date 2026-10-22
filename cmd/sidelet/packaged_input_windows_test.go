//go:build windows

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPackagedWindowsOverlayInput(t *testing.T) {
	binary := os.Getenv("SIDELET_SMOKE_EXE")
	if binary == "" {
		t.Skip("requires a packaged executable and isolated Windows desktop")
	}
	root := t.TempDir()
	if output := os.Getenv("SIDELET_SMOKE_OUTPUT"); output != "" {
		if err := os.MkdirAll(output, 0700); err != nil {
			t.Fatal(err)
		}
		var err error
		root, err = os.MkdirTemp(output, "overlay-input-")
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-native-main=false", "-native-check", "-probe", "-output", root, "-log-file", filepath.Join(root, "input.log"), "-quit-after", "30s")
	cmd.WaitDelay = 5 * time.Second
	out, runErr := cmd.CombinedOutput()
	data, err := os.ReadFile(filepath.Join(root, "native-check.json"))
	if err != nil {
		t.Fatalf("native report missing: %v; process=%v\n%s", err, runErr, out)
	}
	var report struct {
		Passed bool
		Stages []struct {
			Stage  string
			Passed bool
			Error  string
		}
	}
	if err = json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	for _, s := range report.Stages {
		t.Logf("%s passed=%t %s", s.Stage, s.Passed, s.Error)
	}
	if runErr != nil || !report.Passed || len(report.Stages) < 17 {
		t.Fatalf("Windows overlay diagnostics failed: process=%v\n%s\n%s", runErr, data, out)
	}
}
