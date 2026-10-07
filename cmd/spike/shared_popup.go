//go:build windows || darwin

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func (c *controller) ensureSharedPopup() {
	if c.quick.window != nil {
		return
	}
	c.quick.window = c.app.Window.NewWithOptions(c.quickOptions)
	c.add = &overlay{window: c.quick.window, mode: "Passive"}
	c.quick.window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		c.post(message{Type: "popup-close", Window: c.quick.window, PopupView: c.popup.View, PopupRevision: c.popup.Revision})
	})
}

func (c *controller) preparePopup(view string, sessionRevision uint64) {
	revision := c.popup.Prepare(view)
	if view == "add" {
		c.quick.native.ConfigureQuickAdd()
	} else {
		c.quick.native.ConfigureQuickCard()
	}
	c.quick.window.EmitEvent("popup:prepare", map[string]any{"view": view, "revision": revision, "sessionRevision": sessionRevision})
}

func (c *controller) acceptPopupPacket(m message) bool {
	if !c.sharedPopup || c.quick.window == nil || m.Window != c.quick.window || m.Type == "ready" || m.Type == "memory-view" || (m.Type == "pointer" && m.nativePointer) {
		return true
	}
	return c.popup.Accept(m.PopupView, m.PopupRevision)
}
