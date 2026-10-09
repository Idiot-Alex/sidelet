package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

//go:embed design/web-ui.json
var webDesignJSON []byte

type webDesign struct {
	Themes map[string]map[string]string `json:"themes"`
	Icons  map[string]string            `json:"icons"`
}

var productionDesign = func() webDesign {
	var d webDesign
	if err := json.Unmarshal(webDesignJSON, &d); err != nil {
		panic(err)
	}
	return d
}()

var productionIcons = func() map[string]*ui.SVG {
	out := make(map[string]*ui.SVG)
	for name, path := range productionDesign.Icons {
		out[name] = ui.MustParseSVG([]byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="%s"/></svg>`, path)))
	}
	return out
}()

type visualTheme struct {
	*ui.Theme
	ID                                                string
	Alt, Field, Subtle, Soft, WarningSoft, DangerSoft ui.Color
	Radius                                            float32
	HeadingFont                                       string
	HeadingWeight                                     int
}

var webThemes = func() map[string]*visualTheme {
	out := map[string]*visualTheme{}
	for _, id := range []string{"mac", "paper", "graphite"} {
		out[id] = buildWebTheme(id)
	}
	return out
}()

func webTheme(id string) *visualTheme {
	if t, ok := webThemes[id]; ok {
		return t
	}
	return webThemes["mac"]
}

func buildWebTheme(id string) *visualTheme {
	values, ok := productionDesign.Themes[id]
	if !ok {
		id, values = "mac", productionDesign.Themes["mac"]
	}
	color := func(key string) ui.Color { return ui.Hex(values[key]) }
	number := func(key string) float32 {
		n, err := strconv.ParseFloat(strings.TrimSuffix(values[key], "px"), 32)
		if err != nil {
			panic(err)
		}
		return float32(n)
	}
	t := ui.LightTheme()
	t.Dark = id == "graphite"
	t.Background, t.Surface, t.Border = color("app-bg"), color("surface"), color("line")
	t.SurfaceHover, t.SurfacePressed = color("surface-alt"), color("accent-soft")
	t.Text, t.TextMuted = color("ink"), color("muted")
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText = color("accent"), color("accent-hover"), color("accent-hover"), color("on-accent")
	t.Danger, t.Warning, t.Focus = color("danger"), color("warning"), t.Accent
	t.Radius, t.FontSize = number("control-radius"), 13
	t.Font = "system-ui, Helvetica Neue, PingFang SC, sans-serif"
	d := &visualTheme{Theme: t, ID: id, Alt: color("surface-alt"), Field: color("field"), Subtle: color("subtle"), Soft: color("accent-soft"), WarningSoft: color("warning-soft"), DangerSoft: color("danger-soft"), Radius: number("radius"), HeadingWeight: int(number("heading-weight"))}
	d.HeadingFont = t.Font
	if id == "paper" {
		d.HeadingFont = "Iowan Old Style, Songti SC, STSong, Georgia, serif"
	}
	return d
}

func webIcon(c *ui.Context, name string, size float32) {
	ui.Icon(c, productionIcons[name]).Size(size, size)
}

func webButton(c *ui.Context, label string, primary bool) *ui.Element {
	b := ui.ButtonBase(c).Label(label).FontSize(12).LineHeight(4.0/3).Padding(7, 10).Radius(c.Theme().Radius).Border(1, c.Theme().Border).TextColor(c.Theme().TextMuted)
	webFocus(c, b, c.Theme().Radius)
	if primary {
		b.Background(c.Theme().Accent).Border(1, c.Theme().Accent).TextColor(c.Theme().AccentText).FontWeight(550)
		if b.Hovered() || b.Pressed() {
			b.Background(c.Theme().AccentHover).BorderColor(c.Theme().AccentHover)
		}
	} else if b.Hovered() || b.Pressed() {
		b.Background(c.Theme().SurfaceHover).TextColor(c.Theme().Text)
	}
	return b
}

// Match theme.css: a 2px keyboard outline separated by a 3px gap.
// Pointer clicks retain their normal appearance.
func webFocus(c *ui.Context, e *ui.Element, radius float32) *ui.Element {
	e.FocusRing(false).DrawOver(func(p *ui.Painter, r ui.Rect) {
		if e.FocusVisible() {
			p.Stroke(ui.Rect{X: r.X - 5, Y: r.Y - 5, W: r.W + 10, H: r.H + 10}, c.Theme().Focus, radius+5, 2)
		}
	})
	return e
}

func webTextInput(c *ui.Context, value *string) *ui.Element {
	return webFocus(c, ui.TextInput(c, value), c.Theme().Radius)
}

func webTextArea(c *ui.Context, value *string) *ui.Element {
	return webFocus(c, ui.TextArea(c, value), c.Theme().Radius)
}

func webSelect(c *ui.Context, value *string, options []string) *ui.Element {
	return webFocus(c, ui.Select(c, value, options), c.Theme().Radius)
}

func webActionText(c *ui.Context, text string) *ui.Element {
	return ui.Text(c, text).Height(16).FixedLineHeight(16)
}

func webIconButton(c *ui.Context, label, name string, size float32) *ui.Element {
	b := webButton(c, label, false).Size(28, 28).Padding(0).Border(0, ui.Transparent)
	b.Children(func() { webIcon(c, name, size) })
	return b
}

func webCardButton(c *ui.Context, label string, primary bool) *ui.Element {
	b := webButton(c, label, primary).Grow(1).Height(36).Padding(0, 5).Gap(5)
	if !primary {
		t := webThemeFor(c)
		b.Background(t.Alt).TextColor(t.Text)
		if b.Hovered() || b.Pressed() {
			b.Background(t.Soft)
		}
	}
	return b
}

func webThemeFor(c *ui.Context) *visualTheme {
	for _, theme := range webThemes {
		if theme.Theme == c.Theme() {
			return theme
		}
	}
	return webTheme("mac")
}
