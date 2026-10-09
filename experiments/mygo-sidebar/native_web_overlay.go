package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type webCardLayout struct {
	Title, Note, Theme, Due string
	Priority                int
	Width                   float32
}

// Transparent native canvases use the same design tokens as the main window.
// There is deliberately no painted container behind the individual labels.
func (v *views) webStack(c *ui.Context) {
	t := webTheme(v.m.UITheme)
	c.SetTheme(t.Theme)
	c.Root().Background(ui.Transparent)
	m, nextHover := v.m, -1
	if m.Quiet {
		return
	}
	if m.desktopArrange() {
		v.webArrangeStack(c)
		return
	}
	for _, i := range m.orderedIndices() {
		task := m.Tasks[i]
		if !m.sidebarVisible(i) {
			continue
		}
		index := i
		dueColor := t.Accent
		switch dueLabel(task, m.now()) {
		case "即将到期":
			dueColor = t.Warning
		case "已逾期":
			dueColor = t.Danger
		}
		r := m.markerRect(i)
		preview := m.previewRect(i)
		if m.expanded(i) {
			x := float32(0)
			if m.Side == "right" {
				x = stackWidth - 296
			}
			ui.Box(c).Absolute().Left(x).Top(float32(r.Y)).Size(296, float32(m.itemHeight())).Radius(5).Border(1, t.Border).Background(t.Surface)
		}
		handle := ui.Box(c).Key(fmt.Sprintf("handle-%d", i)).Absolute().Left(float32(r.X)).Top(float32(r.Y)).Size(float32(r.Width), float32(r.Height)).Radius(0, 5, 5, 0).Border(1, t.Border).Background(t.Alt).TextColor(dueColor).Cursor(ui.CursorMove).Label(fmt.Sprintf("拖动任务标记 %d", i+1))
		if m.Side == "right" {
			handle.Radius(5, 0, 0, 5)
		}
		if m.expanded(i) {
			handle.Background(ui.Transparent).Border(0, ui.Transparent)
		}
		handle.PointerPosition()
		if v.pointer != nil {
			handle.HandleInput(func(ev ui.InputEvent) bool { return v.pointer(index, ev) })
		}
		handle.DrawOver(func(p *ui.Painter, r ui.Rect) {
			if handle.Hovered() {
				for _, dx := range []float32{r.W/2 - 3, r.W/2 + 1} {
					for _, dy := range []float32{r.H/2 - 6, r.H/2 - 1, r.H/2 + 4} {
						p.Fill(ui.Rect{X: r.X + dx, Y: r.Y + dy, W: 2, H: 2}, dueColor, 1)
					}
				}
			} else {
				p.Fill(ui.Rect{X: r.X + r.W/2 - 1.5, Y: r.Y + (r.H-23)/2, W: 3, H: 23}, dueColor, 1.5)
			}
		})
		if handle.Hovered() {
			nextHover = i
		}
		if !m.Dragging && (m.Hover == i || m.Opened == i) {
			body := ui.ButtonBase(c).Key(fmt.Sprintf("preview-%d", i)).Label(fmt.Sprintf("打开任务 %d", i+1)).Absolute().Left(float32(preview.X)).Top(float32(preview.Y)).Size(float32(preview.Width), float32(preview.Height)).Padding(0, 12).Gap(10).TextColor(t.Text).FocusRing(false)
			body.Children(func() {
				check := ui.ButtonBase(c).Size(16, 16).Radius(8).Border(1, t.Subtle).Label("完成侧栏任务：" + task.Title)
				if check.Clicked() {
					m.open(index)
					if m.complete() {
						v.notify("complete")
						v.closeCard()
					}
				}
				ui.Text(c, task.Title).Grow(1).MinWidth(0).FontSize(13).MaxLines(1).Ellipsis("…")
				if task.DueAt > 0 {
					label := dueLabel(task, m.now())
					if label != "" {
						label += " "
					}
					label += time.UnixMilli(task.DueAt).In(m.now().Location()).Format("15:04")
					ui.Text(c, label).FontSize(10).TextColor(dueColor)
				}
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
	overflowInside := v.webOverflowTab(c)
	if m.undoVisible() {
		r := m.undoRect()
		ui.Row(c).Key("undo-toast").Absolute().Left(float32(r.X)).Top(float32(r.Y)).Size(float32(r.Width), float32(r.Height)).Padding(0, 10).Gap(12).Radius(t.Theme.Radius).Border(1, t.Border).Background(t.Surface).Children(func() {
			ui.Text(c, "已完成").FontSize(12).Grow(1)
			b := webButton(c, "撤销", false).Border(0, ui.Transparent).Padding(0).Background(ui.Transparent)
			b.Children(func() { ui.Text(c, "撤销") })
			if b.Clicked() {
				s := v.service
				if s == nil {
					s = &TasksService{m: m, run: func(f func()) { f() }, changed: v.notify}
				}
				if _, err := s.Undo(); err != nil {
					m.EditError = err.Error()
				}
			}
		})
		c.After(time.Duration(m.UndoUntil-m.now().UnixMilli()) * time.Millisecond)
	}
	if v.renderPresence {
		v.presence(nextHover == m.Opened && m.Opened >= 0 || m.OverflowOpen && overflowInside, v.cardInside)
	}
	nextHover, delay := v.webHover(nextHover, c.Now())
	if delay > 0 {
		c.After(delay)
	}
	if nextHover != m.Hover {
		m.Hover = nextHover
		c.Invalidate()
		v.notify("hover")
	}
}

func (v *views) webCard(c *ui.Context) {
	t := webTheme(v.m.UITheme)
	c.SetTheme(t.Theme)
	c.Root().Background(ui.Transparent)
	m := v.m
	if m.OverflowOpen {
		v.webOverflowCard(c)
		return
	}
	if m.Opened < 0 {
		return
	}
	if v.snoozeTask != m.Opened || m.Editing {
		v.snoozing = false
		v.snoozeTask = m.Opened
	}
	if v.renderPresence || m.Editing {
		v.presence(v.sourceInside, c.Root().Hovered())
	}
	item := m.Tasks[m.Opened]
	var reading *ui.Element
	var errorHeight float32
	footerHeight := float32(61) // 36px controls + 24px padding + top border.
	if v.snoozing && !m.Editing {
		footerHeight = 95
	}
	ui.Column(c).Fill().Radius(t.Radius).Border(1, t.Border).Background(t.Surface).Children(func() {
		ui.Row(c).Height(40).Shrink(0).Padding(10, 16, 2).Children(func() {
			label := "当前任务"
			if m.Editing {
				label = "快速编辑"
			}
			ui.Text(c, label).FontSize(11).LetterSpacing(.33).TextColor(t.TextMuted).Grow(1)
			if webIconButton(c, "关闭快速卡片", "close", 16).Radius(6).Clicked() {
				v.closeCard()
			}
		})
		if m.Opened < 0 {
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			body := ui.Column(c).WidthPercent(100).MinWidth(0).Padding(10, 16, 16)
			if !m.Editing {
				width, _ := c.Size()
				// Bounds is the previous committed layout. A new key waits for
				// fresh metrics when content, theme or wrapping width changes.
				layout := webCardLayout{item.Title, item.Note, m.UITheme, dueText(item, m.now()), item.Priority, width}
				if layout != v.cardLayout {
					v.cardLayout, v.cardLayoutVersion = layout, v.cardLayoutVersion+1
				}
				// An integer key avoids formatting the entire note every frame.
				body.Key(v.cardLayoutVersion)
				reading = body.Label("卡片阅读内容")
			}
			body.Children(func() {
				if m.Editing {
					ui.Column(c).Key("card-title").Gap(6).Margin(0, 0, 14).Children(func() {
						ui.Text(c, "任务标题").FontSize(11).TextColor(t.TextMuted)
						field := ui.TextInput(c, &m.Draft).Label("任务标题").Padding(9).Height(36).Background(t.Field)
						if v.editorFocusPending {
							field.Focus()
							v.editorFocusPending = false
						}
						if field.Submitted() && m.save() {
							v.notify("save")
						}
					})
					ui.Column(c).Key("card-note").Gap(6).Children(func() {
						ui.Text(c, "备注").FontSize(11).TextColor(t.TextMuted)
						ui.TextArea(c, &m.DraftNote).Label("卡片备注").Height(120).Padding(9).LineHeight(1.6).Background(t.Field)
					})
				} else {
					ui.Text(c, item.Title).Font(t.HeadingFont).FontSize(18).FontWeight(t.HeadingWeight).LineHeight(1.4)
					if item.Priority != 0 || item.DueAt > 0 {
						ui.Row(c).Wrap().GapX(8).GapY(6).Margin(10, 0, 0).Children(func() {
							if item.Priority != 0 {
								fg, bg := t.Warning, t.WarningSoft
								if item.Priority == 3 {
									fg, bg = t.Danger, t.DangerSoft
								}
								ui.Text(c, priorityName(item.Priority)).FontSize(10).FixedLineHeight(18).NoWrap().Padding(0, 6).Radius(4).TextColor(fg).Background(bg)
							}
							if item.DueAt > 0 {
								color := t.TextMuted
								if item.DueAt <= m.now().UnixMilli() {
									color = t.Danger
								}
								ui.Row(c).Gap(4).TextColor(color).Children(func() {
									webIcon(c, "clock", 12)
									ui.Text(c, dueText(item, m.now())).FontSize(11).FixedLineHeight(18).FontFeatures("tnum").NoWrap()
								})
							}
						})
					}
					if strings.TrimSpace(item.Note) != "" {
						ui.Text(c, item.Note).FontSize(13).LineHeight(1.65).TextColor(t.TextMuted).Margin(12, 0, 0)
					}
				}
			})
		})
		if m.EditError != "" {
			errorHeight = ui.Text(c, m.EditError).Key(m.EditError).FontSize(12).TextColor(t.Danger).Padding(8, 16).Bounds().H
		}
		ui.Column(c).Height(footerHeight).Shrink(0).Gap(6).Padding(12, 16).BorderColor(t.Border).BorderWidth(1, 0, 0, 0).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				if m.Editing {
					b := webCardButton(c, "保存", true).Disabled(strings.TrimSpace(m.Draft) == "")
					b.Children(func() { webIcon(c, "check", 14); ui.Text(c, "保存") })
					if b.Clicked() && m.save() {
						v.notify("save")
					}
					b = webCardButton(c, "取消", false)
					b.Children(func() { ui.Text(c, "取消") })
					if b.Clicked() {
						m.cancel()
						v.notify("cancel")
					}
				} else if v.snoozing {
					for _, option := range []struct{ label, duration string }{{"30 分钟", "30m"}, {"1 小时", "1h"}, {"明天 09:00", "tomorrow"}} {
						b := webCardButton(c, option.label, false)
						b.Children(func() { ui.Text(c, option.label) })
						if b.Clicked() {
							v.snooze(option.duration)
						}
					}
				} else {
					for _, action := range []struct {
						label, icon string
						primary     bool
					}{{"完成", "check", true}, {"稍后", "clock", false}, {"编辑", "edit", false}} {
						b := webCardButton(c, action.label, action.primary)
						b.Children(func() { webIcon(c, action.icon, 14); ui.Text(c, action.label) })
						if b.Clicked() {
							switch action.label {
							case "完成":
								if m.complete() {
									v.notify("complete")
									v.closeCard()
								}
							case "稍后":
								v.snoozing = true
							case "编辑":
								m.edit()
								v.editorFocusPending = true
								if v.edit != nil {
									v.edit()
								}
								v.notify("edit")
							}
						}
					}
				}
			})
			if v.snoozing && !m.Editing && m.Opened >= 0 {
				b := webButton(c, "返回操作", false).Height(28).Border(0, ui.Transparent).Background(ui.Transparent).Gap(4).Padding(0)
				b.Children(func() { webIcon(c, "back", 13); ui.Text(c, "返回") })
				if b.Clicked() {
					v.snoozing = false
				}
			}
		})
	})
	if reading != nil && !m.Editing && m.Opened >= 0 {
		if height := reading.Bounds().H; height > 0 && (m.EditError == "" || errorHeight > 0) {
			wanted := max(160, min(390, int(math.Ceil(float64(40+height+footerHeight+2+errorHeight)))))
			if m.CardReadHeight != wanted {
				m.CardReadHeight = wanted
				v.notify("card-size")
				c.Invalidate()
			}
		} else {
			c.Invalidate()
		}
	}
	if c.Root().Shortcut(0, ui.KeyEscape) {
		if m.Editing {
			m.cancel()
			v.notify("cancel")
		} else {
			v.closeCard()
		}
	}
}
