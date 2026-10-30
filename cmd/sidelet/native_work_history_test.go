package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWorkHistoryPersistsSnapshotsAndUndo(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	if _, err := s.Add("历史任务", 0); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.Local)
	m.clock = func() time.Time { return now }
	original := m.Tasks[0].Title
	if _, err := s.SetDone(1, true, m.Tasks[0].Version); err != nil {
		t.Fatal(err)
	}
	if len(m.WorkHistory) != 1 {
		t.Fatal("missing completion")
	}
	if _, err := s.Update(1, "改过的名称", 0, m.Tasks[0].Version); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetDone(1, false, m.Tasks[0].Version); err != nil {
		t.Fatal(err)
	}
	if len(m.WorkHistory) != 1 || m.WorkHistory[0].Title != original {
		t.Fatal("reopen or edit changed history")
	}
	now = now.Add(time.Hour)
	if _, err := s.SetDone(1, true, m.Tasks[0].Version); err != nil {
		t.Fatal(err)
	}
	if len(m.WorkHistory) != 2 {
		t.Fatal("second completion was not recorded")
	}
	if _, err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if len(m.WorkHistory) != 1 || m.WorkHistory[0].Title != original {
		t.Fatal("undo removed wrong completion")
	}
	if _, err := s.Delete(1, m.Tasks[0].Version); err != nil {
		t.Fatal(err)
	}
	p.close()
	m2, _, _ := profileFixture(t, dir)
	if len(m2.WorkHistory) != 1 || m2.WorkHistory[0].Title != original || !m2.Tasks[0].Deleted {
		t.Fatal("history lost on deletion/restart")
	}
}

func TestWorkHistoryBackfillRollbackAndTemporary(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	for _, title := range []string{"旧任务", "临时任务"} {
		if _, err := s.Add(title, 0); err != nil {
			t.Fatal(err)
		}
	}
	at := time.Date(2026, 10, 29, 18, 0, 0, 0, time.Local).UnixMilli()
	m.Tasks[0].Done, m.Tasks[0].CompletedAt = true, at
	if err := m.commit(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(m.WorkHistory) != 1 {
		t.Fatal("old completed task not backfilled")
	}
	if err := m.commit(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(m.WorkHistory) != 1 {
		t.Fatal("duplicate backfill")
	}
	p.write = func(string, []byte) error { return errors.New("disk full") }
	if _, err := s.SetDone(2, true, m.Tasks[1].Version); err == nil {
		t.Fatal("expected save failure")
	}
	if len(m.WorkHistory) != 1 || m.Tasks[1].Done {
		t.Fatal("failed save left a log")
	}
	p.write = p.db.Write
	m.Tasks[1].Temporary = true
	if _, err := s.SetDone(2, true, m.Tasks[1].Version); err != nil {
		t.Fatal(err)
	}
	if len(m.WorkHistory) != 1 {
		t.Fatal("temporary task created history")
	}
}

func TestWorkHistoryTimelineDateBoundaries(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	from := time.Date(2026, 3, 7, 0, 0, 0, 0, zone)
	to := from.AddDate(0, 0, 2)
	entries := []workCompletion{
		{1, "起点", from.UnixMilli()},
		{2, "昨天", from.AddDate(0, 0, 1).UnixMilli()},
		{3, "终点前", to.AddDate(0, 0, 1).Add(-time.Millisecond).UnixMilli()},
		{4, "范围外", to.AddDate(0, 0, 1).UnixMilli()},
	}
	days := workLogTimeline(entries, from, to)
	if len(days) != 3 || days[0].entries[0].Title != "终点前" || days[2].entries[0].Title != "起点" {
		t.Fatal(days)
	}
	if strings.Contains(workLogRangeText(days, from, to), "范围外") {
		t.Fatal("inclusive end crossed midnight")
	}
}

func TestWorkHistoryRangeAndDailyClipboard(t *testing.T) {
	m, v, u := webUITester(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.Local)
	m.clock = func() time.Time { return now }
	m.WorkHistory = []workCompletion{{1, "今日完成", now.UnixMilli()}, {2, "十天前完成", now.AddDate(0, 0, -10).UnixMilli()}}
	if err := u.Click("工作日志"); err != nil {
		t.Fatal(err)
	}
	if !u.HasText("今日完成") || u.HasText("十天前完成") {
		t.Fatal("default range wrong")
	}
	if err := u.Click("加载更早记录"); err != nil {
		t.Fatal(err)
	}
	if !u.HasText("十天前完成") {
		t.Fatal("older history not loaded")
	}
	if err := u.Click("复制日志：2026-10-20"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u.Clipboard(), "十天前完成") || strings.Contains(u.Clipboard(), "今日完成") {
		t.Fatal("daily copy included other dates")
	}
	if err := u.Click("复制日志"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u.Clipboard(), "十天前完成") || !strings.Contains(u.Clipboard(), "今日完成") {
		t.Fatal("range copy incomplete")
	}
	v.workLogFrom = "2026-10-31"
	u.Frame()
	if _, ok := u.Find("复制日志"); ok {
		t.Fatal("invalid range can be copied")
	}
}
