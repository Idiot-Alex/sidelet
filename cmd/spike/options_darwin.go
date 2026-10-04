package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"log"
	"sidelet/internal/platform"
)

func overlayOptions(name, url string) application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Name: name, Title: "Sidelet " + name, URL: url, Width: 312, Height: 600,
		Hidden: true, Frameless: true, DisableResize: true, AlwaysOnTop: true,
		BackgroundType: application.BackgroundTypeTransparent, BackgroundColour: application.NewRGBA(0, 0, 0, 0), DefaultContextMenuDisabled: true,
		Mac: application.MacWindow{WindowClass: application.MacWindowClassPanel, CornerType: application.MacWindowCornerTypeSquare, DisableShadow: true, WindowLevel: application.MacWindowLevelFloating, CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces | application.MacWindowCollectionBehaviorFullScreenAuxiliary | application.MacWindowCollectionBehaviorIgnoresCycle, PanelPreferences: application.MacPanelPreferences{NonActivating: true, FloatingPanel: true, BecomesKeyOnlyIfNeeded: true}},
	}
}
func shortcutLabel() string { return "Ctrl+Option+T" }
func registerShortcut(c *controller, w *platform.Window) error {
	if err := w.RegisterKeyboardShortcut(); err != nil {
		return err
	}
	log.Printf("global-shortcut registered shortcut=%s exclusive=true", shortcutLabel())
	return nil
}
func logNativeWindow(o *overlay) {
	log.Printf("native window=%s %s", o.window.Name(), o.native.Diagnostic())
}
