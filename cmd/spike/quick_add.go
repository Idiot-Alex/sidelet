//go:build windows || darwin

package main

import (
	"errors"
	"log"
	"math"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"sidelet/internal/platform"
	"sidelet/internal/quickadd"
	"sidelet/internal/storage"
	"sidelet/internal/todo"
)

const quickAddShortcutLabel = "Ctrl+Shift+Space"

func (c *controller) emitQuickAdd() {
	if c.add != nil {
		c.add.window.EmitEvent("quick-add:state", c.addSession)
	}
}

func (c *controller) bindQuickAdd() error {
	if c.add.native != nil {
		return nil
	}
	if c.memoryDir != "" {
		platform.EnableMemoryDiagnostics()
	}
	w, err := platform.Bind(c.add.window.NativeWindow(), platform.Callbacks{Blur: func() { c.post(message{Type: "blur"}) }})
	if err != nil {
		return err
	}
	c.add.native = w
	w.ConfigureQuickAdd()
	if err = w.SetRegions([]platform.Rect{{X: 0, Y: 0, Width: 540, Height: 260}}, 540, 260); err != nil {
		return err
	}
	return nil
}

func (c *controller) registerQuickAdd(w *platform.Window) {
	if err := w.RegisterQuickAddShortcut(); err != nil {
		c.addShortcutError = "快捷键注册失败，可能已被其他应用占用。仍可从菜单栏或任务窗口快速添加；解除占用后重启 Sidelet。"
		log.Printf("quick-add shortcut failed: %v", err)
	} else {
		log.Printf("quick-add shortcut registered shortcut=%s exclusive=true", quickAddShortcutLabel)
	}
	c.post(message{Type: "settings-refresh"})
}

func (c *controller) createQuickAdd() {
	if c.sharedPopup {
		c.ensureSharedPopup()
		return
	}
	url := "/?view=add&platform=" + runtime.GOOS
	if c.memoryDir != "" {
		url += "&memory=1"
	}
	options := overlayOptions("add", url)
	options.Title = "Sidelet · 快速添加"
	options.Width = 540
	options.Height = 260
	c.add = &overlay{window: c.app.Window.NewWithOptions(options), mode: "Passive"}
	window := c.add.window
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		c.post(message{Type: "quick-add-window-close", Window: window})
	})
}

func (c *controller) openQuickAdd(source string) error {
	if c.store == nil {
		return errors.New("请在正常任务版本中使用快速添加。")
	}
	if c.add == nil || c.add.native == nil {
		if source == "" {
			source = "window"
		}
		c.addPendingSource = source
		if c.add == nil {
			c.createQuickAdd()
		}
		return nil
	}
	if c.addSession.Saving {
		return nil
	}
	if c.addSession.Open {
		if c.sharedPopup && c.popup.Preparing {
			return nil
		}
		c.add.native.ShowInactive()
		return c.add.native.Activate()
	}
	previous := platform.CaptureForeground()
	for _, o := range append(append([]*overlay{}, c.stacks...), c.quick) {
		if o.mode != "Passive" {
			previous = c.previous
			break
		}
	}
	d, err := platform.QuickAddDisplay()
	if err != nil {
		return err
	}
	area, scale := d.WorkArea, d.Scale
	margin := math.Min(12*scale, math.Min(area.Width, area.Height)/4)
	width, height := math.Min(540*scale, area.Width-2*margin), math.Min(260*scale, area.Height-2*margin)
	if c.sharedPopup {
		c.exitModes(false)
		c.hideQuick(false)
	}
	if err = c.add.native.Move(platform.Rect{X: area.X + (area.Width-width)/2, Y: area.Y + math.Max(margin, (area.Height-height)*.28), Width: width, Height: height}); err != nil {
		return err
	}
	c.exitModes(false)
	c.hideQuick(false)
	c.addPrevious = previous
	c.addSession.Begin(time.Now())
	c.emitQuickAdd()
	if c.sharedPopup {
		c.preparePopup("add", c.addSession.Revision)
		log.Printf("quick-add preparing source=%s revision=%d", source, c.addSession.Revision)
		return nil
	}
	return c.activateQuickAdd(source)
}

func (c *controller) presentQuickAdd(m message) error {
	if !c.addSession.Open || c.addSession.Saving || m.Revision != c.addSession.Revision {
		return nil
	}
	return c.activateQuickAdd("shared-popup")
}

func (c *controller) activateQuickAdd(source string) error {
	c.add.native.ShowInactive()
	if err := c.add.native.Activate(); err != nil {
		c.addSession.Blur()
		c.add.native.Hide()
		return err
	}
	c.add.mode = "Editing"
	c.emitQuickAdd()
	log.Printf("quick-add opened source=%s revision=%d", source, c.addSession.Revision)
	if c.traceFocus {
		log.Printf("quick-add focus captured token=%s", c.addPrevious.Diagnostic())
		c.logFocus("quick-add-open")
	}
	return nil
}

// Restore only while Sidelet still owns the foreground. Clicking another app
// dismisses the popup without stealing focus, and keeps its unfinished input.
func (c *controller) hideQuickAdd(restore, clear bool) {
	if c.add == nil || c.add.native == nil || (c.sharedPopup && c.popup.View != "add") {
		return
	}
	wasOpen := c.addSession.Open
	if clear {
		if !c.addSession.Cancel() {
			return
		}
	} else {
		c.addSession.Blur()
	}
	canRestore := restore && wasOpen && platform.ForegroundApplicationIsOurs()
	if c.sharedPopup {
		c.popup.Close("add")
	}
	_ = c.add.native.Passive()
	c.add.native.Hide()
	c.add.mode = "Passive"
	c.emitQuickAdd()
	if canRestore {
		ok := c.addPrevious.Restore()
		log.Printf("quick-add focus restore success=%t token=%s", ok, c.addPrevious.Diagnostic())
	}
	c.logFocus("quick-add-hidden")
}

func (c *controller) processQuickAdd(m message) {
	if c.add == nil || m.Window != c.add.window || m.RequestID == "" {
		return
	}
	var accepted bool
	var reference time.Time
	application.InvokeSync(func() {
		accepted = c.addSession.Open && m.Revision == c.addSession.Revision && !c.addSession.Saving
		reference = c.addSession.Reference
		if accepted && m.Type == "quick-add-save" {
			accepted = c.addSession.StartSave(m.Revision)
			c.emitQuickAdd()
		}
	})
	result := map[string]any{"requestId": m.RequestID, "error": ""}
	if !accepted {
		result["error"] = "这次快速添加已结束或正在保存，请重新打开后重试。"
		application.InvokeSync(func() { m.Window.EmitEvent("todo:result", result) })
		return
	}
	parsed, err := quickadd.Parse(m.Action.Title, m.Enabled, reference)
	if m.Type == "quick-add-preview" {
		result["parsed"] = parsed
		if err != nil {
			result["error"] = err.Error()
		}
		application.InvokeSync(func() { m.Window.EmitEvent("todo:result", result) })
		return
	}
	var state todo.Snapshot
	if err == nil {
		state, err = c.store.Apply(todo.Action{Type: "create", Title: parsed.Title, DueAt: &parsed.DueAt, Pin: m.Action.Pin}, time.Now())
		if err != nil {
			result["error"] = storage.UserMessage(err)
		}
	} else {
		result["error"] = err.Error()
	}
	application.InvokeSync(func() {
		wasOpen := c.addSession.FinishSave(err == nil)
		if err == nil {
			c.acceptPersistentState(state)
			canRestore := wasOpen && platform.ForegroundApplicationIsOurs()
			if c.sharedPopup {
				c.popup.Close("add")
			}
			_ = c.add.native.Passive()
			c.add.native.Hide()
			c.add.mode = "Passive"
			c.emitQuickAdd()
			if canRestore {
				ok := c.addPrevious.Restore()
				log.Printf("quick-add focus restore success=%t token=%s", ok, c.addPrevious.Diagnostic())
			}
			log.Printf("quick-add saved tasks=%d pinned=%t", len(state.Todos), m.Action.Pin)
		} else {
			c.emitQuickAdd()
			log.Printf("quick-add save failed: %v", err)
		}
		m.Window.EmitEvent("todo:result", result)
		c.logFocus("quick-add-save-result")
	})
}
