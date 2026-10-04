//go:build windows || darwin

package main

import (
	"errors"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"sidelet/internal/storage"
	"sidelet/internal/todo"
)

// The queue worker performs SQLite IO outside InvokeSync. Only publication and
// native window changes run on the UI thread, after a successful commit.
func (c *controller) process(m message) {
	if m.Type == "settings-save" || m.Type == "settings-login" || m.Type == "settings-refresh" {
		c.processSettings(m)
		return
	}
	if m.Type == "reminders-sync" || m.Type == "notification-permission" {
		c.syncReminders(m.Type == "notification-permission")
		return
	}
	if m.Type == "stack-drag-end" {
		c.finishStackDrag(m)
		return
	}
	if c.store == nil || (m.Type != "action" && m.Type != "persist-stack" && m.Type != "side" && m.Type != "offset" && m.Type != "density") || (m.Type == "action" && m.Action.Type == "select") {
		application.InvokeSync(func() {
			if err := c.handle(m); err != nil {
				log.Print(err)
				c.acknowledge(m, err)
				c.app.Event.Emit("spike:error", err.Error())
			}
		})
		return
	}
	var allowed bool
	var stack todo.EdgeStack
	var remapCurrent bool
	application.InvokeSync(func() {
		allowed = true
		c.cancelStackDrag()
		if m.Type == "action" && m.Action.Type == "reorder" && !c.arranging {
			allowed = false
		}
		if c.find(m.Window) != nil {
			c.refreshFullscreen()
			allowed = allowed && !c.quiet && !c.fullscreen
		}
		if len(c.snapshot.Stacks) > 0 {
			stack = c.snapshot.Stacks[0]
		}
		if m.Type == "persist-stack" {
			for _, o := range c.stacks {
				if o.stackID == m.Stack.ID && o.displayID == m.Stack.DisplayID && o.side == m.Stack.Side {
					for _, saved := range c.snapshot.Stacks {
						if saved.ID == m.Stack.ID {
							stack = saved
							stack.DisplayID, stack.Side = m.Stack.DisplayID, m.Stack.Side
							remapCurrent = true
						}
					}
				}
			}
		}
	})
	// Display changes may queue several remaps. Keep the latest native target
	// and current persisted offset/density instead of replaying an old snapshot.
	if m.Type == "persist-stack" && !remapCurrent {
		return
	}
	if !allowed {
		application.InvokeSync(func() { c.acknowledge(m, errors.New("desktop input is unavailable")) })
		return
	}
	var state todo.Snapshot
	var err error
	now := time.Now()
	switch m.Type {
	case "action":
		state, err = c.store.Apply(m.Action, now)
	case "persist-stack":
		state, err = c.store.SaveStack(stack, now)
	case "side":
		stack.Side = m.Side
		state, err = c.store.SaveStack(stack, now)
	case "offset":
		stack.Offset = m.Offset
		state, err = c.store.SaveStack(stack, now)
	case "density":
		switch m.ItemHeight {
		case 40:
			stack.Density = "compact"
		case 44:
			stack.Density = "normal"
		case 56:
			stack.Density = "relaxed"
		default:
			err = errors.New("invalid density")
		}
		if err == nil {
			state, err = c.store.SaveStack(stack, now)
		}
	}
	application.InvokeSync(func() {
		if err != nil {
			log.Printf("storage operation=%s action=%s failed: %v", m.Type, m.Action.Type, err)
			c.acknowledge(m, err)
			if m.Window != nil {
				m.Window.EmitEvent("spike:error", storage.UserMessage(err))
			} else {
				c.control.EmitEvent("spike:error", storage.UserMessage(err))
			}
			c.control.EmitEvent("spike:config", c.stackConfig(c.stacks[0]))
			return
		}
		c.acceptPersistentState(state)
		if m.Type == "action" {
			if m.Action.Type == "complete" || m.Action.Type == "snooze" || m.Action.Type == "delete" || m.Action.Type == "unpin" {
				if c.quickSession.Open && c.quickSession.TodoID == m.Action.ID {
					c.hideQuick(true)
				}
			}
			log.Printf("storage committed action=%s tasks=%d", m.Action.Type, len(c.snapshot.Todos))
		}
		c.acknowledge(m, nil)
		if m.Type == "action" && m.Action.Remind != nil && *m.Action.Remind {
			c.post(message{Type: "notification-permission"})
		}
	})
}

func (c *controller) acknowledge(m message, err error) {
	requestID := m.RequestID
	if requestID == "" {
		requestID = m.Action.RequestID
	}
	if requestID == "" || m.Window == nil {
		return
	}
	text := ""
	if err != nil {
		text = storage.UserMessage(err)
	}
	m.Window.EmitEvent("todo:result", map[string]any{"requestId": requestID, "error": text})
}

func (c *controller) acceptPersistentState(state todo.Snapshot) {
	state.SelectedID = c.snapshot.SelectedID
	for _, id := range c.snapshot.OverflowIDs {
		for _, item := range state.Todos {
			if item.ID == id {
				state.OverflowIDs = append(state.OverflowIDs, id)
				break
			}
		}
	}
	c.snapshot = state
	changed := false
	for _, o := range c.stacks {
		for _, saved := range state.Stacks {
			if saved.ID == o.stackID {
				if o.displayID != saved.DisplayID || o.side != saved.Side || o.offset != saved.Offset || o.itemHeight != storage.ItemHeight(saved.Density) {
					changed = true
				}
				o.displayID = saved.DisplayID
				o.side = saved.Side
				o.offset = saved.Offset
				o.itemHeight = storage.ItemHeight(saved.Density)
			}
		}
	}
	if changed {
		if err := c.layout(); err != nil {
			log.Print(err)
			c.app.Event.Emit("spike:error", "桌面位置调整失败，请重试。")
		}
	}
	c.app.Event.Emit("spike:state", c.snapshot.Copy())
	c.scheduleTemporaryCleanup()
	c.post(messageForReminders())
}

func (c *controller) scheduleTemporaryCleanup() {
	if c.cleanupTimer != nil {
		c.cleanupTimer.Stop()
		c.cleanupTimer = nil
	}
	var deadline int64
	for _, item := range c.snapshot.Todos {
		if item.Temporary && item.Completed {
			due := item.CompletedAt + 5001
			if deadline == 0 || due < deadline {
				deadline = due
			}
		}
	}
	if deadline != 0 {
		c.cleanupTimer = time.AfterFunc(max(time.Millisecond, time.Until(time.UnixMilli(deadline))), func() { c.post(message{Type: "action", Action: todo.Action{Type: "cleanup"}}) })
	}
}
