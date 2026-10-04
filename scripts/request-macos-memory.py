#!/usr/bin/env python3
"""Request a local Sidelet snapshot and wait for profiles plus every view reply."""
import argparse
import json
import os
from pathlib import Path
import re
import tempfile
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--dir", required=True, type=Path)
parser.add_argument("--label", required=True)
parser.add_argument("--timeout", type=float, default=30)
args = parser.parse_args()
if not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}", args.label):
    parser.error("label must contain 1–64 letters, digits, underscores or hyphens")
if args.timeout <= 0 or not args.dir.is_dir():
    parser.error("provide an existing memory directory and positive timeout")
destination = args.dir / args.label
if destination.exists():
    parser.error("snapshot already exists; choose a new label")
# Publish a complete request atomically, without overwriting a pending one.
with tempfile.NamedTemporaryFile(dir=args.dir, prefix="request-", delete=False) as f:
    temporary = Path(f.name)
    f.write((args.label + "\n").encode())
    f.flush()
    os.fsync(f.fileno())
try:
    os.link(temporary, args.dir / "request.txt")
finally:
    temporary.unlink()
deadline = time.monotonic() + args.timeout
while time.monotonic() < deadline:
    try:
        native = json.loads((destination / "native.json").read_text())
        if not native.get('enabled') or native.get('boundStates', 0) < 2:
            time.sleep(0.1)
            continue
        expected = ["go.json", "heap.pprof", "allocs.pprof", "goroutine.pprof", "control.json", "quick.json"]
        expected += [f"stack-{i}.json" for i in range(native["boundStates"] - 1)]
        if all((destination / name).is_file() for name in expected):
            # Validate replies, rather than accepting files mid-write.
            for name in expected:
                if name.endswith(".json"):
                    reply = json.loads((destination / name).read_text())
                    if name in ('control.json', 'quick.json') or name.startswith('stack-'):
                        if reply.get('label') != args.label or reply.get('window') != name[:-5]:
                            raise ValueError(f"mismatched view reply: {name}")
            print(destination)
            break
    except (FileNotFoundError, json.JSONDecodeError):
        pass
    time.sleep(0.1)
else:
    raise SystemExit(f"Incomplete snapshot after {args.timeout}s: {destination}; inspect the app log")
