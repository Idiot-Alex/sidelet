//go:build windows

package main

import (
	"golang.org/x/sys/windows"
	"runtime"
	"testing"
	"unsafe"
)

func TestTaskbarRelaunchProperties(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	class, _ := windows.UTF16PtrFromString("STATIC")
	hwnd, _, err := inputUser32.NewProc("CreateWindowExW").Call(0x80, uintptr(unsafe.Pointer(class)), 0, 0x80000000, 20, 20, 56, 56, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal(err)
	}
	defer inputUser32.NewProc("DestroyWindow").Call(hwnd)
	command := `"D:\应用目录\Sidelet.exe" -main`
	icon := `D:\应用目录\icons\note-test.ico,0`
	if err := setTaskbarProperties(hwnd, "Sidelet.Test", command, icon); err != nil {
		t.Fatal(err)
	}
	iid, _ := windows.GUIDFromString("{886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99}")
	format, _ := windows.GUIDFromString("{9F4C2855-9F79-4B39-A8D0-E1D42DE1D5F3}")
	var store *taskbarStore
	hr, _, _ := windows.NewLazySystemDLL("shell32.dll").NewProc("SHGetPropertyStoreForWindow").Call(hwnd, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&store)))
	if int32(hr) < 0 {
		t.Fatal("cannot read taskbar properties")
	}
	defer taskbarStoreCall(store, 2)
	for id, want := range map[uint32]string{2: command, 3: icon, 4: "Sidelet", 5: "Sidelet.Test"} {
		key := taskbarPropertyKey{format, id}
		var value taskbarVariant
		if hr := taskbarStoreCall(store, 5, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&value))); int32(hr) < 0 {
			t.Fatal("GetValue failed")
		}
		var text *uint16
		hr, _, _ = windows.NewLazySystemDLL("propsys.dll").NewProc("PropVariantToStringAlloc").Call(uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&text)))
		if int32(hr) < 0 {
			t.Fatal("string property missing")
		}
		got := windows.UTF16PtrToString(text)
		windows.NewLazySystemDLL("ole32.dll").NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(text)))
		windows.NewLazySystemDLL("ole32.dll").NewProc("PropVariantClear").Call(uintptr(unsafe.Pointer(&value)))
		if got != want {
			t.Fatalf("property %d = %q, want %q", id, got, want)
		}
	}
}
