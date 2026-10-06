//go:build windows

// Package platform contains the Spike's native Windows boundary.
// All exported window operations must run on the window's UI thread.
package platform

import (
	"fmt"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

type Window struct {
	handle    uintptr
	active    bool
	callbacks Callbacks
}
type FocusToken struct{ handle uintptr }
type winRect struct{ Left, Top, Right, Bottom int32 }
type point struct{ X, Y int32 }
type monitorInfo struct {
	Size          uint32
	Monitor, Work winRect
	Flags         uint32
	Device        [32]uint16
}

var (
	user              = syscall.NewLazyDLL("user32.dll")
	gdi               = syscall.NewLazyDLL("gdi32.dll")
	common            = syscall.NewLazyDLL("comctl32.dll")
	shcore            = syscall.NewLazyDLL("shcore.dll")
	getStyle          = user.NewProc("GetWindowLongPtrW")
	setStyle          = user.NewProc("SetWindowLongPtrW")
	setPosition       = user.NewProc("SetWindowPos")
	showWindow        = user.NewProc("ShowWindow")
	setWindowRegion   = user.NewProc("SetWindowRgn")
	getClientRect     = user.NewProc("GetClientRect")
	getWindowRect     = user.NewProc("GetWindowRect")
	clientToScreen    = user.NewProc("ClientToScreen")
	getDPI            = user.NewProc("GetDpiForWindow")
	monitorFromWindow = user.NewProc("MonitorFromWindow")
	getMonitorInfo    = user.NewProc("GetMonitorInfoW")
	getForeground     = user.NewProc("GetForegroundWindow")
	getWindowProcess  = user.NewProc("GetWindowThreadProcessId")
	setForeground     = user.NewProc("SetForegroundWindow")
	isWindow          = user.NewProc("IsWindow")
	createRectRegion  = gdi.NewProc("CreateRectRgn")
	combineRegion     = gdi.NewProc("CombineRgn")
	deleteObject      = gdi.NewProc("DeleteObject")
	setSubclass       = common.NewProc("SetWindowSubclass")
	removeSubclass    = common.NewProc("RemoveWindowSubclass")
	defSubclass       = common.NewProc("DefSubclassProc")
	windows           sync.Map
	subclassCallback  uintptr
)

func init() { subclassCallback = syscall.NewCallback(subclass) }

const subclassID = 0x53494445
const exStyleIndex = ^uintptr(19) // signed GWL_EXSTYLE = -20
const noActivate = uintptr(0x08000000)
const toolWindow = uintptr(0x80)
const appWindow = uintptr(0x40000)
const topmost = ^uintptr(0) // HWND_TOPMOST = -1

// PassiveExtendedStyle keeps native style details inside the platform boundary.
func PassiveExtendedStyle() int { return int(noActivate | toolWindow) }

func check(result uintptr, err error, operation string) error {
	if result != 0 {
		return nil
	}
	return fmt.Errorf("%s failed: %v", operation, err)
}
func Bind(pointer unsafe.Pointer, callbacks Callbacks) (*Window, error) {
	if pointer == nil {
		return nil, fmt.Errorf("window has not been created")
	}
	w := &Window{handle: uintptr(pointer), callbacks: callbacks}
	windows.Store(w.handle, w)
	result, _, err := setSubclass.Call(w.handle, subclassCallback, subclassID, 0)
	if failure := check(result, err, "SetWindowSubclass"); failure != nil {
		windows.Delete(w.handle)
		return nil, failure
	}
	if err := w.Passive(); err != nil {
		return nil, err
	}
	return w, nil
}
func subclass(handle, message, wParam, lParam, id, data uintptr) uintptr {
	if value, ok := windows.Load(handle); ok {
		w := value.(*Window)
		switch message {
		case 0x21: // WM_MOUSEACTIVATE: let the mouse through without activating the window.
			if !w.active {
				return 3
			} // MA_NOACTIVATE
		case 0x312: // WM_HOTKEY
			if wParam == 2 {
				if w.callbacks.QuickAdd != nil {
					go w.callbacks.QuickAdd()
				}
				return 0
			}
			if w.callbacks.Hotkey != nil {
				go w.callbacks.Hotkey()
			}
			return 0
		case 0x7E, 0x1A, 0x2E0: // display, work area, DPI changes
			if w.callbacks.Changed != nil {
				go w.callbacks.Changed()
			}
		case 0x6: // WM_ACTIVATE
			if wParam&0xffff == 0 && w.callbacks.Blur != nil {
				go w.callbacks.Blur()
			}
		case 0x82: // WM_NCDESTROY
			removeSubclass.Call(handle, subclassCallback, id)
			windows.Delete(handle)
		}
	}
	result, _, _ := defSubclass.Call(handle, message, wParam, lParam)
	return result
}
func (w *Window) Passive() error {
	w.active = false
	style, _, _ := getStyle.Call(w.handle, exStyleIndex)
	// SetWindowLongPtr may legitimately return zero; confirm the resulting style instead.
	setStyle.Call(w.handle, exStyleIndex, (style|noActivate|toolWindow)&^appWindow)
	after, _, _ := getStyle.Call(w.handle, exStyleIndex)
	if after&noActivate == 0 {
		return fmt.Errorf("WS_EX_NOACTIVATE was not applied")
	}
	result, _, err := setPosition.Call(w.handle, topmost, 0, 0, 0, 0, 0x33) // NOSIZE | NOMOVE | NOACTIVATE | FRAMECHANGED
	return check(result, err, "SetWindowPos(passive)")
}
func (w *Window) Activate() error {
	style, _, _ := getStyle.Call(w.handle, exStyleIndex)
	setStyle.Call(w.handle, exStyleIndex, style&^noActivate)
	w.active = true
	result, _, err := setForeground.Call(w.handle)
	if result == 0 {
		_ = w.Passive()
		return fmt.Errorf("foreground activation refused: %v", err)
	}
	return nil
}
func CaptureForeground() FocusToken { handle, _, _ := getForeground.Call(); return FocusToken{handle} }
func (token FocusToken) Restore() bool {
	valid, _, _ := isWindow.Call(token.handle)
	if valid == 0 {
		return false
	}
	result, _, _ := setForeground.Call(token.handle)
	return result != 0
}
func ForegroundIsOurs() bool {
	handle, _, _ := getForeground.Call()
	_, ok := windows.Load(handle)
	return ok
}
func ForegroundApplicationIsOurs() bool {
	handle, _, _ := getForeground.Call()
	var pid uint32
	thread, _, _ := getWindowProcess.Call(handle, uintptr(unsafe.Pointer(&pid)))
	return thread != 0 && pid == uint32(os.Getpid())
}
func (w *Window) ShowInactive() {
	showWindow.Call(w.handle, 4)
	setPosition.Call(w.handle, topmost, 0, 0, 0, 0, 0x13)
}
func (w *Window) Hide() { showWindow.Call(w.handle, 0) }
func (w *Window) Move(rect Rect) error {
	result, _, err := setPosition.Call(w.handle, topmost, uintptr(int32(math.Round(rect.X))), uintptr(int32(math.Round(rect.Y))), uintptr(int32(math.Round(rect.Width))), uintptr(int32(math.Round(rect.Height))), 0x10)
	return check(result, err, "SetWindowPos(bounds)")
}
func (w *Window) Display() (Display, error) {
	handle, _, _ := monitorFromWindow.Call(w.handle, 2)
	display, err := readMonitor(handle)
	dpi, _, _ := getDPI.Call(w.handle)
	if dpi != 0 {
		display.Scale = float64(dpi) / 96
	}
	return display, err
}
func readMonitor(handle uintptr) (Display, error) {
	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	result, _, err := getMonitorInfo.Call(handle, uintptr(unsafe.Pointer(&info)))
	if failure := check(result, err, "GetMonitorInfo"); failure != nil {
		return Display{}, failure
	}
	var dpiX, dpiY uint32
	dpiProc := shcore.NewProc("GetDpiForMonitor")
	if dpiProc.Find() == nil {
		dpiProc.Call(handle, 0, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
	}
	if dpiX == 0 {
		dpiX = 96
	}
	convert := func(r winRect) Rect {
		return Rect{float64(r.Left), float64(r.Top), float64(r.Right - r.Left), float64(r.Bottom - r.Top)}
	}
	return Display{ID: syscall.UTF16ToString(info.Device[:]), WorkArea: convert(info.Work), Bounds: convert(info.Monitor), Scale: float64(dpiX) / 96, BackingScale: float64(dpiX) / 96, Primary: info.Flags&1 != 0}, nil
}

var monitorCollectors sync.Map
var monitorSequence atomic.Uint64
var monitorCallback = syscall.NewCallback(func(handle, dc, rect, data uintptr) uintptr {
	value, ok := monitorCollectors.Load(data)
	if !ok {
		return 0
	}
	collector := value.(*[]Display)
	if display, err := readMonitor(handle); err == nil {
		*collector = append(*collector, display)
	}
	return 1
})

func Displays() ([]Display, error) {
	result := []Display{}
	id := uintptr(monitorSequence.Add(1))
	monitorCollectors.Store(id, &result)
	defer monitorCollectors.Delete(id)
	ok, _, err := user.NewProc("EnumDisplayMonitors").Call(0, 0, monitorCallback, id)
	return result, check(ok, err, "EnumDisplayMonitors")
}

// SetRegions converts CSS client coordinates to physical window coordinates.
// The OS owns a successfully installed HRGN; only temporary/failed regions are deleted here.
func (w *Window) SetRegions(rects []Rect, viewportWidth, viewportHeight float64) error {
	if viewportWidth <= 0 || viewportHeight <= 0 {
		return fmt.Errorf("invalid CSS viewport")
	}
	var client, outer winRect
	var origin point
	if result, _, err := getClientRect.Call(w.handle, uintptr(unsafe.Pointer(&client))); result == 0 {
		return fmt.Errorf("GetClientRect: %v", err)
	}
	getWindowRect.Call(w.handle, uintptr(unsafe.Pointer(&outer)))
	clientToScreen.Call(w.handle, uintptr(unsafe.Pointer(&origin)))
	scaleX := float64(client.Right-client.Left) / viewportWidth
	scaleY := float64(client.Bottom-client.Top) / viewportHeight
	region, _, err := createRectRegion.Call(0, 0, 0, 0)
	if failure := check(region, err, "CreateRectRgn"); failure != nil {
		return failure
	}
	for _, rect := range rects {
		if rect.Width <= 0 || rect.Height <= 0 {
			continue
		}
		values := []float64{rect.X, rect.Y, rect.Width, rect.Height}
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				deleteObject.Call(region)
				return fmt.Errorf("non-finite hit region")
			}
		}
		left := math.Max(0, rect.X)
		top := math.Max(0, rect.Y)
		right := math.Min(viewportWidth, rect.X+rect.Width)
		bottom := math.Min(viewportHeight, rect.Y+rect.Height)
		if right <= left || bottom <= top {
			continue
		}
		dx, dy := float64(origin.X-outer.Left), float64(origin.Y-outer.Top)
		part, _, err := createRectRegion.Call(uintptr(int32(math.Floor(left*scaleX+dx))), uintptr(int32(math.Floor(top*scaleY+dy))), uintptr(int32(math.Ceil(right*scaleX+dx))), uintptr(int32(math.Ceil(bottom*scaleY+dy))))
		if part == 0 {
			deleteObject.Call(region)
			return fmt.Errorf("CreateRectRgn(part): %v", err)
		}
		combined, _, err := combineRegion.Call(region, region, part, 2) // RGN_OR
		deleteObject.Call(part)
		if combined == 0 {
			deleteObject.Call(region)
			return fmt.Errorf("CombineRgn: %v", err)
		}
	}
	result, _, err := setWindowRegion.Call(w.handle, region, 1)
	if result == 0 {
		deleteObject.Call(region)
		return fmt.Errorf("SetWindowRgn: %v", err)
	}
	return nil
}
func (w *Window) PlaceQuick(source *Window, anchor Rect, side string) error {
	display, err := source.Display()
	if err != nil {
		return err
	}
	var origin point
	clientToScreen.Call(source.handle, uintptr(unsafe.Pointer(&origin)))
	scale := display.Scale
	area := display.WorkArea
	margin := 10 * scale
	width := math.Min(320*scale, area.Width-2*margin)
	height := math.Min(390*scale, area.Height-2*margin)
	x := float64(origin.X) + anchor.X*scale - width - 12*scale
	if side == "left" {
		x = float64(origin.X) + (anchor.X+anchor.Width)*scale + 12*scale
	}
	y := float64(origin.Y) + anchor.Y*scale - 12*scale
	x = math.Max(area.X+margin, math.Min(x, area.X+area.Width-margin-width))
	y = math.Max(area.Y+margin, math.Min(y, area.Y+area.Height-margin-height))
	return w.Move(Rect{x, y, width, height})
}
func (w *Window) RegisterKeyboardShortcut() error {
	result, _, err := user.NewProc("RegisterHotKey").Call(w.handle, 1, 0x4003, 0x54) // NOREPEAT | CTRL | ALT, T
	return check(result, err, "RegisterHotKey(Ctrl+Alt+T)")
}
func (w *Window) ConfigureQuickAdd() {}
func (w *Window) RegisterQuickAddShortcut() error {
	result, _, err := user.NewProc("RegisterHotKey").Call(w.handle, 2, 0x4006, 0x20) // NOREPEAT | CTRL | SHIFT, SPACE
	return check(result, err, "RegisterHotKey(Ctrl+Shift+Space)")
}
func QuickAddDisplay() (Display, error) {
	handle, _, _ := getForeground.Call()
	monitor, _, _ := monitorFromWindow.Call(handle, 2)
	return readMonitor(monitor)
}
func (w *Window) Close() {
	user.NewProc("UnregisterHotKey").Call(w.handle, 1)
	user.NewProc("UnregisterHotKey").Call(w.handle, 2)
	removeSubclass.Call(w.handle, subclassCallback, subclassID)
	windows.Delete(w.handle)
}

// IsFullscreen is a foreground-bounds heuristic, not game/presentation detection proof.
func IsFullscreen() bool {
	handle, _, _ := getForeground.Call()
	if _, ours := windows.Load(handle); ours || handle == 0 {
		return false
	}
	var class [256]uint16
	user.NewProc("GetClassNameW").Call(handle, uintptr(unsafe.Pointer(&class[0])), 256)
	name := syscall.UTF16ToString(class[:])
	if name == "Progman" || name == "WorkerW" || name == "Shell_TrayWnd" {
		return false
	}
	var rect winRect
	result, _, _ := getWindowRect.Call(handle, uintptr(unsafe.Pointer(&rect)))
	monitor, _, _ := monitorFromWindow.Call(handle, 2)
	display, err := readMonitor(monitor)
	if result == 0 || err != nil {
		return false
	}
	b := display.Bounds
	return float64(rect.Left) <= b.X && float64(rect.Top) <= b.Y && float64(rect.Right) >= b.X+b.Width && float64(rect.Bottom) >= b.Y+b.Height
}

// WatchForeground uses accessibility events; no idle polling loop is installed.
func WatchForeground(changed func()) (func(), error) {
	callback := syscall.NewCallback(func(hook, event, handle, object, child, thread, at uintptr) uintptr {
		if event == 0x800B {
			foreground, _, _ := getForeground.Call()
			if handle != foreground || object != 0 || child != 0 {
				return 0
			}
		}
		go changed()
		return 0
	})
	var hooks []uintptr
	for _, event := range []uintptr{3, 11, 0x800B} { // FOREGROUND, MOVESIZEEND, foreground WINDOW LOCATIONCHANGE
		hook, _, err := user.NewProc("SetWinEventHook").Call(event, event, 0, callback, 0, 0, 2) // OUTOFCONTEXT | SKIPOWNPROCESS
		if hook == 0 {
			for _, h := range hooks {
				user.NewProc("UnhookWinEvent").Call(h)
			}
			return nil, fmt.Errorf("SetWinEventHook: %v", err)
		}
		hooks = append(hooks, hook)
	}
	return func() {
		for _, hook := range hooks {
			user.NewProc("UnhookWinEvent").Call(hook)
		}
	}, nil
}

// macOS title bar appearance is independent of the web content theme.
// Windows keeps its existing system-managed title bar.
func SetControlTheme(_ unsafe.Pointer, _ string) {}
