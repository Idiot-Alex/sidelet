//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"github.com/egoist/mygo"
	"golang.org/x/sys/windows"
)

var (
	clearDWM          = windows.NewLazySystemDLL("dwmapi.dll")
	clearSetAttribute = clearDWM.NewProc("DwmSetWindowAttribute")
	clearExtendFrame  = clearDWM.NewProc("DwmExtendFrameIntoClientArea")
	clearEnableAlpha  = clearDWM.NewProc("DwmEnableBlurBehindWindow")
	clearGDI          = windows.NewLazySystemDLL("gdi32.dll")
	clearCreateRegion = clearGDI.NewProc("CreateRectRgn")
	clearDeleteRegion = clearGDI.NewProc("DeleteObject")
	clearGetStyle     = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowLongPtrW")
)

type clearBlurBehind struct {
	Flags      uint32
	Enable     int32
	Region     uintptr
	Transition int32
}

// Windows 8+ honors alpha through this DWM API without applying a blur.
// A region wholly outside the client area explicitly requests no blurred
// pixels. The native foreground is never subjected to window-wide opacity.
// https://learn.microsoft.com/windows/win32/api/dwmapi/nf-dwmapi-dwmenableblurbehindwindow
func configureClearTransparency(w *mygo.Window, enabled bool) error {
	return configureClearWindow(w.NativeHandle(), enabled)
}

func configureClearWindow(hwnd uintptr, enabled bool) error {
	bb := clearBlurBehind{Flags: 1} // DWM_BB_ENABLE
	if enabled {
		style, _, _ := clearGetStyle.Call(hwnd, ^uintptr(19)) // GWL_EXSTYLE = -20
		if style&0x00200000 == 0 {                            // WS_EX_NOREDIRECTIONBITMAP
			return fmt.Errorf("window has no alpha composition surface")
		}
		backdrop := int32(1) // DWMSBT_NONE, not the SDK's Acrylic backdrop
		hr, _, _ := clearSetAttribute.Call(hwnd, 38, uintptr(unsafe.Pointer(&backdrop)), unsafe.Sizeof(backdrop))
		if int32(hr) < 0 {
			return fmt.Errorf("disable backdrop: HRESULT %#x", uint32(hr))
		}
		margins := [4]int32{}
		hr, _, _ = clearExtendFrame.Call(hwnd, uintptr(unsafe.Pointer(&margins)))
		if int32(hr) < 0 {
			return fmt.Errorf("reset extended frame: HRESULT %#x", uint32(hr))
		}
		region, _, err := clearCreateRegion.Call(^uintptr(0), ^uintptr(0), 0, 0)
		if region == 0 {
			return fmt.Errorf("create clear region: %v", err)
		}
		defer clearDeleteRegion.Call(region)
		bb.Flags, bb.Enable, bb.Region = 3, 1, region // ENABLE | BLURREGION
	}
	hr, _, _ := clearEnableAlpha.Call(hwnd, uintptr(unsafe.Pointer(&bb)))
	if int32(hr) < 0 {
		return fmt.Errorf("configure alpha composition: HRESULT %#x", uint32(hr))
	}
	return nil
}
