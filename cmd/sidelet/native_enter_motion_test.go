package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestEntranceOnlyRestartsOnPresentation(t *testing.T) {
	var motion enterMotion
	now := time.Unix(100, 0)
	if p, active := motion.frame(1, now, false); p != 0 || !active {
		t.Fatal("opening must start animation", p, active)
	}
	if p, active := motion.frame(1, now.Add(80*time.Millisecond), false); p != .875 || !active {
		t.Fatal("expected short ease-out", p, active)
	}
	for _, elapsed := range []time.Duration{popupEnterDuration, time.Second, time.Hour} {
		if p, active := motion.frame(1, now.Add(elapsed), false); p != 1 || active {
			t.Fatal("ordinary rerenders restarted animation", p, active)
		}
	}
	if p, active := motion.frame(2, now.Add(time.Hour), false); p != 0 || !active {
		t.Fatal("reopening must replay entrance", p, active)
	}
	if p, active := motion.frame(2, now.Add(time.Hour), true); p != 1 || active {
		t.Fatal("reduced motion must finish immediately", p, active)
	}
	if p, active := motion.frame(2, now.Add(time.Hour), false); p != 1 || active {
		t.Fatal("disabling reduced motion replayed an old entrance", p, active)
	}
}

func TestEntranceKeepsControlsClickableAndLayoutStable(t *testing.T) {
	clicked := false
	u := ui.NewTester(func(c *ui.Context) {
		enterSurface(c, ui.Column(c).Fill().Padding(16), 0).Children(func() {
			if ui.Button(c, "Confirm").Label("Confirm").Clicked() {
				clicked = true
			}
		})
	}, 320, 200)
	before := visualRect(t, u, "Confirm")
	u.Click("Confirm")
	if !clicked {
		t.Fatal("entrance blocked immediate click")
	}
	u.SetPreferences(ui.Preferences{ReduceMotion: true})
	if after := visualRect(t, u, "Confirm"); before != after {
		t.Fatal("entrance moved click targets", before, after)
	}
}
