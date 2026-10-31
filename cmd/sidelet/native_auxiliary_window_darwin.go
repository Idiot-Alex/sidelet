//go:build darwin

package main

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

func excludeAuxiliaryWindowFromMenu(w *mygo.Window) {
	mygo.RunOnMain(func() {
		window := objc.ID(w.NativeHandle())
		if window == 0 { // Headless tests have no AppKit window.
			return
		}
		// SkipTaskbar is a no-op on macOS. AppKit's window list is also used
		// by the Dock menu; this excludes only the auxiliary window, without
		// changing app activation, focus, transparency or Space membership.
		send(window, "setExcludedFromWindowsMenu:", true)
		app := send(objc.ID(objc.GetClass("NSApplication")), "sharedApplication")
		send(app, "removeWindowsItem:", window)
	})
}
