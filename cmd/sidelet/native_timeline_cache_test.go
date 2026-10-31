package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func timelineCacheFixture(n int) (*nativeTasksView, time.Time) {
	now := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	m := newModel()
	m.Tasks = nil
	m.UITheme = "mac"
	m.clock = func() time.Time { return now }
	for i := range n {
		m.Tasks = append(m.Tasks, task{Title: fmt.Sprintf("Task %d", i), Note: "搜索备注", DueAt: now.Add(time.Duration(i+1) * time.Hour).UnixMilli(), Version: 1, Done: i%3 == 0, CompletedAt: now.Add(-time.Duration(i) * time.Hour).UnixMilli()})
	}
	return newNativeTasksView(testService(m)), now
}

// This is the old preparation path, also used as an independent invalidation
// oracle. The production snapshot API must keep returning detached task copies.
func uncachedTimeline(v *nativeTasksView, now time.Time) ([]taskDateGroup, int) {
	items := v.service.snapshot().Tasks
	visible := make([]taskEntry, 0, len(items))
	for _, item := range items {
		if v.taskVisible(item) {
			visible = append(visible, item)
		}
	}
	return taskTimeline(visible, now, v.taskTimelineMode()), len(visible)
}

func TestTimelineCacheTracksEveryInput(t *testing.T) {
	v, now := timelineCacheFixture(12)
	check := func(t *testing.T) {
		t.Helper()
		want, count := uncachedTimeline(v, now)
		got := v.preparedTimeline(now)
		if !reflect.DeepEqual(got.groups, want) || got.count != count {
			t.Fatalf("stale timeline: got %#v, want %#v", got.groups, want)
		}
	}
	check(t)
	steps := []struct {
		name   string
		change func()
	}{
		{"sidebar edit without service revision", func() { v.service.m.Tasks[1].Title = "Changed"; v.service.m.Tasks[1].Note = "new note" }},
		{"manual ordering and tied dates", func() {
			v.service.m.Tasks[2].DueAt = v.service.m.Tasks[1].DueAt
			v.service.m.Order = []int{3, 2, 3, 999}
		}},
		{"completion", func() { v.service.m.Tasks[1].Done = true; v.service.m.Tasks[1].CompletedAt = now.UnixMilli() }},
		{"completed filter", func() { v.showDone = true }},
		{"all filter", func() { v.filterAll = true }},
		{"search", func() { v.searchQuery = "CHANGED" }},
		{"edit removes search match", func() { v.service.m.Tasks[1].Title = "Other" }},
		{"clear search", func() { v.searchQuery = "" }},
		{"delete", func() { v.service.m.Tasks[2] = task{Deleted: true} }},
		{"append", func() { v.service.m.Tasks = append(v.service.m.Tasks, task{Title: "Added"}) }},
		{"pending", func() { v.filterAll, v.showDone = false, false }},
		{"deadline boundary", func() { now = now.Add(5*time.Hour + time.Millisecond) }},
		{"midnight", func() { now = now.Add(24 * time.Hour) }},
		{"clock backwards", func() { now = now.Add(-48 * time.Hour) }},
		{"timezone", func() { now = now.In(time.FixedZone("UTC+8", 8*3600)) }},
		{"empty profile", func() { v.service.m.Tasks = nil; v.service.m.Order = nil }},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) { step.change(); check(t) })
	}
}

func TestTimelineCacheReusesRowsAndDoesNotOwnSnapshots(t *testing.T) {
	v, now := timelineCacheFixture(30)
	groups := v.preparedTimeline(now).groups
	if n := testing.AllocsPerRun(50, func() { v.preparedTimeline(now.Add(time.Second)) }); n != 0 {
		t.Fatalf("unchanged preparation allocated %g times", n)
	}
	if &groups[0].items[0] != &v.preparedTimeline(now.Add(time.Second)).groups[0].items[0] {
		t.Fatal("recreated unchanged rows")
	}
	snapshot := v.service.snapshot()
	snapshot.Tasks[0].Title = "caller modification"
	if v.service.m.Tasks[0].Title == "caller modification" {
		t.Fatal("snapshot aliases model")
	}
	v.searchQuery = "no match"
	if p := v.preparedTimeline(now); p.count != 0 || len(p.groups) != 0 {
		t.Fatal("old query rows retained")
	}
}

func BenchmarkTimelinePreparation(b *testing.B) {
	for _, n := range []int{3, 300, 1000} {
		for _, cached := range []bool{false, true} {
			b.Run(fmt.Sprintf("tasks=%d/cached=%t", n, cached), func(b *testing.B) {
				v, now := timelineCacheFixture(n)
				v.preparedTimeline(now)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if cached {
						v.preparedTimeline(now)
					} else {
						uncachedTimeline(v, now)
					}
				}
			})
		}
	}
}

func TestTimelineCacheDeadlineAndMidnightBoundaries(t *testing.T) {
	v, now := timelineCacheFixture(0)
	due := now.Add(time.Hour)
	v.service.m.Tasks = []task{{Title: "Deadline", DueAt: due.UnixMilli()}}
	for _, tc := range []struct {
		at  time.Time
		key string
	}{
		{now, "today"}, {due, "today"}, {due.Add(time.Millisecond), "overdue"},
	} {
		got := v.preparedTimeline(tc.at)
		if len(got.groups) != 1 || got.groups[0].key != tc.key {
			t.Fatalf("at %v: %#v", tc.at, got.groups)
		}
	}
	v.filterAll = true
	midnight := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if v.preparedTimeline(midnight.Add(-time.Millisecond)).groups[0].key != "today" {
		t.Fatal("early day change")
	}
	if v.preparedTimeline(midnight).groups[0].key != "yesterday" {
		t.Fatal("missed midnight")
	}
}
