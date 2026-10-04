#!/usr/bin/env bash
set -euo pipefail
sidelet_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$sidelet_root"
npm run check
npm test
bash scripts/go.sh test -race ./internal/... ./cmd/spike
bash scripts/go.sh vet ./internal/... ./cmd/spike
if [[ "$(uname -s)" == Darwin ]]; then
  bash scripts/test-macos-hit-regions.sh
  bash scripts/test-macos-transparency.sh
fi
