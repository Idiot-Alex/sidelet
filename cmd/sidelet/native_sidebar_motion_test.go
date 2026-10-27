package main

import (
	"testing"
	"time"
)

func TestSidebarRetractionReversesContinuously(t *testing.T) {
	var r edgeTitleReveal
	now := time.Unix(1000, 0)
	r.frame(true, now, false)
	start := now.Add(sidebarExtendDuration)
	if p, active := r.frame(false, start, false); p != 1 || !active {
		t.Fatal("exit snapped closed", p, active)
	}
	middle := start.Add(sidebarRetractDuration / 2)
	p, active := r.frame(false, middle, false)
	if !active || p <= 0 || p >= 1 {
		t.Fatal("missing retract frame", p)
	}
	if reversed, active := r.frame(true, middle, false); reversed != p || !active {
		t.Fatal("re-entry jumped instead of reversing", reversed, p)
	}
	if next, _ := r.frame(true, middle.Add(20*time.Millisecond), false); next <= p {
		t.Fatal("re-entry did not extend")
	}
	r.frame(false, middle.Add(time.Second), false)
	if p, active := r.frame(false, middle.Add(2*time.Second), false); p != 0 || active {
		t.Fatal("exit did not settle", p, active)
	}
	r.frame(true, middle.Add(3*time.Second), false)
	if p, active := r.frame(false, middle.Add(3*time.Second), true); p != 0 || active {
		t.Fatal("reduced motion must close immediately")
	}
}

func TestSidebarRetainsNativeRegionOnlyWhileRetracting(t *testing.T) {
	m := newModel()
	m.UITheme, m.Hover, m.Opened = "mac", -1, -1
	m.sidebarRevealed = map[int]bool{0: true}
	if !m.sidebarPresented(0) || m.markerRect(0).Width != 20 {
		t.Fatal("exiting drawer lost its native surface")
	}
	for _, s := range stackInputShapes(m) {
		if s.task == 0 && s.rect.Width != 296 {
			t.Fatal("exit region was clipped early", s)
		}
	}
	m.Dragging = true
	if m.sidebarPresented(0) {
		t.Fatal("exit animation changed drag geometry")
	}
	m.Dragging = false
	delete(m.sidebarRevealed, 0)
	if m.sidebarPresented(0) || m.markerRect(0).Width != 14 {
		t.Fatal("finished drawer still owns its expanded region")
	}
}

func TestSidebarExtendsFromAttachedEdgeWithoutTextReflow(t *testing.T) {
	for _, side := range []string{"left", "right"} {
		previousWidth := float32(0)
		for _, progress := range []float32{0, .25, .5, .75, 1} {
			x, width, bodyX, bodyWidth, contentX := sidebarRevealGeometry(side, 20, progress)
			if width < previousWidth || width != 20+276*progress || bodyWidth != width-20 {
				t.Fatal("invalid extension", side, progress, width, bodyWidth)
			}
			if side == "left" {
				if x != 0 || bodyX != 20 || bodyX+contentX+276 != width {
					t.Fatal("left attachment must extend right", progress, x, bodyX, contentX)
				}
			} else if x+width != stackWidth || bodyX != x || contentX != 0 {
				t.Fatal("right attachment must extend left", progress, x, bodyX, contentX)
			}
			previousWidth = width
		}
	}
}
