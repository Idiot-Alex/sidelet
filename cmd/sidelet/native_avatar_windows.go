//go:build windows

package main

import (
	"github.com/egoist/mygo"
	"unsafe"
)

var (
	avatarIsIconic   = inputUser32.NewProc("IsIconic")
	avatarShowWindow = inputUser32.NewProc("ShowWindow")
)

func restoreAvatarWithoutFocus(hwnd uintptr) bool {
	if iconic, _, _ := avatarIsIconic.Call(hwnd); iconic != 0 {
		avatarShowWindow.Call(hwnd, 4 /* SW_SHOWNOACTIVATE */)
		return true
	}
	return false
}

// Change only shell visibility, preserving the avatar's transparent surface
// and circular region rather than opting into the framework's panel lifecycle.
func hideAvatarFromTaskbar(hwnd uintptr) {
	style, _, _ := inputGetStyle.Call(hwnd, ^uintptr(19)) // GWL_EXSTYLE
	want := (style | winToolWindow) &^ uintptr(0x00040000) // WS_EX_APPWINDOW
	if want != style {
		inputSetStyle.Call(hwnd, ^uintptr(19), want)
		inputSetPos.Call(hwnd, 0, 0, 0, 0, 0, 0x37)
	}
}

// Keep native input near the circle without cutting off its antialiased edge.
// The region uses physical pixels, so query the actual rect after sizing.
func placeAvatarWindow(w *mygo.Window, bounds mygo.Rectangle, created bool) {
	var rect [4]int32
	hwnd := w.NativeHandle()
	if created {
		hideAvatarFromTaskbar(hwnd)
	}
	if restoreAvatarWithoutFocus(hwnd) {
		// A minimized window has a sentinel rect near (-32000,-32000).
		// Restore before sizing or constructing the circular native region.
		created = true
		w.Invalidate()
	}
	dpi, _, _ := inputGetDPI.Call(hwnd)
	if dpi == 0 {
		dpi = 96
	}
	px := func(n int) uintptr { return uintptr(winPX(n, int(dpi))) }
	if ok, _, _ := inputGetRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return
	}
	x, y := int32(px(bounds.X)), int32(px(bounds.Y))
	width, height := int32(px(bounds.Width)), int32(px(bounds.Height))
	resize := rect[2]-rect[0] != width || rect[3]-rect[1] != height
	if !resize && !created {
		if rect[0] != x || rect[1] != y {
			// Move the existing surface without resizing or rebuilding its region.
			inputSetPos.Call(hwnd, 0, px(bounds.X), px(bounds.Y), 0, 0, 0x0415)
		}
		return
	}
	// The default WINDOWPOSCHANGING handler applies caption minimum widths
	// even to this frameless window. Only this fixed-size launcher bypasses it.
	inputSetPos.Call(hwnd, 0, px(bounds.X), px(bounds.Y), px(bounds.Width), px(bounds.Height), 0x0414)
	if ok, _, _ := inputGetRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return
	}
	wpx, hpx := int(rect[2]-rect[0]), int(rect[3]-rect[1])
	rgn, _ := smoothWindowRegion(0, 0, wpx, hpx, min(wpx, hpx)/2)
	if rgn == 0 {
		return
	}
	if ok, _, _ := inputSetRegion.Call(hwnd, rgn, 1); ok == 0 {
		inputDelete.Call(rgn)
	}
}
