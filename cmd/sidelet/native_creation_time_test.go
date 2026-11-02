package main

import (
	"testing"
	"time"
)

func TestNewTaskCreationTimePersistsAndDoesNotChangeOnEdit(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.Local)
	m.clock = func() time.Time { return now }
	if _, err := s.AddWithPin("记录创建时间", "备注", 0, false); err != nil {
		t.Fatal(err)
	}
	created := now.UnixMilli()
	if m.Tasks[0].CreatedAt != created {
		t.Fatal("new task did not record creation time")
	}
	now = now.Add(time.Hour)
	if _, err := s.UpdateDetails(1, "修改后的标题", "修改备注", 3, m.Tasks[0].Version); err != nil {
		t.Fatal(err)
	}
	p.close()
	reloaded, _, _ := profileFixture(t, dir)
	if reloaded.Tasks[0].CreatedAt != created {
		t.Fatal("edit or reload lost creation time")
	}
}

func TestLegacyAllTasksIgnoreSidebarOrder(t *testing.T) {
	items := []taskEntry{{ID: 2}, {ID: 1}, {ID: 3}}
	groups := taskTimeline(items, time.Now(), timelineAll)
	if len(groups) != 1 || groups[0].title != "历史任务" {
		t.Fatal("missing honest legacy group", groups)
	}
	for i, item := range groups[0].items {
		if item.ID != 3-i || item.CreatedAt != 0 {
			t.Fatal("legacy order must follow insertion IDs without inventing dates")
		}
	}
}
