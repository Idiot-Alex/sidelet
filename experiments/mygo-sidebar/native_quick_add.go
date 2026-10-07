package main

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type nativeQuickAdd struct {
	service     *TasksService
	window      *mygo.Window
	text, error string
	focus       bool
	pin         bool
}

func (h *hybridApp) showQuickAdd() {
	if h.quick == nil {
		h.quick = &nativeQuickAdd{service: h.service}
	}
	q := h.quick
	if q.window != nil {
		q.window.Show()
		q.window.Focus()
		return
	}
	q.reset()
	w := mygo.NewWindow(mygo.WindowOptions{Title: "Sidelet · 快速添加", Width: 540, Height: 260, DisableResize: true, Frameless: true, Transparent: true, AlwaysOnTop: true, Content: ui.View(q.render)})
	q.window = w
	w.OnClosed(func() {
		if q.window == w {
			q.window = nil
		}
		if h.window != nil && h.window.IsVisible() {
			h.window.Focus()
		}
		if h.onState != nil {
			h.onState("native-quick-add-closed")
		}
	})
	w.Focus()
	if h.onState != nil {
		h.onState("native-quick-add-opened")
	}
}

func (q *nativeQuickAdd) reset() { q.text, q.error, q.focus, q.pin = "", "", true, false }

func (q *nativeQuickAdd) cancel() {
	if q.window != nil {
		q.window.Close()
	}
}
func (q *nativeQuickAdd) submit() {
	_, err := q.service.AddWithPin(q.text, "", 0, q.pin)
	if err != nil {
		q.error = err.Error()
		return
	}
	q.cancel()
}

func (q *nativeQuickAdd) render(c *ui.Context) {
	t := webTheme(q.service.m.UITheme)
	c.SetTheme(t.Theme)
	c.Root().Background(ui.Transparent)
	ui.Column(c).Fill().Padding(12).Children(func() {
		ui.Column(c).Fill().Padding(15, 18, 12).Border(1, t.Border).Radius(t.Radius).Background(t.Surface).Children(func() {
			ui.Row(c).Gap(7).Margin(0, 0, 12).Children(func() {
				webIcon(c, "plus", 17)
				ui.Text(c, "快速添加").FontSize(12).FontWeight(600).TextColor(t.TextMuted).Grow(1)
				if webIconButton(c, "取消快速添加", "close", 16).Clicked() {
					q.cancel()
				}
			})
			field := ui.TextInput(c, &q.text).Label("快速添加任务标题").Placeholder("明天下午3点 联系客户").FontSize(19).Height(42).Padding(8, 0).Radius(0).BorderColor(t.Border).BorderWidth(0, 0, 1, 0).Background(ui.Transparent)
			if q.focus {
				field.Focus()
				q.focus = false
			}
			if field.Changed() {
				q.error = ""
			}
			if field.Submitted() {
				q.submit()
			}
			text := "整句保存为标题，不设置截止时间。"
			color := t.TextMuted
			if q.error != "" {
				text, color = q.error, t.Danger
			}
			ui.Text(c, text).FontSize(11).TextColor(color).MinHeight(37).Padding(7, 0)
			ui.Spacer(c)
			ui.Row(c).Gap(14).Children(func() {
				recognize := false
				webCheckbox(c, &recognize, "识别时间", 11).Disabled(true)
				webCheckbox(c, &q.pin, "固定到桌面", 11)
				ui.Spacer(c)
				b := webButton(c, "添加", true).Gap(12).Padding(8, 12).Border(0, ui.Transparent).Disabled(strings.TrimSpace(q.text) == "")
				b.Children(func() { ui.Text(c, "添加"); ui.Text(c, "↵").Opacity(.7) })
				if b.Clicked() {
					q.submit()
				}
			})
			ui.Text(c, "Enter 保存 · Esc 取消 · 截止时间默认不发送通知").FontSize(10).TextColor(t.Subtle).Margin(8, 0, 0)
		})
	})
	if c.Root().Shortcut(0, ui.KeyEscape) {
		q.cancel()
	}
}
