//go:build windows

package platform

import (
	"errors"
	"unsafe"
)

// The Dock preference is macOS-only; Windows keeps its taskbar behaviour.
func SetDockVisible(bool, unsafe.Pointer) error { return nil }

func LoginStatus() string { return "unsupported" }
func SetLogin(bool) error { return errors.New("当前版本仅支持 macOS 登录启动。") }
func OpenSystemSettings(bool) error {
	return errors.New("当前版本仅支持打开 macOS 系统设置。")
}
