//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/egoist/mygo"
)

// Opt-in diagnostics for the isolated Windows CI desktop. Moves the cursor
// over our synthetic windows and sends button messages ONLY to their HWNDs.
// No SendInput, foreign-window messages, user profile, or keyboard injection.
// This exercises Win32 routing and MyGo callbacks, not physical input or IME.
func runNativeChecks(a *overlayInput, v *views, probe *mygo.Window, output string, done func(bool)) {
	setCursor := inputUser32.NewProc("SetCursorPos")
	sendMessage := inputUser32.NewProc("SendMessageW")
	setFocus := inputUser32.NewProc("SetFocus")
	releaseCapture := inputUser32.NewProc("ReleaseCapture")
	var originalCursor winPoint
	inputGetCursor.Call(uintptr(unsafe.Pointer(&originalCursor)))
	m := a.m
	m.X, m.Y = m.WorkX+40, m.WorkY+80
	probe.SetBounds(mygo.Rectangle{X: m.WorkX, Y: m.WorkY, Width: min(900, m.WorkWidth), Height: min(700, m.WorkHeight)})
	v.notify("diagnostic-position")
	probe.Focus()
	type stage struct {
		name    string
		prepare func()
		check   func() error
		delay   time.Duration
	}
	assert := func(ok bool, reason string) error {
		if !ok {
			return fmt.Errorf("%s", reason)
		}
		return nil
	}
	passive := func() error {
		front, _, _ := quickGetForeground.Call()
		return assert(front == probe.NativeHandle(), "passive overlay changed the foreground window")
	}
	point := func(b *overlayWindow, x, y int) winPoint {
		p := winPoint{int32(winPX(x, b.windowDPI())), int32(winPX(y, b.windowDPI()))}
		inputClientScreen.Call(b.surface, uintptr(unsafe.Pointer(&p)))
		return p
	}
	move := func(b *overlayWindow, x, y int) {
		p := point(b, x, y)
		setCursor.Call(uintptr(p.X), uintptr(p.Y))
	}
	button := func(b *overlayWindow, msg uintptr, x, y int) {
		dpi := b.windowDPI()
		pos := uintptr(uint16(winPX(x, dpi))) | uintptr(uint16(winPX(y, dpi)))<<16
		wp := uintptr(0)
		if msg == 0x201 || msg == winMouseMove {
			wp = 1
		}
		sendMessage.Call(b.surface, msg, wp, pos)
	}
	marker := m.markerRect(0)
	handleX, handleY := marker.X+7, marker.Y+marker.Height/2
	var cardBounds mygo.Rectangle
	var dragStartY int
	stages := []stage{
		{"idle-region-and-focus", func() {}, func() error {
			if a.lastError != "" || a.hook == 0 || a.stackInput == nil {
				return fmt.Errorf("adapter unavailable: %s", a.lastError)
			}
			b := a.stackInput
			if inputHit(point(b, 200, 120)) != probe.NativeHandle() {
				return fmt.Errorf("transparent blank space did not reach owned underlay")
			}
			if inputHit(point(b, handleX, handleY)) != b.top {
				return fmt.Errorf("marker has no native hit region")
			}
			if inputHit(point(b, 13, marker.Y)) == b.top {
				return fmt.Errorf("rounded transparent corner intercepted input")
			}
			for _, dpi := range []int{96, 120, 144, 192} {
				// Region creation is checked at native DPI below; conversion must
				// also agree with MyGo's bounds at common scale factors.
				if winDIP(int32(winPX(40, dpi)), dpi) != 40 {
					return fmt.Errorf("DPI conversion mismatch at %d", dpi)
				}
			}
			return passive()
		}, 300 * time.Millisecond},
		{"background-hover", func() { move(a.stackInput, handleX, handleY) }, func() error {
			if m.Hover != 0 || inputHit(point(a.stackInput, 200, handleY)) != a.stackInput.top {
				return fmt.Errorf("hover did not expand the native region")
			}
			return passive()
		}, 700 * time.Millisecond},
		{"sdk-focus-veto", func() { setFocus.Call(a.stackInput.surface) }, func() error {
			if a.blockedFocus == 0 {
				return fmt.Errorf("explicit SDK-style SetFocus was not intercepted")
			}
			return passive()
		}, 100 * time.Millisecond},
		{"open-card-from-preview", func() {
			move(a.stackInput, 220, handleY)
			button(a.stackInput, 0x201, 220, handleY)
			button(a.stackInput, winLeftUp, 220, handleY)
		}, func() error {
			if !m.cardOpen() || a.cardInput == nil || !a.card.IsVisible() {
				return fmt.Errorf("preview click did not open card")
			}
			cardBounds = a.card.Bounds()
			return passive()
		}, 300 * time.Millisecond},
		{"source-to-card-handoff", func() { move(a.cardInput, 100, 80) }, func() error {
			if !m.cardOpen() || !v.cardInside || v.sourceInside {
				return fmt.Errorf("card closed or lost pointer presence after handoff")
			}
			if inputHit(point(a.cardInput, 0, 0)) == a.cardInput.top {
				return fmt.Errorf("card's rounded corner intercepted input")
			}
			return passive()
		}, 800 * time.Millisecond},
		{"editor-focus", func() { m.edit(); v.edit(); v.notify("edit") }, func() error {
			front, _, _ := quickGetForeground.Call()
			return assert(front == a.cardInput.top && a.editing && a.card.IsFocused(), "editor did not acquire native focus")
		}, 300 * time.Millisecond},
		{"cancel-focus-and-anchor", func() { m.cancel(); v.notify("cancel") }, func() error {
			if a.editing || a.card.Bounds() != cardBounds {
				return fmt.Errorf("cancel changed card bounds or left active editor")
			}
			return passive()
		}, 300 * time.Millisecond},
		{"repeat-edit-save", func() {
			m.edit()
			v.edit()
			m.Draft = "CI Windows 保存任务"
			m.save()
			v.notify("save")
		}, func() error {
			if m.Tasks[0].Title != "CI Windows 保存任务" || a.editing || a.card.Bounds() != cardBounds {
				return fmt.Errorf("repeat edit/save failed or moved card")
			}
			return passive()
		}, 300 * time.Millisecond},
		{"respect-user-focus-switch", func() { m.edit(); v.edit(); probe.Focus(); m.cancel(); v.notify("cancel") }, passive, 300 * time.Millisecond},
		{"close-before-drag", func() { v.closeCard(); move(a.stackInput, handleX, handleY) }, func() error {
			return assert(!a.card.IsVisible(), "closed card remained visible")
		}, 300 * time.Millisecond},
		{"drag-start", func() { dragStartY = m.Y; button(a.stackInput, 0x201, handleX, handleY) }, func() error {
			if !m.Dragging {
				return fmt.Errorf("marker pointer-down did not start drag")
			}
			return passive()
		}, 100 * time.Millisecond},
		{"drag-equal-local-coordinates", func() {
			button(a.stackInput, winMouseMove, handleX, handleY+20)
			button(a.stackInput, winMouseMove, handleX, handleY+20)
		}, func() error {
			return assert(m.Y == dragStartY+40 && m.Dragging, "equal local coordinates stalled native drag")
		}, 100 * time.Millisecond},
		{"drag-release", func() { button(a.stackInput, winLeftUp, handleX, handleY) }, func() error {
			if m.Dragging || m.Y != dragStartY+40 {
				return fmt.Errorf("release drifted or left capture active")
			}
			return passive()
		}, 100 * time.Millisecond},
		{"capture-loss-cancels-drag", func() {
			dragStartY = m.Y
			button(a.stackInput, 0x201, handleX, handleY)
			button(a.stackInput, winMouseMove, handleX, handleY+20)
			releaseCapture.Call()
		}, func() error {
			return assert(!m.Dragging && m.Y == dragStartY, "capture loss did not restore drag origin")
		}, 100 * time.Millisecond},
		{"drag-after-capture-loss", func() {
			button(a.stackInput, 0x201, handleX, handleY)
			button(a.stackInput, winMouseMove, handleX, handleY+20)
			button(a.stackInput, winLeftUp, handleX, handleY)
		}, func() error {
			return assert(!m.Dragging && m.Y == dragStartY+20, "capture loss left UI pointer state stuck")
		}, 100 * time.Millisecond},
		{"quiet-clears-hit-regions", func() { m.Quiet = true; v.notify("native-main-quiet") }, func() error {
			return assert(!a.stack.IsVisible() && inputHit(point(a.stackInput, handleX, handleY)) != a.stackInput.top, "quiet mode still intercepted input")
		}, 100 * time.Millisecond},
		{"cleanup", func() { a.close() }, func() error {
			return assert(a.hook == 0 && len(inputWindows) == 0, "Win32 callback registry or hook leaked")
		}, 100 * time.Millisecond},
	}
	results := []map[string]any{}
	var next func(int)
	finish := func(passed bool) {
		setCursor.Call(uintptr(originalCursor.X), uintptr(originalCursor.Y))
		data, err := json.MarshalIndent(map[string]any{"passed": passed, "scope": "owned Win32 regions, cursor hover, targeted mouse messages, focus and lifecycle", "hardwareInputVerified": false, "imeVerified": false, "stages": results}, "", "  ")
		if err == nil {
			err = os.WriteFile(filepath.Join(output, "native-check.json"), data, 0600)
		}
		if err != nil {
			passed = false
			log.Print(err)
		}
		done(passed)
		mygo.App.Quit()
	}
	next = func(i int) {
		if i == len(stages) {
			finish(true)
			return
		}
		s := stages[i]
		s.prepare()
		time.AfterFunc(s.delay, func() {
			mygo.RunOnMain(func() {
				err := s.check()
				r := map[string]any{"stage": s.name, "passed": err == nil, "input": a.state()}
				if err != nil {
					r["error"] = err.Error()
				}
				results = append(results, r)
				log.Printf("native-check %s error=%v", s.name, err)
				if err != nil {
					finish(false)
					return
				}
				next(i + 1)
			})
		})
	}
	next(0)
}
