//go:build !windows

package main

import "github.com/egoist/mygo"

func avatarCursorGaze(w *mygo.Window) gazePoint {
	p, r := mygo.Screen.CursorScreenPoint(), w.Bounds()
	return avatarGazeTarget(float64(p.X), float64(p.Y), float64(r.X), float64(r.Y), float64(r.Width), float64(r.Height))
}
