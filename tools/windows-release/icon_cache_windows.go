//go:build windows

package main

import (
	"runtime"
	"syscall"
	"unsafe"
)

// Replacing the executable at the same path can leave Explorer's taskbar
// context menu showing the previous icon even though WM_SETICON is current.
func refreshShellIcon(path string) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	notify := syscall.NewLazyDLL("shell32.dll").NewProc("SHChangeNotify")
	notify.Call(0x00002000, 0x0005|0x1000, uintptr(unsafe.Pointer(p)), 0) // UPDATEITEM, PATHW | FLUSH
	runtime.KeepAlive(p)
	notify.Call(0x08000000, 0x1000, 0, 0) // ASSOCCHANGED: invalidate cached shell icons.
}
