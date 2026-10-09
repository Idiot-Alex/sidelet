package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// Keep the earlier laboratory renderer for the historical comparison. The
// default main window now follows TodoManager / AppHeader / theme.css.
func (v *nativeTasksView) render(c *ui.Context) {
	if v.visual == nil {
		v.visual = webTheme(v.service.m.UITheme)
	}
	c.SetTheme(v.visual.Theme)
	v.reconcile()
	width, _ := c.Size()
	pad, gap, asideWidth, panelPad := float32(32), float32(24), float32(296), float32(20)
	if width <= 900 {
		pad, gap, asideWidth, panelPad = 24, 18, 270, 16
	}
	if width <= 650 {
		pad = 16
	}
	ui.Column(c).Fill().Padding(0, pad, 36).Children(func() {
		v.webHeader(c)
		ui.Scroll(c).Grow(1).Key("page").Padding(20, 0, 0).Children(func() {
			if v.service.m.storageError != "" {
				ui.Text(c, v.service.m.storageError).FontSize(12).TextColor(v.visual.Danger).Background(v.visual.DangerSoft).Padding(12, 16).Radius(v.visual.Theme.Radius).Margin(0, 0, 18)
			}
			if v.settingsOpen {
				v.webSettings(c)
				return
			}
			if v.failed && v.status != "" {
				ui.Text(c, v.status).FontSize(12).TextColor(v.visual.Danger).Background(v.visual.DangerSoft).Padding(12, 16).Radius(v.visual.Theme.Radius).Margin(0, 0, 18)
			}
			v.webNotificationStatus(c)
			v.webUndo(c)
			if width <= 650 {
				ui.Column(c).Gap(gap).Children(func() { v.webTaskList(c, panelPad); v.webForm(c, 17) })
			} else {
				ui.Row(c).Gap(gap).AlignItems(ui.Start).Children(func() {
					ui.Box(c).Grow(1).Children(func() { v.webTaskList(c, panelPad) })
					ui.Box(c).Width(asideWidth).Shrink(0).Children(func() {
						v.webForm(c, func() float32 {
							if width <= 900 {
								return 17
							}
							return 20
						}())
					})
				})
			}
		})
	})
	if !v.datePickerOpen && c.Root().Shortcut(0, ui.KeyEscape) {
		if v.service.m.Arranging {
			_, err := v.service.Arrange(false)
			v.result(err, "")
			v.order.Epoch++
			return
		}
		if v.moreID > 0 {
			v.returnMoreFocus = v.moreID
			v.moreID, v.deleteID = 0, 0
			c.Invalidate()
			return
		}
		if v.editID == 0 {
			return
		}
		v.cancelEdit()
		v.formNote = ""
		v.failed = false
		v.status = ""
	}
}

func (v *nativeTasksView) webHeader(c *ui.Context) {
	t := v.visual
	ui.Row(c).Height(64).Gap(20).BorderColor(t.Border).BorderWidth(0, 0, 1, 0).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(10).Grow(1).Children(func() {
			title := "我的任务"
			if v.settingsOpen {
				title = "设置"
			}
			ui.Text(c, title).Font(t.HeadingFont).FontSize(21).FontWeight(t.HeadingWeight).LineHeight(1.3).LetterSpacing(-.525)
			if !v.settingsOpen {
				ui.Text(c, fmt.Sprint(v.service.m.activeCount())).FontSize(11).FontWeight(500).LineHeight(1.3).Padding(3, 7).Radius(5).Background(t.Soft).TextColor(t.Accent)
			}
		})
		ui.Row(c).Gap(6).Children(func() {
			if v.settingsOpen {
				ui.Text(c, v.service.m.persistenceHint()).FontSize(11).TextColor(t.Subtle)
				b := webButton(c, "返回我的任务", false).Border(0, ui.Transparent).Gap(7).Disabled(v.exportBusy())
				b.Children(func() { webIcon(c, "back", 16); ui.Text(c, "返回我的任务") })
				if b.Clicked() {
					v.settingsOpen = false
				}
				return
			}
			b := webButton(c, "快速添加", false).Height(34).Border(0, ui.Transparent).Gap(6).Disabled(v.service.m.Arranging)
			b.Children(func() { webIcon(c, "plus", 16); ui.Text(c, "快速添加") })
			if b.Clicked() {
				if v.onQuickAdd != nil {
					v.onQuickAdd()
				}
			}
			label := "整理桌面"
			if v.service.m.Arranging {
				label = "完成整理"
			}
			b = webButton(c, label, false).Height(34).Border(0, ui.Transparent).Gap(6).Disabled(v.service.m.Quiet)
			if v.service.m.Arranging {
				b.Background(t.Soft).TextColor(t.Accent)
			}
			b.Children(func() { webIcon(c, "layout", 16); ui.Text(c, label) })
			if b.Clicked() {
				_, err := v.service.Arrange(!v.service.m.Arranging)
				if v.result(err, "") {
					v.moreID, v.deleteID = 0, 0
					v.datePickerOpen = false
					v.order.Epoch++
					v.order.Status = ""
				}
			}
			quietLabel := "安静模式"
			if v.service.m.Quiet {
				quietLabel = "恢复显示"
			}
			b = webButton(c, quietLabel, false).Height(34).Border(0, ui.Transparent).Gap(6)
			if v.service.m.Quiet {
				b.Background(t.Soft).TextColor(t.Accent)
			}
			b.Children(func() { webIcon(c, "moon", 16); ui.Text(c, quietLabel) })
			if b.Clicked() {
				_, err := v.service.SetQuiet(!v.service.m.Quiet)
				if v.result(err, "") {
					v.moreID, v.deleteID = 0, 0
					v.datePickerOpen = false
					v.order.Epoch++
					v.order.Status = ""
				}
			}
			ui.Box(c).Size(1, 18).Background(t.Border).Margin(0, 5)
			b = webIconButton(c, "设置", "settings", 18).Size(34, 34).Disabled(v.service.m.Arranging)
			if b.Clicked() {
				v.moreID, v.deleteID = 0, 0
				v.settingsOpen = true
			}
			b = webIconButton(c, "隐藏主窗口", "hide", 18).Size(34, 34)
			if b.Clicked() && v.onHide != nil {
				v.onHide()
			}
		})
	})
}

func (v *nativeTasksView) webTaskList(c *ui.Context, pad float32) {
	if v.service.m.Arranging {
		v.webOrder(c, pad)
		return
	}
	t := v.visual
	pending := v.service.m.activeCount()
	ui.Column(c).MinHeight(400).Padding(16, pad, 12).Radius(t.Radius).Border(1, t.Border).Background(t.Surface).Children(func() {
		ui.Row(c).Gap(4).Padding(0, 0, 15).BorderColor(t.Border).BorderWidth(0, 0, 1, 0).Children(func() {
			for _, f := range []struct {
				name      string
				count     int
				done, all bool
			}{{"待办", pending, false, false}, {"已完成", v.service.m.totalCount() - pending, true, false}, {"全部", v.service.m.totalCount(), false, true}} {
				selected := v.filterAll == f.all && (f.all || v.showDone == f.done)
				b := webButton(c, fmt.Sprintf("%s %d", f.name, f.count), false).Border(0, ui.Transparent).Padding(7, 12)
				if selected {
					b.Background(t.Soft).TextColor(t.Accent).FontWeight(550)
				}
				b.Children(func() { ui.Text(c, fmt.Sprintf("%s %d", f.name, f.count)) })
				if b.Clicked() {
					v.setFilter(f.done)
					v.filterAll = f.all
					v.formNote = ""
				}
			}
		})
		count := 0
		items := v.service.snapshot().Tasks
		visibleCount := 0
		for _, item := range items {
			if v.filterAll || item.Done == v.showDone {
				visibleCount++
			}
		}
		for _, item := range items {
			if !v.filterAll && item.Done != v.showDone {
				continue
			}
			count++
			row := ui.Column(c).Key(fmt.Sprintf("task-%d", item.ID)).Padding(12, 10).Margin(0, -10).Radius(6)
			if count < visibleCount {
				row.BorderColor(t.Border).BorderWidth(0, 0, 1, 0)
			}
			if row.Hovered() {
				row.Background(t.Alt)
			}
			if v.editID == item.ID {
				row.Background(t.Soft).DrawOver(func(p *ui.Painter, r ui.Rect) { p.Fill(ui.Rect{X: r.X, Y: r.Y + 6, W: 2, H: r.H - 12}, t.Accent, 1) })
			}
			showActions := row.Hovered() || row.FocusWithin() || v.editID == item.ID || v.moreID == item.ID
			row.Children(func() {
				v.webTaskRow(c, item, showActions)
			})
		}
		if count == 0 {
			ui.Column(c).Padding(50, 10).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(78, 72).Radius(12).Border(1, t.Border).Background(t.Field).Children(func() { webIcon(c, "check", 24) })
				title := "这里暂时没有任务。"
				if v.showDone {
					title = "还没有已完成的任务。"
				}
				ui.Text(c, title).Font(t.HeadingFont).FontSize(21).Margin(22, 0, 10)
				ui.Text(c, "可以切换筛选查看其他任务。").FontSize(12).TextColor(t.TextMuted)
			})
		}
		ui.Spacer(c)
		ui.Text(c, v.service.m.persistenceHint()).FontSize(11).TextColor(t.Subtle).Margin(20, 0, 0)
	})
}

func (v *nativeTasksView) webTaskRow(c *ui.Context, item taskEntry, showActions bool) {
	t := v.visual
	ui.Row(c).Gap(10).MinHeight(28).AlignItems(ui.Center).Children(func() {
		label := "完成：" + item.Title
		if item.Done {
			label = "恢复：" + item.Title
		}
		check := ui.ButtonBase(c).Label(label).Size(20, 20).Radius(10).Border(1, t.Subtle)
		if item.Done {
			check.Background(t.Accent).TextColor(t.AccentText).Border(1, t.Accent)
			check.Children(func() { webIcon(c, "check", 13) })
		}
		if check.Clicked() {
			_, err := v.service.SetDone(item.ID, !item.Done, item.Version)
			v.result(err, "")
		}
		b := ui.ButtonBase(c).Label("编辑："+item.Title).Grow(1).Padding(2, 0).Justify(ui.Start).TextColor(t.Text)
		b.Children(func() {
			x := ui.Text(c, item.Title).FontSize(14).FontWeight(550).LineHeight(1.5).MaxLines(2)
			if item.Done {
				x.TextColor(t.Subtle).Strikethrough()
			}
		})
		if b.Clicked() {
			v.beginEdit(item)
			v.formNote = item.Note
		}
		if item.DueAt > 0 {
			color := t.TextMuted
			if !item.Done && item.DueAt < v.service.m.now().UnixMilli() {
				color = t.Danger
			}
			ui.Row(c).Gap(4).TextColor(color).Children(func() { webIcon(c, "clock", 12); ui.Text(c, dueText(item.task, v.service.m.now())).FontSize(11) })
		}
		if item.Priority != 0 && !item.Done {
			fg, bg := t.Warning, t.WarningSoft
			if item.Priority == 3 {
				fg, bg = t.Danger, t.DangerSoft
			}
			ui.Text(c, priorityName(item.Priority)).FontSize(11).LineHeight(1.4).Padding(2, 5).Radius(4).TextColor(fg).Background(bg)
		}
		actions := ui.Row(c).Gap(2)
		if !showActions {
			actions.Opacity(0)
		}
		actions.Children(func() {
			b := webIconButton(c, "编辑任务："+item.Title, "edit", 15)
			if b.Clicked() {
				v.beginEdit(item)
				v.formNote = item.Note
			}
			label := "取消桌面固定：" + item.Title
			if item.Unpinned {
				label = "固定到桌面：" + item.Title
			}
			b = webIconButton(c, label, "pin", 15)
			if !item.Unpinned {
				b.TextColor(t.Accent)
			}
			if b.Clicked() {
				_, err := v.service.SetPinned(item.ID, item.Unpinned, item.Version)
				v.result(err, "")
			}
			more := webIconButton(c, "更多操作："+item.Title, "more", 17)
			if v.returnMoreFocus == item.ID {
				more.Focus()
				v.returnMoreFocus = 0
			}
			if more.Clicked() {
				if v.moreID == item.ID {
					v.moreID = 0
				} else {
					v.moreID = item.ID
					v.menuFocus = true
				}
				v.deleteID = 0
			}
		})
	})
	if item.Note != "" {
		ui.Text(c, item.Note).FontSize(12).LineHeight(1.5).TextColor(t.TextMuted).MaxLines(2).Margin(3, 0, 0, 31)
	}
	if !item.Unpinned || item.Remind && !item.Done || item.Temporary || item.SnoozedUntil > v.service.m.now().UnixMilli() {
		ui.Row(c).Wrap().Gap(10).AlignItems(ui.Center).Margin(3, 0, 0, 31).TextColor(t.TextMuted).Children(func() {
			if !item.Unpinned {
				ui.Row(c).Gap(3).Children(func() { webIcon(c, "pin", 11); ui.Text(c, "桌面").FontSize(11).LineHeight(1.5) })
			}
			if item.Remind && !item.Done {
				label := "系统提醒已开启"
				if item.ReminderSentAt > 0 {
					label = "提醒已交给系统"
				}
				ui.Text(c, label).FontSize(11)
			}
			if item.Temporary {
				ui.Text(c, "临时").FontSize(11)
			}
			if item.SnoozedUntil > v.service.m.now().UnixMilli() {
				ui.Text(c, "暂时隐藏至 "+time.UnixMilli(item.SnoozedUntil).In(time.Local).Format("1/2 15:04")).FontSize(11)
				b := webButton(c, "现在恢复", false).Border(0, ui.Transparent).Padding(1, 4)
				b.Children(func() { ui.Text(c, "现在恢复") })
				if b.Clicked() {
					_, err := v.service.Unsnooze(item.ID, item.Version)
					v.result(err, "")
				}
			}
		})
	}
	if v.moreID == item.ID {
		v.webMore(c, item)
	}
}

func (v *nativeTasksView) webForm(c *ui.Context, pad float32) {
	t := v.visual
	ui.Column(c).Padding(pad).Radius(t.Radius).Border(1, t.Border).Background(t.Surface).Disabled(v.service.m.Arranging).Children(func() {
		ui.Row(c).Gap(9).Margin(0, 0, 22).Children(func() {
			icon, title := "plus", "新建任务"
			if v.editID > 0 {
				icon, title = "list", "编辑任务"
			}
			ui.Box(c).TextColor(t.Accent).Children(func() { webIcon(c, icon, 18) })
			ui.Text(c, title).Font(t.HeadingFont).FontSize(16).FontWeight(600).LineHeight(1.375)
		})
		ui.Column(c).Key("task-form").Gap(7).Children(func() {
			ui.Text(c, "标题").FontSize(12).TextColor(t.TextMuted)
			value, label := &v.newTitle, "新任务标题"
			if v.editID > 0 {
				value, label = &v.draft, "编辑任务标题"
			}
			field := ui.TextInput(c, value).Label(label).Placeholder("接下来想做什么？").Padding(10, 11).Radius(t.Theme.Radius).Background(t.Field).Height(38)
			if v.focusNew && v.editID == 0 || v.focusEdit && v.editID > 0 {
				field.Focus()
				v.focusNew, v.focusEdit = false, false
			}
			if field.Submitted() {
				v.webSubmit()
			}
			ui.Text(c, "备注").FontSize(12).TextColor(t.TextMuted).Margin(16, 0, 0)
			ui.TextArea(c, &v.formNote).Label("任务备注").Placeholder("补充要点，或留空").Height(84).Padding(10, 11).Radius(t.Theme.Radius).Background(t.Field).LineHeight(1.6)
		})
		if v.editID == 0 {
			webCheckbox(c, &v.formPin, "固定到桌面", 12, "pin").Margin(16, 0, 16)
		}
		b := webButton(c, "截止时间与更多选项", false).Border(0, ui.Transparent).Padding(15, 0).Margin(2, 0, 0).BorderColor(t.Border).BorderWidth(1, 0, 0, 0).Gap(6).Justify(ui.Start)
		b.Children(func() { webIcon(c, "chevron", 10); ui.Text(c, "截止时间与更多选项") })
		if b.Clicked() {
			v.optionsOpen = !v.optionsOpen
		}
		if v.optionsOpen {
			ui.Column(c).Gap(7).Margin(0, 0, 16).Children(func() {
				ui.Text(c, "截止时间").FontSize(12).TextColor(t.TextMuted)
				v.webDeadline(c)
				webCheckbox(c, &v.formRemind, "提醒我", 12).Disabled(v.formDue == "")
				hint := "默认只显示到期状态；设置截止时间后可开启通知。"
				if v.formRemind {
					hint = "到截止时间发送系统通知；Sidelet 需在后台运行。"
				}
				ui.Text(c, hint).FontSize(12).TextColor(t.TextMuted).LineHeight(1.7).Margin(0, 0, 14)
				ui.Text(c, "重要程度").FontSize(12).TextColor(t.TextMuted)
				value := &v.newPriority
				if v.editID > 0 {
					value = &v.draftPriority
				}
				ui.Select(c, value, priorityNames).Label("任务重要程度").Height(36).Background(t.Field)
				webCheckbox(c, &v.formTemporary, "临时任务", 12).Margin(12, 0, 0)
				if v.formTemporary {
					ui.Text(c, "完成后保留 5 秒撤销时间，随后自动移除。").FontSize(12).TextColor(t.TextMuted).LineHeight(1.7)
				}
			})
		}
		ui.Row(c).Gap(8).Children(func() {
			text, value := "添加任务", v.newTitle
			if v.editID > 0 {
				text, value = "保存修改", v.draft
			}
			b := webButton(c, text, true).Grow(1).Padding(10, 8).Disabled(value == "")
			b.Children(func() { ui.Text(c, text) })
			if b.Clicked() {
				v.webSubmit()
			}
			if v.editID > 0 {
				b = webButton(c, "取消编辑", false).Grow(1).Padding(10, 8)
				b.Children(func() { ui.Text(c, "取消编辑") })
				if b.Clicked() {
					v.cancelEdit()
					v.formNote = ""
					v.optionsOpen = false
					v.failed = false
					v.status = ""
				}
			}
		})
	})
	v.webPosition(c)
}

func (v *nativeTasksView) webSubmit() {
	due, err := parseDue(v.formDue, v.originalDue, v.originalDueText, v.service.m.now().Location())
	if err != nil {
		v.result(err, "")
		return
	}
	if v.editID > 0 {
		_, err := v.service.SaveTask(v.editID, v.draft, v.formNote, priorityValue(v.draftPriority), false, due, v.formRemind, v.formTemporary, v.editVersion)
		if v.result(err, "") {
			v.cancelEdit()
			v.formNote = ""
			v.optionsOpen = false
		}
	} else {
		_, err := v.service.SaveTask(0, v.newTitle, v.formNote, priorityValue(v.newPriority), v.formPin, due, v.formRemind, v.formTemporary, 0)
		if v.result(err, "") {
			v.resetScheduleDraft()
			v.newTitle, v.formNote = "", ""
			v.newPriority = "普通"
			v.formPin = false
			v.optionsOpen = false
		}
	}
}

func (v *nativeTasksView) webAppearance(c *ui.Context) {
	t := v.visual
	ui.Column(c).Label("外观设置").WidthPercent(100).MaxWidth(850).Margin(0, ui.Auto).Padding(22, 24).Radius(t.Radius).Border(1, t.Border).Background(t.Surface).Children(func() {
		ui.Text(c, "外观").Font(t.HeadingFont).FontSize(16).FontWeight(600)
		ui.Text(c, "同一种风格，贯穿任务窗口与桌面卡片。").FontSize(12).LineHeight(1.6).TextColor(t.TextMuted).Margin(5, 0, 18)
		ui.Row(c).Gap(16).Children(func() {
			for _, item := range []struct{ id, name, description string }{{"mac", "精致 Mac", "柔和中性色，清爽专注"}, {"paper", "温暖纸色", "温润纸面，书写的节奏"}, {"graphite", "深色石墨", "低调深色，清晰有序"}} {
				preview := webTheme(item.id)
				b := ui.ButtonBase(c).Label(item.name).Grow(1).Column().Justify(ui.Start).AlignItems(ui.Stretch)
				b.Children(func() {
					frame := ui.Column(c).Height(118).Padding(0, 12).Radius(8).Border(1, preview.Border).Background(preview.Background)
					if t.ID == item.id {
						frame.DrawOver(func(p *ui.Painter, r ui.Rect) {
							p.Stroke(ui.Rect{X: r.X - 4, Y: r.Y - 4, W: r.W + 8, H: r.H + 8}, t.Accent, 11, 2)
						})
					}
					frame.Children(func() {
						ui.Row(c).Height(24).Gap(3).BorderColor(preview.Border).BorderWidth(0, 0, 1, 0).Children(func() {
							for i := 0; i < 3; i++ {
								ui.Box(c).Size(4, 4).Radius(2).Background(preview.Subtle).Opacity(.55)
							}
							ui.Spacer(c)
							ui.Text(c, "Aa").Font(preview.HeadingFont).FontSize(14).TextColor(preview.Text)
						})
						ui.Row(c).Gap(10).Padding(12, 0, 0).Children(func() {
							ui.Column(c).Grow(1).Gap(4).Children(func() {
								ui.Box(c).Size(40, 5).Radius(2).Background(preview.Text).Margin(0, 0, 5)
								for i := 0; i < 3; i++ {
									ui.Box(c).Height(13).Background(preview.Surface).Border(1, preview.Border).Radius(3).Children(func() { ui.Box(c).Absolute().Left(5).Top(3).Size(4, 4).Radius(2).Border(1, preview.Accent) })
								}
							})
							ui.Column(c).WidthPercent(30).Padding(7, 5).Gap(5).Border(1, preview.Border).Radius(4).Background(preview.Surface).Children(func() {
								for i := 0; i < 2; i++ {
									ui.Box(c).Height(6).Background(preview.Alt)
								}
								ui.Box(c).Height(7).Background(preview.Accent).Margin(4, 0, 0)
							})
						})
					})
					ui.Row(c).Margin(13, 0, 0).Children(func() {
						ui.Text(c, item.name).FontSize(13).FontWeight(550).Grow(1)
						mark := ui.Box(c).Size(17, 17).Radius(8.5).Border(1, t.Border)
						if t.ID == item.id {
							mark.Background(t.Accent).TextColor(t.AccentText).Children(func() { webIcon(c, "check", 12) })
						}
					})
					ui.Text(c, item.description).FontSize(11).TextColor(t.TextMuted).Margin(4, 0, 0)
				})
				if b.Clicked() {
					_, err := v.service.Theme(item.id)
					if v.result(err, "") {
						v.visual = preview
						if v.onThemeChanged != nil {
							v.onThemeChanged(item.id)
						}
					}
				}
			}
		})
		ui.Row(c).Gap(28).AlignItems(ui.Center).BorderColor(t.Border).BorderWidth(1, 0, 0, 0).Margin(20, 0, 0).Padding(20, 0, 0).Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, "在 Dock 中显示").FontSize(13).FontWeight(500)
				ui.Text(c, "点击图标打开任务窗口。关闭后仍可从顶部菜单栏进入。").FontSize(12).LineHeight(1.6).TextColor(t.TextMuted).Margin(5, 0)
			})
			on := v.service.m.Preferences.Appearance.ShowDockIcon
			if webSwitch(c, &on, "在 Dock 中显示").Disabled(!dockPreferenceAvailable() || v.service.applyDock == nil).Changed() {
				value := v.service.m.Preferences
				value.Appearance.ShowDockIcon = on
				_, err := v.service.SavePreferences(value)
				v.result(err, "")
			}
		})
	})
}
