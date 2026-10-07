package main

import (
	"errors"
	"github.com/egoist/mygo"
	"math"
	"sidelet/internal/todo"
	"time"
)

func (s *TasksService) Delete(id int, version uint64) (taskSnapshot, error) {
	return s.mutate("web-delete", func() error {
		t, err := s.editable(id, version)
		if err != nil {
			return err
		}
		// Tombstones keep IDs and concurrent drafts stable; deleted entries never
		// appear in snapshots or input regions. This fixture has no persistent store.
		*t = task{Deleted: true, Version: t.Version + 1}
		if s.m.Opened == id-1 {
			s.m.close()
		}
		if s.m.Hover == id-1 {
			s.m.Hover = -1
		}
		if s.m.UndoID == id {
			s.m.UndoID = 0
			s.m.UndoUntil = 0
		}
		return nil
	})
}
func (s *TasksService) Snooze(id int, duration string, version uint64) (taskSnapshot, error) {
	return s.mutate("web-snooze", func() error {
		until, ok := todo.SnoozeUntil(s.m.now(), duration)
		if !ok {
			return errors.New("稍后提醒时间无效。")
		}
		t, err := s.editable(id, version)
		if err != nil {
			return err
		}
		if t.Done {
			return errors.New("已完成任务不能暂时隐藏。")
		}
		t.SnoozedUntil = until.UnixMilli()
		t.Version++
		if s.m.Opened == id-1 {
			s.m.close()
		}
		if s.m.Hover == id-1 {
			s.m.Hover = -1
		}
		return nil
	})
}
func (s *TasksService) Unsnooze(id int, version uint64) (taskSnapshot, error) {
	return s.mutate("web-unsnooze", func() error {
		t, err := s.editable(id, version)
		if err != nil {
			return err
		}
		if s.m.UITheme == "" && t.SnoozedUntil > s.m.now().UnixMilli() && !t.Done && !t.Unpinned && s.m.sidebarCount() >= s.m.sidebarCapacity() {
			return errors.New("侧栏已满，请先完成一项任务。")
		}
		t.SnoozedUntil = 0
		t.Version++
		return nil
	})
}
func (s *TasksService) Undo() (taskSnapshot, error) {
	return s.mutate("web-undo", func() error {
		m := s.m
		if m.UndoID == 0 || m.now().UnixMilli() > m.UndoUntil {
			return errors.New("撤销时间已结束。")
		}
		t := &m.Tasks[m.UndoID-1]
		if m.UITheme == "" && !t.Unpinned && t.SnoozedUntil <= m.now().UnixMilli() && m.sidebarCount() >= m.sidebarCapacity() {
			return errors.New("侧栏已满，请先完成一项任务。")
		}
		t.Done = false
		t.CompletedAt = 0
		t.Version++
		m.UndoID = 0
		m.UndoUntil = 0
		return nil
	})
}
func (s *TasksService) Layout(side string, offset float64, height int) (taskSnapshot, error) {
	return s.mutate("web-layout", func() error {
		if s.m.Arranging {
			return errors.New("请先完成整理。")
		}
		if (side != "left" && side != "right") || math.IsNaN(offset) || math.IsInf(offset, 0) || offset < 0 || offset > 1 || (height != 40 && height != 44 && height != 56) {
			return errors.New("桌面位置或标签密度无效。")
		}
		if s.m.Editing {
			return errors.New("请先结束卡片编辑。")
		}
		s.m.close()
		s.m.Side, s.m.Offset, s.m.ItemHeight = side, offset, height
		s.m.positionStack()
		return nil
	})
}

// One wake-up per next expiry, with no permanent polling loop. Changes to a
// deadline invalidate the previous callback even if it is already queued.
func (s *TasksService) scheduleExpiry() {
	if s.schedule == nil {
		return
	}
	if s.wake != nil {
		s.wake.Stop()
		s.wake = nil
	}
	now := s.m.now().UnixMilli()
	next := int64(0)
	if s.m.UndoID != 0 && s.m.UndoUntil >= now {
		next = s.m.UndoUntil + 1
	}
	if !s.nextReminder.IsZero() && s.nextReminder.UnixMilli() > now && (next == 0 || s.nextReminder.UnixMilli() < next) {
		next = s.nextReminder.UnixMilli()
	}
	for _, t := range s.m.Tasks {
		if t.Deleted {
			continue
		}
		consider := func(at int64) {
			if at > now && (next == 0 || at < next) {
				next = at
			}
		}
		if !t.Done && t.DueAt > 0 {
			consider(t.DueAt - 30*time.Minute.Milliseconds())
			consider(t.DueAt)
		}
		if t.Temporary && t.Done && t.CompletedAt > 0 {
			consider(t.CompletedAt + 5001)
		}
		if !t.Deleted && t.SnoozedUntil > now && (next == 0 || t.SnoozedUntil < next) {
			next = t.SnoozedUntil
		}
	}
	if next == 0 {
		return
	}
	revision := s.revision
	s.schedule(time.Duration(next-now)*time.Millisecond, func() {
		if revision != s.revision {
			return
		}
		s.m.cleanup()
		s.revision++
		if s.notices != nil {
			s.notices.request()
		}
		if s.changed != nil {
			s.changed("native-main-expiry")
		}
		s.scheduleExpiry()
	})
}
func (s *TasksService) enableExpiry() {
	s.schedule = func(delay time.Duration, f func()) { s.wake = time.AfterFunc(delay, func() { mygo.RunOnMain(f) }) }
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		s.schedule = nil
		if s.wake != nil {
			s.wake.Stop()
		}
	})
}
