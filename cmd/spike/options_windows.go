package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"sidelet/internal/platform"
)

func overlayOptions(name, url string) application.WebviewWindowOptions {
	return application.WebviewWindowOptions{Name: name, Title: "Sidelet " + name, URL: url, Width: 312, Height: 600, Hidden: true, Frameless: true, DisableResize: true, AlwaysOnTop: true, BackgroundType: application.BackgroundTypeTransparent, BackgroundColour: application.NewRGBA(0, 0, 0, 0), DefaultContextMenuDisabled: true, Windows: application.WindowsWindow{ExStyle: platform.PassiveExtendedStyle(), DisableFramelessWindowDecorations: true, BackdropType: application.None}}
}
func shortcutLabel() string                                    { return "Ctrl+Alt+T" }
func registerShortcut(c *controller, w *platform.Window) error { return w.RegisterKeyboardShortcut() }
func logNativeWindow(o *overlay)                               {}
