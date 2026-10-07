#!/usr/bin/env python3
"""Generate MyGo's visual assets from the production Web UI (no new design)."""
import argparse
import hashlib
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parent.parent
DEST = ROOT / "experiments/mygo-sidebar/design/web-ui.json"
SOURCES = ["frontend/src/theme.css", "frontend/src/components/Icon.svelte",
           "frontend/src/components/AppHeader.svelte", "frontend/src/components/TodoManager.svelte",
           "frontend/src/components/SettingsPanel.svelte", "frontend/src/components/QuickCard.svelte",
           "frontend/src/components/QuickAdd.svelte", "frontend/src/components/EdgeStack.svelte",
           "frontend/src/components/TaskOrder.svelte", "frontend/src/lib/geometry.ts", "frontend/src/lib/reorder.ts", "frontend/src/components/StackDragHandle.svelte"]


def generate():
    css = (ROOT / SOURCES[0]).read_text()
    themes = {}
    for name in ("mac", "paper", "graphite"):
        block = re.search(r'\[data-theme="' + name + r'"\]\s*\{([^}]+)\}', css).group(1)
        values = dict(re.findall(r"--([\w-]+)\s*:\s*([^;]+);", block))
        if name != "mac":
            values = {**themes["mac"], **values}
        themes[name] = values
    source = (ROOT / SOURCES[1]).read_text()
    paths = dict(re.findall(r"\b(\w+):\s*'([^']+)'", source[source.index("const paths"):]))
    if len(paths) != 16:
        raise ValueError("Icon.svelte format changed; review the generator")
    return json.dumps({"sourceSHA256": {p: hashlib.sha256((ROOT / p).read_bytes()).hexdigest() for p in SOURCES},
                       "themes": themes, "icons": paths}, ensure_ascii=False, indent=2) + "\n"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="fail if production Web assets changed")
    args = parser.parse_args()
    output = generate()
    if args.check:
        if not DEST.exists() or DEST.read_text() != output:
            raise SystemExit("Web UI changed. Review native UI parity and run python3 scripts/sync-mygo-ui.py.")
        print("MyGo visual assets match the production Web UI sources.")
    else:
        DEST.parent.mkdir(parents=True, exist_ok=True)
        DEST.write_text(output)
        print(f"Generated {DEST.relative_to(ROOT)}")
