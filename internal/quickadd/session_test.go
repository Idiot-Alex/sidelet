package quickadd

import (
	"testing"
	"time"
)

func TestCancelRepeatAndStaleSubmission(t *testing.T) {
	var s Session
	now := time.Now()
	if !s.Begin(now) {
		t.Fatal("cannot open")
	}
	rev := s.Revision
	if s.Begin(now.Add(time.Hour)) || s.Reference != now {
		t.Fatal("repeated entry replaced the session/date")
	}
	if !s.Cancel() || s.Open || s.ResetVersion != 1 {
		t.Fatal("cancel did not dismiss/reset")
	}
	if s.StartSave(rev) {
		t.Fatal("save after cancel accepted")
	}
	s.Begin(now.Add(time.Hour))
	if s.StartSave(rev) {
		t.Fatal("stale save from prior opening accepted")
	}
	if !s.StartSave(s.Revision) || s.StartSave(s.Revision) || s.Cancel() || s.Begin(now) {
		t.Fatal("duplicate submission or cancellation during commit accepted")
	}
	if !s.FinishSave(true) || s.Open || s.Saving || s.ResetVersion != 2 {
		t.Fatal("commit did not clear session")
	}
}

func TestFailureAndExternalFocusWhileSaving(t *testing.T) {
	var s Session
	s.Begin(time.Now())
	s.StartSave(s.Revision)
	if s.FinishSave(false) || !s.Open || s.Saving || s.ResetVersion != 0 {
		t.Fatal("failure cleared the unfinished draft")
	}
	if !s.StartSave(s.Revision) {
		t.Fatal("retry rejected")
	}
	s.Blur()
	if s.FinishSave(true) {
		t.Fatal("commit after external focus change must not restore focus")
	}
	if s.Open || s.Saving || s.ResetVersion != 1 {
		t.Fatal("background commit did not reset")
	}
	s.Begin(time.Now())
	s.Blur()
	reset := s.ResetVersion
	s.Begin(time.Now())
	if s.ResetVersion != reset {
		t.Fatal("blur/reopen discarded input")
	}
}
