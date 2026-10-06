//go:build windows || darwin

package main

import (
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"sidelet/internal/spike"
)

type quickRequest struct {
	message  message
	source   *overlay
	revision uint64
	started  time.Time
}

func (c *controller) ensureQuickCard() {
	if c.quick.window != nil {
		return
	}
	c.quick.window = c.app.Window.NewWithOptions(c.quickOptions)
	window := c.quick.window
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		c.post(message{Type: "close-quick", Window: window})
	})
}

func (c *controller) openQuickCard(m message) error {
	c.refreshFullscreen()
	if c.arranging || c.quiet || c.fullscreen || c.addSession.Open || c.addSession.Saving {
		return nil
	}
	source := c.find(m.Window)
	if source == nil || source == c.quick {
		source = c.stacks[0]
	}
	if source.native == nil {
		return nil
	}
	spike.Apply(&c.snapshot, spike.Action{Type: "select", ID: m.Action.ID}, time.Now())
	c.snapshot.OverflowIDs = nil
	if m.Type == "overflow" {
		for _, id := range m.IDs {
			for _, task := range c.snapshot.Todos {
				if task.ID == id {
					c.snapshot.OverflowIDs = append(c.snapshot.OverflowIDs, id)
				}
			}
		}
	}
	c.quickSession.Begin(source.window.Name(), m.Action.ID)
	c.quickSession.Presence(source.window.Name(), c.pointers[source.window.Name()], time.Now())
	c.quickPending = &quickRequest{message: m, source: source, revision: c.quickSession.RequestRevision, started: time.Now()}
	c.app.Event.Emit("spike:state", c.snapshot.Copy())
	c.emitPresentation()
	c.scheduleQuickClose()
	c.ensureQuickCard()
	return c.prepareQuickCard()
}

func (c *controller) quickRequestValid(request *quickRequest) bool {
	if request == nil || c.quiet || c.fullscreen || c.arranging || c.addSession.Open || c.addSession.Saving {
		return false
	}
	if c.find(request.source.window) != request.source || request.source.native == nil {
		return false
	}
	if !c.quickSession.CanPresent(request.revision, time.Now(), c.interactionActive()) {
		return false
	}
	ids := []int64{request.message.Action.ID}
	if request.message.Type == "overflow" {
		ids = c.snapshot.OverflowIDs
	}
	for _, id := range ids {
		for _, task := range c.snapshot.Todos {
			if task.ID == id && !task.Completed && task.SnoozedUntil <= time.Now().UnixMilli() {
				if c.store == nil || (task.DisplayMode == "EDGE" && task.StackID == request.source.stackID) {
					return true
				}
			}
		}
	}
	return false
}

func (c *controller) prepareQuickCard() error {
	if !c.quickReady || c.quickPending == nil {
		return nil
	}
	r := c.quickPending
	if !c.quickRequestValid(r) {
		c.hideQuick(false)
		return nil
	}
	if err := c.quick.native.PlaceQuick(r.source.native, r.message.Anchor, r.source.side); err != nil {
		c.hideQuick(false)
		return err
	}
	// Publish the latest task/theme before asking Svelte to acknowledge a flush.
	if c.preferences != nil {
		c.quick.window.EmitEvent("settings:state", c.settingsEvent())
	}
	c.quick.window.EmitEvent("spike:state", c.snapshot.Copy())
	c.quick.window.EmitEvent("spike:config", c.stackConfig(r.source))
	c.emitPresentation()
	c.quick.window.EmitEvent("quick:prepare", r.revision)
	return nil
}

func (c *controller) presentQuickCard(m message) error {
	r := c.quickPending
	if m.Window != c.quick.window || r == nil || m.Revision != r.revision {
		return nil
	}
	c.refreshFullscreen()
	if !c.quickRequestValid(r) {
		c.hideQuick(false)
		return nil
	}
	c.quickPending = nil
	c.quick.native.ShowInactive()
	if c.tracePointer || c.traceFocus {
		c.quickPaintStart = r.started
		log.Printf("quick shown revision=%d request-to-show-ms=%.2f", r.revision, float64(time.Since(r.started).Microseconds())/1000)
		c.quick.window.EmitEvent("quick:shown", r.revision)
	}
	if c.tracePointer {
		logNativeWindow(r.source)
		logNativeWindow(c.quick)
	}
	if r.message.Mode == "Editing" || r.source.mode == "KeyboardActive" {
		return c.enterMode(c.quick, r.message.Mode)
	}
	c.scheduleQuickClose()
	return nil
}
