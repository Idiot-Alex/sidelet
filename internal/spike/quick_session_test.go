package spike

import (
	"testing"
	"time"
)

func TestQuickCardTransferAndReentry(t *testing.T) {
	now := time.Now()
	var s QuickSession
	s.Begin("stack-0", "quick", 1)
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

func TestSharedPopupHoverTransferUsesPhysicalCardWindow(t *testing.T) {
	now := time.Unix(100, 0)
	var s QuickSession
	s.Begin("stack-0", "popup", 1)
	request := s.RequestRevision
	s.Presence("stack-0", false, now)
	old := s.Revision
	s.Presence("popup", true, now.Add(200*time.Millisecond))
	if !s.CloseAt.IsZero() || s.Expired(old, now.Add(time.Second)) || !s.CanPresent(request, now.Add(time.Second), false) {
		t.Fatal("moving into the shared popup did not retain the card")
	}
	s.Presence("quick", false, now.Add(time.Second))
	if !s.CloseAt.IsZero() {
		t.Fatal("a different window affected the shared popup")
	}
	s.Presence("popup", false, now.Add(2*time.Second))
	leaveRevision := s.Revision
	if !s.Expired(leaveRevision, now.Add(2500*time.Millisecond)) {
		t.Fatal("leaving the shared popup did not close after 500ms")
	}
	s.Presence("popup", true, now.Add(2400*time.Millisecond))
	if !s.CloseAt.IsZero() || s.Expired(leaveRevision, now.Add(3*time.Second)) {
		t.Fatal("shared popup re-entry did not cancel pending close")
	}
}

func TestQuickCardIgnoresOtherStack(t *testing.T) {
	now := time.Now()
	var s QuickSession
	s.Begin("stack-1", "quick", 5)
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
	s.Begin("stack-0", "quick", 1)
	s.Presence("stack-0", false, now)
	old := s.Revision
	s.Close()
	s.Begin("stack-1", "quick", 8)
	s.Presence("stack-1", false, now)
	if s.Expired(old, now.Add(time.Second)) || !s.Expired(s.Revision, now.Add(time.Second)) {
		t.Fatal("a stale timeout can close the newly opened card")
	}
}

func TestColdCardLoadKeepsLatestTaskAndIgnoresCancelledReplies(t *testing.T) {
	now := time.Unix(100, 0)
	var s QuickSession
	s.Begin("stack-0", "quick", 1)
	first := s.RequestRevision
	s.Presence("stack-0", false, now)
	s.Presence("stack-0", true, now.Add(100*time.Millisecond))
	if !s.CanPresent(first, now.Add(200*time.Millisecond), false) {
		t.Fatal("pointer updates discarded the loading card")
	}
	s.Begin("stack-1", "quick", 8)
	latest := s.RequestRevision
	if s.CanPresent(first, now, true) || !s.CanPresent(latest, now, true) || s.TodoID != 8 {
		t.Fatal("a late first-task renderer reply can reveal the wrong task")
	}
	s.Close() // Escape, quiet mode, foreground change or opening another window.
	if s.CanPresent(latest, now, true) {
		t.Fatal("the renderer reopened a cancelled card")
	}
	s.Begin("stack-0", "quick", 2)
	if s.CanPresent(latest, now, true) || !s.CanPresent(s.RequestRevision, now, false) {
		t.Fatal("an old renderer reply affected the next opening")
	}
}

func TestLoadingCardHonoursLeaveDeadlineAndActiveEditing(t *testing.T) {
	now := time.Unix(100, 0)
	var s QuickSession
	s.Begin("stack-0", "quick", 1)
	request := s.RequestRevision
	s.Presence("stack-0", false, now)
	if !s.CanPresent(request, now.Add(499*time.Millisecond), false) || s.CanPresent(request, now.Add(500*time.Millisecond), false) {
		t.Fatal("a delayed render ignored the 500ms leave deadline")
	}
	if !s.CanPresent(request, now.Add(time.Second), true) {
		t.Fatal("keyboard/editing was discarded while loading")
	}
	s.Close()
	if s.CanPresent(request, now.Add(time.Second), true) {
		t.Fatal("editing allowed a cancelled request to return")
	}
}
