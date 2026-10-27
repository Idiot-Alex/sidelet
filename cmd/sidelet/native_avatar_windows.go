//go:build windows

package main

import (
	"github.com/egoist/mygo"
	"unsafe"
)

// Clip both native paint and mouse input to the circular avatar. The native
// window region uses physical pixels, so query the actual rect after sizing.
func placeAvatarWindow(w *mygo.Window, bounds mygo.Rectangle, created bool) {
	var rect [4]int32
	hwnd := w.NativeHandle()
	dpi, _, _ := inputGetDPI.Call(hwnd)
	if dpi == 0 {
		dpi = 96
	}
	px := func(n int) uintptr { return uintptr(int64(n) * int64(dpi) / 96) }
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
	wpx, hpx := uintptr(rect[2]-rect[0]), uintptr(rect[3]-rect[1])
	rgn, _, _ := inputCreateRound.Call(0, 0, wpx+1, hpx+1, wpx, hpx)
	if rgn == 0 {
		return
	}
	if ok, _, _ := inputSetRegion.Call(hwnd, rgn, 1); ok == 0 {
		inputDelete.Call(rgn)
	}
}
