package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"sidelet/internal/spike"
)

type views struct {
	service                  *TasksService
	session                  spike.QuickSession
	sourceInside, cardInside bool
	closeTimer               *time.Timer
	scheduleClose            func(time.Duration, func())
	renderPresence           bool
	snoozing                 bool
	snoozeTask               int
	m                        *model
	open                     func(int)
	close                    func()
	edit                     func()
	changed                  func(string)
	pointer                  func(int, ui.InputEvent) bool
	editorFocusPending       bool
	hoverCandidate           int
	hoverDeadline            time.Time
	order                    orderUI
	showAll                  func()
}

func (v *views) webHover(hit int, now time.Time) (int, time.Duration) {
	m := v.m
	if m.Quiet || m.Dragging || m.Arranging || m.OverflowOpen {
		v.hoverDeadline = time.Time{}
		return -1, 0
	}
	if m.Opened >= 0 {
		v.hoverDeadline = time.Time{}
		return m.Opened, 0
	}
	if hit == m.Hover {
		v.hoverDeadline = time.Time{}
		return hit, 0
	}
	if v.hoverDeadline.IsZero() || v.hoverCandidate != hit {
		delay := 150 * time.Millisecond
		if hit < 0 {
			delay = 500 * time.Millisecond
		}
		v.hoverCandidate = hit
		v.hoverDeadline = now.Add(delay)
	}
	if now.Before(v.hoverDeadline) {
		return m.Hover, v.hoverDeadline.Sub(now)
	}
	v.hoverDeadline = time.Time{}
	return hit, 0
}

var ink = ui.Hex("#27312b")
var muted = ui.Hex("#657066")
var accent = ui.Hex("#57745d")

func uiFaceColor() ui.Color { return ui.Hex("#f7f8f4") }

func clearRoot(c *ui.Context) {
	t := *c.Theme()
	t.Background, t.Text, t.TextMuted, t.Accent = ui.Transparent, ink, muted, accent
	t.FontSize, t.Radius = 13, 7
	c.SetTheme(&t)
	c.Root().Background(ui.Transparent)
}

func (v *views) stack(c *ui.Context) {
	if v.m.UITheme != "" {
		v.webStack(c)
		return
	}
	clearRoot(c)
	m := v.m
	nextHover := -1
	for i := range m.Tasks {
		if !m.sidebarVisible(i) {
			continue
		}
		index := i
		top := float32(8 + m.rowFor(i)*rowPitch)
		x := float32(0)
		if m.Side == "right" {
			x = stackWidth - 14
		}
		handle := ui.Box(c).Key(fmt.Sprintf("handle-%d", i)).Absolute().Left(x).Top(top).Size(14, 44).
			Radius(5).Background(priorityColor(m.Tasks[i].Priority)).Label(fmt.Sprintf("拖动任务标记 %d", i+1)).Cursor(ui.CursorMove)
		handle.PointerPosition() // Register pointer tracking, including captured moves.
		if v.pointer != nil {
			handle.HandleInput(func(ev ui.InputEvent) bool { return v.pointer(index, ev) })
		}
		if handle.Hovered() {
			nextHover = i
		}
		if !m.Dragging && (m.Hover == i || m.Opened == i) {
			bodyX := float32(14)
			if m.Side == "right" {
				bodyX = 0
			}
			// The bridge touches the handle; the painted face has a small inset.
			body := ui.ButtonBase(c).Key(fmt.Sprintf("preview-%d", i)).Absolute().Left(bodyX).Top(top).Size(stackWidth-14, 44).
				Padding(0, 12).Radius(8).Background(ui.Hex("#f7f8f4")).Border(1, ui.Hex("#dce3d9")).FocusRing(false).
				Label(fmt.Sprintf("打开任务 %d", i+1))
			body.Children(func() {
				ui.Text(c, m.Tasks[index].Title).Grow(1).FontSize(13).MaxLines(1)
				ui.Text(c, "›").FontSize(18).TextColor(muted)
			})
			if body.Hovered() {
				nextHover = i
			}
			if body.Clicked() {
				m.open(i)
				if v.open != nil {
					v.open(i)
				}
			}
		}
	}
	if m.Dragging {
		nextHover = -1
	}
	if m.Opened >= 0 {
		nextHover = m.Opened
	}
	if nextHover != m.Hover {
		m.Hover = nextHover
		c.Invalidate()
		v.notify("hover")
	}
}

func (v *views) card(c *ui.Context) {
	if v.m.UITheme != "" {
		v.webCard(c)
		return
	}
	clearRoot(c)
	m := v.m
	if m.Opened < 0 {
		return
	}
	root := ui.Column(c).Fill().Padding(16).Gap(12).Radius(12).Background(ui.Hex("#f7f8f4")).Border(1, ui.Hex("#dce3d9"))
	root.Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			ui.Box(c).Size(6, 6).Radius(3).Background(priorityColor(m.Tasks[m.Opened].Priority))
			ui.Text(c, "MYGO · 独立实验").FontSize(10).TextColor(muted).Grow(1)
			if ui.Button(c, "关闭").Clicked() {
				v.closeCard()
			}
		})
		if m.Opened < 0 {
			return
		}
		if m.Editing {
			ui.Box(c).Key("task-title").Children(func() {
				field := ui.TextInput(c, &m.Draft).Label("任务标题").Height(36).AutoFocus()
				if v.editorFocusPending {
					field.Focus()
					v.editorFocusPending = false
				}
			})
			help := "只修改合成任务，取消不会保存。"
			if m.EditError != "" {
				help = m.EditError
			}
			ui.Text(c, help).FontSize(11).TextColor(muted)
			ui.Spacer(c)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					m.cancel()
					v.notify("cancel")
				}
				if ui.PrimaryButton(c, "保存").Clicked() && m.save() {
					v.notify("save")
				}
			})
		} else {
			ui.Text(c, m.Tasks[m.Opened].Title).FontSize(19).Bold().MaxLines(2)
			ui.Text(c, m.Tasks[m.Opened].Note).FontSize(13).TextColor(muted)
			ui.Spacer(c)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "编辑").Clicked() {
					m.edit()
					v.editorFocusPending = true
					if v.edit != nil {
						v.edit()
					}
					v.notify("edit")
				}
				if ui.PrimaryButton(c, "完成").Clicked() {
					m.complete()
					v.notify("complete")
					v.closeCard()
				}
			})
		}
	})
	if c.Root().Shortcut(0, ui.KeyEscape) {
		if m.Editing {
			m.cancel()
			v.notify("cancel")
		} else {
			v.closeCard()
		}
	}
}

func (v *views) closeCard() {
	v.m.close()
	if v.close != nil {
		v.close()
	}
	v.notify("close")
}
func (v *views) notify(event string) {
	if !v.m.cardOpen() {
		v.session.Close()
		if v.closeTimer != nil {
			v.closeTimer.Stop()
		}
	}
	if event == "save" || event == "cancel" {
		v.armClose()
	}
	if v.changed != nil {
		v.changed(event)
	}
}
func priorityColor(p int) ui.Color {
	switch p {
	case 2:
		return ui.Hex("#ba8a39")
	case 3:
		return ui.Hex("#b86158")
	default:
		return accent
	}
}
