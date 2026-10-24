//go:build !windows

package main

import "github.com/egoist/mygo"

func configureClearTransparency(*mygo.Window, bool) error { return nil }
