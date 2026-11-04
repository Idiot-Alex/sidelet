//go:build !windows

package main

import "github.com/egoist/mygo"

func watchDesktopLayers(func()) func() { return func() {} }
func raiseDesktopWindow(*mygo.Window)  {}
