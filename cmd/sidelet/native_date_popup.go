package main

import (
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type nativeDatePopup struct {
	window, owner *mygo.Window
}

// Prefer a free side of the owner so its close and save controls stay visible.
func datePopupBounds(owner, anchor, area mygo.Rectangle) mygo.Rectangle {
	w, h := min(320, area.Width), min(500, area.Height)
	x := owner.X + owner.Width + 8
	if x+w > area.X+area.Width {
		x = owner.X - w - 8
	}
	y := anchor.Y
	return mygo.Rectangle{X: max(area.X, min(x, area.X+area.Width-w)), Y: max(area.Y, min(y, area.Y+area.Height-h)), Width: w, Height: h}
}

func (v *nativeTasksView) closeDatePopup(restore bool) {
	p := v.datePopup
	if p == nil {
		return
	}
	v.datePopup = nil
	v.datePickerOpen, v.pickerCalendarOpen = false, false
	p.window.Close()
	if restore && p.owner.IsVisible() {
		p.owner.Focus()
	}
	p.owner.Invalidate()
	if !restore && v.onDatePopupDismiss != nil {
		v.onDatePopupDismiss()
	}
}

func (v *nativeTasksView) openDatePopup(anchor ui.Element) {
	owner := mygo.FocusedWindow()
	// Headless render tests exercise the same panel without a native host.
	if owner == nil || v.datePopup != nil {
		return
	}
	r := anchor.Bounds()
	x, y := anchor.LocalToWindow(0, 0)
	b := owner.ContentBounds()
	a := mygo.Rectangle{X: b.X + int(x), Y: b.Y + int(y), Width: int(r.W), Height: int(r.H)}
	area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: a.X, Y: a.Y}).WorkArea
	bounds := datePopupBounds(owner.Bounds(), a, area)
	p := &nativeDatePopup{owner: owner}
	v.datePopup = p
	v.pickerCalendarOpen = true
	w := newAuxiliaryWindow(mygo.WindowOptions{Title: "Sidelet · 预计完成时间", Parent: owner,
		X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: bounds.Height,
		Frameless: true, DisableResize: true, AlwaysOnTop: true, SkipTaskbar: true, Hidden: true,
		Content: ui.View(func(c *ui.Context) {
			t := webTheme(v.service.m.UITheme)
			v.visual = t
			c.SetTheme(t.Theme)
			c.Root().Background(t.Surface)
			enterSurface(c, ui.Scroll(c).Fill(), 0).Children(func() { v.renderDatePanel(c) })
			if c.Root().Shortcut(0, ui.KeyEscape) {
				v.datePickerOpen = false
			}
			if !v.datePickerOpen {
				v.closeDatePopup(true)
			}
		}),
	})
	p.window = w
	w.SetTitle("Sidelet · 预计完成时间")
	w.OnBlur(func() {
		if v.datePopup == p {
			// Clicking the owner's Save/Add must use the date just chosen.
			// Keep invalid input visible so it cannot silently save an old date.
			if err := v.commitOpenDatePicker(); err != nil {
				v.formDue = fmt.Sprintf("%04d/%02d/%02d %s:%s", v.pickerDate.Year(), v.pickerDate.Month(), v.pickerDate.Day(), v.pickerHour, v.pickerMinute)
			}
			v.closeDatePopup(false)
		}
	})
	w.OnClosed(func() {
		if v.datePopup == p {
			v.datePopup = nil
			v.datePickerOpen, v.pickerCalendarOpen = false, false
			owner.Invalidate()
		}
	})
	go mygo.RunOnMain(func() {
		if v.datePopup == p {
			w.Show()
			w.Focus()
		}
	})
}
