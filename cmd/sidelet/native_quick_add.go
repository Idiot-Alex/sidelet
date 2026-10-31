package main

import (
	"log"
	"runtime"
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
	saved              bool
	savedTitle         string
	presentation       uint64
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
	q.presentation++
	q.saved = false
	q.savedTitle = ""
	q.focus = true
	if q.window == nil {
		w := newAuxiliaryWindow(mygo.WindowOptions{Title: h.service.m.applicationTitle() + " · 快速添加", Width: 540, Height: 360, Hidden: true,
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
	w, h := min(540, area.Width-2*margin), min(360, area.Height-2*margin)
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
	q.saved = false
	q.savedTitle = ""
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
	h := 360
	if q.details != nil {
		if q.details.noteOpen {
			h += 70
		}
		if q.details.datePickerOpen && q.details.datePopup == nil {
			h += 250
		}
		if q.details.pickerCalendarOpen && q.details.datePopup == nil {
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
	if q.details != nil && q.details.datePopup != nil {
		return
	}
	if !q.session.Open {
		return
	}
	q.session.Blur()
	q.dismiss(false) // Preserve input/options; never take focus from a clicked app.
}
func (q *nativeQuickAdd) submit(revision uint64) {
	q.submitMode(revision, false)
}

func quickAddKeys(system string) (ui.Modifiers, string, string) {
	if system == "darwin" {
		return ui.Super, "Return ↩", "⌘ + Return"
	}
	return ui.Ctrl, "Enter", "Ctrl + Enter"
}

func (q *nativeQuickAdd) submitMode(revision uint64, closeAfter bool) {
	if !q.session.StartSave(revision) {
		return
	}
	parsed, err := q.input()
	if q.details.datePickerOpen {
		err = q.details.commitOpenDatePicker()
		if err == nil {
			parsed, err = q.input()
		}
	}
	if err == nil {
		_, err = q.service.SaveTask(0, parsed.Title, q.details.formNote, priorityValue(q.details.newPriority), q.pin, parsed.DueAt, q.details.formRemind, false, 0)
	}
	wasOpen := q.session.FinishSave(err == nil)
	if err != nil {
		q.saved = false
		q.savedTitle = ""
		q.error = err.Error()
		q.focus = q.session.Open
		q.notify("native-quick-add-save-failed")
		return
	}
	if !closeAfter && wasOpen {
		// A new revision rejects delayed clicks/Enter from the saved task.
		// Keep options, window geometry and the original foreground token.
		q.session.Begin(q.service.m.now())
		q.text, q.error, q.details.formNote = "", "", ""
		q.focus, q.saved, q.details.focusNote = true, true, false
		q.savedTitle = parsed.Title
	} else {
		q.reset()
		q.dismiss(wasOpen)
	}
	q.notify("native-quick-add-saved")
}

func (q *nativeQuickAdd) render(c *ui.Context) {
	t := webTheme(q.service.m.UITheme)
	c.SetTheme(t.Theme)
	c.Root().Background(t.Surface)
	_, previewErr := q.input()
	d := q.details
	d.onDatePopupDismiss = func() {
		go mygo.RunOnMain(func() {
			if q.window != nil && !q.window.IsFocused() && q.session.Open {
				q.blur()
			}
		})
	}
	d.visual = t
	pickerWasOpen := d.datePickerOpen || d.pickerCalendarOpen
	heightBefore := q.contentHeight()
	if strings.TrimSpace(q.text) == "" {
		previewErr = nil
	}
	revision := q.session.Revision
	closeMods, enterLabel, closeLabel := quickAddKeys(runtime.GOOS)
	busy := q.session.Saving || !q.session.Open
	enterSurface(c, ui.Column(c).Fill().Background(t.Surface), q.presentation).Children(func() {
		ui.Column(c).Fill().Padding(18, 22, 16).Border(1, t.Border).Background(t.Surface).Children(func() {
			ui.Row(c).Gap(7).Margin(0, 0, 8).AlignItems(ui.Center).Children(func() {
				header := ui.Row(c.Key("quick-result")).Grow(1).MinWidth(0).Height(44).Padding(5, 10).Gap(9).Radius(7).AlignItems(ui.Center).DragWindow().Cursor(ui.CursorMove)
				if q.saved {
					header.Background(t.Soft).Border(1, t.Accent).TextColor(t.Accent).Role(ui.RoleStatus).Label("任务添加成功").Tooltip("已添加：" + q.savedTitle)
				} else {
					header.TextColor(t.TextMuted).Tooltip("拖动可移动窗口")
				}
				header.Children(func() {
					icon := "plus"
					if q.saved {
						icon = "check"
					}
					webIcon(c, icon, 18)
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						if q.saved {
							ui.Text(c, "已添加成功，可继续输入").FontSize(13).FontWeight(600)
							ui.Text(c, q.savedTitle).FontSize(12).TextColor(t.Text).MaxLines(1).Ellipsis("…")
						} else {
							ui.Text(c, "快速添加").FontSize(13).FontWeight(600)
							ui.Text(c, "保存后可连续添加下一条").FontSize(11).TextColor(t.Subtle)
						}
					})
				})
				if webIconButton(c, "取消快速添加", "close", 16).Disabled(busy).Clicked() {
					q.cancel()
				}
			})
			ui.Scroll(c.Key("quick-form-scroll")).Grow(1).MinHeight(0).Children(func() {
				field := guardComposition(ui.TextInput(c.Key("quick-add-title"), &q.text)).Label("快速添加任务标题").Placeholder("接下来想做什么？").FontSize(19).Height(42).Padding(8, 0).Radius(0).BorderColor(t.Border).BorderWidth(0, 0, 1, 0).Background(ui.Transparent).Disabled(busy)
				field.OnTextKey(func(k ui.TextKey) bool {
					if k.Key == ui.KeyEnter && k.Mods == closeMods {
						q.submitMode(revision, true)
						k.SetText(q.text)
						return true
					}
					return false
				})
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
					if d.webTaskNote(c, true, func() { q.submitMode(revision, true) }) {
						q.resizeForNote()
					}
				})

				ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(4, 0, 14).Disabled(busy).Children(func() {
					ui.Text(c, "优先级").FontSize(12).TextColor(t.TextMuted)
					webPriorityRadios(c, &d.newPriority)
				})
				ui.Text(c, "预计完成时间").FontSize(12).TextColor(t.TextMuted).Margin(0, 0, 7)
				ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Shrink(0).Children(func() { d.webDeadlineLayout(c, true) })
					webCheckbox(c.Key("quick-remind"), &d.formRemind, "到时提醒", 12).Shrink(0).Disabled(busy || d.formDue == "").Tooltip("在预计完成时间发送通知，需保持 Sidelet 运行")
				})
			})
			ui.Column(c.Key("quick-submit-area")).Shrink(0).Padding(12, 0, 0).Margin(12, 0, 0).BorderColor(t.Border).BorderWidth(1, 0, 0, 0).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					webCheckbox(c.Key("quick-pin"), &q.pin, "固定到桌面", 12).Shrink(0).Disabled(busy)
					ui.Spacer(c)
					closeButton := webButton(c, "添加并关闭", false).Height(36).Padding(8, 10).Border(0, ui.Transparent).Background(ui.Transparent).TextColor(t.TextMuted).Disabled(busy || strings.TrimSpace(q.text) == "" || previewErr != nil)
					closeButton.Children(func() { ui.Text(c, "添加并关闭") })
					if closeButton.Clicked() {
						q.submitMode(revision, true)
					}
					label := "添加"
					if q.session.Saving {
						label = "保存中…"
					}
					b := webButton(c, label, true).Height(36).MinWidth(84).Gap(12).Padding(8, 14).Border(0, ui.Transparent).Disabled(busy || strings.TrimSpace(q.text) == "" || previewErr != nil)
					b.Children(func() { ui.Text(c, label); ui.Text(c, "↩").Opacity(.7) })
					if b.Clicked() {
						q.submit(revision)
					}
				})
				ui.Text(c, enterLabel+" 添加并继续 · "+closeLabel+" 添加并关闭 · Esc 关闭").FontSize(11).TextColor(t.Subtle).Margin(8, 0, 0)
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
