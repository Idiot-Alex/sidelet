package spike

import "math"

// StackLayout matches frontend/src/lib/geometry.ts, including the reserved +N
// row on short screens and the eight directly displayed tasks on large ones.
type StackGeometry struct {
	Direct, Overflow, Capacity int
	Top, Height, RowHeight     float64
}

func StackLayout(count int, areaHeight, offset, itemHeight float64) StackGeometry {
	count = max(0, count)
	areaHeight, itemHeight = max(0, areaHeight), max(0, itemHeight)
	margin := min(10.0, areaHeight/2)
	capacity := int(max(0, math.Floor((areaHeight-2*margin+6)/(itemHeight+6))))
	direct := min(count, 8, capacity)
	if count > min(capacity, 8) {
		direct = min(count, 8, max(0, capacity-1))
	}
	rows := direct
	if count > direct {
		rows++
	}
	rowHeight := itemHeight
	if capacity == 0 {
		rowHeight = min(itemHeight, max(0, areaHeight-2*margin))
	}
	body := float64(rows)*rowHeight + float64(max(0, rows-1))*6
	top := max(margin, min(areaHeight*max(0, min(1, offset))-body/2, areaHeight-margin-body))
	return StackGeometry{direct, count - direct, capacity, top, body, rowHeight}
}
