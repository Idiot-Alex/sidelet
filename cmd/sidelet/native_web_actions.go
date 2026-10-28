package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Native menus supply keyboard navigation and dismissal on both platforms;
// recovery actions remain visible in the header while their mode is active.
func (v *nativeTasksView) webWindowMenu(menu *ui.Menu) {
	if menu.Item("快速添加").Disabled(v.service.m.Arranging).Chosen() && v.onQuickAdd != nil {
		v.onQuickAdd()
	}
	menu.Separator()
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
func (v *nativeTasksView) webDeleteConfirmation(c *ui.Context, item taskEntry, anchor ui.Element) {
	t := v.visual
	opening := v.menuFocus
	ui.Overlay(c, func() {
		panel := ui.Column(c.Key("delete-confirmation")).Role(ui.RolePopup).Label("删除任务确认").AttachTo(anchor, ui.AnchorLeft, ui.AnchorRight).Width(260).Gap(16).Padding(16).Margin(0, 8, 0, 0).Radius(t.Theme.Radius).Border(1, t.Border).Background(t.Surface).Shadow(0, 5, 18, 0, ui.Hex("#00000018"))
		panel.Children(func() {
			if v.deleteID == item.ID {
				ui.Column(c).Gap(4).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, "确定删除这条任务？").FontSize(14).FontWeight(550).TextColor(t.Text)
					ui.Text(c, "删除后无法恢复").FontSize(12).TextColor(t.TextMuted)
				})
				ui.Row(c).Gap(8).Justify(ui.End).AlignItems(ui.Center).Children(func() {
					b := webButton(c, "取消删除", false).Height(34).Padding(7, 14).Background(t.Surface).TextColor(t.Text)
					b.Children(func() { ui.Text(c, "取消") })
					if v.menuFocus {
						b.Focus()
						v.menuFocus = false
					}
					if b.Clicked() {
						v.moreID, v.deleteID = 0, 0
						v.returnMoreFocus = item.ID
					}
					b = webButton(c, "确认删除", false).Height(34).Padding(7, 14).Gap(6).Border(1, ui.Transparent).Background(t.DangerSoft).TextColor(t.Danger).FontWeight(550)
					if b.Hovered() || b.Pressed() {
						b.BorderColor(t.Danger)
					}
					b.Children(func() { webIcon(c, "trash", 14); ui.Text(c, "删除") })
					if b.Clicked() {
						_, err := v.service.Delete(item.ID, item.Version)
						if v.result(err, "") {
							v.moreID, v.deleteID = 0, 0
						}
					}
				})
			}
		})
		if !opening && panel.PressedOutside() {
			v.deleteOutsideID = item.ID
		}
	})
}
