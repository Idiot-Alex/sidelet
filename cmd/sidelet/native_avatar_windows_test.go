//go:build windows

package main

import (
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestMinimizedAvatarRestoresWithoutTakingFocus(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	class, _ := windows.UTF16PtrFromString("STATIC")
	hwnd, _, err := inputUser32.NewProc("CreateWindowExW").Call(0x80,
		uintptr(unsafe.Pointer(class)), 0, 0x80000000, 20, 20, 56, 56, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal(err)
	}
	defer inputUser32.NewProc("DestroyWindow").Call(hwnd)
	avatarShowWindow.Call(hwnd, 7 /* SW_SHOWMINNOACTIVE */)
	if iconic, _, _ := avatarIsIconic.Call(hwnd); iconic == 0 {
		t.Fatal("fixture did not minimize")
	}
	foreground := inputUser32.NewProc("GetForegroundWindow")
	before, _, _ := foreground.Call()
	if !restoreAvatarWithoutFocus(hwnd) {
		t.Fatal("minimized avatar not detected")
	}
	if iconic, _, _ := avatarIsIconic.Call(hwnd); iconic != 0 {
		t.Fatal("avatar remains minimized")
	}
	after, _, _ := foreground.Call()
	if after != before {
		t.Fatal("restoring avatar stole foreground focus")
	}
	if restoreAvatarWithoutFocus(hwnd) {
		t.Fatal("visible avatar unnecessarily restored")
	}
}
