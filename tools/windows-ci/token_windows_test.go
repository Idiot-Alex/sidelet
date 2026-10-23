package main

import (
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

var tokenChildProbe = flag.Bool("sidelet-token-child", false, "run the restricted child probe")

func TestRestrictedChildProbe(t *testing.T) {
	if !*tokenChildProbe {
		t.Skip("launched only by the restricted environment regression")
	}
	if os.Getenv("SIDELET_TOKEN_ENV_PROBE") != "inherited-from-job" {
		t.Fatal("restricted child lost the job environment")
	}
	if _, err := run([]string{"check", strconv.Itoa(os.Getpid()), filepath.Join(t.TempDir(), "token.json")}); err != nil {
		t.Fatal(err)
	}
	if _, err := run([]string{"access", filepath.Join(t.TempDir(), "access.json")}); err != nil {
		t.Fatal(err)
	}
	// PowerShell/native command output capture creates pipes in the reduced
	// process itself. A parent-created inherited pipe doesn't test that DACL.
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("restricted child cannot create its own output pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()
	if _, err := writer.WriteString("pipe probe"); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	data, err := io.ReadAll(reader)
	if err != nil || string(data) != "pipe probe" {
		t.Fatalf("restricted pipe roundtrip: %q, %v", data, err)
	}
}

func TestRestrictedChildKeepsJobEnvironment(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Skip("restricted process launch is tested only on disposable hosted CI")
	}
	t.Setenv("SIDELET_TOKEN_ENV_PROBE", "inherited-from-job")
	token, err := restrictedToken()
	if err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := commandWithToken(ctx, token, self, "-test.run=^TestRestrictedChildProbe$", "-sidelet-token-child")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("restricted child: %v\n%s", err, output)
	}
}

func TestInspectActualProcessToken(t *testing.T) {
	state, err := inspectToken(windows.GetCurrentProcessToken())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(state.Integrity, "S-1-16-") {
		t.Fatalf("missing integrity label: %+v", state)
	}
}

func TestRestrictedTokenMeetsAcceptance(t *testing.T) {
	token, err := restrictedToken()
	if err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	state, err := inspectToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.validate(); err != nil {
		t.Fatal(err)
	}
}

func TestTokenAcceptanceRejectsPrivileges(t *testing.T) {
	base := tokenState{Integrity: "S-1-16-8192", AdminAttributes: windows.SE_GROUP_USE_FOR_DENY_ONLY}
	for _, tc := range []struct {
		name string
		edit func(*tokenState)
	}{
		{"elevated", func(s *tokenState) { s.Elevated = true }},
		{"high integrity", func(s *tokenState) { s.Integrity = "S-1-16-12288" }},
		{"low integrity", func(s *tokenState) { s.Integrity = "S-1-16-4096" }},
		{"admin enabled", func(s *tokenState) { s.AdminAttributes |= windows.SE_GROUP_ENABLED }},
		{"admin not deny only", func(s *tokenState) { s.AdminAttributes = windows.SE_GROUP_MANDATORY }},
		{"extra enabled privilege", func(s *tokenState) { s.OtherEnabled = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := base
			tc.edit(&state)
			if err := state.validate(); err == nil {
				t.Fatal("unsafe token accepted")
			}
		})
	}
	if err := base.validate(); err != nil {
		t.Fatal(err)
	}
	base.AdminAttributes = 0 // A true standard account has no administrator SID.
	if err := base.validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRunRefusesNonHostedMachine(t *testing.T) {
	for _, env := range [][2]string{{"", ""}, {"true", "self-hosted"}, {"false", "github-hosted"}} {
		t.Setenv("GITHUB_ACTIONS", env[0])
		t.Setenv("RUNNER_ENVIRONMENT", env[1])
		for _, args := range [][]string{{"suite", "amd64", "."}, {"check", "1", "unused.json"}, {"access", "unused.json"}, {"setup", "unused.json", "unused.exe"}} {
			if _, err := run(args); err == nil {
				t.Fatalf("accepted non-hosted environment %v for %v", env, args)
			}
		}
	}
}
