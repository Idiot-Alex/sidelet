//go:build darwin

package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// Observe events delivered to this experiment only. The event is returned
// unchanged; this cannot move the cursor or see another app's keyboard input.
func startInputTrace() func() {
	loadPanelAPI()
	block := objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
		w := send(event, "window")
		data := map[string]any{"event": "delivered-mouse", "type": int(send(event, "type")), "windowNumber": int(send(w, "windowNumber")), "locationInWindow": panelAPI.point(event, objc.RegisterName("locationInWindow")), "cursor": panelAPI.point(objc.ID(objc.GetClass("NSEvent")), objc.RegisterName("mouseLocation")), "native": nativeState()}
		if w != 0 {
			data["windowFrame"] = panelAPI.frame(w, objc.RegisterName("frame"))
		}
		b, _ := json.Marshal(data)
		log.Print(string(b))
		return event
	})
	monitor := send(objc.ID(objc.GetClass("NSEvent")), "addLocalMonitorForEventsMatchingMask:handler:", uint64(0x3fe), block)
	return func() {
		if monitor != 0 {
			send(objc.ID(objc.GetClass("NSEvent")), "removeMonitor:", monitor)
		}
		block.Release()
	}
}

// Read-only AppKit diagnostics. No injected OS events or window overrides.
func nativeState() map[string]any {
	workspace := objc.ID(objc.GetClass("NSWorkspace")).Send(objc.RegisterName("sharedWorkspace"))
	front := workspace.Send(objc.RegisterName("frontmostApplication"))
	app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	key := app.Send(objc.RegisterName("keyWindow"))
	responder := key.Send(objc.RegisterName("firstResponder"))
	return map[string]any{"frontmostPID": int(front.Send(objc.RegisterName("processIdentifier"))), "firstResponderTakesText": responder.Send(objc.RegisterName("respondsToSelector:"), uintptr(objc.RegisterName("insertText:replacementRange:"))) != 0}
}

// Window.Focus alone does not select a native content view as AppKit's
// first responder when the edit action arrived through accessibility.
// Use public AppKit methods on this experiment's own window, without
// changing MyGo's classes, injecting input, or touching another app.
func focusEditor(w *mygo.Window) {
	w.Focus()
	window := objc.ID(w.NativeHandle())
	root := window.Send(objc.RegisterName("contentView"))
	if view := findInputView(root); view != 0 {
		window.Send(objc.RegisterName("makeFirstResponder:"), uintptr(view))
	}
}

func findInputView(view objc.ID) objc.ID {
	if view.Send(objc.RegisterName("acceptsFirstResponder")) != 0 {
		return view
	}
	children := view.Send(objc.RegisterName("subviews"))
	for i, n := 0, int(children.Send(objc.RegisterName("count"))); i < n; i++ {
		if child := findInputView(children.Send(objc.RegisterName("objectAtIndex:"), uintptr(i))); child != 0 {
			return child
		}
	}
	return 0
}

func watchCaptures(_ string, capture func()) {
	requests := make(chan os.Signal, 1)
	signal.Notify(requests, syscall.SIGUSR1)
	defer signal.Stop(requests)
	for range requests {
		capture()
	}
}
