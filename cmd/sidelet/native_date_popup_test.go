package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDatePopupPlacementAndStableOwnerSize(t *testing.T) {
	for _, area := range []mygo.Rectangle{{Width: 1920, Height: 1080}, {X: -1280, Y: 30, Width: 1280, Height: 720}, {Width: 300, Height: 400}} {
		for _, x := range []int{area.X, area.X + area.Width - 320} {
			owner := mygo.Rectangle{X: x, Y: area.Y + 40, Width: 320, Height: 430}
			got := datePopupBounds(owner, owner, area)
			if got.X < area.X || got.Y < area.Y || got.X+got.Width > area.X+area.Width || got.Y+got.Height > area.Y+area.Height {
				t.Fatalf("popup outside work area: %+v %+v", got, area)
			}
		}
	}
	m := newModel()
	m.open(0)
	m.edit()
	d := m.cardDetails
	q := quickFixture(testService(m))
	q.details = d
	card, quick := m.cardEditorHeight(), q.contentHeight()
	d.datePopup = &nativeDatePopup{}
	d.datePickerOpen, d.pickerCalendarOpen = true, true
	if m.cardEditorHeight() != card || q.contentHeight() != quick {
		t.Fatal("opening a native date popup resized its owner")
	}
}

func TestStandaloneCalendarKeepsSingleSurfaceAndStableGrid(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	m.clock = func() time.Time { return time.Date(2026, 12, 31, 10, 0, 0, 0, time.Local) }
	v := newNativeTasksView(testService(m))
	v.datePopup = &nativeDatePopup{}
	v.datePickerOpen, v.pickerCalendarOpen = true, true
	v.pickerDate, v.pickerHour, v.pickerMinute = m.now(), "18", "00"
	u := ui.NewTester(func(c *ui.Context) {
		v.visual = webTheme("mac")
		c.SetTheme(v.visual.Theme)
		c.Root().Background(v.visual.Surface)
		ui.Scroll(c).Fill().Children(func() { v.renderDatePanel(c) })
	}, 320, 500)
	confirm := visualRect(t, u, "确定截止时间")
	if err := u.Click("明天"); err != nil {
		t.Fatal(err)
	}
	if !u.HasText("2027年1月") || !v.pickerCalendarOpen || visualRect(t, u, "确定截止时间") != confirm {
		t.Fatal("shortcut collapsed calendar or changed button position")
	}
	if err := u.Click("2027年1月2日"); err != nil {
		t.Fatal(err)
	}
	if !v.pickerCalendarOpen || visualRect(t, u, "确定截止时间") != confirm {
		t.Fatal("day selection collapsed the calendar")
	}
	if dir := os.Getenv("SIDELET_DATE_POPUP_VISUAL_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, "calendar-single-surface.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, u.Image()); err != nil {
			t.Fatal(err)
		}
	}
}
