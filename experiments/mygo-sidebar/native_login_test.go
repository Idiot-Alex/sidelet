package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

type testLogin struct {
	status    string
	statusErr error
	set       func(bool) error
	calls     []bool
	opens     int
}

func (b *testLogin) Status() (string, error) { return b.status, b.statusErr }
func (b *testLogin) Set(on bool) error {
	b.calls = append(b.calls, on)
	if b.set != nil {
		return b.set(on)
	}
	b.status = "notRegistered"
	if on {
		b.status = "enabled"
	}
	return nil
}
func (b *testLogin) OpenSettings() error { b.opens++; return nil }
func loginFixture(t *testing.T) (*model, *TasksService, *profile, *testLogin) {
	t.Helper()
	m, s, p := profileFixture(t, t.TempDir())
	b := &testLogin{status: "notRegistered"}
	s.login = &nativeLogin{backend: b}
	return m, s, p, b
}

func TestLoginRoundTripPendingAndExternalState(t *testing.T) {
	m, s, p, b := loginFixture(t)
	_, _ = s.AddWithPin("登录验收任务", "备注", 2, true)
	beforeTasks := slices.Clone(m.Tasks)
	b.status = "notFound" // A never-registered bundle can initially report notFound.
	b.set = func(on bool) error {
		b.status = "notRegistered"
		if on {
			b.status = "requiresApproval"
		}
		return nil
	}
	if err := s.ChangeLogin(true); err != nil {
		t.Fatal(err)
	}
	if !m.Preferences.Startup.Enabled || !p.saved.Preferences.Startup.Enabled || s.login.status != "requiresApproval" {
		t.Fatal("pending registration lost")
	}
	// An already-registered item must not be registered again.
	if err := s.ChangeLogin(true); err != nil || len(b.calls) != 1 {
		t.Fatal("repeat registration", err, b.calls)
	}
	dir := filepath.Dir(p.path)
	p.close()
	restored, _, p2 := profileFixture(t, dir)
	if !restored.Preferences.Startup.Enabled {
		t.Fatal("enabled profile not restored")
	}
	before, _ := os.ReadFile(p2.path)
	b.status = "notRegistered"
	if err := s.CheckLogin(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p2.path)
	if loginRegistered(s.login.status) || !bytes.Equal(before, after) || len(b.calls) != 1 {
		t.Fatal("refresh re-enabled or rewrote preference")
	}
	if !reflect.DeepEqual(beforeTasks, m.Tasks) {
		t.Fatal("login changed tasks")
	}
}

func TestLoginFailureRestoresNativeAndDurableState(t *testing.T) {
	for _, kind := range []string{"apply", "disk", "rollback", "status", "unknown", "unsupported", "drag", "unchanged-native-disk"} {
		t.Run(kind, func(t *testing.T) {
			m, s, p, b := loginFixture(t)
			before := p.saved
			data, _ := os.ReadFile(p.path)
			switch kind {
			case "apply":
				b.set = func(on bool) error {
					b.status = "notRegistered"
					if on {
						b.status = "enabled"
						return errors.New("/private/secret native fail")
					}
					return nil
				}
			case "disk", "rollback", "unchanged-native-disk":
				p.write = func(string, []byte) error { return errors.New("disk full") }
				if kind == "rollback" {
					b.set = func(on bool) error {
						b.status = "enabled"
						if !on {
							return errors.New("rollback failed")
						}
						return nil
					}
				}
				if kind == "unchanged-native-disk" {
					b.status = "enabled"
				}
			case "status":
				b.statusErr = errors.New("query failure")
			case "unknown":
				b.status = "unexpected"
			case "unsupported":
				b.status = kind
			case "drag":
				m.startDrag(0, 0)
			}
			err := s.ChangeLogin(true)
			if err == nil {
				t.Fatal("failure accepted")
			}
			if strings.Contains(err.Error(), "/private/secret") {
				t.Fatal("native details exposed")
			}
			after, _ := os.ReadFile(p.path)
			if !bytes.Equal(data, after) || m.Preferences != before.Preferences || s.revision != 0 {
				t.Fatal("failure published or changed file")
			}
			want := []bool{}
			if kind == "apply" || kind == "disk" || kind == "rollback" {
				want = []bool{true, false}
			}
			if !slices.Equal(b.calls, want) {
				t.Fatal("compensation", b.calls, want)
			}
			if kind == "rollback" && (s.login.status != "enabled" || !strings.Contains(err.Error(), "检查系统登录项")) {
				t.Fatal("rollback failure hidden")
			}
			if (kind == "apply" || kind == "disk") && loginRegistered(s.login.status) {
				t.Fatal("native not restored")
			}
		})
	}
}

func TestLoginDisableAndPreferenceIsolation(t *testing.T) {
	m, s, p, b := loginFixture(t)
	if err := s.ChangeLogin(true); err != nil {
		t.Fatal(err)
	}
	value := m.Preferences
	value.Startup.Enabled = false
	if _, err := s.SavePreferences(value); err == nil {
		t.Fatal("general preferences bypassed native switch")
	}
	if len(b.calls) != 1 || !m.Preferences.Startup.Enabled {
		t.Fatal("bypass changed state")
	}
	if err := s.ChangeLogin(false); err != nil {
		t.Fatal(err)
	}
	if m.Preferences.Startup.Enabled || p.saved.Preferences.Startup.Enabled || s.login.status != "notRegistered" || !slices.Equal(b.calls, []bool{true, false}) {
		t.Fatal("disable failed")
	}
	value = m.Preferences
	value.Startup.ShowMainWindow = true
	if _, err := s.SavePreferences(value); err != nil || len(b.calls) != 2 {
		t.Fatal("unrelated preference changed login", err)
	}
	// External enable is authoritative, even if the preference already says off.
	b.status = "enabled"
	if err := s.ChangeLogin(false); err != nil || len(b.calls) != 3 || b.calls[2] {
		t.Fatal("external enable not removed", err)
	}
}

func TestLoginStartupPresentationRespectsPreference(t *testing.T) {
	m, _, p, _ := loginFixture(t)
	for _, r := range []struct{ fresh, show, force, hide, atLogin, want bool }{
		{true, false, false, false, false, true}, {true, false, false, false, true, false},
		{false, false, false, false, true, false}, {false, true, false, false, true, true},
		{true, false, true, false, true, true}, {true, true, true, true, true, false},
	} {
		p.fresh = r.fresh
		m.Preferences.Startup.ShowMainWindow = r.show
		if got := startupMainVisible(m, r.force, r.hide, r.atLogin); got != r.want {
			t.Fatalf("%+v => %v", r, got)
		}
	}
	m.profile = nil
	m.Preferences.Startup.ShowMainWindow = false
	if startupMainVisible(m, false, false, true) {
		t.Fatal("fresh memory launch activated at login")
	}
}

func TestLoginSettingsUIThemes(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, s, _, b := loginFixture(t)
			if _, err := s.Theme(theme); err != nil {
				t.Fatal(err)
			}
			v := newNativeTasksView(s)
			v.settingsOpen = true
			if err := s.CheckLogin(); err != nil {
				t.Fatal(err)
			}
			u := ui.NewTester(v.render, 1120, 800)
			u.SetFocused(true)
			if !u.HasText("系统状态：未开启") {
				t.Fatal("missing native status")
			}
			focus := func(label string) {
				for i := 0; i < 40 && !u.Focused(label); i++ {
					u.Key(0, ui.KeyTab)
				}
				if !u.Focused(label) {
					t.Fatal("missing control", label)
				}
			}
			focus("登录 Mac 后启动 Sidelet")
			u.Key(0, ui.KeySpace)
			if !m.Preferences.Startup.Enabled || !u.HasText("系统状态：已开启") {
				t.Fatal("switch did not commit")
			}
			b.status = "requiresApproval"
			focus("检查登录状态")
			u.Key(0, ui.KeyEnter)
			if !u.HasText("系统状态：等待系统批准") || !u.HasText("请在系统登录项中允许 Sidelet MyGo Lab；完成后返回这里检查状态。") {
				t.Fatal("approval status missing")
			}
			focus("打开系统登录项")
			u.Key(0, ui.KeyEnter)
			if b.opens != 1 {
				t.Fatal("settings not opened")
			}
			b.status = "notRegistered"
			focus("检查登录状态")
			u.Key(0, ui.KeyEnter)
			if !u.HasText("系统状态：未开启") || len(b.calls) != 1 {
				t.Fatal("external state not reflected")
			}
		})
	}
}
