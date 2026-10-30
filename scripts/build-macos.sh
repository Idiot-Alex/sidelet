#!/usr/bin/env bash
set -euo pipefail
sidelet_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$sidelet_root"
if [[ "$(uname -s)" != Darwin ]]; then
  printf '%s\n' 'Build the macOS app on macOS with Xcode Command Line Tools.' >&2
  exit 1
fi
sidelet_arch="${1:-arm64}"
case "$sidelet_arch" in arm64|amd64) ;; *) printf '%s\n' 'Architecture must be arm64 or amd64.' >&2; exit 1 ;; esac
sidelet_clang_arch="$sidelet_arch"
if [[ "$sidelet_arch" == amd64 ]]; then sidelet_clang_arch=x86_64; fi
export MACOSX_DEPLOYMENT_TARGET=13.0
export CGO_CFLAGS="${CGO_CFLAGS:-} -arch $sidelet_clang_arch"
export CGO_LDFLAGS="${CGO_LDFLAGS:-} -arch $sidelet_clang_arch"
export CGO_ENABLED=0
bash scripts/go.sh test ./cmd/sidelet ./internal/...
bash scripts/go.sh vet ./cmd/sidelet ./internal/...

mkdir -p "$sidelet_root/build/bin"
sidelet_version="$(python3 scripts/macos-bundle-info.py version)"
sidelet_build="$(python3 scripts/macos-bundle-info.py build)"
sidelet_bundle="$sidelet_root/build/bin/Sidelet.app"
# Build in a fresh temporary bundle so removed resources cannot leak into releases.
sidelet_work="$(mktemp -d "$sidelet_root/build/bin/.sidelet-build.XXXXXX")"
trap 'rm -rf "$sidelet_work"' EXIT
sidelet_staged="$sidelet_work/Sidelet.app"
mkdir -p "$sidelet_staged/Contents/MacOS" "$sidelet_staged/Contents/Resources" "$sidelet_root/.cache/clang"
GOOS=darwin GOARCH="$sidelet_arch" CGO_ENABLED=0 bash scripts/go.sh build -trimpath \
  -ldflags='-s -w' -o "$sidelet_staged/Contents/MacOS/sidelet" ./cmd/sidelet
cp assets/sidelet.icns "$sidelet_staged/Contents/Resources/Sidelet.icns"
python3 scripts/macos-bundle-info.py --plist "$sidelet_staged/Contents/Info.plist"
plutil -lint "$sidelet_staged/Contents/Info.plist"
python3 scripts/macos-package.py record-build "$sidelet_staged" "$sidelet_arch"
codesign --force --sign - "$sidelet_staged"
codesign --verify --strict "$sidelet_staged"
rm -rf "$sidelet_bundle"
mv "$sidelet_staged" "$sidelet_bundle"
printf 'Built %s %s (%s, build %s)\n' "$sidelet_bundle" "$sidelet_version" "$sidelet_arch" "$sidelet_build"
