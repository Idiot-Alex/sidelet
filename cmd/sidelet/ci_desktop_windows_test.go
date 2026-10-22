//go:build windows

package main

import (
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// This is test-only code: desktop preparation must never ship in Sidelet.
// https://github.com/actions/runner-images/issues/14069
func TestPrepareWindowsCIDesktop(t *testing.T) {
	if os.Getenv("SIDELET_CI_PREPARE_DESKTOP") != "1" {
		t.Skip("explicit disposable CI desktop preparation only")
	}
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || runtime.GOARCH != "arm64" {
		t.Fatal("desktop preparation is restricted to GitHub-hosted Windows ARM64")
	}
	visible := inputUser32.NewProc("IsWindowVisible")
	iconic := inputUser32.NewProc("IsIconic")
	show := inputUser32.NewProc("ShowWindow")
	enumerate := inputUser32.NewProc("EnumWindows")
	className := inputUser32.NewProc("GetClassNameW")
	cloaking := syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmGetWindowAttribute")
	keyboard := inputUser32.NewProc("keybd_event")
	getClass := func(w uintptr) string {
		var name [256]uint16
		className.Call(w, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
		return syscall.UTF16ToString(name[:])
	}
	callback := syscall.NewCallback(func(w, _ uintptr) uintptr {
		v, _, _ := visible.Call(w)
		minimized, _, _ := iconic.Call(w)
		var cloaked int32
		cloaking.Call(w, 14, uintptr(unsafe.Pointer(&cloaked)), 4)
		if v == 0 || minimized != 0 || cloaked != 0 {
			return 1
		}
		class := getClass(w)
		switch class {
		case "Progman", "WorkerW", "Shell_TrayWnd", "Shell_SecondaryTrayWnd":
			return 1
		case "Windows.UI.Core.CoreWindow":
			// Start can be visible above the desktop without being foreground.
			// On this fresh CI desktop Escape dismisses transient shell UI only.
			keyboard.Call(0x1b, 0, 0, 0)
			keyboard.Call(0x1b, 0, 2, 0)
		default:
			// Remaining first-logon dialogs / update terminals are minimized,
			// not terminated. No application under test has been launched yet.
			show.Call(w, 6 /* SW_MINIMIZE */)
		}
		t.Logf("prepared runner window %#x class=%s", w, class)
		return 1
	})
	for i := 0; i < 3; i++ {
		if ok, _, err := enumerate.Call(callback, 0); ok == 0 {
			t.Fatalf("EnumWindows: %v", err)
		}
		time.Sleep(400 * time.Millisecond)
	}
}
