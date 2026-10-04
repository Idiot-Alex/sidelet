#!/usr/bin/env bash
set -euo pipefail
sidelet_root="$(cd "$(dirname "$0")/.." && pwd)"
if command -v go >/dev/null 2>&1; then
  sidelet_go="$(command -v go)"
elif [[ -x "$sidelet_root/.tools/go/bin/go" ]]; then
  sidelet_go="$sidelet_root/.tools/go/bin/go"
else
  printf '%s\n' 'Install Go 1.27.1 or newer, then retry.' >&2
  exit 1
fi
export GOMODCACHE="$sidelet_root/.cache/go-mod"
export GOCACHE="$sidelet_root/.cache/go-build"
if [[ "$(uname -s)" == Darwin && "${GOOS:-darwin}" == darwin ]]; then
  export MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-13.0}"
  export CGO_CFLAGS="${CGO_CFLAGS:-} -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
  export CGO_LDFLAGS="${CGO_LDFLAGS:-} -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET -Wl,-no_warn_duplicate_libraries"
fi
cd "$sidelet_root"
exec "$sidelet_go" "$@"
