//go:build !darwin

package main

import "github.com/egoist/mygo"

// Windows remains compile-only. Do not imply that the macOS panel adapter
// provides native Windows hit testing or focus behavior.
type overlayInput struct {
	onEvent         func(string)
	pointerPresence func(bool, bool)
	directFixture   bool
	moveDrag        func(mygo.Point)
	fallbackWindow  func() *mygo.Window
}

func newOverlayInput(_ *model) *overlayInput   { return &overlayInput{} }
func (*overlayInput) bindStack(_ *mygo.Window) {}
func (*overlayInput) bindCard(_ *mygo.Window)  {}
func (*overlayInput) sync()                    {}
func (*overlayInput) close()                   {}
func (*overlayInput) endEdit()                 {}
func (*overlayInput) handlesNativeDrag() bool  { return false }
func (*overlayInput) state() map[string]any    { return map[string]any{"adapter": "unavailable"} }
func (*overlayInput) beginEdit(w *mygo.Window) { focusEditor(w) }
