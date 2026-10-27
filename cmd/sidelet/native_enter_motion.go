package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const popupEnterDuration = 160 * time.Millisecond

// A presentation token changes only on opening, never while typing, hovering
// or resizing. Animation affects paint, not window bounds or focus ownership.
type enterMotion struct {
	token uint64
	start time.Time
	ready bool
}

func (m *enterMotion) frame(token uint64, now time.Time, reduced bool) (float32, bool) {
	if !m.ready || m.token != token {
		m.token, m.start, m.ready = token, now, true
	}
	if reduced {
		m.start = now.Add(-popupEnterDuration)
	}
	x := max(0, min(1, float32(now.Sub(m.start))/float32(popupEnterDuration)))
	return 1 - (1-x)*(1-x)*(1-x), x < 1
}

func enterSurface(c *ui.Context, element ui.Element, token uint64) ui.Element {
	motion := ui.Local(element, "surface-enter", func() enterMotion { return enterMotion{} })
	progress, active := motion.frame(token, c.Now(), c.Preferences().ReduceMotion)
	if active {
		c.AnimationFrame()
	}
	// Keep controls legible and interactive from the first frame. A short
	// content fade avoids scaling text or moving a window under the pointer.
	return element.Opacity(.35 + .65*progress)
}
