package reminder

import (
	"errors"
	"reflect"
	"sidelet/internal/storage"
	"testing"
	"time"
)

type repoFake struct {
	rows     []storage.Reminder
	failMark bool
}

func (r *repoFake) Reminders() ([]storage.Reminder, error) { return r.rows, nil }
func (r *repoFake) MarkReminderSent(id int64, now time.Time) error {
	if r.failMark {
		return errors.New("disk failure")
	}
	for i := range r.rows {
		if r.rows[i].ID == id {
			r.rows[i].SentAt = now.UnixMilli()
		}
	}
	return nil
}

type systemFake struct {
	state         State
	sent, removed []string
	fail          bool
}

func (s *systemFake) State() (State, error) { return s.state, nil }
func (s *systemFake) Deliver(id, title, body string) error {
	if s.fail {
		return errors.New("OS unavailable")
	}
	s.sent = append(s.sent, id)
	s.state.Delivered = append(s.state.Delivered, id)
	return nil
}
func (s *systemFake) Remove(ids []string) error { s.removed = append(s.removed, ids...); return nil }

func TestSchedulerDeadlineWakeRestartAndIdle(t *testing.T) {
	now := time.Unix(1000, 0)
	r := &repoFake{rows: []storage.Reminder{{ID: 1, At: now.Add(time.Minute).UnixMilli()}, {ID: 2, At: now.Add(time.Hour).UnixMilli()}}}
	s := &systemFake{state: State{Authorization: "authorized"}}
	result, e := Sync(r, s, "test.", now)
	if e != nil || result.Next != now.Add(time.Minute) || len(s.sent) != 0 {
		t.Fatal("future reminder fired early")
	}
	result, e = Sync(r, s, "test.", now.Add(2*time.Hour))
	if e != nil || !result.Next.IsZero() || !result.Changed || len(s.sent) != 2 {
		t.Fatal("wake catch-up did not submit exactly once")
	}
	s.state.Delivered = nil // User cleared Notification Center; durable receipts still suppress repeats.
	result, e = Sync(r, s, "test.", now.Add(3*time.Hour))
	if e != nil || len(s.sent) != 2 || result.Changed || !result.Next.IsZero() {
		t.Fatal("restart/cleared notification duplicated delivery or left an idle timer")
	}
}
func TestSchedulerDeniedCompletedAndProfileScopedCleanup(t *testing.T) {
	now := time.Unix(1000, 0)
	r := &repoFake{rows: []storage.Reminder{{ID: 1, At: 1}, {ID: 2, At: 1, Completed: true}}}
	s := &systemFake{state: State{Authorization: "denied", Pending: []string{"own.2", "own.3", "other.1"}}}
	result, e := Sync(r, s, "own.", now)
	if e != nil || len(s.sent) != 0 || r.rows[0].SentAt != 0 || !result.Next.IsZero() {
		t.Fatal("denied reminder consumed or retried in a loop")
	}
	if !reflect.DeepEqual(s.removed, []string{"own.2", "own.3"}) {
		t.Fatal("cleanup affected another profile or missed cancelled requests")
	}
	s.state.Authorization = "authorized"
	_, e = Sync(r, s, "own.", now)
	if e != nil || !reflect.DeepEqual(s.sent, []string{"own.1"}) {
		t.Fatal("permission recovery failed")
	}
}
func TestSchedulerRecoversFailuresAndAcceptanceCrashWindow(t *testing.T) {
	now := time.Unix(1000, 0)
	r := &repoFake{rows: []storage.Reminder{{ID: 1, At: 1}}}
	s := &systemFake{state: State{Authorization: "authorized"}, fail: true}
	if _, e := Sync(r, s, "own.", now); e == nil || r.rows[0].SentAt != 0 {
		t.Fatal("failed OS submission was marked sent")
	}
	s.fail = false
	r.failMark = true
	if _, e := Sync(r, s, "own.", now); e == nil || len(s.sent) != 1 {
		t.Fatal("disk failure not surfaced")
	}
	r.failMark = false
	if _, e := Sync(r, s, "own.", now); e != nil || len(s.sent) != 1 || r.rows[0].SentAt == 0 {
		t.Fatal("recovery resent an OS-accepted notification")
	}
}
