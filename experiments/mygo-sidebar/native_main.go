package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

var priorityNames = []string{"普通", "重要", "紧急"}

func priorityName(p int) string {
	switch p {
	case 2:
		return "重要"
	case 3:
		return "紧急"
	default:
		return "普通"
	}
}

func priorityValue(name string) int {
	switch name {
	case "重要":
		return 2
	case "紧急":
		return 3
	default:
		return 0
	}
}

// Drafts live outside render frames. Unrelated task mutations never reset an
// input's identity or its IME composition; saves still require its old version.
type nativeTasksView struct {
	service                       *TasksService
	newTitle, newPriority         string
	editID                        int
	editVersion                   uint64
	draft, draftPriority          string
	showDone, focusEdit, focusNew bool
	status                        string
	failed                        bool
	scroll                        ui.ScrollState
	theme                         *ui.Theme
	visual                        *visualTheme
	settingsOpen, optionsOpen     bool
	formNote                      string
	filterAll                     bool
	formDue                       string
	originalDue                   int64
	originalDueText               string
	datePickerOpen                bool
	pickerCalendarOpen            bool
	pickerDate                    time.Time
	pickerHour, pickerMinute      string
	pickerError                   string
	formRemind, formTemporary     bool
	positionOpen                  bool
	moreID, deleteID              int
	returnMoreFocus               int
	menuFocus                     bool
	onHide, onQuickAdd            func()
	onThemeChanged                func(string)
	onExport                      func(string)
	formPin                       bool
	order                         orderUI
}

func newNativeTasksView(s *TasksService) *nativeTasksView {
	t := ui.LightTheme()
	t.Background, t.Surface = ui.Hex("#f7f8f4"), ui.Hex("#ffffff")
	t.SurfaceHover, t.SurfacePressed = ui.Hex("#edf1e9"), ui.Hex("#e3e9df")
	t.Text, t.TextMuted, t.Border = ui.Hex("#303b32"), ui.Hex("#7b847b"), ui.Hex("#dce3d9")
	t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#57745d"), ui.Hex("#49664f"), ui.Hex("#3f5945")
	t.Selection, t.Focus = ui.RGBA(87, 116, 93, .2), ui.RGBA(87, 116, 93, .5)
	return &nativeTasksView{service: s, newPriority: "普通", theme: t}
}

func (v *nativeTasksView) result(err error, success string) bool {
	v.failed = err != nil
	if err != nil {
		v.status = err.Error()
		return false
	}
	v.status = success
	return true
}

func (v *nativeTasksView) add() {
	_, err := v.service.Add(v.newTitle, priorityValue(v.newPriority))
	if v.result(err, "已添加任务。") {
		v.newTitle, v.newPriority = "", "普通"
		v.focusNew = true
	}
}

func (v *nativeTasksView) beginEdit(t taskEntry) {
	v.editID, v.editVersion = t.ID, t.Version
	v.draft, v.draftPriority = t.Title, priorityName(t.Priority)
	v.formNote = t.Note
	v.formDue = localDue(t.DueAt, v.service.m.now().Location())
	v.originalDue, v.originalDueText = t.DueAt, v.formDue
	v.formRemind, v.formTemporary = t.Remind, t.Temporary
	v.datePickerOpen = false
	v.pickerCalendarOpen = false
	v.optionsOpen = t.Priority != 0 || t.DueAt != 0 || t.Temporary || t.Remind
	v.focusEdit, v.status, v.failed = true, "", false
}

func (v *nativeTasksView) cancelEdit() {
	v.editID, v.editVersion = 0, 0
	v.draft, v.draftPriority = "", ""
	v.focusEdit = false
	v.formNote = ""
	v.optionsOpen = false
	v.resetScheduleDraft()
}

func (v *nativeTasksView) save() {
	_, err := v.service.Update(v.editID, v.draft, priorityValue(v.draftPriority), v.editVersion)
	if v.result(err, "已保存修改。") {
		v.cancelEdit()
	}
}

func (v *nativeTasksView) reconcile() {
	if v.moreID > 0 && (v.moreID > len(v.service.m.Tasks) || v.service.m.Tasks[v.moreID-1].Deleted || !v.filterAll && v.service.m.Tasks[v.moreID-1].Done != v.showDone) {
		v.moreID, v.deleteID = 0, 0
	}
	if v.editID > 0 && (v.editID > len(v.service.m.Tasks) || v.service.m.Tasks[v.editID-1].Deleted || !v.filterAll && v.service.m.Tasks[v.editID-1].Done != v.showDone) {
		v.cancelEdit()
		v.status, v.failed = "任务状态已变化，已结束编辑。", false
	}
}

func (v *nativeTasksView) setFilter(done bool) {
	v.moreID, v.deleteID = 0, 0
	v.showDone = done
	v.cancelEdit()
	v.scroll = ui.ScrollState{}
	v.status, v.failed = "", false
}

func (v *nativeTasksView) renderLab(c *ui.Context) {
	c.SetTheme(v.theme)
	v.reconcile()
	snapshot := v.service.snapshot() // render runs on the same main thread.
	pending := v.service.m.activeCount()
	ui.Column(c).Fill().Padding(28, 30, 18).Gap(18).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Text(c, "SIDELET").FontSize(10).TextColor(v.theme.TextMuted)
				ui.Text(c, "把今天的事，放在手边。").FontSize(22).Bold()
			})
			ui.Text(c, fmt.Sprintf("%d 项待办", pending)).TextColor(v.theme.TextMuted)
		})
		ui.Row(c).Key("new-task").Gap(10).AlignItems(ui.Center).Children(func() {
			field := ui.TextInput(c, &v.newTitle).Label("新任务标题").Placeholder("添加一个要做的事…").Height(36).Grow(1).AutoFocus()
			if v.focusNew {
				field.Focus()
				v.focusNew = false
			}
			submit := field.Submitted()
			ui.Select(c, &v.newPriority, priorityNames).Label("新任务重要程度").MinWidth(90).Width(90).Height(36)
			if ui.PrimaryButton(c, "添加").Height(36).Clicked() || submit {
				v.add()
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, fmt.Sprintf("待办 %d", pending)).Disabled(!v.showDone).Clicked() {
				v.setFilter(false)
			}
			if ui.Button(c, fmt.Sprintf("已完成 %d", len(snapshot.Tasks)-pending)).Disabled(v.showDone).Clicked() {
				v.setFilter(true)
			}
			ui.Spacer(c)
			ui.Text(c, "普通 / 重要 / 紧急").FontSize(11).TextColor(v.theme.TextMuted)
		})
		ui.Box(c).Height(24).Children(func() {
			color := v.theme.Accent
			if v.failed {
				color = ui.Hex("#b86158")
			}
			ui.Text(c, v.status).FontSize(12).TextColor(color).MaxLines(1)
		})
		ui.Scroll(c).Key("task-list").Grow(1).TrackScroll(&v.scroll).Children(func() {
			count := 0
			for _, t := range snapshot.Tasks {
				if t.Done != v.showDone {
					continue
				}
				count++
				ui.Column(c).Key(fmt.Sprintf("task-%d", t.ID)).Padding(12, 0).Gap(10).Children(func() {
					if v.editID == t.ID {
						v.editor(c)
					} else {
						v.taskRow(c, t)
					}
					ui.Box(c).Height(1).Background(ui.Hex("#e7ebe4"))
				})
			}
			if count == 0 {
				text := "暂时没有待办，给自己留一点空白。"
				if v.showDone {
					text = "完成的任务会留在这里。"
				}
				ui.Text(c, text).TextColor(v.theme.TextMuted)
			}
		})
		ui.Row(c).Children(func() {
			ui.Text(c, "独立原型 · 任务在本次运行中保留").FontSize(11).TextColor(v.theme.TextMuted).Grow(1)
			ui.Text(c, "⌘1 打开窗口").FontSize(11).TextColor(v.theme.TextMuted)
		})
	})
	if v.editID > 0 && c.Root().Shortcut(0, ui.KeyEscape) {
		v.cancelEdit()
		v.status, v.failed = "已取消修改。", false
	}
}

func (v *nativeTasksView) editor(c *ui.Context) {
	ui.Box(c).Key("editor").Children(func() {
		field := ui.TextInput(c, &v.draft).Label("编辑任务标题").Height(36).AutoFocus()
		if v.focusEdit {
			field.Focus()
			v.focusEdit = false
		}
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Select(c, &v.draftPriority, priorityNames).Label("编辑任务重要程度").MinWidth(90).Width(90).Height(32)
		ui.Spacer(c)
		if ui.Button(c, "取消").Height(32).Clicked() {
			v.cancelEdit()
			v.status, v.failed = "已取消修改。", false
		}
		if ui.PrimaryButton(c, "保存").Height(32).Clicked() {
			v.save()
		}
	})
}

func (v *nativeTasksView) taskRow(c *ui.Context, t taskEntry) {
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		label := "完成"
		if t.Done {
			label = "恢复"
		}
		if ui.Button(c, label).Label(label + "：" + t.Title).Height(30).Clicked() {
			_, err := v.service.SetDone(t.ID, !t.Done, t.Version)
			v.result(err, "已更新任务状态。")
		}
		ui.Column(c).Grow(1).Gap(4).Children(func() {
			ui.Text(c, t.Title).FontSize(14).MaxLines(2)
			ui.Text(c, priorityName(t.Priority)).FontSize(11).TextColor(priorityColor(t.Priority))
		})
		if !t.Done {
			if ui.Button(c, "卡片").Label("打开卡片：" + t.Title).Height(30).Clicked() {
				v.result(v.service.Open(t.ID), "已打开侧边卡片。")
			}
		}
		if ui.Button(c, "编辑").Label("编辑：" + t.Title).Height(30).Clicked() {
			v.beginEdit(t)
		}
	})
}
