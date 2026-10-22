package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Native menus supply keyboard navigation and dismissal on both platforms;
// recovery actions remain visible in the header while their mode is active.
func (v *nativeTasksView) webWindowMenu(menu *ui.Menu) {
	arrange := "整理桌面"
	if v.service.m.Arranging {
		arrange = "完成整理"
	}
	if menu.Item(arrange).Disabled(v.service.m.Quiet).Chosen() {
		v.setArranging(!v.service.m.Arranging)
	}
	quiet := "隐藏标签"
	if v.service.m.Quiet {
		quiet = "显示标签"
	}
	if menu.Item(quiet).Chosen() {
		v.setLabelsHidden(!v.service.m.Quiet)
	}
	menu.Separator()
	if menu.Item("隐藏窗口").Chosen() && v.onHide != nil {
		v.onHide()
	}
}

func (v *nativeTasksView) setArranging(on bool) {
	_, err := v.service.Arrange(on)
	if v.result(err, "") {
		v.resetWindowActionState()
	}
}

func (v *nativeTasksView) setLabelsHidden(hidden bool) {
	_, err := v.service.SetQuiet(hidden)
	if v.result(err, "") {
		v.resetWindowActionState()
	}
}

func (v *nativeTasksView) resetWindowActionState() {
	v.moreID, v.deleteID = 0, 0
	v.datePickerOpen = false
	v.order.Epoch++
	v.order.Status = ""
}

func (v *nativeTasksView) webUndo(c *ui.Context) {
	m, t := v.service.m, v.visual
	if m.UndoID == 0 || m.now().UnixMilli() >= m.UndoUntil {
		return
	}
	c.After(time.Duration(m.UndoUntil-m.now().UnixMilli()) * time.Millisecond)
	ui.Row(c).Gap(12).Padding(12, 16).Margin(0, 0, 18).Radius(t.Theme.Radius).Background(t.Soft).TextColor(t.Accent).Children(func() {
		ui.Text(c, "已完成「"+m.Tasks[m.UndoID-1].Title+"」").FontSize(12).Grow(1)
		b := webButton(c, "撤销完成", false).Background(ui.Transparent).Border(0, ui.Transparent).Padding(0)
		b.Children(func() { ui.Text(c, "撤销完成") })
		if b.Clicked() {
			_, err := v.service.Undo()
			v.result(err, "")
		}
	})
}
func (v *nativeTasksView) webMore(c *ui.Context, item taskEntry) {
	t := v.visual
	opening := v.menuFocus
	panel := ui.Column(c.Key("more-panel")).AlignSelf(ui.End).Width(220).Padding(6).Margin(7, 0, 0).Border(1, t.Border).Radius(t.Theme.Radius).Background(t.Surface)
	panel.Children(func() {
		if v.deleteID == item.ID {
			ui.Text(c, "删除此任务？").FontSize(12).Padding(5, 8, 9)
			ui.Row(c).Gap(6).Justify(ui.End).Children(func() {
				b := webButton(c, "取消删除", false).Padding(6, 9)
				b.Children(func() { ui.Text(c, "取消") })
				if v.menuFocus {
					b.Focus()
					v.menuFocus = false
				}
				if b.Clicked() {
					v.deleteID = 0
					v.menuFocus = true
				}
				b = webButton(c, "确认删除", false).Padding(6, 9).TextColor(t.Danger)
				b.Children(func() { ui.Text(c, "确认删除") })
				if b.Clicked() {
					_, err := v.service.Delete(item.ID, item.Version)
					if v.result(err, "") {
						v.moreID, v.deleteID = 0, 0
					}
				}
			})
		} else {
			b := webButton(c, "删除任务", false).Border(0, ui.Transparent).Padding(7, 8).Gap(6).Justify(ui.Start).TextColor(t.Danger)
			b.Children(func() { webIcon(c, "trash", 14); ui.Text(c, "删除任务") })
			if v.menuFocus {
				b.Focus()
				v.menuFocus = false
			}
			if b.Clicked() {
				v.deleteID = item.ID
				v.menuFocus = true
			}
		}
	})
	if !opening && panel.PressedOutside() {
		v.moreID, v.deleteID = 0, 0
	}
}
