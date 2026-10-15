package main

import (
	"time"
)

// Only title ink moves. Native input regions and the label/handle geometry
// reach their full size immediately, preserving the bridge into the card.
type edgeTitleReveal struct {
	expanded bool
	start    time.Time
}

func (r *edgeTitleReveal) frame(expanded bool, now time.Time, reduced bool) (float32, bool) {
	if !expanded {
		r.expanded = false
		return 0, false
	}
	if !r.expanded {
		r.expanded, r.start = true, now
	}
	if reduced {
		r.start = now.Add(-200 * time.Millisecond)
		return 1, false
	}
	progress := float32(now.Sub(r.start)) / float32(200*time.Millisecond)
	if progress >= 1 {
		return 1, false
	}
	return edgeEaseOut(max(0, progress)), true
}

// CSS ease-out is cubic-bezier(0,0,.58,1), not MyGo's cubic EaseOut.
func edgeEaseOut(x float32) float32 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	lo, hi := float32(0), float32(1)
	for range 16 {
		u := (lo + hi) / 2
		if 3*(1-u)*u*u*.58+u*u*u < x {
			lo = u
		} else {
			hi = u
		}
	}
	u := (lo + hi) / 2
	return 3*(1-u)*u*u + u*u*u
}
