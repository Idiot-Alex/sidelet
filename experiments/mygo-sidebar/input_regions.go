package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func eventScreenPoint(m *model, index int, ev ui.InputEvent) mygo.Point {
	r := m.markerRect(index)
	return mygo.Point{X: m.X + r.X + int(math.Round(float64(ev.X))), Y: m.Y + r.Y + int(math.Round(float64(ev.Y)))}
}

func (m *model) expanded(index int) bool {
	return !m.Dragging && (m.Hover == index || m.Opened == index)
}
func (m *model) markerRect(index int) mygo.Rectangle {
	if index == overflowIndex {
		if m.desktopArrange() {
			return m.arrangeGripRect()
		}
		r := m.overflowRect()
		r.Width = 14
		if m.Side == "right" {
			r.X = stackWidth - r.Width
		}
		return r
	}
	width := 14
	if m.UITheme != "" && m.expanded(index) {
		width = 20
	}
	x := 0
	if m.Side == "right" {
		x = stackWidth - width
	}
	top, height := 8, m.itemHeight()
	if m.UITheme != "" {
		top = 10
		height = int(m.sidebarLayout().RowHeight)
		if m.desktopArrange() {
			top = 52
		}
	}
	return mygo.Rectangle{X: x, Y: top + m.rowFor(index)*(height+6), Width: width, Height: height}
}
func (m *model) overflowRect() mygo.Rectangle {
	direct, tail := m.sidebarItems()
	w := max(50, 38+len(fmt.Sprint(len(tail)))*6)
	x := 0
	if m.Side == "right" {
		x = stackWidth - w
	}
	top := 10
	if m.desktopArrange() {
		top = 52
		w = 296
		if m.Side == "right" {
			x = stackWidth - w
		}
	}
	h := int(m.sidebarLayout().RowHeight)
	return mygo.Rectangle{X: x, Y: top + len(direct)*(h+6), Width: w, Height: h}
}
func (m *model) arrangeRowRect(index int) mygo.Rectangle {
	r := m.markerRect(index)
	r.X, r.Width = 0, 296
	if m.Side == "right" {
		r.X = stackWidth - r.Width
	}
	return r
}
func (m *model) arrangeGripRect() mygo.Rectangle {
	x := 0
	if m.Side == "right" {
		x = stackWidth - 296
	}
	return mygo.Rectangle{X: x, Y: 10, Width: 296, Height: 36}
}
func (m *model) arrangeToolbarRect() mygo.Rectangle {
	r := m.arrangeGripRect()
	r.Y, r.Height = m.stackHeight()-80, 70
	return r
}
func (m *model) previewRect(index int) mygo.Rectangle {
	marker := m.markerRect(index)
	width := stackWidth
	if m.UITheme != "" {
		width = 296
	}
	x := marker.Width
	if m.Side == "right" {
		x = stackWidth - width
	}
	return mygo.Rectangle{X: x, Y: marker.Y, Width: width - marker.Width, Height: m.itemHeight()}
}

// AppKit locations use the current window's bottom-left origin. Handle every
// delivered drag, even when moving the window keeps its local position equal.
func nativeDragScreenPoint(m *model, x, y float64) mygo.Point {
	return mygo.Point{X: m.X + int(math.Round(x)), Y: m.Y + m.stackHeight() - int(math.Round(y))}
}

// Regions share the drawing geometry; the transparent remainder owns no input.
// Keep a stable key so a panel capturing a drag is never replaced mid-gesture.
type inputRegion struct {
	key    string
	marker bool
	task   int
	rect   mygo.Rectangle
}

func stackInputRegions(m *model) []inputRegion {
	if m.Quiet {
		return nil
	}
	regions := make([]inputRegion, 0, len(m.Tasks)+1)
	if m.desktopArrange() {
		regions = append(regions, inputRegion{key: "arrange-grip", marker: true, task: overflowIndex, rect: m.arrangeGripRect()})
		regions = append(regions, inputRegion{key: "arrange-toolbar", task: overflowIndex, rect: m.arrangeToolbarRect()})
	}
	for i := range m.Tasks {
		if !m.sidebarVisible(i) {
			continue
		}
		if m.desktopArrange() {
			regions = append(regions, inputRegion{key: fmt.Sprintf("order-%d", i), task: i, rect: m.arrangeRowRect(i)})
			continue
		}
		regions = append(regions, inputRegion{fmt.Sprintf("marker-%d", i), true, i,
			m.markerRect(i)})
		if m.expanded(i) {
			regions = append(regions, inputRegion{fmt.Sprintf("preview-%d", i), false, i,
				m.previewRect(i)})
		}
	}
	if m.UITheme != "" {
		_, tail := m.sidebarItems()
		if len(tail) > 0 {
			regions = append(regions, inputRegion{key: "overflow", task: overflowIndex, rect: m.overflowRect()})
		}
	}
	if m.undoVisible() && !m.desktopArrange() {
		regions = append(regions, inputRegion{key: "undo", task: m.UndoID - 1, rect: m.undoRect()})
	}
	valid := regions[:0]
	for _, r := range regions {
		if r.rect.Height > 0 && r.rect.Y >= 0 && r.rect.Y+r.rect.Height <= m.stackHeight() {
			valid = append(valid, r)
		}
	}
	return valid
}
