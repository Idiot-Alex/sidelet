//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var profileKernel = syscall.NewLazyDLL("kernel32.dll")
var profileLockFile = profileKernel.NewProc("LockFileEx")
var profileUnlockFile = profileKernel.NewProc("UnlockFileEx")

func lockProfile(f *os.File) error {
	var overlapped syscall.Overlapped
	ok, _, err := profileLockFile.Call(f.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if ok == 0 {
		return err
	}
	return nil
}
func unlockProfile(f *os.File) error {
	var overlapped syscall.Overlapped
	ok, _, err := profileUnlockFile.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if ok == 0 {
		return err
	}
	return nil
}
