#!/usr/bin/env python3
"""Record build provenance, then verify the actual read-only mounted release image."""
import argparse
import hashlib
import json
from pathlib import Path
import plistlib
import re
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parent.parent


def sha256(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(chunk)
    return digest.hexdigest()


def bundle_metadata(bundle, expected):
    info = plistlib.loads((bundle / 'Contents/Info.plist').read_bytes())
    fields = {
        'version': 'CFBundleShortVersionString', 'build': 'CFBundleVersion',
        'bundleIdentifier': 'CFBundleIdentifier', 'minimumOS': 'LSMinimumSystemVersion',
    }
    metadata = {key: info.get(field) for key, field in fields.items()}
    desired = {key: expected['minimumSystemVersion' if key == 'minimumOS' else key] for key in fields}
    if metadata != desired or info.get('CFBundleExecutable') != expected['executable']:
        raise ValueError('Packaged application metadata does not match packaging/macos/app.json')
    return metadata


def record_build(bundle, expected, arch):
    metadata = bundle_metadata(bundle, expected)
    metadata.update({
        'architecture': arch,
        'sourceCommit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
        'sourceDirty': bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT, text=True).strip()),
    })
    (bundle / 'Contents/Resources/build-info.json').write_text(json.dumps(metadata, indent=2) + '\n')


def verify_bundle(bundle, expected, arch):
    metadata = bundle_metadata(bundle, expected)
    binary = bundle / 'Contents/MacOS' / expected['executable']
    binary_hash = sha256(binary)
    provenance = json.loads((bundle / 'Contents/Resources/build-info.json').read_text())
    for key, value in {**metadata, 'architecture': arch}.items():
        if provenance.get(key) != value:
            raise ValueError(f'Packaged build provenance mismatch: {key}')
    if not isinstance(provenance.get('sourceDirty'), bool) or not re.fullmatch(r'[a-f0-9]{40,64}', provenance.get('sourceCommit', '')):
        raise ValueError('Missing source provenance')
    subprocess.run(['codesign', '--verify', '--deep', '--strict', str(bundle)], check=True)
    architectures = subprocess.check_output(['lipo', '-archs', str(binary)], text=True).split()
    if architectures != ['x86_64' if arch == 'amd64' else arch]:
        raise ValueError(f'Unexpected binary architectures: {architectures}')
    return {**provenance, 'binarySHA256': binary_hash}


def verify_image(image, expected, arch):
    filename = f"Sidelet-{expected['version']}-build{expected['build']}-local-{arch}.dmg"
    if image.name != filename:
        raise ValueError('Installer filename does not match the expected version/build/architecture')
    subprocess.run(['hdiutil', 'verify', str(image)], check=True)
    with tempfile.TemporaryDirectory(prefix='sidelet-package-') as temporary:
        mount = Path(temporary) / 'volume'
        mount.mkdir()
        subprocess.run(['hdiutil', 'attach', '-readonly', '-nobrowse', '-mountpoint', str(mount), str(image)], check=True)
        try:
            metadata = verify_bundle(mount / 'Sidelet.app', expected, arch)
        finally:
            subprocess.run(['hdiutil', 'detach', str(mount)], check=True)
    receipt = {
        **metadata, 'schemaVersion': 1, 'platform': 'macOS', 'filename': filename,
        'bytes': image.stat().st_size, 'sha256': sha256(image),
    }
    image.with_name(image.name + '.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(f"Verified {filename}: build {receipt['build']}, SHA-256 {receipt['sha256']}")


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['record-build', 'verify'])
    parser.add_argument('path', type=Path)
    parser.add_argument('architecture', choices=['arm64', 'amd64'])
    args = parser.parse_args()
    expected = json.loads((ROOT / 'packaging/macos/app.json').read_text())
    if args.action == 'record-build':
        record_build(args.path, expected, args.architecture)
    else:
        verify_image(args.path.resolve(), expected, args.architecture)
