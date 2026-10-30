//go:build !windows

package main

import "github.com/egoist/mygo"

func configureTaskbarIcon(*mygo.Window, string) error { return nil }
