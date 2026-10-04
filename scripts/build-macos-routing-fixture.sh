#!/usr/bin/env bash
set -euo pipefail
sidelet_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$sidelet_root"
sidelet_fixture="build/results/Sidelet Routing Fixture.app"
mkdir -p "$sidelet_fixture/Contents/MacOS"
clang -fobjc-arc -fblocks -Wno-deprecated-declarations -mmacosx-version-min=13.0 \
  -framework Cocoa -framework ApplicationServices -framework WebKit -framework Carbon \
  scripts/macos-routing-fixture.m -o "$sidelet_fixture/Contents/MacOS/routing-fixture"
cat > "$sidelet_fixture/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleName</key><string>Sidelet Routing Fixture</string>
  <key>CFBundleIdentifier</key><string>io.sidelet.routingfixture</string>
  <key>CFBundleExecutable</key><string>routing-fixture</string>
  <key>LSUIElement</key><true/>
  <key>NSPrincipalClass</key><string>NSApplication</string>
</dict></plist>
PLIST
codesign --force --sign - "$sidelet_fixture"
printf 'Built %s\n' "$sidelet_fixture"
