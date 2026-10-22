//go:build windows

package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func windowsLoginFixture(t *testing.T) *windowsLogin {
	t.Helper()
	base := `Software\Sidelet\Tests\Login-` + rand.Text()
	w := &windowsLogin{func() (string, error) { return `C:\测试 space 🔔\Sidelet.exe`, nil }, base + `\Run`, base + `\Approval`}
	t.Cleanup(func() {
		for _, path := range []string{w.runKey, w.approvalKey, base} {
			if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil && !errors.Is(err, registry.ErrNotExist) {
				t.Error(err)
			}
		}
	})
	return w
}

func setLoginApproval(t *testing.T, w *windowsLogin, value []byte) {
	t.Helper()
	k, _, err := registry.CreateKey(registry.CURRENT_USER, w.approvalKey, registry.SET_VALUE|registry.WOW64_64KEY)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	if err := k.SetBinaryValue(windowsLoginName, value); err != nil {
		t.Fatal(err)
	}
}

func assertWindowsLoginStatus(t *testing.T, w *windowsLogin, want string) {
	t.Helper()
	got, err := w.Status()
	if err != nil || got != want {
		t.Fatalf("status=%s err=%v want=%s", got, err, want)
	}
}

func TestWindowsLoginRegistryLifecycle(t *testing.T) {
	w := windowsLoginFixture(t)
	assertWindowsLoginStatus(t, w, "notRegistered")
	// An unrelated application value must survive every operation.
	k, _, err := registry.CreateKey(registry.CURRENT_USER, w.runKey, registry.ALL_ACCESS|registry.WOW64_64KEY)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	if err = k.SetStringValue("OtherApp", `"C:\Other.exe"`); err != nil {
		t.Fatal(err)
	}
	if err = w.Set(true); err != nil {
		t.Fatal(err)
	}
	assertWindowsLoginStatus(t, w, "enabled")
	command, _, err := k.GetStringValue(windowsLoginName)
	if err != nil || command != `"C:\测试 space 🔔\Sidelet.exe" --mygo-opened-at-login` {
		t.Fatal(command, err)
	}
	for _, state := range []uint32{2, 3, 6, 7} {
		approval := make([]byte, 12)
		binary.LittleEndian.PutUint32(approval, state)
		approval[4] = 19 // preserve the system timestamp, too
		setLoginApproval(t, w, approval)
		want := "enabled"
		if state&1 != 0 {
			want = "disabledBySystem"
		}
		assertWindowsLoginStatus(t, w, want)
		before, _ := w.read()
		assertWindowsLoginStatus(t, w, want)
		after, _ := w.read()
		if !reflect.DeepEqual(before, after) {
			t.Fatal("status query rewrote registration")
		}
	}
	if err = w.Set(true); err != nil {
		t.Fatal(err)
	}
	assertWindowsLoginStatus(t, w, "enabled")
	state, err := w.read()
	if err != nil || state.approvedPresent {
		t.Fatal("explicit enable did not clear disabled state", err)
	}
	for range 2 {
		if err = w.Set(false); err != nil {
			t.Fatal(err)
		}
		assertWindowsLoginStatus(t, w, "notRegistered")
	}
	if value, _, err := k.GetStringValue("OtherApp"); err != nil || value != `"C:\Other.exe"` {
		t.Fatal("unrelated value changed")
	}
}

func TestWindowsLoginPreservesOtherLocationAndUnknownState(t *testing.T) {
	w := windowsLoginFixture(t)
	other := `"C:\another\Sidelet.exe" --mygo-opened-at-login`
	if err := writeWindowsLoginCommand(w.runKey, other); err != nil {
		t.Fatal(err)
	}
	assertWindowsLoginStatus(t, w, "otherLocation")
	for _, on := range []bool{true, false} {
		if err := w.Set(on); err == nil {
			t.Fatal("overwrote other installation")
		}
	}
	if _, err := w.Checkpoint(); err == nil {
		t.Fatal("accepted other installation for rollback")
	}
	command, _ := w.command()
	if err := writeWindowsLoginCommand(w.runKey, command); err != nil {
		t.Fatal(err)
	}
	for _, approval := range [][]byte{{}, {3}, {8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}} {
		setLoginApproval(t, w, approval)
		if status, err := w.Status(); err == nil || status != "unknown" {
			t.Fatal("unknown approval treated as enabled", status, err)
		}
		if err := w.Set(true); err == nil {
			t.Fatal("overwrote unknown approval")
		}
	}
	w.runKey += "\x00"
	if status, err := w.Status(); err == nil || status != "unknown" {
		t.Fatal("registry error reported as absent")
	}
	w.runKey = strings.TrimSuffix(w.runKey, "\x00")
}

func TestWindowsLoginSQLiteFailureRestoresExactSystemState(t *testing.T) {
	for _, initiallyDisabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "system-disabled"}[initiallyDisabled], func(t *testing.T) {
			w := windowsLoginFixture(t)
			if initiallyDisabled {
				if err := w.Set(true); err != nil {
					t.Fatal(err)
				}
				setLoginApproval(t, w, []byte{3, 0, 0, 0, 42, 43, 44, 45, 46, 47, 48, 49})
			}
			m, s, p, _ := loginFixture(t)
			s.login = &nativeLogin{backend: w}
			before, err := w.read()
			if err != nil {
				t.Fatal(err)
			}
			data, _ := readPersistedProfile(t, p)
			p.write = func(string, []byte) error { return errors.New("synthetic disk failure") }
			if err := s.ChangeLogin(true); err == nil {
				t.Fatal("disk error hidden")
			}
			after, _ := w.read()
			disk, _ := readPersistedProfile(t, p)
			if !reflect.DeepEqual(before, after) || !bytes.Equal(data, disk) || m.Preferences.Startup.Enabled {
				t.Fatal("failed change lost original registration or profile")
			}
		})
	}
}

func TestWindowsLoginExternalDisableDoesNotRewritePreference(t *testing.T) {
	w := windowsLoginFixture(t)
	m, s, p, _ := loginFixture(t)
	s.login = &nativeLogin{backend: w}
	if err := s.ChangeLogin(true); err != nil {
		t.Fatal(err)
	}
	approval := []byte{3, 0, 0, 0, 11, 0, 0, 0, 0, 0, 0, 0}
	setLoginApproval(t, w, approval)
	before, _ := readPersistedProfile(t, p)
	if err := s.CheckLogin(); err != nil {
		t.Fatal(err)
	}
	after, _ := readPersistedProfile(t, p)
	if s.login.status != "disabledBySystem" || !m.Preferences.Startup.Enabled || !bytes.Equal(before, after) {
		t.Fatal("external disable rewrote preference or was hidden")
	}
	// Removing a disabled registration must also restore it after disk failure.
	p.write = func(string, []byte) error { return errors.New("synthetic disk failure") }
	if err := s.ChangeLogin(false); err == nil {
		t.Fatal("disk error hidden")
	}
	state, err := w.read()
	if err != nil || !state.present || !bytes.Equal(state.approval, approval) {
		t.Fatal("disabled registration not restored")
	}
}

func TestWindowsLoginCommandValidation(t *testing.T) {
	for _, path := range []string{`relative.exe`, `C:\bad"name.exe`, "C:\\bad\x00.exe", `C:\script.cmd`, `C:\` + strings.Repeat("a", 260) + `.exe`, `C:\` + strings.Repeat("🔔", 120) + `.exe`} {
		if _, err := windowsLoginCommand(path); err == nil {
			t.Fatal("accepted invalid/too-long command", path)
		}
	}
	path := filepath.Join(`C:\`, "space dir", "中文", "Sidelet.exe")
	if command, err := windowsLoginCommand(path); err != nil || command != `"`+path+`" `+windowsLoginArg {
		t.Fatal(command, err)
	}
}
