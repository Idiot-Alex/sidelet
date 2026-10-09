package main

import (
	"log"
	"strings"
	"time"

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
	service                       *TasksService
	window                        *mygo.Window
	text, error                   string
	focus, pin, recognize, hiding bool
	session                       quickadd.Session
	previous                      quickFocusToken
	ours                          func() bool
	hide                          func()
	onState                       func(string)
}

func (h *hybridApp) showQuickAdd() { h.openQuickAdd("window") }

func (h *hybridApp) openQuickAdd(source string) {
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
		w := mygo.NewWindow(mygo.WindowOptions{Title: "Sidelet · 快速添加", Width: 540, Height: 260, Hidden: true,
			DisableResize: true, Frameless: true, Transparent: true, AlwaysOnTop: true, Content: ui.View(q.render)})
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
	w, h := min(540, area.Width-2*margin), min(260, area.Height-2*margin)
	return mygo.Rectangle{X: area.X + (area.Width-w)/2, Y: area.Y + max(margin, int(float64(area.Height-h)*.28)), Width: w, Height: h}, true
}

func (q *nativeQuickAdd) reset() {
	q.text, q.error, q.focus, q.pin, q.recognize = "", "", true, false, true
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
	parsed, err := quickadd.Parse(q.text, q.recognize, q.session.Reference)
	if err == nil {
		_, err = q.service.SaveTask(0, parsed.Title, "", 0, q.pin, parsed.DueAt, false, false, 0)
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
	c.Root().Background(ui.Transparent)
	preview, previewErr := quickadd.Parse(q.text, q.recognize, q.session.Reference)
	if strings.TrimSpace(q.text) == "" {
		previewErr = nil
	}
	revision := q.session.Revision
	busy := q.session.Saving || !q.session.Open
	ui.Column(c).Fill().Padding(12).Children(func() {
		ui.Column(c).Fill().Padding(15, 18, 12).Border(1, t.Border).Radius(t.Radius).Background(t.Surface).Children(func() {
			ui.Row(c).Gap(7).Margin(0, 0, 12).Children(func() {
				webIcon(c, "plus", 17)
				ui.Text(c, "快速添加").FontSize(12).FontWeight(600).TextColor(t.TextMuted).Grow(1)
				if webIconButton(c, "取消快速添加", "close", 16).Disabled(busy).Clicked() {
					q.cancel()
				}
			})
			field := ui.TextInput(c, &q.text).Label("快速添加任务标题").Placeholder("明天下午3点 联系客户").FontSize(19).Height(42).Padding(8, 0).Radius(0).BorderColor(t.Border).BorderWidth(0, 0, 1, 0).Background(ui.Transparent).Disabled(busy)
			if q.focus {
				field.Focus()
				q.focus = false
			}
			if field.Changed() {
				q.error = ""
				preview, previewErr = quickadd.Parse(q.text, q.recognize, q.session.Reference)
				if strings.TrimSpace(q.text) == "" {
					previewErr = nil
				}
			}
			if field.Submitted() {
				q.submit(revision)
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).MinHeight(37).Padding(7, 0).Children(func() {
				if q.error != "" {
					ui.Text(c, q.error).FontSize(11).TextColor(t.Danger)
					return
				}
				if previewErr != nil {
					ui.Text(c, previewErr.Error()).FontSize(11).TextColor(t.Danger)
					return
				}
				if preview.DueAt != 0 {
					ui.Text(c, time.UnixMilli(preview.DueAt).In(q.session.Reference.Location()).Format("1/2 15:04")).FontSize(11).TextColor(t.Accent).Background(t.Soft).Padding(3, 6).Radius(4).Shrink(0).NoWrap()
					ui.Text(c, preview.Title).FontSize(11).TextColor(t.TextMuted).Grow(1).Shrink(1).MinWidth(0).MaxLines(1).Ellipsis("…")
					return
				}
				text := "整句保存为标题，不设置截止时间。"
				if q.recognize {
					text = "支持今天、明天、后天，如“明天15:30 联系客户”。"
				}
				ui.Text(c, text).FontSize(11).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			ui.Row(c).Gap(14).Children(func() {
				webCheckbox(c, &q.recognize, "识别时间", 11).Disabled(busy)
				webCheckbox(c, &q.pin, "固定到桌面", 11).Disabled(busy)
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
			ui.Text(c, "Enter 保存 · Esc 取消 · 截止时间默认不发送通知").FontSize(10).TextColor(t.Subtle).Margin(8, 0, 0)
		})
	})
	if c.Root().Shortcut(0, ui.KeyEscape) {
		q.cancel()
	}
}
