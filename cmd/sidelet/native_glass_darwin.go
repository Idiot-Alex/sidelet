//go:build darwin

package main

import (
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// Clip the native material and the rendered card together. Otherwise an
// NSVisualEffectView would fill the transparent corners behind the paint.
func setGlassCornerRadius(w *mygo.Window, radius float32) {
	loadPanelAPI()
	view := send(objc.ID(w.NativeHandle()), "contentView")
	send(view, "setWantsLayer:", true)
	layer := send(view, "layer")
	panelAPI.setFloat(layer, objc.RegisterName("setCornerRadius:"), float64(radius))
	send(layer, "setMasksToBounds:", radius > 0)
}
