package main

import (
	"log"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Keep opaque themes immutable: sidebar markers, controls and fallback
// rendering must never inherit an alpha change from another window.
var glassThemes = func() map[string]*visualTheme {
	out := make(map[string]*visualTheme, len(webThemes))
	for id, base := range webThemes {
		visual, theme := *base, *base.Theme
		visual.Theme = &theme
		theme.Background = base.Background.Alpha(.08)
		theme.Surface = base.Surface.Alpha(.38)
		theme.Border = base.Border.Alpha(.7)
		theme.SurfaceHover = base.SurfaceHover.Alpha(.7)
		theme.SurfacePressed = base.SurfacePressed.Alpha(.85)
		visual.Alt, visual.Field, visual.Soft = base.Alt.Alpha(.7), base.Field.Alpha(.82), base.Soft.Alpha(.88)
		if runtime.GOOS == "windows" {
			// Keep the canvas optically clear. Reading surfaces supply their
			// own contrast, rather than tinting the whole window gray.
			theme.Background = base.Background.Alpha(.06)
			if id == "graphite" {
				theme.Background = base.Background.Alpha(.22)
			}
			theme.Surface = base.Surface.Alpha(.32)
			theme.Border = ui.RGB(255, 255, 255).Alpha(.48)
			theme.TextMuted = base.Text
			visual.Subtle = base.TextMuted
			visual.Alt = base.Surface.Alpha(.7)
			visual.Field = base.Surface.Alpha(.56)
		}
		out[id] = &visual
	}
	return out
}()

// A thin reflection and bright inner edge give transparent surfaces a visible
// boundary. These are paint only: they do not intercept input or fade text.
func glassPanel(c *ui.Context, panel ui.Element, t *visualTheme, radius float32) ui.Element {
	if t.Surface.A == 255 {
		return panel
	}
	base := webTheme(t.ID)
	from, to := base.Surface.Alpha(.72), base.Surface.Alpha(.62)
	highlight := ui.RGB(255, 255, 255).Alpha(.68)
	if t.Dark {
		from, to = base.Surface.Alpha(.78), base.Surface.Alpha(.60)
		highlight = ui.RGB(255, 255, 255).Alpha(.24)
	}
	return panel.LinearGradient(ui.LinearGradient{From: from, To: to, Angle: 155}).
		Border(1, highlight).Radius(radius).DrawOver(func(p *ui.Painter, r ui.Rect) {
		p.FillGradient(ui.Rect{X: r.X + radius, Y: r.Y + 1, W: max(0, r.W-2*radius), H: 1},
			ui.LinearGradient{From: highlight, To: highlight.Alpha(0), Angle: 90}, 0)
	})
}

func (m *model) desktopGlassAllowed() bool {
	return m.Preferences.Appearance.DesktopTranslucent && !m.reduceTransparency
}

func cardInputBackingAlpha(m *model) float64 {
	if m.desktopGlassAllowed() {
		// Keep a real hit surface above WindowServer's transparent-pixel
		// threshold, without painting an opaque panel behind the blur.
		return .05
	}
	return 1
}

func desktopSurfaceTheme(c *ui.Context, m *model) *visualTheme {
	return surfaceThemeFor(c, m, m.desktopGlassAllowed())
}

func surfaceThemeFor(c *ui.Context, m *model, enabled bool) *visualTheme {
	base := webTheme(m.UITheme)
	// Unsupported compositors and accessibility settings always keep a
	// readable, opaque background. No window-wide opacity is applied.
	if enabled && c.Vibrancy() && !c.Preferences().HighContrast {
		return glassThemes[base.ID]
	}
	return base
}

// Native materials are updated only when the preference changes, never
// from render frames. Windows requires material capability at creation.
type nativeGlassWindow struct {
	window      *mygo.Window
	material    mygo.Vibrancy
	card        bool
	initialized bool
	enabled     bool
	radius      float32
}

func (g *nativeGlassWindow) bind(w *mygo.Window, m *model, material mygo.Vibrancy, card bool) {
	*g = nativeGlassWindow{window: w, material: material, card: card}
	g.update(m)
}

func (g *nativeGlassWindow) update(m *model) {
	if g.window == nil {
		return
	}
	enabled := m.desktopGlassAllowed()
	if !g.initialized || enabled != g.enabled {
		material := mygo.VibrancyNone
		if enabled {
			material = g.material
		}
		g.window.SetVibrancy(material)
		if err := configureClearTransparency(g.window, enabled); err != nil {
			// Disable the SDK capability flag too, so the renderer selects the
			// opaque theme instead of leaving an unreadable black backdrop.
			g.window.SetVibrancy(mygo.VibrancyNone)
			log.Printf("clear transparency unavailable; using opaque background: %v", err)
		}
	}
	radius := float32(0)
	if enabled && g.card {
		radius = webTheme(m.UITheme).Radius
	}
	if g.card && (!g.initialized || radius != g.radius) {
		setGlassCornerRadius(g.window, radius)
	}
	g.enabled, g.radius, g.initialized = enabled, radius, true
}
