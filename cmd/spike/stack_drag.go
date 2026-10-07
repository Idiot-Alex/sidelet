//go:build windows || darwin

package main

import (
	"errors"
	"log"
	"math"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"sidelet/internal/platform"
	"sidelet/internal/todo"
)

type stackDrag struct {
	revision       uint64
	overlay        *overlay
	before         todo.EdgeStack
	display        platform.Display
	anchor         platform.Rect // task body in CSS coordinates, excluding its handle
	viewportTop    float64       // frozen cropped viewport origin in the WorkArea
	arranging      bool
	transientInput bool
	previous       platform.FocusToken
}

func finite(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

// Screen deltas are in the original display's CSS coordinate system. The
// display and body geometry stay frozen for the entire gesture; moving the
// native window must not change the drag's coordinate origin.
func (d *stackDrag) target(dx, dy float64) (todo.EdgeStack, error) {
	a, scale := d.display.WorkArea, d.display.Scale
	if !finite(dx, dy, scale, a.Width, a.Height, d.anchor.X, d.anchor.Y, d.anchor.Width, d.anchor.Height, d.viewportTop) || scale <= 0 || a.Width <= 0 || a.Height <= 0 || d.anchor.Width <= 0 || d.anchor.Height < 0 {
		return todo.EdgeStack{}, errors.New("invalid drag geometry")
	}
	w := math.Min(312*scale, a.Width)
	x := d.anchor.X*scale + d.anchor.Width*scale/2
	if d.before.Side == "right" {
		x += a.Width - w
	}
	next := d.before
	next.Side = "left"
	if x+dx*scale >= a.Width/2 {
		next.Side = "right"
	}
	half := d.anchor.Height * scale / 2
	low, high := 10*scale+half, a.Height-10*scale-half
	if d.arranging {
		// Only arrange mode needs room for its handle and toolbar.
		low, high = (10+42)*scale+half, a.Height-100*scale-half
	}
	if low > high {
		return todo.EdgeStack{}, errors.New("work area is too short for dragging")
	}
	center := (d.viewportTop + d.anchor.Y + d.anchor.Height/2 + dy) * scale
	next.Offset = math.Max(low, math.Min(high, center)) / a.Height
	return next, nil
}

func (c *controller) beginStackDrag(m message) error {
	c.cancelStackDrag()
	c.refreshFullscreen()
	o := c.find(m.Window)
	if c.store == nil || c.quiet || c.fullscreen || c.addSession.Open || c.addSession.Saving || c.quick.mode == "Editing" || o == nil || o == c.quick || o.native == nil || m.Revision == 0 {
		return errors.New("stack dragging is unavailable")
	}
	var before todo.EdgeStack
	for _, saved := range c.snapshot.Stacks {
		if saved.ID == o.stackID {
			before = saved
		}
	}
	displays, err := platform.Displays()
	if err != nil {
		return err
	}
	for _, display := range displays {
		if display.ID != before.DisplayID {
			continue
		}
		d := &stackDrag{revision: m.Revision, overlay: o, before: before, display: display, anchor: m.Anchor, viewportTop: o.viewportTop, arranging: c.arranging}
		if _, err := d.target(0, 0); err != nil {
			return err
		}
		if !c.arranging {
			previous := platform.CaptureForeground()
			if c.interactionActive() {
				previous = c.previous
			}
			if c.quickSession.Open || c.quickPending != nil {
				c.hideQuick(false)
			}
			if o.mode == "Passive" {
				// A deliberate drag may take focus for Esc, without entering the
				// task keyboard mode or moving DOM focus off the captured handle.
				c.exitModes(false)
				d.previous = previous
				if err := o.native.Activate(); err != nil {
					_ = o.native.Passive()
					previous.Restore()
					return err
				}
				o.window.Focus()
				d.transientInput = true
			}
		}
		c.drag = d
		log.Printf("stack drag started stack=%d revision=%d", before.ID, m.Revision)
		return nil
	}
	return errors.New("stack display is unavailable")
}

func (c *controller) dragTarget(m message) (todo.EdgeStack, error) {
	d := c.drag
	if d == nil || d.arranging != c.arranging || d.revision != m.Revision || d.overlay.window != m.Window || c.quiet || c.fullscreen {
		return todo.EdgeStack{}, errors.New("stack drag was cancelled")
	}
	next, err := d.target(m.X, m.Y)
	if err != nil {
		return next, err
	}
	found := false
	for _, saved := range c.snapshot.Stacks {
		if saved.ID == d.before.ID {
			found = true
			if saved != d.before {
				return next, errors.New("stack layout changed during drag")
			}
		}
		if saved.ID != next.ID && saved.DisplayID == next.DisplayID && saved.Side == next.Side {
			return next, errors.New("target edge is occupied")
		}
	}
	if !found {
		return next, errors.New("stack layout changed during drag")
	}
	return next, nil
}

func (c *controller) previewStackDrag(m message) error {
	next, err := c.dragTarget(m)
	if err != nil {
		return nil
	} // An invalid target is rejected on drop, without log/event flooding.
	d := c.drag
	d.overlay.side, d.overlay.offset = next.Side, next.Offset
	if err = c.placeStack(d.overlay, d.display); err != nil {
		c.cancelStackDrag()
		return err
	}
	return nil
}

func (c *controller) cancelStackDrag() {
	c.cancelStackDragWithRestore(false)
}

func (c *controller) finishStackDragInput(d *stackDrag, restore bool) {
	if !d.transientInput {
		return
	}
	if err := d.overlay.native.Passive(); err != nil {
		log.Print(err)
	}
	if restore && !c.quiet && !c.fullscreen && platform.ForegroundApplicationIsOurs() {
		d.previous.Restore()
	}
}

func (c *controller) cancelStackDragWithRestore(restore bool) {
	d := c.drag
	if d == nil {
		return
	}
	c.drag = nil
	for _, saved := range c.snapshot.Stacks {
		if saved.ID == d.overlay.stackID {
			d.overlay.side, d.overlay.offset = saved.Side, saved.Offset
		}
	}
	if err := c.placeStack(d.overlay, d.display); err != nil {
		log.Print(err)
	}
	d.overlay.window.EmitEvent("stack:drag-cancelled", d.revision)
	c.finishStackDragInput(d, restore)
	log.Printf("stack drag cancelled stack=%d revision=%d", d.before.ID, d.revision)
}

// Like task writes, the SQLite transaction runs off the UI thread. The queue
// serializes drop with cancellation, display changes, and other layout writes.
func (c *controller) finishStackDrag(m message) {
	var next todo.EdgeStack
	var d *stackDrag
	var err error
	application.InvokeSync(func() {
		c.refreshFullscreen()
		next, err = c.dragTarget(m)
		d = c.drag
	})
	var state todo.Snapshot
	if err == nil {
		state, err = c.store.MoveStack(d.before, next, time.Now())
	}
	application.InvokeSync(func() {
		if err != nil {
			if d != nil && d.revision == m.Revision && d.overlay.window == m.Window {
				c.cancelStackDragWithRestore(true)
			}
			c.acknowledge(m, err)
			log.Printf("stack drag failed: %v", err)
			return
		}
		c.drag = nil
		c.acceptPersistentState(state)
		// acceptPersistentState may see the already-previewed position as unchanged.
		c.emitControl("spike:config", c.stackConfig(c.stacks[0]))
		c.acknowledge(m, nil)
		c.finishStackDragInput(d, true)
		log.Printf("stack drag committed stack=%d side=%s offset=%.6f", next.ID, next.Side, next.Offset)
	})
}
