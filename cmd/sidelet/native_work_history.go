package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Completion snapshots are saved atomically with the task, independently of
// subsequent task edits. No notes are duplicated into the historical record.
type workCompletion struct {
	TaskID      int    `json:"taskId"`
	Title       string `json:"title"`
	CompletedAt int64  `json:"completedAt"`
}

func (m *model) workCompletions() []workCompletion {
	result := append([]workCompletion(nil), m.WorkHistory...)
	type key struct {
		id int
		at int64
	}
	known := make(map[key]bool, len(result))
	for _, entry := range result {
		known[key{entry.TaskID, entry.CompletedAt}] = true
	}
	// Upgrade existing completed tasks without inventing dates for old records.
	for i, item := range m.Tasks {
		if item.Deleted || !item.Done || item.Temporary || item.CompletedAt <= 0 || known[key{i + 1, item.CompletedAt}] {
			continue
		}
		result = append(result, workCompletion{i + 1, item.Title, item.CompletedAt})
	}
	return result
}

func (m *model) removeWorkCompletion(id int, at int64) {
	for i := len(m.WorkHistory) - 1; i >= 0; i-- {
		if m.WorkHistory[i].TaskID == id && m.WorkHistory[i].CompletedAt == at {
			m.WorkHistory = append(m.WorkHistory[:i], m.WorkHistory[i+1:]...)
			return
		}
	}
}

type workLogDay struct {
	day     time.Time
	entries []workCompletion
}

func workLogTimeline(entries []workCompletion, from, to time.Time) []workLogDay {
	end := to.AddDate(0, 0, 1).UnixMilli()
	groups := map[string]*workLogDay{}
	for _, entry := range entries {
		if entry.CompletedAt < from.UnixMilli() || entry.CompletedAt >= end {
			continue
		}
		at := time.UnixMilli(entry.CompletedAt).In(from.Location())
		key := at.Format("2006-01-02")
		if groups[key] == nil {
			groups[key] = &workLogDay{day: time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, from.Location())}
		}
		groups[key].entries = append(groups[key].entries, entry)
	}
	days := make([]workLogDay, 0, len(groups))
	for _, group := range groups {
		sort.SliceStable(group.entries, func(i, j int) bool { return group.entries[i].CompletedAt < group.entries[j].CompletedAt })
		days = append(days, *group)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].day.After(days[j].day) })
	return days
}

func (d workLogDay) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s · 完成 %d 项\n", d.day.Format("2006年1月2日"), len(d.entries))
	for _, entry := range d.entries {
		fmt.Fprintf(&b, "- %s %s\n", time.UnixMilli(entry.CompletedAt).In(d.day.Location()).Format("15:04"), strings.Join(strings.Fields(entry.Title), " "))
	}
	return strings.TrimSpace(b.String())
}

func workLogRangeText(days []workLogDay, from, to time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "工作日志 · %s — %s\n", from.Format("2006-01-02"), to.Format("2006-01-02"))
	if len(days) == 0 {
		b.WriteString("\n这段时间暂无完成记录")
	}
	for _, day := range days {
		b.WriteString("\n" + day.text() + "\n")
	}
	return strings.TrimSpace(b.String())
}
