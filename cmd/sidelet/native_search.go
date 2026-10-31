package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

func taskMatchesSearch(item taskEntry, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	return query == "" || strings.Contains(strings.ToLower(item.Title), query) || strings.Contains(strings.ToLower(item.Note), query)
}

func (v *nativeTasksView) taskVisible(item taskEntry) bool {
	return !item.Deleted && (v.filterAll || item.Done == v.showDone) && taskMatchesSearch(item, v.searchQuery)
}

func (v *nativeTasksView) updateSearch(query string) {
	if v.searchQuery == query {
		return
	}
	v.searchQuery = query
	v.moreID, v.deleteID = 0, 0
	*v.currentTaskScroll() = ui.ScrollState{}
	v.jumpToday = false
}

func (v *nativeTasksView) webSearch(c *ui.Context) {
	t := v.visual
	ui.Row(c.Key("task-search")).Height(40).Padding(0, 10).Gap(6).AlignItems(ui.Center).Background(t.Field).Radius(t.Theme.Radius).Margin(0, 0, 8).Children(func() {
		ui.Box(c).TextColor(t.TextMuted).Shrink(0).Children(func() { webIcon(c, "search", 16) })
		hint := "Ctrl+F"
		if runtime.GOOS == "darwin" {
			hint = "⌘F"
		}
		field := webTextInput(c.Key("query"), &v.searchText).Label("搜索任务").Placeholder("搜索任务名称或备注").Tooltip("搜索当前分类 · "+hint).Grow(1).MinWidth(0).Height(34).Padding(0, 4).Border(0, ui.Transparent).Background(ui.Transparent)
		if v.focusSearch {
			field.Focus()
			v.focusSearch = false
		}
		if !field.Composing() {
			v.updateSearch(v.searchText)
			if field.Shortcut(0, ui.KeyEscape) {
				v.searchText = ""
				v.updateSearch("")
			}
		}
		// Reserve the same trailing space in every state: typing must not move
		// the list or change the input width, including while using an IME.
		ui.Row(c.Key("search-status")).Width(86).Shrink(0).Gap(4).AlignItems(ui.Center).Justify(ui.End).Children(func() {
			if v.searchText != "" {
				count := 0
				for _, item := range v.service.snapshot().Tasks {
					if v.taskVisible(item) {
						count++
					}
				}
				ui.Text(c, fmt.Sprintf("%d 项", count)).Label(fmt.Sprintf("当前分类找到 %d 项任务", count)).FontSize(11).TextColor(t.TextMuted).NoWrap()
				b := webIconButton(c, "清空搜索", "close", 14).Size(26, 26)
				if b.Clicked() {
					v.searchText = ""
					v.updateSearch("")
					v.focusSearch = true
					c.Invalidate()
				}
			} else {
				ui.Text(c, hint).FontSize(11).TextColor(t.TextMuted).NoWrap()
			}
		})
	})
}
