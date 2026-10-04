#!/usr/bin/env bash
set -euo pipefail
sidelet_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$sidelet_root"
sidelet_fixture="build/results/MacInteractionFixture.app"
mkdir -p "$sidelet_fixture/Contents/MacOS"
clang -fobjc-arc -fblocks -Wno-deprecated-declarations -mmacosx-version-min=13.0 \
  -framework Cocoa -framework ApplicationServices scripts/macos-interaction-fixture.m \
  -o "$sidelet_fixture/Contents/MacOS/fixture"
cat > "$sidelet_fixture/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleName</key><string>MacInteractionFixture</string>
  <key>CFBundleIdentifier</key><string>io.sidelet.interactionfixture</string>
  <key>CFBundleExecutable</key><string>fixture</string>
  <key>NSPrincipalClass</key><string>NSApplication</string>
</dict></plist>
PLIST
codesign --force --sign - "$sidelet_fixture"
printf 'Built %s\n' "$sidelet_fixture"
