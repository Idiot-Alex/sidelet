package spike

import (
	"testing"
	"time"
)

func TestQuickCardTransferAndReentry(t *testing.T) {
	now := time.Now()
	var s QuickSession
	s.Begin("stack-0", 1)
	s.Presence("stack-0", false, now)
	old := s.Revision
	if s.Expired(old, now.Add(499*time.Millisecond)) {
		t.Fatal("card closed while crossing the transparent gap")
	}
	s.Presence("quick", true, now.Add(200*time.Millisecond))
	if s.Expired(old, now.Add(time.Second)) || !s.CloseAt.IsZero() {
		t.Fatal("old close timer survived entry into the card")
	}
	s.Presence("quick", false, now.Add(time.Second))
	if !s.Expired(s.Revision, now.Add(1500*time.Millisecond)) {
		t.Fatal("card did not close after leaving both windows for 500ms")
	}
}

func TestQuickCardIgnoresOtherStack(t *testing.T) {
	now := time.Now()
	var s QuickSession
	s.Begin("stack-1", 5)
	s.Presence("stack-1", false, now)
	revision := s.Revision
	s.Presence("stack-0", true, now.Add(200*time.Millisecond))
	if !s.Expired(revision, now.Add(500*time.Millisecond)) {
		t.Fatal("another Stack retained the source Stack's card")
	}
}

func TestReopeningInvalidatesQueuedClose(t *testing.T) {
	now := time.Now()
	var s QuickSession
	s.Begin("stack-0", 1)
	s.Presence("stack-0", false, now)
	old := s.Revision
	s.Close()
	s.Begin("stack-1", 8)
	s.Presence("stack-1", false, now)
	if s.Expired(old, now.Add(time.Second)) || !s.Expired(s.Revision, now.Add(time.Second)) {
		t.Fatal("a stale timeout can close the newly opened card")
	}
}
