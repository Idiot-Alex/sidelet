package main

import (
	"math"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type gazePoint struct{ x, y float64 }

const (
	gazeMaxX  = 4.6
	gazeMaxY  = 3.4
	gazeFrame = time.Second / 30
	gazeIdle  = 100 * time.Millisecond
)

// All coordinates passed here use the same units (DIPs on Mac, native pixels
// on Windows). Normalizing by the avatar size also handles mixed-DPI displays.
func avatarGazeTarget(px, py, x, y, width, height float64) gazePoint {
	size := math.Min(width, height)
	if size <= 0 {
		return gazePoint{}
	}
	x += (width-size)/2 + size*62.5/128
	y += (height-size)/2 + size*59.5/128
	dx, dy := (px-x)/size, (py-y)/size
	// Respond on each axis independently: a cursor far to the left must not
	// suppress looking up/down. Soft saturation retains distance changes much
	// farther away than direction-only normalization. Half travel is 6 widths.
	hx, hy := dx/(math.Abs(dx)+6), dy/(math.Abs(dy)+6)
	// Keep diagonal motion within the same eye-shaped travel envelope.
	limit := math.Max(1, math.Hypot(hx, hy))
	return gazePoint{gazeMaxX * hx / limit, gazeMaxY * hy / limit}
}

// One pending timer, dispatched on the UI thread. At rest we only sample the
// cursor at 10 Hz; no invalidation, SVG parsing or image allocation is needed.
type avatarGaze struct {
	offset   gazePoint
	running  bool
	epoch    uint64
	cancel   func()
	last     time.Time
	now      func() time.Time
	schedule func(time.Duration, func()) func()
	sample   func() (gazePoint, bool)
	repaint  func()
}

func (g *avatarGaze) start() {
	if g.running {
		return
	}
	g.running = true
	g.epoch++
	g.last = g.now()
	g.queue(gazeFrame)
}

func (g *avatarGaze) stop() {
	g.running = false
	g.epoch++ // Invalidate a callback already queued on the main thread.
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
}

func (g *avatarGaze) queue(delay time.Duration) {
	epoch := g.epoch
	g.cancel = g.schedule(delay, func() {
		if !g.running || g.epoch != epoch {
			return
		}
		g.cancel = nil
		target, visible := g.sample()
		if !visible {
			g.stop()
			return
		}
		now := g.now()
		dt := min(max(now.Sub(g.last).Seconds(), 0), .1)
		g.last = now
		before := g.offset
		alpha := 1 - math.Exp(-dt/.07)
		g.offset.x += (target.x - g.offset.x) * alpha
		g.offset.y += (target.y - g.offset.y) * alpha
		settled := math.Hypot(target.x-g.offset.x, target.y-g.offset.y) < .02
		if settled {
			g.offset = target
		}
		if math.Hypot(g.offset.x-before.x, g.offset.y-before.y) >= .001 {
			g.repaint()
		}
		if settled {
			g.queue(gazeIdle)
		} else {
			g.queue(gazeFrame)
		}
	})
}

func (a *nativeAvatar) startGaze() {
	if a.window == nil || !a.window.IsVisible() || a.window.IsMinimized() || a.service.m.Preferences.Appearance.HideSidebarAvatar || a.service.m.desktopArrange() {
		return
	}
	if a.gaze.schedule == nil {
		a.gaze.now = time.Now
		a.gaze.schedule = func(delay time.Duration, f func()) func() {
			timer := time.AfterFunc(delay, func() { mygo.RunOnMain(f) })
			return func() { timer.Stop() }
		}
		a.gaze.sample = func() (gazePoint, bool) {
			w := a.window
			if w == nil || !w.IsVisible() || w.IsMinimized() {
				return gazePoint{}, false
			}
			return avatarCursorGaze(w), true
		}
		a.gaze.repaint = func() { a.window.Invalidate() }
	}
	a.gaze.start()
}

type avatarGazeArtwork struct{ body, eyes, cover *ui.SVG }

// Derive layers from the canonical artwork once. The cover masks the moving
// irises at the exact eye-white paths, retaining the lashes and original face.
var avatarGazeArt = sync.OnceValue(func() avatarGazeArtwork {
	source := string(sidebarAvatarSVG)
	before, rest, ok := strings.Cut(source, "<!-- gaze-eyes:start -->")
	if !ok {
		panic("avatar is missing the gaze eye layer")
	}
	eyes, after, ok := strings.Cut(rest, "<!-- gaze-eyes:end -->")
	if !ok {
		panic("avatar is missing the gaze eye layer end")
	}
	_, white, ok := strings.Cut(before, `<path id="gaze-eye-whites"`)
	if !ok {
		panic("avatar is missing the eye-white path")
	}
	white, _, ok = strings.Cut(white, "/>")
	if !ok {
		panic("avatar has an invalid eye-white path")
	}
	mask := `<defs><mask id="gaze-mask" maskUnits="userSpaceOnUse" x="0" y="0" width="128" height="128"><rect width="128" height="128" fill="white"/><path` + strings.Replace(white, `fill="#fffdf4"`, `fill="black"`, 1) + `/></mask></defs>`
	body := before + after
	cover := strings.Replace(body, `<g stroke-linecap=`, mask+`<g mask="url(#gaze-mask)" stroke-linecap=`, 1)
	return avatarGazeArtwork{
		body:  ui.MustParseSVG([]byte(body)),
		eyes:  ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128">` + eyes + `</svg>`)),
		cover: ui.MustParseSVG([]byte(cover)),
	}
})

func drawAvatarGaze(p *ui.Painter, r ui.Rect, offset gazePoint) {
	art := avatarGazeArt()
	size := min(r.W, r.H)
	r = ui.Rect{X: r.X + (r.W-size)/2, Y: r.Y + (r.H-size)/2, W: size, H: size}
	p.Image(art.body, r, ui.Contain)
	scale := size / 128
	// Keep the moving layer inside the opaque face; the cover then clips it to
	// the individual eye shapes. The rest of the desktop stays transparent.
	p.Clip(ui.Rect{X: r.X + 32*scale, Y: r.Y + 45*scale, W: 60 * scale, H: 29 * scale}, 0, func() {
		p.Transform(ui.Translation(float32(offset.x)*scale, float32(offset.y)*scale), 0, 0, func() {
			p.Image(art.eyes, r, ui.Contain)
		})
		p.Image(art.cover, r, ui.Contain)
	})
}
