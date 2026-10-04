package spike

import (
	"testing"
	"time"
)

func TestSnoozeKeepsDueAndOrder(t *testing.T) {
	zone := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2026, 10, 4, 23, 40, 0, 0, zone)
	state := New(now)
	due := state.Todos[0].DueAt
	Apply(&state, Action{Type: "snooze", ID: 1, Duration: "tomorrow"}, now)
	want := time.Date(2026, 10, 5, 9, 0, 0, 0, zone).UnixMilli()
	if state.Todos[0].SnoozedUntil != want || state.Todos[0].DueAt != due {
		t.Fatalf("unexpected snooze: %+v", state.Todos[0])
	}
	for i, todo := range state.Todos {
		if todo.ID != int64(i+1) {
			t.Fatal("fixture order changed")
		}
	}
}
func TestUndoDeadline(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		delay     time.Duration
		completed bool
	}{{4999 * time.Millisecond, false}, {5001 * time.Millisecond, true}} {
		state := New(now)
		Apply(&state, Action{Type: "complete", ID: 1}, now)
		Apply(&state, Action{Type: "undo"}, now.Add(test.delay))
		if state.Todos[0].Completed != test.completed {
			t.Fatalf("completed=%v after %s", state.Todos[0].Completed, test.delay)
		}
		if !test.completed && state.Todos[0].CompletedAt != 0 {
			t.Fatal("undo retained completion timestamp")
		}
	}
}
func TestCompletionIsIdempotent(t *testing.T) {
	now := time.Now()
	state := New(now)
	Apply(&state, Action{Type: "complete", ID: 1}, now)
	Apply(&state, Action{Type: "complete", ID: 1}, now.Add(time.Second))
	if state.UndoUntil != now.Add(5*time.Second).UnixMilli() {
		t.Fatal("repeated completion extended undo deadline")
	}
}
