package main

import (
	"time"
)

const sidebarExtendDuration = 360 * time.Millisecond
const sidebarRetractDuration = 280 * time.Millisecond

// Paint slides out of the attached edge. The native window and drag handle
// stay in place, preserving capture and the pointer bridge into the card.
type edgeTitleReveal struct {
	expanded bool
	start    time.Time
	from     float32
	duration time.Duration
}

func (r *edgeTitleReveal) frame(expanded bool, now time.Time, reduced bool) (float32, bool) {
	if reduced {
		*r = edgeTitleReveal{expanded: expanded}
		return r.sample(now)
	}
	value, active := r.sample(now)
	if r.expanded != expanded {
		r.expanded, r.start, r.from = expanded, now, value
		base, distance := sidebarRetractDuration, value
		if expanded {
			base, distance = sidebarExtendDuration, 1-value
		}
		r.duration = time.Duration(float32(base) * distance)
		return value, r.duration > 0
	}
	return value, active
}

func (r *edgeTitleReveal) sample(now time.Time) (float32, bool) {
	target := float32(0)
	if r.expanded {
		target = 1
	}
	if r.duration <= 0 {
		return target, false
	}
	progress := max(0, float32(now.Sub(r.start))/float32(r.duration))
	if progress >= 1 {
		return target, false
	}
	return r.from + (target-r.from)*edgeEaseOut(progress), true
}

// Return the painted surface and the clipped preview viewport. Content keeps
// its full width and slides under the fixed marker, never reflowing its text.
func sidebarRevealGeometry(side string, markerWidth, progress float32) (x, width, bodyX, bodyWidth, contentX float32) {
	const full = float32(296)
	bodyWidth = (full - markerWidth) * max(0, min(1, progress))
	width = markerWidth + bodyWidth
	if side == "right" {
		x = stackWidth - width
		bodyX = x
	} else {
		bodyX = markerWidth
		contentX = bodyWidth - (full - markerWidth)
	}
	return
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
