//go:build windows

package main

import (
	"fmt"
	"log"
	"reflect"
	"syscall"
	"unsafe"

	"github.com/egoist/mygo"
)

const (
	winNoActivate     = 0x08000000
	winToolWindow     = 0x00000080
	winMouseMove      = 0x0200
	winLeftUp         = 0x0202
	winMouseLeave     = 0x02a3
	winCaptureChanged = 0x0215
)

var (
	inputUser32           = syscall.NewLazyDLL("user32.dll")
	inputGDI              = syscall.NewLazyDLL("gdi32.dll")
	inputComctl           = syscall.NewLazyDLL("comctl32.dll")
	inputGetStyle         = inputUser32.NewProc("GetWindowLongPtrW")
	inputSetStyle         = inputUser32.NewProc("SetWindowLongPtrW")
	inputSetPos           = inputUser32.NewProc("SetWindowPos")
	inputFindChild        = inputUser32.NewProc("FindWindowExW")
	inputGetAncestor      = inputUser32.NewProc("GetAncestor")
	inputGetDPI           = inputUser32.NewProc("GetDpiForWindow")
	inputGetRect          = inputUser32.NewProc("GetWindowRect")
	inputClientScreen     = inputUser32.NewProc("ClientToScreen")
	inputGetCursor        = inputUser32.NewProc("GetCursorPos")
	inputWindowPoint      = inputUser32.NewProc("WindowFromPoint")
	inputSetRegion        = inputUser32.NewProc("SetWindowRgn")
	inputCreateRect       = inputGDI.NewProc("CreateRectRgn")
	inputCreateRound      = inputGDI.NewProc("CreateRoundRectRgn")
	inputCombine          = inputGDI.NewProc("CombineRgn")
	inputDelete           = inputGDI.NewProc("DeleteObject")
	inputSetSubclass      = inputComctl.NewProc("SetWindowSubclass")
	inputRemoveSubclass   = inputComctl.NewProc("RemoveWindowSubclass")
	inputDefSubclass      = inputComctl.NewProc("DefSubclassProc")
	inputSetHook          = inputUser32.NewProc("SetWindowsHookExW")
	inputUnhook           = inputUser32.NewProc("UnhookWindowsHookEx")
	inputNextHook         = inputUser32.NewProc("CallNextHookEx")
	inputThreadID         = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")
	inputSubclassCallback uintptr
	inputFocusCallback    = syscall.NewCallback(overlayFocusHook)
	// All access, including callbacks, is on MyGo's UI thread. No global
	// mouse/keyboard hooks, foreign windows or SDK-private symbols are used.
	inputWindows = map[uintptr]*overlayWindow{}
)

func init() { inputSubclassCallback = syscall.NewCallback(overlaySubclass) }

type winPoint struct{ X, Y int32 }
type winRect struct{ Left, Top, Right, Bottom int32 }

type overlayWindow struct {
	owner                 *overlayInput
	window                *mygo.Window
	top, surface          uintptr
	originalStyle         uintptr
	shapes                []overlayShape
	dpi, offsetX, offsetY int
	regionSet             bool
	releasing             bool
}

type overlayInput struct {
	m                        *model
	stack, card              *mygo.Window
	stackInput, cardInput    *overlayWindow
	onEvent                  func(string)
	pointerPresence          func(bool, bool)
	directFixture            bool
	moveDrag                 func(mygo.Point)
	fallbackWindow           func() *mygo.Window
	hook                     uintptr
	editing, syncing, closed bool
	previous                 quickFocusToken
	lastError                string
	blockedFocus             uint64
}

func newOverlayInput(m *model) *overlayInput     { return &overlayInput{m: m} }
func (a *overlayInput) handlesNativeDrag() bool  { return !a.directFixture && a.lastError == "" }
func (a *overlayInput) bindStack(w *mygo.Window) { a.stack = w; a.stackInput = a.bind(w); a.sync() }
func (a *overlayInput) bindCard(w *mygo.Window)  { a.card = w; a.cardInput = a.bind(w); a.sync() }

func (a *overlayInput) fail(operation string, err error) {
	if a.lastError == "" {
		a.lastError = fmt.Sprintf("%s: %v", operation, err)
		log.Printf("Windows overlay input unavailable: %s", a.lastError)
	}
	// Fail open to the desktop: a failed adapter must never leave a transparent
	// rectangle intercepting other applications. The main task window still works.
	for _, w := range []*mygo.Window{a.stack, a.card} {
		if w != nil {
			w.SetIgnoreMouseEvents(true)
		}
	}
}

func (a *overlayInput) bind(w *mygo.Window) *overlayWindow {
	if a.directFixture {
		return nil
	}
	if a.hook == 0 && a.lastError == "" {
		thread, _, _ := inputThreadID.Call()
		h, _, err := inputSetHook.Call(5 /* WH_CBT */, inputFocusCallback, 0, thread)
		if h == 0 {
			a.fail("SetWindowsHookExW(own UI thread)", err)
			return nil
		}
		a.hook = h
	}
	if a.lastError != "" {
		a.fail("bind", nil)
		return nil
	}
	hwnd := w.NativeHandle()
	class, _ := syscall.UTF16PtrFromString("MyGoSurface")
	child, _, err := inputFindChild.Call(hwnd, 0, uintptr(unsafe.Pointer(class)), 0)
	if child == 0 {
		a.fail("FindWindowExW(MyGoSurface)", err)
		return nil
	}
	style, _, _ := inputGetStyle.Call(hwnd, ^uintptr(19) /* GWL_EXSTYLE=-20 */)
	b := &overlayWindow{owner: a, window: w, top: hwnd, surface: child, originalStyle: style}
	for _, h := range []uintptr{hwnd, child} {
		inputWindows[h] = b
		ok, _, err := inputSetSubclass.Call(h, inputSubclassCallback, 1, 0)
		if ok == 0 {
			delete(inputWindows, h)
			a.fail("SetWindowSubclass", err)
		}
	}
	return b
}

func (b *overlayWindow) passive() bool {
	return b == b.owner.stackInput || !b.owner.editing
}

func overlayFocusHook(code, wp, lp uintptr) uintptr {
	if int32(code) == 5 || int32(code) == 9 { // HCBT_ACTIVATE / HCBT_SETFOCUS
		root, _, _ := inputGetAncestor.Call(wp, 2 /* GA_ROOT */)
		if b := inputWindows[root]; b != nil && b.passive() {
			b.owner.blockedFocus++
			return 1 // Veto MyGoSurface's explicit SetFocus before activation occurs.
		}
	}
	r, _, _ := inputNextHook.Call(0, code, wp, lp)
	return r
}

func overlaySubclass(hwnd, msg, wp, lp, id, data uintptr) uintptr {
	b := inputWindows[hwnd]
	if b == nil {
		r, _, _ := inputDefSubclass.Call(hwnd, msg, wp, lp)
		return r
	}
	a := b.owner
	if msg == 0x21 && b.passive() {
		return 3 /* WM_MOUSEACTIVATE: MA_NOACTIVATE */
	}
	if hwnd == b.surface {
		if (msg == winMouseMove || msg == winLeftUp) && b == a.stackInput && a.m.Dragging && a.moveDrag != nil {
			// Convert the delivered coordinate before SetBounds moves the window.
			// Do not rely on UI-engine moves: it coalesces equal local positions.
			p := winPoint{int32(int16(lp)), int32(int16(lp >> 16))}
			inputClientScreen.Call(hwnd, uintptr(unsafe.Pointer(&p)))
			dpi := b.windowDPI()
			a.moveDrag(mygo.Point{X: winDIP(p.X, dpi), Y: winDIP(p.Y, dpi)})
			if msg == winMouseMove {
				return 0
			}
		}
		if msg == winMouseMove {
			p := winPoint{int32(int16(lp)), int32(int16(lp >> 16))}
			inputClientScreen.Call(hwnd, uintptr(unsafe.Pointer(&p)))
			a.presenceAt(p)
		} else if msg == winMouseLeave {
			a.updatePresence()
		}
		if msg == winCaptureChanged && !b.releasing && b == a.stackInput && a.m.Dragging {
			a.m.cancelDrag()
			b.window.SetPosition(a.m.X, a.m.Y)
			b.window.Invalidate()
			a.sync()
			if a.onEvent != nil {
				a.onEvent("drag-cancel-capture")
			}
		}
	}
	if msg == winLeftUp {
		b.releasing = true
		defer func() { b.releasing = false }()
	}
	r, _, _ := inputDefSubclass.Call(hwnd, msg, wp, lp)
	if msg == 0x82 { // WM_NCDESTROY; callbacks must never retain a recycled HWND.
		inputRemoveSubclass.Call(hwnd, inputSubclassCallback, id)
		delete(inputWindows, hwnd)
	} else if hwnd == b.top && (msg == 0x5 || msg == 0x02e0) { // WM_SIZE / WM_DPICHANGED
		a.sync()
	}
	return r
}

func winDIP(px int32, dpi int) int { return (int(px)*96 + dpi/2) / dpi } // MyGo coordinates
func winPX(dip, dpi int) int       { return (dip*dpi + 48) / 96 }
func (b *overlayWindow) windowDPI() int {
	n, _, _ := inputGetDPI.Call(b.top)
	if n == 0 {
		return 96
	}
	return int(n)
}

func (a *overlayInput) sync() {
	if a.closed || a.directFixture || a.syncing || a.lastError != "" {
		return
	}
	a.syncing = true
	defer func() { a.syncing = false }()
	for _, b := range []*overlayWindow{a.stackInput, a.cardInput} {
		if b == nil {
			continue
		}
		style, _, _ := inputGetStyle.Call(b.top, ^uintptr(19))
		want := style | winToolWindow
		if b.passive() {
			want |= winNoActivate
		} else {
			want &^= winNoActivate
		}
		if want != style {
			inputSetStyle.Call(b.top, ^uintptr(19), want)
			inputSetPos.Call(b.top, 0, 0, 0, 0, 0, 0x37 /* FRAMECHANGED|NOACTIVATE|NOZORDER|NOMOVE|NOSIZE */)
			got, _, _ := inputGetStyle.Call(b.top, ^uintptr(19))
			if got != want {
				a.fail("SetWindowLongPtrW", fmt.Errorf("style did not change"))
				return
			}
		}
		shapes := stackInputShapes(a.m)
		if b == a.cardInput {
			shapes = nil
			if a.m.cardOpen() {
				bounds := b.window.Bounds()
				radius := 12
				if a.m.UITheme != "" {
					radius = int(webTheme(a.m.UITheme).Radius)
				}
				shapes = []overlayShape{{rect: mygo.Rectangle{Width: bounds.Width, Height: bounds.Height}, radius: radius}}
			}
		}
		if err := b.setRegion(shapes); err != nil {
			a.fail("SetWindowRgn", err)
			return
		}
	}
}

func (b *overlayWindow) setRegion(shapes []overlayShape) error {
	dpi := b.windowDPI()
	var frame winRect
	var origin winPoint
	inputGetRect.Call(b.top, uintptr(unsafe.Pointer(&frame)))
	inputClientScreen.Call(b.top, uintptr(unsafe.Pointer(&origin)))
	ox, oy := int(origin.X-frame.Left), int(origin.Y-frame.Top)
	if b.regionSet && b.dpi == dpi && b.offsetX == ox && b.offsetY == oy && reflect.DeepEqual(b.shapes, shapes) {
		return nil
	}
	region, _, err := inputCreateRect.Call(0, 0, 0, 0)
	if region == 0 {
		return err
	}
	owned := true
	defer func() {
		if owned {
			inputDelete.Call(region)
		}
	}()
	for _, s := range shapes {
		x, y := winPX(s.rect.X, dpi)+ox, winPX(s.rect.Y, dpi)+oy
		right, bottom := winPX(s.rect.X+s.rect.Width, dpi)+ox, winPX(s.rect.Y+s.rect.Height, dpi)+oy
		radius := winPX(s.radius, dpi)
		part, err := smoothWindowRegion(x, y, right, bottom, radius)
		if part == 0 {
			return err
		}
		if s.squareEdge != "" {
			left, edge := x, right-radius
			if s.squareEdge == "right" {
				left, edge = x+radius, right
			}
			square, _, e := inputCreateRect.Call(uintptr(left), uintptr(y), uintptr(edge), uintptr(bottom))
			if square == 0 {
				inputDelete.Call(part)
				return e
			}
			ok, _, e := inputCombine.Call(part, part, square, 2 /* RGN_OR */)
			inputDelete.Call(square)
			if ok == 0 {
				inputDelete.Call(part)
				return e
			}
		}
		ok, _, e := inputCombine.Call(region, region, part, 2)
		inputDelete.Call(part)
		if ok == 0 {
			return e
		}
	}
	ok, _, err := inputSetRegion.Call(b.top, region, 1)
	if ok == 0 {
		return err
	}
	owned = false // Windows owns the HRGN after successful SetWindowRgn.
	b.shapes, b.dpi, b.offsetX, b.offsetY, b.regionSet = shapes, dpi, ox, oy, true
	return nil
}

func inputHit(p winPoint) uintptr {
	// Supported Windows targets are amd64 and arm64; POINT is passed by value.
	w, _, _ := inputWindowPoint.Call(uintptr(uint64(uint32(p.X)) | uint64(uint32(p.Y))<<32))
	r, _, _ := inputGetAncestor.Call(w, 2)
	return r
}

func (a *overlayInput) updatePresence() {
	var p winPoint
	ok, _, _ := inputGetCursor.Call(uintptr(unsafe.Pointer(&p)))
	if ok == 0 {
		return
	}
	a.presenceAt(p)
}

func (a *overlayInput) presenceAt(p winPoint) {
	if a.pointerPresence == nil || !a.m.cardOpen() {
		return
	}
	hit := inputHit(p)
	source, card := false, a.cardInput != nil && hit == a.cardInput.top
	if a.stackInput != nil && hit == a.stackInput.top {
		var origin winPoint
		inputClientScreen.Call(a.stackInput.top, uintptr(unsafe.Pointer(&origin)))
		dpi := a.stackInput.windowDPI()
		for _, s := range stackInputShapes(a.m) {
			if s.task != a.m.Opened && !(a.m.OverflowOpen && s.task == overflowIndex) {
				continue
			}
			r := s.rect
			x, y := int(p.X-origin.X), int(p.Y-origin.Y)
			if x >= winPX(r.X, dpi) && x < winPX(r.X+r.Width, dpi) && y >= winPX(r.Y, dpi) && y < winPX(r.Y+r.Height, dpi) {
				source = true
				break
			}
		}
	}
	a.pointerPresence(source, card)
}

func (a *overlayInput) beginEdit(w *mygo.Window) {
	if w == nil || a.lastError != "" {
		return
	}
	if !a.editing {
		a.previous = captureQuickForeground()
		a.editing = true
	}
	a.sync()
	focusEditor(w)
}

func (a *overlayInput) endEdit() {
	if !a.editing {
		return
	}
	front, _, _ := quickGetForeground.Call()
	// A user who switched to another app (or our main window) owns that choice.
	if a.card != nil && front == a.card.NativeHandle() {
		restored := a.previous != nil && a.previous.restore()
		if !restored && a.fallbackWindow != nil {
			if w := a.fallbackWindow(); w != nil && w.IsVisible() {
				w.Focus()
			}
		}
		if current, _, _ := quickGetForeground.Call(); current == a.card.NativeHandle() && a.card.IsVisible() {
			a.card.Hide()
			a.card.ShowInactive()
		}
	}
	if a.previous != nil {
		a.previous.release()
		a.previous = nil
	}
	a.editing = false
	a.sync()
}

func (a *overlayInput) foregroundForQuickAdd() quickFocusToken {
	front, _, _ := quickGetForeground.Call()
	if a.editing && a.card != nil && front == a.card.NativeHandle() {
		if f, ok := a.previous.(*winQuickFocus); ok && f.window != 0 {
			copy := *f
			return &copy
		}
	}
	return captureQuickForeground()
}

func (a *overlayInput) state() map[string]any {
	return map[string]any{"adapter": "win32-shaped-overlay", "focusHookActive": a.hook != 0, "editorActive": a.editing, "blockedFocus": a.blockedFocus, "error": a.lastError, "directFixture": a.directFixture}
}

func (a *overlayInput) close() {
	if a.closed {
		return
	}
	a.endEdit()
	a.closed = true
	for _, b := range []*overlayWindow{a.stackInput, a.cardInput} {
		if b == nil {
			continue
		}
		b.window.Hide()
		for _, h := range []uintptr{b.surface, b.top} {
			if inputWindows[h] != b {
				continue
			}
			inputRemoveSubclass.Call(h, inputSubclassCallback, 1)
			delete(inputWindows, h)
		}
		inputSetRegion.Call(b.top, 0, 0)
		inputSetStyle.Call(b.top, ^uintptr(19), b.originalStyle)
	}
	if a.hook != 0 {
		inputUnhook.Call(a.hook)
		a.hook = 0
	}
}
