package spike

import (
	"math"
	"testing"
)

func TestQuickCardBoundsAndOldRendererFallback(t *testing.T) {
	for _, height := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if QuickCardHeight(height) != 390 {
			t.Fatal("invalid or missing renderer height must keep the safe viewport")
		}
	}
	if QuickCardHeight(180) != 180 || QuickCardHeight(20) != 160 || QuickCardHeight(4000) != 390 {
		t.Fatal("card content must fit within the supported viewport range")
	}
}
