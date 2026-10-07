package todo

import (
	"testing"
	"time"
)

func TestScheduleValidationAndEffectiveTime(t *testing.T) {
	for _, due := range []int64{-1, 253402300800000} {
		if ValidateDeadline(due, false) == nil {
			t.Fatal("invalid deadline accepted", due)
		}
	}
	if ValidateDeadline(0, true) == nil {
		t.Fatal("reminder without deadline accepted")
	}
	for _, due := range []int64{0, 1, 253402300799999} {
		if err := ValidateDeadline(due, due > 0); err != nil {
			t.Fatal(err)
		}
	}
	if EffectiveReminderAt(2000, 3000) != 3000 || EffectiveReminderAt(4000, 3000) != 4000 {
		t.Fatal("snooze must never advance a later deadline")
	}
}

func TestTemporaryExpiryKeepsFullUndoWindow(t *testing.T) {
	at := int64(1000000)
	if TemporaryExpired(true, true, at, time.UnixMilli(at+5000)) {
		t.Fatal("deleted before undo window ended")
	}
	if !TemporaryExpired(true, true, at, time.UnixMilli(at+5001)) {
		t.Fatal("temporary completion did not expire")
	}
	for _, state := range []struct {
		temporary, completed bool
		at                   int64
	}{{false, true, at}, {true, false, at}, {true, true, 0}} {
		if TemporaryExpired(state.temporary, state.completed, state.at, time.UnixMilli(at+6000)) {
			t.Fatal("removed permanent, undone, or undated task")
		}
	}
}
