package main

import (
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
		out[id] = &visual
	}
	return out
}()

func (m *model) glassAllowed() bool {
	return m.Preferences.Appearance.Translucent && !m.reduceTransparency
}

func cardInputBackingAlpha(m *model) float64 {
	if m.glassAllowed() {
		// Keep a real hit surface above WindowServer's transparent-pixel
		// threshold, without painting an opaque panel behind the blur.
		return .05
	}
	return 1
}

func surfaceTheme(c *ui.Context, m *model) *visualTheme {
	base := webTheme(m.UITheme)
	// Unsupported compositors and accessibility settings always keep a
	// readable, opaque background. No window-wide opacity is applied.
	if m.glassAllowed() && c.Vibrancy() && !c.Preferences().HighContrast {
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
	enabled := m.glassAllowed()
	if !g.initialized || enabled != g.enabled {
		material := mygo.VibrancyNone
		if enabled {
			material = g.material
		}
		g.window.SetVibrancy(material)
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
