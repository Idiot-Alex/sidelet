//go:build !darwin

package main

import "github.com/egoist/mygo"

func nativeState() map[string]any      { return map[string]any{} }
func watchCaptures(_ string, _ func()) {}
func focusEditor(w *mygo.Window)       { w.Focus() }
func startInputTrace() func()          { return func() {} }
