package main

import (
	"testing"

	"golang.org/x/sys/windows"
)

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
		for _, args := range [][]string{{"suite", "amd64", "."}, {"check", "1", "unused.json"}, {"setup", "unused.json", "unused.exe"}} {
			if _, err := run(args); err == nil {
				t.Fatalf("accepted non-hosted environment %v for %v", env, args)
			}
		}
	}
}
