//go:build windows

package main

import (
	"errors"
	"os"
	"syscall"
	"unsafe"

	"github.com/egoist/mygo"
)

var quickUser32 = syscall.NewLazyDLL("user32.dll")
var quickGetForeground = quickUser32.NewProc("GetForegroundWindow")
var quickGetPID = quickUser32.NewProc("GetWindowThreadProcessId")
var quickIsWindow = quickUser32.NewProc("IsWindow")
var quickSetForeground = quickUser32.NewProc("SetForegroundWindow")

type winQuickFocus struct {
	window  uintptr
	process uint32
}

func captureQuickForeground() quickFocusToken {
	w, _, _ := quickGetForeground.Call()
	var pid uint32
	quickGetPID.Call(w, uintptr(unsafe.Pointer(&pid)))
	return &winQuickFocus{w, pid}
}
func quickForegroundIsOurs() bool { return captureQuickForeground().pid() == os.Getpid() }
func (f *winQuickFocus) pid() int { return int(f.process) }
func (f *winQuickFocus) release() { f.window, f.process = 0, 0 }
func (f *winQuickFocus) restore() bool {
	if !quickForegroundIsOurs() || f.window == 0 {
		return false
	}
	valid, _, _ := quickIsWindow.Call(f.window)
	var pid uint32
	quickGetPID.Call(f.window, uintptr(unsafe.Pointer(&pid)))
	if valid == 0 || pid != f.process {
		return false
	}
	ok, _, _ := quickSetForeground.Call(f.window)
	return ok != 0
}
func registerNativeQuickAddShortcut(pressed func()) (func(), error) {
	if err := mygo.GlobalShortcut.Register(nativeQuickAccelerator, pressed); err != nil {
		return nil, err
	}
	return func() { mygo.GlobalShortcut.Unregister(nativeQuickAccelerator) }, nil
}
func registerNativeQuickAddDiagnosticShortcut(func()) (func(), error) {
	return nil, errors.New("diagnostic key is available only on macOS")
}
