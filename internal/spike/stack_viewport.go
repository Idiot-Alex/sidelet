package spike

import "math"

// StackViewport crops only blank vertical space. Match frontend stackLayout:
// capacity and offset always use the entire WorkArea, never the cropped height.
// Ten pixels around the rows leave room for keyboard focus outlines.
func StackViewport(count int, areaHeight, offset, itemHeight float64, full bool) (top, height float64) {
	if full {
		return 0, areaHeight
	}
	layout := StackLayout(count, areaHeight, offset, itemHeight)
	body, position := layout.Height, layout.Top
	top = math.Max(0, math.Floor(position-10))
	bottom := math.Min(areaHeight, math.Ceil(position+body+10))
	return top, math.Max(1, bottom-top)
}
