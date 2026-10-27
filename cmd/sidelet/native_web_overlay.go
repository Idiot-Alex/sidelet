package main

import (
	"fmt"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo"
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
	previousRevealed := maps.Clone(m.sidebarRevealed)
	revealed := make(map[int]bool)
	defer func() {
		m.sidebarRevealed = revealed
		if !maps.Equal(previousRevealed, revealed) {
			v.notify("sidebar-reveal-region")
		}
	}()
	previousTooltip := m.stackTooltip
	m.stackTooltip = mygo.Rectangle{}
	defer func() {
		if previousTooltip != m.stackTooltip {
			v.notify("tooltip-region")
		}
	}()
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
		markerColor := t.Accent
		if task.Priority == 2 {
			markerColor = t.Warning
		} else if task.Priority == 3 {
			markerColor = t.Danger
		}
		reveal := ui.Local(c.Root(), fmt.Sprintf("sidebar-reveal-%d", i), func() edgeTitleReveal { return edgeTitleReveal{} })
		progress, animating := reveal.frame(m.expanded(i), c.Now(), c.Preferences().ReduceMotion || m.Dragging)
		presented := m.expanded(i) || progress > 0
		if presented {
			revealed[i] = true
		}
		if m.sidebarRevealed == nil {
			m.sidebarRevealed = make(map[int]bool)
		}
		m.sidebarRevealed[i] = presented
		r := m.markerRect(i)
		preview := m.previewRect(i)
		if animating {
			c.AnimationFrame()
		}
		x, width, bodyX, bodyWidth, contentX := sidebarRevealGeometry(m.Side, float32(r.Width), progress)
		if presented {
			ui.Box(c).Absolute().Left(x).Top(float32(r.Y)).Size(width, float32(m.itemHeight())).Radius(5).Border(1, desktopOutline(t)).Background(t.Surface)
		}
		handle := ui.Box(c.Key(fmt.Sprintf("handle-%d", i))).Absolute().Left(float32(r.X)).Top(float32(r.Y)).Size(float32(r.Width), float32(r.Height)).Radius(0, 5, 5, 0).Border(1, desktopOutline(t)).Background(t.Alt).TextColor(dueColor).Cursor(ui.CursorMove).Label(fmt.Sprintf("拖动任务标记 %d", i+1))
		if m.Side == "right" {
			handle.Radius(5, 0, 0, 5)
		}
		if presented {
			handle.Background(ui.Transparent).Border(0, ui.Transparent)
		}
		handle.PointerPosition()
		if v.pointer != nil {
			handle.HandleInput(func(ev ui.InputEvent) bool { return v.pointer(index, ev) })
		}
		handle.DrawOver(func(p *ui.Painter, r ui.Rect) {
			if task.Priority == 2 || task.Priority == 3 {
				// Two/three bars distinguish priority even without color.
				for n := 0; n < task.Priority; n++ {
					x := r.X + r.W/2 - float32(task.Priority*4-2)/2 + float32(n*4)
					p.Fill(ui.Rect{X: x, Y: r.Y + (r.H-18)/2, W: 2, H: 18}, markerColor, 1)
				}
			} else if handle.Hovered() {
				for _, dx := range []float32{r.W/2 - 3, r.W/2 + 1} {
					for _, dy := range []float32{r.H/2 - 6, r.H/2 - 1, r.H/2 + 4} {
						p.Fill(ui.Rect{X: r.X + dx, Y: r.Y + dy, W: 2, H: 2}, markerColor, 1)
					}
				}
			} else {
				p.Fill(ui.Rect{X: float32(math.Round(float64(r.X + r.W/2 - 1.5))), Y: float32(math.Round(float64(r.Y + (r.H-23)/2))), W: 3, H: 23}, markerColor, 1)
			}
			if dueLabel(task, m.now()) != "" {
				p.Fill(ui.Rect{X: r.X + r.W/2 - 2, Y: r.Y + r.H - 7, W: 4, H: 4}, dueColor, 2)
			}
		})
		if handle.Hovered() {
			nextHover = i
		}
		if !m.Dragging && presented {
			ui.Box(c.Key(fmt.Sprintf("preview-viewport-%d", i))).Absolute().Left(bodyX).Top(float32(preview.Y)).Size(bodyWidth, float32(preview.Height)).Clip().Children(func() {
				body := ui.ButtonBase(c.Key(fmt.Sprintf("preview-%d", i))).Label(fmt.Sprintf("打开任务 %d", i+1)).Absolute().Left(contentX).Top(0).Size(float32(preview.Width), float32(preview.Height)).Padding(0, 12).Gap(10).TextColor(t.Text).FocusRing(false)
				body.Children(func() {
					check := ui.ButtonBase(c).Size(16, 16).Radius(4).Border(1, t.Border).Label("完成侧栏任务：" + task.Title)
					stackTooltip(c, m, check, "标记为完成")
					if check.Clicked() {
						m.open(index)
						if m.complete() {
							v.notify("complete")
							v.closeCard()
						}
					}
					ui.Box(c).Grow(1).MinWidth(0).Height(18).ClipX().Children(func() {
						ui.Text(c, task.Title).WidthPercent(100).FontSize(13).Height(18).FixedLineHeight(18).MaxLines(1).Ellipsis("…")
					})
					if task.Priority == 2 || task.Priority == 3 {
						bg := t.WarningSoft
						if task.Priority == 3 {
							bg = t.DangerSoft
						}
						ui.Text(c, priorityName(task.Priority)).FontSize(10).NoWrap().Shrink(0).Padding(2, 4).Radius(3).TextColor(markerColor).Background(bg)
					}
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
			})
		}
	}
	overflowInside := v.webOverflowTab(c)
	if m.undoVisible() {
		r := m.undoRect()
		ui.Row(c.Key("undo-toast")).Absolute().Left(float32(r.X)).Top(float32(r.Y)).Size(float32(r.Width), float32(r.Height)).Padding(0, 10).Gap(12).Radius(t.Theme.Radius).Border(1, t.Border).Background(t.Surface).Children(func() {
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
	t := desktopSurfaceTheme(c, v.m)
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
	if v.renderPresence || m.Editing {
		v.presence(v.sourceInside, c.Root().Hovered())
	}
	item := m.Tasks[m.Opened]
	details := m.cardDetails
	pickerWasOpen := details != nil && (details.datePickerOpen || details.pickerCalendarOpen)
	editHeightBefore := m.cardEditorHeight()
	var reading ui.Element
	var errorHeight float32
	footerHeight := float32(61) // 36px controls + 24px padding + top border.
	cardSurface := t.Surface
	if t.Background.A > cardSurface.A {
		// A floating card has no main-window base coat behind its surface.
		cardSurface = t.Background
	}
	panel := ui.Column(c).Fill().Radius(t.Radius).Border(1, t.Border).Background(cardSurface)
	glassPanel(c, panel, t, t.Radius).Children(func() {
		ui.Row(c).Height(40).Shrink(0).Padding(10, 16, 2).Children(func() {
			label := "当前任务"
			if m.Editing {
				label = "快速编辑"
			}
			ui.Text(c, label).FontSize(11).TextColor(t.Subtle).Grow(1)
			if webIconButton(c, "关闭快速卡片", "close", 16).Radius(6).Clicked() {
				v.closeCard()
			}
		})
		if m.Opened < 0 {
			return
		}
		ui.Scroll(c.Key(fmt.Sprintf("card-body-%d-%t", m.Opened, m.Editing))).Grow(1).MinHeight(0).Children(func() {
			bodyContext := c
			if !m.Editing {
				width, _ := c.Size()
				// Bounds is the previous committed layout. A new key waits for
				// fresh metrics when content, theme or wrapping width changes.
				layout := webCardLayout{item.Title, item.Note, m.UITheme, dueText(item, m.now()), item.Priority, width}
				if layout != v.cardLayout {
					v.cardLayout, v.cardLayoutVersion = layout, v.cardLayoutVersion+1
				}
				// An integer key avoids formatting the entire note every frame.
				bodyContext = c.Key(v.cardLayoutVersion)
			}
			body := ui.Column(bodyContext).WidthPercent(100).MinWidth(0).Padding(10, 16, 16)
			// Track the entrance on the stable panel, not the keyed reading body:
			// edits and text reflow must not restart the presentation animation.
			motion := ui.Local(panel, "card-enter", func() enterMotion { return enterMotion{} })
			progress, active := motion.frame(v.session.RequestRevision, c.Now(), c.Preferences().ReduceMotion)
			if active {
				c.AnimationFrame()
			}
			body.Opacity(.35 + .65*progress)
			if !m.Editing {
				reading = body.Label("卡片阅读内容")
			}
			body.Children(func() {
				if m.Editing {
					ui.Column(c.Key("card-title")).Gap(6).Margin(0, 0, 14).Children(func() {
						ui.Text(c, "任务标题").FontSize(11).Height(15).FixedLineHeight(15).TextColor(t.TextMuted)
						field := webTextInput(c, &m.Draft).Label("任务标题").Padding(9).Height(35).Background(t.Field)
						if v.editorFocusPending {
							field.Focus()
							v.editorFocusPending = false
						}
						if field.Submitted() && m.save() {
							v.notify("save")
						}
					})
					ui.Column(c.Key("card-note")).Gap(6).Children(func() {
						ui.Text(c, "备注").FontSize(11).Height(15).FixedLineHeight(15).TextColor(t.TextMuted)
						note := webTextArea(c, &m.DraftNote).Label("卡片备注").Height(64).Padding(9).LineHeight(1.6).Background(t.Field)
						if !note.Composing() && (note.Shortcut(ui.Super, ui.KeyEnter) || note.Shortcut(ui.Ctrl, ui.KeyEnter)) && m.save() {
							v.notify("save")
						}
					})
					if details != nil && m.Editing {
						details.visual = t
						ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(14, 0).Children(func() {
							ui.Text(c, "优先级").FontSize(12).TextColor(t.TextMuted)
							webPriorityRadios(c, &details.newPriority)
						})
						ui.Row(c).AlignItems(ui.Center).Margin(0, 0, 7).Children(func() {
							ui.Text(c, "预计完成时间").FontSize(12).TextColor(t.TextMuted).Grow(1)
							webCheckbox(c, &details.formRemind, "到时提醒", 12).Height(16).Disabled(details.formDue == "")
						})
						details.webDeadline(c)
					}
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
			errorHeight = ui.Text(c.Key(m.EditError), m.EditError).Shrink(0).Role(ui.RoleStatus).FontSize(12).TextColor(t.Danger).Padding(8, 16).Bounds().H
		}
		// Editing and reading actions must not share focus/click state: the
		// Save button occupies the same slot as Complete after a save.
		ui.Column(c.Key(fmt.Sprintf("card-actions-%d-%t", m.Opened, m.Editing))).Height(footerHeight).Shrink(0).Gap(6).Padding(12, 16).BorderColor(t.Border).BorderWidth(1, 0, 0, 0).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				if m.Editing {
					b := webCardButton(c, "保存", true).Disabled(strings.TrimSpace(m.Draft) == "")
					b.Children(func() { webIcon(c, "check", 14); webActionText(c, "保存") })
					if b.Clicked() && m.save() {
						v.notify("save")
					}
					b = webCardButton(c, "取消", false)
					b.Children(func() { ui.Text(c, "取消") })
					if b.Clicked() {
						m.cancel()
						v.notify("cancel")
					}
				} else {
					for _, action := range []struct {
						label, icon string
						primary     bool
					}{{"完成", "check", true}, {"编辑", "edit", false}} {
						b := webCardButton(c, action.label, action.primary)
						b.Children(func() { webIcon(c, action.icon, 14); webActionText(c, action.label) })
						if b.Clicked() {
							switch action.label {
							case "完成":
								if m.complete() {
									v.notify("complete")
									v.closeCard()
								}
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
		})
	})
	if reading.Valid() && !m.Editing && m.Opened >= 0 {
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
	if m.Editing && m.cardEditorHeight() != editHeightBefore {
		v.notify("card-size")
	}
	if !pickerWasOpen && c.Root().Shortcut(0, ui.KeyEscape) {
		if m.Editing {
			m.cancel()
			v.notify("cancel")
		} else {
			v.closeCard()
		}
	}
}
