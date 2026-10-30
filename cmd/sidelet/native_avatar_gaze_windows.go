//go:build windows

package main

import (
	"unsafe"

	"github.com/egoist/mygo"
)

func avatarCursorGaze(w *mygo.Window) gazePoint {
	// Do not subtract framework screen DIPs across monitors: their DPI scales
	// can differ. GetCursorPos and GetWindowRect share physical screen pixels.
	var cursor winPoint
	var rect [4]int32
	if ok, _, _ := inputGetCursor.Call(uintptr(unsafe.Pointer(&cursor))); ok == 0 {
		return gazePoint{}
	}
	if ok, _, _ := inputGetRect.Call(w.NativeHandle(), uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return gazePoint{}
	}
	return avatarGazeTarget(float64(cursor.X), float64(cursor.Y), float64(rect[0]), float64(rect[1]), float64(rect[2]-rect[0]), float64(rect[3]-rect[1]))
}
