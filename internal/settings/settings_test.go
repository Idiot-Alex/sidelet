package settings

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPreferencesRoundTripAndFailedReplace(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	if s.Exists || s.Value != Defaults() || s.LoadError != nil {
		t.Fatal(s)
	}
	v := s.Value
	v.Startup.ShowMainWindow = true
	v.Edge = Edge{"left", "compact"}
	if err := s.Save(v); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "settings.json")
	before, _ := os.ReadFile(file)
	reopened := Open(dir)
	if reopened.Value != v || !reopened.Exists {
		t.Fatal(reopened)
	}
	info, _ := os.Stat(file)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	s.replace = func(string, string) error { return errors.New("disk failure") }
	changed := v
	changed.Edge.DefaultSide = "right"
	if s.Save(changed) == nil || s.Value != v {
		t.Fatal("failed save changed memory")
	}
	after, _ := os.ReadFile(file)
	if string(after) != string(before) {
		t.Fatal("failed save replaced file")
	}
	files, _ := filepath.Glob(filepath.Join(dir, ".settings-*"))
	if len(files) != 0 {
		t.Fatal(files)
	}
}
func TestInvalidSettingsArePreserved(t *testing.T) {
	for _, raw := range []string{`{`, `null`, `{"version":2}`, `{"edge":{"defaultSide":"top"}}`, `{"edge":{"future":true}}`, `{"future":true}`, `{} {}`} {
		t.Run(raw, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "settings.json")
			os.WriteFile(file, []byte(raw), 0600)
			s := Open(dir)
			if s.LoadError == nil || s.Value != Defaults() {
				t.Fatal(s)
			}
			if s.Save(Defaults()) == nil {
				t.Fatal("overwrote invalid file")
			}
			data, _ := os.ReadFile(file)
			if string(data) != raw {
				t.Fatal(string(data))
			}
		})
	}
	s := Open(t.TempDir())
	v := Defaults()
	v.Edge.DefaultDensity = "dense"
	if s.Save(v) == nil {
		t.Fatal("accepted invalid density")
	}
}
func TestLoginFailureAndCompensation(t *testing.T) {
	s := Open(t.TempDir())
	calls := []bool{}
	apply := func(value bool) error { calls = append(calls, value); return nil }
	if err := s.ChangeLogin(true, false, apply); err != nil || !Open(filepath.Dir(s.path)).Value.Startup.Enabled {
		t.Fatal(err)
	}
	before := s.Value
	s.replace = func(string, string) error { return errors.New("full disk") }
	calls = nil
	if s.ChangeLogin(false, true, apply) == nil || s.Value != before || !reflect.DeepEqual(calls, []bool{false, true}) {
		t.Fatal(calls, s.Value)
	}
	calls = nil
	if s.ChangeLogin(false, true, func(bool) error { return errors.New("OS denied") }) == nil || s.Value != before {
		t.Fatal("OS failure changed settings")
	}
	count := 0
	if err := s.ChangeLogin(false, true, func(bool) error {
		count++
		if count == 2 {
			return errors.New("compensation failed")
		}
		return nil
	}); err == nil || count != 2 {
		t.Fatal(err)
	}
}

func TestThemesAndLegacyPreferences(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "settings.json")
	legacy := `{"version":1,"edge":{"defaultSide":"left","defaultDensity":"compact"},"startup":{"enabled":false,"showMainWindow":true}}`
	if err := os.WriteFile(file, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	s := Open(dir)
	if s.LoadError != nil || s.Value.Appearance.Theme != "mac" || s.Value.Edge.DefaultSide != "left" || !s.Value.Startup.ShowMainWindow {
		t.Fatalf("legacy preferences lost: %+v", s)
	}
	for _, theme := range []string{"paper", "graphite", "mac"} {
		v := s.Value
		v.Appearance.Theme = theme
		if err := s.Save(v); err != nil {
			t.Fatal(err)
		}
		if got := Open(dir); got.LoadError != nil || got.Value != v {
			t.Fatalf("theme did not survive restart: %+v", got)
		}
	}
	before, _ := os.ReadFile(file)
	v := s.Value
	v.Appearance.Theme = "unknown"
	if s.Save(v) == nil {
		t.Fatal("accepted unknown theme")
	}
	after, _ := os.ReadFile(file)
	if string(before) != string(after) || s.Value.Appearance.Theme != "mac" {
		t.Fatal("invalid theme changed committed preference")
	}
	if err := os.WriteFile(file, []byte(`{"appearance":{"theme":"unknown"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := Open(dir); got.LoadError == nil {
		t.Fatal("accepted unknown theme on disk")
	}
}
