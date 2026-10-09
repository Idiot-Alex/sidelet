//go:build darwin

package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

type macQuickFocus struct{ app, key objc.ID }

func captureQuickForeground() quickFocusToken {
	workspace := send(objc.ID(objc.GetClass("NSWorkspace")), "sharedWorkspace")
	app := send(objc.ID(objc.GetClass("NSApplication")), "sharedApplication")
	return &macQuickFocus{send(send(workspace, "frontmostApplication"), "retain"), send(send(app, "keyWindow"), "retain")}
}
func (a *overlayInput) foregroundForQuickAdd() quickFocusToken {
	if a.editing {
		return &macQuickFocus{send(a.previousApp, "retain"), send(a.previousKey, "retain")}
	}
	return captureQuickForeground()
}
func quickForegroundIsOurs() bool {
	workspace := send(objc.ID(objc.GetClass("NSWorkspace")), "sharedWorkspace")
	return int(send(send(workspace, "frontmostApplication"), "processIdentifier")) == os.Getpid()
}
func (f *macQuickFocus) pid() int { return int(send(f.app, "processIdentifier")) }
func (f *macQuickFocus) restore() bool {
	if !quickForegroundIsOurs() || f.app == 0 || send(f.app, "isTerminated") != 0 {
		return false
	}
	if f.pid() == os.Getpid() {
		if f.key == 0 || send(f.key, "isVisible") == 0 {
			return false
		}
		send(f.key, "makeKeyAndOrderFront:", uintptr(0))
		return send(f.key, "isKeyWindow") != 0
	}
	return send(f.app, "activateWithOptions:", uint(2)) != 0
}
func (f *macQuickFocus) release() {
	send(f.app, "release")
	send(f.key, "release")
	f.app, f.key = 0, 0
}

type carbonQuickID struct{ Signature, ID uint32 }
type carbonQuickType struct{ Class, Kind uint32 }

var quickCarbon struct {
	once       sync.Once
	err        error
	target     func() uintptr
	register   func(uint32, uint32, carbonQuickID, uintptr, uint32, *uintptr) int32
	unregister func(uintptr) int32
	install    func(uintptr, uintptr, uint32, *carbonQuickType, uintptr, *uintptr) int32
	remove     func(uintptr) int32
	parameter  func(uintptr, uint32, uint32, uintptr, uintptr, uintptr, unsafe.Pointer) int32
	callback   uintptr
	pressed    func()
}

const quickCarbonSignature = uint32(0x534C4D51) // SLMQ; isolated from MyGo/formal Sidelet handlers.

func nativeQuickHotkeyMatches(status int32, id carbonQuickID) bool {
	return status == 0 && id.Signature == quickCarbonSignature && id.ID == 1
}

// MyGo v0.2.16 uses non-exclusive Carbon registration. Match the formal app's
// exclusive ownership instead, so launching two instances cannot share a key.
func registerNativeQuickAddShortcut(pressed func()) (func(), error) {
	return registerCarbonQuickShortcut(0x31, (1<<12)|(1<<9), pressed)
}

// Independent diagnostic key when the normal shortcut is already occupied.
// It exercises the same exclusive handler; it does not rebind the user key.
func registerNativeQuickAddDiagnosticShortcut(pressed func()) (func(), error) {
	return registerCarbonQuickShortcut(0x50, (1<<12)|(1<<11)|(1<<9), pressed)
}

func registerCarbonQuickShortcut(code, modifiers uint32, pressed func()) (func(), error) {
	quickCarbon.once.Do(func() {
		lib, err := purego.Dlopen("/System/Library/Frameworks/Carbon.framework/Carbon", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			quickCarbon.err = err
			return
		}
		purego.RegisterLibFunc(&quickCarbon.target, lib, "GetApplicationEventTarget")
		purego.RegisterLibFunc(&quickCarbon.register, lib, "RegisterEventHotKey")
		purego.RegisterLibFunc(&quickCarbon.unregister, lib, "UnregisterEventHotKey")
		purego.RegisterLibFunc(&quickCarbon.install, lib, "InstallEventHandler")
		purego.RegisterLibFunc(&quickCarbon.remove, lib, "RemoveEventHandler")
		purego.RegisterLibFunc(&quickCarbon.parameter, lib, "GetEventParameter")
		quickCarbon.callback = purego.NewCallback(func(_, event, _ uintptr) uintptr {
			var id carbonQuickID
			status := quickCarbon.parameter(event, 0x2d2d2d2d, 0x686b6964, 0, unsafe.Sizeof(id), 0, unsafe.Pointer(&id))
			if !nativeQuickHotkeyMatches(status, id) || quickCarbon.pressed == nil {
				return uintptr(uint32(0xffffd96e))
			} // eventNotHandledErr (-9874).
			quickCarbon.pressed()
			return 0
		})
	})
	if quickCarbon.err != nil {
		return nil, quickCarbon.err
	}
	if quickCarbon.pressed != nil {
		return nil, fmt.Errorf("quick-add shortcut already has an owner")
	}
	var handler, ref uintptr
	stop := func() {
		quickCarbon.pressed = nil
		if ref != 0 {
			if status := quickCarbon.unregister(ref); status != 0 {
				log.Printf("quick-add unregister OSStatus=%d", status)
			}
			ref = 0
		}
		if handler != 0 {
			quickCarbon.remove(handler)
			handler = 0
		}
	}
	typ := carbonQuickType{0x6b657962, 5}
	status := quickCarbon.install(quickCarbon.target(), quickCarbon.callback, 1, &typ, 0, &handler)
	if status == 0 {
		status = quickCarbon.register(code, modifiers, carbonQuickID{quickCarbonSignature, 1}, quickCarbon.target(), 1, &ref)
	}
	if status != 0 {
		stop()
		return nil, fmt.Errorf("exclusive quick-add registration keyCode=%d modifiers=%d OSStatus=%d", code, modifiers, status)
	}
	quickCarbon.pressed = pressed
	return stop, nil
}
