//go:build windows

package main

// GDI regions have binary coverage. Leave two physical pixels around the
// painted curve for the renderer's alpha coverage, while retaining the exact
// rectangular bounds and transparent corner hit-through. This margin must not
// scale with DPI: it protects raster samples, not layout spacing.
func smoothWindowRegion(left, top, right, bottom, radius int) (uintptr, error) {
	const fringe = 2
	radius = max(0, min(radius, min(right-left, bottom-top)/2))
	box, _, err := inputCreateRect.Call(uintptr(left), uintptr(top), uintptr(right), uintptr(bottom))
	if box == 0 || radius == 0 {
		return box, err
	}
	defer inputDelete.Call(box)
	round, _, err := inputCreateRound.Call(uintptr(left-fringe), uintptr(top-fringe),
		uintptr(right+fringe+1), uintptr(bottom+fringe+1), uintptr(2*(radius+fringe)), uintptr(2*(radius+fringe)))
	if round == 0 {
		return 0, err
	}
	if ok, _, err := inputCombine.Call(round, round, box, 1 /* RGN_AND */); ok == 0 {
		inputDelete.Call(round)
		return 0, err
	}
	return round, nil
}
