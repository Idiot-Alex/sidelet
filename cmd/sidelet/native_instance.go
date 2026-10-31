package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
)

func (m *model) applicationTitle() string {
	if m.demoInstance {
		return "Sidelet · 演示"
	}
	return "Sidelet"
}

// Resolve aliases after creating the directory, before acquiring either lock.
// Identity is per profile, so an isolated demo never activates the daily app.
func profileInstanceName(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	path, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return fmt.Sprintf("Sidelet-profile-%x", sha256.Sum256([]byte(path))), nil
}

func requestProfileInstance(dir string, activate func()) (bool, error) {
	key, err := profileInstanceName(dir)
	if err != nil {
		return false, err
	}
	mygo.App.OnSecondInstance(func(_ []string, _ string) { activate() })
	// MyGo keys its cross-platform handoff by app name + package identifier.
	// Use the profile identity only for the lock, then restore the visible name.
	name := mygo.App.Name()
	mygo.App.SetName(key)
	first := mygo.App.RequestSingleInstanceLock()
	mygo.App.SetName(name)
	return first, nil
}
