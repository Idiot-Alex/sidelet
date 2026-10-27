//go:build windows

package main

import "testing"

func TestSmoothWindowRegionPreservesCurveCoverageAndHitThrough(t *testing.T) {
	contains := inputGDI.NewProc("PtInRegion")
	for _, dpi := range []int{96, 120, 144, 192} {
		size, radius := winPX(56, dpi), winPX(28, dpi)
		rgn, err := smoothWindowRegion(0, 0, size, size, radius)
		if rgn == 0 {
			t.Fatalf("dpi %d: %v", dpi, err)
		}
		// Every pixel touched by the ideal circle must survive binary clipping.
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				dx := max(0, max(radius-x-1, x-radius))
				dy := max(0, max(radius-y-1, y-radius))
				if dx*dx+dy*dy < radius*radius {
					if hit, _, _ := contains.Call(rgn, uintptr(x), uintptr(y)); hit == 0 {
						t.Errorf("dpi %d: clipped curve pixel %d,%d", dpi, x, y)
					}
				}
			}
		}
		for _, p := range [][2]int{{0, 0}, {size - 1, 0}, {0, size - 1}, {size - 1, size - 1}, {-1, radius}, {size, radius}} {
			if hit, _, _ := contains.Call(rgn, uintptr(p[0]), uintptr(p[1])); hit != 0 {
				t.Errorf("dpi %d: transparent point %v captures input", dpi, p)
			}
		}
		inputDelete.Call(rgn)
	}
}
