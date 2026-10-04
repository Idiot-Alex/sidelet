package storage

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"sidelet/internal/todo"
)

func visibleOrder(state todo.Snapshot, stackID int64, now time.Time) []int64 {
	var ids []int64
	for _, task := range state.Todos {
		if task.StackID == stackID && !task.Completed && task.SnoozedUntil <= now.UnixMilli() {
			ids = append(ids, task.ID)
		}
	}
	return ids
}

func TestReorderPreservesHiddenSlotsContentAndRestart(t *testing.T) {
	s, dir := openTest(t)
	for i := 0; i < 12; i++ {
		applyTest(t, s, todo.Action{Type: "create", Title: fmt.Sprint(i), Pin: true, DueAt: int64ptr(testNow.Add(time.Hour).UnixMilli())}, testNow)
	}
	applyTest(t, s, todo.Action{Type: "snooze", ID: 2, Duration: "1h"}, testNow)
	before := applyTest(t, s, todo.Action{Type: "complete", ID: 4}, testNow)
	previous := visibleOrder(before, 1, testNow)
	order := append([]int64{12}, previous[:len(previous)-1]...)
	after := applyTest(t, s, todo.Action{Type: "reorder", StackID: 1, PreviousOrder: previous, Order: order}, testNow.Add(time.Second))
	if !reflect.DeepEqual(after.Stacks, before.Stacks) {
		t.Fatal("task reorder changed stack layout")
	}
	if !reflect.DeepEqual(visibleOrder(after, 1, testNow), order) || after.UndoID != before.UndoID || after.UndoUntil != before.UndoUntil {
		t.Fatal("reorder or Undo state changed incorrectly")
	}
	for _, original := range before.Todos {
		for _, current := range after.Todos {
			if original.ID != current.ID {
				continue
			}
			if (current.ID == 2 || current.ID == 4) && current.SortOrder != original.SortOrder {
				t.Fatal("hidden sort slot moved")
			}
			current.SortOrder = original.SortOrder
			if current != original {
				t.Fatalf("reorder modified task content: %+v", current)
			}
		}
	}
	s.Close()
	reopened, err := Open(dir, testNow.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if !reflect.DeepEqual(visibleOrder(viewTest(t, reopened, testNow), 1, testNow), order) {
		t.Fatal("restart lost manual order")
	}
	restored := applyTest(t, reopened, todo.Action{Type: "unsnooze", ID: 2}, testNow)
	restored = applyTest(t, reopened, todo.Action{Type: "reopen", ID: 4}, testNow)
	for _, item := range restored.Todos {
		if item.ID == 2 && item.SortOrder != 20 || item.ID == 4 && item.SortOrder != 40 {
			t.Fatal("restored task lost original slot")
		}
	}
}

func TestReorderRejectsStalePartialDuplicateAndForeignOrders(t *testing.T) {
	s, _ := openTest(t)
	for _, title := range []string{"A", "B", "C"} {
		applyTest(t, s, todo.Action{Type: "create", Title: title, Pin: true}, testNow)
	}
	applyTest(t, s, todo.Action{Type: "create", Title: "unfixed"}, testNow)
	if _, err := s.db.Exec(`INSERT INTO edge_stacks(id,display_id,side,offset,density,created_at,updated_at) VALUES(2,'another','left',0.5,'normal',1,1)`); err != nil {
		t.Fatal(err)
	}
	before := applyTest(t, s, todo.Action{Type: "create", Title: "other stack", Pin: true, StackID: 2}, testNow)
	for _, action := range []todo.Action{
		{StackID: 1, PreviousOrder: []int64{1, 2, 3}, Order: []int64{3, 1}},
		{StackID: 1, PreviousOrder: []int64{1, 2, 3}, Order: []int64{1, 1, 3}},
		{StackID: 1, PreviousOrder: []int64{1, 2, 3}, Order: []int64{4, 2, 3}},
		{StackID: 1, PreviousOrder: []int64{1, 2, 3}, Order: []int64{5, 2, 3}},
		{StackID: 2, PreviousOrder: []int64{1, 2, 3}, Order: []int64{3, 2, 1}},
	} {
		action.Type = "reorder"
		if _, err := s.Apply(action, testNow); err == nil {
			t.Fatalf("invalid reorder accepted: %+v", action)
		}
		if !reflect.DeepEqual(viewTest(t, s, testNow), before) {
			t.Fatal("rejected reorder changed state")
		}
	}
	valid := todo.Action{Type: "reorder", StackID: 1, PreviousOrder: []int64{1, 2, 3}, Order: []int64{3, 1, 2}}
	applyTest(t, s, valid, testNow)
	if _, err := s.Apply(valid, testNow); err == nil {
		t.Fatal("stale order overwrote a committed reorder")
	}
	valid.PreviousOrder = []int64{3, 1, 2}
	valid.Order = []int64{2, 3, 1}
	applyTest(t, s, todo.Action{Type: "snooze", ID: 2, Duration: "30m"}, testNow)
	if _, err := s.Apply(valid, testNow); err == nil {
		t.Fatal("drag included a newly hidden task")
	}
}

func TestReorderWriteFailureRollsBackEverySlot(t *testing.T) {
	s, _ := openTest(t)
	for _, title := range []string{"A", "B", "C"} {
		applyTest(t, s, todo.Action{Type: "create", Title: title, Pin: true}, testNow)
	}
	before := viewTest(t, s, testNow)
	if _, err := s.db.Exec(`CREATE TRIGGER fail_order BEFORE UPDATE ON edge_stack_items WHEN OLD.todo_id=1 BEGIN SELECT RAISE(ABORT,'forced mid-order failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(todo.Action{Type: "reorder", StackID: 1, PreviousOrder: []int64{1, 2, 3}, Order: []int64{3, 1, 2}}, testNow.Add(time.Second)); err == nil {
		t.Fatal("write failure was ignored")
	}
	if !reflect.DeepEqual(viewTest(t, s, testNow), before) {
		t.Fatal("partial order survived rollback")
	}
}
