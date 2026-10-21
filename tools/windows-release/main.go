// windows-release is a host-side build tool. Its dependencies are never linked
// into Sidelet. Run it from either macOS or Windows; no Windows SDK is needed.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

type appInfo struct {
	Name       string `json:"name"`
	Identifier string `json:"bundleIdentifier"`
	Version    string `json:"version"`
	Build      string `json:"build"`
}

func main() {
	root := flag.String("root", ".", "repository root")
	arch := flag.String("arch", "amd64", "amd64 or arm64")
	pack := flag.Bool("package", false, "also create and verify a ZIP and SHA-256 file")
	flag.Parse()
	if err := run(*root, *arch, *pack); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadInfo(root string) (appInfo, [4]uint16, error) {
	var info appInfo
	var fixed [4]uint16
	data, err := os.ReadFile(filepath.Join(root, "packaging/macos/app.json"))
	if err != nil {
		return info, fixed, err
	}
	if err = json.Unmarshal(data, &info); err != nil {
		return info, fixed, err
	}
	if info.Name != "Sidelet" || info.Identifier == "" || !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(info.Version) || !regexp.MustCompile(`^\d+$`).MatchString(info.Build) {
		return info, fixed, errors.New("invalid shared application metadata")
	}
	for i, part := range strings.Split(info.Version+"."+info.Build, ".") {
		n, err := strconv.ParseUint(part, 10, 16)
		if err != nil {
			return info, fixed, fmt.Errorf("Windows version component: %w", err)
		}
		fixed[i] = uint16(n)
	}
	return info, fixed, nil
}

func resources(root string, info appInfo, fixed [4]uint16) (*winres.ResourceSet, error) {
	f, err := os.Open(filepath.Join(root, "assets/sidelet-icon.png"))
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		return nil, err
	}
	icon, err := winres.NewIconFromResizedImage(img, []int{16, 20, 24, 32, 40, 48, 64, 128, 256})
	if err != nil {
		return nil, err
	}
	rs := &winres.ResourceSet{}
	// MyGo's Windows backend loads icon resource 1 for windows and notifications.
	if err = rs.SetIcon(winres.ID(1), icon); err != nil {
		return nil, err
	}
	vi := version.Info{FileVersion: fixed, ProductVersion: [4]uint16{fixed[0], fixed[1], fixed[2], 0}, Type: version.App}
	vi.Flags.Prerelease = true
	for key, value := range map[string]string{
		version.CompanyName: "Sidelet", version.FileDescription: "Sidelet", version.InternalName: "Sidelet",
		version.OriginalFilename: "Sidelet.exe", version.ProductName: info.Name,
		version.FileVersion: info.Version + "." + info.Build, version.ProductVersion: info.Version,
		version.Comments: "Local Windows test build; runtime acceptance pending.", "MyGoIdentifier": info.Identifier,
	} {
		if err = vi.Set(winres.LCIDDefault, key, value); err != nil {
			return nil, err
		}
	}
	rs.SetVersionInfo(vi)
	rs.SetManifest(winres.AppManifest{
		Identity:    winres.AssemblyIdentity{Name: info.Identifier, Version: fixed},
		Description: "Sidelet", Compatibility: winres.Win10AndAbove,
		ExecutionLevel: winres.AsInvoker, DPIAwareness: winres.DPIPerMonitorV2, UseCommonControlsV6: true,
	})
	return rs, nil
}

func goCommand(root, arch string, args ...string) error {
	name := "go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", name), args...)
	cmd.Dir = root
	// Replace inherited targets: the release tool itself always runs on the host.
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if !strings.EqualFold(key, "GOOS") && !strings.EqualFold(key, "GOARCH") && !strings.EqualFold(key, "CGO_ENABLED") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "GOOS=windows", "GOARCH="+arch, "CGO_ENABLED=0")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func run(root, arch string, pack bool) error {
	if arch != "amd64" && arch != "arm64" {
		return errors.New("architecture must be amd64 or arm64")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	info, fixed, err := loadInfo(root)
	if err != nil {
		return err
	}
	rs, err := resources(root, info, fixed)
	if err != nil {
		return err
	}
	object := filepath.Join(root, "cmd/sidelet/sidelet_windows_"+arch+".syso")
	// Never overwrite a resource owned by another build or silently reuse one
	// left by a killed process. A normal error path always removes our object.
	f, err := os.OpenFile(object, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("resource object already exists or cannot be created (%s): %w", object, err)
	}
	defer os.Remove(object)
	err = rs.WriteObject(f, winres.Arch(arch))
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = goCommand(root, arch, "vet", "./cmd/sidelet"); err != nil {
		return err
	}
	binDir := filepath.Join(root, "build/bin")
	if err = os.MkdirAll(binDir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(binDir, ".sidelet-"+arch+"-*.exe")
	if err != nil {
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err = goCommand(root, arch, "build", "-trimpath", "-ldflags=-s -w -H=windowsgui -X github.com/egoist/mygo.production=1", "-o", tmp.Name(), "./cmd/sidelet"); err != nil {
		return err
	}
	if err = verifyPE(tmp.Name(), arch, rs); err != nil {
		return err
	}
	exe := filepath.Join(binDir, "Sidelet-"+arch+".exe")
	if err = os.Rename(tmp.Name(), exe); err != nil {
		return err
	}
	// Reuse the executable's exact multi-resolution icon for the installer.
	icon, err := rs.GetIcon(winres.ID(1))
	if err != nil {
		return err
	}
	var ico bytes.Buffer
	if err = icon.SaveICO(&ico); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(binDir, "Sidelet-"+arch+".ico"), ico.Bytes(), 0644); err != nil {
		return err
	}
	fmt.Printf("Verified %s (version %s, build %s)\n", exe, info.Version, info.Build)
	if pack {
		return packageZIP(root, arch, exe, info)
	}
	return nil
}

func verifyPE(path, arch string, expected *winres.ResourceSet) error {
	p, err := pe.Open(path)
	if err != nil {
		return err
	}
	defer p.Close()
	machine := uint16(pe.IMAGE_FILE_MACHINE_AMD64)
	if arch == "arm64" {
		machine = pe.IMAGE_FILE_MACHINE_ARM64
	}
	h, ok := p.OptionalHeader.(*pe.OptionalHeader64)
	if p.Machine != machine || !ok || h.Subsystem != pe.IMAGE_SUBSYSTEM_WINDOWS_GUI {
		return errors.New("executable has incorrect architecture or subsystem")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	actual, err := winres.LoadFromEXE(f)
	if err != nil {
		return err
	}
	if actual.Count() != expected.Count() {
		return errors.New("unexpected executable resource count")
	}
	var mismatch error
	expected.Walk(func(typ, id winres.Identifier, lang uint16, data []byte) bool {
		if !bytes.Equal(data, actual.Get(typ, id, lang)) {
			mismatch = fmt.Errorf("executable resource differs: type=%v id=%v language=%d", typ, id, lang)
			return false
		}
		return true
	})
	return mismatch
}

func packageZIP(root, arch, exe string, info appInfo) error {
	dir := filepath.Join(root, "build/releases")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	work, err := os.MkdirTemp(dir, ".windows-"+arch+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	name := fmt.Sprintf("Sidelet-%s-build%s-windows-%s-local.zip", info.Version, info.Build, arch)
	content := map[string][]byte{}
	content["Sidelet.exe"], err = os.ReadFile(exe)
	if err != nil {
		return err
	}
	for _, file := range []string{"使用说明.txt", "实机验收.md"} {
		data, err := os.ReadFile(filepath.Join(root, "packaging/windows", file))
		if err != nil {
			return err
		}
		data = bytes.ReplaceAll(data, []byte("{{VERSION}}"), []byte(info.Version+" (build "+info.Build+")"))
		data = bytes.ReplaceAll(data, []byte("{{ARCH}}"), []byte(arch))
		normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
		content[file] = []byte(strings.ReplaceAll(normalized, "\n", "\r\n"))
	}
	metadata := map[string]any{"name": info.Name, "version": info.Version, "build": info.Build, "arch": arch, "runtimeAcceptance": "pending", "authenticodeSigned": false}
	buildInfo, err := buildinfo.ReadFile(exe)
	if err != nil {
		return err
	}
	metadata["goVersion"] = buildInfo.GoVersion
	for _, setting := range buildInfo.Settings {
		if setting.Key == "vcs.revision" {
			metadata["sourceRevision"] = setting.Value
		}
		if setting.Key == "vcs.modified" {
			metadata["sourceModified"] = setting.Value == "true"
		}
	}
	content["build-info.json"], err = json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(work, name))
	if err != nil {
		return err
	}
	z := zip.NewWriter(file)
	folder := strings.TrimSuffix(name, ".zip") + "/"
	for _, entry := range []string{"Sidelet.exe", "使用说明.txt", "实机验收.md", "build-info.json"} {
		h := &zip.FileHeader{Name: folder + entry, Method: zip.Deflate}
		h.SetMode(0644)
		h.SetModTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		w, err := z.CreateHeader(h)
		if err != nil {
			z.Close()
			file.Close()
			return err
		}
		if _, err = w.Write(content[entry]); err != nil {
			z.Close()
			file.Close()
			return err
		}
	}
	if err = z.Close(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = verifyZIP(filepath.Join(work, name), folder, content); err != nil {
		return err
	}
	// verifyZIP read every archived byte (including the PE) and checked its CRC
	// and equality with the already verified inputs.
	data, err := os.ReadFile(filepath.Join(work, name))
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	checksum := hex.EncodeToString(digest[:]) + "  " + name + "\n"
	if err = os.WriteFile(filepath.Join(work, name+".sha256"), []byte(checksum), 0644); err != nil {
		return err
	}
	if err = os.Rename(filepath.Join(work, name), filepath.Join(dir, name)); err != nil {
		return err
	}
	if err = os.Rename(filepath.Join(work, name+".sha256"), filepath.Join(dir, name+".sha256")); err != nil {
		return err
	}
	fmt.Printf("Packaged and verified %s\n%s", filepath.Join(dir, name), checksum)
	return nil
}

func verifyZIP(path, folder string, expected map[string][]byte) error {
	z, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer z.Close()
	if len(z.File) != len(expected) {
		return errors.New("unexpected ZIP entry count")
	}
	seen := map[string]bool{}
	for _, f := range z.File {
		name := strings.TrimPrefix(f.Name, folder)
		want, ok := expected[name]
		if !ok || f.Name != folder+name || seen[name] {
			return errors.New("unexpected or duplicate ZIP entry")
		}
		seen[name] = true
		r, err := f.Open()
		if err != nil {
			return err
		}
		got, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("ZIP content mismatch: %s", name)
		}
	}
	return nil
}
