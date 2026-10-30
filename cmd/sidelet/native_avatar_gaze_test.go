package main

import (
	"image"
	"image/png"
	"math"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestAvatarGazeDirectionDistanceAndScreenScale(t *testing.T) {
	for _, scale := range []float64{1, 1.25, 1.5, 2, 3} {
		// A monitor left of/above the primary has negative screen coordinates.
		x, y, size := -1000*scale, -400*scale, 56*scale
		cx, cy := x+size*62.5/128, y+size*59.5/128
		if got := avatarGazeTarget(cx, cy, x, y, size, size); math.Hypot(got.x, got.y) > 1e-12 {
			t.Fatal("looking at the face should center the eyes", got)
		}
		for _, d := range []gazePoint{{-10000, 0}, {10000, 0}, {0, -10000}, {0, 10000}, {10000, -10000}} {
			got := avatarGazeTarget(cx+d.x*scale, cy+d.y*scale, x, y, size, size)
			want := avatarGazeTarget(56*62.5/128+d.x, 56*59.5/128+d.y, 0, 0, 56, 56)
			if math.Hypot(got.x-want.x, got.y-want.y) > 1e-10 {
				t.Fatalf("DPI or monitor origin changed direction: %v != %v", got, want)
			}
			if got.x*d.x < 0 || got.y*d.y < 0 || got.x*got.x/(gazeMaxX*gazeMaxX)+got.y*got.y/(gazeMaxY*gazeMaxY) > 1.00001 {
				t.Fatal("gaze points the wrong way or escaped the eye", got)
			}
		}
	}
	near := avatarGazeTarget(38, 26, 0, 0, 56, 56)
	far := avatarGazeTarget(1000, 26, 0, 0, 56, 56)
	if near.x <= 0 || far.x <= near.x {
		t.Fatal("near cursor should produce less eye movement")
	}
}

func TestAvatarGazeRemainsExpressiveAcrossDistantCursorPositions(t *testing.T) {
	cx, cy := 56*62.5/128, 56*59.5/128
	at := func(dx, dy float64) gazePoint {
		return avatarGazeTarget(cx+dx, cy+dy, 0, 0, 56, 56)
	}
	// Moving 360 DIP vertically across the opposite side of a desktop should
	// move the eyes at least 0.9 DIP, even 1400 DIP away horizontally.
	up, down := at(-1400, -180), at(-1400, 180)
	if (down.y-up.y)*54/128 < .9 {
		t.Fatal("distant horizontal cursor suppressed vertical eye movement", up, down)
	}
	// Also retain motion along the distance axis instead of immediately
	// pinning the eyes to the same edge everywhere beyond the avatar.
	if (at(1600, 0).x-at(800, 0).x)*54/128 < .2 {
		t.Fatal("far cursor distance no longer affects the eyes")
	}
	// Independent axes must still be continuous at the center and clamp.
	for _, x := range []float64{-3000, -800, -1, 0, 1, 800, 3000} {
		previous := at(x, -3000)
		for y := -2999.; y <= 3000; y++ {
			next := at(x, y)
			if next.y < previous.y || math.Hypot(next.x-previous.x, next.y-previous.y) > .03 {
				t.Fatal("mapping jumps or reverses while moving the cursor", previous, next)
			}
			previous = next
		}
	}
}

func TestAvatarGazeSleepsWithoutRedrawingAndCancelsQueuedFrames(t *testing.T) {
	now := time.Unix(100, 0)
	var callback func()
	var delay time.Duration
	pending, paints, samples := false, 0, 0
	target := gazePoint{3, -2}
	visible := true
	g := avatarGaze{
		now:     func() time.Time { return now },
		sample:  func() (gazePoint, bool) { samples++; return target, visible },
		repaint: func() { paints++ },
		schedule: func(d time.Duration, f func()) func() {
			if pending {
				t.Fatal("multiple pending timers")
			}
			pending, callback, delay = true, f, d
			return func() { pending = false }
		},
	}
	tick := func() {
		t.Helper()
		if !pending {
			t.Fatal("no timer scheduled")
		}
		pending = false
		now = now.Add(delay)
		callback()
	}
	g.start()
	g.start() // Repeated model synchronization must not add timers.
	tick()
	if g.offset.x <= 0 || g.offset.x >= target.x || g.offset.y >= 0 {
		t.Fatal("eyes should ease toward, not jump to, their target")
	}
	for i := 0; i < 30; i++ {
		tick()
	}
	if g.offset != target || delay != gazeIdle {
		t.Fatal("eyes did not settle into idle sampling", g.offset, delay)
	}
	before := paints
	for i := 0; i < 20; i++ {
		tick()
	}
	if paints != before {
		t.Fatal("stationary cursor keeps repainting")
	}
	stale := callback
	g.stop()
	beforeSamples := samples
	stale()
	if pending || samples != beforeSamples {
		t.Fatal("hidden/stopped avatar still polls")
	}
	g.start()
	stale() // A callback from before hide must not mutate a restarted loop.
	if samples != beforeSamples {
		t.Fatal("old callback survived restart")
	}
	visible = false
	tick()
	if g.running || pending {
		t.Fatal("invisible window kept animation alive")
	}
}

func TestAvatarGazeOnlyMovesEyes(t *testing.T) {
	render := func(offset gazePoint) image.Image {
		return ui.NewTester(func(c *ui.Context) {
			c.Root().Background(ui.Transparent)
			ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) { drawAvatarGaze(p, r, offset) })
		}, 128, 128).Image()
	}
	left, right := render(gazePoint{-gazeMaxX, 0}), render(gazePoint{gazeMaxX, 0})
	changed := 0
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			a, b, c, d := left.At(x, y).RGBA()
			e, f, g, h := right.At(x, y).RGBA()
			if a != e || b != f || c != g || d != h {
				changed++
				if x < 33 || x > 91 || y < 47 || y > 73 {
					t.Fatalf("eye movement changed the body at %d,%d", x, y)
				}
			}
		}
	}
	if changed < 30 {
		t.Fatal("eye layers did not visibly move", changed)
	}
	if _, _, _, alpha := right.At(0, 0).RGBA(); alpha != 0 {
		t.Fatal("gaze introduced a background")
	}
	if path := os.Getenv("SIDELET_GAZE_VISUAL"); path != "" {
		states := []gazePoint{{-gazeMaxX, 0}, {0, -gazeMaxY}, {}, {0, gazeMaxY}, {gazeMaxX, 0}}
		u := ui.NewTester(func(c *ui.Context) {
			c.Root().Background(ui.Hex("#edf0ea"))
			ui.Column(c).Padding(16).Gap(8).Children(func() {
				for _, size := range []float32{128, 56} {
					ui.Row(c).Gap(12).Children(func() {
						for _, state := range states {
							ui.Box(c).Size(size, size).Draw(func(p *ui.Painter, r ui.Rect) { drawAvatarGaze(p, r, state) })
						}
					})
				}
			})
		}, 720, 224)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, u.Image()); err != nil {
			t.Fatal(err)
		}
	}
}
