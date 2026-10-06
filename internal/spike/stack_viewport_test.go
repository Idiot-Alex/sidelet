package spike

import (
	"math"
	"testing"
)

func TestStackViewportPreservesScreenPositionsAndOverflow(t *testing.T) {
	for _, tc := range []struct {
		count                           int
		area, offset, item, top, height float64
	}{
		{2, 1000, .35, 44, 293, 114},
		{1, 1000, 0, 44, 0, 64},
		{1, 1000, 1, 44, 936, 64},
		{9, 1000, .35, 44, 118, 464},
		{40, 100, 0, 44, 0, 64},
		{8, 40, 1, 64, 0, 40},
		{0, 1000, .35, 44, 340, 20},
	} {
		top, height := StackViewport(tc.count, tc.area, tc.offset, tc.item, false)
		if top != tc.top || height != tc.height {
			t.Fatalf("%+v: top=%v height=%v", tc, top, height)
		}
	}
	for _, area := range []float64{8, 40, 180, 1007, 1600} {
		for _, item := range []float64{40, 44, 56} {
			for _, count := range []int{0, 1, 2, 8, 9, 40} {
				for _, offset := range []float64{0, .26, .35, 1} {
					top, height := StackViewport(count, area, offset, item, false)
					if top < 0 || height <= 0 || top+height > area || math.IsNaN(height) {
						t.Fatalf("bounds escaped area: area=%v count=%d density=%v offset=%v top=%v height=%v", area, count, item, offset, top, height)
					}
					fullTop, fullHeight := StackViewport(count, area, offset, item, true)
					if fullTop != 0 || fullHeight != area {
						t.Fatal("arrange / Undo must restore whole WorkArea")
					}
				}
			}
		}
	}
}
