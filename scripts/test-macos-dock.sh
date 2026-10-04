#!/usr/bin/env bash
set -euo pipefail
sidelet_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$sidelet_root"
mkdir -p build/results
clang -fobjc-arc -fblocks -Wno-deprecated-declarations -mmacosx-version-min=13.0 \
  -framework Cocoa -framework ServiceManagement \
  scripts/test-macos-dock.m -o build/results/test-macos-dock
exec build/results/test-macos-dock
