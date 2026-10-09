//go:build !darwin

package main

import "errors"

func watchQuickAddRequests(func()) func() { return func() {} }
func requestNativeQuickAdd(int) error {
	return errors.New("quick-add diagnostic request is available only on macOS")
}
