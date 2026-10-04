#!/usr/bin/env python3
"""Single source of bundle metadata; outputs either a field or a complete plist."""
import json
from pathlib import Path
import plistlib
import re
import sys

root = Path(__file__).resolve().parent.parent
info = json.loads((root / 'packaging/macos/app.json').read_text())
assert re.fullmatch(r'\d+\.\d+\.\d+', info['version']), 'Invalid version'
assert re.fullmatch(r'\d+', info['build']), 'Invalid build number'
if len(sys.argv) == 2 and sys.argv[1] in info:
    print(info[sys.argv[1]])
elif len(sys.argv) == 3 and sys.argv[1] == '--plist':
    data = {
        'CFBundleName': info['name'], 'CFBundleDisplayName': info['name'],
        'CFBundleIdentifier': info['bundleIdentifier'], 'CFBundleExecutable': info['executable'],
        'CFBundlePackageType': 'APPL', 'CFBundleShortVersionString': info['version'],
        'CFBundleVersion': info['build'], 'CFBundleIconFile': 'Sidelet.icns',
        'LSMinimumSystemVersion': info['minimumSystemVersion'], 'LSUIElement': True,
        'NSHighResolutionCapable': True, 'NSPrincipalClass': 'NSApplication',
        'NSAppTransportSecurity': {'NSAllowsLocalNetworking': True},
    }
    Path(sys.argv[2]).write_bytes(plistlib.dumps(data))
else:
    raise SystemExit('usage: macos-bundle-info.py FIELD | --plist OUTPUT')
