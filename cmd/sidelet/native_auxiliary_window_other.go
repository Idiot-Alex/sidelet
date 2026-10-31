//go:build !darwin

package main

import "github.com/egoist/mygo"

func excludeAuxiliaryWindowFromMenu(*mygo.Window) {}
