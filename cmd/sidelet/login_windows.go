//go:build windows

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/egoist/mygo"
	"golang.org/x/sys/windows/registry"
)

const (
	windowsLoginRunKey      = `Software\Microsoft\Windows\CurrentVersion\Run`
	windowsLoginApprovalKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
	windowsLoginName        = "Sidelet"
	// Public MyGo startup contract: the SDK consumes this before flag.Parse and
	// reports WasOpenedAtLogin(), so the normal startup preference is reused.
	windowsLoginArg = "--mygo-opened-at-login"
)

type windowsLogin struct {
	executable func() (string, error)
	// Tests inject private HKCU subkeys, never real startup registration.
	runKey, approvalKey string
}

type windowsLoginState struct {
	command         string
	present         bool
	approval        []byte
	approvedPresent bool
}

func newLoginBackend() loginBackend {
	return &windowsLogin{os.Executable, windowsLoginRunKey, windowsLoginApprovalKey}
}

func windowsLoginCommand(exe string) (string, error) {
	if !filepath.IsAbs(exe) || !strings.EqualFold(filepath.Ext(exe), ".exe") || strings.ContainsAny(exe, "\x00\"\r\n") {
		return "", errors.New("login startup requires an absolute executable path")
	}
	command := `"` + exe + `" ` + windowsLoginArg
	// Microsoft's Run key contract limits the entire command to 260 characters.
	if len(utf16.Encode([]rune(command))) > 260 {
		return "", errors.New("login startup command exceeds the Windows Run limit")
	}
	return command, nil
}

func (w *windowsLogin) command() (string, error) {
	exe, err := w.executable()
	if err != nil {
		return "", err
	}
	return windowsLoginCommand(exe)
}

func (w *windowsLogin) read() (state windowsLoginState, err error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, w.runKey, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err == nil {
		var kind uint32
		state.command, kind, err = k.GetStringValue(windowsLoginName)
		k.Close()
		if err == nil && kind != registry.SZ {
			return state, registry.ErrUnexpectedType
		}
		state.present = err == nil
	}
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return state, err
	}
	k, err = registry.OpenKey(registry.CURRENT_USER, w.approvalKey, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err == nil {
		state.approval, _, err = k.GetBinaryValue(windowsLoginName)
		k.Close()
		state.approvedPresent = err == nil
	}
	if errors.Is(err, registry.ErrNotExist) {
		err = nil
	}
	return
}

// StartupApproved is a Windows implementation detail, not a documented API.
// Read only the known 12-byte states; an unfamiliar value must not look enabled.
func windowsLoginDisabled(state windowsLoginState) (bool, error) {
	if !state.approvedPresent {
		return false, nil
	}
	if len(state.approval) != 12 {
		return false, errors.New("unknown startup approval format")
	}
	switch binary.LittleEndian.Uint32(state.approval) {
	case 2, 6:
		return false, nil
	case 3, 7:
		return true, nil
	default:
		return false, errors.New("unknown startup approval state")
	}
}

func (w *windowsLogin) Status() (string, error) {
	command, err := w.command()
	if err != nil {
		return "unknown", err
	}
	state, err := w.read()
	if err != nil {
		return "unknown", err
	}
	if !state.present {
		return "notRegistered", nil
	}
	if !strings.EqualFold(state.command, command) {
		return "otherLocation", nil
	}
	disabled, err := windowsLoginDisabled(state)
	if err != nil {
		return "unknown", err
	}
	if disabled {
		return "disabledBySystem", nil
	}
	return "enabled", nil
}

func deleteWindowsLoginValue(path string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.SET_VALUE|registry.WOW64_64KEY)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer k.Close()
	err = k.DeleteValue(windowsLoginName)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}

func writeWindowsLoginCommand(path, command string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(windowsLoginName, command)
}

func (w *windowsLogin) Set(enabled bool) error {
	command, err := w.command()
	if err != nil {
		return err
	}
	state, err := w.read()
	if err != nil {
		return err
	}
	if state.present && !strings.EqualFold(state.command, command) {
		return errors.New("startup item belongs to another Sidelet location")
	}
	if enabled {
		if _, err := windowsLoginDisabled(state); err != nil {
			return err
		}
		if err := writeWindowsLoginCommand(w.runKey, command); err != nil {
			return err
		}
		// Only an explicit user enable may undo Task Manager's disabled choice.
		return deleteWindowsLoginValue(w.approvalKey)
	}
	if !state.present {
		return nil
	}
	if err := deleteWindowsLoginValue(w.runKey); err != nil {
		return err
	}
	return deleteWindowsLoginValue(w.approvalKey)
}

// Restore both values exactly after registration or SQLite failure, including
// a disabled entry's timestamp. Never overwrite a different executable's entry.
func (w *windowsLogin) Checkpoint() (func() error, error) {
	before, err := w.read()
	if err != nil {
		return nil, err
	}
	command, err := w.command()
	if err != nil {
		return nil, err
	}
	if before.present && !strings.EqualFold(before.command, command) {
		return nil, errors.New("startup item belongs to another Sidelet location")
	}
	return func() error {
		current, err := w.read()
		if err != nil {
			return err
		}
		if current.present && !strings.EqualFold(current.command, command) {
			return errors.New("startup item changed externally during rollback")
		}
		var runErr error
		if before.present {
			runErr = writeWindowsLoginCommand(w.runKey, before.command)
		} else {
			runErr = deleteWindowsLoginValue(w.runKey)
		}
		var approvalErr error
		if before.approvedPresent {
			k, _, err := registry.CreateKey(registry.CURRENT_USER, w.approvalKey, registry.SET_VALUE|registry.WOW64_64KEY)
			approvalErr = err
			if err == nil {
				approvalErr = k.SetBinaryValue(windowsLoginName, before.approval)
				k.Close()
			}
		} else {
			approvalErr = deleteWindowsLoginValue(w.approvalKey)
		}
		return errors.Join(runErr, approvalErr)
	}, nil
}

func (*windowsLogin) OpenSettings() error {
	if err := mygo.Shell.OpenExternal("ms-settings:startupapps"); err != nil {
		return fmt.Errorf("open Windows startup settings: %w", err)
	}
	return nil
}
