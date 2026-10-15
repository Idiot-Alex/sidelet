//go:build darwin

package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

type nsPoint struct{ X, Y float64 }
type nsSize struct{ W, H float64 }
type nsRect struct {
	Origin nsPoint
	Size   nsSize
}

var panelAPI struct {
	once       sync.Once
	panelClass objc.Class
	viewClass  objc.Class
	initWindow func(objc.ID, objc.SEL, nsRect, uint, uint, bool) objc.ID
	initView   func(objc.ID, objc.SEL, nsRect) objc.ID
	tracking   func(objc.ID, objc.SEL, nsRect, uint, objc.ID, objc.ID) objc.ID
	setFrame   func(objc.ID, objc.SEL, nsRect, bool)
	frame      func(objc.ID, objc.SEL) nsRect
	point      func(objc.ID, objc.SEL) nsPoint
	convert    func(objc.ID, objc.SEL, nsPoint) nsPoint
	color      func(objc.ID, objc.SEL, float64, float64, float64, float64) objc.ID
	setFloat   func(objc.ID, objc.SEL, float64)
	getFloat   func(objc.ID, objc.SEL) float64
	mouseEvent func(objc.ID, objc.SEL, uint, nsPoint, uint, float64, int, objc.ID, int, int, float32) objc.ID
	hitWindow  func(objc.ID, objc.SEL, nsPoint, int) int
}

// All objects and this registry are used only on AppKit's main thread.
var inputViews = map[objc.ID]*inputPanel{}

func send(obj objc.ID, name string, args ...any) objc.ID {
	return obj.Send(objc.RegisterName(name), args...)
}

func loadPanelAPI() {
	panelAPI.once.Do(func() {
		lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			panic(err)
		}
		msg, err := purego.Dlsym(lib, "objc_msgSend")
		if err != nil {
			panic(err)
		}
		stret := msg
		if runtime.GOARCH == "amd64" {
			stret, err = purego.Dlsym(lib, "objc_msgSend_stret")
			if err != nil {
				panic(err)
			}
		}
		purego.RegisterFunc(&panelAPI.initWindow, msg)
		purego.RegisterFunc(&panelAPI.initView, msg)
		purego.RegisterFunc(&panelAPI.tracking, msg)
		purego.RegisterFunc(&panelAPI.setFrame, msg)
		purego.RegisterFunc(&panelAPI.frame, stret)
		purego.RegisterFunc(&panelAPI.point, msg)
		purego.RegisterFunc(&panelAPI.convert, msg)
		purego.RegisterFunc(&panelAPI.color, msg)
		purego.RegisterFunc(&panelAPI.setFloat, msg)
		purego.RegisterFunc(&panelAPI.getFloat, msg)
		purego.RegisterFunc(&panelAPI.mouseEvent, msg)
		purego.RegisterFunc(&panelAPI.hitWindow, msg)
		panelAPI.panelClass, err = objc.RegisterClass("SideletLabInputPanel", objc.GetClass("NSPanel"), nil, nil, []objc.MethodDef{
			{Cmd: objc.RegisterName("canBecomeKeyWindow"), Fn: func(objc.ID, objc.SEL) bool { return false }},
			{Cmd: objc.RegisterName("canBecomeMainWindow"), Fn: func(objc.ID, objc.SEL) bool { return false }},
			{Cmd: objc.RegisterName("isAccessibilityElement"), Fn: func(objc.ID, objc.SEL) bool { return false }},
		})
		if err != nil {
			panic(err)
		}
		methods := []objc.MethodDef{
			{Cmd: objc.RegisterName("acceptsFirstMouse:"), Fn: func(objc.ID, objc.SEL, objc.ID) bool { return true }},
			{Cmd: objc.RegisterName("isAccessibilityElement"), Fn: func(objc.ID, objc.SEL) bool { return false }},
		}
		for _, name := range []string{"mouseEntered:", "mouseExited:", "mouseMoved:", "mouseDown:", "mouseUp:", "mouseDragged:", "rightMouseDown:", "rightMouseUp:", "rightMouseDragged:"} {
			name := name
			methods = append(methods, objc.MethodDef{Cmd: objc.RegisterName(name), Fn: func(view objc.ID, _ objc.SEL, event objc.ID) {
				if p := inputViews[view]; p != nil {
					p.forward(name, event)
				}
			}})
		}
		panelAPI.viewClass, err = objc.RegisterClass("SideletLabInputView", objc.GetClass("NSView"), nil, nil, methods)
		if err != nil {
			panic(err)
		}
	})
}

type overlayInput struct {
	m                        *model
	stack, card              *mygo.Window
	panels                   map[string]*inputPanel
	previousApp, previousKey objc.ID
	editing                  bool
	forwarded, entered       int
	onEvent                  func(string)
	pointerPresence          func(bool, bool)
	directFixture            bool
	moveDrag                 func(mygo.Point)
	dragMonitor              objc.ID
	dragBlock                objc.Block
	fallbackWindow           func() *mygo.Window
}

type inputPanel struct {
	owner               *overlayInput
	window              *mygo.Window
	panel, view, target objc.ID
	region              inputRegion
	visible             bool
	frame               nsRect
	tracking            objc.ID
}

func newOverlayInput(m *model) *overlayInput {
	return &overlayInput{m: m, panels: map[string]*inputPanel{}}
}

func (a *overlayInput) bindStack(w *mygo.Window) {
	loadPanelAPI()
	a.stack = w
	w.SetIgnoreMouseEvents(!a.directFixture)
	if a.directFixture {
		// Computer Use delivers directly to the render window. Observe only
		// this window's received drag/release; never synthesize OS input.
		a.dragBlock = objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
			if send(event, "window") == objc.ID(w.NativeHandle()) && a.m.Dragging {
				location := panelAPI.point(event, objc.RegisterName("locationInWindow"))
				a.moveDeliveredDrag(location)
				if send(event, "type") == 6 {
					return 0
				}
			}
			return event
		})
		a.dragMonitor = send(objc.ID(objc.GetClass("NSEvent")), "addLocalMonitorForEventsMatchingMask:handler:", uint64(1<<6|1<<2), a.dragBlock)
	}
}

func (*overlayInput) handlesNativeDrag() bool { return true }

func (a *overlayInput) moveDeliveredDrag(location nsPoint) {
	if a.moveDrag != nil && a.m.Dragging {
		a.moveDrag(nativeDragScreenPoint(a.m, location.X, location.Y))
	}
}

func (a *overlayInput) bindCard(w *mygo.Window) { a.card = w }

func (a *overlayInput) sync() {
	if a.stack == nil {
		return
	}
	if a.directFixture {
		if a.card != nil {
			a.card.SetIgnoreMouseEvents(false)
		}
		return
	}
	wanted := map[string]bool{}
	if a.stack.IsVisible() {
		for _, r := range stackInputRegions(a.m) {
			wanted[r.key] = true
			a.install(a.stack, r)
		}
	}
	if a.card != nil {
		a.card.SetIgnoreMouseEvents(!a.m.Editing)
		if a.card.IsVisible() && a.m.cardOpen() && !a.m.Editing {
			bounds := a.card.Bounds()
			r := inputRegion{key: "card", task: a.m.Opened, rect: mygo.Rectangle{Width: bounds.Width, Height: bounds.Height}}
			wanted[r.key] = true
			a.install(a.card, r)
		}
	}
	for key, p := range a.panels {
		if !wanted[key] && p.visible {
			p.visible = false
			send(p.panel, "orderOut:", uintptr(0))
		}
	}
}

func (a *overlayInput) install(w *mygo.Window, r inputRegion) {
	window := objc.ID(w.NativeHandle())
	// Use the parent's AppKit frame, rather than assuming the primary screen
	// starts at (0,0) or duplicating MyGo's flipped screen-coordinate conversion.
	parent := panelAPI.frame(window, objc.RegisterName("frame"))
	frame := nsRect{nsPoint{parent.Origin.X + float64(r.rect.X), parent.Origin.Y + parent.Size.H - float64(r.rect.Y+r.rect.Height)}, nsSize{float64(r.rect.Width), float64(r.rect.Height)}}
	p := a.panels[r.key]
	if p == nil {
		panel := panelAPI.initWindow(send(objc.ID(panelAPI.panelClass), "alloc"), objc.RegisterName("initWithContentRect:styleMask:backing:defer:"), frame, 1<<7, 2, false)
		for _, name := range []string{"setReleasedWhenClosed:", "setOpaque:", "setHasShadow:", "setHidesOnDeactivate:"} {
			send(panel, name, false)
		}
		for _, name := range []string{"setFloatingPanel:", "setAcceptsMouseMovedEvents:", "setExcludedFromWindowsMenu:"} {
			send(panel, name, true)
		}
		send(panel, "setBackgroundColor:", send(objc.ID(objc.GetClass("NSColor")), "clearColor"))
		view := panelAPI.initView(send(objc.ID(panelAPI.viewClass), "alloc"), objc.RegisterName("initWithFrame:"), nsRect{Size: frame.Size})
		send(view, "setAutoresizingMask:", uint(2|16))
		send(view, "setWantsLayer:", true)
		send(panel, "setContentView:", view)
		send(view, "release") // The panel owns its content view.
		tracking := panelAPI.tracking(send(objc.ID(objc.GetClass("NSTrackingArea")), "alloc"), objc.RegisterName("initWithRect:options:owner:userInfo:"), nsRect{}, 0x01|0x02|0x80|0x200, view, 0)
		send(view, "addTrackingArea:", tracking)
		send(tracking, "release")
		p = &inputPanel{owner: a, window: w, panel: panel, view: view, tracking: tracking, target: findInputView(send(window, "contentView"))}
		if p.target == 0 {
			panic("MyGo native content has no input view")
		}
		a.panels[r.key], inputViews[view] = p, p
		send(window, "addChildWindow:ordered:", panel, int(-1))
	}
	p.region = r
	if p.frame != frame {
		panelAPI.setFrame(p.panel, objc.RegisterName("setFrame:display:"), frame, true)
		p.frame = frame
	}
	// This backing lies below the identical MyGo paint and gives WindowServer
	// an actual hit region. Rounded corners remain transparent in both layers.
	color := uiFaceColor()
	if r.task >= 0 && r.task < len(a.m.Tasks) {
		color = priorityColor(a.m.Tasks[r.task].Priority)
	}
	if a.m.UITheme != "" {
		color = webTheme(a.m.UITheme).Alt
	}
	radius := 5.0
	if !r.marker {
		color, radius = uiFaceColor(), 8
		if a.m.UITheme != "" {
			color = webTheme(a.m.UITheme).Surface
		}
	}
	if r.key == "card" {
		radius = 12
	}
	nativeColor := panelAPI.color(objc.ID(objc.GetClass("NSColor")), objc.RegisterName("colorWithDeviceRed:green:blue:alpha:"), float64(color.R)/255, float64(color.G)/255, float64(color.B)/255, 1)
	layer := send(p.view, "layer")
	send(layer, "setBackgroundColor:", send(nativeColor, "CGColor"))
	panelAPI.setFloat(layer, objc.RegisterName("setCornerRadius:"), radius)
	send(layer, "setMasksToBounds:", true)
	send(p.panel, "setLevel:", send(window, "level"))
	if !p.visible {
		p.visible = true
		send(p.panel, "orderWindow:relativeTo:", int(-1), send(window, "windowNumber"))
	}
}

func (p *inputPanel) forward(name string, event objc.ID) {
	if !p.window.IsVisible() {
		return
	}
	targetWindow := objc.ID(p.window.NativeHandle())
	location := panelAPI.point(event, objc.RegisterName("locationInWindow"))
	screen := panelAPI.convert(p.panel, objc.RegisterName("convertPointToScreen:"), location)
	typ := uint(send(event, "type"))
	if p.window == p.owner.stack && p.owner.m.Dragging && (typ == 6 || typ == 2) {
		local := panelAPI.convert(targetWindow, objc.RegisterName("convertPointFromScreen:"), screen)
		p.owner.moveDeliveredDrag(local)
		if typ == 6 {
			p.owner.forwarded++
			return
		}
	}
	clicks := 0
	tracking := name == "mouseEntered:" || name == "mouseExited:"
	originalName := name
	if tracking {
		// Enter/exit locations can refer to the tracking area's old frame during
		// resize. The current real cursor also keeps adjoining regions continuous.
		screen = panelAPI.point(objc.ID(objc.GetClass("NSEvent")), objc.RegisterName("mouseLocation"))
		typ = 5 // Convert the delivered tracking event to a surface pointer move.
	} else {
		clicks = int(send(event, "clickCount"))
	}
	if p.owner.pointerPresence != nil {
		inside := func(w *mygo.Window, r mygo.Rectangle) bool {
			if w == nil || !w.IsVisible() {
				return false
			}
			f := panelAPI.frame(objc.ID(w.NativeHandle()), objc.RegisterName("frame"))
			x, y := screen.X-f.Origin.X, f.Size.H-(screen.Y-f.Origin.Y)
			return x >= float64(r.X) && x < float64(r.X+r.Width) && y >= float64(r.Y) && y < float64(r.Y+r.Height)
		}
		source, card := false, false
		if i := p.owner.m.Opened; i >= 0 {
			r := p.owner.m.previewRect(i)
			marker := p.owner.m.markerRect(i)
			source = inside(p.owner.stack, r) || inside(p.owner.stack, marker)
		} else if p.owner.m.OverflowOpen {
			source = inside(p.owner.stack, p.owner.m.overflowRect())
		}
		if p.owner.card != nil {
			b := p.owner.card.Bounds()
			card = inside(p.owner.card, mygo.Rectangle{Width: b.Width, Height: b.Height})
		}
		p.owner.pointerPresence(source, card)
	}
	location = panelAPI.convert(targetWindow, objc.RegisterName("convertPointFromScreen:"), screen)
	converted := panelAPI.mouseEvent(objc.ID(objc.GetClass("NSEvent")), objc.RegisterName("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"), typ, location, uint(send(event, "modifierFlags")), panelAPI.getFloat(event, objc.RegisterName("timestamp")), int(send(targetWindow, "windowNumber")), 0, 0, clicks, 1)
	if converted == 0 {
		panic(fmt.Sprintf("cannot retarget delivered %s", name))
	}
	if tracking {
		name = "mouseMoved:"
	}
	// Bypass NSWindow.sendEvent's activation logic. The genuine delivered
	// event enters MyGo's existing surface, layout, buttons and drag capture.
	send(p.target, name, converted)
	p.owner.forwarded++
	if tracking {
		p.owner.entered++
	}
	if p.owner.onEvent != nil && (tracking || typ == 1 || typ == 2) {
		p.owner.onEvent("native-" + originalName)
	}
}

func (a *overlayInput) beginEdit(w *mygo.Window) {
	if !a.editing {
		workspace := send(objc.ID(objc.GetClass("NSWorkspace")), "sharedWorkspace")
		a.previousApp = send(send(workspace, "frontmostApplication"), "retain")
		app := send(objc.ID(objc.GetClass("NSApplication")), "sharedApplication")
		a.previousKey = send(send(app, "keyWindow"), "retain")
		a.editing = true
	}
	a.sync()
	focusEditor(w)
}

func (a *overlayInput) endEdit() {
	if !a.editing {
		return
	}
	a.editing = false
	workspace := send(objc.ID(objc.GetClass("NSWorkspace")), "sharedWorkspace")
	front := send(workspace, "frontmostApplication")
	if int(send(front, "processIdentifier")) == os.Getpid() {
		if a.card != nil && a.card.IsVisible() {
			// resignKeyWindow is a notification hook, not a complete AppKit
			// focus transition. Calling it alone can leave NSApp.keyWindow
			// pointing at an inactive card, so the next edit cannot become key.
			// Order out and show inactive in the same run-loop pass instead.
			a.card.Hide()
			a.card.ShowInactive()
		}
		if int(send(a.previousApp, "processIdentifier")) == os.Getpid() {
			// A menu/accessibility client can explicitly raise a render window.
			// Never restore that window as key after returning to passive mode.
			overlayKey := a.previousKey == objc.ID(a.stack.NativeHandle()) || (a.card != nil && a.previousKey == objc.ID(a.card.NativeHandle()))
			if !overlayKey && send(a.previousKey, "isVisible") != 0 {
				send(a.previousKey, "makeKeyAndOrderFront:", uintptr(0))
			} else if a.fallbackWindow != nil {
				if w := a.fallbackWindow(); w != nil && w.IsVisible() {
					w.Focus()
				}
			}
		} else if a.previousApp != 0 && send(a.previousApp, "isTerminated") == 0 {
			send(a.previousApp, "activateWithOptions:", uint(2))
		}
	}
	send(a.previousApp, "release")
	send(a.previousKey, "release")
	a.previousApp, a.previousKey = 0, 0
}

func (a *overlayInput) state() map[string]any {
	if a.directFixture {
		return map[string]any{"adapter": "CUA-direct-render-fixture", "systemOverlayRoutingVerified": false, "editorActive": a.editing, "returnAppPID": int(send(a.previousApp, "processIdentifier"))}
	}
	visible := 0
	regions := map[string]any{}
	for _, p := range a.panels {
		if p.visible {
			visible++
		}
		center := nsPoint{p.frame.Origin.X + p.frame.Size.W/2, p.frame.Origin.Y + p.frame.Size.H/2}
		regions[p.region.key] = map[string]any{"frame": p.frame, "visible": send(p.panel, "isVisible") != 0, "level": int(send(p.panel, "level")), "windowNumber": int(send(p.panel, "windowNumber")), "hitWindowNumber": panelAPI.hitWindow(objc.ID(objc.GetClass("NSWindow")), objc.RegisterName("windowNumberAtPoint:belowWindowWithWindowNumber:"), center, 0), "ignoresMouse": send(p.panel, "ignoresMouseEvents") != 0, "canBecomeKey": send(p.panel, "canBecomeKeyWindow") != 0}
	}
	state := map[string]any{"adapter": "nonactivating-input-panels", "visiblePanels": visible, "allocatedPanels": len(a.panels), "forwardedEvents": a.forwarded, "trackingEvents": a.entered, "regions": regions}
	if a.stack != nil {
		state["cursor"] = panelAPI.point(objc.ID(objc.GetClass("NSEvent")), objc.RegisterName("mouseLocation"))
		state["stackFrame"] = panelAPI.frame(objc.ID(a.stack.NativeHandle()), objc.RegisterName("frame"))
	}
	return state
}

func (a *overlayInput) close() {
	if a.dragMonitor != 0 {
		send(objc.ID(objc.GetClass("NSEvent")), "removeMonitor:", a.dragMonitor)
		a.dragMonitor = 0
		a.dragBlock.Release()
	}
	for _, p := range a.panels {
		delete(inputViews, p.view)
		send(objc.ID(p.window.NativeHandle()), "removeChildWindow:", p.panel)
		send(p.panel, "orderOut:", uintptr(0))
		send(p.panel, "close")
		send(p.panel, "release")
	}
	a.panels = map[string]*inputPanel{}
	send(a.previousApp, "release")
	send(a.previousKey, "release")
	a.previousApp, a.previousKey = 0, 0
}
