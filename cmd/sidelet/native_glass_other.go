//go:build !darwin

package main

import "github.com/egoist/mygo"

// Other platforms retain their native window shape and painted card radius.
func setGlassCornerRadius(*mygo.Window, float32) {}
