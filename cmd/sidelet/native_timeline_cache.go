package main

import (
	"slices"
	"time"
)

// Only the active query is retained. Compare model values as sidebar edits and
// reminder updates do not all pass through TasksService.revision. Cached rows
// are read-only; UI elements and editor drafts never belong in this cache.
type taskTimelineCache struct {
	ready            bool
	tasks            []task
	order            []int
	mode             taskTimelineMode
	query            string
	builtAt, expires time.Time
	groups           []taskDateGroup
	count            int
}

func (v *nativeTasksView) preparedTimeline(now time.Time) *taskTimelineCache {
	p, m, mode := &v.timelineCache, v.service.m, v.taskTimelineMode()
	sameDay := now.Year() == p.builtAt.Year() && now.YearDay() == p.builtAt.YearDay()
	if p.ready && p.mode == mode && p.query == v.searchQuery &&
		now.Location() == p.builtAt.Location() && sameDay && !now.Before(p.builtAt) &&
		(p.expires.IsZero() || now.Before(p.expires)) &&
		slices.Equal(p.tasks, m.Tasks) && slices.Equal(p.order, m.Order) {
		return p
	}
	items := make([]taskEntry, 0, m.totalCount())
	for _, i := range m.orderedIndices() {
		item := taskEntry{ID: i + 1, task: m.Tasks[i]}
		if v.taskVisible(item) {
			items = append(items, item)
		}
	}
	*p = taskTimelineCache{
		ready: true, tasks: slices.Clone(m.Tasks), order: slices.Clone(m.Order),
		mode: mode, query: v.searchQuery, builtAt: now,
		groups: taskTimeline(items, now, mode), count: len(items),
	}
	if delay := nextTaskTimelineChange(items, now, mode); delay > 0 {
		p.expires = now.Add(delay)
	}
	return p
}
