package main

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestBrandIconAssetsStayInSync(t *testing.T) {
	read := func(path string) []byte {
		t.Helper()
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	for _, path := range []string{"../../assets/sidelet-icon.svg", "../../website/public/sidelet-icon.svg", "../../website/public/favicon.svg"} {
		if !bytes.Equal(read(path), sidebarAvatarSVG) {
			t.Fatalf("%s is stale; run scripts/prepare-icons.go", path)
		}
	}
	svg := ui.MustParseSVG(sidebarAvatarSVG)
	checkPNG := func(data []byte, size int) {
		t.Helper()
		img, e := png.Decode(bytes.NewReader(data))
		if e != nil {
			t.Fatal(e)
		}
		if img.Bounds().Dx() != size || img.Bounds().Dy() != size {
			t.Fatalf("wrong icon size: %v", img.Bounds())
		}
		_, _, _, alpha := img.At(0, 0).RGBA()
		if alpha != 0 {
			t.Fatal("icon corners must be transparent")
		}
		preview := ui.NewTester(func(c *ui.Context) {
			c.Root().Background(ui.Transparent)
			ui.Image(c, svg).Size(float32(size), float32(size))
		}, size, size)
		// Compare after the same PNG round trip: premultiplied renderer colors
		// otherwise differ slightly at antialiased, translucent edges.
		var rendered bytes.Buffer
		if err := png.Encode(&rendered, preview.Image()); err != nil {
			t.Fatal(err)
		}
		want, err := png.Decode(&rendered)
		if err != nil {
			t.Fatal(err)
		}
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				r, g, b, a := img.At(x, y).RGBA()
				wr, wg, wb, wa := want.At(x, y).RGBA()
				if r != wr || g != wg || b != wb || a != wa {
					t.Fatalf("%dpx icon differs from floating mascot at %d,%d; regenerate icons", size, x, y)
				}
			}
		}
	}
	checkPNG(nativeTrayIcon(), 32)
	ico := read("../../assets/sidelet.ico")
	if !bytes.Equal(ico, read("design/app-icon.ico")) {
		t.Fatal("taskbar icon is stale")
	}
	if len(ico) < 6 || binary.LittleEndian.Uint16(ico[2:4]) != 1 {
		t.Fatal("invalid ICO")
	}
	sizes := []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
	if int(binary.LittleEndian.Uint16(ico[4:6])) != len(sizes) {
		t.Fatal("missing Windows icon sizes")
	}
	for i, size := range sizes {
		entry := ico[6+i*16 : 22+i*16]
		start, length := int(binary.LittleEndian.Uint32(entry[12:])), int(binary.LittleEndian.Uint32(entry[8:]))
		if start < 0 || length < 0 || start+length > len(ico) {
			t.Fatal("ICO frame outside file")
		}
		checkPNG(ico[start:start+length], size)
	}
	icns := read("../../assets/sidelet.icns")
	if len(icns) < 8 || string(icns[:4]) != "icns" || int(binary.BigEndian.Uint32(icns[4:8])) != len(icns) {
		t.Fatal("invalid ICNS")
	}
	expected := map[string]int{"icp4": 16, "icp5": 32, "icp6": 64, "ic07": 128, "ic08": 256, "ic09": 512, "ic10": 1024}
	for offset := 8; offset < len(icns); {
		if offset+8 > len(icns) {
			t.Fatal("truncated ICNS")
		}
		kind := string(icns[offset : offset+4])
		length := int(binary.BigEndian.Uint32(icns[offset+4 : offset+8]))
		if length < 8 || offset+length > len(icns) {
			t.Fatal("invalid ICNS chunk")
		}
		size, ok := expected[kind]
		if !ok {
			t.Fatal("unknown/duplicate ICNS chunk", kind)
		}
		checkPNG(icns[offset+8:offset+length], size)
		delete(expected, kind)
		offset += length
	}
	if len(expected) != 0 {
		t.Fatal("missing macOS icon sizes")
	}
}
