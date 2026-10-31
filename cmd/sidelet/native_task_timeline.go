package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"
)

type taskDateGroup struct {
	key, title, detail string
	items              []taskEntry
}

type taskTimelineMode int

const (
	timelinePending taskTimelineMode = iota
	timelineCompleted
	timelineAll
)

func (v *nativeTasksView) taskTimelineMode() taskTimelineMode {
	if v.filterAll {
		return timelineAll
	}
	if v.showDone {
		return timelineCompleted
	}
	return timelinePending
}

func timelineTimestamp(item taskEntry, mode taskTimelineMode) int64 {
	if mode == timelineCompleted {
		return item.CompletedAt
	}
	return item.DueAt
}

// Sort copies only: snapshot order is the tie-breaker and the order of unknown
// dates. Completion history is never substituted for a task's own timestamp.
func taskTimeline(items []taskEntry, now time.Time, mode taskTimelineMode) []taskDateGroup {
	ordered := make([]taskEntry, 0, len(items))
	for _, item := range items {
		if item.Deleted || mode == timelinePending && item.Done || mode == timelineCompleted && !item.Done {
			continue
		}
		ordered = append(ordered, item)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := timelineTimestamp(ordered[i], mode), timelineTimestamp(ordered[j], mode)
		if a <= 0 {
			return false
		}
		if b <= 0 {
			return true
		}
		if mode == timelineCompleted {
			return a > b
		}
		return a < b
	})
	var groups []taskDateGroup
	for _, item := range ordered {
		key, title, detail := "unscheduled", "未安排", ""
		if mode == timelineCompleted {
			key, title = "unknown-completion", "完成日期未知"
		}
		if at := timelineTimestamp(item, mode); at > 0 {
			key, title, detail = taskTimelineDate(time.UnixMilli(at).In(now.Location()), now)
			if mode == timelinePending && at < now.UnixMilli() {
				key, title, detail = "overdue", "已逾期", ""
			}
		}
		if len(groups) == 0 || groups[len(groups)-1].key != key {
			groups = append(groups, taskDateGroup{key: key, title: title, detail: detail})
		}
		groups[len(groups)-1].items = append(groups[len(groups)-1].items, item)
	}
	return groups
}

func taskTimelineDate(at, now time.Time) (key, title, detail string) {
	key, title = at.Format("2006-01-02"), at.Format("1月2日")
	if at.Year() != now.Year() {
		title = at.Format("2006年1月2日")
	}
	detail = []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}[at.Weekday()]
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch key {
	case day.Format("2006-01-02"):
		key, title, detail = "today", "今天", at.Format("1月2日")
	case day.AddDate(0, 0, 1).Format("2006-01-02"):
		key, title, detail = "tomorrow", "明天", at.Format("1月2日")
	case day.AddDate(0, 0, -1).Format("2006-01-02"):
		key, title, detail = "yesterday", "昨天", at.Format("1月2日")
	}
	return
}

// Only request the next deadline or local midnight; completed dates also need
// midnight refresh, even when every task has no planned date.
func nextTaskTimelineChange(items []taskEntry, now time.Time, mode taskTimelineMode) time.Duration {
	var next time.Time
	for _, item := range items {
		if item.Deleted || mode == timelinePending && item.Done || mode == timelineCompleted && !item.Done {
			continue
		}
		if timelineTimestamp(item, mode) <= 0 {
			continue
		}
		if next.IsZero() {
			next = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		}
		if item.Done || mode == timelineCompleted {
			continue
		}
		// Match the strict "past due" comparison at millisecond precision.
		at := time.UnixMilli(item.DueAt).Add(time.Millisecond)
		if at.After(now) && at.Before(next) {
			next = at
		}
	}
	if next.IsZero() {
		return 0
	}
	return next.Sub(now)
}

func (v *nativeTasksView) webTaskTimeline(c *ui.Context, items []taskEntry, anchors map[int]ui.Element) {
	t := v.visual
	now, mode := v.service.m.now(), v.taskTimelineMode()
	if delay := nextTaskTimelineChange(items, now, mode); delay > 0 {
		c.After(delay)
	}
	if len(items) > 0 {
		caption := "按预计完成时间"
		if mode == timelineCompleted {
			caption = "按实际完成日期 · 最近在前"
		}
		if mode == timelineAll {
			caption = "按预计完成日期 · 从早到晚"
		}
		ui.Text(c, caption).FontSize(11).TextColor(t.TextMuted).Margin(6, 0, 2, 24)
	}
	for _, group := range taskTimeline(items, now, mode) {
		color := t.TextMuted
		if group.key == "today" {
			color = t.Accent
		}
		if group.key == "overdue" {
			color = t.Danger
		}
		ui.Column(c.Key(fmt.Sprintf("timeline-%d-%s", mode, group.key))).Margin(0, 0, 12).Children(func() {
			ui.Box(c).Absolute().Left(5).Top(20).Bottom(0).Width(1).Background(t.Border)
			ui.Box(c).Absolute().Left(2).Top(15).Size(7, 7).Radius(4).Background(color)
			ui.Row(c).Height(38).Padding(0, 0, 0, 24).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, group.title).FontSize(13).FontWeight(600).TextColor(color)
				if group.detail != "" {
					ui.Text(c, group.detail).FontSize(11).TextColor(t.TextMuted)
				}
				ui.Text(c, fmt.Sprintf("%d 项", len(group.items))).FontSize(11).TextColor(t.TextMuted)
			})
			ui.Column(c).Margin(0, 0, 0, 16).Children(func() {
				for i, item := range group.items {
					anchors[item.ID] = v.webTaskListRow(c, item, i < len(group.items)-1)
				}
			})
		})
	}
}

func (v *nativeTasksView) webTaskListRow(c *ui.Context, item taskEntry, separator bool) ui.Element {
	t := v.visual
	row := ui.Column(c.Key(fmt.Sprintf("task-%d", item.ID))).Padding(15, 12)
	if separator {
		row.BorderColor(t.Border).BorderWidth(0, 0, 1, 0)
	}
	if row.Hovered() {
		row.Background(t.Alt).Radius(8)
	}
	if v.editID == item.ID {
		row.Background(t.Soft).Radius(8)
	}
	showActions := row.Hovered() || row.FocusWithin() || v.editID == item.ID || v.moreID == item.ID
	var anchor ui.Element
	row.Children(func() { anchor = v.webTaskRow(c, item, showActions) })
	return anchor
}
