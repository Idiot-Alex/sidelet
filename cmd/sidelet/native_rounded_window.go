package main

import (
	"log"
	"runtime"

	"github.com/egoist/mygo"
)

// The Windows compositor needs an alpha surface to blend curved edges with
// the desktop. Transparent alone leaves an opaque backing beneath the UI.
// The content itself stays opaque; only unpainted pixels and edge coverage blend.
func newRoundedDesktopWindow(opts mygo.WindowOptions) *mygo.Window {
	if runtime.GOOS == "windows" {
		opts.Vibrancy = mygo.VibrancyPopover
	}
	w := newAuxiliaryWindow(opts)
	if runtime.GOOS == "windows" {
		if err := configureClearTransparency(w, true); err != nil {
			log.Printf("rounded desktop alpha unavailable: %v", err)
		}
	}
	return w
}
