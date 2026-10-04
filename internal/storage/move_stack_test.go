package storage

import (
	"math"
	"reflect"
	"sidelet/internal/todo"
	"testing"
	"time"
)

func movableStack(t *testing.T, s *Store) todo.Snapshot {
	t.Helper()
	applyTest(t, s, todo.Action{Type: "create", Title: "A", Pin: true}, testNow)
	applyTest(t, s, todo.Action{Type: "create", Title: "B", Pin: true}, testNow)
	state := applyTest(t, s, todo.Action{Type: "complete", ID: 2}, testNow)
	stack := state.Stacks[0]
	stack.DisplayID = "display-a"
	state, err := s.SaveStack(stack, testNow)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestMoveStackPreservesTasksOrderUndoDensityAndRestart(t *testing.T) {
	s, dir := openTest(t)
	before := movableStack(t, s)
	previous := before.Stacks[0]
	next := previous
	next.Side = "left"
	next.Offset = .73
	next.Density = "relaxed"
	after, err := s.MoveStack(previous, next, testNow.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Todos, after.Todos) || before.UndoID != after.UndoID || before.UndoUntil != after.UndoUntil {
		t.Fatal("moving changed tasks/order/Undo")
	}
	expected := previous
	expected.Side = "left"
	expected.Offset = .73
	expected.UpdatedAt = testNow.Add(time.Second).UnixMilli()
	if after.Stacks[0] != expected {
		t.Fatalf("layout or unrelated fields changed: %+v", after.Stacks[0])
	}
	s.Close()
	reopened, err := Open(dir, testNow.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	state := viewTest(t, reopened, testNow)
	if state.Stacks[0] != expected || !reflect.DeepEqual(state.Todos, before.Todos) {
		t.Fatal("restart lost position or changed tasks")
	}
}

func TestMoveStackRejectsStaleOccupiedAndInvalidTargets(t *testing.T) {
	s, _ := openTest(t)
	before := movableStack(t, s)
	previous := before.Stacks[0]
	for _, offset := range []float64{-.1, 1.1, math.NaN(), math.Inf(1)} {
		next := previous
		next.Offset = offset
		if _, err := s.MoveStack(previous, next, testNow); err == nil {
			t.Fatal("invalid offset accepted")
		}
	}
	for _, mutate := range []func(*todo.EdgeStack){func(s *todo.EdgeStack) { s.ID = 99 }, func(s *todo.EdgeStack) { s.Side = "top" }, func(s *todo.EdgeStack) { s.DisplayID = "disconnected" }} {
		next := previous
		mutate(&next)
		if _, err := s.MoveStack(previous, next, testNow); err == nil {
			t.Fatal("invalid target accepted")
		}
	}
	if !reflect.DeepEqual(viewTest(t, s, testNow), before) {
		t.Fatal("invalid move changed data")
	}
	if _, err := s.db.Exec(`INSERT INTO edge_stacks(id,display_id,side,offset,density,created_at,updated_at) VALUES(2,'display-a','left',.5,'normal',1,1)`); err != nil {
		t.Fatal(err)
	}
	next := previous
	next.Side = "left"
	if _, err := s.MoveStack(previous, next, testNow); err == nil {
		t.Fatal("occupied edge accepted")
	}
	next = previous
	next.Offset = .6
	if _, err := s.MoveStack(previous, next, testNow.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	saved := viewTest(t, s, testNow)
	next.Offset = .8
	if _, err := s.MoveStack(previous, next, testNow.Add(2*time.Second)); err == nil {
		t.Fatal("stale move overwrote a newer layout")
	}
	if !reflect.DeepEqual(viewTest(t, s, testNow), saved) {
		t.Fatal("rejected move changed data")
	}
}

func TestMoveStackWriteFailureRollsBack(t *testing.T) {
	s, _ := openTest(t)
	before := movableStack(t, s)
	previous := before.Stacks[0]
	next := previous
	next.Side = "left"
	next.Offset = .8
	if _, err := s.db.Exec(`CREATE TRIGGER fail_move BEFORE UPDATE ON edge_stacks BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MoveStack(previous, next, testNow.Add(time.Second)); err == nil {
		t.Fatal("injected failure succeeded")
	}
	if !reflect.DeepEqual(viewTest(t, s, testNow), before) {
		t.Fatal("failed move changed state")
	}
}
