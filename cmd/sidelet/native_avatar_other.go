//go:build !windows

package main

import "github.com/egoist/mygo"

func placeAvatarWindow(w *mygo.Window, r mygo.Rectangle, _ bool) {
	if w.IsMinimized() {
		w.Restore()
	}
	if w.Bounds() != r {
		w.SetBounds(r)
	}
}
