package main

import (
	"errors"
	"sidelet/internal/spike"
	"slices"
)

const overflowIndex = -2

// IDs always address their original task slot. Sorting exchanges only order
// slots; it never moves payloads, versions, reminder generations or undo IDs.
func (m *model) orderedIndices() []int {
	out := make([]int, 0, len(m.Tasks))
	seen := make(map[int]bool, len(m.Tasks))
	for _, id := range m.Order {
		if id > 0 && id <= len(m.Tasks) && !seen[id] {
			out = append(out, id-1)
			seen[id] = true
		}
	}
	for i := range m.Tasks {
		if !seen[i+1] {
			out = append(out, i)
		}
	}
	return out
}
func (m *model) eligible(i int) bool {
	if i < 0 || i >= len(m.Tasks) {
		return false
	}
	t := m.Tasks[i]
	return !t.Deleted && !t.Done && !t.Unpinned && t.SnoozedUntil <= m.now().UnixMilli()
}
func (m *model) eligibleIDs() []int {
	ids := []int{}
	for _, i := range m.orderedIndices() {
		if m.eligible(i) {
			ids = append(ids, i+1)
		}
	}
	return ids
}
func (m *model) desktopArrange() bool { return !m.Quiet && m.Arranging && m.WorkHeight >= 190 }
func (m *model) sidebarLayout() spike.StackGeometry {
	area := m.WorkHeight
	if area <= 0 {
		area = 1000
	}
	if m.desktopArrange() {
		area -= 132
	} else if m.undoVisible() {
		area -= 46
	}
	return spike.StackLayout(m.sidebarCount(), float64(max(0, area)), m.Offset, float64(m.itemHeight()))
}
func (m *model) sidebarItems() (direct, overflow []int) {
	if m.Quiet {
		return nil, nil
	}
	all := m.eligibleIDs()
	limit := m.sidebarLayout().Direct
	if m.UITheme == "" {
		limit = len(all)
	}
	direct = make([]int, 0, limit)
	for _, id := range all[:limit] {
		direct = append(direct, id-1)
	}
	if !m.Arranging && len(direct) > 0 && m.eligible(m.Opened) && !slices.Contains(direct, m.Opened) {
		direct[len(direct)-1] = m.Opened
	}
	for _, id := range all {
		if !slices.Contains(direct, id-1) {
			overflow = append(overflow, id-1)
		}
	}
	return
}
func (m *model) cardOpen() bool { return m.Opened >= 0 || m.OverflowOpen }
func (m *model) openOverflow() bool {
	_, tail := m.sidebarItems()
	if len(tail) == 0 || m.Quiet || m.Arranging || m.Editing {
		return false
	}
	m.close()
	m.OverflowOpen = true
	return true
}
func (s *TasksService) Arrange(enabled bool) (taskSnapshot, error) {
	return s.mutate("web-arrange", func() error {
		if enabled && s.m.Quiet {
			return errors.New("请先恢复桌面显示。")
		}
		if s.m.Editing {
			return errors.New("请先结束卡片编辑。")
		}
		s.m.close()
		s.m.Arranging = enabled
		return nil
	})
}
func (s *TasksService) Reorder(previous, order []int) (taskSnapshot, error) {
	return s.mutate("web-reorder", func() error {
		m := s.m
		if !m.Arranging || m.Editing {
			return errors.New("请先进入整理桌面。")
		}
		current := m.eligibleIDs()
		if len(current) == 0 || !slices.Equal(current, previous) || len(current) != len(order) {
			return errors.New("任务列表已变化，请按当前顺序重新拖动。")
		}
		remaining := map[int]bool{}
		for _, id := range current {
			remaining[id] = true
		}
		for _, id := range order {
			if !remaining[id] {
				return errors.New("排序包含重复任务或其他桌面的任务。")
			}
			delete(remaining, id)
		}
		if slices.Equal(current, order) {
			return nil
		}
		all := m.orderedIndices()
		next := make([]int, len(all))
		slot := 0
		for k, i := range all {
			next[k] = i + 1
			if m.eligible(i) {
				next[k] = order[slot]
				slot++
			}
		}
		m.Order = next
		return nil
	})
}
func moveOrder(ids []int, source, target int, after bool) []int {
	if source == target || !slices.Contains(ids, source) || !slices.Contains(ids, target) {
		return slices.Clone(ids)
	}
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id != source {
			out = append(out, id)
		}
	}
	at := slices.Index(out, target)
	if after {
		at++
	}
	return slices.Insert(out, at, source)
}
