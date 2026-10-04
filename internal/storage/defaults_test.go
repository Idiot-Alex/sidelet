package storage

import (
	"testing"
	"time"
)

func TestNewStackDefaultsDoNotModifyExistingLayout(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	s, err := OpenWithDefaults(dir, now, "left", "compact")
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Snapshot(now)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if len(first.Stacks) != 1 || first.Stacks[0].Side != "left" || first.Stacks[0].Density != "compact" {
		t.Fatal(first.Stacks)
	}
	s, err = OpenWithDefaults(dir, now.Add(time.Hour), "right", "relaxed")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	second, err := s.Snapshot(now)
	if err != nil || second.Stacks[0] != first.Stacks[0] {
		t.Fatal(second.Stacks, err)
	}
}
