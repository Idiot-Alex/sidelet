//go:build windows || darwin

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"log"
	"runtime"
	"sidelet/internal/platform"
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
	if runtime.GOOS == "darwin" {
		log.Printf("control-viewport restored=true success=%t", platform.SetControlRendering(c.control.NativeWindow(), true))
		c.controlRevision++
		c.controlPreparing = true
		c.control.EmitEvent("control:visibility", map[string]any{"visible": true, "revision": c.controlRevision})
		return
	}
	c.showPreparedControl()
}

func (c *controller) showPreparedControl() {
	c.control.UnMinimise()
	c.control.Show()
	c.control.Focus()
}

func (c *controller) hideControl() {
	c.showControl = false
	c.controlPreparing = false
	c.controlRevision++
	if c.control != nil {
		if runtime.GOOS == "darwin" {
			c.control.EmitEvent("control:visibility", map[string]any{"visible": false, "revision": c.controlRevision})
		}
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
