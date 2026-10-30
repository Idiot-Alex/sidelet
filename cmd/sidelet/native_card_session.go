package main

import (
	"time"

	"github.com/egoist/mygo"
)

func (m *model) cardEditorHeight() int {
	height := 430
	if d := m.cardDetails; d != nil && d.datePopup == nil {
		if d.datePickerOpen {
			height += 250
		}
		if d.pickerCalendarOpen {
			height += 250
		}
	}
	if m.WorkHeight > 20 {
		height = min(height, m.WorkHeight-20)
	}
	return height
}

func (v *views) beginSession(index int) {
	v.cardCopiedUntil = time.Time{}
	v.session.Begin("source", "card", int64(index+1))
	v.sourceInside, v.cardInside = true, false
	if v.closeTimer != nil {
		v.closeTimer.Stop()
	}
}
func (v *views) presence(source, card bool) {
	if !v.session.Open || !v.m.cardOpen() {
		return
	}
	if source != v.sourceInside {
		v.session.Presence("source", source, v.m.now())
		v.sourceInside = source
	}
	if card != v.cardInside {
		v.session.Presence("card", card, v.m.now())
		v.cardInside = card
	}
	v.armClose()
}
func (v *views) armClose() {
	if v.scheduleClose == nil {
		return
	}
	if v.closeTimer != nil {
		v.closeTimer.Stop()
		v.closeTimer = nil
	}
	if !v.session.Open || !v.m.cardOpen() || v.m.Editing || v.m.Dragging || v.session.CloseAt.IsZero() {
		return
	}
	now := v.m.now()
	if !now.Before(v.session.CloseAt) {
		v.session.Presence("card", false, now)
	}
	revision := v.session.Revision
	v.scheduleClose(v.session.CloseAt.Sub(now), func() {
		if !v.m.Editing && !v.m.Dragging && v.session.Expired(revision, v.m.now()) {
			v.closeCard()
		}
	})
}
func (v *views) enableCloseTimer() {
	v.scheduleClose = func(d time.Duration, f func()) { v.closeTimer = time.AfterFunc(d, func() { mygo.RunOnMain(f) }) }
	onQuitCleanup(func() {
		v.scheduleClose = nil
		if v.closeTimer != nil {
			v.closeTimer.Stop()
		}
	})
}

// Same quickRect anchor, 12px separation and 10px work-area margin as the
// formal UI. The un-clamped Y is retained while editor/card heights change.
func (m *model) cardAnchor(index int) (x, y int) {
	r := m.markerRect(index)
	width := 296
	x = m.X + width + 12
	if m.Side == "right" {
		x = m.X + stackWidth - width - cardWidth - 12
	}
	if index == overflowIndex {
		r = m.overflowRect()
		x = m.X + r.X + r.Width + 12
		if m.Side == "right" {
			x = m.X + r.X - cardWidth - 12
		}
	}
	y = m.Y + r.Y - 12
	x = max(m.WorkX+10, min(m.WorkX+m.WorkWidth-cardWidth-10, x))
	return
}
