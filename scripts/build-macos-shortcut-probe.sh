#!/usr/bin/env bash
set -euo pipefail
sidelet_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$sidelet_root"
mkdir -p build/results
clang -fobjc-arc -Wno-deprecated-declarations -mmacosx-version-min=13.0 \
  -framework Cocoa -framework Carbon scripts/macos-shortcut-probe.m \
  -o build/results/macos-shortcut-probe
printf 'Built build/results/macos-shortcut-probe (registration only, no key delivery)\n'
