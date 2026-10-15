package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Opt-in artifacts capture the native card themes. This uses MyGo's
// renderer, not a desktop window capture or evidence of pointer handoff.
func TestCardVisualArtifacts(t *testing.T) {
	dir := os.Getenv("SIDELET_CARD_VISUAL_DIR")
	if dir == "" {
		t.Skip("set SIDELET_CARD_VISUAL_DIR to render comparison artifacts")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"mac", "paper", "graphite"} {
		m := newModel()
		m.UITheme = theme
		m.open(0)
		v := &views{m: m}
		u := ui.NewTester(v.webCard, 320, 390)
		u.SetSize(320, m.CardReadHeight)
		for _, mode := range []string{"read", "edit"} {
			if mode == "edit" {
				m.edit()
				u.SetSize(320, 390)
				u.Frame()
			}
			f, err := os.Create(filepath.Join(dir, "native-"+theme+"-"+mode+".png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, u.Image())
			closeErr := f.Close()
			if err != nil {
				t.Fatal(err)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
		}
	}
}
