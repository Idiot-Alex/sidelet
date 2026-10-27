package main

import (
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"sidelet/internal/quickadd"
)

type quickFocusToken interface {
	restore() bool
	release()
	pid() int
}

type nativeQuickAdd struct {
	service            *TasksService
	window             *mygo.Window
	text, error        string
	focus, pin, hiding bool
	session            quickadd.Session
	previous           quickFocusToken
	ours               func() bool
	hide               func()
	onState            func(string)
	lastBounds         mygo.Rectangle
	details            *nativeTasksView
}

func (h *nativeApp) showQuickAdd() { h.openQuickAdd("window") }

func (h *nativeApp) openQuickAdd(source string) {
	if h.quitting {
		return
	}
	if h.quick == nil {
		h.quick = &nativeQuickAdd{service: h.service, ours: quickForegroundIsOurs, onState: h.onState}
		h.quick.reset()
	}
	q := h.quick
	if q.session.Saving {
		return
	}
	if q.session.Open && q.window != nil {
		focusEditor(q.window)
		return
	}
	q.releaseFocus()
	if h.captureQuickFocus != nil {
		q.previous = h.captureQuickFocus()
	}
	if h.prepareQuickAdd != nil {
		h.prepareQuickAdd()
	}
	q.session.Begin(h.service.m.now())
	q.focus = true
	if q.window == nil {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "Sidelet · 快速添加", Width: 540, Height: 340, Hidden: true,
			DisableResize: true, Frameless: true, AlwaysOnTop: true, Content: ui.View(q.render)})
		q.window = w
		q.hide = w.Hide
		w.OnBlur(func() {
			if q.window == w && !q.hiding && !w.IsFocused() {
				q.blur()
			}
		})
		w.OnClose(func(e *mygo.CloseEvent) { q.handleClose(e, h.quitting) })
		w.OnClosed(func() {
			if q.window == w {
				q.window = nil
				q.session.Blur()
				q.releaseFocus()
			}
		})
	}
	display := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	if bounds, ok := quickAddBounds(display.WorkArea); ok {
		bounds.Height = min(q.contentHeight(), display.WorkArea.Height)
		bounds = restoreQuickAddBounds(bounds, q.lastBounds, display.WorkArea)
		q.window.SetBounds(bounds)
	}
	q.window.Invalidate()
	q.window.ShowInactive()
	focusEditor(q.window)
	log.Printf("native quick-add opened source=%s revision=%d previousPID=%d", source, q.session.Revision, q.previousPID())
	q.notify("native-quick-add-opened")
}

func quickAddBounds(area mygo.Rectangle) (mygo.Rectangle, bool) {
	if area.Width <= 0 || area.Height <= 0 {
		return mygo.Rectangle{}, false
	}
	margin := min(12, area.Width/4, area.Height/4)
	w, h := min(540, area.Width-2*margin), min(340, area.Height-2*margin)
	return mygo.Rectangle{X: area.X + (area.Width-w)/2, Y: area.Y + max(margin, int(float64(area.Height-h)*.28)), Width: w, Height: h}, true
}

func restoreQuickAddBounds(bounds, previous, area mygo.Rectangle) mygo.Rectangle {
	if previous.Width > 0 && previous.Height > 0 {
		bounds.X = max(area.X, min(previous.X, area.X+area.Width-bounds.Width))
		bounds.Y = max(area.Y, min(previous.Y, area.Y+area.Height-bounds.Height))
	}
	return bounds
}

func (q *nativeQuickAdd) reset() {
	q.text, q.error, q.focus, q.pin = "", "", true, false
	q.details = nil
}

type quickTaskInput struct {
	Title string
	DueAt int64
}

func (q *nativeQuickAdd) input() (quickTaskInput, error) {
	var result quickTaskInput
	if q.details == nil {
		q.details = &nativeTasksView{service: q.service, newPriority: "普通"}
		q.details.formDue = localDue(defaultTaskDue(q.session.Reference), q.session.Reference.Location())
	}
	title, err := quickadd.Title(q.text)
	result.Title = title
	if err != nil {
		return result, err
	}
	result.DueAt, err = parseDue(q.details.formDue, 0, "", q.session.Reference.Location())
	return result, err
}
func (q *nativeQuickAdd) contentHeight() int {
	h := 340
	if q.details != nil {
		if q.details.noteOpen {
			h += 70
		}
		if q.details.datePickerOpen {
			h += 210
		}
		if q.details.pickerCalendarOpen {
			h += 250
		}
	}
	return h
}

func (q *nativeQuickAdd) resizeForNote() {
	if q.window == nil {
		return
	}
	r := q.window.Bounds()
	area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: r.X, Y: r.Y}).WorkArea
	r.Height = min(q.contentHeight(), area.Height)
	r = restoreQuickAddBounds(r, r, area)
	if q.window.Bounds() != r {
		q.window.SetBounds(r)
	}
}
func (q *nativeQuickAdd) notify(event string) {
	if q.onState != nil {
		q.onState(event)
	}
}
func (q *nativeQuickAdd) previousPID() int {
	if q.previous != nil {
		return q.previous.pid()
	}
	return 0
}
func (q *nativeQuickAdd) releaseFocus() {
	if q.previous != nil {
		q.previous.release()
		q.previous = nil
	}
}

func (q *nativeQuickAdd) dismiss(restore bool) {
	canRestore := restore && q.ours != nil && q.ours()
	if q.window != nil {
		q.lastBounds = q.window.Bounds()
	}
	q.hiding = true
	if q.hide != nil {
		q.hide()
	}
	q.hiding = false
	if canRestore && q.previous != nil {
		log.Printf("native quick-add focus restore success=%t previousPID=%d", q.previous.restore(), q.previous.pid())
	}
	q.releaseFocus()
	q.notify("native-quick-add-hidden")
}

func (q *nativeQuickAdd) cancel() {
	if !q.session.Cancel() {
		return
	}
	q.reset()
	q.dismiss(true)
}

func (q *nativeQuickAdd) handleClose(e *mygo.CloseEvent, quitting bool) {
	if quitting {
		return
	} // A reused hidden window must not veto application quit.
	e.PreventDefault()
	q.cancel()
}
func (q *nativeQuickAdd) blur() {
	if !q.session.Open {
		return
	}
	q.session.Blur()
	q.dismiss(false) // Preserve input/options; never take focus from a clicked app.
}
func (q *nativeQuickAdd) submit(revision uint64) {
	if !q.session.StartSave(revision) {
		return
	}
	parsed, err := q.input()
	if err == nil {
		_, err = q.service.SaveTask(0, parsed.Title, q.details.formNote, priorityValue(q.details.newPriority), q.pin, parsed.DueAt, false, false, 0)
	}
	wasOpen := q.session.FinishSave(err == nil)
	if err != nil {
		q.error = err.Error()
		q.focus = q.session.Open
		q.notify("native-quick-add-save-failed")
		return
	}
	q.reset()
	q.dismiss(wasOpen)
	q.notify("native-quick-add-saved")
}

func (q *nativeQuickAdd) render(c *ui.Context) {
	t := webTheme(q.service.m.UITheme)
	c.SetTheme(t.Theme)
	c.Root().Background(t.Surface)
	_, previewErr := q.input()
	d := q.details
	d.visual = t
	pickerWasOpen := d.datePickerOpen || d.pickerCalendarOpen
	heightBefore := q.contentHeight()
	if strings.TrimSpace(q.text) == "" {
		previewErr = nil
	}
	revision := q.session.Revision
	busy := q.session.Saving || !q.session.Open
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		ui.Column(c).Fill().Padding(18, 22, 16).Border(1, t.Border).Background(t.Surface).Children(func() {
			ui.Row(c).Gap(7).Margin(0, 0, 8).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Grow(1).Height(28).Gap(7).AlignItems(ui.Center).DragWindow().Cursor(ui.CursorMove).Tooltip("拖动可移动窗口").Children(func() {
					webIcon(c, "plus", 17)
					ui.Text(c, "快速添加").FontSize(13).FontWeight(600).TextColor(t.TextMuted)
				})
				if webIconButton(c, "取消快速添加", "close", 16).Disabled(busy).Clicked() {
					q.cancel()
				}
			})
			ui.Scroll(c.Key("quick-form-scroll")).Grow(1).Children(func() {
				field := guardComposition(ui.TextInput(c.Key("quick-add-title"), &q.text)).Label("快速添加任务标题").Placeholder("接下来想做什么？").FontSize(19).Height(42).Padding(8, 0).Radius(0).BorderColor(t.Border).BorderWidth(0, 0, 1, 0).Background(ui.Transparent).Disabled(busy)
				if q.focus {
					field.Focus()
					q.focus = false
				}
				if field.Changed() {
					q.error = ""
					_, previewErr = q.input()
					if strings.TrimSpace(q.text) == "" {
						previewErr = nil
					}
				}
				if field.Submitted() {
					q.submit(revision)
				}
				if q.error != "" {
					ui.Text(c, q.error).FontSize(12).TextColor(t.Danger).Margin(6, 0)
				} else if previewErr != nil {
					ui.Text(c, previewErr.Error()).FontSize(12).TextColor(t.Danger).Margin(6, 0)
				}
				ui.Column(c).Disabled(busy).Children(func() {
					if d.webTaskNote(c, true, func() { q.submit(revision) }) {
						q.resizeForNote()
					}
				})

				ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(4, 0, 14).Disabled(busy).Children(func() {
					ui.Text(c, "优先级").FontSize(12).TextColor(t.TextMuted)
					webPriorityRadios(c, &d.newPriority)
				})
				ui.Text(c, "预计完成时间").FontSize(12).TextColor(t.TextMuted).Margin(0, 0, 7)
				d.webDeadline(c)
				ui.Spacer(c)
				ui.Row(c).Gap(14).Children(func() {
					webCheckbox(c, &q.pin, "固定到桌面", 12).Disabled(busy)
					ui.Spacer(c)
					label := "添加"
					if q.session.Saving {
						label = "保存中…"
					}
					b := webButton(c, label, true).Gap(12).Padding(8, 12).Border(0, ui.Transparent).Disabled(busy || strings.TrimSpace(q.text) == "" || previewErr != nil)
					b.Children(func() { ui.Text(c, label); ui.Text(c, "↵").Opacity(.7) })
					if b.Clicked() {
						q.submit(revision)
					}
				})
				ui.Text(c, "Enter 添加 · Esc 取消").FontSize(12).TextColor(t.Subtle).Margin(8, 0, 0)
			})
		})
	})
	if q.contentHeight() != heightBefore {
		q.resizeForNote()
	}
	if !pickerWasOpen && c.Root().Shortcut(0, ui.KeyEscape) {
		q.cancel()
	}
}
