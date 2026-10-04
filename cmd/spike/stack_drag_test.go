//go:build windows || darwin

package main

import (
	"math"
	"sidelet/internal/platform"
	"sidelet/internal/todo"
	"testing"
)

func TestStackDragUsesFrozenWorkAreaAndClampsBody(t *testing.T) {
	for _, scale := range []float64{1, 1.5, 2} {
		d := stackDrag{before: todo.EdgeStack{ID: 1, Side: "left", Offset: .35, Density: "normal"}, display: platform.Display{Scale: scale, WorkArea: platform.Rect{X: -1728 * scale, Y: 33 * scale, Width: 1728 * scale, Height: 1000 * scale}}, anchor: platform.Rect{X: 0, Y: 200, Width: 296, Height: 400}}
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
