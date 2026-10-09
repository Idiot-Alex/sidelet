//go:build darwin

package main

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

type macLogin struct{}

var loginFramework struct {
	once sync.Once
	err  error
}

func newLoginBackend() loginBackend { return macLogin{} }

func labLoginService() (objc.ID, error) {
	loginFramework.once.Do(func() {
		_, loginFramework.err = purego.Dlopen("/System/Library/Frameworks/ServiceManagement.framework/ServiceManagement", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	})
	if loginFramework.err != nil {
		return 0, loginFramework.err
	}
	bundle := send(objc.ID(objc.GetClass("NSBundle")), "mainBundle")
	identity := send(bundle, "bundleIdentifier")
	if identity == 0 || objc.Send[string](identity, objc.RegisterName("UTF8String")) != "io.sidelet.mygo-lab" {
		return 0, errors.New("login startup requires the independent Sidelet MyGo Lab app bundle")
	}
	class := objc.ID(objc.GetClass("SMAppService"))
	if class == 0 {
		return 0, errors.New("login startup requires macOS 13 or later")
	}
	service := send(class, "mainAppService")
	if service == 0 {
		return 0, errors.New("main app login service unavailable")
	}
	return service, nil
}

func (macLogin) Status() (string, error) {
	pool := send(objc.ID(objc.GetClass("NSAutoreleasePool")), "new")
	defer send(pool, "drain")
	service, err := labLoginService()
	if err != nil {
		return "unknown", err
	}
	switch int(send(service, "status")) {
	case 0:
		return "notRegistered", nil
	case 1:
		return "enabled", nil
	case 2:
		return "requiresApproval", nil
	case 3:
		return "notFound", nil
	default:
		return "unknown", errors.New("unknown SMAppService status")
	}
}

// Runs on the UI executor, exclusively for this lab's mainAppService. Approval
// pending is a registered item, and is displayed separately from enabled.
func (macLogin) Set(enabled bool) error {
	pool := send(objc.ID(objc.GetClass("NSAutoreleasePool")), "new")
	defer send(pool, "drain")
	service, err := labLoginService()
	if err != nil {
		return err
	}
	status := int(send(service, "status"))
	if enabled && (status == 1 || status == 2) || !enabled && (status == 0 || status == 3) {
		return nil
	}
	selector := "unregisterAndReturnError:"
	if enabled {
		selector = "registerAndReturnError:"
	}
	var nativeError objc.ID
	if !objc.Send[bool](service, objc.RegisterName(selector), uintptr(unsafe.Pointer(&nativeError))) {
		if nativeError != 0 {
			text := send(nativeError, "localizedDescription")
			return errors.New(objc.Send[string](text, objc.RegisterName("UTF8String")))
		}
		return errors.New("SMAppService registration change failed")
	}
	return nil
}

func (macLogin) OpenSettings() error {
	if _, err := labLoginService(); err != nil {
		return err
	}
	send(objc.ID(objc.GetClass("SMAppService")), "openSystemSettingsLoginItems")
	return nil
}
