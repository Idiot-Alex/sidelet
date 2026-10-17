package main

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func releaseFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"packaging/macos", "assets", "cmd/sidelet", "build/bin"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "packaging/macos/app.json"), []byte(`{"name":"Sidelet","bundleIdentifier":"io.sidelet.mygo-lab","version":"0.2.0","build":"31"}`), 0600); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets/sidelet-icon.png"), b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestFailedBuildPreservesExistingBinaryAndCleansOwnedObject(t *testing.T) {
	root := releaseFixture(t)
	exe := filepath.Join(root, "build/bin/Sidelet-amd64.exe")
	previous := []byte("previous successful build")
	if err := os.WriteFile(exe, previous, 0600); err != nil {
		t.Fatal(err)
	}
	// No app module/package exists in this fixture. Validation/build must fail.
	if err := run(root, "amd64", false); err == nil {
		t.Fatal("invalid project built successfully")
	}
	got, err := os.ReadFile(exe)
	if err != nil || !bytes.Equal(got, previous) {
		t.Fatal("failed build damaged existing binary", err)
	}
	if _, err := os.Stat(filepath.Join(root, "cmd/sidelet/sidelet_windows_amd64.syso")); !os.IsNotExist(err) {
		t.Fatal("failed build left generated resource", err)
	}
}

func TestBuildDoesNotOverwriteAnotherBuildsObject(t *testing.T) {
	root := releaseFixture(t)
	object := filepath.Join(root, "cmd/sidelet/sidelet_windows_arm64.syso")
	if err := os.WriteFile(object, []byte("another build owns this"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(root, "arm64", false); err == nil {
		t.Fatal("concurrent resource overwritten")
	}
	got, _ := os.ReadFile(object)
	if string(got) != "another build owns this" {
		t.Fatal("resource ownership lost")
	}
}

func TestWindowsMetadataRejectsVersionOverflow(t *testing.T) {
	root := releaseFixture(t)
	path := filepath.Join(root, "packaging/macos/app.json")
	if err := os.WriteFile(path, []byte(`{"name":"Sidelet","bundleIdentifier":"io.sidelet.mygo-lab","version":"0.2.0","build":"65536"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadInfo(root); err == nil {
		t.Fatal("Windows version silently truncated to 16 bits")
	}
}

func TestZIPVerificationRejectsChangedContentAndUnsafeEntries(t *testing.T) {
	for _, tt := range []struct{ name, entry, content string }{
		{"changed executable", "package/Sidelet.exe", "different binary"},
		{"path traversal", "package/../Sidelet.exe", "expected binary"},
		{"unexpected private file", "package/sidelet-native.sqlite3", "expected binary"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "package.zip")
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			z := zip.NewWriter(f)
			w, err := z.Create(tt.entry)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = w.Write([]byte(tt.content)); err != nil {
				t.Fatal(err)
			}
			if err = z.Close(); err != nil {
				t.Fatal(err)
			}
			if err = f.Close(); err != nil {
				t.Fatal(err)
			}
			if verifyZIP(path, "package/", map[string][]byte{"Sidelet.exe": []byte("expected binary")}) == nil {
				t.Fatal("invalid release archive accepted")
			}
		})
	}
}
