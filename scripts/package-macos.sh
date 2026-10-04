#!/usr/bin/env bash
set -euo pipefail
sidelet_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$sidelet_root"
sidelet_arch="${1:-arm64}"
bash scripts/build-macos.sh "$sidelet_arch"
sidelet_version="$(python3 scripts/macos-bundle-info.py version)"
sidelet_output="$sidelet_root/build/releases"
mkdir -p "$sidelet_output"
sidelet_work="$(mktemp -d "$sidelet_output/.sidelet-package.XXXXXX")"
trap 'rm -rf "$sidelet_work"' EXIT
mkdir "$sidelet_work/volume"
ditto "$sidelet_root/build/bin/Sidelet.app" "$sidelet_work/volume/Sidelet.app"
ln -s /Applications "$sidelet_work/volume/Applications"
python3 - "$sidelet_version" "$sidelet_arch" "$sidelet_work/volume/使用说明.txt" <<'PYINFO'
from pathlib import Path
import sys
platform = 'Apple Silicon（M 系列）' if sys.argv[2] == 'arm64' else 'Intel（x86_64，尚未实机验收）'
text = Path('packaging/macos/使用说明.txt').read_text().replace('{{VERSION}}', sys.argv[1]).replace('{{PLATFORM}}', platform)
Path(sys.argv[3]).write_text(text)
PYINFO
sidelet_filename="Sidelet-$sidelet_version-local-$sidelet_arch.dmg"
hdiutil create -volname "Sidelet $sidelet_version" -srcfolder "$sidelet_work/volume" -format UDZO -fs HFS+ "$sidelet_work/$sidelet_filename"
hdiutil verify "$sidelet_work/$sidelet_filename"
mv -f "$sidelet_work/$sidelet_filename" "$sidelet_output/$sidelet_filename"
(cd "$sidelet_output" && shasum -a 256 "$sidelet_filename" > "$sidelet_filename.sha256")
printf 'Packaged %s\n' "$sidelet_output/$sidelet_filename"
