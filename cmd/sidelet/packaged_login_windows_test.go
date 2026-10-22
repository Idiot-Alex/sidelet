//go:build windows

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Launch the ZIP's real EXE using the exact login argument, with isolated
// profiles. This tests SDK argument consumption/presentation, not an OS logon.
func TestPackagedWindowsLoginStartup(t *testing.T) {
	binary := os.Getenv("SIDELET_SMOKE_EXE")
	if binary == "" {
		t.Skip("requires the packaged Windows executable")
	}
	root := t.TempDir()
	if output := os.Getenv("SIDELET_SMOKE_OUTPUT"); output != "" {
		var err error
		root, err = os.MkdirTemp(output, "login-startup-")
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct {
		name                     string
		fresh, login, show, want bool
	}{
		{"fresh-login", true, true, false, false},
		{"saved-hidden", false, true, false, false},
		{"saved-visible", false, true, true, true},
		{"fresh-manual", true, false, false, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			dir := filepath.Join(root, row.name, "profile 中文")
			if !row.fresh {
				m := newModel()
				m.UITheme = "mac"
				p, err := openProfile(dir, m)
				if err != nil {
					t.Fatal(err)
				}
				s := testService(m)
				prefs := m.Preferences
				prefs.Startup.ShowMainWindow = row.show
				if _, err := s.SavePreferences(prefs); err != nil {
					p.close()
					t.Fatal(err)
				}
				if _, err := s.AddWithPin("登录启动合成任务", "", 1, true); err != nil {
					p.close()
					t.Fatal(err)
				}
				p.close()
			}
			args := []string{}
			if row.login {
				command, err := windowsLoginCommand(binary)
				if err != nil {
					t.Fatal(err)
				}
				parts, err := windows.DecomposeCommandLine(command)
				if err != nil || len(parts) != 2 || parts[0] != binary || parts[1] != windowsLoginArg {
					t.Fatal("Run command did not round trip", parts, err)
				}
				args = append(args, parts[1:]...)
			}
			logPath := filepath.Join(root, row.name, "startup.log")
			args = append(args, "-data-dir", dir, "-log-file", logPath, "-quit-after", "3s")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, args...)
			cmd.WaitDelay = 5 * time.Second
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("packaged login startup: %v\n%s", err, out)
			}
			data, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			scan := bufio.NewScanner(bytes.NewReader(data))
			scan.Buffer(make([]byte, 4096), 1<<20)
			found := false
			for scan.Scan() {
				line := scan.Bytes()
				at := bytes.IndexByte(line, '{')
				if at < 0 {
					continue
				}
				var entry struct {
					Event         string `json:"event"`
					MainOpen      bool   `json:"mainOpen"`
					OpenedAtLogin bool   `json:"openedAtLogin"`
					StorageError  string `json:"storageError"`
				}
				if err := json.Unmarshal(line[at:], &entry); err != nil {
					t.Fatal(err)
				}
				if entry.StorageError != "" {
					t.Fatal(entry.StorageError)
				}
				if entry.Event == "startup-presentation" {
					found = true
					if entry.MainOpen != row.want || entry.OpenedAtLogin != row.login {
						t.Fatalf("unexpected startup: %+v", entry)
					}
				}
			}
			if err := scan.Err(); err != nil {
				t.Fatal(err)
			}
			if !found {
				t.Fatal("packaged process did not report startup")
			}
			if !row.fresh && len(readSmokeProfile(t, dir).Tasks) != 1 {
				t.Fatal("login lost pinned task")
			}
		})
	}
}
