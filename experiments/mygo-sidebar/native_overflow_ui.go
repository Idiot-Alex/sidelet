package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
)

func (v *views) webOverflowTab(c *ui.Context) bool {
	m := v.m
	_, tail := m.sidebarItems()
	if len(tail) == 0 {
		return false
	}
	t := webTheme(m.UITheme)
	r := m.overflowRect()
	if r.Height <= 0 {
		return false
	}
	face := ui.Row(c).Key("overflow-row").Absolute().Left(float32(r.X)).Top(float32(r.Y)).Size(float32(r.Width), float32(r.Height)).Radius(5, 0, 0, 5).Border(1, t.Border).Background(t.Alt)
	if m.Side == "left" {
		face.Radius(0, 5, 5, 0)
	}
	face.Children(func() {
		bodyX := r.X
		if m.Side == "left" && !m.Arranging {
			bodyX += 14
		}
		bodyWidth := r.Width
		if !m.Arranging {
			bodyWidth -= 14
		}
		button := ui.ButtonBase(c).Label(fmt.Sprintf("查看其余%d项任务", len(tail))).Absolute().Left(float32(bodyX-r.X)).Top(0).Size(float32(bodyWidth), float32(r.Height)).TextColor(t.TextMuted).FocusRing(false)
		button.Children(func() { ui.Text(c, fmt.Sprintf("+%d", len(tail))).FontSize(10) })
		if button.Clicked() {
			if m.Arranging {
				if v.showAll != nil {
					v.showAll()
				}
			} else if m.openOverflow() {
				if v.open != nil {
					v.open(overflowIndex)
				} else {
					v.beginSession(overflowIndex)
				}
				v.notify("overflow-open")
			}
		}
		if !m.Arranging {
			grip := m.markerRect(overflowIndex)
			handle := ui.Box(c).Key("overflow-grip").Label("移动整组任务：更多任务").Absolute().Left(float32(grip.X-r.X)).Top(0).Size(14, float32(r.Height)).Cursor(ui.CursorMove)
			handle.DrawOver(func(p *ui.Painter, r ui.Rect) {
				p.Fill(ui.Rect{X: r.X + r.W/2 - 1.5, Y: r.Y + (r.H-23)/2, W: 3, H: min(23, r.H)}, t.TextMuted, 1.5)
			})
			handle.PointerPosition()
			if v.pointer != nil {
				handle.HandleInput(func(ev ui.InputEvent) bool { return v.pointer(overflowIndex, ev) })
			}
		}
	})
	return face.Hovered()
}
func (v *views) webOverflowCard(c *ui.Context) {
	m := v.m
	_, tail := m.sidebarItems()
	if len(tail) == 0 {
		v.closeCard()
		return
	}
	t := webTheme(m.UITheme)
	if v.renderPresence {
		v.presence(v.sourceInside, c.Root().Hovered())
	}
	wanted := max(160, min(390, 66+len(tail)*42))
	if m.CardReadHeight != wanted {
		m.CardReadHeight = wanted
		v.notify("card-size")
	}
	ui.Column(c).Fill().Radius(t.Radius).Border(1, t.Border).Background(t.Surface).Children(func() {
		ui.Row(c).Height(40).Shrink(0).Padding(10, 16, 2).Children(func() {
			ui.Text(c, "更多任务").FontSize(11).LetterSpacing(.33).TextColor(t.TextMuted).Grow(1)
			if webIconButton(c, "关闭快速卡片", "close", 16).Clicked() {
				v.closeCard()
			}
		})
		ui.Scroll(c).Grow(1).Padding(10, 16, 16).Children(func() {
			for k, index := range tail {
				b := ui.ButtonBase(c).Key(fmt.Sprintf("overflow-task-%d", index+1)).Label("查看任务："+m.Tasks[index].Title).Gap(12).Padding(12, 0).TextColor(t.Text).Justify(ui.Start)
				if k < len(tail)-1 {
					b.BorderColor(t.Border).BorderWidth(0, 0, 1, 0)
				}
				b.Children(func() {
					ui.Text(c, m.Tasks[index].Title).Grow(1).MinWidth(0).FontSize(12).LineHeight(1.5)
					webIcon(c, "chevron", 14)
				})
				if b.Clicked() {
					m.open(index)
					if v.open != nil {
						v.open(index)
					} else {
						v.beginSession(index)
					}
					v.notify("overflow-choose")
					break
				}
			}
		})
	})
	if c.Root().Shortcut(0, ui.KeyEscape) {
		v.closeCard()
	}
}
func (v *views) webArrangeStack(c *ui.Context) {
	m := v.m
	t := webTheme(m.UITheme)
	s := v.service
	if s == nil {
		s = &TasksService{m: m, run: func(f func()) { f() }, changed: v.notify}
	}
	direct, _ := m.sidebarItems()
	for k, index := range direct {
		r := m.arrangeRowRect(index)
		orderRow(c, s, &v.order, index+1, k, float32(r.Height)).Absolute().Left(float32(r.X)).Top(float32(r.Y)).Width(float32(r.Width))
	}
	v.webOverflowTab(c)
	r := m.arrangeGripRect()
	grip := ui.Row(c).Key("arrange-grip").Label("移动整组任务").Absolute().Left(float32(r.X)).Top(float32(r.Y)).Size(float32(r.Width), float32(r.Height)).Radius(5).Border(1, t.Border).Background(t.Alt).Cursor(ui.CursorMove).Padding(0, 10).Gap(9).AlignItems(ui.Center)
	grip.Children(func() {
		ui.Box(c).Size(8, 18).DrawOver(func(p *ui.Painter, r ui.Rect) {
			for _, dx := range []float32{2, 6} {
				for _, dy := range []float32{4, 9, 14} {
					p.Fill(ui.Rect{X: r.X + dx - 1, Y: r.Y + dy - 1, W: 2, H: 2}, t.Text, 1)
				}
			}
		})
		label := "移动整组"
		if m.Dragging {
			label = "松手保存 · Esc 取消"
		}
		ui.Text(c, label).FontSize(12).Grow(1)
		ui.Text(c, "↔ ↕").FontSize(12).TextColor(t.TextMuted)
	})
	grip.PointerPosition()
	if v.pointer != nil {
		grip.HandleInput(func(ev ui.InputEvent) bool { return v.pointer(overflowIndex, ev) })
	}
	r = m.arrangeToolbarRect()
	ui.Column(c).Key("arrange-toolbar").Absolute().Left(float32(r.X)).Top(float32(r.Y)).Size(float32(r.Width), float32(r.Height)).Padding(9, 10).Gap(7).Radius(5).Border(1, t.Border).Background(t.Surface).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, "整理桌面").FontSize(12).FontWeight(500).Grow(1)
			b := webButton(c, "全部任务", false).FontSize(11).Padding(5).Border(0, ui.Transparent).Background(t.Alt).TextColor(t.Accent)
			b.Children(func() { ui.Text(c, "全部任务") })
			if b.Clicked() && v.showAll != nil {
				v.showAll()
			}
			b = webButton(c, "完成整理", false).FontSize(11).Padding(5).Border(0, ui.Transparent).Background(t.Alt).TextColor(t.Accent)
			b.Children(func() { ui.Text(c, "完成整理") })
			if b.Clicked() {
				_, err := s.Arrange(false)
				if err != nil {
					v.order.Status = err.Error()
				}
			}
		})
		label := v.order.Status
		if label == "" {
			label = "上方移动整组 · 行内手柄排序 · Esc 退出"
		}
		ui.Text(c, label).FontSize(10).LineHeight(1.4).TextColor(t.TextMuted)
	})
	if c.Root().Shortcut(0, ui.KeyEscape) {
		_, _ = s.Arrange(false)
		v.order.Epoch++
	}
}
