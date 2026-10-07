#!/usr/bin/env bash
set -euo pipefail
lab_root="$(cd "$(dirname "$0")/../.." && pwd)"
lab_go="$lab_root/.tools/go/bin/go"
if command -v go >/dev/null 2>&1; then lab_go="$(command -v go)"; fi
if [[ "$(uname -s)" != Darwin ]]; then printf '%s\n' 'Build the experiment bundle on macOS.' >&2; exit 1; fi
export GOCACHE="$lab_root/.cache/go-build"
export GOMODCACHE="$lab_root/.cache/gomod"
export CGO_ENABLED=0
python3 "$lab_root/scripts/sync-mygo-ui.py" --check
lab_bundle="$lab_root/build/bin/mygo-lab/SideletMyGoLab.app"
mkdir -p "$lab_bundle/Contents/MacOS"
"$lab_go" -C "$lab_root/experiments/mygo-sidebar" test ./...
"$lab_go" -C "$lab_root/experiments/mygo-sidebar" vet ./...
"$lab_go" -C "$lab_root/experiments/mygo-sidebar" build -trimpath -ldflags='-s -w' -o "$lab_bundle/Contents/MacOS/sidelet" .
python3 - "$lab_bundle" <<'PY'
from pathlib import Path
import plistlib
import sys
bundle = Path(sys.argv[1])
info = {
    'CFBundleIdentifier': 'io.sidelet.mygo-lab',
    'CFBundleName': 'Sidelet MyGo Lab',
    'CFBundleDisplayName': 'Sidelet MyGo Lab',
    'CFBundleExecutable': 'sidelet',
    'CFBundlePackageType': 'APPL',
    'CFBundleVersion': '1',
    'CFBundleShortVersionString': '0.0.1',
    'NSHighResolutionCapable': True,
    'LSMinimumSystemVersion': '13.0',
}
(bundle / 'Contents/Info.plist').write_bytes(plistlib.dumps(info))
PY
codesign --force --sign - "$lab_bundle"
codesign --verify --strict "$lab_bundle"
printf 'Built experiment: %s\n' "$lab_bundle"
