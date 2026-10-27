package main

import (
	"github.com/egoist/mygo/ui"
	"strings"
)

// The disclosure never discards the note; collapsed content stays inspectable.
func (v *nativeTasksView) webTaskNote(c *ui.Context, quick bool, submit func()) bool {
	t := v.visual
	changed := false
	label, fieldLabel := "展开备注", "任务备注"
	if v.noteOpen {
		label = "收起备注"
	}
	if quick {
		label, fieldLabel = "快速添加备注", "快速添加任务备注"
	}
	ui.Column(c).Margin(8, 0, 4).Children(func() {
		if !v.noteOpen {
			b := webButton(c, label, false).Border(0, ui.Transparent).Padding(4, 0).Justify(ui.Start).Gap(8).MinWidth(0).WidthPercent(100)
			b.Children(func() {
				if strings.TrimSpace(v.formNote) == "" {
					ui.Text(c, "＋ 添加备注").FontSize(12).TextColor(t.TextMuted)
				} else {
					ui.Text(c, "备注").FontSize(12).TextColor(t.Subtle).Shrink(0)
					ui.Text(c, strings.Join(strings.Fields(v.formNote), " ")).FontSize(12).TextColor(t.TextMuted).Grow(1).MinWidth(0).MaxLines(1).Ellipsis("…")
				}
			})
			if b.Clicked() {
				v.noteOpen, v.focusNote, changed = true, true, true
			}
		} else {
			ui.Row(c).AlignItems(ui.Center).Margin(0, 0, 6).Children(func() {
				ui.Text(c, "备注").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, "可选").FontSize(11).TextColor(t.Subtle).Margin(0, 0, 0, 6)
				ui.Spacer(c)
				b := webButton(c, label, false).Border(0, ui.Transparent).Padding(2, 0)
				b.Children(func() { ui.Text(c, "收起").FontSize(12) })
				if b.Clicked() {
					v.noteOpen, v.focusNote, changed = false, false, true
				}
			})
		}
		if v.noteOpen {
			note := webTextArea(c.Key("task-note"), &v.formNote).Label(fieldLabel).Placeholder("补充要点、链接或说明…").Height(64).Padding(8, 10).Border(1, t.Border).Radius(t.Theme.Radius).Background(t.Field).FontSize(13).LineHeight(1.5)
			if v.focusNote {
				note.Focus()
				v.focusNote = false
			}
			if !note.Composing() && (note.Shortcut(ui.Super, ui.KeyEnter) || note.Shortcut(ui.Ctrl, ui.KeyEnter)) {
				submit()
			}
		}
	})
	return changed
}
