package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"slices"
)

type orderUI struct {
	Epoch   uint64
	FocusID int
	Status  string
}
type orderDrag struct {
	ID       int
	Previous []int
	Epoch    uint64
	Layout   string
}

func orderLayoutKey(m *model) string {
	return fmt.Sprintf("%t:%s:%d:%d:%d", m.Arranging, m.Side, m.itemHeight(), m.WorkWidth, m.WorkHeight)
}
func (o *orderUI) commit(s *TasksService, drag orderDrag, target int, after, keyboard bool) {
	if drag.Epoch != o.Epoch || drag.Layout != orderLayoutKey(s.m) {
		o.Status = "已取消拖动"
		return
	}
	_, err := s.Reorder(drag.Previous, moveOrder(drag.Previous, drag.ID, target, after))
	if err != nil {
		o.Status = err.Error()
		return
	}
	o.Status = "顺序已调整"
	if keyboard {
		o.FocusID = drag.ID
	}
}

// Two drop zones retain the release location even if the pointer moves again
// before rendering. MyGo owns pointer capture, Escape and edge auto-scrolling.
func orderRow(c *ui.Context, s *TasksService, o *orderUI, id, position int, height float32) *ui.Element {
	t := webTheme(s.m.UITheme)
	item := s.m.Tasks[id-1]
	row := ui.Row(c).Key(fmt.Sprintf("order-task-%d", id)).Height(height).Gap(8).AlignItems(ui.Center).Radius(5).Border(1, t.Border).Background(t.Surface)
	row.Children(func() {
		handle := ui.ButtonBase(c).Key("handle").Label("拖动排序"+item.Title).Size(32, height).Shrink(0).TextColor(t.TextMuted).Cursor(ui.CursorGrab).Disabled(len(s.m.eligibleIDs()) < 2)
		payload := ui.Local(handle, "order-drag", func() orderDrag { return orderDrag{} })
		if !handle.Dragging() {
			*payload = orderDrag{id, slices.Clone(s.m.eligibleIDs()), o.Epoch, orderLayoutKey(s.m)}
		}
		handle.Drag(*payload).DrawOver(func(p *ui.Painter, r ui.Rect) {
			for _, dx := range []float32{-3, 2} {
				for _, dy := range []float32{-6, 0, 6} {
					p.Fill(ui.Rect{X: r.X + r.W/2 + dx, Y: r.Y + r.H/2 + dy, W: 2, H: 2}, t.TextMuted, 1)
				}
			}
		})
		if handle.Dragging() {
			row.Background(t.Alt)
		}
		if o.FocusID == id {
			handle.Focus()
			o.FocusID = 0
		}
		for _, direction := range []struct {
			key   ui.Key
			delta int
		}{{ui.KeyUp, -1}, {ui.KeyDown, 1}} {
			if handle.Shortcut(ui.Alt, direction.key) {
				ids := s.m.eligibleIDs()
				k := slices.Index(ids, id) + direction.delta
				if k >= 0 && k < len(ids) {
					o.commit(s, orderDrag{id, ids, o.Epoch, orderLayoutKey(s.m)}, ids[k], direction.delta > 0, true)
				}
			}
		}
		ui.Text(c, item.Title).Grow(1).MinWidth(0).FontSize(12).MaxLines(1)
		ui.Text(c, fmt.Sprint(position+1)).FontSize(10).TextColor(t.Subtle).Padding(0, 12, 0, 0)
		for _, after := range []bool{false, true} {
			zone := ui.Box(c).Key(fmt.Sprint("drop-", after)).Absolute().Left(32).Right(0).HeightPercent(50).Role(ui.RoleNone)
			if after {
				zone.Bottom(0)
			} else {
				zone.Top(0)
			}
			if drag, over := ui.DragOver[orderDrag](zone); over && drag.ID != id {
				zone.DrawOver(func(p *ui.Painter, r ui.Rect) {
					y := r.Y
					if after {
						y = r.Y + r.H - 3
					}
					p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: 3}, t.Accent, 1)
				})
			}
			if drag, dropped := ui.Drop[orderDrag](zone); dropped {
				o.commit(s, drag, id, after, false)
			}
		}
	})
	return row
}
func (v *nativeTasksView) webOrder(c *ui.Context, pad float32) {
	t := v.visual
	ui.Column(c).MinHeight(400).Padding(16, pad, 12).Radius(t.Radius).Border(1, t.Border).Background(t.Surface).Children(func() {
		ui.Text(c, "桌面任务顺序").Font(t.HeadingFont).FontSize(16).FontWeight(600)
		ui.Text(c, "拖动手柄调整顺序，松手即生效。Option + ↑ / ↓ 也可调整。已完成和暂时隐藏的任务保留原位。").FontSize(12).LineHeight(1.7).TextColor(t.TextMuted).Margin(8, 0, 14)
		ids := v.service.m.eligibleIDs()
		side := "右侧"
		if v.service.m.Side == "left" {
			side = "左侧"
		}
		ui.Text(c, fmt.Sprintf("%s桌面 · %d 项", side, len(ids))).FontSize(12).TextColor(t.TextMuted).Margin(0, 0, 10)
		ui.Column(c).Gap(6).Children(func() {
			for k, id := range ids {
				orderRow(c, v.service, &v.order, id, k, 44)
			}
		})
		if len(ids) == 0 {
			ui.Text(c, "暂无可整理的任务，请先固定任务到桌面。").FontSize(12).TextColor(t.TextMuted)
		}
		ui.Text(c, v.service.m.persistenceHint()).FontSize(11).TextColor(t.Subtle).Margin(16, 0, 0)
		if v.order.Status != "" {
			ui.Text(c, v.order.Status).FontSize(11).TextColor(t.TextMuted).Margin(10, 0, 0)
		}
	})
}
