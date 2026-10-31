package main

import (
	"fmt"
	"runtime"

	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// A quiet, single-surface task list. The keyed editor keeps its input state
// when it opens alongside the list or reflows on smaller windows.
func (v *nativeTasksView) render(c *ui.Context) {
	defer v.autosaveNewTaskDraft(c)
	v.visual = webTheme(v.service.m.UITheme)
	c.SetTheme(v.visual.Theme)
	c.Root().Background(v.visual.Surface)
	v.reconcile()
	if !v.settingsOpen && !v.workLogOpen && !v.service.m.Arranging && !v.datePickerOpen {
		mods := ui.Ctrl
		if runtime.GOOS == "darwin" {
			mods = ui.Super
		}
		if c.Root().Shortcut(mods, ui.KeyF) {
			v.focusSearch = true
		}
	}
	width, _ := c.Size()
	pad, bottom, gap, asideWidth, panelPad := float32(32), float32(20), float32(28), float32(320), float32(0)
	if width <= 900 {
		pad, gap, asideWidth, panelPad = 28, 24, 288, 0
		bottom = 16
	}
	if width <= 650 {
		pad = 16
		bottom = 24
	}
	ui.Column(c).Fill().Padding(0, pad, bottom).Children(func() {
		if bar := c.TitleBar(); bar.Height > 0 {
			ui.Row(c).Height(bar.Height).Shrink(0).Padding(0, bar.Right, 0, bar.Left).DragWindow().Children(func() {
				ui.Text(c, v.service.m.applicationTitle()).FontSize(12).FontWeight(500).TextColor(v.visual.TextMuted)
			})
		}
		v.webHeader(c)
		if v.service.m.demoInstance {
			ui.Text(c, "演示实例 · 独立数据，不影响日常任务").FontSize(12).TextColor(v.visual.Accent).Margin(0, 0, 4).Shrink(0)
		}
		v.webDraftSaveError(c)
		if v.settingsOpen {
			v.webSettingsNavigation(c)
		}
		if !v.settingsOpen && !v.workLogOpen && !v.service.m.Arranging {
			v.webTaskWorkspace(c, width, gap, asideWidth)
			return
		}
		pageKey := "page"
		if v.workLogOpen {
			pageKey = "work-log"
		}
		if v.settingsOpen {
			pageKey = "settings-" + v.settingsTab
		}
		page := ui.Scroll(c.Key(pageKey)).Grow(1).Padding(16, 0, 0)
		if pageKey == "page" {
			page.TrackScroll(&v.scroll)
		}
		page.Children(func() {
			if v.service.m.storageError != "" {
				ui.Text(c, v.service.m.storageError).FontSize(12).TextColor(v.visual.Danger).Background(v.visual.DangerSoft).Padding(12, 16).Radius(v.visual.Theme.Radius).Margin(0, 0, 18)
			}
			if v.workLogOpen {
				v.webWorkLog(c)
				return
			}
			if v.settingsOpen {
				v.webSettings(c)
				return
			}
			v.webOrder(c, panelPad)
		})
	})
	if !v.datePickerOpen && c.Root().Shortcut(0, ui.KeyEscape) {
		if v.workLogOpen {
			v.workLogOpen = false
			return
		}
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
		v.failed = false
		v.status = ""
	}
}

// The list owns its viewport; the editor scrolls separately, so keeping search
// visible does not push editor fields or save actions below a taller toolbar.
func (v *nativeTasksView) webTaskWorkspace(c *ui.Context, width float32, gap, asideWidth float32) {
	t := v.visual
	ui.Column(c.Key("task-workspace")).Grow(1).MinHeight(0).Padding(16, 0, 0).Children(func() {
		if v.service.m.storageError != "" {
			ui.Text(c, v.service.m.storageError).FontSize(12).TextColor(t.Danger).Margin(0, 0, 10)
		}
		if v.failed && v.status != "" {
			ui.Text(c, v.status).FontSize(12).TextColor(t.Danger).Margin(0, 0, 10)
		}
		v.webNotificationStatus(c)
		v.webUndo(c)
		editorOpen := v.newFormOpen || v.editID > 0
		layout := ui.Row(c.Key("tasks-layout")).Grow(1).MinHeight(0).WidthPercent(100).MaxWidth(1080).Margin(0, ui.Auto).Gap(gap).AlignItems(ui.Stretch)
		if width <= 650 {
			layout.Column()
			if editorOpen {
				layout.Reverse()
			}
		}
		layout.Children(func() {
			ui.Column(c.Key("task-list-pane")).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
				ui.Column(c.Key("task-toolbar")).Shrink(0).Children(func() { v.webTaskToolbar(c) })
				ui.Scroll(c.Key(fmt.Sprintf("task-scroll-%d", v.taskTimelineMode()))).Label("任务时间轴").Grow(1).MinHeight(0).TrackScroll(v.currentTaskScroll()).Children(func() { v.webTaskList(c, 0) })
			})
			if editorOpen {
				pane := ui.Box(c.Key("task-editor-pane")).Shrink(0)
				if width > 650 {
					pane.Width(asideWidth)
				} else {
					pane.WidthPercent(100).HeightPercent(60)
				}
				pane.Children(func() {
					box := ui.Box(c).Fill()
					enterSurface(c, box, uint64(v.editID)).Children(func() { v.webForm(c, 18) })
				})
			}
		})
		text := "自动保存在本机"
		if v.service.m.profile == nil {
			text = "合成任务 · 不保存"
		}
		ui.Text(c, text).FontSize(11).TextColor(t.TextMuted).Margin(12, 0, 0).Shrink(0)
	})
}

func (v *nativeTasksView) webHeader(c *ui.Context) {
	t := v.visual
	ui.Row(c).Height(68).Gap(20).TextColor(t.Text).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(12).Grow(1).AlignItems(ui.Center).Children(func() {
			title := "我的任务"
			if v.workLogOpen {
				title = "工作日志"
			}
			if v.settingsOpen {
				title = "设置"
			}
			ui.Text(c, title).Font(t.HeadingFont).FontSize(23).FontWeight(600).LineHeight(1.3)
		})
		ui.Row(c).Gap(6).Children(func() {
			if v.settingsOpen || v.workLogOpen {
				b := webButton(c, "返回我的任务", false).TextColor(v.visual.Text).Border(0, ui.Transparent).Gap(7).Disabled(v.exportBusy())
				b.Children(func() { webIcon(c, "back", 16); webActionText(c, "返回我的任务") })
				if b.Clicked() {
					v.settingsOpen = false
					v.workLogOpen = false
				}
				return
			}
			var b ui.Element
			if v.service.m.Arranging {
				b = webIconTextButton(c.Key("finish-arrange"), "完成整理", "完成整理", "check", 16).Height(34).Background(t.Soft).TextColor(t.Accent)
				if b.Clicked() {
					v.setArranging(false)
				}
			}
			if v.service.m.Quiet {
				b = webIconTextButton(c.Key("show-labels"), "显示标签", "显示标签", "eye", 16).Height(34).Background(t.Soft).TextColor(t.Accent)
				if b.Clicked() {
					v.setLabelsHidden(false)
				}
			}
			b = webIconTextButton(c.Key("work-log"), "工作日志", "工作日志", "list", 17).TextColor(t.Text).Height(34).Disabled(v.service.m.Arranging)
			if b.Clicked() {
				v.workLogOpen, v.workLogCopied = true, ""
				v.moreID, v.deleteID = 0, 0
			}
			b = webIconTextButton(c.Key("settings"), "设置", "设置", "settings", 17).TextColor(v.visual.Text).Height(34).Disabled(v.service.m.Arranging)
			if b.Clicked() {
				v.moreID, v.deleteID = 0, 0
				v.settingsOpen = true
				if v.service.login != nil {
					_ = v.service.CheckLogin()
				}
			}
			webIconTextButton(c.Key("main-more"), "更多窗口操作", "更多", "more", 17).TextColor(v.visual.Text).Height(34).Menu(v.webWindowMenu)
		})
	})
}

func (v *nativeTasksView) webTaskToolbar(c *ui.Context) {
	t := v.visual
	pending := v.service.m.activeCount()
	ui.Row(c).Wrap().Gap(8).Padding(0, 0, 14).AlignItems(ui.Center).Children(func() {
		ui.Row(c.Key("task-filters")).Grow(1).Gap(4).Children(func() {
			for _, f := range []struct {
				name      string
				count     int
				done, all bool
			}{{"待办", pending, false, false}, {"已完成", v.service.m.totalCount() - pending, true, false}, {"全部", v.service.m.totalCount(), false, true}} {
				selected := v.filterAll == f.all && (f.all || v.showDone == f.done)
				b := webButton(c, fmt.Sprintf("%s %d", f.name, f.count), false).TextColor(t.TextMuted).Border(0, ui.Transparent).FontSize(13).Padding(7, 12)
				if selected {
					b.Background(t.Soft).TextColor(t.Accent).FontWeight(550)
				}
				b.Children(func() { ui.Text(c, fmt.Sprintf("%s %d", f.name, f.count)) })
				if b.Clicked() && !selected {
					v.setTaskFilter(f.done, f.all)
				}
			}
		})
		b := webButton(c, "回到今天", false).Height(34).Padding(0, 10).FontSize(12).Border(0, ui.Transparent).TextColor(t.TextMuted).Disabled(!v.hasTodayTasks())
		b.Children(func() { ui.Text(c, "回到今天") })
		if b.Clicked() {
			v.jumpToday = true
			c.Invalidate()
		}
		if !v.newFormOpen && v.editID == 0 {
			v.webNewTaskEntry(c)
		}
	})
	v.webSearch(c)
}

func (v *nativeTasksView) webTaskList(c *ui.Context, pad float32) {
	if v.service.m.Arranging {
		v.webOrder(c, pad)
		return
	}
	t := v.visual
	ui.Column(c).WidthPercent(100).Padding(0, pad, 4).Children(func() {
		v.deleteOutsideID, v.deleteTriggerPressed = 0, false
		prepared := v.preparedTimeline(v.service.m.now())
		count := prepared.count
		deleteAnchors := make(map[int]ui.Element)
		v.webTaskTimeline(c, prepared, deleteAnchors)
		if anchor, ok := deleteAnchors[v.moreID]; ok && v.moreID > 0 && v.moreID <= len(v.service.m.Tasks) {
			v.webDeleteConfirmation(c, taskEntry{ID: v.moreID, task: v.service.m.Tasks[v.moreID-1]}, anchor)
		}
		// Resolve outside dismissal after all delete triggers, so switching
		// tasks cannot dismiss the newly opened confirmation.
		if v.deleteOutsideID != 0 && v.moreID == v.deleteOutsideID && !v.deleteTriggerPressed {
			v.moreID, v.deleteID = 0, 0
		}
		if count == 0 {
			if strings.TrimSpace(v.searchQuery) != "" {
				ui.Column(c).Padding(40, 12).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "没有找到匹配的任务").FontSize(18).TextColor(t.Text)
					ui.Text(c, "试试其他关键词，或切换到「全部」。").FontSize(12).TextColor(t.TextMuted)
				})
				return
			}
			ui.Column(c).Padding(50, 10).AlignItems(ui.Center).Children(func() {
				v.webEmptyArt(c)
				title := "这里暂时没有任务。"
				if v.showDone {
					title = "还没有已完成的任务。"
				} else if v.service.m.totalCount() == 0 {
					title = "从一件小事开始"
				}
				ui.Text(c, title).TextColor(v.visual.Text).Font(t.HeadingFont).FontSize(21).FontWeight(600).LineHeight(1.375).Margin(22, 0, 10)
				text := "可以切换筛选查看其他任务。"
				if v.service.m.totalCount() == 0 {
					text = "写下接下来想做的事，让它留在视野里。"
				}
				ui.Text(c, text).FontSize(12).LineHeight(1.8).TextColor(t.TextMuted)
				if v.service.m.totalCount() == 0 {
					b := webButton(c, "写下第一件事", false).Border(0, ui.Transparent).TextColor(t.Accent).Gap(5).Margin(12, 0, 0)
					b.Children(func() { webIcon(c, "plus", 16); ui.Text(c, "写下第一件事") })
					if b.Clicked() {
						v.openNewTask()
						c.Invalidate()
					}
				}
			})
		}
	})
}

func (v *nativeTasksView) webNewTaskEntry(c *ui.Context) {
	b := webButton(c.Key("new-task-entry"), "新建任务", true).Height(34).Padding(0, 12).Gap(6).Shrink(0)
	if v.captureNewTaskDraft() != nil {
		b.Tooltip("继续新建草稿 · " + v.newTitle)
	}
	if v.focusNewEntry {
		b.Focus()
		v.focusNewEntry = false
	}
	b.Children(func() {
		webIcon(c, "plus", 17)
		caption := "新建任务"
		if v.captureNewTaskDraft() != nil {
			caption = "继续草稿"
		}
		webActionText(c, caption)
	})
	if b.Clicked() {
		v.openNewTask()
		c.Invalidate()
	}
}

func (v *nativeTasksView) webEmptyArt(c *ui.Context) {
	t := v.visual
	ui.Box(c).Size(78, 72).Radius(12).Border(1, t.Border).Background(t.Field).Children(func() {
		for i, width := range []float32{34, 24, 30} {
			ui.Box(c).Absolute().Left(14).Top(float32(21+i*13)).Size(width, 4).Radius(3).Background(t.Border)
		}
		ui.Box(c).Absolute().Right(-9).Bottom(-8).Size(30, 30).Radius(15).Border(2, t.Surface).Background(t.Soft).TextColor(t.Accent).AlignItems(ui.Center).Justify(ui.Center).Children(func() { webIcon(c, "check", 14) })
	})
}

func (v *nativeTasksView) webTaskRow(c *ui.Context, item taskEntry, showActions bool) ui.Element {
	t := v.visual
	var deleteAnchor ui.Element
	ui.Row(c).Gap(10).MinHeight(28).AlignItems(ui.Start).Children(func() {
		label := "完成：" + item.Title
		tip := "标记为完成"
		if item.Done {
			label = "恢复：" + item.Title
			tip = "恢复为未完成"
		}
		check := ui.ButtonBase(c).Label(label).Size(20, 20).Shrink(0).Margin(4, 0, 0).Radius(10).Border(1, t.Subtle).Tooltip(tip)
		if item.Done {
			check.Background(t.Accent).TextColor(t.AccentText).Border(1, t.Accent)
			check.Children(func() { webIcon(c, "check", 13) })
		}
		if check.Clicked() {
			_, err := v.service.SetDone(item.ID, !item.Done, item.Version)
			v.result(err, "")
		}
		b := ui.ButtonBase(c).Label("编辑："+item.Title).Grow(1).Shrink(1).MinWidth(0).Padding(2, 0).Justify(ui.Start).TextColor(t.Text)
		b.Children(func() {
			x := ui.Text(c, item.Title).FontSize(15).FontWeight(500).LineHeight(1.6).MaxLines(2)
			if item.Done {
				x.TextColor(t.Subtle).Strikethrough()
			}
		})
		if b.Clicked() {
			v.beginEdit(item)
		}
		if item.Priority != 0 && !item.Done {
			fg, bg := t.Warning, t.WarningSoft
			if item.Priority == 3 {
				fg, bg = t.Danger, t.DangerSoft
			}
			ui.Text(c, priorityName(item.Priority)).FontSize(11).LineHeight(1.4).NoWrap().Padding(3, 6).Margin(3, 0, 0).Radius(4).TextColor(fg).Background(bg).Shrink(0)
		}
		actions := ui.Row(c).Gap(2).Shrink(0)
		actions.Children(func() {
			b := webIconButton(c, "编辑任务："+item.Title, "edit", 15)
			if !showActions {
				b.Opacity(0)
			}
			if b.Clicked() {
				v.beginEdit(item)
			}
			label := "取消桌面固定：" + item.Title
			caption, icon := "移出桌面", "desktop-remove"
			if item.Unpinned {
				label = "固定到桌面：" + item.Title
				caption, icon = "放到桌面", "desktop-add"
			}
			b = webIconButton(c, label, icon, 15).Tooltip(caption)
			if !item.Unpinned {
				b.TextColor(t.Accent)
			} else if !showActions {
				b.Opacity(0)
			}
			if b.Clicked() {
				_, err := v.service.SetPinned(item.ID, item.Unpinned, item.Version)
				v.result(err, "")
			}
			more := webIconButton(c, "删除任务："+item.Title, "trash", 15).Tooltip("删除任务").TextColor(t.Subtle)
			deleteAnchor = more
			if more.Pressed() {
				v.deleteTriggerPressed = true
			}
			if more.Hovered() {
				more.TextColor(t.Danger)
			}
			if v.returnMoreFocus == item.ID {
				more.Focus()
				v.returnMoreFocus = 0
			}
			if more.Clicked() {
				if v.moreID == item.ID {
					v.moreID, v.deleteID = 0, 0
				} else {
					v.moreID, v.deleteID = item.ID, item.ID
					v.menuFocus = true
				}
			}
		})
	})
	if item.Note != "" {
		ui.Text(c, strings.ReplaceAll(item.Note, "\n", " ")).FontSize(13).LineHeight(1.5).TextColor(t.TextMuted).MaxLines(1).Ellipsis("…").Margin(2, 0, 0, 30)
	}
	v.webTaskStatus(c, item)
	return deleteAnchor
}

func (v *nativeTasksView) webTaskStatus(c *ui.Context, item taskEntry) {
	if !item.Done && item.DueAt == 0 && !item.Remind && !item.Temporary {
		return
	}
	t := v.visual
	status := ui.Row(c).Wrap().GapX(10).GapY(5).AlignItems(ui.Center).Margin(4, 0, 0, 30).TextColor(t.Subtle)
	status.Children(func() {
		if item.Done {
			label := "完成时间未记录"
			if item.CompletedAt > 0 {
				at := time.UnixMilli(item.CompletedAt).In(v.service.m.now().Location())
				format := "1/2 15:04"
				if at.Year() != v.service.m.now().Year() {
					format = "2006/1/2 15:04"
				}
				if v.showDone && !v.filterAll {
					format = "15:04"
				}
				label = "完成于 " + at.Format(format)
			}
			ui.Row(c).Gap(4).TextColor(t.TextMuted).Children(func() {
				webIcon(c, "check", 12)
				ui.Text(c, label).FontSize(12).LineHeight(4.0 / 3).FontFeatures("tnum").NoWrap()
			})
		}
		if item.DueAt > 0 && !(item.Done && v.showDone && !v.filterAll) {
			color := t.TextMuted
			if !item.Done && item.DueAt < v.service.m.now().UnixMilli() {
				color = t.Danger
			}
			ui.Row(c).Gap(4).TextColor(color).Children(func() {
				webIcon(c, "clock", 12)
				label := dueText(item.task, v.service.m.now())
				if item.Done {
					label = "计划 " + label
				}
				ui.Text(c, label).FontSize(12).LineHeight(4.0 / 3).FontFeatures("tnum").NoWrap()
			})
		}
		if item.Remind && !item.Done {
			label, tip := "提醒", "系统提醒已开启"
			if item.ReminderSentAt > 0 {
				label, tip = "已提醒", "提醒已交给系统"
			}
			ui.Text(c, label).FontSize(12).Tooltip(tip)
		}
		if item.Temporary {
			ui.Text(c, "临时").FontSize(12)
		}
	})
}

func (v *nativeTasksView) webForm(c *ui.Context, pad float32) {
	t := v.visual
	form := ui.Column(c).Fill().MinHeight(0).Padding(pad).Radius(t.Radius).Background(t.Field).Border(1, t.Border).Disabled(v.service.m.Arranging)
	glassPanel(c, form, t, t.Radius).Children(func() {
		ui.Scroll(c.Key("form-fields")).Label("任务编辑内容").Grow(1).MinHeight(0).Children(func() {
			ui.Row(c).Gap(9).Margin(0, 0, 16).Children(func() {
				icon, title := "plus", "新建任务"
				if v.editID > 0 {
					icon, title = "list", "编辑任务"
				}
				ui.Box(c).TextColor(t.Accent).Children(func() { webIcon(c, icon, 18) })
				ui.Text(c, title).Font(t.HeadingFont).FontSize(16).FontWeight(600).LineHeight(1.375)
			})
			if v.editID > 0 && v.editID <= len(v.service.m.Tasks) {
				item := v.service.m.Tasks[v.editID-1]
				if !item.Deleted && item.Done && item.CompletedAt > 0 {
					label := "完成于 " + time.UnixMilli(item.CompletedAt).In(v.service.m.now().Location()).Format("2006年1月2日 15:04")
					ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).Margin(0, 0, 14)
				}
			}
			ui.Column(c.Key(fmt.Sprintf("task-form-%d", v.editID))).Gap(7).Children(func() {
				ui.Text(c, "标题").FontSize(12).Height(14).FixedLineHeight(14).TextColor(t.TextMuted)
				value, label := &v.newTitle, "新任务标题"
				if v.editID > 0 {
					value, label = &v.draft, "编辑任务标题"
				}
				field := webTextInput(c.Key("title"), value).Label(label).Placeholder("接下来想做什么？").FontSize(15).Padding(10, 11).Radius(t.Theme.Radius).Background(t.Surface).Height(42)
				if v.focusNew && v.editID == 0 || v.focusEdit && v.editID > 0 {
					field.Focus()
					v.focusNew, v.focusEdit = false, false
				}
				if field.Submitted() {
					v.webSubmit()
				}
				v.webTaskNote(c, false, v.webSubmit)
			})
			ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(14, 0).Children(func() {
				ui.Text(c, "优先级").FontSize(12).TextColor(t.TextMuted)
				value := &v.newPriority
				if v.editID > 0 {
					value = &v.draftPriority
				}
				webPriorityRadios(c, value)
			})
			ui.Row(c).AlignItems(ui.Center).Margin(0, 0, 7).Children(func() {
				ui.Text(c, "预计完成时间").FontSize(12).TextColor(t.TextMuted).Grow(1)
				webCheckbox(c, &v.formRemind, "到时提醒", 12).Height(16).Disabled(v.formDue == "").Tooltip("在预计完成时间发送通知，需保持 Sidelet 运行")
			})
			v.webDeadline(c)
			b := webButton(c, "截止时间与更多选项", false).Border(0, ui.Transparent).Padding(15, 0).Margin(2, 0, 0).BorderColor(t.Border).BorderWidth(1, 0, 0, 0).Gap(6).Justify(ui.Start)
			b.Children(func() {
				marker := "▸"
				if v.optionsOpen {
					marker = "▾"
				}
				ui.Text(c, marker).FontSize(12).FixedLineHeight(16)
				ui.Text(c, "更多设置")
			})
			if b.Clicked() {
				v.optionsOpen = !v.optionsOpen
			}
			if !v.optionsOpen {
				v.webFormSummary(c)
			}
			if v.optionsOpen {
				ui.Column(c).Gap(7).Margin(0, 0, 16).Children(func() {
					webCheckbox(c, &v.formTemporary, "临时任务", 12).Margin(12, 0, 0).Tooltip("完成后自动移除，不保留完成记录")
					if v.formTemporary {
						ui.Text(c, "完成后保留 5 秒撤销时间，随后自动移除。").FontSize(12).TextColor(t.TextMuted).LineHeight(1.7)
					}
				})
			}
			if v.editID == 0 {
				webCheckbox(c, &v.formPin, "固定到桌面", 12, "desktop").TextColor(t.Text).Margin(0, 0, 16)
			}
		})
		ui.Row(c).Gap(8).Padding(12, 0, 0).Shrink(0).Children(func() {
			text, value := "添加任务", v.newTitle
			if v.editID > 0 {
				text, value = "保存修改", v.draft
			}
			b := webButton(c, text, true).Grow(1).Height(36).FixedLineHeight(14).Padding(10, 8).Disabled(value == "")
			b.Children(func() { ui.Text(c, text) })
			if b.Clicked() {
				v.webSubmit()
			}
			if v.editID > 0 {
				b = webButton(c, "取消编辑", false).Height(36).FixedLineHeight(14).Padding(10, 12).Border(0, ui.Transparent)
				b.Children(func() { ui.Text(c, "取消编辑") })
				if b.Clicked() {
					v.cancelEdit()
					v.failed = false
					v.status = ""
				}
			} else {
				b = webButton(c, "收起新建任务", false).Height(36).Padding(10, 12).Border(0, ui.Transparent)
				b.Children(func() { ui.Text(c, "收起") })
				if b.Clicked() {
					v.collapseNewTask()
				}
			}
		})
	})
}

func (v *nativeTasksView) webSubmit() {
	if err := v.commitOpenDatePicker(); err != nil {
		v.result(err, "")
		return
	}
	due, err := parseDue(v.formDue, v.originalDue, v.originalDueText, v.service.m.now().Location())
	if err != nil {
		v.result(err, "")
		return
	}
	if v.editID > 0 {
		_, err := v.service.SaveTask(v.editID, v.draft, v.formNote, priorityValue(v.draftPriority), false, due, v.formRemind, v.formTemporary, v.editVersion)
		if v.result(err, "") {
			v.cancelEdit()
		}
	} else {
		_, err := v.service.addNewTask(v.newTitle, v.formNote, priorityValue(v.newPriority), v.formPin, &taskSchedule{due, v.formRemind, v.formTemporary}, true)
		if v.result(err, "") {
			v.draftSaveAt, v.draftSaveError = time.Time{}, ""
			v.resetScheduleDraft()
			v.newTitle, v.formNote = "", ""
			v.newPriority = "普通"
			v.formPin = false
			v.noteOpen, v.focusNote = false, false
			v.optionsOpen = false
			v.newFormOpen = false
			v.defaultDue = ""
			v.focusNewEntry = true
		}
	}
}

// Keep selected options readable after collapsing the detailed controls.
// Incomplete dates stay explicit rather than being presented as valid times.
func (v *nativeTasksView) webFormSummary(c *ui.Context) {
	t := v.visual
	var parts []string
	if v.formDue != "" {
		if due, err := parseDue(v.formDue, v.originalDue, v.originalDueText, v.service.m.now().Location()); err == nil {
			parts = append(parts, time.UnixMilli(due).In(v.service.m.now().Location()).Format("1/2 15:04"))
		} else {
			parts = append(parts, "日期待完善")
		}
	}
	if v.formRemind {
		parts = append(parts, "提醒")
	}
	if v.formTemporary {
		parts = append(parts, "临时任务")
	}
	if len(parts) > 0 {
		ui.Box(c).Label("已选任务选项").Margin(-5, 0, 14).Children(func() {
			ui.Text(c, strings.Join(parts, " · ")).FontSize(12).LineHeight(1.6).TextColor(t.TextMuted)
		})
	}
}

func (v *nativeTasksView) webAppearance(c *ui.Context) {
	t := v.visual
	width, _ := c.Size()
	vertical, horizontal, gap := float32(22), float32(24), float32(16)
	if width <= 900 {
		vertical, horizontal, gap = 20, 20, 14
	}
	ui.Column(c).Label("外观设置").WidthPercent(100).MaxWidth(850).Margin(0, ui.Auto).Padding(vertical, horizontal).BorderColor(t.Border).BorderWidth(1, 0, 0, 0).Children(func() {
		headingHeight := float32(19)
		if t.ID == "paper" {
			headingHeight = 23
		}
		ui.Text(c, "外观").Font(t.HeadingFont).FontSize(16).FontWeight(600).Height(headingHeight).FixedLineHeight(headingHeight)
		ui.Row(c).Gap(gap).Margin(18, 0, 0).Children(func() {
			for _, item := range []struct{ id, name, description string }{{"mac", "精致 Mac", "柔和中性色，清爽专注"}, {"paper", "温暖纸色", "温润纸面，书写的节奏"}, {"graphite", "深色石墨", "低调深色，清晰有序"}} {
				preview := webTheme(item.id)
				b := ui.ButtonBase(c).Label(item.name).Grow(1).Column().Justify(ui.Start).AlignItems(ui.Stretch)
				b.Children(func() {
					frame := ui.Column(c).Height(96).Padding(0, 12).Radius(8).Border(1, preview.Border).Background(preview.Background)
					if b.Hovered() {
						frame.Opacity(.85)
					}
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
						})
						ui.Text(c, "Aa").Absolute().Right(15).Top(3).Font(preview.HeadingFont).FontSize(14).TextColor(preview.Text)
						ui.Row(c).Gap(10).AlignItems(ui.Stretch).Padding(10, 0, 0).Children(func() {
							ui.Column(c).Grow(1).Gap(4).Children(func() {
								ui.Box(c).WidthPercent(40).Height(5).Radius(2).Background(preview.Text).Margin(0, 0, 4)
								for i := 0; i < 3; i++ {
									ui.Box(c).Height(10).Background(preview.Surface).Border(1, preview.Border).Radius(3).Children(func() { ui.Box(c).Absolute().Left(5).Top(3).Size(4, 4).Radius(2).Border(1, preview.Accent) })
								}
							})
						})
					})
					ui.Row(c).Margin(13, 0, 0).Children(func() {
						ui.Text(c, item.name).FontSize(13).FontWeight(550).Height(16).FixedLineHeight(16).Grow(1)
						mark := ui.Box(c).Size(17, 17).Radius(8.5).Border(1, t.Border)
						if t.ID == item.id {
							mark.Background(t.Accent).TextColor(t.AccentText).Children(func() { webIcon(c, "check", 12) })
						}
					})
					ui.Text(c, item.description).FontSize(11).LineHeight(1.6).TextColor(t.TextMuted).Margin(4, 0, 5)
				})
				if b.Clicked() {
					_, err := v.service.Theme(item.id)
					if v.result(err, "") {
						v.visual = webTheme(v.service.m.UITheme)
						c.Invalidate()
						if v.onThemeChanged != nil {
							v.onThemeChanged(item.id)
						}
					}
				}
			}
		})
		if runtime.GOOS == "darwin" {
			ui.Row(c).Gap(28).AlignItems(ui.Center).BorderColor(t.Border).BorderWidth(1, 0, 0, 0).Margin(20, 0, 0).Padding(20, 0, 0).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					ui.Text(c, "在 Dock 中显示").FontSize(13).FontWeight(500).Height(16).FixedLineHeight(16)
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
		}
	})
}

func (v *nativeTasksView) webDesktopTransparency(c *ui.Context) {
	t := v.visual
	ui.Row(c).Gap(28).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, "悬浮任务半透明").FontSize(13).FontWeight(500)
			ui.Text(c, "让桌面悬浮卡片透出背景，主窗口始终使用实色。").FontSize(12).LineHeight(1.6).TextColor(t.TextMuted).Margin(5, 0)
		})
		on := v.service.m.Preferences.Appearance.DesktopTranslucent
		if webSwitch(c, &on, "启用悬浮任务半透明").Changed() {
			value := v.service.m.Preferences
			value.Appearance.DesktopTranslucent = on
			_, err := v.service.SavePreferences(value)
			v.result(err, "")
		}
	})
}
