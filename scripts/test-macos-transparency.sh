#!/usr/bin/env bash
set -euo pipefail
sidelet_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$sidelet_root"
mkdir -p build/results
clang -fobjc-arc -fblocks -Wno-deprecated-declarations -mmacosx-version-min=13.0 \
  -framework Cocoa -framework WebKit -framework ApplicationServices -framework Carbon \
  scripts/test-macos-transparency.m -o build/results/test-macos-transparency
exec build/results/test-macos-transparency "${1:-build/results/macos-transparency.png}"
