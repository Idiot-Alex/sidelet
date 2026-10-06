//go:build windows || darwin

package main

import (
	"math"
	"runtime"
	"time"

	"sidelet/internal/platform"
	"sidelet/internal/spike"
)

func (c *controller) stackVisibleCount(o *overlay, now int64) int {
	count := 0
	for _, task := range c.snapshot.Todos {
		if (task.Completed && now >= task.CompletedAt+800) || task.SnoozedUntil > now {
			continue
		}
		if c.store != nil {
			if task.DisplayMode != "EDGE" || task.StackID != o.stackID {
				continue
			}
		} else if len(c.stacks) > 1 && int((task.ID-1)%int64(len(c.stacks))) != o.index {
			continue
		}
		count++
	}
	return count
}

func (c *controller) placeStack(o *overlay, display platform.Display) error {
	a, scale := display.WorkArea, display.Scale
	w := math.Min(312*scale, a.Width)
	x := a.X
	if o.side == "right" {
		x += a.Width - w
	}
	top, height := 0.0, a.Height/scale
	if runtime.GOOS == "darwin" {
		now := time.Now().UnixMilli()
		top, height = spike.StackViewport(c.stackVisibleCount(o, now), a.Height/scale, o.offset, float64(o.itemHeight), c.arranging || now < c.snapshot.UndoUntil)
	}
	frame := platform.Rect{X: x, Y: a.Y + top*scale, Width: w, Height: height * scale}
	if frame != o.stackFrame || display != o.stackDisplay || o.viewportTop != top {
		o.layoutRevision++
		if err := o.native.Move(frame); err != nil {
			return err
		}
		o.stackFrame, o.stackDisplay, o.viewportTop = frame, display, top
	}
	o.window.EmitEvent("spike:config", c.stackConfig(o))
	o.window.EmitEvent("spike:measure")
	return nil
}
