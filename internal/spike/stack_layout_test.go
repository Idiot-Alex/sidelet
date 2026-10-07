package spike

import "testing"

func TestStackLayoutMatchesFormalOverflowBoundaries(t *testing.T) {
	for _, tc := range []struct {
		count            int
		area             float64
		direct, overflow int
		height, row      float64
	}{
		{8, 1000, 8, 0, 394, 44}, {9, 1000, 8, 1, 444, 44},
		{8, 180, 2, 6, 144, 44}, {8, 100, 0, 8, 44, 44},
		{8, 40, 0, 8, 20, 20}, {8, 8, 0, 8, 0, 0}, {0, 1000, 0, 0, 0, 44},
	} {
		g := StackLayout(tc.count, tc.area, .35, 44)
		if g.Direct != tc.direct || g.Overflow != tc.overflow || g.Height != tc.height || g.RowHeight != tc.row || g.Top < 0 || g.Top+g.Height > tc.area {
			t.Fatalf("%+v: %+v", tc, g)
		}
	}
}
