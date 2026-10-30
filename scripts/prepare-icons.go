//go:build ignore

// Run from the repository root: go run scripts/prepare-icons.go
// The desktop SVG is the source of truth; no external image tools are needed.
package main

import (
	"bytes"
	"encoding/binary"
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func read(path string) []byte     { b, e := os.ReadFile(path); must(e); return b }
func write(path string, b []byte) { must(os.WriteFile(path, b, 0644)) }
func render(svg *ui.SVG, size int) []byte {
	u := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.Transparent)
		ui.Image(c, svg).Size(float32(size), float32(size))
	}, size, size)
	var b bytes.Buffer
	must(png.Encode(&b, u.Image()))
	return b.Bytes()
}
func main() {
	fullBytes := read("cmd/sidelet/design/sidebar-avatar.svg")
	full := ui.MustParseSVG(fullBytes)
	write("assets/sidelet-icon.svg", fullBytes)
	write("website/public/sidelet-icon.svg", fullBytes)
	write("website/public/favicon.svg", fullBytes)
	write("assets/sidelet-icon.png", render(full, 1024))
	write("cmd/sidelet/design/tray-icon.png", render(full, 32))
	sizes := []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
	frames := make([][]byte, len(sizes))
	for i, s := range sizes {
		frames[i] = render(full, s)
	}
	var ico bytes.Buffer
	for _, n := range []uint16{0, 1, uint16(len(sizes))} {
		must(binary.Write(&ico, binary.LittleEndian, n))
	}
	offset := 6 + 16*len(sizes)
	for i, s := range sizes {
		ico.Write([]byte{byte(s % 256), byte(s % 256), 0, 0})
		must(binary.Write(&ico, binary.LittleEndian, uint16(1)))
		must(binary.Write(&ico, binary.LittleEndian, uint16(32)))
		must(binary.Write(&ico, binary.LittleEndian, uint32(len(frames[i]))))
		must(binary.Write(&ico, binary.LittleEndian, uint32(offset)))
		offset += len(frames[i])
	}
	for _, frame := range frames {
		ico.Write(frame)
	}
	write("assets/sidelet.ico", ico.Bytes())
	write("cmd/sidelet/design/app-icon.ico", ico.Bytes())
	var chunks bytes.Buffer
	for _, entry := range []struct {
		kind string
		size int
	}{{"icp4", 16}, {"icp5", 32}, {"icp6", 64}, {"ic07", 128}, {"ic08", 256}, {"ic09", 512}, {"ic10", 1024}} {
		data := render(full, entry.size)
		chunks.WriteString(entry.kind)
		must(binary.Write(&chunks, binary.BigEndian, uint32(8+len(data))))
		chunks.Write(data)
	}
	var icns bytes.Buffer
	icns.WriteString("icns")
	must(binary.Write(&icns, binary.BigEndian, uint32(8+chunks.Len())))
	icns.Write(chunks.Bytes())
	write("assets/sidelet.icns", icns.Bytes())
	// A local review sheet includes the exact small sizes used by the shell.
	must(os.MkdirAll("build/results", 0755))
	preview := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#edf0ea"))
		ui.Row(c).Padding(20).Gap(20).AlignItems(ui.Center).Children(func() {
			ui.Image(c, full).Size(192, 192)
			for _, size := range []float32{16, 24, 32, 48} {
				ui.Image(c, full).Size(size, size)
			}
		})
	}, 440, 232)
	var review bytes.Buffer
	must(png.Encode(&review, preview.Image()))
	write("build/results/brand-icons.png", review.Bytes())
}
