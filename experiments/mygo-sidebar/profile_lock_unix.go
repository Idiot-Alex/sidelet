//go:build !windows

package main

import (
	"os"
	"syscall"
)

func lockProfile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
func unlockProfile(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
