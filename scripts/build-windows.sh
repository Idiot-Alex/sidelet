#!/usr/bin/env bash
set -euo pipefail
sidelet_root="$(cd "$(dirname "$0")/.." && pwd)"
sidelet_arch="${1:-amd64}"
case "$sidelet_arch" in amd64|arm64) ;; *) printf '%s\n' 'Architecture must be amd64 or arm64.' >&2; exit 1 ;; esac
cd "$sidelet_root"
bash scripts/go.sh test ./internal/spike ./internal/storage ./internal/reminder ./internal/settings ./internal/exportdata ./internal/quickadd ./internal/integration
bash scripts/go.sh -C tools/windows-release run . -root "$sidelet_root" -arch "$sidelet_arch"
