//go:build darwin

package platform

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -fblocks -Wno-deprecated-declarations -mmacosx-version-min=13.0
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices -framework Carbon
#include <stdlib.h>
#include "window_darwin.h"
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"unsafe"
)

type Window struct {
	pointer   unsafe.Pointer
	id        uint64
	callbacks Callbacks
}
type FocusToken struct {
	pid       int
	ownWindow uintptr
}

var nativeWindows sync.Map
var nativeSequence atomic.Uint64
var foregroundCallbacks sync.Map

func EnableMemoryDiagnostics() { C.SLEnableMemoryDiagnostics() }
func SetControlTheme(pointer unsafe.Pointer, theme string) {
	value := 0
	if theme == "paper" {
		value = 1
	}
	if theme == "graphite" {
		value = 2
	}
	C.SLControlTheme(pointer, C.int(value))
}
func MemoryDiagnostic() json.RawMessage {
	pointer := C.SLMemoryDiagnostic()
	if pointer == nil {
		return json.RawMessage(`null`)
	}
	defer C.free(unsafe.Pointer(pointer))
	return json.RawMessage(C.GoString(pointer))
}

func Bind(pointer unsafe.Pointer, callbacks Callbacks) (*Window, error) {
	if pointer == nil {
		return nil, fmt.Errorf("window has not been created")
	}
	w := &Window{pointer: pointer, id: nativeSequence.Add(1), callbacks: callbacks}
	nativeWindows.Store(w.id, w)
	if !bool(C.SLBind(pointer, C.uint64_t(w.id))) {
		nativeWindows.Delete(w.id)
		return nil, fmt.Errorf("macOS overlay requires a nonactivating NSPanel")
	}
	return w, nil
}
func (w *Window) Passive() error { C.SLPassive(w.pointer); return nil }
func (w *Window) Activate() error {
	if !bool(C.SLActivate(w.pointer)) {
		return fmt.Errorf("macOS panel refused keyboard focus")
	}
	return nil
}
func (w *Window) RegisterKeyboardShortcut() error {
	if !bool(C.SLRegisterKeyboardShortcut(w.pointer)) {
		return fmt.Errorf("Ctrl+Option+T registration failed: %s", C.GoString(C.SLLastError()))
	}
	return nil
}
func (w *Window) EnableInteractionTest() error {
	if !bool(C.SLEnableInteractionTest(w.pointer)) {
		return fmt.Errorf("interaction test receiver requires a bound panel")
	}
	return nil
}
func FocusDiagnostic() string {
	pointer := C.SLFocusDiagnostic()
	if pointer == nil {
		return "null"
	}
	defer C.free(unsafe.Pointer(pointer))
	return C.GoString(pointer)
}
func (token FocusToken) Diagnostic() string {
	value, _ := json.Marshal(map[string]any{"pid": token.pid, "ownWindow": token.ownWindow})
	return string(value)
}
func CaptureForeground() FocusToken {
	return FocusToken{int(C.SLCaptureForeground()), uintptr(C.SLCaptureOwnWindow())}
}
func (token FocusToken) Restore() bool {
	return bool(C.SLRestoreForeground(C.int(token.pid), C.uintptr_t(token.ownWindow)))
}
func ForegroundIsOurs() bool { return bool(C.SLForegroundIsOurs()) }

// ForegroundIsOurs identifies an active overlay session; this also includes
// the ordinary task window, without relying on an inactive app's key window.
func ForegroundApplicationIsOurs() bool { return int(C.SLCaptureForeground()) == os.Getpid() }
func (w *Window) ShowInactive()         { C.SLShow(w.pointer) }
func (w *Window) Hide()                 { C.SLHide(w.pointer) }
func finiteRect(rect Rect) bool {
	for _, v := range []float64{rect.X, rect.Y, rect.Width, rect.Height} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}
func cRect(rect Rect) C.SLRect {
	return C.SLRect{x: C.double(rect.X), y: C.double(rect.Y), width: C.double(rect.Width), height: C.double(rect.Height)}
}
func (w *Window) Move(rect Rect) error {
	if !finiteRect(rect) || rect.Width <= 0 || rect.Height <= 0 {
		return fmt.Errorf("invalid macOS window bounds")
	}
	if !bool(C.SLMove(w.pointer, cRect(rect))) {
		return fmt.Errorf("SetFrame: %s", C.GoString(C.SLLastError()))
	}
	return nil
}
func readJSON(pointer *C.char, target any) error {
	if pointer == nil {
		return fmt.Errorf("AppKit returned no display information")
	}
	defer C.free(unsafe.Pointer(pointer))
	return json.Unmarshal([]byte(C.GoString(pointer)), target)
}
func (w *Window) Display() (Display, error) {
	var d Display
	err := readJSON(C.SLDisplay(w.pointer), &d)
	return d, err
}
func (w *Window) Diagnostic() string {
	pointer := C.SLDiagnostic(w.pointer)
	if pointer == nil {
		return "diagnostic unavailable"
	}
	defer C.free(unsafe.Pointer(pointer))
	return C.GoString(pointer)
}
func Displays() ([]Display, error) {
	var d []Display
	err := readJSON(C.SLDisplays(), &d)
	return d, err
}
func (w *Window) SetRegions(rects []Rect, width, height float64) error {
	if width <= 0 || height <= 0 || math.IsNaN(width) || math.IsNaN(height) || math.IsInf(width, 0) || math.IsInf(height, 0) {
		return fmt.Errorf("invalid CSS viewport")
	}
	parts := make([]C.SLRect, 0, len(rects))
	for _, rect := range rects {
		if !finiteRect(rect) {
			return fmt.Errorf("non-finite hit region")
		}
		left, top := math.Max(0, rect.X), math.Max(0, rect.Y)
		right, bottom := math.Min(width, rect.X+rect.Width), math.Min(height, rect.Y+rect.Height)
		if right > left && bottom > top {
			parts = append(parts, cRect(Rect{left, top, right - left, bottom - top}))
		}
	}
	var pointer *C.SLRect
	if len(parts) > 0 {
		pointer = &parts[0]
	}
	C.SLRegions(w.pointer, pointer, C.int(len(parts)), C.double(width), C.double(height))
	return nil
}
func (w *Window) PlaceQuick(source *Window, anchor Rect, side string) error {
	if !finiteRect(anchor) {
		return fmt.Errorf("non-finite Quick Card anchor")
	}
	display, err := source.Display()
	if err != nil {
		return err
	}
	origin := C.SLClientOrigin(source.pointer)
	area := display.WorkArea
	margin := math.Min(10, math.Min(area.Width, area.Height)/2)
	width, height := math.Min(320, area.Width-2*margin), math.Min(390, area.Height-2*margin)
	x := float64(origin.x) + anchor.X - width - 12
	if side == "left" {
		x = float64(origin.x) + anchor.X + anchor.Width + 12
	}
	y := float64(origin.y) + anchor.Y - 12
	x = math.Max(area.X+margin, math.Min(x, area.X+area.Width-margin-width))
	y = math.Max(area.Y+margin, math.Min(y, area.Y+area.Height-margin-height))
	return w.Move(Rect{x, y, width, height})
}
func (w *Window) Close() { C.SLClose(w.pointer); nativeWindows.Delete(w.id) }
func IsFullscreen() bool { return bool(C.SLIsFullscreen()) }
func WatchForeground(changed func()) (func(), error) {
	id := nativeSequence.Add(1)
	foregroundCallbacks.Store(id, changed)
	C.SLWatchForeground(C.uint64_t(id))
	return func() { C.SLStopWatching(); foregroundCallbacks.Delete(id) }, nil
}

//export sideletNativeEvent
func sideletNativeEvent(id C.uint64_t, kind C.int, x C.double, y C.double, inside C.bool) {
	if kind == 4 {
		if callback, ok := foregroundCallbacks.Load(uint64(id)); ok {
			callback.(func())()
		}
		return
	}
	if value, ok := nativeWindows.Load(uint64(id)); ok {
		w := value.(*Window)
		switch kind {
		case 6:
			if w.callbacks.TestKeyboard != nil {
				w.callbacks.TestKeyboard()
			}
		case 7:
			if w.callbacks.Hotkey != nil {
				w.callbacks.Hotkey()
			}
		case 1:
			if w.callbacks.Pointer != nil {
				w.callbacks.Pointer(float64(x), float64(y), bool(inside))
			}
		case 2:
			if w.callbacks.Changed != nil {
				w.callbacks.Changed()
			}
		case 3:
			if w.callbacks.Blur != nil {
				w.callbacks.Blur()
			}
		case 5:
			if w.callbacks.CheckFullscreen != nil {
				w.callbacks.CheckFullscreen()
			}
		}
	}
}
