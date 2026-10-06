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
npm run build:frontend
bash scripts/go.sh test ./internal/spike ./internal/storage ./internal/reminder ./internal/settings ./internal/exportdata ./internal/quickadd ./internal/integration ./cmd/spike
bash scripts/go.sh vet ./cmd/spike ./internal/platform

mkdir -p "$sidelet_root/build/bin"
sidelet_version="$(python3 scripts/macos-bundle-info.py version)"
sidelet_build="$(python3 scripts/macos-bundle-info.py build)"
sidelet_bundle="$sidelet_root/build/bin/Sidelet.app"
# Build in a fresh temporary bundle so removed resources cannot leak into releases.
sidelet_work="$(mktemp -d "$sidelet_root/build/bin/.sidelet-build.XXXXXX")"
trap 'rm -rf "$sidelet_work"' EXIT
sidelet_staged="$sidelet_work/Sidelet.app"
mkdir -p "$sidelet_staged/Contents/MacOS" "$sidelet_staged/Contents/Resources" "$sidelet_root/.cache/clang"
GOOS=darwin GOARCH="$sidelet_arch" CGO_ENABLED=1 bash scripts/go.sh build -tags production -trimpath \
  -ldflags "-X main.appVersion=$sidelet_version -X main.appBuild=$sidelet_build" \
  -o "$sidelet_staged/Contents/MacOS/sidelet" ./cmd/spike
swiftc -module-cache-path "$sidelet_root/.cache/clang" scripts/render-macos-icon.swift -o "$sidelet_work/render-icon"
"$sidelet_work/render-icon" assets/sidelet-icon.svg "$sidelet_work/Sidelet.iconset"
iconutil -c icns "$sidelet_work/Sidelet.iconset" -o "$sidelet_staged/Contents/Resources/Sidelet.icns"
python3 scripts/macos-bundle-info.py --plist "$sidelet_staged/Contents/Info.plist"
plutil -lint "$sidelet_staged/Contents/Info.plist"
codesign --force --sign - "$sidelet_staged"
codesign --verify --strict "$sidelet_staged"
rm -rf "$sidelet_bundle"
mv "$sidelet_staged" "$sidelet_bundle"
printf 'Built %s %s (%s, build %s)\n' "$sidelet_bundle" "$sidelet_version" "$sidelet_arch" "$sidelet_build"
