package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestProfileInstanceIdentitySeparatesProfilesAndResolvesAliases(t *testing.T) {
	root := t.TempDir()
	first, err := profileInstanceName(filepath.Join(root, "daily"))
	if err != nil {
		t.Fatal(err)
	}
	again, err := profileInstanceName(filepath.Join(root, "daily", "."))
	if err != nil || first != again {
		t.Fatal("same profile has different identity", err)
	}
	other, err := profileInstanceName(filepath.Join(root, "demo"))
	if err != nil || other == first {
		t.Fatal("independent profile shares an identity", err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(filepath.Join(root, "daily"), alias); err == nil {
		key, err := profileInstanceName(alias)
		if err != nil || key != first {
			t.Fatal("symlink bypasses profile identity", err)
		}
	}
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := profileInstanceName(file); err == nil {
		t.Fatal("invalid profile silently accepted")
	}
}

func TestDemoInstanceHasVisibleIdentityWithoutChangingTasks(t *testing.T) {
	m, v, _ := webUITester(t)
	if m.applicationTitle() != "Sidelet" {
		t.Fatal("daily title changed")
	}
	m.demoInstance = true
	u := ui.NewTester(v.render, 860, 620)
	if m.applicationTitle() != "Sidelet · 演示" || !u.HasText("演示实例 · 独立数据，不影响日常任务") {
		t.Fatal("demo identity missing")
	}
	if m.totalCount() != 3 {
		t.Fatal("identity changed tasks")
	}
}
