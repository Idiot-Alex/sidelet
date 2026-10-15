package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNativeDragKeepsMovingWithEqualLocalCoordinates(t *testing.T) {
	m := newModel()
	m.WorkWidth, m.WorkHeight, m.Y = 1600, 1000, 300
	m.startDrag(7, 330)
	for _, want := range []int{330, 360, 390} {
		p := nativeDragScreenPoint(m, 7, stackHeight-60)
		m.moveDrag(p.X, p.Y)
		if m.Y != want {
			t.Fatalf("equal local coordinates stopped movement: Y=%d, want %d", m.Y, want)
		}
	}
	p := nativeDragScreenPoint(m, 7, stackHeight-30)
	m.moveDrag(p.X, p.Y)
	m.endDrag()
	if m.Y != 390 || m.Dragging {
		t.Fatalf("release moved window again: %+v", m)
	}
}

func TestDragUsesDeliveredCoordinatesWhileWindowMoves(t *testing.T) {
	for _, side := range []string{"left", "right"} {
		t.Run(side, func(t *testing.T) {
			m := newModel()
			m.Side = side
			m.WorkWidth = 1600
			m.WorkHeight = 1000
			m.Y = 300
			x := float32(7)
			if side == "right" {
				m.X = 1600 - stackWidth
				x = stackWidth - 7
			}
			v := &views{m: m}
			v.pointer = func(i int, ev ui.InputEvent) bool {
				p := eventScreenPoint(m, i, ev)
				switch ev.Kind {
				case ui.InputPointerDown:
					m.startDrag(p.X, p.Y)
				case ui.InputPointerMove:
					if !m.Dragging {
						return false
					}
					m.moveDrag(p.X, p.Y)
				case ui.InputPointerUp:
					m.endDrag()
				default:
					return false
				}
				return true
			}
			u := ui.NewTester(v.stack, stackWidth, stackHeight)
			u.Press(x, 80)
			u.Move(x, 180)
			if m.Y != 400 {
				t.Fatalf("event did not move window: Y=%d", m.Y)
			}
			// The cursor is now over the same local point in the moved window.
			// A later OS-cursor sample is neither needed nor allowed to shift it.
			u.Release(x, 80)
			if m.Y != 400 || m.Dragging || m.Side != side {
				t.Fatalf("release drifted: %+v", m)
			}
		})
	}
}

func TestInputRegionsFollowVisibleRowsAndLeaveBlankSpace(t *testing.T) {
	for _, side := range []string{"left", "right"} {
		t.Run(side, func(t *testing.T) {
			m := newModel()
			m.Side = side
			v := &views{m: m}
			u := ui.NewTester(v.stack, stackWidth, stackHeight)
			check := func() {
				t.Helper()
				regions := stackInputRegions(m)
				img := u.Image()
				// Every painted pixel has a native region; blank rows and gaps
				// have none. Rounded corner alpha is handled by the native layer.
				for y := 0; y < stackHeight; y++ {
					for x := 0; x < stackWidth; x++ {
						hit := false
						for _, r := range regions {
							q := r.rect
							if x >= q.X && x < q.X+q.Width && y >= q.Y && y < q.Y+q.Height {
								hit = true
							}
						}
						if img.RGBAAt(x, y).A > 0 && !hit {
							t.Fatalf("painted pixel (%d,%d) has no region", x, y)
						}
						if hit && (y < 8 || y >= 152 || y == 54 || y == 104) {
							t.Fatalf("transparent gap (%d,%d) owns input", x, y)
						}
					}
				}
			}
			check()
			x := float32(7)
			if side == "right" {
				x = stackWidth - 7
			}
			u.Move(x, 30)
			check()
			u.Move(140, 30)
			check()
			m.open(0)
			u.Frame()
			check()
			m.startDrag(7, 30)
			u.Frame()
			check()
			m.cancelDrag()
			m.complete()
			u.Move(140, 160)
			check()
			for _, r := range stackInputRegions(m) {
				if r.task == 0 {
					t.Fatal("completed task still owns input")
				}
			}
		})
	}
}
