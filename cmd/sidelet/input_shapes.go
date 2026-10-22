package main

import "github.com/egoist/mygo"

// A native region follows the painted silhouette, including the square edge
// attached to the display. Expanded themed labels are one continuous surface.
type overlayShape struct {
	rect       mygo.Rectangle
	radius     int
	squareEdge string
	task       int
}

func stackInputShapes(m *model) []overlayShape {
	if m.Quiet {
		return nil
	}
	var shapes []overlayShape
	for _, r := range stackInputRegions(m) {
		s := overlayShape{rect: r.rect, radius: 5, task: r.task}
		if m.UITheme != "" {
			switch {
			case r.key == "overflow":
				s.squareEdge = m.Side
			case r.key == "undo":
				s.radius = int(webTheme(m.UITheme).Theme.Radius)
			case !m.desktopArrange() && r.marker && m.expanded(r.task):
				s.rect.Width = 296
				if m.Side == "right" {
					s.rect.X = stackWidth - s.rect.Width
				}
			case !m.desktopArrange() && !r.marker:
				continue // The expanded marker already includes its preview.
			case !m.desktopArrange() && r.marker:
				s.squareEdge = m.Side
			}
		} else if !r.marker {
			s.radius = 8
		}
		shapes = append(shapes, s)
	}
	if m.stackTooltip.Width > 0 && m.stackTooltip.Height > 0 {
		shapes = append(shapes, overlayShape{rect: m.stackTooltip, radius: 5, task: -3})
	}
	return shapes
}
