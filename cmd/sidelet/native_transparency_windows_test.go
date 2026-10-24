//go:build windows

package main

import (
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestClearTransparencyNativeToggle(t *testing.T) {
	if windows.RtlGetVersion().BuildNumber < 22621 {
		t.Skip("MyGo alpha-material surfaces require Windows 11 22H2")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	user := windows.NewLazySystemDLL("user32.dll")
	class, _ := windows.UTF16PtrFromString("STATIC")
	// A hidden, owned top-level window: no desktop interaction or user data.
	hwnd, _, err := user.NewProc("CreateWindowExW").Call(0x00200080,
		uintptr(unsafe.Pointer(class)), 0, 0x80000000, 0, 0, 64, 64, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatalf("create test window: %v", err)
	}
	defer user.NewProc("DestroyWindow").Call(hwnd)
	get := clearDWM.NewProc("DwmGetWindowAttribute")
	for _, enabled := range []bool{true, false, true, false} {
		if err := configureClearWindow(hwnd, enabled); err != nil {
			t.Fatalf("toggle clear transparency to %t: %v", enabled, err)
		}
		var backdrop int32
		hr, _, _ := get.Call(hwnd, 38, uintptr(unsafe.Pointer(&backdrop)), unsafe.Sizeof(backdrop))
		if int32(hr) < 0 || backdrop != 1 {
			t.Fatalf("material restored unexpectedly: backdrop=%d HRESULT=%#x", backdrop, uint32(hr))
		}
	}
	if err := configureClearWindow(0, true); err == nil {
		t.Fatal("invalid alpha surface did not request opaque fallback")
	}
}
