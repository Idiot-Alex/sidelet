//go:build windows

package main

import (
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestDesktopLayerRecoveryPreservesFocusAndBounds(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	class, _ := windows.UTF16PtrFromString("STATIC")
	hwnd, _, err := inputUser32.NewProc("CreateWindowExW").Call(0x08000080, uintptr(unsafe.Pointer(class)), 0, 0x80000000, 20, 20, 56, 56, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal(err)
	}
	defer inputUser32.NewProc("DestroyWindow").Call(hwnd)
	var before, after [4]int32
	inputGetRect.Call(hwnd, uintptr(unsafe.Pointer(&before)))
	foreground, _, _ := inputUser32.NewProc("GetForegroundWindow").Call()
	raiseDesktopHandle(hwnd)
	style, _, _ := inputGetStyle.Call(hwnd, ^uintptr(19))
	visible, _, _ := inputUser32.NewProc("IsWindowVisible").Call(hwnd)
	inputGetRect.Call(hwnd, uintptr(unsafe.Pointer(&after)))
	current, _, _ := inputUser32.NewProc("GetForegroundWindow").Call()
	if style&8 == 0 || visible == 0 || before != after || foreground != current {
		t.Fatalf("recovery changed focus/bounds or failed to restore layer: style=%x visible=%d rect=%v -> %v focus=%x -> %x", style, visible, before, after, foreground, current)
	}
}
