package spike

import "math"

// StackViewport crops only blank vertical space. Match frontend stackLayout:
// capacity and offset always use the entire WorkArea, never the cropped height.
// Ten pixels around the rows leave room for keyboard focus outlines.
func StackViewport(count int, areaHeight, offset, itemHeight float64, full bool) (top, height float64) {
	if full {
		return 0, areaHeight
	}
	margin := math.Min(10, areaHeight/2)
	capacity := int(math.Max(0, math.Floor((areaHeight-2*margin+6)/(itemHeight+6))))
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
		rowHeight = math.Min(itemHeight, math.Max(0, areaHeight-2*margin))
	}
	body := float64(rows)*rowHeight + float64(max(0, rows-1))*6
	position := math.Max(margin, math.Min(areaHeight*math.Max(0, math.Min(1, offset))-body/2, areaHeight-margin-body))
	top = math.Max(0, math.Floor(position-10))
	bottom := math.Min(areaHeight, math.Ceil(position+body+10))
	return top, math.Max(1, bottom-top)
}
