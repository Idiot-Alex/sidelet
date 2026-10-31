package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestPendingTimelineDateBoundariesAndStableOrder(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 12, 31, 12, 0, 0, 0, loc)
	entry := func(id int, at time.Time) taskEntry {
		due := int64(0)
		if !at.IsZero() {
			due = at.UnixMilli()
		}
		return taskEntry{ID: id, task: task{DueAt: due}}
	}
	items := []taskEntry{entry(1, time.Time{}), entry(2, now.AddDate(0, 0, 2)), entry(3, now.Add(time.Hour)), entry(4, now.Add(-time.Hour)), entry(5, now.AddDate(0, 0, 1)), entry(6, now), entry(7, now.Add(time.Hour)), entry(8, time.Time{})}
	before := append([]taskEntry(nil), items...)
	groups := taskTimeline(items, now, timelinePending)
	var keys []string
	var ids [][]int
	for _, g := range groups {
		keys = append(keys, g.key)
		var groupIDs []int
		for _, item := range g.items {
			groupIDs = append(groupIDs, item.ID)
		}
		ids = append(ids, groupIDs)
	}
	if !reflect.DeepEqual(keys, []string{"overdue", "today", "tomorrow", "2027-01-02", "unscheduled"}) || !reflect.DeepEqual(ids, [][]int{{4}, {6, 3, 7}, {5}, {2}, {1, 8}}) {
		t.Fatalf("unexpected groups: %v %v", keys, ids)
	}
	if groups[3].title != "2027年1月2日" {
		t.Fatal("cross-year date is ambiguous")
	}
	if !reflect.DeepEqual(before, items) {
		t.Fatal("timeline mutated desktop order")
	}
	items[0].Done, items[7].Deleted = true, true
	groups = taskTimeline(items, now, timelinePending)
	if len(groups) != 4 {
		t.Fatal("completed/deleted tasks produced an empty group")
	}
}

func TestTaskTimelineNextWakeUsesLocalCalendar(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, day := range []int{8, 1} {
		month := time.March
		want := 23 * time.Hour
		if day == 1 {
			month, want = time.November, 25*time.Hour
		}
		now := time.Date(2026, month, day, 0, 0, 0, 0, loc)
		items := []taskEntry{{task: task{DueAt: now.AddDate(0, 0, 3).UnixMilli()}}}
		if got := nextTaskTimelineChange(items, now, timelinePending); got != want {
			t.Fatalf("DST midnight: got %s want %s", got, want)
		}
		items[0].DueAt = now.Add(time.Minute).UnixMilli()
		if got := nextTaskTimelineChange(items, now, timelinePending); got != time.Minute+time.Millisecond {
			t.Fatal("missed exact deadline transition", got)
		}
		items[0].Done = true
		if nextTaskTimelineChange(items, now, timelinePending) != 0 {
			t.Fatal("completed-only list scheduled a wake")
		}
	}
	if nextTaskTimelineChange([]taskEntry{{}}, time.Now(), timelinePending) != 0 {
		t.Fatal("undated list scheduled a wake")
	}
}

func TestTimelineSearchCompleteUndoAndMidnightKeepDraft(t *testing.T) {
	m, v, u := webUITester(t)
	hasGroup := func(name string) bool { _, ok := u.Find(name); return ok }
	now := time.Date(2026, 10, 31, 23, 59, 0, 0, time.Local)
	m.clock = func() time.Time { return now }
	m.Tasks[0].DueAt = now.Add(2 * time.Minute).UnixMilli()
	m.Tasks[1].DueAt = now.Add(-time.Hour).UnixMilli()
	u.Frame()
	if !hasGroup("明天") || !hasGroup("已逾期") || !hasGroup("未安排") {
		t.Fatal("missing initial date groups")
	}
	if err := u.Click("编辑：整理今天的工作"); err != nil {
		t.Fatal(err)
	}
	v.draft = "跨日仍保留的编辑草稿"
	now = now.Add(time.Minute)
	u.Frame()
	if !hasGroup("今天") || hasGroup("明天") || v.draft != "跨日仍保留的编辑草稿" {
		t.Fatal("midnight regrouping lost draft or date")
	}
	if err := u.Click("取消编辑"); err != nil {
		t.Fatal(err)
	}
	if err := u.Click("完成：整理今天的工作"); err != nil {
		t.Fatal(err)
	}
	if hasGroup("今天") {
		t.Fatal("empty date group retained after completion")
	}
	if err := u.Click("撤销完成"); err != nil {
		t.Fatal(err)
	}
	if !hasGroup("今天") {
		t.Fatal("undo failed to restore date group")
	}
	v.searchText = "整理"
	u.Frame()
	if hasGroup("已逾期") || hasGroup("未安排") || !hasGroup("今天") {
		t.Fatal("search retained unmatched groups")
	}
	v.searchText = ""
	now = now.Add(time.Minute + time.Millisecond)
	u.Frame()
	if hasGroup("今天") || !hasGroup("已逾期") {
		t.Fatal("deadline did not regroup task")
	}
}

func timelineVisualFixture() (*model, *nativeTasksView) {
	m := newModel()
	now := time.Date(2026, 10, 31, 10, 0, 0, 0, time.Local)
	m.clock = func() time.Time { return now }
	m.Tasks = []task{
		{Title: "整理本周的项目反馈", Note: "把需要跟进的事项留在手边", DueAt: now.Add(-2 * time.Hour).UnixMilli(), Priority: 2, Version: 1},
		{Title: "确认新版侧栏的交互细节", DueAt: now.Add(4 * time.Hour).UnixMilli(), Priority: 3, Version: 1},
		{Title: "完成主界面的时间轴设计", DueAt: now.Add(7 * time.Hour).UnixMilli(), Version: 1},
		{Title: "准备下周的发布清单", DueAt: now.AddDate(0, 0, 1).UnixMilli(), Version: 1},
		{Title: "有空时整理灵感和参考", Version: 1},
	}
	return m, newNativeTasksView(testService(m))
}

func TestTimelineLayoutThemesAndEditor(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		for _, width := range []int{1120, 860, 820, 540} {
			t.Run(fmt.Sprintf("%s-%d", theme, width), func(t *testing.T) {
				m, v := timelineVisualFixture()
				m.UITheme = theme
				u := ui.NewTester(v.render, width, 900)
				u.SetFocused(true)
				check := func() {
					title := visualRect(t, u, "编辑：整理本周的项目反馈")
					complete := visualRect(t, u, "完成：整理本周的项目反馈")
					remove := visualRect(t, u, "删除任务：整理本周的项目反馈")
					if complete.X+complete.W > title.X || title.X+title.W > remove.X || remove.X+remove.W > float32(width) {
						t.Fatal("timeline actions overlap", title, complete, remove)
					}
				}
				check()
				if dir := os.Getenv("SIDELET_TIMELINE_VISUAL_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%d.png", theme, width)))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(f, u.Image())
					closeErr := f.Close()
					if err != nil {
						t.Fatal(err)
					}
					if closeErr != nil {
						t.Fatal(closeErr)
					}
				}
				v.beginEdit(v.service.snapshot().Tasks[0])
				u.Frame()
				check()
			})
		}
	}
}

func TestCompletedAndAllTimelineUseDifferentDates(t *testing.T) {
	now := time.Date(2026, 12, 31, 12, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	today := now.Add(-time.Hour).UnixMilli()
	yesterday := now.AddDate(0, 0, -1).UnixMilli()
	items := []taskEntry{
		{ID: 1, task: task{Done: true, DueAt: yesterday, CompletedAt: today}},
		{ID: 2, task: task{Done: true, DueAt: now.AddDate(0, 0, 1).UnixMilli(), CompletedAt: yesterday}},
		{ID: 3, task: task{Done: true, DueAt: today}},
		{ID: 4, task: task{DueAt: yesterday}},
		{ID: 5, task: task{Done: true, CompletedAt: today}},
		{ID: 6, task: task{Done: true}},
		{ID: 7, task: task{Deleted: true, Done: true, CompletedAt: now.UnixMilli()}},
	}
	before := append([]taskEntry(nil), items...)
	check := func(mode taskTimelineMode, wantKeys []string, wantIDs [][]int) {
		t.Helper()
		var keys []string
		var ids [][]int
		for _, g := range taskTimeline(items, now, mode) {
			keys = append(keys, g.key)
			var groupIDs []int
			for _, item := range g.items {
				groupIDs = append(groupIDs, item.ID)
			}
			ids = append(ids, groupIDs)
		}
		if !reflect.DeepEqual(keys, wantKeys) || !reflect.DeepEqual(ids, wantIDs) {
			t.Fatalf("mode %d: %v %v", mode, keys, ids)
		}
	}
	check(timelineCompleted, []string{"today", "yesterday", "unknown-completion"}, [][]int{{1, 5}, {2}, {3, 6}})
	check(timelineAll, []string{"yesterday", "today", "tomorrow", "unscheduled"}, [][]int{{1, 4}, {3}, {2}, {5, 6}})
	if !reflect.DeepEqual(items, before) {
		t.Fatal("grouping mutated source dates or order")
	}
	items[1].CompletedAt = now.Add(time.Minute).UnixMilli()
	check(timelineCompleted, []string{"today", "unknown-completion"}, [][]int{{2, 1, 5}, {3, 6}})
	// Calendar boundaries use local dates, not a rolling 24-hour interval.
	midnight := time.Date(2027, 1, 1, 0, 0, 0, 0, now.Location())
	groups := taskTimeline(items, midnight, timelineCompleted)
	if groups[0].title != "昨天" {
		t.Fatal("completion group did not roll over at midnight")
	}
	groups = taskTimeline(items, now.AddDate(0, 0, 4), timelineAll)
	if groups[0].title != "2026年12月30日" {
		t.Fatal("past plan from another year lacks year")
	}
}

func TestCompletedTimelineSchedulesMidnightWithoutDeadline(t *testing.T) {
	now := time.Date(2026, 10, 31, 23, 50, 0, 0, time.Local)
	item := taskEntry{task: task{Done: true, CompletedAt: now.UnixMilli()}}
	if got := nextTaskTimelineChange([]taskEntry{item}, now, timelineCompleted); got != 10*time.Minute {
		t.Fatal("missing completion-date wake", got)
	}
	if got := nextTaskTimelineChange([]taskEntry{item}, now, timelineAll); got != 0 {
		t.Fatal("undated plan should not wake", got)
	}
	item.DueAt = now.Add(time.Minute).UnixMilli()
	if got := nextTaskTimelineChange([]taskEntry{item}, now, timelineAll); got != 10*time.Minute {
		t.Fatal("completed plan must wake at midnight, not deadline", got)
	}
	item.CompletedAt = 0
	if got := nextTaskTimelineChange([]taskEntry{item}, now, timelineCompleted); got != 0 {
		t.Fatal("unknown completion date scheduled a wake", got)
	}
}

func TestCompletedAndAllTimelineInteractionAndLayout(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, v := timelineVisualFixture()
			m.UITheme = theme
			now := m.now()
			m.Tasks[0].Done, m.Tasks[0].CompletedAt = true, now.Add(-time.Hour).UnixMilli()
			m.Tasks[1].Done, m.Tasks[1].CompletedAt = true, now.AddDate(0, 0, -1).UnixMilli()
			m.Tasks[4].Done = true // Historical completion with no recorded timestamp.
			u := ui.NewTester(v.render, 860, 900)
			u.SetFocused(true)
			click := func(label string) {
				t.Helper()
				if err := u.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			click("已完成 3")
			for _, label := range []string{"按实际完成日期 · 最近在前", "今天", "昨天", "完成日期未知", "完成于 09:00"} {
				if _, ok := u.Find(label); !ok {
					t.Fatal("missing completion timeline label", label)
				}
			}
			if u.HasText("已逾期") || u.HasText("1970") {
				t.Fatal("completion view shows plan overdue or fabricated date")
			}
			for _, mode := range []string{"completed", "all"} {
				if mode == "all" {
					click("全部 5")
				}
				for _, width := range []int{1120, 820, 540} {
					u.SetSize(width, 900)
					label := visualRect(t, u, "编辑：整理本周的项目反馈")
					action := visualRect(t, u, "删除任务：整理本周的项目反馈")
					if label.X+label.W > action.X || action.X+action.W > float32(width) {
						t.Fatal("completed row actions overlap", label, action)
					}
					if dir := os.Getenv("SIDELET_TIMELINE_VISUAL_DIR"); dir != "" {
						if err := os.MkdirAll(dir, 0755); err != nil {
							t.Fatal(err)
						}
						f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%s-%d.png", theme, mode, width)))
						if err != nil {
							t.Fatal(err)
						}
						err = png.Encode(f, u.Image())
						closeErr := f.Close()
						if err != nil {
							t.Fatal(err)
						}
						if closeErr != nil {
							t.Fatal(closeErr)
						}
					}
				}
				u.SetSize(860, 900)
			}
			if !u.HasText("完成于 10/31 09:00") || !u.HasText("计划 10/31 08:00") {
				t.Fatal("all view confuses planned and completed times")
			}
			click("编辑：整理本周的项目反馈")
			v.draft = "编辑草稿仍在"
			v.searchText = "整理本周"
			u.Frame()
			if _, ok := u.Find("未安排"); ok {
				t.Fatal("search retained unmatched groups")
			}
			click("删除任务：整理本周的项目反馈")
			click("取消删除")
			if v.draft != "编辑草稿仍在" {
				t.Fatal("confirmation discarded editor")
			}
			click("取消编辑")
			v.searchText = ""
			u.Frame()
			click("已完成 3")
			click("恢复：整理本周的项目反馈")
			if _, ok := u.Find("今天"); ok {
				t.Fatal("restore retained empty completion group")
			}
			click("全部 5")
			if !u.HasText("已逾期 · 10/31 08:00") {
				t.Fatal("restored overdue plan missing status")
			}
			click("完成：整理本周的项目反馈")
			click("已完成 3")
			if !u.HasText("完成于 10:00") {
				t.Fatal("new completion reused old timestamp")
			}
		})
	}
}
