package main

import (
	"math"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Windows clips both painting and mouse input to the native window region.
// Include a shown tooltip's real layout bounds, without retaining blank space
// after it disappears. Other platforms keep MyGo's standard tooltip unchanged.
func stackTooltip(c *ui.Context, m *model, anchor ui.Element, text string) {
	if runtime.GOOS != "windows" {
		anchor.Tooltip(text)
		return
	}
	anchor.Description(text)
	t := c.Theme()
	fill, ink := t.Inverse, t.InverseText
	if fill == ui.Transparent {
		fill = t.Text
	}
	if ink == ui.Transparent {
		ink = t.Background
	}
	tip := ui.TooltipBase(c, anchor, func(tip ui.Element) {
		tip.MaxWidth(t.Space(80)).Padding(t.Space(1.25), t.Space(2)).Radius(t.Space(1.25)).Background(fill).TextColor(ink).FontSize(t.FontSize - 1)
		tip.Children(func() { ui.Text(c, text) })
		tip.AttachTo(anchor, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, t.Space(1.5), 0)
	})
	if !tip.Valid() {
		return
	}
	r := tip.Bounds()
	if r.W <= 0 || r.H <= 0 {
		c.Invalidate()
		return
	}
	x, y := int(math.Floor(float64(r.X))), int(math.Floor(float64(r.Y)))
	m.stackTooltip = mygo.Rectangle{X: x, Y: y, Width: int(math.Ceil(float64(r.X+r.W))) - x, Height: int(math.Ceil(float64(r.Y+r.H))) - y}
}
