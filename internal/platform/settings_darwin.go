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
