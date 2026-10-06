#!/usr/bin/env bash
set -euo pipefail
sidelet_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$sidelet_root"
mkdir -p build/results
clang -fobjc-arc -fblocks -Wno-deprecated-declarations -mmacosx-version-min=13.0 \
  -framework Cocoa -framework ApplicationServices -framework Carbon -framework WebKit \
  scripts/test-macos-hit-regions.m -o build/results/test-macos-hit-regions
exec build/results/test-macos-hit-regions
