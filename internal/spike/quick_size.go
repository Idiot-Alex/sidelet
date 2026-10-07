package spike

import "math"

const QuickCardMaxHeight = 390

func QuickCardHeight(height float64) float64 {
	if math.IsNaN(height) || math.IsInf(height, 0) || height <= 0 {
		return QuickCardMaxHeight
	}
	return math.Max(160, math.Min(QuickCardMaxHeight, math.Ceil(height)))
}
