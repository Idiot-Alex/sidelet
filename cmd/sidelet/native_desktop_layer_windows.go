//go:build windows

package main

import (
	"log"
	"syscall"
	"time"

	"github.com/egoist/mygo"
)

// Show Desktop can raise Explorer's WorkerW above windows that still carry
// WS_EX_TOPMOST, without hiding or minimizing them. Reapply the actual Z order
// after foreground changes; no polling, foreground activation, or global keys.
// Register/unregister on MyGo's UI thread, which owns the WinEvent message loop.
func watchDesktopLayers(recover func()) func() {
	active, generation := true, uint64(0)
	var timers []*time.Timer
	callback := syscall.NewCallback(func(_ uintptr, _ uint32, _ uintptr, _ int32, _ int32, _ uint32, _ uint32) uintptr {
		if !active {
			return 0
		}
		generation++
		current := generation
		for _, timer := range timers {
			timer.Stop()
		}
		timers = timers[:0]
		// Explorer completes desktop transitions asynchronously. The second
		// bounded pass handles a late shell reorder, without a permanent timer.
		for _, delay := range []time.Duration{120 * time.Millisecond, 450 * time.Millisecond} {
			timers = append(timers, time.AfterFunc(delay, func() {
				mygo.RunOnMain(func() {
					if active && generation == current {
						recover()
					}
				})
			}))
		}
		return 0
	})
	hook, _, err := inputUser32.NewProc("SetWinEventHook").Call(3, 3, 0, callback, 0, 0, 2) // FOREGROUND, OUTOFCONTEXT | SKIPOWNPROCESS
	if hook == 0 {
		log.Printf("desktop layer recovery unavailable: %v", err)
	}
	return func() {
		active = false
		generation++
		for _, timer := range timers {
			timer.Stop()
		}
		if hook != 0 {
			inputUser32.NewProc("UnhookWinEvent").Call(hook)
		}
	}
}

func raiseDesktopWindow(w *mygo.Window) {
	if w == nil {
		return
	}
	hwnd := w.NativeHandle()
	raiseDesktopHandle(hwnd)
}

func raiseDesktopHandle(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	restoreAvatarWithoutFocus(hwnd)
	// HWND_TOPMOST, NOSIZE | NOMOVE | NOACTIVATE | SHOWWINDOW | NOOWNERZORDER.
	inputSetPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, 0x253)
}
