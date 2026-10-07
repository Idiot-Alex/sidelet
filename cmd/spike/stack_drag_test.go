//go:build windows || darwin

package main

import (
	"math"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"

	"sidelet/internal/platform"
	"sidelet/internal/todo"
)

func TestStackDragUsesFrozenWorkAreaAndClampsBody(t *testing.T) {
	for _, scale := range []float64{1, 1.5, 2} {
		d := stackDrag{arranging: true, before: todo.EdgeStack{ID: 1, Side: "left", Offset: .35, Density: "normal"}, display: platform.Display{Scale: scale, WorkArea: platform.Rect{X: -1728 * scale, Y: 33 * scale, Width: 1728 * scale, Height: 1000 * scale}}, anchor: platform.Rect{X: 0, Y: 200, Width: 296, Height: 400}}
		for _, tc := range []struct {
			dx, dy, offset float64
			side           string
		}{{0, 20, .42, "left"}, {1000, 0, .4, "right"}, {100000, 100000, .7, "right"}, {-100000, -100000, .252, "left"}} {
			next, err := d.target(tc.dx, tc.dy)
			if err != nil || next.Side != tc.side || math.Abs(next.Offset-tc.offset) > 1e-9 {
				t.Fatalf("scale %v delta %+v: %+v %v", scale, tc, next, err)
			}
			if next.ID != d.before.ID || next.Density != d.before.Density {
				t.Fatal("target altered unrelated fields")
			}
		}
		d.before.Side = "right"
		d.anchor.X = 16
		next, err := d.target(-1000, 0)
		if err != nil || next.Side != "left" {
			t.Fatal("right to left drag failed")
		}
	}
}

func TestDragTargetRejectsInvalidatedSessionsAndOccupiedEdge(t *testing.T) {
	window := &application.WebviewWindow{}
	other := &application.WebviewWindow{}
	before := todo.EdgeStack{ID: 1, DisplayID: "display-a", Side: "right", Offset: .35, Density: "compact", UpdatedAt: 1}
	d := &stackDrag{revision: 4, overlay: &overlay{window: window}, before: before, display: platform.Display{Scale: 1, WorkArea: platform.Rect{Width: 1000, Height: 800}}, anchor: platform.Rect{X: 298, Y: 10, Width: 14, Height: 40}, viewportTop: 250}
	m := message{Window: window, Revision: 4, Y: 20}
	newController := func() *controller {
		return &controller{drag: d, snapshot: todo.Snapshot{Stacks: []todo.EdgeStack{before}}}
	}
	if _, err := newController().dragTarget(m); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		invalidate func(*controller, *message)
	}{
		{"cancelled", func(c *controller, _ *message) { c.drag = nil }},
		{"stale revision", func(_ *controller, m *message) { m.Revision = 3 }},
		{"other window", func(_ *controller, m *message) { m.Window = other }},
		{"changed mode", func(c *controller, _ *message) { c.arranging = true }},
		{"quiet", func(c *controller, _ *message) { c.quiet = true }},
		{"fullscreen", func(c *controller, _ *message) { c.fullscreen = true }},
		{"new saved layout", func(c *controller, _ *message) { c.snapshot.Stacks[0].UpdatedAt++ }},
		{"deleted stack", func(c *controller, _ *message) { c.snapshot.Stacks = nil }},
		{"occupied left", func(c *controller, m *message) {
			m.X = -100000
			c.snapshot.Stacks = append(c.snapshot.Stacks, todo.EdgeStack{ID: 2, DisplayID: "display-a", Side: "left"})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, request := newController(), m
			tc.invalidate(c, &request)
			if _, err := c.dragTarget(request); err == nil {
				t.Fatal("invalidated drag accepted")
			}
		})
	}
}

func TestStackDragRejectsInvalidAndInsufficientGeometry(t *testing.T) {
	d := stackDrag{display: platform.Display{Scale: 1, WorkArea: platform.Rect{Width: 1000, Height: 800}}, anchor: platform.Rect{Width: 296, Height: 400}}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := d.target(value, 0); err == nil {
			t.Fatal("non-finite pointer accepted")
		}
	}
	d.display.WorkArea.Height = 200
	if _, err := d.target(0, 0); err == nil {
		t.Fatal("impossible visible body accepted")
	}
}

func TestPassiveStackDragUsesCroppedOriginAndWholeGroup(t *testing.T) {
	for _, scale := range []float64{1, 1.5, 2} {
		d := stackDrag{
			before:  todo.EdgeStack{ID: 9, Side: "right", Offset: .35, Density: "compact"},
			display: platform.Display{Scale: scale, WorkArea: platform.Rect{X: -1728 * scale, Y: 33 * scale, Width: 1728 * scale, Height: 1000 * scale}},
			anchor:  platform.Rect{X: 298, Y: 10, Width: 14, Height: 94}, viewportTop: 293,
		}
		for _, tc := range []struct {
			dx, dy, offset float64
			side           string
		}{{0, 0, .35, "right"}, {0, 20, .37, "right"}, {-100000, 100000, .943, "left"}, {0, -100000, .057, "right"}} {
			next, err := d.target(tc.dx, tc.dy)
			if err != nil || next.Side != tc.side || math.Abs(next.Offset-tc.offset) > 1e-9 || next.ID != 9 || next.Density != "compact" {
				t.Fatalf("scale %v: %+v → %+v %v", scale, tc, next, err)
			}
		}
		// A normal group fits without the arrange mode's extra controls.
		d.display.WorkArea.Height = 130 * scale
		if _, err := d.target(0, 0); err != nil {
			t.Fatalf("compact work area: %v", err)
		}
		d.arranging = true
		if _, err := d.target(0, 0); err == nil {
			t.Fatal("arrange controls cannot fit")
		}
		d.viewportTop = math.NaN()
		if _, err := d.target(0, 0); err == nil {
			t.Fatal("invalid viewport origin accepted")
		}
	}
}
