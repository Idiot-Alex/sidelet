package main

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Render the actual vector through the application's renderer for visual review.
func TestAvatarVectorPreview(t *testing.T) {
	path := os.Getenv("SIDELET_AVATAR_VISUAL")
	if path == "" {
		t.Skip("set SIDELET_AVATAR_VISUAL to render the avatar")
	}
	u := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#f2f4ef"))
		ui.Row(c).Padding(24).Gap(24).AlignItems(ui.Center).Children(func() {
			ui.Image(c, sidebarAvatarVector()).Size(224, 224)
			ui.Image(c, sidebarAvatarVector()).Size(56, 56)
		})
	}, 352, 272)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, u.Image()); err != nil {
		t.Fatal(err)
	}
}
