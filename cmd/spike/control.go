//go:build windows || darwin

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"unsafe"
)

// The task/settings WebView is created only for a foreground entry. Background
// layout, reminders and preferences must work before this window exists.
func (c *controller) ensureControl() {
	if c.control != nil {
		return
	}
	c.control = c.app.Window.NewWithOptions(c.controlOptions)
	window := c.control
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		c.post(message{Type: "hide-control", Window: window})
	})
}

func (c *controller) openControl(settingsOpen *bool) {
	c.showControl = true
	if settingsOpen != nil {
		c.controlSettings = settingsOpen
	}
	c.ensureControl()
	c.presentControl()
}

func (c *controller) presentControl() {
	if !c.controlReady || !c.showControl {
		return
	}
	if c.controlSettings != nil {
		c.control.EmitEvent("settings:open", *c.controlSettings)
		c.controlSettings = nil
	}
	c.control.UnMinimise()
	c.control.Show()
	c.control.Focus()
}

func (c *controller) hideControl() {
	c.showControl = false
	if c.control != nil {
		c.control.Hide()
	}
}

func (c *controller) emitControl(name string, data ...any) {
	if c.control != nil {
		c.control.EmitEvent(name, data...)
	}
}

func (c *controller) controlNative() unsafe.Pointer {
	if c.control == nil {
		return nil
	}
	return c.control.NativeWindow()
}
