#!/usr/bin/env bash
set -euo pipefail
sidelet_root="$(cd "$(dirname "$0")/.." && pwd)"
sidelet_arch="${1:-amd64}"
case "$sidelet_arch" in amd64|arm64) ;; *) printf '%s\n' 'Architecture must be amd64 or arm64.' >&2; exit 1 ;; esac
cd "$sidelet_root"
npm run build:frontend
bash scripts/go.sh test ./internal/spike ./internal/storage ./internal/reminder ./internal/settings ./internal/exportdata ./internal/quickadd ./internal/integration
mkdir -p build/bin
GOOS=windows GOARCH="$sidelet_arch" CGO_ENABLED=0 bash scripts/go.sh vet ./cmd/spike ./internal/platform
GOOS=windows GOARCH="$sidelet_arch" CGO_ENABLED=0 bash scripts/go.sh build -trimpath -ldflags='-H=windowsgui' -o "build/bin/sidelet-spike-$sidelet_arch.exe" ./cmd/spike
bash scripts/go.sh version -m "build/bin/sidelet-spike-$sidelet_arch.exe"
