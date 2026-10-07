//go:build darwin

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// This diagnostic changes only the lab's synthetic model and owned windows.
// It queries AppKit's predicted mouse-down destination and real key window;
// it does not create/post OS input, and is not a hardware hover or IME test.
func runNativeChecks(a *overlayInput, v *views, probe *mygo.Window, output string, done func(bool)) {
	type stage struct {
		name    string
		prepare func()
		check   func() error
	}
	m := a.m
	setStack := func() {
		a.stack.SetBounds(mygo.Rectangle{X: m.X, Y: m.Y, Width: stackWidth, Height: stackHeight})
		a.stack.Invalidate()
		a.sync()
	}
	hit := func(p nsPoint) int {
		return panelAPI.hitWindow(objc.ID(objc.GetClass("NSWindow")), objc.RegisterName("windowNumberAtPoint:belowWindowWithWindowNumber:"), p, 0)
	}
	checkPanels := func(count int, checkHits bool) error {
		visible := 0
		for key, p := range a.panels {
			if !p.visible {
				continue
			}
			visible++
			if send(p.panel, "canBecomeKeyWindow") != 0 || send(p.panel, "canBecomeMainWindow") != 0 || uint(send(p.panel, "styleMask"))&(1<<7) == 0 {
				return fmt.Errorf("%s is not a nonactivating non-key panel", key)
			}
			if uint(send(p.tracking, "options"))&0x80 == 0 {
				return fmt.Errorf("%s tracking is not active in background", key)
			}
			if checkHits {
				center := nsPoint{p.frame.Origin.X + p.frame.Size.W/2, p.frame.Origin.Y + p.frame.Size.H/2}
				if got, want := hit(center), int(send(p.panel, "windowNumber")); got != want {
					return fmt.Errorf("%s center hits %d, want %d", key, got, want)
				}
				corner := nsPoint{p.frame.Origin.X + 0.25, p.frame.Origin.Y + 0.25}
				if hit(corner) == int(send(p.panel, "windowNumber")) {
					return fmt.Errorf("%s rounded transparent corner intercepts input", key)
				}
			}
		}
		if visible != count {
			return fmt.Errorf("visible panels %d, want %d", visible, count)
		}
		return nil
	}
	checkBlank := func() error {
		parent := panelAPI.frame(objc.ID(a.stack.NativeHandle()), objc.RegisterName("frame"))
		p := nsPoint{parent.Origin.X + 200, parent.Origin.Y + parent.Size.H - 115}
		if got, want := hit(p), int(send(objc.ID(probe.NativeHandle()), "windowNumber")); got != want {
			return fmt.Errorf("blank hits %d, want owned underlay %d", got, want)
		}
		return nil
	}
	checkPassive := func() error {
		if a.stack.IsFocused() || (a.card != nil && a.card.IsFocused()) || !probe.IsFocused() {
			return fmt.Errorf("passive overlay changed owned key window")
		}
		return nil
	}
	originalX, originalY := m.X, m.Y
	var cardBounds mygo.Rectangle
	stages := []stage{
		{"idle", func() { probe.Focus() }, func() error {
			if send(objc.ID(a.stack.NativeHandle()), "ignoresMouseEvents") == 0 {
				return fmt.Errorf("render window intercepts mouse")
			}
			if err := checkPanels(3, true); err != nil {
				return err
			}
			if err := checkBlank(); err != nil {
				return err
			}
			return checkPassive()
		}},
		{"preview", func() { m.open(0); setStack() }, func() error {
			if err := checkPanels(4, true); err != nil {
				return err
			}
			if err := checkBlank(); err != nil {
				return err
			}
			return checkPassive()
		}},
		{"passive-card", func() { v.open(0); cardBounds = a.card.Bounds() }, func() error {
			if err := checkPanels(5, false); err != nil {
				return err
			}
			p := a.panels["card"]
			if hit(nsPoint{p.frame.Origin.X + 160, p.frame.Origin.Y + 100}) != int(send(p.panel, "windowNumber")) {
				return fmt.Errorf("card has no native input region")
			}
			return checkPassive()
		}},
		{"edit", func() { m.edit(); a.card.Invalidate(); v.edit() }, func() error {
			if !a.card.IsFocused() || probe.IsFocused() {
				return fmt.Errorf("editor has no key window")
			}
			if send(objc.ID(a.card.NativeHandle()), "ignoresMouseEvents") != 0 {
				return fmt.Errorf("editor ignores input")
			}
			if err := checkPanels(4, false); err != nil {
				return err
			}
			return nil
		}},
		{"cancel-restore", func() { m.cancel(); a.endEdit(); a.sync(); a.card.Invalidate() }, func() error {
			if a.card.Bounds() != cardBounds {
				return fmt.Errorf("cancel moved the card")
			}
			if err := checkPanels(5, false); err != nil {
				return err
			}
			return checkPassive()
		}},
		{"repeat-raised-editor", func() {
			a.card.Focus()
			m.edit()
			v.edit()
			m.cancel()
			a.endEdit()
			a.sync()
			m.edit()
			v.edit()
			a.card.Invalidate()
		}, func() error {
			if !a.card.IsFocused() {
				return fmt.Errorf("second edit did not reacquire native key window")
			}
			if nativeState()["firstResponderTakesText"] != true {
				return fmt.Errorf("second edit has no text responder")
			}
			return nil
		}},
		{"move", func() {
			m.close()
			v.close()
			a.endEdit()
			m.startDrag(7, originalY+30)
			m.moveDrag(107, originalY+130)
			setStack()
		}, func() error {
			if m.X != originalX+100 || m.Y != originalY+100 {
				return fmt.Errorf("drag origin drift")
			}
			return checkPanels(3, true)
		}},
		{"cancel-drag", func() { m.cancelDrag(); setStack() }, func() error {
			if m.X != originalX || m.Y != originalY {
				return fmt.Errorf("cancel did not restore original position")
			}
			return checkPanels(3, true)
		}},
		{"complete", func() { m.open(0); m.complete(); v.close(); setStack() }, func() error {
			if a.card.IsVisible() || a.panels["marker-0"].visible {
				return fmt.Errorf("completed task still visible")
			}
			return checkPanels(2, true)
		}},
		{"cleanup", func() { a.close() }, func() error {
			if len(a.panels) != 0 || len(inputViews) != 0 {
				return fmt.Errorf("native registry leaked")
			}
			return checkBlank()
		}},
	}
	results := []map[string]any{}
	passed := true
	var next func(int)
	next = func(i int) {
		if i == len(stages) {
			b, err := json.MarshalIndent(map[string]any{"passed": passed, "scope": "owned AppKit geometry, focus and lifecycle; no OS input injection", "hardwareHoverVerified": false, "stages": results}, "", "  ")
			if err == nil {
				err = os.WriteFile(filepath.Join(output, "native-check.json"), b, 0644)
			}
			if err != nil {
				passed = false
				log.Print(err)
			}
			log.Printf("native-check passed=%t stages=%d", passed, len(results))
			done(passed)
			mygo.App.Quit()
			return
		}
		s := stages[i]
		s.prepare()
		// A single timer per transition lets AppKit/Core Animation commit the
		// new windows before querying WindowServer. There is no idle polling.
		time.AfterFunc(250*time.Millisecond, func() {
			mygo.RunOnMain(func() {
				err := s.check()
				r := map[string]any{"stage": s.name, "passed": err == nil, "native": nativeState(), "input": a.state()}
				if err != nil {
					passed = false
					r["error"] = err.Error()
				}
				results = append(results, r)
				log.Printf("native-check %s passed=%t error=%v", s.name, err == nil, err)
				next(i + 1)
			})
		})
	}
	next(0)
}
