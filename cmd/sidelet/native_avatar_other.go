//go:build !windows

package main

import "github.com/egoist/mygo"

func placeAvatarWindow(w *mygo.Window, r mygo.Rectangle, _ bool) {
	if w.Bounds() != r {
		w.SetBounds(r)
	}
}
