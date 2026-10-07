package spike

import "testing"

func TestPopupSwitchRejectsLatePackets(t *testing.T) {
	var s PopupSession
	add := s.Prepare("add")
	if !s.Rendered("add", add) || s.Rendered("add", add) {
		t.Fatal("presentation must acknowledge layout exactly once")
	}
	s.Close("add")
	if s.Accept("add", add) {
		t.Fatal("closed popup accepted delayed save or layout")
	}
	card := s.Prepare("quick")
	if s.Accept("add", add) || s.Rendered("add", card) {
		t.Fatal("old view may not acknowledge or operate on the card")
	}
	s.Close("add")
	if !s.Accept("quick", card) || !s.Rendered("quick", card) {
		t.Fatal("closing the inactive add view changed the card")
	}
	s.Close("quick")
	next := s.Prepare("quick")
	if s.Accept("quick", card) || s.Rendered("quick", card) || !s.Rendered("quick", next) {
		t.Fatal("same-view reopen accepted a previous generation")
	}
}

func TestPopupCancelledBeforeLayoutCannotShow(t *testing.T) {
	var s PopupSession
	add := s.Prepare("add")
	s.Close("add")
	card := s.Prepare("quick")
	if s.Rendered("add", add) || !s.Preparing || !s.Rendered("quick", card) {
		t.Fatal("cancelled layout stole the next presentation")
	}
}
