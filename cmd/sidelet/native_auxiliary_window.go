package main

import "github.com/egoist/mygo"

// Auxiliary surfaces are opened through the app, not through the OS list of
// document windows. Keep their titles for accessibility and diagnostics.
func newAuxiliaryWindow(opts mygo.WindowOptions) *mygo.Window {
	w := mygo.NewWindow(opts)
	excludeAuxiliaryWindowFromMenu(w)
	return w
}
