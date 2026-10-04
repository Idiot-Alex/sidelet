//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework ServiceManagement
#include <stdlib.h>
#include "settings_darwin.h"
*/
import "C"
import (
	"errors"
	"unsafe"
)

// SetDockVisible must run on the application's UI thread.
func SetDockVisible(visible bool, control unsafe.Pointer) error {
	var value C.int
	if visible {
		value = 1
	}
	if C.SLSetDockVisible(value, control) == 0 {
		return errors.New("无法更新 Dock 图标，请重试。")
	}
	return nil
}

func LoginStatus() string {
	p := C.SLLoginStatus()
	defer C.free(unsafe.Pointer(p))
	return C.GoString(p)
}
func SetLogin(enabled bool) error {
	var value C.int
	if enabled {
		value = 1
	}
	p := C.SLSetLogin(value)
	if p == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(p))
	return errors.New(C.GoString(p))
}
func OpenSystemSettings(login bool) error {
	var value C.int
	if login {
		value = 1
	}
	if C.SLOpenSettings(value) == 0 {
		return errors.New("无法打开系统设置，请手动打开系统设置。")
	}
	return nil
}
