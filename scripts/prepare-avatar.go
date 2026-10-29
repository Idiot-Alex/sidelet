//go:build ignore

// Run from the repository root: bash scripts/go.sh run scripts/prepare-avatar.go
package main

import (
	"image"
	"image/png"
	"os"

	"golang.org/x/image/draw"
)

func main() {
	if err := generate(); err != nil {
		panic(err)
	}
}

func generate() error {
	f, err := os.Open("assets/sidebar-avatar-source.png")
	if err != nil {
		return err
	}
	source, err := png.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	// 56 DIP at 3x; preserve the source composition and alpha channel.
	target := image.NewNRGBA(image.Rect(0, 0, 168, 168))
	draw.CatmullRom.Scale(target, target.Bounds(), source, source.Bounds(), draw.Src, nil)
	out, err := os.Create("cmd/sidelet/design/sidebar-avatar.png")
	if err != nil {
		return err
	}
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	err = encoder.Encode(out, target)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
