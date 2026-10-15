//go:build !darwin

package main

import (
	"log"

	"github.com/egoist/mygo"
)

func runNativeChecks(_ *overlayInput, _ *views, _ *mygo.Window, _ string, done func(bool)) {
	log.Print("native checks are only implemented on macOS")
	done(false)
	mygo.App.Quit()
}
